#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
JAVA_BIN=${JAVA_BIN:-/opt/homebrew/opt/openjdk@21/bin/java}
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
export JAVA_BIN JAVAC_BIN
if [ "$#" -gt 1 ]; then
 printf 'Usage: sh scripts/open-client.sh [alice|bob|carol|dave|hailam]\n' >&2
 exit 2
fi
case "${1:-}" in
 ''|alice|bob|carol|dave|hailam|--compile-only) ;;
 *) printf 'Unknown local account: %s\n' "$1" >&2; exit 2 ;;
esac
if [ "${1:-}" != --compile-only ]; then
 if ! curl --silent --fail --connect-timeout 2 --max-time 5 http://127.0.0.1:8080/healthz >/dev/null; then
  printf 'The local mig33 server is unavailable at 127.0.0.1:8080. Start it with: go run ./cmd/omni\n' >&2
  exit 1
 fi
fi
if [ "${MIG33_CLIENT_SKIP_BUILD:-0}" != 1 ]; then
 INTEGRATION_SKIP_BUILD=0 sh tools/applets/v4/integration/run.sh --compile-only
 "$JAVAC_BIN" -cp data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar \
  -d data/applets/build/integration/classes tools/applets/v4/V4Desktop.java
fi
if [ "${1:-}" = --compile-only ]; then exit 0; fi
if [ -n "${MIG33_CLIENT_OUTPUT:-}" ]; then
 client_run=$MIG33_CLIENT_OUTPUT
else
 mkdir -p data/desktop
 client_run=$(mktemp -d "$PWD/data/desktop/run.XXXXXX")
fi
mkdir -p "$client_run/home"
printf 'Client log and window preview: %s\n' "$client_run"
exec "$JAVA_BIN" --patch-module java.base=data/applets/build/integration/resource-bridge.jar \
 -Djava.awt.headless=false -Duser.home="$client_run/home" -Dmig33.client.output="$client_run" \
 "-Dmig33.client.label=${MIG33_CLIENT_LABEL:-}" "-Dmig33.client.index=${MIG33_CLIENT_INDEX:-0}" \
 -Xdock:name="mig33 local" \
 -cp data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar:research/applets/v4/artifacts/original/mig33-4.62.jar \
 V4Desktop "$@"
