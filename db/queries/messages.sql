-- name: MessagePhotoOwner :one
SELECT owner FROM service_photos WHERE id=@id;
-- name: CreateMessage :execlastid
INSERT INTO messages(kind,conversation,sender,body,color,created_at,action,photo_id,markup) VALUES(@kind,@conversation,@sender,@body,@color,@created_at,@action,@photo_id,@markup);
-- name: AddMessageAuthor :exec
INSERT INTO message_audience(message_id,username,recipient) VALUES(@message_id,@username,@recipient);
-- name: AddMessageRecipient :exec
INSERT INTO message_audience(message_id,username,recipient) VALUES(@message_id,@username,1) ON CONFLICT(message_id,username) DO UPDATE SET recipient=1;
-- name: AddMessageRequest :exec
INSERT INTO message_requests(username,request_key,request_hash,message_id) VALUES(@username,@request_key,@request_hash,@message_id);
-- name: PendingMessages :many
SELECT m.* FROM messages m JOIN message_audience a ON a.message_id=m.id
WHERE a.username=@viewer AND a.recipient=1 AND a.delivered_at IS NULL AND a.canceled_at IS NULL AND m.kind IN(1,2)
AND NOT EXISTS(SELECT 1 FROM personal_blocks b WHERE b.owner=a.username AND b.target=m.sender) ORDER BY m.id LIMIT @count;
-- name: MarkMessageDelivered :execrows
UPDATE message_audience SET delivered_at=COALESCE(delivered_at,@now) WHERE message_id=@message_id AND username=@username AND recipient=1 AND canceled_at IS NULL;
-- name: PrivateMessageHistory :many
SELECT m.* FROM messages m JOIN message_audience a ON a.message_id=m.id AND a.username=@viewer
WHERE m.kind=1 AND m.id>@after_id AND ((m.sender=@viewer AND m.conversation=@peer) OR (m.sender=@peer AND m.conversation=@viewer)) ORDER BY m.id LIMIT @count;
-- name: RoomMessageHistory :many
SELECT m.* FROM messages m JOIN message_audience a ON a.message_id=m.id AND a.username=@viewer WHERE m.kind=3 AND m.conversation=@room AND m.id>@after_id ORDER BY m.id LIMIT @count;
-- name: HasGroupMessageHistory :one
SELECT EXISTS(SELECT 1 FROM messages m JOIN message_audience a ON a.message_id=m.id WHERE m.kind=2 AND m.conversation=@group_id AND a.username=@viewer);
-- name: GroupMessageHistory :many
SELECT m.* FROM messages m JOIN message_audience a ON a.message_id=m.id AND a.username=@viewer WHERE m.kind=2 AND m.conversation=@group_id AND m.id>@after_id ORDER BY m.id LIMIT @count;
-- name: LatestMessageID :one
SELECT CAST(COALESCE(MAX(id),0) AS INTEGER) AS id FROM messages;
-- name: OwnMessagesAfter :many
SELECT m.* FROM messages m WHERE m.sender=@viewer AND m.id>@after_id AND m.kind IN(1,2) AND m.action=0 ORDER BY m.id LIMIT @count;
-- name: MessageTranscript :many
SELECT m.* FROM messages m JOIN message_audience a ON a.message_id=m.id AND a.username=@viewer WHERE m.id>@after_id AND m.kind IN(1,2) ORDER BY m.id LIMIT @count;
-- name: RequestedMessage :one
SELECT sqlc.embed(m),r.request_hash FROM message_requests r JOIN messages m ON m.id=r.message_id WHERE r.username=@viewer AND r.request_key=@request_key;
-- name: MessageReadAudience :one
SELECT m.kind,m.sender,m.conversation FROM messages m JOIN message_audience a ON a.message_id=m.id AND a.username=@viewer WHERE m.id=@id;
-- name: MarkConversationRead :exec
INSERT INTO conversation_reads(username,kind,conversation,message_id,updated_at) VALUES(@username,@kind,@conversation,@message_id,@updated_at)
ON CONFLICT(username,kind,conversation) DO UPDATE SET message_id=MAX(message_id,excluded.message_id),updated_at=CASE WHEN excluded.message_id>message_id THEN excluded.updated_at ELSE updated_at END;
-- name: ConversationRead :one
SELECT message_id FROM conversation_reads WHERE username=@username AND kind=@kind AND conversation=@conversation;
