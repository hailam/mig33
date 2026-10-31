import { decodeFrames, encodeFrame, MAX_FRAME_SIZE, type Packet } from '../protocol/index.ts';
import { nativeHTTP } from '../generated/http.ts';
import { asError, type PacketTransport, type TransportEvents } from './types.ts';

export class HTTPTransportError extends Error {
  readonly status: number;
  constructor(status: number) {
    super(`HTTP gateway returned ${status}`);
    this.name = 'HTTPTransportError';
    this.status = status;
  }
}

export type HTTPFetch = (url: string, init: {
  method: 'POST' | 'GET'; headers: Record<string, string>; body?: ArrayBuffer;
  signal: AbortSignal; redirect: 'error'; credentials: 'omit';
}) => Promise<Pick<Response, 'status' | 'headers' | 'arrayBuffer' | 'url' | 'redirected'>>;

interface HTTPOptions {
  fetch?: HTTPFetch;
  /** App policy; plain HTTP remains available to original compatibility tests. */
  requireHTTPS?: boolean;
  pollIntervalMs?: number;
  timeoutMs?: number;
}

// Original u transport's base-path exceptions, including its unused100 entry.
const basePathOpcodes = new Set<number>(nativeHTTP.basePathOpcodes);

export class HTTPTransport implements PacketTransport {
  readonly kind = 'http' as const;
  private readonly base: string;
  private readonly fetcher: HTTPFetch;
  private readonly interval: number;
  private readonly timeout: number;
  private events?: TransportEvents;
  private session = '';
  private open = false;
  private polling = false;
  private timer?: ReturnType<typeof setTimeout>;
  private controller?: AbortController;
  private queue: Promise<void> = Promise.resolve();

  constructor(baseURL: string, options: HTTPOptions = {}) {
    const url = new URL(baseURL);
    if (options.requireHTTPS && url.protocol !== 'https:') throw new Error('This app requires an HTTPS gateway.');
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password ||
        url.search || url.hash || url.pathname !== '/') {
      throw new Error('Use an HTTP gateway origin, such as http://127.0.0.1:8080');
    }
    this.base = url.href;
    this.fetcher = options.fetch ?? globalThis.fetch;
    this.interval = options.pollIntervalMs ?? 5000;
    this.timeout = options.timeoutMs ?? 30000;
    if (this.interval < 1 || this.timeout < 1) throw new RangeError('HTTP timings must be positive');
  }

  async connect(events: TransportEvents): Promise<void> {
    if (this.open || this.events) throw new Error('Create a new transport for a new connection');
    this.events = events;
    this.open = true;
  }

  setSession(session: string): void { this.session = session; }

  activate(): void {
    if (!this.session || !this.open) throw new Error('HTTP polling needs an authenticated session');
    if (this.polling) return;
    this.polling = true;
    this.schedulePoll();
  }

  send(packet: Packet): Promise<void> {
    // Resolve the SID at execution time: a preceding201 can assign it.
    return this.enqueue(() => this.exchange(packet));
  }

  close(): void {
    this.open = false;
    this.polling = false;
    clearTimeout(this.timer);
    this.controller?.abort();
    this.session = '';
  }

  private enqueue(work: () => Promise<void>): Promise<void> {
    const next = this.queue.then(async () => {
      if (!this.open) throw new Error('HTTP transport is closed');
      try {
        await work();
      } catch (cause) {
        const error = asError(cause);
        if (this.open) {
          this.close();
          this.events?.closed(error);
        }
        throw error;
      }
    });
    // Rejections are returned to the caller without poisoning the queue or
    // producing an unhandled rejection during cancellation.
    this.queue = next.catch(() => {});
    return next;
  }

  private schedulePoll(): void {
    if (!this.open || !this.polling) return;
    this.timer = setTimeout(() => {
      void this.enqueue(() => this.exchange()).then(
        () => this.schedulePoll(),
        () => {}, // enqueue reports the failure and closes the transport.
      );
    }, this.interval);
  }

  private async exchange(packet?: Packet): Promise<void> {
    const sessionPath = !packet || !basePathOpcodes.has(packet.opcode);
    const path = sessionPath && this.session
      ? nativeHTTP.sessionPath.replace('{sid}', encodeURIComponent(this.session))
      : nativeHTTP.rootPath;
    const url = new URL(path, this.base).href;
    const headers: Record<string, string> = {
      'mig33': '4.62', 'Accept': nativeHTTP.responseContentType, 'Pragma': 'no-cache',
    };
    if (this.session) headers.sid = this.session;
    let body: ArrayBuffer | undefined;
    if (packet) {
      headers['Content-Type'] = nativeHTTP.requestContentType;
      const bytes = encodeFrame(packet);
      body = new ArrayBuffer(bytes.length);
      new Uint8Array(body).set(bytes);
    }
    const controller = new AbortController();
    this.controller = controller;
    const timeout = setTimeout(() => controller.abort(), this.timeout);
    try {
      const response = await this.fetcher(url, {
        method: packet ? 'POST' : 'GET', headers, body,
        signal: controller.signal, redirect: 'error', credentials: 'omit',
      });
      // Defense in depth only: the injected fetch must reject redirects before
      // sending any new request. A post-response check cannot prevent leakage.
      if (response.redirected || (response.url && response.url !== url)) throw new Error('HTTP gateway redirects are not allowed.');
      if (response.status !== 200) throw new HTTPTransportError(response.status);
      // Local gateway returns at most256 frames per response; retain a total
      // bound as well as the codec's individual1MiB frame bound.
      const declared = response.headers.get('Content-Length');
      const maximum = MAX_FRAME_SIZE * 256;
      if (declared !== null && Number(declared) > maximum) throw new Error('HTTP response exceeds packet batch limit');
      const bytes = new Uint8Array(await response.arrayBuffer());
      if (bytes.length > maximum) throw new Error('HTTP response exceeds packet batch limit');
      const packets = decodeFrames(bytes);
      if (this.open && packets.length) this.events?.packets(packets);
    } finally {
      clearTimeout(timeout);
      if (this.controller === controller) this.controller = undefined;
    }
  }
}
