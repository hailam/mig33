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
function nextMessage(source: Mig33Client, matches: (message: ChatMessage) => boolean): Promise<ChatMessage> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => { stop(); reject(new Error(`Expected original-protocol chat message was not delivered: ${matches}`)); }, 15000);
    const stop = source.subscribe((event) => {
      if (event.type === 'message' && matches(event.message)) { clearTimeout(timer); stop(); resolve(event.message); }
    });
  });
}
async function page(source: Mig33Client, url: string, body?: string): Promise<string> {
  const [reply] = await source.requestOriginal(922, (packet) => {
    packet.setByte(1, body === undefined ? 1 : 2).setText(2, url).setInt(4, 1).setInt(6, 0);
    if (body !== undefined) packet.setText(3, body);
    for (const tag of [7, 8, 9, 10]) packet.setText(tag, '');
  }, [923], 923);
  assert.equal(reply.int(1), 200, reply.text(2) ?? 'Missing legacy page body');
  return reply.text(2) ?? '';
}
function hidden(html: string, name: string): string {
  for (const input of html.matchAll(/<input\b[^>]*>/g)) {
    const attributes = new Map([...input[0].matchAll(/([a-z]+)="([^"]*)"/g)].map((match) => [match[1], match[2]]));
    if (attributes.get('name') === name) return attributes.get('value') ?? '';
  }
  throw new Error(`Missing original room form field ${name}`);
}
async function createRoom(source: Mig33Client, server: TestServer, name: string): Promise<void> {
  const url = server.httpURL + '/sites/index.php?c=chatroom&a=create&v=midlet';
  const initial = await page(source, url);
  const csrf = hidden(initial, 'csrf');
  const details = await page(source, url + '&stage=name', `csrf=${csrf}&name=${name}`);
  const created = await page(source, url + '&stage=details',
    `csrf=${csrf}&room=${hidden(details, 'room')}&binding=${hidden(details, 'binding')}&description=Integration%20room`);
  assert.ok(created.includes(`mig33:joinChatroom(${name})`));
}

for (const kind of ['http', 'tcp'] as const) {
  test(`expanded chat actions preserve original permissions, groups, photos and blocking through ${kind}`,
    { skip: !process.env.MIG33_TEST_SERVER_BINARY, timeout: 60000 }, async (t) => {
      const server = await startServer(); t.after(() => server.close());
      const alice = client(server, kind), bob = client(server, kind === 'http' ? 'tcp' : 'http');
      const carol = client(server, 'tcp'), dave = client(server, 'http'), hailam = client(server, 'tcp');
      const clients = [alice, bob, carol, dave, hailam]; t.after(() => clients.forEach((source) => source.close()));
      await Promise.all(clients.map((source, index) => source.login({ username: ['alice', 'bob', 'carol', 'dave', 'hailam'][index],
        password: ['alice-demo', 'bob-demo', 'carol-demo', 'dave-demo', 'hailam'][index] })));
      await Promise.all(clients.map((source) => source.syncContacts()));

      const room = 'Actions' + kind;
      await createRoom(alice, server, room);
      await Promise.all([alice, bob, carol].map((source) => source.joinRoom(room)));
      assert.equal((await alice.getRoomParticipants(room)).participants.find((user) => user.username === 'alice')?.moderator, true);
      await assert.rejects(bob.moderateRoom(room, 'carol', 'mute'), /Only room moderators/);
      await assert.rejects(bob.moderateRoom(room, 'alice', 'kick'), /another room moderator/);
      assert.match((await alice.moderateRoom(room, 'bob', 'mute'))!, /muted/);
      assert.equal((await alice.getRoomParticipants(room)).participants.find((user) => user.username === 'bob')?.muted, true);
      await assert.rejects(bob.sendRoomMessage(room, 'Muted message'), /muted/);
      await alice.moderateRoom(room, 'bob', 'unmute');
      const restored = nextMessage(alice, (message) => message.target === room && message.body === 'Restored');
      await bob.sendRoomMessage(room, 'Restored'); await restored;
      await alice.moderateRoom(room, 'bob', 'kick');
      await assert.rejects(bob.getRoomParticipants(room), /Join the chat room/);
      await bob.joinRoom(room); // Explicit rejoin remains a real703 after generic kick popup.

      // Ordinary participants use the same706 to vote; no synthetic vote API.
      await Promise.all(clients.map((source) => source.joinRoom('Lobby')));
      await carol.moderateRoom('Lobby', 'bob', 'kick');
      await dave.moderateRoom('Lobby', 'bob', 'kick');
      assert.ok((await bob.getRoomParticipants('Lobby')).participants.some((user) => user.username === 'bob'));
      await hailam.moderateRoom('Lobby', 'bob', 'kick');
      await assert.rejects(bob.getRoomParticipants('Lobby'), /Join the chat room/);

      const bobCreated = nextMessage(bob, (message) => message.kind === 2);
      const carolCreated = nextMessage(carol, (message) => message.kind === 2);
      const id = await alice.createGroup('bob', 'carol');
      assert.equal((await bobCreated).target, id); assert.equal((await carolCreated).target, id);
      assert.deepEqual((await alice.getGroupParticipants(id)).participants.map((user) => user.username), ['alice', 'bob', 'carol']);
      const daveInvited = nextMessage(dave, (message) => message.kind === 2 && message.target === id);
      await alice.inviteToGroup(id, 'dave'); await daveInvited;
      assert.equal((await alice.getGroupParticipants(id)).participants.length, 4);
      const colored = nextMessage(bob, (message) => message.target === id && message.body === 'Colored [smile]');
      await alice.sendGroupMessage(id, 'Colored [smile]', 255);
      const colorMessage = await colored;
      assert.equal(colorMessage.packet.int(13), 255); assert.ok(colorMessage.packet.text(11)?.includes('[smile]'));
      const image = await alice.getEmoticon('[smile]'); assert.ok(image.image && image.imageURI);
      const photo = nextMessage(bob, (message) => message.kind === 1 && message.content === 5);
      await alice.sendPhoto(1, 'bob', image.image);
      const photoMessage = await photo;
      const token = photoMessage.packet.text(11)!.split(' ').find((value) => value.startsWith('[photo:'))!;
      assert.ok((await bob.getEmoticon(token)).imageURI);
      assert.match((await alice.uploadPhoto(image.image))!, /c=photo/);
      await alice.addContact('dave');
      assert.ok(alice.contacts.some((contact) => contact.username === 'dave'));
      const daveContact = alice.contacts.find((contact) => contact.username === 'dave')!;
      await alice.createContactGroup('Integration friends');
      const contactGroup = alice.groups.find((group) => group.name === 'Integration friends')!;
      assert.ok(contactGroup);
      await alice.renameContactGroup(contactGroup.id, 'Renamed friends');
      await alice.moveContact(daveContact.id, contactGroup.id);
      await assert.rejects(alice.deleteContactGroup(contactGroup.id), /not empty|contains contacts/i);
      await alice.updateContact(daveContact.id, { display: 'David', mobile: '1234' });
      await alice.syncContacts();
      assert.equal(alice.contacts.find((contact) => contact.id === daveContact.id)?.display, 'David');
      assert.equal(alice.contacts.find((contact) => contact.id === daveContact.id)?.groupId, contactGroup.id);
      assert.equal(alice.groups.find((group) => group.id === contactGroup.id)?.name, 'Renamed friends');
      await alice.moveContact(daveContact.id, -1);
      await alice.deleteContactGroup(contactGroup.id);
      await alice.deleteContact(daveContact.id);
      await alice.syncContacts();
      assert.equal(alice.contacts.some((contact) => contact.id === daveContact.id), false);
      assert.equal(alice.groups.some((group) => group.id === contactGroup.id), false);
      await alice.setPresence(3, 1);
      await bob.syncContacts();
      assert.equal(bob.contacts.find((contact) => contact.username === 'alice')?.presence, 3);
      await alice.setStatus('Integration status', '');
      await bob.syncContacts();
      assert.equal(bob.contacts.find((contact) => contact.username === 'alice')?.status, 'Integration status');
      await alice.setPresence(1, 3);

      const seen: string[] = [];
      const stop = alice.subscribe((event) => { if (event.type === 'message') seen.push(event.message.body); });
      await alice.blockUser('bob');
      const otherViewer = nextMessage(carol, (message) => message.target === id && message.body === 'Blocked only for Alice');
      await bob.sendGroupMessage(id, 'Blocked only for Alice'); await otherViewer;
      const barrier = nextMessage(alice, (message) => message.body === 'After blocked send');
      await carol.sendMessage('alice', 'After blocked send'); await barrier;
      stop();
      assert.ok(!seen.includes('Blocked only for Alice'));
      assert.ok((await page(alice, server.httpURL + '/sites/index.php?c=settings&a=block_list&v=midlet')).includes('bob'));
      await dave.leaveGroup(id);
      await assert.rejects(dave.getGroupParticipants(id), /not a member/);
      assert.equal((await alice.getGroupParticipants(id)).participants.length, 3);
      t.diagnostic(`Expanded chat actions: ${server.outputDirectory}`);
    });
}
