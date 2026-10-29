# Historical embedded-page design evidence

Audit date: **2026-10-28**. Target: **Java ME v4.62 build .000**. The [main image gallery](historical-pages.html) and [product gallery](historical-catalogs.html) reference **120 recovered historical image files**:20 archived official-host illustrations,7 community-author profile captures,76 product/store captures and17 game captures. All120 files have distinct paths and SHA256 hashes; provenance remains separated. [The 52-template coverage matrix](data/historical-pages/route-design-coverage.json) records every known embedded-page URL independently.

**Thirteen of the 52 templates have partial historical-family page visuals,17 have navigation evidence only, and22 have no matching visual in this recovery. No original authenticated page HTML has been recovered.** A screenshot of an opening menu does not recover the page it opens. None of these external images identifies the exact4.62 build, and several predate it; they establish the historical design family and visible fields, not exact build-specific pixel fidelity. Shared profile/store/game-family captures do not identify the precise request URLs used in those images.

## Provenance and what is genuinely recovered

1. **Original JAR resources:** two root 24×24 PNGs and nine theme PNGs are byte-exact 4.62 assets, not approximations. The theme's 80-cell 12×12 sprite sheet, 320×95 artwork, 80×45 logo and placeholder/dialog images are in [assets.html](assets.html), with [theme hashes](data/theme.json). They do not contain profile, store or account page screenshots.
2. **Original native renderer captures:** the existing [240×320 offline gallery](../artifacts/screenshots/240x320/index.html) runs original bytecode with bundled resources and labelled fixtures. Its [manifest](data/screenshots-240x320.json) states emulator/font conditions, synthetic data and absent handset softkeys. These are reproducible renderer evidence, not historical server screenshots.
3. **Archived official-host illustrations:**20 image files were recovered unchanged from Wayback captures of **`blog.mig33.com/uploads/…`**. The capture timestamps are mostly June 2014, while the original URL directories are July 2010 and November 2010. The date in the URL is not by itself proof of publication date. Eight filenames and their guide context are independently visible in a [20 February 2011 mirror](https://mig-world.blogspot.com/2011/02/chatting-and-making-friends_20.html); its live image proxies returned 404. Direct archived official-host images remained retrievable. For example, [the official-host room-creation illustration](https://web.archive.org/web/20140603223736id_/http://blog.mig33.com/uploads/2010/07/chat5c.png) is stored locally as [chat5c.png](../artifacts/historical-pages/chat5c.png).
4. **Community-author profile guides:** 7 original image files were recovered from the original `mig-info.blogspot.com` author's September 2010 guide via its public Blogger feed. They show customized profile pages, including coloured backgrounds/text and inserted native links. One identifies a modified `mig33v42-store` client. Their provenance and limits are separate from the 20 official-host images; see the section below.
5. **Community-author product and game captures:**76 unchanged images from `mig33-news.blogspot.com` and `mig-info.blogspot.com` show2009–2011 store/pack pages, purchase steps and gift commands. Another17 original game captures show migGames, migWorld, game-room activity and selected game screens. The [product catalog](data/historical-product-catalog.json) and [game evidence](data/games-history.json) retain dates, source URLs, hashes and limits. Dated local-currency prices are observations, not canonical billing rules; most original IDs, aliases and downloadable packs remain unrecovered.
6. **Reconstruction:** current local pages, replacement catalogues/text and inferred missing layouts remain our implementation choices. Do not relabel local emulator screenshots as historical screenshots or extrapolate unseen scroll content as recovered fact.

Exact retrieval metadata is in [catalog.json](data/historical-pages/catalog.json), [July CDX index](../artifacts/historical-pages/blog-cdx.txt), [guide image results](data/historical-pages/archived-guide-images.json), [additional image results](data/historical-pages/archived-july-extra-images.json) and [failed direct/proxy attempts](data/historical-pages/official-source-recovery.json). Original-host matching is stronger provenance than an unrelated re-upload, but it still does not prove which executable or handset produced the composite illustrations.

## Visible page contracts and design cues

| Recovered image | What is visible | Limits |
| --- | --- | --- |
| [chat3c](../artifacts/historical-pages/chat3c.png) | Search Chat Rooms page with search field, Primary Language selector and Also search keywords checkbox | Bottom portion and submission payload not shown |
| [chat5c](../artifacts/historical-pages/chat5c.png) | Create Chatroom: Room Name, maximum 15-character note, Primary Language, optional Description, Keywords and Allow users to kick other users checkbox | Image ends partway down the form; not all fields/actions recovered |
| [room1](../artifacts/historical-pages/room1.png) | Room Info links for language/description, keywords, moderators, banned users, kicking rules and ownership | Destination forms and their validation absent |
| [chat8c](../artifacts/historical-pages/chat8c.png) | Group home with icon, members-only approval status, members/rooms/likes/join-request links; linked-room listing with count, description, unlink/pagination and Link Existing Chatrooms | Group forum/member/approval screens not shown |
| [credits5c](../artifacts/historical-pages/credits5c.png) | Credits menus and Recharge page offering merchant/local store, bank deposit, Western Union, voucher/Ukash, cards and Moneybookers, plus Help/account/store links | Period payment-method presentation only; no payment integration or currency rules recovered |
| [passwordddd](../artifacts/historical-pages/passwordddd.png) | Settings/Login menus and old/new/confirm password form with Submit | Does not establish form field names, endpoint or backend rules |
| [passworddddc](../artifacts/historical-pages/passworddddc.png) | Native Login and Forgot Details forms | These are native screens, not new evidence of embedded recovery HTML |
| [authenticate](../artifacts/historical-pages/authenticate.jpg) | Native activation prompt with Enter Code, Resend Code and Remind me later | Historical copy promises capabilities/credits; replacement services need their own truthful state |
| [public2](../artifacts/historical-pages/public2.png), [pvt2](../artifacts/historical-pages/pvt2.png), [chat11](../artifacts/historical-pages/chat11.png), [chat14c](../artifacts/historical-pages/chat14c.png) | Native room, contact, private-chat and privacy navigation | No profile, gift catalogue or block-list page body |
| [how1](../artifacts/historical-pages/how1.png) | Native connection-test and Welcome presentation | Build unspecified |
| [how-to](../artifacts/historical-pages/how-to.png), [Scr000188](../artifacts/historical-pages/Scr000188.jpg) | WAP download/install material and small merchant/download footer | External WAP/handset UI; `how-to` explicitly shows v4.20 rather than 4.62 |

The official-host page examples use white backgrounds, small proportional black text, blue links, compact grey heading bands, blue/cyan native tab/menu surfaces and orange selection outlines. Form editors are simple outlined rectangles with native controls. Composite screenshot strips combine multiple screens; their total image width must not be mistaken for the handset Canvas width. Exact typography and line breaks remain handset/emulator-dependent. Use [the parser contract](../../../../docs/text-and-page-formatting.md) to generate compatible pages and the bytecode to reproduce native chrome.

The other five images—[Footprints1](../artifacts/historical-pages/Footprints1.png), [callss](../artifacts/historical-pages/callss.png), [settings2](../artifacts/historical-pages/settings2.png), [security](../artifacts/historical-pages/security.jpg) and [123](../artifacts/historical-pages/123.png)—are promotional/mascot artwork, not recovered feature screens. Their filenames must not be used to claim coverage of profile footprints, settings, security or calls pages.

## Community-author profile customization captures

The original author published profile-customization guides on [18 September 2010](https://mig-info.blogspot.com/2010/09/trik-kreasikan-background-profile.html?m=1), [24 September 2010](https://mig-info.blogspot.com/2010/09/update-kreasikan-background-profile.html?m=1), and [30 September 2010](https://mig-info.blogspot.com/2010/09/kreasikan-background-profile-via-wap.html?m=1). Original Blogger entries and image URLs were recovered on 28 October 2026. The [source catalog](data/historical-formatting-sources.json) records publication/update times, retrieval URLs, hashes and attribution limits. The seven retained images belong to the 24 September follow-up; six are 240×320 and one is 200×150.

| Capture | Visible evidence | Limits |
| --- | --- | --- |
| [Red background](../artifacts/historical-formatting/profile-red-background.jpg) | Red profile body, black italic status, blue links, migLevel/likes/friends/gifts and Wall heading | Customized profile; no editor or full scroll content |
| [Small background image](../artifacts/historical-formatting/profile-custom-background-small.jpg) | Profile artwork with pink margin | 200×150 composite; does not establish a handset viewport |
| [Profile link](../artifacts/historical-formatting/profile-my-profile-link.jpg) | Added profile link above ordinary profile fields on background artwork | Link destination is described by the guide; no captured request |
| [Room link](../artifacts/historical-formatting/profile-room-link.jpg) | Added room link above profile fields | No room-join result shown |
| [Private-chat link](../artifacts/historical-formatting/profile-private-chat-link.jpg) | Added private-chat link and an explicit `mig33v42-store` title | Modified 4.20-era client; not stock 4.62 evidence |
| [Yellow text](../artifacts/historical-formatting/profile-yellow-text.jpg) | Yellow italic custom heading over background artwork, with normal profile controls | Cannot recover exact underlying HTML or typography |
| [Large text](../artifacts/historical-formatting/profile-large-text.jpg) | Large black text on a green background | Exposed malformed image-attribute text; not a normative layout |

These are partial profile-home visuals for **L035 and L038**. **L034 remains navigation-only** because none shows the profile-edit form. The guide's authoring method involved modifying a profile-image identifier; the visible effects do not establish that the historical service offered a normal customization editor or supported standard chat BBCode. The [formatting research](../../../../docs/text-and-page-formatting.md) distinguishes recovered effects, client rendering and reconstruction choices. Stock 4.62 accepts the body background-image attribute but does not request or paint it: its page parser leaves the request in an unused vector, and the paint helper ignores the image argument. Background colours and inline photos do render. The background artwork in these modified/unspecified historical clients therefore does not establish stock 4.62 support.

## Coverage of the 52 bundled templates

The product/game pass adds four partial page-family matches. **L007** gains migGames catalog/help images, **L029** gains an actual migWorld news/What is Hot/Help page, **L051** gains gift lists/details with recipient input, and **L052** gains store landing and emoticon purchase steps. **L050 remains navigation-only:** a Themes link does not reveal the theme catalog or purchase body. These conservative mappings describe the visible page family; the original request selectors and exact server HTML remain absent.

Useful references include the [migGames catalog](../artifacts/historical-games/games-2010-1.png), [migWorld page](../artifacts/historical-games/games-2009-1.png), [gift listing](../artifacts/historical-catalogs/new-gifts-1.png), [gift detail](../artifacts/historical-catalogs/mafia-migbot-gift-1.png), [store navigation](../artifacts/historical-catalogs/animation-emot-pack-has-been-released-1.png), [purchase confirmation](../artifacts/historical-catalogs/default-emot-pack-has-been-released-3.png) and [purchase completion](../artifacts/historical-catalogs/default-emot-pack-has-been-released-4.png). The confirmation shows a charge and balance, Yes/No links, followed by Insert Emoticon guidance. The captures do not establish transaction or expiry rules.

The machine-readable [coverage matrix](data/historical-pages/route-design-coverage.json) retains each literal ID, exact URL template, purpose, evidence level and image IDs. All 52 URL contracts remain recovered from the original bytecode in [endpoints-classified.json](data/endpoints-classified.json), regardless of visual recovery status.

| Templates | Service | Historical visual status |
| --- | --- | --- |
| L001–L005 | Account history, overview, recharge, transfer | Recharge partial; other entries navigation only |
| L006 | Avatar selection | Navigation only |
| L007 | Games/bots | Partial migGames directory/help page family |
| L008–L009 | Buzz, calls | No matching page visual |
| L010–L012 | Room create, search, setup | Partial page images |
| L013 | Compose mail | No matching page visual |
| L014 | Link/add room to group | Navigation only |
| L015–L016 | Group home, group chatrooms | Partial page images |
| L017–L019 | Group discovery, own groups, members | Navigation only |
| L020 | Group forum | No matching page visual |
| L021–L028 | Support/help, external IM, leaderboard, check-in, Lookout, merchant | No matching page visual |
| L029 | migWorld | Partial news/What is Hot/Help page |
| L030–L033 | Photos, scrapbook, photo wall | No matching page visual |
| L034 | Profile edit | Navigation only |
| L035 | Profile home | Partial historical-family images from community-author guides; modified/unspecified client |
| L036–L037 | People search, blog posts | No matching page visual |
| L038 | Selected profile home | Same partial profile-home image family; exact route-specific response unrecovered |
| L039–L040 | Room/group abuse reporting | No matching page visual |
| L041–L042 | Block list, call-by-name | Navigation only |
| L043 | Change password | Partial page image |
| L044 | Tab ordering | No matching page visual |
| L045–L048 | Chat/profile/update privacy, change mobile | Navigation only |
| L049 | SMS | No matching page visual |
| L050 | Theme store | Navigation only; no theme catalog body |
| L051 | Gift store with selected recipient | Partial gift list/detail family; exact recipient-bound response absent |
| L052 | Store home | Partial store navigation and purchase-page family; exact id=111 response absent |

## Further recovery and implementation boundaries

Broad archived authenticated-site searches timed out; larger blog image/guide index requests returned 503 or timed out. A narrower July 2010 image index and exact archived replay succeeded. These failures do not prove that additional captures do not exist. Search-engine image results also included third-party desktop clients and later Android/Windows designs; those were excluded as evidence for the target Java client.

Registration and the requested static OTP can be implemented against the recovered native 101/102/103/104/204/205 flows without inventing historical web forms. Commerce, games, mail, external IM and calls need explicit replacement behaviour where the old external services cannot be recreated from the JAR. Keep real local data and working navigation separate from claims of recovered historical billing, content, assets or external-provider integration.

For the eventual Expo recreation, preserve these original images and route contracts as references, use original assets for native chrome where applicable, and mark pages reconstructed from parser constraints until stronger historical evidence appears. A v3 appearance/behaviour setting can share the backend while remaining a separately documented renderer choice.
