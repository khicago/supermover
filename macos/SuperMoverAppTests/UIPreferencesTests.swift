import AppKit
import SwiftUI
import XCTest
@testable import SuperMoverApp

final class UIPreferencesTests: XCTestCase {
    func testAppearancePreferenceMapsToSwiftUIAndWindowAppearance() {
        XCTAssertNil(UIAppearancePreference.system.colorScheme)
        XCTAssertEqual(UIAppearancePreference.light.colorScheme, .light)
        XCTAssertEqual(UIAppearancePreference.dark.colorScheme, .dark)

        XCTAssertNil(UIAppearancePreference.system.windowAppearanceName)
        XCTAssertEqual(UIAppearancePreference.light.windowAppearanceName, .aqua)
        XCTAssertEqual(UIAppearancePreference.dark.windowAppearanceName, .darkAqua)
    }

    func testLanguagePreferenceMapsToLocaleIdentifiers() {
        XCTAssertNil(UILanguagePreference.system.localeIdentifier)
        XCTAssertEqual(UILanguagePreference.english.localeIdentifier, "en")
        XCTAssertEqual(UILanguagePreference.simplifiedChinese.localeIdentifier, "zh-Hans")
    }

    func testAppChromeLocalizationLoadsSupportedLanguageResources() {
        let english = AppChromeLocalization(language: .english)
        let simplifiedChinese = AppChromeLocalization(language: .simplifiedChinese)

        XCTAssertEqual(english.text(.settingsTitle), "Settings")
        XCTAssertEqual(simplifiedChinese.text(.settingsTitle), "设置")
        XCTAssertEqual(english.text(.displayPreferencesTitle), "Display Preferences")
        XCTAssertEqual(simplifiedChinese.text(.displayPreferencesTitle), "显示偏好")
        XCTAssertEqual(english.text(.displayOnlyNoticeDetail), "Does not change command previews, migration config files, evidence bundles, or CLI output.")
        XCTAssertEqual(simplifiedChinese.text(.displayOnlyNoticeDetail), "不会更改命令预览、迁移配置文件、证据包或 CLI 输出。")
    }

    func testAppChromeLocalizationLoadsPrepareChromeResources() {
        let english = AppChromeLocalization(language: .english)
        let simplifiedChinese = AppChromeLocalization(language: .simplifiedChinese)

        XCTAssertEqual(english.text(.setupHeaderTitle), "Prepare")
        XCTAssertEqual(simplifiedChinese.text(.setupHeaderTitle), "准备")
        XCTAssertEqual(english.text(.setupConfigCardTitle), "Migration Setup")
        XCTAssertEqual(simplifiedChinese.text(.setupConfigCardTitle), "迁移设置")
        XCTAssertEqual(english.text(.workbenchRoleSourceTitle), "Source")
        XCTAssertEqual(simplifiedChinese.text(.workbenchRoleSourceTitle), "源端")
        XCTAssertEqual(
            english.text(.setupGuideSourceTitle),
            "Prepare this Source"
        )
        XCTAssertEqual(
            simplifiedChinese.text(.setupGuideSourceTitle),
            "准备这台源端 Mac"
        )
        XCTAssertEqual(english.text(.setupActionOpenExistingConfig), "Open Existing Config")
        XCTAssertEqual(simplifiedChinese.text(.setupActionOpenExistingConfig), "打开现有配置")
        XCTAssertEqual(english.text(.setupActionCreateRecommendedConfig), "Create Migration Setup")
        XCTAssertEqual(simplifiedChinese.text(.setupActionCreateRecommendedConfig), "创建迁移设置")
        XCTAssertEqual(english.text(.setupActionChooseCustomConfigLocation), "Choose Custom Location")
        XCTAssertEqual(simplifiedChinese.text(.setupActionChooseCustomConfigLocation), "选择自定义位置")
        XCTAssertEqual(english.text(.setupSourceRootFieldTitle), "Folder to Move From This Mac")
        XCTAssertEqual(simplifiedChinese.text(.setupSourceRootFieldTitle), "从这台 Mac 迁出的目录")
        XCTAssertEqual(english.text(.setupSourceRootPlaceholder), "Choose the folder to move from this Mac")
        XCTAssertEqual(simplifiedChinese.text(.setupSourceRootPlaceholder), "选择要从这台 Mac 传出的目录")
        XCTAssertEqual(english.text(.setupTargetRootFieldTitle), "Folder to Save On This Mac")
        XCTAssertEqual(simplifiedChinese.text(.setupTargetRootFieldTitle), "保存到这台 Mac 的目录")
        XCTAssertEqual(english.text(.setupTargetRootPlaceholder), "Choose where this Mac saves incoming files")
        XCTAssertEqual(simplifiedChinese.text(.setupTargetRootPlaceholder), "选择这台 Mac 接收文件的位置")
        XCTAssertEqual(english.text(.setupStatusTargetPathSet), "target path set")
        XCTAssertEqual(simplifiedChinese.text(.setupStatusTargetPathSet), "目标路径已填写")
        XCTAssertEqual(english.text(.setupStatusTargetPathMissing), "target path missing")
        XCTAssertEqual(simplifiedChinese.text(.setupStatusTargetPathMissing), "目标路径未填写")
        XCTAssertEqual(english.text(.setupProfileAdvancedTitle), "Advanced Options")
        XCTAssertEqual(simplifiedChinese.text(.setupProfileAdvancedTitle), "高级选项")
    }

    func testCoreWorkbenchPageChromeRawKeysHaveSimplifiedChineseTranslations() {
        let simplifiedChinese = AppChromeLocalization(language: .simplifiedChinese)

        XCTAssertEqual(simplifiedChinese.text("Device State"), "设备状态")
        XCTAssertEqual(simplifiedChinese.text("Trust Ceremony"), "信任确认")
        XCTAssertEqual(simplifiedChinese.text("Transfer Route"), "迁移路径")
        XCTAssertEqual(simplifiedChinese.text("Evidence Supporting Surfaces"), "证据辅助面板")
        XCTAssertEqual(simplifiedChinese.text("Install Readiness"), "安装就绪")
        XCTAssertEqual(simplifiedChinese.text("Open Task Dispatch"), "打开任务调度")
        XCTAssertEqual(simplifiedChinese.text("Safety Gates"), "安全门禁")
        XCTAssertEqual(simplifiedChinese.text("Migration Config Network"), "迁移配置网络")
        XCTAssertEqual(simplifiedChinese.text("Acceptance Bundle"), "验收证据包")
        XCTAssertEqual(simplifiedChinese.text("Artifact Catalog"), "工件目录")
        XCTAssertEqual(simplifiedChinese.text("CLI Preview"), "CLI 预览")
    }

    func testTaskDispatchDisplayModelUsesLocalization() {
        let simplifiedChinese = AppChromeLocalization(language: .simplifiedChinese)

        XCTAssertEqual(SuperMoverTaskCategory.profile.localizedTitle(using: simplifiedChinese), "配置")
        XCTAssertEqual(SuperMoverTaskKind.profileInit.localizedDisplayTitle(using: simplifiedChinese), "创建配置文件")
        XCTAssertEqual(SuperMoverTaskKind.lintProfile.localizedDisplayTitle(using: simplifiedChinese), "检查配置")
        XCTAssertEqual(
            SuperMoverTaskKind.profileInit.localizedSummary(using: simplifiedChinese),
            "在这台 Mac 的源端目录可读取后，通过 CLI 创建源端迁移配置文件。"
        )
    }

    func testVisibleWorkbenchRoleChromeDoesNotReadRawRoleTitles() throws {
        let repoRoot = AcceptanceScriptHarness.repoRootURL()
        let contentViewURL = repoRoot
            .appendingPathComponent("macos")
            .appendingPathComponent("SuperMoverApp")
            .appendingPathComponent("ContentView.swift")
        let source = try String(contentsOf: contentViewURL, encoding: .utf8)

        XCTAssertFalse(
            source.contains("store.selectedRole.title"),
            "Visible role values should use localizedTitle(using:) instead of raw WorkbenchRole.title."
        )
        XCTAssertFalse(
            source.contains("label: \"role\""),
            "Visible role badge labels should use the localized role chrome label."
        )
        XCTAssertFalse(
            source.contains("\"Role •"),
            "Visible role metadata prefixes should use the localized role chrome label."
        )
        XCTAssertTrue(source.contains("store.selectedRole.localizedTitle(using: appChromeLocalization)"))
        XCTAssertTrue(source.contains("appChromeLocalization.text(.sidebarRoleLabel)"))
    }

    func testAuxiliaryWorkbenchPanelsUseInjectedLocalization() throws {
        let repoRoot = AcceptanceScriptHarness.repoRootURL()
        let appRoot = repoRoot
            .appendingPathComponent("macos")
            .appendingPathComponent("SuperMoverApp")
        let panelFiles = [
            "ProfileNetworkPanel.swift",
            "PairingReceiptPanel.swift",
            "AcceptanceBundlePanel.swift",
            "AcceptanceOperatorEvidencePanel.swift",
        ]
        let forbiddenChromePatterns = [
            "Text(\"",
            "TextField(\"",
            "Toggle(\"",
            "Picker(\"",
            "ActionButton(\"",
            "PrimaryActionButton(\"",
            "CompactActionButton(\"",
        ]

        for fileName in panelFiles {
            let source = try String(
                contentsOf: appRoot.appendingPathComponent(fileName),
                encoding: .utf8
            )

            XCTAssertTrue(
                source.contains("var localization: AppChromeLocalization"),
                "\(fileName) should receive shared app chrome localization."
            )
            XCTAssertTrue(
                source.contains("localization.text("),
                "\(fileName) should translate visible chrome through AppChromeLocalization."
            )
            for pattern in forbiddenChromePatterns {
                XCTAssertFalse(
                    source.contains(pattern),
                    "\(fileName) should not hard-code visible SwiftUI chrome with \(pattern)."
                )
            }
        }

        let contentView = try String(
            contentsOf: appRoot.appendingPathComponent("ContentView.swift"),
            encoding: .utf8
        )
        XCTAssertTrue(contentView.contains("localization: appChromeLocalization"))

        let evidenceView = try String(
            contentsOf: appRoot.appendingPathComponent("EvidenceSectionView.swift"),
            encoding: .utf8
        )
        XCTAssertTrue(evidenceView.contains("localization: localization"))
    }

    func testAppChromeLocalizationSystemLanguageUsesPreferredSupportedLocalization() {
        let systemChinese = AppChromeLocalization(
            language: .system,
            preferredLanguagesProvider: { ["zh-Hans-US", "en-US"] }
        )

        XCTAssertEqual(systemChinese.text(.settingsTitle), "设置")
    }

    func testAppChromeLocalizationLoadsFromPackagedAppResourceDirectory() throws {
        let root = try makeUIPreferencesTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: root) }
        let languageDirectory = root.appendingPathComponent("zh-hans.lproj", isDirectory: true)
        try FileManager.default.createDirectory(at: languageDirectory, withIntermediateDirectories: true)
        try "\"settings.title\" = \"设置\";\n".write(
            to: languageDirectory.appendingPathComponent("Localizable.strings"),
            atomically: true,
            encoding: .utf8
        )
        let localization = AppChromeLocalization(
            language: .simplifiedChinese,
            resourceDirectoryProviders: [{ root }]
        )

        XCTAssertEqual(localization.text(.settingsTitle), "设置")
    }

    func testAppChromeLocalizationFallsBackToProvidedEnglishTextForMissingKeys() {
        let simplifiedChinese = AppChromeLocalization(language: .simplifiedChinese)

        XCTAssertEqual(
            simplifiedChinese.text(rawKey: "missing.chrome.key", englishFallback: "Missing chrome text"),
            "Missing chrome text"
        )
    }

    @MainActor
    func testInvalidStoredValuesFallBackToSystem() {
        withTemporaryDefaults { defaults in
            defaults.set("sepia", forKey: UIAppearancePreference.storageKey)
            defaults.set("klingon", forKey: UILanguagePreference.storageKey)

            let store = UIPreferencesStore(defaults: defaults)

            XCTAssertEqual(store.appearance, .system)
            XCTAssertEqual(store.language, .system)
        }
    }

    @MainActor
    func testPreferenceChangesPersistOnlyUIDefaultKeys() {
        withTemporaryDefaults { defaults in
            let store = UIPreferencesStore(defaults: defaults)

            store.appearance = .dark
            store.language = .simplifiedChinese

            XCTAssertEqual(defaults.string(forKey: UIAppearancePreference.storageKey), "dark")
            XCTAssertEqual(defaults.string(forKey: UILanguagePreference.storageKey), "simplifiedChinese")
            XCTAssertNil(defaults.string(forKey: "profilePath"))
            XCTAssertNil(defaults.string(forKey: "sourceRootPath"))
            XCTAssertNil(defaults.string(forKey: "targetRootPath"))
        }
    }

    private func withTemporaryDefaults(_ body: (UserDefaults) throws -> Void) rethrows {
        let suiteName = "SuperMoverUIPreferencesTests-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suiteName)!
        defaults.removePersistentDomain(forName: suiteName)
        defer { defaults.removePersistentDomain(forName: suiteName) }
        try body(defaults)
    }

    private func makeUIPreferencesTemporaryDirectory() throws -> URL {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent(
            "SuperMoverUIPreferencesTests-\(UUID().uuidString)",
            isDirectory: true
        )
        try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
        return url
    }
}
