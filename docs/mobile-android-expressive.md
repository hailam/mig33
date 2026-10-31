# Android Material 3 Expressive plan

Status: researched on 2026-10-31; **not implemented or verified on Android**.
No dependencies were installed for this research. The intended Modern Android
presentation uses native Jetpack Compose Material 3 Expressive. iOS retains its
native Liquid Glass presentation; Classic v3/v4 retain their reconstructed
appearance. Material's color, typography, shapes and motion form the Android
design system. [Android Material 3 guidance](https://developer.android.com/develop/ui/compose/designsystems/material3)

The source audit used the installed **`@expo/ui 58.0.24`**, not assumptions from
newer documentation. Under `apps/mobile/node_modules/@expo/ui/`,
`android/build.gradle:41–48` declares Compose Foundation/UI **1.12.1**,
Material3 **1.5.0-beta01**, and graphics-shapes **1.1.0**. Material3 is therefore
a prerelease dependency already selected by Expo. Preserve that version set
for the initial Android build and inspect the resolved Gradle graph before
changing it. Documentation examples may show different versions.

| Capability | Verified installed support and boundary |
| --- | --- |
| Expressive theme | `android/.../HostView.kt:202` calls `MaterialExpressiveTheme`. Host exposes light/dark appearance, seed color and color-role overrides. |
| Native controls | Buttons, switches, selectors, text fields, navigation bar, floating toolbar, menus, alerts and modal bottom sheets are exposed. |
| Spring motion | `src/jetpack-compose/modifiers/animation.ts` exposes `spring` and `animated`; native `AnimatableFloat.kt` uses `animateFloatAsState`. Content-size and color animations are also available. |
| Shape library | `modifiers/index.ts` exposes 27 Material shape presets for clipping/shadows. `Shape` also exposes configurable polygons, stars and rounded shapes. |
| Loading morphs | Native `LoadingIndicator` and `ContainedLoadingIndicator` render morphing shapes. |
| Button press morphs | Stock Button/IconButton wrappers use a static `shape` overload. They do not expose the native interaction-driven `shapes` overload. |
| Custom morphs/transitions | No exposed general `Morph(progress)` or shared-element bridge was found. Host does not expose global motion-scheme or shape-theme customization. |

Expo documents these as native Compose components hosted inside React Native.
[Expo Compose overview](https://docs.expo.dev/versions/latest/sdk/ui/jetpack-compose/)
Native button morphing specifically requires the `ButtonShapes` API; applying
an Expressive theme alone does not activate that overload.
[Android Button API](https://developer.android.com/reference/kotlin/androidx/compose/material3/Button.composable)
Polygon morphing supports compatible rounded polygon geometry, rather than
arbitrary logo or icon paths.
[Android shape guidance](https://developer.android.com/develop/ui/compose/graphics/draw/shapes)

Keep this a Go-first repository. The original HTTP/socket contracts, generated
definitions, handwritten client and session controller remain shared. Android
presentation must not duplicate authorization, command catalogs, packet
handling or business logic in Kotlin. Retain the app-local mise and pnpm setup,
72-hour Node dependency age policy with documented advisory exceptions, and
uv for Python tooling. Any new native dependencies require explicit version
review; pnpm's age gate does not evaluate Maven artifacts.

Implementation proceeds in these steps:

1. Establish the Android SDK/JDK build profile and a reproducible development
   build under `apps/mobile`. Keep generated `apps/mobile/android/` ignored;
   configuration belongs in `app.config.ts` and reproducible plugins/modules.
2. Add `src/ui/ModernControls.android.tsx` and
   `src/ui/NativePresentation.android.tsx` behind the existing shared interfaces.
   Add Android navigation/surface adapters where platform structure differs.
   Use Material symbols and native semantics, menus, dialogs and sheet behavior.
3. Generate coherent Material color roles from mig33's teal using Host
   `seedColor`, and share those roles with React Native content. Treat optional
   wallpaper personalization as a separate preference.
   [Expo Host](https://docs.expo.dev/versions/latest/sdk/ui/jetpack-compose/host/)
4. Use native component motion first. If required press morphs remain absent,
   add a narrow local module at `apps/mobile/modules/mig33-material/` wrapping
   the appropriate Compose overloads. Add custom polygon morphing only for an
   identified interaction. Expo supports local Compose extensions through its
   module API. [Extension guide](https://docs.expo.dev/versions/latest/sdk/ui/jetpack-compose/extending/)
5. Apply spring motion consistently without stacking React Native press
   animations around already animated controls. Preserve Expo's installed
   TextField workaround: it uses `MotionScheme.standard()` because Expressive
   overshoot could temporarily enlarge fields. Respect system animation
   settings and retain the original unread cadence.
   [Material motion guidance](https://m3.material.io/styles/motion/overview/how-it-works)

Android acceptance follows the planned TLS work: Expo offers **HTTPS and
TLS-wrapped TCP only**, with certificate/hostname verification and no cleartext
fallback. Plain HTTP/TCP remain for original clients. Framing, handshake,
tunnel paths and operations remain compatible; this introduces no API version.
Current cleartext development configuration must be reviewed during that
transition. Secure transport acceptance is still outstanding.

Acceptance must cover both secure transports against the Go server, and
unchanged original-client interoperability on plain listeners. Verify native
48dp touch targets, TalkBack, large fonts, light/dark colors, disabled states,
system animation settings, Back/sheet dismissal, actual software-keyboard
geometry, IME composition, UTF-16 selection, slash completion, draft retention,
unread clearing and long-transcript positioning. Measure layout and frame
performance on an emulator and physical device; inspect actual native targets,
not only their React Native wrappers.
[Compose accessibility defaults](https://developer.android.com/develop/ui/compose/accessibility/api-defaults)
