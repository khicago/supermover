import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppCollectionProofParityScriptTests: XCTestCase {
    func testWorkflowStatusReturnsMachineIdentityCorrectionActionsWhenRoleMachineIDsCollapse() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-machine-identity-correction-workflow"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: matchedVerifiedBundleHandoffsJSON
        )
        try rewriteRoleMachineIDs(
            bundleRoot: bundleRoot,
            sourceMachineID: "same-machine",
            targetMachineID: "same-machine"
        )

        let result = try runTwoMachineScript(
            arguments: [
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ]
        )

        let status = try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(result.stdout.utf8)) as? [String: Any]
        )
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 0)
        XCTAssertEqual(status["matches_recorded_machine_pair"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, false)
        XCTAssertEqual(status["installed_app_blocked_reason"] as? String, "same_role_machine_ids")
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, true)
        XCTAssertEqual(status["requires_bundle_handoff_proof"] as? Bool, false)
        XCTAssertEqual(status["blocked_reason"] as? String, "same_role_machine_ids")
        XCTAssertEqual(status["primary_failure"] as? String, "same_role_machine_ids")
        XCTAssertEqual(status["failure_message"] as? String, sameRoleMachineIDsDetail)
        XCTAssertEqual(status["final_evaluation_collection_detail"] as? String, sameRoleMachineIDsDetail)
        XCTAssertNil(status["final_evaluation_machine_facts_detail"] as? String)
        XCTAssertEqual(status["final_evaluation_bundle_handoff_detail"] as? String, unmatchedBundleHandoffDetail)
        XCTAssertEqual(
            status["installed_app_proof_failures"] as? [String],
            ["same_role_machine_ids", "handoff_does_not_match_recorded_machine_pair"]
        )

        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 2)
        XCTAssertEqual(nextActions.map { $0["machine"] as? String }, ["target", "source"])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["target_serve_phase_1", "source_pair"])
        XCTAssertEqual((nextActions.first?["commands"] as? [String])?.count, 1)
        XCTAssertEqual((nextActions.last?["commands"] as? [String])?.count, 1)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.installedAppCollectionProof.blockedReason, .sameRoleMachineIDs)

        let workflowSummary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.map(\.machine), ["target", "source"])
        XCTAssertEqual(workflowSummary.nextActions.map(\.step), ["target_serve_phase_1", "source_pair"])

        let workflowArtifactData = try Data(contentsOf: bundleRoot.appendingPathComponent("workflow.summary.json"))
        let workflowArtifact = try XCTUnwrap(
            JSONSerialization.jsonObject(with: workflowArtifactData) as? [String: Any]
        )
        XCTAssertEqual(workflowArtifact["schema"] as? String, "supermover.acceptance.workflow_summary.v1")
        let operatorSummary = try XCTUnwrap(workflowArtifact["require_operator_evidence"] as? [String: Any])
        XCTAssertEqual(operatorSummary["installed_app_blocked_reason"] as? String, "same_role_machine_ids")
        XCTAssertEqual(operatorSummary["final_evaluation_collection_detail"] as? String, sameRoleMachineIDsDetail)
        XCTAssertNil(operatorSummary["final_evaluation_machine_facts_detail"] as? String)
        XCTAssertEqual(operatorSummary["final_evaluation_bundle_handoff_detail"] as? String, unmatchedBundleHandoffDetail)
        let artifactNextActions = try XCTUnwrap(operatorSummary["next_actions"] as? [[String: Any]])
        XCTAssertEqual(artifactNextActions.map { $0["machine"] as? String }, ["target", "source"])
        XCTAssertEqual(artifactNextActions.map { $0["step"] as? String }, ["target_serve_phase_1", "source_pair"])

        let metaData = try Data(contentsOf: bundleRoot.appendingPathComponent("meta.json"))
        let meta = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        let evidence = try XCTUnwrap(meta["evidence"] as? [String: Any])
        let persistedWorkflowSummary = try XCTUnwrap(evidence["workflow_summary"] as? [String: Any])
        let persistedOperatorSummary = try XCTUnwrap(
            persistedWorkflowSummary["require_operator_evidence"] as? [String: Any]
        )
        XCTAssertEqual(persistedOperatorSummary["installed_app_blocked_reason"] as? String, "same_role_machine_ids")
        XCTAssertEqual(persistedOperatorSummary["final_evaluation_collection_detail"] as? String, sameRoleMachineIDsDetail)
        XCTAssertNil(persistedOperatorSummary["final_evaluation_machine_facts_detail"] as? String)
        XCTAssertEqual(persistedOperatorSummary["final_evaluation_bundle_handoff_detail"] as? String, unmatchedBundleHandoffDetail)
        let persistedNextActions = try XCTUnwrap(
            persistedOperatorSummary["next_actions"] as? [[String: Any]]
        )
        XCTAssertEqual(persistedNextActions.map { $0["machine"] as? String }, ["target", "source"])
        XCTAssertEqual(persistedNextActions.map { $0["step"] as? String }, ["target_serve_phase_1", "source_pair"])
    }

    func testWorkflowStatusReturnsMachineIdentityCorrectionActionsWhenMachineFactsConflictWithArtifacts() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-machine-facts-conflict-workflow"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: matchedVerifiedBundleHandoffsJSON
        )
        try rewriteMachineFactEvidence(
            bundleRoot: bundleRoot,
            sourceMachineID: "other-source-machine",
            targetMachineID: "other-target-machine"
        )

        let result = try runTwoMachineScript(
            arguments: [
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ]
        )

        let status = try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(result.stdout.utf8)) as? [String: Any]
        )
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 0)
        XCTAssertEqual(status["matches_recorded_machine_pair"] as? Bool, false)
        XCTAssertEqual(status["machine_facts_consistent"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, false)
        XCTAssertEqual(status["installed_app_blocked_reason"] as? String, "machine_facts_inconsistent")
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, true)
        XCTAssertEqual(status["requires_bundle_handoff_proof"] as? Bool, false)
        XCTAssertEqual(status["final_evaluation_collection_detail"] as? String, nil)
        XCTAssertEqual(status["final_evaluation_machine_facts_detail"] as? String, roleMachineFactsMismatchDetail)
        XCTAssertEqual(status["final_evaluation_bundle_handoff_detail"] as? String, unmatchedBundleHandoffDetail)
        XCTAssertEqual(
            status["installed_app_proof_failures"] as? [String],
            ["handoff_does_not_match_recorded_machine_pair"]
        )

        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 2)
        XCTAssertEqual(nextActions.map { $0["machine"] as? String }, ["target", "source"])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["target_serve_phase_1", "source_pair"])

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.installedAppCollectionProof.blockedReason, .machineFactsInconsistent)

        let workflowSummary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.map(\.machine), ["target", "source"])
        XCTAssertEqual(workflowSummary.nextActions.map(\.step), ["target_serve_phase_1", "source_pair"])
    }

    func testWorkflowStatusReturnsMachineIdentityCorrectionActionsWhenCanonicalSourceMachineFactsArtifactIsSymlinkedOutsideBundle() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-machine-facts-symlink-workflow"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: matchedVerifiedBundleHandoffsJSON
        )

        let outsideMachineFactsURL = workDir.appendingPathComponent("outside-source.machine.json")
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: outsideMachineFactsURL, atomically: true, encoding: .utf8)
        try FileManager.default.removeItem(at: bundleRoot.appendingPathComponent("source.machine.json"))
        do {
            try FileManager.default.createSymbolicLink(
                at: bundleRoot.appendingPathComponent("source.machine.json"),
                withDestinationURL: outsideMachineFactsURL
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        let result = try runTwoMachineScript(
            arguments: [
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ]
        )

        let status = try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(result.stdout.utf8)) as? [String: Any]
        )
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, false)
        XCTAssertEqual(status["installed_app_blocked_reason"] as? String, "machine_facts_inconsistent")
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, true)
        XCTAssertEqual(status["requires_bundle_handoff_proof"] as? Bool, false)

        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["machine"] as? String }, ["target", "source"])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["target_serve_phase_1", "source_pair"])

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertNil(snapshot.sourceMachineFactsArtifact)
        XCTAssertEqual(snapshot.installedAppCollectionProof.blockedReason, .machineFactsInconsistent)
    }

    func testWorkflowStatusReturnsMachineIdentityCorrectionActionsWhenRolesConflictWithMachineFactsArtifacts() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-role-machine-facts-conflict-workflow"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: matchedVerifiedBundleHandoffsJSON
        )
        try rewriteRoleMachineIDs(
            bundleRoot: bundleRoot,
            sourceMachineID: "other-source-machine",
            targetMachineID: "other-target-machine"
        )

        let result = try runTwoMachineScript(
            arguments: [
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ]
        )

        let status = try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(result.stdout.utf8)) as? [String: Any]
        )
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 0)
        XCTAssertEqual(status["matches_recorded_machine_pair"] as? Bool, false)
        XCTAssertEqual(status["machine_facts_consistent"] as? Bool, true)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, false)
        XCTAssertEqual(status["installed_app_blocked_reason"] as? String, "conflicting_role_and_machine_facts")
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, true)
        XCTAssertEqual(status["requires_bundle_handoff_proof"] as? Bool, false)
        XCTAssertNil(status["final_evaluation_collection_detail"] as? String)
        XCTAssertEqual(status["final_evaluation_machine_facts_detail"] as? String, roleMachineFactsMismatchDetail)
        XCTAssertEqual(status["final_evaluation_bundle_handoff_detail"] as? String, unmatchedBundleHandoffDetail)
        XCTAssertEqual(
            status["installed_app_proof_failures"] as? [String],
            ["handoff_does_not_match_recorded_machine_pair"]
        )

        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 2)
        XCTAssertEqual(nextActions.map { $0["machine"] as? String }, ["target", "source"])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["target_serve_phase_1", "source_pair"])

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.installedAppCollectionProof.blockedReason, .conflictingRoleAndMachineFacts)
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.finalEvaluationMachineFactsDetail,
            roleMachineFactsMismatchDetail
        )

        let workflowSummary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.map(\.machine), ["target", "source"])
        XCTAssertEqual(workflowSummary.nextActions.map(\.step), ["target_serve_phase_1", "source_pair"])
    }

    func testWorkflowStatusTreatsUnmatchedVerifiedBundleHandoffAsMissingPairProof() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-unmatched-workflow"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: unmatchedVerifiedBundleHandoffsJSON
        )

        let result = try runTwoMachineScript(
            arguments: [
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ]
        )

        let status = try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(result.stdout.utf8)) as? [String: Any]
        )
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 0)
        XCTAssertEqual(status["matches_recorded_machine_pair"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, false)
        XCTAssertNil(status["installed_app_blocked_reason"] as? String)
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, false)
        XCTAssertEqual(status["requires_bundle_handoff_proof"] as? Bool, true)
        XCTAssertNil(status["final_evaluation_collection_detail"] as? String)
        XCTAssertNil(status["final_evaluation_machine_facts_detail"] as? String)
        XCTAssertEqual(status["final_evaluation_bundle_handoff_detail"] as? String, unmatchedBundleHandoffDetail)
        XCTAssertEqual(
            status["installed_app_proof_failures"] as? [String],
            ["handoff_does_not_match_recorded_machine_pair"]
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.failures,
            ["handoff_does_not_match_recorded_machine_pair"]
        )
        XCTAssertNil(snapshot.installedAppCollectionProof.blockedReason)
        XCTAssertTrue(snapshot.installedAppCollectionProof.requiresBundleHandoffProof)

        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 1)
        XCTAssertEqual(nextActions.first?["machine"] as? String, "either")
        XCTAssertEqual(nextActions.first?["step"] as? String, "bundle_handoff")
        XCTAssertEqual((nextActions.first?["commands"] as? [String])?.count, 3)

        let workflowSummary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.count, 1)
        XCTAssertEqual(workflowSummary.nextActions.first?.machine, "either")
        XCTAssertEqual(workflowSummary.nextActions.first?.step, "bundle_handoff")
        XCTAssertEqual(workflowSummary.nextActions.first?.commands.count, 3)
    }

    func testWorkflowStatusRequiresRecordedDiscoveryArtifactsBeforeEvaluate() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-discovery-gate-workflow"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: matchedVerifiedBundleHandoffsJSON
        )
        try FileManager.default.removeItem(at: bundleRoot.appendingPathComponent("source.browse.json"))
        try FileManager.default.removeItem(at: bundleRoot.appendingPathComponent("target.advertise.json"))

        let result = try runTwoMachineScript(
            arguments: [
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ]
        )

        let status = try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(result.stdout.utf8)) as? [String: Any]
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_browse", "target_advertise"])

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let workflowSummary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.map(\.step), ["source_browse", "target_advertise"])
    }

    func testWorkflowStatusRequiresOperatorEvidenceBeforeEvaluate() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-operator-gate-workflow"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: matchedVerifiedBundleHandoffsJSON
        )
        try rewriteOperatorEvidenceStatuses(
            bundleRoot: bundleRoot,
            localNetwork: "review",
            firewall: "review",
            pairingConfirmation: "review"
        )

        let result = try runTwoMachineScript(
            arguments: [
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ]
        )

        let status = try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(result.stdout.utf8)) as? [String: Any]
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(
            nextActions.map { $0["step"] as? String },
            ["operator_local_network", "operator_firewall", "operator_pairing_confirmation"]
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let workflowSummary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(
            workflowSummary.nextActions.map(\.step),
            ["operator_local_network", "operator_firewall", "operator_pairing_confirmation"]
        )
    }

    func testWorkflowStatusFailsClosedWhenVerifiedBundleHandoffsContainContradictoryMachinePairs() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-contradictory-workflow"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: contradictoryVerifiedBundleHandoffsJSON
        )

        let result = try runTwoMachineScript(
            arguments: [
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ]
        )

        let status = try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(result.stdout.utf8)) as? [String: Any]
        )
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 2)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["matches_recorded_machine_pair"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, false)
        XCTAssertEqual(status["installed_app_blocked_reason"] as? String, "contradictory_verified_bundle_handoffs")
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, false)
        XCTAssertEqual(status["requires_bundle_handoff_proof"] as? Bool, false)
        XCTAssertNil(status["final_evaluation_collection_detail"] as? String)
        XCTAssertNil(status["final_evaluation_machine_facts_detail"] as? String)
        XCTAssertEqual(status["final_evaluation_bundle_handoff_detail"] as? String, contradictoryBundleHandoffDetail)
        XCTAssertEqual(
            status["installed_app_proof_failures"] as? [String],
            ["contradictory_verified_bundle_handoffs"]
        )

        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 1)
        XCTAssertEqual(nextActions.first?["machine"] as? String, "either")
        XCTAssertEqual(nextActions.first?["step"] as? String, "review_bundle_handoff")
        XCTAssertEqual(nextActions.first?["commands"] as? [String], [])

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let workflowSummary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.count, 1)
        XCTAssertEqual(workflowSummary.nextActions.first?.machine, "either")
        XCTAssertEqual(workflowSummary.nextActions.first?.step, "review_bundle_handoff")
        XCTAssertEqual(workflowSummary.nextActions.first?.commands, [String]())
    }

    func testEvaluateFailsClosedWhenRoleMachineIDsCollapse() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-same-role-machine-ids-evaluate"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: matchedVerifiedBundleHandoffsJSON
        )
        try writeTargetControlPlane(targetRoot: targetRoot)
        try rewriteRoleMachineIDs(
            bundleRoot: bundleRoot,
            sourceMachineID: "same-machine",
            targetMachineID: "same-machine"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.installedAppCollectionProof.blockedReason, .sameRoleMachineIDs)
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.finalEvaluationCollectionDetail,
            sameRoleMachineIDsDetail
        )

        let result = try runTwoMachineScriptAllowFailure(
            arguments: [
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ]
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains(sameRoleMachineIDsDetail),
            "stderr:\n\(result.stderr)"
        )
    }

    func testEvaluateFailsClosedWhenMachineFactsConflictWithArtifacts() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-machine-facts-conflict-evaluate"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: matchedVerifiedBundleHandoffsJSON
        )
        try writeTargetControlPlane(targetRoot: targetRoot)
        try rewriteMachineFactEvidence(
            bundleRoot: bundleRoot,
            sourceMachineID: "other-source-machine",
            targetMachineID: "other-target-machine"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.installedAppCollectionProof.blockedReason, .machineFactsInconsistent)
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.finalEvaluationMachineFactsDetail,
            roleMachineFactsMismatchDetail
        )

        let result = try runTwoMachineScriptAllowFailure(
            arguments: [
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ]
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains(roleMachineFactsMismatchDetail),
            "stderr:\n\(result.stderr)"
        )
    }

    func testEvaluateFailsClosedWhenRolesConflictWithMachineFactsArtifacts() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-role-machine-facts-conflict-evaluate"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: matchedVerifiedBundleHandoffsJSON
        )
        try writeTargetControlPlane(targetRoot: targetRoot)
        try rewriteRoleMachineIDs(
            bundleRoot: bundleRoot,
            sourceMachineID: "other-source-machine",
            targetMachineID: "other-target-machine"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.installedAppCollectionProof.blockedReason, .conflictingRoleAndMachineFacts)
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.finalEvaluationMachineFactsDetail,
            roleMachineFactsMismatchDetail
        )

        let result = try runTwoMachineScriptAllowFailure(
            arguments: [
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ]
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains(roleMachineFactsMismatchDetail),
            "stderr:\n\(result.stderr)"
        )
    }

    func testEvaluateFailsClosedWhenVerifiedBundleHandoffsContainContradictoryMachinePairs() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-contradictory-evaluate"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: contradictoryVerifiedBundleHandoffsJSON
        )
        try writeTargetControlPlane(targetRoot: targetRoot)

        let result = try runTwoMachineScriptAllowFailure(
            arguments: [
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ]
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains(contradictoryBundleHandoffDetail), "stderr:\n\(result.stderr)")
    }

    func testEvaluateFailsClosedWhenRecordedAlternateDiscoveryArtifactIsInvalid() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-alternate-discovery-evaluate"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try writeReadyTwoMachineBundle(
            bundleRoot: bundleRoot,
            bundleHandoffsJSON: matchedVerifiedBundleHandoffsJSON
        )
        try writeTargetControlPlane(targetRoot: targetRoot)
        try rewriteDiscoveryOutputs(
            bundleRoot: bundleRoot,
            sourceBrowseOutput: "evidence/source.browse.selected.json",
            targetAdvertiseOutput: "evidence/target.advertise.selected.json"
        )
        try FileManager.default.removeItem(at: bundleRoot.appendingPathComponent("source.browse.json"))
        try FileManager.default.removeItem(at: bundleRoot.appendingPathComponent("target.advertise.json"))
        try FileManager.default.createDirectory(
            at: bundleRoot.appendingPathComponent("evidence", isDirectory: true),
            withIntermediateDirectories: true
        )
        try """
        {
          "source": "browse",
          "listen": "0.0.0.0:39394",
          "candidate_count": 1,
          "invalid_packets": 0,
          "trusted": true,
          "candidates": []
        }
        """.write(
            to: bundleRoot.appendingPathComponent("evidence/source.browse.selected.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "status": "advertised",
          "listen": "0.0.0.0:39394",
          "destination": "255.255.255.255:39394",
          "service_type": "_supermover._udp",
          "protocol_version": "1",
          "ephemeral_nonce": "nonce-1",
          "capability_flags": ["pairing"],
          "trusted": false,
          "duration": "5s",
          "interval": "1s"
        }
        """.write(
            to: bundleRoot.appendingPathComponent("evidence/target.advertise.selected.json"),
            atomically: true,
            encoding: .utf8
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("evidence/source.browse.selected.json")
            )
        }

        let result = try runTwoMachineScriptAllowFailure(
            arguments: [
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ]
        )

        XCTAssertEqual(result.exitCode, 1)
    }

    private func writeReadyTwoMachineBundle(
        bundleRoot: URL,
        bundleHandoffsJSON: String
    ) throws {
        let targetRoot = bundleRoot
            .deletingLastPathComponent()
            .appendingPathComponent("target-root", isDirectory: true)
        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true,
            bundleHandoffsJSON: bundleHandoffsJSON
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )

        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
    }

    private func writeTargetControlPlane(targetRoot: URL) throws {
        let controlPlane = targetRoot.appendingPathComponent(".supermover", isDirectory: true)
        try FileManager.default.createDirectory(
            at: controlPlane.appendingPathComponent("pairings"),
            withIntermediateDirectories: true
        )
        try FileManager.default.createDirectory(
            at: controlPlane.appendingPathComponent("sessions/session-1"),
            withIntermediateDirectories: true
        )

        try AcceptanceWorkflowFixtures.pairingReceiptJSON().write(
            to: controlPlane.appendingPathComponent("pairings/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceWorkflowFixtures.targetNetworkTransferJSON().write(
            to: controlPlane.appendingPathComponent("sessions/session-1/network-transfer.json"),
            atomically: true,
            encoding: .utf8
        )
    }

    private func runTwoMachineScript(arguments: [String]) throws -> AcceptanceProcessResult {
        let result = try runTwoMachineScriptAllowFailure(arguments: arguments)
        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        return result
    }

    private func runTwoMachineScriptAllowFailure(arguments: [String]) throws -> AcceptanceProcessResult {
        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        return try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path] + arguments,
            environment: [:],
            currentDirectoryURL: repoRoot
        )
    }

    private func rewriteRoleMachineIDs(
        bundleRoot: URL,
        sourceMachineID: String,
        targetMachineID: String
    ) throws {
        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        var document = try XCTUnwrap(
            JSONSerialization.jsonObject(with: data) as? [String: Any]
        )
        var roles = try XCTUnwrap(document["roles"] as? [String: [String: Any]])
        roles["source_pair"]?["machine_id"] = sourceMachineID
        roles["target"]?["machine_id"] = targetMachineID
        document["roles"] = roles

        let rewrittenData = try JSONSerialization.data(
            withJSONObject: document,
            options: [.prettyPrinted, .sortedKeys]
        )
        try rewrittenData.write(to: metaURL)
    }

    private func rewriteMachineFactEvidence(
        bundleRoot: URL,
        sourceMachineID: String,
        targetMachineID: String
    ) throws {
        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        var document = try XCTUnwrap(
            JSONSerialization.jsonObject(with: data) as? [String: Any]
        )
        var evidence = try XCTUnwrap(document["evidence"] as? [String: Any])
        var machineFacts = try XCTUnwrap(evidence["machine_facts"] as? [String: [String: Any]])
        machineFacts["source"]?["machine_id"] = sourceMachineID
        machineFacts["target"]?["machine_id"] = targetMachineID
        evidence["machine_facts"] = machineFacts
        document["evidence"] = evidence

        let rewrittenData = try JSONSerialization.data(
            withJSONObject: document,
            options: [.prettyPrinted, .sortedKeys]
        )
        try rewrittenData.write(to: metaURL)
    }

    private func rewriteDiscoveryOutputs(
        bundleRoot: URL,
        sourceBrowseOutput: String,
        targetAdvertiseOutput: String
    ) throws {
        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        var document = try XCTUnwrap(
            JSONSerialization.jsonObject(with: data) as? [String: Any]
        )
        var evidence = try XCTUnwrap(document["evidence"] as? [String: Any])
        var discovery = try XCTUnwrap(evidence["discovery"] as? [String: [String: Any]])
        discovery["source_browse"]?["output"] = sourceBrowseOutput
        discovery["target_advertise"]?["output"] = targetAdvertiseOutput
        evidence["discovery"] = discovery
        document["evidence"] = evidence

        let rewrittenData = try JSONSerialization.data(
            withJSONObject: document,
            options: [.prettyPrinted, .sortedKeys]
        )
        try rewrittenData.write(to: metaURL)
    }

    private func rewriteOperatorEvidenceStatuses(
        bundleRoot: URL,
        localNetwork: String,
        firewall: String,
        pairingConfirmation: String
    ) throws {
        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        var document = try XCTUnwrap(
            JSONSerialization.jsonObject(with: data) as? [String: Any]
        )
        var evidence = try XCTUnwrap(document["evidence"] as? [String: Any])
        var operatorEvidence = try XCTUnwrap(evidence["operator"] as? [String: [String: Any]])
        operatorEvidence["local_network"]?["status"] = localNetwork
        operatorEvidence["firewall"]?["status"] = firewall
        operatorEvidence["pairing_confirmation"]?["status"] = pairingConfirmation
        evidence["operator"] = operatorEvidence
        document["evidence"] = evidence

        let rewrittenData = try JSONSerialization.data(
            withJSONObject: document,
            options: [.prettyPrinted, .sortedKeys]
        )
        try rewrittenData.write(to: metaURL)
    }

    private let matchedVerifiedBundleHandoffsJSON =
        AcceptanceInstalledAppBundleFixtures.defaultVerifiedBundleHandoffsJSON

    private let contradictoryVerifiedBundleHandoffsJSON = """
    [
      {
        "archive": "bundle.tgz",
        "manifest": "bundle.manifest.json",
        "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
        "meta": "meta.json",
        "verified": true,
        "exporting_machine_id": "source-machine",
        "exporting_machine_label": "source",
        "importing_machine_id": "target-machine",
        "importing_machine_label": "target"
      },
      {
        "archive": "bundle-other.tgz",
        "manifest": "bundle-other.manifest.json",
        "sha256": "2222222222222222222222222222222222222222222222222222222222222222",
        "meta": "meta.json",
        "verified": true,
        "exporting_machine_id": "other-source-machine",
        "exporting_machine_label": "other-source",
        "importing_machine_id": "other-target-machine",
        "importing_machine_label": "other-target"
      }
    ]
    """

    private let unmatchedVerifiedBundleHandoffsJSON = """
    [
      {
        "archive": "bundle-other.tgz",
        "manifest": "bundle-other.manifest.json",
        "sha256": "3333333333333333333333333333333333333333333333333333333333333333",
        "meta": "meta.json",
        "verified": true,
        "exporting_machine_id": "other-source-machine",
        "exporting_machine_label": "other-source",
        "importing_machine_id": "other-target-machine",
        "importing_machine_label": "other-target"
      }
    ]
    """

    private let sameRoleMachineIDsDetail = "source_pair and target share machine_id=same-machine"
    private let roleMachineFactsMismatchDetail =
        "roles.source_pair/target machine_id do not match source.machine.json and target.machine.json"
    private let unmatchedBundleHandoffDetail =
        "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"
    private let contradictoryBundleHandoffDetail =
        "bundle_handoffs contain verified cross-machine archive handoff evidence for machine ids other than the recorded source/target pair"
}
