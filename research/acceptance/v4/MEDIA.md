# Native media and service acceptance

Run against an isolated backend with the unchanged original v4.62 JAR:

```sh
MIG33_TEST_HTTP_PORT=28080 MIG33_TEST_TCP_PORT=29119 \
  sh tools/applets/v4/integration/run-media.sh
```

The backend must listen on those loopback ports and advertise `http://127.0.0.1:28080`. Use a fresh test database; the harness uploads photos, sends mail/messages and records pending provider requests. It runs Alice over TCP, then a fresh Alice JVM over HTTP. `--compile-only` compiles without connecting.

The harness invokes original login preparation, builders, request queue, transport, receive dispatcher, media decoders, browser parser and paint code. It verifies:

- Dynamic local address-book descriptors, their visibly local labels, decoded927 icons, four automatic206 directory acknowledgments, and zero startup errors or207 provider-state packets.
- All914 content types: static PNG, vibration-image PNG, framed animation and framed image/WAV. Every request includes the original trailing space; each response reaches the original emoticon cache.
- PNG/JPEG504 upload sources, omitted/present tag4 parameter, purposes0/1,500 raw-photo uploads and500 scrapbook references, photo913/914 previews, and602's duplicate-tag anomaly/reset.
- Actual gallery pixels and decoded browser image cache entries surviving paint. Image-purpose923 cache durations use milliseconds, unlike page durations; zero prematurely expired valid images. The backend now returns60000ms for successful image responses, while dynamic page responses retain0.
- Original public1001/1002 language listing and1003/1004 installation of the exact bundled English pack, followed by925 acknowledgment.
- Native SMS500 saved as pending_provider with an explicit0 response, local mail500 delivered with an honest ACK, invitation900 and callback800 saved pending. No804 telephone action is received.
- Correlated logout acknowledgments for both transports.

Fixture pixels stand in for camera hardware. Actual camera acquisition, physical audio output and vibration are not asserted. Type4's WAV stream is decoded by the original class; media requests use action mode0 to avoid hardware side effects.

The original icon cache requires nested RMS open counts; the [MicroEmulator compatibility shim](../../../tools/applets/v4/shims/RMS-COMPATIBILITY.md) fixes that emulator defect without editing the applet. Page capture invokes original paint before checking the deferred-tab flag, because original paint can recover a lost tab notification. It still requires two stable populated captures and never changes that flag directly.

See [results](media-result.md), [evidence summary](runs/media-out/evidence-summary.json), and the [service contract](../../../docs/native-services.md).
