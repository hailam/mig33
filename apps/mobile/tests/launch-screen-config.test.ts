import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { copyFile, mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

const require = createRequire(import.meta.url);
const { compileModsAsync, withInfoPlist } = require('expo/config-plugins');
const expoRequire = createRequire(require.resolve('expo/config-plugins'));
const pluginsRequire = createRequire(expoRequire.resolve('@expo/config-plugins'));
const plist = pluginsRequire('@expo/plist').default;
const xcode = pluginsRequire('xcode');
const splash = require('expo-splash-screen/app.plugin.js').default;
const security = require('../plugins/withSecureNetworking.cjs');
const launch = require('../plugins/withLaunchScreen.cjs');

// Minimal actual Xcode project, with one app target and both build variants.
// No ignored local ios/ directory or macOS command is needed by this test.
const projectFixture = `// !$*UTF8*$!
{
  archiveVersion = 1; objectVersion = 54; classes = {}; objects = {
/* Begin PBXProject section */
    A00000000000000000000001 /* Project object */ = {isa = PBXProject; mainGroup = A00000000000000000000002; targets = (A00000000000000000000004 /* mig33 */,); buildConfigurationList = A00000000000000000000008; };
/* End PBXProject section */
/* Begin PBXGroup section */
    A00000000000000000000002 = {isa = PBXGroup; children = (A00000000000000000000003 /* mig33 */,); sourceTree = "<group>"; };
    A00000000000000000000003 /* mig33 */ = {isa = PBXGroup; name = mig33; children = (); sourceTree = "<group>"; };
/* End PBXGroup section */
/* Begin PBXNativeTarget section */
    A00000000000000000000004 /* mig33 */ = {isa = PBXNativeTarget; name = mig33; productName = mig33; productType = "com.apple.product-type.application"; buildConfigurationList = A00000000000000000000008; buildPhases = (A00000000000000000000005 /* Resources */,); };
/* End PBXNativeTarget section */
/* Begin PBXResourcesBuildPhase section */
    A00000000000000000000005 /* Resources */ = {isa = PBXResourcesBuildPhase; buildActionMask = 2147483647; files = (); runOnlyForDeploymentPostprocessing = 0; };
/* End PBXResourcesBuildPhase section */
/* Begin XCBuildConfiguration section */
    A00000000000000000000006 /* Debug */ = {isa = XCBuildConfiguration; name = Debug; buildSettings = {INFOPLIST_FILE = mig33/Info.plist; PRODUCT_NAME = mig33; PRODUCT_BUNDLE_IDENTIFIER = com.hailam.mig33; }; };
    A00000000000000000000007 /* Release */ = {isa = XCBuildConfiguration; name = Release; buildSettings = {INFOPLIST_FILE = mig33/Info.plist; PRODUCT_NAME = mig33; PRODUCT_BUNDLE_IDENTIFIER = com.hailam.mig33; }; };
/* End XCBuildConfiguration section */
/* Begin XCConfigurationList section */
    A00000000000000000000008 /* Build configuration list */ = {isa = XCConfigurationList; buildConfigurations = (A00000000000000000000006 /* Debug */,A00000000000000000000007 /* Release */,); defaultConfigurationName = Release; };
/* End XCConfigurationList section */
  }; rootObject = A00000000000000000000001 /* Project object */;
}
`;

test('CNG selects and links launch resources in Debug and Release without losing final plist policy', async (t) => {
  const root = await mkdtemp(join(tmpdir(), 'mig33-launch-'));
  t.after(() => rm(root, { recursive: true }));
  const app = join(root, 'ios/mig33');
  const projectFile = join(root, 'ios/mig33.xcodeproj/project.pbxproj');
  await mkdir(app, { recursive: true });
  await mkdir(join(root, 'ios/mig33.xcodeproj'));
  await mkdir(join(root, 'assets'));
  await writeFile(join(root, 'package.json'), JSON.stringify({ name: 'mig33-test', version: '0.1.0' }));
  await writeFile(join(app, 'AppDelegate.swift'), '// Fixture used only to locate the native app directory.\n');
  await writeFile(join(app, 'Info.plist'), plist.build({ CFBundleName: 'mig33' }));
  await writeFile(projectFile, projectFixture);
  // Empty sections are normally supplied by the native template.
  const initial = xcode.project(projectFile).parseSync();
  initial.hash.project.objects.PBXBuildFile = {};
  initial.hash.project.objects.PBXFileReference = {};
  await writeFile(projectFile, initial.writeSync());
  const oldCatalog = join(app, 'Images.xcassets/Mig33LaunchLogo.imageset');
  await mkdir(oldCatalog, { recursive: true });
  await writeFile(join(oldCatalog, 'Contents.json'), '{}');
  for (const scale of [1, 2, 3]) {
    const asset = `launch-mark${scale === 1 ? '' : `@${scale}x`}.png`;
    await copyFile(new URL(`../assets/${asset}`, import.meta.url), join(root, 'assets', asset));
  }
  for (let run = 0; run < 3; run++) {
    let config = splash({ name: 'mig33', slug: 'mig33', _internal: { projectRoot: root },
      ios: { bundleIdentifier: 'com.hailam.mig33' } }, {
      backgroundColor: '#f5f9fa', dark: { backgroundColor: '#0b151c' },
    });
    config = withInfoPlist(config, (mod: { modResults: Record<string, unknown> }) => {
      mod.modResults.NSCameraUsageDescription = 'Take a photo to share in mig33.';
      return mod;
    });
    config = launch(security(config)); // Same plugin order as app.config.ts.
    await compileModsAsync(config, { projectRoot: root, platforms: ['ios'] });
    for (const debug of [false, true]) {
      const contents = plist.parse(await readFile(join(app, debug ? 'Info.Debug.plist' : 'Info.plist'), 'utf8'));
      assert.equal(contents.UILaunchStoryboardName, 'Mig33Launch');
      assert.equal(contents.NSCameraUsageDescription, 'Take a photo to share in mig33.');
      assert.deepEqual(JSON.parse(JSON.stringify(contents.NSAppTransportSecurity)), security.iosTransportSecurity(debug));
    }
    const project = xcode.project(projectFile).parseSync();
    const references = Object.entries(project.pbxFileReferenceSection()).filter(([, item]: [string, any]) =>
      item && typeof item === 'object' && String(item.path).replaceAll('"', '') === 'mig33/Mig33Launch.storyboard');
    assert.equal(references.length, 1);
    const [referenceId, reference] = references[0] as [string, any];
    assert.equal(reference.lastKnownFileType, 'file.storyboard');
    assert.equal(reference.fileEncoding, 4);
    assert.equal(reference.explicitFileType, undefined);
    const builds = Object.entries(project.pbxBuildFileSection()).filter(([, item]: [string, any]) => item?.fileRef === referenceId);
    assert.equal(builds.length, 1, 'The storyboard must have an actual PBXBuildFile');
    const phase = project.pbxResourcesBuildPhaseObj(project.getTarget('com.apple.product-type.application').uuid);
    assert.equal(phase.files.filter((item: { value: string }) => item.value === builds[0][0]).length, 1);
    const storyboard = await readFile(join(app, 'Mig33Launch.storyboard'), 'utf8');
    assert.match(storyboard, /firstAttribute="width" constant="176"/);
    assert.match(storyboard, /firstAttribute="height" constant="100.1"/);
    assert.match(storyboard, /identifier="mig33.launch.top"/);
    assert.match(storyboard, /systemColor="systemBackgroundColor"/);
    assert.match(storyboard, /image="Mig33LaunchMark"/);
    assert.doesNotMatch(storyboard, /namedColor|Mig33LaunchLogo|customClass|userDefinedRuntimeAttribute/);
    await assert.rejects(readFile(join(oldCatalog, 'Contents.json')), { code: 'ENOENT' });
    for (const scale of [1, 2, 3]) {
      const suffix = scale === 1 ? '' : `@${scale}x`;
      const filename = `Mig33LaunchMark${suffix}.png`;
      const bytes = await readFile(join(app, filename));
      const source = `launch-mark${suffix}.png`;
      assert.deepEqual(bytes, await readFile(join(root, 'assets', source)));
      // A copied file must also be linked into the app's resource build phase.
      const entries = Object.entries(project.pbxFileReferenceSection()).filter(([, item]: [string, any]) =>
        item && typeof item === 'object' && String(item.path).replaceAll('"', '') === `mig33/${filename}`);
      assert.equal(entries.length, 1);
      const [imageRef, image] = entries[0] as [string, any];
      assert.equal(image.lastKnownFileType, 'image.png');
      assert.equal(image.explicitFileType, undefined);
      const imageBuilds = Object.entries(project.pbxBuildFileSection()).filter(([, item]: [string, any]) => item?.fileRef === imageRef);
      assert.equal(imageBuilds.length, 1);
      assert.equal(phase.files.filter((item: { value: string }) => item.value === imageBuilds[0][0]).length, 1);
      // PNG's IHDR records pixel dimensions independently of the resource name.
      assert.equal(bytes.readUInt32BE(16), 320 * scale);
      assert.equal(bytes.readUInt32BE(20), 182 * scale);
    }
    if (run === 1) {
      // Reproduce the previous orphaned build-phase reference, then check the
      // next prebuild repairs it instead of silently leaving an unbuilt file.
      const buildId = builds[0][0];
      delete project.pbxBuildFileSection()[buildId];
      delete project.pbxBuildFileSection()[`${buildId}_comment`];
      reference.lastKnownFileType = 'unknown';
      await writeFile(projectFile, project.writeSync());
    }
  }
});
