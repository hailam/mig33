import React, { useMemo, type ReactNode } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import type { Contact, ContactGroup } from '../../mig33/client';
import { contactSections, isAvailable, presenceLabel } from '../session/model';
import { useAppearance } from '../appearance';
import { MotionPressable } from '../motion';
import { V4Avatar, V4Icon, V4Logo, type V4IconName } from '../v4/primitives';
import { v4Gradient } from '../v4/theme';
import { v3Colors } from '../v3/theme';
import { ScreenHeading } from './ChatActionsScreens';

function presenceIcon(presence: number): V4IconName {
  return presence === 1 ? 'online' : presence === 3 ? 'busy' : presence === 4 ? 'away' : 'offline';
}

export function FriendsScreen({ username, presence, ownStatus = '', contacts, groups, hideOffline, selected, onContact, onSelf, onRooms,
  collapsedGroupIds, onCollapsedGroupIdsChange, renderContactOptions, renderSelfOptions }: {
  username: string; presence: number; ownStatus?: string; contacts: readonly Contact[]; groups: readonly ContactGroup[];
  hideOffline: boolean; selected: number | 'self'; onContact: (contact: Contact, menu: boolean) => void;
  onSelf: () => void;
  onRooms: () => void;
  collapsedGroupIds: ReadonlySet<number>; onCollapsedGroupIdsChange: (ids: ReadonlySet<number>) => void;
  renderContactOptions?: (contact: Contact) => ReactNode; renderSelfOptions?: () => ReactNode;
}) {
  const { modern, v3 } = useAppearance();
  const styles = useStyles();
  const collapsed = collapsedGroupIds;
  const sections = contactSections(groups, contacts);
  const Row = modern ? MotionPressable : Pressable;
  const online = contacts.filter((contact) => isAvailable(contact.presence)).length;
  function toggle(id: number) {
    const next = new Set(collapsed); if (next.has(id)) next.delete(id); else next.add(id);
    onCollapsedGroupIdsChange(next);
  }
  if (v3) return <View style={[styles.body, styles.v3Frame]}>
    <View pointerEvents="none" style={styles.v3Wallpaper}><V4Logo width={118} /></View>
    <ScrollView testID="friends.screen" style={styles.v3Scroll} keyboardShouldPersistTaps="handled"
      contentContainerStyle={styles.v3Content}>
      {/* Original r constructor inserts Chat Rooms first; Mail is conditional
          on a supplied mail URL, which this presentation slice does not have. */}
      <Pressable testID="rooms.open" style={styles.v3Group} accessibilityRole="button" accessibilityLabel="Chat Rooms" onPress={onRooms}>
        {({ pressed }) => <><V4Icon name="chat" size={16} /><Text style={[styles.v3Name, styles.v3Label, pressed && styles.v3Selected]}>Chat Rooms</Text></>}
      </Pressable>
      {sections.map((section) => <View key={section.id}>
        <Pressable testID={`friends.group.${section.id}`} accessibilityRole="button"
          accessibilityLabel={section.name} disabled={section.contacts.length === 0}
          accessibilityState={{ expanded: section.contacts.length > 0 && !collapsed.has(section.id), disabled: section.contacts.length === 0 }}
          accessibilityValue={section.contacts.length === 0 || collapsed.has(section.id) ? { text: 'collapsed' } : undefined}
          onPress={() => toggle(section.id)} style={styles.v3Group}>
          {({ pressed }) => <>
            <V4Icon name={collapsed.has(section.id) ? 'plus' : 'minus'} size={16} />
            <Text style={[styles.v3GroupLabel, styles.v3Label, pressed && styles.v3Selected]} numberOfLines={1}>{section.name}</Text>
          </>}
        </Pressable>
        {!collapsed.has(section.id) && section.contacts.filter((contact) => !hideOffline || isAvailable(contact.presence)).map((contact) => (
          <Pressable key={contact.id} testID={`friends.contact.${contact.username}`} accessibilityRole="button"
            accessibilityLabel={`${contact.display || contact.username}, ${presenceLabel(contact.presence)}`}
            onPress={() => onContact(contact, false)} onLongPress={() => onContact(contact, true)} style={styles.v3Contact}>
            {({ pressed }) => <>
              <V4Icon name={presenceIcon(contact.presence)} size={16} />
              <Text style={[styles.v3Name, styles.v3Label, (pressed || selected === contact.id) && styles.v3Selected]} numberOfLines={1}>
                {contact.display || contact.username}
              </Text>
            </>}
          </Pressable>
        ))}
      </View>)}
    </ScrollView>
  </View>;
  if (modern) return <ScrollView testID="friends.screen" style={styles.body} contentContainerStyle={styles.content}
    keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag">
    <ScreenHeading title="Friends" subtitle={`${online} online · ${contacts.length} ${contacts.length === 1 ? 'friend' : 'friends'}`} />
    <View style={styles.modernSelf}>
      <View accessible accessibilityLabel={`${username}, ${presenceLabel(presence)}${ownStatus ? `, ${ownStatus}` : ''}`}
        style={styles.modernSelfIdentity}>
        <V4Avatar presence={presence} label={username} size={42} />
        <View style={styles.selfText}><Text style={styles.selfName}>{username}</Text>
          <Text style={styles.modernDetail}>{ownStatus || `You · ${presenceLabel(presence)}`}</Text></View>
      </View>
      <View style={styles.optionsSlot}>{renderSelfOptions ? renderSelfOptions() :
        <ContactOptions label="Your account options" testID="friends.self" onPress={onSelf} />}</View>
    </View>
    {sections.map((section) => {
      const visible = section.contacts.filter((contact) => !hideOffline || isAvailable(contact.presence));
      return <View key={section.id} style={styles.section}>
        <MotionPressable testID={`friends.group.${section.id}`} accessibilityRole="button"
          accessibilityLabel={`${section.name}, ${section.online} online, ${section.contacts.length} friends`}
          // RN0.88 Fabric announces expanded=true but omits expanded=false.
          // Supply only the missing state so VoiceOver does not repeat either.
          accessibilityValue={collapsed.has(section.id) ? { text: 'collapsed' } : undefined}
          accessibilityState={{ expanded: !collapsed.has(section.id) }} onPress={() => toggle(section.id)}
          style={({ pressed }) => [styles.modernGroup, pressed && styles.contactPressed]}>
          <Text style={styles.modernGroupName}>{section.name}</Text>
          <Text style={styles.groupCount}>{section.online}/{section.contacts.length}</Text>
          <V4Icon name={collapsed.has(section.id) ? 'arrowRight' : 'arrowDown'} size={12} />
        </MotionPressable>
        {!collapsed.has(section.id) && <View style={styles.modernList}>
          {visible.map((contact, index) => <View key={contact.id} style={[styles.modernContactLine,
            index > 0 && styles.modernSeparator, selected === contact.id && styles.contactPressed]}>
            <MotionPressable testID={`friends.contact.${contact.username}`} accessibilityRole="button"
              accessibilityLabel={`${contact.display || contact.username}, ${presenceLabel(contact.presence)}${contact.status ? `, ${contact.status}` : ''}`}
              accessibilityHint={isAvailable(contact.presence) ? 'Open conversation. Contact options are also available.' : 'Open contact options.'}
              accessibilityState={{ selected: selected === contact.id }}
              accessibilityActions={[{ name: 'showOptions', label: 'Contact options' }]}
              onAccessibilityAction={(event) => { if (event.nativeEvent.actionName === 'showOptions') onContact(contact, true); }}
              onPress={() => onContact(contact, false)} onLongPress={() => onContact(contact, true)}
              style={({ pressed }) => [styles.modernContact, pressed && styles.contactPressed]}>
              <V4Avatar size={42} presence={contact.presence} label={contact.display || contact.username} />
              <View style={styles.selfText}><Text style={styles.modernName}>{contact.display || contact.username}</Text>
                <Text style={styles.modernDetail} numberOfLines={2}>{presenceLabel(contact.presence)}{contact.status ? ` · ${contact.status}` : ''}</Text>
              </View>
            </MotionPressable>
            <View style={styles.optionsSlot}>{renderContactOptions ? renderContactOptions(contact) :
              <ContactOptions label={`Options for ${contact.display || contact.username}`} testID={`friends.options.${contact.username}`}
                onPress={() => onContact(contact, true)} />}</View>
          </View>)}
          {visible.length === 0 && <Text style={styles.empty}>{hideOffline && section.contacts.length > 0 ? 'No friends online' : 'No friends in this group'}</Text>}
        </View>}
      </View>;
    })}
    {sections.length === 0 && <Text style={styles.empty}>No friends yet. Add someone from the menu to start a conversation.</Text>}
  </ScrollView>;
  return <ScrollView testID="friends.screen" style={styles.body} contentContainerStyle={styles.content}
    keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag">
    {modern && <View style={styles.heading}>
      <View style={styles.headingLine}><Text accessibilityRole="header" style={styles.headingTitle}>Friends</Text><View style={styles.headingAccent} /></View>
      <Text style={styles.headingSubtitle}>{online} online · {contacts.length} {contacts.length === 1 ? 'friend' : 'friends'}</Text>
    </View>}
    <Row accessibilityRole="button" accessibilityLabel={`${username}, ${presenceLabel(presence)}`}
      onPress={onSelf} style={[styles.self, !modern && selected !== 'self' && styles.selfUnselected]} testID="friends.self">
      <V4Avatar presence={presence} label={username} size={modern ? 38 : undefined} />
      <View style={styles.selfText}>
        <View style={styles.identity}>{!modern && <V4Icon name={presenceIcon(presence)} size={17} />}
          <Text style={[styles.selfName, !modern && selected !== 'self' && styles.selfTextUnselected]} numberOfLines={1}>{username}</Text></View>
        <Text style={[styles.selfStatus, !modern && selected !== 'self' && styles.selfTextUnselected]} numberOfLines={1}>
          {ownStatus || (modern ? `You · ${presenceLabel(presence)}` : '<Enter your status message>')}
        </Text>
      </View>
      {!modern && <View style={styles.avatarStrip} accessible={false}>
        <V4Avatar size={27} /><V4Avatar size={27} /><V4Avatar size={27} />
      </View>}
      {modern && <V4Icon name="arrowRight" size={12} />}
    </Row>
    {!modern && <View style={styles.mail} accessibilityLabel="Email unavailable" accessibilityState={{ disabled: true }}>
      <V4Icon name="mail" size={18} /><Text style={styles.mailText}>Email</Text>
    </View>}
    {sections.map((section) => <View key={section.id} style={styles.section}>
      <Row accessibilityRole="button" accessibilityState={{ expanded: !collapsed.has(section.id) }}
        accessibilityValue={collapsed.has(section.id) ? { text: 'collapsed' } : undefined}
        accessibilityLabel={`${section.name}, ${section.online} online, ${section.contacts.length} friends`}
        onPress={() => toggle(section.id)}
        style={styles.group} testID={`friends.group.${section.id}`}>
        <V4Icon name={modern ? (collapsed.has(section.id) ? 'arrowRight' : 'arrowDown') : (collapsed.has(section.id) ? 'plus' : 'minus')} size={modern ? 12 : 18} />
        <Text style={styles.groupText}>{section.name}{!modern && ` (${section.online}/${section.contacts.length})`}</Text>
        {modern && <Text style={styles.groupCount}>{section.online}/{section.contacts.length}</Text>}
      </Row>
      {!collapsed.has(section.id) && section.contacts.filter((contact) => !hideOffline || isAvailable(contact.presence)).map((contact) => (
        <Row accessibilityRole="button" key={contact.id} testID={`friends.contact.${contact.username}`}
          accessibilityLabel={`${contact.display || contact.username}, ${presenceLabel(contact.presence)}`}
          onPress={() => onContact(contact, false)} onLongPress={() => onContact(contact, true)}
          style={({ pressed }) => [styles.contact, (pressed || selected === contact.id) && styles.contactPressed]}>
          {({ pressed }) => <>
            {modern ? <V4Avatar size={46} presence={contact.presence} label={contact.display || contact.username} /> : <V4Icon name={presenceIcon(contact.presence)} size={20} />}
            {/* au's compact row appends literal " -" and t.h on the same line.
                A second line belongs to its separate expanded-selection state. */}
            {modern ? <View style={styles.contactText}>
              <View style={styles.nameLine}>
                <Text style={styles.name} numberOfLines={1}>{contact.display || contact.username}</Text>
                <Text style={styles.presence}>{presenceLabel(contact.presence)}</Text>
              </View>
              <Text style={styles.status} numberOfLines={1}>{contact.status || `@${contact.username}`}</Text>
            </View> : <Text style={[styles.name, styles.contactText, (pressed || selected === contact.id) && styles.selectedText]}
              numberOfLines={1}>
              {contact.display || contact.username}
              {contact.status !== '' && <Text style={[styles.status, (pressed || selected === contact.id) && styles.selectedText]}>
                {` -${contact.status}`}
              </Text>}
            </Text>}
          </>}
        </Row>
      ))}
      {modern && !collapsed.has(section.id) && !section.contacts.some((contact) => !hideOffline || isAvailable(contact.presence)) &&
        <Text style={styles.empty}>{hideOffline && section.contacts.length > 0 ? 'No friends online' : 'No friends in this group'}</Text>}
    </View>)}
    {sections.length === 0 && <View style={styles.group}><V4Icon name="minus" size={18} />
      <Text style={styles.groupText}>Friends (0/0)</Text></View>}
  </ScrollView>;
}

function ContactOptions({ label, testID, onPress }: { label: string; testID: string; onPress: () => void }) {
  const { theme } = useAppearance();
  return <MotionPressable testID={testID} accessibilityRole="button" accessibilityLabel={label} onPress={onPress}
    style={({ pressed }) => ({ minWidth: 44, minHeight: 44, borderRadius: 22, alignItems: 'center', justifyContent: 'center',
      backgroundColor: pressed ? theme.selected : 'transparent' })}>
    <View accessible={false} pointerEvents="none" style={{ flexDirection: 'row', gap: 3 }}>
      {[0, 1, 2].map((dot) => <View key={dot} style={{ width: 3, height: 3, borderRadius: 2, backgroundColor: theme.link }} />)}
    </View>
  </MotionPressable>;
}

function useStyles() {
  const { theme: v4, metrics: v4Metrics, modern, navigationInset } = useAppearance();
  return useMemo(() => StyleSheet.create({
  body: { flex: 1, backgroundColor: modern ? 'transparent' : v4.body }, content: { paddingTop: modern ? navigationInset : 0,
    paddingBottom: modern ? 24 : 8, paddingHorizontal: modern ? v4Metrics.pageInset : 0 },
  v3Frame: { borderWidth: 2, borderTopColor: v3Colors.selectionOutline, borderLeftColor: v3Colors.selectionOutline,
    borderBottomColor: v3Colors.contentBevel, borderRightColor: v3Colors.contentBevel },
  v3Wallpaper: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center', transform: [{ translateX: -3 }] },
  v3Scroll: { flex: 1, backgroundColor: 'transparent' }, v3Content: { paddingVertical: 2, paddingBottom: 8 },
  v3Group: { flexDirection: 'row', alignItems: 'center', minHeight: 44, gap: 3, paddingLeft: 3, paddingRight: 4 },
  v3Contact: { flexDirection: 'row', alignItems: 'center', minHeight: 44, gap: 3, paddingLeft: 8, paddingRight: 4 },
  v3Label: { paddingHorizontal: 2, borderWidth: 1, borderColor: 'transparent', flexShrink: 1 },
  v3GroupLabel: { color: v4.groupText, fontSize: v4Metrics.font }, v3Name: { color: v4.rowText, fontSize: v4Metrics.font },
  v3Selected: { backgroundColor: v4.selected, borderColor: v4.groupBorder, fontWeight: '700' },
  v3Unavailable: { color: v4.muted, fontSize: v4Metrics.font, paddingHorizontal: 3 },
  heading: { paddingTop: 22, paddingBottom: 22 }, headingLine: { flexDirection: 'row', alignItems: 'baseline', gap: 6 },
  headingTitle: { color: v4.text, fontSize: 38, lineHeight: 45, fontWeight: '700', letterSpacing: -1.3 },
  headingAccent: { width: 7, height: 7, borderRadius: 4, backgroundColor: v4.focus },
  headingSubtitle: { color: v4.secondaryText, fontSize: 14, marginTop: 7 },
  self: { flexDirection: 'row', alignItems: 'center', gap: modern ? 12 : 7, minHeight: modern ? 68 : 62, padding: modern ? 0 : 5,
    borderTopWidth: modern ? 0 : 3, borderTopColor: v4.border, borderBottomWidth: modern ? 0 : 1, borderBottomColor: v4.groupBorder,
    backgroundColor: modern ? 'transparent' : v4.selected, backgroundImage: modern ? undefined : v4Gradient(v4.selfTop, v4.selfBottom),
    ...(modern && { paddingBottom: 18, marginBottom: 8, borderBottomWidth: 1 }) },
  selfText: { flex: 1, minWidth: 0 }, identity: { flexDirection: 'row', alignItems: 'center', gap: 3 },
  selfUnselected: { backgroundColor: v4.subtle, backgroundImage: undefined }, selfTextUnselected: { color: v4.text },
  selfName: { color: modern ? v4.text : v4.selectedText, fontSize: modern ? 15 : v4Metrics.font, fontWeight: modern ? '600' : '700', flexShrink: 1 },
  selfStatus: { color: modern ? v4.secondaryText : v4.selectedText, fontSize: modern ? 12 : 12, marginTop: 3 },
  avatarStrip: { flexDirection: 'row', overflow: 'hidden', width: 64, opacity: .8 },
  mail: { minHeight: 32, paddingHorizontal: 5, flexDirection: 'row', alignItems: 'center', gap: 5,
    borderBottomWidth: 1, borderBottomColor: v4.groupBorder, backgroundColor: '#f2feff' },
  mailText: { color: v4.text, fontSize: v4Metrics.smallFont },
  section: modern ? { marginBottom: 16 } : {},
  group: { minHeight: modern ? 48 : 44, paddingHorizontal: modern ? 0 : 5, flexDirection: 'row', gap: modern ? 9 : 4, alignItems: 'center',
    backgroundImage: modern ? undefined : v4Gradient(v4.groupTop, v4.groupBottom), backgroundColor: modern ? 'transparent' : v4.groupBottom,
    borderTopWidth: modern ? 0 : 1, borderBottomWidth: modern ? 0 : 1, borderColor: v4.groupBorder },
  groupText: { color: v4.groupText, fontSize: modern ? 13 : v4Metrics.font, ...(modern && { fontWeight: '600', flex: 1 }) },
  groupCount: { color: v4.secondaryText, fontSize: 12, fontWeight: '500' },
  contact: { minHeight: modern ? 78 : 44, paddingHorizontal: modern ? 2 : 6, paddingVertical: modern ? 14 : 4, flexDirection: 'row',
    gap: modern ? 14 : 5, alignItems: 'center', borderBottomWidth: StyleSheet.hairlineWidth, borderBottomColor: modern ? v4.groupBorder : '#56b3e6',
    ...(!modern && { borderBottomWidth: 1 }) },
  contactPressed: { backgroundColor: v4.selected }, selectedText: { color: v4.selectedText }, contactText: { flex: 1 },
  name: { color: v4.rowText, fontSize: v4Metrics.font, ...(modern && { fontWeight: '600', flex: 1 }) },
  nameLine: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  presence: { color: v4.secondaryText, fontSize: 11, letterSpacing: .1 },
  status: { color: v4.secondaryText, ...(modern && { marginTop: 5, fontSize: 13, lineHeight: 19 }) },
  empty: { color: v4.secondaryText, fontSize: 14, padding: 18 },
  modernSelf: { flexDirection: 'row', alignItems: 'center', padding: 12, paddingRight: 4, gap: 4,
    backgroundColor: v4.dialogBody, borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, borderColor: v4.groupBorder, marginBottom: 14 },
  modernSelfIdentity: { flex: 1, minWidth: 0, flexDirection: 'row', alignItems: 'center', gap: 12 },
  modernGroup: { minHeight: 48, flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 3, borderRadius: 10 },
  modernGroupName: { flex: 1, color: v4.groupText, fontSize: 15, lineHeight: 22, fontWeight: '600' },
  modernList: { backgroundColor: v4.dialogBody, borderRadius: 20, overflow: 'hidden', borderWidth: StyleSheet.hairlineWidth, borderColor: v4.groupBorder },
  modernContactLine: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: 4 },
  modernSeparator: { borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: v4.groupBorder },
  modernContact: { flex: 1, minWidth: 0, minHeight: 76, flexDirection: 'row', alignItems: 'center', gap: 12, paddingHorizontal: 8, paddingVertical: 14, borderRadius: 14 },
  modernName: { color: v4.rowText, fontSize: 16, lineHeight: 23, fontWeight: '600' },
  modernDetail: { color: v4.secondaryText, fontSize: 13, lineHeight: 19, marginTop: 3 },
  optionsSlot: { minWidth: 44, minHeight: 44, alignItems: 'center', justifyContent: 'center' },
  }), [v4, v4Metrics, modern, navigationInset]);
}
