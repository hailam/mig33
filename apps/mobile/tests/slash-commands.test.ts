import assert from 'node:assert/strict';
import test from 'node:test';
import { slashCommands } from '../src/mig33/generated/slash-commands.ts';
import { applySlashSuggestion, getSlashSuggestions, leadingSlashParent, slashQuery, SlashEditorState,
  type SlashCommandContext } from '../src/ui/slashCommands.ts';

const privateChat: SlashCommandContext = { kind: 'private' };
const room: SlashCommandContext = { kind: 'room', joined: true };
const communityRoom: SlashCommandContext = { ...room, communityID: 7 };
const caret = (position: number) => ({ start: position, end: position });
const names = (draft: string, context = privateChat, position = draft.length) =>
  getSlashSuggestions(draft, caret(position), context).map((command) => command.command);
function id(command: string): string {
  const entry = slashCommands.find((entry) => entry.command === command);
  assert.ok(entry, `Missing catalog command: ${command}`);
  return entry.id;
}

test('completion requires a leading slash and a valid collapsed caret within its command token', () => {
  const query = slashQuery(' \n/HU bob', caret(5));
  assert.ok(query);
  assert.deepEqual({ start: query.start, end: query.end, prefix: query.prefix }, { start: 2, end: 5, prefix: 'hu' });
  assert.ok(names('/H').includes('hug'));
  for (const draft of ['hello /hu', 'https://example.com', '//', '/hu!', '/me waves']) assert.equal(slashQuery(draft), null);
  for (const selection of [{ start: 1, end: 2 }, caret(-1), caret(9), caret(1.5), caret(NaN)]) {
    assert.equal(slashQuery('/hu', selection), null);
  }
  assert.equal(slashQuery('/hu', caret(0)), null);
  assert.equal(names('/unknown').length, 0);
});

test('suggestions respect actual conversation scope and mute state without inventing moderator authority', () => {
  assert.ok(names('/').includes('me'));
  assert.ok(names('/').includes('gift'));
  assert.ok(!names('/').includes('games'));
  assert.ok(names('/', room).includes('games'));
  assert.ok(!names('/', room).includes('silence'));
  assert.ok(names('/', communityRoom).includes('silence'));
  assert.ok(!names('/', { ...room, communityID: NaN }).includes('silence'));
  assert.ok(!names('/', { ...room, communityID: 1.5 }).includes('silence'));
  assert.deepEqual(names('/', { ...room, joined: false }), []);
  assert.deepEqual(names('/', { kind: 'group', joined: false }), []);
  const muted = getSlashSuggestions('/', caret(1), { ...communityRoom, muted: true });
  assert.ok(muted.length > 0);
  assert.ok(muted.every((command) => command.allowMuted));
  assert.ok(!muted.some((command) => command.command === 'me'));
  assert.ok(getSlashSuggestions('/sil', caret(4), communityRoom)[0]?.requirement);
  for (const unsupported of ['kick', 'mute', 'unmute']) assert.ok(!names('/', communityRoom).includes(unsupported));
});

test('known gift and bot parents offer subcommands, but never reinterpret argument text', () => {
  assert.ok(!names('/').some((name) => name.includes(' ')));
  assert.ok(names('/gift ').includes('gift search'));
  assert.deepEqual(names('/gift se'), ['gift search']);
  assert.equal(slashQuery('/gift search flowers'), null);
  assert.deepEqual(names('/gift bob'), []);
  assert.equal(slashQuery('/gift bob red rose'), null);
  assert.ok(names('/bot ', communityRoom).includes('bot dice'));
  assert.ok(names('/bot ', communityRoom).includes('bot 8ball'));
  assert.deepEqual(names('/bot st', communityRoom), ['bot stop']);
  assert.deepEqual(names('/bot ', privateChat), []);
  assert.deepEqual(names('/gift ratin'), ['gift rate'], 'documented aliases resolve to the canonical suggestion');
});

test('selection replaces the whole command and retains leading whitespace and existing arguments exactly', () => {
  assert.deepEqual(applySlashSuggestion('/hug', caret(2), privateChat, id('hug')), {
    draft: '/hug ', selection: caret(5),
  });
  assert.deepEqual(applySlashSuggestion(' \n/HU\t bob 😀  ', caret(5), privateChat, id('hug')), {
    draft: ' \n/hug\t bob 😀  ', selection: caret(8),
  });
  assert.deepEqual(applySlashSuggestion('/gift se  red rose -m hi 😀', caret(8), privateChat, id('gift search')), {
    draft: '/gift search  red rose -m hi 😀', selection: caret(14),
  });
  assert.deepEqual(applySlashSuggestion('/ro', caret(3), privateChat, id('roll')), {
    draft: '/roll', selection: caret(5),
  });
  assert.deepEqual(applySlashSuggestion('/bot st', caret(7), communityRoom, id('bot stop')), {
    draft: '/bot stop', selection: caret(9),
  });
});

test('the generic gift-send template appears only before any recipient text exists', () => {
  assert.ok(getSlashSuggestions('/gift ', caret(6), privateChat).some((command) => command.id === 'gift-send'));
  assert.deepEqual(applySlashSuggestion('/gift ', caret(6), privateChat, 'gift-send'), {
    draft: '/gift ', selection: caret(6),
  });
  for (const position of [6, 7, 9]) {
    assert.ok(!getSlashSuggestions('/gift bob', caret(position), privateChat).some((command) => command.id === 'gift-send'));
    assert.equal(applySlashSuggestion('/gift bob', caret(position), privateChat, 'gift-send'), undefined);
  }
});

test('application revalidates the exact query, context and command instead of using stale menu state', () => {
  const query = slashQuery('/hu', caret(3))!;
  assert.ok(applySlashSuggestion('/hu', caret(3), privateChat, id('hug'), query.key));
  assert.equal(applySlashSuggestion('/hu bob', caret(3), privateChat, id('hug'), query.key), undefined);
  assert.equal(applySlashSuggestion('/hu', caret(2), privateChat, id('hug'), query.key), undefined);
  assert.equal(applySlashSuggestion('/hu', caret(3), privateChat, id('roll')), undefined);
  assert.equal(applySlashSuggestion('/sil', caret(4), room, id('silence')), undefined);
  assert.equal(applySlashSuggestion('/me', caret(3), { ...room, muted: true }, id('me')), undefined);
  assert.equal(applySlashSuggestion('/gift se', caret(8), privateChat, id('gift')), undefined);
});

test('replacement obeys the320 UTF-16 unit bound without dropping arguments or slicing emoji', () => {
  const fits = '/h ' + '😀'.repeat(157) + 'x';
  const tooLong = fits + 'y';
  assert.equal(fits.length, 318);
  const result = applySlashSuggestion(fits, caret(2), privateChat, id('hug'));
  assert.ok(result);
  assert.equal(result.draft.length, 320);
  assert.equal(result.draft, '/hug ' + '😀'.repeat(157) + 'x');
  assert.equal(getSlashSuggestions(fits, caret(2), privateChat).find((entry) => entry.id === id('hug'))?.disabledReason, undefined);
  assert.match(getSlashSuggestions(tooLong, caret(2), privateChat).find((entry) => entry.id === id('hug'))?.disabledReason ?? '', /Shorten.*320/);
  assert.equal(applySlashSuggestion(tooLong, caret(2), privateChat, id('hug')), undefined);
  assert.equal(tooLong.length, 319, 'a refused insertion leaves the caller’s draft intact');
});

test('free-form gift mode survives argument entry and caret changes until its actual prefix changes', () => {
  for (const draft of ['/gift ', '/gift bob ', '/gift bob red rose -m hi', ' \n/GIFT bob', '/gift']) {
    assert.equal(leadingSlashParent(draft), 'gift');
  }
  assert.equal(slashQuery('/gift bob '), null, 'argument editing has no completion query but retains its parent');
  assert.equal(leadingSlashParent('/gif'), null);
  assert.equal(leadingSlashParent('hello /gift bob'), null);
  assert.equal(leadingSlashParent('/gifted bob'), null);
  assert.equal(leadingSlashParent('/bot dice'), 'bot');
  assert.equal(leadingSlashParent(''), null);
});

test('batched native text and caret events stay associated with the edited string before props commit', () => {
  const editor = new SlashEditorState('/h bob');
  editor.changeSelection(caret(2));
  editor.changeText('/hu bob');
  editor.syncDraft('/h bob'); // A rerender can still carry the previous prop.
  editor.changeSelection(caret(3));
  assert.deepEqual(editor.snapshot, { draft: '/hu bob', selection: caret(3) });
  editor.syncDraft('/hu bob');
  assert.equal(slashQuery(editor.snapshot.draft, editor.snapshot.selection)?.prefix, 'hu');
  assert.ok(getSlashSuggestions(editor.snapshot.draft, editor.snapshot.selection, privateChat).some((entry) => entry.command === 'hug'));

  editor.changeText('/hug bob', caret(5)); // A suggestion's forced selection.
  editor.syncDraft('/hug bob');
  editor.changeSelection(caret(5));
  assert.deepEqual(editor.snapshot, { draft: '/hug bob', selection: caret(5) });
  editor.syncDraft('/me waves'); // External draft changes do not inherit a stale caret.
  assert.deepEqual(editor.snapshot, { draft: '/me waves' });
  editor.changeSelection(caret(2));
  assert.equal(slashQuery(editor.snapshot.draft, editor.snapshot.selection)?.prefix, 'm');
});
