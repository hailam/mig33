package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/chat"
)

// Group contains current members only; historical message access is preserved
// separately in the audience saved with each message. IDs are case-sensitive.
type Group = chat.Group

// CreateGroup atomically creates a group with its creator and initial members.
// Duplicate member names are harmless. An existing group ID is never replaced.
// An invitee blocking the creator rejects the entire creation with ErrBlocked.
func (s *Store) CreateGroup(ctx context.Context, id, creator string, members []string) (Group, error) {
	if err := groupID(id); err != nil {
		return Group{}, queryError(ctx, err)
	}

	creator, err := username(creator)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}

	names, err := memberNames(append(append([]string(nil), members...), creator))
	if err != nil {
		return Group{}, queryError(ctx, err)
	}
	if len(names) > 50 {
		return Group{}, fmt.Errorf("%w: maximum 50 group members", ErrLimit)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	exists, err := queries(tx).CountPrivateGroup(ctx, id)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}
	if exists != 0 {
		return Group{}, ErrConflict
	}

	now := time.Now().UnixMilli()
	if err := queries(tx).CreatePrivateGroup(ctx, sqlc.CreatePrivateGroupParams{ID: id, Creator: creator, CreatedAt: now}); err != nil {
		return Group{}, queryError(ctx, err)
	}

	for _, name := range names {
		if name != creator {
			blocked, err := isBlocked(ctx, tx, name, creator)
			if err != nil {
				return Group{}, queryError(ctx, err)
			}
			if blocked {
				return Group{}, ErrBlocked
			}
		}
		if err := queries(tx).AddPrivateGroupMember(ctx, sqlc.AddPrivateGroupMemberParams{GroupID: id, Username: name, JoinedAt: now}); err != nil {
			return Group{}, queryError(ctx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Group{}, queryError(ctx, err)
	}

	return Group{ID: id, Creator: creator, Members: names}, nil
}

// Invite adds members only while actor is a current member. Existing members
// remain members, and inviting them again does not alter historical access.
// A new invitee blocking actor rejects the entire invitation with ErrBlocked.
func (s *Store) Invite(ctx context.Context, id, actor string, members []string) (Group, error) {
	if err := groupID(id); err != nil {
		return Group{}, queryError(ctx, err)
	}

	actor, err := username(actor)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}

	names, err := memberNames(members)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	if err := authorizeGroup(ctx, tx, id, actor); err != nil {
		return Group{}, queryError(ctx, err)
	}

	now := time.Now().UnixMilli()
	for _, name := range names {
		existing, err := queries(tx).IsPrivateGroupMember(ctx, sqlc.IsPrivateGroupMemberParams{GroupID: id, Username: name})
		if err != nil {
			return Group{}, queryError(ctx, err)
		}
		if existing {
			continue
		}

		blocked, err := isBlocked(ctx, tx, name, actor)
		if err != nil {
			return Group{}, queryError(ctx, err)
		}
		if blocked {
			return Group{}, ErrBlocked
		}
		if err := queries(tx).InvitePrivateGroupMember(ctx, sqlc.InvitePrivateGroupMemberParams{GroupID: id, Username: name, JoinedAt: now}); err != nil {
			return Group{}, queryError(ctx, err)
		}
	}

	group, err := readGroup(ctx, tx, id)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}
	if len(group.Members) > 50 {
		return Group{}, fmt.Errorf("%w: maximum 50 group members", ErrLimit)
	}

	if err := tx.Commit(); err != nil {
		return Group{}, queryError(ctx, err)
	}

	return group, nil
}

// Leave removes actor from current membership while retaining their earlier
// message audience. The returned group lists remaining members, possibly none.
func (s *Store) Leave(ctx context.Context, id, actor string) (Group, error) {
	if err := groupID(id); err != nil {
		return Group{}, queryError(ctx, err)
	}

	actor, err := username(actor)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	if err := authorizeGroup(ctx, tx, id, actor); err != nil {
		return Group{}, queryError(ctx, err)
	}
	if err := queries(tx).RemovePrivateGroupMember(ctx, sqlc.RemovePrivateGroupMemberParams{GroupID: id, Username: actor}); err != nil {
		return Group{}, queryError(

			// A queued old message would reopen the left group in the legacy client.
			// Cancellation is durable and survives a later re-invitation; history stays.
			ctx, err)
	}

	if err := queries(tx).CancelLeftGroupDeliveries(ctx, sqlc.CancelLeftGroupDeliveriesParams{
		Now:      sql.NullInt64{Int64: time.Now().UnixMilli(), Valid: true},
		Username: actor,
		GroupID:  id,
	}); err != nil {
		return Group{}, queryError(ctx, err)
	}

	group, err := readGroup(ctx, tx, id)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}

	if err := tx.Commit(); err != nil {
		return Group{}, queryError(ctx, err)
	}

	return group, nil
}

// Group returns a group only to a current member.
func (s *Store) Group(ctx context.Context, id, viewer string) (Group, error) {
	if err := groupID(id); err != nil {
		return Group{}, queryError(ctx, err)
	}

	viewer, err := username(viewer)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	if err := authorizeGroup(ctx, tx, id, viewer); err != nil {
		return Group{}, queryError(ctx, err)
	}

	group, err := readGroup(ctx, tx, id)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}

	if err := tx.Commit(); err != nil {
		return Group{}, queryError(ctx, err)
	}

	return group, nil
}

// Groups returns the groups to which viewer currently belongs, sorted by ID.
func (s *Store) Groups(ctx context.Context, viewer string) ([]Group, error) {
	viewer, err := username(viewer)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	rows, err := queries(s.db).ListPrivateGroups(ctx, viewer)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	groups := []Group{}
	for _, row := range rows {
		if len(groups) == 0 || groups[len(groups)-1].ID != row.ID {
			groups = append(groups, Group{ID: row.ID, Creator: row.Creator, Members: []string{}})
		}

		groups[len(groups)-1].Members = append(groups[len(groups)-1].Members, row.Username)
	}

	return groups, nil
}

func authorizeGroup(ctx context.Context, tx *sql.Tx, id, viewer string) error {
	member, err := queries(tx).AuthorizePrivateGroup(ctx, sqlc.AuthorizePrivateGroupParams{Viewer: viewer, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return queryError(ctx, err)
	}
	if !member {
		return ErrForbidden
	}

	return nil
}

func readGroup(ctx context.Context, tx *sql.Tx, id string) (Group, error) {
	creator, err := queries(tx).PrivateGroupCreator(ctx, id)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}

	members, err := queries(tx).ListPrivateGroupMembers(ctx, id)
	if err != nil {
		return Group{}, queryError(ctx, err)
	}

	return Group{ID: id, Creator: creator, Members: members}, nil
}

func memberNames(values []string) ([]string, error) {
	names := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		name, err := username(value)
		if err != nil {
			return nil, fmt.Errorf("member: %w", err)
		}
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}

	sort.Strings(names)
	return names, nil
}
