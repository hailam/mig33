# Original v4.62 formatting acceptance: PASS

Validated on **2026-10-28** with two unchanged v4.62 clients against an isolated, freshly seeded schema 17 database at `/tmp/mig33-formatting-native-20261028-verified.sqlite`. HTTP used port 48080 and TCP used 49119. The isolated backend was stopped after verification.

| Client | Transport | Protocol frames | Canvas screenshots | Logout acknowledgment |
| --- | --- | ---: | ---: | --- |
| Alice | TCP | 94 | 26 | PASS |
| Bob | HTTP polling | 66 | 12 | PASS |

Total: **160 captured binary frames and 38 original Canvas screenshots**. The generated source PNG is excluded from the screenshot count. Both runtime logs contain no Java exceptions or assertion failures. HTML page captures require two identical, populated Canvas paints after the original deferred tab-paint flag clears.

The acceptance verifies:

- Native profile customization Save and Reset through the original 922 form serializer. HTML source and `&`, `+`, `%`, `=` punctuation survive storage and safe output; current source is escaped and the replacement editor starts blank.
- A second original client sees red profile text, a pale solid background, italic/underlined text, and an active `mig33:privateChat(carol)` link. Activating that link opens the native Carol private-chat tab.
- An original 504 upload supplies an owned profile image. The receiving client's image cache holds the decoded PNG; the actual Canvas contains its 24×24 checkerboard, including **288 exact cyan pixels**.
- `/member2/t.php` HTML messages reach private chat, group chat, and Lobby with trusted sender `alice`, plain message text, and the correct uniform RGB in packet 500 field 13. The original renderer paints red, green, and blue text respectively.
- Mixed red/blue HTML remains formatted in WAP history. Native delivery has canonical plain text and no uniform RGB field. Raw HTML does not appear in native chat rows.
- Ordinary native `[color=red]Literal[/color]` stays literal. The harness adds no historical code syntax or client-side BBCode interpretation.

Useful captures: [profile colours](runs/formatting-out/bob/other-colour-profile.png), [inline photo](runs/formatting-out/bob/other-photo-profile.png), [private red text](runs/formatting-out/bob/native-private-red.png), [group green text](runs/formatting-out/bob/native-group-green.png), [room blue text](runs/formatting-out/bob/native-room-blue.png), [rich WAP history](runs/formatting-out/bob/wap-private-rich-history.png), and [literal BBCode](runs/formatting-out/bob/native-bbcode-literal.png). The empty Carol chat opened by the custom profile link is intentional.

**Original-client limitation:** v4.62 recognizes `<body background="…">`, but does not fetch or paint that background image. Its parser places the request in an unused vector; the page painter passes a null image to its background helper. Solid background colours and inline photos are verified. The returned HTML retains the web-browser background attribute, but this report does not claim a native background image. See [bytecode](../../applets/v4/artifacts/bytecode/s.txt), [image-cache observation](runs/formatting-out/bob/profile-images.txt), and [pixel observation](runs/formatting-out/bob/native-photo-observation.txt).

This verifies a reconstructed explicit HTML-authoring service and compatibility with the original renderer. It does not establish the exact historical profile-injection payload, undocumented chat codes, handset fonts, or pixel identity with physical hardware. Screenshots are 240×320 original Canvas output under MicroEmulator, without handset softkey chrome.

Original JAR SHA256: `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

Implementation: [harness](../../../tools/applets/v4/integration/V4FormattingIntegration.java), [runner](../../../tools/applets/v4/integration/run-formatting.sh), [machine-readable evidence](runs/formatting-out/evidence-summary.json), [screenshot index](runs/formatting-out/index.html). With a freshly seeded backend on the isolated ports, run:

```sh
MIG33_TEST_HTTP_PORT=48080 MIG33_TEST_TCP_PORT=49119 \
  sh tools/applets/v4/integration/run-formatting.sh
```

The runner refuses the main 8080/9119 ports and clears only its own `formatting-out` evidence directory before a new run.
