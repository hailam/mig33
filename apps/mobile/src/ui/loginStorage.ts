import type { ConnectionSettings, LoginCredentials } from './session/model.ts';
import { secureConnectionMessage, validateConnection } from './connectionSettings.ts';

export interface RememberedLogin extends LoginCredentials {
  version: 2;
  auto: boolean;
  connection: ConnectionSettings;
}
export interface SecretStorage {
  read(): Promise<string | null>;
  write(value: string): Promise<void>;
  remove(): Promise<void>;
}

export class RetiredLoginError extends Error {
  constructor() {
    super(`The saved login used an older connection and has been removed. ${secureConnectionMessage} Log in again to save your secure connection.`);
    this.name = 'RetiredLoginError';
  }
}

export function decodeLogin(raw: string | null): RememberedLogin | null {
  if (raw === null) return null;
  let value: unknown;
  try { value = JSON.parse(raw); } catch { return null; }
  if (!value || typeof value !== 'object') return null;
  const v = value as Record<string, unknown>;
  if (v.version !== 2 || typeof v.username !== 'string' || !v.username.trim()
    || typeof v.password !== 'string' || !v.password || typeof v.auto !== 'boolean'
    || typeof v.invisible !== 'boolean' || !v.connection || typeof v.connection !== 'object') return null;
  try {
    return { version: 2, username: v.username, password: v.password, auto: v.auto, invisible: v.invisible,
      connection: validateConnection(v.connection) };
  } catch { return null; }
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
    if (raw !== null && value === null) {
      await this.forget();
      let retired = false;
      try {
        const old = JSON.parse(raw);
        retired = old?.version === 1 || ['http', 'tcp'].includes(old?.connection?.mode);
      } catch { /* Invalid JSON is already removed. */ }
      if (retired) throw new RetiredLoginError();
    }
    return value;
  }
  save(value: RememberedLogin): Promise<void> {
    const normalized = decodeLogin(JSON.stringify(value));
    if (!normalized) return Promise.reject(new Error('Only a valid HTTPS or TLS login can be remembered.'));
    return this.enqueue(() => this.storage.write(JSON.stringify(normalized)));
  }
  forget(): Promise<void> { return this.enqueue(() => this.storage.remove()); }
  private enqueue(action: () => Promise<void>): Promise<void> {
    const operation = this.pending.then(action);
    this.pending = operation.catch(() => {});
    return operation;
  }
}
