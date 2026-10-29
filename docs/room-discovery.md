# Room discovery, creation and favorites

The completed local administration, filtered search, categories, deletion, Group-room commands and bot contract are documented in [room completion](room-completion.md). This extends the earlier native room behavior described below.

The original v4.62 client now has working **Room Info**, **Search Rooms**, **Create Room**, **Save to Favorites** and **Remove from Favorites** flows. They use the original PHP-shaped page URLs and native room packets. The server HTML, creation limits and ownership rules are functional replacements; the historical downloaded pages were not recovered, so their labels and layout are not claimed to be pixel-identical historical content.

Room definitions and favorites persist in SQLite schema version7. Presence and joined-room membership remain session state. The existing Lobby, Gaming and Help definitions retain their50-person capacity and existing moderator assignments; this migration does not assign them an owner or resize them.

## Preserved page routes

All pages require an authenticated gateway session. Ordinary HTTP uses the `sid` header; the original client also reaches the same handlers through packet922 and receives packet923. Form actions are absolute URLs on the configured local origin. Ordinary POSTs require `application/x-www-form-urlencoded`; their body is limited to8,192 bytes.

| Route | Methods | Behavior |
| --- | --- | --- |
| `/sites/index.php?c=chatroom&v=midlet&a=setup&roomName=Lobby` | GET, POST | GET shows room information without joining. Joined moderators also receive mute/unmute forms for present nonmoderators. POST applies authorized moderation; see [room moderation](rooms.md#room-info-moderation-forms). |
| `/sites/index.php?c=chatroom&a=search&v=midlet` | GET, POST | GET opens the search form. POST submits the `query` field; GET with one `query` parameter also returns results. |
| `/sites/index.php?c=chatroom&a=create&v=midlet` | GET, POST | GET opens the name step. Two validated POST steps create the public room and its creator's moderator role. |

The route selectors and original feature paths remain unchanged. `stage=name`, `stage=details`, the form fields, and the optional search `query` parameter are replacement form contracts, not recovered historical payloads.

Room Info does not edit or delete a room, change ownership, grant a role or join the viewer. GET has no side effects and its occupancy is a snapshot. Joined moderators can separately submit its member mute/unmute forms. These replacement form contracts and the original binary commands are documented in the [room guide](rooms.md).

## Searching and joining

Search accepts up to128 UTF-8 bytes after trimming outer whitespace. Invalid UTF-8 and NUL characters are rejected. It performs a literal substring search across names and descriptions with ASCII case folding. `%`, `_` and backslashes have no wildcard meaning. Empty search text lists rooms. Results are sorted by name and bounded to100; a full result set displays a prompt to narrow the search.

Each result displays its current occupancy/capacity and offers **Join Room** and **Room Info**. The original browser executes this join link:

```text
mig33:joinChatroom(Test Room)
```

The argument reaches the original703 builder verbatim. Spaces must remain literal: `Test%20Room` would request that literal percent-containing name. The renderer only emits a join command for names within the supported creation alphabet, preventing imported punctuation from becoming command syntax. Info links use the absolute preserved setup URL with its query value URL-escaped normally.

Activating Join Room follows the same membership, capacity and message behavior as joining from the native room browser. Merely opening a result or information page does not join it. The new search form starts empty after each search and shows the previous query as escaped page text; this avoids the original parser's failure to decode entities inside input-value attributes.

## Creating a room

Creation is deliberately split into two forms because the original serializer does not generally URL-encode free text:

1. **Name step:** GET shows title `Create Room`. Its POST action adds `stage=name`. Fields are hidden `csrf`, then free-text `name`; the unnamed submit button is **Continue**. The server validates the session token and normalizes the name, but does not create anything yet.
2. **Description step:** the response retains title `Create Room` and shows the selected name. Its POST action adds `stage=details`. Named controls are hidden `csrf`, hidden `room`, hidden `binding`, then free-text `description`; the unnamed submit button is **Create Room**. `room` contains the hexadecimal UTF-8 bytes of the normalized name. `binding` is an HMAC-SHA256 over the selected name and creation context using the session secret. Changing the name, binding or session fails validation.
3. **Saved room:** the second POST atomically creates the room and its creator's moderator assignment, then returns `Room Created` with Join Room and Room Info links. It does not automatically join or favorite the room.

Local creation rules are:

- A newly created name is1–15 ASCII characters after trimming outer whitespace, matching the recovered Create Room illustration. Existing definitions retain lookup/edit/join compatibility through40 characters. Allowed characters are letters, digits, spaces, underscores or hyphens. Name lookup and uniqueness ignore ASCII case; the first stored spelling remains canonical.
- Description is optional, may contain punctuation and Unicode, and is bounded to1,000 UTF-8 bytes. Invalid UTF-8 and NUL are rejected.
- New rooms are public, placed in global category1, and have capacity50 unique users. Each account may own at most10 created rooms.
- The authenticated session supplies the owner. The form cannot nominate a different owner or moderator. Granting the initial moderator role and saving the definition are one transaction.
- An identical retry by the same owner returns the existing room. A collision owned by someone else, a seeded room name, or different description returns409. A retry never restores a creator role that a trusted operator subsequently revoked.

New-room limits, capacity and automatic creator moderation are explicit local policy. Editing, owner deletion, capacity adjustment, ownership transfer, persistent bans and operator categories are now implemented in [room completion](room-completion.md). Historical migLevel-dependent creation privileges remain unrecovered; Group-linked locks use explicit local rules.

The name and search inputs use native full-width `size="10"`. The description uses `rows="4" cols="10"`, the recovered full-width textarea sentinel. Ordinary browsers also receive name/query `maxlength` attributes; the original parser ignores them, so the server enforces the actual limits.

## Native form encoding

Each form has exactly one free-text field, placed last. For packet922 submissions, the adapter parses the fixed prefix and treats the remainder after `name=`, `description=` or `query=` as one value. Only `%20` becomes a space. Ampersands, plus signs, equals signs, percent characters and Unicode inside the final value remain text rather than becoming extra controls. Hidden controls are checked for duplication, unknown names and expected cardinality. The original serializer may omit an empty free-text field: an omitted description is accepted as empty, and an empty search body lists all rooms. An empty name cannot create a room.

A literal `%20` typed into the original JAR cannot be distinguished from its encoded space and is therefore received as a space. Standard HTTP form encoding can preserve it by encoding the percent sign. This is an original serializer limitation, also described for [Report Abuse](reports.md#original-native-form-encoding).

| Page response | Meaning |
| --- | --- |
|200| Form, search results, room information or successful/idempotent creation. |
|400| Invalid fields, name, description, query or creation stage. |
|401| Missing or expired authenticated session. |
|403| Invalid session token or name/context binding. |
|404| Unknown room or route. |
|405| Unsupported method; setup is read-only. |
|409| Room name already belongs to an incompatible creation. |
|415| Ordinary HTTP POST lacks the supported form content type. |
|429| The account already owns10 created rooms. |
|503| Storage could not load or save the requested state. |

## Native Favorites contract

Favorites are private bookmarks belonging to the authenticated account. They do not move the room out of the public catalog, grant access or moderator authority, or change membership. Removing a favorite does not delete its room.

| Request | Fields | Result |
| --- | --- | --- |
|713| None | Category714 for global Chat rooms (`1:S1`) and personal Favorites (`1:S2`), then715. |
|716| Category `1:S1` or `1:S2`; flag `2:B0/1` | Correlated701 rows followed by717. Category2 selects only the requesting account's favorites. |
|711| Room `1:T` | If this session has not received the favorite row, return correlated701 with category `6:S2`;701 completes the original pending711 and inserts the row. If this session already knows the row, return ordinary ACK1 instead, avoiding the original insertion branch's duplicate-row behavior. |
|712| Category `1:S2`, room `2:T` | Remove the account's bookmark and return correlated ACK1. Repeated removal of an existing room is idempotent. A different category or unknown room is rejected. |

Favorites have a100-room limit per account. Store insertion and quota checks are transactional. Names resolve to their canonical room spelling. Limit failures and other native operation failures use opcode0 rather than a false success acknowledgment.

Category714 flags3/4/5 are `1/1/0` for Chat rooms and `1/1/1` for Favorites. Flag4 supplies the initial collapsed state, subject to the client's saved expansion preference. Flag5 identifies favorite rows so their context menu offers Remove from Favorites. Flag3 makes the original next-page fetch replace prior category rows instead of appending them; both categories return full snapshots. Their717 footer is labeled Refresh; activating it clears the old rows before fetching another snapshot, preventing duplicate rows. Category2 is reserved for per-user Favorites. New rooms default to category1; operators may create categories3+ and assign existing rooms.

The server tracks which favorite rows it has returned in responses for each session. A category2 listing refreshes that set,713 resets it, and712 removes the matching known row. Thus an explicit Save from another instance can insert the row even if the account already saved it elsewhere; a repeated Save in a client that already has it does not create a duplicate.

The requesting client updates its current favorite row through the original response handlers. Other active instances of the same account receive no invented favorites push; refresh/reopen the category to request716 again, or log in again. Saving a room from a conversation still leaves that conversation open. Category browsing does not join a room.

## Persistence and verification

Schema version7 adds room `owner`/`created_at` fields and the account-scoped `room_favorites` table. Migration preserves previous messages, audiences, contact state, blocks, reports, room capacities, descriptions, moderator roles and mutes. Stored room ownership and favorites survive restart; sessions and participation remain temporary.

Go coverage includes durable restart behavior, cross-account favorite isolation, idempotent create/save/remove, canonical names, quota races, literal search and bounds, transactional role-creation rollback, migration preservation/rollback, native form encoding, server authorization and renderer escaping/control order. `go test -race ./...` and `go vet ./...` passed. Original code evidence and protocol constants are separate from these implementation tests.

The [native acceptance guide](../research/acceptance/v4/ROOM_DISCOVERY.md) and [recorded result](../research/acceptance/v4/room-discovery-result.md) document the passing `run-room-discovery.sh` sequence. Three unchanged v4.62 JVM phases captured40 screenshots and127 binary frames, each completing original300 logout with its correlated acknowledgment:

- Hailam over TCP creates a room through both forms, joins through the literal-space command link, inspects Room Info and saves the favorite. Repeated Save completes without a duplicate row. The native Refresh footer is exercised once for Favorites and twice for the global category, with no duplicated rows.
- Bob over HTTP sees his own empty Favorites, searches with blank and exact queries, joins the created room and opens its information page.
- A fresh Hailam client over HTTP loads the persisted favorite, removes it through the native context menu, and verifies that the global room remains.

The creation textarea, information page, search results and Favorites menu were visually inspected. A read-only SQLite check confirmed the exact punctuation-bearing description, Hailam's ownership and moderator role, capacity50, and no remaining favorites after removal. The harness selects original menus and executes original form submission handlers, while assigning text through parsed control models rather than simulating handset typing. Go persistence tests, rather than a fresh JVM alone, establish server-restart durability.
