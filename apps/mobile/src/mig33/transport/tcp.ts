import { FrameDecoder, encodeFrame, type Packet } from '../protocol/index.ts';
import { asError, type PacketTransport, type TransportEvents } from './types.ts';

export interface SocketEvents {
  data(bytes: Uint8Array): void;
  error(error: Error): void;
  close(): void;
}
export interface ByteSocket {
  write(bytes: Uint8Array): Promise<void>;
  close(): void;
}
export type SocketConnector = (events: SocketEvents, signal: AbortSignal) => Promise<ByteSocket>;

/** The same buffering/write logic runs with native TCP and a Node test socket. */
export class TCPTransport implements PacketTransport {
  readonly kind = 'tcp' as const;
  private readonly connector: SocketConnector;
  private readonly connectTimeout: number;
  private readonly decoder = new FrameDecoder();
  private readonly controller = new AbortController();
  private socket?: ByteSocket;
  private events?: TransportEvents;
  private open = false;
  private queue: Promise<void> = Promise.resolve();

  constructor(connector: SocketConnector, connectTimeoutMs = 10000) {
    this.connector = connector;
    this.connectTimeout = connectTimeoutMs;
  }

  async connect(events: TransportEvents): Promise<void> {
    if (this.events || this.controller.signal.aborted) throw new Error('Create a new transport for a new connection');
    this.events = events;
    this.open = true;
    const timer = setTimeout(() => this.fail(new Error('TCP connection timed out')), this.connectTimeout);
    try {
      const socket = await this.connector({
        data: (bytes) => {
          if (!this.open) return;
          try {
            const packets = this.decoder.push(bytes);
            if (packets.length) this.events?.packets(packets);
          } catch (error) { this.fail(asError(error)); }
        },
        error: (error) => this.fail(error),
        close: () => {
          if (!this.open) return;
          try {
            this.decoder.finish();
            this.fail(new Error('TCP connection closed'));
          } catch (error) { this.fail(asError(error)); }
        },
      }, this.controller.signal);
      if (!this.open) {
        socket.close();
        throw new Error('TCP connection cancelled');
      }
      this.socket = socket;
    } catch (error) {
      this.fail(asError(error));
      throw error;
    } finally { clearTimeout(timer); }
  }

  setSession(_session: string): void {}
  activate(): void {}

  send(packet: Packet): Promise<void> {
    const bytes = encodeFrame(packet);
    const next = this.queue.then(async () => {
      if (!this.open || !this.socket) throw new Error('TCP transport is closed');
      try { await this.socket.write(bytes); }
      catch (error) {
        this.fail(asError(error));
        throw error;
      }
    });
    this.queue = next.catch(() => {});
    return next;
  }

  close(): void {
    this.open = false;
    this.controller.abort();
    this.socket?.close();
    this.socket = undefined;
    this.decoder.reset();
  }

  private fail(error: Error): void {
    if (!this.open) return;
    this.close();
    this.events?.closed(error);
  }
}
