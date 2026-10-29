-- name: GetCreditBalance :one
SELECT balance FROM credit_wallets WHERE username=@username;
-- name: GetCreditReceiptByNonce :one
SELECT id,actor,recipient,kind,item,amount,created_at FROM credit_operations WHERE actor=@actor AND nonce=@nonce;
-- name: GetGiftNoteByReceipt :one
SELECT CAST(COALESCE((SELECT note FROM gift_notes WHERE operation_id=@id),'') AS TEXT) AS note;
-- name: EnsureCreditWallet :exec
INSERT OR IGNORE INTO credit_wallets(username) VALUES(@username);
-- name: CreateCreditReceipt :execlastid
INSERT INTO credit_operations(actor,nonce,kind,recipient,amount,item,created_at) VALUES(@actor,@nonce,@kind,@recipient,@amount,@item,@created_at);
-- name: CreateGiftNote :exec
INSERT INTO gift_notes(operation_id,note) VALUES(@operation_id,@note);
-- name: SetCreditBalance :exec
UPDATE credit_wallets SET balance=@balance WHERE username=@username;
-- name: CreateCreditEntry :exec
INSERT INTO credit_entries(operation_id,username,delta,balance_after) VALUES(@operation_id,@username,@delta,@balance_after);
-- name: CreateOwnedStoreItem :exec
INSERT INTO owned_store_items(operation_id,owner,sender,item,created_at) VALUES(@operation_id,@owner,@sender,@item,@created_at);
-- name: ListCreditHistory :many
SELECT e.id,e.operation_id,e.delta,e.balance_after,o.created_at,o.kind,o.actor,o.recipient,o.item FROM credit_entries e JOIN credit_operations o ON o.id=e.operation_id WHERE e.username=@username ORDER BY e.id DESC LIMIT 100;
-- name: ListOwnedStoreItems :many
SELECT id,operation_id,created_at,owner,sender,item FROM owned_store_items WHERE owner=@owner ORDER BY id DESC LIMIT 100;
-- name: GetStoreTheme :one
SELECT theme FROM store_preferences WHERE username=@username;
-- name: HasStoreItem :one
SELECT EXISTS(SELECT 1 FROM owned_store_items WHERE owner=@owner AND item=@item);
-- name: SetStoreTheme :exec
INSERT INTO store_preferences(username,theme) VALUES(@username,@theme) ON CONFLICT(username) DO UPDATE SET theme=excluded.theme;
-- name: GetAuthorizedGiftNote :one
SELECT CAST(COALESCE(n.note,'') AS TEXT) AS note FROM credit_operations o LEFT JOIN gift_notes n ON n.operation_id=o.id WHERE o.id=@id AND o.kind='purchase' AND (o.actor=@viewer OR o.recipient=@viewer);
-- name: GetGiftRating :one
SELECT COUNT(*) AS count,CAST(COALESCE(AVG(rating),0) AS REAL) AS average,CAST(COALESCE(MAX(CASE WHEN username=@username THEN rating ELSE NULL END),0) AS INTEGER) AS own_rating FROM gift_ratings WHERE item=@item;
-- name: IsGiftContact :one
SELECT EXISTS(SELECT 1 FROM contacts WHERE owner=@owner AND username=@username);
-- name: GiftRatingOwnerExists :one
SELECT EXISTS(SELECT 1 FROM identities WHERE username=@username);
-- name: SetGiftRating :exec
INSERT INTO gift_ratings(username,item,rating,updated_at) VALUES(@username,@item,@rating,@updated_at) ON CONFLICT(username,item) DO UPDATE SET rating=excluded.rating,updated_at=excluded.updated_at;
