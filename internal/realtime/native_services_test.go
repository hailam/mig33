package realtime

import (
	"bytes"
	"encoding/binary"
	"image"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/adapters/nativecodec"
	chatstore "github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/protocol"
)

func TestNativeAvatarUsesActualReceivedIDAndOwnership(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	b := login(t, s, "bob")
	photo, err := s.store.AddPhoto(t.Context(), "alice", "", token(), fixtureAsset(t, "default-avatar.png"))
	if err != nil {
		t.Fatal(err)
	}
	key := "photos/" + strconv.FormatInt(photo.ID, 10)
	for _, test := range []struct {
		sid, key string
		want     int16
	}{{b, key, 0}, {a, "https://remote/avatar", 0}, {a, key, 1}} {
		r, _ := s.Handle(t.Context(), test.sid, protocol.New(602, 20).SetText(1, test.key))
		if r[0].Opcode != test.want {
			t.Fatalf("602 %q: %+v", test.key, r)
		}
	}
	if id, _ := s.store.PhotoAvatar(t.Context(), "alice"); id != photo.ID {
		t.Fatal("avatar not saved")
	}

	r, _ := s.Handle(t.Context(), a, protocol.New(602, 21).SetText(1, ""))
	if r[0].Opcode != 1 {
		t.Fatal("reset failed")
	}
	if id, _ := s.store.PhotoAvatar(t.Context(), "alice"); id != 0 {
		t.Fatal("reset not persisted")
	}
}

func TestNativeBrowserImageTTLDoesNotExpireBeforePaint(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	photo, err := s.store.AddPhoto(t.Context(), "alice", "", token(), fixtureAsset(t, "default-avatar.png"))
	if err != nil {
		t.Fatal(err)
	}
	p := protocol.New(922, 24).SetByte(1, 1).SetInt(4, 2).SetText(2, s.base.String()+"/assets/photos/"+strconv.FormatInt(photo.ID, 10)+"?w=96&h=96")
	r, _ := s.Handle(t.Context(), a, p)
	status, _ := r[0].Int(1)
	ttl, _ := r[0].Long(3)
	if status != 200 || ttl != 60000 {
		t.Fatalf("image expires before original paint: status=%d ttl=%d", status, ttl)
	}
	if _, _, err = image.Decode(bytes.NewReader(r[0].Fields[2])); err != nil {
		t.Fatal(err)
	}
	p.SetInt(4, 1).SetText(2, s.featureURL("photo", "home"))
	r, _ = s.Handle(t.Context(), a, p)
	if ttl, _ := r[0].Long(3); ttl != 0 {
		t.Fatal("dynamic page acquired image caching")
	}
}

func TestNativeProviderReceiptsAreIdempotentAndNeverDial(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	requests := []*protocol.Packet{
		protocol.New(800, 40).SetText(1, "+9607771234").SetText(2, "+9607775678").SetByte(4, 0).SetByte(5, 1),
		protocol.New(900, 41).SetText(1, "+9607771234").SetText(2, "A & B"),
	}

	for _, p := range requests {
		for repeat := 0; repeat < 2; repeat++ {
			r, _ := s.Handle(t.Context(), a, p)
			if r[0].Opcode != 1 {
				t.Fatalf("receipt failed: %+v", r)
			}
			for _, reply := range r {
				if reply.Opcode == 804 {
					t.Fatal("pending request dialed a telephone")
				}
			}
			message, _ := r[0].Text(1)
			if !strings.Contains(message, "pending_provider") {
				t.Fatal("misleading receipt")
			}
		}
		kind := "call"
		if p.Opcode == 900 {
			kind = "invite"
		}
		entries, err := s.store.ServiceEntries(t.Context(), kind, "alice", "")
		if err != nil || len(entries) != 1 || entries[0].State != "pending_provider" {
			t.Fatalf("duplicate/sent request: %+v %v", entries, err)
		}
	}

	r, _ := s.Handle(t.Context(), a, protocol.New(900, 42).SetText(1, "not a phone").SetText(2, "Name"))
	if r[0].Opcode != 0 {
		t.Fatal("bad phone accepted")
	}
}

func TestNativeSMSAndMailUseLocalOutboxWithoutFalseSendAck(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	packet := func(channel byte, target, body string, tx int16) *protocol.Packet {
		return protocol.New(500, tx).SetByte(1, channel).SetText(2, "forged-sender").SetByte(3, 1).SetText(4, target).SetShort(6, 1).SetText(8, body)
	}
	for _, test := range []struct {
		channel             byte
		target, kind, state string
		reply               int16
	}{
		{2, "+9607771234", "sms", "pending_provider", 0},
		{9, "person@example.test", "mail", "pending_provider", 1},
		{9, "bob", "mail", "local_delivered", 1},
	} {
		p := packet(test.channel, test.target, "Hello", 80)
		for i := 0; i < 2; i++ {
			r, _ := s.Handle(t.Context(), a, p)
			if r[0].Opcode != test.reply {
				t.Fatalf("provider reply %+v", r)
			}
			tag := int16(1)
			if test.reply == 0 {
				tag = 2
			}
			notice, _ := r[0].Text(tag)
			if !strings.Contains(notice, test.state) {
				t.Fatal("missing honest state", notice)
			}
		}
		entries, err := s.store.ServiceEntries(t.Context(), test.kind, "alice", test.target)
		if err != nil || len(entries) != 1 || entries[0].State != test.state || entries[0].Owner != "alice" {
			t.Fatalf("outbox %+v %v", entries, err)
		}
	}

	if err := s.store.SetIdentityPreference(t.Context(), "bob", "chat_privacy", "nobody"); err != nil {
		t.Fatal(err)
	}

	r, _ := s.Handle(t.Context(), a, packet(9, "bob", "private", 81))
	if r[0].Opcode != 0 {
		t.Fatal("localmail privacy bypass")
	}

	for _, p := range []*protocol.Packet{
		packet(2, "notphone", "text", 82),
		packet(9, "notemail", "text", 83),
		packet(9, "bob", "", 84),
		packet(4, "bob", "externalIM", 85),
		packet(9, "bob", "bytes", 86).SetShort(6, 2),
	} {
		r, _ := s.Handle(t.Context(), a, p)
		if r[0].Opcode != 0 {
			t.Fatal("invalid provider message accepted")
		}
	}
}

func TestNativeIMSettingsDiscardCachedCredentialsAndIconsDecode(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	if _, err := s.store.AddServiceEntry(t.Context(), chatstore.ServiceEntry{
		Kind:   "im",
		Owner:  "alice",
		Target: "MSN",
		Body:   "label",
		Nonce:  token(),
		State:  "disconnected",
	}); err != nil {
		t.Fatal(err)
	}

	r, _ := s.Handle(t.Context(), a, protocol.New(907, 51).SetByte(1, 1).SetByte(30, 2).SetText(16, "obsolete-password-must-not-persist"))
	if len(r) != 2 || r[0].Opcode != 1 || r[1].Opcode != 208 {
		t.Fatalf("907 registry/ack: %+v", r)
	}
	entries, _ := s.store.ServiceEntries(t.Context(), "im", "alice", "")
	if len(entries) != 0 {
		t.Fatal("label not removed")
	}

	r, _ = s.Handle(t.Context(), a, protocol.New(206, 52).SetByte(1, 2).SetByte(2, 1).SetByte(3, 0))
	if len(r) != 1 || r[0].Opcode != 1 {
		t.Fatal("provider connection faked")
	}
	notice, _ := r[0].Text(1)
	if !strings.Contains(notice, "Local saved contacts") || !strings.Contains(notice, "disconnected") {
		t.Fatal("misleading directory acknowledgment")
	}

	r, _ = s.Handle(t.Context(), a, protocol.New(206, 52).SetByte(1, 2))
	if r[0].Opcode != 0 {
		t.Fatal("missing directory mode accepted")
	}

	descriptor := fixturePacket(nativeIMDescriptor(nativeIMServices[0]))
	label, _ := descriptor.Text(2)
	state, _ := descriptor.Byte(4)
	if !strings.Contains(label, "saved contacts (local)") || state != 2 {
		t.Fatal("directory availability mislabeled as provider sign-in")
	}

	r, _ = s.Handle(t.Context(), a, protocol.New(926, 53).SetBytes(1, []byte{2, 3, 4, 6}))
	if len(r) != 4 {
		t.Fatal("missing service icons")
	}

	for _, p := range r {
		for _, tag := range []int16{2, 4, 5, 6} {
			if _, _, err := image.Decode(bytes.NewReader(p.Fields[tag])); err != nil {
				t.Fatal(err)
			}
		}
		if p.Transaction != 53 {
			t.Fatal("uncorrelated icons")
		}
	}

	r, _ = s.Handle(t.Context(), a, protocol.New(926, 54).SetBytes(1, []byte{2, 99}))
	if len(r) != 1 || r[0].Opcode != 0 {
		t.Fatal("partially accepted invalid icons")
	}
}

func TestLanguageDownloadIsPublicAndPreservesConfiguredResource(t *testing.T) {
	s := newTestServer(t)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	w := get(nativecodec.LanguagePrefix + "l")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	catalog, err := protocol.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil || catalog.Opcode != 1002 || len(catalog.Fields) != 3 {
		t.Fatal("invalid language catalog", err)
	}
	u, _ := catalog.Text(3)
	w = get(u)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), fixtureAsset(t, "en-US")) {
		t.Fatal("language download did not preserve configured resource")
	}
	if got := w.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("language content type = %q", got)
	}

	pack, err := protocol.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil || pack.Opcode != 1004 {
		t.Fatal("wrong language resource", err)
	}
	// Synthetic labels verify the configured resource reaches the HTTP client;
	// byte-exact historical verification belongs to the original-JAR harness.
	for tag, want := range map[int16]string{1: "en-US", 2: "Synthetic English fixture", 193: "mig33-test", 361: "synthetic-v1"} {
		if got, ok := pack.Text(tag); !ok || got != want {
			t.Fatalf("language field %d = %q, want %q", tag, got, want)
		}
	}
	if w = get(nativecodec.LanguagePrefix + "missing"); w.Code != 404 || w.Body.String() != "404 page not found\n" {
		t.Fatalf("unknown language response changed: %d %q", w.Code, w.Body.String())
	}

	a := login(t, s, "alice")
	r, _ := s.Handle(t.Context(), a, protocol.New(925, 56).SetText(1, "en-US"))
	if r[0].Opcode != 1 {
		t.Fatal("language selection failed")
	}
	if code, err := s.store.Language(t.Context(), "alice"); err != nil || code != "en-US" {
		t.Fatal("language not persisted")
	}

	r, _ = s.Handle(t.Context(), a, protocol.New(925, 57).SetText(1, "made-up"))
	if r[0].Opcode != 0 {
		t.Fatal("missing translation accepted")
	}
}

func TestEmoticonEntitlementAndOriginalContentFraming(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	b := login(t, s, "bob")
	if err := s.validateEmoticonMessage(t.Context(), "alice", "[smile] [heart]"); err != nil {
		t.Fatal(err)
	}
	if err := s.validateEmoticonMessage(t.Context(), "alice", "[sparkle]"); err == nil {
		t.Fatal("paid send accepted")
	}

	if _, err := s.store.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", token(), 1000); err != nil {
		t.Fatal(err)
	}

	if _, err := s.store.CreditOperation(t.Context(), "alice", "alice", "purchase", "emotes-effects", token(), 200); err != nil {
		t.Fatal(err)
	}
	if err := s.validateEmoticonMessage(t.Context(), "alice", "[buzz] [sparkle] [chime]"); err != nil {
		t.Fatal(err)
	}
	if err := s.validateEmoticonMessage(t.Context(), "alice", "[gift:rose]"); err == nil {
		t.Fatal("forged gift animation accepted")
	}
	for _, e := range catalog.Emoticons() {
		r, _ := s.Handle(t.Context(), b, protocol.New(913, 60).SetText(1, e.Token+" ").SetInt(2, 1))
		p := r[0]
		if p.Opcode != 914 || p.Transaction != 60 {
			t.Fatalf("recipient cannot render %s", e.Token)
		}
		data := bytes.NewReader(p.Fields[4])
		kind, _ := p.Byte(5)
		frames := uint32(1)
		if kind == 3 {
			if err := binary.Read(data, binary.BigEndian, &frames); err != nil {
				t.Fatal(err)
			}
		}
		for i := uint32(0); i < frames; i++ {
			imageBytes := p.Fields[4]
			if kind == 3 || kind == 4 {
				var n uint32
				if err := binary.Read(data, binary.BigEndian, &n); err != nil {
					t.Fatal(err)
				}
				imageBytes = make([]byte, n)
				if _, err := io.ReadFull(data, imageBytes); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := image.Decode(bytes.NewReader(imageBytes)); err != nil {
				t.Fatal(err)
			}
		}
		if kind == 4 {
			var n uint32
			if err := binary.Read(data, binary.BigEndian, &n); err != nil {
				t.Fatal(err)
			}
			wav := make([]byte, n)
			if _, err := io.ReadFull(data, wav); err != nil || !bytes.HasPrefix(wav, []byte("RIFF")) {
				t.Fatal("invalid audio", err)
			}
		}
		if (kind == 3 || kind == 4) && data.Len() != 0 {
			t.Fatal("unconsumed content")
		}
	}
	p := events.New(0, events.Message{Message: chat.Message{PhotoID: 1}})
	decorateEmoticonMessage(p, "[smile] [chime]")
	if triggers, _ := fixturePacket(p).Text(11); triggers != "[photo:1] [smile] [chime]" {
		t.Fatal("photo trigger lost", triggers)
	}
	_ = a
}

func TestGiftMessageSharesPurchaseTransactionAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gift.sqlite")
	s, err := newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.store.CreditOperation(t.Context(), "alice", "alice", "test-recharge", "", token(), 1000); err != nil {
		t.Fatal(err)
	}
	nonce := token()
	for i := 0; i < 2; i++ {
		if _, err = s.store.CreditOperation(t.Context(), "alice", "bob", "purchase", "rose", nonce, 100); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	msgs, err := s.store.Pending(t.Context(), "bob", 100)
	if err != nil || len(msgs) != 1 || !msgs[0].Action || !strings.Contains(msgs[0].Body, "[gift:rose]") {
		t.Fatalf("gift missing/duplicated: %+v %v", msgs, err)
	}
	packet := fixturePacket(s.messagePacket(t.Context(), msgs[0], "bob"))
	if triggers, _ := packet.Text(11); !strings.Contains(triggers, "[gift:rose]") {
		t.Fatal("gift animation trigger absent")
	}
	if balance, _ := s.store.CreditBalance(t.Context(), "alice"); balance != 900 {
		t.Fatal("purchase charged twice")
	}
}

func TestEmoticonPickerRefreshSurvivesFullSessionQueue(t *testing.T) {
	s := newTestServer(t)
	a := login(t, s, "alice")
	x := s.sessions[a]
	for len(x.push) < cap(x.push) {
		x.push <- events.New(0, events.Pong{})
	}
	s.mu.Lock()
	s.refreshEmoticonCatalog(t.Context(), "alice")
	s.mu.Unlock()
	if !x.emoticonsStale {
		t.Fatal("full queue lost required refresh")
	}
	for len(x.push) > 0 {
		<-x.push
	}
	s.mu.Lock()
	s.flushPending(t.Context(), x)
	s.mu.Unlock()
	if x.emoticonsStale {
		t.Fatal("refresh not retried")
	}
	found := false
	for len(x.push) > 0 {
		if p := fixturePacket(<-x.push); p.Opcode == 916 {
			found = true
		}
	}
	if !found {
		t.Fatal("picker refresh absent after queue drained")
	}
}
