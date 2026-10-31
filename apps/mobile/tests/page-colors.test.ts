import assert from 'node:assert/strict';
import test from 'node:test';
import { modernAppearanceTheme } from '../src/ui/appearanceTheme.ts';
import { pageBaseColors, pageColor, pageNodeColors } from '../src/ui/pages/pageColors.ts';
import { parseLegacyPage } from '../src/mig33/legacy-pages.ts';

const dark = modernAppearanceTheme(true), light = modernAppearanceTheme(false);

test('Modern page text corrects explicit black on the default dark background', () => {
  const root = pageBaseColors(dark);
  const text = pageNodeColors({ color: 'black' }, root, dark);
  assert.equal(text.background, dark.body);
  assert.equal(text.foreground, dark.text);
  assert.equal(text.authoredForeground, '#000000');
  assert.equal(pageNodeColors({ style: 'color:#fff' }, root, dark).foreground, '#ffffff');
});

test('a white body supplies readable default text, headings and links even in Dark appearance', () => {
  const body = pageNodeColors({ bgcolor: 'WHITE' }, pageBaseColors(dark), dark, { body: true });
  assert.equal(body.background, '#ffffff');
  assert.equal(body.foreground, '#162b3b');
  assert.equal(body.link, '#075a71');
  assert.equal(pageNodeColors({}, body, dark).foreground, '#162b3b');
  assert.equal(pageNodeColors({}, body, dark, { link: true }).foreground, '#075a71');
});

test('authored readable colors survive and inherited colors are rechecked for nested backgrounds', () => {
  const body = pageNodeColors({ bgcolor: 'white', text: 'navy' }, pageBaseColors(dark), dark, { body: true });
  assert.equal(body.foreground, '#000080');
  const darkInline = pageNodeColors({ style: 'background-color: black' }, body, dark);
  assert.equal(darkInline.foreground, dark.text);
  assert.equal(darkInline.authoredForeground, '#000080');
  const whiteAgain = pageNodeColors({ style: 'background-color: white' }, darkInline, dark);
  assert.equal(whiteAgain.foreground, '#000080', 'The original inherited color is readable again');
  assert.equal(pageNodeColors({ style: 'color:green' }, body, dark, { link: true }).foreground, '#008000');
  assert.equal(pageNodeColors({ style: 'color:pink' }, pageBaseColors(light), light).foreground, light.text);
});

test('transparent and alpha colors compose against the effective parent rather than assuming white', () => {
  const black = pageNodeColors({ bgcolor: 'black' }, pageBaseColors(dark), dark, { body: true });
  const overlay = pageNodeColors({ style: 'background-color:rgba(255,255,255,0.5);color:white' }, black, dark);
  assert.equal(overlay.background, '#808080');
  assert.equal(overlay.foreground, '#000000', 'Pale text must not disappear into its translucent light background');
  const transparent = pageNodeColors({ style: 'background-color:transparent;color:transparent' }, overlay, dark);
  assert.equal(transparent.background, overlay.background);
  assert.equal(transparent.foreground, '#000000');
  assert.equal(pageNodeColors({ style: 'color:#ffffffcc' }, black, dark).foreground, '#ffffffcc');
  assert.equal(pageNodeColors({ style: 'color:#ffffff11' }, black, dark).foreground, dark.text);
});

test('native form surfaces re-evaluate inherited page text and nested links retain link defaults', () => {
  const body = pageNodeColors({ bgcolor: 'white', text: 'black' }, pageBaseColors(dark), dark, { body: true });
  const form = pageNodeColors({}, body, dark, { surface: dark.dialogBody });
  assert.equal(form.background, dark.dialogBody);
  assert.equal(form.foreground, dark.text);
  const link = pageNodeColors({}, body, dark, { link: true });
  assert.equal(link.foreground, '#075a71');
  assert.equal(pageNodeColors({}, link, dark).foreground, link.foreground);
  const darkLinkSpan = pageNodeColors({ style: 'background-color:black' }, link, dark);
  assert.equal(darkLinkSpan.foreground, dark.link);
});

test('the bounded parser handles names, short/long alpha hex and numeric CSS colors safely', () => {
  assert.equal(pageColor(' TeAl '), '#008080');
  assert.equal(pageColor('#abc'), '#aabbcc');
  assert.equal(pageColor('#1234'), '#11223344');
  assert.equal(pageColor('rgb(50% 0% 100%)'), '#8000ff');
  assert.equal(pageColor('rgba(255, 0, 0, 50%)'), '#ff000080');
  assert.equal(pageColor('rgb(300 -5 1 / 0.5)'), '#ff000180');
  for (const value of ['url(https://example.com)', 'var(--color)', '#12345', 'rgb(NaN,0,0)', 'rgb(0,0)', 'not-a-color']) {
    assert.equal(pageColor(value), undefined);
  }
});

test('Modern resolution does not rewrite the original parsed page or wire-authored values', () => {
  const page = parseLegacyPage('<body bgcolor="white"><p style="color: black">Hello <span style="background-color: black;color:white">world</span></p></body>',
    'http://localhost/sites/index.php?c=profile&a=home');
  const original = JSON.stringify(page);
  const body = pageNodeColors(page.body.attributes, pageBaseColors(dark), dark, { body: true });
  const paragraph = pageNodeColors(page.body.children[0].attributes, body, dark);
  pageNodeColors(page.body.children[0].children[1].attributes, paragraph, dark);
  assert.equal(JSON.stringify(page), original);
});
