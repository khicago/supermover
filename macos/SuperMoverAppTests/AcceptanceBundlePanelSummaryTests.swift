import XCTest
@testable import SuperMoverApp

final class AcceptanceBundlePanelSummaryTests: XCTestCase {
    func testPanelSummaryKeepsCollectedBundleInReviewUntilCurrentEvaluationArtifactExists() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-bundle-panel-summary-missing-evaluate"
        )
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-bundle-panel-summary-missing-evaluate-target"
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            status: "evidence_collected"
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/tmp/current-source/SuperMover.app"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/tmp/current-target/SuperMover.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(snapshot.hasCurrentEvaluationEvidence(requireOperatorEvidence: true))

        let summary = snapshot.panelSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.bundle.value, "review")
        XCTAssertEqual(summary.bundle.tone, .caution)
        XCTAssertEqual(summary.release.value, "ready")
        XCTAssertEqual(summary.proof.value, "proved")
        XCTAssertEqual(summary.operator.value, "pass")
    }

    func testPanelSummaryDowngradesCollectedBundleWhenWorkflowStillHasNextActions() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-bundle-panel-summary-stale-status"
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            status: "evidence_collected"
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/tmp/current-source/SuperMover.app"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/tmp/current-target/SuperMover.app"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let workflow = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertFalse(workflow.nextActions.isEmpty)

        let summary = snapshot.panelSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.bundle.value, "review")
        XCTAssertEqual(summary.bundle.tone, .caution)
        XCTAssertEqual(summary.release.value, "ready")
        XCTAssertEqual(summary.release.tone, .positive)
        XCTAssertEqual(summary.proof.value, "proved")
        XCTAssertEqual(summary.proof.tone, .positive)
        XCTAssertEqual(summary.operator.value, "pass")
        XCTAssertEqual(summary.operator.tone, .positive)
    }

    func testPanelSummaryOperatorMetricUsesCurrentOptionalLaneInsteadOfStoredEvaluationFlag() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-bundle-panel-summary-stale-operator-lane"
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "evidence_collected",
          "roles": {},
          "evidence": {
            "evaluation": {
              "pairing_receipt_id": "pair-1",
              "session_id": "session-1",
              "target_root": "/tmp/target-root",
              "output": "evaluation.json",
              "require_operator_evidence": true
            }
          }
        }
        """.write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "evidence_collected",
          "pairing_receipt_id": "pair-1",
          "session_id": "session-1",
          "target_root": "/tmp/target-root",
          "require_operator_evidence": true
        }
        """.write(
            to: bundleRoot.appendingPathComponent("evaluation.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let summary = snapshot.panelSummary(requireOperatorEvidence: false)

        XCTAssertEqual(summary.operator.value, "n/a")
        XCTAssertEqual(summary.operator.tone, .informative)
    }
}
