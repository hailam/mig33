# Friend search and offline chat entry

The original v4.62 **Main Menu → Search New Friends** now searches the configured local accounts. Search results offer **View Profile**, **Chat** and **Add Friend**. A viewed user's profile also offers Chat and Add Friend, including when that user is offline. Self profiles omit these actions.

The recovered client establishes the route and native link commands. The downloaded historical HTML is absent; these forms, result layout and matching rules are reconstructed. The directory currently contains only the five fixture accounts. Registration, reciprocal friend approval, profile privacy and persistent profile editing are separate work.

## Page contracts

All requests require an authenticated gateway session: HTTP supplies the `sid` header; the original client uses922/923. These requests do not change presence, add a contact or join a conversation on the server merely by rendering the page.

| URL | Methods | Contract |
| --- | --- | --- |
| `/sites/index.php?c=profile&a=search&v=midlet` | GET, POST | GET opens the form; GET with one `query` parameter or POST with exactly one `query` form field returns results. |
| `/sites/index.php?c=profile&v=midlet&a=home&username=bob` | GET | View Bob's profile and native action links. Missing/empty username selects the viewer; unknown user returns404. |

Search trims outer whitespace and matches a case-insensitive literal substring of username or display name, excludes the viewer and uses the configured account ordering. Empty search lists all other configured accounts. Input is at most128 UTF-8 bytes before trimming, with NUL and invalid UTF-8 rejected. There are no wildcard operators, demographic filters, pagination or online-only restriction in this local directory.

Ordinary HTTP POST requires `application/x-www-form-urlencoded`, with a maximum8,192-byte body. Duplicate/unknown form fields and duplicate search parameters return400; unsupported methods return405; missing authentication returns401. Duplicate routing selectors return404. An unmatched valid search returns200 with an empty-result notice.

The native form contains one free-text field, `query`, as its final named control. Its serializer only escapes spaces as `%20`, and omits an empty text control entirely. The922 adapter preserves the raw final tail, decodes only `%20`, and treats an absent empty body as an empty query. Literal typed `%20` remains ambiguous in this old serializer. HTTP forms use normal URL decoding. Visible query text is escaped and the editable field is reset to empty to avoid the applet's attribute-entity handling defect.

## Native action links

| Link | Original behavior |
| --- | --- |
| `mig33:privateChat(bob)` | Create or focus a native private-chat tab for Bob. This does not consult roster presence and sends no packet until the user sends a message. A send uses the existing500 direct-message contract. |
| `mig33:inviteFriend(bob)` | Open the original Add Friend form, prefilled with Bob's username and the native contact-group selector. The user must confirm; the existing405 contact operation performs the addition. |

The applet strips command syntax with substring operations and does not URL-decode the argument. Generated username actions are therefore restricted to ASCII letters, digits, period, underscore and hyphen; unsafe imported values do not become executable links. Profile URLs URL-escape the username and preserve literal query separators for the native HTML parser.

The stock offline-contact primary action still selects Send Email, whose service is unavailable. Use **View Profile → Chat**, or **Search New Friends → Chat**, to begin an offline private conversation. This uses recovered client functionality without changing the user's reported presence or repurposing the email route. Existing private tabs also continue to send while their peer is offline. Offline private text is persisted and delivered after the recipient's next contact synchronization; this is transport delivery, not a read receipt.

Evidence: [applet inference audit](../research/applets/shared/backend-inferences.md), [friend-page handler](../internal/adapters/legacyweb/friend_pages.go), [renderer](../internal/pages/friends.go), [page and offline-delivery tests](../internal/acceptance/friend_pages_test.go), and [native acceptance](../research/acceptance/v4/FRIENDS_MODERATION.md).
