#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
sh tools/applets/v4/integration/run.sh --compile-only
python3 - <<'PY'
from pathlib import Path
root=Path('data/applets/integration/vote-report-out')
for folder in [root]+[root/u for u in ['hailam','bob','carol','dave','alice']]:
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file(): p.unlink()
PY
vote_pids=
vote_cleanup() {
 for p in $vote_pids; do kill "$p" 2>/dev/null || true; done
 for p in $vote_pids; do wait "$p" 2>/dev/null || true; done
}
trap vote_cleanup EXIT
trap 'exit 130' INT TERM
for vote_user in bob carol dave alice hailam; do
 vote_transport=tcp
 if [ "$vote_user" = bob ]; then vote_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4VoteReportIntegration sh tools/applets/v4/integration/run.sh "$vote_user" "$vote_transport" >"data/applets/integration/vote-report-out/$vote_user/runtime.log" 2>&1 &
 vote_pids="$vote_pids $!"
done
vote_status=0
for p in $vote_pids; do if ! wait "$p"; then vote_status=1; fi; done
vote_pids=
if [ "$vote_status" != 0 ]; then
 for vote_user in hailam bob carol dave alice; do cat "data/applets/integration/vote-report-out/$vote_user/runtime.log"; done
 exit 1
fi
for vote_user in hailam bob carol dave alice; do cat "data/applets/integration/vote-report-out/$vote_user/result.txt"; done
