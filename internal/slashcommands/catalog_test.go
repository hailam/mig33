package slashcommands_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/rooms"
	"github.com/hailam/mig33/internal/slashcommands"
)

func TestCatalogShapeAndInsertion(t *testing.T) {
	entries := slashcommands.Catalog()
	byID := map[string]slashcommands.Command{}
	topLevel := 0
	for _, entry := range entries {
		if entry.ID == "" || byID[entry.ID].ID != "" {
			t.Fatalf("empty or duplicate ID: %q", entry.ID)
		}
		byID[entry.ID] = entry
		if entry.Parent == "" {
			topLevel++
		}
		if entry.Command == "" || entry.Command != strings.ToLower(entry.Command) || strings.HasPrefix(entry.Command, "/") || strings.TrimSpace(entry.Command) != entry.Command {
			t.Errorf("%s has invalid command %q", entry.ID, entry.Command)
		}
		if entry.Description == "" || !strings.HasPrefix(entry.Usage, "/"+entry.Command) || strings.TrimSpace(entry.Insert) != "/"+entry.Command {
			t.Errorf("%s has invalid presentation or insertion metadata", entry.ID)
		}
		if strings.ContainsAny(entry.Insert, "<>[]|\n\r") {
			t.Errorf("%s inserts a placeholder or line break: %q", entry.ID, entry.Insert)
		}
		if !slices.Contains([]slashcommands.Arguments{slashcommands.NoArguments, slashcommands.OptionalArguments, slashcommands.RequiredArguments}, entry.Arguments) {
			t.Errorf("%s has invalid argument mode", entry.ID)
		}
		if !slices.Contains([]slashcommands.Scope{slashcommands.AllConversations, slashcommands.JoinedRoom, slashcommands.CommunityRoom}, entry.Scope) {
			t.Errorf("%s has invalid scope", entry.ID)
		}
		if entry.Arguments == slashcommands.NoArguments && entry.Insert != "/"+entry.Command {
			t.Errorf("%s should not insert argument space", entry.ID)
		}
		if entry.Arguments != slashcommands.NoArguments && !strings.HasSuffix(entry.Insert, " ") {
			t.Errorf("%s needs space for its arguments", entry.ID)
		}
	}
	if topLevel != 27 {
		t.Errorf("got %d top-level commands, want 27 supported names", topLevel)
	}
	for _, entry := range entries {
		if entry.Parent != "" {
			parent, ok := byID[entry.Parent]
			if !ok || parent.Parent != "" || strings.Fields(entry.Command)[0] != parent.Command {
				t.Errorf("%s has an invalid parent", entry.ID)
			}
		}
	}
	for _, unsupported := range []string{"kick", "mute", "unmute", "help"} {
		if _, exists := byID[unsupported]; exists {
			t.Errorf("unsupported command /%s is suggested", unsupported)
		}
	}
}

func TestCoreSuggestionsExpandWithRealParser(t *testing.T) {
	count := 0
	for _, entry := range slashcommands.Catalog() {
		if entry.Scope != slashcommands.AllConversations || entry.Command == "gift" || entry.Parent != "" {
			continue
		}
		count++
		t.Run(entry.ID, func(t *testing.T) {
			inputs := []string{entry.Insert}
			switch entry.Arguments {
			case slashcommands.RequiredArguments:
				inputs = []string{entry.Insert + "waves hello"}
			case slashcommands.OptionalArguments:
				inputs = append(inputs, entry.Insert+"bob")
			}
			for _, input := range inputs {
				m := chat.Message{Sender: "alice", Body: input}
				if !chat.CoreInput(input) {
					t.Fatalf("core parser does not recognize %q", input)
				}
				if err := chat.ExpandCommand(&m, []string{"alice", "bob"}, []string{"alice", "bob"}); err != nil {
					t.Fatalf("suggested input %q is invalid: %v", input, err)
				}
				if !m.Action || !strings.HasPrefix(m.Body, "alice ") || strings.HasPrefix(m.Body, "/") {
					t.Errorf("%q was not expanded into an action: %#v", input, m)
				}
			}
		})
	}
	if count != 11 {
		t.Errorf("got %d core suggestions, want 11", count)
	}
}

func TestRoomSuggestionsMatchRealParser(t *testing.T) {
	examples := map[string]string{
		"silence": "off", "lock": "on", "announce": "Welcome!", "broadcast": "Welcome!",
		"ban": "bob", "unban": "bob", "description": "A friendly room", "invite": "bob",
		"bot": "dice", "play": "Will it rain?",
	}
	// Read the recognized names as well as executing ParseControl, so adding or
	// removing an actual server command cannot silently leave suggestions stale.
	want := switchStrings(t, "../rooms/commands.go", "name")
	seen := map[string]bool{}
	for _, entry := range slashcommands.Catalog() {
		if entry.Scope == slashcommands.AllConversations {
			continue
		}
		input := entry.Insert + examples[entry.ID]
		name, _, handled := rooms.ParseControl(input)
		if !handled {
			t.Errorf("room parser rejects %s input %q", entry.ID, input)
		}
		if entry.Parent == "" {
			if !want[entry.Command] || name != entry.Command {
				t.Errorf("room parser/catalog mismatch for %s", entry.ID)
			}
			seen[entry.Command] = true
		}
		for _, alias := range entry.Aliases {
			aliasName, _, ok := rooms.ParseControl("/" + alias)
			if !ok || aliasName != name {
				t.Errorf("%s alias %q does not resolve to %s", entry.ID, alias, name)
			}
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("supported room command /%s missing from catalog", name)
		}
	}
}

func TestGiftSuggestionsMatchHandlerAndSendParser(t *testing.T) {
	// Gift subcommands live in the realtime handler, which has no side-effect-
	// free exported dispatcher. Check its actual switch labels rather than
	// inventing a parallel parser or running purchases just to test suggestions.
	want := switchStrings(t, "../realtime/gift_commands.go", "subcommand")
	delete(want, "") // The top-level /gift entry already covers help.
	seen := map[string]bool{}
	var send slashcommands.Command
	for _, entry := range slashcommands.Catalog() {
		if entry.Parent != "gift" {
			continue
		}
		if entry.ID == "gift-send" {
			send = entry
			continue
		}
		for _, command := range append([]string{entry.Command}, entry.Aliases...) {
			keyword, ok := strings.CutPrefix(command, "gift ")
			if !ok || !want[keyword] {
				t.Errorf("gift handler does not recognize suggested command %q", command)
			}
			seen[keyword] = true
		}
	}
	for keyword := range want {
		if !seen[keyword] {
			t.Errorf("supported /gift %s missing from catalog", keyword)
		}
	}
	if send.Command != "gift" || send.Insert != "/gift " || send.Arguments != slashcommands.RequiredArguments {
		t.Fatal("sending a gift must take a username directly, without an invented send keyword")
	}
	items := commerce.GiftCatalogItems()
	if len(items) == 0 {
		t.Fatal("gift examples need a real catalog item")
	}
	input := send.Insert + "bob " + items[0].Name + " -m Thank you!"
	// Remove the actual command and recipient exactly as the handler does.
	_, rest, _ := strings.Cut(input, " ")
	_, args, _ := strings.Cut(rest, " ")
	name, note, err := commerce.ParseGiftSend(args)
	if err != nil || name != items[0].Name || note != "Thank you!" {
		t.Fatalf("suggested gift syntax was rejected: %q, %q, %v", name, note, err)
	}
	if item, ok := commerce.FindGift(name); !ok || item.ID != items[0].ID {
		t.Fatal("parsed gift name does not resolve to the real catalog")
	}
}

func TestScopeAndPermissionMetadata(t *testing.T) {
	for _, entry := range slashcommands.Catalog() {
		gift := entry.Command == "gift" || entry.Parent == "gift"
		if entry.AllowMuted != (gift || entry.ID == "games") {
			t.Errorf("%s does not reflect the handler's explicit room mute gate", entry.ID)
		}
		if entry.Scope == slashcommands.CommunityRoom {
			switch entry.ID {
			case "scores":
				if entry.Requirement != "" {
					t.Error("scores must not claim to require an active bot or moderator role")
				}
			case "play":
				if !strings.Contains(entry.Requirement, "active room bot") || strings.Contains(entry.Requirement, "moderator") {
					t.Error("play needs an active bot, not moderator powers")
				}
			default:
				if !strings.Contains(entry.Requirement, "community's owner or moderator") {
					t.Errorf("%s must distinguish community authority from room moderator status", entry.ID)
				}
			}
		} else if entry.Requirement != "" {
			t.Errorf("%s incorrectly claims community privileges", entry.ID)
		}
	}
}

func switchStrings(t *testing.T, path, variable string) map[string]bool {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]bool{}
	ast.Inspect(f, func(node ast.Node) bool {
		s, ok := node.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		name, ok := s.Tag.(*ast.Ident)
		if !ok || name.Name != variable {
			return true
		}
		for _, node := range s.Body.List {
			for _, expression := range node.(*ast.CaseClause).List {
				literal, ok := expression.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Fatalf("unexpected nonliteral command case in %s", path)
				}
				value, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatal(err)
				}
				result[value] = true
			}
		}
		return false
	})
	if len(result) == 0 {
		t.Fatalf("no %s switch cases found in %s", variable, path)
	}
	return result
}
