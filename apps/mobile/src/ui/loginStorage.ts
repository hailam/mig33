import type { ConnectionSettings, LoginCredentials } from './session/model.ts';

export interface RememberedLogin extends LoginCredentials {
  version: 1;
  auto: boolean;
  connection: ConnectionSettings;
}
export interface SecretStorage {
  read(): Promise<string | null>;
  write(value: string): Promise<void>;
  remove(): Promise<void>;
}

export function decodeLogin(raw: string | null): RememberedLogin | null {
  if (raw === null) return null;
  let value: unknown;
  try { value = JSON.parse(raw); } catch { return null; }
  if (!value || typeof value !== 'object') return null;
  const v = value as Record<string, unknown>;
  if (v.version !== 1 || typeof v.username !== 'string' || !v.username.trim()
    || typeof v.password !== 'string' || !v.password || typeof v.auto !== 'boolean'
    || typeof v.invisible !== 'boolean' || !v.connection || typeof v.connection !== 'object') return null;
  const c = v.connection as Record<string, unknown>;
  if (c.mode === 'http') {
    if (typeof c.httpURL !== 'string') return null;
    try { if (!['http:', 'https:'].includes(new URL(c.httpURL).protocol)) return null; } catch { return null; }
  } else if (c.mode === 'tcp') {
    if (typeof c.host !== 'string' || !c.host.trim() || typeof c.port !== 'string'
      || !/^\d+$/.test(c.port) || Number(c.port) < 1 || Number(c.port) > 65535) return null;
  } else return null;
  return v as unknown as RememberedLogin;
}

/** Serialize changes so a late save cannot resurrect credentials after Forget. */
export class LoginStorage {
  private pending: Promise<void> = Promise.resolve();
  private readonly storage: SecretStorage;
  constructor(storage: SecretStorage) { this.storage = storage; }
  async load(): Promise<RememberedLogin | null> {
    await this.pending;
    const raw = await this.storage.read();
    const value = decodeLogin(raw);
    if (raw !== null && value === null) await this.forget();
    return value;
  }
  save(value: RememberedLogin): Promise<void> {
    return this.enqueue(() => this.storage.write(JSON.stringify(value)));
  }
  forget(): Promise<void> { return this.enqueue(() => this.storage.remove()); }
  private enqueue(action: () => Promise<void>): Promise<void> {
    const operation = this.pending.then(action);
    this.pending = operation.catch(() => {});
    return operation;
  }
}
