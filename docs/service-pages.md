# Embedded pages and local services

The full **52-template catalog** in [routes.json](../internal/pages/routes.json) now has stateful handlers behind the original `/sites/index.php?c=…&a=…&v=midlet` URLs. This is local service coverage, not complete historical product parity. Native acceptance rendered all52 templates and exercised representative forms; see [the screenshots and results](../research/acceptance/v4/PAGES.md). The [original endpoint catalog](../research/applets/v4/docs/endpoints.md) retains the exact URL spellings, native commands and query parameters.

For this backend milestone, unrecovered details use explicit inferred behavior or placeholders. SMS, external email, calls and payments remain mock services; persistent `pending_provider` request records are the intended mock result. External IM stays disconnected pending a later decision. Expo and its v3 UI mode are deferred separately.

## Rendering architecture

Pages use **templ v0.3.1020**; the application requires Go 1.27.3 or later, as recorded in `go.mod`. [components.templ](../internal/pages/components.templ) supplies the document, paragraphs, headings, links, forms, hidden fields, text/password inputs, textareas, checkboxes and pictures. Feature views use typed models and components. Generated `_templ.go` files are build artifacts; edit `.templ` sources and regenerate instead.

Run `go generate ./internal/pages` to regenerate using the pinned compiler version. Ordinary builds need no globally installed templ executable. Business logic validates permissions and changes persistent state; components render the resulting data. There is no HTML string concatenation or `templ.Raw` page escape hatch.

[render.go](../internal/pages/render.go) is the single legacy serialization boundary. Templ escapes user text and attributes normally. The adapter restores only `&amp;` inside `href`, `action` and `src` attributes because the original client does not decode those entities, and emits explicit native-compatible void tags. It does not unescape text or quote delimiters. The URL validator allows the recovered `mig33:` and `local:` schemes while rejecting unsafe schemes, credentials and attribute delimiters.

Forms use absolute POST actions, fixed hidden values first, one unrestricted final text field, and staged forms where more than one field needs arbitrary punctuation. This preserves the applet's serializer, which encodes only spaces and omits empty text controls. HTTP browsers use ordinary form decoding;922 requests use route-specific native decoding. [The formatting specification](text-and-page-formatting.md) records all recovered parser quirks.

## Service coverage and limits

| Service | Persistent local behavior | Historical/provider boundary |
| --- | --- | --- |
| Registration/account recovery | Native101/103/204/205, public registration and reset pages, dynamic accounts, password change, mobile verification, profile fields, preferences | Development code **123456**; no SMS provider. Original case-insensitive Java-hash login remains a compatibility protocol. |
| Profiles/privacy | Profile editing/search, persistent formatting save/reset and owned scrapbook photos; profile visibility, contact restrictions and activity sharing enforced | Explicit customization editor is local; historical guides describe an obsolete profile-image URL workaround. |
| Formatted WAP chat | `/member2/t.php` supports private, group and room conversations, persisted rich source/plain fallback and idempotent sends | Historical path retained; layout, forms and native fallback are local reconstruction choices. |
| Community Groups | Public/private Groups, approval, member/moderator controls, forum topics/replies, owned-room linking and linked-Group slash controls | Separate from private group conversations; inferred control policies are documented in [room completion](room-completion.md). |
| Credits/store | Integer test-credit ledger, recharge, atomic transfer,37 gifts,39 packs, two theme variants, free original-theme reset, inventory and history | Historical names are attributed; IDs, prices and placeholder artwork are local. Payments/settlement remain mock. |
| Buzz/Lookout/check-in/blog | Durable posts and place labels; deletion; authorized activity views; original contact `cid` lookup; native921/422 summaries | Local feed rules; profile privacy and Hide Activity apply. No historical feed dataset or exact ranking recovered. |
| Mail | Local username inbox/outbox, durable delivery; stored external-email mock requests | External addresses stay `pending_provider`; no email sent externally. |
| SMS/calls | Validated, durable mock requests; call-by-name preference | No SMS sent or telephone call placed; this is the intended milestone scope. |
| External IM | Saved account labels marked disconnected | Whether to add an IM mock or omit it remains undecided; external passwords are not collected. |
| Games/leaderboard |25 catalog entries: two playable replacements with retry-stable results/local ranking and23 historical references | Historical references have details pages but no play action or charge; original full game services/rules remain unavailable. |
| Support | Durable local support requests | No external support queue or outbound notification. |
| Photos/avatar | PNG/JPEG validation/storage, scrapbook/wall, owner actions, selected display picture, shared-photo audience authorization | Local photo token representation; camera/device behavior and all historical media formats are not reproduced. |

See [identity](identity.md), [community and commerce](community-commerce.md) and [room discovery](room-discovery.md) for detailed policies and tests.

## Profile customization and formatted chat

**Profile → Edit Profile → Customize Profile** opens `/sites/index.php?c=profile&a=edit&v=midlet&field=customization`. Its CSRF-protected save/reset flow stores a bounded HTML subset independently of About me. Shared typed components escape text, validate colours/native links and resolve only owned `photo:ID` references, rechecking visibility when read. Stock Java 4.62 displays background colours and inline photos; it does not request or paint body background images. Web browsers can display those backgrounds. See the [syntax, image and form contract](text-and-page-formatting.md#implemented-profile-customization).

**migWorld → Formatted Chat** opens `/member2/t.php`; web browsers can sign in there with an existing account, while native requests reuse their gateway session. Private, group and room pages retain rich source and plain text with the normal audience rules. Original native messages support one body RGB: recipients get plain text and a colour only when all text has one explicit colour. Mixed colours/styles remain available in WAP and history metadata; ordinary native input and BBCode stay literal. See [chat persistence](chat.md#formatted-wap-chat) and the [WAP authentication/form contract](text-and-page-formatting.md#implemented-formatted-wap-chat).

## Media contract

Native504 accepts PNG/JPEG bytes in tag2 and returns an original-style ACK URL to the scrapbook. Uploads are limited to1MiB,2048×2048 pixels and100 stored photos per owner. `/api/photos` also accepts raw bytes with `sid`, `X-CSRF-Token` and a48-hex `Idempotency-Key`. Its response includes the local photo ID and URL. Image responses are authorized before writing without holding the shared server mutex during a slow client write.

Photo chat sends accept500 content2 (upload bytes) or5 (own scrapbook ID). Rejected uploads are removed. Successful messages store `photo_id`, return a local `[photo:ID]` trigger in body8/tag11 and preserve normal private/group/room audience rules.913 resolves an authorized photo trigger to914 **type1**, containing a static PNG thumbnail. This uses the recovered emoticon renderer; the trigger naming is local, not a recovered production photo ID format. The local downloadable catalog supports all four media types; recovered pack names use explicit placeholder assets where original downloads remain unavailable. See [native media contracts](native-services.md).

914 type2 triggers vibration, type3 carries framed animation, and type4 carries an image plus WAV data; type4 is not a generic photo format. The [protocol reference](../research/applets/v4/docs/protocol.md) documents these distinctions.

Avatar selection persists the chosen photo, sends603/421 updates and includes avatar keys in contact snapshots. Images use the advertised local asset base. A private photo shared in chat remains accessible to its saved audience; blocking prevents subsequent image retrieval. Previously downloaded device caches cannot be remotely erased.

## Room settings recovered from historical visuals

Archived Room Info shows language/description, keywords, moderators, banned users, kicking rules and ownership. These controls now live under the original setup route with the local `manage=1` selector. Owners can edit those fields, set capacity2–100, manage moderators and transfer ownership; joined moderators can ban/unban ordinary users. Bans persist and prevent rejoining; disabling user kicking cancels the current vote and prevents new ordinary votes. Moderator kicks continue to work.

Ownership changes keep the previous owner as a moderator; the new owner may revoke that role. Capacity cannot be reduced below the active population. Settings changes, ownership, roles and bans receive a persistent audit entry; native kicks, mute/unmute and vote outcomes are also recorded in the [moderation audit](room-completion.md). Historical limits/durations and exact form payloads remain unverified; the limits above are local policy.

New room names now follow the [historical Create Room illustration](../research/applets/v4/artifacts/historical-pages/chat5c.png)'s **15-character limit**. Existing definitions retain the earlier40-character compatibility range for lookup, joining, editing and native links; no room is renamed or removed. Creation remains staged for safe native text serialization. Images also corroborate rooms with50 and70 users. Search language/keyword controls use the explicit local filtering rules in [room completion](room-completion.md); exact historical matching policy remains unknown.

## Design evidence and later UI

[Historical galleries](../research/applets/v4/docs/historical-pages.html) now retain120 distinct images:20 official-host illustrations,7 profile captures,76 product/store captures and17 game captures. Of52 URL templates,13 have partial historical-family page visuals,17 have navigation-only evidence and22 have no matching visual. The captures span different or unspecified clients, including modified4.20, and do not establish exact stock4.62 output. No authenticated production HTML was recovered. [Coverage and provenance](../research/applets/v4/docs/historical-page-designs.md) distinguish screenshots, byte-exact bundled assets, emulator captures and reconstruction choices.

The later recreated UI can use Expo/React Native for Android, iOS and web, with shared service APIs and a separate Classicv3 presentation/behavior preference. Expo supports these targets in one project ([official guide](https://docs.expo.dev/tutorial/introduction/)). Exact legacy appearance will require deliberate custom layout/fonts, not platform-default controls. The new UI and v3 toggle are not built in this pass.
