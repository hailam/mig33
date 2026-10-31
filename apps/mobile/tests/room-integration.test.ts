import assert from 'node:assert/strict';
import test from 'node:test';
import { Mig33Client, type ChatMessage } from '../src/mig33/client.ts';
import { HTTPTransport } from '../src/mig33/transport/http.ts';
import { TCPTransport } from '../src/mig33/transport/tcp.ts';
import { nodeConnector } from './helpers/node-socket.ts';
import { startServer, type TestServer } from './helpers/server.ts';

function client(server: TestServer, kind: 'http' | 'tcp'): Mig33Client {
  return new Mig33Client(kind === 'http' ? new HTTPTransport(server.httpURL, { pollIntervalMs: 100 })
    : new TCPTransport(nodeConnector('127.0.0.1', server.tcpPort)));
}
function expectRoomMessage(source: Mig33Client, room: string, body: string): Promise<ChatMessage> {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => { unsubscribe(); reject(new Error(`Room message not delivered: ${body}`)); }, 15000);
    const unsubscribe = source.subscribe((event) => {
      if (event.type !== 'message' || event.message.kind !== 3 || event.message.target !== room || event.message.body !== body) return;
      clearTimeout(timeout); unsubscribe(); resolve(event.message);
    });
  });
}

for (const kind of ['http', 'tcp'] as const) {
  test(`room discovery, favorites, participants and live messages over ${kind} interoperate with the other original transport`,
    { skip: !process.env.MIG33_TEST_SERVER_BINARY, timeout: 45000 }, async (t) => {
      const server = await startServer();
      t.after(() => server.close());
      const alice = client(server, kind);
      const bob = client(server, kind === 'http' ? 'tcp' : 'http');
      t.after(() => { alice.close(); bob.close(); });
      await alice.login({ username: 'alice', password: 'alice-demo' });
      await bob.login({ username: 'bob', password: 'bob-demo' });
      const categories = await alice.listRoomCategories();
      const normal = categories.find((category) => !category.favorites)!;
      const favorites = categories.find((category) => category.favorites)!;
      assert.ok(normal && favorites);
      const initial = await alice.listRooms(normal.id);
      assert.ok(initial.rooms.some((room) => room.name === 'Lobby' && room.capacity === 50));
      assert.equal(initial.footer, 'Refresh');
      await alice.addRoomFavorite('Lobby');
      assert.deepEqual((await alice.listRooms(favorites.id)).rooms.map((room) => room.name), ['Lobby']);
      await alice.addRoomFavorite('Lobby'); // Loaded duplicate returns ACK instead of701.
      await alice.removeRoomFavorite('Lobby', favorites.id);
      assert.equal((await alice.listRooms(favorites.id)).rooms.length, 0);

      // Verify actual welcome ordering without assuming the configured description.
      const events: string[] = [];
      let receivedWelcome!: () => void;
      const welcome = new Promise<void>((resolve) => { receivedWelcome = resolve; });
      const stop = alice.subscribe((event) => {
        if (event.type === 'room-joining') events.push('joined');
        if (event.type === 'message' && event.message.kind === 3 && event.message.target === 'Lobby') {
          events.push('message'); receivedWelcome();
        }
      });
      assert.equal((await alice.joinRoom('lobby')).name, 'Lobby');
      // TCP may deliver ACK and welcome in separate reads; their wire order is
      //the contract, not whether both share one JavaScript callback.
      await welcome;
      stop();
      assert.deepEqual(events, ['joined', 'message']);
      await bob.joinRoom('Lobby');
      const participants = await alice.getRoomParticipants('Lobby');
      assert.deepEqual(participants.participants.map((user) => user.username), ['alice', 'bob']);

      const receivedByBob = expectRoomMessage(bob, 'Lobby', 'Room hello ✓');
      await alice.sendRoomMessage('Lobby', 'Room hello ✓');
      assert.equal((await receivedByBob).sender, 'alice');
      const receivedByAlice = expectRoomMessage(alice, 'Lobby', 'Room reply 😀');
      await bob.sendRoomMessage('Lobby', 'Room reply 😀');
      assert.equal((await receivedByAlice).sender, 'bob');
      const action = expectRoomMessage(alice, 'Lobby', 'alice waves');
      await alice.sendRoomMessage('Lobby', '/me waves');
      assert.equal((await action).content, 6);
      await alice.leaveRoom('Lobby');
      await assert.rejects(alice.sendRoomMessage('Lobby', 'not joined'), /Join the chat room/);
      await assert.rejects(alice.getRoomParticipants('Lobby'), /Join the chat room/);
      assert.deepEqual((await bob.getRoomParticipants('Lobby')).participants.map((user) => user.username), ['bob']);
      await bob.leaveRoom('Lobby');
      await alice.logout();
      await bob.logout();
      t.diagnostic(`Room transport run: ${server.outputDirectory}`);
    });
}
