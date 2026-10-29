#!/usr/bin/env python3
"""Validate original-byte integrity, manifests, PNG chunks and local doc links."""
import hashlib
import json
import re
import struct
import zlib
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote, urlsplit
from zipfile import ZipFile

ROOT = Path(__file__).resolve().parents[3]
APPLETS = ROOT/'research/applets'
V3, V4 = APPLETS/'v3', APPLETS/'v4'
TOOLS = ROOT/'tools/applets/bin'
REPORT_PATH = APPLETS/'validation.json'
results = []


def check_png(path, size=None):
    data = path.read_bytes()
    assert data[:8] == b'\x89PNG\r\n\x1a\n', path
    if size:
        assert struct.unpack_from('>II', data, 16) == size, path
    pos = 8
    while pos < len(data):
        length = struct.unpack_from('>I', data, pos)[0]
        chunk = data[pos+4:pos+8+length]
        crc = struct.unpack_from('>I', data, pos+8+length)[0]
        assert zlib.crc32(chunk) & 0xffffffff == crc, path
        pos += length + 12
        if chunk[:4] == b'IEND':
            break
    assert pos == len(data), path


builds = [('v3', '3.08', '2b416199dca5f6593eefe8fa1c9b84ec4be6cdc36f68bcf17e1a94193bfd2934',35),
          ('v4', '4.62', 'dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c',26)]
for directory, version, digest, class_count in builds:
    base = APPLETS / directory
    jar_path = base / f'artifacts/original/mig33-{version}.jar'
    assert hashlib.sha256(jar_path.read_bytes()).hexdigest() == digest
    with ZipFile(jar_path) as jar:
        assert jar.testzip() is None
        classes = [x for x in jar.namelist() if x.endswith('.class')]
        assert len(classes) == class_count
        for name in jar.namelist():
            assert (base/'artifacts/extracted'/name).read_bytes() == jar.read(name)
        for name in classes:
            assert (base/'artifacts/vineflower'/name.replace('.class','.java')).is_file()
            assert (base/'artifacts/decompiled'/name.replace('.class','.java')).is_file()
            assert (base/'artifacts/bytecode'/(name[:-6].replace('/','.')+'.txt')).is_file()
    manifest_path = base/'docs/data/screenshots-240x320.json'
    screenshot_directory = base/'artifacts/screenshots/240x320'
    manifest = json.loads(manifest_path.read_text())
    assert manifest['original_jar_sha256'] == digest
    for item in manifest['images']:
        path = screenshot_directory/item['file']
        assert hashlib.sha256(path.read_bytes()).hexdigest() == item['sha256'], path
        check_png(path, (240,320))
    results.append({'version': version, 'jar_sha256': digest, 'classes':class_count,
                    'screenshots':len(manifest['images']), 'original_resources_match':True})

theme = json.loads((V4/'docs/data/theme.json').read_text())
assert len(theme['fields']) == 148 and len(theme['images']) == 9
for entry in theme['fields']:
    assert hashlib.sha256((V4/entry['raw_path']).read_bytes()).hexdigest() == entry['sha256']
for entry in theme['images']:
    path=V4/entry['path']
    assert hashlib.sha256(path.read_bytes()).hexdigest() == entry['sha256']
    check_png(path, (entry['width'],entry['height']))

def active_documents(suffix):
    """Current authored references, excluding immutable captures and run records."""
    excluded = [ROOT/'research/architecture/evidence', ROOT/'research/acceptance/v4/runs']
    excluded += [version/section for version in (V3, V4) for section in ('artifacts', 'discovery')]
    paths = set()
    for directory in (ROOT/'docs', ROOT/'research', ROOT/'tools/applets'):
        for path in directory.rglob('*' + suffix):
            if path.is_file() and not any(path.is_relative_to(base) for base in excluded):
                paths.add(path)
    if suffix == '.md':
        paths.add(ROOT/'README.md')
    return sorted(paths)


def local_destination(document, target):
    target = target.strip('<>')
    parts = urlsplit(target)
    if parts.scheme or parts.netloc or not parts.path:
        return None
    return document.parent/unquote(parts.path)


broken=[]
documents=active_documents('.md')
for path in documents:
    content=path.read_text()
    for target in re.findall(r'\[[^\]]*\]\(([^)]+)\)',content):
        destination = local_destination(path, target)
        if destination is None:
            continue
        if destination.resolve() == REPORT_PATH.resolve():
            continue  # This successful run creates the linked report itself.
        if not destination.exists():
            broken.append({'document':str(path.relative_to(ROOT)),'target':target})
assert not broken, json.dumps(broken,indent=2)

class LocalLinks(HTMLParser):
    def __init__(self, path):
        super().__init__()
        self.path = path

    def handle_starttag(self, tag, attrs):
        for name, value in attrs:
            if name not in ('href', 'src') or not value:
                continue
            destination = local_destination(self.path, value)
            if destination is None:
                continue
            assert destination.exists(), (self.path, value)


html_documents = active_documents('.html') + [
    V3/'artifacts/screenshots/240x320/index.html',
    V4/'artifacts/screenshots/240x320/index.html',
]
for path in html_documents:
    LocalLinks(path).feed(path.read_text())

# Historical screenshots remain unchanged evidence; runtime placeholders are
# checked by Go catalog tests and are never substituted for these originals.
historical_images = {}
for catalog_path, array, path_key, base in [
    ('research/applets/v4/docs/data/historical-pages/catalog.json', 'images', 'file', V4/'artifacts/historical-pages'),
    ('research/applets/v4/docs/data/historical-formatting-sources.json', 'images', 'local_path', ROOT),
    ('research/applets/v4/docs/data/historical-product-catalog.json', 'assets', 'path', ROOT),
    ('research/applets/v4/docs/data/games-history.json', 'images', 'path', ROOT),
]:
    historical = json.loads((ROOT/catalog_path).read_text())
    for entry in historical[array]:
        if path_key not in entry:
            continue  # Explicit failed retrievals carry no retained file.
        path = base/entry[path_key]
        assert path.is_file(), path
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        assert digest == entry['sha256'], path
        historical_images[str(path.relative_to(ROOT))] = digest
assert len(historical_images) == 120
assert len(set(historical_images.values())) == 120

product_evidence = json.loads((V4/'docs/data/historical-product-catalog.json').read_text())
source_urls = {source['id']: source['url'] for source in product_evidence['sources']}
name_sources = {}
for item in product_evidence['catalog_items']:
    name_sources.setdefault(item['name'], set()).update(source_urls[source] for source in item['source_ids'])
assert len(product_evidence['catalog_items']) == 74 and len(name_sources) == 73
runtime_products = json.loads((ROOT/'internal/catalog/products.json').read_text())
assert len(runtime_products) == len({item['id'] for item in runtime_products}) == 78
for item in runtime_products:
    if item['name_evidence'] == 'recovered_name':
        assert item['name'] in name_sources, item['id']
        assert item['source_urls'] and set(item['source_urls']) <= name_sources[item['name']], item['id']

# The comparison embeds its data, so stale manifests must not pass unnoticed.
comparison = (APPLETS/'shared/comparison.html').read_text()
embedded = json.loads(re.search(r'const data=(.*?);\nconst pairs=', comparison).group(1))
for version, directory in [('v3', 'v3'), ('v4', 'v4')]:
    manifest = json.loads((APPLETS/directory/'docs/data/screenshots-240x320.json').read_text())
    assert embedded[version] == manifest, version
pairs = re.search(r'const pairs=\{(.*?)\};', comparison).group(1)
for left, right in re.findall(r"\['([^']+)','([^']+)'\]", pairs):
    assert left in {row['file'] for row in embedded['v3']['images']}, left
    assert right in {row['file'] for row in embedded['v4']['images']}, right

tool_manifest = json.loads((TOOLS/'manifest.json').read_text())
for entry in tool_manifest['tools']:
    data = (TOOLS/entry['file']).read_bytes()
    assert len(data) == entry['bytes'], entry['file']
    assert hashlib.sha256(data).hexdigest() == entry['sha256'], entry['file']

# The endpoint catalog must cover the raw strings while distinguishing scheme
# fragments and the internal browser cache key from actual network requests.
endpoint_data = V4/'docs/data'
raw_endpoints = json.loads((endpoint_data/'protocol-endpoints.json').read_text())
endpoint_catalog = json.loads((endpoint_data/'endpoints-classified.json').read_text())
route_notes = json.loads((endpoint_data/'endpoint-route-notes.json').read_text())
classified = endpoint_catalog['literals']
excluded = endpoint_catalog['excluded']
assert len(classified) == 59 and len(excluded) == 3
assert {x['literal'] for x in classified + excluded} == {x['literal'] for x in raw_endpoints}
assert {x['literal'] for x in excluded} == {'http://', 'https://', 'socket://'}
assert sum(x['kind'] == 'embedded_page_template' for x in classified) == 52
assert sum(x['kind'] == 'internal_browser_home_key' for x in classified) == 1
for row in classified:
    if '%s' in row['literal']:
        assert row['construction_notes'] == route_notes['routes'][row['literal']]
for row in raw_endpoints:
    for ref in row['references']:
        source_line = (ROOT/ref['path']).read_text().splitlines()[ref['line']-1]
        assert row['literal'] in source_line, (row['literal'], ref)

outgoing = json.loads((endpoint_data/'protocol-outgoing.json').read_text())['operations']
assert len(outgoing) == 56 and len({x['opcode'] for x in outgoing}) == 56
assert {x['opcode'] for x in outgoing if x['opcode'] >= 1000} == {1001,1003,1005}

json_count=0
for directory in (V3/'docs/data', V4/'docs/data'):
    for path in directory.rglob('*.json'):
        json.loads(path.read_text())
        json_count+=1

report={'builds':results,'theme_fields':148,'theme_images':9,'documents_checked':len(documents),
        'historical_images_verified':len(historical_images),
        'html_documents_checked':len(html_documents),'tool_binaries_checked':len(tool_manifest['tools']),
        'v4_endpoint_literal_references_checked':sum(len(x['references']) for x in raw_endpoints),
        'v4_endpoint_url_path_literals':len(classified),'v4_page_templates':52,
        'json_files_parsed':json_count,'broken_local_document_links':broken,
        'scope':'Original bytes, extracted resources, decompiler/disassembly coverage, image hashes/PNG CRCs and local links; not live backend behavior.'}
REPORT_PATH.write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report,indent=2))
