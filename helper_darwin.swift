import Cocoa
import Carbon

var im = "?"
if let source = TISCopyCurrentKeyboardInputSource()?.takeRetainedValue(),
   let name = TISGetInputSourceProperty(source, kTISPropertyLocalizedName) {
    im = Unmanaged<CFString>.fromOpaque(name).takeUnretainedValue() as String
}
let caps = NSEvent.modifierFlags.contains(.capsLock)
print("\(im)|\(caps)")
