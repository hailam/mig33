
ALTER TABLE chat_rooms ADD COLUMN owner TEXT NOT NULL DEFAULT '';
ALTER TABLE chat_rooms ADD COLUMN created_at INTEGER NOT NULL DEFAULT 0;
CREATE INDEX rooms_owner ON chat_rooms(owner);
CREATE TABLE room_favorites (
    owner TEXT NOT NULL,
    room TEXT NOT NULL COLLATE NOCASE REFERENCES chat_rooms(name),
    PRIMARY KEY (owner, room)
);
PRAGMA user_version = 7;
