
CREATE TABLE room_moderators (
    room TEXT NOT NULL REFERENCES chat_rooms(name),
    username TEXT NOT NULL,
    PRIMARY KEY (room, username)
);
CREATE TABLE room_mutes (
    room TEXT NOT NULL REFERENCES chat_rooms(name),
    username TEXT NOT NULL,
    PRIMARY KEY (room, username)
);
INSERT INTO room_moderators(room, username)
SELECT name, 'alice' FROM chat_rooms WHERE name IN ('Lobby', 'Gaming', 'Help');
PRAGMA user_version = 3;
