// Synthetic resources for isolated mobile integration servers. Match the
// bounded atlas geometry/pixel pattern in internal/testassets; no applet bytes
// are copied, and these files must never supply production artwork.
import { mkdir, writeFile } from 'node:fs/promises';
import { join, resolve } from 'node:path';
import { crc32, deflateSync } from 'node:zlib';

/** @param {string} name @param {Buffer} data */
function chunk(name, data) {
  const type = Buffer.from(name);
  const size = Buffer.alloc(4), checksum = Buffer.alloc(4);
  size.writeUInt32BE(data.length);
  checksum.writeUInt32BE(crc32(data, crc32(type)));
  return Buffer.concat([size, type, data, checksum]);
}

/** @param {number} width @param {number} height */
function fixturePNG(width, height) {
  const stride = width * 4 + 1;
  const pixels = Buffer.alloc(height * stride); // PNG filter0 before each row.
  for (let y = 0; y < height; y++) for (let x = 0; x < width; x++) {
    const offset = y * stride + x * 4 + 1;
    const cell = Math.floor(x / 12) + Math.floor(y / 12) * 20;
    pixels.set([cell * 3 & 255, x & 255, y * 5 & 255, 255], offset);
  }
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0); header.writeUInt32BE(height, 4);
  header[8] = 8; header[9] = 6; // Eight-bit RGBA, no interlacing.
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk('IHDR', header), chunk('IDAT', deflateSync(pixels)), chunk('IEND', Buffer.alloc(0))]);
}

/** Each caller supplies its newly created, ignored run directory.
 * @param {string} directory
 * @returns {Promise<string>} Absolute resources directory for server config.
 */
export async function writeTestResources(directory) {
  const destination = resolve(directory);
  await mkdir(destination, { recursive: true });
  await writeFile(join(destination, 'sprites.png'), fixturePNG(240, 48), { flag: 'wx' });
  await writeFile(join(destination, 'default-avatar.png'), fixturePNG(24, 24), { flag: 'wx' });
  return destination;
}

if (import.meta.main) {
  if (process.argv.length !== 3) throw new Error('Usage: node tools/mobile/test-resources.mjs <new-resources-directory>');
  await writeTestResources(process.argv[2]);
}
