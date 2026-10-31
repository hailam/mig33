// Local negative TLS/redirect fixtures for the real native app. Only counts and
// fixture names are logged: credentials and application payloads are never saved.
import { readFile, writeFile, appendFile } from 'node:fs/promises';
import { createServer as httpsServer } from 'node:https';
import { createServer as httpServer } from 'node:http';
import { createServer as tlsServer } from 'node:tls';
import { createServer as netServer } from 'node:net';
import { join } from 'node:path';

const [directory, readyPath, eventsPath] = process.argv.slice(2);
if (!directory || !readyPath || !eventsPath) throw new Error('Usage: secure-probes.mjs <tls-directory> <ready.json> <events.jsonl>');
const servers = [], sockets = new Set();
let writes = Promise.resolve();
const log = (event) => { writes = writes.then(() => appendFile(eventsPath, JSON.stringify(event) + '\n')); };
await writeFile(eventsPath, '');

async function listen(server) {
  servers.push(server);
  server.on('connection', (socket) => { sockets.add(socket); socket.on('close', () => sockets.delete(socket)); });
  server.on('tlsClientError', () => {}); // Expected rejection; log no key/certificate details.
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  return server.address().port;
}
const options = async (name) => ({ minVersion: 'TLSv1.2',
  cert: await readFile(join(directory, name + '.pem')),
  key: await readFile(join(directory, name + '-key.pem')),
});
const ports = {};
for (const mode of ['https', 'tls']) {
  const name = mode + '-plaintext';
  ports[name] = await listen(netServer((socket) => {
    socket.on('error', () => {});
    socket.once('data', (bytes) => {
      // TLS starts with a handshake record (22), not an original protocol
      // frame or HTTP request. Never retain the bytes themselves.
      log({ event: bytes[0] === 22 ? 'plaintext-rejected-handshake' : 'unexpected-plaintext', probe: name });
      socket.end('HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\nConnection: close\r\n\r\n');
    });
  }));
}
for (const name of ['wrong-host', 'expired', 'untrusted']) {
  const configuration = await options(name);
  ports['https-' + name] = await listen(httpsServer(configuration, (request, response) => {
    log({ event: 'unexpected-request', probe: 'https-' + name });
    request.resume(); response.writeHead(400); response.end();
  }));
  ports['tls-' + name] = await listen(tlsServer(configuration, (socket) => {
    socket.on('error', () => {});
    socket.on('data', (bytes) => {
      log({ event: 'unexpected-bytes', probe: 'tls-' + name, count: bytes.length });
      socket.destroy();
    });
  }));
}
const sink = await listen(httpServer((request, response) => {
  log({ event: 'unexpected-redirect', probe: 'https-downgrade' });
  request.resume(); response.writeHead(400); response.end();
}));
ports['https-downgrade'] = await listen(httpsServer(await options('server'), (request, response) => {
  log({ event: 'redirect-offered', probe: 'https-downgrade' });
  request.resume(); response.writeHead(307, { Location: `http://127.0.0.1:${sink}/must-not-receive` }); response.end();
}));
await writeFile(readyPath, JSON.stringify(ports, null, 2) + '\n');

let stopping = false;
async function stop() {
  if (stopping) return;
  stopping = true;
  for (const socket of sockets) socket.destroy();
  await Promise.all(servers.map((server) => new Promise((resolve) => server.close(resolve))));
  await writes;
  await appendFile(eventsPath, JSON.stringify({ event: 'probes-stopped' }) + '\n');
  process.exit(0);
}
process.on('SIGTERM', () => { void stop(); });
process.on('SIGINT', () => { void stop(); });
