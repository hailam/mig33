import React, { useEffect, useRef, useState } from 'react';
import { AppState, StyleSheet, View, type NativeSyntheticEvent } from 'react-native';
import { requireNativeView } from 'expo';
import { isGlassEffectAPIAvailable, isLiquidGlassAvailable } from 'expo-glass-effect';
import { useAppearance } from './appearance';
import { ModernLogo } from './ModernLogo';
import type { ModernBrandProps } from './ModernBrand';
export type { ModernBrandProps } from './ModernBrand';

const NativeBrand = requireNativeView<{
  animate: boolean; settled: boolean; dark: boolean; style: { width: number; height: number };
  onReady: (event: NativeSyntheticEvent<Record<string, never>>) => void;
  onComplete: (event: NativeSyntheticEvent<Record<string, never>>) => void;
}>('Mig33BrandGlass', 'Mig33BrandGlassView');

/** The same native glass surface forms the mark and remains at rest. The
 * original image is reserved for accessibility and unsupported systems. */
export function ModernBrand({ width = 176, animate = false, settled = false, onReady, onComplete }: ModernBrandProps) {
  const { dark, modern, reduceMotion, reduceTransparency, increaseContrast } = useAppearance();
  const [interrupted, setInterrupted] = useState(AppState.currentState !== null && AppState.currentState !== 'active');
  const [finished, setFinished] = useState(false);
  const ready = useRef(false), completed = useRef(false), laidOut = useRef(false);
  const callbacks = useRef({ onReady, onComplete }); callbacks.current = { onReady, onComplete };
  const capable = modern && isGlassEffectAPIAvailable() && isLiquidGlassAvailable();
  const fallback = reduceMotion || reduceTransparency || increaseContrast || !capable;
  const settledPose = settled || interrupted || finished;
  function reportReady() {
    if (ready.current) return;
    ready.current = true; callbacks.current.onReady?.();
  }
  function finish() {
    if (completed.current) return;
    completed.current = true; setFinished(true); callbacks.current.onComplete?.();
  }
  useEffect(() => {
    const listener = AppState.addEventListener('change', (state) => { if (state !== 'active') setInterrupted(true); });
    return () => listener.remove();
  }, []);
  useEffect(() => {
    // Accessibility can replace a native subtree after the wrapper laid out.
    // A fallback reports its real layout; native readiness still comes from
    // its display-frame callback. Both complete the episode only once.
    if (fallback && laidOut.current) {
      reportReady();
      if (animate || settled) finish();
    }
  }, [fallback, animate, settled]);
  return <View testID={fallback || settledPose ? 'brand.static' : 'brand.forming'} style={{ width, height: width * 182 / 320 }}
    accessible accessibilityRole="image" accessibilityLabel="mig33"
    onLayout={() => {
      laidOut.current = true;
      if (fallback) {
        reportReady();
        if (animate || settled) finish();
      }
    }}>
    {fallback ? <View accessible={false} accessibilityElementsHidden style={StyleSheet.absoluteFill}>
      <ModernLogo width={width} />
    </View> : <NativeBrand animate={animate} settled={settledPose} dark={dark}
      style={{ width, height: width * 182 / 320 }} onReady={reportReady} onComplete={finish} />}
  </View>;
}
