#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
sh tools/applets/v4/integration/run.sh --compile-only
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
"$JAVAC_BIN" -cp data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar -d data/applets/build/integration/classes tools/applets/v4/integration/V4MirrorIntegration.java
if [ "${1:-}" = --compile-only ]; then exit 0; fi
curl --silent --fail "http://127.0.0.1:${MIG33_TEST_HTTP_PORT:-8080}/healthz" >/dev/null
python3 - <<'PY'
from pathlib import Path
root=Path('data/applets/integration/mirror-out')
for folder in [root]+[root/u for u in ['origin','mirror','bob']]:
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file():p.unlink()
PY
mirror_pids=
mirror_cleanup() {
 for p in $mirror_pids; do kill "$p" 2>/dev/null || true; done
 for p in $mirror_pids; do wait "$p" 2>/dev/null || true; done
}
trap mirror_cleanup EXIT
trap 'exit 130' INT TERM
for mirror_role in mirror bob origin; do
 mirror_transport=tcp
 if [ "$mirror_role" = mirror ]; then mirror_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4MirrorIntegration sh tools/applets/v4/integration/run.sh "$mirror_role" "$mirror_transport" >"data/applets/integration/mirror-out/$mirror_role/runtime.log" 2>&1 &
 mirror_pids="$mirror_pids $!"
done
mirror_status=0
for p in $mirror_pids; do if ! wait "$p"; then mirror_status=1; fi; done
mirror_pids=
if [ "$mirror_status" != 0 ]; then
 for mirror_role in origin mirror bob; do tail -80 "data/applets/integration/mirror-out/$mirror_role/runtime.log"; done
 exit 1
fi
python3 - <<'PY'
from pathlib import Path
import json,hashlib
root=Path('data/applets/integration/mirror-out')
proof={'jar_sha256':hashlib.sha256(Path('research/applets/v4/artifacts/original/mig33-4.62.jar').read_bytes()).hexdigest(),'clients':{}}
for role in ('origin','mirror','bob'):
    folder=root/role
    events=[line.split('\t') for line in (folder/'events.tsv').read_text().splitlines()]
    logout=[e for e in events if e[:2]==['TX','300']][-1]
    assert any(e[:3]==['RX','1',logout[2]] for e in events)
    proof['clients'][role]={'result':(folder/'result.txt').read_text().strip(),'logout_ack':True,'frames':len(list(folder.glob('*.bin'))),'screenshots':len(list(folder.glob('*.png')))}
(root/'evidence-summary.json').write_text(json.dumps(proof,indent=2)+'\n')
print(json.dumps(proof,indent=2))
PY
