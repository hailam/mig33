# Expo transport policy

The Expo application accepts only HTTPS and TLS connections. Both carry the
existing mig33 packet protocol; this does not introduce a replacement API.
Defaults are `https://localhost:8443` and TLS `localhost:9120`. The device must
trust the server certificate, whose subject alternative names must include the
configured hostname or IP address.

`src/ui/connectionSettings.ts` validates runtime input as well as form values.
The session controller, connection probe and platform transport factory all
pass through that policy. The shared HTTP/TCP protocol adapters remain usable
by explicit original-client compatibility tests, outside the Expo factory.

Remembered-login envelope version2 binds credentials to the secure mode and
endpoint. Version1 and old `http`/`tcp` modes are removed with an actionable
notice, including records that previously enabled Auto Login. The app does not
reinterpret an old port, restore its password into the form, or automatically
retry the old connection through another transport.

## HTTPS redirects

The app explicitly injects `fetch` from the already pinned `expo/fetch` module.
Every request, including polling and the unauthenticated connection test, uses
`redirect: 'error'` and `credentials: 'omit'`. Authentication uses the original
SID header/path; ambient web cookies are unnecessary.

This must not be replaced by React Native's global fetch. The installed
React Native0.88 `Libraries/Network/RCTHTTPRequestHandler.mm` forwards the
redirected request from its URLSession delegate. A final-response URL check
alone would occur after a redirected request could disclose its contents.
React Native also documents limitations of its fetch redirect options in its
[networking guide](https://reactnative.dev/docs/network).

The installed Expo58.0.20 implementation enforces the policy before following:

- iOS: `ios/Expo/Fetch/NativeResponse.swift`,
  `urlSession(_:task:willPerformHTTPRedirection:newRequest:completionHandler:)`,
  supplies `nil` to the completion handler unless the mode is `follow`, then
  rejects an `error` redirect. `ExpoURLSessionTask.swift` forwards this delegate
  decision to URLSession.
- Android: `android/src/main/java/expo/modules/fetch/NativeRequest.kt` disables
  both OkHttp `followRedirects` and `followSslRedirects` when the mode is not
  `follow`. `NativeResponse.kt` rejects a redirect in `error` mode.

`HTTPTransport` also rejects a reported redirect or changed response URL before
dispatching packets. This is a secondary integrity check; native refusal is
what prevents forwarding. No cross-origin or downgrade redirect is allowed,
including same-origin redirects. See the
[Expo fetch documentation](https://docs.expo.dev/versions/latest/sdk/expo/#expofetch-api).

## Native build policy

`plugins/withSecureNetworking.cjs` runs during Expo prebuild:

- iOS Release uses the normal `Info.plist` with arbitrary cleartext loads and
  local-network exceptions disabled. Debug uses a separate `Info.Debug.plist`
  with exact exceptions for `localhost`, `127.0.0.1` and `::1`, for local Metro.
  Subdomains are excluded. There is no broad local-network or TLS-version
  exception. These ATS exceptions are host-scoped; the platform cannot limit
  them to Metro's port. The app's independent gateway policy still requires
  HTTPS/TLS in Debug.
- Android's main network-security resource denies cleartext and trusts system
  CAs. Debug/debugOptimized override only exact loopback hosts and emulator
  host alias `10.0.2.2` for Metro. User-installed development CAs are trusted
  only through Android's `debug-overrides`. Release has no user-CA override.

These use Apple's
[ATS exception rules](https://developer.apple.com/documentation/bundleresources/information-property-list/nsapptransportsecurity/nsallowslocalnetworking)
and Android's
[network security configuration](https://developer.android.com/privacy-and-security/security-config).
Local certificate generation and installation belong to development tooling;
the app contains no certificate-validation bypass or automatic trust prompt.

The policy tests cover runtime endpoint rejection before client construction,
old-login deletion, secret-storage validation, HTTP redirect handling, and
Debug/Release policy separation. They complement actual native certificate,
redirect and handshake acceptance tests; a JavaScript fetch mock alone cannot
establish native redirect or certificate behavior.
