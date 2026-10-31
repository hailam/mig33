import assert from 'node:assert/strict';
import test from 'node:test';
import { decodeEmoticon, imageDataURI } from '../src/mig33/media.ts';
import { Packet } from '../src/mig33/protocol/index.ts';

const png = Uint8Array.of(137, 80, 78, 71, 13, 10, 26, 10);
function integer(value: number): Uint8Array {
  const result = new Uint8Array(4); new DataView(result.buffer).setInt32(0, value, false); return result;
}
function concat(...parts: Uint8Array[]): Uint8Array { return Uint8Array.from(parts.flatMap((part) => [...part])); }
function part(value: Uint8Array): Uint8Array { return concat(integer(value.length), value); }

test('image URIs are format-gated and base64 preserves every byte including incomplete triples', () => {
  assert.equal(imageDataURI(png), 'data:image/png;base64,iVBORw0KGgo=');
  for (const length of [8, 9, 10, 11, 100]) {
    const value = concat(png, Uint8Array.from({ length: length - 8 }, (_, index) => index * 73 & 255));
    assert.equal(imageDataURI(value), 'data:image/png;base64,' + Buffer.from(value).toString('base64'));
  }
  assert.equal(imageDataURI(new TextEncoder().encode('<svg onload="x"/>')), undefined);
});

test('914 parses static, framed animation and framed image/audio resources without confusing type with action mode', () => {
  const base = () => new Packet(914, 7).setText(1, '[token]').setText(2, '[token] [alias]').setText(3, 'Label');
  const staticImage = decodeEmoticon(base().setByte(5, 2).setBytes(4, png));
  assert.equal(staticImage.type, 2); assert.deepEqual(staticImage.aliases, ['[token]', '[alias]']);
  assert.equal(staticImage.imageURI, imageDataURI(png));
  const animated = decodeEmoticon(base().setByte(5, 3).setBytes(4, concat(integer(2), part(png), part(png))));
  assert.deepEqual(animated.frames, [png, png]);
  assert.deepEqual(animated.frameURIs, [imageDataURI(png), imageDataURI(png)]);
  const audio = new TextEncoder().encode('RIFFtestWAVE');
  const sound = decodeEmoticon(base().setByte(5, 4).setBytes(4, concat(part(png), part(audio))));
  assert.deepEqual(sound.audio, audio); assert.equal(sound.imageURI, imageDataURI(png));
  assert.equal(decodeEmoticon(base().setByte(5, 4)).type, 0, 'missing field4 forces original type0');
});

test('framed resources reject negative, truncated, oversized and trailing payloads before rendering', () => {
  for (const bytes of [integer(-1), concat(integer(2000000), integer(0)), concat(integer(1), integer(20), png),
    concat(integer(1), part(png), Uint8Array.of(1))]) {
    assert.throws(() => decodeEmoticon(new Packet(914, 1).setByte(5, 3).setBytes(4, bytes)), /resource|frame|data/);
  }
  assert.throws(() => decodeEmoticon(new Packet(914, 1).setByte(5, 4).setBytes(4, png)), /resource/);
});
