# Reviewed dependency backports

These patches are registered for exact package versions in
`../pnpm-workspace.yaml`; the lockfile records their hashes. No major versions
are overridden and no audit finding is ignored. Remove each patch after Expo
adopts a compatible release containing its fix, with the regression tests kept
as evidence until the dependency path is removed.

| Patch | Upstream change | Adaptation |
| --- | --- | --- |
| `glob@13.0.6.patch` | [4682e8c9babee1b385202ede36389634e12076d7](https://github.com/isaacs/node-glob/commit/4682e8c9babee1b385202ede36389634e12076d7), [GHSA-3g3m-2fjr-6pw4](https://github.com/advisories/GHSA-3g3m-2fjr-6pw4) | Four traversal callbacks return immediately when aborted; `onResume` retains callback completion after cancellation. The five upstream edits are applied to both distributed walker modules and the bundled public entry points. |
| `minimatch@10.2.6.patch` | [77a0f5c4b80bdc641c3c930a1d9edf5ad2db57f8](https://github.com/isaacs/minimatch/commit/77a0f5c4b80bdc641c3c930a1d9edf5ad2db57f8), [GHSA-g3hv-75wg-m249](https://github.com/advisories/GHSA-g3hv-75wg-m249) | Bound `**/..` preprocessing by `braceExpandMax` (default100,000), and skip quadratic deduplication above1,000 patterns. Applied to CommonJS and ESM without changing the v10 matching APIs. |

The reviewed upstream changes are translated into published JavaScript;
package metadata, dependency declarations, typings and unrelated algorithms
are unchanged. Original package licenses remain in their distributions.
The existing source maps are not regenerated; debugger source locations inside
these patched dependency internals may therefore be approximate.

`tests/security-dependencies.test.ts` resolves packages through the actual Expo
dependency graph and checks both module formats. Bounded probes fail before the
patch and pass afterwards; they avoid an unbounded denial-of-service payload.
Promise, streaming and synchronous traversal paths are checked, along with
normal Expo patterns and fingerprint exclusions. Raw `pnpm audit` continues to
report the original versions and all four findings; see
[dependency decisions](../DEPENDENCIES.md) for the unresolved findings.

## Native TLS hardening

`react-native-tcp-socket@6.4.3.patch` is a local security correction to the
already pinned release, not an upstream advisory backport or a dependency-age
exception. It is registered and hashed in the same workspace and lockfile.
Rebuild the native development client after installing it; reloading JavaScript
cannot replace its Objective-C/Java changes.

The app's `createTLSTransport(host, port)` retains the original packet framing
and uses platform certificate-chain and hostname trust. Its API exposes no
custom CA, trust-all option, or plaintext fallback. The package's other TLS
configuration APIs are outside this adapter's supported surface.

The reviewed Android source validated the chain using the default factory but
lost the requested hostname when resolving its address. Raw `SSLSocket` does
not enable hostname verification automatically. The patch layers TLS with the
original hostname, sets `SSLParameters` endpoint identification to `HTTPS`,
and sends DNS SNI while omitting SNI for numeric addresses. Verification occurs
during the handshake, before readiness or application reads. These APIs require
Android API24 or newer. See [Android's SSLSocket warning](https://developer.android.com/privacy-and-security/security-ssl#WarningsSslSocket),
[endpoint verification](https://developer.android.com/reference/javax/net/ssl/SSLParameters#setEndpointIdentificationAlgorithm(java.lang.String)),
and [SNIHostName](https://developer.android.com/reference/javax/net/ssl/SNIHostName).

On iOS, the existing default path supplies `kCFStreamSSLPeerName` to
CocoaAsyncSocket and retains system trust evaluation. The patch exports
`socketDidSecure` as a native `secureConnect` event. Android emits that event
only after its handshake returns, before starting its reader. The JavaScript
bridge no longer invents the event from an ordinary `connect`. Android also
bounds the handshake with the connection timeout, reports read failures, and
emits close even when a failed handshake has already closed the descriptor.

`tests/native-socket.test.ts` covers readiness, early-data rejection,
cancellation, binary-copy ownership, and native write completion/failure.
`tests/native-tls-bridge.test.ts` executes the installed package bridge against
an injected native event bus, checking event provenance and listener cleanup.
These tests do not prove platform certificate verification. Native acceptance
must separately exercise trusted, untrusted, wrong-host and plaintext servers;
Android runtime validation remains pending until an SDK/emulator is available.
