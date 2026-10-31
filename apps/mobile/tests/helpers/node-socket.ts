import { createConnection } from 'node:net';
import type { SocketConnector } from '../../src/mig33/transport/tcp.ts';

/** Node adapter for protocol acceptance, not an Expo-native TCP implementation. */
export function nodeConnector(host: string, port: number): SocketConnector {
  return (events, signal) => new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(new Error('TCP connection cancelled'));
      return;
    }
    const socket = createConnection({ host, port });
    let connected = false;
    const abort = () => socket.destroy(new Error('TCP connection cancelled'));
    signal.addEventListener('abort', abort, { once: true });
    socket.setNoDelay(true);
    socket.on('data', (bytes) => {
      if (typeof bytes === 'string') {
        socket.destroy(new Error('Binary TCP socket unexpectedly decoded text'));
        return;
      }
      events.data(new Uint8Array(bytes));
    });
    socket.on('error', (error) => {
      if (!connected) reject(error);
      else events.error(error);
    });
    socket.on('close', () => {
      signal.removeEventListener('abort', abort);
      if (!connected) reject(new Error('TCP closed before connecting'));
      events.close();
    });
    socket.once('connect', () => {
      if (signal.aborted) { abort(); return; }
      connected = true;
      resolve({
        write: (bytes) => new Promise<void>((resolveWrite, rejectWrite) => {
          if (!socket.writable) { rejectWrite(new Error('TCP socket is closed')); return; }
          socket.write(bytes, (error) => error ? rejectWrite(error) : resolveWrite());
        }),
        close: () => socket.destroy(),
      });
    });
  });
}
