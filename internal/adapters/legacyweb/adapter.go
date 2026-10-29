package legacyweb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/catalog"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/community"
	"github.com/hailam/mig33/internal/media"
	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/review"
	"github.com/hailam/mig33/internal/session"
)

type Dependencies struct {
	Customization *accounts.Customization
	Logger        *slog.Logger
	Activity      *activity.Service
	Review        *review.Service
	PublicURL     string
	Accounts      *accounts.Service
	Privacy       *accounts.Privacy
	Catalog       *catalog.Service
	Commerce      *commerce.Service
	Community     *community.Service
	Media         *media.Service
	Rooms         Rooms
	Contacts      Contacts
	Chat          Chat
	Workflow      Workflow
}
type Adapter struct {
	customization *accounts.Customization
	logger        *slog.Logger
	activity      *activity.Service
	review        *review.Service
	publicURL     string
	accounts      *accounts.Service
	privacy       *accounts.Privacy
	catalog       *catalog.Service
	commerce      *commerce.Service
	community     *community.Service
	media         *media.Service
	rooms         Rooms
	contacts      Contacts
	chat          Chat
	workflow      Workflow
}

func New(d Dependencies) *Adapter {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}

	return &Adapter{
		customization: d.Customization,
		logger:        d.Logger,
		publicURL:     d.PublicURL,
		accounts:      d.Accounts,
		privacy:       d.Privacy,
		catalog:       d.Catalog,
		commerce:      d.Commerce,
		community:     d.Community,
		media:         d.Media,
		rooms:         d.Rooms,
		contacts:      d.Contacts,
		chat:          d.Chat,
		workflow:      d.Workflow,
		activity:      d.Activity,
		review:        d.Review,
	}
}

type actor struct {
	id, username, csrf     string
	authenticated, webOnly bool
	rooms                  map[string]bool
}

func actorView(view session.View, joined []string) *actor {
	x := &actor{
		id:            view.ID,
		username:      view.Username,
		csrf:          view.CSRF,
		authenticated: view.Username != "",
		webOnly:       view.WebOnly,
		rooms:         make(map[string]bool, len(joined)),
	}
	for _, room := range joined {
		x.rooms[room] = true
	}

	return x
}

func (s *Adapter) actor(ctx context.Context, id string) (*actor, error) {
	view, joined, err := s.workflow.WebSession(ctx, id)
	if err != nil {
		return nil, err
	}

	return actorView(view, joined), nil
}

func token() string {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}

	return hex.EncodeToString(raw[:])
}
func validToken(id string) bool { return session.ValidToken(id) }

const DevelopmentOTP = accounts.DevelopmentOTP

type profileView struct {
	id                    int32
	name, display, status string
	identity              accounts.Profile
}

func (s *Adapter) account(ctx context.Context, name string) *profileView {
	a, err := s.accounts.Profile(ctx, name)
	if err != nil {
		return nil
	}

	return &profileView{a.ID, a.Username, a.Display, a.Status, a}
}

func (s *Adapter) accountNames(ctx context.Context) []string {
	all, err := s.accounts.Profiles(ctx)
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(all))
	for _, a := range all {
		names = append(names, a.Username)
	}

	sort.Strings(names)
	return names
}

func (s *Adapter) pageUsers(ctx context.Context) []pages.User {
	var users []pages.User
	for _, name := range s.accountNames(ctx) {
		a := s.account(ctx, name)
		if a != nil {
			users = append(users, pages.User{Username: name, DisplayName: a.display, Status: a.status, Online: s.workflow.Presence(ctx, name) != 99})
		}
	}

	return users
}

func (s *Adapter) canViewProfile(ctx context.Context, viewer, owner string) bool {
	allowed, err := s.privacy.CanViewProfile(ctx, viewer, owner)
	return err == nil && allowed
}

func (s *Adapter) canContact(ctx context.Context, viewer, owner string) bool {
	allowed, err := s.privacy.CanContact(ctx, viewer, owner)
	return err == nil && allowed
}

func (s *Adapter) blocksIncoming(ctx context.Context, viewer, owner string) bool {
	blocked, err := s.privacy.BlocksIncoming(ctx, viewer, owner)
	return err != nil || blocked
}

func roomHasUser(users []string, name string) bool {
	for _, user := range users {
		if user == name {
			return true
		}
	}

	return false
}

func transcriptKind(value string) byte {
	switch value {
	case "private":
		return 1
	case "group":
		return 2
	case "room":
		return 3
	}

	return 0
}

func (s *Adapter) logError(ctx context.Context, format string, args ...any) {
	s.logger.ErrorContext(ctx, fmt.Sprintf(format, args...))
}
