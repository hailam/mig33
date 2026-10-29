# mig33 3.08 compared with 4.62

**4.62 is a larger redesign of the Java client, with additional protocol messages, theme/localization resources, richer tabbed navigation, avatars and a revised registration flow.** Both versions retain the same basic nine-byte binary packet envelope. Exact operation/field changes are in [protocol-differences.md](protocol-differences.md). The [visual comparison](comparison.html) uses original-bytecode screenshots, with fixtures and emulator limits explained alongside each version.

| Aspect | 3.08 | 4.62 keypad |
| --- | --- | --- |
| Publisher identity |Project Goth, inc.; `com.projectgoth.Roshan.Main`|Project Goth, Inc.; `main.Main`; build .000|
| Original JAR size |115314 bytes|190378 bytes, 75064 bytes larger|
| Obfuscated class count |35|26; more classes/functions have been merged|
| Main visual style |Pale-blue forms, slanted overlapping conversation tabs, centered light dialogs|Teal gradient headers/tabs, compact inactive icon tabs, teal menus with orange selection|
| Theme resources |Drawing colors mostly literal; 3 bundled PNGs|148 tagged default-theme fields including 9 embedded PNGs; 2 further root PNGs|
| Strings |Mainly Java string literals|419-entry bundled English language packet plus code literals; downloadable locale handling|
| UI atlas |132×48, 42 loaded 12×12 cells|240×48, 80 available 12×12 cells|
| Launcher icon |16×16|24×24|
| Registration |Single form with username,password,mobile,CAPTCHA,terms|Multi-step onboarding and registration, including suggested-name and activation states|
| Registration password appearance |Plain text in original form and editor|Masked field|
| Login |Save Password,Invisible,external IM toggles,Show offline contacts|Remember Login,Auto Login,Invisible,Show Chatroom Avatars; links and revised saved settings|
| Friends/conversations |Contact groups,status icons,conversation tabs|Avatar/status metadata,richer contact/tab model and additional controls|
| IM integrations |mig33,MSN,Yahoo!,Google Talk,AIM|Additional Facebook-related service handling and imagery|
| File selection |GIF,JPEG,JPG recognition;153600-byte actual read limit|Adds PNG;512000-byte actual limit while English error text says 300kb|
| Local persistence |Named RMS stores such asSignIn,Save Password,frms|Many numeric record-store IDs,structured settings,theme/language/contact caches; migration logic|
| Wire transport |Binary TCP with HTTP fallback|Same framing family with broader operations and session/metadata behavior|
| Incoming dispatcher cases |33|55; see exact sets and added operations in protocol difference catalog|
| Pages and downloaded content |Custom page engine;server data required|Expanded page/cache/theme/service presentation;server data still required|

UI claims are grounded in [v3 screens](../v3/docs/screens.md), [v3 rendering](../v3/docs/rendering.md), [v4 screens](../v4/docs/screens.md), [v4 rendering](../v4/docs/rendering.md), their referenced original code and extracted resources. Feature presence is established from implementation; it does not prove every server feature was enabled for every account, country or date.

## Shared wire format does not mean interchangeable clients

Both serialize marker02, opcode(short), transaction(short), body-length(int), then tag(short)/length(int)/bytes fields. Common serialization fixtures match. However, field schemas, authentication choices, client identifiers, configuration delivery, page handling and additional operations differ. A recreation should use an explicit version profile and version-specific fixtures. Do not silently take a v4 field table and call it a v3 API.

Several historical quirks matter: v3 registration initialization writes width and then height to the same tag; the v4 analysis also finds duplicate-tag and inconsistent-accessor behavior. The reference docs preserve these instead of inventing corrected protocols. Descriptive operation names are inferred from client behavior; numeric IDs and wire widths are the recovered evidence.

## Recreating the appearance

Keep the two renderers separate. Share low-level binary transport and device abstractions only where bytecode/fixtures establish compatibility. V3's fixed palette and row formulas cannot reproduce v4's replaceable theme, richer control types or tab geometry. In v4, locale/theme selection is part of the visual state, not decoration added after rendering.

Both clients use native MIDP fonts and text editors. Establish a device profile before pixel comparisons. Capture manifests identify Canvas size, emulator, original JAR hash, image hashes and synthetic fixtures. Native command chrome, downloaded pages and user imagery are not supplied by a screenshot of a static form.

## Release and popularity conclusions

The official February 2014 Java downloads page labeled the 4.6 line newest, and the recovered official keypad JAR/JAD declare 4.62. The official touch variant recovered here declares 4.60 beta .005 and is not the primary v4 reference. This is the latest publisher-verified **mig33 Java keypad** build found, without claiming that later migme or other-platform releases never existed. [Primary evidence and scope](../v4/docs/provenance.md).

There is no verified evidence in the retrieved material that v3 was the most popular version. Mig33's published membership grew during the v4 period, but those service-wide totals do not identify which Java client people used. [Popularity investigation and dated sources](history-and-popularity.md).
