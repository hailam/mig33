
CREATE TABLE personal_blocks (
    owner TEXT NOT NULL,
    target TEXT NOT NULL,
    PRIMARY KEY (owner, target),
    CHECK (owner <> target)
);
PRAGMA user_version = 4;
