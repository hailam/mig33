import { slashCommands, type SlashCommandDefinition } from '../mig33/generated/slash-commands.ts';
import type { EditorSelection } from './composer.ts';

export interface SlashCommandContext {
  kind: 'private' | 'group' | 'room';
  /** Omit when the caller already represents a joined conversation. */
  joined?: boolean;
  communityID?: number;
  muted?: boolean;
}

export type SlashSuggestion = Readonly<SlashCommandDefinition> & {
  /** Local editor capacity, separate from catalog/server permission metadata. */
  readonly disabledReason?: string;
};
export type SlashCommandSuggestion = SlashSuggestion;

export interface SlashEditorObservation { draft: string; selection?: EditorSelection; }

/** Native text and selection callbacks can arrive before React commits a new
 * draft prop. Associate both callbacks synchronously with the edited string;
 * repeated renders of the old prop must not undo that observation. */
export class SlashEditorState {
  private renderedDraft: string;
  private value: SlashEditorObservation;

  constructor(draft: string) { this.renderedDraft = draft; this.value = { draft }; }

  get snapshot(): SlashEditorObservation { return this.value; }

  syncDraft(draft: string): void {
    if (draft === this.renderedDraft) return;
    this.renderedDraft = draft;
    if (this.value.draft !== draft) this.value = { draft };
  }

  changeText(draft: string, selection?: EditorSelection): SlashEditorObservation {
    return this.value = { draft, selection };
  }

  changeSelection(selection: EditorSelection): SlashEditorObservation {
    return this.value = { draft: this.value.draft, selection };
  }
}

export interface SlashQuery {
  /** Identifies the exact draft/caret snapshot for dismissal and stale taps. */
  key: string;
  /** UTF-16 range of the entire command, including its leading slash. */
  start: number;
  end: number;
  /** Case-folded command prefix before the caret, without its slash. */
  prefix: string;
  /** Catalog ID of the parent when completing a gift/bot subcommand. */
  parent?: string;
}

const catalog: readonly SlashCommandDefinition[] = slashCommands;
const spelling = (value: string) => value.replace(/^\//, '').toLowerCase();
const spellings = (command: SlashCommandDefinition) => [command.command, ...(command.aliases ?? [])].map(spelling);

/** Unlike slashQuery, a family's leading prefix does not depend on the caret
 * or on whether the person has reached its free-form arguments. */
export function leadingSlashParent(draft: string): string | null {
  const first = /^\s*\/([a-z0-9]+)(?=\s|$)/i.exec(draft)?.[1].toLowerCase();
  if (!first) return null;
  const parent = catalog.find((command) => !command.parent && spellings(command).includes(first));
  return parent && catalog.some((command) => command.parent === parent.id) ? parent.id : null;
}

/** Only the leading command token, or a known parent's next token, completes.
 * Arguments and unrelated slashes never become replacement ranges. Positions
 * use JavaScript/UITextInput UTF-16 units, matching the existing composer. */
export function slashQuery(draft: string, selection?: EditorSelection): SlashQuery | null {
  const caret = selection === undefined ? draft.length : selection.start;
  if (!Number.isInteger(caret) || caret < 0 || caret > draft.length ||
    (selection !== undefined && selection.end !== caret)) return null;
  const start = draft.search(/\S/);
  if (start < 0 || draft[start] !== '/' || caret <= start) return null;
  const firstStart = start + 1;
  const first = /^\S*/.exec(draft.slice(firstStart))![0];
  if (!/^[a-z0-9]*$/i.test(first)) return null;
  const firstEnd = firstStart + first.length;
  const key = JSON.stringify([draft, caret]);
  if (caret <= firstEnd) return { key, start, end: firstEnd, prefix: draft.slice(firstStart, caret).toLowerCase() };

  const parent = catalog.find((command) => !command.parent && spellings(command).includes(first.toLowerCase()));
  if (!parent || !catalog.some((command) => command.parent === parent.id)) return null;
  const gap = /^\s*/.exec(draft.slice(firstEnd))![0];
  const childStart = firstEnd + gap.length;
  if (caret < childStart) return null;
  const child = /^\S*/.exec(draft.slice(childStart))![0];
  const end = childStart + child.length;
  if (caret > end || !/^[a-z0-9]*$/i.test(child)) return null;
  return { key, start, end, parent: parent.id,
    prefix: `${spelling(parent.command)} ${draft.slice(childStart, caret).toLowerCase()}` };
}

function eligible(command: SlashCommandDefinition, context: SlashCommandContext): boolean {
  if (context.kind !== 'private' && context.kind !== 'group' && context.kind !== 'room') return false;
  if (context.kind !== 'private' && context.joined === false) return false;
  if (context.muted && !command.allowMuted) return false;
  if (command.scope === 'room') return context.kind === 'room';
  if (command.scope === 'community') return context.kind === 'room' &&
    Number.isInteger(context.communityID) && context.communityID! > 0;
  return true;
}

/** Requirements annotate server-owned permissions. In particular, a room's
 * moderator marker does not prove the person's community Group role. */
export function getSlashSuggestions(draft: string, selection: EditorSelection | undefined,
  context: SlashCommandContext): readonly SlashSuggestion[] {
  const query = slashQuery(draft, selection);
  if (!query) return [];
  const parentCommand = query.parent ? catalog.find((command) => command.id === query.parent)?.command : undefined;
  // A family's generic argument template ("Send gift") has no literal
  // subcommand. Offer it only before any recipient token exists, so choosing
  // it cannot erase a partially typed recipient or invent "/gift send".
  const emptyChild = parentCommand !== undefined && query.prefix === `${spelling(parentCommand)} ` &&
    query.end === (selection === undefined ? draft.length : selection.start);
  return catalog.filter((command) => command.parent === query.parent && eligible(command, context) &&
    (spellings(command).some((name) => name.startsWith(query.prefix)) ||
      (emptyChild && command.command === parentCommand))).map((command) =>
    insertCommand(draft, query, command).draft.length <= 320 ? command : {
      ...command, disabledReason: 'Shorten this message to insert the command. The limit is 320 characters.',
    });
}

function insertCommand(draft: string, query: SlashQuery, command: SlashCommandDefinition): {
  draft: string; selection: EditorSelection;
} {
  const replacement = command.insert.trimEnd();
  let suffix = draft.slice(query.end);
  if (!suffix && command.arguments !== 'none') suffix = ' ';
  const result = draft.slice(0, query.start) + replacement + suffix;
  // Existing whitespace and argument text are retained exactly. Put the caret
  // before those arguments, or after the inserted separator for a new command.
  const caret = query.start + replacement.length + /^\s*/.exec(suffix)![0].length;
  return { draft: result, selection: { start: caret, end: caret } };
}

/** Replace only the command span. Selection never sends, fills placeholders,
 * drops existing arguments, or truncates a draft to fit the native limit. */
export function applySlashSuggestion(draft: string, selection: EditorSelection | undefined,
  context: SlashCommandContext, commandId: string, expectedQueryKey?: string): {
    draft: string; selection: EditorSelection;
  } | undefined {
  const query = slashQuery(draft, selection);
  if (!query || (expectedQueryKey !== undefined && expectedQueryKey !== query.key)) return undefined;
  const command = getSlashSuggestions(draft, selection, context).find((entry) => entry.id === commandId);
  if (!command || command.disabledReason) return undefined;
  return insertCommand(draft, query, command);
}
