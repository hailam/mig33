
CREATE TABLE gift_notes (
 operation_id INTEGER PRIMARY KEY REFERENCES credit_operations(id),
 note TEXT NOT NULL CHECK(length(CAST(note AS BLOB)) BETWEEN 1 AND 320)
);
CREATE TABLE gift_ratings (
 username TEXT NOT NULL REFERENCES identities(username),
 item TEXT NOT NULL,
 rating INTEGER NOT NULL CHECK(rating BETWEEN 1 AND 5),
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(username,item)
);
CREATE INDEX gift_ratings_item ON gift_ratings(item);
PRAGMA user_version = 18;
