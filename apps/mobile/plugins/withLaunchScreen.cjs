const { copyFile, readFile, rm, writeFile } = require('node:fs/promises');
const { createRequire } = require('node:module');
const path = require('node:path');
const { IOSConfig, withFinalizedMod, withInfoPlist, withXcodeProject } = require('expo/config-plugins');
const expoRequire = createRequire(require.resolve('expo/config-plugins'));
const pluginsRequire = createRequire(expoRequire.resolve('@expo/config-plugins'));
const plist = pluginsRequire('@expo/plist').default;

const storyboardName = 'Mig33Launch';
const imageName = 'Mig33LaunchMark';
const imageFiles = ['', '@2x', '@3x'].map((suffix) => `${imageName}${suffix}.png`);

// UIKit renders this before React Native starts. Explicit dimensions also
// avoid depending on asset intrinsic-size resolution during launch snapshots.
// Expo retains ownership of the native cover's lifetime. Raw bundle resources
// and UIKit's system color also resolve in the OS launch-snapshot process.
function launchStoryboard() {
  return `<?xml version="1.0" encoding="utf-8"?>
<document type="com.apple.InterfaceBuilder3.CocoaTouch.Storyboard.XIB" version="3.0" toolsVersion="24093.7" targetRuntime="iOS.CocoaTouch" propertyAccessControl="none" useAutolayout="YES" launchScreen="YES" useTraitCollections="YES" useSafeAreas="YES" colorMatched="YES" initialViewController="MIG33-LAUNCH-CONTROLLER">
    <device id="retina6_12" orientation="portrait" appearance="light"/>
    <dependencies>
        <deployment identifier="iOS"/>
        <plugIn identifier="com.apple.InterfaceBuilder.IBCocoaTouchPlugin" version="24053.1"/>
        <capability name="Named colors" minToolsVersion="9.0"/>
        <capability name="Safe area layout guides" minToolsVersion="9.0"/>
        <capability name="documents saved in the Xcode 8 format" minToolsVersion="8.0"/>
    </dependencies>
    <scenes><scene sceneID="MIG33-LAUNCH-SCENE"><objects>
        <viewController id="MIG33-LAUNCH-CONTROLLER" sceneMemberID="viewController">
            <view key="view" contentMode="scaleToFill" id="MIG33-LAUNCH-ROOT">
                <rect key="frame" x="0" y="0" width="393" height="852"/>
                <autoresizingMask key="autoresizingMask" flexibleMaxX="YES" flexibleMaxY="YES"/>
                <subviews>
                    <imageView clipsSubviews="YES" userInteractionEnabled="NO" contentMode="scaleAspectFit" image="${imageName}" translatesAutoresizingMaskIntoConstraints="NO" id="MIG33-LAUNCH-IMAGE">
                        <rect key="frame" x="3" y="75" width="176" height="100.1"/>
                        <constraints>
                            <constraint firstAttribute="width" constant="176" id="MIG33-LAUNCH-WIDTH"/>
                            <constraint firstAttribute="height" constant="100.1" id="MIG33-LAUNCH-HEIGHT"/>
                        </constraints>
                    </imageView>
                </subviews>
                <viewLayoutGuide key="safeArea" id="MIG33-LAUNCH-SAFE"/>
                <color key="backgroundColor" systemColor="systemBackgroundColor"/>
                <constraints>
                    <constraint firstItem="MIG33-LAUNCH-IMAGE" firstAttribute="top" secondItem="MIG33-LAUNCH-SAFE" secondAttribute="top" constant="16" identifier="mig33.launch.top" id="MIG33-LAUNCH-TOP"/>
                    <constraint firstItem="MIG33-LAUNCH-IMAGE" firstAttribute="leading" relation="greaterThanOrEqual" secondItem="MIG33-LAUNCH-ROOT" secondAttribute="leading" constant="3" id="MIG33-LAUNCH-LEADING"/>
                    <constraint firstItem="MIG33-LAUNCH-IMAGE" firstAttribute="leading" secondItem="MIG33-LAUNCH-ROOT" secondAttribute="centerX" constant="-257" priority="750" id="MIG33-LAUNCH-COLUMN"/>
                </constraints>
            </view>
        </viewController>
        <placeholder placeholderIdentifier="IBFirstResponder" id="MIG33-LAUNCH-RESPONDER" sceneMemberID="firstResponder"/>
    </objects></scene></scenes>
    <resources>
        <image name="${imageName}" width="320" height="182"/>
        <systemColor name="systemBackgroundColor"><color white="1" alpha="1" colorSpace="custom" customColorSpace="genericGamma22GrayColorSpace"/></systemColor>
    </resources>
</document>
`;
}

function withLaunchScreen(config) {
  config = withInfoPlist(config, (mod) => {
    mod.modResults.UILaunchStoryboardName = storyboardName;
    return mod;
  });
  config = withXcodeProject(config, (mod) => {
    const project = mod.modResults;
    for (const basename of [`${storyboardName}.storyboard`, ...imageFiles]) {
      const filepath = path.join(mod.modRequest.projectName, basename);
      if (!project.hasFile(filepath)) {
        IOSConfig.XcodeUtils.addResourceFileToGroup({ filepath, groupName: mod.modRequest.projectName,
          project, isBuildFile: true });
      }
      // xcode's extension table does not recognize storyboard files. Set the
      // actual file type so Xcode compiles it, rather than copying XML as data.
      const [fileRef, reference] = Object.entries(project.pbxFileReferenceSection()).find(([, entry]) =>
        entry && typeof entry === 'object' && String(entry.path).replaceAll('"', '') === filepath) ?? [];
      if (!fileRef) throw new Error(`Could not register the native launch resource: ${basename}`);
      reference.lastKnownFileType = basename.endsWith('.storyboard') ? 'file.storyboard' : 'image.png';
      if (basename.endsWith('.storyboard')) reference.fileEncoding = 4;
      else delete reference.fileEncoding;
      delete reference.explicitFileType;
      const builds = project.pbxBuildFileSection();
      let build = Object.entries(builds).find(([, entry]) => entry && typeof entry === 'object' && entry.fileRef === fileRef)?.[0];
      const target = project.getTarget('com.apple.product-type.application').uuid;
      const resources = project.pbxResourcesBuildPhaseObj(target);
      // Repair an earlier prebuild that registered a reference without a build
      // file. Do not remove valid unrelated resources or duplicate this one.
      resources.files = resources.files.filter((entry) => entry.comment !== `${basename} in Resources` || builds[entry.value]);
      if (!build) {
        build = project.generateUuid();
        project.addToPbxBuildFileSection({ uuid: build, fileRef, basename, group: 'Resources' });
      }
      if (!resources.files.some((entry) => entry.value === build)) {
        project.addToPbxResourcesBuildPhase({ uuid: build, fileRef, basename, group: 'Resources', target });
      }
    }
    return mod;
  });
  return withFinalizedMod(config, ['ios', async (mod) => {
    const app = path.join(mod.modRequest.platformProjectRoot, mod.modRequest.projectName);
    // Expo's splash plugin also sets this key, and ordinary mods run in reverse
    // registration order. Apply our final choice after those mods, both on disk
    // and in the config consumed by the build-specific networking finalizer.
    mod.ios.infoPlist.UILaunchStoryboardName = storyboardName;
    for (const filename of ['Info.plist', 'Info.Debug.plist']) {
      const destination = path.join(app, filename);
      let contents;
      try { contents = plist.parse(await readFile(destination, 'utf8')); }
      catch (error) { if (error.code === 'ENOENT' && filename === 'Info.Debug.plist') continue; throw error; }
      await writeFile(destination, plist.build({ ...contents, UILaunchStoryboardName: storyboardName }));
    }
    const source = path.join(mod.modRequest.projectRoot, 'assets');
    for (const suffix of ['', '@2x', '@3x']) {
      await copyFile(path.join(source, `launch-mark${suffix}.png`), path.join(app, `${imageName}${suffix}.png`));
    }
    // Remove only the obsolete asset set generated by this plugin. Its image
    // and named-color lookups failed in the OS snapshot process on iOS27.
    await rm(path.join(app, 'Images.xcassets', 'Mig33LaunchLogo.imageset'), { recursive: true, force: true });
    await writeFile(path.join(app, `${storyboardName}.storyboard`), launchStoryboard());
    return mod;
  }]);
}

module.exports = withLaunchScreen;
module.exports.launchStoryboard = launchStoryboard;
