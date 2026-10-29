# mig33 v4 artifact and release provenance

The latest verified **mig33 Java ME v4** build recovered in this research is **4.62, build `.000`, keypad variant with 24×24 application icon**. Its original publisher JAR and an independent mirror are byte-identical. This is a separate artifact from the documented v3.08 client.

For current download and setup instructions, see [local applet setup](../../../../tools/applets/SETUP.md). The publisher-archive JAR still matched the hash below when checked on 2026-10-29; the bare mirror URL returned an HTML download page on that check. The earlier mirror comparison below remains the record of the original retrieval.

## Verified payload

| Property | Value |
| --- | --- |
| Recovered official JAR | [`research/v4/official-keypad.jar`](../discovery/official-keypad.jar) |
| Official JAD, 2011 | [`research/v4/official-keypad-2011.jad`](../discovery/official-keypad-2011.jad) |
| Official JAD, 2013 | [`research/v4/official-keypad-2013.jad`](../discovery/official-keypad-2013.jad) |
| Original publisher URL | `http://www.mig33.com:80/wap2/v4_60/24x24/mig33v46.jar` |
| First recovered capture | `2011-04-27 07:03:16 UTC` |
| Retrieval | [Archived publisher JAR](https://web.archive.org/web/20110427070316id_/http://www.mig33.com:80/wap2/v4_60/24x24/mig33v46.jar) |
| Size | `190378` bytes, matching both official JADs |
| SHA-256 | `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c` |
| Archive CDX digest (SHA-1, base32) | `6H7WNCXGJQ5YHBPS23MYJKSNK7MUVMFA` |
| Manifest version/build | `MIDlet-Version: 4.62`, `BUILD: .000` |
| Entry point | `main.Main` |
| Vendor | `Project Goth, Inc.` |
| Runtime | MIDP 2.0, CLDC 1.0 |

The mirror `https://kemongers.jw.lt/hp/mig33v46.jar`, linked from the [Kemongers listing](https://kemongers.jw.lt/migori), has the same SHA-256. The filename and marketing line say `4.6`; the actual manifest and official descriptors say `4.62`. Read the manifest instead of inferring the release from a filename.

[`research/v4/provenance.json`](../discovery/provenance.json) records retrieval metadata. CDX evidence is preserved in `research/applets/v4/discovery/v460-keypad-cdx.json`, `v460-keypad-jad-cdx.json`, and `java-jars-cdx.json`.

## Was v4 the latest Java ME line?

**Yes, in the recovered first-party mig33 release evidence.** The publisher's [15 February 2014 Java download listing](https://web.archive.org/web/20140215010316id_/http://www.mig33.com/downloads/j2me/all) labels the v4.6 line “Latest version with full mobile feature set” and “NEWEST”, linking the same v4.62 artifact path. It also offers v4.6 Touch, v4.2, and v3.02 lite as alternatives. The page is preserved at [`official-java-all-2014.html`](../discovery/official-java-all-2014.html).

The [24 December 2013 direct Java download page](https://web.archive.org/web/20131224173731id_/http://www.mig33.com:80/downloads/j2me) independently identifies v4.6 as newest. Its stated 750 KB size conflicts with the verified binary and descriptor; use **190378 bytes**, not that marketing-page size.

The [July 2014 migme download page](https://web.archive.org/web/20140705120430id_/http://wap.mig33.com:80/downloads/) describes a forthcoming migme J2ME replacement and continued access using the mig33 app through the end of 2014. This prevents a stronger claim that no subsequent migme client could ever have existed. The accurate conclusion is **latest verified mig33 Java ME release: 4.62**, not proof that every later rebrand, regional build or device variant has been exhausted.

An archive-domain search for successful Java-archive responses containing v4/v5/v6 paths found v4 releases through the v4.60 distribution directories and no v5/v6 Java ME JAR in those indexed results. Absence from an archive is supporting search coverage, not proof of nonexistence. Android and BlackBerry version numbers belong to separate clients.

## Touch and icon variants

The recovered official touch JAR is **4.60 Touch Beta, build `.005`**, not 4.62:

| Property | Touch payload |
| --- | --- |
| Local file | [`official-touch.jar`](../discovery/official-touch.jar) |
| Official descriptor | [`official-touch.jad`](../discovery/official-touch.jad) |
| Archive retrieval | [2011-06-22 official touch JAR](https://web.archive.org/web/20110622082033id_/http://www.mig33.com:80/wap2/v4_60/touch/mig33v46_touch.jar) |
| Size | `191450` bytes |
| SHA-256 | `9e37acdccf8920ada3854dd626a4d38f3d487186b7b4a375c4e00d0ce1c1821b` |

The first-party archive also indexes a `48x48/mig33v46.jar` icon variant. The main v4 specification targets the verified 24×24 keypad payload. Touch and alternate icon builds must not be silently treated as pixel-identical or functionally identical to it.

## Popularity claim

The recovered publisher download pages establish availability and newest-version positioning. They do **not** provide version-level active-user counts, installation counts, retention data or market share. They cannot establish that v3 was the most popular mig33 client. Community download counters are site-specific and mix original and modified builds; they are not representative usage telemetry. The claim “v3 was most popular” remains **unverified** unless stronger historical metrics are recovered.

## Recheck

```sh
shasum -a 256 research/applets/v4/discovery/official-keypad.jar
cmp research/applets/v4/discovery/official-keypad.jar research/applets/v4/discovery/mirror-mig33v46.jar
unzip -p research/applets/v4/discovery/official-keypad.jar META-INF/MANIFEST.MF
```
