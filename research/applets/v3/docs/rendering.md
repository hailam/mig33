# mig33 3.08 rendering and asset specification

This specification describes the recovered **3.08 MIDP 2.0 client**. The original renderer is preserved in [Vineflower output](../artifacts/vineflower/) and independently in [CFR output](../artifacts/decompiled/); [JVM disassembly](../artifacts/bytecode/) resolves ambiguous decompiler types. Coordinates below use integer arithmetic, as in the bytecode. Pair this with [screens.md](screens.md) for screen contents and transitions.

## What exact reproduction means

The app draws most of its interface on one `Canvas`. It also delegates text editing, command/softkey chrome, camera preview, permission prompts, and font rasterization to the MIDP implementation. There is no bundled font. The same JAR therefore produces different pixels on different handsets. Choose a handset/emulator, font metrics, and usable Canvas dimensions before establishing a pixel reference. Do not treat a modern font at an assumed point size as recovered evidence.

All three bundled PNGs are preserved byte for byte. Dynamic profiles, account pages, emoticon packs, CAPTCHA images, received photos, wallpapers, and server-generated menus require server payload fixtures. The JAR defines how to display these, but does not contain their historical contents. See [capture.md](capture.md) for the actual offline screenshot profile and fixture limitations.

## Geometry and fonts

Source: [`com/projectgoth/Roshan/c.java`](../artifacts/vineflower/com/projectgoth/Roshan/c.java), fields and constructor, lines 178–206; `a(Graphics)` immediately below it.

| Symbol | Definition |
| --- | --- |
| `W`, `H` | `Canvas.getWidth()`, `Canvas.getHeight()`; native command chrome is outside this content area |
| `F` | `Font.getFont(64, 0, 8)`: proportional face (`FACE_PROPORTIONAL=64`), plain, small |
| `FH` | `F.getHeight()` |
| `B` | `F.getBaselinePosition()` if >4, otherwise `FH` |
| `T` | Main tab-bar height `B + 4` (`controller.c`) |
| `D` | Main content height `H - T` (`controller.d`) |
| `LH` | Chat line height: `FH` when >5, otherwise baseline +3 |
| `E` | Current emoticon side, preference 12, default 12 |

Other named fonts: small bold `(64,1,8)`, medium plain `(64,0,0)`, large plain `(64,0,16)`. The browser also uses italic, underlined, and large bold variants. MIDP anchors matter: `20=TOP|LEFT`, `17=TOP|HCENTER`, `24=TOP|RIGHT`, `3=VCENTER|HCENTER`. Text centering often uses **baseline-derived `B`**, not the full glyph bounding box. Preserve this asymmetry.

Keep native integer division order. Examples: overlay width is `(W / 8) * 7`, not `W * 7 / 8`; overlay height is `(D / 16) * 15`. Do not replace these with floating-point percentages.

## Colors and shared drawing primitives

Source: [`com/projectgoth/Roshan/d.java`](../artifacts/vineflower/com/projectgoth/Roshan/d.java), lines 68–144; [literal color inventory](data/colors.json) includes every source location.

| Use | RGB / hex |
| --- | --- |
| Canvas / control interior | white `#ffffff` |
| Text and outlines | black `#000000` |
| Title / normal dialog header | `(170,204,238)` `#aaccee` |
| Error dialog header | `(255,128,128)` `#ff8080` |
| Main content surround / active-tab bottom | `(142,179,231)` `#8eb3e7` |
| Selection outline / content top-left bevel / group text | `(51,85,153)` `#335599` |
| Content bottom-right bevel | `(136,170,221)` `#88aadd` |
| Selected menu/contact text background | `(242,245,251)` `#f2f5fb` |
| Dialog shadow / separators | `(160,160,160)` `#a0a0a0` |
| Disabled menu item | `(192,192,192)` `#c0c0c0` |
| Control top/left border | `(148,149,162)` `#9495a2` |
| Control bottom/right border | `(90,91,100)` `#5a5b64` |
| Button gradient | white → `(188,212,246)` `#bcd4f6` |
| Focus trim, top to bottom | `#fff0cf`, `#fdd889`, `#fbca6a`, `#f8b330`, `#e59700` |

The shared header clears `(0,0,W,height)`, fills `(2,2,W-3,height-1)` blue, centers title at `(W/2,height/2-B/2)`, outlines `(0,0,W-1,height)` and `(0,0,W-1,H-1)`. Its boolean option also fills a blue footer `(0,H-height,W,height)`.

Controls use a white interior, two light border segments at left/top, and two dark segments at right/bottom. Focus consists of five gold edge strokes; it does **not** change the whole control to a gold fill. Gradients draw one horizontal line for every interior scanline. Channel interpolation is `(end*(y-startY) + start*(endY-y))/(endY-startY)` with integer truncation (`ab.a`).

## Assets and sprite atlas

See the visual [asset atlas](assets.html), [PNG metadata](data/assets.json), and [sprite rectangles](data/sprite-atlas.json).

| Resource | Dimensions | Role |
| --- | --- | --- |
| `MiG33.png` | 16×16 | MIDlet launcher icon |
| `j.png` | 132×48 | UI sprite atlas; 42 used cells, each 12×12 |
| `k.png` | 118×62 | pale mig33 background/logo, replaced if `Background` RMS contains an image |

Sprite `n` uses **column-major** addressing: `x=floor(n/4)*12`, `y=(n%4)*12`. IDs 0–41 come from `j.png`. ID 42 is the separately loaded logo/background, not an atlas cell. Drawing uses ARGB with alpha enabled. No resampling is used for the 12×12 icons.

| ID | Code-established role |
| --- | --- |
| 0,1 | Checkbox unchecked / checked |
| 2,3 | Focused checkbox unchecked / checked |
| 4,5 | Expanded / collapsed contact group |
| 8–11 | mig33: online, away, busy, offline |
| 12–15 | MSN: online, away, busy, offline |
| 16–19 | Google Talk: online, away, busy, offline |
| 20–23 | Yahoo!: online, away, busy, offline |
| 24–27 | AIM: online, away, busy, offline |
| 28–31 | Browser loading-animation frames |
| 32 | Directory / drive |
| 33 | Image-file icon / browser image placeholder |
| 34 | MP3 file |
| 35 | Other file |
| 36 | Parent directory |
| 37 | Browser missing-image/loading placeholder |
| 38,39 | Mail with unread items / no unread items |
| 40 | Chat Rooms row |
| 41 | Retained atlas cell; consult call-site inventory before assigning a semantic name |

IDs 6,7 and any context-specific uses remain available in the atlas even where no semantic name is established. Network status uses 1=online, 4=away, 3=busy, 99=offline. Icon offsets are 0,1,2,3 respectively. This is not numeric status order. Sources: controller `a(int)`; `t` checkbox renderer; `r.a(...,String service)`; `c.c(String filename)`.

## Static forms

Source: [`t.java`](../artifacts/vineflower/t.java), constructor for control order; lines 567 onward for paint helpers. The [screen catalog](screens.md) specifies every concrete form.

A form clears the whole Canvas white, selects small plain font, draws controls, then draws the shared title over them. Initial body y is `FH+2`. Constants: left gutter `g=4`; available width `k=W-6`; field width `l=k-20=W-26`; field height `FH+2`; label line height `FH+2`; button height `FH+4`; spacer height 3.

| Internal type | Rendering and space consumed |
| --- | --- |
| 0 | Text field box x=4 through x=4+l; text x=8; consumes `FH+4` |
| 1 | Same box; display one `*` for each input character |
| 2 | Plain label x=4, vertically centered by baseline; consumes `FH+3` |
| 3 | Field with small gradient selector area and triangle at right; text available width `l-fieldHeight-4` |
| 4 | Blank 3-pixel vertical spacer |
| 5 | Button x=6, width `stringWidth(label)+10`, height `FH+4`; text x=12; consumes `FH+6` |
| 6 | Checkbox or informational wrapped line, x=6; optional icon at y+2; text starts x=22; consumes `FH+4` in paint |
| 16 | Security-code field with x endpoints `4` and `(4+l)/2`, y endpoints `y+1` and `y+1+fieldHeight`; server CAPTCHA image at x=4, y+fieldHeight+3; consumes `fieldHeight+imageHeight+3` |

The scroll-budget helper for type6 uses `FH+2`, while paint advances `FH+4`; reproduce the implementation if matching scroll boundaries. `t.b()/d()` skip labels, spacers, and noninteractive informational rows. Forward and reverse painting have separate algorithms. Long fields are truncated using the client's ellipsis routine, not wrapped. Full explanatory text is split into informational rows by font width.

Focus changes the field/button edge; checkbox focus selects sprite 2/3. A field opens a native `TextBox` on activation. There is no inline blinking caret in the Canvas form. Registration's Password uses type0 and is visibly plain; login and external IM account setup use masked type1. The connection-choice form (42) paints the background/logo behind its controls at `x=(W-6)/2-logoWidth/2`, `y=(FH+2)+(H-(FH+2))/2-logoHeight/2`; its horizontal position is offset from exact screen centering.

## Friends list and conversations

Sources: controller `paint` lines 467–653; [`r.java`](../artifacts/vineflower/r.java) lines 663–879; [`ab.java`](../artifacts/vineflower/ab.java) lines 276–450; [`v.java`](../artifacts/vineflower/v.java).

The top tab area is `T=B+4`. Below it, paint blue `(0,T,W,H-T)`, white `(2,T+2,W-4,D-4)`, and clip to that white rectangle. Draw the selected friends list or conversation, then a dark top/left and light bottom/right bevel. The friends logo is painted before rows at `x=2+(W-10)/2-logoWidth/2`, `y=T+2+(D-4)/2-logoHeight/2`. Preserve its three-pixel horizontal offset from exact centering for even widths.

Friends row height is `max(B+6,13)`. Row origin is `(2,T+4)`; inner width is `W-4`. Groups have 12×12 expand/collapse icons at x=5, text at x=22. Contacts have status/service icon x=10 and text x=28. Both selected rows use bold small font, a pale highlight only as wide as the displayed text, and blue rectangular outline. Group text is blue; contact text black. The Chat Rooms and optional Mail rows use their dedicated icons and labels. Collapsed groups hide the following group-member count. Offline filtering and service priority are implemented in `r`; selected service preference is mig33, MSN, Yahoo!, Google Talk, AIM when their status is not 99 (so away/busy also qualify), with fallbacks when all are offline.

Each tab has nominal width `(W-12)/2`, overlaps its neighbor by `T/2`, and begins from x=8. Slanted tab left edges are drawn with lines. Inactive background gradient is `#e4eefc → #c0d8f9`; selected is `#d5e6fb → #8eb3e7`; unread flashing is `#f5c258 → #ff8000`. Tab outer border is `#3b619c` with white bevel. Title is small bold when selected and small plain otherwise, truncated to tab width minus T. Labels are `Friends`, `Group Chat`, room name, or contact display name. Left/right offscreen unread arrows flash orange. The main loop toggles the flash approximately every 750ms while notifications are pending.

Chat starts x=5, y=T+2. Available wrapping width is `(W-4)-13`; small plain text uses `LH`. Sender prefix is truncated to half the available width and followed by `": "`. Prefix and message colors may be server overrides. Default color categories are `0=#086834`, `1=#406ac0`, `2=#ff0000`, `3=#ff7300`, `4=#fcc504`, `5=#993300`, `6=#000000`. The binary's literal values and [color catalog](data/colors.json) are authoritative. The transcript keeps up to 50 rendered line entries; wrapping can make one message occupy several entries. It supports both top-anchored and bottom-anchored scroll paint.

Inline emoticons occupy `E+1` pixels; missing ones draw a dotted gray E×E placeholder and enqueue a download. The internal transcript uses `¤` as color delimiters and `±` around emoticon identifiers. Incoming literal instances of these characters are replaced with spaces before formatting. Images/colors are not browser HTML in this transcript. Source `v.a(message...)` is the exact wrapping/formatting algorithm; a generic rich-text widget will not reproduce its line breaks.

## Menus, errors, confirmation and progress overlays

Source: [`e.java`](../artifacts/vineflower/e.java). Default width `(W/8)*7`, default height `(D/16)*15`, centered at `((W-width)/2, T+(D-height)/2)`. Individual construction sites may alter height. Shadow is a solid `#a0a0a0` rectangle offset (+3,+3). Clip to the overlay, fill white, then paint a blue header inset 2px; error overlay ID21 gets a salmon header. Outline is black.

Header and row line height is `FH+2`. Separators are a gray line inset 4px and consume 3px. Text-only rows start x+8; icon rows have icon x+4 and text x+20. Optional right-side values end at x+width-8. Selection outline encloses the label width plus 4px, with pale fill inset 1px. Disabled rows are gray. Rows wrap selection from last to first except text dialogs, which scroll paragraphs. Dialog IDs20,21,22,37,40 center wrapped text; progress ID23 uses its dedicated animation. Up/down scroll arrows are small line-drawn black triangles, not asset images. Header, border, and reverse-scroll painting use separate code paths; retain them when comparing cropped lower rows.

## Emoticon picker

Source: [`i.java`](../artifacts/vineflower/i.java), `a(Graphics)` around line370. Cell stride is `E+3`; columns `W/stride-2`, rows `H/stride-2`. The grid is centered with `(W-cols*stride)/2` and `(D-rows*stride)/2`. Missing assets appear as dotted gray squares. Selection is a double black rectangle around the cell. The selected emoticon and `[description] alias` are repeated below the grid. Lists, aliases, descriptions, and bitmap pixels are downloaded/cached; **the JAR contains no complete emoticon collection**. Some legacy arrow coordinates use fixed 16-pixel multipliers despite the configurable cell size.

## Embedded page renderer

Sources: [`k.java`](../artifacts/vineflower/k.java), [`m.java`](../artifacts/vineflower/m.java), `o,h,g,d,y,aa,u,b,ad,s,x` in the same directory. This is a small custom parser, not a standards-compliant WebView. Do not substitute a modern browser when matching historical output.

Viewport body width is `W-13`. Header/footer each use `FH+2`; body height `H-2*(FH+2)`. Elements store absolute layout rectangles; scrolling translates x by `-horizontalOffset`, y by `headerHeight-verticalOffset`. It supports text/bold/italic/underline, `<big>`, centered text, paragraphs and breaks, limited div/span styling, links, custom `<tag>` actions, images, form text/password/hidden/submit/reset/checkbox inputs, selects/options, and textareas. See `k.a(Graphics)` for the exact accepted attribute spelling and processing order; arbitrary CSS/JavaScript/table layouts are not implemented.

Background can be a color, centered image, or nearest-neighbor stretched image. Unknown images use sprite placeholders; selected images have gold outlines. Text links use the client text renderer and current font, not OS link widgets. Editable browser controls open native text boxes. Form submission is built by `m`, using the client's limited escaping. Named color `blue` resolves to integer4095 (`#000fff`), not CSS `#0000ff`; this quirk is preserved in `Roshan/d.b(String)`. Relative URL resolution recognizes `http://`; it is not a full modern URI resolver.

Browser loading states are `Loading...`, `Loading Images n/total`, `Loading Complete`, and `Error loading`. Animated GIF decoding is implemented in `x`, with frame storage in `s`. Server-pushed page body, title, links and asset bytes remain necessary for exact pages.

## Camera and file selection

Sources: [`z.java`](../artifacts/vineflower/z.java), default-package [`c.java`](../artifacts/vineflower/c.java). Camera preview is a native `Form("Camera")` with `VideoControl`; native appearance is device-specific. Capture review is a custom `Send Photo` screen: white background, shared title of height B+6, centered `Processing...` or centered preview. Landscape preview width=3W/5; portrait preview height=3H/5; preserve aspect ratio using integer nearest-neighbor sampling.

File selector is a centered carousel: selected row at H/2, large font, boxed control; adjacent rows use small font above/below. Title `Select File`, shared header/footer; directory/image/audio/other/parent icons use the sprite table. Image confirmation title is `Confirm`; display image at `(-horizontalOffset, smallFontHeight-verticalOffset)` and show scroll arrows as needed. File payload limit is 153600 bytes. Native filesystem contents, camera imagery, and OS authorization prompts are external inputs.

## Complete low-level reference and validation

[draw-calls.json](data/draw-calls.json) indexes every direct drawing/color/font/clip statement in the alternative decompilation. [string-locations.json](data/string-locations.json) indexes Java literals; [class-constants.json](data/class-constants.json) independently extracts original constant-pool strings and referenced classes. This makes hidden messages and conditional code auditable beyond screenshots.

Validate a recreation at the same Canvas size/font metrics, with fixed ordered contacts, service states, room names, transcripts, server menu/page responses, emoticon assets, and RMS preferences. Compare top and bottom scroll positions, selected and unselected controls, disabled commands, keyboard/text-editor return, reconnect, notification flash phase, and server error states. Screenshots with fabricated fixture data demonstrate layout, not historical live account contents.
