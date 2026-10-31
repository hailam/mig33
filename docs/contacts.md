# Local contact management

The original v4 **Add a Friend** flow now creates a saved address-book entry. Lists belong to the signed-in user and persist in SQLite, alongside contact groups, display-name overrides, optional mobile metadata, revision/checkpoint data and deletion records. These are directed address books: adding Bob does not add you to Bob's list or create an acceptance request. That relationship policy is a reconstruction choice; the recovered v4 native flow supplies contact operations, not a confirmed friend-request approval exchange.

`hailam` starts with an empty **Friends** group. Alice, Bob, Carol and Dave retain their former demo starter contacts when their saved lists are first initialized. Initialization happens once: removing a contact or an empty group survives restarting the server. Accounts use persistent registration; the five documented local credentials are initial seeds.

## Supported original operations

| Request | Behavior | Response |
| --- | --- | --- |
|400|Complete saved contact/group list|401 groups,402 contacts,420 checkpoint,403 end|
|419,1:L checkpoint|Contact edits/deletions since checkpoint; refresh current contact presence/status|402/406 changes,404/421 live state,420/403|
|405,12?:T username,2?:I group,3?:T display,8?:T mobile,24?:B share-mobile,30?:T external username,31?:B service|Add a known mig33 user, phone-only entry or supported external identity; duplicate add preserves metadata|402 with matching transaction|
|406,1:I contact|Remove from your own list|1 acknowledgment;406 to your other sessions|
|407,1:I contact,2?:I group,3?:T display,8?:T mobile,12?:T same username,30?:TA external names,31?:X services|Edit metadata or aligned external identities; changing between mig33/phone/IM kinds requires remove/add|402 with matching transaction|
|408,2:T name|Create a contact group|401 with matching transaction|
|409,1:I group|Delete an empty group; move/remove its contacts first|1 acknowledgment|
|410,1:I group,2:T name|Rename a group|1 acknowledgment|
|411,1:I contact,2?:I group|Move contact; omitted group means ungrouped|1 acknowledgment|

Successful mutations also return an unsolicited420 checkpoint after their native response. Contact/group IDs are scoped to the owner. For current mig33 accounts, contact IDs use stable account IDs. Group IDs increase within each user's list and are not reused. Phone/IM contact IDs allocate downwards from the positive signed-int maximum, skipping saved contact and current account IDs. Removed local IDs are not reused. Display/mobile edits are private address-book metadata; they do not edit the other account's profile. The share-mobile preference is stored, but this prototype has no verified phone profile to distribute.

Unknown users, self-addition, invalid groups and malformed fields return the original error packet0. Phone-only entries accept a plus-prefixed number with 4–15 digits. External identities support service IDs2 (MSN),3 (AIM),4 (Yahoo),6 (Google Talk), matching the local descriptor registry. These entries remain offline; saving them never connects to or sends data to an external provider. Removing a contact does not delete chat history, block the person or prevent a new direct message. Personal blocks remain separate and survive contact removal/re-addition. Messages from users outside the recipient's address book omit the contact-ID field so the original client can use the sender username.

## Synchronization boundaries

Each effective mutation advances a persisted revision and a strictly increasing millisecond checkpoint. Login203 advertises the current revision. The original client requests400 when its cached revision differs, or419 when it matches. Deletions are retained for delta synchronization; re-adding the same user removes their tombstone. Presence and status are refreshed on419 even if no persistent contact record changed.

Pending contact notifications are delivered before a newer mutation response on the same session. A full snapshot supersedes queued roster notifications, so obsolete rows cannot become part of the refreshed list. Transport responses for a session are serialized so an older queued add cannot arrive after a newer remove and restore the removed row.

Contact402/406 updates and newly created groups are pushed to other active sessions of the same account. The recovered client has **no incoming group rename/delete operation**;920 does not reset its contact cache. Other already-open sessions therefore retain their old checkpoint after such changes and need to sign in again for a full400 refresh. An explicit419 request that spans group changes receives an error requesting that refresh. The session that performed the rename/delete updates immediately through the original acknowledgment handler. A full control queue similarly leaves the session requiring a refresh rather than advancing its checkpoint past missing events.

## Try it with two clients

Start the server, then open two independent login windows:

```sh
go run ./cmd/omni
# In another terminal:
sh scripts/open-clients.sh
```

Sign into one as `hailam` / `hailam`, and the other as `bob` / `bob-demo`. In Hailam's Friends menu, choose the Add a Friend action and enter `bob`. Bob appears in Hailam's list, and either client can send a direct message. You can also join the same public room in both windows.

To explicitly sign in automatically, use `sh scripts/open-clients.sh hailam bob`. No arguments always leaves both signed out. Each client has isolated in-memory RMS/settings, its own session and trace directory, and a distinct window title. The pair builds the harness once, places the windows side by side, checks both become ready, and closes both child processes on Ctrl+C. Closing both windows also ends the launcher. A single window remains available through `sh scripts/open-client.sh`.

Backend changes persist in `data/chat.sqlite`. Client settings reset on each launch. The original JAR remains unchanged and the transport adapter permits only local TCP9119/HTTP8080. Desktop controls are arrows/Enter and F1/F2, or the clickable phone keypad; Mac function keys may require Fn.

## Verification

The [original-client acceptance report](../research/acceptance/v4/contact-report.md) records Add form submission, remove/re-add, contact editing, group management, automatic419 synchronization and two-way messaging through two unchanged v4 clients. Store/server tests cover restart persistence, schema migration, owner isolation, invalid inputs, rollback, monotonic checkpoints, transient presence refresh and ordering across concurrent sessions.

## Phone and external contact representation

Schema15 adds contact kind and external identity arrays without changing existing contact IDs or metadata. Phone-only402 records contain1:I ID, optional2:I group,3:T display and8:T mobile, with **no12**. External402 records also omit12 and instead carry31:X service IDs,32:X matching presence99 bytes and33:TA NUL-separated usernames. A mig33 record takes the original12/13 branch; mixing both branches is rejected. The initial208 descriptor registry precedes the first402 records so the original parser can resolve the service IDs.

405 carries one external name/service pair.407 carries equally sized arrays, with at most one identity per supported service. Arrays are stored in service order and emitted together. Malformed, duplicate-service or unsupported-service arrays fail before storage. Group moves, edits, deletion and owner isolation apply equally to all kinds. A duplicate add returns the saved row, retaining its original display/group preferences.

## Authoritative contact snapshot for the replacement frontend

`GET /api/contacts` requires the gateway `sid` header. It returns JSON `revision`, `checkpoint`, `groups` and `contacts`. Contact fields are `id`, `group_id`, `kind`, optional `username`, `display`, optional `mobile`, `share_mobile`, `presence`, optional `status` and optional `external` entries (`service`, `username`, `presence`). `group_id=-1` means ungrouped. Groups contain `id` and `name`. External presence is99. Results belong exclusively to the authenticated account; private address-book mobile metadata is never shared between owners.

New clients replace their entire saved group/contact collections from this snapshot; absence removes an old row. Polling this endpoint therefore converges rename/delete changes across clients without a logout. Presence/status can change without a persistent revision change, so a revision equality does not replace live-state refresh.

The unmodified v4 client still needs reconnect for another session's group rename/delete. Original`bc`401 dispatch calls`au.a(t,...)`, whose group branch **inserts** a header without replacing an existing ID; pushing401 would duplicate it. There is no incoming409/410 case, and920 only refreshes named views. The source session's correlated ACK already performs its local rename/delete correctly. This is an original-client capability boundary, not a missing server mutation.
