#!/usr/bin/env python3
"""Check the original v4 outgoing catalog, constructor coverage and builder probes.

Run verify-protocol-evidence.py first to regenerate original javap -c -p -s
evidence. This script checks manually reviewed field-write offsets against those
instructions, exhaustively inventories ay(short) sites and generic bc callers,
and executes actual original bc builders with an isolated queue. It does not
start a MIDlet, a transport or a worker, or execute the transformed analysis JAR.
Human meaning/response arrows are not inferred from field types automatically.
"""
from __future__ import annotations

import argparse
from collections import Counter
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess

ROOT = Path(__file__).resolve().parents[3]
BASE = ROOT / 'research/applets/v4'
SHA = 'dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c'
DATA = BASE / 'docs/data'
TYPES = {'B': 'B', 'S': 'S', 'I': 'I', 'L': 'J', 'T': 'Ljava/lang/String;',
         'X': '[B', 'TA': '[Ljava/lang/String;'}
STR = 'Ljava/lang/String;'
DISPATCH = 'a:([Lay;)V'
COMMAND = 'commandAction:(Ljavax/microedition/lcdui/Command;Ljavax/microedition/lcdui/Displayable;)V'


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def parse(path):
    result, current = [], None
    for line_number, line in enumerate(path.read_text().splitlines(), 1):
        if re.match(r'^  (?:public )?static \{\};$', line):
            current = dict(owner=path.stem, declaration=line.strip(), name='<clinit>', instructions=[])
            result.append(current)
        elif re.match(r'^  .*\(.*\);$', line):
            declaration = line.strip()
            current = dict(owner=path.stem, declaration=declaration, name=declaration.split('(')[0].split()[-1], instructions=[])
            result.append(current)
        elif current and line.strip().startswith('descriptor:'):
            current['descriptor'] = line.strip().split(': ', 1)[1]
        elif current and (match := re.match(r'\s*(\d+): ([a-z][a-z_0-9]*)\s*(.*)', line)):
            current['instructions'].append(dict(offset=int(match[1]), op=match[2], args=match[3], line=line_number))
    return result


def integer(ins):
    if ins['op'].startswith('iconst_'):
        return -1 if ins['op'] == 'iconst_m1' else int(ins['op'][-1])
    if ins['op'] in ('sipush', 'bipush'):
        return int(ins['args'].split()[0])
    return None


def field(tag, kind, offset, value, condition=None):
    result = dict(tag=tag, type=kind, offset=offset, value=value)
    if condition:
        result['when'] = condition
    return result


def builder(method, ctor, fields, note=None, owner='bc'):
    result = dict(class_name=owner, method=method, constructor_offset=ctor,
                  fields=[field(*item) for item in fields])
    if note:
        result['note'] = note
    return result


# These annotations are manual source reviews, not generated semantic guesses.
# Each bytecode offset is independently checked below for the expected overload.
def reviews():
    single = lambda name, tag, value: builder(f'{name}:({STR})V', 8, [(tag, 'T', 18, value)])
    empty = builder('a:(S)V', 6, [])
    text = lambda value: builder(f'a:(S{STR})V', 6, [(1, 'T', 16, value)])
    number = lambda value: builder('a:(SI)V', 6, [(1, 'I', 16, value)])
    member = builder(f'a:(S{STR}{STR})V', 6, [(1, 'T', 16, 'argument3 room'), (2, 'T', 29, 'argument2 target')])
    rows = {
        2: [empty, builder('run:()V', 2103, [(1, 'I', 2114, '1 automatic heartbeat')])],
        7: [builder('a:(I)V', 7, [(1, 'I', 17, 'argument1 topic')])],
        9: [builder('a:(BS)V', 7, [(1, 'B', 17, 'argument1 accept/decline'), (2, 'S', 30, 'argument2 prompt transaction')])],
        10: [builder(DISPATCH, 14481, [(1, 'S', 14491, '1'), (2, 'T', 14505, 'one element of aj.l properties')], 'After 203, loops properties when response tag29 absent or byte1.')],
        11: [builder(f'a:(B{STR}{STR})V', 6, [(1, 'B', 19, 'argument1 store type'), (2, 'T', 33, 'argument2 key'), (3, 'T', 51, 'argument3 value', 'argument3 != null')])],
        17: [builder(f'd:({STR})V', 7, [(1, 'T', 17, 'argument1 entered text')])],
        101: [builder('a:()V', 7, [(1, 'I', 19, 'aj.e width'), (1, 'I', 34, 'aj.f height overwrites width')])],
        103: [builder(f'a:({STR*5})V', 33, [(1, 'T', 43, 'argument1 username'), (2, 'T', 56, 'argument2 password'), (3, 'T', 69, 'argument3 mobile'), (4, 'T', 83, 'argument4 challenge identifier'), (5, 'T', 97, 'argument5 captcha'), (9, 'T', 112, 'J2MEv + aj.c + aj.a'), (10, 'T', 127, 'same client string as tag9')])],
        200: [builder(f'a:({STR*2})V', 188, [
            (1, 'S', 199, '1'), (2, 'B', 212, '2'), (3, 'S', 226, 'parsed version without decimal point'),
            (5, 'T', 239, 'argument1 username'), (7, 'T', 253, 'J2MEv + aj.c + aj.a'), (8, 'T', 269, 'platform'),
            (11, 'I', 285, 'aj.D font setting'), (12, 'I', 304, 'aj.e width minus6'), (13, 'I', 324, 'aj.f height minus aj.g bottom bar'),
            (15, 'T', 345, 'settings tag6 language'), (16, 'I', 368, 'configuration tag1 revision'), (17, 'T', 388, 'cell ID'),
            (18, 'T', 410, 'u.a prior session', 'session != null'), (9, 'B', 431, 'bc.u.am desired presence', 'session != null'),
            (9, 'B', 463, '1', 'session == null and preference7 != 1'), (9, 'B', 481, '99', 'session == null and preference7 == 1'),
            (19, 'B', 518, 'preference17 == 1 ? 1 : 0'), (20, 'T', 538, 'LAC'), (21, 'T', 558, 'MCC'), (22, 'T', 578, 'MNC')
        ], 'Credentials are persisted before packet creation; no password field is emitted. Full table reviewed in protocol.md.')],
        202: [builder(DISPATCH, 2637, [(1, 'I', 2672, 'Java hashCode(response201 tag1 challenge + settings password.toLowerCase())')])],
        204: [text('activation code')], 205: [text('mobile')],
        206: [builder('a:(SIIZ)V', 5, [(1, 'B', 19, 'argument2 service'), (2, 'B', 41, 'argument3 mode', 'opcode == 206'), (3, 'B', 55, '0', 'opcode == 206')], 'Boolean argument4 ignored.')],
        300: [empty],
        302: [builder('a:(SIIZ)V', 5, [(1, 'B', 19, 'argument2 service')], 'For opcode302 only tag1 is inserted; arguments3/4 unused on this branch.')],
        400: [empty],
        405: [builder(f'a:({STR*8}IZ{STR*2}B)V', 7, [
            (2, 'I', 27, 'argument9 group', 'argument9 != -1'), (3, 'T', 45, 'argument1 display', 'argument1 != null'),
            (12, 'T', 64, 'argument3 mig33 username', 'argument3 != null'), (24, 'B', 98, 'argument10 share-mobile ? 1 : 0', 'argument3 != null'),
            (8, 'T', 117, 'argument2 mobile', 'argument2 != null'), (30, 'T', 138, 'argument12 external username', 'argument12 != null'),
            (31, 'B', 160, 'argument13 external service', 'argument13 != -1')], 'Arguments4/5/6/7/8/11 ignored.')],
        406: [number('contact ID')],
        407: [builder(f'a:({STR*3}Ljava/util/Vector;Ljava/util/Vector;II)V', 7, [
            (1, 'I', 21, 'argument6 contact ID'), (3, 'T', 39, 'argument1 display', 'argument1 != null'),
            (8, 'T', 58, 'argument2 mobile', 'argument2 != null'), (12, 'T', 77, 'argument3 mig33 username', 'argument3 != null'),
            (2, 'I', 98, 'argument7 group', 'argument7 != -1'),
            (30, 'TA', 225, 'argument4 external usernames', 'both vectors non-null, nonempty and equal length'),
            (31, 'X', 241, 'argument5 services parsed with Byte.parseByte', 'both vectors non-null, nonempty and equal length')])],
        408: [single('j', 2, 'group name')], 409: [number('group ID')],
        410: [builder(f'b:({STR}I)V', 8, [(1, 'I', 18, 'argument2 group ID'), (2, 'T', 31, 'argument1 name')])],
        411: [builder('b:(II)V', 7, [(1, 'I', 18, 'argument1 contact ID'), (2, 'I', 36, 'argument2 group', 'argument2 != -1')])],
        417: [builder(f'b:({STR}Z)V', 8, [(1, 'T', 18, 'argument1 username'), (2, 'B', 31, '2')], 'Boolean argument ignored.')],
        419: [builder(DISPATCH, 14327, [(1, 'L', 14338, 'Long.parseLong(aj.W), or new Date().getTime() when aj.W == null')], '203 tag20 matches cached contact version; timestamp branch offsets14206..14235.')],
        500: [builder(f'a:(S{STR*2}I[BSI)V', 7, [
            (1, 'B', 21, 'argument1 channel'), (2, 'T', 41, 'settings username'), (3, 'B', 57, 'argument4 conversation'),
            (4, 'T', 71, 'argument2 destination'), (6, 'S', 87, 'argument6 content'),
            (8, 'X', 108, 'argument5 bytes', 'argument5 != null'), (8, 'T', 126, 'argument3 text', 'argument5 == null'),
            (13, 'I', 148, 'argument7 RGB', 'argument7 != -1')], 'General conversation variant; no contact tag5.'),
            builder(f'a:(S{STR}I{STR}[BSI)V', 7, [
                (1, 'B', 21, 'argument1 channel'), (2, 'T', 62, 'channel2: aj.o own mobile; otherwise settings username'), (3, 'B', 76, '1'),
                (4, 'T', 90, 'argument2 destination'), (5, 'I', 109, 'argument3 contact ID', 'argument3 != -1'), (6, 'S', 125, 'argument6 content'),
                (8, 'X', 146, 'argument5 bytes', 'argument5 != null'), (8, 'T', 165, 'argument4 text', 'argument5 == null'),
                (13, 'I', 187, 'argument7 RGB', 'argument7 != -1')], 'Private conversation variant. Protocol.md table combines incoming/outgoing fields; tags9..12/14 are not emitted by either builder.')],
        504: [builder('a:(S[BSB)V', 7, [(1, 'S', 20, '1'), (2, 'X', 34, 'argument2 bytes'), (4, 'S', 53, 'argument3 parameter', 'argument3 != -1'), (5, 'B', 68, 'argument4 purpose')], 'First short argument ignored.')],
        600: [builder('a:(II)V', 8, [(1, 'B', 19, 'argument1 new presence'), (2, 'B', 33, 'argument2 old presence')])],
        601: [builder(f'c:({STR*2})V', 8, [(1, 'T', 18, 'argument1 new text'), (2, 'T', 31, 'argument2 old text')])],
        602: [builder(f'd:({STR*2})V', 8, [(1, 'T', 18, 'argument1 new avatar'), (1, 'T', 31, 'argument2 old avatar overwrites new')])],
        703: [builder(f'a:({STR}Z)V', 8, [(1, 'T', 18, 'argument1 room')], 'Boolean argument ignored.')],
        704: [text('room')], 706: [member], 707: [text('room')], 709: [member], 710: [member],
        711: [single('i', 1, 'room')],
        712: [builder(f'a:({STR}S)V', 8, [(1, 'S', 18, 'argument2 category'), (2, 'T', 31, 'argument1 room')])],
        713: [builder('b:()V', 8, [])],
        716: [builder('a:(SB)V', 8, [(1, 'S', 18, 'argument1 category'), (2, 'B', 31, 'argument2 flag')])],
        751: [builder(f'a:({STR*3}B)V', 40, [(1, 'T', 53, 'argument1 existing peer'), (2, 'T', 67, 'argument2 invitee'), (3, 'B', 82, 'argument4 service')], 'Selected when argument3 group ID == null.')],
        752: [builder(f'a:({STR*3}B)V', 11, [(1, 'T', 24, 'argument3 group ID'), (2, 'T', 67, 'argument2 invitee'), (3, 'B', 82, 'argument4 service')], 'Selected when argument3 group ID != null.')],
        753: [builder(f'a:({STR}B)V', 8, [(1, 'T', 18, 'argument1 group ID'), (2, 'B', 31, 'argument2 service')])],
        754: [builder(f'b:({STR}B)V', 8, [(1, 'T', 18, 'argument1 group ID'), (2, 'B', 31, 'argument2 service')])],
        800: [builder(f'a:({STR*2}II)V', 8, [(4, 'B', 18, '0'), (5, 'B', 31, '1'), (1, 'T', 44, 'argument1 origin'), (2, 'T', 57, 'argument2 destination')], 'Both integer arguments ignored.')],
        900: [builder(f'b:({STR*2})V', 8, [(1, 'T', 18, 'argument1 mobile'), (2, 'T', 31, 'argument2 name')])],
        904: [single('h', 2, 'mobile')],
        907: [builder('b:(B)V', None, [(1, 'B', 22, '1')], 'Mutates aj.aD opcode at12; retains all cached fields. Byte argument ignored.'),
              builder(COMMAND, None, [(30, 'B', 16565, 'aj.ao selected service')], 'Caller creates906 cache if absent at16536, inserts service then calls bc.b:(B)V at16598.', owner='aj')],
        913: [builder(f'a:({STR}I)V', 8, [(1, 'T', 36, 'argument1 token + literal U+0020'), (2, 'I', 49, 'argument2 action mode')])],
        921: [builder('a:(B)V', 8, [(1, 'B', 18, 'argument1 subscription flag')])],
        922: [builder(f'a:(I{STR*3}Lbl;I)V', 7, [
            (1, 'B', 38, 'argument3 != null ? 2 POST : 1 GET'), (2, 'T', 52, 'argument2 URL'), (3, 'T', 70, 'argument3 form body', 'argument3 != null'),
            (4, 'I', 84, 'argument1 purpose'), (5, 'T', 104, 'argument4 asset/cache key', 'argument4 != null'), (6, 'I', 119, '0'),
            (7, 'T', 136, 'cell ID'), (8, 'T', 153, 'LAC'), (9, 'T', 170, 'MCC'), (10, 'T', 187, 'MNC'),
            (11, 'TA', 223, 'one assembled Cookie header string', 'ac.a(URL) nonempty')], 'Final integer argument ignored; callback stored in ay.b at238 is not a wire field.'),
              builder('b:(Ljavax/microedition/lcdui/Graphics;)V', 5283, [(1, 'B', 5293, '1'), (2, 'T', 5333, 'resolved body background URL'), (4, 'I', 5347, '2')], 'Minimal HTML-parser object stored in s.F vector at5359; not evidence of full network-builder mandatory fields.', owner='s')],
        925: [single('a', 1, 'language code')],
        926: [builder(DISPATCH, 4884, [(1, 'X', 4908, 'one byte: external descriptor service ID')], 'Issued while processing208 if descriptor icon is not cached.')],
        1001: [single('b', 1, 'language-list URL')], 1003: [single('c', 1, 'language-pack URL')], 1005: [single('g', 1, 'raw HTTP URL')],
    }
    return rows


# All calls to the five generic opcode-taking request builders, across all26
# original classes. Literal arguments manually reviewed at these call offsets.
GENERIC = {
    ('bc', DISPATCH, 14383): 400,
    ('aj', 'keyPressed:(I)V', 1276): 707,
    **{('aj', COMMAND, offset): opcode for offset, opcode in [
        (1152, 710), (2523, 707), (2773, 704), (4086, 302), (4149, 206),
        (8398, 300), (8881, 302), (8914, 206), (11493, 300), (15556, 2),
        (15782, 205), (16107, 204), (16202, 2), (16279, 2), (16689, 709),
        (16727, 706), (16819, 409), (16886, 406), (16960, 205), (20926, 707)]},
    ('aj', 'c:(I)Z', 92): 206,
    ('aj', 'n:()V', 122): 206,
    ('aj', 'n:()V', 195): 206,
}
GENERIC_METHODS = {'a:(S)V', f'a:(S{STR})V', f'a:(S{STR*2})V', 'a:(SI)V', 'a:(SIIZ)V'}


def expected_probes():
    result = {}
    def add(label, opcode, fields):
        encoded = {}
        for tag, kind, value in fields:
            if kind == 'T': raw = value.encode('utf-8')
            elif kind == 'TA': raw = '\0'.join(value).encode('utf-8')
            elif kind == 'X': raw = bytes(value)
            else: raw = struct.pack('>' + {'B': 'b', 'S': 'h', 'I': 'i', 'L': 'q'}[kind], value)
            encoded[tag] = raw
        result[label] = opcode, encoded
    for label, opcode, tag, value in [
        ('security',17,1,'answer'), ('avatar',602,1,'old'), ('add_contact_group',408,2,'group'),
        ('forgot',904,2,'mobile'), ('add_favorite',711,1,'room'), ('language',925,1,'en-US'),
        ('language_list_job',1001,1,'http://example.test/list'), ('language_pack_job',1003,1,'http://example.test/pack'),
        ('raw_http_job',1005,1,'http://example.test/raw'), ('generic_text',704,1,'room'),
        ('generic_activate',204,1,'code'), ('generic_mobile',205,1,'mobile'), ('generic_participants',707,1,'room')]:
        add(label,opcode,[(tag,'T',value)])
    for label, opcode, value in [('help',7,5), ('generic_int',406,42), ('generic_delete_group',409,7), ('registration_start',101,320)]:
        add(label, opcode, [(1,'I',value)])
    add('status',601,[(1,'T','new'),(2,'T','old')])
    for label in ('block_true','block_false'): add(label,417,[(1,'T','peer'),(2,'B',2)])
    for label in ('join_true','join_false'): add(label,703,[(1,'T','room')])
    for label in ('callback','callback_changed'): add(label,800,[(1,'T','from'),(2,'T','to'),(4,'B',0),(5,'B',1)])
    for label in ('connect_true','connect_false'): add(label,206,[(1,'B',3),(2,'B',1),(3,'B',0)])
    add('disconnect',302,[(1,'B',3)])
    add('prompt',9,[(1,'B',1),(2,'S',99)])
    add('presence',600,[(1,'B',4),(2,'B',1)])
    add('emoticon',913,[(1,'T',':) '),(2,'I',1)])
    add('category',716,[(1,'S',7),(2,'B',1)])
    add('remove_favorite',712,[(1,'S',7),(2,'T','room')])
    add('rename_group',410,[(1,'I',42),(2,'T','group')])
    add('move_group',411,[(1,'I',42),(2,'I',7)])
    add('move_no_group',411,[(1,'I',42)])
    for label, value in [('activity_on',1),('activity_off',0)]: add(label,921,[(1,'B',value)])
    add('private_group_new',751,[(1,'T','peer'),(2,'T','invitee'),(3,'B',3)])
    add('private_group_invite',752,[(1,'T','group'),(2,'T','invitee'),(3,'B',3)])
    for label,opcode in [('private_group_leave',753),('private_group_members',754)]: add(label,opcode,[(1,'T','group'),(2,'B',3)])
    add('invite_friend',900,[(1,'T','mobile'),(2,'T','name')])
    for label,opcode in [('room_categories',713),('generic_empty',400),('generic_test',2),('generic_logout',300)]: add(label,opcode,[])
    for label,opcode in [('generic_member',706),('generic_mute',709),('generic_unmute',710)]: add(label,opcode,[(1,'T','room'),(2,'T','target')])
    add('preference_present',11,[(1,'B',2),(2,'T','key'),(3,'T','value')])
    add('preference_absent',11,[(1,'B',2),(2,'T','key')])
    add('upload_parameter',504,[(1,'S',1),(2,'X',[0,1,255]),(4,'S',7),(5,'B',0)])
    add('upload_no_parameter',504,[(1,'S',1),(2,'X',[0,1,255]),(5,'B',1)])
    add('edit_contact_full',407,[(1,'I',42),(2,'I',7),(3,'T','display'),(8,'T','mobile'),(12,'T','mig33'),(30,'TA',['alice','bob']),(31,'X',[3,4])])
    for label in ('edit_contact_min','edit_contact_mismatch','edit_contact_empty'): add(label,407,[(1,'I',42)])
    add('add_contact_full',405,[(2,'I',7),(3,'T','display'),(8,'T','mobile'),(12,'T','mig33'),(24,'B',1),(30,'T','external'),(31,'B',3)])
    add('add_contact_min',405,[])
    add('registration_submit',103,[(1,'T','user'),(2,'T','password'),(3,'T','mobile'),(4,'T','code'),(5,'T','email'),(9,'T','J2MEv4.62-test'),(10,'T','J2MEv4.62-test')])
    for label in ('cached_settings','cached_settings_changed_argument'): add(label,907,[(1,'B',1),(2,'T','retained'),(30,'B',3)])
    return result


def validate_packets(output):
    expected = expected_probes()
    actual = {}
    for line in output.splitlines():
        label, equals, raw = line.partition('=')
        assert equals and label not in actual, line
        actual[label] = bytes.fromhex(raw)
    assert actual.keys() == expected.keys(), (actual.keys() - expected.keys(), expected.keys() - actual.keys())
    evidence = []
    for label, raw in actual.items():
        version, opcode, transaction, length = struct.unpack('>BhhI', raw[:9])
        assert (version, transaction, length) == (2, 0, len(raw) - 9), label
        pos, fields = 9, {}
        while pos < len(raw):
            tag, size = struct.unpack('>hI', raw[pos:pos+6]); pos += 6
            assert tag not in fields and pos + size <= len(raw), label
            fields[tag] = raw[pos:pos+size]; pos += size
        assert (opcode, fields) == expected[label], (label, opcode, fields, expected[label])
        evidence.append(dict(case=label, opcode=opcode, status='passed', packet_sha256=hashlib.sha256(raw).hexdigest()))
    return evidence


def java_tool(name, home):
    candidates = [Path(home) / 'bin' / name] if home else []
    candidates += [Path('/opt/homebrew/opt/openjdk@21/bin') / name]
    if shutil.which(name): candidates.append(Path(shutil.which(name)))
    return str(next(p for p in candidates if p.is_file()))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--java-home')
    parser.add_argument('--output', type=Path, default=DATA / 'protocol-audit-outgoing.json')
    args = parser.parse_args()
    jar = BASE / 'artifacts/original/mig33-4.62.jar'
    assert digest(jar) == SHA, 'Incorrect original JAR'
    all_methods = [m for p in sorted((BASE / 'artifacts/bytecode').rglob('*.txt')) for m in parse(p)]
    methods = {(m['owner'], m['name'] + ':' + m['descriptor']): m for m in all_methods}
    rawdir = ROOT / 'data/protocol-audit'; rawdir.mkdir(parents=True, exist_ok=True)
    constructors = []
    generic_calls = []
    for method in all_methods:
        identity = (method['owner'], method['name'] + ':' + method['descriptor'])
        instructions = method['instructions']
        for index, ins in enumerate(instructions):
            if 'Method ay."<init>":(S)V' in ins['args']:
                value = integer(instructions[index-1])
                constructors.append(dict(opcode=value if value is not None else 'variable', class_name=method['owner'], method=identity[1], declaration=method['declaration'], offset=ins['offset'], line=ins['line']))
            call = re.search(r'// Method (?:(bc)\.)?(a:\([^ ]+\)V)$', ins['args'])
            if call and (call[1] or method['owner'] == 'bc') and call[2] in GENERIC_METHODS:
                key = (*identity, ins['offset'])
                assert key in GENERIC, ('Unreviewed generic builder call', key)
                opcode = GENERIC[key]
                assert any(integer(x) == opcode for x in instructions[max(0,index-15):index]), ('Literal not found before reviewed call', key)
                generic_calls.append(dict(class_name=method['owner'], method=identity[1], offset=ins['offset'], line=ins['line'], opcode=opcode, helper='bc.'+call[2]))
    assert {(*[c[k] for k in ('class_name','method')],c['offset']) for c in generic_calls} == set(GENERIC)
    catalog_constructors = json.loads((DATA / 'protocol-constructions.json').read_text())
    expected_inventory = Counter((str(opcode), c['class'], c['method'], c['line']) for opcode, entries in catalog_constructors.items() for c in entries)
    observed_inventory = Counter((str(c['opcode']), c['class_name'], c['declaration'], c['line']) for c in constructors)
    assert expected_inventory == observed_inventory, (expected_inventory-observed_inventory, observed_inventory-expected_inventory)
    (rawdir / 'outgoing-constructors.json').write_text(json.dumps(constructors,indent=2)+'\n')
    (rawdir / 'outgoing-generic-callers.json').write_text(json.dumps(generic_calls,indent=2)+'\n')
    review = reviews()
    catalog_path = DATA / 'protocol-outgoing.json'
    catalog = json.loads(catalog_path.read_text())['operations']
    assert set(review) == {r['opcode'] for r in catalog} and len(catalog) == 56
    writes = 0
    for opcode, variants in review.items():
        for variant in variants:
            method = methods[(variant['class_name'],variant['method'])]
            instructions = {i['offset']: i for i in method['instructions']}
            if variant['constructor_offset'] is not None:
                ins = instructions[variant['constructor_offset']]
                assert 'Method ay."<init>":(S)V' in ins['args'], (opcode, ins)
            for f in variant['fields']:
                ins = instructions[f['offset']]
                expected = 'Method bj."<init>":(S' + TYPES[f['type']] + ')V'
                assert expected in ins['args'], (opcode,f,ins,expected)
                writes += 1
    emulator = ROOT / 'tools/applets/bin/microemulator-2.0.4.jar'
    pinned = next(x for x in json.loads((emulator.parent/'manifest.json').read_text())['tools'] if x['file'] == emulator.name)
    assert digest(emulator) == pinned['sha256'], 'Incorrect MicroEmulator'
    classes = rawdir / 'builder-classes'; classes.mkdir(exist_ok=True)
    cp = os.pathsep.join([str(jar),str(emulator)])
    harness = ROOT / 'tools/applets/v4/protocol-builder-audit.java'
    compiled = subprocess.run([java_tool('javac',args.java_home), '-cp',cp,'-d',str(classes),str(harness)],capture_output=True,text=True,check=True)
    (rawdir / 'builder-compile.log').write_text(compiled.stdout+compiled.stderr)
    run = subprocess.run([java_tool('java',args.java_home),'-Djava.awt.headless=true','-cp',os.pathsep.join([str(classes),cp]),'ProtocolBuilderAudit'],capture_output=True,text=True,check=True)
    (rawdir / 'original-builder-output.txt').write_text(run.stdout)
    cases = validate_packets(run.stdout)
    (rawdir / 'builder-case-results.json').write_text(json.dumps(cases,indent=2)+'\n')
    wrong_913 = 'animated/photo content'
    rows = []
    for catalog_row in catalog:
        opcode = catalog_row['opcode']
        row = dict(opcode=opcode, status='verified', builders=review[opcode],
                   runtime_cases=[r['case'] for r in cases if r['opcode'] == opcode])
        callers = [c for c in generic_calls if c['opcode'] == opcode]
        if callers: row['generic_callers'] = callers
        if opcode in (1001,1003,1005): row['scope'] = 'local HTTP job; not a gateway opcode'
        if opcode == 913:
            row['status'] = 'correction_required' if wrong_913 in catalog_row['schema_and_flow'] else 'corrected'
            row['finding'] = 'Pre-audit JSON claimed response914 mode1 can open animated/photo content. Original request-mode1 enables type2 vibration and type4 WAV playback. Request fields themselves were correct.'
            row['response_evidence'] = [
                'bc.a:([Lay;)V@5819..5867 reads pending-request mode; type4 calls aj.a([BLjava/lang/String;)V; type2 calls aj.d(I)V',
                'aj.d:(I)V@0..10 invokes Display.vibrate(500)',
                'aj.a:([BLjava/lang/String;)V@17..33 invokes bd(byte[])',
                'bd.<init>:([B)V@9..12 sets audio/x-wav; bd.run:()V@132..142 creates and starts Player']
        if opcode == 419: row['precision_note'] = 'Before this audit, the saved-timestamp summary omitted the null-cache fallback to current time; the recovered branch is included above.'
        if opcode in (200,500,922): row['detail_location'] = 'protocol.md dedicated table, reviewed together with JSON summary'
        rows.append(row)
    result = dict(
        scope='All56 outgoing rows: request construction and original-client argument/branch behavior. Purpose names remain contextual interpretations. Response arrows are not proof of server behavior; incoming and transport audits cover their client-side consumers.',
        original_jar_sha256=SHA, catalog_sha256=digest(catalog_path),
        status_meaning=dict(verified='Reviewed request fields agree with original construction; not a complete server contract', correction_required='Confirmed contradictory catalog claim', corrected='Previously contradictory claim removed; evidence retained'),
        evidence=dict(disassembly='Original javap -c -p -s, method descriptors and instruction offsets',
                      constructor_sites=len(constructors), constructor_inventory='exact opcode/class/declaration/line multiset match',
                      generic_call_sites=len(generic_calls), generic_opcodes=sorted({c['opcode'] for c in generic_calls}),
                      generic_opcodes_outside_catalog=sorted({c['opcode'] for c in generic_calls}-{r['opcode'] for r in catalog}),
                      checked_field_constructor_references=writes,
                      distinct_field_constructor_offsets=len({(b['class_name'],b['method'],f['offset']) for variants in review.values() for b in variants for f in b['fields']}),
                      runtime_cases=len(cases), runtime_opcode_count=len({c['opcode'] for c in cases}),
                      runtime_opcodes=sorted({c['opcode'] for c in cases}), static_only_opcodes=sorted(set(review)-{c['opcode'] for c in cases}),
                      runtime_scope='Actual original bc builders and ay serializer; private queue captured. Local-job serialization is only an inspection technique, not transport evidence.',
                      harness_sha256=digest(harness)),
        limitations=['Field/tag/value/condition annotations are manual source review; the automatic static check verifies method and constructor-overload offsets, not arbitrary Java semantics.',
                     'Runtime probes cover representative present/absent and ignored-argument paths for49 operations;200/202/10/419/500/922/926 are static-only.',
                     'No historical server is available here. Catalog completeness is bounded by observed code in this exact original client, not all server operations.',
                     'TLV fields have no on-wire type discriminator. Builder/accessor usage supplies these type labels; absence/mandatory/server-validation rules require separate evidence.'],
        operations=rows)
    args.output.parent.mkdir(parents=True,exist_ok=True)
    args.output.write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps(dict(rows=len(rows), constructor_sites=len(constructors), generic_calls=len(generic_calls), checked_writes=writes, runtime_cases=len(cases), runtime_opcodes=len({c['opcode'] for c in cases}), output=str(args.output.relative_to(ROOT))),indent=2))


if __name__ == '__main__':
    main()
