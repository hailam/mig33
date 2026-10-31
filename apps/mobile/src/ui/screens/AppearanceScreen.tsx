import React, { useState } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import { useAppearance, type AppearanceMode } from '../appearance';
import { appearanceThemes } from '../appearanceTheme';
import { MotionPressable } from '../motion';
import { V4Icon } from '../v4/primitives';
import { ScreenHeading } from './ChatActionsScreens';
import { v4Gradient } from '../v4/theme';

const choices: { mode: AppearanceMode; title: string; description: string }[] = [
  { mode: 'modern', title: 'Modern', description: 'The colors and spirit of v4, with spacious controls and messages you can write right in the conversation.' },
  { mode: 'classic', title: 'Classic v4', description: 'Original gradients, compact lists and the separate Write Message screen, adapted for touch.' },
  { mode: 'classic-v3', title: 'Classic v3', description: 'The blue and white look of mig33 3.08, with slanted tabs, simple lists and gold focus outlines, adapted for touch.' },
];

export function AppearanceScreen() {
  const { mode, setMode, theme, metrics, modern } = useAppearance();
  if (modern) return <ScrollView testID="appearance.screen" style={{ backgroundColor: 'transparent' }}
    contentContainerStyle={{ paddingHorizontal: 20, paddingBottom: 24, gap: 12 }}>
    <ScreenHeading title="Make it yours" subtitle="Switch styles while your conversations stay open." />
    {choices.map((choice) => <AppearanceChoice key={choice.mode} choice={choice} selected={mode === choice.mode} onSelect={() => setMode(choice.mode)} />)}
    <Text style={{ color: theme.secondaryText, fontSize: 13, lineHeight: 20, paddingHorizontal: 4, paddingTop: 4 }}>
      Saved on this device. Modern follows your device’s light or dark appearance.
    </Text>
  </ScrollView>;
  return <ScrollView testID="appearance.screen" contentContainerStyle={{ padding: modern ? 20 : 8, gap: 16 }}>
    <Text style={{ color: theme.text, fontSize: metrics.font, lineHeight: metrics.lineHeight }}>
      Choose how mig33 looks. Your connection and conversations stay open when you switch.
    </Text>
    {choices.map((choice) => <Pressable key={choice.mode} testID={`appearance.${choice.mode}`}
      accessibilityRole="radio" accessibilityState={{ selected: mode === choice.mode, checked: mode === choice.mode }}
      accessibilityLabel={`${choice.title}. ${choice.description}`}
      onPress={() => setMode(choice.mode)}
      style={{ padding: 16, minHeight: 92, gap: 10, borderWidth: mode === choice.mode ? 2 : 1,
        borderColor: mode === choice.mode ? theme.link : theme.groupBorder,
        borderRadius: modern ? 16 : 0, backgroundColor: theme.dialogBody }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}>
        {mode === choice.mode ? <V4Icon name="check" size={20} color={theme.link} />
          : <View style={{ width: 20, height: 20, borderRadius: 10, borderWidth: 1, borderColor: theme.secondaryText }} />}
        <Text style={{ color: theme.text, fontSize: metrics.font + 1, fontWeight: '700', flex: 1 }}>{choice.title}</Text>
        {choice.mode === 'modern' && <Text style={{ color: theme.secondaryText, fontSize: metrics.smallFont }}>Default</Text>}
      </View>
      <Text style={{ color: theme.secondaryText, fontSize: metrics.font, lineHeight: metrics.lineHeight }}>{choice.description}</Text>
    </Pressable>)}
    <Text style={{ color: theme.secondaryText, fontSize: metrics.smallFont, lineHeight: metrics.lineHeight }}>
      Your choice is saved on this device.
    </Text>
  </ScrollView>;
}

function AppearanceChoice({ choice, selected, onSelect }: {
  choice: typeof choices[number]; selected: boolean; onSelect: () => void;
}) {
  const { theme } = useAppearance();
  const { fontScale } = useWindowDimensions();
  const [focused, setFocused] = useState(false);
  const description = choice.mode === 'modern' ? 'Liquid Glass controls, spacious lists and chat in place.'
    : choice.mode === 'classic' ? 'The original teal gradients and compact v4 layout.'
      : 'The blue tabs and simple lists of mig33 3.08.';
  return <MotionPressable testID={`appearance.${choice.mode}`} accessibilityRole="radio"
    accessibilityState={{ checked: selected, selected }} accessibilityLabel={`${choice.title}. ${description}`}
    onPress={onSelect} onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
    style={({ pressed }) => ({ borderRadius: 20, borderWidth: 2,
      borderColor: focused ? theme.focus : selected ? theme.link : theme.groupBorder,
      backgroundColor: pressed ? theme.selected : theme.dialogBody, padding: 14, gap: 14,
      flexDirection: fontScale > 1.4 ? 'column' : 'row', alignItems: fontScale > 1.4 ? 'stretch' : 'center' })}>
    <ThemePreview mode={choice.mode} />
    <View style={{ flex: fontScale > 1.4 ? undefined : 1, minWidth: 0, gap: 5 }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}>
        <Text style={{ flex: 1, color: theme.text, fontSize: 17, lineHeight: 24, fontWeight: '600' }}>{choice.title}</Text>
        <View style={{ width: 22, height: 22, borderRadius: 11, borderWidth: selected ? 0 : 1.5,
          borderColor: theme.secondaryText, backgroundColor: selected ? theme.link : 'transparent', alignItems: 'center', justifyContent: 'center' }}>
          {selected && <V4Icon name="check" size={14} color={theme.buttonText} />}
        </View>
      </View>
      <Text style={{ color: theme.secondaryText, fontSize: 14, lineHeight: 21 }}>{description}</Text>
    </View>
  </MotionPressable>;
}

/** A decorative preview of each theme, not another interactive glass layer. */
function ThemePreview({ mode }: { mode: AppearanceMode }) {
  const { theme } = useAppearance();
  const palette = mode === 'modern' ? theme : appearanceThemes[mode];
  const modern = mode === 'modern';
  return <View accessible={false} importantForAccessibility="no-hide-descendants" pointerEvents="none"
    style={{ width: 72, height: 78, borderRadius: modern ? 14 : 5, borderWidth: StyleSheet.hairlineWidth,
      borderColor: theme.border, padding: 6, backgroundColor: palette.body, overflow: 'hidden' }}>
    <View style={{ height: modern ? 13 : 9, borderRadius: modern ? 7 : 0, marginBottom: 8,
      backgroundColor: modern ? palette.selected : palette.headerBottom,
      backgroundImage: mode === 'classic' ? v4Gradient(palette.headerTop, palette.headerBottom) : undefined }} />
    {[0, 1, 2].map((row) => <View key={row} style={{ flexDirection: 'row', alignItems: 'center', gap: 5, marginBottom: 7 }}>
      <View style={{ width: 11, height: 11, borderRadius: modern ? 4 : 0, backgroundColor: modern ? palette.groupTop : palette.selected }} />
      <View style={{ height: 3, flex: 1, marginRight: row === 1 ? 7 : 0, borderRadius: modern ? 2 : 0, backgroundColor: palette.secondaryText, opacity: .6 }} />
    </View>)}
  </View>;
}
