# Original-client personal blocking integration

Start the current local backend, then run:

```sh
sh tools/applets/v4/integration/run-block.sh
```

Three unchanged v4.62 clients log in as Alice and Carol over TCP and Bob over HTTP. The runner uses the verified original JAR, loopback-only networking adapter, isolated temporary homes and in-memory RMS described in the [shared runtime boundary](README.md#runtime-boundary). It replaces only `block-out/`, preserving the earlier private, group, public-room and moderation evidence. Each client completes original 300 logout before a successful exit.

The sequence joins Lobby and creates a private group containing all three accounts, then checks:

1. Alice opens the original room menu, selects **Block & Mute User**, selects Bob in the original participant picker, and accepts the original confirmation. That visible path produces 417 with username `bob` and mode byte2, followed by an acknowledgment.
2. Bob sends private, group and room text. Alice receives none of those three 500 payloads. Carol receives and renders Bob's group and room text. Bob remains a Lobby member; this personal block does not perform a moderator mute or remove him.
3. Alice's outgoing private message still reaches Bob.
4. The original Privacy menu's **Manage Block List** action fetches the local page using 922/923. The original HTML parser retains Bob's unblock form and its hidden `action`, `username` and session CSRF fields.
5. The harness focuses the original submit control and calls the original controller's FIRE-key handler. The original browser serializes its parsed hidden fields and submits them in a new 922 request. The test checks that the URL and body exactly match the original form serializer's output, requires 923 status200, and waits for the refreshed **Manage Block List** page with Bob's form removed.
6. Bob's new private message reaches Alice after unblocking. None of the old blocked text payloads appears in Alice's captured incoming frames, including after unblocking.

See the [recorded result](block-report.md). Each client's directory contains exact binary frames, packet events, runtime logs, original Canvas screenshots and a `result.txt` written only after the complete sequence and logout pass.

## UI scope and page compatibility

The block path executes the original menu construction, menu navigation methods, participant selector, confirmation and original Command handler. The harness opens the Privacy menu through its original menu-construction method and selects its original item. It selects tabs and the HTML submit focus through reflection; it does not simulate every keypad action. The submit action itself uses the emulator's FIRE key code and original `keyPressed`, followed by the original form serializer and network builder. No unblock URL, form body or CSRF value is substituted into the outgoing request by the harness.

The local HTML page is reconstruction fixture content, not recovered historical server HTML. Its form uses an **absolute action URL with literal ampersands**. Runtime checks exposed two original parser limitations: it retains `&amp;` literally in form action attributes, and relative action resolution can lose or duplicate the `/sites/` directory. The original parser supports hidden inputs and an `<input type="submit">` button. The tested fields contain plain account/action text and a hexadecimal CSRF value; this does not establish support for arbitrary modern HTML or form-encoding edge cases.

Screenshots use the original 240×320 Canvas painter with MicroEmulator desktop fonts. Native handset command bars and text editors are outside the capture. The [moderation run](ROOM_MODERATION.md) separately tests room-level 706/709/710 behavior. Queue cancellation before a block, cross-session CSRF rejection, direct HTTP form handling, persistence across restart and other backend edge cases are server-test coverage, not claims made by this three-client run.
