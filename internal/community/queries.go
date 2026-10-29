package community

import "context"

func (s *Service) Role(ctx context.Context, id int64, user string) (string, error) {
	return s.repo.CommunityRole(ctx, id, user)
}
