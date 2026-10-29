# Personal Block & Mute and Manage Block List

The original v4.62 **Block & Mute User** action now works against the Go backend. It sends personal block417, not room-moderator mute709. Personal blocks belong to the blocking account and persist across sessions and server restarts.

## Behavior

When Alice blocks Bob:

- Bob's incoming direct, private-group and public-room text is suppressed for Alice. Other users still receive Bob's messages.
- Existing undelivered Bob→Alice messages are canceled in SQLite and buffered copies are canceled in all Alice sessions. Unblocking does not replay them.
- New messages written while blocked do not grant Alice a history/delivery audience. Bob keeps his own sent-message history; private sends receive a normal acknowledgment without revealing the block.
- Existing conversation history is retained. A block does not erase earlier history, change room/group membership or change profiles/presence.
- Alice can still send outgoing text. The block is directional; it does not create a reverse Bob→Alice block.
- Bob cannot bypass the block by creating a new private group with Alice or inviting Alice to another group. Already-existing group membership remains intact.
- Server room notices, kick notifications and moderator enforcement remain visible. Blocking a moderator's human-authored text does not disable their permissions.

The audience filtering, block update and queue cancellation are transactional. Pending-message retrieval checks blocks again, as does the final transport boundary. Per-account write gates make block completion wait for already-started server writes without stalling unrelated users; later writes recheck the committed block. Bytes already sent over a network cannot be recalled.

These delivery policies are explicit reconstruction choices. The archived client proves the request/menu contract, but does not contain the historical server's full block policy.

## Native request and original page URL

| Interface | Behavior |
| --- | --- |
| `417`, `1:T username`, `2:B2` | Block another configured user and return correlated opcode1; repeated blocks are idempotent |
| Invalid/self/unknown target or another mode | Opcode0 error; no mutation |
| `GET /sites/index.php?c=settings&a=block_list&v=midlet` | Authenticated Manage Block List page listing the caller's blocks |
| `POST` to that same URL | Authenticated unblock form: `action=unblock`, `username`, session-bound `csrf` |

The original417 builder always emits mode2, even when its boolean argument is false. No guessed binary unblock mode was added. The page is available both directly over HTTP with `sid` and through the original922/923 embedded-page transport. GET never changes block state. POST rejects wrong/missing tokens, ambiguous fields and invalid targets. Form data is limited to8KiB.

The URL is recovered; the page body and unblock POST fields are newly implemented. Each form action uses the configured **absolute HTTP origin** plus the original path/query. Actual native requests showed that a filename-only action could lose `/sites/`; a leading `/sites/` also does not follow ordinary browser resolution in this parser. The original parser leaves attribute entities undecoded, so fixed query ampersands are literal. Dynamic values are escaped; current usernames and CSRF tokens are ASCII-safe for the old limited form serializer.

## Use and validation

In the original client, select **Block & Mute User**, choose a user and accept its confirmation. To unblock, open **Settings → Manage Block List** and activate that user's **Unblock** form. A participant already marked muted by the room moderator follows a different original action: room unmute710. See the [room-moderation distinction](rooms.md).

```sh
go run ./cmd/omni
# In another terminal:
sh tools/applets/v4/integration/run-block.sh
```

The original-client harness uses the real room menu, participant picker and block confirmation, then the original Manage Block List browser and form activation. It also checks that other room/group recipients remain unaffected and that new direct messages arrive after unblock. [Native test guide and evidence](../research/acceptance/v4/BLOCKING.md).

Go tests additionally cover persistence/restart, direct and tunneled POSTs, session isolation and CSRF rejection, malformed modes, queued cancellation across sessions, no replay after unblock, reduced history audiences, invitation bypass prevention, room moderator/system-message exemptions and synchronization of concurrent writes. SQLite migration4 preserves the previous chat, room and moderation data.

Personal blocking is now implemented. Real accounts/contacts, broader privacy settings and the recreated frontend remain separate work; the [target and gaps](target-and-gaps.md) document distinguishes those requirements from completed backend behavior.
