#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
sh tools/applets/v4/integration/run.sh --compile-only
python3 - <<'PY'
from pathlib import Path
root=Path('data/applets/integration/block-out')
for folder in [root]+[root/u for u in ['alice','bob','carol']]:
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file(): p.unlink()
PY
block_pids=
block_cleanup() {
 for p in $block_pids; do kill "$p" 2>/dev/null || true; done
 for p in $block_pids; do wait "$p" 2>/dev/null || true; done
}
trap block_cleanup EXIT
trap 'exit 130' INT TERM
for block_user in bob carol alice; do
 block_transport=tcp
 if [ "$block_user" = bob ]; then block_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4BlockIntegration sh tools/applets/v4/integration/run.sh "$block_user" "$block_transport" >"data/applets/integration/block-out/$block_user/runtime.log" 2>&1 &
 block_pids="$block_pids $!"
done
block_status=0
for p in $block_pids; do if ! wait "$p"; then block_status=1; fi; done
block_pids=
if [ "$block_status" != 0 ]; then
 for block_user in alice bob carol; do cat "data/applets/integration/block-out/$block_user/runtime.log"; done
 exit 1
fi
for block_user in alice bob carol; do cat "data/applets/integration/block-out/$block_user/result.txt"; done
