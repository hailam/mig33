-- name: AddPersonalBlock :exec
INSERT INTO personal_blocks(owner,target) VALUES(@owner,@target) ON CONFLICT(owner,target) DO NOTHING;
-- name: CancelBlockedDeliveries :exec
UPDATE message_audience SET canceled_at=@now
WHERE username=@owner AND recipient=1 AND delivered_at IS NULL AND canceled_at IS NULL
AND message_id IN (SELECT id FROM messages WHERE sender=@target);
-- name: RemovePersonalBlock :exec
DELETE FROM personal_blocks WHERE owner=@owner AND target=@target;
-- name: ListPersonalBlocks :many
SELECT target FROM personal_blocks WHERE owner=@owner ORDER BY target;
-- name: IsPersonalBlocked :one
SELECT EXISTS(SELECT 1 FROM personal_blocks WHERE owner=@owner AND target=@target);
