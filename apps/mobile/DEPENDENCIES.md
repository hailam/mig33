# Mobile dependency decisions

Reviewed on 2026-10-31. This records the installed dependency graph and remaining work; it is not a security exception or a clean-audit claim.

## Node and Expo

The app has its own mise configuration, pnpm workspace policy and lockfile. It does not create a JavaScript workspace at the Go repository root. Node 26.12.1 and pnpm 12.16.1 are pinned in `mise.toml`.

`pnpm-workspace.yaml` requires packages to be at least 4,320 minutes old, checks the age of locked dependencies, rejects missing publication times and exotic subdependencies, and permits no dependency build scripts. Exact advisory-backed release-age exceptions must follow the repository's Node policy and be recorded in `security-exceptions.json`; that file is currently empty. Age checks do not replace vulnerability auditing.

The selected Expo SDK 58 tuple is Expo 58.0.20, React 19.3.0, React Native 0.88.0, Expo Dev Client 58.0.16 and Expo Build Properties 58.0.11. These match the selected Expo release's [bundled native module manifest](https://unpkg.com/expo@58.0.20/bundledNativeModules.json). At selection time, Expo 58.0.21 and Expo Dev Client 58.0.17 had been published less than 72 hours earlier. Their newer recommendations do not justify bypassing the age policy.

The v4 Login form adds `expo-secure-store@58.0.2`, matching Expo 58.0.20's
bundled `~58.0.2` recommendation. It was published on 2026-10-01 at 19:30:00 UTC
and installed under the unchanged three-day policy. Remember Login uses
`WHEN_UNLOCKED_THIS_DEVICE_ONLY`; no plaintext password file is written.
Appearance uses a separate preference key, so forgetting login does not reset
the selected theme.
The package's [config plugin](https://docs.expo.dev/versions/v58.0.0/sdk/securestore/)
is included in `app.config.ts`. Native gradients use React Native's existing
`backgroundImage` support, with no gradient or icon package added.

Modern iOS controls use `expo-glass-effect@58.0.5`, matching Expo's bundled
`~58.0.5` recommendation. Its registry publication time is
`2026-10-23T23:23:40.402Z`; it passed the unchanged three-day policy on October31.
The [native GlassView](https://docs.expo.dev/versions/v58.0.0/sdk/glass-effect/)
uses Apple's Liquid Glass on supported iOS versions, with runtime availability
checks and a solid fallback for reduced transparency or unsupported platforms.
Motion uses React Native's existing Animated API; no animation package is added.

Modern iOS menus, alerts and adaptive sheets add `@expo/ui@58.0.24`, matching
Expo58.0.20's bundled `~58.0.24` recommendation. The registry publication time is
`2026-10-27T12:16:28.415Z`; installation passed the unchanged72-hour policy.
The newer58.0.25 was still underage when selected. The only new transitive
package is `sf-symbols-typescript@2.2.0`; optional worklets and React DOM peers are
not installed. [SwiftUI Menu](https://docs.expo.dev/versions/v58.0.0/sdk/ui/swift-ui/menu/)
provides an actual anchored menu, and
[BottomSheet](https://docs.expo.dev/versions/v58.0.0/sdk/ui/swift-ui/bottomsheet/)
provides system detents and a dismissal-completed callback. Controlled
[Alert](https://docs.expo.dev/versions/v58.0.0/sdk/ui/swift-ui/alert/) has a
presented-state callback but no dismissal-completed callback. Platform-specific
modules isolate these native hosts from the existing Android/Classic fallbacks.
Native presentations retain their hosts across appearance/accessibility changes;
the system handles Reduce Motion, and sheets use an opaque native background
when Reduce Transparency or Increase Contrast is enabled. This dependency
requires a new native development build before the new controls can run.
The raw audit after installation reports the same four findings below; this
addition introduced no new audit finding. Release-age exceptions remain empty.

Photo sharing adds `expo-image-picker@58.0.10` (published
`2026-10-01T19:30:35.206Z`) and `expo-image-manipulator@58.0.15` (published
`2026-10-19T14:35:08.571Z`). Both match the installed Expo bundled module
manifest and passed the unchanged three-day policy. The system
[photo picker](https://docs.expo.dev/versions/latest/sdk/imagepicker/) requests
camera permission only when taking a photo; its plugin disables microphone
permission. The [image manipulator](https://docs.expo.dev/versions/latest/sdk/imagemanipulator/)
converts selected photos to JPEG and bounds dimensions/bytes before a user
confirms sending through the original protocol. Legacy service pages use native
controls and the original922/923 requests; no browser or HTML package was added.

```sh
mise -C apps/mobile install node pnpm
mise -C apps/mobile exec -- pnpm install --frozen-lockfile
mise -C apps/mobile exec -- pnpm run check
mise -C apps/mobile exec -- pnpm audit
EXPO_NO_TELEMETRY=1 mise -C apps/mobile exec -- pnpm run doctor
```

## Outstanding audit findings

The audit exits nonzero with one high, two moderate and one low finding. None is
suppressed. The two moderate findings have narrowly scoped local backports,
described below; the audit still reports their unchanged package versions.

| Package | Advisory | Dependency path | Status |
| --- | --- | --- | --- |
| `zod@3.25.76` | [GHSA-mrhx-c7xc-m3w5](https://github.com/advisories/GHSA-mrhx-c7xc-m3w5), high | `expo > @expo/cli > zod` | The advisory reports versions below 4.6.6 affected; Expo CLI requires Zod 3. |
| `deepmerge@4.3.1` | [GHSA-gcqx-w48m-w2h4](https://github.com/advisories/GHSA-gcqx-w48m-w2h4), low | `expo > @expo/config > deepmerge`, plus build-tool paths | The advisory lists no patched release. |
| `minimatch@10.2.6` | [GHSA-g3hv-75wg-m249](https://github.com/advisories/GHSA-g3hv-75wg-m249), moderate | `expo > @expo/fingerprint > minimatch`, and Expo's `glob` paths | Version-scoped upstream-derived patch bounds preprocessing; raw audit remains visible. |
| `glob@13.0.6` | [GHSA-3g3m-2fjr-6pw4](https://github.com/advisories/GHSA-3g3m-2fjr-6pw4), moderate | `expo > @expo/cli > glob`, config, config-plugins, fingerprint and Metro tooling | Version-scoped upstream-derived patch stops traversal after cancellation; raw audit remains visible. |

The Zod finding concerns excessive validation work and memory use for oversized collections. Inspection of installed `@expo/cli@58.1.16` found its Zod use in the development-tool plugin schemas: locally discovered plugin metadata and JSON output from plugin CLI commands are validated with `safeParse`. These are development tooling paths; the mobile protocol implementation does not import Zod. This narrows the observed exposure but does not prove the tooling cannot be attacked.

The upstream [Zod fix](https://github.com/colinhacks/zod/pull/6652) changes Zod 4 collection parsers to reject collections exceeding configured upper size bounds before parsing their elements. It does not bound unconstrained collections. The inspected Expo schemas have arrays without upper bounds. The fix therefore cannot simply be copied into Zod 3 and called a complete remediation for these paths. No supported Zod 3 backport was found. Expo 58.0.21's [CLI 58.1.17 manifest](https://unpkg.com/@expo/cli@58.1.17/package.json) still declares `zod: ^3.25.76`, so upgrading to that underage Expo patch would not resolve the finding. An untested Zod 3-to-4 override has not been introduced.

The deepmerge finding concerns prototype poisoning of merged results. The observed paths are Expo configuration and build tooling. No application protocol code imports it. A supported upstream replacement or fix still needs evaluation; downgrading outside the reported range alone is not evidence of a safe replacement.

Recheck both findings when updating Expo. A compatible upstream fix or a narrowly tested upstream-derived patch must be verified before either finding is marked resolved. `security-exceptions.json` is for temporary release-age exceptions, not vulnerability waivers.

### Compatible backports for glob and minimatch

Both moderate advisories entered the GitHub advisory database on October30.
The same `glob@13.0.6` and `minimatch@10.2.6` versions were already in the
baseline lockfile; the photo and appearance dependencies did not introduce them.
These packages are used by Expo development/build tooling, not the mobile chat
protocol. This identifies the observed paths without claiming that local
tooling inputs are always trustworthy.

The npm registry reports patched releases `glob@14.0.2` at
`2026-10-27T22:07:00.325Z` and `minimatch@11.0.1` at
`2026-10-27T22:26:03.755Z`, both older than72 hours when reviewed. However,
[minimatch11](https://github.com/isaacs/minimatch/blob/v11.0.1/changelog.md)
removes APIs and replaces regular-expression matchers, while
[glob14](https://github.com/isaacs/node-glob/blob/v14.0.2/changelog.md)
changes matching and ignore semantics and upgrades path-scurry's major version.
The current Expo CLI58.1.17 still requests glob13. A major-version override is
therefore not a drop-in security update.

The exact-version [pnpm patches](patches/README.md) adapt the two advisory-linked
upstream fixes to the installed distributions. They add no dependencies, do not
execute install scripts, and leave the release-age policy and its empty exception
list unchanged. Both CommonJS and ESM public entry points are covered, including
glob's bundled entry points and its unbundled walker modules.

The regression tests first reproduced both defects on the unpatched graph:
256 patterns despite a64-pattern limit, and11 directory reads after cancellation
at the first read. With the patches, the cap is64 and traversal makes only the
initial read. Tests also cover larger-set deduplication, normal Expo patterns,
the retained minimatch10 API and Expo fingerprint exclusion exceptions.
Type checking, effective policy checks, Expo configuration and iOS export are
required checks for this backport. Keep all four advisories visible in raw audit
output; these patches mitigate the two specific moderate defects, not the
remaining Zod/deepmerge findings or every possible resource-exhaustion case.

## Native TCP and current validation

The [react-native-tcp-socket](https://github.com/Rapsssito/react-native-tcp-socket) package is pinned to 6.4.3. Original raw TCP requires a development build containing that native module; Expo Go cannot supply it. Expo SDK 58 uses React Native's [New Architecture](https://docs.expo.dev/guides/new-architecture/), and Expo Doctor reports this package as untested against it. That warning remains visible.

TypeScript checks, protocol/session/transport tests, iOS prebuild, native build
and an iOS Hermes bundle export have passed. XCTest on the iPhone18 Pro
simulator with iOS27.2 and Xcode27.2 verifies native HTTP/TCP login and private
messaging, plus SecureStore save/restore/delete. This establishes operation in
that simulator configuration; it does not establish Android or physical-device
compatibility. The Expo Doctor warning remains visible.

Expo Doctor also reports that newer SDK patch versions are recommended. Review this separately from native-module compatibility: the current exact pins were selected under the release-age policy, and no version-warning exclusions have been added.

## Optional iOS build tools

`mise.ios.toml` adds Ruby only when the `ios` profile is selected. It pins the supported [Ruby 3.4.12 release](https://www.ruby-lang.org/en/downloads/), uses its bundled Bundler 2.6.9, and requires a precompiled Ruby instead of falling back to a source build. mise documents its precompiled Ruby distribution and verification [here](https://mise.jdx.dev/lang/ruby.html#precompiled-binaries).

The app-root `Gemfile` pins [CocoaPods 1.17.0](https://rubygems.org/gems/cocoapods/versions/1.17.0); `Gemfile.lock` records its transitive gems. This follows CocoaPods' [Bundler guidance](https://guides.cocoapods.org/using/a-gemfile.html). The Gemfile lives outside the generated `ios/` directory so Expo prebuild can recreate that directory. Gems and Bundler settings are stored under ignored `data/mobile-gems/`, without modifying system Ruby. The installed Expo CLI discovers the app-root Gemfile and invokes CocoaPods through Bundler.

```sh
mise -C apps/mobile -E ios install node pnpm ruby
mise -C apps/mobile -E ios exec -- bundle install
mise -C apps/mobile -E ios exec -- bundle exec pod --version
# After installing Xcode and an iOS Simulator runtime:
EXPO_NO_TELEMETRY=1 mise -C apps/mobile -E ios exec -- pnpm ios
```

Full Xcode, its first-launch setup and an installed iOS Simulator runtime are required for the final command. Apple's standalone Command Line Tools do not provide the iOS SDK or Simulator.
