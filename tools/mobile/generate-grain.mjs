// Deterministic, original texture for Modern's content backdrop. No downloaded
// artwork or runtime generation. Run with the app's mise-managed Node.
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { deflateSync } from 'node:zlib';

const output = resolve(import.meta.dirname, '../../apps/mobile/assets');
mkdirSync(output, { recursive: true });
function chunk(name, data) {
  const type = Buffer.from(name);
  let crc = 0xffffffff;
  for (const byte of Buffer.concat([type, data])) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit++) crc = crc & 1 ? 0xedb88320 ^ (crc >>> 1) : crc >>> 1;
  }
  const size = Buffer.alloc(4), checksum = Buffer.alloc(4);
  size.writeUInt32BE(data.length); checksum.writeUInt32BE((crc ^ 0xffffffff) >>> 0);
  return Buffer.concat([size, type, data, checksum]);
}
for (const scale of [1, 2, 3]) {
  const side = 64 * scale, stride = side * 2 + 1;
  const pixels = Buffer.alloc(side * stride);
  let seed = 0x33c0ffee;
  for (let y = 0; y < side; y++) for (let x = 0; x < side; x++) {
    seed ^= seed << 13; seed ^= seed >>> 17; seed ^= seed << 5;
    const offset = y * stride + x * 2 + 1;
    pixels[offset] = seed & 255;
    pixels[offset + 1] = 8 + ((seed >>> 8) & 15);
  }
  const header = Buffer.alloc(13);
  header.writeUInt32BE(side, 0); header.writeUInt32BE(side, 4);
  header[8] = 8; header[9] = 4; // Eight-bit grayscale + alpha.
  const png = Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', header), chunk('IDAT', deflateSync(pixels)), chunk('IEND', Buffer.alloc(0))]);
  writeFileSync(resolve(output, `grain${scale === 1 ? '' : `@${scale}x`}.png`), png);
}
