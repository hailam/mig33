#!/usr/bin/env -S uv run --no-project python
"""Recover v3 gradient and transcript literals omitted by the setColor index."""
from hashlib import sha256
import json
import os
from pathlib import Path
import re
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parents[3]
V3 = ROOT / 'research/applets/v3'
JAR = V3 / 'artifacts/original/mig33-3.08.jar'
DIGEST = '2b416199dca5f6593eefe8fa1c9b84ec4be6cdc36f68bcf17e1a94193bfd2934'
JAVAP = os.environ.get('JAVAP_BIN', '/opt/homebrew/opt/openjdk@21/bin/javap')


def main():
    payload = JAR.read_bytes()
    if sha256(payload).hexdigest() != DIGEST:
        raise SystemExit('Unexpected v3 JAR; refusing to extract theme evidence')
    bytecode = {}
    class_hashes = {}
    with zipfile.ZipFile(JAR) as archive:
        for name in ('com.projectgoth.Roshan.d', 'ab', 't'):
            class_hashes[name] = sha256(archive.read(name.replace('.', '/') + '.class')).hexdigest()
            bytecode[name] = subprocess.check_output(
                [JAVAP, '-c', '-p', '-s', '-classpath', str(JAR), name], text=True)
            path = V3 / 'artifacts/bytecode' / (name.replace('.', '/') + '.javap.txt')
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(bytecode[name])

    def method(owner, signature):
        body = bytecode[owner].split('  ' + signature + '\n', 1)[1]
        body = re.split(r'\n  \S', body, maxsplit=1)[0]
        instructions = {}
        for match in re.finditer(r'^\s+(\d+):\s+([a-z][a-z0-9_]*)([^\n]*)', body, re.M):
            instructions[int(match[1])] = (match[2], match[3].strip())
        return instructions

    def literal(instructions, offset):
        op, operand = instructions[offset]
        if op.startswith('iconst_'):
            return -1 if op == 'iconst_m1' else int(op.removeprefix('iconst_'))
        if op in ('bipush', 'sipush'):
            return int(operand)
        if op in ('ldc', 'ldc_w'):
            return int(operand.split('// int ', 1)[1])
        raise ValueError(f'Expected integer literal at {offset}, found {op} {operand}')

    def source(owner, signature, descriptor, offsets, call_offset=None):
        evidence = {
            'class': owner,
            'class_sha256': class_hashes[owner],
            'method': signature,
            'descriptor': descriptor,
            'bytecode_offsets': offsets,
            'disassembly': 'artifacts/bytecode/' + owner.replace('.', '/') + '.javap.txt',
        }
        if call_offset is not None:
            evidence['gradient_call_offset'] = call_offset
        return evidence

    colors = {}
    evidence = {}
    gradients = [
        ('ab', 'public void a(javax.microedition.lcdui.Graphics);',
         '(Ljavax/microedition/lcdui/Graphics;)V',
         [('stripTop', [20, 23, 26]), ('stripBottom', [29, 32, 35])], 46),
        ('t', 'private int a(javax.microedition.lcdui.Graphics, java.lang.String, int, int, boolean);',
         '(Ljavax/microedition/lcdui/Graphics;Ljava/lang/String;IIZ)I',
         [('buttonTop', [42, 45, 48]), ('buttonBottom', [51, 54, 57])], 60),
        ('ab', 'public int a(int, int, boolean, javax.microedition.lcdui.Graphics);',
         '(IIZLjavax/microedition/lcdui/Graphics;)I',
         [('activeTabTop', [184, 187, 190]), ('activeTabBottom', [193, 196, 199])], 210),
        ('ab', 'public int a(int, int, boolean, javax.microedition.lcdui.Graphics);',
         '(IIZLjavax/microedition/lcdui/Graphics;)I',
         [('unreadTop', [261, 264, 267]), ('unreadBottom', [269, 272, 275])], 284),
        ('ab', 'public int a(int, int, boolean, javax.microedition.lcdui.Graphics);',
         '(IIZLjavax/microedition/lcdui/Graphics;)I',
         [('inactiveTabTop', [292, 295, 298]), ('inactiveTabBottom', [301, 304, 307])], 318),
    ]
    for owner, signature, descriptor, endpoints, call_offset in gradients:
        instructions = method(owner, signature)
        assert instructions[call_offset][0] == 'invokestatic'
        for role, offsets in endpoints:
            rgb = [literal(instructions, offset) for offset in offsets]
            assert all(0 <= channel <= 255 for channel in rgb)
            colors[role] = '#' + ''.join(f'{channel:02x}' for channel in rgb)
            evidence[role] = source(owner, signature, descriptor, offsets, call_offset)
            evidence[role]['rgb'] = rgb

    owner = 'com.projectgoth.Roshan.d'
    signature = 'public static int a(int);'
    instructions = method(owner, signature)
    transcript = []
    transcript_evidence = []
    for category, offset in enumerate([44, 47, 50, 53, 56, 59, 62]):
        value = literal(instructions, offset)
        transcript.append(f'#{value:06x}')
        item = source(owner, signature, '(I)I', [offset])
        item.update(category=category, decimal=value, switch_offset=1)
        transcript_evidence.append(item)

    result = {
        'original_jar_sha256': DIGEST,
        'method': 'Integer operands read from javap disassembly of the hash-verified original JAR; no prose or app theme input.',
        'colors': colors,
        'color_evidence': evidence,
        'transcriptColors': transcript,
        'transcript_evidence': transcript_evidence,
    }
    output = V3 / 'docs/data/theme-bytecode-evidence.json'
    output.write_text(json.dumps(result, indent=2) + '\n')
    print(f'Extracted {len(colors)} gradient endpoints and {len(transcript)} transcript colors to {output.relative_to(ROOT)}')


if __name__ == '__main__':
    main()
