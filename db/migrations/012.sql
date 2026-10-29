
CREATE TABLE room_settings (
 room TEXT PRIMARY KEY COLLATE NOCASE REFERENCES chat_rooms(name),
 language TEXT NOT NULL DEFAULT 'English', keywords TEXT NOT NULL DEFAULT '',
 user_kicks INTEGER NOT NULL DEFAULT 1 CHECK(user_kicks IN(0,1))
);
CREATE TABLE room_bans (
 room TEXT NOT NULL COLLATE NOCASE REFERENCES chat_rooms(name),
 username TEXT NOT NULL, actor TEXT NOT NULL, created_at INTEGER NOT NULL,
 PRIMARY KEY(room,username)
);
CREATE TABLE room_changes (
 id INTEGER PRIMARY KEY AUTOINCREMENT,room TEXT NOT NULL COLLATE NOCASE REFERENCES chat_rooms(name),
 actor TEXT NOT NULL,operation TEXT NOT NULL,value TEXT NOT NULL,created_at INTEGER NOT NULL
);
PRAGMA user_version=12;
