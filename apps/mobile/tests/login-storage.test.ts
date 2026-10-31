import assert from 'node:assert/strict';
import { test } from 'node:test';
import { decodeLogin, LoginStorage, RetiredLoginError, type RememberedLogin } from '../src/ui/loginStorage.ts';

const saved: RememberedLogin = { version: 2, username: 'alice', password: 'alice-demo', auto: true,
  invisible: false, connection: { mode: 'https', httpURL: 'https://localhost:8443' } };

test('remembered login remains bound to its selected server and transport', () => {
  assert.deepEqual(decodeLogin(JSON.stringify(saved)), saved);
  assert.equal(decodeLogin('{broken'), null);
  assert.equal(decodeLogin(JSON.stringify({ ...saved, connection: { mode: 'https', httpURL: 'http://localhost' } })), null);
  assert.equal(decodeLogin(JSON.stringify({ ...saved, connection: { mode: 'tls', host: 'localhost', port: '0' } })), null);
  const tls = { ...saved, connection: { mode: 'tls', host: 'localhost', port: '9120' } };
  assert.deepEqual(decodeLogin(JSON.stringify(tls)), tls);
});

test('old remembered HTTP/TCP credentials are removed with an actionable notice and never restored for auto-login', async () => {
  for (const connection of [{ mode: 'http', httpURL: 'http://localhost:8080' },
    { mode: 'http', httpURL: 'https://localhost:8443' }, { mode: 'tcp', host: 'localhost', port: '9119' }]) {
    let raw: string | null = JSON.stringify({ ...saved, version: 1, connection });
    assert.equal(decodeLogin(raw), null);
    const store = new LoginStorage({ read: async () => raw, write: async (next) => { raw = next; }, remove: async () => { raw = null; } });
    await assert.rejects(store.load(), (error: unknown) => error instanceof RetiredLoginError && /HTTPS or TLS/.test(error.message));
    assert.equal(raw, null);
    assert.equal(await store.load(), null, 'No credentials or auto-login survive the migration rejection');
  }
});

test('remembering an insecure or malformed endpoint never writes secrets', async () => {
  let writes = 0;
  const store = new LoginStorage({ read: async () => null, write: async () => { writes++; }, remove: async () => {} });
  for (const httpURL of ['http://localhost:8080', 'https://user:password@localhost', 'https://localhost/path', 'https://localhost/?sid=secret']) {
    await assert.rejects(store.save({ ...saved, connection: { mode: 'https', httpURL } }), /HTTPS or TLS/);
  }
  assert.equal(writes, 0);
});

test('forget after a pending save does not leave credentials behind', async () => {
  let value: string | null = null;
  let release!: () => void;
  const gate = new Promise<void>((resolve) => { release = resolve; });
  const store = new LoginStorage({ read: async () => value,
    write: async (next) => { await gate; value = next; }, remove: async () => { value = null; } });
  const write = store.save(saved);
  const forget = store.forget();
  release();
  await Promise.all([write, forget]);
  assert.equal(await store.load(), null);
});

test('failed save does not prevent a later deletion and invalid stored data is removed', async () => {
  let value: string | null = '{}';
  const store = new LoginStorage({ read: async () => value, write: async () => { throw new Error('locked'); },
    remove: async () => { value = null; } });
  await assert.rejects(store.save(saved), /locked/);
  assert.equal(await store.load(), null);
  assert.equal(value, null);
});
