# Original client personal-blocking result

**PASS — 2026-10-26.** All three unchanged v4.62 clients completed the personal-blocking sequence and original logout with no runtime exceptions. The final run preserved **114 binary frames and 15 original Canvas screenshots**: Alice TCP43/10, Bob HTTP39/2 and Carol TCP32/3.

| Check | Result |
| --- | --- |
| Visible block flow | Original room menu → Bob participant selection → original **Block & Mute User** confirmation → 417 with username `bob`, mode byte2 → acknowledgment |
| Private text | Bob's blocked private500 absent from Alice's incoming frames |
| Group text | Bob's group500 absent from Alice; Carol's original group renderer displayed it |
| Room text | Bob's Lobby500 absent from Alice; Carol's original room renderer displayed it |
| Membership | Original participant popup still listed Alice, Bob and Carol after the personal block |
| Outgoing text | Alice's private message still reached Bob's original chat renderer |
| Block-list page | Original Privacy item fetched the page through922/923; original parser retained Bob's form and hidden fields |
| Unblock | Original browser form serializer and FIRE handler sent the parsed URL and body unchanged in922;923 returned status200 and the refreshed native page displayed “bob has been unblocked.” |
| After unblocking | Bob's new private500 reached Alice and rendered; all three earlier blocked text payloads remained absent from Alice's incoming frame captures |
| Cleanup | All three clients sent original300 and received matching acknowledgments |

Evidence:

- Alice: [result](runs/block-out/alice/result.txt), [log](runs/block-out/alice/runtime.log), [packet events](runs/block-out/alice/events.tsv).
- Bob: [result](runs/block-out/bob/result.txt), [log](runs/block-out/bob/runtime.log), [packet events](runs/block-out/bob/events.tsv).
- Carol: [result](runs/block-out/carol/result.txt), [log](runs/block-out/carol/runtime.log), [packet events](runs/block-out/carol/events.tsv).

Selected original-renderer captures show the [room action](runs/block-out/alice/room-block-menu.png), [Bob picker](runs/block-out/alice/block-user-picker.png), [confirmation](runs/block-out/alice/block-confirmation.png), [block-list form](runs/block-out/alice/manage-block-list.png), [successful unblock page](runs/block-out/alice/block-list-after-unblock.png), [new private message](runs/block-out/alice/private-after-unblock.png), and Carol's [room](runs/block-out/carol/bob-room-visible-to-carol.png) and [group](runs/block-out/carol/bob-group-visible-to-carol.png) messages from Bob.

The final original POST is [Alice TX-014-922](runs/block-out/alice/TX-014-922.bin); its successful response is [RX-026-923](runs/block-out/alice/RX-026-923.bin). The parsed form summary [redacts the session CSRF value](runs/block-out/alice/native-form-model.txt); exact captured packet files retain the local test session data. The earlier failed URL-resolution attempts were replaced by this final passing run. Earlier acceptance suites' evidence directories were preserved.

The menu confirmation and browser submit handlers execute original application bytecode. Reflection opens menus, selects tabs and focuses the parsed submit control; this is not a full keypad-navigation test. Local HTML, account names, messages and block policy are reconstruction fixtures. The screenshots use original Canvas painting with MicroEmulator desktop fonts. See [reproduction and exact scope](BLOCKING.md).
