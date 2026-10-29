
CREATE TABLE identities (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 username TEXT NOT NULL UNIQUE COLLATE NOCASE,
 display TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT '',
 mobile TEXT NOT NULL DEFAULT '',
 bio TEXT NOT NULL DEFAULT '',
 country TEXT NOT NULL DEFAULT '',
 activated INTEGER NOT NULL DEFAULT 0 CHECK(activated IN (0,1)),
 password_salt BLOB NOT NULL,
 password_key BLOB NOT NULL,
 password_iterations INTEGER NOT NULL,
 legacy_hash INTEGER NOT NULL,
 legacy_units INTEGER NOT NULL,
 created_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX identity_mobile ON identities(mobile) WHERE mobile <> '';
CREATE TABLE identity_preferences (
 username TEXT NOT NULL REFERENCES identities(username),
 name TEXT NOT NULL,
 value TEXT NOT NULL,
 PRIMARY KEY(username,name)
);
CREATE TABLE identity_challenges (
 token TEXT PRIMARY KEY,
 purpose TEXT NOT NULL,
 username TEXT NOT NULL DEFAULT '',
 mobile TEXT NOT NULL DEFAULT '',
 expires_at INTEGER NOT NULL,
 attempts INTEGER NOT NULL DEFAULT 0,
 used INTEGER NOT NULL DEFAULT 0 CHECK(used IN (0,1))
);
PRAGMA user_version = 9;
