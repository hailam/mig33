# Original-client pages and text-color acceptance

The [2026-10-27 acceptance run](pages-result.md) checks all 52 embedded-page URL templates, native form submissions, and a message sent through the original text-color picker. Open the [native screenshot gallery](runs/pages-out/index.html) for the latest successful run.

Start the local backend on its normal loopback listeners with a dedicated test database and seeded Alice/Bob/Carol accounts, then run:

```sh
sh tools/applets/v4/integration/run-pages.sh
```

The runner compiles the shared integration harness and `V4PagesIntegration.java`, checks backend health, starts Alice over TCP and Bob over the HTTP tunnel in independent JVMs, and stops its own processes. The backend is managed separately. `--compile-only` compiles without launching clients. `--audit-only` checks existing captures and regenerates their summary/gallery without launching clients or contacting the backend. The normal run replaces its previous `pages-out` evidence; successful operations persist rooms, a community and forum post, messages, a private group, credit ledger entries, purchases, and the selected theme in the test database.

## Assertions

1. Alice opens the actual **Write** editor and its **Change Text Color** command, moves the original palette selection to red, and activates the original Select command. Sending from the original TextBox produces opcode 500 with RGB tag 13 equal to `16711680`. Bob receives that color and the original message renderer produces a red body. The palette itself came from opcode 924.
2. Original browser forms create an owned room and a public community. Separate owner forms set room capacity to 70, save keywords, and disable user vote-kicking. The harness selects the text control belonging to each chosen form, then checks the returned persisted value. A native forum submission preserves `Native & plus + percent 50% equals = café 世界` despite the client's unusual raw form serialization.
3. All 52 `embedded_page_template` literals in the endpoint catalog are visited through the original browser request method and opcode 922. Context-sensitive URLs receive an existing room, a valid private-group identifier, an actual community `cid`, and suitable seeded account values.
4. Each page must return a correlated successful 923 response. The harness tokenizes the returned bytes through the original tokenizer and waits for the actual browser's loaded token vector and page title to match, with a nonempty parsed display model. It then waits for the original tab selector's deferred-paint flag to clear and invokes the original Canvas painter until two consecutive content-area images are identical and populated. The raw HTML, model text, PNG and pixel metrics are retained. The same paint check applies to every before-submit capture.
5. The native browser submits Buzz, local mail, credit recharge, credit transfer, gift purchase, theme purchase and theme selection forms. For every submission, the outgoing method/action/body must match the original browser's own serialization of the selected form. Bob verifies the delivered local mail, received Rose, and transferred balance through his native pages.
6. Theme selection receives a complete opcode 1000 packet. The loaded original theme's chat-background field changes from white, and the actual chat Canvas is captured with the new background.
7. Bob opens the original Updates tab and subscribes with opcode 921. After the initial backlog, Alice's new native Buzz form submission must produce a live 422 update. The original activity renderer must contain the safe summary and link; its native Canvas is captured before an acknowledged unsubscribe.
8. Both clients must receive the logout ACK matching their own logout transaction. The runner independently checks those recorded frames before generating its evidence summary.

## Evidence and limits

The gallery and `evidence-summary.json` are generated only after both clients report PASS and an independent image audit succeeds. Per-client directories retain `events.tsv`, binary frames, page HTML, `pages.tsv`, `page-render-metrics.tsv`, parsed-model text, screenshots and runtime logs. `capture-pixel-audit.tsv` checks every saved PNG's content rectangle `(4,24)–(232,306)`, excluding the tab bar, borders and scrollbar. A populated capture requires at least two colors and no single color covering 99.5% or more of that rectangle. Two-color content is valid: the original palette grid's interior is orange and white. The sole allowed empty image is the deliberately blank chat used to show the theme's background color. `routes.tsv` records the 52 literal routes and context placeholders used by the run. Unique timestamp names avoid collisions with previous room/community fixtures.

This paint check corrects an earlier verification gap: nine screenshots captured gray bodies while their parsed page models were already populated. The original `k.g` selects a tab with its `c` flag set; `k.run` clears that flag after a 400 ms stable-tab delay, and `aj.paint` draws only the gray body while it remains set. The harness observes that original behavior rather than overriding the flag. Parser completion alone is insufficient screenshot evidence.

The test runs the unchanged original v4.62 JAR in MicroEmulator at 240×320 with desktop fonts; handset softkeys are outside the captured Canvas. It invokes original commands and methods by reflection and assigns editor text through the original widgets. It does not simulate every handset keystroke or validate every link, branch, scrolling position and error condition on all 52 services. Registration and password flows have a [separate native acceptance run](IDENTITY.md).

These are local replacement pages rendered by the original app. Passing this test establishes compatibility with the native browser, color picker and transports; it does not establish that every replacement page matches an unavailable historical server design. See the [recovered historical design catalog](../../applets/v4/docs/historical-page-designs.md), [exact formatting contract](../../../docs/text-and-page-formatting.md), [shared runtime boundary](README.md#runtime-boundary), and [harness source](../../../tools/applets/v4/integration/V4PagesIntegration.java).
