
CREATE TABLE contact_rosters (
    owner TEXT PRIMARY KEY NOT NULL,
    revision INTEGER NOT NULL CHECK (revision > 0 AND revision <= 2147483647),
    checkpoint INTEGER NOT NULL,
    next_group_id INTEGER NOT NULL
);
CREATE TABLE contact_groups (
    owner TEXT NOT NULL REFERENCES contact_rosters(owner),
    id INTEGER NOT NULL CHECK (id > 0),
    name TEXT NOT NULL COLLATE NOCASE,
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (owner, id),
    UNIQUE (owner, name)
);
CREATE TABLE contacts (
    owner TEXT NOT NULL REFERENCES contact_rosters(owner),
    id INTEGER NOT NULL CHECK (id > 0),
    group_id INTEGER,
    username TEXT NOT NULL,
    display TEXT NOT NULL,
    mobile TEXT NOT NULL,
    share_mobile INTEGER NOT NULL CHECK (share_mobile IN (0, 1)),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (owner, id),
    UNIQUE (owner, username),
    FOREIGN KEY (owner, group_id) REFERENCES contact_groups(owner, id),
    CHECK (owner <> username)
);
CREATE TABLE contact_deletions (
    owner TEXT NOT NULL REFERENCES contact_rosters(owner),
    kind TEXT NOT NULL CHECK (kind IN ('contact', 'group')),
    id INTEGER NOT NULL,
    deleted_at INTEGER NOT NULL,
    PRIMARY KEY (owner, kind, id)
);
PRAGMA user_version = 5;
