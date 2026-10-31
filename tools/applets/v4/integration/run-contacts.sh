#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
sh tools/applets/v4/integration/run.sh --compile-only
uv run --no-project python - <<'PY'
from pathlib import Path
root=Path('data/applets/integration/contact-out')
for folder in [root]+[root/u for u in ['hailam','bob']]:
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file(): p.unlink()
PY
contact_pids=
contact_cleanup() {
 for p in $contact_pids; do kill "$p" 2>/dev/null || true; done
 for p in $contact_pids; do wait "$p" 2>/dev/null || true; done
}
trap contact_cleanup EXIT
trap 'exit 130' INT TERM
for contact_user in bob hailam; do
 contact_transport=tcp
 if [ "$contact_user" = bob ]; then contact_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4ContactIntegration sh tools/applets/v4/integration/run.sh "$contact_user" "$contact_transport" >"data/applets/integration/contact-out/$contact_user/runtime.log" 2>&1 &
 contact_pids="$contact_pids $!"
done
contact_status=0
for p in $contact_pids; do if ! wait "$p"; then contact_status=1; fi; done
contact_pids=
if [ "$contact_status" != 0 ]; then
 for contact_user in hailam bob; do cat "data/applets/integration/contact-out/$contact_user/runtime.log"; done
 exit 1
fi
for contact_user in hailam bob; do cat "data/applets/integration/contact-out/$contact_user/result.txt"; done
