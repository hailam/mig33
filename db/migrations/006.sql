
CREATE TABLE abuse_reports (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 reporter TEXT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('room','group')),
 context TEXT NOT NULL,
 target TEXT NOT NULL,
 category TEXT NOT NULL CHECK(category IN ('harassment','spam','content','other')),
 details TEXT NOT NULL,
 nonce TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('pending','reviewed','dismissed')),
 created_at INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 review_note TEXT NOT NULL,
 UNIQUE(reporter,nonce)
);
CREATE INDEX abuse_reports_status ON abuse_reports(status,id);
CREATE TABLE abuse_report_reviews (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 report_id INTEGER NOT NULL REFERENCES abuse_reports(id),
 status TEXT NOT NULL CHECK(status IN ('reviewed','dismissed')),
 note TEXT NOT NULL,
 created_at INTEGER NOT NULL
);
PRAGMA user_version = 6;
