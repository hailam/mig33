package native_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/session"
	"github.com/hailam/mig33/internal/testsupport"
)

func finishNative(t *testing.T, fixture *testsupport.Fixture, id string, packets []*protocol.Packet) {
	t.Helper()
	for _, packet := range packets {
		if fixture.Native.AllowPacket(t.Context(), id, packet) {
			fixture.Native.FinishWrite(t.Context(), []*protocol.Packet{packet}, true)
		}
	}
}

func loginNative(t *testing.T, fixture *testsupport.Fixture, username string) string {
	t.Helper()
	replies, id := fixture.Native.Handle(t.Context(), "", protocol.New(200, 10).SetText(5, username))
	if len(replies) != 1 || replies[0].Opcode != 201 {
		t.Fatalf("challenge: %+v", replies)
	}
	challenge, _ := replies[0].Text(1)
	finishNative(t, fixture, id, replies)
	replies, id = fixture.Native.Handle(t.Context(), id, protocol.New(202, 11).SetInt(1, protocol.JavaHash(challenge+username+"-demo")))
	if len(replies) != 1 || replies[0].Opcode != 203 {
		t.Fatalf("login: %+v", replies)
	}
	finishNative(t, fixture, id, replies)
	replies, _ = fixture.Native.Handle(t.Context(), id, protocol.New(400, 12))
	finishNative(t, fixture, id, replies)
	drainNative(t, fixture, id)
	return id
}

func drainNative(t *testing.T, fixture *testsupport.Fixture, id string) {
	t.Helper()
	for range 32 {
		packets := fixture.Native.Poll(t.Context(), id, 128)
		if len(packets) == 0 {
			return
		}
		finishNative(t, fixture, id, packets)
	}
	t.Fatal("native queue did not drain")
}

func sendNative(t *testing.T, fixture *testsupport.Fixture, id, recipient, body string, color *int32) {
	t.Helper()
	packet := protocol.New(500, 90).SetByte(1, 1).SetByte(3, 1).SetText(4, recipient).SetShort(6, 1).SetText(8, body)
	if color != nil {
		packet.SetInt(13, *color)
	}
	replies, _ := fixture.Native.Handle(t.Context(), id, packet)
	if len(replies) != 1 || replies[0].Opcode != 1 {
		message := ""
		if len(replies) > 0 {
			message, _ = replies[0].Text(2)
		}
		t.Fatalf("send failed: %s", message)
	}
	finishNative(t, fixture, id, replies)
}

func takeMessage(t *testing.T, fixture *testsupport.Fixture, id string) *protocol.Packet {
	t.Helper()
	packets := fixture.Native.Poll(t.Context(), id, 128)
	var message *protocol.Packet
	for _, packet := range packets {
		if packet.Opcode == 500 && message == nil {
			message = packet
			continue
		}
		finishNative(t, fixture, id, []*protocol.Packet{packet})
	}
	if message == nil {
		t.Fatal("chat message not queued")
	}
	return message
}

func TestCanceledAdmissionRetainsDurableMessageForSameSessionRetry(t *testing.T) {
	fixture := testsupport.Open(t, "http://127.0.0.1:8080", ":memory:")
	alice, bob := loginNative(t, fixture, "alice"), loginNative(t, fixture, "bob")
	sendNative(t, fixture, alice, "bob", "retry after cancellation", nil)
	message := takeMessage(t, fixture, bob)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if fixture.Native.AllowPacket(ctx, bob, message) {
		t.Fatal("canceled admission accepted message")
	}
	pending, err := fixture.Repository.Pending(t.Context(), "bob", 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("cancellation changed durable outbox: %+v %v", pending, err)
	}
	retry := takeMessage(t, fixture, bob)
	if body, _ := retry.Text(8); body != "retry after cancellation" {
		t.Fatal(body)
	}
	finishNative(t, fixture, bob, []*protocol.Packet{retry})
	pending, err = fixture.Repository.Pending(t.Context(), "bob", 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("successful write retained outbox: %+v %v", pending, err)
	}
}

func TestFailedOwnMirrorWriteRetriesWithoutChangingStoredMessage(t *testing.T) {
	fixture := testsupport.Open(t, "http://127.0.0.1:8080", ":memory:")
	composer, mirror, bob := loginNative(t, fixture, "alice"), loginNative(t, fixture, "alice"), loginNative(t, fixture, "bob")
	color := int32(0x123456)
	sendNative(t, fixture, composer, "bob", "hello [smile]", &color)
	first := takeMessage(t, fixture, mirror)
	if body, _ := first.Text(8); body != "alice: hello [smile]" {
		t.Fatal(body)
	}
	if peer, _ := first.Text(2); peer != "bob" {
		t.Fatal(peer)
	}
	if _, hasColor := first.Fields[13]; hasColor {
		t.Fatal("native action mirror retained unsupported RGB")
	}
	fixture.Native.FinishWrite(t.Context(), []*protocol.Packet{first}, false)
	retry := takeMessage(t, fixture, mirror)
	if body, _ := retry.Text(8); body != "alice: hello [smile]" {
		t.Fatal(body)
	}
	finishNative(t, fixture, mirror, []*protocol.Packet{retry})
	for _, packet := range fixture.Native.Poll(t.Context(), mirror, 128) {
		if packet.Opcode == 500 {
			t.Fatal("successful own mirror repeated")
		}
		finishNative(t, fixture, mirror, []*protocol.Packet{packet})
	}
	history, err := fixture.Repository.History(t.Context(), "alice", 1, "bob", 0, 10)
	if err != nil || len(history) != 1 || history[0].Body != "hello [smile]" || history[0].Color == nil || *history[0].Color != color {
		t.Fatalf("stored message changed: %+v %v", history, err)
	}
	drainNative(t, fixture, bob)
}

func TestBlockingRejectsAnAlreadyBufferedMessage(t *testing.T) {
	fixture := testsupport.Open(t, "http://127.0.0.1:8080", ":memory:")
	alice, bob := loginNative(t, fixture, "alice"), loginNative(t, fixture, "bob")
	sendNative(t, fixture, alice, "bob", "queued before block", nil)
	message := takeMessage(t, fixture, bob)
	replies, _ := fixture.Native.Handle(t.Context(), bob, protocol.New(417, 4).SetText(1, "alice").SetByte(2, 2))
	if len(replies) != 1 || replies[0].Opcode != 1 {
		t.Fatal("block failed")
	}
	finishNative(t, fixture, bob, replies)
	if fixture.Native.AllowPacket(t.Context(), bob, message) {
		t.Fatal("buffered message bypassed committed block")
	}
}

func TestThemeSelectionIgnoresBrowserQueueAndRejectsFullNativeQueue(t *testing.T) {
	fixture := testsupport.Open(t, "http://127.0.0.1:8080", ":memory:")
	bob := loginNative(t, fixture, "bob")
	browser, err := fixture.Core.LoginBrowser(t.Context(), session.BrowserLogin{Username: "bob", Password: "bob-demo"})
	if err != nil {
		t.Fatal(err)
	}
	item, ok := catalog.Lookup("theme-orange")
	if !ok {
		t.Fatal("theme fixture missing")
	}
	if _, err := fixture.Repository.CreditOperation(t.Context(), "bob", "bob", "test-recharge", "", strings.Repeat("1", 48), 10000); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.Repository.CreditOperation(t.Context(), "bob", "bob", "purchase", item.ID, strings.Repeat("2", 48), item.Price); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 135; i++ {
		if err := fixture.Core.ApplyTheme(t.Context(), browser.ID, browser.CSRF, item.ID); err != nil {
			t.Fatalf("theme update %d: %v", i, err)
		}
		packets := fixture.Native.Poll(t.Context(), bob, 128)
		seen := false
		for _, packet := range packets {
			if packet.Opcode == 1000 {
				seen = true
			}
		}
		if !seen {
			t.Fatal("native theme update missing")
		}
		finishNative(t, fixture, bob, packets)
	}
	if packets := fixture.Core.Poll(t.Context(), browser.ID, 128); len(packets) != 0 {
		t.Fatalf("browser accumulated %d native events", len(packets))
	}
	for range 128 {
		if err := fixture.Core.ApplyTheme(t.Context(), browser.ID, browser.CSRF, item.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := fixture.Core.ApplyTheme(t.Context(), browser.ID, browser.CSRF, "default"); !errors.Is(err, faults.ErrLimit) {
		t.Fatalf("full queue changed theme: %v", err)
	}
	if selected, err := fixture.Repository.SelectedStoreTheme(t.Context(), "bob"); err != nil || selected != item.ID {
		t.Fatalf("failed selection committed: %s %v", selected, err)
	}
	drainNative(t, fixture, bob)
}
