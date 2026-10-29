package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hailam/mig33/internal/adapters/sqlite/sqlc"
	"github.com/hailam/mig33/internal/community"
)

// Community is a persistent social group, separate from a private group chat.
type (
	Community       = community.Community
	CommunityMember = community.CommunityMember
	CommunityPost   = community.CommunityPost
)

func communityModel(row sqlc.GetCommunityRow) Community {
	return Community{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Owner:       row.Owner,
		Private:     row.Private != 0,
		Members:     int(row.Members),
	}
}

func (s *Store) Community(ctx context.Context, id int64) (Community, error) {
	row, err := queries(s.db).GetCommunity(ctx, id)
	return communityModel(row), queryError(ctx, err)
}

func (s *Store) Communities(ctx context.Context, viewer string, mine bool) ([]Community, error) {
	rows, err := queries(s.db).ListCommunities(ctx, sqlc.ListCommunitiesParams{Mine: boolInt64(mine), Viewer: viewer})
	if err != nil {
		return nil, queryError(ctx, err)
	}

	var out []Community
	for _, row := range rows {
		out = append(out, communityModel(sqlc.GetCommunityRow(row)))
	}

	return out, nil
}

func (s *Store) CommunityRole(ctx context.Context, id int64, user string) (string, error) {
	role, err := queries(s.db).GetCommunityRole(ctx, sqlc.GetCommunityRoleParams{GroupID: id, Username: user})
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}

	return role, queryError(ctx, err)
}

func validCommunityText(text string, limit int) bool {
	return len(text) <= limit && utf8.ValidString(text) && !strings.ContainsRune(text, 0)
}

func (s *Store) CreateCommunity(ctx context.Context, owner, name, description string, private bool) (Community, error) {
	owner, err := username(owner)
	if err != nil {
		return Community{}, queryError(ctx, err)
	}

	name, err = NormalizeRoomName(name)
	if err != nil || !validCommunityText(description, 1000) {
		return Community{}, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Community{}, queryError(ctx, err)
	}
	defer tx.Rollback()

	record, err := queries(tx).GetCommunityByName(ctx, name)
	g := communityModel(sqlc.GetCommunityRow(record))
	err = queryError(ctx, err)
	if err == nil {
		if g.Owner != owner || g.Description != description || g.Private != private {
			return Community{}, ErrConflict
		}

		return g, queryError(ctx, tx.Commit())
	}
	if !errors.Is(err, ErrNotFound) {
		return Community{}, queryError(ctx, err)
	}

	n, err := queries(tx).CountOwnerCommunities(ctx, owner)
	if err != nil {
		return Community{}, queryError(ctx, err)
	}
	if n >= 10 {
		return Community{}, ErrLimit
	}

	id, err := queries(tx).CreateCommunity(ctx, sqlc.CreateCommunityParams{
		Name:        name,
		Description: description,
		Owner:       owner,
		Private:     boolInt64(private),
		CreatedAt:   time.Now().UnixMilli(),
	})
	if err != nil {
		return Community{}, queryError(ctx, err)
	}
	if err = queries(tx).SetCommunityMember(ctx, sqlc.SetCommunityMemberParams{GroupID: id, Username: owner, Role: "owner"}); err != nil {
		return Community{}, queryError(ctx, err)
	}

	return Community{ID: id, Name: name, Description: description, Owner: owner, Private: private, Members: 1}, queryError(ctx, tx.Commit())
}

// CommunityMembership changes only membership state; it never grants room-wide
// moderator authority. Private-group requests remain pending until approved.
func (s *Store) CommunityMembership(ctx context.Context, id int64, actor, target, operation string) error {
	actor, err := username(actor)
	if err != nil {
		return queryError(ctx, err)
	}

	target, err = username(target)
	if err != nil {
		return queryError(ctx, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	record, err := queries(tx).GetCommunity(ctx, id)
	g := communityModel(record)
	err = queryError(ctx, err)
	if err != nil {
		return queryError(ctx, err)
	}

	actorRole, err := queries(tx).GetCommunityRole(ctx, sqlc.GetCommunityRoleParams{GroupID: id, Username: actor})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return queryError(ctx, err)
	}

	targetRole, err := queries(tx).GetCommunityRole(ctx, sqlc.GetCommunityRoleParams{GroupID: id, Username: target})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return queryError(ctx, err)
	}

	role := ""
	remove := false
	switch operation {
	case "join":
		if actor != target {
			return ErrForbidden
		}
		if targetRole != "" {
			return queryError(ctx, tx.Commit())
		}
		if g.Members >= 1000 {
			return ErrLimit
		}

		role = "member"
		if g.Private {
			role = "pending"
		}
	case "leave":
		if actor != target || targetRole == "owner" {
			return ErrForbidden
		}

		remove = true
	case "approve":
		if actorRole != "owner" && actorRole != "moderator" {
			return ErrForbidden
		}
		if targetRole != "pending" {
			return ErrConflict
		}
		if g.Members >= 1000 {
			return ErrLimit
		}

		role = "member"
	case "add":
		if actorRole != "owner" && actorRole != "moderator" {
			return ErrForbidden
		}
		if targetRole != "" && targetRole != "pending" {
			return queryError(ctx, tx.Commit())
		}
		if g.Members >= 1000 {
			return ErrLimit
		}

		role = "member"
	case "promote":
		if actorRole != "owner" || targetRole != "member" {
			return ErrForbidden
		}

		role = "moderator"
	case "demote":
		if actorRole != "owner" || targetRole != "moderator" {
			return ErrForbidden
		}

		role = "member"
	case "remove":
		if actorRole != "owner" && actorRole != "moderator" {
			return ErrForbidden
		}
		if targetRole == "owner" || (targetRole == "moderator" && actorRole != "owner") {
			return ErrForbidden
		}

		remove = true
	default:
		return ErrInvalid
	}
	if remove {
		err = queries(tx).DeleteCommunityMember(ctx, sqlc.DeleteCommunityMemberParams{GroupID: id, Username: target})
	} else {
		err = queries(tx).SetCommunityMember(ctx, sqlc.SetCommunityMemberParams{GroupID: id, Username: target, Role: role})
	}
	if err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) CommunityMembers(ctx context.Context, id int64, actor string) ([]CommunityMember, error) {
	g, err := s.Community(ctx, id)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	role, err := s.CommunityRole(ctx, id, actor)
	if err != nil {
		return nil, queryError(ctx, err)
	}
	if g.Private && (role == "" || role == "pending") {
		return nil, ErrForbidden
	}

	rows, err := queries(s.db).ListCommunityMembers(ctx, sqlc.ListCommunityMembersParams{GroupID: id, IncludePending: boolInt64(role == "owner" || role == "moderator")})
	if err != nil {
		return nil, queryError(ctx, err)
	}

	var out []CommunityMember
	for _, row := range rows {
		out = append(out, CommunityMember(row))
	}

	return out, nil
}

func (s *Store) LinkCommunityRoom(ctx context.Context, id int64, actor, room string, remove bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return queryError(ctx, err)
	}
	defer tx.Rollback()

	role, err := queries(tx).GetCommunityRole(ctx, sqlc.GetCommunityRoleParams{GroupID: id, Username: actor})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return queryError(ctx, err)
	}
	if role != "owner" && role != "moderator" {
		return ErrForbidden
	}

	r, err := readRoom(ctx, tx, room)
	if err != nil {
		return queryError(ctx, err)
	}
	if r.Owner != actor {
		return ErrForbidden
	}
	if remove {
		err = queries(tx).DeleteCommunityRoom(ctx, sqlc.DeleteCommunityRoomParams{GroupID: id, Room: r.Name})
	} else {
		existing, queryErr := queries(tx).GetRoomCommunity(ctx, r.Name)
		err = queryErr
		if err == nil && existing != id {
			return ErrConflict
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return queryError(ctx, err)
		}

		err = queries(tx).AddCommunityRoom(ctx, sqlc.AddCommunityRoomParams{GroupID: id, Room: r.Name})
	}
	if err != nil {
		return queryError(ctx, err)
	}

	return queryError(ctx, tx.Commit())
}

func (s *Store) CommunityRooms(ctx context.Context, id int64, actor string) ([]Room, error) {
	g, err := s.Community(ctx, id)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	role, err := s.CommunityRole(ctx, id, actor)
	if err != nil {
		return nil, queryError(ctx, err)
	}
	if g.Private && (role == "" || role == "pending") {
		return nil, ErrForbidden
	}

	rows, err := queries(s.db).ListCommunityRooms(ctx, id)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	out := make([]Room, 0, len(rows))
	for _, row := range rows {
		out = append(out, Room{
			Name:        row.Name,
			Category:    int16(row.Category),
			Capacity:    int(row.Capacity),
			Description: row.Description,
			Owner:       row.Owner,
		})
	}

	return out, nil
}

func (s *Store) AddCommunityPost(ctx context.Context, id, parent int64, actor, body, nonce string) (int64, error) {
	if strings.TrimSpace(body) == "" || !validCommunityText(body, 2000) || !validCommerceNonce(nonce) {
		return 0, ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, queryError(ctx, err)
	}
	defer tx.Rollback()

	role, err := queries(tx).GetCommunityRole(ctx, sqlc.GetCommunityRoleParams{GroupID: id, Username: actor})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, queryError(ctx, err)
	}
	if err != nil || role == "pending" {
		return 0, ErrForbidden
	}

	previous, err := queries(tx).CommunityPostByNonce(ctx, sqlc.CommunityPostByNonceParams{Author: actor, Nonce: nonce})
	if err == nil {
		if previous.GroupID != id || previous.ParentID != parent || previous.Body != body {
			return 0, ErrConflict
		}

		return previous.ID, queryError(ctx, tx.Commit())
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, queryError(ctx, err)
	}
	if parent != 0 {
		record, err := queries(tx).GetCommunityPostParent(ctx, parent)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, queryError(ctx, err)
		}
		if err != nil || record.GroupID != id || record.ParentID != 0 {
			return 0, ErrInvalid
		}
	}

	postID, err := queries(tx).CreateCommunityPost(ctx, sqlc.CreateCommunityPostParams{
		GroupID:   id,
		ParentID:  parent,
		Author:    actor,
		Body:      body,
		Nonce:     nonce,
		CreatedAt: time.Now().UnixMilli(),
	})
	if err != nil {
		return 0, queryError(ctx, err)
	}

	return postID, queryError(ctx, tx.Commit())
}

func (s *Store) CommunityPosts(ctx context.Context, id int64, actor string) ([]CommunityPost, error) {
	g, err := s.Community(ctx, id)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	role, err := s.CommunityRole(ctx, id, actor)
	if err != nil {
		return nil, queryError(ctx, err)
	}
	if g.Private && (role == "" || role == "pending") {
		return nil, ErrForbidden
	}

	rows, err := queries(s.db).ListCommunityPosts(ctx, id)
	if err != nil {
		return nil, queryError(ctx, err)
	}

	var out []CommunityPost
	for _, row := range rows {
		out = append(out, CommunityPost(row))
	}

	return out, nil
}
