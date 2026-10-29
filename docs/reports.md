# Report Abuse

The v4 client's original **Report Abuse** menu now opens a working form, saves a private pending report, and returns a reference number. Reports survive server restart. Submission does not kick, ban, mute, notify the reported user, or imply that a human has reviewed the report.

This is a local replacement for unavailable historical server HTML. The original JAR, native menu, URL shape, and packet transport are preserved; the form's labels, categories, validation, storage, and operator workflow are reconstructed behavior, not recovered mig33 policy.

## Client endpoint

```text
GET|POST /sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=Chatroom&subject_id=Lobby
GET|POST /sites/index.php?c=report&a=report_abuse&v=midlet&type=4&subject=Group%20Chat&subject_id=<group-id>
```

Both ordinary HTTP and the client's page request packet `922` reach the same handler; native responses use packet `923`. The existing authenticated session supplies the reporter identity. Client-supplied reporter names are rejected.

GET checks current membership and returns the form without creating a report. POST checks membership again before validating and saving it. Chat room membership is the requesting session's live room membership; private group membership comes from the persisted group. A room report stores the canonical room name, while a private group report stores its group ID. A room URL never implicitly identifies a particular accused user.

The form title is **Report Abuse**. Its action is an absolute URL on the configured local server origin. It has these named controls, in this order:

| Field | Control | Meaning |
| --- | --- | --- |
| `csrf` | Hidden | Token belonging to the authenticated session. |
| `nonce` | Hidden | Random 48-character lowercase hexadecimal form identifier. |
| `binding` | Hidden | HMAC-SHA256 binding the nonce to the session, conversation kind, and canonical conversation identifier. |
| `category` | Select | `spam` (default), `harassment`, `content`, or `other`. |
| `target` | Select | Empty for the conversation in general, or a currently present known account other than the reporter. |
| `details` | Textarea | Required description; final named field. |

The submit control is unnamed and labeled **Submit Report**. All displayed dynamic text is HTML escaped. The native HTML parser needs literal `&` query separators in the action; individual URL values are escaped. The textarea uses `rows="4" cols="10"`: the recovered parser interprets `cols=10` as the page width minus four pixels, `5` as half width, and `7` as three-quarter width. Other values, including a conventional `24`, leave its control width zero. This is a native parser contract, not a standard browser column count.

Descriptions must contain non-whitespace text, valid UTF-8, no NUL character, and at most 2,000 UTF-8 bytes. The full submitted form is bounded to 8,192 bytes. Duplicate controls, unknown controls, unknown categories, and invalid or departed targets are rejected. An empty target is valid, including a conversation with no other members.

A successful response has title **Report Submitted** and includes `Report #N saved for local review.` The stored report begins with status `pending`. Repeating an identical submission with the same reporter and nonce returns the same reference; it does not create another report. Reusing that nonce with different report content returns `409`. Replay still requires a valid session, matching form binding, and current conversation/target membership.

| Response | Meaning |
| --- | --- |
| `200` | Form opened, or report saved/already saved identically. |
| `400` | Malformed context, controls, target, or description. |
| `401` | Missing or expired authenticated session. |
| `403` | Not a conversation member, invalid session token, or mismatched form binding. |
| `404` | Unknown chat room. |
| `405` | Unsupported HTTP method. |
| `409` | Submitted form reused with different content. |
| `415` | Ordinary HTTP POST is not form URL encoded. |
| `503` | Report could not be persisted. |

## Original native form encoding

The recovered JAR's form serializer concatenates values directly and only replaces spaces with `%20`; it does not generally URL-encode text. A normal query parser would corrupt `+` and treat `&` in a description as another form control.

For report POSTs transported by packet `922`, the adapter parses the fixed/select controls before the first `&details=` and treats the entire remaining body as the description. It decodes only `%20` to a space. Keeping the textarea last and the submit control unnamed preserves literal `&`, `=`, `+`, `%`, Unicode, newlines, and text such as `&csrf=anything` without letting text override control values. The original JAR is unchanged. The parser contract is covered by server tests, including a punctuation and Unicode payload.

A literal `%20` typed into the original JAR is indistinguishable from a space encoded by that JAR and is therefore saved as a space. This is a limitation of its original serializer. Ordinary HTTP forms use standard URL encoding and can preserve a literal `%20` by encoding its percent sign.

## Local operator review

The client cannot list reports, read another user's report, inspect review notes, or mark a report reviewed. There is no public review endpoint. The following CLI commands are trusted local operator actions: access is controlled by permission to run the operator CLI and open its SQLite database. Use the same `-db` path as the running local server.

```sh
go run ./cmd/cli -db data/chat.sqlite -reports
go run ./cmd/cli -db data/chat.sqlite -reports -report-status all -report-limit 100
go run ./cmd/cli -db data/chat.sqlite -report-id 1
go run ./cmd/cli -db data/chat.sqlite -review-report 1 -report-status reviewed -report-note 'Reviewed the submitted details.'
go run ./cmd/cli -db data/chat.sqlite -review-report 1 -report-status dismissed -report-note 'Insufficient information to act.'
```

Commands print JSON and exit without starting listeners. The queue defaults to pending reports, newest first. A review requires status `reviewed` or `dismissed` and a nonempty note within the same text bounds as a description. Reviews may change a previous decision; each changed decision/note appends a timestamped audit record. Repeating an identical current status/note is idempotent. Updating the report and appending its review record are one transaction. A review does not itself execute a kick, ban, or mute.

Reports contain their ID, trusted reporter, conversation kind and identifier, optional target, category, description, status, creation/update timestamps in Unix milliseconds, and latest review note. Detail inspection also returns the ordered review history. The private form nonce is not included in operator JSON.

The Go operator APIs are `Server.ListReports(status, limit)`, `Server.ReportDetails(id)`, and `Server.ReviewReport(id, status, note)`. Empty status means all at the storage/API layer; the CLI accepts `all`. These APIs are intentionally not routed to the client.

## Persistence and checks

SQLite schema version 6 adds `abuse_reports` with a unique `(reporter, nonce)` key and `abuse_report_reviews` for audit entries. Existing messages, contacts, blocks, rooms, and moderation data are preserved by migration. Reports remain available if the conversation later becomes inaccessible to the reporter.

Automated checks cover schema migration, restart persistence, concurrent duplicate submission, changed replay rejection, group membership, failed-review transaction rollback, session/CSRF isolation, conversation binding, membership changes before submission, size/content validation, private report visibility, HTML escaping, native form parsing, and absence of an automatic kick.

The original v4.62 client acceptance harness also opens the real room Report Abuse menu, fills the parsed controls, submits through the original FIRE action and unchanged packet `922` serializer, and checks packet `923` success plus an unchanged participant roster. Its [form model](../research/acceptance/v4/runs/vote-report-out/hailam/report-form-model.txt) records a 228-pixel textarea at 240-pixel screen width. The [filled form screenshot](../research/acceptance/v4/runs/vote-report-out/hailam/report-submit-focused.png), [confirmation](../research/acceptance/v4/runs/vote-report-out/hailam/report-confirmation.html), and [PASS result](../research/acceptance/v4/runs/vote-report-out/hailam/result.txt) capture the corrected native rendering and submission.

```sh
go test -race ./internal/adapters/sqlite/repositories ./internal/pages ./internal/realtime ./internal/review ./internal/acceptance
```
