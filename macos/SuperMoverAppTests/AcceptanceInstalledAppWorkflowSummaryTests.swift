import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppWorkflowSummaryTests: XCTestCase {
    func testWorkflowSummaryRequiresPackagingEvidenceBeforeEvaluateWhenStrictTwoMachineHandoffIsReadyButReleaseEvidenceMissing() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-release-evidence-missing")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence", "target_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source", "target"])
        XCTAssertTrue(summary.nextActions[0].commands[0].contains("record-packaging-evidence"))
        XCTAssertTrue(summary.nextActions[1].commands[0].contains("--machine target"))
    }

    func testWorkflowSummaryTreatsMissingAppAuditArtifactAsMissingReleaseEvidenceEvenWhenMetaStillReferencesIt() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-stale-app-audit-meta")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/tmp/current-target/SuperMover.app"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source"])
    }

    func testWorkflowSummaryAdvancesToEvaluateWhenStrictTwoMachineReleaseEvidenceAndHandoffAreReady() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-release-evidence-ready")
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-release-evidence-ready-target")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false
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
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.count, 1)
        XCTAssertEqual(summary.nextActions.first?.step, "evaluate")
        XCTAssertTrue(summary.nextActions.first?.commands.first?.contains("--require-operator-evidence") == true)
    }

    func testWorkflowSummaryRejectsSourceNotarizationBoundToTargetNotaryLog() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-cross-bound-notary-log")
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-cross-bound-notary-log-target")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false
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
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: "/tmp/current-source/SuperMover.app",
            auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(
                appPath: "/tmp/current-source/SuperMover.app"
            ),
            notaryLogPath: "target.notary-log.json"
        ).write(
            to: bundleRoot.appendingPathComponent("source.notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.source.notarizationReady)
        XCTAssertEqual(
            snapshot.installedAppReleaseEvidence.source.notarizationFailureMessage,
            "source.notarization.json is not release-ready"
        )

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source"])
    }

    func testWorkflowSummaryRejectsSourceNotarizationBoundToTargetAuditPath() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-cross-bound-notary-audit")
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-cross-bound-notary-audit-target")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false
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
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: "/tmp/current-source/SuperMover.app",
            auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(
                appPath: "/tmp/current-target/SuperMover.app"
            ),
            notaryLogPath: "source.notary-log.json"
        ).write(
            to: bundleRoot.appendingPathComponent("source.notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.source.notarizationReady)
        XCTAssertEqual(
            snapshot.installedAppReleaseEvidence.source.notarizationFailureMessage,
            "source.notarization.json does not match source.app-audit.json and source.provenance.json"
        )

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source"])
    }

    func testWorkflowSummaryRejectsSourceNotarizationWithMalformedNotaryLog() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-malformed-notary-log")
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-malformed-notary-log-target")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false
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
        try "not json\n".write(
            to: bundleRoot.appendingPathComponent("source.notary-log.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.source.notarizationReady)
        XCTAssertEqual(
            snapshot.installedAppReleaseEvidence.source.notarizationFailureMessage,
            "source.notarization.json is not release-ready"
        )

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source"])
    }

    func testWorkflowSummaryRejectsSourceNotarizationWithMismatchedNotaryLogJobID() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-mismatched-notary-log")
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-mismatched-notary-log-target")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false
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
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON(
            submissionID: "22222222-2222-2222-2222-222222222222"
        ).write(
            to: bundleRoot.appendingPathComponent("source.notary-log.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.source.notarizationReady)
        XCTAssertEqual(
            snapshot.installedAppReleaseEvidence.source.notarizationFailureMessage,
            "source.notarization.json is not release-ready"
        )

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source"])
    }

    func testWorkflowSummaryRejectsSourceNotarizationWithUnknownAuthMode() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-unknown-notary-auth")
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-unknown-notary-auth-target")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false
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
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: "/tmp/current-source/SuperMover.app",
            auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(
                appPath: "/tmp/current-source/SuperMover.app"
            ),
            authMode: "manual",
            notaryLogPath: "source.notary-log.json"
        ).write(
            to: bundleRoot.appendingPathComponent("source.notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.source.notarizationReady)
        XCTAssertEqual(
            snapshot.installedAppReleaseEvidence.source.notarizationFailureMessage,
            "source.notarization.json is not release-ready"
        )

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source"])
    }

    func testWorkflowSummaryRejectsSourceNotarizationWithFailureRecord() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-notary-failure-record")
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-notary-failure-record-target")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false
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
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: "/tmp/current-source/SuperMover.app",
            auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(
                appPath: "/tmp/current-source/SuperMover.app"
            ),
            notaryLogPath: "source.notary-log.json"
        ).write(
            to: bundleRoot.appendingPathComponent("source.notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        try addNotarizationFailureRecord(to: bundleRoot.appendingPathComponent("source.notarization.json"))
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.source.notarizationReady)
        XCTAssertEqual(
            snapshot.installedAppReleaseEvidence.source.notarizationFailureMessage,
            "source.notarization.json is not release-ready"
        )

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source"])
    }

    func testWorkflowSummaryRejectsSourceNotarizationWithMalformedSubmissionID() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-notary-malformed-submission-id")
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-notary-malformed-submission-id-target")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false
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
        try setNotarizationSubmissionID(
            "manual-pass",
            to: bundleRoot.appendingPathComponent("source.notarization.json")
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.source.notarizationReady)
        XCTAssertEqual(
            snapshot.installedAppReleaseEvidence.source.notarizationFailureMessage,
            "source.notarization.json is not release-ready"
        )

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source"])
    }

    func testWorkflowSummaryRequiresCollectionReviewWhenCollectionMachineCountIsBelowTwo() throws {
        let bundleRoot = try makeReleaseReadyInstalledAppBundle(
            named: "acceptance-workflow-invalid-machine-count",
            collectionMode: "two_machine",
            machineCount: 1,
            includeWorkflowSummaryArtifact: false
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.installedAppCollectionProof.failures, ["invalid_collection"])
        XCTAssertEqual(snapshot.installedAppCollectionProof.finalEvaluationCollectionDetail, "collection.machine_count=1")

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["review_collection"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["either"])
        XCTAssertEqual(summary.nextActions.first?.commands, [])
    }

    func testWorkflowSummaryRequiresCollectionReviewWhenCollectionModeIsNotTwoMachine() throws {
        let bundleRoot = try makeReleaseReadyInstalledAppBundle(
            named: "acceptance-workflow-invalid-collection-mode",
            collectionMode: "same_machine",
            includeWorkflowSummaryArtifact: false
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.installedAppCollectionProof.failures, ["invalid_collection"])
        XCTAssertEqual(snapshot.installedAppCollectionProof.finalEvaluationCollectionDetail, "collection.mode=same_machine")

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["review_collection"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["either"])
        XCTAssertEqual(summary.nextActions.first?.commands, [])
    }

    func testWorkflowSummaryPrefersPackagingEvidenceBeforeCollectionCorrectionWhenReleaseEvidenceIsMissing() throws {
        let bundleRoot = try makeReleaseReadyInstalledAppBundle(
            named: "acceptance-workflow-release-before-collection-correction",
            collectionMode: "same_machine",
            includeWorkflowSummaryArtifact: false
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try FileManager.default.removeItem(at: bundleRoot.appendingPathComponent("source.app-audit.json"))

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.installedAppCollectionProof.failures, ["invalid_collection"])
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.ok)

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source"])
        XCTAssertTrue(summary.nextActions.first?.commands.first?.contains("record-packaging-evidence") == true)

        let shellSummary = try workflowStatus(bundleRoot: bundleRoot, requireOperatorEvidence: true)
        let shellNextActions = try XCTUnwrap(shellSummary["next_actions"] as? [[String: Any]])

        XCTAssertEqual(shellNextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])
        XCTAssertEqual(shellNextActions.map { $0["machine"] as? String }, ["source"])
    }

    func testWorkflowSummaryRequestsBundleHandoffWhenVerifiedHandoffDoesNotMatchRecordedPair() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-unmatched-handoff")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            bundleHandoffsJSON: """
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
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.count, 1)
        XCTAssertEqual(summary.nextActions.first?.machine, "either")
        XCTAssertEqual(summary.nextActions.first?.step, "bundle_handoff")
        XCTAssertTrue(summary.nextActions.first?.action.contains("pack/unpack/merge") == true)
        XCTAssertTrue(summary.nextActions.first?.commands.first?.contains("pack-bundle") == true)
    }

    func testWorkflowSummaryRequiresFreshPackagingEvidenceWhenLoadedNotarizationDoesNotMatchBundledAuditAndProvenance() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-release-evidence-stale-notary")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false
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
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: "/tmp/stale-source/SuperMover.app",
            auditPath: "/tmp/stale-source/SuperMover.app.audit.json"
        ).write(
            to: bundleRoot.appendingPathComponent("source.notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source"])
    }

    func testWorkflowSummaryRequiresFreshPackagingEvidenceWhenLoadedAuditProvenanceDoesNotMatchBundledProvenance() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-release-evidence-stale-provenance")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        let sourceProvenance = try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/tmp/current-source/SuperMover.app"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/tmp/current-target/SuperMover.app"
        )

        var staleSourceProvenance = sourceProvenance
        staleSourceProvenance["git_dirty"] = true
        try AcceptanceReleaseEvidenceFixtures.jsonString(staleSourceProvenance).write(
            to: bundleRoot.appendingPathComponent("source.provenance.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["source"])
    }

    func testWorkflowSummaryIgnoresStaleStrictArtifactThatLacksReleaseEvidenceFields() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-stale-artifact")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: true
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        try staleWorkflowSummaryJSON.write(
            to: bundleRoot.appendingPathComponent("workflow.summary.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["source_packaging_evidence", "target_packaging_evidence"])
    }

    func testWorkflowSummaryQuotedProfilePathStaysCurrentAcrossAppAndShell() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-quoted-profile")
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-quoted-profile-target")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let profilePath = "/tmp/agent's/source.profile.json"
        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: true
        )
        .replacingOccurrences(of: "/tmp/source.profile.json", with: profilePath)
        .write(
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
        try staleProofMatchedWorkflowSummaryJSON
            .replacingOccurrences(of: "/tmp/source.profile.json", with: profilePath)
            .write(
                to: bundleRoot.appendingPathComponent("workflow.summary.json"),
                atomically: true,
                encoding: .utf8
            )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)
        let expectedCommand =
            "sh macos/script/acceptance-two-machine.sh evaluate --bundle-root '<bundle-root>' --target-root '<target-root>' --source-profile '/tmp/agent'\"'\"'s/source.profile.json' --require-operator-evidence"

        XCTAssertEqual(summary.nextActions.map(\.step), ["evaluate"])
        XCTAssertEqual(summary.nextActions.first?.commands, [expectedCommand])

        let shellSummary = try workflowStatus(bundleRoot: bundleRoot, requireOperatorEvidence: true)
        let shellNextActions = try XCTUnwrap(shellSummary["next_actions"] as? [[String: Any]])
        let normalizedShellCommands = (shellNextActions.first?["commands"] as? [String])?.map {
            $0.replacingOccurrences(of: bundleRoot.path, with: "<bundle-root>")
        }

        XCTAssertEqual(shellNextActions.count, 1)
        XCTAssertEqual(shellNextActions.first?["step"] as? String, "evaluate")
        XCTAssertEqual(normalizedShellCommands, [expectedCommand])
    }

    func testWorkflowSummaryDefaultIgnoresStaleArtifactThatLacksCurrentTwoMachineProofFields() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-default-stale-artifact")
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-default-stale-artifact-target")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: true
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try staleWorkflowSummaryJSON.write(
            to: bundleRoot.appendingPathComponent("workflow.summary.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(summary.nextActions.map(\.step), ["evaluate"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["either"])
        XCTAssertTrue(summary.nextActions.first?.commands.first?.contains("acceptance-two-machine.sh evaluate") == true)
    }

    func testWorkflowSummaryIgnoresStrictArtifactWhenCollectionModeIsNotTwoMachine() throws {
        let bundleRoot = try makeReleaseReadyInstalledAppBundle(
            named: "acceptance-workflow-non-two-machine-stale-artifact",
            collectionMode: "same_machine",
            includeWorkflowSummaryArtifact: true
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try staleProofMatchedWorkflowSummaryJSON.write(
            to: bundleRoot.appendingPathComponent("workflow.summary.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.installedAppCollectionProof.failures, ["invalid_collection"])
        XCTAssertEqual(snapshot.installedAppCollectionProof.finalEvaluationCollectionDetail, "collection.mode=same_machine")

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["review_collection"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["either"])
        XCTAssertEqual(summary.nextActions.first?.commands, [])
    }

    func testWorkflowSummaryDefaultIgnoresStaleArtifactWhenCollectionModeIsNotTwoMachine() throws {
        let bundleRoot = try makeReleaseReadyInstalledAppBundle(
            named: "acceptance-workflow-default-non-two-machine-stale-artifact",
            collectionMode: "same_machine",
            includeWorkflowSummaryArtifact: true
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try staleWorkflowSummaryJSON.write(
            to: bundleRoot.appendingPathComponent("workflow.summary.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(summary.nextActions.map(\.step), ["evaluate"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["either"])
        XCTAssertTrue(summary.nextActions.first?.commands.first?.contains("acceptance-two-machine.sh evaluate") == true)
    }

    func testWorkflowSummaryIgnoresStrictArtifactWhenInstalledAppProofFieldsDisagreeWithBundleState() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-proof-mismatch")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: true
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(
            to: bundleRoot.appendingPathComponent("source.machine.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "target"
        }
        """.write(
            to: bundleRoot.appendingPathComponent("target.machine.json"),
            atomically: true,
            encoding: .utf8
        )
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
        try staleProofMatchedWorkflowSummaryJSON.write(
            to: bundleRoot.appendingPathComponent("workflow.summary.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["target_serve_phase_1", "source_pair"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["target", "source"])
    }

    func testWorkflowSummaryIgnoresStrictArtifactWhenBundleStatusAndActionsAreStale() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-status-mismatch")
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: true,
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
        try staleProofMatchedWorkflowSummaryJSON.write(
            to: bundleRoot.appendingPathComponent("workflow.summary.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(
            summary.nextActions.map(\.step),
            [
                "target_serve_phase_1",
                "source_browse",
                "target_advertise",
                "source_pair",
                "target_import",
                "source_transfer",
            ]
        )
        XCTAssertEqual(summary.nextActions.map(\.machine), ["target", "source", "target", "source", "target", "source"])

        let shellSummary = try workflowStatus(bundleRoot: bundleRoot, requireOperatorEvidence: true)
        let shellNextActions = try XCTUnwrap(shellSummary["next_actions"] as? [[String: Any]])

        XCTAssertEqual(
            shellNextActions.map { $0["step"] as? String },
            [
                "target_serve_phase_1",
                "source_browse",
                "target_advertise",
                "source_pair",
                "target_import",
                "source_transfer",
            ]
        )
        XCTAssertEqual(
            shellNextActions.map { $0["machine"] as? String },
            ["target", "source", "target", "source", "target", "source"]
        )
    }

    func testWorkflowSummaryRequiresEvaluateWhenCollectedBundleLacksCurrentEvaluationArtifact() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-workflow-missing-evaluation-artifact"
        )
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-workflow-missing-evaluation-artifact-target"
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

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["evaluate"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["either"])
        XCTAssertTrue(summary.nextActions.first?.commands.first?.contains("--require-operator-evidence") == true)

        let shellSummary = try workflowStatus(bundleRoot: bundleRoot, requireOperatorEvidence: true)
        let shellNextActions = try XCTUnwrap(shellSummary["next_actions"] as? [[String: Any]])

        XCTAssertEqual(shellNextActions.map { $0["step"] as? String }, ["evaluate"])
        XCTAssertEqual(shellNextActions.map { $0["machine"] as? String }, ["either"])
    }

    func testWorkflowSummaryRequiresStrictReevaluateWhenCollectedEvaluationUsedWeakerLane() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-workflow-stale-evaluation-lane"
        )
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-workflow-stale-evaluation-lane-target"
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let meta = addEvaluationRecord(
            to: AcceptanceInstalledAppBundleFixtures.bundleMeta(
                includeReleaseEvidenceMeta: true,
                includeWorkflowSummaryArtifact: false,
                status: "evidence_collected"
            ),
            targetRoot: targetRoot.path,
            requireOperatorEvidence: false
        )
        try meta.write(
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
        try writeEvaluationArtifact(
            bundleRoot: bundleRoot,
            targetRoot: targetRoot.path,
            requireOperatorEvidence: false
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(snapshot.hasCurrentEvaluationEvidence(requireOperatorEvidence: true))
        XCTAssertTrue(snapshot.hasCurrentEvaluationEvidence(requireOperatorEvidence: false))

        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.step), ["evaluate"])
        XCTAssertEqual(summary.nextActions.map(\.machine), ["either"])
        XCTAssertTrue(summary.nextActions.first?.commands.first?.contains("--require-operator-evidence") == true)

        let shellSummary = try workflowStatus(bundleRoot: bundleRoot, requireOperatorEvidence: true)
        let shellNextActions = try XCTUnwrap(shellSummary["next_actions"] as? [[String: Any]])

        XCTAssertEqual(shellNextActions.map { $0["step"] as? String }, ["evaluate"])
        XCTAssertEqual(shellNextActions.map { $0["machine"] as? String }, ["either"])
    }

    private func makeReleaseReadyInstalledAppBundle(
        named: String,
        collectionMode: String,
        machineCount: Int = 2,
        includeWorkflowSummaryArtifact: Bool
    ) throws -> URL {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: named)

        try releaseReadyBundleMetaJSON(
            collectionMode: collectionMode,
            machineCount: machineCount,
            includeWorkflowSummaryArtifact: includeWorkflowSummaryArtifact
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
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(
            bundleRoot: bundleRoot,
            targetRoot: bundleRoot.appendingPathComponent("target-root", isDirectory: true)
        )

        return bundleRoot
    }

    private func releaseReadyBundleMetaJSON(
        collectionMode: String,
        machineCount: Int,
        includeWorkflowSummaryArtifact: Bool
    ) -> String {
        AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: includeWorkflowSummaryArtifact
        )
        .replacingOccurrences(
            of: "\"mode\": \"two_machine\"",
            with: "\"mode\": \"\(collectionMode)\""
        )
        .replacingOccurrences(
            of: "\"machine_count\": 2",
            with: "\"machine_count\": \(machineCount)"
        )
    }

    private func addEvaluationRecord(
        to meta: String,
        targetRoot: String,
        requireOperatorEvidence: Bool
    ) -> String {
        meta.replacingOccurrences(
            of: """
            "operator": {
            """,
            with: """
            "evaluation": {
              "pairing_receipt_id": "pair-1",
              "session_id": "session-1",
              "target_root": "\(targetRoot)",
              "output": "evaluation.json",
              "require_operator_evidence": \(requireOperatorEvidence ? "true" : "false")
            },
            "operator": {
            """
        )
    }

    private func writeEvaluationArtifact(
        bundleRoot: URL,
        targetRoot: String,
        requireOperatorEvidence: Bool
    ) throws {
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "evidence_collected",
          "pairing_receipt_id": "pair-1",
          "session_id": "session-1",
          "target_root": "\(targetRoot)",
          "require_operator_evidence": \(requireOperatorEvidence ? "true" : "false")
        }
        """.write(
            to: bundleRoot.appendingPathComponent("evaluation.json"),
            atomically: true,
            encoding: .utf8
        )
    }

    private func addNotarizationFailureRecord(to url: URL) throws {
        let data = try Data(contentsOf: url)
        var root = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        root["failure"] = [
            "id": "notary_rejected",
            "message": "notarytool rejected the submission",
        ]
        let updated = try JSONSerialization.data(withJSONObject: root, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: url)
    }

    private func setNotarizationSubmissionID(_ submissionID: String, to url: URL) throws {
        let data = try Data(contentsOf: url)
        var root = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        var submission = try XCTUnwrap(root["submission"] as? [String: Any])
        submission["id"] = submissionID
        root["submission"] = submission
        let updated = try JSONSerialization.data(withJSONObject: root, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: url)
    }

    private var staleWorkflowSummaryJSON: String {
        """
        {
          "schema": "supermover.acceptance.workflow_summary.v1",
          "default": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "next_actions": [],
            "steps": []
          },
          "require_operator_evidence": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "next_actions": [
              {
                "machine": "either",
                "step": "evaluate",
                "action": "run acceptance evaluate against the merged bundle",
                "commands": [
                  "sh macos/script/acceptance-two-machine.sh evaluate --bundle-root '<bundle-root>' --target-root '<target-root>' --source-profile '/tmp/source.profile.json' --require-operator-evidence"
                ]
              }
            ],
            "steps": []
          }
        }
        """
    }

    private var staleProofMatchedWorkflowSummaryJSON: String {
        """
        {
          "schema": "supermover.acceptance.workflow_summary.v1",
          "default": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "next_actions": [],
            "steps": []
          },
          "require_operator_evidence": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "collection_mode": "two_machine",
            "machine_count": 2,
            "verified_bundle_handoffs": 1,
            "verified_cross_machine_bundle_handoffs": 1,
            "matches_recorded_machine_pair": true,
            "has_installed_app_machine_pair_proof": true,
            "installed_app_proof_ok": true,
            "installed_app_proof_failures": [],
            "ok": false,
            "failures": [],
            "blocked_reason": null,
            "missing_requirements": [],
            "primary_failure": null,
            "failure_message": null,
            "requires_machine_identity_correction": false,
            "requires_bundle_handoff_proof": false,
            "final_evaluation_collection_detail": null,
            "final_evaluation_machine_facts_detail": null,
            "final_evaluation_bundle_handoff_detail": null,
            "source_app_audit_ready": true,
            "target_app_audit_ready": true,
            "source_notarization_ready": true,
            "target_notarization_ready": true,
            "installed_app_release_evidence_ok": true,
            "installed_app_release_evidence_failures": [],
            "role_machine_ids": {
              "source": "source-machine",
              "target": "target-machine"
            },
            "machine_fact_ids": {
              "source": "source-machine",
              "target": "target-machine"
            },
            "machine_fact_artifact_ids": {
              "source": "source-machine",
              "target": "target-machine"
            },
            "machine_facts_consistent": true,
            "next_actions": [
              {
                "machine": "either",
                "step": "evaluate",
                "action": "run acceptance evaluate against the merged bundle",
                "commands": [
                  "sh macos/script/acceptance-two-machine.sh evaluate --bundle-root '<bundle-root>' --target-root '<target-root>' --source-profile '/tmp/source.profile.json' --require-operator-evidence"
                ]
              }
            ],
            "steps": []
          }
        }
        """
    }

    private func workflowStatus(
        bundleRoot: URL,
        requireOperatorEvidence: Bool
    ) throws -> [String: Any] {
        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        var arguments = [
            repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
            "workflow-status",
            "--bundle-root", bundleRoot.path,
        ]
        if requireOperatorEvidence {
            arguments.append("--require-operator-evidence")
        }
        let result = try AcceptanceScriptHarness.runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: arguments,
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        return try XCTUnwrap(
            JSONSerialization.jsonObject(with: Data(result.stdout.utf8)) as? [String: Any]
        )
    }

}
