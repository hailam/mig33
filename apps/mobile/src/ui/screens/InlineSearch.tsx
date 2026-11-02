import React, { useMemo, type ReactNode } from 'react';
import { ActivityIndicator, Keyboard, StyleSheet, Text, View } from 'react-native';
import type { OriginalSearchResult } from '../../mig33/original-search';
import { useAppearance } from '../appearance';
import { MotionPressable } from '../motion';
import { V4Avatar, V4Button, V4Field, V4Icon } from '../v4/primitives';

/** Search stays in the list. The keyboard and icon submit the same original
 * server search; clearing restores the list's existing browsing state. */
export function InlineSearchField({ kind, value, onChange, onSubmit, onClear, active = !!value }: {
  kind: 'people' | 'rooms'; value: string; onChange: (value: string) => void;
  onSubmit: () => void; onClear: () => void;
  active?: boolean;
}) {
  const styles = useStyles();
  const label = kind === 'people' ? 'Search people' : 'Search rooms';
  function submit() { Keyboard.dismiss(); if (active) onSubmit(); }
  return <View style={styles.searchRow}>
    <V4Field testID={`search.${kind}.query`} variant="search" accessibilityLabel={label}
      placeholder={kind === 'people' ? 'Name or username' : 'Room name or description'}
      value={value} onChangeText={onChange} onSubmitEditing={submit} returnKeyType="search"
      submitBehavior="submit" autoCapitalize="none" autoCorrect={false}
      containerStyle={styles.field} />
    <V4Button testID={`search.${kind}.submit`} label={label} accessibilityLabel={label}
      systemImage="magnifyingglass" iconOnly variant="secondary" disabled={!active}
      onPress={submit} style={styles.icon} />
    {(active || !!value) && <V4Button testID={`search.${kind}.clear`} label="Clear search" accessibilityLabel="Clear search"
      systemImage="xmark" iconOnly variant="secondary" onPress={onClear} style={styles.icon} />}
  </View>;
}

export function InlineSearchResults({ kind, query, submittedQuery, loading, searched, error, results,
  onRetry, onAction, renderOptions, currentRequest = true, disabled = false, notice = '' }: {
  kind: 'people' | 'rooms'; query: string; submittedQuery: string; loading: boolean; searched: boolean;
  error: string; results: readonly OriginalSearchResult[]; onRetry: () => void;
  onAction: (href: string) => void; renderOptions?: (result: OriginalSearchResult) => ReactNode;
  currentRequest?: boolean;
  disabled?: boolean;
  notice?: string;
}) {
  const { theme } = useAppearance();
  const styles = useStyles();
  const current = currentRequest && submittedQuery === query.trim();
  const pending = loading || !current || !searched;
  if (current && error) return <View style={styles.status}>
    <Text accessibilityRole="alert" style={styles.detail}>{error}</Text>
    <V4Button testID={`search.${kind}.retry`} label="Retry search" accessibilityLabel="Retry search"
      systemImage="arrow.clockwise" iconOnly variant="secondary" onPress={onRetry} style={styles.icon} />
  </View>;
  if (pending) return <View testID={`search.${kind}.loading`} style={styles.status} accessibilityLiveRegion="polite">
    <ActivityIndicator color={theme.link} /><Text style={styles.detail}>Searching…</Text>
  </View>;
  return <View testID={`search.${kind}.results`}>
    <Text accessibilityRole="header" accessibilityLiveRegion="polite" style={styles.summary}>
      {results.length ? `${results.length} ${results.length === 1 ? 'result' : 'results'}`
        : kind === 'people' ? 'No people found' : 'No rooms found'}
    </Text>
    {!!notice && <Text testID={`search.${kind}.notice`} style={styles.detail}>{notice}</Text>}
    {results.length === 0 && <Text style={styles.detail}>Try another name.</Text>}
    <View style={results.length ? styles.list : undefined}>
      {results.map((result, index) => {
        const primary = result.actions.find((action) => /^mig33:(privateChat|joinChatroom|joinGroupChatroom)\(/.test(action.href)) ?? result.actions[0];
        return <View key={result.key} style={[styles.result, index > 0 && styles.separator]}>
          <MotionPressable testID={kind === 'rooms' ? `rooms.room.${result.title}` : `search.${kind}.result.${result.key}`} accessibilityRole="button"
            accessibilityLabel={[result.title, result.detail].filter(Boolean).join(', ')}
            accessibilityHint={primary?.label} disabled={disabled || !primary}
            accessibilityState={{ disabled: disabled || !primary }}
            onPress={() => { if (primary) { Keyboard.dismiss(); onAction(primary.href); } }}
            style={({ pressed }) => [styles.target, pressed && { backgroundColor: theme.selected }]}>
            {kind === 'people' ? <V4Avatar size={42} label={result.title} />
              : <View style={styles.symbol}><V4Icon name="conversation" size={23} color={theme.link} /></View>}
            <View style={styles.identity}><Text style={styles.name}>{result.title}</Text>
              {!!result.detail && <Text style={styles.detail}>{result.detail}</Text>}</View>
          </MotionPressable>
          {renderOptions ? <View style={styles.options}>{renderOptions(result)}</View>
            : <View>{result.actions.filter((action) => action !== primary).map((action) =>
              <V4Button key={action.href} label={action.label} disabled={disabled} onPress={() => onAction(action.href)} />)}</View>}
        </View>;
      })}
    </View>
  </View>;
}

function useStyles() {
  const { theme } = useAppearance();
  return useMemo(() => StyleSheet.create({
    searchRow: { flexDirection: 'row', alignItems: 'flex-start', gap: 8, marginBottom: 16 },
    field: { flex: 1, minWidth: 0, marginBottom: 0 }, icon: { width: 48, height: 48 },
    status: { minHeight: 80, paddingVertical: 20, flexDirection: 'row', alignItems: 'center', gap: 12 },
    summary: { color: theme.secondaryText, fontSize: 14, marginBottom: 12 },
    list: { borderRadius: 20, overflow: 'hidden', backgroundColor: theme.dialogBody,
      borderWidth: StyleSheet.hairlineWidth, borderColor: theme.groupBorder },
    result: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: 4 },
    separator: { borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: theme.groupBorder },
    target: { flex: 1, minWidth: 0, minHeight: 76, flexDirection: 'row', alignItems: 'center',
      gap: 12, padding: 12, borderRadius: 14 },
    identity: { flex: 1, minWidth: 0 }, name: { color: theme.text, fontSize: 16, lineHeight: 23, fontWeight: '600' },
    detail: { color: theme.secondaryText, fontSize: 14, lineHeight: 21, flexShrink: 1 },
    symbol: { width: 42, height: 42, alignItems: 'center', justifyContent: 'center' },
    options: { minWidth: 44, minHeight: 44, justifyContent: 'center' },
  }), [theme]);
}
