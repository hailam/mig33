import React, { useMemo, useState, type ReactNode } from 'react';
import { ActivityIndicator, Image, Modal, Pressable, SafeAreaView, ScrollView, StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import type { Contact } from '../../mig33/client';
import { useAppearance } from '../appearance';
import { MotionEntrance, MotionPressable } from '../motion';
import { isAvailable } from '../session/model';
import { V4Avatar, V4Button, V4Field, V4Icon } from '../v4/primitives';
import { v4Gradient } from '../v4/theme';
import { v3Colors } from '../v3/theme';

/** The room708 flags are optional for private-group755 participants. */
export interface ChatParticipant { username: string; moderator?: boolean; muted?: boolean; }
export interface ParticipantPickerScreenProps {
  title: string;
  context?: string;
  participants: readonly ChatParticipant[];
  username: string;
  loading: boolean;
  disabledUsernames?: readonly string[];
  allowSelf?: boolean;
  mutedAnnotation?: string;
  onSelect: (username: string) => void;
  onOptions?: (participant: ChatParticipant) => void;
  renderParticipantOptions?: (participant: ChatParticipant) => ReactNode;
  onRefresh?: () => void;
  embedded?: boolean;
}

/** One source-ordered picker serves Private Chat, Kick User, Mute User and
 * Insert Username. The caller supplies eligibility and owns confirmation;
 * choosing a participant never makes a request from this presentation layer.
 */
export function ParticipantPickerScreen({ title, context, participants, username, loading, disabledUsernames = [],
  allowSelf = false, mutedAnnotation = '(Muted)', onSelect, onOptions, renderParticipantOptions, onRefresh, embedded = false }: ParticipantPickerScreenProps) {
  const { modern, v3, theme } = useAppearance();
  const styles = useStyles();
  const Row = modern ? MotionPressable : Pressable;
  const disabled = new Set(disabledUsernames.map((name) => name.toLowerCase()));
  return <ScrollView testID="participant.picker" style={[styles.body, v3 && !embedded && styles.v3Frame]}
    contentContainerStyle={[styles.content, embedded && styles.embeddedContent]} keyboardShouldPersistTaps="handled">
    {modern && !embedded && <ScreenHeading title={title} subtitle={context} />}
    {onRefresh && <View style={styles.summary}><Text style={styles.hint}>{loading ? 'Loading participants…' :
      `${participants.length} ${participants.length === 1 ? 'participant' : 'participants'}`}</Text>
      <V4Button testID="participant.refresh" variant={modern ? 'secondary' : undefined}
        systemImage={modern ? 'arrow.clockwise' : undefined} iconOnly={modern}
        label="Refresh" accessibilityLabel={modern ? 'Refresh participants' : undefined} busy={loading} onPress={onRefresh}
        style={modern ? { width: 48, height: 48 } : undefined} /></View>}
    <View style={styles.list}>
      {participants.map((participant) => {
        const own = participant.username.toLowerCase() === username.toLowerCase();
        const inactive = loading || disabled.has(participant.username.toLowerCase()) || own && !allowSelf;
        const metadata = [own && 'You', participant.moderator && 'Admin', participant.muted && 'Muted'].filter(Boolean).join(' · ');
        return <View key={participant.username} style={styles.rowLine}>
          <Row testID={`participant.select.${participant.username}`} accessibilityRole="button"
            accessibilityLabel={`${participant.username}${metadata ? `, ${metadata}` : ''}`}
            accessibilityState={{ disabled: inactive }} disabled={inactive}
            onPress={() => onSelect(participant.username)}
            onLongPress={onOptions && !own ? () => onOptions(participant) : undefined}
            style={({ pressed }) => [styles.row, pressed && !v3 && styles.pressed]}>
            {({ pressed }) => <>
              {modern ? <V4Avatar size={42} label={participant.username} /> : <V4Icon name="person" size={v3 ? 16 : 20} />}
              <View style={styles.identity}>
                <Text numberOfLines={modern ? undefined : 1} style={[styles.name, inactive && styles.disabled,
                  pressed && (v3 ? styles.v3Selected : styles.selectedText)]}>{participant.username}</Text>
                {modern && metadata !== '' && <Text style={styles.detail}>{metadata}</Text>}
              </View>
              {!modern && metadata !== '' && <Text style={[styles.annotation, inactive && styles.disabled]}>
                {participant.moderator ? '(Admin)' : participant.muted ? mutedAnnotation : own ? '(You)' : ''}
              </Text>}
            </>}
          </Row>
          {!own && (modern && renderParticipantOptions ? <View style={styles.optionsSlot}>{renderParticipantOptions(participant)}</View>
            : onOptions && <OptionsButton testID={`participant.options.${participant.username}`}
              username={participant.username} disabled={loading} onPress={() => onOptions(participant)} />)}
        </View>;
      })}
    </View>
    {participants.length === 0 && <View style={styles.empty}>
      {loading && <ActivityIndicator color={theme.link} />}
      <Text style={styles.emptyText}>{loading ? 'Loading participants…' : 'No participants to show.'}</Text>
    </View>}
  </ScrollView>;
}

export function InviteChatScreen({ contacts, excludedUsernames, busy, onInvite, context }: {
  contacts: readonly Contact[];
  excludedUsernames: readonly string[];
  busy: boolean;
  onInvite: (username: string) => void;
  context?: string;
}) {
  const { modern, v3 } = useAppearance();
  const styles = useStyles();
  const [username, setUsername] = useState('');
  const [validation, setValidation] = useState('');
  const Row = modern ? MotionPressable : Pressable;
  const excluded = new Set(excludedUsernames.map((name) => name.toLowerCase()));
  const available = [...new Map(contacts.filter((contact) => contact.username && isAvailable(contact.presence)
    && !excluded.has(contact.username.toLowerCase())).map((contact) => [contact.username.toLowerCase(), contact])).values()];
  function invite() {
    const name = username.trim().toLowerCase();
    if (!name) return;
    if (excluded.has(name)) { setValidation('Choose someone who is not already in this chat.'); return; }
    setValidation(''); onInvite(name);
  }
  return <ScrollView testID="invite.screen" style={[styles.body, v3 && styles.v3Frame]} contentContainerStyle={styles.content}
    keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag">
    {modern && <ScreenHeading title="Add to chat" subtitle={context} />}
    {/* The recovered Classic picker offers online friends. Modern also exposes
        the existing native request's registered-username capability. */}
    {modern && <View style={styles.inviteForm}>
      <V4Field testID="invite.username" label="Username" value={username} onChangeText={(text) => { setUsername(text); setValidation(''); }}
        autoCapitalize="none" autoCorrect={false} editable={!busy} returnKeyType="go" onSubmitEditing={invite} />
      {validation !== '' && <Text accessibilityLiveRegion="polite" style={styles.validation}>{validation}</Text>}
      <V4Button testID="invite.submit" label="Invite" busy={busy} disabled={!username.trim()} onPress={invite} />
    </View>}
    <Text accessibilityRole="header" style={styles.sectionTitle}>{modern ? 'ONLINE FRIENDS' : 'Invite User'}</Text>
    <View style={styles.list}>{available.map((contact) => <Row key={contact.username}
      testID={`invite.contact.${contact.username}`} accessibilityRole="button" accessibilityState={{ disabled: busy }} disabled={busy}
      accessibilityLabel={`Invite ${contact.display || contact.username}`} onPress={() => onInvite(contact.username)}
      style={({ pressed }) => [styles.row, styles.rowLine, pressed && !v3 && styles.pressed]}>
      {({ pressed }) => <>
        {modern ? <V4Avatar size={42} label={contact.display || contact.username} /> : <V4Icon name="online" size={v3 ? 16 : 20} />}
        <View style={styles.identity}><Text style={[styles.name, pressed && (v3 ? styles.v3Selected : styles.selectedText)]} numberOfLines={1}>
          {contact.display || contact.username}</Text>
          {modern && <Text style={styles.detail}>@{contact.username}</Text>}
        </View>
        {modern && <V4Icon name="plus" size={16} />}
      </>}
    </Row>)}</View>
    {available.length === 0 && <Text style={styles.emptyText}>None of your contacts are online and available to invite.</Text>}
  </ScrollView>;
}

export interface EmoticonChoice { name: string; shortcut: string; imageURI?: string; }
export function EmoticonPicker({ items, loading = false, onSelect }: {
  items: readonly EmoticonChoice[]; loading?: boolean; onSelect: (shortcut: string) => void;
}) {
  const { theme } = useAppearance();
  const styles = useStyles();
  return <ScrollView testID="composer.emoticons" keyboardShouldPersistTaps="handled" contentContainerStyle={styles.grid}>
    {items.map((item, index) => <EmoticonButton key={`${item.name}:${index}`} item={item} onSelect={onSelect} />)}
    {items.length === 0 && <View style={styles.empty}>
      {loading && <ActivityIndicator color={theme.link} />}
      <Text style={styles.emptyText}>{loading ? 'Loading emoticons…' : 'No emoticons available.'}</Text>
    </View>}
  </ScrollView>;
}

function EmoticonButton({ item, onSelect }: { item: EmoticonChoice; onSelect: (shortcut: string) => void }) {
  const styles = useStyles();
  const { modern } = useAppearance();
  const { fontScale } = useWindowDimensions();
  const [failedURI, setFailedURI] = useState<string | undefined>();
  return <MotionPressable testID={`composer.emoticon.${item.name}`} accessibilityRole="button"
    accessibilityLabel={`${item.name}, ${item.shortcut}`} onPress={() => onSelect(item.shortcut)}
    style={({ pressed }) => [styles.emoticonCell, modern && fontScale > 1.3 && { width: 132 }, pressed && styles.pressed]}>
    {item.imageURI && failedURI !== item.imageURI ? <Image source={{ uri: item.imageURI }} style={styles.emoticonImage}
      resizeMode="contain" onError={() => setFailedURI(item.imageURI)} accessible={false} />
      : <Text style={styles.emoticonShortcut}>{item.shortcut}</Text>}
    <Text style={styles.emoticonName} numberOfLines={modern ? undefined : 2}>{item.name}</Text>
  </MotionPressable>;
}

/** Server-supplied924 choices; Default clears the optional outgoing override. */
export function TextColorPicker({ colors, selected, onSelect }: {
  colors: readonly number[]; selected?: number; onSelect: (color: number) => void;
}) {
  const { theme, modern } = useAppearance();
  const { fontScale } = useWindowDimensions();
  const styles = useStyles();
  const palette = [...new Set(colors.filter((value) => Number.isInteger(value) && value >= 0 && value <= 0xffffff))];
  return <ScrollView testID="composer.colors" contentContainerStyle={styles.grid}>
    <MotionPressable testID="composer.color.default" accessibilityRole="button" accessibilityLabel="Default text color"
      accessibilityState={{ selected: selected === undefined || selected < 0 }} onPress={() => onSelect(-1)}
      style={({ pressed }) => [styles.colorCell, modern && fontScale > 1.3 && { width: 132 }, (pressed || selected === undefined || selected < 0) && styles.colorSelected]}>
      <View style={[styles.colorSwatch, { backgroundColor: theme.body, alignItems: 'center', justifyContent: 'center' }]}>
        <Text style={{ color: theme.text, fontSize: 20 }}>Aa</Text>
      </View><Text style={styles.colorLabel}>Default</Text>
    </MotionPressable>
    {palette.map((value) => {
      const hex = `#${value.toString(16).padStart(6, '0')}`;
      return <MotionPressable key={value} testID={`composer.color.${value}`} accessibilityRole="button"
        accessibilityLabel={`Text color ${hex}`} accessibilityState={{ selected: selected === value }} onPress={() => onSelect(value)}
        style={({ pressed }) => [styles.colorCell, modern && fontScale > 1.3 && { width: 132 }, (pressed || selected === value) && styles.colorSelected]}>
        <View style={[styles.colorSwatch, { backgroundColor: hex }]} />
        <Text style={styles.colorLabel}>{hex.toUpperCase()}</Text>
        {selected === value && <View style={styles.colorCheck}><V4Icon name="check" size={12} color={theme.link} /></View>}
      </MotionPressable>;
    })}
    {palette.length === 0 && <Text style={styles.emptyText}>No additional text colors available.</Text>}
  </ScrollView>;
}

/** Solid content sheet. It can share Application's single native Modal to
 * avoid presenting a second UIKit modal while another action is dismissing.
 */
export function ChatActionSheet({ visible, title, children, onClose, embedded = false }: {
  visible: boolean; title: string; children: ReactNode; onClose: () => void; embedded?: boolean;
}) {
  const { modern } = useAppearance();
  const { height } = useWindowDimensions();
  const styles = useStyles();
  if (embedded && !visible) return null;
  const content = <SafeAreaView style={styles.sheetOverlay}>
    <Pressable accessibilityRole="button" accessibilityLabel={`Close ${title}`} onPress={onClose} style={StyleSheet.absoluteFill} />
    <MotionEntrance style={styles.sheet} accessibilityViewIsModal transitionKey={`${visible}:${title}`}>
      {modern && <View accessible={false} style={styles.sheetHandle} />}
      <Text accessibilityRole="header" style={styles.sheetTitle}>{title}</Text>
      <View style={[styles.sheetBody, { height: Math.min(440, height * .55) }]}>{children}</View>
      <View style={styles.sheetFooter}><V4Button testID="chat.sheet.close" label="Close" onPress={onClose} /></View>
    </MotionEntrance>
  </SafeAreaView>;
  return embedded ? content : <Modal transparent visible={visible} animationType="none" onRequestClose={onClose}>{content}</Modal>;
}

export function ParticipantOptionsButton({ username, disabled = false, onPress, testID }: {
  username: string; disabled?: boolean; onPress: () => void; testID?: string;
}) {
  return <OptionsButton testID={testID ?? `participants.options.${username}`} username={username} disabled={disabled} onPress={onPress} />;
}
function OptionsButton({ testID, username, disabled, onPress }: {
  testID: string; username: string; disabled: boolean; onPress: () => void;
}) {
  const styles = useStyles();
  const { modern } = useAppearance();
  return <MotionPressable testID={testID} accessibilityRole="button" accessibilityLabel={`Options for ${username}`}
    accessibilityState={{ disabled }} disabled={disabled} onPress={onPress} style={({ pressed }) => [styles.options, modern && pressed && styles.pressed]}>
    <View pointerEvents="none" style={styles.dots}>{[0, 1, 2].map((dot) => <View key={dot} style={styles.dot} />)}</View>
  </MotionPressable>;
}

export function ScreenHeading({ title, subtitle, action }: { title: string; subtitle?: string; action?: ReactNode }) {
  const styles = useStyles();
  const { fontScale } = useWindowDimensions();
  return <View style={[styles.heading, !!action && styles.headingWithAction, !!action && fontScale > 1.3 && styles.headingStacked]}>
    <View style={[styles.headingCopy, !!action && fontScale <= 1.3 && { flex: 1 }]}><Text accessibilityRole="header" style={styles.headingTitle}>{title}</Text>
      {subtitle && <Text style={styles.subtitle}>{subtitle}</Text>}</View>
    {action && <View style={styles.headingAction}>{action}</View>}
  </View>;
}

function useStyles() {
  const { theme, metrics, modern, v3, navigationInset } = useAppearance();
  return useMemo(() => StyleSheet.create({
    body: { flex: 1, backgroundColor: modern ? 'transparent' : theme.body },
    content: { paddingTop: modern ? navigationInset : 6, paddingHorizontal: modern ? metrics.pageInset : 6, paddingBottom: 24 },
    embeddedContent: { paddingTop: 4, paddingHorizontal: 8, paddingBottom: 8 },
    v3Frame: { borderWidth: 2, borderTopColor: v3Colors.selectionOutline, borderLeftColor: v3Colors.selectionOutline,
      borderBottomColor: v3Colors.contentBevel, borderRightColor: v3Colors.contentBevel },
    heading: { paddingTop: 14, paddingBottom: 18 }, headingWithAction: { flexDirection: 'row', alignItems: 'center', gap: 12 },
    headingStacked: { flexDirection: 'column', alignItems: 'stretch' }, headingCopy: { minWidth: 0 }, headingAction: { alignSelf: 'flex-start' },
    headingTitle: { color: theme.text, fontSize: 28, lineHeight: 35, fontWeight: '700', letterSpacing: -.6 },
    subtitle: { color: theme.secondaryText, fontSize: 14, lineHeight: 21, marginTop: 5 },
    summary: { flexDirection: 'row', alignItems: 'center', gap: 12, paddingBottom: 12 },
    hint: { flex: 1, color: theme.secondaryText, fontSize: metrics.smallFont },
    list: { backgroundColor: modern ? theme.dialogBody : 'transparent', borderRadius: modern ? 20 : 0, overflow: 'hidden', paddingHorizontal: modern ? 14 : 0,
      ...(modern && { borderWidth: StyleSheet.hairlineWidth, borderColor: theme.groupBorder }) },
    rowLine: { flexDirection: 'row', borderBottomWidth: v3 ? 0 : StyleSheet.hairlineWidth, borderBottomColor: theme.groupBorder },
    row: { flex: 1, minWidth: 0, minHeight: modern ? 76 : 44, paddingHorizontal: modern ? 0 : 3, paddingVertical: modern ? 14 : 4,
      flexDirection: 'row', gap: modern ? 12 : 5, alignItems: 'center' },
    identity: { flex: 1, minWidth: 0 }, name: { color: theme.text, fontSize: metrics.font, fontWeight: modern ? '600' : '400',
      ...(v3 && { borderWidth: 1, borderColor: 'transparent', alignSelf: 'flex-start', maxWidth: '100%', paddingHorizontal: 2 }) },
    detail: { color: theme.secondaryText, fontSize: modern ? 13 : 12, lineHeight: modern ? 19 : 18, marginTop: modern ? 3 : 5 },
    annotation: { color: theme.secondaryText, fontSize: metrics.smallFont },
    disabled: { color: theme.muted }, pressed: { backgroundColor: theme.selected }, selectedText: { color: theme.selectedText },
    v3Selected: { backgroundColor: theme.selected, borderColor: theme.groupBorder, fontWeight: '700' },
    empty: { paddingVertical: 20, alignItems: 'center', width: '100%' },
    emptyText: { color: theme.secondaryText, fontSize: metrics.smallFont, lineHeight: 20, padding: 12 },
    inviteForm: { backgroundColor: theme.dialogBody, borderRadius: 20, padding: 16, marginBottom: 28 },
    validation: { color: theme.text, fontSize: 13, lineHeight: 20, marginBottom: 10 },
    sectionTitle: { color: theme.secondaryText, fontSize: modern ? 11 : metrics.font, letterSpacing: modern ? 1 : 0,
      fontWeight: modern ? '600' : '400', paddingVertical: 10 },
    options: { width: 44, minHeight: 44, borderRadius: modern ? 22 : 0, alignItems: 'center', justifyContent: 'center' },
    optionsSlot: { minWidth: 44, minHeight: 44, alignItems: 'center', justifyContent: 'center' },
    dots: { flexDirection: 'row', gap: 3 }, dot: { width: 3, height: 3, borderRadius: 2, backgroundColor: theme.secondaryText },
    grid: { flexDirection: 'row', flexWrap: 'wrap', padding: modern ? 12 : 4, gap: modern ? 8 : 4 },
    emoticonCell: { width: 88, minHeight: 90, alignItems: 'center', justifyContent: 'center', padding: 6,
      borderRadius: modern ? 16 : 1, borderWidth: 1, borderColor: theme.groupBorder },
    emoticonImage: { width: 32, height: 32 }, emoticonShortcut: { color: theme.text, fontSize: 22, fontWeight: '600', minHeight: 32 },
    emoticonName: { color: theme.secondaryText, textAlign: 'center', fontSize: modern ? 13 : 11, lineHeight: modern ? 19 : 15, marginTop: 5 },
    colorCell: { width: 88, minHeight: 84, padding: 7, gap: 7, borderWidth: 2, borderColor: 'transparent', borderRadius: modern ? 14 : 1 },
    colorSelected: { borderColor: theme.focus }, colorSwatch: { height: 36, borderRadius: modern ? 8 : 0, borderWidth: 1, borderColor: theme.groupBorder },
    colorLabel: { fontSize: modern ? 12 : 10, textAlign: 'center', color: theme.text }, colorCheck: { position: 'absolute', top: 3, right: 3,
      backgroundColor: theme.dialogBody, borderRadius: 9, width: 18, height: 18, alignItems: 'center', justifyContent: 'center' },
    sheetOverlay: { flex: 1, backgroundColor: modern ? '#00000055' : '#00000033', justifyContent: modern ? 'flex-end' : 'center', padding: modern ? 12 : 8 },
    sheet: { maxHeight: '88%', minHeight: 140, backgroundColor: theme.dialogBody, borderRadius: modern ? 28 : 0,
      borderWidth: modern ? 0 : 1, borderColor: theme.dialogBorder, overflow: 'hidden',
      ...(v3 && { shadowColor: v3Colors.shadow, shadowOpacity: 1, shadowRadius: 0, shadowOffset: { width: 3, height: 3 } }) },
    sheetHandle: { width: 34, height: 4, borderRadius: 2, backgroundColor: theme.border, alignSelf: 'center', marginTop: 10 },
    sheetTitle: { fontSize: modern ? 21 : metrics.font, fontWeight: modern ? '600' : '400', color: modern ? theme.text : theme.headerText,
      backgroundColor: modern ? 'transparent' : theme.headerTop, backgroundImage: modern ? undefined : v4Gradient(theme.headerTop, theme.headerBottom),
      textAlign: modern ? 'left' : 'center', paddingHorizontal: modern ? 20 : 6, paddingTop: modern ? 18 : 5, paddingBottom: modern ? 14 : 5 },
    sheetBody: { flexShrink: 1, minHeight: 72 }, sheetFooter: { alignItems: 'flex-end', paddingHorizontal: 16, paddingBottom: 8 },
  }), [theme, metrics, modern, v3, navigationInset]);
}
