import React, { forwardRef, useCallback, useEffect, useLayoutEffect, useRef, useState, type ComponentRef, type ReactNode } from 'react';
import {
  ActivityIndicator, Animated, Easing, Modal, Platform, Pressable, SafeAreaView, ScrollView, StyleSheet,
  Text, TextInput, View, useWindowDimensions,
  type LayoutChangeEvent, type NativeScrollEvent, type NativeSyntheticEvent,
  type StyleProp, type TextInputProps, type TextStyle, type ViewStyle,
} from 'react-native';

import { v4, v4Gradient, v4Metrics, v4ThemeFields } from './theme';
import { useAppearance, type AppearanceMetrics, type AppearanceTheme } from '../appearance';
import { avatarAppearance, modernControlColors, modernDarkHighContrastTheme, modernDarkTheme,
  modernHighContrastTheme, modernMetrics, modernTheme } from '../appearanceTheme';
import { GlassSurface } from '../glass';
import { ModernButton, ModernChoice, ModernSymbol, ModernToggle, supportsModernControls } from '../ModernControls';
import { MotionEntrance, MotionPressable } from '../motion';
import { v3Colors, v3Metrics, v3Theme } from '../v3/theme';

export type V4IconName = 'friends' | 'chat' | 'conversation' | 'page' | 'mail' | 'person'
  | 'online' | 'away' | 'busy' | 'offline' | 'arrowRight' | 'arrowDown'
  | 'plus' | 'minus' | 'close' | 'check' | 'menu' | 'search';

const modernSymbols: Record<V4IconName, string> = {
  friends: 'person.2.fill', person: 'person.fill', chat: 'bubble.left.and.bubble.right',
  conversation: 'bubble.left', page: 'doc.text', mail: 'envelope', online: 'circle.fill',
  away: 'clock.fill', busy: 'minus.circle.fill', offline: 'circle.fill',
  arrowRight: 'chevron.right', arrowDown: 'chevron.down', plus: 'plus', minus: 'minus',
  close: 'xmark', check: 'checkmark', menu: 'ellipsis', search: 'magnifyingglass',
};

/** Small native shape redraws of the original sprite roles; no bitmap chrome. */
export function V4Icon({ name, size: requestedSize, color: requestedColor }: {
  name: V4IconName; size?: number; color?: string;
}) {
  const { theme, metrics, modern, v3, dark } = useAppearance();
  const size = requestedSize ?? metrics.icon;
  const color = requestedColor ?? theme.secondaryText;
  if (modern && supportsModernControls) {
    const presenceColor = name === 'online' ? dark ? '#30d158' : '#248a3d'
      : name === 'away' ? dark ? '#ffd60a' : '#956d00'
        : name === 'busy' ? dark ? '#ff6961' : '#c0352b' : color;
    return <ModernSymbol name={modernSymbols[name]} size={size} color={requestedColor ?? presenceColor} />;
  }
  const line = Math.max(1, size / 12);
  let drawing: ReactNode;
  if (name === 'friends' || name === 'person') {
    drawing = <>
      {name === 'friends' && <>
        <View style={{ position: 'absolute', width: size * .25, height: size * .25, borderRadius: size,
          left: size * .05, top: size * .12, backgroundColor: color, opacity: .72 }} />
        <View style={{ position: 'absolute', width: size * .44, height: size * .37,
          borderTopLeftRadius: size * .2, borderTopRightRadius: size * .2,
          left: 0, top: size * .43, backgroundColor: color, opacity: .72 }} />
      </>}
      <View style={{ position: 'absolute', width: size * .31, height: size * .31, borderRadius: size,
        left: size * (name === 'friends' ? .48 : .35), top: size * .08, backgroundColor: color }} />
      <View style={{ position: 'absolute', width: size * .56, height: size * .42,
        borderTopLeftRadius: size * .28, borderTopRightRadius: size * .28,
        left: size * (name === 'friends' ? .36 : .22), top: size * .48, backgroundColor: color }} />
    </>;
  } else if (name === 'chat' || name === 'conversation') {
    drawing = <>
      <View style={{ position: 'absolute', left: size * .08, top: size * .1,
        width: size * .82, height: size * .59, borderRadius: size * .14,
        borderWidth: line, borderColor: color, backgroundColor: name === 'chat' && !modern ? (v3 ? '#fefec3' : '#d4fb55') : 'transparent',
        // v3 j.png cell40 uses pale yellow, not the v4 chat sprite's lime.
        backgroundImage: v3 && name === 'chat' ? v4Gradient('#ffff89', '#fefec3') : undefined }} />
      <View style={{ position: 'absolute', left: size * .22, top: size * .62,
        width: size * .22, height: size * .22, borderLeftWidth: line, borderBottomWidth: line,
        borderColor: color, transform: [{ skewY: '-35deg' }] }} />
      {v3 && name === 'chat' && [.3, .53].map((left) => <View key={left} style={{ position: 'absolute',
        left: size * left, top: size * .27, width: size * .12, height: size * .16,
        borderTopWidth: line, borderRightWidth: line, borderColor: '#000000' }} />)}
    </>;
  } else if (name === 'page') {
    drawing = <>
      <View style={{ position: 'absolute', left: size * .15, top: size * .04, width: size * .7,
        height: size * .88, borderWidth: line, borderColor: color, backgroundColor: modern ? 'transparent' : '#fffbd3' }} />
      {[.28, .47, .66].map((top) => <View key={top} style={{ position: 'absolute', left: size * .28,
        top: size * top, width: size * .42, height: line, backgroundColor: color }} />)}
    </>;
  } else if (name === 'mail') {
    drawing = <>
      <View style={{ position: 'absolute', left: size * .06, top: size * .2, width: size * .88,
        height: size * .62, borderWidth: line, borderColor: color, backgroundColor: modern ? 'transparent' : '#ffffff' }} />
      <View style={{ position: 'absolute', left: size * .25, top: size * .08, width: size * .48,
        height: size * .48, borderBottomWidth: line, borderRightWidth: line,
        borderColor: color, transform: [{ rotate: '45deg' }] }} />
    </>;
  } else if (['online', 'away', 'busy', 'offline'].includes(name)) {
    const fill = name === 'online' ? '#7dca11' : name === 'away' ? '#efcf06' : name === 'busy' ? '#e54729' : '#b7b7b7';
    // v3 j.png cells8–11 are green figures, with a clock/no-entry badge;
    // offline is the red figure. These are not v4's colored presence circles.
    drawing = v3 ? <>
      <View style={{ position: 'absolute', width: size * .62, height: size * .36, left: size * .24, bottom: 0,
        borderRadius: size * .25, borderWidth: 1, borderColor: name === 'offline' ? '#d41413' : '#137936',
        backgroundColor: name === 'offline' ? '#e33939' : '#38ab4b' }} />
      <View style={{ position: 'absolute', width: size * .75, height: size * .65, left: size * .16, top: 0,
        borderRadius: size * .4, borderWidth: size * .16, borderColor: name === 'offline' ? '#e33939' : '#38ab4b',
        backgroundColor: '#ffffff' }} />
      {(name === 'away' || name === 'busy') && <View style={{ position: 'absolute', left: 0, bottom: 0,
        width: size * .65, height: size * .65, borderRadius: size, borderWidth: 1,
        borderColor: name === 'busy' ? '#d41413' : '#000000', backgroundColor: name === 'busy' ? '#fb7171' : '#ffffff' }}>
        {name === 'busy' ? <View style={{ position: 'absolute', left: 2, right: 2, top: size * .24,
          height: 2, backgroundColor: '#ffffff' }} /> : <>
          <View style={{ position: 'absolute', width: 1, height: size * .21, left: size * .27, top: size * .08, backgroundColor: '#000000' }} />
          <View style={{ position: 'absolute', width: size * .2, height: 1, left: size * .27, top: size * .27, backgroundColor: '#000000' }} />
        </>}
      </View>}
    </> : <View style={{ width: size * .68, height: size * .68, borderRadius: size,
      borderWidth: line, borderColor: name === 'offline' ? '#888888' : '#496a13',
      backgroundColor: fill, backgroundImage: modern ? undefined : v4Gradient('#ffffff', fill) }} />;
  } else if (name === 'arrowRight' || name === 'arrowDown') {
    drawing = <View style={{ width: 0, height: 0, borderStyle: 'solid',
      borderLeftWidth: name === 'arrowRight' ? size * .4 : size * .27,
      borderRightWidth: name === 'arrowRight' ? 0 : size * .27,
      borderTopWidth: name === 'arrowRight' ? size * .27 : size * .4,
      borderBottomWidth: name === 'arrowRight' ? size * .27 : 0,
      borderLeftColor: name === 'arrowRight' ? color : 'transparent', borderRightColor: 'transparent',
      borderTopColor: name === 'arrowRight' ? 'transparent' : color, borderBottomColor: 'transparent' }} />;
  } else if (name === 'menu') {
    // Use one stable native layout box for the navigation strokes. Avoid
    // absolute child offsets when the adjacent scrollable tabs change width.
    drawing = <View collapsable={false} style={{ width: size * .68, height: size * .46 + line,
      justifyContent: 'space-between' }}>
      {[0, 1, 2].map((index) => <View key={index} style={{ height: line, width: '100%',
        borderRadius: line, backgroundColor: color }} />)}
    </View>;
  } else if (v3 && (name === 'plus' || name === 'minus')) {
    // j.png cells4/5: square beveled group control with a black plus/minus.
    drawing = <View style={{ width: size * .88, height: size * .88, borderRadius: 1,
      borderWidth: 1, borderTopColor: '#9196a3', borderLeftColor: '#9196a3',
      borderRightColor: '#575862', borderBottomColor: '#575862',
      backgroundImage: v4Gradient('#fefefe', '#c4cedb'), alignItems: 'center', justifyContent: 'center' }}>
      <View style={{ width: size * .5, height: Math.max(1, size / 8), backgroundColor: '#000000' }} />
      {name === 'plus' && <View style={{ position: 'absolute', width: Math.max(1, size / 8),
        height: size * .5, backgroundColor: '#000000' }} />}
    </View>;
  } else if (name === 'check') {
    drawing = <View style={{ width: size * .35, height: size * .64, borderBottomWidth: line * 1.4,
      borderRightWidth: line * 1.4, borderColor: color, transform: [{ rotate: '42deg' }], marginTop: -size * .16 }} />;
  } else {
    drawing = <>
      <View style={{ position: 'absolute', left: size * .18, top: size * .46, width: size * .64,
        height: line, backgroundColor: color, transform: [{ rotate: name === 'close' ? '45deg' : '0deg' }] }} />
      {name !== 'minus' && <View style={{ position: 'absolute', left: size * .46, top: size * .18,
        width: line, height: size * .64, backgroundColor: color,
        transform: [{ rotate: name === 'close' ? '45deg' : '0deg' }] }} />}
    </>;
  }
  return <View accessible={false} pointerEvents="none" style={{ width: size, height: size,
    alignItems: 'center', justifyContent: 'center' }}>{drawing}</View>;
}

/** The welcome logo's two speech bubbles, redrawn with native shapes and text. */
export function V4Logo({ width: requestedWidth }: { width?: number }) {
  const { modern, v3 } = useAppearance();
  const width = requestedWidth ?? (v3 ? 118 : modern ? 136 : 112);
  if (v3) {
    const scale = width / 118;
    // k.png is118x62. Its two dominant nonwhite colors are #c3edf8/#cdf0c8;
    // native rounded text approximates its letterforms without retaining PNGs.
    const fontFamily = Platform.OS === 'ios' ? 'ArialRoundedMTBold' : 'sans-serif';
    return <View accessibilityLabel="mig33" accessibilityRole="image" style={{ width, height: 62 * scale }}>
      <Text allowFontScaling={false} style={{ position: 'absolute', left: 0, bottom: 0,
        fontFamily, fontSize: 52 * scale, lineHeight: 62 * scale, fontWeight: '700',
        letterSpacing: -4 * scale, color: '#c3edf8' }}>mig</Text>
      <Text allowFontScaling={false} style={{ position: 'absolute', right: 0, top: -3 * scale,
        fontFamily, fontSize: 36 * scale, lineHeight: 44 * scale, fontWeight: '700',
        letterSpacing: -3 * scale, color: '#cdf0c8' }}>33</Text>
    </View>;
  }
  const scale = width / 112;
  return <View accessibilityLabel="mig33" accessibilityRole="image" style={{ width, height: 66 * scale }}>
    <View style={{ position: 'absolute', left: 0, top: 4 * scale, width: 91 * scale, height: 59 * scale,
      borderRadius: 32 * scale, backgroundColor: '#00a8da' }}>
      <Text allowFontScaling={false} style={{ color: '#ffffff', fontSize: 43 * scale, lineHeight: 56 * scale,
        fontWeight: '700', letterSpacing: -3 * scale, textAlign: 'center',
        fontFamily: Platform.OS === 'ios' ? 'ArialRoundedMTBold' : 'sans-serif' }}>mig</Text>
    </View>
    <View style={{ position: 'absolute', left: 20 * scale, top: 52 * scale, width: 19 * scale, height: 12 * scale,
      backgroundColor: '#00a8da', transform: [{ skewX: '-25deg' }] }} />
    <View style={{ position: 'absolute', right: 0, top: 0, width: 39 * scale, height: 31 * scale,
      borderRadius: 18 * scale, backgroundColor: '#ff6a21', borderWidth: 2 * scale, borderColor: '#ffffff' }}>
      <Text allowFontScaling={false} style={{ color: '#ffffff', fontSize: 23 * scale, lineHeight: 27 * scale,
        fontWeight: '700', letterSpacing: -2 * scale, textAlign: 'center',
        fontFamily: Platform.OS === 'ios' ? 'ArialRoundedMTBold' : 'sans-serif' }}>33</Text>
    </View>
  </View>;
}

export function V4Avatar({ presence, size: requestedSize, label = '' }: { presence?: number; size?: number; label?: string }) {
  const { modern, dark, metrics } = useAppearance();
  const size = requestedSize ?? metrics.avatar;
  const status: V4IconName = presence === 1 ? 'online' : presence === 4 ? 'away' : presence === 3 ? 'busy' : 'offline';
  if (modern) {
    const avatar = avatarAppearance(label, dark);
    return <View accessible={false} style={{ width: size, height: size, borderRadius: size * .36,
      backgroundColor: avatar.background, alignItems: 'center', justifyContent: 'center' }}>
      {avatar.initials ? <Text allowFontScaling={false} style={{ color: avatar.foreground,
        fontSize: size * .36, fontWeight: '600', letterSpacing: -.4 }}>{avatar.initials}</Text>
        : <V4Icon name="person" size={size * .5} color={avatar.foreground} />}
      {presence !== undefined && <View style={{ position: 'absolute', right: -2, bottom: -2 }}>
        <V4Icon name={status} size={14} />
      </View>}
    </View>;
  }
  // Native redraw of the tilted grey smiling tile in the original default
  // avatar/theme132. It intentionally uses geometry rather than the PNG.
  return <View accessible={false} style={{ width: size, height: size, backgroundColor: '#f2f6f6',
    borderWidth: 1, borderColor: '#ccd6d9', borderRadius: modern ? size * .28 : 0,
    alignItems: 'center', justifyContent: 'center' }}>
    <View style={{ width: size * .79, height: size * .78, borderRadius: size * (modern ? .23 : .04),
      backgroundColor: '#cbd6da', transform: [{ rotate: modern ? '0deg' : '-10deg' }] }}>
      <View style={{ position: 'absolute', left: size * .16, top: size * .17,
        width: size * .09, height: size * .1, borderRadius: size * .05, backgroundColor: '#6f848d' }} />
      <View style={{ position: 'absolute', right: size * .15, top: size * .17,
        width: size * .09, height: size * .1, borderRadius: size * .05, backgroundColor: '#6f848d' }} />
      <View style={{ position: 'absolute', left: size * .17, top: size * .39,
        width: size * .45, height: size * .2, borderBottomLeftRadius: size * .24,
        borderBottomRightRadius: size * .24, backgroundColor: '#ffffff' }} />
    </View>
    {presence !== undefined && <View style={{ position: 'absolute', right: -3, bottom: -3 }}>
      <V4Icon name={status} size={14} />
    </View>}
  </View>;
}

export function V4Header({ title, children }: { title: string; children?: ReactNode }) {
  const { styles } = usePrimitiveAppearance();
  return <View style={styles.header}>
    <Text accessibilityRole="header" numberOfLines={1} style={styles.headerTitle}>{title}</Text>
    {children}
  </View>;
}

export interface V4ButtonProps {
  label: string; onPress: () => void; disabled?: boolean; busy?: boolean;
  style?: StyleProp<ViewStyle>; accessibilityLabel?: string; testID?: string;
  variant?: 'primary' | 'secondary' | 'plain';
  systemImage?: string; iconOnly?: boolean; selected?: boolean;
}
export function V4Button({ label, onPress, disabled = false, busy = false, style, accessibilityLabel, testID,
  variant = 'primary', systemImage, iconOnly, selected }: V4ButtonProps) {
  const { styles, theme: v4, modern, v3 } = usePrimitiveAppearance();
  const [focused, setFocused] = useState(false);
  if (modern && supportsModernControls) return <ModernButton {...{ label, onPress, disabled, busy, style, accessibilityLabel, testID,
    variant, systemImage, iconOnly, selected }} />;
  const refreshIcon = modern && iconOnly && systemImage === 'arrow.clockwise';
  return <MotionPressable testID={testID} accessibilityRole="button" accessibilityLabel={accessibilityLabel ?? label}
    accessibilityState={{ disabled: disabled || busy, busy }} disabled={disabled || busy} onPress={onPress}
    onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
    style={[styles.buttonTarget, refreshIcon && { padding: 0, alignItems: 'center' }, style]}>
    {({ pressed }) => <View style={[styles.button, refreshIcon && { width: 44, minWidth: 44, minHeight: 44, padding: 0, borderRadius: 22 },
      !v3 && (pressed || focused) && styles.focused, disabled && styles.disabled]}>
      {busy ? <ActivityIndicator size="small" color={v4.buttonText} /> :
        <Text style={[styles.buttonText, refreshIcon && { fontSize: 24, lineHeight: 28 }]}>{refreshIcon ? '↻' : label}</Text>}
      {v3 && (pressed || focused) && <V3FocusTrim />}
    </View>}
  </MotionPressable>;
}

/** Compatibility name; every Modern action now uses the same native control. */
export function GlassActionButton(props: V4ButtonProps) { return <V4Button {...props} />; }

export type V4FieldRef = ComponentRef<typeof TextInput>;
export const V4Field = forwardRef<V4FieldRef, TextInputProps & {
  label?: string; error?: boolean; containerStyle?: StyleProp<ViewStyle>;
  variant?: 'form' | 'search' | 'composer';
}>(function V4Field({ label, error, containerStyle, style, onFocus, onBlur, editable = true, variant = 'form', ...props }, ref) {
  const { styles, theme: v4, modern, v3, increaseContrast } = usePrimitiveAppearance();
  const [focused, setFocused] = useState(false);
  if (modern) {
    const input = <TextInput ref={ref} {...props} editable={editable} accessibilityLabel={props.accessibilityLabel ?? label}
      selectionColor={v4.link} placeholderTextColor={v4.muted}
      onFocus={(event) => { setFocused(true); onFocus?.(event); }}
      onBlur={(event) => { setFocused(false); onBlur?.(event); }}
      style={[styles.modernFieldInput, !editable && { color: v4.muted }, style,
        { borderWidth: 0, backgroundColor: 'transparent' }]} />;
    return <View style={[styles.fieldGroup, variant === 'composer' && { marginBottom: 0 }, containerStyle]}>
      {label && <Text style={styles.label}>{label}</Text>}
      {variant === 'search' ? <GlassSurface style={styles.searchField}>
        <View collapsable={false} style={styles.searchFieldContent}>
          <V4Icon name="search" size={18} /><View style={{ flex: 1 }}>{input}</View>
        </View>
      </GlassSurface> : variant === 'composer' ? input : <View style={[styles.modernFormField,
        { borderColor: error ? v4.focus : focused ? v4.link : v4.bevelLight,
          borderWidth: error || focused || increaseContrast ? 1 : StyleSheet.hairlineWidth }]}>{input}</View>}
    </View>;
  }
  return <View style={[styles.fieldGroup, v3 && label && { paddingRight: 20 }, containerStyle]}>
    {label && <Text style={styles.label}>{label}</Text>}
    <View style={[styles.fieldOutline, !v3 && (focused || error) && styles.fieldFocused, !editable && styles.disabled]}>
      <TextInput ref={ref} {...props} editable={editable} accessibilityLabel={props.accessibilityLabel ?? label}
        selectionColor={v4.focus} placeholderTextColor={v4.muted}
        onFocus={(event) => { setFocused(true); onFocus?.(event); }}
        onBlur={(event) => { setFocused(false); onBlur?.(event); }}
        style={[styles.field, style]} />
      {v3 && (focused || error) && <V3FocusTrim />}
    </View>
  </View>;
});

export function V4Checkbox({ label, checked, onChange, disabled = false, testID }: {
  label: string; checked: boolean; onChange?: (value: boolean) => void; disabled?: boolean; testID?: string;
}) {
  const { styles, theme, modern, v3 } = usePrimitiveAppearance();
  const inactive = disabled || !onChange;
  if (modern && supportsModernControls) return <ModernToggle {...{ label, checked, onChange, disabled: inactive, testID }} />;
  return <MotionPressable testID={testID} accessibilityRole="checkbox" accessibilityLabel={label}
    accessibilityState={{ checked, disabled: inactive }} disabled={inactive}
    onPress={() => onChange?.(!checked)} style={({ pressed }) => [styles.checkRow, !v3 && pressed && styles.rowFocus]}>
    {({ pressed }) => <>
      <View style={[styles.checkBox, modern && checked && { backgroundColor: theme.buttonBottom, borderColor: theme.buttonBottom }, inactive && styles.disabled]}>
        {checked && <V4Icon name="check" size={v3 ? 12 : 15} color={modern ? theme.buttonText : v3 ? '#000000' : '#24a71b'} />}
        {v3 && pressed && <V3FocusTrim />}
      </View>
      <Text style={[styles.text, inactive && styles.disabledText]}>{label}</Text>
    </>}
  </MotionPressable>;
}

export function V4ChoiceGroup({ label, value, options, onChange, disabled, variant = 'menu', testID, style, hideLabel, accessibilityLabel }: {
  label?: string; value: string; options: readonly { value: string; label: string; testID?: string; disabled?: boolean }[];
  onChange: (value: string) => void; disabled?: boolean; variant?: 'menu' | 'segmented'; testID?: string; style?: StyleProp<ViewStyle>;
  hideLabel?: boolean; accessibilityLabel?: string;
}) {
  const { modern } = useAppearance();
  if (modern && supportsModernControls) return <ModernChoice {...{ label, value, options, onChange, disabled, variant, testID, style, hideLabel, accessibilityLabel }} />;
  return <View style={style}>{options.map((option) => <V4Checkbox key={option.value} label={option.label}
    testID={option.testID} checked={option.value === value} disabled={disabled || option.disabled}
    onChange={() => onChange(option.value)} />)}</View>;
}

export function V4Link({ label, onPress, disabled = false, style, testID }: {
  label: string; onPress?: () => void; disabled?: boolean; style?: StyleProp<TextStyle>; testID?: string;
}) {
  const { styles } = usePrimitiveAppearance();
  return <MotionPressable testID={testID} accessibilityRole="link" accessibilityState={{ disabled: disabled || !onPress }}
    disabled={disabled || !onPress} onPress={onPress}
    style={({ pressed }) => [styles.linkTarget, pressed && styles.rowFocus]}>
    <Text style={[styles.link, (disabled || !onPress) && styles.disabledText, style]}>{label}</Text>
  </MotionPressable>;
}

export interface V4Command { label: string; onPress: () => void; disabled?: boolean; testID?: string; }
export function V4CommandBar({ left, right }: { left?: V4Command; right?: V4Command }) {
  const { styles, modern } = usePrimitiveAppearance();
  if (modern) return <View style={styles.commandDock}>
    {[left, right].map((command, index) => command ?
      <V4Button key={index} {...command} variant={command.label === 'Send' ? 'primary' : 'secondary'}
        systemImage={command.label === 'Back' ? 'chevron.left' : undefined} /> : <View key={index} style={{ flex: 1 }} />)}
  </View>;
  return <View style={styles.commandBar}>
    {[left, right].map((command, index) => command ?
      <Pressable key={index} testID={command.testID} accessibilityRole="button" disabled={command.disabled}
        accessibilityState={{ disabled: command.disabled }} onPress={command.onPress}
        style={({ pressed }) => [styles.command, index === 1 && styles.commandRight, pressed && styles.rowFocus]}>
        <Text style={[styles.commandText, command.disabled && styles.disabledText]}>{command.label}</Text>
      </Pressable> : <View key={index} style={styles.command} />)}
  </View>;
}

export interface V4Tab { id: string; label: string; icon: V4IconName; unread?: boolean; }

/** aj.run/c.run share a750ms phase across every inactive unread tab. Modern
 * softens only the local plate transition; no native glass host is animated.
 */
function useUnreadPhase(unread: boolean) {
  const { modern, reduceMotion } = useAppearance();
  const [on, setOn] = useState(true);
  const opacity = useRef(new Animated.Value(1)).current;
  useEffect(() => {
    setOn(true);
    if (!unread || reduceMotion) return;
    const timer = setInterval(() => setOn((value) => !value), 750);
    return () => clearInterval(timer);
  }, [unread, reduceMotion]);
  useEffect(() => {
    opacity.stopAnimation();
    if (!modern || reduceMotion || !unread) { opacity.setValue(1); return; }
    Animated.timing(opacity, { toValue: on ? 1 : 0, duration: 140,
      easing: Easing.inOut(Easing.quad), useNativeDriver: true }).start();
    return () => opacity.stopAnimation();
  }, [on, modern, reduceMotion, unread, opacity]);
  return { on: reduceMotion || on, name: reduceMotion ? 'steady' : on ? 'on' : 'off', opacity };
}

/** A non-accessible sibling of the tab's accessibility element. Native tests
 * can synchronize captures without exposing animation phases to VoiceOver. */
function UnreadPhaseMarker({ id, phase }: { id: string; phase: string }) {
  return <View collapsable={false} accessible={false} pointerEvents="none"
    testID={`tab.unread-${phase}.${id}`} style={StyleSheet.absoluteFill} />;
}

/** Measure the actual tab and viewport: labels, Dynamic Type, theme changes and
 * new conversations all change their widths. Coalesce layout events before
 * revealing the selection; ordinary user scrolling does not snap back.
 */
function useSelectedTabReveal(selected: string) {
  const { reduceMotion } = useAppearance();
  const ref = useRef<ComponentRef<typeof ScrollView>>(null);
  const layouts = useRef(new Map<string, { x: number; width: number }>());
  const viewport = useRef(0);
  const contentWidth = useRef(0);
  const offset = useRef(0);
  const frame = useRef<number | null>(null);
  const attach = useCallback((instance: ComponentRef<typeof ScrollView> | null) => {
    if (ref.current === instance) return;
    ref.current = instance;
    // Switching between the v3 strip and glass creates a new native viewport.
    // Its offset starts at zero even when this hook retained the same selection.
    offset.current = 0; viewport.current = 0; contentWidth.current = 0;
    layouts.current.clear();
  }, []);
  const reveal = useCallback(() => {
    if (frame.current !== null) cancelAnimationFrame(frame.current);
    frame.current = requestAnimationFrame(() => {
      frame.current = null;
      const tab = layouts.current.get(selected);
      const width = viewport.current;
      if (!ref.current || !tab || width <= 0 || contentWidth.current <= 0) return;
      const left = Math.max(0, tab.x - 5);
      const right = tab.x + tab.width + 5;
      let x = offset.current;
      if (tab.width + 10 >= width || left < x) x = left;
      else if (right > x + width) x = right - width;
      x = Math.max(0, Math.min(x, contentWidth.current - width));
      if (Math.abs(x - offset.current) < 1) return;
      offset.current = x;
      ref.current.scrollTo({ x, animated: !reduceMotion });
    });
  }, [selected, reduceMotion]);
  useLayoutEffect(() => { reveal(); }, [reveal]);
  useEffect(() => () => { if (frame.current !== null) cancelAnimationFrame(frame.current); }, []);
  return {
    scrollProps: {
      ref: attach,
      onLayout: (event: LayoutChangeEvent) => { viewport.current = event.nativeEvent.layout.width; reveal(); },
      onContentSizeChange: (width: number) => { contentWidth.current = width; reveal(); },
      onScroll: (event: NativeSyntheticEvent<NativeScrollEvent>) => { offset.current = event.nativeEvent.contentOffset.x; },
      scrollEventThrottle: 16,
    },
    onTabLayout: (id: string) => (event: LayoutChangeEvent) => {
      const { x, width } = event.nativeEvent.layout;
      layouts.current.set(id, { x, width });
      reveal();
    },
  };
}

export function V4Tabs({ tabs, selected, onSelect, onMenu, menuControl, animateIn = false }: {
  tabs: readonly V4Tab[]; selected: string; onSelect: (id: string) => void; onMenu?: () => void;
  menuControl?: ReactNode; animateIn?: boolean;
}) {
  const { styles, theme: v4, modern, v3, dark, increaseContrast, reduceTransparency } = usePrimitiveAppearance();
  const viewport = useWindowDimensions();
  const colors = modernControlColors(dark, increaseContrast);
  const reveal = useSelectedTabReveal(selected);
  const pulse = useUnreadPhase(tabs.some((tab) => tab.id !== selected && tab.unread));
  if (v3) return <V3Tabs tabs={tabs} selected={selected} onSelect={onSelect} pulse={pulse} />;
  if (modern && supportsModernControls) {
    const destinations = tabs.filter((tab) => !/^(peer|room|group):/.test(tab.id));
    const conversations = tabs.filter((tab) => /^(peer|room|group):/.test(tab.id));
    const tabButton = (tab: V4Tab, compact = false) => {
      const active = tab.id === selected;
      const unread = !!tab.unread && !active;
      const glowing = unread && pulse.on;
      return <View key={tab.id} onLayout={compact ? undefined : reveal.onTabLayout(tab.id)}>
        <ModernButton testID={`tab.${tab.id}`} label={tab.label} systemImage={modernSymbols[tab.icon]}
          accessibilityLabel={`${tab.label}${unread ? ', unread messages' : ''}`} onPress={() => onSelect(tab.id)}
          variant="secondary" selected={active} highlighted={unread} iconOnly={compact}
          // Keep the native style stable throughout the unread pulse. Changing
          // glass/glassProminent mid-gesture can replace the pressed button.
          tintColor={glowing ? '#f5a34e' : active || unread ? reduceTransparency ? v4.selected : colors.glassTint : v4.text}
          foregroundColor={glowing ? '#38200d' : active ? reduceTransparency ? v4.selectedText : colors.selectedInk : v4.text}
          style={compact ? { width: 46, height: 48 } : { maxWidth: 188, minHeight: 48 }} />
        {unread && <UnreadPhaseMarker id={tab.id} phase={pulse.name} />}
      </View>;
    };
    return <View style={styles.nativeNavigation}>
      <View style={styles.nativeDestinations}>{destinations.map((tab) => tabButton(tab,
        conversations.length > 0 || destinations.length > 2 || viewport.width < 390 || viewport.fontScale > 1.15))}</View>
      {conversations.length > 0 && <ScrollView testID="tabs.items" {...reveal.scrollProps} horizontal
        style={{ flex: 1, minWidth: 0 }} showsHorizontalScrollIndicator indicatorStyle={dark ? 'white' : 'black'}
        contentContainerStyle={styles.nativeConversations}>{conversations.map((tab) => tabButton(tab))}</ScrollView>}
      {conversations.length === 0 && <View style={{ flex: 1 }} />}
      {menuControl ?? (onMenu && <ModernButton label="Open menu" testID="command.menu" systemImage="ellipsis"
        iconOnly variant="secondary" onPress={onMenu} style={{ width: 46, height: 48 }} />)}
    </View>;
  }
  const tabsContent = <ScrollView testID="tabs.items" {...reveal.scrollProps} horizontal style={modern ? { flex: 1 } : undefined}
    showsHorizontalScrollIndicator={false} contentContainerStyle={styles.tabs}>
      {tabs.map((tab) => {
        const active = tab.id === selected;
        const unread = !!tab.unread && !active;
        return <View key={tab.id} onLayout={reveal.onTabLayout(tab.id)}>
        <MotionPressable testID={`tab.${tab.id}`} accessibilityRole="tab" accessibilityLabel={`${tab.label}${unread ? ', unread messages' : ''}`}
          accessibilityState={{ selected: active }} onPress={() => onSelect(tab.id)}
          style={[styles.tab, active && styles.activeTab, !modern && unread && pulse.on && styles.unreadTab]}>
          {!modern && unread && pulse.on && <View accessible={false} pointerEvents="none" style={{
            position: 'absolute', top: 0, left: 0, right: 0, height: 1, backgroundColor: v4ThemeFields[13] }} />}
          <V4Icon name={tab.icon} color={modern ? active ? colors.selectedInk : v4.text : v4.tabText} size={20} />
          {(active || modern) && <Text numberOfLines={1} style={[styles.tabTitle,
            modern && { color: active ? colors.selectedInk : v4.text, fontWeight: active ? '700' : '500' }]}>{tab.label}</Text>}
          {modern && unread && <Animated.View accessible={false} pointerEvents="none"
            style={[StyleSheet.absoluteFill, styles.modernUnreadPlate, { opacity: pulse.opacity }]}>
            <V4Icon name={tab.icon} color="#38200d" size={20} />
            <Text numberOfLines={1} style={[styles.tabTitle, { color: '#38200d' }]}>{tab.label}</Text>
          </Animated.View>}
        </MotionPressable>
        {unread && <UnreadPhaseMarker id={tab.id} phase={pulse.name} />}
        </View>;
      })}
    </ScrollView>;
  return modern ? <View style={styles.tabDock}><GlassSurface style={styles.glassTabs} animateIn={animateIn}>
    {/* Expo's glass host mounts into UIVisualEffectView.contentView. Keep one
        native child there while React updates the scrollable tabs and menu. */}
    <View collapsable={false} style={styles.glassTabsContent}>
      {tabsContent}
    {/* Keep this persistent glass child free of a native animated transform.
        With dynamic tabs, that layer can retain a blank icon until pressed. */}
    {menuControl !== undefined ? <View style={styles.navigationMenu}>{menuControl}</View> : onMenu && <Pressable accessibilityRole="button" accessibilityLabel="Open menu"
      testID="command.menu" onPress={onMenu}
      style={({ pressed }) => [styles.navigationMenu, pressed && { backgroundColor: v4.selected }]}>
        <V4Icon name="menu" size={21} color={v4.text} />
    </Pressable>}
    </View>
  </GlassSurface></View>
    : <View style={styles.tabStrip}>{tabsContent}</View>;
}

/** d.java:111–125: gold focus strokes around the existing control interior. */
function V3FocusTrim() {
  return <View accessible={false} pointerEvents="none" style={[StyleSheet.absoluteFill,
    { borderWidth: 1, borderColor: v3Colors.focus[2] }]}>
    <View style={{ position: 'absolute', left: 1, right: 1, top: 0, height: 1, backgroundColor: v3Colors.focus[0] }} />
    <View style={{ position: 'absolute', left: 1, right: 1, top: 1, height: 1, backgroundColor: v3Colors.focus[1] }} />
    <View style={{ position: 'absolute', left: 1, right: 1, bottom: 1, height: 1, backgroundColor: v3Colors.focus[3] }} />
    <View style={{ position: 'absolute', left: 1, right: 1, bottom: 0, height: 1, backgroundColor: v3Colors.focus[4] }} />
  </View>;
}

function interpolateColor(top: string, bottom: string, fraction: number): string {
  const channels = [1, 3, 5].map((offset) => Math.trunc(
    parseInt(top.slice(offset, offset + 2), 16) * (1 - fraction)
    + parseInt(bottom.slice(offset, offset + 2), 16) * fraction,
  ).toString(16).padStart(2, '0'));
  return `#${channels.join('')}`;
}

/** ab.java:346–374: a45-degree left edge, scanline fill and white bevel. */
function V3TabPlate({ active, unread }: { active: boolean; unread: boolean }) {
  const height = v3Metrics.tabHeight;
  const top = unread ? v3Colors.unreadTop : active ? v3Colors.activeTabTop : v3Colors.inactiveTabTop;
  const bottom = unread ? v3Colors.unreadBottom : active ? v3Colors.activeTabBottom : v3Colors.inactiveTabBottom;
  return <View accessible={false} pointerEvents="none" style={{ position: 'absolute', left: 0, right: 0, bottom: 0, height }}>
    {Array.from({ length: height }, (_, row) => <View key={row} style={{ position: 'absolute',
      left: height - row - 1, right: 0, top: row, height: 1,
      backgroundColor: row === 0 ? v3Colors.tabBorder : row === 1 ? v3Colors.white :
        row === height - 1 && active ? v3Colors.surround : interpolateColor(top, bottom, row / (height - 1)),
      borderLeftWidth: 1, borderRightWidth: 1, borderColor: v3Colors.tabBorder }}>
      {row > 1 && row < height - 1 && <View style={{ position: 'absolute', left: 0, top: 0, bottom: 0,
        width: 1, backgroundColor: v3Colors.white }} />}
    </View>)}
  </View>;
}

function V3Tabs({ tabs, selected, onSelect, pulse }: {
  tabs: readonly V4Tab[]; selected: string; onSelect: (id: string) => void; pulse: ReturnType<typeof useUnreadPhase>;
}) {
  const { width } = useWindowDimensions();
  const reveal = useSelectedTabReveal(selected);
  const tabWidth = Math.max(120, Math.floor((width - 12) / 2));
  return <View style={v3Styles.tabStrip}>
    <ScrollView testID="tabs.items" {...reveal.scrollProps} horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={v3Styles.tabContents}>
      {tabs.map((tab, index) => <View key={tab.id} onLayout={reveal.onTabLayout(tab.id)}
        style={{ width: tabWidth, height: 44, marginLeft: index === 0 ? 0 : -v3Metrics.tabHeight / 2,
          zIndex: tab.id === selected ? 2 : 1 }}>
        <Pressable testID={`tab.${tab.id}`} accessibilityRole="tab"
        accessibilityLabel={`${tab.id === 'friends' ? 'Friends' : tab.label}${tab.unread && tab.id !== selected ? ', unread messages' : ''}`}
        accessibilityState={{ selected: tab.id === selected }} onPress={() => onSelect(tab.id)}
        style={{ flex: 1, justifyContent: 'center', paddingLeft: v3Metrics.tabHeight + 4, paddingRight: 10 }}>
        <V3TabPlate active={tab.id === selected} unread={!!tab.unread && tab.id !== selected && pulse.on} />
        <Text numberOfLines={1} style={{ color: '#000000', fontSize: 15, lineHeight: 20,
          textAlign: 'center', fontWeight: tab.id === selected ? '700' : '400' }}>
          {tab.id === 'friends' ? 'Friends' : tab.label}
        </Text>
      </Pressable>
      {!!tab.unread && tab.id !== selected && <UnreadPhaseMarker id={tab.id} phase={pulse.name} />}
      </View>)}
    </ScrollView>
  </View>;
}

const v3Styles = StyleSheet.create({
  selectedMenuText: { backgroundColor: v3Colors.selected, borderWidth: 1,
    borderColor: v3Colors.selectionOutline, paddingHorizontal: 2 },
  // ab.a(Graphics), original bytecode offsets20–46: (197,218,247) to
  // (243,248,254) across the entire strip, before individual tabs are drawn.
  tabStrip: { backgroundColor: v3Colors.stripBottom, backgroundImage: v4Gradient(v3Colors.stripTop, v3Colors.stripBottom),
    borderBottomWidth: 2, borderColor: v3Colors.surround },
  tabContents: { paddingHorizontal: 8, minHeight: 44, alignItems: 'flex-end' },
});

export interface V4MenuItem {
  id: string; label: string; onPress?: () => void; icon?: V4IconName; disabled?: boolean; separator?: boolean; testID?: string;
  children?: readonly V4MenuItem[]; destructive?: boolean;
}
export function V4Menu({ visible, title = 'Menu', items, onClose, embedded = false }: {
  visible: boolean; title?: string; items: readonly V4MenuItem[]; onClose: () => void; embedded?: boolean;
}) {
  const { styles, theme: v4, modern, v3 } = usePrimitiveAppearance();
  const [commandsHeight, setCommandsHeight] = useState(56);
  if (embedded && !visible) return null;
  const availableItems = modern ? items.filter((item) => !item.disabled && item.onPress) : items;
  const selected = availableItems.find((item) => !item.disabled && item.onPress)?.id;
  const content = <View style={[styles.overlay, modern && { paddingBottom: commandsHeight + 12 }]}>
      <Pressable accessibilityRole="button" accessibilityLabel="Close menu" onPress={onClose} style={StyleSheet.absoluteFill} />
      <MotionEntrance accessibilityViewIsModal style={styles.menu} transitionKey={`${visible}:${title}`}>
        {modern && <View accessible={false} style={styles.sheetHandle} />}
        <Text accessibilityRole="header" style={styles.menuTitle}>{title}</Text>
        <ScrollView testID="menu.items" style={styles.menuItems}>
          {availableItems.map((item) => <MotionPressable key={item.id} testID={item.testID} accessibilityRole="menuitem"
            accessibilityState={{ disabled: item.disabled || !item.onPress }} disabled={item.disabled || !item.onPress}
            onPress={() => { onClose(); item.onPress?.(); }}
            style={({ pressed }) => [styles.menuItem, item.separator && styles.menuSeparator,
              !v3 && (pressed || (!modern && item.id === selected)) && styles.menuPressed]}>
            {({ pressed }) => <>
              {item.icon && !v3 && <V4Icon name={item.icon} size={18} color={item.disabled ? v4.menuDisabled : v4.menuText} />}
              <Text style={[styles.menuText, (item.disabled || !item.onPress) && styles.menuDisabled,
                v3 && (pressed || item.id === selected) && v3Styles.selectedMenuText]}>{item.label}</Text>
            </>}
          </MotionPressable>)}
        </ScrollView>
      </MotionEntrance>
      {modern ? <SafeAreaView style={styles.modernModalCommands}
        onLayout={(event) => setCommandsHeight(event.nativeEvent.layout.height)}>
        <V4CommandBar right={{ label: 'Cancel', onPress: onClose }} />
      </SafeAreaView> : <View style={styles.modalCommands}><V4CommandBar right={{ label: 'Cancel', onPress: onClose }} /></View>}
    </View>;
  return embedded ? content :
    <Modal visible={visible} transparent animationType="none" onRequestClose={onClose}>{content}</Modal>;
}

export function V4Dialog({ visible, title, message, onClose, actions, embedded = false }: {
  visible: boolean; title: string; message: string; onClose: () => void; actions?: readonly V4Command[]; embedded?: boolean;
}) {
  const { styles, v3 } = usePrimitiveAppearance();
  if (embedded && !visible) return null;
  const content = <View style={styles.dialogOverlay}>
      <MotionEntrance accessibilityViewIsModal style={styles.dialog} transitionKey={`${visible}:${title}`}>
        <Text accessibilityRole="header" style={[styles.dialogTitle,
          v3 && title === 'Error' && { backgroundColor: v3Colors.errorHeader }]}>{title}</Text>
        <ScrollView style={styles.dialogMessageScroll}>
          <Text style={styles.dialogMessage}>{message}</Text>
        </ScrollView>
        <View style={styles.dialogActions}>
          {(actions ?? [{ label: 'Ok', onPress: onClose }]).map((action) => <V4Button key={action.label} {...action} />)}
        </View>
      </MotionEntrance>
    </View>;
  return embedded ? content :
    <Modal visible={visible} transparent animationType="none" onRequestClose={onClose}>{content}</Modal>;
}

function createStyles(v4: AppearanceTheme, v4Metrics: AppearanceMetrics, modern: boolean, dark = false, increaseContrast = false) {
const controlColors = modernControlColors(dark, increaseContrast);
const styles = StyleSheet.create({
  text: { color: v4.text, fontSize: v4Metrics.font, lineHeight: v4Metrics.lineHeight, flexShrink: 1 },
  header: { minHeight: v4Metrics.headerHeight, justifyContent: 'center', backgroundColor: v4.headerBottom,
    backgroundImage: v4Gradient(v4.headerTop, v4.headerBottom), paddingHorizontal: 6, paddingVertical: 3 },
  headerTitle: { color: v4.headerText, fontSize: v4Metrics.font, lineHeight: 21, textAlign: 'center' },
  buttonTarget: { minHeight: v4Metrics.touchTarget, justifyContent: 'center', alignSelf: 'flex-start', paddingHorizontal: 2 },
  button: { minHeight: 28, minWidth: 50, paddingHorizontal: 9, paddingVertical: 3, justifyContent: 'center', alignItems: 'center',
    borderRadius: 2, borderWidth: 1, borderTopColor: v4.bevelLight, borderLeftColor: v4.bevelLight,
    borderRightColor: v4.bevelDark, borderBottomColor: v4.bevelDark,
    backgroundColor: v4.buttonBottom, backgroundImage: v4Gradient(v4.buttonTop, v4.buttonBottom) },
  buttonText: { color: v4.buttonText, fontSize: v4Metrics.font, lineHeight: 20 },
  focused: { borderColor: v4.focus, borderTopColor: v4.focusTop, borderLeftColor: v4.focus,
    borderRightColor: v4.focus, borderBottomColor: v4.focus },
  fieldGroup: { width: '100%', marginBottom: 7 },
  modernFieldInput: { minHeight: 48, color: v4.text, fontSize: v4Metrics.font, paddingHorizontal: 14,
    paddingVertical: 12, includeFontPadding: false },
  modernFormField: { backgroundColor: v4.dialogBody, borderRadius: 16, borderWidth: StyleSheet.hairlineWidth },
  searchField: { borderRadius: 24 },
  searchFieldContent: { flexDirection: 'row', alignItems: 'center', paddingLeft: 16, minHeight: 48 },
  label: { fontSize: v4Metrics.font, color: v4.label, lineHeight: 22 },
  fieldOutline: { borderWidth: 1, borderColor: 'transparent', padding: 1 },
  fieldFocused: { borderColor: v4.focus, borderTopColor: v4.focusTop },
  field: { minHeight: 40, fontSize: v4Metrics.font, color: v4.text, backgroundColor: v4.body,
    // y.a(Graphics,IIIIII), bytecode lines5387+: first color draws left/top;
    // the second draws right/bottom. Form fields pass theme52 then theme53.
    borderWidth: 1, borderTopColor: v4.bevelLight, borderLeftColor: v4.bevelLight,
    borderRightColor: v4.bevelDark, borderBottomColor: v4.bevelDark, borderRadius: 1,
    paddingHorizontal: 5, paddingVertical: 6, includeFontPadding: false },
  checkRow: { minHeight: v4Metrics.touchTarget, flexDirection: 'row', alignItems: 'center', paddingHorizontal: 2, gap: 6,
    borderWidth: 1, borderColor: 'transparent' },
  checkBox: { width: 20, height: 20, backgroundColor: '#ffffff', justifyContent: 'center', alignItems: 'center',
    borderWidth: 1, borderTopColor: v4.bevelLight, borderLeftColor: v4.bevelLight,
    borderRightColor: v4.bevelDark, borderBottomColor: v4.bevelDark },
  linkTarget: { minHeight: v4Metrics.touchTarget, minWidth: v4Metrics.touchTarget, justifyContent: 'center', alignSelf: 'flex-start', paddingHorizontal: 2,
    borderWidth: 1, borderColor: 'transparent' },
  link: { fontSize: v4Metrics.font, color: v4.link, textDecorationLine: 'underline', lineHeight: 21 },
  rowFocus: { borderColor: v4.focus, borderTopColor: v4.focusTop },
  disabled: { opacity: .5 },
  disabledText: { color: v4.muted },
  commandBar: { minHeight: v4Metrics.commandHeight, flexDirection: 'row', justifyContent: 'space-between',
    backgroundColor: '#ffffff', borderTopWidth: 1, borderColor: v4.border },
  command: { minHeight: v4Metrics.commandHeight, minWidth: 70, justifyContent: 'center', alignItems: 'flex-start', paddingHorizontal: 10,
    borderWidth: 1, borderColor: 'transparent' },
  commandRight: { alignItems: 'flex-end' },
  commandText: { fontSize: v4Metrics.font, fontWeight: '600', color: '#222222' },
  commandDock: { flexDirection: 'row', justifyContent: 'space-between', gap: 10, paddingHorizontal: 20, paddingVertical: 8 },
  nativeNavigation: { flexDirection: 'row', alignItems: 'center', gap: 8, paddingHorizontal: 12,
    paddingTop: 6, paddingBottom: 14, minHeight: 68 },
  nativeDestinations: { flexDirection: 'row', alignItems: 'center', gap: 6, flexShrink: 0 },
  nativeConversations: { flexDirection: 'row', alignItems: 'center', gap: 8, paddingHorizontal: 3, paddingVertical: 4 },
  glassActionTarget: { minHeight: 48, minWidth: 72 },
  glassAction: { minHeight: 48, minWidth: 72, borderRadius: 24, paddingHorizontal: 20,
    justifyContent: 'center', alignItems: 'center', borderWidth: StyleSheet.hairlineWidth, borderColor: v4.border },
  glassButtonTarget: { minHeight: 54, alignSelf: 'stretch', paddingVertical: 3 },
  glassButton: { minHeight: 54, borderRadius: 27 },
  glassButtonContent: { minHeight: 54, borderRadius: 27, paddingHorizontal: 22, paddingVertical: 13,
    justifyContent: 'center', alignItems: 'center' },
  glassButtonText: { fontSize: 17, lineHeight: 26, fontWeight: '700' },
  tabDock: { paddingHorizontal: 20, paddingTop: 4, paddingBottom: 10 },
  glassTabs: { borderRadius: 28, borderWidth: StyleSheet.hairlineWidth, borderColor: v4.border,
    flexDirection: 'row', alignItems: 'center' },
  glassTabsContent: { width: '100%', minHeight: 54, flexDirection: 'row', alignItems: 'center' },
  navigationMenu: { width: 48, height: 48, marginRight: 3, borderRadius: 24,
    alignItems: 'center', justifyContent: 'center' },
  tabStrip: { backgroundColor: v4.stripBottom, backgroundImage: v4Gradient(v4.stripTop, v4.stripBottom),
    borderBottomWidth: 2, borderBottomColor: v4.border },
  tabs: { minHeight: v4Metrics.tabHeight + 4, paddingTop: 4, paddingHorizontal: 5, gap: 3, alignItems: 'flex-end' },
  tab: { height: v4Metrics.tabHeight, minWidth: 44, paddingHorizontal: 9, flexDirection: 'row', gap: 6, alignItems: 'center',
    justifyContent: 'center', borderTopLeftRadius: 4, borderTopRightRadius: 4,
    borderWidth: 1, borderColor: v4.tabBorder, borderBottomWidth: 0, backgroundColor: v4.inactiveTabTop,
    backgroundImage: v4Gradient(v4.inactiveTabTop, v4.inactiveTabBottom) },
  activeTab: { minWidth: 104, maxWidth: 190, backgroundColor: v4.activeTabBottom,
    backgroundImage: v4Gradient(v4.activeTabTop, v4.activeTabBottom) },
  unreadTab: { backgroundImage: v4Gradient(v4.unreadTop, v4.unreadBottom) },
  modernUnreadPlate: { borderRadius: 23, flexDirection: 'row', gap: 6, alignItems: 'center', justifyContent: 'center',
    paddingHorizontal: 16, backgroundColor: v4.unreadBottom, backgroundImage: v4Gradient(v4.unreadTop, v4.unreadBottom),
    boxShadow: '0 1px 5px rgba(229, 130, 48, 0.28)' },
  tabTitle: { color: v4.tabText, fontSize: v4Metrics.font, fontWeight: '700', flexShrink: 1 },
  overlay: { flex: 1, backgroundColor: 'rgba(0,0,0,0.58)', justifyContent: 'flex-end', paddingBottom: 78 },
  menu: { alignSelf: 'flex-start', minWidth: 190, maxWidth: '90%', maxHeight: '75%', marginLeft: 2,
    borderWidth: 1, borderColor: v4.menuBorder, backgroundColor: v4.menuBody },
  menuTitle: { color: v4.menuText, fontSize: v4Metrics.font, fontWeight: '700', paddingHorizontal: 7,
    paddingVertical: 5, backgroundColor: v4.menuHeader },
  menuItems: { flexGrow: 0 },
  sheetHandle: { alignSelf: 'center', width: 34, height: 4, borderRadius: 2, backgroundColor: v4.border, marginTop: 4, marginBottom: 7 },
  menuItem: { minHeight: v4Metrics.touchTarget, flexDirection: 'row', alignItems: 'center', gap: 8, paddingHorizontal: 8, paddingVertical: 7 },
  menuSeparator: { borderTopWidth: 1, borderTopColor: '#57b9e9' },
  menuPressed: { backgroundColor: v4.menuSelected },
  menuText: { color: v4.menuText, fontSize: v4Metrics.font, lineHeight: 20, flexShrink: 1 },
  menuDisabled: { color: v4.menuDisabled },
  modalCommands: { position: 'absolute', left: 0, right: 0, bottom: 0, paddingBottom: 34, backgroundColor: '#ffffff' },
  modernModalCommands: { position: 'absolute', left: 0, right: 0, bottom: 0, backgroundColor: '#ffffff' },
  dialogOverlay: { flex: 1, padding: 18, justifyContent: 'center', backgroundColor: 'rgba(0,0,0,0.58)' },
  dialog: { borderWidth: 2, borderColor: v4.dialogBorder, backgroundColor: v4.dialogBody, padding: 10, maxHeight: '75%' },
  dialogTitle: { color: v4.dialogTitle, fontSize: 16, fontWeight: '700', marginBottom: 8 },
  dialogMessageScroll: { flexGrow: 0 },
  dialogMessage: { color: v4.text, fontSize: v4Metrics.font, lineHeight: 22 },
  dialogActions: { flexDirection: 'row', gap: 12, justifyContent: 'flex-end', marginTop: 8 },
});
  if (!modern) return styles;
  return { ...styles, ...StyleSheet.create({
    header: { ...styles.header, paddingHorizontal: 24, paddingVertical: 12, backgroundColor: 'transparent', backgroundImage: undefined },
    headerTitle: { ...styles.headerTitle, color: v4.text, fontSize: 20, lineHeight: 28, textAlign: 'left', fontWeight: '600' },
    buttonTarget: { ...styles.buttonTarget, paddingHorizontal: 0, paddingVertical: 3 },
    button: { ...styles.button, minHeight: 48, paddingHorizontal: 14, paddingVertical: 11,
      borderRadius: 24, borderColor: v4.buttonBottom, borderTopColor: v4.buttonBottom,
      borderLeftColor: v4.buttonBottom, borderRightColor: v4.buttonBottom, borderBottomColor: v4.buttonBottom,
      backgroundImage: undefined },
    buttonText: { ...styles.buttonText, fontWeight: '600', lineHeight: 24 },
    fieldGroup: { ...styles.fieldGroup, marginBottom: 14 },
    label: { ...styles.label, marginBottom: 5, fontWeight: '500', lineHeight: 24 },
    fieldOutline: { ...styles.fieldOutline, borderRadius: 15, padding: 2 },
    field: { ...styles.field, minHeight: 50, borderRadius: 15, backgroundColor: v4.dialogBody,
      paddingHorizontal: 13, paddingVertical: 11 },
    checkRow: { ...styles.checkRow, paddingHorizontal: 3, gap: 11, borderRadius: 10 },
    checkBox: { ...styles.checkBox, width: 23, height: 23, borderRadius: 7, backgroundColor: v4.dialogBody },
    linkTarget: { ...styles.linkTarget, paddingHorizontal: 3, borderRadius: 10 },
    link: { ...styles.link, textDecorationLine: 'none', fontWeight: '500', lineHeight: 24 },
    commandBar: { ...styles.commandBar, paddingHorizontal: 6, borderColor: v4.border },
    command: { ...styles.command, paddingHorizontal: 14, borderRadius: 12 },
    commandText: { ...styles.commandText, color: v4.link },
    tabStrip: { ...styles.tabStrip, backgroundImage: undefined, borderBottomWidth: 1 },
    tabs: { ...styles.tabs, minHeight: 54, paddingTop: 5, paddingBottom: 5,
      paddingHorizontal: 5, gap: 4, alignItems: 'center' },
    tab: { ...styles.tab, maxWidth: 190, minWidth: 44, borderRadius: 23,
      borderWidth: increaseContrast ? 1.5 : StyleSheet.hairlineWidth,
      borderBottomWidth: increaseContrast ? 1.5 : StyleSheet.hairlineWidth, borderColor: 'transparent',
      borderTopLeftRadius: 23, borderTopRightRadius: 23,
      paddingHorizontal: 16, backgroundImage: undefined, backgroundColor: 'transparent' },
    activeTab: { ...styles.activeTab, minWidth: 104, backgroundImage: undefined,
      backgroundColor: controlColors.selectedFill, borderColor: controlColors.selectedOutline },
    unreadTab: { backgroundImage: undefined },
    tabTitle: { ...styles.tabTitle, fontWeight: '600' },
    overlay: { ...styles.overlay, backgroundColor: 'rgba(12,30,42,0.42)', paddingBottom: 100,
      paddingHorizontal: 12 },
    menu: { ...styles.menu, alignSelf: 'stretch', minWidth: 0, maxWidth: '100%', marginLeft: 0,
      borderRadius: 30, overflow: 'hidden', borderWidth: 0, paddingVertical: 9 },
    menuTitle: { ...styles.menuTitle, fontSize: 19, lineHeight: 27, paddingHorizontal: 20, paddingVertical: 12 },
    menuItem: { ...styles.menuItem, paddingHorizontal: 20, paddingVertical: 13, gap: 12 },
    menuSeparator: { ...styles.menuSeparator, borderTopColor: v4.border },
    menuText: { ...styles.menuText, lineHeight: 24 },
    modalCommands: { ...styles.modalCommands, paddingBottom: 34 },
    modernModalCommands: { ...styles.modernModalCommands, backgroundColor: 'transparent' },
    dialogOverlay: { ...styles.dialogOverlay, padding: 24, backgroundColor: 'rgba(12,30,42,0.42)' },
    dialog: { ...styles.dialog, borderWidth: 0, borderRadius: 28, padding: 24 },
    dialogTitle: { ...styles.dialogTitle, fontSize: 21, lineHeight: 28, marginBottom: 10 },
    dialogMessage: { ...styles.dialogMessage, lineHeight: 24 },
    dialogActions: { ...styles.dialogActions, marginTop: 18, gap: 10 },
  }) };
}

const classicStyles = createStyles(v4, v4Metrics, false);
const modernStyles = createStyles(modernTheme, modernMetrics, true);
const modernDarkStyles = createStyles(modernDarkTheme, modernMetrics, true, true);
const modernHighContrastStyles = createStyles(modernHighContrastTheme, modernMetrics, true, false, true);
const modernDarkHighContrastStyles = createStyles(modernDarkHighContrastTheme, modernMetrics, true, true, true);
const v3Base = createStyles(v3Theme, v3Metrics, false);
const classicV3Styles = { ...v3Base, ...StyleSheet.create({
  header: { ...v3Base.header, backgroundImage: undefined, backgroundColor: '#ffffff',
    borderWidth: 1, borderColor: '#000000', paddingHorizontal: 1, paddingVertical: 1 },
  headerTitle: { ...v3Base.headerTitle, backgroundColor: v3Colors.header, lineHeight: 24, fontWeight: '400' },
  button: { ...v3Base.button, borderRadius: 0, minHeight: 26, minWidth: 0, paddingHorizontal: 7 },
  fieldGroup: { ...v3Base.fieldGroup, marginBottom: 4 },
  field: { ...v3Base.field, borderRadius: 0 },
  checkBox: { ...v3Base.checkBox, width: 16, height: 16, borderColor: '#000000',
    borderTopColor: '#000000', borderLeftColor: '#000000', borderRightColor: '#000000', borderBottomColor: '#000000' },
  commandBar: { ...v3Base.commandBar, borderColor: '#000000' },
  overlay: { ...v3Base.overlay, justifyContent: 'center', paddingBottom: 44, backgroundColor: 'transparent' },
  menu: { ...v3Base.menu, alignSelf: 'center', width: '87.5%', minWidth: 0, maxWidth: '90%',
    marginLeft: 0, padding: 2, boxShadow: '3px 3px 0px #a0a0a0' },
  menuTitle: { ...v3Base.menuTitle, textAlign: 'center', fontWeight: '400', paddingVertical: 4, marginBottom: 2 },
  menuItem: { ...v3Base.menuItem, paddingHorizontal: 4, paddingVertical: 4 },
  menuSeparator: { ...v3Base.menuSeparator, borderTopColor: v3Colors.shadow },
  menuText: { ...v3Base.menuText, paddingHorizontal: 3, borderWidth: 1, borderColor: 'transparent' },
  dialogOverlay: { ...v3Base.dialogOverlay, backgroundColor: 'transparent', padding: 22 },
  dialog: { ...v3Base.dialog, padding: 2, borderWidth: 1, boxShadow: '3px 3px 0px #a0a0a0' },
  dialogTitle: { ...v3Base.dialogTitle, backgroundColor: v3Colors.header, textAlign: 'center',
    paddingVertical: 4, fontSize: 15, lineHeight: 20, fontWeight: '400' },
  dialogMessage: { ...v3Base.dialogMessage, textAlign: 'center', paddingHorizontal: 10, lineHeight: 20 },
  dialogActions: { ...v3Base.dialogActions, paddingHorizontal: 8, marginTop: 6 },
}) };
function usePrimitiveAppearance() {
  const appearance = useAppearance();
  return { ...appearance, styles: appearance.v3 ? classicV3Styles :
    appearance.modern ? (appearance.dark
      ? appearance.increaseContrast ? modernDarkHighContrastStyles : modernDarkStyles
      : appearance.increaseContrast ? modernHighContrastStyles : modernStyles) : classicStyles };
}
