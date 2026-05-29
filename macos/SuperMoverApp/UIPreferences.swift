import AppKit
import SwiftUI

enum UIAppearancePreference: String, CaseIterable, Identifiable {
  case system
  case light
  case dark

  static let storageKey = "supermover.ui.appearance"

  var id: String { rawValue }

  init(storedRawValue: String?) {
    self = storedRawValue.flatMap(Self.init(rawValue:)) ?? .system
  }

  var title: String {
    switch self {
    case .system:
      return "System"
    case .light:
      return "Light"
    case .dark:
      return "Dark"
    }
  }

  func localizedTitle(using localization: AppChromeLocalization) -> String {
    switch self {
    case .system:
      return localization.text(.appearanceSystemTitle)
    case .light:
      return localization.text(.appearanceLightTitle)
    case .dark:
      return localization.text(.appearanceDarkTitle)
    }
  }

  var colorScheme: ColorScheme? {
    switch self {
    case .system:
      return nil
    case .light:
      return .light
    case .dark:
      return .dark
    }
  }

  var windowAppearanceName: NSAppearance.Name? {
    switch self {
    case .system:
      return nil
    case .light:
      return .aqua
    case .dark:
      return .darkAqua
    }
  }
}

enum UILanguagePreference: String, CaseIterable, Identifiable {
  case system
  case english
  case simplifiedChinese

  static let storageKey = "supermover.ui.language"

  var id: String { rawValue }

  init(storedRawValue: String?) {
    self = storedRawValue.flatMap(Self.init(rawValue:)) ?? .system
  }

  var title: String {
    switch self {
    case .system:
      return "System"
    case .english:
      return "English"
    case .simplifiedChinese:
      return "简体中文"
    }
  }

  func localizedTitle(using localization: AppChromeLocalization) -> String {
    switch self {
    case .system:
      return localization.text(.languageSystemTitle)
    case .english:
      return localization.text(.languageEnglishTitle)
    case .simplifiedChinese:
      return localization.text(.languageSimplifiedChineseTitle)
    }
  }

  var localeIdentifier: String? {
    switch self {
    case .system:
      return nil
    case .english:
      return "en"
    case .simplifiedChinese:
      return "zh-Hans"
    }
  }

  var locale: Locale {
    localeIdentifier.map(Locale.init(identifier:)) ?? .autoupdatingCurrent
  }
}

@MainActor
final class UIPreferencesStore: ObservableObject {
  private let defaults: UserDefaults

  @Published var appearance: UIAppearancePreference {
    didSet {
      defaults.set(appearance.rawValue, forKey: UIAppearancePreference.storageKey)
    }
  }

  @Published var language: UILanguagePreference {
    didSet {
      defaults.set(language.rawValue, forKey: UILanguagePreference.storageKey)
    }
  }

  init(defaults: UserDefaults = .standard) {
    self.defaults = defaults
    appearance = UIAppearancePreference(storedRawValue: defaults.string(forKey: UIAppearancePreference.storageKey))
    language = UILanguagePreference(storedRawValue: defaults.string(forKey: UILanguagePreference.storageKey))

    if defaults.string(forKey: UIAppearancePreference.storageKey) != appearance.rawValue {
      defaults.set(appearance.rawValue, forKey: UIAppearancePreference.storageKey)
    }
    if defaults.string(forKey: UILanguagePreference.storageKey) != language.rawValue {
      defaults.set(language.rawValue, forKey: UILanguagePreference.storageKey)
    }
  }
}
