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
        XCTAssertEqual(
            english.text(.displayPreferencesSubtitle),
            "Choose this app window's appearance and interface language."
        )
        XCTAssertEqual(
            simplifiedChinese.text(.displayPreferencesSubtitle),
            "选择这个应用窗口的外观和界面语言。"
        )
        XCTAssertEqual(english.text(.languagePickerTitle), "Interface Language")
        XCTAssertEqual(simplifiedChinese.text(.languagePickerTitle), "界面语言")
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

        XCTAssertEqual(simplifiedChinese.text("Migration workbench"), "迁移工作台")
        XCTAssertEqual(
            simplifiedChinese.text("Foreground transfer, durable evidence, and operator checkpoints presented as one native desk."),
            "前台迁移、持久证据和操作员检查点汇聚在一个原生工作台中。"
        )
        XCTAssertEqual(simplifiedChinese.text("Focus"), "焦点")
        XCTAssertEqual(simplifiedChinese.text("Files"), "文件")
        XCTAssertEqual(simplifiedChinese.text("Transfers"), "传输")
        XCTAssertEqual(simplifiedChinese.text("LIVE"), "运行中")
        XCTAssertEqual(simplifiedChinese.text("Not checked"), "未检查")
        XCTAssertEqual(simplifiedChinese.text("not checked"), "未检查")
        XCTAssertEqual(simplifiedChinese.text("Source Config"), "源端配置")
        XCTAssertEqual(simplifiedChinese.text("Target Evidence"), "目标端证据")
        XCTAssertEqual(simplifiedChinese.text("Target evidence not loaded"), "未加载目标端证据")
        XCTAssertEqual(simplifiedChinese.text("Pass"), "通过")
        XCTAssertEqual(simplifiedChinese.text("Blocked"), "已阻塞")
        XCTAssertEqual(simplifiedChinese.text("Artifact"), "工件")
        XCTAssertEqual(simplifiedChinese.text("Network transfer"), "网络迁移")
        XCTAssertEqual(simplifiedChinese.text("No output yet."), "暂无输出。")
        XCTAssertEqual(
            String(format: simplifiedChinese.text("%d more facts retained in raw evidence."), 3),
            "还有 3 条事实保留在原始证据中。"
        )
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

    func testLiteralAppChromeLocalizationKeysHavePackagedResources() throws {
        let repoRoot = AcceptanceScriptHarness.repoRootURL()
        let appRoot = repoRoot
            .appendingPathComponent("macos")
            .appendingPathComponent("SuperMoverApp")
        let resourceRoot = appRoot.appendingPathComponent("Resources")

        let literalKeys = try Self.literalLocalizationKeys(in: appRoot)
        XCTAssertFalse(literalKeys.isEmpty)

        let englishKeys = try Self.localizedResourceKeys(
            at: resourceRoot
                .appendingPathComponent("en.lproj")
                .appendingPathComponent("Localizable.strings")
        )
        let simplifiedChineseKeys = try Self.localizedResourceKeys(
            at: resourceRoot
                .appendingPathComponent("zh-Hans.lproj")
                .appendingPathComponent("Localizable.strings")
        )

        XCTAssertTrue(literalKeys.subtracting(englishKeys).isEmpty)
        XCTAssertTrue(literalKeys.subtracting(simplifiedChineseKeys).isEmpty)
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

    private static func literalLocalizationKeys(in appRoot: URL) throws -> Set<String> {
        let enumerator = FileManager.default.enumerator(
            at: appRoot,
            includingPropertiesForKeys: [.isRegularFileKey],
            options: [.skipsHiddenFiles]
        )
        let pattern = #"(?:appChromeLocalization|localization)\.text\(\"((?:[^\"\\]|\\.)*)\"\)"#
        let regex = try NSRegularExpression(pattern: pattern)
        var keys = Set<String>()

        while let fileURL = enumerator?.nextObject() as? URL {
            guard fileURL.pathExtension == "swift" else {
                continue
            }
            let source = try String(contentsOf: fileURL, encoding: .utf8)
            let range = NSRange(source.startIndex..<source.endIndex, in: source)
            regex.enumerateMatches(in: source, range: range) { match, _, _ in
                guard
                    let match,
                    let keyRange = Range(match.range(at: 1), in: source)
                else {
                    return
                }
                keys.insert(String(source[keyRange]))
            }
        }

        return keys
    }

    private static func localizedResourceKeys(at url: URL) throws -> Set<String> {
        let source = try String(contentsOf: url, encoding: .utf8)
        let regex = try NSRegularExpression(pattern: #"^\s*\"((?:[^\"\\]|\\.)*)\"\s*="#, options: [.anchorsMatchLines])
        let range = NSRange(source.startIndex..<source.endIndex, in: source)
        var keys = Set<String>()
        regex.enumerateMatches(in: source, range: range) { match, _, _ in
            guard
                let match,
                let keyRange = Range(match.range(at: 1), in: source)
            else {
                return
            }
            keys.insert(String(source[keyRange]))
        }
        return keys
    }
}
