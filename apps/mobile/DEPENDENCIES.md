# Mobile dependency decisions

Reviewed on 2026-10-30 UTC. This records the installed dependency graph and remaining work; it is not a security exception or a clean-audit claim.

## Node and Expo

The app has its own mise configuration, pnpm workspace policy and lockfile. It does not create a JavaScript workspace at the Go repository root. Node 26.12.1 and pnpm 12.16.1 are pinned in `mise.toml`.

`pnpm-workspace.yaml` requires packages to be at least 4,320 minutes old, checks the age of locked dependencies, rejects missing publication times and exotic subdependencies, and permits no dependency build scripts. Exact advisory-backed release-age exceptions must follow the repository's Node policy and be recorded in `security-exceptions.json`; that file is currently empty. Age checks do not replace vulnerability auditing.

The selected Expo SDK 58 tuple is Expo 58.0.20, React 19.3.0, React Native 0.88.0, Expo Dev Client 58.0.16 and Expo Build Properties 58.0.11. These match the selected Expo release's [bundled native module manifest](https://unpkg.com/expo@58.0.20/bundledNativeModules.json). At selection time, Expo 58.0.21 and Expo Dev Client 58.0.17 had been published less than 72 hours earlier. Their newer recommendations do not justify bypassing the age policy.

```sh
mise -C apps/mobile install node pnpm
mise -C apps/mobile exec -- pnpm install --frozen-lockfile
mise -C apps/mobile exec -- pnpm run check
mise -C apps/mobile exec -- pnpm audit
EXPO_NO_TELEMETRY=1 mise -C apps/mobile exec -- pnpm run doctor
```

## Outstanding audit findings

The audit of the committed lockfile exits nonzero with one high and one low finding. Neither is suppressed.

| Package | Advisory | Dependency path | Status |
| --- | --- | --- | --- |
| `zod@3.25.76` | [GHSA-mrhx-c7xc-m3w5](https://github.com/advisories/GHSA-mrhx-c7xc-m3w5), high | `expo > @expo/cli > zod` | The advisory reports versions below 4.6.6 affected; Expo CLI requires Zod 3. |
| `deepmerge@4.3.1` | [GHSA-gcqx-w48m-w2h4](https://github.com/advisories/GHSA-gcqx-w48m-w2h4), low | `expo > @expo/config > deepmerge`, plus build-tool paths | The advisory lists no patched release. |

The Zod finding concerns excessive validation work and memory use for oversized collections. Inspection of installed `@expo/cli@58.1.16` found its Zod use in the development-tool plugin schemas: locally discovered plugin metadata and JSON output from plugin CLI commands are validated with `safeParse`. These are development tooling paths; the mobile protocol implementation does not import Zod. This narrows the observed exposure but does not prove the tooling cannot be attacked.

The upstream [Zod fix](https://github.com/colinhacks/zod/pull/6652) changes Zod 4 collection parsers to reject collections exceeding configured upper size bounds before parsing their elements. It does not bound unconstrained collections. The inspected Expo schemas have arrays without upper bounds. The fix therefore cannot simply be copied into Zod 3 and called a complete remediation for these paths. No supported Zod 3 backport was found. Expo 58.0.21's [CLI 58.1.17 manifest](https://unpkg.com/@expo/cli@58.1.17/package.json) still declares `zod: ^3.25.76`, so upgrading to that underage Expo patch would not resolve the finding. An untested Zod 3-to-4 override has not been introduced.

The deepmerge finding concerns prototype poisoning of merged results. The observed paths are Expo configuration and build tooling. No application protocol code imports it. A supported upstream replacement or fix still needs evaluation; downgrading outside the reported range alone is not evidence of a safe replacement.

Recheck both findings when updating Expo. A compatible upstream fix or a narrowly tested upstream-derived patch must be verified before either finding is marked resolved. `security-exceptions.json` is for temporary release-age exceptions, not vulnerability waivers.

## Native TCP and current validation

The [react-native-tcp-socket](https://github.com/Rapsssito/react-native-tcp-socket) package is pinned to 6.4.3. Original raw TCP requires a development build containing that native module; Expo Go cannot supply it. Expo SDK 58 uses React Native's [New Architecture](https://docs.expo.dev/guides/new-architecture/), and Expo Doctor reports this package as untested against it. That warning remains visible.

TypeScript checks, protocol/session/transport tests, iOS prebuild without installing Pods, native autolinking discovery and an iOS Hermes bundle export have passed. These checks do not establish native TCP operation. An iOS Simulator build and live HTTP/TCP checks remain required. Android native build compatibility has not been verified.

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
