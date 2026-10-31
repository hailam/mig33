# Secure transports for Expo

Implementation date: 2026-10-31. Expo now accepts only HTTPS and TLS sockets.
Native iOS secure acceptance passed in `data/mobile-ui/runs/run-uo2mqrvl`:
all nine rejection cases and the complete HTTPS/TLS feature workflow passed.

## Compatibility boundary

The original HTTP and plain TCP listeners remain available for original clients.
Separately configured HTTPS and TLS-wrapped TCP listeners serve Expo, using the
same binary frames, authentication handshake, gateway paths, polling rules,
opcodes and application services. No command-discovery endpoint, new protocol
version or additional packet fields are part of this work. Expo's command
suggestions remain a bundled catalog generated from Go metadata.

Expo enforces secure modes at connection creation, probing and saved-connection
restoration as well as in the form. Old version1 HTTP/TCP saved logins are
removed with an actionable configuration message; their credentials are not
restored or replayed. Version2 binds saved credentials to an explicit HTTPS/TLS
endpoint. The shared library and original-client acceptance tools retain plain
adapters. See the [application security policy](../apps/mobile/SECURITY.md).

## Go implementation

The existing config and lifecycle now support optional `http.tls_listen` and
`socket.tls_listen`, with `tls.cert_file`, `tls.key_file` and
`tls.handshake_timeout`. CLI equivalents are `-https`, `-tls-tcp`, `-tls-cert`
and `-tls-key`; the timeout is configured through YAML or
`MIG33_TLS_HANDSHAKE_TIMEOUT`. Secure listeners are disabled until configured.
Certificates are loaded before starting the application graph or listeners.
The secure adapters share that graph with the original listeners, using
[`tls.NewListener`](https://pkg.go.dev/crypto/tls#NewListener) and HTTPS with
[`http.Server.ServeTLS`](https://pkg.go.dev/net/http#Server.ServeTLS).

TLS requires at least TLS 1.2. Handshakes have bounded deadlines and are tracked
for shutdown before bytes enter the existing socket decoder. Certificates are
loaded at startup: deploy renewed certificate/key files and restart the process
to load them. Listener addresses and `public_url` remain explicit configuration;
they are not inferred from a request's Host header.

The current `public_url` is deliberately HTTP for legacy clients. URLs supplied
in original203/922 exchanges are also identifiers: Expo's legacy-page adapter
sends them inside original packets and receives page/image bytes through that
transport. Their spelling alone does not imply an additional cleartext request.
Preserve those identifiers while encrypting the enclosing connection.

The HTTPS listener exposes only the native root/session gateway and `/healthz`.
Browser, operator, JSON API and direct-resource routes remain on the original
HTTP listener. Original page/photo packets still travel inside the secure
tunnel. Exposing the entire browser router over HTTPS is a separate phase:
its WAP form actions, links and Origin validation need consistent secure browser
origins while preserving legacy behavior.
Relevant code: [HTTP router](../internal/adapters/httptransport/router.go),
[browser validation](../internal/controllers/browser.go),
[legacy URL policy](../internal/adapters/legacyweb/transport.go).

## Native connector and trust

The pinned `react-native-tcp-socket@6.4.3` already provides
[`connectTLS`](https://github.com/Rapsssito/react-native-tcp-socket#tlsconnecttls).
The Expo connector resolves only on authenticated `secureConnect`, retaining
abort handling, binary delivery, write completion and framing bounds. HTTPS
uses the existing binary tunnel adapter with a required HTTPS origin and the
installed `expo/fetch`. Its native iOS/Android implementations reject redirects
before following; React Native's global fetch is not used for app protocol
requests. Both same-origin and cross-origin redirects are rejected, including
HTTPS-to-HTTP downgrades. Requests omit ambient cookies.

Installed-source review found two important details:

- Upstream Android creates an `SSLSocket` without configuring hostname
  verification. The version-pinned patch now verifies the original hostname
  and supplies it for SNI before reporting success, using platform chain trust.
  [Android TLS guidance](https://developer.android.com/privacy-and-security/security-ssl)
- iOS's default trust path sets `kCFStreamSSLPeerName`. Its custom `ca` branch
  compares certificate bytes rather than providing general CA-chain validation.
  The Expo adapter uses OS-trusted certificates without a custom `ca` or disabled
  verification; it does not use that custom branch as a CA bundle interface.

Development CA/server material stays ignored and must match the actual hostname
or IP. Install trust only on a newly created dedicated simulator; the installed
`simctl keychain` interface does not offer individual root removal. Android's
Debug network-security config accepts user-installed development CAs; Release
uses system CAs only. Neither platform has a certificate-verification bypass.
[Android debug trust configuration](https://developer.android.com/privacy-and-security/security-config#debug-overrides),
[Apple certificate trust](https://support.apple.com/en-us/102390).

Release-wide cleartext exceptions are removed. The Expo config plugin emits
separate Debug/Release policies: Debug allows exact loopback hosts for Metro,
plus `10.0.2.2` on Android. These are host-scoped exceptions, not app transport
choices. ATS does not enforce raw-socket security; the native connector enforces
TLS. [Apple ATS guidance](https://developer.apple.com/documentation/security/preventing-insecure-network-connections).

## Local setup

From the repository root, generate a **new** ignored directory. The generator
does not overwrite existing material or install trust:

```sh
go run ./tools/devcert -out data/local-tls --test-fixtures
```

The normal server certificate lasts 30 days and includes `localhost`,
`127.0.0.1` and `::1`. The local CA lasts one year. `--test-fixtures` also creates
wrong-host, expired and untrusted certificates for negative acceptance tests.
Trust only `ca.pem`; keep all private keys local. If the directory already
exists, reuse its valid material or choose a fresh `-out` directory and pass
that path to the commands below and the harness's `--tls-dir` option.
Use `-hosts` with comma-separated DNS names/IPs when generating certificates
for a different development endpoint; do not include URL schemes or ports.

Create a dedicated simulator using available device/runtime identifiers:

```sh
xcrun simctl list devicetypes
xcrun simctl list runtimes
xcrun simctl create 'mig33 Secure Tests' DEVICE_TYPE_ID RUNTIME_ID
# Use the newly returned UDID in each following command.
xcrun simctl boot DEDICATED_SIMULATOR_UDID
xcrun simctl bootstatus DEDICATED_SIMULATOR_UDID -b
xcrun simctl keychain DEDICATED_SIMULATOR_UDID add-root-cert data/local-tls/ca.pem
```

This installs trust only into that dedicated simulator. No existing device is
reset, and the app/harness never installs trust automatically.

For interactive development, start all four listeners:

```sh
go run ./cmd/omni -env local \
  -https 127.0.0.1:8443 -tls-tcp 127.0.0.1:9120 \
  -tls-cert data/local-tls/server.pem -tls-key data/local-tls/server-key.pem
```

The Expo defaults are HTTPS `https://localhost:8443` and TLS `localhost:9120`.
Original HTTP/TCP stay at the configured legacy addresses (normally 8080/9119).
Keep the existing `public_url` for original browser/page identifiers. For a
physical device, configure a reachable listener and a certificate covering the
actual host; loopback certificates and simulator trust do not configure it.

## Acceptance and follow-up

Go and Node checks cover listener lifecycle, secure settings and saved-login
rejection. The live `secure-integration.test.ts` exercises all four listeners,
original pages and media; original-Java HTTP/TCP acceptance remains separate.
The native harness now has a dedicated secure workflow:

```sh
go build -o data/mobile-integration/server ./cmd/omni
uv run --no-project python tools/mobile/ios_smoke.py \
  --device DEDICATED_SIMULATOR_UDID --tls-dir data/local-tls \
  --appearance light --secure-only
```

Prepare the native development build and Metro first, as described in the
[harness setup](../tools/mobile/README.md). The passing run
`data/mobile-ui/runs/run-uo2mqrvl` used the dedicated `mig33-Secure` simulator
(`F3680A72-058D-41B7-ACD0-A3EB3FE27A3A`). Its `result.json` records nine rejected
connections: plaintext targets, expired certificates, wrong-host certificates
and untrusted certificates for both HTTPS and TLS, plus an HTTPS-to-HTTP
redirect. After shutting down and draining the probes, the harness verified
the `probes-stopped` marker and zero application payloads at rejected targets.

The same run completed Modern HTTPS and Classic v3 TLS login, Unicode room
creation and original page forms, room information, mute/unmute, moderator
kick and ordinary vote-kick, block/unblock, photo preview, text color and
emoticons, group creation/invitation/participants/messaging/leave, and logout.
The compatibility peer used the retained plain HTTP listener. XCTest passed
with zero failures; the runner exited successfully and restored the generated
project and simulator appearance.

The full mobile check passed 191 tests with four optional skips; four separate
original-Java interoperability checks also passed. Go race tests, vet,
formatting and govulncheck passed; the 111 packet contracts and five generated
artifacts are current, and iOS Hermes export passed. Android native operation,
physical iOS devices and software-keyboard geometry remain unverified.

No dependency additions were needed by the transport adapters. Retain
app-local mise/pnpm, exact pins and the 72-hour Node release-age policy; only a
documented advisory can justify an exception. Python tooling uses uv. Go checks
continue to include govulncheck. Android's native presentation research is in
[Material 3 Expressive](mobile-android-expressive.md); protocol enhancements
remain a separate future phase.
