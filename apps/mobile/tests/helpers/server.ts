import { spawn } from 'node:child_process';
import { createWriteStream } from 'node:fs';
import { mkdir, mkdtemp } from 'node:fs/promises';
import { createServer } from 'node:net';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { writeTestResources } from '../../../../tools/mobile/test-resources.mjs';

export const repositoryRoot = fileURLToPath(new URL('../../../../', import.meta.url));

export interface TestServer {
  httpURL: string;
  httpPort: number;
  tcpPort: number;
  outputDirectory: string;
  close(): Promise<void>;
}

async function unusedPort(): Promise<number> {
  const socket = createServer();
  return new Promise((resolvePort, reject) => {
    socket.once('error', reject);
    socket.listen(0, '127.0.0.1', () => {
      const address = socket.address();
      if (!address || typeof address === 'string') {
        socket.close();
        reject(new Error('No loopback test port assigned'));
        return;
      }
      socket.close((error) => error ? reject(error) : resolvePort(address.port));
    });
  });
}

/** Start an isolated real Go process; never reuse the developer's database/ports. */
export async function startServer(binary = process.env.MIG33_TEST_SERVER_BINARY): Promise<TestServer> {
  if (!binary) throw new Error('Set MIG33_TEST_SERVER_BINARY to a built cmd/omni executable');
  const outputRoot = join(repositoryRoot, 'data/mobile-integration/runs');
  await mkdir(outputRoot, { recursive: true });
  const outputDirectory = await mkdtemp(join(outputRoot, 'run-'));
  const resourceDirectory = await writeTestResources(join(outputDirectory, 'resources'));
  const httpPort = await unusedPort();
  let tcpPort = await unusedPort();
  while (tcpPort === httpPort) tcpPort = await unusedPort();
  const httpURL = `http://127.0.0.1:${httpPort}`;
  // Config is explicit; inherited MIG33_TEST_* variables are not server config.
  const env = { ...process.env };
  for (const key of Object.keys(env)) if (key.startsWith('MIG33_')) delete env[key];
  // Always override the ignored recovered-resource default, even on machines
  // where applet setup has populated it. Clean CI and local runs use one fixture.
  env.MIG33_RESOURCES_DIRECTORY = resourceDirectory;
  const log = createWriteStream(join(outputDirectory, 'server.log'));
  const child = spawn(resolve(repositoryRoot, binary), [
    '-config-dir', join(repositoryRoot, 'configs'), '-env', '',
    '-http', `127.0.0.1:${httpPort}`, '-tcp', `127.0.0.1:${tcpPort}`,
    '-public-url', httpURL, '-db', ':memory:', '-operators', '',
  ], { cwd: repositoryRoot, env, stdio: ['ignore', 'pipe', 'pipe'] });
  let failure: Error | undefined;
  child.once('error', (error) => { failure = error; });
  child.stdout.pipe(log, { end: false });
  child.stderr.pipe(log, { end: false });
  let stopped = false;
  const close = async () => {
    if (stopped) return;
    stopped = true;
    if (child.exitCode === null && child.signalCode === null && child.pid) {
      const exited = new Promise<void>((resolveExit) => child.once('exit', () => resolveExit()));
      child.kill('SIGTERM');
      const force = setTimeout(() => child.kill('SIGKILL'), 10000);
      force.unref();
      await exited;
      clearTimeout(force);
    }
    await new Promise<void>((resolveLog) => log.end(resolveLog));
  };
  try {
    const deadline = Date.now() + 20000;
    while (Date.now() < deadline) {
      if (failure) throw failure;
      if (child.exitCode !== null || child.signalCode !== null) {
        throw new Error(`Go server exited; see ${join(outputDirectory, 'server.log')}`);
      }
      try {
        const response = await fetch(httpURL + '/healthz', { signal: AbortSignal.timeout(700) });
        if (response.ok) return { httpURL, httpPort, tcpPort, outputDirectory, close };
      } catch { /* Listener is not ready yet. */ }
      await delay(50);
    }
    throw new Error(`Go server startup timed out; see ${join(outputDirectory, 'server.log')}`);
  } catch (error) {
    await close();
    throw error;
  }
}
