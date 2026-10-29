#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
sh tools/applets/v4/integration/run.sh --compile-only
python3 - <<'PY'
from pathlib import Path
root=Path('data/applets/integration/room-moderation-out')
for folder in [root]+[root/u for u in ['alice','bob','carol']]:
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file(): p.unlink()
PY
moderation_pids=
moderation_cleanup() {
 for p in $moderation_pids; do kill "$p" 2>/dev/null || true; done
 for p in $moderation_pids; do wait "$p" 2>/dev/null || true; done
}
trap moderation_cleanup EXIT
trap 'exit 130' INT TERM
for moderation_user in bob carol alice; do
 moderation_transport=tcp
 if [ "$moderation_user" = bob ]; then moderation_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4RoomModerationIntegration sh tools/applets/v4/integration/run.sh "$moderation_user" "$moderation_transport" >"data/applets/integration/room-moderation-out/$moderation_user/runtime.log" 2>&1 &
 moderation_pids="$moderation_pids $!"
done
moderation_status=0
for p in $moderation_pids; do if ! wait "$p"; then moderation_status=1; fi; done
moderation_pids=
if [ "$moderation_status" != 0 ]; then
 for moderation_user in alice bob carol; do cat "data/applets/integration/room-moderation-out/$moderation_user/runtime.log"; done
 exit 1
fi
for moderation_user in alice bob carol; do cat "data/applets/integration/room-moderation-out/$moderation_user/result.txt"; done
