# Native group chat flow in mig33 4.62

This note describes private group conversations (`500` conversation type `2`), not public chat rooms (`3`) or the website's community/group pages. Wire behavior comes from the original class instructions; the normal-flow source references below are reading aids that omit exception handlers. Replacement-server policy is labeled separately.

## Requests and responses

| Request | Fields | Success |
|---|---|---|
|751 create from private chat|1:T existing private peer,2:T new invitee,3:B external-service ID (1 for native mig33)|750 with matching transaction|
|752 invite to existing group|1:T group ID,2:T invitee,3:B service|1 acknowledgment with matching transaction|
|753 leave|1:T group ID,2:B service|1 acknowledgment with matching transaction|
|754 participants|1:T group ID,2:B service|755 with matching transaction|
|500 group text|1:B1 native channel,2:T claimed sender,3:B2,4:T group ID,6:S1 text,8:T body,13?:I color|1 acknowledgment; incoming500 to other participants|

Builders: `bc.java` normal-flow694–704,859–872,1025–1054. UI invite caller: `aj.java` normal-flow3698, original `aj.txt`4704. The backend must take the sender from the authenticated session, not trust outgoing500 tag2.

`750` reads1:T group ID,2?:T old private peer,3?:B service(default1). With a pending transaction, the client ignores response2 and takes the old peer from the original request's tag1. With no matching request, it takes response2. It maps the service to a message channel, finds a private conversation for that peer, changes its group ID and conversation type, and clears the private-contact object. **It creates no tab when no matching private conversation exists.** Evidence: `bc.java` normal-flow2848–2878; `k.java`1043–1077; `s.java`1249–1267.

Thus a reliable sequence for creator Alice, existing peer Bob and invitee Carol is:

1. Return750 to Alice with the751 transaction,1:T new group ID,3:B1.
2. Push750 to Bob with transaction0,1:T group ID,2:T `alice`,3:B1, to convert Bob's existing conversation if present.
3. Push500 describing the group creation/join to Alice, Bob and Carol. This creates a missing group tab, including Carol's. No750 is needed for Carol; sending one could unnecessarily convert an unrelated existing private conversation.

Deliver Alice's750 before her first group500. Reversing that order may create a new group tab and subsequently convert the old private tab into a second tab for the same group. A normal752 invitation requires no750 for the inviter, who already has the group ID; a500 notification can create the new invitee's tab.

## Group message and system notification rendering

Incoming500 with3:B2 and4:T group ID finds or creates a native group conversation/tab4. Sender2:T equal to4:T group ID selects informational style3 and replaces the visible sender label with localized string135, `Group Chat`. Use6:S1 for ordinary text system notifications. The body8:T is arbitrary server text; no historical join/leave sentence is recoverable from this client.

A minimal compatible system notification is therefore500 transaction0 with1:B1,2:T group ID,3:B2,4:T group ID,6:S1,8:T a clearly chosen replacement-server message. Omit9 unless intentionally requesting its special marker style; sender==group ID already selects the informational style. Numeric colors12/13 are optional. Evidence: `bc.java` normal-flow2510–2554; `k.java`1203–1373.

Ordinary group messages use2:T actual authenticated sender. The client optimistically appends its outgoing text before acknowledgment, so normal live fanout should not echo the same text to the sending connection. Other sessions belonging to that same user may still need it. Offline history replay is a separate policy and needs a cursor to avoid duplicate display.

## Participant lists and leaving

755 uses2:T all participants,3?:T administrators,4?:T muted users and5?:B service(default1). Each list is semicolon-terminated: `alice;bob;carol;`, including a delimiter after the last member. The renderer calls `indexOf(';')` for each element and does not handle an unterminated tail. Empty optional lists may be omitted or empty. Native private groups need no invented administrator/muted membership roles;2 and5 suffice.

755 is consumed only when its transaction matches a pending request. It populates a temporary participant/action selector, then removes that pending request. An unsolicited755 does not refresh an open selector. Let the client ask754 again; use500 for membership notices. Evidence: `bc.java` normal-flow2653–2748; UI requests in `aj.java`1240–1247,1642–1649,3507.

On leave, the UI records the group ID in a temporary ignore list, sends753, and immediately closes its local tab. Incoming group500 for an ignored ID is discarded. Either success or failure removes the ID from that ignore list. **Do not send a leave notification back to the leaving connection after its acknowledgment**, because the next group500 would create the tab again. Notify remaining members only. Evidence: `aj.java`3567–3580; `bc.java`1360–1364,1570–1573,2510–2513.

Errors use0 with2:T readable error text and the request transaction.751/752 errors display a dialog;753 failure does not restore the closed tab. The success response for752 is a plain1 acknowledgment. No group-invitation accept/decline opcode is used by these flows.

## Replacement-server policy recommendation

These choices are implementation policy, not claims about the original service:

- Authorize every invite, participant read, message and leave against stored membership. Limit the implementation to native service/channel1 initially; reject unsupported external IM service IDs explicitly.
- Create751 with the authenticated creator plus two distinct, registered users. Existing group752 invitations are idempotent when the invitee is already a member. Persist membership changes before acknowledging them.
- Keep membership separate from online presence. Inviting an offline registered account may succeed; durable group messages/notifications can be delivered after login. Disconnecting does not implicitly leave a persistent group.
- Rehydrate groups on login with a group500 notification or pending group message;750 alone cannot create a missing tab. Never replay an old correlated750 transaction from another connection.
- Store group messages once with group ID, authenticated sender and a durable sequence. Fan out to current members, enforce per-session delivery cursors and bounded queues, and prevent leavers from receiving later traffic/history they are no longer authorized to read.
- Participant755 lists membership, not merely currently online sessions. The packet does not offer a native per-member online/offline array.

Public-room operations703–719 and community/group HTML routes are independent features and should not be used to implement private group chat.
