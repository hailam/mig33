# Go dependency context and bootstrap architecture research

Research date: 2026-10-29. Status: explicit-constructor recommendation implemented; full Go race tests, vet, schema and reference checks passed on Go 1.27.3. Target: original mig33 HTTP/socket compatibility, SQLite/sqlc persistence and four executable entrypoints. The findings below preserve the pre-refactor audit; see the [backend architecture](../../docs/backend.md#composition-routing-and-persistence) for the current implementation.

The subsequent [command-package and domain-boundary study](go-domain-boundaries.md) examines `cmd/internal` and options for splitting application capabilities. Those capability boundaries are now implemented; their final verification status is tracked separately in the [capability implementation record](evidence/go-capability-implementation.json).

The user selected the latest stable Go release rather than retaining the initial Go 1.25 module baseline. Go 1.27.3 is verified in the [official downloads feed](https://go.dev/dl/?mode=json); the [release history](https://go.dev/doc/devel/release) records its release on 2026-10-19. The module requirement has been updated separately from the architecture proposal. The [toolchain validation record](evidence/go-toolchain.json) records the compiler and checks actually run.

Use explicit constructor injection for long-lived dependencies and pass `context.Context` to operations that perform I/O, blocking work or transitively invoke context-aware work. Keep executable composition near `cmd/*`. Restrict `bootstrap` to lifecycle coordination over caller-supplied operations. For this application's current dependency graph, ordinary Go constructors with a small, well-tested lifecycle mechanism are the preferred option. Fx is a credible alternative when declarative dependency modules justify its runtime machinery.

There is no single officially designated Go “gold architecture.” The evidence consists of standard-library contracts, Go team guidance, maintained libraries and their authors' design explanations. The choices below distinguish those contracts from recommendations for this backend. The [source catalog](go-backend-sources.json) retains URLs and supported claims; the [repository audit](go-backend-audit.json) retains counts, locations and file hashes. No new framework has been adopted.

## Findings in the pre-refactor code

These findings describe the source snapshot in the retained audit, before the accepted implementation. Old paths and observations refer to that snapshot. Links now lead to current equivalent owners; the audit retains original hashes and call sites, and the [migration inventory](evidence/capability-test-migration.json) records test relocation.

| Finding | Evidence | Consequence |
| --- | --- | --- |
| Repository ports have 124 feature operations plus `Close`, across eight interfaces; none accepts a context. | `internal/ports/repositories.go` (baseline); [current chat repository port](../../internal/chat/repository.go) | Context propagation stops at the application boundary. |
| SQLite feature repositories contain 242 calls to `context.Background()` across 20 files and 44 calls to `db.Begin()`. | [Detailed storage inventory](go-backend-audit.json), [message persistence](../../internal/adapters/sqlite/repositories/messages.go) | Request deadlines and cancellation do not reach the generated queries or transaction acquisition. These are call-site counts, not distinct named queries. |
| The migration runner has another `db.Begin()` and context-free driver operations. | [Migration runner](../../internal/adapters/sqlite/migrations/migrate.go) | Startup migration has no caller-controlled cancellation path. |
| sqlc already generates context-aware query methods and a `DBTX` interface. | [Generated database boundary](../../internal/adapters/sqlite/sqlc/db.go) | The missing context plumbing is in handwritten ports/adapters, not sqlc. |
| Modern services accept `ctx` and usually check it before taking the global application mutex. | [Transcript use cases](../../internal/realtime/transcripts.go) | Cancellation while waiting for the lock is not observed by that initial check; database calls subsequently lose the context entirely. |
| Native packet operations and trusted administration operations omit context. | [Native service port](../../internal/ports/native.go), `internal/ports/administration.go` (baseline); [current CLI consumer contracts](../../cmd/cli/main.go) | Closing socket I/O does not cancel an already executing application/database operation. |
| The application constructor seeds/loads storage and schedules a timer. Application `Close` also closes the injected repository. | [Application construction](../../internal/realtime/application.go), [presence sweep](../../internal/realtime/presence.go) | Construction, execution and resource ownership are coupled. Lifecycle migration must preserve exactly one owner of repository cleanup. |
| Bootstrap constructs SQLite/services/transports; its runner knows TCP addresses, `net.Listener` and HTTP shutdown errors. | [Original source inventory](go-backend-audit.json), previously `internal/bootstrap/app.go` and `internal/bootstrap/runtime.go` | That draft did not satisfy the requested mechanism-only boundary. |

The dependency graph is small: one repository implementation supplies one shared application, which feeds the HTTP controllers and socket adapter. The number of feature files and queries does not, by itself, make the construction graph complicated. This is why explicit wiring remains practical.

## Accepted implementation

The implementation keeps composition in `cmd/*`, with shared core construction and cleanup in [cmd/internal/wiring](../../cmd/internal/wiring/core.go). [Bootstrap](../../internal/bootstrap/runtime.go) accepts generic startup/run/stop functions and imports only standard-library packages. Its buffered completion channel retains worker and cleanup errors, handles unexpected worker exits and bounds shutdown waiting. No container or additional lifecycle library was needed.

The service constructor now only assembles required repository and logger dependencies. `Initialize(ctx)` owns startup I/O; `Run(ctx)` owns and joins session/vote maintenance; `Close` leaves the caller's repository open. All 124 repository operations and migration work accept caller contexts, and all 44 feature transactions use `BeginTx`. Services preserve cancellation causes while retaining their public error messages. Architecture guards enforce these boundaries.

Cancellation tests cover the failure boundaries that parameter changes alone would miss: database pool waits, rollback after a message insert, driver lock waits, cancellation during a successful commit, retryable delivery reservations, retained own-message cursors, ballot eligibility failures, complete inventory snapshots and committed identity cache updates. Registration's existing transaction now includes its initial roster using existing generated queries. HTTP lifecycle tests cover graceful drain and forced closure; startup tests cover rollback and a shared HTTP/socket session.

The original [source audit](go-backend-audit.json) remains the historical baseline. The [implementation validation record](evidence/go-architecture-implementation.json) retains exact commands, results, toolchain version, source hashes and cancellation limits. It also confirms that all 57 OpenAPI, generated-code, SQL query and migration files are unchanged. The [current backend guide](../../docs/backend.md#composition-routing-and-persistence) records the adopted design and its operational limits; [original-client acceptance](../acceptance/v4/lifecycle-result.md) records native runtime checks separately.

## What to pass and where

| Kind of input | Passing convention proposed here | Examples |
| --- | --- | --- |
| Long-lived collaborators | Required constructor arguments, stored in the receiver | Repository capability, logger, external provider client |
| Component configuration | A small typed value passed at construction | HTTP timeout settings, database path, chat policy |
| Per-operation lifetime | First method parameter named `ctx` | Request cancellation, deadlines, trace propagation |
| Business input and authority | Explicit typed method arguments | Authenticated actor, message command, room identifier |
| Optional behavior | Options only when there are real defaults or independently optional choices | Instrumentation or retry policy |
| Pure computation | Ordinary arguments; no unused context | Vote threshold calculation, parsing a small identifier |

Dependency injection in Go commonly consists of passing dependencies to ordinary constructors. It does not require a container. Constructors should receive the collaborators they actually use, rather than a global application object. [Go dependency injection discussion](https://go.dev/blog/wire)

An API shape for a future extracted use case could be:

```go
// Illustrative design, not a new implementation in this repository.
type MessageWriter interface {
    SaveMessage(ctx context.Context, message domain.Message,
        recipients []string) (domain.Message, error)
}

func NewChat(writer MessageWriter, logger *slog.Logger) *Chat

func (s *Chat) Send(ctx context.Context, actor Actor,
    command SendMessage) (MessageReceipt, error)
```

The constructor stores `writer` and `logger`; `Send` receives its caller's context. A constructor that only assembles values needs no context. An `Open(ctx, ...)`, `Initialize(ctx)` or migration operation that performs I/O does need one, but must not retain that initialization context for later requests. A long-lived service should not store a request context in its struct. [Contexts and structs](https://go.dev/blog/context-and-structs)

Interfaces should be shaped by consumers' actual requirements. Implementations can return concrete types. Our application-owned `ports` package can remain the shared contract location, but it should contain purposeful capabilities; the composite 124-method repository should not become the dependency of every future service. Extract smaller service dependencies when extracting actual use cases. This does not require an interface for every concrete helper or constructor. [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments#interfaces)

Required dependencies should remain explicit. Functional options suit optional configuration; using `WithRepository` to hide a mandatory repository makes an invalid service easier to construct. A typed configuration/dependency struct is reasonable when a constructor has several related inputs, provided required fields are validated and it does not become a service locator. This is our API-design choice, informed by [Uber's functional-options guidance](https://github.com/uber-go/guide/blob/master/style.md#functional-options), rather than a universal prohibition on other constructor styles.

## Context ownership and cancellation

`context.Context` represents the lifetime and metadata of an operation. Pass it through service and repository calls; derive a shorter deadline when a component needs a tighter budget, and release the derived context with its cancel function. Context values are for request-scoped metadata, not database handles, configuration or dependency lookup. Do not pass nil. `WithoutCancel` deliberately removes cancellation and the deadline; it supplies neither a shutdown owner nor durability. [Context API](https://pkg.go.dev/context)

The desired database path is:

```text
HTTP request.Context() or native operation context
  -> controller or socket adapter
  -> service method(ctx, typed input)
  -> repository method(ctx, domain input)
  -> sqlc method(ctx, generated params)
  -> database/sql context-aware operation
```

Go's database guidance explicitly propagates the caller's context into queries, so request cancellation and derived query deadlines can reach storage. Replacing that context midway with `Background()` severs the connection. [Canceling database operations](https://go.dev/doc/database/cancel-operations)

The contexts need distinct ownership:

| Work | Owner and lifetime |
| --- | --- |
| Startup checks, database opening, migrations and initial state loading | Entrypoint-owned startup context with a deliberate budget |
| HTTP request | `r.Context()`, optionally with a tighter operation deadline |
| Native connection | Socket adapter owns connection cancellation and I/O deadlines |
| Native packet operation | Derived operation context; bounded separately from the long-lived connection |
| Presence cleanup, vote expiry and background delivery | Application worker lifetime, with a fresh bounded child per unit of work |
| CLI action | Command context derived from process signal handling, optionally bounded by an operation timeout |
| Shutdown | A fresh bounded context that remains usable after the run/signal context is canceled |

These lifetimes are recommendations for this backend. A session ID is identity/state, not a context lifetime. HTTP polling ends repeatedly while the user's authenticated session and queued delivery state continue. Likewise, a durable private message must remain queued after its sender's request ends.

### HTTP draining

A server request's context is canceled on connection closure, HTTP/2 request cancellation or when its handler returns. `Server.Shutdown` stops accepting new connections and waits for active HTTP connections to become idle; its deadline bounds that wait without forcibly closing the remaining active connections. The adapter must explicitly force closure if required. Shutdown does not manage hijacked connections. [HTTP API](https://pkg.go.dev/net/http)

For our proposed drain-first policy, do not make every active HTTP request a child of the immediately canceled SIGTERM context. Otherwise shutdown cancels in-flight requests before they can drain. Keep signal notification, request lifetime and shutdown budget distinct. The HTTP adapter should own its `BaseContext` policy and any forced closure. Bootstrap should only invoke the supplied lifecycle operations. This is a design inference from the HTTP/context contracts, not a requirement that every Go server must use the same shutdown policy.

### Cancellation is cooperative

Adding `ctx` parameters does not make arbitrary code interruptible. Queries must use the context, loops must observe it, and blocking I/O must support cancellation or deadlines. `sync.Mutex.Lock` blocks until the mutex is available and has no context argument. [Mutex API](https://pkg.go.dev/sync#Mutex.Lock)

For the current shared application, first recheck cancellation after acquiring its mutex and before beginning new I/O. That prevents some canceled operations from starting mutations, but it does not make lock acquisition interruptible. Reducing lock scope or introducing a cancellable admission mechanism is a separate concurrency change requiring delivery-order tests. Preserve the existing lock ordering, authorization checks and membership invariants; do not replace locks with polling merely to claim cancellation support.

### Durability and completion

Cancellation reports that a caller no longer wants to wait. It is not a general guarantee that no side effect occurred. The proposed chat contract should distinguish a transaction that has not committed from a message already accepted into durable storage. After acceptance, delivery belongs to application-owned work and stored pending deliveries. For modern JSON/keyed operations, a client that loses its response can retry using the same idempotency key and resolve the durable result. Original native message submissions supply no such key, so resubmission does not have that deduplication guarantee; durable pending delivery is a separate mechanism.

This is a recommendation based on our existing message/audience/idempotency transaction, not a claim of exactly-once network delivery. The transactional-outbox pattern retains delivery intent in the same transaction as the business state, and receivers may still encounter duplicates. [Transactional outbox guidance](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html) Avoid arbitrary goroutines detached from request cancellation as a substitute for the durable queue. Native delivery completion and cleanup also need an explicit bounded owner when their connection has already ended.

## Bootstrap and composition boundaries

The composition root is where concrete dependencies are selected and assembled, near the executable entrypoint. It can use supporting functions without relocating application policy into a generic mechanism. [Composition Root](https://blog.ploeh.dk/2011/07/28/CompositionRoot/)

Go's module guidance supports separate executable packages under `cmd` and shared implementation packages under `internal`. It does not prescribe a universal package named `bootstrap`. We are deliberately assigning that name the mechanism-only responsibility requested for this project. [Go module organization](https://go.dev/doc/modules/layout)

Proposed responsibilities:

| Package or component | Owns |
| --- | --- |
| `cmd/*` and nearby composition functions | Flags, config selection, choosing implementations, constructing dependencies, process exit policy |
| Optional shared construction helper near the commands | Repeated core wiring and its cleanup contract, only if meaningful duplication exists |
| SQLite adapter | Database resources, migrations, transaction handling, generated-query binding and storage-error mapping |
| HTTP/socket adapters | Their configuration validation, listeners, readiness, serving, draining, deadlines and expected transport errors |
| Application services | Use cases, shared session state and explicitly started application workers |
| `bootstrap` | Executing supplied startup/run/stop operations, startup rollback, failure propagation and bounded shutdown |

Bootstrap should have no predefined HTTP/socket/database factories, mode enumeration, application-wide config parameter, concrete application container or `GetService` lookup. It should not require every component to have a TCP address or listener. A worker, server and one-shot CLI action have different lifecycles; do not force all of them through a server descriptor. CLI may use constructors and deferred cleanup directly.

For `omni`, its composition code builds one service/repository graph and passes the same services to both transport constructors. Separate HTTP and socket processes still do not share live sessions merely because their database path matches. That limitation is independent of the bootstrap design.

Lifecycle behavior worth retaining or adding:

1. Complete validation before acquiring resources where possible.
2. Separate synchronous startup/readiness from long-running execution.
3. If startup fails, release only acquired resources in reverse dependency order.
4. Publish combined readiness only after both selected transports have successfully started.
5. Observe background failures and stop peer components deliberately.
6. Stop accepting work, drain transports and stop application workers before closing storage.
7. Use a fresh shutdown deadline and preserve both the original failure and cleanup errors.
8. Let each adapter normalize expected errors and implement forced closure; generic bootstrap has no HTTP error cases.

These are proposed acceptance requirements. They should be tested with arbitrary supplied tasks, not only HTTP/socket fixtures, to demonstrate that bootstrap is genuinely generic.

## Options that fit this backend

Dependency construction and lifecycle coordination are related but separate decisions.

| Option | Strengths here | Costs and limits | Assessment |
| --- | --- | --- | --- |
| Explicit constructors with a small lifecycle coordinator using standard contexts and optionally `errgroup` | Compile-time dependency checking, visible ownership, easy stepping through startup, no container in services | We own readiness, rollback and shutdown policy; `errgroup` alone does not supply them | Recommended for the current small construction graph |
| Explicit constructors with `oklog/run` | Established execute/interrupt grouping for heterogeneous blocking tasks | Construction, readiness, startup rollback and shutdown deadlines still need explicit design | Good alternative if actor-style execution fits our adapters better |
| Fx at the composition boundary | Providers, modules, dependency resolution and lifecycle hooks reduce repeated assembly | Runtime graph errors and framework concepts; constructor side effects and background-error handling still require care | Suitable if independently composed modules and workers become substantial, or declarative assembly is an explicit preference |

`errgroup.WithContext` cancels its derived context on the first non-nil task error or when `Wait` returns. A task returning nil does not itself cancel its siblings, and cancellation does not force a task to return. Therefore successful daemon exit policy and a separate shutdown context are still our responsibility. Pin a version compatible with the module's Go baseline before adopting it. [errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup)

The initial Go 1.25 compatibility concern is superseded: `x/sync` v0.24.0 requires Go 1.26, which the selected Go 1.27.3 toolchain satisfies. No `x/sync` dependency has been added as part of this research. [Versioned module requirement](https://raw.githubusercontent.com/golang/sync/v0.24.0/go.mod)

`oklog/run` groups an execute function with an interrupt function. When the first actor returns, the group interrupts all actors and waits for completion. It provides execution coordination without knowing the application's transports. The interrupt functions must actually unblock their actors; this is not a built-in deadline guarantee. [oklog/run](https://github.com/oklog/run)

Fx registers constructors during initialization and executes lifecycle startup hooks in registration order, shutdown hooks in reverse order. Its modules can remain thin wrappers around ordinary constructors. A service should not depend on the DI container or receive request contexts from it. Fx also needs deliberate handling of failures in background serving loops. [Fx lifecycle](https://uber-go.github.io/fx/lifecycle.html), [Fx modules](https://uber-go.github.io/fx/modules.html)

Our current constructor performs database work and starts the presence timer before returning. Simply placing it in `fx.Provide` would not resolve that ownership problem: initialization can acquire resources before lifecycle startup. Separate assembly from initialization/start, or explicitly retain cleanup on construction failure. Do not register both repository cleanup and the existing application `Close` as independent owners of the same resource.

Dig is a dependency-injection building block rather than an application lifecycle framework; its maintainers explicitly discourage treating it as a runtime service locator. It would leave us to build more surrounding machinery, so it is not a preferred direct dependency here. [Dig](https://github.com/uber-go/dig)

Google Wire demonstrates compile-time constructor wiring, but its official repository was archived on 2025-08-25 and states that it is no longer maintained. Do not adopt the original project as the default for new work here. A maintained fork would need its own assessment. [Wire project status](https://github.com/google/wire)

## Repository and transaction patterns

Keep sqlc output inside the database adapter. Application ports should expose domain inputs/results and operation contexts; they should not expose generated rows, `sql.DB`, `sql.Tx`, SQL strings or sqlc's `DBTX` interface.

Within the adapter, `DBTX` allows generated queries to bind to a database or transaction, and `Queries.WithTx` creates a transaction-bound query object. That adapter-level mechanism is already available. [sqlc 1.31.1 transactions](https://docs.sqlc.dev/en/v1.31.1/howto/transactions.html)

The proposed operation sequence is `BeginTx(ctx, ...)`, deferred rollback, all relevant sqlc calls on the transaction-bound queries with the same context, then commit. Do not mix `db` calls into an operation already using `tx`; they execute outside that transaction and may create inconsistent reads or deadlocks. [Go transaction guidance](https://go.dev/doc/database/execute-transactions)

`database/sql` documents that a transaction's begin context governs its lifetime and cancellation triggers rollback. Driver support still matters to the responsiveness of cancellation. Treat transaction errors and cancellation as explicit outcomes; preserve idempotency for clients whose acknowledgement was lost. [database/sql](https://pkg.go.dev/database/sql)

Two transaction-boundary options fit:

| Pattern | Suitable use | Recommendation |
| --- | --- | --- |
| Atomic semantic repository operation | Save message plus audience/idempotency record; credit/gift mutation; membership change with checks | Keep as the default. These boundaries already protect important invariants. |
| Application-owned transaction callback port with a narrowly scoped repository bundle | A real use case must atomically combine multiple repository capabilities | Introduce only when such a use case requires it. The implementation owns `sql.Tx`; the service receives domain-level capabilities valid only inside the callback. |

Do not place the active SQL transaction in context values, pass raw transaction handles to services, or automatically wrap every HTTP request in a transaction. Native packets, scheduled tasks and CLI actions also invoke use cases. Transaction boundaries should reflect atomic business effects, not the transport used to request them.

A transaction callback must supply methods already bound to that transaction. They must not call `Begin` again. Existing atomic methods on the ordinary store start their own transactions; calling those methods from an outer transaction can block on the single-connection pool. Reusing them requires extracting transaction-bound internal operations first.

The pinned go-sqlite3 v1.14.52 driver forwards query and transaction-start contexts, but its `BeginTx` implementation does not apply `driver.TxOptions`. Commit and rollback use background contexts internally. This does not establish a universal hard deadline for every SQLite call, lock wait or commit; cancellation also must not turn an already successful commit into a fabricated failure. The follow-up implementation should test the actual driver and our single-connection pool, including cancellation while waiting for a connection or writer lock. [Pinned context adapter](https://raw.githubusercontent.com/mattn/go-sqlite3/v1.14.52/sqlite3_sql.go), [pinned driver implementation](https://github.com/mattn/go-sqlite3/blob/v1.14.52/sqlite3.go)

## Other dependencies and API discipline

Use component-owned configuration values. Read environment/YAML/flags at the executable boundary, then pass the relevant settings into constructors. A chat service should not read environment variables or receive unrelated HTTP listener settings. Required dependencies should fail validation during construction, before serving traffic.

Logging is a long-lived collaborator; `*slog.Logger` is a reasonable concrete dependency without inventing a logging interface. Use context-aware logging calls when request metadata matters. Go's slog design permits handlers to inspect context information and deliberately omitted a standard logger-in-context API because of the hidden-dependency concern. [Structured logging with slog](https://go.dev/blog/slog)

Time is a test seam for vote expiry, cooldowns and sessions. A small `now func() time.Time` may suffice for policy calculations; timer ownership needs a lifecycle as well. Do not mechanically wrap every standard-library function. Go 1.25's `testing/synctest` can help deterministic timer/concurrency tests, but real network I/O and cgo are not durably blocking inside its test bubbles. SQLite integration tests therefore remain real integration tests. [Testing time](https://go.dev/blog/testing-time)

Keep cryptographic randomness appropriate for session/password-related operations. If a deterministic seam is needed in tests, inject a narrowly scoped generator there; do not replace production security-sensitive randomness with a predictable source. Ordinary pure helpers such as string normalization do not need interfaces merely to satisfy a dependency-injection aesthetic. [Google Go decisions](https://google.github.io/styleguide/go/decisions#cryptorand)

Preserve `context.Canceled` and `context.DeadlineExceeded` so callers can classify them with `errors.Is`. Map database-specific absence/conflict conditions to application-owned errors at the adapter boundary. Wrapping an implementation-specific error with `%w` makes it observable as part of the API, so error exposure must be deliberate. User-facing controllers still map errors to the original HTTP/packet response contracts. [Go error wrapping guidance](https://go.dev/blog/go1.13-errors)

Cancellation and repository failures should be classified before legacy wrappers replace them with generic text. The current `ServiceError` has no underlying cause. A future error design can preserve machine classification internally while retaining safe, historically compatible external messages. This does not require replacing all errors with a single large error framework.

## Proposed implementation sequence and acceptance evidence

The recommendation is explicit constructors plus a generic lifecycle mechanism. Fx remains the main alternative if declarative assembly is preferred. Choosing either does not change the need for operation contexts and proper ownership.

1. Agree the construction and lifecycle contracts. Inventory one owner for each database, application worker and listener. Keep `bootstrap` free of application and transport imports.
2. Separate the current constructor's value assembly, startup I/O and recurring worker startup. Give startup work an explicit context. Preserve existing seeding and schema behavior.
3. Migrate one complete request path through controller, service, repository port, adapter and sqlc. Add native/CLI operation contexts; do not stop after adding only service parameters.
4. Expand context propagation across the remaining repository capabilities, helper queries and migrations. Keep existing atomic operations and idempotency hashes unchanged.
5. Integrate transport-specific shutdown and readiness with generic lifecycle callbacks. Exercise HTTP/socket together against one shared service instance.
6. Address global-lock contention separately, using existing delivery/order tests as constraints. Do not combine a large concurrency redesign with mechanical context propagation.

Meaningful tests should establish:

- The original context deadline and cancellation reach a fake repository and real sqlc-backed query.
- Cancellation while waiting for a pooled connection or SQLite writer lock is handled with documented bounds for the pinned driver.
- A canceled or failed transaction cannot leave a partially saved message/audience or credit operation.
- A committed modern JSON message whose response was lost resolves through the same idempotency key and remains deliverable; native resubmission is tested under its separate contract.
- Cancellation after mutex admission is rechecked; tests accurately expose the remaining non-cancellable lock wait.
- A constructor does not start unowned background work; failed startup closes acquired resources exactly once.
- A second transport's bind failure rolls back the first; readiness is not published prematurely.
- A background serving failure is observed and causes the intended peer shutdown.
- Graceful HTTP drain uses a live shutdown context, then forced closure occurs when its budget expires.
- Native socket workers and timers stop before shared repository closure.
- CLI execution needs no transport lifecycle, and invalid flags/config produce no database side effects.
- Original-client packet ordering, cross-transport sessions, page behavior and delivery retry contracts remain unchanged.

The existing architecture guards enforce some package directions, but tests that only prohibit names such as `Mode` do not prove a generic mechanism. The stronger check is that a caller can register a non-network worker without changing bootstrap or importing application infrastructure into it.

## Retained evidence

The [source catalog](go-backend-sources.json) records primary references, their scope and the date checked. Organization-specific style guides are identified as such; project recommendations are not attributed to the Go language specification. The [repository audit](go-backend-audit.json) records the original inspected source snapshot and detailed context/transaction call sites. It remains unchanged as the pre-refactor baseline; implementation and validation are recorded separately.
