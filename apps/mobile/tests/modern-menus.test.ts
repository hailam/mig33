import assert from 'node:assert/strict';
import test from 'node:test';
import { modernChatItems, modernListItems } from '../src/ui/modernMenus.ts';

test('Modern lists remove only their redundant built-in refresh and retain original handlers', () => {
  const items = [
    { id: 'refresh', label: 'Refresh Contact List', onPress: () => {} },
    { id: 'refresh', label: 'Refresh Rooms', onPress: () => {} },
    { id: 'refresh', label: 'Refresh', onPress: () => {} },
    { id: 'more', label: 'Refresh Rooms', onPress: () => {} },
    { id: 'search', label: 'Search Rooms', disabled: true, onPress: () => {} },
    { id: 'server:refresh', label: 'Refresh Contact List', onPress: () => {} },
  ];
  const snapshot = [...items];
  assert.deepEqual(modernListItems(items, 'friends'), items.slice(1));
  assert.deepEqual(modernListItems(items, 'rooms'), [items[0], ...items.slice(2)]);
  assert.deepEqual(items, snapshot);
  assert.equal(modernListItems(items, 'friends').at(-1), items.at(-1));
});

test('Modern menu organization retains every original action, permission and server addition', () => {
  const items = ['participants', 'info', 'favorite', 'profile', 'private', 'add', 'gift', 'call', 'sms', 'credit',
    'kick', 'mute', 'block', 'report', 'blocks', 'chats', 'main', 'appearance', 'leave', 'server-added'].map((id) => ({
      id, label: id, disabled: id === 'mute', onPress: () => id,
    }));
  const snapshot = [...items];
  for (const kind of ['room', 'private', 'group'] as const) {
    const result = modernChatItems(items, kind, () => {});
    const leaves = result.flatMap((entry) => entry.children ?? [entry]);
    assert.equal(leaves.length, items.length);
    for (const original of items) assert.equal(leaves.filter((entry) => entry === original).length, 1);
  }
  assert.deepEqual(items, snapshot);
});

test('Modern room moderation opens the original group and keeps leave directly reachable', () => {
  let opened: unknown;
  const kick = { id: 'kick', label: 'Kick User', onPress: () => {} };
  const leave = { id: 'leave', label: 'Leave Room', destructive: true, onPress: () => {} };
  const result = modernChatItems([kick, leave], 'room', (title, items) => { opened = { title, items }; });
  result.find((entry) => entry.id === 'modern-moderation')!.onPress!();
  assert.deepEqual(opened, { title: 'Moderation', items: [kick] });
  assert.equal(result.at(-1), leave);
  assert.equal(result.some((entry) => entry.children?.length === 0), false);
});
