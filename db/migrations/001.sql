
CREATE TABLE chat_groups (
    id TEXT PRIMARY KEY NOT NULL,
    creator TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE TABLE group_members (
    group_id TEXT NOT NULL REFERENCES chat_groups(id),
    username TEXT NOT NULL,
    joined_at INTEGER NOT NULL,
    PRIMARY KEY (group_id, username)
);
CREATE INDEX group_members_username ON group_members(username, group_id);
CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind INTEGER NOT NULL CHECK (kind IN (1, 2)),
    conversation TEXT NOT NULL,
    sender TEXT NOT NULL,
    body TEXT NOT NULL,
    color INTEGER,
    created_at INTEGER NOT NULL
);
CREATE INDEX messages_conversation ON messages(kind, conversation, id);
CREATE TABLE message_audience (
    message_id INTEGER NOT NULL REFERENCES messages(id),
    username TEXT NOT NULL,
    recipient INTEGER NOT NULL CHECK (recipient IN (0, 1)),
    delivered_at INTEGER,
    canceled_at INTEGER,
    PRIMARY KEY (message_id, username)
);
CREATE INDEX audience_pending ON message_audience(username, delivered_at, message_id);
PRAGMA user_version = 1;
