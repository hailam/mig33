# mig33 4.62 screens, controls, and navigation

This specification covers the recovered **4.62 Java ME client**. It contains 17 populated built-in form branches, five tabbed view families, camera/file flows, native text editors, menus, and server-driven pages. A screen is not identified by the integer `M` alone: selected tab type, action `aO`, selected row, menu/dialog stacks, transport, connection status, and server data also affect the result.

Exact text and ordered constructor branches are supplied as data alongside this document. Historical account pages, profiles, room lists, custom menus, captcha images, photos, avatars, and downloadable emoticons were delivered by the server. Their original contents are not bundled in this JAR; reconstruction must distinguish original client chrome from fixture content.

## Evidence and how to use it

- [All 419 bundled en-US strings](data/screens-en-US.json), preserving punctuation, capitalization, placeholders, and Unicode ellipses. The resource is a packet with magic 2, opcode 1004, transaction 0; its fields are UTF-8 strings keyed by localization ID. A label present in this resource does **not** establish that a screen or menu entry is reachable.
- [17 built-in form branches](data/screens-forms.json): original source plus localized source, retaining conditional rows, initialization, and control order. Spacers are real rows and must be preserved.
- [Menu/form builder methods](data/screens-builders.json): 46 structured methods with raw and localized source. This includes conditional rows, icons, submenus, row action values, disabled flags, shortcuts, and server-extension insertion.
- [Complete annotated command handler](data/screens-actions-localized.txt): source line numbers and exact English values for each `y.e(N)` reference. This is the comprehensive branch-level companion to the action tables below.
- [Localization call-site inventory](data/screens-source-localized.txt), [screen catalog](data/screens-catalog.json), and [literal UI/state/command inventory](data/screens-source-inventory.json) make the evidence searchable.
- [39 original-bytecode captures](../artifacts/screenshots/240x320/index.html) illustrate static forms, scroll states and menus; [capture.md](capture.md) documents the explicit fixtures and handset limitations.
- The layout, colors, fonts, and resource atlas are covered in [rendering.md](rendering.md) and [theme-fields.md](theme-fields.md). Use those with this document; native Java ME `TextBox` and camera `Form` are handset-rendered.

Source notation below: `Y` = [original Vineflower y.java](../artifacts/vineflower/y.java), `A` = [original Vineflower aj.java](../artifacts/vineflower/aj.java), `N` = [normal-flow aj.java](../artifacts/normal-flow/aj.java), `K` = [k.java](../artifacts/vineflower/k.java). Line numbers are part of the checked-in artifacts. The original bytecode is in [aj.txt](../artifacts/bytecode/aj.txt).

`commandAction` defeated three ordinary decompiler passes. The `N` derivative removes exception-handler tables **only for analysis**, leaving instruction bytes unchanged; it recovers normal branches but not correct catch/finally boundaries. It must never replace the original JAR or be executed. Original Vineflower, bytecode, and unchanged instructions remain authoritative for error handling. Where an obfuscated class merges unrelated members with the same Java name, use method descriptors and field types rather than assuming the decompiled Java can compile.

## State and input model

`aj.M` is the primary state. `aj.aO` is the action context, and can select a different meaning for the same command. `aH` is a last-in-first-out menu stack; `aI` is a dialog stack; `aF` and `aG` are progress/reconnection overlays. `aj.aj` is the current built-in form, `ak` is the help/first-run form. Back first handles native editors, progress cancellation, open dropdowns, dialogs, or menus, then the primary state. A reconstruction with one flat route per number loses observable behavior. See N1581, N1777, N5856.

Tabbed views use `k` and `bj` tab records. Tab type maps to screen state as follows (K108):

| Tab type | Primary state | View |
|---|---:|---|
| 1 | 10 | Friends/contact list |
| 2 | 50 | Updates |
| 3, 4, 5, 6 | 6 | Private, group, room, and external-service conversations |
| 7, 9 | 48 | Embedded page; type 9 has a restricted menu |
| 8 | 55 | Chat Rooms |

The three initial base tabs use the saved order array (`{7,1,8}` or `{1,8,7}` defaults); the Updates tab is conditional on setting 17. Opening a conversation or page creates/selects another tab. Left/right switch tabs in states 6/10/48/50/55, while grids use them for grid navigation. FIRE activates the current action: in connected conversations it opens Write, in disconnected conversations it opens the reconnect menu; an inactive room can offer Rejoin. Friends FIRE expands a group, opens contact More Options, updates own status, opens mail, or follows a notification according to row type. See N1324–1577 and K108.

Menus use explicit selection and a focus row. Disabled entries remain visible and cannot activate. Right can enter an available submenu. `3` and `9` page menu selections; `#` closes the Active Chats overlay. Numeric shortcut handling is context-sensitive (e.g. `1` My Profile, `3` migWorld, `7` Groups, `9` Updates); preserve the conditions in N1000–1320 rather than applying every shortcut globally. Menu separators are flags attached to rows, not additional selectable entries.

## Shared form and editor behavior

In the order descriptions below, **field(X)** means a label row, an editable text row, then a spacer, as generated by `Y.b(label,value)`. **Paragraph** is wrapped into label rows. A checkbox paragraph may wrap over several rows; only its first row carries the actual toggle. Chinese locale uses a separate wrap path. Buttons, links, dropdowns, phone fields, password fields, captcha fields, images, and step headings are distinct widget types.

| Type | Role in the built-in forms |
|---:|---|
| 0 | Editable text |
| 1 | Masked password |
| 2 | Label/wrapped text |
| 3 | Alternate text/history row |
| 4 | Spacer |
| 5 | Button |
| 6 | Checkbox |
| 9 | Mobile-number field |
| 10 | Link |
| 12 | Expandable choice/dropdown |
| 15 | Image |
| 16 | Captcha/security-code field |
| 17 | Step heading |

Except Login, help/first-run, and Resend Code, initial focus finds the first interactive row. Help/first-run begin at the first row; Resend Code focuses its button. Login is populated/focused by its own initializer. Group dropdowns contain `<none>`, `Create Contact Group`, then actual groups in their received order. Selecting Create Contact Group opens the native `Group Name` editor; selecting a real group closes the dropdown. See Y2726–3100 and N4965.

Native field editor `cd` uses a maximum of **320 characters** in the form paths. Constraints are Java ME `ANY=0`, `NUMERIC=2` for Port (and the separate Continue-on-Verification-Code branch), `PHONENUMBER=3` for mobile values, and `PASSWORD=65536` when the key equals `Password`. `Username` is lowercased when committed. A supplied initial value of length **greater than or equal to** the requested maximum is cleared. Continue commits to the form, Back normally discards the edit; state/action-specific Back branches can reopen the preceding editor. See N1915–2070, N3072, N5759.

The chat `ce` editor is separate: title **Write Message:**, maximum 320, ANY. It has Send, Back, Insert Emoticon, Insert Username, and optionally Change Text Color. Write clears its contents. Empty/whitespace-only Send does nothing. Nonempty Send trims the text; slash-prefixed commands are sent but do not create the normal immediate local echo. The payload and local echo preserve conversation transport, channel, color, and avatar metadata. Insert Username appends the peer directly in private chat, or fetches/selects users in groups/rooms. Emoticon insertion appends its shortcut plus a space. See N1630–1655, N3093–3134, N5097–5135, N5752, N6286, N6700.

Update Status Msg opens a 320-character ANY editor initialized to **`is `**, including the trailing space, action54. Continue sends and persists the status.

The generic SMS editor path uses **160 characters**, then a Mobile Number editor (320, phone constraint), then sends; this path remains in the command handler even though normal Call/SMS menu entries now open server pages. The retained add-phone wizard is Mobile Number → Display Name → group chooser. Do not confuse these retained handlers with a built-in form route that is always reachable.

Form submission scans rows in order. Missing required values generate **`%s is required.`** under **Error**; `null`, empty, and a lone `+` count as absent. Display Name/Your Name/Name also reject whitespace-only strings. Contact Details allows empty mig33 Username/Mobile Number, whereas add/register/login flows require their relevant fields. Password, Server, Port, security code, and selected required IM/group values have their own branches. Unchecked registration terms generate **You must agree to the terms and conditions.** These checks do not imply modern server-side requirements that are not present in this binary. See N2205–2522.

### Command/softkey state table

These are the commands installed by the original client; the handset chooses their physical placement. `A / B` means both commands are installed, not an assurance that every emulator paints A on the left. Command priorities and types are preserved in the [inventory](data/screens-source-inventory.json). N659–925.

| Situation | Installed commands |
|---|---|
| Generic form editable/checkbox row | Edit / Back |
| Generic form button/link | Continue / Back (the button itself keeps its specific label, e.g. Login or Submit) |
| Login field or button/link | Edit or Continue / Exit mig33 |
| Connection chooser button | Continue / Exit mig33 |
| Authentication41 field or button/link | Edit or Continue; no Back |
| Captcha60 field or button | Edit or Continue / Dismiss |
| Contact9 mig33 Username row | Back only |
| Group dropdown in1/30/52/57 | Select / Back |
| First launch49 | Continue / Exit mig33 |
| Registration help36 | Close only |
| Friends10; Updates50 | Menu only |
| Connected ordinary conversation | Menu / Write |
| Disconnected conversation | Menu only |
| Inactive group conversation (connected) | Close only |
| Inactive room conversation (connected) | Rejoin / Close |
| Emoticon7 (and retained61) | Insert / Close |
| Text color59 | Select / Close |
| Camera processing | Close only |
| Camera preview ready | Continue / Close |
| File root/list (`a==0`) | Continue / Close |
| File preview/loading (`a!=0`) | Continue / Back if preview image exists, otherwise Back only |
| Embedded page normal | Menu / Back if page history has more than one entry, otherwise Menu |
| Embedded page open selection | Select / Back |
| Rooms55 | Menu / Refresh when connected, Menu only offline |
| Any ordinary menu | Select / Back |
| Dialog scheme1/2/3 | Continue / Back / Continue+Back respectively |
| Dialog scheme4/default | Select / Back |
| Dialog scheme5 or9 | Yes / No |
| Dialog scheme6/7/8 | Continue+Cancel / Dismiss / Ok+Cancel respectively |
| Dialog scheme10 | Close installed, but FIRE action is Dismiss in the original branch |
| Progress except mode4 | Back only |

## Built-in forms in exact row order

### 49 — first launch; 54/13 — connection setup

**49:** the displayed form title is **mig33**. Its paragraph is: “Thanks for launching mig33 for the first time. We are about to run a few tests to ensure your phone is configured correctly.” Continue enters 54. `paint` also constructs an unused/current-form object using that paragraph as its title, but it draws `ak` with the mig33 title; use the actual draw branch. Y2983; N423, N2870.

**54, Connection Settings:** paragraph “Please select your connection type below. If you are unsure, try TCP first.” → spacer → TCP → spacer → HTTP → spacer → Manual. TCP sets transport 2; HTTP sets transport 1 and the gateway URL; each runs the connection test, with **Testing Connection...TCP** or **Testing Connection...HTTP** status construction. Manual enters 13. Back enters Login (15). Y2996; N2125–2171.

**13, Connection Settings:** field(Server, `bc.e`) → field(Port, `bc.f`) → TCP checkbox → HTTP checkbox → spacer → Connect. TCP and HTTP are mutually exclusive; selecting one clears the other. Port opens a numeric editor. Connect lowercases the host, strips `http://` for TCP, and ensures an HTTP trailing slash in the HTTP path; starts the connection test. Back enters 54 if stored item `25` exists, otherwise 49. Exact default endpoint values belong to the transport/configuration evidence, not fixed screen text. Y2821; N1940–2003, N2541–2570.

Connection failure can display “Unable to connect. Your phone may not be setup correctly. Would you like to learn how to correct this?” Yes/Ok opens the external APN-help URL. The native browser result is outside the client-rendered screen set. N1608, N1680.

### 43 — Welcome to mig33; 15 — Login; 14 — Forgot Details

**43, Welcome to mig33:** image from theme resource 122, anchor 17 → paragraph “Join mig33 and meet new friends, chat and play games and much more!” → Login → Registration → Change Language. Login enters15; Registration enters62; Change Language requests the language list. Back enters49. Y2973; N2673, N1839.

**15, Login:** field(Username) → Password label → masked password → Remember Login checkbox → Auto Login checkbox → Login As Invisible checkbox → Show Chatroom Avatars checkbox → spacer → Login → spacer → Forgot password? link → spacer → Register as a new user. link → spacer → Connection Settings link → spacer → Change Language link.

Defaults derive from record fields 8/5/7 and setting25 respectively. Remember Login controls whether saved username/password fill the fields. Selecting Auto Login in this form forces Remember Login on. Login commits these settings, chooses presence99 if invisible else1, and calls the login routine with username/password. Forgot password? enters14; registration enters62; Connection Settings enters54; Change Language fetches `http://www.mig33.com/sites/resources/language/v4.6/l`. Exit mig33 exits directly from states15/49/54; other connected routes request logout first. Y2850; N1936–1990, N2078, N2130, N2175, N2576; A17649.

**14, Forgot Details:** paragraph “Your username and password will be sent by SMS to your mobile phone.” → spacer → phone field initialized from `aj.o` → Continue. Required phone number; Continue requests account recovery (`bc.h(phone)`) and shows Retrieving... . Back returns15. Y2844; N2572.

### 62/30/41 — three-step registration and authentication; 40 — resend

**62, Registration, Step 1 of 3:** step heading → optional “Username is already in use” if suggestions were supplied → Select a username label → field(Username, candidate or empty) → optional Suggested Names label and one link per suggestion → Check Availability. The candidate is lowercased on editor commit. Check Availability requests the registration `check_username` URL; a short response proceeds to captcha/step2, while the bracket/comma-separated suggestion response rebuilds62 with alternatives. Selecting a suggestion chooses it and requests the next stage directly. On the suggestion path the constructor title is **Register as a new user.** rather than Registration. Back goes15. Y3024; N2187, N2625, N5440–5474.

**30, Registration, Step 2 of 3:** step heading → Username label + read-only chosen username + spacer (or field(Username) when no chosen value was provided) → Password label → masked password → Mobile Number* label → phone field from supplied registration data → spacer → Enter Security Code label → captcha/security-code widget with downloaded image → spacer → spacer → checked “I agree to the terms and conditions (see www.mig33.com).” → Terms and Conditions link → spacer → Submit → Help link → paragraph “*Your phone number is used to enable the free services within mig33. It will not be shared with any other party outside mig33.”

Submit validates fields and the agreement, shows Processing..., and sends selected username, password, mobile number, captcha/session token, and security code. Terms and Conditions launches the external `c=help&v=wap&a=tc` page. Help fetches server help text (state36). Back returns Login. Unlike v3, the registration password is a masked widget in this build. Y2905; N2117, N2585, N2648.

**36, fetched help:** title/content are supplied remotely; newline-separated text becomes wrapped label rows and empty lines become spacers. Draws `ak`. Close returns30. No bundled historical help body can be reconstructed. Y2938; N1739.

**41, Authenticate Account, Step 3 of 3:** step heading → spacer → field(Verification Code) → Activate → spacer → Resend Code link → spacer → Activate later link. Ordinary Edit on Verification Code falls through to the generic ANY editor, max320, action27. A separate Continue-on-the-code-row branch invokes a NUMERIC editor with action13; the normal paint path installs Edit for that row. Activate submits opcode204 with the native code value. Resend Code enters40; Activate later continues post-login without completing authentication. Preserve the original handler's use of `cd.getString()` in the Activate path rather than inventing a new form validation rule. Y2963; N2173, N2654–2668.

**40, Resend Code:** paragraph “This is your mobile number. Please change it if it is incorrect.” → spacer → phone field from `aj.o` → Continue, initially focused. Continue requests opcode205; Back goes41. Y2957; N2591.

### 57/52/1 — add contacts; 9 — Contact Details; 23 — invitation

**57, Add a Friend:** mig33 Username label → editable Username with optional supplied peer name → Add to Group label → group dropdown → unchecked Share my mobile number → spacer → Add. Required username; submit passes group ID or−1 and share-mobile boolean. Back restores the selected tab. Y3005; N2614.

**52, Add Phone Contact:** Phone Number: label → phone field → field(Name) → Add to Group label → group dropdown → spacer → Add. Required phone and name; submit passes group ID or−1. Back enters Friends10. Y2986; N2596.

**1, Add IM Friend:** IM label → dropdown of configured/connected external services whose `f==2` → Username label → editable username with optional prefill → Add to Group label → group dropdown → spacer → Add. Required username and valid service selection. Submit passes external service byte, group ID or−1, and `false` share-mobile. Back restores the tab. Service order follows Hashtable enumeration; it is not a fixed alphabetical list. Y2738; N2525.

**9, Contact Details:** field(Display Name) → field(mig33 Username) → Mobile Number paragraph → phone field → one field for each dynamically configured IM service from `aj.E()` → Update. Values are populated from the selected contact. The mig33 Username field is displayed but Edit is explicitly ignored. Display Name is required; empty mobile/mig33 identity is permitted. Update sends the contact identifiers, display/mobile/mig33 values and service ID/value arrays. Back restores the selected tab. Y2807; N2000–2005, N2537, N4040–4073.

**23, Invite 2 mig33:** this is also the destination of **Add Friends → By Phone Number** in this build, not a separate lookup form. Paragraph “Invite your friends to join mig33! Get bonus credit when they activate their account.” → Mobile Number paragraph → phone field → field(Your Name, current username) → Invite. Phone and name required; calls the invite request and shows Processing... . Back restores the selected tab. Y2898; N2582.

### 60 — Captcha Form

Server-supplied prompt paragraph → spacer → spacer → captcha/security-code widget → spacer → spacer → Send. The image and prompt are provided by response data; Send validates security code and calls `bc.d(code)`. It is distinct from registration captcha and returns to the saved state (`aT`) on its Dismiss path. Back in the general form handler returns Login. Y3015; N1582, N2622.

## Tabbed views and auxiliary screens

| State | Contents and behavior | Main evidence |
|---:|---|---|
| 10 | Friends: heterogeneous rows for self, groups, contacts, mail, notifications, and service content. Server contact identity, display name, presence, avatar, status, group ordering, hidden-offline setting, and expanded groups determine visible rows. FIRE on contact uses More Options; Menu uses the full contact menu. Self uses Update Status Msg/Set Presence. Mail and notification rows have dedicated menus. | [au.java](../artifacts/vineflower/au.java), N1503, N5050 |
| 6 | Conversation: private, group, room, and external-service variants share renderer `s`. Transcript entries carry sender, type, text, color, avatar, optional image/payload/link and local/system state. Active/disconnected room changes Write/Close/Rejoin actions. Membership/user menus are fetched on demand. | [s.java](../artifacts/vineflower/s.java), N1439, N3093, N3334 |
| 48 | Embedded remote page: title, body, form controls, images, URL/action targets, and additional menus come from the response. The client applies theme and parser rules; individual historical pages are unavailable. Page Back first closes an open selection, then follows internal history URL; Refresh reloads; Home is offered on the home tab; Close Tab or Exit depends on tab. | [bl.java](../artifacts/vineflower/bl.java), N3170, N4993 |
| 50 | Updates: server-ordered activity rows, optional avatar/content images, relative time, selected actionable link. Menu adds Click Link only when selected row is actionable; then Main Menu and Close Tab. Closing persists setting17=0 and can show the reopen hint. | [av.java](../artifacts/vineflower/av.java), N4920 |
| 55 | Chat Rooms: categories, favorite rooms, rooms and navigable server items. Category FIRE expands/collapses; room FIRE joins. Context menu depends on item type/favorite state. Closing persists setting19=0 and can show the reopen hint. | [bj.java](../artifacts/vineflower/bj.java), N4910, A15876 |
| 7 | Emoticon grid with names/shortcuts and downloaded image availability. Insert returns to the chat editor and appends shortcut plus space; Close returns without insertion. Missing artwork is fetched; up to five download names are batched in the paint path. | N443–493, N5098 |
| 59 | Change Text Color grid, populated from server/persisted `aM` integer colors. Select stores the chosen RGB value on the conversation and returns to editor; Close returns without a selection. This is a color grid, not another emoticon sheet. | A13093, N3188, N5141 |
| 61 | Retained references associated with last-used emoticons, but paint/input/insert bodies contain literal null-array operations. No working screen content can be established from this binary. The localization string “Last Used Emoticon” alone does not establish a usable feature. | N439, N1380, N5116; original bytecode offsets1885/1886 in paint and20711/20712 in commandAction |
| 45 | Native live camera Form → Capture → custom Processing.../photo preview. Share Photo preview title; send outcome depends on action context (chat share, display picture, upload). Camera snapshot is JPEG with requested resolution and fallbacks. | [d.java](../artifacts/vineflower/d.java), N3292, N2695 |
| 47 | File selection using FileConnection roots/directories; image/audio/file icons and selection; then image preview/upload flow. Back can navigate parent/reset preview; Close releases the selector and restores tab. | [ao.java](../artifacts/vineflower/ao.java), N1843, N2700, N3306 |

Updates relative times are localized: under one minute **< 1 m ago**, then integer minutes, integer hours, **Yesterday** from one to two days, then integer days. The original expression is `(int)(nowMillis - timestampMillis) / 1000`, casting milliseconds before division; preserve it only if byte-compatible behavior is required. av29–78.

Camera settings are Camera Default, 120×160, 240×320, 288×352, 480×640, 160×120, 320×240, 352×288, 640×480; unsupported entries are disabled and current selection is marked `*`. Default requests 120×160 on the special handset path, 320×240 otherwise. Capture failure can show the exact permission message, exception text or ERRxx prefixes; the original `d.run()` retains the complete fallback/catch behavior. A16444; d36–132.

Storage accepts JPEG/GIF/PNG image paths; other entries can be displayed but format handling is conditional. The **actual size check is 512000 bytes**, while localization125 says “File size too large. Maximum file size is 300 kb only.” Preserve the original mismatch when documenting/fidelity-testing it. Security refusal says “You are not allowed to access this part of the phone.” Memory failure uses localization419. ao492 and surrounding catches.

## Ordered menus

In the tables, `→` means a submenu and `[condition]` controls inclusion/disabled state. `|` marks a separator after the preceding entry where identified; inspect the exact row flags in [screens-builders.json](data/screens-builders.json) for pixel-faithful menu drawing. A menu ID is an overlay context and is often reused by different titles.

The **common main section** is, in order: **migWorld** (shortcut3), **My mig →**, **My Friends →**, **Invite 2 mig33**, **Groups** (7), **Games**, **Store**, **Leaderboard**, **Merchant Center |**, **IM →** [services exist], **Call / SMS →**, **Search New Friends**, **Settings →**, **Help**, and **Exit mig33** [full-context menus only]. Most network entries are disabled offline; Exit remains available. Server entries can be inserted at a requested index or appended, so a historical user's menu order cannot be inferred solely from the built-in list. A13417–13476.

| Context / builder / overlay | Exact built-in order and conditions |
|---|---|
| Main Menu, `ab()`,62 | Common section without Exit mig33. |
| Friends self, `K()`,51 | Update Status Msg; Set Presence→; Check In [capability]; My Profile(1); Create Contact Group \|; common full section; server `ar`. |
| Contact, `a(t,false,false)`,1 | Private Chat [online], else Send Email [mig33 identity], else disabled Private Chat; Call; SMS; Transfer Credit [mig33 identity]; View Profile [mig33 identity]; More Options→ \|; common full section; server `au`. |
| Contact More Options, `a(t,true,false)` / variant | Starts with basic contact actions when `var3` is false; then Give a Gift, Lookout, Buzz, Send Email [online], View Photos, Block & Mute User [these require mig33 identity]; Move Contact To→; Change Display Name; Delete Friend. Full common section is omitted. |
| Contact group, `a(t)`,2 | Expand or Collapse; Rename; Create Contact Group; Delete \|; common full section; server `av`. |
| Mail row, `L()`,39 | Read Email; Compose Email \|; common full section; server `as`. |
| Notifications, `H()`,62 | View Notifications \|; common full section; server `at`. |
| My Friends, `aa()`,62 | Add Friends→; Hide Offline Friends or Show Offline Friends; Updates. |
| Add Friends, `Z()`,62 | By Username; By Phone Number; Add Phone Contact; Add IM Friend [connected external service exists]. |
| My mig, `X()`,62 | My Profile(1); My Avatar; My Groups(7); My Blog; My Photos; My Presence→; Change Display Picture→; Edit Profile; Change Theme; Credits→. |
| Credits, `P()`,62 | Check Account Balance; Account History; Buy Credits; Transfer Credit. |
| Call / SMS, `Y()`,61 | Call; SMS. |
| Settings, `O()`,54 | Change Password; Tab Order; Privacy→; Login→; Images & Bandwidth→; Sound with ON/OFF suffix; Change Mobile #; Language with the locale suffix after its final hyphen (e.g. US); Clear Cache; Camera→ [camera capable]; server `aw`. |
| Login settings, `J()`,62 | Auto Login; Remember Login; Login As Invisible, each with ON/OFF suffix. Turning Remember Login off clears the saved password; menu Auto Login toggles independently. |
| Images & Bandwidth, `N()`,62 | Profile Pictures; Emoticons; Chatroom Avatars, each with Show/Hide suffix. These suffixes use different internal polarity tests; copy the branch logic, not a guessed common boolean. |
| Privacy, `I()`,62 | Profile; My Updates(9); Call by Name; Private Chat; Manage Block List. |
| Presence, `Q()`,9 | Online(icon8,value1); Away(9,4); Busy(10,3); Appear Offline(11,99), with `*` for current value. |
| Camera, `T()`,62 | Camera Settings; Camera Retest. |
| Photo source, `j(int)`,28 | From Camera [disabled if unavailable]; From Storage [disabled if unavailable]; From My Photo [omitted for Upload photo context26]. Title Share Photo, Change Display Picture(context55), or Upload photo(context26). |
| IM service, `h()`,24 | Sign in or Sign out; Edit Login \|; Auto Login ON/OFF; Remove IM [disabled if not configured]. |
| IM, `R()`,27 | Configured services in Hashtable order; Sign in on all IM if none online and entries exist, otherwise Sign out of all IM if any online; Add New IM if an unconfigured service remains. If none configured, list available services directly. |
| Private Chat transport, `b(t)`,25 | mig33 identity [online]; then connected external services for which contact is online. A single option starts immediately; multiple options show the chooser. |
| Move/add group chooser, `d(String,bool)`,8 | Optional `<none>` followed by server group names; action/context determines add versus move. |
| Language list,57 → confirmation67 | Server list of locale display names/locale IDs/download URLs. Selection asks “You are about to change your language to %s. Continue?” en-US can reset from bundled resource; other choices download. |
| Active Chats, `K.j()`,47 | Existing conversation tabs only, in tab order: private peer name; Group Chat; room name; external peer name. `*` suffix denotes pending/unread state. Selection switches to the stored tab index. |
| Group Menu, `W()`,29 | Visit Groups; Chat Rooms; Forums; Members. Present for conversations with a group/community ID. |
| Disconnected, `G()`,1 | Reconnect now [disabled while already trying] \|; Exit mig33. |

Source entry points for that table: A15767–16766 and A17633; K1254. The JSON appendix includes the full methods and all numeric action values.

**Conversation menu (`U()`,29):** always begins Write with separator. Group Menu is included when the conversation has a positive community/group ID.

- Private: for mig33 transport, Add as friend(action103), User Profile, Lookout, Give a Gift; Add to Chat→ when transport supports it; Share Photo→; for mig33 transport, Call User106, SMS User108, Transfer Credit1000, Block & Mute User; server private extensions `az`.
- Group: for mig33 transport, User Profiles102→ (shortcut `*`); supported transports add Add to Chat→, Private Chat101→, Add as friend103→; Share Photo→; for mig33 transport, Call User106→, Transfer Credit1000→, Give a Gift100→, Block & Mute User105→, Report Abuse; server group extensions `aA`.
- Room: User Profiles102→ (`*`), Private Chat101→, Add as friend103→, Give a Gift100→, Call User106→, SMS User108→, Transfer Credit1000→, Kick User104→, Block & Mute User105→, Report Abuse, Room Info, Save to Favorites; server room extensions `aB`.
- Common tail: Active Chats→ (`#`); Search Rooms [room only]; Store; Main Menu→; Close Chat.

All participant submenus depend on membership replies. Add to Chat uses the online-contact list and shows “None of your contacts are online.” when unavailable. Selecting the invite recipient sends group/room-specific requests and inserts the localized invitation system text. User action IDs100/102/106/108/1000 open gift/profile/call/SMS/transfer pages;101 opens chat;103 opens add-friend or add-IM form;104 requests kick confirmation. Action105 sends personal block417 after confirmation for an ordinary entry, or room unmute710 for an entry marked muted by708 field4. It does not ordinarily send room mute709. See N3334–3845 and the detailed distinction below.

**Chat Rooms menu (`M()`,10):** category → Expand/Collapse; room → Join Chatroom, Room Info, Remove from Favorites or Save to Favorites; other navigable item → Go. Then Active Chats [available], Search Rooms, Create Room, Main Menu→, server `ay`. Room/category payloads determine the exact label and hierarchy. A15876.

**Embedded page menu (38):** first action by selected page element: Select for types2/6, Edit for3/4/8, Click Link for5/7. Then page-supplied menu entries; then Refresh, optional Home, Main Menu→ unless tabtype9. Final action is Close Tab for a closable page or Exit mig33 for the root/home branch. The title may be changed by the page menu. Updates menu46 has optional Click Link, Main Menu→, Close Tab, server `ax`. N4993–5044; A13042.

## Actions and page routes

The following menu items open a remotely defined page in the embedded renderer. These are route templates, not bundled static screens. `y.e(path, substitution)` expands the configured base/session context; escaped parameters and page protocol are specified by the API documentation. Preserve the selected contact, conversation, group, or self substitution exactly. N3882–4812.

| Labels/action | Historical path/query template |
|---|---|
| migWorld / What's New / Hot Stuff | `/sites/index.php?c=migworld&v=midlet&a=get_mobpage` (parameter order varies) |
| My Profile / View Profile / User Profile(s) | `c=profile&v=midlet&a=home[&username=…]` |
| Edit Profile | `c=profile&a=edit&v=midlet` |
| Search New Friends | `c=profile&a=search&v=midlet` |
| My Blog | `c=profile&a=user_blog_posts&v=midlet&username=…` |
| My Avatar | `c=avatar&v=midlet&a=home` |
| My Photos / View Photos | `c=photo&v=midlet&username=…&a=home` |
| migPhotos | `c=photos&a=wall&v=midlet` |
| Store | `c=store&v=midlet&a=main&id=111` |
| Change Theme | `c=store&a=home&e=3&ty=5&v=midlet` |
| Give a Gift | `c=store&v=midlet&a=home&e=3&ty=1&username=…` |
| Games | `c=bot&a=bot_list&v=midlet` |
| Leaderboard | `c=leaderboard&v=midlet&a=home` |
| Groups | `c=group&v=midlet&a=list_groups_home&e=3` |
| My Groups | `c=group&v=midlet&a=list_my_groups&e=3` |
| Group Visit/Rooms/Forums/Members | `c=group&v=midlet&a=home/list_chatrooms/view_forum/members&cid=…` (one action per route) |
| Search Rooms / Create Room | `c=chatroom&a=search/create&v=midlet` |
| Room Info | `c=chatroom&v=midlet&a=setup&roomName=…` |
| Call / Call User | `c=call&a=home&v=midlet&f=call&number_to_call=…` |
| SMS / SMS User | `c=sms&a=home&v=midlet&f=sms&to_number=…` |
| Compose/Send Email | `c=email&a=compose&v=midlet[&to=…]` |
| Edit IM Login | `c=im&a=setup_im&im=…&v=midlet` |
| Check Account Balance / Account History / Buy Credits | `c=account&a=home/get_history/recharge_credit&v=midlet` (one action per route) |
| Transfer Credit | `c=account&v=midlet&a=transfer_credit[&recipient_username=…]` |
| Privacy Profile / My Updates / Call by Name / Private Chat / Manage Block List | `c=settings&a=profile_privacy_settings/view_update_settings/call_by_name/chat_privacy/block_list&v=midlet` (one action per route) |
| Change Password / Change Mobile # / Tab Order | `c=settings&a=change_password/change_mobile/change_tab_order&v=midlet` (one action per route; parameter order varies) |
| Merchant Center | `c=merchant&v=midlet&a=dashboard` |
| Help / Contact Us | `c=help&a=main/contact_us&v=midlet[&to=…]` |

Other local actions:

- Presence sends the selected presence value and updates the marker; group expand/collapse affects local roster visibility. Show/Hide Offline Friends persists the setting and rebuilds the roster. Settings toggles persist their respective RMS values; the Login form couples Auto Login to Remember Login, while the settings-menu toggles are separate; external-IM Auto Login is service-specific.
- Create Contact Group and Rename open Group Name/New Group Name native editors,320 ANY; Continue requests create/rename. Delete Group is permitted only for an empty group, else “Group must be empty and must not contain hidden contacts.” Delete Friend asks “You are about to delete %s from your friend list.” and conditionally adds “This will also delete this contact from your desktop IM.” Confirmation sends deletion and closes an existing matching chat. N4134–4145, N4396, N4834–4878.
- Block & Mute User asks “Are you sure you would like to block %s?” For an ordinary participant, confirmation override107 calls `bc.b(username,true)`, which sends personal block417 with1:T username and2:B2; it carries no room name. For a participant marked muted in708 field4, action105 instead sends room unmute710 with1:T room and2:T username. Kick User asks the localized kick question, then sends706. The separate retained confirmation branch `aO=33` sends room mute709, but no normal visible room-menu path to that branch was found in this4.62 build. The visible `aO=33` assignment atN4231 belongs to Move Contact To and dispatches410 through list8, not the mute confirmation. Evidence: N2952–2989, N3214–3217, N3803–3815; `bc.java` normal-flow686–690 and723–728.
- Server menu911 category11 adds URL entries to the room menu; field2 is an insertion index, not a native command ID. Selecting such an entry appends `roomName=<current room>` and opens the page. It can expose a server-provided moderation page, but does not itself invoke709, and the recovered `mig33:` command dispatcher contains no room-mute command. Evidence: `bc.java` normal-flow2906–2944; N3334–3352, N5173–5374, N6421–6440. Historical server-provided moderation pages were not recovered.
- Save/Remove Favorites changes the selected room's favorite status through the room request. Join Chatroom opens/selects the room conversation. Close Chat closes that tab; close Updates/Rooms also changes saved visibility as described above.
- Clear Cache asks the exact string245 shown in the localization file, logs out, refreshes contacts/cache, and logs back in; it is not just deleting image thumbnails. Camera Retest starts configuration progress with a percentage and preserves the original capability probes.
- Language change clears menus/dialogs and reconstructs form objects. Bundled en-US reset uses resource validation; downloaded resources must match locale and field361=`1`. A14833 and N1595.
- Photo source/submit depends on `aO`:−1 sends to active conversation;55 updates display picture;26 uploads to a remote page. Files/camera are released and prior tab restored as their branches specify. The preview title alone does not identify the eventual request. N2695–2810, N3292.

## Dialogs, transient states, and reconstruction boundaries

The original command handler has 13 exception-table entries, retained with catch-body disassembly in [screens-actions-exceptions.txt](data/screens-actions-exceptions.txt). These include camera initialization failure (localized318 plus exception message), browser-launch failure (317 plus message), silent restoration after camera close errors, restoration of SMS-editor commands after editor errors, fallback upload metadata after page-data parsing fails, a Lookout stack-trace path, and monitor-release/rethrow handlers. These branches are absent or misleading in the exception-stripped derivative and must be reinstated from original bytecode.

Generic overlays use a title, wrapped body, return state, action context, command scheme, optional icon, and sometimes a separate `n` action override. Schemes create combinations of Continue/Back, Yes/No, Ok/Cancel, Dismiss, etc. Error, Info, Warning, Confirm, Success, connection progress, authentication reminders, group invitations, and server announcements are therefore a **family of parameterized screens**, not one static dialog each. See Y constructors and N2880–2997.

Progress strings include Connecting..., Logging in..., Downloading friend list..., Processing..., Retrieving..., Retrieving Info..., Trying to reconnect..., Sending SMS..., Requesting Authentication Code..., Request Call Back..., and Configuring Camera %s… . Preserve periods versus the ellipsis glyph from the localization data. Progress Back cancels the outstanding operation (`ab.c`) and removes the progress overlay. Reconnect menu is disabled while a reconnect attempt is already running.

Queued server invitations/notifications use `aL`; accept/decline removes the head item and sends the relevant response. Dismiss can return to Login when disconnected. External URL handling may ask “Do you want to exit mig33?” before `platformRequest`; the handset/browser then supplies its own UI. These native OS screens cannot be pixel-specified by this application JAR.

For a complete implementation, use the exact constructor and action appendices as executable design evidence, original bytecode for exception semantics, and the renderer/theme specification for geometry. Supply recorded or deliberately labeled fixture replies for each dynamic screen. Do not invent historical profile fields, shop contents, room members, server menu extensions, captcha art, or service availability and present them as recovered facts.
