// Byte-oriented UTF-8 without Node Buffer or platform TextEncoder polyfills.
// Raw fields always remain available, independent of this text interpretation.

/** Encode valid Unicode text. Unpaired UTF-16 surrogates are rejected locally. */
export function encodeUTF8(text: string): Uint8Array {
  let size = 0;
  for (const character of text) {
    const code = character.codePointAt(0)!;
    if (code >= 0xd800 && code <= 0xdfff) {
      throw new RangeError('Text contains an unpaired UTF-16 surrogate');
    }
    size += code < 0x80 ? 1 : code < 0x800 ? 2 : code < 0x10000 ? 3 : 4;
  }
  const bytes = new Uint8Array(size);
  let offset = 0;
  for (const character of text) {
    const code = character.codePointAt(0)!;
    if (code < 0x80) {
      bytes[offset++] = code;
    } else if (code < 0x800) {
      bytes[offset++] = 0xc0 | (code >> 6);
      bytes[offset++] = 0x80 | (code & 0x3f);
    } else if (code < 0x10000) {
      bytes[offset++] = 0xe0 | (code >> 12);
      bytes[offset++] = 0x80 | ((code >> 6) & 0x3f);
      bytes[offset++] = 0x80 | (code & 0x3f);
    } else {
      bytes[offset++] = 0xf0 | (code >> 18);
      bytes[offset++] = 0x80 | ((code >> 12) & 0x3f);
      bytes[offset++] = 0x80 | ((code >> 6) & 0x3f);
      bytes[offset++] = 0x80 | (code & 0x3f);
    }
  }
  return bytes;
}

/**
 * Decode UTF-8, retaining BOM characters and replacing malformed sequences.
 * Malformed text replacement is not a claim about every historical device's
 * decoder. Callers needing exact bytes must use Packet.bytes()/fields.
 */
export function decodeUTF8(bytes: Uint8Array): string {
  const characters: string[] = [];
  let offset = 0;
  while (offset < bytes.length) {
    const first = bytes[offset++];
    if (first < 0x80) {
      characters.push(String.fromCharCode(first));
      continue;
    }
    let continuation: number;
    let code: number;
    let low = 0x80;
    let high = 0xbf;
    if (first >= 0xc2 && first <= 0xdf) {
      continuation = 1;
      code = first & 0x1f;
    } else if (first >= 0xe0 && first <= 0xef) {
      continuation = 2;
      code = first & 0x0f;
      if (first === 0xe0) low = 0xa0;
      if (first === 0xed) high = 0x9f;
    } else if (first >= 0xf0 && first <= 0xf4) {
      continuation = 3;
      code = first & 0x07;
      if (first === 0xf0) low = 0x90;
      if (first === 0xf4) high = 0x8f;
    } else {
      characters.push('\ufffd');
      continue;
    }
    let valid = true;
    for (let index = 0; index < continuation; index++) {
      const next = bytes[offset];
      if (offset === bytes.length || next < low || next > high) {
        valid = false;
        break;
      }
      offset++;
      code = (code << 6) | (next & 0x3f);
      low = 0x80;
      high = 0xbf;
    }
    characters.push(valid ? String.fromCodePoint(code) : '\ufffd');
  }
  return characters.join('');
}
