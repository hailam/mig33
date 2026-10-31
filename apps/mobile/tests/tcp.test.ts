import assert from 'node:assert/strict';
import { setImmediate as tick } from 'node:timers/promises';
import test from 'node:test';
import { TCPTransport, type ByteSocket, type SocketConnector, type SocketEvents } from '../src/mig33/transport/tcp.ts';
import { Packet, ProtocolError, encodeFrame, decodeFrame } from '../src/mig33/protocol/index.ts';

function fixture() {
  let socketEvents!: SocketEvents;
  const writes: Uint8Array[] = [];
  let closes = 0;
  let write: (bytes: Uint8Array) => Promise<void> = async (bytes) => { writes.push(bytes); };
  const socket: ByteSocket = {
    write: (bytes) => write(bytes),
    close: () => { closes++; socketEvents.close(); },
  };
  const connector: SocketConnector = async (events) => { socketEvents = events; return socket; };
  const transport = new TCPTransport(connector);
  const received: Packet[] = [];
  const errors: Array<Error | undefined> = [];
  return {
    transport, received, errors, writes, socket,
    connect: () => transport.connect({ packets: (packets) => received.push(...packets), closed: (error) => errors.push(error) }),
    get socketEvents() { return socketEvents; },
    get closes() { return closes; },
    writeWith(handler: (bytes: Uint8Array) => Promise<void>) { write = handler; },
  };
}

test('TCP accepts arbitrary fragments and multiple packets without exposing incomplete frames', async () => {
  const f = fixture();
  await f.connect();
  try {
    const first = encodeFrame(new Packet(201, 1).setText(1, 'challenge').setText(2, 'session'));
    const second = encodeFrame(new Packet(500, 0).setText(8, 'Hello 😀'));
    const combined = Uint8Array.from(Buffer.concat([first, second]));
    f.socketEvents.data(combined.subarray(0, 8));
    assert.equal(f.received.length, 0);
    f.socketEvents.data(combined.subarray(8, first.length + 3));
    assert.equal(f.received.length, 1);
    f.socketEvents.data(combined.subarray(first.length + 3));
    assert.deepEqual(f.received.map((packet) => packet.opcode), [201, 500]);
    assert.equal(f.received[1].text(8), 'Hello 😀');
    assert.equal(f.errors.length, 0);
  } finally { f.transport.close(); }
});

test('TCP waits for each write before starting the next and snapshots packet bytes', async () => {
  const f = fixture();
  let finish!: () => void;
  const firstWrite = new Promise<void>((resolve) => { finish = resolve; });
  f.writeWith(async (bytes) => {
    f.writes.push(bytes);
    if (f.writes.length === 1) await firstWrite;
  });
  await f.connect();
  try {
    const packet = new Packet(500, 1).setText(8, 'first');
    const first = f.transport.send(packet);
    packet.setText(8, 'changed after send');
    const second = f.transport.send(new Packet(500, 2).setText(8, 'second'));
    await tick();
    assert.equal(f.writes.length, 1);
    assert.equal(decodeFrame(f.writes[0]).text(8), 'first');
    finish();
    await Promise.all([first, second]);
    assert.deepEqual(f.writes.map((bytes) => decodeFrame(bytes).transaction), [1, 2]);
  } finally { f.transport.close(); }
});

test('TCP write failure closes once and prevents queued writes', async () => {
  const f = fixture();
  f.writeWith(async () => { throw new Error('Socket write failed'); });
  await f.connect();
  const first = f.transport.send(new Packet(500, 1));
  const second = f.transport.send(new Packet(500, 2));
  await Promise.all([assert.rejects(first, /write failed/), assert.rejects(second, /closed/i)]);
  assert.equal(f.closes, 1);
  assert.equal(f.errors.length, 1);
});

test('TCP EOF during a frame reports truncation; complete EOF reports remote closure', async () => {
  for (const partial of [false, true]) {
    const f = fixture();
    await f.connect();
    const bytes = encodeFrame(new Packet(500, 1).setText(8, 'message'));
    f.socketEvents.data(partial ? bytes.subarray(0, bytes.length - 1) : bytes);
    f.socketEvents.close();
    assert.equal(f.errors.length, 1);
    if (partial) {
      assert.ok(f.errors[0] instanceof ProtocolError);
      assert.equal((f.errors[0] as ProtocolError).code, 'truncated');
      assert.equal(f.received.length, 0);
    } else {
      assert.match(f.errors[0]!.message, /closed/);
      assert.equal(f.received.length, 1);
    }
    f.socketEvents.close();
    f.socketEvents.error(new Error('Late error'));
    assert.equal(f.errors.length, 1);
  }
});

test('TCP malformed data closes the connection and ignores later input', async () => {
  const f = fixture();
  await f.connect();
  f.socketEvents.data(Uint8Array.of(3, 0, 1, 0, 1, 0, 0, 0, 0));
  assert.equal(f.errors.length, 1);
  assert.equal(f.closes, 1);
  f.socketEvents.data(encodeFrame(new Packet(3, 1)));
  assert.equal(f.received.length, 0);
});

test('TCP connect cancellation closes a connector that completes after cancellation', async () => {
  let finish!: (socket: ByteSocket) => void;
  let signal!: AbortSignal;
  let closes = 0;
  const connector: SocketConnector = async (_events, abortSignal) => {
    signal = abortSignal;
    return new Promise<ByteSocket>((resolve) => { finish = resolve; });
  };
  const transport = new TCPTransport(connector);
  const connecting = transport.connect({ packets: () => {}, closed: () => {} });
  const cancelled = assert.rejects(connecting, /cancelled/);
  transport.close();
  assert.equal(signal.aborted, true);
  finish({ write: async () => {}, close: () => { closes++; } });
  await cancelled;
  assert.equal(closes, 1);
});
