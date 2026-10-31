import assert from 'node:assert/strict';
import { setImmediate as tick } from 'node:timers/promises';
import test from 'node:test';
import { Mig33Client } from '../src/mig33/client.ts';
import { Packet } from '../src/mig33/protocol/index.ts';
import type { PacketTransport, TransportEvents } from '../src/mig33/transport/types.ts';
import { SessionController } from '../src/ui/session/controller.ts';
import { contactSections, isAvailable, presenceLabel } from '../src/ui/session/model.ts';

class FakeTransport implements PacketTransport {
  readonly kind = 'http' as const;
  events?: TransportEvents;
  sent: Packet[] = [];
  autoAcknowledge = true;
  connectWait?: Promise<void>;
  async connect(events: TransportEvents): Promise<void> { this.events = events; await this.connectWait; }
  async send(packet: Packet): Promise<void> {
    this.sent.push(packet);
    if (packet.opcode === 200) this.receive(new Packet(201, packet.transaction).setText(1, 'token').setText(2, 'sid'));
    else if (packet.opcode === 202) this.receive(new Packet(203, packet.transaction));
    else if (packet.opcode === 400) this.receive(
      new Packet(401, packet.transaction).setInt(1, 8).setText(2, 'Family'),
      new Packet(401, packet.transaction).setInt(1, 3).setText(2, 'Friends'),
      new Packet(402, packet.transaction).setInt(1, 1).setInt(2, 3).setText(12, 'bob').setByte(13, 4),
      new Packet(402, packet.transaction).setInt(1, 2).setInt(2, 8).setText(12, 'carol').setByte(13, 27),
      new Packet(403, packet.transaction),
    );
    else if (packet.opcode === 300 || this.autoAcknowledge) this.receive(new Packet(1, packet.transaction));
  }
  setSession(): void {}
  activate(): void {}
  close(): void {}
  receive(...packets: Packet[]): void { this.events!.packets(packets); }
  acknowledge(): void { this.receive(new Packet(1, this.sent.at(-1)!.transaction)); }
}

const connection = { mode: 'https', httpURL: 'https://localhost:8443' } as const;
const credentials = { username: 'Alice', password: 'alice-demo' };
function setup() {
  const transports: FakeTransport[] = [];
  const controller = new SessionController(() => {
    const transport = new FakeTransport();
    transports.push(transport);
    return new Mig33Client(transport, { requestTimeoutMs: 2000 });
  }, 'test');
  return { controller, transports };
}
function incoming(sender: string, body: string, target = 'alice'): Packet {
  return new Packet(500, 0).setText(2, sender).setByte(3, 1).setText(4, target).setShort(6, 1).setText(8, body);
}

test('session login exposes immutable identity, source presence, and server-ordered contact groups', async () => {
  const { controller, transports } = setup();
  try {
    assert.equal(await controller.login({ ...credentials, invisible: true }, connection), true);
    const state = controller.getSnapshot();
    assert.equal(state.username, 'alice');
    assert.equal(state.presence, 99);
    assert.equal(transports[0].sent[0].byte(9), 99);
    const sections = contactSections(state.groups, state.contacts);
    assert.deepEqual(sections.map((group) => [group.name, group.contacts[0].username, group.online]), [
      ['Family', 'carol', 0], ['Friends', 'bob', 1],
    ]);
    assert.equal(presenceLabel(27), 'Unknown');
    assert.equal(isAvailable(27), false);
    assert.deepEqual([1, 3, 4, 99].map(presenceLabel), ['Online', 'Busy', 'Away', 'Offline']);
  } finally { controller.dispose(); }
});

test('private threads count unread independently and retain repeated equal messages', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    const first = incoming('bob', 'hello');
    transport.receive(first, first, incoming('bob', 'hello'), incoming('carol', 'hi'));
    assert.deepEqual(controller.getSnapshot().conversations.map((chat) => [chat.peer, chat.rows.length, chat.unread]), [
      ['bob', 2, 2], ['carol', 1, 1],
    ]);
    controller.openConversation(' Bob ');
    transport.receive(incoming('bob', 'visible'));
    assert.equal(controller.getSnapshot().conversations[0].unread, 0);
    controller.selectConversation(null);
    transport.receive(incoming('bob', 'unread'));
    assert.equal(controller.getSnapshot().conversations[0].unread, 1);
    transport.receive(incoming('someone', 'room text', 'room').setByte(3, 3));
    assert.equal(controller.getSnapshot().conversations.length, 2);
  } finally { controller.dispose(); }
});

test('pages retain private identity and draft while unread accumulates, and returning clears only that chat', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    assert.equal(controller.getSnapshot().conversationVisible, true);
    controller.openConversation('bob');
    controller.setDraft('bob', 'Keep my draft');
    controller.setConversationVisible(false);
    transport.receive(incoming('bob', 'Behind the page'), incoming('carol', 'Another chat'),
      incoming('alice', 'Own message', 'bob'));
    let state = controller.getSnapshot();
    assert.equal(state.activePeer, 'bob');
    assert.equal(state.conversations[0].draft, 'Keep my draft');
    assert.deepEqual(state.conversations.map((chat) => [chat.peer, chat.unread]), [['bob', 1], ['carol', 1]]);
    controller.setConversationVisible(false);
    transport.receive(incoming('bob', 'Still behind the page'));
    assert.equal(controller.getSnapshot().conversations[0].unread, 2);
    controller.setConversationVisible(true);
    // Write and conversation menus retain the same logical selection.
    transport.receive(incoming('bob', 'While writing'));
    state = controller.getSnapshot();
    assert.deepEqual(state.conversations.map((chat) => [chat.peer, chat.unread]), [['bob', 0], ['carol', 1]]);
    assert.equal(state.conversations[0].draft, 'Keep my draft');
    controller.setConversationVisible(false);
    controller.closeConversation('bob');
    controller.setConversationVisible(true);
    assert.equal(controller.getSnapshot().activePeer, null);
    assert.deepEqual(controller.getSnapshot().conversations.map((chat) => chat.peer), ['carol']);
    transport.receive(incoming('bob', 'New message after closing'));
    assert.equal(controller.getSnapshot().conversations.at(-1)!.unread, 1);
    controller.setConversationVisible(false);
    controller.openConversation('bob');
    transport.receive(incoming('bob', 'Immediately after selecting'));
    assert.equal(controller.getSnapshot().conversations.at(-1)!.unread, 0);
    assert.equal(controller.getSnapshot().conversationVisible, true);
    controller.setConversationVisible(false);
    await controller.login(credentials, connection);
    assert.equal(controller.getSnapshot().conversationVisible, true, 'a new session resets selection visibility');
  } finally { controller.dispose(); }
});

test('canceling connection progress prevents late login and ignores a double login tap', async () => {
  const transport = new FakeTransport();
  let finishConnect!: () => void;
  transport.connectWait = new Promise((resolve) => { finishConnect = resolve; });
  let clients = 0;
  const controller = new SessionController(() => { clients++; return new Mig33Client(transport); }, 'test');
  try {
    const login = controller.login(credentials, connection);
    assert.equal(await controller.login(credentials, connection), false);
    assert.equal(clients, 1);
    controller.cancel();
    finishConnect();
    assert.equal(await login, false);
    assert.equal(controller.getSnapshot().state, 'closed');
    assert.equal(controller.getSnapshot().error, '');
    assert.deepEqual(transport.sent, []);
  } finally { controller.dispose(); }
});

test('drafts belong to their conversations and new edits survive an older send acknowledgment', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const transport = transports[0];
    transport.autoAcknowledge = false;
    controller.openConversation('bob');
    controller.setDraft('bob', '  first\n');
    controller.setDraft('carol', 'separate');
    const sending = controller.send();
    await tick();
    assert.equal(transport.sent.at(-1)!.text(8), 'first', 'the original Write editor trims the submitted payload');
    assert.equal(await controller.send(), false, 'a second tap does not duplicate the send');
    controller.setDraft('bob', 'edited while waiting');
    transport.acknowledge();
    assert.equal(await sending, true);
    const [bob, carol] = controller.getSnapshot().conversations;
    assert.equal(bob.rows[0].body, 'first');
    assert.equal(bob.draft, 'edited while waiting');
    assert.equal(carol.draft, 'separate');
    transport.autoAcknowledge = true;
    controller.setDraft('bob', '  next message  ');
    assert.equal(await controller.send(), true);
    assert.equal(controller.getSnapshot().conversations[0].draft, '', 'an unchanged padded draft clears after its acknowledgment');
    assert.equal(controller.getSnapshot().conversations[0].rows.at(-1)!.body, 'next message');
  } finally { controller.dispose(); }
});

test('closing a conversation during send does not reopen it when the acknowledgment arrives', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    transports[0].autoAcknowledge = false;
    controller.openConversation('bob');
    controller.setDraft('bob', 'closing');
    const sending = controller.send();
    await tick();
    controller.closeConversation('bob');
    transports[0].acknowledge();
    await sending;
    assert.deepEqual(controller.getSnapshot().conversations, []);
    assert.equal(controller.getSnapshot().activePeer, null);
  } finally { controller.dispose(); }
});

test('a failed send retains its draft and a later session ignores the old operation', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    const old = transports[0];
    old.autoAcknowledge = false;
    controller.openConversation('bob');
    controller.setDraft('bob', 'unsent');
    const rejected = controller.send();
    await tick();
    old.receive(new Packet(0, old.sent.at(-1)!.transaction).setText(2, 'No such user'));
    assert.equal(await rejected, false);
    assert.equal(controller.getSnapshot().conversations[0].draft, 'unsent');
    assert.equal(controller.getSnapshot().error, 'No such user');
    const interrupted = controller.send();
    await tick();
    controller.suspend();
    assert.equal(await controller.login({ username: 'carol', password: 'carol-demo' }, connection), true);
    await interrupted;
    old.receive(incoming('bob', 'late'));
    assert.equal(controller.getSnapshot().username, 'carol');
    assert.deepEqual(controller.getSnapshot().conversations, []);
    assert.equal(controller.getSnapshot().error, '');
  } finally { controller.dispose(); }
});

test('server-rendered actions and self-addressed messages produce no duplicate local echo', async () => {
  const { controller, transports } = setup();
  try {
    await controller.login(credentials, connection);
    controller.openConversation('bob');
    controller.setDraft('bob', '/me waves');
    await controller.send();
    transports[0].receive(incoming('bob', 'alice waves').setShort(6, 6));
    assert.equal(controller.getSnapshot().conversations[0].rows.length, 1);
    assert.equal(controller.getSnapshot().conversations[0].rows[0].action, true);
    transports[0].receive(incoming('bob', 'colored action').setShort(6, 6).setInt(13, 0x123456).setInt(12, 0xabcdef));
    assert.equal(controller.getSnapshot().conversations[0].rows[1].action, false);
    assert.equal(controller.getSnapshot().conversations[0].rows[1].color, 0x123456);
    assert.equal(controller.getSnapshot().conversations[0].rows[1].senderColor, 0xabcdef);
    controller.openConversation('alice');
    controller.setDraft('alice', 'note to myself');
    await controller.send();
    transports[0].receive(incoming('alice', 'note to myself'));
    assert.equal(controller.getSnapshot().conversations[1].rows.length, 1);
  } finally { controller.dispose(); }
});
