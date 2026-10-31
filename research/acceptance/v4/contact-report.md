# Original-client contact management result

**PASS — 2026-10-26, final backend run.** Both unchanged v4.62 clients completed their assertions and original logout without runtime exceptions. The final run preserved **81 binary frames and15 original Canvas screenshots**: Hailam TCP56/12; Bob HTTP25/3.

| Check | Result |
| --- | --- |
| Empty initial address book | Hailam received Friends401 and no402 records; the original screen showed Friends(0/0) |
| Native Add form | Prefilled Bob, focused Add, executed original command validation and405 serialization; correlated402 inserted Bob |
| Remove and re-add | Native406 plus correlated1 removed Bob; the original Add form restored him |
| Contact groups |408/401 created Contact Test;410/1 renamed Local Friends;411/1 moved Bob into it and back to Friends;409/1 deleted the empty group |
| Contact editing |407/402 changed Bob's local display name to Robert Local without changing his username |
| Reauthentication | Original200/201/202/203 chose419 using the retained revision/checkpoint; the run received404/423, then420/403 completed synchronization and preserved Robert Local. The historical interpretation of423 as status was incorrect; see the correction below |
| Independent users | Bob retained his own Hailam contact while Hailam's roster changed |
| Messaging | Hailam TCP→Bob HTTP and Bob HTTP→Hailam TCP messages rendered in original private-chat tabs |
| Cleanup | Both clients sent original300 and received matching acknowledgments |

Evidence:

- Hailam: [result](runs/contact-out/hailam/result.txt), [runtime log](runs/contact-out/hailam/runtime.log), [packet events](runs/contact-out/hailam/events.tsv).
- Bob: [result](runs/contact-out/bob/result.txt), [runtime log](runs/contact-out/bob/runtime.log), [packet events](runs/contact-out/bob/events.tsv).

Selected original-renderer captures: [empty Friends](runs/contact-out/hailam/initial-roster.png), [original Add form](runs/contact-out/hailam/added-bob-form.png), [added Bob](runs/contact-out/hailam/added-bob.png), [after deletion](runs/contact-out/hailam/after-delete-bob.png), [edited contact in Local Friends](runs/contact-out/hailam/edited-contact-custom-group.png), [retained in Friends after419](runs/contact-out/hailam/after-incremental-relogin.png), [Hailam's chat](runs/contact-out/hailam/chat-after-add-contact.png), and [Bob's received message](runs/contact-out/bob/message-from-new-contact.png). Roster captures invoke the original scroll-to-top action so headers and counts remain visible. Group identity and membership assertions inspect original row models as well as protocol replies; matching TSVs preserve the [custom-group rows](runs/contact-out/hailam/edited-contact-custom-group-model.tsv) and [post-sync rows](runs/contact-out/hailam/after-incremental-relogin-model.tsv).

The first native add is [TX-007-405](runs/contact-out/hailam/TX-007-405.bin) and [RX-010-402](runs/contact-out/hailam/RX-010-402.bin). Reauthentication's native incremental request is [TX-018-419](runs/contact-out/hailam/TX-018-419.bin), followed by [RX-030-404](runs/contact-out/hailam/RX-030-404.bin), [RX-031-423](runs/contact-out/hailam/RX-031-423.bin), [RX-032-420](runs/contact-out/hailam/RX-032-420.bin) and [RX-033-403](runs/contact-out/hailam/RX-033-403.bin).

The original form command, serializer, dispatcher and Canvas painter execute preserved application bytecode. Reflection supplies the test username, focuses controls and selects conversation tabs. Other contact/group operations use original builders with their selected-row context. This is not a full keypad-navigation or server-restart test. The backend's persistent store and migration tests provide restart coverage. See [reproduction and scope](CONTACTS.md).

**Semantic correction, 2026-10-31:** original423 updates an avatar key, while421 updates status text. Likewise402 tag27 is status and tag28 is avatar. The original contact renderer establishes this mapping; the [source audit](../../applets/v4/docs/protocol-catalog-audit.md#contact-statusavatar-semantic-correction-2026-10-31) records the exact bytecode chain. The historical frames and PASS above remain a record of that run, which did not establish correct status/avatar rendering. They must not be cited as proof of those semantics or silently rewritten as newly captured evidence.
