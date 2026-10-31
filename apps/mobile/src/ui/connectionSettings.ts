/** Expo connection policy. The shared protocol adapters separately retain the
 * original cleartext transports for historical-client compatibility tests. */
export type ConnectionSettings =
  | { mode: 'https'; httpURL: string }
  | { mode: 'tls'; host: string; port: string };
export interface ConnectionForm { host: string; port: string; mode: 'https' | 'tls'; }

export const defaultConnections: Record<ConnectionSettings['mode'], ConnectionSettings> = {
  https: { mode: 'https', httpURL: 'https://localhost:8443' },
  tls: { mode: 'tls', host: 'localhost', port: '9120' },
};
export const secureConnectionMessage = 'Choose HTTPS or TLS in Connection Settings. Unencrypted HTTP and TCP connections are no longer supported by this app.';

function portNumber(value: unknown): number {
  if (typeof value !== 'string' || !/^\d+$/.test(value) || Number(value) < 1 || Number(value) > 65535) {
    throw new Error('Enter a port from 1 to 65535.');
  }
  return Number(value);
}

function secureOrigin(value: unknown): URL {
  if (typeof value !== 'string') throw new Error('Enter an HTTPS server address.');
  let url: URL;
  try { url = new URL(value); } catch { throw new Error('Enter an HTTPS server address.'); }
  if (url.protocol !== 'https:' || !url.hostname || url.username || url.password ||
    url.pathname !== '/' || url.search || url.hash) {
    throw new Error('Use an HTTPS server origin without a path, credentials, query or fragment.');
  }
  if (url.port === '0') throw new Error('Enter a port from 1 to 65535.');
  return url;
}

function serverHost(value: unknown): string {
  if (typeof value !== 'string' || !value.trim() || /[\s/@?#\\]/.test(value) || value.includes('://')) {
    throw new Error('Enter a server hostname or IP address without a URL or port.');
  }
  const host = value.trim();
  if (host.startsWith('[') ? !host.endsWith(']') : host.split(':').length === 2) {
    throw new Error('Enter the server port in the Port field.');
  }
  const authority = host.includes(':') && !host.startsWith('[') && host.split(':').length > 2 ? `[${host}]` : host;
  let url: URL;
  try { url = new URL(`https://${authority}`); } catch { throw new Error('Enter a valid server hostname or IP address.'); }
  if (url.port || !url.hostname || url.pathname !== '/') throw new Error('Enter the server port in the Port field.');
  return url.hostname.replace(/^\[|\]$/g, '');
}

/** Validate runtime data too: saved settings and JS callers cannot bypass the
 * secure-only selector by supplying a value cast to ConnectionSettings. */
export function validateConnection(value: unknown): ConnectionSettings {
  if (!value || typeof value !== 'object') throw new Error(secureConnectionMessage);
  const connection = value as Record<string, unknown>;
  if (connection.mode === 'https') return { mode: 'https', httpURL: secureOrigin(connection.httpURL).origin };
  if (connection.mode === 'tls') return { mode: 'tls', host: serverHost(connection.host), port: String(portNumber(connection.port)) };
  throw new Error(secureConnectionMessage);
}

export function connectionFromForm(form: ConnectionForm): ConnectionSettings {
  const port = portNumber(form.port);
  if (form.mode === 'https') {
    const host = form.host.trim();
    const url = secureOrigin(host.includes('://') ? host : `https://${host}`);
    url.port = String(port);
    return validateConnection({ mode: 'https', httpURL: url.href });
  }
  return validateConnection({ ...form, port: String(port) });
}

export function connectionToForm(value: ConnectionSettings): ConnectionForm {
  const connection = validateConnection(value);
  if (connection.mode === 'tls') return connection;
  const url = new URL(connection.httpURL);
  const port = url.port || '443';
  url.port = '';
  return { mode: 'https', host: url.hostname, port };
}
