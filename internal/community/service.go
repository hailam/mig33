package community

import (
	"context"
	"strings"

	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/rooms"
)

type Accounts interface {
	Exists(context.Context, string) (bool, error)
}

type RoomReader interface {
	Room(context.Context, string) (rooms.Room, error)
	SearchRooms(context.Context, string, int) ([]rooms.Room, error)
}

type Service struct {
	repo     Repository
	accounts Accounts
	rooms    RoomReader
}

func New(repo Repository, accounts Accounts, rooms RoomReader) *Service {
	return &Service{repo: repo, accounts: accounts, rooms: rooms}
}

type View struct {
	Community Community
	Role      string
	CanRead   bool
}

type MembershipCommand struct {
	ID                       int64
	Actor, Target, Operation string
}

type PostCommand struct {
	ID, Parent         int64
	Actor, Body, Nonce string
}

func (s *Service) Create(ctx context.Context, owner, name, description string, private bool) (Community, error) {
	name, err := rooms.NormalizeRoomName(name)
	if err != nil {
		return Community{}, err
	}
	return s.repo.CreateCommunity(ctx, owner, name, description, private)
}

func (s *Service) List(ctx context.Context, viewer string, mine bool) ([]Community, error) {
	return s.repo.Communities(ctx, viewer, mine)
}

func (s *Service) View(ctx context.Context, viewer string, id int64) (View, error) {
	group, err := s.repo.Community(ctx, id)
	if err != nil {
		return View{}, err
	}

	role, err := s.repo.CommunityRole(ctx, id, viewer)
	if err != nil {
		return View{}, err
	}
	return View{Community: group, Role: role, CanRead: !group.Private || role != "" && role != "pending"}, nil
}

func (s *Service) Membership(ctx context.Context, command MembershipCommand) error {
	switch command.Operation {
	case "join", "leave":
		command.Target = command.Actor
	case "approve", "remove", "promote", "demote", "add":
		command.Target = strings.ToLower(strings.TrimSpace(command.Target))
		exists, err := s.accounts.Exists(ctx, command.Target)
		if err != nil {
			return err
		}
		if !exists {
			return faults.ErrNotFound
		}
	default:
		return faults.ErrInvalid
	}

	return s.repo.CommunityMembership(ctx, command.ID, command.Actor, command.Target, command.Operation)
}

func (s *Service) Members(ctx context.Context, actor string, id int64) ([]CommunityMember, error) {
	return s.repo.CommunityMembers(ctx, id, actor)
}

func (s *Service) LinkRoom(ctx context.Context, actor string, id int64, room string, remove bool) error {
	return s.repo.LinkCommunityRoom(ctx, id, actor, room, remove)
}

func (s *Service) Rooms(ctx context.Context, actor string, id int64) ([]rooms.Room, error) {
	return s.repo.CommunityRooms(ctx, id, actor)
}

func (s *Service) Posts(ctx context.Context, actor string, id int64) ([]CommunityPost, error) {
	return s.repo.CommunityPosts(ctx, id, actor)
}

func (s *Service) AddPost(ctx context.Context, command PostCommand) (int64, error) {
	if command.Parent < 0 {
		return 0, faults.ErrInvalid
	}
	return s.repo.AddCommunityPost(ctx, command.ID, command.Parent, command.Actor, command.Body, command.Nonce)
}

// LinkableGroups validates room ownership before exposing eligible community
// choices. LinkRoom still performs the authoritative checks in its transaction.
func (s *Service) LinkableGroups(ctx context.Context, actor, name string) (rooms.Room, []Community, error) {
	room, err := s.rooms.Room(ctx, name)
	if err != nil {
		return rooms.Room{}, nil, err
	}
	if room.Owner != actor {
		return rooms.Room{}, nil, faults.ErrForbidden
	}

	groups, err := s.repo.Communities(ctx, actor, true)
	if err != nil {
		return rooms.Room{}, nil, err
	}

	eligible := make([]Community, 0, len(groups))
	for _, group := range groups {
		role, err := s.repo.CommunityRole(ctx, group.ID, actor)
		if err != nil {
			return rooms.Room{}, nil, err
		}
		if role == "owner" || role == "moderator" {
			eligible = append(eligible, group)
		}
	}
	return room, eligible, nil
}

func (s *Service) LinkableRooms(ctx context.Context, actor string, id int64) ([]rooms.Room, error) {
	role, err := s.repo.CommunityRole(ctx, id, actor)
	if err != nil {
		return nil, err
	}
	if role != "owner" && role != "moderator" {
		return nil, faults.ErrForbidden
	}

	all, err := s.rooms.SearchRooms(ctx, "", 100)
	if err != nil {
		return nil, err
	}

	owned := make([]rooms.Room, 0, len(all))
	for _, room := range all {
		if room.Owner == actor {
			owned = append(owned, room)
		}
	}
	return owned, nil
}
