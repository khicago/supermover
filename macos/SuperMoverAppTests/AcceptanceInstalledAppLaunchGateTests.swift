import Foundation
import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppLaunchGateTests: XCTestCase {
    private var temporaryBundleRoots: [URL] = []

    override func tearDown() {
        for url in temporaryBundleRoots {
            try? FileManager.default.removeItem(at: url)
        }
        temporaryBundleRoots.removeAll()
        super.tearDown()
    }

    func testGateSkipsNonTwoMachineCollection() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(bundleState: .loaded(collectionMode: "same_machine"))
        )

        XCTAssertNil(verdict)
    }

    func testGateFailsClosedWhenAcceptanceBundleCannotBeLoaded() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(bundleState: .invalid(path: "/tmp/bundle", detail: "Acceptance bundle meta.json is malformed at /tmp/bundle/meta.json."))
        )

        XCTAssertEqual(
            verdict,
            .invalidAcceptanceBundle(
                machine: "source",
                detail: "Acceptance bundle meta.json is malformed at /tmp/bundle/meta.json."
            )
        )
    }

    func testGateRequiresPackagedAppBeforeOtherChecks() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(
                cliMode: .development,
                readiness: "development launcher"
            )
        )

        XCTAssertEqual(
            verdict,
            .requiresPackagedApp(machine: "source", mode: .development, readiness: "development launcher")
        )
        XCTAssertEqual(
            verdict?.preflightError,
            "Real two-machine installed-app acceptance tasks require a packaged app. Current CLI mode is development (development launcher)."
        )
    }

    func testGateFailsClosedWhenPackagingAuditMissing() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(includeAppAudit: false)
        )

        XCTAssertEqual(verdict, .missingPackagingAudit(machine: "source"))
    }

    func testGateFailsClosedWhenPackagingAuditPassesButLacksRequiredAppPath() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(
                appAudit: makeAppAudit(appPath: nil)
            )
        )

        XCTAssertEqual(
            verdict,
            .packagingAuditNotReady(machine: "source", readiness: "distribution_ready")
        )
    }

    func testGateFailsClosedWhenPackagingAuditPassesButIsNotDistributionReady() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(
                appAudit: makeAppAudit(
                    status: "pass",
                    readiness: "review_only",
                    passReady: true
                )
            )
        )

        XCTAssertEqual(
            verdict,
            .packagingAuditNotReady(machine: "source", readiness: "review_only")
        )
    }

    func testGateFailsClosedWhenNotarizationIsMissing() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(includeNotarizationArtifact: false)
        )

        XCTAssertEqual(verdict, .missingNotarization(machine: "source"))
    }

    func testGateFailsClosedWhenNotarizationIsNotReleaseReady() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(
                notarizationArtifact: makeNotarizationArtifact(
                    status: "blocked",
                    submissionStatus: "Invalid",
                    auditStatus: "blocked",
                    auditReadiness: "blocked",
                    auditPassReady: false
                )
            )
        )

        XCTAssertEqual(verdict, .notarizationNotReady(machine: "source", status: "blocked"))
    }

    func testGateFailsClosedWhenNotarizationPassStatusStillLacksReleaseReadyFields() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(
                notarizationArtifact: makeNotarizationArtifact(
                    status: "pass",
                    submissionStatus: "In Progress",
                    auditStatus: "pass",
                    auditReadiness: "distribution_ready",
                    auditPassReady: true
                )
            )
        )

        XCTAssertEqual(verdict, .notarizationNotReady(machine: "source", status: "pass"))
    }

    func testGateReviewsWhenCurrentEvaluationEvidenceIsMissing() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput()
        )

        XCTAssertEqual(
            verdict,
            .finalEvaluationPending(
                machine: "source",
                detail: "Loaded source packaging audit is accepted, notarization evidence is accepted, and distinct-machine installed-app proof matches the current bundle, but the bundle does not currently satisfy strict bundle-local final-evaluation truth for this merged evidence. Launch can still collect phase evidence; final evaluate must record current strict bundle truth before this advisory can pass."
            )
        )
        XCTAssertNil(verdict?.preflightError)
    }

    func testGateReturnsReadyWhenCurrentEvaluationEvidenceIsPresent() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(hasCurrentEvaluationPassState: true)
        )

        XCTAssertEqual(verdict, .ready(machine: "source", auditReadiness: "distribution_ready"))
    }

    func testGateReviewsIncompleteInstalledAppProofWhenAuditAndNotarizationAreReady() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(installedAppCollectionProof: makeIncompleteInstalledAppCollectionProof())
        )

        XCTAssertEqual(
            verdict,
            .installedAppProofIncomplete(
                machine: "source",
                detail: "Loaded source packaging audit is accepted, but distinct-machine installed-app proof is not complete yet: role machine ids are not recorded yet; source.machine.json / target.machine.json are not recorded yet; verified bundle_handoffs are not recorded yet. Launch can still collect phase evidence; final evaluate remains blocked until machine facts and a verified cross-machine bundle handoff prove the recorded source/target pair."
            )
        )
        XCTAssertNil(verdict?.preflightError)
    }

    func testGateBlocksWhenVerifiedHandoffDoesNotMatchRecordedPair() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(installedAppCollectionProof: makeUnmatchedInstalledAppCollectionProof())
        )

        XCTAssertEqual(
            verdict,
            .installedAppProofBlocked(
                machine: "source",
                detail: "Current acceptance bundle is missing distinct-machine archive handoff proof: bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids. Distinct-machine installed-app tasks remain blocked until the bundle_handoff pack/unpack/merge procedure is completed for the recorded machine pair."
            )
        )
        XCTAssertTrue(verdict?.preflightError?.contains("bundle_handoff pack/unpack/merge procedure") == true)
    }

    func testGateBlocksContradictoryInstalledAppProofWhenVerifiedHandoffConflictsWithRecordedPair() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(installedAppCollectionProof: makeContradictoryInstalledAppCollectionProof())
        )

        XCTAssertEqual(
            verdict,
            .installedAppProofBlocked(
                machine: "source",
                detail: "Current acceptance bundle already contains contradictory archive handoff evidence: bundle_handoffs contain verified cross-machine archive handoff evidence for machine ids other than the recorded source/target pair. Distinct-machine installed-app tasks remain blocked until the bundle is corrected."
            )
        )
        XCTAssertTrue(verdict?.preflightError?.contains("contradictory archive handoff evidence") == true)
    }

    func testGateBlocksWhenMachineFactMetadataDisagreesWithArtifacts() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(installedAppCollectionProof: makeMachineFactMetadataMismatchProof())
        )

        XCTAssertEqual(
            verdict,
            .installedAppProofBlocked(
                machine: "source",
                detail: "Current acceptance bundle machine fact metadata does not agree with source.machine.json and target.machine.json. Distinct-machine installed-app tasks remain blocked until the bundle is corrected."
            )
        )
    }

    func testGateBlocksWhenRoleMachineIDsConflictWithMachineFactEvidence() {
        let verdict = AcceptanceInstalledAppLaunchGate.evaluate(
            makeInput(installedAppCollectionProof: makeConflictingRoleAndMachineFactProof())
        )

        XCTAssertEqual(
            verdict,
            .installedAppProofBlocked(
                machine: "source",
                detail: "Current acceptance bundle records conflicting role machine ids and machine fact evidence. Distinct-machine installed-app tasks remain blocked until the bundle is corrected."
            )
        )
    }

    func testReadyPreviewBlocksWhenLocalNotarizationCheckFlagsIssue() {
        let preview = AcceptanceInstalledAppLaunchGate.Verdict
            .ready(machine: "source", auditReadiness: "distribution_ready")
            .preview(localNotarizationBlockingDetail: "Loaded source packaging audit is accepted, but local source notarization evidence is malformed and launch will fail closed until it is replaced.")

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("local source notarization evidence is malformed"))
    }

    func testFinalEvaluationPendingPreviewRemainsReview() {
        let preview = AcceptanceInstalledAppLaunchGate.Verdict
            .finalEvaluationPending(
                machine: "source",
                detail: "current evaluation pending"
            )
            .preview()

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .review)
        XCTAssertEqual(preview.detail, "current evaluation pending")
    }

    func testRequiresPackagedAppPreviewRemainsBlocked() {
        let preview = AcceptanceInstalledAppLaunchGate.Verdict
            .requiresPackagedApp(machine: "source", mode: .development, readiness: "development launcher")
            .preview(localNotarizationBlockingDetail: "ignored")

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("require a packaged app"))
    }

    private func makeInput(
        machine: String = "source",
        bundleState: AcceptanceInstalledAppLaunchGate.BundleState = .loaded(collectionMode: "two_machine"),
        cliMode: CLIProvenance.Mode = .bundled,
        readiness: String = "distribution_ready",
        includeAppAudit: Bool = true,
        appAudit: AcceptanceBundleSnapshot.AppAuditArtifact? = nil,
        includeNotarizationArtifact: Bool = true,
        notarizationArtifact: AcceptanceBundleSnapshot.NotarizationArtifact? = nil,
        installedAppCollectionProof: AcceptanceInstalledAppCollectionProofSummary? = nil,
        hasCurrentEvaluationPassState: Bool = false
    ) -> AcceptanceInstalledAppLaunchGate.Input {
        let resolvedAppAudit = includeAppAudit ? (appAudit ?? makeAppAudit()) : nil
        let resolvedNotarization = includeNotarizationArtifact
            ? (notarizationArtifact ?? makeNotarizationArtifact(notaryLogPath: "\(machine).notary-log.json"))
            : nil
        return AcceptanceInstalledAppLaunchGate.Input(
            machine: machine,
            bundleState: bundleState,
            cliProvenance: makeCLIProvenance(mode: cliMode, readiness: readiness),
            installedAppCollectionProof: installedAppCollectionProof ?? makeReadyInstalledAppCollectionProof(),
            releaseEvidenceMachine: makeReleaseEvidenceMachine(
                machine: machine,
                appAudit: resolvedAppAudit,
                notarizationArtifact: resolvedNotarization
            ),
            hasCurrentEvaluationPassState: hasCurrentEvaluationPassState
        )
    }

    private func makeReleaseEvidenceMachine(
        machine: String,
        appAudit: AcceptanceBundleSnapshot.AppAuditArtifact?,
        notarizationArtifact: AcceptanceBundleSnapshot.NotarizationArtifact?
    ) -> AcceptanceInstalledAppReleaseEvidenceSummary.Machine {
        let provenance = appAudit?.provenance?.manifest
        let bundleRoot = makeBundleRootWithNotaryLogIfNeeded(
            machine: machine,
            notarizationArtifact: notarizationArtifact
        )
        let snapshot: AcceptanceBundleLoadedSnapshot
        switch machine {
        case "source":
            snapshot = makeReleaseEvidenceSnapshot(
                bundleRootPath: bundleRoot.path,
                sourceProvenanceArtifact: provenance,
                sourceAppAuditArtifact: appAudit,
                sourceNotarizationArtifact: notarizationArtifact
            )
        case "target":
            snapshot = makeReleaseEvidenceSnapshot(
                bundleRootPath: bundleRoot.path,
                targetProvenanceArtifact: provenance,
                targetAppAuditArtifact: appAudit,
                targetNotarizationArtifact: notarizationArtifact
            )
        default:
            fatalError("Unsupported machine \(machine)")
        }

        let summary = AcceptanceInstalledAppReleaseEvidenceSummary.evaluate(snapshot: snapshot)
        return machine == "source" ? summary.source : summary.target
    }

    private func makeBundleRootWithNotaryLogIfNeeded(
        machine: String,
        notarizationArtifact: AcceptanceBundleSnapshot.NotarizationArtifact?
    ) -> URL {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent(
            "acceptance-launch-gate-\(UUID().uuidString)",
            isDirectory: true
        )
        try? FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
        temporaryBundleRoots.append(url)
        guard
            let notaryLogPath = notarizationArtifact?.notary_log?.path,
            !notaryLogPath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
            !notaryLogPath.hasPrefix("/")
        else {
            return url
        }
        try? AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(
            to: url.appendingPathComponent(notaryLogPath),
            atomically: true,
            encoding: .utf8
        )
        return url
    }

    private func makeReleaseEvidenceSnapshot(
        bundleRootPath: String = "/tmp/acceptance-bundle",
        sourceProvenanceArtifact: AcceptanceBundleSnapshot.ProvenanceArtifact? = nil,
        targetProvenanceArtifact: AcceptanceBundleSnapshot.ProvenanceArtifact? = nil,
        sourceAppAuditArtifact: AcceptanceBundleSnapshot.AppAuditArtifact? = nil,
        targetAppAuditArtifact: AcceptanceBundleSnapshot.AppAuditArtifact? = nil,
        sourceNotarizationArtifact: AcceptanceBundleSnapshot.NotarizationArtifact? = nil,
        targetNotarizationArtifact: AcceptanceBundleSnapshot.NotarizationArtifact? = nil
    ) -> AcceptanceBundleLoadedSnapshot {
        AcceptanceBundleLoadedSnapshot(
            bundleRootPath: bundleRootPath,
            meta: AcceptanceBundleSnapshot(
                schema: "supermover.acceptance.two_machine.v1",
                status: "in_progress",
                collection: .init(mode: "two_machine", machine_count: 2),
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
            sourceProvenanceArtifact: sourceProvenanceArtifact,
            targetProvenanceArtifact: targetProvenanceArtifact,
            sourceAppAuditArtifact: sourceAppAuditArtifact,
            targetAppAuditArtifact: targetAppAuditArtifact,
            sourceNotarizationArtifact: sourceNotarizationArtifact,
            targetNotarizationArtifact: targetNotarizationArtifact,
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

    private func makeReadyInstalledAppCollectionProof() -> AcceptanceInstalledAppCollectionProofSummary {
        AcceptanceInstalledAppCollectionProofSummary(
            collectionMode: "two_machine",
            machineCount: 2,
            roleMachineIDs: ["source": "source-machine", "target": "target-machine"],
            machineFactIDs: ["source": "source-machine", "target": "target-machine"],
            machineFactArtifactIDs: ["source": "source-machine", "target": "target-machine"],
            collectionOK: true,
            roleMachineIDsPresent: true,
            roleMachineIDsDistinct: true,
            machineFactArtifactsPresent: true,
            machineFactArtifactsSchemaOK: true,
            machineFactArtifactIDsPresent: true,
            machineFactArtifactIDsDistinct: true,
            verifiedBundleHandoffs: 1,
            verifiedCrossMachineBundleHandoffs: 1,
            matchesRecordedMachinePair: true,
            machineFactsConsistent: true,
            installedAppMachinePair: .init(source: "source-machine", target: "target-machine"),
            hasInstalledAppMachinePairProof: true,
            primaryFailure: nil,
            failureMessage: nil,
            ok: true,
            failures: []
        )
    }

    private func makeIncompleteInstalledAppCollectionProof() -> AcceptanceInstalledAppCollectionProofSummary {
        AcceptanceInstalledAppCollectionProofSummary(
            collectionMode: "two_machine",
            machineCount: 2,
            roleMachineIDs: ["source": nil, "target": nil],
            machineFactIDs: ["source": nil, "target": nil],
            machineFactArtifactIDs: ["source": nil, "target": nil],
            collectionOK: true,
            roleMachineIDsPresent: false,
            roleMachineIDsDistinct: false,
            machineFactArtifactsPresent: false,
            machineFactArtifactsSchemaOK: false,
            machineFactArtifactIDsPresent: false,
            machineFactArtifactIDsDistinct: false,
            verifiedBundleHandoffs: 0,
            verifiedCrossMachineBundleHandoffs: 0,
            matchesRecordedMachinePair: false,
            machineFactsConsistent: true,
            installedAppMachinePair: nil,
            hasInstalledAppMachinePairProof: false,
            primaryFailure: "missing_role_machine_ids",
            failureMessage: "missing source/target role machine ids",
            ok: false,
            failures: [
                "missing_role_machine_ids",
                "missing_machine_fact_artifacts",
                "missing_verified_bundle_handoffs",
                "handoff_does_not_match_recorded_machine_pair",
            ]
        )
    }

    private func makeUnmatchedInstalledAppCollectionProof() -> AcceptanceInstalledAppCollectionProofSummary {
        AcceptanceInstalledAppCollectionProofSummary(
            collectionMode: "two_machine",
            machineCount: 2,
            roleMachineIDs: ["source": "source-machine", "target": "target-machine"],
            machineFactIDs: ["source": "source-machine", "target": "target-machine"],
            machineFactArtifactIDs: ["source": "source-machine", "target": "target-machine"],
            collectionOK: true,
            roleMachineIDsPresent: true,
            roleMachineIDsDistinct: true,
            machineFactArtifactsPresent: true,
            machineFactArtifactsSchemaOK: true,
            machineFactArtifactIDsPresent: true,
            machineFactArtifactIDsDistinct: true,
            verifiedBundleHandoffs: 1,
            verifiedCrossMachineBundleHandoffs: 0,
            matchesRecordedMachinePair: false,
            machineFactsConsistent: true,
            installedAppMachinePair: .init(source: "source-machine", target: "target-machine"),
            hasInstalledAppMachinePairProof: true,
            primaryFailure: "handoff_does_not_match_recorded_machine_pair",
            failureMessage: "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids",
            ok: false,
            failures: ["handoff_does_not_match_recorded_machine_pair"]
        )
    }

    private func makeContradictoryInstalledAppCollectionProof() -> AcceptanceInstalledAppCollectionProofSummary {
        AcceptanceInstalledAppCollectionProofSummary(
            collectionMode: "two_machine",
            machineCount: 2,
            roleMachineIDs: ["source": "source-machine", "target": "target-machine"],
            machineFactIDs: ["source": "source-machine", "target": "target-machine"],
            machineFactArtifactIDs: ["source": "source-machine", "target": "target-machine"],
            collectionOK: true,
            roleMachineIDsPresent: true,
            roleMachineIDsDistinct: true,
            machineFactArtifactsPresent: true,
            machineFactArtifactsSchemaOK: true,
            machineFactArtifactIDsPresent: true,
            machineFactArtifactIDsDistinct: true,
            verifiedBundleHandoffs: 2,
            verifiedCrossMachineBundleHandoffs: 1,
            matchesRecordedMachinePair: false,
            machineFactsConsistent: true,
            installedAppMachinePair: .init(source: "source-machine", target: "target-machine"),
            hasInstalledAppMachinePairProof: true,
            primaryFailure: "contradictory_verified_bundle_handoffs",
            failureMessage: "bundle_handoffs contain verified cross-machine archive handoff evidence for machine ids other than the recorded source/target pair",
            ok: false,
            failures: ["contradictory_verified_bundle_handoffs"]
        )
    }

    private func makeMachineFactMetadataMismatchProof() -> AcceptanceInstalledAppCollectionProofSummary {
        AcceptanceInstalledAppCollectionProofSummary(
            collectionMode: "two_machine",
            machineCount: 2,
            roleMachineIDs: ["source": "source-machine", "target": "target-machine"],
            machineFactIDs: ["source": "other-source-machine", "target": "other-target-machine"],
            machineFactArtifactIDs: ["source": "source-machine", "target": "target-machine"],
            collectionOK: true,
            roleMachineIDsPresent: true,
            roleMachineIDsDistinct: true,
            machineFactArtifactsPresent: true,
            machineFactArtifactsSchemaOK: true,
            machineFactArtifactIDsPresent: true,
            machineFactArtifactIDsDistinct: true,
            verifiedBundleHandoffs: 0,
            verifiedCrossMachineBundleHandoffs: 0,
            matchesRecordedMachinePair: false,
            machineFactsConsistent: false,
            installedAppMachinePair: nil,
            hasInstalledAppMachinePairProof: false,
            primaryFailure: "missing_verified_bundle_handoffs",
            failureMessage: "missing verified bundle_handoffs",
            ok: false,
            failures: ["missing_verified_bundle_handoffs", "handoff_does_not_match_recorded_machine_pair"]
        )
    }

    private func makeConflictingRoleAndMachineFactProof() -> AcceptanceInstalledAppCollectionProofSummary {
        AcceptanceInstalledAppCollectionProofSummary(
            collectionMode: "two_machine",
            machineCount: 2,
            roleMachineIDs: ["source": "source-machine", "target": "target-machine"],
            machineFactIDs: ["source": "other-source-machine", "target": "other-target-machine"],
            machineFactArtifactIDs: ["source": "other-source-machine", "target": "other-target-machine"],
            collectionOK: true,
            roleMachineIDsPresent: true,
            roleMachineIDsDistinct: true,
            machineFactArtifactsPresent: true,
            machineFactArtifactsSchemaOK: true,
            machineFactArtifactIDsPresent: true,
            machineFactArtifactIDsDistinct: true,
            verifiedBundleHandoffs: 0,
            verifiedCrossMachineBundleHandoffs: 0,
            matchesRecordedMachinePair: false,
            machineFactsConsistent: true,
            installedAppMachinePair: nil,
            hasInstalledAppMachinePairProof: false,
            primaryFailure: "missing_verified_bundle_handoffs",
            failureMessage: "missing verified bundle_handoffs",
            ok: false,
            failures: ["missing_verified_bundle_handoffs", "handoff_does_not_match_recorded_machine_pair"]
        )
    }

    private func makeCLIProvenance(
        mode: CLIProvenance.Mode,
        readiness: String
    ) -> CLIProvenance {
        CLIProvenance(
            mode: mode,
            readinessLevel: .pass,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1.0",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: readiness,
            detail: readiness
        )
    }

    private func makeAppAudit(
        status: String = "pass",
        readiness: String = "distribution_ready",
        passReady: Bool = true,
        appPath: String? = "/tmp/SuperMover.app"
    ) -> AcceptanceBundleSnapshot.AppAuditArtifact {
        AcceptanceBundleSnapshot.AppAuditArtifact(
            schema: "supermover.macos.app_audit.v1",
            status: status,
            readiness: readiness,
            app_path: appPath,
            provenance: .init(
                path: "/tmp/SuperMover.app/Contents/Resources/supermover-provenance.json",
                manifest: .init(
                    schema: "supermover.macos.provenance.v1",
                    app_bundle_id: "dev.supermover.macapp",
                    app_version: "0.1.0",
                    build_profile: "test",
                    git_commit: "abcdef",
                    git_dirty: false,
                    cli_version: "supermover 0.1.0-dev",
                    cli_relative_path: "Contents/Resources/bin/supermover",
                    built_at: "2026-06-03T00:00:00Z",
                    signing: "developer-id"
                )
            ),
            summary: .init(
                pass_ready: passReady,
                blocking_checks: 0
            )
        )
    }

    private func makeNotarizationArtifact(
        status: String = "pass",
        submissionStatus: String = "Accepted",
        auditStatus: String = "pass",
        auditReadiness: String = "distribution_ready",
        auditPassReady: Bool = true,
        notaryLogPath: String = "source.notary-log.json"
    ) -> AcceptanceBundleSnapshot.NotarizationArtifact {
        let appPath = "/tmp/SuperMover.app"
        return AcceptanceBundleSnapshot.NotarizationArtifact(
            schema: "supermover.macos.notarization.v1",
            checked_at: "2026-06-01T00:00:00Z",
            status: status,
            app_path: appPath,
            work_dir: "/tmp/notary",
            auth_mode: "keychain_profile",
            archive_path: "/tmp/notary/SuperMover.app.zip",
            submission: .init(
                id: "11111111-1111-1111-1111-111111111111",
                status: submissionStatus,
                message: nil,
                path: nil
            ),
            notary_log: .init(path: notaryLogPath),
            audit: .init(
                path: "\(appPath).notary/post-staple.audit.json",
                status: auditStatus,
                readiness: auditReadiness,
                pass_ready: auditPassReady,
                blocking_checks: 0
            ),
            failure: nil
        )
    }
}
