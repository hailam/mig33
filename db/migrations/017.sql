
ALTER TABLE messages ADD COLUMN markup TEXT NOT NULL DEFAULT '';
PRAGMA user_version = 17;
