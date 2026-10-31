import { v4, v4Metrics } from './v4/theme.ts';
import { v3Theme, v3Metrics } from './v3/theme.ts';

export type AppearanceMode = 'modern' | 'classic' | 'classic-v3';
export type AppearanceTheme = { readonly [Key in keyof typeof v4]: string };
export type AppearanceMetrics = { readonly [Key in keyof typeof v4Metrics]: number };

/** Keep the original saved `classic` value mapped to v4. */
export function parseAppearanceMode(value: string | null): AppearanceMode {
  return value === 'classic' || value === 'classic-v3' ? value : 'modern';
}

/** Modern keeps v4's teal, navy and orange identity; Classic stays source exact. */
export const modernTheme: AppearanceTheme = {
  body: '#f7fafb', text: '#162b3b', label: '#334b5b', border: '#d5e3e8',
  headerTop: '#0b5e7c', headerBottom: '#0b4b68', headerText: '#ffffff',
  activeTabTop: '#0b6480', activeTabBottom: '#0b6480', tabBorder: '#d5e3e8',
  inactiveTabTop: '#e8f1f4', inactiveTabBottom: '#e8f1f4', tabText: '#ffffff',
  unreadTop: '#ffcf83', unreadBottom: '#f59b49',
  stripTop: '#ffffff', stripBottom: '#ffffff',
  buttonTop: '#0b6480', buttonBottom: '#0b6480', buttonText: '#ffffff',
  menuHeader: '#ffffff', menuBody: '#ffffff', menuSelected: '#e8f1f4',
  menuText: '#162b3b', menuDisabled: '#7c8d99', menuBorder: '#d5e3e8',
  dialogBorder: '#d5e3e8', dialogBody: '#ffffff', dialogTitle: '#163d53',
  focusTop: '#b74a10', focus: '#b74a10', bevelLight: '#718692', bevelDark: '#718692',
  link: '#0b6480', selected: '#e8f1f4', selectedText: '#123e54',
  rowText: '#162b3b', secondaryText: '#526c7c',
  groupTop: '#eef5f7', groupBottom: '#eef5f7', groupBorder: '#e0eaee', groupText: '#365a6d',
  selfTop: '#0b6480', selfBottom: '#0b6480', subtle: '#eef4f6', muted: '#586f7f',
  transcript: '#162b3b',
};

/** Semantic equivalents for the system's Dark appearance, not inverted colors. */
export const modernDarkTheme: AppearanceTheme = {
  body: '#0b151c', text: '#edf5f8', label: '#cfdee5', border: '#2a3e49',
  headerTop: '#163d4b', headerBottom: '#102f3c', headerText: '#ffffff',
  activeTabTop: '#8bd5d8', activeTabBottom: '#8bd5d8', tabBorder: '#2a3e49',
  inactiveTabTop: '#1b303b', inactiveTabBottom: '#1b303b', tabText: '#10343e',
  unreadTop: '#ffd195', unreadBottom: '#f2a05c',
  stripTop: '#15242e', stripBottom: '#15242e',
  buttonTop: '#8bd5d8', buttonBottom: '#8bd5d8', buttonText: '#10343e',
  menuHeader: '#15242e', menuBody: '#15242e', menuSelected: '#23424d',
  menuText: '#edf5f8', menuDisabled: '#8c9fa9', menuBorder: '#2a3e49',
  dialogBorder: '#2a3e49', dialogBody: '#15242e', dialogTitle: '#edf5f8',
  focusTop: '#f8a169', focus: '#f8a169', bevelLight: '#809aa8', bevelDark: '#809aa8',
  link: '#8bd5d8', selected: '#23424d', selectedText: '#edf5f8',
  rowText: '#edf5f8', secondaryText: '#a9bbc5',
  groupTop: '#19323e', groupBottom: '#19323e', groupBorder: '#263c47', groupText: '#bad4df',
  selfTop: '#1b5264', selfBottom: '#174351', subtle: '#1b303b', muted: '#8c9fa9',
  transcript: '#edf5f8',
};

/** These are Modern semantic adaptations only. Original Classic theme fields
 * and packet-provided colors remain separate. The normal palette already
 * meets text/control thresholds; Increase Contrast strengthens their edges.
 */
export const modernHighContrastTheme: AppearanceTheme = {
  ...modernTheme, text: '#0c202e', label: '#183342', secondaryText: '#334b5b',
  muted: '#3e5666', border: '#526f7e', groupBorder: '#718692',
  bevelLight: '#426173', bevelDark: '#426173', focusTop: '#963806', focus: '#963806',
  link: '#064b61', selectedText: '#064358', transcript: '#0c202e',
};
export const modernDarkHighContrastTheme: AppearanceTheme = {
  ...modernDarkTheme, text: '#ffffff', label: '#edf5f8', secondaryText: '#d7e5eb',
  muted: '#b7cbd5', border: '#94adb9', groupBorder: '#809aa8',
  bevelLight: '#b9cdd7', bevelDark: '#b9cdd7', focusTop: '#ffc086', focus: '#ffc086',
  link: '#b4f1f2', selectedText: '#ffffff', transcript: '#ffffff',
};

export function modernAppearanceTheme(dark: boolean, increaseContrast = false): AppearanceTheme {
  return dark ? increaseContrast ? modernDarkHighContrastTheme : modernDarkTheme
    : increaseContrast ? modernHighContrastTheme : modernTheme;
}

/** Selection is a translucent tint over the existing native glass surface,
 * never an opaque replacement or a second glass layer. The outline and ink
 * also identify selection when the backing content changes.
 */
export function modernControlColors(dark: boolean, increaseContrast = false) {
  return {
    selectedFill: dark ? 'rgba(139,213,216,0.14)' : 'rgba(11,100,128,0.10)',
    selectedInk: dark ? increaseContrast ? '#e2ffff' : '#baf0f1' : increaseContrast ? '#064358' : '#075a71',
    selectedOutline: dark ? increaseContrast ? '#b4f1f2' : '#70aaaf' : increaseContrast ? '#064b61' : '#548997',
    glassTint: dark ? 'rgba(139,213,216,0.14)' : 'rgba(11,100,128,0.10)',
  };
}

export const modernMetrics: AppearanceMetrics = {
  font: 16, smallFont: 14, lineHeight: 24, headerHeight: 56,
  commandHeight: 56, touchTarget: 48, tabHeight: 44,
  icon: 22, avatar: 48, pageInset: 20,
};

/** 68pt navigation dock plus16pt breathing room for content beneath its glass. */
export const modernNavigationInset = 84;

export const appearanceThemes: Readonly<Record<AppearanceMode, AppearanceTheme>> = {
  modern: modernTheme, classic: v4, 'classic-v3': v3Theme,
};
export const appearanceMetrics: Readonly<Record<AppearanceMode, AppearanceMetrics>> = {
  modern: modernMetrics, classic: v4Metrics, 'classic-v3': v3Metrics,
};

export function avatarAppearance(label: string, dark: boolean) {
  const words = label.trim().split(/\s+/).filter(Boolean);
  const initials = [words[0], ...(words.length > 1 ? [words.at(-1)] : [])]
    .map((word) => Array.from(word ?? '')[0] ?? '').join('').toLocaleUpperCase();
  let hash = 0;
  for (const point of label.toLocaleLowerCase()) hash = (hash * 31 + point.codePointAt(0)!) >>> 0;
  const colors = dark
    ? [['#24454b', '#b3e2df'], ['#4c392c', '#f7d4b3'], ['#39364e', '#d4caef'], ['#244137', '#c0dfc7']]
    : [['#d7ebe9', '#265d5c'], ['#f3e1d3', '#815231'], ['#e7e1f1', '#62507f'], ['#dfeadf', '#3d6047']];
  const [background, foreground] = colors[hash % colors.length];
  return { initials, background, foreground };
}
