import AppKit
import Combine
import Foundation

enum SuperMoverCLIError: LocalizedError {
    case repoRootNotFound
    case bundledBinaryMissing(String)
    case developmentBuildFailed(String)

    var errorDescription: String? {
        switch self {
        case .repoRootNotFound:
            return "Could not locate the SuperMover repo root or a bundled supermover binary."
        case let .bundledBinaryMissing(path):
            return "Bundled supermover binary is missing or not executable: \(path)"
        case let .developmentBuildFailed(detail):
            return "Could not build the development supermover binary: \(detail)"
        }
    }
}

struct CLIInvocation {
    let executableURL: URL
    let arguments: [String]
    let workingDirectoryURL: URL
}

struct CLIProvenance: Equatable {
    enum Mode: String {
        case bundled
        case development
        case unavailable

        var title: String {
            switch self {
            case .bundled:
                return "bundled"
            case .development:
                return "development"
            case .unavailable:
                return "unavailable"
            }
        }
    }

    enum ReadinessLevel: String {
        case pass
        case review
        case blocked
    }

    let mode: Mode
    let readinessLevel: ReadinessLevel
    let executablePath: String
    let workingDirectoryPath: String
    let bundleIdentifier: String
    let appVersion: String
    let provenancePath: String?
    let provenanceStatus: String
    let bundleCommit: String?
    let bundledCLIVersion: String?
    let buildProfile: String?
    let signing: String?
    let gitDirty: Bool?
    let builtAt: String?
    let readiness: String
    let detail: String
}

struct AcceptanceInstalledAppLaunchPreview: Equatable {
    enum State: Equatable {
        case pass
        case review
        case blocked
    }

    let machine: String
    let state: State
    let detail: String

    var title: String {
        "\(machine.capitalized) installed-app launch gate"
    }
}

struct TaskRunGate: Equatable {
    let isRunnable: Bool
    let note: String?

    static let runnable = TaskRunGate(isRunnable: true, note: nil)

    static func blocked(_ note: String) -> TaskRunGate {
        TaskRunGate(isRunnable: false, note: note)
    }
}

enum SuperMoverTaskCategory: String, CaseIterable, Identifiable {
    case all = "All"
    case runtime = "Runtime"
    case profile = "Profile"
    case local = "Local"
    case review = "Review"
    case pairing = "Pairing"
    case network = "Network"
    case sync = "Sync"
    case foreground = "Foreground"

    var id: String { rawValue }

    var title: String {
        switch self {
        case .profile:
            return "Config"
        default:
            return rawValue
        }
    }
}

enum SuperMoverTaskKind: String, CaseIterable, Identifiable {
    case version = "Version"
    case profileInit = "Profile Init"
    case lintProfile = "Profile Lint"
    case profileSetTarget = "Profile Set Target"
    case profileSetNetwork = "Profile Set Network"
    case profileAdoptPairing = "Profile Adopt Pairing"
    case dryRun = "Dry Run Publish"
    case publish = "Publish"
    case verify = "Verify"
    case health = "Health"
    case report = "Report"
    case status = "Status"
    case driftList = "Drift List"
    case driftAcknowledge = "Drift Acknowledge"
    case driftResolve = "Drift Resolve"
    case driftExpire = "Drift Expire"
    case recoverDryRun = "Recover Dry Run"
    case driftRecord = "Drift Record"
    case pruneReview = "Prune Review"
    case pruneApprovals = "Prune Approvals"
    case pruneApprove = "Prune Approve"
    case pruneSupersede = "Prune Supersede"
    case reconcilePlan = "Reconcile Plan"
    case reconcileApply = "Reconcile Apply"
    case discoverAddress = "Discover Address"
    case discoverBrowse = "Discover Browse"
    case discoverAdvertise = "Discover Advertise"
    case pair = "Pair"
    case networkDryRun = "Network Push Dry Run"
    case networkPush = "Network Push"
    case syncQueueEnqueue = "Sync Queue Enqueue"
    case syncQueueStatus = "Sync Queue Status"
    case syncQueueList = "Sync Queue List"
    case syncQueueReady = "Sync Queue Ready"
    case syncQueueCancel = "Sync Queue Cancel"
    case syncQueueFail = "Sync Queue Fail"
    case syncRun = "Sync Run"
    case syncLoop = "Sync Loop"
    case syncWatch = "Sync Watch"
    case syncNetworkRun = "Sync Network Run"
    case syncNetworkDiscoverRun = "Sync Network Discover Run"
    case syncNetworkLoop = "Sync Network Loop"
    case serve = "Serve"
    case daemonInstall = "Daemon Install"
    case daemonRun = "Daemon Run"
    case daemonStatus = "Daemon Status"
    case daemonLogs = "Daemon Logs"
    case daemonRestart = "Daemon Restart"
    case daemonStop = "Daemon Stop"
    case dashboard = "Dashboard"

    var id: String { rawValue }

    var displayTitle: String {
        switch self {
        case .profileInit:
            return "Create Config File"
        case .lintProfile:
            return "Lint Config"
        case .profileSetTarget:
            return "Update Config Target"
        case .profileSetNetwork:
            return "Update Config Network"
        case .profileAdoptPairing:
            return "Adopt Pairing Receipt"
        default:
            return rawValue
        }
    }

    var taskCategory: SuperMoverTaskCategory {
        switch self {
        case .version:
            return .runtime
        case .profileInit, .lintProfile, .profileSetTarget, .profileSetNetwork, .profileAdoptPairing:
            return .profile
        case .dryRun, .publish, .verify, .health, .report, .status, .driftList, .recoverDryRun:
            return .local
        case .driftRecord, .driftAcknowledge, .driftResolve, .driftExpire, .pruneReview, .pruneApprovals, .pruneApprove, .pruneSupersede, .reconcilePlan, .reconcileApply:
            return .review
        case .discoverAddress, .discoverBrowse, .discoverAdvertise, .pair:
            return .pairing
        case .networkDryRun, .networkPush:
            return .network
        case .syncQueueEnqueue, .syncQueueStatus, .syncQueueList, .syncQueueReady,
             .syncQueueCancel, .syncQueueFail, .syncRun, .syncLoop, .syncWatch,
             .syncNetworkRun, .syncNetworkDiscoverRun, .syncNetworkLoop:
            return .sync
        case .serve, .daemonInstall, .daemonRun, .daemonStatus, .daemonLogs, .daemonRestart, .daemonStop, .dashboard:
            return .foreground
        }
    }

    var category: String {
        taskCategory.title
    }

    static func tasks(in category: SuperMoverTaskCategory) -> [SuperMoverTaskKind] {
        switch category {
        case .all:
            return allCases
        default:
            return allCases.filter { $0.taskCategory == category }
        }
    }

    var requiresProfile: Bool {
        switch self {
        case .version, .discoverAddress, .discoverBrowse:
            return false
        default:
            return true
        }
    }

    var requiresExistingProfile: Bool {
        requiresProfile && self != .profileInit
    }

    var summary: String {
        switch self {
        case .version:
            return "Read the selected supermover binary version for provenance checks."
        case .profileInit:
            return "Create a migration config file through the CLI after app-side source and target path readiness checks."
        case .lintProfile:
            return "Validate migration config SSOT and fail fast on policy errors."
        case .profileSetTarget:
            return "Persist the selected target path into the migration config after app-side readiness checks."
        case .profileSetNetwork:
            return "Persist reviewed receiver and local TLS identity material into migration config SSOT."
        case .profileAdoptPairing:
            return "Import an exported pairing receipt into the target migration config and target control-plane receipt store."
        case .dryRun:
            return "Preflight local migration without mutating target state."
        case .publish:
            return "Run a one-way local publish with a session id."
        case .verify:
            return "Verify published target state against manifest evidence."
        case .health:
            return "Read health and recovery evidence from the target control plane."
        case .report:
            return "Read the current operator review surface."
        case .status:
            return "Read compact current migration config and target status."
        case .driftList:
            return "Read live target drift without persisting it."
        case .driftAcknowledge:
            return "Add review metadata to one persisted drift record."
        case .driftResolve:
            return "Resolve one persisted drift record after fresh detector evidence is clean."
        case .driftExpire:
            return "Retire one stale persisted drift record without claiming target restoration."
        case .recoverDryRun:
            return "Inspect local recovery actions without mutation."
        case .driftRecord:
            return "Persist current live drift findings as review records."
        case .pruneReview:
            return "Read current prune review truth."
        case .pruneApprovals:
            return "Read current-scope prune approval inventory without mutating approval or target state."
        case .pruneApprove:
            return "Write a prune approval artifact from fresh dry-run evidence without deleting target files."
        case .pruneSupersede:
            return "Supersede one existing prune approval artifact without writing prune receipts or deleting target files."
        case .reconcilePlan:
            return "Build a narrow persisted-drift repair plan without mutation, optionally filtered by persisted drift ids."
        case .reconcileApply:
            return "Apply one or more selected narrow persisted-drift repairs with explicit intent."
        case .discoverAddress:
            return "Resolve one explicit low-information target address hint. This is not trust."
        case .discoverBrowse:
            return "Browse unauthenticated LAN discovery hints and classify duplicates or ambiguity."
        case .discoverAdvertise:
            return "Advertise a bounded low-information target hint from migration config policy."
        case .pair:
            return "Verify the target code, pin target identity, and write pairing receipt evidence."
        case .networkDryRun:
            return "Validate paired config-backed network transfer without contacting receiver."
        case .networkPush:
            return "Run the current bounded config-backed network push path."
        case .syncQueueEnqueue:
            return "Snapshot migration config roots into the durable changed-file queue; this does not copy or watch files."
        case .syncQueueStatus:
            return "Read durable changed-file queue counters from the selected config target."
        case .syncQueueList:
            return "List every persisted queue entry, including terminal and retry states."
        case .syncQueueReady:
            return "List queue entries that are currently ready for a bounded sync pass."
        case .syncQueueCancel:
            return "Mark one durable queue entry canceled with an operator reason."
        case .syncQueueFail:
            return "Mark one durable queue entry failed as terminal operator review evidence."
        case .syncRun:
            return "Run one bounded local incremental sync pass with an explicit session id."
        case .syncLoop:
            return "Run a foreground local polling loop; it is stoppable here and not a detached daemon."
        case .syncWatch:
            return "Run a foreground OS watcher for selected config source roots."
        case .syncNetworkRun:
            return "Run one bounded config-pinned mTLS incremental network pass."
        case .syncNetworkDiscoverRun:
            return "Run one LAN-discovery-gated network pass; discovery is only an availability gate, not trust."
        case .syncNetworkLoop:
            return "Run a foreground config-pinned network polling loop."
        case .serve:
            return "Run foreground pairing plus optional receiver service."
        case .daemonInstall:
            return "Write foreground daemon lifecycle install evidence; this does not install an OS service."
        case .daemonRun:
            return "Run the config-backed foreground daemon in this supervised app process."
        case .daemonStatus:
            return "Read durable foreground daemon lifecycle state."
        case .daemonLogs:
            return "Read durable daemon lifecycle events."
        case .daemonRestart:
            return "Persist a foreground daemon restart intent with an explicit operator reason."
        case .daemonStop:
            return "Persist a foreground daemon stop intent with an explicit operator reason."
        case .dashboard:
            return "Launch the loopback-only read-only target integrity page."
        }
    }

    var longRunning: Bool {
        switch self {
        case .serve, .daemonRun, .dashboard, .syncLoop, .syncWatch, .syncNetworkLoop:
            return true
        default:
            return false
        }
    }

    var invalidatesStructuredEvidenceOnLaunch: Bool {
        switch self {
        case .version:
            return false
        case .profileInit,
             .profileSetTarget,
             .profileSetNetwork,
             .profileAdoptPairing,
             .publish,
             .driftRecord,
             .driftAcknowledge,
             .driftResolve,
             .driftExpire,
             .pruneApprove,
             .pruneSupersede,
             .reconcileApply,
             .pair,
             .networkPush,
             .syncQueueEnqueue,
             .syncQueueCancel,
             .syncQueueFail,
             .syncRun,
             .syncLoop,
             .syncWatch,
             .syncNetworkRun,
             .syncNetworkDiscoverRun,
             .syncNetworkLoop,
             .serve,
             .daemonInstall,
             .daemonRun,
             .daemonRestart,
             .daemonStop:
            return true
        case .lintProfile,
             .dryRun,
             .verify,
             .health,
             .report,
             .status,
             .driftList,
             .recoverDryRun,
             .pruneReview,
             .pruneApprovals,
             .reconcilePlan,
             .discoverAddress,
             .discoverBrowse,
             .discoverAdvertise,
             .networkDryRun,
             .syncQueueStatus,
             .syncQueueList,
             .syncQueueReady,
             .daemonStatus,
             .daemonLogs,
             .dashboard:
            return false
        }
    }

    var supervisedSlot: SupervisedProcessSlot {
        switch self {
        case .serve:
            return .targetServe
        case .daemonRun:
            return .foregroundDaemon
        case .dashboard:
            return .targetDashboard
        case .syncLoop:
            return .sourceSyncLoop
        case .syncWatch:
            return .sourceSyncWatch
        case .syncNetworkLoop:
            return .sourceNetworkLoop
        default:
            return .foregroundAction
        }
    }

    func buildArguments(using input: TaskInput) -> [String] {
        switch self {
        case .version:
            return ["version"]
        case .profileInit:
            return ["profile", "init", "--profile", input.profilePath, "--source", input.sourceRootPath, "--target", input.targetRootPath, "--id", input.requiredProfileID, "--name", input.requiredProfileName] + input.optionalTargetIDArgs
        case .lintProfile:
            return ["profile", "lint", "--profile", input.profilePath]
        case .profileSetTarget:
            return ["profile", "set-target", "--profile", input.profilePath, "--target", input.targetRootPath] + input.optionalTargetIDArgs + input.optionalTargetNameArgs
        case .profileSetNetwork:
            return ["profile", "set-network", "--profile", input.profilePath] + input.profileNetwork.setArguments
        case .profileAdoptPairing:
            return ["profile", "adopt-pairing", "--profile", input.profilePath] + input.pairingReceipt.adoptArguments
        case .dryRun:
            return ["push", "--profile", input.profilePath, "--dry-run"]
        case .publish:
            return ["push", "--profile", input.profilePath, "--session", input.requiredSessionID]
        case .verify:
            return ["verify", "--profile", input.profilePath, "--format", "json"] + input.optionalSessionArgs
        case .health:
            return ["health", "--profile", input.profilePath, "--format", "json"]
        case .report:
            return ["report", "--profile", input.profilePath, "--format", "json"] + input.optionalSessionArgs
        case .status:
            return ["status", "--profile", input.profilePath, "--format", "json"]
        case .driftList:
            return ["drift", "list", "--profile", input.profilePath, "--format", "json"] + input.optionalSessionArgs
        case .driftAcknowledge:
            return ["drift", "acknowledge", "--profile", input.profilePath, "--id", input.requiredSingleDriftID, "--reason", input.requiredReason, "--format", "json"] + input.optionalReviewerArgs
        case .driftResolve:
            return ["drift", "resolve", "--profile", input.profilePath, "--id", input.requiredSingleDriftID, "--reason", input.requiredReason, "--format", "json"] + input.optionalReviewerArgs
        case .driftExpire:
            return ["drift", "expire", "--profile", input.profilePath, "--id", input.requiredSingleDriftID, "--reason", input.requiredReason, "--format", "json"] + input.optionalReviewerArgs
        case .recoverDryRun:
            return ["recover", "--profile", input.profilePath, "--dry-run", "--format", "json"] + input.optionalSessionArgs
        case .driftRecord:
            return ["drift", "record", "--profile", input.profilePath, "--format", "json"] + input.optionalSessionArgs
        case .pruneReview:
            return ["prune", "review", "--profile", input.profilePath, "--format", "json"] + input.optionalSessionArgs
        case .pruneApprovals:
            return ["prune", "approvals", "--profile", input.profilePath, "--format", "json"]
        case .pruneApprove:
            return ["prune", "approve", "--profile", input.profilePath, "--id", input.requiredApprovalID, "--reason", input.requiredReason, "--reviewer", input.requiredReviewer, "--format", "json"] + input.requiredSoftDeleteArgs + input.optionalExpiresAtArgs
        case .pruneSupersede:
            return ["prune", "supersede", "--profile", input.profilePath, "--id", input.requiredApprovalID, "--reason", input.requiredReason, "--reviewer", input.requiredReviewer, "--format", "json"]
        case .reconcilePlan:
            return ["reconcile", "plan", "--profile", input.profilePath, "--format", "json"] + input.optionalDriftIDArgs + input.optionalSessionArgs
        case .reconcileApply:
            return ["reconcile", "apply", "--profile", input.profilePath, "--apply", "--reason", input.requiredReason, "--format", "json"] + input.optionalDriftIDArgs + input.optionalSessionArgs + input.optionalReviewerArgs
        case .discoverAddress:
            return ["discover", "--address", input.requiredPairingTargetAddress, "--timeout", input.requiredDiscoveryBrowseTimeout, "--format", "json"]
        case .discoverBrowse:
            return ["discover", "browse", "--listen", input.requiredDiscoveryBrowseListen, "--timeout", input.requiredDiscoveryBrowseTimeout, "--format", "json"]
        case .discoverAdvertise:
            return ["discover", "advertise", "--profile", input.profilePath] + input.optionalDiscoveryAdvertiseListenArgs + ["--dest", input.requiredDiscoveryAdvertiseDestination, "--interval", input.requiredDiscoveryAdvertiseInterval, "--duration", input.requiredDiscoveryAdvertiseDuration, "--format", "json"]
        case .pair:
            return ["pair", "--profile", input.profilePath, "--target", input.requiredPairingTargetAddress, "--verification-code", input.requiredPairingVerificationCode, "--method", input.requiredPairingMethod, "--timeout", input.requiredPairingTimeout] + input.pairingReceipt.exportArguments
        case .networkDryRun:
            return ["push", "--network", "--profile", input.profilePath, "--dry-run", "--format", "json"] + input.optionalSessionArgs
        case .networkPush:
            return ["push", "--network", "--profile", input.profilePath, "--format", "json", "--session", input.requiredSessionID]
        case .syncQueueEnqueue:
            return ["sync", "queue", "enqueue", "--profile", input.profilePath, "--format", "json"]
        case .syncQueueStatus:
            return ["sync", "queue", "status", "--profile", input.profilePath, "--format", "json"]
        case .syncQueueList:
            return ["sync", "queue", "list", "--profile", input.profilePath, "--format", "json"]
        case .syncQueueReady:
            return ["sync", "queue", "ready", "--profile", input.profilePath, "--format", "json"]
        case .syncQueueCancel:
            return ["sync", "queue", "cancel", "--profile", input.profilePath, "--id", input.requiredQueueEntryID, "--reason", input.requiredReason, "--format", "json"]
        case .syncQueueFail:
            return ["sync", "queue", "fail", "--profile", input.profilePath, "--id", input.requiredQueueEntryID, "--reason", input.requiredReason, "--format", "json"]
        case .syncRun:
            return ["sync", "run", "--profile", input.profilePath, "--session", input.requiredSessionID, "--retry-backoff", input.requiredSyncRetryBackoff, "--format", "json"]
        case .syncLoop:
            return ["sync", "loop", "--profile", input.profilePath, "--session-prefix", input.requiredSessionPrefix, "--interval", input.requiredSyncInterval, "--max-runs", input.requiredSyncMaxRuns, "--retry-backoff", input.requiredSyncRetryBackoff, "--format", "json"]
        case .syncWatch:
            return ["sync", "watch", "--profile", input.profilePath, "--session-prefix", input.requiredSessionPrefix, "--settle", input.requiredSyncSettle, "--max-events", input.requiredSyncMaxEvents, "--retry-backoff", input.requiredSyncRetryBackoff, "--format", "json"]
        case .syncNetworkRun:
            return ["sync", "network", "run", "--profile", input.profilePath, "--session", input.requiredSessionID, "--retry-backoff", input.requiredSyncRetryBackoff, "--format", "json"]
        case .syncNetworkDiscoverRun:
            return ["sync", "network", "discover-run", "--profile", input.profilePath, "--session", input.requiredSessionID, "--listen", input.requiredSyncDiscoveryListen, "--timeout", input.requiredSyncDiscoveryTimeout, "--retry-backoff", input.requiredSyncRetryBackoff, "--format", "json"]
        case .syncNetworkLoop:
            return ["sync", "network", "loop", "--profile", input.profilePath, "--session-prefix", input.requiredSessionPrefix, "--interval", input.requiredSyncInterval, "--max-runs", input.requiredSyncMaxRuns, "--retry-backoff", input.requiredSyncRetryBackoff, "--format", "json"]
        case .serve:
            return ["serve", "--profile", input.profilePath, "--listen", input.listenAddress]
        case .daemonInstall:
            return ["daemon", "install", "--profile", input.profilePath]
        case .daemonRun:
            return ["daemon", "run", "--foreground", "--profile", input.profilePath, "--listen", input.listenAddress]
        case .daemonStatus:
            return ["daemon", "status", "--profile", input.profilePath, "--format", "json"]
        case .daemonLogs:
            return ["daemon", "logs", "--profile", input.profilePath, "--format", "json", "--tail", "20"]
        case .daemonRestart:
            return ["daemon", "restart", "--profile", input.profilePath, "--reason", input.requiredReason, "--format", "json"]
        case .daemonStop:
            return ["daemon", "stop", "--profile", input.profilePath, "--reason", input.requiredReason]
        case .dashboard:
            return ["dashboard", "--profile", input.profilePath, "--listen", input.listenAddress]
        }
    }
}

struct TaskInput {
    static let defaultProfileID = "profile-local"
    static let defaultProfileName = "Local profile"

    var profilePath: String
    var sourceRootPath: String
    var targetRootPath: String
    var profileID: String
    var profileName: String
    var targetID: String
    var targetName: String
    var sessionID: String
    var sessionPrefix: String
    var queueEntryID: String
    var syncRetryBackoff: String
    var syncInterval: String
    var syncMaxRuns: String
    var syncSettle: String
    var syncMaxEvents: String
    var syncDiscoveryListen: String
    var syncDiscoveryTimeout: String
    var listenAddress: String
    var pairingTargetAddress: String
    var pairingVerificationCode: String
    var pairingMethod: String
    var pairingTimeout: String
    var pairingReceipt: PairingReceiptDraft = PairingReceiptDraft()
    var profileNetwork: ProfileNetworkDraft = ProfileNetworkDraft()
    var discoveryBrowseListen: String
    var discoveryBrowseTimeout: String
    var discoveryAdvertiseListen: String
    var discoveryAdvertiseDestination: String
    var discoveryAdvertiseDuration: String
    var discoveryAdvertiseInterval: String
    var driftIDsInput: String
    var approvalID: String
    var softDeleteIDsInput: String
    var expiresAt: String
    var reason: String
    var reviewer: String

    var requiredSessionID: String {
        sessionID.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var requiredSessionPrefix: String {
        sessionPrefix.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var requiredQueueEntryID: String {
        queueEntryID.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var requiredSyncRetryBackoff: String {
        let trimmed = syncRetryBackoff.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "1m" : trimmed
    }

    var requiredSyncInterval: String {
        let trimmed = syncInterval.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "1m" : trimmed
    }

    var requiredSyncMaxRuns: String {
        let trimmed = syncMaxRuns.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "0" : trimmed
    }

    var requiredSyncSettle: String {
        let trimmed = syncSettle.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "250ms" : trimmed
    }

    var requiredSyncMaxEvents: String {
        let trimmed = syncMaxEvents.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "0" : trimmed
    }

    var requiredSyncDiscoveryListen: String {
        let trimmed = syncDiscoveryListen.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "0.0.0.0:39394" : trimmed
    }

    var requiredSyncDiscoveryTimeout: String {
        let trimmed = syncDiscoveryTimeout.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "2s" : trimmed
    }

    var requiredProfileID: String {
        let trimmed = profileID.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? Self.defaultProfileID : trimmed
    }

    var requiredProfileName: String {
        let trimmed = profileName.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? Self.defaultProfileName : trimmed
    }

    var optionalTargetIDArgs: [String] {
        let trimmed = targetID.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            return []
        }
        return ["--target-id", trimmed]
    }

    var optionalTargetNameArgs: [String] {
        let trimmed = targetName.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            return []
        }
        return ["--name", trimmed]
    }

    var driftIDs: [String] {
        driftIDsInput
            .components(separatedBy: CharacterSet(charactersIn: ", \n\t"))
            .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
    }

    var requiredSingleDriftID: String {
        driftIDs.first ?? ""
    }

    var requiredReason: String {
        reason.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var requiredApprovalID: String {
        approvalID.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var requiredPairingTargetAddress: String {
        pairingTargetAddress.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var requiredPairingVerificationCode: String {
        pairingVerificationCode.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var requiredPairingMethod: String {
        let trimmed = pairingMethod.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "sas" : trimmed
    }

    var requiredPairingTimeout: String {
        let trimmed = pairingTimeout.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "5s" : trimmed
    }

    var requiredDiscoveryBrowseListen: String {
        let trimmed = discoveryBrowseListen.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "0.0.0.0:39394" : trimmed
    }

    var requiredDiscoveryBrowseTimeout: String {
        let trimmed = discoveryBrowseTimeout.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "2s" : trimmed
    }

    var requiredDiscoveryAdvertiseDestination: String {
        let trimmed = discoveryAdvertiseDestination.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "255.255.255.255:39394" : trimmed
    }

    var optionalDiscoveryAdvertiseListenArgs: [String] {
        let trimmed = discoveryAdvertiseListen.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            return []
        }
        return ["--listen", trimmed]
    }

    var requiredDiscoveryAdvertiseDuration: String {
        let trimmed = discoveryAdvertiseDuration.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "10s" : trimmed
    }

    var requiredDiscoveryAdvertiseInterval: String {
        let trimmed = discoveryAdvertiseInterval.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? "1s" : trimmed
    }

    var softDeleteIDs: [String] {
        softDeleteIDsInput
            .components(separatedBy: CharacterSet(charactersIn: ", \n\t"))
            .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
    }

    var requiredReviewer: String {
        reviewer.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var optionalSessionArgs: [String] {
        let trimmed = sessionID.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            return []
        }
        return ["--session", trimmed]
    }

    var optionalDriftIDArgs: [String] {
        driftIDs.flatMap { ["--id", $0] }
    }

    var optionalReviewerArgs: [String] {
        let trimmed = reviewer.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            return []
        }
        return ["--reviewer", trimmed]
    }

    var requiredSoftDeleteArgs: [String] {
        softDeleteIDs.flatMap { ["--soft-delete", $0] }
    }

    var optionalExpiresAtArgs: [String] {
        let trimmed = expiresAt.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            return []
        }
        return ["--expires-at", trimmed]
    }
}

enum SelectedProfilePathState: Equatable {
    case none
    case existingFile
    case newDestination
    case missingFile
    case directory

    var filesystemSummary: String? {
        switch self {
        case .none:
            return nil
        case .existingFile:
            return "Existing config file"
        case .newDestination:
            return "New config destination"
        case .missingFile:
            return "Config file missing"
        case .directory:
            return "Folder selected"
        }
    }

    var allowsExplicitCreate: Bool {
        self == .newDestination
    }
}

struct ProfileSelectionContext: Equatable {
    let title: String
    let detail: String?
    let metadata: String?
    let rawPathLabel: String
    let rawPath: String?
    let evidenceID: String?
    let pathState: SelectedProfilePathState
    let showsSourceIdentityFields: Bool
    let showsTargetIdentityFields: Bool

    var allowsExplicitCreate: Bool {
        pathState.allowsExplicitCreate
    }
}

struct SetupGuide: Equatable {
    struct Step: Equatable, Identifiable {
        let id: String
        let index: Int
        let title: String
        let detail: String
        let statusLabel: String
        let state: GateState
        let primaryActionTitle: String?
        let primaryTask: SuperMoverTaskKind?
        let secondaryActionTitle: String?
    }

    let title: String
    let subtitle: String
    let steps: [Step]
}

enum ProfileDestinationPlan: Equatable {
    case selectedOnly(note: String)
    case initialize(arguments: [String], note: String)
}

enum WorkbenchRole: String, CaseIterable, Identifiable {
    case source
    case target
    case observer

    var id: String { rawValue }

    var title: String {
        switch self {
        case .source:
            return "Source"
        case .target:
            return "Target"
        case .observer:
            return "Observer"
        }
    }

    var summary: String {
        switch self {
        case .source:
            return "Prepare source roots, config identity, pairing inputs, and bounded transfer inputs."
        case .target:
            return "Prepare target root, config evidence, listen inputs, and read-only evidence access."
        case .observer:
            return "Inspect selected target evidence without mutation or long-running services."
        }
    }

    var allowedSetup: String {
        switch self {
        case .source:
            return "create config, lint config, update target, dry-run preparation"
        case .target:
            return "target root selection, config lint, listen readiness preparation"
        case .observer:
            return "config selection, status/report/health evidence reads"
        }
    }

    func localizedTitle(using localization: AppChromeLocalization) -> String {
        switch self {
        case .source:
            return localization.text(.workbenchRoleSourceTitle)
        case .target:
            return localization.text(.workbenchRoleTargetTitle)
        case .observer:
            return localization.text(.workbenchRoleObserverTitle)
        }
    }

    func localizedSummary(using localization: AppChromeLocalization) -> String {
        switch self {
        case .source:
            return localization.text(.workbenchRoleSourceSummary)
        case .target:
            return localization.text(.workbenchRoleTargetSummary)
        case .observer:
            return localization.text(.workbenchRoleObserverSummary)
        }
    }

    func localizedAllowedSetup(using localization: AppChromeLocalization) -> String {
        switch self {
        case .source:
            return localization.text(.workbenchRoleSourceAllowedSetup)
        case .target:
            return localization.text(.workbenchRoleTargetAllowedSetup)
        case .observer:
            return localization.text(.workbenchRoleObserverAllowedSetup)
        }
    }

    func localizedSetupGuideTitle(using localization: AppChromeLocalization) -> String {
        switch self {
        case .source:
            return localization.text(.setupGuideSourceTitle)
        case .target:
            return localization.text(.setupGuideTargetTitle)
        case .observer:
            return localization.text(.setupGuideObserverTitle)
        }
    }

    func allows(task: SuperMoverTaskKind) -> Bool {
        switch self {
        case .source:
            switch task {
            case .version,
                    .profileInit, .profileSetTarget, .profileSetNetwork, .lintProfile, .dryRun,
                    .publish, .verify, .health, .report, .status, .driftList,
                    .driftAcknowledge, .driftResolve, .driftExpire, .recoverDryRun,
                    .driftRecord, .pruneReview, .pruneApprovals,
                    .pruneApprove, .pruneSupersede, .reconcilePlan,
                    .reconcileApply, .discoverAddress, .discoverBrowse, .pair,
                    .networkDryRun, .networkPush,
                    .syncQueueEnqueue, .syncQueueStatus, .syncQueueList,
                    .syncQueueReady, .syncQueueCancel, .syncQueueFail,
                    .syncRun, .syncLoop, .syncWatch, .syncNetworkRun,
                    .syncNetworkDiscoverRun, .syncNetworkLoop,
                    .daemonInstall, .daemonRun, .daemonStatus, .daemonLogs,
                    .daemonRestart, .daemonStop:
                return true
            case .profileAdoptPairing, .discoverAdvertise, .serve, .dashboard:
                return false
            }
        case .target:
            switch task {
            case .version,
                    .profileSetTarget, .profileSetNetwork, .profileAdoptPairing, .lintProfile, .verify, .health, .report, .status,
                    .driftList, .pruneReview, .pruneApprovals, .discoverAdvertise,
                    .syncQueueStatus, .syncQueueList, .syncQueueReady,
                    .serve, .daemonInstall, .daemonRun, .daemonStatus, .daemonLogs,
                    .daemonRestart, .daemonStop, .dashboard:
                return true
            default:
                return false
            }
        case .observer:
            switch task {
            case .version, .lintProfile, .verify, .health, .report, .status, .driftList,
                    .pruneReview, .pruneApprovals, .syncQueueStatus,
                    .syncQueueList, .syncQueueReady, .daemonStatus, .daemonLogs,
                    .dashboard:
                return true
            default:
                return false
            }
        }
    }
}

enum SupervisedProcessSlot: String, CaseIterable, Identifiable {
    case foregroundAction
    case sourceSyncLoop
    case sourceSyncWatch
    case sourceNetworkLoop
    case targetServe
    case foregroundDaemon
    case targetDashboard

    var id: String { rawValue }

    var title: String {
        switch self {
        case .foregroundAction:
            return "Foreground Action"
        case .sourceSyncLoop:
            return "Source Sync Loop"
        case .sourceSyncWatch:
            return "Source Sync Watch"
        case .sourceNetworkLoop:
            return "Source Network Loop"
        case .targetServe:
            return "Target Serve"
        case .foregroundDaemon:
            return "Foreground Daemon"
        case .targetDashboard:
            return "Target Dashboard"
        }
    }

    var summary: String {
        switch self {
        case .foregroundAction:
            return "One bounded command at a time; does not stop supervised target services."
        case .sourceSyncLoop:
            return "Foreground local polling queue consumer; not a detached daemon."
        case .sourceSyncWatch:
            return "Foreground OS watcher for selected source roots; stoppable from the app."
        case .sourceNetworkLoop:
            return "Foreground config-pinned mTLS network polling loop."
        case .targetServe:
            return "Foreground pairing and receiver service for the target role."
        case .foregroundDaemon:
            return "Config-backed foreground daemon; writes lifecycle evidence, not an OS-managed service."
        case .targetDashboard:
            return "Loopback read-only evidence dashboard process."
        }
    }

    var stopLabel: String {
        switch self {
        case .foregroundAction:
            return "Stop Foreground Action"
        case .sourceSyncLoop:
            return "Stop Sync Loop"
        case .sourceSyncWatch:
            return "Stop Sync Watch"
        case .sourceNetworkLoop:
            return "Stop Network Loop"
        case .targetServe:
            return "Stop Target Serve"
        case .foregroundDaemon:
            return "Terminate Foreground Daemon"
        case .targetDashboard:
            return "Stop Target Dashboard"
        }
    }

    var isLongRunning: Bool {
        switch self {
        case .foregroundAction:
            return false
        case .sourceSyncLoop, .sourceSyncWatch, .sourceNetworkLoop, .targetServe, .foregroundDaemon, .targetDashboard:
            return true
        }
    }
}

struct TaskRun: Identifiable {
    enum State {
        case idle
        case running
        case finished(Int32)
        case failedToLaunch(String)
        case cancelled
    }

    let id = UUID()
    let kind: SuperMoverTaskKind
    let slot: SupervisedProcessSlot
    let launchedAt: Date
    let commandLine: [String]
    let contextSignature: String
    var processIdentifier: Int32?
    var stdout: String
    var stderr: String
    var state: State
}

struct ProcessLifecycleEvent: Identifiable {
    let id = UUID()
    let occurredAt: Date
    let slot: SupervisedProcessSlot
    let kind: SuperMoverTaskKind
    let message: String
}

enum AppEventSeverity: String {
    case info
    case review
    case error
}

enum StructuredArtifactKind: String {
    case status
    case verify
    case sourceConsistency
    case health
    case report
    case driftList
    case discoveryHints
    case discoveryBrowse
    case discoveryAdvertise
    case daemonStatus
    case daemonLogs
    case driftRecord
    case driftMutation
    case pruneReview
    case pruneApprovals
    case pruneApprovalAuthoring
    case pruneApprovalSupersede
    case reconcile
    case syncQueue
    case syncRun
    case syncLoop
    case syncWatch
    case syncNetworkRun
    case syncNetworkDiscoverRun
    case syncNetworkLoop

    var title: String {
        switch self {
        case .status:
            return "status"
        case .verify:
            return "verify"
        case .sourceConsistency:
            return "source consistency"
        case .health:
            return "health"
        case .report:
            return "report"
        case .driftList:
            return "drift list"
        case .discoveryHints:
            return "discovery hints"
        case .discoveryBrowse:
            return "discovery browse"
        case .discoveryAdvertise:
            return "discovery advertise"
        case .daemonStatus:
            return "daemon status"
        case .daemonLogs:
            return "daemon logs"
        case .driftRecord:
            return "drift record"
        case .driftMutation:
            return "drift mutation"
        case .pruneReview:
            return "prune review"
        case .pruneApprovals:
            return "prune approvals"
        case .pruneApprovalAuthoring:
            return "prune approval authoring"
        case .pruneApprovalSupersede:
            return "prune approval supersede"
        case .reconcile:
            return "reconcile"
        case .syncQueue:
            return "sync queue"
        case .syncRun:
            return "sync run"
        case .syncLoop:
            return "sync loop"
        case .syncWatch:
            return "sync watch"
        case .syncNetworkRun:
            return "sync network run"
        case .syncNetworkDiscoverRun:
            return "sync network discover-run"
        case .syncNetworkLoop:
            return "sync network loop"
        }
    }
}

struct AppEvent: Identifiable {
    let id = UUID()
    let occurredAt: Date
    let severity: AppEventSeverity
    let title: String
    let detail: String
}

struct ArtifactReadProblem: Identifiable {
    let id = UUID()
    let occurredAt: Date
    let artifactKind: StructuredArtifactKind
    let task: SuperMoverTaskKind
    let problem: String
    let rawSample: String
}

struct DiscoveryAdvertisementSnapshot: Decodable {
    let service_type: String
    let protocol_version: String
    let ephemeral_nonce: String
    let capability_flags: [String]
}

struct DiscoveryAddressHintSnapshot: Decodable, Identifiable {
    var id: String {
        "\(address)|\(advertisement.protocol_version)|\(advertisement.ephemeral_nonce)|\(seen_at)"
    }

    let address: String
    let advertisement: DiscoveryAdvertisementSnapshot
    let seen_at: String
    let expires_at: String
    let trusted: Bool
}

struct DiscoveryCandidateSnapshot: Decodable, Identifiable {
    var id: String {
        "\(hint.id)|\(classification)|\(duplicate_count)"
    }

    let hint: DiscoveryAddressHintSnapshot
    let classification: String
    let duplicate_count: Int
    let ambiguity_reasons: [String]?

    enum CodingKeys: String, CodingKey {
        case hint
        case classification = "class"
        case duplicate_count
        case ambiguity_reasons
    }
}

struct DiscoveryBrowseSnapshot: Decodable {
    let source: String
    let listen: String
    let candidate_count: Int
    let invalid_packets: Int
    let trusted: Bool
    let candidates: [DiscoveryCandidateSnapshot]
}

struct DiscoveryAdvertiseSnapshot: Decodable {
    let status: String
    let listen: String
    let destination: String
    let service_type: String
    let protocol_version: String
    let ephemeral_nonce: String
    let capability_flags: [String]
    let trusted: Bool
    let duration: String
    let interval: String
}

struct SyncQueueSummarySnapshot: Decodable {
    struct Root: Decodable, Identifiable {
        var id: String { root }
        let root: String
        let queued: Int
        let in_flight: Int
        let backoff: Int
        let canceled: Int
        let done: Int
        let failed: Int
        let ready: Int
        let total: Int
    }

    let profile_id: String
    let target_id: String
    let queued: Int
    let in_flight: Int
    let backoff: Int
    let canceled: Int
    let done: Int
    let failed: Int
    let ready: Int
    let total: Int
    let roots: [Root]?
    let by_status: [String: Int]?
    let warning_count: Int?
    let state_path: String?
    let generated_at: String
}

struct SyncQueueEntrySnapshot: Decodable, Identifiable {
    var id: String { queueID }

    let queueID: String
    let profile_id: String?
    let target_id: String?
    let root: String?
    let path: String
    let kind: String
    let digest: String?
    let symlink_target: String?
    let mod_time: String?
    let size: Int64?
    let mode: UInt32?
    let enqueued_at: String?
    let status: String
    let attempts: Int?
    let last_error: String?
    let next_due_at: String?
    let canceled_at: String?
    let done_at: String?
    let failed_at: String?
    let updated_at: String?

    enum CodingKeys: String, CodingKey {
        case queueID = "id"
        case profile_id
        case target_id
        case root
        case path
        case kind
        case digest
        case symlink_target
        case mod_time
        case size
        case mode
        case enqueued_at
        case status
        case attempts
        case last_error
        case next_due_at
        case canceled_at
        case done_at
        case failed_at
        case updated_at
    }
}

struct SyncQueueSkippedEntrySnapshot: Decodable, Identifiable {
    var id: String { "\(root):\(path):\(reason)" }
    let root: String
    let path: String
    let reason: String
}

struct SyncQueueSnapshot: Decodable {
    let operation: String
    let mode: String
    let state: String?
    let state_path: String?
    let summary: SyncQueueSummarySnapshot
    let enqueued: [SyncQueueEntrySnapshot]?
    let skipped: [SyncQueueSkippedEntrySnapshot]?
    let entries: [SyncQueueEntrySnapshot]?
    let entry: SyncQueueEntrySnapshot?
    let reason: String?
}

struct SyncRunReceiptSnapshot: Decodable {
    let session_id: String
    let status: String
    let started_at: String?
    let finished_at: String?
    let state_path: String?
    let run_path: String?
    let ready: [SyncQueueEntrySnapshot]?
    let in_flight: [SyncQueueEntrySnapshot]?
    let published: [SyncQueueEntrySnapshot]?
    let retried: [SyncQueueEntrySnapshot]?
    let error: String?
    let recovered_in_flight: Int?
    let summary: SyncQueueSummarySnapshot
}

struct SyncRunSnapshot: Decodable {
    let operation: String
    let mode: String
    let enqueue: SyncQueueSnapshot
    let run: SyncRunReceiptSnapshot
}

struct SyncNetworkPlanSnapshot: Decodable {
    let profile_id: String
    let target_id: String
    let source_device_id: String?
    let target_device_id: String?
    let pairing_receipt_id: String?
    let session_id: String?
    let dry_run: Bool?
    let transfer: String
    let encrypted_transfer: String
    let resume: String?
    let resume_authority: String?
    let resume_outcome: String?
    let resumed_bytes: Int64?
    let files: Int
    let bytes: Int64
    let chunks: Int?
    let warnings: Int
    let status: String?
    let stage: String?
    let error_code: String?
}

struct SyncNetworkRunSnapshot: Decodable {
    let operation: String
    let mode: String
    let enqueue: SyncQueueSnapshot
    let run: SyncRunReceiptSnapshot
    let network: SyncNetworkPlanSnapshot
}

struct SyncNetworkDiscoveryGateSnapshot: Decodable {
    let status: String
    let reason: String
    let profile_address: String
    let matched_address: String?
    let matched_class: String?
    let capability_flags: [String]?
    let expires_at: String?
    let candidate_count: Int
    let invalid_packets: Int
    let trusted: Bool
}

struct SyncNetworkDiscoverRunSnapshot: Decodable {
    let operation: String
    let mode: String
    let discovery: SyncNetworkDiscoveryGateSnapshot
    let enqueue: SyncQueueSnapshot?
    let run: SyncRunReceiptSnapshot?
    let network: SyncNetworkPlanSnapshot?

    var executedRun: Bool {
        guard discovery.status == "matched", let enqueue, let run else {
            return false
        }
        return !enqueue.operation.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            && !run.session_id.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            && !run.status.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }
}

struct SyncLoopSnapshot: Decodable {
    let operation: String
    let mode: String
    let session_prefix: String
    let interval: String
    let max_runs: Int
    let status: String
    let completed_runs: Int
    let published_runs: Int
    let idle_runs: Int
    let retrying_runs: Int
    let runs: [SyncRunSnapshot]?
}

struct SyncWatchSnapshot: Decodable {
    let operation: String
    let mode: String
    let session_prefix: String
    let settle: String
    let max_events: Int
    let status: String
    let watched_roots: [String]?
    let watched_dirs: Int
    let event_batches: Int
    let events_seen: Int
    let completed_runs: Int
    let published_runs: Int
    let idle_runs: Int
    let retrying_runs: Int
    let baseline: SyncRunSnapshot?
    let runs: [SyncRunSnapshot]?
}

struct SyncNetworkLoopSnapshot: Decodable {
    let operation: String
    let mode: String
    let session_prefix: String
    let interval: String
    let max_runs: Int
    let status: String
    let completed_runs: Int
    let published_runs: Int
    let idle_runs: Int
    let retrying_runs: Int
    let network_attempts: Int
    let network_published_runs: Int
    let network_not_attempted_runs: Int
    let runs: [SyncNetworkRunSnapshot]?
}

enum EvidenceAvailability: Equatable {
    case available(String)
    case unavailable(String)

    var label: String {
        switch self {
        case .available:
            return "available"
        case .unavailable:
            return "unavailable"
        }
    }

    var detail: String {
        switch self {
        case let .available(detail), let .unavailable(detail):
            return detail
        }
    }
}

struct VerifySnapshot: Decodable {
    struct Summary: Decodable {
        let manifest_count: Int
        let manifest_entries: Int
        let files_expected: Int
        let files_verified: Int
        let warnings: Int
        let soft_deletes: Int
        let target_drifts: Int
        let artifact_problems: Int
        let error_findings: Int
        let warning_findings: Int
        let skipped_digest: Int
    }

    struct ManifestSummary: Decodable, Identifiable {
        var id: String { manifestID }

        let manifestID: String
        let session_id: String
        let root_id: String?
        let created_at: String
        let entries: Int
        let files: Int

        enum CodingKeys: String, CodingKey {
            case manifestID = "id"
            case session_id
            case root_id
            case created_at
            case entries
            case files
        }
    }

    struct Finding: Decodable, Identifiable {
        var id: String { "\(session_id):\(path):\(kind):\(severity)" }

        let kind: String
        let severity: String
        let session_id: String
        let path: String
        let target_path: String
        let message: String
        let expected_size: Int64?
        let actual_size: Int64?
        let expected_digest: String?
        let actual_digest: String?
        let error: String?
    }

    struct WarningRecord: Decodable, Identifiable {
        var id: String { warningID }

        let warningID: String
        let session_id: String?
        let code: String
        let message: String
        let severity: String?
        let paths: [String]?
        let target_path: String?
        let created_at: String?

        enum CodingKeys: String, CodingKey {
            case warningID = "id"
            case session_id
            case code
            case message
            case severity
            case paths
            case target_path
            case created_at
        }
    }

    struct SoftDeleteRecord: Decodable, Identifiable {
        var id: String { softDeleteID }

        let softDeleteID: String
        let session_id: String?
        let root_id: String?
        let source_path: String
        let target_path: String
        let kind: String?
        let reason: String?

        enum CodingKeys: String, CodingKey {
            case softDeleteID = "id"
            case session_id
            case root_id
            case source_path
            case target_path
            case kind
            case reason
        }
    }

    struct TargetDriftRecord: Decodable, Identifiable {
        var id: String { driftID }

        let driftID: String
        let session_id: String?
        let root_id: String?
        let path: String
        let change: String
        let review_state: String?
        let detected_at: String?

        enum CodingKeys: String, CodingKey {
            case driftID = "id"
            case session_id
            case root_id
            case path
            case change
            case review_state
            case detected_at
        }
    }

    struct ArtifactProblem: Decodable, Identifiable {
        var id: String { "\(session_id ?? "global"):\(path):\(error)" }

        let session_id: String?
        let path: String
        let error: String
    }

    let target_root: String
    let session_id: String?
    let manifest: ManifestSummary
    let summary: Summary
    let findings: [Finding]?
    let warnings: [WarningRecord]?
    let soft_deletes: [SoftDeleteRecord]?
    let target_drifts: [TargetDriftRecord]?
    let artifact_problems: [ArtifactProblem]?
    let manifests: [ManifestSummary]?

    var reviewRequired: Bool {
        summary.error_findings > 0 ||
            summary.warning_findings > 0 ||
            summary.warnings > 0 ||
            summary.soft_deletes > 0 ||
            summary.target_drifts > 0 ||
            summary.artifact_problems > 0 ||
            summary.manifest_count == 0
    }

    var statusLabel: String {
        if summary.manifest_count == 0 {
            return "no manifest"
        }
        return reviewRequired ? "manifest review" : "target-manifest verified"
    }

    var profileRootIdentity: EvidenceAvailability {
        let rootID = manifest.root_id?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if rootID.isEmpty {
            return .unavailable("No manifest root_id is present in the selected verify evidence.")
        }
        return .available("\(rootID) is a profile root identity, not a cryptographic content root.")
    }

    var merkleRootProof: EvidenceAvailability {
        .unavailable("No wired Merkle tree or content-root proof is emitted by current SuperMover verification.")
    }

    var currentSourceComparison: EvidenceAvailability {
        .unavailable("Current source tree comparison is not part of verify; this checks target state against published manifest evidence.")
    }
}

struct StatusSnapshot: Decodable {
    struct Overall: Decodable {
        let status: String
        let target_status: String
    }

    struct Counts: Decodable {
        let warnings: Int
        let soft_deletes: Int
        let target_drifts: Int
        let live_target_drifts: Int
        let live_target_drift_artifact_problems: Int
        let prune_unapplied_approvals: Int
        let prune_active_approvals: Int
        let prune_stale_approvals: Int
        let prune_expired_approvals: Int
        let prune_consumed_approvals: Int
        let prune_receipt_issues: Int
        let recovery_issues: Int
        let artifact_problems: Int
        let network_transfers: Int
    }

    struct LatestSession: Decodable {
        let id: String?
        let manifest_id: String?
        let created_at: String?
        let entries: Int
        let completeness_status: String
        let files_expected: Int
        let files_verified: Int
        let verification_errors: Int
        let verification_warnings: Int
    }

    struct ArtifactProblemSourceCount: Decodable {
        let source: String
        let count: Int
    }

    struct PruneReview: Decodable {
        let status: String
        let action: String
    }

    struct Pairing: Decodable {
        let status: String
        let receipt_id: String?
        let target_device_id: String?
        let paired_at: String?
        let method: String?
        let verified_at: String?
        let evidence: String?
        let receipt_source: String?
        let receipt_path: String?
        let source_receipt_path: String?
        let target_receipt_path: String?
        let encrypted_transfer: String
        let issue: String?
    }

    struct Privacy: Decodable {
        let status: String
        let mode: String?
        let traffic_level: Int?
        let claim: String
        let local_push: String
        let network_transfer: String
        let residual_leakage: [String]
        let configured_reductions: [String]?
        let overhead_status: String
        let overhead_source: String
    }

    struct TrafficPrivacy: Decodable {
        let status: String
        let claim: String
        let blockers: [String]?
    }

    struct NetworkTransfer: Decodable, Identifiable {
        var id: String { session_id }
        let session_id: String
        let status: String
        let stage: String?
        let error_code: String?
        let error: String?
        let action: String
    }

    struct Network: Decodable {
        let status: String
        let artifact_problems: Int
        let transfers: [NetworkTransfer]?
    }

    let profile_id: String
    let target_id: String
    let target_root: String
    let overall: Overall
    let issues: [String]?
    let latest_session: LatestSession
    let counts: Counts
    let prune_review: PruneReview
    let pairing: Pairing
    let privacy: Privacy
    let traffic_privacy_acceptance: TrafficPrivacy
    let network: Network
    let artifact_problem_sources: [ArtifactProblemSourceCount]?
}

struct ReportSnapshot: Decodable {
    struct Overall: Decodable {
        let status: String
        let issues: [String]?
    }

    struct Summary: Decodable {
        let warnings: Int
        let soft_deletes: Int
        let target_drifts: Int
        let live_target_drifts: Int
        let prune_candidates: Int
        let prune_refusals: Int
        let prune_approvals: Int
        let network_transfers: Int
        let artifact_problems: Int
    }

    struct LatestSession: Decodable {
        let id: String?
        let manifest_id: String?
        let created_at: String?
        let entries: Int
        let files: Int
        let completeness: Completeness
    }

    struct Completeness: Decodable {
        let status: String
        let files_expected: Int
        let files_verified: Int
        let verification_errors: Int
        let verification_warnings: Int
    }

    struct PruneReview: Decodable {
        let status: String
        let approval_required: Bool
        let apply: String
        let summary: PruneReviewSummary
    }

    struct PruneReviewSummary: Decodable {
        let candidates: Int
        let refusals: Int
        let approvals: Int
        let unapplied_approvals: Int
        let receipt_issues: Int
    }

    struct Pairing: Decodable {
        let status: String
        let receipt_id: String?
        let target_device_id: String?
        let paired_at: String?
        let method: String?
        let verified_at: String?
        let evidence: String?
        let receipt_source: String?
        let receipt_path: String?
        let source_receipt_path: String?
        let target_receipt_path: String?
        let encrypted_transfer: String
        let issue: String?
    }

    struct Privacy: Decodable {
        let status: String
        let claim: String
        let network_transfer: String
    }

    struct HealthSummary: Decodable {
        let incomplete_sessions: Int
        let invalid_records: Int
        let artifact_problems: Int
        let target_drifts: Int
        let network_transfers: Int
    }

    struct Health: Decodable {
        let healthy: Bool
        let summary: HealthSummary
    }

    struct ArtifactProblem: Decodable, Identifiable {
        var id: String { "\(source ?? "unknown"):\(path):\(error)" }
        let source: String?
        let path: String
        let error: String
    }

    struct NetworkTransfer: Decodable, Identifiable {
        var id: String { session_id }
        let session_id: String
        let status: String
        let stage: String?
        let error_code: String?
        let error: String?
        let action: String
    }

    let profile_id: String?
    let target_id: String?
    let target_root: String
    let overall: Overall
    let summary: Summary
    let latest_session: LatestSession
    let prune_review: PruneReview
    let pairing: Pairing
    let privacy: Privacy
    let health: Health
    let artifact_problems: [ArtifactProblem]?
    let network_transfers: [NetworkTransfer]?
}

struct PruneReviewSnapshot: Decodable {
    struct EnvelopeAuthorization: Decodable {
        let approval_bypass: Bool
        let approval_writing: String
        let receipt_writing: String
        let physical_pruning: String
        let target_deletion: String
        let apply_requires: String
    }

    struct Policy: Decodable {
        let mode: String
        let require_review: Bool
        let retention_days: Int?
        let allow_physical_prune: Bool
    }

    struct PruneReviewSummary: Decodable {
        let soft_deletes: Int
        let candidates: Int
        let refusals: Int
        let approvals: Int
        let unapplied_approvals: Int
        let active_approvals: Int
        let stale_approvals: Int
        let expired_approvals: Int
        let consumed_approvals: Int
        let receipts: Int
        let receipt_issues: Int
        let artifact_problems: Int
    }

    struct Candidate: Decodable, Identifiable {
        var id: String { soft_delete_id }
        let soft_delete_id: String
        let detected_session_id: String
        let root_id: String
        let source_path: String
        let target_path: String
        let kind: String
        let detected_at: String
        let intended_action: String
        let physical_pruning: String
        let approval_writing: String
        let receipt_writing: String
        let review_required: Bool
    }

    struct Refusal: Decodable, Identifiable {
        var id: String { (soft_delete_id ?? "unknown") + ":" + reason_code }
        let soft_delete_id: String?
        let detected_session_id: String?
        let source_path: String?
        let target_path: String?
        let reason_code: String
        let message: String
    }

    struct PruneReview: Decodable {
        let status: String
        let dry_run: Bool
        let approval_required: Bool
        let approval_authoring: String
        let physical_pruning: String
        let apply: String
        let approval_source: String
        let receipt_source: String
        let profile_delete_policy: Policy?
        let summary: PruneReviewSummary
        let candidates: [Candidate]?
        let refusals: [Refusal]?
        let approvals: [Approval]?
        let receipts: [Receipt]?
    }

    struct ArtifactProblem: Decodable, Identifiable {
        var id: String { "\(source ?? "unknown"):\(path):\(error)" }
        let source: String?
        let path: String
        let error: String
    }

    struct ApprovalEvidence: Decodable, Identifiable {
        var id: String { soft_delete_id + ":" + state + ":" + target_path }
        let soft_delete_id: String
        let target_path: String
        let state: String
        let reason: String?
        let reason_code: String?
    }

    struct ApprovalItem: Decodable, Identifiable {
        var id: String { soft_delete_id + ":" + target_path }
        let soft_delete_id: String
        let detected_session_id: String
        let previous_session_id: String
        let previous_manifest_id: String
        let root_id: String
        let source_path: String
        let target_path: String
        let kind: String
        let size: Int64?
        let digest: String?
        let symlink_target: String?
        let detected_at: String
        let soft_delete_ref: String
    }

    struct Approval: Decodable, Identifiable {
        let id: String
        let profile_id: String
        let target_id: String
        let root_id: String
        let status: String
        let unapplied: Bool
        let release_state: String
        let release_blocker: Bool
        let release_reason: String?
        let release_action: String
        let linked_receipt_id: String?
        let linked_receipt_status: String?
        let path: String
        let action: String
        let physical_pruning: String
        let created_at: String
        let approved_by: String?
        let approved_at: String?
        let superseded_by: String?
        let superseded_at: String?
        let expires_at: String?
        let review_tool: String
        let profile_snapshot_id: String?
        let profile_snapshot_path: String?
        let profile_snapshot_digest: String?
        let approval_scope_digest: String?
        let approval_reason: String?
        let refusal_reason: String?
        let profile_delete_policy: Policy
        let current_evidence: [ApprovalEvidence]?
        let items: [ApprovalItem]?
    }

    struct Receipt: Decodable, Identifiable {
        let id: String
        let prune_session_id: String
        let approval_id: String
        let profile_id: String
        let target_id: String
        let status: String
        let dry_run: Bool
        let action: String
        let started_at: String
        let ended_at: String?
        let path: String
        let approval_scope_digest: String
    }

    let schema: String
    let scope: String
    let target_root: String
    let profile_id: String?
    let target_id: String?
    let session_filter: String?
    let latest_session_id: String?
    let status: String
    let review_required: Bool
    let action: String
    let read_only: Bool
    let authorization: EnvelopeAuthorization
    let prune_review: PruneReview
    let artifact_problems: [ArtifactProblem]?
}

struct PruneApprovalsSnapshot: Decodable {
    struct ApprovalItem: Decodable, Identifiable {
        var id: String { soft_delete_id + ":" + target_path }
        let soft_delete_id: String
        let soft_delete_ref: String
        let detected_session_id: String
        let previous_session_id: String
        let previous_manifest_id: String
        let root_id: String
        let source_path: String
        let target_path: String
        let kind: String
        let size: Int64?
        let digest: String?
        let symlink_target: String?
        let detected_at: String
    }

    struct Approval: Decodable, Identifiable {
        let id: String
        let profile_id: String
        let target_id: String
        let root_id: String
        let created_at: String
        let approved_by: String?
        let approved_at: String?
        let review_tool: String
        let approval_scope_digest: String
        let expires_at: String?
        let status: String
        let approval_reason: String?
        let refusal_reason: String?
        let superseded_by: String?
        let superseded_at: String?
        let items: [ApprovalItem]?
    }

    let target_root: String
    let profile_id: String
    let target_id: String
    let approvals: [Approval]?
}

struct PruneApprovalAuthoringSnapshot: Decodable {
    struct DryRunSummary: Decodable {
        let soft_deletes: Int
        let candidates: Int
        let refusals: Int
        let artifact_problems: Int
    }

    struct ApprovalItem: Decodable, Identifiable {
        var id: String { soft_delete_id + ":" + target_path }
        let soft_delete_id: String
        let detected_session_id: String
        let previous_session_id: String
        let previous_manifest_id: String
        let source_path: String
        let target_path: String
        let kind: String
        let size: Int64?
        let digest: String?
        let symlink_target: String?
    }

    struct Approval: Decodable {
        let id: String
        let profile_id: String
        let target_id: String
        let root_id: String
        let status: String
        let approved_by: String?
        let approved_at: String?
        let expires_at: String?
        let approval_reason: String?
        let approval_scope_digest: String
        let profile_snapshot_id: String
    }

    let schema: String
    let target_root: String
    let profile_id: String
    let target_id: String
    let approval_id: String
    let approval_path: String
    let profile_snapshot_id: String
    let profile_snapshot_path: String
    let profile_snapshot_digest: String
    let approval_scope_digest: String
    let approval_writing: String
    let profile_snapshot_writing: String
    let physical_pruning: String
    let receipt_writing: String
    let approval: Approval
    let items: [ApprovalItem]
    let dry_run_summary: DryRunSummary
}

struct PruneApprovalSupersedeSnapshot: Decodable {
    struct Approval: Decodable {
        let id: String
        let profile_id: String
        let target_id: String
        let status: String
        let approved_by: String?
        let approved_at: String?
        let superseded_by: String?
        let superseded_at: String?
        let review_tool: String
        let refusal_reason: String?
    }

    let target_root: String
    let profile_id: String
    let target_id: String
    let approval_id: String
    let approval_path: String
    let approval: Approval
}

struct HealthSnapshot: Decodable {
    struct Summary: Decodable {
        let incomplete_sessions: Int
        let invalid_records: Int
        let artifact_problems: Int
        let target_drifts: Int
        let network_transfers: Int
    }

    struct RecoveryItem: Decodable, Identifiable {
        var id: String { session_id + ":" + action + ":" + path }
        let session_id: String
        let state: String
        let action: String
        let reason: String
        let path: String
        let updated_at: String
    }

    struct InvalidRecord: Decodable, Identifiable {
        var id: String { (session_id ?? "unknown") + ":" + path + ":" + error }
        let session_id: String?
        let path: String
        let error: String
    }

    struct ArtifactIssue: Decodable, Identifiable {
        var id: String { (source ?? "unknown") + ":" + path + ":" + error }
        let source: String?
        let session_id: String?
        let path: String
        let error: String
    }

    struct NetworkTransfer: Decodable, Identifiable {
        var id: String { session_id }
        let session_id: String
        let status: String
        let stage: String?
        let error_code: String?
        let error: String?
        let action: String
    }

    let target_root: String
    let healthy: Bool
    let summary: Summary
    let items: [RecoveryItem]?
    let invalid: [InvalidRecord]?
    let artifacts: [ArtifactIssue]?
    let network_transfers: [NetworkTransfer]?
}

struct DriftListSnapshot: Decodable {
    struct Manifest: Decodable {
        let session_id: String?
    }

    struct Summary: Decodable {
        let manifest_count: Int
        let manifest_entries: Int
        let target_drifts: Int
        let artifact_problems: Int
    }

    struct Drift: Decodable, Identifiable {
        var id: String { path + ":" + change }
        let path: String
        let change: String
        let evidence: [String]?
    }

    struct ArtifactProblem: Decodable, Identifiable {
        var id: String { (session_id ?? "unknown") + ":" + path + ":" + err }
        let session_id: String?
        let path: String
        let err: String
    }

    let target_root: String
    let session_id: String?
    let manifest: Manifest
    let summary: Summary
    let target_drifts: [Drift]?
    let artifact_problems: [ArtifactProblem]?
}

struct DriftRecordSnapshot: Decodable {
    struct Record: Decodable, Identifiable {
        var id: String { recordID }

        let recordID: String
        let path: String
        let change: String
        let session_id: String
        let review_state: String
        let recorded: Bool
        let existing: Bool
        let reopened: Bool

        enum CodingKeys: String, CodingKey {
            case recordID = "id"
            case path
            case change
            case session_id
            case review_state
            case recorded
            case existing
            case reopened
        }
    }

    struct ArtifactProblem: Decodable, Identifiable {
        var id: String { (session_id ?? "unknown") + ":" + path + ":" + err }

        let session_id: String?
        let path: String
        let err: String
    }

    let target_root: String
    let session_id: String?
    let detected: Int
    let recorded: Int
    let existing: Int
    let reopened: Int
    let manifest_count: Int
    let artifact_problems: [ArtifactProblem]?
    let records: [Record]?
}

struct DaemonStatusSnapshot: Decodable {
    struct Install: Decodable {
        let installed_at: String
        let run_mode: String
        let service_manager: String
        let command: [String]
    }

    struct StateRecord: Decodable {
        let status: String
        let run_mode: String
        let service_manager: String
        let pid: Int?
        let mode: String?
        let pairing_address: String?
        let receiver_address: String?
        let started_at: String?
        let updated_at: String
        let stopped_at: String?
        let last_error: String?
    }

    struct StopIntent: Decodable {
        let requested_at: String
        let reason: String?
        let requested_by_pid: Int?
    }

    struct RestartIntent: Decodable {
        let requested_at: String
        let reason: String?
        let requested_by_pid: Int?
    }

    struct LifecycleEvent: Decodable, Identifiable {
        let id: String
        let type: String
        let recorded_at: String
        let message: String?
        let details: [String: String]?
    }

    let profile_id: String
    let target_id: String
    let installed: Bool
    let state: String
    let run_mode: String?
    let service_manager: String?
    let scope_issues: [String]?
    let install: Install?
    let state_record: StateRecord?
    let stop_intent: StopIntent?
    let restart_intent: RestartIntent?
    let lifecycle_events: [LifecycleEvent]?
}

struct DaemonLogsSnapshot: Decodable {
    struct Event: Decodable, Identifiable {
        let id: String
        let type: String
        let recorded_at: String
        let message: String?
        let details: [String: String]?
    }

    let profile_id: String
    let target_id: String
    let events: [Event]
    let scope_issues: [String]?
}

struct DriftMutationSnapshot: Decodable {
    let id: String
    let path: String
    let previous_state: String
    let review_state: String
    let reviewed_at: String
    let reviewer: String?
    let reason: String
    let profile_id: String
    let target_id: String
    let session_id: String
    let repair: String?
    let manifest_rewrite: String?
    let prune: String?
}

struct ReconcileSnapshot: Decodable {
    struct Expected: Decodable {
        let session_id: String?
        let manifest_id: String?
        let kind: String
        let path: String?
        let size: Int64?
        let digest: String?
        let mode: UInt32?
        let mod_time: String?
    }

    struct ObservedBefore: Decodable {
        let present: Bool?
        let kind: String
        let path: String?
        let size: Int64?
        let digest: String?
        let mode: UInt32?
        let mod_time: String?
    }

    struct SourceEvidence: Decodable {
        let root_id: String
        let path: String
        let size: Int64
        let digest: String
        let mode: UInt32?
    }

    struct Summary: Decodable {
        let records: Int
        let planned: Int
        let applied: Int
        let noop: Int
        let refused: Int
        let artifact_problems: Int
    }

    struct Action: Decodable, Identifiable {
        var id: String { drift_id + ":" + action }
        let drift_id: String
        let path: String
        let change: String
        let action: String
        let result: String
        let session_id: String
        let expected: Expected
        let observed_before: ObservedBefore
        let source_evidence: SourceEvidence?
        let reviewed_at: String?
        let reviewer: String?
        let reason: String?
    }

    struct Refusal: Decodable, Identifiable {
        var id: String { (drift_id ?? "global") + ":" + reason_code + ":" + (path ?? "none") }
        let drift_id: String?
        let path: String?
        let change: String?
        let action: String?
        let reason_code: String
        let message: String
        let observed_before: ObservedBefore
    }

    struct ArtifactProblem: Decodable, Identifiable {
        var id: String { (session_id ?? "unknown") + ":" + path + ":" + error }
        let session_id: String?
        let path: String
        let error: String
    }

    let schema: String
    let target_root: String
    let profile_id: String
    let target_id: String
    let session_id: String?
    let generated_at: String
    let apply_intent: Bool
    let summary: Summary
    let actions: [Action]?
    let refusals: [Refusal]?
    let artifact_problems: [ArtifactProblem]?
}

@MainActor
final class AppStore: ObservableObject {
    typealias CLICommandOutput = (stdout: String, stderr: String, exitCode: Int32)

    var profileDefaultsRootURL = FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent(".supermover", isDirectory: true)

    @Published var selectedRole: WorkbenchRole = .source {
        didSet {
            resetSetupScopedEvidenceIfChanged(from: oldValue.rawValue, to: selectedRole.rawValue)
            refreshSelectedTaskAcceptanceLaunchPreview()
        }
    }
    @Published var profilePath = "" {
        didSet { resetSetupScopedEvidenceIfChanged(from: oldValue, to: profilePath) }
    }
    @Published var sourceRootPath = "" {
        didSet { resetSetupScopedEvidenceIfChanged(from: oldValue, to: sourceRootPath) }
    }
    @Published var targetRootPath = "" {
        didSet { resetSetupScopedEvidenceIfChanged(from: oldValue, to: targetRootPath) }
    }
    @Published var profileID = TaskInput.defaultProfileID {
        didSet { resetSetupScopedEvidenceIfChanged(from: oldValue, to: profileID) }
    }
    @Published var profileName = TaskInput.defaultProfileName {
        didSet { resetSetupScopedEvidenceIfChanged(from: oldValue, to: profileName) }
    }
    @Published var targetID = "" {
        didSet { resetSetupScopedEvidenceIfChanged(from: oldValue, to: targetID) }
    }
    @Published var targetName = "" {
        didSet { resetSetupScopedEvidenceIfChanged(from: oldValue, to: targetName) }
    }
    @Published var sessionID = "" {
        didSet { resetSetupScopedEvidenceIfChanged(from: oldValue, to: sessionID) }
    }
    @Published var sessionPrefix = "" {
        didSet { clearSyncSnapshotsIfChanged(from: oldValue, to: sessionPrefix) }
    }
    @Published var queueEntryID = "" {
        didSet { clearSyncSnapshotsIfChanged(from: oldValue, to: queueEntryID) }
    }
    @Published var syncRetryBackoff = "1m" {
        didSet { clearSyncSnapshotsIfChanged(from: oldValue, to: syncRetryBackoff) }
    }
    @Published var syncInterval = "1m" {
        didSet { clearSyncSnapshotsIfChanged(from: oldValue, to: syncInterval) }
    }
    @Published var syncMaxRuns = "0" {
        didSet { clearSyncSnapshotsIfChanged(from: oldValue, to: syncMaxRuns) }
    }
    @Published var syncSettle = "250ms" {
        didSet { clearSyncSnapshotsIfChanged(from: oldValue, to: syncSettle) }
    }
    @Published var syncMaxEvents = "0" {
        didSet { clearSyncSnapshotsIfChanged(from: oldValue, to: syncMaxEvents) }
    }
    @Published var syncDiscoveryListen = "0.0.0.0:39394" {
        didSet { clearSyncSnapshotsIfChanged(from: oldValue, to: syncDiscoveryListen) }
    }
    @Published var syncDiscoveryTimeout = "2s" {
        didSet { clearSyncSnapshotsIfChanged(from: oldValue, to: syncDiscoveryTimeout) }
    }
    @Published var listenAddress = "127.0.0.1:0" {
        didSet { resetSetupScopedEvidenceIfChanged(from: oldValue, to: listenAddress) }
    }
    @Published var pairingTargetAddress = "" {
        didSet { clearExplicitDiscoveryIfChanged(from: oldValue, to: pairingTargetAddress) }
    }
    @Published var pairingVerificationCode = ""
    @Published var pairingMethod = "sas"
    @Published var pairingTimeout = "5s"
    @Published var pairingReceipt = PairingReceiptDraft() {
        didSet {
            resetSetupScopedEvidenceIfChanged(
                from: oldValue.contextInputs.joined(separator: "\u{1F}"),
                to: pairingReceipt.contextInputs.joined(separator: "\u{1F}")
            )
        }
    }
    @Published var profileNetwork = ProfileNetworkDraft() {
        didSet {
            resetSetupScopedEvidenceIfChanged(
                from: oldValue.contextInputs.joined(separator: "\u{1F}"),
                to: profileNetwork.contextInputs.joined(separator: "\u{1F}")
            )
        }
    }
    @Published var discoveryBrowseListen = "0.0.0.0:39394" {
        didSet { clearBrowseDiscoveryIfChanged(from: oldValue, to: discoveryBrowseListen) }
    }
    @Published var discoveryBrowseTimeout = "2s" {
        didSet {
            clearBrowseDiscoveryIfChanged(from: oldValue, to: discoveryBrowseTimeout)
            clearExplicitDiscoveryIfChanged(from: oldValue, to: discoveryBrowseTimeout)
        }
    }
    @Published var discoveryAdvertiseListen = "" {
        didSet { clearAdvertiseDiscoveryIfChanged(from: oldValue, to: discoveryAdvertiseListen) }
    }
    @Published var discoveryAdvertiseDestination = "255.255.255.255:39394" {
        didSet { clearAdvertiseDiscoveryIfChanged(from: oldValue, to: discoveryAdvertiseDestination) }
    }
    @Published var discoveryAdvertiseDuration = "10s" {
        didSet { clearAdvertiseDiscoveryIfChanged(from: oldValue, to: discoveryAdvertiseDuration) }
    }
    @Published var discoveryAdvertiseInterval = "1s" {
        didSet { clearAdvertiseDiscoveryIfChanged(from: oldValue, to: discoveryAdvertiseInterval) }
    }
    @Published var selectedTask: SuperMoverTaskKind = .status {
        didSet { refreshSelectedTaskAcceptanceLaunchPreview() }
    }
    @Published var activeRuns: [SupervisedProcessSlot: TaskRun] = [:]
    @Published var focusedProcessSlot: SupervisedProcessSlot = .foregroundAction
    @Published var recentRuns: [TaskRun] = []
    @Published var processEvents: [ProcessLifecycleEvent] = []
    @Published var appEvents: [AppEvent] = []
    @Published var artifactReadProblems: [ArtifactReadProblem] = []
    @Published var evidenceEnvelopes: [StructuredArtifactKind: StructuredEvidenceEnvelope] = [:]
    @Published var evidenceEnvelopeHistory: [StructuredEvidenceEnvelope] = []
    @Published var evidenceArtifactCatalog: EvidenceArtifactCatalog?
    @Published var evidenceArtifactQuery = ""
    @Published var evidenceArtifactFamilyFilter = "all"
    @Published var acceptanceBundlePath = "" {
        didSet {
            if normalizedForSignature(oldValue) != normalizedForSignature(acceptanceBundlePath) {
                acceptanceBundleSnapshot = nil
                acceptanceBundleLoadError = ""
            }
            refreshSelectedTaskAcceptanceLaunchPreview()
        }
    }
    @Published var acceptanceBundleSnapshot: AcceptanceBundleLoadedSnapshot? {
        didSet { refreshSelectedTaskAcceptanceLaunchPreview() }
    }
    @Published var acceptanceBundleLoadError = ""
    @Published private(set) var selectedTaskAcceptanceLaunchPreview: AcceptanceInstalledAppLaunchPreview?
    var acceptanceInstalledAppLaunchCoordinator = AcceptanceInstalledAppLaunchCoordinator()
    var acceptanceBundleAuthoringCoordinator = AcceptanceBundleAuthoringCoordinator()
    var acceptanceBundleOperations = AcceptanceBundleAppOperations()
    var cliCommandRunner: (([String]) throws -> CLICommandOutput)?
    @Published var acceptanceOperatorEvidence = AcceptanceOperatorEvidenceDraft()
    @Published var acceptanceEvaluation = AcceptanceEvaluationDraft()
    @Published var acceptanceServePhase = "1"
    @Published var cliProvenance = CLIResolver.provenance() {
        didSet { refreshSelectedTaskAcceptanceLaunchPreview() }
    }
    @Published var note = "This app shell wraps the current SuperMover CLI. Discovery hints are untrusted; detached background sync and broad automatic repair remain outside this app stage."
    @Published var statusSnapshot: StatusSnapshot?
    @Published var verifySnapshot: VerifySnapshot?
    @Published var sourceConsistencySnapshot: AcceptanceBundleSnapshot.SourceConsistencyEvidence?
    @Published var healthSnapshot: HealthSnapshot?
    @Published var reportSnapshot: ReportSnapshot?
    @Published var driftListSnapshot: DriftListSnapshot?
    @Published var driftRecordSnapshot: DriftRecordSnapshot?
    @Published var discoveryHintsSnapshot: [DiscoveryAddressHintSnapshot]?
    @Published var discoveryBrowseSnapshot: DiscoveryBrowseSnapshot?
    @Published var discoveryAdvertiseSnapshot: DiscoveryAdvertiseSnapshot?
    @Published var daemonStatusSnapshot: DaemonStatusSnapshot?
    @Published var daemonLogsSnapshot: DaemonLogsSnapshot?
    @Published var pruneReviewSnapshot: PruneReviewSnapshot?
    @Published var pruneApprovalsSnapshot: PruneApprovalsSnapshot?
    @Published var pruneApprovalAuthoringSnapshot: PruneApprovalAuthoringSnapshot?
    @Published var pruneApprovalSupersedeSnapshot: PruneApprovalSupersedeSnapshot?
    @Published var driftMutationSnapshot: DriftMutationSnapshot?
    @Published var reconcileSnapshot: ReconcileSnapshot?
    @Published var syncQueueSnapshot: SyncQueueSnapshot?
    @Published var syncRunSnapshot: SyncRunSnapshot?
    @Published var syncLoopSnapshot: SyncLoopSnapshot?
    @Published var syncWatchSnapshot: SyncWatchSnapshot?
    @Published var syncNetworkRunSnapshot: SyncNetworkRunSnapshot?
    @Published var syncNetworkDiscoverRunSnapshot: SyncNetworkDiscoverRunSnapshot?
    @Published var syncNetworkLoopSnapshot: SyncNetworkLoopSnapshot?
    @Published var serveReadinessSnapshot: ServeReadinessSnapshot?
    @Published var driftIDsInput = ""
    @Published var approvalID = ""
    @Published var softDeleteIDsInput = ""
    @Published var expiresAt = ""
    @Published var reason = "" {
        didSet { clearSyncSnapshotsIfChanged(from: oldValue, to: reason) }
    }
    @Published var reviewer = ""

    private var processControllers: [SupervisedProcessSlot: ProcessController] = [:]
    private var serveReadyFilePaths: [SupervisedProcessSlot: String] = [:]
    private var serveReadyFileContextSignatures: [SupervisedProcessSlot: String] = [:]
    private var networkPushBaselineFilePaths: [SupervisedProcessSlot: String] = [:]
    private var networkPushBaselineContextSignatures: [SupervisedProcessSlot: String] = [:]

    var activeRun: TaskRun? {
        activeRuns[focusedProcessSlot] ?? activeRuns.values.sorted { $0.launchedAt > $1.launchedAt }.first
    }

    var effectiveSourceConsistencySnapshot: AcceptanceBundleSnapshot.SourceConsistencyEvidence? {
        if let sourceConsistencySnapshot {
            return sourceConsistencyEvidenceBoundToCurrentContext(sourceConsistencySnapshot)
        }
        guard let loaded = acceptanceBundleSnapshot?.sourceConsistencyArtifact else {
            return nil
        }
        return sourceConsistencyEvidenceBoundToCurrentContext(
            loaded,
            sourceProfilePath: acceptanceBundleCurrentSourceProfilePath,
            requiresProfileBinding: true
        )
    }

    var setupContextSignature: String {
        [
            profilePath,
            sourceRootPath,
            targetRootPath,
            profileID,
            profileName,
            targetID,
            targetName,
            sessionID,
            listenAddress,
            pairingReceipt.trimmedExportTarget,
            pairingReceipt.trimmedImportReceiptFile,
            profileNetwork.trimmedReceiverURL,
            profileNetwork.trimmedTLSCertificatePath,
            profileNetwork.trimmedTLSPrivateKeyPath,
            profileNetwork.clearReceiverURL ? "clear-receiver-url" : "",
            profileNetwork.clearTLSIdentity ? "clear-tls-identity" : "",
            selectedRole.rawValue,
        ]
        .map(normalizedForSignature)
        .joined(separator: "\u{1F}")
    }

    var isProfileSelected: Bool {
        selectedProfilePath != nil
    }

    var selectedProfilePathState: SelectedProfilePathState {
        profileSelectionContext.pathState
    }

    var profileSelectionContext: ProfileSelectionContext {
        resolvedProfileSelectionContext
    }

    var selectedProfileDisplayTitle: String {
        profileSelectionContext.title
    }

    var selectedProfileDisplayDetail: String? {
        profileSelectionContext.detail
    }

    var selectedProfileDisplayMetadata: String? {
        profileSelectionContext.metadata
    }

    var selectedProfileRawPath: String? {
        profileSelectionContext.rawPath
    }

    var selectedProfileRawPathLabel: String {
        profileSelectionContext.rawPathLabel
    }

    var selectedProfileEvidenceID: String? {
        profileSelectionContext.evidenceID
    }

    var selectedProfileShowsSourceIdentityFields: Bool {
        profileSelectionContext.showsSourceIdentityFields
    }

    var selectedProfileShowsTargetIdentityFields: Bool {
        profileSelectionContext.showsTargetIdentityFields
    }

    var selectedProfileAllowsExplicitCreate: Bool {
        profileSelectionContext.allowsExplicitCreate
    }

    var recommendedProfileDestinationPath: String {
        profileDefaultsRootURL
            .appendingPathComponent("profile-local.json", isDirectory: false)
            .path
    }

    var selectedProfileIsExistingFile: Bool {
        selectedProfilePathState == .existingFile
    }

    var setupGuide: SetupGuide {
        let sourceReadiness = directoryReadiness(path: sourceRootPath, requiresWrite: false)
        let targetReadiness = directoryReadiness(path: targetRootPath, requiresWrite: true)
        let configStep = setupGuideConfigStep()
        let foldersStep = setupGuideFoldersStep(sourceReadiness: sourceReadiness, targetReadiness: targetReadiness)
        let validationStep = setupGuideValidationStep()
        return SetupGuide(
            title: "Prepare this \(selectedRole.title)",
            subtitle: "Pick the role, choose the migration config file, then validate the folders that the CLI will use.",
            steps: [configStep, foldersStep, validationStep]
        )
    }

    func localizedSetupGuide(using localization: AppChromeLocalization) -> SetupGuide {
        let sourceReadiness = directoryReadiness(path: sourceRootPath, requiresWrite: false)
        let targetReadiness = directoryReadiness(path: targetRootPath, requiresWrite: true)
        let configStep = localizedSetupGuideConfigStep(using: localization)
        let foldersStep = localizedSetupGuideFoldersStep(
            using: localization,
            sourceReadiness: sourceReadiness,
            targetReadiness: targetReadiness
        )
        let validationStep = localizedSetupGuideValidationStep(using: localization)
        return SetupGuide(
            title: selectedRole.localizedSetupGuideTitle(using: localization),
            subtitle: localization.text(.setupGuideSubtitle),
            steps: [configStep, foldersStep, validationStep]
        )
    }

    func localizedProfileSelectionContext(using localization: AppChromeLocalization) -> ProfileSelectionContext {
        let rawContext = profileSelectionContext
        let title: String
        let detail: String?
        let metadata: String?
        let fileName = selectedProfileFileName

        if let evidenceID = rawContext.evidenceID {
            title = evidenceID
            detail = fileName ?? localization.text(.setupProfileSelectedConfigFileTitle)
            metadata = localizedProfileSelectionMetadata(for: rawContext.pathState, using: localization)
        } else if selectedRole == .source, rawContext.pathState == .newDestination {
            let isRecommendedDestination = isRecommendedProfileDestination(rawContext.rawPath)
            let defaultTitle = isRecommendedDestination
                ? localization.text(.setupProfileRecommendedSourceConfigTitle)
                : localization.text(.setupProfileNewSourceConfigTitle)
            title = customizedDraftProfileName ?? defaultTitle
            detail = customizedDraftProfileID.map { "ID: \($0)" }
                ?? localization.text(.setupProfileNewSourceConfigDetail)
            metadata = isRecommendedDestination
                ? localization.text(.setupProfileRecommendedConfigReadyMetadata)
                : localization.text(.setupProfileNewConfigReadyMetadata)
        } else {
            switch rawContext.pathState {
            case .none:
                title = localization.text(.setupProfileNoConfigTitle)
                detail = selectedRole == .source
                    ? localization.text(.setupProfileNoConfigDetailSource)
                    : localization.text(.setupProfileNoConfigDetailExistingOnly)
                metadata = nil
            case .existingFile:
                title = fileName ?? localization.text(.setupProfileSelectedConfigTitle)
                detail = localizedProfilePathStateSummary(rawContext.pathState, using: localization)
                metadata = nil
            case .directory:
                title = fileName ?? localization.text(.setupProfileSelectedFolderTitle)
                detail = localizedProfilePathStateSummary(rawContext.pathState, using: localization)
                metadata = localization.text(.setupProfileDirectoryMetadata)
            case .missingFile:
                title = fileName ?? localization.text(.setupProfileSelectedConfigTitle)
                detail = localizedProfilePathStateSummary(rawContext.pathState, using: localization)
                metadata = localization.text(.setupProfileMissingMetadata)
            case .newDestination:
                title = fileName ?? localization.text(.setupProfileSelectedConfigTitle)
                detail = localizedProfilePathStateSummary(rawContext.pathState, using: localization)
                metadata = localizedProfileSelectionMetadata(for: rawContext.pathState, using: localization)
            }
        }

        return ProfileSelectionContext(
            title: title,
            detail: detail,
            metadata: metadata,
            rawPathLabel: localization.text(.setupProfileRawPathLabel),
            rawPath: rawContext.rawPath,
            evidenceID: rawContext.evidenceID,
            pathState: rawContext.pathState,
            showsSourceIdentityFields: rawContext.showsSourceIdentityFields,
            showsTargetIdentityFields: rawContext.showsTargetIdentityFields
        )
    }

    var selectedProfileCommandPreviewValue: String {
        guard let fileName = selectedProfileFileName else {
            return "<Selected Config>"
        }
        return "<Selected Config: \(fileName)>"
    }

    var missingProfileCommandPreviewValue: String {
        "<Config File Required>"
    }

    func commandPreviewArguments(for task: SuperMoverTaskKind) -> [String] {
        let arguments = task.buildArguments(using: currentInput)
        guard task.requiresProfile else {
            return arguments
        }
        var previewArguments = arguments
        guard let flagIndex = previewArguments.firstIndex(of: "--profile") else {
            return previewArguments
        }
        let valueIndex = previewArguments.index(after: flagIndex)
        guard valueIndex < previewArguments.endIndex else {
            return previewArguments
        }
        previewArguments[valueIndex] =
            selectedProfilePath == nil ? missingProfileCommandPreviewValue : selectedProfileCommandPreviewValue
        return previewArguments
    }

    var currentInput: TaskInput {
        TaskInput(
            profilePath: profilePath,
            sourceRootPath: sourceRootPath,
            targetRootPath: targetRootPath,
            profileID: profileID,
            profileName: profileName,
            targetID: targetID,
            targetName: targetName,
            sessionID: sessionID,
            sessionPrefix: sessionPrefix,
            queueEntryID: queueEntryID,
            syncRetryBackoff: syncRetryBackoff,
            syncInterval: syncInterval,
            syncMaxRuns: syncMaxRuns,
            syncSettle: syncSettle,
            syncMaxEvents: syncMaxEvents,
            syncDiscoveryListen: syncDiscoveryListen,
            syncDiscoveryTimeout: syncDiscoveryTimeout,
            listenAddress: listenAddress,
            pairingTargetAddress: pairingTargetAddress,
            pairingVerificationCode: pairingVerificationCode,
            pairingMethod: pairingMethod,
            pairingTimeout: pairingTimeout,
            pairingReceipt: pairingReceipt,
            profileNetwork: profileNetwork,
            discoveryBrowseListen: discoveryBrowseListen,
            discoveryBrowseTimeout: discoveryBrowseTimeout,
            discoveryAdvertiseListen: discoveryAdvertiseListen,
            discoveryAdvertiseDestination: discoveryAdvertiseDestination,
            discoveryAdvertiseDuration: discoveryAdvertiseDuration,
            discoveryAdvertiseInterval: discoveryAdvertiseInterval,
            driftIDsInput: driftIDsInput,
            approvalID: approvalID,
            softDeleteIDsInput: softDeleteIDsInput,
            expiresAt: expiresAt,
            reason: reason,
            reviewer: reviewer
        )
    }

    private var selectedEvidenceProfileID: String? {
        nonEmptyTrimmed(statusSnapshot?.profile_id)
            ?? nonEmptyTrimmed(reportSnapshot?.profile_id)
    }

    private var resolvedProfileSelectionContext: ProfileSelectionContext {
        let path = selectedProfilePath
        let pathState = selectedProfilePathState(for: path, role: selectedRole)
        let rawPathLabel = "File location"
        let hasUsableSelectedProfile = pathState == .existingFile || pathState == .newDestination

        guard path != nil else {
            let emptyDetail =
                selectedRole == .source
                ? "No file picking needed. Choose folders, then create the setup."
                : "Open an existing migration config file to load roots, pairing, network pins, and evidence links."
            return ProfileSelectionContext(
                title: selectedRole == .source ? "Recommended setup" : "No migration config selected",
                detail: emptyDetail,
                metadata: nil,
                rawPathLabel: rawPathLabel,
                rawPath: nil,
                evidenceID: nil,
                pathState: pathState,
                showsSourceIdentityFields: false,
                showsTargetIdentityFields: false
            )
        }

        let fileName = selectedProfileFileName

        if let evidenceID = selectedEvidenceProfileID {
            return ProfileSelectionContext(
                title: evidenceID,
                detail: fileName ?? "Selected migration config file",
                metadata: profileSelectionMetadata(for: pathState),
                rawPathLabel: rawPathLabel,
                rawPath: path,
                evidenceID: evidenceID,
                pathState: pathState,
                showsSourceIdentityFields: false,
                showsTargetIdentityFields: hasUsableSelectedProfile && selectedRole != .observer
            )
        }

        if selectedRole == .source, pathState == .newDestination {
            let explicitProfileName = customizedDraftProfileName
            let explicitProfileID = customizedDraftProfileID
            let isRecommendedDestination = isRecommendedProfileDestination(path)
            return ProfileSelectionContext(
                title: explicitProfileName
                    ?? (isRecommendedDestination ? "Recommended setup ready" : "Custom setup location"),
                detail: explicitProfileID.map { "ID: \($0)" } ?? "Choose folders, then create the setup.",
                metadata: isRecommendedDestination
                    ? "Recommended location selected."
                    : "Ready to create through the selected file.",
                rawPathLabel: rawPathLabel,
                rawPath: path,
                evidenceID: nil,
                pathState: pathState,
                showsSourceIdentityFields: true,
                showsTargetIdentityFields: true
            )
        }

        switch pathState {
        case .existingFile:
            return ProfileSelectionContext(
                title: fileName ?? "Selected migration config",
                detail: pathState.filesystemSummary,
                metadata: nil,
                rawPathLabel: rawPathLabel,
                rawPath: path,
                evidenceID: nil,
                pathState: pathState,
                showsSourceIdentityFields: false,
                showsTargetIdentityFields: selectedRole != .observer
            )
        case .directory:
            return ProfileSelectionContext(
                title: fileName ?? "Selected folder",
                detail: pathState.filesystemSummary,
                metadata: "Choose a .json migration config file.",
                rawPathLabel: rawPathLabel,
                rawPath: path,
                evidenceID: nil,
                pathState: pathState,
                showsSourceIdentityFields: false,
                showsTargetIdentityFields: false
            )
        case .missingFile:
            return ProfileSelectionContext(
                title: fileName ?? "Selected migration config",
                detail: pathState.filesystemSummary,
                metadata: "Open an existing migration config file before reading evidence.",
                rawPathLabel: rawPathLabel,
                rawPath: path,
                evidenceID: nil,
                pathState: pathState,
                showsSourceIdentityFields: false,
                showsTargetIdentityFields: false
            )
        case .newDestination:
            return ProfileSelectionContext(
                title: fileName ?? "Selected migration config",
                detail: pathState.filesystemSummary,
                metadata: profileSelectionMetadata(for: pathState),
                rawPathLabel: rawPathLabel,
                rawPath: path,
                evidenceID: nil,
                pathState: pathState,
                showsSourceIdentityFields: false,
                showsTargetIdentityFields: selectedRole != .observer
            )
        case .none:
            return ProfileSelectionContext(
                title: selectedRole == .source ? "Recommended setup" : "No migration config selected",
                detail: nil,
                metadata: nil,
                rawPathLabel: rawPathLabel,
                rawPath: nil,
                evidenceID: nil,
                pathState: pathState,
                showsSourceIdentityFields: false,
                showsTargetIdentityFields: false
            )
        }
    }

    private var selectedProfileFileName: String? {
        guard let path = selectedProfilePath else {
            return nil
        }
        let fileName = URL(filePath: path).lastPathComponent
        return nonEmptyTrimmed(fileName)
    }

    private var selectedProfilePath: String? {
        nonEmptyTrimmed(profilePath)
    }

    private func isRecommendedProfileDestination(_ path: String?) -> Bool {
        guard let path = nonEmptyTrimmed(path) else {
            return false
        }
        return URL(fileURLWithPath: path).standardizedFileURL.path
            == URL(fileURLWithPath: recommendedProfileDestinationPath).standardizedFileURL.path
    }

    private var acceptanceBundleCurrentSourceProfilePath: String? {
        acceptanceBundleSnapshot?.sourceTransferArtifact?.profile
            ?? acceptanceBundleSnapshot?.meta.roles["source_transfer"]?.profile
            ?? acceptanceBundleSnapshot?.sourcePairArtifact?.profile
            ?? acceptanceBundleSnapshot?.meta.roles["source_pair"]?.profile
    }

    private func selectedProfilePathState(
        for path: String?,
        role: WorkbenchRole
    ) -> SelectedProfilePathState {
        guard let path = nonEmptyTrimmed(path) else {
            return .none
        }
        var isDirectory: ObjCBool = false
        let exists = FileManager.default.fileExists(atPath: path, isDirectory: &isDirectory)
        if exists {
            return isDirectory.boolValue ? .directory : .existingFile
        }
        return role == .source ? .newDestination : .missingFile
    }

    private func profileSelectionNote(for path: String) -> String {
        switch selectedProfilePathState(for: path, role: selectedRole) {
        case .none:
            return "Choose a migration config file first."
        case .existingFile:
            return "Migration config selected. Run Lint Config or Status to load current evidence."
        case .newDestination:
            switch profileDestinationPlan(for: path) {
            case .initialize:
                return "New migration config destination selected. Review the current roots, then click Create Config File."
            case let .selectedOnly(note):
                return note
            }
        case .missingFile:
            return "Selected migration config file does not exist. Open an existing config file before reading evidence."
        case .directory:
            return "Selected path is a folder. Choose a .json migration config file."
        }
    }

    private func setupGuideConfigStep() -> SetupGuide.Step {
        let state: GateState
        let detail: String
        let primaryTitle: String?
        let primaryTask: SuperMoverTaskKind?
        let secondaryTitle: String?

        switch selectedProfilePathState {
        case .none:
            state = .pending
            detail =
                selectedRole == .source
                ? "Choose folders, then create the recommended setup. Existing and custom config files live in Advanced."
                : "Open an existing migration config file before reading evidence or running role tasks."
            primaryTitle = selectedRole == .source ? "Create Migration Setup" : "Open Existing Config"
            primaryTask = selectedRole == .source ? .profileInit : nil
            secondaryTitle = nil
        case .existingFile:
            state = .pass
            detail = "Existing migration config selected. It remains the source of truth for CLI execution."
            primaryTitle = "Open Existing Config"
            primaryTask = nil
            secondaryTitle = nil
        case .newDestination:
            state = selectedRole == .source ? .review : .blocked
            detail =
                selectedRole == .source
                ? "New destination selected. Write the migration config through the CLI before other tasks use it."
                : "Targets and observers need an existing migration config file."
            primaryTitle = selectedRole == .source ? "Create New Config File" : "Open Existing Config"
            primaryTask = selectedRole == .source ? .profileInit : nil
            secondaryTitle = nil
        case .missingFile:
            state = .blocked
            detail = selectedRole == .source
                ? "Selected config file is missing. Create the recommended setup instead, or open an existing file from Advanced."
                : "Selected config file is missing. Open an existing migration config file."
            primaryTitle = selectedRole == .source ? "Create Migration Setup" : "Open Existing Config"
            primaryTask = selectedRole == .source ? .profileInit : nil
            secondaryTitle = nil
        case .directory:
            state = .blocked
            detail = "A folder is selected. Choose a .json migration config file."
            primaryTitle = "Open Existing Config"
            primaryTask = nil
            secondaryTitle = nil
        }

        return SetupGuide.Step(
            id: "config",
            index: 1,
            title: "Migration config file",
            detail: detail,
            statusLabel: profileSelectionContext.pathState.filesystemSummary ?? "not selected",
            state: state,
            primaryActionTitle: primaryTitle,
            primaryTask: primaryTask,
            secondaryActionTitle: secondaryTitle
        )
    }

    private func setupGuideFoldersStep(sourceReadiness: String, targetReadiness: String) -> SetupGuide.Step {
        let title: String
        let detail: String
        let statusLabel: String
        let state: GateState
        let creatingConfig = selectedProfilePathState == .newDestination
        let hasSourceRootInput = nonEmptyTrimmed(sourceRootPath) != nil
        let hasTargetRootInput = nonEmptyTrimmed(targetRootPath) != nil

        switch selectedRole {
        case .source:
            title = "Choose folders"
            detail = "Choose this Mac's folder to move, then enter the destination path that the target Mac will own."
            if creatingConfig || hasSourceRootInput || hasTargetRootInput {
                let targetPathStatus = hasTargetRootInput ? "target path set" : "target path missing"
                statusLabel = "source \(sourceReadiness) / \(targetPathStatus)"
                state = (sourceReadiness == "readable" && hasTargetRootInput) ? .pass : .pending
            } else {
                statusLabel = "optional unless creating/updating"
                state = .neutral
            }
        case .target:
            title = "Destination folder"
            detail = "Use this folder only when explicitly updating the selected setup target. Lint and Status read the saved setup."
            if hasTargetRootInput {
                statusLabel = "target \(targetReadiness)"
                state = targetReadiness == "writable" ? .pass : .pending
            } else {
                statusLabel = "optional unless updating"
                state = .neutral
            }
        case .observer:
            title = "Evidence target"
            detail = "Observer mode reads existing target evidence; it does not author source or target folders."
            statusLabel = "not required"
            state = .neutral
        }

        return SetupGuide.Step(
            id: "folders",
            index: 2,
            title: title,
            detail: detail,
            statusLabel: statusLabel,
            state: state,
            primaryActionTitle: selectedRole == .observer ? nil : "Choose Folder",
            primaryTask: nil,
            secondaryActionTitle: nil
        )
    }

    private func setupGuideValidationStep() -> SetupGuide.Step {
        let lintPassed = hasSuccessfulRun(.lintProfile)
        let statusRead = hasSuccessfulRun(.status)
        let detail: String
        let primaryTitle: String
        let primaryTask: SuperMoverTaskKind
        let isValidated: Bool
        let statusLabel: String

        switch selectedRole {
        case .observer:
            detail = "Run Read Status to load evidence from the selected config without mutating target state."
            primaryTitle = "Read Status"
            primaryTask = .status
            isValidated = statusRead
            statusLabel = statusRead ? "status read" : "not validated"
        case .target:
            detail = "Run Lint Config or Read Status to confirm the selected config still matches durable evidence."
            primaryTitle = "Lint Existing Config"
            primaryTask = .lintProfile
            isValidated = lintPassed || statusRead
            statusLabel = lintPassed ? "lint passed" : (statusRead ? "status read" : "not validated")
        case .source:
            detail = "Create or open the config, then run Lint Config before treating setup as ready."
            primaryTitle = selectedProfilePathState == .existingFile ? "Lint Existing Config" : "Lint Config"
            primaryTask = .lintProfile
            isValidated = lintPassed
            statusLabel = lintPassed ? "lint passed" : "not validated"
        }

        return SetupGuide.Step(
            id: "validate",
            index: 3,
            title: "Validate before moving",
            detail: detail,
            statusLabel: statusLabel,
            state: isValidated ? .pass : .pending,
            primaryActionTitle: primaryTitle,
            primaryTask: primaryTask,
            secondaryActionTitle: selectedRole == .target ? "Read Status" : nil
        )
    }

    private func localizedSetupGuideConfigStep(using localization: AppChromeLocalization) -> SetupGuide.Step {
        let state: GateState
        let detail: String
        let primaryTitle: String?
        let primaryTask: SuperMoverTaskKind?
        let secondaryTitle: String?

        switch selectedProfilePathState {
        case .none:
            state = .pending
            detail = selectedRole == .source
                ? localization.text(.setupConfigDetailNoneSource)
                : localization.text(.setupConfigDetailNoneExistingOnly)
            primaryTitle = selectedRole == .source
                ? localization.text(.setupActionCreateRecommendedConfig)
                : localization.text(.setupActionOpenExistingConfig)
            primaryTask = selectedRole == .source ? .profileInit : nil
            secondaryTitle = nil
        case .existingFile:
            state = .pass
            detail = localization.text(.setupConfigDetailExistingFile)
            primaryTitle = localization.text(.setupActionOpenExistingConfig)
            primaryTask = nil
            secondaryTitle = nil
        case .newDestination:
            state = selectedRole == .source ? .review : .blocked
            detail = selectedRole == .source
                ? localization.text(.setupConfigDetailNewDestinationSource)
                : localization.text(.setupConfigDetailNewDestinationExistingOnly)
            primaryTitle = selectedRole == .source
                ? localization.text(.setupActionCreateNewConfigFile)
                : localization.text(.setupActionOpenExistingConfig)
            primaryTask = selectedRole == .source ? .profileInit : nil
            secondaryTitle = nil
        case .missingFile:
            state = .blocked
            detail = selectedRole == .source
                ? localization.text(.setupConfigDetailMissingFileSource)
                : localization.text(.setupConfigDetailMissingFile)
            primaryTitle = selectedRole == .source
                ? localization.text(.setupActionCreateRecommendedConfig)
                : localization.text(.setupActionOpenExistingConfig)
            primaryTask = selectedRole == .source ? .profileInit : nil
            secondaryTitle = nil
        case .directory:
            state = .blocked
            detail = localization.text(.setupConfigDetailDirectory)
            primaryTitle = localization.text(.setupActionOpenExistingConfig)
            primaryTask = nil
            secondaryTitle = nil
        }

        return SetupGuide.Step(
            id: "config",
            index: 1,
            title: localization.text(.setupConfigStepTitle),
            detail: detail,
            statusLabel: localizedProfilePathStateSummary(profileSelectionContext.pathState, using: localization)
                ?? localization.text(.setupStatusNotSelected),
            state: state,
            primaryActionTitle: primaryTitle,
            primaryTask: primaryTask,
            secondaryActionTitle: secondaryTitle
        )
    }

    private func localizedSetupGuideFoldersStep(
        using localization: AppChromeLocalization,
        sourceReadiness: String,
        targetReadiness: String
    ) -> SetupGuide.Step {
        let title: String
        let detail: String
        let statusLabel: String
        let state: GateState
        let creatingConfig = selectedProfilePathState == .newDestination
        let hasSourceRootInput = nonEmptyTrimmed(sourceRootPath) != nil
        let hasTargetRootInput = nonEmptyTrimmed(targetRootPath) != nil

        switch selectedRole {
        case .source:
            title = localization.text(.setupFoldersTitleSource)
            detail = localization.text(.setupFoldersDetailSource)
            if creatingConfig || hasSourceRootInput || hasTargetRootInput {
                let targetPathStatus = hasTargetRootInput
                    ? localization.text(.setupStatusTargetPathSet)
                    : localization.text(.setupStatusTargetPathMissing)
                statusLabel = [
                    localizedRootStatus(prefix: "source", readiness: sourceReadiness, using: localization),
                    targetPathStatus,
                ].joined(separator: " / ")
                state = (sourceReadiness == "readable" && hasTargetRootInput) ? .pass : .pending
            } else {
                statusLabel = localization.text(.setupStatusOptionalCreatingUpdating)
                state = .neutral
            }
        case .target:
            title = localization.text(.setupFoldersTitleTarget)
            detail = localization.text(.setupFoldersDetailTarget)
            if hasTargetRootInput {
                statusLabel = localizedRootStatus(prefix: "target", readiness: targetReadiness, using: localization)
                state = targetReadiness == "writable" ? .pass : .pending
            } else {
                statusLabel = localization.text(.setupStatusOptionalUpdating)
                state = .neutral
            }
        case .observer:
            title = localization.text(.setupFoldersTitleObserver)
            detail = localization.text(.setupFoldersDetailObserver)
            statusLabel = localization.text(.setupStatusNotRequired)
            state = .neutral
        }

        return SetupGuide.Step(
            id: "folders",
            index: 2,
            title: title,
            detail: detail,
            statusLabel: statusLabel,
            state: state,
            primaryActionTitle: selectedRole == .observer ? nil : localization.text(.setupActionChooseFolder),
            primaryTask: nil,
            secondaryActionTitle: nil
        )
    }

    private func localizedSetupGuideValidationStep(using localization: AppChromeLocalization) -> SetupGuide.Step {
        let lintPassed = hasSuccessfulRun(.lintProfile)
        let statusRead = hasSuccessfulRun(.status)
        let detail: String
        let primaryTitle: String
        let primaryTask: SuperMoverTaskKind
        let isValidated: Bool
        let statusLabel: String

        switch selectedRole {
        case .observer:
            detail = localization.text(.setupValidationDetailObserver)
            primaryTitle = localization.text(.setupActionReadStatus)
            primaryTask = .status
            isValidated = statusRead
            statusLabel = statusRead ? localization.text(.setupStatusStatusRead) : localization.text(.setupStatusNotValidated)
        case .target:
            detail = localization.text(.setupValidationDetailTarget)
            primaryTitle = localization.text(.setupActionLintExistingConfig)
            primaryTask = .lintProfile
            isValidated = lintPassed || statusRead
            statusLabel = lintPassed
                ? localization.text(.setupStatusLintPassed)
                : (statusRead ? localization.text(.setupStatusStatusRead) : localization.text(.setupStatusNotValidated))
        case .source:
            detail = localization.text(.setupValidationDetailSource)
            primaryTitle = selectedProfilePathState == .existingFile
                ? localization.text(.setupActionLintExistingConfig)
                : localization.text(.setupActionLintConfig)
            primaryTask = .lintProfile
            isValidated = lintPassed
            statusLabel = lintPassed ? localization.text(.setupStatusLintPassed) : localization.text(.setupStatusNotValidated)
        }

        return SetupGuide.Step(
            id: "validate",
            index: 3,
            title: localization.text(.setupValidationTitle),
            detail: detail,
            statusLabel: statusLabel,
            state: isValidated ? .pass : .pending,
            primaryActionTitle: primaryTitle,
            primaryTask: primaryTask,
            secondaryActionTitle: selectedRole == .target ? localization.text(.setupActionReadStatus) : nil
        )
    }

    private func localizedProfilePathStateSummary(
        _ pathState: SelectedProfilePathState,
        using localization: AppChromeLocalization
    ) -> String? {
        switch pathState {
        case .none:
            return nil
        case .existingFile:
            return localization.text(.setupStatusExistingConfigFile)
        case .newDestination:
            return localization.text(.setupStatusNewConfigDestination)
        case .missingFile:
            return localization.text(.setupStatusConfigFileMissing)
        case .directory:
            return localization.text(.setupStatusFolderSelected)
        }
    }

    private func localizedRootStatus(
        prefix: String,
        readiness: String,
        using localization: AppChromeLocalization
    ) -> String {
        let readinessKey: String
        switch readiness {
        case "readable":
            readinessKey = "readable"
        case "not readable":
            readinessKey = "notReadable"
        case "writable":
            readinessKey = "writable"
        case "not writable":
            readinessKey = "notWritable"
        default:
            readinessKey = "notSelected"
        }
        let localizedReadiness = localizedDirectoryReadiness(readiness, using: localization)
        let labelKey: AppChromeLocalization.Key = prefix == "source" ? .setupRootStatusSourceLabel : .setupRootStatusTargetLabel
        return localization.text(
            rawKey: "setup.rootStatus.\(prefix).\(readinessKey)",
            englishFallback: "\(localization.text(labelKey)) \(localizedReadiness)"
        )
    }

    private func localizedDirectoryReadiness(
        _ readiness: String,
        using localization: AppChromeLocalization
    ) -> String {
        switch readiness {
        case "readable":
            return localization.text(.setupDirectoryReadable)
        case "not readable":
            return localization.text(.setupDirectoryNotReadable)
        case "writable":
            return localization.text(.setupDirectoryWritable)
        case "not writable":
            return localization.text(.setupDirectoryNotWritable)
        default:
            return localization.text(.setupDirectoryNotSelected)
        }
    }

    private func nonEmptyTrimmed(_ value: String?) -> String? {
        guard let value else {
            return nil
        }
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    private var customizedDraftProfileID: String? {
        let trimmed = nonEmptyTrimmed(profileID)
        return trimmed == TaskInput.defaultProfileID ? nil : trimmed
    }

    private var customizedDraftProfileName: String? {
        let trimmed = nonEmptyTrimmed(profileName)
        return trimmed == TaskInput.defaultProfileName ? nil : trimmed
    }

    private func profileSelectionMetadata(for pathState: SelectedProfilePathState) -> String? {
        switch pathState {
        case .none:
            return nil
        case .existingFile:
            return nil
        case .newDestination:
            return "Ready to create through the selected file."
        case .missingFile:
            return "Config file missing"
        case .directory:
            return "Choose a .json migration config file."
        }
    }

    private func localizedProfileSelectionMetadata(
        for pathState: SelectedProfilePathState,
        using localization: AppChromeLocalization
    ) -> String? {
        switch pathState {
        case .none, .existingFile:
            return nil
        case .newDestination:
            return localization.text(.setupProfileNewConfigReadyMetadata)
        case .missingFile:
            return localization.text(.setupProfileMissingMetadata)
        case .directory:
            return localization.text(.setupProfileDirectoryMetadata)
        }
    }

    func currentContextSignature(for kind: SuperMoverTaskKind) -> String {
        ([setupContextSignature, kind.rawValue] + kind.contextInputs(using: currentInput))
            .map(normalizedForSignature)
            .joined(separator: "\u{1F}")
    }

    func isCurrentContext(_ run: TaskRun) -> Bool {
        run.contextSignature == currentContextSignature(for: run.kind)
    }

    func browseProfile() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = false
        panel.canChooseFiles = true
        panel.allowsMultipleSelection = false
        panel.allowedContentTypes = [.json]
        if panel.runModal() == .OK, let url = panel.url {
            profilePath = url.path
            note = "Migration config selected. Run Lint Config or Status to load current evidence."
        }
    }

    func useRecommendedProfileDestination() {
        applyProfileDestinationSelection(recommendedProfileDestinationPath)
    }

    func chooseProfileDestination() {
        let panel = NSSavePanel()
        panel.allowedContentTypes = [.json]
        panel.canCreateDirectories = true
        panel.nameFieldStringValue = profilePath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            ? URL(fileURLWithPath: recommendedProfileDestinationPath).lastPathComponent
            : URL(filePath: profilePath).lastPathComponent
        if panel.runModal() == .OK, let url = panel.url {
            applyProfileDestinationSelection(url.path)
        }
    }

    func profileDestinationPlan(for path: String) -> ProfileDestinationPlan {
        let trimmedPath = path.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedPath.isEmpty else {
            return .selectedOnly(note: "Choose a migration config destination first.")
        }

        let state = selectedProfilePathState(for: trimmedPath, role: selectedRole)
        guard selectedRole == .source else {
            return .selectedOnly(note: "Migration config destination selected.")
        }
        switch state {
        case .directory:
            return .selectedOnly(
                note: "That selection is a folder. Choose a .json migration config file, not a directory."
            )
        case .existingFile:
            return .selectedOnly(
                note:
                    "That migration config file already exists. Open it as an existing config, or choose a new destination."
            )
        case .none, .missingFile:
            return .selectedOnly(note: "Choose a migration config destination first.")
        case .newDestination:
            break
        }
        guard state == .newDestination else {
            return .selectedOnly(
                note:
                    "That migration config file already exists. Open it as an existing config, or choose a new destination."
            )
        }

        var input = currentInput
        input.profilePath = trimmedPath

        let sourceRoot = input.sourceRootPath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !sourceRoot.isEmpty else {
            return .selectedOnly(
                note: "New migration config destination selected. Choose a readable source root before writing the config file."
            )
        }
        let targetRoot = input.targetRootPath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !targetRoot.isEmpty else {
            return .selectedOnly(
                note: "New migration config destination selected. Enter the destination path from the target Mac before writing the config file."
            )
        }
        guard isReadableDirectory(sourceRoot) else {
            return .selectedOnly(
                note:
                    "New migration config destination selected. Choose an accessible source directory before writing the config file."
            )
        }

        return .initialize(
            arguments: SuperMoverTaskKind.profileInit.buildArguments(using: input),
            note: "Writing migration config through CLI. Run Lint Config before treating setup as ready."
        )
    }

    func applyProfileDestinationSelection(_ path: String) {
        profilePath = path
        note = profileSelectionNote(for: path)
    }

    func browseSourceRoot() {
        browseDirectory { [weak self] path in
            self?.sourceRootPath = path
        }
    }

    func browseTargetRoot() {
        browseDirectory { [weak self] path in
            self?.targetRootPath = path
        }
    }

    func choosePairingReceiptExportTarget() {
        let panel = NSSavePanel()
        panel.canCreateDirectories = true
        panel.allowedContentTypes = [.json]
        panel.nameFieldStringValue = pairingReceipt.trimmedExportTarget.isEmpty ? "pairing-receipt.json" : URL(fileURLWithPath: pairingReceipt.trimmedExportTarget).lastPathComponent
        if panel.runModal() == .OK, let url = panel.url {
            pairingReceipt.exportTarget = url.path
        }
    }

    func browsePairingReceiptImportFile() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = false
        panel.canChooseFiles = true
        panel.allowsMultipleSelection = false
        panel.allowedContentTypes = [.json]
        if panel.runModal() == .OK, let url = panel.url {
            pairingReceipt.importReceiptFile = url.path
        }
    }

    private func browseDirectory(assign: (String) -> Void) {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        if panel.runModal() == .OK, let url = panel.url {
            assign(url.path)
        }
    }

    func taskRunGate(for task: SuperMoverTaskKind? = nil) -> TaskRunGate {
        let task = task ?? selectedTask
        let input = currentInput
        guard !task.requiresProfile || !profilePath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            return .blocked("Select a migration config file first.")
        }
        if task == .profileInit, selectedProfilePathState != .newDestination {
            return .blocked("Choose a new migration config destination before running Create Config File. Existing config files must be opened, not overwritten.")
        }
        if task.requiresExistingProfile, selectedProfilePathState != .existingFile {
            return .blocked("Open an existing migration config file before running \(task.displayTitle). A new config destination is only valid for Create Config File.")
        }
        guard selectedRole.allows(task: task) else {
            return .blocked("\(selectedRole.title) role cannot run \(task.displayTitle) in this setup shell. Switch roles or choose a role-allowed evidence task.")
        }
        if task.requiresSourceRoot && input.sourceRootPath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            return .blocked("Select a readable source root first.")
        }
        if task.requiresTargetRootInput && input.targetRootPath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            if task == .profileInit {
                return .blocked("Enter the destination path from the target Mac first.")
            }
            return .blocked("Select a target root first.")
        }
        if task.requiresSourceRoot, !isReadableDirectory(input.sourceRootPath) {
            return .blocked("Source root is not readable by this app. Choose an accessible source directory before writing a migration config.")
        }
        if task.requiresWritableTargetRoot, !isWritableDirectory(input.targetRootPath) {
            return .blocked("Target root is not writable by this app. Choose an accessible target directory before writing target config evidence.")
        }
        if task.requiresSessionID && input.requiredSessionID.isEmpty {
            return .blocked("Provide an explicit session id first. This app no longer creates hidden mutating session defaults.")
        }
        if task.requiresSessionPrefix && input.requiredSessionPrefix.isEmpty {
            return .blocked("Provide an explicit session prefix first. Foreground sync loops write generated run receipts from that prefix.")
        }
        if task.requiresQueueEntryID && input.requiredQueueEntryID.isEmpty {
            return .blocked("Provide a durable queue entry id first.")
        }
        if task.requiresPairingTargetAddress && input.requiredPairingTargetAddress.isEmpty {
            return .blocked("Provide a target address first. Discovery hints can fill the address field, but pairing still requires target-side code verification.")
        }
        if task.requiresPairingVerificationCode && input.requiredPairingVerificationCode.isEmpty {
            return .blocked("Provide the verification code shown by target serve first.")
        }
        if task.requiresImportReceiptFile && input.pairingReceipt.trimmedImportReceiptFile.isEmpty {
            return .blocked("Select an exported pairing receipt file first.")
        }
        if task.requiresSingleDriftID && input.driftIDs.isEmpty {
            return .blocked("Provide one persisted drift id first.")
        }
        if task.requiresSingleDriftID && input.driftIDs.count != 1 {
            return .blocked("Provide exactly one persisted drift id for this task.")
        }
        if task.requiresAnyDriftIDs && input.driftIDs.isEmpty {
            return .blocked("Provide at least one persisted drift id first.")
        }
        if task.requiresApprovalID && input.requiredApprovalID.isEmpty {
            return .blocked("Provide an approval id first.")
        }
        if task.requiresSoftDeleteIDs && input.softDeleteIDs.isEmpty {
            return .blocked("Provide at least one soft-delete id first.")
        }
        if task.requiresReviewer && input.requiredReviewer.isEmpty {
            return .blocked("Provide a reviewer first.")
        }
        if task.requiresReason && input.requiredReason.isEmpty {
            return .blocked("Provide a reason first.")
        }
        if let preflightError = acceptanceInstalledAppLaunchPreflightError(for: task) {
            return .blocked(preflightError)
        }
        return .runnable
    }

    func runSelectedTask() {
        let gate = taskRunGate()
        guard gate.isRunnable else {
            let message = gate.note ?? "Task is blocked by the current app gate."
            note = message
            if acceptanceInstalledAppLaunchPreflightError(for: selectedTask) == message {
                appendAppEvent(
                    severity: .review,
                    title: "acceptance launch blocked",
                    detail: message
                )
            }
            return
        }
        let args = selectedTask.buildArguments(using: currentInput)
        launch(kind: selectedTask, arguments: args)
    }

    func acceptanceInstalledAppLaunchPreflightError(for kind: SuperMoverTaskKind) -> String? {
        acceptanceInstalledAppLaunchCoordinator.preflightError(
            for: kind,
            using: acceptanceInstalledAppLaunchDependencies()
        )
    }

    func acceptanceInstalledAppLaunchPreview(for kind: SuperMoverTaskKind) -> AcceptanceInstalledAppLaunchPreview? {
        acceptanceInstalledAppLaunchCoordinator.preview(
            for: kind,
            using: acceptanceInstalledAppLaunchDependencies()
        )
    }

    private func refreshSelectedTaskAcceptanceLaunchPreview() {
        selectedTaskAcceptanceLaunchPreview = acceptanceInstalledAppLaunchPreview(for: selectedTask)
    }

    private func captureSourceConsistencyProofIfAvailableAndFinalizeNetworkPush(for finished: TaskRun) {
        guard finished.kind == .networkPush, finished.slot == .foregroundAction, isCurrentContext(finished) else {
            return
        }
        guard networkPushBaselineContextSignatures[finished.slot] == finished.contextSignature else {
            completeNetworkPushPostSuccess(for: finished)
            return
        }
        guard let profilePath = selectedProfilePath,
              let baselineURL = networkPushBaselineFileURL(for: finished.slot, kind: finished.kind),
              FileManager.default.fileExists(atPath: baselineURL.path) else {
            appendAppEvent(
                severity: .review,
                title: "source consistency baseline missing",
                detail: "Network Push finished without a readable source baseline sidecar."
            )
            sourceConsistencySnapshot = synthesizedSourceConsistencyFailureSnapshot(
                mode: "baseline_missing",
                sessionID: currentInput.requiredSessionID,
                detail: "Network Push finished without a readable source baseline sidecar."
            )
            completeNetworkPushPostSuccess(for: finished)
            return
        }

        let arguments = [
            "verify", "source-consistency",
            "--profile", profilePath,
            "--baseline", baselineURL.path,
            "--format", "json",
        ]

        if let cliCommandRunner {
            applySourceConsistencyCaptureResult(
                Result { try cliCommandRunner(arguments) },
                for: finished
            )
            return
        }

        DispatchQueue.global(qos: .utility).async {
            let result = Result { try Self.executeCLICommand(arguments: arguments, runner: nil) }
            Task { @MainActor [weak self] in
                self?.applySourceConsistencyCaptureResult(result, for: finished)
            }
        }
    }

    private func applySourceConsistencyCaptureResult(
        _ result: Result<CLICommandOutput, Error>,
        for finished: TaskRun
    ) {
        defer { completeNetworkPushPostSuccess(for: finished) }

        switch result {
        case let .success(output):
            storeSourceConsistencyCaptureOutput(output, for: finished)
        case let .failure(error):
            if isCurrentContext(finished) {
                sourceConsistencySnapshot = synthesizedSourceConsistencyFailureSnapshot(
                    mode: "capture_failed",
                    sessionID: currentInput.requiredSessionID,
                    detail: "Bundled CLI source-consistency capture failed: \(error.localizedDescription)"
                )
            }
            appendAppEvent(
                severity: .review,
                title: "source consistency capture failed",
                detail: error.localizedDescription
            )
        }
    }

    private func storeSourceConsistencyCaptureOutput(
        _ output: CLICommandOutput,
        for finished: TaskRun
    ) {
        let freshness: StructuredEvidenceFreshness = isCurrentContext(finished) ? .current : .stale
        let envelope = StructuredEvidenceEnvelope(
            artifactKind: .sourceConsistency,
            task: .networkPush,
            loadedAt: Date(),
            contextSignature: finished.contextSignature,
            exitCode: output.exitCode,
            rawStdout: output.stdout,
            stderrSample: String(output.stderr.trimmingCharacters(in: .whitespacesAndNewlines).prefix(2000)),
            freshness: freshness
        )
        appendEvidenceEnvelope(envelope, promoteCurrent: freshness == .current)

        let decoded: AcceptanceBundleSnapshot.SourceConsistencyEvidence? = {
            guard let data = output.stdout.data(using: .utf8) else {
                recordArtifactReadProblem(
                    .sourceConsistency,
                    task: .networkPush,
                    problem: "stdout is not valid UTF-8",
                    raw: output.stdout
                )
                return nil
            }
            return decodeStructuredArtifact(.sourceConsistency, task: .networkPush, data: data)
        }()

        if let decoded {
            if freshness == .current {
                sourceConsistencySnapshot = sourceConsistencyEvidenceBoundToCurrentContext(decoded)
            }
            appendAppEvent(
                severity: output.exitCode == 0 ? .info : .review,
                title: "source consistency captured",
                detail: output.exitCode == 0 ? "Captured current-source proof from bundled CLI baseline compare." : "Bundled CLI reported current-source mismatch after network push."
            )
            return
        }

        if freshness == .current {
            sourceConsistencySnapshot = synthesizedSourceConsistencyFailureSnapshot(
                mode: "artifact_invalid",
                sessionID: currentInput.requiredSessionID,
                detail: "Bundled CLI did not emit valid current-source JSON. Review artifact read problems before trusting current-source proof."
            )
        }
        appendAppEvent(
            severity: .review,
            title: "source consistency capture invalid",
            detail: "Bundled CLI did not emit valid current-source JSON."
        )
    }

    private func completeNetworkPushPostSuccess(for finished: TaskRun) {
        guard isCurrentContext(finished) else {
            appendAppEvent(
                severity: .review,
                title: "network push post-processing stale",
                detail: "Current-source capture finished after setup inputs changed, so acceptance auto-record was skipped."
            )
            cleanupNetworkPushBaselineFile(for: finished.slot)
            return
        }
        if autoRecordAcceptanceArtifactIfConfigured(for: .networkPush) {
            cleanupNetworkPushBaselineFile(for: finished.slot)
        }
    }

    private func synthesizedSourceConsistencyFailureSnapshot(
        mode: String,
        sessionID: String,
        detail: String
    ) -> AcceptanceBundleSnapshot.SourceConsistencyEvidence {
        AcceptanceBundleSnapshot.SourceConsistencyEvidence(
            schema: "supermover.acceptance.current_source_consistency.v1",
            output: nil,
            baseline: nil,
            status: "blocked",
            mode: mode,
            session_id: sessionID.isEmpty ? nil : sessionID,
            entry_count: nil,
            mismatch_count: nil,
            detail: detail
        )
    }

    private func cleanupCompletedLaunchArtifacts(
        slot: SupervisedProcessSlot,
        kind: SuperMoverTaskKind,
        finishedSuccessfully: Bool
    ) {
        switch kind {
        case .serve:
            cleanupServeReadyFile(for: slot)
        case .networkPush where !finishedSuccessfully:
            cleanupNetworkPushBaselineFile(for: slot)
        default:
            break
        }
    }

    private func cleanupServeReadyFile(for slot: SupervisedProcessSlot) {
        guard let path = serveReadyFilePaths.removeValue(forKey: slot) else {
            serveReadyFileContextSignatures.removeValue(forKey: slot)
            return
        }
        serveReadyFileContextSignatures.removeValue(forKey: slot)
        try? FileManager.default.removeItem(atPath: path)
    }

    private func cleanupNetworkPushBaselineFile(for slot: SupervisedProcessSlot) {
        guard let path = networkPushBaselineFilePaths.removeValue(forKey: slot) else {
            networkPushBaselineContextSignatures.removeValue(forKey: slot)
            return
        }
        networkPushBaselineContextSignatures.removeValue(forKey: slot)
        try? FileManager.default.removeItem(atPath: path)
    }

    nonisolated private static func executeCLICommand(
        arguments: [String],
        runner: (([String]) throws -> CLICommandOutput)?
    ) throws -> CLICommandOutput {
        if let runner {
            return try runner(arguments)
        }
        let invocation = try CLIResolver.resolve(arguments: arguments)
        let process = Process()
        process.executableURL = invocation.executableURL
        process.arguments = invocation.arguments
        process.currentDirectoryURL = invocation.workingDirectoryURL
        process.environment = ProcessInfo.processInfo.environment
        let stdoutPipe = Pipe()
        let stderrPipe = Pipe()
        process.standardOutput = stdoutPipe
        process.standardError = stderrPipe
        try process.run()
        process.waitUntilExit()
        let stdout = String(decoding: stdoutPipe.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
        let stderr = String(decoding: stderrPipe.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
        return (stdout, stderr, process.terminationStatus)
    }

    private func normalizedProfileContextPath(_ path: String?) -> String {
        guard let path = nonEmptyTrimmed(path) else {
            return ""
        }
        return URL(fileURLWithPath: path).standardizedFileURL.path
    }

    private func currentSourceContextMismatchSnapshot(
        from snapshot: AcceptanceBundleSnapshot.SourceConsistencyEvidence,
        mode: String,
        sessionID: String?,
        detail: String
    ) -> AcceptanceBundleSnapshot.SourceConsistencyEvidence {
        AcceptanceBundleSnapshot.SourceConsistencyEvidence(
            schema: snapshot.schema,
            output: snapshot.output,
            baseline: snapshot.baseline,
            status: "blocked",
            mode: mode,
            session_id: sessionID,
            entry_count: snapshot.entry_count,
            mismatch_count: snapshot.mismatch_count,
            detail: detail
        )
    }

    private func sourceConsistencyEvidenceBoundToCurrentContext(
        _ snapshot: AcceptanceBundleSnapshot.SourceConsistencyEvidence,
        sourceProfilePath: String? = nil,
        requiresProfileBinding: Bool = false
    ) -> AcceptanceBundleSnapshot.SourceConsistencyEvidence {
        if requiresProfileBinding || sourceProfilePath != nil {
            let currentProfilePath = normalizedProfileContextPath(selectedProfilePath)
            if currentProfilePath.isEmpty {
                return currentSourceContextMismatchSnapshot(
                    from: snapshot,
                    mode: "profile_missing",
                    sessionID: currentInput.requiredSessionID.isEmpty ? snapshot.session_id : currentInput.requiredSessionID,
                    detail: "Current-source proof requires a currently selected migration config before it can be treated as app-context evidence."
                )
            }

            let evidenceProfilePath = normalizedProfileContextPath(sourceProfilePath)
            if evidenceProfilePath.isEmpty {
                return currentSourceContextMismatchSnapshot(
                    from: snapshot,
                    mode: "profile_missing",
                    sessionID: currentInput.requiredSessionID.isEmpty ? snapshot.session_id : currentInput.requiredSessionID,
                    detail: "Current-source proof in the loaded acceptance bundle is not bound to a profile path."
                )
            }

            if currentProfilePath != evidenceProfilePath {
                return currentSourceContextMismatchSnapshot(
                    from: snapshot,
                    mode: "profile_mismatch",
                    sessionID: currentInput.requiredSessionID.isEmpty ? snapshot.session_id : currentInput.requiredSessionID,
                    detail: "Current-source proof belongs to a different profile path than the current app selection."
                )
            }
        }

        let status = snapshot.status.trimmingCharacters(in: .whitespacesAndNewlines)
        let mode = snapshot.mode.trimmingCharacters(in: .whitespacesAndNewlines)
        guard status == "pass", mode == "current_source_verified" else {
            return snapshot
        }

        let currentSessionID = currentInput.requiredSessionID
        guard !currentSessionID.isEmpty else {
            return currentSourceContextMismatchSnapshot(
                from: snapshot,
                mode: "session_missing",
                sessionID: snapshot.session_id,
                detail: "Current-source proof requires the current transfer session input before it can be treated as app-context evidence."
            )
        }
        let proofSessionID = snapshot.session_id?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !proofSessionID.isEmpty, proofSessionID == currentSessionID else {
            return currentSourceContextMismatchSnapshot(
                from: snapshot,
                mode: "session_mismatch",
                sessionID: currentSessionID,
                detail: "Current-source proof session_id is missing or does not match the current transfer session input."
            )
        }
        return snapshot
    }

    func usePairingTargetAddress(_ address: String) {
        pairingTargetAddress = address
        note = "Filled target address from an untrusted discovery hint. Pairing still depends on target-side verification code confirmation."
    }

    func selectTaskAndRun(_ task: SuperMoverTaskKind) {
        selectedTask = task
        runSelectedTask()
    }

    func directoryReadiness(path: String, requiresWrite: Bool) -> String {
        let trimmed = path.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.isEmpty {
            return "not selected"
        }
        var isDirectory: ObjCBool = false
        guard FileManager.default.fileExists(atPath: trimmed, isDirectory: &isDirectory), isDirectory.boolValue else {
            return "missing"
        }
        if requiresWrite {
            return FileManager.default.isWritableFile(atPath: trimmed) ? "writable" : "not writable"
        }
        return FileManager.default.isReadableFile(atPath: trimmed) ? "readable" : "not readable"
    }

    func refreshEvidenceArtifactCatalog() {
        let trimmed = targetRootPath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            evidenceArtifactCatalog = nil
            note = "Select a target root before loading .supermover artifacts."
            return
        }
        guard directoryReadiness(path: trimmed, requiresWrite: false) == "readable" else {
            evidenceArtifactCatalog = nil
            note = "Target root is not readable by this app. Choose an accessible target directory before loading artifacts."
            return
        }

        let catalog = EvidenceArtifactCatalogReader().read(
            targetRootURL: URL(fileURLWithPath: trimmed, isDirectory: true)
        )
        evidenceArtifactCatalog = catalog
        let artifactCount = catalog.artifacts.count
        let problemCount = catalog.problems.count
        note = "Loaded \(artifactCount) .supermover artifact\(artifactCount == 1 ? "" : "s") with \(problemCount) catalog problem\(problemCount == 1 ? "" : "s")."
        appendAppEvent(
            severity: problemCount == 0 ? .info : .review,
            title: "artifact catalog loaded",
            detail: "\(artifactCount) artifact(s), \(problemCount) problem(s) from \(catalog.controlPlaneURL.path)."
        )
    }

    func browseAcceptanceBundle() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        if panel.runModal() == .OK, let url = panel.url {
            acceptanceBundlePath = url.path
            refreshAcceptanceBundle()
        }
    }

    func browseAcceptanceOperatorArtifact() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = false
        panel.canChooseFiles = true
        panel.allowsMultipleSelection = false
        if panel.runModal() == .OK, let url = panel.url {
            acceptanceOperatorEvidence.artifactPath = url.path
        }
    }

    func clearAcceptanceOperatorArtifact() {
        acceptanceOperatorEvidence.artifactPath = ""
    }

    func recordAcceptanceDiscoveryBrowseArtifact() {
        guard let snapshot = discoveryBrowseSnapshot else {
            note = "Run discovery browse first."
            return
        }
        recordAcceptanceArtifact {
            try acceptanceBundleAuthoringCoordinator.writeBrowse(
                bundleRootURL: try currentAcceptanceBundleURL(),
                snapshot: snapshot
            )
        }
    }

    func recordAcceptanceDiscoveryAdvertiseArtifact() {
        guard let snapshot = discoveryAdvertiseSnapshot else {
            note = "Run discovery advertise first."
            return
        }
        let input = currentInput
        recordAcceptanceArtifact {
            try acceptanceBundleAuthoringCoordinator.writeAdvertise(
                bundleRootURL: try currentAcceptanceBundleURL(),
                snapshot: snapshot,
                profilePath: input.profilePath
            )
        }
    }

    func recordAcceptanceServePhaseArtifact() {
        guard let readiness = serveReadinessSnapshot else {
            note = "Run target serve first."
            return
        }
        guard let phase = Int(acceptanceServePhase.trimmingCharacters(in: .whitespacesAndNewlines)), phase > 0 else {
            note = "Serve phase must be a positive integer."
            return
        }
        let input = currentInput
        recordAcceptanceArtifact {
            try acceptanceBundleAuthoringCoordinator.writeServePhase(
                bundleRootURL: try currentAcceptanceBundleURL(),
                phase: phase,
                readiness: readiness,
                profilePath: input.profilePath
            )
        }
    }

    func recordAcceptanceSourcePairArtifact() {
        let input = currentInput
        recordAcceptanceArtifact {
            try acceptanceBundleAuthoringCoordinator.writeSourcePair(
                bundleRootURL: try currentAcceptanceBundleURL(),
                input: input
            )
        }
    }

    func recordAcceptanceSourceTransferArtifact() {
        let input = currentInput
        recordAcceptanceArtifact {
            let bundleURL = try currentAcceptanceBundleURL()
            let coordinator = acceptanceBundleAuthoringCoordinator
            let transferArtifacts = try coordinator.writeStructuredTransferArtifacts(
                bundleRootURL: bundleURL,
                verifyEnvelope: currentEvidenceEnvelope(.verify),
                statusEnvelope: currentEvidenceEnvelope(.status),
                reportEnvelope: currentEvidenceEnvelope(.report),
                healthEnvelope: currentEvidenceEnvelope(.health)
            )
            return try coordinator.writeSourceTransfer(
                bundleRootURL: bundleURL,
                input: input,
                fallbackTargetMode: serveReadinessSnapshot?.mode ?? "pairing",
                sourceBaselineURL: networkPushBaselineFileURL(for: .foregroundAction, kind: .networkPush),
                sourceConsistencyEnvelope: currentEvidenceEnvelope(.sourceConsistency),
                verifyArtifactPath: transferArtifacts.verify,
                statusArtifactPath: transferArtifacts.status,
                reportArtifactPath: transferArtifacts.report,
                healthArtifactPath: transferArtifacts.health,
                pushStdout: currentRunStdout(for: .networkPush)
            )
        }
    }

    func recordAcceptanceTargetImportArtifact() {
        let input = currentInput
        recordAcceptanceArtifact {
            try acceptanceBundleAuthoringCoordinator.writeTargetImport(
                bundleRootURL: try currentAcceptanceBundleURL(),
                input: input,
                adoptedStdout: currentRunStdout(for: .profileAdoptPairing)
            )
        }
    }

    func recordAcceptanceEvaluationArtifact() {
        guard selectedRole != .source else {
            note = "Source role cannot finalize acceptance evaluation. Switch to target or observer after loading target-side evidence."
            return
        }
        let targetRoot = targetRootPath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !targetRoot.isEmpty else {
            note = "Choose a target root before writing acceptance evaluation."
            return
        }
        recordAcceptanceArtifact {
            try acceptanceBundleOperations.recordEvaluation(
                bundleRootURL: try currentAcceptanceBundleURL(),
                targetRootURL: URL(fileURLWithPath: targetRoot, isDirectory: true),
                requireOperatorEvidence: acceptanceEvaluationMode.requireOperatorEvidence
            )
        }
    }

    var acceptanceEvaluationMode: AcceptanceEvaluationMode {
        AcceptanceEvaluationMode.resolve(
            snapshot: acceptanceBundleSnapshot,
            draft: acceptanceEvaluation
        )
    }

    func setAcceptanceEvaluationRequireOperatorEvidence(_ value: Bool) {
        if acceptanceEvaluationMode.isLockedForTwoMachineCollection {
            acceptanceEvaluation.requireOperatorEvidence = true
            return
        }
        acceptanceEvaluation.requireOperatorEvidence = value
    }

    func recordAcceptancePackagingEvidence() {
        guard selectedRole != .observer else {
            note = "Observer role cannot record local packaging evidence into the acceptance bundle."
            return
        }
        let machine = selectedRole == .source ? "source" : "target"
        recordAcceptanceArtifact {
            try acceptanceBundleOperations.recordPackagingEvidence(
                bundleRootURL: try currentAcceptanceBundleURL(),
                machine: machine,
                collectedBy: "app-\(selectedRole.rawValue)"
            )
        }
    }

    func recordAcceptanceOperatorEvidence() {
        guard selectedRole != .observer else {
            note = "Observer role cannot record manual evidence into the acceptance bundle."
            return
        }
        let trimmed = acceptanceBundlePath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            note = "Choose an acceptance bundle directory first."
            return
        }
        let machineIdentity = operatorEvidenceMachineIdentity(
            kind: acceptanceOperatorEvidence.kind
        )
        if acceptanceOperatorEvidence.status == .pass, machineIdentity == nil {
            note = "Record source/target machine facts before writing pass manual evidence."
            return
        }
        let record = acceptanceOperatorEvidence.makeRecord(machineIdentity: machineIdentity)
        let url = URL(fileURLWithPath: trimmed, isDirectory: true)
        do {
            try AcceptanceBundleMetaStore().recordOperatorEvidence(
                bundleRootURL: url,
                record: record
            )
            refreshAcceptanceBundle()
            note = "Recorded manual evidence for \(acceptanceOperatorEvidence.kind.title)."
            appendAppEvent(
                severity: record.status == "pass" ? .info : .review,
                title: "manual evidence recorded",
                detail: "\(record.kind)=\(record.status) in \(trimmed)"
            )
        } catch {
            note = error.localizedDescription
            acceptanceBundleLoadError = error.localizedDescription
            appendAppEvent(
                severity: .review,
                title: "manual evidence rejected",
                detail: error.localizedDescription
            )
        }
    }

    private func operatorEvidenceMachineIdentity(
        kind: AcceptanceOperatorEvidenceKind
    ) -> AcceptanceOperatorEvidenceMachineIdentity? {
        let facts: AcceptanceBundleSnapshot.MachineFactsArtifact?
        switch kind {
        case .localNetwork, .firewall:
            facts = acceptanceBundleSnapshot?.targetMachineFactsArtifact
        case .pairingConfirmation:
            facts = acceptanceBundleSnapshot?.sourceMachineFactsArtifact
        }
        guard let machineID = facts?.machine_id.trimmingCharacters(in: .whitespacesAndNewlines),
              !machineID.isEmpty else {
            return nil
        }
        let machineLabel = facts?.machine_label?.trimmingCharacters(in: .whitespacesAndNewlines)
        return AcceptanceOperatorEvidenceMachineIdentity(
            id: machineID,
            label: machineLabel?.isEmpty == true ? nil : machineLabel
        )
    }

    private func currentAcceptanceBundleURL() throws -> URL {
        let trimmed = acceptanceBundlePath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingBundlePath
        }
        return URL(fileURLWithPath: trimmed, isDirectory: true)
    }

    private func recordAcceptanceArtifact(
        _ write: () throws -> AcceptanceBundleArtifactAuthoringResult
    ) {
        do {
            let result = try write()
            refreshAcceptanceBundle()
            note = "Recorded \(result.kind.rawValue) into acceptance bundle."
            appendAppEvent(
                severity: .info,
                title: "acceptance artifact recorded",
                detail: result.detail
            )
        } catch {
            note = error.localizedDescription
            appendAppEvent(
                severity: .review,
                title: "acceptance artifact rejected",
                detail: error.localizedDescription
            )
        }
    }

    @discardableResult
    private func autoRecordAcceptanceArtifactIfConfigured(for kind: SuperMoverTaskKind) -> Bool {
        guard !acceptanceBundlePath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            return false
        }
        let input = currentInput
        let coordinator = acceptanceBundleAuthoringCoordinator
        do {
            let result: AcceptanceBundleArtifactAuthoringResult?
            switch kind {
            case .discoverBrowse:
                if let snapshot = discoveryBrowseSnapshot {
                    result = try coordinator.writeBrowse(
                        bundleRootURL: currentAcceptanceBundleURL(),
                        snapshot: snapshot
                    )
                } else {
                    result = nil
                }
            case .discoverAdvertise:
                if let snapshot = discoveryAdvertiseSnapshot {
                    result = try coordinator.writeAdvertise(
                        bundleRootURL: currentAcceptanceBundleURL(),
                        snapshot: snapshot,
                        profilePath: input.profilePath
                    )
                } else {
                    result = nil
                }
            case .serve:
                if let readiness = serveReadinessSnapshot,
                   let phase = Int(acceptanceServePhase.trimmingCharacters(in: .whitespacesAndNewlines)),
                   phase > 0 {
                    result = try coordinator.writeServePhase(
                        bundleRootURL: currentAcceptanceBundleURL(),
                        phase: phase,
                        readiness: readiness,
                        profilePath: input.profilePath
                    )
                } else {
                    result = nil
                }
            case .pair:
                result = try coordinator.writeSourcePair(
                    bundleRootURL: currentAcceptanceBundleURL(),
                    input: input,
                    pairStdout: currentRunStdout(for: .pair)
                )
            case .profileAdoptPairing:
                result = try coordinator.writeTargetImport(
                    bundleRootURL: currentAcceptanceBundleURL(),
                    input: input,
                    adoptedStdout: currentRunStdout(for: .profileAdoptPairing)
                )
            case .networkPush:
                let bundleURL = try currentAcceptanceBundleURL()
                let transferArtifacts = try coordinator.writeStructuredTransferArtifacts(
                    bundleRootURL: bundleURL,
                    verifyEnvelope: currentEvidenceEnvelope(.verify),
                    statusEnvelope: currentEvidenceEnvelope(.status),
                    reportEnvelope: currentEvidenceEnvelope(.report),
                    healthEnvelope: currentEvidenceEnvelope(.health)
                )
                result = try coordinator.writeSourceTransfer(
                    bundleRootURL: bundleURL,
                    input: input,
                    fallbackTargetMode: serveReadinessSnapshot?.mode ?? "pairing",
                    sourceBaselineURL: networkPushBaselineFileURL(for: .foregroundAction, kind: .networkPush),
                    sourceConsistencyEnvelope: currentEvidenceEnvelope(.sourceConsistency),
                    verifyArtifactPath: transferArtifacts.verify,
                    statusArtifactPath: transferArtifacts.status,
                    reportArtifactPath: transferArtifacts.report,
                    healthArtifactPath: transferArtifacts.health,
                    pushStdout: currentRunStdout(for: .networkPush)
                )
            default:
                result = nil
            }
            guard let result else {
                return false
            }
            refreshAcceptanceBundle()
            appendAppEvent(
                severity: .info,
                title: "acceptance artifact auto-recorded",
                detail: result.detail
            )
            return true
        } catch {
            appendAppEvent(
                severity: .review,
                title: "acceptance artifact auto-record skipped",
                detail: "\(kind.rawValue): \(error.localizedDescription)"
            )
            return false
        }
    }

    private func currentEvidenceEnvelope(_ kind: StructuredArtifactKind) -> StructuredEvidenceEnvelope? {
        guard let envelope = evidenceEnvelopes[kind], envelope.freshness == .current else {
            return nil
        }
        let trimmed = envelope.rawStdout.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : envelope
    }

    private func currentRunStdout(for kind: SuperMoverTaskKind) -> String? {
        guard let run = recentRuns.first(where: { candidate in
            guard candidate.kind == kind, isCurrentContext(candidate) else {
                return false
            }
            if case .finished(0) = candidate.state {
                return true
            }
            return false
        }) else {
            return nil
        }
        let trimmed = run.stdout.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : run.stdout
    }

    @MainActor
    func triggerAcceptanceAutoRecordForTesting(_ kind: SuperMoverTaskKind) {
        autoRecordAcceptanceArtifactIfConfigured(for: kind)
    }

    func refreshAcceptanceBundle() {
        let trimmed = acceptanceBundlePath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            acceptanceBundleSnapshot = nil
            acceptanceBundleLoadError = "Choose an acceptance bundle directory first."
            note = acceptanceBundleLoadError
            return
        }
        let url = URL(fileURLWithPath: trimmed, isDirectory: true)
        do {
            let snapshot = try AcceptanceBundleReader().load(bundleRootURL: url)
            acceptanceBundleSnapshot = snapshot
            acceptanceBundleLoadError = ""
            let advisorySummary = snapshot.panelSummary(
                requireOperatorEvidence: acceptanceEvaluationMode.requireOperatorEvidence
            )
            let bundleStatusDetail =
                advisorySummary.bundle.value == snapshot.status
                ? advisorySummary.bundle.value
                : "\(advisorySummary.bundle.value) (meta.status=\(snapshot.status))"
            note = "Loaded acceptance bundle evidence: \(bundleStatusDetail)."
            appendAppEvent(
                severity: advisorySummary.bundle.tone == .positive ? .info : .review,
                title: "acceptance bundle loaded",
                detail: "\(trimmed) -> \(bundleStatusDetail)"
            )
        } catch {
            acceptanceBundleSnapshot = nil
            acceptanceBundleLoadError = error.localizedDescription
            note = acceptanceBundleLoadError
            appendAppEvent(
                severity: .review,
                title: "acceptance bundle rejected",
                detail: error.localizedDescription
            )
        }
    }

    func refreshCLIProvenance() {
        cliProvenance = CLIResolver.provenance()
        note = "\(cliProvenance.mode.title) CLI provenance: \(cliProvenance.readiness)."
    }

    func hasSuccessfulRun(_ kind: SuperMoverTaskKind) -> Bool {
        let signature = currentContextSignature(for: kind)
        return recentRuns.contains { run in
            guard run.kind == kind, run.contextSignature == signature else { return false }
            if case .finished(0) = run.state {
                return true
            }
            return false
        }
    }

    func hasLoadedPersistedDriftID(_ id: String?) -> Bool {
        let normalized = normalizedEvidenceID(id)
        guard !normalized.isEmpty else {
            return false
        }
        if verifySnapshot?.target_drifts?.contains(where: { $0.id == normalized }) == true {
            return true
        }
        if driftRecordSnapshot?.records?.contains(where: { $0.id == normalized }) == true {
            return true
        }
        return false
    }

    func runEvidenceReviewMetadataAction(_ action: EvidenceNextAction, task: SuperMoverTaskKind) {
        let currentAction = evidenceNextAction(for: action.kind)
        guard selectedRole.allows(task: task) else {
            note = "\(selectedRole.title) role cannot run \(task.rawValue) from the Evidence Vault."
            return
        }
        guard evidenceReviewMetadataArgumentsMatch(action, task: task) else {
            note = "Evidence action changed before execution. Refresh loaded evidence and confirm the command preview again."
            appendAppEvent(
                severity: .review,
                title: "evidence action refused",
                detail: "\(task.rawValue) preview no longer matches current app inputs."
            )
            return
        }
        guard currentAction.allowsExecution else {
            note = "Evidence action is missing required loaded evidence or operator intent."
            return
        }
        guard currentAction.commandPreview == action.commandPreview else {
            note = "Evidence action changed before execution. Refresh loaded evidence and confirm the command preview again."
            appendAppEvent(
                severity: .review,
                title: "evidence action refused",
                detail: "\(task.rawValue) loaded evidence or command preview changed before execution."
            )
            return
        }
        selectedTask = task
        runSelectedTask()
    }

    func evidenceReviewMetadataArgumentsMatch(_ action: EvidenceNextAction, task: SuperMoverTaskKind) -> Bool {
        guard action.allowsExecution,
              let preview = action.commandPreview,
              preview.profilePathSource == .appStoreSelectedProfile else {
            return false
        }
        let previewArguments = preview.arguments.map { argument in
            argument == "<AppStore.profilePath>" ? currentInput.profilePath : argument
        }
        return previewArguments == task.buildArguments(using: currentInput)
    }

    func hasLoadedDurableSyncQueueEntryID(_ id: String?) -> Bool {
        let normalized = normalizedEvidenceID(id)
        guard !normalized.isEmpty, let queue = syncQueueSnapshot else {
            return false
        }
        if queue.entry?.id == normalized {
            return true
        }
        if queue.entries?.contains(where: { $0.id == normalized }) == true {
            return true
        }
        return queue.enqueued?.contains(where: { $0.id == normalized }) == true
    }

    func hasLoadedPruneCandidateSoftDeleteID(_ id: String?) -> Bool {
        let normalized = normalizedEvidenceID(id)
        guard !normalized.isEmpty else {
            return false
        }
        return pruneReviewSnapshot?.prune_review.candidates?.contains(where: { $0.id == normalized }) == true
    }

    func hasExactlyOneLoadedPruneCandidateSoftDeleteID(_ ids: [String]) -> Bool {
        guard ids.count == 1 else {
            return false
        }
        return hasLoadedPruneCandidateSoftDeleteID(ids[0])
    }

    func hasLoadedPruneApprovalID(_ id: String?) -> Bool {
        let normalized = normalizedEvidenceID(id)
        guard !normalized.isEmpty else {
            return false
        }
        return pruneApprovalsSnapshot?.approvals?.contains(where: { $0.id == normalized }) == true
    }

    func evidenceNextActions(for kinds: [EvidenceNextAction.Kind]) -> [EvidenceNextAction] {
        kinds.map { evidenceNextAction(for: $0) }
    }

    func evidenceNextAction(for kind: EvidenceNextAction.Kind) -> EvidenceNextAction {
        NextActionPlanner().plan(kind, intent: evidenceNextActionIntent(for: kind))
    }

    func run(in slot: SupervisedProcessSlot) -> TaskRun? {
        activeRuns[slot]
    }

    func isRunning(_ slot: SupervisedProcessSlot) -> Bool {
        guard processControllers[slot] != nil else {
            return false
        }
        guard let run = activeRuns[slot] else { return false }
        return isCurrentContext(run)
    }

    func isProcessAlive(_ slot: SupervisedProcessSlot) -> Bool {
        processControllers[slot] != nil
    }

    func isStaleRunning(_ slot: SupervisedProcessSlot) -> Bool {
        guard processControllers[slot] != nil else {
            return false
        }
        guard let run = activeRuns[slot] else { return false }
        return !isCurrentContext(run)
    }

    func supervisionStateLabel(for slot: SupervisedProcessSlot) -> String {
        if isStaleRunning(slot) {
            return "stale running"
        }
        if isRunning(slot) {
            return "running"
        }
        guard let state = activeRuns[slot]?.state else {
            return "idle"
        }
        switch state {
        case .idle:
            return "idle"
        case .running:
            return "running"
        case .finished(let code):
            return "exit \(code)"
        case .failedToLaunch:
            return "failed"
        case .cancelled:
            return "cancelled"
        }
    }

    private func isReadableDirectory(_ path: String) -> Bool {
        directoryReadiness(path: path, requiresWrite: false) == "readable"
    }

    private func isWritableDirectory(_ path: String) -> Bool {
        directoryReadiness(path: path, requiresWrite: true) == "writable"
    }

    private func normalizedForSignature(_ value: String) -> String {
        value.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private func normalizedEvidenceID(_ value: String?) -> String {
        value?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    }

    private func acceptanceInstalledAppLaunchDependencies() -> AcceptanceInstalledAppLaunchCoordinator.Dependencies {
        AcceptanceInstalledAppLaunchCoordinator.Dependencies(
            currentContext: { [self] in
                AcceptanceInstalledAppLaunchCoordinator.Context(
                    selectedRole: selectedRole,
                    bundlePath: acceptanceBundlePath,
                    loadedSnapshot: acceptanceBundleSnapshot,
                    cliProvenance: cliProvenance
                )
            },
            currentBundleURL: { [self] in
                try currentAcceptanceBundleURL()
            },
            refreshBundle: { [self] in
                refreshAcceptanceBundle()
            },
            acceptanceBundleOperations: acceptanceBundleOperations
        )
    }

    private func evidenceNextActionIntent(for kind: EvidenceNextAction.Kind) -> EvidenceNextAction.OperatorIntent {
        let input = currentInput
        switch kind {
        case .driftRecord:
            return EvidenceNextAction.OperatorIntent(sessionID: input.requiredSessionID)
        case .driftAcknowledge, .driftResolve, .driftExpire, .reconcileApply:
            let driftID = input.requiredSingleDriftID
            return EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: driftID,
                selectedDurableEvidenceVerified: hasLoadedPersistedDriftID(driftID),
                reason: input.requiredReason,
                reviewer: input.requiredReviewer,
                sessionID: input.requiredSessionID
            )
        case .syncQueueCancel, .syncQueueFail:
            let entryID = input.requiredQueueEntryID
            return EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: entryID,
                selectedDurableEvidenceVerified: hasLoadedDurableSyncQueueEntryID(entryID),
                reason: input.requiredReason
            )
        case .pruneApprove:
            let softDeleteID = input.softDeleteIDs.count == 1 ? input.softDeleteIDs[0] : nil
            return EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: softDeleteID,
                selectedDurableEvidenceVerified: hasExactlyOneLoadedPruneCandidateSoftDeleteID(input.softDeleteIDs),
                approvalID: input.requiredApprovalID,
                reason: input.requiredReason,
                reviewer: input.requiredReviewer,
                expiresAt: input.expiresAt
            )
        case .pruneSupersede:
            let approvalID = input.requiredApprovalID
            return EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: approvalID,
                selectedDurableEvidenceVerified: hasLoadedPruneApprovalID(approvalID),
                reason: input.requiredReason,
                reviewer: input.requiredReviewer
            )
        default:
            return .empty
        }
    }

    private func resetSetupScopedEvidenceIfChanged(from oldValue: String, to newValue: String) {
        guard normalizedForSignature(oldValue) != normalizedForSignature(newValue) else {
            return
        }
        clearStructuredSnapshots()
        cleanupNetworkPushBaselineFile(for: .foregroundAction)
        evidenceArtifactCatalog = nil
        if processControllers.keys.contains(where: { isStaleRunning($0) }) {
            note = "Setup context changed. Existing foreground processes are stale for the current config/listen inputs; stop and restart their slots before treating them as current."
        }
    }

    private func clearSyncSnapshotsIfChanged(from oldValue: String, to newValue: String) {
        guard normalizedForSignature(oldValue) != normalizedForSignature(newValue) else {
            return
        }
        clearSyncSnapshots()
        if processControllers.keys.contains(where: { isStaleRunning($0) }) {
            note = "Sync inputs changed. Existing foreground sync processes are stale for the current inputs; stop and restart their slots before treating them as current."
        }
    }

    private func clearExplicitDiscoveryIfChanged(from oldValue: String, to newValue: String) {
        guard normalizedForSignature(oldValue) != normalizedForSignature(newValue) else {
            return
        }
        markEvidenceEnvelopesStale([.discoveryHints])
        discoveryHintsSnapshot = nil
    }

    private func clearBrowseDiscoveryIfChanged(from oldValue: String, to newValue: String) {
        guard normalizedForSignature(oldValue) != normalizedForSignature(newValue) else {
            return
        }
        markEvidenceEnvelopesStale([.discoveryBrowse])
        discoveryBrowseSnapshot = nil
    }

    private func clearAdvertiseDiscoveryIfChanged(from oldValue: String, to newValue: String) {
        guard normalizedForSignature(oldValue) != normalizedForSignature(newValue) else {
            return
        }
        markEvidenceEnvelopesStale([.discoveryAdvertise])
        discoveryAdvertiseSnapshot = nil
    }

    private func clearStructuredSnapshots() {
        markAllEvidenceEnvelopesStale()
        statusSnapshot = nil
        verifySnapshot = nil
        sourceConsistencySnapshot = nil
        healthSnapshot = nil
        reportSnapshot = nil
        driftListSnapshot = nil
        driftRecordSnapshot = nil
        discoveryHintsSnapshot = nil
        discoveryBrowseSnapshot = nil
        discoveryAdvertiseSnapshot = nil
        daemonStatusSnapshot = nil
        daemonLogsSnapshot = nil
        pruneReviewSnapshot = nil
        pruneApprovalsSnapshot = nil
        pruneApprovalAuthoringSnapshot = nil
        pruneApprovalSupersedeSnapshot = nil
        driftMutationSnapshot = nil
        reconcileSnapshot = nil
        syncQueueSnapshot = nil
        syncRunSnapshot = nil
        syncLoopSnapshot = nil
        syncWatchSnapshot = nil
        syncNetworkRunSnapshot = nil
        syncNetworkDiscoverRunSnapshot = nil
        syncNetworkLoopSnapshot = nil
        serveReadinessSnapshot = nil
        artifactReadProblems = []
    }

    private func clearStructuredSnapshots(for artifactKinds: [StructuredArtifactKind]) {
        markEvidenceEnvelopesStale(artifactKinds)
        for artifactKind in artifactKinds {
            switch artifactKind {
            case .status:
                statusSnapshot = nil
            case .verify:
                verifySnapshot = nil
            case .sourceConsistency:
                sourceConsistencySnapshot = nil
            case .health:
                healthSnapshot = nil
            case .report:
                reportSnapshot = nil
            case .driftList:
                driftListSnapshot = nil
            case .driftRecord:
                driftRecordSnapshot = nil
            case .discoveryHints:
                discoveryHintsSnapshot = nil
            case .discoveryBrowse:
                discoveryBrowseSnapshot = nil
            case .discoveryAdvertise:
                discoveryAdvertiseSnapshot = nil
            case .daemonStatus:
                daemonStatusSnapshot = nil
            case .daemonLogs:
                daemonLogsSnapshot = nil
            case .driftMutation:
                driftMutationSnapshot = nil
            case .pruneReview:
                pruneReviewSnapshot = nil
            case .pruneApprovals:
                pruneApprovalsSnapshot = nil
            case .pruneApprovalAuthoring:
                pruneApprovalAuthoringSnapshot = nil
            case .pruneApprovalSupersede:
                pruneApprovalSupersedeSnapshot = nil
            case .reconcile:
                reconcileSnapshot = nil
            case .syncQueue:
                syncQueueSnapshot = nil
            case .syncRun:
                syncRunSnapshot = nil
            case .syncLoop:
                syncLoopSnapshot = nil
            case .syncWatch:
                syncWatchSnapshot = nil
            case .syncNetworkRun:
                syncNetworkRunSnapshot = nil
            case .syncNetworkDiscoverRun:
                syncNetworkDiscoverRunSnapshot = nil
            case .syncNetworkLoop:
                syncNetworkLoopSnapshot = nil
            }
        }
        if artifactKinds.isEmpty {
            serveReadinessSnapshot = nil
        }
        artifactReadProblems.removeAll { problem in
            artifactKinds.contains(problem.artifactKind)
        }
    }

    func prepareStructuredEvidenceForLaunch(kind: SuperMoverTaskKind) {
        if kind.invalidatesStructuredEvidenceOnLaunch {
            evidenceArtifactCatalog = nil
            clearStructuredSnapshots()
            return
        }
        clearStructuredSnapshots(for: kind.structuredArtifactKinds)
    }

    private func clearSyncSnapshots() {
        markEvidenceEnvelopesStale([
            .syncQueue,
            .syncRun,
            .syncLoop,
            .syncWatch,
            .syncNetworkRun,
            .syncNetworkDiscoverRun,
            .syncNetworkLoop,
        ])
        syncQueueSnapshot = nil
        syncRunSnapshot = nil
        syncLoopSnapshot = nil
        syncWatchSnapshot = nil
        syncNetworkRunSnapshot = nil
        syncNetworkDiscoverRunSnapshot = nil
        syncNetworkLoopSnapshot = nil
        artifactReadProblems.removeAll { problem in
            switch problem.artifactKind {
            case .syncQueue,
                 .syncRun,
                 .syncLoop,
                 .syncWatch,
                 .syncNetworkRun,
                 .syncNetworkDiscoverRun,
                 .syncNetworkLoop:
                return true
            default:
                return false
            }
        }
    }

    func stopActiveTask() {
        stopProcess(in: focusedProcessSlot)
    }

    func stopProcess(in slot: SupervisedProcessSlot) {
        guard let controller = processControllers[slot] else {
            note = "\(slot.title) is not running."
            return
        }
        appendProcessEvent(slot: slot, kind: activeRuns[slot]?.kind ?? .status, message: "Operator requested foreground termination for \(slot.title).")
        note = "Stopping \(slot.title) by terminating its foreground process."
        controller.terminate()
    }

    func revealDashboardURL() {
        guard let run = activeRuns[.targetDashboard] else {
            note = "Target Dashboard has not emitted a URL yet."
            return
        }
        if let url = DashboardURLParser.firstURL(in: run.stderr) {
            NSWorkspace.shared.open(url)
        } else {
            note = "No dashboard URL found in Target Dashboard stderr yet."
        }
    }

    private func launch(kind: SuperMoverTaskKind, arguments: [String]) {
        let slot = kind.supervisedSlot
        guard processControllers[slot] == nil else {
            if isStaleRunning(slot) {
                note = "\(slot.title) is still running for a previous setup context. Stop that slot before starting \(kind.rawValue) for the current inputs."
            } else {
                note = "\(slot.title) is already running. Stop that slot before starting \(kind.rawValue)."
            }
            return
        }
        let start = Date()
        let contextSignature = currentContextSignature(for: kind)
        let argumentsWithServeReadyFile = argumentsForLaunch(kind: kind, baseArguments: arguments, contextSignature: contextSignature)
        var run = TaskRun(
            kind: kind,
            slot: slot,
            launchedAt: start,
            commandLine: argumentsWithServeReadyFile,
            contextSignature: contextSignature,
            processIdentifier: nil,
            stdout: "",
            stderr: "",
            state: .running
        )
        focusedProcessSlot = slot
        activeRuns[slot] = run
        prepareStructuredEvidenceForLaunch(kind: kind)

        do {
            let invocation = try CLIResolver.resolve(arguments: argumentsWithServeReadyFile)
            let controller = try ProcessController(invocation: invocation)
            let runID = run.id
            let controllerIdentity = ObjectIdentifier(controller)
            processControllers[slot] = controller
            controller.onStdout = { [weak self] chunk in
                Task { @MainActor in
                    guard let self else { return }
                    guard self.activeRuns[slot]?.id == runID else { return }
                    guard let state = self.activeRuns[slot]?.state else { return }
                    guard case .running = state else { return }
                    self.activeRuns[slot]?.stdout.append(chunk)
                }
            }
            controller.onStderr = { [weak self] chunk in
                Task { @MainActor in
                    guard let self else { return }
                    guard self.activeRuns[slot]?.id == runID else { return }
                    guard let state = self.activeRuns[slot]?.state else { return }
                    guard case .running = state else { return }
                    self.activeRuns[slot]?.stderr.append(chunk)
                    if kind == .dashboard, DashboardURLParser.firstURL(in: self.activeRuns[slot]?.stderr ?? "") != nil {
                        self.note = "Target Dashboard is supervised. Open the emitted loopback URL; this is not LAN browsing and not sync."
                    }
                    if kind == .serve {
                        self.refreshServeReadinessFromFileIfCurrent(slot: slot, kind: kind)
                    }
                    if kind == .serve, let summary = ServeInfoParser.firstSummary(in: self.activeRuns[slot]?.stderr ?? "") {
                        self.note = "\(slot.title): \(summary)"
                    }
                }
            }
            controller.onExit = { [weak self] code, terminated, output in
                Task { @MainActor in
                    guard let self else { return }
                    guard self.activeRuns[slot]?.id == runID else { return }
                    guard var finished = self.activeRuns[slot] else { return }
                    finished.stdout = output.stdout
                    finished.stderr = output.stderr
                    finished.state = terminated ? .cancelled : .finished(code)
                    self.recentRuns.insert(finished, at: 0)
                    self.activeRuns[slot] = finished
                    if let current = self.processControllers[slot], ObjectIdentifier(current) == controllerIdentity {
                        self.processControllers[slot] = nil
                    }
                    let stopReason = terminated ? "operator cancellation" : "exit \(code)"
                    self.appendProcessEvent(slot: slot, kind: kind, message: "\(slot.title) stopped with \(stopReason).")
                    self.handleStructuredCompletion(for: kind, finished: finished, exitCode: code, terminated: terminated)
                    if !terminated, code == 0 {
                        self.handleSuccessfulTextCompletion(for: kind, finished: finished)
                    }
                    if kind == .serve {
                        self.refreshServeReadinessFromFileIfCurrent(slot: slot, kind: kind)
                    }
                    self.cleanupCompletedLaunchArtifacts(
                        slot: slot,
                        kind: kind,
                        finishedSuccessfully: !terminated && code == 0
                    )
                    if kind.longRunning && !terminated {
                        self.note = "\(slot.title) exited. Review stderr and target-side evidence before assuming any durable state change."
                    }
                }
            }
            try controller.start()
            activeRuns[slot]?.processIdentifier = controller.processIdentifier
            appendProcessEvent(slot: slot, kind: kind, message: "Started \(kind.rawValue) in \(slot.title) with pid \(controller.processIdentifier).")
            if slot.isLongRunning {
                note = "\(slot.title) started as a supervised foreground process. Stop it from this app; durable truth still comes from CLI JSON and target evidence."
            }
            if kind == .serve {
                refreshServeReadinessFromFileIfCurrent(slot: slot, kind: kind)
            }
        } catch {
            cleanupCompletedLaunchArtifacts(
                slot: slot,
                kind: kind,
                finishedSuccessfully: false
            )
            run.state = .failedToLaunch(error.localizedDescription)
            activeRuns[slot] = run
            recentRuns.insert(run, at: 0)
            processControllers[slot] = nil
            appendProcessEvent(slot: slot, kind: kind, message: "Failed to launch \(kind.rawValue): \(error.localizedDescription)")
            note = "Failed to launch SuperMover CLI: \(error.localizedDescription)"
        }
    }

    private func argumentsForLaunch(kind: SuperMoverTaskKind, baseArguments: [String], contextSignature: String) -> [String] {
        if kind == .serve {
            cleanupServeReadyFile(for: .targetServe)
            let readyURL = FileManager.default.temporaryDirectory.appendingPathComponent("supermover-serve-ready-\(UUID().uuidString).json")
            serveReadyFilePaths[.targetServe] = readyURL.path
            serveReadyFileContextSignatures[.targetServe] = contextSignature
            serveReadinessSnapshot = nil
            return baseArguments + ["--ready-file", readyURL.path]
        }
        if kind == .networkPush {
            cleanupNetworkPushBaselineFile(for: .foregroundAction)
            let baselineURL = FileManager.default.temporaryDirectory.appendingPathComponent("supermover-network-baseline-\(UUID().uuidString).json")
            networkPushBaselineFilePaths[.foregroundAction] = baselineURL.path
            networkPushBaselineContextSignatures[.foregroundAction] = contextSignature
            return baseArguments + ["--source-baseline", baselineURL.path]
        }
        return baseArguments
    }

    private func serveReadyFileURL(for slot: SupervisedProcessSlot, kind: SuperMoverTaskKind) -> URL? {
        guard kind == .serve, slot == .targetServe, let path = serveReadyFilePaths[slot], !path.isEmpty else {
            return nil
        }
        return URL(fileURLWithPath: path)
    }

    private func networkPushBaselineFileURL(for slot: SupervisedProcessSlot, kind: SuperMoverTaskKind) -> URL? {
        guard kind == .networkPush, slot == .foregroundAction, let path = networkPushBaselineFilePaths[slot], !path.isEmpty else {
            return nil
        }
        return URL(fileURLWithPath: path)
    }

    func refreshServeReadinessFromFileIfCurrent(slot: SupervisedProcessSlot, kind: SuperMoverTaskKind) {
        guard kind == .serve, slot == .targetServe else {
            return
        }
        guard let run = activeRuns[slot], isCurrentContext(run) else {
            return
        }
        guard serveReadyFileContextSignatures[slot] == run.contextSignature else {
            return
        }
        guard let url = serveReadyFileURL(for: slot, kind: kind) else {
            serveReadinessSnapshot = nil
            return
        }
        guard let data = try? Data(contentsOf: url) else {
            serveReadinessSnapshot = nil
            return
        }
        guard let decoded = try? JSONDecoder().decode(ServeReadinessSnapshot.self, from: data) else {
            serveReadinessSnapshot = nil
            return
        }
        serveReadinessSnapshot = decoded
        note = "\(slot.title): \(decoded.summaryLine)"
    }

    func installServeReadyFileForTesting(_ path: String, contextSignature: String) {
        serveReadyFilePaths[.targetServe] = path
        serveReadyFileContextSignatures[.targetServe] = contextSignature
        activeRuns[.targetServe] = TaskRun(
            kind: .serve,
            slot: .targetServe,
            launchedAt: Date(),
            commandLine: [],
            contextSignature: contextSignature,
            processIdentifier: 1,
            stdout: "",
            stderr: "",
            state: .running
        )
    }

    func installNetworkPushBaselineFileForTesting(_ path: String, contextSignature: String) {
        networkPushBaselineFilePaths[.foregroundAction] = path
        networkPushBaselineContextSignatures[.foregroundAction] = contextSignature
    }

    func recordSuccessfulCompletionForTesting(_ finished: TaskRun) {
        recentRuns.insert(finished, at: 0)
        handleSuccessfulTextCompletion(for: finished.kind, finished: finished)
    }

    private func appendProcessEvent(slot: SupervisedProcessSlot, kind: SuperMoverTaskKind, message: String) {
        processEvents.insert(ProcessLifecycleEvent(occurredAt: Date(), slot: slot, kind: kind, message: message), at: 0)
        if processEvents.count > 40 {
            processEvents.removeLast(processEvents.count - 40)
        }
        appendAppEvent(severity: .info, title: "process event", detail: "\(slot.title): \(message)")
    }

    private func appendAppEvent(severity: AppEventSeverity, title: String, detail: String) {
        appEvents.insert(AppEvent(occurredAt: Date(), severity: severity, title: title, detail: detail), at: 0)
        if appEvents.count > 80 {
            appEvents.removeLast(appEvents.count - 80)
        }
    }

    private func handleSuccessfulTextCompletion(for kind: SuperMoverTaskKind, finished: TaskRun) {
        guard isCurrentContext(finished) else {
            appendAppEvent(
                severity: .review,
                title: "stale command completion",
                detail: "\(kind.rawValue) finished for previous inputs. Output is retained in Recent Runs but was not promoted as current guidance."
            )
            if kind == .pair {
                note = "Pair finished for previous pairing inputs. Review the retained run output, then rerun Status or Report for durable evidence before trusting current setup."
            }
            return
        }
        if kind == .profileInit {
            note = "Migration config created through CLI. Run Lint Config before treating setup as ready."
        } else if kind == .profileSetTarget {
            note = "Migration config target updated through CLI. Run Lint Config and Status to load evidence."
        } else if kind == .profileSetNetwork {
            note = "Migration config network material updated through CLI. Run Lint Config and Status before treating receiver readiness as current."
        } else if kind == .profileAdoptPairing {
            note = "Target pairing receipt imported through CLI. Restart target serve and run Status or Report before trusting paired receiver readiness."
        } else if kind == .pair {
            let summary = PairInfoParser.firstSummary(in: finished.stdout) ?? "Pair command succeeded"
            note = "\(summary). Run Status or Report to confirm config pin and receipt evidence."
        }
        if kind == .networkPush {
            captureSourceConsistencyProofIfAvailableAndFinalizeNetworkPush(for: finished)
            return
        }
        autoRecordAcceptanceArtifactIfConfigured(for: kind)
    }

    func captureStructuredResult(for kind: SuperMoverTaskKind, stdout: String, stderr: String, exitCode: Int32, contextSignature: String) {
        guard contextSignature == currentContextSignature(for: kind) else {
            for artifactKind in kind.structuredArtifactKinds {
                storeStaleEvidenceEnvelope(
                    artifactKind: artifactKind,
                    task: kind,
                    stdout: stdout,
                    stderr: stderr,
                    exitCode: exitCode,
                    contextSignature: contextSignature
                )
            }
            note = "Task finished for a previous setup context. Output is retained, but structured evidence was not promoted; rerun against the current profile and roots."
            appendAppEvent(severity: .review, title: "stale structured output", detail: "\(kind.rawValue) finished for a previous setup context.")
            return
        }
        let data = Data(stdout.utf8)
        func decodeAndStore<T: Decodable>(_ artifactKind: StructuredArtifactKind, as type: T.Type) -> T? {
            storeEvidenceEnvelope(
                artifactKind: artifactKind,
                task: kind,
                stdout: stdout,
                stderr: stderr,
                exitCode: exitCode,
                contextSignature: contextSignature
            )
            return decodeStructuredArtifact(artifactKind, task: kind, data: data)
        }
        switch kind {
        case .status:
            if let decoded = decodeAndStore(.status, as: StatusSnapshot.self) {
                statusSnapshot = decoded
            }
        case .verify:
            if let decoded = decodeAndStore(.verify, as: VerifySnapshot.self) {
                verifySnapshot = decoded
            }
        case .health:
            if let decoded = decodeAndStore(.health, as: HealthSnapshot.self) {
                healthSnapshot = decoded
            }
        case .report:
            if let decoded = decodeAndStore(.report, as: ReportSnapshot.self) {
                reportSnapshot = decoded
            }
        case .driftList:
            if let decoded = decodeAndStore(.driftList, as: DriftListSnapshot.self) {
                driftListSnapshot = decoded
            }
        case .driftRecord:
            if let decoded = decodeAndStore(.driftRecord, as: DriftRecordSnapshot.self) {
                driftRecordSnapshot = decoded
            }
        case .discoverAddress:
            if let decoded = decodeAndStore(.discoveryHints, as: [DiscoveryAddressHintSnapshot].self) {
                discoveryHintsSnapshot = decoded
            }
        case .discoverBrowse:
            if let decoded = decodeAndStore(.discoveryBrowse, as: DiscoveryBrowseSnapshot.self) {
                discoveryBrowseSnapshot = decoded
            }
        case .discoverAdvertise:
            if let decoded = decodeAndStore(.discoveryAdvertise, as: DiscoveryAdvertiseSnapshot.self) {
                discoveryAdvertiseSnapshot = decoded
            }
        case .daemonStatus:
            if let decoded = decodeAndStore(.daemonStatus, as: DaemonStatusSnapshot.self) {
                daemonStatusSnapshot = decoded
            }
        case .daemonLogs:
            if let decoded = decodeAndStore(.daemonLogs, as: DaemonLogsSnapshot.self) {
                daemonLogsSnapshot = decoded
            }
        case .driftAcknowledge, .driftResolve, .driftExpire:
            if let decoded = decodeAndStore(.driftMutation, as: DriftMutationSnapshot.self) {
                driftMutationSnapshot = decoded
            }
        case .pruneReview:
            if let decoded = decodeAndStore(.pruneReview, as: PruneReviewSnapshot.self) {
                pruneReviewSnapshot = decoded
            }
        case .pruneApprovals:
            if let decoded = decodeAndStore(.pruneApprovals, as: PruneApprovalsSnapshot.self) {
                pruneApprovalsSnapshot = decoded
            }
        case .pruneApprove:
            if let decoded = decodeAndStore(.pruneApprovalAuthoring, as: PruneApprovalAuthoringSnapshot.self) {
                pruneApprovalAuthoringSnapshot = decoded
            }
        case .pruneSupersede:
            if let decoded = decodeAndStore(.pruneApprovalSupersede, as: PruneApprovalSupersedeSnapshot.self) {
                pruneApprovalSupersedeSnapshot = decoded
            }
        case .reconcilePlan, .reconcileApply:
            if let decoded = decodeAndStore(.reconcile, as: ReconcileSnapshot.self) {
                reconcileSnapshot = decoded
            }
        case .syncQueueEnqueue,
             .syncQueueStatus,
             .syncQueueList,
             .syncQueueReady,
             .syncQueueCancel,
             .syncQueueFail:
            if let decoded = decodeAndStore(.syncQueue, as: SyncQueueSnapshot.self) {
                syncQueueSnapshot = decoded
            }
        case .syncRun:
            if let decoded = decodeAndStore(.syncRun, as: SyncRunSnapshot.self) {
                syncRunSnapshot = decoded
            }
        case .syncLoop:
            if let decoded = decodeAndStore(.syncLoop, as: SyncLoopSnapshot.self) {
                syncLoopSnapshot = decoded
            }
        case .syncWatch:
            if let decoded = decodeAndStore(.syncWatch, as: SyncWatchSnapshot.self) {
                syncWatchSnapshot = decoded
            }
        case .syncNetworkRun:
            if let decoded = decodeAndStore(.syncNetworkRun, as: SyncNetworkRunSnapshot.self) {
                syncNetworkRunSnapshot = decoded
            }
        case .syncNetworkDiscoverRun:
            if let decoded = decodeAndStore(.syncNetworkDiscoverRun, as: SyncNetworkDiscoverRunSnapshot.self) {
                syncNetworkDiscoverRunSnapshot = decoded
            }
        case .syncNetworkLoop:
            if let decoded = decodeAndStore(.syncNetworkLoop, as: SyncNetworkLoopSnapshot.self) {
                syncNetworkLoopSnapshot = decoded
            }
        default:
            break
        }
    }

    private func storeEvidenceEnvelope(artifactKind: StructuredArtifactKind, task: SuperMoverTaskKind, stdout: String, stderr: String, exitCode: Int32, contextSignature: String) {
        let envelope = StructuredEvidenceEnvelope(
            artifactKind: artifactKind,
            task: task,
            loadedAt: Date(),
            contextSignature: contextSignature,
            exitCode: exitCode,
            rawStdout: stdout,
            stderrSample: String(stderr.trimmingCharacters(in: .whitespacesAndNewlines).prefix(2000)),
            freshness: .current
        )
        appendEvidenceEnvelope(envelope, promoteCurrent: true)
    }

    private func storeStaleEvidenceEnvelope(artifactKind: StructuredArtifactKind, task: SuperMoverTaskKind, stdout: String, stderr: String, exitCode: Int32, contextSignature: String) {
        let envelope = StructuredEvidenceEnvelope(
            artifactKind: artifactKind,
            task: task,
            loadedAt: Date(),
            contextSignature: contextSignature,
            exitCode: exitCode,
            rawStdout: stdout,
            stderrSample: String(stderr.trimmingCharacters(in: .whitespacesAndNewlines).prefix(2000)),
            freshness: .stale
        )
        appendEvidenceEnvelope(envelope, promoteCurrent: false)
    }

    private func appendEvidenceEnvelope(_ envelope: StructuredEvidenceEnvelope, promoteCurrent: Bool) {
        if promoteCurrent {
            evidenceEnvelopes[envelope.artifactKind] = envelope
        }
        evidenceEnvelopeHistory.insert(envelope, at: 0)
        if evidenceEnvelopeHistory.count > 32 {
            evidenceEnvelopeHistory.removeLast(evidenceEnvelopeHistory.count - 32)
        }
    }

    private func markAllEvidenceEnvelopesStale() {
        markEvidenceEnvelopesStale(Array(evidenceEnvelopes.keys))
    }

    private func markEvidenceEnvelopesStale(_ artifactKinds: [StructuredArtifactKind]) {
        for artifactKind in artifactKinds {
            evidenceEnvelopes[artifactKind]?.freshness = .stale
        }
        for index in evidenceEnvelopeHistory.indices where artifactKinds.contains(evidenceEnvelopeHistory[index].artifactKind) {
            evidenceEnvelopeHistory[index].freshness = .stale
        }
    }

    private func decodeStructuredArtifact<T: Decodable>(_ artifactKind: StructuredArtifactKind, task: SuperMoverTaskKind, data: Data) -> T? {
        guard !data.isEmpty else {
            recordArtifactReadProblem(artifactKind, task: task, problem: "missing stdout JSON", raw: "")
            return nil
        }
        do {
            let decoded = try JSONDecoder().decode(T.self, from: data)
            appendAppEvent(severity: .info, title: "artifact decoded", detail: "\(artifactKind.title) decoded from \(task.rawValue).")
            return decoded
        } catch {
            let raw = String(data: data, encoding: .utf8) ?? "<non-utf8 stdout>"
            recordArtifactReadProblem(artifactKind, task: task, problem: error.localizedDescription, raw: raw)
            return nil
        }
    }

    private func recordArtifactReadProblem(_ artifactKind: StructuredArtifactKind, task: SuperMoverTaskKind, problem: String, raw: String) {
        let sample = String(raw.prefix(2000))
        artifactReadProblems.insert(
            ArtifactReadProblem(
                occurredAt: Date(),
                artifactKind: artifactKind,
                task: task,
                problem: problem,
                rawSample: sample
            ),
            at: 0
        )
        if artifactReadProblems.count > 40 {
            artifactReadProblems.removeLast(artifactReadProblems.count - 40)
        }
        appendAppEvent(severity: .error, title: "artifact read problem", detail: "\(artifactKind.title): \(problem)")
    }

    private func handleStructuredCompletion(for kind: SuperMoverTaskKind, finished: TaskRun, exitCode: Int32, terminated: Bool) {
        guard kind.hasStructuredSnapshot else { return }
        if terminated {
            recordStructuredOutputSkip(kind, exitCode: exitCode, stdout: finished.stdout, stderr: finished.stderr, reason: "operator cancellation")
            return
        }
        let hasStdout = !finished.stdout.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        if exitCode != 0, !hasStdout {
            recordStructuredOutputSkip(kind, exitCode: exitCode, stdout: finished.stdout, stderr: finished.stderr, reason: "no structured stdout")
            return
        }
        if exitCode != 0, !kind.allowsReviewExitStructuredSnapshot {
            recordStructuredOutputSkip(kind, exitCode: exitCode, stdout: finished.stdout, stderr: finished.stderr, reason: "non-successful mutating command")
            return
        }
        if exitCode != 0 {
            appendAppEvent(severity: .review, title: "structured output from review exit", detail: "\(kind.rawValue) exited \(exitCode); JSON is treated as review evidence, not success.")
        }
        captureStructuredResult(for: kind, stdout: finished.stdout, stderr: finished.stderr, exitCode: exitCode, contextSignature: finished.contextSignature)
    }

    private func recordStructuredOutputSkip(_ kind: SuperMoverTaskKind, exitCode: Int32, stdout: String, stderr: String, reason: String) {
        let stderrSample = stderr.trimmingCharacters(in: .whitespacesAndNewlines)
        let stdoutSample = stdout.trimmingCharacters(in: .whitespacesAndNewlines)
        let sample = stderrSample.isEmpty ? stdoutSample : stderrSample
        let sampleSuffix = sample.isEmpty ? "" : " sample: \(String(sample.prefix(500)))"
        let severity: AppEventSeverity = reason == "operator cancellation" ? .review : .error
        appendAppEvent(severity: severity, title: "structured output skipped", detail: "\(kind.rawValue) exited \(exitCode); \(reason).\(sampleSuffix)")
    }
}

private extension SuperMoverTaskKind {
    var structuredArtifactKinds: [StructuredArtifactKind] {
        switch self {
        case .status:
            return [.status]
        case .verify:
            return [.verify]
        case .health:
            return [.health]
        case .report:
            return [.report]
        case .driftList:
            return [.driftList]
        case .driftRecord:
            return [.driftRecord]
        case .discoverAddress:
            return [.discoveryHints]
        case .discoverBrowse:
            return [.discoveryBrowse]
        case .discoverAdvertise:
            return [.discoveryAdvertise]
        case .daemonStatus:
            return [.daemonStatus]
        case .daemonLogs:
            return [.daemonLogs]
        case .driftAcknowledge, .driftResolve, .driftExpire:
            return [.driftMutation]
        case .pruneReview:
            return [.pruneReview]
        case .pruneApprovals:
            return [.pruneApprovals]
        case .pruneApprove:
            return [.pruneApprovalAuthoring]
        case .pruneSupersede:
            return [.pruneApprovalSupersede]
        case .reconcilePlan, .reconcileApply:
            return [.reconcile]
        case .syncQueueEnqueue,
             .syncQueueStatus,
             .syncQueueList,
             .syncQueueReady,
             .syncQueueCancel,
             .syncQueueFail:
            return [.syncQueue]
        case .syncRun:
            return [.syncRun]
        case .syncLoop:
            return [.syncLoop]
        case .syncWatch:
            return [.syncWatch]
        case .syncNetworkRun:
            return [.syncNetworkRun]
        case .syncNetworkDiscoverRun:
            return [.syncNetworkDiscoverRun]
        case .syncNetworkLoop:
            return [.syncNetworkLoop]
        case .profileInit,
             .version,
             .lintProfile,
             .profileSetTarget,
             .profileSetNetwork,
             .profileAdoptPairing,
             .dryRun,
             .publish,
             .recoverDryRun,
             .pair,
             .networkDryRun,
             .networkPush,
             .serve,
             .daemonInstall,
             .daemonRun,
             .daemonRestart,
             .daemonStop,
             .dashboard:
            return []
        }
    }

    var hasStructuredSnapshot: Bool {
        !structuredArtifactKinds.isEmpty
    }

    var allowsReviewExitStructuredSnapshot: Bool {
        switch self {
        case .status,
             .verify,
             .health,
             .report,
             .driftList,
             .driftRecord,
             .daemonStatus,
             .daemonLogs,
             .pruneReview,
             .pruneApprovals,
             .reconcilePlan,
             .syncQueueEnqueue,
             .syncQueueStatus,
             .syncQueueList,
             .syncQueueReady,
             .syncQueueCancel,
             .syncQueueFail,
             .syncRun,
             .syncLoop,
             .syncWatch,
             .syncNetworkRun,
             .syncNetworkDiscoverRun,
             .syncNetworkLoop:
            return true
        default:
            return false
        }
    }

    var requiresSourceRoot: Bool {
        switch self {
        case .profileInit:
            return true
        default:
            return false
        }
    }

    var requiresTargetRootInput: Bool {
        switch self {
        case .profileInit, .profileSetTarget:
            return true
        default:
            return false
        }
    }

    var requiresWritableTargetRoot: Bool {
        switch self {
        case .profileSetTarget:
            return true
        default:
            return false
        }
    }

    var requiresSessionID: Bool {
        switch self {
        case .publish, .networkPush, .syncRun, .syncNetworkRun, .syncNetworkDiscoverRun:
            return true
        default:
            return false
        }
    }

    var requiresSessionPrefix: Bool {
        switch self {
        case .syncLoop, .syncWatch, .syncNetworkLoop:
            return true
        default:
            return false
        }
    }

    var requiresQueueEntryID: Bool {
        switch self {
        case .syncQueueCancel, .syncQueueFail:
            return true
        default:
            return false
        }
    }

    var requiresPairingTargetAddress: Bool {
        switch self {
        case .discoverAddress, .pair:
            return true
        default:
            return false
        }
    }

    var requiresPairingVerificationCode: Bool {
        switch self {
        case .pair:
            return true
        default:
            return false
        }
    }

    var requiresSingleDriftID: Bool {
        switch self {
        case .driftAcknowledge, .driftResolve, .driftExpire:
            return true
        default:
            return false
        }
    }

    var requiresAnyDriftIDs: Bool {
        switch self {
        case .reconcileApply:
            return true
        default:
            return false
        }
    }

    var requiresApprovalID: Bool {
        switch self {
        case .pruneApprove, .pruneSupersede:
            return true
        default:
            return false
        }
    }

    var requiresSoftDeleteIDs: Bool {
        switch self {
        case .pruneApprove:
            return true
        default:
            return false
        }
    }

    var requiresReason: Bool {
        switch self {
        case .driftAcknowledge, .driftResolve, .driftExpire, .reconcileApply,
             .pruneApprove, .pruneSupersede, .syncQueueCancel, .syncQueueFail,
             .daemonRestart, .daemonStop:
            return true
        default:
            return false
        }
    }

    var requiresReviewer: Bool {
        switch self {
        case .pruneApprove, .pruneSupersede:
            return true
        default:
            return false
        }
    }

    var requiresImportReceiptFile: Bool {
        switch self {
        case .profileAdoptPairing:
            return true
        default:
            return false
        }
    }

    func contextInputs(using input: TaskInput) -> [String] {
        switch self {
        case .profileAdoptPairing:
            return input.pairingReceipt.contextInputs
        case .profileSetNetwork:
            return input.profileNetwork.contextInputs
        case .discoverAddress:
            return [input.requiredPairingTargetAddress, input.requiredDiscoveryBrowseTimeout]
        case .discoverBrowse:
            return [input.requiredDiscoveryBrowseListen, input.requiredDiscoveryBrowseTimeout]
        case .discoverAdvertise:
            return [
                input.discoveryAdvertiseListen,
                input.requiredDiscoveryAdvertiseDestination,
                input.requiredDiscoveryAdvertiseDuration,
                input.requiredDiscoveryAdvertiseInterval,
            ]
        case .pair:
            return [
                input.requiredPairingTargetAddress,
                input.requiredPairingVerificationCode,
                input.requiredPairingMethod,
                input.requiredPairingTimeout,
            ]
        case .driftAcknowledge, .driftResolve, .driftExpire:
            return [input.requiredSingleDriftID, input.requiredReason, input.requiredReviewer]
        case .pruneApprove:
            return [
                input.requiredApprovalID,
                input.softDeleteIDs.joined(separator: "\u{1E}"),
                input.requiredReason,
                input.requiredReviewer,
                input.expiresAt.trimmingCharacters(in: .whitespacesAndNewlines),
            ]
        case .pruneSupersede:
            return [input.requiredApprovalID, input.requiredReason, input.requiredReviewer]
        case .syncQueueCancel, .syncQueueFail:
            return [input.requiredQueueEntryID, input.requiredReason]
        case .daemonRun:
            return [input.listenAddress]
        case .daemonRestart, .daemonStop:
            return [input.requiredReason]
        case .syncRun, .syncNetworkRun:
            return [input.requiredSessionID, input.requiredSyncRetryBackoff]
        case .syncNetworkDiscoverRun:
            return [
                input.requiredSessionID,
                input.requiredSyncDiscoveryListen,
                input.requiredSyncDiscoveryTimeout,
                input.requiredSyncRetryBackoff,
            ]
        case .syncLoop, .syncNetworkLoop:
            return [
                input.requiredSessionPrefix,
                input.requiredSyncInterval,
                input.requiredSyncMaxRuns,
                input.requiredSyncRetryBackoff,
            ]
        case .syncWatch:
            return [
                input.requiredSessionPrefix,
                input.requiredSyncSettle,
                input.requiredSyncMaxEvents,
                input.requiredSyncRetryBackoff,
            ]
        default:
            return []
        }
    }
}

struct ProcessOutputSnapshot {
    let stdout: String
    let stderr: String
}

final class ProcessOutputAccumulator: @unchecked Sendable {
    private let lock = NSLock()
    private var stdout = ""
    private var stderr = ""

    func appendStdout(_ text: String) {
        lock.lock()
        stdout.append(text)
        lock.unlock()
    }

    func appendStderr(_ text: String) {
        lock.lock()
        stderr.append(text)
        lock.unlock()
    }

    func snapshot() -> ProcessOutputSnapshot {
        lock.lock()
        let snapshot = ProcessOutputSnapshot(stdout: stdout, stderr: stderr)
        lock.unlock()
        return snapshot
    }
}

enum CLIResolver {
    private typealias BundleProvenanceManifest = SuperMoverBundledProvenanceManifest

    private enum BundleProvenanceLoad {
        case loaded(BundleProvenanceManifest)
        case missing(String)
        case malformed(String, String)
    }

    static func resolve(arguments: [String]) throws -> CLIInvocation {
        try resolve(
            arguments: arguments,
            resourceURL: Bundle.main.resourceURL,
            isPackagedApp: isPackagedAppBundle(),
            repoRoot: findRepoRoot()
        )
    }

    static func resolve(
        arguments: [String],
        resourceURL: URL?,
        isPackagedApp: Bool,
        repoRoot: URL?
    ) throws -> CLIInvocation {
        if isPackagedApp {
            guard let bundled = bundledBinaryCandidateURL(resourceURL: resourceURL),
                  FileManager.default.isExecutableFile(atPath: bundled.path) else {
                throw SuperMoverCLIError.bundledBinaryMissing(bundledBinaryCandidateURL(resourceURL: resourceURL)?.path ?? "Contents/Resources/bin/supermover")
            }
            return CLIInvocation(
                executableURL: bundled,
                arguments: arguments,
                workingDirectoryURL: bundled.deletingLastPathComponent()
            )
        }

        if let bundled = bundledBinaryURL(resourceURL: resourceURL) {
            guard FileManager.default.isExecutableFile(atPath: bundled.path) else {
                throw SuperMoverCLIError.bundledBinaryMissing(bundled.path)
            }
            return CLIInvocation(
                executableURL: bundled,
                arguments: arguments,
                workingDirectoryURL: bundled.deletingLastPathComponent()
            )
        }

        guard let repoRoot else {
            throw SuperMoverCLIError.repoRootNotFound
        }
        return try developmentInvocation(repoRoot: repoRoot, arguments: arguments)
    }

    static func provenance() -> CLIProvenance {
        let bundleIdentifier = Bundle.main.bundleIdentifier ?? "unbundled"
        let appVersion = [
            Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String,
            Bundle.main.object(forInfoDictionaryKey: "CFBundleVersion") as? String,
        ]
            .compactMap { value in
                guard let value, !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
                    return nil
                }
                return value
            }
            .joined(separator: " build ")
        return provenance(
            resourceURL: Bundle.main.resourceURL,
            bundleIdentifier: bundleIdentifier,
            appVersion: appVersion.isEmpty ? "unknown" : appVersion,
            isPackagedApp: isPackagedAppBundle(),
            repoRoot: findRepoRoot()
        )
    }

    static func provenance(
        resourceURL: URL?,
        bundleIdentifier: String,
        appVersion: String,
        isPackagedApp: Bool,
        repoRoot: URL?
    ) -> CLIProvenance {
        let manifestLoad = bundledProvenanceManifest(resourceURL: resourceURL)
        let manifest: BundleProvenanceManifest?
        let provenanceStatus: String
        switch manifestLoad {
        case .loaded(let loaded):
            manifest = loaded
            provenanceStatus = "loaded"
        case .missing(let path):
            manifest = nil
            provenanceStatus = "missing at \(path)"
        case .malformed(let path, let error):
            manifest = nil
                provenanceStatus = "malformed at \(path): \(error)"
        }
        let provenancePath = bundledProvenanceURL(resourceURL: resourceURL)?.path

        if let bundled = bundledBinaryCandidateURL(resourceURL: resourceURL), FileManager.default.fileExists(atPath: bundled.path) {
            let executable = FileManager.default.isExecutableFile(atPath: bundled.path)
            let manifestCompleteness = bundledManifestCompleteness(manifest)
            let readinessLevel: CLIProvenance.ReadinessLevel
            let readiness: String
            let detail: String
            if !executable {
                readinessLevel = .blocked
                readiness = "bundled binary not executable"
                detail = "Packaged app has a bundled binary path, but it is not executable."
            } else if provenanceStatus != "loaded" {
                readinessLevel = .blocked
                readiness = "provenance unavailable"
                detail = "Packaged app has an executable CLI, but the provenance manifest is \(provenanceStatus)."
            } else if let manifestCompleteness {
                readinessLevel = .blocked
                readiness = "provenance incomplete"
                detail = manifestCompleteness
            } else if requiresLocalBundleReview(manifest) {
                readinessLevel = .review
                readiness = "local bundle review"
                detail = "Packaged app has an executable CLI and manifest, but signing is \(manifest?.signing ?? "unknown") and git_dirty is \(manifest?.git_dirty.map(String.init) ?? "unknown"). Treat as local evidence until release signing/notarization is complete."
            } else {
                readinessLevel = .pass
                readiness = "ready"
                detail = "Packaged app will run Contents/Resources/bin/supermover with complete signed provenance. Notarization remains external release evidence."
            }
            return CLIProvenance(
                mode: .bundled,
                readinessLevel: readinessLevel,
                executablePath: bundled.path,
                workingDirectoryPath: bundled.deletingLastPathComponent().path,
                bundleIdentifier: bundleIdentifier,
                appVersion: appVersion,
                provenancePath: provenancePath,
                provenanceStatus: provenanceStatus,
                bundleCommit: manifest?.git_commit,
                bundledCLIVersion: manifest?.cli_version,
                buildProfile: manifest?.build_profile,
                signing: manifest?.signing,
                gitDirty: manifest?.git_dirty,
                builtAt: manifest?.built_at,
                readiness: readiness,
                detail: detail
            )
        }

        if isPackagedApp {
            return CLIProvenance(
                mode: .unavailable,
                readinessLevel: .blocked,
                executablePath: bundledBinaryCandidateURL(resourceURL: resourceURL)?.path ?? "Contents/Resources/bin/supermover",
                workingDirectoryPath: resourceURL?.path ?? FileManager.default.currentDirectoryPath,
                bundleIdentifier: bundleIdentifier,
                appVersion: appVersion,
                provenancePath: provenancePath,
                provenanceStatus: provenanceStatus,
                bundleCommit: manifest?.git_commit,
                bundledCLIVersion: manifest?.cli_version,
                buildProfile: manifest?.build_profile,
                signing: manifest?.signing,
                gitDirty: manifest?.git_dirty,
                builtAt: manifest?.built_at,
                readiness: "missing bundled CLI",
                detail: "Packaged app is missing Contents/Resources/bin/supermover and will not fall back to the development launcher."
            )
        }

        if let repoRoot {
            return CLIProvenance(
                mode: .development,
                readinessLevel: .review,
                executablePath: repoRoot.appendingPathComponent(".tmp/macos-app/supermover-dev").path,
                workingDirectoryPath: repoRoot.path,
                bundleIdentifier: bundleIdentifier,
                appVersion: appVersion.isEmpty ? "development" : appVersion,
                provenancePath: provenancePath,
                provenanceStatus: provenanceStatus,
                bundleCommit: manifest?.git_commit,
                bundledCLIVersion: manifest?.cli_version,
                buildProfile: manifest?.build_profile ?? "development build-and-exec",
                signing: manifest?.signing,
                gitDirty: manifest?.git_dirty,
                builtAt: manifest?.built_at,
                readiness: "development launcher",
                detail: "No packaged CLI is bundled; the app builds cmd/supermover into .tmp/macos-app before launching commands."
            )
        }

        return CLIProvenance(
            mode: .unavailable,
            readinessLevel: .blocked,
            executablePath: bundledBinaryCandidateURL(resourceURL: resourceURL)?.path ?? "missing",
            workingDirectoryPath: resourceURL?.path ?? FileManager.default.currentDirectoryPath,
            bundleIdentifier: bundleIdentifier,
            appVersion: appVersion,
            provenancePath: provenancePath,
            provenanceStatus: provenanceStatus,
            bundleCommit: manifest?.git_commit,
            bundledCLIVersion: manifest?.cli_version,
            buildProfile: manifest?.build_profile,
            signing: manifest?.signing,
            gitDirty: manifest?.git_dirty,
            builtAt: manifest?.built_at,
            readiness: "missing CLI",
            detail: "Could not find an executable bundled CLI or the development repository root."
        )
    }

    private static func bundledBinaryURL(resourceURL: URL?) -> URL? {
        let bundled = bundledBinaryCandidateURL(resourceURL: resourceURL)
        if let bundled, FileManager.default.fileExists(atPath: bundled.path) {
            return bundled
        }
        return nil
    }

    private static func bundledBinaryCandidateURL(resourceURL: URL?) -> URL? {
        resourceURL?.appendingPathComponent("bin/supermover")
    }

    private static func bundledProvenanceURL(resourceURL: URL?) -> URL? {
        resourceURL?.appendingPathComponent("supermover-provenance.json")
    }

    private static func bundledProvenanceManifest(resourceURL: URL?) -> BundleProvenanceLoad {
        guard let url = bundledProvenanceURL(resourceURL: resourceURL) else {
            return .missing("Bundle.main.resourceURL/supermover-provenance.json")
        }
        guard FileManager.default.fileExists(atPath: url.path) else {
            return .missing(url.path)
        }
        do {
            let data = try Data(contentsOf: url)
            return .loaded(try JSONDecoder().decode(BundleProvenanceManifest.self, from: data))
        } catch {
            return .malformed(url.path, String(describing: error))
        }
    }

    private static func bundledManifestCompleteness(_ manifest: BundleProvenanceManifest?) -> String? {
        guard let manifest else {
            return "Packaged app is missing a readable provenance manifest."
        }
        var missing: [String] = []
        if manifest.schema != BundleProvenanceManifest.schemaID {
            missing.append("schema")
        }
        if manifest.app_bundle_id?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty != false {
            missing.append("app_bundle_id")
        }
        if manifest.app_version?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty != false {
            missing.append("app_version")
        }
        if manifest.git_commit?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty != false {
            missing.append("git_commit")
        }
        if manifest.cli_version?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty != false {
            missing.append("cli_version")
        }
        if manifest.cli_relative_path != BundleProvenanceManifest.bundledCLIRelativePath {
            missing.append("cli_relative_path")
        }
        if manifest.build_profile?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty != false {
            missing.append("build_profile")
        }
        if manifest.signing?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty != false {
            missing.append("signing")
        }
        if manifest.git_dirty == nil {
            missing.append("git_dirty")
        }
        if manifest.built_at?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty != false {
            missing.append("built_at")
        }
        if missing.isEmpty {
            return nil
        }
        return "Packaged app provenance is missing or invalid: \(missing.joined(separator: ", "))."
    }

    private static func requiresLocalBundleReview(_ manifest: BundleProvenanceManifest?) -> Bool {
        guard let manifest else {
            return true
        }
        let signing = manifest.signing?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return signing.isEmpty || signing == "unsigned" || signing == "-" || manifest.git_dirty == true
    }

    private static func isPackagedAppBundle() -> Bool {
        Bundle.main.bundleURL.pathExtension == "app"
    }

    private static func findRepoRoot() -> URL? {
        var candidates: [URL] = []
        candidates.append(URL(filePath: FileManager.default.currentDirectoryPath))
        if let executable = Bundle.main.executableURL {
            candidates.append(executable.deletingLastPathComponent())
        }
        for candidate in candidates {
            if let root = walkUpToRepoRoot(startingAt: candidate) {
                return root
            }
        }
        return nil
    }

    private static func walkUpToRepoRoot(startingAt url: URL) -> URL? {
        var current = url.standardizedFileURL
        let rootPath = current.pathComponents.first ?? "/"
        while true {
            let goMod = current.appendingPathComponent("go.mod").path
            let cliMain = current.appendingPathComponent("cmd/supermover/main.go").path
            if FileManager.default.fileExists(atPath: goMod) && FileManager.default.fileExists(atPath: cliMain) {
                return current
            }
            let parent = current.deletingLastPathComponent()
            if parent.path == current.path || current.path == rootPath {
                return nil
            }
            current = parent
        }
    }

    private static func developmentInvocation(repoRoot: URL, arguments: [String]) throws -> CLIInvocation {
        let binaryURL = try developmentBinaryURL(repoRoot: repoRoot)
        return CLIInvocation(
            executableURL: URL(filePath: "/bin/sh"),
            arguments: [
                "-c",
                """
                binary=$1
                shift
                child=
                terminate_child() {
                    if [ -n "$child" ]; then
                        kill "$child" 2>/dev/null
                        wait "$child" 2>/dev/null
                    fi
                    exit 143
                }
                trap terminate_child INT TERM
                go build -o "$binary" ./cmd/supermover &
                child=$!
                wait "$child"
                status=$?
                child=
                if [ "$status" -ne 0 ]; then
                    exit "$status"
                fi
                trap - INT TERM
                exec "$binary" "$@"
                """,
                "supermover-dev-launch",
                binaryURL.path,
            ] + arguments,
            workingDirectoryURL: repoRoot
        )
    }

    private static func developmentBinaryURL(repoRoot: URL) throws -> URL {
        let outputDirectory = repoRoot.appendingPathComponent(".tmp/macos-app", isDirectory: true)
        do {
            try FileManager.default.createDirectory(at: outputDirectory, withIntermediateDirectories: true)
        } catch {
            throw SuperMoverCLIError.developmentBuildFailed("could not create \(outputDirectory.path): \(error.localizedDescription)")
        }
        return outputDirectory.appendingPathComponent("supermover-dev")
    }
}

final class ProcessController: @unchecked Sendable {
    let process = Process()
    let stdoutPipe = Pipe()
    let stderrPipe = Pipe()

    var onStdout: ((String) -> Void)?
    var onStderr: ((String) -> Void)?
    var onExit: ((Int32, Bool, ProcessOutputSnapshot) -> Void)?

    private let output = ProcessOutputAccumulator()
    private let stateLock = NSLock()
    private var wasTerminated = false

    var processIdentifier: Int32 {
        process.processIdentifier
    }

    init(invocation: CLIInvocation) throws {
        process.executableURL = invocation.executableURL
        process.arguments = invocation.arguments
        process.currentDirectoryURL = invocation.workingDirectoryURL
        process.standardOutput = stdoutPipe
        process.standardError = stderrPipe
        process.environment = ProcessInfo.processInfo.environment
        stdoutPipe.fileHandleForReading.readabilityHandler = { [weak self] handle in
            guard let self else { return }
            let data = handle.availableData
            if data.isEmpty { return }
            if let text = String(data: data, encoding: .utf8) {
                self.recordStdout(text)
            }
        }
        stderrPipe.fileHandleForReading.readabilityHandler = { [weak self] handle in
            guard let self else { return }
            let data = handle.availableData
            if data.isEmpty { return }
            if let text = String(data: data, encoding: .utf8) {
                self.recordStderr(text)
            }
        }
        process.terminationHandler = { [weak self] process in
            guard let self else { return }
            self.stdoutPipe.fileHandleForReading.readabilityHandler = nil
            self.stderrPipe.fileHandleForReading.readabilityHandler = nil
            self.drainRemainingOutput()
            self.onExit?(process.terminationStatus, self.terminatedByOperator(), self.outputSnapshot())
        }
    }

    func start() throws {
        try process.run()
    }

    func terminate() {
        guard process.isRunning else { return }
        markTerminatedByOperator()
        process.terminate()
    }

    private func recordStdout(_ text: String) {
        output.appendStdout(text)
        onStdout?(text)
    }

    private func recordStderr(_ text: String) {
        output.appendStderr(text)
        onStderr?(text)
    }

    private func drainRemainingOutput() {
        let stdoutData = stdoutPipe.fileHandleForReading.readDataToEndOfFile()
        if !stdoutData.isEmpty, let text = String(data: stdoutData, encoding: .utf8) {
            recordStdout(text)
        }
        let stderrData = stderrPipe.fileHandleForReading.readDataToEndOfFile()
        if !stderrData.isEmpty, let text = String(data: stderrData, encoding: .utf8) {
            recordStderr(text)
        }
    }

    private func markTerminatedByOperator() {
        stateLock.lock()
        wasTerminated = true
        stateLock.unlock()
    }

    private func terminatedByOperator() -> Bool {
        stateLock.lock()
        let terminated = wasTerminated
        stateLock.unlock()
        return terminated
    }

    private func outputSnapshot() -> ProcessOutputSnapshot {
        output.snapshot()
    }
}

enum DashboardURLParser {
    static func firstURL(in stderr: String) -> URL? {
        guard let range = stderr.range(of: "dashboard: url=") else { return nil }
        let suffix = stderr[range.upperBound...]
        guard let urlToken = suffix.split(separator: " ").first else { return nil }
        return URL(string: String(urlToken))
    }
}

enum ServeInfoParser {
    static func firstSummary(in stderr: String) -> String? {
        let lines = stderr.split(separator: "\n")
        if let receiver = lines.last(where: { $0.contains("serve: receiver listening address=") }) {
            return String(receiver)
        }
        if let pairing = lines.last(where: { $0.contains("serve: listening address=") }) {
            return String(pairing)
        }
        return nil
    }
}

enum PairInfoParser {
    static func firstSummary(in stdout: String) -> String? {
        stdout
            .split(separator: "\n")
            .last(where: { $0.hasPrefix("pair: ") })
            .map(String.init)
    }

    static func receiptID(in stdout: String) -> String? {
        guard let summary = firstSummary(in: stdout) else { return nil }
        return summary
            .split(separator: " ")
            .first(where: { $0.hasPrefix("receipt=") })
            .map { String($0.dropFirst("receipt=".count)) }
    }
}
