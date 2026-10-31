import assert from 'node:assert/strict';
import test from 'node:test';
import { readableMessageColor } from '../src/ui/screens/messageColor.ts';

test('Modern preserves readable packet colors and substitutes dark-surface black/blue', () => {
  assert.equal(readableMessageColor(0x000000, '#162b3b', '#ffffff'), '#000000');
  assert.equal(readableMessageColor(0x000000, '#edf5f8', '#15242e'), '#edf5f8');
  assert.equal(readableMessageColor(0x406ac0, '#8bd5d8', '#15242e'), '#8bd5d8');
  assert.equal(readableMessageColor(0xffcc33, '#edf5f8', '#19323e'), '#ffcc33');
  assert.equal(readableMessageColor(0xffffff, '#162b3b', '#ffffff'), '#162b3b');
});

test('Modern enforces the normal-text4.5 threshold and handles original default markers', () => {
  assert.equal(readableMessageColor(0x767676, '#162b3b', '#ffffff'), '#767676');
  assert.equal(readableMessageColor(0x777777, '#162b3b', '#ffffff'), '#162b3b');
  for (const value of [undefined, -1, Number.NaN]) {
    assert.equal(readableMessageColor(value, '#edf5f8', '#0b151c'), '#edf5f8');
  }
});
