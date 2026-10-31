import React, { type ComponentProps } from 'react';
import { StyleSheet, View, useWindowDimensions } from 'react-native';
import {
  Alert, BottomSheet, Button, Divider, Group, Host, HStack, Label, Menu, Rectangle,
  RNHostView, Section, Spacer, Text, Toggle, VStack,
} from '@expo/ui/swift-ui';
import {
  accessibilityHidden, accessibilityLabel, buttonBorderShape, buttonStyle, controlSize, disabled, font, labelStyle,
  foregroundStyle, frame, interactiveDismissDisabled, padding,
  presentationBackground, presentationDetents, presentationDragIndicator, presentationSizing,
} from '@expo/ui/swift-ui/modifiers';
import { useAppearance } from './appearance';
import type { V4IconName, V4MenuItem } from './v4/primitives';
import type { NativeAlertProps, NativeMenuAnchorProps, NativeSheetProps } from './NativePresentation';

export type { NativeAlertAction, NativeAlertProps, NativeMenuAnchorProps, NativeSheetProps } from './NativePresentation';
export const supportsNativePresentation: boolean = true;

type SystemImage = ComponentProps<typeof Button>['systemImage'];
const menuSymbols: Partial<Record<V4IconName, SystemImage>> = {
  friends: 'person.2', chat: 'bubble.left.and.bubble.right', conversation: 'bubble.left',
  page: 'doc.text', mail: 'envelope', person: 'person.crop.circle',
  plus: 'plus', minus: 'minus', close: 'xmark', check: 'checkmark', menu: 'ellipsis',
};

function menuEntries(items: readonly V4MenuItem[]): React.ReactNode {
  return items.map((item) => <React.Fragment key={item.id}>
    {item.separator && <Divider />}
    {item.children?.length ? <Menu label={item.label} systemImage={item.icon ? menuSymbols[item.icon] : undefined}
      testID={item.testID} modifiers={[disabled(Boolean(item.disabled))]}>
      {menuEntries(item.children)}
    </Menu> : item.checked !== undefined ? <Toggle label={item.label} testID={item.testID} isOn={item.checked}
      onIsOnChange={(next) => { if (!item.disabled) item.onCheckedChange?.(next); }}
      modifiers={[disabled(Boolean(item.disabled || !item.onCheckedChange))]} /> : <Button label={item.label} testID={item.testID}
      systemImage={item.icon ? menuSymbols[item.icon] : undefined}
      role={item.destructive ? 'destructive' : 'default'} onPress={item.onPress}
      modifiers={[disabled(Boolean(item.disabled || !item.onPress))]} />}
  </React.Fragment>);
}

/** A real SwiftUI menu owns its label's hit target, placement and dismissal. */
export function NativeMenuAnchor({ items, title, children, testID, accessibilityLabel: label,
  systemImage, variant = 'plain', disabled: inactive = false }: NativeMenuAnchorProps) {
  const { dark, theme } = useAppearance();
  const name = label ?? title ?? 'Menu';
  const labelMinimum = variant === 'plain' ? 44 : 30;
  return <Host matchContents colorScheme={dark ? 'dark' : 'light'} seedColor={theme.link} style={styles.menuHost}>
    <Menu testID={testID} modifiers={[buttonStyle(variant), controlSize('regular'), disabled(inactive),
      accessibilityLabel(name), ...(systemImage ? [buttonBorderShape('circle'), frame({ minWidth: 44, minHeight: 44 })] : [])]}
      label={systemImage ? <Label title={name} systemImage={systemImage as SystemImage} modifiers={[
        labelStyle('iconOnly'), frame({ minWidth: labelMinimum, minHeight: labelMinimum }),
      ]} /> : <RNHostView matchContents><View pointerEvents="none" accessible={false} style={styles.menuLabel}>{children}</View></RNHostView>}>
      {title ? <Section title={title}>{menuEntries(items)}</Section> : menuEntries(items)}
    </Menu>
  </Host>;
}

/** Keep this host mounted when closed. SwiftUI owns the detents, safe area,
 * keyboard avoidance, Reduce Motion behavior and dismissal completion. */
export function NativeSheet({ isPresented, onIsPresentedChange, onDismiss, title, children,
  detents = ['medium', 'large'], dismissible = true, testID = 'sheet.native', closeTestID = 'sheet.close',
}: NativeSheetProps) {
  const { width } = useWindowDimensions();
  const { dark, theme, reduceTransparency, increaseContrast } = useAppearance();
  return <Host style={[styles.presentationHost, { width }]} pointerEvents="none"
    colorScheme={dark ? 'dark' : 'light'} seedColor={theme.link}>
    <BottomSheet isPresented={isPresented} onIsPresentedChange={onIsPresentedChange} onDismiss={onDismiss}>
      <Group modifiers={[
        presentationDetents(detents.length ? [...detents] : ['large']),
        presentationDragIndicator(dismissible ? 'visible' : 'hidden'),
        presentationSizing('form'), interactiveDismissDisabled(!dismissible),
        // The normal background is the native adaptive sheet material. Keep
        // the same presenter when accessibility requests an opaque surface.
        ...(reduceTransparency || increaseContrast ? [presentationBackground(theme.dialogBody)] : []),
      ]}>
        <VStack spacing={0} modifiers={[frame({ maxWidth: Infinity, maxHeight: Infinity, alignment: 'topLeading' })]}>
          <HStack spacing={16} modifiers={[padding({ top: 18, leading: 20, bottom: 12, trailing: 20 })]}>
            {title && <Text modifiers={[font({ textStyle: 'headline' })]}>{title}</Text>}
            <Spacer />
            <Button testID={closeTestID} role="cancel" onPress={() => onIsPresentedChange(false)}
              modifiers={[buttonStyle('glass'), buttonBorderShape('circle'), controlSize('regular'),
                accessibilityLabel('Close'), frame({ minWidth: 44, minHeight: 44 }), disabled(!dismissible)]}>
              <Label title="Close" systemImage="xmark" modifiers={[
                labelStyle('iconOnly'), frame({ minWidth: 30, minHeight: 30 }),
              ]} />
            </Button>
          </HStack>
          <RNHostView>
            {/* This receives the presented window's actual width and remaining
                height, including changes as the user drags between detents.
                No matchContents height: long forms must be able to scroll. */}
            <View testID={testID} style={styles.sheetContent}>{children}</View>
          </RNHostView>
        </VStack>
      </Group>
    </BottomSheet>
  </Host>;
}

/** SwiftUI Alert is controlled, but has no dismissal-completed callback.
 * Application orchestration must not treat onIsPresentedChange as one. */
export function NativeAlert({ presentationKey, isPresented, title, message, actions, onIsPresentedChange }: NativeAlertProps) {
  const { dark, theme } = useAppearance();
  const buttons = actions?.length ? actions : [{ label: 'OK', onPress: () => {}, testID: 'dialog.ok' }];
  return <Host style={styles.alertHost} pointerEvents="none" colorScheme={dark ? 'dark' : 'light'} seedColor={theme.link}>
    <Alert key={presentationKey} title={title} isPresented={isPresented} onIsPresentedChange={onIsPresentedChange}>
      <Alert.Trigger>
        {/* A persistent real view attaches .alert to its hosting controller.
            It is clear and inaccessible, not hidden or conditionally removed. */}
        <Rectangle modifiers={[foregroundStyle('clear'), frame({ width: 1, height: 1 }), accessibilityHidden()]} />
      </Alert.Trigger>
      {message && <Alert.Message><Text>{message}</Text></Alert.Message>}
      <Alert.Actions>
        {buttons.map((action, index) => <Button key={`${index}:${action.label}`} label={action.label} testID={action.testID}
          role={action.cancel ? 'cancel' : action.destructive ? 'destructive' : 'default'}
          // SwiftUI dismisses its own binding on action. Forward only that
          // native event; a synthetic false here races the actual dispatcher.
          onPress={action.onPress} />)}
      </Alert.Actions>
    </Alert>
  </Host>;
}

const styles = StyleSheet.create({
  menuHost: { minWidth: 44, minHeight: 44, flexShrink: 0 },
  menuLabel: { minWidth: 44, minHeight: 44, alignItems: 'center', justifyContent: 'center' },
  presentationHost: { position: 'absolute', top: 0, left: 0, height: 1 },
  alertHost: { position: 'absolute', top: 0, left: 0, width: 1, height: 1 },
  sheetContent: { flexGrow: 1, height: 0, paddingBottom: 16 },
});
