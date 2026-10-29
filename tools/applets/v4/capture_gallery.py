#!/usr/bin/env python3
from pathlib import Path
import json,html,hashlib
root=Path(__file__).resolve().parents[3]
v4=root/'research/applets/v4'; directory=v4/'artifacts/screenshots/240x320'
rows=[s.split('\t',1) for s in (directory/'index.tsv').read_text().splitlines()]
items=[{'file':n+'.png','description':d,'sha256':hashlib.sha256((directory/(n+'.png')).read_bytes()).hexdigest()} for n,d in rows]
manifest={'profile':'240x320 Canvas, MicroEmulator 2.0.4 with upstream Displayable null-command fix; original default_v2 theme and en-US locale',
 'original_jar_sha256':hashlib.sha256((v4/'artifacts/original/mig33-4.62.jar').read_bytes()).hexdigest(),
 'microemulator_sha256':hashlib.sha256((root/'tools/applets/bin/microemulator-2.0.4.jar').read_bytes()).hexdigest(),
 'native_softkeys_included':False,
 'resource_bridge_entries':['default_v2','en-US'],
 'displayable_source':'https://github.com/barteo/microemu/blob/578b56783b417ea72349290c9b11dce4e895ccb7/microemulator/microemu-midp/src/main/java/javax/microedition/lcdui/Displayable.java',
 'displayable_source_sha256':hashlib.sha256((root/'tools/applets/v4/shims/javax/microedition/lcdui/Displayable.java').read_bytes()).hexdigest(),
 'network':'Connector throws for all protocols. Validation run:4 refused attempts including mandatory self-test.',
 'images':items}
(v4/'docs/data/screenshots-240x320.json').write_text(json.dumps(manifest,indent=2)+'\n')
figures=''.join('<figure><a href="'+html.escape(i['file'])+'"><img width="240" height="320" loading="lazy" src="'+html.escape(i['file'])+'"></a><figcaption><b>'+html.escape(i['file'])+'</b><br>'+html.escape(i['description'])+'</figcaption></figure>' for i in items)
(directory/'index.html').write_text('<!doctype html><meta charset="utf-8"><title>mig33 4.62 original bytecode captures</title><style>body{font:14px system-ui;background:#f0f2f5;color:#172331;margin:24px}main{display:flex;flex-wrap:wrap;gap:24px}figure{width:260px;margin:0}img{border:1px solid #666;image-rendering:pixelated}figcaption{overflow-wrap:anywhere;margin:8px 0 22px}</style><h1>mig33 4.62 — original bytecode captures</h1><p>240×320 Canvas; MicroEmulator fonts. Native device softkeys excluded. Fixture content and offline menus are identified in captions. These are reproducible runtime captures, not historical handset screenshots.</p><main>'+figures+'</main>')
print('Indexed',len(items),'v4 captures')
