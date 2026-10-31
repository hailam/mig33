import assert from 'node:assert/strict';
import { setImmediate as tick } from 'node:timers/promises';
import test from 'node:test';
import { Mig33Client } from '../src/mig33/client.ts';
import { Packet } from '../src/mig33/protocol/index.ts';
import type { PacketTransport, TransportEvents } from '../src/mig33/transport/types.ts';
import { SessionController } from '../src/ui/session/controller.ts';

class RoomTransport implements PacketTransport {
  readonly kind = 'http' as const;
  events?: TransportEvents;
  sent: Packet[] = [];
  handler?: (packet: Packet) => void;
  async connect(events: TransportEvents): Promise<void> { this.events = events; }
  async send(packet: Packet): Promise<void> {
    this.sent.push(packet);
    if (packet.opcode === 200) this.receive(new Packet(201, packet.transaction).setText(1, 'token').setText(2, 'sid'));
    else if (packet.opcode === 202) this.receive(new Packet(203, packet.transaction));
    else if (packet.opcode === 400) this.receive(new Packet(403, packet.transaction));
    else this.handler?.(packet);
  }
  setSession(): void {}
  activate(): void {}
  close(): void {}
  receive(...packets: Packet[]): void { this.events!.packets(packets); }
}
const credentials = { username: 'alice', password: 'alice-demo' };
const connection = { mode: 'http', httpURL: 'http://localhost:8080' } as const;
function setup() {
  const transports: RoomTransport[] = [];
  const controller = new SessionController(() => {
    const transport = new RoomTransport();
    transports.push(transport);
    return new Mig33Client(transport, { requestTimeoutMs: 2000 });
  }, 'test');
  return { controller, transports };
}
function incoming(room: string, sender: string, body: string, content = 1): Packet {
  return new Packet(500, 0).setByte(1, 1).setText(2, sender).setByte(3, 3).setText(4, room).setShort(6, content).setText(8, body);
}
function joinReply(transport: RoomTransport, name: string, transaction = transport.sent.at(-1)!.transaction): void {
  transport.receive(new Packet(701, transaction).setText(1, name).setInt(3, 50).setInt(4, 1).setShort(6, 1),
    new Packet(1, transaction), incoming(name, name, `Welcome to ${name}.`));
}
async function join(controller: SessionController, transport: RoomTransport, name = 'Lobby'): Promise<void> {
  const joining = controller.joinRoom(name);
  await tick();
  joinReply(transport, name);
  assert.equal(await joining, true);
}

test('joined community metadata survives early701 and is refreshed or cleared by rejoin', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    const joining = controller.joinRoom('Club'); await tick();
    let request = transport.sent.at(-1)!;
    transport.receive(new Packet(701, request.transaction).setText(1, 'Club').setInt(8, 37));
    assert.equal(controller.getSnapshot().roomConversations[0].communityId, 37);
    transport.receive(new Packet(1, request.transaction)); assert.equal(await joining, true);
    controller.setRoomDraft('Club', 'Keep my draft');
    const rejoining = controller.rejoinRoom('Club'); await tick();
    request = transport.sent.at(-1)!;
    transport.receive(new Packet(701, request.transaction).setText(1, 'Club'), new Packet(1, request.transaction));
    assert.equal(await rejoining, true);
    const room = controller.getSnapshot().roomConversations[0];
    assert.equal(room.communityId, undefined, 'unlinked room must lose its Group Menu');
    assert.equal(room.draft, 'Keep my draft');
  } finally { controller.dispose(); }
});

test('join preserves same-batch welcome and never steals navigation; room and private identities cannot collide', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    const joining = controller.joinRoom('lobby');
    assert.equal(await controller.joinRoom('lobby'), false, 'double join cannot submit another request');
    controller.openConversation('Lobby');
    controller.setDraft('lobby', 'private draft');
    await tick();
    joinReply(transport, 'Lobby');
    assert.equal(await joining, true);
    let state = controller.getSnapshot();
    assert.equal(state.activePeer, 'lobby');
    assert.equal(state.activeRoom, null);
    assert.equal(state.roomConversations[0].name, 'Lobby');
    assert.deepEqual(state.roomConversations[0].rows.map((row) => row.body), ['Welcome to Lobby.']);
    assert.equal(state.roomConversations[0].unread, 1);
    controller.setRoomDraft('LOBBY', 'room draft');
    controller.openRoom('lobby');
    state = controller.getSnapshot();
    assert.equal(state.activePeer, null);
    assert.equal(state.activeRoom, 'Lobby');
    assert.equal(state.roomConversations[0].unread, 0);
    assert.equal(state.conversations[0].draft, 'private draft');
    assert.equal(state.roomConversations[0].draft, 'room draft');
    controller.selectConversation(null);
    transport.receive(incoming('Lobby', 'bob', 'hello'), incoming('Unjoined', 'carol', 'hidden'));
    assert.equal(controller.getSnapshot().roomConversations.length, 1);
    assert.equal(controller.getSnapshot().roomConversations[0].unread, 1);
    transport.receive(incoming('Lobby', '', 'Room notice without sender', 6));
    assert.equal(controller.getSnapshot().roomConversations[0].rows.at(-1)!.body, 'Room notice without sender');
  } finally { controller.dispose(); }
});

test('pages preserve room membership and draft while counting unread until the room becomes visible', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    await join(controller, transport);
    await join(controller, transport, 'Gaming');
    controller.openRoom('LOBBY');
    controller.setRoomDraft('Lobby', 'Room draft');
    controller.setConversationVisible(false);
    transport.receive(incoming('Lobby', 'bob', 'Behind room info'), incoming('Gaming', 'carol', 'Elsewhere'),
      incoming('Lobby', 'alice', 'Own room action', 6));
    let state = controller.getSnapshot();
    assert.equal(state.activeRoom, 'Lobby');
    assert.equal(state.roomConversations[0].state, 'joined');
    assert.equal(state.roomConversations[0].draft, 'Room draft');
    assert.deepEqual(state.roomConversations.map((room) => room.unread), [1, 2]);
    controller.setConversationVisible(true);
    transport.receive(incoming('Lobby', 'bob', 'While writing'));
    state = controller.getSnapshot();
    assert.deepEqual(state.roomConversations.map((room) => room.unread), [0, 2]);
    assert.equal(state.roomConversations[0].draft, 'Room draft');
    controller.setConversationVisible(false);
    controller.openRoom('lobby');
    transport.receive(incoming('Lobby', 'bob', 'Immediately after selecting'));
    assert.equal(controller.getSnapshot().roomConversations[0].unread, 0);
    assert.equal(controller.getSnapshot().conversationVisible, true);
  } finally { controller.dispose(); }
});

test('leaving suppresses room traffic while pending, restores draft on rejection, and removes membership only on ACK', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    await join(controller, transport);
    controller.openRoom('Lobby');
    controller.setRoomDraft('Lobby', 'keep this draft');
    const rejected = controller.leaveRoom('Lobby');
    await tick();
    transport.receive(incoming('Lobby', 'bob', 'suppressed'), new Packet(0, transport.sent.at(-1)!.transaction).setText(2, 'Try again'));
    assert.equal(await rejected, false);
    let room = controller.getSnapshot().roomConversations[0];
    assert.equal(room.state, 'joined');
    assert.equal(room.draft, 'keep this draft');
    assert.equal(room.rows.length, 1);
    transport.receive(incoming('Lobby', 'bob', 'restored'));
    const leaving = controller.leaveRoom('Lobby');
    await tick();
    transport.receive(new Packet(1, transport.sent.at(-1)!.transaction), incoming('Lobby', 'bob', 'after ACK in same batch'));
    assert.equal(await leaving, true);
    transport.receive(incoming('Lobby', 'bob', 'late'));
    assert.equal(controller.getSnapshot().roomConversations.length, 0);
    assert.equal(controller.getSnapshot().activeRoom, null);
  } finally { controller.dispose(); }
});

test('room send trims once, preserves later edits, blocks duplicates and adds no literal slash echo', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    await join(controller, transport);
    controller.openRoom('Lobby');
    controller.setRoomDraft('Lobby', '  First message\n');
    const sending = controller.sendRoom();
    assert.equal(await controller.sendRoom(), false);
    await tick();
    const request = transport.sent.at(-1)!;
    assert.deepEqual([request.byte(3), request.text(4), request.text(8)], [3, 'Lobby', 'First message']);
    controller.setRoomDraft('Lobby', 'Later edit');
    transport.receive(new Packet(1, request.transaction));
    assert.equal(await sending, true);
    let room = controller.getSnapshot().roomConversations[0];
    assert.equal(room.draft, 'Later edit');
    assert.deepEqual(room.rows.map((row) => row.body), ['Welcome to Lobby.', 'First message']);
    controller.setRoomDraft('Lobby', ' /me waves ');
    const action = controller.sendRoom();
    await tick();
    transport.receive(incoming('Lobby', 'alice', 'alice waves', 6), new Packet(1, transport.sent.at(-1)!.transaction));
    assert.equal(await action, true);
    room = controller.getSnapshot().roomConversations[0];
    assert.equal(room.rows.filter((row) => row.body === 'alice waves').length, 1);
    assert.equal(room.rows.at(-1)!.action, true);
    assert.equal(room.draft, '');
  } finally { controller.dispose(); }
});

test('failed room send retains draft and generic kick notices do not infer membership from English', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    await join(controller, transport);
    controller.setRoomDraft('Lobby', 'message');
    const sending = controller.sendRoom('Lobby');
    await tick();
    transport.receive(new Packet(5, 0).setByte(1, 0).setText(2, 'You were kicked from Lobby by bob.').setByte(3, 1).setText(4, 'Chat room'),
      new Packet(0, transport.sent.at(-1)!.transaction).setText(2, 'Join the chat room before sending messages'));
    assert.equal(await sending, false);
    assert.equal(controller.getSnapshot().roomConversations[0].draft, 'message');
    assert.equal(controller.getSnapshot().roomConversations[0].state, 'joined', 'original popup does not remove the tab');
    assert.match(controller.getSnapshot().error, /Join the chat room/);
  } finally { controller.dispose(); }
});

test('disconnect and new login never replay a join or let stale room requests mutate the new session', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    await join(controller, transport);
    const listing = controller.loadRoomCategories();
    await tick();
    const stale = transport.sent.at(-1)!;
    controller.suspend();
    assert.equal(controller.getSnapshot().roomConversations.length, 0, 'closed transport has no joined memberships');
    assert.equal(await listing, false);
    await controller.login(credentials, connection);
    transport.receive(new Packet(714, stale.transaction).setShort(1, 1).setText(2, 'Stale'), new Packet(715, stale.transaction),
      incoming('Lobby', 'bob', 'old connection'));
    const state = controller.getSnapshot();
    assert.equal(state.roomConversations.length, 0);
    assert.equal(state.roomCategories.length, 0);
    assert.deepEqual(transports[1].sent.map((packet) => packet.opcode), [200, 202, 400]);
  } finally { controller.dispose(); }
});

test('canonical join names replace the pending identity and remain selectable by the requested name', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    const joining = controller.joinRoom('Old Room');
    await tick();
    joinReply(transport, 'New Room');
    assert.equal(await joining, true);
    assert.deepEqual(controller.getSnapshot().roomConversations.map((room) => [room.name, room.state]), [['New Room', 'joined']]);
    controller.openRoom('Old Room');
    assert.equal(controller.getSnapshot().activeRoom, 'New Room');
    controller.setRoomDraft('Old Room', 'canonical draft');
    assert.equal(controller.getSnapshot().roomConversations[0].draft, 'canonical draft');
  } finally { controller.dispose(); }
});

test('category footer appends or replaces according to714 flag3, and global refresh restarts loaded lists and Favorites', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    let refresh = 0;
    transport.handler = (packet) => {
      if (packet.opcode === 713) {
        refresh++;
        transport.receive(new Packet(714, packet.transaction).setShort(1, 1).setText(2, 'Rooms').setByte(3, 0).setByte(5, 0),
          new Packet(714, packet.transaction).setShort(1, 2).setText(2, 'Favorites').setByte(3, 1).setByte(5, 1),
          new Packet(715, packet.transaction));
      } else if (packet.opcode === 716) {
        const name = packet.short(1) === 2 ? 'Favorite' : packet.byte(2) === 1 ? 'Lobby' : 'Gaming';
        transport.receive(new Packet(701, packet.transaction).setText(1, name).setInt(4, refresh),
          new Packet(717, packet.transaction).setText(1, packet.short(1) === 2 ? 'Refresh' : 'More'));
      }
    };
    assert.equal(await controller.loadRoomCategories(), true);
    assert.equal(controller.getSnapshot().roomCategories[1].loaded, true, 'Favorites are known before menu labels are chosen');
    assert.equal(await controller.loadRoomCategory(1), true);
    assert.equal(await controller.loadRoomCategory(1), true);
    assert.deepEqual(controller.getSnapshot().roomCategories[0].rooms.map((room) => room.name), ['Lobby', 'Gaming']);
    assert.equal(transport.sent.at(-1)!.byte(2), 0, 'footer requests use original continuation flag');
    assert.equal(await controller.loadRoomCategory(2), true);
    assert.equal(controller.getSnapshot().roomCategories[1].rooms.length, 1, 'flag3 true replaces instead of duplicating');
    assert.equal(await controller.loadRoomCategories(), true);
    const category = controller.getSnapshot().roomCategories[0];
    assert.deepEqual(category.rooms.map((room) => [room.name, room.occupancy]), [['Lobby', 2]]);
    assert.ok(transport.sent.slice(-2).every((packet) => packet.opcode === 716 && packet.byte(2) === 1));
  } finally { controller.dispose(); }
});

test('a fresh favorite snapshot wins over an older in-flight category request', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    transport.handler = (packet) => {
      if (packet.opcode === 713) transport.receive(new Packet(714, packet.transaction).setShort(1, 2).setText(2, 'Favorites').setByte(3, 1).setByte(5, 1),
        new Packet(715, packet.transaction));
      else if (packet.opcode === 716) transport.receive(new Packet(717, packet.transaction).setText(1, 'Refresh'));
    };
    await controller.loadRoomCategories();
    transport.handler = undefined;
    const older = controller.loadRoomCategory(2);
    await tick();
    const oldRequest = transport.sent.at(-1)!;
    const adding = controller.setRoomFavorite('New', true);
    await tick();
    transport.receive(new Packet(701, transport.sent.at(-1)!.transaction).setText(1, 'New').setShort(6, 2));
    await tick();
    const freshRequest = transport.sent.at(-1)!;
    assert.equal(freshRequest.byte(2), 1);
    transport.receive(new Packet(701, freshRequest.transaction).setText(1, 'New').setShort(6, 2),
      new Packet(717, freshRequest.transaction).setText(1, 'Refresh'));
    assert.equal(await adding, true);
    transport.receive(new Packet(701, oldRequest.transaction).setText(1, 'Old').setShort(6, 2),
      new Packet(717, oldRequest.transaction).setText(1, 'Refresh'));
    assert.equal(await older, false);
    assert.deepEqual(controller.getSnapshot().roomCategories[0].rooms.map((room) => room.name), ['New']);
    assert.equal(controller.getSnapshot().roomCategories[0].loading, false);
  } finally { controller.dispose(); }
});

test('favorites refresh only the category snapshot and do not join or leave the room', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    transport.handler = (packet) => {
      if (packet.opcode === 713) transport.receive(new Packet(714, packet.transaction).setShort(1, 2).setText(2, 'Favorites').setByte(5, 1),
        new Packet(715, packet.transaction));
      else if (packet.opcode === 711) transport.receive(new Packet(701, packet.transaction).setText(1, 'Lobby').setShort(6, 2));
      else if (packet.opcode === 716) transport.receive(new Packet(701, packet.transaction).setText(1, 'Lobby').setInt(3, 50).setInt(4, 0).setShort(6, 2),
        new Packet(717, packet.transaction).setText(1, 'Refresh'));
    };
    assert.equal(await controller.loadRoomCategories(), true);
    assert.equal(await controller.setRoomFavorite('Lobby', true), true);
    assert.deepEqual(controller.getSnapshot().roomCategories[0].rooms.map((room) => room.name), ['Lobby']);
    assert.equal(controller.getSnapshot().roomCategories[0].loaded, true);
    assert.equal(controller.getSnapshot().roomConversations.length, 0);
    assert.equal(controller.getSnapshot().roomAction, null);
  } finally { controller.dispose(); }
});
