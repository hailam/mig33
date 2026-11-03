import SwiftUI

/// A pen-stroke reveal mask in the logo's original 320×182 coordinate space.
/// Use in source coordinates, or place with the owning body during motion.
/// These authored centerlines describe writing motion, not artwork:
/// every sweep is intersected with its own unchanged source glyph, including
/// the g counter, so a broad pen cannot reveal a neighboring letter early.
@available(iOS 17.0, macOS 14.0, *)
struct Mig33LogoWritingMask: Shape {
  enum Part { case mig, number }

  let part: Part
  var progress: CGFloat

  var animatableData: CGFloat {
    get { progress }
    set { progress = newValue }
  }

  func path(in rect: CGRect) -> Path {
    let amount = min(1, max(0, progress))
    var result = Path()
    for stroke in part == .mig ? Artwork.migStrokes : Artwork.numberStrokes {
      let fraction = min(1, max(0, (amount - stroke.start) / (stroke.end - stroke.start)))
      guard fraction > 0 else { continue }
      result.addPath(stroke.reveal(fraction))
    }
    // Preserve a truly empty reveal before the first scheduled stroke.
    // In particular, never intersect it with a body: SwiftUI's empty-left
    // Boolean operation can return that entire body instead of empty ink.
    guard !result.isEmpty else { return Path() }
    return result.applying(CGAffineTransform(scaleX: rect.width / 320, y: rect.height / 182))
      .applying(CGAffineTransform(translationX: rect.minX, y: rect.minY))
  }

  static func clippedInk(_ ink: Path, to contour: Path) -> Path {
    guard !ink.isEmpty, !contour.isEmpty else { return Path() }
    return ink.intersection(contour)
  }

  private struct Stroke {
    let glyph: Path
    let centerline: Path
    let width: CGFloat
    let start: CGFloat
    let end: CGFloat
    let dab: Bool
    private let fullSweep: Path

    init(glyph: Path, centerline: Path, width: CGFloat, start: CGFloat, end: CGFloat, dab: Bool = false) {
      self.glyph = glyph.normalized(eoFill: true)
      self.centerline = centerline; self.width = width; self.start = start; self.end = end; self.dab = dab
      self.fullSweep = Mig33LogoWritingMask.clippedInk(
        Self.sweep(centerline, width: width, dab: dab, fraction: 1).normalized(eoFill: false), to: self.glyph)
    }

    func reveal(_ fraction: CGFloat) -> Path {
      // Completed sweeps are cached, not replaced with an unverified full
      // fill. Only the actively written glyph needs a Boolean path operation.
      if fraction >= 1 { return fullSweep }
      // A pen retraces stems with nonzero winding; even-odd on its overlapping
      // stroke outline would erase earlier ink. Only the glyph uses even-odd.
      return Mig33LogoWritingMask.clippedInk(
        Self.sweep(centerline, width: width, dab: dab, fraction: fraction).normalized(eoFill: false), to: glyph)
    }

    private static func sweep(_ centerline: Path, width: CGFloat, dab: Bool, fraction: CGFloat) -> Path {
      if dab {
        // The i dot is a stationary pen dab, written after the g. Growing the
        // nib locally avoids either a fly-in or an instant full-size dot.
        let diameter = width * fraction
        return Path(ellipseIn: CGRect(x: 134.1 - diameter / 2, y: 54.5 - diameter / 2,
          width: diameter, height: diameter))
      }
      return centerline.trimmedPath(from: 0, to: fraction)
        .strokedPath(StrokeStyle(lineWidth: width, lineCap: .round, lineJoin: .round))
    }
  }

  private enum Artwork {
    private static let mig = contours(Mig33LogoPaths.mig)
    private static let number = contours(Mig33LogoPaths.number)

    // Global order: m, i stem, g, i dot, first 3, second 3. The last 2% is a
    // completed-ink hold, not a final full-fill substitution. Pen widths have
    // coverage margin inside each glyph; the exact source contour clips them.
    static let migStrokes: [Stroke] = [
      Stroke(glyph: mig[1], centerline: Path { path in
        path.move(to: CGPoint(x: 63, y: 68))
        path.addQuadCurve(to: CGPoint(x: 69, y: 76), control: CGPoint(x: 69, y: 68))
        path.addLine(to: CGPoint(x: 69, y: 100))
        path.addLine(to: CGPoint(x: 69, y: 81))
        path.addCurve(to: CGPoint(x: 93, y: 81), control1: CGPoint(x: 69, y: 64), control2: CGPoint(x: 93, y: 64))
        path.addLine(to: CGPoint(x: 93, y: 100))
        path.addLine(to: CGPoint(x: 93, y: 81))
        path.addCurve(to: CGPoint(x: 117, y: 81), control1: CGPoint(x: 93, y: 65), control2: CGPoint(x: 117, y: 64))
        path.addLine(to: CGPoint(x: 117, y: 101))
        path.addCurve(to: CGPoint(x: 131, y: 117), control1: CGPoint(x: 117, y: 111), control2: CGPoint(x: 122, y: 115))
      }, width: 18, start: 0, end: 0.36),
      Stroke(glyph: mig[2], centerline: Path { path in
        path.move(to: CGPoint(x: 134, y: 70)); path.addLine(to: CGPoint(x: 134, y: 100))
      }, width: 15.5, start: 0.36, end: 0.43),
      Stroke(glyph: joined([mig[3], mig[4]]), centerline: Path { path in
        path.move(to: CGPoint(x: 181, y: 75))
        path.addCurve(to: CGPoint(x: 151, y: 85), control1: CGPoint(x: 172, y: 65), control2: CGPoint(x: 151, y: 65))
        path.addCurve(to: CGPoint(x: 184, y: 85), control1: CGPoint(x: 151, y: 106), control2: CGPoint(x: 184, y: 107))
        path.addLine(to: CGPoint(x: 184, y: 70))
        path.addLine(to: CGPoint(x: 184, y: 109))
        path.addCurve(to: CGPoint(x: 147, y: 124), control1: CGPoint(x: 184, y: 130), control2: CGPoint(x: 169, y: 134))
      }, width: 18, start: 0.43, end: 0.72),
      Stroke(glyph: mig[0], centerline: Path(), width: 16, start: 0.72, end: 0.76, dab: true),
    ]

    static let numberStrokes: [Stroke] = [
      Stroke(glyph: number[0], centerline: three(offset: 0), width: 11, start: 0.76, end: 0.87),
      Stroke(glyph: number[1], centerline: three(offset: 26), width: 11, start: 0.87, end: 0.98),
    ]

    private static func three(offset: CGFloat) -> Path {
      Path { path in
        func point(_ x: CGFloat, _ y: CGFloat) -> CGPoint { CGPoint(x: x + offset, y: y) }
        path.move(to: point(221, 43))
        path.addCurve(to: point(236, 45), control1: point(225, 36), control2: point(236, 37))
        path.addCurve(to: point(229, 52), control1: point(236, 49), control2: point(232, 50))
        path.addCurve(to: point(237, 61), control1: point(235, 52), control2: point(240, 56))
        path.addCurve(to: point(220, 62), control1: point(234, 72), control2: point(225, 69))
      }
    }

    private static func joined(_ paths: [Path]) -> Path {
      var result = Path(); for path in paths { result.addPath(path) }; return result
    }

    /// Preserve source subpaths verbatim. Their order is documented in
    /// Mig33LogoPaths: dot, m, i stem, g outer/counter, then the two 3 contours.
    private static func contours(_ source: Path) -> [Path] {
      var result: [Path] = [], current = Path()
      source.forEach { element in
        switch element {
        case .move(let point): current = Path(); current.move(to: point)
        case .line(let point): current.addLine(to: point)
        case .quadCurve(let end, let control): current.addQuadCurve(to: end, control: control)
        case .curve(let end, let first, let second): current.addCurve(to: end, control1: first, control2: second)
        case .closeSubpath: current.closeSubpath(); result.append(current)
        }
      }
      return result
    }
  }
}
