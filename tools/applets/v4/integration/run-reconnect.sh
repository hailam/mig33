#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
export MIG33_TEST_HTTP_PORT=${MIG33_TEST_HTTP_PORT:-18080}
export MIG33_TEST_TCP_PORT=${MIG33_TEST_TCP_PORT:-19119}
sh tools/applets/v4/integration/run.sh --compile-only
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
"$JAVAC_BIN" -cp data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar -d data/applets/build/integration/classes tools/applets/v4/integration/V4PagesIntegration.java tools/applets/v4/integration/V4ReconnectIntegration.java
if [ "${1:-}" = --compile-only ]; then exit 0; fi
curl --silent --fail "http://127.0.0.1:$MIG33_TEST_HTTP_PORT/healthz" >/dev/null
python3 - <<'PY'
from pathlib import Path
from datetime import datetime,timezone
root=Path('data/applets/integration/reconnect-out')
for folder in [root]+[root/user for user in ['alice','bob','carol']]:
 folder.mkdir(parents=True,exist_ok=True)
 for p in folder.iterdir():
  if p.is_file():p.unlink()
(root/'room-name.txt').write_text('Reconnect '+datetime.now(timezone.utc).strftime('%Y%m%d%H%M%S')+'\n')
PY
reconnect_pids=
reconnect_cleanup() {
 for pid in $reconnect_pids; do kill "$pid" 2>/dev/null || true; done
 for pid in $reconnect_pids; do wait "$pid" 2>/dev/null || true; done
}
trap reconnect_cleanup EXIT
trap 'exit 130' INT TERM
for reconnect_user in alice bob carol; do
 reconnect_transport=http
 if [ "$reconnect_user" = bob ]; then reconnect_transport=tcp; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4ReconnectIntegration sh tools/applets/v4/integration/run.sh "$reconnect_user" "$reconnect_transport" >"data/applets/integration/reconnect-out/$reconnect_user/runtime.log" 2>&1 &
 reconnect_pids="$reconnect_pids $!"
done
reconnect_status=0
for pid in $reconnect_pids; do if ! wait "$pid"; then reconnect_status=1; fi; done
reconnect_pids=
if [ "$reconnect_status" != 0 ]; then
 for reconnect_user in alice bob carol; do tail -50 "data/applets/integration/reconnect-out/$reconnect_user/runtime.log"; done
 exit 1
fi
python3 - <<'PY'
from pathlib import Path
import json,hashlib
root=Path('data/applets/integration/reconnect-out')
summary={'original_jar_sha256':hashlib.sha256(Path('research/applets/v4/artifacts/original/mig33-4.62.jar').read_bytes()).hexdigest(),'room':(root/'room-name.txt').read_text().strip(),'scenarios':['normal automatic rejoin','full-room rejoin rejection','banned-room rejoin rejection','kick popup and retained tab','kick alone allows later reconnect rejoin'],'clients':{}}
for user in ['alice','bob','carol']:
 folder=root/user;result=(folder/'result.txt').read_text().strip();assert result.startswith('PASS ')
 events=[line.split('\t') for line in (folder/'events.tsv').read_text().splitlines()];logout=[e for e in events if e[:2]==['TX','300']][-1];assert any(e[:3]==['RX','1',logout[2]] for e in events)
 summary['clients'][user]={'result':result,'screenshots':len(list(folder.glob('*.png'))),'binary_frames':len(list(folder.glob('*.bin'))),'correlated_logout':True,'login203':sum(e[:2]==['RX','203'] for e in events),'join703':sum(e[:2]==['TX','703'] for e in events)}
(root/'evidence-summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary,indent=2))
PY
