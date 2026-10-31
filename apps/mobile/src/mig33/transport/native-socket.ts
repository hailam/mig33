import type { ByteSocket, SocketConnector } from './tcp.ts';

/** The small native boundary is injectable without loading React Native in tests. */
export interface NativeByteSocket {
  on(event: 'connect' | 'secureConnect' | 'close', handler: () => void): unknown;
  on(event: 'error', handler: (error: unknown) => void): unknown;
  on(event: 'data', handler: (bytes: Uint8Array | string) => void): unknown;
  removeAllListeners(): unknown;
  destroy(): unknown;
  setNoDelay(enabled: boolean): unknown;
  setKeepAlive(enabled: boolean): unknown;
  write(bytes: Uint8Array, encoding: undefined, callback: (error?: Error) => void): unknown;
}

export interface NativeSocketHandle {
  socket: NativeByteSocket;
  /** Called only after every listener and the cancellation handler are attached. */
  start(): void;
}

function asError(value: unknown): Error {
  return value instanceof Error ? value : new Error(String(value));
}

/**
 * TLS requires the patched native handshake event, not an ordinary TCP connect.
 * Early bytes fail closed: no protocol challenge or credential can cross the
 * shared framing boundary until the OS has authenticated the peer.
 */
export function nativeSocketConnector(
  create: () => NativeSocketHandle,
  secure: boolean,
): SocketConnector {
  const name = secure ? 'TLS' : 'TCP';
  return (events, signal) => new Promise<ByteSocket>((resolve, reject) => {
    if (signal.aborted) {
      reject(new Error(`${name} connection cancelled.`));
      return;
    }
    let handle: NativeSocketHandle;
    try { handle = create(); } catch (error) { reject(asError(error)); return; }
    const { socket } = handle;
    const pendingWrites = new Set<(reason: Error) => void>();
    let connected = false;
    let closed = false;

    const shutdown = (reason: Error, reportError = false, destroy = true) => {
      if (closed) return;
      closed = true;
      signal.removeEventListener('abort', onAbort);
      socket.removeAllListeners();
      if (destroy) {
        try { socket.destroy(); } catch (error) { reason = asError(error); reportError = true; }
      }
      for (const fail of pendingWrites) fail(reason);
      pendingWrites.clear();
      if (!connected) reject(reason);
      else {
        if (reportError) events.error(reason);
        events.close();
      }
    };
    const onAbort = () => shutdown(new Error(`${name} connection cancelled.`));
    signal.addEventListener('abort', onAbort, { once: true });

    socket.on('error', (error) => shutdown(asError(error), true));
    socket.on('close', () => shutdown(new Error(`${name} connection closed.`), false, false));
    socket.on('data', (bytes) => {
      if (closed) return;
      if (!connected) {
        shutdown(new Error(`${name} received data before ${secure ? 'peer authentication' : 'connection readiness'}.`), true);
        return;
      }
      if (typeof bytes === 'string') {
        shutdown(new Error(`${name} transport received text instead of binary bytes.`), true);
        return;
      }
      events.data(new Uint8Array(bytes));
    });
    socket.on(secure ? 'secureConnect' : 'connect', () => {
      if (closed || connected) return;
      try {
        socket.setNoDelay(true);
        socket.setKeepAlive(true);
      } catch (error) { shutdown(asError(error), true); return; }
      connected = true;
      resolve({
        write(bytes) {
          return new Promise<void>((resolveWrite, rejectWrite) => {
            if (closed) { rejectWrite(new Error(`${name} connection is closed.`)); return; }
            const fail = (error: Error) => {
              pendingWrites.delete(fail);
              rejectWrite(error);
            };
            pendingWrites.add(fail);
            try {
              // Native completion, rather than the enqueue return value, is
              // the shared transport's backpressure boundary.
              socket.write(bytes, undefined, (error) => {
                if (!pendingWrites.delete(fail)) return;
                if (error) {
                  rejectWrite(asError(error));
                  shutdown(asError(error), true);
                } else resolveWrite();
              });
            } catch (error) { fail(asError(error)); shutdown(asError(error), true); }
          });
        },
        close() { shutdown(new Error(`${name} connection closed.`)); },
      });
    });

    // A factory can synchronously cancel while constructing its native handle.
    if (signal.aborted) { onAbort(); return; }
    try { handle.start(); } catch (error) { shutdown(asError(error), true); }
  });
}
