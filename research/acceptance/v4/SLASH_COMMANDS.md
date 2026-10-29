# Original-client slash-command acceptance

The [2026-10-27 acceptance run](slash-result.md) passed with 68 screenshots and 227 binary frames.

Start the local backend with a dedicated test database, then run:

```sh
sh tools/applets/v4/integration/run-slash.sh
```

Alice and Carol use the original TCP transport; Bob uses the original HTTP tunnel. Each unchanged v4.62 client runs in an independent JVM with an isolated temporary home and in-memory RMS. After the live clients exit, a fresh Bob JVM verifies queued private-action delivery. The runner starts and stops its JVMs, while the backend is managed separately.

The test stores transformed action messages, creates a private group, and leaves those records in its test database. Use a dedicated database for repeatable evidence. The original JAR is hash-checked during compilation and the connector permits only the two explicit loopback listeners.

## Native assertions

1. Alice uses the actual **Write** menu, original TextBox, and original Send command for `/me`, `/hug`, `/slap`, `/wave`, `/smile`, `/laugh`, `/cry`, `/brb`, `/roll`, `/fortune`, and `/8ball` in Lobby. The harness sets a selected text color and checks that the original request carries it.
2. Each client receives exactly one expanded action packet. The incoming content type is 6, color tag 13 is absent, and the original renderer shows the action without an added `username:` prefix. The original editor does not locally echo slash text, and recipients receive no raw slash command.
3. Unsupported commands, invalid targets, and unavailable `/kick` produce the original inline Error feedback. The participant list remains unchanged.
4. A private action appears once in Alice's Bob tab and Bob's Alice tab. Alice receives no self-chat tab. Carol receives no copy of the private action.
5. A private-group action reaches all three original group tabs with the same content-6 rendering.
6. After Bob and Carol log out, Alice sends another private action while Bob is offline. Her echo stays in the Bob tab. A fresh Bob JVM receives the stored action with content type 6 and renders it once.
7. Every PASS file requires its client's correlated logout acknowledgment. Event summaries match ACKs by transaction rather than assuming recorder-thread ordering.

## Renderer contract

The original editor trims submitted text and suppresses local display for every string beginning `/` (`aj.java`, normal flow around 3093–3136). A text-message ACK does not display successful command output. The server therefore sends the transformed result back to the sender as well as other recipients.

Incoming content type 6 suppresses the sender prefix only when message-color tag 13 is absent or -1. The original `s.a` renderer then clears its local display-sender value, disables the avatar, and uses theme category 5's color. Supplying an explicit color bypasses that branch and brings back the sender prefix. Packet tag 2 can therefore retain the authenticated actor for room/group recipients without adding a colon. Tag 14 does not suppress sender prefixes and is left unset.

Private receive routing uses tag 2 as the conversation peer. An action echo sent to its own author therefore uses the destination peer in tag 2, while the expanded body and persisted sender retain the authenticated actor. Recipient packets use the actor normally. The harness checks the resulting tabs and body, not merely the packet schema.

The action vocabulary, output templates and random-result policies have separate research provenance. This runner verifies compatibility with the unchanged client; it does not establish historical server text or random distributions. Text values are assigned through the actual native TextBox rather than simulated handset key presses. All editor command handling, serialization, network transports, incoming dispatch and Canvas rendering execute unchanged. The harness waits for asynchronous MicroEmulator display transitions before activating Send. Captures use the original 240×320 Canvas with desktop fonts.

See the [shared runtime boundary](README.md#runtime-boundary), [command provenance and feature guide](../../../docs/slash-commands.md), and [harness source](../../../tools/applets/v4/integration/V4SlashIntegration.java).
