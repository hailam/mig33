package native_test

import (
	"testing"
	"time"

	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/testsupport"
)

func TestSharedSessionSocketCleanupPreservesCommittedHTTPAcknowledgment(t *testing.T) {
	fixture := testsupport.Open(t, "http://127.0.0.1:8080", ":memory:")
	id := loginNative(t, fixture, "alice")
	// The HTTP gateway holds this gate from command execution through writing.
	// A socket using the same SID can close independently during that interval.
	finishResponse := fixture.Native.BeginResponse(id)
	defer finishResponse()
	replies, currentID := fixture.Native.Handle(t.Context(), id, protocol.New(601, 111).SetText(1, "committed over HTTP"))
	if currentID != id || len(replies) != 2 || replies[0].Opcode != 1 {
		t.Fatal("status command did not commit and return its acknowledgment")
	}
	identity, err := fixture.Repository.Identity(t.Context(), "alice")
	if err != nil || identity.Status != "committed over HTTP" {
		t.Fatalf("status command did not commit: %+v %v", identity, err)
	}
	closed := make(chan struct{})
	go func() {
		fixture.Native.DropSession(t.Context(), id)
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("socket cleanup unexpectedly waited for the HTTP response gate")
	}
	if !fixture.Native.AllowPacket(t.Context(), currentID, replies[0]) {
		t.Fatal("same-SID socket cleanup discarded the HTTP acknowledgment for an already committed command")
	}
	fixture.Native.FinishWrite(t.Context(), replies, true)
}

func TestSharedSessionCleanupStillRejectsBufferedChat(t *testing.T) {
	fixture := testsupport.Open(t, "http://127.0.0.1:8080", ":memory:")
	alice, bob := loginNative(t, fixture, "alice"), loginNative(t, fixture, "bob")
	sendNative(t, fixture, alice, "bob", "keep durable after socket close", nil)
	message := takeMessage(t, fixture, bob)
	fixture.Native.DropSession(t.Context(), bob)
	if fixture.Native.AllowPacket(t.Context(), bob, message) {
		t.Fatal("buffered chat was admitted after the receiving session closed")
	}
	pending, err := fixture.Repository.Pending(t.Context(), "bob", 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("session cleanup discarded the durable message: %+v %v", pending, err)
	}
}
