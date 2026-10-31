import React, { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { ActivityIndicator, Image, Keyboard, Modal, Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions, type TextStyle } from 'react-native';
import { legacyText, type LegacyControl, type LegacyForm, type LegacyNode, type LegacyValues } from '../../mig33/legacy-pages';
import { useAppearance } from '../appearance';
import { V4Button, V4Checkbox, V4ChoiceGroup, V4Field, V4Link, V4Menu } from '../v4/primitives';
import { supportsNativePresentation } from '../NativePresentation';
import type { LegacyBrowser } from './useLegacyPages';
import { pageBaseColors, pageNodeColors, type PageColors } from './pageColors';

const inlineTags = new Set(['b', 'strong', 'i', 'em', 'u', 'span', 'font', 'big', 'small', 'a', 'br', '#text']);
function color(value: string | undefined): string | undefined {
  if (!value) return undefined;
  return /^(#[\da-f]{3}|#[\da-f]{6}|#[\da-f]{8}|black|white|red|green|blue|yellow|orange|brown|purple|pink|gray|grey|silver|navy|teal|aqua|lime|maroon|olive|fuchsia|transparent)$/i.test(value.trim()) ? value.trim() : undefined;
}
function textStyle(node: LegacyNode): TextStyle {
  const style: { -readonly [Key in keyof TextStyle]: TextStyle[Key] } = {};
  if (['b', 'strong', 'em'].includes(node.tag)) style.fontWeight = '700';
  if (node.tag === 'i') style.fontStyle = 'italic';
  if (node.tag === 'u') style.textDecorationLine = 'underline';
  if (node.tag === 'big') style.fontSize = 22;
  if (node.tag === 'small') style.fontSize = 12;
  if (node.tag === 'center') style.textAlign = 'center';
  if (color(node.attributes.color)) style.color = color(node.attributes.color);
  for (const declaration of (node.attributes.style ?? '').split(';')) {
    const [key, value] = declaration.split(':').map((part) => part.trim());
    if (key === 'color' && color(value)) style.color = color(value);
    if (key === 'background-color' && color(value)) style.backgroundColor = color(value);
    if (key === 'font-weight' && value === 'bold') style.fontWeight = '700';
    if (key === 'font-style' && value === 'italic') style.fontStyle = 'italic';
    if (key === 'text-decoration' && value === 'underline') style.textDecorationLine = 'underline';
    if (key === 'text-align' && ['left', 'right', 'center'].includes(value)) style.textAlign = value as 'left' | 'right' | 'center';
  }
  return style;
}

type ViewPhoto = (uri: string, label?: string) => void;

function PageImage({ node, browser, colors, onViewPhoto }: {
  node: LegacyNode; browser: LegacyBrowser; colors?: PageColors; onViewPhoto?: ViewPhoto;
}) {
  const { theme, modern, reduceMotion } = useAppearance();
  const [uri, setURI] = useState<string>();
  const [failed, setFailed] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [fullURI, setFullURI] = useState<string>();
  const [fullError, setFullError] = useState(false);
  const viewPhoto = useRef(onViewPhoto);
  viewPhoto.current = onViewPhoto;
  const currentPage = useRef({ id: browser.page?.id, loading: browser.loading });
  currentPage.current = { id: browser.page?.id, loading: browser.loading };
  const sharedViewer = modern && supportsNativePresentation && !!onViewPhoto;
  const viewport = useWindowDimensions();
  useEffect(() => {
    let active = true; setURI(undefined); setFailed(false);
    try { void browser.loadImage(node.attributes.src).then((value) => { if (active) setURI(value); })
      .catch(() => { if (active) setFailed(true); }); } catch { setFailed(true); }
    return () => { active = false; };
  }, [browser.loadImage, node.attributes.src]);
  useEffect(() => {
    if (!expanded) return;
    let active = true; setFullURI(undefined); setFullError(false);
    const pageID = browser.page?.id;
    // The Go image route serves originals when its thumbnail dimensions are
    // absent. It still checks the current session through922 purpose2.
    try {
      const target = new URL(node.attributes.src, browser.page?.url);
      target.searchParams.delete('w'); target.searchParams.delete('h');
      void browser.loadImage(target.href).then((value) => {
        if (!active) return;
        setFullURI(value);
        if (sharedViewer) {
          setExpanded(false);
          if (currentPage.current.id === pageID && !currentPage.current.loading) {
            viewPhoto.current?.(value, node.attributes.alt || 'Photo');
          }
        }
      }).catch(() => {
        if (!active) return;
        setFullError(true);
        if (sharedViewer) setExpanded(false);
      });
    } catch {
      setFullError(true);
      if (sharedViewer) setExpanded(false);
    }
    return () => { active = false; };
  }, [expanded, browser.loadImage, browser.page?.id, browser.page?.url, node.attributes.src, node.attributes.alt, sharedViewer]);
  const width = Math.min(320, Math.max(32, Number(node.attributes.width) || 96));
  const height = Math.min(400, Math.max(32, Number(node.attributes.height) || 96));
  let photo = false;
  try { photo = /^\/assets\/photos\/\d+$/.test(new URL(node.attributes.src, browser.page?.url).pathname); } catch { /* Display the failed-image state. */ }
  return <View style={{ minHeight: height, marginVertical: 6, alignItems: 'flex-start', backgroundColor: colors?.localBackground }}>
    {uri ? <Pressable disabled={!photo || browser.loading || sharedViewer && expanded}
      accessibilityState={{ disabled: browser.loading || sharedViewer && expanded, busy: sharedViewer && expanded }}
      accessibilityRole={photo ? 'button' : undefined} accessibilityLabel={node.attributes.alt || (photo ? 'View full photo' : 'Page image')}
      testID={photo ? `page.photo.${node.id}` : undefined} onPress={() => { Keyboard.dismiss(); setExpanded(true); }}>
      <Image source={{ uri }} style={{ width, height }} resizeMode="contain" />
    </Pressable>
      : failed ? <Text style={{ color: colors?.foreground ?? theme.secondaryText }}>{node.attributes.alt || 'Image unavailable'}</Text>
        : <ActivityIndicator color={colors?.link ?? theme.link} style={{ width, height }} />}
    {sharedViewer && expanded && <ActivityIndicator accessibilityLabel="Loading full photo" color={colors?.link ?? theme.link} />}
    {sharedViewer && fullError && <Text accessibilityRole="alert" style={{ color: colors?.foreground ?? theme.text }}>Photo unavailable. Tap the photo to try again.</Text>}
    <Modal visible={expanded && !sharedViewer} animationType={modern ? reduceMotion ? 'none' : 'slide' : 'fade'} transparent onRequestClose={() => setExpanded(false)}>
      <View testID="page.photo.viewer" accessibilityViewIsModal style={{ flex: 1, backgroundColor: '#080b10f5', justifyContent: 'center', alignItems: 'center', padding: 24 }}>
        {fullURI ? <Image accessibilityLabel={node.attributes.alt || 'Full photo'} source={{ uri: fullURI }} resizeMode="contain"
          style={{ width: Math.max(32, viewport.width - 48), height: Math.max(32, viewport.height - 180) }} />
          : fullError ? <Text accessibilityRole="alert" style={{ color: '#ffffff' }}>Photo unavailable.</Text> : <ActivityIndicator color="#ffffff" />}
        <V4Button label="Close Photo" testID="page.photo.close" onPress={() => setExpanded(false)} style={{ marginTop: 20 }} />
      </View>
    </Modal>
  </View>;
}

function SelectControl({ control, value, disabled, onChange }: {
  control: LegacyControl; value: string; disabled: boolean; onChange: (value: string) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const { theme, metrics, modern } = useAppearance();
  const selected = control.options?.find((option) => option.value === value);
  const options = (control.options ?? []).map((option, index) => ({ ...option, testID: `page.option.${control.name}.${index}` }));
  // Picker requires a matching selection tag. Retain unknown server values
  // visibly without changing what an untouched form sends back.
  if (!selected) options.unshift({ value, label: value || 'Choose an option', disabled: true, testID: `page.option.${control.name}.current` });
  if (modern) return <V4ChoiceGroup testID={`page.field.${control.name}`} label={control.label} hideLabel variant="menu"
    value={value} disabled={disabled} style={{ marginBottom: 14 }}
    options={options}
    onChange={(next) => { if (!disabled) { Keyboard.dismiss(); onChange(next); } }} />;
  return <>
    <Pressable testID={`page.field.${control.name}`} accessibilityRole="button"
      accessibilityLabel={`${control.label}: ${selected?.label ?? value}`} accessibilityHint="Choose an option"
      disabled={disabled} onPress={() => { Keyboard.dismiss(); setExpanded(true); }}
      style={{ minHeight: 48, padding: 12, borderWidth: 1, borderColor: theme.border, backgroundColor: theme.dialogBody,
        flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
      <Text style={{ flex: 1, color: theme.text, fontSize: metrics.font }}>{selected?.label ?? value}</Text>
      <Text style={{ color: theme.link }}>▾</Text>
    </Pressable>
    <V4Menu visible={expanded} title={control.label} onClose={() => setExpanded(false)} items={(control.options ?? []).map((option, index) => ({
      id: String(index), label: option.label, disabled: option.disabled, testID: `page.option.${control.name}.${index}`,
      onPress: () => { onChange(option.value); setExpanded(false); },
    }))} />
  </>;
}

/** Renders server-authored content as native themed controls. All network and
 * mutations remain in922/923; arbitrary HTML never runs in a WebView.
 */
export function LegacyPageScreen({ browser, onViewPhoto }: { browser: LegacyBrowser; onViewPhoto?: ViewPhoto }) {
  const { theme, metrics, modern, navigationInset } = useAppearance();
  const page = browser.page;
  const currentPage = useRef({ page, loading: browser.loading });
  currentPage.current = { page, loading: browser.loading };
  const [drafts, setDrafts] = useState<Record<number, Record<number, string | boolean>>>({});
  useEffect(() => { setDrafts({}); }, [browser.open]);
  useEffect(() => { if (!page) setDrafts({}); }, [page]);
  const values: LegacyValues = page ? drafts[page.id] ?? {} : {};
  const baseColors = pageBaseColors(theme);
  const bodyColors = page ? pageNodeColors(page.body.attributes, baseColors, theme, { body: true }) : baseColors;
  const styles = useMemo(() => StyleSheet.create({
    screen: { flex: 1, backgroundColor: theme.body },
    content: { paddingTop: modern ? navigationInset + 18 : 8, paddingHorizontal: modern ? metrics.pageInset : 8, paddingBottom: 28 },
    title: { fontSize: modern ? 30 : metrics.font + 3, fontWeight: '700', color: theme.text, marginBottom: 18 },
    text: { fontSize: metrics.font, lineHeight: modern ? 24 : metrics.font + 6, color: theme.text },
    paragraph: { marginBottom: modern ? 14 : 8 },
    form: { marginVertical: 10, padding: modern ? 16 : 5, borderRadius: modern ? 20 : 0,
      backgroundColor: modern ? theme.dialogBody : theme.body, borderWidth: StyleSheet.hairlineWidth, borderColor: theme.groupBorder },
    link: { color: theme.link, textDecorationLine: 'underline' },
    button: { alignSelf: 'flex-start', marginVertical: 8 },
    progress: { padding: 14, flexDirection: 'row', gap: 12, alignItems: 'center' },
    error: { padding: 12, color: theme.text, fontSize: metrics.font },
    rule: { height: StyleSheet.hairlineWidth, backgroundColor: theme.groupBorder, marginVertical: 12 },
  }), [theme, metrics, modern, navigationInset]);
  function change(control: LegacyControl, value: string | boolean, form: LegacyForm) {
    if (!page || currentPage.current.loading || currentPage.current.page?.id !== page.id) return;
    setDrafts((previous) => {
      const next = { ...previous[page.id], [control.id]: value };
      if (control.kind === 'radio' && value === true) for (const other of form.controls) {
        if (other.id !== control.id && other.kind === 'radio' && other.name === control.name) next[other.id] = false;
      }
      // Match the browser's bounded history rather than retaining old form
      // contents for the entire lifetime of a session.
      const retained = Object.fromEntries(Object.entries(previous).sort(([left], [right]) => Number(right) - Number(left)).slice(0, 30));
      return { ...retained, [page.id]: next };
    });
  }
  function renderControl(control: LegacyControl, form: LegacyForm): ReactNode {
    const disabled = control.disabled || browser.loading;
    if (control.kind === 'hidden') return null;
    if (control.kind === 'submit') return <V4Button key={control.id} testID={`page.submit.${form.id}.${control.id}`}
      label={control.value || 'Continue'} disabled={disabled} style={styles.button}
      onPress={() => { Keyboard.dismiss(); void browser.submit(form, values, control.id); }} />;
    if (control.kind === 'file') return <V4Button key={control.id} label="Choose Photo" variant="secondary" disabled={disabled}
      onPress={() => { void browser.follow('mig33:uploadPhotoToScrapbook()'); }} />;
    if (modern && control.kind === 'radio') {
      const radios = form.controls.filter((entry) => entry.kind === 'radio'
        && (control.name ? entry.name === control.name : entry.id === control.id));
      if (control.id !== radios[0]?.id) return null;
      // A radio group is one exclusive native choice, never independent
      // switches. Choosing its selected option cannot clear the selection.
      const selected = radios.findLast((entry) => Boolean(values[entry.id] ?? entry.checked));
      const options = radios.map((entry) => ({ value: String(entry.id),
        label: entry.label === entry.name ? entry.value || entry.label : entry.label,
        disabled: entry.disabled, testID: `page.option.${entry.name}.${entry.id}` }));
      if (!selected) options.unshift({ value: '', label: 'Choose an option', disabled: true, testID: `page.option.${control.name}.empty` });
      return <V4ChoiceGroup key={control.id} testID={`page.field.${control.name}`} label={control.name || control.label}
        hideLabel variant="menu" value={selected ? String(selected.id) : ''} disabled={browser.loading}
        style={{ marginBottom: 14 }} options={options}
        onChange={(id) => {
          const next = radios.find((entry) => String(entry.id) === id);
          if (next && !next.disabled) change(next, true, form);
        }} />;
    }
    if (['checkbox', 'radio'].includes(control.kind)) {
      const checkbox = <V4Checkbox key={control.id} testID={`page.field.${control.name}`}
        label={control.label} checked={Boolean(values[control.id] ?? control.checked)} disabled={disabled}
        onChange={(checked) => change(control, checked, form)} />;
      // Native controls retain their semantic theme surface even inside an
      // authored white/dark form. Their labels must not inherit a page color.
      return modern ? <View key={control.id} style={{ backgroundColor: theme.dialogBody }}>{checkbox}</View> : checkbox;
    }
    if (control.kind === 'select') return <SelectControl key={control.id} control={control}
      value={String(values[control.id] ?? control.value)} disabled={disabled} onChange={(value) => change(control, value, form)} />;
    if (['button', 'reset'].includes(control.kind)) return null;
    return <V4Field key={control.id} testID={`page.field.${control.name}`} accessibilityLabel={control.label}
      value={String(values[control.id] ?? control.value)} editable={!disabled}
      onChangeText={(value) => change(control, value, form)} multiline={control.kind === 'textarea'}
      secureTextEntry={control.kind === 'password'} autoCapitalize="none" autoCorrect={control.kind === 'textarea'}
      maxLength={control.maxLength} keyboardType={control.kind === 'number' ? 'numeric' : control.kind === 'email' ? 'email-address' : 'default'}
      style={control.kind === 'textarea' ? { minHeight: 100, textAlignVertical: 'top' } : undefined} />;
  }
  function paintedText(node: LegacyNode, colors: PageColors): TextStyle {
    return modern ? { ...textStyle(node), color: colors.foreground, backgroundColor: colors.localBackground } : textStyle(node);
  }
  function inline(node: LegacyNode, parent: PageColors): ReactNode {
    if (node.tag === '#text') return node.text?.replace(/\s+/g, ' ');
    if (node.tag === 'br') return '\n';
    const colors = modern ? pageNodeColors(node.attributes, parent, theme, { link: node.tag === 'a' }) : parent;
    if (node.tag === 'a') return <Text key={node.id} accessibilityRole="link" accessibilityState={{ disabled: browser.loading }}
      style={[styles.link, modern && paintedText(node, colors)]}
      onPress={browser.loading ? undefined : () => { Keyboard.dismiss(); void browser.follow(node.attributes.href ?? ''); }}>{node.children.map((child) => inline(child, colors))}</Text>;
    return <Text key={node.id} style={paintedText(node, colors)}>{node.children.map((child) => inline(child, colors))}</Text>;
  }
  function render(node: LegacyNode, form?: LegacyForm, parent = baseColors): ReactNode {
    const colors = modern ? pageNodeColors(node.attributes, parent, theme, {
      link: node.tag === 'a', body: node === page?.body, surface: node.tag === 'form' ? theme.dialogBody : undefined,
    }) : parent;
    if (node.tag === '#text') return node.text?.trim() ? <Text key={node.id} style={[styles.text, modern && { color: colors.foreground }]}>{node.text.replace(/\s+/g, ' ')}</Text> : null;
    if (node.tag === 'form') {
      const owner = page?.forms.find((entry) => entry.id === node.id);
      return owner ? <View key={node.id} testID={`page.form.${node.id}`} style={[styles.form, modern && { backgroundColor: colors.background }]}>
        {node.children.map((child) => render(child, owner, colors))}</View> : null;
    }
    if (['input', 'textarea', 'select', 'button'].includes(node.tag)) {
      const control = form?.controls.find((entry) => entry.id === node.id);
      return control && form ? renderControl(control, form) : null;
    }
    if (node.tag === 'img') return node.attributes.src ? <PageImage key={node.id} node={node} browser={browser}
      colors={modern ? colors : undefined} onViewPhoto={onViewPhoto} /> : null;
    if (node.tag === 'hr') return <View key={node.id} style={styles.rule} />;
    if (node.tag === 'a' && node.attributes.href && node.children.some((child) => child.tag === 'img')) {
      return <Pressable key={node.id} accessibilityRole="link" accessibilityLabel={legacyText(node) || 'Open image'}
        accessibilityState={{ disabled: browser.loading }} disabled={browser.loading}
        style={modern && { backgroundColor: colors.localBackground }}
        onPress={() => { void browser.follow(node.attributes.href); }}>{node.children.map((child) => render(child, form, colors))}</Pressable>;
    }
    if (modern && node.tag === 'a' && node.attributes.href) return <Pressable key={node.id} testID={`page.link.${node.id}`}
      accessibilityRole="link" accessibilityLabel={legacyText(node).replace(/\s+/g, ' ').trim()}
      accessibilityState={{ disabled: browser.loading }} disabled={browser.loading}
      style={{ minHeight: metrics.touchTarget, justifyContent: 'center', alignSelf: 'flex-start' }}
      onPress={() => { Keyboard.dismiss(); void browser.follow(node.attributes.href); }}>
      <Text style={[styles.text, styles.link, paintedText(node, colors)]}>{node.children.map((child) => inline(child, colors))}</Text>
    </Pressable>;
    if (node.tag === 'a' && node.attributes.href) return <V4Link key={node.id} testID={`page.link.${node.id}`}
      label={legacyText(node).replace(/\s+/g, ' ').trim()} disabled={browser.loading}
      onPress={() => { Keyboard.dismiss(); void browser.follow(node.attributes.href); }} />;
    const children = node.children.filter((child) => child.tag !== '#text' || child.text?.trim());
    if (node.tag === 'p' && children.length === 1 && children[0].tag === 'a') return <View key={node.id}
      style={[styles.paragraph, modern && { backgroundColor: colors.localBackground }]}>{render(children[0], form, colors)}</View>;
    if ((['p', 'center', 'h1', 'h2', 'h3', 'li', 'pre'].includes(node.tag) || inlineTags.has(node.tag))
      && !hasBlockContent(node)) {
      return <Text key={node.id} style={[styles.text, styles.paragraph, paintedText(node, colors), /^h[1-3]$/.test(node.tag) && { fontWeight: '700' }]}>
        {node.children.map((child) => inline(child, colors))}</Text>;
    }
    return <View key={node.id} style={modern && { backgroundColor: colors.localBackground }}>{node.children.map((child) => render(child, form, colors))}</View>;
  }
  return <ScrollView testID="page.screen" style={styles.screen} contentContainerStyle={styles.content}
    keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag">
    {browser.error !== '' && <Text testID="page.error" accessibilityRole="alert" style={styles.error}>{browser.error}</Text>}
    {browser.error !== '' && <V4Button label="Reload Page" variant="secondary" onPress={() => { void browser.refresh(); }} disabled={browser.loading} />}
    {browser.loading && <View style={styles.progress}><ActivityIndicator color={theme.link} /><Text style={styles.text}>Loading…</Text></View>}
    {page && <View key={page.id} style={{ backgroundColor: modern ? bodyColors.background : color(page.body.attributes.bgcolor) }}>
      {modern && <Text accessibilityRole="header" style={[styles.title, { color: bodyColors.foreground }]}>{page.title}</Text>}
      {render(page.body)}
    </View>}
  </ScrollView>;
}

function hasBlockContent(node: LegacyNode): boolean {
  return node.children.some((child) => ['img', 'input', 'form', 'select', 'textarea'].includes(child.tag) || hasBlockContent(child));
}
