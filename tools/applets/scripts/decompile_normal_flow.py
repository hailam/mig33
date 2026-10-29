#!/usr/bin/env python3
"""Make an ANALYSIS-ONLY JAR with exception tables omitted.

Never run the output: error behavior is deliberately incomplete. Bytecode
instructions, signatures and constant pools are unchanged. This helps decompilers
read obfuscated normal flow; original javap output remains authoritative.
"""
import json
import struct
from pathlib import Path
from zipfile import ZipFile

ROOT = Path(__file__).resolve().parents[3] / 'research/applets/v4'
TARGET = ROOT / 'artifacts/analysis-only'
TARGET.mkdir(parents=True, exist_ok=True)
changes = []


def transform(data, name):
    def u2(pos): return struct.unpack_from('>H', data, pos)[0]
    def u4(pos): return struct.unpack_from('>I', data, pos)[0]
    count, idx, pos, names = u2(8), 1, 10, {}
    while idx < count:
        tag, pos = data[pos], pos + 1
        if tag == 1:
            size = u2(pos)
            names[idx] = data[pos+2:pos+2+size].decode('utf-8', errors='replace')
            pos += size + 2
        elif tag in (7,8,16,19,20): pos += 2
        elif tag in (3,4,9,10,11,12,17,18): pos += 4
        elif tag in (5,6): pos += 8; idx += 1
        elif tag == 15: pos += 3
        else: raise ValueError(tag)
        idx += 1
    pos += 6
    pos += 2 + 2*u2(pos)
    output = bytearray(data[:pos])
    for section in ('fields','methods'):
        n = u2(pos)
        output += data[pos:pos+2]
        pos += 2
        for _ in range(n):
            member = names[u2(pos+2)] + names[u2(pos+4)]
            attr_count = u2(pos+6)
            output += data[pos:pos+8]
            pos += 8
            for _ in range(attr_count):
                tag, length = u2(pos), u4(pos+2)
                payload = data[pos+6:pos+6+length]
                if section == 'methods' and names[tag] == 'Code':
                    code_len = struct.unpack_from('>I', payload, 4)[0]
                    exc_pos = 8 + code_len
                    exc_count = struct.unpack_from('>H', payload, exc_pos)[0]
                    if exc_count:
                        changes.append({'class': name, 'method': member, 'removed_handlers': exc_count})
                        payload = payload[:exc_pos] + b'\0\0' + payload[exc_pos+2+8*exc_count:]
                        assert payload[8:8+code_len] == data[pos+14:pos+14+code_len]
                output += struct.pack('>HI', tag, len(payload)) + payload
                pos += 6 + length
    output += data[pos:]
    return output


with ZipFile(ROOT / 'artifacts/original/mig33-4.62.jar') as src, ZipFile(TARGET / 'DO-NOT-RUN-no-exceptions.jar', 'w') as dst:
    for entry in src.infolist():
        data = src.read(entry)
        dst.writestr(entry.filename, transform(data, entry.filename) if entry.filename.endswith('.class') else data)
(ROOT / 'docs/data/normal-flow-transformations.json').write_text(json.dumps(changes, indent=2)+'\n')
(TARGET / 'README.txt').write_text('ANALYSIS ONLY. Do not execute this JAR.\nCode exception-handler tables have been removed solely to aid decompilation.\nError paths are incomplete. Original bytecode instructions are unchanged.\nOriginal JAR and javap disassembly remain authoritative.\n')
print(f'{len(changes)} methods transformed for normal-flow reading only')
