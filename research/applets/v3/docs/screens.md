# mig33 3.08 screen and interaction specification

This specification covers the recovered Java MIDP 2.0 client, identified as version 3.08. It inventories **31 recognized primary screen IDs, 29 overlay IDs, native text editors, and the camera form**. These are implementation states, not 60 independent full-screen pages: a menu or dialog overlays a primary screen, and several primary IDs share one renderer.

Static forms, strings, controls, branches and key handling are recoverable from the JAR. **The historical contents of server-rendered profiles, account, scrapbook, community, mail, help pages, registration challenges and downloaded emoticons are not in this JAR.** Their client containers and interactions are specified here. Exact historical server pages require captured response fixtures. The [46-image original-bytecode gallery](../artifacts/screenshots/240x320/index.html) supplies a concrete 240×320 emulator reference for the captured client states; [capture methodology](capture.md) identifies synthetic data and runtime limits. Native editors, camera preview, fonts and softkey placement require a specified handset/runtime; the Java application does not define their pixels.

## Evidence and notation

References such as `C:515` identify the preserved CFR output at the indicated line; `VC` is the structurally cleaner Vineflower controller. Both outputs are retained, because obfuscated identifiers and some inferred Java types make either unsuitable as directly compilable original source. The bytecode is the final reference when they disagree.

| Alias | File | Responsibility |
|---|---|---|
| C | [CFR controller](../artifacts/decompiled/com/projectgoth/Roshan/c.java) | State, commands, validation, menus, action dispatch |
| VC | [Vineflower controller](../artifacts/vineflower/com/projectgoth/Roshan/c.java) | Structured action dispatch; same original class |
| T | [Form renderer](../artifacts/decompiled/t.java) | Ordered controls and form drawing |
| A | [Network/controller bridge](../artifacts/decompiled/com/projectgoth/Roshan/a.java) | Response-created views and transitions |
| E | [Overlay renderer](../artifacts/decompiled/e.java) | Menus, text dialogs, progress |
| R | [Contact list](../artifacts/vineflower/r.java) | Roster sorting, collapse, row rendering |
| V | [Conversation](../artifacts/vineflower/v.java) | Transcript rows, wrapping, scroll/history |
| Tabs | [Tab manager](../artifacts/vineflower/ab.java) | Contacts/conversation tabs and unread activity |
| I | [Emoticons](../artifacts/vineflower/i.java) | Grid, cache, downloaded image data |
| Files | [Storage picker](../artifacts/vineflower/c.java) | File roots, directories and image preview |
| Camera | [Camera](../artifacts/vineflower/z.java) | Native capture and send preview |
| Page | [Embedded page renderer](../artifacts/vineflower/k.java) | Server page format and controls |

`al` is the primary state, `bm` is the overlay ID (`0` means none), `ah` is the current action/context, `y` selects IM setup (`85` MSN, `86` Yahoo!, `87` Google Talk, `88` AIM), and `U` tracks the IM setup/sign-in sequence. Do not merge their namespaces: for example primary `8` is Contact Request, overlay `8` is Add to Group, and action `8` sends mail.

The exact constructor statements, including dynamic values, separators and source locations, are preserved in [form rows](data/screens-form-rows.json). The [state catalog](data/screens-catalog.json) enumerates every recognized state. The [source inventory](data/screens-source-inventory.json) retains all 87 display-constructor occurrences, 137 literal menu-row statements, literal text-editor call sites, and every string-literal occurrence across the decompiled classes. It deliberately also contains internal/protocol strings; it is a searchable evidence index, not an assertion that each string is a visible label.

## Shared interaction rules

Forms are custom Canvas content, not native MIDP Forms. They contain label rows, editable value rows, short vertical spacers, buttons, and checkboxes. Selecting a value and pressing `Edit` opens a separate native `TextBox`. Pressing `Continue` in that editor copies the value back; `Back` discards the edit. Buttons use `Continue`; checkboxes use `Edit` to toggle. The focused control therefore changes the first command dynamically. Native commands are installed through MIDP; their left/right placement and whether the platform puts them under a menu are handset dependent. (`C:817–989`, `C:4020–4100`, `C:4178`.)

All general field editors permit 320 characters. SMS body permits 160; chat and mail messages permit 320. Constraints are `ANY=0`, `NUMERIC=2`, `PHONENUMBER=3`, `PASSWORD=65536` as seen in call sites. Port and authentication code use numeric; phone values use phone-number constraints. Login and IM passwords are masked in the Canvas and native editor; **registration password is a plain visible text field and uses constraint 0**. Native phone fields strip a trailing NUL on acceptance. Username lowercasing depends on context: login/IM editors with title `Username` lowercase; add-contact `mig33 Username` lowercases; registration's `Username` editor runs action 50 and is not lowercased by that handler. (`C:1827`, `C:3247–3367`, `C:4012–4100`; `T:65–125`.)

Forms generally focus the first eligible control. Login initially focuses its first button, then its credential-prefill routine moves focus to the first missing username/password. Resend Code initially focuses Continue. Static text/help forms focus their first text row for scrolling. Keep the line wrapping and spacer rules in T; a paragraph is converted into word-wrapped label rows using the actual device font. Blank help lines become 3-pixel spacers. Long strings in a field are ellipsized for display, while the full value remains stored. (`T:319–355`, `T:461–486`, `T:690–820`.)

A single validation pass visits form rows in order. The first required empty field produces overlay 21 titled `Error`, with `<field label> is required.`. Requiredness is usually non-null/nonzero length, without trimming. Registration additionally requires its terms checkbox. There is no client-enforced username length or password-strength policy beyond the 320-character editor limit; server validation may add rules and messages. Do not invent such rules as recovered behavior. (`C:3615–3850`.)

## Primary screens

### Loading and first-run sequence

Before normal UI initialization the Canvas draws the background image at screen center and `Loading...` at center. Existing saved credentials select state 15; no saved credentials select state 49. Initial state 15 is constructed with title `mig33`; subsequent rebuilding uses `Login`. This title difference is present in code. (`C:497–512`, `C:611–625`, `C:737`.)

| State | Title and ordered content | Commands and outcome |
|---|---|---|
| 49 | `mig33`; paragraph `Starting mig33 for the first time. We are about to perform a few tests to ensure that the phone is configured correctly` | Continue → state 42 titled mig33. Exit closes MIDlet. |
| 42 | `mig33` initially; `Connection Type` when opened from login. Paragraph `Please select your connection type below. If you are unsure, try TCP first.`; buttons TCP, spacer, HTTP, spacer, Manual | TCP/HTTP show Connection / Connecting... progress and send connection test. Manual → 13. Exit in initial mode; Back → Login in settings mode. |
| 13 | `Connection Type`; Server field initialized from gateway setting; spacer; Port initialized from port setting; spacer; extra spacer; Connect | Server and Port required; Port numeric editor. Connect → Ping / Testing Connection... and test. Back → 42. |
| 43 | `Welcome to mig33`; `Connection test using TCP is successful. Please select:` or HTTP equivalent; Login; text Or; Registration; spacer | Login → 15. Registration fetches challenge and enters 30. Back is installed but has no explicit state43 branch in the recovered handler; do not assume it returns Login. |

After a successful connection test, an explicitly opened Connection Type flow returns to Login; initial connection setup goes to 43. The selected transport and manual server/port are persisted. Tests may fall back to HTTP; the post-login sequence can display `Your current connection is connected through HTTP instead of TCP.`. (`T:55–62,128–144,288–291`; `C:1750–1791,3382–3399,3487–3509`; `A:849–862`.)

### Login — state 15

Title `mig33` on the initial saved-credential path, otherwise `Login`. Ordered rows:

1. Username label, text field, spacer.
2. Password label, masked field.
3. Save Password checkbox; Sign in as Invisible checkbox.
4. Spacer, Login button; spacer, Register button; spacer, Forgot Details button; spacer, Connection Type button; spacer.
5. `Connect to:` paragraph.
6. MSN, Yahoo!, Google Talk, AIM checkboxes in that order.
7. Two spacers; Show offline contacts checkbox.

Save Password defaults on; invisible/offline-contacts defaults off. The IM defaults reflect stored account-configuration flags (off on fresh state). Saved username is prefilled; saved password is filled only when Save Password is enabled. Login requires both. Submission persists all checkbox values and shows Login / Connecting..., followed by network progress including `Downloading contact list...`. Register fetches the captcha/challenge before building Registration. Forgot Details → 14. Connection Type → 42 with title Connection Type. Exit closes the MIDlet. (`T:64–91`; `C:738–741,3569–3624,3789–3868`; `VC:943–985`.)

The post-login sequence is transport-fallback notice → account authentication if required → announcement if supplied → setup for each selected but unconfigured IM → sequential IM connections → contact list. Announcements may be a text overlay or an embedded page, according to a server flag. Each stage can therefore change the visible screen without another user menu choice. (`C:1750–1810`.)

### Registration — state 30

Title `Registration`. Ordered controls: Username field; Password field; Mobile Number label; `(Inc. Country Code)` label; phone field prefilled from challenge field 1; spacer; `Enter Security Code` label; security-code field with challenge image directly below it; spacer; checked checkbox `I agree to the terms and conditions (see www.mig33.com)`; spacer; Submit; Help?. Username, Password, Mobile Number and Enter Security Code are required. Unchecking terms yields `You must agree to the terms and conditions.`. The security-code editor is ANY with 320 limit, not numeric-only. Submit sends the returned challenge identifier with the entered code, then shows Registration / Registering User.... Success stores credentials and shows server text under `Success`, then returns Login. Back → Login. Help fetches state 36. (`T:92–126`; `C:3625–3850,3869–3873`; `A:1268–1275,1383–1389`.)

The exact captcha bitmap is dynamic; its height controls the row height. A recreation needs a sample challenge response to reproduce any particular image, phone prefix, and visual state.

### Forgot Details — state 14

Title `Forgot Details`. Paragraph `Your username and password will be sent by SMS to your mobile phone.`; spacer; Mobile Number field prefilled with account number `z`; Continue. Number required; phone constraint. Continue shows Forgot Details / Requesting Info.... A successful request returns Login. Back → Login. (`T:207–213`; `C:3864–3868`; `A:1534–1537`.)

### Authenticate Account — state 41

Title `Authenticate Account`. Paragraph `Enter your authentication code to make calls, send SMS and receive your free credit!`; spacer; Enter Code; spacer; Resend Code; spacer; Remind me later. No ordinary Back command. Enter Code opens numeric 320-character editor; nonempty acceptance sends opcode 204 with Authenticate Account / Processing.... Success marks account authenticated and shows server text in Success overlay. Resend Code → 40. Remind me later advances the post-login sequence, or returns contacts if opened by a restricted call/SMS action. (`T:161–171`; `C:3573–3604`; `A:1514–1518`.)

### Resend Code — state 40

Title `Resend Code`. Paragraph `This is your mobile number. Please change it if it is incorrect.`; spacer; Mobile Number field prefilled `z`; Continue. Continue is initially focused. Submission requires phone, shows Resend Code / Processing..., sends opcode 205, updates `z` on success and returns 41. Back → 41. (`T:214–220`; `C:3854–3860`; `A:1519–1523`.)

### Contact list — state 10

The primary Canvas has a tab strip and a scrollable contact list on the background/wallpaper. The contact list is the special tab at index `-1`; each open private/group/room chat occupies another tab. Left/right move between tabs; unread tabs and offscreen-tab indicators blink. Switching to a conversation selects state 6. Roster row kinds are group (`q=0`), contact (`q=1`), Chat Rooms (`q=3`) and optional Mail (`q=4`). Mail label is `Mail (<unread count>)`; it is only inserted when a mail URL was supplied. Contact names, identities, status, groups, order and counts are server data. Empty groups cannot expand; collapsed groups hide their contacts. (`Tabs:18–155,283–398`; `R:36–43,663–880`; `n.java:24–46`.)

Menu varies by selected row: contact → overlay 1; group → 2; Chat Rooms → 10; Mail → 39. FIRE on a roster row opens and immediately selects that menu's default action: start chat/send mail, expand/collapse a group, open rooms or open mail. The main command is Menu. Contact-request notifications interrupt contacts or chat and switch to state 8; file-received prompts can similarly overlay them. A server message `P` can become an Info overlay or embedded page when not editing a native TextBox. (`C:618–670,2100–2161,4375–4402`.)

### Add Contact — state 3

Title `Add Contact`; paragraph `Type of Contact:`; buttons mig33, Mobile Number, MSN, Yahoo!, Google Talk, AIM, separated by spacers. Back → roster with prior context menu. mig33 → 4. Mobile Number opens a phone-number editor, then Display Name editor, then group selection if multiple groups exist. Third-party choices open their named native editor (`MSN Username`, `Yahoo! ID`, `Gtalk Username`, `AIM Username`) and submit an add-contact request. Mobile Number and Display Name must be nonempty; Display Name is explicitly trimmed for its emptiness check. Third-party handles are trimmed before submission. (`T:145–159`; `C:3245–3280,3323–3345,3544–3572`.)

### Add mig33 contact — state 4

Title `Add mig33 contact`; mig33 Username label/field/spacer; unchecked `Share my mobile number` checkbox; spacer; Continue. Username required. The field may be prefilled from a private chat or group-member selection. Continue uses the only group automatically, otherwise overlay 8 titled Add to Group. Finish submits username/group/share flag. Back → Add Contact when on roster, or conversation when entered from chat. (`T:273–279`; `C:2568–2579,3941–3956,3991–4005`.)

### Contact Details — state 9

Title `Contact Details`; editable fields in order: Display Name, mig33 Username, Mobile Number, MSN Username, Yahoo! ID, Gtalk Username, AIM Username; each label+value+spacer; Update. Values come from selected contact. Only Display Name is required; all identity/phone fields may be blank in this screen. Update sends all values with contact and group IDs and shows Update Details / Processing.... Back restores the originating roster or conversation menu. (See [exact form rows](data/screens-form-rows.json), case 9; `C:3941–3950,4113–4137`.)

### Contact Request — state 8

Title `Contact Request`; `<username> has added you to his/her contact list.`; spacer; Allow; spacer; Reject; spacer; View User; spacer; Skip; spacer; Skip All; spacer; checked `Add <username> to my contact list`; unchecked `Share my mobile number with <username>`. This screen uses form object `k`, not `j`. Allow respects the two checkboxes and routes through group selection when needed, then sends accept. Reject sends reject. View User opens the supplied requester profile URL. Skip removes only the first queued request; Skip All clears the queue; both return contacts. No ordinary Back command is installed. (`T:221–243`; `C:619–626,3510–3532,4151–4175`.)

### Conversation — state 6

Title area is the tab strip; the active tab shows contact/room identity. Transcript supports text, inline downloaded emoticons, per-span colors, status notices, group/room messages and photo-status messages. Conversation types are private `1`, group `2`, room `3`. The history cap is **50 rendered lines**, not 50 messages: long wrapped messages consume multiple lines and old lines are removed. (`V:265–477`.)

Commands: Menu → overlay 29; Write opens native `Write Message:` editor, maximum 320 ANY, with Send, Back, Insert Emoticon, and for group/room only Insert Username. Send trims whitespace; an empty result leaves the editor open. Non-slash text is appended locally before the network result; slash-leading text is sent without local echo. Transport/destination varies by contact, room ID or group chat ID. Accepted send returns to Canvas conversation. Back discards the current editor and returns conversation. (`C:3084–3104,3281–3322`.)

Incoming photos are not decoded as inline transcript images by this path: a `File Received` overlay asks Ok/No; Ok opens the supplied link through embedded page mode, No dismisses that queued item. Photo send adds `Sending Photo in progress...`; acknowledgement inserts `Photo Sent!`. Presence notifications append `<name> is online`, `is busy`, `is away` or `is offline`, and may play the MIDP info sound when Sound is on. (`C:651–662,2010–2034`; `A:1370–1380`; `Tabs:35–78`.)

### Emoticon picker — state 7

Reached from Insert Emoticon in message editor; white grid with downloaded icons, two black selection rectangles and selected description/token below. The item list and icon bitmaps come from the server/cache; missing items display dotted placeholders and trigger opcode 913 fetching. Cell step is `emoticonSize + 3`; columns `width / step - 2`; rows `height / step - 2`. Up/down/left/right move selection, 3 and 9 page. Insert appends the selected token plus a trailing space to the text buffer, then returns native editor. Close returns without insertion. No hardcoded emoji pack should be substituted as historical evidence. (`I:12–46,369–470`; `C:1990–2007,4302–4348,4437–4445`.)

### Create Room — state 11

Title `Create Room`; New room name field; Create; Help?. Field required. Create shows Create Room / Processing..., sends opcode 705, and joins/opens returned room on success. Help fetches state 12. Back → Chat Rooms overlay. (`T:280–285`; `C:3540–3547,3932–3938`; `A:1497–1512`.)

### Search Room — state 34

Title `Search Room`; Room Name field; Search; Help?. Nonempty search required. Search clears prior room result object and requests page 1; results appear in overlay 7 with title `Chat Rooms (<page>/<pageCount>)`, occupancy labels, Next and/or Previous. Help fetches state 35. Back → default Chat Rooms overlay. (`T:267–272`; `C:3923–3932`; `A:949–988`.)

### Invite Friend — state 23

Title `Invite Friend`; paragraph `Invite your friends to join mig33. Get bonus credit when they activate their account.`; spacer; Mobile Number field; Your name field initialized to saved mig33 username; Invite Friend. Both fields required. Submission shows Invite Friend / Processing...; success uses server response text under Success. Back → contact list context menu. (`T:173–180`; `C:3891–3895`; `A:1525–1528`.)

### Make Call — state 17

Title `Make Call`; Destination Number label and history-enabled phone field; spacer; Origin Number label and history-enabled phone field; spacer. When server flag `ax` allows call-through, include Call-through checkbox, spacer. Then Call now, spacer; when `ax`, Compare Rates, spacer; Help?. Destination defaults to the selected contact's phone if available; Origin defaults to own account number. If destination is prefilled the focus advances to Call now; otherwise focus stays destination. Both numbers required. Right on a number field opens Recent Numbers (overlay 41) or shows `You have no recently used numbers.`. (`T:181–206`; `C:4138–4150,4324–4348`.)

Call now persists number history and requests rate/call information; a confirmation text may come from the server. Continue on callback confirmation requests the callback. A call-through response may show confirmation, then invokes `platformRequest("tel:" + number)` and exits only if the platform requires it. Compare Rates requests mode 0. Help topic is 34 when call-through is offered, otherwise 4. An unauthenticated account selecting Make Call sees Info / `You need to activate your account first.` with Continue → state 41. (`C:426–435,2910–2941,3157–3168,3875–3890`; `A:1252–1266,1414–1428`.)

### External IM setup — state 22

Titles are `MSN Setup`, `Yahoo! Setup`, `Gtalk Setup`, `AIM Setup`. Username label; for MSN only paragraph `(e.g. user@mail.com)`; Username field; spacer; Password label; masked field; spacer; Save. Both required. Existing account settings can prefill credentials. Save updates respective fields in account packet 906, then resumes initial IM setup or returns Other IM menu. On initial sign-in, Back disables that service's auto-connect checkbox and advances to the next required service; in manual settings it returns the service menu. (`T:244–266`; `C:2791–2828,2335–2370,3896–3922`; `A:864–891`.)

### Help text — states 12, 18, 35 and 36

| ID | Title | Requested topic | Close destination |
|---|---|---|---|
| 12 | Create Room | 2 | 11 |
| 18 | Make Call | 4 or 34 | prior form `j.i`, normally 17 |
| 35 | Search Room | 1 | 34 |
| 36 | Registration Help? | 5 | 30 |

Body is response field 1, split at newlines into wrapped paragraphs and spacer rows. Commands are a blank first command plus Close; arrows scroll. No help prose is embedded in the artifact. (`T:300–318`; `C:2035–2062`; `A:526–545`.)

### About mig33 — state 1

Title `About mig33`; `Ver: ` plus `Main.d` (manifest version); `Web: www.mig33.com`; `Wap: wap.mig33.com`; spacer; `Copyright 2006 Project Goth Inc.`. Blank first command and Close; Close → contacts with Settings overlay. (`T:292–299`; `C:2063–2067`.)

### Embedded page modes — states 44, 46, 48

These use the same custom page renderer. Actual page-opening calls in the recovered controller use state **48**; 44 and 46 are recognized paint/command branches but no literal assignment entering either was found. Treat them as compatibility branches until a runtime trace shows an entry path, not three proven separate products.

Named entry points are My Scrapbook, My Profile, View Profile, My Account, mig33 Community, Open Mail, Send Photo → From Scrapbook, announcement URLs, contact-request View User, incoming file URLs, and server-injected menu URLs. They request/cache a server URL then fetch page data through opcode 998. Loading status is `Loading...`, `Loading Images <received>/<total>`, `Loading Complete`, or `Error loading`. Menu contains Continue when the focused element is actionable, then Close Window. History enables Back only after load advances beyond stage 1 and history contains more than one entry. A select-choice overlay changes commands to Select/Back. (`C:446–455,669–706,2154–2161,2643–2660,3400–3440`; Page.)

Page actions returned by renderer include kind 0/1 (navigate/submit with URL and payload), kind 2 (send selected scrapbook photo into conversation), kind 4 (invoke external native browser and terminate client), and upload-photo mode (From Camera/From Storage). Browser-launch failure yields `Unable to invoke native browser: <message>`. Closing pages returns current roster/chat; closing an announcement resumes post-login sequence. Server-driven page titles, body text, forms, images and custom menu items cannot be enumerated from this binary alone. The renderer grammar and protocol specification are required alongside this screen map. (`C:3384–3443`.)

### Camera and send preview — state 45 plus native Camera form

Selecting From Camera creates native `Form("Camera")`, with a VideoControl item and Capture/Close commands. Capture changes to custom Canvas state 45 while a thread captures the frame. Canvas title `Send Photo`, white background, centered `Processing...`; completion shows scaled image centered and enables Continue/Close. Landscape images use target width `3 * screenWidth / 5`, proportional height; portrait/square uses target height `3 * screenHeight / 5`, proportional width. Scaling is nearest-neighbor. Continue sends bytes to chat or uploads to a server page depending on context; Close returns caller and releases player. (`C:2663–2685,3444–3486`; Camera.)

Camera resolution comes from Settings; native capture defaults differ for device strings. Nokia5300/6233/6085/6131 use `capture://image`; other devices use `capture://video`. Some Sony Ericsson paths set preview size to 120×160 or W300c 116×108. Native camera permission dialogs belong to the device runtime. Errors include `You need to allow the application to access the mobile phone Camera. ` and `Ooops. Unfortunately we detected that your mobile phone does not support taking pictures from the application. <exception>`. (`Camera:24–139`; `C:2673–2681`.)

### Storage picker and confirmation — state 47

Entry starts at device file roots with title `Select File`. Root names shorter than 5 characters are admitted. The selected row is vertically centered and uses the large font; surrounding rows use standard font. Folder icon 32, image icon 33, mp3 icon 34, generic file 35, parent `...` icon 36. Image recognition is case-insensitive `.gif`, `.jpeg`, `.jpg`; PNG is not included in this classifier. Directories navigate; image choice enters substate 1 titled Confirm; other file types have no select action here. (`Files:40–76,112–142,143–220,290–328`.)

File limit is **153600 bytes**; larger files report `File size too large.`. Preview loads raw image and scrolls horizontally/vertically in 20-pixel increments using arrow keys; it is not fit-to-screen. Processing or errors appear centered when no bytes are ready. List commands Continue/Close; preview Continue/Back only when bytes exist, otherwise blank/Back. Back from preview resets to file listing; Close returns embedded page if one exists, otherwise current roster/chat. Continue on ready preview sends/uploads as for camera. Capability failure: `Ooops. Unfortunately your Phone does not support taking pictures from the storage.`. Security failure: `MIDlet needs to be signed to be able to retrieve photos from the storage.`. (`Files:77–111,143–218,220–365`; `C:895–914,2338–2398,3444–3486`.)

## Menus and overlays

A `—` below means an actual separator row whose label is null, not a visible em dash. Keep separators, ordering and conditional items; they affect keyboard navigation and scroll height. Overlay type 2 rows are disabled. Other row types in these menus are selectable. Menu position, size, border/shadow and scrolling are defined by E and the rendering specification. Numeric row IDs and primary screen IDs are independent.

### Context menus 1, 2, 10 and 39

| Overlay | Ordered entries |
|---|---|
| 1 — selected contact, title Menu | Chat; —; Make Call; Send SMS; Invite Friend; My Scrapbook; Set Presence; —; View Profile; View Details; mig33 Community; Add Contact; Add Group; Delete Contact; Move to Group; —; My Profile; My Account; Other IM; Settings; Exit |
| 2 — selected group, title Menu | Collapse; —; Make Call; Send SMS; Invite Friend; My Scrapbook; Set Presence; —; mig33 Community; Add Contact; Add Group; Rename Group; Delete Group; —; My Profile; My Account; Other IM; Settings; Exit |
| 10 — Chat Rooms row, title Menu | Open Chat Rooms; —; Make Call; Send SMS; Invite Friend; My Scrapbook; Set Presence; —; mig33 Community; Add Contact; Add Group; —; My Profile; My Account; Other IM; Settings; Exit |
| 39 — Mail row, title Menu | Open Mail; —; Make Call; Send SMS; Add Contact; Invite Friend; —; My Account; My Scrapbook; mig33 Community; Other IM; —; Add Group; Set Presence; Settings; Exit |

Group first row becomes Expand when collapsed; empty group's Expand is disabled. Contact Chat becomes Send Mail when offline, a mig33 identity exists and mail URL is available; otherwise Chat is disabled for offline contact. Network availability and each contact's available identities then determine the chat-transport choices. View Profile does nothing if selected contact lacks mig33 username. Server-defined menu items may be inserted/replaced; their URL receives selected `cid` and `pres` or `gid` when appropriate. (`C:515–604,2108–2154,2720–2769`; `A` server menu response handling.)

Main-menu actions not covered by the form sections:

| Action | Immediate result | Completion/validation |
|---|---|---|
| Add Group | Native Group Name editor, max 320 ANY | Nonempty → Add Group / Processing... and group create |
| Rename Group | Native New Group Name editor, max 320 ANY, initially empty | Nonempty → Update / Processing... and group rename |
| Delete Contact | Overlay 22, title Delete Contact | `You are about to delete "<display name>" from your contact list.` plus, if any external IM handle exists, `This will also delete this contact from your desktop IM.` with **no inserted space** between sentences. Continue sends delete; Back cancels. |
| Delete Group | Empty group and at least two total groups required | Otherwise `Group must be empty and must not contain hidden contacts.` or `You must have at least one group.` under Warning. Confirmation: `You are about to delete the group "<group>" from your contact list.`. |
| Move to Group | Overlay 8 Add to Group populated by all groups | Finish moves selected contact |
| Set Presence | Overlay 9 Presence | Online (icon8/value1), Away (icon9/value4), Busy (icon10/value3), Appear Offline (icon11/value99); selection → Presence / Changing Presence |
| Exit | Logout / Logging Out... | Sends logout; resulting disconnect/error path may show server message and return Login. Pre-login Exit directly destroys MIDlet. |

Source: `C:2853–2906,2970–3020,3128–3150,3991–4008`.

### Chat transport selector — overlay 25

Title `Chat`. Candidate order: mig33 chat (type1), MSN chat (4), Yahoo! chat (6), Gtalk chat (8), AIM chat (5). Include an item only if account transport is connected, contact has that transport identity, and its presence is not 99; mig33 checks identity/presence directly. If more than one candidate, show selector; otherwise bypass it and open chat. These conversation transport numbers differ from account-login service codes. (`C:2737–2773`.)

### Chat Rooms — overlay 7

Default list: favorite room rows with room icon 40 and right text `(<current>/<capacity>)`; separator; More Rooms; Search; Create Room; then server-defined room-menu links. With no favorite rooms it still contains separator and the three controls. A room name sends join 703 after recording it in favorites; More Rooms retrieves listings; Search → 34; Create Room → 11. A paginated result changes title to `Chat Rooms (<page>/<totalPages>)`, lists room rows then separator if pagination is needed, Next when page < total, Previous when page > 1. Optional server info can overlay the results. `No chat rooms found.` is an Error returning to this menu. Back from default menu returns Chat Rooms context menu; Back from paginated results rebuilds default favorites. Server menu links append favorites as f1/f2/… query parameters. (`C:1833–1856,2420–2457`; `A:949–988`.)

### Conversation menu — overlay 29

Title `Menu`. Exact branches:

| Conversation kind | Ordered entries |
|---|---|
| Room (3) | View User's Profile; Private Chat; Kick User; Mute User; Leave Room; server-supplied room actions |
| Group chat (2) | View User's Profile; Add to Contact List; Invite User; Send Photo; Close Chat |
| Private (1) | View User's Profile; Add to Contact List **only if contact ID is -1**; Invite User and Block User **only if mig33 username exists**; Send Photo; Close Chat; server-supplied transport-specific actions |

View User's Profile gets a user picker for room/group, or fetches profile for private mig33 user. Private Chat, Kick User and Mute User fetch the room-user list first. Invite User shows online mig33 contacts excluding self; empty result → Warning / `None of your contacts are online.`. Leave Room requests 704. Group Close Chat requests 753; private Close Chat removes the local tab. Block User confirmation is `Are you sure you would like to block <username> ?` with the space before `?`. From Scrapbook is available under Send Photo. Server-added actions receive `roomName=<name>`, `gcid=<groupChatId>`, or `imType=<transport>&cid=<contactId>`. (`C:1869–1928,2464–2617`.)

### User pickers — overlays 30, 31, 32, 33, 35 and 42

| ID | Title | Row population / Select behavior |
|---|---|---|
| 30 | Kick User | Server room user list; select requests kick opcode 706, progress Kick User / Request Kick... |
| 31 | Private Chat | Insert `Enter Username` then separator before server room users. Select a row opens private chat. Enter Username opens 320 ANY editor and accepts only a lowercased name matching a listed user. |
| 32 | Invite User | Online mig33 roster contacts, excluding self; select sends invite with current conversation/user/group ID |
| 33 | View User's Profile | Room/group user rows; select requests profile |
| 33 with action 44 | Emote Target | Same list; selection trims current message then appends space plus selected username and returns editor |
| 35 | Add to Contact List | Group-chat user rows; select opens state4 prefilled. If list size ≤1, Error `The chat room is empty.` |
| 42 | Mute User | User rows; right annotation `(Un-mute)` for muted entries, select opcode710 for unmute else709 for mute |

User rows are initially given icon argument 73, which E interprets through its icon logic; admin names gain right annotation `(Admin)`. In the response handler the admin annotation can replace another right annotation on that row, so reproduce the actual update order. Back normally restores Conversation Menu; when opened for Insert Username it returns message editor and action7. (`A:755–826`; `C:2601–2645,2234–2253`.)

### Group chooser — overlay 8

Title `Add to Group`; every group name in current group-vector order, with its group ID carried as row data. Commands Finish and Back. Destination depends on current primary screen: add mobile contact, move existing contact, add mig33 contact, or accept contact request. If only one group exists, several add flows bypass this overlay and submit with that group's ID. Back restores original overlay ID or base view. (`C:1858–1867,2208–2213,3991–4017`.)

### External IM menus — overlays 27 and 24

Overlay 27 title `IMs`: MSN, Yahoo!, Google Talk, AIM (selector values85–88). Selecting one opens overlay24 with its service title. Entries: Sign In, Sign Out, separator, Add/Edit IM, Remove IM. Remove IM disabled when that service has no stored setup. Sign In and Sign Out send service requests with Connect/Disconnect progress; Add/Edit IM loads settings if absent then opens state22. Remove IM confirms `Removing <service> IM will remove all <service> contacts as well as your login details for that messenger.` before clearing the service's fields. Back from service menu goes to IMs; Back from IMs goes to roster context menu. (`C:1700–1733,2783–2828,3042–3051,3168–3207`.)

### Settings — overlay 3

Title `Settings`. Initial ordered rows:

| Row | Value / action |
|---|---|
| Save Password | ON/OFF; toggles persistent key1 |
| Sound | ON/OFF; toggles key2 |
| Blocked Users | Fetch list → overlay34 |
| Current Conn. | Disabled; HTTP or TCP; refreshed when Settings opens |
| Emoticons | ON/OFF; toggles key11 and global render switch |
| Clear Cache | Progress Clear Cache / Processing...; async clears non-preserved RMS stores |
| Camera Settings | Disabled without camera support; opens calibration or resolution list |
| Camera Retest | Disabled without camera support; resets calibration and starts tests |
| About mig33 | State1 |

Fresh defaults: Save Password ON; Sound ON; Emoticons ON; camera resolution Camera Default; invisible OFF; show offline contacts OFF; call-through mode defaults2 (enabled when capability exists); icon size12. Initial setup menu construction hardcodes Save Password display ON, but login submission/toggling updates it. Cache clearing preserves the whitelist of account/configuration/device-calibration stores in I, including saved login (`SignIn`) and Background; the exact list is `I:108–165`. It removes other stores such as cached emoticons and potentially recent/favorite data, according to that whitelist. (`C:590–604,3007–3083`; `VC:940–985`; I.)

Blocked Users (overlay34) is a semicolon-separated server list. Commands Unblock/Back; Unblock shows `Unblocking <name>` and submits unblock. Empty list → Error / `Your block list is empty.`; Back returns Settings. (`A:829–846`; `C:2164–2168,2248–2253`.)

### Camera Settings — overlays 36 and 40

Overlay36 title Camera Settings. Resolution order: Camera Default, 120x160, 240x320, 288x352, 480x640, 160x120, 320x240, 352x288, 640x480. Current setting right annotation `*`; unsupported tested resolutions disabled. Selection persists its row index and returns Settings. Calibration overlay40 has same title and text `Please wait.. <percentage>%`, starting at10%, with blank/Back commands. Back cancels testing and returns Settings. Retest resets calibration flag so tests run again. The application worker probes eight explicit resolutions and updates support flags. (`C:1735–1748,3070–3083,4577–4636`.)

### Photo source — overlay 28

Title `Send Photo` when opened from conversation; From Camera, From Storage, From Scrapbook. Camera/storage rows disabled when respective capabilities absent. Title `Upload Photos` when requested by an embedded page; only From Camera and From Storage. Back returns conversation menu or embedded page according to caller. (`C:2597–2605,2662–2699,3437–3443`.)

### Recent Numbers — overlay 41

Title `Recent Numbers`; saved strings for the focused Destination Number or Origin Number field. Select replaces that field and closes overlay; Left also dismisses it. First use shows Info-style Recent Numbers / `You have no recently used numbers.`. Histories are recorded when Call now is submitted. (`C:2642–2647,4324–4348`.)

### Embedded page menu — overlay 38

Title `Menu`; Continue only when renderer reports actionable focus, then Close Window. Close Chat is also recognized by the handler, although the literal menu constructor adds Close Window. Continue invokes page action handling; Close Window clears page renderer and returns contacts/chat. Choice controls within the page use the renderer's `V` selection state rather than a separate global overlay ID. (`C:2154–2161,2648–2662`.)

### Information, warnings, confirmation and progress

| Overlay | Typical title | Commands | Behavior |
|---|---|---|---|
| 20 | Info / Success / Announcement / Recent Numbers | Continue only | Uses saved `ah` to return/advance; text often from server |
| 21 | Error / Warning / service name | Blank + Back | Returns saved parent overlay; parent -1 clears chats and returns Login |
| 22 | Delete Contact / Delete Group / Block User / Remove IM / Confirm / Info | Continue + Back | Continue dispatches pending action; Back cancels |
| 23 | Operation title | Blank + Back | Animated progress, current message, saved parent; Back cancels network work and stops animation |
| 37 | File Received | Ok + No | Body from incoming file field4; Ok loads URL field5, No drops queued file |
| 40 | Camera Settings | Blank + Back | Calibration status as above |

Progress title/text pairs include Login / Connecting..., Login / Downloading contact list..., Chat Rooms / Retrieving..., Chat Rooms / Joining chat room..., Presence / Changing Presence, Invite User / Inviting user..., View Profile / Retrieving Info..., Make Call / Retrieving Info..., Authenticate Account / Processing..., Registration / Registering User..., SMS / Sending SMS..., Logout / Logging Out..., Send Photo / Retrieving Info..., Upload Photos / Processing.... The full literals index preserves exact spelling/capitalization for all call sites. Several errors are server-supplied; preserve unknown text as data rather than replacing it with guessed historical wording. (`C:1679–1698,817–875`; A.)

Server disconnect shows Error / `Unexpected disconnection. Please reconnect.<server suffix>` and routes Back to Login. Server logout messages can use the same Error style. Generic request errors use the currently active progress parent as their return target. Thus dismissal needs the saved parent and action context, not simply a hardcoded return-to-contacts rule. (`A:1233–1250,1549–1558`.)

## Native editors as separate screens

These are MIDP TextBoxes with native layout, not inline replicas of the Canvas field. Except chat compose, helper `C:4178` installs Continue and Back. The exact native character-counter, keypad input mode, cursor, masking glyph, scrolling and menu implementation depend on the selected runtime.

| Editor | Maximum | Constraints | Accepted outcome |
|---|---:|---|---|
| Username, mig33 Username, Display Name, Your name, IM handles, room names, Server | 320 | ANY | Copy field or invoke context action |
| Login/IM Password | 320 | PASSWORD | Copy masked value |
| Registration Password, Enter Security Code | 320 | ANY | Copy visible value |
| Port, Enter Code | 320 | NUMERIC | Copy port / submit authentication |
| Mobile Number, Destination Number, Origin Number | 320 | PHONENUMBER | Copy or advance SMS/contact flow |
| Group Name, New Group Name | 320 | ANY | Create/rename group if nonempty |
| SMS Message: | 160 | ANY | Store message then ask Mobile Number |
| Write Message: — chat | 320 | ANY | Send; Back; Insert Emoticon; Insert Username for rooms/groups |
| Write Message: — Send Mail | 320 | ANY | Submit mail after nonempty check |
| Enter Username — room Private Chat | 320 | ANY | Must match a user in fetched list |
| Embedded-page field | Defined by Page call site | Defined by Page | Updates selected page form field |

SMS flow requires account activation, opens SMS Message: then Mobile Number prefilled from selected contact when present. Back from number returns the prior body. Number acceptance sends transport2 message; success shows Sending SMS... / `Your SMS has been sent.`. There is no local SMS-body nonempty test before proceeding to number. (`C:2979–3005,3247–3265`; `A:1368–1374`.)

## Key mapping and navigation

The controller uses `Canvas.getGameAction`, so physical numeric/D-pad key mappings vary by runtime. UP=1, LEFT=2, RIGHT=5, DOWN=6, FIRE=8 in this implementation. There is no `pointerPressed`/`pointerDragged` implementation in the recovered client.

| Context | Up/down | Left/right | FIRE / other keys |
|---|---|---|---|
| Overlay active | Move selection or scroll text; progress ignores list movement | Left dismisses Recent Numbers | Executes current default command; disabled rows ignored |
| Contact list | Move roster cursor | Switch tabs | FIRE opens and selects context menu default; raw `3` pages up (first to viewport start), `9` pages down (first to viewport end), `1` jumps to first row, `7` to last visible row |
| Conversation | Scroll transcript | Switch tabs | FIRE invokes Write when no overlay |
| Form | Move to next/previous editable field, button or checkbox, scrolling viewport | Generally no action; right on call number opens history | Edit/Continue according to focused row |
| Help/About/first-run text | Scroll rows | No special action | Close/Continue as installed |
| Embedded page (44/48) | Move/scroll once loading stage >1 | Horizontal/element movement | Activate item/menu; raw3/9 page; state46 shares paint/commands but is omitted from several direction branches |
| Emoticon picker | Row movement | Column movement | Insert; raw3/9 page |
| File list | Move selected file | No directory jump | Continue opens directory/image |
| File image preview | Pan by20px | Pan by20px | Continue sends if data ready |

Raw numeric keys are ASCII49/51/55/57 for1/3/7/9 and only run in the non-game-action/default branch. Reproduce the runtime's game-action mapping before applying these shortcuts, otherwise a key may be handled twice or differently. (`C:4193–4446`.)

## Reconstruction coverage and remaining fixtures

All primary switch cases, overlay constructors, native displayable types and literal labels found in this client are indexed. Screen IDs not enumerated here are not evidence of missing screens; the numbering is sparse. Likewise recognized modes44/46 are not evidence of a visible entry point.

For a fidelity baseline, capture and fix: handset Canvas dimensions; font glyphs and metrics; native editor/softkey skin; persisted settings and login state; contact/group roster; transport-presence combinations; room membership/occupancy/admin/mute lists; announcements; account-activation flag; mail count/URL; server menu definitions; registration challenge; embedded-page bodies and images; emoticon metadata and bitmaps; file/camera capability results. Without these, the same original JAR legitimately produces different screens.

The recovered client can define exact static application behavior. A complete historical pixel-for-pixel recreation additionally needs the missing runtime and server fixtures. The renderer source and extracted original images should be used directly as evidence; generic modern chat UI assumptions would change this application.
