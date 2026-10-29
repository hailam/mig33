# Original-client room moderation integration

Start the current local backend, then run:

```sh
sh tools/applets/v4/integration/run-room-moderation.sh
```

The seeded local policy makes Alice a moderator in Lobby, Gaming and Help. Bob and Carol are ordinary members. These are reconstruction test permissions, not recovered historical account privileges.

The harness uses three unchanged v4.62 clients: Alice and Carol over TCP, Bob over HTTP. It writes only `room-moderation-out/`, preserving the [basic room](ROOMS.md), [group](GROUPS.md) and [private](README.md) runs. It uses the same verified original JAR, loopback-only adapter, isolated temporary homes and in-memory RMS. Successful clients complete original 300 logout before exiting.

The test joins all three users to Lobby and Carol to Gaming, then verifies:

1. Original707/708 participant data marks Alice as moderator. The original popup's item selectors are 1 for moderator,2 for muted and 3 for ordinary.
2. Bob's original706 request targeting moderator Alice is rejected with opcode0 and an original error dialog. Ordinary-user requests against eligible regular members now start/join votes; the [five-client vote test](VOTE_REPORT.md) covers that flow.
3. Alice's original 709 request mutes Bob. A refreshed original participant popup marks Bob muted. His500 send is rejected; Alice's later room text still reaches him and Carol.
4. Alice's original 710 request unmutes Bob. His participant marker returns to ordinary, and his reply is rendered by Alice and Carol.
5. Alice's original 706 kick removes Carol from the Lobby participant list. Carol receives packet5, which the original Canvas painter turns into its information popup. The harness checks its text, captures it and invokes the original OK command to dismiss it.
6. The original Lobby tab remains after the notification. Carol's later500 send is rejected, and no subsequent Lobby500 message arrives. A successful Gaming participant request confirms her other room remains active.

`runtime.log`, event TSV, exact binary frames, original model text and screenshots are preserved for each client. `result.txt` is written only after the complete sequence and logout pass. See the [recorded moderation result](room-moderation-report.md).

The older recorded moderation captures predate ordinary-user vote-kicking and rejected Bob's request against Carol. The current runner targets protected moderator Alice for this negative check; the recorded vote/report run proves the new ordinary-user behavior separately.

## UI and protocol scope

The test invokes original packet builders for 706/709/710, then verifies original receive handlers, dialogs, participant popups and room rendering. It does not simulate selecting every visible menu action.

The visible **Block & Mute User** action is a distinct path: selector 105 opens confirmation 107, whose accepted action calls the417 block-user builder. It must not be described as a tested menu path to709. The retained action 33 handler sends709; selecting an already-muted user in selector 105 sends710. The local backend's room-moderator mute/unmute policy is tested through the original 709/710 packet builders. The separate [personal-blocking run](BLOCKING.md) exercises the visible confirmation417 and original browser unblock form.

There is no recovered room-only forced-close packet in the v4.62 dispatcher. The backend sends a packet5 notice and enforces removal server-side. The test deliberately confirms that the original tab stays open; it does not manufacture a close-tab push or claim historical kick presentation beyond the original information popup behavior.

The shared [runtime and screenshot boundaries](README.md#runtime-boundary) apply: original application bytecode, reflection-driven actions, desktop MicroEmulator fonts,240×320 Canvas, local fixture data and no legacy external network access.
