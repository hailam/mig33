package realtime

import (
	"context"
	"errors"
	"testing"

	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/protocol"
)

func TestCanceledPacketAdmissionReleasesReservationForSameSessionRetry(t *testing.T) {
	app, repo := initializedApplication(t)
	alice, bob := login(t, app, "alice"), login(t, app, "bob")
	app.Handle(t.Context(), alice, privateMessage("bob", "retry canceled operation"))
	queue := app.PushQueue(t.Context(), bob)
	var packet *protocol.Packet
	for packet == nil {
		select {
		case p := <-queue:
			if p.Opcode == 500 {
				packet = p
			}
		default:
			t.Fatal("message was not queued")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// A rejected write must do only bookkeeping, even with canceled storage.
	app.useStore(&serviceRepositoryStub{})
	if app.AllowPacket(ctx, bob, packet) {
		t.Fatal("canceled operation admitted a queued user message")
	}
	app.mu.Lock()
	if len(app.sessions[bob].inflight) != 0 || len(app.deliveries) != 0 {
		t.Error("rejected packet retained its in-memory reservation")
	}
	app.mu.Unlock()
	app.useStore(repo)
	pending, err := repo.Pending(t.Context(), "bob", 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("rejection changed the durable outbox: %+v %v", pending, err)
	}
	packets := requestPacket(t, app, bob, nil)
	if len(packets) != 1 || packets[0].Opcode != 500 {
		t.Fatalf("same session did not retry: %+v", packets)
	}
	if body, _ := packets[0].Text(8); body != "retry canceled operation" {
		t.Fatal("retried message changed")
	}
}

func TestCanceledActivityAdmissionReleasesTransientMetadata(t *testing.T) {
	app, _ := initializedApplication(t)
	id := login(t, app, "bob")
	packet := events.New(0, events.Activity{})
	app.mu.Lock()
	app.rememberActivityPacket(app.sessions[id], activity.ServiceEntry{ID: 19, Owner: "alice"}, packet)
	app.mu.Unlock()
	app.useStore(&serviceRepositoryStub{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if app.Coordinator.AllowEvent(ctx, id, packet) {
		t.Fatal("canceled operation admitted an activity packet")
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	if len(app.sessions[id].activityPackets) != 0 {
		t.Fatal("rejected activity packet retained transient metadata")
	}
}

type groupLookupRepository struct {
	Repository
	group func(context.Context, string, string) (chat.Group, error)
}

func (repo *groupLookupRepository) Group(ctx context.Context, id, user string) (chat.Group, error) {
	return repo.group(ctx, id, user)
}

func TestOwnGroupMirrorRetainsCursorAndRetryAfterFailedMembershipLookup(t *testing.T) {
	for _, pending := range []bool{false, true} {
		for name, cause := range map[string]error{
			"cancellation":                       context.Canceled,
			"cancellation_with_membership_error": errors.Join(context.Canceled, faults.ErrNotFound),
			"temporary_storage_error":            errors.New("temporary storage failure"),
		} {
			if pending {
				name += "_pending"
			}
			t.Run(name, func(t *testing.T) {
				app, repo := initializedApplication(t)
				id := login(t, app, "alice")
				if _, err := repo.CreateGroup(t.Context(), "test-group", "alice", []string{"bob"}); err != nil {
					t.Fatal(err)
				}
				message, err := repo.SaveMessage(t.Context(), chat.Message{Kind: 2, Conversation: "test-group", Sender: "alice", Body: "retry own group mirror"}, nil)
				if err != nil {
					t.Fatal(err)
				}
				x := app.sessions[id]
				if pending {
					x.ownPending[message.ID] = message
					x.ownAfter = message.ID
				}
				before := x.ownAfter
				app.useStore(&groupLookupRepository{Repository: repo, group: func(context.Context, string, string) (chat.Group, error) {
					return chat.Group{}, cause
				}})
				app.mu.Lock()
				app.flushOwnMessages(t.Context(), x)
				if x.ownAfter != before || (pending && len(x.ownPending) != 1) || len(x.inflight) != 0 {
					t.Error("failed lookup consumed an undelivered own message")
				}
				app.mu.Unlock()
				app.useStore(repo)
				packets := requestPacket(t, app, id, nil)
				found := false
				for _, p := range packets {
					if body, _ := p.Text(8); p.Opcode == 500 && body == message.Body {
						found = true
					}
				}
				if !found {
					t.Fatal("same session lost its own group mirror after a failed lookup")
				}
			})
		}
	}
}
