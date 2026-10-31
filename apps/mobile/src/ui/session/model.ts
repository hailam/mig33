import type { ClientState, Contact, ContactGroup, EmoticonAsset, EmoticonCatalog, RoomParticipant, RoomSummary, ServerMenuEntry } from '../../mig33/client.ts';
export type { RoomParticipant, RoomSummary } from '../../mig33/client.ts';

export type { ConnectionSettings } from '../connectionSettings.ts';

export interface LoginCredentials { username: string; password: string; invisible?: boolean; }
export interface TranscriptRow {
  id: string;
  sender: string;
  body: string;
  own: boolean;
  action: boolean;
  color?: number;
  senderColor?: number;
  avatar?: string;
  content?: number;
  tokens?: readonly string[];
  imageURI?: string;
  mediaReference?: string;
}
export interface Conversation {
  peer: string;
  rows: readonly TranscriptRow[];
  draft: string;
  unread: number;
}
export interface RoomCategoryView {
  id: number;
  name: string;
  favorites: boolean;
  rooms: readonly RoomSummary[];
  loading: boolean;
  loaded: boolean;
  footer: string;
  /** Original714 flag3: footer reload clears rows when true, otherwise appends. */
  replaceOnMore: boolean;
}
/** Separate membership identity; peer permits reuse of the transcript/editor. */
export interface RoomConversation extends Conversation {
  name: string;
  communityId?: number;
  state: 'joining' | 'joined' | 'leaving';
  participants: readonly RoomParticipant[];
  participantsLoaded: boolean;
}
export interface GroupConversation extends Conversation {
  id: string;
  state: 'joined' | 'leaving';
  participants: readonly RoomParticipant[];
  participantsLoaded: boolean;
}
export interface SessionSnapshot {
  state: ClientState;
  username: string;
  presence: number;
  ownStatus: string;
  contacts: readonly Contact[];
  groups: readonly ContactGroup[];
  conversations: readonly Conversation[];
  activePeer: string | null;
  /** Whether the retained conversation is the logical read-selected tab. */
  conversationVisible: boolean;
  roomCategories: readonly RoomCategoryView[];
  roomConversations: readonly RoomConversation[];
  activeRoom: string | null;
  roomsLoading: boolean;
  roomAction: string | null;
  groupConversations: readonly GroupConversation[];
  activeGroup: string | null;
  notice: { title: string; text: string } | null;
  siteURL: string;
  chatColors: readonly number[];
  selectedTextColor: number;
  emoticons: EmoticonCatalog;
  emoticonAssets: Readonly<Record<string, EmoticonAsset>>;
  serverMenus: readonly ServerMenuEntry[];
  error: string;
  syncing: boolean;
  sending: boolean;
}

export function initialSession(): SessionSnapshot {
  return {
    state: 'idle', username: '', presence: 1, ownStatus: '', contacts: [], groups: [],
    conversations: [], activePeer: null, conversationVisible: true, error: '', syncing: false, sending: false,
    roomCategories: [], roomConversations: [], activeRoom: null, roomsLoading: false, roomAction: null,
    groupConversations: [], activeGroup: null, notice: null, siteURL: '', chatColors: [0], selectedTextColor: -1,
    emoticons: { standard: [], other: [] }, emoticonAssets: {}, serverMenus: [],
  };
}

export function normalizePeer(peer: string): string { return peer.trim().toLowerCase(); }
export function roomKey(name: string): string { return name.trim().toLowerCase(); }
export function newRoomConversation(name: string): RoomConversation {
  return { name, peer: name, rows: [], draft: '', unread: 0, state: 'joining', participants: [], participantsLoaded: false };
}
export function newGroupConversation(id: string): GroupConversation {
  return { id, peer: id, rows: [], draft: '', unread: 0, state: 'joined', participants: [], participantsLoaded: false };
}

/** Original protocol presence values, not an online/offline boolean. */
export function presenceLabel(presence: number): string {
  switch (presence) {
    case 1: return 'Online';
    case 3: return 'Busy';
    case 4: return 'Away';
    case 99: return 'Offline';
    default: return 'Unknown';
  }
}

export function isAvailable(presence: number): boolean {
  return presence === 1 || presence === 3 || presence === 4;
}

export interface ContactSection {
  id: number;
  name: string;
  contacts: readonly Contact[];
  online: number;
}

/** Preserve server group and contact order; append unassigned groups in wire order. */
export function contactSections(groups: readonly ContactGroup[], contacts: readonly Contact[]): ContactSection[] {
  const sections = new Map<number, ContactSection>();
  for (const group of groups) sections.set(group.id, { ...group, contacts: [], online: 0 });
  for (const contact of contacts) {
    // This first UI supports mig33 peers; external service records remain in
    // the raw roster rather than being mislabeled as mig33 contacts.
    if (!contact.username) continue;
    const section = sections.get(contact.groupId) ?? {
      id: contact.groupId, name: contact.groupId === -1 ? 'Friends' : 'Other friends', contacts: [], online: 0,
    };
    sections.set(section.id, {
      ...section, contacts: [...section.contacts, contact], online: section.online + Number(isAvailable(contact.presence)),
    });
  }
  return [...sections.values()];
}

export function newConversation(peer: string): Conversation { return { peer, rows: [], draft: '', unread: 0 }; }

// Retain the prototype's200 logical-message bound per conversation. The
// original's50 internal entries also count wrapped parts, so it is not a
// directly interchangeable message cap (research v4 rendering.md).
export function appendTranscript<T extends Conversation>(conversation: T, row: TranscriptRow, unread: boolean): T {
  return { ...conversation, rows: [...conversation.rows.slice(-199), row], unread: conversation.unread + Number(unread) };
}
