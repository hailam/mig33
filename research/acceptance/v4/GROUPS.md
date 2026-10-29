# Original-client group integration

The [recorded acceptance result](group-report.md) passed with all four original clients, including participant screens, group message delivery and rejected sends before invitation and after leaving.

Run the local backend with the Alice, Bob, Carol and Dave test participants, then execute:

```sh
sh tools/applets/v4/integration/run-group.sh
```

This starts four separate unchanged v4.62 JARs. Alice and Dave use the original TCP transport. Bob and Carol use the original HTTP tunnel. Each JVM has isolated in-memory RMS and a temporary home. The existing loopback-only Connector bridge is reused. Output is kept in `group-out/`, separately from the private-message acceptance run in `out/`.

The script compiles the harness, verifies the original JAR hash, waits for each client, and returns nonzero if any client assertion fails. Individual steps have a 60-second deadline. Run it against the default local addresses, HTTP 127.0.0.1:8080 and TCP 127.0.0.1:9119. It requires the `alice`, `bob`, `carol` and `dave` accounts with passwords `<username>-demo`.

## Sequence

1. All four original clients log in and receive contacts/categories. The harness opens an original private conversation between Alice and Bob from the original contact model.
2. Alice calls the original 751 builder to convert that private conversation into a group with Carol. The actual750 response's group ID is decoded using the original packet parser. Alice's original group tab must contain that same ID.
3. Alice's original 754 request receives755. The original participant popup must contain exactly Alice, Bob and Carol. The popup and its names are saved.
4. Alice sends a conversation 2 message through the original 500 builder. Bob and Carol must contain the message in their original wrapped chat rows. Bob replies; Alice and Carol's original group renderers must contain his reply.
5. Dave tries to send to the group before being invited. The original request must receive a matching opcode0 error. This also exercises the original client's in-chat error rendering.
6. Alice invites Dave through752. A second original 755 participant popup must show all four users. Alice sends another group message; Bob, Carol and Dave must each render it. Dave replies, and all other members must render that reply.
7. Carol leaves through753 and receives a matching acknowledgment. A subsequent group send by Carol must receive a matching opcode0 error.
8. Alice's third755 participant popup must contain Alice, Bob and Dave only. Alice sends a final message; Bob and Dave must render it. Carol continues HTTP polling and must not receive this final message.

All actions use original packet builders; original serializers and receive dispatchers run over real local sockets/HTTP. Marker files coordinate when the JVMs take the next test action and do not insert any server response or application model data.

## Evidence

Each `group-out/<username>/` directory contains `runtime.log`, packet-event TSV, exact binary frames, original wrapped chat rows and screenshots. `result.txt` is written only after that client's entire sequence passes.

Alice also produces the original `participants-three.png`, `participants-four.png` and `participants-after-leave.png` popups, with corresponding `*-model.txt` files. Dave's `outsider-denied.png` and Carol's `postleave-denied.png` show how the original client displays those rejected sends.

This verifies selected original client group flows, not every UI action. The harness invokes controller/network methods by reflection, selects tabs and invokes the original Canvas painter. Sending through a network builder does not insert a local outgoing echo. Likewise, the leave test deliberately keeps Carol's local group view available so it can attempt and verify a rejected post-leave send. That test arrangement does not claim to reproduce the ordinary keypad-driven close-tab sequence.

The shared [runtime boundary and screenshot qualifications](README.md#runtime-boundary) apply: desktop MicroEmulator fonts,240×320 Canvas, local fixture data, no native handset command bars and no historical server-page fidelity claim. Durable storage, restart recovery and broader backend behavior are verified separately by the Go tests described in the backend guide.
