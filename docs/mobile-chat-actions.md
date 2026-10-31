# Mobile chat actions and original-protocol compatibility

The Expo app uses the original v4.62 HTTP tunnel and TCP protocol for all three appearances: Modern, Classic v4 and Classic v3. Appearance changes do not select a different API. The generated [wire contract](../api/asyncapi.yaml) remains authoritative for packet fields; the [client](../apps/mobile/src/mig33/client.ts), [session controller](../apps/mobile/src/ui/session/controller.ts) and native page adapter add orchestration over it.

This checklist separates what the original clients establish from local server policy. The original JARs establish menu labels, request builders, response parsing and native/page handoffs. They do not recover historical server-rendered page contents, billing, moderation thresholds, or external provider behavior. Original-source anchors are retained in the [v4 screen specification](../research/applets/v4/docs/screens.md), [v3 screen specification](../research/applets/v3/docs/screens.md), [v4 protocol](../research/applets/v4/docs/protocol.md) and [endpoint construction inventory](../research/applets/v4/docs/data/endpoint-route-notes.json).

## Original conversation menus

These are source observations, not a requirement to reproduce inaccessible keypad interactions on a touch device. Common actions remain reachable through themed native controls; Modern can also offer direct composer and participant controls.

| Original client/context | Ordered built-in actions before server extensions |
| --- | --- |
| v4 private mig33 chat | Write; conditional Group Menu; Add as friend; User Profile; Lookout; Give a Gift; Add to Chat; Share Photo; Call User; SMS User; Transfer Credit; Block & Mute User |
| v4 private group | Write; conditional Group Menu; User Profiles; Add to Chat; Private Chat; Add as friend; Share Photo; Call User; Transfer Credit; Give a Gift; Block & Mute User; Report Abuse |
| v4 room | Write; conditional Group Menu; User Profiles; Private Chat; Add as friend; Give a Gift; Call User; SMS User; Transfer Credit; Kick User; Block & Mute User; Report Abuse; Room Info; Save to Favorites |
| v4 common tail | Active Chats; Search Rooms for a room; Store; Main Menu; Close Chat |
| v3 room | View User's Profile; Private Chat; Kick User; Mute User; Leave Room; server room actions |
| v3 private group | View User's Profile; Add to Contact List; Invite User; Send Photo; Close Chat |
| v3 private chat | View User's Profile; Add to Contact List when contact ID is−1; Invite User and Block User when a mig33 identity exists; Send Photo; Close Chat; server transport-specific actions |

Evidence: v4 `aj.U()` bytecode offsets31–1022, resolved against the bundled [English packet](../research/applets/v4/docs/data/screens-en-US.json); v3 `c` menu/action branches recorded in its [conversation specification](../research/applets/v3/docs/screens.md#conversation-menus). External-service variants have different availability branches. In particular, **v4's room menu does not contain Add to Chat or Share Photo**. Extra Modern shortcuts are touch adaptations, not recovered v4 room rows.

The v4 **Block & Mute User** label is misleading: the normal branch sends personal block417, without a room. A participant already marked muted can instead trigger room unmute710. The retained709 builder exists, but no normal v4 visible-menu route to its mute confirmation was found. V3 has an explicit Mute User flow which selects709 or710 from the participant's muted marker. Expo keeps personal blocking and moderator room muting as separate actions. See [room moderation evidence](rooms.md#moderation) and the original [screen action notes](../research/applets/v4/docs/screens.md).

## Core conversation checklist

All operations in this table have existing Go handlers. Packet numbers describe the retained protocol, not new endpoints.

| Action | Original contract / handoff | Permission and state semantics | Mobile implementation and verification |
| --- | --- | --- | --- |
| Room categories, list, refresh |713→714*/715;716→701*/717 |701 may arrive before request completion; category and request context are retained. Footer is server-authored. | Client/session room orchestration; real HTTP/TCP and native rooms acceptance. |
| Join room |703→701/ACK, followed by500 welcome | Record membership from the correlated join701 before delivering a same-batch welcome. Canonical room name comes from the server. | Native Modern/v3 flow, original-JAR interoperability and room integration tests. |
| Linked Community Group |Joined701 tag8:I |Positive community ID enables Group Menu. Group pages append this value as `cid`; it is distinct from a private group-chat ID. Rejoin refreshes or clears it when a link changes. | Go joined-room event/codec and client/session `communityId`; regression tests cover link, unlink and list isolation. |
| Room message / action |500, conversation kind3, target4; `/me` accepted by Go | Sender/target of incoming room500 are distinct; room membership and mute are checked by Go. | Per-room drafts/transcripts; both transport directions tested. |
| Participant list |707→708 |708 has no room field: retain request context. Fields2/3/4 contain participants/moderators/muted users. Unsolicited708 must not update an arbitrary room. | Per-room participant state; profile/private/action selection. |
| Leave room |704→ACK | Suppress queued room messages after leave. Other rooms, private chats and session survive. | Confirmation, acknowledged return to catalog, repeated joins and two-room isolation. |
| Favorites |711/712→701 or ACK | A bookmark is not membership. Duplicate favorite can acknowledge without701. | Add/remove in catalog and room actions; transport/native tests. |
| Kick / regular-user vote |706 with room1 and target2 | Same opcode: moderator kick is immediate; regular user initiates/joins a vote. Caller/target must be present; self and other moderators are protected. | Client/session action; Go permission/vote tests and real transport integration. Native expanded-action flow exercises both. |
| Moderator mute/unmute |709/710 | Room-specific persistent sending restriction; reading, private chat and other rooms remain available. Require current membership and moderator rights. | Explicit moderator action using708 role/mute markers; server remains authority. |
| Personal block |417 username1, mode2 | V4 builder always writes2; its boolean does not enable unblocking. Hide blocked sender only for blocker. | Confirmation; session action; HTTP/TCP integration. |
| Unblock |`settings/block_list` form over922/923 | Authenticated CSRF-protected page; no invented v4 unblock opcode. | Native page form and live integration. V3 appearance still uses this v4 contract. |
| Private messages |500 kind1 | Private tab closing is local; server privacy checks still apply. | Existing two-way original-JAR and native tests retained. |
| Create private group / Add to Chat |751→750 plus messages | Two initial peers;750 identifies the private conversation. This is distinct from persistent Community Groups. | Typed client/session methods and mixed-transport integration. |
| Invite to private group |752 | Membership and participant limit checked by Go;750/500 synchronize recipients. There is no recovered accept/decline workflow to invent. | Native invite form, session participant updates and integration. |
| Private-group members / leave |754→755;753→ACK | Correlate755 with requested group; leaving removes only that group. | Native participant/leave controls and session tests. |
| Add friend/contact |405 and original roster sync | Existing contacts/domain rules; roster changes arrive through the original contact stream. | Native form from private/participant actions; client integration. |
| Body color |924 palette; outgoing500 tag13 | Only advertised colors are offered. One RGB value applies to a message body; arbitrary HTML/BBCode is not native chat formatting. | Session palette and composer selection; wire/unit tests. |
| Emoticons |916 catalog;913→914 resource |914 types1/2 static,3 framed animation,4 framed image/audio. Photo triggers share the resource path. | Format-gated image/frame parsing and catalog picker. Audio/provider limits remain explicit. |
| Local transcript operations |No new network operation | Clear, draft, active-tab selection and private Close Chat affect local state. | Session/UI operations preserve other conversations and appearance changes. |

The original generic server5 notice does **not** identify a room. Expo displays its text without parsing English to infer which room was kicked or silently removing membership. A later participant/message request supplies authoritative success or failure. Original timeout/error responses remain visible; no success is synthesized from the user's tap.

Vote thresholds, vote expiry, room capacities, ownership limits and durable moderation audit are replacement-server policies. They are documented separately in [rooms](rooms.md), [room discovery](room-discovery.md) and [room completion](room-completion.md), with their historical evidence and limits.

## Linked pages and media checklist

The [page adapter](../apps/mobile/src/mig33/legacy-pages.ts) sends original922 and receives923 through the active authenticated transport. The [native page screen](../apps/mobile/src/ui/pages/LegacyPageScreen.tsx) renders server-authored text, links, images and form controls. It does not add a JSON API or execute JavaScript. Existing Go handlers and generated page markup are the source of form actions, hidden CSRF/binding/nonce fields, select options and button labels.

| Original page/action | Retained path or packet | Implemented surface / limitation |
| --- | --- | --- |
| Profile, friend profile, edit profile |`profile/home`, `profile/edit` | Native links, typed controls, profile formatting subset; `mig33:privateChat` and `inviteFriend` return to native actions. Profile privacy remains server-enforced. |
| Find friends |`profile/search` | Server-backed search form; distinct from local roster filtering. |
| Search/Create Room |`chatroom/search`, `chatroom/create` | Search form and two-stage name/description form; creation does not automatically join. Returned native link joins the room. |
| Room Info and moderation |`chatroom/setup&roomName=…` | Description/occupancy/roles; server-generated mute/unmute forms only for allowed members. |
| Room Settings |Same setup route with local `manage=1` selector | Owner language, description, keywords, capacity, kicking policy, moderators, ownership transfer and explicit deletion confirmation. Joined moderators can ban/unban ordinary users. These reconstructed forms are not claimed to match historical HTML. |
| Report room/group abuse |`report/report_abuse&type=4&subject=Chatroom` or `Group%20Chat`, `subject_id=…` | Membership checked again on GET/POST; optional target, category/details, CSRF and context binding retained. Reports do not automatically kick anyone. |
| Give a Gift / Store |`store/home`, original `ty=1&username=…` gift context | Catalog links, detail forms, ratings and purchases; credits/catalog placeholders are explicitly local. Native Back does not repeat a purchase. |
| Account/history/transfer/recharge |`account/home`, `get_history`, `transfer_credit`, `recharge_credit` | Native structured forms preserve hidden recipient/binding/nonce values. Local test credits only; no real payment or settlement provider. |
| Community Groups |`group/home`, `list_groups_home`, `list_my_groups`, `members`, `view_forum`, `list_chatrooms`, `add_to_group` | Public/private groups, membership/approval, forum and linked-room pages remain server-authored. Separate from751 private group chat. |
| Photo upload |504 with PNG/JPEG bytes | Native photo selection feeds original upload; ACK supplies scrapbook URL. Go limits1MiB/2048×2048/100 photos per owner. No `/api/photos` dependency in Expo. |
| Share photo |500 content2 upload or content5 own scrapbook reference | Retain destination conversation locally while browsing `photo/home&f=sendfromphoto`. `sendScrapbookPhoto(ID)` resumes that destination. |
| Scrapbook / photo wall |`photo/home`, `photos/wall`;922 purpose2 for image data | Authorized thumbnails; tap opens full photo through the same authenticated transport; own avatar/delete forms and upload native link. |
| Display picture |`avatar/home` / scrapbook avatar form | Existing server avatar update and423 contact push; photo selection/upload remains a native handoff. |
| Lookout, Buzz, Blog, Location |`lookout/home`, `buzz/home`, `profile/user_blog_posts`, `location/checkin` | Existing service forms and navigation remain usable through922. Original context query parameters are preserved. |
| Mail, SMS, Call, Invite |`email/compose`, `sms/home`, `call/home`; native invite workflow where applicable | Local Go queue/mock providers remain labeled. No real external email, carrier SMS or telephone call is implied. |
| Games, leaderboard, help, IM settings, migWorld |Existing selectors in the Go route registry | Render existing pages and native links; external IM bridges are not connected. No invented MSN/Yahoo network. |
| WAP formatted chat |`/member2/t.php` | Existing page forms can render their supported formatting; native500 still carries plain text plus at most one RGB value. |

The authoritative implemented route registry is [page_routes.go](../internal/adapters/legacyweb/page_routes.go), with service routes in [service_pages.go](../internal/adapters/legacyweb/service_pages.go). Page availability and policy are covered in [service pages](service-pages.md), [community/commerce](community-commerce.md), [reports](reports.md) and [native media](native-services.md). The mobile page renderer can follow these linked pages without a separate screen implementation for every backend form.

### Form compatibility and page lifecycle

- Page requests use922 purpose1; image requests use purpose2. The current session supplies authentication; credentials are not copied into page URLs. Login203 tag26 supplies the site base.
- Ordinary links and images are restricted to the connected server's HTTP(S) origin. `mig33:` commands are parsed as case-sensitive literal data and delegated to native code. No script execution, arbitrary browser event handlers or external cookie forwarding occurs.
- Hidden server fields are immutable in the form model. GET queries use normal encoding. POST encoding mirrors each existing Go native decoder: fixed values first, one unrestricted text tail last, with only spaces replaced by `%20` in that tail. This preserves literal `&`, `+`, `=`, other `%` and Unicode. A literally typed `%20` cannot roundtrip through the historical decoder; the app reports that case instead of silently changing it. See [form serialization evidence](text-and-page-formatting.md#forms-and-serialization).
- Forms remain staged when the server needs multiple unrestricted values. No client guesses CSRF tokens, nonces, recipient bindings or room permissions. Rejected server pages retain their explanatory body.
- Back uses cached page state and form drafts; it never automatically resubmits a POST. Reload performs GET. A later navigation wins over an older response, and logout/connection changes cancel display of late replies and clear page cookies/images.
- Native page controls cover text/password/textarea/select/checkbox/radio/hidden/submit. File input is a native photo handoff, not multipart emulation. Arbitrary historic HTML/custom tags and third-party browser behavior are outside this constrained renderer.
- V3's998 is a local HTTP job, not a gateway operation. Do not send998 to the Go gateway to implement the v3 appearance; both Expo transports use the existing v4 page contract.

## Server menus and native page commands

Server911 carries `1:I category`, `2:I insertion index`, `3:T label`, `4:T navigation value`. The second field is not an opcode. Entries are retained in received order and applied to the corresponding native menu.

| V4 category | Menu | Context appended to a server URL prefix |
| ---: | --- | --- |
|1 |Own profile |None |
|2 |Home/Friends |None |
|3 |Updates |None |
|4 |Selected contact |`cid=<contactID>&pres=<presence>` |
|5 |Selected contact group |`gid=<groupID>` |
|6 |Settings |None |
|7 |Embedded page |None |
|8 |Room listing |None |
|9 |Private chat |`imType=<service>&cid=<contactID>` |
|10 |Private group chat |`gcid=<groupID>` |
|11 |Room chat |`roomName=<room>` |

The original concatenates these suffixes without inserting an initial `?` or `&`; the server prefix must provide its separator. This differs from the built-in `%s` URL templates. V3's911 category mapping differs; the Classic v3 appearance does not reinterpret a v4 server push using that older mapping. Source: [dynamic menu protocol](../research/applets/v4/docs/protocol.md#dynamic-menu911-and-room-theme719), [dynamic route construction](../research/applets/v4/docs/data/endpoint-route-notes.json), original `aj` conversation dispatch offsets3412–3551.

The optional positive Community Group ID in a joined701 is directly traced through original `bc` offsets3071–3095/3200, `s` constructor133–136 (`s.a`), `aj.U`51–70 (Group Menu), and group-page dispatch3118–3159 plus adjacent branches (`cid=s.a`). It does not denote the private-group ID used by750. Go obtains it from the room's durable Community Group link; unlinking is reflected on the next703 response.

The complete recovered `mig33:` command set is `checkIn`, `reorderHomeTab`, `auth`, `sendScrapbookPhoto`, `uploadPhotoToScrapbook`, `invokeNativeBrowser`, `setDisplayPicture…` prefix, `inviteToMig33`, `inviteFriend`, `privateChat`, `call`, `sms`, `updateStatus`, `joinGroupChatroom`, `joinChatroom`, `closeTab`, `focusTab`, `close`, `newBrowser`, and `imMenu`. Arguments remain literal rather than JavaScript or generally URL-decoded strings. `joinGroupChatroom` takes the first comma-separated segment as the room; `focusTab` uses0Friends/1Updates/2Rooms/3last chat. Unknown commands have no original action. [Original dispatch and arguments](../research/applets/v4/docs/protocol.md#web-to-native-mig33-commands).

Current Go pages emit `privateChat`, `inviteFriend`, `joinChatroom`, `reorderHomeTab`, `uploadPhotoToScrapbook` and `sendScrapbookPhoto`; those native handoffs are essential acceptance cases. Registration/activation and external platform/provider commands are adjacent features, not evidence that a chat action has succeeded.

For touch navigation, `focusTab(1)` currently opens the existing Lookout page as the Updates surface. It is an adaptation of the original native Updates tab, not a newly recovered screen. `reorderHomeTab(0)` orders migWorld/Friends/Rooms, `1` orders Friends/Rooms/migWorld, and `2` moves Rooms first; the mobile handler changes the actual tab order. The original persisted these three tab types in RMS22–24 and focused the first tab (`k.a(int)` offsets7–62/220–320; bundled `bj.a=[7,1,8]`, `bj.b=[1,8,7]`). Server-side Tab Order forms and mobile local preference lifetime remain separate concerns.

## Unread tabs and selected conversation

Both original clients flash the **whole inactive unread tab**, not a dot. Their shared phase switches every750ms, making a full on/off cycle1500ms. The original rendering is a binary color change, not an interpolated opacity animation.

| Original | On phase | Off phase | Paint evidence |
| --- | --- | --- | --- |
| v4.62 | Full tab gradient `#efcf06 → #fe8300`, top highlight `#f9ffb3` | Inactive gradient `#0f95d3 → #05bad3` | `k.a(IIZ,Graphics)` offsets149–315; theme fields11/12/13 and8/9 |
| v3.08 | Full slanted tab gradient `#f5c258 → #ff8000` | Inactive gradient `#e4eefc → #c0d8f9` | `ab.a(IIZ,Graphics)` offsets218–318 |

V4 `aj.run()` offsets137–180 toggles `ai`, requests tab repaint with `a(1)`, and sleeps750ms. When animated chat content is also active, offsets243–333 use three250ms ticks before toggling the same phase and repainting. V3 `c.run()` offsets164–201 toggles `ag`, repaints the top tab rectangle and sleeps750ms. Offscreen unread tabs also flash the corresponding original scroll arrow; the touch app instead exposes tabs through horizontal scrolling. Source recovery and geometry are documented in [v4 rendering](../research/applets/v4/docs/rendering.md) and [v3 rendering](../research/applets/v3/docs/rendering.md), with palette evidence in [v4 theme fields](../research/applets/v4/docs/theme-fields.md) and [v3 bytecode literals](../research/applets/v3/docs/data/theme-bytecode-evidence.json).

Unread belongs to a tab, not a message timer. V4 `k.a(IZ)` offsets24–63 marks a previously clear tab and increments the global pending count; another message in an already unread tab does not restart the phase. Offsets66–102 clear it and decrement the count. `aj.d()/e()/f()` maintain that count and pending flag; the flashing loop stops when no pending tabs remain. V3 uses `v.l()/a(Z)` and `c.b()/c()/d()` for the equivalent state. Selecting a tab clears it immediately: v4 `k.g(I)` offsets72–78; v3 `ab.c(I)` offsets153–181. Original close bookkeeping is not copied: the mobile loop derives pending state from the tabs that still exist.

The original decides whether a message is unread from the **selected tab**, even when an editor or menu covers its transcript. V4's message append branch compares `k.b` to the destination tab at offsets918–941; presence notices use the same rule at `k.a(II)` offsets239–250. Opening an embedded page creates/selects type7 (`aj.a(String,String,ZZ,bl)` offsets74–117, then `k.b(ILs;Lbl;Z)` offsets65–83), so the conversation behind it becomes unread-eligible. Write keeps the conversation selected: v4 `aj.as()` offsets0–41 shows `ce` without changing `k.b`; v3 `commandAction` offsets9197–9349 similarly shows Write Message without selecting another tab.

The mobile session therefore retains active conversation identity and drafts separately from `conversationVisible`. Pages and Appearance set it false; returning to a conversation clears only that conversation's unread state. Treating Appearance as unread-eligible is a touch adaptation because the original has no equivalent theme screen. Write, participant pickers and conversation menus retain logical selection. Private, room and group tests cover hidden incoming messages, returning, independent unread state, retained drafts, and750 conversion while hidden. Reduced Motion retains a static full orange tab instead of flashing.

## Verification and remaining boundaries

- [Room tests](../apps/mobile/tests/rooms.test.ts) and [real room integration](../apps/mobile/tests/room-integration.test.ts) cover request context, early701/welcome ordering, favorites, participants, independent rooms and HTTP/TCP interoperability.
- [Original-JAR interoperability](../apps/mobile/tests/interop.test.ts) uses unchanged Java builders/parsers for private messages and room join/messages/participants/leave in both transport pairings.
- [Expanded-action integration](../apps/mobile/tests/chat-actions-integration.test.ts) covers real Go room rights, kicks/votes/mutes, private groups, media and personal blocks. Unit tests separately exercise stale-session protection and packet parsing.
- [Page unit tests](../apps/mobile/tests/legacy-pages.test.ts) cover parsing, form controls, encoding, immutable CSRF fields, origin/native-link handling, navigation races, image purpose and explanatory error pages.
- [Page integration](../apps/mobile/tests/legacy-pages-integration.test.ts) passes on isolated HTTP and TCP servers: profile read/edit with literal punctuation/Unicode; native private link; search/two-stage room creation; description/moderator/ban/unban/ownership forms and forbidden access; join-gated report; block-list unblock; local gift purchase; authenticated thumbnail/full-image loading; linked community/account/photo/help pages.
- [Native harness](../tools/mobile/README.md) captures actual iOS UI. The basic room run passed both HTTP Modern and TCP Classic v3, with live theme changes, participants, private navigation, two-room drafts, favorites and leave. Expanded acceptance also passes Modern HTTP and Classic v3 TCP room forms, moderator mute/unmute/kick, ordinary vote kicks, personal block/unblock, incoming photos, composer choices and private-group creation/invitation/replies/leave. Modern native-control acceptance passes light mode and dark accessibility-large text, including native menu/alert/sheet interaction, retained browse state, the login disclosure, refresh icons, unread tapping, composer clearance and stable history reading. Reduce Transparency and Increase Contrast are tested through real system settings. Software-keyboard geometry and physical devices remain unverified; these runs do not establish that every historical page has been visually checked.

Historical live page layouts, provider networks, production product downloads, real payments, account activation delivery and historical server moderation policy remain unrecovered or intentionally disconnected. Server-generated local forms and policies are functional reconstructions, clearly distinguished from verified original client contracts.
