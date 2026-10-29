
CREATE TABLE messages_v2 (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind INTEGER NOT NULL CHECK (kind IN (1, 2, 3)),
    conversation TEXT NOT NULL,
    sender TEXT NOT NULL,
    body TEXT NOT NULL,
    color INTEGER,
    created_at INTEGER NOT NULL
);
INSERT INTO messages_v2(id, kind, conversation, sender, body, color, created_at)
SELECT id, kind, conversation, sender, body, color, created_at FROM messages;
CREATE TABLE message_audience_v2 (
    message_id INTEGER NOT NULL REFERENCES messages_v2(id),
    username TEXT NOT NULL,
    recipient INTEGER NOT NULL CHECK (recipient IN (0, 1)),
    delivered_at INTEGER,
    canceled_at INTEGER,
    PRIMARY KEY (message_id, username)
);
INSERT INTO message_audience_v2(message_id, username, recipient, delivered_at, canceled_at)
SELECT message_id, username, recipient, delivered_at, canceled_at FROM message_audience;
DROP TABLE message_audience;
DROP TABLE messages;
ALTER TABLE messages_v2 RENAME TO messages;
ALTER TABLE message_audience_v2 RENAME TO message_audience;
CREATE INDEX messages_conversation ON messages(kind, conversation, id);
CREATE INDEX audience_pending ON message_audience(username, delivered_at, message_id);
CREATE TABLE chat_rooms (
    name TEXT PRIMARY KEY NOT NULL COLLATE NOCASE,
    category INTEGER NOT NULL CHECK (category > 0),
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    description TEXT NOT NULL
);
INSERT INTO chat_rooms(name, category, capacity, description) VALUES
    ('Lobby', 1, 50, 'Meet other users and chat.'),
    ('Gaming', 1, 50, 'Talk about games.'),
    ('Help', 1, 50, 'Ask other users for help.');
PRAGMA user_version = 2;
