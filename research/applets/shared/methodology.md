# Recovery and validation methodology

## Acquisition

Downloaded candidate JARs from historical mirrors, read ZIP manifests, then compared SHA-256 hashes against original publisher URLs preserved by the Internet Archive. Version identity comes from the JAR and official JAD, not the mirror filename. Original bytes were not modified. Primary source URLs, archive timestamps, CDX data and local copies are recorded in the two provenance documents.

## Static recovery

CFR 0.152 and Vineflower 1.12.0 analyzed both JARs without executing them. `javap -p -c -s` disassembled every original class. Procyon 0.6.0 supplied a third reading of v4. Decompiled symbols retain obfuscation and may collide; casts, field widths, calls and branches were checked against bytecode for protocol-sensitive details.

```sh
JAVA_BIN=/opt/homebrew/opt/openjdk@21/bin/java
"$JAVA_BIN" -jar tools/applets/bin/cfr-0.152.jar research/applets/v3/artifacts/original/mig33-3.08.jar --outputdir research/applets/v3/artifacts/decompiled --silent true
"$JAVA_BIN" -jar tools/applets/bin/vineflower-1.12.0.jar research/applets/v3/artifacts/original/mig33-3.08.jar research/applets/v3/artifacts/vineflower
"$JAVA_BIN" -jar tools/applets/bin/cfr-0.152.jar research/applets/v4/artifacts/original/mig33-4.62.jar --outputdir research/applets/v4/artifacts/decompiled --silent true
"$JAVA_BIN" -jar tools/applets/bin/vineflower-1.12.0.jar research/applets/v4/artifacts/original/mig33-4.62.jar research/applets/v4/artifacts/vineflower
"$JAVA_BIN" -jar tools/applets/bin/procyon-decompiler-0.6.0.jar -jar research/applets/v4/artifacts/original/mig33-4.62.jar -o research/applets/v4/artifacts/procyon
```

Javap is invoked per original class with `-classpath <original.jar> -p -c -s <fully.qualified.Class>`. Decompiler logs and summaries remain with each output. A source file's existence does not mean every method successfully decompiled.

## V4 normal-flow recovery

All three ordinary decompilers failed the large UI command handler and incoming protocol dispatcher. An alternate Vineflower run with finally processing disabled also failed. The source of difficulty was obfuscated exception/control-flow structure.

[`decompile_normal_flow.py`](../../../tools/applets/scripts/decompile_normal_flow.py) creates a separate explicitly named `DO-NOT-RUN-no-exceptions.jar` with **Code exception-handler tables omitted**. It asserts that every method's bytecode instruction sequence remains identical and retains original constant pools and signatures. This allows Vineflower to present the normal branch structure of both handlers. The [transformation list](../v4/docs/data/normal-flow-transformations.json) records 284 affected methods.

```sh
python3 tools/applets/scripts/decompile_normal_flow.py
"$JAVA_BIN" -jar tools/applets/bin/vineflower-1.12.0.jar research/applets/v4/artifacts/analysis-only/DO-NOT-RUN-no-exceptions.jar research/applets/v4/artifacts/normal-flow
```

This output is solely a reading aid. Omitting handlers changes exceptions, synchronization cleanup and finally semantics. No screenshot, serializer fixture or behavioral execution uses this derivative. Original disassembly and original handlers remain authoritative. Analysis notes distinguish successful-path evidence from unresolved exceptional behavior.

## Resource inventories

The stdlib-only inventory script parses original class constant pools, preserves all string constants/referenced class names, hashes every ZIP entry, reads PNG dimensions and indexes source draw/literal locations. The v4 theme decoder validates its packet framing, extracts all 148 fields and 9 embedded PNGs, and preserves hashes/offsets. The screen catalog separately decodes 419 language values.

Regex/source inventories are audit aids, not semantic parsers. A method with a variable field ID or a failed decompile can require bytecode inspection. Original-byte and normal-flow references complement these catalogs.

## Offline execution

MicroEmulator 2.0.4 supplies MIDP rendering and in-memory RMS. A Connector replacement rejects all network and file protocols; harnesses verify its resolved location and refusal before running application rendering. Original application class files are used unmodified. V4 uses a resource-only bridge and an upstream null-command compatibility fix for the emulator; details and source attribution are in its capture notes.

Screenshots force known screen states through reflection. Server-dependent data is synthetic and explicitly marked. This verifies the original layout at the captured emulator profile, not live server integration or historical phone-font identity.

Protocol fixture harnesses instantiate original packet/field classes to obtain real serialized bytes. Reference codecs compare valid encodings, primitives, UTF-8, field replacement, concatenation and authentication vectors. They deliberately reject malformed lengths more strictly than the old clients. No login, payment, messaging or account action was sent to legacy services.

## Remaining evidence boundaries

Static reverse engineering reveals what these clients send, consume and draw; it cannot prove the entire backend's acceptance rules or recover absent historical payloads. Screens/pages that depend on server flags, themes, language updates, account state, remote markup, photos or emoticons require fixtures. A complete live-system replica additionally requires a reconstructed backend and a chosen handset profile. These are documented external inputs, not silently fabricated details.
