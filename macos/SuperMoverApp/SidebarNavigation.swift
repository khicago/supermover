import SwiftUI

enum AppSection: String, CaseIterable, Identifiable, Equatable {
  case setup
  case controlRoom
  case devices
  case pairing
  case transfer
  case sync
  case verification
  case evidence
  case driftReview
  case taskDispatch
  case settings

  var id: String { rawValue }

  var title: String {
    switch self {
    case .setup: return "Prepare"
    case .controlRoom: return "Home"
    case .devices: return "Connect"
    case .pairing: return "Pairing"
    case .transfer: return "Move"
    case .sync: return "Sync"
    case .verification: return "Verify & Repair"
    case .evidence: return "Evidence Vault"
    case .driftReview: return "Drift Review"
    case .taskDispatch: return "Task Dispatch"
    case .settings: return "Settings"
    }
  }

  var localizationKey: AppChromeLocalization.Key {
    switch self {
    case .setup: return .sidebarSetupTitle
    case .controlRoom: return .sidebarHomeTitle
    case .devices: return .sidebarDevicesTitle
    case .transfer: return .sidebarTransferTitle
    case .verification: return .sidebarVerificationTitle
    case .evidence: return .sidebarEvidenceTitle
    case .taskDispatch: return .sidebarTaskDispatchTitle
    case .settings: return .settingsTitle
    case .pairing, .sync, .driftReview:
      return ownerSection.localizationKey
    }
  }

  func localizedTitle(using localization: AppChromeLocalization) -> String {
    localization.text(localizationKey)
  }

  var heading: String {
    switch self {
    case .setup: return "Prepare"
    case .controlRoom: return "Migration Control Room"
    case .devices: return "Connect"
    case .pairing: return "Pairing"
    case .transfer: return "Move"
    case .sync: return "Incremental Sync"
    case .verification: return "Verify & Repair"
    case .evidence: return "Evidence Vault"
    case .driftReview: return "Drift Review"
    case .taskDispatch: return "Task Dispatch"
    case .settings: return "Settings"
    }
  }

  var subtitle: String {
    switch self {
    case .setup:
      return
        "Prepare this Mac's role and migration config file before running explicit commands."
    case .controlRoom:
      return "Overview, safety gates, next action, and deep links into owner pages."
    case .devices:
      return "Device state, pairing trust, receiver readiness, and connection diagnostics."
    case .pairing:
      return "Confirm target trust before enabling receiver transfer."
    case .transfer:
      return "Operate one-time transfer, dry-run, changed-file queue, and foreground sync modes."
    case .sync:
      return
        "Operate the durable changed-file queue and foreground sync loops without implying background daemon behavior."
    case .verification:
      return "Verify target content, inspect drift, and run explicit repair/prune decisions."
    case .evidence:
      return "Inspect durable receipts, reports, health and lifecycle logs."
    case .driftReview:
      return "Persist, review, reconcile and prune target drift explicitly."
    case .taskDispatch:
      return "Run wired CLI tasks through explicit migration config inputs and role gates."
    case .settings:
      return "Configure command inputs without hiding migration config policy."
    }
  }

  var icon: String {
    switch self {
    case .setup: return "person.crop.circle.badge.checkmark"
    case .controlRoom: return "house.fill"
    case .devices: return "point.3.connected.trianglepath.dotted"
    case .pairing: return "link"
    case .transfer: return "arrow.right.arrow.left.square"
    case .sync: return "arrow.triangle.2.circlepath"
    case .verification: return "checkmark.seal"
    case .evidence: return "doc.text.magnifyingglass"
    case .driftReview: return "exclamationmark.arrow.triangle.2.circlepath"
    case .taskDispatch: return "terminal.fill"
    case .settings: return "slider.horizontal.3"
    }
  }

  func availability(for role: WorkbenchRole) -> SectionAvailability {
    switch self {
    case .setup, .controlRoom, .devices, .verification, .evidence, .taskDispatch, .settings:
      return .available
    case .pairing:
      switch role {
      case .source, .target:
        return .available
      case .observer:
        return .readOnly(
          "Observer can inspect pairing evidence but cannot browse, advertise, serve, or pair devices."
        )
      }
    case .transfer:
      return role == .source
        ? .available
        : .roleGated(
          "Bounded transfer execution is source-owned; this role should inspect evidence instead.")
    case .sync:
      return role == .source
        ? .available
        : .readOnly(
          "This role can inspect sync queue evidence, but source owns queue mutation and sync execution."
        )
    case .driftReview:
      return role == .source
        ? .available
        : .readOnly(
          "This role can inspect drift/prune evidence; mutation stays source/operator controlled.")
    }
  }

  var ownerSection: AppSection {
    switch self {
    case .pairing:
      return .devices
    case .sync:
      return .transfer
    case .driftReview:
      return .verification
    case .setup, .controlRoom, .devices, .transfer, .verification, .evidence, .taskDispatch, .settings:
      return self
    }
  }

  var showsFixedOwnerModeStrip: Bool {
    switch self {
    case .devices, .pairing, .transfer, .sync, .verification, .driftReview:
      return true
    case .setup, .controlRoom, .evidence, .taskDispatch, .settings:
      return false
    }
  }

  static let homeSection: AppSection = .controlRoom

  static let sidebarGroups: [SidebarNavigationGroup] = [
    SidebarNavigationGroup(
      id: "workflow",
      title: "Workflow",
      sections: [.setup, .devices, .transfer, .verification]
    ),
    SidebarNavigationGroup(
      id: "evidence",
      title: "Evidence",
      sections: [.evidence]
    ),
    SidebarNavigationGroup(
      id: "system",
      title: "System",
      sections: [.taskDispatch, .settings]
    ),
  ]

  static var topLevelNavigationSections: [AppSection] {
    [homeSection] + sidebarGroups.flatMap(\.sections)
  }

  static func localizedSidebarGroups(using localization: AppChromeLocalization) -> [SidebarNavigationGroup] {
    sidebarGroups.map { group in
      SidebarNavigationGroup(
        id: group.id,
        title: group.localizedTitle(using: localization),
        sections: group.sections
      )
    }
  }
}

struct SidebarNavigationGroup: Identifiable, Equatable {
  let id: String
  let title: String
  let sections: [AppSection]

  var localizationKey: AppChromeLocalization.Key? {
    switch id {
    case "workflow":
      return .sidebarWorkflowGroupTitle
    case "evidence":
      return .sidebarEvidenceGroupTitle
    case "system":
      return .sidebarSystemGroupTitle
    default:
      return nil
    }
  }

  func localizedTitle(using localization: AppChromeLocalization) -> String {
    guard let localizationKey else {
      return title
    }
    return localization.text(localizationKey)
  }
}

enum SectionAvailability: Equatable {
  case available
  case planned(String)
  case readOnly(String)
  case roleGated(String)

  var label: String {
    switch self {
    case .available:
      return "available"
    case .planned:
      return "planned"
    case .readOnly:
      return "read-only"
    case .roleGated:
      return "role-gated"
    }
  }

  var detail: String {
    switch self {
    case .available:
      return "This section has current app-backed actions or evidence views for the selected role."
    case let .planned(detail), let .readOnly(detail), let .roleGated(detail):
      return detail
    }
  }

  var tint: Color {
    switch self {
    case .available:
      return SMColor.green
    case .planned:
      return SMColor.blue
    case .readOnly:
      return SMColor.secondaryText
    case .roleGated:
      return SMColor.amber
    }
  }

  var icon: String {
    switch self {
    case .available:
      return "checkmark.circle.fill"
    case .planned:
      return "sparkles"
    case .readOnly:
      return "eye"
    case .roleGated:
      return "lock.fill"
    }
  }
}

struct SidebarRow: View {
  let section: AppSection
  let availability: SectionAvailability
  let isSelected: Bool
  let localization: AppChromeLocalization
  let action: () -> Void

  var body: some View {
    Button(action: action) {
      HStack(spacing: 11) {
        Image(systemName: section.icon)
          .font(.system(size: 13, weight: .semibold))
          .frame(width: 18)
        Text(section.localizedTitle(using: localization))
          .font(.system(size: 13, weight: isSelected ? .semibold : .medium))
        Spacer()
        if availability != .available {
          Image(systemName: availability.icon)
            .font(.system(size: 10, weight: .semibold))
            .foregroundStyle(availability.tint)
            .frame(width: 20, height: 20)
            .background(isSelected ? Color.clear : availability.tint.opacity(0.08))
            .clipShape(RoundedRectangle(cornerRadius: 7, style: .continuous))
            .help("\(availability.label): \(availability.detail)")
        }
      }
      .foregroundStyle(isSelected ? SMColor.primaryText : SMColor.secondaryText)
      .padding(.horizontal, 12)
      .padding(.vertical, 10)
      .background(isSelected ? SMColor.card.opacity(0.95) : Color.clear)
      .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
    }
    .buttonStyle(.plain)
    .help(section.subtitle)
  }
}
