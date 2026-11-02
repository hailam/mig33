import React, { createContext, useContext, useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Animated, AppState, Easing, StyleSheet, type ViewProps } from 'react-native';
import { useAppearance } from './appearance';

export type ModernMotionPhase = 'preparing' | 'startup' | 'authenticating' | 'arriving' | 'idle';
type FinitePhase = 'startup' | 'arriving';
export interface ModernMotionState {
  phase: ModernMotionPhase;
  startupProgress: Animated.Value;
  arrivalProgress: Animated.Value;
  /** Repeating native0..1 cycle; consumers map both endpoints to the same pose. */
  waitingProgress: Animated.Value;
  enabled: boolean;
  waiting: boolean;
}

const ModernMotionContext = createContext<ModernMotionState | null>(null);
const settle = Easing.bezier(.18, .72, .2, 1);
export const modernMotionDuration = { startup: 750, arriving: 650 } as const;

/** Shared clocks coordinate background and content around the native entry
 * episodes. The native logo owns its completion; the caller owns readiness
 * and authentication. Changing phase cancels obsolete content motion. */
export function ModernMotionProvider({ phase, connectionActive = false, onSettled, children }: {
  phase: ModernMotionPhase;
  /** The real session is connecting/authenticating, not merely restoring UI. */
  connectionActive?: boolean;
  onSettled?: (phase: FinitePhase) => void;
  children: ReactNode;
}) {
  const { modern, reduceMotion } = useAppearance();
  const [foreground, setForeground] = useState(() => AppState.currentState == null || AppState.currentState === 'active');
  const enabled = modern && !reduceMotion && foreground;
  const waiting = phase === 'authenticating' && connectionActive && enabled;
  const startupProgress = useRef(new Animated.Value(phase === 'preparing' || phase === 'startup' ? 0 : 1)).current;
  const arriving = phase === 'arriving';
  // Mount every new arrival with its source pose already in the native tree.
  // Resetting a reused value from1 in a layout effect can still paint one
  // settled frame before Fabric receives the reset, then visibly jump down.
  const arrivalProgress = useMemo(() => new Animated.Value(arriving ? 0 : 1), [arriving]);
  const waitingProgress = useRef(new Animated.Value(0)).current;
  const completion = useRef(onSettled);
  const generation = useRef(0);
  const episode = useRef({ phase, settled: false });
  const pause = useRef<() => void>(() => {});
  completion.current = onSettled;

  useEffect(() => {
    const listener = AppState.addEventListener('change', (state) => {
      // Stop and settle in the native event callback, before backgrounding can
      // suspend React's next commit. Foreground never replays a settled phase.
      if (state !== 'active') pause.current();
      setForeground(state === 'active');
    });
    return () => listener.remove();
  }, []);

  useLayoutEffect(() => {
    const current = ++generation.current;
    let animation: Animated.CompositeAnimation | undefined;
    if (episode.current.phase !== phase) episode.current = { phase, settled: false };
    startupProgress.stopAnimation(); arrivalProgress.stopAnimation(); waitingProgress.stopAnimation();
    startupProgress.setValue(phase === 'preparing' || phase === 'startup' ? 0 : 1);
    arrivalProgress.setValue(phase === 'arriving' ? 0 : 1);
    waitingProgress.setValue(0);
    const finite = phase === 'startup' || phase === 'arriving' ? phase : null;
    const finish = () => {
      if (!finite || generation.current !== current || episode.current.settled) return;
      episode.current.settled = true;
      completion.current?.(finite);
    };
    const pauseCurrent = () => {
      animation?.stop();
      startupProgress.stopAnimation(); arrivalProgress.stopAnimation(); waitingProgress.stopAnimation();
      startupProgress.setValue(1); arrivalProgress.setValue(1); waitingProgress.setValue(0);
      finish();
    };
    pause.current = pauseCurrent;
    if (finite) {
      const progress = finite === 'startup' ? startupProgress : arrivalProgress;
      if (!enabled || episode.current.settled) {
        progress.setValue(1);
        finish();
      } else {
        animation = Animated.timing(progress, { toValue: 1, duration: modernMotionDuration[finite],
          easing: settle, useNativeDriver: true, isInteraction: false });
        animation.start(({ finished }) => {
          if (finished) finish();
        });
      }
    } else if (waiting) {
      // Ambient waiting has no percentage or implied completion. It continues
      // only while the real connection is busy and never blocks list work.
      // A single timing loop stays native between cycles too. A sequence would
      // require JS callbacks to restart each half of the waiting motion.
      animation = Animated.loop(Animated.timing(waitingProgress, { toValue: 1, duration: 2900,
        easing: Easing.linear, useNativeDriver: true, isInteraction: false }));
      animation.start();
    }
    return () => {
      generation.current++;
      if (pause.current === pauseCurrent) pause.current = () => {};
      animation?.stop();
      startupProgress.stopAnimation(); arrivalProgress.stopAnimation(); waitingProgress.stopAnimation();
    };
  }, [phase, enabled, waiting, startupProgress, arrivalProgress, waitingProgress]);

  const value = useMemo(() => ({ phase, startupProgress, arrivalProgress, waitingProgress, enabled, waiting }),
    [phase, startupProgress, arrivalProgress, waitingProgress, enabled, waiting]);
  return <ModernMotionContext.Provider value={value}>{children}</ModernMotionContext.Provider>;
}

/** Optional so the reusable backdrop and arrival wrapper remain safe outside
 * the coordinated application flow, including Classic screens. */
export function useModernMotion(): ModernMotionState | null { return useContext(ModernMotionContext); }

/** Stagger ordinary content on the shared clock. Any subtree containing native
 * glass must set glass=true: its ancestor opacity stays exactly1 throughout. */
export function ModernMotionItem({ children, style, when = 'startup', order = 0, glass = false, distance = 20, range, ...props }: ViewProps & {
  children: ReactNode;
  when?: FinitePhase;
  order?: number;
  glass?: boolean;
  distance?: number;
  /** Optional interval on the shared progress, e.g. [.9,1] after the logo passes. */
  range?: readonly [number, number];
}) {
  const motion = useModernMotion();
  const progress = when === 'startup' ? motion?.startupProgress : motion?.arrivalProgress;
  const moving = motion?.enabled && (motion.phase === when || when === 'startup' && motion.phase === 'preparing');
  const defaultStart = Math.min(5, Math.max(0, order)) * .07;
  const requested = range?.every(Number.isFinite) ? range.map((value) => Math.max(0, Math.min(1, value))) : undefined;
  const [start, end] = requested && requested[0] < requested[1]
    ? requested : [defaultStart, Math.min(1, defaultStart + .65)];
  const transform = StyleSheet.flatten(style)?.transform;
  return <Animated.View {...props} style={[style, {
    opacity: glass || !moving || !progress ? 1 : progress.interpolate({
      inputRange: [start, end], outputRange: [0, 1], extrapolate: 'clamp',
    }),
    transform: [...(Array.isArray(transform) ? transform : []), { translateY: moving && progress
      ? progress.interpolate({ inputRange: [start, end], outputRange: [distance, 0], extrapolate: 'clamp' }) : 0 }],
  }]}>{children}</Animated.View>;
}
