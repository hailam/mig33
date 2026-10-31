import type { RawPacket } from '../generated/contracts.ts';
import { decodeUTF8, encodeUTF8 } from './utf8.ts';

export const HEADER_SIZE = 9;
export const FIELD_HEADER_SIZE = 6;
// Match the reconstruction's safe limit; this is not a recovered server limit.
export const MAX_FRAME_SIZE = 1 << 20;
export const MAX_BODY_SIZE = MAX_FRAME_SIZE - HEADER_SIZE;
const MARKER = 2;
const MIN_LONG = -(1n << 63n);
const MAX_LONG = (1n << 63n) - 1n;

export type ProtocolErrorCode = 'invalid-marker' | 'invalid-length' | 'too-large' | 'truncated' | 'decoder-failed';

export class ProtocolError extends Error {
  readonly code: ProtocolErrorCode;

  constructor(code: ProtocolErrorCode, message: string) {
    super(message);
    this.name = 'ProtocolError';
    this.code = code;
  }
}

function integer(value: number, min: number, max: number, label: string): void {
  if (!Number.isInteger(value) || value < min || value > max) {
    throw new RangeError(`${label} must be an integer from ${min} to ${max}`);
  }
}

function short(value: number, label: string): void {
  integer(value, -32768, 32767, label);
}

function limit(maxFrameSize: number): void {
  integer(maxFrameSize, HEADER_SIZE, MAX_FRAME_SIZE, 'Frame limit');
}

function view(bytes: Uint8Array): DataView {
  return new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
}

/**
 * Raw fields have no transmitted type discriminator. Accessors interpret
 * prefixes of the same bytes and do not establish per-message requiredness.
 */
export class Packet implements RawPacket {
  readonly opcode: number;
  readonly transaction: number;
  readonly fields: Map<number, Uint8Array>;

  constructor(opcode: number, transaction: number, fields?: ReadonlyMap<number, Uint8Array>) {
    short(opcode, 'Opcode');
    short(transaction, 'Transaction');
    this.opcode = opcode;
    this.transaction = transaction;
    this.fields = new Map();
    for (const [tag, bytes] of fields ?? []) this.setBytes(tag, bytes);
  }

  setBytes(tag: number, bytes: Uint8Array): this {
    short(tag, 'Field tag');
    this.fields.set(tag, bytes.slice());
    return this;
  }

  setText(tag: number, text: string): this {
    return this.setBytes(tag, encodeUTF8(text));
  }

  setByte(tag: number, value: number): this {
    integer(value, -128, 127, 'Byte');
    return this.setBytes(tag, Uint8Array.of(value & 0xff));
  }

  setShort(tag: number, value: number): this {
    short(value, 'Short');
    const bytes = new Uint8Array(2);
    view(bytes).setInt16(0, value, false);
    return this.setBytes(tag, bytes);
  }

  setInt(tag: number, value: number): this {
    integer(value, -2147483648, 2147483647, 'Int');
    const bytes = new Uint8Array(4);
    view(bytes).setInt32(0, value, false);
    return this.setBytes(tag, bytes);
  }

  setLong(tag: number, value: bigint): this {
    if (typeof value !== 'bigint' || value < MIN_LONG || value > MAX_LONG) {
      throw new RangeError('Long must be a signed 64-bit bigint');
    }
    const bytes = new Uint8Array(8);
    let bits = BigInt.asUintN(64, value);
    for (let index = 7; index >= 0; index--) {
      bytes[index] = Number(bits & 0xffn);
      bits >>= 8n;
    }
    return this.setBytes(tag, bytes);
  }

  /** Original wire encoding: UTF-8 values separated by NUL, no final separator. */
  setStringArray(tag: number, values: readonly string[]): this {
    return this.setText(tag, values.join('\0'));
  }

  bytes(tag: number): Uint8Array | undefined {
    return this.fields.get(tag)?.slice();
  }

  text(tag: number): string | undefined {
    const bytes = this.fields.get(tag);
    return bytes === undefined ? undefined : decodeUTF8(bytes);
  }

  byte(tag: number): number | undefined {
    const bytes = this.fields.get(tag);
    // The original empty-byte getter throws; expose safe absence here while
    // preserving the present-empty field in fields/bytes().
    return bytes && bytes.length >= 1 ? (bytes[0] << 24) >> 24 : undefined;
  }

  short(tag: number): number | undefined {
    const bytes = this.fields.get(tag);
    return bytes && bytes.length >= 2 ? view(bytes).getInt16(0, false) : undefined;
  }

  int(tag: number): number | undefined {
    const bytes = this.fields.get(tag);
    return bytes && bytes.length >= 4 ? view(bytes).getInt32(0, false) : undefined;
  }

  long(tag: number): bigint | undefined {
    const bytes = this.fields.get(tag);
    if (bytes === undefined || bytes.length < 8) return undefined;
    let bits = 0n;
    for (let index = 0; index < 8; index++) bits = (bits << 8n) | BigInt(bytes[index]);
    return BigInt.asIntN(64, bits);
  }

  /**
   * Original 4.62 raw-field TA accessor, including its retained-length loop
   * condition, which can omit the final element. It is not a normal split().
   * Evidence: original bj.j:()[Ljava/lang/String; offsets34..136 and accessor
   * probes in research/applets/v4/docs/data/protocol-audit-transports.json.
   */
  stringArray(tag: number): string[] | undefined {
    const bytes = this.fields.get(tag);
    if (bytes === undefined) return undefined;
    const values: string[] = [];
    let start = 0;
    let length = 0;
    while (start + length < bytes.length) {
      if (bytes[start] !== 0) {
        length = 0;
        while (start + length < bytes.length && bytes[start + length] !== 0) length++;
        values.push(decodeUTF8(bytes.subarray(start, start + length)));
      }
      start += length + 1;
    }
    return values;
  }
}

export function createPacket(opcode: number, transaction: number): Packet {
  return new Packet(opcode, transaction);
}

export interface EncodeOptions {
  fieldOrder?: 'sorted' | 'insertion';
  maxFrameSize?: number;
}

/** Sorted tags match the Go encoder; insertion order permits fixture reproduction. */
export function encodeFrame(packet: RawPacket, options: EncodeOptions = {}): Uint8Array {
  const maxFrameSize = options.maxFrameSize ?? MAX_FRAME_SIZE;
  limit(maxFrameSize);
  short(packet.opcode, 'Opcode');
  short(packet.transaction, 'Transaction');
  const fields = [...packet.fields.entries()];
  if (options.fieldOrder !== 'insertion') fields.sort(([a], [b]) => a - b);
  let frameSize = HEADER_SIZE;
  for (const [tag, bytes] of fields) {
    short(tag, 'Field tag');
    if (!(bytes instanceof Uint8Array)) throw new TypeError('Field payload must be Uint8Array');
    frameSize += FIELD_HEADER_SIZE + bytes.length;
    if (frameSize > maxFrameSize) throw new ProtocolError('too-large', 'Frame exceeds the configured limit');
  }
  const frame = new Uint8Array(frameSize);
  const data = view(frame);
  frame[0] = MARKER;
  data.setInt16(1, packet.opcode, false);
  data.setInt16(3, packet.transaction, false);
  data.setInt32(5, frameSize - HEADER_SIZE, false);
  let offset = HEADER_SIZE;
  for (const [tag, bytes] of fields) {
    data.setInt16(offset, tag, false);
    data.setInt32(offset + 2, bytes.length, false);
    frame.set(bytes, offset + FIELD_HEADER_SIZE);
    offset += FIELD_HEADER_SIZE + bytes.length;
  }
  return frame;
}

function declaredFrameSize(header: Uint8Array, maxFrameSize: number): number {
  if (header[0] !== MARKER) throw new ProtocolError('invalid-marker', 'Frame marker must be 0x02');
  const bodySize = view(header).getInt32(5, false);
  if (bodySize < 0) throw new ProtocolError('invalid-length', 'Negative frame body length');
  if (bodySize > maxFrameSize - HEADER_SIZE) {
    throw new ProtocolError('too-large', 'Frame exceeds the configured limit');
  }
  return HEADER_SIZE + bodySize;
}

/** Decode exactly one complete frame; any truncation or trailing bytes fail. */
export function decodeFrame(bytes: Uint8Array, maxFrameSize = MAX_FRAME_SIZE): Packet {
  limit(maxFrameSize);
  if (bytes.length < HEADER_SIZE) throw new ProtocolError('truncated', 'Truncated frame header');
  const frameSize = declaredFrameSize(bytes, maxFrameSize);
  if (frameSize > bytes.length) throw new ProtocolError('truncated', 'Truncated frame body');
  if (frameSize !== bytes.length) throw new ProtocolError('invalid-length', 'Trailing bytes after one frame');
  const data = view(bytes);
  const packet = new Packet(data.getInt16(1, false), data.getInt16(3, false));
  for (let offset = HEADER_SIZE; offset < frameSize;) {
    if (offset + FIELD_HEADER_SIZE > frameSize) {
      throw new ProtocolError('invalid-length', 'Truncated field header inside frame');
    }
    const tag = data.getInt16(offset, false);
    const length = data.getInt32(offset + 2, false);
    offset += FIELD_HEADER_SIZE;
    if (length < 0 || length > frameSize - offset) {
      throw new ProtocolError('invalid-length', 'Field extends beyond its frame body');
    }
    packet.setBytes(tag, bytes.subarray(offset, offset + length));
    offset += length;
  }
  return packet;
}

/**
 * Incremental TCP framing. Retained bytes never exceed maxFrameSize, even for
 * fragmented headers or a chunk containing many frames. Inputs are copied;
 * packets never retain the network callback's potentially reused backing array.
 * A malformed stream poisons the decoder until reset() for a new connection.
 */
export class FrameDecoder {
  private readonly header = new Uint8Array(HEADER_SIZE);
  private headerLength = 0;
  private frame: Uint8Array | undefined;
  private frameLength = 0;
  private failed = false;
  private readonly maxFrameSize: number;

  constructor(maxFrameSize = MAX_FRAME_SIZE) {
    limit(maxFrameSize);
    this.maxFrameSize = maxFrameSize;
  }

  get bufferedBytes(): number {
    return this.frame === undefined ? this.headerLength : this.frameLength;
  }

  push(chunk: Uint8Array): Packet[] {
    if (this.failed) throw new ProtocolError('decoder-failed', 'Reset decoder after a malformed stream');
    const packets: Packet[] = [];
    let offset = 0;
    try {
      while (offset < chunk.length) {
        if (this.frame === undefined) {
          const count = Math.min(HEADER_SIZE - this.headerLength, chunk.length - offset);
          this.header.set(chunk.subarray(offset, offset + count), this.headerLength);
          this.headerLength += count;
          offset += count;
          if (this.headerLength < HEADER_SIZE) continue;
          const size = declaredFrameSize(this.header, this.maxFrameSize);
          this.frame = new Uint8Array(size);
          this.frame.set(this.header);
          this.frameLength = HEADER_SIZE;
          this.headerLength = 0;
        }
        const count = Math.min(this.frame.length - this.frameLength, chunk.length - offset);
        this.frame.set(chunk.subarray(offset, offset + count), this.frameLength);
        this.frameLength += count;
        offset += count;
        if (this.frameLength === this.frame.length) {
          packets.push(decodeFrame(this.frame, this.maxFrameSize));
          this.frame = undefined;
          this.frameLength = 0;
        }
      }
      return packets;
    } catch (error) {
      this.reset();
      this.failed = true;
      throw error;
    }
  }

  /** Call at EOF or the end of an HTTP response to reject a partial last frame. */
  finish(): void {
    if (this.failed) throw new ProtocolError('decoder-failed', 'Reset decoder after a malformed stream');
    if (this.bufferedBytes !== 0) {
      this.reset();
      this.failed = true;
      throw new ProtocolError('truncated', 'Stream ended during a frame');
    }
  }

  reset(): void {
    this.headerLength = 0;
    this.frame = undefined;
    this.frameLength = 0;
    this.failed = false;
  }
}

/** A successful HTTP poll may contain zero frames. */
export function decodeFrames(bytes: Uint8Array, maxFrameSize = MAX_FRAME_SIZE): Packet[] {
  const decoder = new FrameDecoder(maxFrameSize);
  const packets = decoder.push(bytes);
  decoder.finish();
  return packets;
}
