import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

// This draws the native animation's initial geometry; it never reads or edits
// a bitmap. See the generated
// assets' launch-mark.md for the deliberate static-material limitation.
@main
struct LaunchPosterRenderer {
    static func main() throws {
        guard CommandLine.arguments.count == 2 else {
            throw NSError(domain: "LaunchPosterRenderer", code: 1,
                userInfo: [NSLocalizedDescriptionKey: "Usage: render-launch-posters OUTPUT_DIRECTORY"])
        }
        let output = URL(fileURLWithPath: CommandLine.arguments[1], isDirectory: true)
        let body = CGMutablePath()
        // The native animation starts with a clean lens, then morphs into the
        // historical contour. The second body is entirely inside this ellipse.
        body.addEllipse(in: CGRect(x: 98.76, y: 52.58, width: 74.48, height: 77.84))
        for scale in 1...3 {
            let width = 320 * scale
            let height = 182 * scale
            guard let colorSpace = CGColorSpace(name: CGColorSpace.sRGB),
                let context = CGContext(data: nil, width: width, height: height, bitsPerComponent: 8,
                    bytesPerRow: width * 4, space: colorSpace,
                    bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue) else {
                throw NSError(domain: "LaunchPosterRenderer", code: 2)
            }
            context.translateBy(x: 0, y: CGFloat(height))
            context.scaleBy(x: CGFloat(scale), y: -CGFloat(scale))
            context.setShouldAntialias(true)
            context.addPath(body)
            context.setFillColor(red: 0, green: 0.68, blue: 0.86, alpha: 0.28)
            context.fillPath()
            let suffix = scale == 1 ? "" : "@\(scale)x"
            let file = output.appendingPathComponent("launch-mark\(suffix).png")
            guard let image = context.makeImage(),
                let destination = CGImageDestinationCreateWithURL(file as CFURL, UTType.png.identifier as CFString, 1, nil) else {
                throw NSError(domain: "LaunchPosterRenderer", code: 3)
            }
            CGImageDestinationAddImage(destination, image, nil)
            guard CGImageDestinationFinalize(destination) else {
                throw NSError(domain: "LaunchPosterRenderer", code: 4)
            }
            print(file.path)
        }
    }
}
