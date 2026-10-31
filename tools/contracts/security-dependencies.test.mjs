import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import test from 'node:test';

// Resolve the real parser dependency graph, including pnpm's patch identity.
// A separately installed minimatch would not establish that Spectral uses it.
const require = createRequire(import.meta.url);
const parserRequire = createRequire(require.resolve('@asyncapi/parser'));
const spectralEntry = parserRequire.resolve('@stoplight/spectral-core');
const spectralRequire = createRequire(spectralEntry);
const { Ruleset } = spectralRequire('./ruleset/index.js');
const { minimatch: sourceMatches } = spectralRequire('./ruleset/utils/minimatch.js');
const minimatch = spectralRequire('minimatch');

test('the parser resolves the reviewed fixed minimatch release through Spectral', () => {
  const spectralPackage = JSON.parse(readFileSync(join(dirname(spectralEntry), '../package.json'), 'utf8'));
  assert.equal(spectralPackage.version, '1.24.0', 'review the adapter patch when Spectral changes');
  assert.equal(spectralRequire('minimatch/package.json').version, '11.0.2');
  assert.equal(typeof minimatch.minimatch, 'function');
});

test('Spectral source matching retains basename, globstar, braces, extglobs and exclusions', () => {
  for (const [source, pattern, expected] of [
    ['api/v4/asyncapi.yaml', '*.yaml', true],
    ['api/v4/asyncapi.yaml', 'api/**/asyncapi.yaml', true],
    ['api/asyncapi.yml', '**/asyncapi.{yaml,yml}', true],
    ['api/asyncapi.yaml', '**/asyncapi.@(yaml|json)', true],
    ['api/asyncapi.json', '*.yaml', false],
    ['api/.asyncapi.yaml', '*.yaml', false],
    ['api/asyncapi.yaml', '!*.json', true],
    ['api/asyncapi.json', '!*.json', false],
    ['api/asyncapi.yaml', 'API/*.yaml', false],
  ]) assert.equal(sourceMatches(source, pattern), expected, `${source} against ${pattern}`);
});

test('Spectral file and JSON-pointer overrides still select the intended diagnostic severity', () => {
  const ruleset = new Ruleset({
    rules: {
      reviewed: { given: '$', severity: 'error', then: { function: () => [{ message: 'reviewed diagnostic' }] } },
    },
    overrides: [
      { files: ['**/ignored.{yaml,yml}'], rules: { reviewed: 'off' } },
      { files: ['**/warning.@(yaml|json)'], rules: { reviewed: 'warn' } },
      { files: ['**/asyncapi.yaml#/info'], rules: { reviewed: 'warn' } },
    ],
  }, { source: '/workspace/.spectral.yaml' });
  assert.equal(ruleset.fromSource('/workspace/api/ignored.yml').rules.reviewed.severity, -1);
  assert.equal(ruleset.fromSource('/workspace/api/warning.json').rules.reviewed.severity, 1);
  assert.equal(ruleset.fromSource('/workspace/api/other.yaml').rules.reviewed.severity, 0);
  const source = '/workspace/api/asyncapi.yaml';
  const rule = ruleset.fromSource(source).rules.reviewed;
  assert.equal(rule.getSeverityForSource(source, ['info', 'title']), 1);
  assert.equal(rule.getSeverityForSource(source, ['channels']), 0);
  assert.equal(rule.getSeverityForSource('/workspace/api/other.yaml', ['info']), 0);
});

test('the installed minimatch bounds the advisory preprocessing case', () => {
  // A small upstream-style probe exercises the fixed expansion cap without an
  // unbounded denial-of-service payload. Spectral itself only sets matchBase.
  const capped = new minimatch.Minimatch(Array(8).fill('**/../a/b').join('/'), {
    optimizationLevel: 2, braceExpandMax: 64,
  });
  assert.equal(capped.set.length, 64);
  const larger = new minimatch.Minimatch('{x,*}/{0..999}', { optimizationLevel: 2 });
  assert.equal(larger.set.length, 2000);
  assert.equal(larger.match('x/5'), true);
  assert.equal(larger.match('y/999'), true);
  assert.equal(larger.match('.y/5'), false);
});
