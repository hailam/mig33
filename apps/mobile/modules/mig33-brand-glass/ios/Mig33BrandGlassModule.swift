import ExpoModulesCore
import SwiftUI

/// CADisplayLink retains its target. Keep the owning view weak so a removed
/// React subtree cannot remain alive waiting for a display frame.
@MainActor
private final class Mig33BrandFrameTarget: NSObject {
  weak var view: Mig33BrandGlassView?
  @objc func tick(_ link: CADisplayLink) { view?.receiveReadyFrame(link) }
}

public final class Mig33BrandGlassModule: Module {
  public func definition() -> ModuleDefinition {
    Name("Mig33BrandGlass")
    View(Mig33BrandGlassView.self) {
      Events("onReady", "onComplete")
      Prop("animate") { (view: Mig33BrandGlassView, value: Bool) in view.model.animate = value }
      Prop("settled") { (view: Mig33BrandGlassView, value: Bool) in view.model.settled = value }
      Prop("dark") { (view: Mig33BrandGlassView, value: Bool) in view.model.dark = value }
    }
  }
}

public final class Mig33BrandGlassView: ExpoView {
  let model = Mig33BrandModel()
  private var host: UIHostingController<Mig33BrandScene>!
  private var readinessLink: CADisplayLink?
  private var readinessFrames = 0
  private var readyLayoutSize = CGSize.zero
  let onReady = EventDispatcher()
  let onComplete = EventDispatcher()

  public required init(appContext: AppContext? = nil) {
    super.init(appContext: appContext)
    backgroundColor = .clear
    isOpaque = false
    isUserInteractionEnabled = false
    clipsToBounds = false
    host = UIHostingController(rootView: Mig33BrandScene(model: model))
    host.safeAreaRegions = []
    host.view.backgroundColor = .clear
    host.view.isOpaque = false
    host.view.isUserInteractionEnabled = false
    host.view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
    addSubview(host.view)
    model.onComplete = { [weak self] in self?.onComplete([:]) }
  }

  public override func layoutSubviews() {
    super.layoutSubviews()
    host.view.frame = bounds
    scheduleReadiness()
  }

  public override func didMoveToWindow() {
    super.didMoveToWindow()
    guard window != nil else {
      model.attached = false
      readinessLink?.invalidate()
      readinessLink = nil
      readinessFrames = 0
      if model.ready { model.settled = true }
      host.willMove(toParent: nil)
      host.view.removeFromSuperview()
      host.removeFromParent()
      return
    }
    // Follow UIKit containment so SwiftUI inherits the actual scene lifecycle.
    // The content itself never presents anything or receives touches.
    if host.parent == nil, let parent = nearestViewController,
       parent.viewIfLoaded.map({ isDescendant(of: $0) }) == true,
       !(parent is UINavigationController), !(parent is UITabBarController) {
      parent.addChild(host)
      addSubview(host.view)
      host.didMove(toParent: parent)
    } else if host.view.superview !== self {
      addSubview(host.view)
    }
    host.view.frame = bounds
    model.attached = true
    scheduleReadiness()
  }

  private var nearestViewController: UIViewController? {
    var responder: UIResponder? = next
    while let current = responder {
      if let controller = current as? UIViewController { return controller }
      responder = current.next
    }
    return nil
  }

  private func scheduleReadiness() {
    guard !model.ready, window != nil, host.view.window != nil,
      bounds.width.isFinite, bounds.height.isFinite,
      bounds.width > 0, bounds.height > 0 else { return }
    if readyLayoutSize != bounds.size {
      readyLayoutSize = bounds.size
      readinessFrames = 0
    }
    guard readinessLink == nil else { return }
    let target = Mig33BrandFrameTarget()
    target.view = self
    let link = CADisplayLink(target: target, selector: #selector(Mig33BrandFrameTarget.tick(_:)))
    readinessLink = link
    link.add(to: .main, forMode: .common)
  }

  fileprivate func receiveReadyFrame(_ link: CADisplayLink) {
    guard link === readinessLink, !model.ready, window != nil, host.view.window != nil,
      bounds.width > 0, bounds.height > 0, bounds.size == readyLayoutSize else {
      link.invalidate()
      if link === readinessLink { readinessLink = nil; readinessFrames = 0 }
      return
    }
    // The first callback coordinates the positive native layout with SwiftUI;
    // the second occurs after that display cycle had a chance to commit. This
    // is frame coordination, independent of 60/120 Hz and application timing.
    host.view.layoutIfNeeded()
    readinessFrames += 1
    guard readinessFrames >= 2 else { return }
    link.invalidate()
    readinessLink = nil
    model.ready = true
    onReady([:])
  }
}
