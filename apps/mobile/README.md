# mig33 mobile

An Expo development app using the original mig33 4.62 HTTP tunnel and TCP
protocol. The first slice supports login, full contact synchronization, private
text messages, and logout. The Go backend stays in the root module.

## Run the iOS development app

From the repository root:

```sh
mise -C apps/mobile install node pnpm
mise -C apps/mobile exec -- pnpm install --frozen-lockfile
go run ./cmd/omni -env local
```

In a second terminal, after installing full Xcode and its iOS simulator runtime,
follow [native tool setup](DEPENDENCIES.md) to install the optional Ruby/CocoaPods
profile, then launch:

```sh
EXPO_NO_TELEMETRY=1 mise -C apps/mobile -E ios exec -- pnpm ios
```

The app uses an Expo development build because TCP requires native code.
Expo Go cannot supply that module. For later development sessions:

```sh
mise -C apps/mobile exec -- pnpm start
```

Sign in with `alice / alice-demo` or `bob / bob-demo`. Connection settings
select HTTP or TCP. The iOS simulator defaults to the Mac's loopback server:
`http://127.0.0.1:8080` and TCP `127.0.0.1:9119`. A physical device requires a
reachable server address and listener configuration.

The native `ios/` and `android/` directories are generated from app config and
ignored. Rebuild native code when native dependencies or app config change.
Android configuration is present; iOS is the first verification target.

## Contracts and client ownership

`src/mig33/generated/contracts.ts` comes from the reviewed message definitions.
`src/mig33/generated/http.ts` projects original gateway paths, MIME types and
session-routing metadata from OpenAPI. Generate and check both from the root:

```sh
sh scripts/generate-contracts.sh
sh scripts/generate-contracts.sh --check
```

The handwritten codec and session layer use those definitions. They preserve
raw fields, unknown tags, absent versus empty values, signed Java shorts,
bigint longs, and the original string-array decoder's observed behavior.
Frame bounds reject unsafe malformed inputs. No Node or React dependencies
enter the shared protocol/session code.

The HTTP adapter sends binary `image/gif` bodies, assigns the session after201,
uses the original base/session URL rules, and polls after authentication.
The TCP adapter shares the frame decoder and serializes writes until completion.
The native connector supplies socket bytes; the Node connector exists only in
test helpers. Both use the original binary login handshake.

## Checks

```sh
mise -C apps/mobile exec -- pnpm run check
mise -C apps/mobile exec -- pnpm run export:ios
```

Default checks enforce dependency policy, type-check the app, and run codec,
session and transport tests. Native resources, a running server and Java are
not required. Optional live tests skip unless explicitly enabled.

Run the same core against isolated real Go listeners:

```sh
go build -o data/mobile-integration/server ./cmd/omni
MIG33_TEST_SERVER_BINARY="$PWD/data/mobile-integration/server" \
  mise -C apps/mobile exec -- pnpm run check
```

To test against the unchanged original Java client, prepare the ignored JARs
using [applet setup](../../tools/applets/SETUP.md), install JDK21 and uv, then run:

```sh
MIG33_TEST_SERVER_BINARY="$PWD/data/mobile-integration/server" \
MIG33_TEST_ORIGINAL_CLIENT=1 \
  mise -C apps/mobile exec -- node --test tests/interop.test.ts
```

This starts an isolated database and ephemeral listeners. It checks TypeScript
HTTP with original Java TCP, and TypeScript TCP with original Java HTTP.
Reception must pass through the original parser and populate its chat model.
Replies use the original builder and require a correlated acknowledgment.
Logs and captures stay under ignored `data/mobile-integration/runs/`.
Python launch points use uv.

## Verified scope and remaining work

The Node-executed core passes the nine original serializer fixtures, malformed
and fragmented-frame tests, live Go transport tests, and bidirectional
original-Java interoperability. iOS prebuild, TCP podspec autolinking and the
Metro/Hermes export pass. These checks do not replace a simulator run of the
native HTTP and TCP modules; that remains pending full Xcode setup.

There is no automatic replay after a timeout or lost connection: a failed send
can have an unknown delivery outcome. Backgrounding closes the local
connection; sign in again to reconnect. HTTP server session expiry and TCP
disconnect cleanup retain their existing behavior. Contact caching and
incremental419 synchronization, room UI, persistent transcripts, and a full
resume state machine are later slices.

Password hashing uses Java UTF-16 code units. The portable lowercase policy
covers ISO Latin-1 and unchanged uncased characters. Non-Latin-1 uppercase
passwords requiring device-dependent historical mappings are rejected with
an explicit error until a target-device policy is verified.

Both Node scopes retain the three-day release delay, exact pins, frozen
lockfiles, and documented advisory-backed exact-version exceptions. None are
enabled. Known development-tool advisories are recorded in
[DEPENDENCIES.md](DEPENDENCIES.md), without a blanket audit exemption.
