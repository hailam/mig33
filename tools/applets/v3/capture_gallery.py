#!/usr/bin/env python3
"""Index actual original-bytecode captures; no synthetic screenshot generation."""
from pathlib import Path
import hashlib
import html
import json
root = Path(__file__).resolve().parents[3]
v3 = root / 'research/applets/v3'
directory = v3 / 'artifacts/screenshots/240x320'
rows = [line.split('\t', 1) for line in (directory / 'index.tsv').read_text().splitlines()]
items = []
for stem, note in rows:
    path = directory / (stem + '.png')
    items.append({'file': path.name, 'description': note,
                  'sha256': hashlib.sha256(path.read_bytes()).hexdigest()})
manifest = {'profile': 'MicroEmulator 2.0.4, 240x320 Canvas, default font profile',
            'original_jar_sha256': hashlib.sha256((v3 / 'artifacts/original/mig33-3.08.jar').read_bytes()).hexdigest(),
            'microemulator_sha256': hashlib.sha256((root / 'tools/applets/bin/microemulator-2.0.4.jar').read_bytes()).hexdigest(),
            'native_softkeys_included': False,
            'network': 'Connector shim throws for every protocol; one test refusal, no application network attempts',
            'images': items}
(v3 / 'docs/data/screenshots-240x320.json').write_text(json.dumps(manifest, indent=2) + '\n')
figures = ''.join('<figure><a href="'+html.escape(i['file'])+'"><img loading="lazy" width="240" height="320" src="'+html.escape(i['file'])+'"></a><figcaption><b>'+html.escape(i['file'])+'</b><br>'+html.escape(i['description'])+'</figcaption></figure>' for i in items)
(directory / 'index.html').write_text('<!doctype html><meta charset="utf-8"><title>mig33 3.08 original bytecode captures</title><style>body{font:14px system-ui;background:#f0f2f5;color:#172331;margin:24px}main{display:flex;flex-wrap:wrap;gap:24px}figure{margin:0;width:260px}img{image-rendering:pixelated;border:1px solid #666}figcaption{overflow-wrap:anywhere;margin:8px 0 22px}</style><h1>mig33 3.08 — original bytecode captures</h1><p>240×320 Canvas, MicroEmulator 2.0.4 fonts. Native device softkeys excluded. Fixture records are synthetic. These are reproducible runtime captures, not historical handset screenshots.</p><main>'+figures+'</main>')
print(f'Indexed {len(items)} captures')
