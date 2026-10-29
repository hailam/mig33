#!/usr/bin/env python3
"""Offline mig33 3.08 packet codec. Never opens network connections."""
import argparse
import json
import struct
from pathlib import Path

DATA = Path(__file__).resolve().parents[3] / 'research/applets/v3/docs/data'


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
    fixtures = json.loads((DATA / 'protocol-fixtures.json').read_text())
    joined = b''
    for item in fixtures['packets']:
        fields = {f['tag']: value(f['type'], f['value']) for f in item['fields']}
        reference = bytes.fromhex(item['original_java_hex'])
        parsed = decode_many(reference)
        expected = (item['opcode'], item['transaction'], fields)
        assert parsed == [expected], item['name']
        encoded = encode(item['opcode'], item['transaction'], fields.items())
        assert decode_many(encoded) == [expected], item['name']
        # Preserve original Hashtable enumeration order to obtain identical bytes.
        original_order = list(parsed[0][2].items())
        assert encode(item['opcode'], item['transaction'], original_order) == reference
        joined += reference
    assert len(decode_many(joined)) == len(fixtures['packets'])
    for sample in fixtures['java_hash_vectors']:
        assert java_hash(sample['input']) == sample['hash']
    assert challenge_response_ascii('nonce-42', 'PaSsWord') == fixtures['challenge_ascii']['result']
    malformed = [b'\x02', b'\x03\0\2\0\1\0\0\0\0',
                 bytes.fromhex('0200020001ffffffff'),
                 bytes.fromhex('0200020001000000060001ffffffff'),
                 bytes.fromhex('020002000100000007000100000002ff')]
    for data in malformed:
        try:
            decode_many(data)
        except ValueError:
            pass
        else:
            raise AssertionError('accepted invalid packet')
    assert decode_many(encode(1, 0, [(1, b'first'), (1, b'last')]))[0][2][1] == b'last'
    print(f"Validated {len(fixtures['packets'])} original-JAR packet fixtures, concatenated framing, Java hash, duplicate fields and malformed-length rejection.")


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--self-test', action='store_true')
    parser.add_argument('--decode-hex', help='decode one or more packet hex strings')
    args = parser.parse_args()
    if args.self_test:
        self_test()
    elif args.decode_hex:
        print(json.dumps([{'opcode': op, 'transaction': tx,
                           'fields': {str(tag): data.hex() for tag, data in fields.items()}}
                          for op, tx, fields in decode_many(bytes.fromhex(args.decode_hex))], indent=2))
    else:
        parser.print_help()
