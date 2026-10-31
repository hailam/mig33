import { Mig33Client, type AddContactOptions, type ContactDetails, type OwnPresence,
  type ChatMessage, type ClientEvent, type OriginalRequest } from '../../mig33/client.ts';
import type { Packet } from '../../mig33/protocol/index.ts';
import { imageDataURI } from '../../mig33/media.ts';
import { validateConnection } from '../connectionSettings.ts';
import {
  appendTranscript, initialSession, newConversation, newRoomConversation, newGroupConversation, normalizePeer, roomKey,
  type ConnectionSettings, type Conversation, type LoginCredentials, type RoomConversation, type GroupConversation,
  type SessionSnapshot, type TranscriptRow,
} from './model.ts';

interface LiveSession { client: Mig33Client; unsubscribe: () => void; }
type CreateClient = (connection: ConnectionSettings) => Mig33Client;
function message(error: unknown): string { return error instanceof Error ? error.message : String(error); }
function color(value: number | undefined): number | undefined {
  return value !== undefined && value >= 0 && value <= 0xffffff ? value : undefined;
}

/** UI session ownership; platform dependencies are injected by the React hook. */
export class SessionController {
  private snapshot = initialSession();
  private readonly listeners = new Set<() => void>();
  private live?: LiveSession;
  private connecting = false;
  private sendingSession?: LiveSession;
  private privateSend?: { session: LiveSession; peer: string; convertedGroup?: string };
  private disconnectingSession?: LiveSession;
  private syncingSession?: LiveSession;
  private roomsLoadingSession?: LiveSession;
  private roomActionSession?: LiveSession;
  private readonly roomCategoryLoads = new Map<number, { session: LiveSession }>();
  private readonly roomAliases = new Map<string, string>();
  private readonly assetLoads = new Map<string, { session: LiveSession; generation: number }>();
  private assetGeneration = 0;
  private nextRow = 0;
  private seenPackets = new WeakSet<Packet>();
  private readonly createClient: CreateClient;
  private readonly platform: string;

  constructor(createClient: CreateClient, platform: string) {
    this.createClient = createClient;
    this.platform = platform;
  }

  readonly getSnapshot = (): SessionSnapshot => this.snapshot;
  readonly subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  readonly clearError = (): void => { this.update({ error: '' }); };
  readonly clearNotice = (): void => { this.update({ notice: null }); };
  readonly requestOriginal: OriginalRequest = async (opcode, configure, replies, complete) => {
    const current = this.live;
    if (!current || current.client.state !== 'online') throw new Error('Log in before opening this page');
    const packets = await current.client.requestOriginal(opcode, configure, replies, complete);
    if (!this.isCurrent(current)) throw new Error('The page belongs to a closed session');
    return packets;
  };
  readonly setTextColor = (value: number): void => {
    if (value === -1 || this.snapshot.chatColors.includes(value)) this.update({ selectedTextColor: value });
  };

  readonly login = async (credentials: LoginCredentials, connection: ConnectionSettings): Promise<boolean> => {
    if (this.connecting) return false;
    this.connecting = true;
    this.release();
    this.seenPackets = new WeakSet();
    this.snapshot = {
      ...initialSession(), username: normalizePeer(credentials.username),
      presence: credentials.invisible ? 99 : 1, state: 'connecting',
    };
    this.notify();
    let session: LiveSession | undefined;
    try {
      const client = this.createClient(validateConnection(connection));
      session = { client, unsubscribe: () => {} };
      const current = session;
      this.live = current;
      current.unsubscribe = client.subscribe((event) => {
        if (this.live === current) this.receive(current, event);
      });
      await client.login({ ...credentials, platform: this.platform });
      if (this.live !== current || client.state !== 'online') return false;
      await this.syncContacts();
      return this.live === current && client.state === 'online';
    } catch (error) {
      if ((!session || this.live === session) && this.snapshot.state !== 'closed') {
        this.update({ state: 'error', error: message(error), syncing: false, sending: false });
      }
      return false;
    } finally { this.connecting = false; }
  };

  readonly logout = async (): Promise<boolean> => {
    const current = this.live;
    if (!current || this.disconnectingSession === current) return false;
    this.disconnectingSession = current;
    try {
      await current.client.logout();
      return true;
    } catch (error) {
      if (this.live === current) this.update({ error: message(error) });
      return false;
    } finally {
      if (this.disconnectingSession === current) this.disconnectingSession = undefined;
    }
  };

  readonly syncContacts = async (): Promise<boolean> => {
    const current = this.live;
    if (!current || current.client.state !== 'online' || this.syncingSession === current) return false;
    this.syncingSession = current;
    this.update({ syncing: true, error: '' });
    try {
      await current.client.syncContacts();
      if (this.live !== current || current.client.state !== 'online') return false;
      this.update({ contacts: current.client.contacts, groups: current.client.groups });
      return true;
    } catch (error) {
      if (this.live === current && this.snapshot.state !== 'closed') this.update({ error: message(error) });
      return false;
    } finally {
      if (this.syncingSession === current) {
        this.syncingSession = undefined;
        if (this.live === current) this.update({ syncing: false });
      }
    }
  };

  readonly openConversation = (name: string): void => {
    const peer = normalizePeer(name);
    if (!peer) return;
    const current = this.find(peer);
    this.setConversation({ ...(current ?? newConversation(peer)), unread: 0 }, {
      activePeer: peer, activeRoom: null, activeGroup: null, conversationVisible: true,
    });
  };

  /** Original pages select another tab; Write and chat menus retain selection. */
  readonly setConversationVisible = (visible: boolean): void => {
    if (this.snapshot.conversationVisible === visible) return;
    const change: Partial<SessionSnapshot> = { conversationVisible: visible };
    if (visible) {
      const { activePeer, activeRoom, activeGroup } = this.snapshot;
      change.conversations = this.snapshot.conversations.map((conversation) =>
        conversation.peer === activePeer && conversation.unread ? { ...conversation, unread: 0 } : conversation);
      change.roomConversations = this.snapshot.roomConversations.map((room) =>
        roomKey(room.name) === roomKey(activeRoom ?? '') && room.unread ? { ...room, unread: 0 } : room);
      change.groupConversations = this.snapshot.groupConversations.map((group) =>
        group.id === activeGroup && group.unread ? { ...group, unread: 0 } : group);
    }
    this.update(change);
  };

  readonly selectConversation = (peer: string | null): void => {
    if (peer === null) this.update({ activePeer: null, activeRoom: null, activeGroup: null });
    else this.openConversation(peer);
  };

  readonly closeConversation = (name: string): void => {
    const peer = normalizePeer(name);
    this.update({
      conversations: this.snapshot.conversations.filter((conversation) => conversation.peer !== peer),
      activePeer: this.snapshot.activePeer === peer ? null : this.snapshot.activePeer,
    });
  };

  readonly setDraft = (name: string, draft: string): void => {
    const peer = normalizePeer(name);
    if (!peer) return;
    this.setConversation({ ...(this.find(peer) ?? newConversation(peer)), draft });
  };

  readonly loadRoomCategories = async (): Promise<boolean> => {
    const current = this.live;
    if (!current || current.client.state !== 'online' || this.roomsLoadingSession === current) return false;
    this.roomsLoadingSession = current;
    this.update({ roomsLoading: true, error: '' });
    try {
      const categories = await current.client.listRoomCategories();
      if (!this.isCurrent(current)) return false;
      this.update({ roomCategories: categories.map((category) => {
        const previous = this.snapshot.roomCategories.find((entry) => entry.id === category.id);
        return { id: category.id, name: category.name, favorites: category.favorites, replaceOnMore: category.flags[0] === 1,
          rooms: previous?.rooms ?? [], loading: previous?.loading ?? false, loaded: previous?.loaded ?? false,
          footer: previous?.footer ?? '' };
      }) });
      // The global refresh restarts loaded lists (original aj.q uses716 flag1).
      // Always load Favorites so Add/Remove labels reflect actual membership.
      const refreshed = await Promise.all(this.snapshot.roomCategories.filter((category) => category.loaded || category.favorites)
        .map((category) => this.refreshRoomCategory(category.id, true)));
      return this.isCurrent(current) && refreshed.every(Boolean);
    } catch (error) { this.roomError(current, error); return false; }
    finally {
      if (this.roomsLoadingSession === current) {
        this.roomsLoadingSession = undefined;
        if (this.live === current) this.update({ roomsLoading: false });
      }
    }
  };

  readonly loadRoomCategory = (id: number): Promise<boolean> => this.refreshRoomCategory(id, false);
  readonly reloadRoomCategory = (id: number): Promise<boolean> => this.refreshRoomCategory(id, true);

  private async refreshRoomCategory(id: number, reset: boolean): Promise<boolean> {
    const current = this.live;
    const category = this.snapshot.roomCategories.find((entry) => entry.id === id);
    if (!current || current.client.state !== 'online' || (!reset && this.roomCategoryLoads.get(id)?.session === current) || !category) return false;
    const ticket = { session: current };
    this.roomCategoryLoads.set(id, ticket);
    const initial = reset || !category.loaded;
    this.update({ error: '', roomCategories: this.snapshot.roomCategories.map((category) => category.id === id
      ? { ...category, loading: true } : category) });
    try {
      const listing = await current.client.listRooms(id, initial);
      if (!this.isCurrent(current) || this.roomCategoryLoads.get(id) !== ticket) return false;
      this.update({ roomCategories: this.snapshot.roomCategories.map((category) => category.id === id
        ? { ...category, rooms: initial || category.replaceOnMore ? listing.rooms
          : [...new Map([...category.rooms, ...listing.rooms].map((room) => [roomKey(room.name), room])).values()],
        footer: listing.footer, loaded: true } : category) });
      return true;
    } catch (error) {
      if (this.roomCategoryLoads.get(id) === ticket) this.roomError(current, error);
      return false;
    } finally {
      if (this.roomCategoryLoads.get(id) === ticket) {
        this.roomCategoryLoads.delete(id);
        if (this.live === current) this.update({ roomCategories: this.snapshot.roomCategories.map((category) => category.id === id
          ? { ...category, loading: false } : category) });
      }
    }
  }

  /** Joining never navigates: callers decide whether their original view is still active. */
  readonly joinRoom = async (name: string): Promise<boolean> => {
    const current = this.live;
    const key = roomKey(name);
    if (!current || current.client.state !== 'online' || !key || this.roomActionSession) return false;
    const previous = this.findRoom(name);
    if (previous?.state === 'joined') return true;
    if (previous) return false;
    this.beginRoomAction(current, `join:${key}`);
    this.setRoom(newRoomConversation(name.trim()));
    try {
      const room = await current.client.joinRoom(name);
      if (!this.isCurrent(current)) return false;
      const conversation = this.findRoom(room.name);
      if (conversation) this.setRoom({ ...conversation, state: 'joined', communityId: room.communityId });
      return !!conversation;
    } catch (error) {
      if (this.live === current && this.findRoom(name)?.state === 'joining') this.removeRoom(name);
      this.roomError(current, error);
      return false;
    } finally { this.endRoomAction(current); }
  };

  readonly openRoom = (name: string): void => {
    const conversation = this.findRoom(name);
    if (conversation?.state !== 'joined') return;
    this.setRoom({ ...conversation, unread: 0 }, {
      activeRoom: conversation.name, activePeer: null, activeGroup: null, conversationVisible: true,
    });
  };

  readonly leaveRoom = async (name: string): Promise<boolean> => {
    const current = this.live;
    const conversation = this.findRoom(name);
    if (!current || current.client.state !== 'online' || conversation?.state !== 'joined'
      || this.roomActionSession || this.sendingSession) return false;
    this.beginRoomAction(current, `leave:${roomKey(name)}`);
    this.setRoom({ ...conversation, state: 'leaving' });
    try {
      await current.client.leaveRoom(conversation.name);
      if (!this.isCurrent(current)) return false;
      this.removeRoom(conversation.name);
      return true;
    } catch (error) {
      if (this.live === current) {
        const latest = this.findRoom(name);
        if (latest) this.setRoom({ ...latest, state: 'joined' });
      }
      this.roomError(current, error);
      return false;
    } finally { this.endRoomAction(current); }
  };

  readonly setRoomDraft = (name: string, draft: string): void => {
    const conversation = this.findRoom(name);
    if (conversation) this.setRoom({ ...conversation, draft });
  };

  readonly sendRoom = async (name?: string): Promise<boolean> => {
    const current = this.live;
    const conversation = this.findRoom(name ?? this.snapshot.activeRoom ?? '');
    if (!current || current.client.state !== 'online' || conversation?.state !== 'joined'
      || !conversation.draft.trim() || this.sendingSession) return false;
    const originalDraft = conversation.draft;
    const text = originalDraft.trim();
    this.sendingSession = current;
    this.update({ sending: true, error: '' });
    try {
      const selectedColor = this.snapshot.selectedTextColor;
      await current.client.sendRoomMessage(conversation.name, text, selectedColor);
      if (!this.isCurrent(current)) return false;
      const latest = this.findRoom(conversation.name);
      if (latest?.state === 'joined') {
        const rendered = text.startsWith('/') ? latest : appendTranscript(latest,
          this.row({ sender: current.client.username, body: text, own: true, action: false, color: color(selectedColor), tokens: this.messageTokens(text) }), false);
        this.setRoom({ ...rendered, draft: latest.draft === originalDraft ? '' : latest.draft });
      }
      return true;
    } catch (error) { this.roomError(current, error); return false; }
    finally {
      if (this.sendingSession === current) {
        this.sendingSession = undefined;
        if (this.live === current) this.update({ sending: false });
      }
    }
  };

  readonly loadRoomParticipants = async (name: string): Promise<boolean> => {
    const current = this.live;
    const conversation = this.findRoom(name);
    if (!current || current.client.state !== 'online' || conversation?.state !== 'joined' || this.roomActionSession) return false;
    this.beginRoomAction(current, `participants:${roomKey(name)}`);
    try {
      const result = await current.client.getRoomParticipants(conversation.name);
      if (!this.isCurrent(current)) return false;
      const latest = this.findRoom(result.room);
      if (latest) this.setRoom({ ...latest, participants: result.participants, participantsLoaded: true });
      return !!latest;
    } catch (error) { this.roomError(current, error); return false; }
    finally { this.endRoomAction(current); }
  };

  readonly setRoomFavorite = async (name: string, enabled: boolean): Promise<boolean> => {
    const current = this.live;
    const favorite = this.snapshot.roomCategories.find((category) => category.favorites);
    if (!current || current.client.state !== 'online' || !roomKey(name) || this.roomActionSession || !favorite) return false;
    this.beginRoomAction(current, `favorite:${roomKey(name)}`);
    try {
      if (enabled) await current.client.addRoomFavorite(name);
      else await current.client.removeRoomFavorite(name, favorite.id);
      if (!this.isCurrent(current)) return false;
      // Refresh the authoritative snapshot after both first-add701 and repeated
      //ACK responses; never confuse a favorite701 with a room join.
      return await this.refreshRoomCategory(favorite.id, true);
    } catch (error) { this.roomError(current, error); return false; }
    finally { this.endRoomAction(current); }
  };

  readonly rejoinRoom = async (name: string): Promise<boolean> => {
    const previous = this.findRoom(name);
    if (!previous) return this.joinRoom(name);
    return this.runAction(`rejoin:${roomKey(name)}`, async (current) => {
      this.setRoom({ ...previous, state: 'joining' });
      try {
        const result = await current.client.joinRoom(previous.name);
        if (!this.isCurrent(current)) return;
        const latest = this.findRoom(result.name);
        if (latest) this.setRoom({ ...latest, state: 'joined', communityId: result.communityId });
      } catch (error) {
        if (this.isCurrent(current)) {
          const latest = this.findRoom(previous.name);
          if (latest) this.setRoom({ ...latest, state: 'joined' });
        }
        throw error;
      }
    });
  };

  /** Explicit local tab dismissal; it does not claim a successful server leave. */
  readonly closeRoom = (name: string): void => { this.removeRoom(name); };

  readonly moderateRoom = (name: string, username: string, operation: 'kick' | 'mute' | 'unmute'): Promise<boolean> => {
    const room = this.findRoom(name);
    if (room?.state !== 'joined') return Promise.resolve(false);
    return this.runAction(`${operation}:${roomKey(name)}`, async (current) => {
      const text = await current.client.moderateRoom(room.name, username, operation);
      if (!this.isCurrent(current)) return;
      this.update({ notice: { title: 'Room action', text: text || (operation === 'kick'
        ? 'Kick request accepted. The room decides whether this removes the user or records a vote.'
        : `${username} has been ${operation === 'mute' ? 'muted' : 'unmuted'} in ${room.name}.`) } });
      try {
        const list = await current.client.getRoomParticipants(room.name);
        if (!this.isCurrent(current)) return;
        const latest = this.findRoom(room.name);
        if (latest) this.setRoom({ ...latest, participants: list.participants, participantsLoaded: true });
      } catch (error) { this.roomError(current, error); } // Mutation already acknowledged; never replay it.
    });
  };

  readonly blockUser = (username: string): Promise<boolean> => this.runAction(`block:${normalizePeer(username)}`, async (current) => {
    await current.client.blockUser(username);
    if (this.isCurrent(current)) this.update({ notice: { title: 'User blocked', text: `${username} has been added to your block list.` } });
  });

  readonly addContact = (username: string, options?: AddContactOptions): Promise<boolean> => this.runAction(`contact:${normalizePeer(username)}`, async (current) => {
    await current.client.addContact(username, options);
    if (this.isCurrent(current)) this.update({ notice: { title: 'Friend added', text: `${username} is in your contacts.` } });
  });

  readonly updateContact = (id: number, details: ContactDetails): Promise<boolean> =>
    this.runAction(`contact:update:${id}`, async ({ client }) => { await client.updateContact(id, details); });
  readonly deleteContact = (id: number): Promise<boolean> =>
    this.runAction(`contact:delete:${id}`, async ({ client }) => { await client.deleteContact(id); });
  readonly createContactGroup = (name: string): Promise<boolean> =>
    this.runAction('contacts:create-group', async ({ client }) => { await client.createContactGroup(name); });
  readonly renameContactGroup = (id: number, name: string): Promise<boolean> =>
    this.runAction(`contacts:rename-group:${id}`, async ({ client }) => { await client.renameContactGroup(id, name); });
  readonly deleteContactGroup = (id: number): Promise<boolean> =>
    this.runAction(`contacts:delete-group:${id}`, async ({ client }) => { await client.deleteContactGroup(id); });
  readonly moveContact = (id: number, groupId: number): Promise<boolean> =>
    this.runAction(`contact:move:${id}`, async ({ client }) => { await client.moveContact(id, groupId); });
  readonly setPresence = (presence: OwnPresence): Promise<boolean> =>
    this.runAction('presence', async ({ client }) => { await client.setPresence(presence, this.snapshot.presence); });
  readonly setStatus = (status: string): Promise<boolean> =>
    this.runAction('status', async ({ client }) => { await client.setStatus(status, this.snapshot.ownStatus); });

  readonly createGroup = (peer: string, invitee: string): Promise<boolean> => {
    if (!this.find(normalizePeer(peer))) return Promise.resolve(false);
    return this.runAction('group:create', async (current) => { await current.client.createGroup(peer, invitee); });
  };

  readonly inviteToGroup = (id: string, username: string): Promise<boolean> => {
    if (this.findGroup(id)?.state !== 'joined') return Promise.resolve(false);
    return this.runAction(`group:invite:${id}`, async (current) => {
      await current.client.inviteToGroup(id, username);
      if (!this.isCurrent(current)) return;
      this.update({ notice: { title: 'User invited', text: `${username} has been invited to the group chat.` } });
      try {
        const list = await current.client.getGroupParticipants(id);
        if (!this.isCurrent(current)) return;
        const latest = this.findGroup(id);
        if (latest) this.setGroup({ ...latest, participants: list.participants, participantsLoaded: true });
      } catch (error) { this.roomError(current, error); }
    });
  };

  readonly openGroup = (id: string): void => {
    const group = this.findGroup(id);
    if (group?.state === 'joined') this.setGroup({ ...group, unread: 0 }, {
      activeGroup: id, activePeer: null, activeRoom: null, conversationVisible: true,
    });
  };

  readonly leaveGroup = (id: string): Promise<boolean> => {
    const group = this.findGroup(id);
    if (group?.state !== 'joined' || this.sendingSession) return Promise.resolve(false);
    return this.runAction(`group:leave:${id}`, async (current) => {
      this.setGroup({ ...group, state: 'leaving' });
      try {
        await current.client.leaveGroup(id);
        if (!this.isCurrent(current)) return;
        this.update({ groupConversations: this.snapshot.groupConversations.filter((entry) => entry.id !== id),
          activeGroup: this.snapshot.activeGroup === id ? null : this.snapshot.activeGroup });
      } catch (error) {
        const latest = this.findGroup(id);
        if (this.isCurrent(current) && latest) this.setGroup({ ...latest, state: 'joined' });
        throw error;
      }
    });
  };

  readonly loadGroupParticipants = (id: string): Promise<boolean> => {
    if (this.findGroup(id)?.state !== 'joined') return Promise.resolve(false);
    return this.runAction(`group:participants:${id}`, async (current) => {
      const list = await current.client.getGroupParticipants(id);
      if (!this.isCurrent(current)) return;
      const latest = this.findGroup(id);
      if (latest) this.setGroup({ ...latest, participants: list.participants, participantsLoaded: true });
    });
  };

  readonly setGroupDraft = (id: string, draft: string): void => {
    const group = this.findGroup(id);
    if (group) this.setGroup({ ...group, draft });
  };

  readonly sendGroup = async (id?: string): Promise<boolean> => {
    const current = this.live;
    const group = this.findGroup(id ?? this.snapshot.activeGroup ?? '');
    if (!current || !this.isCurrent(current) || group?.state !== 'joined' || !group.draft.trim() || this.sendingSession) return false;
    const originalDraft = group.draft, text = originalDraft.trim(), selectedColor = this.snapshot.selectedTextColor;
    this.sendingSession = current;
    this.update({ sending: true, error: '' });
    try {
      await current.client.sendGroupMessage(group.id, text, selectedColor);
      if (!this.isCurrent(current)) return false;
      const latest = this.findGroup(group.id);
      if (latest?.state === 'joined') {
        const rendered = text.startsWith('/') ? latest : appendTranscript(latest,
          this.row({ sender: current.client.username, body: text, own: true, action: false, color: color(selectedColor), tokens: this.messageTokens(text) }), false);
        this.setGroup({ ...rendered, draft: latest.draft === originalDraft ? '' : latest.draft });
      }
      return true;
    } catch (error) { this.roomError(current, error); return false; }
    finally { this.finishSend(current); }
  };

  readonly sendPhoto = async (kind: 1 | 2 | 3, name: string, data: Uint8Array | string): Promise<boolean> => {
    const current = this.live;
    const conversation = kind === 1 ? this.find(normalizePeer(name)) : kind === 2 ? this.findGroup(name) : this.findRoom(name);
    if (!current || !this.isCurrent(current) || !conversation || this.sendingSession
      || ('state' in conversation && conversation.state !== 'joined')) return false;
    const send = kind === 1 ? { session: current, peer: normalizePeer(name), convertedGroup: undefined as string | undefined } : undefined;
    if (send) this.privateSend = send;
    this.sendingSession = current;
    this.update({ sending: true, error: '' });
    try {
      await current.client.sendPhoto(kind, name, data);
      if (!this.isCurrent(current)) return false;
      const row = this.row({ sender: current.client.username, body: 'Photo', own: true, action: false,
        content: typeof data === 'string' ? 5 : 2, mediaReference: typeof data === 'string' ? data : undefined,
        tokens: typeof data === 'string' ? [`[photo:${data}]`] : [], imageURI: typeof data === 'string' ? undefined : imageDataURI(data) });
      if (kind === 1 && normalizePeer(name) !== current.client.username) {
        if (send?.convertedGroup) {
          const latest = this.findGroup(send.convertedGroup);
          if (latest?.state === 'joined') this.setGroup(appendTranscript(latest, row, false));
        } else {
          const latest = this.find(normalizePeer(name));
          if (latest) this.setConversation(appendTranscript(latest, row, false));
        }
      }
      if (kind === 2) { const latest = this.findGroup(name); if (latest?.state === 'joined') this.setGroup(appendTranscript(latest, row, false)); }
      if (kind === 3) { const latest = this.findRoom(name); if (latest?.state === 'joined') this.setRoom(appendTranscript(latest, row, false)); }
      return true;
    } catch (error) { this.roomError(current, error); return false; }
    finally { this.finishSend(current); }
  };

  readonly uploadPhoto = async (data: Uint8Array, purpose: 0 | 1 | 2 = 0): Promise<string | undefined> => {
    const current = this.live;
    if (!current || !this.isCurrent(current) || this.sendingSession) return undefined;
    this.sendingSession = current;
    this.update({ sending: true, error: '' });
    try {
      const url = await current.client.uploadPhoto(data, purpose);
      return this.isCurrent(current) ? url : undefined;
    } catch (error) { this.roomError(current, error); return undefined; }
    finally { this.finishSend(current); }
  };

  readonly loadEmoticon = async (token: string): Promise<boolean> => {
    const current = this.live;
    if (!current || !this.isCurrent(current)) return false;
    if (this.snapshot.emoticonAssets[token]) return true;
    if (this.assetLoads.get(token)?.session === current) return false;
    const generation = this.assetGeneration;
    const ticket = { session: current, generation };
    this.assetLoads.set(token, ticket);
    try {
      const asset = await current.client.getEmoticon(token);
      if (!this.isCurrent(current) || generation !== this.assetGeneration) return false;
      this.update({ emoticonAssets: { ...this.snapshot.emoticonAssets, [token]: asset } });
      return true;
    } catch { return false; } // A missing decorative resource must not interrupt chat.
    finally { if (this.assetLoads.get(token) === ticket) this.assetLoads.delete(token); }
  };

  readonly send = async (name?: string): Promise<boolean> => {
    const current = this.live;
    const peer = normalizePeer(name ?? this.snapshot.activePeer ?? '');
    const conversation = this.find(peer);
    if (!current || current.client.state !== 'online' || !conversation?.draft.trim() || this.sendingSession) return false;
    const originalDraft = conversation.draft;
    // Original Write Message Send trims the editor value before serialization.
    // Keep the original snapshot separately to preserve edits during the send.
    const text = originalDraft.trim();
    const send = { session: current, peer, convertedGroup: undefined as string | undefined };
    this.privateSend = send;
    this.sendingSession = current;
    this.update({ sending: true, error: '' });
    try {
      const selectedColor = this.snapshot.selectedTextColor;
      await current.client.sendMessage(peer, text, selectedColor);
      if (this.live !== current || current.client.state !== 'online') return false;
      // Closing the tab during a send is intentional; a late acknowledgment
      // must not reopen it. Incoming messages can still open a new unread tab.
      const latest = send.convertedGroup ? this.findGroup(send.convertedGroup) : this.find(peer);
      if (latest) {
        // Slash actions are rendered by the server. Self-addressed text also
        // returns through500, so neither needs a second local transcript row.
        const rendered = text.startsWith('/') || peer === current.client.username
          ? latest
          : appendTranscript(latest, this.row({ sender: current.client.username, body: text, own: true, action: false, color: color(selectedColor), tokens: this.messageTokens(text) }), false);
        // Preserve edits made while this particular draft was in flight.
        const updated = { ...rendered, draft: latest.draft === originalDraft ? '' : latest.draft };
        if (send.convertedGroup) this.setGroup(updated as GroupConversation); else this.setConversation(updated);
      }
      return true;
    } catch (error) {
      // An unknown delivery outcome leaves the draft intact and never retries.
      if (this.live === current && this.snapshot.state !== 'closed') this.update({ error: message(error) });
      return false;
    } finally {
      if (this.privateSend === send) this.privateSend = undefined;
      if (this.sendingSession === current) {
        this.sendingSession = undefined;
        if (this.live === current) this.update({ sending: false });
      }
    }
  };

  /** Backgrounding closes this live connection without replay or auto-login. */
  readonly suspend = (): void => { this.live?.client.close(); };

  /** Back from connection progress cancels authentication and its pending work. */
  readonly cancel = (): void => { this.live?.client.close(); };

  readonly dispose = (): void => { this.release(); };

  private receive(session: LiveSession, event: ClientEvent): void {
    if (event.type === 'state') {
      if (event.state === 'closed' || event.state === 'error') this.roomAliases.clear();
      this.update({
        state: event.state,
        ...(event.error ? { error: event.error.message } : {}),
        ...(['closed', 'error'].includes(event.state) ? {
          syncing: false, sending: false, roomsLoading: false, roomAction: null,
          activeRoom: null, roomConversations: [], activeGroup: null,
          roomCategories: this.snapshot.roomCategories.map((category) => ({ ...category, loading: false })),
        } : {}),
      });
    } else if (event.type === 'contacts') {
      this.update({ contacts: event.contacts, groups: session.client.groups });
    } else if (event.type === 'groups') this.update({ groups: event.groups });
    else if (event.type === 'message') this.receiveMessage(session, event.message);
    else if (event.type === 'room-joining') {
      const previous = this.findRoom(event.requestedName);
      if (previous?.state === 'joining') {
        const canonicalKey = roomKey(event.room.name);
        const canonical = this.snapshot.roomConversations.find((room) => roomKey(room.name) === canonicalKey);
        this.roomAliases.set(roomKey(event.requestedName), canonicalKey);
        // Replace the pending requested identity instead of leaving an orphan
        //when701 supplies a different canonical name. Keep an existing tab.
        const resolved = { ...(canonical ?? previous), name: event.room.name, peer: event.room.name, communityId: event.room.communityId };
        this.update({ roomConversations: this.snapshot.roomConversations
          .filter((room) => room !== previous && room !== canonical).concat(resolved) });
      }
    } else if (event.type === 'notice' && event.text) this.update({ notice: { title: event.title, text: event.text } });
    else if (event.type === 'site') this.update({ siteURL: event.url });
    else if (event.type === 'own-presence') this.update({ presence: event.presence });
    else if (event.type === 'own-status') this.update({ ownStatus: event.status });
    else if (event.type === 'colors') this.update({ chatColors: [...new Set(event.colors)],
      selectedTextColor: event.colors.includes(this.snapshot.selectedTextColor) ? this.snapshot.selectedTextColor : -1 });
    else if (event.type === 'emoticons') this.update({ emoticons: event.catalog });
    else if (event.type === 'emoticons-cleared') { this.assetGeneration++; this.assetLoads.clear(); this.update({ emoticonAssets: {} }); }
    else if (event.type === 'server-menu') this.update({ serverMenus: [...this.snapshot.serverMenus, event.entry] });
    else if (event.type === 'group-created' && event.service === 1 && event.peer) {
      const previous = this.find(normalizePeer(event.peer));
      if (!previous) return; // Original750 converts an existing private tab only.
      if (this.privateSend?.session === session && this.privateSend.peer === previous.peer) this.privateSend.convertedGroup = event.id;
      const existing = this.findGroup(event.id);
      const converted = { ...newGroupConversation(event.id), ...previous, ...existing, peer: event.id, id: event.id };
      const selected = this.snapshot.activePeer === previous.peer;
      this.update({ conversations: this.snapshot.conversations.filter((entry) => entry.peer !== previous.peer),
        groupConversations: [...this.snapshot.groupConversations.filter((entry) => entry.id !== event.id), converted],
        ...(selected ? { activePeer: null, activeRoom: null, activeGroup: event.id } : {}) });
    }
  }

  private receiveMessage(session: LiveSession, incoming: ChatMessage): void {
    if (![1, 2, 3].includes(incoming.kind) || this.seenPackets.has(incoming.packet)) return;
    this.seenPackets.add(incoming.packet);
    //500 does not define a stable message ID: equal text or transactions are
    //not safe duplicate keys. Suppress repeated dispatch of one Packet only.
    const self = normalizePeer(incoming.sender) === session.client.username;
    const peer = normalizePeer(self ? incoming.target : incoming.sender);
    const room = incoming.kind === 3 ? this.findRoom(incoming.target) : undefined;
    const group = incoming.kind === 2 && incoming.target ? this.findGroup(incoming.target) ?? newGroupConversation(incoming.target) : undefined;
    // Match original704 suppression. Unsolicited room text never opens a tab
    //or implicitly rejoins a session, including after a successful leave.
    if (incoming.kind === 3 && (!room || room.state === 'leaving')) return;
    if (incoming.kind === 2 && (!group || group.state === 'leaving')) return;
    if (incoming.kind === 1 && !peer) return;
    const bodyColor = color(incoming.packet.int(13));
    // Original s.a: content6 is action-style only when body color is absent/-1.
    const action = incoming.content === 6 && (incoming.packet.int(13) ?? -1) === -1;
    const tokens = incoming.packet.text(11)?.split(/\s+/).filter(Boolean) ?? [];
    const photo = tokens.find((token) => /^\[photo:\d+\]$/.test(token));
    const imageBytes = incoming.content === 2 ? incoming.packet.fields.get(8) : undefined;
    const imageURI = imageBytes && imageDataURI(imageBytes);
    const row = this.row({
      sender: incoming.sender, body: imageURI ? 'Photo' : incoming.body, own: self && !action, action,
      color: bodyColor, senderColor: color(incoming.packet.int(12)), avatar: incoming.packet.text(10),
      content: incoming.content, tokens, mediaReference: photo?.slice(7, -1), imageURI,
    });
    if (room) {
      this.setRoom(appendTranscript(room, row,
        (!this.snapshot.conversationVisible || roomKey(this.snapshot.activeRoom ?? '') !== roomKey(room.name)) && !self));
      return;
    }
    if (group) {
      this.setGroup(appendTranscript(group, row,
        (!this.snapshot.conversationVisible || this.snapshot.activeGroup !== group.id) && !self));
      return;
    }
    this.setConversation(appendTranscript(
      this.find(peer) ?? newConversation(peer), row,
      (!this.snapshot.conversationVisible || this.snapshot.activePeer !== peer) && !self,
    ));
  }

  private row(row: Omit<TranscriptRow, 'id'>): TranscriptRow { return { ...row, id: String(++this.nextRow) }; }
  private messageTokens(text: string): readonly string[] {
    return [...new Set([...this.snapshot.emoticons.standard, ...this.snapshot.emoticons.other].filter((token) => text.includes(token)))];
  }
  private async runAction(action: string, work: (session: LiveSession) => Promise<void>): Promise<boolean> {
    const current = this.live;
    if (!current || !this.isCurrent(current) || this.roomActionSession) return false;
    this.beginRoomAction(current, action);
    try { await work(current); return this.isCurrent(current); }
    catch (error) { this.roomError(current, error); return false; }
    finally { this.endRoomAction(current); }
  }
  private finishSend(session: LiveSession): void {
    if (this.sendingSession !== session) return;
    this.sendingSession = undefined;
    this.privateSend = undefined;
    if (this.live === session) this.update({ sending: false });
  }
  private findGroup(id: string): GroupConversation | undefined {
    return this.snapshot.groupConversations.find((entry) => entry.id === id);
  }
  private setGroup(group: GroupConversation, change: Partial<SessionSnapshot> = {}): void {
    const exists = this.findGroup(group.id);
    this.update({ ...change, groupConversations: exists
      ? this.snapshot.groupConversations.map((entry) => entry.id === group.id ? group : entry)
      : [...this.snapshot.groupConversations, group] });
  }
  private isCurrent(session: LiveSession): boolean { return this.live === session && session.client.state === 'online'; }
  private roomError(session: LiveSession, error: unknown): void {
    if (this.live === session && this.snapshot.state !== 'closed') this.update({ error: message(error) });
  }
  private beginRoomAction(session: LiveSession, action: string): void {
    this.roomActionSession = session;
    this.update({ roomAction: action, error: '' });
  }
  private endRoomAction(session: LiveSession): void {
    if (this.roomActionSession !== session) return;
    this.roomActionSession = undefined;
    if (this.live === session) this.update({ roomAction: null });
  }
  private findRoom(name: string): RoomConversation | undefined {
    const key = this.roomAliases.get(roomKey(name)) ?? roomKey(name);
    return this.snapshot.roomConversations.find((entry) => roomKey(entry.name) === key);
  }
  private setRoom(conversation: RoomConversation, change: Partial<SessionSnapshot> = {}): void {
    const exists = this.findRoom(conversation.name);
    this.update({ ...change, roomConversations: exists
      ? this.snapshot.roomConversations.map((entry) => roomKey(entry.name) === roomKey(conversation.name) ? conversation : entry)
      : [...this.snapshot.roomConversations, conversation] });
  }
  private removeRoom(name: string): void {
    const key = this.roomAliases.get(roomKey(name)) ?? roomKey(name);
    for (const [alias, target] of this.roomAliases) if (target === key) this.roomAliases.delete(alias);
    this.update({ roomConversations: this.snapshot.roomConversations.filter((entry) => roomKey(entry.name) !== key),
      activeRoom: roomKey(this.snapshot.activeRoom ?? '') === key ? null : this.snapshot.activeRoom });
  }
  private find(peer: string): Conversation | undefined { return this.snapshot.conversations.find((entry) => entry.peer === peer); }
  private setConversation(conversation: Conversation, change: Partial<SessionSnapshot> = {}): void {
    const exists = this.snapshot.conversations.some((entry) => entry.peer === conversation.peer);
    this.update({ ...change, conversations: exists
      ? this.snapshot.conversations.map((entry) => entry.peer === conversation.peer ? conversation : entry)
      : [...this.snapshot.conversations, conversation] });
  }
  private update(change: Partial<SessionSnapshot>): void { this.snapshot = { ...this.snapshot, ...change }; this.notify(); }
  private notify(): void { for (const listener of this.listeners) listener(); }
  private release(): void {
    const previous = this.live;
    this.live = undefined;
    this.sendingSession = undefined;
    this.privateSend = undefined;
    this.syncingSession = undefined;
    this.roomsLoadingSession = undefined;
    this.roomActionSession = undefined;
    this.roomCategoryLoads.clear();
    this.roomAliases.clear();
    this.assetLoads.clear();
    this.assetGeneration++;
    this.disconnectingSession = undefined;
    previous?.unsubscribe();
    previous?.client.close();
  }
}
