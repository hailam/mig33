import React from 'react';
import { Image } from 'react-native';

/** Preserve the original bubble mark and custom lettering. The source image
 * remains unmodified; provenance and comparison notes live beside the asset. */
export function ModernLogo({ width = 144 }: { width?: number }) {
  return <Image source={require('../../assets/mig33-logo.png')} accessibilityRole="image"
    accessibilityLabel="mig33" resizeMode="contain" fadeDuration={0}
    style={{ width, height: width * 182 / 320 }} />;
}
