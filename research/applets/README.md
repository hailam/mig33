# Applet reference library

The recovered targets are mig33 **3.08** and **4.62 build .000**. Their publisher JARs are preserved locally with recorded hashes. Decompiled source is a reading aid; original bytecode remains the authority.

To obtain the uncommitted binaries for a fresh checkout, follow [local applet setup](../../tools/applets/SETUP.md). It includes verified publisher-archive URLs, the emulator download, exact destinations and checksums.

| Reference | v3.08 | v4.62 |
| --- | --- | --- |
| Screens and menus | [Screen specification](v3/docs/screens.md) | [Screen specification](v4/docs/screens.md) |
| Protocol/API | [Protocol](v3/docs/protocol.md) | [Endpoints](v4/docs/endpoints.md), [packets](v4/docs/protocol.md) |
| Drawing and layout | [Rendering](v3/docs/rendering.md) | [Rendering](v4/docs/rendering.md) |
| Assets | [Atlas](v3/docs/assets.html) | [Atlas](v4/docs/assets.html), [theme fields](v4/docs/theme-fields.md) |
| Capture method | [v3 capture](v3/docs/capture.md) | [v4 capture](v4/docs/capture.md) |
| Download identity | [Provenance](v3/docs/provenance.md) | [Provenance](v4/docs/provenance.md) |
| Structured catalogs | [v3 data](v3/docs/data/) | [v4 data](v4/docs/data/) |

The [v4 protocol source audit](v4/docs/protocol-catalog-audit.md) checks the catalogs against original bytecode and offline original-class probes, with per-operation evidence and explicit extraction limits.

Each version has `docs/` for maintained references, `artifacts/` for recovered/derived local inputs, and `discovery/` for download/source investigation. Entire `artifacts/` directories are Git-ignored. Research catalogs and capture manifests live under `docs/data/`; small JSON/Markdown discovery metadata remains eligible for Git.

The `file` entries in `screenshots-240x320.json` resolve under the same version's local `artifacts/screenshots/240x320/` directory. Image `file` entries in v4's `docs/data/historical-pages/catalog.json` resolve under local `artifacts/historical-pages/`. Moving the metadata does not move or alter those image files.

Cross-version material lives in `shared/`: [comparison](shared/comparison.md), [visual comparison](shared/comparison.html), [protocol differences](shared/protocol-differences.md), [history/popularity](shared/history-and-popularity.md), [methodology](shared/methodology.md), and [backend inferences](shared/backend-inferences.md).

The [historical-page design catalog](v4/docs/historical-page-designs.md) distinguishes recovered designs from inferred replacements. Images and screenshot galleries referenced by these documents require the local artifact collection.

Executable capture, extraction and test code lives in [tools/applets](../../tools/applets/README.md). Original-client service acceptance is recorded separately in [acceptance/v4](../acceptance/v4/README.md).
