
CREATE TABLE room_categories(id INTEGER PRIMARY KEY CHECK(id=1 OR id>=3),name TEXT NOT NULL UNIQUE);
INSERT INTO room_categories(id,name) VALUES(1,'Chat rooms');
CREATE TABLE room_tombstones(name TEXT PRIMARY KEY COLLATE NOCASE,owner TEXT NOT NULL,deleted_at INTEGER NOT NULL);
CREATE TABLE room_changes_v13(id INTEGER PRIMARY KEY AUTOINCREMENT,room TEXT NOT NULL COLLATE NOCASE,actor TEXT NOT NULL,operation TEXT NOT NULL,value TEXT NOT NULL,created_at INTEGER NOT NULL);
INSERT INTO room_changes_v13 SELECT * FROM room_changes;
DROP TABLE room_changes;
ALTER TABLE room_changes_v13 RENAME TO room_changes;
CREATE INDEX room_changes_room ON room_changes(room,id);
CREATE TABLE community_room_controls(room TEXT PRIMARY KEY COLLATE NOCASE REFERENCES chat_rooms(name) ON DELETE CASCADE,silenced INTEGER NOT NULL DEFAULT 0,locked INTEGER NOT NULL DEFAULT 0,bot TEXT NOT NULL DEFAULT '',bot_expires INTEGER NOT NULL DEFAULT 0,bot_rounds INTEGER NOT NULL DEFAULT 0);
CREATE TABLE community_room_bans(group_id INTEGER NOT NULL REFERENCES communities(id),username TEXT NOT NULL,actor TEXT NOT NULL,created_at INTEGER NOT NULL,PRIMARY KEY(group_id,username));
CREATE TABLE room_bot_results(id INTEGER PRIMARY KEY AUTOINCREMENT,room TEXT NOT NULL COLLATE NOCASE,username TEXT NOT NULL,game TEXT NOT NULL,result TEXT NOT NULL,points INTEGER NOT NULL,created_at INTEGER NOT NULL);
CREATE INDEX room_bot_results_room ON room_bot_results(room,id);
CREATE TRIGGER community_room_unlink_controls AFTER DELETE ON community_rooms BEGIN DELETE FROM community_room_controls WHERE room=OLD.room; END;
PRAGMA user_version=13;
