import React, { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { AccessibilityInfo, Platform, useColorScheme } from 'react-native';
import { appearanceMetrics, appearanceThemes, modernAppearanceTheme, modernNavigationInset, type AppearanceMetrics, type AppearanceMode, type AppearanceTheme } from './appearanceTheme';

export type { AppearanceMode, AppearanceMetrics, AppearanceTheme } from './appearanceTheme';

interface Appearance {
  mode: AppearanceMode;
  setMode: (mode: AppearanceMode) => void;
  modern: boolean;
  v3: boolean;
  dark: boolean;
  reduceMotion: boolean;
  reduceTransparency: boolean;
  increaseContrast: boolean;
  theme: AppearanceTheme;
  metrics: AppearanceMetrics;
  navigationInset: number;
  setNavigationInset: (height: number) => void;
}

const AppearanceContext = createContext<Appearance | null>(null);

/** Controlled preference: changing colors never replaces the live session tree. */
export function AppearanceProvider({ mode, onChange, children }: {
  mode: AppearanceMode; onChange: (mode: AppearanceMode) => void; children: ReactNode;
}) {
  const scheme = useColorScheme();
  const dark = mode === 'modern' && scheme === 'dark';
  // Conservative defaults prevent an initial animation or transparent flash
  // before native accessibility preferences have arrived.
  const [reduceMotion, setReduceMotion] = useState(true);
  const [reduceTransparency, setReduceTransparency] = useState(true);
  const [increaseContrast, setIncreaseContrast] = useState(false);
  const [navigationInset, setNavigationInset] = useState(modernNavigationInset);
  useEffect(() => {
    let active = true;
    let motionRevision = 0;
    let transparencyRevision = 0;
    let contrastRevision = 0;
    const motion = AccessibilityInfo.addEventListener('reduceMotionChanged', (enabled) => {
      motionRevision++; setReduceMotion(enabled);
    });
    const transparency = Platform.OS === 'ios'
      ? AccessibilityInfo.addEventListener('reduceTransparencyChanged', (enabled) => {
        transparencyRevision++; setReduceTransparency(enabled);
      }) : undefined;
    const contrast = Platform.OS === 'ios' || Platform.OS === 'android'
      ? AccessibilityInfo.addEventListener(Platform.OS === 'ios' ? 'darkerSystemColorsChanged' : 'highTextContrastChanged', (enabled) => {
        contrastRevision++; setIncreaseContrast(enabled);
      }) : undefined;
    void AccessibilityInfo.isReduceMotionEnabled().then((enabled) => {
      if (active && motionRevision === 0) setReduceMotion(enabled);
    }).catch(() => {});
    if (Platform.OS === 'ios') void AccessibilityInfo.isReduceTransparencyEnabled().then((enabled) => {
      if (active && transparencyRevision === 0) setReduceTransparency(enabled);
    }).catch(() => {});
    const contrastQuery = Platform.OS === 'ios' ? AccessibilityInfo.isDarkerSystemColorsEnabled()
      : Platform.OS === 'android' ? AccessibilityInfo.isHighTextContrastEnabled() : undefined;
    void contrastQuery?.then((enabled) => {
      if (active && contrastRevision === 0) setIncreaseContrast(enabled);
    }).catch(() => {});
    return () => { active = false; motion.remove(); transparency?.remove(); contrast?.remove(); };
  }, []);
  const value = useMemo<Appearance>(() => ({ mode, setMode: onChange, modern: mode === 'modern', v3: mode === 'classic-v3', dark,
    reduceMotion, reduceTransparency, increaseContrast, navigationInset, setNavigationInset,
    theme: mode === 'modern' ? modernAppearanceTheme(dark, increaseContrast) : appearanceThemes[mode],
    metrics: appearanceMetrics[mode] }), [mode, onChange, dark, reduceMotion, reduceTransparency, increaseContrast, navigationInset]);
  return <AppearanceContext.Provider value={value}>{children}</AppearanceContext.Provider>;
}

export function useAppearance(): Appearance {
  const appearance = useContext(AppearanceContext);
  if (!appearance) throw new Error('AppearanceProvider is required.');
  return appearance;
}
