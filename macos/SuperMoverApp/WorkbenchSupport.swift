import SwiftUI

enum GateState: String, CaseIterable {
  case pass
  case pending
  case review
  case blocked
  case planned
  case neutral

  var title: String {
    switch self {
    case .pass:
      return "pass"
    case .pending:
      return "pending"
    case .review:
      return "review"
    case .blocked:
      return "blocked"
    case .planned:
      return "planned"
    case .neutral:
      return "not checked"
    }
  }

  func localizedTitle(using localization: AppChromeLocalization) -> String {
    switch self {
    case .pass:
      return localization.text("Pass")
    case .pending:
      return localization.text("Pending")
    case .review:
      return localization.text("Review")
    case .blocked:
      return localization.text("Blocked")
    case .planned:
      return localization.text("Planned")
    case .neutral:
      return localization.text("Not checked")
    }
  }

  var color: Color {
    switch self {
    case .pass:
      return SMColor.green
    case .pending, .neutral:
      return SMColor.secondaryText
    case .review:
      return SMColor.amber
    case .blocked:
      return SMColor.red
    case .planned:
      return SMColor.blue
    }
  }
}

extension EvidenceRunwayState {
  var gateState: GateState {
    switch self {
    case .pass:
      return .pass
    case .pending:
      return .pending
    case .review:
      return .review
    }
  }
}
