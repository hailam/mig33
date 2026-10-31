import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

const require = createRequire(import.meta.url);
const plugin = require('../plugins/withSecureNetworking.cjs');
const { iosTransportSecurity, androidNetworkSecurity } = plugin;
const { XML, compileModsAsync } = require('expo/config-plugins');

test('iOS Release denies cleartext; Debug allows only exact Metro loopback hosts', () => {
  const release = iosTransportSecurity(false);
  assert.equal(release.NSAllowsArbitraryLoads, false);
  assert.equal(release.NSAllowsLocalNetworking, false);
  assert.equal(release.NSExceptionDomains, undefined);
  const debug = iosTransportSecurity(true);
  assert.equal(debug.NSAllowsArbitraryLoads, false);
  assert.equal(debug.NSAllowsLocalNetworking, false);
  assert.deepEqual(Object.keys(debug.NSExceptionDomains).sort(), ['127.0.0.1', '::1', 'localhost']);
  for (const exception of Object.values(debug.NSExceptionDomains) as Record<string, unknown>[]) {
    assert.equal(exception.NSExceptionAllowsInsecureHTTPLoads, true);
    assert.equal(exception.NSIncludesSubdomains, false);
    assert.equal(exception.NSExceptionMinimumTLSVersion, undefined, 'No TLS downgrade exceptions');
  }
});

test('Android prebuild creates missing Debug manifests and retains strict Release policy across repeated runs', async (t) => {
  const root = await mkdtemp(join(tmpdir(), 'mig33-network-policy-'));
  t.after(() => rm(root, { recursive: true }));
  const main = join(root, 'android/app/src/main');
  await mkdir(main, { recursive: true });
  await writeFile(join(root, 'package.json'), JSON.stringify({ name: 'mig33-test', version: '0.1.0' }));
  await writeFile(join(main, 'AndroidManifest.xml'), '<?xml version="1.0" encoding="utf-8"?>' +
    '<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.hailam.mig33">' +
    '<application android:name=".MainApplication" android:usesCleartextTraffic="true" /></manifest>');
  for (let run = 0; run < 2; run++) {
    await compileModsAsync(plugin({ name: 'mig33', slug: 'mig33', android: { package: 'com.hailam.mig33' } }),
      { projectRoot: root, platforms: ['android'] });
    for (const variant of ['main', 'debug', 'debugOptimized']) {
      const folder = join(root, 'android/app/src', variant);
      const manifest = await readFile(join(folder, 'AndroidManifest.xml'), 'utf8');
      assert.match(manifest, /android:usesCleartextTraffic="false"/);
      const policy = await readFile(join(folder, 'res/xml/mig33_network_security_config.xml'), 'utf8');
      assert.equal(policy, androidNetworkSecurity(variant !== 'main'));
      if (variant === 'main') assert.match(manifest, /android:networkSecurityConfig="@xml\/mig33_network_security_config"/);
    }
  }
});

test('Android Release trusts system CAs without cleartext; Debug restricts Metro and user CA trust', async () => {
  for (const debug of [false, true]) {
    const xml = await XML.parseXMLAsync(androidNetworkSecurity(debug));
    const policy = xml['network-security-config'];
    assert.equal(policy['base-config'][0].$.cleartextTrafficPermitted, 'false');
    assert.equal(policy['base-config'][0]['trust-anchors'][0].certificates[0].$.src, 'system');
    if (!debug) {
      assert.equal(policy['domain-config'], undefined);
      assert.equal(policy['debug-overrides'], undefined);
      continue;
    }
    const local = policy['domain-config'][0];
    assert.equal(local.$.cleartextTrafficPermitted, 'true');
    assert.deepEqual(local.domain.map((domain: { _: string }) => domain._).sort(), ['10.0.2.2', '127.0.0.1', '::1', 'localhost']);
    assert.ok(local.domain.every((domain: { $: Record<string, string> }) => domain.$.includeSubdomains === 'false'));
    assert.equal(policy['debug-overrides'][0]['trust-anchors'][0].certificates[0].$.src, 'user');
  }
});
