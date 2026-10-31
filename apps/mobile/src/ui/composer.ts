export interface EditorSelection { start: number; end: number; }

/** Touch adaptation of the original concatenating editor: preserve a selected
 * replacement range and surrounding text, never truncate an existing draft.
 */
export function insertComposerText(draft: string, text: string, selection?: EditorSelection): {
  draft: string; selection: EditorSelection;
} | undefined {
  const start = Math.max(0, Math.min(draft.length, selection?.start ?? draft.length));
  const end = Math.max(start, Math.min(draft.length, selection?.end ?? start));
  const before = draft.slice(0, start), after = draft.slice(end);
  const inserted = (before && !/\s$/.test(before) ? ' ' : '') + text + (/^\s/.test(after) ? '' : ' ');
  if (before.length + inserted.length + after.length > 320) return undefined;
  const caret = before.length + inserted.length;
  return { draft: before + inserted + after, selection: { start: caret, end: caret } };
}
