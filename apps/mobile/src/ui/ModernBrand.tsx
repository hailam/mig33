import React, { useEffect, useRef } from 'react';
import { View } from 'react-native';
import { ModernLogo } from './ModernLogo';

export interface ModernBrandProps {
  width?: number;
  animate?: boolean;
  /** Settle immediately after interaction, cancellation or a navigation change. */
  settled?: boolean;
  onReady?: () => void;
  onComplete?: () => void;
}

/** Other platforms retain the exact static brand without importing SwiftUI. */
export function ModernBrand({ width = 176, animate = false, settled = false, onReady, onComplete }: ModernBrandProps) {
  const ready = useRef(false), complete = useRef(false);
  const callbacks = useRef({ onReady, onComplete }); callbacks.current = { onReady, onComplete };
  useEffect(() => {
    if (ready.current && (animate || settled) && !complete.current) {
      complete.current = true; callbacks.current.onComplete?.();
    }
  }, [animate, settled]);
  return <View testID="brand.static" onLayout={() => {
    if (!ready.current) { ready.current = true; callbacks.current.onReady?.(); }
    if ((animate || settled) && !complete.current) { complete.current = true; callbacks.current.onComplete?.(); }
  }}><ModernLogo width={width} /></View>;
}
