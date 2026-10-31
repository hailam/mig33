# mig33 mobile

An Expo development app carrying the original mig33 4.62 HTTP tunnel over HTTPS
and native packet stream over TLS. The app supports contacts, private/group/room chat, moderation,
photos, composer tools and original service pages. Modern is the default appearance; Classic v4 follows
the v4.62 Default V2 theme, and Classic v3 follows the 3.08 renderer.
The Go backend stays in the root module.

## Appearance

Choose **Appearance** on Login or in a conversation/Friends menu. Modern uses
v4's teal/navy/orange palette with spacious controls, grouped preferences,
clear friend lists and an inline message composer. Classic v4 retains the original
gradients, compact rows, command bar and separate Write Message screen.
Classic v3 has blue headers, slanted text tabs, a plain Friends list, gold focus
outlines and centered white menus. Its login uses the original Save Password,
Sign in as Invisible and Connection Type labels and form order.
The choice is saved separately from login credentials. Changing appearance
preserves the active connection, conversation history and unsent drafts.

`src/ui/appearanceTheme.ts` selects the three palettes and metrics;
`src/ui/appearance.tsx` supplies a controlled provider. Screens adapt their
presentation while sharing one session/controller and original wire protocol.
No separate client, transport or API version is introduced by a theme.

Modern follows system light/dark appearance. On supported iOS versions,
`ModernControls.ios.tsx` uses SwiftUI glass buttons, switches, pickers and SF
Symbols through `@expo/ui`. Navigation, Send and primary/secondary actions use
these controls; contextual menus, alerts and sheets are also native SwiftUI.
Search and the message editor use Expo's native `GlassView`. Messages, lists
and form content remain readable solid surfaces, following
[Apple's material hierarchy](https://developer.apple.com/design/human-interface-guidelines/materials).
Unsupported platforms and Reduce Transparency use solid fallbacks. Native
controls own their press behavior; content transitions use the native Animated
driver and respect Reduce Motion. Glass surfaces and their ancestors stay at
full opacity while the system material handles translucency.

Text inputs retain UIKit editing, autofill and UTF-16 selection semantics.
The composer has no inner border; ordinary form fields have one outline and
a restrained focus color. Friends and Rooms expose visible context menus,
preserve collapsed groups, expanded categories and the room filter across
navigation, and use a measured navigation inset for larger text sizes. Refresh
uses an accessible glass icon button beside the room filter and participant lists.
Modern Login keeps Remember Login visible; More sign-in options reveals Auto
Login and invisible sign-in, with their current states summarized when collapsed.
The transcript reserves composer space in a measured list footer and follows
new messages only while the reader remains at the bottom.

Modern retains packet-supplied message and sender colors when they meet a
4.5:1 contrast ratio against their current surface; otherwise it uses the
theme's readable text color. Classic preserves the original colors, and raw
protocol values remain unchanged.

`src/ui/v3` contains the original 3.08 renderer colors and touch metrics.
`src/ui/v4` contains source-tagged v4 values and shared native drawing primitives;
`src/ui/screens` contains the forms, Friends list, transcript and Write editor.
`src/ui/session` owns separate private/group/room drafts, unread counts and live-session state.
`src/ui/pages` renders the existing service pages with native themed controls;
`src/mig33/legacy-pages.ts` retains their original transport and form semantics.
The protocol and generated contracts remain in `src/mig33`.

The Classic v4 slice reconstructs Welcome, Login, both Connection Settings
forms, My Friends, private conversation tabs, the Write Message editor and
teal/orange menus and dialogs. Gradients, bevels, icons and the welcome logo are
native style/shape redraws. No original theme bitmaps are shipped in the app.
Colors are checked against the independently extracted Default V2 packet.
Classic logo/icon shapes and font rasterization are approximations, not
byte-identical reproductions of the original assets. Modern uses the historical
mig33 logo with [recorded provenance](assets/mig33-logo.md), system symbols and
an original procedural background grain.

Classic v3 follows the [3.08 rendering specification](../../research/applets/v3/docs/rendering.md)
and [screen inventory](../../research/applets/v3/docs/screens.md), checked against
the hash-verified original JAR and [reference captures](../../research/applets/v3/docs/capture.md).
It omits v4's self-profile block, avatar columns and group counts; contact
selection surrounds the label only. Conversation text uses v3's sender-color
categories unless a packet supplies an override. Menus retain original labels
for private, group and room contexts; More Options exposes the broader v4
actions without changing the v3 wire protocol. Appearance and Contact Groups
are added entries. The latter exposes rename/delete/move commands through touch.

Touch adaptations are explicit: 44-point interactive targets, direct tapping
an online contact to open chat (long press opens its menu), scrolling tabs,
native keyboard avoidance and safe-area handling. Original labels, form order,
selection colors, grouped contacts and inline sender/transcript layout are
preserved. Both composers trim submitted text and limit it to 320 characters.
Composer tools insert at the touch cursor or replace its selection and preserve
the surrounding draft; this improves on the original whole-string concatenation.
Modern coordinates native menu, alert and sheet presentation; Classic uses a
shared modal host. Signed-in menus say **Log out**, with explicit Log out and
Cancel confirmation actions: this ends the session rather than exiting the
application. Modern iOS retains the system alert's own button appearance.
The first slice retains 200 logical messages per
conversation, unlike the original 50 internal entries that include wrapped parts.

Remember Login uses device-only, unlocked secure storage. Auto Login implies
Remember Login and runs only when opening the app; it never replays a message
or reconnects after backgrounding. Turning Remember Login off removes the saved
credentials immediately. Login As Invisible uses the original presence 99 field.
Connection Settings probes the selected secure server with original opcode 2/3.
Remembered logins now use version2: old HTTP/TCP records are deleted with a
configuration notice and never automatically replayed. Choose HTTPS or TLS and
log in again to save the new endpoint. See [security policy](SECURITY.md).
Classic v3 calls the same saved-credential preference Save Password and hides the
Auto Login checkbox, which did not exist in 3.08. Switching themes preserves
any Auto Login preference set in Modern/v4. Unlike the historical default,
Save Password requires an explicit choice. Show offline contacts controls the
shared roster filter. External IM login remains disabled.
The manual connection form has explicit HTTPS/TLS selectors in every appearance.
These secure endpoint choices intentionally extend the historical v3/v4 forms.

Registration, account recovery, language changes and room-avatar preferences
remain separate from the authenticated chat scope. Historical server-page
layouts and device-native font/editor behavior remain fidelity limits. The
existing Go service pages are functional reconstructions rendered through the
original922/923 contract. See the [original rendering contract](../../research/applets/v4/docs/rendering.md)
and [reproducible reference captures](../../research/applets/v4/docs/capture.md).
Avatar views currently draw placeholders; fetching and displaying user images
is a later slice. Classic v3 changes presentation only: all three appearances
continue using the existing v4-compatible client over HTTPS and TLS.

New messages illuminate the whole inactive conversation tab in orange. The
Classic themes follow the original shared750ms on/off cadence and gradients;
Modern uses the same cadence on its entire native glass button. Selecting
the conversation clears the cue. Reduce Motion keeps a steady orange highlight.
An open service page or Appearance screen leaves the retained conversation
eligible for unread messages; the Write editor keeps its conversation selected,
matching the original applet's read behavior.

The [native UI harness](../../tools/mobile/README.md) captures all three appearances
while testing HTTPS and TLS against isolated Go listeners. It also checks theme
persistence, draft/history preservation across theme changes, saved-login
restoration, Auto Login and credential deletion through real Keychain operations.

## Chat Rooms

Open the Rooms tab in Modern, Chat Rooms in Classic v4, or the Chat Rooms row
in Classic v3's Friends list. Expand a category to load its rooms. Filter rooms
searches the categories already loaded on this device; it does not claim to
perform a server-wide search. Room rows show the server's occupancy and capacity.
Their context menu supports joining and adding/removing Favorites.

Joined rooms get separate tabs, drafts, unread counts and transcripts. The room
menu opens Participants, Favorites and Leave Room alongside the original chat
actions. Participants retain server
order and show Admin/Muted metadata; selecting another person opens a private
chat. Leave confirms before discarding an unsent draft. Changing appearance
preserves membership and all room/private drafts.

Rooms use the existing original packets over HTTPS and TLS. Category
footers preserve the server's label and append/replace behavior; Refresh reloads
loaded categories and Favorites. A join's room record and acknowledgment are
processed before its same-response welcome. Pending leave suppresses room
traffic; a rejected leave retains the draft and membership. An unjoined room's
message cannot open a tab. Disconnect clears room membership with no automatic
rejoin or message replay.

The v3 category screen adapts its older favorites overlay to the shared original
v4 client. The menu also opens server Search Rooms and Create Room. Room Info
leads to the server's permission-checked settings: description, capacity,
moderators, bans, ownership transfer and deletion. This remains separate from
the local filter.

## Chat actions and service pages

All three themes share these actions. The [source-backed checklist](../../docs/mobile-chat-actions.md)
records original menu ordering, packets, permissions and compatibility limits.

- Room kick uses706: moderators kick immediately; ordinary users vote according
  to server policy. Moderator mute/unmute uses709/710 and refreshes participants.
  Personal Block uses417 and affects incoming messages to you across chats.
  Unblock uses the original Blocked Users page. Classic v4's historical
  “Block & Mute User” label retains its source-specific behavior.
- Private groups use751–755 and message kind2. Add to Chat converts the current
  private thread, preserving history and drafts. Members may invite; Leave
  removes membership. There is no invented accept/decline handshake or group kick.
- Composer tools insert usernames, server-catalog emoticons and original text
  colors. Slash commands go to the server without a duplicate local echo.
  A leading `/` opens a local command picker in Modern chat and the Classic Write
  editor. Typing filters commands; `/gift ` and `/bot ` expose their subcommands.
  Selecting fills the draft and preserves its arguments/caret; only Send executes
  it. Room/community context and observed mute state filter suggestions, while
  unobservable permission requirements are labeled and checked by the server.
  Choices that exceed the original 320 UTF-16-unit limit explain why they are
  unavailable. Unknown slash text can still be entered and sent.
  Photo sharing supports camera, system library and scrapbook; selected photos
  are normalized to the original JPEG/frame limits and previewed before Send.
  Received photos and framed emoticons use authenticated original resources.
- Profiles, gifts/store, credit transfer, reports, calls/SMS, scrapbook,
  community groups/forums and settings use the existing service pages over
  either HTTPS or TLS. Native forms preserve hidden CSRF/nonces and text encoding;
  Back does not repeat purchases or other form submissions.
- Contacts support add/edit/delete/move, contact-group creation/rename/deletion,
  presence and status updates. Original server-defined menu extensions retain
  their context and insertion position.

Generic kick notices cannot safely identify a room. Rejoin Room explicitly
sends703 again while preserving its draft/history; Close Local Tab discards
only the local view. Neither action silently retries an uncertain message.
Calls, SMS, mail and payments retain the backend's mock/test-credit boundaries.
Paid content and historical assets retain the documented catalog limitations;
downloaded audio emoticons are not automatically played.

## Run the iOS development app

From the repository root:

```sh
mise -C apps/mobile install node pnpm
mise -C apps/mobile exec -- pnpm install --frozen-lockfile
go run ./tools/devcert -out data/local-tls --test-fixtures
go run ./cmd/omni -env local \
  -https 127.0.0.1:8443 -tls-tcp 127.0.0.1:9120 \
  -tls-cert data/local-tls/server.pem -tls-key data/local-tls/server-key.pem
```

Certificate generation requires a new directory and never replaces existing
keys. Follow [local secure setup](../../docs/mobile-secure-transports.md#local-setup)
to create a dedicated simulator and install only `ca.pem` into its trust store.
The normal certificate covers `localhost`, `127.0.0.1` and `::1`; the additional
fixtures support negative tests. No existing simulator needs to be reset.

In a second terminal, after installing full Xcode and its iOS simulator runtime,
follow [native tool setup](DEPENDENCIES.md) to install the optional Ruby/CocoaPods
profile. Regenerate native configuration, install pods through that profile,
then launch on the dedicated simulator:

```sh
mise -C apps/mobile exec -- pnpm prebuild --platform ios --no-install
EXPO_NO_TELEMETRY=1 mise -C apps/mobile -E ios exec -- pnpm ios --device DEDICATED_SIMULATOR_UDID
```

The app uses an Expo development build because TLS sockets require native code.
Expo Go cannot supply that module. For later development sessions:

```sh
mise -C apps/mobile exec -- pnpm start
```

Sign in with `alice / alice-demo` or `bob / bob-demo`. Connection settings
select HTTPS or TLS. The iOS simulator defaults to the Mac's loopback server:
`https://localhost:8443` and TLS `localhost:9120`. Certificates and hostnames are
verified with no cleartext fallback. A physical device requires a reachable
listener, matching certificate and trusted issuer. The server retains plain
HTTP/TCP listeners for original clients; the Expo app cannot select them.

If Expo's AppleScript window activation is unavailable after a successful build,
start Metro separately and open the installed development app through `simctl`:

```sh
EXPO_NO_TELEMETRY=1 NODE_OPTIONS=--dns-result-order=ipv4first \
  mise -C apps/mobile exec -- pnpm start --localhost
# In another terminal, use the dedicated simulator's UDID.
open -a DeviceHub
xcrun simctl openurl DEDICATED_SIMULATOR_UDID 'exp+mig33-mobile://?__expo_url=http%3A%2F%2F127.0.0.1%3A8081&__expo_disable_fab=1&__expo_disable_auto_launch=1'
```

The IPv4 setting makes Metro's localhost listener match its advertised address.
Xcode27 uses Device Hub; older Xcode releases use the Simulator application.
The launch flags hide Expo's floating developer-tools button for that process;
it otherwise overlaps the app's navigation menu. Developer shortcuts remain
available, and the flags do not change saved Expo preferences.

The native `ios/` and `android/` directories are generated from app config and
ignored. Rebuild native code when native dependencies or app config change.
Android configuration is present; iOS is the first verification target.

## Contracts and client ownership

`src/mig33/generated/contracts.ts` comes from the reviewed message definitions.
`src/mig33/generated/http.ts` projects original gateway paths, MIME types and
session-routing metadata from OpenAPI. Generate and check both from the root:

```sh
sh scripts/generate-contracts.sh
sh scripts/generate-contracts.sh --check
```

The same generator emits `src/mig33/generated/slash-commands.ts` from
`internal/slashcommands/catalog.go`. This is bundled editing assistance for the
Go server's supported commands, not a recovered original registry or a server
capability announcement. It adds no endpoint, packet field or protocol operation;
selected commands still use ordinary message500. See the
[catalog boundaries](../../docs/slash-commands.md#expo-command-discovery).

Secure transport support retains plain TCP and HTTP for original clients and adds
TLS-wrapped TCP and HTTPS with the same framing, handshake, tunnel paths and
operations. Expo offers only the secure transports, with certificate and
hostname verification and no cleartext fallback. This is a transport addition;
new protocol features and discovery endpoints remain a separate future phase.
The [secure transport implementation](../../docs/mobile-secure-transports.md)
records local certificates, listener scope and compatibility tests;
[SECURITY.md](SECURITY.md) records redirect rejection, saved-login migration
and separate Debug/Release network policies.

After iOS, Android design research will assess Material 3 Expressive native
controls, shape morphing, the extended shape library and spring motion against
the pinned Expo/native versions, accessibility settings and performance. The
Android presentation and native secure transport acceptance remain unverified.
See the [Material 3 Expressive implementation plan](../../docs/mobile-android-expressive.md)
for the installed native capabilities and the morphing APIs needing a bridge.

The handwritten codec and session layer use those definitions. They preserve
raw fields, unknown tags, absent versus empty values, signed Java shorts,
bigint longs, and the original string-array decoder's observed behavior.
Frame bounds reject unsafe malformed inputs. No Node or React dependencies
enter the shared protocol/session code.

The HTTP adapter sends binary `image/gif` bodies, assigns the session after201,
uses the original base/session URL rules, and polls after authentication.
The TCP adapter shares the frame decoder and serializes writes until completion.
The native connector supplies bytes only after an authenticated TLS handshake;
the Node connectors exist in test helpers. HTTPS uses `expo/fetch` with native
redirect rejection. Both use the original binary login handshake.

## Checks

```sh
mise -C apps/mobile exec -- pnpm run check
mise -C apps/mobile exec -- pnpm run export:ios
```

Default checks enforce dependency policy, type-check the app, and run codec,
session and transport tests. Native resources, a running server and Java are
not required. Optional live tests skip unless explicitly enabled.

Run the same core against isolated real Go listeners:

```sh
go build -o data/mobile-integration/server ./cmd/omni
MIG33_TEST_SERVER_BINARY="$PWD/data/mobile-integration/server" \
  mise -C apps/mobile exec -- pnpm run check
```

With a server binary configured, this includes simultaneous HTTPS, TLS and
retained plain HTTP/TCP interoperability. The secure integration helper generates
its own temporary certificates; it does not change OS trust. Native certificate
and redirect acceptance uses `--secure-only` in the
[dedicated-simulator harness](../../tools/mobile/README.md).

Each live test generates bounded synthetic PNG resources in its ignored run
directory and explicitly configures the server to use them. Emoticon and photo
round trips therefore work without extracted applet assets, including when
those assets happen to exist locally. The fixture follows `internal/testassets`
and is never used by the production server.

To test against the unchanged original Java client, prepare the ignored JARs
using [applet setup](../../tools/applets/SETUP.md), install JDK21 and uv, then run:

```sh
MIG33_TEST_SERVER_BINARY="$PWD/data/mobile-integration/server" \
MIG33_TEST_ORIGINAL_CLIENT=1 \
  mise -C apps/mobile exec -- node --test tests/interop.test.ts
```

This starts an isolated database and ephemeral listeners. It checks TypeScript
HTTP with original Java TCP, and TypeScript TCP with original Java HTTP.
Reception must pass through the original parser and populate its chat model.
Replies use the original builder and require a correlated acknowledgment.
Logs and captures stay under ignored `data/mobile-integration/runs/`.
Python launch points use uv.

## Verified scope and remaining work

The Node-executed core passes the nine original serializer fixtures, malformed
and fragmented-frame tests, live Go transport tests, and bidirectional
original-Java interoperability. The secure implementation's full mobile check
passed 191 tests with four optional skips; all four separate original-Java
checks passed. Go race tests, vet, formatting and govulncheck passed, generated
contracts are current, and iOS Hermes export passed.

Native secure acceptance passed in `data/mobile-ui/runs/run-uo2mqrvl` on the
dedicated `mig33-Secure` simulator. All nine rejection cases passed: plaintext
targets and expired, wrong-host and untrusted certificates for both HTTPS and
TLS, plus an HTTPS-to-HTTP redirect. The harness drained the probes before
verifying zero application payloads at rejected targets. Modern HTTPS and
Classic v3 TLS then completed Unicode room/page forms, moderation, private
blocks, photos, colors/emoticons, group creation/invitation/participants/chat/
leave and logout against the retained plain HTTP compatibility peer. The run
exited successfully and restored the generated project and simulator appearance.
See [secure transport evidence](../../docs/mobile-secure-transports.md#acceptance-and-follow-up).

The following results are the **previous plain-transport baseline**, before
the secure-only migration: iOS prebuild, native build and Metro/Hermes export
passed, and XCTest on an iPhone18 Pro simulator running iOS27.2 verified
HTTP/TCP messaging and real Keychain login preferences. That full light-mode
run also verified appearance persistence and retaining a
live conversation and draft across theme changes. Native control acceptance
passes in light mode and dark mode with accessibility-large Dynamic Type,
including retained browse state, native menus/alerts/sheet detents, private and
room replies, whole-tab unread cues, Reduce Transparency and Increase Contrast.
The large-text pass verifies the login disclosure, refresh buttons, disabled
Send, the newest reply above the composer, and reading-history position during
incoming messages. Dark-mode expanded-feature acceptance also passes Modern
HTTP and Classic v3 TCP moderation, groups, personal blocks and photos.
These simulator runs used hardware-keyboard input; software-keyboard geometry
remains unverified because the run did not expose enough visible software keys.
Android native operation and physical iOS devices have not been verified.
The focused Classic v3 run also passes HTTP and TCP messaging, appearance
restoration after restart, and conversation/draft retention through
v3 → Modern → v4 → v3. Its source-color checks use independently extracted
paint-call locations and gradient/category literals from the original JAR.

There is no automatic replay after a timeout or lost connection: a failed send
can have an unknown delivery outcome. Backgrounding closes the local
connection; sign in again to reconnect. HTTP server session expiry and TCP
disconnect cleanup retain their existing behavior. Contact caching and
incremental419 synchronization, persistent transcripts, and a full
resume state machine are later slices.

Password hashing uses Java UTF-16 code units. The portable lowercase policy
covers ISO Latin-1 and unchanged uncased characters. Non-Latin-1 uppercase
passwords requiring device-dependent historical mappings are rejected with
an explicit error until a target-device policy is verified.

Both Node scopes retain the three-day release delay, exact pins, frozen
lockfiles, and documented advisory-backed exact-version exceptions. None are
enabled. Known development-tool advisories are recorded in
[DEPENDENCIES.md](DEPENDENCIES.md), without a blanket audit exemption.
