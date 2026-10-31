import React, { useEffect, useRef, useState, type ReactNode } from 'react';
import { Keyboard, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { NativeAlert, NativeSheet, type NativeAlertAction } from './NativePresentation';
import { useAppearance } from './appearance';
import { NativeAlertCoordinator, retainSheetSnapshot } from './nativePresentationState';
import type { V4MenuItem } from './v4/primitives';

export interface OverlayAlert {
  key: string; title: string; message: string; actions?: readonly NativeAlertAction[]; onClose: () => void;
}
export interface OverlaySheet {
  key: string; title: string; content?: ReactNode; menu?: readonly V4MenuItem[];
  onClose: () => void; dismissible?: boolean;
}

/** One persistent SwiftUI sheet presenter. In particular, camera/library and
 * subsequent sheets start only after UIKit completes this sheet's dismissal. */
export function NativeOverlayLayer({ sheet, alert }: { sheet: OverlaySheet | null; alert: OverlayAlert | null }) {
  const [retained, setRetained] = useState<OverlaySheet | null>(null);
  const [presented, setPresented] = useState(false);
  const afterDismiss = useRef<(() => void) | null>(null);
  const latestSheet = useRef<OverlaySheet | null>(null);
  latestSheet.current = retainSheetSnapshot(latestSheet.current, retained, sheet);
  const visible = latestSheet.current;
  const coordinator = useRef(new NativeAlertCoordinator<OverlayAlert>()).current;
  coordinator.setDesired(alert);
  const [alertState, setAlertState] = useState(coordinator.state);
  useEffect(() => {
    if (coordinator.commit()) setAlertState(coordinator.state);
  }, [alert, alertState, coordinator]);
  useEffect(() => {
    if (!retained && sheet) { setRetained(sheet); setPresented(true); }
    else if (retained && sheet?.key !== retained.key) setPresented(false);
  }, [retained?.key, sheet?.key]);

  function closeSheet() {
    Keyboard.dismiss();
    setPresented(false);
    visible?.onClose();
  }
  function select(item: V4MenuItem) {
    if (item.disabled || !item.onPress) return;
    afterDismiss.current = item.onPress;
    closeSheet();
  }
  const activeAlert = alertState.alert;
  const generation = alertState.generation;
  const alertActions = activeAlert?.actions?.length ? activeAlert.actions
    : [{ label: 'OK', onPress: () => {}, testID: 'dialog.ok' }];
  const alertView = <NativeAlert presentationKey={String(generation)}
    isPresented={alertState.phase === 'presented' || alertState.phase === 'waiting-native'}
    title={activeAlert?.title ?? ''} message={activeAlert?.message}
    actions={alertActions.map((action) => ({ ...action, onPress: () => {
      const selected = coordinator.choose(generation);
      setAlertState(coordinator.state);
      if (selected) { selected.onClose(); action.onPress(); }
    } }))}
    onIsPresentedChange={(value) => {
      if (value) return;
      const closed = coordinator.nativeClosed(generation);
      setAlertState(coordinator.state);
      closed?.onClose();
    }} />;
  return <>
    <NativeSheet isPresented={presented} title={visible?.title} dismissible={visible?.dismissible}
      closeTestID={visible?.menu ? 'menu.close' : 'chat.sheet.close'}
      onIsPresentedChange={(value) => { if (!value) closeSheet(); }}
      onDismiss={() => {
        setPresented(false); setRetained(null);
        const action = afterDismiss.current; afterDismiss.current = null;
        // The native callback is completion, not an estimated animation delay.
        action?.();
      }}>
      {visible?.menu ? <MenuContents items={visible.menu} onSelect={select} /> : visible?.content}
      {retained && alertView}
    </NativeSheet>
    {!retained && alertView}
  </>;
}

function MenuContents({ items, onSelect }: { items: readonly V4MenuItem[]; onSelect: (item: V4MenuItem) => void }) {
  const { theme, dark } = useAppearance();
  return <ScrollView testID="menu.items" keyboardShouldPersistTaps="handled" contentContainerStyle={styles.menu}>
    {items.map((item, index) => <View key={`${item.id}:${index}`}>
      {item.separator && <View style={{ height: StyleSheet.hairlineWidth, backgroundColor: theme.border, marginVertical: 8 }} />}
      <Pressable testID={item.testID} accessibilityRole="button" accessibilityLabel={item.label}
        accessibilityState={{ disabled: !!item.disabled || !item.onPress }} disabled={item.disabled || !item.onPress}
        onPress={() => onSelect(item)} style={({ pressed }) => [styles.row, pressed && { backgroundColor: theme.selected }]}>
        <Text style={[styles.label, { color: item.disabled ? theme.muted : item.destructive ? dark ? '#ffb3a9' : '#a12524' : theme.text }]}>{item.label}</Text>
        {item.children?.length ? <Text style={{ color: theme.secondaryText }}>›</Text> : null}
      </Pressable>
    </View>)}
  </ScrollView>;
}
const styles = StyleSheet.create({
  menu: { paddingHorizontal: 12, paddingBottom: 16 },
  row: { minHeight: 50, paddingHorizontal: 12, paddingVertical: 13, borderRadius: 12, flexDirection: 'row', alignItems: 'center', gap: 12 },
  label: { flex: 1, fontSize: 16, lineHeight: 24 },
});
