# Slash commands original-client result

**PASS — 2026-10-27.** Four JVM runs of the unchanged v4.62 client completed native slash-command entry, room/private/group action rendering, inline error feedback, and offline action delivery. The final runner exited 0, and every client received its correlated logout acknowledgment.

The final evidence contains **68 screenshots and 227 binary frames**, with [machine-readable counts](runs/slash-out/evidence-summary.json). The original JAR SHA-256 remains `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

| Client | Transport | Screenshots | Frames | Result |
| --- | --- | ---: | ---: | --- |
| Alice | TCP | 37 | 114 | [PASS](runs/slash-out/alice/result.txt) |
| Bob | HTTP tunnel | 15 | 45 | [PASS](runs/slash-out/bob/result.txt) |
| Carol | TCP | 14 | 44 | [PASS](runs/slash-out/carol/result.txt) |
| Returning Bob | HTTP tunnel | 2 | 24 | [PASS](runs/slash-out/returning-bob/result.txt) |

## Native entry and room actions

Alice opened the actual **Write** menu, filled the original TextBox, and activated its original Send command for all 11 supported commands: `/me`, `/hug`, `/slap`, `/wave`, `/smile`, `/laugh`, `/cry`, `/brb`, `/roll`, `/fortune`, and `/8ball`. Every outgoing 500 retained the entered command and selected color `#336699`.

Each room member, including Alice, received exactly one transformed 500 result. Each result used content type 6 with color tag 13 omitted. The original renderer displayed the complete action once, without an extra sender colon or avatar. The sender's original editor did not add a local raw-slash row, and neither recipient received the raw command. This run's random examples were `alice rolls 89`, `alice fortune: Make time for someone you have missed.`, and `alice 8ball: Ask again later.`

Evidence: [original Write menu](runs/slash-out/alice/room-action-00-write-menu.png), [submitted editor value](runs/slash-out/alice/room-action-00-editor.txt), [all 11 rendered actions](runs/slash-out/alice/room-action-10.png), [Bob's rendered actions](runs/slash-out/bob/room-action-10.png), [Carol's rendered actions](runs/slash-out/carol/room-action-10.png), [native text model](runs/slash-out/alice/room-action-10-model.txt), [Alice wire events](runs/slash-out/alice/events.tsv).

## Errors and conversation routing

`/definitely_missing`, `/hug missing`, and `/kick bob` received correlated opcode 0 responses. The original client displayed red inline Error lines with the available-command, invalid-target, and unavailable-kick guidance. The subsequent native participant popup still contained Alice, Bob and Carol.

A private `/me` action produced `alice checks private action routing`. Alice's own result stayed in her Bob tab, with tag 2 set to the routing peer `bob`; Bob's copy used tag 2 `alice` and appeared in his Alice tab. The body retained the authenticated actor. Alice had no self-chat tab, and Carol received no private copy.

The clients then created a private group. `/wave bob` produced one `alice waves to bob.` action in all three group tabs. Finally, after Bob and Carol logged out, Alice sent an action to offline Bob. Her own result stayed in the Bob tab. A fresh Bob JVM received `alice queues an offline action` with content type 6 and displayed it once in the Alice tab.

Evidence: [inline errors](runs/slash-out/alice/unavailable-kick-error.png), [unchanged participants](runs/slash-out/alice/members-unchanged-after-errors.png), [Alice's own private echo](runs/slash-out/alice/private-own-action-echo.png), [Bob's private result](runs/slash-out/bob/private-action-from-alice.png), [Carol's group result](runs/slash-out/carol/group-action-from-alice.png), [Alice's offline-send echo](runs/slash-out/alice/offline-own-action-echo.png), [queued action after Bob logs in](runs/slash-out/returning-bob/offline-action-delivered-after-login.png), [returning Bob wire events](runs/slash-out/returning-bob/events.tsv).

## Independent storage verification

A read-only query of `/tmp/mig33-slash-20261027.sqlite` verified schema version 8 and the [persisted action records](runs/slash-out/persisted-actions.json). Final-run message IDs 4–17 contain 11 room actions, two private actions, and one group action. Every record has authenticated sender `alice`, `action=1`, and a null stored color. No message body begins with a raw slash command. The offline action's Alice and Bob audience rows both have delivery timestamps.

IDs 1–3 came from two preliminary native-editor timing attempts. Those attempts stored three room actions before the harness added waits for MicroEmulator's asynchronous display transitions. They are explicitly separated from the final run in the storage evidence. All preserved screenshots, packet captures and PASS files belong to the successful final run.

## Interpretation and scope

The room, private, group, offline and error screenshots were visually inspected. Content type 6 with default color is validated as the original prefix-free rendering path. The original applet's received tag 14 does not provide that behavior. Own-private echo routing needs the peer in tag 2, while server persistence and action text retain the real actor.

The harness assigns text to the original TextBox and invokes the real Send command after that editor becomes current. It does not simulate individual handset key presses. Packet builders, transports, receive dispatch and Canvas rendering execute unchanged. Captures use the original 240×320 Canvas with desktop MicroEmulator fonts. Command provenance and reconstructed wording/random policies are documented in the [feature guide](../../../docs/slash-commands.md); the native run establishes compatibility rather than historical server-output identity.

See the [reproduction guide](SLASH_COMMANDS.md) and [harness source](../../../tools/applets/v4/integration/V4SlashIntegration.java).
