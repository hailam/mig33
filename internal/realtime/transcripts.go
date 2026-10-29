package realtime

import (
	"context"

	"github.com/hailam/mig33/internal/chat"
	"github.com/hailam/mig33/internal/commands"
	"github.com/hailam/mig33/internal/faults"
	"github.com/hailam/mig33/internal/formatting"
)

func (s *Coordinator) SyncChat(ctx context.Context, sid string, q chat.PageQuery) (chat.MessagePage, error) {
	if err := ctx.Err(); err != nil {
		return chat.MessagePage{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireSession(ctx, sid)
	if err != nil {
		return chat.MessagePage{}, err
	}
	return s.chatService.Sync(ctx, x.username, q)
}

func (s *Coordinator) MarkChatRead(ctx context.Context, sid, csrf string, input chat.ReadInput) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, sid, csrf)
	if err != nil {
		return 0, err
	}

	kind := chat.Kind(input.Kind)
	if kind == 0 || input.Target == "" || input.ThroughID <= 0 {
		return 0, faults.NewError(faults.Invalid, "Valid kind, target and through_id required")
	}
	if kind == 3 {
		room, err := s.roomsService.Room(ctx, input.Target)
		if err != nil || !x.rooms[room.Name] {
			return 0, faults.WrapError(faults.Forbidden, "Join the chat room first", err)
		}
		input.Target = room.Name
	}
	return s.chatService.MarkRead(ctx, x.username, input)
}

func (s *Coordinator) SendChatMessage(ctx context.Context, sid, csrf, key string, input chat.MessageInput) (chat.MessageResult, error) {
	if err := ctx.Err(); err != nil {
		return chat.MessageResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	x, err := s.requireMutation(ctx, sid, csrf)
	if err != nil {
		return chat.MessageResult{}, err
	}
	if !validToken(key) {
		return chat.MessageResult{}, faults.NewError(faults.Invalid, "Idempotency-Key must contain 48 lowercase hexadecimal characters")
	}

	message, hash, err := chat.PrepareInput(input)
	if err != nil {
		return chat.MessageResult{}, err
	}
	if result, found, err := s.chatService.Replay(ctx, x.username, key, hash); found || err != nil {
		return result, err
	}
	if err := s.recipientAllowed(ctx, x, message.Kind, message.Conversation); err != nil {
		return chat.MessageResult{}, faults.NewError(faults.Invalid, err.Error())
	}

	if message.PhotoID != 0 {
		if _, err := s.mediaService.OwnedPhoto(ctx, x.username, message.PhotoID); err != nil {
			return chat.MessageResult{}, faults.NewError(faults.Invalid, "Photo unavailable; select one of your scrapbook photos")
		}
		message.Body = "Photo " + photoToken(message.PhotoID)
	}

	if message.Markup != "" {
		doc, err := formatting.ParseChat(message.Markup)
		if err != nil {
			return chat.MessageResult{}, faults.NewError(faults.Invalid, err.Error())
		}
		message.Body = formatting.PlainText(doc)
	}

	message.Sender = x.username
	message.RequestKey = key
	message.RequestHash = hash
	saved, err := s.sendPrepared(ctx, x, &commands.Request{}, message)
	if err != nil {
		return chat.MessageResult{}, faults.WrapError(faults.Invalid, err.Error(), ctx.Err())
	}

	// Preserve the established post-commit read/error contract for modern sends.
	saved, err = s.chatService.RequestedMessage(ctx, x.username, key, hash)
	if err != nil {
		return chat.MessageResult{}, faults.WrapError(faults.Unavailable, "Saved message could not be read", err)
	}
	return chat.MessageResult{Message: saved}, nil
}
