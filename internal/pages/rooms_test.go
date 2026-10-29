package pages

import (
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
)

func roomFormFields(t *testing.T, result Result, names ...string) string {
	t.Helper()
	body := string(result.Body)
	fields := regexp.MustCompile(`name="([^"]+)"`).FindAllStringSubmatch(body, -1)
	if len(fields) != len(names) {
		t.Fatalf("form fields = %v, want %v", fields, names)
	}
	for i, name := range names {
		if fields[i][1] != name {
			t.Fatalf("field order = %v, want %v", fields, names)
		}
	}
	if result.Status != 200 || !strings.Contains(body, `<form method="post"`) {
		t.Fatalf("invalid native form: %s", body)
	}
	return body
}

func TestRoomCreateNativeTwoStageContract(t *testing.T) {
	nameAction := "http://127.0.0.1:8080/sites/index.php?c=chatroom&a=create&v=midlet&stage=name"
	name := roomFormFields(t, RoomCreate(nameAction, "csrf", 15), "csrf", "name")
	if !strings.Contains(name, `action="`+nameAction+`"`) || strings.Contains(name, "&amp;a=") ||
		!strings.Contains(name, `<input type="text" name="name" value="" size="10" maxlength="15">`) ||
		!strings.Contains(name, `<input type="submit" value="Continue">`) {
		t.Fatal("name step lost full-width input or native form action")
	}
	roomName := "Test Room_2-A"
	detailsAction := strings.Replace(nameAction, "stage=name", "stage=details", 1)
	details := roomFormFields(t, RoomCreateDetails(detailsAction, "csrf", roomName, "bound"), "csrf", "room", "binding", "description")
	if !strings.Contains(details, `name="room" value="`+hex.EncodeToString([]byte(roomName))+`"`) ||
		!strings.Contains(details, `name="binding" value="bound"`) ||
		!strings.Contains(details, `<textarea name="description" rows="4" cols="10">`) ||
		!strings.Contains(details, `<input type="submit" value="Create Room">`) {
		t.Fatal("details step lost encoded name, binding, final textarea or unnamed submit")
	}
	if !strings.Contains(details, "Room: "+roomName) || strings.Contains(name, "Your room is ready") {
		t.Fatal("name step claims creation or details step hides the chosen name")
	}
}

func TestRoomSearchNativeLinksAndEscapedQuery(t *testing.T) {
	action := "http://127.0.0.1:8080/sites/index.php?c=chatroom&a=search&v=midlet"
	room := RoomView{
		Name:      "Test Room",
		Capacity:  50,
		Occupancy: 3,
		InfoURL:   "http://127.0.0.1:8080/sites/index.php?c=chatroom&a=setup&v=midlet&roomName=Test+Room",
	}
	body := roomFormFields(t, RoomSearch(action, `&name="<injected>"`, []RoomView{room}, true), "query", "query", "query")
	forms := regexp.MustCompile(`(?s)<form[^>]*>.*?</form>`).FindAllString(body, -1)
	if len(forms) != 3 {
		t.Fatalf("want three independent filters: %s", body)
	}
	for _, form := range forms {
		if strings.Count(form, `name="query"`) != 1 {
			t.Fatal("native filter contains multiple free text values")
		}
	}
	if strings.Contains(body, "<injected>") || !strings.Contains(body, "Results for: &amp;name=") ||
		!strings.Contains(body, `name="query" value="" size="10"`) {
		t.Fatal("query HTML escaped incorrectly or unsafe dynamic value was put in native input")
	}
	if !strings.Contains(body, `href="mig33:joinChatroom(Test Room)"`) || strings.Contains(body, "joinChatroom(Test%20Room)") ||
		!strings.Contains(body, `href="`+room.InfoURL+`"`) || !strings.Contains(body, "People: 3 / 50") {
		t.Fatalf("result lost original join/info link contract: %s", body)
	}
	initial := string(RoomSearch(action, "", nil, false).Body)
	empty := string(RoomSearch(action, "missing", nil, true).Body)
	if strings.Contains(initial, "No chat rooms found.") || !strings.Contains(empty, "No chat rooms found.") {
		t.Fatal("initial form and empty result have the same state")
	}
}

func TestRoomInfoReadOnlyAndJoinGrammar(t *testing.T) {
	view := RoomView{
		Name:        "Test Room",
		Description: `<script>"bad" & more</script>`,
		Owner:       "hailam",
		Capacity:    50,
		Occupancy:   2,
		Moderators:  []string{"hailam", "alice"},
	}
	body := string(RoomInfo(view).Body)
	for _, wanted := range []string{"<title>Room Info</title>", "People: 2 / 50", "Owner: hailam", "Moderators: hailam, alice", "&lt;script&gt;"} {
		if !strings.Contains(body, wanted) {
			t.Fatalf("room info missing %q", wanted)
		}
	}
	if strings.Contains(body, "<form") || strings.Contains(body, "<script>") {
		t.Fatal("room info is mutable or permits dynamic HTML")
	}
	for _, unsafe := range []string{
		"",
		" Room",
		"Room ",
		"Test%20Room",
		"Test&Room",
		"Test(Room)",
		`Test"><input>`,
		"Room\nTwo",
		"Røøm",
		strings.Repeat("a", 41),
	} {
		view.Name = unsafe
		if body := string(RoomInfo(view).Body); strings.Contains(body, "mig33:joinChatroom(") || strings.Contains(body, `<input>`) {
			t.Fatalf("unsafe room name %q became an executable command or markup", unsafe)
		}
	}
	view.Name = "Test Room"
	created := string(RoomCreated(view).Body)
	if !strings.Contains(created, "<title>Room Created</title>") || !strings.Contains(created, "Your room is ready.") || strings.Contains(created, "You are its moderator.") ||
		!strings.Contains(created, `href="mig33:joinChatroom(Test Room)"`) || strings.Contains(created, "You have joined") {
		t.Fatal("creation confirmation omits native join or implies automatic membership")
	}
}
