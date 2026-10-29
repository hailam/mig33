package commerce

import "context"

func (s *Service) OwnedItems(ctx context.Context, owner string) ([]OwnedStoreItem, error) {
	return s.repo.OwnedStoreItems(ctx, owner)
}

func (s *Service) GiftNote(ctx context.Context, id int64, viewer string) (string, error) {
	return s.repo.GiftNote(ctx, id, viewer)
}
