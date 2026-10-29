#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
sh tools/applets/v4/integration/run.sh --compile-only
python3 - <<'PY'
from pathlib import Path
import time
root=Path('data/applets/integration/identity-out')
for folder in [root,root/'register',root/'return']:
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file():p.unlink()
stamp=str(int(time.time()))
(root/'username.txt').write_text('nativeid'+stamp+'\n')
(root/'mobile.txt').write_text('+960'+stamp+'\n')
PY
identity_user=$(cat data/applets/integration/identity-out/username.txt)
for identity_phase in register return; do
 identity_transport=tcp
 if [ "$identity_phase" = return ]; then identity_transport=http; fi
 if ! IDENTITY_PHASE="$identity_phase" INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4IdentityIntegration \
  sh tools/applets/v4/integration/run.sh "$identity_user" "$identity_transport" >"data/applets/integration/identity-out/$identity_phase/runtime.log" 2>&1; then
  cat "data/applets/integration/identity-out/$identity_phase/runtime.log"
  exit 1
 fi
 cat "data/applets/integration/identity-out/$identity_phase/result.txt"
done
python3 - <<'PY'
from pathlib import Path
import json,hashlib
root=Path('data/applets/integration/identity-out')
summary={'jar_sha256':hashlib.sha256(Path('research/applets/v4/artifacts/original/mig33-4.62.jar').read_bytes()).hexdigest(),'username':(root/'username.txt').read_text().strip(),'clients':{}}
for phase,transport in [('register','tcp'),('return','http')]:
 folder=root/phase
 events=[x.split('\t') for x in (folder/'events.tsv').read_text().splitlines()]
 logout=[x for x in events if x[:2]==['TX','300']][-1]
 assert any(x[:3]==['RX','1',logout[2]] for x in events)
 result=(folder/'result.txt').read_text().strip();assert result.startswith('PASS')
 summary['clients'][phase]={'transport':transport,'screenshots':len(list(folder.glob('*.png'))),'frames':len(list(folder.glob('*.bin'))),'logout_ack':True,'result':result}
(root/'evidence-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(summary,indent=2))
PY
