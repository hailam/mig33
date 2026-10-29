# v4.62 original-bytecode captures

The [local gallery](../artifacts/screenshots/240x320/index.html) contains **39 PNG captures** produced by the preserved v4.62 JAR. [The manifest](data/screenshots-240x320.json) records hashes, capture descriptions and runtime dependencies. No decompiled or control-flow-transformed class was executed.

## Profile

The application Canvas is **240 × 320 pixels**, using the original `default_v2` theme and `en-US` language packet. MicroEmulator supplies fonts and LCDUI behavior. The screenshots show application drawing, excluding native device softkey bars, native text editors and OS dialogs. Font rasterization is emulator/host dependent; these are not claims of exact historical handset pixels.

Capture manifests retain the source hashes used for each recorded run. The later precommit cleanup normalized whitespace in the MicroEmulator `Displayable.java` and `RecordStoreImpl.java` shims without changing their code or license headers; historical capture hashes remain unchanged. New acceptance runs record the current harness hashes separately.

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

The script uses the same installed JDK defaults as the v3 capture tool. Override `JAVA_BIN` and `JAVAC_BIN` if necessary; the resource bridge requires a modular JDK. It compiles the harness and compatibility shims, runs the original JAR with blocked connections, and regenerates the gallery and hashes. [`V4Capture.java`](../../../../tools/applets/v4/V4Capture.java) records all forced states and fixture values. Distinct v4 class output prevents v3 and v4 obfuscated class names from colliding.

Validation: the full script completed without skipped forms or menus and indexed 39 captures. Startup, Login, registration, Add IM Friend, activation, presence, and Main Menu were visually inspected. Different JDK font rasterization can change image hashes.
