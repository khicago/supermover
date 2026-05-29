import Foundation

struct AcceptanceBundleSnapshot: Decodable, Equatable {
    struct CollectionRecord: Decodable, Equatable {
        let mode: String?
        let machine_count: Int?
    }

    struct RoleRecord: Decodable, Equatable {
        let profile: String
        let status: String
        let machine_id: String?
        let machine_label: String?
    }

    struct AppAuditEvidence: Decodable, Equatable {
        let collected_by: String
        let output: String
        let exit_code: Int
        let status: String
        let readiness: String
        let pass_ready: Bool
        let blocking_checks: Int
    }

    typealias ProvenanceArtifact = SuperMoverBundledProvenanceManifest

    struct AppAuditArtifact: Decodable, Equatable {
        struct Summary: Decodable, Equatable {
            let pass_ready: Bool?
            let blocking_checks: Int?
        }

        struct Provenance: Decodable, Equatable {
            let path: String?
            let manifest: AcceptanceBundleSnapshot.ProvenanceArtifact?
        }

        let schema: String?
        let status: String?
        let readiness: String?
        let app_path: String?
        let provenance: Provenance?
        let summary: Summary?
    }

    struct NotarizationEvidence: Decodable, Equatable {
        let collected_by: String
        let output: String
        let notary_log: String?
        let status: String
        let audit_status: String?
        let audit_readiness: String?
        let audit_pass_ready: Bool?
    }

    struct NotarizationArtifact: Decodable, Equatable {
        struct Submission: Decodable, Equatable {
            let id: String?
            let status: String?
            let message: String?
            let path: String?
        }

        struct NotaryLog: Decodable, Equatable {
            let path: String?
        }

        struct Audit: Decodable, Equatable {
            let path: String?
            let status: String?
            let readiness: String?
            let pass_ready: Bool?
            let blocking_checks: Int?
        }

        struct Failure: Decodable, Equatable {
            let id: String?
            let summary: String?
            let detail: String?
        }

        let schema: String?
        let checked_at: String?
        let status: String
        let app_path: String?
        let work_dir: String?
        let auth_mode: String?
        let archive_path: String?
        let submission: Submission?
        let notary_log: NotaryLog?
        let audit: Audit?
        let failure: Failure?
    }

    struct MachineFactsEvidence: Decodable, Equatable {
        let output: String
        let machine_id: String
        let machine_label: String?
    }

    struct MachineFactsArtifact: Decodable, Equatable {
        let schema: String?
        let machine_id: String
        let machine_label: String?
    }

    struct CLISurfaceEvidence: Decodable, Equatable {
        let command: String
        let expected: String
        let output: String
        let status: String
    }

    struct DiscoveryEvidence: Decodable, Equatable {
        let output: String
        let trusted: Bool
    }

    struct TargetReadyEvidence: Decodable, Equatable {
        let address: String
        let verification_code: String
        let mode: String
    }

    struct TargetServePhase: Decodable, Equatable, Identifiable {
        var id: Int { phase }
        let phase: Int
        let ready: String
    }

    struct SourcePairEvidence: Decodable, Equatable {
        let pairing_receipt_id: String
        let receipt_path: String
        let target_address: String
        let output: String?
        let pair: String?
    }

    struct TargetImportEvidence: Decodable, Equatable {
        let pairing_receipt_id: String
        let adopted: String?
    }

    struct SourceTransferEvidence: Decodable, Equatable {
        let session_id: String
        let receiver_address: String
        let output: String?
        let verify: String?
        let status: String?
        let report: String?
        let health: String?
        let push: String?
    }

    struct SourceConsistencyEvidence: Decodable, Equatable {
        let schema: String?
        let output: String?
        let baseline: String?
        let status: String
        let mode: String
        let session_id: String?
        let entry_count: Int?
        let mismatch_count: Int?
        let detail: String?
    }

    struct EvaluationEvidence: Decodable, Equatable {
        let pairing_receipt_id: String
        let session_id: String
        let target_root: String
        let output: String?
        let require_operator_evidence: Bool?
    }

    struct OperatorEvidence: Decodable, Equatable {
        let status: String
        let detail: String
        let artifact: String?
        let machine_id: String?
        let machine_label: String?
    }

    struct BundleHandoffEvidence: Decodable, Equatable {
        let archive: String
        let manifest: String
        let sha256: String
        let meta: String
        let verified: Bool
        let exporting_machine_id: String?
        let exporting_machine_label: String?
        let importing_machine_id: String?
        let importing_machine_label: String?
    }

    struct WorkflowStatusArtifact: Decodable, Equatable {
        struct WorkflowStep: Decodable, Equatable {
            let id: String
            let machine: String
            let description: String
            let done: Bool
        }

        struct WorkflowAction: Decodable, Equatable {
            let machine: String
            let step: String
            let action: String
            let commands: [String]
        }

        let schema: String?
        let bundle_status: String?
        let collection_mode: String?
        let machine_count: Int?
        let verified_bundle_handoffs: Int?
        let verified_cross_machine_bundle_handoffs: Int?
        let matches_recorded_machine_pair: Bool?
        let has_installed_app_machine_pair_proof: Bool?
        let installed_app_proof_ok: Bool?
        let installed_app_proof_failures: [String]?
        let ok: Bool?
        let failures: [String]?
        let blocked_reason: String?
        let missing_requirements: [String]?
        let primary_failure: String?
        let failure_message: String?
        let requires_machine_identity_correction: Bool?
        let requires_bundle_handoff_proof: Bool?
        let final_evaluation_collection_detail: String?
        let final_evaluation_machine_facts_detail: String?
        let final_evaluation_bundle_handoff_detail: String?
        let source_app_audit_ready: Bool?
        let target_app_audit_ready: Bool?
        let source_notarization_ready: Bool?
        let target_notarization_ready: Bool?
        let installed_app_release_evidence_ok: Bool?
        let installed_app_release_evidence_failures: [String]?
        let role_machine_ids: [String: String?]?
        let machine_fact_ids: [String: String?]?
        let machine_fact_artifact_ids: [String: String?]?
        let machine_facts_consistent: Bool?
        let steps: [WorkflowStep]
        let next_actions: [WorkflowAction]
    }

    struct WorkflowSummaryEvidence: Decodable, Equatable {
        let output: String?
        let `default`: WorkflowStatusArtifact?
        let require_operator_evidence: WorkflowStatusArtifact?
    }

    struct WorkflowSummaryArtifact: Decodable, Equatable {
        let schema: String?
        let `default`: WorkflowStatusArtifact?
        let require_operator_evidence: WorkflowStatusArtifact?
    }

    struct Evidence: Decodable, Equatable {
        let app_audit: [String: AppAuditEvidence]?
        let notarization: [String: NotarizationEvidence]?
        let machine_facts: [String: MachineFactsEvidence]?
        let bundle_handoffs: [BundleHandoffEvidence]?
        let workflow_summary: WorkflowSummaryEvidence?
        let cli_surface: [String: CLISurfaceEvidence]?
        let target_ready: TargetReadyEvidence?
        let target_serve_phases: [TargetServePhase]?
        let discovery: [String: DiscoveryEvidence]?
        let source_pair: SourcePairEvidence?
        let target_import: TargetImportEvidence?
        let source_transfer: SourceTransferEvidence?
        let source_consistency: SourceConsistencyEvidence?
        let evaluation: EvaluationEvidence?
        let operatorEvidence: [String: OperatorEvidence]?

        enum CodingKeys: String, CodingKey {
            case app_audit
            case notarization
            case machine_facts
            case bundle_handoffs
            case workflow_summary
            case cli_surface
            case target_ready
            case target_serve_phases
            case discovery
            case source_pair
            case target_import
            case source_transfer
            case source_consistency
            case evaluation
            case operatorEvidence = "operator"
        }
    }

    let schema: String
    let status: String
    let collection: CollectionRecord?
    let roles: [String: RoleRecord]
    let evidence: Evidence

    var isCollected: Bool {
        status == "evidence_collected"
    }

    var discoveryTrustedUnexpected: Bool {
        (evidence.discovery ?? [:]).values.contains(where: \.trusted)
    }

    var hasBlockedAppAudit: Bool {
        (evidence.app_audit ?? [:]).values.contains(where: {
            $0.status != "pass" || $0.readiness != "distribution_ready" || $0.pass_ready == false
        })
    }

    var sourceAppAudit: AppAuditEvidence? {
        evidence.app_audit?["source"]
    }

    var targetAppAudit: AppAuditEvidence? {
        evidence.app_audit?["target"]
    }

    var sourceNotarization: NotarizationEvidence? {
        evidence.notarization?["source"]
    }

    var targetNotarization: NotarizationEvidence? {
        evidence.notarization?["target"]
    }

    var sourceMachineFacts: MachineFactsEvidence? {
        evidence.machine_facts?["source"]
    }

    var targetMachineFacts: MachineFactsEvidence? {
        evidence.machine_facts?["target"]
    }

    var sourceBrowse: DiscoveryEvidence? {
        evidence.discovery?["source_browse"]
    }

    var targetAdvertise: DiscoveryEvidence? {
        evidence.discovery?["target_advertise"]
    }

    var operatorEvidence: [String: OperatorEvidence] {
        evidence.operatorEvidence ?? [:]
    }
}
