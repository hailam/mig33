# Protocol changes: mig33 3.08 → 4.62

This comparison is between the two recovered binaries, not a claim about every intervening v3/v4 release. The [3.08 reference](../v3/docs/protocol.md) and [4.62 reference](../v4/docs/protocol.md) contain full schemas and source references. The binary packet envelope and login hash remain compatible, but contact synchronization, external IM discovery, room listing, page transport and dynamic UI configuration require separate version-specific behavior.

| Evidence | 3.08 | 4.62 |
|---|---|---|
| SHA-256 | `2b416199dca5f6593eefe8fa1c9b84ec4be6cdc36f68bcf17e1a94193bfd2934` | `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c` |
| Packet/field classes | `w` / `p` | `ay` / merged `bj` |
| Network / transport | `com.projectgoth.Roshan.a` / `.b` | `bc` / `u` |
| Incoming top-level cases | 33 | 55 |
| Shared incoming cases | 29 | 29 |

The [machine-readable direct-read comparison](../v4/docs/data/protocol-version-differences.json) was generated from the two incoming inventories. It explicitly excludes delegated UI accesses and exception-handler traversal; use the semantic tables below rather than treating it as a complete protocol diff by itself. Original bytecode remains authoritative when decompilation fails. V4 normal-flow derivative code has unchanged instructions but omitted exception tables and must never be executed.

## Unchanged foundation

- Nine-byte header `[02][opcode:S][transaction:S][bodyLength:I]`, then `[tag:S][length:I][payload]` fields, big-endian numbers, raw UTF-8 strings, unordered fields, duplicate tags last-wins. Neither has a wire field-type marker.
- TCP gateway host `gateway.mig33.com`, ports25/9119, HTTP base `http://gateway.mig33.com/`, binary POST `Content-Type: image/gif`, session path suffix and base-path exceptions200/100/2/904.
- Login200→201→202→203.201 carries challenge and session;202 carries the signed32-bit Java hash of `challenge + password.toLowerCase()`. No password is sent in200.
- Basic message500 fields1/2/3/4/5/6/8, presence values1/3/4/99, contact/group integer IDs, room names/group-chat string IDs, error0 field2 and contextual success1.
- Registration101's duplicate tag1 bug: height overwrites width. Exact reproduction retains this unless deliberately offering a corrected mode.

These are supported by actual serializer executions: five common packet fixtures—ping,error,ack,UTF-8 chat,duplicate registration tag—are **byte-identical** between the original classes. A no-field ping fixture is still valid in v4 even though the automatic keepalive now includes1:I1. See [v4 fixture output](../v4/docs/data/protocol-original-fixtures.txt) and [v3 fixture output](../v3/docs/data/protocol-original-fixtures.txt).

## Transport and authentication changes

| Area | 3.08 | 4.62 | Reconstruction consequence |
|---|---|---|---|
| Transaction counter | Int repeatedly cast to short while incrementing; eventually becomes negative | Int counter until normal integer overflow; wire `(counter %32767)+1` | Do not copy v3's early short rollover into v4 |
| Pending requests | Largely serial operation/wait flow | Pending vector plus many immediately continued operations; optional parallel HTTP queue | Responses can complete while further requests are outstanding |
| Keepalive | TCP2 no fields;60s idle | TCP2 with1:I1;60s idle; duplicate pending heartbeat triggers reconnect | Preserve distinction from manual no-field test |
| Contact download wait | 90s TCP400 | 120s TCP400 | Loading/reconnect timing differs |
| Other waiting | Usually60s | Test2:45/60s; many async operations proceed; most others1s | Recreate timers from version-specific code, not a shared60s timeout |
| HTTP headers | Page-specific mig33/sid/sw/sh/ver/User-Agent | Shared helper adds language,epoch,cellid,lac,mcc,mnc,cookies; User-Agent may include username | Device/context headers apply to tunnel and direct web requests |
| Browser transport | Internal998 page job,999 image job |922/923 gateway API plus direct HTTPS and parallel HTTP routes | Separate web and binary gateways are no longer sufficient without the922 adapter |
| Login client identity |200 tag3:S308;7:T `J2MEv3.08.165` |200 tag3:S462;7:T `J2MEv4.62.000` | Route fixture/server profile by client version |
|200 removed tag |14:I wallpaper revision | Not emitted | Wallpaper handshake is replaced by config/theme resources |
|200 added tags | None |15:T language,16:I config revision,17:T cellID,18?:T prior session,19:B activity preference,20:T LAC,21:T MCC,22:T MNC | Support reconnect/context metadata |
|200 presence9 |1 or99 from invisible-login preference | Current desired presence during reconnect;1/99 on fresh login | Preserve reconnect state |
|203 IM state | Fixed6/7/8/17 service flags | Dynamic208 descriptors; old flags not read | Fixed service slots cannot reproduce v4 |
|203 currency/unused |11:T currency,12:T auxiliary;18:B call-through flag |11/12 not read;18 read/discarded | Call/account content moves into pages |
|203 added state | No persistent contact checkpoint |20:I cache revision,23:T,24:I,26:T site URL,27:B,28:T profile string,29:B system-info flag,31/32 ambiguous dimensions/settings,33/35/37 feature flags | Match exact field reads including anomalies; see detailed v4 table |
| Registration103 | Fields1–5 | Adds9:T and10:T same client-build string | v4 sends extra identity data |
| Challenge/username check | Registration-only101/102/103/104 | Adds16/17 general security challenge and1005 HTTP username check | Additional screens and recovery paths |

V4's mixed203 accessor reads are real:31 and32 are used as integers for dimensions, then31 as short and32 as string for settings.33/35/37 are checked through byte accessors then read as integers. The client alone does not provide a clean intended server schema that resolves this conflict.

## Contacts, presence, external IM, and profile

| Opcode/flow | 3.08 | 4.62 |
|---|---|---|
| Initial list | Always400→401/402→403 | If203 revision matches saved revision,4191:L checkpoint; otherwise400.420 carries revision/time; contacts cached in RMS |
|402 fixed external accounts |14/15 MSN,16/17 AIM,18/19 Yahoo,22/23 Google Talk |31:X service IDs,32:X presence bytes,33:TA usernames; native mig33 and generic external branches separate |
|402 profile metadata | No avatar/status fields read |27:T avatar key,28:T status text |
|404 presence | Fixed byte tags2/3/4/5/7 |2:B mig33 or9:X service IDs +10:X presence bytes |
|405 add | Fixed14/16/18/22 external usernames |30:T username +31:B generic service; fixed external tags not emitted |
|407 edit | Fixed external username tags;2:I group always |30:TA usernames +31:X services;2:I group omitted−1 |
|411 move |2:I group always |2:I omitted−1 |
|406 server push | No top-level incoming handler | Incoming1:I deletes contact/cache and closes chat |
|412/413/414 | Native incoming request + accept/reject packets | No corresponding top-level412 or concrete413/414 requests recovered |
|415/416 | Binary blocked-user list | No concrete415 or top-level416; block-list settings is HTML |
|417 |2:B2 block or1 unblock |2:B2 always; boolean parameter ignored |
|600 presence |1:B new status | Adds2:B previous status; error branch incorrectly reads2:I |
|601/602/603 | Absent | Status/avatar update requests and own-profile result;602 overwrites tag1 with old avatar and omits rollback2 |
|421/423 | Absent | Incremental avatar/status update + optional3:L checkpoint |
|422/921 | Absent | HTML-like Updates feed items and subscription toggle |
|206 connect |3:B show-offline flag |3:B0 always |
|207 service state | Four hard-coded services | Registry lookup by208 descriptor |
|208/926/927 | Absent | Dynamic service ID,name,message channel,state and downloaded status icons |
|905/906/907 |905 fetches credentials;906 fixed15–26 username/password/flag slots;907 edits selected credentials | No905 call;906 packet cache/service30;9071:B1 with30:B selected service for removal. Setup credentials are handled through `c=im&a=setup_im` pages |

External service IDs and message channels were already separate enums in v3. V4 explicitly maps between them through208 field3. A compatible implementation must preserve that mapping instead of assuming service ID equals500 channel. V4 uses NUL-separated UTF-8 arrays, which v3 did not construct for contact fields; this is a field-payload change, not a new packet envelope.

## Chat rooms, group chats, messages, and media

| Operation | 3.08 | 4.62 |
|---|---|---|
| Room listing |700 search/page/recent filter →701* /702 page count |713 categories →714*/715;716 category contents →701*/717;718 links. Search/create use HTML paths |
|701 room record |1:T name,3:I capacity,4:I occupancy | Adds6:S category;703 reply also uses8:I,10:T,11:I theme ID,12:T conversation metadata |
|703 join | Ack creates room conversation |701 can create conversation with theme and room data;ack adjusts readiness |
|705 create | Native binary request | No concrete builder/caller; `c=chatroom&a=create` page |
|711/712 | Absent | Add favorite/remove category room |
|719 | Absent | Server room theme with imageURL and independent style integers |
|708/755 lists | Participants/admins/muted semicolon lists | Adds5:B service(default1) |
|750 group assigned | Group ID and original peer | Adds3:B service(default1) |
|751/752 group invite | Peer/group +invitee | Adds3:B service |
|753/754 group leave/list |1:T group ID | Adds2:B service |
|500 incoming | Basic text/color/emoticons | Adds10:T avatar and14:T auxiliary metadata |
|500 outgoing | Does not emit color13 |13?:I selected message color |
|504 upload |1:S1,2:X bytes,4:S parameter |4 conditional−1; adds5:B purpose; photo-home fallback depends on5 |
|913 emoticon fetch |1:T exact token |1:T token plus trailing space,2:I action mode |
|914 emoticon result | Identifier/aliases/label/image | Adds5:B content type; absent4 forces type0; can open richer image/animation flows |
|924 | Absent | Server-supplied semicolon list of selectable chat colors |
|800 callback | Dynamic4:B confirmed flag,5:B callback/call-through mode | Always4:B0,5:B1; ignores provided ints |
|805 rates | Binary request supported | No concrete caller; rates/call workflow is HTML |

The semicolon participant parser still expects a delimiter after the final member. Missing color overrides still use−1. Both keep optimistic local outgoing chat text and contextual error/ack behavior, but v4 retains more reconnect state and supports avatar/status/theme-dependent rendering.

## Server-driven UI and pages

Four v3 top-level incoming handlers are absent in4.62: **412,416,702,912**. Twenty-six are new: **8,11,12,14,16,208,406,420,421,422,423,603,714,715,717,718,719,918,923,924,927,928,1000,1002,1004,1005**. The shared29 handlers often have different side effects, as detailed above; matching the opcode alone is not sufficient.

| Interface | 3.08 | 4.62 |
|---|---|---|
|5 notification |2 text/URL,3 mode | Adds1 subtype and4 title/auxiliary data; queued notification model |
|8/9 prompts | Absent | Server prompt and explicit accept/decline response with original transaction |
|11/12 preferences | Absent | Server read/write numeric and string preferences |
|14 banner | Absent | Contact-list banner/link |
|910 feature URL | Binary feature selector returns URL | No concrete910 request; many explicit `/sites/index.php` paths |
|998/999 | Local page/image jobs;999 lowercases entireURL | Removed local jobs;922/923 handles pages/assets, callbacks, cache times and cookies |
|HTTPS | Historical v3 resolver recognizes only`http://` | Direct HTTPS route; broader resolver and link handling |
|Redirects/cookies | Not implemented in recovered page helper |301/302 handling in parallel direct path; approximate cookie store (not standards-compliant) |
|Wallpaper912 | Stored wallpaper version and bytes | No912 handler;1000config/theme,719room themes and922assets |
|Language | English literals | Bundled language1004 + downloadable1002catalog/1004packs,925language notification |
|920 reset |1contact tab,3clear emoticons | Adds2config reset,4language reset,5menu/tab rebuild |
|918/928 | Absent | URL navigation and processing completion instructions |

Menu911 still has1:I category,2:I action,3:T label,4:T auxiliary/navigation value, but its categories were **reassigned**:

| Category | 3.08 | 4.62 |
|---:|---|---|
|1|Contact context|Own profile|
|2|Group context|Home/contact-list|
|3|Settings|Updates|
|4|Chat Rooms entry|Contact context|
|5|Mail entry|Group context|
|6|Room listing|Settings|
|7|No handler|Embedded browser|
|8|Embedded browser|Room listing|
|9|Room chat|Private chat|
|10|mig33 private|Group chat|
|11|MSN private|Room chat|
|12–14|Yahoo/AIM/Google Talk private|No handler|

V4 has an explicit web-to-native command interface `mig33:method(argument)` for20 recognized method names/prefixes, including joins, private chats, photo upload/send, status edit, native browser and tab control. The [full command table](../v4/docs/protocol.md#web-to-native-mig33-commands) records exact argument behavior. These commands are not REST endpoints and are not JavaScript. Recreating only page pixels would miss their native transitions.

The [62-literal v4 endpoint inventory](../v4/docs/data/protocol-endpoints.json) gives precise source locations. It includes profiles,groups,photos,avatars,rooms,store,call/SMS/email,account,settings,help,reporting,location,language and registration availability. Dynamic pages and server-added URLs remain unrecovered historical content. Endpoint literals establish routes, not backend implementation or original response bodies.

## Fixture and compatibility strategy

Keep one framing implementation with version-specific payload/state handlers. Use independent fixtures for3.08 and4.62 login203, contact downloads, IM setup, room lists, menus and browser pages. A v4 server simulator needs contact revisions/timestamps, descriptor208/icon927, category714/715/717, theme719/config1000, language1004 and922/923 responses. A v3 simulator instead needs fixed IM slots, room700/702 paging, feature910 URLs, wallpaper912 and local998/999 page behavior.

Run `python3 tools/applets/v3/protocol-codec.py --self-test` and `python3 tools/applets/v4/protocol-codec.py --self-test`. Nine original-class v4 fixtures and seven v3 fixtures validate payload encoding; five shared examples match byte-for-byte. New v4 examples cover64-bit timestamps,NUL-separated accounts,outbound color13,and602 duplicate-tag behavior. The tests use preserved classes only for serialization and never execute a MIDlet or contact a service.

Neither binary supplies server validation rules, credit accounting, moderation policy, dynamic HTML, historical user content, or evidence that the legacy endpoints still operate. The comparison is precise about observable client behavior and explicit about those missing server contracts.
