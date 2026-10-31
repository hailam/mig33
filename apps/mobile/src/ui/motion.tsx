import React, { forwardRef, useEffect, useLayoutEffect, useRef, useState, type ComponentRef, type ReactNode } from 'react';
import { Animated, Easing, Pressable, StyleSheet, View, type PressableProps, type ViewProps } from 'react-native';
import { useAppearance } from './appearance';

const AnimatedPressable = Animated.createAnimatedComponent(Pressable);

/** Same hit target and accessibility props as Pressable; no layout wrapper. */
export const MotionPressable = forwardRef<ComponentRef<typeof View>, PressableProps>(function MotionPressable(
  { style, onPressIn, onPressOut, disabled, ...props }, ref,
) {
  const { modern, reduceMotion } = useAppearance();
  const scale = useRef(new Animated.Value(1)).current;
  const [pressed, setPressed] = useState(false);
  useEffect(() => {
    if (!modern || reduceMotion || disabled) { scale.stopAnimation(); scale.setValue(1); }
    if (disabled) setPressed(false);
    return () => scale.stopAnimation();
  }, [modern, reduceMotion, disabled, scale]);
  function animate(toValue: number, duration: number) {
    if (!modern || reduceMotion || disabled) return;
    Animated.timing(scale, { toValue, duration, easing: Easing.out(Easing.cubic), useNativeDriver: true }).start();
  }
  // AnimatedProps ignores a function-valued style. Resolve Pressable's style
  // callback here so the scale node is attached to the actual native host.
  const base = typeof style === 'function' ? style({ pressed: pressed || props.testOnly_pressed === true }) : style;
  const transform = StyleSheet.flatten(base)?.transform;
  return <AnimatedPressable {...props} ref={ref} disabled={disabled}
    onPressIn={(event) => { setPressed(true); animate(.975, 110); onPressIn?.(event); }}
    onPressOut={(event) => { setPressed(false); animate(1, 180); onPressOut?.(event); }}
    style={[base, { transform: [...(Array.isArray(transform) ? transform : []), { scale }] }]} />;
});

/** Modest one-shot transition. Disable fading around a native glass surface. */
export function MotionEntrance({ children, style, transitionKey, fade = true, ...props }: ViewProps & {
  children: ReactNode; transitionKey?: string | number; fade?: boolean;
}) {
  const { modern, reduceMotion } = useAppearance();
  const progress = useRef(new Animated.Value(1)).current;
  const previous = useRef<{ key?: string | number } | null>(null);
  useLayoutEffect(() => {
    const transition = previous.current === null || previous.current.key !== transitionKey;
    previous.current = { key: transitionKey };
    progress.stopAnimation();
    if (!modern || reduceMotion || !transition) { progress.setValue(1); return; }
    progress.setValue(0);
    Animated.timing(progress, { toValue: 1, duration: 220, easing: Easing.out(Easing.cubic), useNativeDriver: true }).start();
    return () => progress.stopAnimation();
  }, [transitionKey, modern, reduceMotion, progress]);
  return <Animated.View {...props} style={[style, { opacity: fade ? progress : 1,
    transform: [{ translateY: progress.interpolate({ inputRange: [0, 1], outputRange: [9, 0] }) }] }]}>
    {children}
  </Animated.View>;
}
