import React, { useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Image, Keyboard, KeyboardAvoidingView, Modal, Platform, SafeAreaView, ScrollView, StatusBar, StyleSheet, Text, View, useColorScheme } from 'react-native';
import type { Contact, RoomSummary } from '../mig33/client';
import { useMig33Session, isAvailable, type ConnectionSettings } from './session/useMig33Session';
import { connectionFromForm, connectionToForm, probeConnection } from './connection';
import { defaultConnections as defaults } from './connectionSettings';
import { RetiredLoginError } from './loginStorage';
import { loginStorage } from './nativeLoginStorage';
import { loadAppearance, saveAppearance, loadHomeTabOrder, saveHomeTabOrder } from './nativeAppearanceStorage';
import { AppearanceProvider, useAppearance, type AppearanceMode } from './appearance';
import { MotionEntrance } from './motion';
import { ModernBackdrop } from './ModernBackdrop';
import { LoginArrival } from './LoginArrival';
import { modernChatItems } from './modernMenus';
import { NativeMenuAnchor, supportsNativePresentation } from './NativePresentation';
import { NativeOverlayLayer, type OverlayAlert, type OverlaySheet } from './NativeOverlayLayer';
import { AppearanceScreen } from './screens/AppearanceScreen';
import { ConnectionChooser, LoginScreen, ManualConnectionScreen, WelcomeScreen, type ConnectionForm, type LoginForm } from './screens/AuthScreens';
import { FriendsScreen } from './screens/FriendsScreen';
import { RoomsScreen, RoomParticipantsScreen } from './screens/RoomsScreens';
import { ConversationScreen, peerDisplay, WriteScreen } from './screens/ConversationScreen';
import { ChatActionSheet, EmoticonPicker, InviteChatScreen, ParticipantPickerScreen, TextColorPicker, type ChatParticipant } from './screens/ChatActionsScreens';
import { useLegacyPages } from './pages/useLegacyPages';
import { LegacyPageScreen } from './pages/LegacyPageScreen';
import type { NativePageAction } from '../mig33/legacy-pages';
import type { TranscriptRow } from './session/model';
import { chatActions, chatActionPage, legacySelector, type ChatAction, type ChatContext } from './chatActions';
import { choosePhoto, type SelectedPhoto } from './nativePhoto';
import { useRosterActions } from './useRosterActions';
import { insertComposerText, type EditorSelection } from './composer';
import type { SlashCommandContext } from './slashCommands';
import { V4Button, V4CommandBar, V4Dialog, V4Header, V4Icon, V4Menu, V4Tabs, type V4MenuItem, type V4Tab } from './v4/primitives';

type Screen = 'welcome' | 'login' | 'connection' | 'manual' | 'friends' | 'rooms' | 'participants' | 'conversation' | 'write' | 'appearance' | 'picker' | 'invite' | 'page';
interface Menu { title: string; items: V4MenuItem[]; }
interface Picker { context: ChatContext; action: ChatAction | 'username'; title: string; returnTo: Screen; }
interface Confirmation { title: string; message: string; label?: string; testID?: string; run: () => void | Promise<void>; }

export default function Application() {
  const systemDark = useColorScheme() === 'dark';
  const [mode, setMode] = useState<AppearanceMode | null>(null);
  const [appearanceError, setAppearanceError] = useState('');
  useEffect(() => {
    let cancelled = false;
    void loadAppearance().then((saved) => { if (!cancelled) setMode(saved); })
      .catch(() => { if (!cancelled) { setMode('modern'); setAppearanceError('Appearance could not be loaded. Modern is selected.'); } });
    return () => { cancelled = true; };
  }, []);
  if (mode === null) return <SafeAreaView style={{ flex: 1, justifyContent: 'center', backgroundColor: systemDark ? '#0b151c' : '#f5f9fa' }}>
    <ActivityIndicator accessibilityLabel="Loading appearance" />
  </SafeAreaView>;
  return <AppearanceProvider mode={mode} onChange={(next) => {
    setMode(next);
    void saveAppearance(next).catch(() => setAppearanceError('Appearance changed, but could not be saved. Please select it again.'));
  }}>
    <ApplicationContent appearanceError={appearanceError} clearAppearanceError={() => setAppearanceError('')} />
  </AppearanceProvider>;
}

function ApplicationContent({ appearanceError, clearAppearanceError }: { appearanceError: string; clearAppearanceError: () => void }) {
  const { theme: v4, metrics: v4Metrics, modern, v3, mode, dark, setNavigationInset } = useAppearance();
  const nativePresentation = modern && supportsNativePresentation;
  const styles = useMemo(() => StyleSheet.create({
    safe: { flex: 1, backgroundColor: modern ? 'transparent' : v4.body },
    window: { flex: 1, backgroundColor: modern ? 'transparent' : v4.body, borderWidth: modern ? 0 : 2, borderColor: v4.border },
    navigation: { position: 'absolute', top: 0, left: 0, right: 0, zIndex: 10 },
    body: { flex: 1 }, progress: { flexDirection: 'row', alignItems: 'center', gap: 6, minHeight: 40,
      paddingHorizontal: modern ? 20 : 6, backgroundColor: v4.groupBottom, borderTopWidth: 1, borderColor: v4.groupBorder },
    progressText: { color: v4.secondaryText, fontSize: v4Metrics.smallFont },
  }), [v4, v4Metrics, modern]);
  const session = useMig33Session();
  const [screen, setScreen] = useState<Screen>('login');
  const [form, setForm] = useState<LoginForm>({ username: '', password: '', remember: false, auto: false, invisible: false });
  const [connection, setConnection] = useState<ConnectionSettings>(defaults.https);
  const [manual, setManual] = useState<ConnectionForm>(connectionToForm(defaults.https));
  const [ready, setReady] = useState(false);
  const [error, setError] = useState('');
  const [testing, setTesting] = useState('');
  const [menu, setMenu] = useState<Menu | null>(null);
  const [confirmExit, setConfirmExit] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState<string | null>(null);
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [picker, setPicker] = useState<Picker | null>(null);
  const [invite, setInvite] = useState<ChatContext | null>(null);
  const [composer, setComposer] = useState<'emoticons' | 'colors' | null>(null);
  const [photo, setPhoto] = useState<{ image: SelectedPhoto; context: ChatContext | null } | null>(null);
  const [viewPhoto, setViewPhoto] = useState<string | null>(null);
  const [photoBusy, setPhotoBusy] = useState(false);
  const [homeOrder, setHomeOrder] = useState<readonly string[]>(['friends', 'rooms']);
  const [forcedSelection, setForcedSelection] = useState<(EditorSelection & { key: string }) | null>(null);
  const editorSelections = useRef(new Map<string, EditorSelection>());
  const [hideOffline, setHideOffline] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);
  const [arriving, setArriving] = useState(false);
  const [selectedFriend, setSelectedFriend] = useState<number | 'self'>('self');
  const [collapsedGroupIds, setCollapsedGroupIds] = useState<ReadonlySet<number>>(new Set());
  const [expandedCategoryIds, setExpandedCategoryIds] = useState<ReadonlySet<number>>(new Set());
  const [roomFilter, setRoomFilter] = useState('');
  const probe = useRef<AbortController | null>(null);
  const appearanceReturn = useRef<Screen>('login');
  const pageReturn = useRef<Screen>('friends');
  const photoTarget = useRef<ChatContext | null>(null);
  const asyncNavigation = useRef(0);
  const submitting = useRef(false);
  const mounted = useRef(true);
  const wasOnline = useRef(false);
  const participantLookup = useRef<string | null>(null);
  const alertIdentities = useRef(new WeakMap<object, string>());
  const alertSequence = useRef(0);
  const currentForm = useRef(form);
  const currentNavigation = useRef({ screen, peer: session.activePeer, room: session.activeRoom, group: session.activeGroup });
  const pages = useLegacyPages({ requestOriginal: session.requestOriginal, siteURL: session.siteURL,
    online: session.online, onNativeAction: nativePageAction });
  const roster = useRosterActions(session, { openPage, openChat, showMenu: setMenu, confirm: setConfirmation,
    extendMenu: (items, category, context) => extendMenu(items, category, undefined, undefined, context?.groupId) });
  currentForm.current = form;
  currentNavigation.current = { screen, peer: session.activePeer, room: session.activeRoom, group: session.activeGroup };

  useEffect(() => {
    mounted.current = true;
    let cancelled = false;
    void loadHomeTabOrder().then((order) => { if (!cancelled) setHomeOrder(order); })
      .catch(() => { if (!cancelled) setError('Tab order could not be loaded.'); });
    void loginStorage.load().then((saved) => {
      if (cancelled) return;
      if (saved) {
        const loaded = { username: saved.username, password: saved.password, remember: true,
          auto: saved.auto, invisible: saved.invisible ?? false };
        currentForm.current = loaded;
        setForm(loaded); setConnection(saved.connection); setManual(connectionToForm(saved.connection));
        if (saved.auto) void connect(loaded, saved.connection);
      }
    }).catch((cause: unknown) => { if (!cancelled) setError(cause instanceof RetiredLoginError
      ? cause.message : 'Saved login could not be read. Please log in again.'); })
      .finally(() => { if (!cancelled) setReady(true); });
    return () => { cancelled = true; mounted.current = false; probe.current?.abort(); };
  }, []);

  useEffect(() => {
    if (session.online && !wasOnline.current) {
      setArriving(modern); setScreen('friends'); setSelectedFriend('self'); Keyboard.dismiss();
    }
    if (!session.online && wasOnline.current) {
      asyncNavigation.current++; setScreen('login'); setMenu(null); setConfirmLeave(null); setConfirmation(null);
      setPicker(null); setInvite(null); setComposer(null); setPhoto(null); setViewPhoto(null); photoTarget.current = null;
      editorSelections.current.clear(); setForcedSelection(null);
      setArriving(false);
      setCollapsedGroupIds(new Set()); setExpandedCategoryIds(new Set()); setRoomFilter('');
    }
    wasOnline.current = session.online;
  }, [session.online]);

  useEffect(() => {
    session.setConversationVisible(screen !== 'page' && screen !== 'appearance');
  }, [screen, session.setConversationVisible]);

  useEffect(() => {
    if (arriving && screen !== 'friends') setArriving(false);
  }, [arriving, screen]);

  useEffect(() => {
    if (!nativePresentation || !session.online || !session.activeRoom) { participantLookup.current = null; return; }
    const room = session.activeRoomConversation;
    if (room && !room.participantsLoaded && !session.roomAction && participantLookup.current !== room.name) {
      // Native menus describe their actions before opening. Resolve moderator
      // controls once on room entry without making requests during render.
      participantLookup.current = room.name;
      void session.loadRoomParticipants(room.name);
    }
  }, [nativePresentation, session.online, session.activeRoom, session.activeRoomConversation?.participantsLoaded, session.roomAction]);

  useEffect(() => {
    // An acknowledged leave may finish while Appearance is open. Returning
    // from it must not leave an editor pointing at a removed room.
    if (session.online && ['conversation', 'write', 'participants'].includes(screen)
      && !session.activeConversation && !session.activeRoomConversation && !session.activeGroupConversation) setScreen('rooms');
  }, [screen, session.online, session.activeConversation, session.activeRoomConversation, session.activeGroupConversation]);

  function changeForm(change: Partial<LoginForm>) {
    const next = { ...currentForm.current, ...change };
    currentForm.current = next;
    setForm(next);
    if (change.remember === false) void loginStorage.forget()
      .catch(() => setError('The saved login could not be removed. Please try again.'));
  }
  async function connect(values = form, target = connection) {
    if (submitting.current) return;
    submitting.current = true;
    setError(''); Keyboard.dismiss();
    try {
      if (!await session.login(values, target) || !mounted.current) return;
      if (values.remember && currentForm.current.remember) {
        await loginStorage.save({ version: 2, username: values.username, password: values.password,
          auto: values.auto, invisible: values.invisible, connection: target });
      } else {
        if (currentForm.current.username === values.username && currentForm.current.password === values.password) {
          const cleared = { ...currentForm.current, password: '' };
          currentForm.current = cleared;
          setForm(cleared);
        }
        await loginStorage.forget();
      }
    } catch { if (mounted.current) setError('Login succeeded, but the saved login could not be updated.'); }
    finally { submitting.current = false; }
  }
  async function testConnection(target: ConnectionSettings) {
    if (probe.current) return;
    Keyboard.dismiss();
    const active = new AbortController(); probe.current = active;
    setError(''); setTesting(`Testing Connection...${target.mode.toUpperCase()}`);
    try {
      await probeConnection(target, active.signal);
      if (!mounted.current || active.signal.aborted) return;
      setConnection(target); setManual(connectionToForm(target)); setScreen('login');
    } catch (cause) {
      if (mounted.current && !active.signal.aborted) setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (probe.current === active) probe.current = null;
      if (mounted.current) setTesting('');
    }
  }
  function applyManual() {
    try { void testConnection(connectionFromForm(manual)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : String(cause)); }
  }
  function selectTab(id: string) {
    asyncNavigation.current++;
    Keyboard.dismiss();
    if (id === 'friends') { session.selectConversation(null); setScreen('friends'); }
    else if (id === 'rooms') openRooms();
    else if (id === 'migworld') openPage(legacySelector('migworld', 'get_mobpage'));
    else if (id.startsWith('room:')) {
      const room = session.roomConversations.find((entry) => entry.name === id.slice(5));
      if (room?.state === 'joined') { session.openRoom(room.name); setScreen('conversation'); }
    }
    else if (id.startsWith('group:')) {
      const group = session.groupConversations.find((entry) => entry.id === id.slice(6));
      if (group?.state === 'joined') { session.openGroup(group.id); setScreen('conversation'); }
    }
    else if (id.startsWith('peer:')) { session.openConversation(id.slice(5)); setScreen('conversation'); }
  }
  function openChat(peer: string) { asyncNavigation.current++; session.openConversation(peer); setScreen('conversation'); }
  function openRooms() {
    asyncNavigation.current++;
    Keyboard.dismiss(); session.selectConversation(null); setScreen('rooms');
    if (session.roomCategories.length === 0 && !session.roomsLoading) void session.loadRoomCategories();
  }
  async function joinRoom(name: string) {
    if (await session.joinRoom(name) && mounted.current && currentNavigation.current.screen === 'rooms') {
      session.openRoom(name); setScreen('conversation');
    }
  }
  function participants(name: string) {
    if (session.roomAction || !session.roomConversations.some((room) => room.name === name && room.state === 'joined')) return;
    session.openRoom(name); setScreen('participants'); void session.loadRoomParticipants(name);
  }
  async function leaveRoom(name: string) {
    setConfirmLeave(null);
    if (await session.leaveRoom(name) && mounted.current) {
      const navigation = currentNavigation.current;
      if (navigation.screen === 'conversation' || navigation.screen === 'participants' || navigation.screen === 'write') {
        if (navigation.room === name || navigation.room === null && navigation.peer === null && navigation.group === null) openRooms();
      }
    }
  }
  function activeContext(): ChatContext | null {
    return session.activeRoom ? { kind: 'room', id: session.activeRoom }
      : session.activeGroup ? { kind: 'group', id: session.activeGroup }
        : session.activePeer ? { kind: 'private', id: session.activePeer } : null;
  }
  function contextParticipants(context: ChatContext): readonly ChatParticipant[] {
    return context.kind === 'room' ? session.roomConversations.find((entry) => entry.name === context.id)?.participants ?? []
      : context.kind === 'group' ? session.groupConversations.find((entry) => entry.id === context.id)?.participants ?? []
        : [{ username: context.id }];
  }
  function refreshParticipants(context: ChatContext) {
    if (context.kind === 'room') return session.loadRoomParticipants(context.id);
    if (context.kind === 'group') return session.loadGroupParticipants(context.id);
    return Promise.resolve(true);
  }
  function openPage(url: string) {
    asyncNavigation.current++;
    session.setConversationVisible(false);
    Keyboard.dismiss(); setMenu(null);
    if (screen !== 'page') { pageReturn.current = screen; pages.close(); }
    setScreen('page'); void pages.open(url);
  }
  function pageBack() {
    asyncNavigation.current++;
    if (pages.back()) return;
    pages.close(); setScreen(pageReturn.current);
  }
  async function nativePageAction(action: NativePageAction) {
    switch (action.name) {
      case 'privateChat': openChat(action.argument); return;
      case 'inviteFriend': await session.addContact(action.argument); return;
      case 'inviteToMig33': roster.openInviteToMig33(); return;
      case 'updateStatus': roster.openStatus(); return;
      case 'call': roster.openCall(action.argument); return;
      case 'sms': roster.openSMS(action.argument); return;
      case 'checkIn': openPage(legacySelector('location', 'checkin')); return;
      case 'imMenu': openPage(legacySelector('im', 'setup_im')); return;
      case 'joinChatroom':
      case 'joinGroupChatroom': {
        const name = action.name === 'joinGroupChatroom' ? action.argument.split(',')[0] : action.argument;
        const ticket = ++asyncNavigation.current;
        if (await session.joinRoom(name) && mounted.current && ticket === asyncNavigation.current) {
          session.openRoom(name); setScreen('conversation');
        }
        return;
      }
      case 'sendScrapbookPhoto': {
        const context = photoTarget.current;
        if (!context || context.kind === 'room' || !/^\d+$/.test(action.argument)) throw new Error('Open Share Photo from a private or group chat first.');
        const ticket = ++asyncNavigation.current;
        if (await session.sendPhoto(context.kind === 'private' ? 1 : 2, context.id, action.argument)
          && mounted.current && ticket === asyncNavigation.current) {
          context.kind === 'private' ? session.openConversation(context.id) : session.openGroup(context.id);
          setScreen('conversation');
        }
        return;
      }
      case 'uploadPhotoToScrapbook': showPhotoSources(null); return;
      case 'close':
      case 'closeTab': pages.close(); setScreen(pageReturn.current); return;
      case 'focusTab': {
        if (action.argument === '0') selectTab('friends');
        else if (action.argument === '1') openPage(legacySelector('lookout', 'home'));
        else if (action.argument === '2') openRooms();
        else if (action.argument === '3') {
          const context = activeContext();
          if (context) setScreen('conversation'); else activeChats();
        } else throw new Error('That tab is not open.');
        return;
      }
      case 'reorderHomeTab': {
        if (!/^[012]$/.test(action.argument)) throw new Error('Invalid tab position.');
        const order = action.argument === '0' ? ['migworld', 'friends', 'rooms'] : action.argument === '1' ? ['friends', 'rooms', 'migworld']
          : ['rooms', ...[...new Set([...homeOrder, 'migworld'])].filter((id) => id !== 'rooms')];
        setHomeOrder(order); selectTab(order[0]);
        void saveHomeTabOrder(order).catch(() => setError('Tab order changed but could not be saved.'));
        return;
      }
      default: throw new Error(`The linked action ${action.name} is not available.`);
    }
  }
  function chooseParticipant(context: ChatContext, action: ChatAction | 'username', title: string) {
    setPicker({ context, action, title, returnTo: screen === 'write' ? 'write' : 'conversation' });
    setScreen('picker'); void refreshParticipants(context);
  }
  function openInvite(context: ChatContext) {
    setInvite(context); setScreen('invite');
    if (context.kind === 'group') void session.loadGroupParticipants(context.id);
  }
  async function inviteUser(username: string) {
    if (!invite) return;
    const ticket = ++asyncNavigation.current;
    const ok = invite.kind === 'private' ? await session.createGroup(invite.id, username)
      : invite.kind === 'group' && await session.inviteToGroup(invite.id, username);
    if (ok && mounted.current && ticket === asyncNavigation.current && currentNavigation.current.screen === 'invite') setScreen('conversation');
  }
  function askAction(context: ChatContext, action: ChatAction, username?: string) {
    if (action === 'invite') { openInvite(context); return; }
    if (action === 'photo') { showPhotoSources(context); return; }
    if (action === 'write') { setScreen('write'); return; }
    const target = username ?? (context.kind === 'private' ? context.id : undefined);
    const item = chatActions(context.kind, false).find((entry) => entry.action === action);
    if (!target && (item?.target || ['mute', 'unmute'].includes(action))) {
      chooseParticipant(context, action, item?.label ?? (action === 'mute' ? 'Mute User' : 'Unmute User')); return;
    }
    const contact = target && session.contacts.find((entry) => entry.username.toLowerCase() === target.toLowerCase());
    const address = (action === 'call' || action === 'sms') && context.kind === 'private' && contact ? contact.packet.text(8) || target : target;
    const url = action === 'lookout' && contact
      ? legacySelector('lookout', 'home', { cid: String(contact.id), pres: String(contact.presence) })
      : chatActionPage(action, context, address);
    if (url) { openPage(url); return; }
    if (!target) return;
    if (action === 'private') { openChat(target); return; }
    if (action === 'add') { void session.addContact(target); return; }
    const participant = contextParticipants(context).find((entry) => entry.username.toLowerCase() === target.toLowerCase());
    // Recovered v4 Block & Mute uses417 except its already-muted room branch.
    const operation = action === 'block' && context.kind === 'room' && !modern && !v3 && participant?.muted ? 'unmute'
      : action === 'mute' && participant?.muted ? 'unmute' : action;
    if (!['kick', 'mute', 'unmute', 'block'].includes(operation)) return;
    const verb = operation === 'block' ? 'block' : operation === 'kick' ? 'kick' : operation === 'mute' ? 'mute' : 'unmute';
    setConfirmation({ title: operation === 'block' ? 'Block User' : `${verb[0].toUpperCase()}${verb.slice(1)} User`,
      message: operation === 'block' ? `Are you sure you would like to block ${target}?`
        : operation === 'kick' ? `Are you sure you would like to kick ${target}?`
          : `${verb[0].toUpperCase()}${verb.slice(1)} ${target} in ${context.id}?`,
      label: operation === 'kick' ? 'Kick' : operation === 'block' ? 'Block' : verb[0].toUpperCase() + verb.slice(1),
      run: async () => {
        const ticket = ++asyncNavigation.current;
        const ok = operation === 'block' ? await session.blockUser(target)
          : context.kind === 'room' && await session.moderateRoom(context.id, target, operation as 'kick' | 'mute' | 'unmute');
        if (ok && mounted.current && ticket === asyncNavigation.current && currentNavigation.current.screen === 'picker') setScreen('conversation');
      } });
  }
  function participantOptions(context: ChatContext, participant: ChatParticipant) {
    setMenu(participantMenuModel(context, participant));
  }
  function participantMenuModel(context: ChatContext, participant: ChatParticipant): Menu {
    const own = contextParticipants(context).find((entry) => entry.username.toLowerCase() === session.username.toLowerCase());
    const items: V4MenuItem[] = [
      { id: 'private', label: 'Private Chat', onPress: () => openChat(participant.username) },
      ...(['profile', 'add', 'gift', 'call', 'credit', 'block'] as ChatAction[]).map((action) => ({ id: action,
        label: ({ profile: 'User Profile', add: 'Add as friend', gift: 'Give a Gift', call: 'Call User', credit: 'Transfer Credit', block: 'Block messages' } as Record<string, string>)[action],
        onPress: () => askAction(context, action, participant.username), testID: `chat.action.${action}` })),
    ];
    if (context.kind === 'room' && !participant.moderator) {
      items.push({ id: 'kick', label: own?.moderator ? 'Kick User' : 'Vote to kick', disabled: !!own?.muted || !!session.roomAction,
        onPress: () => askAction(context, 'kick', participant.username), testID: 'chat.action.kick' });
      if (own?.moderator) items.push({ id: 'mute', label: participant.muted ? 'Unmute in this room' : 'Mute in this room',
        onPress: () => askAction(context, participant.muted ? 'unmute' : 'mute', participant.username),
        testID: participant.muted ? 'chat.action.unmute' : 'chat.action.mute' });
    }
    return { title: participant.username, items };
  }
  function setContextDraft(context: ChatContext, value: string) {
    if (context.kind === 'room') session.setRoomDraft(context.id, value);
    else if (context.kind === 'group') session.setGroupDraft(context.id, value);
    else session.setDraft(context.id, value);
  }
  function appendToDraft(context: ChatContext, text: string) {
    const conversation = context.kind === 'room' ? session.roomConversations.find((entry) => entry.name === context.id)
      : context.kind === 'group' ? session.groupConversations.find((entry) => entry.id === context.id)
        : session.conversations.find((entry) => entry.peer === context.id);
    if (!conversation) return;
    const key = `${context.kind === 'private' ? 'peer' : context.kind}:${context.id}`;
    const result = insertComposerText(conversation.draft, text, editorSelections.current.get(key));
    if (!result) { setError('There is not enough space in this message. Shorten it before inserting more text.'); return; }
    setContextDraft(context, result.draft);
    editorSelections.current.set(key, result.selection);
    setForcedSelection({ key, ...result.selection });
  }
  function composerTools() {
    const context = activeContext(); if (!context) return;
    setMenu(composerMenuModel(context));
  }
  function composerMenuModel(context: ChatContext): Menu {
    return { title: 'Message', items: [
      { id: 'emoticon', label: 'Insert Emoticon', testID: 'composer.open.emoticons', onPress: () => {
        Keyboard.dismiss(); setComposer('emoticons');
        for (const token of [...session.emoticons.standard, ...session.emoticons.other].slice(0, 36)) void session.loadEmoticon(token);
      } },
      { id: 'username', label: 'Insert Username', onPress: () => context.kind === 'private'
        ? appendToDraft(context, context.id) : chooseParticipant(context, 'username', 'Insert Username') },
      { id: 'color', label: 'Change Text Color', testID: 'composer.open.colors', onPress: () => { Keyboard.dismiss(); setComposer('colors'); } },
      ...(context.kind !== 'room' ? [{ id: 'photo', label: 'Share Photo', onPress: () => showPhotoSources(context),
        children: photoSourcesModel(context).items }] : []),
    ] };
  }
  function showPhotoSources(context: ChatContext | null) {
    setMenu(photoSourcesModel(context));
  }
  function photoSourcesModel(context: ChatContext | null): Menu {
    return { title: context ? 'Share Photo' : 'Upload Photo', items: [
      { id: 'library', label: 'Choose from Photos', onPress: () => { void pickPhoto('library', context); }, testID: 'photo.library' },
      { id: 'camera', label: 'Take Photo', onPress: () => { void pickPhoto('camera', context); }, testID: 'photo.camera' },
      ...(context ? [{ id: 'scrapbook', label: 'My Scrapbook', onPress: () => {
        photoTarget.current = context; openPage(legacySelector('photo', 'home', { f: 'sendfromphoto' }));
      }, testID: 'photo.scrapbook' }] : []),
    ] };
  }
  async function pickPhoto(source: 'camera' | 'library', context: ChatContext | null) {
    if (photoBusy) return;
    const ticket = ++asyncNavigation.current; setPhotoBusy(true);
    try {
      const image = await choosePhoto(source);
      if (image && mounted.current && ticket === asyncNavigation.current) setPhoto({ image, context });
    } catch (cause) { if (mounted.current && ticket === asyncNavigation.current) setError(cause instanceof Error ? cause.message : String(cause)); }
    finally { if (mounted.current) setPhotoBusy(false); }
  }
  async function sendPhoto() {
    if (!photo) return;
    const selected = photo;
    const ticket = ++asyncNavigation.current;
    if (selected.context) {
      const { kind, id } = selected.context;
      if (await session.sendPhoto(kind === 'private' ? 1 : kind === 'group' ? 2 : 3, id, selected.image.bytes)
        && mounted.current && ticket === asyncNavigation.current) setPhoto(null);
    } else {
      const url = await session.uploadPhoto(selected.image.bytes);
      if (url && mounted.current && ticket === asyncNavigation.current) { setPhoto(null); openPage(url); }
    }
  }
  async function openPhoto(row: TranscriptRow) {
    Keyboard.dismiss();
    const ticket = ++asyncNavigation.current;
    if (row.imageURI) { setViewPhoto(row.imageURI); return; }
    const reference = row.mediaReference ?? row.tokens?.find((token) => /^\[photo:\d+\]$/.test(token));
    const id = reference?.match(/^\[photo:(\d+)\]$/)?.[1] ?? reference;
    if (!id || !/^\d+$/.test(id)) return;
    try { const uri = await pages.loadImage(`/assets/photos/${id}`); if (mounted.current && ticket === asyncNavigation.current) setViewPhoto(uri); }
    catch (cause) { if (mounted.current && ticket === asyncNavigation.current) setError(cause instanceof Error ? cause.message : String(cause)); }
  }
  function isFavoriteRoom(name: string) {
    return session.roomCategories.some((category) => category.favorites
      && category.rooms.some((room) => room.name.toLowerCase() === name.toLowerCase()));
  }
  function favoriteItem(name: string): V4MenuItem {
    const favorite = isFavoriteRoom(name);
    return { id: 'favorite', label: favorite ? 'Remove from Favorites' : 'Add to Favorites',
      disabled: !!session.roomAction || !session.roomCategories.some((category) => category.favorites), testID: 'room.favorite',
      onPress: () => { void session.setRoomFavorite(name, !favorite); } };
  }
  function roomMenu(room: RoomSummary) {
    setMenu(roomMenuModel(room));
  }
  function roomMenuModel(room: RoomSummary): Menu {
    const joined = session.roomConversations.find((entry) => entry.name.toLowerCase() === room.name.toLowerCase());
    return { title: room.name, items: [
      { id: 'join', label: joined ? 'Open Room' : 'Join Room', disabled: !!session.roomAction,
        onPress: () => { void joinRoom(room.name); }, testID: 'room.join' },
      { id: 'info', label: 'Room Info', onPress: () => openPage(legacySelector('chatroom', 'setup', { roomName: room.name })), testID: 'chat.action.info' },
      favoriteItem(room.name),
      ...(joined ? [
        { id: 'participants', label: 'Participants', onPress: () => participants(joined.name), testID: 'room.participants', disabled: !!session.roomAction },
        { id: 'leave', label: 'Leave Room', onPress: () => setConfirmLeave(joined.name), testID: 'room.leave', disabled: !!session.roomAction },
      ] : []),
      appearanceItem,
    ] };
  }
  function openAppearance() { asyncNavigation.current++; session.setConversationVisible(false); Keyboard.dismiss(); appearanceReturn.current = screen; setScreen('appearance'); }
  function openLoginConnection() {
    if (busy) return;
    Keyboard.dismiss(); setManual(connectionToForm(connection)); setScreen('connection');
  }
  function loginMenuModel(): Menu {
    const setPreference = (id: 'auto' | 'invisible', value: boolean) => {
      if (busy) return;
      changeForm({ [id]: value, ...(id === 'auto' && value ? { remember: true } : {}) });
    };
    const preference = (id: 'auto' | 'invisible', label: string): V4MenuItem => ({
      id, label: nativePresentation ? label : `${label}: ${form[id] ? 'On' : 'Off'}`,
      checked: form[id], icon: form[id] ? 'check' : undefined, testID: `login.${id}`, disabled: busy,
      onCheckedChange: (value) => setPreference(id, value),
      onPress: () => setPreference(id, !currentForm.current[id]),
    });
    return { title: 'Sign-in options', items: [
      preference('auto', 'Auto Login'), preference('invisible', 'Login As Invisible'),
      { id: 'connection', label: 'Connection Settings', separator: true, testID: 'login.connection', disabled: busy, onPress: openLoginConnection },
      { id: 'appearance', label: 'Appearance', testID: 'appearance.open', disabled: busy, onPress: () => { if (!busy) openAppearance(); } },
    ] };
  }
  const appearanceItem: V4MenuItem = { id: 'appearance', label: 'Appearance', onPress: openAppearance, testID: 'appearance.open' };
  const roomsItem: V4MenuItem = { id: 'rooms', label: modern ? 'Rooms' : 'Chat Rooms', onPress: openRooms, testID: 'menu.rooms', icon: 'chat' };
  function activeChats() { setMenu(activeChatsMenu()); }
  function activeChatsMenu(): Menu {
    return { title: 'Active Chats', items: [...session.conversations.map((chat) => ({ id: `peer:${chat.peer}`,
      label: `${peerDisplay(chat.peer, session.contacts)}${chat.unread ? ' *' : ''}`,
      icon: 'conversation' as const, onPress: () => openChat(chat.peer) })),
      ...session.roomConversations.filter((room) => room.state === 'joined').map((room) => ({ id: `room:${room.name}`, label: `${room.name}${room.unread ? ' *' : ''}`,
        icon: 'chat' as const, onPress: () => selectTab(`room:${room.name}`) })),
      ...session.groupConversations.map((group) => ({ id: `group:${group.id}`, label: `Group Chat${group.unread ? ' *' : ''}`,
        icon: 'conversation' as const, onPress: () => selectTab(`group:${group.id}`) }))] };
  }
  function pageItem(id: string, label: string, controller: string, action: string, values: Record<string, string> = {}): V4MenuItem {
    return { id, label, onPress: () => openPage(legacySelector(controller, action, values)), testID: `chat.action.${id}` };
  }
  function extendMenu(items: V4MenuItem[], category: number, context?: ChatContext, contact?: Contact, groupId?: number): V4MenuItem[] {
    const result = [...items];
    for (const entry of session.serverMenus.filter((value) => value.category === category)) {
      let suffix = '';
      if (contact) suffix = `cid=${contact.id}&pres=${contact.presence}`;
      else if (groupId !== undefined) suffix = `gid=${groupId}`;
      else if (context?.kind === 'private') suffix = `imType=1&cid=${session.contacts.find((value) => value.username === context.id)?.id ?? -1}`;
      else if (context?.kind === 'group') suffix = `gcid=${encodeURIComponent(context.id)}`;
      else if (context?.kind === 'room') suffix = `roomName=${encodeURIComponent(context.id)}`;
      const item: V4MenuItem = { id: `server:${category}:${entry.index}:${entry.label}`, label: entry.label,
        onPress: () => openPage(entry.value + suffix) };
      result.splice(Math.max(0, Math.min(entry.index, result.length)), 0, item);
    }
    return result;
  }
  function chatMenu(context: ChatContext) {
    const room = context.kind === 'room' ? session.roomConversations.find((value) => value.name === context.id) : undefined;
    if (room && !room.participantsLoaded && !session.roomAction) void session.loadRoomParticipants(room.name);
    setMenu(chatMenuModel(context));
  }
  function chatMenuModel(context: ChatContext): Menu {
    const room = context.kind === 'room' ? session.roomConversations.find((value) => value.name === context.id) : undefined;
    const own = room?.participants.find((value) => value.username.toLowerCase() === session.username.toLowerCase());
    const mapped = chatActions(context.kind, v3).filter((item) => !(v3 && item.action === 'add' && context.kind === 'private'
      && session.contacts.some((contact) => contact.username === context.id))).map((item): V4MenuItem => ({
        id: item.action, label: modern && item.action === 'block' ? 'Block messages' : item.label,
        disabled: !!session.roomAction && ['kick', 'mute', 'unmute', 'block', 'add', 'invite'].includes(item.action)
          || item.action === 'kick' && !!own?.muted || item.action === 'mute' && room?.participantsLoaded && !own?.moderator,
        icon: item.target && context.kind !== 'private' ? 'arrowRight' : undefined,
        testID: `chat.action.${item.action}`, onPress: () => askAction(context, item.action),
      }));
    const extras: V4MenuItem[] = [
      ...(context.kind === 'room' ? [
        { id: 'participants', label: 'Participants', onPress: () => participants(context.id), testID: 'room.participants', disabled: !!session.roomAction },
        ...(modern ? [{ id: 'mute', label: 'Mute / unmute in this room', testID: 'chat.action.mute',
          disabled: room?.participantsLoaded && !own?.moderator || !!session.roomAction,
          onPress: () => chooseParticipant(context, 'mute', 'Mute / unmute in this room') }] : []),
        favoriteItem(context.id),
      ] : context.kind === 'group' ? [{ id: 'participants', label: 'Participants', testID: 'group.participants',
        onPress: () => chooseParticipant(context, 'private', 'Participants') }] : []),
      pageItem('blocks', 'Blocked Users', 'settings', 'block_list'),
      { id: 'chats', label: 'Active Chats', icon: 'arrowRight', onPress: activeChats, children: activeChatsMenu().items },
      ...(context.kind === 'room' ? [pageItem('search', 'Search Rooms', 'chatroom', 'search')] : []),
      pageItem('store', 'Store', 'store', 'main', { id: '111' }),
      { id: 'main', label: 'Main Menu', icon: 'arrowRight', onPress: mainMenu, children: mainMenuModel().items },
      roomsItem, appearanceItem,
      ...(context.kind === 'room' ? [
        { id: 'rejoin', label: 'Rejoin Room', testID: 'chat.action.rejoin', disabled: !!session.roomAction,
          onPress: () => { void session.rejoinRoom(context.id); } },
        { id: 'close-local', label: 'Close Local Tab', testID: 'chat.action.close-local', onPress: () => setConfirmation({
          title: 'Close Local Tab', message: `Discard this tab and its unsent message for ${context.id}? Use Leave Room to end your server membership.`, label: 'Close',
          run: () => { session.closeRoom(context.id); openRooms(); },
        }) },
        { id: 'leave', label: 'Leave Room', destructive: true, onPress: () => setConfirmLeave(context.id), testID: 'room.leave', disabled: !!session.roomAction },
      ] : [{ id: 'close', label: context.kind === 'group' && modern ? 'Leave Group Chat' : 'Close Chat',
        testID: context.kind === 'group' ? 'group.leave' : 'chat.close', destructive: true, onPress: () => {
          if (context.kind === 'private') { session.closeConversation(context.id); setScreen('friends'); }
          else setConfirmation({ title: 'Leave Group Chat', message: 'Leave this group chat? Your unsent message will be discarded.',
            label: 'Leave', testID: 'group.leave.confirm', run: async () => {
              const ticket = ++asyncNavigation.current;
              if (await session.leaveGroup(context.id) && ticket === asyncNavigation.current) setScreen('friends');
            } });
        } }]),
    ];
    // Preserve the compact recovered v3 menu; broader v4 functionality remains
    // reachable through More Options in this presentation.
    const items: V4MenuItem[] = [
      ...(room?.communityId ? [{ id: 'community', label: 'Group Menu', icon: 'arrowRight' as const, testID: 'chat.action.community', onPress: () => setMenu({
        title: 'Group Menu', items: [
          pageItem('group-home', 'Group Home', 'group', 'home', { cid: String(room.communityId) }),
          pageItem('group-rooms', 'Group Chat Rooms', 'group', 'list_chatrooms', { cid: String(room.communityId) }),
          pageItem('group-forum', 'Group Forum', 'group', 'view_forum', { cid: String(room.communityId) }),
          pageItem('group-members', 'Group Members', 'group', 'members', { cid: String(room.communityId) }),
        ],
      }) }] : []),
      ...(!modern ? [{ id: 'write', label: 'Write', onPress: () => setScreen('write') }] : []), ...mapped,
      ...(v3 ? [{ id: 'more', label: 'More Options', testID: 'chat.more', icon: 'arrowRight' as const, onPress: () => setMenu({ title: 'More Options', items: [
        ...chatActions(context.kind, false).filter((item) => !mapped.some((value) => value.id === item.action)).map((item) => ({
          id: item.action, label: item.label, testID: `chat.action.${item.action}`, onPress: () => askAction(context, item.action),
        })), ...extras,
      ] }) }, ...(context.kind === 'room' ? extras.filter((item) => ['participants', 'favorite', 'leave'].includes(item.id))
        : extras.filter((item) => item.id === 'close')), appearanceItem] : extras),
    ];
    const extended = extendMenu(items, context.kind === 'room' ? 11 : context.kind === 'group' ? 10 : 9, context);
    return { title: context.kind === 'room' ? context.id : context.kind === 'group' ? 'Group Chat' : 'Menu',
      items: modern ? modernChatItems(extended, context.kind, (title, items) => setMenu({ title, items })) : extended };
  }
  function friendsMenu() { setMenu(friendsMenuModel()); }
  function friendsMenuModel(): Menu {
    return { title: 'My Friends', items: extendMenu([
      ...roster.friendsItems(),
      { id: 'offline', label: hideOffline ? 'Show Offline Friends' : 'Hide Offline Friends', onPress: () => setHideOffline(!hideOffline) },
      pageItem('updates', 'Updates', 'lookout', 'home'),
    ], 2) };
  }
  function v3RosterMenu(contact?: Contact) {
    setMenu({ title: 'Menu', items: [
      ...(contact ? [{ id: 'chat', label: 'Chat', disabled: !isAvailable(contact.presence),
        onPress: isAvailable(contact.presence) ? () => openChat(contact.username) : undefined }] : []),
      pageItem('call', 'Make Call', 'call', 'home', { f: 'call', number_to_call: contact?.username ?? '' }),
      pageItem('sms', 'Send SMS', 'sms', 'home', { f: 'sms', to_number: contact?.username ?? '' }),
      { id: 'invite-mig33', label: 'Invite Friend', onPress: roster.openInviteToMig33 },
      pageItem('scrapbook', 'My Scrapbook', 'photo', 'home'),
      ...(contact ? [pageItem('profile', 'View Profile', 'profile', 'home', { username: contact.username }), ...roster.contactExtraItems(contact)] : []),
      pageItem('community', 'mig33 Community', 'group', 'list_groups_home'),
      ...roster.friendsItems(), ...roster.selfItems().filter((item) => !['create-contact-group', 'settings'].includes(item.id)),
      pageItem('my-profile', 'My Profile', 'profile', 'home', { username: session.username }),
      pageItem('account', 'My Account', 'account', 'home'), pageItem('im', 'Other IM', 'im', 'setup_im'),
      { id: 'settings', label: 'Settings', onPress: settingsMenu },
      roomsItem,
      appearanceItem,
      { id: 'exit', label: 'Log out', onPress: () => setConfirmExit(true), testID: 'menu.exit' },
    ] });
  }
  function mainMenu() { setMenu(mainMenuModel()); }
  function mainMenuModel(): Menu {
    return { title: 'Main Menu', items: [
      pageItem('migworld', 'migWorld', 'migworld', 'get_mobpage'),
      { id: 'mymig', label: 'My mig', icon: 'arrowRight', onPress: () => setMenu({ title: 'My mig', items: [
        pageItem('profile', 'My Profile', 'profile', 'home', { username: session.username }),
        pageItem('account', 'My Account', 'account', 'home'), pageItem('scrapbook', 'My Scrapbook', 'photo', 'home'),
        pageItem('avatar', 'Display Picture', 'avatar', 'home'), pageItem('lookout', 'Lookout', 'lookout', 'home'),
        pageItem('mail', 'Mail', 'email', 'compose'), pageItem('blog', 'My Blog', 'profile', 'user_blog_posts', { username: session.username }),
      ] }) },
      { id: 'friends', label: 'My Friends', icon: 'arrowRight', onPress: friendsMenu, children: friendsMenuModel().items },
      roomsItem,
      { id: 'invite-mig33', label: 'Invite 2 mig33', onPress: roster.openInviteToMig33, testID: 'main.invite' },
      pageItem('groups', 'Groups', 'group', 'list_groups_home'), pageItem('games', 'Games', 'bot', 'bot_list'),
      pageItem('store', 'Store', 'store', 'main', { id: '111' }), pageItem('leaderboard', 'Leaderboard', 'leaderboard', 'home'),
      pageItem('merchant', 'Merchant Center', 'merchant', 'dashboard'),
      { id: 'calls', label: 'Call / SMS', separator: true, onPress: () => setMenu({ title: 'Call / SMS', items: [
        pageItem('call', 'Call', 'call', 'home', { f: 'call' }), pageItem('sms', 'SMS', 'sms', 'home', { f: 'sms' }),
      ] }) },
      pageItem('find', 'Search New Friends', 'profile', 'search'),
      { id: 'settings', label: 'Settings', onPress: settingsMenu, children: settingsMenuModel().items }, pageItem('help', 'Help', 'help', 'main'),
      appearanceItem,
      { id: 'exit', label: 'Log out', onPress: () => setConfirmExit(true), testID: 'menu.exit' },
    ] };
  }
  function settingsMenu() { setMenu(settingsMenuModel()); }
  function settingsMenuModel(): Menu {
    return { title: 'Settings', items: extendMenu([
      ...roster.settingsItems(), appearanceItem,
    ], 6) };
  }
  function contactMenu(contact: Contact, contextual: boolean) {
    setSelectedFriend(contact.id);
    if (!contextual && isAvailable(contact.presence)) { openChat(contact.username); return; }
    if (v3) { v3RosterMenu(contact); return; }
    setMenu(contactMenuModel(contact));
  }
  function contactMenuModel(contact: Contact): Menu {
    return { title: contact.display || contact.username, items: extendMenu([
      { id: 'private', label: isAvailable(contact.presence) ? 'Private Chat' : 'Send Email',
        onPress: isAvailable(contact.presence) ? () => openChat(contact.username)
          : () => openPage(legacySelector('email', 'compose', { username: contact.username })), icon: 'conversation' },
      ...(['call', 'sms', 'credit', 'profile'] as ChatAction[]).map((action) => ({
        id: action, label: ({ call: 'Call', sms: 'SMS', credit: 'Transfer Credit', profile: 'View Profile' } as Record<string, string>)[action],
        onPress: () => askAction({ kind: 'private', id: contact.username }, action),
      })),
      { id: 'more', label: 'More Options', icon: 'arrowRight', onPress: () => setMenu({ title: contact.username, items: roster.contactExtraItems(contact) }) },
      { id: 'main', label: 'Main Menu', separator: true, icon: 'arrowRight', onPress: mainMenu, children: mainMenuModel().items },
      roomsItem,
      appearanceItem,
      { id: 'exit', label: 'Log out', onPress: () => setConfirmExit(true), testID: 'menu.exit' },
    ], 4, undefined, contact) };
  }
  function openMenu() {
    showMenu(false);
  }
  function showMenu(forceSelf: boolean) {
    Keyboard.dismiss();
    if (v3 && !['rooms', 'conversation', 'participants', 'write', 'picker', 'invite', 'page'].includes(screen)) {
      const contact = !forceSelf && selectedFriend !== 'self' ? session.contacts.find((entry) => entry.id === selectedFriend) : undefined;
      v3RosterMenu(contact); return;
    }
    if (['conversation', 'participants', 'write', 'picker', 'invite'].includes(screen)) {
      const context = activeContext();
      if (context) { chatMenu(context); return; }
    }
    setMenu(navigationMenuModel(forceSelf));
  }
  /** Pure descriptions let SwiftUI own each menu at its actual trigger. */
  function navigationMenuModel(forceSelf = false): Menu {
    if (screen === 'rooms') {
      return { title: 'Chat Rooms', items: extendMenu([
        { ...pageItem('search', 'Search Rooms', 'chatroom', 'search'), testID: 'rooms.search' },
        { ...pageItem('create', 'Create Room', 'chatroom', 'create'), testID: 'rooms.create' },
        { id: 'chats', label: 'Active Chats', onPress: activeChats, children: activeChatsMenu().items },
        { id: 'refresh', label: 'Refresh Rooms', disabled: session.roomsLoading || !!session.roomAction,
          onPress: () => { void session.loadRoomCategories(); } },
        { id: 'main', label: 'Main Menu', onPress: mainMenu, children: mainMenuModel().items },
        appearanceItem,
        { id: 'exit', label: 'Log out', onPress: () => setConfirmExit(true), testID: 'menu.exit' },
      ], 8) };
    }
    if (['conversation', 'participants', 'write', 'picker', 'invite'].includes(screen)) {
      const context = activeContext();
      if (context) return chatMenuModel(context);
    }
    if (screen === 'page') {
      return { title: pages.page?.title ?? 'Page', items: extendMenu([
        { id: 'refresh', label: 'Refresh', onPress: () => { void pages.refresh(); }, disabled: pages.loading },
        { id: 'close', label: 'Close Page', onPress: () => { pages.close(); setScreen(pageReturn.current); } },
        roomsItem, appearanceItem,
      ], 7) };
    }
    if (!modern && !forceSelf && screen === 'friends' && selectedFriend !== 'self') {
      const contact = session.contacts.find((entry) => entry.id === selectedFriend);
      if (contact) return contactMenuModel(contact);
    }
      return { title: 'My Friends', items: extendMenu([
        ...roster.selfItems(),
        pageItem('profile', 'My Profile', 'profile', 'home', { username: session.username }),
        { id: 'friends', label: 'My Friends', separator: true, icon: 'arrowRight', onPress: friendsMenu, children: friendsMenuModel().items },
        { id: 'refresh', label: 'Refresh Contact List', onPress: () => { void session.syncContacts(); } },
        { id: 'chats', label: 'Active Chats', icon: 'arrowRight', disabled: session.conversations.length + session.roomConversations.length + session.groupConversations.length === 0, onPress: activeChats, children: activeChatsMenu().items },
        { id: 'main', label: 'Main Menu', icon: 'arrowRight', onPress: mainMenu, children: mainMenuModel().items },
        roomsItem,
        appearanceItem,
        { id: 'exit', label: 'Log out', onPress: () => setConfirmExit(true), testID: 'menu.exit' },
      ], 1) };
  }

  async function logout() {
    setConfirmExit(false); setLoggingOut(true); await session.logout();
    if (mounted.current) setLoggingOut(false);
  }
  async function send() {
    const peer = session.activePeer;
    const room = session.activeRoom;
    const group = session.activeGroup;
    const sent = room ? await session.sendRoom(room) : group ? await session.sendGroup(group) : await session.send();
    if (sent && mounted.current && currentNavigation.current.screen === 'write'
      && currentNavigation.current.peer === peer && currentNavigation.current.room === room && currentNavigation.current.group === group) {
      Keyboard.dismiss(); setScreen('conversation');
    }
  }
  function dismissError() { setError(''); session.clearError(); clearAppearanceError(); }

  const busy = session.busy || !ready;
  const conversation = session.activeRoomConversation ?? session.activeGroupConversation ?? session.activeConversation;
  const conversationKey = session.activeRoom ? `room:${session.activeRoom}` : session.activeGroup ? `group:${session.activeGroup}` : `peer:${session.activePeer}`;
  const changeDraft = (text: string) => {
    setForcedSelection(null);
    if (session.activeRoom) session.setRoomDraft(session.activeRoom, text);
    else if (session.activeGroup) session.setGroupDraft(session.activeGroup, text);
    else if (session.activePeer) session.setDraft(session.activePeer, text);
  };
  const selection = forcedSelection?.key === conversationKey ? { start: forcedSelection.start, end: forcedSelection.end } : undefined;
  const changeSelection = (value: EditorSelection) => {
    editorSelections.current.set(conversationKey, value);
    if (forcedSelection?.key === conversationKey && forcedSelection.start === value.start && forcedSelection.end === value.end) {
      setForcedSelection(null); // Native editor has applied the inserted caret; return control to touch input.
    }
  };
  const applyDraft = (text: string, value: EditorSelection) => {
    changeDraft(text);
    editorSelections.current.set(conversationKey, value);
    setForcedSelection({ key: conversationKey, ...value });
  };
  const commandContext: SlashCommandContext = session.activeRoomConversation ? {
    kind: 'room', joined: session.activeRoomConversation.state === 'joined',
    communityID: session.activeRoomConversation.communityId,
    muted: session.activeRoomConversation.participants.find((person) => person.username.toLowerCase() === session.username.toLowerCase())?.muted,
  } : session.activeGroupConversation ? { kind: 'group', joined: session.activeGroupConversation.state === 'joined' }
    : { kind: 'private' };
  const sending = session.sending || !!session.activeRoomConversation && session.activeRoomConversation.state !== 'joined'
    || !!session.activeGroupConversation && session.activeGroupConversation.state !== 'joined';
  const tabs: V4Tab[] = [...(v3 ? ['friends'] : homeOrder).map((id): V4Tab => ({ id,
    label: id === 'friends' ? v3 ? 'Friends' : 'My Friends' : id === 'rooms' ? modern ? 'Rooms' : 'Chat Rooms' : 'migWorld',
    icon: id === 'friends' ? 'friends' : id === 'rooms' ? 'chat' : 'page' })),
    ...session.conversations.map((chat): V4Tab => ({ id: `peer:${chat.peer}`,
      label: peerDisplay(chat.peer, session.contacts), icon: 'conversation', unread: chat.unread > 0 })),
    ...session.roomConversations.filter((room) => room.state !== 'joining').map((room): V4Tab => ({ id: `room:${room.name}`, label: room.name, icon: 'chat', unread: room.unread > 0 })),
    ...session.groupConversations.map((group): V4Tab => ({ id: `group:${group.id}`, label: 'Group Chat', icon: 'conversation', unread: group.unread > 0 }))];
  const selectedTab = screen === 'friends' ? 'friends' : screen === 'rooms' ? 'rooms'
    : screen === 'page' ? pages.page?.url.includes('c=migworld') ? 'migworld' : 'page' : conversationKey;
  const progress = testing || (photoBusy ? 'Preparing photo…' : loggingOut ? 'Logging out...' : session.syncing ? 'Downloading friend list...'
    : session.state === 'connecting' ? 'Connecting...' : session.state === 'authenticating' ? 'Logging in...' : '');
  const title = screen === 'page' ? pages.page?.title ?? 'Loading…' : screen === 'picker' ? picker?.title ?? 'Participants'
    : screen === 'invite' ? 'Add to Chat' : screen === 'rooms' ? 'Chat Rooms' : screen === 'participants' ? 'Participants' : screen === 'appearance' ? 'Appearance' : screen === 'welcome' ? 'Welcome to mig33' : screen === 'connection' || screen === 'manual'
    ? (v3 ? 'Connection Type' : 'Connection Settings') : screen === 'write' ? 'Write Message:' : 'Login';

  const displayedError = error || session.error || appearanceError;
  const errorSource = error ? 'app' : session.error ? 'session' : 'appearance';
  const errorIdentity = useMemo(() => ({}), [displayedError, errorSource]);
  const exitIdentity = useMemo(() => ({}), [confirmExit]);
  const leaveIdentity = useMemo(() => ({}), [confirmLeave]);
  function alertIdentity(kind: string, value: object): string {
    let key = alertIdentities.current.get(value);
    if (!key) { key = `${kind}:${++alertSequence.current}`; alertIdentities.current.set(value, key); }
    return key;
  }
  const nativeAlert: OverlayAlert | null = displayedError ? {
    key: alertIdentity('error', errorIdentity), title: 'Error', message: displayedError, onClose: dismissError,
  } : session.notice ? {
    key: alertIdentity('notice', session.notice), title: session.notice.title, message: session.notice.text, onClose: session.clearNotice,
    actions: [{ label: 'OK', testID: 'notice.dismiss', onPress: session.clearNotice }],
  } : confirmation ? {
    key: alertIdentity('confirmation', confirmation), title: confirmation.title, message: confirmation.message, onClose: () => setConfirmation(null),
    actions: [{ label: confirmation.label ?? 'Confirm', testID: confirmation.testID ?? 'action.confirm',
      destructive: /^(Delete|Kick|Leave|Block|Close)/.test(confirmation.label ?? ''),
      onPress: () => { const action = confirmation.run; setConfirmation(null); void action(); } },
      { label: 'Cancel', cancel: true, onPress: () => setConfirmation(null) }],
  } : confirmExit ? {
    key: alertIdentity('exit', exitIdentity), title: 'Log out', message: 'Log out of mig33?', onClose: () => setConfirmExit(false),
    actions: [{ label: 'Log out', testID: 'dialog.confirm', onPress: () => { void logout(); } },
      { label: 'Cancel', testID: 'dialog.cancel', cancel: true, onPress: () => setConfirmExit(false) }],
  } : confirmLeave !== null ? {
    key: alertIdentity('leave', leaveIdentity), title: 'Leave Room', onClose: () => setConfirmLeave(null),
    message: `Leave ${confirmLeave}?${session.roomConversations.find((room) => room.name === confirmLeave)?.draft.trim() ? ' Your unsent message will be discarded.' : ''}`,
    actions: [{ label: 'Leave', destructive: true, testID: 'room.leave.confirm', onPress: () => { void leaveRoom(confirmLeave); } },
      { label: 'Cancel', cancel: true, onPress: () => setConfirmLeave(null) }],
  } : null;
  const composerContent = composer === 'colors' ? <TextColorPicker colors={session.chatColors} selected={session.selectedTextColor}
    onSelect={(color) => { session.setTextColor(color); setComposer(null); }} />
    : <EmoticonPicker items={[...new Set([...session.emoticons.standard, ...session.emoticons.other])].map((token) => ({
      name: session.emoticonAssets[token]?.label || token, shortcut: token, imageURI: session.emoticonAssets[token]?.imageURI,
    }))} onSelect={(token) => { const context = activeContext(); if (context) appendToDraft(context, token); setComposer(null); }} />;
  const photoContent = <>
    <ScrollView minimumZoomScale={1} maximumZoomScale={4} contentContainerStyle={{ flexGrow: 1 }}>
      <Image testID="photo.preview" accessibilityLabel="Photo" source={{ uri: photo?.image.uri ?? viewPhoto ?? '' }} resizeMode="contain"
        style={{ width: '100%', height: 340 }} />
    </ScrollView>
    {photo && <V4CommandBar right={{ label: session.sending ? 'Sending…' : 'Send', disabled: session.sending,
      testID: 'photo.send', onPress: () => { void sendPhoto(); } }} />}
  </>;
  const nativeSheet: OverlaySheet | null = composer ? {
    key: `composer:${composer}`, title: composer === 'colors' ? 'Text Color' : 'Emoticons',
    content: composerContent, onClose: () => setComposer(null),
  } : photo || viewPhoto ? {
    key: photo ? 'photo:send' : 'photo:view', title: photo ? photo.context ? 'Send Photo' : 'Upload Photo' : 'Photo',
    content: photoContent, dismissible: !session.sending,
    onClose: () => { if (!session.sending) { setPhoto(null); setViewPhoto(null); } },
  } : menu ? { key: 'menu', title: menu.title, menu: menu.items, onClose: () => setMenu(null) } : roster.nativePrompt;
  const nativeNavigationMenu = nativePresentation && session.online ? navigationMenuModel() : null;
  const loginMenu = modern && screen === 'login' ? loginMenuModel() : null;
  const composerContext = activeContext();
  const nativeComposerMenu = nativePresentation && composerContext ? composerMenuModel(composerContext) : null;
  function optionsAnchor(model: Menu, testID: string, label: string, disabled = false) {
    return <NativeMenuAnchor items={model.items} title={model.title} testID={testID}
      accessibilityLabel={label} systemImage="ellipsis" variant="plain" disabled={disabled} />;
  }
  return <View style={{ flex: 1, backgroundColor: v4.body }} testID={`theme.${mode}`}>
    {modern && <ModernBackdrop variant={['welcome', 'login', 'connection', 'manual'].includes(screen) ? 'auth'
      : screen === 'conversation' ? 'chat' : 'home'} />}
    <SafeAreaView style={styles.safe}>
    <StatusBar barStyle={dark ? 'light-content' : 'dark-content'} />
    <KeyboardAvoidingView style={styles.window} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      {session.online && !['login', 'welcome', 'write', 'appearance'].includes(screen) && !(v3 && ['rooms', 'participants', 'picker', 'invite', 'page'].includes(screen))
        ? modern ? <View style={styles.navigation} pointerEvents="box-none"
          onLayout={(event) => setNavigationInset(Math.ceil(event.nativeEvent.layout.height) + 12)}>
          <MotionEntrance fade={false}><V4Tabs tabs={tabs} selected={selectedTab} animateIn={arriving}
            menuControl={nativeNavigationMenu ? <NativeMenuAnchor items={nativeNavigationMenu.items}
              title={nativeNavigationMenu.title} testID="command.menu" accessibilityLabel="Open menu"
              systemImage="ellipsis" variant="glass" /> : undefined}
            onSelect={selectTab} onMenu={openMenu} /></MotionEntrance>
        </View> : <V4Tabs tabs={tabs} selected={selectedTab} onSelect={selectTab} />
        : !(modern && ['login', 'welcome', 'appearance', 'connection', 'manual'].includes(screen)) && <V4Header title={title} />}
      <MotionEntrance style={styles.body} transitionKey={`${screen}:${conversationKey}`} fade={false}>
        {screen === 'appearance' && <AppearanceScreen />}
        {screen === 'page' && session.online && <LegacyPageScreen browser={pages}
          onViewPhoto={nativePresentation ? (uri) => { Keyboard.dismiss(); setViewPhoto(uri); } : undefined} />}
        {screen === 'welcome' && <WelcomeScreen onLogin={() => setScreen('login')} />}
        {screen === 'login' && <LoginScreen form={form} onChange={changeForm} busy={busy}
          onAppearance={openAppearance}
          menuControl={loginMenu ? nativePresentation ? <NativeMenuAnchor items={loginMenu.items}
            title={loginMenu.title} testID="login.menu" accessibilityLabel="Sign-in options"
            systemImage="ellipsis" variant="glass" disabled={busy} />
            : <V4Button label="⋯" accessibilityLabel="Sign-in options" testID="login.menu" variant="secondary"
              disabled={busy} onPress={() => { Keyboard.dismiss(); setMenu(loginMenu); }} /> : undefined}
          showOffline={!hideOffline} onShowOfflineChange={(show) => setHideOffline(!show)}
          onLogin={() => { void connect(); }} onConnection={openLoginConnection} />}
        {screen === 'connection' && <ConnectionChooser busy={!!testing}
          onChoose={(mode) => { void testConnection(connection.mode === mode ? connection : defaults[mode]); }}
          onManual={() => setScreen('manual')} />}
        {screen === 'manual' && <ManualConnectionScreen form={manual} busy={!!testing}
          onChange={(change) => setManual((before) => ({ ...before, ...change }))} onConnect={applyManual} />}
        {screen === 'friends' && session.online && <LoginArrival active={arriving} onComplete={() => setArriving(false)}>
          <FriendsScreen username={session.username} presence={session.presence}
          ownStatus={session.ownStatus}
          contacts={session.contacts} groups={session.groups} hideOffline={hideOffline} onContact={contactMenu}
          collapsedGroupIds={collapsedGroupIds} onCollapsedGroupIdsChange={setCollapsedGroupIds}
          renderContactOptions={nativePresentation ? (contact) => optionsAnchor(contactMenuModel(contact),
            `friends.options.${contact.username}`, `Options for ${contact.display || contact.username}`) : undefined}
          renderSelfOptions={nativePresentation ? () => optionsAnchor(navigationMenuModel(true), 'friends.self', 'Your account options') : undefined}
          onRooms={openRooms}
          selected={selectedFriend} onSelf={() => { setSelectedFriend('self'); showMenu(true); }} />
        </LoginArrival>}
        {screen === 'rooms' && session.online && <RoomsScreen categories={session.roomCategories} loading={session.roomsLoading}
          expandedCategoryIds={expandedCategoryIds} onExpandedCategoryIdsChange={setExpandedCategoryIds}
          filter={roomFilter} onFilterChange={setRoomFilter} onSearch={() => openPage(legacySelector('chatroom', 'search'))}
          renderRoomOptions={nativePresentation ? (room) => optionsAnchor(roomMenuModel(room), `rooms.more.${room.name}`,
            `Options for ${room.name}`, !!session.roomAction) : undefined}
          action={session.roomAction} onExpand={(id) => { void session.loadRoomCategory(id); }}
          onMore={(id) => { void session.loadRoomCategory(id); }}
          onJoin={(name) => { void joinRoom(name); }} onRoomMenu={roomMenu}
          onRefresh={() => { void session.loadRoomCategories(); }} />}
        {screen === 'participants' && session.activeRoomConversation && <RoomParticipantsScreen room={session.activeRoomConversation.name}
          participants={session.activeRoomConversation.participants} username={session.username}
          loading={!!session.roomAction} onRefresh={() => { void session.loadRoomParticipants(session.activeRoomConversation!.name); }}
          onSelect={(username) => { if (username.toLowerCase() !== session.username.toLowerCase()) openChat(username); }}
          renderParticipantOptions={nativePresentation ? (participant) => optionsAnchor(participantMenuModel({
            kind: 'room', id: session.activeRoomConversation!.name }, participant), `participants.options.${participant.username}`,
          `Options for ${participant.username}`, !!session.roomAction) : undefined}
          onOptions={(participant) => participantOptions({ kind: 'room', id: session.activeRoomConversation!.name }, participant)} />}
        {screen === 'picker' && picker && <ParticipantPickerScreen title={picker.title} context={picker.context.kind === 'group' ? 'Group Chat' : picker.context.id}
          username={session.username} participants={contextParticipants(picker.context)} loading={!!session.roomAction}
          allowSelf={picker.action === 'username' || picker.action === 'profile'}
          disabledUsernames={['kick', 'mute', 'unmute'].includes(picker.action)
            ? contextParticipants(picker.context).filter((entry) => entry.moderator).map((entry) => entry.username) : []}
          mutedAnnotation={['mute', 'block'].includes(picker.action) ? '(Un-mute)' : undefined}
          onRefresh={() => { void refreshParticipants(picker.context); }}
          onOptions={picker.action === 'private' ? (participant) => participantOptions(picker.context, participant) : undefined}
          renderParticipantOptions={nativePresentation && picker.action === 'private' ? (participant) => optionsAnchor(
            participantMenuModel(picker.context, participant), `participant.options.${participant.username}`,
            `Options for ${participant.username}`, !!session.roomAction) : undefined}
          onSelect={(username) => {
            if (picker.action === 'username') { appendToDraft(picker.context, username); setScreen(picker.returnTo); }
            else askAction(picker.context, picker.action, username);
          }} />}
        {screen === 'invite' && invite && <InviteChatScreen key={`${invite.kind}:${invite.id}`} contacts={session.contacts}
          excludedUsernames={[session.username, ...(invite.kind === 'private' ? [invite.id] : contextParticipants(invite).map((entry) => entry.username))]}
          busy={!!session.roomAction} context={invite.kind === 'private' ? `Start a group with ${invite.id}` : 'Invite someone to this group chat'}
          onInvite={(username) => { void inviteUser(username); }} />}
        {screen === 'conversation' && conversation && <ConversationScreen key={conversationKey} conversation={conversation}
          title={session.activeGroup ? 'Group Chat' : undefined}
          contacts={session.activeRoom || session.activeGroup ? [] : session.contacts} onWrite={() => setScreen('write')} sending={sending}
          onChange={changeDraft} onSend={() => { void send(); }} onTools={composerTools}
          toolsControl={nativeComposerMenu ? <NativeMenuAnchor items={nativeComposerMenu.items}
            testID="chat.tools" accessibilityLabel="Message tools" systemImage="plus" variant="glass" /> : undefined}
          selection={selection} onSelectionChange={changeSelection} commandContext={commandContext} onApplyDraft={applyDraft}
          emoticonAssets={session.emoticonAssets} onLoadMedia={(token) => { void session.loadEmoticon(token); }}
          onMediaOpen={(row) => { void openPhoto(row); }} />}
        {screen === 'write' && conversation && <WriteScreen key={conversationKey} conversation={conversation} sending={sending}
          onChange={changeDraft} onTools={composerTools} selection={selection} onSelectionChange={changeSelection}
          commandContext={commandContext} onApplyDraft={applyDraft} />}
      </MotionEntrance>
      {progress !== '' && <View style={styles.progress} accessibilityLiveRegion="polite">
        <ActivityIndicator size="small" color={v4.secondaryText} /><Text style={styles.progressText}>{progress}</Text>
      </View>}
      {screen === 'appearance' ? <V4CommandBar right={{ label: 'Back', testID: 'appearance.back', onPress: () => setScreen(appearanceReturn.current) }} />
        : screen === 'page' ? <V4CommandBar left={modern ? undefined : { label: 'Menu', testID: 'command.menu', onPress: openMenu }}
          right={{ label: 'Back', testID: 'page.back', onPress: pageBack }} />
        : screen === 'picker' ? <V4CommandBar right={{ label: 'Back', testID: 'participant.back', onPress: () => { asyncNavigation.current++; setScreen(picker?.returnTo ?? 'conversation'); } }} />
        : screen === 'invite' ? <V4CommandBar right={{ label: 'Back', testID: 'invite.back', onPress: () => { asyncNavigation.current++; setScreen('conversation'); } }} />
        : screen === 'participants' ? <V4CommandBar right={{ label: 'Back', testID: 'participants.back', onPress: () => setScreen('conversation') }} />
        : screen === 'rooms' && v3 ? <V4CommandBar left={{ label: 'Menu', testID: 'command.menu', onPress: openMenu }}
          right={{ label: 'Back', testID: 'rooms.back', onPress: () => selectTab('friends') }} />
        : progress !== '' ? <V4CommandBar right={{ label: 'Back', disabled: loggingOut, onPress: () => {
        probe.current?.abort(); if (session.busy || session.syncing) session.cancel();
      } }} /> : screen === 'write' ? <V4CommandBar
        left={{ label: 'Send', testID: 'chat.send', onPress: () => { void send(); }, disabled: sending || !conversation?.draft.trim() }}
        right={{ label: 'Back', testID: 'chat.back', onPress: () => { Keyboard.dismiss(); setScreen('conversation'); } }} />
        : session.online ? (modern ? null : <V4CommandBar left={screen === 'conversation' ? { label: 'Menu', onPress: openMenu, testID: 'command.menu' } : undefined}
          right={screen === 'conversation' ? { label: 'Write', testID: 'chat.write', onPress: () => setScreen('write') }
            : { label: 'Menu', onPress: openMenu, testID: 'command.menu' }} />)
          : !(modern && (screen === 'login' || screen === 'welcome')) && <V4CommandBar right={{ label: 'Back', disabled: busy, onPress: () => {
            Keyboard.dismiss(); setScreen(screen === 'manual' ? 'connection' : screen === 'login' ? 'welcome' : 'login');
          } }} />}
    </KeyboardAvoidingView>
    {nativePresentation && <NativeOverlayLayer sheet={nativeSheet} alert={nativeAlert} />}
    <Modal visible={!nativePresentation && (menu !== null || confirmExit || confirmLeave !== null || confirmation !== null || composer !== null || photo !== null || viewPhoto !== null || roster.prompt !== null || !!session.notice || !!displayedError)} transparent animationType="none"
      onRequestClose={() => { setMenu(null); setConfirmExit(false); setConfirmLeave(null); setConfirmation(null); setComposer(null); setPhoto(null); setViewPhoto(null); roster.closePrompt(); session.clearNotice(); dismissError(); }}>
      {displayedError
        ? <V4Dialog embedded visible title="Error" message={displayedError} onClose={dismissError} />
        : session.notice ? <V4Dialog embedded visible title={session.notice.title} message={session.notice.text} onClose={session.clearNotice}
          actions={[{ label: 'OK', testID: 'notice.dismiss', onPress: session.clearNotice }]} />
        : confirmation ? <V4Dialog embedded visible title={confirmation.title} message={confirmation.message} onClose={() => setConfirmation(null)}
          actions={[{ label: confirmation.label ?? 'Confirm', testID: confirmation.testID ?? 'action.confirm', onPress: () => {
            const action = confirmation.run; setConfirmation(null); void action();
          } }, { label: 'Cancel', onPress: () => setConfirmation(null) }]} />
        : composer ? <ChatActionSheet embedded visible title={composer === 'colors' ? 'Text Color' : 'Emoticons'} onClose={() => setComposer(null)}>
          {composerContent}
        </ChatActionSheet>
        : photo || viewPhoto ? <ChatActionSheet embedded visible title={photo ? photo.context ? 'Send Photo' : 'Upload Photo' : 'Photo'}
          onClose={() => { if (!session.sending) { setPhoto(null); setViewPhoto(null); } }}>
          {photoContent}
        </ChatActionSheet>
        : confirmExit ? <V4Dialog embedded visible title="Log out" message="Log out of mig33?" onClose={() => setConfirmExit(false)}
          actions={[{ label: 'Log out', testID: 'dialog.confirm', onPress: () => { void logout(); } },
            { label: 'Cancel', testID: 'dialog.cancel', onPress: () => setConfirmExit(false) }]} />
          : confirmLeave !== null ? <V4Dialog embedded visible title="Leave Room" onClose={() => setConfirmLeave(null)}
            message={`Leave ${confirmLeave}?${session.roomConversations.find((room) => room.name === confirmLeave)?.draft.trim() ? ' Your unsent message will be discarded.' : ''}`}
            actions={[{ label: 'Leave', testID: 'room.leave.confirm', onPress: () => { void leaveRoom(confirmLeave); } },
              { label: 'Cancel', onPress: () => setConfirmLeave(null) }]} />
            : menu ? <V4Menu embedded visible title={menu.title} items={menu.items} onClose={() => setMenu(null)} /> : roster.prompt}
    </Modal>
  </SafeAreaView></View>;
}
