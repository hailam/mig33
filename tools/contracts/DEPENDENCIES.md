# Contract validator dependency decisions

Reviewed on 2026-10-31. This package scope runs development-time contract
validation; it is independent of the Go server and Expo dependency graph.

## Spectral and minimatch

The registry audit reported
[GHSA-g3hv-75wg-m249](https://github.com/advisories/GHSA-g3hv-75wg-m249)
through `@asyncapi/parser@3.6.3` → `@stoplight/spectral-core@1.24.0` →
`minimatch@3.1.5`. The advisory's affected range is below 11.0.1; its detailed
reproducer concerns optimization level 2 preprocessing in 10.2.6. We do not infer
that every older version has that exact implementation, or suppress the audit
because this validator currently uses trusted local documents.

Registry metadata checked at 2026-10-31T13:35Z showed that the latest Spectral
core release was still 1.24.0 with `minimatch: ^3.1.4`. The latest AsyncAPI parser,
3.6.6, still depended on Spectral core 1.x and did not remove this dependency.
Updating the parser alone therefore would not address the finding.

The fix selects the upstream `minimatch@11.0.2` release through an exact,
parent-specific override for `@stoplight/spectral-core@1.24.0`. Its registry
publication timestamp is `2026-10-28T01:08:11.021Z`, over 84 hours before review.
Version 11.0.3, published `2026-10-28T20:00:34.713Z`, had not cleared 72 hours at
review time. The selected release supports our pinned Node 26; installation
continues to enforce the three-day policy on transitive dependencies. There is
no release-age exception or audit exclusion.

This major-version change requires an explicit compatibility adapter. Spectral
has one minimatch wrapper, used by file-level and JSON-pointer rule overrides.
Its [upstream source](https://github.com/stoplightio/spectral/blob/v6.17.0/packages/core/src/ruleset/utils/minimatch.ts)
calls the old default function with `{ matchBase: true }`. Minimatch 11 exposes a
named `minimatch` function; a bare override would cause a runtime TypeError.
The [version-specific patch](patches/@stoplight__spectral-core@1.24.0.patch)
changes only that distributed wrapper's import and call, removing the unused
interop helper. It preserves the options, return value and declaration. It
does not change Spectral's rules or bypass validation. The original source map
is retained, so debugger locations inside this tiny patched wrapper can differ.

The lockfile records the exact release integrity and Spectral patch hash.
`security-dependencies.test.mjs` resolves packages through the actual AsyncAPI
parser, checks the selected version, exercises basename/globstar/brace/extglob
and exclusion matching, and verifies both Spectral override call sites. Its
small preprocessing probe checks the cap introduced by the
[upstream fix](https://github.com/isaacs/minimatch/commit/77a0f5c4b80bdc641c3c930a1d9edf5ad2db57f8).
Matching and override tests passed against the original graph before the change;
all four tests pass with the updated graph, alongside complete AsyncAPI/schema
and TypeScript validation. The registry audit reports no vulnerabilities.

Recheck with:

```sh
mise -C tools/contracts exec -- pnpm install --frozen-lockfile
mise -C tools/contracts exec -- pnpm run check
mise -C tools/contracts exec -- pnpm audit
```

Remove the override and patch together when a reviewed Spectral release adopts
a fixed minimatch version with a supported import. Keep the matching/override
regressions; adjust the version guard only after reviewing the replacement.
