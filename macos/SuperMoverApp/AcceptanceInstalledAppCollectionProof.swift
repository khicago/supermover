import Foundation

enum AcceptanceInstalledAppCollectionProofConstants {
    static let machineFactsSchema = "supermover.acceptance.machine_facts.v1"
    static let sourceMachineFactsArtifact = "source.machine.json"
    static let targetMachineFactsArtifact = "target.machine.json"
}

enum AcceptanceInstalledAppCollectionBlockedReason: Equatable {
    case invalidCollection
    case sameRoleMachineIDs
    case invalidMachineFactArtifactSchema
    case sameMachineFactArtifactIDs
    case machineFactsInconsistent
    case conflictingRoleAndMachineFacts
    case contradictoryVerifiedBundleHandoffs

    var workflowStatusValue: String {
        switch self {
        case .invalidCollection:
            return "invalid_collection"
        case .sameRoleMachineIDs:
            return "same_role_machine_ids"
        case .invalidMachineFactArtifactSchema:
            return "invalid_machine_fact_artifact_schema"
        case .sameMachineFactArtifactIDs:
            return "same_machine_fact_artifact_ids"
        case .machineFactsInconsistent:
            return "machine_facts_inconsistent"
        case .conflictingRoleAndMachineFacts:
            return "conflicting_role_and_machine_facts"
        case .contradictoryVerifiedBundleHandoffs:
            return "contradictory_verified_bundle_handoffs"
        }
    }
}

extension AcceptanceInstalledAppCollectionBlockedReason {
    var supportsMachineIdentityCorrectionLaunch: Bool {
        switch self {
        case .sameRoleMachineIDs,
             .invalidMachineFactArtifactSchema,
             .sameMachineFactArtifactIDs,
             .machineFactsInconsistent,
             .conflictingRoleAndMachineFacts:
            return true
        case .invalidCollection,
             .contradictoryVerifiedBundleHandoffs:
            return false
        }
    }
}

enum AcceptanceInstalledAppCollectionMissingRequirement: Equatable {
    case roleMachineIDs
    case machineFactArtifacts
    case machineFactArtifactIDs
    case machineFactMetadata
    case verifiedBundleHandoffs

    var workflowStatusValue: String {
        switch self {
        case .roleMachineIDs:
            return "role_machine_ids"
        case .machineFactArtifacts:
            return "machine_fact_artifacts"
        case .machineFactArtifactIDs:
            return "machine_fact_artifact_ids"
        case .machineFactMetadata:
            return "machine_fact_metadata"
        case .verifiedBundleHandoffs:
            return "verified_bundle_handoffs"
        }
    }
}

enum AcceptanceInstalledAppMachineIdentityCorrectionLaunch: Equatable {
    case sourcePair(reason: String)
    case targetServe(reason: String)

    var machine: String {
        switch self {
        case .sourcePair:
            return "source"
        case .targetServe:
            return "target"
        }
    }

    var detail: String {
        switch self {
        case let .sourcePair(reason):
            return "Current acceptance bundle needs installed-app machine identity correction (\(reason)). Launch can rewrite source_pair and source.machine.json evidence from this installed app; final evaluate remains blocked until the target installed app records matching target evidence and any required bundle_handoff is completed."
        case let .targetServe(reason):
            return "Current acceptance bundle needs installed-app machine identity correction (\(reason)). Launch can rewrite target role and target.machine.json evidence from this installed app; final evaluate remains blocked until the source installed app records matching source_pair evidence and any required bundle_handoff is completed."
        }
    }
}

struct AcceptanceInstalledAppCollectionProofSummary: Equatable {
    struct MachinePair: Equatable {
        let source: String
        let target: String

        var machineIDs: Set<String> {
            Set([source, target])
        }

        var dictionary: [String: String?] {
            [
                "source": source,
                "target": target,
            ]
        }
    }

    let collectionMode: String
    let machineCount: Int
    let roleMachineIDs: [String: String?]
    let machineFactIDs: [String: String?]
    let machineFactArtifactIDs: [String: String?]
    let collectionOK: Bool
    let roleMachineIDsPresent: Bool
    let roleMachineIDsDistinct: Bool
    let machineFactArtifactsPresent: Bool
    let machineFactArtifactsSchemaOK: Bool
    let machineFactArtifactIDsPresent: Bool
    let machineFactArtifactIDsDistinct: Bool
    let verifiedBundleHandoffs: Int
    let verifiedCrossMachineBundleHandoffs: Int
    let matchesRecordedMachinePair: Bool
    let machineFactsConsistent: Bool
    let installedAppMachinePair: MachinePair?
    let hasInstalledAppMachinePairProof: Bool
    let primaryFailure: String?
    let failureMessage: String?
    let ok: Bool
    let failures: [String]

    var workflowStatusFailureMessage: String? {
        finalEvaluationCollectionDetail
            ?? finalEvaluationMachineFactsDetail
            ?? finalEvaluationBundleHandoffDetail
            ?? failureMessage
    }

    var blockedReason: AcceptanceInstalledAppCollectionBlockedReason? {
        let machineFactIDsPresent = machineFactIDs.values.allSatisfy { $0 != nil }

        if !collectionOK {
            return .invalidCollection
        }
        if roleMachineIDsPresent && !roleMachineIDsDistinct {
            return .sameRoleMachineIDs
        }
        if machineFactArtifactsPresent && !machineFactArtifactsSchemaOK {
            return .invalidMachineFactArtifactSchema
        }
        if machineFactArtifactIDsPresent && !machineFactArtifactIDsDistinct {
            return .sameMachineFactArtifactIDs
        }
        if !machineFactsConsistent {
            return .machineFactsInconsistent
        }
        if roleMachineIDsPresent
            && machineFactArtifactsPresent
            && machineFactArtifactIDsPresent
            && machineFactIDsPresent
            && !hasInstalledAppMachinePairProof {
            return .conflictingRoleAndMachineFacts
        }
        if failures.contains("contradictory_verified_bundle_handoffs") {
            return .contradictoryVerifiedBundleHandoffs
        }
        return nil
    }

    var missingRequirements: [AcceptanceInstalledAppCollectionMissingRequirement] {
        var requirements: [AcceptanceInstalledAppCollectionMissingRequirement] = []
        let machineFactIDsPresent = machineFactIDs.values.allSatisfy { $0 != nil }

        if !roleMachineIDsPresent {
            requirements.append(.roleMachineIDs)
        }
        if !machineFactArtifactsPresent {
            requirements.append(.machineFactArtifacts)
        } else if !machineFactArtifactIDsPresent {
            requirements.append(.machineFactArtifactIDs)
        } else if !machineFactIDsPresent {
            requirements.append(.machineFactMetadata)
        }
        if verifiedBundleHandoffs == 0 {
            requirements.append(.verifiedBundleHandoffs)
        }
        return requirements
    }

    var requiresMachineIdentityCorrection: Bool {
        if let blockedReason {
            return blockedReason != .contradictoryVerifiedBundleHandoffs
        }
        return missingRequirements.contains { $0 != .verifiedBundleHandoffs }
    }

    var requiresBundleHandoffProof: Bool {
        blockedReason == nil
            && !ok
            && !requiresMachineIdentityCorrection
            && failures.contains("handoff_does_not_match_recorded_machine_pair")
    }

    var finalEvaluationCollectionDetail: String? {
        switch blockedReason {
        case .invalidCollection:
            if collectionMode != "two_machine" {
                let label = collectionMode.isEmpty ? "missing collection.mode" : "collection.mode=\(collectionMode)"
                return label
            }
            return "collection.machine_count=\(machineCount)"
        case .sameRoleMachineIDs:
            let sharedMachineID = Self.cleanID(roleMachineIDs["source"] ?? nil)
                ?? Self.cleanID(roleMachineIDs["target"] ?? nil)
                ?? "unknown"
            return "source_pair and target share machine_id=\(sharedMachineID)"
        default:
            if Self.cleanID(roleMachineIDs["source"] ?? nil) == nil {
                return "missing roles.source_pair.machine_id"
            }
            if Self.cleanID(roleMachineIDs["target"] ?? nil) == nil {
                return "missing roles.target.machine_id"
            }
            return nil
        }
    }

    var finalEvaluationMachineFactsDetail: String? {
        switch blockedReason {
        case .sameMachineFactArtifactIDs:
            let sharedMachineID = Self.cleanID(machineFactArtifactIDs["source"] ?? nil)
                ?? Self.cleanID(machineFactArtifactIDs["target"] ?? nil)
                ?? "unknown"
            return "source and target machine facts share machine_id=\(sharedMachineID)"
        case .machineFactsInconsistent, .conflictingRoleAndMachineFacts:
            return "roles.source_pair/target machine_id do not match source.machine.json and target.machine.json"
        default:
            return nil
        }
    }

    var finalEvaluationBundleHandoffDetail: String? {
        if failures.contains("missing_verified_bundle_handoffs") {
            return "missing verified bundle_handoffs"
        }
        if failures.contains("contradictory_verified_bundle_handoffs") {
            return "bundle_handoffs contain verified cross-machine archive handoff evidence for machine ids other than the recorded source/target pair"
        }
        if failures.contains("handoff_does_not_match_recorded_machine_pair") {
            return "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"
        }
        return nil
    }

    func machineIdentityCorrectionLaunch(
        for kind: SuperMoverTaskKind
    ) -> AcceptanceInstalledAppMachineIdentityCorrectionLaunch? {
        guard requiresMachineIdentityCorrection else {
            return nil
        }
        if let blockedReason, !blockedReason.supportsMachineIdentityCorrectionLaunch {
            return nil
        }

        let reason =
            finalEvaluationCollectionDetail
            ?? finalEvaluationMachineFactsDetail
            ?? missingMachineIdentityRequirementDetail
            ?? failureMessage
            ?? "source/target machine identity evidence does not agree"
        switch kind {
        case .pair:
            return .sourcePair(reason: reason)
        case .serve:
            return .targetServe(reason: reason)
        default:
            return nil
        }
    }

    private var missingMachineIdentityRequirementDetail: String? {
        for requirement in missingRequirements {
            switch requirement {
            case .roleMachineIDs:
                return "missing source/target role machine ids"
            case .machineFactArtifacts:
                return "missing source/target machine fact artifacts"
            case .machineFactArtifactIDs:
                return "missing source/target machine fact artifact machine_id"
            case .machineFactMetadata:
                return "missing source/target machine fact metadata"
            case .verifiedBundleHandoffs:
                continue
            }
        }
        return nil
    }

    static func evaluate(snapshot: AcceptanceBundleLoadedSnapshot) -> AcceptanceInstalledAppCollectionProofSummary {
        let roleMachineIDs: [String: String?] = [
            "source": cleanID(snapshot.meta.roles["source_pair"]?.machine_id),
            "target": cleanID(snapshot.meta.roles["target"]?.machine_id),
        ]
        let machineFactIDs: [String: String?] = [
            "source": cleanID(snapshot.meta.sourceMachineFacts?.machine_id),
            "target": cleanID(snapshot.meta.targetMachineFacts?.machine_id),
        ]

        let sourceArtifactSchemaOK = snapshot.sourceMachineFactsArtifact?.schema == AcceptanceInstalledAppCollectionProofConstants.machineFactsSchema
        let targetArtifactSchemaOK = snapshot.targetMachineFactsArtifact?.schema == AcceptanceInstalledAppCollectionProofConstants.machineFactsSchema
        let machineFactArtifactsPresent = snapshot.sourceMachineFactsArtifact != nil && snapshot.targetMachineFactsArtifact != nil
        let machineFactArtifactsSchemaOK = sourceArtifactSchemaOK && targetArtifactSchemaOK
        let machineFactArtifactIDs: [String: String?] = [
            "source": cleanID(snapshot.sourceMachineFactsArtifact?.machine_id),
            "target": cleanID(snapshot.targetMachineFactsArtifact?.machine_id),
        ]

        var machineFactsConsistent = true
        for machine in ["source", "target"] {
            let metaRecord: AcceptanceBundleSnapshot.MachineFactsEvidence?
            let artifact: AcceptanceBundleSnapshot.MachineFactsArtifact?
            switch machine {
            case "source":
                metaRecord = snapshot.meta.sourceMachineFacts
                artifact = snapshot.sourceMachineFactsArtifact
            case "target":
                metaRecord = snapshot.meta.targetMachineFacts
                artifact = snapshot.targetMachineFactsArtifact
            default:
                metaRecord = nil
                artifact = nil
            }

            let output = cleanID(metaRecord?.output)
            if output != nil && (artifact == nil || artifact?.schema != AcceptanceInstalledAppCollectionProofConstants.machineFactsSchema) {
                machineFactsConsistent = false
                break
            }
            if let artifact {
                let metaMachineID = cleanID(metaRecord?.machine_id)
                if let metaMachineID, metaMachineID != cleanID(artifact.machine_id) {
                    machineFactsConsistent = false
                    break
                }
            }
        }

        let roleMachinePair = distinctPair(roleMachineIDs)
        let machineFactPair = distinctPair(machineFactIDs)
        let machineFactArtifactPair = distinctPair(machineFactArtifactIDs)

        let roleMatchesMetaMachineFacts = roleMachinePair == machineFactPair && roleMachinePair != nil
        let roleMatchesMachineFactArtifacts = roleMachinePair == machineFactArtifactPair && roleMachinePair != nil
        let metaMatchesMachineFactArtifacts = machineFactPair == machineFactArtifactPair && machineFactPair != nil

        let installedAppMachinePair =
            machineFactsConsistent
            && roleMatchesMetaMachineFacts
            && roleMatchesMachineFactArtifacts
            && metaMatchesMachineFactArtifacts
            ? machineFactArtifactPair
            : nil

        let verifiedBundleHandoffs = snapshot.bundleHandoffs.filter(\.verified).count
        let matchedHandoffs = snapshot.bundleHandoffs.filter { handoff in
            guard handoff.verified,
                  let machinePair = installedAppMachinePair,
                  let exportingMachineID = cleanID(handoff.exporting_machine_id),
                  let importingMachineID = cleanID(handoff.importing_machine_id),
                  exportingMachineID != importingMachineID else {
                return false
            }
            return Set([exportingMachineID, importingMachineID]) == machinePair.machineIDs
        }
        let contradictoryVerifiedHandoffs = snapshot.bundleHandoffs.filter { handoff in
            guard handoff.verified,
                  let machinePair = installedAppMachinePair,
                  let exportingMachineID = cleanID(handoff.exporting_machine_id),
                  let importingMachineID = cleanID(handoff.importing_machine_id),
                  exportingMachineID != importingMachineID else {
                return false
            }
            return Set([exportingMachineID, importingMachineID]) != machinePair.machineIDs
        }
        let hasContradictoryVerifiedHandoffs = !matchedHandoffs.isEmpty && !contradictoryVerifiedHandoffs.isEmpty

        let collectionMode = snapshot.collectionMode
        let machineCount = snapshot.machineCount
        let collectionOK = collectionMode == "two_machine" && machineCount >= 2
        let roleMachineIDsPresent = roleMachineIDs.values.allSatisfy { $0 != nil }
        let roleMachineIDsDistinct = roleMachinePair != nil
        let machineFactArtifactIDsPresent = machineFactArtifactIDs.values.allSatisfy { $0 != nil }
        let machineFactArtifactIDsDistinct = machineFactArtifactPair != nil

        var failures: [String] = []
        if !collectionOK {
            failures.append("invalid_collection")
        }
        if !roleMachineIDsPresent {
            failures.append("missing_role_machine_ids")
        } else if !roleMachineIDsDistinct {
            failures.append("same_role_machine_ids")
        }
        if !machineFactArtifactsPresent {
            failures.append("missing_machine_fact_artifacts")
        } else if !machineFactArtifactsSchemaOK {
            failures.append("invalid_machine_fact_artifact_schema")
        } else if !machineFactArtifactIDsPresent {
            failures.append("missing_machine_fact_artifact_ids")
        } else if !machineFactArtifactIDsDistinct {
            failures.append("same_machine_fact_artifact_ids")
        }
        if verifiedBundleHandoffs < 1 {
            failures.append("missing_verified_bundle_handoffs")
        }
        if hasContradictoryVerifiedHandoffs {
            failures.append("contradictory_verified_bundle_handoffs")
        }
        if matchedHandoffs.isEmpty {
            failures.append("handoff_does_not_match_recorded_machine_pair")
        }

        let primaryFailure = failures.first
        let sameMachineFactArtifactID = cleanID(machineFactArtifactIDs["source"] ?? nil) ?? ""
        let fallbackFailureMessage: String? =
            switch primaryFailure {
            case "invalid_collection":
                "invalid two-machine collection evidence"
            case "missing_role_machine_ids":
                "missing source/target role machine ids"
            case "same_role_machine_ids":
                "source and target roles share machine_id"
            case "missing_machine_fact_artifacts":
                "missing source/target machine fact artifacts"
            case "invalid_machine_fact_artifact_schema":
                "invalid source/target machine fact artifact schema"
            case "missing_machine_fact_artifact_ids":
                "missing source/target machine fact artifact machine_id"
            case "same_machine_fact_artifact_ids":
                "source and target machine facts share machine_id=\(sameMachineFactArtifactID)"
            case "missing_verified_bundle_handoffs":
                "missing verified bundle_handoffs"
            case "contradictory_verified_bundle_handoffs":
                "bundle_handoffs contain verified cross-machine archive handoff evidence for machine ids other than the recorded source/target pair"
            case "handoff_does_not_match_recorded_machine_pair":
                "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"
            default:
                nil
            }
        func makeSummary(failureMessage: String?) -> AcceptanceInstalledAppCollectionProofSummary {
            AcceptanceInstalledAppCollectionProofSummary(
                collectionMode: collectionMode,
                machineCount: machineCount,
                roleMachineIDs: roleMachineIDs,
                machineFactIDs: machineFactIDs,
                machineFactArtifactIDs: machineFactArtifactIDs,
                collectionOK: collectionOK,
                roleMachineIDsPresent: roleMachineIDsPresent,
                roleMachineIDsDistinct: roleMachineIDsDistinct,
                machineFactArtifactsPresent: machineFactArtifactsPresent,
                machineFactArtifactsSchemaOK: machineFactArtifactsSchemaOK,
                machineFactArtifactIDsPresent: machineFactArtifactIDsPresent,
                machineFactArtifactIDsDistinct: machineFactArtifactIDsDistinct,
                verifiedBundleHandoffs: verifiedBundleHandoffs,
                verifiedCrossMachineBundleHandoffs: matchedHandoffs.count,
                matchesRecordedMachinePair: !matchedHandoffs.isEmpty && !hasContradictoryVerifiedHandoffs,
                machineFactsConsistent: machineFactsConsistent,
                installedAppMachinePair: installedAppMachinePair,
                hasInstalledAppMachinePairProof: installedAppMachinePair != nil,
                primaryFailure: primaryFailure,
                failureMessage: failureMessage,
                ok: failures.isEmpty,
                failures: failures
            )
        }

        let provisionalSummary = makeSummary(failureMessage: fallbackFailureMessage)
        let failureMessage =
            provisionalSummary.finalEvaluationCollectionDetail
            ?? provisionalSummary.finalEvaluationMachineFactsDetail
            ?? provisionalSummary.finalEvaluationBundleHandoffDetail
            ?? fallbackFailureMessage

        return makeSummary(failureMessage: failureMessage)
    }

    func matches(workflowStatus artifact: AcceptanceBundleSnapshot.WorkflowStatusArtifact) -> Bool {
        artifact.collection_mode == collectionMode
            && artifact.machine_count == machineCount
            && artifact.verified_bundle_handoffs == verifiedBundleHandoffs
            && artifact.verified_cross_machine_bundle_handoffs == verifiedCrossMachineBundleHandoffs
            && artifact.matches_recorded_machine_pair == matchesRecordedMachinePair
            && artifact.has_installed_app_machine_pair_proof == hasInstalledAppMachinePairProof
            && artifact.installed_app_proof_ok == ok
            && artifact.installed_app_proof_failures == failures
            && artifact.failures == failures
            && artifact.blocked_reason == blockedReason?.workflowStatusValue
            && artifact.missing_requirements == missingRequirements.map(\.workflowStatusValue)
            && artifact.primary_failure == primaryFailure
            && artifact.failure_message == workflowStatusFailureMessage
            && artifact.requires_machine_identity_correction == requiresMachineIdentityCorrection
            && artifact.requires_bundle_handoff_proof == requiresBundleHandoffProof
            && artifact.final_evaluation_collection_detail == finalEvaluationCollectionDetail
            && artifact.final_evaluation_machine_facts_detail == finalEvaluationMachineFactsDetail
            && artifact.final_evaluation_bundle_handoff_detail == finalEvaluationBundleHandoffDetail
            && artifact.role_machine_ids == roleMachineIDs
            && artifact.machine_fact_ids == machineFactIDs
            && artifact.machine_fact_artifact_ids == machineFactArtifactIDs
            && artifact.machine_facts_consistent == machineFactsConsistent
    }

    private static func cleanID(_ value: String?) -> String? {
        guard let value else {
            return nil
        }
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    private static func distinctPair(_ mapping: [String: String?]) -> MachinePair? {
        guard let source = cleanID(mapping["source"] ?? nil),
              let target = cleanID(mapping["target"] ?? nil),
              source != target else {
            return nil
        }
        return MachinePair(source: source, target: target)
    }
}
