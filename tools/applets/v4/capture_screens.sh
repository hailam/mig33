#!/bin/sh
set -eu
cd "$(dirname "$0")/../../.."
JAVA_BIN=${JAVA_BIN:-/opt/homebrew/opt/openjdk@21/bin/java}
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
mkdir -p data/applets/build/v4/capture-classes
# Only original resource bytes enter java.base, never application classes.
uv run --no-project python - <<'PY'
from zipfile import ZipFile
with ZipFile('research/applets/v4/artifacts/original/mig33-4.62.jar') as src, ZipFile('data/applets/build/v4/resource-bridge.jar','w') as dst:
    for name in ['default_v2','en-US']:
        dst.writestr(name,src.read(name))
PY
"$JAVAC_BIN" -cp tools/applets/bin/microemulator-2.0.4.jar -d data/applets/build/v4/capture-classes \
 tools/applets/v4/shims/javax/microedition/io/Connector.java \
 tools/applets/v4/shims/javax/microedition/media/control/VideoControl.java \
 tools/applets/v4/shims/javax/microedition/lcdui/Displayable.java \
 tools/applets/v4/V4Capture.java
capture_home=$(mktemp -d "${TMPDIR:-/tmp}/mig33-v4-capture.XXXXXX")
trap 'rmdir "$capture_home" 2>/dev/null || true' EXIT
"$JAVA_BIN" --patch-module java.base=data/applets/build/v4/resource-bridge.jar \
 -Djava.awt.headless=true -Duser.home="$capture_home" \
 -cp data/applets/build/v4/capture-classes:tools/applets/bin/microemulator-2.0.4.jar:research/applets/v4/artifacts/original/mig33-4.62.jar V4Capture
uv run --no-project python tools/applets/v4/capture_gallery.py
