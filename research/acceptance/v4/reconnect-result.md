# Native retained-tab reconnect acceptance result

**PASS — 2026-10-27**, unchanged Java ME v4.62 build.000. Original JAR SHA-256 `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

Three original clients completed the [reconnect runner](RECONNECT.md) against isolated HTTP18080/TCP19119 and `/tmp/mig33-completion-reconnect-final-v16.sqlite`. Main app data/listeners were not used. Evidence contains **29 screenshots and312 binary frames**, with all three correlated logout acknowledgments and no exception/assertion/error runtime lines.

| Client | Transport | Screenshots | Frames | Observed203 /703 |
| --- | --- | --- | --- | --- |
| Alice, room owner | HTTP |12|85|1 /1|
| Bob, reconnecting member | TCP |14|174|6 /6|
| Carol, capacity fixture | HTTP |3|53|1 /1|

Alice created the room through native forms, set capacity2, applied/lifted a ban through Room Settings, and kicked Bob through the original706 builder. Carol joined/left using the original native room model. Bob retained the same room tab across all reconnects.

Verified behavior:

- A successful native reconnect203 automatically caused the retained tab to send703. Membership/readiness restored and sending succeeded.
- A room filled during disconnection rejected rejoin with `This chat room is full`. The original retained tab displayed the error in red, remained unavailable, and a send attempt was rejected.
- A room ban rejected the next automatic rejoin with `You are banned from this chat room`. The original tab rendered the error and sending remained rejected.
- Removing the ban allowed the next native reconnect and message.
- Native706 kick displayed the original notification5 popup. The tab remained present; sending failed until a later reconnect issued703 and succeeded. A kick alone is therefore distinct from the persistent ban in this local contract.

The final full-room, banned-room and kick-popup screenshots were visually inspected. The full/banned errors occupy the original room transcript, and the kick is the original modal notification over the retained room tab.

Evidence: [summary](runs/reconnect-out/evidence-summary.json), [full room](runs/reconnect-out/bob/full-denied.png), [banned room](runs/reconnect-out/bob/ban-denied.png), [kick popup](runs/reconnect-out/bob/kick-notification.png), [final owner participant list](runs/reconnect-out/alice/final-participants-model.txt).

Harness corrections made during validation: use the original complete login preparation; invoke the actual connection-loss path with reconnect progress enabled; paint through the original deferred-tab recovery before assessing readiness; balance nested MIDP RMS opens in the MicroEmulator compatibility shim; and give frames global filenames so new TCP connections cannot overwrite earlier evidence. None modifies the original applet JAR, forces room readiness or injects membership. The retry delay is120 seconds only to coordinate fixtures; the original Reconnect Now method starts the login immediately.
