import XCTest
@testable import SuperMoverApp

final class WorkbenchNavigationTests: XCTestCase {
    func testSidebarNavigationSeparatesHomeFromWorkflowGroups() {
        XCTAssertEqual(AppSection.homeSection, .controlRoom)
        XCTAssertEqual(AppSection.homeSection.title, "Home")

        XCTAssertEqual(AppSection.sidebarGroups.map(\.title), ["Workflow", "Evidence", "System"])
        XCTAssertEqual(AppSection.sidebarGroups.map(\.id), ["workflow", "evidence", "system"])
        XCTAssertEqual(
            AppSection.sidebarGroups.first(where: { $0.id == "workflow" })?.sections,
            [.setup, .devices, .transfer, .verification]
        )
        XCTAssertEqual(
            AppSection.sidebarGroups.first(where: { $0.id == "evidence" })?.sections,
            [.evidence]
        )
        XCTAssertEqual(
            AppSection.sidebarGroups.first(where: { $0.id == "system" })?.sections,
            [.taskDispatch, .settings]
        )
    }

    func testTopLevelNavigationOmitsMergedInternalPages() {
        XCTAssertEqual(
            AppSection.topLevelNavigationSections,
            [.controlRoom, .setup, .devices, .transfer, .verification, .evidence, .taskDispatch, .settings]
        )
        XCTAssertFalse(AppSection.topLevelNavigationSections.contains(.pairing))
        XCTAssertFalse(AppSection.topLevelNavigationSections.contains(.sync))
        XCTAssertFalse(AppSection.topLevelNavigationSections.contains(.driftReview))
    }

    func testMergedPagesResolveToTheirOwnerPages() {
        XCTAssertEqual(AppSection.pairing.ownerSection, .devices)
        XCTAssertEqual(AppSection.sync.ownerSection, .transfer)
        XCTAssertEqual(AppSection.driftReview.ownerSection, .verification)
        XCTAssertEqual(AppSection.taskDispatch.ownerSection, .taskDispatch)

        XCTAssertEqual(AppSection.setup.title, "Prepare")
        XCTAssertEqual(AppSection.devices.title, "Connect")
        XCTAssertEqual(AppSection.transfer.title, "Move")
        XCTAssertEqual(AppSection.verification.title, "Verify & Repair")
        XCTAssertEqual(AppSection.evidence.title, "Evidence Vault")
        XCTAssertEqual(AppSection.taskDispatch.title, "Task Dispatch")
    }

    func testOwnerSectionsDeclareFixedModeStripPolicy() {
        let sectionsWithOwnerModes: [AppSection] = [
            .devices,
            .pairing,
            .transfer,
            .sync,
            .verification,
            .driftReview,
        ]
        let sectionsWithoutOwnerModes: [AppSection] = [
            .controlRoom,
            .setup,
            .evidence,
            .taskDispatch,
            .settings,
        ]

        XCTAssertTrue(sectionsWithOwnerModes.allSatisfy(\.showsFixedOwnerModeStrip))
        XCTAssertTrue(sectionsWithoutOwnerModes.allSatisfy { !$0.showsFixedOwnerModeStrip })
    }

    func testOwnerModeStripUsesSingleSharedImplementation() throws {
        let source = try contentViewSource()

        XCTAssertFalse(source.contains("connectModeStrip"))
        XCTAssertFalse(source.contains("moveModeStrip"))
        XCTAssertFalse(source.contains("verifyRepairModeStrip"))
        XCTAssertTrue(source.contains("private func ownerModeStrip<ID: Hashable>"))
        XCTAssertEqual(source.components(separatedBy: "private func ownerModeStrip<ID: Hashable>").count - 1, 1)
    }

    func testOwnerModeStripDoesNotDuplicateOwnerPageTitles() throws {
        let source = try contentViewSource()
        guard
            let stripStart = source.range(of: "private func ownerModeStrip<ID: Hashable>"),
            let nextView = source.range(of: "\n  @ViewBuilder\n  private var connectView", range: stripStart.upperBound..<source.endIndex)
        else {
            return XCTFail("Expected ownerModeStrip source section")
        }
        let stripSource = String(source[stripStart.lowerBound..<nextView.lowerBound])

        XCTAssertFalse(stripSource.contains("PanelHeader("))
        XCTAssertFalse(stripSource.contains("title:"))
        XCTAssertFalse(stripSource.contains("subtitle:"))
        XCTAssertTrue(stripSource.contains("Spacer(minLength: 0)"))
    }

    func testLanguageSwitchingLivesInGlobalSidebarChrome() throws {
        let source = try contentViewSource()

        XCTAssertTrue(source.contains("private var globalLanguageMenu: some View"))
        XCTAssertTrue(source.contains("globalLanguageMenu"))
        XCTAssertFalse(
            source.contains("private var languagePreferencePicker"),
            "Language switching is global app chrome, not a Settings-page control."
        )
        XCTAssertTrue(source.contains("appChromeLocalization.text(.globalLanguageMenuTitle)"))
        XCTAssertEqual(
            source.components(separatedBy: "$uiPreferences.language").count - 1,
            1,
            "ContentView should bind language selection only from the global app chrome menu."
        )
    }

    func testEvidencePageUsesSidebarIdentityAsPageTitle() throws {
        let repoRoot = AcceptanceScriptHarness.repoRootURL()
        let evidenceViewURL = repoRoot
            .appendingPathComponent("macos")
            .appendingPathComponent("SuperMoverApp")
            .appendingPathComponent("EvidenceSectionView.swift")
        let source = try String(contentsOf: evidenceViewURL, encoding: .utf8)

        XCTAssertTrue(source.contains("AppSection.evidence.localizedTitle(using: localization)"))
        XCTAssertFalse(
            source.contains("title: \"Evidence\""),
            "Evidence page title should share the sidebar identity instead of introducing a parallel page name."
        )
    }

    func testSidebarNavigationLocalizedLabelsDoNotChangeNavigationIdentity() {
        let simplifiedChinese = AppChromeLocalization(language: .simplifiedChinese)

        XCTAssertEqual(AppSection.homeSection.localizedTitle(using: simplifiedChinese), "首页")
        XCTAssertEqual(AppSection.setup.localizedTitle(using: simplifiedChinese), "准备")
        XCTAssertEqual(AppSection.devices.localizedTitle(using: simplifiedChinese), "连接")
        XCTAssertEqual(AppSection.transfer.localizedTitle(using: simplifiedChinese), "迁移")
        XCTAssertEqual(AppSection.verification.localizedTitle(using: simplifiedChinese), "验证与修复")
        XCTAssertEqual(AppSection.evidence.localizedTitle(using: simplifiedChinese), "证据库")
        XCTAssertEqual(AppSection.taskDispatch.localizedTitle(using: simplifiedChinese), "任务调度")
        XCTAssertEqual(AppSection.settings.localizedTitle(using: simplifiedChinese), "设置")
        XCTAssertEqual(AppSection.localizedSidebarGroups(using: simplifiedChinese).map(\.title), ["工作流", "证据", "系统"])
        XCTAssertEqual(AppSection.localizedSidebarGroups(using: simplifiedChinese).map(\.id), ["workflow", "evidence", "system"])
        XCTAssertEqual(AppSection.localizedSidebarGroups(using: simplifiedChinese).flatMap(\.sections), AppSection.sidebarGroups.flatMap(\.sections))
        XCTAssertEqual(
            AppSection.topLevelNavigationSections.map(\.id),
            ["controlRoom", "setup", "devices", "transfer", "verification", "evidence", "taskDispatch", "settings"]
        )
    }

    func testTaskDispatchCategoriesAreStableAndCoverAllTasks() {
        XCTAssertEqual(
            SuperMoverTaskCategory.allCases,
            [.all, .runtime, .profile, .local, .review, .pairing, .network, .sync, .foreground]
        )
        XCTAssertEqual(SuperMoverTaskCategory.profile.title, "Config")
        XCTAssertEqual(SuperMoverTaskKind.profileInit.displayTitle, "Create Config File")
        XCTAssertEqual(SuperMoverTaskKind.lintProfile.displayTitle, "Lint Config")
        XCTAssertEqual(SuperMoverTaskKind.profileSetTarget.displayTitle, "Update Config Target")
        XCTAssertEqual(SuperMoverTaskKind.profileSetNetwork.displayTitle, "Update Config Network")
        XCTAssertEqual(SuperMoverTaskKind.profileAdoptPairing.displayTitle, "Adopt Pairing Receipt")
        XCTAssertEqual(SuperMoverTaskKind.profileInit.rawValue, "Profile Init")
        XCTAssertEqual(SuperMoverTaskKind.tasks(in: .all), SuperMoverTaskKind.allCases)
        XCTAssertEqual(SuperMoverTaskKind.status.taskCategory, .local)
        XCTAssertEqual(SuperMoverTaskKind.verify.taskCategory, .local)
        XCTAssertEqual(SuperMoverTaskKind.syncRun.taskCategory, .sync)
        XCTAssertEqual(SuperMoverTaskKind.daemonRun.taskCategory, .foreground)

        let groupedTasks = Set(
            SuperMoverTaskCategory.allCases
                .filter { $0 != .all }
                .flatMap { SuperMoverTaskKind.tasks(in: $0) }
        )
        XCTAssertEqual(groupedTasks, Set(SuperMoverTaskKind.allCases))
    }

    private func contentViewSource() throws -> String {
        let repoRoot = AcceptanceScriptHarness.repoRootURL()
        let contentViewURL = repoRoot
            .appendingPathComponent("macos")
            .appendingPathComponent("SuperMoverApp")
            .appendingPathComponent("ContentView.swift")
        return try String(contentsOf: contentViewURL, encoding: .utf8)
    }
}
