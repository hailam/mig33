import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createWriteStream } from 'node:fs';
import { writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { createInterface } from 'node:readline';
import test, { before } from 'node:test';
import { Mig33Client, type ChatMessage } from '../src/mig33/client.ts';
import { HTTPTransport } from '../src/mig33/transport/http.ts';
import { TCPTransport } from '../src/mig33/transport/tcp.ts';
import { nodeConnector } from './helpers/node-socket.ts';
import { repositoryRoot, startServer, type TestServer } from './helpers/server.ts';

// Optional original-client test. Downloaded applet/JVM artifacts stay local and
// are unnecessary for normal Go checks and the pure TypeScript test suite.
const enabled = process.env.MIG33_TEST_ORIGINAL_CLIENT === '1' && !!process.env.MIG33_TEST_SERVER_BINARY;
const runner = join(repositoryRoot, 'tools/applets/v4/integration/mobile-peer.sh');

before(() => {
  if (enabled) execFileSync('sh', [runner, '--compile-only'], { cwd: repositoryRoot, stdio: 'pipe' });
});

interface PeerEvent { event: string; [key: string]: string; }

class OriginalPeer {
  private readonly child;
  private readonly lines;
  private readonly log;
  private readonly queued: PeerEvent[] = [];
  private waiting?: { event: string; resolve(event: PeerEvent): void; reject(error: Error): void; timer: ReturnType<typeof setTimeout> };
  private failure?: Error;
  private stopped = false;
  readonly output: string;

  constructor(server: TestServer, transport: 'http' | 'tcp') {
    this.output = join(server.outputDirectory, 'original-' + transport);
    const logPath = join(server.outputDirectory, 'original-' + transport + '.log');
    this.log = createWriteStream(logPath);
    this.child = spawn('sh', [runner, 'bob', transport, this.output], {
      cwd: repositoryRoot,
      env: {
        ...process.env, MOBILE_PEER_SKIP_BUILD: '1',
        MIG33_TEST_HTTP_PORT: String(server.httpPort), MIG33_TEST_TCP_PORT: String(server.tcpPort),
      },
      stdio: ['pipe', 'pipe', 'pipe'],
    });
    this.child.stderr.pipe(this.log, { end: false });
    this.lines = createInterface({ input: this.child.stdout });
    this.lines.on('line', (line) => {
      try {
        const event = JSON.parse(line) as PeerEvent;
        if (event.event === 'error') {
          this.fail(new Error(`Original peer: ${event.message}; see ${logPath}`));
        } else if (this.waiting?.event === event.event) {
          const waiting = this.waiting;
          this.waiting = undefined;
          clearTimeout(waiting.timer);
          waiting.resolve(event);
        } else this.queued.push(event);
      } catch (error) { this.fail(error as Error); }
    });
    this.child.on('error', (error) => this.fail(error));
    this.child.stdin.on('error', (error) => this.fail(error));
    this.child.on('exit', (code, signal) => {
      if (!this.stopped) this.fail(new Error(`Original peer exited ${code ?? signal}; see ${logPath}`));
    });
  }

  command(kind: 'EXPECT' | 'SEND' | 'PRESENCE' | 'STOP' | 'ROOM_JOIN' | 'ROOM_EXPECT' | 'ROOM_SEND'
    | 'ROOM_PARTICIPANTS' | 'ROOM_LEAVE', ...values: string[]): void {
    if (this.failure) throw this.failure;
    this.child.stdin.write([kind, ...values.map((value) => Buffer.from(value, 'utf8').toString('base64'))].join('\t') + '\n');
  }

  async presence(peer: string, value: number): Promise<void> {
    this.command('PRESENCE', peer, String(value));
    const event = await this.next('presence');
    assert.equal(event.peer, peer);
    assert.equal(event.value, String(value));
    assert.equal(event.proof, 'original-404-and-roster-model');
  }

  next(event: string): Promise<PeerEvent> {
    if (this.failure) return Promise.reject(this.failure);
    const found = this.queued.findIndex((value) => value.event === event);
    if (found >= 0) return Promise.resolve(this.queued.splice(found, 1)[0]);
    assert.equal(this.waiting, undefined, 'Original peer commands are sequential');
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.waiting = undefined;
        reject(new Error(`Original peer timed out waiting for ${event}; see ${this.output}`));
      }, 70000);
      this.waiting = { event, resolve, reject, timer };
    });
  }

  async logout(): Promise<void> {
    this.command('STOP');
    assert.equal((await this.next('closed')).proof, 'original-logout-ack');
    this.stopped = true;
  }

  async close(): Promise<void> {
    this.stopped = true;
    this.fail(new Error('Peer closed'));
    this.lines.close();
    if (this.child.exitCode === null && this.child.signalCode === null) {
      const done = new Promise<void>((resolveExit) => this.child.once('exit', () => resolveExit()));
      this.child.kill('SIGTERM');
      const force = setTimeout(() => this.child.kill('SIGKILL'), 10000);
      force.unref();
      await done;
      clearTimeout(force);
    }
    await new Promise<void>((resolveLog) => this.log.end(resolveLog));
  }

  private fail(error: Error): void {
    this.failure ??= error;
    if (this.waiting) {
      clearTimeout(this.waiting.timer);
      this.waiting.reject(error);
      this.waiting = undefined;
    }
  }
}

function messageFrom(client: Mig33Client, sender: string, text: string | RegExp, room?: string): Promise<ChatMessage> {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      unsubscribe();
      reject(new Error('TypeScript client did not receive the original peer reply'));
    }, 30000);
    const unsubscribe = client.subscribe((event) => {
      if (event.type === 'message' && event.message.sender === sender
        && (typeof text === 'string' ? event.message.body === text : text.test(event.message.body))
        && (room === undefined || event.message.kind === 3 && event.message.target === room)) {
        clearTimeout(timeout);
        unsubscribe();
        resolve(event.message);
      }
      if (event.type === 'state' && (event.state === 'error' || event.state === 'closed')) {
        clearTimeout(timeout);
        unsubscribe();
        reject(event.error ?? new Error('TypeScript client failed'));
      }
    });
  });
}

for (const transport of ['http', 'tcp'] as const) {
  test(`TypeScript ${transport} rooms interoperate with unchanged Java ${transport === 'http' ? 'tcp' : 'http'} client`,
    { skip: !enabled, timeout: 150000 }, async (t) => {
      const server = await startServer();
      t.after(() => server.close());
      const peerTransport = transport === 'http' ? 'tcp' : 'http';
      const peer = new OriginalPeer(server, peerTransport);
      t.after(() => peer.close());
      assert.equal((await peer.next('ready')).proof, 'original-login-roster-categories');
      const client = new Mig33Client(transport === 'http'
        ? new HTTPTransport(server.httpURL, { pollIntervalMs: 100 })
        : new TCPTransport(nodeConnector('127.0.0.1', server.tcpPort)));
      t.after(() => client.close());
      await client.login({ username: 'alice', password: 'alice-demo' });
      await client.syncContacts();
      await peer.presence('alice', 1);
      const categories = await client.listRoomCategories();
      const publicCategory = categories.find((category) => !category.favorites);
      assert.ok(publicCategory);
      assert.ok((await client.listRooms(publicCategory.id)).rooms.some((room) => room.name === 'Lobby'));

      peer.command('ROOM_JOIN', 'Lobby');
      const joined = await peer.next('room-joined');
      assert.equal(joined.proof, 'original-703-701-ack-and-ready-room-model');
      const joinEvents: string[] = [];
      const stopJoinEvents = client.subscribe((event) => {
        if (event.type === 'room-joining' && event.room.name === 'Lobby') joinEvents.push('room-joining');
        if (event.type === 'message' && event.message.kind === 3 && event.message.target === 'Lobby') joinEvents.push('message');
      });
      const welcome = messageFrom(client, 'Lobby', /^Welcome to Lobby\./, 'Lobby');
      void welcome.catch(() => {});
      assert.equal((await client.joinRoom('lObBy')).name, 'Lobby');
      // TCP may split ACK1 and welcome500 into different socket reads. The
      // protocol guarantees their order, not a single transport callback.
      await welcome;
      stopJoinEvents();
      assert.deepEqual(joinEvents, ['room-joining', 'message'], 'Correlated room model must precede the join welcome');
      const participants = await client.getRoomParticipants('Lobby');
      assert.equal(participants.room, 'Lobby');
      assert.deepEqual(participants.participants.map((entry) => entry.username), ['alice', 'bob']);
      peer.command('ROOM_PARTICIPANTS', 'Lobby', 'alice;bob');
      assert.equal((await peer.next('room-participants')).proof, 'original-707-708-and-participant-popup');

      const outbound = `Expo room ${transport} ✓`;
      peer.command('ROOM_EXPECT', 'Lobby', 'alice', outbound);
      await client.sendRoomMessage('Lobby', outbound);
      const receipt = await peer.next('room-received');
      assert.equal(receipt.proof, 'original-ay-kind3-target-sender-and-room-model');
      assert.equal(receipt.room, 'Lobby');
      assert.equal(receipt.from, 'alice');
      assert.equal(receipt.text, outbound);
      const inbound = `Java room ${peerTransport} ✓`;
      const delivery = messageFrom(client, 'bob', inbound, 'Lobby');
      void delivery.catch(() => {});
      peer.command('ROOM_SEND', 'Lobby', inbound);
      const sent = await peer.next('room-sent');
      assert.equal(sent.proof, 'original-kind3-builder-and-correlated-ack');
      const reply = await delivery;
      assert.equal(reply.kind, 3);
      assert.equal(reply.target, 'Lobby');
      assert.equal(reply.sender, 'bob');
      assert.equal(reply.content, 1);

      await client.leaveRoom('Lobby');
      await assert.rejects(client.sendRoomMessage('Lobby', 'Must not send after leave'), /Join the chat room/);
      peer.command('ROOM_PARTICIPANTS', 'Lobby', 'bob');
      assert.equal((await peer.next('room-participants')).names, 'bob');
      peer.command('ROOM_LEAVE', 'Lobby');
      const left = await peer.next('room-left');
      assert.equal(left.proof, 'original-704-ack-closed-tab-cleared-suppression');
      await client.logout();
      await peer.presence('alice', 99);
      await peer.logout();
      await writeFile(join(server.outputDirectory, 'interop-room-result.json'), JSON.stringify({
        passed: true, runtime: 'Node TypeScript core; not Expo device proof',
        typescriptTransport: transport, originalTransport: peerTransport,
        originalJarSHA256: 'dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c',
        room: 'Lobby', joinEvents, participants: participants.participants.map((entry) => entry.username),
        originalJoinProof: joined.proof, originalReceiveProof: receipt.proof,
        originalSendProof: sent.proof, originalLeaveProof: left.proof,
        originalParticipantsAfterTypeScriptLeave: ['bob'],
      }, null, 2) + '\n');
      t.diagnostic(`Original-client room byte captures, UI models and screenshots: ${server.outputDirectory}`);
    });
}

for (const transport of ['http', 'tcp'] as const) {
  test(`TypeScript ${transport} exchanges private messages with unchanged Java ${transport === 'http' ? 'tcp' : 'http'} client`,
    { skip: !enabled, timeout: 150000 }, async (t) => {
      const server = await startServer();
      t.after(() => server.close());
      const peerTransport = transport === 'http' ? 'tcp' : 'http';
      const peer = new OriginalPeer(server, peerTransport);
      t.after(() => peer.close());
      assert.equal((await peer.next('ready')).proof, 'original-login-roster-categories');

      const client = new Mig33Client(transport === 'http'
        ? new HTTPTransport(server.httpURL, { pollIntervalMs: 100 })
        : new TCPTransport(nodeConnector('127.0.0.1', server.tcpPort)));
      t.after(() => client.close());
      const receivedOpcodes: number[] = [];
      client.subscribe((event) => { if (event.type === 'packet') receivedOpcodes.push(event.packet.opcode); });
      await client.login({ username: 'alice', password: 'alice-demo' });
      assert.equal(client.state, 'online');
      const contacts = await client.syncContacts();
      assert.ok(contacts.some((contact) => contact.username === 'bob'), 'Original contact snapshot contains peer');
      for (const opcode of [201, 203, 402, 403]) assert.ok(receivedOpcodes.includes(opcode), `Missing original reply ${opcode}`);
      await peer.presence('alice', 1);

      const outbound = `Expo ${transport} ✓`;
      const inbound = `Java ${peerTransport} ✓`;
      peer.command('EXPECT', 'alice', outbound);
      await client.sendMessage('bob', outbound);
      const receipt = await peer.next('received');
      assert.equal(receipt.proof, 'original-ay-and-chat-model');
      assert.equal(receipt.text, outbound);
      const delivery = messageFrom(client, 'bob', inbound);
      void delivery.catch(() => {}); // Preserve failures while awaiting the Java acknowledgment.
      peer.command('SEND', 'alice', inbound);
      const sent = await peer.next('sent');
      assert.equal(sent.proof, 'original-builder-and-correlated-ack');
      const reply = await delivery;
      assert.equal(reply.target, 'alice');
      assert.equal(reply.kind, 1);
      assert.equal(reply.content, 1);
      await client.logout();
      assert.equal(client.state, 'closed');
      // Original bc.run sets n=true at logout offsets1268–1270. Its receive
      // dispatcher then destroys the MIDlet on ANY next packet (0–4,14713–14717),
      // so a queued offline404 can preempt the logout ACK. Observe the completed
      // online→offline roster transition before requesting Java logout; no sleep
      // or backend/transport modification is needed to serialize this harness.
      await peer.presence('alice', 99);
      await peer.logout();
      await writeFile(join(server.outputDirectory, 'interop-result.json'), JSON.stringify({
        passed: true, runtime: 'Node TypeScript core; not Expo device proof',
        typescriptTransport: transport, originalTransport: peerTransport,
        originalJarSHA256: 'dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c',
        login: [200, 201, 202, 203], contactCompletion: 403,
        originalReceiveProof: receipt.proof, originalSendProof: sent.proof,
        originalPresenceProof: 'original-404-and-roster-model', originalPresenceTransition: [1, 99],
        receivedOpcodes,
      }, null, 2) + '\n');
      t.diagnostic(`Original-client byte captures and logs: ${server.outputDirectory}`);
    });
}
