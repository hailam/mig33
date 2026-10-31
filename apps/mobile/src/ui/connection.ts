import { Packet } from '../mig33/protocol';
import { HTTPTransport } from '../mig33/transport/http';
import { createTCPTransport } from '../mig33/transport/tcp.native';
import type { ConnectionSettings } from './session/model';
import type { ConnectionForm } from './screens/AuthScreens';

export function connectionFromForm(form: ConnectionForm): ConnectionSettings {
  const port = Number(form.port);
  if (!/^\d+$/.test(form.port) || !Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error('Enter a port from 1 to 65535.');
  }
  const host = form.host.trim();
  if (!host) throw new Error('Enter a server.');
  if (form.mode === 'http') {
    const url = new URL(host.includes('://') ? host : `http://${host}`);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) throw new Error('Enter an HTTP server address.');
    url.port = String(port);
    return { mode: 'http', httpURL: url.toString().replace(/\/$/, '') };
  }
  return { mode: 'tcp', host: host.replace(/^http:\/\//i, '').replace(/\/$/, ''), port: String(port) };
}

export function connectionToForm(connection: ConnectionSettings): ConnectionForm {
  if (connection.mode === 'tcp') return { ...connection };
  const url = new URL(connection.httpURL);
  const port = url.port || (url.protocol === 'https:' ? '443' : '80');
  url.port = '';
  return { mode: 'http', host: url.toString().replace(/\/$/, ''), port };
}

/** Original opcode2/3 connection probe; no login or application session is created. */
export function probeConnection(connection: ConnectionSettings, signal: AbortSignal): Promise<void> {
  const transport = connection.mode === 'http'
    ? new HTTPTransport(connection.httpURL)
    : createTCPTransport(connection.host, Number(connection.port));
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
