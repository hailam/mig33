# Original client public-room acceptance result

**PASS — 2026-10-25.** Three unchanged v4.62 clients completed public-room browsing, joining, participant inspection, messaging and leaving. Alice and Carol used the original TCP transport; Bob used the original HTTP tunnel.

| Check | Observed result |
| --- | --- |
| Room catalog | Original713/714/715 and 716/701/717 produced Gaming, Help and Lobby |
| Occupancy | Original catalog rendered Lobby 2/50, then 3/50, then 2/50; Gaming remained 1/50 |
| Joining | Original703 received701 then matching acknowledgment; room model became ready and pending 703 cleared |
| Participants | Original707/708 popup showed Lobby members2→3→2 |
| Room messaging | Original conversation3 text rendered in every intended recipient's room model, with matching received500 frames |
| Room isolation | Carol in Gaming received no initial Lobby message; sending to Lobby without joining received opcode0 |
| Leaving | Original704 close sequence removed Carol's Lobby tab and its acknowledgment cleared the pending suppression entry |
| After leaving | Carol's Lobby send was rejected; no final Lobby 500 arrived, while her Gaming membership remained active |
| Logout | All three original clients sent300 and received matching acknowledgments |
| Private-chat regression | Passed afterward: six roster rows, two collapsed-category model rows, bidirectional private text, profile rendering and original logout |

The final passing run preserves **149 binary frames and 27 original Canvas screenshots**, with no runtime exceptions:

- Alice: [result](runs/room-out/alice/result.txt), [log](runs/room-out/alice/runtime.log), [packet events](runs/room-out/alice/events.tsv).
- Bob: [result](runs/room-out/bob/result.txt), [log](runs/room-out/bob/runtime.log), [packet events](runs/room-out/bob/events.tsv).
- Carol: [result](runs/room-out/carol/result.txt), [log](runs/room-out/carol/runtime.log), [packet events](runs/room-out/carol/events.tsv).

Selected captures show the [original room list with occupancy](runs/room-out/alice/rooms-three-lobby-one-gaming.png), [three-member participant popup](runs/room-out/alice/participants-three.png), [room conversation](runs/room-out/bob/room-carol-reply.png), [Carol's Gaming view after leaving Lobby](runs/room-out/carol/after-leave-lobby.png), and [post-leave rejection](runs/room-out/carol/postleave-denied.png).

The harness initially assumed the category started expanded. The server supplies a collapsed category, so the final harness invokes the original category-expansion method before refreshing. That required no server change. Failed-run research/applets/v3/artifacts were replaced by the final passing run.

This report covers public rooms, not moderation; moderation has a separate run and evidence directory. The original JAR is unchanged. Reflection drives selected original methods rather than keypad entry. Screenshots show original Canvas rendering with desktop MicroEmulator fonts and local fixture data. See [reproduction and compatibility details](ROOMS.md).
