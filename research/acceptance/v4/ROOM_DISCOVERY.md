# Original-client room discovery and favorites

The recorded [2026-10-27 acceptance run](room-discovery-result.md) passed all three phases and preserved 40 screenshots plus 127 binary frames.

Start the current local backend with a dedicated test database, then run:

```sh
sh tools/applets/v4/integration/run-room-discovery.sh
```

The runner creates one public room with a unique name beginning `Discovery `, creates and removes Hailam's favorite, and leaves the room definition in the test database. It neither starts nor stops the backend. Avoid using the ordinary desktop database for repeatable acceptance captures; the local per-owner room quota also applies to this runner.

Three sequential independent JVMs load the unchanged v4.62 JAR. Each has an isolated temporary home and in-memory RMS. Hailam's creation phase uses the original TCP transport. Bob's observer phase and Hailam's fresh-login phase use the original HTTP tunnel. The loopback-only adapter rejects external destinations. All phases finish original opcode 300 logout before the next client starts.

## Assertions

1. **Create Room:** Hailam opens the actual Chat Rooms menu action. The original browser parses the name form; the harness sets its native input model and activates the original submit control with FIRE. The first POST returns the description form. Its native full-width textarea receives `Local friends & chat + 50% fun = yes`, then a second original form POST returns **Room Created**.
2. **Join and Room Info:** the original browser activates the exact `mig33:joinChatroom(Discovery …)` link. Original opcode 703 carries the literal room name with its space preserved, and 701 plus ACK creates an active room tab. The actual room menu opens **Room Info**, whose response contains the persisted owner, description and capacity.
3. **Save to Favorites:** the room menu emits 711. Its correlated 701 places the room in category 2. The native category carries the favorite flag, so its room menu offers **Remove from Favorites**. Repeating Save from the room tab receives ACK 1 and does not insert another native row.
4. **Account isolation and Search Rooms:** Bob's category 2 does not contain Hailam's favorite. Bob opens the original Search Rooms menu and submits an empty query, which the native serializer omits; available rooms are returned. He then fills the parsed query control, submits via FIRE, joins through the returned native command link and opens Room Info. His participant popup confirms his membership.
5. **Saved favorites and removal:** a new Hailam JVM/session fetches category 2 and finds the saved room despite empty local RMS. Its actual Remove from Favorites action emits 712 with category 2 and the canonical room name. The row disappears, remains absent after another category fetch, and the public room still appears in the global catalog.
6. **Catalog refresh:** the original category footer is activated through FIRE, once for Favorites and twice for the global catalog. Each original 716 request uses flag 0, and the resulting native catalog has exactly the same unique room rows without duplication.

The runner saves per-phase runtime logs, native model summaries, HTML responses, original Canvas screenshots, binary frames and result files under `room-discovery-out/`. A PASS file is written only after that phase's assertions and correlated logout acknowledgment succeed.

## Original contracts and scope

Recovered opcode 711 uses text tag 1 for the room; 701 returns category tag 6, capacity 3 and occupancy 4 when the requesting session needs the row. Its receive branch completes pending 711 directly. The native 712 builder uses short tag 1 for category and text tag 2 for room; the native UI removes the row optimistically before its ACK arrives. A repeated add already known to the requesting session uses the original generic ACK completion path to avoid the native 701 handler's unconditional row insertion. Another session can still receive 701 for a favorite it has not loaded.

Category opcode 714 tag 5 identifies a favorite category. Tag 4 is its initial collapsed state, subject to stored client preferences. Tag 3 controls clearing the previous category rows before a next-page request. Both local categories set tag 3 to 1 because they return full snapshots; category 2 additionally sets the favorite flag. Opcode 717 supplies the footer label **Refresh**, which the original client renders and activates with a 716 flag-0 request. The native general Refresh action also clears rows before issuing flag-1 requests.

The browser's command link parser passes the `joinChatroom` argument directly to the original join builder. It does not decode percent escapes in that argument; literal spaces must remain literal. Page HTTP URLs use URL escaping separately. The two creation forms keep only one free-text value in each native POST because the original serializer escapes spaces but does not generally escape ampersands, plus signs, percent signs or equals signs. Its ambiguity for typed literal `%20` remains documented in the backend feature contract.

The harness selects native menus and executes original command handlers, browser parsing, form serialization, link activation, transport and receive dispatch. It sets text values and focus through parsed models instead of simulating handset text entry. Captures use the original 240×320 Canvas with desktop MicroEmulator fonts. The pages, ownership rules and room limits are reconstructed local-server behavior, not recovered historical HTML or server policy. Fresh-JVM favorite retrieval proves persistence beyond client memory; backend Go tests separately cover server/store restart persistence.

The [shared runtime boundary](README.md#runtime-boundary) applies. The public-room and moderation harnesses remain separate and their captured evidence is preserved.
