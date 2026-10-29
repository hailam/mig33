package realtime

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hailam/mig33/internal/accounts"
	"github.com/hailam/mig33/internal/activity"
	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/commerce"
	"github.com/hailam/mig33/internal/events"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/formatting"
	"github.com/hailam/mig33/internal/review"
	sessionview "github.com/hailam/mig33/internal/session"
)

func validCSRF(x *session, value string) bool {
	return x.csrf != "" && subtle.ConstantTimeCompare([]byte(x.csrf), []byte(value)) == 1
}

func (s *Coordinator) requireMutation(ctx context.Context, id, csrf string) (*session, error) {
	x, err := s.requireSession(ctx, id)
	if err != nil {
		return nil, err
	}
	if !validCSRF(x, csrf) {
		return nil, faults.NewError(faults.Forbidden, "Invalid form token")
	}
	return x, nil
}

func (s *Coordinator) WebSession(ctx context.Context, id string) (sessionview.View, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireSession(ctx, id)
	if err != nil {
		return sessionview.View{}, nil, err
	}

	joined := make([]string, 0, len(x.rooms))
	for room := range x.rooms {
		joined = append(joined, room)
	}
	sort.Strings(joined)
	return s.sessionView(x), joined, nil
}

func (s *Coordinator) RequireNative(ctx context.Context, id string) (sessionview.View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return sessionview.View{ID: id}, err
	}
	x := s.active(ctx, id)
	if x == nil {
		return sessionview.View{}, fmt.Errorf("Session missing or expired; log in again")
	}
	x.touched = time.Now()
	if !x.authenticated {
		return sessionview.View{ID: id}, fmt.Errorf("Complete login first")
	}
	return s.sessionView(x), nil
}

func (s *Coordinator) Presence(ctx context.Context, name string) byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ctx.Err() != nil {
		return 99
	}
	return s.presence(name)
}

func (s *Coordinator) Participants(ctx context.Context, room string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ctx.Err() != nil {
		return nil
	}
	return s.roomParticipants(room)
}

func (s *Coordinator) RegisterIdentity(ctx context.Context, input accounts.Registration) (accounts.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, err := s.accountService.Register(ctx, input)
	if err != nil {
		return accounts.Profile{}, err
	}

	s.cacheIdentity(a)
	return accounts.PublicProfile(a), nil
}

func (s *Coordinator) RecoverIdentity(ctx context.Context, challenge, code, password string) (accounts.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, err := s.accountService.Recover(ctx, challenge, code, password)
	if err != nil {
		return accounts.Profile{}, err
	}

	s.cacheIdentity(a)
	s.invalidateIdentitySessions(ctx, a.Username, "")
	return accounts.PublicProfile(a), nil
}

func (s *Coordinator) UpdateProfile(ctx context.Context, id, csrf, field, value string) (accounts.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return accounts.Profile{}, err
	}

	a, err := s.accountService.UpdateProfile(ctx, s.accounts[x.username].identity, field, value)
	if err != nil {
		return accounts.Profile{}, err
	}

	s.cacheIdentity(a)
	s.broadcastContactStatus(ctx, x.username)
	return accounts.PublicProfile(a), nil
}

func (s *Coordinator) BeginPasswordChange(ctx context.Context, id, csrf, password string) (accounts.IdentityChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return accounts.IdentityChallenge{}, err
	}
	return s.accountService.BeginPasswordChange(ctx, s.accounts[x.username].identity, x.csrf, password)
}

func (s *Coordinator) CompletePasswordChange(ctx context.Context, id, csrf, challenge, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return err
	}

	a, err := s.accountService.ChangePassword(ctx, s.accounts[x.username].identity, x.csrf, challenge, password)
	if err != nil {
		return err
	}

	s.cacheIdentity(a)
	s.invalidateIdentitySessions(ctx, x.username, x.id)
	return nil
}

func (s *Coordinator) ChangeMobile(ctx context.Context, id, csrf, mobile, code string) (accounts.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return accounts.Profile{}, err
	}
	if code != accounts.DevelopmentOTP {
		return accounts.Profile{}, faults.ErrInvalid
	}

	a, err := s.accountService.ChangeMobile(ctx, s.accounts[x.username].identity, mobile, true)
	if err != nil {
		return accounts.Profile{}, err
	}

	s.cacheIdentity(a)
	return accounts.PublicProfile(a), nil
}

func (s *Coordinator) Unblock(ctx context.Context, id, csrf, target string) error {
	if gate := s.deliveryGate(id); gate != nil {
		gate.Lock()
		defer gate.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return err
	}

	target = strings.ToLower(target)
	if s.accounts[target] == nil || target == x.username {
		return faults.ErrInvalid
	}
	return s.contactsService.SetBlocked(ctx, x.username, target, false)
}

func commandError(result []*events.Event) error {
	for _, event := range result {
		if rejected, ok := event.Data.(events.Rejected); ok {
			return fmt.Errorf("%s", rejected.Message)
		}
	}
	return nil
}

func (s *Coordinator) JoinRoom(ctx context.Context, id, csrf, room string) error {
	return s.changeRoomMembership(ctx, id, csrf, room, commands.RoomJoin)
}

func (s *Coordinator) LeaveRoom(ctx context.Context, id, csrf, room string) error {
	return s.changeRoomMembership(ctx, id, csrf, room, commands.RoomLeave)
}

func (s *Coordinator) changeRoomMembership(ctx context.Context, id, csrf, room string, op commands.RoomOperation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return err
	}
	return commandError(s.roomOperation(ctx, x, &commands.Request{Input: commands.Room{Operation: op, Name: commands.Value(room)}}))
}

func (s *Coordinator) ModerateRoom(ctx context.Context, id, csrf, room, target string, muted bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return "", err
	}

	op := commands.RoomMute
	if !muted {
		op = commands.RoomUnmute
	}
	result := s.roomOperation(ctx, x, &commands.Request{Input: commands.Room{Operation: op, Name: commands.Value(room), Target: commands.Value(target)}})
	if err := commandError(result); err != nil {
		return "", err
	}
	if len(result) == 1 {
		if ack, ok := result[0].Data.(events.Acknowledged); ok && ack.Message != nil {
			return *ack.Message, nil
		}
	}
	return "", nil
}

func (s *Coordinator) ChangeRoom(ctx context.Context, id, csrf, name, operation, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return err
	}

	room, err := s.roomsService.Room(ctx, name)
	if err != nil {
		return err
	}

	mod, err := s.roomsService.IsRoomModerator(ctx, room.Name, x.username)
	if err != nil {
		return err
	}
	if room.Owner != x.username && (!mod || !x.rooms[room.Name]) {
		return faults.ErrForbidden
	}

	if operation == "delete" {
		if value != room.Name {
			return faults.ErrInvalid
		}
		if err := s.roomsService.DeleteOwnedRoom(ctx, room.Name, x.username); err != nil {
			return err
		}
		s.cancelKickVote(ctx, room.Name, "the room was deleted")
		for _, other := range s.sessions {
			if other.rooms[room.Name] {
				s.roomLeave(ctx, other, room.Name)
				if !other.webOnly {
					select {
					case other.push <- events.New(0, events.Popup{Title: "Chat room", Message: room.Name + " was deleted by its owner. Close the retained room tab."}):
					default:
					}
				}
			}
			delete(other.knownFavorites, room.Name)
		}
		return nil
	}

	switch operation {
	case "owner", "moderator-add", "moderator-remove", "ban", "unban":
		value = strings.ToLower(strings.TrimSpace(value))
		if s.accounts[value] == nil {
			return faults.ErrNotFound
		}
	case "capacity":
		n, err := strconv.Atoi(value)
		if err != nil || n < len(s.roomParticipants(room.Name)) {
			return faults.NewError(faults.Conflict, "Capacity cannot be lower than the current room population.")
		}
	}
	if err := s.roomsService.ManageRoom(ctx, room.Name, x.username, operation, value); err != nil {
		return err
	}
	if operation == "ban" {
		if vote := s.kickVotes[room.Name]; vote != nil && vote.Target == value {
			s.cancelKickVote(ctx, room.Name, "the target was banned")
		}
		s.performRoomKick(ctx, room.Name, value, x.username)
	}
	if operation == "kicks" && value == "0" {
		s.cancelKickVote(ctx, room.Name, "the room owner disabled user kicking")
	}
	if operation == "owner" || operation == "moderator-add" {
		if vote := s.kickVotes[room.Name]; vote != nil && vote.Target == value {
			s.cancelKickVote(ctx, room.Name, "the target became a moderator")
		}
	}
	return nil
}

func (s *Coordinator) Purchase(ctx context.Context, id, csrf string, input commerce.PurchaseCommand) (commerce.PurchaseResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return commerce.PurchaseResult{}, err
	}
	input.Actor = x.username
	result, err := s.commerceService.Purchase(ctx, input)
	if err != nil {
		return result, err
	}
	if result.RefreshEmoticons {
		s.refreshEmoticonCatalog(ctx, input.Recipient)
	}
	if result.DurableMessages {
		for _, other := range s.sessions {
			s.flushPending(ctx, other)
		}
	}
	return result, nil
}

func (s *Coordinator) ApplyTheme(ctx context.Context, id, csrf, item string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return err
	}
	return s.applyStoreTheme(ctx, x.username, item)
}

func (s *Coordinator) SetAvatar(ctx context.Context, id, csrf string, photo int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return err
	}
	return s.setPhotoAvatar(ctx, x, photo)
}

func (s *Coordinator) PublishActivity(ctx context.Context, id, csrf string, input activity.ServiceEntry) (activity.ServiceEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return activity.ServiceEntry{}, err
	}
	entry, err := s.activityService.Publish(ctx, x.username, input.Kind, input.Body, input.Nonce)
	if err != nil {
		return entry, err
	}
	s.broadcastActivity(ctx, entry)
	return entry, nil
}

func (s *Coordinator) SendFormatted(ctx context.Context, id, csrf string, kind byte, target, source, format, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return err
	}
	if !validToken(key) {
		return fmt.Errorf("Reopen this message form")
	}

	body, markup := source, ""
	if format == "html" {
		doc, err := formatting.ParseChat(source)
		if err != nil {
			return err
		}
		body, markup = formatting.PlainText(doc), source
	} else if format != "plain" {
		return fmt.Errorf("Choose plain or formatted text")
	} else if !chat.CoreInput(source) {
		return fmt.Errorf("Use the room menu for this command")
	}

	data, _ := json.Marshal([]any{"wap", kind, target, format, source})
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if _, err := s.chatService.RequestedMessage(ctx, x.username, key, hash); err == nil {
		return nil
	} else if !errors.Is(err, faults.ErrNotFound) {
		return fmt.Errorf("Message form was already used; reopen it")
	}
	if kind == 1 {
		target = strings.ToLower(target)
	}
	if err := s.recipientAllowed(ctx, x, kind, target); err != nil {
		return err
	}

	_, err = s.sendPrepared(ctx, x, &commands.Request{}, chat.Message{
		Kind:         kind,
		Conversation: target,
		Sender:       x.username,
		Body:         body,
		Markup:       markup,
		RequestKey:   key,
		RequestHash:  hash,
	})
	return err
}

func (s *Coordinator) reportContext(ctx context.Context, x *session, kind, target string) (review.Conversation, error) {
	var names []string
	switch kind {
	case "room":
		r, err := s.roomsService.Room(ctx, target)
		if err != nil {
			return review.Conversation{}, faults.WrapError(faults.NotFound, "This chat room was not found.", err)
		}
		target = r.Name
		if !x.rooms[target] {
			return review.Conversation{}, faults.ErrForbidden
		}
		names = s.roomParticipants(target)
	case "group":
		g, err := s.chatService.Group(ctx, target, x.username)
		if err != nil {
			return review.Conversation{}, faults.ErrForbidden
		}
		names = g.Members
	default:
		return review.Conversation{}, faults.ErrInvalid
	}

	filtered := []string{}
	for _, name := range names {
		if name != x.username && s.accounts[name] != nil {
			filtered = append(filtered, name)
		}
	}
	sort.Strings(filtered)
	return review.Conversation{Reporter: x.username, Kind: kind, Context: target, Targets: filtered}, nil
}

func (s *Coordinator) ReportTargets(ctx context.Context, id, kind, target string) (string, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireSession(ctx, id)
	if err != nil {
		return "", nil, err
	}

	c, err := s.reportContext(ctx, x, kind, target)
	return c.Context, c.Targets, err
}

func (s *Coordinator) SubmitReport(ctx context.Context, id, csrf string, input review.ReportSubmission) (review.AbuseReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, id, csrf)
	if err != nil {
		return review.AbuseReport{}, err
	}

	c, err := s.reportContext(ctx, x, input.Kind, input.Context)
	if err != nil {
		return review.AbuseReport{}, err
	}
	return s.reviewService.Submit(ctx, c, review.SubmitCommand{Target: input.Target, Category: input.Category, Details: input.Details, Nonce: input.Nonce})
}
