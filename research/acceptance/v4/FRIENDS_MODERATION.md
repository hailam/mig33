# Original-client friend discovery and room management

Start the local backend with a fresh dedicated test database, then run:

```sh
sh tools/applets/v4/integration/run-friend-moderation.sh
```

The [recorded acceptance run](friend-moderation-result.md) passed all three original-client phases and preserved 50 screenshots and 133 binary frames.

The runner first starts Hailam alone over the original TCP transport. Hailam searches for Bob while Bob is offline, opens his profile, activates the native Chat command, sends a queued private message, and activates Add Friend. Hailam logs out before Alice/TCP and Bob/HTTP start concurrently. Bob receives the queued message. Alice and Bob join Lobby to exercise room moderation, then log out in order.

Use a fresh database because the first phase asserts that Hailam has not already added Bob. The runner adds that contact and stores its test messages. It starts and stops only its own JVMs; it does not manage the backend. Each JVM uses an isolated temporary home and in-memory RMS. The original JAR is hash-checked during compilation, and the connector permits only the explicit loopback listeners.

## Assertions

1. The original **Search New Friends** menu opens `/sites/index.php?c=profile&a=search&v=midlet`. The original browser submits an empty search and a case-insensitive `BOB` search. Results exclude the viewer and expose native Chat/Add Friend links plus Bob's profile link.
2. Bob's profile renders **Presence: Offline** while offering Chat and Add Friend. FIRE on `mig33:privateChat(bob)` opens a native private-chat tab. The original 500 builder sends the queued message. FIRE on `mig33:inviteFriend(bob)` opens the original prefilled Add Contact form; its actual submit action emits 405 and the 402 response adds Bob to Hailam's native roster.
3. A later independent Bob JVM receives the queued message and renders it in the original private-chat tab. Both Alice and Bob join Lobby. Ordinary Bob's Room Info page contains no moderator form.
4. Alice opens Room Info through the actual room menu. The original browser parses its fixed hidden fields and submits **Mute bob** through 922 POST. Bob's room send is rejected, while his private message still reaches Alice. The native participant popup marks Bob muted.
5. Alice submits **Unmute bob** through the page; Bob's next room message is accepted. Alice mutes him again, then uses the stock **Block & Mute User → bob (Un-Mute)** picker. That marked-user action emits 710, and Bob's next room message succeeds.
6. No client emits personal-block opcode 417. Bob logs out first; Alice waits for his correlated logout marker and offline-presence event before logging out. All PASS files require their client's correlated logout ACK.

The runner writes native Canvas captures, HTML responses, binary packets, runtime logs, markers and per-client PASS files under `friend-moderation-out/`. Its final summary matches logout ACKs by transaction: concurrent recorder threads can record a received ACK before the send thread records its TX event.

## Recovered contracts and scope

The profile search and profile-home route templates are original. Browser links `mig33:privateChat(USERNAME)` and `mig33:inviteFriend(USERNAME)` are original commands. Their arguments are passed literally, without percent decoding. Private Chat creates or selects a native chat tab locally; sending the first message uses opcode 500. Invite Friend opens the original state-57 Add Contact form.

The visible **Block & Mute User** action is context-sensitive. For an ordinary unmarked participant it sends personal-block 417 after confirmation, with text tag 1 for username and byte tag 2 set to 2. For a participant listed in 708 muted-users tag 4, the same picker supplies an **Un-Mute** suffix and sends room-unmute 710 with text tags 1 and 2 for room and target. The retained 709 mute-confirmation branch has no ordinary visible entry point found in this build. The Room Info moderation form is therefore an explicit local reconstruction using the shared room-moderation service.

The search form, matching rules, profile action placement, and moderator page layout are reconstructed server behavior. This run verifies one present moderation target. Separate backend tests submit the second of two forms and check target isolation; code inspection confirms the native parser creates separate form models, but multiple-target selection was not exercised in this native run. The harness sets parsed text and focus models, then executes original menus, form serialization, FIRE activation, transport, receive dispatch and Canvas rendering. The queued message is sent through the original builder after the native Chat link opens its tab. It does not simulate handset text entry. Captures are 240×320 with desktop MicroEmulator fonts.

See the [shared runtime boundary](README.md#runtime-boundary), [friend discovery feature](../../../docs/friend-discovery.md), [room semantics](../../../docs/rooms.md), and [harness source](../../../tools/applets/v4/integration/V4FriendModerationIntegration.java).
