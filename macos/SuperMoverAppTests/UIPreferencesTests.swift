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
        XCTAssertEqual(english.text(.setupConfigCardTitle), "Migration Config")
        XCTAssertEqual(simplifiedChinese.text(.setupConfigCardTitle), "迁移配置")
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
        XCTAssertEqual(english.text(.setupActionCreateRecommendedConfig), "Create Recommended Config")
        XCTAssertEqual(simplifiedChinese.text(.setupActionCreateRecommendedConfig), "创建推荐配置")
        XCTAssertEqual(english.text(.setupActionChooseCustomConfigLocation), "Choose Custom Location")
        XCTAssertEqual(simplifiedChinese.text(.setupActionChooseCustomConfigLocation), "选择自定义位置")
        XCTAssertEqual(english.text(.setupProfileAdvancedTitle), "Advanced")
        XCTAssertEqual(simplifiedChinese.text(.setupProfileAdvancedTitle), "高级")
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
