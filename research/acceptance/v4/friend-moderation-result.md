# Friend discovery and room management original-client result

**PASS — 2026-10-27.** Three unchanged v4.62 clients completed friend search, offline profile Chat, Add Friend, queued private-message delivery, and room mute/unmute management against the local Go backend. Every client received its correlated logout acknowledgment and exited successfully.

The final evidence contains **50 screenshots and 133 binary frames**, with [machine-readable counts](runs/friend-moderation-out/evidence-summary.json). The original JAR SHA-256 remains `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

| Client | Transport | Screenshots | Frames | Result |
| --- | --- | ---: | ---: | --- |
| Hailam | TCP | 15 | 31 | [PASS](runs/friend-moderation-out/hailam/result.txt) |
| Alice | TCP | 28 | 57 | [PASS](runs/friend-moderation-out/alice/result.txt) |
| Bob | HTTP tunnel | 7 | 45 | [PASS](runs/friend-moderation-out/bob/result.txt) |

## Search and offline Chat

Hailam opened the original **Search New Friends** menu and submitted both an empty search and `BOB` through the native HTML serializer. Results used the original profile route and native command links. Bob's profile displayed **Presence: Offline** with working Chat and Add Friend links.

Activating `mig33:privateChat(bob)` opened the original private-chat tab. The original message builder sent `Offline profile chat from Hailam` while Bob was offline. Activating `mig33:inviteFriend(bob)` opened the original Add Contact form; submitting it sent 405 and its 402 response added Bob to Hailam's roster. Hailam then logged out. A later independent Bob JVM received and rendered the queued private message during login.

Evidence: [original search menu](runs/friend-moderation-out/hailam/search-new-friends-menu.png), [search results](runs/friend-moderation-out/hailam/friend-search-bob-result.png), [offline profile](runs/friend-moderation-out/hailam/offline-profile.png), [native private tab](runs/friend-moderation-out/hailam/offline-private-chat.png), [Add Contact form](runs/friend-moderation-out/hailam/add-friend-native-form.png), [new roster contact](runs/friend-moderation-out/hailam/added-offline-friend.png), [queued message received by Bob](runs/friend-moderation-out/bob/offline-dm-delivered-on-login.png), [Hailam wire events](runs/friend-moderation-out/hailam/events.tsv).

## Page moderation and native Un-Mute

Alice and Bob joined Lobby. Alice's original Room Info menu displayed the authorized **Mute bob** form; Bob's page had no moderation form. Alice submitted the native form, and Bob's subsequent room send was rejected. His private message still reached Alice, and the native participant popup showed Bob muted.

Alice submitted **Unmute bob**, after which Bob could send to the room again. She muted him once more through the page. The stock **Block & Mute User** participant picker then displayed **bob (Un-Mute)**; selecting it sent original opcode 710, and Bob's next room message succeeded. No client sent personal-block opcode 417.

Evidence: [moderator Room Info](runs/friend-moderation-out/alice/moderator-room-info.png), [ordinary-user Room Info](runs/friend-moderation-out/bob/ordinary-room-info.png), [mute result](runs/friend-moderation-out/alice/page-mute-bob-updated.png), [muted participant marker](runs/friend-moderation-out/alice/participants-after-page-mute.png), [private messaging while muted](runs/friend-moderation-out/alice/dm-remains-available-while-muted.png), [page unmute result](runs/friend-moderation-out/alice/page-unmute-bob-updated.png), [stock Un-Mute picker](runs/friend-moderation-out/alice/native-unmute-bob-picker.png), [native unmute acknowledgment](runs/friend-moderation-out/alice/native-unmute-success.png), [successful room message afterward](runs/friend-moderation-out/alice/reply-after-native-unmute.png), [Alice wire events](runs/friend-moderation-out/alice/events.tsv).

## Independent verification and limits

A read-only query of `/tmp/mig33-friend-moderation-20261027.sqlite` confirmed the [persisted state](runs/friend-moderation-out/persisted-state.json): Hailam's Bob contact exists, Bob's queued-message audience row has a delivery timestamp, no personal blocks exist, no room mutes remain, and neither rejected muted message was persisted.

The inspected offline profile, queued-message view, moderator form, mute result, and native Un-Mute picker rendered correctly on the original Canvas. The route templates and native commands are recovered; the HTML pages and search/mute form layouts are local reconstruction. This run uses one eligible room-management target. Backend tests cover permission and validation edge cases.

All three Java phases passed on the first native run. The initial summary script assumed TX300 would precede its ACK in the event log; Alice's two recorder threads logged those lines in the opposite order. The summary was corrected to match transactions, then rerun successfully against the preserved evidence. No native protocol assertion failed.

Text and focus are set through native parsed models, while actual menu handlers, forms, command links, transports and dispatchers execute unchanged. The message sender invokes the original builder after the native profile Chat action opens its tab. Screenshots use a 240×320 Canvas with desktop MicroEmulator fonts. See the [reproduction guide](FRIENDS_MODERATION.md) and [harness source](../../../tools/applets/v4/integration/V4FriendModerationIntegration.java).
