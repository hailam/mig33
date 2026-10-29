-- name: ListRoomCategories :many
SELECT id,name FROM room_categories ORDER BY id;

-- name: CountRoomsInCategory :one
SELECT COUNT(*) FROM chat_rooms WHERE category=@category;

-- name: RemoveRoomCategory :exec
DELETE FROM room_categories WHERE id=@id;

-- name: CountOtherRoomCategories :one
SELECT COUNT(*) FROM room_categories WHERE id!=@id;

-- name: SaveRoomCategory :exec
INSERT INTO room_categories(id,name) VALUES(@id,@name) ON CONFLICT(id) DO UPDATE SET name=excluded.name;

-- name: RoomCategoryExists :one
SELECT EXISTS(SELECT 1 FROM room_categories WHERE id=@id);

-- name: SetRoomCategory :exec
UPDATE chat_rooms SET category=@category WHERE name=@name;

-- name: AddRoomTombstone :exec
INSERT INTO room_tombstones(name,owner,deleted_at) VALUES(@name,@owner,@deleted_at);

-- name: RemoveRoomCommunityLinks :exec
DELETE FROM community_rooms WHERE room=@room;

-- name: RemoveRoomFavorites :exec
DELETE FROM room_favorites WHERE room=@room;

-- name: RemoveRoomModerators :exec
DELETE FROM room_moderators WHERE room=@room;

-- name: RemoveRoomMutes :exec
DELETE FROM room_mutes WHERE room=@room;

-- name: RemoveRoomBans :exec
DELETE FROM room_bans WHERE room=@room;

-- name: RemoveRoomSettings :exec
DELETE FROM room_settings WHERE room=@room;

-- name: RemoveRoomDefinition :exec
DELETE FROM chat_rooms WHERE name=@name;

-- name: GetGroupRoomControls :one
SELECT r.group_id,
 CAST(COALESCE(c.silenced,0) AS INTEGER) AS silenced,
 CAST(COALESCE(c.locked,0) AS INTEGER) AS locked,
 CAST(COALESCE(c.bot,'') AS TEXT) AS bot,
 CAST(COALESCE(c.bot_expires,0) AS INTEGER) AS bot_expires,
 CAST(COALESCE(c.bot_rounds,0) AS INTEGER) AS bot_rounds
FROM community_rooms r LEFT JOIN community_room_controls c ON c.room=r.room WHERE r.room=@room;

-- name: GetGroupRoomRole :one
SELECT role FROM community_members WHERE group_id=@group_id AND username=@username;

-- name: IsCommunityRoomBanned :one
SELECT EXISTS(SELECT 1 FROM community_room_bans WHERE group_id=@group_id AND username=@username);

-- name: ListLinkedRoomDefinitions :many
SELECT r.* FROM chat_rooms r JOIN community_rooms c ON c.room=r.name WHERE c.group_id=@group_id ORDER BY r.name;

-- name: EnsureCommunityRoomControls :exec
INSERT OR IGNORE INTO community_room_controls(room) VALUES(@room);

-- name: SetCommunityRoomSilenced :exec
UPDATE community_room_controls SET silenced=@silenced WHERE room=@room;

-- name: SetCommunityRoomLocked :exec
UPDATE community_room_controls SET locked=@locked WHERE room=@room;

-- name: AddCommunityRoomBan :exec
INSERT OR IGNORE INTO community_room_bans(group_id,username,actor,created_at) VALUES(@group_id,@username,@actor,@created_at);

-- name: RemoveCommunityRoomBan :exec
DELETE FROM community_room_bans WHERE group_id=@group_id AND username=@username;

-- name: StartRoomBot :exec
UPDATE community_room_controls SET bot=@bot,bot_expires=@bot_expires,bot_rounds=0 WHERE room=@room;

-- name: StopRoomBot :exec
UPDATE community_room_controls SET bot='',bot_expires=0,bot_rounds=0 WHERE room=@room;

-- name: AddRoomInvitation :exec
INSERT INTO service_entries(kind,owner,target,body,state,nonce,created_at)
VALUES('mail',@owner,@target,@body,'local_delivered',@nonce,@created_at);

-- name: AdvanceRoomBot :exec
UPDATE community_room_controls SET bot_rounds=bot_rounds+1 WHERE room=@room;

-- name: AddRoomBotResult :exec
INSERT INTO room_bot_results(room,username,game,result,points,created_at) VALUES(@room,@username,@game,@result,@points,@created_at);

-- name: ListRoomBotScores :many
SELECT username,COUNT(*) AS rounds,CAST(SUM(points) AS INTEGER) AS points
FROM room_bot_results WHERE room=@room AND game='dice' GROUP BY username ORDER BY SUM(points) DESC,username LIMIT 20;
