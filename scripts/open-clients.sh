#!/bin/sh
# Two independent original clients. No arguments leaves both signed out.
set -eu
cd "$(dirname "$0")/.."
if [ "$#" -ne 0 ] && [ "$#" -ne 2 ]; then
 printf 'Usage: sh scripts/open-clients.sh [ACCOUNT1 ACCOUNT2]\nOmit both accounts to open two login screens. Example: sh scripts/open-clients.sh hailam bob\n' >&2
 exit 2
fi
for client_account in "$@"; do
 case "$client_account" in
  alice|bob|carol|dave|hailam) ;;
  *) printf 'Unknown local account: %s\n' "$client_account" >&2; exit 2 ;;
 esac
done
if ! curl --silent --fail --connect-timeout 2 --max-time 5 http://127.0.0.1:8080/healthz >/dev/null; then
 printf 'The local mig33 server is unavailable at 127.0.0.1:8080. Start it with: go run ./cmd/omni\n' >&2
 exit 1
fi
# Build before starting either JVM: both use the same read-only classes and bridge.
MIG33_CLIENT_SKIP_BUILD=0 sh scripts/open-client.sh --compile-only
mkdir -p data/desktop
pair_run=$(mktemp -d "$PWD/data/desktop/pair.XXXXXX")
mkdir -p "$pair_run/client-1" "$pair_run/client-2"
client_one_pid=
client_two_pid=
pair_cleanup() {
 for client_pid in "$client_one_pid" "$client_two_pid"; do
  if [ -n "$client_pid" ]; then kill "$client_pid" 2>/dev/null || true; fi
 done
 for client_pid in "$client_one_pid" "$client_two_pid"; do
  if [ -n "$client_pid" ]; then wait "$client_pid" 2>/dev/null || true; fi
 done
}
trap pair_cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
start_client() {
 MIG33_CLIENT_SKIP_BUILD=1 MIG33_CLIENT_OUTPUT="$pair_run/client-$1" \
 MIG33_CLIENT_LABEL="Client $1" MIG33_CLIENT_INDEX="$1" \
  sh scripts/open-client.sh "$2" >"$pair_run/client-$1/runtime.log" 2>&1 &
 launched_pid=$!
}
start_client 1 "${1:-}"
client_one_pid=$launched_pid
start_client 2 "${2:-}"
client_two_pid=$launched_pid
printf 'Opening two local clients. Logs and previews: %s\n' "$pair_run"
wait_ready() {
 client_attempt=0
 while [ ! -f "$pair_run/client-$1/ready.txt" ]; do
  if ! kill -0 "$2" 2>/dev/null || [ "$client_attempt" -ge 30 ]; then
   printf 'Client %s did not become ready. Its log follows:\n' "$1" >&2
   tail -n 20 "$pair_run/client-$1/runtime.log" >&2
   return 1
  fi
  client_attempt=$((client_attempt + 1))
  sleep 1
 done
}
wait_ready 1 "$client_one_pid"
wait_ready 2 "$client_two_pid"
printf 'Both clients are ready. Close both windows or press Ctrl+C here to stop them.\n'
pair_status=0
wait "$client_one_pid" || pair_status=$?
client_one_pid=
wait "$client_two_pid" || pair_status=$?
client_two_pid=
exit "$pair_status"
