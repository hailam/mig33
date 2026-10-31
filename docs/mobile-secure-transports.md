# Secure transports for Expo

Research date: 2026-10-31. This is the next implementation phase; the current
Expo build still supports the original plain transports.

## Compatibility boundary

Keep the original HTTP and plain TCP listeners for original clients. Add
separately configured HTTPS and TLS-wrapped TCP listeners for Expo, using the
same binary frames, authentication handshake, gateway paths, polling rules,
opcodes and application services. No command-discovery endpoint, new protocol
version or additional packet fields are part of this work. Expo's command
suggestions remain a bundled catalog generated from Go metadata.

Expo must offer only HTTPS and TLS sockets once this phase lands. Enforce that
at connection creation, probing and saved-connection restoration as well as in
the form. Reject old insecure saved connections with an actionable configuration
message; never try them, silently fall back, or guess a secure port. The shared
protocol library and original-client acceptance tools can retain plain adapters.

## Go implementation

Extend the existing config and bootstrap lifecycle with optional secure listener
addresses and certificate/key paths. Load and validate certificates before
starting services so partial startup rolls back cleanly. Keep one application
graph and the existing HTTP/socket adapters. Go supports TLS wrapping with
[`tls.NewListener`](https://pkg.go.dev/crypto/tls#NewListener) and HTTPS with
[`http.Server.ServeTLS`](https://pkg.go.dev/net/http#Server.ServeTLS).

Use modern TLS configuration, bounded handshake deadlines, cancellation and
shutdown tracking before passing bytes to the existing socket decoder. A TLS
handshake must not occupy an unbounded connection slot. Certificate renewal
must have a documented deployment/restart path. Secure addresses and externally
advertised origins must be explicit configuration rather than inferred from an
untrusted Host header.

The current `public_url` is deliberately HTTP for legacy clients. URLs supplied
in original203/922 exchanges are also identifiers: Expo's legacy-page adapter
sends them inside original packets and receives page/image bytes through that
transport. Their spelling alone does not imply an additional cleartext request.
Preserve those identifiers while encrypting the enclosing connection.

Sharing the entire HTTP router needs a further browser audit: WAP form actions
and Origin validation currently use that single public URL. HTTPS browser pages
must not submit forms to HTTP or accept an arbitrary Origin. Before exposing
the complete router over HTTPS, use explicitly configured secure browser origins
and consistent form/link generation while preserving legacy HTTP behavior.
If the initial secure listener exposes only the native gateway needed by Expo,
record that scope explicitly and test that other routes are unavailable there.
Relevant code: [HTTP router](../internal/adapters/httptransport/router.go),
[browser validation](../internal/controllers/browser.go),
[legacy URL policy](../internal/adapters/legacyweb/transport.go).

## Native connector and trust

The pinned `react-native-tcp-socket@6.4.3` already provides
[`connectTLS`](https://github.com/Rapsssito/react-native-tcp-socket#tlsconnecttls).
Adapt the existing native connector to resolve only on `secureConnect`, retaining
abort handling, binary delivery, write completion and framing bounds. HTTPS uses
the existing binary tunnel adapter with an enforced secure URL. Test redirects:
validation of the initial URL alone must not permit an HTTPS-to-HTTP downgrade.

Installed-source review found two important details:

- Android creates an `SSLSocket` and starts its handshake without configuring
  hostname verification. Certificate-chain trust alone is insufficient. Add a
  narrowly scoped version-pinned patch that verifies the original hostname and
  supplies it for SNI before reporting success, with wrong-host acceptance tests.
  [Android TLS guidance](https://developer.android.com/privacy-and-security/security-ssl)
- iOS's default trust path sets `kCFStreamSSLPeerName`. Its custom `ca` branch
  compares certificate bytes rather than providing general CA-chain validation.
  Prefer OS-trusted certificates without a custom `ca` or disabled verification.
  Do not treat that custom branch as an ordinary CA bundle interface.

For development, generate ignored local CA/server material with SANs matching the
actual hostnames/IPs. Use a dedicated simulator for trust installation because
the installed `simctl keychain` interface does not offer individual root removal.
Android can use debug-only trust anchors, expressed through a persistent Expo
config plugin. Production configuration must reject unknown, expired and
wrong-host certificates. [Android debug trust configuration](https://developer.android.com/privacy-and-security/security-config#debug-overrides),
[Apple certificate trust](https://support.apple.com/en-us/102390).

Remove production-wide cleartext exceptions. Keep any Metro development
allowance narrowly scoped; Metro is a development service, not an application
transport. ATS does not itself enforce security for raw sockets, so the native
connector must enforce TLS. [Apple ATS guidance](https://developer.apple.com/documentation/security/preventing-insecure-network-connections).

## Acceptance and follow-up

Verify all four listeners concurrently, unchanged original Java HTTP/TCP
interoperability, Expo HTTPS/TLS login and messaging, room/group operations,
original page/photo delivery, startup rollback and shutdown during handshakes.
Negative tests must reject untrusted, expired and wrong-host certificates and
downgrade redirects before application credentials are transmitted. Include
insecure saved-state restoration and ensure no reconnect path bypasses policy.

No dependency additions are required by the proposed transport adapters. Retain
app-local mise/pnpm, exact pins and the 72-hour Node release-age policy; only a
documented advisory can justify an exception. Python tooling uses uv. Go checks
continue to include govulncheck. Android's native presentation research is in
[Material 3 Expressive](mobile-android-expressive.md); protocol enhancements
remain a separate future phase.
