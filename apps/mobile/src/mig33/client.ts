import { messageDescriptors as messages } from './generated/contracts.ts';
import { Packet, loginChallengeHash } from './protocol/index.ts';
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
  username: string;
  display: string;
  presence: number;
  packet: Packet;
}
export type ClientEvent =
  | { type: 'state'; state: ClientState; error?: Error }
  | { type: 'message'; message: ChatMessage }
  | { type: 'contacts'; contacts: readonly Contact[] }
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
  packets: Packet[];
  replies: Set<number>;
  complete: number;
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
}

/** One live original-protocol session. Never automatically replay a failed send. */
export class Mig33Client {
  readonly transport: PacketTransport;
  private readonly requestTimeout: number;
  private readonly heartbeatInterval: number;
  private readonly listeners = new Set<(event: ClientEvent) => void>();
  private readonly pending = new Map<number, Pending>();
  private readonly roster = new Map<number, Contact>();
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
        .setText(8, options.platform ?? 'mig33 Expo').setByte(9, 1)
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
    try {
      await this.request(new Packet(messages.Client400.opcode, this.nextTransaction()), [401, 402, 420, 403], 403);
      // receive applies snapshot rows and pushes in wire order. Replaying rows
      // here would undo a push delivered in the same batch after403.
      this.emit({ type: 'contacts', contacts: this.contacts });
      return this.contacts;
    } finally { this.syncing = false; }
  }

  async sendMessage(recipient: string, text: string): Promise<void> {
    this.requireOnline();
    if (!recipient.trim() || !text) throw new Error('Enter a recipient and message');
    const packet = new Packet(messages.Client500.opcode, this.nextTransaction())
      .setByte(1, 1).setText(2, this.name).setByte(3, 1)
      .setText(4, recipient.trim().toLowerCase()).setShort(6, 1).setText(8, text);
    await this.request(packet, [1], 1);
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

  private request(packet: Packet, replies: number[], complete: number): Promise<Packet[]> {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.fail(new Error('Request timed out; its outcome is unknown. Reconnect before sending again.'));
      }, this.requestTimeout);
      this.pending.set(packet.transaction, { packets: [], replies: new Set(replies), complete, resolve, reject, timer });
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
    if (pending && packet.opcode === 0) {
      clearTimeout(pending.timer);
      this.pending.delete(packet.transaction);
      pending.reject(new RemoteError(packet));
    } else if (pending?.replies.has(packet.opcode)) {
      pending.packets.push(packet);
      if (packet.opcode === pending.complete) {
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
    if (this.current === 'online') {
      if (packet.opcode === 402) this.updateContact(packet);
      if (packet.opcode === 406) this.roster.delete(packet.int(1) ?? -1);
      if (packet.opcode === 404) {
        const contact = this.roster.get(packet.int(1) ?? -1);
        if (contact && packet.byte(2) !== undefined) contact.presence = packet.byte(2)!;
      }
      if (!pending && [402, 404, 406].includes(packet.opcode)) this.emit({ type: 'contacts', contacts: this.contacts });
    }
    this.emit({ type: 'packet', packet });
  }

  private updateContact(packet: Packet): void {
    const id = packet.int(1);
    if (id === undefined) return;
    this.roster.set(id, {
      id, username: packet.text(12) ?? '', display: packet.text(3) ?? packet.text(12) ?? '',
      presence: packet.byte(13) ?? 99, packet,
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
