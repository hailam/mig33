-- name: ListRooms :many
SELECT * FROM chat_rooms WHERE CAST(@category AS INTEGER)=0 OR category=@category ORDER BY name;
-- name: GetRoom :one
SELECT * FROM chat_rooms WHERE name=@name;
-- name: RoomTombstoned :one
SELECT EXISTS(SELECT 1 FROM room_tombstones WHERE name=@name);
-- name: CountOwnedRooms :one
SELECT COUNT(*) FROM chat_rooms WHERE owner=@owner;
-- name: CreateRoomDefinition :exec
INSERT INTO chat_rooms(name,category,capacity,description,owner,created_at)
VALUES(@name,1,@capacity,@description,@owner,@created_at);
-- name: SearchRoomDefinitions :many
SELECT r.* FROM chat_rooms r LEFT JOIN room_settings s ON s.room=r.name
WHERE (instr(lower(r.name),lower(@query))>0 OR instr(lower(r.description),lower(@query))>0)
AND (CAST(@language AS TEXT)='' OR lower(COALESCE(s.language,'English'))=lower(@language))
AND instr(lower(COALESCE(s.keywords,'')),lower(@keywords))>0 ORDER BY r.name LIMIT @result_limit;
-- name: FavoriteRoomDefinitions :many
SELECT r.* FROM room_favorites f JOIN chat_rooms r ON r.name=f.room WHERE f.owner=@owner ORDER BY r.name;
-- name: IsFavoriteRoom :one
SELECT EXISTS(SELECT 1 FROM room_favorites WHERE owner=@owner AND room=@room);
-- name: CountFavoriteRooms :one
SELECT COUNT(*) FROM room_favorites WHERE owner=@owner;
-- name: AddFavoriteRoom :exec
INSERT INTO room_favorites(owner,room) VALUES(@owner,@room);
-- name: RemoveFavoriteRoom :exec
DELETE FROM room_favorites WHERE owner=@owner AND room=@room;
-- name: IsRoomModerator :one
SELECT EXISTS(SELECT 1 FROM room_moderators WHERE room=@room AND username=@username);
-- name: IsRoomMuted :one
SELECT EXISTS(SELECT 1 FROM room_mutes WHERE room=@room AND username=@username);
-- name: IsRoomBanned :one
SELECT EXISTS(SELECT 1 FROM room_bans WHERE room=@room AND username=@username);
-- name: ListRoomModerators :many
SELECT username FROM room_moderators WHERE room=@room ORDER BY username;
-- name: ListRoomMuted :many
SELECT username FROM room_mutes WHERE room=@room ORDER BY username;
-- name: ListRoomBanned :many
SELECT username FROM room_bans WHERE room=@room ORDER BY username;
-- name: AddRoomModerator :exec
INSERT INTO room_moderators(room,username) VALUES(@room,@username) ON CONFLICT(room,username) DO NOTHING;
-- name: RemoveRoomModerator :exec
DELETE FROM room_moderators WHERE room=@room AND username=@username;
-- name: AddRoomMute :exec
INSERT INTO room_mutes(room,username) VALUES(@room,@username) ON CONFLICT(room,username) DO NOTHING;
-- name: RemoveRoomMute :exec
DELETE FROM room_mutes WHERE room=@room AND username=@username;
