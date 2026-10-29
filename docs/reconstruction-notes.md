# Reconstruction notes and milestones

This retains the feature milestones and reference overview assembled during reconstruction. Start with the [project README](../README.md) for the current layout and run commands.

Recovered and analyzed **mig33 3.08** and **mig33 4.62 build .000**. Both original JARs match archived downloads from the publisher byte for byte. The references cover the binary protocol, screen and menu contents, input constraints, navigation, drawing rules, original assets, localization, and differences between versions.

**Start with the [version comparison](../research/applets/shared/comparison.md), [visual comparison](../research/applets/shared/comparison.html), or the version-specific screen specifications below.**

The [eleven core slash commands](slash-commands.md) now generate native action messages in private chat, private groups and public rooms. Try `/me waves hello`, `/hug bob` or `/roll`. Results persist for history/offline delivery and return to the author. Historical command evidence, replacement wording and unresolved moderation scope are documented.

The [full embedded-page catalog and local services](service-pages.md) now cover registration with development OTP **123456**, persistent profiles/settings, community Groups/forums, test credits, gifts/themes, photos, activity updates, mail and provider-request pages. HTML comes from **typed templ components**, with shared forms, links and a single legacy serialization adapter. [Native acceptance](../research/acceptance/v4/PAGES.md) checks all52 original URL templates and real forms in the unchanged applet. External payments, SMS, calls and IM networks are not connected.

[Historical formatting](text-and-page-formatting.md#implemented-profile-customization) now includes a persistent **Customize Profile** editor and formatted chat at **http://127.0.0.1:8080/member2/t.php**. Existing account credentials work there. Original profile guides and seven additional screenshots are recovered; mixed chat styling is retained on WAP while the Java client receives its supported plain-text/single-colour presentation. Stock4.62 displays profile colours and inline photos; background images require a web browser.

The [native formatting acceptance](../research/acceptance/v4/formatting-result.md) passed over TCP and HTTP with38 screenshots and160 frames. Schema17 preserves all prior records and adds rich-message storage; the live migration retained18 messages and five accounts.

The backend completion pass adds [room search/admin and Group commands](room-completion.md), [operator review](operator-review.md), phone/external contacts, [downloadable media effects and English language download](native-services.md), live copies to your other sessions, and [idempotent JSON sends/read positions](chat.md). Enable the operator dashboard with `-operators hailam` and open `/operator`; it is disabled by default. Provider calls remain explicit local requests.

[Browse120 recovered historical images](../research/applets/v4/docs/historical-pages.html):20 official-host illustrations,7 community profiles,76 product captures and17 game captures. The [design coverage matrix](../research/applets/v4/docs/historical-page-designs.md) now records13 partial page matches,17 navigation-only matches and22 without visuals; none is claimed to be an exact4.62 server-page archive.

The [expanded catalog](community-commerce.md) has78 local products using recovered names or existing replacements, with explicit placeholder art and test prices. Games lists23 historical reference entries beside2 playable local replacements. Public `GET /api/catalog` exposes products, emoticons, games, recovered languages and mock-provider modes for the future frontend.

Historical `/gift` discovery, sending with notes and ratings now work with the test-credit ledger. `/games` and `/bot stop` follow recovered help syntax. [Native catalog acceptance](../research/acceptance/v4/catalog-result.md) passed with53 screenshots and203 frames; schema18 preserved all existing records during the live upgrade. New room names follow the recovered15-character limit, while existing longer names remain usable.

## Reconstruction direction

The current target is the **local v4.62 backend**. SMS/OTP, external email, calls and payments stay mocked; IM remains disconnected/TBD. Recovered behavior is implemented where evidence exists, with explicit inferred replacements and reference placeholders elsewhere. Expo and its **Settings → Experience → Classic v3** presentation option are deferred UI work sharing this backend, not a separate backend milestone.

The [Go compatibility backend](backend.md) now prioritizes **direct messages and public chat rooms**, with private group chat also supported. [Chat history, offline private/group delivery and membership](chat.md) persist in SQLite. [Public rooms](rooms.md) support browsing, joining, participants, room text, leaving, moderator roles, kicks and persistent mute/unmute. Run `go run ./cmd/omni`; defaults are HTTP `127.0.0.1:8080`, TCP `127.0.0.1:9119`, and database `data/chat.sqlite`. Test accounts are `alice`, `bob`, `carol`, `dave`, each with password `<username>-demo`, plus `hailam` with password `hailam`; Alice starts as room moderator. The new frontend and Classic v3 settings switch remain to be built.

The executable entrypoints are `cmd/http`, `cmd/socket`, `cmd/omni` and `cmd/cli`. Each constructs its selected transports; `cmd/internal/wiring` shares core construction and resource ownership. `internal/bootstrap` only coordinates supplied lifecycle hooks. `omni` serves HTTP and the native socket protocol using one shared capability graph and realtime coordinator; `http` and `socket` serve their respective transports, and `cli` runs operator actions without listeners. [YAML configuration and overlays](../configs/README.md) define defaults, environment selection and `MIG33_*` overrides. The [OpenAPI contract](../api/openapi.yaml) documents the HTTP surface, including the original query-selected pages and binary tunnel.

The [application architecture](backend.md#composition-routing-and-persistence) now has capability-owned types, concrete services and repository ports for accounts, contacts, chat, rooms, community, commerce, catalog, media, activity and review. Controllers consume explicit behavior contracts. One realtime coordinator owns sessions, live participation and ordered delivery; native codecs and the legacy-web adapter own packet/page translation. SQLite/sqlc retains the existing atomic registration, message and gift workflows. The former all-feature service, domain-alias package and aggregate ports have been removed.

The retained [Go dependency, context and bootstrap research](../research/architecture/go-backend-architecture.md) compares explicit wiring, lifecycle libraries and Fx, with primary sources and the original source audit. Its accepted recommendation uses explicit constructors, a lifecycle-only bootstrap and caller contexts through services, repository ports and sqlc. The [backend architecture](backend.md#composition-routing-and-persistence) describes the implementation and cancellation limits.

The retained [command package research](../research/architecture/go-domain-boundaries.md) and [ports/adapters migration study](../research/architecture/go-hexagonal-migration.md) explain the implemented boundaries and alternatives. All 179 original service test names are [mapped to the new suites](../research/architecture/evidence/capability-test-migration.json). Full race tests, vet, architecture guards, generator/schema checks and all 17 original-client suites pass. The [implementation record](../research/architecture/evidence/go-capability-implementation.json) retains the 48 client scenarios, source/binary hashes, regression fixes and documented harness corrections.

To open the **original v4 app interactively**, start the backend, then run `sh scripts/open-client.sh alice`. Omit `alice` to start at Login, or use `bob` for a second client. This opens MicroEmulator's desktop phone with the unchanged original JAR, routed to the local backend. Use the clickable phone keypad, arrow keys, Enter, and F1/F2 for the left/right softkeys (Fn+F1/F2 on some Macs). The phone skin uses its native176×220 display. Each launch gets isolated, in-memory client settings and a new trace directory under `data/desktop/`; backend chat data persists in `data/chat.sqlite`. The launcher needs a graphical desktop and a JDK, using the same Java overrides as the integration scripts. On macOS a sandboxed process may require permission to open a GUI. This is an emulator preview; the new frontend remains unimplemented.

[Contact management](contacts.md) now supports saved add/remove/edit/move operations and contact groups. Hailam starts with an empty Friends list. Run `sh scripts/open-clients.sh` to open two independent signed-out clients side by side, or `sh scripts/open-clients.sh hailam bob` to sign into both explicitly.

New acceptance runs cover [media and languages](../research/acceptance/v4/media-result.md), [retained-tab reconnects](../research/acceptance/v4/reconnect-result.md), and [simultaneous sessions/phone and external contacts](../research/acceptance/v4/mirrors-result.md):51 screenshots and742 wire frames in total, with all8 client runs logged out.

The [original-client integration test](../research/acceptance/v4/README.md) connects two unchanged v4.62 clients, using TCP and HTTP, and verifies login, contacts, messages in both directions and a rendered profile page. Its guide links the captured packets and screenshots.

The [four-client group test](../research/acceptance/v4/GROUPS.md) verifies group creation, invitations, participant lists, delivery to every member, leaving and unauthorized-send rejection through the original v4 client.

The [public-room test](../research/acceptance/v4/ROOMS.md) verifies the original room browser, joining, occupancy, participant lists, messages and leaving. The [moderation test](../research/acceptance/v4/ROOM_MODERATION.md) exercises moderator and mute markers, permission checks and kicks, and documents the separate stock personal-block action.

[Personal Block & Mute](blocking.md) now persists and filters incoming text, with unblocking through the original Manage Block List page. The [target and gap audit](target-and-gaps.md) defines the intended v4 chat experience, remaining core functionality and the later Classic v3 profile.

[Report Abuse](reports.md) now saves reports for local operator review. Regular users can start/join a vote through **room Menu → Kick User**; the [required count scales with room occupancy](rooms.md#regular-user-vote-kicking), from three to nine under the documented local policy, within60 seconds. Moderators kick immediately. The [five-client acceptance guide](../research/acceptance/v4/VOTE_REPORT.md) covers both features. The [native core-flow audit](core-flow-audit.md) records corrected presence and group-restoration bugs, working offline-chat entry and validated reconnect/media behavior and remaining physical-device limits.

[Room discovery](room-discovery.md) now supports **Search Rooms**, **Create Room**, **Room Info** with moderator mute/unmute controls and saved **Favorites** through the original menus and URLs. A newly created public room belongs to its creator, who receives moderator rights. Room definitions and per-account favorites survive restart. The new pages replace unavailable historical server HTML. [Search New Friends and Profile → Chat](friend-discovery.md) now provide directory, Add Friend and offline-DM entry. The [applet inference audit](../research/applets/shared/backend-inferences.md) records the supporting bytecode and separates confirmed contracts from unknown server policy.

| Reference | v3.08 | v4.62 keypad |
| --- | --- | --- |
| Screens, menus, actions and validation | [Screen specification](../research/applets/v3/docs/screens.md) | [Screen specification](../research/applets/v4/docs/screens.md) |
| API and binary protocol | [Protocol specification](../research/applets/v3/docs/protocol.md) | [Endpoint catalog](../research/applets/v4/docs/endpoints.md), [packet specification](../research/applets/v4/docs/protocol.md) |
| Colors, dimensions, typography and layout | [Rendering specification](../research/applets/v3/docs/rendering.md) | [Rendering specification](../research/applets/v4/docs/rendering.md) |
| Original graphics | [Asset atlas](../research/applets/v3/docs/assets.html) | [Theme assets](../research/applets/v4/docs/assets.html), [all theme fields](../research/applets/v4/docs/theme-fields.md) |
| Original-bytecode screenshots | [46 captures](../research/applets/v3/artifacts/screenshots/240x320/index.html) | [39 captures](../research/applets/v4/artifacts/screenshots/240x320/index.html) |
| Screenshot profile and reproduction | [Capture notes](../research/applets/v3/docs/capture.md) | [Capture notes](../research/applets/v4/docs/capture.md) |
| Artifact identity and source history | [Provenance](../research/applets/v3/docs/provenance.md) | [Provenance](../research/applets/v4/docs/provenance.md) |
| Structured evidence catalogs | [JSON/data](../research/applets/v3/docs/data) | [JSON/data](../research/applets/v4/docs/data) |

[Exact protocol differences](../research/applets/shared/protocol-differences.md) complement the broader comparison. [Version history and popularity evidence](../research/applets/shared/history-and-popularity.md) explains why **v3's alleged “most popular” status is unverified** and why **4.62 is the latest publisher-verified mig33 Java keypad build recovered here**, rather than a claim about every platform or later migme successor.

## What is exact and what remains external

Original JARs, JADs, resources, class disassembly, theme values and string packs are preserved. Screenshots invoke original unmodified application classes in an offline emulator. Fixed client forms and menus are documented with original text, order, constraints and source references.

Historical server-generated profile/account/shop/help pages, user-specific menus, emoticon packs, photos and live account data are not fully contained in either JAR. Their client-visible protocols and renderers are documented; missing historical payloads are explicitly identified. Synthetic data in captures is labeled as a fixture. Native text editing, command bars, camera preview and font rasterization require a chosen handset/emulator profile. **This package cannot honestly promise pixel-identical reconstruction of every historical server screen on every handset.**

The recovered classes are obfuscated. Decompiled output is a reading aid, not a ready-to-build original source tree. v4 additionally includes a clearly marked analysis-only normal-flow decompilation; its exception tables were omitted to make otherwise unreadable methods understandable. That derivative was never executed. Original bytecode remains the authority for exceptional behavior.

## Preserved binaries

| Build | Size | SHA-256 |
| --- | ---: | --- |
| [3.08 JAR](../research/applets/v3/artifacts/original/mig33-3.08.jar) |115314 bytes|`2b416199dca5f6593eefe8fa1c9b84ec4be6cdc36f68bcf17e1a94193bfd2934`|
| [4.62 JAR](../research/applets/v4/artifacts/original/mig33-4.62.jar) |190378 bytes|`dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`|

V3 has 35 classes and 3 PNGs. V4 has 26 heavily merged classes, 2 root PNGs, a 148-field theme with 9 embedded PNGs, and 419 English language values. Class count is not a measure of feature count.

## Reproduce and inspect

All scripts run locally. Original legacy network endpoints are never used by the capture/fixture tests; the capture harness shadows MIDP Connector with a deny-all implementation. The original JARs are archival inputs, not applications to launch directly against old servers.

```sh
# Rebuild byte/resource/draw inventories from the preserved inputs.
python3 tools/applets/scripts/inventory.py
python3 tools/applets/scripts/inventory.py v4
python3 tools/applets/scripts/extract_v4_theme.py
python3 tools/applets/scripts/build_v4_endpoints.py

# Render original bytecode through the offline capture harnesses.
sh tools/applets/v3/capture_screens.sh
sh tools/applets/v4/capture_screens.sh

# Check original serialization fixtures and package integrity.
python3 tools/applets/v3/protocol-codec.py --self-test
python3 tools/applets/v4/protocol-codec.py --self-test
python3 tools/applets/scripts/validate_reference.py
```

Capture scripts support `JAVA_BIN`/`JAVAC_BIN` overrides. The analysis used the installed Homebrew JDK and MicroEmulator 2.0.4. Tool binaries, versions, sources and hashes are recorded in [tools/manifest.json](../tools/applets/bin/manifest.json). For original-source decompilation steps and the exceptional-flow caveat, see [methodology](../research/applets/shared/methodology.md).

The [validation report](../research/applets/validation.json) records original binary/resource integrity, class coverage, screenshot hashes and PNG checks, theme extraction, tool hashes, JSON parsing and local documentation/gallery links. Protocol codecs separately verify 7 v3 and 9 v4 original-class serialization fixtures.

This directory was empty and was not a Git repository at the start. No commit was created.
