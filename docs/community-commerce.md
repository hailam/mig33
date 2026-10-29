# Community Groups, credits and store services

These durable local services retain the recovered v4.62 page URLs, controller/action/view selectors and embedded-browser markup contract. Recovered catalog names and visible historical details carry evidence references; unrecovered IDs, artwork, prices and rules use explicit local replacements or placeholders. Payments remain mocked with test credits. Expo and its v3 UI mode are separate deferred work; no surviving production HTML is claimed here.

## Implemented routes

All routes use /sites/index.php, v=midlet, the authenticated gateway sid, and GET to display a page. Mutations use POST, a session CSRF value and the local operation selector documented below. The same handlers serve HTTP and packets 922/923.

| Controller | Actions | Behavior |
| --- | --- | --- |
| group | home, list_groups_home, list_my_groups | Directory, owned/joined/pending Groups, public/private Group creation, overview and join/leave requests |
| group | members | Public membership list or restricted private list; owner/moderator approval and adding/removing members; owner-only promotion/demotion |
| group | list_chatrooms, add_to_group | Link/unlink existing rooms, list links and native room-join links |
| group | view_forum | Topics and one-level replies, latest 100 posts, membership required to post, private content restricted to approved members |
| account | home, get_history | Wallet balance and latest 100 ledger entries, including operation receipts, participants, deltas, post-operation balances and UTC timestamps |
| account | recharge_credit | Explicit local test recharge of 10, 50 or 100 credits; no real payment |
| account | transfer_credit | Select a registered recipient and transfer a positive amount with at most two decimal places |
| store | home, main | Gift/emoticon-pack/theme catalog, purchase, recipient delivery, owned inventory, theme selection and free original-default reset |
| merchant | dashboard | Account ledger and balance; clearly describes the absence of real settlement or merchant commissions |

The original recipient_username and gift-store username parameters are retained. ty=1 filters gifts; ty=5 filters themes. The latter is directly tied to **Change Theme**, localized string 54 at aj.java:4363, rather than inferred from the route name. Store main id=111 remains accepted.

The Group pages use the original positive cid parameter to carry selection across page links. The local aliases group_id and id remain accepted, but duplicate selectors or multiple aliases in one URL are rejected. Bare home lists Groups. Member/forum/linked-room pages need a selected Group. Original add_to_group&roomName=... displays the viewer's eligible Groups for that room.

## Community policy

Social **Groups** use communities, community_members, community_rooms and community_posts. They do not reuse chat_groups or its private conversation membership.

Public Groups allow immediate joining. Private Groups record pending requests until an owner or Group moderator approves them. A nonmember can see a private Group's directory entry and description, but cannot see its member list, forum or linked-room list. An owner cannot leave and orphan the Group. Only the owner promotes/demotes Group moderators; a Group moderator cannot remove an owner or another moderator.

Linking a room requires both ownership of that room and an owner/moderator role in the destination Group. A room belongs to at most one Group. A Group role does **not** implicitly grant room moderation privileges. Linking to a private Group does not silently turn the room private. Explicit linked-Group controls implement bans, lock/silence, invitations, descriptions and immediate announcements/broadcasts, with bounded local bots; their authorization and inferred rules are documented in [room completion](room-completion.md).

Create Group uses a staged name/description form. Group names retain the local1–40-character restricted alphabet; this is separate from the recovered15-character limit for newly created chat rooms. Owners may create up to10 Groups and the approved-member limit is1000. These Group limits are explicit reconstruction choices, not recovered historical values.

Forum posts preserve text, including ampersands, plus signs, percent signs and Unicode. They accept up to 2000 UTF-8 bytes, prohibit empty/whitespace-only content, require approved membership and use an actor-scoped 48-hex nonce. An identical retry returns the original post; a changed payload under the same nonce conflicts. Replies must refer to a root topic in the same Group.

## Credit ledger and store

Credits are integer hundredths of **local test credits**, with no cash value. A wallet starts at zero. Real payment processing, SMS charges, voice billing, exchange rates, cash withdrawals, merchant commissions and settlement are not connected.

Each operation is a single SQLite transaction. Transfers write an equal debit and credit; purchases write the buyer's debit and the recipient's inventory item together. Recharge writes a clearly identified test-credit operation. Amounts must be positive, at most 1,000,000.00 credits per operation; wallet balance is capped at 10,000,000,000.00 credits. Overdrafts are rejected.

credit_operations has a unique actor/nonce pair. Repeating a submitted form returns the same receipt and never duplicates money or inventory. Reusing a nonce for a different kind, amount, item, recipient or gift note conflicts. Concurrent duplicate submissions are covered by a store test.

The runtime catalog contains **78 products:37 gifts,39 emoticon packs and two colour-theme variants**. The [historical evidence catalog](../research/applets/v4/docs/data/historical-product-catalog.json) has74 records covering73 unique names; those records are separate from the runtime product definitions. Product metadata identifies recovered names, source URLs, local category placement, placeholder artwork and local test prices. Original IDs, exact billing and complete downloadable assets remain unavailable. Existing Rose1.00, Birthday cake2.50, Thank you star1.50, Effects pack2.00 and theme3.00 prices remain compatibility fixtures. The additional products use explicit local test prices, never inferred historical charges.

Store listings support `kind=gift|emoticon_pack|theme`, a case-insensitive `query`, zero-based `page` with eight products per page, and `item` detail selection. Recovered-name products expose details and purchase confirmation; the original retained `ty=1`/`ty=5` filters still work. Gift recipients see their items in inventory and receive a native animated action receipt when another account purchases the gift. All37 receipt animations use local atlas-based placeholders. The38 recovered-name emoticon packs each unlock one placeholder token; Effects retains its three media effects. Themes are personal purchases and cannot be bought for another account.

### Gift notes, chat sending and ratings

The [original19 January2011 author guide](https://mig-info.blogspot.com/2011/01/mig33-new-command-gift-on-chat.html) supplies `/gift USER NAME [-m MESSAGE]` and `/gift rate NAME RATING`, alongside catalog discovery. Native chat accepts these in private messages, private group conversations and joined rooms; group/room membership is checked before any mutation. A command's confirmation goes only to the requesting session in its current tab. A gift to another account creates a separate private action receipt for sender and recipient, regardless of the command's originating tab. Self-purchases retain inventory without generating a private self-chat receipt. See [the full command contract](slash-commands.md#gift-commands).

`CreditOperationWithNote` commits the debit, inventory, optional note, gift message and immutable recipient audience in one SQLite transaction. Failure to store any part rolls everything back. The old `CreditOperation` API delegates with an empty note, preserving old receipts and retries. Server handlers check the recipient exists; gift privacy and directional blocks are rechecked inside the debit transaction. The recipient's `chat_privacy` everyone/friends/nobody policy matches direct chat; friends means the recipient has the sender in their contact list. An already committed receipt remains an idempotent replay even if privacy later changes. Later blocking can cancel still-pending delivery through the existing block policy, without rewriting the purchase ledger or owned inventory.

Notes contain at most320 UTF-8 bytes. Control/format characters, invalid UTF-8 and reserved `[gift:`, `[photo:` and `[pack:` triggers are rejected; paid emoticon ownership is checked by both native and web handlers. Notes are appended as literal text (`Message: ...`) without rich-text markup and escaped through templates on Store pages. `GiftNote(operationID, viewer)` returns a note only to that operation's sender or recipient. The owned-gift list and `/gift gifts` show attached notes. Empty historical notes remain absent rows, interpreted as an empty string.

Native purchase retries use the existing native request nonce derived from the session ID plus the entire encoded packet. Exact packet retries in that session return one receipt; a changed packet or new login is a new purchase. This is the available protocol boundary, not a cross-session exactly-once guarantee. A full requester queue rejects the command before mutation; a full or offline recipient queue leaves the committed receipt pending for the normal polling/login delivery path. Native commands spend local test credits only; no payment provider is invoked.

Gift ratings persist one current integer vote per account/item, replacing the previous value. The local scale is1–5; original screenshots establish a decimal average but do not recover the accepted endpoints, repeat-vote policy or full historical aggregate. The item detail page and `/gift display` expose count, mean and the current account's vote. Unrated items report count0/mean0; own rating0 means no vote. Ratings do not require purchasing the gift, debit credits, change inventory or publish chat text. Repeating an identical vote leaves the count unchanged; submitting an older value again replaces the current value. Both `/gift rate` and the CSRF-protected Store rating form share this storage rule.

Schema18 adds only `gift_notes` (operation ID foreign key) and `gift_ratings` (account foreign key, account/item primary key,1–5 check and item index). Existing ledger, inventory, messages and receipts remain unchanged. Migration tests verify a schema17 receipt replays after upgrade without a second debit or message; restart tests cover notes, rating counts and current votes.

Public `GET /api/catalog` returns this static product metadata with token definitions,25 game entries, the original English pack and mock-provider modes. It does not reveal inventory, balances or accounts and cannot make purchases. See the [API fields and units](backend.md#public-catalog-metadata).

Selecting a purchased theme requires ownership and persists in store_preferences. The server sends a **complete opcode1000** packet built from the recovered23,718-byte default_v2 resource. Its two local variants change only identifiers0/2 and proven chat colours139/142–145; all other bytes remain unchanged. **Use original default theme** selects `item=default` through the same `op=theme` form, requires no purchase and restores the unmodified original resource. The native1000 receiver persists the theme and clears rendering caches itself. Selection/reset reaches the account's native sessions and restores on login. A full native destination queue rejects selection before changing saved state; browser queues are excluded.

## Embedded forms

The original browser submits hidden values first and only replaces spaces with %20. Forms therefore put one unrestricted text field last. Their native parser preserves its raw tail rather than treating typed ampersands, plus or percent signs as form syntax. Hex carries selected names and room identifiers. Credit amounts and fixed fields use restricted alphabets.

Form actions are absolute local URLs. Query separators remain literal ampersands in attributes because the original parser does not decode attribute entities. Text nodes are HTML escaped. No JavaScript, modern CSS or external payment redirects are required.

Local POST operations are:

| Service | op |
| --- | --- |
| Group creation | create, create-details |
| Membership | join, leave, approve, remove, promote, demote, member |
| Room linking | link, unlink |
| Forum | post |
| Credit transfer | recipient, transfer |
| Test recharge | recharge |
| Store | purchase, theme, rate |

## Validation and remaining evidence

Storage tests cover private access, pending approval, owner protection, room ownership, exclusive linking, forum retry/cross-Group isolation, concurrent credit retries, overdraft rejection, ledger balance, gift duplication and theme ownership. Gift tests additionally exercise audience-insertion rollback, cross-connection block/purchase races, note validation and authorization, changed-note conflicts, legacy empty-note replay, schema17 migration, restart and one-vote aggregation. Server tests cover native punctuation parsing, strict amounts, CSRF, recipient binding, escaped page content, original-style action URLs, full theme preservation and theme delivery/restoration, plus gift routing, requester-only confirmations, membership/privacy rejection and full/offline recipient delivery.

These tests establish Go handler/storage behavior and the recovered packet shape. They do not establish historical production markup, prices, policy limits or exact rendered page appearance. Surviving screenshots and archive evidence belong in the design research catalog; native screenshot acceptance should remain separate from these tests.
