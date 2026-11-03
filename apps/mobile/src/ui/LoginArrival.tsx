import React, { useLayoutEffect, useRef, type ReactNode } from 'react';
import { Animated, Easing, View } from 'react-native';
import { useAppearance } from './appearance';
import { modernMotionDuration, useModernMotion } from './ModernMotion';

/** Friends contains native menu anchors, so this wrapper never fades it.
 * Coordinated children own their stagger; the provider owns completion. */
export function LoginArrival({ active, onComplete, children }: {
  active: boolean; onComplete: () => void; children: ReactNode;
}) {
  const { modern, reduceMotion } = useAppearance();
  const coordinated = useModernMotion() !== null;
  const progress = useRef(new Animated.Value(1)).current;
  const complete = useRef(onComplete);
  complete.current = onComplete;
  useLayoutEffect(() => {
    progress.stopAnimation();
    if (coordinated || !active) { progress.setValue(1); return; }
    if (!modern || reduceMotion) { progress.setValue(1); complete.current(); return; }
    progress.setValue(0);
    const animation = Animated.timing(progress, { toValue: 1, duration: modernMotionDuration.arriving,
      easing: Easing.bezier(.18, .72, .2, 1), useNativeDriver: true, isInteraction: false,
    });
    animation.start(({ finished }) => { if (finished) complete.current(); });
    return () => animation.stop();
  }, [active, coordinated, modern, reduceMotion, progress]);
  if (coordinated) return <View testID={active ? 'login.arrival' : undefined} style={{ flex: 1 }}>{children}</View>;
  return <Animated.View testID={active ? 'login.arrival' : undefined} style={{ flex: 1, opacity: 1,
    transform: [{ translateY: progress.interpolate({ inputRange: [0, 1], outputRange: [16, 0] }) }],
  }}>{children}</Animated.View>;
}
