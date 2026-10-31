import { messageDescriptors as messages } from './generated/contracts.ts';
import { Packet, encodeFrame, loginChallengeHash } from './protocol/index.ts';
import { decodeEmoticon, type EmoticonAsset } from './media.ts';
export type { EmoticonAsset } from './media.ts';
import { asError, type PacketTransport } from './transport/types.ts';

export type ClientState = 'idle' | 'connecting' | 'authenticating' | 'online' | 'closed' | 'error';
export interface ChatMessage {
  sender: string;
  target: string;
  body: string;
  kind: number;
  content: number;
  packet: Packet;
}
export interface Contact {
  id: number;
  groupId: number;
  username: string;
  display: string;
  presence: number;
  avatar: string;
  status: string;
  packet: Packet;
}
export interface ContactGroup { id: number; name: string; }
export interface ContactDetails { display?: string; groupId?: number; mobile?: string; }
export interface AddContactOptions extends ContactDetails { shareMobile?: boolean; }
export type OwnPresence = 1 | 3 | 4 | 99;
export interface RoomCategory {
  id: number;
  name: string;
  favorites: boolean;
  flags: readonly [number, number, number];
  packet: Packet;
}
export interface RoomSummary {
  name: string;
  capacity?: number;
  occupancy?: number;
  categoryId?: number;
  /** Original701 tag8, the persistent Community Group linked to a room. */
  communityId?: number;
  packet: Packet;
}
export interface RoomParticipant { username: string; moderator: boolean; muted: boolean; }
export interface RoomParticipants {
  room: string;
  participants: readonly RoomParticipant[];
  service: number;
  packet: Packet;
}
export interface RoomListing { categoryId: number; rooms: readonly RoomSummary[]; footer: string; }
export interface ServerMenuEntry { category: number; index: number; label: string; value: string; packet: Packet; }
export interface EmoticonCatalog { standard: readonly string[]; other: readonly string[]; }
export type OriginalRequest = (opcode: number, configure: (packet: Packet) => void,
  replies: readonly number[], complete: number | readonly number[]) => Promise<Packet[]>;
export type ClientEvent =
  | { type: 'state'; state: ClientState; error?: Error }
  | { type: 'message'; message: ChatMessage }
  | { type: 'contacts'; contacts: readonly Contact[] }
  | { type: 'groups'; groups: readonly ContactGroup[] }
  | { type: 'room-joining'; requestedName: string; room: RoomSummary }
  | { type: 'notice'; title: string; text: string }
  | { type: 'group-created'; id: string; peer?: string; service: number }
  | { type: 'site'; url: string }
  | { type: 'own-presence'; presence: OwnPresence }
  | { type: 'own-status'; status: string }
  | { type: 'colors'; colors: readonly number[] }
  | { type: 'emoticons'; catalog: EmoticonCatalog }
  | { type: 'emoticons-cleared' }
  | { type: 'server-menu'; entry: ServerMenuEntry }
  | { type: 'packet'; packet: Packet };

export class RemoteError extends Error {
  readonly transaction: number;
  constructor(packet: Packet) {
    super(packet.text(2) ?? 'The server rejected the operation');
    this.name = 'RemoteError';
    this.transaction = packet.transaction;
  }
}

interface Pending {
  request: Packet;
  packets: Packet[];
  replies: Set<number>;
  complete: ReadonlySet<number>;
  resolve(packets: Packet[]): void;
  reject(error: Error): void;
  timer: ReturnType<typeof setTimeout>;
}
interface ClientOptions { requestTimeoutMs?: number; heartbeatIntervalMs?: number; }
interface LoginOptions {
  username: string;
  password: string;
  platform?: string;
  width?: number;
  height?: number;
  invisible?: boolean;
}

/** One live original-protocol session. Never automatically replay a failed send. */
export class Mig33Client {
  readonly transport: PacketTransport;
  private readonly requestTimeout: number;
  private readonly heartbeatInterval: number;
  private readonly listeners = new Set<(event: ClientEvent) => void>();
  private readonly pending = new Map<number, Pending>();
  private readonly roster = new Map<number, Contact>();
  private readonly contactGroups = new Map<number, ContactGroup>();
  private heartbeat?: ReturnType<typeof setInterval>;
  private counter = 0;
  private lastActivity = 0;
  private started = false;
  private syncing = false;
  private current: ClientState = 'idle';
  private name = '';

  constructor(transport: PacketTransport, options: ClientOptions = {}) {
    this.transport = transport;
    this.requestTimeout = options.requestTimeoutMs ?? 30000;
    this.heartbeatInterval = options.heartbeatIntervalMs ?? 60000;
  }

  get state(): ClientState { return this.current; }
  get username(): string { return this.name; }
  get contacts(): readonly Contact[] { return [...this.roster.values()]; }
  get groups(): readonly ContactGroup[] { return [...this.contactGroups.values()]; }
  subscribe(listener: (event: ClientEvent) => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  async login(options: LoginOptions): Promise<void> {
    if (this.started) throw new Error('Create a new client to log in again');
    this.started = true;
    this.name = options.username.trim().toLowerCase();
    if (!this.name || !options.password) throw new Error('Enter a username and password');
    this.changeState('connecting');
    try {
      await this.transport.connect({
        packets: (packets) => { for (const packet of packets) this.receive(packet); },
        closed: (error) => this.fail(error ?? new Error('Connection closed')),
      });
      this.expectLoginState('connecting');
      this.changeState('authenticating');
      const start = new Packet(messages.Client200.opcode, this.nextTransaction())
        .setShort(1, 1).setByte(2, 2).setShort(3, 462)
        .setText(5, this.name).setText(7, 'J2MEv4.62.000')
        // Original login200 tag9 carries the selected presence, including 99
        // for Login As Invisible (bc login builder; protocol.md Login200).
        .setText(8, options.platform ?? 'mig33 Expo').setByte(9, options.invisible ? 99 : 1)
        .setInt(11, 0).setInt(12, (options.width ?? 240) - 6)
        .setInt(13, options.height ?? 320).setText(15, 'en-US')
        .setInt(16, 0).setText(17, '').setByte(19, 0)
        .setText(20, '').setText(21, '').setText(22, '');
      const [challenge] = await this.request(start, [201], 201);
      this.expectLoginState('authenticating');
      const token = challenge.text(1);
      const session = challenge.text(2);
      if (token === undefined || !session) throw new Error('Incomplete original login challenge');
      this.transport.setSession(session);
      await this.request(
        new Packet(messages.Client202.opcode, this.nextTransaction())
          .setInt(1, loginChallengeHash(token, options.password)), [203], 203,
      );
      this.expectLoginState('authenticating');
      this.transport.activate();
      this.lastActivity = Date.now();
      this.changeState('online');
      if (this.transport.kind === 'tcp') {
        this.heartbeat = setInterval(() => {
          if (this.current === 'online' && Date.now() - this.lastActivity >= this.heartbeatInterval) {
            void this.request(new Packet(messages.Client2.opcode, this.nextTransaction()).setInt(1, 1), [3], 3)
              .catch((error) => this.fail(asError(error)));
          }
        }, Math.min(5000, this.heartbeatInterval));
      }
    } catch (error) {
      this.fail(asError(error));
      throw error;
    }
  }

  async syncContacts(): Promise<readonly Contact[]> {
    this.requireOnline();
    if (this.syncing) throw new Error('Contact synchronization is already pending');
    this.syncing = true;
    // No persisted contact cache yet: always request the original full snapshot.
    this.roster.clear();
    this.contactGroups.clear();
    try {
      await this.request(new Packet(messages.Client400.opcode, this.nextTransaction()), [401, 402, 420, 403], 403);
      // receive applies snapshot rows and pushes in wire order. Replaying rows
      // here would undo a push delivered in the same batch after403.
      this.emit({ type: 'groups', groups: this.groups });
      this.emit({ type: 'contacts', contacts: this.contacts });
      return this.contacts;
    } finally { this.syncing = false; }
  }

  async sendMessage(recipient: string, text: string, color?: number): Promise<void> {
    this.requireOnline();
    if (!recipient.trim() || !text) throw new Error('Enter a recipient and message');
    const packet = new Packet(messages.Client500.opcode, this.nextTransaction())
      .setByte(1, 1).setText(2, this.name).setByte(3, 1)
      .setText(4, recipient.trim().toLowerCase()).setShort(6, 1).setText(8, text);
    applyColor(packet, color);
    await this.request(packet, [1], 1);
  }

  async listRoomCategories(): Promise<readonly RoomCategory[]> {
    this.requireOnline();
    const packets = await this.request(new Packet(messages.Client713.opcode, this.nextTransaction()), [714, 715], 715);
    const categories = new Map<number, RoomCategory>();
    for (const packet of packets) {
      const id = packet.short(1);
      const name = packet.text(2);
      if (packet.opcode !== 714 || id === undefined || name === undefined) continue;
      const flags = [packet.byte(3) ?? 0, packet.byte(4) ?? 0, packet.byte(5) ?? 0] as const;
      categories.set(id, { id, name, favorites: flags[2] === 1, flags, packet });
    }
    return [...categories.values()];
  }

  async listRooms(categoryId: number, initial = true): Promise<RoomListing> {
    this.requireOnline();
    const packets = await this.request(new Packet(messages.Client716.opcode, this.nextTransaction())
      .setShort(1, categoryId).setByte(2, initial ? 1 : 0), [701, 717], 717);
    const rooms = new Map<string, RoomSummary>();
    for (const packet of packets) {
      if (packet.opcode !== 701) continue;
      const room = roomSummary(packet);
      if (room) rooms.set(room.name.toLowerCase(), { ...room, categoryId });
    }
    //717 has no category field: the originating716 owns this snapshot.
    return { categoryId, rooms: [...rooms.values()], footer: packets.at(-1)?.text(1) ?? '' };
  }

  async joinRoom(name: string): Promise<RoomSummary> {
    this.requireOnline();
    const packets = await this.request(new Packet(messages.Client703.opcode, this.nextTransaction())
      .setText(1, requiredRoomName(name)), [701, 1], 1);
    const packet = packets.find((reply) => reply.opcode === 701);
    const room = packet && roomSummary(packet);
    if (!room) throw new Error('The room join was acknowledged without room details');
    return room;
  }

  async leaveRoom(name: string): Promise<void> {
    this.requireOnline();
    await this.request(new Packet(messages.Client704.opcode, this.nextTransaction()).setText(1, requiredRoomName(name)), [1], 1);
  }

  async getRoomParticipants(name: string): Promise<RoomParticipants> {
    this.requireOnline();
    const room = requiredRoomName(name);
    const [packet] = await this.request(new Packet(messages.Client707.opcode, this.nextTransaction()).setText(1, room), [708], 708);
    return participantList(room, packet);
  }

  async addRoomFavorite(name: string): Promise<void> {
    this.requireOnline();
    // A new favorite returns701; an already-loaded favorite may return ACK1.
    await this.request(new Packet(messages.Client711.opcode, this.nextTransaction())
      .setText(1, requiredRoomName(name)), [701, 1], [701, 1]);
  }

  async removeRoomFavorite(name: string, categoryId: number): Promise<void> {
    this.requireOnline();
    await this.request(new Packet(messages.Client712.opcode, this.nextTransaction())
      .setShort(1, categoryId).setText(2, requiredRoomName(name)), [1], 1);
  }

  async sendRoomMessage(name: string, text: string, color?: number): Promise<void> {
    this.requireOnline();
    if (!text) throw new Error('Enter a message');
    const packet = new Packet(messages.Client500.opcode, this.nextTransaction())
      .setByte(1, 1).setText(2, this.name).setByte(3, 3)
      .setText(4, requiredRoomName(name)).setShort(6, 1).setText(8, text);
    applyColor(packet, color);
    await this.request(packet, [1], 1);
  }

  async moderateRoom(name: string, target: string, operation: 'kick' | 'mute' | 'unmute'): Promise<string | undefined> {
    this.requireOnline();
    const opcode = { kick: messages.Client706.opcode, mute: messages.Client709.opcode, unmute: messages.Client710.opcode }[operation];
    const [ack] = await this.request(new Packet(opcode, this.nextTransaction()).setText(1, requiredRoomName(name))
      .setText(2, requiredUsername(target)), [1], 1);
    return ack.text(1);
  }

  async blockUser(username: string): Promise<void> {
    this.requireOnline();
    //417's historical boolean is ignored; mode2 is always personal blocking.
    await this.request(new Packet(messages.Client417.opcode, this.nextTransaction())
      .setText(1, requiredUsername(username)).setByte(2, 2), [1], 1);
  }

  async addContact(username: string, options: AddContactOptions = {}): Promise<void> {
    this.requireOnline();
    const packet = new Packet(messages.Client405.opcode, this.nextTransaction())
      .setText(12, requiredUsername(username)).setByte(24, options.shareMobile ? 1 : 0);
    applyContactDetails(packet, options);
    await this.request(packet, [402], 402);
    this.emit({ type: 'contacts', contacts: this.contacts });
  }

  async updateContact(id: number, details: ContactDetails): Promise<void> {
    this.requireOnline();
    const packet = new Packet(messages.Client407.opcode, this.nextTransaction()).setInt(1, positiveId(id));
    applyContactDetails(packet, details);
    await this.request(packet, [402], 402);
    this.emit({ type: 'contacts', contacts: this.contacts });
  }

  async deleteContact(id: number): Promise<void> {
    this.requireOnline();
    await this.request(new Packet(messages.Client406.opcode, this.nextTransaction()).setInt(1, positiveId(id)), [1], 1);
  }

  async createContactGroup(name: string): Promise<void> {
    this.requireOnline();
    await this.request(new Packet(messages.Client408.opcode, this.nextTransaction()).setText(2, requiredGroupName(name)), [401], 401);
    this.emit({ type: 'groups', groups: this.groups });
  }

  async renameContactGroup(id: number, name: string): Promise<void> {
    this.requireOnline();
    await this.request(new Packet(messages.Client410.opcode, this.nextTransaction())
      .setInt(1, positiveId(id)).setText(2, requiredGroupName(name)), [1], 1);
  }

  async deleteContactGroup(id: number): Promise<void> {
    this.requireOnline();
    await this.request(new Packet(messages.Client409.opcode, this.nextTransaction()).setInt(1, positiveId(id)), [1], 1);
  }

  async moveContact(id: number, groupId: number): Promise<void> {
    this.requireOnline();
    const packet = new Packet(messages.Client411.opcode, this.nextTransaction()).setInt(1, positiveId(id));
    if (groupId !== -1) packet.setInt(2, positiveId(groupId));
    await this.request(packet, [1], 1);
  }

  async setPresence(presence: OwnPresence, previous: number): Promise<void> {
    this.requireOnline();
    if (![1, 3, 4, 99].includes(presence)) throw new Error('Choose a valid presence');
    await this.request(new Packet(messages.Client600.opcode, this.nextTransaction())
      .setByte(1, presence).setByte(2, previous), [1], 1);
  }

  async setStatus(status: string, previous: string): Promise<void> {
    this.requireOnline();
    await this.request(new Packet(messages.Client601.opcode, this.nextTransaction())
      .setText(1, status).setText(2, previous), [1], 1);
  }

  async createGroup(peer: string, invitee: string): Promise<string> {
    this.requireOnline();
    const [created] = await this.request(new Packet(messages.Client751.opcode, this.nextTransaction())
      .setText(1, requiredUsername(peer)).setText(2, requiredUsername(invitee)).setByte(3, 1), [750], 750);
    const id = created.text(1);
    if (!id) throw new Error('The server did not supply a group identifier');
    return id;
  }

  async inviteToGroup(id: string, invitee: string): Promise<void> {
    this.requireOnline();
    await this.request(new Packet(messages.Client752.opcode, this.nextTransaction())
      .setText(1, requiredRoomName(id)).setText(2, requiredUsername(invitee)).setByte(3, 1), [1], 1);
  }

  async leaveGroup(id: string): Promise<void> {
    this.requireOnline();
    await this.request(new Packet(messages.Client753.opcode, this.nextTransaction())
      .setText(1, requiredRoomName(id)).setByte(2, 1), [1], 1);
  }

  async getGroupParticipants(id: string): Promise<RoomParticipants> {
    this.requireOnline();
    const group = requiredRoomName(id);
    const [packet] = await this.request(new Packet(messages.Client754.opcode, this.nextTransaction())
      .setText(1, group).setByte(2, 1), [755], 755);
    return participantList(group, packet);
  }

  async sendGroupMessage(id: string, text: string, color?: number): Promise<void> {
    this.requireOnline();
    if (!text) throw new Error('Enter a message');
    const packet = new Packet(messages.Client500.opcode, this.nextTransaction())
      .setByte(1, 1).setText(2, this.name).setByte(3, 2).setText(4, requiredRoomName(id)).setShort(6, 1).setText(8, text);
    applyColor(packet, color);
    await this.request(packet, [1], 1);
  }

  async sendPhoto(kind: 1 | 2 | 3, target: string, data: Uint8Array | string): Promise<void> {
    this.requireOnline();
    const packet = new Packet(messages.Client500.opcode, this.nextTransaction()).setByte(1, 1).setText(2, this.name)
      .setByte(3, kind).setText(4, kind === 1 ? requiredUsername(target) : requiredRoomName(target))
      .setShort(6, typeof data === 'string' ? 5 : 2);
    if (typeof data === 'string') packet.setText(8, data); else packet.setBytes(8, data);
    encodeFrame(packet); // Validate locally before an oversized upload enters the transport.
    await this.request(packet, [1], 1);
  }

  async uploadPhoto(data: Uint8Array, purpose: 0 | 1 | 2 = 0, parameter = -1): Promise<string | undefined> {
    this.requireOnline();
    const packet = new Packet(messages.Client504.opcode, this.nextTransaction()).setShort(1, 1).setBytes(2, data).setByte(5, purpose);
    if (parameter !== -1) packet.setShort(4, parameter);
    encodeFrame(packet);
    const [ack] = await this.request(packet, [1], 1);
    return ack.text(1);
  }

  async getEmoticon(token: string): Promise<EmoticonAsset> {
    this.requireOnline();
    if (!token.trim() || /\s/.test(token.trim())) throw new Error('Choose one emoticon or photo token');
    //The action-mode0 fetch does not trigger vibration or audio side effects.
    const [packet] = await this.request(new Packet(messages.Client913.opcode, this.nextTransaction())
      .setText(1, token.trim() + ' ').setInt(2, 0), [914], 914);
    return decodeEmoticon(packet);
  }

  readonly requestOriginal: OriginalRequest = async (opcode, configure, replies, complete) => {
    this.requireOnline();
    const packet = new Packet(opcode, this.nextTransaction());
    configure(packet);
    encodeFrame(packet);
    return this.request(packet, replies, complete);
  }

  async logout(): Promise<void> {
    if (this.current !== 'online') { this.close(); return; }
    try { await this.request(new Packet(messages.Client300.opcode, this.nextTransaction()), [1], 1); }
    finally { this.close(); }
  }

  close(): void {
    if (this.current === 'closed') return;
    this.current = 'closed';
    this.stop(new Error('Client closed'));
    this.changeState('closed');
  }

  private requireOnline(): void {
    if (this.current !== 'online') throw new Error('Log in before using chat');
  }

  private expectLoginState(expected: ClientState): void {
    if (this.current !== expected) throw new Error('Login cancelled');
  }

  private nextTransaction(): number {
    for (let attempt = 0; attempt < 32767; attempt++) {
      this.counter = this.counter % 32767 + 1;
      if (!this.pending.has(this.counter)) return this.counter;
    }
    throw new Error('Too many pending requests');
  }

  private request(packet: Packet, replies: readonly number[], complete: number | readonly number[]): Promise<Packet[]> {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.fail(new Error('Request timed out; its outcome is unknown. Reconnect before sending again.'));
      }, this.requestTimeout);
      this.pending.set(packet.transaction, { request: packet, packets: [], replies: new Set(replies),
        complete: new Set(typeof complete === 'number' ? [complete] : complete), resolve, reject, timer });
      this.lastActivity = Date.now();
      void Promise.resolve().then(() => {
        if (this.current === 'closed' || this.current === 'error') throw new Error('Client closed');
        return this.transport.send(packet);
      }).catch((error) => this.fail(asError(error)));
    });
  }

  private receive(packet: Packet): void {
    if (this.current === 'closed' || this.current === 'error') return;
    this.lastActivity = Date.now();
    const pending = this.pending.get(packet.transaction);
    // Apply ACK-only mutations in wire order. Promise continuations run after
    // this batch and must never overwrite a newer402/401/603 push.
    if (packet.opcode === 1 && pending?.replies.has(1)) this.applyMutationAck(pending.request);
    if (packet.opcode === 750) {
      const id = packet.text(1);
      if (id) this.emit({ type: 'group-created', id,
        peer: pending?.request.opcode === 751 ? pending.request.text(1) : packet.text(2), service: packet.byte(3) ?? 1 });
    }
    //701 also represents listings and favorites. Only a correlated703 creates
    //room membership, synchronously before the batch's ACK and welcome500.
    if (packet.opcode === 701 && pending?.request.opcode === 703) {
      const room = roomSummary(packet);
      if (room) this.emit({ type: 'room-joining', requestedName: pending.request.text(1) ?? room.name, room });
    }
    if (pending && packet.opcode === 0) {
      clearTimeout(pending.timer);
      this.pending.delete(packet.transaction);
      pending.reject(new RemoteError(packet));
    } else if (pending?.replies.has(packet.opcode)) {
      pending.packets.push(packet);
      if (pending.complete.has(packet.opcode)) {
        clearTimeout(pending.timer);
        this.pending.delete(packet.transaction);
        pending.resolve(pending.packets);
      }
    }
    if (packet.opcode === 301) {
      this.fail(new Error(packet.text(1) ?? 'The server ended the session'));
      return;
    }
    if (packet.opcode === 500) {
      this.emit({ type: 'message', message: {
        sender: packet.text(2) ?? '', target: packet.text(4) ?? '',
        body: packet.text(8) ?? '', kind: packet.byte(3) ?? 1,
        content: packet.short(6) ?? 1, packet,
      } });
    }
    if (packet.opcode === 5) this.emit({ type: 'notice', title: packet.text(4) ?? '', text: packet.text(2) ?? '' });
    if (packet.opcode === 203) {
      this.emit({ type: 'site', url: packet.text(26) ?? '' });
      this.emit({ type: 'own-status', status: packet.text(28) ?? '' });
    }
    if (packet.opcode === 603 && packet.text(1) !== undefined) this.emit({ type: 'own-status', status: packet.text(1)! });
    if (packet.opcode === 924) this.emit({ type: 'colors', colors: [0, ...(packet.text(2)?.split(';') ?? [])
      .filter((value) => /^\d+$/.test(value)).map(Number).filter((value) => value > 0 && value <= 0xffffff)] });
    if (packet.opcode === 916) this.emit({ type: 'emoticons', catalog: {
      standard: tokens(packet.text(1)), other: tokens(packet.text(2)),
    } });
    if (packet.opcode === 915) this.emit({ type: 'emoticons-cleared' });
    if (packet.opcode === 911 && packet.int(1) !== undefined && packet.int(2) !== undefined) {
      this.emit({ type: 'server-menu', entry: { category: packet.int(1)!, index: packet.int(2)!,
        label: packet.text(3) ?? '', value: packet.text(4) ?? '', packet } });
    }
    if (this.current === 'online') {
      if (packet.opcode === 401) {
        const id = packet.int(1);
        if (id !== undefined) this.contactGroups.set(id, { id, name: packet.text(2) ?? '' });
        if (!pending) this.emit({ type: 'groups', groups: this.groups });
      }
      if (packet.opcode === 402) this.applyContactRecord(packet);
      if (packet.opcode === 406) this.roster.delete(packet.int(1) ?? -1);
      if (packet.opcode === 404) {
        const contact = this.roster.get(packet.int(1) ?? -1);
        if (contact && packet.byte(2) !== undefined) {
          this.roster.set(contact.id, { ...contact, presence: packet.byte(2)! });
        }
      }
      // Original bc dispatches421 to t.h status and423 to t.p avatar.
      // Their cache checkpoint
      // remains raw until incremental contact-cache persistence is supported.
      if (packet.opcode === 421 || packet.opcode === 423) {
        const contact = this.roster.get(packet.int(1) ?? -1);
        if (contact) this.roster.set(contact.id, {
          ...contact,
          ...(packet.opcode === 421 ? { status: packet.text(2) ?? '' } : { avatar: packet.text(2) ?? '' }),
        });
      }
      if (!pending && [402, 404, 406, 421, 423].includes(packet.opcode)) {
        this.emit({ type: 'contacts', contacts: this.contacts });
      }
    }
    this.emit({ type: 'packet', packet });
  }

  private applyMutationAck(request: Packet): void {
    const id = request.int(1) ?? -1;
    switch (request.opcode) {
      case 406:
        this.roster.delete(id);
        this.emit({ type: 'contacts', contacts: this.contacts });
        break;
      case 409:
        this.contactGroups.delete(id);
        this.emit({ type: 'groups', groups: this.groups });
        break;
      case 410:
        if (this.contactGroups.has(id)) this.contactGroups.set(id, { id, name: request.text(2) ?? '' });
        this.emit({ type: 'groups', groups: this.groups });
        break;
      case 411: {
        const contact = this.roster.get(id);
        if (contact) this.roster.set(id, { ...contact, groupId: request.int(2) ?? -1 });
        this.emit({ type: 'contacts', contacts: this.contacts });
        break;
      }
      case 600: this.emit({ type: 'own-presence', presence: request.byte(1) as OwnPresence }); break;
      case 601: this.emit({ type: 'own-status', status: request.text(1) ?? '' }); break;
    }
  }

  private applyContactRecord(packet: Packet): void {
    const id = packet.int(1);
    if (id === undefined) return;
    // Original bc.a:([Lay;)V offset11135 constructs a fresh t for each402.
    // Optional absent fields do not retain metadata from the previous record.
    const username = packet.text(12) ?? '';
    this.roster.set(id, {
      id, groupId: packet.int(2) ?? -1, username,
      display: packet.text(3) ?? username,
      presence: packet.byte(13) ?? 99,
      status: packet.text(27) ?? '', avatar: packet.text(28) ?? '', packet,
    });
  }

  private stop(error: Error): void {
    clearInterval(this.heartbeat);
    this.transport.close();
    for (const pending of this.pending.values()) {
      clearTimeout(pending.timer);
      pending.reject(error);
    }
    this.pending.clear();
  }

  private fail(error: Error): void {
    if (this.current === 'closed' || this.current === 'error') return;
    this.current = 'error';
    this.stop(error);
    this.changeState('error', error);
  }

  private changeState(state: ClientState, error?: Error): void {
    this.current = state;
    this.emit({ type: 'state', state, error });
  }

  private emit(event: ClientEvent): void {
    for (const listener of this.listeners) listener(event);
  }
}

function requiredRoomName(name: string): string {
  const value = name.trim();
  if (!value) throw new Error('Choose a room');
  return value;
}

function positiveId(id: number): number {
  if (!Number.isInteger(id) || id <= 0 || id > 0x7fffffff) throw new Error('Choose a valid contact or group');
  return id;
}
function requiredGroupName(name: string): string {
  if (!name.trim()) throw new Error('Enter a group name');
  return name.trim();
}
function applyContactDetails(packet: Packet, details: ContactDetails): void {
  if (details.groupId !== undefined) packet.setInt(2, details.groupId === -1 ? -1 : positiveId(details.groupId));
  if (details.display !== undefined) packet.setText(3, details.display);
  if (details.mobile !== undefined) packet.setText(8, details.mobile);
}

function roomSummary(packet: Packet): RoomSummary | undefined {
  const name = packet.text(1);
  if (!name) return undefined;
  const communityId = packet.int(8);
  return { name, capacity: packet.int(3), occupancy: packet.int(4), categoryId: packet.short(6),
    communityId: communityId !== undefined && communityId > 0 ? communityId : undefined, packet };
}

function terminatedNames(value: string | undefined): string[] {
  // Original708 lists are semicolon-terminated, and preserve server ordering.
  return value?.split(';').slice(0, -1).filter((name) => name !== '') ?? [];
}

function tokens(value: string | undefined): string[] { return value?.split(/\s+/).filter(Boolean) ?? []; }
function requiredUsername(value: string): string {
  const username = value.trim().toLowerCase();
  if (!username) throw new Error('Choose a user');
  return username;
}
function applyColor(packet: Packet, value: number | undefined): void {
  if (value === undefined || value === -1) return;
  if (!Number.isInteger(value) || value < 0 || value > 0xffffff) throw new Error('Choose a valid message color');
  packet.setInt(13, value);
}
function participantList(room: string, packet: Packet): RoomParticipants {
  const moderators = new Set(terminatedNames(packet.text(3)).map((name) => name.toLowerCase()));
  const muted = new Set(terminatedNames(packet.text(4)).map((name) => name.toLowerCase()));
  return { room, service: packet.byte(5) ?? 1, packet,
    participants: terminatedNames(packet.text(2)).map((username) => ({ username,
      moderator: moderators.has(username.toLowerCase()), muted: muted.has(username.toLowerCase()) })) };
}
