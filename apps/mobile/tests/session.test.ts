import assert from 'node:assert/strict';
import { setImmediate as tick } from 'node:timers/promises';
import test from 'node:test';
import { Mig33Client, RemoteError, type ClientEvent } from '../src/mig33/client.ts';
import { Packet, loginChallengeHash } from '../src/mig33/protocol/index.ts';
import type { PacketTransport, TransportEvents } from '../src/mig33/transport/types.ts';

class FakeTransport implements PacketTransport {
  readonly kind = 'http' as const;
  events?: TransportEvents;
  sent: Packet[] = [];
  session = '';
  activations = 0;
  closes = 0;
  connection?: Promise<void>;
  closeNotifies = false;
  handler?: (packet: Packet) => void;

  async connect(events: TransportEvents): Promise<void> {
    this.events = events;
    await this.connection;
  }
  send(packet: Packet): Promise<void> {
    this.sent.push(packet);
    if (packet.opcode === 200) this.receive(new Packet(201, packet.transaction).setText(1, 'challenge').setText(2, 'session'));
    else if (packet.opcode === 202) this.receive(new Packet(203, packet.transaction));
    this.handler?.(packet);
    return Promise.resolve();
  }
  setSession(session: string): void { this.session = session; }
  activate(): void { this.activations++; }
  close(): void {
    this.closes++;
    if (this.closeNotifies) this.events?.closed(new Error('Socket closed synchronously'));
  }
  receive(...packets: Packet[]): void { this.events!.packets(packets); }
}

async function online(): Promise<{ transport: FakeTransport; client: Mig33Client }> {
  const transport = new FakeTransport();
  const client = new Mig33Client(transport, { requestTimeoutMs: 5000 });
  await client.login({ username: ' Alice ', password: 'MiXeD 😀' });
  return { transport, client };
}

test('original login performs 200/201/202/203 with session assignment and UTF-16 proof', async () => {
  const { transport, client } = await online();
  try {
    assert.equal(client.state, 'online');
    assert.equal(client.username, 'alice');
    assert.equal(transport.session, 'session');
    assert.equal(transport.activations, 1);
    assert.deepEqual(transport.sent.map((packet) => packet.opcode), [200, 202]);
    const [login, proof] = transport.sent;
    assert.equal(login.text(5), 'alice');
    assert.equal(login.text(7), 'J2MEv4.62.000');
    assert.equal(login.short(3), 462);
    assert.equal(login.fields.has(6), false, 'Login200 must not introduce a password field');
    assert.equal(proof.int(1), loginChallengeHash('challenge', 'MiXeD 😀'));
    assert.notEqual(login.transaction, proof.transaction);
    await assert.rejects(client.login({ username: 'alice', password: 'x' }), /new client/i);
  } finally { client.close(); }
});

test('contact snapshot completes only after matching 403, retaining all 402 records', async () => {
  const { transport, client } = await online();
  try {
    let complete = false;
    const snapshot = client.syncContacts().then((contacts) => { complete = true; return contacts; });
    await tick();
    const transaction = transport.sent.at(-1)!.transaction;
    transport.receive(
      new Packet(401, transaction).setInt(1, 7).setText(2, 'Friends'),
      new Packet(402, transaction).setInt(1, 1).setText(12, 'bob').setText(3, 'Bob').setByte(13, 1),
      new Packet(420, transaction).setInt(1, 12).setLong(2, 1234567890123n),
      new Packet(403, transaction + 1),
    );
    await tick();
    assert.equal(complete, false);
    transport.receive(
      new Packet(402, transaction).setInt(1, 2).setText(12, 'carol').setByte(13, 99),
      new Packet(403, transaction),
    );
    const contacts = await snapshot;
    assert.deepEqual(contacts.map((contact) => [contact.id, contact.username, contact.presence]), [
      [1, 'bob', 1], [2, 'carol', 99],
    ]);
  } finally { client.close(); }
});

test('contact pushes after the snapshot terminator in the same batch are not lost', async () => {
  const { transport, client } = await online();
  try {
    const snapshot = client.syncContacts();
    await tick();
    const transaction = transport.sent.at(-1)!.transaction;
    transport.receive(
      new Packet(402, transaction).setInt(1, 1).setText(12, 'bob').setByte(13, 1),
      new Packet(403, transaction),
      new Packet(402, 0).setInt(1, 2).setText(12, 'carol').setByte(13, 3),
      new Packet(404, 0).setInt(1, 1).setByte(2, 4),
    );
    await snapshot;
    assert.deepEqual(client.contacts.map((contact) => [contact.username, contact.presence]), [['bob', 4], ['carol', 3]]);
  } finally { client.close(); }
});

test('a deletion push during contact synchronization is not undone by the completed snapshot', async () => {
  const { transport, client } = await online();
  try {
    const snapshot = client.syncContacts();
    await tick();
    const transaction = transport.sent.at(-1)!.transaction;
    transport.receive(new Packet(402, transaction).setInt(1, 1).setText(12, 'bob').setByte(13, 1));
    transport.receive(new Packet(406, 0).setInt(1, 1));
    transport.receive(new Packet(403, transaction));
    await snapshot;
    assert.deepEqual(client.contacts, []);
  } finally { client.close(); }
});

test('unsolicited chat does not complete a pending request with the same transaction', async () => {
  const { transport, client } = await online();
  const events: ClientEvent[] = [];
  client.subscribe((event) => events.push(event));
  try {
    let complete = false;
    const sending = client.sendMessage('Bob', 'Hello ✓').then(() => { complete = true; });
    await tick();
    const request = transport.sent.at(-1)!;
    assert.equal(request.opcode, 500);
    assert.equal(request.text(4), 'bob');
    assert.equal(request.text(8), 'Hello ✓');
    transport.receive(new Packet(500, request.transaction).setText(2, 'bob').setText(4, 'alice').setText(8, 'Hi 😀'));
    await tick();
    assert.equal(complete, false);
    assert.equal(events.filter((event) => event.type === 'message').length, 1);
    transport.receive(new Packet(1, request.transaction + 1));
    await tick();
    assert.equal(complete, false);
    transport.receive(new Packet(1, request.transaction));
    await sending;
  } finally { client.close(); }
});

test('opcode 0 rejects exactly its request without an implicit retry or session teardown', async () => {
  const { transport, client } = await online();
  try {
    const sending = client.sendMessage('bob', 'first');
    const rejected = assert.rejects(sending, (error: unknown) => error instanceof RemoteError && error.message === 'No such user');
    await tick();
    const request = transport.sent.at(-1)!;
    const count = transport.sent.length;
    transport.receive(new Packet(0, request.transaction).setText(2, 'No such user'));
    await rejected;
    await tick();
    assert.equal(transport.sent.length, count);
    assert.equal(client.state, 'online');
    const second = client.sendMessage('bob', 'second');
    await tick();
    transport.receive(new Packet(1, transport.sent.at(-1)!.transaction));
    await second;
  } finally { client.close(); }
});

test('closing a session rejects all pending operations and ignores late replies', async () => {
  const { transport, client } = await online();
  const first = client.sendMessage('bob', 'first');
  const second = client.sendMessage('carol', 'second');
  const failures = [assert.rejects(first, /closed/i), assert.rejects(second, /closed/i)];
  await tick();
  client.close();
  await Promise.all(failures);
  for (const packet of transport.sent) transport.receive(new Packet(1, packet.transaction));
  assert.equal(client.state, 'closed');
  assert.equal(transport.closes, 1);
});

test('close while connecting cannot resume authentication after connect resolves', async () => {
  const transport = new FakeTransport();
  let connect!: () => void;
  transport.connection = new Promise((resolve) => { connect = resolve; });
  const client = new Mig33Client(transport);
  const login = client.login({ username: 'alice', password: 'alice-demo' });
  const rejected = assert.rejects(login, /closed|cancel/i);
  client.close();
  connect();
  await rejected;
  assert.equal(client.state, 'closed');
  assert.equal(transport.sent.length, 0);
  assert.equal(transport.activations, 0);
});

test('a synchronous transport send failure ends the session and clears request timers', async () => {
  const { transport, client } = await online();
  transport.handler = (packet) => { if (packet.opcode === 500) throw new Error('Write failed synchronously'); };
  try {
    await assert.rejects(client.sendMessage('bob', 'message'), /Write failed synchronously/);
    assert.equal(client.state, 'error');
    assert.equal(transport.closes, 1);
  } finally { client.close(); }
});

test('synchronous close notification cannot recursively fail the client', async () => {
  const { transport, client } = await online();
  transport.closeNotifies = true;
  assert.doesNotThrow(() => client.close());
  assert.equal(client.state, 'closed');
  assert.equal(transport.closes, 1);
});

test('an ambiguous request timeout closes the session and never retries either send', async () => {
  const transport = new FakeTransport();
  const client = new Mig33Client(transport, { requestTimeoutMs: 15 });
  await client.login({ username: 'alice', password: 'alice-demo' });
  const first = client.sendMessage('bob', 'first');
  const second = client.sendMessage('carol', 'second');
  await Promise.all([
    assert.rejects(first, /outcome is unknown/i),
    assert.rejects(second, /outcome is unknown/i),
  ]);
  assert.equal(client.state, 'error');
  assert.equal(transport.closes, 1);
  assert.deepEqual(transport.sent.map((packet) => packet.opcode), [200, 202, 500, 500]);
});
