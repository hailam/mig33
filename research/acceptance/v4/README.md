# Original v4.62 client integration

These guides and `runs/` contain retained acceptance evidence. Active harness code lives in [tools/applets/v4/integration](../../../tools/applets/v4/integration/); fresh suite output goes to `data/applets/integration/`. Captured runs are local and Git-ignored. Historical paths inside their manifests describe the original execution; use the [relocation map](../../relocation-map.json) to find moved files.

This harness loads the unchanged publisher JAR into two independent MicroEmulator JVMs and connects them to the local Go backend. Alice uses the original TCP transport; Bob uses the original HTTP tunnel, including its empty GET polls. The original packet builders, serializers, receive dispatcher, contact/category models, private-chat renderer and page parser execute from the original JAR.

The separate [four-client group acceptance run](GROUPS.md) checks group creation, participant screens, invitation, message fanout, leaving and membership enforcement. It writes to `data/applets/integration/group-out/`, preserving this private-message run's `data/applets/integration/out/` directory.

The [public-room acceptance run](ROOMS.md) uses three original clients to browse Lobby/Gaming/Help, join rooms, inspect participant popups, exchange room messages and verify room isolation and leaving. Its evidence is separate in `data/applets/integration/room-out/`.

The [room-moderation run](ROOM_MODERATION.md) verifies moderator permissions, mute/unmute, kick notifications and enforcement through the original builders and renderers, with separate evidence in `data/applets/integration/room-moderation-out/`.

The [personal-blocking run](BLOCKING.md) exercises the visible original **Block & Mute User** confirmation, checks blocker-only private/group/room filtering, and submits the original browser's parsed unblock form. Its evidence is separate in `data/applets/integration/block-out/`.

Start the backend in one terminal:

```sh
go run ./cmd/omni
```

Then run the complete acceptance sequence from the repository root:

```sh
sh tools/applets/v4/integration/run-pair.sh
```

The script requires Python3, curl and a JDK. Defaults use `/opt/homebrew/opt/openjdk@21/bin/{java,javac}`; override `JAVA_BIN` and `JAVAC_BIN` together for another installation. It verifies the original JAR's SHA-256, rebuilds the harness, gives each client an isolated temporary home and in-memory RMS, waits for Bob's successful empty poll, runs Alice, waits for both clients and cleans up their JVMs and temporary homes. It requires the backend's default addresses and seeded accounts. It does not start or stop the backend. Each run replaces generated evidence under `data/applets/integration/out/`.

## Assertions and evidence

| Behavior | Evidence |
| --- | --- |
| Both transport logins | Original200→201→202→203 packets and populated original UI models |
| Contacts | Original400→401/402/420/403; rendered roster rows include header/group rows as well as seeded contacts |
| Room categories | Original713→714/715; three category-model rows, including built-in entries |
| HTTP polling | Original GET receives HTTP200, `image/gif`, an empty response body and continues to receive later packets; both Content-Length0 and chunked responses are supported |
| Alice TCP → Bob HTTP | Original outgoing500, Bob's original receive dispatcher and wrapped chat rows contain the expected incoming message |
| Bob HTTP → Alice TCP | Original outgoing500, Alice's original receive dispatcher and wrapped chat rows contain the expected reply |
| Message acknowledgments | Backend opcode1 responses preserved in the wire captures |
| Profile page | A new original922 request and923 reply, followed by original page parser title `User Profile` |

The human-readable logs are [Alice](runs/out/alice/runtime.log) and [Bob](runs/out/bob/runtime.log). Packet events are [Alice TSV](runs/out/alice/events.tsv) and [Bob TSV](runs/out/bob/events.tsv); each event links by filename to the exact captured binary frame in the same directory. `result.txt` appears only after that client's assertions pass. `chat-model.txt` preserves original wrapped row strings, including the original renderer's inline control characters.

Screenshots are original 240×320 Canvas painting with data received from the local backend:

- [Alice's contacts](runs/out/alice/friends-after-login.png), [room categories](runs/out/alice/room-categories.png), [Bob's reply](runs/out/alice/chat-from-bob.png), [profile page](runs/out/alice/profile-page.png).
- [Bob's contacts](runs/out/bob/friends-after-login.png), [room categories](runs/out/bob/room-categories.png), [Alice's message](runs/out/bob/chat-from-alice.png).

## Runtime boundary

The harness calls original controller and network methods through reflection to initialize login, submit messages, select tabs and request the profile page. It does not simulate keypad text entry or prove every UI action. It detaches asynchronous display painting and invokes the original Canvas painter for stable screenshots. Message screenshots show received text; direct network-builder invocation does not insert a local outgoing echo.

MicroEmulator supplies Java desktop fonts and MIDP runtime behavior. Native handset command bars and text editors are outside these Canvas captures. Local account names, messages, room labels and HTML pages are test data, not recovered historical server content. These images therefore verify original renderer compatibility with the new backend, not historical handset pixel identity.

The only networking adapter is [the integration Connector](../../../tools/applets/v4/integration/shims/javax/microedition/io/Connector.java): it permits exactly `socket://127.0.0.1:9119` and HTTP URLs with the literal host `127.0.0.1` and port8080. Other hosts, ports, protocols and URL user info are rejected before any connection. Redirects are disabled. The original startup analytics requests are recorded as `BLOCK` and never sent. The mandatory invalid-host self-test checks this boundary at startup.

The HTTP adapter preserves an original MIDP quirk: opening then flushing an empty output stream on a GET sends a GET with no body. Calling the JDK's ordinary output-stream method directly would silently turn that request into POST, so the adapter supplies an empty sink for that case. It rejects nonempty GET bodies. Wire payloads are not rewritten.

Empty polls are counted only after the response stream reaches EOF without returning any bytes. This works when the server uses chunked transfer encoding and provides no Content-Length header. Private-message assertions inspect the expected original private-chat rows so restored group notifications cannot satisfy them accidentally.

The original JAR remains byte-identical, SHA-256 `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`. `resource-bridge.jar` contains only its original `default_v2` and `en-US` bytes to support the legacy bootstrap-class resource lookup. The existing MicroEmulator `Displayable.removeCommand(null)` compatibility shim and linking-only VideoControl interface are reused from the offline capture harness. No decompiled or exception-stripped application class is executed.

## Login fixture details

The successful203 response includes tag27 as byte0 because the original dispatcher immediately unboxes it. It includes tag31 as **short0** and tag32 as an empty UTF-8 string. The original dispatcher reads31 twice: an optional integer dimension check and a later mandatory short setting. A two-byte31 payload makes the integer accessor return null after caught EOF, bypassing optional dimensions, while the short accessor returns0. Empty32 is a valid empty string. [FieldProbe.java](../../../tools/applets/v4/integration/FieldProbe.java) records the original raw-field accessor probe used to resolve this mixed interpretation. Runtime login and subsequent rendering validate this fixture through the real dispatcher.

The HTTP gateway base is `http://127.0.0.1:8080/`; the original client appends its session token to that base and also supplies the `sid` header. `/session` is not the path used in this test.

## Isolated full acceptance

Run all 17 suites against a single freshly built backend binary, with a separate
database and HTTP/TCP listener pair for each suite:

```sh
python3 tools/applets/v4/integration/isolated_acceptance.py \
  --evidence /tmp/mig33-native-acceptance
```

The evidence directory must not already exist. Use `--suites pair` for a pilot or
a comma-separated list to select suites. The runner owns its processes and uses
ports 60080–60096 and 61080–61096; it does not start, stop or access the demo listeners
on 8080/9119 or `data/chat.sqlite`. Each suite receives a fresh seeded SQLite file
and an isolated copy of the harness in `/tmp`.

The runner retains the binary, production source archive and SHA-256 manifest,
build metadata, configuration and harness manifests, per-suite server logs,
captured native frames and screenshots, and SQLite integrity results. Its summary
checks each client's PASS marker and correlated logout acknowledgment. The
publisher JAR stays unchanged. Shell copies replace the health probe's default
port and correct the older discovery/reconnect room-name
fixtures to fit the existing 15-character creation limit. Those fixture names
retain a space and a timestamp; the correction manifest records their exact
expressions and original/isolated script hashes. An isolated slash-test helper
also selects the latest presence packet for the requested contact instead of
the globally latest packet. It still requires that contact's exact ID, offline
presence 99, and logout marker; another contact's update cannot mask the assertion.
Its source and correction are recorded alongside the fixture changes. The existing Connector performs
its restricted loopback remapping. The complete inventory is 17 `run-*.sh` suites
plus shared `run.sh`.

After investigating a harness-only failure, `--resume` keeps the exact retained
binary, verifies that its production sources still match, preserves failed
attempts, and runs only failed or unfinished suites in fresh fixture databases.
Successful earlier suites keep their original evidence and provenance. Production
changes require a new evidence directory and a complete new run.
