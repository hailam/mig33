-- name: GetIdentity :one
SELECT * FROM identities WHERE username = @username;
-- name: GetIdentityByMobile :one
SELECT * FROM identities WHERE mobile = @mobile;
-- name: ListIdentities :many
SELECT * FROM identities ORDER BY username;
-- name: SeedIdentity :exec
INSERT INTO identities(id,username,display,status,mobile,activated,password_salt,password_key,password_iterations,legacy_hash,legacy_units,created_at) VALUES(@id,@username,@display,@status,@mobile,@activated,@password_salt,@password_key,@password_iterations,@legacy_hash,@legacy_units,@created_at) ON CONFLICT(username) DO NOTHING;
-- name: DeleteExpiredIdentityChallenges :exec
DELETE FROM identity_challenges WHERE expires_at < @now OR used = 1;
-- name: CountIdentityChallenges :one
SELECT count(*) FROM identity_challenges;
-- name: CreateIdentityChallenge :exec
INSERT INTO identity_challenges(token,purpose,username,mobile,expires_at) VALUES(@token,@purpose,@username,@mobile,@expires_at);
-- name: GetIdentityChallenge :one
SELECT * FROM identity_challenges WHERE token=@token AND purpose=@purpose AND used=0 AND attempts<5 AND expires_at>@now;
-- name: FailIdentityChallenge :exec
UPDATE identity_challenges SET attempts=attempts+1 WHERE token=@token AND used=0;
-- name: ConsumeIdentityChallenge :execrows
UPDATE identity_challenges SET used=1 WHERE token=@token AND purpose=@purpose AND used=0 AND attempts<5 AND expires_at>@now;
-- name: CountConflictingIdentities :one
SELECT count(*) FROM identities WHERE username=@username OR (mobile<>'' AND mobile=@mobile);
-- name: RegisterIdentity :execlastid
INSERT INTO identities(username,display,status,mobile,activated,password_salt,password_key,password_iterations,legacy_hash,legacy_units,created_at) VALUES(@username,@display,@status,@mobile,@activated,@password_salt,@password_key,@password_iterations,@legacy_hash,@legacy_units,@created_at);
-- name: UpdateIdentityDisplay :execrows
UPDATE identities SET display=@value WHERE username=@username;
-- name: UpdateIdentityStatus :execrows
UPDATE identities SET status=@value WHERE username=@username;
-- name: UpdateIdentityBio :execrows
UPDATE identities SET bio=@value WHERE username=@username;
-- name: UpdateIdentityCountry :execrows
UPDATE identities SET country=@value WHERE username=@username;
-- name: CountMobileOwners :one
SELECT count(*) FROM identities WHERE mobile=@mobile AND username<>@username;
-- name: SetIdentityMobile :execrows
UPDATE identities SET mobile=@mobile,activated=@activated WHERE username=@username;
-- name: ActivateIdentity :execrows
UPDATE identities SET activated=1 WHERE username=@username;
-- name: IdentityChallengeOwner :one
SELECT username,mobile FROM identity_challenges WHERE token=@token AND purpose=@purpose;
-- name: GetIdentityMobile :one
SELECT mobile FROM identities WHERE username=@username;
-- name: ReplaceIdentityPassword :execrows
UPDATE identities SET password_salt=@password_salt,password_key=@password_key,password_iterations=@password_iterations,legacy_hash=@legacy_hash,legacy_units=@legacy_units WHERE username=@username;
-- name: SetIdentityPreference :exec
INSERT INTO identity_preferences(username,name,value) VALUES(@username,@name,@value) ON CONFLICT(username,name) DO UPDATE SET value=excluded.value;
-- name: GetIdentityPreference :one
SELECT value FROM identity_preferences WHERE username=@username AND name=@name;
