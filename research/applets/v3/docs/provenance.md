# Artifact provenance

The documented application is **mig33 v3.08 for Java ME**, a MIDP 2.0 / CLDC 1.0 MIDlet. Its entry point is `com.projectgoth.Roshan.Main`. It is a mobile Java application, not a browser `java.applet.Applet`.

For current download and setup instructions, see [local applet setup](../../../../tools/applets/SETUP.md). The publisher-archive JAR still matched the hash below when checked on 2026-10-29; the bare mirror URL returned an HTML download page on that check. The earlier mirror comparison below remains the record of the original retrieval.

## Verified original

| Property | Value |
| --- | --- |
| Local JAR | [`artifacts/original/mig33-3.08.jar`](../artifacts/original/mig33-3.08.jar) |
| Local official JAD | [`artifacts/original/mig33-3.08.jad`](../artifacts/original/mig33-3.08.jad) |
| Original publisher URL | `http://wap.mig33.com:80/v3_08/mig33v3.jar` |
| Wayback capture | `2009-04-17 05:22:59 UTC` |
| Downloaded payload | [Archived official JAR](https://web.archive.org/web/20090417052259id_/http://wap.mig33.com:80/v3_08/mig33v3.jar) |
| Official descriptor | [Archived official JAD](https://web.archive.org/web/20090417052254id_/http://wap.mig33.com:80/v3_08/mig33v3.jad) |
| File size | `115314` bytes; agrees with official JAD |
| SHA-256 | `2b416199dca5f6593eefe8fa1c9b84ec4be6cdc36f68bcf17e1a94193bfd2934` |
| Archive CDX digest (SHA-1, base32) | `NWOF7EBZ4HUHASIYU4CO7YB6EEVEWCDS` |
| ZIP entries | 39 entries, including 35 `.class` files, three PNG images and the manifest |
| Class ZIP timestamps | `2009-04-10 10:45`; ZIP timestamps have no timezone and do not prove a release date |

The first recovered file came from the [Kemongers original-version listing](https://kemongers.jw.lt/migori), at `https://kemongers.jw.lt/hp/mig33v3_08.jar`. The downloaded mirror file and the archived first-party JAR have **identical SHA-256 hashes**, not merely matching manifests. This establishes that the analyzed mirror payload matches the preserved publisher payload. It is not a cryptographic publisher-signature verification: the JAR contains no signature files.

The Archive CDX index also records the same payload digest at `http://www.mig33.com:80/wap2/v3_08/mig33v3.jar` on `2010-06-23 00:00:21 UTC`. A later HTML response at that URL is not an application binary. This distinction matters when retrieving old download addresses.

The [mig33world download listing](https://mig33world.weebly.com/download.html) corroborates the version and links to that publisher path. These community pages helped locate the artifact; the application's manifest, official descriptor and archived publisher payload establish its identity.

## Exact metadata

```text
MIDlet-1: mig33v3,MiG33.png,com.projectgoth.Roshan.Main
MIDlet-Jar-Size: 115314
MIDlet-Jar-URL: mig33v3.jar
MIDlet-Name: mig33v3
MIDlet-Vendor: Project Goth, inc.
MIDlet-Version: 3.08
MicroEdition-Configuration: CLDC-1.0
MicroEdition-Profile: MIDP-2.0
```

The JAD is preserved byte-for-byte and consequently refers to its historical filename `mig33v3.jar`; the local JAR has a descriptive filename. Loading the JAR directly avoids changing the official descriptor.

## Evidence retained

- [`research/candidates/provenance.json`](../discovery/provenance.json): retrieval timestamp, artifact hash, manifest and source URLs.
- [`research/candidates/official-jar-cdx.json`](../discovery/official-jar-cdx.json): original archive-index response for the JAR.
- [`research/candidates/official-jad-cdx.json`](../discovery/official-jad-cdx.json): archive-index response for the descriptor.
- [`research/candidates/mig33v3_08-archive-original.jar`](../discovery/mig33v3_08-archive-original.jar): retrieved official payload.
- [`research/candidates/mig33v3_08.jar`](../discovery/mig33v3_08.jar): independently downloaded mirror payload.
- [`research/candidates/kemongers-migori.html`](../discovery/kemongers-migori.html) and [`research/candidates/mig33world-download.html`](../discovery/mig33world-download.html): discovery listings.

## Other recovered version and scope

`research/applets/v3/discovery/mig33v3_02.jar` is a different MIDP 1.0 build whose manifest reports v3.02 and Project Goth. Its SHA-256 is `5b158e0d0ed4ad1db0fbacd672c0e8400f2fe4d75692f1afd3012a9afb52c17d`. It is retained as a discovery candidate and is **not** the source for the v3.08 screen or API specification. No publisher-payload comparison was made for v3.02.

v3.08 is the latest recoverable Java ME v3 version identified in this research. This does not establish that every historical manufacturer-specific or regional build was identical. No successful live connection to the historical mig33 service was needed to recover or statically analyze the binary.

## Recheck

```sh
shasum -a 256 research/applets/v3/artifacts/original/mig33-3.08.jar
unzip -p research/applets/v3/artifacts/original/mig33-3.08.jar META-INF/MANIFEST.MF
cmp research/applets/v3/artifacts/original/mig33-3.08.jar research/applets/v3/discovery/mig33v3_08-archive-original.jar
cmp research/applets/v3/discovery/mig33v3_08.jar research/applets/v3/discovery/mig33v3_08-archive-original.jar
```
