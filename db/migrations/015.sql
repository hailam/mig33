
CREATE TABLE operator_reviews (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 kind TEXT NOT NULL CHECK(kind IN ('report','support')),
 item_id INTEGER NOT NULL,
 actor TEXT NOT NULL REFERENCES identities(username),
 status TEXT NOT NULL, note TEXT NOT NULL, nonce TEXT NOT NULL,
 created_at INTEGER NOT NULL, UNIQUE(actor,nonce)
);
CREATE INDEX operator_reviews_item ON operator_reviews(kind,item_id,id);
CREATE TABLE abuse_report_reviews_v15 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 report_id INTEGER NOT NULL REFERENCES abuse_reports(id),
 status TEXT NOT NULL CHECK(status IN ('pending','reviewed','dismissed')),
 note TEXT NOT NULL, created_at INTEGER NOT NULL
);
INSERT INTO abuse_report_reviews_v15 SELECT * FROM abuse_report_reviews;
DROP TABLE abuse_report_reviews;
ALTER TABLE abuse_report_reviews_v15 RENAME TO abuse_report_reviews;
ALTER TABLE contacts ADD COLUMN contact_kind TEXT NOT NULL DEFAULT 'mig33';
ALTER TABLE contacts ADD COLUMN external_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE contact_rosters ADD COLUMN next_local_id INTEGER NOT NULL DEFAULT 2147483647;
PRAGMA user_version = 15;
