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
internal/services.Application implements the use cases and depends on domain
types and repository ports. Bootstrap injects the SQLite/sqlc implementation;
no controller accesses repositories or generated database records.

Only the modern-json tag is selected for generation: health, catalog,
contacts, session metadata, transcript sync/history/groups, message sending
and read-position updates. Media, browser, historical page and native gateway
handlers remain compatibility adapters.

/sites/index.php has exactly one GET and one POST operation. c/a/v select
the historical page. x-legacy-routes points to the implemented selector registry
in internal/services/page_routes.go, the historical catalog and public selector
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
