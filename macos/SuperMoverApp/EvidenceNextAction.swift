import Foundation

struct EvidenceNextAction: Equatable, Identifiable {
    enum Kind: String, CaseIterable, Identifiable {
        case driftRecord
        case driftAcknowledge
        case driftResolve
        case driftExpire
        case syncQueueCancel
        case syncQueueFail
        case pruneApprove
        case pruneSupersede
        case pruneApply
        case reconcileApply
        case pair
        case publish
        case networkPush
        case syncRun
        case syncLoop
        case syncWatch
        case syncNetworkRun
        case syncNetworkDiscoverRun
        case syncNetworkLoop

        var id: String { rawValue }
    }

    struct OperatorIntent: Equatable {
        static let empty = OperatorIntent()

        let selectedDurableEvidenceID: String?
        let selectedDurableEvidenceVerified: Bool
        let approvalID: String?
        let reason: String?
        let reviewer: String?
        let expiresAt: String?
        let sessionID: String?

        init(
            selectedDurableEvidenceID: String? = nil,
            selectedDurableEvidenceVerified: Bool = false,
            approvalID: String? = nil,
            reason: String? = nil,
            reviewer: String? = nil,
            expiresAt: String? = nil,
            sessionID: String? = nil
        ) {
            self.selectedDurableEvidenceID = selectedDurableEvidenceID
            self.selectedDurableEvidenceVerified = selectedDurableEvidenceVerified
            self.approvalID = approvalID
            self.reason = reason
            self.reviewer = reviewer
            self.expiresAt = expiresAt
            self.sessionID = sessionID
        }

        func value(for field: NextActionRequirement.Field) -> String? {
            switch field {
            case .selectedDurableEvidenceID:
                return normalized(selectedDurableEvidenceID)
            case .loadedEvidenceSelection:
                return selectedDurableEvidenceVerified ? "verified" : nil
            case .approvalID:
                return normalized(approvalID)
            case .reason:
                return normalized(reason)
            case .reviewer:
                return normalized(reviewer)
            case .expiresAt:
                return normalized(expiresAt)
            case .session:
                return normalized(sessionID)
            }
        }

        private func normalized(_ value: String?) -> String? {
            guard let trimmed = value?.trimmingCharacters(in: .whitespacesAndNewlines),
                  !trimmed.isEmpty else {
                return nil
            }
            return trimmed
        }
    }

    struct CommandPreview: Equatable {
        enum ProfilePathSource: String {
            case appStoreSelectedProfile = "AppStore.profilePath"
        }

        let executableName: String
        let profilePathSource: ProfilePathSource
        let arguments: [String]
    }

    let kind: Kind
    let title: String
    let summary: String
    let safety: NextActionSafety
    let requirements: [NextActionRequirement]
    let missingRequirements: [NextActionRequirement]
    let prefill: OperatorIntent
    let commandPreview: CommandPreview?
    let disabledReason: String?

    var allowsExecution: Bool {
        safety.executionBoundary == .reviewMetadataExecutable &&
            missingRequirements.isEmpty &&
            commandPreview != nil
    }

    var id: Kind { kind }
}

struct NextActionRequirement: Equatable, Hashable, Identifiable {
    enum EvidenceIDKind: String {
        case persistedDrift
        case syncQueueEntry
        case pruneApproval
        case softDelete
    }

    enum Field: Equatable, Hashable {
        case selectedDurableEvidenceID(EvidenceIDKind)
        case loadedEvidenceSelection
        case approvalID
        case reason
        case reviewer
        case expiresAt
        case session

        var id: String {
            switch self {
            case let .selectedDurableEvidenceID(kind):
                return "selectedDurableEvidenceID:\(kind.rawValue)"
            case .loadedEvidenceSelection:
                return "loadedEvidenceSelection"
            case .approvalID:
                return "approvalID"
            case .reason:
                return "reason"
            case .reviewer:
                return "reviewer"
            case .expiresAt:
                return "expiresAt"
            case .session:
                return "session"
            }
        }

        var displayLabel: String {
            switch self {
            case let .selectedDurableEvidenceID(kind):
                switch kind {
                case .persistedDrift:
                    return "persisted drift ID"
                case .syncQueueEntry:
                    return "sync queue entry ID"
                case .pruneApproval:
                    return "prune approval ID"
                case .softDelete:
                    return "soft-delete ID"
                }
            case .loadedEvidenceSelection:
                return "current loaded evidence selection"
            case .approvalID:
                return "approval ID"
            case .reason:
                return "reason"
            case .reviewer:
                return "reviewer"
            case .expiresAt:
                return "expires at"
            case .session:
                return "session ID"
            }
        }
    }

    enum Necessity: String {
        case required
        case optional
    }

    let field: Field
    let necessity: Necessity
    let detail: String

    var id: String { "\(field.id):\(necessity.rawValue)" }

    static func required(_ field: Field, _ detail: String) -> NextActionRequirement {
        NextActionRequirement(field: field, necessity: .required, detail: detail)
    }

    static func optional(_ field: Field, _ detail: String) -> NextActionRequirement {
        NextActionRequirement(field: field, necessity: .optional, detail: detail)
    }
}

struct NextActionSafety: Equatable {
    enum Boundary: String {
        case firstSliceMetadataPreview
        case plannedPreviewDisabled
        case excludedFromFirstSlice
    }

    enum ExecutionBoundary: String {
        case reviewMetadataExecutable
        case previewOnly
        case disabled
    }

    let boundary: Boundary
    let executionBoundary: ExecutionBoundary
    let explanation: String

    var allowsCommandPreview: Bool {
        boundary == .firstSliceMetadataPreview
    }

    static func reviewMetadataExecutable(_ explanation: String) -> NextActionSafety {
        NextActionSafety(
            boundary: .firstSliceMetadataPreview,
            executionBoundary: .reviewMetadataExecutable,
            explanation: explanation
        )
    }

    static func firstSliceMetadataPreview(_ explanation: String) -> NextActionSafety {
        NextActionSafety(
            boundary: .firstSliceMetadataPreview,
            executionBoundary: .previewOnly,
            explanation: explanation
        )
    }

    static func plannedPreviewDisabled(_ explanation: String) -> NextActionSafety {
        NextActionSafety(
            boundary: .plannedPreviewDisabled,
            executionBoundary: .disabled,
            explanation: explanation
        )
    }

    static func excludedFromFirstSlice(_ explanation: String) -> NextActionSafety {
        NextActionSafety(
            boundary: .excludedFromFirstSlice,
            executionBoundary: .disabled,
            explanation: explanation
        )
    }
}

struct NextActionPlanner {
    private static let profilePlaceholder = "<AppStore.profilePath>"

    func plan(
        _ kind: EvidenceNextAction.Kind,
        intent: EvidenceNextAction.OperatorIntent = .empty
    ) -> EvidenceNextAction {
        let definition = definition(for: kind)
        var missingRequirements = definition.requirements.filter { requirement in
            requirement.necessity == .required && intent.value(for: requirement.field) == nil
        }
        if missingRequirements.isEmpty, definition.requiresVerifiedEvidenceSelection {
            let verifiedSelection = NextActionRequirement.required(
                .loadedEvidenceSelection,
                "Select an id from current loaded Evidence Vault artifacts before previewing a mutation command."
            )
            if intent.value(for: verifiedSelection.field) == nil {
                missingRequirements.append(verifiedSelection)
            }
        }
        let commandPreview = makeCommandPreview(
            definition: definition,
            intent: intent,
            missingRequirements: missingRequirements
        )

        return EvidenceNextAction(
            kind: kind,
            title: definition.title,
            summary: definition.summary,
            safety: definition.safety,
            requirements: definition.requirements,
            missingRequirements: missingRequirements,
            prefill: intent,
            commandPreview: commandPreview,
            disabledReason: disabledReason(
                safety: definition.safety,
                missingRequirements: missingRequirements
            )
        )
    }

    func planInventory(
        intent: EvidenceNextAction.OperatorIntent = .empty
    ) -> [EvidenceNextAction] {
        EvidenceNextAction.Kind.allCases.map { plan($0, intent: intent) }
    }

    private func makeCommandPreview(
        definition: ActionDefinition,
        intent: EvidenceNextAction.OperatorIntent,
        missingRequirements: [NextActionRequirement]
    ) -> EvidenceNextAction.CommandPreview? {
        guard definition.safety.allowsCommandPreview,
              missingRequirements.isEmpty,
              let arguments = definition.commandArguments(intent) else {
            return nil
        }
        return EvidenceNextAction.CommandPreview(
            executableName: "supermover",
            profilePathSource: .appStoreSelectedProfile,
            arguments: arguments
        )
    }

    private func disabledReason(
        safety: NextActionSafety,
        missingRequirements: [NextActionRequirement]
    ) -> String? {
        if !safety.allowsCommandPreview {
            return safety.explanation
        }
        if !missingRequirements.isEmpty {
            let fields = missingRequirements.map(\.field.displayLabel).joined(separator: ", ")
            return "Missing required operator intent: \(fields)."
        }
        return nil
    }

    private func definition(for kind: EvidenceNextAction.Kind) -> ActionDefinition {
        switch kind {
        case .driftRecord:
            return ActionDefinition(
                title: "Record Drift",
                summary: "Persist current live drift findings as durable review records; does not repair, resolve, or prune.",
                safety: .reviewMetadataExecutable("Executable review-metadata action. Records drift review evidence and leaves target contents unchanged."),
                requirements: [
                    .optional(.session, "Scope recording to a selected evidence session when one is available."),
                ],
                commandArguments: { intent in
                    var arguments = ["drift", "record", "--profile", Self.profilePlaceholder, "--format", "json"]
                    Self.appendOptional("--session", field: .session, from: intent, to: &arguments)
                    return arguments
                }
            )
        case .driftAcknowledge:
            return driftReviewDefinition(
                title: "Acknowledge Drift",
                summary: "Add acknowledgement metadata to one existing persisted drift record.",
                verb: "acknowledge"
            )
        case .driftResolve:
            return driftReviewDefinition(
                title: "Resolve Drift",
                summary: "Mark one existing persisted drift record resolved after fresh clean detector evidence.",
                verb: "resolve"
            )
        case .driftExpire:
            return driftReviewDefinition(
                title: "Expire Drift",
                summary: "Retire one stale persisted drift record without claiming target restoration.",
                verb: "expire"
            )
        case .syncQueueCancel:
            return syncQueueReviewDefinition(
                title: "Cancel Sync Queue Entry",
                summary: "Mark one durable queue entry canceled with an operator reason.",
                verb: "cancel"
            )
        case .syncQueueFail:
            return syncQueueReviewDefinition(
                title: "Fail Sync Queue Entry",
                summary: "Mark one durable queue entry failed as terminal operator review evidence.",
                verb: "fail"
            )
        case .pruneApprove:
            return ActionDefinition(
                title: "Approve Prune",
                summary: "Write a prune approval artifact from verified soft-delete evidence without deleting target files.",
                safety: .reviewMetadataExecutable("Executable review-metadata action. Writes prune approval metadata only; physical prune apply remains excluded."),
                requirements: [
                    .required(.selectedDurableEvidenceID(.softDelete), "Select reviewed soft-delete evidence before approval authoring."),
                    .required(.approvalID, "Provide the explicit prune approval id used for prune approve --id."),
                    .required(.reason, "Explain why prune approval is appropriate."),
                    .required(.reviewer, "Record the reviewer approving the prune evidence."),
                    .optional(.expiresAt, "Record an RFC3339 approval expiry when one is required."),
                ],
                commandArguments: { intent in
                    guard let softDeleteID = intent.value(for: .selectedDurableEvidenceID(.softDelete)),
                          let approvalID = intent.value(for: .approvalID),
                          let reason = intent.value(for: .reason),
                          let reviewer = intent.value(for: .reviewer) else {
                        return nil
                    }
                    var arguments = [
                        "prune", "approve",
                        "--profile", Self.profilePlaceholder,
                        "--id", approvalID,
                        "--reason", reason,
                        "--reviewer", reviewer,
                        "--format", "json",
                        "--soft-delete", softDeleteID,
                    ]
                    Self.appendOptional("--expires-at", field: .expiresAt, from: intent, to: &arguments)
                    return arguments
                }
            )
        case .pruneSupersede:
            return ActionDefinition(
                title: "Supersede Prune Approval",
                summary: "Supersede one existing prune approval artifact without deleting target files.",
                safety: .reviewMetadataExecutable("Executable review-metadata action. Supersedes durable approval metadata only; physical prune apply remains excluded."),
                requirements: [
                    .required(.selectedDurableEvidenceID(.pruneApproval), "Select one existing prune approval artifact."),
                    .required(.reason, "Explain why the approval is being superseded."),
                    .required(.reviewer, "Record the reviewer superseding the approval."),
                ],
                commandArguments: { intent in
                    guard let approvalID = intent.value(for: .selectedDurableEvidenceID(.pruneApproval)),
                          let reason = intent.value(for: .reason),
                          let reviewer = intent.value(for: .reviewer) else {
                        return nil
                    }
                    return [
                        "prune", "supersede",
                        "--profile", Self.profilePlaceholder,
                        "--id", approvalID,
                        "--reason", reason,
                        "--reviewer", reviewer,
                        "--format", "json",
                    ]
                }
            )
        case .pruneApply:
            return excludedDefinition(
                title: "Apply Prune",
                summary: "Physical prune execution is explicitly outside first-slice next actions.",
                explanation: "Prune apply mutates target files and must stay outside previewable Evidence Vault auto-actions."
            )
        case .reconcileApply:
            return ActionDefinition(
                title: "Apply Reconcile",
                summary: "Repair execution is explicitly outside first-slice next actions.",
                safety: .excludedFromFirstSlice("Reconcile apply can change target contents and is excluded from first-slice auto-actions."),
                requirements: [
                    .required(.selectedDurableEvidenceID(.persistedDrift), "Select a persisted drift record before repair apply."),
                    .required(.reason, "Explain the repair intent."),
                    .optional(.reviewer, "Record the reviewer when available."),
                    .optional(.session, "Scope apply to a session when required by the review flow."),
                ],
                commandArguments: { _ in nil }
            )
        case .pair:
            return excludedDefinition(
                title: "Pair Target",
                summary: "Pairing pins trust state and is explicitly outside first-slice next actions.",
                explanation: "Pairing changes trust state and is excluded from Evidence Vault auto-actions."
            )
        case .publish:
            return excludedDefinition(
                title: "Publish",
                summary: "Push execution is explicitly outside first-slice next actions.",
                explanation: "Publish can mutate target state and is excluded from Evidence Vault auto-actions."
            )
        case .networkPush:
            return excludedDefinition(
                title: "Network Push",
                summary: "Network push execution is explicitly outside first-slice next actions.",
                explanation: "Network push transfers data and is excluded from Evidence Vault auto-actions."
            )
        case .syncRun:
            return syncExecutionExcludedDefinition(title: "Sync Run")
        case .syncLoop:
            return syncExecutionExcludedDefinition(title: "Sync Loop")
        case .syncWatch:
            return syncExecutionExcludedDefinition(title: "Sync Watch")
        case .syncNetworkRun:
            return syncExecutionExcludedDefinition(title: "Sync Network Run")
        case .syncNetworkDiscoverRun:
            return syncExecutionExcludedDefinition(title: "Sync Network Discover Run")
        case .syncNetworkLoop:
            return syncExecutionExcludedDefinition(title: "Sync Network Loop")
        }
    }

    private func driftReviewDefinition(
        title: String,
        summary: String,
        verb: String
    ) -> ActionDefinition {
        ActionDefinition(
            title: title,
            summary: summary,
            safety: .reviewMetadataExecutable("Executable review-metadata action. Updates durable drift review metadata and does not repair, reconcile, prune, or suppress live detector output."),
            requirements: [
                .required(.selectedDurableEvidenceID(.persistedDrift), "Select one persisted .supermover drift record."),
                .required(.reason, "Record the operator reason for the drift review decision."),
                .optional(.reviewer, "Record the reviewer when available."),
            ],
            commandArguments: { intent in
                guard let driftID = intent.value(for: .selectedDurableEvidenceID(.persistedDrift)),
                      let reason = intent.value(for: .reason) else {
                    return nil
                }
                var arguments = [
                    "drift", verb,
                    "--profile", Self.profilePlaceholder,
                    "--id", driftID,
                    "--reason", reason,
                    "--format", "json",
                ]
                Self.appendOptional("--reviewer", field: .reviewer, from: intent, to: &arguments)
                return arguments
            }
        )
    }

    private func syncQueueReviewDefinition(
        title: String,
        summary: String,
        verb: String
    ) -> ActionDefinition {
        ActionDefinition(
            title: title,
            summary: summary,
            safety: .reviewMetadataExecutable("Executable review-metadata action. Updates durable queue review metadata and does not run sync transfer work."),
            requirements: [
                .required(.selectedDurableEvidenceID(.syncQueueEntry), "Select one durable sync queue entry."),
                .required(.reason, "Record the operator reason for the queue review decision."),
            ],
            commandArguments: { intent in
                guard let entryID = intent.value(for: .selectedDurableEvidenceID(.syncQueueEntry)),
                      let reason = intent.value(for: .reason) else {
                    return nil
                }
                return [
                    "sync", "queue", verb,
                    "--profile", Self.profilePlaceholder,
                    "--id", entryID,
                    "--reason", reason,
                    "--format", "json",
                ]
            }
        )
    }

    private func excludedDefinition(
        title: String,
        summary: String,
        explanation: String
    ) -> ActionDefinition {
        ActionDefinition(
            title: title,
            summary: summary,
            safety: .excludedFromFirstSlice(explanation),
            requirements: [],
            commandArguments: { _ in nil }
        )
    }

    private func syncExecutionExcludedDefinition(title: String) -> ActionDefinition {
        excludedDefinition(
            title: title,
            summary: "Sync execution is explicitly outside first-slice next actions.",
            explanation: "Sync execution can copy data, contact a receiver, or run a foreground worker and is excluded from first-slice auto-actions."
        )
    }

    private static func appendOptional(
        _ flag: String,
        field: NextActionRequirement.Field,
        from intent: EvidenceNextAction.OperatorIntent,
        to arguments: inout [String]
    ) {
        guard let value = intent.value(for: field) else {
            return
        }
        arguments += [flag, value]
    }
}

private struct ActionDefinition {
    let title: String
    let summary: String
    let safety: NextActionSafety
    let requirements: [NextActionRequirement]
    let commandArguments: (EvidenceNextAction.OperatorIntent) -> [String]?

    var requiresVerifiedEvidenceSelection: Bool {
        requirements.contains { requirement in
            guard requirement.necessity == .required else {
                return false
            }
            if case .selectedDurableEvidenceID = requirement.field {
                return true
            }
            return false
        }
    }
}
