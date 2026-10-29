# Public chat rooms and moderation

The completed local administration, filtered search, categories, deletion, Group-room commands and bot contract are documented in [room completion](room-completion.md). This extends the earlier native room behavior described below.

The current priority is direct messages and public chat rooms. These are distinct from invitation-based private group conversations and from the historical website's community Groups pages. The backend retains the group-chat work while implementing public rooms through their original v4.62 protocol operations.

## Room flow

The persistent local room catalog contains **Lobby**, **Gaming** and **Help**, in category1, each with capacity50 unique users. Room names resolve case-insensitively and responses use their canonical spelling. These are local fixture rooms; they are not claimed to be the historical server's catalog.

Twenty-five was not a universal historical room limit. A [September6,2009 firsthand community report](https://mig33-news.blogspot.com/2009/09/mig33-chat-room-user-limit.html) describes newly created rooms allowing30 users while older public/private rooms retained25. A [February20,2011 community guide](https://mig-world.blogspot.com/2011/02/chatting-and-making-friends_20.html) says higher creator migLevels allowed larger rooms, without giving numerical tiers. These sources establish larger rooms, not a universal50-user policy. The recovered client displays the server's701 occupancy4/capacity3 integers and parses the full708 participant list; neither path imposes25. The26-tab limit is separate from room membership.

| Request | Contract |
| --- | --- |
| `713` | Two `714` categories followed by `715`: global Chat rooms1 and per-account Favorites2; both start collapsed, with Favorites marked by tag5=1 |
| `716` | `1:S category`, `2:B flag`; category1 lists public rooms, category2 lists this account's favorites; returns `701` rows with room1, capacity3, unique-user occupancy4 and category6, followed by `717` |
| `711` | `1:T room`; persist favorite; correlated701 in category2 if this session does not know the row, otherwise ACK1 avoids duplicate native rows |
| `712` | `1:S category=2`, `2:T room`; remove only this account's favorite and acknowledge; room and live membership remain |
| `703` | `1:T room`; returns correlated `701`, then opcode1 acknowledgment, then a room information500 |
| `704` | `1:T room`; remove this session's subscription and acknowledge |
| `707` | `1:T room`; require membership; return correlated `708` with semicolon-terminated participant2, moderator3 and muted4 lists, service5=1 |
| `500` | Channel1, conversation3, room in4, content type1, text8; require current room membership and an unmuted sender |

The original client needs both701 and its following acknowledgment to finish joining.701 creates the native room tab; the acknowledgment marks it active and clears the pending703. Repeated joins are idempotent. Participants and occupancy count usernames once even when a user joins from multiple sessions.

Participation belongs to each session. A second session of the same account receives room messages only if it joins too. Leaving removes only that session; the user remains in the participant list while another of their sessions is joined. TCP disconnect, logout and session expiry remove subscriptions. HTTP has no persistent socket whose close proves logout; it uses explicit300 or session expiry. Other joined rooms and direct messaging continue independently.

Rooms send live messages to current participants and do not replay missed room traffic offline. The server requires a703 join in each new session. During a transient reconnect, the original client automatically sends703 for retained room tabs; a fresh launch without those tabs has nothing to rejoin. See the [original reconnect evidence](core-flow-audit.md#reconnect-behavior-an-important-documentation-correction). Text is still stored with its audience for authorized history. `/api/chat/history?kind=room&target=Lobby` requires the requesting session to be joined, and returns only messages addressed to that account while it participated. A newcomer does not receive earlier room messages. A slow participant with no available queue in any joined session causes a new room send to fail before committing; the sender can retry.

The original **Search Rooms**, **Create Room** and **Room Info** routes now work. Search finds literal substrings in names and descriptions; native page links join results. Creation collects a validated name, then an optional description, saves the room and grants its creator moderator rights. Info shows description, owner, moderators and occupancy/capacity; its GET has no side effects. Joined moderators also receive member mute/unmute forms. New rooms use capacity50 and a10-room ownership limit per account. Favorites are persistent bookmarks, limited to100 per account; they do not automatically join rooms. See [room discovery contracts and limits](room-discovery.md).

## Moderator roles

Alice is seeded as a moderator in all three local rooms. Bob, Carol and Dave start as regular users. Roles and mutes persist in SQLite, and seeding happens only in the migration, so later operator changes survive restart.

The original room menus expose Kick User to ordinary users too. The same706 request now starts or joins a vote for a regular user and immediately kicks for a stored moderator. Both caller and target must be present in the selected room. Neither path permits targeting oneself or another moderator. Roles are room-specific.

| Action | Original request | Behavior |
| --- | --- | --- |
| Kick | `706`, room1 and username2 | Moderator: remove target immediately. Regular user: start/join a vote; a successful vote removes the target. Both remove every session from this room, cancel queued room text and notify the target through native packet5 |
| Mute | `709`, room1 and username2 | Persist a room-specific mute; reject subsequent room sends before storing them; keep reading/receiving available |
| Unmute | `710`, room1 and username2 | Remove that room's mute and restore sending |
| View roles | `707 →708` | The original participant popup marks moderators from3 and muted users from4 |

A kicked user stays logged in, retains other rooms and can rejoin. A mute survives leave/rejoin and server restarts; it does not block direct messages or other rooms. The message transaction checks the mute again before insertion, so operator changes cannot be bypassed by a concurrent send.

The recovered build has no incoming room-only forced-close opcode. Kick uses its supported generic notification5 and removes server membership. The old room tab may remain visible; subsequent sends are rejected until rejoin. The server does not invent an opcode702 or disconnect the user's entire account. The ordinary704 leave flow closes the tab through the original client UI.

The stock visible **Block & Mute User** action is a separate personal-block flow: its confirmation sends417 with a username and no room identifier. That flow is now implemented, including persistent blocks and the original Manage Block List URL for unblocking. It hides that user's incoming text only for the blocker; other participants and moderator authority remain unaffected. The709 room-moderation flow is verified through the retained original request builder/dispatcher; the visible personal-block action does not invoke it. The already-muted participant path does send710. See [personal blocking](blocking.md) and the [moderation integration scope](../research/acceptance/v4/ROOM_MODERATION.md).

Trusted local operators can grant/revoke roles with the database-backed CLI. These commands exit after changing the role and do not start listeners:

```sh
go run ./cmd/cli -db data/chat.sqlite -room Lobby -room-moderator bob
go run ./cmd/cli -db data/chat.sqlite -room Lobby -room-moderator bob -revoke
```

The username must be one of the configured accounts. This operator action is not exposed as a client HTTP/binary endpoint. Granting a role clears any existing mute on that user in the room. There is no application super-admin role; operators can revoke a moderator role when necessary. Existing participant popups refresh their role markers on the next707 request.

## Room Info moderation forms

Use **joined room → Menu → Room Info → Room Moderation**. A joined moderator sees one **Mute username** or **Unmute username** form per present nonmoderator. Ordinary participants and moderators viewing a room they have not joined see information only. Self and other moderators are not listed as targets. The unchanged applet can render and submit these forms; they are reconstructed HTML, not recovered historical layout.

GET `/sites/index.php?c=chatroom&a=setup&v=midlet&roomName=Lobby` has no side effects. POST to the same URL requires exactly these fields:

| Field | Meaning |
| --- | --- |
| `csrf` | Current session's form token |
| `room` | Hex-encoded canonical room name, matching the URL room |
| `binding` | HMAC binding the room and moderation purpose to this session |
| `target` | Present member's canonical username |
| `operation` | `mute` or `unmute` |

Both HTTP and922/923 call the existing709/710 moderation service. The form is not authority: the server rechecks the caller's current room membership and stored moderator role, the target's membership, and protection of self/other moderators. Store changes recheck roles transactionally. The authenticated caller always supplies the actor identity; form fields cannot impersonate a moderator.

POST requires authentication, an8,192-byte maximum body and, for direct HTTP, `application/x-www-form-urlencoded`. These native forms contain only fixed safe hidden values, so normal form decoding is sufficient. Invalid fields/operation return400; unauthorized, expired or altered context returns403; stale/invalid targets or rejected service actions return409. Successful/repeated desired-state submissions return200 with updated controls and confirmation. A repeated mute does not emit another room notification.

Mute/unmute changes take effect immediately and notify the room when state changes. Native participant markers refresh on the next707 request; an unsolicited708 has no effect without a matching pending request. Mutes persist across reconnects and restarts. They do not block the target's reading, private messages or other rooms. Ordinary **Block & Mute User** continues to invoke personal417. The marked-muted participant branch can also unmute via native710.

Evidence: [server forms](../internal/adapters/legacyweb/room_management.go), [authorization and state tests](../internal/acceptance/room_management_test.go), [applet inference](../research/applets/shared/backend-inferences.md), and [original-client acceptance](../research/acceptance/v4/FRIENDS_MODERATION.md).

## Validation and scope

### Regular-user vote-kicking

The original v4 room flow is **room tab → Menu → Kick User → choose participant → confirm**. The room menu factory `aj.U()` adds action104 without checking the current user's moderator role. Its participant action opens confirmation31; confirming sends706 with room1 and target2. There is no separate recovered Vote Kick menu label or dedicated voting opcode. These observations establish the client action; a server can interpret the same request as either a vote or an immediate moderator kick. Source: `research/applets/v4/artifacts/normal-flow/aj.java` lines2954,3803,7319; original bytecode remains authoritative.

Historical first-person [April2007 user discussion](https://www.howardforums.com/threads/mig33-new-version-released-instant-messenger-for-j2me-mobiles.1138507/) describes ordinary users voting to remove someone and nine votes in a25-user room. The implementation follows the occupancy bands in a [2010 community guide](https://amcomig33.blogspot.com/2010/08/tips-1.html), with totals including the initiator:

| Unique active users when voting starts | Total votes required |
| --- | ---: |
| 1–8 |3|
| 9–11 |4|
| 12–14 |5|
| 15–17 |6|
| 18–20 |7|
| 21–24 |8|
| 25 |9|
| More than25 |9 — local fallback; no verified historical rule recovered |

A [2011 community guide](https://koharesse.wordpress.com/2011/05/11/mig33-tips-trick-2/) instead puts20 users in the eight-vote tier. We deliberately use the2010 table above; this boundary is not claimed as verified4.62 server behavior. The historical table stops at25 despite evidence of larger rooms. Our larger fixture rooms retain the last documented threshold of nine as an explicit fallback, not an inferred historical formula.

The local server now implements the following explicit reconstruction policy:

- **The required count depends on occupancy when the ballot starts**, following the table above. Count distinct active accounts joined to that room, including the target, initiator and any moderators or muted members; multiple sessions of one account count once. Other rooms, unauthenticated sessions and expired sessions do not contribute. Room capacity is not occupancy.
- **The count is fixed for that ballot.** Later joins/leaves affect who can vote and which votes remain, but do not change the required total. A new ballot recalculates it. This snapshot behavior is a local choice because the historical sources do not establish how mid-vote occupancy changes were handled.
- Use the same Kick User action to join an existing vote. A room of up to eight users still needs three distinct current voters, so testing successful removal requires at least four accounts including the target. Starting below that size is allowed; enough eligible voters must join before expiry to finish.
- Each room has at most one active ballot, lasting **60 seconds**. Progress and expiry appear as native room system-message500 text. A repeated vote from the same account, even in another session, does not increment the count.
- A muted user cannot vote. Leaving one's last session in the room withdraws the vote; the ballot is canceled if no votes remain. The target leaving or becoming a moderator cancels the ballot.
- Starting a vote has a **60-second cooldown per initiator per room**, measured from its start. Voting in someone else's ballot is still allowed. Ballots are temporary and disappear on server restart.
- At the threshold, the target loses all sessions' membership in that room and receives the native notification popup. Other rooms and login remain active. Immediate rejoin is permitted; this is not a ban. A moderator can still kick immediately.

The table is community-documented history; the disputed boundary, fallback above25, fixed ballot threshold, timers and eligibility rules remain explicit reconstruction choices. There is no credit charge. The original706 request and supported500/5 responses are retained. Every progress, duplicate, departure and completion notice uses the ballot's stored required count.

### Report Abuse

The room's **Menu → Report Abuse** action opens `/sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=Chatroom&subject_id=<room>`. It reports the room context; the private-group variant uses `subject=Group%20Chat` and its group ID. The authenticated replacement form now saves a durable report with an optional participant target, category and description, and returns a report reference. A local operator can list, inspect, review or dismiss reports through the CLI. Reporting does not automatically remove anyone. See [report fields, endpoints and review commands](reports.md). The form content is reconstructed; the URL and embedded browser are original.

Go tests cover native room packet ordering, capacity, canonical names, unique-user occupancy, per-session isolation, multi-session leave/disconnect/kick, queued-message cancellation, room history access, backpressure, mute enforcement, vote thresholds/deduplication/departure/expiry, protected targets, persistent roles/mutes, report authorization/replay/review and migration rollback/data preservation. Occupancy tests exercise each band boundary through actual706 requests, including the26/30/50-user fallback; joins/leaves during a ballot, a fresh ballot after cancellation, duplicate/expired/other-room sessions, moderator occupancy and immediate moderator removal are covered separately. Extra accounts exist only in isolated Go test servers; the interactive demo still has five configured identities.

With the backend running locally:

```sh
sh tools/applets/v4/integration/run-room.sh
sh tools/applets/v4/integration/run-room-moderation.sh
sh tools/applets/v4/integration/run-vote-report.sh
```

The [original-client room guide](../research/acceptance/v4/ROOMS.md) records room browsing, occupancy, joining, participant popups, messaging, isolation and ordinary leave behavior. The moderation runner separately exercises the unchanged original client's moderator/mute markers, permission errors, mute/unmute and kick notification. These harnesses use original builders, serializers, dispatchers and Canvas rendering, driven through reflection; they do not prove every keypad action or historical server policy.

Passing captured evidence is in the [room acceptance report](../research/acceptance/v4/room-report.md) and [moderation acceptance report](../research/acceptance/v4/room-moderation-report.md). The [five-client vote/report guide](../research/acceptance/v4/VOTE_REPORT.md) covers three ordinary voters, the target, a moderator and the native report form. Earlier moderation captures predate ordinary-user voting; current permission tests target protected moderators.

Room editing/deletion, persistent bans, categories and moderation audit are now implemented in [room completion](room-completion.md). Automatic content moderation and external network providers remain outside the local contract. Room creation and favorites711/712 also work. Kick/mute notices, room names and role policies are explicit reconstruction choices; historical server-side policy was not recovered from the JAR.
