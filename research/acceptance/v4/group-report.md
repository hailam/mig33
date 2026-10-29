# Original client group acceptance result

**PASS — 2026-10-25.** Four unchanged mig33 v4.62 clients completed the group-chat acceptance sequence against the local Go backend. Alice and Dave used the original TCP transport; Bob and Carol used the original HTTP tunnel.

| Check | Observed result |
| --- | --- |
| Create751→750 | Alice's existing original private tab converted to the returned group ID |
| Participants754→755 | Original popup contained exactly Alice/Bob/Carol, then all four users, then Alice/Bob/Dave after Carol left |
| Three-member conversation 2 messages | Alice's text rendered in Bob and Carol; Bob's reply rendered in Alice and Carol |
| Outsider send | Dave's original 500 received correlated opcode0; original renderer displayed the rejection |
| Invite752 | Correlated acknowledgment; Dave joined the participant list and received the next group message |
| Four-member conversation 2 messages | Alice's text rendered in Bob, Carol and Dave; Dave's reply rendered in Alice, Bob and Carol |
| Leave753 | Carol received the correlated acknowledgment and disappeared from the original participant popup |
| Post-leave send | Carol's original 500 received correlated opcode0 and an in-chat error |
| Post-leave delivery | Bob and Dave rendered the final message; Carol continued polling and did not receive that message |
| Private-chat regression | Existing two-client run passed afterward against the same database with restored group notifications present; roster 6 rows, categories 3 rows |

The final group run preserved **130 exact binary frames and 18 original Canvas screenshots**. All four runtime logs completed without exceptions. These paths contain only the final passing run; the earlier harness failure was replaced when rerunning:

- Alice: [result](runs/group-out/alice/result.txt), [runtime log](runs/group-out/alice/runtime.log), [packet events](runs/group-out/alice/events.tsv).
- Bob: [result](runs/group-out/bob/result.txt), [runtime log](runs/group-out/bob/runtime.log), [packet events](runs/group-out/bob/events.tsv).
- Carol: [result](runs/group-out/carol/result.txt), [runtime log](runs/group-out/carol/runtime.log), [packet events](runs/group-out/carol/events.tsv).
- Dave: [result](runs/group-out/dave/result.txt), [runtime log](runs/group-out/dave/runtime.log), [packet events](runs/group-out/dave/events.tsv).

Selected original-renderer captures:

- Participants: [three members](runs/group-out/alice/participants-three.png), [four members](runs/group-out/alice/participants-four.png), [after leaving](runs/group-out/alice/participants-after-leave.png).
- Messages: [Carol receiving group replies](runs/group-out/carol/group-dave-reply.png), [Dave after invitation](runs/group-out/dave/group-after-invite.png), [Bob after Carol leaves](runs/group-out/bob/group-after-carol-leaves.png).
- Rejections: [Dave before invitation](runs/group-out/dave/outsider-denied.png), [Carol after leaving](runs/group-out/carol/postleave-denied.png).

One harness defect was corrected before this passing run: HTTP empty polls were initially recognized only through Content-Length0. The backend's chunked responses omit that length; the adapter now counts an empty poll from actual EOF with zero bytes. The original client already handled those responses correctly. Private-chat waits were also tightened to inspect the expected original private conversation text, so restored group500 packets cannot satisfy them prematurely.

The original JAR remains unchanged and hash-verified. Actions use reflection to invoke original methods; the test does not automate keypad input or prove every group UI action. Screenshots use the original 240×320 Canvas renderer with MicroEmulator fonts and local test data. See [group reproduction and scope](GROUPS.md) and the shared [runtime boundary](README.md#runtime-boundary). Durable storage and restart behavior have separate Go tests; this client run does not claim to establish restart durability.
