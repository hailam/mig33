import SwiftUI

@MainActor
final class Mig33BrandModel: ObservableObject {
  @Published var animate = false { didSet { if animate != oldValue { invalidateAnimation() } } }
  @Published var settled = false { didSet { if settled != oldValue { invalidateAnimation() } } }
  @Published var dark = false
  @Published var ready = false
  @Published var attached = false { didSet { if !attached { invalidateAnimation() } } }
  @Published private(set) var completed = false
  private var animationEpoch: UInt64 = 0
  var onComplete: (() -> Void)?

  func beginAnimation() -> UInt64 {
    invalidateAnimation()
    return animationEpoch
  }

  func invalidateAnimation(_ epoch: UInt64? = nil) {
    if let epoch, epoch != animationEpoch { return }
    animationEpoch &+= 1
  }

  func isCurrentAnimation(_ epoch: UInt64) -> Bool {
    attached && ready && animate && !settled && !completed && epoch == animationEpoch
  }

  func complete(_ epoch: UInt64? = nil) {
    guard attached, ready, !completed else { return }
    if let epoch, epoch != animationEpoch { return }
    completed = true
    onComplete?()
  }

}

/// Each source edge corresponds to one analytic ellipse arc. Interpolating
/// their endpoints and control points opens the source notch gradually, with
/// no hole, font substitution, or sudden switch of path topology. At one the
/// result is the bounded fitted contour; at zero it is a smooth, complete lens.
private struct Mig33Part: Shape {
  let source: Path
  let sourceBounds: CGRect
  var expansion: CGFloat

  var animatableData: CGFloat {
    get { expansion }
    set { expansion = newValue }
  }

  func path(in rect: CGRect) -> Path {
    let bounds = sourceBounds
    let exact = source.applying(CGAffineTransform(translationX: -bounds.minX, y: -bounds.minY))
      .applying(CGAffineTransform(scaleX: rect.width / bounds.width, y: rect.height / bounds.height))
      .applying(CGAffineTransform(translationX: rect.minX, y: rect.minY))
    let amount = min(1, max(0, expansion))
    if amount == 0 { return Ellipse().path(in: rect) }
    if amount == 1 { return exact }
    typealias Curve = (start: CGPoint, first: CGPoint, second: CGPoint, end: CGPoint)
    var curves: [Curve] = []
    var current = CGPoint.zero
    func mix(_ start: CGPoint, _ end: CGPoint, _ fraction: CGFloat) -> CGPoint {
      CGPoint(x: start.x + (end.x - start.x) * fraction, y: start.y + (end.y - start.y) * fraction)
    }
    exact.forEach { element in
      switch element {
      case .move(let point): current = point
      case .line(let end):
        curves.append((current, mix(current, end, 1 / 3), mix(current, end, 2 / 3), end)); current = end
      case .curve(let end, let first, let second):
        curves.append((current, first, second, end)); current = end
      case .quadCurve(let end, let control):
        curves.append((current, mix(current, control, 2 / 3), mix(end, control, 2 / 3), end)); current = end
      case .closeSubpath: break
      }
    }
    guard let firstCurve = curves.first else { return exact }
    func at(_ curve: Curve, _ t: CGFloat) -> CGPoint {
      let a = mix(curve.start, curve.first, t), b = mix(curve.first, curve.second, t), c = mix(curve.second, curve.end, t)
      return mix(mix(a, b, t), mix(b, c, t), t)
    }
    let lengths = curves.map { curve in (1...12).reduce(CGFloat.zero) { length, step in
      let previous = at(curve, CGFloat(step - 1) / 12), next = at(curve, CGFloat(step) / 12)
      return length + hypot(next.x - previous.x, next.y - previous.y)
    } }
    let perimeter = lengths.reduce(0, +)
    let rx = rect.width / 2, ry = rect.height / 2
    let origin = atan2((firstCurve.start.y - rect.midY) / ry, (firstCurve.start.x - rect.midX) / rx)
    func ellipse(_ angle: CGFloat) -> CGPoint {
      CGPoint(x: rect.midX + rx * cos(angle), y: rect.midY + ry * sin(angle))
    }
    var result = Path(), distance: CGFloat = 0
    result.move(to: mix(ellipse(origin), firstCurve.start, amount))
    for index in curves.indices {
      let curve = curves[index]
      let a = origin + 2 * .pi * distance / perimeter
      distance += lengths[index]
      let b = origin + 2 * .pi * distance / perimeter
      let first = ellipse(a), last = ellipse(b)
      let tangent = 4 / 3 * tan((b - a) / 4)
      let firstControl = CGPoint(x: first.x - tangent * rx * sin(a), y: first.y + tangent * ry * cos(a))
      let lastControl = CGPoint(x: last.x + tangent * rx * sin(b), y: last.y - tangent * ry * cos(b))
      result.addCurve(to: mix(last, curve.end, amount),
        control1: mix(firstControl, curve.first, amount),
        control2: mix(lastControl, curve.second, amount))
    }
    result.closeSubpath()
    return result
  }
}

/// Both the glass mask and its ink use the shared canvas coordinates before
/// effect association. A union must see the actual moving contours, rather
/// than two origin-aligned masks moved by later rendering transforms.
private struct Mig33PlacedPart: Shape {
  let source: Path
  let sourceBounds: CGRect
  let isInk: Bool
  var center: CGPoint
  var scale: CGSize
  var expansion: CGFloat

  typealias PlacementData = AnimatablePair<AnimatablePair<CGFloat, CGFloat>,
    AnimatablePair<AnimatablePair<CGFloat, CGFloat>, CGFloat>>
  var animatableData: PlacementData {
    get { .init(.init(center.x, center.y), .init(.init(scale.width, scale.height), expansion)) }
    set {
      center = CGPoint(x: newValue.first.first, y: newValue.first.second)
      scale = CGSize(width: newValue.second.first.first, height: newValue.second.first.second)
      expansion = newValue.second.second
    }
  }

  func path(in rect: CGRect) -> Path {
    let canvasX = rect.width / 320, canvasY = rect.height / 182
    let width = sourceBounds.width * scale.width * canvasX
    let height = sourceBounds.height * scale.height * canvasY
    let placement = CGRect(x: rect.minX + center.x * canvasX - width / 2,
      y: rect.minY + center.y * canvasY - height / 2, width: width, height: height)
    if !isInk {
      return Mig33Part(source: source, sourceBounds: sourceBounds, expansion: expansion).path(in: placement)
    }
    return placing(source, in: rect)
  }

  func placing(_ path: Path, in rect: CGRect) -> Path {
    path.applying(CGAffineTransform(translationX: -sourceBounds.midX, y: -sourceBounds.midY))
      .applying(CGAffineTransform(scaleX: scale.width * rect.width / 320, y: scale.height * rect.height / 182))
      .applying(CGAffineTransform(translationX: rect.minX + center.x * rect.width / 320,
        y: rect.minY + center.y * rect.height / 182))
  }

  func place(_ point: CGPoint, in rect: CGRect) -> CGPoint {
    CGPoint(x: rect.minX + (center.x + (point.x - sourceBounds.midX) * scale.width) * rect.width / 320,
      y: rect.minY + (center.y + (point.y - sourceBounds.midY) * scale.height) * rect.height / 182)
  }
}

/// Contact is one explicit filled region, rendered by one real glass effect.
/// The bridge overlaps inside each body so boolean union removes internal rims.
@available(iOS 26.0, *)
private struct Mig33ContactShape: Shape {
  var main: Mig33PlacedPart
  var lobe: Mig33PlacedPart
  var waist: CGFloat
  let separated: Bool
  let secondary: Bool

  typealias ContactData = AnimatablePair<Mig33PlacedPart.PlacementData,
    AnimatablePair<Mig33PlacedPart.PlacementData, CGFloat>>
  var animatableData: ContactData {
    get { .init(main.animatableData, .init(lobe.animatableData, waist)) }
    set {
      main.animatableData = newValue.first
      lobe.animatableData = newValue.second.first
      waist = newValue.second.second
    }
  }

  func path(in rect: CGRect) -> Path {
    if separated { return (secondary ? lobe : main).path(in: rect) }
    if secondary { return Path() }
    let bodies = main.path(in: rect).union(lobe.path(in: rect))
    guard waist > 0 else { return bodies }
    let a = main.place(CGPoint(x: 194, y: 54), in: rect)
    let b = lobe.place(CGPoint(x: lobe.sourceBounds.minX + 6, y: lobe.sourceBounds.midY), in: rect)
    let length = hypot(b.x - a.x, b.y - a.y)
    guard length > 0.001 else { return bodies }
    let axis = CGPoint(x: (b.x - a.x) / length, y: (b.y - a.y) / length)
    let normal = CGPoint(x: -axis.y, y: axis.x)
    let middle = CGPoint(x: (a.x + b.x) / 2, y: (a.y + b.y) / 2)
    let unit = min(rect.width / 320, rect.height / 182)
    // Both roots and waist vanish continuously. A fixed root width would
    // leave finite bridge wings that disappear at the final pinch frame.
    let fraction = min(1, max(0, waist / 8))
    let endRadius = 12 * fraction * unit, neckRadius = 8 * fraction * fraction * unit
    func shifted(_ point: CGPoint, along: CGFloat = 0, across: CGFloat = 0) -> CGPoint {
      CGPoint(x: point.x + axis.x * along + normal.x * across,
        y: point.y + axis.y * along + normal.y * across)
    }
    let topA = shifted(a, across: endRadius), topB = shifted(b, across: endRadius)
    let topMiddle = shifted(middle, across: neckRadius)
    let bottomA = shifted(a, across: -endRadius), bottomB = shifted(b, across: -endRadius)
    let bottomMiddle = shifted(middle, across: -neckRadius)
    var bridge = Path()
    bridge.move(to: topA)
    bridge.addCurve(to: topMiddle, control1: shifted(topA, along: length * 0.2),
      control2: shifted(topMiddle, along: -length * 0.2))
    bridge.addCurve(to: topB, control1: shifted(topMiddle, along: length * 0.2),
      control2: shifted(topB, along: -length * 0.2))
    bridge.addLine(to: bottomB)
    bridge.addCurve(to: bottomMiddle, control1: shifted(bottomB, along: -length * 0.2),
      control2: shifted(bottomMiddle, along: length * 0.2))
    bridge.addCurve(to: bottomA, control1: shifted(bottomMiddle, along: -length * 0.2),
      control2: shifted(bottomA, along: length * 0.2))
    bridge.closeSubpath()
    return bodies.union(bridge)
  }
}

/// Feed continuous RGB/alpha values to the stable native effect; a boolean
/// cyan→orange descriptor replacement can transiently drop its tint.
@available(iOS 26.0, *)
private struct Mig33GlassPigment: AnimatableModifier {
  var shape: Mig33ContactShape
  let dark: Bool
  let orange: Bool
  var progress: Double
  var animatableData: AnimatablePair<Mig33ContactShape.ContactData, Double> {
    get { .init(shape.animatableData, progress) }
    set { shape.animatableData = newValue.first; progress = newValue.second }
  }
  func body(content: Content) -> some View {
    let amount = min(1, max(0, progress))
    let color = Color(red: orange ? amount : 0,
      green: 0.68 + (orange ? (0.43 - 0.68) * amount : 0),
      blue: 0.86 + (orange ? (0.08 - 0.86) * amount : 0))
    let initial = dark ? 0.36 : 0.28, final = dark ? 0.64 : 0.60
    return content.glassEffect(.regular.tint(color.opacity(initial + (final - initial) * amount)), in: shape)
  }
}

struct Mig33BrandScene: View {
  @ObservedObject var model: Mig33BrandModel
  @Environment(\.scenePhase) private var scenePhase

  var body: some View {
    Group {
      if #available(iOS 26.0, *) {
        Mig33GlassFormation(model: model)
      } else {
        Color.clear
          .onChange(of: model.ready) { ready in if ready { model.complete() } }
      }
    }
    .environment(\.colorScheme, model.dark ? .dark : .light)
    .allowsHitTesting(false)
    .accessibilityHidden(true)
    .onChange(of: scenePhase) { phase in
      if phase != .active && model.ready { model.settled = true }
    }
  }
}

@available(iOS 26.0, *)
private struct Mig33GlassFormation: View {
  @ObservedObject var model: Mig33BrandModel
  @Namespace private var glass
  @State private var stage = 0
  @State private var writingProgress: CGFloat = 0
  @State private var contourExpansion: CGFloat = 0
  @State private var pigmentStrength: Double = 0
  @State private var bridgeWaist: CGFloat = 0
  @State private var separateMaterials = false
  @State private var impact: CGFloat = 0
  @State private var writingCompleted = false
  @State private var pigmentCompleted = false
  private var request: String { "\(model.attached):\(model.ready):\(model.animate):\(model.settled)" }
  // A settled remount must draw its final pose on its first native render;
  // readiness coordinates visibility, never the geometry of that pose.
  private var displayStage: Int { model.settled ? 5 : stage }
  private var displayExpansion: CGFloat { model.settled ? 1 : contourExpansion }
  private var displayWriting: CGFloat { model.settled ? 1 : writingProgress }
  private var displayImpact: CGFloat { model.settled ? 0 : impact }

  var body: some View {
    GeometryReader { geometry in
      let scale = min(geometry.size.width / 320, geometry.size.height / 182)
      let main = Mig33PlacedPart(source: Mig33LogoPaths.cyan, sourceBounds: Mig33LogoPaths.cyanBounds, isInk: false,
        center: CGPoint(x: Mig33LogoPaths.cyanBounds.midX - 0.8 * displayImpact,
          y: Mig33LogoPaths.cyanBounds.midY + 0.3 * displayImpact),
        scale: CGSize(width: displayStage == 0 ? 0.38 : displayStage == 1 ? 1.06 : 1 + 0.016 * displayImpact,
          height: displayStage == 0 ? 0.56 : displayStage == 1 ? 0.88 : 1 + 0.018 * displayImpact), expansion: displayExpansion)
      let lobe = Mig33PlacedPart(source: Mig33LogoPaths.orange, sourceBounds: Mig33LogoPaths.orangeBounds, isInk: false,
        center: displayStage == 0
          ? CGPoint(x: Mig33LogoPaths.cyanBounds.midX, y: Mig33LogoPaths.cyanBounds.midY - 5)
          : displayStage == 1 ? CGPoint(x: 209, y: 61)
            : displayStage == 2 ? CGPoint(x: 244, y: 54.5)
              : displayStage == 3 ? CGPoint(x: 256.5, y: 48.5)
                : displayStage == 4 ? CGPoint(x: 239.5, y: 55.8)
                  : CGPoint(x: Mig33LogoPaths.orangeBounds.midX, y: Mig33LogoPaths.orangeBounds.midY),
        scale: CGSize(width: displayStage == 0 ? 0.4 : displayStage == 1 ? 0.62 : 1,
          height: displayStage == 0 ? 0.4 : displayStage == 1 ? 0.62 : 1), expansion: displayExpansion)
      ZStack(alignment: .topLeading) {
        // The closest rebound pose keeps a2.236px contour gap. This smaller
        // merge distance preserves separation while both bodies yield.
        GlassEffectContainer(spacing: 1) {
          ZStack(alignment: .topLeading) {
            bubble(main: main, lobe: lobe, secondary: false)
            bubble(main: main, lobe: lobe, secondary: true)
          }.frame(width: 320, height: 182)
        }
        // Ink identities never participate in the compound-to-separated
        // material transfer. Each reveal keeps its own animation clock.
        ink(body: main, part: .mig)
        ink(body: lobe, part: .number)
      }
      .frame(width: 320, height: 182, alignment: .topLeading)
      .scaleEffect(scale, anchor: .topLeading)
      .frame(width: geometry.size.width, height: geometry.size.height, alignment: .topLeading)
    }
    .task(id: request) {
      guard model.attached, model.ready, !model.completed else { return }
      if model.settled {
        var transaction = Transaction(); transaction.disablesAnimations = true
        withTransaction(transaction) {
          stage = 5; impact = 0; contourExpansion = 1; bridgeWaist = 0; separateMaterials = true
          pigmentStrength = 1; writingProgress = 1
        }
        model.complete(); return
      }
      guard model.animate else { return }
      let epoch = model.beginAnimation()
      writingCompleted = false; pigmentCompleted = false
      do {
        withAnimation(.spring(duration: 0.2, bounce: 0.04)) { stage = 1 }
        try await Task.sleep(for: .milliseconds(200))
        try Task.checkCancellation()
        // Stretch one explicit native glass silhouette. The authored connector
        // overlaps inside both bodies, leaving no internal material seam.
        withAnimation(.easeInOut(duration: 0.36)) { stage = 2; contourExpansion = 1; bridgeWaist = 8 }
        try await Task.sleep(for: .milliseconds(180))
        try Task.checkCancellation()
        guard model.isCurrentAnimation(epoch) else { return }
        // Writing overlaps the stretch, pinch and rebound. Both its reveal
        // and placement interpolate in native source coordinates.
        withAnimation(.linear(duration: 1.2), completionCriteria: .removed) {
          writingProgress = 1
        } completion: {
          guard model.isCurrentAnimation(epoch) else { return }
          writingCompleted = true
          if pigmentCompleted { model.complete(epoch) }
        }
        try await Task.sleep(for: .milliseconds(180))
        try Task.checkCancellation()
        // This is a real positional overshoot,14px outward and6px upward,
        // not a scale bounce or a second shape entering from below.
        withAnimation(.easeOut(duration: 0.28)) { stage = 3; bridgeWaist = 0 }
        try await Task.sleep(for: .milliseconds(280))
        try Task.checkCancellation()
        guard model.isCurrentAnimation(epoch) else { return }
        // The compound mask now has two disjoint islands,15.86px apart.
        // Transfer the same low-cyan material to individual stable effect
        // hosts before the spring and long before pigment changes.
        var transaction = Transaction(); transaction.disablesAnimations = true
        withTransaction(transaction) { separateMaterials = true }
        // Return slightly past rest, with a delayed yielding pulse in the
        // larger body. The masks stay disjoint even at their closest pose.
        withAnimation(.easeInOut(duration: 0.16).delay(0.08)) { impact = 1 }
        withAnimation(.timingCurve(0.3, 0, 0.2, 1, duration: 0.24), completionCriteria: .removed) {
          stage = 4
        } completion: {
          guard model.isCurrentAnimation(epoch) else { return }
          // Let both bodies settle together. Pigment waits for the actual
          // spring tail rather than an estimated duration.
          withAnimation(.spring(duration: 0.26, bounce: 0.16), completionCriteria: .removed) {
            stage = 5; impact = 0
          } completion: {
            guard model.isCurrentAnimation(epoch) else { return }
            withAnimation(.easeInOut(duration: 0.22), completionCriteria: .removed) {
              pigmentStrength = 1
            } completion: {
              guard model.isCurrentAnimation(epoch) else { return }
              pigmentCompleted = true
              if writingCompleted { model.complete(epoch) }
            }
          }
        }
      } catch { model.invalidateAnimation(epoch) }
    }
    .onDisappear { model.invalidateAnimation() }
  }

  private func bubble(main: Mig33PlacedPart, lobe: Mig33PlacedPart, secondary: Bool) -> some View {
    // Only the material lives inside this effect host. Revealed ink is a
    // stable sibling, independent of an initially empty secondary mask.
    let mask = Mig33ContactShape(main: main, lobe: lobe, waist: model.settled ? 0 : bridgeWaist,
      separated: model.settled || separateMaterials, secondary: secondary)
    return Color.clear
      .frame(width: 320, height: 182)
      .modifier(Mig33GlassPigment(shape: mask, dark: model.dark, orange: secondary,
        progress: model.settled ? 1 : pigmentStrength))
      .glassEffectID(secondary ? "33" : "mig", in: glass)
      .glassEffectTransition(.identity)
  }

  private func ink(body: Mig33PlacedPart, part: Mig33LogoWritingMask.Part) -> some View {
    let bounds = body.sourceBounds
    let sourceBody = Mig33PlacedPart(source: body.source, sourceBounds: bounds, isInk: false,
      center: CGPoint(x: bounds.midX, y: bounds.midY), scale: CGSize(width: 1, height: 1),
      expansion: body.expansion)
    return Mig33LogoWritingMask(part: part, progress: displayWriting)
      .fill(.white, style: FillStyle(eoFill: true))
      .frame(width: 320, height: 182)
      // Native clipping preserves an empty reveal. Path.intersection with
      // an empty left operand can instead return the entire body on SwiftUI.
      .clipShape(sourceBody)
      // Reveal, contour clipping, scale and translation have independent
      // animatable values. A new geometry transaction must not retime the
      // 1200ms writing progress as a member of the same animatable vector.
      .scaleEffect(x: body.scale.width, y: body.scale.height,
        anchor: UnitPoint(x: bounds.midX / 320, y: bounds.midY / 182))
      .offset(x: body.center.x - bounds.midX, y: body.center.y - bounds.midY)
      .shadow(color: .black.opacity(model.dark ? 0.28 : 0.12), radius: 0.7, y: 0.4)
  }
}
