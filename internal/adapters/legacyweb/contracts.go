// Package legacyweb translates the historical selector URLs and form dialect
// into typed capability calls and renders the original embedded-page contract.
package legacyweb

import (
	"context"
	"net/url"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/contacts"
	"github.com/hailam/mig33/internal/pages"
	"github.com/hailam/mig33/internal/review"
	"github.com/hailam/mig33/internal/rooms"
	"github.com/hailam/mig33/internal/session"
)

type Request struct {
	Method string
	URL    *url.URL
	Form   url.Values
}
type Result = pages.Result

// Workflow contains only use cases that coordinate shared realtime state with
// durable capability mutations. Implementations authorize at execution time.
type Workflow interface {
	WebSession(context.Context, string) (session.View, []string, error)
	Presence(context.Context, string) byte
	Participants(context.Context, string) []string
	RegisterIdentity(context.Context, accounts.Registration) (accounts.Profile, error)
	RecoverIdentity(context.Context, string, string, string) (accounts.Profile, error)
	UpdateProfile(context.Context, string, string, string, string) (accounts.Profile, error)
	BeginPasswordChange(context.Context, string, string, string) (accounts.IdentityChallenge, error)
	CompletePasswordChange(context.Context, string, string, string, string) error
	ChangeMobile(context.Context, string, string, string, string) (accounts.Profile, error)
	Unblock(context.Context, string, string, string) error
	JoinRoom(context.Context, string, string, string) error
	LeaveRoom(context.Context, string, string, string) error
	ModerateRoom(context.Context, string, string, string, string, bool) (string, error)
	ChangeRoom(context.Context, string, string, string, string, string) error
	Purchase(context.Context, string, string, commerce.PurchaseCommand) (commerce.PurchaseResult, error)
	ApplyTheme(context.Context, string, string, string) error
	SetAvatar(context.Context, string, string, int64) error
	PublishActivity(context.Context, string, string, activity.ServiceEntry) (activity.ServiceEntry, error)
	SendFormatted(context.Context, string, string, byte, string, string, string, string) error
	ReportTargets(context.Context, string, string, string) (string, []string, error)
	SubmitReport(context.Context, string, string, review.ReportSubmission) (review.AbuseReport, error)
}

// Rooms and Contacts are consumer-owned query/use-case ports, implemented by
// capability services. Persistence adapters are not page dependencies.
type Rooms interface {
	Room(context.Context, string) (rooms.Room, error)
	Rooms(context.Context, int16) ([]rooms.Room, error)
	SearchRooms(context.Context, string, int) ([]rooms.Room, error)
	SearchRoomsFiltered(context.Context, rooms.RoomSearchFilter, int) ([]rooms.Room, error)
	CreateRoom(context.Context, string, string, string) (rooms.Room, error)
	IsRoomModerator(context.Context, string, string) (bool, error)
	RoomModerators(context.Context, string) ([]string, error)
	RoomMuted(context.Context, string) ([]string, error)
	RoomSettings(context.Context, string) (rooms.RoomSettings, error)
	RoomBans(context.Context, string) ([]string, error)
	RoomChanges(context.Context, string) ([]rooms.RoomChange, error)
	GroupRoomControls(context.Context, string) (rooms.GroupRoomControls, error)
	RoomBotScores(context.Context, string) ([]rooms.RoomBotScore, error)
	RoomCategories(context.Context) ([]rooms.RoomCategory, error)
}
type Contacts interface {
	BlockedUsers(context.Context, string) ([]string, error)
	Contact(context.Context, string, int32) (contacts.Contact, error)
}
type Chat interface {
	Group(context.Context, string, string) (chat.Group, error)
	Groups(context.Context, string) ([]chat.Group, error)
	History(context.Context, string, byte, string, int64, int) ([]chat.Message, error)
}
