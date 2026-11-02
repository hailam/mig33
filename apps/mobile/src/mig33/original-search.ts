import {
  LegacyPages, legacyText, parseNativeAction, resolveLegacyURL,
  type LegacyControl, type LegacyForm, type LegacyNode, type LegacyPage, type OriginalRequest,
} from './legacy-pages.ts';

export type OriginalSearchKind = 'people' | 'rooms';
export interface OriginalSearchFilters { readonly language?: string; readonly keywords?: string; }
export interface OriginalSearchAction { readonly label: string; readonly href: string; }
export interface OriginalSearchResult {
  readonly key: string;
  readonly title: string;
  readonly detail: string;
  readonly actions: readonly OriginalSearchAction[];
}
export interface OriginalSearchState {
  readonly query: string;
  readonly language: string;
  readonly keywords: string;
  readonly notice: string;
  readonly results: readonly OriginalSearchResult[];
  readonly loading: boolean;
  readonly searched: boolean;
  readonly error: string;
}

const idle: OriginalSearchState = { query: '', language: '', keywords: '', notice: '', results: [], loading: false, searched: false, error: '' };
const controller = (kind: OriginalSearchKind) => kind === 'people' ? 'profile' : 'chatroom';
const text = (node: LegacyNode) => legacyText(node).replace(/\s+/g, ' ').trim();
const nodes = (node: LegacyNode): LegacyNode[] => [node, ...node.children.flatMap(nodes)];

/** Select by the original route and successful controls, never HTML/AX order.
 * The room page has three different forms whose input is named query.
 */
export function originalSearchForm(page: LegacyPage, siteURL: string, kind: OriginalSearchKind): {
  form: LegacyForm; input: LegacyControl; submit: LegacyControl;
} {
  const matches = page.forms.flatMap((form) => {
    const action = new URL(resolveLegacyURL(form.action, siteURL, page.url));
    if (form.method !== 'POST' || action.pathname !== new URL(siteURL).pathname
      || action.searchParams.getAll('c').length !== 1 || action.searchParams.get('c') !== controller(kind)
      || action.searchParams.getAll('a').length !== 1 || action.searchParams.get('a') !== 'search'
      || action.searchParams.getAll('v').length !== 1 || action.searchParams.get('v') !== 'midlet') return [];
    const fields = action.searchParams.getAll('field');
    if (fields.length > 1 || fields.length === 1 && fields[0] !== 'query') return [];
    const editable = form.controls.filter((control) => !['hidden', 'submit', 'button', 'reset'].includes(control.kind));
    const input = editable[0];
    const submits = form.controls.filter((control) => control.kind === 'submit' && !control.disabled);
    if (editable.length !== 1 || !input || input.name !== 'query' || !['text', 'search'].includes(input.kind)
      || input.disabled || submits.length !== 1) return [];
    return [{ form, input, submit: submits[0] }];
  });
  if (matches.length !== 1) throw new Error('The server did not return a supported search form.');
  return matches[0];
}

/** These are the actual server-authored paragraphs and links, not inferred
 * roster presence or room permissions. Forms are excluded from result data.
 */
function blocks(root: LegacyNode): LegacyNode[] {
  if (root.tag === 'form') return [];
  if (['p', 'h1', 'h2', 'h3', 'li', 'a'].includes(root.tag)) return [root];
  return root.children.flatMap(blocks);
}

function action(node: LegacyNode, page: LegacyPage, siteURL: string): OriginalSearchAction {
  const href = node.attributes.href;
  const label = text(node);
  if (!href || !label) throw new Error('The server returned an incomplete search action.');
  const native = parseNativeAction(href);
  return { label, href: native ? href : resolveLegacyURL(href, siteURL, page.url) };
}

function identity(href: string, kind: OriginalSearchKind, siteURL: string): string | undefined {
  const native = parseNativeAction(href);
  if (native) {
    const names = kind === 'people' ? ['privateChat', 'inviteFriend'] : ['joinChatroom'];
    return names.includes(native.name) && native.argument ? native.argument : undefined;
  }
  const url = new URL(href);
  const c = kind === 'people' ? 'profile' : 'chatroom';
  const a = kind === 'people' ? 'home' : 'setup';
  const field = kind === 'people' ? 'username' : 'roomName';
  return url.pathname === new URL(siteURL).pathname && url.searchParams.getAll('c').length === 1
    && url.searchParams.get('c') === c && url.searchParams.getAll('a').length === 1 && url.searchParams.get('a') === a
    && url.searchParams.get('v') === 'midlet'
    && url.searchParams.getAll(field).length === 1 ? url.searchParams.get(field) || undefined : undefined;
}

/** Project the supported original HTML's result groups. Preserve link labels,
 * ordering and literal native arguments; unknown responses fail visibly.
 */
export function originalSearchResults(page: LegacyPage, siteURL: string, kind: OriginalSearchKind): readonly OriginalSearchResult[] {
  const results: OriginalSearchResult[] = [];
  const occurrences = new Map<string, number>();
  let pending: { title: string; detail: string[]; actions: OriginalSearchAction[] } | undefined;
  let sawEmpty = false;
  function finish() {
    if (!pending?.actions.length) { pending = undefined; return; }
    const identities = [...new Set(pending.actions.map((entry) => identity(entry.href, kind, siteURL)).filter((value) => value !== undefined))];
    if (identities.length !== 1) throw new Error('The server returned an unfamiliar search result.');
    const id = identities[0];
    const key = `${kind}:${id}`;
    const occurrence = occurrences.get(key) ?? 0;
    occurrences.set(key, occurrence + 1);
    results.push({ key: occurrence ? `${key}:${occurrence}` : key, title: pending.title || id,
      detail: kind === 'people' ? [id, ...pending.detail].join('\n') : pending.detail.join('\n'), actions: pending.actions });
    pending = undefined;
  }
  for (const block of blocks(page.body)) {
    const content = text(block);
    if (!content) continue;
    if (content === (kind === 'people' ? 'No friends found.' : 'No chat rooms found.')) sawEmpty = true;
    const all = nodes(block);
    const links = all.filter((node) => node.tag === 'a' && node.attributes.href).map((node) => action(node, page, siteURL));
    const heading = all.find((node) => ['b', 'strong', 'h1', 'h2', 'h3'].includes(node.tag));
    if (heading && !links.length) {
      finish();
      pending = { title: text(heading), detail: [], actions: [] };
    } else if (links.length) {
      if (!pending) throw new Error('The server returned an unfamiliar search result.');
      pending.actions.push(...links);
    } else if (pending) pending.detail.push(content);
  }
  finish();
  if (results.length === 0 && !sawEmpty) throw new Error('The server returned an unfamiliar search response.');
  return results;
}

function searchNotice(page: LegacyPage, kind: OriginalSearchKind): string {
  if (kind !== 'rooms') return '';
  // Retain the server's explicit limit explanation; never infer a total
  // from the number of returned rows.
  return blocks(page.body).map(text).find((value) =>
    value === 'Showing up to 100 rooms. Narrow your search to find a room.') ?? '';
}

function responsePage(browser: LegacyPages): LegacyPage {
  const state = browser.getSnapshot();
  if (state.error) throw new Error(state.error);
  if (!state.page) throw new Error('The server did not return a search page.');
  if (state.page.status < 200 || state.page.status >= 300) {
    const message = blocks(state.page.body).map(text).filter(Boolean).join(' ').slice(0, 500);
    throw new Error(message || `Search failed (${state.page.status}).`);
  }
  return state.page;
}

/** Each search owns an independent original922/923 page session. Canceling
 * invalidates replies without closing the shared chat connection or changing
 * the application's page history. No new route or wire representation exists.
 */
export class OriginalSearch {
  private state: OriginalSearchState = idle;
  private readonly listeners = new Set<() => void>();
  private generation = 0;
  private browser?: LegacyPages;
  private readonly request: OriginalRequest;
  private readonly siteURL: string;
  private readonly kind: OriginalSearchKind;
  constructor(request: OriginalRequest, siteURL: string, kind: OriginalSearchKind) {
    this.request = request; this.siteURL = siteURL; this.kind = kind;
  }
  readonly getSnapshot = (): OriginalSearchState => this.state;
  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener); return () => { this.listeners.delete(listener); };
  };
  private update(state: OriginalSearchState) { this.state = state; for (const listener of this.listeners) listener(); }
  readonly clear = (): void => {
    this.generation++; this.browser?.close(); this.browser = undefined; this.update(idle);
  };
  readonly close = (): void => { this.clear(); };
  readonly search = async (value: string, filters: OriginalSearchFilters = {}): Promise<void> => {
    const query = value.trim();
    const language = this.kind === 'rooms' ? (filters.language ?? '').trim() : '';
    const keywords = this.kind === 'rooms' ? (filters.keywords ?? '').trim() : '';
    const submitted = { query, language, keywords };
    const generation = ++this.generation;
    this.browser?.close();
    const browser = new LegacyPages(this.request, this.siteURL, () => {});
    this.browser = browser;
    this.update({ ...submitted, notice: '', results: [], loading: true, searched: false, error: '' });
    try {
      const route = new URL(this.siteURL);
      route.search = new URLSearchParams({ c: controller(this.kind), a: 'search', v: 'midlet' }).toString();
      // Fetch the original form with its server-bound filters. Its returned
      // action carries these values into the unchanged single-query POST.
      if (this.kind === 'rooms') {
        route.searchParams.set('language', language);
        route.searchParams.set('keywords', keywords);
      }
      await browser.open(route.href);
      if (generation !== this.generation) return;
      const { form, input, submit } = originalSearchForm(responsePage(browser), this.siteURL, this.kind);
      await browser.submit(form, { [input.id]: query }, submit.id);
      if (generation !== this.generation) return;
      const page = responsePage(browser);
      const results = originalSearchResults(page, this.siteURL, this.kind);
      this.update({ ...submitted, results, notice: searchNotice(page, this.kind), loading: false, searched: true, error: '' });
    } catch (error) {
      if (generation === this.generation) this.update({ ...submitted, notice: '', results: [], loading: false, searched: true,
        error: error instanceof Error ? error.message : String(error) });
    } finally {
      browser.close();
      if (this.browser === browser) this.browser = undefined;
    }
  };
}
