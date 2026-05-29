import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppCollectionProofTests: XCTestCase {
    func testCollectionProofPrefersCanonicalMachineFactsArtifactsOverAlternateMetaOutputs() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-canonical-machine-facts-preferred"
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try makeBundleMeta(
            sourceMachineFactsOutput: "source.machine.selected.json",
            targetMachineFactsOutput: "target.machine.selected.json"
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.selected.json",
            machineID: "other-source-machine",
            machineLabel: "other-source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "target.machine.selected.json",
            machineID: "other-target-machine",
            machineLabel: "other-target"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let proof = snapshot.installedAppCollectionProof

        guard snapshot.sourceMachineFactsArtifact?.machine_id == "source-machine",
              snapshot.targetMachineFactsArtifact?.machine_id == "target-machine" else {
            return XCTFail(
                "expected canonical source.machine.json / target.machine.json artifacts to override alternate meta-selected outputs"
            )
        }
        XCTAssertTrue(proof.hasInstalledAppMachinePairProof)
        XCTAssertTrue(proof.ok)
    }

    func testCollectionProofRejectsAlternateMetaOutputsThatMaskCanonicalMachineFactsMismatch() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-canonical-machine-facts-mismatch"
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try makeBundleMeta(
            sourceMachineFactsOutput: "source.machine.selected.json",
            targetMachineFactsOutput: "target.machine.selected.json"
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(
            bundleRoot: bundleRoot,
            sourceMachineID: "other-source-machine",
            sourceMachineLabel: "other-source",
            targetMachineID: "other-target-machine",
            targetMachineLabel: "other-target"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.selected.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "target.machine.selected.json",
            machineID: "target-machine",
            machineLabel: "target"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let proof = snapshot.installedAppCollectionProof

        guard snapshot.sourceMachineFactsArtifact?.machine_id == "other-source-machine",
              snapshot.targetMachineFactsArtifact?.machine_id == "other-target-machine" else {
            return XCTFail(
                "expected proof to read canonical source.machine.json / target.machine.json artifacts even when alternate meta-selected outputs look valid"
            )
        }
        XCTAssertFalse(proof.hasInstalledAppMachinePairProof)
        XCTAssertEqual(proof.blockedReason, .machineFactsInconsistent)
    }

    func testFinalEvaluationCollectionDetailPrefersInvalidCollectionBeforeMissingRoleMachineIDs() {
        let proof = makeProof(
            collectionMode: "same_machine",
            machineCount: 1,
            roleMachineIDs: ["source": nil, "target": nil],
            collectionOK: false,
            roleMachineIDsPresent: false,
            roleMachineIDsDistinct: false,
            ok: false,
            failures: ["invalid_collection", "missing_role_machine_ids"]
        )

        XCTAssertEqual(proof.finalEvaluationCollectionDetail, "collection.mode=same_machine")
    }

    func testFinalEvaluationCollectionDetailPrefersMissingSourceRoleMachineID() {
        let proof = makeProof(
            roleMachineIDs: ["source": nil, "target": "target-machine"],
            roleMachineIDsPresent: false,
            roleMachineIDsDistinct: false,
            ok: false,
            failures: ["missing_role_machine_ids"]
        )

        XCTAssertEqual(proof.finalEvaluationCollectionDetail, "missing roles.source_pair.machine_id")
    }

    func testFinalEvaluationCollectionDetailReturnsSharedRoleMachineID() {
        let proof = makeProof(
            roleMachineIDs: ["source": "same-machine", "target": "same-machine"],
            roleMachineIDsDistinct: false,
            ok: false,
            failures: ["same_role_machine_ids"]
        )

        XCTAssertEqual(proof.finalEvaluationCollectionDetail, "source_pair and target share machine_id=same-machine")
    }

    func testEvaluatedCollectionProofUsesFinalEvaluationDetailAsFailureMessageWhenRoleMachineIDIsMissing() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-missing-role-failure-message"
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            sourceMachineID: "",
            targetMachineID: "target-machine"
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.failureMessage,
            "missing roles.source_pair.machine_id"
        )
    }

    func testFinalEvaluationMachineFactsDetailReturnsRoleMismatch() {
        let proof = makeProof(
            machineFactIDs: ["source": "other-source-machine", "target": "other-target-machine"],
            machineFactArtifactIDs: ["source": "other-source-machine", "target": "other-target-machine"],
            hasInstalledAppMachinePairProof: false,
            ok: false,
            failures: ["missing_verified_bundle_handoffs", "handoff_does_not_match_recorded_machine_pair"]
        )

        XCTAssertEqual(
            proof.finalEvaluationMachineFactsDetail,
            "roles.source_pair/target machine_id do not match source.machine.json and target.machine.json"
        )
    }

    func testFinalEvaluationBundleHandoffDetailReturnsContradictoryPairMessage() {
        let proof = makeProof(
            verifiedBundleHandoffs: 2,
            verifiedCrossMachineBundleHandoffs: 1,
            matchesRecordedMachinePair: false,
            ok: false,
            failures: ["contradictory_verified_bundle_handoffs"]
        )

        XCTAssertEqual(
            proof.finalEvaluationBundleHandoffDetail,
            "bundle_handoffs contain verified cross-machine archive handoff evidence for machine ids other than the recorded source/target pair"
        )
    }

    func testFinalEvaluationBundleHandoffDetailReturnsUnmatchedPairMessage() {
        let proof = makeProof(
            verifiedBundleHandoffs: 1,
            verifiedCrossMachineBundleHandoffs: 0,
            matchesRecordedMachinePair: false,
            ok: false,
            failureMessage: "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids",
            failures: ["handoff_does_not_match_recorded_machine_pair"]
        )

        XCTAssertEqual(
            proof.finalEvaluationBundleHandoffDetail,
            "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"
        )
    }

    func testMachineIdentityCorrectionLaunchUsesProofOwnedAllowlistAndReason() {
        let proof = makeProof(
            machineFactIDs: ["source": "other-source-machine", "target": "other-target-machine"],
            machineFactArtifactIDs: ["source": "other-source-machine", "target": "other-target-machine"],
            hasInstalledAppMachinePairProof: false,
            ok: false,
            failures: ["missing_verified_bundle_handoffs", "handoff_does_not_match_recorded_machine_pair"]
        )

        XCTAssertEqual(
            proof.machineIdentityCorrectionLaunch(for: .pair),
            .sourcePair(
                reason: "roles.source_pair/target machine_id do not match source.machine.json and target.machine.json"
            )
        )
        XCTAssertEqual(
            proof.machineIdentityCorrectionLaunch(for: .serve),
            .targetServe(
                reason: "roles.source_pair/target machine_id do not match source.machine.json and target.machine.json"
            )
        )
    }

    func testMachineIdentityCorrectionLaunchAllowsMissingMachineFactCorrection() {
        let proof = makeProof(
            roleMachineIDs: ["source": "source-machine", "target": "target-machine"],
            machineFactIDs: ["source": nil, "target": nil],
            machineFactArtifactIDs: ["source": nil, "target": nil],
            machineFactArtifactsPresent: false,
            machineFactArtifactIDsPresent: false,
            machineFactArtifactIDsDistinct: false,
            verifiedBundleHandoffs: 0,
            verifiedCrossMachineBundleHandoffs: 0,
            matchesRecordedMachinePair: false,
            hasInstalledAppMachinePairProof: false,
            ok: false,
            failures: [
                "missing_machine_fact_artifacts",
                "missing_verified_bundle_handoffs",
                "handoff_does_not_match_recorded_machine_pair",
            ]
        )

        XCTAssertTrue(proof.requiresMachineIdentityCorrection)
        XCTAssertEqual(
            proof.machineIdentityCorrectionLaunch(for: .pair),
            .sourcePair(reason: "missing source/target machine fact artifacts")
        )
        XCTAssertEqual(
            proof.machineIdentityCorrectionLaunch(for: .serve),
            .targetServe(reason: "missing source/target machine fact artifacts")
        )
        XCTAssertNil(proof.machineIdentityCorrectionLaunch(for: .networkPush))
    }

    func testMachineIdentityCorrectionLaunchRejectsNonCorrectableReasonsAndUnrelatedTasks() {
        let invalidCollectionProof = makeProof(
            collectionMode: "same_machine",
            machineCount: 1,
            collectionOK: false,
            roleMachineIDsPresent: false,
            roleMachineIDsDistinct: false,
            ok: false,
            failures: ["invalid_collection"]
        )
        XCTAssertNil(invalidCollectionProof.machineIdentityCorrectionLaunch(for: .pair))

        let contradictoryHandoffProof = makeProof(
            verifiedBundleHandoffs: 2,
            verifiedCrossMachineBundleHandoffs: 1,
            matchesRecordedMachinePair: false,
            ok: false,
            failures: ["contradictory_verified_bundle_handoffs"]
        )
        XCTAssertNil(contradictoryHandoffProof.machineIdentityCorrectionLaunch(for: .pair))

        let sameMachineProof = makeProof(
            roleMachineIDs: ["source": "same-machine", "target": "same-machine"],
            roleMachineIDsDistinct: false,
            ok: false,
            failures: ["same_role_machine_ids"]
        )
        XCTAssertNil(sameMachineProof.machineIdentityCorrectionLaunch(for: .verify))
    }

    func testCollectionProofMatchRejectsWorkflowStatusArtifactThatLacksCurrentVerdictFields() {
        let proof = makeProof(
            roleMachineIDs: ["source": "same-machine", "target": "same-machine"],
            roleMachineIDsDistinct: false,
            verifiedCrossMachineBundleHandoffs: 0,
            matchesRecordedMachinePair: false,
            ok: false,
            failureMessage: "source_pair and target share machine_id=same-machine",
            failures: ["same_role_machine_ids", "handoff_does_not_match_recorded_machine_pair"]
        )
        let artifact = AcceptanceBundleSnapshot.WorkflowStatusArtifact(
            schema: "supermover.acceptance.workflow_status.v1",
            bundle_status: "in_progress",
            collection_mode: "two_machine",
            machine_count: 2,
            verified_bundle_handoffs: 1,
            verified_cross_machine_bundle_handoffs: 0,
            matches_recorded_machine_pair: false,
            has_installed_app_machine_pair_proof: true,
            installed_app_proof_ok: false,
            installed_app_proof_failures: ["same_role_machine_ids", "handoff_does_not_match_recorded_machine_pair"],
            ok: nil,
            failures: nil,
            blocked_reason: nil,
            missing_requirements: nil,
            primary_failure: nil,
            failure_message: nil,
            requires_machine_identity_correction: nil,
            requires_bundle_handoff_proof: nil,
            final_evaluation_collection_detail: nil,
            final_evaluation_machine_facts_detail: nil,
            final_evaluation_bundle_handoff_detail: nil,
            source_app_audit_ready: true,
            target_app_audit_ready: true,
            source_notarization_ready: true,
            target_notarization_ready: true,
            installed_app_release_evidence_ok: true,
            installed_app_release_evidence_failures: [],
            role_machine_ids: ["source": "same-machine", "target": "same-machine"],
            machine_fact_ids: ["source": "source-machine", "target": "target-machine"],
            machine_fact_artifact_ids: ["source": "source-machine", "target": "target-machine"],
            machine_facts_consistent: true,
            steps: [],
            next_actions: []
        )

        XCTAssertFalse(proof.matches(workflowStatus: artifact))
    }

    func testCollectionProofMatchAcceptsWorkflowStatusArtifactWithCurrentVerdictFields() {
        let proof = makeProof(
            roleMachineIDs: ["source": "same-machine", "target": "same-machine"],
            roleMachineIDsDistinct: false,
            verifiedCrossMachineBundleHandoffs: 0,
            matchesRecordedMachinePair: false,
            ok: false,
            failureMessage: "source_pair and target share machine_id=same-machine",
            failures: ["same_role_machine_ids", "handoff_does_not_match_recorded_machine_pair"]
        )
        let artifact = AcceptanceBundleSnapshot.WorkflowStatusArtifact(
            schema: "supermover.acceptance.workflow_status.v1",
            bundle_status: "in_progress",
            collection_mode: "two_machine",
            machine_count: 2,
            verified_bundle_handoffs: 1,
            verified_cross_machine_bundle_handoffs: 0,
            matches_recorded_machine_pair: false,
            has_installed_app_machine_pair_proof: true,
            installed_app_proof_ok: false,
            installed_app_proof_failures: ["same_role_machine_ids", "handoff_does_not_match_recorded_machine_pair"],
            ok: false,
            failures: ["same_role_machine_ids", "handoff_does_not_match_recorded_machine_pair"],
            blocked_reason: "same_role_machine_ids",
            missing_requirements: [],
            primary_failure: "same_role_machine_ids",
            failure_message: "source_pair and target share machine_id=same-machine",
            requires_machine_identity_correction: true,
            requires_bundle_handoff_proof: false,
            final_evaluation_collection_detail: "source_pair and target share machine_id=same-machine",
            final_evaluation_machine_facts_detail: nil,
            final_evaluation_bundle_handoff_detail: "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids",
            source_app_audit_ready: true,
            target_app_audit_ready: true,
            source_notarization_ready: true,
            target_notarization_ready: true,
            installed_app_release_evidence_ok: true,
            installed_app_release_evidence_failures: [],
            role_machine_ids: ["source": "same-machine", "target": "same-machine"],
            machine_fact_ids: ["source": "source-machine", "target": "target-machine"],
            machine_fact_artifact_ids: ["source": "source-machine", "target": "target-machine"],
            machine_facts_consistent: true,
            steps: [],
            next_actions: []
        )

        XCTAssertTrue(proof.matches(workflowStatus: artifact))
    }

    func testCollectionProofMatchIgnoresWorkflowCompletionOKField() {
        let proof = makeProof(
            roleMachineIDs: ["source": "same-machine", "target": "same-machine"],
            roleMachineIDsDistinct: false,
            verifiedCrossMachineBundleHandoffs: 0,
            matchesRecordedMachinePair: false,
            ok: false,
            failureMessage: "source_pair and target share machine_id=same-machine",
            failures: ["same_role_machine_ids", "handoff_does_not_match_recorded_machine_pair"]
        )
        let artifact = AcceptanceBundleSnapshot.WorkflowStatusArtifact(
            schema: "supermover.acceptance.workflow_status.v1",
            bundle_status: "in_progress",
            collection_mode: "two_machine",
            machine_count: 2,
            verified_bundle_handoffs: 1,
            verified_cross_machine_bundle_handoffs: 0,
            matches_recorded_machine_pair: false,
            has_installed_app_machine_pair_proof: true,
            installed_app_proof_ok: false,
            installed_app_proof_failures: ["same_role_machine_ids", "handoff_does_not_match_recorded_machine_pair"],
            ok: true,
            failures: ["same_role_machine_ids", "handoff_does_not_match_recorded_machine_pair"],
            blocked_reason: "same_role_machine_ids",
            missing_requirements: [],
            primary_failure: "same_role_machine_ids",
            failure_message: "source_pair and target share machine_id=same-machine",
            requires_machine_identity_correction: true,
            requires_bundle_handoff_proof: false,
            final_evaluation_collection_detail: "source_pair and target share machine_id=same-machine",
            final_evaluation_machine_facts_detail: nil,
            final_evaluation_bundle_handoff_detail: "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids",
            source_app_audit_ready: true,
            target_app_audit_ready: true,
            source_notarization_ready: true,
            target_notarization_ready: true,
            installed_app_release_evidence_ok: true,
            installed_app_release_evidence_failures: [],
            role_machine_ids: ["source": "same-machine", "target": "same-machine"],
            machine_fact_ids: ["source": "source-machine", "target": "target-machine"],
            machine_fact_artifact_ids: ["source": "source-machine", "target": "target-machine"],
            machine_facts_consistent: true,
            steps: [],
            next_actions: []
        )

        XCTAssertTrue(proof.matches(workflowStatus: artifact))
    }

    func testCollectionProofMatchUsesShellFailureMessageDetailFromEvaluatedProof() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-derived-failure-match"
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            sourceMachineID: "",
            targetMachineID: "target-machine"
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)

        let proof = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot).installedAppCollectionProof
        let artifact = AcceptanceBundleSnapshot.WorkflowStatusArtifact(
            schema: "supermover.acceptance.workflow_status.v1",
            bundle_status: "in_progress",
            collection_mode: proof.collectionMode,
            machine_count: proof.machineCount,
            verified_bundle_handoffs: proof.verifiedBundleHandoffs,
            verified_cross_machine_bundle_handoffs: proof.verifiedCrossMachineBundleHandoffs,
            matches_recorded_machine_pair: proof.matchesRecordedMachinePair,
            has_installed_app_machine_pair_proof: proof.hasInstalledAppMachinePairProof,
            installed_app_proof_ok: proof.ok,
            installed_app_proof_failures: proof.failures,
            ok: false,
            failures: proof.failures,
            blocked_reason: proof.blockedReason?.workflowStatusValue,
            missing_requirements: proof.missingRequirements.map(\.workflowStatusValue),
            primary_failure: proof.primaryFailure,
            failure_message: proof.workflowStatusFailureMessage,
            requires_machine_identity_correction: proof.requiresMachineIdentityCorrection,
            requires_bundle_handoff_proof: proof.requiresBundleHandoffProof,
            final_evaluation_collection_detail: proof.finalEvaluationCollectionDetail,
            final_evaluation_machine_facts_detail: proof.finalEvaluationMachineFactsDetail,
            final_evaluation_bundle_handoff_detail: proof.finalEvaluationBundleHandoffDetail,
            source_app_audit_ready: true,
            target_app_audit_ready: true,
            source_notarization_ready: true,
            target_notarization_ready: true,
            installed_app_release_evidence_ok: true,
            installed_app_release_evidence_failures: [],
            role_machine_ids: proof.roleMachineIDs,
            machine_fact_ids: proof.machineFactIDs,
            machine_fact_artifact_ids: proof.machineFactArtifactIDs,
            machine_facts_consistent: proof.machineFactsConsistent,
            steps: [],
            next_actions: []
        )

        XCTAssertEqual(proof.failureMessage, "missing roles.source_pair.machine_id")
        XCTAssertTrue(proof.matches(workflowStatus: artifact))
    }

    private func makeProof(
        collectionMode: String = "two_machine",
        machineCount: Int = 2,
        roleMachineIDs: [String: String?] = ["source": "source-machine", "target": "target-machine"],
        machineFactIDs: [String: String?] = ["source": "source-machine", "target": "target-machine"],
        machineFactArtifactIDs: [String: String?] = ["source": "source-machine", "target": "target-machine"],
        collectionOK: Bool = true,
        roleMachineIDsPresent: Bool = true,
        roleMachineIDsDistinct: Bool = true,
        machineFactArtifactsPresent: Bool = true,
        machineFactArtifactsSchemaOK: Bool = true,
        machineFactArtifactIDsPresent: Bool = true,
        machineFactArtifactIDsDistinct: Bool = true,
        verifiedBundleHandoffs: Int = 1,
        verifiedCrossMachineBundleHandoffs: Int = 1,
        matchesRecordedMachinePair: Bool = true,
        machineFactsConsistent: Bool = true,
        installedAppMachinePair: AcceptanceInstalledAppCollectionProofSummary.MachinePair? = .init(
            source: "source-machine",
            target: "target-machine"
        ),
        hasInstalledAppMachinePairProof: Bool = true,
        ok: Bool = true,
        failureMessage: String? = nil,
        failures: [String] = []
    ) -> AcceptanceInstalledAppCollectionProofSummary {
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
            verifiedCrossMachineBundleHandoffs: verifiedCrossMachineBundleHandoffs,
            matchesRecordedMachinePair: matchesRecordedMachinePair,
            machineFactsConsistent: machineFactsConsistent,
            installedAppMachinePair: installedAppMachinePair,
            hasInstalledAppMachinePairProof: hasInstalledAppMachinePairProof,
            primaryFailure: failures.first,
            failureMessage: failureMessage ?? failures.first,
            ok: ok,
            failures: failures
        )
    }

    private func makeBundleMeta(
        sourceMachineFactsOutput: String,
        targetMachineFactsOutput: String
    ) -> String {
        AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false
        )
        .replacingOccurrences(
            of: "\"output\": \"source.machine.json\"",
            with: "\"output\": \"\(sourceMachineFactsOutput)\""
        )
        .replacingOccurrences(
            of: "\"output\": \"target.machine.json\"",
            with: "\"output\": \"\(targetMachineFactsOutput)\""
        )
    }

    private func writeMachineFactsArtifact(
        bundleRoot: URL,
        relativePath: String,
        machineID: String,
        machineLabel: String
    ) throws {
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "\(machineID)",
          "machine_label": "\(machineLabel)"
        }
        """.write(
            to: bundleRoot.appendingPathComponent(relativePath),
            atomically: true,
            encoding: .utf8
        )
    }
}
