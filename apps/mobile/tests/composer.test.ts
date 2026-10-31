import assert from 'node:assert/strict';
import test from 'node:test';
import { insertComposerText } from '../src/ui/composer.ts';

test('inserting a username replaces only the selected range and places the caret after it', () => {
  assert.deepEqual(insertComposerText('hello someone today', 'bob', { start: 6, end: 13 }), {
    draft: 'hello bob today', selection: { start: 9, end: 9 },
  });
  assert.deepEqual(insertComposerText('hello', '[smile]'), {
    draft: 'hello [smile] ', selection: { start: 14, end: 14 },
  });
});

test('composer tools preserve full drafts and native UTF-16 limits without clipping surrounding text', () => {
  const full = '😀'.repeat(160);
  assert.equal(insertComposerText(full, 'bob'), undefined);
  assert.equal(full.length, 320);
  const replacement = insertComposerText('x'.repeat(320), 'bob', { start: 0, end: 5 });
  assert.equal(replacement?.draft, 'bob ' + 'x'.repeat(315));
  assert.equal(insertComposerText('hello world', '[smile]', { start: 0, end: 0 })?.draft, '[smile] hello world');
});
