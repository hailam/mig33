# Restore the local applet runtime

The current backend targets **mig33 4.62, build `.000`, 24×24 keypad variant**. A fresh checkout needs two local downloads to open that original client or run the two-client compatibility test: its publisher JAR and MicroEmulator 2.0.4. The launchers load the JAR directly; a JAD descriptor is unnecessary for this workflow.

Go builds and unit tests use authored synthetic fixtures and do not require these downloads, Java, or recovered applet resources. The running backend loads optional historical downloads from a local directory. Downloaded binaries, extracted resources and generated output are covered by the root [.gitignore](../../.gitignore).

## Prerequisites

Run the commands below from the repository root. Have Python 3.9+, curl, a POSIX shell and a JDK providing both `java` and `javac`. Running the backend or isolated acceptance also requires Go 1.27.3+ and a C compiler for SQLite; see the [project quick start](../../README.md#run-locally). The [fresh-setup check](setup-validation.json) passed with Go 1.27.3, OpenJDK 26.0.1 and Python 3.14.6.

The Java scripts default to `/opt/homebrew/opt/openjdk@21/bin/`. On the verified machine that symlink actually resolves to OpenJDK 26.0.1, so the directory name is not a version check. For another installation, set both executable paths and inspect their versions:

```sh
export JAVA_BIN=/absolute/path/to/jdk/bin/java
export JAVAC_BIN=/absolute/path/to/jdk/bin/javac
"$JAVA_BIN" -version
"$JAVAC_BIN" -version
```

The graphical launcher is tested on macOS and currently includes the macOS-specific `-Xdock:name` option. The headless compatibility test below does not use that option.

## Download sources and expected files

| File | Download source | Required local destination | Bytes |
| --- | --- | --- | ---: |
| mig33 4.62 keypad | [Archived publisher JAR, 27 April 2011](https://web.archive.org/web/20110427070316id_/http://www.mig33.com:80/wap2/v4_60/24x24/mig33v46.jar) | `research/applets/v4/artifacts/original/mig33-4.62.jar` | 190378 |
| MicroEmulator 2.0.4 | [Maven Central JAR](https://repo.maven.apache.org/maven2/org/microemu/microemulator/2.0.4/microemulator-2.0.4.jar) | `tools/applets/bin/microemulator-2.0.4.jar` | 643996 |

Both URLs were downloaded and hash-verified again on **2026-10-29**; see the [download check record](download-checks.json). The historical publisher filename is `mig33v46.jar`, but its manifest identifies version **4.62**. Save it under the local name above, which the scripts expect. The touch beta is a different payload.

The old `kemongers.jw.lt` mirror URLs returned HTML download pages during this check, despite HTTP 200 responses. Earlier research retrieved matching binaries there, but the bare URLs currently cannot be treated as direct JAR downloads. Use the archived publisher URL and verify the bytes. The [v4 provenance notes](../../research/applets/v4/docs/provenance.md) retain the historical publisher and mirror comparison.

## Download and verify

These commands download into temporary `.download` files. The Python check verifies both files before moving them into the locations used by the launchers.

```sh
mkdir -p research/applets/v4/artifacts/original tools/applets/bin

curl --fail --location --retry 2 --connect-timeout 15 --max-time 120 \
  'https://web.archive.org/web/20110427070316id_/http://www.mig33.com:80/wap2/v4_60/24x24/mig33v46.jar' \
  --output research/applets/v4/artifacts/original/mig33-4.62.jar.download

curl --fail --location --retry 2 --connect-timeout 15 --max-time 120 \
  'https://repo.maven.apache.org/maven2/org/microemu/microemulator/2.0.4/microemulator-2.0.4.jar' \
  --output tools/applets/bin/microemulator-2.0.4.jar.download

python3 - <<'PY'
from hashlib import sha256
from pathlib import Path

files = [
    (
        "research/applets/v4/artifacts/original/mig33-4.62.jar",
        190378,
        "dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c",
    ),
    (
        "tools/applets/bin/microemulator-2.0.4.jar",
        643996,
        "dbd5f3eb8365d3e839d6a203149e0e3776fc1a0585e16ac1fc23f76c9fcae1c6",
    ),
]
for name, size, digest in files:
    candidate = Path(name + ".download")
    data = candidate.read_bytes()
    if len(data) != size or sha256(data).hexdigest() != digest:
        raise SystemExit(f"Verification failed: {candidate}; do not use this download")
for name, _, _ in files:
    Path(name + ".download").replace(name)
    print(f"Verified and installed: {name}")
PY
```

If an archive request is temporarily unavailable, retry it later or obtain the same file through the archive's browser interface. Keep the required filename, size and SHA-256; an HTML response or a different client build must not replace the verified JAR.

## Prepare optional backend resources

After downloading the v4 JAR, run:

```sh
python3 tools/applets/scripts/prepare_runtime_assets.py
```

The script verifies the publisher JAR's size and SHA-256, extracts and verifies all four resources, then installs them under the ignored `data/applets/resources/v4/` directory. It never downloads or executes Java. `sprites.png` is the exact contents of theme field 129; the other three files are exact JAR entries. The original JAR is unchanged.

| File | Server use | Bytes |
| --- | --- | ---: |
| `default_v2` | Original theme reset and local blue/orange theme variants | 23718 |
| `en-US` | English language-pack download | 10411 |
| `sprites.png` | Reconstructed downloadable emoticons and gift placeholders | 9226 |
| `default-avatar.png` | Placeholder icons for mocked external IM providers | 595 |

The default `resources.directory` is `data/applets/resources/v4`, relative to the backend's working directory. Override it through YAML or `MIG33_RESOURCES_DIRECTORY`; the extractor accepts `--output /absolute/path` and `--jar /path/to/verified.jar`. Keep custom resource directories outside tracked source. Prepare resources before starting the backend; restart it after replacing cached sprite content.

A missing resource directory does not prevent the backend from starting. Core login, direct messages and chat rooms work without it; resource-dependent downloads return an unavailable response. The original client already bundles its default theme and English text. A saved theme selection or downloadable emoticon can still request a server resource during login or chat.

## Open the client

Start the local backend in one terminal:

```sh
go run ./cmd/omni -env local
```

In another terminal, from the same repository root:

```sh
sh scripts/open-clients.sh
```

This opens two signed-out windows. Use `hailam` / `hailam` and `bob` / `bob-demo`, or run `sh scripts/open-clients.sh hailam bob` to log in automatically. These launchers configure the original applet for the local backend.

The launcher builds Java classes and the `default_v2`/`en-US` resource bridge under `data/applets/build/`. No decompiler installation or Go code generation is needed for this setup.

## Run an isolated smoke test

The following command builds its own backend, creates a fresh database and uses separate listeners on ports 60080/61080. The evidence directory must not already exist; choose a new name for later runs.

```sh
python3 tools/applets/v4/integration/isolated_acceptance.py \
  --suites pair --evidence data/applets/validation/first-pair
```

The two downloads above are sufficient for this TCP/HTTP login, chat and page test. The isolated driver prepares its own verified runtime resources under its temporary backend directory and records their hashes with the evidence. See the [acceptance guide](../../research/acceptance/v4/README.md) for the recorded assertions. [setup-validation.json](setup-validation.json) records the earlier setup run, before resources were moved out of the Go build.

## Additional input for all 17 compatibility suites

After preparing the runtime resources, the media suite also needs the original `default-avatar.png` at its research fixture path. Copy the verified extraction:

```sh
python3 - <<'PY'
from pathlib import Path

target = Path("research/applets/v4/artifacts/extracted/default-avatar.png")
target.parent.mkdir(parents=True, exist_ok=True)
target.write_bytes(Path("data/applets/resources/v4/default-avatar.png").read_bytes())
PY

python3 tools/applets/v4/integration/isolated_acceptance.py \
  --evidence data/applets/validation/first-full-run
```

The pages suite uses the maintained `research/applets/v4/docs/data/endpoints-classified.json` already included with the source. Existing suite reports live under [research/acceptance/v4](../../research/acceptance/v4/README.md); large captured runs remain local.

## Optional v3 and full research restoration

For v3 analysis, download the [archived publisher v3.08 JAR](https://web.archive.org/web/20090417052259id_/http://wap.mig33.com:80/v3_08/mig33v3.jar) to `research/applets/v3/artifacts/original/mig33-3.08.jar`, using the same temporary-download and verification procedure. Its size is **115314 bytes** and its SHA-256 is:

```text
2b416199dca5f6593eefe8fa1c9b84ec4be6cdc36f68bcf17e1a94193bfd2934
```

That source also matched its recorded hash on 2026-10-29. The [v3 provenance notes](../../research/applets/v3/docs/provenance.md) include its optional archived JAD and original metadata.

`validate_reference.py` checks the full research corpus: both applets, extracted resources, decompiler/disassembly output, original screen captures, historical web images and all four tool JARs. Restoring the client runtime does not restore that entire corpus. Use the [research methodology](../../research/applets/shared/methodology.md) and [tool manifest](bin/manifest.json) for those additional inputs and download URLs.

Run inventory generators only after restoring their complete extraction/decompilation inputs; incomplete inputs can otherwise replace maintained indexes with incomplete output. Normal backend/client testing uses the existing indexes.
