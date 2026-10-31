import assert from 'node:assert/strict';
import test from 'node:test';
import { connectionFromForm, connectionToForm, defaultConnections, validateConnection,
  type ConnectionSettings } from '../src/ui/connectionSettings.ts';
import { SessionController } from '../src/ui/session/controller.ts';

test('Expo connection settings accept only explicit HTTPS or TLS and preserve the selected secure endpoint', () => {
  assert.deepEqual(connectionFromForm({ mode: 'https', host: 'localhost', port: '8443' }), defaultConnections.https);
  assert.deepEqual(connectionFromForm({ mode: 'tls', host: 'localhost', port: '9120' }), defaultConnections.tls);
  for (const connection of [defaultConnections.https, defaultConnections.tls,
    { mode: 'https', httpURL: 'https://example.test' } as const,
    { mode: 'tls', host: '::1', port: '9120' } as const]) {
    assert.deepEqual(connectionFromForm(connectionToForm(connection)), connection);
  }
  for (const invalid of [null, {}, { mode: 'http', httpURL: 'https://localhost' },
    { mode: 'tcp', host: 'localhost', port: '9120' },
    ...['http://localhost', 'file:///tmp/a', 'https://u:p@localhost', 'https://localhost/route',
      'https://localhost/?x=1', 'https://localhost/#fragment', 'https://localhost:0'].map((httpURL) => ({ mode: 'https', httpURL })),
    ...['', 'http://localhost', 'host:443', '[::1]:443', 'host/path', 'host?x', 'host#x', 'u@host', 'host\\path']
      .map((host) => ({ mode: 'tls', host, port: '9120' })),
    ...['0', '65536', '-1', '4.5', ' 443', '443junk'].map((port) => ({ mode: 'tls', host: 'localhost', port }))]) {
    assert.throws(() => validateConnection(invalid));
  }
  assert.throws(() => connectionFromForm({ mode: 'https', host: 'http://localhost', port: '8443' }), /HTTPS/);
});

test('insecure runtime settings fail before the UI session can construct a client or send credentials', async () => {
  let clients = 0;
  const session = new SessionController(() => { clients++; throw new Error('Must not create a client'); }, 'test');
  for (const invalid of [{ mode: 'http', httpURL: 'http://localhost' },
    { mode: 'tcp', host: 'localhost', port: '9119' }, { mode: 'https', httpURL: 'http://localhost' }]) {
    assert.equal(await session.login({ username: 'alice', password: 'secret' }, invalid as ConnectionSettings), false);
    assert.equal(session.getSnapshot().state, 'error');
    assert.match(session.getSnapshot().error, /HTTPS/);
    assert.equal(clients, 0);
  }
  session.dispose();
});
