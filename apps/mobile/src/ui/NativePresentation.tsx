import type { ReactNode } from 'react';
import type { V4MenuItem } from './v4/primitives';

export interface NativeMenuAnchorProps {
  items: readonly V4MenuItem[];
  title?: string;
  children?: ReactNode;
  testID?: string;
  accessibilityLabel?: string;
  systemImage?: string;
  variant?: 'glass' | 'plain';
  disabled?: boolean;
}

export interface NativeSheetProps {
  isPresented: boolean;
  onIsPresentedChange: (presented: boolean) => void;
  /** Called after the native dismissal finishes; use this for presentation handoffs. */
  onDismiss: () => void;
  title?: string;
  children: ReactNode;
  detents?: readonly ('medium' | 'large')[];
  dismissible?: boolean;
  testID?: string;
  closeTestID?: string;
}

export interface NativeAlertAction {
  label: string;
  onPress: () => void;
  testID?: string;
  destructive?: boolean;
  cancel?: boolean;
}

export interface NativeAlertProps {
  /** Recreate only the native alert attachment for a new logical presentation. */
  presentationKey?: string;
  isPresented: boolean;
  title: string;
  message?: string;
  actions?: readonly NativeAlertAction[];
  onIsPresentedChange: (presented: boolean) => void;
}

// Metro selects NativePresentation.ios.tsx on iOS. Keeping this module free of
// SwiftUI imports lets other platforms use the existing application fallback.
export const supportsNativePresentation: boolean = false;
export function NativeMenuAnchor(_props: NativeMenuAnchorProps) { return null; }
export function NativeSheet(_props: NativeSheetProps) { return null; }
export function NativeAlert(_props: NativeAlertProps) { return null; }
