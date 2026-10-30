#!/usr/bin/env python3
"""Audit the v4 incoming catalog against original javap output.

Independent checks use method-scoped CFG traversal and an abstract JVM operand
stack/local/field analysis. Branches are conservatively followed both ways;
this is evidence of possible reads, not a verifier or a server schema inference.
The existing extractor/catalog are never modified. Raw bytecode stays ignored.
"""
from __future__ import annotations

import argparse
from collections import deque
from dataclasses import dataclass
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[3]
BASE = ROOT / 'research/applets/v4'
JAR_SHA = 'dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c'
TYPES = {'b': 'B', 'c': 'S', 'd': 'I', 'e': 'L', 'f': 'T', 'g': 'X', 'h': 'TA'}
RETURNS = {'b': 'Ljava/lang/Byte;', 'c': 'Ljava/lang/Short;', 'd': 'Ljava/lang/Integer;',
           'e': 'Ljava/lang/Long;', 'f': 'Ljava/lang/String;', 'g': '[B', 'h': '[Ljava/lang/String;'}


@dataclass(frozen=True)
class Ins:
    offset: int
    op: str
    args: str
    line: int


@dataclass
class Method:
    owner: str
    declaration: str
    descriptor: str
    instructions: list[Ins]
    switches: dict[int, dict[str, int]]
    handlers: list[dict]

    @property
    def label(self):
        name = '<clinit>' if self.declaration.endswith('static {};') else self.declaration.split('(')[0].split()[-1]
        return f'{self.owner}.{name}:{self.descriptor}'


def methods(path):
    output, current, switch, in_exceptions = [], None, None, False
    for n, line in enumerate(path.read_text().splitlines(), 1):
        if re.match(r'^  .*\(.*\);$', line) or re.match(r'^  (?:public )?static \{\};$', line):
            current = Method(path.stem, line.strip(), '', [], {}, [])
            output.append(current)
            switch, in_exceptions = None, False
        elif current and line.strip().startswith('descriptor:'):
            current.descriptor = line.strip().split(': ', 1)[1]
        elif current:
            m = re.match(r'\s*(\d+): ([a-z][a-z_0-9]*)\s*(.*)', line)
            if m:
                x = Ins(int(m[1]), m[2], m[3], n)
                current.instructions.append(x)
                switch = x.offset if x.op in ('lookupswitch', 'tableswitch') else None
                if switch is not None:
                    current.switches[switch] = {}
            elif switch is not None:
                m = re.match(r'\s*(-?\d+|default): (\d+)$', line)
                if m:
                    current.switches[switch][m[1]] = int(m[2])
                elif '}' in line:
                    switch = None
            elif 'Exception table:' in line:
                in_exceptions = True
            elif in_exceptions:
                m = re.match(r'\s*(\d+)\s+(\d+)\s+(\d+)\s+(.*)', line)
                if m:
                    current.handlers.append(dict(start=int(m[1]), end=int(m[2]), target=int(m[3]), type=m[4]))
    for m in output:
        offsets = [x.offset for x in m.instructions]
        assert m.descriptor, (path, m.declaration, 'missing descriptor; use javap -s')
        assert len(offsets) == len(set(offsets)), (path, m.label, 'method boundaries overlapped')
    return output


def successors(m, off):
    index = {x.offset: i for i, x in enumerate(m.instructions)}
    x = m.instructions[index[off]]
    follow = [m.instructions[index[off] + 1].offset] if index[off] + 1 < len(m.instructions) else []
    if x.op in ('goto', 'goto_w'):
        return [int(x.args.split()[0])]
    if x.op.startswith('if'):
        return [int(x.args.split()[0])] + follow
    if x.op in ('tableswitch', 'lookupswitch'):
        return list(m.switches[off].values())
    if x.op.endswith('return') or x.op in ('return', 'athrow'):
        return []
    return follow


def reachable(m, start, stop, exceptions=False):
    known = {x.offset for x in m.instructions}
    seen, queue = set(), [start]
    while queue:
        off = queue.pop()
        if off == stop or off in seen:
            continue
        assert off in known, (m.label, off)
        seen.add(off)
        queue.extend(successors(m, off))
        if exceptions:
            queue.extend(h['target'] for h in m.handlers if h['start'] <= off < h['end'])
    return seen


# One abstract item per JVM value; width distinguishes long/double from category1.
@dataclass(frozen=True)
class Value:
    origins: frozenset[str] = frozenset({'unknown'})
    constant: int | None = None
    width: int = 1


def value(origin='unknown', constant=None, width=1):
    return Value(frozenset({origin}), constant, width)


def merge_value(a, b):
    origins = a.origins | b.origins
    if len(origins) > 12:
        origins = frozenset({'unknown'})
    return Value(origins, a.constant if a.constant == b.constant else None, a.width)


def descriptor_types(desc):
    result = []
    i = 0
    while i < len(desc):
        start = i
        while desc[i] == '[':
            i += 1
        if desc[i] == 'L':
            i = desc.index(';', i) + 1
        else:
            i += 1
        result.append(desc[start:i])
    return result


def width(desc):
    return 2 if desc in ('J', 'D') else 1


def step(m, x, state):
    stack, loc, heap = list(state[0]), dict(state[1]), dict(state[2])
    observation = None
    op, arg = x.op, x.args
    def pop():
        assert stack, (m.label, x.offset, op, 'empty stack')
        return stack.pop()
    def push(v): stack.append(v)
    def local_index(): return int(op.rsplit('_', 1)[1]) if '_' in op else int(arg.split()[0])
    def field():
        name = arg.split('// Field ', 1)[1]
        if '.' not in name.split(':', 1)[0]: name = m.owner + '.' + name
        return name
    if op.startswith(('aload', 'iload', 'lload', 'fload', 'dload')):
        v = loc.get(local_index(), value(width=2 if op[0] in 'ld' else 1))
        push(v)
    elif op.startswith(('astore', 'istore', 'lstore', 'fstore', 'dstore')):
        loc[local_index()] = pop()
    elif op.startswith('iconst_'):
        push(value('constant', -1 if op == 'iconst_m1' else int(op[-1])))
    elif op.startswith('lconst_'):
        push(value('constant', int(op[-1]), 2))
    elif op in ('bipush', 'sipush'):
        push(value('constant', int(arg)))
    elif op in ('ldc', 'ldc_w', 'ldc2_w'):
        mat = re.search(r'// (?:int|long) (-?\d+)', arg)
        push(value('constant', int(mat[1]) if mat else None, 2 if op == 'ldc2_w' else 1))
    elif op == 'aconst_null': push(value('null'))
    elif op == 'new': push(value('new:' + arg.split('// class ')[1] + '@' + str(x.offset)))
    elif op in ('getstatic', 'getfield'):
        f = field()
        recv = pop() if op == 'getfield' else value('static')
        key = (tuple(sorted(recv.origins)), f)
        push(heap.get(key, value('field:' + f, width=width(f.split(':')[1]))))
    elif op in ('putstatic', 'putfield'):
        v = pop()
        recv = pop() if op == 'putfield' else value('static')
        heap[(tuple(sorted(recv.origins)), field())] = v
    elif op == 'aaload':
        pop()
        arr = pop()
        push(value('response') if arr.origins == frozenset({'response-array'}) else value('array-element'))
    elif op in ('baload', 'iaload', 'saload', 'caload', 'laload'):
        pop(); pop(); push(value(width=2 if op == 'laload' else 1))
    elif op.endswith('astore') and len(op) == 7:
        pop(); pop(); pop()
    elif op in ('bastore', 'aastore', 'iastore', 'sastore', 'castore', 'lastore'):
        pop(); pop(); pop()
    elif op == 'arraylength': pop(); push(value())
    elif op == 'newarray': pop(); push(value('new-array@' + str(x.offset)))
    elif op == 'checkcast': pass
    elif op == 'dup': push(stack[-1])
    elif op == 'dup_x1':
        a, b = pop(), pop(); stack.extend([a, b, a])
    elif op == 'swap':
        a, b = pop(), pop(); stack.extend([a, b])
    elif op == 'pop': pop()
    elif op in ('iadd', 'isub', 'irem', 'lmul'):
        b, a = pop(), pop()
        push(value(width=2 if op == 'lmul' else 1))
    elif op in ('i2s', 'i2b', 'i2c', 'i2l'):
        a = pop(); push(Value(a.origins, a.constant, 2 if op == 'i2l' else 1))
    elif op == 'iinc':
        index = int(arg.split(',')[0]); loc[index] = value('loop-variable')
    elif op in ('monitorenter', 'monitorexit', 'athrow'):
        pop()
    elif op.startswith('if'):
        pop()
        if op.startswith(('if_icmp', 'if_acmp')): pop()
    elif op in ('tableswitch', 'lookupswitch'): pop()
    elif op in ('goto', 'goto_w', 'return'): pass
    elif op in ('ireturn', 'areturn', 'lreturn'): pop()
    elif op.startswith('invoke'):
        call = re.search(r'// (?:InterfaceMethod|Method) (.+?):(\([^)]*\))(.+)$', arg)
        assert call, x
        name, args_desc, ret = call.groups()
        args = [pop() for _ in descriptor_types(args_desc[1:-1])][::-1]
        recv = value('static') if op == 'invokestatic' else pop()
        if name.startswith('ay.') and args_desc == '(S)' and name[3:] in (*TYPES, 'a'):
            assert ret == RETURNS.get(name[3:], 'Lbj;'), (name, ret)
            observation = {'bytecode_offset': x.offset, 'tag': args[0].constant,
                           'type': TYPES.get(name[3:], 'raw-field'), 'receiver_origins': sorted(recv.origins)}
        if ret != 'V':
            origin = 'pending-request' if name in ('b', 'bc.b') and args_desc == '(J)' and ret == 'Lay;' else 'return:' + name + '@' + str(x.offset)
            push(value(origin, width=width(ret)))
    else:
        raise AssertionError(('unsupported instruction', m.label, x))
    return (tuple(stack), loc, heap), observation


def join(old, new):
    assert len(old[0]) == len(new[0]), ('stack height disagreement', old[0], new[0])
    stack = tuple(merge_value(a, b) for a, b in zip(old[0], new[0]))
    loc = {k: merge_value(old[1].get(k, value()), new[1].get(k, value())) for k in old[1].keys() | new[1].keys()}
    heap = {k: merge_value(old[2].get(k, value('field:' + k[1])), new[2].get(k, value('field:' + k[1]))) for k in old[2].keys() | new[2].keys()}
    return stack, loc, heap


def provenance(m, start, stop):
    instructions = {x.offset: x for x in m.instructions}
    states = {start: ((), {0: value('this'), 1: value('response-array'), 2: value('response-index')}, {})}
    work = deque([start])
    passes = 0
    while work:
        off = work.popleft()
        out, _ = step(m, instructions[off], states[off])
        passes += 1
        assert passes < 100000, ('analysis did not converge', start)
        for nxt in successors(m, off):
            if nxt == stop: continue
            merged = join(states[nxt], out) if nxt in states else out
            if nxt not in states or states[nxt] != merged:
                states[nxt] = merged
                work.append(nxt)
    observations = []
    for off in sorted(states):
        _, obs = step(m, instructions[off], states[off])
        if obs: observations.append(obs)
    return observations, passes


def immediate_tag(m, i):
    p = m.instructions[i - 1]
    if p.op.startswith('iconst_'):
        return -1 if p.op == 'iconst_m1' else int(p.op[-1])
    if p.op in ('bipush', 'sipush'): return int(p.args)
    return None


def current_catalog_shape(m, visited):
    reads = []
    for i, x in enumerate(m.instructions):
        if x.offset not in visited: continue
        mat = re.search(r'Method ay\.([bcdefgh]):\(S\)', x.args)
        if not mat: continue
        tag = immediate_tag(m, i)
        if tag is None: continue
        direct = [p.op for p in m.instructions[max(0, i - 4):i - 1]] == ['aload_1', 'iload_2', 'aaload']
        reads.append(dict(tag=tag, type=TYPES[mat[1]], bytecode_offset=x.offset, direct_response_read=direct))
    fields = sorted({(r['tag'], r['type']) for r in reads if r['direct_response_read']})
    return reads, [dict(tag=t, type=k) for t, k in fields]


def source_ref(m, offset):
    return {'class': m.owner, 'method': m.label, 'bytecode_offset': offset,
            'file': f'research/applets/v4/artifacts/bytecode/{m.owner}.txt'}


def reproduce_extractor(source):
    """Run the unchanged historical extractor under a temporary fake repo root."""
    audit = ROOT / 'data/protocol-audit'
    audit.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='incoming-reproduction-', dir=audit) as temporary:
        root = Path(temporary)
        script = root / 'tools/applets/v4/protocol-extract.py'
        script.parent.mkdir(parents=True)
        shutil.copyfile(ROOT / 'tools/applets/v4/protocol-extract.py', script)
        base = root / 'research/applets/v4'
        (base / 'artifacts/bytecode').mkdir(parents=True)
        (base / 'docs/data').mkdir(parents=True)
        shutil.copyfile(source / 'bc.txt', base / 'artifacts/bytecode/bc.txt')
        subprocess.run([sys.executable, str(script)], check=True, capture_output=True)
        generated = (base / 'docs/data/protocol-incoming.json').read_bytes()
        assert generated == (BASE / 'docs/data/protocol-incoming.json').read_bytes()
        return hashlib.sha256(generated).hexdigest()


def grouped_reads(m, reads):
    """Group exact getter offsets; guard/value labels use the next instruction.

    A guard is an immediate null comparison. Other sites are called value reads;
    that label does not assert a final semantic use or server-side requirement.
    """
    next_ins = {x.offset: y for x, y in zip(m.instructions, m.instructions[1:])}
    grouped = {}
    for read in reads:
        key = (read['tag'], read['type'])
        row = grouped.setdefault(key, {'tag': key[0], 'view': key[1], 'presence_guard_offsets': [], 'value_read_offsets': []})
        purpose = 'presence_guard_offsets' if next_ins[read['bytecode_offset']].op in ('ifnull', 'ifnonnull') else 'value_read_offsets'
        row[purpose].append(read['bytecode_offset'])
    return [grouped[key] for key in sorted(grouped, key=lambda k: (k[0] is None, k[0] or 0, k[1]))]


def reviewed_annex(all_methods, dispatcher):
    """Hand-reviewed source paths, with mechanical assertions on their anchors.

    These annotations do not claim a general interprocedural taint analysis.
    They separate explicit delegation, deferred UI access, and cached packets.
    """
    def method(owner, declaration):
        return next(m for m in all_methods[owner] if m.declaration == declaration)
    def anchor(m, offset, contains):
        ins = next(x for x in m.instructions if x.offset == offset)
        assert contains in ins.args, (m.label, offset, contains, ins.args)
        return source_ref(m, offset)
    def getters(m):
        out = []
        for i, x in enumerate(m.instructions):
            match = re.search(r'Method ay\.([bcdefgh]):\(S\)', x.args)
            if match:
                out.append({'tag': immediate_tag(m, i), 'view': TYPES[match[1]], 'offset': x.offset})
        return out
    enqueue = method('aj', 'public final void a(ay);')
    queued = method('aj', 'private ay ai();')
    paint = method('aj', 'public final void paint(javax.microedition.lcdui.Graphics);')
    action = method('aj', 'public final void commandAction(javax.microedition.lcdui.Command, javax.microedition.lcdui.Displayable);')
    icons = method('aj', 'public static boolean c(ay);')
    cache_icons = method('aj', 'public static synchronized void d(ay);')
    language = method('aj', 'public static boolean b(ay);')
    language_lookup = method('y', 'public static java.lang.String e(int);')
    config_save = method('j', 'public static void a(ay);')
    config_int = method('y', 'public static int d(int);')
    config_presence = method('y', 'public static boolean c(int);')
    config_image = method('y', 'public static javax.microedition.lcdui.Image d(int);')
    login_builder = method('bc', 'public final void a(java.lang.String, java.lang.String);')
    return [
        {'opcode': 208, 'classification': 'cached-other-packet-not-response',
         'finding': 'The aj.c(ay) call receives the cached icon packet returned by aj.g(int), not the current 208 response. Its image fields must not be merged into the 208 response schema.',
         'evidence': [anchor(dispatcher, 4867, 'Method aj.g:(I)Lay;'), anchor(dispatcher, 4925, 'Method aj.c:(Lay;)Z')]},
        {'opcode': 502, 'classification': 'deferred-response-reads',
         'finding': 'Current packet is queued in aj.bW by aj.a(ay), returned by aj.ai(), and later read by the UI: tag 4 text for display and tag 5 text for the accepted URL.',
         'evidence': [anchor(dispatcher, 1177, 'Method aj.a:(Lay;)V'), anchor(enqueue, 12, 'Vector.addElement:'),
                      anchor(queued, 22, 'Field bW:'), anchor(paint, 4353, 'Method ay.f:(S)'), anchor(action, 20483, 'Method ay.f:(S)')],
         'response_fields': [{'tag': 4, 'view': 'T', 'method': paint.label, 'offset': 4353}, {'tag': 5, 'view': 'T', 'method': action.label, 'offset': 20483}]},
        {'opcode': 906, 'classification': 'response-alias-not-direct-pattern',
         'finding': 'Response is stored as aj.aD, then tag 30 is read through that alias. All three reads are already in all_getter_reads, but direct_response_read is false because the extractor recognizes only an immediate array-load sequence. The null branch repeats the same getter and unboxes it; no optional-field guarantee follows.',
         'evidence': [anchor(dispatcher, 3895, 'Field aj.aD:Lay;'), anchor(dispatcher, 3907, 'Method ay.b:(S)'), anchor(dispatcher, 3922, 'Method ay.b:(S)'), anchor(dispatcher, 3956, 'Method ay.b:(S)')]},
        {'opcode': 927, 'classification': 'delegated-response-reads',
         'finding': 'Current response is passed to aj.c(ay), which reads tag 1 as byte and tags 2 through 6 as image byte arrays. Success passes the same response to aj.d(ay), which reads tag 1 before caching serialized bytes.',
         'evidence': [anchor(dispatcher, 3282, 'Method aj.c:(Lay;)Z'), anchor(dispatcher, 3291, 'Method aj.d:(Lay;)V')],
         'delegates': [{'method': icons.label, 'getter_sites': getters(icons)}, {'method': cache_icons.label, 'getter_sites': getters(cache_icons)}]},
        {'opcode': 1000, 'classification': 'whole-packet-persistence-and-deferred-config',
         'finding': 'j.a(ay) serializes the entire incoming resource; its own tag 1 reads target settings j.a, not this packet. Dispatcher installs current response as aj.j. Subsequent consumers use dynamic integer/presence/image accessors; login reads config tag 1 as the revision. An empty dispatcher direct_fields array does not mean an empty config schema.',
         'evidence': [anchor(dispatcher, 13288, 'Method j.a:(Lay;)V'), anchor(config_save, 13, 'Method ay.b:()[B'),
                      anchor(dispatcher, 13294, 'Field aj.j:Lay;'), anchor(config_int, 16, 'Method ay.d:(S)'),
                      anchor(config_presence, 5, 'Method ay.a:(S)'), anchor(config_image, 179, 'Method ay.g:(S)'),
                      anchor(login_builder, 362, 'Method ay.d:(S)')],
         'limit': 'This audits packet forwarding and generic accessors. It does not infer every UI theme key or meaning from all call sites.'},
        {'opcode': 1002, 'classification': 'dynamic-tag-reads-omitted',
         'finding': 'Three direct current-response text getters are absent even from all_getter_reads because the tag is computed. The loop starts i=0, iterates i<field_count, and reads only when i%3==1.',
         'dynamic_getters': [{'offset': 12983, 'tag_expression': '(short)(i+1)', 'view': 'T'},
                            {'offset': 12993, 'tag_expression': '(short)(i+2)', 'view': 'T'},
                            {'offset': 13001, 'tag_expression': '(short)i', 'view': 'T'}],
         'evidence': [anchor(dispatcher, 12956, 'Method ay.a:()I'), anchor(dispatcher, 12983, 'Method ay.f:(S)'),
                      anchor(dispatcher, 12993, 'Method ay.f:(S)'), anchor(dispatcher, 13001, 'Method ay.f:(S)')],
         'limit': 'Field count is not maximum tag; dense triples are a client assumption, not a recovered server validation rule.'},
        {'opcode': 1004, 'classification': 'delegated-and-deferred-language',
         'finding': 'aj.b(ay) persists the whole response, installs it as aj.s, and reads tag 1 text into settings. Later y.e(int) reads a dynamic text tag from aj.s. Dispatcher itself also reads tag 1 before sending language confirmation.',
         'evidence': [anchor(dispatcher, 13053, 'Method aj.b:(Lay;)Z'), anchor(language, 67, 'Field s:Lay;'),
                      anchor(language, 81, 'Method ay.f:(S)'), anchor(language_lookup, 11, 'Method ay.f:(S)')],
         'limit': 'The runtime lookup accepts dynamic resource IDs; dispatcher direct_fields is not a complete language-resource schema.'}
    ]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, default=BASE / 'docs/data/protocol-audit-incoming.json')
    args = parser.parse_args()
    jar = BASE / 'artifacts/original/mig33-4.62.jar'
    assert hashlib.sha256(jar.read_bytes()).hexdigest() == JAR_SHA
    source = BASE / 'artifacts/bytecode'
    all_methods = {cl: methods(source / f'{cl}.txt') for cl in ('bc', 'ay', 'aj', 'j', 'y')}
    m = next(x for x in all_methods['bc'] if x.declaration == 'public final synchronized void a(ay[]);')
    assert m.descriptor in ('', '([Lay;)V')
    m.descriptor = '([Lay;)V'
    dispatch = min(m.switches)
    starts = {int(k): v for k, v in m.switches[dispatch].items() if k != 'default'}
    stop = m.switches[dispatch]['default']
    byoff = {x.offset: x for x in m.instructions}
    assert byoff[stop].op == 'iinc' and byoff[stop].args == '2, 1'
    catalog = json.loads((BASE / 'docs/data/protocol-incoming.json').read_text())
    assert len(catalog) == len(starts) == 55
    assert set(starts) == {r['opcode'] for r in catalog}
    results = []
    catalog_sites, actual_sites, response_sites = set(), set(), set()
    for row in catalog:
        op = row['opcode']
        seen = reachable(m, starts[op], stop)
        with_exceptions = reachable(m, starts[op], stop, exceptions=True)
        reads, fields = current_catalog_shape(m, seen)
        all_reads, passes = provenance(m, starts[op], stop)
        provenance_reads = [r for r in all_reads if r['type'] != 'raw-field']
        presence_reads = [r for r in all_reads if r['type'] == 'raw-field']
        current_offsets = {r['bytecode_offset'] for r in reads}
        response_reads = [r for r in provenance_reads if 'response' in r['receiver_origins']]
        old_direct_offsets = {r['bytecode_offset'] for r in reads if r['direct_response_read']}
        assert old_direct_offsets <= {r['bytecode_offset'] for r in response_reads}, op
        assert all(r['receiver_origins'] == ['response'] for r in response_reads), op
        indirect_response_reads = [r for r in response_reads if r['bytecode_offset'] not in old_direct_offsets]
        exception_getters = [x.offset for x in m.instructions if x.offset in with_exceptions - seen and re.search(r'Method ay\.[bcdefgh]:\(S\)', x.args)]
        assert set(seen) == {x for x in row['normal_control_flow_offsets']}, op
        assert reads == row['all_getter_reads'], op
        assert fields == row['direct_fields'], op
        assert starts[op] == row['bytecode_entry'], op
        assert current_offsets <= {r['bytecode_offset'] for r in provenance_reads}, op
        catalog_sites.update(current_offsets)
        actual_sites.update(r['bytecode_offset'] for r in provenance_reads)
        response_sites.update(r['bytecode_offset'] for r in response_reads)
        results.append({'opcode': op, 'status': 'matches_declared_extractor_scope', 'entry': source_ref(m, starts[op]),
                        'normal_cfg_instruction_count': len(seen),
                        'catalog_direct_field_view_count': len(fields),
                        'catalog_getter_read_count': len(reads),
                        'all_typed_getter_site_count': len(provenance_reads),
                        'current_response_getter_site_count': len(response_reads),
                        'current_response_field_views': grouped_reads(m, response_reads),
                        'additional_response_reads_outside_direct_fields_scope': indirect_response_reads,
                        'raw_field_getters_outside_typed_getter_scope': presence_reads,
                        'non_response_getter_site_count': len(provenance_reads) - len(response_reads),
                        'extra_exception_reachable_instruction_count': len(with_exceptions - seen),
                        'extra_exception_getter_offsets': exception_getters,
                        'abstract_interpretation_steps': passes})
    output = {'scope': 'Every top-level incoming dispatcher case; method-scoped normal CFG, conservative exception CFG, and intraprocedural receiver provenance. Delegated/deferred consumers require the separately reviewed annex; no complete server schema is inferred.',
              'source': {'jar': str(jar.relative_to(ROOT)), 'jar_sha256': JAR_SHA,
                         'dispatcher': m.label, 'dispatch_offset': dispatch, 'iteration_boundary': stop,
                         'bytecode_sha256': {cl: hashlib.sha256((source / f'{cl}.txt').read_bytes()).hexdigest() for cl in all_methods}},
              'checks': {'top_level_cases': len(starts), 'catalog_rows': len(catalog), 'catalog_rows_matching_source': len(results),
                         'catalog_schema_and_scope_preserved': True,
                         'unchanged_extractor_byte_identical_reproduction_sha256': reproduce_extractor(source),
                         'counts_across_case_paths': {
                             'unit': 'Sum of getter instruction observations separately reachable from each opcode entry; shared handlers such as 708/755 count for each opcode.',
                             'catalog_constant_tag_typed_getters': sum(r['catalog_getter_read_count'] for r in results),
                             'all_typed_getters': sum(r['all_typed_getter_site_count'] for r in results),
                             'current_response_typed_getters': sum(r['current_response_getter_site_count'] for r in results)},
                         'unique_dispatcher_instruction_sites': {'catalog_constant_tag_typed_getters': len(catalog_sites),
                                                                'all_typed_getters': len(actual_sites), 'current_response_typed_getters': len(response_sites)},
                         'exception_model': 'Conservative edges from every instruction in each protected interval; extra reachability is an upper bound, not proof that every handler executes.'},
              'operations': results,
              'reviewed_delegated_alias_and_dynamic_annex': reviewed_annex(all_methods, m),
              'interpretation_notes': [
                  'All 55 rows reproduce and match the extractor scope. This is not a finding that their fields are a complete wire schema.',
                  '906 alias getters already occur in all_getter_reads. Only direct_fields excludes them. 1002 dynamic getters are absent from all_getter_reads itself.',
                  '203 tags 31/32 have multiple value interpretations. Raw binary fields carry no discriminator, so simultaneous typed views do not prove impossible serialization.',
                  '203 tags 33/35/37 use a byte presence guard then an integer value read. 421/423 tag 3 uses text presence then a long value read. These differ from multiple semantic value uses.',
                  'Field-view presence/value labels classify immediate null checks versus other uses; they do not establish requiredness, complete semantic use, or server validation.',
                  'The conservative exception-edge audit adds no dispatcher typed getter sites for any opcode. Error effects remain distinct from field discovery.',
                  'The normal CFG follows both sides of branches; it is not a proof that every path is feasible for one concrete packet.'
              ]}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(output, indent=2) + '\n')
    print(json.dumps(output['checks']))
    print('Additional current-response reads:', [(r['opcode'], [(x['bytecode_offset'], x['tag'], x['type']) for x in r['additional_response_reads_outside_direct_fields_scope']]) for r in results if r['additional_response_reads_outside_direct_fields_scope']])


if __name__ == '__main__':
    main()
