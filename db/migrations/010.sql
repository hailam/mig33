
CREATE TABLE communities(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL COLLATE NOCASE UNIQUE,description TEXT NOT NULL,owner TEXT NOT NULL,private INTEGER NOT NULL CHECK(private IN(0,1)),created_at INTEGER NOT NULL);
CREATE TABLE community_members(group_id INTEGER NOT NULL REFERENCES communities(id),username TEXT NOT NULL,role TEXT NOT NULL CHECK(role IN('owner','moderator','member','pending')),PRIMARY KEY(group_id,username));
CREATE INDEX community_members_user ON community_members(username);
CREATE TABLE community_rooms(group_id INTEGER NOT NULL REFERENCES communities(id),room TEXT NOT NULL COLLATE NOCASE UNIQUE REFERENCES chat_rooms(name),PRIMARY KEY(group_id,room));
CREATE TABLE community_posts(id INTEGER PRIMARY KEY AUTOINCREMENT,group_id INTEGER NOT NULL REFERENCES communities(id),parent_id INTEGER NOT NULL DEFAULT 0,author TEXT NOT NULL,body TEXT NOT NULL,nonce TEXT NOT NULL,created_at INTEGER NOT NULL,UNIQUE(author,nonce));
CREATE TABLE credit_wallets(username TEXT PRIMARY KEY NOT NULL,balance INTEGER NOT NULL DEFAULT 0 CHECK(balance>=0 AND balance<=1000000000000));
CREATE TABLE credit_operations(id INTEGER PRIMARY KEY AUTOINCREMENT,actor TEXT NOT NULL,nonce TEXT NOT NULL,kind TEXT NOT NULL,recipient TEXT NOT NULL,amount INTEGER NOT NULL CHECK(amount>0),item TEXT NOT NULL DEFAULT '',created_at INTEGER NOT NULL,UNIQUE(actor,nonce));
CREATE TABLE credit_entries(id INTEGER PRIMARY KEY AUTOINCREMENT,operation_id INTEGER NOT NULL REFERENCES credit_operations(id),username TEXT NOT NULL,delta INTEGER NOT NULL,balance_after INTEGER NOT NULL CHECK(balance_after>=0),UNIQUE(operation_id,username));
CREATE INDEX credit_entries_user ON credit_entries(username,id);
CREATE TABLE owned_store_items(id INTEGER PRIMARY KEY AUTOINCREMENT,operation_id INTEGER NOT NULL UNIQUE REFERENCES credit_operations(id),owner TEXT NOT NULL,sender TEXT NOT NULL,item TEXT NOT NULL,created_at INTEGER NOT NULL);
CREATE INDEX owned_items_user ON owned_store_items(owner,id);
CREATE TABLE store_preferences(username TEXT PRIMARY KEY NOT NULL,theme TEXT NOT NULL);
PRAGMA user_version=10;
