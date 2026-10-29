# Original-client contact integration

This sequence opens two independent, headless original v4.62 clients: **Hailam over TCP** and **Bob over HTTP**. Hailam starts with an empty Friends group, while Bob retains the demo roster. It exercises persistent, directional address books; it is not a friend-request approval workflow. See the [recorded acceptance result](contact-report.md).

Start the backend on its default ports using a **fresh temporary database**, then run:

```sh
go run ./cmd/omni -db /tmp/mig33-contacts-acceptance.sqlite
# In a second terminal:
sh tools/applets/v4/integration/run-contacts.sh
```

Choose a database path that does not already contain an earlier run. The harness deliberately rejects an existing Bob entry in Hailam's address book instead of silently deleting user data. It leaves Hailam's test Bob entry and the exchanged test messages in that database. It replaces only generated files in `contact-out/`; other acceptance evidence is preserved.

The two clients use the existing loopback-only Connector, isolated temporary homes and in-memory RMS. The original JAR hash is verified before compilation. No historical service is contacted, and neither Java binary nor packet payload is rewritten. Closing the sequence sends original logout packets and cleans up both JVMs.

## Assertions

- The original Add a Friend form is constructed with username `bob`. The harness focuses Add, then invokes the original command handler; native validation and the native405 builder produce the request, and the original402 dispatcher inserts Bob.
- Native406 plus its correlated acknowledgment removes Bob from the originating roster. Repeating the original add form restores him.
- Native408/401 creates a contact group,410/1 renames it,411/1 moves Bob, and407/402 changes Bob's local display name. Bob then moves back to Friends and409/1 deletes the empty temporary group.
- Reauthentication through the original login builder chooses419 when the saved revision matches. The original incremental synchronization completes and preserves the edited contact.
- Hailam and Bob exchange direct messages through original500 builders and chat renderers. Hailam's address-book changes do not alter Bob's own roster.

Screenshots, original roster-model TSVs and exact incoming/outgoing frames are written under `contact-out/hailam/` and `contact-out/bob/`. Each `result.txt` is created only after that client's assertions and logout succeed. The guide's companion result report records the actual run, frame counts and selected captures.

## Scope

The Add form uses its original command handler. Other contact/group actions invoke original builders with the same selected-contact/group context that their UI handlers normally set. Reflection selects tabs and focuses controls; this is not an exhaustive keypad-navigation test. Screenshots are the original240×320 Canvas renderer with MicroEmulator desktop fonts. Dynamic names, page content, account policy and server wording are reconstruction data.

This test covers the originating client's group changes. The recovered incoming protocol has no general group-delete/rename push: other simultaneously signed-in instances may need to reconnect for those headers to refresh. The backend's authorization, persistence, malformed-request handling and migration cases are separately covered by Go tests.
