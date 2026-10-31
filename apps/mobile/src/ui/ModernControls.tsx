import type { StyleProp, ViewStyle } from 'react-native';

export interface ModernButtonProps {
  label: string;
  onPress: () => void;
  variant?: 'primary' | 'secondary' | 'plain';
  disabled?: boolean;
  busy?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
  accessibilityLabel?: string;
  systemImage?: string;
  iconOnly?: boolean;
  selected?: boolean;
  /** Keep true for the entire unread/highlight episode; pulse its tint, not
   * the native button style, so a phase change cannot replace a press target. */
  highlighted?: boolean;
  /** Supply `none` for read tabs too: their native background and press target
   * stay mounted while the decorative unread field changes. */
  unreadPhase?: 'none' | 'on' | 'off' | 'steady';
  tintColor?: string;
  foregroundColor?: string;
  controlSize?: 'regular' | 'large' | 'extraLarge';
}

export interface ModernToggleProps {
  label: string;
  checked: boolean;
  onChange?: (checked: boolean) => void;
  disabled?: boolean;
  testID?: string;
  style?: StyleProp<ViewStyle>;
}

export interface ModernChoiceProps {
  label?: string;
  accessibilityLabel?: string;
  hideLabel?: boolean;
  value: string;
  options: readonly { value: string; label: string; testID?: string; disabled?: boolean }[];
  onChange: (value: string) => void;
  disabled?: boolean;
  testID?: string;
  style?: StyleProp<ViewStyle>;
  variant?: 'menu' | 'segmented';
}

export interface ModernSymbolProps {
  name: string;
  size: number;
  color?: string;
}

// Metro uses the iOS implementation. Callers retain their existing controls
// elsewhere, without importing SwiftUI into another platform's bundle.
export const supportsModernControls: boolean = false;
export function ModernButton(_props: ModernButtonProps) { return null; }
export function ModernToggle(_props: ModernToggleProps) { return null; }
export function ModernChoice(_props: ModernChoiceProps) { return null; }
export function ModernSymbol(_props: ModernSymbolProps) { return null; }
