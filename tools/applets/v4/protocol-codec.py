#!/usr/bin/env python3
"""Offline mig33 4.62 packet codec. Never opens network connections."""
import argparse
import json
import struct
from pathlib import Path

APPLETS = Path(__file__).resolve().parents[3] / 'research/applets'


def encode(opcode, transaction, fields):
    """fields: iterable (tag, bytes); repeated tags use the final value."""
    fields = dict(fields)
    body = b''.join(struct.pack('>hi', tag, len(value)) + value for tag, value in fields.items())
    return struct.pack('>Bhhi', 2, opcode, transaction, len(body)) + body


def decode_many(data):
    packets = []
    offset = 0
    while offset < len(data):
        if len(data) - offset < 9:
            raise ValueError('truncated packet header')
        magic, opcode, transaction, size = struct.unpack_from('>Bhhi', data, offset)
        if magic != 2 or size < 0 or offset + 9 + size > len(data):
            raise ValueError('invalid magic or body length')
        offset += 9
        end = offset + size
        fields = {}
        while offset < end:
            if end - offset < 6:
                raise ValueError('truncated field header')
            tag, field_size = struct.unpack_from('>hi', data, offset)
            offset += 6
            if field_size < 0 or offset + field_size > end:
                raise ValueError('invalid field length')
            fields[tag] = data[offset:offset + field_size]
            offset += field_size
        packets.append((opcode, transaction, fields))
    return packets


def value(kind, content):
    if kind == 'TA':
        return b'\0'.join(part.encode('utf-8') for part in content)
    if kind == 'T':
        return content.encode('utf-8')
    if kind == 'X':
        return bytes.fromhex(content)
    return struct.pack({'B': '>b', 'S': '>h', 'I': '>i', 'L': '>q'}[kind], content)


def java_hash(text):
    """Java String.hashCode, including UTF-16 surrogate pairs and signed result."""
    raw = text.encode('utf-16-be', 'surrogatepass')
    result = 0
    for i in range(0, len(raw), 2):
        result = (31 * result + int.from_bytes(raw[i:i + 2], 'big')) & 0xffffffff
    return result if result < 0x80000000 else result - 0x100000000


def challenge_response_ascii(challenge, password):
    """ASCII-password fixture helper. Unicode lowercasing must match target Java."""
    if not password.isascii():
        raise ValueError('use target Java lowercasing for non-ASCII passwords')
    return java_hash(challenge + password.lower())


def self_test():
    fixtures = json.loads((APPLETS / 'v4/docs/data/protocol-fixtures.json').read_text())
    baseline = json.loads((APPLETS / 'v3/docs/data/protocol-fixtures.json').read_text())
    v3 = {p['name']: p for p in baseline['packets']}
    joined = b''
    identical = []
    for item in fixtures['packets']:
        fields = {f['tag']: value(f['type'], f['value']) for f in item['fields']}
        raw = bytes.fromhex(item['original_java_hex'])
        parsed = decode_many(raw)
        assert parsed == [(item['opcode'], item['transaction'], fields)], item['name']
        assert encode(item['opcode'], item['transaction'], parsed[0][2].items()) == raw
        if item['name'] in v3:
            assert item['original_java_hex'] == v3[item['name']]['original_java_hex'], item['name']
            identical.append(item['name'])
        joined += raw
    assert len(decode_many(joined)) == len(fixtures['packets'])
    print(f"Validated {len(fixtures['packets'])} original v4 packets; {len(identical)} common packets byte-identical to original v3: {', '.join(identical)}.")


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--self-test', action='store_true')
    args = parser.parse_args()
    if args.self_test:
        self_test()
    else:
        parser.print_help()
