import { useEffect, useRef, useSyncExternalStore } from 'react';
import { AppState, Platform } from 'react-native';
import { Mig33Client } from '../../mig33/client.ts';
import { createSecureTransport } from '../connection.ts';
import { SessionController } from './controller.ts';
import type { ConnectionSettings } from './model.ts';

function createClient(connection: ConnectionSettings): Mig33Client {
  return new Mig33Client(createSecureTransport(connection));
}

export function useMig33Session() {
  const ref = useRef<SessionController | null>(null);
  if (ref.current === null) ref.current = new SessionController(createClient, `Expo ${Platform.OS}`);
  const session = ref.current;
  const snapshot = useSyncExternalStore(session.subscribe, session.getSnapshot, session.getSnapshot);
  useEffect(() => {
    const listener = AppState.addEventListener('change', (state) => { if (state === 'background') session.suspend(); });
    return () => { listener.remove(); session.dispose(); };
  }, [session]);
  return {
    ...snapshot,
    online: snapshot.state === 'online',
    busy: snapshot.state === 'connecting' || snapshot.state === 'authenticating',
    activeConversation: snapshot.conversations.find((conversation) => conversation.peer === snapshot.activePeer),
    activeRoomConversation: snapshot.roomConversations.find((conversation) => conversation.name === snapshot.activeRoom),
    activeGroupConversation: snapshot.groupConversations.find((conversation) => conversation.id === snapshot.activeGroup),
    login: session.login, logout: session.logout, syncContacts: session.syncContacts,
    openConversation: session.openConversation, selectConversation: session.selectConversation,
    setConversationVisible: session.setConversationVisible,
    closeConversation: session.closeConversation, setDraft: session.setDraft,
    send: session.send, clearError: session.clearError, cancel: session.cancel,
    loadRoomCategories: session.loadRoomCategories, loadRoomCategory: session.loadRoomCategory,
    reloadRoomCategory: session.reloadRoomCategory,
    joinRoom: session.joinRoom, openRoom: session.openRoom, leaveRoom: session.leaveRoom,
    setRoomDraft: session.setRoomDraft, sendRoom: session.sendRoom,
    loadRoomParticipants: session.loadRoomParticipants, setRoomFavorite: session.setRoomFavorite,
    rejoinRoom: session.rejoinRoom, closeRoom: session.closeRoom, moderateRoom: session.moderateRoom,
    blockUser: session.blockUser, addContact: session.addContact,
    updateContact: session.updateContact, deleteContact: session.deleteContact,
    createContactGroup: session.createContactGroup, renameContactGroup: session.renameContactGroup,
    deleteContactGroup: session.deleteContactGroup, moveContact: session.moveContact,
    setPresence: session.setPresence, setStatus: session.setStatus,
    createGroup: session.createGroup, inviteToGroup: session.inviteToGroup, leaveGroup: session.leaveGroup,
    openGroup: session.openGroup, loadGroupParticipants: session.loadGroupParticipants,
    setGroupDraft: session.setGroupDraft, sendGroup: session.sendGroup,
    clearNotice: session.clearNotice, requestOriginal: session.requestOriginal,
    setTextColor: session.setTextColor, loadEmoticon: session.loadEmoticon,
    sendPhoto: session.sendPhoto, uploadPhoto: session.uploadPhoto,
  };
}

export type Mig33Session = ReturnType<typeof useMig33Session>;
export type { ConnectionSettings, LoginCredentials, Conversation, TranscriptRow,
  RoomCategoryView, RoomConversation, RoomParticipant, RoomSummary, GroupConversation } from './model.ts';
export { contactSections, presenceLabel, isAvailable } from './model.ts';
