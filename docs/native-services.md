# Native media, provider adapters and languages

This completes the local service contracts behind additional v4.62 packets. Exact historical provider behavior, artwork catalogs, prices and translations are not implied. The authoritative original field layouts remain in [the protocol reference](../research/applets/v4/docs/protocol.md).

SMS, external email, calls and payments intentionally remain mocked for this milestone. `pending_provider` is a durable mock-request state, not an external-delivery claim or a requirement to connect a provider now. Local username mail delivers to the local inbox. IM remains a disconnected saved-contact adapter; whether to add an IM mock or omit it is deferred. Public [`GET /api/catalog`](backend.md#public-catalog-metadata) exposes these modes without account data.

Historical theme, English-pack, emoticon and provider-icon downloads read local resources through the native codec's injected filesystem. They are not embedded in the backend executable or committed to Git. Follow [resource preparation](../tools/applets/SETUP.md#prepare-optional-backend-resources) to extract the verified original files into `data/applets/resources/v4`, selected by `resources.directory` or `MIG33_RESOURCES_DIRECTORY`. Core services start without those files; affected downloads report unavailable resources. The original client retains its bundled default theme and language.

| Request | Local response and state |
| --- | --- |
|602 avatar | Accepts an owned `photos/ID` key in tag1, or empty/default-avatar.png to clear it. Saves the choice and broadcasts603/423. The original builder writes the old ID over the new ID in tag1; the server applies only what arrives. The gallery remains the reliable native way to choose a new image. |
|800 callback | Validates origin/destination phone numbers, stores a pending_provider call and returns a correlated acknowledgment plus a notification. Never returns804 or starts a telephone call. |
|900 invite | Saves a pending_provider invitation and returns its explicit request receipt in ACK tag1. No SMS is sent. |
|500 channel2 | Saves a text SMS request as pending_provider. Returns an explicit0 response containing the saved request ID and status: the original ACK path hardcodes “SMS sent”, so ACK would be false. |
|500 channel9 | Delivers mail to an authorized local username's inbox; external email remains pending_provider. ACK tag1 displays the actual result. Personal blocks and recipient chat privacy apply to local mail. |
|206/302 local IM directory |206 acknowledges availability of the explicitly named local saved-contact directory; its receipt states that the external provider remains disconnected.302 acknowledges the request and restores that local directory adapter. No207 connected response, external connection or password is requested. |
|907 IM settings | Removes the selected service's saved local account labels. Cached unknown fields are discarded, including any obsolete credentials. |
|926 IM icons | Returns927 image fields from the recovered neutral default avatar. These are placeholder service icons, not recovered provider branding. |
|913 emoticon | Resolves local photo tokens and the local emoticon catalog to correctly framed914 data; the request's original trailing space is accepted. |
|925 language | Persists selection of the recovered en-US pack. Unknown/unavailable translations are rejected. |

Native call/invite/provider-message retries within one authenticated session are deduplicated using a hash of session ID and the complete packet, including its transaction. This is a local retry policy, not a claim about historical server semantics. Retrying after a new login creates a new request. Persistent receipts never mean that an external provider sent or charged anything.

The dynamic208 service mapping uses the recovered v3 service/channel pairs, advertised explicitly to v4: MSN2/4, AIM3/5, Yahoo4/6 and Google Talk6/8. Descriptors are visibly named “MSN saved contacts (local)” and equivalent names. Their state2 indicates the local directory adapter is available, while every external contact has offline presence99 and external messaging is rejected. This explicit local adapter avoids an original rendering bug:402 chooses dynamic service icons only at state2; other states make offline external contacts index nonexistent bundled sprite102. No external provider session is established or represented as established. A new native session receives these descriptors before contacts that reference them;302/907 restore directory availability after the original ACK mutates it. The client automatically sends206 for these configured adapters after login. Its ACK has no specialized provider-state transition, so acknowledging local-directory availability avoids false startup error dialogs without sending207 connected or attempting an external sign-in. Only recovered mode1/tag3=0 is accepted.

## Emoticons and gifts

The initial916 picker contains free `[smile]`, `[heart]` and `[gift]`. The **39 purchasable emoticon packs** comprise the local Effects pack and38 packs using recovered historical names. Effects costs2.00 test credits and adds `[buzz]` (type2 vibration image), `[sparkle]` (type3 animation) and `[chime]` (type4 image plus WAV). Each other pack currently unlocks one clearly labeled static `[pack:…]` placeholder token; it does not claim to contain the original full pack. Purchases refresh native pickers across connected sessions and ownership persists. Sending paid tokens requires ownership; recipients may download their assets without buying the sender's pack.

With the historical resources prepared, runtime token images are decoded from exact cells of the original `default_v2` field129 atlas, extracted unchanged to the local `data/applets/resources/v4/sprites.png`. The atlas is20 columns by4 rows,12px per cell. The local definitions use cell5,25,27 and49, numbered from0 in row-major order. The [historical product research](../research/applets/v4/docs/data/historical-product-catalog.json) recovers74 records representing73 unique names and supporting captures. It does not recover the complete production download binaries. Runtime IDs, token aliases, frame sequences, placeholder artwork placement and test prices are local choices. The short660Hz PCM chime is generated replacement audio. Frame type3 is big-endian frame count followed by length/image pairs; type4 is length/image then length/WAV. Types1/2 contain raw PNG data.

The runtime store includes **37 gifts**, each with an animated `[gift:ID]` receipt. Purchasing one for another local user writes its action message in the same transaction as the test-credit charge and inventory entry. Repeating a purchase nonce creates neither another charge nor another message. Recipient privacy/block rules apply before purchase. Gift tokens are reserved for completed purchases and cannot be forged by sending their text. Their small animations use recovered atlas artwork as explicit placeholders, not original production gift animations. The first three aliases remain `[gift:rose]`, `[gift:cake]` and `[gift:star]` for compatibility; additional IDs and tokens are listed in `/api/catalog`.

Two purchasable theme variants retain the complete original theme resource while changing selected colours. **Use original default theme** restores the byte-exact bundled default free, without an inventory entitlement. Theme selection/reset persists and delivers only to native sessions; browser sessions never accumulate unused native theme/avatar packets. See [the store contract](community-commerce.md#credit-ledger-and-store).

## Language HTTP resources

These public resources work before login and retain the original language-list path:

- `GET /sites/resources/language/v4.6/l` returns binary1002 with dense code/name/absolute-pack-URL triples.
- `GET /sites/resources/language/v4.6/en-US` returns the exact10411-byte bundled1004 English resource.

1001 and1003 are jobs inside the original client, not new gateway opcodes. The local integration bridge redirects only the exact original language-list URL to the test server. English is the sole available original pack and is listed as such in `/api/catalog`. Further translations require complete recovered or explicitly authored packs; this implementation does not label English copies as other languages.

## Validation

Go tests use authored synthetic resources to cover ownership, private mail, malformed inputs, retry deduplication, icon/image decoding, binary language equality, all914 content formats, paid-token enforcement, full-queue picker refresh and persistent atomic gift delivery. Original-resource compatibility is checked separately with the local publisher JAR. The [native media acceptance passed](../research/acceptance/v4/media-result.md) using the unchanged original JAR's serializers, network receiver, gallery renderer and media/language decoders over TCP and HTTP:266 protocol frames,6 screenshots and both logout acknowledgments. Fixture PNG/JPEG bytes replace camera hardware; physical camera capture, sound output and vibration are outside the headless emulator's acceptance scope.

Image-purpose922 responses use923 tag3=60000 milliseconds so successfully decoded photos survive the original browser's next paint. Ordinary dynamic page responses retain0; the original page branch interprets this field in seconds instead. Tests and native cache assertions protect this distinction.
