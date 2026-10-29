#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
sh tools/applets/v4/integration/run.sh --compile-only
python3 - <<'PY'
from pathlib import Path
root=Path('data/applets/integration/group-out')
for folder in [root]+[root/u for u in ['alice','bob','carol','dave']]:
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file(): p.unlink()
PY
group_pids=
group_cleanup() {
 for p in $group_pids; do kill "$p" 2>/dev/null || true; done
 for p in $group_pids; do wait "$p" 2>/dev/null || true; done
}
trap group_cleanup EXIT
trap 'exit 130' INT TERM
for group_user in bob carol dave alice; do
 group_transport=tcp
 if [ "$group_user" = bob ] || [ "$group_user" = carol ]; then group_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4GroupIntegration sh tools/applets/v4/integration/run.sh "$group_user" "$group_transport" >"data/applets/integration/group-out/$group_user/runtime.log" 2>&1 &
 group_pids="$group_pids $!"
done
group_status=0
for p in $group_pids; do if ! wait "$p"; then group_status=1; fi; done
group_pids=
if [ "$group_status" != 0 ]; then
 for group_user in alice bob carol dave; do cat "data/applets/integration/group-out/$group_user/runtime.log"; done
 exit 1
fi
for group_user in alice bob carol dave; do cat "data/applets/integration/group-out/$group_user/result.txt"; done
