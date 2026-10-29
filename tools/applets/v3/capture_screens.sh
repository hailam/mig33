#!/bin/sh
set -eu
cd "$(dirname "$0")/../../.."
# JAVA_BIN/JAVAC_BIN may select another installed JDK without changing user settings.
JAVA_BIN=${JAVA_BIN:-/opt/homebrew/opt/openjdk@21/bin/java}
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
mkdir -p data/applets/build/v3/classes
"$JAVAC_BIN" -cp tools/applets/bin/microemulator-2.0.4.jar -d data/applets/build/v3/classes \
 tools/applets/v3/capture-src/javax/microedition/io/Connector.java \
 tools/applets/v3/capture-src/javax/microedition/media/control/VideoControl.java \
 tools/applets/v3/capture-src/Capture.java
capture_home=$(mktemp -d "${TMPDIR:-/tmp}/mig33-capture.XXXXXX")
trap 'rmdir "$capture_home" 2>/dev/null || true' EXIT
"$JAVA_BIN" -Djava.awt.headless=true -Duser.home="$capture_home" \
 -cp data/applets/build/v3/classes:tools/applets/bin/microemulator-2.0.4.jar:research/applets/v3/artifacts/original/mig33-3.08.jar Capture
python3 tools/applets/v3/capture_gallery.py
