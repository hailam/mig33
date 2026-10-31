import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import { mkdtemp, mkdir, writeFile, rm } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { pathToFileURL } from 'node:url';
import test from 'node:test';

// Resolve the actual Expo dependency graph, including pnpm patch identities.
const require = createRequire(import.meta.url);
const expoRequire = createRequire(require.resolve('expo/package.json'));
const cliRequire = createRequire(expoRequire.resolve('@expo/cli/package.json'));
const globCJS = cliRequire.resolve('glob');
const minimatchCJS = createRequire(globCJS).resolve('minimatch');
const fingerprintRequire = createRequire(expoRequire.resolve('@expo/fingerprint/package.json'));
const fingerprintPath = fingerprintRequire('./build/utils/Path.js');

for (const format of ['commonjs', 'esm']) {
  const load = (path: string) => import(pathToFileURL(path.replace('/commonjs/', `/${format}/`)).href);
  test(`${format}: minimatch preprocessing is bounded without changing ordinary Expo patterns`, async () => {
    const { Minimatch, minimatch } = await load(minimatchCJS);
    // Eight expansions suffice to fail unpatched10.2.6 without an unbounded PoC.
    const capped = new Minimatch(Array(8).fill('**/../a/b').join('/'), { optimizationLevel: 2, braceExpandMax: 64 });
    assert.equal(capped.set.length, 64);
    const large = new Minimatch('{x,*}/{0..999}', { optimizationLevel: 2 });
    assert.equal(large.set.length, 2000);
    assert.equal(large.match('x/5'), true); assert.equal(large.match('y/999'), true);
    assert.equal(large.match('.y/5'), false);
    assert.equal(minimatch('android/app/src/main/java/example/MainActivity.kt', '**/MainActivity.@(java|kt)'), true);
    assert.equal(minimatch('ios/mig33.xcodeproj/project.pbxproj', '**/*.xcodeproj/project.pbxproj'), true);
    assert.equal(minimatch('src/.hidden.ts', '**/*.ts', { dot: true }), true);
    assert.equal(minimatch('node_modules/expo/index.js', '**/node_modules/**'), true);
    // Preserve the v10 API required by its consumers; do not install a v11 rewrite.
    assert.ok(new Minimatch('**/*.ts').makeRe() instanceof RegExp);
  });

  test(`${format}: glob abort stops filesystem traversal for promise, stream and sync paths`, async (t) => {
    const api = await load(globCJS);
    const cwd = await mkdtemp(join(tmpdir(), 'mig33-glob-security-'));
    t.after(() => rm(cwd, { recursive: true, force: true }));
    for (const path of ['a/a/a/file.ts', 'a/b/b/file.ts', 'b/a/a/file.ts', 'b/b/b/file.ts']) {
      await mkdir(dirname(join(cwd, path)), { recursive: true }); await writeFile(join(cwd, path), '');
    }
    assert.equal((await api.glob('**/*.ts', { cwd })).length, 4);
    for (const mode of ['promise', 'stream', 'sync']) {
      const controller = new AbortController();
      const reason = new Error('Bounded traversal cancelled');
      let reads = 0;
      const counted = {
        ...fs,
        readdir: (...args: unknown[]) => { if (++reads === 1) controller.abort(reason); return Reflect.apply(fs.readdir, fs, args); },
        readdirSync: (...args: unknown[]) => { if (++reads === 1) controller.abort(reason); return Reflect.apply(fs.readdirSync, fs, args); },
      };
      const options = { cwd, fs: counted, signal: controller.signal };
      if (mode === 'promise') await assert.rejects(api.glob('**/*', options), reason);
      else if (mode === 'stream') await new Promise<void>((resolve, reject) => {
        const stream = api.globStream('**/*', options);
        stream.on('error', (error: Error) => { try { assert.equal(error, reason); resolve(); } catch (failure) { reject(failure); } });
        stream.resume();
      });
      else assert.throws(() => api.globSync('**/*', options), reason);
      await delay(25); // Let pending callbacks expose any traversal after rejection.
      assert.equal(reads, 1, `${mode} continued filesystem reads after cancellation`);
    }
  });
}

test('Expo fingerprint preserves directory exclusions and negated file exceptions', () => {
  const matchers = fingerprintPath.buildPathMatchObjects(['**/node_modules/**', '!**/node_modules/kept.js']);
  assert.equal(fingerprintPath.isIgnoredPathWithMatchObjects('node_modules/removed.js', matchers), true);
  assert.equal(fingerprintPath.isIgnoredPathWithMatchObjects('node_modules/kept.js', matchers), false);
  assert.equal(fingerprintPath.isIgnoredPathWithMatchObjects('src/app.ts', matchers), false);
});
