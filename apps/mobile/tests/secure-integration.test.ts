import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
import test from 'node:test';
import { Mig33Client, type ChatMessage } from '../src/mig33/client.ts';
import { HTTPTransport } from '../src/mig33/transport/http.ts';
import { TCPTransport } from '../src/mig33/transport/tcp.ts';
import { nodeConnector } from './helpers/node-socket.ts';
import { tlsConnector, trustedFetch } from './helpers/secure-socket.ts';
import { startServer } from './helpers/server.ts';

function received(client: Mig33Client, body: string): Promise<ChatMessage> {
  const result = new Promise<ChatMessage>((resolve, reject) => {
    const timer = setTimeout(() => { unsubscribe(); reject(new Error(`Missing delivery: ${body}`)); }, 10000);
    const unsubscribe = client.subscribe((event) => {
      if (event.type === 'message' && event.message.body === body) {
        clearTimeout(timer); unsubscribe(); resolve(event.message);
      }
    });
  });
  void result.catch(() => {});
  return result;
}

test('HTTPS and TLS sockets share original login, pages, media and chat with both retained plain listeners', {
  skip: !process.env.MIG33_TEST_SERVER_BINARY, timeout: 90000,
}, async () => {
  const server = await startServer(undefined, { tls: true });
  const secure = server.secure!;
  const ca = await readFile(join(secure.directory, 'ca.pem'), 'utf8');
  const options = { pollIntervalMs: 40, requireHTTPS: true, fetch: trustedFetch(ca) };
  const alice = new Mig33Client(new HTTPTransport(secure.httpsURL, options));
  const bob = new Mig33Client(new TCPTransport(tlsConnector('127.0.0.1', secure.tlsPort, ca)));
  const carol = new Mig33Client(new HTTPTransport(server.httpURL, { pollIntervalMs: 40 }));
  const hailam = new Mig33Client(new TCPTransport(nodeConnector('127.0.0.1', server.tcpPort)));
  const clients = [alice, bob, carol, hailam];
  const names = ['alice', 'bob', 'carol', 'hailam'];
  try {
    await Promise.all(clients.map((client, index) => client.login({
      username: names[index], password: names[index] === 'hailam' ? 'hailam' : names[index] + '-demo',
    })));
    for (let index = 0; index < clients.length; index++) {
      assert.equal((await clients[index].syncContacts()).length, names[index] === 'hailam' ? 0 : 4);
    }
    for (let index = 0; index < clients.length; index++) {
      const next = (index + 1) % clients.length;
      const body = `Four listeners ${names[index]} → ${names[next]} ✓`;
      const incoming = received(clients[next], body);
      await clients[index].sendMessage(names[next], body);
      assert.equal((await incoming).sender, names[index]);
    }
    await Promise.all(clients.map((client) => client.joinRoom('Lobby')));
    const roomBody = 'Encrypted room → original listeners';
    const roomReplies = clients.filter((client) => client !== bob).map((client) => received(client, roomBody));
    await bob.sendRoomMessage('Lobby', roomBody);
    for (const reply of await Promise.all(roomReplies)) assert.equal(reply.kind, 3);

    // Historical HTTP URL identifiers remain inside encrypted922; no browser
    // redirect or direct cleartext download is needed for service pages/media.
    for (const client of [alice, bob]) {
      const [page] = await client.requestOriginal(922, (packet) => {
        packet.setByte(1, 1).setText(2, server.httpURL + '/sites/index.php?c=profile&a=home&v=midlet&username=alice')
          .setInt(4, 1).setInt(6, 0);
        for (const tag of [7, 8, 9, 10]) packet.setText(tag, '');
      }, [923], 923);
      assert.equal(page.int(1), 200); assert.match(page.text(2) ?? '', /alice/i);
      const image = await client.getEmoticon('[smile]');
      assert.ok(image.image && image.image.length > 8);
    }
    const image = await alice.getEmoticon('[smile]');
    const photo = new Promise<ChatMessage>((resolve, reject) => {
      const timer = setTimeout(() => { unsubscribe(); reject(new Error('Missing encrypted photo')); }, 10000);
      const unsubscribe = bob.subscribe((event) => {
        if (event.type === 'message' && event.message.content === 5) {
          clearTimeout(timer); unsubscribe(); resolve(event.message);
        }
      });
    });
    void photo.catch(() => {});
    await alice.sendPhoto(1, 'bob', image.image!);
    assert.equal((await photo).sender, 'alice');
    await Promise.all(clients.map((client) => client.logout()));
  } finally {
    for (const client of clients) client.close();
    await server.close();
  }
});
