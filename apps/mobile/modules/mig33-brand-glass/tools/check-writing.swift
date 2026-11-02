import Foundation
import SwiftUI

/// Offline acceptance check for the production writing mask against the
/// unchanged source-filled glyphs. No simulator, app host or image assets.
@main
struct WritingCoverageCheck {
  private static let scale = 8
  private static let width = 320 * scale, height = 182 * scale
  private static let canvas = CGRect(x: 0, y: 0, width: 320, height: 182)

  private static func raster(_ path: Path) -> [UInt8] {
    var pixels = [UInt8](repeating: 0, count: width * height)
    pixels.withUnsafeMutableBytes { bytes in
      let context = CGContext(data: bytes.baseAddress, width: width, height: height, bitsPerComponent: 8,
        bytesPerRow: width, space: CGColorSpaceCreateDeviceGray(), bitmapInfo: 0)!
      context.scaleBy(x: CGFloat(scale), y: CGFloat(scale))
      context.setShouldAntialias(false)
      context.addPath(path.cgPath)
      context.setFillColor(gray: 1, alpha: 1)
      context.drawPath(using: .eoFill)
    }
    return pixels
  }

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

  private static func joined(_ paths: [Path]) -> Path {
    var result = Path()
    paths.forEach { result.addPath($0) }
    return result
  }

  static func main() {
    let mig = contours(Mig33LogoPaths.mig), number = contours(Mig33LogoPaths.number)
    precondition(mig.count == 5 && number.count == 2, "Source glyph grouping changed; review writing order")
    var failures = 0
    // SwiftUI currently returns the right-hand shape for an empty left-hand
    // Path.intersection. The production guard must keep blank ink blank,
    // including before the number strokes begin and after affine placement.
    let lobe = Mig33LogoPaths.orange
    for (name, ink, contour) in [
      ("empty ink", Path(), lobe),
      ("empty clip", Mig33LogoPaths.number, Path()),
      ("two empty paths", Path(), Path()),
      ("placed empty ink", Path().applying(CGAffineTransform(translationX: 14, y: -6)), lobe),
    ] {
      let result = Mig33LogoWritingMask.clippedInk(ink, to: contour)
      let count = raster(result).filter { $0 > 0 }.count
      print("\(name): filledSamples=\(count), empty=\(result.isEmpty)")
      if count != 0 || !result.isEmpty { failures += 1 }
    }
    func compare(_ name: String, part: Mig33LogoWritingMask.Part, progress: CGFloat, expected: Path) {
      let source = raster(expected)
      let actual = raster(Mig33LogoWritingMask(part: part, progress: progress).path(in: canvas))
      var missing = 0, leaked = 0
      for index in source.indices {
        if source[index] > 0 && actual[index] == 0 { missing += 1 }
        if source[index] == 0 && actual[index] > 0 { leaked += 1 }
      }
      print("\(name) progress=\(progress): missing=\(missing), leaked=\(leaked)")
      if missing != 0 || leaked != 0 { failures += 1 }
    }

    // Test immediately before each promised completion, not only progress 1.
    // The expected prefix also catches early neighboring-letter exposure.
    for progress: CGFloat in [0.1, 0.36, 0.43, 0.72, 0.759999, 0.76] {
      compare("number before its first stroke", part: .number, progress: progress, expected: Path())
    }
    for (name, part, end, expected) in [
      ("m", Mig33LogoWritingMask.Part.mig, 0.36, mig[1]),
      ("i stem", .mig, 0.43, joined([mig[1], mig[2]])),
      ("g and counter", .mig, 0.72, joined([mig[1], mig[2], mig[3], mig[4]])),
      ("i dot", .mig, 0.76, Mig33LogoPaths.mig),
      ("first 3", .number, 0.87, number[0]),
      ("second 3", .number, 0.98, Mig33LogoPaths.number),
    ] {
      compare(name, part: part, progress: end - 0.000001, expected: expected)
    }
    for (name, part, expected) in [
      ("mig", Mig33LogoWritingMask.Part.mig, Mig33LogoPaths.mig),
      ("number", .number, Mig33LogoPaths.number),
    ] {
      compare("\(name) blank", part: part, progress: 0, expected: Path())
      compare("\(name) almost final", part: part, progress: 0.999999, expected: expected)
      compare("\(name) final", part: part, progress: 1, expected: expected)

      var previous = raster(Path()), edgeChanges = 0, interiorLoss = 0
      for step in 0...64 {
        let actual = raster(Mig33LogoWritingMask(part: part, progress: CGFloat(step) / 64).path(in: canvas))
        for index in actual.indices where previous[index] > 0 && actual[index] == 0 {
          let x = index % width, y = index / width
          // Boolean curve flattening can change a boundary sample by 1/8px.
          // Losing an already-solid 3×3 neighborhood is a real ink reversal.
          let solid = x > 0 && x < width - 1 && y > 0 && y < height - 1
            && [-width - 1, -width, -width + 1, -1, 1, width - 1, width, width + 1]
              .allSatisfy { previous[index + $0] > 0 }
          if solid { interiorLoss += 1 } else { edgeChanges += 1 }
        }
        previous = actual
      }
      print("\(name) 65-step reveal: interiorLoss=\(interiorLoss), quantizedEdgeChanges=\(edgeChanges)")
      if interiorLoss != 0 { failures += 1 }
    }
    print(failures == 0 ? "Writing coverage PASS" : "Writing coverage FAILED: \(failures) checks")
    exit(failures == 0 ? 0 : 1)
  }
}
