# Persistent accounts, registration and identity pages

The local server now stores accounts, profile fields, mobile activation and account preferences in SQLite. Schema migration9 adds these records; subsequent migrations add the other local services. The five original development accounts are seeded once. Restarting does not restore an old password, overwrite a profile, or recreate deleted contacts. Newly registered accounts start with an empty Friends address book and appear in the people directory.

This is a local reconstruction. The fixed development verification/security code is **123456**. No SMS provider is called. The original client permits “Activate later”; an unactivated native account can therefore log in and use local services, while203 tag2 continues to report its unactivated state until204 succeeds. No unsupported historical access restriction has been invented.

## Original binary and HTTP contracts

| Request | Local behavior |
|---|---|
|101|Creates a15-minute registration challenge;102 carries an empty mobile default in1, a48-hex token in2 and a PNG showing123456 in3. The duplicate width/height tag in the original request remains untouched.|
|103|Reads username1, password2, mobile3, challenge4, entered security code5. Validates and consumes the challenge, stores an unactivated account and returns correlated ACK1. The applet saves these credentials and returns to Login. Client build fields9/10 remain accepted as metadata.|
|200→201→202→203|Uses the persisted account.203 tag1 is the mobile number; tag2 is its activation flag; tag28 is the persistent status. New accounts use the same TCP/HTTP sessions and chat services as seeded accounts.|
|204|An authenticated account supplies code in1.123456 activates it persistently. Five failed attempts require a new login.|
|205|An authenticated account supplies its replacement mobile in1. Saves it as unactivated and returns to the original verification screen; the development code remains123456.|
|7→6|Returns local registration help, including the static code and the absence of SMS delivery.|
|904|Returns explicit local recovery guidance via error0. The original ACK handler ignores server text and always displays a bundled SMS-sent message, so ACK would misrepresent an unconfigured provider. Password reset is functional on the public recovery page below.|
|`/sites/index.php?c=registration_api&v=midlet&a=check_username&username=…`|Public GET. Available usernames return literal`[]`. Taken names return a bracket/comma list such as`[alice1,alice2,alice3]`, without JSON quotes: the original1005 handler does not strip quotation marks. The loopback test bridge maps the applet's hardcoded official URL to this local route.|

Username rules—3–32 ASCII letters/digits/`_.-`—and the mobile rule—6–20 digits with an optional leading`+`—are reconstruction policy, not recovered historical server validation. Passwords accept4–320 Java UTF-16 units. Mobile numbers are normalized with a leading`+` and are unique when present. Seeded accounts have no mobile until one is set in Settings.

## Implemented pages

All use the original`/sites/index.php` shape with`v=midlet`. HTTP uses the gateway`sid` header; native authenticated pages use922/923. GET displays forms, POST performs mutations. Forms have absolute actions and per-session CSRF values; public forms have single-use challenge tokens.

| Controller/action | Behavior |
|---|---|
|`registration/home`|Public local registration, terms checkbox and development verification. A successful web signup is already activated because its OTP step is included. This is a reconstruction extension; the applet uses101/103.|
|`account/recover`|Public staged reset: registered mobile, then OTP and new password. Successful reset consumes its token and signs out all existing sessions. This is a reconstruction extension for the absent SMS provider.|
|`profile/home`|Own or selected profile, persistent display/status/about/country, online presence, native Chat/Add Friend links and photo/blog links. Enforces target profile privacy.|
|`profile/edit`|Separate forms for display name, status, about and country. Each form has one unrestricted final text field, preserving original form serialization. Status changes update contact-status notifications.|
|`settings/change_password`|Verifies current password, issues a session-bound15-minute second-step token, saves the new password and signs out other sessions. Saved device login details must be updated by the user.|
|`settings/change_mobile`|Saves a unique mobile after development OTP verification; marks it activated.|
|`settings/chat_privacy`|Everyone, the recipient's own contacts, or nobody. Enforced on new direct messages and private-group invitations. Existing room participation is independent.|
|`settings/profile_privacy_settings`|Everyone, the target's own contacts, or self only. Enforced on profile views and shared profile-content surfaces.|
|`settings/view_update_settings`|Persistent activity-sharing preference, consumed by the local social feed.|
|`settings/call_by_name`|Persistent call-by-username preference, consumed by the local calling service.|
|`settings/change_tab_order`|Two original-compatible layouts. After saving, an Apply link invokes`mig33:reorderHomeTab(1)` for Friends/Chat Rooms/migWorld, or`(0)` for migWorld/Friends/Chat Rooms. The original method stores order in device RMS.|

Profile layout, form wording and account-policy choices are reconstructed service behavior. They are not claimed to be recovered original HTML. Original applet frame/menu styling and renderer are unchanged. See the separate formatting/design evidence documents for visual provenance.

## Password and challenge persistence

No plaintext password is stored. A salted PBKDF2-HMAC-SHA256 verifier with600,000 iterations verifies web password changes. The implementation uses the Go project's[`golang.org/x/crypto/pbkdf2`](https://pkg.go.dev/golang.org/x/crypto/pbkdf2) package, at a pinned dependency version. The current application requires Go 1.27.3 or later, as recorded in `go.mod`.

The original202 challenge also requires a legacy verifier: Java string hash of the lowercased password plus its UTF-16 unit count. Concatenation allows computing`hash(challenge+password)` from these values, including overflow, without recovering the password. This remains a weak, password-equivalent compatibility mechanism; PBKDF2 does not make the historical wire protocol production authentication. Password comparison remains case insensitive to agree with the original applet. Non-English handset-specific lowercase rules are not claimed to have been exhaustively reproduced.

Registration and recovery tokens expire after15 minutes, allow at most five wrong OTP attempts and are consumed atomically with account creation/reset. Password-change tokens are bound to the current session's CSRF secret. Reset also rechecks that the registered mobile has not changed. The constant development OTP is intentionally unsuitable as a production verification boundary.

## Templates and native form constraints

Identity presentation lives in`internal/pages/identity_views.templ` with typed profile/editor models and shared components from`components.templ`. Handlers perform authorization and state changes, then pass data to these templates; they do not assemble HTML strings. Shared rendering preserves native URL-attribute ampersands without unescaping user text.

## Validation

The old serializer escapes spaces only and omits empty controls. Password fields must be the final unrestricted field;`&`, `=`, `+` and literal percent sequences are preserved as its raw tail. A literal typed`%20` remains ambiguous with a space in the original serializer. Normal HTTP browser forms use standard form decoding. The settings editor avoids placing user text into HTML attributes because the original parser does not decode attribute entities.

Go tests cover native registration/activation and PNG decoding, restart persistence, Unicode Java hashing, public signup/reset, expired/used/purpose-bound challenges, session invalidation, native punctuation-heavy password forms, CSRF, profile privacy and direct-message privacy. Native acceptance evidence is recorded separately under`tools/applets/v4/integration`.

## Profile customization

Profile → Edit Profile → Customize Profile opens the retained edit route with `field=customization`. Saved HTML-subset authoring supports coloured text/backgrounds, font styles, owned scrapbook images and native chat/room links. It persists separately from About me; Reset restores the ordinary profile. Visibility and photo ownership remain enforced. Stock4.62 displays solid backgrounds and inline photos; photo backgrounds require a web browser. The form and input restrictions are local reconstruction choices informed by original September2010 guides and screenshots. See the [formatting contract](text-and-page-formatting.md#implemented-profile-customization).
