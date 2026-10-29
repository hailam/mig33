-- name: CountPrivateGroup :one
SELECT COUNT(*) FROM chat_groups WHERE id=@id;
-- name: CreatePrivateGroup :exec
INSERT INTO chat_groups(id,creator,created_at) VALUES(@id,@creator,@created_at);
-- name: AddPrivateGroupMember :exec
INSERT INTO group_members(group_id,username,joined_at) VALUES(@group_id,@username,@joined_at);
-- name: IsPrivateGroupMember :one
SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=@group_id AND username=@username);
-- name: InvitePrivateGroupMember :exec
INSERT INTO group_members(group_id,username,joined_at) VALUES(@group_id,@username,@joined_at) ON CONFLICT(group_id,username) DO NOTHING;
-- name: RemovePrivateGroupMember :exec
DELETE FROM group_members WHERE group_id=@group_id AND username=@username;
-- name: CancelLeftGroupDeliveries :exec
UPDATE message_audience SET canceled_at=@now WHERE username=@username AND recipient=1 AND delivered_at IS NULL AND canceled_at IS NULL
AND message_id IN (SELECT id FROM messages WHERE kind=2 AND conversation=@group_id);
-- name: ListPrivateGroups :many
SELECT g.id,g.creator,m.username FROM chat_groups g JOIN group_members mine ON mine.group_id=g.id AND mine.username=@viewer JOIN group_members m ON m.group_id=g.id ORDER BY g.id,m.username;
-- name: AuthorizePrivateGroup :one
SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=g.id AND username=@viewer) FROM chat_groups g WHERE g.id=@id;
-- name: PrivateGroupCreator :one
SELECT creator FROM chat_groups WHERE id=@id;
-- name: ListPrivateGroupMembers :many
SELECT username FROM group_members WHERE group_id=@id ORDER BY username;
