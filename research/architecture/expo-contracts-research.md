# Expo integration with the original mig33 API

Research date: 2026-10-30; implementation update: 2026-10-31. Current-phase constraint: retain the original API and protocol through both original transports, HTTP and TCP/socket. Both are required compatibility surfaces. Introducing a new application API is a separate future phase. The [contract workflow](../../api/README.txt) and [Expo client slice](../../apps/mobile/README.md) implement the initial stages. The codec and Node-executed client pass original-Java interoperability over both transports; native iOS simulator validation remains pending Xcode. Sections below retain the research rationale; the app README records current commands and limitations.

The Expo app should act as another compatible client of the reconstructed original service. Its UI and internal TypeScript abstractions can be new, while its network requests retain the original routes, packet formats, authentication, and operation semantics. The client design needs a shared packet codec and session core with separate HTTP and TCP/socket adapters. Both existing server transports remain supported and independently verified. An HTTP-first experiment is only a testing sequence, not a decision to drop socket support.

## Scope and evidence

The working baseline is the preserved Java ME 4.62 client, its recovered documentation, and executable compatibility fixtures. The separate 3.08 profile must remain distinct where behavior differs. Evidence recovered from one client build does not establish every historical server behavior.

| Evidence | What it establishes |
|---|---|
| [Original endpoint catalog](../applets/v4/docs/endpoints.md) | TCP/HTTP gateways, session URL rules, HTTP headers, page and resource entry points |
| [Recovered protocol description](../applets/v4/docs/protocol.md) | Login, contacts, messages, rooms, presence, framing, transactions, and observed quirks |
| [Outgoing catalog](../applets/v4/docs/data/protocol-outgoing.json) and [incoming catalog](../applets/v4/docs/data/protocol-incoming.json) | Machine-readable client observations with provenance, subject to documented extraction limits |
| [Original-client packet fixtures](../../internal/protocol/testdata/original_v4_packets.json) | Examples produced by the original serializer for compatibility verification |
| [Go codec](../../internal/protocol/packet.go) and [HTTP gateway](../../internal/controllers/native.go) | Current reconstruction behavior, including explicit bounds and transport error handling |
| [OpenAPI contract](../../api/openapi.yaml) | Current HTTP interface description, containing both original-compatible surfaces and project-specific extensions |

The existing `/api/chat/*` JSON endpoints are not evidence of an original REST chat API. Their missing JSON login and room operations are not prerequisites for this phase. They remain outside the proposed compatibility client's dependency surface. No removal of existing endpoints is proposed.

For each required feature, distinguish original-client observations, currently implemented server behavior, and unresolved historical behavior. If a feature is missing, research its original operation before proposing a compatible implementation. Do not fill evidence gaps with newly designed endpoints.

## Original transports and session behavior

The original core API sends binary packets through either TCP or an HTTP tunnel. Feature pages and resources also use recovered HTTP routes and, in some flows, packets 922/923. Their exact entry points are documented in the [endpoint catalog](../applets/v4/docs/endpoints.md).

| Concern | Compatibility requirement |
|---|---|
| HTTP addressing | Base gateway URL for specified pre-session/control operations; session-suffixed URL for other requests and polling, following the recovered rules |
| HTTP requests | POST serialized binary packets; GET polls for queued packets |
| Media type | Original tunnel requests use `image/gif` even though the body is binary protocol data; responses use the same historical media type |
| Login | Preserve `200 → 201 → 202 → 203`, including challenge hashing, session assignment, and original client fields |
| Results | Distinguish HTTP failures, binary opcode 0 errors, opcode 1 acknowledgments, multi-packet replies, and unsolicited events |
| Contacts and rooms | Use the recovered packet operations and completion/checkpoint semantics |
| Web features | Preserve observed page/resource routes, server-provided bases, and form behavior where needed |
| Recovery | Investigate the original session, pending-request, synchronization, and reconnect behavior; do not assume a new replay-cursor API |

The current [HTTP gateway](../../internal/controllers/native.go) accepts one or more framed packets per POST and returns replies plus queued delivery frames. GET drains queued frames immediately and can return an empty body; it is not an SSE stream or a newly designed long-poll endpoint. A successful HTTP status does not mean every enclosed operation succeeded.

The HTTP contract now describes original `image/gif` POST bodies alongside accepted `application/octet-stream`. The current implementation does not enforce request Content-Type. This corrects the contract description without changing the API.

The original session handshake uses Java string hashing over UTF-16 code units and historical password case conversion. Reuse original-client fixtures to verify TypeScript behavior, including non-ASCII values. Do not substitute browser-form login, bearer authentication, or a new JSON login route.

The current server tracks delivery completion around writes and flushes. Its [delivery acceptance tests](../../internal/acceptance/chat_test.go) distinguish failed transport writes from successful delivery. Neither a transaction ID nor a successful server write should be treated as proof of exactly-once end-user receipt. Retry decisions must follow the actual protocol; the JSON API's idempotency-key behavior cannot be assumed for native packet sends.

## Expo transport research

| Required transport | Compatibility fit | What must be proven |
|---|---|---|
| Original binary HTTP tunnel | Existing original-client transport; retained alongside TCP | Byte-preserving requests/responses, MIME behavior, session routing, empty polls, multiple frames, ordering, and cancellation |
| Original raw TCP | Same packet protocol with original connection behavior | Suitable Expo native module, development-build integration, fragmentation, keepalive, and reconnect handling |

Expo documents a cross-platform Fetch implementation, and React Native documents HTTP networking. These support investigating the HTTP tunnel without adding a server API, but they do not prove this client's binary handling on a device. Compare actual sent and received bytes with fixtures in iOS and Android builds. [Expo Fetch](https://docs.expo.dev/versions/latest/sdk/expo/), [React Native networking](https://reactnative.dev/docs/network)

The first platform target remains unresolved: iOS/Android only, or web too. A browser cannot use its WebSocket API as a raw TCP socket. The original HTTP tunnel may be a browser candidate, subject to same-origin/CORS, restricted headers, and cookie behavior. That browser limitation does not remove native TCP from the client design or either transport from the server. Any development proxy must preserve the original requests; it should not introduce a translated JSON API. [WebSocket interface](https://websockets.spec.whatwg.org/#the-websocket-interface)

Mobile background/resume behavior, ambiguous send failures, session expiry, and rejoining rooms need compatibility experiments. Record limitations that the original protocol cannot resolve. A new delivery guarantee or session mechanism belongs in the future API phase.

## What OpenAPI and AsyncAPI can generate

Decision: use AsyncAPI to document the original message contract shared by HTTP and TCP/socket. Keep OpenAPI responsible for the HTTP surface and the existing wire specification/fixtures responsible for exact binary encoding. The implemented workflow generates AsyncAPI 3.0.0 and TypeScript declarations from one reviewed structured source and validates them with pinned tooling. This does not introduce a new server API or establish a complete generated client.

| Contract layer | Role in this phase | Generation limit |
|---|---|---|
| OpenAPI | Describe original HTTP paths, parameters, media types, binary bodies, and observed HTTP errors | Does not describe the packet fields inside a binary body |
| Packet schema/catalog | Describe opcodes, tags, primitive encodings, optionality, and provenance | Existing extraction catalogs need review before serving as complete generator input |
| AsyncAPI | Document original logical messages, direction, transactions, replies, and connection information for both transports | Does not automatically generate the custom binary codec or session state machine |
| Sequence/state documentation | Explain authentication, multipart completion, polling, and reconnect behavior | Must be backed by behavioral tests; declarations alone cannot implement it |

OpenAPI remains useful, but generating a conventional JSON SDK would not produce the required client. `openapi-fetch` exposes custom body serialization and `parseAs: "arrayBuffer"`; evaluate those against a small direct-fetch wrapper using the same documented HTTP contract. Select a wrapper based on byte fidelity and understandable errors, not generation volume. [openapi-fetch API](https://openapi-ts.dev/openapi-fetch/api)

The existing [Go generation configuration](../../api/oapi-codegen.yaml) selects only the `modern-json` tag. It therefore does not generate the native gateway implementation. Any client contract projection must select the original surfaces deliberately and preserve their referenced schemas. Pin tools and reject generated-source drift in CI once a generator is selected.

AsyncAPI can describe the original protocol's logical messages and correlation, including protocols named TCP, but there is no standard binding for our custom packet framing. Any decoded-object representation used for documentation must be clearly labeled; it is not JSON transmitted on the wire. [AsyncAPI specification](https://www.asyncapi.com/docs/reference/specification/v3.0.0)

Reuse message definitions across the two transport descriptions. Describe the HTTP tunnel's batching and polling explicitly: one HTTP response can contain several packets, so a packet must not be equated with an entire HTTP response. AsyncAPI has an HTTP binding, but that binding does not infer this tunnel behavior or the binary encoding. Link HTTP method/media-type details to OpenAPI to avoid competing descriptions. [HTTP binding](https://github.com/asyncapi/bindings/blob/master/http/README.md)

Modelina can generate TypeScript models and Go structs from supported schemas. That is useful only if those models accurately represent the recovered packet semantics; it does not supply binary serialization or reconnect behavior. The newer AsyncAPI baked-in client templates are experimental and are not a basis for assuming a working compatibility SDK. [Modelina](https://github.com/asyncapi/modelina), [template status](https://www.asyncapi.com/docs/tools/generator/baked-in-templates)

For the first experiment, a small TypeScript codec checked against the existing Go and Java fixtures is a stronger starting point than designing a general generator. Generate repetitive opcode/tag declarations or helpers only after confirming the catalog's completeness and ambiguous fields. Kaitai Struct may help independent binary inspection, but its documented write support is limited to Java and Python, so it does not establish a complete Go/TypeScript encoder solution. [Kaitai serialization](https://doc.kaitai.io/serialization.html)

The [source audit](../applets/v4/docs/protocol-catalog-audit.md) verifies the catalogs against the original client. They are faithful within their stated scopes, but are not standalone generator-ready schemas: outgoing entries use compact prose and references to detailed tables, while incoming entries record scoped accessor observations rather than complete response models. Raw fields can legitimately have multiple accessor views; that is not evidence of a corrupt extraction or an impossible wire type. Keep the catalogs as provenance evidence and review the audited definitions into AsyncAPI before generating code.

The implementation keeps one reviewed definition per logical message in `api/mig33-v4.json`, then generates AsyncAPI and TypeScript. Wire/accessor annotations remain attached to these definitions instead of a separately authored TypeScript model. The generated `x-mig33-wire`, `x-mig33-observation` and `x-mig33-field` extensions preserve framing and evidence; ordinary generators do not understand their custom semantics automatically. The small Go generator emits raw packet and accessor declarations rather than a complete SDK. Keep the generic framing specification and original evidence linked separately. [AsyncAPI extensions](https://www.asyncapi.com/docs/reference/specification/v3.0.0#specificationExtensions)

Generating Go types is optional. The existing `internal/protocol` codec and `internal/adapters/nativecodec` mappings have different responsibilities from the normalized application commands/events. Preserve that separation and the [dependency guards](../../internal/ports/boundaries_test.go); do not replace domain models merely because a generator supports Go. A small Go generator could emit reviewed declarations later, but reimplementing an AsyncAPI validator in Go is not justified by repository language preference.

## Binary compatibility requirements

The original frame and tagged fields are:

```text
marker:1 | opcode:2 | transaction:2 | bodyLength:4 | body
body field = tag:2 | length:4 | raw payload:length
```

Opcode, transaction, and tags are signed Java shorts. Numeric fields use big-endian encoding; each opcode determines how to interpret field bytes because there is no transmitted type discriminator. Keep absent and empty values distinct, preserve unknown fields, and handle signed 64-bit values without JavaScript number precision loss.

Duplicate tags retain their final payload. Receivers accept arbitrary field order; the Go encoder sorts fields while the original used a Hashtable. Fixture comparisons must distinguish meaningful byte differences from permitted field-order variation. Include fragmented TCP frames, concatenated frames, truncated input, and length bounds in codec checks.

Do not silently normalize recovered anomalies. For example, response 203 accesses tags 31 and 32 through several numeric/text views of the same raw payload. The [protocol reference](../applets/v4/docs/protocol.md) records the confirmed accesses and uncertainty about their combined application meaning. A generated schema must not erase those views or confuse a presence test with a value's schema or a recovered server validation rule.

## Repository layout and application boundary

Recommendation after comparing layouts: retain the root Go module and start with a UI-independent client module inside the Expo app. The previously proposed `packages/mig33-client` is a later extraction option. One application, two transports, and generated declarations do not by themselves require a separate npm package.

Go's server-project guidance supports `cmd` entrypoints, `internal` implementation packages, and other directories for non-Go code. This repository already follows that model. Keep the backend in place; adding Expo does not require moving it under `apps/server`, splitting the Go module, or introducing `go.work`. [Go project layout](https://go.dev/doc/modules/layout#server-project)

| Option | Fit for the current phase | When it becomes worthwhile |
|---|---|---|
| `apps/mobile/src/mig33` | Recommended: one app, an explicit client boundary, and independent protocol tests | Now |
| `apps/mobile` plus `packages/mig33-client` | Valid but adds a package manifest, exports, linking, and workspace configuration | A second independent JavaScript application or separately distributed SDK |
| `clients/typescript` or `sdk/typescript` | Useful naming for a maintained SDK, with the same packaging costs | External consumers or a separate release/versioning requirement |
| Go client embedded through native bindings | Could share some implementation, but adds mobile build and bridge integration | A substantial reusable Go client core whose savings justify that integration |

A browser target within the same Expo app and a Node-based test runner are not automatically independent library consumers. Pure client tests can run without React while the source remains app-local.

App layout remains proposed; the contract source, generated artifacts and scoped tooling now exist:

```text
go.mod
go.sum
cmd/                                  existing Go entrypoints
internal/
  protocol/                           existing Go binary codec
  adapters/nativecodec/               existing packet/application mapping
api/
  openapi.yaml                        HTTP contract
  mig33-v4.json                       reviewed original-message definitions
  asyncapi.yaml                       generated original-message contract
  generated/                         generated TypeScript declarations and coverage
apps/mobile/
  package.json                        private app package
  pnpm-lock.yaml                      selected package manager
  app.config.ts
  src/
    app/                              routes if Expo Router is selected
    mig33/                            client module, outside the route tree
      generated/                      reviewed generated declarations/types
      protocol/                       handwritten binary codec
      session/                        original client session lifecycle
      transport/
        http.ts                       original HTTP tunnel
        tcp.native.ts                 original raw TCP
  modules/                            only if a custom native module is needed
tools/
  applets/                            existing compatibility tooling
  contracts/                          pinned generator tooling if needed
scripts/                              explicit generation/check commands
research/applets/                     recovered evidence and provenance
```

The subdirectories should follow actual code size; the first slice may need only a few files. Keep the codec and session core free of React and platform imports. HTTP request/poll scheduling belongs in the HTTP adapter; TCP buffering and connection lifecycle belong in the TCP adapter. Share packet definitions and request bookkeeping without pretending the transports have identical behavior.

A root JavaScript workspace is unnecessary initially. Keep the app's package manifest and lockfile under `apps/mobile`. If multiple reusable JavaScript packages later emerge, Expo supports standard workspaces and automatic Metro configuration; extraction remains available. A workspace does not inherently conflict with Go ownership, but it should solve an actual package-sharing need. [Expo monorepos](https://docs.expo.dev/guides/monorepos/)

Keep app-specific Expo/EAS configuration in the app directory. EAS documents running its commands from that directory even when the app lives in a larger repository. Selecting EAS hosting/build services is a separate choice. [EAS repository layout](https://docs.expo.dev/build-reference/build-with-monorepos/)

The client may expose ergonomic local functions such as `login` or `sendMessage`. Those functions encode original operations and decode original replies. They are TypeScript abstractions, not new server endpoints or a new wire protocol.

```text
Expo UI
  → shared compatible TypeScript codec and session core
      ├─ original binary HTTP adapter → existing HTTP gateway
      └─ original TCP adapter         → existing socket listener
  → existing Go native adapter
  → existing application capabilities
```

Use Expo development builds for platform verification. If Continuous Native Generation is selected, maintain native configuration through app config/plugins and keep generated native projects untracked. [Development builds](https://docs.expo.dev/develop/development-builds/introduction/), [Continuous Native Generation](https://docs.expo.dev/workflow/continuous-native-generation/)

## Go reuse and the native TCP dependency

Go-first ownership should preserve backend build independence, application boundaries, and shared protocol evidence. It does not establish that embedding the Go runtime is the lowest-cost mobile design.

`gomobile bind` can produce Android AAR and Apple XCFramework artifacts. An Expo application would still need native integration and a bridge to JavaScript. Gobind supports a restricted exported-type surface; the current `Packet.Fields map[int16][]byte` would need a compatible wrapper. The server codec also does not contain a complete client session engine, so handshake orchestration, pending transactions, polling, and mobile lifecycle work would remain. This makes Go embedding a higher-complexity starting point here, rather than a proven reduction in implementation cost. [Gomobile](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile), [binding restrictions](https://github.com/golang/mobile/blob/master/cmd/gobind/doc.go)

Go WebAssembly also needs runtime integration and host networking. Its documented browser/Node execution path does not establish an Expo raw-TCP solution. Keep it outside the initial approach. [Go WebAssembly](https://go.dev/wiki/WebAssembly)

For the TypeScript approach, `react-native-tcp-socket` is a candidate native dependency, not a verified selection. Its documentation advertises Android/iOS TCP and byte-oriented socket APIs, but its broad React Native compatibility statement does not prove compatibility with the Expo release chosen for this app. Expo SDK 55 and later require the New Architecture; test an actual development build on each target platform. Native data callbacks contain arbitrary byte chunks, not complete mig33 packets. [TCP library](https://github.com/Rapsssito/react-native-tcp-socket), [Expo architecture guidance](https://docs.expo.dev/guides/new-architecture/)

If that dependency does not meet the requirements, a small app-local Expo module is an alternative. Expo explicitly supports local native modules for one application; a separately published package is unnecessary. Selecting a TCP implementation is a required experiment while retaining both original server transports. [Expo module guidance](https://docs.expo.dev/modules/get-started/)

## Build and generation ownership

Preserve the current [.github/workflows/go.yml](../../.github/workflows/go.yml) job: Go formatting, vet, and race tests should continue without installing the mobile app's dependencies. The current workflow does not check generation drift; that would be an explicit addition after choosing generators.

| Check | Proposed dependencies and scope |
|---|---|
| Go backend | Existing Go/C toolchain and formatter workflow; all current Go packages |
| Client core | App's locked Node dependencies; type checks and protocol/session tests without a device |
| Contract generation | Pinned contract tooling; schema validation, regeneration, and diff checks |
| Native integration | iOS/Android development builds; original HTTP and TCP operation matrix |

If AsyncAPI tooling requires Node, keep a small locked tool package under `tools/contracts` instead of making backend contract checks install Expo. That tool package and the app may have independent lockfiles because they serve independent dependency graphs; neither needs to import the other. Existing Go generators can remain pinned through their current scripts. Generated runtime sources should be committed and usable without running generators during ordinary builds. [Go generation guidance](https://go.dev/blog/generate)

One repository-specific detail matters when Node dependencies arrive: `.gitignore` does not control Go package discovery. The repository's Go 1.27.3 toolchain supports module `ignore` directives. A proposed `ignore node_modules` directive would exclude directories of that name at any depth while preserving `go vet ./...` and `go test -race ./...`. Do not narrow checks to only `cmd` and `internal`, because this repository also contains Go packages under `configs` and `db`. This is a future setup change; `go.mod` has not been edited. [Go module ignore directive](https://go.dev/ref/mod#go-mod-file-ignore)

TypeScript tests should read the existing original serializer fixture corpus from the repository, without adding a third committed copy or bundling it into the app. The [Go test fixture](../../internal/protocol/testdata/original_v4_packets.json) is an unchanged copy of the [research fixture](../applets/v4/docs/data/protocol-fixtures.json), explicitly documented by the [Go test](../../internal/protocol/packet_test.go). There are nine such packets, so additional authored edge cases and original-client integration remain necessary. Cross-language agreement alone is weaker evidence than comparison with original bytes.

Extract a package only when there is a concrete second application, external SDK consumer, or independent release requirement. Keeping a narrow local client entrypoint now makes that extraction straightforward while avoiding premature package infrastructure.

## Research experiments before implementation

| Experiment | Acceptance evidence |
|---|---|
| Compatibility inventory | Map login, contacts, messaging, rooms, presence, and required web features to original operations and current server support; label uncertainty and extensions |
| Codec proof | Read original fixtures in TypeScript; compare encoding with original/Go results under the documented field-order rules; verify numeric, text, empty, unknown, and malformed cases |
| Original HTTP session | Complete the binary login handshake, contact synchronization, and private-message exchange from an Expo development build without a new endpoint |
| Original TCP session | Complete the same operations through the socket listener from an Expo development build; verify fragmented reads, partial writes, keepalive, and connection teardown |
| Room and presence flow | Discover/join/leave using original operations and process asynchronous original replies alongside the preserved Java client |
| Failure behavior | Exercise empty polls, application errors within HTTP 200, concurrent replies, disconnect during send, expiration, and restart; document the guarantees actually observed |
| Dual-transport compatibility | Run the required operation matrix over both HTTP and TCP, including concurrent original-client sessions; verify each transport's own delivery and reconnect behavior |
| Generation decision | Prove binary request/response handling before choosing an OpenAPI wrapper; generate packet helpers only from reviewed evidence |

Keep the preserved Java launchers, probes, and integration tools: they supply independent compatibility evidence. Any later implementation must continue to pass original-client checks.

## Separate future phase

A newly designed JSON API, REST login and room endpoints, WebSocket/SSE events, a new replay cursor, or different authentication/session semantics require a separate product and protocol design phase. The earlier HTTP-generator and modern-event approach belongs there. None is a prerequisite for the current compatible Expo client.

The initial study changed research documentation only. The subsequent app-local implementation now provides the codec, original login/contact/message session, and both transports. It introduces no new server endpoint or wire protocol. The user selected the iOS simulator as the first native target.
