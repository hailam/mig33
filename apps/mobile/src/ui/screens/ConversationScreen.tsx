import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { ActivityIndicator, FlatList, Image, Pressable, ScrollView, StyleSheet, Text, View,
  type NativeScrollEvent, type NativeSyntheticEvent, type StyleProp, type ViewStyle } from 'react-native';
import type { Contact, EmoticonAsset } from '../../mig33/client';
import type { Conversation, TranscriptRow } from '../session/model';
import { useAppearance } from '../appearance';
import { GlassSurface } from '../glass';
import { ModernButton, supportsModernControls } from '../ModernControls';
import { MotionEntrance, MotionPressable } from '../motion';
import { V4Avatar, V4Button, V4Field, V4Icon } from '../v4/primitives';
import { v4ThemeFields } from '../v4/theme';
import { v3Colors, v3MessageColors } from '../v3/theme';
import { readableMessageColor } from './messageColor';
import { useSlashCommandComposer } from '../useSlashCommandComposer';
import type { SlashCommandContext } from '../slashCommands';
import { SlashCommandSuggestions } from './SlashCommandSuggestions';

export function peerDisplay(peer: string, contacts: readonly Contact[]): string {
  return contacts.find((contact) => contact.username === peer)?.display || peer;
}
function color(value: number | undefined, fallback: string): string {
  return value === undefined || value < 0 ? fallback : `#${(value & 0xffffff).toString(16).padStart(6, '0')}`;
}

type EmoticonAssets = Readonly<Record<string, EmoticonAsset>>;
export interface ComposerSelection { start: number; end: number; }
export function ConversationScreen({ conversation, contacts, onWrite, sending, onChange, onSend, onTools, toolsControl, title,
  onMediaOpen, emoticonAssets = {}, onLoadMedia, selection, onSelectionChange, commandContext, onApplyDraft }: {
  conversation: Conversation; contacts: readonly Contact[]; onWrite: () => void;
  sending: boolean; onChange: (text: string) => void; onSend: () => void;
  onTools?: () => void;
  toolsControl?: ReactNode;
  title?: string;
  onMediaOpen?: (row: TranscriptRow) => void;
  emoticonAssets?: EmoticonAssets;
  onLoadMedia?: (token: string) => void;
  selection?: ComposerSelection;
  onSelectionChange?: (selection: ComposerSelection) => void;
  commandContext: SlashCommandContext;
  onApplyDraft: (text: string, selection: ComposerSelection) => void;
}) {
  const { modern, v3, theme } = useAppearance();
  const styles = useStyles();
  const slash = useSlashCommandComposer({ draft: conversation.draft, context: commandContext,
    selection, disabled: sending, onChange, onSelectionChange, onApply: onApplyDraft });
  const list = useRef<FlatList<TranscriptRow>>(null);
  const anchor = useRef({ pinned: true, dragging: false, momentum: false, awaitingMomentum: false });
  const pendingFollow = useRef<number | null>(null);
  const conversationIdentity = 'name' in conversation ? `room:${conversation.name}`
    : 'id' in conversation ? `group:${conversation.id}` : `peer:${conversation.peer}`;
  const cancelFollow = useCallback(() => {
    if (pendingFollow.current !== null) cancelAnimationFrame(pendingFollow.current);
    pendingFollow.current = null;
  }, []);
  const followBottom = useCallback(() => {
    const state = anchor.current;
    if (!state.pinned || state.dragging || state.momentum || pendingFollow.current !== null) return;
    pendingFollow.current = requestAnimationFrame(() => {
      pendingFollow.current = null;
      const current = anchor.current;
      if (!current.pinned || current.dragging || current.momentum) return;
      list.current?.scrollToEnd({ animated: false });
    });
  }, []);
  useLayoutEffect(() => {
    cancelFollow();
    anchor.current = { pinned: true, dragging: false, momentum: false, awaitingMomentum: false };
    followBottom();
    return cancelFollow;
  }, [conversationIdentity, cancelFollow, followBottom]);
  useLayoutEffect(() => { followBottom(); }, [conversation.rows.at(-1)?.id, followBottom]);
  function updateUserAnchor(event: NativeSyntheticEvent<NativeScrollEvent>) {
    const { contentOffset, contentSize, layoutMeasurement, contentInset } = event.nativeEvent;
    anchor.current.pinned = contentOffset.y + layoutMeasurement.height >= contentSize.height + contentInset.bottom - 40;
  }
  const [composerHeight, setComposerHeight] = useState(64);
  const knownRows = useRef(new Set(conversation.rows.map((row) => row.id)));
  const arrivingRows = new Set(conversation.rows.filter((row) => !knownRows.current.has(row.id)).map((row) => row.id));
  useEffect(() => { knownRows.current = new Set(conversation.rows.map((row) => row.id)); }, [conversation.rows]);
  const transcript = <FlatList testID="chat.screen" ref={list} data={conversation.rows} keyExtractor={(row) => row.id}
    style={styles.body} contentContainerStyle={[styles.transcript, modern && { paddingBottom: 0 }]}
    // VirtualizedList.scrollToEnd includes its measured footer, but not
    // content-container bottom padding. Keep the composer reserve in that
    // footer so the final message is fully above the floating controls.
    ListFooterComponent={modern ? <View onLayout={followBottom} style={{ height: composerHeight + 24 }} /> : undefined}
    keyboardShouldPersistTaps="handled" keyboardDismissMode="interactive"
    onScroll={(event) => {
      // Layout changes, image loads and our own scroll commands can all emit
      // onScroll. Only a person's gesture can change whether we follow replies.
      if (anchor.current.dragging || anchor.current.momentum) updateUserAnchor(event);
    }} scrollEventThrottle={32}
    onScrollBeginDrag={(event) => {
      cancelFollow();
      anchor.current.dragging = true;
      anchor.current.momentum = false;
      anchor.current.awaitingMomentum = false;
      updateUserAnchor(event);
    }}
    onScrollEndDrag={(event) => {
      updateUserAnchor(event);
      anchor.current.dragging = false;
      anchor.current.awaitingMomentum = true;
      followBottom();
    }}
    onMomentumScrollBegin={() => {
      if (anchor.current.awaitingMomentum) {
        cancelFollow();
        anchor.current.momentum = true;
        anchor.current.awaitingMomentum = false;
      }
    }}
    onMomentumScrollEnd={(event) => {
      if (!anchor.current.momentum) return;
      updateUserAnchor(event);
      anchor.current.momentum = false;
      followBottom();
    }}
    onContentSizeChange={followBottom}
    onLayout={followBottom}
    ListEmptyComponent={modern ? <View style={styles.emptyConversation}>
      <View style={styles.emptySymbol}><V4Avatar size={64} label={title ?? peerDisplay(conversation.peer, contacts)} /></View>
      <Text style={styles.emptyTitle}>{title ?? peerDisplay(conversation.peer, contacts)}</Text>
      <Text style={styles.emptyDescription}>Start a conversation.</Text>
    </View> : v3 ? undefined : <Pressable accessibilityRole="button" accessibilityLabel="Write a message"
      onPress={onWrite} style={styles.emptyTarget}><Text style={styles.empty}>Write</Text></Pressable>}
    renderItem={({ item, index }) => {
      const photo = item.content === 2 || item.content === 5 || !!item.imageURI || !!item.mediaReference;
      const media = photo ? <MessagePhoto row={item} assets={emoticonAssets} onLoad={onLoadMedia} onOpen={onMediaOpen} /> : null;
      const message = <InlineMessage row={item} assets={emoticonAssets} onLoad={onLoadMedia} />;
      if (modern) {
        const previous = conversation.rows[index - 1];
        const grouped = previous !== undefined && !previous.action && !item.action
          && previous.sender === item.sender && previous.own === item.own;
        const animate = arrivingRows.has(item.id);
        if (item.action) return <MessageEntrance animate={animate}>
          <Text selectable style={[styles.action, { color: readableMessageColor(item.color, theme.transcript, theme.body) }]}>{message}</Text>
        </MessageEntrance>;
        const background = item.own ? theme.groupTop : theme.dialogBody;
        return <MessageEntrance animate={animate} style={[styles.bubbleRow, item.own && styles.ownRow, !grouped && styles.newGroup]}>
          {!item.own && item.avatar !== '-1' && <View style={styles.modernAvatar}>
            {!grouped && <V4Avatar size={30} label={peerDisplay(item.sender, contacts)} />}
          </View>}
          <View style={[styles.bubble, item.own ? styles.ownBubble : styles.peerBubble]}>
            {!item.own && !grouped && <Text style={[styles.sender, { color: readableMessageColor(item.senderColor, theme.link, background) }]}>
              {peerDisplay(item.sender, contacts)}
            </Text>}
            {media}
            {(!photo || hasPhotoCaption(item)) && <Text selectable style={[styles.bubbleText,
              { color: readableMessageColor(item.color, theme.transcript, background) }]}>{message}</Text>}
          </View>
        </MessageEntrance>;
      }
      return <View style={styles.messageRow}>
      {/* Original content6/action and avatar-1 paths have no avatar column.
          Normal messages retain it across wrapped lines, even for one sender. */}
      {!v3 && !item.action && item.avatar !== '-1' && <View style={styles.avatarGutter}><V4Avatar size={28} /></View>}
      <View style={styles.messageContent}><Text selectable style={[styles.message, { color: color(item.color, v3 && item.action ? v3MessageColors[5] : theme.transcript) }]}>
        {!item.action && <Text style={{ color: color(item.senderColor, v3 ? v3MessageColors[item.own ? 0 : 1] : v4ThemeFields[144]) }}>
          {peerDisplay(item.sender, contacts)}: </Text>}
        {(!photo || hasPhotoCaption(item)) && message}
      </Text>
      {media}</View>
    </View>;
    }} />;
  if (v3) return <View style={[styles.conversation, styles.v3Frame]}>{transcript}</View>;
  if (!modern) return transcript;
  return <View style={styles.conversation}>
    {transcript}
    <View style={styles.composer} onLayout={(event) => setComposerHeight(event.nativeEvent.layout.height)}>
      {slash.visible && <SlashCommandSuggestions commands={slash.commands} onSelect={slash.select} onDismiss={slash.dismiss} />}
      <View style={styles.composerRow}>
        {toolsControl ? <View style={styles.toolsButton}>{toolsControl}</View> : onTools && <MotionPressable testID="chat.tools" accessibilityRole="button" accessibilityLabel="Message tools"
          onPress={onTools} style={styles.toolsButton}><V4Icon name="plus" size={19} color={theme.link} /></MotionPressable>}
        <GlassSurface style={styles.composerFieldSurface}><V4Field ref={slash.input} testID="chat.message" accessibilityLabel="Message" placeholder="Message" value={conversation.draft}
          onChangeText={slash.onChangeText} onFocus={slash.onFocus} onBlur={slash.onBlur} multiline maxLength={320} textAlignVertical="top"
          selection={selection} onSelectionChange={(event) => slash.onSelectionChange(event.nativeEvent.selection)}
          variant="composer" containerStyle={styles.composerField} style={styles.composerInput} /></GlassSurface>
        {supportsModernControls ? <ModernButton label="Send" testID="chat.send" systemImage="arrow.up" iconOnly variant="primary"
          onPress={onSend} busy={sending} disabled={!conversation.draft.trim()} style={styles.nativeSend} />
          : <MotionPressable testID="chat.send" accessibilityRole="button" accessibilityLabel="Send"
          accessibilityState={{ busy: sending, disabled: sending || !conversation.draft.trim() }}
          disabled={sending || !conversation.draft.trim()} onPress={onSend}
          style={[styles.sendButton, !conversation.draft.trim() && styles.sendDisabled]}>
          {sending ? <ActivityIndicator size="small" color={theme.buttonText} /> : <Text style={styles.sendArrow}>↑</Text>}
        </MotionPressable>}
      </View>
    </View>
  </View>;
}

function hasPhotoCaption(row: TranscriptRow): boolean {
  const text = row.body.trim();
  return text !== '' && text !== row.mediaReference && !/^\[photo:[^\]]+\]$/.test(text);
}

function useMediaRequests(tokens: readonly string[], assets: EmoticonAssets, onLoad?: (token: string) => void) {
  const requested = useRef(new Set<string>());
  const previous = useRef(assets);
  useEffect(() => {
    if (!onLoad) return;
    // Original915 invalidates downloaded resources. Permit one new fetch after
    // an existing asset disappears, while avoiding repeated failed fetches.
    for (const token of tokens) if (previous.current[token] && !assets[token]) requested.current.delete(token);
    previous.current = assets;
    for (const token of tokens) if (!assets[token] && !requested.current.has(token)) {
      requested.current.add(token); onLoad(token);
    }
  }, [tokens, assets, onLoad]);
}

/** The wire's token list determines which strings may acquire artwork. Unknown
 * or missing downloads keep their original readable shortcut in the transcript.
 */
function InlineMessage({ row, assets, onLoad }: { row: TranscriptRow; assets: EmoticonAssets; onLoad?: (token: string) => void }) {
  const tokens = (row.tokens ?? []).filter((token) => token.length > 0 && token !== row.mediaReference);
  useMediaRequests(tokens, assets, onLoad);
  const drawable = tokens.filter((token) => assets[token]?.imageURI || assets[token]?.frameURIs?.length).sort((a, b) => b.length - a.length);
  if (drawable.length === 0) return row.body;
  const parts: ReactNode[] = [];
  let position = 0;
  while (position < row.body.length) {
    let nearest = row.body.length;
    let selected = '';
    for (const token of drawable) {
      const found = row.body.indexOf(token, position);
      if (found >= 0 && found < nearest) { nearest = found; selected = token; }
    }
    if (nearest > position) parts.push(row.body.slice(position, nearest));
    if (!selected) break;
    parts.push(<EmoticonImage key={`${position}:${selected}`} asset={assets[selected]} />);
    position = nearest + selected.length;
  }
  return <>{parts}</>;
}

function EmoticonImage({ asset }: { asset: EmoticonAsset }) {
  const { reduceMotion } = useAppearance();
  const [frame, setFrame] = useState(0);
  const [failed, setFailed] = useState(false);
  const frames = asset.frameURIs;
  useEffect(() => {
    setFrame(0); setFailed(false);
    if (reduceMotion || !frames || frames.length < 2) return;
    const interval = setInterval(() => setFrame((before) => (before + 1) % frames.length), 180);
    return () => clearInterval(interval);
  }, [frames, reduceMotion, asset.imageURI]);
  if (failed) return asset.token;
  const uri = frames?.[reduceMotion ? 0 : frame] ?? asset.imageURI;
  return <Image source={{ uri }} accessibilityLabel={asset.label || asset.token} style={{ width: 22, height: 22 }}
    resizeMode="contain" onError={() => setFailed(true)} />;
}

function MessagePhoto({ row, assets, onLoad, onOpen }: {
  row: TranscriptRow; assets: EmoticonAssets; onLoad?: (token: string) => void; onOpen?: (row: TranscriptRow) => void;
}) {
  const styles = useStyles();
  const [failed, setFailed] = useState(false);
  const [aspect, setAspect] = useState(4 / 3);
  const reference = row.mediaReference && (/^\d+$/.test(row.mediaReference) ? `[photo:${row.mediaReference}]` : row.mediaReference);
  useMediaRequests(reference ? [reference] : [], assets, onLoad);
  const uri = row.imageURI ?? (reference ? assets[reference]?.imageURI : undefined);
  useEffect(() => { setFailed(false); }, [uri]);
  return <Pressable testID={`chat.photo.${row.id}`} accessibilityRole={onOpen ? 'button' : 'image'}
    accessibilityLabel={`${failed ? 'Unavailable photo' : 'Photo'} from ${row.sender}`}
    accessibilityHint={onOpen ? 'Open photo' : undefined} disabled={!onOpen} onPress={() => onOpen?.(row)}
    style={styles.photo}>
    {uri && !failed ? <Image source={{ uri }} style={[styles.photoImage, { aspectRatio: aspect }]} resizeMode="contain"
      onError={() => setFailed(true)} onLoad={(event) => {
        const { width, height } = event.nativeEvent.source;
        if (width > 0 && height > 0) setAspect(Math.min(2, Math.max(.6, width / height)));
      }} /> : <View style={styles.photoPlaceholder}><V4Icon name="page" size={30} />
      <Text style={styles.photoCaption}>{failed ? 'Photo unavailable' : 'Photo'}</Text></View>}
  </Pressable>;
}

/** Only a row first mounted after delivery enters; restored/virtualized history stays still. */
function MessageEntrance({ animate, children, style }: { animate: boolean; children: ReactNode; style?: StyleProp<ViewStyle> }) {
  const onArrival = useRef(animate).current;
  return onArrival ? <MotionEntrance style={style}>{children}</MotionEntrance> : <View style={style}>{children}</View>;
}

export function ChatsScreen({ conversations, contacts, onOpen }: {
  conversations: readonly Conversation[]; contacts: readonly Contact[]; onOpen: (peer: string) => void;
}) {
  const { modern } = useAppearance();
  const styles = useStyles();
  const Row = modern ? MotionPressable : Pressable;
  return <ScrollView testID="chats.screen" style={styles.body}>
    {conversations.map((conversation) => <Row key={conversation.peer} accessibilityRole="button"
      testID={`chats.peer.${conversation.peer}`} onPress={() => onOpen(conversation.peer)}
      style={({ pressed }) => [styles.chatRow, pressed && styles.selected]}>
      {modern ? <V4Avatar size={44} label={peerDisplay(conversation.peer, contacts)} /> : <V4Icon name="conversation" size={20} />}
      {modern ? <View style={styles.chatText}>
        <Text style={[styles.chatName, conversation.unread > 0 && styles.unread]} numberOfLines={1}>
          {peerDisplay(conversation.peer, contacts)}
        </Text>
        <Text style={styles.preview} numberOfLines={1}>{conversation.draft ? `Draft: ${conversation.draft}` : conversation.rows.at(-1)?.body || 'Start a conversation'}</Text>
      </View> : <Text style={[styles.chatName, conversation.unread > 0 && styles.unread]} numberOfLines={1}>
        {peerDisplay(conversation.peer, contacts)}{conversation.unread > 0 ? ' *' : ''}
      </Text>}
      {modern && conversation.unread > 0 && <Text style={styles.unreadBadge}>{conversation.unread}</Text>}
    </Row>)}
  </ScrollView>;
}

/** The original native Write Message editor, adapted to direct touch input. */
export function WriteScreen({ conversation, onChange, sending, onTools, selection, onSelectionChange, commandContext, onApplyDraft }: {
  conversation: Conversation; onChange: (text: string) => void; sending: boolean;
  onTools?: () => void;
  selection?: ComposerSelection;
  onSelectionChange?: (selection: ComposerSelection) => void;
  commandContext: SlashCommandContext;
  onApplyDraft: (text: string, selection: ComposerSelection) => void;
}) {
  const styles = useStyles();
  const slash = useSlashCommandComposer({ draft: conversation.draft, context: commandContext,
    selection, disabled: sending, onChange, onSelectionChange, onApply: onApplyDraft });
  return <View style={styles.editor}>
    {onTools && <View style={styles.editorToolbar}><V4Button testID="chat.tools" label="Message tools" onPress={onTools} disabled={sending} /></View>}
    {slash.visible && <SlashCommandSuggestions commands={slash.commands} onSelect={slash.select} onDismiss={slash.dismiss} maxHeight={180} />}
    <V4Field ref={slash.input} testID="chat.message" accessibilityLabel="Message" value={conversation.draft} onChangeText={slash.onChangeText}
      onFocus={slash.onFocus} onBlur={slash.onBlur}
      multiline autoFocus editable={!sending} maxLength={320} textAlignVertical="top"
      selection={selection} onSelectionChange={(event) => slash.onSelectionChange(event.nativeEvent.selection)}
      containerStyle={styles.editorField} style={styles.editorInput} />
  </View>;
}

function useStyles() {
  const { theme: v4, metrics: v4Metrics, modern, v3, navigationInset } = useAppearance();
  return useMemo(() => StyleSheet.create({
  body: { flex: 1, backgroundColor: modern ? 'transparent' : v4.body }, transcript: { padding: modern ? 18 : 3,
    paddingTop: modern ? navigationInset : v3 ? 0 : 3, flexGrow: 1 },
  v3Frame: { borderWidth: 2, borderTopColor: v3Colors.selectionOutline, borderLeftColor: v3Colors.selectionOutline,
    borderBottomColor: v3Colors.contentBevel, borderRightColor: v3Colors.contentBevel },
  messageRow: { flexDirection: 'row', alignItems: 'flex-start', marginBottom: v3 ? 0 : 4 },
  avatarGutter: { width: 34, paddingTop: 1 },
  messageContent: { flex: 1, minWidth: 0 },
  message: { fontSize: v4Metrics.font, lineHeight: v4Metrics.lineHeight, color: v4.transcript },
  emptyTarget: { minHeight: 44, alignSelf: 'flex-start', justifyContent: 'center', padding: 4 },
  empty: { color: v4.link, fontSize: v4Metrics.font, textDecorationLine: 'underline' },
  chatRow: { flexDirection: 'row', alignItems: 'center', gap: modern ? 12 : 6, minHeight: modern ? 84 : 44,
    paddingHorizontal: modern ? v4Metrics.pageInset : 6, borderBottomWidth: 1, borderBottomColor: modern ? v4.groupBorder : '#56b3e6' },
  selected: { backgroundColor: modern ? v4.selected : v4.focusTop },
  chatName: { color: v4.text, fontSize: v4Metrics.font, ...(!modern && { flex: 1 }) },
  unread: { fontWeight: '700' }, editor: { flex: 1, padding: 4 },
  editorField: { flex: 1 }, editorInput: { minHeight: 180, maxHeight: 360 },
  editorToolbar: { flexDirection: 'row', justifyContent: 'flex-end', paddingHorizontal: 2 },
  chatText: { flex: 1 }, preview: { color: v4.secondaryText, fontSize: 13, marginTop: 5 },
  unreadBadge: { color: v4.headerText, backgroundColor: v4.headerTop, minWidth: 24, height: 24,
    textAlign: 'center', lineHeight: 24, borderRadius: 12, overflow: 'hidden', fontSize: 12, fontWeight: '700' },
  conversation: { flex: 1, backgroundColor: modern ? 'transparent' : v4.body },
  emptyConversation: { flex: 1, alignItems: 'center', justifyContent: 'center', padding: 24 },
  emptySymbol: { alignItems: 'center', justifyContent: 'center', marginBottom: 22 },
  emptyTitle: { color: v4.text, fontSize: 26, fontWeight: '700', textAlign: 'center', letterSpacing: -.6 },
  emptyDescription: { color: v4.secondaryText, fontSize: 15, marginTop: 8, textAlign: 'center' },
  bubbleRow: { flexDirection: 'row', alignItems: 'flex-end', marginBottom: 4 },
  ownRow: { justifyContent: 'flex-end' }, newGroup: { marginTop: 14 },
  modernAvatar: { width: 38, alignSelf: 'flex-start', paddingTop: 3 },
  bubble: { maxWidth: '84%', borderRadius: 21, paddingHorizontal: 15, paddingVertical: 11 },
  ownBubble: { backgroundColor: v4.groupTop, borderBottomRightRadius: 5 },
  peerBubble: { backgroundColor: v4.dialogBody, borderBottomLeftRadius: 5 },
  sender: { fontSize: 12, fontWeight: '600', marginBottom: 4 },
  bubbleText: { fontSize: v4Metrics.font, lineHeight: v4Metrics.lineHeight },
  photo: { width: 210, maxWidth: '100%', marginVertical: 4, backgroundColor: v4.subtle,
    borderRadius: modern ? 14 : 0, overflow: 'hidden' },
  photoImage: { width: '100%' }, photoPlaceholder: { minHeight: 140, alignItems: 'center', justifyContent: 'center', gap: 10 },
  photoCaption: { color: v4.secondaryText, fontSize: v4Metrics.smallFont },
  action: { fontSize: 14, lineHeight: 21, textAlign: 'center', marginVertical: 14, paddingHorizontal: 20 },
  composer: { position: 'absolute', left: 12, right: 12, bottom: 10, borderRadius: 32, gap: 8 },
  composerRow: { flexDirection: 'row', alignItems: 'flex-end', gap: 8 },
  composerFieldSurface: { flex: 1, minWidth: 0, borderRadius: 26 },
  composerField: { flex: 1, width: 'auto', marginBottom: 0 },
  composerInput: { minHeight: 48, maxHeight: 120, paddingHorizontal: 16, paddingVertical: 13, backgroundColor: 'transparent', borderRadius: 24 },
  nativeSend: { width: 48, minHeight: 48, alignSelf: 'flex-end' },
  toolsButton: { width: 44, height: 46, alignItems: 'center', justifyContent: 'center', alignSelf: 'flex-end', marginBottom: 3 },
  sendButton: { width: 46, height: 46, borderRadius: 23, alignSelf: 'flex-end', marginBottom: 3,
    alignItems: 'center', justifyContent: 'center', backgroundColor: v4.buttonTop },
  sendDisabled: { backgroundColor: v4.muted },
  sendArrow: { color: v4.buttonText, fontSize: 27, fontWeight: '500', lineHeight: 32, marginTop: -2 },
  }), [v4, v4Metrics, modern, v3, navigationInset]);
}
