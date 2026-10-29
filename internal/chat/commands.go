package chat

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

// These names are historically attested. Except /brb's recorded wording, the
// short templates are explicit local replacements, not recovered server text.
var emoteTemplates = map[string][2]string{
	"hug":   {"hugs everyone.", "hugs %s."},
	"slap":  {"slaps their forehead.", "slaps %s."},
	"wave":  {"waves.", "waves to %s."},
	"smile": {"smiles.", "smiles at %s."},
	"laugh": {"laughs.", "laughs with %s."},
	"cry":   {"cries.", "cries on %s's shoulder."},
	"brb":   {"will be right back", "will be right back, %s."},
}

// Core chat actions commit their result and request key with the message.
// Administrative/bot commands use separate state changes and cannot promise
// message-form idempotency, so JSON/WAP composers exclude them.
func CoreInput(body string) bool {
	text := strings.TrimSpace(body)
	if !strings.HasPrefix(text, "/") {
		return true
	}
	parts := strings.Fields(text)
	name := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	_, emote := emoteTemplates[name]
	return emote || name == "me" || name == "roll" || name == "fortune" || name == "8ball"
}

// The original banks and roll bounds were not recovered. These small banks
// and inclusive1–100 roll range are documented reconstruction choices.
var fortuneReplies = []string{
	"A friendly conversation may brighten your day.",
	"A little patience can open a new door.",
	"Make time for someone you have missed.",
	"Today is a good day to try something new.",
	"Small acts of kindness travel far.",
}

var eightBallReplies = []string{
	"Yes.", "No.", "Maybe.", "Ask again later.",
	"Very likely.", "Unlikely.", "The answer is unclear.", "Give it some time.",
}

// ExpandCommand applies chat action policy to an already authorized conversation.
// Members and knownUsers are immutable snapshots supplied by the realtime owner.
func ExpandCommand(m *Message, members, knownUsers []string) error {
	text := strings.TrimSpace(m.Body)
	if !strings.HasPrefix(text, "/") {
		return nil
	}
	known := func(target string) bool {
		for _, user := range knownUsers {
			if user == target {
				return true
			}
		}
		return false
	}
	contains := func(target string) bool {
		for _, user := range members {
			if user == target {
				return true
			}
		}
		return false
	}

	name, args := text[1:], ""
	if split := strings.IndexFunc(name, unicode.IsSpace); split >= 0 {
		name, args = name[:split], strings.TrimSpace(name[split:])
	}
	name = strings.ToLower(name)
	var action string
	if templates, found := emoteTemplates[name]; found {
		if args == "" {
			action = templates[0]
		} else {
			target := strings.ToLower(args)
			if len(strings.Fields(args)) != 1 || !known(target) || !contains(target) {
				return fmt.Errorf("Use /%s with no target or one username in this conversation", name)
			}
			action = fmt.Sprintf(templates[1], target)
		}
	} else {
		switch name {
		case "me":
			if args == "" {
				return fmt.Errorf("Usage: /me <action text>")
			}
			action = args
		case "roll", "fortune", "8ball":
			if args != "" {
				return fmt.Errorf("Usage: /%s (no arguments)", name)
			}
			count := 100
			if name == "fortune" {
				count = len(fortuneReplies)
			} else if name == "8ball" {
				count = len(eightBallReplies)
			}
			n, err := rand.Int(rand.Reader, big.NewInt(int64(count)))
			if err != nil {
				return fmt.Errorf("A random response could not be generated; try again")
			}
			switch name {
			case "roll":
				action = fmt.Sprintf("rolls %d", n.Int64()+1)
			case "fortune":
				action = "fortune: " + fortuneReplies[n.Int64()]
			case "8ball":
				action = "8ball: " + eightBallReplies[n.Int64()]
			}
		case "kick":
			return fmt.Errorf("Slash /kick is unavailable: its historical meaning varied. Use the room Menu > Kick User")
		case "mute", "unmute":
			return fmt.Errorf("Slash /%s is unavailable. Room moderators can use Menu > Room Info", name)
		case "silence", "ban", "unban", "lock", "unlock", "announce", "broadcast", "description", "invite", "bot", "botstop":
			return fmt.Errorf("Use this command in a community Group-linked chat room")
		default:
			return fmt.Errorf("Unsupported command. Available: /me /hug /slap /wave /smile /laugh /cry /brb /roll /fortune /8ball")
		}
	}
	m.Body = m.Sender + " " + action
	if len(m.Body) > 64*1024 {
		return fmt.Errorf("Expanded command exceeds 65536 UTF-8 bytes")
	}
	m.Action = true
	m.Color = nil // Native content6 suppresses its prefix only with default color.
	return nil
}
