# Original-client vote kick and abuse reporting

Start the current local backend, then run:

```sh
sh tools/applets/v4/integration/run-vote-report.sh
```

Use a dedicated test database with the standard seeded accounts and no other clients in Lobby. This run creates a persisted abuse report and briefly removes Bob from Lobby. Hailam uses password `hailam`; the remaining accounts use their seeded `<username>-demo` passwords. Alice has the seeded moderator role. The runner neither starts nor stops the backend.

Five independent JVMs load the unchanged v4.62 JAR. Bob uses its original HTTP transport; Hailam, Carol, Dave and Alice use its original TCP transport. The shared loopback-only adapter rejects external destinations. Each client has an isolated temporary home and in-memory RMS. Evidence is written to `vote-report-out/`, leaving the other integration runs intact.

The sequence checks:

1. Hailam opens the actual room **Menu → Report Abuse** action. The original browser parses the returned form. The harness checks that its textarea has full page width, selects category `spam`, target `bob`, and details `Repeated spam & flooding + 50% noise = test` in the original parsed control models, then activates the original submit control using its FIRE key. The original serializer emits 922 POST unchanged and the original browser renders 923 status 200 with **Report Submitted**. All five room participants remain.
2. Hailam opens **Menu → Kick User → bob → Yes**. The original menu, participant selector and confirmation emit 706, with tag1 `Lobby` and tag2 `bob`. The original chat renderer receives the first `1/3` vote notice.
3. Hailam repeats the action. The backend acknowledges it and returns the duplicate-vote notice. Bob remains in the room. Carol then uses the same native menu path and sees `2/3`, proving the repeated vote did not increment the count.
4. Dave uses the same native menu path. The third distinct vote removes Bob and produces a `3/3` room notice. Bob's original dispatcher and Canvas show **You were kicked from Lobby by a room vote.** His later room send is rejected and he receives no subsequent room text.
5. Bob rejoins. Alice uses the same native menu path; her moderator request kicks Bob immediately. The original popup names Alice. Bob's subsequent room send is rejected. All clients perform original 300 logout before their result files are written. Logouts are ordered, with the preceding client's offline notification drained first, because the original client exits on the next received packet after sending 300.

Screenshots, menu confirmations, form-model summaries, original rendered room text, event TSV and exact binary frames are captured per client. See the [recorded result](vote-report-result.md).

## Recovered behavior and reconstruction policy

The native v4.62 room menu includes **Kick User** for ordinary members. Both ordinary members and moderators use 706; there is no separate recovered vote-kick opcode or separate **Vote Kick** menu entry. The JAR sends a room and target, so server policy determines whether that request starts a vote or performs an immediate kick. The native room's **Report Abuse** action opens `/sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=Chatroom&subject_id=Lobby` through 922/923.

The current required count follows the [documented occupancy table](../../../docs/rooms.md#regular-user-vote-kicking), ranging from three to nine including the initiator. This five-member test room requires three. Each ballot fixes its count from unique active room accounts at initiation; the60-second ballot and60-second initiator cooldown remain local policy, with one active ballot per room. A vote can start in a smaller room; newly joined members can vote. The target cannot vote for their own removal, moderators are protected, and muted members cannot vote. Leaving withdraws a member's vote when their last room session leaves. A ballot cancels if its target leaves or it loses every vote. The community sources disagree at20 users; the implementation follows the2010 table, with nine above25 as an unverified fallback. Fixed ballot thresholds, timings and edge policies are reconstruction choices. The original-client run covers the five-member flow and duplicate handling; Go tests cover the larger occupancy bands and policy edges.

The report form and its confirmation are local replacement server pages. A submitted report is stored for local review; it does not itself apply moderation. The harness executes the original browser parser and serializer, but supplies control values through reflection instead of simulating handset text entry. Native form serialization only escapes spaces, so the server preserves punctuation in the final `details` field. A literal `%20` in native details is indistinguishable from an escaped space. The original parser treats textarea `cols="10"` as full width; ordinary HTML column counts such as 24 leave a narrow default field. The acceptance run checks the original parsed pixel width and captures the filled form.

The [shared runtime boundary](README.md#runtime-boundary) applies: original bytecode and Canvas renderer, desktop MicroEmulator fonts, 240×320 captures and local fixture content. These are compatibility evidence, not historical handset pixel-identity claims. The original room tab remains after a kick; removal is enforced by the server and explained by the original information popup.
