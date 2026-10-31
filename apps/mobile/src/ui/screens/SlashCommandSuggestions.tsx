import React, { useMemo } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useAppearance } from '../appearance';
import { GlassSurface } from '../glass';
import { MotionPressable } from '../motion';
import type { SlashSuggestion } from '../slashCommands';
import { V4Button } from '../v4/primitives';

export interface SlashCommandSuggestionsProps {
  commands: readonly SlashSuggestion[];
  onSelect: (id: string) => void;
  onDismiss: () => void;
  disabled?: boolean;
  maxHeight?: number;
}

/** Inline assistance only: choosing a row edits the composer's draft. The
 * caller retains keyboard focus and decides when to send the original text.
 */
export function SlashCommandSuggestions({ commands, onSelect, onDismiss, disabled = false,
  maxHeight = 220 }: SlashCommandSuggestionsProps) {
  const { modern, theme, metrics } = useAppearance();
  const styles = useMemo(() => StyleSheet.create({
    panel: { maxHeight, flexShrink: 1, borderRadius: modern ? 22 : 0, overflow: 'hidden' },
    solid: { backgroundColor: theme.dialogBody, borderWidth: 1, borderColor: theme.border },
    content: { maxHeight, flexShrink: 1 },
    header: { minHeight: 44, paddingLeft: modern ? 16 : 6, paddingRight: modern ? 6 : 4,
      flexDirection: 'row', alignItems: 'center', gap: 8, borderBottomWidth: StyleSheet.hairlineWidth,
      borderBottomColor: theme.border },
    title: { flex: 1, minWidth: 0, color: theme.text, fontSize: metrics.font, fontWeight: '600' },
    count: { color: theme.secondaryText, fontSize: metrics.smallFont },
    close: { width: modern ? 44 : 72, minHeight: 44 },
    scroll: { flexGrow: 0, flexShrink: 1 },
    list: { paddingHorizontal: modern ? 6 : 0, paddingBottom: modern ? 6 : 0 },
    row: { minHeight: 44, paddingHorizontal: modern ? 10 : 6, paddingVertical: modern ? 10 : 6,
      borderRadius: modern ? 14 : 0, gap: 3 },
    pressed: { backgroundColor: theme.selected },
    name: { color: theme.link, fontSize: metrics.font, fontWeight: '600' },
    description: { color: theme.text, fontSize: metrics.smallFont },
    usage: { color: theme.secondaryText, fontSize: metrics.smallFont, fontFamily: 'monospace' },
    requirement: { color: theme.secondaryText, fontSize: metrics.smallFont },
    selected: { color: theme.selectedText },
    disabled: { color: theme.muted },
    empty: { color: theme.secondaryText, fontSize: metrics.font, padding: modern ? 16 : 8 },
  }), [modern, theme, metrics, maxHeight]);
  const Row = modern ? MotionPressable : Pressable;
  // One stable native child keeps UIKit's glass content mount independent of
  // the number of filtered rows. Rows and Close do not add another glass layer.
  const content = <View testID="composer.commands" collapsable={false} style={styles.content}>
    <View style={styles.header}>
      <Text accessibilityRole="header" style={styles.title}>Commands</Text>
      <Text accessibilityLabel={`${commands.length} ${commands.length === 1 ? 'command' : 'commands'}`} style={styles.count}>
        {commands.length}
      </Text>
      <V4Button testID="composer.commands.close" label="Close" accessibilityLabel="Close commands"
        variant="plain" systemImage="xmark" iconOnly={modern} style={styles.close} onPress={onDismiss} />
    </View>
    <ScrollView style={styles.scroll} contentContainerStyle={styles.list}
      keyboardShouldPersistTaps="always" keyboardDismissMode="none" showsVerticalScrollIndicator>
      {commands.map((command) => {
        const inactive = disabled || !!command.disabledReason;
        return <Row key={command.id} testID={`composer.command.${command.id}`}
        accessibilityRole="button" accessibilityLabel={`/${command.command}. ${command.description}. ${command.usage}${command.requirement ? `. ${command.requirement}` : ''}`}
        accessibilityHint={command.disabledReason ?? 'Insert command into message'} accessibilityState={{ disabled: inactive }} disabled={inactive}
        onPress={() => onSelect(command.id)} style={({ pressed }) => [styles.row, pressed && styles.pressed]}>
        {({ pressed }) => <>
          <Text style={[styles.name, pressed && styles.selected, inactive && styles.disabled]}>/{command.command}</Text>
          <Text style={[styles.description, pressed && styles.selected, inactive && styles.disabled]}>{command.description}</Text>
          <Text style={[styles.usage, pressed && styles.selected, inactive && styles.disabled]}>{command.usage}</Text>
          {command.requirement && <Text style={[styles.requirement, pressed && styles.selected, inactive && styles.disabled]}>
            {command.requirement}
          </Text>}
          {command.disabledReason && <Text style={styles.requirement}>{command.disabledReason}</Text>}
        </>}
      </Row>;
      })}
      {commands.length === 0 && <Text style={styles.empty}>No matching commands</Text>}
    </ScrollView>
  </View>;
  return modern ? <GlassSurface style={styles.panel}>{content}</GlassSurface>
    : <View style={[styles.panel, styles.solid]}>{content}</View>;
}
