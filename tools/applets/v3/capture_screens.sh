#!/bin/sh
set -eu
cd "$(dirname "$0")/../../.."
# JAVA_BIN/JAVAC_BIN may select another installed JDK without changing user settings.
JAVA_BIN=${JAVA_BIN:-/opt/homebrew/opt/openjdk@21/bin/java}
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
uv run --no-project python - <<'PY'
from hashlib import sha256
from pathlib import Path

for name, digest in [
    ('research/applets/v3/artifacts/original/mig33-3.08.jar', '2b416199dca5f6593eefe8fa1c9b84ec4be6cdc36f68bcf17e1a94193bfd2934'),
    ('tools/applets/bin/microemulator-2.0.4.jar', 'dbd5f3eb8365d3e839d6a203149e0e3776fc1a0585e16ac1fc23f76c9fcae1c6'),
]:
    if sha256(Path(name).read_bytes()).hexdigest() != digest:
        raise SystemExit(f'Unexpected capture input: {name}')
PY
mkdir -p data/applets/build/v3/classes
"$JAVAC_BIN" -cp tools/applets/bin/microemulator-2.0.4.jar -d data/applets/build/v3/classes \
 tools/applets/v3/capture-src/javax/microedition/io/Connector.java \
 tools/applets/v3/capture-src/javax/microedition/media/control/VideoControl.java \
 tools/applets/v3/capture-src/Capture.java
capture_home=$(mktemp -d "${TMPDIR:-/tmp}/mig33-capture.XXXXXX")
trap 'rmdir "$capture_home" 2>/dev/null || true' EXIT
"$JAVA_BIN" -Djava.awt.headless=true -Duser.home="$capture_home" \
 -cp data/applets/build/v3/classes:tools/applets/bin/microemulator-2.0.4.jar:research/applets/v3/artifacts/original/mig33-3.08.jar Capture
uv run --no-project python tools/applets/v3/capture_gallery.py
