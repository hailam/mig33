# Static launch pose

`launch-mark.png` and its `@2x`/`@3x` variants are generated from the native brand
animation's initial geometric pose. No existing bitmap is edited. The underlying
historical mark and contour provenance are documented
in `mig33-logo.md` and the native module's `PROVENANCE.md`.

All scales use the native formation's initial ellipse on its 320×182 canvas:
`x=98.76, y=52.58, width=74.48, height=77.84`. The smaller second body is fully
inside this silhouette. The static fill is cyan RGB `(0,0.68,0.86)`, alpha 0.28
in both appearances. There is no lettering. The transparent
outputs are 320×182, 640×364, and 960×546. Each scale is drawn directly from the
geometry for the 176×100.1 launch region. Both the raw app bundle and React Native
receive explicit 1x, 2x, and 3x renditions. The native shape
subsequently morphs from this lens into the historical source-derived contour.

This is a static geometric/color approximation for the OS launch storyboard;
it is not a rasterized native Liquid Glass material. The in-app formation uses
the real native material after its first frame is ready. Visual acceptance must
check the transition between them rather than claim identical optical output.

The launch storyboard uses raw `Mig33LaunchMark*.png` bundle resources and UIKit's
`systemBackgroundColor`. Controlled iOS 27 simulator probes showed that catalog
image pixels were absent from the OS launch snapshot even while their view and
constraints rendered correctly. Raw resources were visible during the system's
opening animation in both appearances. The preparing screen uses the same
white/black base and static image; native dark glass retains its own 0.36 tint.

Regenerate on macOS from the repository root:

```sh
mkdir -p data/launch-plugin
xcrun swiftc -parse-as-library -module-cache-path data/launch-plugin/swift-cache \
  tools/mobile/render_launch_posters.swift \
  -o data/launch-plugin/render-launch-posters
data/launch-plugin/render-launch-posters apps/mobile/assets
```
