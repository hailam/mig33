# Native brand formation

This local Expo module renders the existing mig33 mark with native SwiftUI
Liquid Glass. It adds no package dependency. iOS 26 or later uses custom
`Shape` paths in one `GlassEffectContainer` and namespace; the React wrapper
uses the unchanged static mark on other systems and when Reduce Motion,
Reduce Transparency, or Increase Contrast is enabled.

The paths in `ios/Mig33LogoPaths.swift` derive from the existing
[`mig33-logo.png`](../../assets/mig33-logo.png), SHA-256
`5ca22dc09c3a6ad3d6894f4c55145c999535c76087d6a6ab32021f4bd80cbbb7`.
Its [asset provenance](../../assets/mig33-logo.md) distinguishes this secondary
320×182 historical copy from the publisher's smaller JAR image. These are
traced raster contours, **not recovered publisher vector artwork**.

`tools/extract-contours.swift` reads the original through AppKit without
changing it. It separates cyan, orange, and white pixels, fills enclosed
lettering holes in the two body masks, and traces their pixel boundaries.
The generator removes collinear vertices, conditions raster-frequency noise,
and fits closed cubic Bezier contours. Body conditioning uses a nine-sample
kernel with displacement capped at 0.75 source pixels; lettering retains its
five-sample kernel and 0.35px cap. It validates each result against the untouched source polygon
in both directions: curves are flattened with 0.01px chord tolerance, both
boundaries are sampled at no more than 0.05px intervals, and the sampled maximum
must remain within 0.95px for the two bodies and 0.45px for lettering. The
flattening and sampling budget keeps total deviation approximately within
1 source pixel for bodies and 0.5px for lettering. Current sampled maxima are
0.93664px (cyan, 14 cubics), 0.92361px (orange, 5 cubics), 0.44952px (`mig`),
and 0.44854px (`33`). The wider body budget removes raster steps that the
native glass rim made conspicuous; the fitted glyph paths remain byte-identical.
The five `mig` and two `33` contours retain their winding and source counters;
even-odd fill preserves the holes. It does not substitute a system font,
redraw the mark by eye, or claim a recovered vector master.

Fitted controls must advance monotonically along each curve's chord; an
otherwise distance-correct reversing handle is rejected and subdivided, so it
cannot introduce a tiny loop. The independent topology checker flattens curves
to 0.003px, rejects self/inter-contour crossings and escaped lettering, and
reports winding and containment. It confirms all nine contours remain simple
and the `g` counter remains inside its parent.

Regenerate from the repository root on macOS:

```sh
xcrun swift -module-cache-path /private/tmp/mig33-logo-module-cache \
  apps/mobile/modules/mig33-brand-glass/tools/extract-contours.swift \
  apps/mobile/assets/mig33-logo.png \
  > apps/mobile/modules/mig33-brand-glass/ios/Mig33LogoPaths.swift
mise exec -- node apps/mobile/modules/mig33-brand-glass/tools/check-contours.mjs \
  apps/mobile/modules/mig33-brand-glass/ios/Mig33LogoPaths.swift
```

The body shapes receive the actual glass material. They begin as smooth
ellipses: each fitted curve corresponds to an ellipse arc, and the native
animatable shape interpolates its endpoints and control points into the
source-derived contour. During contact one explicit filled-region `Path.union`
combines both placed bodies and a narrowing cubic bridge, and one genuine
glass effect renders that mask. This avoids relying on `glassEffectUnion`,
which concealed the moving lobe in the recorded native candidates. The smaller
lobe forms an attached upper-right shoulder over 200ms. From 200–560ms the
cyan lens stretches to `(244,54.5)` while its notch opens into the fitted
contour and the bridge develops. From 560–840ms the lobe travels to
`(256.5,48.5)`, a positional overshoot 14px outward and 6px upward from its
resting center `(242.5,54.5)`. Both bridge roots and waist shrink continuously;
the waist narrows faster, and the additional filled region tends to zero.
At the disjoint overshoot, whose measured contour gap is 15.86487px, the mask
transfers to two equivalent individual effects in a transaction with animations
disabled. Both retain identical low-cyan material. Effect hosts stay mounted.
The longer stretch and pinch add 240ms so the connecting neck is easier to see.

The lobe returns over 240ms to `(239.5,55.8)`, slightly inward past its resting
position. After the first 80ms of that return, the main body yields left 0.8px
and down 0.3px while expanding 1.6% horizontally and 1.8% vertically. Both
reach this response pose together and then spring back to their exact resting
geometry with duration 0.26 and bounce 0.16. A 201-position contour sweep,
including the maximum main-body pulse at every return position, measured a
minimum 2.23628px gap; constant container spacing is 1px, so the bodies remain
separate throughout the response. The normal resting gap remains 5.43888px.
The final spring's actual `.removed` completion starts the 220ms color phase, rather than
starting color at an estimated end time while the spring tail is still moving.
Both lobes remain the same low-tint cyan throughout the geometry; only after settling does
the smaller lobe turn orange and tint strength rise from 0.28 to 0.60 in light
mode, or 0.36 to 0.64 in dark mode. An animatable modifier owns both geometry
and continuous RGB/alpha interpolation; it does not switch a Boolean tint
descriptor at the start of coloring. Writing overlaps the body motion: it
starts 380ms into formation, halfway through the stretch, and runs for 1200ms.
`Mig33LogoWritingMask` writes the exact white artwork in order: `m`, `i` stem,
`g`, `i` dot, then both `3` glyphs. With the authored schedule, the first `3`
begins at approximately 1292ms and the second completes at 1556ms, during
the return and settling portion rather than after the color phase. Authored open centerlines reveal each glyph
under its own unchanged source contour, preserving the `g` counter and keeping
adjacent letters hidden. Writing is rendered by two stable ink siblings outside
the glass material hosts. Each source-space reveal owns only its writing
progress; a native clip shape, scale, and offset independently follow its
bubble's current contour and placement. Combining placement and reveal in one
animatable vector had allowed a new, shorter geometry transaction to accelerate
the number strokes. Separate animation owners preserve the 1200ms reveal while
the bubbles move. Native clipping also avoids a confirmed SwiftUI Boolean edge
case: `Path().intersection(body)` returned the entire body, producing a white
bubble before any scheduled ink. Empty reveals now remain empty, and remaining
glyph Boolean operations explicitly guard empty operands. The mask affects ink
only. Its completed sweeps cover
the exact source fill without a final full-fill substitution; an 8× raster
check found no missing or leaked samples just before each stroke completes.
An offline 4× check of 254 moving poses found zero outside-body pixels and
zero clipped ink pixels; comparison with an independent affine expression
found no interior discrepancy and 21 quarter-pixel boundary differences from
floating-point evaluation order. Actual writing and pigment completions form
a guarded two-way barrier: the decorative sequence completes only after both,
with no fixed total timer standing in for either native animation.
Individual effect identities
stay constant and the `.identity` glass transition prevents an additional
inferred entrance. The same native
glass remains mounted at rest: there is no late tint surge, bitmap overlay,
gradient replacement, or retirement transition. The host and ancestors remain
at opacity one. A settled mount derives its final pose immediately, without
briefly showing the seed while waiting for readiness. There is no looping logo,
progress claim, or readiness delay.

Apple documents [custom glass effects](https://developer.apple.com/documentation/swiftui/applying-liquid-glass-to-custom-views),
[filled-region path union](https://developer.apple.com/documentation/swiftui/path/union(_:eofill:)),
[trimmed shapes](https://developer.apple.com/documentation/swiftui/shape/trim(from:to:)),
and [spring motion](https://developer.apple.com/videos/play/wwdc2023/10158/).
The disjoint material transfer and tint interpolation require native
frame review: vector equality and API checks alone cannot prove optical
continuity. The writing is an original sequence inspired by native welcome
animations, not a claim to reproduce Apple's unpublished choreography.

The native launch storyboard and app preparation view use the same static
[first-pose artwork](../../assets/launch-mark.md) against the system light/dark
background. The image approximates the initial geometry and color; it is not a
rasterized Liquid Glass effect. The live material takes over after native
readiness, and recording review must assess that optical handoff.

The native host ignores touches and exposes no controls. Readiness requires
an attached window, positive bounds, and two display-link callbacks after that
layout, giving the first native display cycle time to commit before uncovering
it. Detach invalidates this observer. UIKit containment and the React AppState
listener settle an interrupted episode. Accessibility changes and `settled`
also end it immediately. A mounted instance emits `onReady` and `onComplete`
at most once. Completion callbacks come from SwiftUI's native animation
completion, guarded by the current animation epoch and attachment; the authored
intermediate overlaps use cancellable task waits. The parent controls
application readiness and allows the login form to appear in parallel.

Native registration and Swift/TypeScript compilation can be checked without
running a simulator. The actual glass merge, pigment development, settled
material, and scene integration must be evaluated through native recording
review; compilation alone does not establish visual acceptance. The mobile
README records completed runs and separates automated checks from visual review.

## Writing-mask verification

The centerlines in `ios/Mig33LogoWriting.swift` are authored reveal trajectories,
not replacement lettering or recovered publisher vectors. The existing filled
glyphs remain unchanged. Each open, rounded pen stroke is clipped to its own
glyph; the `g` retains its even-odd counter. The 1200ms sequence writes `m`, the
`i` stem, `g`, a stationary `i`-dot dab, and the two `3`s. Completed pen sweeps
are cached; there is no final full-fill shortcut.

`tools/check-writing.swift` compares the production mask against the original
glyph paths on an 8× binary raster. It requires zero missing or leaked samples
just before each glyph completes and at global progress `0.999999` and `1`,
and checks the initially empty mask. It also verifies the production empty-
operand guard, including transformed empty ink, and requires the entire number
mask to remain blank through progress0.76. Prefix comparisons catch neighboring
letters appearing early. A 65-step sweep rejects loss inside previously solid
ink. It separately reports boundary quantization from Boolean path flattening:
the current masks have 33 changed boundary samples for `mig` and 3 for `33`,
all at the 1/8-source-pixel sampling edge, with zero interior loss. Final
coverage and isolation checks have zero discrepancies. These geometry checks
complement native recording review; they do not prove timing or material quality.

Run from the repository root on macOS, without building or launching the app:

```sh
xcrun swiftc -O -module-cache-path /private/tmp/mig33-logo-module-cache \
  apps/mobile/modules/mig33-brand-glass/ios/Mig33LogoPaths.swift \
  apps/mobile/modules/mig33-brand-glass/ios/Mig33LogoWriting.swift \
  apps/mobile/modules/mig33-brand-glass/tools/check-writing.swift \
  -o /private/tmp/mig33-check-writing
/private/tmp/mig33-check-writing
```
