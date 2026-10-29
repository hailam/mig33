# Room discovery and favorites original-client result

**PASS — 2026-10-27.** Three independent JVMs running the unchanged v4.62 client completed Create Room, Search Rooms, Room Info, native room joining, and favorite add/retrieve/remove against the local Go backend. Each phase ended with original opcode 300 logout and its correlated ACK before the next phase began.

The run preserved **40 screenshots and 127 binary frames**, with [machine-readable counts](runs/room-discovery-out/evidence-summary.json). The original JAR remained SHA-256 `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

| Phase | User | Transport | Screenshots | Frames | Result |
| --- | --- | --- | ---: | ---: | --- |
| Creator | Hailam | TCP | 20 | 58 | [PASS](runs/room-discovery-out/creator/result.txt) |
| Observer | Bob | HTTP tunnel | 14 | 38 | [PASS](runs/room-discovery-out/observer/result.txt) |
| Returning | Hailam | HTTP tunnel | 6 | 31 | [PASS](runs/room-discovery-out/returning/result.txt) |

## Create, join and inspect

Hailam opened **Chat Rooms → Create Room** through the original menu. The original browser parsed the name form and submitted it through opcode 922. The response supplied the second form, whose full-width native textarea accepted:

```text
Local friends & chat + 50% fun = yes
```

The second native POST returned **Room Created**. Activating its original `mig33:joinChatroom(Discovery 20261027100827)` link emitted opcode 703 with the literal space preserved. The original receive dispatcher opened the room tab. Its **Room Info** menu action displayed the room name, occupancy, capacity 50, owner Hailam, moderator Hailam, and the exact description.

A read-only query of `/tmp/mig33-room-discovery-refresh-20261027.sqlite` independently verified the [persisted room and moderator](runs/room-discovery-out/persisted-room.json) after all clients had exited. The punctuation-bearing description survived the original serializer, backend decoding and SQLite persistence unchanged.

Evidence: [name form](runs/room-discovery-out/creator/create-name-form.png), [filled full-width details form](runs/room-discovery-out/creator/create-details-submit.png), [creation result](runs/room-discovery-out/creator/create-details-result.png), [joined room](runs/room-discovery-out/creator/created-room-joined.png), [Room Info](runs/room-discovery-out/creator/created-room-info.png), [wire events](runs/room-discovery-out/creator/events.tsv).

## Search and account isolation

Bob's independent client first fetched Favorites and did not see Hailam's saved room. He opened the original **Search Rooms** action and submitted the empty input. The original serializer omitted that value; the local page accepted the resulting empty query and returned the available rooms. Bob then submitted the exact new room name, activated its Join Room link, and opened Room Info from the resulting room tab. His native participant popup confirmed membership.

Evidence: [Bob's empty Favorites](runs/room-discovery-out/observer/other-account-favorites.png), [empty-query results](runs/room-discovery-out/observer/search-empty-query-result.png), [exact-query results](runs/room-discovery-out/observer/search-created-room-rendered.png), [joined room](runs/room-discovery-out/observer/search-result-joined.png), [Room Info](runs/room-discovery-out/observer/observer-room-info.png), [participants](runs/room-discovery-out/observer/observer-participants.png), [wire events](runs/room-discovery-out/observer/events.tsv).

## Favorites and a new login

The original room **Save to Favorites** action emitted opcode 711. Its correlated 701 response and subsequent category 2 fetch exposed one native favorite row. Repeating Save in the same loaded session received ACK 1 and kept the row count at one. This matters because the original 701 handler inserts a favorite row without deduplicating it.

A fresh Hailam JVM with empty local RMS fetched category 2 and recovered the saved favorite. Its actual **Remove from Favorites** menu action emitted opcode 712 with category 2 and the canonical room name. The row disappeared, stayed absent after another category fetch, and the created room remained in the global catalog. The independent SQLite query confirmed that no favorite remained for this room.

Evidence: [favorite after add](runs/room-discovery-out/creator/favorites-after-add.png), [repeat-add response](runs/room-discovery-out/creator/favorite-repeat-ack.png), [favorite after new login](runs/room-discovery-out/returning/favorites-after-new-login.png), [original removal menu](runs/room-discovery-out/returning/remove-favorite-menu.png), [empty Favorites after refresh](runs/room-discovery-out/returning/favorites-empty-after-refresh.png), [room retained in the catalog](runs/room-discovery-out/returning/global-room-survives-unfavorite.png), [wire events](runs/room-discovery-out/returning/events.tsv).

## Repeated catalog refresh

The harness activated the native **Refresh** footer through FIRE, once in Favorites and twice in the global catalog. Every request used opcode 716 with flag 0, and every resulting catalog retained exactly one row per room. Both category headers set opcode 714 tag 3 to 1, causing the original client to clear previous rows before this footer request. This is necessary because the local server returns complete category snapshots; tag 3 set to 0 would append duplicate rooms on footer refresh. The separate native general Refresh action also clears rows before its flag-1 requests.

Evidence: [Favorites refresh](runs/room-discovery-out/creator/favorites-footer-refresh.png), [first global refresh](runs/room-discovery-out/creator/global-footer-refresh.png), [second global refresh](runs/room-discovery-out/creator/global-footer-repeat.png), [unique native rows after the second refresh](runs/room-discovery-out/creator/global-footer-repeat-model.txt).

## Interpretation and runtime boundary

The harness invokes actual native menu commands, original browser parsing, form serialization, link activation, network transports and receive dispatch. Text values and focus are assigned through the parsed native models instead of simulated handset text entry. The selected creation, room-info, search and Favorites screenshots were visually inspected. Captures use the original 240×320 Canvas with desktop MicroEmulator fonts; they establish compatibility, not historical handset pixel identity.

The PHP-style routes and native protocol contracts are recovered from the client. These HTML pages, ownership rules, room limits and catalog policy are local reconstruction choices. The native run proves that favorite persistence survives a fresh JVM/login; backend tests separately cover server/store restart persistence. It does not attempt a server restart during the native sequence.

The runner permits only the two explicit loopback listeners and records blocked original external analytics attempts. All three JVMs exited after passing, and the runner exited 0. See the [guide](ROOM_DISCOVERY.md), [harness source](../../../tools/applets/v4/integration/V4RoomDiscoveryIntegration.java), and [backend feature contract](../../../docs/room-discovery.md) for reproduction and limitations.
