package realtime

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func reviewPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestHiddenActivityDisappearsFromFeedsAndLeaderboard(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	for _, e := range []chatstore.ServiceEntry{
		{Kind: "buzz", Owner: "alice", Body: "Activity marker", State: "published", Nonce: token()},
		{Kind: "checkin", Owner: "alice", Body: "Check-in marker", State: "published", Nonce: token()},
		{Kind: "game", Owner: "alice", Target: "roll", Body: "Result marker", State: "completed", Nonce: token()},
		{Kind: "blog", Owner: "alice", Body: "Authored blog marker", State: "published", Nonce: token()},
	} {
		if _, err := s.store.AddServiceEntry(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.store.SetIdentityPreference(t.Context(), "alice", "view_update_settings", "off"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{s.featureURL("buzz", "home"), s.featureURL("lookout", "home") + "&username=alice"} {
		w := fetchReport(t, s, b, "GET", path, "")
		if w.Code != 200 || strings.Contains(w.Body.String(), "Activity marker") {
			t.Fatal(w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "Check-in marker") {
			t.Fatal("hidden check-in leaked")
		}
		w = fetchReport(t, s, a, "GET", path, "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Activity marker") {
			t.Fatal("owner lost records", w.Code, w.Body.String())
		}
	}
	w := fetchReport(t, s, b, "GET", s.featureURL("leaderboard", "home"), "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "alice: 1 games") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = fetchReport(t, s, b, "GET", s.featureURL("profile", "user_blog_posts")+"&username=alice", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Authored blog marker") {
		t.Fatal("activity preference hid explicitly authored blog", w.Code, w.Body.String())
	}
	if err := s.store.SetIdentityPreference(t.Context(), "alice", "view_update_settings", "on"); err != nil {
		t.Fatal(err)
	}
	w = fetchReport(t, s, b, "GET", s.featureURL("buzz", "home"), "")
	if !strings.Contains(w.Body.String(), "Activity marker") {
		t.Fatal("re-enabling activity did not restore feed")
	}
	if !strings.Contains(w.Body.String(), "Check-in marker") {
		t.Fatal("public check-in absent from activity feed")
	}
	if err := s.store.SetIdentityPreference(t.Context(), "alice", "profile_privacy_settings", "private"); err != nil {
		t.Fatal(err)
	}
	w = fetchReport(t, s, b, "GET", s.featureURL("leaderboard", "home"), "")
	if strings.Contains(w.Body.String(), "alice: 1 games") {
		t.Fatal("private profile leaked through leaderboard")
	}
}

func TestOriginalActivityContactIDAndNativeSubscription(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	entry, err := s.store.AddServiceEntry(t.Context(), chatstore.ServiceEntry{Kind: "buzz", Owner: "alice", Body: "Original cid update", State: "published", Nonce: token()})
	if err != nil {
		t.Fatal(err)
	}
	contact, err := s.store.ContactByUsername(t.Context(), "bob", "alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, controller := range []string{"lookout", "buzz"} {
		u := s.featureURL(controller, "home") + "&cid=" + strconv.Itoa(int(contact.ID)) + "&pres=99"
		w := fetchReport(t, s, b, "GET", u, "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Original cid update") {
			t.Fatal("original contact cid ignored", w.Code, w.Body.String())
		}
		w = fetchReport(t, s, b, "GET", u+"&username=bob", "")
		if w.Code != 400 {
			t.Fatal("ambiguous contact selectors accepted", w.Code)
		}
	}
	r, _ := s.Handle(t.Context(), b, protocol.New(921, 91).SetByte(1, 1))
	if len(r) != 2 || r[0].Opcode != 1 || r[1].Opcode != 422 {
		t.Fatal("native snapshot", r)
	}
	if age, ok := r[1].Int(1); !ok || age < 0 {
		t.Fatal("native age", r[1])
	}
	if kind, ok := r[1].Byte(2); !ok || kind != 1 {
		t.Fatal("native type", r[1])
	}
	if _, ok := r[1].Text(3); ok {
		t.Fatal("missing avatar should not fetch fake /assets/0")
	}
	body, _ := r[1].Text(4)
	if !strings.Contains(body, "alice posted an update.") || !strings.Contains(body, "<a href=") {
		t.Fatal("native safe summary", body)
	}
	if !s.AllowPacket(t.Context(), b, r[1]) {
		t.Fatal("visible snapshot rejected")
	}
	s.FinishWrite(t.Context(), r, true)
	r, _ = s.Handle(t.Context(), b, protocol.New(921, 92).SetByte(1, 1))
	if len(r) != 1 {
		t.Fatal("duplicate subscription repeats snapshot")
	}
	f := serviceTestForm(s, a, "publish", "checkin", "Native Check-in")
	u := s.featureURL("location", "checkin")
	for i := 0; i < 2; i++ {
		w := fetchReport(t, s, a, "POST", u, f.Encode())
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	s.mu.Lock()
	var pending []*events.Event
	for len(s.sessions[b].push) > 0 {
		packet := <-s.sessions[b].push
		if fixturePacket(packet).Opcode == 422 {
			pending = append(pending, packet)
		}
	}
	s.mu.Unlock()
	if len(pending) != 1 {
		t.Fatal("retry broadcast duplicate activity", len(pending))
	}
	if text, _ := fixturePacket(pending[0]).Text(4); !strings.Contains(text, "alice checked in.") {
		t.Fatal(text)
	}
	if err = s.store.SetIdentityPreference(t.Context(), "alice", "view_update_settings", "off"); err != nil {
		t.Fatal(err)
	}
	if s.allowPacket(t.Context(), b, pending[0]) {
		t.Fatal("queued hidden activity leaked")
	}
	if err = s.store.SetIdentityPreference(t.Context(), "alice", "view_update_settings", "on"); err != nil {
		t.Fatal(err)
	}
	s.Handle(t.Context(), b, protocol.New(921, 93).SetByte(1, 0))
	f = serviceTestForm(s, a, "publish", "buzz", "After unsubscribe")
	w := fetchReport(t, s, a, "POST", s.featureURL("buzz", "home"), f.Encode())
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	s.mu.Lock()
	n := len(s.sessions[b].push)
	s.mu.Unlock()
	if n != 0 {
		t.Fatal("unsubscribed user received activity", n)
	}
	if err = s.store.DeleteServiceEntry(t.Context(), "alice", entry.ID); err != nil {
		t.Fatal(err)
	}
}

func TestNativeActivityCancelsDeletedBlockedAndUnsubscribedPackets(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	s.Handle(t.Context(), b, protocol.New(921, 95).SetByte(1, 1))
	publish := func(body string) (*events.Event, chatstore.ServiceEntry) {
		f := serviceTestForm(s, a, "publish", "buzz", body)
		w := fetchReport(t, s, a, "POST", s.featureURL("buzz", "home"), f.Encode())
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		entry, err := s.store.ServiceEntryByNonce(t.Context(), "alice", f.Get("nonce"))
		if err != nil {
			t.Fatal(err)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for len(s.sessions[b].push) > 0 {
			p := <-s.sessions[b].push
			if fixturePacket(p).Opcode == 422 {
				return p, entry
			}
		}
		t.Fatal("subscribed session missing update")
		return nil, entry
	}
	p, entry := publish("Delete before delivery")
	if err := s.store.DeleteServiceEntry(t.Context(), "alice", entry.ID); err != nil {
		t.Fatal(err)
	}
	if s.allowPacket(t.Context(), b, p) {
		t.Fatal("deleted activity still delivered")
	}
	p, _ = publish("Block before delivery")
	s.Handle(t.Context(), b, protocol.New(417, 96).SetText(1, "alice").SetByte(2, 2))
	if s.allowPacket(t.Context(), b, p) {
		t.Fatal("blocked activity still delivered")
	}
	if err := s.store.SetBlocked(t.Context(), "bob", "alice", false); err != nil {
		t.Fatal(err)
	}
	p, _ = publish("Unsubscribe before delivery")
	s.Handle(t.Context(), b, protocol.New(921, 97).SetByte(1, 0))
	if s.allowPacket(t.Context(), b, p) {
		t.Fatal("unsubscribed activity still delivered")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sessions[b].activityPackets) != 0 {
		t.Fatal("canceled activity metadata leaked")
	}
}

func TestPhoto914PrivateAudienceAndBlocking(t *testing.T) {
	s := newTestServer(t)
	a, b, c := login(t, s, "alice"), login(t, s, "bob"), login(t, s, "carol")
	photo, err := s.store.AddPhoto(t.Context(), "alice", "Private original", token(), reviewPNG(t, 192, 48))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetIdentityPreference(t.Context(), "alice", "profile_privacy_settings", "private"); err != nil {
		t.Fatal(err)
	}
	tokenName := photoToken(photo.ID)
	request := func(sid string) *protocol.Packet {
		r, _ := s.Handle(t.Context(), sid, protocol.New(913, 77).SetText(1, tokenName+" ").SetInt(2, 1))
		if len(r) != 1 {
			t.Fatal(r)
		}
		return r[0]
	}
	if p := request(b); p.Opcode != 0 {
		t.Fatal("unshared private photo accessible")
	}
	send := protocol.New(500, 70).SetByte(1, 1).SetByte(3, 1).SetText(4, "bob").SetShort(6, 5).SetText(8, strconv.FormatInt(photo.ID, 10))
	r, _ := s.Handle(t.Context(), a, send)
	if r[0].Opcode != 1 {
		t.Fatal(r)
	}
	p := request(b)
	if p.Opcode != 914 || p.Transaction != 77 {
		t.Fatal(p)
	}
	for _, tag := range []int16{1, 2} {
		if value, _ := p.Text(tag); value != tokenName {
			t.Fatal("alias mismatch", tag, value)
		}
	}
	if kind, _ := p.Byte(5); kind != 1 {
		t.Fatal("photo wrongly uses vibration/audio type", kind)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(p.Fields[4]))
	if err != nil || format != "png" || config.Width != 96 || config.Height != 24 {
		t.Fatal(config, format, err)
	}
	if p = request(c); p.Opcode != 0 {
		t.Fatal("photo audience leaked to third user")
	}
	forward := protocol.New(500, 71).SetByte(1, 1).SetByte(3, 1).SetText(4, "carol").SetShort(6, 5).SetText(8, strconv.FormatInt(photo.ID, 10))
	r, _ = s.Handle(t.Context(), b, forward)
	if r[0].Opcode != 0 {
		t.Fatal("recipient forwarded another owner's private photo")
	}
	r, _ = s.Handle(t.Context(), b, protocol.New(417, 72).SetText(1, "alice").SetByte(2, 2))
	if r[0].Opcode != 1 {
		t.Fatal(r)
	}
	if p = request(b); p.Opcode != 0 {
		t.Fatal("blocked media remained accessible")
	}
	req := httptest.NewRequest("GET", s.base.String()+"/assets/photos/"+strconv.FormatInt(photo.ID, 10), nil)
	req.Header.Set("sid", b)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal("HTTP bypassed media block", w.Code)
	}
	if err = s.store.SetBlocked(t.Context(), "bob", "alice", false); err != nil {
		t.Fatal(err)
	}
	if p = request(b); p.Opcode != 0 {
		t.Fatal("unblock restored canceled private photo audience")
	}
}

type pausedMediaWriter struct {
	header           http.Header
	started, release chan struct{}
	once             sync.Once
}

func (w *pausedMediaWriter) Header() http.Header { return w.header }

func (w *pausedMediaWriter) WriteHeader(int) {}

func (w *pausedMediaWriter) Write(b []byte) (int, error) {
	w.once.Do(func() { close(w.started) })
	<-w.release
	return len(b), nil
}

func TestSlowMediaDownloadDoesNotHoldChatMutex(t *testing.T) {
	s := newTestServer(t)
	sid := login(t, s, "alice")
	photo, err := s.store.AddPhoto(t.Context(), "alice", "", token(), reviewPNG(t, 2, 2))
	if err != nil {
		t.Fatal(err)
	}
	w := &pausedMediaWriter{header: make(http.Header), started: make(chan struct{}), release: make(chan struct{})}
	finished := make(chan struct{})
	req := httptest.NewRequest("GET", s.base.String()+"/assets/photos/"+strconv.FormatInt(photo.ID, 10), nil)
	req.Header.Set("sid", sid)
	go func() { s.ServeHTTP(w, req); close(finished) }()
	select {
	case <-w.started:
	case <-time.After(time.Second):
		close(w.release)
		t.Fatal("download never started")
	}
	ping := make(chan struct{})
	go func() { s.Handle(t.Context(), sid, protocol.New(2, 88)); close(ping) }()
	blocked := false
	select {
	case <-ping:
	case <-time.After(time.Second):
		blocked = true
	}
	close(w.release)
	<-finished
	<-ping
	if blocked {
		t.Fatal("image response held shared chat mutex")
	}
}

func TestRejectedRawPhotoMessagesDoNotConsumeScrapbookSlots(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	data := reviewPNG(t, 3, 2)
	for _, destination := range []struct {
		kind   byte
		target string
	}{{1, "missing_user"}, {2, "not-a-group"}, {3, "Lobby"}} {
		p := protocol.New(500, 81).SetByte(1, 1).SetByte(3, destination.kind).SetText(4, destination.target).SetShort(6, 2).SetBytes(8, data)
		r, _ := s.Handle(t.Context(), a, p)
		if r[0].Opcode != 0 {
			t.Fatal("expected rejection", destination, r)
		}
		photos, err := s.store.Photos(t.Context(), "alice")
		if err != nil || len(photos) != 0 {
			t.Fatal("rejected send persisted upload", destination, len(photos), err)
		}
	}
	if err := s.store.SetIdentityPreference(t.Context(), "bob", "chat_privacy", "nobody"); err != nil {
		t.Fatal(err)
	}
	r, _ := s.Handle(t.Context(), a, protocol.New(500, 82).SetByte(1, 1).SetByte(3, 1).SetText(4, "bob").SetShort(6, 2).SetBytes(8, data))
	if r[0].Opcode != 0 {
		t.Fatal("privacy rejection missing")
	}
	if photos, _ := s.store.Photos(t.Context(), "alice"); len(photos) != 0 {
		t.Fatal("private recipient rejection persisted upload")
	}
	joinRoom(t, s, b, "Lobby")
	if err := s.store.SetRoomMuted(t.Context(), "Lobby", "alice", "bob", true); err != nil {
		t.Fatal(err)
	}
	r, _ = s.Handle(t.Context(), b, protocol.New(500, 83).SetByte(1, 1).SetByte(3, 3).SetText(4, "Lobby").SetShort(6, 2).SetBytes(8, data))
	if r[0].Opcode != 0 {
		t.Fatal("mute rejection missing")
	}
	if photos, _ := s.store.Photos(t.Context(), "bob"); len(photos) != 0 {
		t.Fatal("muted room send persisted upload")
	}
}

func TestLocalProviderPrivacyUsesOriginalPrefillParameters(t *testing.T) {
	s := newTestServer(t)
	a, b := login(t, s, "alice"), login(t, s, "bob")
	w := fetchReport(t, s, a, "GET", s.featureURL("email", "compose")+"&to=bob", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "To: bob") {
		t.Fatal("original email to= ignored", w.Code, w.Body.String())
	}
	if err := s.store.SetIdentityPreference(t.Context(), "bob", "call_by_name", "off"); err != nil {
		t.Fatal(err)
	}
	f := serviceTestForm(s, a, "send", "bob", "+9607775678")
	w = fetchReport(t, s, a, "POST", s.featureURL("call", "home")+"&number_to_call=bob", f.Encode())
	if w.Code != 403 {
		t.Fatal("call_by_name ignored", w.Code, w.Body.String())
	}
	if err := s.store.SetIdentityPreference(t.Context(), "bob", "call_by_name", "on"); err != nil {
		t.Fatal(err)
	}
	w = fetchReport(t, s, a, "POST", s.featureURL("call", "home")+"&number_to_call=bob", f.Encode())
	if w.Code != 200 || !strings.Contains(w.Body.String(), "pending_provider") {
		t.Fatal(w.Code, w.Body.String())
	}
	s.Handle(t.Context(), b, protocol.New(417, 90).SetText(1, "alice").SetByte(2, 2))
	f = serviceTestForm(s, a, "send", "bob", "Blocked local mail")
	w = fetchReport(t, s, a, "POST", s.featureURL("email", "compose")+"&to=bob", f.Encode())
	if w.Code != 403 {
		t.Fatal("blocked user delivered local mail", w.Code)
	}
	entries, err := s.store.ServiceEntries(t.Context(), "mail", "", "bob")
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
}
