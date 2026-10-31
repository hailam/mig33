interface Identified { key: string; }
type AlertPhase = 'idle' | 'presented' | 'waiting-native' | 'closed';
export interface AlertPresentation<T> {
  alert: T | null;
  generation: number;
  phase: AlertPhase;
}

/** React commits `closed` before commit() advances to the next alert. This is
 * a binding transition, not an assertion that a native animation has ended. */
export class NativeAlertCoordinator<T extends Identified> {
  state: AlertPresentation<T> = { alert: null, generation: 0, phase: 'idle' };
  private desired: T | null = null;
  private dismissedKey: string | null = null;

  setDesired(alert: T | null) { this.desired = alert; }

  /** Called in an effect after the current presentation state was committed. */
  commit(): boolean {
    const { alert, phase, generation } = this.state;
    if (phase === 'presented' && !this.desired) {
      this.dismissedKey = alert!.key;
      this.state = { alert, phase: 'closed', generation };
      return true;
    }
    if (phase !== 'idle' && phase !== 'closed') return false;
    const next = this.desired?.key !== this.dismissedKey ? this.desired : null;
    if (next) {
      this.state = { alert: next, phase: 'presented', generation: generation + 1 };
      return true;
    }
    if (phase === 'closed') {
      this.state = { alert: null, phase: 'idle', generation };
      return true;
    }
    return false;
  }

  /** The control itself dismisses SwiftUI's alert. Keep its prop true until
   * SwiftUI reports the actual false binding; otherwise Expo suppresses that
   * event because the prop already equals the new native value. */
  choose(generation: number): T | null {
    if (generation !== this.state.generation || this.state.phase !== 'presented') return null;
    const alert = this.state.alert!;
    this.state = { ...this.state, phase: 'waiting-native' };
    // A superseded episode must not run an old action or clear its successor.
    return this.desired?.key === alert.key ? alert : null;
  }

  nativeClosed(generation: number): T | null {
    if (generation !== this.state.generation || !['presented', 'waiting-native'].includes(this.state.phase)) return null;
    const alert = this.state.alert!;
    const close = this.state.phase === 'presented' && this.desired?.key === alert.key ? alert : null;
    this.dismissedKey = alert.key;
    this.state = { ...this.state, phase: 'closed' };
    return close;
  }
}

/** Save current field values/callbacks while open, then retain that final
 * snapshot while the parent removes or replaces the sheet during dismissal. */
export function retainSheetSnapshot<T extends Identified>(previous: T | null, active: T | null, incoming: T | null): T | null {
  if (!active) return null;
  if (incoming?.key === active.key) return incoming;
  return previous?.key === active.key ? previous : active;
}
