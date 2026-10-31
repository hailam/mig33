import assert from 'node:assert/strict';
import { setImmediate as tick } from 'node:timers/promises';
import test from 'node:test';
import { Mig33Client, type ClientEvent } from '../src/mig33/client.ts';
import { Packet } from '../src/mig33/protocol/index.ts';
import type { PacketTransport, TransportEvents } from '../src/mig33/transport/types.ts';

class RoomTransport implements PacketTransport {
  readonly kind = 'http' as const;
  events?: TransportEvents;
  sent: Packet[] = [];
  async connect(events: TransportEvents): Promise<void> { this.events = events; }
  async send(packet: Packet): Promise<void> {
    this.sent.push(packet);
    if (packet.opcode === 200) this.receive(new Packet(201, packet.transaction).setText(1, 'token').setText(2, 'sid'));
    else if (packet.opcode === 202) this.receive(new Packet(203, packet.transaction));
  }
  setSession(): void {}
  activate(): void {}
  close(): void {}
  receive(...packets: Packet[]): void { this.events!.packets(packets); }
}

async function setup() {
  const transport = new RoomTransport();
  const client = new Mig33Client(transport, { requestTimeoutMs: 2000 });
  await client.login({ username: 'alice', password: 'alice-demo' });
  return { client, transport };
}
function entry(transaction: number, name: string, category = 1): Packet {
  return new Packet(701, transaction).setText(1, name).setInt(3, 50).setInt(4, 2).setShort(6, category);
}

test('room category and listing snapshots retain server order and request context across interleaved replies', async () => {
  const { client, transport } = await setup();
  try {
    const categories = client.listRoomCategories();
    await tick();
    const request = transport.sent.at(-1)!;
    assert.equal(request.opcode, 713);
    transport.receive(new Packet(714, request.transaction).setShort(1, 8).setText(2, 'Regional').setByte(3, 1).setByte(4, 0).setByte(5, 0),
      new Packet(714, request.transaction).setShort(1, 2).setText(2, 'Favorites').setByte(3, 1).setByte(4, 1).setByte(5, 1),
      new Packet(715, request.transaction));
    assert.deepEqual((await categories).map(({ id, name, favorites, flags }) => ({ id, name, favorites, flags })), [
      { id: 8, name: 'Regional', favorites: false, flags: [1, 0, 0] },
      { id: 2, name: 'Favorites', favorites: true, flags: [1, 1, 1] },
    ]);
    const first = client.listRooms(8);
    const second = client.listRooms(2);
    await tick();
    const [a, b] = transport.sent.slice(-2);
    assert.deepEqual([a.short(1), a.byte(2), b.short(1), b.byte(2)], [8, 1, 2, 1]);
    transport.receive(entry(a.transaction, 'Maldives', 8), entry(b.transaction, 'Lobby', 2),
      new Packet(717, b.transaction).setText(1, 'Refresh favorites'),
      entry(a.transaction, 'Gaming', 8), new Packet(717, a.transaction).setText(1, 'Refresh region'));
    assert.deepEqual((await first).rooms.map((room) => room.name), ['Maldives', 'Gaming']);
    assert.deepEqual({ ...(await second), rooms: (await second).rooms.map((room) => room.name) },
      { categoryId: 2, rooms: ['Lobby'], footer: 'Refresh favorites' });
  } finally { client.close(); }
});

test('only correlated join701 initializes a room before ACK and same-batch welcome', async () => {
  const { client, transport } = await setup();
  const events: ClientEvent[] = [];
  client.subscribe((event) => events.push(event));
  try {
    const joining = client.joinRoom(' lobby ');
    const listing = client.listRooms(1);
    await tick();
    const [join, list] = transport.sent.slice(-2);
    assert.equal(join.text(1), 'lobby');
    transport.receive(entry(0, 'Unsolicited'), entry(list.transaction, 'Gaming'), entry(join.transaction, 'Lobby').setInt(8, 37),
      new Packet(1, join.transaction), new Packet(500, 0).setText(2, 'Lobby').setByte(3, 3)
        .setText(4, 'Lobby').setShort(6, 1).setText(8, 'Welcome to Lobby.'),
      new Packet(717, list.transaction).setText(1, 'Refresh'));
    const room = await joining;
    assert.equal(room.name, 'Lobby');
    assert.equal(room.communityId, 37);
    assert.equal((await listing).rooms[0].name, 'Gaming');
    assert.deepEqual(events.filter((event) => event.type === 'room-joining' || event.type === 'message').map((event) => event.type),
      ['room-joining', 'message']);
    assert.equal(events.filter((event) => event.type === 'room-joining').length, 1);
  } finally { client.close(); }
});

test('participants retain order and correlate to their own707 even when replies arrive out of order', async () => {
  const { client, transport } = await setup();
  try {
    const lobby = client.getRoomParticipants('Lobby');
    const gaming = client.getRoomParticipants('Gaming');
    await tick();
    const [a, b] = transport.sent.slice(-2);
    transport.receive(new Packet(708, 0).setText(2, 'wrong;'),
      new Packet(708, b.transaction).setText(2, 'carol;bob;').setText(3, 'BOB;').setText(4, 'carol;'),
      new Packet(708, a.transaction).setText(2, 'alice;bob;').setText(3, 'alice;').setByte(5, 1));
    assert.deepEqual((await gaming).participants, [
      { username: 'carol', moderator: false, muted: true }, { username: 'bob', moderator: true, muted: false },
    ]);
    assert.equal((await gaming).room, 'Gaming');
    assert.equal((await gaming).service, 1);
    assert.equal((await lobby).room, 'Lobby');
    assert.deepEqual((await lobby).participants.map((user) => user.username), ['alice', 'bob']);
  } finally { client.close(); }
});

test('group755 participants use their originating754 identity and discard unterminated list tails', async () => {
  const { client, transport } = await setup();
  try {
    const first = client.getGroupParticipants('group-one');
    const second = client.getGroupParticipants('group-two');
    await tick();
    const [a, b] = transport.sent.slice(-2);
    assert.deepEqual([a.opcode, a.text(1), a.byte(2), b.text(1)], [754, 'group-one', 1, 'group-two']);
    transport.receive(new Packet(755, 0).setText(2, 'unsolicited;'),
      new Packet(755, b.transaction).setText(2, 'carol;bob;ignored-tail').setText(3, 'bob;').setText(4, 'carol;'),
      new Packet(755, a.transaction).setText(2, 'alice;bob;'));
    assert.equal((await first).room, 'group-one');
    assert.equal((await second).room, 'group-two');
    assert.deepEqual((await second).participants, [
      { username: 'carol', moderator: false, muted: true }, { username: 'bob', moderator: true, muted: false },
    ]);
  } finally { client.close(); }
});

test('favorite additions accept701 orACK without joining; removal retains original tag layout', async () => {
  const { client, transport } = await setup();
  const joined: ClientEvent[] = [];
  client.subscribe((event) => { if (event.type === 'room-joining') joined.push(event); });
  try {
    const add = client.addRoomFavorite('Lobby');
    await tick();
    transport.receive(entry(transport.sent.at(-1)!.transaction, 'Lobby', 2));
    await add;
    const repeat = client.addRoomFavorite('Lobby');
    await tick();
    transport.receive(new Packet(1, transport.sent.at(-1)!.transaction));
    await repeat;
    const remove = client.removeRoomFavorite('Lobby', 2);
    await tick();
    const request = transport.sent.at(-1)!;
    assert.deepEqual([request.opcode, request.short(1), request.text(2)], [712, 2, 'Lobby']);
    transport.receive(new Packet(1, request.transaction));
    await remove;
    assert.equal(joined.length, 0);
  } finally { client.close(); }
});

test('room sends use500 kind3 with unchanged spelling while private sends keep kind1', async () => {
  const { client, transport } = await setup();
  try {
    const room = client.sendRoomMessage('Island Chat', 'Hello');
    const privateMessage = client.sendMessage('ISLAND', 'Private');
    await tick();
    const [a, b] = transport.sent.slice(-2);
    assert.deepEqual([a.opcode, a.byte(1), a.text(2), a.byte(3), a.text(4), a.short(6), a.text(8)],
      [500, 1, 'alice', 3, 'Island Chat', 1, 'Hello']);
    assert.deepEqual([b.byte(3), b.text(4)], [1, 'island']);
    transport.receive(new Packet(1, a.transaction), new Packet(1, b.transaction));
    await Promise.all([room, privateMessage]);
  } finally { client.close(); }
});
