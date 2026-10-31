import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';

// Query only these non-secret settings through the selected pnpm, so both
// package scopes enforce their effective policy without sharing JS dependencies.
const directory = resolve(process.argv[2] ?? '.');
assert.ok(existsSync(resolve(directory, 'pnpm-workspace.yaml')), 'Commit the package scope policy');
const setting = (key) => JSON.parse(execFileSync('pnpm',
  ['--dir', directory, 'config', 'get', key, '--json'], { encoding: 'utf8' }).trim());
const read = (name) => JSON.parse(readFileSync(resolve(directory, name), 'utf8'));
const manifest = read('package.json');
const exceptions = read('security-exceptions.json');
assert.equal(setting('minimumReleaseAge'), 4320, 'Dependencies must age for three days');
assert.equal(setting('minimumReleaseAgeStrict'), true, 'Do not fall back to younger versions');
assert.equal(setting('minimumReleaseAgeIgnoreMissingTime'), false, 'Unknown publication age must fail');
assert.equal(setting('trustLockfile'), false, 'Recheck release age on frozen lockfile installs');
assert.equal(setting('blockExoticSubdeps'), true, 'Transitive packages must use registry resolution');
assert.equal(setting('strictDepBuilds'), true);
for (const [name, allowed] of Object.entries(setting('allowBuilds') ?? {})) {
  assert.equal(allowed, false, `Review install-script execution separately: ${name}`);
}
assert.ok(Array.isArray(exceptions));

const exactVersion = /^(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$/;
const exactPackage = /^(?:@[a-z0-9._-]+\/)?[a-z0-9._-]+@\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;
for (const [name, version] of Object.entries({ ...manifest.dependencies, ...manifest.devDependencies })) {
  assert.match(version, exactVersion, `${name} must use an exact registry version`);
}
assert.match(manifest.packageManager, /^pnpm@\d+\.\d+\.\d+$/);

const selectors = [];
for (const entry of exceptions) {
  assert.deepEqual(Object.keys(entry).sort(), ['advisory', 'expires', 'package', 'reason']);
  assert.match(entry.package, exactPackage, 'Select the exact version that fixes the advisory');
  assert.equal(new URL(entry.advisory).protocol, 'https:', 'Link the public security advisory');
  assert.ok(typeof entry.reason === 'string' && entry.reason.trim().length >= 20,
    'Explain why the cited fix requires bypassing the three-day wait');
  assert.ok(Number.isFinite(Date.parse(entry.expires)) && Date.parse(entry.expires) > Date.now(),
    `Expired or invalid exception for ${entry.package}`);
  selectors.push(entry.package);
}
assert.equal(new Set(selectors).size, selectors.length, 'Duplicate exceptions');
assert.deepEqual([...(setting('minimumReleaseAgeExclude') ?? [])].sort(), selectors.sort(),
  'Every exact exclusion needs a reviewed security advisory, reason and expiry');
// Human review must establish that the exact version fixes the cited advisory.
console.log(`${manifest.name}: three-day dependency policy; ${exceptions.length} documented exceptions.`);
