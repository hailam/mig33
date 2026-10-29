#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
sh tools/applets/v4/integration/run.sh --compile-only
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
"$JAVAC_BIN" -cp data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar -d data/applets/build/integration/classes tools/applets/v4/integration/V4PagesIntegration.java tools/applets/v4/integration/V4MediaIntegration.java
if [ "${1:-}" = --compile-only ]; then exit 0; fi
curl --silent --fail "http://127.0.0.1:${MIG33_TEST_HTTP_PORT:-8080}/healthz" >/dev/null
python3 - <<'PY'
from pathlib import Path
root=Path('data/applets/integration/media-out')
for folder in (root,root/'tcp',root/'http'):
 folder.mkdir(parents=True,exist_ok=True)
 for p in folder.iterdir():
  if p.is_file():p.unlink()
PY
for media_transport in tcp http; do
 mkdir -p "data/applets/integration/media-out/$media_transport"
 if ! INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4MediaIntegration sh tools/applets/v4/integration/run.sh alice "$media_transport" >"data/applets/integration/media-out/$media_transport/runtime.log" 2>&1; then
  tail -90 "data/applets/integration/media-out/$media_transport/runtime.log"
  exit 1
 fi
 cat "data/applets/integration/media-out/$media_transport/result.txt"
done
python3 - <<'PY'
from pathlib import Path
import json,hashlib
root=Path('data/applets/integration/media-out')
result={'jar_sha256':hashlib.sha256(Path('research/applets/v4/artifacts/original/mig33-4.62.jar').read_bytes()).hexdigest(),'clients':{}}
for transport in ('tcp','http'):
 folder=root/transport
 events=[e.split('\t') for e in (folder/'events.tsv').read_text().splitlines()]
 logout=[e for e in events if e[:2]==['TX','300']][-1]
 assert any(e[:3]==['RX','1',logout[2]] for e in events)
 assert not any(e[:2]==['RX','804'] for e in events)
 result['clients'][transport]={'result':(folder/'result.txt').read_text().strip(),'logout_ack':True,'frames':len(list(folder.glob('TX-*.bin')))+len(list(folder.glob('RX-*.bin'))),'screenshots':len(list(folder.glob('*.png')))}
(root/'evidence-summary.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps(result,indent=2))
PY
