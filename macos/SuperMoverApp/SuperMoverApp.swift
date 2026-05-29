import AppKit
import SwiftUI

@main
struct SuperMoverDesktopApp: App {
    @StateObject private var store = AppStore()
    @StateObject private var uiPreferences = UIPreferencesStore()

    var body: some Scene {
        WindowGroup("SuperMover") {
            ContentView()
                .environmentObject(store)
                .environmentObject(uiPreferences)
                .environment(\.locale, uiPreferences.language.locale)
                .preferredColorScheme(uiPreferences.appearance.colorScheme)
                .frame(
                    minWidth: WorkbenchWindowMetrics.minimumContentSize.width,
                    minHeight: WorkbenchWindowMetrics.minimumContentSize.height
                )
                .background(WindowChromeConfigurator(appearanceName: uiPreferences.appearance.windowAppearanceName))
        }
        .defaultSize(
            width: WorkbenchWindowMetrics.minimumContentSize.width,
            height: WorkbenchWindowMetrics.minimumContentSize.height
        )
        .windowResizability(.contentSize)
    }
}

enum WindowChromeMetrics {
    static let dragZoomIdentifier = NSUserInterfaceItemIdentifier("SuperMoverWindowDragZoomZone")
    static let trafficLightReservedWidth: CGFloat = 96
    static let dragZoomHeight: CGFloat = 30
}

@MainActor
private struct WindowChromeConfigurator: NSViewRepresentable {
    let appearanceName: NSAppearance.Name?

    func makeNSView(context: Context) -> WindowChromeAttachmentView {
        let view = WindowChromeAttachmentView(frame: .zero)
        view.onWindowUpdate = { window in
            WorkbenchWindowChrome.apply(to: window, appearanceName: appearanceName)
        }
        return view
    }

    func updateNSView(_ view: WindowChromeAttachmentView, context: Context) {
        view.onWindowUpdate = { window in
            WorkbenchWindowChrome.apply(to: window, appearanceName: appearanceName)
        }
        view.applyWindowChromeIfPossible()
    }
}

@MainActor
enum WorkbenchWindowChrome {
    static func apply(to window: NSWindow, appearanceName: NSAppearance.Name? = nil) {
        window.appearance = appearanceName.flatMap(NSAppearance.init(named:))
        window.titleVisibility = .hidden
        window.titlebarAppearsTransparent = true
        window.isMovableByWindowBackground = true
        window.styleMask.insert([.fullSizeContentView, .resizable])
        window.contentMinSize = WorkbenchWindowMetrics.minimumContentSize

        let frameSize = window.frame.size
        let contentLayoutSize = window.contentLayoutRect.size
        if let minimumFrameSize = WorkbenchWindowMetrics.minimumWindowFrameSize(
            for: frameSize,
            contentLayoutSize: contentLayoutSize
        ) {
            window.minSize = minimumFrameSize
        }
        if let clampedFrameSize = WorkbenchWindowMetrics.clampedWindowFrameSize(
            for: frameSize,
            contentLayoutSize: contentLayoutSize
        ), frameSize != clampedFrameSize {
            let clampedFrame = NSRect(origin: window.frame.origin, size: clampedFrameSize)
            window.setFrame(clampedFrame, display: false)
        }
        installHitZone(in: window)
    }

    private static func installHitZone(in window: NSWindow) {
        guard let container = window.contentView?.superview else {
            return
        }
        let hitZone = existingHitZone(in: container) ?? WindowDragZoomView(frame: .zero)
        hitZone.frame = NSRect(
            x: WindowChromeMetrics.trafficLightReservedWidth,
            y: max(0, container.bounds.height - WindowChromeMetrics.dragZoomHeight),
            width: max(0, container.bounds.width - WindowChromeMetrics.trafficLightReservedWidth),
            height: WindowChromeMetrics.dragZoomHeight
        )
        hitZone.autoresizingMask = [.width, .minYMargin]
        if hitZone.superview == nil {
            container.addSubview(hitZone, positioned: .above, relativeTo: nil)
        }
    }

    private static func existingHitZone(in container: NSView) -> WindowDragZoomView? {
        container.subviews.first { view in
            view.identifier == WindowChromeMetrics.dragZoomIdentifier
        } as? WindowDragZoomView
    }
}

private final class WindowChromeAttachmentView: NSView {
    var onWindowUpdate: (@MainActor (NSWindow) -> Void)?

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        applyWindowChromeIfPossible()
    }

    override func layout() {
        super.layout()
        applyWindowChromeIfPossible()
    }

    func applyWindowChromeIfPossible() {
        guard let window else {
            return
        }
        onWindowUpdate?(window)
    }
}

private final class WindowDragZoomView: NSView {
    private var frameBeforeZoom: NSRect?

    override init(frame frameRect: NSRect) {
        super.init(frame: frameRect)
        identifier = WindowChromeMetrics.dragZoomIdentifier
        wantsLayer = true
        layer?.backgroundColor = NSColor.clear.cgColor
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) {
        nil
    }

    override var mouseDownCanMoveWindow: Bool {
        true
    }

    override func hitTest(_ point: NSPoint) -> NSView? {
        bounds.contains(point) ? self : nil
    }

    override func acceptsFirstMouse(for event: NSEvent?) -> Bool {
        true
    }

    override func mouseDown(with event: NSEvent) {
        if event.clickCount >= 2 {
            toggleWindowZoomToVisibleFrame()
            return
        }
        window?.performDrag(with: event)
    }

    private func toggleWindowZoomToVisibleFrame() {
        guard let window else {
            return
        }
        let visibleFrame = window.screen?.visibleFrame ?? NSScreen.main?.visibleFrame
        guard let visibleFrame else {
            window.performZoom(nil)
            return
        }
        let minimumWindowFrameSize = window.frameRect(
            forContentRect: NSRect(origin: .zero, size: WorkbenchWindowMetrics.minimumContentSize)
        ).size
        let targetFrame = WorkbenchWindowMetrics.zoomedFrame(
            for: visibleFrame,
            minimumWindowFrameSize: minimumWindowFrameSize
        )
        if window.frame.isApproximatelyEqual(to: targetFrame), let frameBeforeZoom {
            window.setFrame(frameBeforeZoom, display: true, animate: true)
            self.frameBeforeZoom = nil
            return
        }
        frameBeforeZoom = window.frame
        window.setFrame(targetFrame, display: true, animate: true)
    }
}

private extension NSRect {
    func isApproximatelyEqual(to other: NSRect, tolerance: CGFloat = 2) -> Bool {
        abs(origin.x - other.origin.x) <= tolerance
            && abs(origin.y - other.origin.y) <= tolerance
            && abs(size.width - other.size.width) <= tolerance
            && abs(size.height - other.size.height) <= tolerance
    }
}
