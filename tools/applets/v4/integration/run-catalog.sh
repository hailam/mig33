#!/bin/sh
set -eu
cd "$(dirname "$0")/../../../.."
sh tools/applets/v4/integration/run.sh --compile-only
JAVAC_BIN=${JAVAC_BIN:-/opt/homebrew/opt/openjdk@21/bin/javac}
"$JAVAC_BIN" -cp data/applets/build/integration/classes:tools/applets/bin/microemulator-2.0.4.jar \
 -d data/applets/build/integration/classes tools/applets/v4/integration/V4PagesIntegration.java tools/applets/v4/integration/V4FormattingIntegration.java tools/applets/v4/integration/V4CatalogIntegration.java
if [ "${1:-}" = --compile-only ]; then exit 0; fi
if [ "${MIG33_TEST_HTTP_PORT:-8080}" = 8080 ] || [ "${MIG33_TEST_TCP_PORT:-9119}" = 9119 ]; then
 echo "Use an isolated fixture backend via MIG33_TEST_HTTP_PORT and MIG33_TEST_TCP_PORT." >&2
 exit 1
fi
curl --silent --fail "http://127.0.0.1:$MIG33_TEST_HTTP_PORT/healthz" >/dev/null
python3 - <<'PY'
from pathlib import Path
root=Path('data/applets/integration/catalog-out')
for folder in (root,root/'alice',root/'bob'):
    folder.mkdir(parents=True,exist_ok=True)
    for p in folder.iterdir():
        if p.is_file():p.unlink()
PY
catalog_pids=
catalog_cleanup() {
 for p in $catalog_pids; do kill "$p" 2>/dev/null || true; done
 for p in $catalog_pids; do wait "$p" 2>/dev/null || true; done
}
trap catalog_cleanup EXIT
trap 'exit 130' INT TERM
for catalog_user in bob alice; do
 catalog_transport=tcp
 if [ "$catalog_user" = bob ]; then catalog_transport=http; fi
 INTEGRATION_SKIP_BUILD=1 INTEGRATION_MAIN_CLASS=V4CatalogIntegration \
  sh tools/applets/v4/integration/run.sh "$catalog_user" "$catalog_transport" >"data/applets/integration/catalog-out/$catalog_user/runtime.log" 2>&1 &
 catalog_pids="$catalog_pids $!"
done
catalog_status=0
for p in $catalog_pids; do if ! wait "$p"; then catalog_status=1; fi; done
catalog_pids=
if [ "$catalog_status" != 0 ]; then
 for catalog_user in alice bob; do tail -90 "data/applets/integration/catalog-out/$catalog_user/runtime.log"; done
 exit 1
fi
python3 - <<'PY'
from pathlib import Path
import json,hashlib,html,os,urllib.request
from datetime import datetime,timezone
root=Path('data/applets/integration/catalog-out')
proof={'date':datetime.now(timezone.utc).date().isoformat(),'jar_sha256':hashlib.sha256(Path('research/applets/v4/artifacts/original/mig33-4.62.jar').read_bytes()).hexdigest(),'clients':{},'scope':'Recovered catalog names with explicitly local IDs, prices and atlas placeholders; not original pack download bytes or full historical game engines.'}
with urllib.request.urlopen('http://127.0.0.1:'+os.environ['MIG33_TEST_HTTP_PORT']+'/api/catalog') as response:
    catalog=json.load(response)
assert len(catalog['products'])==78 and len(catalog['games'])==25
assert catalog['providers']['sms']=='mock_recorded' and catalog['providers']['external_im']=='disconnected_tbd'
(root/'public-catalog.json').write_text(json.dumps(catalog,indent=2)+'\n')
proof['public_catalog']={'products':len(catalog['products']),'games':len(catalog['games']),'providers':catalog['providers']}
cards=[]
for user,transport in [('alice','tcp'),('bob','http')]:
    folder=root/user
    result=(folder/'result.txt').read_text().strip()
    assert result.startswith('PASS ')
    events=[line.split('\t') for line in (folder/'events.tsv').read_text().splitlines()]
    logout=[e for e in events if e[:2]==['TX','300']][-1]
    assert any(e[:3]==['RX','1',logout[2]] for e in events)
    shots=[p for p in folder.glob('*.png') if p.name!='background-fixture.png']
    proof['clients'][user]={'transport':transport,'result':result,'logout_ack':True,'frames':len(list(folder.glob('TX-*.bin')))+len(list(folder.glob('RX-*.bin'))),'screenshots':len(shots)}
    for p in sorted(shots):cards.append('<figure><a href="'+user+'/'+p.name+'"><img src="'+user+'/'+p.name+'"></a><figcaption>'+user+' / '+html.escape(p.stem)+'</figcaption></figure>')
(root/'evidence-summary.json').write_text(json.dumps(proof,indent=2)+'\n')
(root/'index.html').write_text('<!doctype html><meta charset="utf-8"><title>Native catalog acceptance</title><style>body{font:16px system-ui;margin:30px;background:#f4f6fa}.grid{display:flex;flex-wrap:wrap;gap:12px}figure{margin:0;padding:12px;background:white;border:1px solid #ccd4df;width:240px}img{width:240px;height:320px}figcaption{overflow-wrap:anywhere}</style><h1>Original v4.62 catalog acceptance</h1><p>Local catalog renditions rendered by unchanged original bytecode. 240×320; handset softkeys excluded.</p><div class="grid">'+''.join(cards)+'</div>')
print(json.dumps(proof,indent=2))
PY
if [ -n "${MIG33_TEST_DB:-}" ]; then
python3 - <<'PY'
from pathlib import Path
import os,json,sqlite3
p=Path(os.environ['MIG33_TEST_DB']).resolve()
db=sqlite3.connect(p.as_uri()+'?mode=ro',uri=True)
assert db.execute('pragma user_version').fetchone()==(18,)
purchases=db.execute("select item,amount,count(*) from credit_operations where actor='alice' and kind='purchase' group by item,amount order by item").fetchall()
assert purchases==[('big-hug',100,1),('emotes-lovely-pack',200,1),('kiss',100,1)],purchases
assert db.execute("select balance from credit_wallets where username='alice'").fetchone()==(600,)
for item in ['big-hug','kiss']:
 assert db.execute("select count(*) from owned_store_items where owner='bob' and sender='alice' and item=?",(item,)).fetchone()==(1,)
 token='[gift:'+item+']'
 rows=db.execute("select id,body from messages where sender='alice' and conversation='bob' and instr(body,?)>0",(token,)).fetchall()
 assert len(rows)==1,(item,rows)
 assert db.execute("select count(*) from message_audience where message_id=? and username='bob'",(rows[0][0],)).fetchone()==(1,)
 if item=='kiss':assert rows[0][1]=='alice sent bob a Kiss [gift:kiss] Message: Native gift note & plus + equals ='
notes=db.execute("select n.note from gift_notes n join credit_operations o on o.id=n.operation_id where o.actor='alice' and o.recipient='bob' and o.item='kiss'").fetchall()
assert notes==[('Native gift note & plus + equals =',)],notes
assert db.execute("select username,item,rating from gift_ratings").fetchall()==[('alice','kiss',4)]
assert db.execute("select bot,bot_expires,bot_rounds from community_room_controls where room='Catalog Room 15'").fetchone()==('',0,0)
assert db.execute('pragma integrity_check').fetchone()==('ok',)
assert db.execute('pragma foreign_key_check').fetchall()==[]
proof={'database':str(p),'schema':18,'purchase_rows':purchases,'alice_balance_hundredths':600,'recipient_inventory_per_gift':1,'recipient_message_per_gift':1,'gift_note_preserved':True,'ratings':1,'two_word_bot_stop_persisted':True,'integrity':'ok'}
Path('data/applets/integration/catalog-out/persistence-proof.json').write_text(json.dumps(proof,indent=2)+'\n')
print(json.dumps(proof,indent=2))
PY
fi
