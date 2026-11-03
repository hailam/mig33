import React, { useEffect, useId, useRef, useState, type ComponentProps } from 'react';
import { Platform, useWindowDimensions, View } from 'react-native';
import { Background, Button, GlassEffectContainer, Host, HStack, Label, Menu, Namespace, ScrollView, Spacer, useNativeState } from '@expo/ui/swift-ui';
import {
  accessibilityAddTraits, accessibilityElement, accessibilityLabel, animation, Animation, buttonBorderShape,
  buttonStyle, clipped, contentShape, controlSize, fixedSize, font, foregroundStyle, frame, glassEffect, glassEffectId,
  id, labelStyle, lineLimit, offset, onGeometryChange, onScrollPhaseChange, opacity, padding, scaleEffect,
  scrollClipDisabled, scrollPosition, scrollTargetLayout, shapes, tint,
} from '@expo/ui/swift-ui/modifiers';
import { isGlassEffectAPIAvailable, isLiquidGlassAvailable } from 'expo-glass-effect';
import { useAppearance } from './appearance';
import { modernControlColors } from './appearanceTheme';
import { NativeMenuItems } from './NativePresentation';
import { useModernMotion } from './ModernMotion';
import { NativeUnreadAmberField } from './ModernControls.ios';
import type { ModernNavigationProps } from './ModernNavigation';

export type { ModernNavigationConversation, ModernNavigationMenu, ModernNavigationProps, ModernNavigationTab } from './ModernNavigation';
// Drag-phase callbacks require iOS18 to cancel pending automatic reveals.
// Older systems keep the existing React Native strip and selected-tab reveal.
export const supportsModernNavigation: boolean = Number.parseInt(String(Platform.Version), 10) >= 18;
type SystemImage = ComponentProps<typeof Button>['systemImage'];
type Bounds = { x: number; y: number; width: number; height: number };
type Seed = Readonly<Record<string, { x: number; scale: number }>>;

/** Destinations and the actual Menu share one native material/sampling tree.
 * Their permanent native controls form the arrival; no decorative duplicate
 * plate or React press target stands in for the final navigation.
 * https://developer.apple.com/documentation/swiftui/applying-liquid-glass-to-custom-views
 */
export function ModernNavigation({ tabs, selected, onSelect, compact, menu, conversations = [], arriving = false }: ModernNavigationProps) {
  const { dark, theme, reduceMotion, reduceTransparency, increaseContrast } = useAppearance();
  const motion = useModernMotion();
  const { fontScale } = useWindowDimensions();
  const namespace = useId();
  const glass = isGlassEffectAPIAvailable() && isLiquidGlassAvailable() && !reduceTransparency;
  const canAnimate = glass && !reduceMotion && arriving &&
    (!motion || motion.enabled && motion.phase === 'arriving');
  const colors = modernControlColors(dark, increaseContrast);
  const [expanded, setExpanded] = useState(!arriving || !canAnimate);
  const [seed, setSeed] = useState<Seed | null>(null);
  const measured = useRef(new Map<string, Bounds>());
  const played = useRef(false);
  const pendingFrame = useRef<number | null>(null);
  const [conversationSize, setConversationSize] = useState({ fontScale, height: 0 });
  const [conversationWidth, setConversationWidth] = useState(0);
  const selectedConversation = conversations.some((tab) => tab.id === selected) ? selected : null;
  const conversationPosition = useNativeState<string | null>(null);
  const laidOutConversations = useRef(new Set<string>());
  const viewportWidth = useRef(0);
  const pendingReveal = useRef<string | null>(null);
  const revealFrame = useRef<number | null>(null);
  const currentSelection = useRef(selectedConversation);
  currentSelection.current = selectedConversation;
  const conversationIDs = JSON.stringify(conversations.map((tab) => tab.id));
  useEffect(() => {
    const present = new Set<string>(JSON.parse(conversationIDs));
    for (const target of laidOutConversations.current) {
      if (!present.has(target)) laidOutConversations.current.delete(target);
    }
  }, [conversationIDs]);
  function cancelReveal() {
    pendingReveal.current = null;
    if (revealFrame.current !== null) cancelAnimationFrame(revealFrame.current);
    revealFrame.current = null;
  }
  function revealAfterLayout() {
    const target = pendingReveal.current;
    if (!target || viewportWidth.current <= 0 || !laidOutConversations.current.has(target) || revealFrame.current !== null) return;
    // A new tab's React commit precedes its native scroll target registration.
    // Wait for that target's real geometry, then request one reveal. Repeated
    // geometry/pulse events never rewrite the binding after this request.
    revealFrame.current = requestAnimationFrame(() => {
      revealFrame.current = null;
      if (pendingReveal.current !== target || currentSelection.current !== target) return;
      pendingReveal.current = null;
      conversationPosition.set(target);
    });
  }
  useEffect(() => {
    cancelReveal();
    pendingReveal.current = selectedConversation;
    revealAfterLayout();
    return cancelReveal;
  }, [selectedConversation, conversationPosition]);
  const keys = [...tabs.map((tab) => `tab:${tab.id}`), ...(menu ? ['menu'] : [])];
  // Conversation buttons use Dynamic Type body text, a30pt minimum label and the
  // regular native button's vertical padding. The strip adds8pt. Seed a finite
  // slot from those metrics, then let its real content measurement grow it.
  const conversationHeight = Math.max(56, Math.ceil(21 * fontScale + 22),
    conversationSize.fontScale === fontScale ? conversationSize.height : 0);
  const maximumConversationWidth = Math.max(44, Math.min(188, conversationWidth > 0 ? conversationWidth - 6 : 188));

  function prepareGeometry() {
    if (!arriving || !canAnimate || played.current || seed) return;
    const row = measured.current.get('row');
    if (!row || keys.some((key) => !measured.current.has(key))) return;
    const center = row.x + row.width / 2;
    const initial: Record<string, { x: number; scale: number }> = {};
    for (const key of keys) {
      const bounds = measured.current.get(key)!;
      initial[key] = { x: center - (bounds.x + bounds.width / 2), scale: Math.min(1, 46 / bounds.width) };
    }
    setSeed(initial);
  }

  function measure(key: string, bounds: Bounds) {
    if (played.current || bounds.width <= 0 || bounds.height <= 0) return;
    measured.current.set(key, bounds);
    prepareGeometry();
  }

  useEffect(() => {
    if (!arriving || !canAnimate) {
      if (arriving) played.current = true;
      setExpanded(true);
    } else if (!played.current) {
      setExpanded(false);
      prepareGeometry();
      if (seed && pendingFrame.current === null) {
        // Commit the measured compact source pose before starting the native
        // spring. This follows layout; it is not an authentication delay.
        pendingFrame.current = requestAnimationFrame(() => {
          pendingFrame.current = null;
          played.current = true;
          setExpanded(true);
        });
      }
    }
    return () => {
      if (pendingFrame.current !== null) cancelAnimationFrame(pendingFrame.current);
      pendingFrame.current = null;
    };
  }, [arriving, canAnimate, seed]);

  const joined = arriving && canAnimate && !expanded;
  const preparing = joined && !seed;
  // Include eligibility in the native animation value: a background/settled
  // provider update must also cancel an already-running spring or label delay.
  const animationState = (joined ? 1 : 0) + (canAnimate ? 2 : 0);
  // A late-mounted arrival may already have its final allocation. Seed the
  // compact pose immediately; only the outward formation runs the spring.
  const expanding = canAnimate && !joined;
  const inkVisible = !canAnimate;
  const transition = animation(expanding ? Animation.spring({ duration: .65, bounce: .08 })
    : Animation.linear({ duration: 0 }), animationState);
  function material(key: string, circle: boolean, active = false) {
    return [
      ...(glass ? [glassEffect({ glass: { variant: preparing ? 'identity' : 'regular', interactive: !reduceMotion,
        ...(active ? { tint: colors.glassTint } : {}) }, shape: circle ? 'circle' : 'capsule' }),
        glassEffectId(key, namespace)] : []),
      // The HStack keeps its final allocation. Only the real native control
      // transforms: all controls initially share one46pt source at row center.
      scaleEffect({ x: joined ? seed?.[key]?.scale ?? 1 : 1, y: 1 }),
      offset({ x: joined ? seed?.[key]?.x ?? 0 : 0, y: 0 }), transition,
    ];
  }
  function label(name: string, symbol: string, circle: boolean) {
    return <Label title={name} systemImage={symbol as SystemImage} modifiers={[
      labelStyle(circle ? 'iconOnly' : 'titleAndIcon'), font({ textStyle: 'body', weight: 'semibold' }),
      padding({ horizontal: circle ? 0 : glass ? 12 : 0 }),
      ...(circle ? [frame({ width: glass ? 46 : 30 })] : []),
      frame({ minHeight: glass ? 46 : 30 }),
      // Plain controls must expose the full padded label, not just visible ink.
      ...(glass ? [contentShape(circle ? shapes.circle() : shapes.capsule(), ['interaction', 'accessibility'])] : []),
      // Never carry readable labels through overlapping glass. The arrival
      // boundary settles geometry before revealing ink in its final capsule.
      // Only lettering fades; glass and all ancestors stay at opacity1.
      opacity(inkVisible ? 1 : 0), animation(inkVisible && !reduceMotion
        ? Animation.easeOut({ duration: .18 }) : Animation.linear({ duration: 0 }), inkVisible),
    ]} />;
  }

  const navigation = <HStack spacing={8} modifiers={[frame({ maxWidth: Infinity, minHeight: 48 }),
            onGeometryChange((bounds) => measure('row', bounds))]}>
            <HStack spacing={6} modifiers={[fixedSize({ horizontal: true, vertical: true })]}>
              {tabs.map((tab) => {
                const key = `tab:${tab.id}`;
                const active = tab.id === selected;
                const ink = active ? glass ? colors.selectedInk : theme.selectedText : theme.text;
                return <HStack key={tab.id} spacing={0} modifiers={[onGeometryChange((bounds) => measure(key, bounds))]}>
                  <Button testID={`tab.${tab.id}`} onPress={() => onSelect(tab.id)} modifiers={[
                    buttonStyle(glass ? 'plain' : active ? 'borderedProminent' : 'bordered'),
                    buttonBorderShape(compact ? 'circle' : 'capsule'), controlSize('regular'),
                    tint(active && !glass ? theme.selected : theme.link), foregroundStyle(ink),
                    accessibilityLabel(tab.label), ...(active ? [accessibilityAddTraits(['isSelected'])] : []),
                    ...material(key, compact, active),
                  ]}>{label(tab.label, tab.systemImage, compact)}</Button>
                </HStack>;
              })}
            </HStack>
            {conversations.length > 0 ? <HStack spacing={0} modifiers={[
              frame({ maxWidth: Infinity, minWidth: 0 }), frame({ height: conversationHeight }), clipped(),
              onGeometryChange(({ width }) => {
                if (Number.isFinite(width) && width > 0) {
                  viewportWidth.current = width;
                  setConversationWidth((before) => Math.abs(before - width) < .5 ? before : width);
                  revealAfterLayout();
                }
              }),
            ]}>
              {/* Collect moving glass inside the scroll view's clip. An outer
                  material container can composite it past the viewport even when
                  the logical scroll bounds and hit targets remain correct. */}
              <ScrollView testID="tabs.items" axes="horizontal" showsIndicators modifiers={[
                frame({ maxWidth: Infinity, maxHeight: Infinity }), scrollClipDisabled(false), clipped(),
                scrollPosition(conversationPosition, { anchor: 'center' }),
                onScrollPhaseChange((phase) => {
                  if (phase === 'tracking' || phase === 'interacting') cancelReveal();
                }),
              ]}>
                <GlassEffectContainer spacing={4}>
                <HStack spacing={8} modifiers={[scrollTargetLayout(), padding({ horizontal: 3, vertical: 4 }),
                  // Keep the material's collected extent on a container, not
                  // the first button's accessible frame.
                  accessibilityElement('contain'),
                  onGeometryChange(({ height }) => {
                    if (Number.isFinite(height) && height > 0) setConversationSize((before) =>
                      before.fontScale === fontScale && Math.abs(before.height - height) < .5 ? before : { fontScale, height });
                  }),
                ]}>
                  {conversations.map((tab) => {
                    const active = tab.id === selected;
                    const unread = !active && tab.unreadPhase !== 'none';
                    const solidUnread = !glass && unread;
                    const controlTint = solidUnread ? tab.unreadPhase === 'off' ? '#f4c68f' : '#f5a34e'
                      : active ? glass ? colors.glassTint : theme.selected : theme.text;
                    const ink = solidUnread ? '#38200d' : active ? glass ? colors.selectedInk : theme.selectedText : theme.text;
                    return <Background key={tab.id} modifiers={[onGeometryChange(({ width, height }) => {
                      if (width > 0 && height > 0) {
                        laidOutConversations.current.add(tab.id);
                        revealAfterLayout();
                      }
                    }), id(tab.id)]}>
                      <Button testID={`tab.${tab.id}`} onPress={() => onSelect(tab.id)} modifiers={[
                        buttonStyle(glass ? active ? 'glassProminent' : 'glass' : active || unread ? 'borderedProminent' : 'bordered'),
                        buttonBorderShape('capsule'), controlSize('regular'), tint(controlTint), foregroundStyle(ink),
                        frame({ minWidth: 44, maxWidth: maximumConversationWidth, minHeight: 44 }),
                        contentShape(shapes.capsule(), ['interaction', 'accessibility']),
                        accessibilityLabel(`${tab.label}${unread ? ', unread messages' : ''}`),
                        ...(active ? [accessibilityAddTraits(['isSelected'])] : []),
                      ]}>
                        <Label title={tab.label} systemImage={tab.systemImage as SystemImage} modifiers={[
                          labelStyle('titleAndIcon'), font({ textStyle: 'body', weight: 'semibold' }), lineLimit(1),
                          frame({ minHeight: 30, maxWidth: maximumConversationWidth - 28 }),
                        ]} />
                      </Button>
                      <Background.Content><NativeUnreadAmberField phase={glass && !active ? tab.unreadPhase : 'none'} circle={false} /></Background.Content>
                    </Background>;
                  })}
                </HStack>
                </GlassEffectContainer>
              </ScrollView>
            </HStack> : <Spacer minLength={0} />}
            {menu && <HStack spacing={0} modifiers={[fixedSize({ horizontal: true, vertical: true }),
              onGeometryChange((bounds) => measure('menu', bounds))]}>
              <Menu testID="command.menu" label={label('Open menu', 'ellipsis', true)} modifiers={[
                buttonStyle(glass ? 'plain' : 'bordered'), buttonBorderShape('circle'), controlSize('regular'),
                foregroundStyle(theme.link), tint(theme.link), accessibilityLabel('Open menu'), ...material('menu', true),
              ]}><NativeMenuItems items={menu.items} title={menu.title} /></Menu>
            </HStack>}
          </HStack>;

  return <View accessible={false} testID={glass ? 'glass.native' : 'glass.solid'}
    style={{ flex: 1, minHeight: 48, justifyContent: 'center' }}>
    <Host matchContents={{ vertical: true }} alignment="leading" style={{ alignSelf: 'stretch' }}
      colorScheme={dark ? 'dark' : 'light'} seedColor={theme.link}>
      <Namespace id={namespace}>
        {/* Arrival has no open conversations. Its fixed controls share one
            container; the scrolling branch owns its material inside its clip. */}
        {conversations.length === 0 ? <GlassEffectContainer spacing={4}>{navigation}</GlassEffectContainer> : navigation}
      </Namespace>
    </Host>
  </View>;
}
