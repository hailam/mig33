// Build-specific transport policy. Metro exceptions exist only in Debug;
// application gateways still pass the independent HTTPS/TLS runtime policy.
const { access, mkdir, writeFile } = require('node:fs/promises');
const path = require('node:path');
const { createRequire } = require('node:module');
const { withInfoPlist, withXcodeProject, withAndroidManifest, withFinalizedMod, AndroidConfig } = require('expo/config-plugins');
// Use the plist implementation already owned/pinned by Expo's config toolchain.
const expoRequire = createRequire(require.resolve('expo/config-plugins'));
const pluginsRequire = createRequire(expoRequire.resolve('@expo/config-plugins'));
const plist = pluginsRequire('@expo/plist').default;

function iosTransportSecurity(debug) {
  return {
    NSAllowsArbitraryLoads: false,
    NSAllowsLocalNetworking: false,
    ...(debug ? { NSExceptionDomains: Object.fromEntries(['localhost', '127.0.0.1', '::1'].map((host) => [host, {
      NSExceptionAllowsInsecureHTTPLoads: true,
      NSIncludesSubdomains: false,
    }])) } : {}),
  };
}

function androidNetworkSecurity(debug) {
  return `<?xml version="1.0" encoding="utf-8"?>
<network-security-config>
  <base-config cleartextTrafficPermitted="false">
    <trust-anchors><certificates src="system" /></trust-anchors>
  </base-config>
${debug ? `  <domain-config cleartextTrafficPermitted="true">
    <domain includeSubdomains="false">localhost</domain>
    <domain includeSubdomains="false">127.0.0.1</domain>
    <domain includeSubdomains="false">::1</domain>
    <domain includeSubdomains="false">10.0.2.2</domain>
  </domain-config>
  <debug-overrides><trust-anchors><certificates src="user" /></trust-anchors></debug-overrides>
` : ''}</network-security-config>
`;
}

function withSecureNetworking(config) {
  let infoPlistPath;
  config = withInfoPlist(config, (mod) => {
    mod.modResults.NSAppTransportSecurity = iosTransportSecurity(false);
    return mod;
  });
  config = withXcodeProject(config, (mod) => {
    for (const entry of Object.values(mod.modResults.pbxXCBuildConfigurationSection())) {
      if (!entry || typeof entry !== 'object' || !entry.buildSettings?.INFOPLIST_FILE) continue;
      const settings = entry.buildSettings;
      if (String(settings.PRODUCT_BUNDLE_IDENTIFIER).replaceAll('"', '') !== mod.ios.bundleIdentifier) continue;
      const base = String(settings.INFOPLIST_FILE).replaceAll('"', '').replace(/Info\.Debug\.plist$/, 'Info.plist');
      if (!base.endsWith('/Info.plist') || base.includes('$')) throw new Error('Review the app Info.plist path before configuring secure networking.');
      infoPlistPath = base;
      settings.INFOPLIST_FILE = String(entry.name).replaceAll('"', '') === 'Debug' ? base.replace(/Info\.plist$/, 'Info.Debug.plist') : base;
    }
    if (!infoPlistPath) throw new Error('Could not locate the app target for secure networking policy.');
    return mod;
  });
  config = withFinalizedMod(config, ['ios', async (mod) => {
    if (!infoPlistPath) throw new Error('Missing app Info.plist configuration.');
    // All Expo Info.plist mods have finished. Keep both build variants identical
    // except for the narrowly scoped Debug Metro exception.
    for (const debug of [false, true]) {
      const filename = debug ? infoPlistPath.replace(/Info\.plist$/, 'Info.Debug.plist') : infoPlistPath;
      await writeFile(path.join(mod.modRequest.platformProjectRoot, filename), plist.build({
        ...mod.ios.infoPlist, NSAppTransportSecurity: iosTransportSecurity(debug),
      }));
    }
    return mod;
  }]);
  config = withAndroidManifest(config, (mod) => {
    const app = AndroidConfig.Manifest.getMainApplicationOrThrow(mod.modResults);
    app.$['android:usesCleartextTraffic'] = 'false';
    app.$['android:networkSecurityConfig'] = '@xml/mig33_network_security_config';
    return mod;
  });
  return withFinalizedMod(config, ['android', async (mod) => {
    for (const variant of ['main', 'debug', 'debugOptimized']) {
      const root = path.join(mod.modRequest.platformProjectRoot, 'app/src', variant);
      await mkdir(path.join(root, 'res/xml'), { recursive: true });
      await writeFile(path.join(root, 'res/xml/mig33_network_security_config.xml'), androidNetworkSecurity(variant !== 'main'));
      if (variant === 'main') continue;
      const filename = path.join(root, 'AndroidManifest.xml');
      let manifest;
      try { await access(filename); manifest = await AndroidConfig.Manifest.readAndroidManifestAsync(filename); }
      catch (error) {
        if (error.code !== 'ENOENT') throw error;
        manifest = { manifest: { $: { 'xmlns:android': 'http://schemas.android.com/apk/res/android' }, application: [{ $: {} }] } };
      }
      // Variant overlays normally omit android:name and merge into the main
      // application; Expo's getMainApplicationOrThrow only accepts the latter.
      const applications = manifest.manifest.application ??= [];
      if (applications.length > 1) throw new Error('Expected one application in the Debug manifest overlay.');
      const app = applications[0] ?? (applications[0] = { $: {} });
      app.$ ??= {};
      app.$['android:usesCleartextTraffic'] = 'false';
      await AndroidConfig.Manifest.writeAndroidManifestAsync(filename, manifest);
    }
    return mod;
  }]);
}

module.exports = withSecureNetworking;
module.exports.iosTransportSecurity = iosTransportSecurity;
module.exports.androidNetworkSecurity = androidNetworkSecurity;
