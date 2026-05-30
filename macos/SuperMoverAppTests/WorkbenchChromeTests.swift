import AppKit
import XCTest
@testable import SuperMoverApp

final class WorkbenchChromeTests: XCTestCase {
    func testMinimumContentWidthCoversPinnedDetailWorkbenchLayout() {
        XCTAssertEqual(
            WorkbenchWindowMetrics.minimumContentSize.width,
            WorkbenchLayoutMetrics.minimumPinnedDetailWorkbenchWidth()
        )
        XCTAssertGreaterThan(
            WorkbenchWindowMetrics.minimumContentSize.width,
            1260
        )
    }

    func testWindowMetricsClampUndersizedContentToMinimum() {
        XCTAssertEqual(
            WorkbenchWindowMetrics.clampedContentSize(for: CGSize(width: 980, height: 700)),
            WorkbenchWindowMetrics.minimumContentSize
        )
        XCTAssertEqual(
            WorkbenchWindowMetrics.clampedContentSize(for: CGSize(width: 1400, height: 760)),
            CGSize(width: 1400, height: WorkbenchWindowMetrics.minimumContentSize.height)
        )
        XCTAssertEqual(
            WorkbenchWindowMetrics.clampedContentSize(for: CGSize(width: 1400, height: 920)),
            CGSize(width: 1400, height: 920)
        )
    }

    func testWindowMetricsClampWindowFrameUsingChromeInsets() {
        XCTAssertEqual(
            WorkbenchWindowMetrics.minimumWindowFrameSize(
                for: CGSize(width: 1260, height: 820),
                contentLayoutSize: CGSize(width: 1260, height: 792)
            ),
            CGSize(width: WorkbenchWindowMetrics.minimumContentSize.width, height: 848)
        )
        XCTAssertEqual(
            WorkbenchWindowMetrics.clampedWindowFrameSize(
                for: CGSize(width: 980, height: 700),
                contentLayoutSize: CGSize(width: 980, height: 672)
            ),
            CGSize(width: WorkbenchWindowMetrics.minimumContentSize.width, height: 848)
        )
        XCTAssertEqual(
            WorkbenchWindowMetrics.clampedWindowFrameSize(
                for: CGSize(width: 1400, height: 920),
                contentLayoutSize: CGSize(width: 1400, height: 892)
            ),
            CGSize(width: 1400, height: 920)
        )
        XCTAssertNil(
            WorkbenchWindowMetrics.minimumWindowFrameSize(
                for: CGSize(width: 1260, height: 820),
                contentLayoutSize: .zero
            )
        )
    }

    func testWindowMetricsZoomedFrameHonorsMinimumWindowSize() {
        let minimumWindowFrameSize = CGSize(width: WorkbenchWindowMetrics.minimumContentSize.width, height: 848)
        XCTAssertEqual(
            WorkbenchWindowMetrics.zoomedFrame(
                for: CGRect(x: 32, y: 120, width: 1100, height: 760),
                minimumWindowFrameSize: minimumWindowFrameSize
            ),
            CGRect(x: 32, y: 32, width: WorkbenchWindowMetrics.minimumContentSize.width, height: 848)
        )
        XCTAssertEqual(
            WorkbenchWindowMetrics.zoomedFrame(
                for: CGRect(x: 32, y: 120, width: 1440, height: 900),
                minimumWindowFrameSize: minimumWindowFrameSize
            ),
            CGRect(x: 32, y: 120, width: 1440, height: 900)
        )
    }

    func testDetailPageHeaderUsesNativePinnedSectionHeader() throws {
        let source = try workbenchChromeSource()
        guard
            let hostStart = source.range(of: "struct DetailPageHost<"),
            let nextComponent = source.range(
                of: "\nstruct WorkbenchMediaSlot",
                range: hostStart.upperBound..<source.endIndex
            )
        else {
            return XCTFail("Expected DetailPageHost source section")
        }
        let hostSource = String(source[hostStart.lowerBound..<nextComponent.lowerBound])

        XCTAssertTrue(
            hostSource.contains("LazyVStack(alignment: .leading, spacing: 0, pinnedViews: [.sectionHeaders])")
        )
        XCTAssertTrue(hostSource.contains("Section {"))
        XCTAssertFalse(hostSource.contains("GeometryReader"))
        XCTAssertFalse(hostSource.contains("stickyHeaderOffset"))
        XCTAssertFalse(hostSource.contains("headerPositionReader"))
        XCTAssertFalse(hostSource.contains("offset(y:"))
        XCTAssertFalse(source.contains("detailPageStickyHeaderOffset"))
        XCTAssertFalse(source.contains("DetailPageHeaderMinYPreferenceKey"))
    }

    func testFixedOwnerModeStripLeavesDedicatedBodyGap() {
        XCTAssertLessThan(
            WorkbenchLayoutMetrics.mainContentTopPadding,
            WorkbenchLayoutMetrics.mainContentVerticalPadding
        )
        XCTAssertGreaterThan(WorkbenchLayoutMetrics.fixedOwnerModeStripBottomPadding, 0)
        XCTAssertGreaterThan(WorkbenchLayoutMetrics.fixedOwnerModeStripBodyGap, 0)
        XCTAssertLessThan(
            WorkbenchLayoutMetrics.fixedOwnerModeStripBodyGap,
            WorkbenchLayoutMetrics.mainContentVerticalPadding
        )
    }

    func testWrappedRowMetricsSplitOverflowingItemsIntoMultipleRows() {
        XCTAssertEqual(
            WorkbenchLayoutMetrics.wrappedRowMetrics(
                for: [
                    CGSize(width: 140, height: 20),
                    CGSize(width: 160, height: 18),
                    CGSize(width: 100, height: 30),
                    CGSize(width: 120, height: 16),
                ],
                maxWidth: 320,
                itemSpacing: 12
            ),
            [
                WorkbenchWrappedRowMetrics(indices: [0, 1], width: 312, height: 20),
                WorkbenchWrappedRowMetrics(indices: [2, 3], width: 232, height: 30),
            ]
        )
    }

    func testWrappedRowMetricsKeepItemsOnSingleRowWhenWidthIsUnbounded() {
        XCTAssertEqual(
            WorkbenchLayoutMetrics.wrappedRowMetrics(
                for: [
                    CGSize(width: 140, height: 20),
                    CGSize(width: 160, height: 18),
                    CGSize(width: 100, height: 30),
                ],
                maxWidth: .greatestFiniteMagnitude,
                itemSpacing: 12
            ),
            [
                WorkbenchWrappedRowMetrics(indices: [0, 1, 2], width: 424, height: 30),
            ]
        )
    }

    private func workbenchChromeSource() throws -> String {
        let repoRoot = AcceptanceScriptHarness.repoRootURL()
        let workbenchChromeURL = repoRoot
            .appendingPathComponent("macos")
            .appendingPathComponent("SuperMoverApp")
            .appendingPathComponent("WorkbenchChrome.swift")
        return try String(contentsOf: workbenchChromeURL, encoding: .utf8)
    }

    @MainActor
    func testWindowChromeApplyExpandsUsableLayoutAndKeepsSingleHitZone() {
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 980, height: 700),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )

        WorkbenchWindowChrome.apply(to: window)
        let resizedFrameSize = window.frame.size

        XCTAssertEqual(window.titleVisibility, .hidden)
        XCTAssertTrue(window.titlebarAppearsTransparent)
        XCTAssertTrue(window.isMovableByWindowBackground)
        XCTAssertTrue(window.styleMask.contains(.fullSizeContentView))
        XCTAssertGreaterThanOrEqual(window.contentMinSize.width, WorkbenchWindowMetrics.minimumContentSize.width)
        XCTAssertGreaterThanOrEqual(window.contentMinSize.height, WorkbenchWindowMetrics.minimumContentSize.height)
        XCTAssertEqual(window.minSize, resizedFrameSize)
        XCTAssertGreaterThanOrEqual(resizedFrameSize.width, WorkbenchWindowMetrics.minimumContentSize.width)
        XCTAssertGreaterThanOrEqual(resizedFrameSize.height, WorkbenchWindowMetrics.minimumContentSize.height)
        XCTAssertGreaterThanOrEqual(
            window.contentLayoutRect.size.width,
            WorkbenchWindowMetrics.minimumContentSize.width
        )
        XCTAssertGreaterThanOrEqual(
            window.contentLayoutRect.size.height,
            WorkbenchWindowMetrics.minimumContentSize.height
        )

        guard let container = window.contentView?.superview else {
            return XCTFail("Expected a window container view")
        }

        XCTAssertEqual(
            container.subviews.filter { $0.identifier == WindowChromeMetrics.dragZoomIdentifier }.count,
            1
        )

        WorkbenchWindowChrome.apply(to: window)

        XCTAssertEqual(window.frame.size, resizedFrameSize)
        XCTAssertEqual(
            container.subviews.filter { $0.identifier == WindowChromeMetrics.dragZoomIdentifier }.count,
            1
        )
    }

    @MainActor
    func testWindowChromeAppliesRequestedAppearanceWithoutChangingHitZone() {
        let window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1280, height: 840),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )

        WorkbenchWindowChrome.apply(to: window, appearanceName: UIAppearancePreference.dark.windowAppearanceName)

        XCTAssertEqual(window.appearance?.name, .darkAqua)
        guard let container = window.contentView?.superview else {
            return XCTFail("Expected a window container view")
        }
        XCTAssertEqual(
            container.subviews.filter { $0.identifier == WindowChromeMetrics.dragZoomIdentifier }.count,
            1
        )

        WorkbenchWindowChrome.apply(to: window, appearanceName: UIAppearancePreference.system.windowAppearanceName)

        XCTAssertNil(window.appearance)
        XCTAssertEqual(
            container.subviews.filter { $0.identifier == WindowChromeMetrics.dragZoomIdentifier }.count,
            1
        )
    }
}
