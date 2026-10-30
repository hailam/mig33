# Original v4 protocol catalog source audit

Audit date: 2026-10-30. Target: preserved mig33 Java ME 4.62 build .000.

The protocol catalogs are largely faithful to the original client within their documented scopes. They are compact research references, not complete server schemas. The audit found a stale description, several details worth clarifying, and extraction boundaries that must be represented before code generation. The earlier description of “conflicting historical field interpretations” was too broad: the original wire format has raw fields, and several accessor views can coexist without an encoding contradiction.

## Authority and method

The unchanged publisher JAR has SHA-256:

```text
dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c
```

All 26 classes were disassembled directly with `javap -c -p -s`. These flags reproduce the original source-line references; an initial run without `-s` shifted lines because it omitted descriptors. All 128 endpoint references match when the correct flags are used. Method descriptors and bytecode offsets are also recorded so that findings do not depend only on text line numbers.

Original bytecode is the authority. The audit did not execute an analysis-only JAR, start a MIDlet, connect to historical services, or infer historical server behavior from the Go reconstruction. Offline probes execute original field/packet classes and selected request builders under OpenJDK 21 with the pinned MicroEmulator classes available. Builder probes bypass constructors and capture the private queue to avoid worker, transport, and settings side effects. They do not simulate a complete running handset.

The checks combine independent inventories, method-scoped control-flow analysis, receiver provenance, hand-reviewed semantic annotations with mechanically checked anchors, and executable original-class probes. Automated anchor checks confirm the cited instructions; they are not a general proof of every semantic statement.

## Coverage and retained evidence

| Area | Coverage and result | Detailed evidence |
|---|---|---|
| Source identity | Original JAR verified; all 26 classes disassembled | [Reproduction record](data/protocol-audit-reproduction.json) |
| Outgoing catalog | All 56 rows reviewed; 152 field-constructor references covering 141 distinct instruction sites | [Outgoing audit](data/protocol-audit-outgoing.json) |
| Packet construction inventory | All 67 recorded sites match opcode, class, method, and line | [Outgoing audit](data/protocol-audit-outgoing.json) |
| Generic request helpers | All 25 observed call sites mapped to 14 opcodes; none outside the outgoing catalog | [Outgoing audit](data/protocol-audit-outgoing.json) |
| Actual request builders | 62 offline cases pass across 49 operations; seven operations reviewed statically | [Outgoing audit](data/protocol-audit-outgoing.json) |
| Incoming catalog | All 55 rows match original dispatch entries, normal CFG reachability, constant-tag reads, and direct field sets | [Incoming audit](data/protocol-audit-incoming.json) |
| Existing incoming extractor | Unchanged script reproduces the catalog byte for byte | [Incoming audit](data/protocol-audit-incoming.json) |
| Endpoint inventory | All 62 literals independently found in original class constant pools; all 128 executable references match | [Transport audit](data/protocol-audit-transports.json) |
| Route construction | All 28 route-note entries across 58 call sites, including 17 placeholder templates across 41 sites; four dynamic URL categories reviewed | [Transport audit](data/protocol-audit-transports.json) |
| Original serializer fixtures | All nine reproduce byte for byte; research and Go fixture copies remain identical | [Reproduction record](data/protocol-audit-reproduction.json) |
| Primitive and frame behavior | 33 bounded probes execute original classes and pass their recorded checks | [Transport audit](data/protocol-audit-transports.json) |
| Version-comparison metadata | Existing v3/v4 catalogs reproduce the recorded comparison, including 13 changed field sets | [Reproduction record](data/protocol-audit-reproduction.json) |

The constructor-site count includes synthetic responses, local jobs, and RMS objects; it is not a count of network operations. Likewise, the nine serializer fixtures exercise the original serializer with synthetic input, while the 62 builder probes exercise actual original request-building methods. They provide different evidence.

Static-only outgoing operations are `10, 200, 202, 419, 500, 922, 926`. Their audit entries include exact methods, field writes, values, and conditions. They were not invoked merely to inflate runtime coverage, because some involve login, settings, or broader application state.

## Corrections and refinements

### Opcode 913 response description

The JSON description incorrectly said request mode 1 could open animated/photo content. The request fields were correct: tag 1 is the token plus a literal trailing space, and tag 2 is an integer action mode. The response handler reads that mode from the pending request. For mode 1, response 914 type 2 triggers 500 ms vibration and type 4 plays WAV content. The Markdown protocol description was already accurate; the JSON now agrees.

Evidence: `bc.a:([Lay;)V` offsets 5819–5867, `aj.d:(I)V` offsets 0–10, `aj.a:([BLjava/lang/String;)V` offsets 17–33, and `bd`'s audio constructor/player path. The [outgoing audit](data/protocol-audit-outgoing.json) retains the pre-audit finding and corrected status.

### Opcode 419 timestamp fallback

The “saved timestamp” summary omitted a real branch: when `aj.W` is null, the client uses the current time. Otherwise it parses the saved value. Both the JSON and Markdown summaries now include this fallback. Evidence: `bc.a:([Lay;)V` offsets 14206–14235 and field construction at 14338.

### Multiple accessor views are not conflicting wire types

Received fields have no type discriminator. The original decoder constructs raw-byte fields; numeric getters read a prefix. The probe value `00010002` is simultaneously byte 0, short 1, int 65538, and a text string containing four control characters.

For response 203, tags 31/32 really are consumed as integer dimensions and later as a short/text pair. Their combined application meaning is not fully established, but the reads are reproducible and can coexist on the wire. The protocol description now states this precisely.

Other mixed getter observations have simpler explanations: 203 tags 33/35/37 use a byte getter as a presence check before an integer read; 421/423 tag 3 uses a text getter as a presence check before a long read. A generator must distinguish these from multiple semantic value uses. A null check alone also does not establish server-side optionality.

Typed in-memory fields and received raw fields differ: an integer constructed in memory can reject a short/text accessor, while the same bytes after serialization and decoding support those views. Tests must exercise the received representation when making claims about incoming packets.

### String-array decoding is stranger than the previous prose suggested

The previous warning emphasized empty segments. Original-class execution confirms loss of ordinary nonempty values too:

| Raw text bytes | Original decoded array |
|---|---|
| `a` | `[a]` |
| `a\0b` | `[a]` |
| `a\0bc` | `[a, bc]` |
| `ab\0cd\0ef` | `[ab, cd]` |
| `alice@example.test\0bob` | `[alice@example.test]` |

The final example is the payload of the existing string-array fixture. That fixture correctly verifies serialization; it never established that the original decoder round-trips every element. The updated prose uses “can lose” deliberately: not every final element is dropped. Exact cases and source anchors are in the [transport audit](data/protocol-audit-transports.json).

This is a client decoder observation. It does not justify changing the recorded wire bytes or assuming that a historical server used the same decoder.

### Empty fields and malformed frames

An absent field and a present empty field differ. For present empty raw data, the original byte getter throws `ArrayIndexOutOfBoundsException`; short/int/long getters return null for insufficient width. Original frame probes also confirm acceptance of a negative body length as empty and a field extending beyond the declared body length, while a negative field length throws.

These behaviors are documented as original implementation quirks. The Go codec's safer length checks and non-panicking accessor results remain unchanged. Compatibility evidence for ordinary packets should not be confused with a requirement to reproduce malformed-input defects.

### Endpoint scope wording

The route notes called all 17 `%s` templates relative `/sites/` templates. The actual set contains 16 relative templates and one absolute registration URL. The count wording is corrected; route literals are unchanged. All 28 route-note entries and their 58 call sites were reviewed, including all 41 placeholder-template sites and the shared URL/form helpers. The [transport audit](data/protocol-audit-transports.json) records the substitutions, source anchors, and limits of the downstream UI review.

## What the incoming catalog does and does not contain

The existing extractor accurately implements its declared scope. Its `direct_fields` recognizes an immediate response-array load followed by a constant tag and typed getter. `all_getter_reads` is broader, but still requires a constant tag and includes pending-request/settings reads. Neither array is a complete response schema.

| Operation | Additional source evidence | Catalog implication |
|---|---|---|
| 906 | Three tag 30 byte reads through the response stored in `aj.aD` | Already in `all_getter_reads`; excluded from `direct_fields` by the immediate-load rule |
| 1002 | Three text getters use computed tags `(short)(i+1)`, `(short)(i+2)`, `(short)i` | Missing even from `all_getter_reads` because tags are not immediate constants |
| 502 | Whole packet queued; later UI reads tags 4/5 as text | Requires a deferred-consumer annotation |
| 927 | Current response passed to the image helper, reading tag 1 and tags 2–6 | Requires delegated field evidence |
| 208 | Calls that same image helper on a cached packet | Do not merge the helper's fields into response 208 |
| 1000 | Whole resource persisted/installed; generic theme accessors read it later | Empty dispatcher fields do not imply an empty resource schema |
| 1004 | Language packet persisted/installed; later dynamic language lookups | Dispatcher fields do not enumerate the full language resource |

The independent analysis counts 408 constant-tag getter observations already cataloged, 411 total typed-getter observations, and 341 current-response observations. These are sums over opcode paths: a shared handler counts once for each opcode that reaches it. Unique instruction counts are 392, 395, and 325 respectively. The three extra instruction sites are the computed tags in 1002; 906 is an alias-classification issue, not three missing `all_getter_reads` entries.

Conservative exception-edge traversal adds no typed getter sites in the dispatcher. This does not establish complete exception semantics. The delegated/deferred annex is a reviewed set of concrete paths, not a universal interprocedural analysis or a complete inventory of every theme/language key.

## Confirmed quirks retained without correction

Actual original builder probes confirm duplicate tag 1 writes in 101 and 602, ignored boolean/integer arguments in several helpers, conditional contact fields, 907's preservation of cached fields, the literal trailing space in 913, and duplicate client strings in 103. These are evidence-backed behaviors, not extraction mistakes to normalize.

The original HTTP and TCP transports remain distinct views of the same packet protocol. HTTP polling can carry multiple packets; an HTTP success can enclose an application error. Local jobs 1001/1003/1005 remain local fetch jobs rather than invented gateway operations. This audit makes no API or transport change.

## Consequences for AsyncAPI and code generation

The revised conclusion is: use the catalogs with their evidence and explicit scope. Compact prose is not a correctness defect. Before generation, represent operation direction, field writes versus reads, conditions, request variants, aliases/delegation, correlation, and raw-byte views explicitly.

The per-operation audit records provide that preparation for the reviewed slices. They are not automatically a new authoritative IDL. Keep original evidence separate from normalized generation metadata, and retain unknown/raw fields. Do not infer server requiredness, allowed opcodes, or guaranteed replies solely from a client's builders and dispatch cases.

## Reproduction

Prerequisites are the verified original v4 JAR, pinned MicroEmulator JAR, an installed JDK, and Python. The [setup guide](../../../../tools/applets/SETUP.md) supplies download locations and checksums. No decompiler or transformed JAR is needed for this audit.

From the repository root:

```sh
python3 tools/applets/v4/verify-protocol-evidence.py --output research/applets/v4/docs/data/protocol-audit-reproduction.json
python3 tools/applets/v4/audit-outgoing.py
python3 tools/applets/v4/audit-incoming.py
python3 tools/applets/v4/audit-transports.py --write-catalog
go test ./internal/protocol
```

The verification tools accept `--java-home` where Java execution is needed. They check original inputs before executing probes. Raw disassembly, compiled probes, and execution output remain ignored under `artifacts/bytecode` and `data/protocol-audit`; the small source tools, results, and this report are tracked material.

The incoming catalog and its original extractor remain unchanged. The source audit additionally verifies deterministic incoming output, constructor/endpoint inventories, Markdown/JSON summary consistency, fixture-copy equality, and version-comparison consistency.

## Remaining limits

This is an audit of this exact client build and the named catalogs. It does not recover the original server implementation, all server-driven form submissions, every dynamic resource key, or every handset runtime behavior. Route construction checks do not cover every preceding username-editor normalization step or complete subsequent photo/UI flow. The original JAD is absent locally, so its distribution-URL claim is explicitly not revalidated; the JAR manifest's installer notification is verified independently. The v3 comparison check validates existing metadata, not a fresh v3 source audit. Desktop-JVM probes should not be extended into claims about all CLDC Unicode, locale, or charset behavior.

No Expo code, AsyncAPI implementation, Go behavior, or original transport was changed. The changes are catalog/documentation corrections and reproducible audit tooling.
