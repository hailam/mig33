# Architecture refactor native acceptance: PASS

Validated 2026-10-28 using an isolated binary built from `cmd/omni`, fresh SQLite fixtures and the unchanged mig33 4.62 publisher JAR. Four existing harnesses passed across **11 client sessions, 741 captured frames and 105 Canvas screenshots**. Every client received its logout acknowledgment.

| Suite | Clients | Frames | Screenshots | Verified behavior |
| --- | --- | --- | --- | --- |
| Private messages | Alice TCP, Bob HTTP | 98 | 7 | Login, contacts, categories, empty HTTP polling, bidirectional DMs and native profile page |
| Private groups | Alice/Dave TCP, Bob/Carol HTTP | 224 | 18 | Creation, participants, invitation, message fanout, leaving and outsider denial |
| Public rooms | Alice/Carol TCP, Bob HTTP | 216 | 27 | Room discovery, occupancy, participants, room text, room isolation and membership enforcement |
| Catalog and services | Alice TCP, Bob HTTP | 203 | 53 | Native pages/forms, pagination, recharge, purchases, gift notes/ratings, entitled downloads, games, room creation, community linking and bot stop |

The [complete evidence summary](runs/refactor-out/evidence-summary.json) records binary/JAR hashes, per-client results and database integrity. Each suite folder contains runtime logs, original packet frames, model dumps and screenshots: [private](runs/refactor-out/private/alice/runtime.log), [groups](runs/refactor-out/groups/alice/runtime.log), [rooms](runs/refactor-out/rooms/alice/runtime.log), [catalog](runs/refactor-out/catalog/alice/runtime.log).

All four databases remain at schema18 with clean integrity and foreign-key checks. Catalog persistence verifies single purchases, the expected remaining credit balance, literal gift-note preservation, the saved rating and stopped bot state. The public catalog still reports78 products and25 game entries.

Only isolated listeners `127.0.0.1:58080` and `127.0.0.1:59119` were used. Older harnesses hardcode `:8080` inside native page-request URLs; those fixtures advertise that origin while the Connector shim redirects every network request to the isolated listener. The catalog fixture advertises its actual `:58080` origin. The main backend/database and earlier acceptance research/applets/v3/artifacts were untouched. All isolated servers and client JVMs were stopped after completion.

The harnesses execute original packet builders, receive dispatchers, UI models, native HTML parser and Canvas renderer. They drive selected original methods through reflection; this is not exhaustive keypad-driven UI testing.
