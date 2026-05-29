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
