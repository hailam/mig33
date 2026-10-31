# mig33 4.62 protocol and API reference

This is the client-visible API of the preserved **J2ME 4.62** binary, SHA-256 `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`. It is a separate specification from [3.08](../../v3/docs/protocol.md). It documents all 55 top-level incoming opcode cases, every concrete outgoing packet construction, generic request helpers and their recovered callers, HTTP interfaces, and wire-level anomalies. It does not establish the behavior of other v4 releases or of an operational server. No account login or legacy application service was contacted.

Start with the [endpoint catalog](endpoints.md) for gateway addresses, all 52 built-in page URL templates, exact parameter construction, HTTP header profiles, dynamic URL sources and installer/resource distinctions. Use this document for the binary field-level contract.

## Evidence and conventions

The authoritative sources are the [original class bytecode](../artifacts/bytecode/), especially [bc.txt](../artifacts/bytecode/bc.txt), [ay.txt](../artifacts/bytecode/ay.txt), [bj.txt](../artifacts/bytecode/bj.txt), [u.txt](../artifacts/bytecode/u.txt), and [aj.txt](../artifacts/bytecode/aj.txt). `bc` is the network controller, `u` the transport, `ay` the packet, and part of merged class `bj` the field codec. The same obfuscated field name can occur with different JVM descriptors; a name such as `bj.b` alone is not a unique field.

Some original methods fail in CFR, Vineflower, and Procyon. For reading normal branches, [bc.java](../artifacts/normal-flow/bc.java) and [aj.java](../artifacts/normal-flow/aj.java) are analysis-only decompilations made after removing exception tables from a derivative. **Their instruction bytes are unchanged, but their catch/finally semantics are not usable. Never execute this derivative.** `N:line` below refers to this normal-flow `bc.java`; `U:line` to normal-flow `u.java`; `J:line` to normal-flow `aj.java`. For original error handling and synchronization use bytecode. [Transformation record](data/normal-flow-transformations.json) and [generator](../../../../tools/applets/scripts/decompile_normal_flow.py) record the procedure; ordinary original decompilation remains in [vineflower](../artifacts/vineflower/).

[protocol-incoming.json](data/protocol-incoming.json) records every top-level incoming case, original bytecode entry, reachable normal instruction offsets, and direct tag/accessor reads. Individual `protocol-opcode-N.txt` files retain the corresponding original instructions. The extractor follows branch targets, including the remote continuation of opcode203, rather than assigning contiguous bytecode ranges. It does not claim to traverse exception handlers or infer fields read in delegated UI functions. Those delegated reads are documented here. [protocol-constructions.json](data/protocol-constructions.json) indexes every concrete `new ay(opcode)` in the original classes, including internal and RMS packets. [protocol-endpoints.json](data/protocol-endpoints.json) indexes exact endpoint literals and original bytecode line references.

Types: `B` signed byte, `S` signed 16-bit big-endian short, `I` signed 32-bit big-endian integer, `L` signed 64-bit big-endian long, `T` raw UTF-8 text, `X` raw bytes, `TA` UTF-8 string array separated by single NUL bytes. `?` denotes optional/conditional use by this client; it is not a recovered server validation rule. There is no transmitted field-type discriminator. Descriptive operation names below are inferred from observed behavior.

## Framing and transport

The framing is unchanged from 3.08: `[02:B][opcode:S][transaction:S][bodyLength:I][fields...]`, with a nine-byte header. Each field is `[tag:S][length:I][payload]`; no field count or trailing marker exists. Fields are stored in a `Hashtable` and duplicate tags replace earlier values. Enumeration order is not guaranteed. A receiver must accept arbitrary field order. Multiple packets can be concatenated in an HTTP response. See [ay.java](../artifacts/vineflower/ay.java) and original [bj field constructors/accessors](../artifacts/bytecode/bj.txt).

The primitive accessors return null for absent fields and read from the start of the raw payload. A present empty raw field is different: the byte accessor throws `ArrayIndexOutOfBoundsException`, while undersized short/int/long reads return null. A raw field can support several accessor views at once; this does not establish a single transmitted type. Strings are UTF-8 without Java `writeUTF` framing. `TA` writes a zero **between** values, with no final zero, but its original raw-field decoder can lose the final unterminated element even for ordinary nonempty strings: `a\0b` decodes as `[a]`, and `ab\0cd\0ef` as `[ab,cd]`. These are original-client decoding behaviors, not a reason to change the recorded wire bytes or assume the historical server shared the same decoder. Long values and string arrays are used by v4 contact synchronization and generic external IM accounts. The original parser trusts lengths; the supplied offline codec rejects truncated or invalid lengths. The [source audit](protocol-catalog-audit.md) records original-class probes and the limits of these observations.

| Interface | Observed behavior | Evidence |
|---|---|---|
| TCP defaults | `socket://gateway.mig33.com:25`; connection test first uses saved host/port or defaults, then swaps25↔9119 for its second attempt. Connection-selection state governs fallback | N:125,459–522; original `bc.run` bytecode for exception branches |
| HTTP base | `http://gateway.mig33.com/`; saved host can override it, adding scheme and trailing slash | N:15,459–480 |
| Binary tunnel | POST serialized packet, `Content-Type: image/gif`, byte-count Content-Length; HTTP200 required. GET polls when no packet | U:492–575 |
| Session suffix | Base path for200/100/2/904, otherwise base + session when non-null; polling also uses session suffix | U:505–513 |
| Keepalive | Idle TCP >60s: opcode2 with **1:I1**. Idle HTTP >5s: GET poll. A still-pending ping triggers reconnect | N:408–446 |
| Transactions | Int counter incremented, wire transaction `(short)(counter % 32767 + 1)`; pending vector supports several outstanding operations | N:100–106,203–235,454 |
| Scheduling | TCP400 may wait120s; connection-test2 waits45s before login/60s after login; many operations immediately continue while their result remains pending. Most other operations wait1s.300 sets logout state | N:543–602 |
| HTTP timer | 120s before login,30s after login; direct HTTPS helper has its own timeout/cleanup | U:212–225; original `u` bytecode |
| Parallel browser | Login203 can enable a separate `h` queue for922 when using HTTP. These requests can take a direct HTTP route rather than gateway tunneling | N:243–258; U:403–490 |

HTTP headers added by the shared helper are `mig33: 4.62` (the name comes from English resource193), `sw`, `sh`, `cellid`, `lac`, `mnc`, `mcc`, `ver`, `lang`, `epoch` in milliseconds, `sid` when logged in, `Cookie`, and `User-Agent` containing platform plus saved username when present. Mode1 adds `Pragma: no-cache`. These headers also apply to binary tunnel requests. U:272–305. Property fallbacks for cell ID, LAC, MCC and MNC are in J:8868–9119, including vendor-specific Nokia/Sony Ericsson/Samsung names and IMSI-derived fallbacks. Missing information commonly becomes an empty string.

The ordinary HTTP helper adds `Cookie` even when the assembled value is empty, because the condition tests null, not emptiness. Its header name `mig33` is technically the current language packet's string193; the bundled English value is `mig33`. The separate analytics client below has a different header set. Neither path sets an `Authorization` header or implements OAuth/Bearer authentication. Gateway authentication is the binary handshake; web requests use its session string and the cookie store.

Saved connection settings are string keys25 (host) and26 (TCP port), with integer preference10 selecting HTTP1/TCP2. On successful201, HTTP saves its base after stripping `http://` and the final slash; TCP saves host/port. The HTTP selector resets the base to `http://gateway.mig33.com/`, but the network thread can subsequently reconstruct it from the saved host. The site base for feature pages is a different variable: `aj.Q` is assigned from203 tag26 and has **no hard-coded `www.mig33.com` fallback**. `y.e(relativePath,target)` returns null until that site base exists. N:459–480,1816–1833,1980–1990; J:2166; `y.java` normal-flow1029–1039.

## Login, registration, and session state

Login remains **200 →201 →202 →203**.201 provides1:T challenge and2:T session.202 sends1:I `JavaStringHash(challenge + password.toLowerCase())`. Java UTF-16 code-unit hashing and historical case conversion matter. No password field occurs in200. Credentials are first stored in the settings packet and RMS; this operation does not imply successful authentication. N:862–945,1802–1845.

### Outgoing login200

| Tag | Type | Value |
|---:|---|---|
| 1 | S | 1 |
| 2 | B | 2 |
| 3 | S | 462, computed from displayed version |
| 5 | T | Username |
| 7 | T | `J2MEv4.62.000`, from MIDlet-Version4.62 and BUILD.000 |
| 8 | T | Device platform |
| 9 | B | Existing session: current desired presence; otherwise1 or99 from invisible-login preference |
| 11 | I | Font-size setting |
| 12 | I | Canvas width−6 |
| 13 | I | Canvas height−bottom bar |
| 15 | T | Language code from settings tag6 |
| 16 | I | Config/resource revision, current config tag1 |
| 17 | T | Cell ID |
| 18? | T | Previous session string, reconnect only |
| 19 | B | 1 if settings integer17 is1, otherwise0; activity/update subscription preference |
| 20 | T | LAC |
| 21 | T | MCC |
| 22 | T | MNC |

### Incoming login203

N:1846–2077 and [original opcode203 instructions](data/protocol-opcode-203.txt) are important: this build reads some raw fields through several accessors, which must not be “corrected” from assumptions about a single field type.

| Tags | Reads and effect |
|---|---|
| 1:T,2:B | Own mobile and activation flag (`!=0`) |
| 3?:T,4:B | Announcement; mode1 text, other mode URL |
| 13?:T,14:I | Mail URL and unread count |
| 16?:I | Emoticon size; updates preference and clears cache when changed |
| 18?:B | Read and discarded |
| 20:I | Contact-cache revision, compared with saved revision |
| 23:T | Stored settings string tag9 |
| 24?:I | Client value `aj.S`; retain exact value; not independently identified |
| 26:T | URL from which site base `aj.Q` is cut at first slash after position7 |
| 27:B | Read and compared with2; no persistent effect in recovered normal branch |
| 28:T | Own profile string `aj.n`, later also updated by603 tag3 |
| 29?:B | Absent or1 triggers device/system information upload10 for every string from `aj.l()` |
| 31:I and32:I | If both exist, used as avatar width/height |
| 31:S and32:T | **The same raw tags are subsequently read through different accessors**: short saved as settings11, string appended to site base and saved as settings10 |
| 33:B→I,35:B→I,37:B→I | Existence tested with byte accessor, then int accessor's low byte compared with1. Control parallel HTTP queue, feature flag `bc.a`, and feature flag `bc.b`. Absent defaults: true,false,true respectively |

Tags31/32 have multiple confirmed accessor interpretations whose combined application meaning remains uncertain. They are not an impossible binary encoding: received fields contain raw bytes, and numeric getters read prefixes. For example, raw `00010002` is int65538, short1, and a four-character control string. A fixture intended to drive this original build must account for both reads or document which branch it exercises; do not invent substitute tags. Tags33/35/37 can be represented as four-byte integers to satisfy both accessors; the initial byte read only tests non-null. See the [source audit](protocol-catalog-audit.md) for the distinction between presence checks and value reads.

After203, the client ensures room categories713 are requested. If203 tag20 equals the saved contact revision, it requests **419 with1:L saved synchronization time**, falling back to current time when no timestamp is saved; otherwise400 performs a full list.401/402 records,406 deletions,421/423 metadata changes and420 checkpoints update the persistent contact cache.403 marks completion. This is substantially different from v3's unconditional full400 download. During reconnect, some203 profile initialization is skipped, and chats retain local state.

Registration101 still inserts width and height under **the same tag1:I**, so only height survives.103 has1:T username,2:T password,3:T mobile,4:T challenge identifier,5:T entered captcha,9:T client build string and10:T the **same client build string**.102 returns1:T prompt/mobile string,2:T challenge token,3:X image;104 replaces prompt/token/image. A separate security challenge16 uses1:T prompt,2:X image,3:T token and answer17 sends1:T entered text. Registration username checking uses the local1005 HTTP job described below.

## Outgoing operation catalog

This table describes network requests and explicitly labels local jobs. A [machine-readable copy](data/protocol-outgoing.json) contains all56 entries. Fields not mentioned are absent from the concrete builder, except907 which retains a received packet. Reference N:353–406 and N:651–1124 covers all typed builders; UI helper calls are in J:1624–5197. Protocol constructor inventory includes synthetic errors/responses and RMS420 records, which are not additional server requests.

| Opcode | Purpose | Sent fields / flow |
|---:|---|---|
| 2 | Test connection / heartbeat | Manual test no fields →3; automatic heartbeat1:I1 |
| 7 | Get help | 1:I topic; recovered UI registration topic5 →6 |
| 9 | Reply to server prompt8 | 1:B1 accept /0 decline;2:S original prompt transaction |
| 10 | Upload device/system information | 1:S1;2:T one property string, repeated after203 when enabled |
| 11 | Reply to preference query12 | 1:B store type1 numeric/2 string;2:T key;3?:T value |
| 17 | Answer security challenge16 | 1:T entered text |
| 101 | Start registration | 1:I height; duplicate width tag is overwritten →102 |
| 103 | Submit registration | Five text fields + duplicate client string9/10; see above →ack/104 |
| 200 | Login | Dedicated table →201 |
| 202 | Login challenge answer | 1:I Java hash →203 |
| 204 | Activate account | 1:T activation code →ack |
| 205 | Resend/change activation mobile | 1:T mobile →ack |
| 206 | Connect external IM | 1:B service,2:B mode,3:B0 (boolean argument ignored) →207 |
| 300 | Logout | No fields |
| 302 | Disconnect external IM | 1:B service →207/ack |
| 400 | Full contact synchronization | No fields →401/402/420/403 |
| 405 | Add contact | 2?:I group;3?:T display;8?:T mobile;12?:T mig33 with24:B share-mobile flag;30?:T external username;31?:B external service →402 |
| 406 | Delete contact | 1:I contact ID →ack or push406 |
| 407 | Edit contact | 1:I contact;2?:I group;3?:T display;8?:T mobile;12?:T mig33;30?:TA external usernames +31?:X parallel service bytes. Arrays sent only if both nonempty vectors have equal lengths →402 |
| 408 | Add group | 2:T name →401 |
| 409 | Delete group | 1:I group →ack |
| 410 | Rename group | 1:I group;2:T name →ack |
| 411 | Move contact | 1:I contact;2?:I group, omitted for−1 →ack |
| 417 | Block user | 1:T username;2:B2 **always**; boolean parameter ignored |
| 419 | Incremental contact synchronization | 1:L saved timestamp, or current time when no saved timestamp exists →cache updates/403 |
| 500 | Send message/photo | Dedicated table below →ack/error |
| 504 | Upload photo | 1:S1;2:X bytes;4?:S parameter, omitted−1;5:B purpose. Ack1:T opens URL; absent URL with purpose0 opens photo home |
| 600 | Set presence | 1:B new presence;2:B old presence for rollback →ack/error |
| 601 | Set status text | 1:T new text;2:T old text →ack/error;603 may update profile |
| 602 | Set avatar | Inserts1:T new ID then1:T old ID: **old ID wins**; no tag2 is sent. Preserve anomaly |
| 703 | Join room | 1:T room; boolean argument ignored →701 room state and/or ack |
| 704 | Leave room | 1:T room →ack; suppress incoming messages while pending |
| 706 | Kick room member | 1:T room;2:T target →ack |
| 707 | Room participants | 1:T room →708 |
| 709 | Mute room member | 1:T room;2:T target →ack |
| 710 | Unmute room member | 1:T room;2:T target →ack |
| 711 | Add favorite room | 1:T room →701 |
| 712 | Remove room from category | 1:S category;2:T room →ack |
| 713 | Get room categories | No fields →714*/715 |
| 716 | Get category contents | 1:S category;2:B flag →701*/717 |
| 751 | Create group chat from private | 1:T existing peer;2:T invitee;3:B service →750 |
| 752 | Invite to group | 1:T group ID;2:T invitee;3:B service →ack |
| 753 | Leave group | 1:T group ID;2:B service →ack |
| 754 | Group participants | 1:T group ID;2:B service →755 |
| 800 | Confirm callback | 1:T origin;2:T destination;4:B0;5:B1. Both int arguments ignored →ack/804 |
| 900 | Invite friend | 1:T mobile;2:T name →ack |
| 904 | Forgot details | 2:T mobile →ack |
| 907 | Remove/update IM settings | Cached906 changed to907;1:B1; UI sets30:B selected service before request. Retains all other cached fields |
| 913 | Get emoticon | 1:T token **plus a trailing space**;2:I action mode. Pending mode1 enables914 type2 vibration/type4 audio side effects; it is not a photo-viewer mode |
| 921 | Subscribe/unsubscribe activity updates | 1:B1 opens Updates tab,0 closes/disables; errors requeue same request |
| 922 | Fetch embedded web page/image | Dedicated HTTP table below →923, gateway or local routing |
| 925 | Set language | 1:T language code after1004 |
| 926 | Get external IM icons | 1:X array of service IDs (this caller sends one byte) →927 |
| 1001 | **Local language-list job** | 1:T URL; HTTP fetch binary1002; not serialized to gateway |
| 1003 | **Local language-pack job** | 1:T URL; HTTP fetch binary1004; not serialized to gateway |
| 1005 | **Local raw HTTP job** | 1:T URL; returns synthetic1005 with1:X body; username availability check |

Generic helpers accept arbitrary opcode with no fields,1:T,1:I,or1:T+2:T; that capability alone is not evidence for additional server APIs. Generic room-member helper's arguments are `(opcode,target,room)` but emits tag1=room/tag2=target. No concrete outgoing413/414/415/700/705/805/905/910/998/999 is present in this build. For external IM settings,906 can be constructed locally as an empty cache object before907; that construction is not a request to fetch906.

### Messages500 and contacts402

| Tag | Message500 type and meaning |
|---:|---|
| 1:B | Message channel: mig33=1,SMS=2,mail=9; external channel/service mapping now comes from208 descriptors |
| 2:T | Sender username; outgoingSMS uses own mobile |
| 3:B | Conversation1 private,2 group,3 room |
| 4:T | Destination peer,group ID,room name |
| 5?:I | Contact ID; outgoing private omitted for−1 |
| 6:S | Content1 text,2 bytes/photo,5 photo reference; incoming6 selects action-style rendering when body color13 is absent/−1 |
| 8:T/X | Text or raw upload; inbound500 reads as UTF-8 text |
| 9?:B | Incoming room/group style marker; presence of field selects message style4 |
| 10?:T | Incoming avatar ID/cache key; private fetches when avatar display enabled |
| 11?:T | Emoticon triggers |
| 12?:I | Incoming sender-name RGB; missing−1/default |
| 13?:I | Message RGB; **v4 also sends this** when selected color is not−1 |
| 14?:T | Extra incoming string forwarded into chat model; preserve exactly pending rendering semantics |

Original `s.a(...)` offsets84–111 select theme category5, remove the displayed sender/colon prefix and disable the avatar prefix for incoming content6 when body RGB is−1. Category5 uses theme field148, with bundled/fallback brown `#993300`. This establishes the rendering contract, not a server command catalog. Slash commands still leave the original client as content1. See the [reconstruction's command evidence and adapter behavior](../../../../docs/slash-commands.md), including special peer routing for an author's own private-action response.

402 retains1:I contact ID,2?:I group,3?:T display,8?:T mobile,12?:T mig33,13:B mig33 presence.27?:T is status text (`t.h`);28?:T avatar key (`t.p`). These semantics follow the original setter keys2/91 through the contact text/image renderer; see the [checked bytecode evidence](data/protocol-audit-contact-metadata.json) and [semantic audit correction](protocol-catalog-audit.md#contact-statusavatar-semantic-correction-2026-10-31). Generic external accounts use31:X service IDs,32?:X presence bytes (defaults99),33:TA usernames. The normal path chooses the mig33 branch when12 is present and the generic external branch otherwise; do not assume one402 mixes both. Arrays are positional, so lengths and ordering matter.404 uses1:I contact,2?:B mig33 presence,9?:X external service IDs,10?:X corresponding presences. Presence values remain1 online,3 busy,4 away,99 offline/invisible. N:2164–2381; [t.java](../artifacts/vineflower/t.java).

## Incoming operation catalog

Every top-level case in the recovered dispatcher appears here. Some require a matching pending transaction; pushes such as500 or menu/theme updates do not.0 errors and1 acknowledgments dispatch further on the pending request opcode. Unknown top-level opcodes fall through. Each row has its original bytecode evidence in `data/protocol-opcode-<number>.txt`.

The machine catalog records immediate constant-tag getter reads in this dispatcher, not a complete response schema. Its `all_getter_reads` also includes pending-request/settings reads. In particular,906 tag30 is read through a stored response alias,1002 uses three computed text tags, and502/927/1000/1004 have delegated or deferred consumers. The [incoming audit](data/protocol-audit-incoming.json) verifies all55 existing rows against the original bytecode and records those additional paths without changing the meaning of `direct_fields`. An empty `direct_fields` array is not evidence that an operation has no payload.

| Opcode | Fields consumed and result |
|---:|---|
| 0 | 2:T error; contextual dialog/chat/cache rollback.600 rollback reads request2 as **I although sent B**;602 rollback asks request2:T although builder overwrites1 and sends no2. Error branch anomalies are preserved |
| 1 | Contextual1?:T text/URL; completes pending transaction. Specialized behavior for SMS/mail/photo, mute/unmute, registration, callback, language and contact operations |
| 3 | No fields; successful connection test/ping. Persists host/port/mode during connection setup |
| 5 | 1:B subtype,2:T text/URL,3:B presentation,4?:T title/auxiliary string; queued notification model type2 |
| 6 | 1:T help; matched7 topic5 registration help |
| 8 | 1:B prompt subtype,2:T body,3:T auxiliary URL/text,4:T title,5?:I auxiliary value(default0); queues model type1 with source transaction; reply9 |
| 11 | 1:B store type,2:T key,3:T value. Type1 parses numeric key/value into integer preferences;type2 stores strings |
| 12 | 1:B store type,2:T key; client responds11 with stored value |
| 14 | 1?:T contact-list banner content,2?:T banner link; caches packet and updates contact UI; missing1 clears it |
| 16 | 1?:T security prompt,2?:X image,3?:T token; enters screen60; reply17 |
| 102 | 1:T registration prompt/mobile,2:T token,3:X captcha; opens registration form |
| 104 | 1:T failure/prompt,2:T replacement token,3:X replacement captcha; replaces registration challenge |
| 201 | 1:T challenge,2:T session; queues202 |
| 203 | Login result; detailed table and cache synchronization above |
| 207 | 1:B external service,2:B1 connected/other disconnected,3:T error when disconnected; updates dynamic descriptor |
| 208 | 1:B service ID,2:T name,3:B message-channel mapping,4:B state,5?:B extra descriptor flag(default0); constructs registry entry, queues926 if uncached icons, evaluates auto-login |
| 301 | 1:T disconnected reason; clears/retains session state through reconnect UI |
| 401 | 1:I group ID,2:T name; updates contact group and RMS; completes408 |
| 402 | Contact record; table above; completes405/407 |
| 403 | No fields; contact synchronization complete; persists pending checkpoint and updates UI |
| 404 | 1:I contact,2?:B mig33 presence,9?:X external services,10?:X external presences |
| 406 | 1:I deleted contact; removes cache and associated conversation |
| 420 | 1:I contact revision,2:L checkpoint time; matched synchronization stores checkpoint pending completion, unsolicited packet applies immediately |
| 421 | 1?:I contact(default−1),2?:T status text(defaultempty),3?:L checkpoint time; **presence of3 tested with text accessor before long read** |
| 422 | 1:I age in seconds,2:B feed item type,3:T avatar/cache key,4:T small HTML/text item; appends activity entry and marks Updates tab unread. `av.a` computes timestamp as current time−age*1000 |
| 423 | 1?:I contact,2?:T avatar/cache key,3?:L checkpoint; same3 text-presence check as421 |
| 500 | Message; dedicated schema above |
| 502 | Queues whole packet; delegated UI reads4:T notification and5:T URL on acceptance (J:289,1677) |
| 503 | 1:I unread mail count |
| 603 | 1?:T own status saved settings4;2?:T own avatar saved settings3;3?:T profile string `aj.n`;4?:T own profile data saved settings12 |
| 701 | 1:T room,3:I capacity,4:I occupancy,6:S category for lists. A reply to703 instead builds room conversation using8?:I community ID(default0; positive enables Group Menu),10?:T `s.h`,11?:I theme ID,12?:T `s.i`.711 reply adds favorite |
| 708 | 2:T participants,3?:T admins,4?:T muted users; semicolon-terminated lists;5?:B service(default1) |
| 714 | 1:S category ID,2:T label,3:B,4:B,5:B category flags;4 controls expansion and is combined with locally saved category state |
| 715 | No fields; categories complete; appends search URL and schedules category contents |
| 717 | 1:T category completion/footer string; category taken from outstanding716 tag1 |
| 718 | 1:T label,2:T URL; adds room-list link |
| 719 | Room theme:1:I theme ID,2:I background property,3:T imageURL,4–15:I style values. Mapping to exact fields below |
| 750 | 1:T group ID,2?:T original peer,3?:B service(default1); converts private conversation to group |
| 755 | Same schema/selector behavior as708 |
| 804 | 1:T dial string,2?:T confirmation; ultimately `platformRequest("tel:"+dial)` |
| 906 | Whole packet cached;30:B selected external service read by handler. UI removal changes30 and sends907. Old v3 credential-tag layout is not interpreted here |
| 911 | 1:I menu category,2:I action,3:T label,4:T auxiliary/navigation value; category mapping below |
| 914 | 1:T identifier,2:T aliases,3:T label,4?:X image/content,5?:B type(default1,forced0 if no4). Type1 static image;type2 image plus vibration;type3 framed animation;type4 image plus framed WAV audio. See content formats below |
| 915 | No fields; clear emoticon cache |
| 916 | 1:T standard triggers,2:T other triggers; replaces token vectors |
| 918 | 1:T URL,2:B boolean navigation flag (`==1`); opens embedded page |
| 920 | 1:I reset:1 contacts tab;2 reload config/theme;3 clear emoticons;4 reload language and UI;5 rebuild menus/tabs |
| 923 | 1:I HTTP status,2?:X body,3?:L cache/time value,6?:I asset route selector,11?:TA Set-Cookie strings. See web handling below |
| 924 | 2:T semicolon-separated decimal integer chat colors; client prepends0, populates the grid and enables Change Text Color when multiple choices exist. It does not automatically open the picker |
| 927 | Delegates to `aj.c(ay)`:1:B service,2–6?:X images.2→`bj.f:Image`,3 decoded then discarded,4→`bj.a:Image`,5→`bj.g:Image`,6→`bj.i:Image`; successful definitions cached |
| 928 | No fields; closes special processing state/repaints |
| 1000 | Whole config/theme resource packet saved to RMS and installed as `aj.j`; arbitrary tags interpreted by rendering/resource accessors |
| 1002 | Language catalog triples1:T code,2:T display name,3:T URL;4/5/6 etc. Loop uses field count and indices congruent1 mod3; use dense tags |
| 1004 | Whole language packet installed,1:T language code; logged-in client sends925 and reloads categories713 |
| 1005 | 1:X HTTP body decoded with platform default and passed to username-check response parser |

### Emoticon914 content formats and side effects

The reply must correlate to a pending913. Tag5 is the **content type**, independent of the request's tag2 action mode. Types1 and2 take raw MIDP-decodable image bytes in tag4. Type2 additionally vibrates for500ms when request mode is1 or when an already-cached token is rendered through the matching action path. It is not an animated-photo viewer: original `aj.d(int)` calls `Display.vibrate(500)` and ignores its integer parameter.

Type3 carries a big-endian32-bit frame count, followed by that many big-endian32-bit image lengths and image byte arrays. Type4 carries a big-endian32-bit image length and image bytes, then a big-endian32-bit audio length and audio bytes. The preview comes from the first image; the second stream is handed to `Manager.createPlayer(...,"audio/x-wav")`. Sending a plain JPEG/PNG as type4 is invalid. Missing tag4 forces type0 in the dispatcher; this does not establish a useful zero-byte image format.

Incoming500 tag11 provides space-separated special-token aliases. The renderer matches them against body8, inserts its internal markers and requests uncached tokens through913 with the original trailing space. A static image token can use914 type1; this establishes inline image rendering, not a full-screen photo gallery or its dimensions. Photo-reference outgoing500 content5 and upload504 are separate contracts.

Evidence: [914 dispatcher bytecode](data/protocol-opcode-914.txt); [aj.txt](../artifacts/bytecode/aj.txt), methods `a(byte[]):Image[]` at14560, `b(byte[]):Object[]` at14644 and `d(int)` at17169; [bd.txt](../artifacts/bytecode/bd.txt) for WAV playback. Reading aids: `aj.java` lines5532–5573,6377–6379,8430–8473; `bc.java` lines2946–2974; `s.java` lines736–780 and1190. See [text-and-page-formatting.md](../../../../docs/text-and-page-formatting.md) for the distinction between chat metadata and HTML pages.

### Dynamic menu911 and room theme719

Joined-room701 tag8 is a community ID, distinct from the private-group chat identifier returned by750. Original `bc.a:([Lay;)V` reads the optional integer at3071–3095, defaults it to0, and passes it as the final argument to `s.<init>:(Lt;ILjava/lang/String;ILaj;I)V` at3200–3201. That constructor stores it in `s.a` at133–136. `aj.U:()Ly;` includes Group Menu only when `s.a > 0` at51–71; `aj.commandAction:(Ljavax/microedition/lcdui/Command;Ljavax/microedition/lcdui/Displayable;)V` appends this value as `&cid=` for the community chat-room route at3118–3175 and adjacent forum/member routes. See the [semantic audit](protocol-catalog-audit.md#joined-room701-community-id-refinement-2026-10-31). This identifies the existing field; its tag, integer width, optionality and transport remain unchanged.

Menu category mapping is **different from3.08**:1→`aj.ar` own-profile menu;2→`aj.as` home/contact-list menu;3→`aj.at` Updates menu;4→`aj.au` contact context menu;5→`aj.av` group context menu;6→`aj.aw` Settings;7→`aj.ax` embedded page;8→`aj.ay` room listing;9→`aj.az` private chat;10→`aj.aA` group chat;11→`aj.aB` room chat. Compare N:2941–2979 with J:6028,7274–7469,7697,8106–8153. Category values12–14 used by v3 no longer have branches.

719 maps to nullable `Integer` fields in merged `bj`;3 is imageURL `c:String`. Room701 field11 chooses this theme by ID and stores the object in `s.k`. If a field is null, its getter falls back first to the global theme packet, then to a hard-coded value. N:2814–2845; [bj setters](../artifacts/vineflower/bj.java), line419 onward; [normal-flow getters](../artifacts/normal-flow/bj.java), lines1801–1904.

|719 tag|Field/getter|Global fallback tag|Hard-coded fallback|
|---:|---|---:|---|
|2|`b:Integer` / `b()`|139|16777215 (`#ffffff`), background|
|4|`e:Integer` / `e()`|141|40, background/layout dimension|
|5|`f:Integer` / `f()`|142|550964 (`#086834`)|
|6|`g:Integer` / `g()`|143|0 (`#000000`)|
|7|`h:Integer` / `h()`|144|4221632 (`#406ac0`)|
|8|`i:Integer` / `i()`|145|0|
|9|`j:Integer` / `j()`|146|16565508 (`#fcc504`)|
|10|`k:Integer` / `k()`|147|0|
|11|`l:Integer` / `l()`|148|10040064 (`#993300`)|
|12|`m:Integer` / `m()`|149|16711680 (`#ff0000`)|
|13|`n:Integer` / `n()`|150|16741120 (`#ff7300`)|
|14|`o:Integer` / `o()`|151|0|
|15|`p:Integer` / `p()`|138|10461087 (`#9f9f9f`)|

Renderer `y.a(category,theme)` selects sender color and `y.b(category,theme)` body color. Categories0→gettersf/g,1→h/i,2→m/m,3→n/o,4→j/k,5→l/l,6→global theme112 (default0),7→p/p. This preserves independent message categories and their fallback behavior without guessing server-facing names for every category. See [y.java](../artifacts/normal-flow/y.java), lines1245–1305.

## Embedded page and resource APIs

### Request922 / response923

| Request tag | Value |
|---:|---|
| 1:B | 1 GET,2 POST |
| 2:T | URL |
| 3?:T | Form body; presence selects POST |
| 4:I | Purpose1 page,2 image/asset,3 auxiliary fetch |
| 5?:T | Local asset/cache key; distinguishes generic browser image callback from application asset callback |
| 6:I | 0 in the recovered builder; response6=1 selects alternate asset installer |
| 7:T,8:T,9:T,10:T | Cell ID,LAC,MCC,MNC |
| 11?:TA | One string containing assembled Cookie header, if nonempty |

`ay.b:bl` is a **local Java callback reference**, not a wire field. Request922 can be sent to the gateway over TCP or the binary HTTP tunnel. With the parallel HTTP queue it can instead perform a direct web request. URLs starting with `https` always enter a direct-fetch branch. The direct branch creates a synthetic923 with matching transaction; HTTPS helper status is200 on byte result/404 otherwise. The HTTP parallel path follows301/302 using Location, returns actual nonredirect failure status, and preserves request5/6 for asset handling. U:388–610.

The HTML parser also constructs a minimal922 object for `<body background="…">` containing1:B1,2:T resolved URL,4:I2 and stores it in its internal vector (`s.java`, normal-flow2281–2288). This construction must not be mistaken for the full network builder or evidence that its other metadata fields are mandatory for every922 object.

Response923 first stores11:TA cookies against the original URL. For purpose1, status200 sends raw body to the browser and multiplies3:L by1000 for its cache timestamp; non200 generates an error/retry HTML form and passes3 without that multiplication. Purpose2 with no request5 invokes browser image callback with3:L unchanged, default−1; with5 it installs application image bytes and uses response6:I1 for alternate installation. Purpose3 simply removes the pending request. N:3053–3150.

For browser images, `bl.a(String,long,byte[])` adds that unchanged value to `System.currentTimeMillis()`: tag3 is therefore a **millisecond lifetime**, with−1 meaning no expiry. Zero expires immediately and can produce a broken image even after a successful PNG response. This differs from the successful HTML-page branch's seconds conversion. The local server gives successful purpose2 image responses60000ms while HTML and errors retain0; that lifetime is a reconstruction choice. Original `bl.java` normal-flow260–299 and its bytecode establish the cache behavior.

Form POST uses `application/x-www-form-urlencoded` and platform-default `String.getBytes()`. The parallel direct922 branch measures encoded byte length; the separate HTTPS/private helper retains a character-count Content-Length quirk. The shared cookie helper is not a modern browser cookie implementation: it groups by a host-substring match, stores all semicolon-separated components as key/value entries (including cookie attributes), ignores usual path/expiry/security rules, limits approximately1024 bytes, and omits cookies for URLs containing `img.mig33.com`. Exact behavior is in [ac.java](../artifacts/vineflower/ac.java).

### Hard-coded paths and local jobs

The complete literal inventory is [protocol-endpoints.json](data/protocol-endpoints.json), including repeated references and `%s` placeholders. It contains62 distinct strings:59 URL/path literals plus the three scheme fragments `http://`, `https://`, `socket://`. It is not a count of62 independent services. Generated socket addresses, session suffixes, image URLs and the manifest installer notification need the rules below. URLs supplied by the server, HTML forms, menu911 or packets918/203/502 are additional interfaces whose historical content is not bundled.

| Family | Recovered path/actions |
|---|---|
| Language list | `http://www.mig33.com/sites/resources/language/v4.6/l`; local1001 downloads binary1002; its selected URL drives1003/binary1004 |
| Registration check | `http://www.mig33.com/sites/index.php?c=registration_api&v=midlet&a=check_username&username=%s`; local1005 returns raw text |
| External help | `/sites/index.php?c=help&v=wap&a=apn` and `a=tc`; platform browser |
| Profile | `c=profile`: home,search,edit,user_blog_posts; optionalusername |
| Photos/avatar | `c=photo`:home,sendfromphoto; `c=photos&a=wall`; `c=avatar&a=home` |
| Groups | `c=group`:list_my_groups,list_groups_home,add_to_group,list_chatrooms,members,view_forum,home; cid/roomName parameters |
| Rooms | `c=chatroom`:search,setup,create; roomName |
| Calls/SMS/mail | `c=call&a=home&f=call&number_to_call=…`;`c=sms&a=home&f=sms&to_number=…`;`c=email&a=compose&to=…` |
| Account | `c=account`:home,recharge_credit,get_history,transfer_credit; recipient_username |
| Store | `c=store`:home,main; e=3,ty=1/5,username or id111 |
| Settings | `c=settings`:profile_privacy_settings,view_update_settings,call_by_name,block_list,change_password,change_mobile,change_tab_order,chat_privacy |
| Other | migworld/get_mobpage,leaderboard/home,bot/bot_list,merchant/dashboard,help/main/contact_us,report/report_abuse,lookout/home,buzz/home,location/checkin,im/setup_im |

Additional URL constructions and non-request literals require separate treatment:

| Interface | Exact construction and behavior | Evidence |
|---|---|---|
| Browser home/reset **cache key** | `http://www.mig33.com/sites/c=migworld&v=midlet&a=new_home`. `bl.c()` uses this literal to find/create a local page model and update history/cache bookkeeping; that method does **not** invoke an HTTP fetch. The literal lacks `index.php?`, but it is not evidence of a requested endpoint | `bl.java` normal-flow349–362 and private cache/history helpers; original `bl.txt`1048,1061,1086,1091 |
| Avatar/application image | For an identifier not beginning `http`, concatenate `assetBase + id + "?w=" + width + "&h=" + height + "&a=1&c=1"`. `assetBase` comes from203 tag23, persisted in settings packet9. Absolute `http…` identifiers bypass this construction. The square helper uses global `G`, initially25 | J:6306–6311; N:1987; original `aj.txt`16927,16958 |
| Image host special case | `img.mig33.com` appears only as a cookie-suppression match. This does not establish that every image uses that host; the asset base is server supplied | [ac.java](../artifacts/vineflower/ac.java), line26 |
| Installer notification | `http://www.mig33.com/midlet/member/c_info.php?awtrack=wap` is `MIDlet-Install-Notify`; it belongs to the handset installer contract, not the application's gateway/page request builders | [MANIFEST.MF](../artifacts/extracted/META-INF/MANIFEST.MF), line6 |
| Analytics | Direct GET `http://www.mig33.com/sites/lib/ga_midlet.php?utmr={previousAction}&utmp={currentAction}&utmvid={visitorId}&hash={digest}`; details below | `bj.java` normal-flow580–692; original `bj.txt`1448–1490 |

The analytics worker has16 action names listed at the start of `bj.java`, covering welcome/login/language/connection and registration steps/results. Its visitor ID is settings packet14, initialized from the current millisecond time when absent. The digest is lowercase hexadecimal from an MD5 implementation fed the platform-default bytes of `"gamerz-" + visitorId`. **The digest state is reused across calls**, including a discarded constructor call: this is not equivalent to a stateless MD5 calculation on every request. Preserve the original state machine if reproducing analytics exactly. `bj.java` normal-flow556–574,1639–1665,2547–2569,2633–2652.

For a constant visitor ID, digest call number `n` hashes that input repeated `n` times. The first network attempt normally uses `n=2`; a fresh retry appends again, while following a redirect does not reconstruct the hash. Offline invocation of the original digest class verified visitor ID `123`: calls1/2/3 return `d987dcedad3a0f70182b352333b73c9e`, `6ea152b7887572197244b0aa3b7844a3`, and `5439d948de42d7f5913c7dac8dd300c4`, matching MD5 of `b"gamerz-123" * n`. This isolated check did not initialize the MIDlet or connect to a service.

Analytics independently sets `mig33`, `sw`, `sh`, `cellid`, `lac`, `mnc`, `mcc`, `ver`, `lang`, `epoch`, and `User-Agent` containing only the platform. It does not add `sid`, `Cookie`, or `Pragma`. It follows301/302, accepts200, and response text exactly `1` disables the worker via persisted settings13. Analytics is not part of the binary login authentication protocol.

Most paths include `v=midlet`; consult the literal inventory to preserve exact parameter order, spelling, optional suffixes and substituted values. `y.e(path,target)` resolves against the site base derived from203 and may substitute username/target. Changing the endpoints into invented JSON APIs would not reproduce the client. The embedded browser remains a constrained HTML implementation with forms, links, images and custom tags, without JavaScript; its complete layout/parser behavior belongs to the screen specification.

Username-check1005 is not parsed by a JSON library. A response of length≤2 advances to registration101; longer responses are sliced from the first `[` through the penultimate character, split by comma, and displayed as suggested usernames. Preserve the source parser's quoting/whitespace behavior if supplying historical-style fixtures. J:5445–5477.

### Web-to-native `mig33:` commands

J:5173–5374 intercepts links beginning exactly `mig33:`. It reads the method before the first `(` and treats everything between that parenthesis and the final character as one argument. No escaping, nested-expression parser, JavaScript evaluation or general URL-decoding appears in this dispatch. The ordinary link path resolves against the site base and opens922. These commands are essential for reproducing server-generated screens that hand control back to the native UI.

| Method | Argument and effect |
|---|---|
| `checkIn` | Location/check-in argument forwarded to location handler |
| `reorderHomeTab` | Integer passed to tab-order method; repaint on success |
| `auth` | Argument unused; opens activation screen41 |
| `sendScrapbookPhoto` | Photo reference; sends500 content5 to saved target conversation, appends local confirmation and restores chat |
| `uploadPhotoToScrapbook` | Argument unused; opens photo source menu26 |
| `invokeNativeBrowser` | URL passed to platform browser |
| `setDisplayPicture…` | Method-prefix match; opens photo source menu55 |
| `inviteToMig33` | Argument unused; opens invite workflow |
| `inviteFriend` | Argument forwarded to friend invite handler |
| `privateChat` | Username; opens native mig33 conversation |
| `call` | Number; opens call HTML path |
| `sms` | Number; opens SMS HTML path |
| `updateStatus` | Argument unused; opens native status editor |
| `joinGroupChatroom` | Splits comma and joins first element as room |
| `joinChatroom` | Room name; queues703 |
| `closeTab` | Closes current tab, then focuses0 contacts/1 Updates/2 Rooms if present |
| `focusTab` | 0 contacts/1 Updates/2 Rooms/3 last chat; creates Updates or Rooms tab if necessary |
| `close` | Argument unused; invokes tab/browser close method |
| `newBrowser` | URL; opens a new embedded browser |
| `imMenu` | Argument unused; opens external IM menu |

Unknown method names have no matching action. Names are case-sensitive. Besides this20-method dispatch, browser forms/custom tags may hand back photo bytes or navigation values through other native callbacks; use the screen parser implementation for those element-specific rules.

## Persistence, anomalies, and reconstruction boundaries

The settings/config/language/contact cache use serialized packets in RMS as well as scalar preferences. A packet present in a RecordStore is not necessarily a network operation. Local420 checkpoint records share a layout with synchronization packets. See [j.java](../artifacts/vineflower/j.java), [au.java](../artifacts/vineflower/au.java) and the screen/state reference. Store credentials, presence, language, avatar/status, group expansion, revision/checkpoint and server-provided menus as distinct state.

The original anomalies relevant to exact reproduction are101's duplicate dimension tag,602's duplicate avatar tag and missing rollback2,600's byte-written/int-read rollback2,203's reused31/32 and mixed accessor types,421/423's text null-check before long read,103's identical9/10 client strings,417's ignored block boolean,800's ignored mode arguments, and913's appended space. These are evidenced by original instruction descriptors or original-class serializer fixtures, not merely inferred from broken Java output. Error paths are especially sensitive to failed accessors; use original exception tables when matching crashes/recovery.

Server-provided theme1000, language1004, menus911, theme719, activity422, external IM208/927 and HTML pages can change both screen content and flow without changing this JAR. Bundled defaults establish the shipped appearance; they cannot prove the last historical server response. Registration policy, account activation logic, message routing guarantees, moderation rules, credit/call billing, HTTP form backend validation, dynamic response bodies and current endpoint availability are not recoverable from the client alone.

## Offline fixtures and verification

[protocol-codec.py](../../../../tools/applets/v4/protocol-codec.py) validates framing, typed payloads, Java hash behavior and captured serialization. [protocol-fixtures.json](data/protocol-fixtures.json) contains nine byte-exact packets generated with **original** `ay`/`bj` classes by [protocol-fixture-harness.java](../../../../tools/applets/v4/protocol-fixture-harness.java); [raw output](data/protocol-original-fixtures.txt) is retained. The harness does not initialize the MIDlet or connect to services. Run `python3 tools/applets/v4/protocol-codec.py --self-test` from the workspace.

Five common examples—ping,error,ack,UTF-8 chat,and duplicate registration tag—are byte-identical to the corresponding original3.08 serializer output. New fixtures exercise64-bit contact checkpoints,NUL-separated external accounts,colored chat,and duplicate avatar tags. These checks validate the codec and anomalies; they do not simulate login or establish complete server compatibility. The [version comparison](../../shared/protocol-differences.md) separates unchanged framing from incompatible state/API changes.
