import Foundation

extension EvidenceArtifactFamily {
  func localizedTitle(using localization: AppChromeLocalization) -> String {
    switch self {
    case .profile:
      return localization.text("Config")
    case .pairing:
      return localization.text("Pairing")
    case .session:
      return localization.text("Session")
    case .networkTransfer:
      return localization.text("Network transfer")
    case .warning:
      return localization.text("Warning")
    case .deleted:
      return localization.text("Soft delete")
    case .drift:
      return localization.text("Drift")
    case .pruneApproval:
      return localization.text("Prune approval")
    case .pruneReceipt:
      return localization.text("Prune receipt")
    case .reconcileReceipt:
      return localization.text("Reconcile receipt")
    case .daemon:
      return localization.text("Daemon")
    case .daemonEvent:
      return localization.text("Daemon event")
    case .incrementalSyncQueue:
      return localization.text("Incremental sync queue")
    case .incrementalSyncRun:
      return localization.text("Incremental sync run")
    case .agentInfluence:
      return localization.text("Agent influence")
    case .historyIndex:
      return localization.text("History index")
    case .recoveryState:
      return localization.text("Recovery state")
    case .unknownControl:
      return localization.text("Unknown control artifact")
    }
  }

  func localizedStageLabel(using localization: AppChromeLocalization) -> String {
    switch self {
    case .profile:
      return localization.text("Config")
    case .pairing:
      return localization.text("Pairing")
    case .session, .networkTransfer:
      return localization.text("Transfer")
    case .warning:
      return localization.text("Warning")
    case .deleted, .pruneApproval, .pruneReceipt:
      return localization.text("Prune")
    case .drift, .reconcileReceipt:
      return localization.text("Review")
    case .daemon, .daemonEvent:
      return localization.text("Daemon")
    case .incrementalSyncQueue, .incrementalSyncRun:
      return localization.text("Sync")
    case .agentInfluence, .historyIndex, .recoveryState, .unknownControl:
      return localization.text("Control")
    }
  }
}
