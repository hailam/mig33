// Package realtime owns sessions, live participation and ordered delivery.
// Capability services own durable business rules; adapters own all presentation.
package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/community"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/media"
	"github.com/hailam/mig33/internal/review"
	"github.com/hailam/mig33/internal/rooms"
)

const (
	sessionTTL   = 2 * time.Hour
	challengeTTL = time.Minute
)

type account struct {
	id                    int32
	name, display, status string
	identity              accounts.Identity
	deliveryGate          sync.RWMutex
}
type session struct {
	id, username, challenge, csrf      string
	otpAttempts                        int
	created, touched                   time.Time
	authenticated                      bool
	presence                           byte
	push                               chan *events.Event
	ready, webOnly, groupRestoreLoaded bool
	pendingGroups                      []string
	inflight                           map[int64]*events.Event
	rooms, knownFavorites              map[string]bool
	rosterStale, activities            bool
	activityPackets                    map[events.Handle]activityDelivery
	activitySeen                       map[int64]bool
	responseGate                       sync.Mutex
	ownAfter                           int64
	ownSkipped                         map[int64]bool
	ownPending                         map[int64]chat.Message
	emoticonsStale                     bool
}
type delivery struct {
	kind                       byte
	target                     string
	sessionID, username, actor string
	messageID                  int64
	canceled, ownMirror        bool
}

// Dependencies contains concrete capability services, not a repository aggregate.
// Constructors do no I/O; Initialize and Run use their caller's contexts.
type Dependencies struct {
	Community *community.Service
	Accounts  *accounts.Service
	Contacts  *contacts.Service
	Chat      *chat.Service
	Rooms     *rooms.Service
	Catalog   *catalog.Service
	Commerce  *commerce.Service
	Media     *media.Service
	Activity  *activity.Service
	Review    *review.Service
	Privacy   *accounts.Privacy
	Logger    *slog.Logger
}

type Coordinator struct {
	communityService             *community.Service
	mu                           sync.Mutex
	accounts                     map[string]*account
	sessions                     map[string]*session
	deliveries                   map[events.Handle]delivery
	nextDelivery                 events.Handle
	kickVotes                    map[string]*kickVote
	voteCooldowns                map[voteInitiator]time.Time
	operators                    map[string]bool
	accountService               *accounts.Service
	contactsService              *contacts.Service
	chatService                  *chat.Service
	roomsService                 *rooms.Service
	catalogService               *catalog.Service
	commerceService              *commerce.Service
	mediaService                 *media.Service
	activityService              *activity.Service
	reviewService                *review.Service
	privacy                      *accounts.Privacy
	logger                       *slog.Logger
	closed, initialized, running bool
	maintenance, stop            chan struct{}
}

func New(d Dependencies) (*Coordinator, error) {
	if d.Community == nil || d.Accounts == nil || d.Contacts == nil ||
		d.Chat == nil || d.Rooms == nil || d.Catalog == nil ||
		d.Commerce == nil || d.Media == nil || d.Activity == nil ||
		d.Review == nil || d.Privacy == nil || d.Logger == nil {
		return nil, fmt.Errorf("all realtime dependencies are required")
	}

	return &Coordinator{
		communityService: d.Community,
		accountService:   d.Accounts,
		contactsService:  d.Contacts,
		chatService:      d.Chat,
		roomsService:     d.Rooms,
		catalogService:   d.Catalog,
		commerceService:  d.Commerce,
		mediaService:     d.Media,
		activityService:  d.Activity,
		reviewService:    d.Review,
		privacy:          d.Privacy,
		logger:           d.Logger,
		accounts:         map[string]*account{},
		sessions:         map[string]*session{},
		deliveries:       map[events.Handle]delivery{},
		kickVotes:        map[string]*kickVote{},
		voteCooldowns:    map[voteInitiator]time.Time{},
		maintenance:      make(chan struct{}, 1),
		stop:             make(chan struct{}),
	}, nil
}

func (s *Coordinator) Initialize(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if s.closed {
		return fmt.Errorf("application is closed")
	}
	if s.initialized {
		return nil
	}

	all, err := s.accountService.Initialize(ctx)
	if err != nil {
		return err
	}
	for _, a := range all {
		s.cacheIdentity(a)
	}
	if err := s.initializeContacts(ctx); err != nil {
		return err
	}

	s.initialized = true
	return nil
}

func (s *Coordinator) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	close(s.stop)
	for room := range s.kickVotes {
		s.clearKickVote(room)
	}
	return nil
}

func (s *Coordinator) logError(ctx context.Context, format string, args ...any) {
	s.logger.ErrorContext(ctx, fmt.Sprintf(format, args...))
}

func token() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func failure(p *commands.Request, message string) *events.Event {
	return events.Failure(p.Correlation, message)
}

func (s *Coordinator) reserveEvent(e *events.Event) {
	if e.Handle == 0 {
		s.nextDelivery++
		e.Handle = s.nextDelivery
	}
}

func (s *Coordinator) reserve(e *events.Event, d delivery) {
	s.reserveEvent(e)
	s.deliveries[e.Handle] = d
}

func (s *Coordinator) active(ctx context.Context, id string) *session {
	if ctx.Err() != nil {
		return nil
	}
	x := s.sessions[id]
	if x == nil {
		return nil
	}
	if time.Since(x.touched) > sessionTTL || (!x.authenticated && time.Since(x.created) > challengeTTL) {
		s.removeSession(ctx, id)
		return nil
	}
	return x
}

// Execute authorizes and serializes a typed command. Native framing and JSON
// decoding are performed independently by their adapters.
func (s *Coordinator) Execute(ctx context.Context, id string, p *commands.Request) ([]*events.Event, string) {
	if err := ctx.Err(); err != nil {
		return []*events.Event{failure(p, err.Error())}, id
	}

	if _, block := p.Input.(commands.Block); block {
		if gate := s.deliveryGate(id); gate != nil {
			gate.Lock()
			defer gate.Unlock()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return []*events.Event{failure(p, err.Error())}, id
	}

	if _, ok := p.Input.(commands.Ping); ok {
		if x := s.active(ctx, id); x != nil {
			x.touched = time.Now()
		}
		return []*events.Event{events.New(p.Correlation, events.Pong{})}, id
	}
	if replies, handled := s.identityPublicCommand(ctx, p); handled {
		return replies, id
	}

	if input, ok := p.Input.(commands.Login); ok {
		username, valid := input.Username.Get()
		if !valid || username == "" || len(username) > 128 {
			return []*events.Event{failure(p, "Username is required")}, id
		}

		for key := range s.sessions {
			s.active(ctx, key)
		}
		if len(s.sessions) >= 256 {
			return []*events.Event{failure(p, "Too many sessions")}, id
		}
		if id != "" {
			s.removeSession(ctx, id)
		}

		now := time.Now()
		x := &session{
			id:        token(),
			username:  strings.ToLower(username),
			challenge: token(),
			csrf:      token(),
			created:   now,
			touched:   now,
			presence:  1,
			push:      make(chan *events.Event, 128),
			inflight:  map[int64]*events.Event{},
			rooms:     map[string]bool{},
		}
		if value, ok := input.Presence.Get(); ok && validPresence(value) {
			x.presence = value
		}
		s.sessions[x.id] = x
		return []*events.Event{events.New(p.Correlation, events.Challenge{Challenge: x.challenge, SessionID: x.id})}, x.id
	}

	x := s.active(ctx, id)
	if x == nil {
		return []*events.Event{failure(p, "Session missing or expired; log in again")}, ""
	}
	x.touched = time.Now()

	if input, ok := p.Input.(commands.Authenticate); ok {
		proof, valid := input.Proof.Get()
		a := s.accounts[x.username]
		if x.authenticated || !valid || a == nil || proof != accounts.LegacyProof(a.identity, x.challenge) {
			s.removeSession(ctx, id)
			return []*events.Event{failure(p, "Login failed")}, ""
		}

		roster, err := s.contactsService.Contacts(ctx, x.username)
		if err != nil {
			s.removeSession(ctx, id)
			return []*events.Event{failure(p, "Contacts could not be loaded; try again")}, ""
		}

		latest, err := s.chatService.LatestMessageID(ctx)
		if err != nil {
			s.removeSession(ctx, id)
			return []*events.Event{failure(p, "Chat state could not be loaded; try again")}, ""
		}

		x.ownAfter = latest
		x.authenticated = true
		x.challenge = ""
		s.broadcastPresence(ctx, x.username)
		login := events.New(p.Correlation, events.Authenticated{Mobile: a.identity.Mobile, Activated: a.identity.Activated, RosterRevision: roster.Revision, Status: a.status})
		if err := s.restoreStoreTheme(ctx, x); err != nil {
			s.removeSession(ctx, id)
			return []*events.Event{failure(p, "Saved theme could not be loaded; try again")}, ""
		}
		return []*events.Event{login}, id
	}

	if !x.authenticated {
		return []*events.Event{failure(p, "Complete login first")}, id
	}

	ack := events.Ack(p.Correlation)
	switch input := p.Input.(type) {
	case commands.Activate, commands.SetMobile:
		return []*events.Event{s.identityCommand(ctx, x, p)}, id
	case commands.Logout:
		s.removeSession(ctx, id)
		return []*events.Event{ack}, ""
	case commands.Contact:
		if input.Operation == commands.ContactSnapshot || input.Operation == commands.ContactChanges {
			return s.contactSync(ctx, x, p), id
		}
		return s.contactOperation(ctx, x, p), id
	case commands.SendMessage:
		return []*events.Event{s.message(ctx, x, p)}, id
	case commands.UploadPhoto:
		return []*events.Event{s.uploadPhoto(ctx, x, p)}, id
	case commands.MediaToken:
		return []*events.Event{s.mediaToken(ctx, x, p)}, id
	case commands.Block:
		return []*events.Event{s.blockUser(ctx, x, p)}, id
	case commands.Group:
		return s.groupOperation(ctx, x, p), id
	case commands.SetPresence:
		value, ok := input.Presence.Get()
		if !ok || !validPresence(value) {
			return []*events.Event{failure(p, "Invalid presence")}, id
		}
		x.presence = value
		s.broadcastPresence(ctx, x.username)
		return []*events.Event{ack}, id
	case commands.SetStatus:
		value, ok := input.Status.Get()
		if !ok || len(value) > 512 {
			return []*events.Event{failure(p, "Invalid status")}, id
		}
		a, err := s.accountService.UpdateProfile(ctx, s.accounts[x.username].identity, "status", value)
		if err != nil {
			return []*events.Event{failure(p, "Status could not be saved")}, id
		}
		s.cacheIdentity(a)
		s.broadcastContactStatus(ctx, x.username)
		return []*events.Event{ack, events.New(0, events.Status{Text: value})}, id
	case commands.Room:
		return s.roomOperation(ctx, x, p), id
	case commands.SubscribeActivity:
		return s.subscribeActivity(ctx, x, p), id
	case commands.Telemetry:
		return []*events.Event{ack}, id
	case commands.Unknown:
		return []*events.Event{failure(p, input.Description)}, id
	default:
		return s.nativeServicePackets(ctx, x, p), id
	}
}

func validPresence(p byte) bool { return p == 1 || p == 3 || p == 4 || p == 99 }

func (s *Coordinator) presence(username string) byte {
	value := byte(99)
	for _, x := range s.sessions {
		if x.authenticated && x.username == username && time.Since(x.touched) < sessionTTL && x.presence != 99 && x.presence < value {
			value = x.presence
		}
	}
	return value
}

func (s *Coordinator) ConfigureOperators(names []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := map[string]bool{}
	for _, raw := range names {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if s.accounts[name] == nil {
			return fmt.Errorf("unknown operator account %q", name)
		}
		next[name] = true
	}
	s.operators = next
	s.reviewService.ConfigureOperators(names)
	return nil
}

func (s *Coordinator) SetRoomModerator(ctx context.Context, room, username string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	username = strings.ToLower(username)
	if s.accounts[username] == nil {
		return fmt.Errorf("unknown account %q", username)
	}
	if err := s.roomsService.SetRoomModerator(ctx, room, username, enabled); err != nil {
		return err
	}
	if enabled {
		if definition, err := s.roomsService.Room(ctx, room); err == nil {
			if vote := s.kickVotes[definition.Name]; vote != nil && vote.Target == username {
				s.cancelKickVote(ctx, definition.Name, "the target is now a moderator")
			}
		}
	}
	return nil
}

func (s *Coordinator) DropSession(ctx context.Context, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.removeSession(ctx, id)
}
