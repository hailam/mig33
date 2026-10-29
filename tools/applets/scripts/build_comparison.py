#!/usr/bin/env python3
"""Build a local visual comparison from the original-bytecode capture manifests."""
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
v3 = json.loads((ROOT/'research/applets/v3/docs/data/screenshots-240x320.json').read_text())
v4 = json.loads((ROOT/'research/applets/v4/docs/data/screenshots-240x320.json').read_text())
data = json.dumps({'v3': v3, 'v4': v4}).replace('</', '<\\/')
page = '''<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>mig33 3.08 and 4.62 visual comparison</title>
<style>:root{font-family:system-ui;color:#152536;background:#edf1f5}body{margin:32px auto;max-width:1100px;padding:0 20px}h1{font-size:28px}p{line-height:1.55}a{color:#075b8b}.controls{display:flex;gap:20px;flex-wrap:wrap;margin:24px 0}select{font:inherit;padding:8px;max-width:100%}.versions{display:grid;grid-template-columns:1fr 1fr;gap:24px}.card{padding:20px;background:white;border:1px solid #c4d0db;border-radius:8px;min-width:0}.screen{display:block;width:calc(240px * var(--scale,1.5));height:calc(320px * var(--scale,1.5));max-width:100%;object-fit:contain;object-position:top left;image-rendering:pixelated;margin:20px 0}.caption{min-height:55px;font-size:14px}.note{background:#fff6df;border-left:4px solid #cf9019;padding:12px 16px}h2{font-size:21px}code{font-size:12px;overflow-wrap:anywhere}@media(max-width:650px){.versions{grid-template-columns:1fr}.screen{--scale:1}}
</style><h1>mig33 3.08 and 4.62 visual comparison</h1>
<p>These images were rendered by the original application bytecode at a 240 × 320 Canvas size. Choose equivalent forms or explore each capture set independently.</p>
<p class="note">Emulator fonts are used. Native softkey bars and text editors are outside these images. Captures labeled <strong>fixture</strong> contain synthetic data; server-dependent historical pages have not been invented. V4 menu overlays use Login as a reproducible background, not a navigation claim.</p>
<div class="controls"><label>Compare <select id="preset"><option value="login">Login</option><option value="registration">Registration</option><option value="contacts">Contact details</option><option value="first">First run</option><option value="manual">Manual connection</option><option value="auth">Authentication prompt</option><option value="custom">Choose separately</option></select></label><label>Scale <select id="scale"><option value="1">1× pixels</option><option value="1.5" selected>1.5×</option><option value="2">2× pixels</option></select></label></div>
<div class="versions"><section class="card"><h2>mig33 3.08</h2><select id="v3" aria-label="Version 3 screen"></select><img class="screen" id="img3" alt="mig33 3.08 screen"><p class="caption" id="cap3"></p><a id="raw3">Open original PNG</a><p><a href="../v3/docs/screens.md">Screen specification</a> · <a href="../v3/docs/capture.md">Capture profile</a></p></section><section class="card"><h2>mig33 4.62</h2><select id="v4" aria-label="Version 4 screen"></select><img class="screen" id="img4" alt="mig33 4.62 screen"><p class="caption" id="cap4"></p><a id="raw4">Open original PNG</a><p><a href="../v4/docs/screens.md">Screen specification</a> · <a href="../v4/docs/capture.md">Capture profile</a></p></section></div>
<p><a href="comparison.md">Behavior and asset comparison</a> · <a href="protocol-differences.md">Protocol differences</a> · <a href="history-and-popularity.md">Release and popularity evidence</a> · <a href="../../../README.md">All references</a></p>
<script>const data=__DATA__;
const pairs={login:['15-login.png','15-login.png'],registration:['30-registration-fixture.png','30-registration-no-captcha.png'],contacts:['09-contact-details.png','09-contact-details.png'],first:['49-first-run.png','49-first-run.png'],manual:['13-manual-connection.png','13-manual-connection.png'],auth:['41-authenticate.png','41-authenticate.png']};
function draw(v){const n=v==='v3'?'3':'4',sel=document.getElementById(v),row=data[v].images.find(x=>x.file===sel.value),base=v==='v3'?'../v3/artifacts/screenshots/240x320/':'../v4/artifacts/screenshots/240x320/';if(!row)return;const img=document.getElementById('img'+n);img.src=base+row.file;img.alt=row.description;document.getElementById('cap'+n).textContent=row.description;document.getElementById('raw'+n).href=img.src;}
for(const v of ['v3','v4']){const sel=document.getElementById(v);for(const row of data[v].images){const o=document.createElement('option');o.value=row.file;o.textContent=row.file.replace('.png','');sel.append(o);}sel.onchange=()=>{document.getElementById('preset').value='custom';draw(v);};}
function choose(){const pair=pairs[document.getElementById('preset').value];if(!pair)return;['v3','v4'].forEach((v,i)=>{const found=data[v].images.some(x=>x.file===pair[i]);if(found)document.getElementById(v).value=pair[i];draw(v);});}
document.getElementById('preset').onchange=choose;document.getElementById('scale').onchange=e=>document.documentElement.style.setProperty('--scale',e.target.value);choose();</script></html>'''
# Resolve presets against actual filenames so a renamed capture cannot silently
# display a previous screen under the wrong preset.
aliases = {
    '30-registration-fixture.png': ['30-registration-fixture.png', '30-registration.png'],
    '49-first-run.png': ['49-first-run.png', '49-first-run-instructions.png'],
    '41-authenticate.png': ['41-authenticate.png', '41-authentication.png'],
}
for target, candidates in aliases.items():
    available = {x['file'] for x in v3['images']}
    replacement = next((x for x in candidates if x in available), None)
    if replacement:
        # Only the left element of each pair; preserve v4's independent filename.
        page = page.replace("['" + target + "',", "['" + replacement + "',")
(ROOT/'research/applets/shared/comparison.html').write_text(page.replace('__DATA__', data))
print(f'Comparison contains {len(v3["images"])} v3 and {len(v4["images"])} v4 captures')
