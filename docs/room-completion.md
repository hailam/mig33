# Room administration and Group-room command completion

Implemented 2026-10-27. This is the local reconstruction contract. Existing source evidence establishes the native slash transport and linked Group context, not the exact historical server rules, wording, limits or game catalog.

## Discovery, categories and deletion

Room Search accepts `query`, `language` and `keywords` query parameters. `query` is a literal case-insensitive substring of name or description; `language` is an exact case-insensitive language name; `keywords` is a literal substring of the saved keywords. Filters combine with AND. `%`, `_` and backslash are ordinary characters. Default language is English. Limit is 100 rooms. Native forms change one field at a time using `field=query|language|keywords`; the other filters travel in the action URL. Text inputs remain empty because the original parser does not decode attribute entities. Current filters display as escaped page text.

The operator dashboard manages stored categories and assigns rooms. Category1 is the default and cannot be deleted; category2 is the native account-specific Favorites view and cannot be stored. Operator-created categories start at3. Names are unique, at most80 bytes; the local cap is30 categories. Deleting a category requires moving/deleting its rooms first. Native713/714/715 and716/701/717 list these categories and their rooms.

Room owners can delete their own rooms from Room Settings by entering the exact room name. Role, mute, ban, settings, favorite and Group-link rows are removed in one transaction. The server drops live membership, cancels buffered room messages and ballots, and sends notification5 to retained native tabs. Historical messages, game results and moderation audit remain. A permanent name tombstone prevents a new room acquiring the previous room's name-based history. Seed rooms without an owner cannot be deleted through this form.

Schema13 preserves existing audit rows while removing their room foreign key, so audit survives definition deletion. The newest100 events are shown in Room Settings; stored events are not limited to100. Native kick, mute/unmute, role changes and vote start/cast/pass/withdraw/cancel are recorded. Timer/cleanup operations still complete if an audit write fails and log the failure.

## Explicit Group-room command scope

The administration and bot-play commands require a currently joined room linked to a community Group. They do not apply to private group conversations or unlinked public rooms. The invoking account must be that Group's owner or moderator, except `/play` and `/scores`. A room-only moderator does not automatically receive Group administration rights. Read-only `/games` is also available to ordinary participants in any joined public room and returns its catalog only to the requester.

| Command | Local behavior |
| --- | --- |
| `/silence [on|off]`, `/unsilence` | Stop ordinary users from sending in the current room; room and Group moderators can still speak. Reading/joining remains available. |
| `/lock [on|off]`, `/unlock` | Restrict new entry to approved Group members, its moderators, or room moderators. Existing visitors remain; use a ban for removal. Private Group visibility does not automatically lock linked rooms. |
| `/announce text` | Send an identified room notice to the current room. |
| `/broadcast text` | Send an identified notice to every room currently linked to the Group. |
| `/description text` | Change the current linked room's description. |
| `/ban username`, `/unban username` | Persist a Group-wide room ban; ban removes every target session from every linked room and prevents later entry. Group owners/moderators and any linked-room owner/moderator are protected from a new ban. Existing bans may be lifted after promotion. |
| `/invite username` | Save a local Mail invitation. Recipient privacy and personal blocks apply. No membership is granted and room locks/bans remain enforced. |
| `/bot dice`, `/bot 8ball` | Start one local replacement bot in this room, lasting at most30 minutes or20 successful rounds. Starting over an active bot fails. |
| `/bot stop`, `/botstop` | Stop the current room bot. The two-word spelling is visible in an original migGames help capture; the existing local alias remains supported. |
| `/games` | Privately list playable local replacements and historical reference entries. No game starts, message is stored or credits charged. |
| `/play` | Any unmuted participant plays one active dice round: uniform integer1–6. |
| `/play question` | Any unmuted participant asks the active8ball bot; question limit320 bytes, replacement response bank. |
| `/scores` | Show the current room's top20 dice totals, ranked by summed points then username; totals and round counts persist. |

Commands are case-insensitive; bot names are lowercase. Text fields are bounded to1000 UTF-8 bytes unless stated. Announcements/control responses are privileged moderator notices using native room-information messages; their durable record is the moderation audit. Personal blocks do not hide these administrative notices. Ordinary player results use actor-tagged native content6 and are filtered by personal blocks again at transport write. Bot results are stored separately with their chosen result, points, account and time. A partial historical catalog is now recovered; complete original mechanics and ranking formulas remain unknown.

## Recovered game catalog and explicit placeholders

The [original author's game guide](https://mig33-news.blogspot.com/p/mig33-games.html) names20 games. A [mig33-issued November2010 announcement](https://www.prnewswire.com/news-releases/mobile-community-mig33-launches-game-developer-program-announces-new-social-game-partnerships-106642023.html) adds partner games Moonbase, Club Wars and Kooky Pets. The [evidence catalog](../research/applets/v4/docs/data/games-history.json) records these23 names and17 unchanged community screenshots, including original migGames help with `/games`, `/bot <game>` and `/bot stop`. Captures identify a historical design family, not an exact4.62 server response. Paid/free reports conflict for some games and are never used as billing rules.

The retained Games route `/sites/index.php?c=bot&a=bot_list&v=midlet` now lists two playable local replacements and23 clearly unavailable historical references. The local `game=ID` query opens details. Reference entries have no play form, cannot be submitted as a play target, and never charge credits or create results. Existing Dice Roll and Magic8Ball forms preserve their targets, CSRF and durable nonce behavior. The historical Dice reference is separate from these replacement mechanics; named partner games do not silently launch an unrelated game.

Queues are checked before side effects. A full joined-client queue rejects a control command so it can be retried; no mutation or successful audit is written. Native500 contains no durable request key, so resubmitting a successful command is a new request. `/play` therefore draws again only for a new accepted command. Core `/me`, emotes, `/roll`, `/fortune` and `/8ball` retain their existing message/history behavior. `/kick`, `/mute` and `/unmute` still direct users to native moderation controls rather than changing ambiguous historical slash semantics.

Unlinking a room deletes its Group-specific silence/lock/bot controls. Group bans remain scoped to that Group and do not govern the unlinked public room. Relinking starts with default room controls.

## Multiple sessions and validation

Ordinary room text goes to the sender's other joined sessions as well as recipients. The originating session keeps its original local echo without an incoming duplicate. Room traffic remains live: full secondary session queues may require history recovery, and leaving/reconnecting does not replay missed traffic.

Tests cover literal/combined search, category reservations and nonempty deletion, owner-only room deletion with tombstones/audit retention, persistence/unlink behavior, Group command permissions and scope, privacy-aware invitations, cross-room ban/rejoin, lock/silence, bot bounds and scores, full-queue failure without mutation, native moderation audit, and same-account room delivery.

Native retained-tab reconnect now also [passes](../research/acceptance/v4/reconnect-result.md):29 screenshots,312 frames, normal/full/banned/kicked cases and three logout acknowledgments.
