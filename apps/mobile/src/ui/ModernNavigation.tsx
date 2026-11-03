import type { V4MenuItem } from './v4/primitives';

export interface ModernNavigationTab {
  id: string;
  label: string;
  systemImage: string;
}

export interface ModernNavigationConversation extends ModernNavigationTab {
  unreadPhase: 'none' | 'on' | 'off' | 'steady';
}

export interface ModernNavigationMenu {
  title?: string;
  items: readonly V4MenuItem[];
}

export interface ModernNavigationProps {
  tabs: readonly ModernNavigationTab[];
  selected: string;
  onSelect: (id: string) => void;
  compact: boolean;
  menu?: ModernNavigationMenu;
  /** Native scroll targets between the fixed destinations and Menu. */
  conversations?: readonly ModernNavigationConversation[];
  /** A newly authenticated session, never an ordinary tab change. */
  arriving?: boolean;
}

// SwiftUI stays out of other platform bundles; those callers keep their
// existing themed destinations until their native presentation is available.
export const supportsModernNavigation: boolean = false;
export function ModernNavigation(_props: ModernNavigationProps) { return null; }
