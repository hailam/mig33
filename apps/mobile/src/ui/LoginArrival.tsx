import React, { useLayoutEffect, useRef, type ReactNode } from 'react';
import { Animated, Easing } from 'react-native';
import { useAppearance } from './appearance';

/** Only Friends content fades here. Native glass controls remain outside this
 * subtree, at full opacity, so UIKit can render their material correctly. */
export function LoginArrival({ active, onComplete, children }: {
  active: boolean; onComplete: () => void; children: ReactNode;
}) {
  const { reduceMotion } = useAppearance();
  const progress = useRef(new Animated.Value(active && !reduceMotion ? 0 : 1)).current;
  const complete = useRef(onComplete);
  complete.current = onComplete;
  useLayoutEffect(() => {
    progress.stopAnimation();
    if (!active || reduceMotion) {
      progress.setValue(1);
      if (active) complete.current();
      return;
    }
    progress.setValue(0);
    Animated.timing(progress, { toValue: 1, duration: 420,
      easing: Easing.bezier(.16, .7, .2, 1), useNativeDriver: true,
    }).start(({ finished }) => { if (finished) complete.current(); });
    return () => progress.stopAnimation();
  }, [active, reduceMotion, progress]);
  return <Animated.View testID={active ? 'login.arrival' : undefined} style={{ flex: 1,
    opacity: progress, transform: [{ translateY: progress.interpolate({ inputRange: [0, 1], outputRange: [16, 0] }) }],
  }}>{children}</Animated.View>;
}
