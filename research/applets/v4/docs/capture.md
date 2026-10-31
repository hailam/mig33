# v4.62 original-bytecode captures

The [local gallery](../artifacts/screenshots/240x320/index.html) contains **39 PNG captures** produced by the preserved v4.62 JAR. [The manifest](data/screenshots-240x320.json) records hashes, capture descriptions and runtime dependencies. No decompiled or control-flow-transformed class was executed.

## Profile

The application Canvas is **240 × 320 pixels**, using the original `default_v2` theme and `en-US` language packet. MicroEmulator supplies fonts and LCDUI behavior. The screenshots show application drawing, excluding native device softkey bars, native text editors and OS dialogs. Font rasterization is emulator/host dependent; these are not claims of exact historical handset pixels.

Capture manifests retain the source hashes used for each recorded run. The current gallery was refreshed on 2026-10-31 with the installed desktop JDK and the normalized MicroEmulator `Displayable.java` shim. Earlier manifests remain in Git history. Image hashes can change with the desktop font rasterizer; a refreshed manifest describes the refreshed local PNGs rather than claiming pixel identity with an earlier host.

The gallery includes all nonempty static form families reached by the direct constructor: Add IM Friend, Contact Details, manual connection, password recovery, Login, Invite Friend, registration details, generic information, resend verification code, account activation, welcome, first run, add-phone-contact, connection-type chooser, add-mig33-contact, registration username availability, and a standalone CAPTCHA form. Three longer forms include final focus/scroll captures. Eighteen original menu factories are exercised over the original Login screen.

The Add IM Friend screen uses an explicitly synthetic provider record named `MSN (fixture)`, because the actual provider registry arrives from the server. The generic information screen contains sample text. The mobile-number fixtures use `+9600000000`. The standalone CAPTCHA uses a synthetic image reading `FIXTURE`. Registration is captured with no server CAPTCHA image; it does not reconstruct that missing image. Menu items reflect the offline session flags, so unavailable items use the default theme’s disabled-text cyan (`#09acd3`). The Login underlay is deliberately selected to keep these menu captures reproducible; it does not assert that every menu is normally reachable from Login.

Server-rendered pages, arbitrary server menus, live room/contact/profile data, camera content, and handset-native screens are outside this capture set. The screen and API specifications describe the mechanisms needed to reproduce those states when the missing inputs are supplied.

## Compatibility work

The original application JAR remains byte-for-byte unchanged. Two emulator compatibility adjustments are required:

1. The client loads packaged packets through `new Object().getClass().getResourceAsStream(...)`. On a modular desktop JDK, `Object` cannot see application-classpath resources. The script creates a resource-only JAR containing exactly the original `default_v2` and `en-US` bytes, and exposes it through `--patch-module java.base=...`. It contains **no executable classes**. The original packet parser still loads and decodes the original bytes.
2. MicroEmulator 2.0.4 throws on `Displayable.removeCommand(null)`, which the client calls during startup. The harness uses the upstream [Issue 44 fix, commit `578b567`](https://github.com/barteo/microemu/commit/578b56783b417ea72349290c9b11dce4e895ccb7), whose `Displayable` implementation ignores null removal. The complete upstream source and its license header are retained at [`scripts/shims/javax/microedition/lcdui/Displayable.java`](../../../../tools/applets/v4/shims/javax/microedition/lcdui/Displayable.java).

These adjustments affect resource visibility and command-removal compatibility. Screen geometry, theme packets, widget painting, form constructors, menu factories and localization are executed from the original mig33 JAR.

## Network and reproducibility

The local Connector shim rejects all protocols and stream-opening calls. It is checked before MIDlet initialization with a mandatory refused request. A validated complete run reported **four refused attempts**, including the self-test; no request was sent by the MIDlet. RMS is in-memory and the emulator home is a fresh temporary directory. A `VideoControl` interface stub provides linking only; no camera action is invoked.

```sh
sh tools/applets/v4/capture_screens.sh
```

Both Python steps in this launcher run through `uv run --no-project python`.

The script uses the same installed JDK defaults as the v3 capture tool. Override `JAVA_BIN` and `JAVAC_BIN` if necessary; the resource bridge requires a modular JDK. It compiles the harness and compatibility shims, runs the original JAR with blocked connections, and regenerates the gallery and hashes. [`V4Capture.java`](../../../../tools/applets/v4/V4Capture.java) records all forced states and fixture values. Distinct v4 class output prevents v3 and v4 obfuscated class names from colliding.

Validation: the full script completed without skipped forms or menus and indexed 39 captures. Startup, Login, registration, Add IM Friend, activation, presence, and Main Menu were visually inspected. Different JDK font rasterization can change image hashes.

## Live UI references for the mobile reconstruction

The 2026-10-31 refresh also ran the original `pair` and `contacts` acceptance suites against isolated Go processes and databases. All four clients passed over original HTTP/TCP, producing 241 binary frames and 22 Canvas screenshots. The original JAR hash is unchanged. These use synthetic local accounts and server responses; they establish the original renderer's behavior with those inputs, not historical account content.

The ignored evidence directory is [`data/v4-ui-reference-native`](../../../../data/v4-ui-reference-native/). Its `evidence-summary.json`, source/harness manifests, binary, wire captures and model text retain the exact run inputs. Reproduce into a new evidence directory with:

```sh
uv run --no-project python tools/applets/v4/integration/isolated_acceptance.py \
  --evidence data/v4-ui-reference-native --suites pair,contacts
```

| UI state | Original Canvas reference | What it establishes |
| --- | --- | --- |
| Login | `artifacts/screenshots/240x320/15-login.png` | Compact form, four checkbox rows, beveled action and underlined links; no welcome artwork on this form. |
| Welcome | `artifacts/screenshots/240x320/43-welcome.png` | Centered logo image, introductory paragraph, Login/Registration/Change Language actions. |
| Connection chooser | `artifacts/screenshots/240x320/54-connection-type.png` | Original explanation, TCP/HTTP choice and manual-connection route. |
| Friends | `data/v4-ui-reference-native/pair/alice/friends-after-login.png` | Active My Friends tab, selected self-summary, Email row, group count, presence icons and cyan row separators. |
| Custom contact group | `data/v4-ui-reference-native/contacts/hailam/edited-contact-custom-group.png` | Empty default Friends group plus expanded custom group; display names differ from usernames. |
| Private conversation | `data/v4-ui-reference-native/pair/alice/chat-from-bob.png` | Active peer tab, avatar gutter, inline colored sender prefix, black body and wrapped continuation. |
| Contact display name in chat | `data/v4-ui-reference-native/contacts/hailam/chat-after-add-contact.png` | The original transcript and tab use the saved contact display name. |

The table mixes v4-relative `artifacts/` paths with repository-relative `data/` paths. Application Canvas captures omit handset softkey chrome. Touchable controls, safe areas, larger text and the mobile keyboard therefore require explicit adaptation decisions beyond these original pixel references.

## Contact status and avatar regression

The extended original `contacts` suite independently verifies the contact fields that the mobile reconstruction exposed as reversed in the server adapter: **402 tag 27 is status text, tag 28 is the avatar key; 421 updates status and 423 updates the avatar**. Bob uses the original 601 status builder, 504 photo upload/gallery flow and 602 avatar builder. Hailam receives the real server responses through the unchanged original parser and dispatcher.

At each checkpoint, the harness asserts the live roster's `t.h` status and `t.p` avatar, reads the original persisted RecordStore `31` using `au.a(byte[])`, and checks the original `t.a(t)` → `au.a(byte[])` serialization round trip. Captured packets independently check the opcode, contact ID and field slots. Status and avatar fixtures differ deliberately, and each push must preserve the other value.

| Checkpoint | Original rendered status | Original avatar key |
| --- | --- | --- |
| 402 contact response | `Status from 402` | `photos/1` |
| 421 status push | `Status from 421` | `photos/1` |
| 423 avatar push | `Status from 421` | `photos/2` |

The isolated run at [`data/v4-contact-fields-regression`](../../../../data/v4-contact-fields-regression/) passed with two original clients, 177 frames and 19 screenshots. Its `contacts/hailam/contact-fields-{402,421,423}.png` files show the actual Friends Canvas; adjacent `-fields.tsv` and `-rms.bin` files retain the asserted model and serialization evidence. The run uses synthetic 12×12 PNG fixtures. Its manifest retains the unchanged publisher JAR hash, the exact corrected Go binary/source, and earlier harness attempts.

Reproduce into a fresh evidence directory:

```sh
uv run --no-project python tools/applets/v4/integration/isolated_acceptance.py \
  --evidence data/v4-contact-fields-regression --suites contacts
```
