-- name: GetRoomSettings :one
SELECT CAST(COALESCE((SELECT language FROM room_settings s WHERE s.room=@room),'English') AS TEXT) AS language,
 CAST(COALESCE((SELECT keywords FROM room_settings s WHERE s.room=@room),'') AS TEXT) AS keywords,
 CAST(COALESCE((SELECT user_kicks FROM room_settings s WHERE s.room=@room),1) AS INTEGER) AS user_kicks;

-- name: ListRoomChanges :many
SELECT id,actor,operation,value,created_at FROM room_changes WHERE room=@room ORDER BY id DESC LIMIT 100;

-- name: EnsureRoomSettings :exec
INSERT OR IGNORE INTO room_settings(room) VALUES(@room);

-- name: SetRoomLanguage :exec
UPDATE room_settings SET language=@language WHERE room=@room;

-- name: SetRoomKeywords :exec
UPDATE room_settings SET keywords=@keywords WHERE room=@room;

-- name: SetRoomDescription :exec
UPDATE chat_rooms SET description=@description WHERE name=@name;

-- name: SetRoomUserKicks :exec
UPDATE room_settings SET user_kicks=@user_kicks WHERE room=@room;

-- name: SetRoomCapacity :exec
UPDATE chat_rooms SET capacity=@capacity WHERE name=@name;

-- name: SetRoomOwner :exec
UPDATE chat_rooms SET owner=@owner WHERE name=@name;

-- name: AddRoomBan :exec
INSERT OR IGNORE INTO room_bans(room,username,actor,created_at) VALUES(@room,@username,@actor,@created_at);

-- name: RemoveRoomBan :exec
DELETE FROM room_bans WHERE room=@room AND username=@username;

-- name: AddRoomChange :exec
INSERT INTO room_changes(room,actor,operation,value,created_at) VALUES(@room,@actor,@operation,@value,@created_at);
