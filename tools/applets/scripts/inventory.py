#!/usr/bin/env python3
"""Build auditable inventories directly from the recovered JAR and decompilation.

Standard library only. Does not execute the MIDlet or access the network.
"""
import hashlib
import html
import json
import re
import struct
import sys
from pathlib import Path
from zipfile import ZipFile

REPO = Path(__file__).resolve().parents[3]
VERSIONS = {'v3': '3.08', 'v4': '4.62'}
target = sys.argv[1] if len(sys.argv) > 1 else 'v3'
ROOT = REPO / 'research/applets' / target if target in VERSIONS else Path(target).resolve()
if ROOT.name not in VERSIONS:
    raise SystemExit('Choose v3, v4, or the path to a version reference directory.')
VERSION = VERSIONS[ROOT.name]
OUT = ROOT / 'docs/data'


def class_pool(data):
    assert data[:4] == b'\xca\xfe\xba\xbe'
    minor, major, count = struct.unpack_from('>HHH', data, 4)
    pool, pos, idx = [None] * count, 10, 1
    while idx < count:
        tag, pos = data[pos], pos + 1
        if tag == 1:
            size = struct.unpack_from('>H', data, pos)[0]
            pos += 2
            raw = data[pos:pos+size].replace(b'\xc0\x80', b'\0')
            # JVM modified UTF-8 uses surrogate pairs for supplementary chars.
            value = raw.decode('utf-8', errors='surrogatepass')
            value = value.encode('utf-16', errors='surrogatepass').decode('utf-16')
            pos += size
        elif tag in (7, 8, 16, 19, 20):
            value = struct.unpack_from('>H', data, pos)[0]
            pos += 2
        elif tag in (3, 4, 9, 10, 11, 12, 17, 18):
            value = data[pos:pos+4].hex()
            pos += 4
        elif tag in (5, 6):
            value = data[pos:pos+8].hex()
            pos += 8
            pool[idx] = (tag, value)
            idx += 2
            continue
        elif tag == 15:
            value = data[pos:pos+3].hex()
            pos += 3
        else:
            raise ValueError((idx, tag))
        pool[idx] = (tag, value)
        idx += 1
    strings = [{'constant_pool_index': n, 'value': pool[x[1]][1]}
               for n, x in enumerate(pool) if x and x[0] == 8]
    refs = [pool[x[1]][1] for x in pool if x and x[0] == 7]
    return {'class_version': f'{major}.{minor}', 'strings': strings,
            'referenced_classes': refs}


def write_json(name, value):
    (OUT / name).write_text(json.dumps(value, indent=2, ensure_ascii=False) + '\n')


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    files, classes, images = [], [], []
    with ZipFile(ROOT / f'artifacts/original/mig33-{VERSION}.jar') as jar:
        for entry in jar.infolist():
            data = jar.read(entry)
            record = {'path': entry.filename, 'bytes': len(data),
                      'sha256': hashlib.sha256(data).hexdigest(),
                      'zip_timestamp': list(entry.date_time)}
            files.append(record)
            if entry.filename.endswith('.class'):
                classes.append({'path': entry.filename, **class_pool(data)})
            if data.startswith(b'\x89PNG\r\n\x1a\n'):
                width, height = struct.unpack_from('>II', data, 16)
                images.append({**record, 'width': width, 'height': height})
    draw_calls, colors, literals = [], {}, []
    for path in sorted((ROOT / 'artifacts/vineflower').rglob('*.java')):
        for line_no, line in enumerate(path.read_text().splitlines(), 1):
            source = f'{path.relative_to(ROOT)}:{line_no}'
            if re.search(r'\.(?:draw\w+|fill\w+|setColor|setFont|setClip)\(', line):
                draw_calls.append({'source': source, 'java': line.strip()})
            for match in re.finditer(r'\.setColor\((\d+)(?:, (\d+), (\d+))?\)', line):
                values = [int(v) for v in match.groups() if v is not None]
                rgb = values[0] if len(values) == 1 else (values[0] << 16) | (values[1] << 8) | values[2]
                colors.setdefault(f'#{rgb:06x}', []).append(source)
            for match in re.finditer(r'"(?:\\.|[^"\\])*"', line):
                literals.append({'source': source, 'java_literal': match.group()})
    write_json('jar-inventory.json', files)
    write_json('class-constants.json', classes)
    write_json('assets.json', images)
    write_json('draw-calls.json', draw_calls)
    write_json('colors.json', colors)
    write_json('string-locations.json', literals)
    print(f'{len(files)} entries; {len(classes)} classes; {len(images)} images; '
          f'{sum(len(c["strings"]) for c in classes)} class string constants; '
          f'{len(draw_calls)} drawing statements; {len(colors)} literal colors')
    if VERSION != '3.08':
        return
    write_json('sprite-atlas.json', [{'id': n, 'file': 'j.png', 'x': (n // 4) * 12,
                                    'y': (n % 4) * 12, 'width': 12, 'height': 12}
                                   for n in range(42)])
    cards = ''.join(f'<figure><div class="sprite" style="background-position:-{n//4*48}px -{n%4*48}px"></div><figcaption>{n}: ({n//4*12}, {n%4*12})</figcaption></figure>' for n in range(42))
    imgs = ''.join(f'<figure><img src="../artifacts/extracted/{html.escape(x["path"])}"><figcaption>{html.escape(x["path"])} · {x["width"]}×{x["height"]}</figcaption></figure>' for x in images)
    swatches = ''.join(f'<div class="swatch"><i style="background:{c}"></i><code>{c}</code> ({len(refs)} references)</div>' for c, refs in sorted(colors.items()))
    (ROOT / 'docs/assets.html').write_text(f'''<!doctype html><meta charset="utf-8"><title>mig33 3.08 original assets</title>
<style>body{{font:16px system-ui;max-width:1100px;margin:40px auto;background:#f5f6f8;color:#162334}}h1,h2{{font-weight:600}}.grid{{display:flex;flex-wrap:wrap;gap:12px}}figure{{margin:0;padding:16px;background:white;border:1px solid #ccd4df;border-radius:6px;min-width:110px}}img,.sprite{{image-rendering:pixelated}}figcaption{{font-size:12px;margin-top:10px}}.sprite{{width:48px;height:48px;background-image:url('../artifacts/extracted/j.png');background-size:528px 192px}}.swatch{{display:inline-block;width:250px;padding:8px}}i{{display:inline-block;width:32px;height:20px;margin-right:8px;border:1px solid #ccd4df}}</style>
<h1>mig33 3.08 original assets</h1><p>Extracted bytes from the verified original JAR. Sprites are shown at 4× nearest-neighbor scale; IDs and coordinates follow the client loader. This is an asset atlas, not a screenshot.</p>
<h2>Bundled images at original size</h2><div class="grid">{imgs}</div>
<h2>42 application sprites</h2><div class="grid">{cards}</div>
<h2>Literal drawing colors</h2><p>Dynamic server colors and calculated gradient intermediate colors are excluded. Full source references are in <a href="data/colors.json">colors.json</a>.</p>{swatches}''')


if __name__ == '__main__':
    main()
