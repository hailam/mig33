# Applet analysis and test tools

Reusable source belongs here; recovered inputs and written findings live under [research/applets](../../research/applets/README.md). Java files are optional client launchers, tests and emulator shims. They are not needed to build or run the Go backend.

For a fresh checkout, start with [local applet setup](SETUP.md): verified download URLs, exact filenames/checksums, prerequisites and the two-client smoke test. It needs only the v4.62 publisher JAR and MicroEmulator for normal client testing.

| Location | Purpose |
| --- | --- |
| `bin/` | Pinned MicroEmulator/decompiler JARs, source metadata and hashes |
| `scripts/` | Inventory, extraction, cross-version comparison and reference validation |
| `v3/` | Offline original-bytecode screenshot harness |
| `v4/` | v4 capture, interactive desktop launcher, shims and native integration suites |

JARs are local and ignored. Their required versions and hashes are in [bin/manifest.json](bin/manifest.json); no binaries are automatically downloaded. Java compilation and resource bridges go to `data/applets/build/`. Fresh integration output goes to `data/applets/integration/`; interactive windows use `data/desktop/`.

Prepare the backend's optional historical downloads from the verified local v4 JAR:

```sh
uv run --no-project python tools/applets/scripts/prepare_runtime_assets.py
```

This verifies the JAR and all four extracted resources before writing `data/applets/resources/v4/`. It requires Python only, does not download anything, and leaves the original JAR unchanged. These files stay ignored; a clean checkout can build and run core services without them. The [setup guide](SETUP.md#prepare-optional-backend-resources) describes the resource configuration and unavailable-download behavior.

For the [v4 protocol source audit](../../research/applets/v4/docs/protocol-catalog-audit.md), the original v4 JAR, MicroEmulator, Python, and a JDK are sufficient; decompiled sources are unnecessary:

```sh
uv run --no-project python tools/applets/v4/verify-protocol-evidence.py
uv run --no-project python tools/applets/v4/audit-outgoing.py
uv run --no-project python tools/applets/v4/audit-incoming.py
uv run --no-project python tools/applets/v4/audit-transports.py --write-catalog
```

These commands reproduce disassembly, check all outgoing/incoming catalog entries and endpoint references, and run isolated original-class probes. They do not start a MIDlet or connect to a service. The audit scripts write reviewed result JSON; raw dumps, compiled classes, and logs remain under ignored `artifacts/bytecode` and `data/protocol-audit` directories. See the audit report for scope and limits.

The following research-generation commands require the complete extraction/decompilation corpus. Restore those inputs before regenerating maintained indexes; normal client setup uses the existing indexes. From the repository root:

```sh
uv run --no-project python tools/applets/scripts/inventory.py v3
uv run --no-project python tools/applets/scripts/inventory.py v4
uv run --no-project python tools/applets/scripts/extract_v4_theme.py
uv run --no-project python tools/applets/scripts/build_v4_endpoints.py
uv run --no-project python tools/applets/scripts/build_comparison.py
uv run --no-project python tools/applets/scripts/validate_reference.py
uv run --no-project python tools/applets/v3/protocol-codec.py --self-test
uv run --no-project python tools/applets/v4/protocol-codec.py --self-test
```

Original offline screen capture:

```sh
sh tools/applets/v3/capture_screens.sh
sh tools/applets/v4/capture_screens.sh
```

Interactive clients require a running local backend and a graphical desktop:

```sh
sh scripts/open-client.sh
sh scripts/open-clients.sh
```

For isolated backend acceptance, choose a new output directory:

```sh
uv run --no-project python tools/applets/v4/integration/isolated_acceptance.py --evidence data/applets/validation/manual-run
```

The driver builds the backend, uses separate ports and databases, prepares verified runtime resources in its temporary directory, and retains its binary/source/harness evidence plus resource hashes. Existing documented room-name and presence-helper corrections apply only to isolated harness copies. The publisher JAR is unchanged. See the [acceptance guide](../../research/acceptance/v4/README.md) for individual suites and prior results.
