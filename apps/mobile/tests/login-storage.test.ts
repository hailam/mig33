import assert from 'node:assert/strict';
import { test } from 'node:test';
import { decodeLogin, LoginStorage, type RememberedLogin } from '../src/ui/loginStorage.ts';

const saved: RememberedLogin = { version: 1, username: 'alice', password: 'alice-demo', auto: true,
  invisible: false, connection: { mode: 'http', httpURL: 'http://127.0.0.1:8080' } };

test('remembered login remains bound to its selected server and transport', () => {
  assert.deepEqual(decodeLogin(JSON.stringify(saved)), saved);
  assert.equal(decodeLogin('{broken'), null);
  assert.equal(decodeLogin(JSON.stringify({ ...saved, connection: { mode: 'http', httpURL: 'file:///tmp/a' } })), null);
  assert.equal(decodeLogin(JSON.stringify({ ...saved, connection: { mode: 'tcp', host: 'localhost', port: '0' } })), null);
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
