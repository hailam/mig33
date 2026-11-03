import React, { useEffect, useState } from 'react';
import { ActivityIndicator, Image, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import * as SplashScreen from 'expo-splash-screen';
import { ModernBrand } from './ModernBrand';
import { useModernMotion } from './ModernMotion';

// Run before React mounts; release is driven by a real matching frame, never
// the network or an animation-duration timer.
void SplashScreen.preventAutoHideAsync().catch(() => {});
export function releaseNativeLaunch() { void SplashScreen.hideAsync().catch(() => {}); }

/** Matches the native light/dark launch layout while local preferences resolve.
 * An automatic login continues here, so credentials never flash underneath. */
export function ModernLaunch({ dark, ready = false, connecting = false }: {
  dark: boolean; ready?: boolean; connecting?: boolean;
}) {
  const [laidOut, setLaidOut] = useState(false);
  const insets = useSafeAreaInsets();
  // Saved credentials can resolve before appearance and accessibility. Keep
  // the matching poster until those preferences can choose the real material.
  const activeConnecting = ready && connecting;
  useEffect(() => {
    if (!ready || !laidOut || connecting) return;
    const frame = requestAnimationFrame(releaseNativeLaunch);
    return () => cancelAnimationFrame(frame);
  }, [ready, laidOut, connecting]);
  return <View testID="launch.preparing" onLayout={() => setLaidOut(true)} style={[
    StyleSheet.absoluteFill, { backgroundColor: activeConnecting ? 'transparent' : dark ? '#000000' : '#ffffff' },
  ]}>
    <View style={{ flex: 1, paddingTop: insets.top, paddingBottom: insets.bottom,
      paddingLeft: insets.left, paddingRight: insets.right }}><View style={styles.brandColumn}>
      {activeConnecting ? <ModernBrand width={176} animate onReady={releaseNativeLaunch} /> : <Image
        source={require('../../assets/launch-mark.png')}
        accessibilityRole="image" accessibilityLabel="mig33" fadeDuration={0}
        style={{ width: 176, height: 176 * 182 / 320 }} />}
    </View></View>
    {activeConnecting && <View accessibilityLiveRegion="polite" style={styles.connecting}>
      <ActivityIndicator color={dark ? '#82d0d4' : '#006d83'} />
      <Text style={{ color: dark ? '#c7d9df' : '#385765', fontSize: 15 }}>Signing in…</Text>
    </View>}
  </View>;
}

/** Keep the same native mark mounted after its startup callbacks are removed.
 * Readiness comes from its first drawn frame; idle displays its settled pose. */
export function ModernStartupLogo({ onReady, onComplete }: { onReady?: () => void; onComplete?: () => void }) {
  const motion = useModernMotion();
  const startup = onReady !== undefined;
  return <ModernBrand width={176} animate={startup && motion?.phase === 'startup'}
    settled={!startup || (motion?.phase !== 'preparing' && motion?.phase !== 'startup')}
    onReady={onReady ? () => { onReady(); releaseNativeLaunch(); } : undefined} onComplete={onComplete} />;
}

const styles = StyleSheet.create({
  brandColumn: { width: '100%', maxWidth: 520, alignSelf: 'center', paddingTop: 16, paddingLeft: 3 },
  connecting: { position: 'absolute', top: '62%', alignSelf: 'center', flexDirection: 'row', alignItems: 'center', gap: 10 },
});
