package native_test

import (
	"testing"

	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/testsupport"
)

func TestClosingLoggedOutConnectionPreservesAnotherLogoutAcknowledgment(t *testing.T) {
	fixture := testsupport.Open(t, "http://127.0.0.1:8080", ":memory:")
	alice := loginNative(t, fixture, "alice")
	bob := loginNative(t, fixture, "bob")
	// Both responses reset their transport's current session ID. Bob finishes
	// first and closes while Alice's acknowledgment still awaits its write.
	aliceReplies, aliceID := fixture.Native.Handle(t.Context(), alice, protocol.New(300, 81))
	bobReplies, bobID := fixture.Native.Handle(t.Context(), bob, protocol.New(300, 82))
	if aliceID != "" || bobID != "" || len(aliceReplies) != 1 || len(bobReplies) != 1 {
		t.Fatal("logout did not clear session and return one acknowledgment")
	}
	finishNative(t, fixture, bobID, bobReplies)
	fixture.Native.DropSession(t.Context(), bobID)
	ack := aliceReplies[0]
	if ack.Opcode != 1 || ack.Transaction != 81 || !fixture.Native.AllowPacket(t.Context(), aliceID, ack) {
		t.Fatal("closing another logged-out connection discarded Alice's logout acknowledgment")
	}
	fixture.Native.FinishWrite(t.Context(), aliceReplies, true)
}

func TestLogoutResponseLivesUntilItsTransportFinishes(t *testing.T) {
	fixture := testsupport.Open(t, "http://127.0.0.1:8080", ":memory:")
	alice := loginNative(t, fixture, "alice")
	bob := loginNative(t, fixture, "bob")
	aliceReplies, _ := fixture.Native.Handle(t.Context(), alice, protocol.New(300, 91))
	bobReplies, _ := fixture.Native.Handle(t.Context(), bob, protocol.New(300, 92))
	fixture.Native.DropSession(t.Context(), alice)
	if !fixture.Native.AllowPacket(t.Context(), "", aliceReplies[0]) {
		t.Fatal("session cleanup discarded a committed logout acknowledgment")
	}
	if !fixture.Native.AllowPacket(t.Context(), "", bobReplies[0]) {
		t.Fatal("session cleanup discarded another session's outstanding response")
	}
	fixture.Native.FinishWrite(t.Context(), aliceReplies, true)
	fixture.Native.FinishWrite(t.Context(), bobReplies, true)
	if fixture.Native.AllowPacket(t.Context(), "", aliceReplies[0]) {
		t.Fatal("finished transport response retained its association")
	}
}

func TestConnectionWithoutSessionCannotDiscardAnonymousResponse(t *testing.T) {
	fixture := testsupport.Open(t, "http://127.0.0.1:8080", ":memory:")
	replies, _ := fixture.Native.Handle(t.Context(), "", protocol.New(7, 101))
	fixture.Native.DropSession(t.Context(), "")
	if len(replies) != 1 || !fixture.Native.AllowPacket(t.Context(), "", replies[0]) {
		t.Fatal("a connection without a session discarded an unrelated public response")
	}
	fixture.Native.FinishWrite(t.Context(), replies, true)
}
