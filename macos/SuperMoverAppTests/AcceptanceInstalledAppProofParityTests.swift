import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppProofParityTests: XCTestCase {
    func testMissingHandoffProofStaysAlignedAcrossWorkflowLaunchAndEvaluate() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-missing-handoff-parity"
        )
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-missing-handoff-parity-target"
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true,
            bundleHandoffsJSON: "[]"
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
        try writeAcceptanceFlowArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let missingReason = "missing verified bundle_handoffs"

        XCTAssertEqual(
            snapshot.installedAppCollectionProof.finalEvaluationBundleHandoffDetail,
            missingReason
        )

        let workflowSummary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.count, 1)
        XCTAssertEqual(workflowSummary.nextActions.first?.machine, "either")
        XCTAssertEqual(workflowSummary.nextActions.first?.step, "bundle_handoff")

        let launchVerdict = AcceptanceInstalledAppLaunchGate.evaluate(
            .init(
                machine: "source",
                bundleState: .loaded(collectionMode: "two_machine"),
                cliProvenance: makeBundledCLIProvenance(),
                installedAppCollectionProof: snapshot.installedAppCollectionProof,
                releaseEvidenceMachine: snapshot.installedAppReleaseEvidence.source,
                hasCurrentEvaluationPassState: false
            )
        )
        guard case let .installedAppProofBlocked(machine, detail)? = launchVerdict else {
            return XCTFail("expected installedAppProofBlocked, got \(String(describing: launchVerdict))")
        }
        XCTAssertEqual(machine, "source")
        XCTAssertTrue(detail.contains(missingReason))

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection(missingReason)
            )
        }
    }

    func testUnmatchedHandoffProofStaysAlignedAcrossWorkflowLaunchAndEvaluate() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-installed-app-proof-parity")
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-installed-app-proof-parity-target")
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true,
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
        try writeAcceptanceFlowArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let unmatchedReason =
            "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"

        let workflowSummary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.count, 1)
        XCTAssertEqual(workflowSummary.nextActions.first?.machine, "either")
        XCTAssertEqual(workflowSummary.nextActions.first?.step, "bundle_handoff")

        let launchVerdict = AcceptanceInstalledAppLaunchGate.evaluate(
            .init(
                machine: "source",
                bundleState: .loaded(collectionMode: "two_machine"),
                cliProvenance: makeBundledCLIProvenance(),
                installedAppCollectionProof: snapshot.installedAppCollectionProof,
                releaseEvidenceMachine: snapshot.installedAppReleaseEvidence.source,
                hasCurrentEvaluationPassState: false
            )
        )
        guard case let .installedAppProofBlocked(machine, detail)? = launchVerdict else {
            return XCTFail("expected installedAppProofBlocked, got \(String(describing: launchVerdict))")
        }
        XCTAssertEqual(machine, "source")
        XCTAssertTrue(detail.contains(unmatchedReason))

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection(unmatchedReason)
            )
        }
    }

    func testContradictoryHandoffProofStaysAlignedAcrossWorkflowLaunchPreviewPreflightAndEvaluate() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-contradictory-parity"
        )
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-contradictory-parity-target"
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true,
            bundleHandoffsJSON: """
            [
              {
                "archive": "bundle-good.tgz",
                "manifest": "bundle-good.manifest.json",
                "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
                "meta": "meta.json",
                "verified": true,
                "exporting_machine_id": "source-machine",
                "exporting_machine_label": "source",
                "importing_machine_id": "target-machine",
                "importing_machine_label": "target"
              },
              {
                "archive": "bundle-bad.tgz",
                "manifest": "bundle-bad.manifest.json",
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
        try writeAcceptanceFlowArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let contradictoryReason =
            "bundle_handoffs contain verified cross-machine archive handoff evidence for machine ids other than the recorded source/target pair"

        XCTAssertEqual(
            snapshot.installedAppCollectionProof.failures,
            ["contradictory_verified_bundle_handoffs"]
        )
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.blockedReason,
            .contradictoryVerifiedBundleHandoffs
        )
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.finalEvaluationBundleHandoffDetail,
            contradictoryReason
        )

        let workflowSummary = snapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.count, 1)
        XCTAssertEqual(workflowSummary.nextActions.first?.machine, "either")
        XCTAssertEqual(workflowSummary.nextActions.first?.step, "review_bundle_handoff")
        XCTAssertEqual(workflowSummary.nextActions.first?.commands, [String]())

        let launchGateVerdict = try XCTUnwrap(
            AcceptanceInstalledAppLaunchGate.evaluate(
                .init(
                    machine: "source",
                    bundleState: .loaded(collectionMode: "two_machine"),
                    cliProvenance: makeBundledCLIProvenance(),
                    installedAppCollectionProof: snapshot.installedAppCollectionProof,
                    releaseEvidenceMachine: snapshot.installedAppReleaseEvidence.source,
                    hasCurrentEvaluationPassState: false
                )
            )
        )
        guard case let .installedAppProofBlocked(machine, detail) = launchGateVerdict else {
            return XCTFail("expected installedAppProofBlocked, got \(launchGateVerdict)")
        }
        XCTAssertEqual(machine, "source")
        XCTAssertTrue(detail.contains("contradictory archive handoff evidence"))
        XCTAssertTrue(detail.contains(contradictoryReason))

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        var bundleURLRequested = false
        var bundleRefreshed = false
        let dependencies = AcceptanceInstalledAppLaunchCoordinator.Dependencies(
            currentContext: {
                .init(
                    selectedRole: .source,
                    bundlePath: bundleRoot.path,
                    loadedSnapshot: snapshot,
                    cliProvenance: self.makeBundledCLIProvenance()
                )
            },
            currentBundleURL: {
                bundleURLRequested = true
                XCTFail("launch preflight should not request bundle packaging access when contradictory handoff proof is already blocked")
                return bundleRoot
            },
            refreshBundle: {
                bundleRefreshed = true
                XCTFail("launch preflight should not refresh bundle state when contradictory handoff proof is already blocked")
            },
            acceptanceBundleOperations: AcceptanceBundleAppOperations(
                resourceURLProvider: {
                    XCTFail("launch preview/preflight should not probe packaging evidence when contradictory handoff proof is already blocked")
                    return nil
                },
                packagingCollectorFactory: {
                    XCTFail("launch preview/preflight should not collect packaging evidence when contradictory handoff proof is already blocked")
                    return AcceptancePackagingEvidenceCollector()
                }
            )
        )

        let preview = try XCTUnwrap(coordinator.preview(for: .pair, using: dependencies))
        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertEqual(preview.detail, launchGateVerdict.preview().detail)
        XCTAssertFalse(bundleURLRequested)
        XCTAssertFalse(bundleRefreshed)

        let preflightError = try XCTUnwrap(coordinator.preflightError(for: .pair, using: dependencies))
        XCTAssertEqual(preflightError, launchGateVerdict.preflightError)
        XCTAssertEqual(preflightError, preview.detail)
        XCTAssertFalse(bundleURLRequested)
        XCTAssertFalse(bundleRefreshed)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection(contradictoryReason)
            )
        }
    }

    func testStaleReleaseEvidenceStaysAlignedAcrossWorkflowLaunchPreviewAndEvaluateUntilPackagingEvidenceIsRefreshed() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-stale-release-evidence-parity"
        )
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-stale-release-evidence-parity-target"
        )
        let packagedApp = try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
            named: "SuperMover-stale-release-evidence-parity",
            cliVersion: "supermover 0.1.0-dev",
            includeNotarizationSidecar: true
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: targetRoot)
            try? FileManager.default.removeItem(at: packagedApp.appURL)
        }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
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
            auditPath: "/tmp/stale-source/SuperMover.app.notary/post-staple.audit.json",
            notaryLogPath: "source.notary-log.json"
        ).write(
            to: bundleRoot.appendingPathComponent("source.notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        try writeAcceptanceFlowArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        var loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertTrue(loadedSnapshot.installedAppCollectionProof.ok)
        XCTAssertFalse(loadedSnapshot.installedAppReleaseEvidence.source.notarizationReady)
        XCTAssertEqual(
            loadedSnapshot.installedAppReleaseEvidence.source.notarizationFailureMessage,
            "source.notarization.json does not match source.app-audit.json and source.provenance.json"
        )

        let workflowSummary = loadedSnapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.map(\.machine), ["source"])
        XCTAssertEqual(workflowSummary.nextActions.map(\.step), ["source_packaging_evidence"])
        XCTAssertTrue(
            workflowSummary.nextActions.first?.action.contains("record source release packaging evidence") == true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        func dependencies() -> AcceptanceInstalledAppLaunchCoordinator.Dependencies {
            AcceptanceInstalledAppLaunchCoordinator.Dependencies(
                currentContext: {
                    .init(
                        selectedRole: .source,
                        bundlePath: bundleRoot.path,
                        loadedSnapshot: loadedSnapshot,
                        cliProvenance: self.makeBundledCLIProvenance(resourcesURL: packagedApp.resourcesURL)
                    )
                },
                currentBundleURL: { bundleRoot },
                refreshBundle: {
                    loadedSnapshot = (try? AcceptanceBundleReader().load(bundleRootURL: bundleRoot)) ?? loadedSnapshot
                },
                acceptanceBundleOperations: AcceptanceBundleAppOperations(
                    resourceURLProvider: { packagedApp.resourcesURL }
                )
            )
        }

        let preview = try XCTUnwrap(coordinator.preview(for: .pair, using: dependencies()))
        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .review)
        XCTAssertTrue(preview.detail.contains("fresh packaging evidence"))
        XCTAssertTrue(preview.detail.contains("release-ready notarization"))

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidNotarizationEvidence("source")
            )
        }

        XCTAssertNil(coordinator.preflightError(for: .pair, using: dependencies()))

        let refreshedWorkflowSummary = loadedSnapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(refreshedWorkflowSummary.nextActions.map(\.machine), ["either"])
        XCTAssertEqual(refreshedWorkflowSummary.nextActions.map(\.step), ["evaluate"])

        let refreshedPreview = try XCTUnwrap(coordinator.preview(for: .pair, using: dependencies()))
        XCTAssertEqual(refreshedPreview.machine, "source")
        XCTAssertEqual(refreshedPreview.state, .review)
        XCTAssertTrue(refreshedPreview.detail.contains("current strict bundle truth"))

        XCTAssertNoThrow(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        )

        loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)

        let evaluatedWorkflowSummary = loadedSnapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(evaluatedWorkflowSummary.nextActions, [])

        let evaluatedPreview = try XCTUnwrap(coordinator.preview(for: .pair, using: dependencies()))
        XCTAssertEqual(evaluatedPreview.machine, "source")
        XCTAssertEqual(evaluatedPreview.state, .pass)
        XCTAssertTrue(evaluatedPreview.detail.contains("notarization evidence is accepted"))
    }

    func testMachineIdentityCorrectionProofStaysAlignedAcrossWorkflowCorrectiveLaunchesAndEvaluate() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-machine-identity-parity"
        )
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-machine-identity-parity-target"
        )
        let packagedApp = try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
            named: "SuperMover-machine-identity-parity",
            cliVersion: "supermover 0.1.0-dev",
            includeNotarizationSidecar: true
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: targetRoot)
            try? FileManager.default.removeItem(at: packagedApp.appURL)
        }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true,
            sourceMachineID: "same-machine",
            targetMachineID: "same-machine"
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(
            bundleRoot: bundleRoot,
            sourceMachineID: "same-machine",
            targetMachineID: "same-machine"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: packagedApp.appURL.path,
            cliVersion: "supermover 0.1.0-dev"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: packagedApp.appURL.path,
            cliVersion: "supermover 0.1.0-dev"
        )
        try writeAcceptanceFlowArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        var loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        let correctionReason = "source_pair and target share machine_id=same-machine"

        XCTAssertEqual(
            loadedSnapshot.installedAppCollectionProof.blockedReason,
            .sameRoleMachineIDs
        )
        XCTAssertEqual(
            loadedSnapshot.installedAppCollectionProof.finalEvaluationCollectionDetail,
            correctionReason
        )

        let workflowSummary = loadedSnapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.map(\.step), ["target_serve_phase_1", "source_pair"])
        XCTAssertEqual(workflowSummary.nextActions.map(\.machine), ["target", "source"])

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let operations = AcceptanceBundleAppOperations(
            resourceURLProvider: { packagedApp.resourcesURL },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { appBundleURL, outputURL in
                        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
                            appPath: appBundleURL.path,
                            provenanceManifest: try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(
                                appBundleURL: appBundleURL
                            )
                        ).write(to: outputURL, atomically: true, encoding: .utf8)
                        return .init(
                            exitCode: 0,
                            status: "pass",
                            readiness: "distribution_ready",
                            passReady: true,
                            blockingChecks: 0
                        )
                    }
                )
            }
        )

        func dependencies(selectedRole: WorkbenchRole) -> AcceptanceInstalledAppLaunchCoordinator.Dependencies {
            AcceptanceInstalledAppLaunchCoordinator.Dependencies(
                currentContext: {
                    .init(
                        selectedRole: selectedRole,
                        bundlePath: bundleRoot.path,
                        loadedSnapshot: loadedSnapshot,
                        cliProvenance: self.makeBundledCLIProvenance(resourcesURL: packagedApp.resourcesURL)
                    )
                },
                currentBundleURL: { bundleRoot },
                refreshBundle: {
                    loadedSnapshot = (try? AcceptanceBundleReader().load(bundleRootURL: bundleRoot)) ?? loadedSnapshot
                },
                acceptanceBundleOperations: operations
            )
        }

        let pairPreview = try XCTUnwrap(
            coordinator.preview(for: .pair, using: dependencies(selectedRole: .source))
        )
        XCTAssertEqual(pairPreview.machine, "source")
        XCTAssertEqual(pairPreview.state, .review)
        XCTAssertTrue(pairPreview.detail.contains("machine identity correction"))
        XCTAssertTrue(pairPreview.detail.contains("rewrite source_pair"))

        let servePreview = try XCTUnwrap(
            coordinator.preview(for: .serve, using: dependencies(selectedRole: .target))
        )
        XCTAssertEqual(servePreview.machine, "target")
        XCTAssertEqual(servePreview.state, .review)
        XCTAssertTrue(servePreview.detail.contains("machine identity correction"))
        XCTAssertTrue(servePreview.detail.contains("rewrite target role"))

        XCTAssertNil(
            coordinator.preflightError(for: .pair, using: dependencies(selectedRole: .source))
        )
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("source.version.txt").path
            )
        )

        XCTAssertNil(
            coordinator.preflightError(for: .serve, using: dependencies(selectedRole: .target))
        )
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("target.version.txt").path
            )
        )

        let unrelatedTaskError = try XCTUnwrap(
            coordinator.preflightError(for: .networkPush, using: dependencies(selectedRole: .source))
        )
        XCTAssertTrue(unrelatedTaskError.contains("same machine_id"))

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection(correctionReason)
            )
        }
    }

    func testStaleStrictEvaluationRequiresSourcePairRepairBeforeUnrelatedLaunchesCanProceed() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-stale-evaluation-source-pair"
        )
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-stale-evaluation-source-pair-target"
        )
        let packagedApp = try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
            named: "SuperMover-stale-evaluation-source-pair",
            cliVersion: "supermover 0.1.0-dev",
            includeNotarizationSidecar: true
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: targetRoot)
            try? FileManager.default.removeItem(at: packagedApp.appURL)
        }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: packagedApp.appURL.path,
            cliVersion: "supermover 0.1.0-dev"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: packagedApp.appURL.path,
            cliVersion: "supermover 0.1.0-dev"
        )
        try writeAcceptanceFlowArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        XCTAssertNoThrow(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        )

        try FileManager.default.removeItem(at: bundleRoot.appendingPathComponent("source.pair.txt"))

        var loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertTrue(loadedSnapshot.hasCurrentEvaluationEvidence(requireOperatorEvidence: true))
        XCTAssertFalse(loadedSnapshot.hasCurrentEvaluationPassState(requireOperatorEvidence: true))

        let workflowSummary = loadedSnapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.map(\.step), ["source_pair", "target_import"])
        XCTAssertEqual(workflowSummary.nextActions.map(\.machine), ["source", "target"])

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        func dependencies(selectedRole: WorkbenchRole) -> AcceptanceInstalledAppLaunchCoordinator.Dependencies {
            AcceptanceInstalledAppLaunchCoordinator.Dependencies(
                currentContext: {
                    .init(
                        selectedRole: selectedRole,
                        bundlePath: bundleRoot.path,
                        loadedSnapshot: loadedSnapshot,
                        cliProvenance: self.makeBundledCLIProvenance(resourcesURL: packagedApp.resourcesURL)
                    )
                },
                currentBundleURL: { bundleRoot },
                refreshBundle: {
                    loadedSnapshot = (try? AcceptanceBundleReader().load(bundleRootURL: bundleRoot)) ?? loadedSnapshot
                },
                acceptanceBundleOperations: AcceptanceBundleAppOperations(
                    resourceURLProvider: { packagedApp.resourcesURL }
                )
            )
        }

        let pairPreview = try XCTUnwrap(
            coordinator.preview(for: .pair, using: dependencies(selectedRole: .source))
        )
        XCTAssertEqual(pairPreview.machine, "source")
        XCTAssertEqual(pairPreview.state, .blocked)
        XCTAssertTrue(pairPreview.detail.contains("export source pairing receipt"))

        let pairError = try XCTUnwrap(
            coordinator.preflightError(for: .pair, using: dependencies(selectedRole: .source))
        )
        XCTAssertTrue(pairError.contains("export source pairing receipt"))

        let pushPreview = try XCTUnwrap(
            coordinator.preview(for: .networkPush, using: dependencies(selectedRole: .source))
        )
        XCTAssertEqual(pushPreview.machine, "source")
        XCTAssertEqual(pushPreview.state, .blocked)
        XCTAssertTrue(pushPreview.detail.contains("export source pairing receipt"))

        let pushError = try XCTUnwrap(
            coordinator.preflightError(for: .networkPush, using: dependencies(selectedRole: .source))
        )
        XCTAssertTrue(pushError.contains("export source pairing receipt"))

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingRequiredArtifact("source.pair.txt")
            )
        }
    }

    func testStaleStrictEvaluationAllowsSourceBrowseRepairButBlocksOtherLaunches() throws {
        try assertStaleStrictEvaluationRepair(
            named: "source-browse",
            drift: { bundleRoot in
                try FileManager.default.removeItem(
                    at: bundleRoot.appendingPathComponent("source.browse.json")
                )
            },
            expectedNextActionStep: "source_browse",
            expectedNextActionMachine: "source",
            matchingTask: .discoverBrowse,
            matchingRole: .source,
            matchingMachine: "source",
            matchingAction: "collect source browse evidence",
            unrelatedTask: .pair,
            unrelatedRole: .source,
            expectedEvaluationError: .missingRequiredArtifact("source.browse.json")
        )
    }

    func testStaleStrictEvaluationAllowsTargetAdvertiseRepairButBlocksOtherLaunches() throws {
        try assertStaleStrictEvaluationRepair(
            named: "target-advertise",
            drift: { bundleRoot in
                try FileManager.default.removeItem(
                    at: bundleRoot.appendingPathComponent("target.advertise.json")
                )
            },
            expectedNextActionStep: "target_advertise",
            expectedNextActionMachine: "target",
            matchingTask: .discoverAdvertise,
            matchingRole: .target,
            matchingMachine: "target",
            matchingAction: "collect target advertise evidence",
            unrelatedTask: .pair,
            unrelatedRole: .source,
            expectedEvaluationError: .missingRequiredArtifact("target.advertise.json")
        )
    }

    func testStaleStrictEvaluationAllowsTargetImportRepairButBlocksOtherLaunches() throws {
        try assertStaleStrictEvaluationRepair(
            named: "target-import",
            drift: { bundleRoot in
                try self.rewriteTargetImportPairingReceiptID(
                    bundleRoot: bundleRoot,
                    pairingReceiptID: "pair-mismatch"
                )
            },
            expectedNextActionStep: "target_import",
            expectedNextActionMachine: "target",
            matchingTask: .profileAdoptPairing,
            matchingRole: .target,
            matchingMachine: "target",
            matchingAction: "import pairing receipt on target",
            unrelatedTask: .serve,
            unrelatedRole: .target,
            expectedEvaluationError: .invalidTargetImportEvidence
        )
    }

    func testStaleStrictEvaluationAllowsSourceTransferRepairButBlocksOtherLaunches() throws {
        try assertStaleStrictEvaluationRepair(
            named: "source-transfer",
            drift: { bundleRoot in
                try FileManager.default.removeItem(
                    at: bundleRoot.appendingPathComponent("source.network-push.txt")
                )
            },
            expectedNextActionStep: "source_transfer",
            expectedNextActionMachine: "source",
            matchingTask: .networkPush,
            matchingRole: .source,
            matchingMachine: "source",
            matchingAction: "run source mTLS transfer and consistency proof",
            unrelatedTask: .pair,
            unrelatedRole: .source,
            expectedEvaluationError: .missingRequiredArtifact("source.network-push.txt")
        )
    }

    func testStaleStrictEvaluationBlocksMatchingLaunchWhenMultipleRequiredStepsReopen() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-stale-evaluation-multi-step"
        )
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-stale-evaluation-multi-step-target"
        )
        let packagedApp = try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
            named: "SuperMover-stale-evaluation-multi-step",
            cliVersion: "supermover 0.1.0-dev",
            includeNotarizationSidecar: true
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: targetRoot)
            try? FileManager.default.removeItem(at: packagedApp.appURL)
        }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: packagedApp.appURL.path,
            cliVersion: "supermover 0.1.0-dev"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: packagedApp.appURL.path,
            cliVersion: "supermover 0.1.0-dev"
        )
        try writeAcceptanceFlowArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        XCTAssertNoThrow(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        )

        try FileManager.default.removeItem(at: bundleRoot.appendingPathComponent("source.browse.json"))
        try FileManager.default.removeItem(at: bundleRoot.appendingPathComponent("target.advertise.json"))

        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertTrue(loadedSnapshot.hasCurrentEvaluationEvidence(requireOperatorEvidence: true))
        XCTAssertFalse(loadedSnapshot.hasCurrentEvaluationPassState(requireOperatorEvidence: true))

        let workflowSummary = loadedSnapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.map(\.step), ["source_browse", "target_advertise"])
        XCTAssertEqual(workflowSummary.nextActions.map(\.machine), ["source", "target"])

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let dependencies = AcceptanceInstalledAppLaunchCoordinator.Dependencies(
            currentContext: {
                .init(
                    selectedRole: .source,
                    bundlePath: bundleRoot.path,
                    loadedSnapshot: loadedSnapshot,
                    cliProvenance: self.makeBundledCLIProvenance(resourcesURL: packagedApp.resourcesURL)
                )
            },
            currentBundleURL: { bundleRoot },
            refreshBundle: {},
            acceptanceBundleOperations: AcceptanceBundleAppOperations(
                resourceURLProvider: { packagedApp.resourcesURL }
            )
        )

        let browsePreview = try XCTUnwrap(
            coordinator.preview(for: .discoverBrowse, using: dependencies)
        )
        XCTAssertEqual(browsePreview.machine, "source")
        XCTAssertEqual(browsePreview.state, .blocked)
        XCTAssertTrue(browsePreview.detail.contains("collect source browse evidence"))

        let browseError = try XCTUnwrap(
            coordinator.preflightError(for: .discoverBrowse, using: dependencies)
        )
        XCTAssertTrue(browseError.contains("collect source browse evidence"))

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingRequiredArtifact("source.browse.json")
            )
        }
    }

    func testStaleStrictEvaluationBlocksLaunchWhenOperatorEvidenceDrifts() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-stale-evaluation-operator"
        )
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-stale-evaluation-operator-target"
        )
        let packagedApp = try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
            named: "SuperMover-stale-evaluation-operator",
            cliVersion: "supermover 0.1.0-dev",
            includeNotarizationSidecar: true
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: targetRoot)
            try? FileManager.default.removeItem(at: packagedApp.appURL)
        }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: packagedApp.appURL.path,
            cliVersion: "supermover 0.1.0-dev"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: packagedApp.appURL.path,
            cliVersion: "supermover 0.1.0-dev"
        )
        try writeAcceptanceFlowArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        XCTAssertNoThrow(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        )
        try rewriteOperatorEvidence(
            bundleRoot: bundleRoot,
            kind: "firewall",
            status: "blocked",
            detail: "dismissed"
        )

        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertTrue(loadedSnapshot.hasCurrentEvaluationEvidence(requireOperatorEvidence: true))
        XCTAssertFalse(loadedSnapshot.hasCurrentEvaluationPassState(requireOperatorEvidence: true))

        let workflowSummary = loadedSnapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.map(\.step), ["operator_firewall"])
        XCTAssertEqual(workflowSummary.nextActions.map(\.machine), ["target"])

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let dependencies = AcceptanceInstalledAppLaunchCoordinator.Dependencies(
            currentContext: {
                .init(
                    selectedRole: .source,
                    bundlePath: bundleRoot.path,
                    loadedSnapshot: loadedSnapshot,
                    cliProvenance: self.makeBundledCLIProvenance(resourcesURL: packagedApp.resourcesURL)
                )
            },
            currentBundleURL: { bundleRoot },
            refreshBundle: {},
            acceptanceBundleOperations: AcceptanceBundleAppOperations(
                resourceURLProvider: { packagedApp.resourcesURL }
            )
        )

        let pairPreview = try XCTUnwrap(coordinator.preview(for: .pair, using: dependencies))
        XCTAssertEqual(pairPreview.machine, "source")
        XCTAssertEqual(pairPreview.state, .blocked)
        XCTAssertTrue(pairPreview.detail.contains("record firewall evidence"))

        let pairError = try XCTUnwrap(
            coordinator.preflightError(for: .pair, using: dependencies)
        )
        XCTAssertTrue(pairError.contains("record firewall evidence"))

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingOperatorEvidence("firewall")
            )
        }
    }

    private func writeAcceptanceFlowArtifacts(bundleRoot: URL, targetRoot: URL) throws {
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
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

    private func assertStaleStrictEvaluationRepair(
        named name: String,
        drift: (URL) throws -> Void,
        expectedNextActionStep: String,
        expectedNextActionMachine: String,
        matchingTask: SuperMoverTaskKind,
        matchingRole: WorkbenchRole,
        matchingMachine: String,
        matchingAction: String,
        unrelatedTask: SuperMoverTaskKind,
        unrelatedRole: WorkbenchRole,
        expectedEvaluationError: AcceptanceBundleEvaluationCoordinator.EvaluationError
    ) throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-stale-evaluation-\(name)"
        )
        let targetRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-installed-app-proof-stale-evaluation-\(name)-target"
        )
        let packagedApp = try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
            named: "SuperMover-stale-evaluation-\(name)",
            cliVersion: "supermover 0.1.0-dev",
            includeNotarizationSidecar: true
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: targetRoot)
            try? FileManager.default.removeItem(at: packagedApp.appURL)
        }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: true,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: packagedApp.appURL.path,
            cliVersion: "supermover 0.1.0-dev"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: packagedApp.appURL.path,
            cliVersion: "supermover 0.1.0-dev"
        )
        try writeAcceptanceFlowArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        XCTAssertNoThrow(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        )

        try drift(bundleRoot)

        var loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertTrue(loadedSnapshot.hasCurrentEvaluationEvidence(requireOperatorEvidence: true))
        XCTAssertFalse(loadedSnapshot.hasCurrentEvaluationPassState(requireOperatorEvidence: true))

        let workflowSummary = loadedSnapshot.workflowSummary(requireOperatorEvidence: true)
        XCTAssertEqual(workflowSummary.nextActions.map(\.step), [expectedNextActionStep])
        XCTAssertEqual(workflowSummary.nextActions.map(\.machine), [expectedNextActionMachine])

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        func dependencies(
            selectedRole: WorkbenchRole
        ) -> AcceptanceInstalledAppLaunchCoordinator.Dependencies {
            AcceptanceInstalledAppLaunchCoordinator.Dependencies(
                currentContext: {
                    .init(
                        selectedRole: selectedRole,
                        bundlePath: bundleRoot.path,
                        loadedSnapshot: loadedSnapshot,
                        cliProvenance: self.makeBundledCLIProvenance(resourcesURL: packagedApp.resourcesURL)
                    )
                },
                currentBundleURL: { bundleRoot },
                refreshBundle: {
                    loadedSnapshot = (try? AcceptanceBundleReader().load(bundleRootURL: bundleRoot))
                        ?? loadedSnapshot
                },
                acceptanceBundleOperations: AcceptanceBundleAppOperations(
                    resourceURLProvider: { packagedApp.resourcesURL }
                )
            )
        }

        let matchingPreview = try XCTUnwrap(
            coordinator.preview(for: matchingTask, using: dependencies(selectedRole: matchingRole))
        )
        XCTAssertEqual(matchingPreview.machine, matchingMachine)
        XCTAssertEqual(matchingPreview.state, .review)
        XCTAssertTrue(matchingPreview.detail.contains(matchingAction))

        XCTAssertNil(
            coordinator.preflightError(for: matchingTask, using: dependencies(selectedRole: matchingRole))
        )
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("\(matchingMachine).version.txt").path
            )
        )

        let unrelatedPreview = try XCTUnwrap(
            coordinator.preview(for: unrelatedTask, using: dependencies(selectedRole: unrelatedRole))
        )
        XCTAssertEqual(unrelatedPreview.state, .blocked)
        XCTAssertTrue(unrelatedPreview.detail.contains(matchingAction))

        let unrelatedError = try XCTUnwrap(
            coordinator.preflightError(for: unrelatedTask, using: dependencies(selectedRole: unrelatedRole))
        )
        XCTAssertTrue(unrelatedError.contains(matchingAction))

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                expectedEvaluationError
            )
        }
    }

    private func rewriteOperatorEvidence(
        bundleRoot: URL,
        kind: String,
        status: String,
        detail: String
    ) throws {
        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        var meta = try XCTUnwrap(
            JSONSerialization.jsonObject(with: data) as? [String: Any]
        )
        var evidence = try XCTUnwrap(meta["evidence"] as? [String: Any])
        var operatorEvidence = try XCTUnwrap(evidence["operator"] as? [String: Any])
        var record = (operatorEvidence[kind] as? [String: Any]) ?? [:]
        record["status"] = status
        record["detail"] = detail
        operatorEvidence[kind] = record
        evidence["operator"] = operatorEvidence
        meta["evidence"] = evidence
        let rewritten = try JSONSerialization.data(withJSONObject: meta, options: [.prettyPrinted, .sortedKeys])
        try rewritten.write(to: metaURL)
    }

    private func rewriteTargetImportPairingReceiptID(
        bundleRoot: URL,
        pairingReceiptID: String
    ) throws {
        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        var meta = try XCTUnwrap(
            JSONSerialization.jsonObject(with: data) as? [String: Any]
        )
        var evidence = try XCTUnwrap(meta["evidence"] as? [String: Any])
        var targetImport = try XCTUnwrap(evidence["target_import"] as? [String: Any])
        targetImport["pairing_receipt_id"] = pairingReceiptID
        evidence["target_import"] = targetImport
        meta["evidence"] = evidence
        let rewritten = try JSONSerialization.data(withJSONObject: meta, options: [.prettyPrinted, .sortedKeys])
        try rewritten.write(to: metaURL)
    }

    private func makeBundledCLIProvenance() -> CLIProvenance {
        CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1.0",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef123456",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-01-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
    }

    private func makeBundledCLIProvenance(resourcesURL: URL) -> CLIProvenance {
        CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: resourcesURL.appendingPathComponent("bin/supermover").path,
            workingDirectoryPath: resourcesURL.appendingPathComponent("bin").path,
            bundleIdentifier: "dev.supermover.macapp",
            appVersion: "0.1.0",
            provenancePath: resourcesURL.appendingPathComponent("supermover-provenance.json").path,
            provenanceStatus: "loaded",
            bundleCommit: "deadbeefdead",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "Developer ID Application: Test (TEAMID1234)",
            gitDirty: false,
            builtAt: "2026-06-03T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
    }

    private func makeCurrentSourceConsistencyJSON() -> String {
        """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "status": "pass",
          "mode": "current_source_verified",
          "session_id": "session-1",
          "detail": "test fixture"
        }
        """
    }

    private func makeSourceBaselineJSON() -> String {
        """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-1",
          "root_id": "root-1",
          "root_path": "/tmp/source",
          "session_id": "session-1",
          "created_at": "2026-01-01T00:00:00Z",
          "entries": []
        }
        """
    }

    private func makeVerifyJSON(targetRoot: String) -> String {
        """
        {
          "target_root": "\(targetRoot)",
          "session_id": "session-1",
          "manifest": {"id":"m1","session_id":"session-1","root_id":"root-1","created_at":"2026-01-01T00:00:00Z","entries":1,"files":1},
          "summary": {
            "manifest_count": 1,
            "manifest_entries": 1,
            "files_expected": 1,
            "files_verified": 1,
            "warnings": 0,
            "soft_deletes": 0,
            "target_drifts": 0,
            "artifact_problems": 0,
            "error_findings": 0,
            "warning_findings": 0,
            "skipped_digest": 0
          }
        }
        """
    }

    private func makeReportJSON(targetRoot: String) -> String {
        """
        {
          "target_root": "\(targetRoot)",
          "overall": {"status":"ok","issues":[]},
          "summary": {"warnings":0,"soft_deletes":0,"target_drifts":0,"live_target_drifts":0,"prune_candidates":0,"prune_refusals":0,"prune_approvals":0,"network_transfers":1,"artifact_problems":0},
          "latest_session": {"id":"session-1","manifest_id":"m1","created_at":"2026-01-01T00:00:00Z","entries":1,"files":1,"completeness":{"status":"verified","files_expected":1,"files_verified":1,"verification_errors":0,"verification_warnings":0}},
          "prune_review": {"status":"clear","approval_required":false,"apply":"none","summary":{"candidates":0,"refusals":0,"approvals":0,"unapplied_approvals":0,"receipt_issues":0}},
          "pairing": {"status":"paired_receipt_valid","receipt_id":"pair-1","target_device_id":"dst-spki","paired_at":"2026-01-01T00:00:00Z","method":"verification_code","verified_at":"2026-01-01T00:00:00Z","evidence":"receipt","receipt_source":"target_control","receipt_path":"p","source_receipt_path":"s","target_receipt_path":"t","encrypted_transfer":"required"},
          "privacy": {"status":"review","claim":"bounded","network_transfer":"published"},
          "health": {"healthy":true,"summary":{"incomplete_sessions":0,"invalid_records":0,"artifact_problems":0,"target_drifts":0,"network_transfers":1}}
        }
        """
    }

    private func makeStatusJSON(targetRoot: String) -> String {
        """
        {
          "profile_id": "profile-1",
          "target_id": "target-1",
          "target_root": "\(targetRoot)",
          "overall": {"status":"ok","target_status":"ok"},
          "issues": [],
          "latest_session": {"id":"session-1","manifest_id":"m1","created_at":"2026-01-01T00:00:00Z","entries":1,"completeness_status":"verified","files_expected":1,"files_verified":1,"verification_errors":0,"verification_warnings":0},
          "counts": {"warnings":0,"soft_deletes":0,"target_drifts":0,"live_target_drifts":0,"live_target_drift_artifact_problems":0,"prune_unapplied_approvals":0,"prune_active_approvals":0,"prune_stale_approvals":0,"prune_expired_approvals":0,"prune_consumed_approvals":0,"prune_receipt_issues":0,"recovery_issues":0,"artifact_problems":0,"network_transfers":1},
          "prune_review": {"status":"clear","action":"none"},
          "pairing": {"status":"paired_receipt_valid","receipt_id":"pair-1","target_device_id":"dst-spki","paired_at":"2026-01-01T00:00:00Z","method":"verification_code","verified_at":"2026-01-01T00:00:00Z","evidence":"receipt","receipt_source":"target_control","receipt_path":"p","source_receipt_path":"s","target_receipt_path":"t","encrypted_transfer":"required"},
          "privacy": {"status":"review","mode":"bounded","traffic_level":2,"claim":"bounded","local_push":"disabled","network_transfer":"published","residual_leakage":[],"configured_reductions":[],"overhead_status":"published","overhead_source":"network-transfer"},
          "traffic_privacy_acceptance": {"status":"review","claim":"bounded","blockers":[]},
          "network": {"status":"published","artifact_problems":0,"transfers":[{"session_id":"session-1","status":"published","stage":"commit","action":"preserved"}]},
          "artifact_problem_sources": []
        }
        """
    }

    private func makeHealthJSON(targetRoot: String) -> String {
        """
        {
          "target_root": "\(targetRoot)",
          "healthy": true,
          "summary": {"incomplete_sessions":0,"invalid_records":0,"artifact_problems":0,"target_drifts":0,"network_transfers":1},
          "items": [],
          "invalid": [],
          "artifacts": [],
          "network_transfers": [{"session_id":"session-1","status":"published","stage":"commit","action":"preserved"}]
        }
        """
    }
}
