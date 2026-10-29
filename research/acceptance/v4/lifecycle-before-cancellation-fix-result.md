# Lifecycle acceptance before delivery-cancellation correction: PASS

Validated 2026-10-29 against an isolated Go 1.27.3 `cmd/omni` binary and the unchanged original mig33 4.62 publisher JAR. Six harnesses passed across **18 client sessions, 1,353 captured frames and 249 Canvas screenshots**. Every client received its correlated logout acknowledgment.

| Suite | Clients | Frames | Screenshots | Verified behavior |
| --- | --- | --- | --- | --- |
| Private messages | Alice TCP, Bob HTTP | 98 | 7 | Login, contacts, categories, HTTP polling, bidirectional DMs and native profile page |
| Private groups | Alice/Dave TCP, Bob/Carol HTTP | 225 | 18 | Creation, participants, invitation, message fanout, leaving and outsider denial |
| Public rooms | Alice/Carol TCP, Bob HTTP | 215 | 27 | Discovery, occupancy, participants, room text, isolation and membership enforcement |
| Embedded pages and text color | Alice TCP, Bob HTTP | 272 | 104 | All 52 literal templates, original forms/parser, color picker, message colors, purchases and theme selection |
| Catalog and services | Alice TCP, Bob HTTP | 203 | 53 | Pagination, recharge, purchases, gift notes/ratings, entitled downloads, games and bot lifecycle |
| Vote kicking and reports | Hailam/Carol/Dave/Alice TCP, Bob HTTP | 340 | 40 | Native abuse-report form, distinct votes, duplicate suppression, three-vote kick, moderator kick and post-kick restrictions |

The [complete evidence summary](runs/lifecycle-before-cancellation-fix-out/evidence-summary.json) records per-client results and binary/JAR hashes. The tested binary SHA-256 is `5fe1127cc8e942c77ad42883447d319c1345147efff84e4ce71927a6540faf7c`.

Runtime logs, original frames, models and screenshots are retained for [private messages](runs/lifecycle-before-cancellation-fix-out/private/alice/runtime.log), [groups](runs/lifecycle-before-cancellation-fix-out/groups/alice/runtime.log), [rooms](runs/lifecycle-before-cancellation-fix-out/rooms/alice/runtime.log), [pages](runs/lifecycle-before-cancellation-fix-out/pages/index.html), [catalog](runs/lifecycle-before-cancellation-fix-out/catalog/index.html), and [votes/reports](runs/lifecycle-before-cancellation-fix-out/vote-report/hailam/runtime.log). The page suite covered all 52 template IDs and passed its independent screenshot audit. All six fresh SQLite databases remain at schema 18 with clean integrity and foreign-key checks. Catalog checks confirm 78 products, 25 games and [expected purchases, balances, gift notes, ratings and stopped bot state](runs/lifecycle-before-cancellation-fix-out/catalog/persistence-proof.json).

Each suite used a fresh database and isolated listeners `127.0.0.1:58080` and `127.0.0.1:59119`. The older harnesses advertised their embedded `:8080` page origin while the Connector shim redirected all network requests to isolated ports; catalog advertised `:58080`. Compilation and harness output ran in a new temporary tree. The main backend, interactive clients, main database and previous evidence were untouched. All isolated servers and JVM runners exited; both isolated listener ports were confirmed free afterward.

The harnesses execute original packet builders, dispatchers, UI models, HTML parsing, forms and Canvas rendering through selected reflective calls. They do not exhaustively simulate keypad interaction or establish historical pixel identity for reconstructed server pages. Lifecycle shutdown ordering, context cancellation and forced HTTP draining have separate Go regression tests; successful native scenarios do not exhaust every cancellation timing.
