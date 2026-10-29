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

The script defaults to the installed Homebrew JDK path. Override `JAVA_BIN` and `JAVAC_BIN` with absolute executable paths if needed. It compiles only the capture harness and shims, loads the preserved original JAR, writes PNGs, then regenerates the gallery and hash manifest. `tools/applets/v3/capture-src/Capture.java` is the complete record of all forced states and fixture values.

Validation: a complete 240 × 320 run generated 46 images without exceptions. Login, registration, settings, contacts and chat were visually inspected. Native font changes between JDKs or operating systems may change image hashes even when original application drawing instructions are unchanged.
