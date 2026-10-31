# Reviewed dependency backports

These patches are registered for exact package versions in
`../pnpm-workspace.yaml`; the lockfile records their hashes. No major versions
are overridden and no audit finding is ignored. Remove each patch after Expo
adopts a compatible release containing its fix, with the regression tests kept
as evidence until the dependency path is removed.

| Patch | Upstream change | Adaptation |
| --- | --- | --- |
| `glob@13.0.6.patch` | [4682e8c9babee1b385202ede36389634e12076d7](https://github.com/isaacs/node-glob/commit/4682e8c9babee1b385202ede36389634e12076d7), [GHSA-3g3m-2fjr-6pw4](https://github.com/advisories/GHSA-3g3m-2fjr-6pw4) | Four traversal callbacks return immediately when aborted; `onResume` retains callback completion after cancellation. The five upstream edits are applied to both distributed walker modules and the bundled public entry points. |
| `minimatch@10.2.6.patch` | [77a0f5c4b80bdc641c3c930a1d9edf5ad2db57f8](https://github.com/isaacs/minimatch/commit/77a0f5c4b80bdc641c3c930a1d9edf5ad2db57f8), [GHSA-g3hv-75wg-m249](https://github.com/advisories/GHSA-g3hv-75wg-m249) | Bound `**/..` preprocessing by `braceExpandMax` (default100,000), and skip quadratic deduplication above1,000 patterns. Applied to CommonJS and ESM without changing the v10 matching APIs. |

The reviewed upstream changes are translated into published JavaScript;
package metadata, dependency declarations, typings and unrelated algorithms
are unchanged. Original package licenses remain in their distributions.
The existing source maps are not regenerated; debugger source locations inside
these patched dependency internals may therefore be approximate.

`tests/security-dependencies.test.ts` resolves packages through the actual Expo
dependency graph and checks both module formats. Bounded probes fail before the
patch and pass afterwards; they avoid an unbounded denial-of-service payload.
Promise, streaming and synchronous traversal paths are checked, along with
normal Expo patterns and fingerprint exclusions. Raw `pnpm audit` continues to
report the original versions and all four findings; see
[dependency decisions](../DEPENDENCIES.md) for the unresolved findings.
