# Private and group chat implementation

This backend implements the recovered v4.62 direct-message, private-group and public-room flows. This guide focuses on direct/private-group persistence; the [room guide](rooms.md) covers room membership, live traffic and moderation. Existing gateway and feature-page URLs are unchanged. The JSON endpoints below are new reconstruction APIs for the future frontend; they are not attributed to the old mig33 service.

## Native packet contract

All requests use the original binary envelope and transaction correlation described in [the protocol reference](../research/applets/v4/docs/protocol.md). The session supplies the authenticated sender; a client cannot impersonate another user through message field2.

| Request | Required fields | Result |
| --- | --- | --- |
| `500` private text | `1:B1`, `3:B1`, `4:T recipient`, `6:S1`, `8:T body`; optional `13:I color` | Commit message/audience, then `1` acknowledgment; recipient receives `500` with trusted sender and sender's contact ID5 |
| `500` group text | `1:B1`, `3:B2`, `4:T groupID`, `6:S1`, `8:T body`; optional `13:I color` | Require current membership; commit audience snapshot; deliver to every other member |
| `751` create | `1:T existingPeer`, `2:T invitee`, `3:B1` | Create a group containing caller, peer and invitee; correlated `750` carries group ID1 and service3 |
| `752` invite | `1:T groupID`, `2:T invitee`, `3:B1` | Require membership; add invitee atomically; `1` acknowledgment |
| `753` leave | `1:T groupID`, `2:B1` | Remove caller and cancel pending group deliveries atomically; `1` acknowledgment |
| `754` participants | `1:T groupID`, `2:B1` | Require membership; correlated `755` with `2:T alice;bob;...;` and `5:B1` |

Text must be valid UTF-8, non-whitespace, and at most 65,536 bytes. The original UI's editor has a smaller limit; the server limit also bounds other compatible clients. Ordinary messages keep their original body/color. Supported [slash commands](slash-commands.md) store the expanded action body, set `action=true` and use incoming content6 without explicit color. Action results also return to their author, whose native client suppressed local echo; raw slash commands are not broadcast. Unsupported channels/content types, unknown recipients, outsiders and storage failures produce opcode0 errors rather than successful acknowledgments.

Any current group member may invite another registered account. The creator has no special administration privileges and may leave while the group continues. Creation needs three distinct accounts. Inviting an existing member is idempotent. There are at most50 members per group. Offline invitees are allowed; their membership is restored after their next login/contact synchronization. An empty group has no members who can send or invite.

The original750 handler converts an existing private tab; it does not always create a new group tab. The server therefore returns the creator's750 before a group information500, sends the old peer an unsolicited750 with the correct old-peer field, and sends group information messages to other members. Information messages use sender=groupID, which selects the original localized Group Chat label. Their wording is newly written local-server policy. [Original code audit](../research/applets/v4/docs/data/group-flow-notes.md).

The participant755 response must match a pending754 and use a trailing semicolon for every username. It is requested on demand, not pushed as an unsolicited participant list. The leave path does not send a new group message to the departing account, since that would reopen its closed tab. Buffered deliveries are canceled and checked again at the transport boundary, including a fast leave/rejoin.

## Persistence and delivery

The default database is `data/chat.sqlite`, configurable with `-db`. SQLite uses transactional schema migrations through version18, foreign keys, WAL, synchronous FULL and a busy timeout. Version2 adds room definitions/history while preserving existing message IDs/audiences; version3 adds moderator roles and mutes; version4 adds personal blocks; version5 adds saved contact lists/groups, checkpoints and deletion records; version6 adds abuse reports and their review history; version7 adds room ownership and per-account favorites; version8 adds the action-message flag without changing old message bodies, IDs or audiences. Versions9–12 add persistent identities, community/commerce services, social/media records and photo references, and room settings/bans/change logs. Version13 adds room categories, deletion tombstones, Group controls/bots and moderation audit preservation;14 reserves a migration slot;15 adds operator review and phone/external contacts;16 adds message request keys and private read positions;17 adds optional rich-text source with an empty default for existing messages;18 adds gift notes and one current gift rating per account/item. The schema12→16 migration check preserved every preexisting row in all33 original tables; evidence is in `data/backups/chat-v16-migration-proof.json`. The pinned [Go SQLite driver](https://github.com/mattn/go-sqlite3) uses CGO. New database files are created with mode0600 and new parent directories with0700. The development database directory is excluded from Git.

The store contains:

- `chat_groups` and `group_members`: durable conversation IDs and current memberships.
- `messages`: immutable IDs, conversation type/target, authenticated sender, text, optional color, rich source and timestamp.
- `message_audience`: immutable per-message access grants, recipient delivery state and leave cancellation state.
- `chat_rooms`, `room_moderators`, `room_mutes`: public-room definitions and persistent room-specific permissions.
- `room_favorites`: saved public rooms for each account, independent of live membership.
- `personal_blocks`: directed, persistent per-user blocks.
- `abuse_reports` and `abuse_report_reviews`: durable reports and operator review records; see [report handling](reports.md).

Saving a message and its audience is one transaction. For a group, membership authorization and the audience snapshot occur inside that same transaction. The acknowledgment is sent only after commit. A disk/storage failure is reported to the sender.

Personal blocks remove blocked senders from the receiving user's new delivery/history audience without affecting other recipients. Blocking cancels old undelivered text; unblocking permits future text and does not replay canceled messages. Stored earlier history remains available. A private send can therefore be acknowledged and remain in the sender's history while the blocked recipient receives nothing. [Detailed personal-block behavior](blocking.md).

Each live session has a bounded128-packet queue. A full queue or offline recipient leaves private/group messages in the database; polling, successful writes and reconnects continue draining them. Merely reading a pending message or putting it in memory does not mark it delivered. TCP encoding/write and HTTP write/flush must succeed before delivery state advances. Failed private/group writes preserve the pending message. Sessions are temporary and clients log in again after a restart; group restoration and offline delivery occur after contact synchronization. Group-restoration notices now resume from a per-session queue as transport space becomes available, rechecking membership before enqueueing. Tests cover257 groups with full/partial queues and cross-session leaves. They remain transient control notices, so failed writes do not acquire new durable receipt guarantees; the native26-tab display limit also remains separate. See the [corrected queue defect](core-flow-audit.md#3-a-full-queue-can-truncate-private-group-restoration). Room messages have durable history but no offline replay. Each new session must send703 to join; the original client does this automatically for retained room tabs during transient reconnects, while a fresh launch has no retained tabs.

The old protocol has no client receipt for individual incoming messages. A successful server transport write is not proof of application display or reading. A crash between a write and its delivery-state update can cause a replay; a native client retry can also create another message because historical transaction IDs are connection-local, not durable idempotency keys. The JSON send API adds persistent idempotency keys, while the read API stores an account-private checkpoint. Neither invents native read receipts.

Ordinary sends now copy to the sender's other already authenticated native sessions. The originating applet keeps its local echo. Private copies use native content6 with `alice: original text` in the body and the peer as the routing field, because incoming private500 has no outbound-message flag. Those copies use the native action color; stored history, recipient packets and JSON keep the original body/RGB. Private-group copies keep ordinary sender/style. Pending copies survive full queues and failed writes for the session's lifetime; fresh logins fetch old own messages through the JSON history API rather than replaying them into native tabs. Room copies reach the sender's other sessions that have joined that room and retain live-only room semantics.

Leaving cancels still-pending group delivery, including messages sent before the leave, to prevent the old UI from reopening that group. Historical access remains for messages the account was entitled to when they were sent. A later invitation does not grant older messages or messages sent while the account was absent. System join/leave/restoration notices are transient; group membership and user-written text are durable.

## Reconstruction chat APIs

All endpoints require the authenticated gateway session in the `sid` header and return JSON with `Cache-Control: no-store`. POST requests also require `Content-Type: application/json` and the `X-CSRF-Token` returned by the session endpoint. They do not replace original binary login or any old URL.

| Endpoint | Parameters / response |
| --- | --- |
| `GET /api/chat/session` | Returns authenticated `username` and `csrf_token` |
| `GET /api/chat/history` | Required `kind=private`, `group` or `room`, and `target`. Optional `after_id` (default0), `limit` (default50,1–200). Returns ascending `messages`, `next_after_id`, `has_more`, and `read_through_id`; room history requires this session to be joined |
| `GET /api/chat/groups` | Returns `groups` containing only the caller's current memberships |
| `GET /api/chat/sync` | Optional `after_id` and `limit` as above. Returns audience-authorized private/group records across conversations, with `next_after_id` and `has_more`. Clients merge by immutable message `id`; public rooms use their membership-protected history endpoint |
| `POST /api/chat/messages` | JSON `kind`, `target`, `body`; optional RGB integer `color`, owned `photo_id` instead of body, or mutually exclusive HTML-subset `markup`. Requires a48-character lowercase hexadecimal `Idempotency-Key`. Supports ordinary text/photos and core action commands. Room administration uses its separate controls |
| `POST /api/chat/read` | JSON `kind`, `target`, `through_id`. Advances only this account's conversation read position, monotonically; the referenced message must belong to that exact conversation and audience. No receipt is broadcast to peers |

The first successful send returns201 with `{message, replayed:false}`. Repeating the same account/key and normalized payload returns200 with the same stored message and `replayed:true`, including after a process restart. Reusing the key for a different payload returns409. Message/audience/idempotency records commit in one transaction. Private peer names are normalized to lowercase before hashing. Slash randomness is evaluated only for the first request. Polling/history retrieval does not mark a message read; the client must explicitly advance its own read position. This contract provides a stable foundation for later Expo chat state without adding fields to legacy packets.

```sh
curl -H "sid: YOUR_GATEWAY_SESSION" \
  'http://127.0.0.1:8080/api/chat/history?kind=private&target=bob&after_id=0&limit=50'
```

A history message has `id`, `kind` (1/2/3), `conversation` (stored destination/group ID/room name), `sender`, `body`, optional `color` and `markup`, and UTC `created_at`. Private history includes both directions with the selected peer. Group results have `id`, `creator` and `members`. See the Go model tags for exact JSON fields.

History is filtered by the immutable audience of each message. An outsider cannot read another private conversation or group. A former member can read previously addressed messages but cannot list the group as a current membership or receive new traffic. A current new member may see an empty historical result until messages are addressed to them. Unauthenticated requests return401; malformed queries400; inaccessible groups404; unsupported methods405.

The original JAR has no recovered general history-fetch operation. It receives queued messages through500 but does not automatically load all previously delivered history through these new JSON endpoints. The future frontend can use them explicitly.

## Formatted WAP chat

The retained historical `/member2/t.php` surface now accepts local HTML-subset formatting for direct, private-group and public-room conversations. Rich source and plain fallback commit with audience and request keys; JSON history/sync retains the optional `markup`. Native recipients receive plain text with a single RGB only when the text has one uniform explicit colour. Mixed formatting stays on the WAP surface. Browser sessions read history instead of consuming native queues. Plain WAP core actions are idempotent; room administration uses separate controls. See the [complete authoring, authentication and form contract](text-and-page-formatting.md#implemented-formatted-wap-chat).

The live schema16→17 migration preserved every original row/column value in41 tables, including18 messages and five accounts; existing messages have empty markup. The backup and comparison hashes are recorded in `data/backups/chat-v17-migration-proof.json`.

## Verification and remaining scope

`go test -race ./...` covers membership/authorization, message snapshots, private history in both directions, offline/restart delivery, backpressure, failed HTTP write/flush retry, transactional rollback,50-member concurrency, and cancellation across leave/rejoin. The original-client [private acceptance test](../research/acceptance/v4/README.md) and [four-member group test](../research/acceptance/v4/GROUPS.md) execute unchanged original serializers, transports, receive dispatchers and renderers with captured screenshots and binary frames.

Registration creates persistent accounts beyond the five seeds. Media, local service requests, dynamic emoticons and Group-room controls are documented in their service guides. External providers, historical catalog fidelity, transcript search/retention policy, the Expo frontend and Classic v3 presentation remain separate. Public rooms and moderation are documented in the [room guide](rooms.md).

The completion tests also cover150 queued own-session mirrors, failed-write retry without duplicated local echo, persistent send replay/conflict across restart, private transcript isolation, pagination and monotonic private read positions. Original-client own-DM/group routing and offline phone/external contacts passed in the [three-client native evidence](../research/acceptance/v4/mirrors-result.md).
