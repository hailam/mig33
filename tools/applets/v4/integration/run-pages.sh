#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
sh tools/applets/v4/integration/run.sh --compile-only
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
"$JAVAC_BIN" -cp data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar \
 -d data/applets/build/integration/classes tools/applets/v4/integration/V4PagesIntegration.java tools/applets/v4/integration/V4PageCaptureAudit.java
if [ "${1:-}" = --compile-only ]; then exit 0; fi
if [ "${1:-}" != --audit-only ]; then
curl --silent --fail http://127.0.0.1:8080/healthz >/dev/null
python3 - <<'PY'
from pathlib import Path
from datetime import datetime,timezone
import json
root=Path('data/applets/integration/pages-out')
for folder in [root,root/'alice',root/'bob']:
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file():p.unlink()
suffix=datetime.now(timezone.utc).strftime('%Y%m%d%H%M%S')
# Preserve the full timestamp while respecting the historical 15-character limit.
(root/'room-name.txt').write_text('P'+suffix+'\n')
(root/'community-name.txt').write_text('Community '+suffix+'\n')
routes=[]
for r in json.load(open('research/applets/v4/docs/data/endpoints-classified.json'))['literals']:
    if r['kind']!='embedded_page_template':continue
    id=r['id'];url=r['literal']
    replacements={'L005':'bob','L008':'','L009':'9607777777','L012':'{ROOM}',
      'L013':'&to=bob','L014':'{ROOM}','L021':'&to=alice','L023':'MSN',
      'L026':'','L032':'alice','L037':'alice','L038':'bob','L039':'{ROOM}',
      'L040':'{CHATGROUP}','L049':'9607777777','L051':'bob'}
    url=url.replace('%s',replacements.get(id,''))
    if id in ['L015','L016','L019','L020']:url+='&cid={COMMUNITY}'
    if id=='L025':url+='HOME'
    routes.append(id+'\t'+url+'\t'+r['purpose'])
assert len(routes)==52
(root/'routes.tsv').write_text('\n'.join(routes)+'\n')
PY
pages_pids=
pages_cleanup() {
 for pid in $pages_pids; do kill "$pid" 2>/dev/null || true; done
 for pid in $pages_pids; do wait "$pid" 2>/dev/null || true; done
}
trap pages_cleanup EXIT
trap 'exit 130' INT TERM
for pages_user in alice bob; do
 pages_transport=tcp
 if [ "$pages_user" = bob ]; then pages_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4PagesIntegration \
  sh tools/applets/v4/integration/run.sh "$pages_user" "$pages_transport" >"data/applets/integration/pages-out/$pages_user/runtime.log" 2>&1 &
 pages_pids="$pages_pids $!"
done
pages_status=0
for pid in $pages_pids; do if ! wait "$pid"; then pages_status=1; fi; done
pages_pids=
if [ "$pages_status" != 0 ]; then
 for pages_user in alice bob; do tail -70 "data/applets/integration/pages-out/$pages_user/runtime.log"; done
 exit 1
fi
fi
JAVA_BIN=${JAVA_BIN:-/opt/homebrew/opt/openjdk@21/bin/java}
"$JAVA_BIN" -cp data/applets/build/integration/classes V4PageCaptureAudit data/applets/integration/pages-out
python3 - <<'PY'
from pathlib import Path
from datetime import datetime,timezone
import json,hashlib,html
root=Path('data/applets/integration/pages-out')
summary={'date':datetime.now(timezone.utc).date().isoformat(),
 'jar_sha256':hashlib.sha256(Path('research/applets/v4/artifacts/original/mig33-4.62.jar').read_bytes()).hexdigest(),
 'clients':{},'literal_templates':52,'historical_page_fidelity':'Local replacement pages rendered by original client; not recovered historical server HTML'}
cards=[]
for user,transport in [('alice','tcp'),('bob','http')]:
 folder=root/user;result=(folder/'result.txt').read_text().strip();assert result.startswith('PASS ')
 events=[r.split('\t') for r in (folder/'events.tsv').read_text().splitlines()]
 logout=[r for r in events if r[:2]==['TX','300']][-1]
 assert any(r[:3]==['RX','1',logout[2]] for r in events)
 captures=sorted(folder.glob('*.png'))
 summary['clients'][user]={'transport':transport,'result':result,'screenshots':len(captures),'frames':len(list(folder.glob('*.bin'))),'logout_transaction':int(logout[2]),'correlated_logout_ack':True}
 metrics=[line.split('\t') for line in (folder/'page-render-metrics.tsv').read_text().splitlines()]
 names={row[0] for row in metrics}
 assert len(names)==len(metrics)
 assert all(int(row[1])>=2 and int(row[2])>=3 and float(row[3])<0.995 for row in metrics)
 for response in folder.glob('*.html'):assert response.stem in names
 summary['clients'][user]['stable_populated_page_captures']=len(metrics)
 for p in captures:cards.append('<figure><a href="'+user+'/'+p.name+'"><img src="'+user+'/'+p.name+'" alt="'+html.escape(p.stem)+'"></a><figcaption>'+user+' / '+html.escape(p.stem)+'</figcaption></figure>')
 print(result)
covered=[r.split('\t')[0] for r in (root/'alice'/'pages.tsv').read_text().splitlines() if r.startswith('L')]
assert len(covered)==52 and len(set(covered))==52
summary['covered_template_ids']=covered
audit=[line.split('\t') for line in (root/'capture-pixel-audit.tsv').read_text().splitlines()[1:]]
assert len(audit)==sum(c['screenshots'] for c in summary['clients'].values())
assert all(row[3]=='true' or row[4]=='true' for row in audit)
summary['canvas_capture_audit']={'captures_checked':len(audit),'populated':sum(row[3]=='true' for row in audit),'intentional_empty_chat':sum(row[4]=='true' for row in audit),'blank_page_captures':0,'body_bounds':[4,24,232,306],'minimum_distinct_colors':2,'maximum_dominant_color_fraction':0.995}
(root/'evidence-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
(root/'index.html').write_text('<!doctype html><meta charset="utf-8"><title>Original v4.62 native pages acceptance</title><style>body{font:16px system-ui;margin:30px;background:#f4f6fa}.grid{display:flex;flex-wrap:wrap;gap:12px}figure{margin:0;padding:12px;background:white;border:1px solid #ccd4df;width:240px}img{width:240px;height:320px}figcaption{overflow-wrap:anywhere}</style><h1>Original v4.62 page and colour acceptance</h1><p>Actual original-bytecode rendering of local replacement pages.240×320 Canvas; handset softkeys excluded. <a href="evidence-summary.json">Evidence summary</a></p><div class="grid">'+''.join(cards)+'</div>')
print('Verified52 original URL templates; saved native gallery and evidence summary.')
PY
