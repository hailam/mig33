import type { Packet } from './protocol/index.ts';

export interface EmoticonAsset {
  token: string;
  label: string;
  aliases: readonly string[];
  type: number;
  image?: Uint8Array;
  imageURI?: string;
  frames?: readonly Uint8Array[];
  frameURIs?: readonly string[];
  audio?: Uint8Array;
}

/** Only image formats decoded by the original resource path become image URIs. */
export function imageDataURI(bytes: Uint8Array): string | undefined {
  const png = [137, 80, 78, 71, 13, 10, 26, 10].every((value, index) => bytes[index] === value);
  const jpeg = bytes[0] === 255 && bytes[1] === 216 && bytes[2] === 255;
  const gif = bytes[0] === 71 && bytes[1] === 73 && bytes[2] === 70 && bytes[3] === 56
    && (bytes[4] === 55 || bytes[4] === 57) && bytes[5] === 97;
  const mime = png ? 'image/png' : jpeg ? 'image/jpeg' : gif ? 'image/gif' : undefined;
  if (!mime) return undefined;
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  let encoded = '';
  for (let offset = 0; offset < bytes.length; offset += 3) {
    const a = bytes[offset], b = bytes[offset + 1] ?? 0, c = bytes[offset + 2] ?? 0;
    encoded += alphabet[a >> 2] + alphabet[((a & 3) << 4) | (b >> 4)]
      + (offset + 1 < bytes.length ? alphabet[((b & 15) << 2) | (c >> 6)] : '=')
      + (offset + 2 < bytes.length ? alphabet[c & 63] : '=');
  }
  return `data:${mime};base64,${encoded}`;
}

/** Original914 types3/4 use signed big-endian framed lengths, not plain PNGs. */
export function decodeEmoticon(packet: Packet): EmoticonAsset {
  const data = packet.fields.get(4);
  const type = data ? packet.byte(5) ?? 1 : 0;
  const result: EmoticonAsset = { token: packet.text(1) ?? '', label: packet.text(3) ?? '',
    aliases: packet.text(2)?.split(/\s+/).filter(Boolean) ?? [], type };
  if (!data) return result;
  if (type === 1 || type === 2) return { ...result, image: data, imageURI: imageDataURI(data) };
  if (type !== 3 && type !== 4) return result;
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  let offset = 0;
  const readLength = () => {
    if (offset + 4 > data.length) throw new Error('Truncated emoticon resource');
    const length = view.getInt32(offset, false); offset += 4;
    if (length < 0) throw new Error('Invalid emoticon resource length');
    return length;
  };
  const readPart = () => {
    const length = readLength();
    if (length > data.length - offset) throw new Error('Truncated emoticon resource');
    const part = data.slice(offset, offset + length); offset += length;
    return part;
  };
  if (type === 3) {
    const count = readLength();
    if (count > (data.length - offset) / 4) throw new Error('Invalid emoticon frame count');
    const frames: Uint8Array[] = [];
    for (let index = 0; index < count; index++) frames.push(readPart());
    if (offset !== data.length) throw new Error('Unexpected data after emoticon frames');
    return { ...result, frames, image: frames[0], imageURI: frames[0] && imageDataURI(frames[0]),
      frameURIs: frames.map(imageDataURI).filter((uri): uri is string => uri !== undefined) };
  }
  const image = readPart(), audio = readPart();
  if (offset !== data.length) throw new Error('Unexpected data after emoticon audio');
  return { ...result, image, imageURI: imageDataURI(image), audio };
}
