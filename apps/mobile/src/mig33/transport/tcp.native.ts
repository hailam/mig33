import TcpSocket from 'react-native-tcp-socket';

import { TCPTransport, type ByteSocket, type SocketConnector } from './tcp.ts';

function asError(value: unknown): Error {
  return value instanceof Error ? value : new Error(String(value));
}

/**
 * Native socket callbacks deliver arbitrary byte chunks. Framing, transaction
 * correlation and protocol heartbeat handling belong to the shared client.
 * This adapter requires an Expo development build, not Expo Go.
 */
export function createTCPTransport(host: string, port: number): TCPTransport {
  if (!host.trim() || !Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error('A server host and TCP port between 1 and 65535 are required.');
  }
  const connect: SocketConnector = (events, signal) => new Promise<ByteSocket>((resolve, reject) => {
    if (signal.aborted) {
      reject(new Error('TCP connection cancelled.'));
      return;
    }
    const socket = new TcpSocket.Socket();
    const pendingWrites = new Set<(reason: Error) => void>();
    let connected = false;
    let closed = false;

    const shutdown = (reason: Error, reportError = false, destroy = true) => {
      if (closed) return;
      closed = true;
      signal.removeEventListener('abort', onAbort);
      socket.removeAllListeners();
      if (destroy) {
        try {
          socket.destroy();
        } catch (error) {
          reason = asError(error);
          reportError = true;
        }
      }
      for (const fail of pendingWrites) fail(reason);
      pendingWrites.clear();
      if (!connected) reject(reason);
      else {
        if (reportError) events.error(reason);
        events.close();
      }
    };
    const onAbort = () => shutdown(new Error('TCP connection cancelled.'));
    signal.addEventListener('abort', onAbort, { once: true });

    socket.on('error', (error) => shutdown(asError(error), true));
    socket.on('close', () => shutdown(new Error('TCP connection closed.'), false, false));
    socket.on('data', (bytes) => {
      if (closed) return;
      if (typeof bytes === 'string') {
        shutdown(new Error('TCP transport received text instead of binary bytes.'), true);
        return;
      }
      // Copy the native buffer; its lifetime must not constrain shared framing.
      events.data(new Uint8Array(bytes));
    });
    socket.on('connect', () => {
      if (closed) return;
      try {
        socket.setNoDelay(true);
        socket.setKeepAlive(true);
      } catch (error) {
        shutdown(asError(error), true);
        return;
      }
      connected = true;
      resolve({
        write(bytes) {
          return new Promise<void>((resolveWrite, rejectWrite) => {
            if (closed) {
              rejectWrite(new Error('TCP connection is closed.'));
              return;
            }
            const fail = (error: Error) => {
              pendingWrites.delete(fail);
              rejectWrite(error);
            };
            pendingWrites.add(fail);
            try {
              // Resolve after native completion, not the enqueue return value.
              // The shared transport serializes writes, providing backpressure.
              socket.write(bytes, undefined, (error) => {
                pendingWrites.delete(fail);
                if (error) {
                  rejectWrite(asError(error));
                  shutdown(asError(error), true);
                } else resolveWrite();
              });
            } catch (error) {
              fail(asError(error));
              shutdown(asError(error), true);
            }
          });
        },
        close() {
          shutdown(new Error('TCP connection closed.'));
        },
      });
    });

    try {
      // The shared connector also aborts its deadline. Native connectTimeout
      // prevents an OS-level connect from lingering after a failed attempt.
      socket.connect({ host: host.trim(), port, connectTimeout: 10_000 });
    } catch (error) {
      shutdown(asError(error), true);
    }
  });
  return new TCPTransport(connect);
}
