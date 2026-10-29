package rooms

import "context"

func (s *Service) SetRoomModerator(ctx context.Context, room, user string, enabled bool) error {
	return s.repository.SetRoomModerator(ctx, room, user, enabled)
}

func (s *Service) RoomCategories(ctx context.Context) ([]RoomCategory, error) {
	return s.repository.RoomCategories(ctx)
}

func (s *Service) Room(ctx context.Context, name string) (Room, error) {
	return s.repository.Room(ctx, name)
}

func (s *Service) RoomModerators(ctx context.Context, room string) ([]string, error) {
	return s.repository.RoomModerators(ctx, room)
}

func (s *Service) RoomMuted(ctx context.Context, room string) ([]string, error) {
	return s.repository.RoomMuted(ctx, room)
}

func (s *Service) IsRoomModerator(ctx context.Context, room, user string) (bool, error) {
	return s.repository.IsRoomModerator(ctx, room, user)
}

func (s *Service) IsRoomMuted(ctx context.Context, room, user string) (bool, error) {
	return s.repository.IsRoomMuted(ctx, room, user)
}

func (s *Service) SetRoomMuted(ctx context.Context, room, actor, target string, muted bool) error {
	return s.repository.SetRoomMuted(ctx, room, actor, target, muted)
}

func (s *Service) AddRoomFavorite(ctx context.Context, owner, room string) (Room, error) {
	return s.repository.AddRoomFavorite(ctx, owner, room)
}

func (s *Service) RemoveRoomFavorite(ctx context.Context, owner, room string) error {
	return s.repository.RemoveRoomFavorite(ctx, owner, room)
}

func (s *Service) FavoriteRooms(ctx context.Context, owner string) ([]Room, error) {
	return s.repository.FavoriteRooms(ctx, owner)
}

func (s *Service) RecordRoomEvent(ctx context.Context, room, actor, operation, value string) error {
	return s.repository.RecordRoomEvent(ctx, room, actor, operation, value)
}

func (s *Service) RoomSettings(ctx context.Context, room string) (RoomSettings, error) {
	return s.repository.RoomSettings(ctx, room)
}

func (s *Service) Rooms(ctx context.Context, category int16) ([]Room, error) {
	return s.repository.Rooms(ctx, category)
}

func (s *Service) SearchRooms(ctx context.Context, query string, limit int) ([]Room, error) {
	return s.repository.SearchRooms(ctx, query, limit)
}

func (s *Service) SearchRoomsFiltered(ctx context.Context, filter RoomSearchFilter, limit int) ([]Room, error) {
	return s.repository.SearchRoomsFiltered(ctx, filter, limit)
}

func (s *Service) CreateRoom(ctx context.Context, owner, name, description string) (Room, error) {
	return s.repository.CreateRoom(ctx, owner, name, description)
}

func (s *Service) RoomBanned(ctx context.Context, room, user string) (bool, error) {
	return s.repository.RoomBanned(ctx, room, user)
}

func (s *Service) RoomBans(ctx context.Context, room string) ([]string, error) {
	return s.repository.RoomBans(ctx, room)
}

func (s *Service) RoomChanges(ctx context.Context, room string) ([]RoomChange, error) {
	return s.repository.RoomChanges(ctx, room)
}

func (s *Service) ManageRoom(ctx context.Context, room, actor, operation, value string) error {
	return s.repository.ManageRoom(ctx, room, actor, operation, value)
}

func (s *Service) GroupRoomControls(ctx context.Context, room string) (GroupRoomControls, error) {
	return s.repository.GroupRoomControls(ctx, room)
}

func (s *Service) GroupRoomAccess(ctx context.Context, room, user string) (bool, bool, error) {
	return s.repository.GroupRoomAccess(ctx, room, user)
}

func (s *Service) ManageGroupRoom(ctx context.Context, room, actor, operation, value string) ([]Room, error) {
	return s.repository.ManageGroupRoom(ctx, room, actor, operation, value)
}

func (s *Service) PlayRoomBot(ctx context.Context, room, user, expectedGame, result string, points int) error {
	return s.repository.PlayRoomBot(ctx, room, user, expectedGame, result, points)
}

func (s *Service) RoomBotScores(ctx context.Context, room string) ([]RoomBotScore, error) {
	return s.repository.RoomBotScores(ctx, room)
}

func (s *Service) DeleteOwnedRoom(ctx context.Context, room, actor string) error {
	return s.repository.DeleteOwnedRoom(ctx, room, actor)
}

func (s *Service) ManageRoomCategory(ctx context.Context, actor string, id int16, name string, remove bool) error {
	return s.repository.ManageRoomCategory(ctx, actor, id, name, remove)
}

func (s *Service) SetRoomCategory(ctx context.Context, actor, room string, id int16) error {
	return s.repository.SetRoomCategory(ctx, actor, room, id)
}
