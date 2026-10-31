import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { v4ThemeFields } from '../src/ui/v4/theme.ts';
import { v3Colors } from '../src/ui/v3/theme.ts';
import { modernAppearanceTheme, modernControlColors, parseAppearanceMode } from '../src/ui/appearanceTheme.ts';

test('Default V2 UI colors match the independently extracted original theme fields', () => {
  const source: { opcode: number; fields: { tag: number; int32?: number; length: number }[] } = JSON.parse(
    readFileSync(new URL('../../../research/applets/v4/docs/data/theme.json', import.meta.url), 'utf8'));
  assert.equal(source.opcode, 1000);
  const fields = new Map(source.fields.map((field) => [field.tag, field]));
  for (const [id, value] of Object.entries(v4ThemeFields)) {
    const field = fields.get(Number(id));
    assert.equal(field?.length, 4, `Theme ${id} must be an original int32 field`);
    assert.equal(Number.parseInt(value.slice(1), 16), field?.int32, `Default V2 color ${id}`);
  }
});

test('v3 renderer colors have independent source locations in the recovered 3.08 inventory', () => {
  const source: Record<string, string[]> = JSON.parse(readFileSync(
    new URL('../../../research/applets/v3/docs/data/colors.json', import.meta.url), 'utf8'));
  const bytecode: { colors: Record<string, string>; transcriptColors: string[] } = JSON.parse(readFileSync(
    new URL('../../../research/applets/v3/docs/data/theme-bytecode-evidence.json', import.meta.url), 'utf8'));
  for (const [role, values] of Object.entries(v3Colors)) {
    // Gradients and the category switch aren't in the direct paint-call index.
    if (role === 'transcript') { assert.deepEqual(values, bytecode.transcriptColors); continue; }
    for (const value of typeof values === 'string' ? [values] : values) {
      assert.ok(source[value]?.length || bytecode.colors[role] === value,
        `${role} ${value} must occur in the extracted original renderer`);
    }
  }
});

test('appearance preferences preserve existing v4 selection and accept v3 across upgrades', () => {
  assert.equal(parseAppearanceMode('classic'), 'classic');
  assert.equal(parseAppearanceMode('classic-v3'), 'classic-v3');
  assert.equal(parseAppearanceMode('modern'), 'modern');
  for (const unknown of [null, '', 'v3', 'future-theme']) {
    assert.equal(parseAppearanceMode(unknown), 'modern');
  }
});

function channels(hex: string): number[] {
  return [1, 3, 5].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16));
}
function contrast(first: string | number[], second: string | number[]): number {
  const luminance = (value: string | number[]) => {
    const rgb = (typeof value === 'string' ? channels(value) : value).map((channel) => {
      const x = channel / 255;
      return x <= .04045 ? x / 12.92 : ((x + .055) / 1.055) ** 2.4;
    });
    return .2126 * rgb[0] + .7152 * rgb[1] + .0722 * rgb[2];
  };
  const a = luminance(first), b = luminance(second);
  return (Math.max(a, b) + .05) / (Math.min(a, b) + .05);
}

test('Modern placeholders, unchecked controls and focus survive light/dark and Increase Contrast', () => {
  for (const dark of [false, true]) for (const increased of [false, true]) {
    const theme = modernAppearanceTheme(dark, increased);
    for (const surface of [theme.body, theme.dialogBody]) {
      for (const [role, color] of Object.entries({ placeholder: theme.muted, text: theme.text, label: theme.label })) {
        assert.ok(contrast(color, surface) >= 4.5, `${role} must meet4.5:1 on ${surface}; dark=${dark}, increased=${increased}`);
      }
      for (const [role, color] of Object.entries({ checkbox: theme.bevelLight, checkboxLowerEdge: theme.bevelDark, focus: theme.focus, focusTop: theme.focusTop })) {
        assert.ok(contrast(color, surface) >= 3, `${role} must meet3:1 on ${surface}; dark=${dark}, increased=${increased}`);
      }
    }
  }
});

test('Modern selected glass tabs retain translucent fill with readable ink and a visible outline', () => {
  for (const dark of [false, true]) for (const increased of [false, true]) {
    const theme = modernAppearanceTheme(dark, increased), controls = modernControlColors(dark, increased);
    const rgba = controls.selectedFill.match(/[\d.]+/g)!.map(Number);
    assert.ok(rgba[3] > 0 && rgba[3] <= .15, 'Selected tabs must retain the native glass surface');
    for (const surface of [theme.body, theme.dialogBody]) {
      const blended = channels(surface).map((value, index) => rgba[index] * rgba[3] + value * (1 - rgba[3]));
      assert.ok(contrast(controls.selectedInk, blended) >= 4.5, `Selected text contrast: dark=${dark}, increased=${increased}`);
      assert.ok(contrast(controls.selectedInk, surface) >= 4.5, 'Glass action text must remain readable on the solid fallback');
      assert.ok(contrast(controls.selectedOutline, surface) >= 3, `Selected outline contrast: dark=${dark}, increased=${increased}`);
    }
  }
});
