#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
sh tools/applets/v4/integration/run.sh --compile-only
python3 - <<'PY'
from pathlib import Path
root=Path('data/applets/integration/out')
for folder in [root,root/'alice',root/'bob']:
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file() and (p.suffix in {'.bin','.png','.tsv','.log','.txt'} or p.name=='ready'): p.unlink()
PY
INTEGRATION_SKIP_BUILD=1 sh tools/applets/v4/integration/run.sh bob http >data/applets/integration/out/bob/runtime.log 2>&1 &
integration_bob_pid=$!
trap 'kill "$integration_bob_pid" 2>/dev/null || true; wait "$integration_bob_pid" 2>/dev/null || true' EXIT INT TERM
integration_wait=0
while [ ! -f data/applets/integration/out/bob/ready ]; do
 if ! kill -0 "$integration_bob_pid" 2>/dev/null || [ "$integration_wait" -ge 40 ]; then
  cat data/applets/integration/out/bob/runtime.log
  exit 1
 fi
 sleep 1
 integration_wait=$((integration_wait+1))
done
if ! INTEGRATION_SKIP_BUILD=1 sh tools/applets/v4/integration/run.sh alice tcp >data/applets/integration/out/alice/runtime.log 2>&1; then
 cat data/applets/integration/out/alice/runtime.log
 exit 1
fi
if ! wait "$integration_bob_pid"; then
 cat data/applets/integration/out/bob/runtime.log
 exit 1
fi
trap - EXIT INT TERM
cat data/applets/integration/out/alice/result.txt data/applets/integration/out/bob/result.txt
