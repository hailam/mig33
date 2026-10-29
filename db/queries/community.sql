-- name: GetCommunity :one
SELECT g.id,g.name,g.description,g.owner,g.private,(SELECT count(*) FROM community_members m WHERE m.group_id=g.id AND m.role!='pending') AS members FROM communities g WHERE g.id=@id;
-- name: GetCommunityByName :one
SELECT g.id,g.name,g.description,g.owner,g.private,(SELECT count(*) FROM community_members m WHERE m.group_id=g.id AND m.role!='pending') AS members FROM communities g WHERE g.name=@name;
-- name: ListCommunities :many
SELECT g.id,g.name,g.description,g.owner,g.private,(SELECT count(*) FROM community_members m WHERE m.group_id=g.id AND m.role!='pending') AS members FROM communities g WHERE CAST(@mine AS INTEGER)=0 OR EXISTS(SELECT 1 FROM community_members m WHERE m.group_id=g.id AND m.username=@viewer) ORDER BY g.name LIMIT 100;
-- name: GetCommunityRole :one
SELECT role FROM community_members WHERE group_id=@group_id AND username=@username;
-- name: CountOwnerCommunities :one
SELECT count(*) FROM communities WHERE owner=@owner;
-- name: CreateCommunity :execlastid
INSERT INTO communities(name,description,owner,private,created_at) VALUES(@name,@description,@owner,@private,@created_at);
-- name: SetCommunityMember :exec
INSERT INTO community_members(group_id,username,role) VALUES(@group_id,@username,@role) ON CONFLICT(group_id,username) DO UPDATE SET role=excluded.role;
-- name: DeleteCommunityMember :exec
DELETE FROM community_members WHERE group_id=@group_id AND username=@username;
-- name: ListCommunityMembers :many
SELECT username,role FROM community_members WHERE group_id=@group_id AND (CAST(@include_pending AS INTEGER)=1 OR role!='pending') ORDER BY username;
-- name: DeleteCommunityRoom :exec
DELETE FROM community_rooms WHERE group_id=@group_id AND room=@room;
-- name: GetRoomCommunity :one
SELECT group_id FROM community_rooms WHERE room=@room;
-- name: AddCommunityRoom :exec
INSERT OR IGNORE INTO community_rooms(group_id,room) VALUES(@group_id,@room);
-- name: ListCommunityRooms :many
SELECT r.name,r.category,r.capacity,r.description,r.owner FROM community_rooms c JOIN chat_rooms r ON r.name=c.room WHERE c.group_id=@group_id ORDER BY r.name;
-- name: CommunityPostByNonce :one
SELECT id,group_id,parent_id,body FROM community_posts WHERE author=@author AND nonce=@nonce;
-- name: GetCommunityPostParent :one
SELECT group_id,parent_id FROM community_posts WHERE id=@id;
-- name: CreateCommunityPost :execlastid
INSERT INTO community_posts(group_id,parent_id,author,body,nonce,created_at) VALUES(@group_id,@parent_id,@author,@body,@nonce,@created_at);
-- name: ListCommunityPosts :many
SELECT id,group_id,parent_id,created_at,author,body FROM community_posts WHERE group_id=@group_id ORDER BY id DESC LIMIT 100;
