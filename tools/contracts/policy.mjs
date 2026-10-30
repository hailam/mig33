import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { parse } from 'yaml';

const read = (name) => readFile(new URL(name, import.meta.url), 'utf8');
const config = parse(await read('pnpm-workspace.yaml'));
const manifest = JSON.parse(await read('package.json'));
const exceptions = JSON.parse(await read('security-exceptions.json'));

assert.equal(config.minimumReleaseAge, 4320, 'Dependencies must age for three days');
assert.equal(config.minimumReleaseAgeStrict, true, 'Do not fall back to younger versions');
assert.equal(config.minimumReleaseAgeIgnoreMissingTime, false, 'Unknown publication age must fail');
assert.equal(config.trustLockfile, false, 'Recheck release age even on frozen lockfile installs');
assert.equal(config.blockExoticSubdeps, true, 'Transitive packages must use registry resolution');
assert.equal(config.strictDepBuilds, true);
assert.deepEqual(config.allowBuilds, { '@scarf/scarf': false },
  'These validation dependencies require no installation scripts; telemetry stays disabled');
assert.ok(Array.isArray(exceptions));

const exactVersion = /^(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?$/;
const exactPackage = /^(?:@[a-z0-9._-]+\/)?[a-z0-9._-]+@\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;
for (const [name, version] of Object.entries(manifest.devDependencies)) {
  assert.match(version, exactVersion, `${name} must use an exact registry version`);
}
assert.match(manifest.packageManager, /^pnpm@\d+\.\d+\.\d+$/);

const selectors = [];
for (const entry of exceptions) {
  assert.deepEqual(Object.keys(entry).sort(), ['advisory', 'expires', 'package', 'reason']);
  assert.match(entry.package, exactPackage, 'An exception must select one exact package version');
  assert.equal(new URL(entry.advisory).protocol, 'https:', 'Link the public security advisory');
  assert.ok(typeof entry.reason === 'string' && entry.reason.trim().length >= 20,
    'Explain why the advisory requires bypassing the three-day wait');
  assert.ok(Number.isFinite(Date.parse(entry.expires)) && Date.parse(entry.expires) > Date.now(),
    `Expired or invalid exception for ${entry.package}`);
  selectors.push(entry.package);
}
assert.equal(new Set(selectors).size, selectors.length, 'Duplicate exceptions');
assert.deepEqual([...(config.minimumReleaseAgeExclude ?? [])].sort(), selectors.sort(),
  'Every exact exclusion needs a reviewed security advisory, reason and expiry');

// The release date itself is enforced by pnpm against registry metadata at install.
// This check guards the committed settings; it cannot prove an advisory's relevance.
console.log(`Dependency policy: three days; ${exceptions.length} documented security exceptions.`);
