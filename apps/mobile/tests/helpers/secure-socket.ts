import { connect } from 'node:tls';
import { request } from 'node:https';
import type { SocketConnector } from '../../src/mig33/transport/tcp.ts';

/** Node acceptance connector: normal chain/hostname verification stays enabled. */
export function tlsConnector(host: string, port: number, ca: string): SocketConnector {
  return (events, signal) => new Promise((resolve, reject) => {
    if (signal.aborted) { reject(new Error('TLS connection cancelled')); return; }
    const socket = connect({ host, port, ca, minVersion: 'TLSv1.2', servername: host === 'localhost' ? host : undefined });
    let authenticated = false;
    const abort = () => socket.destroy(new Error('TLS connection cancelled'));
    signal.addEventListener('abort', abort, { once: true });
    socket.on('error', (error) => authenticated ? events.error(error) : reject(error));
    socket.on('close', () => {
      signal.removeEventListener('abort', abort);
      if (!authenticated) reject(new Error('TLS closed before authentication'));
      events.close();
    });
    socket.once('secureConnect', () => {
      if (signal.aborted) { abort(); return; }
      authenticated = true;
      socket.on('data', (bytes) => {
        if (typeof bytes === 'string') { socket.destroy(new Error('TLS returned text instead of bytes')); return; }
        events.data(new Uint8Array(bytes));
      });
      resolve({
        write: (bytes) => new Promise<void>((done, fail) => socket.write(bytes, (error) => error ? fail(error) : done())),
        close: () => socket.destroy(),
      });
    });
  });
}

/** Test-only custom CA fetch. It never follows redirects or disables TLS trust. */
export function trustedFetch(ca: string): typeof fetch {
  return async (input, init) => {
    const url = new URL(String(input));
    if (url.protocol !== 'https:' || init?.redirect !== 'error') throw new Error('Secure test fetch requires HTTPS and redirect:error');
    return new Promise<Response>((resolve, reject) => {
      const req = request(url, { ca, method: init.method, headers: Object.fromEntries(new Headers(init.headers)),
        signal: init.signal ?? undefined, minVersion: 'TLSv1.2' }, (response) => {
        if ((response.statusCode ?? 0) >= 300 && (response.statusCode ?? 0) < 400) {
          response.resume(); reject(new Error('Redirect rejected')); return;
        }
        const chunks: Buffer[] = [];
        response.on('data', (chunk: Buffer) => chunks.push(chunk));
        response.on('error', reject);
        response.on('end', () => {
          const headers = new Headers();
          for (const [name, value] of Object.entries(response.headers)) if (value !== undefined) headers.set(name, Array.isArray(value) ? value.join(', ') : value);
          resolve(new Response(Buffer.concat(chunks), { status: response.statusCode, headers }));
        });
      });
      req.on('error', reject);
      req.end(init.body ? Buffer.from(init.body as ArrayBuffer) : undefined);
    });
  };
}
