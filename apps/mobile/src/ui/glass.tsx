import React, { useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { Platform, View, type ColorValue, type StyleProp, type ViewStyle } from 'react-native';
import { GlassView, isGlassEffectAPIAvailable, isLiquidGlassAvailable } from 'expo-glass-effect';
import { useAppearance } from './appearance';

/** One native regular-glass control surface; content cards remain solid.
 * Keep this surface and its ancestors at opacity1, per UIVisualEffectView.
 * https://developer.apple.com/design/human-interface-guidelines/materials
 */
export function GlassSurface({ children, style, interactive = false, animateIn = false, tintColor }: {
  children: ReactNode; style?: StyleProp<ViewStyle>; interactive?: boolean; animateIn?: boolean; tintColor?: ColorValue;
}) {
  const { modern, dark, theme, reduceMotion, reduceTransparency } = useAppearance();
  const available = Platform.OS === 'ios' && isGlassEffectAPIAvailable() && isLiquidGlassAvailable();
  const enabled = modern && !reduceTransparency;
  const [laidOut, setLaidOut] = useState(false);
  const entered = useRef(false);
  const animateMaterial = animateIn && laidOut && enabled && !reduceMotion && !entered.current;
  useLayoutEffect(() => {
    if (laidOut && (enabled || reduceMotion)) entered.current = true;
  }, [laidOut, enabled, reduceMotion]);
  if (!available) return <View testID="glass.solid" style={[style, { backgroundColor: theme.dialogBody }]}>{children}</View>;
  // Retain the same native parent when accessibility or appearance changes:
  // a focused composer and the session beneath it must not be recreated.
  return <GlassView testID={enabled ? 'glass.native' : 'glass.solid'} style={[style, { backgroundColor: enabled ? 'transparent' : theme.dialogBody,
    backgroundImage: undefined }]} colorScheme={dark ? 'dark' : 'light'} tintColor={tintColor}
    onLayout={() => { if (!laidOut) setLaidOut(true); }}
    // Expo applies this through UIView.animate to the real UIGlassEffect.
    // Layout starts the effect immediately; no opacity wrapper or delay.
    glassEffectStyle={{ style: enabled && (laidOut || !animateIn || reduceMotion) ? 'regular' : 'none',
      animate: animateMaterial, animationDuration: .28 }} isInteractive={interactive && !reduceMotion}>
    {children}
  </GlassView>;
}
