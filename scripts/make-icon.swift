// Run from the repository root. The app bundle and the window share one mark.
import AppKit
import Foundation

let source = URL(fileURLWithPath: "internal/contexts/transfer/presentation/art/skiff.svg")
guard let mark = NSImage(contentsOf: source),
      let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: 1024, pixelsHigh: 1024,
          bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB,
          bytesPerRow: 0, bitsPerPixel: 0),
      let context = NSGraphicsContext(bitmapImageRep: bitmap) else {
    fatalError("Unable to rasterize the Skiff mark")
}
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = context
mark.draw(in: NSRect(x: 0, y: 0, width: 1024, height: 1024))
NSGraphicsContext.restoreGraphicsState()
guard let data = bitmap.representation(using: .png, properties: [:]) else {
    fatalError("Unable to encode the app icon")
}
try data.write(to: URL(fileURLWithPath: "resources/icon.png"), options: .atomic)
