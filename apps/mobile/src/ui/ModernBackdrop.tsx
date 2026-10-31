import React from 'react';
import { Image, StyleSheet, View } from 'react-native';
import { useAppearance } from './appearance';

/** Quiet content depth beneath native glass. This is ordinary background
 * artwork, never a simulated glass material or a moving layer behind text. */
export function ModernBackdrop({ variant = 'home' }: { variant?: 'auth' | 'chat' | 'home' }) {
  const { modern, dark, theme, increaseContrast } = useAppearance();
  if (!modern) return null;
  const colors = dark
    ? variant === 'auth' ? ['#163742', '#0b151c', '#211e22'] : ['#142c36', '#0b151c', '#101b22']
    : variant === 'auth' ? ['#dceef0', '#f7f9f7', '#f5e9dc'] : ['#e5eff1', '#f6f8f7', '#f2eee7'];
  return <View accessible={false} accessibilityElementsHidden importantForAccessibility="no-hide-descendants"
    pointerEvents="none" style={[StyleSheet.absoluteFill, { backgroundColor: theme.body }]}>
    {!increaseContrast && <>
      <View style={[StyleSheet.absoluteFill, {
        backgroundImage: `linear-gradient(155deg, ${colors[0]} 0%, ${colors[1]} 55%, ${colors[2]} 100%)`,
      }]} />
      {variant === 'chat' && <Image source={require('../../assets/grain.png')} resizeMode="repeat"
        style={[StyleSheet.absoluteFill, { width: '100%', height: '100%', opacity: dark ? .18 : .24 }]} />}
    </>}
  </View>;
}
