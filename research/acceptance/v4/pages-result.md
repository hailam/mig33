# Pages and text-color acceptance result

Run date: 2026-10-27. Original v4.62 JAR SHA-256: `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

**PASS after the templ migration:** all 52 literal embedded-page URL templates rendered in the unchanged original client. Fifteen native POST submissions, original text-color selection and red message delivery, recipient-side mail/gift/balance checks, live native theme application, room settings, and a live Updates notification also passed against the dedicated schema-v12 backend.

| Client | Transport | Page opens (GET) | Native POST forms | Screenshots | Binary frames | Logout ACK |
|---|---|---:|---:|---:|---:|---|
| Alice | TCP | 62 | 15 | 98 | 188 | Correlated |
| Bob | HTTP tunnel | 3 | 0 | 6 | 38 | Correlated |
| Total | Both original transports | 65 | 15 | 104 | 226 | Both observed |

The form checks cover room creation, public-community creation, a forum post, Buzz, local mail, test-credit recharge, credit transfer, a Rose gift, an Ocean blue theme purchase, and native theme application. Three owner forms additionally set capacity to 70, save keywords, and disable ordinary-user vote-kicking, with the saved values verified in the returned native page. Recipient checks confirm a 1.25 transfer and Rose delivery. Punctuation and Unicode survive the original client's form serialization. The native color picker sends red `500/13 = 16711680`, and Bob's original chat view renders the message in red. The complete theme packet changes the chat Canvas background visibly.

Bob opens the original Updates tab and subscribes using 921. Alice's new Buzz post produces a live 422 packet after the initial backlog; the original activity renderer displays `alice posted an update.` and its Read update link. Bob then unsubscribes and receives the correlated acknowledgment.

Representative screenshots were visually inspected across the acceptance runs: the actual color picker, red recipient message, room-search page, community forum with punctuation/Unicode, delivered local mail, changed chat background, saved room settings, and the live Updates entry. All 52 page checks additionally compare the actual browser's parsed token stream with the original tokenizer's output for the correlated response and require a populated display model.

The final rerun also verifies actual Canvas output. All **95 page and before-submit captures** passed the original tab-paint readiness check and two consecutive identical populated-body paints. An independent audit of **all 104 PNGs** found **103 populated captures, one intentionally empty theme-background chat, and zero blank page captures**. The metrics are recorded in [`capture-pixel-audit.tsv`](runs/pages-out/capture-pixel-audit.tsv) and the per-client `page-render-metrics.tsv` files. The 65 GET opens plus 15 submitted-form responses account for 80 page-result captures; the remaining 15 page captures show forms before submission.

This supersedes nine earlier gray-body screenshots. Those had valid parsed models but were saved during the original tab selector's 400 ms deferred-paint interval (`k.g`/`k.run`, observed by `aj.paint`). Waiting for the original flag to clear and asserting actual body pixels fixed the evidence gap without changing the app or backend. The latest Room Settings and Account History captures were visually checked after that correction.

Open the [104-image native gallery](runs/pages-out/index.html) or the [machine-readable evidence summary](runs/pages-out/evidence-summary.json). The [runner specification](PAGES.md) describes reproduction and limits. Captures show replacement backend pages inside the original browser; historical design recovery is tracked separately and remains incomplete.
