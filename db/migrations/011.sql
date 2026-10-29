
CREATE TABLE service_entries (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 kind TEXT NOT NULL, owner TEXT NOT NULL, target TEXT NOT NULL DEFAULT '',
 body TEXT NOT NULL, state TEXT NOT NULL, nonce TEXT NOT NULL,
 created_at INTEGER NOT NULL,
 UNIQUE(owner, nonce)
);
CREATE INDEX service_entries_owner ON service_entries(owner, kind, id);
CREATE TABLE service_photos (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 owner TEXT NOT NULL, caption TEXT NOT NULL DEFAULT '', mime TEXT NOT NULL,
 data BLOB NOT NULL, nonce TEXT NOT NULL, created_at INTEGER NOT NULL,
 UNIQUE(owner, nonce)
);
CREATE TABLE service_avatars (
 owner TEXT PRIMARY KEY, photo_id INTEGER REFERENCES service_photos(id) ON DELETE SET NULL
);
ALTER TABLE messages ADD COLUMN photo_id INTEGER NOT NULL DEFAULT 0;
PRAGMA user_version = 11;
