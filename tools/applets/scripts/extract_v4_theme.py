#!/usr/bin/env python3
"""Extract default v4 theme TLVs and PNG resources without executing the client."""
import hashlib
import html
import json
import struct
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3] / 'research/applets/v4'
OUT = ROOT / 'artifacts/theme'
OUT.mkdir(parents=True, exist_ok=True)
raw = (ROOT / 'artifacts/extracted/default_v2').read_bytes()
marker, opcode, txn, length = struct.unpack_from('>BhhI', raw)
assert marker == 2 and opcode == 1000 and len(raw) == length + 9
pos, fields, images = 9, [], []
while pos < len(raw):
    tag, size = struct.unpack_from('>hI', raw, pos)
    pos += 6
    payload = raw[pos:pos+size]
    assert len(payload) == size
    pos += size
    item = {'tag': tag, 'length': size, 'sha256': hashlib.sha256(payload).hexdigest()}
    path = OUT / f'field-{tag:03d}.bin'
    path.write_bytes(payload)
    item['raw_path'] = str(path.relative_to(ROOT))
    if size == 4:
        item['int32'] = struct.unpack('>i', payload)[0]
        if 0 <= item['int32'] <= 0xffffff:
            item['rgb_candidate'] = f'#{item["int32"]:06x}'
    elif size == 1:
        item['int8'] = payload[0]
    # PNGs may be direct TLVs or embedded in a count/length wrapper or font blob.
    cursor, image_idx = 0, 0
    while True:
        start = payload.find(b'\x89PNG\r\n\x1a\n', cursor)
        if start < 0:
            break
        end = start + 8
        while end + 12 <= len(payload):
            chunk_size = struct.unpack_from('>I', payload, end)[0]
            kind = payload[end+4:end+8]
            end += chunk_size + 12
            if kind == b'IEND':
                break
        assert end <= len(payload) and kind == b'IEND'
        png = payload[start:end]
        png_path = OUT / f'field-{tag:03d}-{image_idx:02d}.png'
        png_path.write_bytes(png)
        width, height = struct.unpack_from('>II', png, 16)
        image = {'tag': tag, 'index': image_idx, 'offset': start, 'length': len(png),
                 'width': width, 'height': height, 'path': str(png_path.relative_to(ROOT)),
                 'sha256': hashlib.sha256(png).hexdigest()}
        images.append(image)
        item.setdefault('pngs', []).append(image)
        cursor, image_idx = end, image_idx + 1
    if size not in (1,4) and not item.get('pngs'):
        try:
            text = payload.decode('utf-8')
            if all(c.isprintable() or c in '\r\n\t' for c in text):
                item['utf8'] = text
        except UnicodeDecodeError:
            pass
    fields.append(item)
fields.sort(key=lambda x: x['tag'])
data = {'opcode': opcode, 'transaction': txn, 'fields': fields, 'images': images}
(ROOT / 'docs/data/theme.json').write_text(json.dumps(data, indent=2)+'\n')
cards = ''.join(f'<figure><img src="../{x["path"]}"><figcaption>Tag {x["tag"]}, image {x["index"]} · {x["width"]}×{x["height"]}<br><a href="../{x["path"]}">Original PNG</a></figcaption></figure>' for x in images)
swatches = ''.join(f'<tr><td>{x["tag"]}</td><td>{x["int32"]}</td><td><i style="background:{x["rgb_candidate"]}"></i>{x["rgb_candidate"]}</td></tr>' for x in fields if 'rgb_candidate' in x)
(ROOT / 'docs/assets.html').write_text(f'''<!doctype html><meta charset="utf-8"><title>mig33 4.62 theme resources</title>
<style>body{{font:16px system-ui;max-width:1100px;margin:40px auto;background:#f5f6f8;color:#162334}}.grid{{display:flex;flex-wrap:wrap;gap:12px}}figure{{margin:0;padding:16px;background:white;border:1px solid #ccd4df;overflow:auto;max-width:100%}}img{{image-rendering:pixelated;max-width:none}}figcaption{{font-size:12px;margin-top:10px}}td{{padding:5px 20px}}i{{display:inline-block;width:32px;height:20px;margin-right:8px;border:1px solid #ccd4df}}</style>
<h1>mig33 4.62 theme resources</h1><p>Exact PNG byte ranges extracted from the default_v2 packet. These are original assets, not screen mockups. Tag meaning is determined by client call sites; a 32-bit value may be a metric or flag rather than a color.</p><p><a href="data/theme.json">Full theme metadata and hashes</a> · <a href="rendering.md">Rendering specification</a></p>
<h2>Embedded images</h2><div class="grid">{cards}</div><h2>Four-byte fields (RGB previews are candidates)</h2><table><tr><th>Tag</th><th>Integer</th><th>RGB interpretation</th></tr>{swatches}</table>''')
print(f'{len(fields)} theme fields; {len(images)} PNG resources; {len(raw)} bytes parsed exactly')
