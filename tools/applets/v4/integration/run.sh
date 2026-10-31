#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
JAVA_BIN=${JAVA_BIN:-/opt/homebrew/opt/openjdk@21/bin/java}
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
mkdir -p data/applets/build/integration/classes data/applets/integration/out
if [ "${INTEGRATION_SKIP_BUILD:-0}" != 1 ]; then
uv run --no-project python - <<'PY'
import hashlib
from zipfile import ZipFile
p='research/applets/v4/artifacts/original/mig33-4.62.jar'
assert hashlib.sha256(open(p,'rb').read()).hexdigest()=='dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c', 'Original JAR changed'
with ZipFile(p) as src, ZipFile('data/applets/build/integration/resource-bridge.jar','w') as dst:
    for n in ['default_v2','en-US']: dst.writestr(n,src.read(n))
PY
"$JAVAC_BIN" -cp tools/applets/bin/microemulator-2.0.4.jar -d data/applets/build/integration/classes \
 tools/applets/v4/integration/shims/javax/microedition/io/Connector.java \
 tools/applets/v4/shims/javax/microedition/media/control/VideoControl.java \
 tools/applets/v4/shims/javax/microedition/lcdui/Displayable.java \
 tools/applets/v4/shims/org/microemu/util/RecordStoreImpl.java \
 tools/applets/v4/integration/V4Integration.java \
 tools/applets/v4/integration/V4GroupIntegration.java \
 tools/applets/v4/integration/V4RoomIntegration.java \
 tools/applets/v4/integration/V4RoomModerationIntegration.java \
 tools/applets/v4/integration/V4BlockIntegration.java \
 tools/applets/v4/integration/V4ContactIntegration.java \
 tools/applets/v4/integration/V4VoteReportIntegration.java \
 tools/applets/v4/integration/V4RoomDiscoveryIntegration.java \
 tools/applets/v4/integration/V4FriendModerationIntegration.java \
 tools/applets/v4/integration/V4SlashIntegration.java \
 tools/applets/v4/integration/V4IdentityIntegration.java
fi
if [ "${1:-}" = --compile-only ]; then exit 0; fi
integration_home=$(mktemp -d "${TMPDIR:-/tmp}/mig33-v4-integration.XXXXXX")
integration_java_pid=
integration_cleanup() {
 if [ -n "$integration_java_pid" ]; then
  kill "$integration_java_pid" 2>/dev/null || true
  wait "$integration_java_pid" 2>/dev/null || true
 fi
 rm -rf -- "$integration_home"
}
trap integration_cleanup EXIT
trap 'exit 130' INT TERM
"$JAVA_BIN" --patch-module java.base=data/applets/build/integration/resource-bridge.jar \
 -Djava.awt.headless=true -Duser.home="$integration_home" \
 -cp data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar:research/applets/v4/artifacts/original/mig33-4.62.jar "${INTEGRATION_MAIN_CLASS:-V4Integration}" "${1:-alice}" "${2:-tcp}" &
integration_java_pid=$!
wait "$integration_java_pid"
integration_java_pid=
