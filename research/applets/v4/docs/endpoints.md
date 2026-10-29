# mig33 4.62 API entry points and endpoint catalog

This reference targets **Java ME 4.62 build .000**, the preserved keypad client. Its SHA-256 is `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`. Numeric packet schemas, authentication, responses and protocol quirks are in the companion [protocol reference](protocol.md).

For the runnable replacement server and the subset currently implemented, see the [Go backend guide](../../../../docs/backend.md). This catalog describes the recovered client contract; a listed endpoint is not a claim that every historical feature has been rebuilt.

The core API carries numbered **binary packets over TCP or an HTTP tunnel**. Profile, account, store and other feature pages use HTTP URLs and server-provided markup. A page that opens a form does not reveal that form's eventual submit URL, fields or backend validation. There is no recovered REST/JSON login, message or room API to list.

The original string inventory contains **62 literals: 52 relative page templates, 7 absolute URL/prefix literals, and 3 scheme fragments**. The scheme fragments are not endpoints. Multiple templates can invoke the same controller/action, and servers can provide further URLs. These counts describe client evidence, not the historical backend's total number of endpoints.

Use [endpoints-classified.json](data/endpoints-classified.json) for the classified catalog and bytecode references. The unchanged [raw inventory](data/protocol-endpoints.json) preserves every original literal. [Route construction notes](data/endpoint-route-notes.json) explain runtime substitutions. All analysis is offline; current availability of legacy domains is not asserted.

## Network bases and transport

| Entry point | Transport / method | What it carries |
| --- | --- | --- |
| `socket://gateway.mig33.com:25` | TCP byte stream | Default binary gateway: login, contacts, messages, rooms, presence and configuration |
| `socket://gateway.mig33.com:9119` | TCP byte stream | Alternate port in the connection-test sequence; the second attempt switches between 25 and 9119 |
| `http://gateway.mig33.com/` | POST for packets; GET for polling | HTTP fallback for the same binary protocol |
| `{HTTP_GATEWAY}{session}` | POST / GET | Session-specific tunnel URL, assembled by direct concatenation |
| `{WEB_BASE}sites/index.php?...` | GET for built-in navigation; form-dependent requests thereafter | Embedded feature pages; base ends in `/`, and the resolver removes the leading slash from the relative template |
| `{ASSET_BASE}{assetId}?w={width}&h={height}&a=1&c=1` | Image fetch through 922/923 or the direct HTTP path | Avatars and other images resolved from server-provided identifiers |

`HTTP_GATEWAY` defaults to the URL above and includes a trailing slash. Saved/manual host and port settings can override the defaults. Tunnel requests **200, 100, 2 and 904** use the base URL without a session suffix; other requests append the session when it exists. Polls also use the session URL. Opcode 100 occurs in the transport's exception list even though this build has no recovered concrete outgoing builder for it.

`WEB_BASE` is derived from **login response 203, field 26**: the code takes the substring through the first `/` found from character offset 7. It is not a general URI parser. There is **no hard-coded default website base** for the relative feature routes. Before `aj.Q` is initialized, `y.e(path, substitution)` returns null, even if its input is absolute. Do not infer the live website host solely from other hard-coded `www.mig33.com` links.

`ASSET_BASE` comes from **203 field 23**, persisted as settings field 9. Asset identifiers beginning with `http` pass through unchanged. One image builder accepts explicit dimensions; the second uses `G` for both dimensions. The literal `img.mig33.com` is used to suppress cookies for matching URLs; it is not a complete fixed image endpoint.

Sources: [bc.java](../artifacts/normal-flow/bc.java), lines 125, 459–522 and 1980 onward; [u.java](../artifacts/normal-flow/u.java), lines 119 and 505 onward; [aj.java](../artifacts/normal-flow/aj.java), lines 6306–6311; [ac.java](../artifacts/vineflower/ac.java). Normal-flow source omits exception handlers for analysis; original [bytecode](../artifacts/bytecode/) remains authoritative.

## How requests and authentication work

1. **Connect/test:** open the selected gateway; a manual test sends packet 2 and expects 3.
2. **Login:** send 200 with username, client/device and preference fields. Receive 201 with challenge and session. Send 202 field 1 as the signed Java hash of `challenge + password.toLowerCase()`. Receive 203 with login state, website/asset bases and configuration flags.
3. **Synchronize:** request full contacts with 400, or incremental contacts with 419 when the saved revision matches. Request room categories with 713. Configuration, language and service descriptors supply additional state.
4. **Open a page:** resolve one of the templates below and create 922. Its method field is 1 for GET or 2 for POST; field 2 is URL, optional field 3 is the form body, and field 4 selects page/image/auxiliary handling. The transport may forward it through the gateway or fetch the URL directly. Response 923 contains status, bytes, cache metadata and cookies.

The full packet envelope is `[0x02][opcode:int16][transaction:int16][bodyLength:int32]`, followed by `[tag:int16][length:int32][value bytes]` fields. Numbers are big-endian; strings use raw UTF-8. Duplicate tags overwrite earlier values. The HTTP tunnel posts these bytes with **`Content-Type: image/gif`** and expects binary packets in its response. The content type does not make the payload an image.

The shared HTTP helper sends:

| Header | Recovered value / condition |
| --- | --- |
| `mig33` | `4.62`; the header name is actually language string 193, whose bundled English value is `mig33` |
| `ver` | MIDlet version `4.62` |
| `sw`, `sh` | Client dimensions |
| `cellid`, `lac`, `mnc`, `mcc` | Available device/network values, often empty when unsupported |
| `lang` | Saved language code |
| `epoch` | Current client time in milliseconds |
| `sid` | Session string when the logged-in flag is set |
| `User-Agent` | Constructed platform string, with saved username appended when present |
| `Cookie` | The client's assembled cookie string, including an empty value when returned by its helper |
| `Pragma` | `no-cache` in helper mode 1 |

These headers also apply to the binary HTTP tunnel. The separate analytics request has a different header profile, described below. There is no OAuth/Bearer flow in the recovered client. The old cookie parser uses host-substring matching and does not implement modern cookie path, expiry or security semantics; see the protocol reference before reproducing it.

For error handling, distinguish transport errors, binary **0** (error text in tag 2), binary **1** (acknowledgment interpreted using the pending transaction), and **923**'s HTTP status. An HTTP 200 tunnel response can still contain a binary application error. Complete request fields and matching response cases are in [protocol.md](protocol.md#outgoing-operation-catalog).

## Hard-coded absolute URLs

| ID | Exact URL or prefix | Method / expected result |
| --- | --- | --- |
| L054 | `http://gateway.mig33.com/` | Binary tunnel POST / GET polling |
| L055 | `http://www.mig33.com/sites/c=migworld&v=midlet&a=new_home` | **Internal document/cache/history key** in `bl.c()`; that method does not fetch it. It has neither `index.php` nor `?` |
| L056 | `http://www.mig33.com/sites/index.php?c=help&v=wap&a=apn` | Handed to the handset's external browser for connection help |
| L057 | `http://www.mig33.com/sites/index.php?c=help&v=wap&a=tc` | Handed to the external browser for terms and conditions |
| L058 | `http://www.mig33.com/sites/index.php?c=registration_api&v=midlet&a=check_username&username=%s` | Local job 1005 performs a GET after substitution; raw text feeds the suggested-username parser |
| L059 | `http://www.mig33.com/sites/lib/ga_midlet.php?` | Prefix for a separate analytics GET; query construction below |
| L060 | `http://www.mig33.com/sites/resources/language/v4.6/l` | Local job 1001 GET; binary packet 1002 containing language code/name/pack-URL triples |

The language-pack URLs themselves come from packet 1002. Local job 1003 fetches the selected pack and installs binary packet 1004. Jobs 1001, 1003 and 1005 are client-side queue identifiers; they are not three additional gateway endpoints.

The username-check response is **not parsed with a JSON parser**. A body of length at most 2 advances to registration; a longer body is sliced from the first `[` to the penultimate character and split by commas. Original quotation/whitespace quirks matter. Server validation and a complete error schema are not recoverable from that parser alone.

The legacy browser-home literal is preserved because it occurs in executable bytecode. Its initialization call site creates a local page model and updates cache/history without issuing a network request. It must not be counted as a verified remote endpoint. The regular migWorld route appears separately below.

## Complete built-in feature page templates

Every row below is an exact relative literal, retaining query order, capitalization and `%s`. These are **initial GET navigations** from built-in actions, carried through the page transport. Further form submissions are defined by the returned markup. Opening `change_password`, `create` or `transfer_credit` should not be modeled as a documented state-changing request with invented parameters.

Relative routes resolve against `WEB_BASE`. `v=midlet` is the feature-page rendering mode. `%s` is a client substitution marker, not a server query parameter. In some rows it represents an entire query suffix. The two migWorld query orderings are separate literals but address the same controller/action.

<!-- BEGIN GENERATED PAGE ROUTES -->

| ID | Page opened | Exact relative URL template | Original bytecode |
| --- | --- | --- | --- |
| L001 | Account history | `/sites/index.php?c=account&a=get_history&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 6636 |
| L002 | Account overview | `/sites/index.php?c=account&a=home&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 6626 |
| L003 | Recharge credit | `/sites/index.php?c=account&a=recharge_credit&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 6646 |
| L004 | Transfer credit page | `/sites/index.php?c=account&v=midlet&a=transfer_credit` | [aj.txt](../artifacts/bytecode/aj.txt), line 4379 |
| L005 | Transfer credit page | `/sites/index.php?c=account&v=midlet&a=transfer_credit&recipient_username=%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 4369 |
| L006 | Avatar selection | `/sites/index.php?c=avatar&v=midlet&a=home` | [aj.txt](../artifacts/bytecode/aj.txt), line 6934 |
| L007 | Games / bot list | `/sites/index.php?c=bot&a=bot_list&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 7702 |
| L008 | Buzz page | `/sites/index.php?c=buzz&a=home&v=midlet%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 6696 |
| L009 | Call setup page | `/sites/index.php?c=call&a=home&v=midlet&f=call&number_to_call=%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 4413 |
| L010 | Create chat room page | `/sites/index.php?c=chatroom&a=create&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 6796 |
| L011 | Search chat rooms | `/sites/index.php?c=chatroom&a=search&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 5104 |
| L012 | Room information / settings | `/sites/index.php?c=chatroom&v=midlet&a=setup&roomName=%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 5061 |
| L013 | Compose mail | `/sites/index.php?c=email&a=compose&v=midlet%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 7909 |
| L014 | Link / add room to a group | `/sites/index.php?c=group&v=midlet&a=add_to_group&roomName=%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 5077 |
| L015 | Group home | `/sites/index.php?c=group&v=midlet&a=home` | [aj.txt](../artifacts/bytecode/aj.txt), line 5445 |
| L016 | Group chat rooms | `/sites/index.php?c=group&v=midlet&a=list_chatrooms` | [aj.txt](../artifacts/bytecode/aj.txt), line 5350 |
| L017 | Discover groups | `/sites/index.php?c=group&v=midlet&a=list_groups_home&e=3` | [aj.txt](../artifacts/bytecode/aj.txt), line 3090 |
| L018 | My groups | `/sites/index.php?c=group&v=midlet&a=list_my_groups&e=3` | [aj.txt](../artifacts/bytecode/aj.txt), line 2807 |
| L019 | Group members | `/sites/index.php?c=group&v=midlet&a=members` | [aj.txt](../artifacts/bytecode/aj.txt), line 5382 |
| L020 | Group forum | `/sites/index.php?c=group&v=midlet&a=view_forum` | [aj.txt](../artifacts/bytecode/aj.txt), line 5414 |
| L021 | Contact support page | `/sites/index.php?c=help&a=contact_us&v=midlet%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 6884 |
| L022 | Help home | `/sites/index.php?c=help&a=main&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 7712 |
| L023 | External IM account setup | `/sites/index.php?c=im&a=setup_im&im=%s&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 5708 |
| L024 | Leaderboard | `/sites/index.php?c=leaderboard&v=midlet&a=home` | [aj.txt](../artifacts/bytecode/aj.txt), line 8468 |
| L025 | Location check-in page | `/sites/index.php?c=location&a=checkin&v=midlet&f=` | [aj.txt](../artifacts/bytecode/aj.txt), line 21935 |
| L026 | Lookout page | `/sites/index.php?c=lookout&a=home&v=midlet%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 4985 |
| L027 | Merchant dashboard | `/sites/index.php?c=merchant&v=midlet&a=dashboard` | [aj.txt](../artifacts/bytecode/aj.txt), line 6874 |
| L028 | migWorld / discovery page | `/sites/index.php?c=migworld&a=get_mobpage&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 8458 |
| L029 | migWorld / discovery page | `/sites/index.php?c=migworld&v=midlet&a=get_mobpage` | [aj.txt](../artifacts/bytecode/aj.txt), line 3131 |
| L030 | Photos / scrapbook page | `/sites/index.php?c=photo&a=home&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 5599 |
| L031 | Photos / scrapbook page | `/sites/index.php?c=photo&v=midlet&a=home&f=sendfromphoto` | [aj.txt](../artifacts/bytecode/aj.txt), line 5589 |
| L032 | Photos / scrapbook page | `/sites/index.php?c=photo&v=midlet&username=%s&a=home` | [aj.txt](../artifacts/bytecode/aj.txt), line 6732 |
| L033 | migPhotos wall | `/sites/index.php?c=photos&a=wall&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 8492 |
| L034 | Edit profile page | `/sites/index.php?c=profile&a=edit&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 7692 |
| L035 | Own or selected user profile | `/sites/index.php?c=profile&a=home&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 2783 |
| L036 | Search people | `/sites/index.php?c=profile&a=search&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 8502 |
| L037 | User blog posts | `/sites/index.php?c=profile&a=user_blog_posts&v=midlet&username=%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 8478 |
| L038 | Own or selected user profile | `/sites/index.php?c=profile&v=midlet&a=home&username=%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 4487 |
| L039 | Report room / group-chat abuse page | `/sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=Chatroom&subject_id=%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 5249 |
| L040 | Report room / group-chat abuse page | `/sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=Group%20Chat&subject_id=%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 5236 |
| L041 | Manage blocked users | `/sites/index.php?c=settings&a=block_list&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 6834 |
| L042 | Call-by-name settings | `/sites/index.php?c=settings&a=call_by_name&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 6844 |
| L043 | Change password page | `/sites/index.php?c=settings&a=change_password&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 6616 |
| L044 | Tab-order settings | `/sites/index.php?c=settings&a=change_tab_order&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 6596 |
| L045 | Chat privacy settings | `/sites/index.php?c=settings&a=chat_privacy&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 8592 |
| L046 | Profile privacy settings | `/sites/index.php?c=settings&a=profile_privacy_settings&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 6864 |
| L047 | Activity update settings | `/sites/index.php?c=settings&a=view_update_settings&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 6854 |
| L048 | Change mobile number page | `/sites/index.php?c=settings&v=midlet&a=change_mobile` | [aj.txt](../artifacts/bytecode/aj.txt), line 6606 |
| L049 | Compose SMS page | `/sites/index.php?c=sms&a=home&v=midlet&f=sms&to_number=%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 4389 |
| L050 | Gift or theme store | `/sites/index.php?c=store&a=home&e=3&ty=5&v=midlet` | [aj.txt](../artifacts/bytecode/aj.txt), line 7305 |
| L051 | Gift or theme store | `/sites/index.php?c=store&v=midlet&a=home&e=3&ty=1&username=%s` | [aj.txt](../artifacts/bytecode/aj.txt), line 4521 |
| L052 | Store home | `/sites/index.php?c=store&v=midlet&a=main&id=111` | [aj.txt](../artifacts/bytecode/aj.txt), line 5114 |

<!-- END GENERATED PAGE ROUTES -->

## Constructed parameters and encoding

The [route construction notes](data/endpoint-route-notes.json) record each `%s` replacement and appended parameter at its call sites. The helper replaces the **first** `%s` using the raw supplied string; it does not URL-encode that value. Passing null leaves the marker unchanged. The normal navigation path replaces ASCII spaces with `%20`; it is not a general percent encoder.

| Route / parameter | Actual source or expansion |
| --- | --- |
| `registration_api/check_username`: `username=%s` | Username field; this calls `y.b` directly and works before the website base is set |
| `im/setup_im`: `im=%s` | Decimal selected external-service ID |
| `profile/home`, gift store: `username=%s` | Private peer, selected participant or selected roster contact's mig33 username |
| `profile/user_blog_posts`: `username=%s` | Current signed-in username |
| `photo/home`: `username=%s` | Current username for My Photos; selected contact username, falling back to self, for View Photos |
| `account/transfer_credit`: `recipient_username=%s` | Peer or participant username; **roster action uses the contact's display name** when non-null. A separate template omits the recipient |
| `call/home`: `number_to_call=%s` | Peer mobile, falling back to username; roster mobile; or native-link argument. **Participant action 106 also reads the current roster mobile**, rather than the selected participant name |
| `sms/home`: `to_number=%s` | Peer mobile or username, roster mobile, selected participant name, or native-link argument; a value need not be a phone number |
| `chatroom/setup`, `group/add_to_group`: `roomName=%s` | Current or selected room name, according to the action |
| `report/report_abuse`: `subject_id=%s` | Current group-chat ID for `Group%20Chat`; current room name for `Chatroom` |
| `lookout/home`, `buzz/home`: final `%s` | Whole suffix `&cid={contact_id}&pres={presence}`. The private Lookout branch uses peer contact ID with the cached roster contact's presence |
| `email/compose`: final `%s` | `&to={selected_username}`, or empty string for unaddressed Compose Email |
| `help/contact_us`: final `%s` | `&to={current_signed_in_username}` |
| Group `home`, `list_chatrooms`, `members`, `view_forum` | Append `&cid={community_id}` from the current conversation. This is a different kind of ID from Lookout/Buzz's contact `cid` |
| `location/checkin`: trailing `f=` | Append `HOME` for the self-menu action, or the raw `mig33:checkIn(...)` argument |

The upload-504 fallback is a distinct quirk: when no response URL is present and request purpose is 0, it calls `y.e(photoTemplate, null)`, leaving literal **`username=%s`** in the URL. Do not silently replace it with the current username and describe that as original behavior. `photo/home&f=sendfromphoto` opens a selection page while retaining its recipient in local state; it has no additional recipient query or POST body at that point.

Server custom-menu prefixes have their own rules. Contact and group menus append `cid={contact_id}&pres={presence}` or `gid={contact_group_id}`. Conversation menus append `imType={service}&cid={contact_id}`, `gcid={group_chat_id}` or `roomName={room}`. The client inserts **no leading `?` or `&`** before those first keys; the supplied prefix must include the separator.

The embedded form implementation builds name/value pairs from server-defined controls. Its default method is `get`: it appends the constructed query to the action URL. Other method values select a POST body, sent as `application/x-www-form-urlencoded` with platform-default string bytes. The original escaping is limited; the direct HTTPS helper also has a character-count Content-Length quirk. See [s.java](../artifacts/normal-flow/s.java) and the [web transport contract](protocol.md#request922--response923). Do not claim complete form schemas merely because these opening URLs are known.

### Analytics query

```text
GET http://www.mig33.com/sites/lib/ga_midlet.php?utmr={previousAction}&utmp={currentAction}&utmvid={visitorId}&hash={digest}
```

The 16 action labels are preserved in [bj.java](../artifacts/normal-flow/bj.java), lines 16–33. Visitor ID is saved setting 14, initially generated from the current time. The hash implementation accumulates input across calls: it repeatedly appends `gamerz-` plus visitor ID to its digest state. The constructor makes one discarded call, so the first network attempt uses the digest after a second append. A stateless `MD5("gamerz-" + id)` on each request would not reproduce the original behavior. The [protocol reference](protocol.md) records the offline original-class verification.

Analytics sends `mig33`, `sw`, `sh`, `cellid`, `lac`, `mnc`, `mcc`, `ver`, `lang`, `epoch` and a platform-only User-Agent. It does **not** use the shared helper's session, cookie or pragma headers. It follows 301/302 redirects; a response body of `1` disables tracking. Source: `bj.run()`, normal-flow lines 580–692 and original bytecode.

## URLs supplied at runtime

| Source | URL / value used |
| --- | --- |
| Login 203 fields 23 and 26 | Asset base and URL from which the website base is derived |
| Login 203 fields 13 and 3/4 | Mail URL and conditional announcement URL |
| Login 203 field 32 | Appended to website base and saved as setting 10; this build also reads field 32 as an integer earlier, an unresolved protocol inconsistency |
| Packet 1002 | Selected language-pack URL |
| Packet 719 field 3 | Room-theme image URL |
| Packet 911 field 4 | Custom menu navigation value; selected contact/group context can be appended by the client |
| Packet 918 field 1 | Open an embedded page |
| Packets 5/8 and delegated 502 | Notification/prompt links, conditional on their presentation/action fields |
| Room list 718 field 2 | Navigable room-list link |
| HTML response bodies and HTTP redirects | Further links, form actions, image/background sources and Location targets |
| Certain acknowledgment packets | Uploaded-photo or other operation-specific continuation URLs |

This is why no list of bundled strings can be the complete historical service endpoint list. The recoverable contract is the URL selection/construction rule and the renderer/transport behavior; absent historical payloads remain external inputs.

## Binary operations behind the gateway

These are API operations addressed by opcode, **not URL paths**. Their complete tag/type/request/response schemas are in [protocol.md](protocol.md#outgoing-operation-catalog), with [outgoing JSON](data/protocol-outgoing.json) and [incoming bytecode inventory](data/protocol-incoming.json).

| Family | Client-to-server opcodes |
| --- | --- |
| Connection, help, prompts and preferences | 2 test/heartbeat; 7 help; 9 prompt reply; 10 device information; 11 preference reply; 17 security challenge answer |
| Registration, login and session | 101 registration start; 103 registration submit; 200 login; 202 challenge response; 204 activation; 205 activation mobile/resend; 300 logout; 904 forgot details |
| External IM | 206 connect; 302 disconnect; 907 cached settings update/removal; 926 icons |
| Contacts and contact groups | 400 full sync; 405 add; 406 delete; 407 edit; 408 add group; 409 delete group; 410 rename group; 411 move contact; 417 block; 419 incremental sync |
| Chat and photos | 500 send message/photo; 504 upload photo |
| Own profile state | 600 presence; 601 status text; 602 avatar |
| Chat rooms | 703 join; 704 leave; 706 kick; 707 participants; 709 mute; 710 unmute; 711 favorite; 712 remove category room; 713 categories; 716 category contents |
| Group chat | 751 create from private chat; 752 invite; 753 leave; 754 participants |
| Calls and invitations | 800 callback; 900 invite friend |
| Content, activity and language | 913 emoticon; 921 activity subscription; 922 page/image request; 925 language selection |

There are **53 network-operation entries**, plus the **3 local HTTP jobs** described above, in the 56-entry outgoing catalog. The incoming dispatcher has 55 top-level cases, including server pushes and resource packets. These counts are not expected to match. For example, live messages arrive as 500, contact updates as 402/404/406/421/423, activity as 422, configuration as 1000, and custom menus as 911.

## Installation and non-network interfaces

| Value | Responsibility |
| --- | --- |
| `http://www.mig33.com/midlet/member/c_info.php?awtrack=wap` | Manifest/JAD `MIDlet-Install-Notify`; device application manager notification, not app network code |
| `http://www.mig33.com/wap2/v4_60/24x24/mig33v46.jar` | JAD `MIDlet-Jar-URL`; binary distribution, not runtime chat API |
| `mig33:method(argument)` | Client-side command dispatch from pages; [20 recognized commands](protocol.md#web-to-native-mig33-commands) |
| `tel:` | Handset dialing handoff |
| `file:///`, `capture://image`, `capture://video` | Local filesystem and camera interfaces |
| `local:` and bundled PNG paths | Resource/image lookup |
| `http://`, `https://`, `socket://` alone | Scheme strings used in parsing/construction; not complete endpoints |

The installation values are verified from the [original manifest](../artifacts/extracted/META-INF/MANIFEST.MF) and [JAD](../artifacts/original/mig33-4.62.jad). Locale text mentioning `www.mig33.com` or `wap.mig33.com` is not, by itself, evidence of another request.

## Reconstruction contract and limits

The agreed implementation starts with a stable **v4.62** core. A later **Settings → Experience → Classic v3** profile will reuse this API and shared account/chat data while changing presentation and supported interaction flows. V3 action mappings must be explicit where its original binary operations differ. The setting is planned, not implemented in this analysis package.

This catalog documents the client-visible API. It does not recover the backend's authorization policy, complete form submission schemas, moderation rules, billing rules or historical page contents. Those require a separately specified replacement backend or recorded responses. Preserve original protocol anomalies in a compatibility layer where exact behavior is required, and identify deliberate changes explicitly.

To regenerate and validate the evidence catalog from the workspace:

```sh
python3 tools/applets/scripts/build_v4_endpoints.py
python3 tools/applets/scripts/validate_reference.py
```
