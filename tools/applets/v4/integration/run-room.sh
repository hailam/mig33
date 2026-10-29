#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
sh tools/applets/v4/integration/run.sh --compile-only
python3 - <<'PY'
from pathlib import Path
root=Path('data/applets/integration/room-out')
for folder in [root]+[root/u for u in ['alice','bob','carol']]:
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file(): p.unlink()
PY
room_pids=
room_cleanup() {
 for p in $room_pids; do kill "$p" 2>/dev/null || true; done
 for p in $room_pids; do wait "$p" 2>/dev/null || true; done
}
trap room_cleanup EXIT
trap 'exit 130' INT TERM
for room_user in bob carol alice; do
 room_transport=tcp
 if [ "$room_user" = bob ]; then room_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4RoomIntegration sh tools/applets/v4/integration/run.sh "$room_user" "$room_transport" >"data/applets/integration/room-out/$room_user/runtime.log" 2>&1 &
 room_pids="$room_pids $!"
done
room_status=0
for p in $room_pids; do if ! wait "$p"; then room_status=1; fi; done
room_pids=
if [ "$room_status" != 0 ]; then
 for room_user in alice bob carol; do cat "data/applets/integration/room-out/$room_user/runtime.log"; done
 exit 1
fi
for room_user in alice bob carol; do cat "data/applets/integration/room-out/$room_user/result.txt"; done
