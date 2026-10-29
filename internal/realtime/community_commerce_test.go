package realtime

import (
	"bytes"
	"net/url"
	"strconv"
	"strings"
	"testing"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func TestCommercePageTransferAuthorizationAndIdempotence(t *testing.T) {
	s := newTestServer(t)
	sid := login(t, s, "alice")
	x := s.sessions[sid]
	u, _ := url.Parse(s.ccURL("account", "recharge_credit") + "&op=recharge")
	f := url.Values{"csrf": {x.csrf}, "nonce": {strings.Repeat("a", 48)}, "amount": {"1000"}}
	r := ancillaryPage(t, s, x, "POST", u, f)
	if r.Status != 200 {
		t.Fatal(r)
	}

	r = ancillaryPage(t, s, x, "POST", u, f)
	if r.Status != 200 {
		t.Fatal(r)
	}
	if balance, _ := s.store.CreditBalance(t.Context(), "alice"); balance != 1000 {
		t.Fatal(balance)
	}

	u, _ = url.Parse(s.ccURL("account", "transfer_credit") + "&op=transfer")
	f = url.Values{
		"csrf":      {x.csrf},
		"nonce":     {strings.Repeat("b", 48)},
		"recipient": {"bob"},
		"binding":   {fixtureReportBinding(x.csrf, "credit-transfer", "recipient", "bob")},
		"amount":    {"3.50"},
	}

	r = ancillaryPage(t, s, x, "POST", u, f)
	if r.Status != 200 {
		t.Fatal(r.Status, string(r.Body))
	}

	r = ancillaryPage(t, s, x, "POST", u, f)
	if r.Status != 200 {
		t.Fatal(r.Status)
	}
	if balance, _ := s.store.CreditBalance(t.Context(), "bob"); balance != 350 {
		t.Fatal(balance)
	}
	f.Set("recipient", "carol")
	r = ancillaryPage(t, s, x, "POST", u, f)
	if r.Status != 400 {
		t.Fatal("changed bound recipient", r.Status)
	}
	f.Set("csrf", "wrong")
	r = ancillaryPage(t, s, x, "POST", u, f)
	if r.Status != 403 {
		t.Fatal("csrf", r.Status)
	}

	u, _ = url.Parse(s.ccURL("store", "home") + "&ty=1&username=bob")
	r = ancillaryPage(t, s, x, "GET", u, nil)
	body := string(r.Body)
	if r.Status != 200 || !strings.Contains(body, "action=\"http://127.0.0.1:8080/sites/index.php?c=store&a=home&v=midlet&op=purchase&ty=1\"") {
		t.Fatal(r.Status, body)
	}
	if strings.Contains(body, "&amp;") {
		t.Fatal("native attribute entities would break routing")
	}
}

func TestCommunityPagesPrivateMembershipAndForum(t *testing.T) {
	s := newTestServer(t)
	a := s.sessions[login(t, s, "alice")]
	b := s.sessions[login(t, s, "bob")]
	g, err := s.store.CreateCommunity(t.Context(), "alice", "Private Island", "Description", true)
	if err != nil {
		t.Fatal(err)
	}
	base := s.ccURL("group", "home") + "&group_id=" + strconv.FormatInt(g.ID, 10)
	u, _ := url.Parse(base + "&op=join")
	r := ancillaryPage(t, s, b, "POST", u, url.Values{"csrf": {b.csrf}})
	if r.Status != 200 || !strings.Contains(string(r.Body), "waiting for approval") {
		t.Fatal(r.Status, string(r.Body))
	}

	u, _ = url.Parse(s.ccURL("group", "view_forum") + "&group_id=" + strconv.FormatInt(g.ID, 10))
	r = ancillaryPage(t, s, b, "GET", u, nil)
	if r.Status != 403 {
		t.Fatal(r.Status)
	}

	u, _ = url.Parse(s.ccURL("group", "members") + "&group_id=" + strconv.FormatInt(g.ID, 10) + "&op=approve")
	r = ancillaryPage(t, s, b, "POST", u, url.Values{"csrf": {b.csrf}, "username": {"bob"}})
	if r.Status != 403 {
		t.Fatal(r.Status)
	}

	r = ancillaryPage(t, s, a, "POST", u, url.Values{"csrf": {a.csrf}, "username": {"bob"}})
	if r.Status != 200 {
		t.Fatal(r.Status, string(r.Body))
	}

	u, _ = url.Parse(s.ccURL("group", "view_forum") + "&group_id=" + strconv.FormatInt(g.ID, 10) + "&op=post")
	f := url.Values{"csrf": {b.csrf}, "parent": {"0"}, "nonce": {strings.Repeat("c", 48)}, "body": {"Welcome & <script>"}}
	r = ancillaryPage(t, s, b, "POST", u, f)
	if r.Status != 200 || !strings.Contains(string(r.Body), "Welcome &amp; &lt;script&gt;") {
		t.Fatal(r.Status, string(r.Body))
	}

	r = ancillaryPage(t, s, b, "POST", u, f)
	if r.Status != 200 {
		t.Fatal(r.Status)
	}

	posts, err := s.store.CommunityPosts(t.Context(), g.ID, "alice")
	if err != nil || len(posts) != 1 {
		t.Fatal(posts, err)
	}
}

func TestStoreThemePreservesConfiguredFieldsAndRestores(t *testing.T) {
	configured, err := protocol.Decode(bytes.NewReader(fixtureAsset(t, "default_v2")))
	if err != nil {
		t.Fatal(err)
	}
	theme := fixturePacket(events.New(0, events.Theme{ID: "theme-orange"}))
	if theme.Opcode != 1000 || len(theme.Fields) != len(configured.Fields) {
		t.Fatal("partial theme")
	}
	changed := map[int16]bool{0: true, 2: true, 139: true, 142: true, 143: true, 144: true, 145: true}
	for tag, value := range configured.Fields {
		if !changed[tag] && !bytes.Equal(value, theme.Fields[tag]) {
			t.Fatalf("theme modified field%d", tag)
		}
	}

	s := newTestServer(t)
	x := s.sessions[login(t, s, "alice")]
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.store.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", strings.Repeat("d", 48), 1000)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.store.CreditOperation(t.Context(), "alice", "alice", "purchase", "theme-orange", strings.Repeat("e", 48), 300)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyStoreTheme(t.Context(), "alice", "theme-orange"); err != nil {
		t.Fatal(err)
	}
	found := false
	for len(x.push) > 0 {
		p := fixturePacket(<-x.push)
		if p.Opcode == 1000 {
			found = true
		}
	}
	if !found {
		t.Fatal("theme was not delivered")
	}
	if err = s.restoreStoreTheme(t.Context(), x); err != nil {
		t.Fatal(err)
	}
	if p := fixturePacket(<-x.push); p.Opcode != 1000 {
		t.Fatal(p.Opcode)
	}
	for len(x.push) < cap(x.push) {
		x.push <- events.New(0, events.Pong{})
	}
	if err = s.applyStoreTheme(t.Context(), "alice", "theme-orange"); err != chatstore.ErrLimit {
		t.Fatal(err)
	}
}

func TestCommunityCommerceHTTPAndNativePageTransports(t *testing.T) {
	s := newTestServer(t)
	sid := login(t, s, "alice")
	s.mu.Lock()
	csrf := s.sessions[sid].csrf
	s.mu.Unlock()
	create := s.ccURL("group", "home") + "&op=create"
	replies, _ := s.Handle(t.Context(), sid, protocol.New(922, 90).SetByte(1, 2).SetText(2, create).SetText(3, "csrf="+csrf+"&private=0&name=Native%20Island"))
	if len(replies) != 1 || replies[0].Opcode != 923 {
		t.Fatal(replies)
	}
	status, _ := replies[0].Int(1)
	body, _ := replies[0].Text(2)
	if status != 200 {
		t.Fatal(status, body)
	}
	fields := roomHiddenFields(body)
	description := "Native & group + 50% ✓"
	replies, _ = s.Handle(t.Context(), sid, protocol.New(922, 91).
		SetByte(1, 2).
		SetText(2, s.ccURL("group", "home")+"&op=create-details").
		SetText(3, fields.Encode()+"&description="+strings.ReplaceAll(description, " ", "%20")))
	status, _ = replies[0].Int(1)
	if status != 200 {
		body, _ = replies[0].Text(2)
		t.Fatal(status, body)
	}
	groups, err := s.store.Communities(t.Context(), "alice", true)
	if err != nil || len(groups) != 1 || groups[0].Description != description || groups[0].Private {
		t.Fatal(groups, err)
	}
	w := fetchReport(t, s, sid, "GET", s.ccURL("group", "home")+"&cid="+strconv.FormatInt(groups[0].ID, 10), "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Native Island") || !strings.Contains(w.Body.String(), "Your role: owner") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = fetchReport(t, s, sid, "POST", s.ccURL("account", "recharge_credit")+"&op=recharge", url.Values{
		"csrf":   {csrf},
		"nonce":  {strings.Repeat("f", 48)},
		"amount": {"1000"},
	}.Encode())
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = fetchReport(t, s, sid, "GET", s.ccURL("account", "get_history"), "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "test-recharge") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = fetchReport(t, s, "", "GET", s.ccURL("group", "list_my_groups"), "")
	if w.Code != 401 {
		t.Fatal("anonymous Group page", w.Code)
	}
}
