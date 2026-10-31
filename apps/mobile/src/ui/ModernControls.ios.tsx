import React, { useState, type ComponentProps } from 'react';
import { StyleSheet, View } from 'react-native';
import { Background, Button, Host, Image, Label, Picker, ProgressView, Rectangle, Text, Toggle, ZStack } from '@expo/ui/swift-ui';
import {
  accessibilityAddTraits, accessibilityHidden, accessibilityLabel, accessibilityValue,
  animation, Animation, aspectRatio, buttonBorderShape, buttonStyle, controlSize, disabled, font,
  foregroundStyle, frame, labelsHidden, labelStyle, lineLimit, mask, offset, opacity, pickerStyle,
  resizable, strokeBorder, tag, tint, toggleStyle,
} from '@expo/ui/swift-ui/modifiers';
import { isGlassEffectAPIAvailable, isLiquidGlassAvailable } from 'expo-glass-effect';
import { useAppearance } from './appearance';
import type { ModernButtonProps, ModernChoiceProps, ModernSymbolProps, ModernToggleProps } from './ModernControls';

export type { ModernButtonProps, ModernChoiceProps, ModernSymbolProps, ModernToggleProps } from './ModernControls';
export const supportsModernControls: boolean = true;
type SystemImage = ComponentProps<typeof Button>['systemImage'];

/** A native button owns both its material and its press behavior. Do not wrap
 * this host in another glass plate, opacity animation, or React press target. */
export function ModernButton({ label, onPress, variant = 'secondary', disabled: unavailable = false,
  busy = false, style, testID, accessibilityLabel: spokenLabel, systemImage, iconOnly = false,
  selected = false, highlighted = false, unreadPhase, tintColor, foregroundColor, controlSize: size = 'regular',
}: ModernButtonProps) {
  const { dark, theme, reduceTransparency } = useAppearance();
  const layout = StyleSheet.flatten(style);
  const stretch = layout?.alignSelf === 'stretch' || typeof layout?.width === 'number' ||
    (typeof layout?.width === 'string' && layout.width !== 'auto') ||
    (typeof layout?.flex === 'number' && layout.flex > 0) || (layout?.flexGrow ?? 0) > 0;
  const [allocatedWidth, setAllocatedWidth] = useState<number>();
  const width = stretch ? typeof layout?.width === 'number' ? layout.width : allocatedWidth : undefined;
  const maxWidth = typeof layout?.maxWidth === 'number' ? layout.maxWidth : undefined;
  const minWidth = Math.max(44, typeof layout?.minWidth === 'number' ? layout.minWidth : 0);
  const glass = isGlassEffectAPIAvailable() && isLiquidGlassAvailable() && !reduceTransparency;
  const material = variant !== 'plain';
  const tab = unreadPhase !== undefined;
  const unread = tab && unreadPhase !== 'none';
  // Primary actions keep the native transparent material visible. Reserve
  // prominent tinted glass for persistent selection/highlight state; changing
  // a tint during an unread pulse must not switch the native button style.
  const prominent = selected || highlighted || (!glass && unread);
  const nativeStyle = !material ? 'plain' : glass
    ? prominent ? 'glassProminent' : 'glass'
    : prominent ? 'borderedProminent' : 'bordered';
  const color = !glass && unread ? unreadPhase === 'off' ? '#f4c68f' : '#f5a34e' : tintColor ?? theme.link;
  const accent = !glass && unread ? '#38200d' : foregroundColor ?? (variant === 'primary' ? theme.link : theme.text);
  // An explicit foreground style overrides SwiftUI's disabled ink. Keep the
  // material intact while making unavailable actions visibly inactive.
  const ink = unavailable && !busy ? theme.muted : accent;
  const button = <Button testID={testID} onPress={() => { if (!unavailable && !busy) onPress(); }} modifiers={[
    buttonStyle(nativeStyle), buttonBorderShape(iconOnly ? 'circle' : 'capsule'), controlSize(size),
    tint(color), disabled(unavailable || busy), accessibilityLabel(spokenLabel ?? label),
    accessibilityValue(busy ? 'In progress' : ''),
    ...(selected ? [accessibilityAddTraits(['isSelected'])] : []),
    foregroundStyle(ink),
    ...(width !== undefined ? [frame({ width: Math.max(44, width) })]
      : [frame({ minWidth, maxWidth })]),
    frame({ minHeight: 44 }),
  ]}>
    {/* Constrain the label inside the actual button. An outer44pt frame
        alone leaves the system button's accessibility bounds at42pt. */}
    <ZStack modifiers={[frame({ minHeight: material ? 30 : 44, ...(stretch ? { maxWidth: Infinity } : {}) })]}>
      {/* Only the label fades while busy; the material stays at opacity1
          and the native button retains its size and accessibility node. */}
      <Label title={label} systemImage={systemImage as SystemImage} modifiers={[
        labelStyle(iconOnly && systemImage ? 'iconOnly' : systemImage ? 'titleAndIcon' : 'titleOnly'),
        font({ textStyle: 'body', weight: 'semibold' }), opacity(busy ? 0 : 1),
        ...(maxWidth !== undefined ? [lineLimit(1)] : []),
      ]} />
      {busy && <ProgressView modifiers={[accessibilityHidden(), tint(accent)]} />}
    </ZStack>
  </Button>;
  return <View accessible={false} testID={material ? glass ? 'glass.native' : 'glass.solid' : undefined}
    style={[styles.button, style]} onLayout={(event) => {
      const next = event.nativeEvent.layout.width;
      if (stretch && next > 0) setAllocatedWidth((before) => before !== undefined && Math.abs(before - next) < .5 ? before : next);
    }}>
    {/* Keep matchContents fixed for this host's lifetime, including when a
        navigation destination changes between a full label and compact icon.
        Yoga supplies stretched widths; native labels supply intrinsic widths. */}
    <Host matchContents alignment="center" colorScheme={dark ? 'dark' : 'light'} seedColor={color}>
      {tab ? <Background>
        {button}
        <Background.Content>
          {/* This is color behind the real native material, not another glass
              layer. Background keeps the button's intrinsic layout unchanged. */}
          <UnreadAmberField phase={glass ? unreadPhase : 'none'} circle={iconOnly} />
        </Background.Content>
      </Background> : button}
    </Host>
  </View>;
}

/** SwiftUI moves the background color locally; the glass and its touch target
 * never translate, fade or change style as the shared750ms phase advances. */
function UnreadAmberField({ phase, circle }: { phase: NonNullable<ModernButtonProps['unreadPhase']>; circle: boolean }) {
  const { dark, theme, reduceMotion, increaseContrast } = useAppearance();
  const active = phase !== 'none';
  const steady = reduceMotion || phase === 'steady';
  const bright = active && (steady || phase === 'on');
  const strength = increaseContrast ? .6 : 1;
  const drift = !active || steady ? 0 : phase === 'on' ? 14 : -14;
  const timing = animation(Animation.easeInOut({ duration: active && !steady ? .75 : 0 }), bright);
  // Regular glass softens the field substantially in light appearance. Use
  // saturated orange underneath it, with a quieter dark field for light ink.
  const firstOpacity = active ? (bright ? dark ? .34 : .76 : dark ? .13 : .24) * strength : 0;
  const secondOpacity = active ? (bright ? dark ? .18 : .44 : dark ? .07 : .14) * strength : 0;
  return <ZStack modifiers={[mask(circle ? 'circle' : 'capsule'),
    // Keep even the contrast edge behind the native Button's hit target.
    strokeBorder({ shape: circle ? 'circle' : 'capsule',
      color: active && increaseContrast ? theme.focus : 'clear', style: { lineWidth: 1.25 } }), accessibilityHidden()]}>
    <Rectangle modifiers={[foregroundStyle('#ff7900'),
      opacity(active ? (bright ? dark ? .18 : .32 : dark ? .04 : .075) * strength : 0), timing]} />
    <Rectangle modifiers={[foregroundStyle({ type: 'radialGradient', colors: ['#ff820a', 'rgba(255,130,10,0)'],
      center: { x: .22, y: .32 }, startRadius: 0, endRadius: 76 }),
      offset({ x: drift, y: steady ? 0 : -drift * .2 }), opacity(firstOpacity), timing]} />
    <Rectangle modifiers={[foregroundStyle({ type: 'radialGradient', colors: ['#ffb63e', 'rgba(255,182,62,0)'],
      center: { x: .78, y: .7 }, startRadius: 0, endRadius: 62 }),
      offset({ x: -drift, y: steady ? 0 : drift * .15 }), opacity(secondOpacity), timing]} />
  </ZStack>;
}

/** SwiftUI supplies the switch, its state announcements and its hit target. */
export function ModernToggle({ label, checked, onChange, disabled: unavailable = false, testID, style }: ModernToggleProps) {
  const { dark, theme } = useAppearance();
  return <Host matchContents={{ vertical: true }} style={[styles.formControl, style]}
    colorScheme={dark ? 'dark' : 'light'} seedColor={theme.link}>
    <Toggle label={label} isOn={checked} testID={testID}
      onIsOnChange={(next) => { if (!unavailable) onChange?.(next); }} modifiers={[
        toggleStyle('switch'), tint(theme.link), disabled(unavailable || !onChange),
        font({ textStyle: 'body' }), foregroundStyle(theme.text),
        frame({ minHeight: 44, maxWidth: Infinity }),
      ]} />
  </Host>;
}

/** The platform owns selection, checkmarks and menu/segmented interaction. */
export function ModernChoice({ label, accessibilityLabel: spokenLabel, hideLabel = false, value, options,
  onChange, disabled: unavailable = false, testID, style, variant = 'segmented',
}: ModernChoiceProps) {
  const { dark, theme } = useAppearance();
  return <Host matchContents={{ vertical: true }} style={[styles.formControl, style]}
    colorScheme={dark ? 'dark' : 'light'} seedColor={theme.link}>
    {/* Expo's native Picker needs a label slot even when it is intentionally
        empty; an absent label otherwise produces no native Picker at all. */}
    <Picker<string> label={label ?? ''} selection={value} testID={testID}
      onSelectionChange={(next) => {
        if (!unavailable && next !== value && options.some((option) => option.value === next && !option.disabled)) onChange(next);
      }} modifiers={[
        pickerStyle(variant), disabled(unavailable), tint(theme.link), font({ textStyle: 'body' }),
        frame({ minHeight: 44, maxWidth: Infinity, alignment: 'leading' }),
        ...(spokenLabel || label ? [accessibilityLabel(spokenLabel ?? label!)] : []),
        ...(hideLabel ? [labelsHidden()] : []),
      ]}>
      {options.map((option) => <Text key={option.value} testID={option.testID}
        modifiers={[tag(option.value), disabled(Boolean(option.disabled))]}>{option.label}</Text>)}
    </Picker>
  </Host>;
}

/** Decorative SF Symbol; the surrounding action carries its accessible name. */
export function ModernSymbol({ name, size, color }: ModernSymbolProps) {
  const { dark, theme } = useAppearance();
  return <Host pointerEvents="none" style={{ width: size, height: size }}
    colorScheme={dark ? 'dark' : 'light'} modifiers={[accessibilityHidden()]}>
    <Image systemName={name as SystemImage} color={color ?? theme.text} modifiers={[
      resizable(), aspectRatio({ contentMode: 'fit' }), frame({ width: size, height: size }), accessibilityHidden(),
    ]} />
  </Host>;
}

const styles = StyleSheet.create({
  button: { alignSelf: 'flex-start', minWidth: 44, minHeight: 44, flexShrink: 0, justifyContent: 'center' },
  formControl: { alignSelf: 'stretch', minHeight: 44, flexShrink: 0 },
});
