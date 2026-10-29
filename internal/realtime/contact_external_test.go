package realtime

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/protocol"
)

func TestPhoneAndExternalContactsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "external.sqlite")
	s, err := newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if s != nil {
			s.Close()
		}
	}()
	h := loginHailam(t, s)
	phone := protocol.New(405, 100).SetText(8, "+9607654321").SetText(3, "Phone friend").SetInt(2, 1)
	r := contactRequest(t, s, h, phone, 402)
	phoneID, _ := r[0].Int(1)
	if _, present := r[0].Fields[12]; present {
		t.Fatal("phone contact must omit mig33 branch")
	}
	if mobile, _ := r[0].Text(8); mobile != "+9607654321" || phoneID <= 5 {
		t.Fatal("phone contact identity")
	}
	r = contactRequest(t, s, h, protocol.New(405, 101).SetText(8, "+9607654321").SetText(3, "Must not replace"), 402)
	if id, _ := r[0].Int(1); id != phoneID {
		t.Fatal("duplicate phone allocated new row")
	}
	if name, _ := r[0].Text(3); name != "Phone friend" {
		t.Fatal("duplicate phone replaced metadata")
	}
	r = contactRequest(t, s, h, protocol.New(405, 102).SetText(30, "friend@example.test").SetByte(31, 2).SetInt(2, 1), 402)
	imID, _ := r[0].Int(1)
	if imID == phoneID {
		t.Fatal("phone and IM IDs collide")
	}
	if _, present := r[0].Fields[12]; present {
		t.Fatal("external contact must omit mig33 branch")
	}
	if !reflect.DeepEqual(r[0].Fields[31], []byte{2}) || !reflect.DeepEqual(r[0].Fields[32], []byte{99}) {
		t.Fatal("external service/presence shape")
	}
	if names, _ := r[0].Text(33); names != "friend@example.test" {
		t.Fatal("external username output")
	}
	r = contactRequest(t, s, h, protocol.New(407, 103).SetInt(1, imID).SetText(30, "yahoo.friend\x00msn.friend").SetBytes(31, []byte{4, 2}).SetText(3, "Two accounts"), 402)
	if !reflect.DeepEqual(r[0].Fields[31], []byte{2, 4}) || !reflect.DeepEqual(r[0].Fields[32], []byte{99, 99}) {
		t.Fatal("external arrays not canonical and aligned")
	}
	if names, _ := r[0].Text(33); names != "msn.friend\x00yahoo.friend" {
		t.Fatal("external names not aligned")
	}
	for _, p := range []*protocol.Packet{
		protocol.New(405, 104).SetText(30, "unknown").SetByte(31, 5),
		protocol.New(405, 105).SetText(30, "first\x00second").SetByte(31, 2),
		protocol.New(407, 106).SetInt(1, imID).SetText(30, "one").SetBytes(31, []byte{2, 4}),
		protocol.New(407, 107).SetInt(1, imID).SetText(30, "one\x00two").SetBytes(31, []byte{2, 2}),
		protocol.New(405, 108).SetText(12, "bob").SetText(30, "external").SetByte(31, 2),
		protocol.New(405, 109).SetText(8, "555-NO"),
	} {
		contactRequest(t, s, h, p, 0)
	}
	before, _ := s.store.Contacts(t.Context(), "hailam")
	if len(before.Contacts) != 2 {
		t.Fatal("rejected or duplicate contacts persisted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = nil
	s, err = newTestOpen(t, "http://127.0.0.1:8080", path)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := s.store.Contacts(t.Context(), "hailam")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("external roster not durable")
	}
	h = loginHailam(t, s)
	contactRequest(t, s, h, protocol.New(407, 110).SetInt(1, phoneID).SetText(8, "+9607000001"), 402)
	other := login(t, s, "bob")
	contactRequest(t, s, other, protocol.New(407, 111).SetInt(1, imID).SetText(3, "forged"), 0)
	contactRequest(t, s, h, protocol.New(406, 112).SetInt(1, imID), 1)
	if _, err := s.store.Contact(t.Context(), "hailam", imID); err == nil {
		t.Fatal("external delete failed")
	}
}

func TestContactsSnapshotConvergesGroupsAndIsPrivate(t *testing.T) {
	s := newTestServer(t)
	h := loginHailam(t, s)
	if r := operatorRequest(s, "GET", "/api/contacts", "", nil); r.Code != 401 {
		t.Fatal("anonymous roster access")
	}
	contactRequest(t, s, h, protocol.New(408, 100).SetText(2, "Temporary"), 401)
	contactRequest(t, s, h, protocol.New(410, 101).SetInt(1, 1).SetText(2, "Renamed"), 1)
	contactRequest(t, s, h, protocol.New(409, 102).SetInt(1, 2), 1)
	contactRequest(t, s, h, protocol.New(405, 103).SetText(8, "+9607654321"), 402)
	r := operatorRequest(s, "GET", "/api/contacts", h, nil)
	if r.Code != 200 || !strings.Contains(r.Body.String(), "Renamed") || strings.Contains(r.Body.String(), "Temporary") {
		t.Fatalf("authoritative groups: %d %s", r.Code, r.Body.String())
	}
	var body struct {
		Revision int32 `json:"revision"`
		Groups   []struct {
			ID   int32  `json:"id"`
			Name string `json:"name"`
		} `json:"groups"`
		Contacts []struct {
			Kind     string `json:"kind"`
			Username string `json:"username"`
			Mobile   string `json:"mobile"`
		} `json:"contacts"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Revision <= 4 || len(body.Groups) != 1 || len(body.Contacts) != 1 || body.Contacts[0].Kind != "phone" || body.Contacts[0].Username != "" {
		t.Fatalf("snapshot shape: %+v", body)
	}
	b := login(t, s, "bob")
	r = operatorRequest(s, "GET", "/api/contacts", b, nil)
	if r.Code != 200 || strings.Contains(r.Body.String(), "+9607654321") {
		t.Fatal("another account's private phone leaked")
	}
}
