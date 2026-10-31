/** Bounded CSS color support for the native page subset. These values are
 * presentation only: the original page tree and922/923 payload stay intact.
 */
interface RGBA { rgb: readonly number[]; alpha: number; }
export interface PagePalette { body: string; text: string; link: string; }
export interface PageColors {
  background: string;
  foreground: string;
  link: string;
  authoredForeground?: string;
  withinLink: boolean;
  localBackground?: string;
}
const named: Readonly<Record<string, string>> = {
  black: '#000000', white: '#ffffff', red: '#ff0000', green: '#008000', blue: '#0000ff',
  yellow: '#ffff00', orange: '#ffa500', brown: '#a52a2a', purple: '#800080', pink: '#ffc0cb',
  gray: '#808080', grey: '#808080', silver: '#c0c0c0', navy: '#000080', teal: '#008080',
  aqua: '#00ffff', cyan: '#00ffff', lime: '#00ff00', maroon: '#800000', olive: '#808000',
  fuchsia: '#ff00ff', magenta: '#ff00ff', transparent: '#00000000',
};
const clamp = (value: number, maximum: number) => Math.max(0, Math.min(maximum, value));
function parse(value: string | undefined): RGBA | undefined {
  if (!value) return;
  const text = named[value.trim().toLowerCase()] ?? value.trim().toLowerCase();
  if (/^#[\da-f]{3,4}$/.test(text)) {
    const channels = [...text.slice(1)].map((part) => Number.parseInt(part + part, 16));
    return { rgb: channels.slice(0, 3), alpha: (channels[3] ?? 255) / 255 };
  }
  if (/^#(?:[\da-f]{6}|[\da-f]{8})$/.test(text)) {
    return { rgb: [1, 3, 5].map((offset) => Number.parseInt(text.slice(offset, offset + 2), 16)),
      alpha: text.length === 9 ? Number.parseInt(text.slice(7, 9), 16) / 255 : 1 };
  }
  const match = /^rgba?\(([^()]*)\)$/.exec(text);
  if (!match) return;
  const parts = match[1].trim().split(/[\s,/]+/);
  if (![3, 4].includes(parts.length) || parts.some((part) => !/^[+-]?(?:\d+\.?\d*|\.\d+)%?$/.test(part))) return;
  const rgb = parts.slice(0, 3).map((part) => clamp(part.endsWith('%') ? Number.parseFloat(part) * 255 / 100 : Number.parseFloat(part), 255));
  const alpha = parts[3] ? clamp(Number.parseFloat(parts[3]) / (parts[3].endsWith('%') ? 100 : 1), 1) : 1;
  if ([...rgb, alpha].some((value) => !Number.isFinite(value))) return;
  return { rgb, alpha };
}
function hex(rgb: readonly number[]): string {
  return '#' + rgb.map((value) => Math.round(value).toString(16).padStart(2, '0')).join('');
}
export function pageColor(value: string | undefined): string | undefined {
  const parsed = parse(value);
  return parsed && hex(parsed.rgb) + (parsed.alpha === 1 ? '' : Math.round(parsed.alpha * 255).toString(16).padStart(2, '0'));
}
function over(foreground: RGBA, background: readonly number[]): number[] {
  return foreground.rgb.map((value, index) => value * foreground.alpha + background[index] * (1 - foreground.alpha));
}
function luminance(rgb: readonly number[]): number {
  const values = rgb.map((value) => { const c = value / 255; return c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4; });
  return values[0] * .2126 + values[1] * .7152 + values[2] * .0722;
}
function readable(color: string, background: string): boolean {
  const candidate = parse(color), surface = parse(background);
  if (!candidate || !surface) return false;
  const a = luminance(over(candidate, surface.rgb)), b = luminance(surface.rgb);
  return (Math.max(a, b) + .05) / (Math.min(a, b) + .05) >= 4.5;
}
function foreground(candidate: string | undefined, background: string, palette: PagePalette, link: boolean): string {
  const choices = [candidate, link ? palette.link : palette.text,
    ...(link ? ['#075a71', '#baf0f1'] : ['#162b3b', '#edf5f8']), '#000000', '#ffffff'];
  return choices.find((value): value is string => !!value && readable(value, background))!;
}
export function pageBaseColors(palette: PagePalette): PageColors {
  const background = pageColor(palette.body) ?? '#ffffff';
  return { background, foreground: foreground(undefined, background, palette, false),
    link: foreground(undefined, background, palette, true), withinLink: false };
}

/** Retain the authored foreground separately from the corrected paint color.
 * It can become legible again inside a differently colored descendant.
 */
export function pageNodeColors(attributes: Readonly<Record<string, string>>, parent: PageColors, palette: PagePalette,
  options: { link?: boolean; body?: boolean; surface?: string } = {}): PageColors {
  let authored = pageColor(attributes.color ?? (options.body ? attributes.text : undefined));
  let backgroundColor = options.body ? pageColor(attributes.bgcolor) : undefined;
  for (const declaration of (attributes.style ?? '').split(';')) {
    const colon = declaration.indexOf(':');
    if (colon < 0) continue;
    const key = declaration.slice(0, colon).trim().toLowerCase();
    const value = pageColor(declaration.slice(colon + 1));
    if (key === 'color' && value) authored = value;
    if (key === 'background-color' && value) backgroundColor = value;
  }
  const base = pageColor(options.surface) ?? parent.background;
  const background = backgroundColor ? hex(over(parse(backgroundColor)!, parse(base)!.rgb)) : base;
  const withinLink = options.link || parent.withinLink;
  const inherited = options.link && !parent.withinLink ? undefined : parent.authoredForeground;
  const authoredForeground = authored ?? inherited;
  return { background, foreground: foreground(authoredForeground, background, palette, withinLink),
    link: foreground(undefined, background, palette, true), authoredForeground, withinLink,
    ...(backgroundColor && { localBackground: background }) };
}
