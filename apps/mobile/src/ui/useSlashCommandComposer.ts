import { useEffect, useRef, useState } from 'react';
import type { EditorSelection } from './composer';
import { applySlashSuggestion, getSlashSuggestions, leadingSlashParent, slashQuery, SlashEditorState, type SlashCommandContext } from './slashCommands';
import type { V4FieldRef } from './v4/primitives';

/** Editing assistance stays local: only the existing Send action uses the
 * transport. Each mounted editor owns its focus, caret and dismissal state. */
export function useSlashCommandComposer({ draft, context, selection, disabled, onChange, onSelectionChange, onApply }: {
  draft: string;
  context: SlashCommandContext;
  selection?: EditorSelection;
  disabled: boolean;
  onChange: (text: string) => void;
  onSelectionChange?: (selection: EditorSelection) => void;
  onApply: (text: string, selection: EditorSelection) => void;
}) {
  const input = useRef<V4FieldRef>(null);
  const mounted = useRef(true);
  const editor = useRef(new SlashEditorState(draft));
  editor.current.syncDraft(draft);
  const [focused, setFocused] = useState(false);
  const [observed, setObserved] = useState(editor.current.snapshot);
  const [dismissedKey, setDismissedKey] = useState<string | null>(null);
  const [freeformParent, setFreeformParent] = useState<string | null>(null);
  const caret = selection ?? (observed.draft === draft ? observed.selection : undefined);
  const query = slashQuery(draft, caret);
  const commands = getSlashSuggestions(draft, caret, context);
  const latest = useRef({ draft, caret, context, disabled, focused, query });
  latest.current = { draft, caret, context, disabled, focused, query };
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    if (freeformParent && leadingSlashParent(draft) !== freeformParent) setFreeformParent(null);
  }, [freeformParent, draft]);

  function select(id: string) {
    const current = latest.current;
    if (!mounted.current || current.disabled || !current.focused || !query || query.key !== current.query?.key ||
      current.draft !== editor.current.snapshot.draft) return;
    const command = getSlashSuggestions(current.draft, current.caret, current.context).find((entry) => entry.id === id);
    if (!command || command.disabledReason) return;
    const result = applySlashSuggestion(current.draft, current.caret, current.context, id, query.key);
    if (!result) return;
    const nextQuery = slashQuery(result.draft, result.selection);
    // Choosing a family opens its next level. Choosing the free-form gift
    // syntax leaves the recipient to the user rather than completing a keyword.
    setDismissedKey(!command.parent && nextQuery?.parent === command.id ? null : nextQuery?.key ?? null);
    setFreeformParent(command.parent && command.command === command.parent ? command.parent : null);
    setObserved(editor.current.changeText(result.draft, result.selection));
    onApply(result.draft, result.selection);
    input.current?.focus();
  }

  return {
    input,
    commands,
    visible: focused && !disabled && query !== null && query.key !== dismissedKey &&
      query.parent !== freeformParent && (!query.parent || commands.length > 0),
    select,
    dismiss: () => { setDismissedKey(query?.key ?? null); input.current?.focus(); },
    onFocus: () => setFocused(true),
    onBlur: () => setFocused(false),
    onChangeText: (text: string) => {
      // Native selection events follow edits. Until that event arrives, use
      // the new draft's end, never a stale selection from a different string.
      setObserved(editor.current.changeText(text));
      onChange(text);
    },
    onSelectionChange: (value: EditorSelection) => {
      setObserved(editor.current.changeSelection(value));
      onSelectionChange?.(value);
    },
  };
}
