import assert from 'node:assert/strict';
import test from 'node:test';

import { Mig33Client, RemoteError, type ChatMessage } from '../src/mig33/client.ts';
import { HTTPTransport } from '../src/mig33/transport/http.ts';
import { TCPTransport } from '../src/mig33/transport/tcp.ts';
import { nodeConnector } from './helpers/node-socket.ts';
import { startServer } from './helpers/server.ts';

function receive(client: Mig33Client, sender: string, body: string): Promise<ChatMessage> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => { unsubscribe(); reject(new Error('Message delivery timed out')); }, 10000);
    const unsubscribe = client.subscribe((event) => {
      if (event.type === 'message' && event.message.sender === sender && event.message.body === body) {
        clearTimeout(timer);
        unsubscribe();
        resolve(event.message);
      }
    });
  });
}

test('shared client logs in, syncs contacts and exchanges messages across both original transports', {
  skip: !process.env.MIG33_TEST_SERVER_BINARY, timeout: 60000,
}, async (t) => {
  for (const aliceMode of ['http', 'tcp'] as const) {
    await t.test(`Alice over ${aliceMode}, Bob over ${aliceMode === 'http' ? 'tcp' : 'http'}`, async () => {
      const server = await startServer();
      const http = () => new HTTPTransport(server.httpURL, { pollIntervalMs: 40 });
      const tcp = () => new TCPTransport(nodeConnector('127.0.0.1', server.tcpPort));
      const invalid = new Mig33Client(http(), { requestTimeoutMs: 10000 });
      const alice = new Mig33Client(aliceMode === 'http' ? http() : tcp(), { requestTimeoutMs: 10000 });
      const bob = new Mig33Client(aliceMode === 'http' ? tcp() : http(), { requestTimeoutMs: 10000 });
      try {
        await assert.rejects(invalid.login({ username: 'alice', password: 'wrong-password' }), RemoteError);
        assert.equal(invalid.state, 'error');
        await alice.login({ username: 'alice', password: 'alice-demo' });
        await bob.login({ username: 'bob', password: 'bob-demo' });
        const aliceContacts = await alice.syncContacts();
        const bobContacts = await bob.syncContacts();
        assert.ok(aliceContacts.some((contact) => contact.username === 'bob'));
        assert.ok(bobContacts.some((contact) => contact.username === 'alice'));
        const first = `Hello ${aliceMode} ✓ ދިވެހި 😀`;
        const incomingBob = receive(bob, 'alice', first);
        // Attach the receive rejection immediately if a send fails first.
        void incomingBob.catch(() => {});
        await alice.sendMessage('bob', first);
        const delivered = await incomingBob;
        assert.equal(delivered.kind, 1);
        assert.equal(delivered.content, 1);
        const reply = `Reply to ${aliceMode}`;
        const incomingAlice = receive(alice, 'bob', reply);
        void incomingAlice.catch(() => {});
        await bob.sendMessage('alice', reply);
        assert.equal((await incomingAlice).body, reply);
        await bob.logout();
        await alice.logout();
        assert.equal(alice.state, 'closed');
        assert.equal(bob.state, 'closed');
      } finally {
        invalid.close();
        alice.close();
        bob.close();
        await server.close();
      }
    });
  }
});
