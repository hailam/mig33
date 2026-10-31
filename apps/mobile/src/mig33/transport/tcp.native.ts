import TcpSocket from 'react-native-tcp-socket';

import { nativeSocketConnector } from './native-socket.ts';
import { TCPTransport } from './tcp.ts';

function connectionOptions(host: string, port: number) {
  if (!host.trim() || !Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error('A server host and TCP port between 1 and 65535 are required.');
  }
  return { host: host.trim(), port, connectTimeout: 10_000 };
}

/** Original plaintext transport for compatibility tools; the app uses TLS. */
export function createTCPTransport(host: string, port: number): TCPTransport {
  const options = connectionOptions(host, port);
  return new TCPTransport(nativeSocketConnector(() => {
    const socket = new TcpSocket.Socket();
    return { socket, start: () => { socket.connect(options); } };
  }, false));
}

/**
 * TLS preserves the original framed socket protocol. Certificate chain and
 * hostname validation use OS trust; callers cannot supply a CA or bypass it.
 * Requires our pinned native handshake/Android identity-verification patch and
 * a rebuilt Expo development client (not Expo Go).
 */
export function createTLSTransport(host: string, port: number): TCPTransport {
  const options = connectionOptions(host, port);
  return new TCPTransport(nativeSocketConnector(() => {
    const raw = new TcpSocket.Socket();
    // Construct before connect so the native bridge knows this socket requires
    // TLS. Deliberately omit ca/rejectUnauthorized/other trust overrides.
    const socket = new TcpSocket.TLSSocket(raw);
    return { socket, start: () => { raw.connect(options); } };
  }, true));
}
