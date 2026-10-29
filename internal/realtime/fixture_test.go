package realtime

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"testing"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/adapters/httptransport"
	"github.com/hailam/mig33/internal/adapters/legacyweb"
	"github.com/hailam/mig33/internal/adapters/native"
	"github.com/hailam/mig33/internal/adapters/nativecodec"
	"github.com/hailam/mig33/internal/adapters/socket"
	"github.com/hailam/mig33/internal/adapters/sqlite/repositories"
	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/community"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/controllers"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/media"
	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/protocol"
	"github.com/hailam/mig33/internal/review"
	"github.com/hailam/mig33/internal/rooms"
	sessionview "github.com/hailam/mig33/internal/session"
	"github.com/hailam/mig33/internal/testassets"
)

// Repository is test-fixture composition only. No application constructor or
// capability consumes this aggregate; each service receives its own port.
type Repository interface {
	accounts.Repository
	contacts.Repository
	chat.Repository
	rooms.Repository
	commerce.Repository
	community.Repository
	media.Repository
	review.Repository
	activity.Repository
}

// Application is only the existing regression suite's facade over the final
// graph, not a retained business implementation. Its methods call real adapters.
type Application struct {
	*Coordinator
	store   Repository
	base    *url.URL
	adapter *native.Adapter
	codec   *nativecodec.Codec
	pages   *legacyweb.Adapter
	handler http.Handler
}

func testDependencies(repo Repository, logger *slog.Logger) Dependencies {
	privacy := accounts.NewPrivacy(repo, repo)
	cat := catalog.New(repo)
	room := rooms.New(repo)
	return Dependencies{
		Accounts:  accounts.NewService(repo),
		Contacts:  contacts.New(repo),
		Chat:      chat.New(repo),
		Rooms:     room,
		Catalog:   cat,
		Commerce:  commerce.New(repo, privacy, cat),
		Community: community.New(repo, privacy, room),
		Media:     media.New(repo, privacy),
		Activity:  activity.New(repo, privacy, repo),
		Review:    review.New(repo, repo, nil),
		Privacy:   privacy,
		Logger:    logger,
	}
}

func NewWithRepository(publicURL string, repo Repository, logger *slog.Logger) (*Application, error) {
	if repo == nil {
		return nil, fmt.Errorf("repository is required")
	}
	u, err := url.Parse(publicURL)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("invalid public URL")
	}
	u.Path = ""
	core, err := New(testDependencies(repo, logger))
	if err != nil {
		return nil, err
	}

	app := &Application{Coordinator: core, store: repo, base: u}
	app.bindAdapters()
	return app, nil
}

func (s *Application) bindAdapters() {
	s.pages = legacyweb.New(legacyweb.Dependencies{
		PublicURL:     s.base.String(),
		Logger:        s.logger,
		Accounts:      s.accountService,
		Customization: accounts.NewCustomization(s.store, s.mediaService),
		Privacy:       s.privacy,
		Catalog:       s.catalogService,
		Commerce:      s.commerceService,
		Community:     s.communityService,
		Media:         s.mediaService,
		Activity:      s.activityService,
		Review:        s.reviewService,
		Rooms:         s.roomsService,
		Contacts:      s.contactsService,
		Chat:          s.chatService,
		Workflow:      s.Coordinator,
	})
	s.codec = nativecodec.New(testassets.FS())
	s.adapter = native.New(s.Coordinator, s.pages, s.base.String(), s.codec)
	s.handler = httptransport.New(controllers.New(controllers.Dependencies{
		Sessions:  s.Coordinator,
		Chat:      s.Coordinator,
		Contacts:  s.Coordinator,
		Pages:     s.pages,
		Media:     s.pages,
		Operator:  s.Coordinator,
		Native:    s.adapter,
		Language:  s.codec,
		PublicURL: s.base.String(),
		Catalog:   s.Catalog,
	}))
}

func (s *Application) useStore(repo Repository) {
	s.store = repo
	d := testDependencies(repo, s.logger)
	s.accountService = d.Accounts
	s.contactsService = d.Contacts
	s.chatService = d.Chat
	s.roomsService = d.Rooms
	s.catalogService = d.Catalog
	s.commerceService = d.Commerce
	s.communityService = d.Community
	s.mediaService = d.Media
	s.activityService = d.Activity
	s.privacy = d.Privacy
	names := []string{}
	for name := range s.operators {
		names = append(names, name)
	}
	s.reviewService = review.New(repo, repo, names)
	// Keep the native adapter and its outstanding frame associations. Its core
	// pointer is unchanged; pure HTTP/page dependencies are rebound separately.
	old := s.adapter
	s.bindAdapters()
	s.adapter = old
}

func (s *Application) ConfigureOperators(names []string) error {
	return s.Coordinator.ConfigureOperators(names)
}

func newTestApp(t *testing.T, origin string) (*Application, error) {
	return newTestOpen(t, origin, ":memory:")
}

func newTestOpen(t *testing.T, origin, path string) (*Application, error) {
	t.Helper()
	repo, err := repositories.Open(t.Context(), path)
	if err != nil {
		return nil, err
	}
	app, err := NewWithRepository(origin, repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		repo.Close()
		return nil, err
	}
	if err := app.Initialize(t.Context()); err != nil {
		app.Close()
		repo.Close()
		return nil, err
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	for {
		app.mu.Lock()
		running := app.running
		app.mu.Unlock()
		if running {
			break
		}
		select {
		case err := <-done:
			cancel()
			app.Close()
			repo.Close()
			return nil, err
		default:
			runtime.Gosched()
		}
	}

	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil && err != context.Canceled {
			t.Error(err)
		}
		if err := app.Close(); err != nil {
			t.Error(err)
		}
		if err := repo.Close(); err != nil {
			t.Error(err)
		}
	})
	return app, nil
}

func (s *Application) Handle(ctx context.Context, id string, p *protocol.Packet) ([]*protocol.Packet, string) {
	return s.adapter.Handle(ctx, id, p)
}

func (s *Application) AllowPacket(ctx context.Context, id string, p *protocol.Packet) bool {
	return s.adapter.AllowPacket(ctx, id, p)
}

func (s *Application) FinishWrite(ctx context.Context, p []*protocol.Packet, written bool) {
	s.adapter.FinishWrite(ctx, p, written)
}

func (s *Application) DropSession(ctx context.Context, id string) { s.adapter.DropSession(ctx, id) }

func (s *Application) PushQueue(ctx context.Context, id string) <-chan *protocol.Packet {
	out := s.adapter.Poll(ctx, id, 128)
	queue := make(chan *protocol.Packet, len(out))
	for _, p := range out {
		queue <- p
	}
	return queue
}

func (s *Application) Poll(ctx context.Context, id string, limit int) []*protocol.Packet {
	return s.adapter.Poll(ctx, id, limit)
}

func (s *Application) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Application) ServeTCP(ctx context.Context, l net.Listener) error {
	return socket.New(s.adapter).Serve(ctx, l)
}

func (s *Application) PublicURL() string { return s.base.String() }

func (s *Application) Catalog(ctx context.Context) (catalog.CatalogSnapshot, error) {
	return catalog.Snapshot(ctx, s.base.String())
}

func (s *Application) FeaturePage(ctx context.Context, id string, p legacyweb.Request) (pages.Result, error) {
	return s.pages.FeaturePage(ctx, id, p)
}

func (s *Application) Media(ctx context.Context, p legacyweb.MediaRequest) (legacyweb.MediaResult, error) {
	return s.pages.Media(ctx, p)
}

func (s *Application) BrowserPage(ctx context.Context, id string, p legacyweb.Request) (pages.Result, error) {
	return s.pages.BrowserPage(ctx, id, p)
}

func (s *Application) OperatorDashboard(ctx context.Context, p sessionview.OperatorDashboardRequest) (pages.Result, error) {
	return s.pages.OperatorDashboard(ctx, p)
}

func (s *Application) LanguageResource(ctx context.Context, name string) (nativecodec.Resource, error) {
	return s.codec.LanguageResource(ctx, s.base.String(), name)
}

func fixturePacket(e *events.Event) *protocol.Packet {
	return nativecodec.New(testassets.FS()).Encode(context.Background(), e, "http://local")
}

func fixtureEvent(p *protocol.Packet) *events.Event {
	switch p.Opcode {
	case 3:
		return events.New(p.Transaction, events.Pong{})
	case 1:
		return events.Ack(p.Transaction)
	case 750:
		id, _ := p.Text(1)
		peer, _ := p.Text(2)
		return events.New(p.Transaction, events.GroupCreated{ID: id, Peer: peer})
	case 500:
		kind, _ := p.Byte(3)
		target, _ := p.Text(4)
		body, _ := p.Text(8)
		sender, _ := p.Text(2)
		return events.New(p.Transaction, events.Message{Message: chat.Message{Kind: kind, Conversation: target, Body: body, Sender: sender}})
	}
	panic(fmt.Sprintf("unsupported fixture event %d", p.Opcode))
}

const DevelopmentOTP = accounts.DevelopmentOTP

var (
	verifyPassword   = accounts.VerifyPassword
	passwordIdentity = accounts.PasswordIdentity
	legacyProof      = accounts.LegacyProof
)

func (s *Application) identityChallenge(ctx context.Context, purpose, user, mobile string) (accounts.IdentityChallenge, error) {
	return s.accountService.Challenge(ctx, purpose, user, mobile)
}
