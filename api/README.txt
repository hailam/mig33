Original compatibility contracts and generated adapters

The original HTTP and raw TCP interfaces remain unchanged. The message
contract targets the recovered J2ME 4.62 client, not every historical
server or the separate 3.08 profile.

Contract ownership and generation

    audited research catalogs and original serializer fixtures
        -> reviewed api/mig33-v4.json
        -> Go generator in tools/contracts/
        -> api/asyncapi.yaml
        -> api/generated/mig33-v4.ts
        -> api/generated/coverage.json

mig33-v4.json is the single reviewed message-definition source. It has 111
records: all 56 outgoing catalog entries and all 55 incoming dispatcher
entries. Of these, 53 requests and 52 responses/events belong to both
gateway transports. Two direct resource results and four local objects
remain documented without being advertised as gateway operations.

Each definition retains direction, delivery, opcode, tags, accessor views,
presence observations, conditions, constructor variants and provenance.
SHA-256 fingerprints and JSON pointers bind it to audited evidence.
Changing an evidence input requires reviewing the affected definitions
before updating its recorded hash. Generation does not interpret prose
or silently refresh evidence hashes.

Run from the repository root (Go only):

    sh scripts/generate-contracts.sh
    sh scripts/generate-contracts.sh --check
    go test ./tools/contracts ./internal/protocol

Generated files are deterministic and require neither Java nor the
original JAR. Original artifacts remain ignored. The Go backend does not
require Node, pnpm, or a root JavaScript workspace.

AsyncAPI 3.0.0 uses four logical directional channels: request and response
directions for TCP and for the HTTP tunnel. They reuse message components.
From the described server's perspective, receive means client-to-server;
send means server-to-client. Channel identifiers are not new routes.
This is a recovered compatibility target, not an assertion that every
operation is currently implemented by the Go reconstruction.

Payload schemas describe an inspection projection:

    { opcode: 200, transaction: 1, fields: { "1": "0001" } }

The network sends binary frames, never that JSON object. Field values in
the projection are exact lowercase hexadecimal bytes. Unknown signed-short
tags remain allowed; absent and present-empty differ. Only the frame
envelope is required. Observed getters and builder conditions are not
historical server validation rules.

x-mig33-wire describes framing and primitive views. x-mig33-observation
retains the reviewed operation, branches, dynamic/delegated consumers,
reply observations and evidence. x-mig33-field preserves each field's
value views and separate presence-test views. These are project extensions,
not standard bindings for a custom binary protocol. Multiple views of a
single raw field (notably 203 tags31/32) must not become independently
encoded fields. Constructor sites can compose; they are not necessarily
mutually exclusive schema variants.

Generated TypeScript exports raw packet types, inspection projections,
opcode/tag metadata, transport membership and optional observed accessor
types. Long accessor values use bigint. It is a type/index artifact, not
a client SDK, decoder, request builder, or session implementation.
Full conditions and dynamic fields remain in the source and AsyncAPI.
Future Expo code can consume these declarations without a separately
published package; its codec/session core can stay app-local.

Standards validation and dependency policy

mise manages tool versions; pnpm manages Node validation dependencies.
Both are scoped to tools/contracts. Install and validate from repo root:

    mise -C tools/contracts install node pnpm
    mise -C tools/contracts exec -- pnpm install --frozen-lockfile
    mise -C tools/contracts exec -- pnpm run check
    mise -C tools/contracts exec -- pnpm audit

The first use of a checked-in mise configuration may require a local
trust step. Follow mise's prompt after reviewing the file.

pnpm-workspace.yaml enforces minimumReleaseAge: 4320 (three days) for direct
and transitive dependencies. It configures this single tool package, not a
root app workspace. Direct dependencies and tools are pinned; CI uses the
frozen lockfile. Scoped mise pins also have a three-day release-age policy.

A proven security advisory can justify an age exception for the exact
package version that fixes it. Verify the fixed version against the advisory;
an HTTPS link alone does not prove that relationship. Record its URL, reason and expiry in
tools/contracts/security-exceptions.json and match it with an exact
minimumReleaseAgeExclude entry. No blanket exclusions are enabled.
Review and remove expired exceptions.

Go vulnerability checks run locally and in the existing Go CI job:

    sh scripts/check-go-vulnerabilities.sh

This pins govulncheck independently of application dependencies and uses
its live vulnerability database. It fails for vulnerable code reachable
from this project's source. Module-only advisories may still be reported
when the affected package/symbol is not imported or used.

Contract CI audits the locked Node dependencies, validates AsyncAPI with
the official parser, checks original fixture projections and invalid
envelopes, type-checks the declarations,
verifies evidence coverage and generated drift, and validates OpenAPI.
These checks do not establish untested historical server behavior, a
complete state machine, or device-level Expo TCP support.

HTTP contract and generated adapter

openapi.yaml describes 23 actual HTTP paths and 28 operations using OpenAPI
3.0.3. It preserves historical URLs, native binary envelopes, HTML pages and
the reconstruction JSON API. It does not describe TCP framing as REST.

Run from the repository root:

    ./scripts/generate-openapi.sh

This validates the entire document with kin-openapi v0.142.0, then generates
Go transport DTOs and the standard net/http server interface/router using
oapi-codegen v2.8.1. The generated runtime dependency is
github.com/oapi-codegen/runtime v1.8.0. Tool modules are pinned independently
of application dependencies. Generated types are defined in the HTTP adapter
and mapped by controllers; they do not enter services or repository ports.

internal/controllers.Controller directly implements the generated
httpapi.ServerInterface, with handlers split across feature files in one
package. Controllers consume generated parameters and request DTOs, invoke
application service ports, and map domain results to generated response DTOs.
Capability services implement the use cases using their own models and
consumer-owned ports. cmd/internal/wiring injects the SQLite/sqlc adapters;
no controller accesses repositories or generated database records.

Only the modern-json tag is selected for generation: health, catalog,
contacts, session metadata, transcript sync/history/groups, message sending
and read-position updates. Media, browser, historical page and native gateway
handlers remain compatibility adapters.

/sites/index.php has exactly one GET and one POST operation. c/a/v select
the historical page. x-legacy-routes points to the implemented selector registry
in internal/adapters/legacyweb/page_routes.go, the historical catalog and public selector
exceptions. The historical catalog records historical
recognition, not current implementation or complete per-page form schemas.
Conditional field requirements and permissions still belong to the page
dispatcher. Do not create invented OpenAPI paths containing query strings.

The generated standard server wrapper binds query/header parameters; it
does not implement authentication, all schema constraints or application
rules. Bind errors occur before application handlers. The httptransport adapter
delegates binding errors to existing feature validation to preserve status and
authentication ordering. net/http GET patterns also match HEAD; the adapter
preserves the existing explicit method checks and 405 responses.

The generation proof test verifies modern endpoint dispatch, required
parameter parsing, unchanged request body delivery, and fallback routing
for historical pages and native gateway sessions.

Contract details that should not be normalized accidentally:

- GET /api/chat/session reads an existing authenticated SID; it is not login.
- JSON request conversation kind is textual; Message.kind in replies is 1/2/3.
- API failures are usually plain text; photo/service failures can be HTML.
- The native HTTP tunnel returns image/gif MIME with binary packet bytes,
  not a GIF image. Language resources use application/octet-stream.
- Native page packet 922 invokes the same dispatcher directly and receives
  response packet 923; no internal HTTP request is performed.
- A SID in a gateway path is a path parameter, not an OpenAPI apiKey scheme.
  Header and browser-cookie sessions are modeled as apiKey schemes.
