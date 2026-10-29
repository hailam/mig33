
CREATE TABLE message_requests (
    username TEXT NOT NULL,
    request_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    message_id INTEGER NOT NULL REFERENCES messages(id),
    PRIMARY KEY(username,request_key)
);
CREATE TABLE conversation_reads (
    username TEXT NOT NULL,
    kind INTEGER NOT NULL CHECK(kind IN (1,2,3)),
    conversation TEXT NOT NULL,
    message_id INTEGER NOT NULL REFERENCES messages(id),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY(username,kind,conversation)
);
PRAGMA user_version = 16;
