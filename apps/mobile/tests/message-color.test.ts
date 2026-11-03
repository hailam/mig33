import assert from 'node:assert/strict';
import test from 'node:test';
import { readableMessageColor } from '../src/ui/screens/messageColor.ts';

function contrast(a: string, b: string): number {
  const luminance = (hex: string) => {
    const linear = [1, 3, 5].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16) / 255)
      .map((channel) => channel <= .04045 ? channel / 12.92 : ((channel + .055) / 1.055) ** 2.4);
    return linear[0] * .2126 + linear[1] * .7152 + linear[2] * .0722;
  };
  const x = luminance(a), y = luminance(b);
  return (Math.max(x, y) + .05) / (Math.min(x, y) + .05);
}

test('Modern preserves readable packet colors exactly and adapts unreadable colors without erasing hue', () => {
  assert.equal(readableMessageColor(0x000000, '#162b3b', '#ffffff'), '#000000');
  assert.equal(readableMessageColor(0xffcc33, '#edf5f8', '#19323e'), '#ffcc33');
  const blue = readableMessageColor(0x0000ff, '#edf5f8', '#19323e');
  assert.notEqual(blue, '#edf5f8');
  assert.equal(blue.slice(5), 'ff');
  assert.equal(blue.slice(1, 3), blue.slice(3, 5));
  assert.ok(contrast(blue, '#19323e') >= 4.5);
  const red = readableMessageColor(0xff0000, '#162b3b', '#ffffff');
  assert.notEqual(red, '#162b3b');
  assert.equal(red.slice(3), '0000');
  assert.ok(contrast(red, '#ffffff') >= 4.5);
});

test('original924 chromatic choices stay distinct and every choice is readable on both Modern bubble surfaces', () => {
  const palette = [0, 16711680, 32768, 255, 8388736, 16744192, 3368601, 10040064, 8421504];
  for (const [background, fallback] of [['#ffffff', '#162b3b'], ['#eef5f7', '#162b3b'],
    ['#15242e', '#edf5f8'], ['#19323e', '#edf5f8']]) {
    const rendered = palette.map((value) => readableMessageColor(value, fallback, background));
    // Black and gray can reach the same minimum neutral contrast boundary.
    // Every colored choice must retain its own hue instead of becoming Default.
    assert.equal(new Set(rendered.slice(1, -1)).size, palette.length - 2, `Distinct hues collapsed on ${background}`);
    assert.ok(new Set(rendered).size >= palette.length - 1, `Palette collapsed on ${background}`);
    for (let index = 0; index < palette.length; index++) {
      const original = '#' + palette[index].toString(16).padStart(6, '0');
      assert.ok(contrast(rendered[index], background) >= 4.5, `${rendered[index]} on ${background}`);
      assert.notEqual(rendered[index], fallback, 'A real palette choice silently became Default');
      if (contrast(original, background) >= 4.5) assert.equal(rendered[index], original);
    }
  }
});

test('quantized threshold, neutral extremes and original default/invalid markers remain safe', () => {
  assert.equal(readableMessageColor(0x767676, '#162b3b', '#ffffff'), '#767676');
  assert.equal(readableMessageColor(0x777777, '#162b3b', '#ffffff'), '#767676');
  for (const value of [0, 0xffffff, 0x777777]) for (const background of ['#000000', '#ffffff', '#777777']) {
    assert.ok(contrast(readableMessageColor(value, '#123456', background), background) >= 4.5);
  }
  for (const value of [undefined, -1, Number.NaN, Infinity, 0x1000000, 2.5]) {
    assert.equal(readableMessageColor(value, '#edf5f8', '#0b151c'), '#edf5f8');
  }
  assert.equal(readableMessageColor(255, '#edf5f8', 'invalid'), '#edf5f8');
});
