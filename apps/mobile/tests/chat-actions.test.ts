import assert from 'node:assert/strict';
import { setImmediate as tick } from 'node:timers/promises';
import test from 'node:test';
import { Mig33Client } from '../src/mig33/client.ts';
import { Packet } from '../src/mig33/protocol/index.ts';
import type { PacketTransport, TransportEvents } from '../src/mig33/transport/types.ts';
import { SessionController } from '../src/ui/session/controller.ts';

class Transport implements PacketTransport {
  readonly kind = 'http' as const;
  events?: TransportEvents;
  sent: Packet[] = [];
  handler?: (packet: Packet) => void;
  async connect(events: TransportEvents): Promise<void> { this.events = events; }
  async send(packet: Packet): Promise<void> {
    this.sent.push(packet);
    if (packet.opcode === 200) this.receive(new Packet(201, packet.transaction).setText(1, 'token').setText(2, 'sid'));
    else if (packet.opcode === 202) this.receive(new Packet(203, packet.transaction).setText(26, 'http://localhost:8080/sites/index.php').setText(28, 'Initial status'),
      new Packet(924, 0).setText(2, '255;32768;invalid;16777216'), new Packet(916, 0).setText(1, '[smile] [wave]').setText(2, '[special]'));
    else if (packet.opcode === 400) this.receive(new Packet(403, packet.transaction));
    else this.handler?.(packet);
  }
  receive(...packets: Packet[]): void { this.events!.packets(packets); }
  setSession(): void {}
  activate(): void {}
  close(): void {}
}
const credentials = { username: 'alice', password: 'alice-demo' };
const connection = { mode: 'http', httpURL: 'http://localhost:8080' } as const;
async function setup() {
  const transports: Transport[] = [];
  const controller = new SessionController(() => {
    const transport = new Transport(); transports.push(transport);
    return new Mig33Client(transport, { requestTimeoutMs: 2000 });
  }, 'test');
  await controller.login(credentials, connection);
  return { controller, transport: transports[0], transports };
}
function text(kind: number, target: string, sender: string, body: string, content = 1): Packet {
  return new Packet(500, 0).setByte(1, 1).setText(2, sender).setByte(3, kind).setText(4, target).setShort(6, content).setText(8, body);
}
async function join(controller: SessionController, transport: Transport): Promise<void> {
  const joining = controller.joinRoom('Lobby'); await tick();
  const request = transport.sent.at(-1)!;
  transport.receive(new Packet(701, request.transaction).setText(1, 'Lobby'), new Packet(1, request.transaction), text(3, 'Lobby', 'Lobby', 'Welcome'));
  assert.equal(await joining, true);
}

test('native moderation keeps exact706/709/710 fields, refreshes roles after ACK and separates notice from error', async () => {
  const { controller, transport } = await setup();
  try {
    await join(controller, transport);
    for (const [operation, opcode, muted] of [['mute', 709, true], ['unmute', 710, false], ['kick', 706, false]] as const) {
      const action = controller.moderateRoom('Lobby', 'BOB', operation);
      assert.equal(await controller.moderateRoom('Lobby', 'BOB', operation), false);
      await tick();
      const request = transport.sent.at(-1)!;
      assert.deepEqual([request.opcode, request.text(1), request.text(2)], [opcode, 'Lobby', 'bob']);
      transport.receive(new Packet(1, request.transaction).setText(1, 'Server confirmation'));
      await tick();
      const participants = transport.sent.at(-1)!;
      assert.equal(participants.opcode, 707);
      transport.receive(new Packet(708, participants.transaction).setText(2, 'alice;bob;').setText(3, 'alice;').setText(4, muted ? 'bob;' : ''));
      assert.equal(await action, true);
      assert.equal(controller.getSnapshot().roomConversations[0].participants[1].muted, muted);
      assert.deepEqual(controller.getSnapshot().notice, { title: 'Room action', text: 'Server confirmation' });
      assert.equal(controller.getSnapshot().error, '');
    }
    controller.clearNotice();
    const rejected = controller.moderateRoom('Lobby', 'alice', 'kick'); await tick();
    transport.receive(new Packet(0, transport.sent.at(-1)!.transaction).setText(2, 'You cannot moderate yourself'));
    assert.equal(await rejected, false);
    assert.equal(controller.getSnapshot().notice, null);
    assert.match(controller.getSnapshot().error, /cannot moderate yourself/);
  } finally { controller.dispose(); }
});

test('personal block uses immutable417 mode2; adding a friend updates the roster through402', async () => {
  const { controller, transport } = await setup();
  try {
    const blocking = controller.blockUser(' BOB '); await tick();
    const block = transport.sent.at(-1)!;
    assert.deepEqual([block.opcode, block.text(1), block.byte(2)], [417, 'bob', 2]);
    transport.receive(new Packet(1, block.transaction));
    assert.equal(await blocking, true);
    assert.equal(controller.getSnapshot().notice?.title, 'User blocked');
    const adding = controller.addContact('carol'); await tick();
    const add = transport.sent.at(-1)!;
    assert.deepEqual([add.opcode, add.text(12), add.byte(24)], [405, 'carol', 0]);
    transport.receive(new Packet(402, add.transaction).setInt(1, 12).setText(12, 'carol').setText(3, 'Carol').setByte(13, 1));
    assert.equal(await adding, true);
    assert.equal(controller.getSnapshot().contacts[0].username, 'carol');
  } finally { controller.dispose(); }
});

test('roster mutations preserve native optional fields and apply ACK changes before later authoritative pushes', async () => {
  const { controller, transport } = await setup();
  try {
    const adding = controller.addContact(' BOB ', { display: 'Robert', mobile: '123', groupId: -1, shareMobile: true }); await tick();
    let request = transport.sent.at(-1)!;
    assert.deepEqual([request.opcode, request.text(12), request.text(3), request.text(8), request.int(2), request.byte(24)],
      [405, 'bob', 'Robert', '123', -1, 1]);
    transport.receive(new Packet(402, request.transaction).setInt(1, 12).setText(12, 'bob').setText(3, 'Robert'));
    assert.equal(await adding, true);
    const updating = controller.updateContact(12, { display: 'Bob', mobile: '' }); await tick();
    request = transport.sent.at(-1)!;
    assert.deepEqual([request.opcode, request.int(1), request.text(3), request.text(8)], [407, 12, 'Bob', '']);
    assert.equal(request.fields.has(2), false); assert.equal(request.fields.has(12), false);
    transport.receive(new Packet(402, request.transaction).setInt(1, 12).setText(12, 'bob').setText(3, 'Bob'),
      new Packet(421, 0).setInt(1, 12).setText(2, 'Newer status'));
    assert.equal(await updating, true);
    assert.equal(controller.getSnapshot().contacts[0].status, 'Newer status');
    const moving = controller.moveContact(12, -1); await tick();
    request = transport.sent.at(-1)!;
    assert.equal(request.opcode, 411); assert.equal(request.fields.has(2), false);
    transport.receive(new Packet(1, request.transaction), new Packet(402, 0).setInt(1, 12).setText(12, 'bob').setInt(2, 7));
    assert.equal(await moving, true);
    assert.equal(controller.getSnapshot().contacts[0].groupId, 7);
    const deleting = controller.deleteContact(12); await tick();
    request = transport.sent.at(-1)!;
    assert.deepEqual([request.opcode, request.int(1)], [406, 12]);
    transport.receive(new Packet(1, request.transaction), new Packet(402, 0).setInt(1, 12).setText(12, 'bob').setText(3, 'Re-added'));
    assert.equal(await deleting, true);
    assert.equal(controller.getSnapshot().contacts[0].display, 'Re-added');
  } finally { controller.dispose(); }
});

test('contact group CRUD retains source ordering, respects rejection and never overwrites a later401', async () => {
  const { controller, transport } = await setup();
  try {
    transport.receive(new Packet(401, 0).setInt(1, 3).setText(2, 'First'));
    const creating = controller.createContactGroup('Work'); await tick();
    let request = transport.sent.at(-1)!;
    assert.deepEqual([request.opcode, request.text(2)], [408, 'Work']);
    transport.receive(new Packet(401, request.transaction).setInt(1, 7).setText(2, 'Work'));
    assert.equal(await creating, true);
    const renaming = controller.renameContactGroup(7, 'Team'); await tick();
    request = transport.sent.at(-1)!;
    assert.deepEqual([request.opcode, request.int(1), request.text(2)], [410, 7, 'Team']);
    transport.receive(new Packet(1, request.transaction), new Packet(401, 0).setInt(1, 7).setText(2, 'Later name'));
    assert.equal(await renaming, true);
    assert.deepEqual(controller.getSnapshot().groups, [{ id: 3, name: 'First' }, { id: 7, name: 'Later name' }]);
    const rejected = controller.deleteContactGroup(7); await tick();
    transport.receive(new Packet(0, transport.sent.at(-1)!.transaction).setText(2, 'Group is not empty'));
    assert.equal(await rejected, false); assert.equal(controller.getSnapshot().groups.length, 2);
    const deleting = controller.deleteContactGroup(7); await tick();
    request = transport.sent.at(-1)!;
    assert.deepEqual([request.opcode, request.int(1)], [409, 7]);
    transport.receive(new Packet(1, request.transaction));
    assert.equal(await deleting, true); assert.deepEqual(controller.getSnapshot().groups, [{ id: 3, name: 'First' }]);
  } finally { controller.dispose(); }
});

test('own presence/status use original old-value fields; rejection and newer603 preserve authoritative state', async () => {
  const { controller, transport } = await setup();
  try {
    assert.equal(controller.getSnapshot().ownStatus, 'Initial status');
    const presence = controller.setPresence(99); await tick();
    let request = transport.sent.at(-1)!;
    assert.deepEqual([request.opcode, request.byte(1), request.byte(2)], [600, 99, 1]);
    assert.equal(controller.getSnapshot().presence, 1);
    transport.receive(new Packet(1, request.transaction));
    assert.equal(await presence, true); assert.equal(controller.getSnapshot().presence, 99);
    const rejected = controller.setPresence(3); await tick();
    request = transport.sent.at(-1)!;
    assert.equal(request.byte(2), 99);
    transport.receive(new Packet(0, request.transaction).setText(2, 'Try later'));
    assert.equal(await rejected, false); assert.equal(controller.getSnapshot().presence, 99);
    const status = controller.setStatus('Updated'); await tick();
    request = transport.sent.at(-1)!;
    assert.deepEqual([request.opcode, request.text(1), request.text(2)], [601, 'Updated', 'Initial status']);
    transport.receive(new Packet(1, request.transaction), new Packet(603, 0).setText(1, 'Newer status'));
    assert.equal(await status, true); assert.equal(controller.getSnapshot().ownStatus, 'Newer status');
    transport.receive(new Packet(603, 0).setText(2, 'Avatar only'));
    assert.equal(controller.getSnapshot().ownStatus, 'Newer status');
  } finally { controller.dispose(); }
});

test('750 converts private history and draft before following kind2 message, with separate group and room identity', async () => {
  const { controller, transport } = await setup();
  try {
    controller.openConversation('bob');
    transport.receive(text(1, 'alice', 'bob', 'Private history'));
    controller.setDraft('bob', 'Preserved draft');
    const creating = controller.createGroup('bob', 'carol'); await tick();
    const request = transport.sent.at(-1)!;
    assert.deepEqual([request.opcode, request.text(1), request.text(2), request.byte(3)], [751, 'bob', 'carol', 1]);
    transport.receive(new Packet(750, request.transaction).setText(1, 'Lobby').setByte(3, 1), text(2, 'Lobby', 'Lobby', 'Group created'));
    assert.equal(await creating, true);
    let state = controller.getSnapshot();
    assert.equal(state.activePeer, null);
    assert.equal(state.activeGroup, 'Lobby');
    assert.equal(state.conversations.length, 0);
    assert.deepEqual(state.groupConversations[0].rows.map((row) => row.body), ['Private history', 'Group created']);
    assert.equal(state.groupConversations[0].draft, 'Preserved draft');
    await join(controller, transport);
    controller.openRoom('Lobby');
    transport.receive(text(2, 'Lobby', 'bob', 'Group unread'), text(2, 'Lobby', 'bob', 'Group unread'));
    state = controller.getSnapshot();
    assert.equal(state.roomConversations.length, 1);
    assert.equal(state.groupConversations[0].unread, 2);
    controller.openGroup('Lobby');
    assert.equal(controller.getSnapshot().activeRoom, null);
    assert.equal(controller.getSnapshot().groupConversations[0].unread, 0);
  } finally { controller.dispose(); }
});

test('a page-hidden private chat stays unread-eligible when750 converts it to a group, until returning', async () => {
  const { controller, transport } = await setup();
  try {
    controller.openConversation('bob');
    controller.setDraft('bob', 'Preserved group draft');
    controller.setConversationVisible(false);
    transport.receive(new Packet(750, 0).setText(1, 'g-one').setText(2, 'bob'),
      text(2, 'g-one', 'bob', 'Converted behind page'), text(2, 'g-two', 'carol', 'Other group'),
      text(2, 'g-one', 'alice', 'Own group action', 6));
    let state = controller.getSnapshot();
    assert.equal(state.activePeer, null);
    assert.equal(state.activeGroup, 'g-one');
    assert.equal(state.conversationVisible, false);
    assert.equal(state.groupConversations[0].draft, 'Preserved group draft');
    assert.deepEqual(state.groupConversations.map((group) => group.unread), [1, 1]);
    controller.setConversationVisible(true);
    transport.receive(text(2, 'g-one', 'bob', 'While writing'));
    state = controller.getSnapshot();
    assert.deepEqual(state.groupConversations.map((group) => group.unread), [0, 1]);
    assert.equal(state.groupConversations[0].draft, 'Preserved group draft');
    controller.setConversationVisible(false);
    controller.openGroup('g-one');
    transport.receive(text(2, 'g-one', 'bob', 'Immediately after selecting'));
    assert.equal(controller.getSnapshot().groupConversations[0].unread, 0);
    assert.equal(controller.getSnapshot().conversationVisible, true);
  } finally { controller.dispose(); }
});

test('unsolicited750 converts only an existing private chat, and a late private send ACK follows its converted thread', async () => {
  const { controller, transport } = await setup();
  try {
    transport.receive(new Packet(750, 0).setText(1, 'unopened').setText(2, 'absent'));
    assert.equal(controller.getSnapshot().groupConversations.length, 0);
    transport.receive(text(2, 'unopened', 'carol', 'Invited group message'));
    assert.equal(controller.getSnapshot().groupConversations[0].unread, 1);
    controller.openConversation('bob'); controller.setDraft('bob', 'pending message');
    const sending = controller.send(); await tick();
    const request = transport.sent.at(-1)!;
    transport.receive(new Packet(750, 0).setText(1, 'converted').setText(2, 'bob'),
      text(2, 'converted', 'bob', 'Converted'), new Packet(1, request.transaction));
    assert.equal(await sending, true);
    const converted = controller.getSnapshot().groupConversations.find((group) => group.id === 'converted')!;
    assert.deepEqual(converted.rows.map((row) => row.body), ['Converted', 'pending message']);
    assert.equal(converted.draft, '');
    assert.equal(controller.getSnapshot().conversations.length, 0);
  } finally { controller.dispose(); }
});

test('group send and leave preserve draft edits, suppress pending-leave traffic, and allow a later server invitation', async () => {
  const { controller, transport } = await setup();
  try {
    transport.receive(text(2, 'g-one', 'bob', 'Hello'));
    controller.openGroup('g-one'); controller.setGroupDraft('g-one', ' /me waves ');
    const sending = controller.sendGroup(); await tick();
    const request = transport.sent.at(-1)!;
    assert.deepEqual([request.byte(3), request.text(4), request.text(8)], [2, 'g-one', '/me waves']);
    controller.setGroupDraft('g-one', 'Later edit');
    transport.receive(text(2, 'g-one', 'alice', 'alice waves', 6), new Packet(1, request.transaction));
    assert.equal(await sending, true);
    assert.equal(controller.getSnapshot().groupConversations[0].rows.length, 2);
    assert.equal(controller.getSnapshot().groupConversations[0].draft, 'Later edit');
    const rejected = controller.leaveGroup('g-one'); await tick();
    transport.receive(text(2, 'g-one', 'bob', 'Suppressed'), new Packet(0, transport.sent.at(-1)!.transaction).setText(2, 'Try later'));
    assert.equal(await rejected, false);
    assert.equal(controller.getSnapshot().groupConversations[0].state, 'joined');
    const leaving = controller.leaveGroup('g-one'); await tick();
    assert.deepEqual([transport.sent.at(-1)!.opcode, transport.sent.at(-1)!.byte(2)], [753, 1]);
    transport.receive(new Packet(1, transport.sent.at(-1)!.transaction), text(2, 'g-one', 'bob', 'Suppressed same batch'));
    assert.equal(await leaving, true);
    assert.equal(controller.getSnapshot().groupConversations.length, 0);
    transport.receive(text(2, 'g-one', 'bob', 'You were invited again'));
    assert.equal(controller.getSnapshot().groupConversations[0].rows[0].body, 'You were invited again');
  } finally { controller.dispose(); }
});

test('explicit room rejoin sends703 again and preserves transcript/draft; local close does not pretend to send704', async () => {
  const { controller, transport } = await setup();
  try {
    await join(controller, transport);
    controller.openRoom('Lobby'); controller.setRoomDraft('Lobby', 'Keep me');
    transport.receive(new Packet(5, 0).setText(2, 'You were kicked.').setText(4, 'Chat room'));
    assert.equal(controller.getSnapshot().notice?.text, 'You were kicked.');
    assert.equal(controller.getSnapshot().error, '');
    const rejoining = controller.rejoinRoom('Lobby'); await tick();
    const request = transport.sent.at(-1)!;
    assert.equal(request.opcode, 703);
    transport.receive(new Packet(701, request.transaction).setText(1, 'Lobby'), new Packet(1, request.transaction), text(3, 'Lobby', 'Lobby', 'Welcome again'));
    assert.equal(await rejoining, true);
    assert.equal(controller.getSnapshot().roomConversations[0].draft, 'Keep me');
    assert.deepEqual(controller.getSnapshot().roomConversations[0].rows.map((row) => row.body), ['Welcome', 'Welcome again']);
    const sent = transport.sent.length;
    controller.closeRoom('Lobby');
    assert.equal(transport.sent.length, sent);
    assert.equal(controller.getSnapshot().roomConversations.length, 0);
  } finally { controller.dispose(); }
});

test('login resources retain original site, colors and token metadata; selected RGB is sent and local tokens render', async () => {
  const { controller, transport } = await setup();
  try {
    assert.equal(controller.getSnapshot().siteURL, 'http://localhost:8080/sites/index.php');
    assert.deepEqual(controller.getSnapshot().chatColors, [0, 255, 32768]);
    assert.deepEqual(controller.getSnapshot().emoticons, { standard: ['[smile]', '[wave]'], other: ['[special]'] });
    controller.openConversation('bob'); controller.setTextColor(255); controller.setDraft('bob', 'Hello [smile]');
    const sending = controller.send(); await tick();
    assert.equal(transport.sent.at(-1)!.int(13), 255);
    transport.receive(new Packet(1, transport.sent.at(-1)!.transaction)); await sending;
    const row = controller.getSnapshot().conversations[0].rows[0];
    assert.equal(row.color, 255); assert.deepEqual(row.tokens, ['[smile]']);
    controller.setTextColor(-1); controller.setDraft('bob', 'Default');
    const defaultSend = controller.send(); await tick();
    assert.equal(transport.sent.at(-1)!.fields.has(13), false);
    transport.receive(new Packet(1, transport.sent.at(-1)!.transaction)); await defaultSend;
    transport.receive(new Packet(911, 0).setInt(1, 11).setInt(2, 3).setText(3, 'Room page').setText(4, '/room'));
    assert.deepEqual(controller.getSnapshot().serverMenus.map(({ category, index, label, value }) => ({ category, index, label, value })),
      [{ category: 11, index: 3, label: 'Room page', value: '/room' }]);
  } finally { controller.dispose(); }
});

test('photo send preserves raw500 bytes or reference semantics and retains the text draft', async () => {
  const { controller, transport } = await setup();
  const png = Uint8Array.of(137, 80, 78, 71, 13, 10, 26, 10);
  try {
    controller.openConversation('bob'); controller.setDraft('bob', 'Unsent text');
    const sending = controller.sendPhoto(1, 'bob', png); await tick();
    const request = transport.sent.at(-1)!;
    assert.equal(request.short(6), 2); assert.deepEqual(request.fields.get(8), png);
    transport.receive(new Packet(1, request.transaction)); assert.equal(await sending, true);
    let conversation = controller.getSnapshot().conversations[0];
    assert.equal(conversation.draft, 'Unsent text');
    assert.equal(conversation.rows[0].imageURI, 'data:image/png;base64,iVBORw0KGgo=');
    const reference = controller.sendPhoto(1, 'bob', '42'); await tick();
    assert.deepEqual([transport.sent.at(-1)!.short(6), transport.sent.at(-1)!.text(8)], [5, '42']);
    transport.receive(new Packet(1, transport.sent.at(-1)!.transaction)); await reference;
    conversation = controller.getSnapshot().conversations[0];
    assert.deepEqual(conversation.rows.at(-1)!.tokens, ['[photo:42]']);
    transport.receive(text(1, 'alice', 'bob', 'Photo [photo:43]', 5).setText(11, '[photo:43]'));
    assert.equal(controller.getSnapshot().conversations[0].rows.at(-1)!.mediaReference, '43');
    transport.receive(text(1, 'alice', 'bob', '', 2).setBytes(8, png));
    assert.equal(controller.getSnapshot().conversations[0].rows.at(-1)!.imageURI, 'data:image/png;base64,iVBORw0KGgo=');
    assert.equal(controller.getSnapshot().conversations[0].rows.at(-1)!.body, 'Photo');
    const converting = controller.sendPhoto(1, 'bob', png); await tick();
    transport.receive(new Packet(750, 0).setText(1, 'photo-group').setText(2, 'bob'),
      new Packet(1, transport.sent.at(-1)!.transaction));
    assert.equal(await converting, true);
    assert.equal(controller.getSnapshot().groupConversations[0].rows.at(-1)!.imageURI, 'data:image/png;base64,iVBORw0KGgo=');
    assert.equal(controller.getSnapshot().groupConversations[0].draft, 'Unsent text');
    assert.equal(controller.getSnapshot().conversations.length, 0);
  } finally { controller.dispose(); }
});

test('resource invalidation915 rejects older914 replies and permits a fresh correlated fetch', async () => {
  const { controller, transport } = await setup();
  try {
    const old = controller.loadEmoticon('[smile]'); await tick();
    const a = transport.sent.at(-1)!;
    assert.equal(await controller.loadEmoticon('[smile]'), false);
    transport.receive(new Packet(915, 0));
    const fresh = controller.loadEmoticon('[smile]'); await tick();
    const b = transport.sent.at(-1)!;
    const png = Uint8Array.of(137, 80, 78, 71, 13, 10, 26, 10);
    transport.receive(new Packet(914, a.transaction).setText(1, '[smile]').setText(3, 'Old').setBytes(4, png));
    assert.equal(await old, false); assert.deepEqual(controller.getSnapshot().emoticonAssets, {});
    transport.receive(new Packet(914, b.transaction).setText(1, '[smile]').setText(3, 'Fresh').setBytes(4, png));
    assert.equal(await fresh, true);
    assert.equal(controller.getSnapshot().emoticonAssets['[smile]'].label, 'Fresh');
  } finally { controller.dispose(); }
});

test('original page bridge assigns correlation and rejects stale-session replies without updating a new login', async () => {
  const { controller, transport } = await setup();
  try {
    const fetching = controller.requestOriginal(922, (packet) => packet.setByte(1, 1).setText(2, 'http://localhost/page'), [923], 923);
    const rejection = assert.rejects(fetching, /Client closed|closed session/);
    await tick();
    const request = transport.sent.at(-1)!;
    assert.equal(request.opcode, 922); assert.ok(request.transaction > 0);
    await controller.login(credentials, connection);
    transport.receive(new Packet(923, request.transaction).setInt(1, 200).setText(2, '<p>Old page</p>'));
    await rejection;
    assert.equal(controller.getSnapshot().error, '');
  } finally { controller.dispose(); }
});
