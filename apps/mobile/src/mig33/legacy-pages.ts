import { messageDescriptors as messages } from './generated/contracts.ts';
import { Packet, decodeUTF8, encodeUTF8 } from './protocol/index.ts';
import { imageDataURI } from './media.ts';
import type { OriginalRequest } from './client.ts';
export type { OriginalRequest } from './client.ts';

export interface LegacyNode { id: number; tag: string; attributes: Record<string, string>; children: LegacyNode[]; text?: string; }
export interface LegacyOption { value: string; label: string; disabled: boolean; }
export interface LegacyControl {
  id: number; name: string; kind: string; value: string; label: string;
  checked: boolean; disabled: boolean; maxLength?: number; options?: LegacyOption[];
}
export interface LegacyForm { id: number; action: string; method: 'GET' | 'POST'; controls: LegacyControl[]; }
export interface LegacyPage {
  id: number; url: string; status: number; title: string; body: LegacyNode; forms: LegacyForm[];
}
export type LegacyValues = Readonly<Record<number, string | boolean>>;
export interface NativePageAction { name: string; argument: string; }
export interface LegacyPageState { page: LegacyPage | null; loading: boolean; error: string; canGoBack: boolean; }

const voidTags = new Set(['area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta', 'param', 'source', 'wbr']);
const omittedTags = new Set(['script', 'style', 'iframe', 'object', 'embed', 'noscript']);
const entities: Record<string, string> = { amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: '\u00a0',
  copy: '©', reg: '®', hellip: '…', ndash: '–', mdash: '—', bull: '•', middot: '·' };

export function decodeEntities(value: string): string {
  return value.replace(/&(#x[\da-f]+|#\d+|[a-z]+);/gi, (original, entity: string) => {
    if (!entity.startsWith('#')) return entities[entity.toLowerCase()] ?? original;
    const number = entity[1].toLowerCase() === 'x' ? parseInt(entity.slice(2), 16) : parseInt(entity.slice(1), 10);
    return number > 0 && number <= 0x10ffff && !(number >= 0xd800 && number <= 0xdfff) ? String.fromCodePoint(number) : '�';
  });
}

export function legacyText(node: LegacyNode): string {
  return node.text ?? node.children.map(legacyText).join(node.tag === 'br' ? '\n' : '');
}
function cleanText(node: LegacyNode): string { return legacyText(node).replace(/\s+/g, ' ').trim(); }
function descendants(node: LegacyNode): LegacyNode[] { return [node, ...node.children.flatMap(descendants)]; }

/** Bounded, non-executing HTML subset used by the existing Go templ pages.
 * Script, stylesheets, event handlers and browser execution are never enabled.
 */
export function parseLegacyPage(html: string, url: string, status = 200, id = 0): LegacyPage {
  if (encodeUTF8(html).length > 1024 * 1024) throw new Error('The page is too large.');
  let nextID = 0;
  const root: LegacyNode = { id: nextID++, tag: 'document', attributes: {}, children: [] };
  const stack = [root];
  const tokens = html.match(/<!--[\s\S]*?(?:-->|$)|<![^>]*>|<\/?[a-z](?:"[^"]*"|'[^']*'|[^'">])*>|[^<]+|</gi) ?? [];
  for (const token of tokens) {
    if (nextID > 10000) throw new Error('The page contains too many elements.');
    if (token.startsWith('<!--') || token.startsWith('<!')) continue;
    const end = /^<\/\s*([a-z][\w:-]*)/i.exec(token);
    if (end) {
      const index = stack.map((node) => node.tag).lastIndexOf(end[1].toLowerCase());
      if (index > 0) stack.length = index;
      continue;
    }
    const start = /^<([a-z][\w:-]*)\b([\s\S]*?)\/?\s*>$/i.exec(token);
    if (start) {
      if (stack.length > 64) throw new Error('The page is nested too deeply.');
      const tag = start[1].toLowerCase();
      // HTML's optional closing tags matter for old pages as well as templ.
      if ((tag === 'p' || tag === 'li' || tag === 'option') && stack.at(-1)?.tag === tag) stack.pop();
      const attributes: Record<string, string> = {};
      const attribute = /([^\s=/'"<>]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s'"=<>`]+)))?/g;
      for (const match of start[2].matchAll(attribute)) {
        const name = match[1].toLowerCase();
        if (!name.startsWith('on') && !(name in attributes)) attributes[name] = decodeEntities(match[2] ?? match[3] ?? match[4] ?? '');
      }
      const node: LegacyNode = { id: nextID++, tag, attributes, children: [] };
      stack.at(-1)!.children.push(node);
      if (!voidTags.has(tag) && !token.endsWith('/>')) stack.push(node);
    } else {
      stack.at(-1)!.children.push({ id: nextID++, tag: '#text', attributes: {}, children: [], text: decodeEntities(token) });
    }
  }
  const all = descendants(root);
  const titleNode = all.find((node) => node.tag === 'title');
  const title = (titleNode ? cleanText(titleNode).slice(0, 180) : '') || 'mig33';
  const body = all.find((node) => node.tag === 'body') ?? root;
  function prune(node: LegacyNode): void {
    node.children = node.children.filter((child) => !omittedTags.has(child.tag) && child.tag !== 'head');
    node.children.forEach(prune);
  }
  prune(body);
  const forms = descendants(body).filter((node) => node.tag === 'form').map((form): LegacyForm => {
    let previous = '';
    const controls: LegacyControl[] = [];
    for (const node of descendants(form)) {
      if (node.tag === 'p' || node.tag === 'label') previous = cleanText(node);
      if (!['input', 'textarea', 'select', 'button'].includes(node.tag)) continue;
      const a = node.attributes;
      const kind = node.tag === 'input' ? (a.type ?? 'text').toLowerCase() : node.tag === 'button' ? (a.type ?? 'submit') : node.tag;
      const options = node.tag === 'select' ? descendants(node).filter((child) => child.tag === 'option').map((option) => ({
        value: option.attributes.value ?? legacyText(option), label: cleanText(option), disabled: 'disabled' in option.attributes,
      })) : undefined;
      const selected = node.tag === 'select' ? descendants(node).find((child) => child.tag === 'option' && 'selected' in child.attributes) : undefined;
      const maxLength = Number(a.maxlength ?? (node.tag === 'textarea' ? a.size : undefined));
      controls.push({ id: node.id, name: a.name ?? '', kind,
        value: node.tag === 'textarea' ? legacyText(node) : node.tag === 'select'
          ? selected?.attributes.value ?? (selected ? legacyText(selected) : options?.[0]?.value ?? '')
          : a.value ?? (kind === 'checkbox' || kind === 'radio' ? 'on' : kind === 'submit' ? cleanText(node) || 'Continue' : ''),
        label: a['aria-label'] || a.title || previous || a.name || 'Value', checked: 'checked' in a, disabled: 'disabled' in a,
        ...(Number.isInteger(maxLength) && maxLength > 0 && { maxLength }), ...(options && { options }),
      });
    }
    return { id: form.id, action: form.attributes.action || url,
      method: form.attributes.method?.toLowerCase() === 'post' ? 'POST' : 'GET', controls };
  });
  return { id, url, status, title, body, forms };
}

export function resolveLegacyURL(value: string, siteURL: string, currentURL?: string): string {
  const site = new URL(siteURL);
  const url = new URL(value, currentURL ?? siteURL);
  if (!['http:', 'https:'].includes(site.protocol) || url.origin !== site.origin || url.username || url.password) {
    throw new Error('Only pages from the connected mig33 server can open here.');
  }
  url.hash = '';
  return url.href;
}

export function parseNativeAction(value: string): NativePageAction | undefined {
  if (!value.startsWith('mig33:')) return undefined;
  const match = /^mig33:([A-Za-z][A-Za-z0-9]*?)\(([^\r\n]*)\)$/.exec(value);
  if (!match) throw new Error('This native link is malformed.');
  // The original command parser treats this as one literal argument. Do not
  // URL-decode it or evaluate it as JavaScript.
  return { name: match[1], argument: match[2] };
}

/** Go's original922 decoder protects an unrestricted final text field. Every
 * other value uses form encoding; the raw tail only escapes spaces, as the JAR
 * did. Using URLSearchParams for the whole body corrupts &, + and Unicode text.
 */
function rawTailField(url: URL): string | undefined {
  const c = url.searchParams.get('c'); const a = url.searchParams.get('a');
  if (c === 'report' && a === 'report_abuse') return 'details';
  if (c === 'registration') return 'password';
  if (c === 'account' && a === 'recover') return url.searchParams.get('stage') === 'complete' ? 'password' : 'mobile';
  if (c === 'profile' && a === 'edit') return 'value';
  if (c === 'settings' && a === 'change_password') return 'password';
  if (['group', 'account', 'store', 'merchant'].includes(c ?? '')) {
    const op = url.searchParams.get('op');
    return ({ create: 'name', 'create-details': 'description', post: 'body', recipient: 'username', member: 'username',
      ...(c === 'store' ? { purchase: 'note' } : {}) } as Record<string, string>)[op ?? ''];
  }
  if (c === 'chatroom') {
    if (a === 'setup') return url.searchParams.get('manage') === '1' ? 'body' : undefined;
    if (a === 'create') return url.searchParams.get('stage') === 'details' ? 'description' : 'name';
    if (a === 'search') return 'query';
  }
  if (c === 'profile' && a === 'search') return 'query';
  // Service pages put any unrestricted text in the final body field.
  if (['lookout', 'buzz', 'location', 'photo', 'photos', 'avatar', 'email', 'sms', 'call', 'bot', 'leaderboard', 'help', 'im', 'migworld'].includes(c ?? '')
    || c === 'profile' && a === 'user_blog_posts') return 'body';
  if (url.pathname === '/member2/t.php') return 'body';
  return undefined;
}

export function formEntries(form: LegacyForm, values: LegacyValues, submitID?: number): [string, string][] {
  const entries: [string, string][] = [];
  for (const control of form.controls) {
    if (control.disabled || !control.name || ['button', 'reset'].includes(control.kind)) continue;
    if (control.kind === 'file') throw new Error('Choose a photo with Share Photo before submitting this form.');
    if (control.kind === 'submit' && control.id !== submitID) continue;
    if (['checkbox', 'radio'].includes(control.kind)) {
      if (values[control.id] ?? control.checked) entries.push([control.name, control.value]);
      continue;
    }
    const value = control.kind === 'hidden' || control.kind === 'submit' ? control.value : String(values[control.id] ?? control.value);
    entries.push([control.name, value]);
  }
  return entries;
}

export function encodeLegacyForm(form: LegacyForm, values: LegacyValues, action: string, submitID?: number): string {
  const entries = formEntries(form, values, submitID);
  const tail = form.method === 'POST' ? rawTailField(new URL(action)) : undefined;
  const prefix = entries.filter(([name]) => name !== tail).map(([name, value]) => `${encodeURIComponent(name)}=${encodeURIComponent(value)}`).join('&');
  const last = entries.filter(([name]) => name === tail);
  if (last.length > 1) throw new Error('This form contains duplicate text fields.');
  if (last[0]?.[1].includes('%20')) throw new Error('This legacy form cannot preserve the literal text %20.');
  const result = prefix + (last.length ? (prefix ? '&' : '') + `${last[0][0]}=${last[0][1].replaceAll(' ', '%20')}` : '');
  if (encodeUTF8(result).length > 8192) throw new Error('This form is too large to send.');
  return result;
}

export class LegacyPages {
  private state: LegacyPageState = { page: null, loading: false, error: '', canGoBack: false };
  private readonly listeners = new Set<() => void>();
  private readonly history: LegacyPage[] = [];
  private readonly images = new Map<string, Promise<string>>();
  private generation = 0;
  private lifecycle = 0;
  private revision = 0;
  private lastURL?: string;
  private cookies = new Map<string, string>();
  private readonly request: OriginalRequest;
  readonly siteURL: string;
  private readonly nativeAction: (action: NativePageAction) => void | Promise<void>;
  constructor(request: OriginalRequest, siteURL: string,
    nativeAction: (action: NativePageAction) => void | Promise<void>) {
    this.request = request; this.siteURL = siteURL; this.nativeAction = nativeAction;
  }
  readonly getSnapshot = (): LegacyPageState => this.state;
  readonly subscribe = (listener: () => void): (() => void) => { this.listeners.add(listener); return () => this.listeners.delete(listener); };
  private update(change: Partial<LegacyPageState>): void {
    this.state = { ...this.state, ...change, canGoBack: this.history.length > 0 };
    for (const listener of this.listeners) listener();
  }
  private async fetch(url: string, body?: string, purpose = 1): Promise<Packet> {
    const lifecycle = this.lifecycle;
    const cookies = [...this.cookies].map(([name, value]) => `${name}=${value}`).join('; ');
    const [reply] = await this.request(messages.Client922.opcode, (packet) => {
      packet.setByte(1, body === undefined ? 1 : 2).setText(2, url).setInt(4, purpose).setInt(6, 0);
      for (const field of [7, 8, 9, 10]) packet.setText(field, '');
      if (body !== undefined) packet.setText(3, body);
      if (cookies) packet.setStringArray(11, [cookies]);
    }, [messages.Server923.opcode], messages.Server923.opcode);
    if (lifecycle !== this.lifecycle) throw new Error('The page session ended.');
    if (!reply || reply.int(1) === undefined) throw new Error('The server returned an incomplete page response.');
    for (const cookie of reply.stringArray(11) ?? []) {
      const [pair, ...attributes] = cookie.split(';');
      const split = pair.indexOf('=');
      if (split <= 0 || /[\r\n]/.test(cookie)) continue;
      const name = pair.slice(0, split).trim(); const value = pair.slice(split + 1).trim();
      const domain = attributes.find((part) => part.trim().toLowerCase().startsWith('domain='))?.split('=')[1]?.trim().replace(/^\./, '');
      if (domain && domain !== new URL(url).hostname) continue;
      if (attributes.some((part) => /^\s*max-age=0\s*$/i.test(part))) this.cookies.delete(name);
      else if (this.cookies.size < 32 || this.cookies.has(name)) this.cookies.set(name, value);
    }
    return reply;
  }
  readonly open = async (value: string): Promise<void> => { await this.navigate(value); };
  private async navigate(value: string, body?: string, remember = true): Promise<void> {
    const generation = ++this.generation;
    this.update({ loading: true, error: '' });
    try {
      const url = resolveLegacyURL(value, this.siteURL, this.state.page?.url);
      this.lastURL = url;
      const reply = await this.fetch(url, body);
      if (generation !== this.generation) return;
      const page = parseLegacyPage(decodeUTF8(reply.bytes(2) ?? new Uint8Array()), url, reply.int(1), ++this.revision);
      if (remember && this.state.page) {
        this.history.push(this.state.page);
        if (this.history.length > 30) this.history.shift();
      }
      this.update({ page, loading: false });
    } catch (error) {
      if (generation === this.generation) this.update({ loading: false, error: error instanceof Error ? error.message : String(error) });
    }
  }
  readonly follow = async (href: string): Promise<void> => {
    // A link cannot replace a pending form result or start a native mutation
    // while that form is still being submitted. Explicit app navigation uses
    // open(), whose generation guard continues to allow deliberate replacement.
    if (this.state.loading) return;
    try {
      const action = parseNativeAction(href);
      if (!action) await this.open(href);
      else if (action.name === 'newBrowser') await this.open(action.argument);
      else await this.nativeAction(action);
    } catch (error) { this.update({ error: error instanceof Error ? error.message : String(error) }); }
  };
  readonly submit = async (form: LegacyForm, values: LegacyValues, submitID?: number): Promise<void> => {
    if (this.state.loading) return;
    try {
      if (!this.state.page?.forms.includes(form)) throw new Error('This form is no longer current. Open it again.');
      const action = resolveLegacyURL(form.action, this.siteURL, this.state.page.url);
      const body = encodeLegacyForm(form, values, action, submitID);
      if (form.method === 'GET') {
        const target = new URL(action);
        for (const [name, value] of new URLSearchParams(body)) target.searchParams.append(name, value);
        await this.navigate(target.href);
      } else await this.navigate(action, body);
    } catch (error) { this.update({ error: error instanceof Error ? error.message : String(error) }); }
  };
  readonly back = (): boolean => {
    const page = this.history.pop();
    if (!page) return false;
    this.generation++; this.update({ page, loading: false, error: '' }); return true;
  };
  readonly refresh = async (): Promise<void> => {
    // Refresh never automatically repeats a purchase, report or other POST.
    const url = this.state.page?.url ?? this.lastURL;
    if (url) await this.navigate(url, undefined, false);
  };
  readonly close = (): void => {
    this.generation++; this.lifecycle++; this.history.length = 0; this.cookies.clear(); this.images.clear();
    this.lastURL = undefined;
    this.update({ page: null, loading: false, error: '' });
  };
  readonly loadImage = (value: string): Promise<string> => {
    const url = resolveLegacyURL(value, this.siteURL, this.state.page?.url);
    const existing = this.images.get(url);
    if (existing) return existing;
    const result = this.fetch(url, undefined, 2).then((reply) => {
      if (reply.int(1) !== 200) throw new Error('Image unavailable.');
      const uri = imageDataURI(reply.bytes(2) ?? new Uint8Array());
      if (!uri) throw new Error('The server did not return a supported image.');
      return uri;
    }).catch((error) => { this.images.delete(url); throw error; });
    if (this.images.size >= 64) this.images.delete(this.images.keys().next().value!);
    this.images.set(url, result);
    return result;
  };
}
