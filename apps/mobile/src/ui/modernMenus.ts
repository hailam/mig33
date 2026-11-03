import type { V4MenuItem } from './v4/primitives';

/** Modern lists expose pull to refresh and an accessible heading action.
 * Apply to built-in entries before adding the server's menu items;
 * page refresh and category continuation commands have different behavior. */
export function modernListItems(items: readonly V4MenuItem[], list: 'friends' | 'rooms'): V4MenuItem[] {
  const duplicateLabel = list === 'friends' ? 'Refresh Contact List' : 'Refresh Rooms';
  return items.filter((item) => item.id !== 'refresh' || item.label !== duplicateLabel);
}

/** Organize the original actions without changing their handlers, rights or
 * server-supplied additions. Unknown additions remain available under More. */
export function modernChatItems(items: readonly V4MenuItem[], kind: 'private' | 'room' | 'group',
  openGroup: (title: string, items: V4MenuItem[]) => void): V4MenuItem[] {
  const remaining = [...items];
  function take(ids: readonly string[]) {
    const selected: V4MenuItem[] = [];
    for (const id of ids) {
      for (let index = 0; index < remaining.length;) {
        if (remaining[index].id === id) selected.push(...remaining.splice(index, 1));
        else index++;
      }
    }
    return selected;
  }
  const primary = take(kind === 'room' ? ['participants', 'info', 'favorite']
    : kind === 'group' ? ['participants', 'invite', 'block', 'blocks'] : ['profile', 'invite', 'block', 'blocks']);
  const ending = take(['appearance', 'leave', 'close']);
  const people = take(['private', 'profile', 'add', 'gift', 'call', 'sms', 'credit']);
  const moderation = kind === 'room' ? take(['kick', 'mute', 'unmute', 'block', 'report', 'blocks']) : [];
  function group(id: string, title: string, children: V4MenuItem[]): V4MenuItem[] {
    return children.length ? [{ id: `modern-${id}`, label: title, testID: `chat.menu.${id}`,
      children, onPress: () => openGroup(title, children) }] : [];
  }
  return [...primary, ...group('people', 'People', people), ...group('moderation', 'Moderation', moderation),
    ...group('more', 'More', remaining), ...ending];
}
