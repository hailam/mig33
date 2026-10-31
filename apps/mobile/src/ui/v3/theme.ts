import type { AppearanceMetrics, AppearanceTheme } from '../appearanceTheme.ts';

/** mig33 3.08's literal renderer colors, not a v4 opcode1000 theme.
 * Evidence: research/applets/v3/docs/rendering.md and data/colors.json.
 * Source paths below are relative to v3/artifacts/vineflower/.
 */
export const v3Colors = {
  // com/projectgoth/Roshan/d.java:64–97: canvas, title and control bevel.
  white: '#ffffff', black: '#000000', header: '#aaccee',
  bevelLight: '#9495a2', bevelDark: '#5a5b64', buttonBottom: '#bcd4f6',
  // d.java:111–125: five edge strokes; the interior does not turn gold.
  focus: ['#fff0cf', '#fdd889', '#fbca6a', '#f8b330', '#e59700'],
  // controller c.java:498–521; r.java:814–846: content and selection.
  surround: '#8eb3e7', selectionOutline: '#335599', contentBevel: '#88aadd', selected: '#f2f5fb',
  // ab.java:283–398: slanted, overlapping tabs and unread flash colors.
  activeTabTop: '#d5e6fb', activeTabBottom: '#8eb3e7',
  inactiveTabTop: '#e4eefc', inactiveTabBottom: '#c0d8f9', tabBorder: '#3b619c',
  stripTop: '#c5daf7', stripBottom: '#f3f8fe',
  unreadTop: '#f5c258', unreadBottom: '#ff8000',
  // e.java:283–329,385–431: centered overlays, salmon error header.
  shadow: '#a0a0a0', disabled: '#c0c0c0', errorHeader: '#ff8080',
  // Roshan.d.a(int):(I)I, called by v: transcript categories0–6.
  transcript: ['#086834', '#406ac0', '#ff0000', '#ff7300', '#fcc504', '#993300', '#000000'],
} as const;

const c = v3Colors;
export const v3MessageColors = {
  0: c.transcript[0], 1: c.transcript[1], 2: c.transcript[2], 3: c.transcript[3],
  4: c.transcript[4], 5: c.transcript[5], 6: c.transcript[6],
} as const;
export const v3Theme: AppearanceTheme = {
  body: c.white, text: c.black, label: c.black, border: c.surround,
  headerTop: c.header, headerBottom: c.header, headerText: c.black,
  activeTabTop: c.activeTabTop, activeTabBottom: c.activeTabBottom, tabBorder: c.tabBorder,
  inactiveTabTop: c.inactiveTabTop, inactiveTabBottom: c.inactiveTabBottom, tabText: c.black,
  unreadTop: c.unreadTop, unreadBottom: c.unreadBottom,
  stripTop: c.stripTop, stripBottom: c.stripBottom,
  buttonTop: c.white, buttonBottom: c.buttonBottom, buttonText: c.black,
  menuHeader: c.header, menuBody: c.white, menuSelected: c.selected,
  menuText: c.black, menuDisabled: c.disabled, menuBorder: c.black,
  dialogBorder: c.black, dialogBody: c.white, dialogTitle: c.black,
  focusTop: c.focus[0], focus: c.focus[2], bevelLight: c.bevelLight, bevelDark: c.bevelDark,
  link: '#0000ff', selected: c.selected, selectedText: c.black,
  rowText: c.black, secondaryText: c.black,
  groupTop: c.white, groupBottom: c.white, groupBorder: c.selectionOutline, groupText: c.selectionOutline,
  selfTop: c.white, selfBottom: c.white, subtle: c.white, muted: c.disabled,
  transcript: c.black,
};

/** Touch adaptation: source geometry is font-derived, with12px source icons.
 * Keep44pt hit targets while preserving the source's rectangular controls.
 */
export const v3Metrics: AppearanceMetrics = {
  font: 15, smallFont: 13, lineHeight: 20, headerHeight: 28,
  commandHeight: 44, touchTarget: 44, tabHeight: 44,
  icon: 16, avatar: 40, pageInset: 6,
};
