# Local operator review

The browser dashboard at `/operator` reviews abuse reports and support requests, and manages room categories. It uses the same persistent accounts as the original client. This is a local administration interface; its layout and access policy are reconstruction choices, not recovered mig33 pages.

## Enable and sign in

There are no default operators. Start the server with an explicit allowlist:

```sh
go run ./cmd/omni -operators alice
```

Open `http://127.0.0.1:8080/operator`, then sign in with Alice's current password (`alice-demo` for an unchanged seed). Multiple operators use a comma-separated list. Every named account must already exist. Omitting the flag disables every operator route with HTTP404; it does not retain privileges from an earlier server run.

The browser login verifies the persistent PBKDF2 password verifier. Its session cookie is HttpOnly, SameSite=Strict and restricted to `/operator`; it is Secure when served over TLS. The normal authenticated `sid` header can also access the dashboard, but the account must still be allowlisted. Ordinary users, room moderators and community Group moderators have no operator access. ConfigureOperators replaces the runtime allowlist atomically; removed accounts immediately lose access, including already-open browser sessions.

The login form is bound to a short-lived browser cookie. Every mutation and logout requires the current session's CSRF token. Submitted cross-origin requests with a foreign Origin header are rejected. Sessions expire through the existing session lifecycle and are invalidated by applicable password/account operations. No external messages, charges, provider calls or automatic account punishment occur when an operator changes a review status.

## Queue and decisions

The dashboard lists reports and support requests in pages of100, newest first, with links to older entries. Report detail includes reporter, target, room/group context, reason and submitted evidence. Support detail includes the sender and request text. Text is escaped through the shared templ components, including review notes and potentially hostile report content.

Reports can become `reviewed`, `dismissed` or `pending` (reopened). Support requests can become `completed`, `dismissed` or `open` (reopened). A nonblank note is required. Decisions include an expected previous state; a stale form receives HTTP409 instead of overwriting a newer decision. An operator-and-form nonce makes repeating the same POST idempotent and rejects reuse with different content.

Schema15 stores an append-only `operator_reviews` audit with work-item kind/ID, actor, status, note and time. The decision, this audit and the existing report review history commit in one transaction. Previous CLI review records are retained and remain visible in the complete report history; those old records do not retroactively acquire a named actor. Review notes are operator-only and do not become public posts or outbound support replies.

## Categories

The same dashboard adds and renames room categories, deletes empty categories and assigns an existing room to a category. Category1 is the base room category and cannot be deleted;2 belongs to the native per-account Favorites view and cannot be reassigned. IDs3–32767 are available for local categories, subject to the store's30-category limit. Category and assignment changes are recorded in the room audit with the operator's username. These controls do not grant room-owner or community Group roles.

## HTTP contract

| Route | Method | Behavior |
| --- | --- | --- |
| `/operator/login` | GET, POST | Login form and password verification |
| `/operator` | GET | Queue, pagination, categories and assignment controls |
| `/operator?kind=report&id=…` | GET | Report and audit detail |
| `/operator?kind=support&id=…` | GET | Support request and audit detail |
| `/operator/review` | POST | CSRF-protected, idempotent review decision |
| `/operator/categories` | POST | Category save/delete or room assignment |
| `/operator/logout` | POST | CSRF-protected session invalidation |

Queue pagination uses `report_before` or `support_before`, each an exclusive positive ID cursor. Review fields are `csrf`, `nonce`, `kind`, `id`, `status`, `expected` and `note`. Category fields are `csrf`, `op` (`save`, `delete`, `assign`), `id` and either `name` or `room`. Forms use normal browser URL encoding, not the original applet's limited free-text serializer.

## Validation

Server tests cover disabled/default access, invalid allowlists, ordinary-user denial, cookie-bound browser login, logout, CSRF rejection, escaped evidence, idempotent review retries, conflicting stale decisions, reopening, report/support restart persistence, category assignment and rejection of deleting an occupied category. The older report CLI remains compatible.
