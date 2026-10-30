import type { Packet } from '../protocol/index.ts';

export interface TransportEvents {
  packets(packets: Packet[]): void;
  closed(error?: Error): void;
}

/** Packet transport only. Login, correlation and application state stay shared. */
export interface PacketTransport {
  readonly kind: 'http' | 'tcp';
  connect(events: TransportEvents): Promise<void>;
  send(packet: Packet): Promise<void>;
  setSession(session: string): void;
  activate(): void;
  close(): void;
}

export function asError(value: unknown): Error {
  return value instanceof Error ? value : new Error(String(value));
}
