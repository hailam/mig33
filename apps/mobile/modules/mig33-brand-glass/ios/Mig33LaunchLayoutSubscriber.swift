import ExpoModulesCore
import UIKit

/// Expo extracts the launch controller's view and installs it as an RN loading
/// view. That detached controller's safe-area guide can remain at zero even
/// after the view reaches a window. Keep only our image aligned with the real
/// window, while Expo continues to own the cover's visibility and lifetime.
public final class Mig33LaunchLayoutSubscriber: ExpoAppDelegateSubscriber {
  public func customizeRootView(_ rootView: UIView) {
    // All subscribers finish synchronously before the next main-queue turn.
    // This works regardless of our position relative to ExpoSplashScreen.
    DispatchQueue.main.async { [weak rootView] in
      guard let rootView else { return }
      Mig33LaunchLayoutObserver.install(in: rootView)
    }
  }
}

private final class Mig33LaunchLayoutObserver: UIView {
  private weak var imageView: UIImageView?
  private var storyboardTop: NSLayoutConstraint?
  private var windowTop: NSLayoutConstraint?

  static func install(in view: UIView) {
    if let top = view.constraints.first(where: { $0.identifier == "mig33.launch.top" }),
       let image = top.firstItem as? UIImageView {
      guard !view.subviews.contains(where: { $0 is Mig33LaunchLayoutObserver }) else { return }
      let observer = Mig33LaunchLayoutObserver(frame: .zero)
      observer.imageView = image
      observer.storyboardTop = top
      observer.isUserInteractionEnabled = false
      observer.isAccessibilityElement = false
      observer.accessibilityElementsHidden = true
      observer.isHidden = true
      view.addSubview(observer)
      return
    }
    for child in view.subviews { install(in: child) }
  }

  override func didMoveToWindow() {
    super.didMoveToWindow()
    guard let window, let imageView else { return }
    windowTop?.isActive = false
    storyboardTop?.isActive = false
    let top = imageView.topAnchor.constraint(equalTo: window.safeAreaLayoutGuide.topAnchor, constant: 16)
    top.identifier = "mig33.launch.windowTop"
    top.isActive = true
    windowTop = top
  }

  override func willMove(toWindow newWindow: UIWindow?) {
    // A constraint installed on the window otherwise retains the image after
    // Expo removes its cover. Restore the local constraint for a future attach.
    windowTop?.isActive = false
    windowTop = nil
    storyboardTop?.isActive = true
    super.willMove(toWindow: newWindow)
  }
}
