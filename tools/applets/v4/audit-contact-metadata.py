#!/usr/bin/env python3
"""Check reviewed status/avatar data-flow anchors against the unchanged v4 JAR.

These are narrow, hand-reviewed semantic chains, not a general data-flow proof.
Fresh javap output is temporary; only the small checked evidence record is saved.
"""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import runpy
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[3]
BASE = ROOT / 'research/applets/v4'
JAR_SHA = 'dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c'
DISPATCH = 'bc.a:([Lay;)V'
SETTER = 't.a:(ILjava/lang/String;II)V'
UPDATE = 'au.a:(ILjava/lang/String;Ljava/lang/String;)Lt;'
PAINT = 'au.a:(Ljavax/microedition/lcdui/Graphics;Z)I'
ROW = 'au.a:(IIZLt;Ljava/lang/String;Ljava/lang/String;ILjavax/microedition/lcdui/Graphics;ILjava/lang/String;ZI)Z'
IMAGE = 'au.a:(Ljavax/microedition/lcdui/Graphics;Ljava/lang/String;IIZ)V'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--java-home', type=Path)
    parser.add_argument('--check', action='store_true')
    parser.add_argument('--output', type=Path, default=BASE / 'docs/data/protocol-audit-contact-metadata.json')
    args = parser.parse_args()
    jar = BASE / 'artifacts/original/mig33-4.62.jar'
    assert hashlib.sha256(jar.read_bytes()).hexdigest() == JAR_SHA
    javap = str(args.java_home / 'bin/javap') if args.java_home else shutil.which('javap')
    assert javap, 'Install a JDK or pass --java-home'
    parse = runpy.run_path(str(ROOT / 'tools/applets/v4/audit-incoming.py'))['methods']
    methods = {}
    with tempfile.TemporaryDirectory(prefix='mig33-contact-audit-') as temp:
        for owner in ('bc', 't', 'au'):
            path = Path(temp) / f'{owner}.txt'
            path.write_bytes(subprocess.check_output([javap, '-classpath', str(jar), '-c', '-p', '-s', owner]))
            methods.update((method.label, method) for method in parse(path))

    def anchor(label, checks):
        instructions = {instruction.offset: instruction for instruction in methods[label].instructions}
        result = {'method': label, 'instructions': []}
        for offset, op, operand in checks:
            instruction = instructions[offset]
            assert instruction.op == op and operand in instruction.args, (label, offset, instruction, op, operand)
            result['instructions'].append({'offset': offset, 'instruction': f'{op} {instruction.args}'.rstrip()})
        return result

    dispatch = methods[DISPATCH].switches[min(methods[DISPATCH].switches)]
    assert {opcode: dispatch[str(opcode)] for opcode in (402, 421, 423)} == {402: 11135, 421: 5941, 423: 1023}
    setter = next(switch for switch in methods[SETTER].switches.values() if '91' in switch)
    assert setter['2'] == 329 and setter['91'] == 228
    model = {
        'status': {
            'field': 't.h', 'setter_key': 2, 'meaning': 'status text',
            'anchors': [
                anchor(SETTER, [(331, 'putfield', 'Field h:Ljava/lang/String;')]),
                anchor(PAINT, [(2939, 'getfield', 'Field t.h:Ljava/lang/String;')]),
                anchor(ROW, [(318, 'aload', '6'), (336, 'invokevirtual', 'Graphics.drawString:')]),
            ],
        },
        'avatar': {
            'field': 't.p', 'setter_key': 91, 'meaning': 'avatar image/cache key',
            'anchors': [
                anchor(SETTER, [(230, 'putfield', 'Field p:Ljava/lang/String;')]),
                anchor(PAINT, [(2955, 'getfield', 'Field t.p:Ljava/lang/String;')]),
                anchor(ROW, [(362, 'aload', '10'), (379, 'invokespecial', 'a:(Ljavax/microedition/lcdui/Graphics;Ljava/lang/String;IIZ)V')]),
                anchor(IMAGE, [(45, 'invokestatic', 'aj.d:(Ljava/lang/String;)Ljava/lang/String;'),
                               (49, 'invokestatic', 'aj.c:(Ljava/lang/String;Z)Ljavax/microedition/lcdui/Image;'),
                               (134, 'invokevirtual', 'Graphics.drawImage:'),
                               (150, 'invokevirtual', 'aj.d:(Ljava/lang/String;Ljava/lang/String;)V')]),
            ],
        },
    }
    fields = [
        {'opcode': 402, 'tag': 27, 'semantic': 'status', 'model': '/model/status', 'anchors': [
            anchor(DISPATCH, [(11834, 'bipush', '27'), (11844, 'iconst_2', ''),
                              (11848, 'bipush', '27'), (11850, 'invokevirtual', 'ay.f:(S)Ljava/lang/String;'),
                              (11853, 'invokevirtual', 't.a:(ILjava/lang/String;)V')])]},
        {'opcode': 402, 'tag': 28, 'semantic': 'avatar', 'model': '/model/avatar', 'anchors': [
            anchor(DISPATCH, [(11859, 'bipush', '28'), (11869, 'bipush', '91'),
                              (11874, 'bipush', '28'), (11876, 'invokevirtual', 'ay.f:(S)Ljava/lang/String;'),
                              (11879, 'invokevirtual', 't.a:(ILjava/lang/String;)V')])]},
        {'opcode': 421, 'tag': 2, 'semantic': 'status', 'model': '/model/status', 'anchors': [
            anchor(DISPATCH, [(5986, 'iconst_2', ''), (5987, 'invokevirtual', 'ay.f:(S)Ljava/lang/String;'),
                              (6002, 'aload', '8'), (6004, 'aconst_null', ''),
                              (6005, 'invokevirtual', 'au.a:(ILjava/lang/String;Ljava/lang/String;)Lt;')]),
            anchor(UPDATE, [(74, 'aload_2', ''), (80, 'iconst_2', ''), (88, 'aload_2', ''),
                            (95, 'invokevirtual', 't.a:(ILjava/lang/String;)V')])]},
        {'opcode': 423, 'tag': 2, 'semantic': 'avatar', 'model': '/model/avatar', 'anchors': [
            anchor(DISPATCH, [(1068, 'iconst_2', ''), (1069, 'invokevirtual', 'ay.f:(S)Ljava/lang/String;'),
                              (1084, 'aconst_null', ''), (1085, 'aload', '8'),
                              (1087, 'invokevirtual', 'au.a:(ILjava/lang/String;Ljava/lang/String;)Lt;')]),
            anchor(UPDATE, [(110, 'aload_3', ''), (116, 'bipush', '91'), (125, 'aload_3', ''),
                            (132, 'invokevirtual', 't.a:(ILjava/lang/String;)V')])]},
    ]
    result = {
        'source': {'jar': str(jar.relative_to(ROOT)), 'jar_sha256': JAR_SHA,
                   'command': 'javap -classpath <original-jar> -c -p -s <class>'},
        'scope': 'Reviewed incoming contact status/avatar chains; checked instruction anchors, not full interprocedural verification or historical-server execution.',
        'dispatch_entries': {str(opcode): dispatch[str(opcode)] for opcode in (402, 421, 423)},
        'setter_switch_targets': {'2': setter['2'], '91': setter['91']},
        'model': model, 'fields': fields,
        'unchanged': ['203 tag28 remains own status', '500 tag10 remains message avatar',
                      '601 remains outgoing status', '602 remains outgoing avatar with original duplicate-tag1 behavior',
                      '603 tag1 remains own status and tag2 own avatar', 'Packet opcodes, tags, raw widths and transport framing remain unchanged'],
    }
    raw = json.dumps(result, ensure_ascii=False, indent=2) + '\n'
    if args.check:
        assert args.output.read_text() == raw, 'Contact metadata audit output is stale'
    else:
        args.output.write_text(raw)
    print('Checked original402 tags27/28 and421/423 status/avatar semantic chains')


if __name__ == '__main__':
    main()
