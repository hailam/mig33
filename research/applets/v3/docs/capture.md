# Original-bytecode screen captures

The [local screenshot gallery](../artifacts/screenshots/240x320/index.html) contains **46 PNG captures** rendered by the original, unmodified v3.08 class files. [The manifest](data/screenshots-240x320.json) identifies every image, fixture and SHA-256 hash. These supplement the screen and rendering specifications; they are not a claim that every server-dependent screen has been recovered.

## Rendering profile and limits

- Canvas size: **240 × 320** pixels. This is the application drawing area, not the full physical handset display including platform chrome.
- Runtime: MicroEmulator **2.0.4**, default device font configuration, Java AWT in headless mode. Font rasterization reflects this emulator and host. It does not establish pixel identity with Nokia, Sony Ericsson or other handset fonts.
- Renderer: original `com.projectgoth.Roshan.c`, `t`, `e`, `r`, `ab`, and `v` bytecode. The harness invokes original constructors and paint methods through reflection. It does not redraw screen widgets in a replacement implementation.
- Native LCDUI command bars, text editors, camera preview and OS permission prompts are not included in the Canvas images. Their appearance depends on the handset runtime.
- Stored preferences are initialized from empty in-memory RMS. The login example follows the blank-credential route and focuses Username. Additional `-end` images show final reachable focus/scroll positions.
- Explicitly marked **fixture** images contain synthetic contacts, room occupancy, chat messages, a mobile number and/or CAPTCHA. These exercise the original renderers without claiming to reconstruct historical server content. The welcome state is forced and does not represent a successful live login.

At 176 × 208, MicroEmulator 2.0.4's `drawRGB` implementation threw a clipping exception when checkbox pixels extended beyond the target image. Those partial experimental images were removed. Only the validated 240 × 320 profile is published.

## What is captured

The gallery includes About, first-run instructions, contact-type selection, add-contact and contact-details forms, contact requests, create/search room, manual connection, connection-type choice, forgot details, login, callback/call-through variants, four external IM setup forms, invite friend, registration, resend code, authentication prompt, and the connection-test welcome state.

It also includes contact/group/chat-room/mail context menus, Settings, the external IM provider menu, the MSN account menu, camera-resolution settings, a room-list fixture, empty and populated contact lists, and a private-chat fixture. Longer forms and primary menus have additional final-selection captures.

Server-delivered page markup, arbitrary server menus, account/profile/store content, downloaded emoticons and camera/photo/file-selection states remain governed by the corresponding specifications and recovered assets. Captures here do not supply missing server responses.

## Offline isolation

[`Connector.java`](../../../../tools/applets/v3/capture-src/javax/microedition/io/Connector.java) shadows the emulator Connector class and throws `ConnectionNotFoundException` for **every** protocol and stream-opening API. The harness checks the resolved class location and makes a mandatory refused test call before starting the MIDlet. The successful run recorded one refusal from that self-test and zero application connection attempts.

The original MIDlet is loaded using the system class loader after the shim on the classpath. RMS uses the emulator's `memory` implementation; the emulator home is a newly created temporary directory. The capture harness does not invoke camera or native platform-request actions. A minimal `VideoControl` interface stub satisfies bytecode linking; it does not implement a camera.

This is a targeted offline rendering harness, not a general-purpose security sandbox for arbitrary JAR files.

## Reproduce

The emulator JAR is from [Maven Central's MicroEmulator 2.0.4 artifact](https://repo.maven.apache.org/maven2/org/microemu/microemulator/2.0.4/microemulator-2.0.4.jar). Its hash is recorded in the capture manifest. The [upstream headless implementation](https://github.com/barteo/microemu/blob/master/microemulator/microemu-javase-swing/src/main/java/org/microemu/app/Headless.java) supplies the rendering context.

```sh
sh tools/applets/v3/capture_screens.sh
```

The script defaults to the installed Homebrew JDK path. Override `JAVA_BIN` and `JAVAC_BIN` with absolute executable paths if needed. Python runs through `uv`. Before compiling, the script verifies both the original JAR and MicroEmulator hashes. It compiles only the capture harness and shims, loads the preserved original JAR, writes PNGs, then regenerates the gallery and hash manifest. `tools/applets/v3/capture-src/Capture.java` is the complete record of all forced states and fixture values.

Validation: a complete 240 × 320 run generated 46 images without exceptions. Login, registration, settings, contacts and chat were visually inspected. Native font changes between JDKs or operating systems may change image hashes even when original application drawing instructions are unchanged.

## v3 theme reconstruction audit — 2026-10-31

The archived publisher JAR was restored and independently hash-checked against
`2b416199dca5f6593eefe8fa1c9b84ec4be6cdc36f68bcf17e1a94193bfd2934`.
All 46 images were regenerated with OpenJDK 21.0.13.0.3 and the recorded
MicroEmulator 2.0.4 payload. The run refused exactly one connection, its mandatory
shim self-test; the application attempted none. PNGs, extracted original assets,
class files and disassembly remain ignored local artifacts.

The focused theme references are:

| Surface | Original renderer / evidence | Capture |
| --- | --- | --- |
| Login | `t` state15; fresh RMS, Username focus; Save Password checked | [15-login.png](../artifacts/screenshots/240x320/15-login.png) |
| Welcome | `t` state43; explicitly forced successful TCP-test fixture | [43-welcome-tcp.png](../artifacts/screenshots/240x320/43-welcome-tcp.png) |
| Connection choice | `t` state42; TCP, HTTP, Manual buttons and pale logo | [42-connection-type.png](../artifacts/screenshots/240x320/42-connection-type.png) |
| Friends | Original `r` roster and controller; synthetic group and contacts | [10-contacts-fixture.png](../artifacts/screenshots/240x320/10-contacts-fixture.png) |
| Private conversation | Original `ab` tabs and `v` transcript; incoming category1, outgoing category0 | [06-chat-fixture.png](../artifacts/screenshots/240x320/06-chat-fixture.png) |
| Contact menu | Original `e` menu created by controller; selected label has a narrow outline | [menu-contact.png](../artifacts/screenshots/240x320/menu-contact.png) |
| Settings | Original constructor menu, including disabled values | [menu-settings.png](../artifacts/screenshots/240x320/menu-settings.png) |

These establish v3's blue/white panels, slanted overlapping tabs, black text,
small status icons, narrow rectangular row highlights and five-stroke gold focus
trim. The roster starts with **Chat Rooms**, then groups and contacts; group
labels do not display counts. There is no avatar column or status subtitle in
this v3 roster. The centered background is the exact 118×62 `k.png`: pale cyan
`mig` with raised pale green `33`. Its SHA-256 is
`15e482baa119ea166345daa3b5927d21ee526f2eb790f7cc68a42c867e865d46`.
The 132×48 `j.png` atlas has SHA-256
`8a129a43efd442072a4e500b5815b84bc2402dcda78998776d3dd0eb4ece46e6`.
Both are available locally under `artifacts/extracted/`.

The original Login orders Username, Password, Save Password, Sign in as
Invisible, Login, Register, Forgot Details, Connection Type, the four external-IM
checkboxes and Show offline contacts. Its separate native field editors and
Write Message TextBox are outside these Canvas captures. A touch adaptation
must identify changed input metrics or controls as adaptations; native softkeys
must not be claimed as recovered pixels.

The direct `setColor` inventory omits gradient arguments and transcript category
returns. [theme-bytecode-evidence.json](data/theme-bytecode-evidence.json) fills
that gap from the verified JAR, independently of application theme code. It
records ten gradient endpoints and all seven transcript categories with
class hashes, JVM descriptors and bytecode offsets. Reproduce with:

```sh
uv run --no-project python tools/applets/v3/extract-theme-evidence.py
```

Set `JAVAP_BIN` if the installed JDK differs. The extractor retains ignored
disassembly under `artifacts/bytecode/` and writes only the compact evidence JSON
into the tracked documentation. In `com.projectgoth.Roshan.d.a(int):(I)I`, the
switch at bytecode1 returns categories0–6 at offsets44/47/50/53/56/59/62:
`#086834`, `#406ac0`, `#ff0000`, `#ff7300`, `#fcc504`, `#993300`, `#000000`.
`ab.a(int,int,boolean,Graphics)` supplies selected, unread and inactive gradients
at call offsets210/284/318; `t.a(Graphics,String,int,int,boolean)` supplies button
gradient arguments at offset60. `ab.a(Graphics)` supplies the tab-strip backing
gradient `#c5daf7` → `#f3f8fe` at call offset46. These are literals in original instructions,
not colors estimated from antialiased screenshots.
