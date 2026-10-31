import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import test from 'node:test';
import { runInNewContext } from 'node:vm';

const require = createRequire(import.meta.url);
const packageRoot = dirname(require.resolve('react-native-tcp-socket/package.json'));
const packageRequire = createRequire(join(packageRoot, 'package.json'));

interface BridgeSocket {
  _id: number;
  on(event: string, listener: (...args: unknown[]) => void): unknown;
  destroy(): void;
}

/** Run the installed, patched JS bridge against an injected native event bus.
 * Only ESM import/export syntax is adapted; its runtime methods are unchanged.
 * This verifies event provenance, not either platform's TLS implementation. */
function bridge() {
  const bus = new EventEmitter();
  const connects: number[] = [];
  const destroyed: number[] = [];
  let nextId = 0;
  const bindings: Record<string, unknown> = {
    Buffer, setTimeout, clearTimeout,
    EventEmitter: packageRequire('eventemitter3'),
    nativeEventEmitter: {
      addListener(name: string, listener: (...args: unknown[]) => void) {
        bus.on(name, listener);
        return { remove: () => bus.removeListener(name, listener) };
      },
    },
    getNextId: () => nextId++,
    NativeModules: { TcpSockets: {
      connect: (id: number) => connects.push(id),
      startTLS: () => {},
      destroy: (id: number) => destroyed.push(id),
    } },
    Image: { resolveAssetSource: () => { throw new Error('No custom CA is used by this test.'); } },
  };
  function load(name: string): unknown {
    const source = readFileSync(join(packageRoot, 'src', name), 'utf8')
      .replace(/^import [^\n]+;\n/gm, '')
      .replace('export default class ', 'module.exports = class ')
      .replace('export default {', 'const defaultExport = {');
    const module = { exports: {} };
    runInNewContext(source, { ...bindings, module }, { filename: name, timeout: 1000 });
    return module.exports;
  }
  bindings.Socket = load('Socket.js');
  bindings.TLSSocket = load('TLSSocket.js');
  bindings.Server = class {};
  bindings.TLSServer = class {};
  const api = load('index.js') as { connectTLS: (options: object, ready: () => void) => BridgeSocket };
  return { api, bus, connects, destroyed };
}

const connection = { localAddress: '127.0.0.1', localPort: 50000, remoteAddress: '127.0.0.1', remotePort: 9120, remoteFamily: 'IPv4' };

test('installed TLS bridge never synthesizes secureConnect from an ordinary connect event', () => {
  const f = bridge();
  let ready = 0;
  const socket = f.api.connectTLS({ host: 'localhost', port: 9120 }, () => { ready++; });
  assert.deepEqual(f.connects, [socket._id]);
  f.bus.emit('connect', { id: socket._id, connection });
  assert.equal(ready, 0);
  f.bus.emit('secureConnect', { id: socket._id + 99 });
  assert.equal(ready, 0);
  f.bus.emit('secureConnect', { id: socket._id });
  assert.equal(ready, 1);
  f.bus.emit('close', { id: socket._id, error: false });
  assert.equal(f.bus.listenerCount('secureConnect'), 0);
});

test('native TLS failure and cancellation cannot be followed by a usable secureConnect event', () => {
  for (const failure of ['certificate', 'cancel']) {
    const f = bridge();
    let ready = 0;
    const errors: unknown[] = [];
    const socket = f.api.connectTLS({ host: 'localhost', port: 9120 }, () => { ready++; });
    socket.on('error', (error) => errors.push(error));
    if (failure === 'certificate') f.bus.emit('error', { id: socket._id, error: 'certificate rejected' });
    else socket.destroy();
    f.bus.emit('secureConnect', { id: socket._id });
    assert.equal(ready, 0);
    assert.equal(errors.length, failure === 'certificate' ? 1 : 0);
    assert.ok(f.destroyed.includes(socket._id));
    f.bus.emit('close', { id: socket._id, error: failure === 'certificate' });
    assert.equal(f.bus.listenerCount('secureConnect'), 0);
  }
});
