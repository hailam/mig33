import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import { setImmediate as tick } from 'node:timers/promises';
import test from 'node:test';
import { nativeSocketConnector, type NativeByteSocket } from '../src/mig33/transport/native-socket.ts';

class FakeSocket extends EventEmitter implements NativeByteSocket {
  destroys = 0;
  settings: string[] = [];
  writes: { bytes: Uint8Array; finish: (error?: Error) => void }[] = [];
  destroy() { this.destroys++; }
  setNoDelay(enabled: boolean) { this.settings.push(`noDelay:${enabled}`); }
  setKeepAlive(enabled: boolean) { this.settings.push(`keepAlive:${enabled}`); }
  write(bytes: Uint8Array, _encoding: undefined, finish: (error?: Error) => void) {
    this.writes.push({ bytes, finish });
    return false; // An enqueued write is not a completed native write.
  }
}

function fixture(secure = true) {
  const socket = new FakeSocket();
  const abort = new AbortController();
  const received: Uint8Array[] = [];
  const errors: Error[] = [];
  let closes = 0;
  let starts = 0;
  let creates = 0;
  let start = () => { starts++; };
  const connector = nativeSocketConnector(() => {
    creates++;
    return { socket, start: () => start() };
  }, secure);
  return {
    socket, abort, received, errors,
    connect: () => connector({ data: (bytes) => received.push(bytes), error: (error) => errors.push(error), close: () => { closes++; } }, abort.signal),
    startWith(next: () => void) { start = next; },
    get closes() { return closes; },
    get starts() { return starts; },
    get creates() { return creates; },
  };
}

test('TLS ignores ordinary connect and resolves only the native authenticated handshake event', async () => {
  const f = fixture();
  let ready = false;
  const connecting = f.connect().then((socket) => { ready = true; return socket; });
  assert.equal(f.starts, 1);
  f.socket.emit('connect');
  await tick();
  assert.equal(ready, false);
  assert.deepEqual(f.socket.settings, []);
  f.socket.emit('secureConnect');
  const socket = await connecting;
  assert.deepEqual(f.socket.settings, ['noDelay:true', 'keepAlive:true']);
  f.socket.emit('secureConnect');
  assert.equal(f.socket.settings.length, 2);
  socket.close();
});

test('TLS rejects bytes before peer authentication without delivering protocol data', async () => {
  const f = fixture();
  const rejected = assert.rejects(f.connect(), /before peer authentication/);
  f.socket.emit('connect');
  f.socket.emit('data', Uint8Array.of(1, 2, 3));
  await rejected;
  f.socket.emit('secureConnect');
  assert.equal(f.received.length, 0);
  assert.equal(f.socket.destroys, 1);
  assert.equal(f.closes, 0);
});

test('TLS certificate/hostname rejection fails before readiness', async () => {
  const f = fixture();
  const rejected = assert.rejects(f.connect(), /hostname mismatch/);
  f.socket.emit('error', new Error('TLS certificate hostname mismatch'));
  await rejected;
  assert.equal(f.socket.destroys, 1);
  assert.equal(f.received.length, 0);
});

test('cancellation before construction or while handshaking never yields a usable socket', async () => {
  for (const beforeStart of [true, false]) {
    const f = fixture();
    if (beforeStart) f.abort.abort();
    const rejected = assert.rejects(f.connect(), /cancelled/);
    f.abort.abort();
    await rejected;
    assert.equal(f.creates, beforeStart ? 0 : 1);
    assert.equal(f.socket.destroys, beforeStart ? 0 : 1);
    f.socket.emit('secureConnect');
    assert.equal(f.socket.settings.length, 0);
  }
});

test('listeners are installed before synchronous start callbacks and native failures', async () => {
  const f = fixture();
  f.startWith(() => { f.socket.emit('secureConnect'); });
  (await f.connect()).close();
  const broken = fixture();
  broken.startWith(() => { throw new Error('Bridge unavailable'); });
  await assert.rejects(broken.connect(), /Bridge unavailable/);
  assert.equal(broken.socket.destroys, 1);
});

test('authenticated binary bytes are copied and writes await native completion', async () => {
  const f = fixture();
  const connecting = f.connect();
  f.socket.emit('secureConnect');
  const socket = await connecting;
  const bytes = Uint8Array.of(4, 5, 6);
  f.socket.emit('data', bytes);
  bytes[0] = 99;
  assert.deepEqual(f.received[0], Uint8Array.of(4, 5, 6));
  let completed = false;
  const write = socket.write(bytes).then(() => { completed = true; });
  await tick();
  assert.equal(completed, false);
  f.socket.writes[0].finish();
  await write;
  assert.equal(completed, true);
  socket.close();
});

test('closing rejects pending writes and late native completion cannot report success', async () => {
  const f = fixture();
  const connecting = f.connect();
  f.socket.emit('secureConnect');
  const socket = await connecting;
  const rejected = assert.rejects(socket.write(Uint8Array.of(1)), /cancelled/);
  f.abort.abort();
  await rejected;
  f.socket.writes[0].finish();
  f.socket.writes[0].finish(new Error('Late failure'));
  socket.close();
  assert.equal(f.closes, 1);
  assert.equal(f.errors.length, 0);
  assert.equal(f.socket.destroys, 1);
  await assert.rejects(socket.write(Uint8Array.of(2)), /closed/);
});

test('native write failures close once and propagate their original error', async () => {
  const f = fixture();
  const connecting = f.connect();
  f.socket.emit('secureConnect');
  const socket = await connecting;
  const rejected = assert.rejects(socket.write(Uint8Array.of(1)), /send failed/);
  f.socket.writes[0].finish(new Error('Native send failed'));
  await rejected;
  assert.equal(f.errors.length, 1);
  assert.equal(f.closes, 1);
  assert.equal(f.socket.destroys, 1);
});

test('TLS refuses a text-encoded socket and remote EOF settles an unfinished handshake', async () => {
  const f = fixture();
  const connecting = f.connect();
  f.socket.emit('secureConnect');
  await connecting;
  f.socket.emit('data', 'plaintext');
  assert.match(f.errors[0].message, /text instead of binary/);
  assert.equal(f.received.length, 0);
  const unfinished = fixture();
  const rejected = assert.rejects(unfinished.connect(), /closed/);
  unfinished.socket.emit('close');
  await rejected;
  assert.equal(unfinished.socket.destroys, 0);
});

test('original plaintext compatibility connector still uses connect readiness', async () => {
  const f = fixture(false);
  const connecting = f.connect();
  f.socket.emit('connect');
  const socket = await connecting;
  f.socket.emit('data', Uint8Array.of(1));
  assert.equal(f.received.length, 1);
  socket.close();
});
