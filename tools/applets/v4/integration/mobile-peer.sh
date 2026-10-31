#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."

# Reuse the existing, loopback-only original-client bridge and JVM shims.
# Compilation does not start a MIDlet or contact an application service.
JAVA_BIN=${JAVA_BIN:-/opt/homebrew/opt/openjdk@21/bin/java}
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
uv run --no-project python - <<'PY'
import hashlib
from pathlib import Path
expected = {
    "research/applets/v4/artifacts/original/mig33-4.62.jar": "dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c",
    "tools/applets/bin/microemulator-2.0.4.jar": "dbd5f3eb8365d3e839d6a203149e0e3776fc1a0585e16ac1fc23f76c9fcae1c6",
}
for name, digest in expected.items():
    path = Path(name)
    if not path.is_file():
        raise SystemExit(f"Missing {name}; follow tools/applets/SETUP.md")
    if hashlib.sha256(path.read_bytes()).hexdigest() != digest:
        raise SystemExit(f"Unexpected JAR bytes: {name}")
PY

if [ "${MOBILE_PEER_SKIP_BUILD:-0}" != 1 ]; then
 uv run --no-project sh tools/applets/v4/integration/run.sh --compile-only
 "$JAVAC_BIN" -cp data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar \
  -d data/applets/build/integration/classes tools/applets/v4/integration/V4MobilePeer.java
fi
if [ "${1:-}" = --compile-only ]; then exit 0; fi
if [ "$#" != 3 ]; then
 echo "Usage: $0 <demo-user> <http|tcp> <output-directory>" >&2
 exit 2
fi
# Acceptance runs must opt into isolated listeners, never the user's live ports.
: "${MIG33_TEST_HTTP_PORT:?Set an isolated HTTP listener port}"
: "${MIG33_TEST_TCP_PORT:?Set an isolated TCP listener port}"
if [ "$MIG33_TEST_HTTP_PORT" = 8080 ] || [ "$MIG33_TEST_TCP_PORT" = 9119 ]; then
 echo "Use isolated listener ports, not 8080/9119" >&2
 exit 2
fi

mobile_peer_home=$(mktemp -d "${TMPDIR:-/tmp}/mig33-v4-mobile-peer.XXXXXX")
mobile_peer_pid=
mobile_peer_cleanup() {
 if [ -n "$mobile_peer_pid" ]; then
  kill "$mobile_peer_pid" 2>/dev/null || true
  wait "$mobile_peer_pid" 2>/dev/null || true
 fi
 rm -rf -- "$mobile_peer_home"
}
trap mobile_peer_cleanup EXIT
trap 'exit 130' INT TERM
# Explicit fd 0 preserves the command pipe for the background Java child.
"$JAVA_BIN" --patch-module java.base=data/applets/build/integration/resource-bridge.jar \
 -Djava.awt.headless=true -Duser.home="$mobile_peer_home" \
 -cp data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar:research/applets/v4/artifacts/original/mig33-4.62.jar \
 V4MobilePeer "$@" <&0 &
mobile_peer_pid=$!
wait "$mobile_peer_pid"
mobile_peer_pid=
