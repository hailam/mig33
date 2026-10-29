# Backend behavior inferred from the original v4 client

Audit date: **2026-10-27**. Reference: **mig33 Java ME 4.62 build .000**, original JAR SHA-256 `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

The applet establishes more than packet layouts: its navigation, pending-request handling, local echo and rendering impose requirements on a compatible backend. It does not contain the historical server's database rules, downloaded HTML pages or moderation policy. This note separates three kinds of finding:

- **Confirmed client contract:** directly visible in the original instruction bytes.
- **Backend implication:** an implementation consequence or likely intent inferred from that contract.
- **Unresolved policy:** behavior the client cannot establish, which must remain a documented reconstruction choice.

The [normal-flow sources](../v4/artifacts/normal-flow/) are readable analysis derivatives. Their exception tables were removed to permit decompilation; they are not runnable replacements and do not establish catch/finally behavior. The [original bytecode](../v4/artifacts/bytecode/) is authoritative. Bytecode offsets below are local to the named method. Existing field catalogs remain in [the v4 protocol](../v4/docs/protocol.md) and [endpoint catalog](../v4/docs/endpoints.md).

## Findings that unlock missing native flows

### 1. A downloaded page can start an offline private conversation

**Confirmed:** the embedded browser recognizes this exact link:

```html
<a href="mig33:privateChat(bob)">Chat</a>
```

`aj.a(String,bl)` strips `mig33:`, takes the command name before `(` and the argument between the parentheses, then dispatches `privateChat` to `aj.a(argument,(byte)1)`. That helper creates a temporary contact and opens/focuses a private conversation through `k.a(t,int,int)`. It does not look up the recipient's current roster presence and does not require the recipient to be in the address book.

Evidence: [aj.java](../v4/artifacts/normal-flow/aj.java), lines5172–5177,5250,5361 and5673–5686. In [aj.txt](../v4/artifacts/bytecode/aj.txt), `a(String,bl)` offsets13–54 parse the argument;157–164 and681–686 dispatch it; `a(String,byte)` offsets0–72 build and open the conversation. The main-menu search action opens `/sites/index.php?c=profile&a=search&v=midlet` at normal-flow3905, `commandAction` bytecode10019.

**Backend implication:** a replacement Profile page with a Chat link supplies a genuine native offline-DM entry. Friend Search can link to profiles or directly to Chat. There is no need to alter an offline contact's reported presence or redirect the Email feature. This resolves the earlier audit's statement that no general offline entry had been established; the original Friends primary action still selects Send Email when presence is99. These actions are now implemented in [friend discovery](../../../docs/friend-discovery.md).

**Limits:** this command sends no packet until a message is sent, and opening a tab is not delivery confirmation. Existing block checks, registered-recipient validation and durable message delivery still apply when500 is sent. The native total-tab limit still applies. This confirms a supported navigation mechanism, not the historical Profile page's exact layout or its former privacy rules.

### 2. `inviteFriend` opens Add a Friend; it is not an invitation API

**Confirmed:** the following page link opens the native **Add a Friend** form, state57, prefilled with the argument:

```html
<a href="mig33:inviteFriend(bob)">Add Friend</a>
```

Evidence: [aj.java](../v4/artifacts/normal-flow/aj.java), lines5245–5247 and5724–5729. In [aj.txt](../v4/artifacts/bytecode/aj.txt), `a(String,bl)` offsets141–156 call `m(String)`; `m(String)` offsets0–41 construct the state57 form. The user then confirms the existing405 contact flow. The similarly named `inviteToMig33` command opens the separate state23 mobile invitation form; outgoing900 takes a mobile number and name.

**Backend implication:** directory results can offer native Add Friend without inventing a new web mutation or silently changing contact membership. The link itself does not add a contact or send an external invitation. Its name is not evidence of a reciprocal friend-request/accept protocol.

**Argument constraint for both links:** the command parser performs substring operations, with no URL decoding. `privateChat(bob%20smith)` would pass the percent text literally. Emit links only for usernames that fit the command grammar and HTML attributes; the present fixture names do. Do not treat an arbitrary user-supplied URL argument as proof of identity or authorization.

### 3. Room menus can expose backend moderation pages

**Confirmed:** incoming911 adds a server-defined menu entry. For a room-chat menu it uses `1:I=11`, `2:I=action`, `3:T=label`, `4:T=URL`. The receiver appends it to `aj.aB`, and `aj.U()` includes that vector in the room menu. Selecting a chat-menu row with a non-null auxiliary URL appends context:

| Conversation | Literal suffix appended by the applet |
| --- | --- |
| Public room | `roomName=<room name>` |
| Private group | `gcid=<group ID>` |
| Private chat | `imType=<service>&cid=<contact ID>` |

The URL prefix must supply its own `?` or `&`; the applet concatenates the suffix and does not URL-encode the room name here.

Evidence: [bc.java](../v4/artifacts/normal-flow/bc.java), lines2907–2944; [original911 instructions](../v4/docs/data/protocol-opcode-911.txt), offsets4450–4576 for construction and room-vector insertion. [aj.java](../v4/artifacts/normal-flow/aj.java), lines3334–3350 and7231–7337, shows suffix construction and menu inclusion.

**Backend implication:** a labeled replacement moderation page can be reached through the unchanged applet. Alternatively, the existing Room Info page can link to such controls. A menu entry is not a grant of moderator authority: permissions and membership must be checked at submission. The client's raw concatenation supports the conservative room-name alphabet already used by local room creation; unrestricted imported names need additional compatibility work.

The current implementation uses the existing **Room Info** route for authenticated moderator forms; it does not require911 to open these controls. See [room moderation](../../../docs/rooms.md) and [native friend/moderation acceptance](../../acceptance/v4/FRIENDS_MODERATION.md). This uses the recovered navigation surface with explicitly new form payloads and local authorization rules.

**Limits:**911 is an append operation with no deduplication in this receiver. Repeated delivery can duplicate menu rows. It does not execute an arbitrary moderation opcode. The visible stock “Block & Mute User” action uses personal417; the retained709 room-mute builder and710 unmute are separate contracts. See [moderation boundaries](../../../docs/target-and-gaps.md#compatibility-boundaries-that-affect-requirements).

## Further backend consequences found in packet and UI handling

### Participant markers require a requested refresh

**Confirmed:** the shared708/755 branch first looks up the outstanding transaction and ignores the packet if none exists. The branch then builds a selector from the current UI action, not a persistent participant model. Tags2,3 and4 are respectively participants, administrators and muted users, using semicolon-terminated strings. The final item also needs its semicolon. Administrator styling is applied after muted styling and can override it for a user listed in both sets.

Evidence: [bc.java](../v4/artifacts/normal-flow/bc.java), lines2653–2743; [original708 instructions](../v4/docs/data/protocol-opcode-708.txt), offsets6088–6099 for the pending-request check and6173 onward for list parsing.

**Backend implication:** unsolicited708 is not a reliable way to refresh native role/mute markers after web moderation. Enforce permission changes immediately on the server; the next actual707 request supplies current markers. A page can say that the operation succeeded without claiming an already-open selector was refreshed. The original client cannot establish push-based participant synchronization for a future frontend.

### Slash-prefixed text suppresses local echo

**Confirmed:** Send trims the text, ignores an empty result, and adds its own local transcript line only when the result does **not** begin with `/`. It still submits slash-prefixed text through the ordinary500 builder. Ordinary text therefore already appears in the sending client before the server acknowledges it.

Evidence: [aj.java](../v4/artifacts/normal-flow/aj.java), lines3093–3134; [aj.txt](../v4/artifacts/bytecode/aj.txt), `commandAction` offsets17354–17452 skip local echo for `/`, followed by the regular send paths. The successful500 ACK branch in [bc.java](../v4/artifacts/normal-flow/bc.java), lines1498–1535, has special photo/mail/SMS behavior but does not add a second ordinary-text echo. Error0 can add contextual chat feedback using the failed request.

**Backend implication:** ordinary chat should avoid sending the same plain message back to its originating native client. Slash text suggests an expectation of server command results or transformed messages. The former raw-pass-through policy left the sender without its own slash line. The implementation now expands eleven researched commands, stores their generated action and returns it to the sender as incoming content6. Unknown or unavailable commands fail explicitly without broadcasting raw text. [Command research and local choices](../../../docs/slash-commands.md).

**Unresolved:** the binary does not enumerate `/kick`, `/me`, `/mute` or their syntax, permissions, fees or output. The presence of the slash branch proves none of those individual commands. The follow-up research establishes core command names; templates, roll bounds and remaining moderation context are still documented local choices or evidence gaps. Do not infer read receipts or exact-once delivery from ACK1.

### Native input limits and message timestamps are client behavior

**Confirmed:** the native chat editor is constructed with a maximum of320 Java characters, and Send trims its content. This does not establish a historical backend byte limit. Current local storage accepts larger UTF-8 messages; that is a separate service limit, not evidence that this applet could compose them.

The rendered conversation records `System.currentTimeMillis()` and `Calendar.getInstance()` after appending a message. Its paint path can add a time separator after more than300,000 milliseconds of inactivity, using that locally saved calendar. Incoming500 has no established message ID, original server timestamp or read-receipt tag in the recovered schema. Tag14 is passed into the renderer as an extra string that changes name-prefix layout; it must not be relabeled a timestamp without further evidence.

Evidence: [aj.java](../v4/artifacts/normal-flow/aj.java), line6700 and3093; [aj.txt](../v4/artifacts/bytecode/aj.txt), initialization offsets1204–1214 construct `ce` with320. [s.java](../v4/artifacts/normal-flow/s.java), lines515–517,628–731 and987–989; [k.java](../v4/artifacts/normal-flow/k.java), lines1205–1360; [original500 instructions](../v4/docs/data/protocol-opcode-500.txt).

**Backend implication:** history timestamps belong in the explicit reconstruction history API. An old offline message displayed on delivery should not be claimed to show its original send time in this native UI. A future frontend can use server IDs/time for history and deduplication while preserving native packet compatibility.

### Message text is transformed by the original renderer

**Confirmed:** before wrapping, `s.a(...)` trims message text and replaces seven internal markup delimiters with spaces: `¤`, `±`, `Ω`, `µ`, `♣`, `▲` and `∫`. Server-provided metadata, emoticon trigger lists and colors drive separate rendering paths; raw user text is not HTML. A room/group sender string equal to the conversation ID selects system-message styling and, for private groups, a generic group label.

Evidence: [s.java](../v4/artifacts/normal-flow/s.java), lines672–683; [k.java](../v4/artifacts/normal-flow/k.java), lines1284–1295 and1332–1340. The corresponding original methods are in [s.txt](../v4/artifacts/bytecode/s.txt) and [k.txt](../v4/artifacts/bytecode/k.txt).

**Backend implication:** exact visual recreation must reproduce these transformations rather than treating a wire text string as an exact screenshot transcript. Preserve message content separately from display adaptation. System-message styling is a client presentation rule; it does not authorize a client to impersonate a system sender. The backend must continue deriving sender identity from the session.

### Reconnect and leave flags are not server-side membership rules

**Confirmed:** successful reconnect203 can call `k.f()`, which issues fresh703 joins for retained type5 room tabs. The703 builder ignores its boolean parameter. While704/753 is pending, incoming room/group messages for that conversation are suppressed by local vectors; failure paths can remove that suppression. A new application launch has no retained room tabs to rejoin.

Evidence: [bc.java](../v4/artifacts/normal-flow/bc.java), lines386–392,2021 and2522–2524; [k.java](../v4/artifacts/normal-flow/k.java), line106; [aj.java](../v4/artifacts/normal-flow/aj.java), line6033. See the [core-flow audit](../../../docs/core-flow-audit.md#reconnect-behavior-an-important-documentation-correction) for the full chain.

**Backend implication:** preserve session-scoped room membership and respond to the client's actual rejoin requests. Do not persist a session's joined rooms merely to make reconnect appear seamless. Capacity changes, failed rejoins, kicks with retained tabs and disconnects during leave still need native failure-path acceptance tests. The ignored boolean cannot establish a special silent-rejoin server contract.

## What remains impossible to recover from these client branches

| Question | What the binary establishes | What remains a reconstruction policy or evidence gap |
| --- | --- | --- |
| Vote-kick thresholds |706 supplies room and target; ordinary users can invoke the menu | Required votes, occupancy sampling, expiry, cooldown, immunity, thresholds above25 and any charges. Current rules are documented in [rooms](../../../docs/rooms.md). |
| Large room limits |701 capacity and occupancy are32-bit integers;708 parses the full supplied list | Actual historical capacity tiers, owner-level requirements and quotas. The26-tab limit is unrelated to occupancy. |
| Contact approval |405 adds a contact; page link opens that form | Reciprocal acceptance, notification rules and privacy. No concrete outgoing413/414/415 builder was recovered in this v4 build. |
| Search behavior |The exact profile and room search URLs and native action links exist | Original query field names, matching/ranking, pagination, privacy filtering and HTML layouts. Replacement forms must be labeled as such. |
| Room moderation |706/709/710 and role/mute list markers exist | Ban durations, appeals, room-owner inheritance, fees, action history and every permission edge. |
| Message delivery |Local echo,500,ACK/error and reconnect behavior are visible | Original retention periods, offline quotas, multi-session fanout, durable IDs, read receipts and exact-once guarantees. |
| Profiles, media and emoticons |603/421/914 and native renderers consume server data | Complete historical assets, profile visibility, content policy, upload quotas and media lifetime. |

The new Profile→Chat, Add Friend and Room Info moderation flows now use these findings. The core slash-command response is also implemented using incoming content6; retained-tab reconnect failures and media delivery remain next native priorities. A faithful new frontend should also preserve the320-character composer, local echo rules and the distinction between local display time and original message time. Account persistence, profile/privacy policy and media catalogs still require deliberate backend design even with the full applet available.
