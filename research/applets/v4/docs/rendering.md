# mig33 4.62 rendering and assets

This is the **keypad 4.62 build .000**, using its bundled `Default V2` theme and `en-US` language pack. It is a separate renderer from [3.08](../../v3/docs/rendering.md). Its full screen contents, constraints and actions are in [screens.md](screens.md); default visuals are in the [asset gallery](assets.html) and [original-bytecode captures](capture.md).

## Fidelity boundary and source hierarchy

Native MIDP fonts, text editors, softkey bars, camera preview, filesystem permissions and usable Canvas dimensions depend on the handset/emulator. There is no recovered universal phone font or one canonical physical screen resolution. All documented drawing coordinates operate inside the Canvas. Screenshot fixtures specify an emulator profile and synthetic data; they do not recover a historical account's server responses.

Use [original JAR](../artifacts/original/mig33-4.62.jar) and [javap disassembly](../artifacts/bytecode/) as authoritative. [Vineflower](../artifacts/vineflower/), [CFR](../artifacts/decompiled/) and [Procyon](../artifacts/procyon/) are reading aids. V4 merges unrelated classes into classes with colliding member names/types; the source is not directly recompilable. Some methods defeated all three decompilers.

[normal-flow](../artifacts/normal-flow/) exposes those methods by decompiling an **analysis-only derivative with exception tables omitted**. The original bytecode instruction bytes were asserted unchanged by [the transformation script](../../../../tools/applets/scripts/decompile_normal_flow.py). This derivative is never executed. It describes branch and draw order on the normal path; it cannot establish catch, finally, cleanup or exceptional behavior. Those remain in original disassembly. Do not rebuild the app from the derivative.

## Theme and localization resource formats

`default_v2` is a 23,718-byte serialized protocol packet with opcode 1000, transaction 0 and 148 tagged fields. `en-US` is a 10,411-byte opcode 1004 packet containing 419 strings. They use the same nine-byte framing and six-byte-per-field overhead as the network protocol. The [theme decoder](../../../../tools/applets/scripts/extract_v4_theme.py) checks exact framing length and extracts every field unchanged. Theme packets can also be replaced by server/RMS data.

| Getter | Meaning |
| --- | --- |
| `y.d(int):int` | Read signed BE int32 from theme `aj.j`; absent field returns 0 |
| `y.d(int):Image` | Read PNG bytes from the same field namespace, with per-ID caching |
| `y.c(int):boolean` | Theme field exists |
| `y.e(int):String` | Raw UTF-8 language value from `aj.s`; missing value may be null |

JVM method descriptors distinguish overloaded getters even where a decompiler makes the Java ambiguous. Default theme 0=`default_v2`, theme 1=21, theme 2=`Default V2`. The full [148-field table](theme-fields.md), [hash/byte inventory](data/theme.json), and [literal getter references](data/theme-usage.json) retain every value. RGB previews of four-byte values are only interpretations: values such as 40, 1 or -1 can be dimensions/flags/sentinels. Dynamic indexes require following the consuming method.

Theme loads from RMS 33 if available; otherwise from bundled `default_v2`. Language loads from RMS 45, validated against saved locale and marker 361, with fallback to `en-US`. Preserve numeric IDs independently from translated labels. [All English strings](data/screens-en-US.json) are extracted without normalization, including misspellings and contradictory limit text.

## All image assets

| Source | Dimensions | Role / evidence |
| --- | --- | --- |
| Root `MiG33.png` | 24×24 | MIDlet launcher icon |
| Root `default-avatar.png` | 24×24 | Default avatar resource |
| Theme74 | 320×95 | Header/background artwork; retain as original PNG |
| Theme122 | 80×45 | mig33 logo used by first-run/welcome presentation |
| Theme129 | 240×48 | 80-cell application sprite sheet, 12×12 cells |
| Theme130 | 6×6 | Small indicator artwork |
| Theme131 | 36×39 | Status/dialog artwork |
| Theme132 | 25×25 | Default friends/avatar placeholder |
| Theme133 | 6×6 | Small indicator artwork |
| Theme136,137 | 24×24 each | Additional service/presence imagery |

All nine embedded PNGs were extracted byte for byte into [artifacts/theme](../artifacts/theme/). Their dimensions, offsets, lengths and SHA-256 hashes are in [theme.json](data/theme.json). Root PNGs are separately inventoried in [assets.json](data/assets.json). There is **no bundled theme126 image** in this default pack; the normal background-image getter therefore returns null. Missing theme136/137 have a special 1×1 fallback; other missing images generally return null.

Sprite extraction remains column-major: `x=(id/4)*12`, `y=(id%4)*12`; `Graphics.drawRegion` uses no transform. The sheet has 80 cells, versus v3's 42 used cells. Do not copy a v3 meaning onto every index. V4 code uses 52 Friends tab, 53 Chat tab, 54 a service/server tab, 56 page tab, 57 group/room tab, 58 private/conversation tab; 59 submenu arrow; 69 combo-box arrow; 28–31 loading animation. Exact status/provider icons and variable IDs are selected by `y.a(int,bj)`, `y.b(int,bj)` and the friends renderer. The sheet includes additional Facebook imagery absent from v3.

## Dimensions and fonts

Source: original `aj.<init>` disassembly; readable [normal-flow/aj.java](../artifacts/normal-flow/aj.java) constructor around 6733. `W=aj.e=getWidth()`, `H=aj.f=getHeight()`. Font is proportional-small-plain (`FACE_PROPORTIONAL=64`), `(64,0,8)`, bold-small `(64,1,8)`, medium-plain `(64,0,0)`, large-plain `(64,0,16)`.

Let `B` be baseline position unless baseline≤4, then font height. Main tab height `T=aj.g=B+9`; body height `H-T`; shared header/footer metric `aj.F=B+6`; content extents `aj.q=H-T-4`, `aj.r=W-4`. Chat line metric `aj.D` is font height when>5, otherwise baseline+3. Normal avatar metric `aj.G=2*(B+6)-2`. Emoticon default side is 12; server settings can change it. Use actual runtime font measurements and integer arithmetic.

The selected main tab starts with nominal width 61, inactive tabs 21. Available strip width is `W-14`; visible count is `(available-61)/21+1`; the base selected width is adjusted to consume the remainder. `k` keeps separate active/base widths, so use its calculation and current selected-tab state. Tab bodies are rectangular with rounded-corner line details, replacing v3's slanted overlapping tabs. Inactive tabs show only an icon; the active tab adds a bold, width-truncated title.

## Default colors and primitives

Colors below are **theme defaults**, not hardcoded permanent colors. Sources: `y` primitive methods and `k` tab paint. All other theme values and references are listed in [theme-fields.md](theme-fields.md).

| Component | Theme tags | Default values |
| --- | --- | --- |
| Form header gradient / text |62,63 /64| `#0991b8 → #0c4b7b` / white |
| Active tab gradient |5,6| `#098fb6 → #0c4b7b` |
| Inactive tab gradient |8,9| `#0f95d3 → #05bad3` |
| Unread tab flash |11,12| `#efcf06 → #fe8300` |
| Tab border / active text |14 /17| `#02485d` / white |
| Tab-strip backdrop |3,4| white → `#d5d5d5` |
| Button gradient / text |18,19 /20| white → `#cbcbcb` / `#222222` |
| Menu header / header text |21 /22| `#0c5787` / white |
| Menu normal / selected row |23 /24| `#0b6d98` / `#e66a02` |
| Menu row text / disabled |25,26 /30| white / `#09acd3` |
| Menu border |31| `#0b4778` |
| Dialog edge / body / title |44,45,46| `#0c4b7b`, white, `#0c4b7b` |
| Input text / labels / body |47,48,49| `#222222`, `#222222`, white |
| Input focus top / other trim |50,51| `#fdb87b`, `#f37905` |
| Input bevel top-left / bottom-right |52,53| `#9b9ca8`, `#62626e` |
| Main background / border |123,124| white / `#0b4778` |

`y.a(Graphics,x,y,border,width,height,...)` first paints the theme124 border, clips to the inset, fills theme123 or the requested override, then optionally tiles/anchors the theme126 background image. Image tiling and anchor handling are client algorithms, not CSS background defaults. Rounded-body corners are formed by painting four border-color pixels. Linear gradients are generated into int arrays and drawn per scanline. Native softkey chrome is separate.

## Forms and input controls

Source: [y.java](../artifacts/vineflower/y.java), form builder at 2726, main form paint 107, control methods around 672–930. Forms have a `B+6`-pixel gradient header. Body clears/clips to inset2 below the header. Full input width remains `W-26`, x=4; field height is small-font-height+2, button height font-height+4, spacer3. Header title is small plain white, horizontally centered at y=0. Gold focus now consists of two inset rectangular outlines (theme51), with theme50 top highlight.

| Row type | Rendering |
| --- | --- |
|0|Plain text field; text x=6|
|1|Masked password field, one star per input character|
|2|Label/wrapped information line|
|3|Contact/number selector with sprite6 at right; optional display label distinct from value|
|4|Three-pixel spacer|
|5|Gradient action button at x=6, label x=12, label-width+10 endpoint calculation|
|6|Checkbox/information row, using atlas states and stored selection|
|9|Phone field beginning x=16, with literal `+` outside at x=6; editable digits begin x=18|
|10|Link-styled action row, distinct from gradient button|
|12|Combo box with sprite69 and popup option list|
|15|Image row with alignment metadata and 5px leading space|
|16|Security-code field with server CAPTCHA below|
|17|Right-side image plus text, with image-aware vertical alignment|

The main renderer reserves combo rows, paints other controls, then draws the expanded focused combo last so it overlays later rows. Combo popup opens upward when the field is below half-screen, otherwise downward; visible rows and starting option derive from available height and selection. Preserve native text-box constraints and return state from [screens.md](screens.md). The three-step registration uses password type1, correcting v3's visible registration password behavior.

Forward and reverse scroll paint remain different paths. Text truncation uses the client's measured-font `...` routine; explanation wrapping uses its tokenization. Do not substitute generic browser line wrapping if comparing pixels. All exact localized row labels and order are in the screen-form data.

## Menus, dialogs and temporary notifications

V4 menus are positioned above the lower command area. In `y.a(Graphics,x,bottomInset)`, top y=`H-visibleRowsHeight-titleHeight-bottomInset`. Menu width is computed from its content/model, not the v3 fixed seven-eighths screen width. Header is small bold with white text, body is teal, selected row orange, disabled text cyan. Icon gutter adds 16px; submenu arrow consumes 12px; optional right label and boxed shortcut consume width. Rows are clipped/truncated to the remainder. Separators are conditional per item. Scroll arrows occupy eight-pixel bands at the visible menu edges.

Modal styles are handled by `y.a(Graphics,boolean)`: full-screen status, full-screen welcome with logo, bordered centered dialogs, and a bottom status strip. Their shared status illustration is theme131. Bottom status strip uses loading icons 28–31 and resets after 31; it is not the v3 centered progress rectangle. Exact titles, buttons, widths and constructed modes are in [screens.md](screens.md) and constructor/call-site inventories.

## Friends, chats and service tabs

`au` renders friends/groups/profile summary using the theme; `k` holds the tab model; `s` combines conversation and page layout behavior. V4 can display avatar thumbnails and status text, online counts, richer chatroom presentation and additional service tabs. Data and visibility depend on server responses and preferences. Default avatar uses theme132, centered in a `G×G` square; downloaded avatar URLs are formed from a server base with requested width/height. Missing download requests are queued only while connected.

Main tab model types and captions are resolved in `k.a(...)`: Friends, Chat, conversation, group/room, browser/server pages and other services. Full normal-path draw order is readable in [normal-flow/aj.paint](../artifacts/normal-flow/aj.java), [normal-flow/au](../artifacts/normal-flow/au.java), [normal-flow/k](../artifacts/normal-flow/k.java), and [normal-flow/s](../artifacts/normal-flow/s.java). Preserve top-level paint invalidation flags: v4 can repaint selected regions rather than clearing everything every frame.

Conversation history still caps its `y` entries at 50; multi-part wrapped entries are prefixed with internal `▲part/total▲` markers. V4 adds richer inline metadata, avatar placement, colors, timestamps/gap handling and links, so v3's line model alone is insufficient. Theme142–151 supply conversation palette categories and companion flags. Content is client-formatted from packets; it is not a native text widget.

Server HTML pages are rendered by the custom layout engine and resource cache (`s`, `bl`, `bj`, `y`, `av`), not a contemporary browser. Images, native input activations, relative URL rules, cached pages, selected links, and server-specified presentation need the original parser behavior. The exact normal-path parser and element paint are retained in source; absent historical bodies cannot be inferred from page titles or menu names.

## Photo, file and device-dependent views

File selection and image confirmation use `ao`; camera uses `d` plus native `VideoControl`. PNG is now recognized as an image alongside GIF/JPEG/JPG. The actual file-read ceiling is **512000 bytes** (`ao.java` near492), while language entry 125 says **300kb**. Retain this discrepancy in a faithful reproduction instead of treating the label as enforcement. All file roots, actual photo pixels, camera driver behavior and OS prompts are external inputs.

## Exhaustive reconstruction references

- [Every drawing statement](data/draw-calls.json): 499 direct draw/color/font/clip statements in the first Vineflower pass. Failed methods are additionally covered by original bytecode and the normal-flow reference; this count is not an all-method coverage claim.
- [Theme getter locations](data/theme-usage.json) and [all default fields](theme-fields.md): exact colors, dimensions, flags and images.
- [English pack](data/screens-en-US.json): all 419 strings with numeric IDs.
- [Original constant-pool strings/classes](data/class-constants.json) and [Java string locations](data/string-locations.json): literals outside the language pack and original class references.
- [Source inventory](data/jar-inventory.json): every extracted JAR entry and hash.

Compare at the same Canvas/font profile with fixed language, theme, RMS state, selected tabs, scroll position, flash phase, server menu/page fixtures, contacts and images. Dynamic theme or locale updates can alter every visible screen without changing the JAR version. Screenshot differences across operating systems can also come solely from font rasterization.
