import Foundation

struct AcceptanceBundleArtifactIssue: Identifiable, Equatable {
    let artifact: String
    let problem: String

    var id: String { "\(artifact):\(problem)" }
}

struct AcceptanceSourcePairArtifact: Decodable, Equatable {
    let profile: String
    let target_address: String
    let verification_code: String
    let pairing_receipt_id: String
    let receipt_path: String
}

struct AcceptanceSourceTransferArtifact: Decodable, Equatable {
    let profile: String
    let session_id: String
    let target_address: String
    let receiver_address: String
    let target_mode: String
}

struct AcceptanceEvaluationArtifact: Decodable, Equatable {
    let schema: String
    let status: String
    let pairing_receipt_id: String
    let session_id: String
    let target_root: String
    let require_operator_evidence: Bool?
}

struct AcceptanceServePhaseArtifact: Equatable, Identifiable {
    var id: Int { phase }

    let phase: Int
    let path: String
    let readiness: ServeReadinessSnapshot
}

struct AcceptanceBundleLoadedSnapshot {
    struct InstalledAppMachinePairProof: Equatable {
        let sourceMachineID: String
        let targetMachineID: String

        var machineIDs: Set<String> { Set([sourceMachineID, targetMachineID]) }
    }

    struct VerifiedInstalledAppMachinePairHandoff: Equatable {
        let machinePair: InstalledAppMachinePairProof
        let handoff: AcceptanceBundleSnapshot.BundleHandoffEvidence
    }

    struct WorkflowStep: Equatable, Identifiable {
        let id: String
        let machine: String
        let description: String
        let done: Bool
    }

    struct WorkflowAction: Equatable, Identifiable {
        let machine: String
        let step: String
        let action: String
        let commands: [String]

        var id: String { "\(machine):\(step)" }
    }

    let bundleRootPath: String
    let meta: AcceptanceBundleSnapshot
    let sourceBrowseSnapshot: DiscoveryBrowseSnapshot?
    let targetAdvertiseSnapshot: DiscoveryAdvertiseSnapshot?
    let targetReadyArtifact: ServeReadinessSnapshot?
    let sourceProvenanceArtifact: AcceptanceBundleSnapshot.ProvenanceArtifact?
    let targetProvenanceArtifact: AcceptanceBundleSnapshot.ProvenanceArtifact?
    let sourceAppAuditArtifact: AcceptanceBundleSnapshot.AppAuditArtifact?
    let targetAppAuditArtifact: AcceptanceBundleSnapshot.AppAuditArtifact?
    let sourceNotarizationArtifact: AcceptanceBundleSnapshot.NotarizationArtifact?
    let targetNotarizationArtifact: AcceptanceBundleSnapshot.NotarizationArtifact?
    let sourceMachineFactsArtifact: AcceptanceBundleSnapshot.MachineFactsArtifact?
    let targetMachineFactsArtifact: AcceptanceBundleSnapshot.MachineFactsArtifact?
    let workflowSummaryArtifact: AcceptanceBundleSnapshot.WorkflowSummaryArtifact?
    let sourcePairArtifact: AcceptanceSourcePairArtifact?
    let hasSourcePairReceiptArtifact: Bool
    let hasValidSourcePairReceiptArtifact: Bool
    let hasSourcePairTranscriptArtifact: Bool
    let sourceTransferArtifact: AcceptanceSourceTransferArtifact?
    let hasSourceNetworkPushTranscriptArtifact: Bool
    let sourceConsistencyArtifact: AcceptanceBundleSnapshot.SourceConsistencyEvidence?
    let hasDecodedSourceConsistencyArtifact: Bool
    let hasSourceConsistencyBaselineArtifact: Bool
    let sourceVerifyArtifact: VerifySnapshot?
    let sourceStatusArtifact: StatusSnapshot?
    let sourceReportArtifact: ReportSnapshot?
    let sourceHealthArtifact: HealthSnapshot?
    let evaluationArtifact: AcceptanceEvaluationArtifact?
    let hasTargetImportTranscriptArtifact: Bool
    let targetServePhaseArtifacts: [AcceptanceServePhaseArtifact]
    let operatorEvidence: [String: AcceptanceBundleSnapshot.OperatorEvidence]
    let issues: [AcceptanceBundleArtifactIssue]

    var status: String { meta.status }
    var schema: String { meta.schema }
    var isCollected: Bool { meta.isCollected }
    var discoveryTrustedUnexpected: Bool { meta.discoveryTrustedUnexpected }
    var sourceAppAudit: AcceptanceBundleSnapshot.AppAuditEvidence? { meta.sourceAppAudit }
    var targetAppAudit: AcceptanceBundleSnapshot.AppAuditEvidence? { meta.targetAppAudit }
    var sourceNotarization: AcceptanceBundleSnapshot.NotarizationEvidence? { meta.sourceNotarization }
    var targetNotarization: AcceptanceBundleSnapshot.NotarizationEvidence? { meta.targetNotarization }
    var sourceMachineFacts: AcceptanceBundleSnapshot.MachineFactsEvidence? { meta.sourceMachineFacts }
    var targetMachineFacts: AcceptanceBundleSnapshot.MachineFactsEvidence? { meta.targetMachineFacts }
    var bundleHandoffs: [AcceptanceBundleSnapshot.BundleHandoffEvidence] { meta.evidence.bundle_handoffs ?? [] }
    var persistedEvaluationRequiresOperatorEvidence: Bool { meta.evidence.evaluation?.require_operator_evidence == true }
    var collectionMode: String { meta.collection?.mode?.trimmingCharacters(in: .whitespacesAndNewlines) ?? "" }
    var machineCount: Int { meta.collection?.machine_count ?? 0 }
    var missingOperatorEvidence: [String] {
        AcceptanceManualEvidenceRequirement.strictTwoMachineRequirements.compactMap { requirement in
            hasRecordedManualEvidence(requirement) ? nil : requirement.kind
        }
    }

    private func shellQuote(_ value: String) -> String {
        "'" + value.replacingOccurrences(of: "'", with: "'\"'\"'") + "'"
    }

    private func normalizedForWorkflowDisplay(_ action: WorkflowAction) -> WorkflowAction {
        WorkflowAction(
            machine: action.machine,
            step: action.step,
            action: action.action,
            commands: action.commands.map {
                $0.replacingOccurrences(
                    of: shellQuote(bundleRootPath),
                    with: shellQuote("<bundle-root>")
                )
            }
        )
    }

    private var requireOperatorEvidenceCompatibleCollection: Bool {
        collectionMode == "two_machine" && machineCount >= 2
    }

    private var currentPairingReceiptID: String? {
        AcceptanceControlPlaneID.safeRawSegment(
            sourcePairArtifact?.pairing_receipt_id
            ?? meta.evidence.source_pair?.pairing_receipt_id
        )
    }

    private var currentSessionID: String? {
        AcceptanceControlPlaneID.safeRawSegment(
            sourceTransferArtifact?.session_id
            ?? meta.evidence.source_transfer?.session_id
        )
    }

    func hasCurrentEvaluationEvidence(requireOperatorEvidence: Bool) -> Bool {
        guard status == "evidence_collected",
              let evaluationArtifact,
              evaluationArtifact.schema == "supermover.acceptance.two_machine.v1",
              evaluationArtifact.status == "evidence_collected"
        else {
            return false
        }

        let recordedRequireOperatorEvidence = evaluationArtifact.require_operator_evidence
            ?? meta.evidence.evaluation?.require_operator_evidence
            ?? false
        if requireOperatorEvidence && !recordedRequireOperatorEvidence {
            return false
        }

        if let currentPairingReceiptID,
           evaluationArtifact.pairing_receipt_id != currentPairingReceiptID {
            return false
        }
        if let currentSessionID,
           evaluationArtifact.session_id != currentSessionID {
            return false
        }
        if let expectedTargetRoot = meta.evidence.evaluation?.target_root
            .trimmingCharacters(in: .whitespacesAndNewlines),
           !expectedTargetRoot.isEmpty,
           evaluationArtifact.target_root != expectedTargetRoot {
            return false
        }

        return true
    }

    func hasCurrentEvaluationPassState(requireOperatorEvidence: Bool) -> Bool {
        hasCurrentEvaluationEvidence(requireOperatorEvidence: requireOperatorEvidence)
            && workflowSummary(requireOperatorEvidence: requireOperatorEvidence).nextActions.isEmpty
    }

    private var workflowSummaryFallbackArtifact: AcceptanceBundleSnapshot.WorkflowSummaryArtifact? {
        let summary = meta.evidence.workflow_summary
        guard summary?.default != nil || summary?.require_operator_evidence != nil else {
            return nil
        }
        return AcceptanceBundleSnapshot.WorkflowSummaryArtifact(
            schema: "supermover.acceptance.workflow_summary.v1",
            default: summary?.default,
            require_operator_evidence: summary?.require_operator_evidence
        )
    }

    private func hasCleanText(_ value: String?) -> Bool {
        guard let value else {
            return false
        }
        return !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    private func cleanText(_ value: String?) -> String {
        value?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
    }

    private func normalizedPath(_ value: String?) -> String {
        let path = cleanText(value)
        guard !path.isEmpty else {
            return ""
        }
        return URL(fileURLWithPath: path).standardizedFileURL.path
    }

    private var currentSourceTransferTargetRoot: String? {
        let roots = [
            normalizedPath(sourceVerifyArtifact?.target_root),
            normalizedPath(sourceReportArtifact?.target_root),
            normalizedPath(sourceStatusArtifact?.target_root),
            normalizedPath(sourceHealthArtifact?.target_root),
        ]
        guard let first = roots.first else {
            return nil
        }
        guard roots.allSatisfy({ !$0.isEmpty }) else {
            return nil
        }
        return roots.allSatisfy { $0 == first } ? first : nil
    }

    private var currentEvaluationTargetRoot: String? {
        let artifactRoot = normalizedPath(evaluationArtifact?.target_root)
        if !artifactRoot.isEmpty {
            return artifactRoot
        }
        let metaRoot = normalizedPath(meta.evidence.evaluation?.target_root)
        return metaRoot.isEmpty ? nil : metaRoot
    }

    private var sourceTransferTargetRootMatchesCurrentEvaluation: Bool {
        guard let evaluationTargetRoot = currentEvaluationTargetRoot else {
            return true
        }
        guard let sourceTransferTargetRoot = currentSourceTransferTargetRoot else {
            return false
        }
        return sourceTransferTargetRoot == evaluationTargetRoot
    }

    private var sourcePairEvidenceReady: Bool {
        guard let sourcePairArtifact else {
            return false
        }
        return hasCleanText(sourcePairArtifact.pairing_receipt_id)
            && currentPairingReceiptID != nil
            && sourcePairMatchesCurrentTargetReady(sourcePairArtifact)
            && hasCleanText(sourcePairArtifact.receipt_path)
            && hasSourcePairReceiptArtifact
            && hasValidSourcePairReceiptArtifact
            && hasSourcePairTranscriptArtifact
    }

    private var sourceBrowseEvidenceReady: Bool {
        guard let sourceBrowse = meta.sourceBrowse,
              hasCleanText(sourceBrowse.output),
              let sourceBrowseSnapshot
        else {
            return false
        }
        return sourceBrowseSnapshot.trusted == false
    }

    private var sourceBrowseFinalEvaluateReady: Bool {
        guard meta.sourceBrowse != nil else {
            return true
        }
        return sourceBrowseEvidenceReady
    }

    private var targetAdvertiseEvidenceReady: Bool {
        guard let targetAdvertise = meta.targetAdvertise,
              hasCleanText(targetAdvertise.output),
              let targetAdvertiseSnapshot
        else {
            return false
        }
        return targetAdvertiseSnapshot.status == "advertised" && targetAdvertiseSnapshot.trusted == false
    }

    private var targetAdvertiseFinalEvaluateReady: Bool {
        guard meta.targetAdvertise != nil else {
            return true
        }
        return targetAdvertiseEvidenceReady
    }

    var targetReadyEvidenceReady: Bool {
        guard let targetReady = meta.evidence.target_ready,
              let targetReadyArtifact,
              hasCleanText(targetReady.address),
              hasCleanText(targetReady.mode),
              hasCleanText(targetReadyArtifact.address),
              hasCleanText(targetReadyArtifact.mode),
              targetReady.address == targetReadyArtifact.address,
              targetReady.mode == targetReadyArtifact.mode
        else {
            return false
        }

        let artifactVerification = targetReadyArtifact.verification_code?
            .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if targetReady.mode == "pairing" || targetReady.mode == "pairing-only" {
            return hasCleanText(targetReady.verification_code)
                && targetReady.verification_code == artifactVerification
        }
        return artifactVerification.isEmpty || targetReady.verification_code == artifactVerification
    }

    private var targetImportEvidenceReady: Bool {
        guard sourcePairEvidenceReady else {
            return false
        }
        guard let targetImport = meta.evidence.target_import,
              targetImport.pairing_receipt_id == currentPairingReceiptID,
              hasCleanText(targetImport.adopted)
        else {
            return false
        }
        return hasTargetImportTranscriptArtifact
    }

    private var targetImportFinalEvaluateReady: Bool {
        guard sourcePairEvidenceReady else {
            return false
        }
        guard let targetImport = meta.evidence.target_import,
              targetImport.pairing_receipt_id == currentPairingReceiptID,
              hasCleanText(targetImport.adopted)
        else {
            return false
        }
        return hasTargetImportTranscriptArtifact
    }

    private var sourceTransferEvidenceReady: Bool {
        guard let sourceTransferArtifact,
              hasCleanText(sourceTransferArtifact.session_id),
              let currentSessionID,
              currentPairingReceiptID != nil,
              sourceTransferMatchesCurrentTargetReady(sourceTransferArtifact),
              hasSourceNetworkPushTranscriptArtifact,
              let sourceVerifyArtifact,
              currentSourceTransferTargetRoot != nil,
              sourceTransferTargetRootMatchesCurrentEvaluation,
              sourceVerifyArtifact.summary.files_verified >= 1,
              sourceVerifyArtifact.summary.error_findings == 0,
              sourceVerifyArtifact.summary.artifact_problems == 0,
              let sourceReportArtifact,
              sourceReportArtifact.pairing.status == "paired_receipt_valid",
              let reportPairingReceiptID = sourceReportArtifact.pairing.receipt_id,
              reportPairingReceiptID == currentPairingReceiptID,
              sourceStatusEvidenceReady(sessionID: currentSessionID),
              sourceHealthEvidenceReady(sessionID: currentSessionID),
              hasDecodedSourceConsistencyArtifact,
              hasSourceConsistencyBaselineArtifact,
              let sourceConsistencyArtifact,
              sourceConsistencyArtifact.schema == "supermover.acceptance.current_source_consistency.v1",
              sourceConsistencyArtifact.status == "pass",
              sourceConsistencyArtifact.mode == "current_source_verified",
              sourceConsistencyArtifact.session_id == currentSessionID
        else {
            return false
        }
        return true
    }

    private func sourcePairMatchesCurrentTargetReady(_ sourcePair: AcceptanceSourcePairArtifact) -> Bool {
        guard targetReadyEvidenceReady else {
            return true
        }
        return sourcePair.target_address == cleanText(targetReadyArtifact?.address)
    }

    private func sourceTransferMatchesCurrentTargetReady(_ sourceTransfer: AcceptanceSourceTransferArtifact) -> Bool {
        guard targetReadyEvidenceReady else {
            return true
        }
        guard let targetReadyArtifact,
              targetReadyReceiverTransferReady(targetReadyArtifact),
              sourceTransfer.target_address == cleanText(targetReadyArtifact.address),
              sourceTransfer.target_mode == cleanText(targetReadyArtifact.mode) else {
            return false
        }
        let expectedReceiver = cleanText(targetReadyArtifact.receiver_address)
        return sourceTransfer.receiver_address == expectedReceiver
    }

    private func targetReadyReceiverTransferReady(_ readiness: ServeReadinessSnapshot) -> Bool {
        cleanText(readiness.receiver_address).isEmpty == false
            && readiness.receiver_routes == true
            && readiness.push_network == true
            && readiness.transfer == true
    }

    private func sourceStatusEvidenceReady(sessionID: String) -> Bool {
        guard let status = sourceStatusArtifact else {
            return false
        }
        return hasCleanText(status.profile_id)
            && hasCleanText(status.target_id)
            && hasCleanText(status.target_root)
            && hasCleanText(status.overall.status)
            && hasCleanText(status.overall.target_status)
            && status.latest_session.id == sessionID
            && hasCleanText(status.latest_session.completeness_status)
            && status.latest_session.files_expected >= 0
            && status.latest_session.files_verified >= 0
            && status.latest_session.verification_errors >= 0
            && status.counts.artifact_problems >= 0
            && status.counts.network_transfers >= 0
    }

    private func sourceHealthEvidenceReady(sessionID: String) -> Bool {
        guard let health = sourceHealthArtifact else {
            return false
        }
        return hasCleanText(health.target_root)
            && health.summary.incomplete_sessions >= 0
            && health.summary.invalid_records >= 0
            && health.summary.artifact_problems >= 0
            && health.summary.target_drifts >= 0
            && health.summary.network_transfers >= 0
            && health.network_transfers?.contains(where: { transfer in
                transfer.session_id == sessionID && hasCleanText(transfer.status)
            }) == true
    }

    func workflowSummary(requireOperatorEvidence: Bool) -> (steps: [WorkflowStep], nextActions: [WorkflowAction]) {
        let bundlePlaceholder = "<bundle-root>"
        let sourceProfile = meta.roles["source_transfer"]?.profile
            ?? meta.roles["source_pair"]?.profile
            ?? "<source-profile>"
        let targetProfile = meta.roles["target"]?.profile
            ?? meta.roles["target_import"]?.profile
            ?? meta.roles["target_advertise"]?.profile
            ?? "<target-profile>"
        let targetRoot = meta.evidence.evaluation?.target_root ?? "<target-root>"

        func commands(for step: String) -> [String] {
            switch step {
            case "target_serve_phase_1":
                return ["sh macos/script/acceptance-two-machine.sh target-serve --profile \(shellQuote(targetProfile)) --bundle-root \(shellQuote(bundlePlaceholder))"]
            case "source_browse":
                return ["sh macos/script/acceptance-two-machine.sh source-browse --bundle-root \(shellQuote(bundlePlaceholder))"]
            case "target_advertise":
                return ["sh macos/script/acceptance-two-machine.sh target-advertise --profile \(shellQuote(targetProfile)) --bundle-root \(shellQuote(bundlePlaceholder))"]
            case "source_pair":
                return ["sh macos/script/acceptance-two-machine.sh source-pair --profile \(shellQuote(sourceProfile)) --bundle-root \(shellQuote(bundlePlaceholder))"]
            case "target_import":
                return ["sh macos/script/acceptance-two-machine.sh target-import --profile \(shellQuote(targetProfile)) --bundle-root \(shellQuote(bundlePlaceholder))"]
            case "source_transfer":
                return ["sh macos/script/acceptance-two-machine.sh source-transfer --profile \(shellQuote(sourceProfile)) --bundle-root \(shellQuote(bundlePlaceholder)) --session '<session-id>'"]
            case "operator_local_network":
                return ["sh macos/script/acceptance-two-machine.sh record-operator-evidence --bundle-root \(shellQuote(bundlePlaceholder)) --kind local_network --status pass --detail 'accepted prompt on target'"]
            case "operator_firewall":
                return ["sh macos/script/acceptance-two-machine.sh record-operator-evidence --bundle-root \(shellQuote(bundlePlaceholder)) --kind firewall --status pass --detail 'allowed firewall access on target'"]
            case "operator_pairing_confirmation":
                return ["sh macos/script/acceptance-two-machine.sh record-operator-evidence --bundle-root \(shellQuote(bundlePlaceholder)) --kind pairing_confirmation --status pass --detail 'physical pairing code confirmed on both devices'"]
            case "source_packaging_evidence":
                return ["sh macos/script/acceptance-two-machine.sh record-packaging-evidence --bundle-root \(shellQuote(bundlePlaceholder)) --machine source --app '<source-app>'"]
            case "target_packaging_evidence":
                return ["sh macos/script/acceptance-two-machine.sh record-packaging-evidence --bundle-root \(shellQuote(bundlePlaceholder)) --machine target --app '<target-app>'"]
            case "bundle_handoff":
                return [
                    "sh macos/script/acceptance-two-machine.sh pack-bundle --bundle-root \(shellQuote(bundlePlaceholder)) --archive '<bundle.tgz>'",
                    "sh macos/script/acceptance-two-machine.sh unpack-bundle --archive '<bundle.tgz>' --manifest '<bundle.manifest.json>' --bundle-root '<incoming-bundle>'",
                    "sh macos/script/acceptance-two-machine.sh merge-bundle --bundle-root \(shellQuote(bundlePlaceholder)) --incoming-bundle-root '<incoming-bundle>'",
                ]
            case "evaluate":
                let operatorFlag = requireOperatorEvidence ? " --require-operator-evidence" : ""
                return ["sh macos/script/acceptance-two-machine.sh evaluate --bundle-root \(shellQuote(bundlePlaceholder)) --target-root \(shellQuote(targetRoot)) --source-profile \(shellQuote(sourceProfile))\(operatorFlag)"]
            default:
                return []
            }
        }

        func installedAppCollectionCorrectionActions(
            for proof: AcceptanceInstalledAppCollectionProofSummary
        ) -> [WorkflowAction]? {
            if proof.blockedReason == .invalidCollection {
                return [
                    WorkflowAction(
                        machine: "either",
                        step: "review_collection",
                        action: "correct collection.mode=two_machine and collection.machine_count>=2 before installed-app evaluation",
                        commands: []
                    )
                ]
            }

            if proof.blockedReason == .contradictoryVerifiedBundleHandoffs {
                return [
                    WorkflowAction(
                        machine: "either",
                        step: "review_bundle_handoff",
                        action: "remove contradictory installed-app bundle_handoff evidence before final evaluate",
                        commands: []
                    )
                ]
            }

            if proof.requiresMachineIdentityCorrection {
                return [
                    WorkflowAction(
                        machine: "target",
                        step: "target_serve_phase_1",
                        action: "rewrite target role and target.machine.json evidence from the installed app before handoff proof",
                        commands: commands(for: "target_serve_phase_1")
                    ),
                    WorkflowAction(
                        machine: "source",
                        step: "source_pair",
                        action: "rewrite source role and source.machine.json evidence from the installed app before handoff proof",
                        commands: commands(for: "source_pair")
                    ),
                ]
            }

            if proof.requiresBundleHandoffProof {
                return [
                    WorkflowAction(
                        machine: "either",
                        step: "bundle_handoff",
                        action: "pack/unpack/merge bundle evidence across distinct machines before installed-app evaluation",
                        commands: commands(for: "bundle_handoff")
                    )
                ]
            }

            return nil
        }

        var steps: [WorkflowStep] = [
            WorkflowStep(
                id: "target_serve_phase_1",
                machine: "target",
                description: "start target pairing serve",
                done: targetReadyEvidenceReady
            ),
            WorkflowStep(
                id: "source_browse",
                machine: "source",
                description: "collect source browse evidence",
                done: sourceBrowseEvidenceReady
            ),
            WorkflowStep(
                id: "target_advertise",
                machine: "target",
                description: "collect target advertise evidence",
                done: targetAdvertiseEvidenceReady
            ),
            WorkflowStep(
                id: "source_pair",
                machine: "source",
                description: "export source pairing receipt",
                done: sourcePairEvidenceReady
            ),
            WorkflowStep(
                id: "target_import",
                machine: "target",
                description: "import pairing receipt on target",
                done: targetImportEvidenceReady
            ),
            WorkflowStep(
                id: "source_transfer",
                machine: "source",
                description: "run source mTLS transfer and consistency proof",
                done: sourceTransferEvidenceReady
            ),
        ]

        let manualEvidenceRequirements = manualEvidenceRequirements(
            requireOperatorEvidence: requireOperatorEvidence
        )
        steps.append(
            contentsOf: manualEvidenceRequirements.map { requirement in
                WorkflowStep(
                    id: requirement.step,
                    machine: requirement.machine,
                    description: requirement.description,
                    done: hasRecordedManualEvidence(requirement)
                )
            }
        )

        let phaseActions = steps.compactMap { step -> WorkflowAction? in
            guard !step.done else { return nil }
            return WorkflowAction(machine: step.machine, step: step.id, action: step.description, commands: commands(for: step.id))
        }
        let evaluateGateActions = [
            (!targetReadyEvidenceReady) ? WorkflowAction(
                machine: "target",
                step: "target_serve_phase_1",
                action: "start target pairing serve",
                commands: commands(for: "target_serve_phase_1")
            ) : nil,
            (!sourceBrowseFinalEvaluateReady) ? WorkflowAction(
                machine: "source",
                step: "source_browse",
                action: "collect source browse evidence",
                commands: commands(for: "source_browse")
            ) : nil,
            (!targetAdvertiseFinalEvaluateReady) ? WorkflowAction(
                machine: "target",
                step: "target_advertise",
                action: "collect target advertise evidence",
                commands: commands(for: "target_advertise")
            ) : nil,
            (!sourcePairEvidenceReady) ? WorkflowAction(
                machine: "source",
                step: "source_pair",
                action: "export source pairing receipt",
                commands: commands(for: "source_pair")
            ) : nil,
            (!targetImportFinalEvaluateReady) ? WorkflowAction(
                machine: "target",
                step: "target_import",
                action: "import pairing receipt on target",
                commands: commands(for: "target_import")
            ) : nil,
            (!sourceTransferEvidenceReady) ? WorkflowAction(
                machine: "source",
                step: "source_transfer",
                action: "run source mTLS transfer and consistency proof",
                commands: commands(for: "source_transfer")
            ) : nil,
        ].compactMap { $0 } + manualEvidenceRequirements.compactMap { requirement in
            guard !hasRecordedManualEvidence(requirement) else {
                return nil
            }
            return WorkflowAction(
                machine: requirement.machine,
                step: requirement.step,
                action: requirement.description,
                commands: commands(for: requirement.step)
            )
        }

        let collectionProof = installedAppCollectionProof
        let releaseEvidence = installedAppReleaseEvidence
        let hasCurrentEvaluation = hasCurrentEvaluationEvidence(
            requireOperatorEvidence: requireOperatorEvidence
        )
        let nextActions: [WorkflowAction]
        if requireOperatorEvidence {
            if !releaseEvidence.ok {
                nextActions = releaseEvidence.missingMachines.map { machine in
                    WorkflowAction(
                        machine: machine,
                        step: "\(machine)_packaging_evidence",
                        action: "record \(machine) release packaging evidence from the installed app before final evaluate",
                        commands: commands(for: "\(machine)_packaging_evidence")
                    )
                }
            } else if !collectionProof.ok {
                nextActions = installedAppCollectionCorrectionActions(for: collectionProof) ?? [
                    WorkflowAction(
                        machine: "either",
                        step: "review_collection",
                        action: "review installed-app collection proof before final evaluate",
                        commands: []
                    )
                ]
            } else if !hasCurrentEvaluation {
                if evaluateGateActions.isEmpty {
                    nextActions = [
                        WorkflowAction(
                            machine: "either",
                            step: "evaluate",
                            action: "run acceptance evaluate against the merged bundle",
                            commands: commands(for: "evaluate")
                        )
                    ]
                } else {
                    nextActions = evaluateGateActions
                }
            } else {
                nextActions = phaseActions
            }
        } else {
            if phaseActions.isEmpty && !hasCurrentEvaluation {
                nextActions = [
                    WorkflowAction(
                        machine: "either",
                        step: "evaluate",
                        action: "run acceptance evaluate against the merged bundle",
                        commands: commands(for: "evaluate")
                    )
                ]
            } else {
                nextActions = phaseActions
            }
        }

        if let artifact = workflowSummaryArtifact ?? workflowSummaryFallbackArtifact {
            let selected = requireOperatorEvidence ? artifact.require_operator_evidence : artifact.default
            let strictArtifactIsCurrent = workflowSummaryArtifactIsCurrent(
                selected,
                requireOperatorEvidence: requireOperatorEvidence,
                currentSteps: steps,
                currentNextActions: nextActions
            )
            if let selected, strictArtifactIsCurrent {
                let artifactSteps = selected.steps.map {
                    WorkflowStep(id: $0.id, machine: $0.machine, description: $0.description, done: $0.done)
                }
                let artifactNextActions = selected.next_actions.map {
                    normalizedForWorkflowDisplay(
                        WorkflowAction(machine: $0.machine, step: $0.step, action: $0.action, commands: $0.commands)
                    )
                }
                return (steps: artifactSteps, nextActions: artifactNextActions)
            }
        }

        return (steps, nextActions)
    }

    private func workflowSummaryArtifactIsCurrent(
        _ artifact: AcceptanceBundleSnapshot.WorkflowStatusArtifact?,
        requireOperatorEvidence: Bool,
        currentSteps: [WorkflowStep],
        currentNextActions: [WorkflowAction]
    ) -> Bool {
        guard let artifact else {
            return false
        }
        guard !requireOperatorEvidence || requireOperatorEvidenceCompatibleCollection else {
            return false
        }

        let collectionProof = installedAppCollectionProof
        let releaseEvidence = installedAppReleaseEvidence
        let currentWorkflowOK = hasCurrentEvaluationEvidence(
            requireOperatorEvidence: requireOperatorEvidence
        ) && currentNextActions.isEmpty

        return collectionProof.matches(workflowStatus: artifact)
            && artifact.bundle_status == status
            && artifact.ok == currentWorkflowOK
            && artifact.source_app_audit_ready == releaseEvidence.source.appAuditReady
            && artifact.target_app_audit_ready == releaseEvidence.target.appAuditReady
            && artifact.source_notarization_ready == releaseEvidence.source.notarizationReady
            && artifact.target_notarization_ready == releaseEvidence.target.notarizationReady
            && artifact.installed_app_release_evidence_ok == releaseEvidence.ok
            && artifact.installed_app_release_evidence_failures == releaseEvidence.failures
            && artifact.steps.map({
                WorkflowStep(id: $0.id, machine: $0.machine, description: $0.description, done: $0.done)
            }) == currentSteps
            && artifact.next_actions.map({
                normalizedForWorkflowDisplay(
                    WorkflowAction(machine: $0.machine, step: $0.step, action: $0.action, commands: $0.commands)
                )
            }) == currentNextActions.map(normalizedForWorkflowDisplay)
    }
}
