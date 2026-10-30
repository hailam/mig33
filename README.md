# mig33

Go compatibility backend for the recovered mig33 Java ME 4.62 client. Module: `github.com/hailam/mig33`.

The backend implements direct messages, public chat rooms, private group conversations, contacts, moderation, accounts and the historical embedded-page catalog. Recovered behavior is documented separately from inferred replacements. SMS, external email, calls and payments remain local mocks. The [Expo client](apps/mobile/README.md) implements the first original-protocol login and messaging slice; Classic v3 presentation remains deferred.

## Run locally

Requires Go 1.27.3 or newer and a C compiler for SQLite. The backend builds, runs and tests without Java or recovered applet files.

```sh
go run ./cmd/omni -env local
```

HTTP listens on `127.0.0.1:8080`, the native socket on `127.0.0.1:9119`, and persistent data lives in `data/chat.sqlite`. Test accounts include `hailam` / `hailam`, `alice` / `alice-demo` and `bob` / `bob-demo`. The local overlay enables report administration for `hailam`.

Follow [local applet setup](tools/applets/SETUP.md) to obtain and verify the two Git-ignored JARs and prepare the optional historical download resources, then open two independent clients at Login:

```sh
sh scripts/open-clients.sh
```

Use `sh scripts/open-clients.sh hailam bob` to log in automatically. The [applet tooling guide](tools/applets/README.md) explains prerequisites, capture commands and isolated acceptance runs.

## Project layout

| Location | Purpose |
| --- | --- |
| `cmd/{http,socket,omni,cli}` | Executable entrypoints |
| `cmd/internal/{command,wiring}` | Plumbing shared only by entrypoints |
| `internal/` | Capability services, repository ports, adapters, controllers and generic bootstrap |
| [apps/mobile/](apps/mobile/README.md) | Expo app and app-local original HTTP/TCP client |
| `api/`, `db/`, `configs/` | Original HTTP/message contracts, sqlc queries/migrations and YAML configuration |
| [docs/](docs/README.md) | Backend behavior, architecture and development guides |
| [research/](research/README.md) | Applet references, architecture studies and acceptance reports |
| [tools/applets/](tools/applets/README.md) | Reusable analysis, emulator and compatibility-test tooling |
| `scripts/` | Application generation and client-launch commands |
| `data/`, `bin/` | Ignored local state, generated output and executables |

Research documents, small catalogs, provenance metadata and tooling source belong in Git. Downloaded JARs/JADs, recovered/decompiled source, recovered graphics/language/theme resources, screenshots, capture archives, databases and generated classes remain local and are ignored. Optional Java source under `tools/applets/` provides client launchers, compatibility tests and emulator shims; it is not a backend runtime dependency. See the [research storage policy](research/README.md#tracked-research-and-local-artifacts).

The backend reads optional historical resources from `data/applets/resources/v4`, configured through `resources.directory` or `MIG33_RESOURCES_DIRECTORY`. Restore them with `python3 tools/applets/scripts/prepare_runtime_assets.py` after downloading the verified v4 JAR. Without them, core login and text chat remain available; theme, English-pack, emoticon and placeholder provider-icon downloads report unavailable resources. Go tests use authored synthetic fixtures.

## Development

```sh
sh scripts/format-go.sh --install
sh scripts/format-go.sh --write
sh scripts/format-go.sh --check
go test -race ./...
go vet ./...
sh scripts/generate-openapi.sh
sh scripts/generate-contracts.sh --check
sh scripts/check-go-vulnerabilities.sh
sh scripts/generate-sqlc.sh
uv run --no-project python scripts/export-sqlc-schema.py --check
```

The [development guide](docs/development.md) explains formatting, import grouping and readability conventions. Python tooling runs through uv. CI checks handwritten Go formatting, vet, race tests and reachable Go vulnerabilities; generated source remains owned by its generator. A separate mobile job checks the client and both live transports against an isolated Go server.

The [contract guide](api/README.txt) covers reviewed 4.62 definitions, generated AsyncAPI and TypeScript, original HTTP/TCP compatibility, and fixture validation. Its optional Node tooling uses scoped mise versions and pnpm with a three-day dependency release delay; advisory-backed exceptions require an exact version and documented reason.

The [backend architecture](docs/backend.md#composition-routing-and-persistence) describes capability ownership, typed adapters, transactions and lifecycle handling. [Configuration](configs/README.md) documents YAML overlays and environment overrides. [Reconstruction notes](docs/reconstruction-notes.md) retain the earlier feature milestones and evidence links.
