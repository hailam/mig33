# Native iOS UI smoke test

This harness drives the real Expo development app with XCTest. It signs in as
Alice, opens Bob from Friends, writes a message, verifies Bob's network reply,
opens the menu and logs out, using Modern's inline composer over HTTPS,
Classic v4's separate Write Message screen over TLS, and Classic v3
over HTTPS. All three themes must survive a process restart. Switching between
all three while signed in must retain the conversation and unsent draft. The Bob
peer and Go backend use temporary loopback listeners and an in-memory database.
The harness assigns temporary HTTPS/TLS and original HTTP/TCP listeners; it
does not reuse the developer's default 8443/9120 or 8080/9119 listeners. The Bob
fixture still uses the original plain loopback HTTP gateway, checking that the
secure app and retained compatibility transports share the same services.
The runner generates synthetic sprite/avatar PNGs in its ignored run directory
and explicitly supplies that resource directory to the backend. Emoticon/photo
checks need no recovered applet assets; this fixture is shared with the Node
live tests and never changes production resources. Its colored cells are test
content, so native captures do not prove historical emoticon artwork fidelity.
Modern Friends must expose the native glass marker, so this acceptance requires
an iOS simulator that supports Liquid Glass with Reduce Transparency disabled.

After both transport round trips, the same test verifies Remember Login across
a process restart, authenticates with the restored password, verifies Auto Login,
then unchecks Remember and confirms both fields are empty after another restart.
This exercises the actual iOS Keychain; storage errors fail the test.

Prerequisites are the app's existing mise Node/pnpm/Ruby profile, installed
CocoaPods bundle, generated iOS workspace, full Xcode and a simulator. No extra
Node, Ruby or Python dependencies are installed by these tools.

Use a newly created **dedicated simulator** with the local development CA
installed. Generate new ignored certificate material from the repository root:

```sh
go run ./tools/devcert -out data/local-tls --test-fixtures
```

The output directory must not already exist. Reuse valid existing material or
choose a fresh `-out` directory and pass it with `--tls-dir`. Certificates cover
`localhost`, `127.0.0.1` and `::1`; `--test-fixtures` additionally creates
wrong-host, expired and untrusted certificates. Follow
[dedicated simulator setup](../../docs/mobile-secure-transports.md#local-setup)
to select available device/runtime IDs and create a separate simulator. After
booting that new simulator, trust only the normal CA:

```sh
xcrun simctl keychain DEDICATED_SIMULATOR_UDID add-root-cert data/local-tls/ca.pem
```

No existing simulator is reset. The runner does not install or remove trust,
and private keys stay local. Rebuild native code after applying the TLS package
patch and secure-network config plugin; see the [app setup](../../apps/mobile/README.md).
[SECURITY.md](../../apps/mobile/SECURITY.md) describes certificate verification,
native redirect rejection, retired saved logins and Debug-only Metro exceptions.

Build the backend from the repository root:

```sh
go build -o data/mobile-integration/server ./cmd/omni
```

Keep Metro running in another terminal. Explicit IPv4 resolution avoids Expo's
localhost listener/advertisement mismatch on this Mac:

```sh
EXPO_NO_TELEMETRY=1 NODE_OPTIONS=--dns-result-order=ipv4first \
  mise -C apps/mobile exec -- pnpm start --localhost
```

Then run with the dedicated simulator's UDID:

```sh
uv run --no-project python tools/mobile/ios_smoke.py \
  --device DEDICATED_SIMULATOR_UDID --tls-dir data/local-tls \
  --appearance light --secure-only
```

`--secure-only` first rejects plaintext targets and wrong-host, expired and
untrusted certificates on both HTTPS and TLS, plus an HTTPS-to-HTTP redirect:
nine rejection cases. It requires rejection captures, stops and drains the
probes, checks their `probes-stopped` marker, and verifies zero application
payloads at rejected targets. It then runs the expanded room, group,
moderation, page and photo flow over Modern HTTPS and Classic v3 TLS. It is
mutually exclusive with other focused modes. Probe events stay in
`secure-probe-events.jsonl`.

This workflow passed in `data/mobile-ui/runs/run-uo2mqrvl` on the dedicated
`mig33-Secure` simulator (`F3680A72-058D-41B7-ACD0-A3EB3FE27A3A`). Its
`result.json` records all security checks and the complete feature workflow;
XCTest completed with zero failures, the runner exited successfully, and the
generated project and simulator appearance were restored. This run does not
verify Android, physical iOS devices, software-keyboard geometry or the
separate full Keychain lifecycle workflow.

All modes now require TLS material; `--tls-dir` defaults to `data/local-tls`.
Omit `--secure-only` for the original full appearance/Keychain workflow, now
over secure app transports. After that pass, capture Dark Mode with the shorter
Modern HTTPS flow:

```sh
uv run --no-project python tools/mobile/ios_smoke.py \
  --device DEDICATED_SIMULATOR_UDID --appearance dark --visual-only
```

`--appearance` is optional and restores the simulator's previous setting even
when the test fails. `appearance.json` records that restoration. `--visual-only`
checks login, native glass Friends, inline messaging, both chat and Friends
menus, and logout; it omits TLS sockets,
theme restart/switching and Keychain lifecycle checks. Its `result.json` records
only that narrower scope.

For focused v3 validation on both original transports:

```sh
uv run --no-project python tools/mobile/ios_smoke.py \
  --device DEDICATED_SIMULATOR_UDID --appearance light --v3-only
```

This checks Classic v3 login, Friends, Write Message, replies, menus and logout
over HTTPS and TLS, saved appearance after a restart, and live conversation/draft
preservation through v3 → Modern → v4 → v3. It leaves Classic v3 selected and
does not repeat the saved-login/Keychain lifecycle checks.

For room acceptance using the original protocol over HTTPS and TLS:

```sh
uv run --no-project python tools/mobile/ios_smoke.py \
  --device DEDICATED_SIMULATOR_UDID --appearance light --rooms-only
```

This uses Modern over HTTPS and Classic v3 over TLS. It expands the public
category, locally filters Lobby, joins and checks the first welcome message,
opens participants and finds Bob, adds and removes a favorite, and sends a room
message that the isolated Bob peer acknowledges and answers. Bob joins Lobby
and Gaming before the app starts. Tapping Bob in participants also opens a
private conversation, verifies a private reply, then returns to the joined room.
The check keeps different unsent drafts in
both rooms, switches through all three themes with the conversation intact,
then confirms leaving each room and returning to the catalog. Room captures
include the catalog, filtered list, welcome, participants, favorites, reply,
retained drafts and leave confirmation. Peer evidence records kind3 and the
canonical room target, so a private-message reply cannot satisfy the room test.
This focused flow does not repeat the full private-chat lifecycle, saved
appearance after restart, or the Keychain lifecycle. `--rooms-only`, `--v3-only` and `--visual-only` are
mutually exclusive; the default private-chat flow remains available.

For the expanded original chat features on both transports:

```sh
uv run --no-project python tools/mobile/ios_smoke.py \
  --device DEDICATED_SIMULATOR_UDID --appearance light --features-only
```

This focused workflow uses Modern over HTTPS and Classic v3 over TLS. Alice
creates a room through the original two-stage page form, submits Unicode and
punctuation in its description, and checks the rendered Room Info round trip.
Real Bob, Carol, Dave and Hailam gateway clients then join that room. The UI
mutes Bob, verifies that his actual room send is rejected, unmutes him and
verifies delivery, then kicks him and checks an authoritative707 participant
reply. A separate room is created through the original page transport by Bob:
Alice is an ordinary member there, so her706 starts a vote rather than an
immediate moderator kick. Carol and Hailam supply the remaining votes; the
test verifies Dave remains after the first two votes and is absent after three.

The same workflow personally blocks Bob, verifies suppressed incoming text,
opens the original Manage Block List page, unblocks him through its form and
checks resumed delivery. It creates a private group from Bob's conversation,
invites Carol and Dave, checks original755 membership, exchanges a group
message and reply, and confirms753 removes Alice's membership and local tab.
It also delivers a real500 photo from Bob, opens its original full-size page
resource, checks the composer color picker, and sends a supplied emoticon
shortcut through500 before verifying the acknowledged draft becomes empty.
Screenshots and real peer events accompany each stage. These checks do not
claim full coverage of every linked service, media source, composer setting or
historical server policy.

`--features-only` is mutually exclusive with the other focused modes. Its
fixture control listener binds only to a temporary loopback port and requires
a random per-run token. It is owned by the harness, never registered in the
application or backend, and only directs real original-protocol clients.
Fixture setup does not grant roles through a test API or write the database.
The runner records scope separately and still restores its temporary Xcode
project and simulator appearance. Do not run any native mode until all app and
repository writes are paused.

Use `--unread-only` for focused unread-tab acceptance. The authenticated Bob
fixture sends real private messages while Friends is selected. XCTest captures
Modern's brighter/dimmer amber-field phases and Classic v4/v3's whole-tab
orange/normal phases,
checks that changing themes retains unread state, and verifies opening the
conversation clears it. Modern's native control must retain its size and open
during the animation; a second incoming message rearms the Classic checks.
It covers HTTPS and TLS. A separate HTTPS pass enables
the real iOS Reduce Motion setting, checks a steady unread cue in all three themes,
then restores the previous system setting. Settings changes happen while
logged out because backgrounding the app intentionally suspends its session.
Phase test IDs are non-accessible views; VoiceOver only announces unread
messages, not animation implementation details.

Use `--modern-only --appearance light` and a separate `--appearance dark` run
for focused Modern presentation acceptance. It checks the original logo on
Login, the stable successful-login destination with native glass, real private
and room message round trips, text editing, menu actions, system alerts,
and a composer sheet dragged from medium to large. It also captures the whole
amber field beneath native glass in both phases and verifies selecting its conversation
clears unread state. Transient login animation frames are not delayed or polled
for a screenshot; a screen recording is needed to review those frames.
It also verifies that collapsed Friends groups, expanded room categories and
the room filter survive navigation, and captures native row-options menus.
Modern Login keeps Remember Login visible and secondary options inside its
ellipsis menu. The harness checks the menu's44pt target, all four entries,
native checked states for Auto Login/invisible, and Auto Login enabling Remember.
After a send acknowledgment it checks that the empty Send control is disabled.
Real incoming messages overflow the transcript, then a short final reply must
fit entirely above the composer without a repair scroll. A separate real drag
enters history; the next incoming message must preserve that reading position.

Add `--content-size accessibility-large` for a Dynamic Type pass. The runner
reads the existing simulator size before changing it, restores it in cleanup
even on test failure, and records both values in `content-size.json`.

This mode then enables the real Reduce Transparency and Increase Contrast
settings in separate, logged-out passes. It checks the app's solid glass
fallback and captures login controls, Friends and native sheets for contrast
review. It restores each original setting using normal cleanup plus registered
XCTest teardown, including on interrupted assertions. The runner's appearance
override is independently restored. It uses HTTPS; `--features-only` remains
the broader HTTPS/TLS protocol and native-presentation regression workflow.

Use `--commands-only --appearance light` for the local slash picker workflow.
It runs Modern over HTTPS and Classic v3 over TLS, covering `/` discovery, prefix
filtering, private/room scope, gift subcommands, dismissal and retained draft,
selection without sending, and explicit private/room `/wave` delivery. The peer
asserts exactly two private and two Lobby action results across both transports;
selection must not execute a command. Modern checks the settled picker/editor/nav
bounds. The workflow also checks Log out/Cancel wording and 44pt touch targets,
captures the system alert, cancels without ending the session, then logs out.
Keyboard-responder checks prove editing continuity; they do not establish
software-keyboard geometry when the simulator uses a hardware keyboard.

Run after native prebuild/pod installation with app and repository edits paused
for the entire test. Metro can refresh even after changes to tests or README
files; a refresh during a gesture can discard the tap. An earlier recording
captured exactly this at Bob's correctly targeted first tap, so retrying taps
would hide an unstable test input. Also avoid concurrent native builds.
The runner temporarily adds a
`Mig33SmokeUITests` target and scheme using the already pinned `xcodeproj` gem,
then restores the project and removes its temporary scheme in `finally`.
The Ruby helper uses `mise exec -- ruby -rbundler/setup` so the pinned interpreter
loads the locked gems directly; a second Ruby lookup through `bundle exec` can
accidentally select macOS's system Ruby. `prepare.log` records the actual runtime.
Simulator builds retain Xcode's normal ad-hoc signing (`CODE_SIGN_IDENTITY=-`)
and simulated application-identifier entitlement injection. Disabling signing
omits those Mach-O entitlement sections and makes SecureStore fail with OSStatus
`-34018`; a paid Apple developer account is not required for this simulator run.
An original project copy, process logs, configuration, peer message evidence,
XCTest result bundle and screenshot attachments remain under ignored
`data/mobile-ui/runs/run-*`. No app source is patched for testing.

Screenshots include `http-modern-friends`, `http-modern-conversation-reply`,
`tcp-classic-compose-message`, `http-classic-v3-friends`, Appearance selections, restored themes,
retained conversation/draft, and menu/logout states. XCTest also
retains accessibility trees for empty chat, the reply, and the focused chat
menu check, plus the screen and accessibility tree on failure. These are functional
smoke captures; compare them with the separately captured original v3/v4 renderer
before claiming visual fidelity.

The harness uses XCTest's public `XCUIApplication.open(_:)` API to open the
Expo development URL, so it does not require AppleScript desktop automation.
Expo's launch flags `__expo_disable_fab=1` and `__expo_disable_auto_launch=1`
hide developer overlays for that process without changing saved preferences.
The floating tools button otherwise overlaps Modern's navigation menu and can
intercept a correctly targeted XCTest tap.

These runs normally use simulator hardware keyboard input. Software-keyboard
layout is reported as **not verified** unless the test records visible keys,
on-screen keyboard geometry, and an unobscured message field and Send control.
The corresponding capture and result field distinguish that check from typing.
The attempted public macOS
System Events menu inspection was denied with Apple Events error `-1743`, so no
keyboard setting was changed. Software-keyboard layout needs a separate check
with desktop Automation permission or manual use of Device Hub's keyboard menu.
