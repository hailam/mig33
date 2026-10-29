-- name: ServiceEntryByNonce :one
SELECT * FROM service_entries WHERE owner=@owner AND nonce=@nonce;
-- name: ServiceEntryByID :one
SELECT * FROM service_entries WHERE id=@id;
-- name: CreateServiceEntry :execlastid
INSERT INTO service_entries(kind,owner,target,body,state,nonce,created_at) VALUES(@kind,@owner,@target,@body,@state,@nonce,@created_at);
-- name: ListActivityEntries :many
SELECT * FROM service_entries WHERE kind IN ('buzz','checkin') AND (owner=@owner OR @owner='') ORDER BY created_at DESC,id DESC LIMIT 100;
-- name: ListServiceEntries :many
SELECT * FROM service_entries WHERE kind=@kind AND (owner=@owner OR @owner='') AND (target=@target OR @target='') ORDER BY id DESC LIMIT 100;
-- name: DeleteServiceEntry :execrows
DELETE FROM service_entries WHERE owner=@owner AND id=@id AND kind IN ('buzz','blog','checkin','im');
-- name: PhotoByNonce :one
SELECT * FROM service_photos WHERE owner=@owner AND nonce=@nonce;
-- name: CountOwnerPhotos :one
SELECT COUNT(*) FROM service_photos WHERE owner=@owner;
-- name: CreatePhoto :execlastid
INSERT INTO service_photos(owner,caption,mime,data,nonce,created_at) VALUES(@owner,@caption,@mime,@data,@nonce,@created_at);
-- name: ListPhotos :many
SELECT id,owner,caption,mime,created_at FROM service_photos WHERE owner=@owner OR @owner='' ORDER BY id DESC LIMIT 100;
-- name: GetPhoto :one
SELECT * FROM service_photos WHERE id=@id;
-- name: PhotoSharedWith :one
SELECT EXISTS(SELECT 1 FROM messages m JOIN message_audience a ON a.message_id=m.id WHERE m.photo_id=@photo_id AND a.username=@username AND a.canceled_at IS NULL AND NOT EXISTS(SELECT 1 FROM personal_blocks b WHERE b.owner=a.username AND b.target=m.sender));
-- name: GetPhotoOwner :one
SELECT owner FROM service_photos WHERE id=@id;
-- name: SetPhotoAvatar :exec
INSERT INTO service_avatars(owner,photo_id) VALUES(@owner,@photo_id) ON CONFLICT(owner) DO UPDATE SET photo_id=excluded.photo_id;
-- name: GetPhotoAvatar :one
SELECT photo_id FROM service_avatars WHERE owner=@owner;
-- name: DeletePhoto :execrows
DELETE FROM service_photos WHERE owner=@owner AND id=@id;
-- name: ClearPhotoAvatar :exec
DELETE FROM service_avatars WHERE owner=@owner;
-- name: RemoveIMLabels :exec
DELETE FROM service_entries WHERE owner=@owner AND kind='im' AND lower(target)=lower(@service);
