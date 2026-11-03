import React from 'react';
import { Animated, Image, StyleSheet, View, useWindowDimensions } from 'react-native';
import { useAppearance } from './appearance';
import { useModernMotion } from './ModernMotion';

type BackdropVariant = 'auth' | 'chat' | 'home';
function gradient(dark: boolean, variant: BackdropVariant) {
  const colors = dark
    ? variant === 'auth' ? ['#163742', '#0b151c', '#211e22'] : ['#142c36', '#0b151c', '#101b22']
    : variant === 'auth' ? ['#dceef0', '#f7f9f7', '#f5e9dc'] : ['#e5eff1', '#f6f8f7', '#f2eee7'];
  return `linear-gradient(155deg, ${colors[0]} 0%, ${colors[1]} 55%, ${colors[2]} 100%)`;
}

/** Ordinary decorative light beneath real native materials. Only these colors
 * move or fade; the app, controls and glass presenters remain at full opacity. */
export function ModernBackdrop({ variant = 'home' }: { variant?: BackdropVariant }) {
  const { modern, dark, theme, increaseContrast, reduceTransparency } = useAppearance();
  const motion = useModernMotion();
  const { width, height } = useWindowDimensions();
  if (!modern) return null;
  const arriving = motion?.phase === 'arriving';
  const startup = motion?.phase === 'startup';
  const launching = startup || motion?.phase === 'preparing';
  const waiting = motion?.waiting;
  const authPhase = motion?.phase === 'authenticating' || startup || motion?.phase === 'preparing';
  const base = arriving ? 'home' : authPhase ? 'auth' : variant;
  const light = motion?.enabled && !increaseContrast && !reduceTransparency;
  const progress = arriving ? motion?.arrivalProgress : motion?.startupProgress;
  const sweep = light && (arriving || startup) && progress;
  const waitingWave = motion?.waitingProgress.interpolate({
    inputRange: [0, .125, .25, .375, .5, .625, .75, .875, 1],
    outputRange: [0, .1464, .5, .8536, 1, .8536, .5, .1464, 0],
  });
  const bandHeight = Math.max(260, height * .34);
  const teal = dark ? 'rgba(65,204,206,0.19)' : 'rgba(33,173,181,0.23)';
  const amber = dark ? 'rgba(245,155,72,0.14)' : 'rgba(244,166,91,0.20)';
  const band = `linear-gradient(0deg, transparent 0%, ${teal} 38%, ${amber} 62%, transparent 100%)`;
  return <View accessible={false} accessibilityElementsHidden importantForAccessibility="no-hide-descendants"
    pointerEvents="none" style={[StyleSheet.absoluteFill, {
      backgroundColor: launching ? dark ? '#000000' : '#ffffff' : theme.body, overflow: 'hidden',
    }]}>
    {!increaseContrast && <>
      {launching && motion ? <Animated.View style={[StyleSheet.absoluteFill, {
        backgroundImage: gradient(dark, 'auth'), opacity: motion.startupProgress,
      }]} /> : <View style={[StyleSheet.absoluteFill, { backgroundImage: gradient(dark, base) }]} />}
      {arriving && motion && <Animated.View style={[StyleSheet.absoluteFill, {
        backgroundImage: gradient(dark, 'auth'),
        opacity: motion.arrivalProgress.interpolate({ inputRange: [0, 1], outputRange: [1, 0] }),
      }]} />}
      {variant === 'chat' && <Image source={require('../../assets/grain.png')} resizeMode="repeat"
        style={[StyleSheet.absoluteFill, { width: '100%', height: '100%', opacity: dark ? .18 : .24 }]} />}
      {sweep && <Animated.View testID={arriving ? 'motion.arrival-light' : 'motion.startup-light'}
        style={{ position: 'absolute', left: -width * .4, top: 0, width: width * 1.8, height: bandHeight,
          backgroundImage: band,
          opacity: progress.interpolate({ inputRange: [0, .12, .72, 1], outputRange: [0, 1, .85, 0] }),
          transform: [
            { translateY: progress.interpolate({ inputRange: [0, 1],
              outputRange: arriving ? [height * .92, -bandHeight] : [height * .38, -bandHeight * .7] }) },
            { rotate: '-18deg' },
          ],
        }} />}
      {light && waiting && waitingWave && <Animated.View testID="motion.waiting-light" style={{
        position: 'absolute', left: -width * .4, top: height * .62, width: width * 1.8, height: bandHeight,
        backgroundImage: band,
        opacity: waitingWave.interpolate({ inputRange: [0, 1], outputRange: [.35, .8] }),
        transform: [
          { translateX: waitingWave.interpolate({ inputRange: [0, 1], outputRange: [-36, 36] }) },
          { translateY: waitingWave.interpolate({ inputRange: [0, 1], outputRange: [12, -12] }) },
          { rotate: '-18deg' },
        ],
      }} />}
    </>}
  </View>;
}
