import Foundation

struct AcceptanceInstalledAppLaunchGate {
    enum BundleState: Equatable {
        case notConfigured
        case invalid(path: String, detail: String)
        case loaded(collectionMode: String)
    }

    struct Input: Equatable {
        let machine: String
        let bundleState: BundleState
        let cliProvenance: CLIProvenance
        let installedAppCollectionProof: AcceptanceInstalledAppCollectionProofSummary?
        let releaseEvidenceMachine: AcceptanceInstalledAppReleaseEvidenceSummary.Machine?
        let hasCurrentEvaluationPassState: Bool

        init(
            machine: String,
            bundleState: BundleState,
            cliProvenance: CLIProvenance,
            installedAppCollectionProof: AcceptanceInstalledAppCollectionProofSummary?,
            releaseEvidenceMachine: AcceptanceInstalledAppReleaseEvidenceSummary.Machine?,
            hasCurrentEvaluationPassState: Bool
        ) {
            self.machine = machine
            self.bundleState = bundleState
            self.cliProvenance = cliProvenance
            self.installedAppCollectionProof = installedAppCollectionProof
            self.releaseEvidenceMachine = releaseEvidenceMachine
            self.hasCurrentEvaluationPassState = hasCurrentEvaluationPassState
        }
    }

    enum Verdict: Equatable {
        case invalidAcceptanceBundle(machine: String, detail: String)
        case requiresPackagedApp(machine: String, mode: CLIProvenance.Mode, readiness: String)
        case missingPackagingAudit(machine: String)
        case packagingAuditNotReady(machine: String, readiness: String)
        case missingNotarization(machine: String)
        case notarizationNotReady(machine: String, status: String)
        case installedAppProofIncomplete(machine: String, detail: String)
        case installedAppProofBlocked(machine: String, detail: String)
        case finalEvaluationPending(machine: String, detail: String)
        case ready(machine: String, auditReadiness: String)

        var machine: String {
            switch self {
            case let .invalidAcceptanceBundle(machine, _),
                 let .requiresPackagedApp(machine, _, _),
                 let .missingPackagingAudit(machine),
                 let .packagingAuditNotReady(machine, _),
                 let .missingNotarization(machine),
                 let .notarizationNotReady(machine, _),
                 let .installedAppProofIncomplete(machine, _),
                 let .installedAppProofBlocked(machine, _),
                 let .finalEvaluationPending(machine, _),
                 let .ready(machine, _):
                return machine
            }
        }

        var preflightError: String? {
            switch self {
            case let .invalidAcceptanceBundle(_, detail):
                return "Real two-machine installed-app acceptance tasks require a readable acceptance bundle before phase execution. Current acceptance bundle could not be loaded: \(detail)"
            case let .requiresPackagedApp(_, mode, readiness):
                return "Real two-machine installed-app acceptance tasks require a packaged app. Current CLI mode is \(mode.title) (\(readiness))."
            case let .missingPackagingAudit(machine):
                return "Real two-machine installed-app acceptance tasks require ready \(machine) packaging audit before phase execution. Current \(machine) audit readiness is unknown."
            case let .packagingAuditNotReady(machine, readiness):
                return "Real two-machine installed-app acceptance tasks require ready \(machine) packaging audit before phase execution. Current \(machine) audit readiness is \(readiness)."
            case let .missingNotarization(machine):
                return "Real two-machine installed-app acceptance tasks require release-ready \(machine) notarization evidence before phase execution. Current acceptance bundle has no \(machine).notarization.json."
            case let .notarizationNotReady(machine, status):
                return "Real two-machine installed-app acceptance tasks require release-ready \(machine) notarization evidence before phase execution. Current \(machine) notarization status is \(status)."
            case let .installedAppProofBlocked(_, detail):
                return detail
            case .installedAppProofIncomplete, .finalEvaluationPending, .ready:
                return nil
            }
        }

        func preview(localNotarizationBlockingDetail: String? = nil) -> AcceptanceInstalledAppLaunchPreview {
            switch self {
            case .invalidAcceptanceBundle, .requiresPackagedApp, .installedAppProofBlocked:
                break
            default:
                if let localNotarizationBlockingDetail {
                    return AcceptanceInstalledAppLaunchPreview(
                        machine: machine,
                        state: .blocked,
                        detail: localNotarizationBlockingDetail
                    )
                }
            }
            switch self {
            case let .invalidAcceptanceBundle(machine, detail):
                return AcceptanceInstalledAppLaunchPreview(
                    machine: machine,
                    state: .blocked,
                    detail: "Real two-machine installed-app acceptance tasks require a readable acceptance bundle. Current acceptance bundle could not be loaded: \(detail)"
                )
            case let .requiresPackagedApp(machine, mode, readiness):
                return AcceptanceInstalledAppLaunchPreview(
                    machine: machine,
                    state: .blocked,
                    detail: "Real two-machine installed-app acceptance tasks require a packaged app. Current CLI mode is \(mode.title) (\(readiness))."
                )
            case let .missingPackagingAudit(machine):
                return AcceptanceInstalledAppLaunchPreview(
                    machine: machine,
                    state: .review,
                    detail: "No loaded \(machine) packaging audit is present in the current two-machine acceptance bundle. Launch will record fresh packaging evidence and require ready audit before phase execution."
                )
            case let .packagingAuditNotReady(machine, readiness):
                return AcceptanceInstalledAppLaunchPreview(
                    machine: machine,
                    state: .review,
                    detail: "Loaded \(machine) packaging audit is \(readiness). Launch will record fresh packaging evidence and still require ready audit before phase execution."
                )
            case let .missingNotarization(machine):
                return AcceptanceInstalledAppLaunchPreview(
                    machine: machine,
                    state: .review,
                    detail: "No loaded \(machine) notarization evidence is present in the current two-machine acceptance bundle. Launch will record fresh packaging evidence and require release-ready notarization before phase execution."
                )
            case let .notarizationNotReady(machine, status):
                return AcceptanceInstalledAppLaunchPreview(
                    machine: machine,
                    state: .review,
                    detail: "Loaded \(machine) notarization evidence is \(status). Launch will record fresh packaging evidence and still require release-ready notarization before phase execution."
                )
            case let .installedAppProofIncomplete(machine, detail):
                return AcceptanceInstalledAppLaunchPreview(
                    machine: machine,
                    state: .review,
                    detail: detail
                )
            case let .installedAppProofBlocked(machine, detail):
                return AcceptanceInstalledAppLaunchPreview(
                    machine: machine,
                    state: .blocked,
                    detail: detail
                )
            case let .finalEvaluationPending(machine, detail):
                return AcceptanceInstalledAppLaunchPreview(
                    machine: machine,
                    state: .review,
                    detail: detail
                )
            case let .ready(machine, auditReadiness):
                return AcceptanceInstalledAppLaunchPreview(
                    machine: machine,
                    state: .pass,
                    detail: "Loaded \(machine) packaging audit is \(auditReadiness), notarization evidence is accepted, and distinct-machine installed-app proof matches the current bundle."
                )
            }
        }
    }

    private enum InstalledAppCollectionState: Equatable {
        case incomplete(detail: String)
        case blocked(detail: String)
        case proved
    }

    static func evaluate(_ input: Input) -> Verdict? {
        switch input.bundleState {
        case .notConfigured:
            return nil
        case let .invalid(_, detail):
            return .invalidAcceptanceBundle(machine: input.machine, detail: detail)
        case let .loaded(collectionMode):
            guard collectionMode == "two_machine" else {
                return nil
            }
        }

        guard input.cliProvenance.mode == .bundled else {
            return .requiresPackagedApp(
                machine: input.machine,
                mode: input.cliProvenance.mode,
                readiness: input.cliProvenance.readiness
            )
        }
        guard let releaseEvidence = input.releaseEvidenceMachine else {
            return .installedAppProofBlocked(
                machine: input.machine,
                detail: "Real two-machine installed-app acceptance tasks require a loaded acceptance bundle with current release-evidence details before phase execution."
            )
        }
        guard releaseEvidence.appAuditPresent else {
            return .missingPackagingAudit(machine: input.machine)
        }
        guard releaseEvidence.appAuditReady else {
            return .packagingAuditNotReady(
                machine: input.machine,
                readiness: releaseEvidence.appAuditReadiness ?? "unknown"
            )
        }
        guard releaseEvidence.notarizationPresent else {
            return .missingNotarization(machine: input.machine)
        }
        guard releaseEvidence.notarizationReady else {
            return .notarizationNotReady(
                machine: input.machine,
                status: releaseEvidence.notarizationStatus ?? "unknown"
            )
        }

        switch installedAppCollectionState(for: input) {
        case let .incomplete(detail):
            return .installedAppProofIncomplete(machine: input.machine, detail: detail)
        case let .blocked(detail):
            return .installedAppProofBlocked(machine: input.machine, detail: detail)
        case .proved:
            guard input.hasCurrentEvaluationPassState else {
                return .finalEvaluationPending(
                    machine: input.machine,
                    detail: "Loaded \(input.machine) packaging audit is accepted, notarization evidence is accepted, and distinct-machine installed-app proof matches the current bundle, but the bundle does not currently satisfy strict bundle-local final-evaluation truth for this merged evidence. Launch can still collect phase evidence; final evaluate must record current strict bundle truth before this advisory can pass."
                )
            }
            return .ready(
                machine: input.machine,
                auditReadiness: releaseEvidence.appAuditReadiness ?? "unknown"
            )
        }
    }

    private static func installedAppCollectionState(for input: Input) -> InstalledAppCollectionState {
        guard let proof = input.installedAppCollectionProof else {
            return .blocked(
                detail: "Real two-machine installed-app acceptance tasks require a loaded acceptance bundle with current installed-app proof details before phase execution."
            )
        }
        if proof.ok {
            return .proved
        }
        if let blockedDetail = blockedInstalledAppCollectionDetail(for: proof) {
            return .blocked(detail: blockedDetail)
        }
        if proof.requiresBundleHandoffProof {
            return .blocked(detail: blockedBundleHandoffProofDetail(for: proof))
        }

        let reasons = incompleteInstalledAppCollectionReasons(for: proof)
        let detail: String
        if reasons.isEmpty {
            detail = "Loaded \(input.machine) packaging audit is accepted, but distinct-machine installed-app proof is not complete yet. Launch can still collect phase evidence; final evaluate remains blocked until machine facts and a verified cross-machine bundle handoff prove the recorded source/target pair."
        } else {
            detail = "Loaded \(input.machine) packaging audit is accepted, but distinct-machine installed-app proof is not complete yet: \(reasons.joined(separator: "; ")). Launch can still collect phase evidence; final evaluate remains blocked until machine facts and a verified cross-machine bundle handoff prove the recorded source/target pair."
        }
        return .incomplete(detail: detail)
    }

    private static func blockedInstalledAppCollectionDetail(
        for proof: AcceptanceInstalledAppCollectionProofSummary
    ) -> String? {
        switch proof.blockedReason {
        case .invalidCollection:
            return "Real two-machine installed-app acceptance tasks require collection.mode=two_machine with machine_count >= 2. Current acceptance bundle does not satisfy that contract."
        case .sameRoleMachineIDs:
            return "Current acceptance bundle records the same machine_id for source and target roles. Distinct-machine installed-app tasks remain blocked until the bundle is corrected."
        case .invalidMachineFactArtifactSchema:
            return "Current acceptance bundle machine fact artifacts are malformed or use the wrong schema. Distinct-machine installed-app tasks remain blocked until the bundle is corrected."
        case .sameMachineFactArtifactIDs:
            return "Current acceptance bundle machine fact artifacts collapse source and target onto the same machine_id. Distinct-machine installed-app tasks remain blocked until the bundle is corrected."
        case .machineFactsInconsistent:
            return "Current acceptance bundle machine fact metadata does not agree with source.machine.json and target.machine.json. Distinct-machine installed-app tasks remain blocked until the bundle is corrected."
        case .contradictoryVerifiedBundleHandoffs:
            let reason = proof.failureMessage ?? "bundle_handoffs do not prove the recorded source/target machine pair"
            return "Current acceptance bundle already contains contradictory archive handoff evidence: \(reason). Distinct-machine installed-app tasks remain blocked until the bundle is corrected."
        case .conflictingRoleAndMachineFacts:
            return "Current acceptance bundle records conflicting role machine ids and machine fact evidence. Distinct-machine installed-app tasks remain blocked until the bundle is corrected."
        case nil:
            return nil
        }
    }

    private static func blockedBundleHandoffProofDetail(
        for proof: AcceptanceInstalledAppCollectionProofSummary
    ) -> String {
        let reason =
            proof.finalEvaluationBundleHandoffDetail
            ?? proof.failureMessage
            ?? "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"
        return "Current acceptance bundle is missing distinct-machine archive handoff proof: \(reason). Distinct-machine installed-app tasks remain blocked until the bundle_handoff pack/unpack/merge procedure is completed for the recorded machine pair."
    }

    private static func incompleteInstalledAppCollectionReasons(
        for proof: AcceptanceInstalledAppCollectionProofSummary
    ) -> [String] {
        var reasons: [String] = []

        for requirement in proof.missingRequirements {
            switch requirement {
            case .roleMachineIDs:
                reasons.append("role machine ids are not recorded yet")
            case .machineFactArtifacts:
                reasons.append("source.machine.json / target.machine.json are not recorded yet")
            case .machineFactArtifactIDs:
                reasons.append("machine fact artifact machine ids are missing")
            case .machineFactMetadata:
                reasons.append("machine fact metadata is not fully recorded yet")
            case .verifiedBundleHandoffs:
                reasons.append("verified bundle_handoffs are not recorded yet")
            }
        }

        return reasons
    }
}
