# Controller/service/repository native acceptance: PASS

Validated 2026-10-29 against an isolated `cmd/omni` binary using the unchanged original mig33 4.62 publisher JAR. Five harnesses passed across **13 client sessions, 1,025 captured frames and 209 Canvas screenshots**. Every successful client session received its correlated logout acknowledgment.

| Suite | Clients | Frames | Screenshots | Verified behavior |
| --- | --- | --- | --- | --- |
| Private messages | Alice TCP, Bob HTTP | 99 | 7 | Login, contacts, categories, empty HTTP polling, bidirectional DMs and native profile page |
| Private groups | Alice/Dave TCP, Bob/Carol HTTP | 234 | 18 | Creation, participants, invitation, message fanout, leaving and outsider denial |
| Public rooms | Alice/Carol TCP, Bob HTTP | 217 | 27 | Discovery, occupancy, participants, room text, isolation and membership enforcement |
| Embedded pages and text color | Alice TCP, Bob HTTP | 272 | 104 | All 52 literal page templates, original forms and parser, color picker, native message color, purchases and theme selection |
| Catalog and services | Alice TCP, Bob HTTP | 203 | 53 | Store pagination, recharge, purchase confirmation, gift notes/ratings, entitled downloads, games, room creation, community linking and bot stop |

The [complete evidence summary](runs/controllers-out/evidence-summary.json) records the tested binary/JAR hashes, per-client results, totals and database checks. Each suite contains runtime logs, original packets, native model dumps and screenshots: [private messages](runs/controllers-out/private/alice/runtime.log), [groups](runs/controllers-out/groups/alice/runtime.log), [rooms](runs/controllers-out/rooms/alice/runtime.log), [native page gallery](runs/controllers-out/pages/index.html), [catalog gallery](runs/controllers-out/catalog/index.html). All five fresh databases remain at schema 18 with clean integrity and foreign-key checks.

The page suite verified all 52 template IDs and independently audited all 104 screenshots, with no blank page captures. The catalog suite verified 78 products and 25 game entries. Its [persistence checks](runs/controllers-out/catalog/persistence-proof.json) confirm single purchases, the expected credit balance, literal gift-note preservation, saved rating and stopped bot state.

An initial page run exposed an outdated test fixture: the runner generated a 20-character room name while the backend correctly enforces the existing 15-character limit. That response and its captures are retained in [the initial failure](runs/controllers-out/pages-initial-fixture-failure/alice/runtime.log); they are excluded from the successful-run totals above. The isolated rerun used a valid 14-character name. The permanent runner now generates `P` plus the full `YYYYMMDDHHMMSS` timestamp, exactly 15 characters. No production behavior was changed for this fixture correction.

Only isolated listeners `127.0.0.1:58080` and `127.0.0.1:59119` and fresh fixture databases were used. Older harnesses embed `:8080` in page-request URLs, so those fixtures advertised that origin while the Connector shim redirected network traffic to the isolated listeners. Catalog advertised its actual `:58080` origin. Harness compilation and output ran in a separate temporary tree; the main backend, interactive clients, database and prior acceptance evidence were untouched. All isolated server and harness processes exited, and both isolated listener ports were confirmed free afterward.

These harnesses execute the original packet builders, receive dispatchers, UI models, HTML parser and Canvas renderer. They invoke selected original methods through reflection; they do not exhaustively drive every keypad interaction. Native page compatibility does not establish that reconstructed replacement pages reproduce unavailable historical server designs.
