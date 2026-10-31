/**
 * Default V2, mig33 4.62 build .000. Values are copied from the original
 * opcode1000 packet, inventoried in research/applets/v4/docs/data/theme.json.
 * Numeric tags stay beside values so a visual change can be checked against
 * the original packet. Roles come from rendering.md and theme-usage.json.
 * Dimensions below are the explicit touch adaptation, not recovered pixels.
 */
export const v4ThemeFields = {
  3: '#ffffff', 4: '#d5d5d5', 5: '#098fb6', 6: '#0c4b7b',
  8: '#0f95d3', 9: '#05bad3', 11: '#efcf06', 12: '#fe8300', 13: '#f9ffb3',
  14: '#02485d', 17: '#ffffff', 18: '#ffffff', 19: '#cbcbcb',
  20: '#222222', 21: '#0c5787', 22: '#ffffff', 23: '#0b6d98',
  24: '#e66a02', 25: '#ffffff', 30: '#09acd3', 31: '#0b4778',
  44: '#0c4b7b', 45: '#ffffff', 46: '#0c4b7b', 47: '#222222',
  48: '#222222', 49: '#ffffff', 50: '#fdb87b', 51: '#f37905',
  52: '#9b9ca8', 53: '#62626e', 61: '#3653e0', 62: '#0991b8',
  63: '#0c4b7b', 64: '#ffffff', 76: '#e66a02', 77: '#ffffff',
  78: '#222222', 79: '#0a5589', 87: '#e66a02', 88: '#793300',
  93: '#075d8d', 94: '#fd8011', 95: '#d15800', 96: '#793300',
  97: '#f2feff', 98: '#b8f2fb', 99: '#7ed3e0', 100: '#ebebeb',
  101: '#b75300', 120: '#222222', 123: '#ffffff', 124: '#0b4778',
  125: '#56b3e6', 138: '#9f9f9f', 139: '#ffffff', 142: '#086834',
  144: '#406ac0', 146: '#fcc504', 148: '#993300', 149: '#ff0000',
  150: '#ff7300',
} as const;

const field = v4ThemeFields;
export const v4 = {
  body: field[123], text: field[47], label: field[48], border: field[124],
  headerTop: field[62], headerBottom: field[63], headerText: field[64],
  activeTabTop: field[5], activeTabBottom: field[6], tabBorder: field[14],
  inactiveTabTop: field[8], inactiveTabBottom: field[9], tabText: field[17],
  unreadTop: field[11], unreadBottom: field[12],
  stripTop: field[3], stripBottom: field[4],
  buttonTop: field[18], buttonBottom: field[19], buttonText: field[20],
  menuHeader: field[21], menuBody: field[23], menuSelected: field[24],
  menuText: field[25], menuDisabled: field[30], menuBorder: field[31],
  dialogBorder: field[44], dialogBody: field[45], dialogTitle: field[46],
  focusTop: field[50], focus: field[51], bevelLight: field[52], bevelDark: field[53],
  link: field[61], selected: field[76], selectedText: field[77],
  rowText: field[78], secondaryText: field[79],
  groupTop: field[97], groupBottom: field[98], groupBorder: field[99], groupText: field[93],
  selfTop: field[94], selfBottom: field[95], subtle: field[100], muted: field[138],
  transcript: field[120],
} as const;

export const v4Metrics = {
  font: 15,
  smallFont: 13,
  lineHeight: 21,
  headerHeight: 28,
  commandHeight: 44,
  touchTarget: 44,
  tabHeight: 44,
  icon: 20,
  avatar: 40,
  pageInset: 6,
} as const;

/** Native React Native gradient, keeping the original top-to-bottom stops. */
export function v4Gradient(top: string, bottom: string): string {
  return `linear-gradient(to bottom, ${top}, ${bottom})`;
}
