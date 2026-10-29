# Original client room-moderation result

**PASS — 2026-10-25.** All three unchanged v4.62 clients completed room moderation and original logout with no runtime exceptions. The run preserved **124 binary frames and 18 original Canvas screenshots**.

| Check | Result |
| --- | --- |
| Moderator presentation | Original708 popup marked Alice moderator |
| Unauthorized kick | Bob 706→opcode0; original red error dialog rendered |
| Mute | Alice 709 acknowledged; original popup marked Bob muted |
| Muted messaging | Bob 500 rejected; Bob still rendered incoming room text |
| Unmute | Alice 710 acknowledged; Bob's marker became ordinary and his reply reached Alice and Carol |
| Kick | Alice 706 acknowledged; original participant popup excluded Carol |
| Kick notification | Original packet5 handler and painter produced the expected information popup; original OK action dismissed it |
| Original tab behavior | Lobby tab remained open after the kick notification |
| Enforcement after kick | Carol's500 rejected; no later Lobby500 arrived; Gaming membership remained active |
| Cleanup | All three clients sent original 300 and received matching acknowledgments |

Evidence:

- Alice: [result](runs/room-moderation-out/alice/result.txt), [log](runs/room-moderation-out/alice/runtime.log), [packet events](runs/room-moderation-out/alice/events.tsv).
- Bob: [result](runs/room-moderation-out/bob/result.txt), [log](runs/room-moderation-out/bob/runtime.log), [packet events](runs/room-moderation-out/bob/events.tsv).
- Carol: [result](runs/room-moderation-out/carol/result.txt), [log](runs/room-moderation-out/carol/runtime.log), [packet events](runs/room-moderation-out/carol/events.tsv).

Selected original-renderer captures show [moderator and muted participant icons](runs/room-moderation-out/alice/participants-muted.png), [Bob's denied kick](runs/room-moderation-out/bob/unauthorized-kick-denied.png), [Bob's muted-send rejection](runs/room-moderation-out/bob/muted-send-denied.png), [Carol's kick notification](runs/room-moderation-out/carol/kick-notification.png), and [Carol's rejected send after dismissal](runs/room-moderation-out/carol/postkick-send-denied.png).

The original notification model contained: “You were kicked from Lobby by alice.” That wording is a local server fixture; the original client supplied the dialog and rendering. The test invoked the original OK Command object and verified that the queued notification and dialog were both consumed.

This proves selected original packet-builder and receive/rendering paths. It does not prove keypad navigation through **Block & Mute User**, which uses 417 in the visible confirmation path. See [reproduction and exact UI scope](ROOM_MODERATION.md). Server tests separately cover removing a kicked account's multiple room sessions; this client run used one session per account.
