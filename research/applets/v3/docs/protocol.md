# mig33 3.08 protocol and API reference

This specification describes the recovered J2ME 3.08 client, SHA-256 `2b416199dca5f6593eefe8fa1c9b84ec4be6cdc36f68bcf17e1a94193bfd2934`. It covers the client-visible wire protocol, every incoming opcode dispatched by this build, the outgoing requests it constructs, its HTTP page interface, and reconstruction limits. It is a static reverse-engineering result, not a specification of a currently operating service. No legacy server was contacted to test login or application operations.

There is no REST or JSON API in this client. A binary tagged-field protocol travels over a persistent TCP connection or an HTTP tunnel. A separate HTML renderer requests server-generated pages. Names such as “login challenge” below are descriptive names assigned from behavior; the obfuscated binary contains numeric IDs, not those names.

## Evidence and notation

Code paths in this document are relative to the workspace. `A` means [network controller com.projectgoth.Roshan.a](../artifacts/decompiled/com/projectgoth/Roshan/a.java), `B` means [transport com.projectgoth.Roshan.b](../artifacts/decompiled/com/projectgoth/Roshan/b.java), and `C` means [UI controller com.projectgoth.Roshan.c](../artifacts/decompiled/com/projectgoth/Roshan/c.java). Numbers after a colon are line numbers in the CFR output. Full bytecode is in [artifacts/bytecode](../artifacts/bytecode/); the independent [Vineflower output](../artifacts/vineflower/) preserves numeric casts more reliably.

**Do not derive numeric field widths from uncast literals in CFR.** For example, CFR prints login field 1 as `new p(1, 1)`, but bytecode calls `(short, short)`, making its payload two bytes, not four. The widths below were cross-checked against the bytecode and Vineflower output. There is no on-wire field type discriminator.

Notation: `B` = one signed Java byte; `S` = two-byte signed Java short, big-endian; `I` = four-byte signed Java int, big-endian; `T` = raw UTF-8 bytes; `X` = arbitrary bytes. `?` means this client conditionally sends or safely checks the field. Mandatory versus optional here describes client behavior, not a recovered server validation contract. Numeric IDs are decimal unless prefixed `0x`.

The [incoming field inventory](data/protocol-incoming.json) retains every direct field read in the network dispatch with its source line. The [request construction inventory](data/protocol-builders.json) retains every constructor call in the outgoing builder section and its original source expression. These are evidence indexes, not substitutes for the semantic tables below.

## Transport and session lifecycle

| Interface | Exact client behavior | Evidence |
|---|---|---|
| Default TCP | `socket://gateway.mig33.com:25`; if opening fails, retry `socket://gateway.mig33.com:9119`; if both fail, switch to HTTP | A:53–101 |
| Saved/manual TCP | Saved server and port override automatic fallback. Manual connection uses the entered host/port. An opening failure returns failure instead of trying the default servers | A:57–98; C:1100 onward |
| HTTP binary tunnel | Base `http://gateway.mig33.com/`. Send binary packet as HTTP POST, `Content-Type: image/gif`, `Content-Length` = serialized packet byte count. HTTP status must be 200 | B:29,64–118 |
| HTTP session path | POST to the base for opcodes 200, 100, 2, 904. Other operations use base + session string from opcode 201 field 2. No separator is inserted. Polling is GET to base + session | B:75–99 |
| TCP data | Write serialized packet and flush; reader repeatedly reads one packet from the stream | B:160–180,224–249; w:89–112 |
| Keepalive/poll | After contact-list completion: if queue empty and idle >60,000 ms on TCP, send opcode 2; if idle >5,000 ms on HTTP, perform GET with no request packet | A:1574–1601 |
| Queue and timeout | One queued operation processed at a time. TCP waits 90,000 ms for opcode 400, otherwise 60,000 ms. Completion handlers set the wake flag. Uncompleted wait synthesizes disconnect opcode 301 | A:1606–1645 |
| Transaction | A short transaction ID is assigned before sending. Normal sequence is positive IDs; response matching accepts the current ID or any response ID ≤0. See rollover quirk below | A:1587,1608,1664 |
| Connection loss | Socket read failure synthesizes 301 field1 = reconnection text. HTTP errors synthesize 0 field2 = connection error text | B:119–130,232–236 |

`Main.a=1` identifies HTTP and `Main.b=2` identifies TCP. TCP is the initial default unless the stored `TCP` preference is 1. [Main.java:18–35](../artifacts/decompiled/com/projectgoth/Roshan/Main.java).

There are two historical transport references whose application paths are not present: opcode **100** receives unauthenticated HTTP base-path treatment, and opcode **903** with byte field2 = 3 forces HTTP even when the configured mode is TCP. No outgoing builder or UI call for either exists in this JAR. Their operation schemas cannot be recovered from this client.

The queue counter is an int incremented with a cast to short (`n2 = (short)(n2 + 1)`); the assigned transaction is `(short)(n2 % 32767 + 1)`. Do not assume it stays positive after 32,768 operations. The code also checks response IDs inconsistently: asynchronous pushes such as 500, 404, 911 do not require a match, whereas request responses usually do. A replacement server should echo the transaction for request results and can use 0 for pushes.

## Binary framing

Evidence: [w.java:74–143](../artifacts/decompiled/w.java), [p.java](../artifacts/decompiled/p.java).

| Offset | Bytes | Meaning |
|---|---:|---|
| 0 | 1 | Magic / packet marker, always `02` |
| 1 | 2 | Opcode, big-endian short |
| 3 | 2 | Transaction ID, big-endian short |
| 5 | 4 | Body length in bytes, excluding the 9-byte header |
| 9 | body length | Zero or more tagged fields |

Each field is `[tag:S][payloadLength:I][payload:payloadLength bytes]`. There is **no field count**, type byte, terminating zero, padding, compression, checksum, or encryption in this framing code. Thus a field adds six bytes overhead and a packet is `9 + Σ(6 + payload size)` bytes long. Multiple packets may be concatenated in one HTTP response; TCP uses each header's body length to read successive packets.

Fields live in a `Hashtable<Short,p>`. Adding a tag replaces any previous field with the same tag. Enumeration order is not a protocol guarantee. A replacement codec must accept any field order and treat repeated tags as last-wins if emulating this client. String payloads are UTF-8 without a length prefix within the payload; this differs from the `writeUTF` format used in RMS storage. Embedded nulls have ordinary UTF-8 encoding.

The decoder's primitive accessors read the first byte/short/int/long from the raw payload. Field types are determined by the operation and tag, never by payload length alone. Missing fields return `null`; many callers immediately unbox them and therefore require them to exist. The legacy parser does not bound lengths or enforce exact field-body boundaries. The provided reference codec rejects malformed lengths, while preserving valid packet encoding.

Examples:

- Ping, transaction 1: `02 00 02 00 01 00 00 00 00`.
- Error opcode 0, transaction 1, field2 UTF-8 `Bad`: `02 00 00 00 01 00 00 00 09 00 02 00 00 00 03 42 61 64`.
- Acknowledgement opcode 1 with no fields and transaction 1: `02 00 01 00 01 00 00 00 00`.

## Authentication and initial download

1. Client saves the supplied username/password to RMS `SignIn`, then sends **200** with platform and rendering metadata. Password is absent from 200.
2. Server sends **201**: field1 challenge string, field2 session string. The session string becomes the HTTP path suffix and `sid` page header.
3. Client sends **202**: field1 is Java `String.hashCode()` of `challenge + password.toLowerCase()` as a signed 32-bit int. The username is not part of this calculation. Preserve Java's UTF-16 code-unit hash and the historical lowercasing behavior; an ASCII test vector avoids locale ambiguity.
4. Server sends **203**, providing account state and enabled services. Client automatically queues **400** for contacts.
5. Server sends **401** group records and **402** contact records, followed by **403** completion. The client enters the contact list and enables idle polling/keepalive.
6. It can then connect enabled external IM accounts using **206**, receive menu pushes **911**, emoticon lists **916**, announcement **5**, and other asynchronous state.

Evidence: A:263–299,375–379,895–945,992–1007,1009–1022. This is a weak historical challenge calculation, not modern password authentication. For reconstruction, keep compatibility inside a local fixture environment; a newly designed public service should not inherit this credential design.

### Login request 200

| Tag | Type | Value |
|---:|---|---|
| 1 | S | 1, client protocol constant; descriptive meaning beyond constant not recoverable |
| 2 | B | 2, platform/client constant |
| 3 | S | `major*100 + minor` = 308 |
| 5 | T | Username |
| 7 | T | `J2MEv3.08.165` |
| 8 | T | `microedition.platform`, fallback `Unknown` |
| 9 | B | 99 if “Sign in as Invisible”, otherwise 1 |
| 11 | I | Font metric `C.aW`: current font height if >5, otherwise baseline+3 |
| 12 | I | Canvas width minus 6 |
| 13 | I | Canvas height minus top/tab bar height `C.c` |
| 14 | I | Stored `Wallpaper` preference/version |

### Login result 203

| Tag | Type | Client use |
|---:|---|---|
| 1 | T | Own mobile number |
| 2 | B | Account activated if nonzero |
| 3? | T | Announcement text or URL |
| 4 | B | Announcement mode: 1 = local dialog, other = HTML URL; read when tag3 exists |
| 6 | B | MSN state: nonzero = account configured, 2 = connected |
| 7 | B | AIM state, same convention |
| 8 | B | Yahoo state, same convention |
| 11 | T | Currency label; inserted into `Amount (<currency>)` |
| 12 | T | Stored in `C.R`; no subsequent use found in this client |
| 13? | T | Mail URL; presence enables mail UI |
| 14 | I | Unread mail count, read when mail URL exists |
| 16? | I | Emoticon square size; changed value updates preference and clears cache |
| 17 | B | Google Talk state, same convention |
| 18? | B | Call-through available if value 1 |

Other fields, including unused numeric gaps, have no recovered client semantics.

## Outgoing operation catalog

`ack` means incoming opcode1; `error` means opcode0 field2. Unless a richer result is listed, the common error path can show a server error string. Source ranges refer to A unless marked C. All field tags not listed are absent in the corresponding builder.

| Opcode | Operation | Fields and expected result | Evidence |
|---:|---|---|---|
| 2 | Connection test / ping | No fields → 3. Also used by idle TCP keepalive | C:3478,3971; A:1586 |
| 7 | Get help text | 1:I topic. Topics 1 search room, 2 create room, 4 make call, 34 call-through, 5 registration → 6 field1 text | A:363; C:3548–3568 |
| 101 | Registration initialization | 1:I canvas height. **Original bug:** width is first written to tag1, then height overwrites it. No tag2 is sent → 102 | A:213–218; bytecode confirms |
| 103 | Submit registration | 1:T username; 2:T password; 3:T mobile; 4:T challenge/token from 102 tag2; 5:T entered security code → ack message or 104 refreshed challenge | A:175–183; C:3889 |
| 200 | Begin login | See dedicated table → 201 | A:263–299 |
| 202 | Challenge answer | 1:I Java hash of challenge + lowercased password → 203 | A:375–379 |
| 204 | Activate account | 1:T entered activation code → ack message; marks own account activated | C:3330; A:1507 |
| 205 | Set mobile / resend activation | 1:T mobile → ack; updates own mobile and enters activation prompt | C:3142,3874; A:1512 |
| 206 | External IM sign-in | 1:B service; 2:B 1; 3:B show-offline-contacts flag → ack / 207 | A:322–331; C:1795–1804 |
| 300 | Exit / logout | No fields. Request completion destroys MIDlet, regardless of detailed payload | C:2879,4070; A:514–517 |
| 302 | External IM sign-out | 1:B service → ack; updates IM setup/navigation | A:322–331; C:2795 |
| 400 | Download contact list | No fields → repeated 401/402, then 403 | A:941 |
| 405 | Add contact | 2?:I group; 3?:T display name; 8?:T mobile; 12?:T mig33 username; 14?:T MSN; 16?:T AIM; 18?:T Yahoo; 22?:T Google Talk; 24:B share mobile flag, included only with tag12 → 402 or ack | A:446–477 |
| 406 | Delete contact | 1:I contact ID → ack; removes selected contact | C:3155 |
| 407 | Edit contact | 1:I contact ID; 2:I group ID; 3?:T display name; 8?:T mobile; 12?:T mig33; 14?:T MSN; 16?:T AIM; 18?:T Yahoo; 22?:T Google Talk → 402 | A:480–506; C:3951 |
| 408 | Add group | 2:T group name → 401 | A:416–420; C:3247 |
| 409 | Delete group | 1:I group ID → ack | C:3160 |
| 410 | Rename group | 1:I group ID; 2:T new name → ack | A:423–428 |
| 411 | Move contact to group | 1:I contact ID; 2:I destination group ID → ack | A:437–442 |
| 413 | Accept contact request | 1:T requester; 2:B add-to-my-list flag; 3:I group ID; 4:B share-mobile flag → 402 or ack | A:354–361; C:4173 |
| 414 | Reject contact request | 1:T requester → ack | C:3496 |
| 415 | Get blocked users | 1:B 2 → 416 | A:444–449 |
| 417 | Block/unblock user | 1:T username; 2:B 2=block, 1=unblock → ack | A:233–239; C:2150,3205 |
| 500 | Send chat/SMS/mail/photo | See message table → ack/error; remote messages also use 500 | A:390–425; C:3257–3269,3297–3306,3398–3402,3439–3443 |
| 504 | Upload photo to web feature | 1:S 1; 2:X image bytes; 4:S selected `<tag>` type/id-derived parameter, default 1; value3 converted to1 by UI → ack tag1 URL, else reuse Scrapbook URL | A:199–206; C:3449–3458 |
| 600 | Set presence | 1:B presence → ack | A:431–435; C:2414 |
| 700 | List/search chat rooms | 1?:T search text; 2?:B page, omitted when -1; 3?:T recent-room string → repeated701, then702 | A:337–352; C:2437–2450,2827,3941 |
| 703 | Join chat room | 1:T room name → ack; opens room chat | C:2431; A:1499–1504 |
| 704 | Leave chat room | 1:T room name → ack; closes chat | C:2556; A:1522 |
| 705 | Create room | 1:T room name → ack; opens room chat | C:3946; A:1499–1504 |
| 706 | Kick room user | 1:T room name; 2:T target username → ack | C:2629; A:241–247 |
| 707 | Get room participants | 1:T room name → 708 | C:2533–2549 |
| 709 | Mute room user | 1:T room name; 2:T username → ack | C:2639 |
| 710 | Unmute room user | 1:T room name; 2:T username → ack | C:2635 |
| 751 | Turn private chat into group chat | 1:T existing peer username; 2:T invited username → 750 with group ID | A:249–260; C:2624 |
| 752 | Invite user to existing group | 1:T group ID; 2:T invited username → ack | Same builder as751 |
| 753 | Leave group chat | 1:T group ID → ack; closes chat | C:2587 |
| 754 | Get group participants | 1:T group ID → 755 | C:2569 |
| 800 | Call request / confirm | 1:T origin number; 2:T destination number; 4:B 1=initial request, 0=confirmed; 5:B call mode 1=callback/default, 2=call-through → ack message or804 platform dial instruction | A:302–318; C:3175,3897 |
| 805 | Compare call rates | 1:T origin number; 2:T destination number → ack tag1 informational text | A:302–318; C:3901 |
| 900 | Invite friend by SMS | 1:T mobile number; 2:T your name → ack message | A:382–388; C:3906 |
| 904 | Forgot details | 2:T mobile number → ack; returns to login | A:225–230; C:3884 |
| 905 | Get external IM settings | No fields → 906 | C:2802 |
| 907 | Update external IM settings | Reuses entire cached906 packet, changes opcode to907, edits relevant fields. See settings table below → ack | A:320–324; C:3181–3200,3911–3934 |
| 910 | Get feature page URL | 1:B feature; 2?:T target username → ack tag1 URL. Features1 scrapbook,2 user profile,3 own profile,4 account,5 community,6 scrapbook photo picker | A:208–212; C:2496,2691,2920–2930,3028–3038 |
| 913 | Get emoticon asset | 1:T emoticon token → 914, or error installs “not found” placeholder | C:1953; A:1158–1164 |
| 998 | **Internal page job** | 1:T URL; 2?:T form body. Consumed locally by B; **not serialized to the gateway**. Produces synthetic ack after HTTP fetch | C:452; B:64–67 |
| 999 | **Internal image job** | 1:T URL; 2:I renderer image slot (−1 for background). Consumed locally; **not gateway wire operation**. Produces synthetic ack fields1:I image kind,2:X image bytes | A:220–224,1620; B:350–407; k:467–469,574–576 |

The uncalled generic `a(short,String,String)` can construct arbitrary opcode + two strings. Its only recovered call is page operation998. Similarly, generic one-string/no-field/int helpers are not evidence of unspecified additional server APIs.

### Message packet 500

| Tag | Type | Sent by client / consumed on receive |
|---:|---|---|
| 1 | B | Channel: 1 mig33, 2 SMS, 4 MSN, 5 AIM, 6 Yahoo, 8 Google Talk, 9 mail |
| 2 | T | Sender: own username, except SMS uses own mobile. Incoming sender display identifier |
| 3 | B | Conversation type: 1 private, 2 group, 3 room |
| 4 | T | Destination username/mobile, group ID, or room name |
| 5? | I | Contact ID for private contact conversation; omitted if −1 |
| 6 | S | Content type: 1 text, 2 raw image upload, 5 scrapbook photo reference. Incoming type6 is rendered without sender prefix and uses special body-color default |
| 8 | T or X | Message text, photo-reference string (type5), or raw image bytes (type2). Incoming opcode500 rendering always treats this field as string; file notification uses502 |
| 9? | B | Incoming-only marker: presence selects message style4 for group/room. The byte's numeric value is not examined |
| 11? | T | Incoming space-separated emoticon trigger list used to replace matching text with image markers |
| 12? | I | Incoming sender-name RGB color override; absent maps to −1/default |
| 13? | I | Incoming message/body RGB color override; absent maps to −1/default |

The channel codes above are different from the **external IM service codes** in206/207/302: 2 MSN, 3 AIM, 4 Yahoo, 6 Google Talk. Presence is 1 online, 3 busy, 4 away, 99 offline/invisible. Do not share one enum between message channel and external IM login service.

A private outgoing text is locally appended immediately by the UI; the server acknowledgement is not needed to display it. SMS success displays a local success dialog; mail success displays ack field1. For successful image sends (types2 or5), the client inserts `Photo Sent!`. Errors for chat channels enter the conversation, while SMS/mail errors open a dialog. Incoming sender equal to room/group identifier becomes an informational message; group informational sender is labeled `Group Chat`. A incoming room message can cause the room tab to become active. Evidence: A:1001–1007,1172–1202,1384–1405; [ab.java:181–290](../artifacts/decompiled/ab.java); [v.java:249–309](../artifacts/decompiled/v.java).

### External IM settings 906 and 907

| Service | Username T | Password T | Flag B |
|---|---:|---:|---:|
| MSN | 15 | 16 | 17 |
| Yahoo | 18 | 19 | 20 |
| AIM | 21 | 22 | 23 |
| Google Talk | 24 | 25 | 26 |

On save, the selected username/password are inserted and its flag is set to0. On remove, username/password become empty strings. Other fields from906 are preserved because907 is built by mutating the received packet object, including any fields this client does not interpret. The UI can prefill both username and password from cached906. Evidence: C:2807–2816,3181–3200,3911–3934; A:865–892,1340–1382.

## Incoming operation catalog

All **33 top-level opcode cases** in A's dispatcher are listed. A server can send other opcodes, but this client has no top-level handler for them and silently falls through.

| Opcode | Meaning | Fields consumed and effects | Evidence |
|---:|---|---|---|
| 0 | Error | 2:T error text. Current request determines whether dialog, chat error, emoticon placeholder, login cleanup, or IM progression. Other error fields are not consumed | A:1143–1229 |
| 1 | Ack / success | Contextual1?:T message or URL; schemas for local998/999 are distinct. Wakes request. See request catalog and ack notes | A:1278–1537 |
| 3 | Connection test success | No fields. In connection-selection/manual screens persists connection mode/host/port, closes test socket, goes to login or successful-test screen | A:849–864 |
| 5 | Server notification | 2:T text or URL; 3:B mode,1=dialog, otherwise embedded page | A:696–700; C:632–640 |
| 6 | Help text | 1:T. Chooses title/screen from outstanding7 field1; 1 search,2 create,4/34 calls,5 registration | A:527–547 |
| 102 | Registration challenge | 1:T suggested mobile; 2:T opaque challenge identifier; 3:X captcha image. Creates registration form | A:1269–1276; t:94–110 |
| 104 | Registration rejected/new challenge | 1:T error message; 2:T replacement challenge identifier; 3:X replacement captcha. Restores submitted username/password/mobile while replacing captcha | A:1130–1142 |
| 201 | Login challenge | 1:T challenge; 2:T session; automatically queues202 | A:992–1000 |
| 203 | Login result | Dedicated table above; automatically queues400 | A:895–946 |
| 207 | External IM state | 1:B service; 2:B1=connected, otherwise disconnected/error; 3:T disconnect/error message when not connected | A:717–755 |
| 301 | Disconnected | 1:T reason. Cleans up session and transport when login had completed | A:1230–1247 |
| 401 | Contact group record | 1:I group ID; 2:T group name. Adds group; special completion for outstanding408 | A:1111–1129 |
| 402 | Contact record | See contact table; adds or replaces record. Can complete405/407/413 | A:1024–1110 |
| 403 | Contact list complete | No fields; stops progress, selects contacts, enables polling and subsequent login navigation | A:1009–1023 |
| 404 | Presence update | 1:I contact ID; 2?:B mig33; 3?:B MSN; 4?:B AIM; 5?:B Yahoo; 7?:B Google Talk. Missing values leave the existing status unchanged | A:1248–1251; r:480–535 |
| 412 | Incoming contact request | 1:T requester username; 2:T auxiliary string stored with request, not displayed/used in accept packet. Adds pending request | A:556–563; C:625–631 |
| 416 | Blocked users | 2?:T semicolon-terminated usernames. Creates blocked list or empty-list message | A:830–848 |
| 500 | Incoming message | Dedicated message table above | A:1001–1008 |
| 502 | File received | Packet queued; 4:T notification text displayed; 5:T URL opened on acceptance. Remaining fields not read by this build | A:683–689; C:654–661,2020,4544–4569 |
| 503 | Mail count | 1:I unread mail count | A:701–704 |
| 701 | Chat room record | 1:T room name; 3:I capacity; 4:I occupancy. Displays `(occupancy/capacity)`; queues into recent-room list when700 tag3 was present | A:947–958 |
| 702 | Chat room list end | 1:B total pages; 2?:T info message. Adds Next/Previous where appropriate; zero results produce error | A:959–991 |
| 708 | Room participants | 2:T usernames; 3?:T admins; 4?:T muted users. All are semicolon-terminated lists. Chooses kick/mute/profile/private-chat/add-contact selector based on current UI action | A:756–829 |
| 750 | Group-chat created/assigned | 1:T group ID; 2?:T old private-chat peer identifier. Matching751 updates active chat, otherwise tag2 locates existing private chat | A:705–716; ab:153–179 |
| 755 | Group participants | Same consumed fields and selector behavior as708 | A:756–829 |
| 804 | Platform call instruction | 1:T dial string; 2?:T confirmation text. If present, ask confirmation; otherwise call `platformRequest("tel:" + tag1)` directly | A:1252–1268; C:426–435 |
| 906 | IM settings | Tags15–26 described above; cache packet, open selected IM setup, prefill username | A:865–894 |
| 911 | Add server menu item | 1:I menu category; 2:I action ID; 3:T label; 4:T secondary text or navigation URL. Menu mapping below | A:587–682 |
| 912 | Wallpaper update | 1:I stored wallpaper version; 2:X image; stores and reloads assets | A:690–695; C.a(byte[]) |
| 914 | Emoticon asset | 1:T primary identifier; 2?:T space-separated aliases; 3?:T descriptive label displayed in emoticon picker; 4:X encoded image. Extracts a square `emoticonSize × emoticonSize` image, caches aliases and asset | A:564–573; i:331–370 |
| 915 | Clear emoticon cache | No fields; clears image cache | A:548–551 |
| 916 | Emoticon trigger lists | 1?:T space-separated standard triggers; 2?:T space-separated other triggers. Replaces both trigger vectors | A:552–555; C:991–1035 |
| 920 | Server reset instruction | 1:I:1 selects contact tab;3 clears emoticon cache. Other values only cause repaint | A:574–586 |

### Contact record 402

| Tag | Type | Meaning |
|---:|---|---|
| 1 | I | Contact ID |
| 2? | I | Group ID |
| 3? | T | Display name |
| 8? | T | Mobile number |
| 12? | T | mig33 username |
| 13 | B | mig33 presence, read if12 exists |
| 14? | T | MSN username |
| 15? | B | MSN presence; defaults99 when14 exists but15 absent |
| 16? | T | AIM username |
| 17? | B | AIM presence; defaults99 |
| 18? | T | Yahoo username |
| 19? | B | Yahoo presence; defaults99 |
| 22? | T | Google Talk username |
| 23? | B | Google Talk presence; defaults99 |

Displayed overall presence is selected from the available services; live presence updates prefer a non-offline service in mig33, MSN, Yahoo, Google Talk, AIM order. Record IDs and group IDs are integers, whereas group-chat IDs and room names are strings.

### Server menu categories for 911

| Category | Destination |
|---:|---|
| 1 | Contact context menu (`aE`) |
| 2 | Group context menu (`aF`) |
| 3 | Settings (`aG`) |
| 4 | Chat Rooms entry context menu (`aI`) |
| 5 | Mail entry context menu (`aJ`) |
| 6 | Chat room listing extra options (`Y`) |
| 8 | Embedded-browser extra menu (`ae`) |
| 9 | Room conversation extra menu (`X`) |
| 10 | mig33 private conversation extra menu (`Z`) |
| 11 | MSN private conversation extra menu (`aa`) |
| 12 | Yahoo private conversation extra menu (`ab`) |
| 13 | AIM private conversation extra menu (`ac`) |
| 14 | Google Talk private conversation extra menu (`ad`) |

The payload supplies menu labels and actions that are absent from the JAR. Exact historical lists cannot be reconstructed by inventing rows from later mig33 versions. See A:587–682 and C:515–610,1833–1930.

## Embedded HTTP page API

The HTML interface is the implementation of profiles, scrapbook, account, community, mail, some photo selection/upload workflows, dynamic menus, and server-provided announcements. Initial page URLs come from910/ack,203,5,502, or dynamically provided menu fields; most are **not hard-coded endpoints** in the JAR.

For internal998 page fetches (B:256–350):

| Header or property | Value |
|---|---|
| `mig33` | `3.08` |
| `sid` | Session from201 tag2 |
| `sw` | Full canvas width as decimal |
| `sh` | Full canvas height as decimal |
| `ver` | `3.08` |
| `User-Agent` | `microedition.platform`, if present |
| Method | GET if form body is null; POST otherwise |
| POST Content-Type | `application/x-www-form-urlencoded` |
| POST Content-Length | Java string character count, **not guaranteed encoded byte count** |
| POST body encoding | Platform-default `String.getBytes()` |
| URL transformation | Replace spaces with `%20` only |
| Accepted response | HTTP200; response decoded by platform-default `InputStreamReader` |

Page HTML is split into text and tag tokens after trimming lines. A line boundary inserts a space in accumulated text. It is not a general browser engine and has no JavaScript implementation. Supported forms and tags are parsed by [k.java](../artifacts/decompiled/k.java) / its [Vineflower version](../artifacts/vineflower/k.java): title, text styling, center, line/paragraph breaks, limited div/span inline colors, anchors, images, forms, text/password/hidden/submit/reset/checkbox inputs, textarea, select/options, and custom `<tag id="..." type="..." href="...">` elements. Recreate this parser's subset and defaults if matching the old UI; substituting a modern full browser can change layout and form behavior.

Internal999 images use a separate plain HTTP request with the **entire URL lowercased**, including the path/query. The page `sid`, `mig33`, `sw`, `sh`, `ver` headers are not added there. Synthetic result1 has field2 raw bytes and field1 an int determined by the lowercased URL suffix: `.gif`→1, `.jpg`→2, everything else→3. Failure returns opcode1 without image bytes. Images are therefore not obtained by a gateway opcode999 request. B:350–407.

Relative links use the current URL's directory. A leading `/` is stripped and appended to that base rather than resolved against the site root. Only strings starting `http://` are treated as absolute; this historical resolver does not implement normal HTTPS URL resolution. Evidence: [d.java:301–344](../artifacts/decompiled/com/projectgoth/Roshan/d.java).

The binary protocol is not enough to reproduce page screens: the original HTML, images, account data, and custom-tag values must be recovered independently or supplied as clearly labeled fixtures. No historical page response body is bundled in the recovered JAR except the generic page-loading error fallback.

## State, persistence, and reproduction details

- `SignIn` record1 consists of `DataOutputStream.writeUTF(username)` followed by `writeUTF(password)`. The login builder writes this record before submitting200, regardless of whether the password-save toggle is enabled; the toggle affects prefilling later. This storage is Java modified UTF-8 with two-byte lengths, not wire UTF-8. A:115–172,263–266; C login form restoration.
- TCP/HTTP preference, manual server and port, offline contacts, external IM auto-login, invisible mode, wallpaper revision and emoticon size live in RMS and influence outgoing requests. See [screens documentation](screens.md) for the full settings presentation and source-driven state definitions.
- Contacts, pending contact requests, unread-mail count, conversation IDs, page URLs, menus, trigger lists, image caches and wallpaper are populated by server responses. Preserve their distinctions when writing fixture data.
- The recent-room filter700 tag3 is built from RMS `frms` as a string with a leading semicolon before every saved room (`;room1;room2`). It differs from the server participant-list format. C:1651–1678.
- Semicolon lists708/755/416 require a delimiter after the final member in this parser; it calls `substring(start,indexOf(";",start))` without an unterminated-tail branch. An empty string represents no members.
- Incoming text is trimmed and literal U+00A4/U+00B1 markers are replaced before the renderer introduces its own color and emoticon markers. Respect these transformations when generating chat reference output. v:249–309.
- Numeric message colors are integer RGB values, not CSS strings. Missing12/13 uses −1 internally for default colors.
- Binary APIs do not reveal server-side moderation rules, registration validation, rate pricing, credit accounting, activation-code generation, delivery guarantees, room capacity policy, message retention, server error-code vocabulary, or current endpoint availability. They reveal only what this client sends and displays.

## Reproduction fixtures and validation

[data/protocol-codec.py](../../../../tools/applets/v3/protocol-codec.py) is an offline standard-library encoder/decoder, with UTF-16 Java-hash helper and fixture verification. [data/protocol-fixtures.json](data/protocol-fixtures.json) contains byte-exact packet examples. [data/protocol-fixture-harness.java](../../../../tools/applets/v3/protocol-fixture-harness.java) uses only original `p` and `w` serialization classes from the preserved JAR to produce reference packets; it does not start the MIDlet or connect to a network.

Run `python3 tools/applets/v3/protocol-codec.py --self-test` from the workspace. Field maps are compared independently of order. Reconstructing a server should start with fixtures for login201/203, groups401, contacts402, completion403, rooms701/702/708, messages500, file502, and dynamic pages. These fixtures are test data; they do not establish the original service's historical content.
