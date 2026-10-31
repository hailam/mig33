import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

import {
  Packet, ProtocolError, FrameDecoder, encodeFrame, decodeFrame, decodeFrames,
  HEADER_SIZE, MAX_FRAME_SIZE, encodeUTF8, decodeUTF8, javaStringHash,
  loginChallengeHash, lowercasePortablePassword,
} from '../src/mig33/protocol/index.ts';

interface FixtureField {
  tag: number;
  type: 'B' | 'S' | 'I' | 'L' | 'T' | 'X' | 'TA';
  value: number | string | string[];
}

interface Fixture {
  name: string;
  opcode: number;
  transaction: number;
  fields: FixtureField[];
  original_java_hex: string;
}

const fixtures = JSON.parse(readFileSync(
  new URL('../../../internal/protocol/testdata/original_v4_packets.json', import.meta.url), 'utf8',
)).packets as Fixture[];
const probes = JSON.parse(readFileSync(
  new URL('../../../research/applets/v4/docs/data/protocol-audit-transports.json', import.meta.url), 'utf8',
)).original_runtime_probes as Array<{
  name: string; bytes?: string; byte?: number | null | { throws: string };
  short?: number | null; int?: number | null; long_decimal?: string | null;
  text?: string | null; text_array?: string[] | null;
}>;

function hex(bytes: Uint8Array): string {
  return Buffer.from(bytes).toString('hex');
}

function unhex(value: string): Uint8Array {
  return Uint8Array.from(Buffer.from(value, 'hex'));
}

function concatenate(parts: Uint8Array[]): Uint8Array {
  return Uint8Array.from(Buffer.concat(parts));
}

// Node's independent primitive encoders establish expected field bytes instead
// of deriving expected values through the production codec.
function fieldBytes(field: FixtureField): Uint8Array {
  if (field.type === 'T') return Uint8Array.from(Buffer.from(field.value as string, 'utf8'));
  if (field.type === 'X') return unhex(field.value as string);
  if (field.type === 'TA') return Uint8Array.from(Buffer.from((field.value as string[]).join('\0')));
  const result = Buffer.alloc({ B: 1, S: 2, I: 4, L: 8 }[field.type]);
  switch (field.type) {
    case 'B': result.writeInt8(field.value as number); break;
    case 'S': result.writeInt16BE(field.value as number); break;
    case 'I': result.writeInt32BE(field.value as number); break;
    case 'L': result.writeBigInt64BE(BigInt(field.value as number)); break;
  }
  return Uint8Array.from(result);
}

function fixturePacket(fixture: Fixture): Packet {
  const packet = new Packet(fixture.opcode, fixture.transaction);
  for (const field of fixture.fields) packet.setBytes(field.tag, fieldBytes(field));
  return packet;
}

function buildFixture(fixture: Fixture): Packet {
  const packet = new Packet(fixture.opcode, fixture.transaction);
  for (const field of fixture.fields) {
    switch (field.type) {
      case 'B': packet.setByte(field.tag, field.value as number); break;
      case 'S': packet.setShort(field.tag, field.value as number); break;
      case 'I': packet.setInt(field.tag, field.value as number); break;
      case 'L': packet.setLong(field.tag, BigInt(field.value as number)); break;
      case 'T': packet.setText(field.tag, field.value as string); break;
      case 'TA': packet.setStringArray(field.tag, field.value as string[]); break;
      case 'X': packet.setBytes(field.tag, unhex(field.value as string)); break;
    }
  }
  return packet;
}

function canonical(packet: Packet): object {
  return {
    opcode: packet.opcode, transaction: packet.transaction,
    fields: Object.fromEntries([...packet.fields.entries()].sort(([a], [b]) => a - b).map(
      ([tag, value]) => [tag, hex(value)],
    )),
  };
}

function expectProtocolError(code: string, action: () => unknown): void {
  assert.throws(action, (error: unknown) => error instanceof ProtocolError && error.code === code);
}

test('all original serializer fixtures decode to independently encoded values', async (t) => {
  assert.equal(fixtures.length, 9);
  for (const fixture of fixtures) {
    await t.test(fixture.name, () => {
      const raw = unhex(fixture.original_java_hex);
      const packet = decodeFrame(raw);
      assert.deepEqual(canonical(packet), canonical(fixturePacket(fixture)));
      // Decoded insertion order is the exact original Hashtable enumeration.
      assert.equal(hex(encodeFrame(packet, { fieldOrder: 'insertion' })), fixture.original_java_hex);
      // The deterministic writer emits the same fields in signed-tag order.
      const encoded = encodeFrame(buildFixture(fixture));
      const tags = [...decodeFrame(encoded).fields.keys()];
      assert.deepEqual(tags, [...tags].sort((a, b) => a - b));
      assert.deepEqual(canonical(decodeFrame(encoded)), canonical(packet));
    });
  }
});

test('all audited raw accessor probes retain their original interpretations', async (t) => {
  for (const probe of probes.filter((p) => /^(raw_|array_|wire_)/.test(p.name))) {
    await t.test(probe.name, () => {
      const packet = new Packet(203, 1).setBytes(31, unhex(probe.bytes!));
      assert.equal(packet.byte(31), typeof probe.byte === 'object' ? undefined : probe.byte ?? undefined);
      assert.equal(packet.short(31), probe.short ?? undefined);
      assert.equal(packet.int(31), probe.int ?? undefined);
      assert.equal(packet.long(31), probe.long_decimal == null ? undefined : BigInt(probe.long_decimal));
      assert.equal(packet.text(31), probe.text);
      assert.deepEqual(packet.stringArray(31), probe.text_array);
      assert.equal(hex(packet.bytes(31)!), probe.bytes);
    });
  }
});

test('signed ranges, numeric prefix views and 64-bit precision remain intact', () => {
  const packet = new Packet(-32768, 32767)
    .setByte(-32768, -128)
    .setShort(-1, -32768)
    .setInt(0, -2147483648)
    .setLong(32767, -(1n << 63n))
    .setLong(99, (1n << 63n) - 1n)
    .setBytes(31, unhex('000100020304050607'));
  const decoded = decodeFrame(encodeFrame(packet));
  assert.equal(decoded.opcode, -32768);
  assert.equal(decoded.transaction, 32767);
  assert.equal(decoded.byte(-32768), -128);
  assert.equal(decoded.short(-1), -32768);
  assert.equal(decoded.int(0), -2147483648);
  assert.equal(decoded.long(32767), -9223372036854775808n);
  assert.equal(decoded.long(99), 9223372036854775807n);
  assert.equal(decoded.byte(31), 0);
  assert.equal(decoded.short(31), 1);
  assert.equal(decoded.int(31), 65538);
  assert.equal(decoded.long(31), 0x0001000203040506n);
  assert.deepEqual([...decoded.fields.keys()], [-32768, -1, 0, 31, 99, 32767]);
});

test('absent, empty, undersized and unknown raw fields remain distinct', () => {
  const packet = new Packet(32767, -1).setBytes(-32768, new Uint8Array()).setBytes(31, unhex('ff'));
  assert.equal(packet.bytes(9000), undefined);
  assert.equal(packet.text(9000), undefined);
  assert.equal(packet.stringArray(9000), undefined);
  assert.equal(packet.fields.has(-32768), true);
  assert.deepEqual(packet.bytes(-32768), new Uint8Array());
  assert.equal(packet.text(-32768), '');
  assert.deepEqual(packet.stringArray(-32768), []);
  assert.equal(packet.byte(-32768), undefined); // Safe local API, unlike original throw.
  assert.equal(packet.short(31), undefined);
  assert.equal(packet.int(31), undefined);
  assert.equal(packet.long(31), undefined);
  assert.deepEqual(canonical(decodeFrame(encodeFrame(packet))), canonical(packet));
});

test('duplicate tags keep the last payload on construction and reception', () => {
  const registration = new Packet(101, 5).setInt(1, 240).setInt(1, 320);
  assert.equal(hex(encodeFrame(registration)), fixtures.find((f) => f.name === 'registration_duplicate_tag')!.original_java_hex);
  const avatar = new Packet(602, 9).setText(1, 'new-avatar').setText(1, 'old-avatar');
  assert.equal(hex(encodeFrame(avatar)), fixtures.find((f) => f.name === 'avatar_duplicate_tag')!.original_java_hex);
  const decoded = decodeFrame(unhex('02000200010000000e0001000000016100010000000162'));
  assert.equal(decoded.fields.size, 1);
  assert.equal(decoded.text(1), 'b');
});

test('byte ownership protects against network buffer reuse and caller mutation', () => {
  const supplied = Uint8Array.of(1, 2, 3);
  const packet = new Packet(2, 1).setBytes(20, supplied);
  supplied.fill(255);
  assert.equal(hex(packet.bytes(20)!), '010203');
  const returned = packet.bytes(20)!;
  returned.fill(0);
  assert.equal(hex(packet.bytes(20)!), '010203');
  const encoded = encodeFrame(packet);
  const decoded = decodeFrame(encoded);
  encoded.fill(0);
  assert.equal(hex(decoded.bytes(20)!), '010203');
});

test('incremental decoder accepts every split of concatenated original frames', () => {
  const combined = concatenate(fixtures.map((fixture) => unhex(fixture.original_java_hex)));
  const expected = fixtures.map((fixture) => canonical(fixturePacket(fixture)));
  for (let split = 0; split <= combined.length; split++) {
    const decoder = new FrameDecoder();
    const packets = [...decoder.push(combined.subarray(0, split)), ...decoder.push(combined.subarray(split))];
    decoder.finish();
    assert.deepEqual(packets.map(canonical), expected, `split at byte ${split}`);
    assert.equal(decoder.bufferedBytes, 0);
  }
  const decoder = new FrameDecoder();
  const packets: Packet[] = [];
  for (const byte of combined) packets.push(...decoder.push(Uint8Array.of(byte)));
  decoder.finish();
  assert.deepEqual(packets.map(canonical), expected);
  assert.deepEqual(decodeFrames(new Uint8Array()), []);
});

test('incremental decoder copies partial callback buffers', () => {
  const bytes = encodeFrame(new Packet(500, 1).setText(8, 'Hello 😀'));
  const decoder = new FrameDecoder();
  const head = bytes.slice(0, 11);
  assert.deepEqual(decoder.push(head), []);
  head.fill(255);
  const [packet] = decoder.push(bytes.subarray(11));
  decoder.finish();
  assert.equal(packet.text(8), 'Hello 😀');
});

test('truncated and malformed streams fail without unsafe allocation or resynchronization', () => {
  const cases: Array<[string, string]> = [
    ['030002000100000000', 'invalid-marker'],
    ['0200020001ffffffff', 'invalid-length'],
    ['02000200017fffffff', 'too-large'],
    ['0200020001000000060001ffffffff', 'invalid-length'],
    ['02000200010000000100', 'invalid-length'],
    ['02000200010000000700010000000261', 'invalid-length'],
    ['020002000100000007000100000001', 'truncated'],
    ['02000200010000000000', 'invalid-length'],
  ];
  for (const [raw, code] of cases) expectProtocolError(code, () => decodeFrame(unhex(raw)));
  const complete = encodeFrame(new Packet(2, 1).setText(1, 'value'));
  for (let length = 1; length < complete.length; length++) {
    const decoder = new FrameDecoder();
    assert.deepEqual(decoder.push(complete.subarray(0, length)), []);
    expectProtocolError('truncated', () => decoder.finish());
    expectProtocolError('decoder-failed', () => decoder.push(complete));
    decoder.reset();
    assert.equal(decoder.push(complete).length, 1);
  }
  const decoder = new FrameDecoder(64);
  expectProtocolError('too-large', () => decoder.push(unhex('02000200017fffffff')));
  assert.equal(decoder.bufferedBytes, 0);
  expectProtocolError('decoder-failed', () => decoder.push(encodeFrame(new Packet(2, 1))));
  decoder.reset();
  assert.equal(decoder.push(encodeFrame(new Packet(2, 1))).length, 1);
});

test('frame limits include the header and bound retained partial frames', () => {
  const frame = encodeFrame(new Packet(2, 1).setBytes(1, new Uint8Array(MAX_FRAME_SIZE - HEADER_SIZE - 6)));
  assert.equal(frame.length, MAX_FRAME_SIZE);
  const decoder = new FrameDecoder();
  assert.deepEqual(decoder.push(frame.subarray(0, frame.length - 1)), []);
  assert.equal(decoder.bufferedBytes, MAX_FRAME_SIZE - 1);
  assert.equal(decoder.push(frame.subarray(-1)).length, 1);
  assert.equal(decoder.bufferedBytes, 0);
  expectProtocolError('too-large', () => encodeFrame(
    new Packet(2, 1).setBytes(1, new Uint8Array(MAX_FRAME_SIZE - HEADER_SIZE - 5)),
  ));
  expectProtocolError('too-large', () => encodeFrame(new Packet(2, 1).setByte(1, 1), { maxFrameSize: 15 }));
});

test('numeric inputs cannot silently wrap or lose precision', () => {
  for (const value of [-32769, 32768, 1.5, NaN, Infinity]) {
    assert.throws(() => new Packet(value, 0), RangeError);
    assert.throws(() => new Packet(0, value), RangeError);
    assert.throws(() => new Packet(0, 0).setBytes(value, new Uint8Array()), RangeError);
  }
  assert.throws(() => new Packet(0, 0).setByte(1, 128), RangeError);
  assert.throws(() => new Packet(0, 0).setInt(1, 2147483648), RangeError);
  assert.throws(() => new Packet(0, 0).setLong(1, 1n << 63n), RangeError);
  assert.throws(() => new Packet(0, 0).setLong(1, -(1n << 63n) - 1n), RangeError);
  assert.throws(() => new FrameDecoder(8), RangeError);
  assert.throws(() => new FrameDecoder(MAX_FRAME_SIZE + 1), RangeError);
  // Direct raw-map mutation is permitted, but the final encoder still validates.
  const packet = new Packet(0, 0);
  packet.fields.set(32768, new Uint8Array());
  assert.throws(() => encodeFrame(packet), RangeError);
});

test('UTF-8 matches an independent implementation for Unicode boundaries and malformed input', () => {
  const values = ['', '\0', 'hello', '✓ 😀', 'ދިވެހި', 'Åé', '\ufefftext',
    String.fromCodePoint(0x7f, 0x80, 0x7ff, 0x800, 0xd7ff, 0xe000, 0xffff, 0x10000, 0x10ffff)];
  let state = 12345;
  for (let index = 0; index < 1000; index++) {
    state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
    const point = state % 0x110000;
    if (point < 0xd800 || point > 0xdfff) values.push(String.fromCodePoint(point));
  }
  for (const text of values) {
    assert.deepEqual(encodeUTF8(text), new TextEncoder().encode(text));
    assert.equal(decodeUTF8(encodeUTF8(text)), text);
  }
  for (const invalid of ['ff', 'c0af', 'e0a0', 'e08080', 'eda080', 'f0808080', 'f4908080', 'e228a1', 'f09f92']) {
    const bytes = unhex(invalid);
    assert.equal(decodeUTF8(bytes), new TextDecoder('utf-8', { ignoreBOM: true }).decode(bytes), invalid);
  }
  assert.throws(() => encodeUTF8('\ud800'), RangeError);
  assert.throws(() => encodeUTF8('\udc00'), RangeError);
});

test('Java hashes use UTF-16 units and explicit portable password case conversion', () => {
  assert.equal(javaStringHash(''), 0);
  assert.equal(javaStringHash('hello'), 99162322);
  assert.equal(javaStringHash('😀'), 1772899);
  assert.equal(javaStringHash('\ud800'), 55296);
  assert.equal(javaStringHash('polygenelubricants'), -2147483648);
  assert.equal(lowercasePortablePassword('MiXeD ÀÖØÞ × ß 😀'), 'mixed àöøþ × ß 😀');
  assert.equal(loginChallengeHash('challenge', 'MiXeD 😀'), javaStringHash('challengemixed 😀'));
  assert.throws(() => loginChallengeHash('challenge', 'İtest'), /unverified/);
  assert.throws(() => loginChallengeHash('challenge', 'Σtest'), /unverified/);
  assert.equal(loginChallengeHash('x', 'İtest', () => 'itest'), javaStringHash('xitest'));
});
