
ALTER TABLE messages ADD COLUMN action INTEGER NOT NULL DEFAULT 0 CHECK (action IN (0, 1));
PRAGMA user_version = 8;
