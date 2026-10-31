import { Packet } from '../mig33/protocol';
import { HTTPTransport } from '../mig33/transport/http';
import { fetch as nativeFetch } from 'expo/fetch';
import { createTLSTransport } from '../mig33/transport/tcp.native';
import type { ConnectionSettings } from './session/model';
import { validateConnection } from './connectionSettings';
export { connectionFromForm, connectionToForm } from './connectionSettings';

/** Expo's native fetch honors redirect:error before following on iOS/Android;
 * React Native's global XMLHttpRequest-backed fetch does not. Never substitute
 * the global fetch here, even for the unauthenticated connection probe. */
export function createSecureTransport(value: ConnectionSettings) {
  const connection = validateConnection(value);
  return connection.mode === 'https'
    ? new HTTPTransport(connection.httpURL, { requireHTTPS: true, fetch: nativeFetch })
    : createTLSTransport(connection.host, Number(connection.port));
}

/** Original opcode2/3 connection probe; no login or application session is created. */
export function probeConnection(connection: ConnectionSettings, signal: AbortSignal): Promise<void> {
  const transport = createSecureTransport(connection);
  return new Promise((resolve, reject) => {
    let finished = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const finish = (error?: Error) => {
      if (finished) return;
      finished = true;
      clearTimeout(timer);
      signal.removeEventListener('abort', cancel);
      transport.close();
      if (error) reject(error); else resolve();
    };
    const cancel = () => finish(new Error('Connection test cancelled.'));
    signal.addEventListener('abort', cancel, { once: true });
    if (signal.aborted) { cancel(); return; }
    timer = setTimeout(() => finish(new Error('Connection test timed out.')), 15000);
    void transport.connect({
      packets: (packets) => {
        for (const packet of packets) {
          if (packet.transaction !== 1) continue;
          if (packet.opcode === 3) finish();
          else if (packet.opcode === 0) finish(new Error(packet.text(2) ?? 'Connection test rejected.'));
        }
      },
      closed: (error) => finish(error ?? new Error('Connection closed during the test.')),
    }).then(() => {
      if (!finished) return transport.send(new Packet(2, 1).setInt(1, 1));
    }).catch((error: unknown) => finish(error instanceof Error ? error : new Error(String(error))));
  });
}
