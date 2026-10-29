package chat

import "context"

func (s *Service) CreateGroup(ctx context.Context, id, actor string, members []string) (Group, error) {
	return s.repository.CreateGroup(ctx, id, actor, members)
}

func (s *Service) Invite(ctx context.Context, id, actor string, members []string) (Group, error) {
	return s.repository.Invite(ctx, id, actor, members)
}

func (s *Service) Leave(ctx context.Context, id, actor string) (Group, error) {
	return s.repository.Leave(ctx, id, actor)
}

func (s *Service) Group(ctx context.Context, id, viewer string) (Group, error) {
	return s.repository.Group(ctx, id, viewer)
}

func (s *Service) History(ctx context.Context, viewer string, kind byte, target string, after int64, limit int) ([]Message, error) {
	return s.repository.History(ctx, viewer, kind, target, after, limit)
}

func (s *Service) Pending(ctx context.Context, viewer string, limit int) ([]Message, error) {
	return s.repository.Pending(ctx, viewer, limit)
}

func (s *Service) MarkDelivered(ctx context.Context, viewer string, id int64) error {
	return s.repository.MarkDelivered(ctx, viewer, id)
}

func (s *Service) LatestMessageID(ctx context.Context) (int64, error) {
	return s.repository.LatestMessageID(ctx)
}

func (s *Service) OwnMessagesAfter(ctx context.Context, viewer string, after int64, limit int) ([]Message, error) {
	return s.repository.OwnMessagesAfter(ctx, viewer, after, limit)
}

func (s *Service) RequestedMessage(ctx context.Context, viewer, key, hash string) (Message, error) {
	return s.repository.RequestedMessage(ctx, viewer, key, hash)
}
