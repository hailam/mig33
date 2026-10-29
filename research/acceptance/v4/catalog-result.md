# Original v4.62 catalog acceptance: PASS

Validated **2026-10-28** against an isolated, freshly seeded schema18 database at `/tmp/mig33-catalog-native-v18-final-20261028.sqlite`, with HTTP48080 and TCP49119. Both clients execute the unchanged original JAR. The isolated backend was stopped after verification; the main server and database were not used.

| Client | Transport | Captured protocol frames | Original Canvas screenshots | Logout ACK |
| --- | --- | ---: | ---: | --- |
| Alice | TCP | 138 | 44 | PASS |
| Bob | HTTP polling | 65 | 9 | PASS |

Total: **203 wire frames and53 screenshots**. There were13 explicit page opens and8 native form submissions. Page captures require two identical populated Canvas paints after the original deferred tab-paint flag clears. Both runtime logs contain no Java exceptions or assertion failures.

Verified flows:

- Store pagination: eight products per page, native Next/Previous links, product details, balance, recipient and purchase/cancel controls.
- Native test recharge, Lovely Pack purchase, refreshed916 entitlement, and original913/914 type1 decoding of `[pack:lovely-pack]`. Bob downloads and renders the received placeholder without owning the sender's pack.
- Big Hug purchase through its native form, recipient inventory, one recipient action message, and original type3 animation decoding.
- `/gift bob Kiss -m Native gift note & plus + equals =` sends one gift and preserves its literal note in native chat, stored inventory and escaped HTML. `/gift rate Kiss 4`, `/gift display Kiss` and Bob's `/gift gifts` show persisted rating/received-gift state. The1–5 scale remains an explicit local rule.
- `/gift search hug` and `/gift display Big Hug` handle recovered command names and multiword names; their discovery responses remain private to Alice even while Bob occupies the same room.
- Games lists executable local replacements and historical reference entries. Moonbase details have no play control. `/games` returns discovery only to its requester.
- A15-character room is created through both original forms, linked to a newly created community Group, then `/bot dice` starts the local bot and the recovered two-word `/bot stop` stops it.
- Public `/api/catalog` reports78 products,25 game entries and explicit provider-mock/disconnected modes. This is a separate static HTTP check, not an original JAR endpoint.

The read-only [persistence proof](runs/catalog-out/persistence-proof.json) verifies exactly one Big Hug and one Kiss debit/inventory/message, one Lovely Pack purchase, Alice's remaining6.00 test credits, the exact gift note, one4/5 rating, the stopped bot state, SQLite integrity and foreign keys. Discovery did not create extra gift transactions.

The native run exposed that newline-separated local gift notices paint without visible spacing in this renderer. New gift notices now use ordinary spaces and semicolons; gift receipts separate the icon from `Message:` with a space. The final captures verify the correction without modifying the original JAR. This is separate from ordinary sender formatting, which already inserts `sender: message`.

Representative captures: [gift confirmation](runs/catalog-out/alice/big-hug-detail.png), [pack detail](runs/catalog-out/alice/lovely-pack-detail.png), [gift note in chat](runs/catalog-out/bob/command-gift-note-chat.png), [rating/details](runs/catalog-out/alice/rated-gift-display.png), [received gifts](runs/catalog-out/bob/received-gifts-result.png), [unavailable historical game](runs/catalog-out/alice/moonbase-reference.png), and [bot stopped](runs/catalog-out/alice/bot-stopped-result.png). The [gallery](runs/catalog-out/index.html), [wire/count metadata](runs/catalog-out/evidence-summary.json), and [public catalog snapshot](runs/catalog-out/public-catalog.json) contain the complete evidence.

Historical product names are source-backed; new IDs, test prices, aliases and placeholder representations are local. The12px images use the recovered default atlas, and gift animations use local frame sequences. These are **not recovered standalone historical pack downloads**. The23 reference games are unavailable for play; this suite does not claim their original engines or billing rules. Screenshots are240×320 original Canvas output under MicroEmulator, without physical handset softkey chrome.

Original JAR SHA256: `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

Implementation: [harness](../../../tools/applets/v4/integration/V4CatalogIntegration.java) and [runner](../../../tools/applets/v4/integration/run-catalog.sh). With a fresh isolated backend on the specified ports:

```sh
MIG33_TEST_HTTP_PORT=48080 MIG33_TEST_TCP_PORT=49119 \
  MIG33_TEST_DB=/tmp/your-fresh-catalog-fixture.sqlite \
  sh tools/applets/v4/integration/run-catalog.sh
```

The runner refuses the main8080/9119 ports and clears only its own `catalog-out` evidence directory. `MIG33_TEST_DB` enables the optional read-only persistence assertions against that fixture; it never changes the database.
