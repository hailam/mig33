# mig33 4.62 default theme fields

Every serialized field in `default_v2`, ordered by numeric tag. Integer interpretations are signed big-endian values; their roles follow the consuming code. Missing fields are absent, not assumed zero in this catalog. `y.d(int)` returns zero for a missing integer field; the overloaded image getter returns null or a 1×1 placeholder for specific tags.

[Extracted images](assets.html) · [all usage locations](data/theme-usage.json) · [raw bytes and hashes](data/theme.json)

Verification on 2026-10-31 matched all 148 catalogued payload lengths and SHA-256 hashes, plus every stored int32 value, against `default_v2` inside the unchanged v4.62 JAR. The 202 source references below formerly had an incorrect `v3` directory prefix; each class/line suffix was checked against the same-tag entry in the v4 usage inventory before correcting the prefix. Decompiled source line numbers remain reading aids; original bytecode is authoritative.

| Tag | Value / asset | Source references |
| --- | --- | --- |
| 0 | default_v2 | No direct literal getter found; may be metadata or dynamically indexed. |
| 1 | 21 / `#000015` | No direct literal getter found; may be metadata or dynamically indexed. |
| 2 | Default V2 | No direct literal getter found; may be metadata or dynamically indexed. |
| 3 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/k.java:872` |
| 4 | 14013909 / `#d5d5d5` | `research/applets/v4/artifacts/vineflower/k.java:872` |
| 5 | 626614 / `#098fb6` | `research/applets/v4/artifacts/vineflower/k.java:869` |
| 6 | 805755 / `#0c4b7b` | `research/applets/v4/artifacts/vineflower/k.java:869` |
| 7 | 3256808 / `#31b1e8` | `research/applets/v4/artifacts/vineflower/k.java:899` |
| 8 | 1021395 / `#0f95d3` | `research/applets/v4/artifacts/vineflower/k.java:870` |
| 9 | 375507 / `#05bad3` | `research/applets/v4/artifacts/vineflower/k.java:870` |
| 10 | 310996 / `#04bed4` | `research/applets/v4/artifacts/vineflower/k.java:901` |
| 11 | 15716102 / `#efcf06` | `research/applets/v4/artifacts/vineflower/k.java:871` |
| 12 | 16679680 / `#fe8300` | `research/applets/v4/artifacts/vineflower/k.java:871` |
| 13 | 16383923 / `#f9ffb3` | `research/applets/v4/artifacts/vineflower/k.java:907` |
| 14 | 149597 / `#02485d` | `research/applets/v4/artifacts/vineflower/k.java:892`; `research/applets/v4/artifacts/vineflower/k.java:928` |
| 15 | 820153 / `#0c83b9` | `research/applets/v4/artifacts/vineflower/k.java:890` |
| 16 | 3256808 / `#31b1e8` | No direct literal getter found; may be metadata or dynamically indexed. |
| 17 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/k.java:986` |
| 18 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/bj.java:1398`; `research/applets/v4/artifacts/vineflower/y.java:684` |
| 19 | 13355979 / `#cbcbcb` | `research/applets/v4/artifacts/vineflower/bj.java:1398`; `research/applets/v4/artifacts/vineflower/y.java:684` |
| 20 | 2236962 / `#222222` | `research/applets/v4/artifacts/vineflower/bj.java:1411`; `research/applets/v4/artifacts/vineflower/y.java:695` |
| 21 | 808839 / `#0c5787` | `research/applets/v4/artifacts/vineflower/y.java:429` |
| 22 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:432` |
| 23 | 748952 / `#0b6d98` | `research/applets/v4/artifacts/vineflower/y.java:447` |
| 24 | 15100418 / `#e66a02` | `research/applets/v4/artifacts/vineflower/y.java:445` |
| 25 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:468`; `research/applets/v4/artifacts/vineflower/y.java:494` |
| 26 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:470`; `research/applets/v4/artifacts/vineflower/y.java:496` |
| 27 | 12237498 / `#bababa` | `research/applets/v4/artifacts/vineflower/au.java:2683` |
| 28 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:483` |
| 29 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:481` |
| 30 | 634067 / `#09acd3` | `research/applets/v4/artifacts/vineflower/y.java:474`; `research/applets/v4/artifacts/vineflower/y.java:499` |
| 31 | 739192 / `#0b4778` | `research/applets/v4/artifacts/vineflower/y.java:424`; `research/applets/v4/artifacts/vineflower/y.java:522` |
| 32 | 891859 / `#0d9bd3` | `research/applets/v4/artifacts/vineflower/y.java:504` |
| 33 | 808839 / `#0c5787` | `research/applets/v4/artifacts/vineflower/y.java:534` |
| 34 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:551` |
| 35 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:564` |
| 36 | 805755 / `#0c4b7b` | No direct literal getter found; may be metadata or dynamically indexed. |
| 37 | 748952 / `#0b6d98` | `research/applets/v4/artifacts/vineflower/y.java:543` |
| 38 | 13240096 / `#ca0720` | `research/applets/v4/artifacts/vineflower/y.java:532` |
| 39 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:549` |
| 40 | 2236962 / `#222222` | `research/applets/v4/artifacts/vineflower/y.java:566` |
| 41 | 13240096 / `#ca0720` | No direct literal getter found; may be metadata or dynamically indexed. |
| 42 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:541` |
| 43 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:1499` |
| 44 | 805755 / `#0c4b7b` | `research/applets/v4/artifacts/vineflower/y.java:628` |
| 45 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:632` |
| 46 | 805755 / `#0c4b7b` | `research/applets/v4/artifacts/vineflower/y.java:605`; `research/applets/v4/artifacts/vineflower/y.java:612`; `research/applets/v4/artifacts/vineflower/y.java:635` |
| 47 | 2236962 / `#222222` | `research/applets/v4/artifacts/vineflower/bj.java:1428`; `research/applets/v4/artifacts/vineflower/ao.java:47`; `research/applets/v4/artifacts/vineflower/y.java:708`; +2 in usage JSON |
| 48 | 2236962 / `#222222` | `research/applets/v4/artifacts/vineflower/d.java:220`; `research/applets/v4/artifacts/vineflower/bj.java:1096`; `research/applets/v4/artifacts/vineflower/ao.java:91`; +4 in usage JSON |
| 49 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:702`; `research/applets/v4/artifacts/vineflower/y.java:718`; `research/applets/v4/artifacts/vineflower/y.java:722`; +1 in usage JSON |
| 50 | 16627835 / `#fdb87b` | `research/applets/v4/artifacts/vineflower/y.java:1601` |
| 51 | 15956229 / `#f37905` | `research/applets/v4/artifacts/vineflower/y.java:1598` |
| 52 | 10198184 / `#9b9ca8` | `research/applets/v4/artifacts/vineflower/bj.java:1397`; `research/applets/v4/artifacts/vineflower/bj.java:1417`; `research/applets/v4/artifacts/vineflower/s.java:2037`; +6 in usage JSON |
| 53 | 6447726 / `#62626e` | `research/applets/v4/artifacts/vineflower/bj.java:1397`; `research/applets/v4/artifacts/vineflower/bj.java:1417`; `research/applets/v4/artifacts/vineflower/s.java:2037`; +6 in usage JSON |
| 54 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/bj.java:1359`; `research/applets/v4/artifacts/vineflower/y.java:832` |
| 55 | 15100418 / `#e66a02` | `research/applets/v4/artifacts/vineflower/bj.java:1361`; `research/applets/v4/artifacts/vineflower/y.java:834`; `research/applets/v4/artifacts/vineflower/y.java:865` |
| 56 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/bj.java:1366`; `research/applets/v4/artifacts/vineflower/y.java:848` |
| 57 | 2236962 / `#222222` | `research/applets/v4/artifacts/vineflower/bj.java:1368`; `research/applets/v4/artifacts/vineflower/y.java:846` |
| 58 | 10198184 / `#9b9ca8` | `research/applets/v4/artifacts/vineflower/bj.java:1389`; `research/applets/v4/artifacts/vineflower/y.java:856` |
| 59 | 15100418 / `#e66a02` | `research/applets/v4/artifacts/vineflower/bj.java:1472`; `research/applets/v4/artifacts/vineflower/y.java:912` |
| 60 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/bj.java:1480`; `research/applets/v4/artifacts/vineflower/y.java:914` |
| 61 | 3560416 / `#3653e0` | `research/applets/v4/artifacts/vineflower/s.java:3107`; `research/applets/v4/artifacts/vineflower/y.java:918` |
| 62 | 627128 / `#0991b8` | `research/applets/v4/artifacts/vineflower/d.java:143`; `research/applets/v4/artifacts/vineflower/ao.java:34`; `research/applets/v4/artifacts/vineflower/y.java:2735` |
| 63 | 805755 / `#0c4b7b` | `research/applets/v4/artifacts/vineflower/d.java:143`; `research/applets/v4/artifacts/vineflower/ao.java:34`; `research/applets/v4/artifacts/vineflower/y.java:2735` |
| 64 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/d.java:217`; `research/applets/v4/artifacts/vineflower/ao.java:40`; `research/applets/v4/artifacts/vineflower/y.java:415` |
| 65 | 16777215 / `#ffffff` | No direct literal getter found; may be metadata or dynamically indexed. |
| 66 | 15100418 / `#e66a02` | `research/applets/v4/artifacts/vineflower/ao.java:50` |
| 67 | 15100418 / `#e66a02` | `research/applets/v4/artifacts/vineflower/ao.java:50` |
| 68 | 15100418 / `#e66a02` | `research/applets/v4/artifacts/vineflower/ao.java:50` |
| 69 | 2236962 / `#222222` | `research/applets/v4/artifacts/vineflower/bj.java:1063` |
| 70 | 16496504 / `#fbb778` | `research/applets/v4/artifacts/vineflower/bj.java:1050`; `research/applets/v4/artifacts/vineflower/s.java:1396`; `research/applets/v4/artifacts/vineflower/s.java:1426` |
| 71 | 14837760 / `#e26800` | `research/applets/v4/artifacts/vineflower/bj.java:1057` |
| 72 | -1 | `research/applets/v4/artifacts/vineflower/au.java:1340` |
| 73 | -1 | `research/applets/v4/artifacts/vineflower/au.java:1340` |
| 74 | [PNG 320×95](../artifacts/theme/field-074-00.png) | `research/applets/v4/artifacts/vineflower/au.java:2466` |
| 75 | 40 / `#000028` | `research/applets/v4/artifacts/vineflower/au.java:2477`; `research/applets/v4/artifacts/vineflower/au.java:2478`; `research/applets/v4/artifacts/vineflower/au.java:2479`; +7 in usage JSON |
| 76 | 15100418 / `#e66a02` | `research/applets/v4/artifacts/vineflower/au.java:2668`; `research/applets/v4/artifacts/vineflower/au.java:2702`; `research/applets/v4/artifacts/vineflower/au.java:2729` |
| 77 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/au.java:2676`; `research/applets/v4/artifacts/vineflower/au.java:2710`; `research/applets/v4/artifacts/vineflower/au.java:2791`; +1 in usage JSON |
| 78 | 2236962 / `#222222` | `research/applets/v4/artifacts/vineflower/au.java:2789` |
| 79 | 677257 / `#0a5589` | `research/applets/v4/artifacts/vineflower/au.java:2798` |
| 80 | 677257 / `#0a5589` | `research/applets/v4/artifacts/vineflower/au.java:2800` |
| 81 | 2236962 / `#222222` | `research/applets/v4/artifacts/vineflower/au.java:2708` |
| 82 | 12040119 / `#b7b7b7` | No direct literal getter found; may be metadata or dynamically indexed. |
| 83 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/bj.java:1213`; `research/applets/v4/artifacts/vineflower/bj.java:1248`; `research/applets/v4/artifacts/vineflower/au.java:2359`; +2 in usage JSON |
| 84 | 2236962 / `#222222` | `research/applets/v4/artifacts/vineflower/bj.java:1211`; `research/applets/v4/artifacts/vineflower/bj.java:1250`; `research/applets/v4/artifacts/vineflower/au.java:2402`; +2 in usage JSON |
| 85 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/au.java:2334`; `research/applets/v4/artifacts/vineflower/au.java:2362`; `research/applets/v4/artifacts/vineflower/au.java:2417` |
| 86 | 677257 / `#0a5589` | `research/applets/v4/artifacts/vineflower/au.java:2415` |
| 87 | 15100418 / `#e66a02` | `research/applets/v4/artifacts/vineflower/bj.java:1184`; `research/applets/v4/artifacts/vineflower/bj.java:1238`; `research/applets/v4/artifacts/vineflower/au.java:2330`; +2 in usage JSON |
| 88 | 7942912 / `#793300` | `research/applets/v4/artifacts/vineflower/bj.java:1186`; `research/applets/v4/artifacts/vineflower/bj.java:1240`; `research/applets/v4/artifacts/vineflower/au.java:2349`; +1 in usage JSON |
| 89 | 5749225 / `#57b9e9` | No direct literal getter found; may be metadata or dynamically indexed. |
| 90 | 0 / `#000000` | No direct literal getter found; may be metadata or dynamically indexed. |
| 91 | 1 / `#000001` | No direct literal getter found; may be metadata or dynamically indexed. |
| 92 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/bj.java:1162`; `research/applets/v4/artifacts/vineflower/bj.java:1307`; `research/applets/v4/artifacts/vineflower/au.java:2635` |
| 93 | 482701 / `#075d8d` | `research/applets/v4/artifacts/vineflower/bj.java:1160`; `research/applets/v4/artifacts/vineflower/bj.java:1309`; `research/applets/v4/artifacts/vineflower/au.java:2633` |
| 94 | 16613393 / `#fd8011` | `research/applets/v4/artifacts/vineflower/bj.java:2683`; `research/applets/v4/artifacts/vineflower/au.java:1341` |
| 95 | 13719552 / `#d15800` | `research/applets/v4/artifacts/vineflower/bj.java:2683`; `research/applets/v4/artifacts/vineflower/au.java:1341` |
| 96 | 7942912 / `#793300` | `research/applets/v4/artifacts/vineflower/bj.java:1138`; `research/applets/v4/artifacts/vineflower/bj.java:1291`; `research/applets/v4/artifacts/vineflower/au.java:2611` |
| 97 | 15924991 / `#f2feff` | `research/applets/v4/artifacts/vineflower/bj.java:2684`; `research/applets/v4/artifacts/vineflower/au.java:1342` |
| 98 | 12120827 / `#b8f2fb` | `research/applets/v4/artifacts/vineflower/bj.java:2684`; `research/applets/v4/artifacts/vineflower/au.java:1342` |
| 99 | 8311776 / `#7ed3e0` | `research/applets/v4/artifacts/vineflower/bj.java:1126`; `research/applets/v4/artifacts/vineflower/bj.java:1279`; `research/applets/v4/artifacts/vineflower/au.java:2599` |
| 100 | 15461355 / `#ebebeb` | `research/applets/v4/artifacts/vineflower/au.java:2665` |
| 101 | 12014336 / `#b75300` | `research/applets/v4/artifacts/vineflower/au.java:2674` |
| 102 | 677257 / `#0a5589` | No direct literal getter found; may be metadata or dynamically indexed. |
| 103 | 2236962 / `#222222` | No direct literal getter found; may be metadata or dynamically indexed. |
| 104 | 16777215 / `#ffffff` | No direct literal getter found; may be metadata or dynamically indexed. |
| 105 | 8947848 / `#888888` | `research/applets/v4/artifacts/vineflower/bj.java:1197`; `research/applets/v4/artifacts/vineflower/bj.java:1202` |
| 106 | 15100418 / `#e66a02` | No direct literal getter found; may be metadata or dynamically indexed. |
| 107 | 8947848 / `#888888` | No direct literal getter found; may be metadata or dynamically indexed. |
| 108 | 2236962 / `#222222` | No direct literal getter found; may be metadata or dynamically indexed. |
| 109 | 2187492 / `#2160e4` | No direct literal getter found; may be metadata or dynamically indexed. |
| 110 | 16777215 / `#ffffff` | No direct literal getter found; may be metadata or dynamically indexed. |
| 111 | 15100418 / `#e66a02` | No direct literal getter found; may be metadata or dynamically indexed. |
| 112 | 2236962 / `#222222` | `research/applets/v4/artifacts/vineflower/y.java:1318`; `research/applets/v4/artifacts/vineflower/y.java:1350` |
| 113 | 16496504 / `#fbb778` | `research/applets/v4/artifacts/vineflower/s.java:1545`; `research/applets/v4/artifacts/vineflower/s.java:1558` |
| 114 | 3498204 / `#3560dc` | No direct literal getter found; may be metadata or dynamically indexed. |
| 115 | 16738816 / `#ff6a00` | No direct literal getter found; may be metadata or dynamically indexed. |
| 116 | 16253181 / `#f800fd` | No direct literal getter found; may be metadata or dynamically indexed. |
| 117 | 10354902 / `#9e00d6` | No direct literal getter found; may be metadata or dynamically indexed. |
| 118 | 16716800 / `#ff1400` | No direct literal getter found; may be metadata or dynamically indexed. |
| 119 | 2260996 / `#228004` | No direct literal getter found; may be metadata or dynamically indexed. |
| 120 | 2236962 / `#222222` | `research/applets/v4/artifacts/vineflower/s.java:2951`; `research/applets/v4/artifacts/vineflower/s.java:3226`; `research/applets/v4/artifacts/vineflower/s.java:3492` |
| 121 | 15100418 / `#e66a02` | `research/applets/v4/artifacts/vineflower/s.java:2116` |
| 122 | [PNG 80×45](../artifacts/theme/field-122-00.png) | `research/applets/v4/artifacts/vineflower/y.java:595`; `research/applets/v4/artifacts/vineflower/y.java:2975` |
| 123 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/y.java:1494` |
| 124 | 739192 / `#0b4778` | `research/applets/v4/artifacts/vineflower/y.java:1489`; `research/applets/v4/artifacts/vineflower/y.java:1577`; `research/applets/v4/artifacts/vineflower/y.java:1582`; +1 in usage JSON |
| 125 | 5682150 / `#56b3e6` | `research/applets/v4/artifacts/vineflower/au.java:2686`; `research/applets/v4/artifacts/vineflower/au.java:2714`; `research/applets/v4/artifacts/vineflower/au.java:2852` |
| 129 | [PNG 240×48](../artifacts/theme/field-129-00.png) | `research/applets/v4/artifacts/vineflower/y.java:1445` |
| 130 | [PNG 6×6](../artifacts/theme/field-130-00.png) | `research/applets/v4/artifacts/vineflower/y.java:1058` |
| 131 | [PNG 36×39](../artifacts/theme/field-131-00.png) | `research/applets/v4/artifacts/vineflower/bl.java:271`; `research/applets/v4/artifacts/vineflower/y.java:614`; `research/applets/v4/artifacts/vineflower/y.java:637` |
| 132 | [PNG 25×25](../artifacts/theme/field-132-00.png) | `research/applets/v4/artifacts/vineflower/au.java:2310` |
| 133 | [PNG 6×6](../artifacts/theme/field-133-00.png) | `research/applets/v4/artifacts/vineflower/y.java:2046` |
| 134 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/bj.java:1194`; `research/applets/v4/artifacts/vineflower/bj.java:1195` |
| 135 | 0 / `#000000` | `research/applets/v4/artifacts/vineflower/au.java:2682`; `research/applets/v4/artifacts/vineflower/au.java:2684` |
| 136 | [PNG 24×24](../artifacts/theme/field-136-00.png) | `research/applets/v4/artifacts/vineflower/y.java:1946`; `research/applets/v4/artifacts/vineflower/y.java:2488` |
| 137 | [PNG 24×24](../artifacts/theme/field-137-00.png) | `research/applets/v4/artifacts/vineflower/y.java:2490` |
| 138 | 10461087 / `#9f9f9f` | `research/applets/v4/artifacts/vineflower/bj.java:2055` |
| 139 | 16777215 / `#ffffff` | `research/applets/v4/artifacts/vineflower/bj.java:2113` |
| 141 | 40 / `#000028` | `research/applets/v4/artifacts/vineflower/bj.java:2063` |
| 142 | 550964 / `#086834` | `research/applets/v4/artifacts/vineflower/bj.java:2121` |
| 143 | 0 / `#000000` | `research/applets/v4/artifacts/vineflower/bj.java:2079` |
| 144 | 4221632 / `#406ac0` | `research/applets/v4/artifacts/vineflower/bj.java:2023` |
| 145 | 0 / `#000000` | `research/applets/v4/artifacts/vineflower/bj.java:2071` |
| 146 | 16565508 / `#fcc504` | `research/applets/v4/artifacts/vineflower/bj.java:2105` |
| 147 | 0 / `#000000` | `research/applets/v4/artifacts/vineflower/bj.java:2097` |
| 148 | 10040064 / `#993300` | `research/applets/v4/artifacts/vineflower/bj.java:2031` |
| 149 | 16711680 / `#ff0000` | `research/applets/v4/artifacts/vineflower/bj.java:2047` |
| 150 | 16741120 / `#ff7300` | `research/applets/v4/artifacts/vineflower/bj.java:2039` |
| 151 | 0 / `#000000` | `research/applets/v4/artifacts/vineflower/bj.java:2089` |
