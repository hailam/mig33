#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
sh tools/applets/v4/integration/run.sh --compile-only
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
"$JAVAC_BIN" -cp data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar \
 -d data/applets/build/integration/classes tools/applets/v4/integration/V4PagesIntegration.java tools/applets/v4/integration/V4FormattingIntegration.java
if [ "${1:-}" = --compile-only ]; then exit 0; fi
if [ "${MIG33_TEST_HTTP_PORT:-8080}" = 8080 ] || [ "${MIG33_TEST_TCP_PORT:-9119}" = 9119 ]; then
 echo "Use an isolated fixture backend via MIG33_TEST_HTTP_PORT and MIG33_TEST_TCP_PORT." >&2
 exit 1
fi
curl --silent --fail "http://127.0.0.1:$MIG33_TEST_HTTP_PORT/healthz" >/dev/null
python3 - <<'PY'
from pathlib import Path
root=Path('data/applets/integration/formatting-out')
for folder in (root,root/'alice',root/'bob'):
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file():p.unlink()
PY
formatting_pids=
formatting_cleanup() {
 for p in $formatting_pids; do kill "$p" 2>/dev/null || true; done
 for p in $formatting_pids; do wait "$p" 2>/dev/null || true; done
}
trap formatting_cleanup EXIT
trap 'exit 130' INT TERM
for formatting_user in bob alice; do
 formatting_transport=tcp
 if [ "$formatting_user" = bob ]; then formatting_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4FormattingIntegration \
  sh tools/applets/v4/integration/run.sh "$formatting_user" "$formatting_transport" >"data/applets/integration/formatting-out/$formatting_user/runtime.log" 2>&1 &
 formatting_pids="$formatting_pids $!"
done
formatting_status=0
for p in $formatting_pids; do if ! wait "$p"; then formatting_status=1; fi; done
formatting_pids=
if [ "$formatting_status" != 0 ]; then
 for formatting_user in alice bob; do tail -90 "data/applets/integration/formatting-out/$formatting_user/runtime.log"; done
 exit 1
fi
python3 - <<'PY'
from pathlib import Path
import json,hashlib,html
from datetime import datetime,timezone
root=Path('data/applets/integration/formatting-out')
proof={'date':datetime.now(timezone.utc).date().isoformat(),'jar_sha256':hashlib.sha256(Path('research/applets/v4/artifacts/original/mig33-4.62.jar').read_bytes()).hexdigest(),'clients':{},'scope':'Reconstructed explicit HTML authoring; original native renderer, no claim of exact historical injection or unavailable chat-code syntax.','native_background_image':False,'background_limitation':'Original v4.62 parses body background but neither fetches nor paints it; solid background colours and inline photos are verified.'}
cards=[]
for user,transport in [('alice','tcp'),('bob','http')]:
    folder=root/user
    result=(folder/'result.txt').read_text().strip()
    assert result.startswith('PASS ')
    events=[line.split('\t') for line in (folder/'events.tsv').read_text().splitlines()]
    logout=[e for e in events if e[:2]==['TX','300']][-1]
    assert any(e[:3]==['RX','1',logout[2]] for e in events)
    shots=[p for p in folder.glob('*.png') if p.name!='background-fixture.png']
    proof['clients'][user]={'transport':transport,'result':result,'logout_ack':True,'frames':len(list(folder.glob('*.bin'))),'screenshots':len(shots)}
    for p in sorted(shots):cards.append('<figure><a href="'+user+'/'+p.name+'"><img src="'+user+'/'+p.name+'"></a><figcaption>'+user+' / '+html.escape(p.stem)+'</figcaption></figure>')
(root/'evidence-summary.json').write_text(json.dumps(proof,indent=2)+'\n')
(root/'index.html').write_text('<!doctype html><meta charset="utf-8"><title>Native formatting acceptance</title><style>body{font:16px system-ui;margin:30px;background:#f4f6fa}.grid{display:flex;flex-wrap:wrap;gap:12px}figure{margin:0;padding:12px;background:white;border:1px solid #ccd4df;width:240px}img{width:240px;height:320px}figcaption{overflow-wrap:anywhere}</style><h1>Original v4.62 formatting acceptance</h1><p>Local reconstructed HTML authoring rendered by unchanged original bytecode. 240×320; handset softkeys excluded.</p><div class="grid">'+''.join(cards)+'</div>')
print(json.dumps(proof,indent=2))
PY
