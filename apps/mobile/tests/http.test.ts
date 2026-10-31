import assert from 'node:assert/strict';
import { setImmediate as tick } from 'node:timers/promises';
import test from 'node:test';
import { HTTPTransport, HTTPTransportError } from '../src/mig33/transport/http.ts';
import { Packet, encodeFrame, decodeFrames } from '../src/mig33/protocol/index.ts';
import type { TransportEvents } from '../src/mig33/transport/types.ts';

interface FetchCall { url: string; init: RequestInit; }
function response(...packets: Packet[]): Response {
  return new Response(Buffer.concat(packets.map((packet) => encodeFrame(packet))), {
    status: 200, headers: { 'Content-Type': 'image/gif' },
  });
}
function events(): { handlers: TransportEvents; packets: Packet[]; errors: Array<Error | undefined> } {
  const packets: Packet[] = [];
  const errors: Array<Error | undefined> = [];
  return { packets, errors, handlers: {
    packets: (received) => packets.push(...received), closed: (error) => errors.push(error),
  } };
}
function fetcher(handler: (call: FetchCall) => Response | Promise<Response>): typeof fetch {
  return (async (input: string | URL | Request, init: RequestInit = {}) => handler({ url: String(input), init })) as typeof fetch;
}

test('HTTP preserves binary POSTs, original MIME, SID routing exceptions and HTTP 200 packet errors', async () => {
  const calls: FetchCall[] = [];
  const sink = events();
  const transport = new HTTPTransport('http://127.0.0.1:8080', { fetch: fetcher((call) => {
    calls.push(call);
    return response(new Packet(0, calls.length).setText(2, 'Application error'));
  }) });
  await transport.connect(sink.handlers);
  transport.setSession('abc123');
  try {
    for (const opcode of [200, 100, 2, 904, 202, 500]) await transport.send(new Packet(opcode, opcode));
    assert.deepEqual(calls.map((call) => call.url), [
      'http://127.0.0.1:8080/', 'http://127.0.0.1:8080/', 'http://127.0.0.1:8080/',
      'http://127.0.0.1:8080/', 'http://127.0.0.1:8080/abc123', 'http://127.0.0.1:8080/abc123',
    ]);
    for (const call of calls) {
      assert.equal(call.init.method, 'POST');
      assert.equal(new Headers(call.init.headers).get('content-type'), 'image/gif');
      assert.equal(new Headers(call.init.headers).get('sid'), 'abc123');
      assert.equal(call.init.redirect, 'error');
      const packet = decodeFrames(new Uint8Array(call.init.body as ArrayBuffer))[0];
      assert.equal(packet.transaction, packet.opcode);
    }
    assert.equal(sink.packets.length, 6);
    assert.equal(sink.errors.length, 0, 'opcode0 inside HTTP200 belongs to the client session');
  } finally { transport.close(); }
});

test('HTTP serializes exchanges and resolves session routing when the queued exchange executes', async () => {
  const calls: FetchCall[] = [];
  let finishFirst!: (response: Response) => void;
  const firstResponse = new Promise<Response>((resolve) => { finishFirst = resolve; });
  const transport = new HTTPTransport('http://localhost:8080/', { fetch: fetcher((call) => {
    calls.push(call);
    return calls.length === 1 ? firstResponse : response();
  }) });
  await transport.connect({
    packets: (packets) => { for (const packet of packets) if (packet.opcode === 201) transport.setSession(packet.text(2)!); },
    closed: () => {},
  });
  try {
    const first = transport.send(new Packet(200, 1));
    const second = transport.send(new Packet(202, 2));
    await tick();
    assert.equal(calls.length, 1);
    finishFirst(response(new Packet(201, 1).setText(1, 'challenge').setText(2, 'assigned')));
    await Promise.all([first, second]);
    assert.equal(calls[1].url, 'http://localhost:8080/assigned');
  } finally { transport.close(); }
});

test('HTTP parses several frames and polls the session path with empty HTTP 200 bodies', async () => {
  let polled!: () => void;
  const poll = new Promise<void>((resolve) => { polled = resolve; });
  const calls: FetchCall[] = [];
  const sink = events();
  const transport = new HTTPTransport('http://localhost:8080/', {
    pollIntervalMs: 1, fetch: fetcher((call) => {
      calls.push(call);
      if (call.init.method === 'GET') { polled(); return response(); }
      return response(new Packet(1, 1), new Packet(500, 0).setText(8, 'push'));
    }),
  });
  await transport.connect(sink.handlers);
  try {
    await transport.send(new Packet(500, 1).setText(8, 'request'));
    assert.deepEqual(sink.packets.map((packet) => packet.opcode), [1, 500]);
    transport.setSession('session');
    transport.activate();
    transport.activate();
    await poll;
    transport.close();
    await tick();
    assert.equal(calls.filter((call) => call.init.method === 'GET').length, 1);
    assert.equal(calls.at(-1)!.url, 'http://localhost:8080/session');
    assert.equal(calls.at(-1)!.init.body, undefined);
    assert.equal(sink.errors.length, 0);
  } finally { transport.close(); }
});

test('HTTP failures close once and queued exchanges cannot leak after failure', async () => {
  const sink = events();
  let calls = 0;
  const transport = new HTTPTransport('http://localhost:8080/', { fetch: fetcher(() => {
    calls++;
    return new Response('expired', { status: 401 });
  }) });
  await transport.connect(sink.handlers);
  const first = transport.send(new Packet(500, 1));
  const second = transport.send(new Packet(500, 2));
  await Promise.all([
    assert.rejects(first, (error: unknown) => error instanceof HTTPTransportError && error.status === 401),
    assert.rejects(second, /closed/i),
  ]);
  assert.equal(calls, 1);
  assert.equal(sink.errors.length, 1);
});

test('HTTP close aborts in-flight fetch and rejects queued sends without a remote-close event', async () => {
  const sink = events();
  let calls = 0;
  let signal!: AbortSignal;
  const transport = new HTTPTransport('http://localhost:8080/', {
    fetch: fetcher((call) => {
      calls++;
      signal = call.init.signal!;
      return new Promise<Response>((_resolve, reject) => {
        signal.addEventListener('abort', () => reject(new Error('Aborted')), { once: true });
      });
    }),
  });
  await transport.connect(sink.handlers);
  const first = transport.send(new Packet(500, 1));
  const second = transport.send(new Packet(500, 2));
  const failures = [assert.rejects(first, /Aborted/), assert.rejects(second, /closed/i)];
  await tick();
  transport.close();
  await Promise.all(failures);
  assert.equal(signal.aborted, true);
  assert.equal(calls, 1);
  assert.equal(sink.errors.length, 0);
});

test('HTTP rejects malformed binary responses and unexpected redirects', async () => {
  for (const makeResponse of [
    () => new Response(Uint8Array.of(3, 0, 1, 0, 1, 0, 0, 0, 0), { status: 200 }),
    () => { throw new TypeError('redirect rejected'); },
  ]) {
    const sink = events();
    const transport = new HTTPTransport('http://localhost:8080/', { fetch: fetcher(makeResponse) });
    await transport.connect(sink.handlers);
    await assert.rejects(transport.send(new Packet(2, 1)));
    assert.equal(sink.errors.length, 1);
    assert.equal(sink.packets.length, 0);
  }
});

test('HTTP gateway configuration rejects path, credentials and query-based origins', () => {
  for (const url of ['ws://localhost', 'http://user:pass@localhost', 'http://localhost/path', 'http://localhost/?x=1']) {
    assert.throws(() => new HTTPTransport(url), /origin/);
  }
  assert.throws(() => new HTTPTransport('http://localhost', { pollIntervalMs: 0 }), RangeError);
});
