# Vote kick and Report Abuse original-client result

**PASS — 2026-10-26.** Five unchanged v4.62 clients completed the native report, vote-kick and moderator-kick sequence against the local Go backend. Bob used the original HTTP tunnel; Hailam, Carol, Dave and Alice used original TCP. All five sent original 300 logout and received their correlated acknowledgments.

The final run preserved **40 screenshots and 224 binary frames**, with [machine-readable counts](runs/vote-report-out/evidence-summary.json). Its report was independently verified in the dedicated SQLite database `/tmp/mig33-vote-report-20261026.sqlite` through a read-only connection. The original JAR remained SHA-256 `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

| Client | Transport | Screenshots | Frames | Result |
| --- | --- | ---: | ---: | --- |
| Hailam | TCP | 17 | 50 | [PASS](runs/vote-report-out/hailam/result.txt) |
| Bob | HTTP | 4 | 40 | [PASS](runs/vote-report-out/bob/result.txt) |
| Carol | TCP | 7 | 47 | [PASS](runs/vote-report-out/carol/result.txt) |
| Dave | TCP | 6 | 45 | [PASS](runs/vote-report-out/dave/result.txt) |
| Alice | TCP | 6 | 42 | [PASS](runs/vote-report-out/alice/result.txt) |

## Report Abuse

Hailam opened the original room **Menu → Report Abuse** action. Its original 922 request retained the route `/sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=Chatroom&subject_id=Lobby`. The original browser parsed the hidden fields and visible controls, then its own serializer and FIRE activation submitted the form through 922 POST. The correlated 923 response had status 200 and rendered **Report Submitted**.

The original parser produced a **228-pixel textarea**, equal to its page width minus four. The filled form was visually inspected. The parser uses special textarea column values: `5` selects half width, `7` selects three quarters, and `10` selects full width. The replacement page uses `cols="10"`; the harness asserts the resulting native pixel width.

The stored [report #3](runs/vote-report-out/persisted-report.json) contains reporter `hailam`, context `room / Lobby`, target `bob`, category `spam`, status `pending`, and exactly:

```text
Repeated spam & flooding + 50% noise = test
```

The ampersand, plus, percent and equals characters survived the original serializer, backend decoding and persistence unchanged. All five room members remained after the report.

Evidence: [room menu](runs/vote-report-out/hailam/report-room-menu.png), [empty form](runs/vote-report-out/hailam/report-form.png), [filled form](runs/vote-report-out/hailam/report-submit-focused.png), [native form model](runs/vote-report-out/hailam/report-form-model.txt), [confirmation](runs/vote-report-out/hailam/report-submitted.png), [unchanged membership](runs/vote-report-out/hailam/report-does-not-kick.png), [wire events](runs/vote-report-out/hailam/events.tsv).

## Ordinary-user votes and moderator kick

Hailam, Carol and Dave each used the actual **Menu → Kick User → bob → Yes** sequence. The unchanged client emitted the same 706 packet, with room in tag 1 and target in tag 2. Hailam's first request produced `1/3`; repeating it acknowledged the request and displayed the duplicate-vote notice without adding a vote. Carol's next request produced `2/3`, and the original participant popup still contained all five users. Dave's third distinct vote produced `3/3` and removed Bob.

Bob's original receive dispatcher and Canvas displayed **You were kicked from Lobby by a room vote.** His attempted room send received an opcode 0 error. A subsequent room message reached remaining participants and did not reach Bob. Bob then rejoined successfully. Alice's moderator request, through the same native menu and confirmation, kicked him immediately; the popup read **You were kicked from Lobby by alice.** His next room send was rejected again.

Evidence: [Hailam's Kick User menu](runs/vote-report-out/hailam/start-vote-menu.png), [participant picker](runs/vote-report-out/hailam/start-vote-picker.png), [confirmation](runs/vote-report-out/hailam/start-vote-confirmation.png), [vote start](runs/vote-report-out/hailam/vote-started.png), [Carol's second vote](runs/vote-report-out/carol/second-vote-count.png), [members after two distinct votes](runs/vote-report-out/carol/two-votes-still-five-members.png), [third-vote result](runs/vote-report-out/dave/third-vote-count.png), [Bob's vote popup](runs/vote-report-out/bob/vote-kick-notification.png), [Alice's native confirmation](runs/vote-report-out/alice/moderator-instant-kick-confirmation.png), [Bob's moderator popup](runs/vote-report-out/bob/moderator-kick-notification.png).

## Interpretation and runtime boundary

This capture used the original local three-vote rule. The subsequent [occupancy-based implementation](../../../docs/rooms.md#regular-user-vote-kicking) still requires three votes for this five-member room; larger bands are covered by Go tests rather than this capture. The60-second expiry and60-second initiator cooldown remain local reconstruction policy. The recovered client establishes the menu, confirmation and706 payload; it does not establish historical server thresholds or timing. The abuse-report pages are local replacements. This run verifies the native room-report flow; group-context reporting and vote-policy edge cases are covered separately by backend tests.

The harness selects native menus and executes their original handlers. It supplies select and textarea values through the original parsed models rather than simulating handset text entry. Screenshots use the original 240×320 Canvas painter with desktop MicroEmulator fonts. They establish compatibility with the unchanged client, not historical handset pixel identity. The original room tab remains after a kick.

Teardown is deliberately ordered. The original client sets its logout flag immediately after sending 300 and terminates on its next received packet, even an unrelated departure or presence update. Each client therefore waits for the preceding client's logout acknowledgment and offline event before sending its own 300. This avoids mistaking the original client's early shutdown for a missing server acknowledgment.

The runner permits only the two explicit loopback listeners. Original external analytics attempts were blocked and captured. All five JVMs exited after passing; the runner exited 0. See [the guide](VOTE_REPORT.md) and [harness source](../../../tools/applets/v4/integration/V4VoteReportIntegration.java) to reproduce the sequence.
