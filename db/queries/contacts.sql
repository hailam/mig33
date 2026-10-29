-- name: HasContactRoster :one
SELECT EXISTS(SELECT 1 FROM contact_rosters WHERE owner=@owner);
-- name: CreateContactRoster :exec
INSERT INTO contact_rosters(owner,revision,checkpoint,next_group_id) VALUES(@owner,4,@now,2);
-- name: CreateDefaultContactGroup :exec
INSERT INTO contact_groups(owner,id,name,updated_at) VALUES(@owner,1,'Friends',@now);
-- name: GetContactRoster :one
SELECT * FROM contact_rosters WHERE owner=@owner;
-- name: ChangedContactGroups :many
SELECT id,name FROM contact_groups WHERE owner=@owner AND updated_at>@after_id ORDER BY id;
-- name: ChangedContacts :many
SELECT * FROM contacts WHERE owner=@owner AND updated_at>@after_id ORDER BY id;
-- name: DeletedContacts :many
SELECT id FROM contact_deletions WHERE owner=@owner AND kind=@kind AND deleted_at>@after_id ORDER BY id;
-- name: GetContact :one
SELECT * FROM contacts WHERE owner=@owner AND id=@id;
-- name: GetContactByIdentity :one
SELECT * FROM contacts WHERE owner=@owner AND username=@username;
-- name: IsLocalContactIDOccupied :one
SELECT EXISTS(SELECT 1 FROM contacts c WHERE c.owner=@owner AND c.id=@id) OR EXISTS(SELECT 1 FROM identities i WHERE i.id=@id) AS occupied;
-- name: SetNextLocalContactID :exec
UPDATE contact_rosters SET next_local_id=@next_id WHERE owner=@owner;
-- name: HasConflictingContact :one
SELECT EXISTS(SELECT 1 FROM contacts WHERE owner=@owner AND (id=@id OR username=@username));
-- name: CountContacts :one
SELECT COUNT(*) FROM contacts WHERE owner=@owner;
-- name: ClearContactDeletion :exec
DELETE FROM contact_deletions WHERE owner=@owner AND kind='contact' AND id=@id;
-- name: HasDuplicateContactIdentity :one
SELECT EXISTS(SELECT 1 FROM contacts WHERE owner=@owner AND username=@username AND id<>@id);
-- name: UpdateContact :exec
UPDATE contacts SET group_id=@group_id,username=@username,display=@display,mobile=@mobile,share_mobile=@share_mobile,external_json=@external_json,updated_at=@now WHERE owner=@owner AND id=@id;
-- name: DeleteContact :execrows
DELETE FROM contacts WHERE owner=@owner AND id=@id;
-- name: CountContactGroups :one
SELECT COUNT(*) FROM contact_groups WHERE owner=@owner;
-- name: CreateContactGroup :exec
INSERT INTO contact_groups(owner,id,name,updated_at) VALUES(@owner,@id,@name,@now);
-- name: AdvanceContactGroupID :exec
UPDATE contact_rosters SET next_group_id=next_group_id+1 WHERE owner=@owner;
-- name: ContactGroupName :one
SELECT name FROM contact_groups WHERE owner=@owner AND id=@id;
-- name: RenameContactGroup :exec
UPDATE contact_groups SET name=@name,updated_at=@now WHERE owner=@owner AND id=@id;
-- name: ContactGroupOccupied :one
SELECT EXISTS(SELECT 1 FROM contacts WHERE owner=@owner AND group_id=@id);
-- name: DeleteContactGroup :exec
DELETE FROM contact_groups WHERE owner=@owner AND id=@id;
-- name: MarkContactDeleted :exec
INSERT INTO contact_deletions(owner,kind,id,deleted_at) VALUES(@owner,@kind,@id,@now) ON CONFLICT(owner,kind,id) DO UPDATE SET deleted_at=excluded.deleted_at;
-- name: AdvanceContacts :exec
UPDATE contact_rosters SET revision=revision+1,checkpoint=@now WHERE owner=@owner;
-- name: HasContactGroup :one
SELECT EXISTS(SELECT 1 FROM contact_groups WHERE owner=@owner AND id=@id);
-- name: ContactGroupNameConflict :one
SELECT EXISTS(SELECT 1 FROM contact_groups WHERE owner=@owner AND name=@name COLLATE NOCASE AND id<>@id);
-- name: CreateContact :exec
INSERT INTO contacts(owner,id,group_id,username,display,mobile,share_mobile,updated_at,contact_kind,external_json) VALUES(@owner,@id,@group_id,@username,@display,@mobile,@share_mobile,@now,@contact_kind,@external_json);
