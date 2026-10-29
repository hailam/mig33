# Text colours and embedded-page format

Research updated: **2026-10-28**. Target: recovered **mig33 Java ME v4.62 build .000**. This document specifies what this client can render and submit. Original profile-customization guides and screenshots are now recovered; exact historical server validation, colour entitlement rules and the palette remain unknown.

The primary evidence is the original bytecode in [s.txt](../research/applets/v4/artifacts/bytecode/s.txt), [y.txt](../research/applets/v4/artifacts/bytecode/y.txt), [bc.txt](../research/applets/v4/artifacts/bytecode/bc.txt) and [aj.txt](../research/applets/v4/artifacts/bytecode/aj.txt). The corresponding `normal-flow` Java is a reading aid: identifiers collide, and some decompiled types/control flow are invalid. Important quirks below were cross-checked against the bytecode. The original JAR SHA256 is `dbf12ffbd44bc6e48d09cf35863736731c87c078877f76ed4b780a4dc5eff92c`.

## Three separate colour mechanisms

| Mechanism | Contract | Confidence |
| --- | --- | --- |
| Ordinary chat body colour | Outgoing/incoming opcode500 optional **tag13 INT**, a packed RGB integer; absent or−1 selects the theme default | Original client |
| Sender-name colour | Incoming500 optional **tag12 INT**; absent or−1 selects the appropriate theme/category colour | Original client |
| Embedded page text/background | A small HTML-like renderer with selected tags and inline colour properties, described below | Original client |
| Emotes and `/me` | The server returns500 **content tag6 SHORT6**, with body colour absent/−1. The client removes the usual sender/colon and avatar prefix and selects action colour | Original client; command catalogue is server policy |

The recovered client's chat-body path does not parse HTML or BBCode. Sending `<font color="red">hello</font>`, Markdown, BBCode or CSS does not invoke its page renderer. This establishes the client behavior only: an unavailable historical backend could have transformed typed input before delivery. Emoticon shortcuts are another mechanism: the client replaces recognised tokens using its image/alias catalogue; this is not arbitrary rich-text markup.

### Sender/message spacing

Ordinary native chat uses **`sender: message`**, with one ASCII space after the colon. The original `s.class` constant23 is exactly bytes `3a 20`; both sender-prefix branches append it (`s.java`, lines686–688). The transcript painter draws that complete sender segment and advances by the same font's `stringWidth`, including its trailing space (`s.java`, lines1109–1111). The backend sends sender and body in separate packet500 fields; it does not need to pad the body, which the client trims. [Captured original model](../research/acceptance/v4/runs/out/bob/chat-model.txt) and [native screenshot](../research/acceptance/v4/runs/out/bob/chat-from-alice.png) preserve the separator. Its pixel width depends on the emulator/handset font. Private copies to another session of the author use the documented explicit `sender: message` action fallback, also with a space.

### Native Change Text Color workflow

1. Server sends **924**, **tag2 TEXT**, containing semicolon-separated **decimal** integers, for example `16711680;255;32768`. It must not contain `#ff0000`, `0x…` or empty tokens.
2. Client clears `aj.aM`, prepends integer **0** (black), parses the supplied values with `Integer.valueOf`, and rebuilds its grid. More than one total entry enables the composer command **Change Text Color**. This packet does **not** automatically open the picker.
3. In a conversation, open **Write → Menu → Change Text Color**. Screen59 shows the grid; Select copies the selected integer into the conversation's `e` field and returns to the composer. Close returns without selecting.
4. Subsequent messages in that conversation send selected colour13 when it is not−1. Ordinary local echo uses the same selection. Colour is per conversation; it is not an account role or a global server setting.

Login resets the palette vector. A compatible server must supply924 after login if it offers this feature; merely accepting message tag13 leaves the original chooser unavailable. Opcode925 is language selection and has no colour-picker role. The original palette/order, which users were entitled to which colours, and whether entitlement was paid have not been recovered. A replacement palette is a reconstruction choice.

Evidence: [bc.java](../research/applets/v4/artifacts/normal-flow/bc.java), lines3114–3128; [aj.java](../research/applets/v4/artifacts/normal-flow/aj.java), lines3188–3194,5141–5145,6071–6080,6285–6290,8334; [924 bytecode extract](../research/applets/v4/docs/data/protocol-opcode-924.txt). The wire field is signed32-bit; restricting the replacement server's public selection to `0x000000`–`0xffffff` and treating−1 as default is a local validation policy, not proof of historical server validation.

### Names, roles and action colour

Incoming tag12 overrides the name's RGB independently of body13. Room admins are separately identified by participant-list708 tag3 and the room moderation UI. A red or gold message/name is not itself proof of moderator privileges. Client-supplied colours must never determine backend authorization or change the authenticated sender.

The room theme719 and global theme1000 supply fallbacks. The original `y.a(category,theme)` / `y.b(category,theme)` map as follows; category numbers are internal presentation categories, not a recovered role registry:

| Category | Name default | Body default | Global theme tags |
| --- | --- | --- | --- |
| 0 | `#086834` | black |142 /143 |
| 1 | `#406ac0` | black |144 /145 |
| 2 | red | red |149 /149 |
| 3 | `#ff7300` | black |150 /151 |
| 4 | `#fcc504` | black |146 /147 |
| 5 | `#993300` | `#993300` |148 /148 |
| 6 | black | black |112 /112 |
| 7 | `#9f9f9f` | `#9f9f9f` |138 /138 |

An explicit incoming body colour takes precedence over the content6 action special case. Therefore reconstructed action messages omit13: adding a colour to a content6 packet can restore the normal sender prefix and change its presentation. Default brown comes from theme148, and a different theme can change it. See [slash-commands.md](slash-commands.md) for delivery, private self-echo routing and command evidence, and [protocol.md](../research/applets/v4/docs/protocol.md#dynamic-menu911-and-room-theme719) for complete719 fields.

The chat renderer trims body text and replaces its internal delimiters **`¤ ± Ω µ ♣ ▲ ∫`** with spaces before building styled runs. Those delimiters carry internal colour/avatar/wrapping information after the renderer inserts them. They are not a supported user formatting syntax in this build. Evidence: `s.java` lines645–650,673–705; `y.java` lines1245–1305; `bj.java` lines1801–1904.

### Historical reports and their limits

A first-person **19 March2010** account describes brown command output in both private conversations and rooms, including `/me`, `/brb` and `/roll`. This agrees with the recovered default action presentation. [Contemporary account](https://khusnullutfi.wordpress.com/2010/03/19/command-text-code-buat-chatting-di-mig33/).

A **February2011** community post describes pasted-character colour tricks as client bugs and says its actual code was removed. It does not identify a reproducible v4.62 contract. The recovered renderer's delimiter replacement is direct evidence against treating such tricks as supported markup. [Contemporary colour-code report](https://pakmig33.wordpress.com/2011/02/).

An **August2010** tips page specifically places HTML bold/italic/underline/font-colour chat at **`wap.mig33.com/member2/t.php`**. That is a WAP site claim, not evidence that Java chat500 parses HTML. Its separate nickname tricks have no trustworthy version attribution. Preserve genuine message metadata and the native chooser; do not infer arbitrary HTML chat or nickname spoofing from these reports. [Contemporary WAP account](https://amcomig33.blogspot.com/2010/08/tips-1.html).

### Typed historical formatting: follow-up, 2026-10-28

The requested historical colour-code tricks are a separate recovery target from the working native picker. Contemporary reports support their existence, but do not establish one universal syntax or compatibility with the recovered4.62 build.

| Surface | Surviving evidence | Recovery boundary |
| --- | --- | --- |
| Early Java chat colour/name tricks | A first-person13 April2007 forum post describes special-symbol colour codes and altered displayed sender names. A25 February2011 post describes pasted characters producing red, green and blue text and calls the behavior a client bug. | No complete reproducible code sequence or affected build is established. The2011 post's code itself is replaced with an editorial marker. |
| Profile text/background customization | Original18,24 and30 September2010 guides and seven screenshots show background colours/images, text styles and native links. Their displayed square brackets are explicitly placeholders for angle brackets. | Original first-person text is recovered through the Blogger feed/mobile views. Some images show a modified4.20 client; exact4.62 attribution and authenticated response bodies remain unavailable. |
| WAP chat formatting | The2010 tips page above places HTML formatting at the separate WAP chat endpoint. | This does not establish how formatting reached Java-chat recipients or whether the same syntax worked in the Java composer. |
| Standard BBCode | No chat parser for `[color=red]text[/color]` was found in either recovered client. | Historical server-side conversion remains possible but unverified. No replacement BBCode feature has been implemented from this evidence. |

Sources: [first-person2007 report, post14](https://www.howardforums.com/threads/mig33-new-version-released-instant-messenger-for-j2me-mobiles.1138507/), [February2011 colour-code report](https://pakmig33.wordpress.com/2011/02/), and the original [18 September profile guide](https://mig-info.blogspot.com/2010/09/trik-kreasikan-background-profile.html?m=1), [24 September update](https://mig-info.blogspot.com/2010/09/update-kreasikan-background-profile.html?m=1), and [30 September WAP workaround](https://mig-info.blogspot.com/2010/09/kreasikan-background-profile-via-wap.html?m=1). The [source catalog](../research/applets/v4/docs/data/historical-formatting-sources.json) records publication/update dates, original feed entry hashes, retrieval paths and seven unchanged screenshot hashes. These are historical community reports, not official protocol specifications.

Version checks also matter. Originalv3.08 bytecode already replaces `¤` and `±` in **message bodies** with spaces ([v.txt](../research/applets/v3/artifacts/bytecode/v.txt), lines864–874; [v.java](../research/applets/v3/artifacts/decompiled/v.java), lines269–270). Originalv4.62 additionally strips `Ω µ ♣ ▲ ∫` before constructing its internal styled transcript. The internal renderer still uses generated colour delimiters, which does not make them supported typed input. This body sanitization does not cover every name argument and cannot disprove all historical nickname tricks. Actual name acceptance, affected versions and server transformations remain unknown.

A separate [January2011 author guide](https://mig-info.blogspot.com/2011/01/trik-emoticon-bergambar-pada-status.html) preserves15 private-use Unicode characters, including `U+EEBC` and `U+EEC9`, reported to appear as emoticon glyphs in status/chat on some Java handsets. The author explicitly describes handset-dependent support. The [product-source catalog](../research/applets/v4/docs/data/historical-product-catalog.json) records all15 code points. They remain ordinary Unicode text in this backend; a universal image mapping or colour syntax would require the corresponding handset font and is not inferred from the guide.

The reconstructed profile editor and WAP chat now reproduce the supported visual effects described below. The exact early special-character chat-code sequence remains unrecovered; the surviving2011 comment string has no matching colour grammar or build attribution. It is not promoted to a working feature. Standard square-bracket BBCode stays literal.

### Implemented profile customization

Open **Profile → Edit Profile → Customize Profile**. The retained route is `/sites/index.php?c=profile&a=edit&v=midlet&field=customization`. GET shows syntax help, current source and owned scrapbook photo references. POST uses `csrf`, `op=save` and one final `value` field. `op=reset` clears the customization. Source persists independently of the plain About me field.

Example accepted input:

```html
<body bgcolor="#fff0cc">
<p style="color:red"><b>Hello!</b></p>
<p><i>Welcome to my profile.</i></p>
<a href="mig33:joinChatroom(Lobby)">Visit Lobby</a>
</body>
```

The bounded authoring subset supports `body` background colour/image, `p`, `span`, `div`, `b`/`strong`, `i`, `u`, `em`, `big`, `small`, `center`, `br`, `a`, `img`, and `font color` normalized to a span. `em` renders as bold, matching the original client. Named or six-digit RGB colours become explicit hex values. Source must use balanced tags, at most4096 UTF-8 bytes,128 nodes and eight nesting levels. These validation rules and the editor form are local choices; the recovered guides used an obsolete profile-picture URL workaround.

Images use `photo:ID`, for example `<img src="photo:12" width="48" height="48">` or `<body background="photo:12">…</body>`. Saving checks ownership; reading checks current photo/profile visibility. Deleted or newly inaccessible images disappear without breaking the profile. Asset URLs resolve locally and previews are bounded for the original image renderer. Links support explicit `mig33:privateChat(username)` and `mig33:joinChatroom(room)` actions. Ordinary profile identity and account controls render separately from user styling.

**Background-image boundary:** browsers display the owned image as a profile background. Stock4.62 parses the attribute but does not request or paint the background image. Its renderer passes a null image to the background helper (`s.txt`, offsets39–48 in the paint method), leaves background requests in an unused vector, and the helper in `y.java` ignores its image argument. Solid background colours and inline photos work in the original client. Historical screenshots with background artwork include modified or unspecified clients and do not establish stock4.62 support.

`internal/formatting` parses source into typed nodes. Reusable `internal/pages/formatting.templ` components escape text and render only permitted elements; raw user HTML is never passed through. Canonical style attributes avoid templ's automatic trailing semicolon, which breaks the original paragraph-colour parser. Profile privacy, CSRF checks, owned-photo restrictions, persistence and reset are covered by tests.

### Implemented formatted WAP chat

The historical path **`/member2/t.php`** now provides an authenticated local chat page, also reachable as **migWorld → Formatted Chat**. A browser can sign in with an existing account at that path; native922 uses its existing gateway session. Browser sessions use a separate HttpOnly cookie and CSRF-protected login/logout. Page layouts, forms, query fields and native fallback are reconstruction choices: no original WAP request/response capture was recovered.

| Operation | Local contract |
| --- | --- |
| Open directory | GET `/member2/t.php`; visible profiles, current private groups and public rooms |
| Open conversation | GET with `kind=private`, `group` or `room` and `target`; optional ascending `after_id` pagination,50 records/page |
| Send | POST to the conversation URL with `csrf`,48-hex `nonce`, `op=send`, `format=plain` or `html`, final `body` field |
| Room membership | POST `csrf` and `op=join` or `leave`; applies existing capacity, ban and Group-room rules |
| Browser sign out | POST `csrf` and `op=logout`; removes that browser session and its room memberships |

Formatted input accepts the text/style subset above, excluding images and page backgrounds. For example `<font color="red"><b>Hello</b></font>` produces red bold text on the WAP page. Rich source, plain fallback, single RGB when uniform, audience and send nonce commit together. Same-form retries return the existing result; a nonce reused with changed content fails. JSON `/api/chat/messages` also accepts mutually exclusive `markup` instead of `body`, `color` or `photo_id`, using its existing persistent idempotency-key contract. History/sync responses include optional `markup` alongside plain `body`.

**Native delivery limit:** original500 supports a single body RGB, not arbitrary rich-text runs. Native recipients receive plain text and a colour only when all non-whitespace text has the same explicit colour. Mixed colours and font styles remain in WAP/history metadata. Ordinary native composer text remains literal, matching its local echo. Formatted slash text is prose; it cannot become a moderation command. Plain WAP input supports the same durable core actions as the JSON API; Group/bot administration stays in its existing controls.

Browser room sessions use authorized stored history instead of native push queues. They count toward room capacity and obey kick/mute/ban/privacy rules. WAP reads do not invent native read/delivery receipts. Original native form parsing preserves the final free-text tail; the old literal`%20` ambiguity still applies.

[Original-client acceptance](../research/acceptance/v4/formatting-result.md) passed with Alice over TCP and Bob over HTTP:38 screenshots,160 wire frames and both logout acknowledgments. It checks native profile Save/Reset, punctuation, colours, inline-photo pixels, native chat links, rich WAP history and direct/private-group/room RGB fallback. Backend race tests additionally cover permissions, malformed markup, durable retries, restart persistence and browser room sessions.

## What the embedded pages return

The original browser is a Java Canvas renderer (`bl`, `s`, `bj`, `y`), not a WebView. A page returns **UTF-8 HTML-like text bytes**, either over the client's direct HTTP path or in **923 tag2 BYTES** replying to922. For the ordinary page route,923 tag1 is HTTP status200 and tag3 is the cache duration value interpreted in seconds. Non200 produces the client's retry/error page. Cookies use923 tag11 string array. Images use the separate922 purpose2 path and raw image bytes; no JSON envelope, base64 wrapper or WML is required. See [protocol web transport](../research/applets/v4/docs/protocol.md) for exact922 fields and923 routing.

`bl.a(String,String,byte[],long)` first tokenizes UTF-8 via `y.a(InputStreamReader)`; a platform-default decoding fallback runs only if no tokens were produced. Lines are trimmed, empty lines skipped, and surviving line breaks become spaces. The tokenizer splits at literal `<` and `>` and is not quote-aware. Emit lowercase tags, double-quoted attributes, explicit closing tags, and `<br>` rather than relying on HTML5 parsing rules. Attribute names usually compare case-insensitively, but tag names, input type values and CSS property parsing do not consistently do so.

### Supported page elements

| Element | Recognised properties and actual behaviour |
| --- | --- |
| `html`, `head`, `title`, `body` | Basic document/title handling. `body` renders `bgcolor`; `background` and bare `stretched` are parsed but stock4.62 does not request or paint a background image. Default page background white; initial font MIDP proportional/plain/small and text black. Keep the head minimal. |
| `p` | Starts a paragraph/line boundary; optional exact-prefix `style="color:…"`. It treats the remainder as a single colour, so do not append unrelated properties. |
| `br` | Line break; exact `<br>` is the recognized form. |
| `b`, `strong`, `em` | Set MIDP **bold**. `em` is bold, not italic. |
| `i`, `u` | Set italic or underline. These set a font style rather than combining flags, so nested bold+italic is not normal CSS composition. Closing tags pop the font stack. |
| `big`, `small` | MIDP LARGE/SMALL font size, retaining current style; actual pixels depend on the emulator/handset. |
| `center` | Centers collected display elements; closing ends that mode. |
| `span` | `style` recognises lowercase `color:` and `background-color:`. Closing restores paragraph/default text colour; this is not a complete nested CSS cascade. |
| `div` | Selected `color:` and `background-color:` properties. `display:` is parsed, but the original `block` comparison has no distinct layout branch; do not depend on it as a CSS block layout engine. Its style loop includes a200ms sleep per recognised property. |
| `a` | `href`; links are underlined and blue (`#0000ff`) by default, or theme61 when using the special theme mode. Relative links use `y.d(base,href)`; `mig33:` is dispatched to native actions. `.wav`, `.mid`, `.mp3` suffixes select a media handler. |
| `img` | `src`, integer `width`, `height`, `hspace`, `vspace`; a `style="float:…right…"` special case. Specify dimensions. No general scaling/CSS sizing/`alt` layout is implemented here. `local:` sources can load bundled assets, including special `migbotLoading` and `migbotHappy` aliases. |
| `table`, `tr`, `td` | Small fixed-column layout. Nonstandard **`tr cols="N"`** divides available width into N columns. This is not a table algorithm with colspan/rowspan/automatic cell sizing. |
| `form`, `input`, `select`, `option`, `textarea` | Native controls; exact submission rules below. |
| `menu title="…"`, `menuitem label="…" href="…"` | Custom page softkey/menu entries. These are not an HTML navigation menu widget. |
| `prefetch url="…"` | Queues a page in the browser's prefetch mechanism. |
| `style midletTheme` | Custom marker selects theme-driven page text/background/link rendering. It is not a stylesheet parser. |

There is no implementation of JavaScript, DOM scripting, external stylesheets, CSS selectors, flex/grid, `<font>` colour tags, `<blink>`, headings, `<hr>`, list bullets, `<button>`, radio controls, file upload inputs or modern HTML validation in this renderer. Unknown tags can be ignored while their text remains visible, and some tag checks use broad prefixes, so unsupported markup can have unexpected effects rather than graceful fallback. Emit the documented subset. Evidence: [s.java](../research/applets/v4/artifacts/normal-flow/s.java), renderer lines1825–3130; [bl.java](../research/applets/v4/artifacts/normal-flow/bl.java), lines262–327; [y.java](../research/applets/v4/artifacts/normal-flow/y.java), lines1729–1820 and3241–3308.

### Colours and entities inside pages

Colour strings accept `#RRGGBB`, `0xRRGGBB`, or hexadecimal digits without a prefix. Named colours are black, blue, cyan, gray/grey, green, magenta, red, white, yellow. **Named `blue` maps to4095 (`#000fff`)**, an original-client quirk verified in `y.txt` offsets176–179; use explicit hex to obtain standard blue. Named green is bright `#00ff00`. CSS `rgb()`, `rgba()`, alpha and CSS shorthand expansion are not implemented: `#abc` is integer `0x000abc`, not expanded `#aabbcc`.

Ordinary displayed text decodes decimal `&#NNN;` plus `&copy;`→`(c)`, `&amp;`, `&lt;`, `&gt;`, `&nbsp;`→space. Hex numeric entities, `&quot;`, `&apos;` and a general HTML entity table are absent. Numeric decoding casts to one Java `char`; it is not a Unicode scalar-value decoder. Invalid numeric references can throw. Text in title/textarea/select-option handling bypasses the ordinary text entity path, so do not assume identical decoding there.

**Attributes are not entity-decoded.** Write URL query separators as literal `&`, not `&amp;`, for this client. Percent-encode each dynamic URL value before constructing the URL. Attribute values cannot safely contain literal quote/angle brackets; use suitable restricted values, a separate edit flow, or empty prefilled controls rather than treating generic HTML escaping as native round-trip encoding. Escape user content for ordinary display as text, not markup. Evidence: `s.java` lines2023–2059; `y.java` colour parser1651–1705 and attribute tokenizer3241–3308.

## Forms and serialization

| Control | Supported behaviour |
| --- | --- |
| `form` | `method` defaults `get`; `action` resolved using a different, simpler helper than ordinary links. Use **POST and an absolute action URL**. `</form>` finalizes the form; multiple sequential forms are supported. |
| Text/password `input` | `type`, `name`, `value`, `alt`, `size`. Omitted type means text. `alt` is the edit-dialog title. Both use a fixed **320-character** native editor; password uses MIDP password constraint. `maxlength`, `required`, `pattern`, `disabled` do not implement validation. |
| Input width | `size="5"` half width, `size="7"` three quarters, any other numeric size full width; default5. Use10 for full width consistently with textarea. |
| Hidden input | Nonempty `name=value` data; serialized before visible controls regardless of its position. |
| Checkbox | Bare `checked` sets initial state. Checked submits `name=on`; declared `value` is ignored. Unchecked submits nothing. |
| Submit/reset input | Both enter the same submission path; reset does not implement reset semantics. All named submit/reset controls with nonempty value are serialized, not just the clicked one. Prefer one unnamed submit per form. |
| Select/option | `select name`; `size` parsed but not used as a modern multi-select size. `option value`, bare `selected`; selected option supplies the value. Use explicit closing option/select tags. |
| Textarea | `name`, `rows`, `cols`, `alt`, custom `size`. `size` controls native editor maximum; `rows` visible lines; `cols` supports **5,7,10** only for half/three-quarter/full width. A conventional cols24 can produce a zero-width control. |

Submission is a compatibility quirk, not standard URL-encoded form data:

- Hidden values first, then controls in document order; empty text/password/textarea/option/hidden values are omitted.
- Concatenate raw `name=value` pairs with `&` and replace only literal spaces with **`%20`** over the whole result.
- Literal `+`, `%`, `&`, `=`, Unicode and line breaks are not otherwise encoded. Ordinary `url.ParseQuery` semantics therefore cannot losslessly parse every native submission.
- GET appends `?` even if the action already contains a query string. POST keeps the action and submits the serialized string separately.

For replacement pages, keep fixed hidden values restricted (for example safe hex identifiers), validate simple fields, and put at most **one unconstrained free-text field last**. The adapter can parse fixed prefixes and recover the final tail, decoding only `%20` for this native transport. A literally typed `%20` remains ambiguous with an actual space; do not claim exact round trips for that case. Use staged forms when more than one arbitrary value is necessary. Native account registration packets are a separate typed protocol and avoid this HTML serialization problem; their constraints are documented in [protocol.md](../research/applets/v4/docs/protocol.md).

An appropriate page shape is:

```html
<html><head><title>Edit profile</title></head><body>
<p><b>Edit profile</b></p>
<form method="post" action="http://127.0.0.1:8080/sites/index.php?c=profile&v=midlet&a=edit_profile">
<input type="hidden" name="field" value="about">
<textarea name="value" rows="3" cols="10" size="320" alt="About me"></textarea>
<input type="submit" value="Save">
</form>
</body></html>
```

This illustrates compatible markup, not a recovered historical page or an assertion about the current application's exact field names. Evidence: `s.java` form serialization1446–1542, controls1544–1584, form/input2315–2483, select/option2485–2529, textarea2931–2976.

## Native links and future Expo client

The `mig33:method(argument)` command table is in [protocol.md](../research/applets/v4/docs/protocol.md#web-to-native-mig33-commands). The client takes the argument literally; it does not URL-decode it. For example, `mig33:joinChatroom(Lobby Test)` uses a literal space, while the corresponding HTTP query value needs URL escaping. `mig33:privateChat(alice)`, `mig33:inviteFriend(alice)` and `mig33:auth()` hand control to native workflows. This is not JavaScript execution.

An Expo recreation can reuse the backend's account, message, room and moderation rules and preserve the compatibility routes. The original app's native screens and these small server pages are separate presentation surfaces. A future Expo renderer should use typed message metadata for colours/actions and an explicitly supported page model or subset renderer. A general WebView alone would behave differently around entities, forms, table layout, typography and unsupported tags. The proposed v3 skin/behaviour setting belongs above the shared backend; visual fidelity still needs the separate screen/asset/font evidence in [v4/docs/screens.md](../research/applets/v4/docs/screens.md) and [v4/docs/rendering.md](../research/applets/v4/docs/rendering.md).

## Remaining evidence gaps

The client proves parsing, available widgets, URLs and dispatch behaviour. It does not contain the original server-generated HTML pages, historical palette/entitlement policy, all command responses or every moderation policy. Reconstructed page text/layout and a modern Expo implementation must remain labelled as replacements until original response captures or screenshots corroborate them. No live historical service was contacted with credentials during this audit.
