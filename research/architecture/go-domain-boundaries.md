# Go command packages and domain boundaries

Research date: 2026-10-29. Decision status: capability architecture implemented and verified with full race/vet, generator/schema checks and all 17 original-client suites. The follow-up [ports/adapters migration study](go-hexagonal-migration.md) records the complete accepted scope. See the [current backend architecture](../../docs/backend.md#composition-routing-and-persistence), [implementation record](evidence/go-capability-implementation.json) and [test migration inventory](evidence/capability-test-migration.json).

The implementation keeps `cmd/internal` for command plumbing and composition, moves business types and operations into capability packages, and gives one realtime coordinator ownership of ephemeral state and delivery ordering. The former `services.Application`, aggregate ports and `internal/domain` aliases have been removed. Native codecs and historical page adapters now own presentation.

The analysis below preserves the original audit and alternatives. Its old package names, counts and suggested intermediate facades describe that snapshot, not the final topology. The retained audit JSON and source hashes are unchanged. Current links lead to equivalent owners after extraction. The [retired source snapshot](evidence/capability-before-source.tar.gz) retains the exact deleted packages and tests; its SHA-256 is `7ce316122bdbe1de35e5e74ee2b9a10f67f92c7241dba1590b863228fafe6a5b`.

## Command support under cmd

`cmd` is a directory convention. `internal` has an enforced import restriction: only packages beneath the directory containing that `internal` directory can import its packages. Consequently, `cmd/internal/wiring` is available to our commands, while `internal/services` cannot import it. The [official internal-package rule](https://go.dev/doc/go1.4#internalpackages) and [current loader implementation](https://go.dev/src/cmd/go/internal/load/pkg.go) establish this behavior. The Go toolchain itself uses [cmd/internal](https://go.dev/src/cmd/internal/).

Our `cmd/internal/command` handles flags, configuration and process signals. `cmd/internal/wiring` constructs the core and returns its cleanup function. At the audit snapshot their three production files totaled 172 physical lines; only the four executable packages imported them. These helpers are ordinary packages beside the executable directories, not code inside a `package main` that other packages import.

| Import | Allowed by the internal boundary |
| --- | --- |
| `cmd/http` → `cmd/internal/wiring` | Yes |
| `cmd/cli` → `cmd/internal/wiring` | Yes |
| `cmd/internal/wiring` → `internal/services` | Yes |
| `internal/services` → `cmd/internal/wiring` | No |
| `tests/integration` → `cmd/internal/wiring` | No |

An [isolated Go 1.27.3 compile probe](evidence/command-package-boundary.json) confirmed both the allowed and rejected cases. The integration tests for shared assembly currently live under `cmd/internal/wiring`, where that visibility is appropriate.

There are three reasonable placement options:

| Placement | Advantage | Tradeoff and fit |
| --- | --- | --- |
| **Keep command helpers and wiring in `cmd/internal`** | Matches current consumers and prevents application code from importing command assembly. | Recommended now. Keep business rules out of these helpers. |
| Keep command helpers there; move wiring to `internal/composition` | Other repository packages can reuse assembly. | Useful when an actual caller outside `cmd` needs it. Broadens permitted imports and needs dependency guards. |
| Wire everything independently in each executable | Every graph is visible in its executable. | Reasonable for tiny or divergent commands; repeats initialization, rollback and cleanup here. |

The [official layout guide](https://go.dev/doc/modules/layout) describes common arrangements, including root-level `internal`, without prescribing a universal layout. Moving helpers solely to make `cmd` contain only executable directories offers little benefit here. Putting application assembly back into `bootstrap` would conflict with the selected lifecycle-only responsibility.

## What the pre-refactor service package contained

The [AST audit](go-domain-boundaries-audit.json) excludes test files from production counts and retains source positions and hashes. Physical line counts include comments and blank lines.

| Area | Production files | Physical lines |
| --- | ---: | ---: |
| `internal/services` | 41 | 8,229 |
| `internal/domain` | 20 | 571 |
| `internal/ports` | 5 | 282 |
| SQLite repositories | 23 | 4,005 |
| Controllers | 9 | 883 |

These are not 41 independent services. The service files form one package, with **182 methods on one `Application`, including 38 exported methods**. That application owns accounts, sessions, delivery state, room ballots and operator settings, and receives the composite repository interface containing 124 operations. Repository access appears in 33 service files, account access in 23, and session access in 19. The audit identifies 238 syntactic cross-file calls between application methods; these are source relationships, not measured runtime traffic.

`internal/domain` already contains shared models, request/result types and some pure rules. It provides a useful dependency boundary against infrastructure, but does not give separate business capabilities ownership of their models and behavior. Services also import protocol types in 24 files and page rendering in 19, so extracting capabilities must account for historical presentation code.

The case for modularization is the mix of responsibilities and broad dependencies. There is no meaningful file-count threshold. Go's [package-naming guidance](https://go.dev/blog/package-names) emphasizes coherent purpose and usable APIs. Dave Cheney's [package-design advice](https://dave.cheney.net/practical-go/presentations/qcon-china.html#_consider_fewer_larger_packages) provides a useful counterweight: excessive splitting can turn private implementation details into an unnecessarily large exported surface.

## Domains and packages mean different things

A business domain describes the problem being modeled. A capability package gives related behavior, types and dependencies an owner. A DDD bounded context additionally defines where a model and its vocabulary have a consistent meaning and how it relates to other models. A Go package is not automatically a bounded context. [Bounded Context](https://martinfowler.com/bliki/BoundedContext.html)

In this backend, private chat groups, contact-list groups, public rooms and community Groups are different concepts. Private groups have conversation membership; contact groups organize a roster; rooms have live participants and kick rules; community Groups have roles, forums and room links. A generic `GroupService` spanning all four would obscure those distinctions.

Logical modules can share a process and database. A Go module means a `go.mod` boundary; it is unrelated to calling a business capability a module. Introducing capability packages does not require multiple Go modules, databases or deployed services. Hexagonal architecture concerns application boundaries and adapters, rather than a mandatory folder hierarchy. [Original ports-and-adapters article](https://alistair.cockburn.us/hexagonal-architecture)

## Three practical architecture options

| Option | Shape | Benefit | Cost and suitability |
| --- | --- | --- | --- |
| **A. Smaller services within the current layers** | Keep `services`, `domain`, `ports`, and adapters; introduce focused concrete service types. | Small migration steps; narrow constructor dependencies can be introduced immediately. | Same-package private access remains unrestricted, so boundaries rely heavily on discipline. Useful as the first step. |
| **B. Capability packages in a modular monolith** | Packages such as `accounts`, `contacts`, `chat`, `rooms`, `community`, and `commerce` own their use cases, types and required ports. | Package APIs make ownership visible; services become independently understandable and testable. | Requires deliberate contracts for shared transactions, sessions and delivery. **Recommended target.** |
| **C. Explicit DDD bounded contexts** | Selected contexts have richer domain models, application orchestration, mappings and possibly context-local adapters. | Stronger isolation when meanings, rules or ownership genuinely diverge. | More mappings and integration contracts. Apply selectively once actual context boundaries justify the cost. |

A and B can form successive stages of one change. C is an additional modeling commitment, not a synonym for adding more directories. A larger package remains appropriate when splitting it would expose tightly coupled internal state.

Microservices are a separate deployment decision. Current shared sessions, ordering gates and atomic operations provide concrete reasons to keep one application graph. Fowler's [monolith-first discussion](https://martinfowler.com/bliki/MonolithFirst.html) describes the cost of premature distribution; it is architectural experience and advice, not a guarantee that every modular monolith can later be split cheaply.

## Proposed capability ownership

These are candidate boundaries, not instructions to create every directory at once.

| Capability | Owns | Boundary requiring care |
| --- | --- | --- |
| Accounts | Registration, credentials, recovery, activation, profile/preferences | Committed credential cache and session invalidation; initial roster creation. |
| Contacts | Roster, roster groups, friend relationships and personal block rules | Blocking must coordinate with in-flight and queued delivery. |
| Chat | DMs, private chat groups, message audience, history, idempotency and read positions | Shared realtime sessions, ordering and delivery reservations. |
| Rooms | Directory/settings, capacity, moderation and voting policies | Live participants and community-linked room permissions. |
| Community | Community Groups, roles, forums and room associations | Room ownership and authorization when linking a room. |
| Commerce | Test-credit ledger, inventory, purchases, gifts and entitlement | Gift purchase includes durable message creation. |
| Catalog | Public product/game/emoticon metadata | Reuse the existing `internal/catalog`; public metadata differs from ownership and purchasing. |
| Media and review capabilities | Photo ownership/access; report and support review respectively | Define them from their use cases, rather than reproducing the mixed `ServicesRepository` bucket. |

Mock SMS/email/payment/IM integrations remain adapters or small recorded-request capabilities. Their page count does not justify one business context per provider. Historical packet/page translation is a compatibility boundary; it should translate into the same typed use cases as the modern API.

A compact target might look like this:

```text
cmd/
  http/  socket/  omni/  cli/
  internal/command/           flags, config, process policy
  internal/wiring/            concrete graph and cleanup ownership
internal/
  bootstrap/                  generic lifecycle only
  controllers/                one package, generated HTTP interface
  accounts/  contacts/  chat/  rooms/  community/  commerce/
  catalog/                    existing static catalog, expanded as needed
  services/                   shrinking compatibility/orchestration facade
  adapters/
    httpapi/  httptransport/  socket/
    sqlite/repositories/  sqlite/sqlc/  sqlite/migrations/
```

The existing OpenAPI → controller → use case → repository flow remains useful. Controllers can receive a typed dependencies struct containing the specific services their handlers require while continuing to implement the single generated interface. It should not become a name-based service lookup mechanism.

A capability can initially have `service.go`, `types.go`, `repository.go`, and focused policy files in one package. Add subpackages when they provide an actual visibility or dependency boundary. Interfaces such as `rooms.Repository` are still ports; they do not require a central package named `ports`. Go's [interface guidance](https://go.dev/wiki/CodeReviewComments#interfaces) generally places contracts with consumers. Move existing central types/contracts gradually, retaining only genuinely shared vocabulary and errors centrally.

## Transactions and realtime ownership constrain the split

The following invariants are observable in current code:

| Workflow | Must remain coordinated |
| --- | --- |
| [Registration](../../internal/adapters/sqlite/repositories/identity.go) | Account, initial roster and Friends group commit together. |
| [Message acceptance](../../internal/adapters/sqlite/repositories/messages.go) | Relevant membership/block/photo checks, message, immutable audience and idempotency key. |
| [Gift purchase](../../internal/adapters/sqlite/repositories/commerce.go) | Privacy checks, debit/ledger, inventory, gift note and durable gift message. |
| [Community room linking](../../internal/adapters/sqlite/repositories/community.go) | Community permissions and room ownership checks with the association change. |
| [Personal blocking](../../internal/realtime/blocking.go) | Delivery gate, persisted block, and cancellation of queued traffic retain their ordering. |

Keep specialized atomic repository operations during the first extraction. A logical capability boundary does not require its repository to make an independent commit for each table. If transaction composition becomes necessary later, use an explicit workflow-owned transaction boundary whose adapter supplies transaction-bound capabilities; avoid exposing `*sql.Tx` or generated rows through business ports. Go documents the risks of mixing transactional and nontransactional operations. [Go transaction guidance](https://go.dev/doc/database/execute-transactions)

One SQLite adapter can implement several narrow capability interfaces and use the existing generated query package. sqlc's [transaction binding](https://docs.sqlc.dev/en/v1.31.1/howto/transactions.html) supports shared transaction-bound queries. Its [configuration](https://docs.sqlc.dev/en/v1.31.1/reference/config.html) also permits separate query sets/output packages, but that is a later storage-maintenance choice. Splitting generated packages now would add mapping work without establishing business ownership.

Keep one owner of session maps, presence, delivery gates and live room participation initially. Extracting room rules does not require moving live room state at the same time. Existing separate `http` and `socket` processes cannot share those maps simply by pointing at the same SQLite file; `omni` provides the shared graph.

Use direct typed calls for required synchronous workflows. An in-memory event bus cannot replace a durable gift/message transaction. If asynchronous side effects later become useful, specify persistence, retries and delivery semantics explicitly. The [Microsoft domain-event reference](https://learn.microsoft.com/en-us/dotnet/architecture/microservices/microservice-ddd-cqrs-patterns/domain-events-design-implementation) distinguishes transactional dispatch from eventual consistency; its .NET examples are conceptual guidance here, not a Go implementation prescription.

## Original extraction sequence considered

1. Record capability owners and invariant boundaries. Preserve the source audit and existing native acceptance as the baseline.
2. Extract a small concrete service without moving shared state. The public `Catalog` use case needs the configured origin and static metadata, making it a useful pilot using the existing catalog assets. Report-review/read operations offer another limited slice.
3. Give that service only its actual collaborators and types. Retain the compatibility facade while controllers/native handlers start delegating to it.
4. Move a cohesive capability into its package once its public API is small. Reject dependencies on `*services.Application`, raw session maps, or the 124-operation repository aggregate.
5. Extract account, commerce and community workflows with their current atomic persistence contracts. Separate pure room/vote policy before changing ownership of live participants or delivery locks.
6. Move packet decoding and historical rendering toward compatibility adapters using typed results. Preserve original URLs, opcode behavior, error ordering, message serialization and idempotency hashes; `domain.MessageInput` field order currently participates in its hash.
7. Enforce import directions and test the actual boundary: authorization, rollback, post-commit cache updates, block/write ordering and same-session delivery retries. Run relevant original-client suites when behavior or transport wiring changes.

Each extraction should reduce the surface that a caller can access without requiring a larger shared interface or a new import cycle. Keep the current lock behavior during initial package moves; changing concurrency ownership deserves its own design and tests.

The [source catalog](go-domain-boundaries-sources.json), [repository audit](go-domain-boundaries-audit.json), and [command boundary evidence](evidence/command-package-boundary.json) retain the supporting research. The earlier [dependency and lifecycle research](go-backend-architecture.md) remains the reference for the already implemented bootstrap/context design.
