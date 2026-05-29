import XCTest
@testable import SuperMoverApp

final class AcceptanceEvaluationModeTests: XCTestCase {
    func testTwoMachineSnapshotForcesOperatorEvidenceLane() {
        let mode = AcceptanceEvaluationMode.resolve(
            snapshot: snapshot(collectionMode: "two_machine"),
            draft: AcceptanceEvaluationDraft(requireOperatorEvidence: false)
        )

        XCTAssertTrue(mode.requireOperatorEvidence)
        XCTAssertTrue(mode.isLockedForTwoMachineCollection)
    }

    func testNonTwoMachineSnapshotUsesDraftSelection() {
        let mode = AcceptanceEvaluationMode.resolve(
            snapshot: snapshot(collectionMode: "same_machine"),
            draft: AcceptanceEvaluationDraft(requireOperatorEvidence: false)
        )

        XCTAssertFalse(mode.requireOperatorEvidence)
        XCTAssertFalse(mode.isLockedForTwoMachineCollection)
    }

    private func snapshot(collectionMode: String) -> AcceptanceBundleLoadedSnapshot {
        AcceptanceBundleLoadedSnapshot(
            bundleRootPath: "/tmp/acceptance-bundle",
            meta: AcceptanceBundleSnapshot(
                schema: "supermover.acceptance.two_machine.v1",
                status: "in_progress",
                collection: .init(mode: collectionMode, machine_count: 1),
                roles: [:],
                evidence: .init(
                    app_audit: nil,
                    notarization: nil,
                    machine_facts: nil,
                    bundle_handoffs: nil,
                workflow_summary: nil,
                cli_surface: nil,
                target_ready: nil,
                target_serve_phases: nil,
                discovery: nil,
                source_pair: nil,
                target_import: nil,
                source_transfer: nil,
                source_consistency: nil,
                evaluation: nil,
                    operatorEvidence: nil
                )
            ),
            sourceBrowseSnapshot: nil,
            targetAdvertiseSnapshot: nil,
            targetReadyArtifact: nil,
            sourceProvenanceArtifact: nil,
            targetProvenanceArtifact: nil,
            sourceAppAuditArtifact: nil,
            targetAppAuditArtifact: nil,
            sourceNotarizationArtifact: nil,
            targetNotarizationArtifact: nil,
            sourceMachineFactsArtifact: nil,
            targetMachineFactsArtifact: nil,
            workflowSummaryArtifact: nil,
            sourcePairArtifact: nil,
            hasSourcePairReceiptArtifact: false,
            hasValidSourcePairReceiptArtifact: false,
            hasSourcePairTranscriptArtifact: false,
            sourceTransferArtifact: nil,
            hasSourceNetworkPushTranscriptArtifact: false,
            sourceConsistencyArtifact: nil,
            hasDecodedSourceConsistencyArtifact: false,
            hasSourceConsistencyBaselineArtifact: false,
            sourceVerifyArtifact: nil,
            sourceStatusArtifact: nil,
            sourceReportArtifact: nil,
            sourceHealthArtifact: nil,
            evaluationArtifact: nil,
            hasTargetImportTranscriptArtifact: false,
            targetServePhaseArtifacts: [],
            operatorEvidence: [:],
            issues: []
        )
    }
}
