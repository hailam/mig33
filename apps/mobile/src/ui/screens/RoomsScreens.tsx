import React, { useMemo, type ReactNode } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import type { RoomParticipant, RoomSummary } from '../../mig33/client';
import type { RoomCategoryView } from '../session/model';
import { useAppearance } from '../appearance';
import { MotionPressable } from '../motion';
import { V4Avatar, V4Button, V4Field, V4Icon } from '../v4/primitives';
import { v4Gradient } from '../v4/theme';
import { v3Colors } from '../v3/theme';
import { ParticipantOptionsButton, ScreenHeading } from './ChatActionsScreens';

export interface RoomsScreenProps {
  categories: readonly RoomCategoryView[];
  loading: boolean;
  action: string | null;
  onExpand: (id: number) => void;
  onMore: (id: number) => void;
  onJoin: (name: string) => void;
  onRoomMenu: (room: RoomSummary) => void;
  onRefresh: () => void;
  expandedCategoryIds: ReadonlySet<number>;
  onExpandedCategoryIdsChange: (ids: ReadonlySet<number>) => void;
  filter: string;
  onFilterChange: (text: string) => void;
  onSearch: () => void;
  renderRoomOptions?: (room: RoomSummary) => ReactNode;
}

/** v4's 714/701 category order and occupancy, using touch-sized rows.
 * v3 keeps its room icon, right-hand (current/capacity), and blue/white chrome;
 * presenting categories on a screen is an adaptation of its favorites overlay.
 * Filtering is local. The original remote Search/Create page flows are separate.
 */
export function RoomsScreen({ categories, loading, action, onExpand, onMore, onJoin, onRoomMenu, onRefresh,
  expandedCategoryIds, onExpandedCategoryIdsChange, filter, onFilterChange, onSearch, renderRoomOptions }: RoomsScreenProps) {
  const { modern, v3, theme } = useAppearance();
  const styles = useStyles();
  const expanded = expandedCategoryIds;
  const query = filter.trim().toLocaleLowerCase();
  const Row = modern ? MotionPressable : Pressable;
  function toggle(category: RoomCategoryView) {
    const opening = !expanded.has(category.id);
    const next = new Set(expanded);
    if (opening) next.add(category.id); else next.delete(category.id);
    onExpandedCategoryIdsChange(next);
    if (opening && !category.loaded && !category.loading) onExpand(category.id);
  }
  if (modern) return <ScrollView testID="rooms.screen" style={styles.body} contentContainerStyle={styles.content}
    keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag">
    <ScreenHeading title="Rooms" subtitle="Browse a category or search all rooms."
      action={<V4Button testID="rooms.search" label="Search" accessibilityLabel="Search all rooms" onPress={onSearch} />} />
    <View style={styles.modernFilterRow}>
      <V4Field testID="rooms.filter" variant="search" accessibilityLabel="Filter loaded rooms"
        accessibilityHint="Open a category to load its rooms. Use Search to search all rooms."
        placeholder="Filter loaded rooms" value={filter} onChangeText={onFilterChange} autoCapitalize="none"
        autoCorrect={false} clearButtonMode="while-editing" returnKeyType="done"
        containerStyle={styles.modernFilter} />
      <V4Button testID="rooms.refresh" variant="secondary" systemImage="arrow.clockwise" iconOnly
        label="Refresh" accessibilityLabel="Refresh room categories" style={styles.refreshIcon}
        busy={loading} disabled={Boolean(action)} onPress={onRefresh} />
    </View>
    {categories.map((category) => {
      const open = expanded.has(category.id);
      const rooms = category.rooms.filter((room) => room.name.toLocaleLowerCase().includes(query));
      return <View key={category.id} style={styles.section}>
        <MotionPressable testID={`rooms.category.${category.id}`} accessibilityRole="button"
          // RN0.88 Fabric omits the false expanded state from its AX value.
          accessibilityValue={!open ? { text: 'collapsed' } : undefined}
          accessibilityLabel={category.name} accessibilityState={{ expanded: open, busy: category.loading }}
          onPress={() => toggle(category)} style={({ pressed }) => [styles.modernCategory, pressed && styles.selected]}>
          <Text style={styles.modernCategoryName}>{category.name}</Text>
          {category.loading ? <ActivityIndicator size="small" color={theme.link} />
            : category.loaded && <Text style={styles.categoryCount}>{category.rooms.length}</Text>}
          <V4Icon name={open ? 'arrowDown' : 'arrowRight'} size={12} />
        </MotionPressable>
        {open && <View style={styles.roomList}>
          {rooms.map((room, index) => {
            const joining = action === `join:${room.name.trim().toLowerCase()}`;
            const detail = joining ? 'Joining…' : roomDescription(room);
            return <View key={room.name} style={[styles.modernRoomLine, index > 0 && styles.modernSeparator]}>
              <MotionPressable testID={`rooms.room.${room.name}`} accessibilityRole="button" disabled={Boolean(action)}
                accessibilityLabel={`${room.name}, ${detail}`} accessibilityHint="Join room. Room options are also available."
                accessibilityState={{ disabled: Boolean(action), busy: joining }}
                accessibilityActions={[{ name: 'showOptions', label: 'Room options' }]}
                onAccessibilityAction={(event) => { if (event.nativeEvent.actionName === 'showOptions') onRoomMenu(room); }}
                onPress={() => onJoin(room.name)} onLongPress={() => onRoomMenu(room)}
                style={({ pressed }) => [styles.room, pressed && styles.selected]}>
                <View style={styles.roomSymbol}>{joining ? <ActivityIndicator size="small" color={theme.link} />
                  : <V4Icon name="conversation" size={23} color={theme.link} />}</View>
                <View style={styles.roomIdentity}><Text style={styles.roomName}>{room.name}</Text>
                  <Text style={styles.roomDetail} accessibilityLiveRegion={joining ? 'polite' : 'none'}>{detail}</Text></View>
              </MotionPressable>
              <View style={styles.optionsSlot}>{renderRoomOptions ? renderRoomOptions(room) :
                <ParticipantOptionsButton testID={`rooms.more.${room.name}`} username={room.name} disabled={Boolean(action)} onPress={() => onRoomMenu(room)} />}</View>
            </View>;
          })}
          {rooms.length === 0 && <Text style={styles.empty}>{category.loading ? 'Loading rooms…' : !category.loaded ? 'Rooms have not loaded. Try again.'
            : query ? 'No matching rooms in this category.' : category.favorites ? 'No favorite rooms yet.' : 'No rooms in this category.'}</Text>}
          {category.loaded && category.footer !== '' && <View style={styles.modernFooter}>
            <V4Button testID={`rooms.category.more.${category.id}`} variant="secondary" label={category.footer}
              accessibilityLabel={`${category.footer}, ${category.name}`} busy={category.loading} disabled={Boolean(action)}
              onPress={() => onMore(category.id)} />
          </View>}
          {!category.loaded && !category.loading && <View style={styles.modernFooter}>
            <V4Button testID={`rooms.category.retry.${category.id}`} variant="secondary" label="Retry"
              accessibilityLabel={`Retry loading ${category.name}`} onPress={() => onExpand(category.id)} />
          </View>}
        </View>}
      </View>;
    })}
    {categories.length === 0 && <View style={styles.emptyState}>{loading && <ActivityIndicator color={theme.link} />}
      <Text style={styles.empty}>{loading ? 'Loading chat rooms…' : 'No room categories available.'}</Text></View>}
  </ScrollView>;
  return <ScrollView testID="rooms.screen" style={[styles.body, v3 && styles.v3Frame]}
    contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag">
    <View style={styles.introduction}>
      {modern && <View style={styles.heading}>
        <View style={styles.headingLine}><Text accessibilityRole="header" style={styles.headingTitle}>Chat Rooms</Text>
          <View style={styles.headingAccent} /></View>
        <Text style={styles.subtitle}>A place to meet. A conversation to join.</Text>
      </View>}
      <V4Field testID="rooms.filter" label="Filter rooms" accessibilityLabel="Filter rooms already loaded"
        placeholder={modern ? 'Find a room in your loaded categories' : undefined}
        value={filter} onChangeText={onFilterChange} autoCapitalize="none" autoCorrect={false} clearButtonMode="while-editing"
        returnKeyType="done" containerStyle={styles.filterField} />
      <View style={styles.filterHelp}>
        <Text style={styles.hint}>Filters rooms already loaded. Expand a category to load it.</Text>
        <V4Button testID="rooms.refresh" label="Refresh" busy={loading} onPress={onRefresh} style={styles.refreshButton} />
      </View>
    </View>
    {categories.map((category) => {
      const open = expanded.has(category.id);
      const rooms = category.rooms.filter((room) => room.name.toLocaleLowerCase().includes(query));
      return <View key={category.id} style={styles.section}>
        <Row testID={`rooms.category.${category.id}`} accessibilityRole="button"
          accessibilityValue={!open ? { text: 'collapsed' } : undefined}
          accessibilityLabel={category.name} accessibilityState={{ expanded: open, busy: category.loading }}
          onPress={() => toggle(category)} style={styles.category}>
          <V4Icon name={modern ? (open ? 'arrowDown' : 'arrowRight') : (open ? 'minus' : 'plus')}
            size={modern ? 12 : v3 ? 16 : 18} />
          <Text style={styles.categoryName}>{category.name}</Text>
          {category.loaded && <Text style={styles.categoryCount}>{category.rooms.length}</Text>}
          {category.loading && <ActivityIndicator size="small" color={theme.link} />}
        </Row>
        {open && <View style={styles.roomList}>
          {rooms.map((room) => <View key={room.name} style={styles.roomLine}>
            <Row testID={`rooms.room.${room.name}`} accessibilityRole="button" disabled={Boolean(action)}
              accessibilityLabel={`${room.name}${room.occupancy === undefined ? '' : `, ${room.occupancy} participants`}`}
              accessibilityHint="Join room. More actions are available."
              accessibilityState={{ disabled: Boolean(action) }}
              accessibilityActions={[{ name: 'showOptions', label: 'Room options' }]}
              onAccessibilityAction={(event) => { if (event.nativeEvent.actionName === 'showOptions') onRoomMenu(room); }}
              onPress={() => onJoin(room.name)} onLongPress={() => onRoomMenu(room)}
              style={({ pressed }) => [styles.room, pressed && !v3 && styles.selected]}>
              {({ pressed }) => <>
                <View style={styles.roomSymbol}><V4Icon name={modern ? 'conversation' : 'chat'} size={modern ? 24 : v3 ? 16 : 20} /></View>
                <View style={styles.roomIdentity}>
                  <Text numberOfLines={1} style={[styles.roomName, pressed && (v3 ? styles.v3SelectedLabel : styles.selectedText)]}>{room.name}</Text>
                  {modern && <Text style={styles.roomDetail}>{room.occupancy === undefined ? 'Join the conversation' :
                    `${room.occupancy} ${room.occupancy === 1 ? 'person' : 'people'} here`}</Text>}
                </View>
                {!modern && <Text style={[styles.occupancy, pressed && !v3 && styles.selectedText]}>{roomOccupancy(room)}</Text>}
              </>}
            </Row>
            {modern && <MotionPressable testID={`rooms.more.${room.name}`} accessibilityRole="button"
              accessibilityLabel={`Options for ${room.name}`} onPress={() => onRoomMenu(room)}
              disabled={Boolean(action)} accessibilityState={{ disabled: Boolean(action) }} style={styles.more}>
              <View pointerEvents="none" style={styles.moreDots}>{[0, 1, 2].map((dot) => <View key={dot} style={styles.moreDot} />)}</View>
            </MotionPressable>}
          </View>)}
          {rooms.length === 0 && <Text style={styles.empty}>
            {category.loading ? 'Loading rooms…' : !category.loaded ? 'Rooms have not loaded. Try again.' : query ?
              'No matching rooms in this category.' : category.favorites ? 'No favorite rooms yet.' : 'No rooms in this category.'}
          </Text>}
          {category.loaded && category.footer !== '' && <View style={styles.categoryFooter}>
            <V4Button testID={`rooms.category.more.${category.id}`} label={category.footer}
              accessibilityLabel={`${category.footer}, ${category.name}`} busy={category.loading}
              onPress={() => onMore(category.id)} style={styles.refreshButton} />
          </View>}
          {!category.loaded && !category.loading && <View style={styles.categoryFooter}>
            <V4Button testID={`rooms.category.retry.${category.id}`} label="Retry"
              accessibilityLabel={`Retry loading ${category.name}`} onPress={() => onExpand(category.id)} style={styles.refreshButton} />
          </View>}
        </View>}
      </View>;
    })}
    {categories.length === 0 && <View style={styles.emptyState}>
      {loading ? <ActivityIndicator color={theme.link} /> : <V4Icon name="conversation" size={36} />}
      <Text style={styles.empty}>{loading ? 'Loading chat rooms…' : 'No room categories available.'}</Text>
    </View>}
    {action !== null && <View style={styles.progress} accessibilityLiveRegion="polite">
      <ActivityIndicator size="small" color={theme.link} /><Text style={styles.hint}>Updating room…</Text>
    </View>}
  </ScrollView>;
}

export interface RoomParticipantsScreenProps {
  room: string;
  participants: readonly RoomParticipant[];
  loading: boolean;
  username: string;
  onRefresh: () => void;
  onSelect: (username: string) => void;
  onOptions?: (participant: RoomParticipant) => void;
  renderParticipantOptions?: (participant: RoomParticipant) => ReactNode;
}

/** 708 preserves server member order. Admin/muted badges are descriptive;
 * tapping opens private chat and does not imply a moderation request.
 */
export function RoomParticipantsScreen({ room, participants, loading, username, onRefresh, onSelect, onOptions, renderParticipantOptions }: RoomParticipantsScreenProps) {
  const { modern, v3, theme } = useAppearance();
  const styles = useStyles();
  const Row = modern ? MotionPressable : Pressable;
  return <ScrollView testID="participants.screen" style={[styles.body, v3 && styles.v3Frame]} contentContainerStyle={styles.content}>
    <View style={styles.introduction}>
      {modern && <ScreenHeading title="People" subtitle={room} />}
      <View style={styles.participantSummary}>
        <Text style={styles.hint}>{loading ? 'Refreshing participants…' : `${participants.length} ${participants.length === 1 ? 'participant' : 'participants'}`}</Text>
        <V4Button testID="participants.refresh" variant={modern ? 'secondary' : undefined}
          systemImage={modern ? 'arrow.clockwise' : undefined} iconOnly={modern}
          label="Refresh" accessibilityLabel={modern ? 'Refresh participants' : undefined} busy={loading} onPress={onRefresh}
          style={modern ? styles.refreshIcon : styles.refreshButton} />
      </View>
    </View>
    <View style={styles.roomList}>
      {participants.map((participant) => {
        const own = participant.username.toLocaleLowerCase() === username.toLocaleLowerCase();
        const detail = [own && 'You', participant.moderator && 'Admin', participant.muted && 'Muted'].filter(Boolean).join(' · ');
        return <View key={participant.username} style={styles.participantLine}>
        <Row testID={`participants.user.${participant.username}`} accessibilityRole="button"
          accessibilityLabel={`${participant.username}${detail ? `, ${detail}` : ''}`} accessibilityHint={own ? undefined : 'Open private chat'}
          accessibilityState={{ disabled: own }} disabled={own} onPress={() => onSelect(participant.username)}
          onLongPress={onOptions && !own ? () => onOptions(participant) : undefined}
          style={({ pressed }) => [styles.participant, pressed && !v3 && styles.selected]}>
          {({ pressed }) => <>
            {modern ? <V4Avatar size={44} label={participant.username} /> : <V4Icon name="person" size={v3 ? 16 : 20} />}
            <View style={styles.roomIdentity}>
              <Text numberOfLines={modern ? undefined : 1} style={[styles.roomName, pressed && (v3 ? styles.v3SelectedLabel : styles.selectedText)]}>{participant.username}</Text>
              {modern && detail !== '' && <Text style={styles.roomDetail}>{detail}</Text>}
            </View>
            {!modern && detail !== '' && <Text style={[styles.occupancy, pressed && !v3 && styles.selectedText]}>
              {own ? '(You)' : participant.moderator ? '(Admin)' : participant.muted ? '(Muted)' : ''}
            </Text>}
            {modern && !own && !onOptions && <V4Icon name="arrowRight" size={12} />}
          </>}
        </Row>
        {!own && (modern && renderParticipantOptions ? <View style={styles.optionsSlot}>{renderParticipantOptions(participant)}</View>
          : onOptions && <ParticipantOptionsButton username={participant.username} disabled={loading} onPress={() => onOptions(participant)} />)}
        </View>;
      })}
    </View>
    {participants.length === 0 && <View style={styles.emptyState}>
      {loading && <ActivityIndicator color={theme.link} />}
      <Text style={styles.empty}>{loading ? 'Loading participants…' : 'No participants to show.'}</Text>
    </View>}
  </ScrollView>;
}

function roomOccupancy(room: RoomSummary): string {
  if (room.occupancy !== undefined && room.capacity !== undefined) return `(${room.occupancy}/${room.capacity})`;
  if (room.occupancy !== undefined) return `(${room.occupancy})`;
  return '';
}

function roomDescription(room: RoomSummary): string {
  if (room.occupancy !== undefined && room.capacity !== undefined) return `${room.occupancy} of ${room.capacity} people`;
  if (room.occupancy !== undefined) return `${room.occupancy} ${room.occupancy === 1 ? 'person' : 'people'} here`;
  if (room.capacity !== undefined) return `Space for ${room.capacity} people`;
  return 'Join the conversation';
}

function useStyles() {
  const { modern, v3, theme, metrics, navigationInset } = useAppearance();
  return useMemo(() => StyleSheet.create({
    body: { flex: 1, backgroundColor: modern ? 'transparent' : theme.body },
    content: { paddingTop: modern ? navigationInset : 0, paddingBottom: modern ? 28 : 8, paddingHorizontal: modern ? metrics.pageInset : 0 },
    v3Frame: { borderWidth: 2, borderTopColor: v3Colors.selectionOutline, borderLeftColor: v3Colors.selectionOutline,
      borderBottomColor: v3Colors.contentBevel, borderRightColor: v3Colors.contentBevel },
    introduction: { paddingHorizontal: modern ? 0 : 6, paddingTop: modern ? 0 : 8 },
    heading: { paddingTop: 22, paddingBottom: 24 }, headingLine: { flexDirection: 'row', alignItems: 'baseline', gap: 6 },
    headingTitle: { color: theme.text, fontSize: 38, lineHeight: 45, fontWeight: '700', letterSpacing: -1.3 },
    headingAccent: { width: 7, height: 7, borderRadius: 4, backgroundColor: theme.focus },
    subtitle: { color: theme.secondaryText, fontSize: 14, lineHeight: 21, marginTop: 7 },
    filterField: { marginBottom: 2 }, filterHelp: { flexDirection: 'row', alignItems: 'center', gap: 12, paddingBottom: modern ? 18 : 8 },
    hint: { flex: 1, color: theme.secondaryText, fontSize: modern ? 12 : metrics.smallFont, lineHeight: 18 },
    refreshButton: { alignSelf: 'center', minWidth: modern ? 90 : 70 },
    refreshIcon: { alignSelf: 'center', width: 48, height: 48 },
    section: { marginBottom: modern ? 14 : 0 },
    category: { minHeight: 48, flexDirection: 'row', alignItems: 'center', gap: modern ? 10 : 4,
      paddingHorizontal: modern ? 0 : 5, borderTopWidth: modern || v3 ? 0 : 1, borderBottomWidth: modern || v3 ? 0 : 1,
      borderColor: theme.groupBorder, backgroundImage: modern || v3 ? undefined : v4Gradient(theme.groupTop, theme.groupBottom) },
    categoryName: { flex: 1, color: theme.groupText, fontSize: modern ? 13 : metrics.font, fontWeight: modern ? '600' : '400' },
    categoryCount: { color: theme.secondaryText, fontSize: 12 },
    roomList: { backgroundColor: modern ? theme.dialogBody : 'transparent', borderRadius: modern ? 20 : 0,
      paddingHorizontal: modern ? 14 : 0, overflow: 'hidden', ...(modern && { borderWidth: StyleSheet.hairlineWidth, borderColor: theme.groupBorder }) },
    roomLine: { flexDirection: 'row', alignItems: 'stretch', borderBottomWidth: modern ? StyleSheet.hairlineWidth : v3 ? 0 : 1,
      borderBottomColor: theme.groupBorder },
    room: { flex: 1, minWidth: 0, minHeight: modern ? 76 : 44, flexDirection: 'row', alignItems: 'center', gap: modern ? 12 : 5,
      paddingLeft: modern ? 0 : 8, paddingRight: modern ? 0 : 6, paddingVertical: modern ? 15 : 4 },
    roomSymbol: modern ? { width: 44, height: 44, borderRadius: 14, backgroundColor: theme.groupTop, alignItems: 'center', justifyContent: 'center' } : {},
    roomIdentity: { flex: 1, minWidth: 0 }, roomName: { color: theme.rowText, fontSize: metrics.font, fontWeight: modern ? '600' : '400',
      ...(modern && { lineHeight: 23 }),
      ...(v3 && { alignSelf: 'flex-start', borderWidth: 1, borderColor: 'transparent', paddingHorizontal: 2, maxWidth: '100%' }) },
    roomDetail: { color: theme.secondaryText, fontSize: modern ? 13 : 12, marginTop: modern ? 3 : 6, lineHeight: modern ? 19 : 17 },
    occupancy: { color: theme.secondaryText, fontSize: metrics.smallFont, paddingLeft: 3 },
    selected: { backgroundColor: theme.selected }, selectedText: { color: theme.selectedText },
    v3SelectedLabel: { backgroundColor: theme.selected, borderColor: theme.groupBorder, fontWeight: '700' },
    more: { width: 44, alignItems: 'center', justifyContent: 'center' },
    moreDots: { flexDirection: 'row', gap: 3 }, moreDot: { width: 3, height: 3, borderRadius: 2, backgroundColor: theme.secondaryText },
    categoryFooter: { paddingVertical: 4, alignItems: 'flex-end', paddingHorizontal: 6 },
    empty: { color: theme.secondaryText, fontSize: metrics.smallFont, lineHeight: 20, padding: modern ? 18 : 8 },
    emptyState: { paddingTop: 30, alignItems: 'center' },
    progress: { padding: 12, flexDirection: 'row', alignItems: 'center', gap: 10 },
    participantSummary: { flexDirection: 'row', gap: 12, alignItems: 'center', paddingBottom: modern ? 18 : 8 },
    participantLine: { flexDirection: 'row', alignItems: 'stretch' },
    participant: { flex: 1, minWidth: 0, flexDirection: 'row', alignItems: 'center', gap: modern ? 14 : 6,
      minHeight: modern ? 78 : 44, paddingVertical: modern ? 15 : 4, paddingHorizontal: modern ? 0 : 8,
      borderBottomWidth: modern ? StyleSheet.hairlineWidth : v3 ? 0 : 1, borderBottomColor: theme.groupBorder },
    modernFilterRow: { flexDirection: 'row', alignItems: 'center', gap: 8, marginBottom: 12 },
    modernFilter: { flex: 1, width: 'auto', marginBottom: 0 },
    modernCategory: { flexDirection: 'row', alignItems: 'center', minHeight: 48, paddingHorizontal: 3, gap: 10, borderRadius: 10 },
    modernCategoryName: { flex: 1, color: theme.groupText, fontSize: 15, lineHeight: 22, fontWeight: '600' },
    modernRoomLine: { flexDirection: 'row', alignItems: 'center' },
    modernSeparator: { borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: theme.groupBorder },
    modernFooter: { alignItems: 'flex-end', paddingVertical: 8, borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: theme.groupBorder },
    optionsSlot: { minWidth: 44, minHeight: 44, alignItems: 'center', justifyContent: 'center' },
  }), [modern, v3, theme, metrics, navigationInset]);
}
