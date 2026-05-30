import XCTest
@testable import SuperMoverApp

final class AppStoreTests: XCTestCase {
    func testDiscoveryAndPairingCommandsUseExplicitInputs() {
        let input = TaskInput(
            profilePath: "/tmp/profile.json",
            sourceRootPath: "/tmp/source",
            targetRootPath: "/tmp/target",
            profileID: "profile-local",
            profileName: "Local profile",
            targetID: "",
            targetName: "",
            sessionID: "",
            sessionPrefix: "",
            queueEntryID: "",
            syncRetryBackoff: "1m",
            syncInterval: "1m",
            syncMaxRuns: "0",
            syncSettle: "250ms",
            syncMaxEvents: "0",
            syncDiscoveryListen: "0.0.0.0:39394",
            syncDiscoveryTimeout: "2s",
            listenAddress: "127.0.0.1:4000",
            pairingTargetAddress: "10.0.0.20:39395",
            pairingVerificationCode: "123456",
            pairingMethod: "sas",
            pairingTimeout: "7s",
            discoveryBrowseListen: "0.0.0.0:39394",
            discoveryBrowseTimeout: "3s",
            discoveryAdvertiseListen: "0.0.0.0:0",
            discoveryAdvertiseDestination: "255.255.255.255:39394",
            discoveryAdvertiseDuration: "11s",
            discoveryAdvertiseInterval: "2s",
            driftIDsInput: "",
            approvalID: "",
            softDeleteIDsInput: "",
            expiresAt: "",
            reason: "",
            reviewer: ""
        )

        XCTAssertEqual(
            SuperMoverTaskKind.discoverBrowse.buildArguments(using: input),
            ["discover", "browse", "--listen", "0.0.0.0:39394", "--timeout", "3s", "--format", "json"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.discoverAddress.buildArguments(using: input),
            ["discover", "--address", "10.0.0.20:39395", "--timeout", "3s", "--format", "json"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.discoverAdvertise.buildArguments(using: input),
            ["discover", "advertise", "--profile", "/tmp/profile.json", "--listen", "0.0.0.0:0", "--dest", "255.255.255.255:39394", "--interval", "2s", "--duration", "11s", "--format", "json"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.pair.buildArguments(using: input),
            ["pair", "--profile", "/tmp/profile.json", "--target", "10.0.0.20:39395", "--verification-code", "123456", "--method", "sas", "--timeout", "7s"]
        )
    }

    func testDiscoverAdvertiseOmitsListenWhenUnset() {
        let input = TaskInput(
            profilePath: "/tmp/profile.json",
            sourceRootPath: "",
            targetRootPath: "",
            profileID: "",
            profileName: "",
            targetID: "",
            targetName: "",
            sessionID: "",
            sessionPrefix: "",
            queueEntryID: "",
            syncRetryBackoff: "1m",
            syncInterval: "1m",
            syncMaxRuns: "0",
            syncSettle: "250ms",
            syncMaxEvents: "0",
            syncDiscoveryListen: "0.0.0.0:39394",
            syncDiscoveryTimeout: "2s",
            listenAddress: "127.0.0.1:4000",
            pairingTargetAddress: "",
            pairingVerificationCode: "",
            pairingMethod: "",
            pairingTimeout: "",
            discoveryBrowseListen: "",
            discoveryBrowseTimeout: "",
            discoveryAdvertiseListen: "",
            discoveryAdvertiseDestination: "239.1.2.3:39394",
            discoveryAdvertiseDuration: "4s",
            discoveryAdvertiseInterval: "1s",
            driftIDsInput: "",
            approvalID: "",
            softDeleteIDsInput: "",
            expiresAt: "",
            reason: "",
            reviewer: ""
        )

        XCTAssertEqual(
            SuperMoverTaskKind.discoverAdvertise.buildArguments(using: input),
            ["discover", "advertise", "--profile", "/tmp/profile.json", "--dest", "239.1.2.3:39394", "--interval", "1s", "--duration", "4s", "--format", "json"]
        )
    }

    func testPairingTasksAreRoleGated() {
        XCTAssertTrue(WorkbenchRole.source.allows(task: .discoverBrowse))
        XCTAssertTrue(WorkbenchRole.source.allows(task: .discoverAddress))
        XCTAssertTrue(WorkbenchRole.source.allows(task: .pair))
        XCTAssertFalse(WorkbenchRole.source.allows(task: .discoverAdvertise))

        XCTAssertTrue(WorkbenchRole.target.allows(task: .discoverAdvertise))
        XCTAssertTrue(WorkbenchRole.target.allows(task: .serve))
        XCTAssertFalse(WorkbenchRole.target.allows(task: .pair))

        XCTAssertFalse(WorkbenchRole.observer.allows(task: .discoverBrowse))
        XCTAssertFalse(WorkbenchRole.observer.allows(task: .discoverAdvertise))
        XCTAssertFalse(WorkbenchRole.observer.allows(task: .pair))
    }

    func testSyncCommandsUseProfileSSOTAndExplicitRunInputs() {
        let input = TaskInput(
            profilePath: "/tmp/profile.json",
            sourceRootPath: "/tmp/source",
            targetRootPath: "/tmp/target",
            profileID: "profile-local",
            profileName: "Local profile",
            targetID: "",
            targetName: "",
            sessionID: "sync-run-1",
            sessionPrefix: "sync-loop",
            queueEntryID: "entry-1",
            syncRetryBackoff: "45s",
            syncInterval: "5s",
            syncMaxRuns: "2",
            syncSettle: "100ms",
            syncMaxEvents: "3",
            syncDiscoveryListen: "127.0.0.1:0",
            syncDiscoveryTimeout: "250ms",
            listenAddress: "127.0.0.1:4000",
            pairingTargetAddress: "",
            pairingVerificationCode: "",
            pairingMethod: "",
            pairingTimeout: "",
            discoveryBrowseListen: "",
            discoveryBrowseTimeout: "",
            discoveryAdvertiseListen: "",
            discoveryAdvertiseDestination: "",
            discoveryAdvertiseDuration: "",
            discoveryAdvertiseInterval: "",
            driftIDsInput: "",
            approvalID: "",
            softDeleteIDsInput: "",
            expiresAt: "",
            reason: "operator review",
            reviewer: ""
        )

        XCTAssertEqual(
            SuperMoverTaskKind.syncQueueCancel.buildArguments(using: input),
            ["sync", "queue", "cancel", "--profile", "/tmp/profile.json", "--id", "entry-1", "--reason", "operator review", "--format", "json"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.syncRun.buildArguments(using: input),
            ["sync", "run", "--profile", "/tmp/profile.json", "--session", "sync-run-1", "--retry-backoff", "45s", "--format", "json"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.syncLoop.buildArguments(using: input),
            ["sync", "loop", "--profile", "/tmp/profile.json", "--session-prefix", "sync-loop", "--interval", "5s", "--max-runs", "2", "--retry-backoff", "45s", "--format", "json"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.syncWatch.buildArguments(using: input),
            ["sync", "watch", "--profile", "/tmp/profile.json", "--session-prefix", "sync-loop", "--settle", "100ms", "--max-events", "3", "--retry-backoff", "45s", "--format", "json"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.syncNetworkDiscoverRun.buildArguments(using: input),
            ["sync", "network", "discover-run", "--profile", "/tmp/profile.json", "--session", "sync-run-1", "--listen", "127.0.0.1:0", "--timeout", "250ms", "--retry-backoff", "45s", "--format", "json"]
        )
    }

    func testSyncTasksAreRoleGatedAndLongRunningSlotsAreSeparate() {
        XCTAssertTrue(WorkbenchRole.source.allows(task: .syncQueueEnqueue))
        XCTAssertTrue(WorkbenchRole.source.allows(task: .syncQueueCancel))
        XCTAssertTrue(WorkbenchRole.source.allows(task: .syncRun))
        XCTAssertTrue(WorkbenchRole.source.allows(task: .syncWatch))
        XCTAssertTrue(WorkbenchRole.source.allows(task: .syncNetworkLoop))

        XCTAssertTrue(WorkbenchRole.target.allows(task: .syncQueueStatus))
        XCTAssertTrue(WorkbenchRole.target.allows(task: .syncQueueList))
        XCTAssertTrue(WorkbenchRole.target.allows(task: .syncQueueReady))
        XCTAssertFalse(WorkbenchRole.target.allows(task: .syncRun))
        XCTAssertFalse(WorkbenchRole.target.allows(task: .syncQueueCancel))

        XCTAssertTrue(WorkbenchRole.observer.allows(task: .syncQueueStatus))
        XCTAssertFalse(WorkbenchRole.observer.allows(task: .syncNetworkRun))

        XCTAssertTrue(SuperMoverTaskKind.syncLoop.longRunning)
        XCTAssertEqual(SuperMoverTaskKind.syncLoop.supervisedSlot, .sourceSyncLoop)
        XCTAssertTrue(SuperMoverTaskKind.syncWatch.longRunning)
        XCTAssertEqual(SuperMoverTaskKind.syncWatch.supervisedSlot, .sourceSyncWatch)
        XCTAssertTrue(SuperMoverTaskKind.syncNetworkLoop.longRunning)
        XCTAssertEqual(SuperMoverTaskKind.syncNetworkLoop.supervisedSlot, .sourceNetworkLoop)
    }

    @MainActor
    func testDriftExpireCommandAndReviewMutationRoleGates() {
        var input = taskInput(profilePath: "/tmp/profile.json")
        input.driftIDsInput = " drift-1 "
        input.reason = " stale review evidence "
        input.reviewer = " ops "

        XCTAssertEqual(
            SuperMoverTaskKind.driftExpire.buildArguments(using: input),
            ["drift", "expire", "--profile", "/tmp/profile.json", "--id", "drift-1", "--reason", "stale review evidence", "--format", "json", "--reviewer", "ops"]
        )

        let store = AppStore()
        store.profilePath = "/tmp/profile.json"
        store.driftIDsInput = "drift-1"
        store.reason = "stale review evidence"
        store.reviewer = "ops"
        let firstContext = store.currentContextSignature(for: .driftExpire)
        store.driftIDsInput = "drift-2"
        XCTAssertNotEqual(store.currentContextSignature(for: .driftExpire), firstContext)

        let reviewMutations: [SuperMoverTaskKind] = [
            .driftRecord,
            .driftAcknowledge,
            .driftResolve,
            .driftExpire,
            .syncQueueCancel,
            .syncQueueFail,
            .pruneApprove,
            .pruneSupersede,
        ]
        for task in reviewMutations {
            XCTAssertTrue(WorkbenchRole.source.allows(task: task), "\(task)")
            XCTAssertFalse(WorkbenchRole.target.allows(task: task), "\(task)")
            XCTAssertFalse(WorkbenchRole.observer.allows(task: task), "\(task)")
        }
    }

    func testDevelopmentCLIResolverUsesBuildThenExecLauncher() throws {
        let cliArguments = ["sync", "run", "--session", "sync id; $(unsafe)"]
        let invocation = try CLIResolver.resolve(arguments: cliArguments)
        if invocation.executableURL.lastPathComponent == "supermover" {
            throw XCTSkip("Packaged CLI binary is present; development launcher is not used.")
        }

        XCTAssertEqual(invocation.executableURL.path, "/bin/sh")
        XCTAssertGreaterThanOrEqual(invocation.arguments.count, 5)
        XCTAssertEqual(invocation.arguments[0], "-c")
        XCTAssertTrue(invocation.arguments[1].contains("trap terminate_child INT TERM"))
        XCTAssertTrue(invocation.arguments[1].contains("kill \"$child\""))
        XCTAssertTrue(invocation.arguments[1].contains("go build -o \"$binary\" ./cmd/supermover &"))
        XCTAssertTrue(invocation.arguments[1].contains("exec \"$binary\" \"$@\""))
        XCTAssertEqual(invocation.arguments[2], "supermover-dev-launch")
        XCTAssertTrue(invocation.arguments[3].hasSuffix(".tmp/macos-app/supermover-dev"))
        XCTAssertEqual(Array(invocation.arguments.suffix(cliArguments.count)), cliArguments)
    }

    func testCLIProvenanceReportsRunnableMode() {
        let provenance = CLIResolver.provenance()

        XCTAssertFalse(provenance.bundleIdentifier.isEmpty)
        XCTAssertFalse(provenance.appVersion.isEmpty)
        XCTAssertFalse(provenance.executablePath.isEmpty)
        XCTAssertFalse(provenance.workingDirectoryPath.isEmpty)
        switch provenance.mode {
        case .bundled:
            XCTAssertTrue(provenance.executablePath.hasSuffix("Contents/Resources/bin/supermover") || provenance.executablePath.hasSuffix("bin/supermover"))
            XCTAssertNotEqual(provenance.provenanceStatus, "missing")
            XCTAssertNotEqual(provenance.readinessLevel, .blocked)
        case .development:
            XCTAssertTrue(provenance.executablePath.hasSuffix(".tmp/macos-app/supermover-dev"))
            XCTAssertEqual(provenance.readiness, "development launcher")
            XCTAssertEqual(provenance.readinessLevel, .review)
        case .unavailable:
            XCTFail("test workspace should expose either bundled or development CLI provenance")
        }
    }

    func testPackagedCLIProvenanceBlocksMissingCLIWithoutDevelopmentFallback() throws {
        let resourceURL = try makeTemporaryDirectory()
        let repoRoot = try makeTemporaryDirectory()
        defer {
            try? FileManager.default.removeItem(at: resourceURL)
            try? FileManager.default.removeItem(at: repoRoot)
        }
        try writeProvenanceManifest(
            in: resourceURL,
            signing: "Developer ID Application: Example",
            gitDirty: false
        )

        let provenance = CLIResolver.provenance(
            resourceURL: resourceURL,
            bundleIdentifier: "dev.supermover.macapp",
            appVersion: "0.1.0",
            isPackagedApp: true,
            repoRoot: repoRoot
        )

        XCTAssertEqual(provenance.mode, .unavailable)
        XCTAssertEqual(provenance.readinessLevel, .blocked)
        XCTAssertEqual(provenance.readiness, "missing bundled CLI")
        XCTAssertTrue(provenance.detail.contains("will not fall back"))

        XCTAssertThrowsError(
            try CLIResolver.resolve(
                arguments: ["version"],
                resourceURL: resourceURL,
                isPackagedApp: true,
                repoRoot: repoRoot
            )
        ) { error in
            guard case SuperMoverCLIError.bundledBinaryMissing = error else {
                return XCTFail("expected bundledBinaryMissing, got \(error)")
            }
        }
    }

    func testPackagedCLIProvenanceGatesMalformedUnsignedAdHocAndDirtyBundles() throws {
        let resourceURL = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: resourceURL) }
        try makeExecutableBundledCLI(in: resourceURL)

        try "not-json".write(to: resourceURL.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        var provenance = CLIResolver.provenance(
            resourceURL: resourceURL,
            bundleIdentifier: "dev.supermover.macapp",
            appVersion: "0.1.0",
            isPackagedApp: true,
            repoRoot: nil
        )
        XCTAssertEqual(provenance.readinessLevel, .blocked)
        XCTAssertEqual(provenance.readiness, "provenance unavailable")
        XCTAssertTrue(provenance.provenanceStatus.hasPrefix("malformed"))

        try writeProvenanceManifest(in: resourceURL, signing: nil, gitDirty: false)
        provenance = CLIResolver.provenance(resourceURL: resourceURL, bundleIdentifier: "dev.supermover.macapp", appVersion: "0.1.0", isPackagedApp: true, repoRoot: nil)
        XCTAssertEqual(provenance.readinessLevel, .blocked)
        XCTAssertEqual(provenance.readiness, "provenance incomplete")

        try writeProvenanceManifest(in: resourceURL, signing: "Developer ID Application: Example", gitDirty: false, cliRelativePath: nil)
        provenance = CLIResolver.provenance(resourceURL: resourceURL, bundleIdentifier: "dev.supermover.macapp", appVersion: "0.1.0", isPackagedApp: true, repoRoot: nil)
        XCTAssertEqual(provenance.readinessLevel, .blocked)
        XCTAssertTrue(provenance.detail.contains("cli_relative_path"))

        try writeProvenanceManifest(in: resourceURL, signing: "unsigned", gitDirty: false)
        provenance = CLIResolver.provenance(resourceURL: resourceURL, bundleIdentifier: "dev.supermover.macapp", appVersion: "0.1.0", isPackagedApp: true, repoRoot: nil)
        XCTAssertEqual(provenance.readinessLevel, .review)
        XCTAssertEqual(provenance.readiness, "local bundle review")

        try writeProvenanceManifest(in: resourceURL, signing: "-", gitDirty: false)
        provenance = CLIResolver.provenance(resourceURL: resourceURL, bundleIdentifier: "dev.supermover.macapp", appVersion: "0.1.0", isPackagedApp: true, repoRoot: nil)
        XCTAssertEqual(provenance.readinessLevel, .review)
        XCTAssertTrue(provenance.detail.contains("signing is -"))

        try writeProvenanceManifest(in: resourceURL, signing: "Developer ID Application: Example", gitDirty: true)
        provenance = CLIResolver.provenance(resourceURL: resourceURL, bundleIdentifier: "dev.supermover.macapp", appVersion: "0.1.0", isPackagedApp: true, repoRoot: nil)
        XCTAssertEqual(provenance.readinessLevel, .review)
        XCTAssertEqual(provenance.gitDirty, true)

        try writeProvenanceManifest(in: resourceURL, signing: "Developer ID Application: Example", gitDirty: false)
        provenance = CLIResolver.provenance(resourceURL: resourceURL, bundleIdentifier: "dev.supermover.macapp", appVersion: "0.1.0", isPackagedApp: true, repoRoot: nil)
        XCTAssertEqual(provenance.readinessLevel, .pass)
        XCTAssertEqual(provenance.readiness, "ready")
    }

    func testVersionAndForegroundDaemonCommandsUseExplicitBoundary() {
        var input = taskInput(profilePath: "/tmp/profile.json")
        input.listenAddress = "127.0.0.1:39999"
        input.reason = "operator requested restart"

        XCTAssertEqual(SuperMoverTaskKind.version.buildArguments(using: input), ["version"])
        XCTAssertEqual(
            SuperMoverTaskKind.daemonInstall.buildArguments(using: input),
            ["daemon", "install", "--profile", "/tmp/profile.json"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.daemonRun.buildArguments(using: input),
            ["daemon", "run", "--foreground", "--profile", "/tmp/profile.json", "--listen", "127.0.0.1:39999"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.daemonRestart.buildArguments(using: input),
            ["daemon", "restart", "--profile", "/tmp/profile.json", "--reason", "operator requested restart", "--format", "json"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.daemonStop.buildArguments(using: input),
            ["daemon", "stop", "--profile", "/tmp/profile.json", "--reason", "operator requested restart"]
        )

        XCTAssertTrue(SuperMoverTaskKind.daemonRun.longRunning)
        XCTAssertEqual(SuperMoverTaskKind.daemonRun.supervisedSlot, .foregroundDaemon)
        XCTAssertTrue(WorkbenchRole.source.allows(task: .daemonRun))
        XCTAssertTrue(WorkbenchRole.target.allows(task: .daemonRun))
        XCTAssertFalse(WorkbenchRole.observer.allows(task: .daemonRun))
        XCTAssertTrue(WorkbenchRole.observer.allows(task: .version))
    }

    func testVerifyCommandRequestsJSONAndOptionalSession() {
        let input = taskInput(profilePath: "/tmp/profile.json", sessionID: "session-1")

        XCTAssertEqual(
            SuperMoverTaskKind.verify.buildArguments(using: input),
            ["verify", "--profile", "/tmp/profile.json", "--format", "json", "--session", "session-1"]
        )
    }

    func testVerifySnapshotDecodesManifestAndReviewEvidence() throws {
        let verify = try decodeFixture(VerifySnapshot.self, from: verifyReviewJSON())

        XCTAssertEqual(verify.target_root, "/tmp/target")
        XCTAssertEqual(verify.manifest.manifestID, "manifest-1")
        XCTAssertEqual(verify.manifest.root_id, "root-1")
        XCTAssertEqual(verify.summary.files_expected, 2)
        XCTAssertEqual(verify.summary.files_verified, 1)
        XCTAssertTrue(verify.reviewRequired)
        XCTAssertEqual(verify.statusLabel, "manifest review")
        XCTAssertEqual(verify.findings?.first?.kind, "digest_mismatch")
        XCTAssertEqual(verify.warnings?.first?.code, "digest_missing")
        XCTAssertEqual(verify.soft_deletes?.first?.source_path, "old.txt")
        XCTAssertEqual(verify.target_drifts?.first?.change, "extra")
        XCTAssertEqual(verify.artifact_problems?.first?.path, ".supermover/bad.json")
    }

    func testVerifySnapshotKeepsRootIdentitySeparateFromMerkleProof() throws {
        let verify = try decodeFixture(VerifySnapshot.self, from: verifyReviewJSON())

        XCTAssertEqual(verify.profileRootIdentity.label, "available")
        XCTAssertTrue(verify.profileRootIdentity.detail.contains("profile root identity"))
        XCTAssertEqual(verify.merkleRootProof.label, "unavailable")
        XCTAssertTrue(verify.merkleRootProof.detail.contains("No wired Merkle tree"))
        XCTAssertEqual(verify.currentSourceComparison.label, "unavailable")
    }

    func testVerifySnapshotNoManifestRequiresReviewAndRootUnavailable() throws {
        let verify = try decodeFixture(
            VerifySnapshot.self,
            from: """
            {
              "target_root": "/tmp/target",
              "manifest": {
                "id": "",
                "session_id": "",
                "created_at": "",
                "entries": 0,
                "files": 0
              },
              "summary": {
                "manifest_count": 0,
                "manifest_entries": 0,
                "files_expected": 0,
                "files_verified": 0,
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
        )

        XCTAssertTrue(verify.reviewRequired)
        XCTAssertEqual(verify.statusLabel, "no manifest")
        XCTAssertEqual(verify.profileRootIdentity.label, "unavailable")
    }

    func testEvidenceGateEvaluationKeepsVerifyOutOfTargetPreflight() {
        let verifyOnlyClean = EvidenceGateEvaluation(
            hasStatusEvidence: false,
            statusNeedsReview: false,
            hasReportEvidence: false,
            reportNeedsReview: false,
            hasHealthEvidence: false,
            healthNeedsReview: false,
            hasVerifyEvidence: true,
            verifyNeedsReview: false
        )
        XCTAssertEqual(verifyOnlyClean.targetPreflightState, .pending)
        XCTAssertEqual(verifyOnlyClean.verificationState, .pass)
        XCTAssertEqual(verifyOnlyClean.aggregateEvidenceState, .pass)

        let statusClean = EvidenceGateEvaluation(
            hasStatusEvidence: true,
            statusNeedsReview: false,
            hasReportEvidence: false,
            reportNeedsReview: false,
            hasHealthEvidence: false,
            healthNeedsReview: false,
            hasVerifyEvidence: false,
            verifyNeedsReview: false
        )
        XCTAssertEqual(statusClean.targetPreflightState, .pass)
        XCTAssertEqual(statusClean.verificationState, .review)

        let statusReview = EvidenceGateEvaluation(
            hasStatusEvidence: true,
            statusNeedsReview: true,
            hasReportEvidence: false,
            reportNeedsReview: false,
            hasHealthEvidence: false,
            healthNeedsReview: false,
            hasVerifyEvidence: false,
            verifyNeedsReview: false
        )
        XCTAssertEqual(statusReview.targetPreflightState, .review)
        XCTAssertEqual(statusReview.aggregateEvidenceState, .review)
    }

    func testEvidenceNextActionPlannerRefusesPreviewWhenMutationIntentIsMissing() {
        let planner = NextActionPlanner()

        let missingDriftID = planner.plan(
            .driftAcknowledge,
            intent: EvidenceNextAction.OperatorIntent(reason: "reviewed target drift")
        )
        XCTAssertNil(missingDriftID.commandPreview)
        XCTAssertEqual(missingDriftID.missingRequirements.map(\.field), [
            .selectedDurableEvidenceID(.persistedDrift),
        ])

        let missingDriftReason = planner.plan(
            .driftResolve,
            intent: EvidenceNextAction.OperatorIntent(selectedDurableEvidenceID: "drift-1")
        )
        XCTAssertNil(missingDriftReason.commandPreview)
        XCTAssertEqual(missingDriftReason.missingRequirements.map(\.field), [
            .reason,
        ])

        let missingQueueReason = planner.plan(
            .syncQueueCancel,
            intent: EvidenceNextAction.OperatorIntent(selectedDurableEvidenceID: "entry-1")
        )
        XCTAssertNil(missingQueueReason.commandPreview)
        XCTAssertEqual(missingQueueReason.missingRequirements.map(\.field), [
            .reason,
        ])
    }

    func testEvidenceNextActionPlannerBuildsReviewMetadataCommands() {
        let planner = NextActionPlanner()

        let expire = planner.plan(
            .driftExpire,
            intent: EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: " drift-1 ",
                selectedDurableEvidenceVerified: true,
                reason: " stale review evidence ",
                reviewer: " ops "
            )
        )
        XCTAssertEqual(expire.safety.boundary, .firstSliceMetadataPreview)
        XCTAssertEqual(expire.safety.executionBoundary, .reviewMetadataExecutable)
        XCTAssertTrue(expire.allowsExecution)
        XCTAssertEqual(expire.commandPreview?.profilePathSource, .appStoreSelectedProfile)
        XCTAssertEqual(expire.commandPreview?.arguments, [
            "drift", "expire",
            "--profile", "<AppStore.profilePath>",
            "--id", "drift-1",
            "--reason", "stale review evidence",
            "--format", "json",
            "--reviewer", "ops",
        ])

        let record = planner.plan(
            .driftRecord,
            intent: EvidenceNextAction.OperatorIntent(sessionID: " session-1 ")
        )
        XCTAssertEqual(record.commandPreview?.arguments, [
            "drift", "record",
            "--profile", "<AppStore.profilePath>",
            "--format", "json",
            "--session", "session-1",
        ])
        XCTAssertTrue(record.allowsExecution)

        let fail = planner.plan(
            .syncQueueFail,
            intent: EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: " entry-1 ",
                selectedDurableEvidenceVerified: true,
                reason: " terminal operator review "
            )
        )
        XCTAssertEqual(fail.commandPreview?.arguments, [
            "sync", "queue", "fail",
            "--profile", "<AppStore.profilePath>",
            "--id", "entry-1",
            "--reason", "terminal operator review",
            "--format", "json",
        ])
        XCTAssertTrue(fail.allowsExecution)
    }

    func testEvidenceNextActionPlannerRequiresLoadedEvidenceBeforePreviewingMutationCommands() {
        let planner = NextActionPlanner()

        let unverified = planner.plan(
            .driftAcknowledge,
            intent: EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: "drift-1",
                reason: "reviewed target drift"
            )
        )
        XCTAssertNil(unverified.commandPreview)
        XCTAssertEqual(unverified.missingRequirements.map(\.field), [
            .loadedEvidenceSelection,
        ])
        XCTAssertTrue(unverified.disabledReason?.contains("current loaded evidence selection") == true)
        XCTAssertFalse(unverified.allowsExecution)

        let skippedQueueRowID = planner.plan(
            .syncQueueFail,
            intent: EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: "root-1:.hidden/secret.txt:unchanged_digest",
                reason: "terminal operator review"
            )
        )
        XCTAssertNil(skippedQueueRowID.commandPreview)
        XCTAssertEqual(skippedQueueRowID.missingRequirements.map(\.field), [
            .loadedEvidenceSelection,
        ])
        XCTAssertFalse(skippedQueueRowID.allowsExecution)

        let verified = planner.plan(
            .driftAcknowledge,
            intent: EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: "drift-1",
                selectedDurableEvidenceVerified: true,
                reason: "reviewed target drift"
            )
        )
        XCTAssertNotNil(verified.commandPreview)
        XCTAssertTrue(verified.allowsExecution)
    }

    func testEvidenceNextActionPlannerAllowsExecutionOnlyForCompleteReviewMetadataActions() {
        let planner = NextActionPlanner()

        let allowed: [(EvidenceNextAction.Kind, EvidenceNextAction.OperatorIntent)] = [
            (.driftRecord, EvidenceNextAction.OperatorIntent()),
            (.driftAcknowledge, EvidenceNextAction.OperatorIntent(selectedDurableEvidenceID: "drift-1", selectedDurableEvidenceVerified: true, reason: "reviewed target drift")),
            (.driftResolve, EvidenceNextAction.OperatorIntent(selectedDurableEvidenceID: "drift-1", selectedDurableEvidenceVerified: true, reason: "target restored")),
            (.driftExpire, EvidenceNextAction.OperatorIntent(selectedDurableEvidenceID: "drift-1", selectedDurableEvidenceVerified: true, reason: "stale review evidence")),
            (.syncQueueCancel, EvidenceNextAction.OperatorIntent(selectedDurableEvidenceID: "queue-1", selectedDurableEvidenceVerified: true, reason: "operator canceled")),
            (.syncQueueFail, EvidenceNextAction.OperatorIntent(selectedDurableEvidenceID: "queue-1", selectedDurableEvidenceVerified: true, reason: "terminal review")),
            (.pruneApprove, EvidenceNextAction.OperatorIntent(selectedDurableEvidenceID: "soft-delete-1", selectedDurableEvidenceVerified: true, approvalID: "approval-1", reason: "reviewed prune candidate", reviewer: "ops")),
            (.pruneSupersede, EvidenceNextAction.OperatorIntent(selectedDurableEvidenceID: "approval-1", selectedDurableEvidenceVerified: true, reason: "newer approval replaces this one", reviewer: "ops")),
        ]
        for (kind, intent) in allowed {
            let action = planner.plan(kind, intent: intent)
            XCTAssertEqual(action.safety.executionBoundary, .reviewMetadataExecutable, "\(kind)")
            XCTAssertNotNil(action.commandPreview, "\(kind)")
            XCTAssertTrue(action.allowsExecution, "\(kind)")
        }

        let excluded: [EvidenceNextAction.Kind] = [
            .pruneApply,
            .reconcileApply,
            .pair,
            .publish,
            .networkPush,
            .syncRun,
            .syncLoop,
            .syncWatch,
            .syncNetworkRun,
            .syncNetworkDiscoverRun,
            .syncNetworkLoop,
        ]
        for kind in excluded {
            let action = planner.plan(kind)
            XCTAssertEqual(action.safety.boundary, .excludedFromFirstSlice)
            XCTAssertNil(action.commandPreview)
            XCTAssertFalse(action.allowsExecution)
        }
    }

    func testPruneApprovePlanningRequiresLoadedSoftDeleteAndExplicitApprovalID() {
        let planner = NextActionPlanner()

        let missingApprovalID = planner.plan(
            .pruneApprove,
            intent: EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: "soft-delete-1",
                selectedDurableEvidenceVerified: true,
                reason: "reviewed prune candidate",
                reviewer: "ops"
            )
        )
        XCTAssertNil(missingApprovalID.commandPreview)
        XCTAssertFalse(missingApprovalID.allowsExecution)
        XCTAssertEqual(missingApprovalID.missingRequirements.map(\.field), [.approvalID])

        let unloadedSoftDelete = planner.plan(
            .pruneApprove,
            intent: EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: "soft-delete-1",
                approvalID: "approval-1",
                reason: "reviewed prune candidate",
                reviewer: "ops"
            )
        )
        XCTAssertNil(unloadedSoftDelete.commandPreview)
        XCTAssertFalse(unloadedSoftDelete.allowsExecution)
        XCTAssertEqual(unloadedSoftDelete.missingRequirements.map(\.field), [.loadedEvidenceSelection])

        let ready = planner.plan(
            .pruneApprove,
            intent: EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: " soft-delete-1 ",
                selectedDurableEvidenceVerified: true,
                approvalID: " approval-1 ",
                reason: " reviewed prune candidate ",
                reviewer: " ops ",
                expiresAt: " 2026-06-01T00:00:00Z "
            )
        )
        XCTAssertTrue(ready.allowsExecution)
        XCTAssertEqual(ready.commandPreview?.arguments, [
            "prune", "approve",
            "--profile", "<AppStore.profilePath>",
            "--id", "approval-1",
            "--reason", "reviewed prune candidate",
            "--reviewer", "ops",
            "--format", "json",
            "--soft-delete", "soft-delete-1",
            "--expires-at", "2026-06-01T00:00:00Z",
        ])
    }

    @MainActor
    func testEvidenceReviewMetadataExecutionRequiresPreviewToMatchCurrentInputs() {
        let store = AppStore()
        store.profilePath = "/tmp/profile.json"
        store.selectedRole = .source
        store.approvalID = "approval-1"
        store.softDeleteIDsInput = "soft-delete-1 soft-delete-2"
        store.reason = "reviewed prune candidate"
        store.reviewer = "ops"

        let action = NextActionPlanner().plan(
            .pruneApprove,
            intent: EvidenceNextAction.OperatorIntent(
                selectedDurableEvidenceID: "soft-delete-1",
                selectedDurableEvidenceVerified: true,
                approvalID: "approval-1",
                reason: "reviewed prune candidate",
                reviewer: "ops"
            )
        )

        XCTAssertTrue(action.allowsExecution)
        XCTAssertFalse(store.evidenceReviewMetadataArgumentsMatch(action, task: .pruneApprove))

        store.runEvidenceReviewMetadataAction(action, task: .pruneApprove)

        XCTAssertTrue(store.note.contains("changed before execution"))
        XCTAssertTrue(store.recentRuns.isEmpty)
    }

    @MainActor
    func testEvidenceReviewMetadataExecutionRequiresEvidenceStillLoadedAtRunTime() throws {
        let store = AppStore()
        store.profilePath = "/tmp/profile.json"
        store.selectedRole = .source
        store.queueEntryID = "queue-entry-1"
        store.reason = "operator cancelled duplicate entry"
        store.syncQueueSnapshot = try decodeFixture(
            SyncQueueSnapshot.self,
            from: """
            {
              "operation": "list",
              "mode": "queue_only",
              "state": "present",
              "summary": \(minimalQueueSummaryJSON()),
              "entries": [
                {
                  "id": "queue-entry-1",
                  "profile_id": "profile-local",
                  "target_id": "local:profile-local",
                  "root": "root-1",
                  "path": ".hidden/secret.txt",
                  "kind": "file",
                  "status": "queued"
                }
              ]
            }
            """
        )

        let action = store.evidenceNextAction(for: .syncQueueFail)
        XCTAssertTrue(action.allowsExecution)

        store.syncQueueSnapshot = nil
        store.runEvidenceReviewMetadataAction(action, task: .syncQueueFail)

        XCTAssertTrue(store.note.contains("missing required loaded evidence"))
        XCTAssertTrue(store.recentRuns.isEmpty)
    }

    @MainActor
    func testLoadedEvidenceResolversRejectSkippedQueueRowsAndPruneRefusals() throws {
        let store = AppStore()
        store.syncQueueSnapshot = try decodeFixture(
            SyncQueueSnapshot.self,
            from: """
            {
              "operation": "list",
              "mode": "queue_only",
              "state": "present",
              "summary": \(minimalQueueSummaryJSON()),
              "entries": [
                {
                  "id": "queue-entry-1",
                  "profile_id": "profile-local",
                  "target_id": "local:profile-local",
                  "root": "root-1",
                  "path": ".hidden/secret.txt",
                  "kind": "file",
                  "status": "queued"
                }
              ],
              "skipped": [
                {
                  "root": "root-1",
                  "path": ".hidden/secret.txt",
                  "reason": "unchanged_digest"
                }
              ]
            }
            """
        )
        XCTAssertTrue(store.hasLoadedDurableSyncQueueEntryID("queue-entry-1"))
        XCTAssertFalse(store.hasLoadedDurableSyncQueueEntryID("root-1:.hidden/secret.txt:unchanged_digest"))

        store.pruneReviewSnapshot = try decodeFixture(
            PruneReviewSnapshot.self,
            from: pruneReviewCandidateAndRefusalJSON()
        )
        XCTAssertTrue(store.hasLoadedPruneCandidateSoftDeleteID("soft-delete-candidate"))
        XCTAssertFalse(store.hasLoadedPruneCandidateSoftDeleteID("soft-delete-refusal"))
        XCTAssertTrue(store.hasExactlyOneLoadedPruneCandidateSoftDeleteID(["soft-delete-candidate"]))
        XCTAssertFalse(store.hasExactlyOneLoadedPruneCandidateSoftDeleteID(["soft-delete-candidate", "soft-delete-refusal"]))

        store.driftMutationSnapshot = DriftMutationSnapshot(
            id: "drift-after-mutation",
            path: "file.txt",
            previous_state: "needs_review",
            review_state: "resolved",
            reviewed_at: "2026-05-31T07:00:00Z",
            reviewer: nil,
            reason: "target restored",
            profile_id: "profile-local",
            target_id: "local:profile-local",
            session_id: "session-1",
            repair: nil,
            manifest_rewrite: nil,
            prune: nil
        )
        XCTAssertFalse(store.hasLoadedPersistedDriftID("drift-after-mutation"))
    }

    @MainActor
    func testEvidenceArtifactCatalogRefreshUsesTargetRootAndClearsOnSetupChange() throws {
        let targetRoot = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent("AppStoreArtifactCatalogTests-\(UUID().uuidString)", isDirectory: true)
        let warningURL = targetRoot
            .appendingPathComponent(".supermover", isDirectory: true)
            .appendingPathComponent("warnings", isDirectory: true)
            .appendingPathComponent("warning-1.json")
        try FileManager.default.createDirectory(at: warningURL.deletingLastPathComponent(), withIntermediateDirectories: true)
        try #"{"id":"warning-1","message":"needs review"}"#.write(to: warningURL, atomically: true, encoding: .utf8)
        addTeardownBlock {
            try? FileManager.default.removeItem(at: targetRoot)
        }

        let store = AppStore()
        store.targetRootPath = targetRoot.path
        store.refreshEvidenceArtifactCatalog()

        XCTAssertEqual(store.evidenceArtifactCatalog?.artifacts.map(\.relativePath), [".supermover/warnings/warning-1.json"])
        XCTAssertEqual(store.evidenceArtifactCatalog?.artifacts.first?.family, .warning)

        store.prepareStructuredEvidenceForLaunch(kind: .pruneApprove)
        XCTAssertNil(store.evidenceArtifactCatalog)

        store.refreshEvidenceArtifactCatalog()
        XCTAssertNotNil(store.evidenceArtifactCatalog)

        store.targetRootPath = targetRoot.appendingPathComponent("other", isDirectory: true).path
        store.refreshEvidenceArtifactCatalog()
        XCTAssertNil(store.evidenceArtifactCatalog)
    }

    func testEvidenceVaultGenericCardsSortBySeverityWithoutTreatingUnavailableAsPass() {
        let cards = EvidenceVaultBuilder.cards(
            from: [
                EvidenceCardInput(
                    category: .status,
                    status: "ready",
                    detail: "Status evidence is clean.",
                    severity: .ok,
                    facts: [
                        EvidenceFact(key: "warnings", label: "Warnings", value: "0", severity: .ok),
                    ],
                    rawSurfaceID: "status"
                ),
                EvidenceCardInput(
                    category: .health,
                    status: "needs review",
                    detail: "Recovery evidence needs operator review.",
                    severity: .review,
                    facts: [
                        EvidenceFact(key: "recovery_issues", label: "Recovery issues", value: "1", severity: .critical),
                    ],
                    rawSurfaceID: "health"
                ),
            ]
        )

        XCTAssertEqual(cards.map(\.category), [.health, .verify, .currentSourceConsistency, .report, .status])
        XCTAssertEqual(cards.map(\.severity), [.critical, .unavailable, .unavailable, .unavailable, .ok])
        XCTAssertEqual(cards.first?.nextAction?.mode, .readOnly)
    }

    func testEvidenceVaultMissingEvidenceYieldsUnavailableNotCheckedCards() {
        let cards = EvidenceVaultBuilder().cards()

        XCTAssertEqual(Set(cards.map(\.category)), Set(EvidenceCategory.allCases))
        XCTAssertTrue(cards.allSatisfy { $0.severity == .unavailable })
        XCTAssertTrue(cards.allSatisfy { $0.status == "not checked" })
        XCTAssertFalse(cards.contains { $0.severity == .ok })
        XCTAssertTrue(cards.allSatisfy { $0.facts.contains(EvidenceFact.unavailable) })
    }

    func testEvidenceVaultBuildsReadOnlyCardsFromTypedSnapshots() throws {
        let verify = try decodeFixture(VerifySnapshot.self, from: verifyReviewJSON())
        let status = try decodeFixture(StatusSnapshot.self, from: statusReviewJSON())
        let report = try decodeFixture(ReportSnapshot.self, from: reportOKJSON())
        let health = try decodeFixture(HealthSnapshot.self, from: healthOKJSON())
        let sourceConsistency = try decodeFixture(
            AcceptanceBundleSnapshot.SourceConsistencyEvidence.self,
            from: """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "pass",
              "mode": "current_source_verified",
              "session_id": "session-1",
              "entry_count": 3,
              "mismatch_count": 0,
              "detail": "Current source tree matches the transfer baseline used for the network push session."
            }
            """
        )

        let cards = EvidenceVaultBuilder(
            verify: verify,
            sourceConsistency: sourceConsistency,
            status: status,
            report: report,
            health: health
        ).cards()

        XCTAssertEqual(Set(cards.map(\.category)), Set(EvidenceCategory.allCases))
        XCTAssertEqual(cards.first?.category, .verify)
        XCTAssertEqual(cards.first?.severity, .critical)

        let verifyCard = try XCTUnwrap(cards.first { $0.category == .verify })
        XCTAssertEqual(verifyCard.status, "manifest review")
        XCTAssertEqual(verifyCard.rawSurfaceID, "verify")
        XCTAssertEqual(verifyCard.nextAction?.mode, .readOnly)
        XCTAssertTrue(verifyCard.facts.contains { $0.key == "error_findings" && $0.severity == .critical })

        let statusCard = try XCTUnwrap(cards.first { $0.category == .status })
        XCTAssertEqual(statusCard.severity, .review)
        XCTAssertTrue(statusCard.facts.contains { $0.key == "target_drifts" && $0.value == "1" })

        let reportCard = try XCTUnwrap(cards.first { $0.category == .report })
        XCTAssertEqual(reportCard.severity, .ok)

        let healthCard = try XCTUnwrap(cards.first { $0.category == .health })
        XCTAssertEqual(healthCard.severity, .ok)

        let sourceConsistencyCard = try XCTUnwrap(cards.first { $0.category == .currentSourceConsistency })
        XCTAssertEqual(sourceConsistencyCard.severity, .ok)
        XCTAssertEqual(sourceConsistencyCard.status, "current source verified")
        XCTAssertTrue(sourceConsistencyCard.facts.contains { $0.key == "session_id" && $0.value == "session-1" })
        XCTAssertTrue(sourceConsistencyCard.facts.contains { $0.key == "entry_count" && $0.value == "3" })
    }

    func testEvidenceVaultCardsSurfaceNonZeroCLIExitAndPrioritizeIssueFacts() throws {
        let verify = try decodeFixture(VerifySnapshot.self, from: verifyReviewJSON())
        let cards = EvidenceVaultBuilder(
            verify: verify,
            envelopes: [
                .verify: StructuredEvidenceEnvelope(
                    artifactKind: .verify,
                    task: .verify,
                    loadedAt: Date(),
                    contextSignature: "context",
                    exitCode: 1,
                    rawStdout: verifyReviewJSON(),
                    stderrSample: "",
                    freshness: .current
                ),
            ]
        ).cards()

        let verifyCard = try XCTUnwrap(cards.first { $0.category == .verify })
        XCTAssertEqual(verifyCard.severity, .critical)
        XCTAssertTrue(verifyCard.facts.contains { $0.key == "cli_exit" && $0.value == "1" && $0.severity == .review })
        XCTAssertTrue(verifyCard.displayFacts(maxCount: 6).contains { $0.key == "error_findings" })
        XCTAssertTrue(verifyCard.displayFacts(maxCount: 6).contains { $0.key == "cli_exit" })
    }

    func testEvidenceVaultBuildsReviewCardForBlockedCurrentSourceConsistency() throws {
        let sourceConsistency = try decodeFixture(
            AcceptanceBundleSnapshot.SourceConsistencyEvidence.self,
            from: """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "blocked",
              "mode": "current_source_mismatch",
              "session_id": "session-2",
              "entry_count": 3,
              "mismatch_count": 2,
              "detail": "Current source tree no longer matches the transfer baseline used for the network push session."
            }
            """
        )

        let cards = EvidenceVaultBuilder(sourceConsistency: sourceConsistency).cards()
        let card = try XCTUnwrap(cards.first { $0.category == .currentSourceConsistency })
        XCTAssertEqual(card.severity, .review)
        XCTAssertEqual(card.status, "current source mismatch")
        XCTAssertTrue(card.facts.contains { $0.key == "mismatch_count" && $0.value == "2" && $0.severity == .review })
    }

    @MainActor
    func testEffectiveSourceConsistencySnapshotFallsBackToLoadedAcceptanceBundle() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "ready",
            targetAudit: "ready",
            includeInstalledAppProof: true,
            sourceConsistencyJSON: """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "pass",
              "mode": "current_source_verified",
              "session_id": "session-bundle",
              "entry_count": 7,
              "mismatch_count": 0,
              "detail": "Current source tree matches the transfer baseline used for the network push session."
            }
            """
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.profilePath = "/tmp/source.profile.json"
        store.sessionID = "session-bundle"
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        XCTAssertNil(store.sourceConsistencySnapshot)
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.session_id, "session-bundle")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.entry_count, 7)
        XCTAssertEqual(
            store.effectiveSourceConsistencySnapshot?.detail,
            "Current source tree matches the transfer baseline used for the network push session."
        )
    }

    @MainActor
    func testEffectiveSourceConsistencySnapshotPrefersLiveCaptureOverLoadedAcceptanceBundle() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "ready",
            targetAudit: "ready",
            includeInstalledAppProof: true,
            sourceConsistencyJSON: """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "blocked",
              "mode": "current_source_mismatch",
              "session_id": "session-bundle",
              "entry_count": 7,
              "mismatch_count": 2,
              "detail": "Bundle proof is stale."
            }
            """
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.profilePath = "/tmp/source.profile.json"
        store.sessionID = "session-live"
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()
        store.sourceConsistencySnapshot = try decodeFixture(
            AcceptanceBundleSnapshot.SourceConsistencyEvidence.self,
            from: """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "pass",
              "mode": "current_source_verified",
              "session_id": "session-live",
              "entry_count": 3,
              "mismatch_count": 0,
              "detail": "Live capture is current."
            }
            """
        )

        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.session_id, "session-live")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.detail, "Live capture is current.")
    }

    @MainActor
    func testEffectiveSourceConsistencySnapshotFailsClosedWithoutCurrentSessionInput() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "ready",
            targetAudit: "ready",
            includeInstalledAppProof: true,
            sourceConsistencyJSON: """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "pass",
              "mode": "current_source_verified",
              "session_id": "session-bundle",
              "entry_count": 7,
              "mismatch_count": 0,
              "detail": "Current source tree matches the transfer baseline used for the network push session."
            }
            """
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.profilePath = "/tmp/source.profile.json"
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.status, "blocked")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.mode, "session_missing")
        XCTAssertEqual(
            store.effectiveSourceConsistencySnapshot?.detail,
            "Current-source proof requires the current transfer session input before it can be treated as app-context evidence."
        )
    }

    @MainActor
    func testEffectiveSourceConsistencySnapshotFailsClosedWhenLoadedBundleSessionMismatchesCurrentSession() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "ready",
            targetAudit: "ready",
            includeInstalledAppProof: true,
            sourceConsistencyJSON: """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "pass",
              "mode": "current_source_verified",
              "session_id": "session-bundle",
              "entry_count": 7,
              "mismatch_count": 0,
              "detail": "Bundle proof is for a different session."
            }
            """
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.profilePath = "/tmp/source.profile.json"
        store.sessionID = "session-current"
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.status, "blocked")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.mode, "session_mismatch")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.session_id, "session-current")
        XCTAssertEqual(
            store.effectiveSourceConsistencySnapshot?.detail,
            "Current-source proof session_id is missing or does not match the current transfer session input."
        )
    }

    @MainActor
    func testEffectiveSourceConsistencySnapshotFailsClosedWhenLoadedBundleSessionIsMissingForCurrentSession() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "ready",
            targetAudit: "ready",
            includeInstalledAppProof: true,
            sourceConsistencyJSON: """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "pass",
              "mode": "current_source_verified",
              "entry_count": 7,
              "mismatch_count": 0,
              "detail": "Bundle proof omitted session binding."
            }
            """
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.profilePath = "/tmp/source.profile.json"
        store.sessionID = "session-current"
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.status, "blocked")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.mode, "session_mismatch")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.session_id, "session-current")
        XCTAssertEqual(
            store.effectiveSourceConsistencySnapshot?.detail,
            "Current-source proof session_id is missing or does not match the current transfer session input."
        )
    }

    @MainActor
    func testEffectiveSourceConsistencySnapshotFailsClosedWhenLoadedBundleProfileMismatchesCurrentProfile() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "ready",
            targetAudit: "ready",
            includeInstalledAppProof: true,
            sourceConsistencyJSON: """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "pass",
              "mode": "current_source_verified",
              "session_id": "session-bundle",
              "entry_count": 7,
              "mismatch_count": 0,
              "detail": "Bundle proof is for a different profile."
            }
            """
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.profilePath = "/tmp/other.profile.json"
        store.sessionID = "session-bundle"
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.status, "blocked")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.mode, "profile_mismatch")
        XCTAssertEqual(
            store.effectiveSourceConsistencySnapshot?.detail,
            "Current-source proof belongs to a different profile path than the current app selection."
        )
    }

    @MainActor
    func testEffectiveSourceConsistencySnapshotFailsClosedWhenLiveCaptureSessionMismatchesCurrentSession() throws {
        let store = AppStore()
        store.sessionID = "session-current"
        store.sourceConsistencySnapshot = try decodeFixture(
            AcceptanceBundleSnapshot.SourceConsistencyEvidence.self,
            from: """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "pass",
              "mode": "current_source_verified",
              "session_id": "session-live",
              "entry_count": 3,
              "mismatch_count": 0,
              "detail": "Live proof is for a different session."
            }
            """
        )

        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.status, "blocked")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.mode, "session_mismatch")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.session_id, "session-current")
        XCTAssertEqual(
            store.effectiveSourceConsistencySnapshot?.detail,
            "Current-source proof session_id is missing or does not match the current transfer session input."
        )
    }

    @MainActor
    func testEffectiveSourceConsistencySnapshotFailsClosedWhenLiveCaptureSessionIsMissingForCurrentSession() throws {
        let store = AppStore()
        store.sessionID = "session-current"
        store.sourceConsistencySnapshot = try decodeFixture(
            AcceptanceBundleSnapshot.SourceConsistencyEvidence.self,
            from: """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "pass",
              "mode": "current_source_verified",
              "entry_count": 3,
              "mismatch_count": 0,
              "detail": "Live proof omitted session binding."
            }
            """
        )

        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.status, "blocked")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.mode, "session_mismatch")
        XCTAssertEqual(store.effectiveSourceConsistencySnapshot?.session_id, "session-current")
        XCTAssertEqual(
            store.effectiveSourceConsistencySnapshot?.detail,
            "Current-source proof session_id is missing or does not match the current transfer session input."
        )
    }

    @MainActor
    func testVerifySnapshotClearsOnSetupContextChange() throws {
        let store = AppStore()
        store.verifySnapshot = try decodeFixture(VerifySnapshot.self, from: verifyReviewJSON())

        store.profilePath = "/tmp/other-profile.json"

        XCTAssertNil(store.verifySnapshot)
    }

    @MainActor
    func testSetupContextChangeMarksEvidenceEnvelopeStaleWithoutDroppingRawOutput() {
        let store = AppStore()
        store.evidenceEnvelopes[.verify] = StructuredEvidenceEnvelope(
            artifactKind: .verify,
            task: .verify,
            loadedAt: Date(),
            contextSignature: "old-context",
            exitCode: 0,
            rawStdout: #"{"summary":{"files_verified":1}}"#,
            stderrSample: "stderr sample",
            freshness: .current
        )

        store.profilePath = "/tmp/other-profile.json"

        let envelope = store.evidenceEnvelopes[.verify]
        XCTAssertEqual(envelope?.freshness, .stale)
        XCTAssertEqual(envelope?.rawStdout, #"{"summary":{"files_verified":1}}"#)
        XCTAssertEqual(envelope?.stderrSample, "stderr sample")
    }

    @MainActor
    func testSetupContextChangeMarksSourceConsistencyEnvelopeStaleWithoutDroppingRawOutput() {
        let store = AppStore()
        store.evidenceEnvelopes[.sourceConsistency] = StructuredEvidenceEnvelope(
            artifactKind: .sourceConsistency,
            task: .networkPush,
            loadedAt: Date(),
            contextSignature: "old-context",
            exitCode: 0,
            rawStdout: #"{"status":"pass","mode":"current_source_verified"}"#,
            stderrSample: "stderr sample",
            freshness: .current
        )

        store.profilePath = "/tmp/other-profile.json"

        let envelope = store.evidenceEnvelopes[.sourceConsistency]
        XCTAssertEqual(envelope?.freshness, .stale)
        XCTAssertEqual(envelope?.rawStdout, #"{"status":"pass","mode":"current_source_verified"}"#)
        XCTAssertEqual(envelope?.stderrSample, "stderr sample")
    }

    @MainActor
    func testSuccessfulNetworkPushCapturesSourceConsistencyEvidenceFromCLI() throws {
        let store = AppStore()
        let dir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        let baselineURL = dir.appendingPathComponent("source.baseline.json")
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-src",
          "root_id": "root-src",
          "root_path": "/tmp/source",
          "session_id": "session-1",
          "created_at": "2026-06-01T10:00:00Z",
          "entries": []
        }
        """.write(to: baselineURL, atomically: true, encoding: .utf8)

        store.profilePath = "/tmp/profile.json"
        store.sessionID = "session-1"
        store.cliCommandRunner = { arguments in
            XCTAssertEqual(
                arguments,
                [
                    "verify", "source-consistency",
                    "--profile", "/tmp/profile.json",
                    "--baseline", baselineURL.path,
                    "--format", "json",
                ]
            )
            return (
                #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#,
                "",
                0
            )
        }

        let contextSignature = store.currentContextSignature(for: .networkPush)
        store.installNetworkPushBaselineFileForTesting(baselineURL.path, contextSignature: contextSignature)

        store.recordSuccessfulCompletionForTesting(
            TaskRun(
                kind: .networkPush,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: contextSignature,
                processIdentifier: nil,
                stdout: #"{"status":"published"}"#,
                stderr: "",
                state: .finished(0)
            )
        )

        let envelope = try XCTUnwrap(store.evidenceEnvelopes[.sourceConsistency])
        XCTAssertEqual(envelope.task, .networkPush)
        XCTAssertEqual(envelope.contextSignature, contextSignature)
        XCTAssertEqual(envelope.exitCode, 0)
        XCTAssertEqual(envelope.freshness, .current)
        XCTAssertEqual(
            envelope.rawStdout,
            #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#
        )
        XCTAssertTrue(store.appEvents.contains { event in
            event.title == "source consistency captured" &&
            event.detail.contains("current-source proof")
        })
        XCTAssertTrue(FileManager.default.fileExists(atPath: baselineURL.path))
    }

    @MainActor
    func testSuccessfulNetworkPushBlocksCapturedSourceConsistencyWhenCLIProofSessionMismatchesCurrentSession() throws {
        let store = AppStore()
        let dir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        let baselineURL = dir.appendingPathComponent("source.baseline.json")
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-src",
          "root_id": "root-src",
          "root_path": "/tmp/source",
          "session_id": "session-1",
          "created_at": "2026-06-01T10:00:00Z",
          "entries": []
        }
        """.write(to: baselineURL, atomically: true, encoding: .utf8)

        store.profilePath = "/tmp/profile.json"
        store.sessionID = "session-1"
        store.cliCommandRunner = { _ in
            (
                #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-other"}"#,
                "",
                0
            )
        }

        let contextSignature = store.currentContextSignature(for: .networkPush)
        store.installNetworkPushBaselineFileForTesting(baselineURL.path, contextSignature: contextSignature)

        store.recordSuccessfulCompletionForTesting(
            TaskRun(
                kind: .networkPush,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: contextSignature,
                processIdentifier: nil,
                stdout: #"{"status":"published"}"#,
                stderr: "",
                state: .finished(0)
            )
        )

        let envelope = try XCTUnwrap(store.evidenceEnvelopes[.sourceConsistency])
        XCTAssertEqual(
            envelope.rawStdout,
            #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-other"}"#
        )
        XCTAssertEqual(store.sourceConsistencySnapshot?.status, "blocked")
        XCTAssertEqual(store.sourceConsistencySnapshot?.mode, "session_mismatch")
        XCTAssertEqual(store.sourceConsistencySnapshot?.session_id, "session-1")
        XCTAssertEqual(
            store.sourceConsistencySnapshot?.detail,
            "Current-source proof session_id is missing or does not match the current transfer session input."
        )
    }

    @MainActor
    func testSuccessfulNetworkPushInvalidSourceConsistencyJSONFailsClosedAndRecordsArtifactProblem() throws {
        let store = AppStore()
        let dir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        let baselineURL = dir.appendingPathComponent("source.baseline.json")
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-src",
          "session_id": "session-1",
          "entries": []
        }
        """.write(to: baselineURL, atomically: true, encoding: .utf8)

        store.profilePath = "/tmp/profile.json"
        store.sessionID = "session-1"
        store.cliCommandRunner = { _ in
            ("not-json", "", 0)
        }

        let contextSignature = store.currentContextSignature(for: .networkPush)
        store.installNetworkPushBaselineFileForTesting(baselineURL.path, contextSignature: contextSignature)

        store.recordSuccessfulCompletionForTesting(
            TaskRun(
                kind: .networkPush,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: contextSignature,
                processIdentifier: nil,
                stdout: #"{"status":"published"}"#,
                stderr: "",
                state: .finished(0)
            )
        )

        let envelope = try XCTUnwrap(store.evidenceEnvelopes[.sourceConsistency])
        XCTAssertEqual(envelope.rawStdout, "not-json")
        XCTAssertEqual(store.sourceConsistencySnapshot?.status, "blocked")
        XCTAssertEqual(store.sourceConsistencySnapshot?.mode, "artifact_invalid")
        XCTAssertEqual(
            store.sourceConsistencySnapshot?.detail,
            "Bundled CLI did not emit valid current-source JSON. Review artifact read problems before trusting current-source proof."
        )
        XCTAssertEqual(store.artifactReadProblems.first?.artifactKind, .sourceConsistency)
        XCTAssertTrue(store.appEvents.contains { $0.title == "source consistency capture invalid" })
        XCTAssertTrue(FileManager.default.fileExists(atPath: baselineURL.path))
    }

    @MainActor
    func testSetupContextChangeCleansRetainedNetworkPushBaselineFile() throws {
        let store = AppStore()
        let dir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        let baselineURL = dir.appendingPathComponent("source.baseline.json")
        try Data("{}".utf8).write(to: baselineURL)

        store.installNetworkPushBaselineFileForTesting(
            baselineURL.path,
            contextSignature: "context-1"
        )
        XCTAssertTrue(FileManager.default.fileExists(atPath: baselineURL.path))

        store.profilePath = "/tmp/new-profile.json"

        XCTAssertFalse(FileManager.default.fileExists(atPath: baselineURL.path))
    }

    @MainActor
    func testSuccessfulNetworkPushBlocksCapturedSourceConsistencyWhenCLIProofSessionIsMissingForCurrentSession() throws {
        let store = AppStore()
        let dir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        let baselineURL = dir.appendingPathComponent("source.baseline.json")
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-src",
          "session_id": "session-1",
          "entries": []
        }
        """.write(to: baselineURL, atomically: true, encoding: .utf8)

        store.profilePath = "/tmp/profile.json"
        store.sessionID = "session-1"
        store.cliCommandRunner = { _ in
            (
                #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified"}"#,
                "",
                0
            )
        }

        let contextSignature = store.currentContextSignature(for: .networkPush)
        store.installNetworkPushBaselineFileForTesting(baselineURL.path, contextSignature: contextSignature)

        store.recordSuccessfulCompletionForTesting(
            TaskRun(
                kind: .networkPush,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: contextSignature,
                processIdentifier: nil,
                stdout: #"{"status":"published"}"#,
                stderr: "",
                state: .finished(0)
            )
        )

        let envelope = try XCTUnwrap(store.evidenceEnvelopes[.sourceConsistency])
        XCTAssertEqual(
            envelope.rawStdout,
            #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified"}"#
        )
        XCTAssertEqual(store.sourceConsistencySnapshot?.status, "blocked")
        XCTAssertEqual(store.sourceConsistencySnapshot?.mode, "session_mismatch")
        XCTAssertEqual(store.sourceConsistencySnapshot?.session_id, "session-1")
        XCTAssertEqual(
            store.sourceConsistencySnapshot?.detail,
            "Current-source proof session_id is missing or does not match the current transfer session input."
        )
    }

    @MainActor
    func testEvidenceVaultRetainsMultipleReadSurfacesInSameContext() {
        let store = AppStore()

        store.prepareStructuredEvidenceForLaunch(kind: .status)
        store.captureStructuredResult(
            for: .status,
            stdout: statusReviewJSON(),
            stderr: "",
            exitCode: 0,
            contextSignature: store.currentContextSignature(for: .status)
        )

        store.prepareStructuredEvidenceForLaunch(kind: .report)
        store.captureStructuredResult(
            for: .report,
            stdout: reportOKJSON(),
            stderr: "",
            exitCode: 0,
            contextSignature: store.currentContextSignature(for: .report)
        )

        XCTAssertNotNil(store.statusSnapshot)
        XCTAssertNotNil(store.reportSnapshot)
        XCTAssertEqual(store.evidenceEnvelopes[.status]?.freshness, .current)
        XCTAssertEqual(store.evidenceEnvelopes[.report]?.freshness, .current)
        XCTAssertEqual(store.evidenceEnvelopeHistory.filter { $0.freshness == .current }.map(\.artifactKind).sorted(by: { $0.rawValue < $1.rawValue }), [.report, .status])
    }

    @MainActor
    func testStaleStructuredCompletionIsRetainedAsStaleRawEnvelopeOnly() {
        let store = AppStore()
        let oldSignature = store.currentContextSignature(for: .status)

        store.profilePath = "/tmp/other-profile.json"
        store.captureStructuredResult(
            for: .status,
            stdout: statusReviewJSON(),
            stderr: "old context stderr",
            exitCode: 1,
            contextSignature: oldSignature
        )

        XCTAssertNil(store.statusSnapshot)
        XCTAssertNil(store.evidenceEnvelopes[.status])
        let envelope = store.evidenceEnvelopeHistory.first
        XCTAssertEqual(envelope?.artifactKind, .status)
        XCTAssertEqual(envelope?.freshness, .stale)
        XCTAssertEqual(envelope?.exitCode, 1)
        XCTAssertEqual(envelope?.stderrSample, "old context stderr")
    }

    @MainActor
    func testSelectedProfileDisplayFallsBackToSelectedFileName() {
        let store = AppStore()
        store.profilePath = "/tmp/source.profile.json"

        XCTAssertTrue(store.isProfileSelected)
        XCTAssertEqual(store.selectedProfileDisplayTitle, "Custom setup location")
        XCTAssertEqual(store.selectedProfileDisplayDetail, "Choose folders, then create the setup.")
        XCTAssertEqual(store.selectedProfileDisplayMetadata, "Ready to create through the selected file.")
        XCTAssertEqual(store.selectedProfileRawPath, "/tmp/source.profile.json")
        XCTAssertEqual(store.selectedProfileRawPathLabel, "File location")
        XCTAssertNil(store.selectedProfileEvidenceID)
        XCTAssertTrue(store.selectedProfileShowsSourceIdentityFields)
        XCTAssertTrue(store.selectedProfileShowsTargetIdentityFields)
        XCTAssertTrue(store.selectedProfileAllowsExplicitCreate)
    }

    @MainActor
    func testSelectedProfileDisplayExposesLoadedEvidenceProfileIDSeparatelyFromFileName() throws {
        let store = AppStore()
        store.profilePath = "/tmp/source.profile.json"
        store.statusSnapshot = try decodeFixture(
            StatusSnapshot.self,
            from: statusReviewJSON().replacingOccurrences(
                of: "\"profile-local\"",
                with: "\"profile-evidence\""
            )
        )

        XCTAssertEqual(store.selectedProfileDisplayTitle, "profile-evidence")
        XCTAssertEqual(store.selectedProfileDisplayDetail, "source.profile.json")
        XCTAssertEqual(store.selectedProfileDisplayMetadata, "Ready to create through the selected file.")
        XCTAssertEqual(store.selectedProfileRawPath, "/tmp/source.profile.json")
        XCTAssertEqual(store.selectedProfileEvidenceID, "profile-evidence")
        XCTAssertFalse(store.selectedProfileShowsSourceIdentityFields)
        XCTAssertTrue(store.selectedProfileShowsTargetIdentityFields)
    }

    @MainActor
    func testSelectedProfileDisplayUsesExistingFileSummaryWithoutLeakingRawPath() throws {
        let store = AppStore()
        let profileURL = makeTemporaryProfileURL(named: "source.profile.json")
        try "{}".write(to: profileURL, atomically: true, encoding: .utf8)
        defer { try? FileManager.default.removeItem(at: profileURL) }

        store.profilePath = profileURL.path

        XCTAssertEqual(store.selectedProfileDisplayTitle, profileURL.lastPathComponent)
        XCTAssertEqual(store.selectedProfileDisplayDetail, "Existing config file")
        XCTAssertNil(store.selectedProfileDisplayMetadata)
        XCTAssertEqual(store.selectedProfileRawPath, profileURL.path)
        XCTAssertFalse(store.selectedProfileShowsSourceIdentityFields)
        XCTAssertTrue(store.selectedProfileShowsTargetIdentityFields)
        XCTAssertFalse(store.selectedProfileAllowsExplicitCreate)
    }

    @MainActor
    func testSelectedProfileDisplayUpdatesWhenProfilePathChangesAndClearsCurrentEvidence() throws {
        let store = AppStore()
        store.profilePath = "/tmp/source.profile.json"
        store.statusSnapshot = try decodeFixture(StatusSnapshot.self, from: statusReviewJSON())

        XCTAssertEqual(store.selectedProfileDisplayTitle, "profile-local")
        XCTAssertEqual(store.selectedProfileEvidenceID, "profile-local")

        store.profilePath = "/tmp/other-profile.json"

        XCTAssertNil(store.statusSnapshot)
        XCTAssertEqual(store.selectedProfileDisplayTitle, "Custom setup location")
        XCTAssertEqual(store.selectedProfileDisplayDetail, "Choose folders, then create the setup.")
        XCTAssertEqual(store.selectedProfileDisplayMetadata, "Ready to create through the selected file.")
        XCTAssertEqual(store.selectedProfileRawPath, "/tmp/other-profile.json")
        XCTAssertNil(store.selectedProfileEvidenceID)
    }

    @MainActor
    func testSelectedProfileDisplayUsesDraftIdentityForNewSourceDestination() {
        let store = AppStore()
        store.profileName = "Studio Migration"
        store.profileID = "profile-studio"
        store.profilePath = "/tmp/studio.profile.json"

        XCTAssertEqual(store.selectedProfileDisplayTitle, "Studio Migration")
        XCTAssertEqual(store.selectedProfileDisplayDetail, "ID: profile-studio")
        XCTAssertEqual(store.selectedProfileDisplayMetadata, "Ready to create through the selected file.")
        XCTAssertTrue(store.selectedProfileShowsSourceIdentityFields)
        XCTAssertTrue(store.selectedProfileShowsTargetIdentityFields)
    }

    @MainActor
    func testRecommendedProfileDestinationSelectsSuperMoverProfileWithoutAutoLaunching() throws {
        let store = AppStore()
        let defaultsRootURL = try makeTemporaryDirectory().appendingPathComponent(".supermover", isDirectory: true)
        defer { try? FileManager.default.removeItem(at: defaultsRootURL.deletingLastPathComponent()) }
        store.profileDefaultsRootURL = defaultsRootURL
        store.selectedTask = .status

        store.useRecommendedProfileDestination()

        XCTAssertEqual(store.profilePath, defaultsRootURL.appendingPathComponent("profile-local.json").path)
        XCTAssertEqual(store.recommendedProfileDestinationPath, store.profilePath)
        XCTAssertEqual(store.selectedProfilePathState, .newDestination)
        XCTAssertEqual(store.selectedProfileDisplayTitle, "Recommended source migration config")
        XCTAssertEqual(store.selectedProfileDisplayMetadata, "Recommended location selected.")
        XCTAssertEqual(store.selectedTask, .status)
        XCTAssertTrue(store.recentRuns.isEmpty)
        XCTAssertTrue(store.activeRuns.isEmpty)
    }

    @MainActor
    func testSelectedProfileDisplayMarksDirectorySelectionsForReview() throws {
        let store = AppStore()
        let directoryURL = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: directoryURL) }

        store.profilePath = directoryURL.path

        XCTAssertEqual(store.selectedProfilePathState, .directory)
        XCTAssertEqual(store.selectedProfileDisplayTitle, directoryURL.lastPathComponent)
        XCTAssertEqual(store.selectedProfileDisplayDetail, "Folder selected")
        XCTAssertEqual(store.selectedProfileDisplayMetadata, "Choose a .json migration config file.")
        XCTAssertFalse(store.selectedProfileShowsSourceIdentityFields)
        XCTAssertFalse(store.selectedProfileShowsTargetIdentityFields)
    }

    @MainActor
    func testTargetRoleExistingProfileSelectionShowsTargetIdentityFieldsOnly() throws {
        let store = AppStore()
        let profileURL = makeTemporaryProfileURL(named: "target.profile.json")
        try "{}".write(to: profileURL, atomically: true, encoding: .utf8)
        defer { try? FileManager.default.removeItem(at: profileURL) }

        store.selectedRole = .target
        store.profilePath = profileURL.path

        XCTAssertFalse(store.selectedProfileShowsSourceIdentityFields)
        XCTAssertTrue(store.selectedProfileShowsTargetIdentityFields)
        XCTAssertFalse(store.selectedProfileAllowsExplicitCreate)
    }

    @MainActor
    func testProfileDestinationPlanInitializesNewSourceProfileWhenRootsAreReady() throws {
        let store = AppStore()
        let sourceRoot = try makeTemporaryDirectory()
        let targetRoot = try makeTemporaryDirectory()
        defer {
            try? FileManager.default.removeItem(at: sourceRoot)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        store.selectedRole = .source
        store.sourceRootPath = sourceRoot.path
        store.targetRootPath = targetRoot.path
        store.profileName = "Studio Migration"
        store.profileID = "profile-studio"
        store.targetID = "target-1"

        XCTAssertEqual(
            store.profileDestinationPlan(for: "/tmp/studio.profile.json"),
            .initialize(
                arguments: [
                    "profile", "init",
                    "--profile", "/tmp/studio.profile.json",
                    "--source", sourceRoot.path,
                    "--target", targetRoot.path,
                    "--id", "profile-studio",
                    "--name", "Studio Migration",
                    "--target-id", "target-1",
                ],
                note: "Writing migration config through CLI. Run Lint Config before treating setup as ready."
            )
        )
    }

    @MainActor
    func testProfileDestinationPlanStaysSelectionOnlyUntilRootsAreReady() {
        let store = AppStore()
        store.selectedRole = .source

        XCTAssertEqual(
            store.profileDestinationPlan(for: "/tmp/studio.profile.json"),
            .selectedOnly(
                note: "New migration config destination selected. Choose a readable source root before writing the config file."
            )
        )
    }

    @MainActor
    func testProfileDestinationPlanRejectsDirectorySelections() throws {
        let store = AppStore()
        let directoryURL = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: directoryURL) }

        store.selectedRole = .source

        XCTAssertEqual(
            store.profileDestinationPlan(for: directoryURL.path),
            .selectedOnly(
                note: "That selection is a folder. Choose a .json migration config file, not a directory."
            )
        )
    }

    @MainActor
    func testSetupGuideExplainsEmptySourcePreparationInUserOrder() {
        let store = AppStore()
        store.selectedRole = .source

        let guide = store.setupGuide

        XCTAssertEqual(guide.title, "Prepare this Source")
        XCTAssertEqual(guide.steps.map(\.title), [
            "Migration config file",
            "Choose folders",
            "Validate before moving",
        ])
        XCTAssertEqual(guide.steps.map(\.state), [.pending, .neutral, .pending])
        XCTAssertEqual(guide.steps[0].primaryActionTitle, "Create Migration Setup")
        XCTAssertEqual(guide.steps[0].primaryTask, .profileInit)
        XCTAssertNil(guide.steps[0].secondaryActionTitle)
        XCTAssertEqual(guide.steps[0].detail, "Choose folders, then create the recommended setup. Existing and custom config files live in Advanced.")
        XCTAssertEqual(guide.steps[1].detail, "Choose the folder to move and the destination folder. Lint and Status read the saved setup.")
        XCTAssertEqual(guide.steps[2].detail, "Create or open the config, then run Lint Config before treating setup as ready.")
    }

    @MainActor
    func testLocalizedSetupGuideExplainsEmptySourcePreparationWithoutChangingRawGuide() {
        let store = AppStore()
        store.selectedRole = .source
        let localization = AppChromeLocalization(language: .simplifiedChinese)

        let localizedGuide = store.localizedSetupGuide(using: localization)

        XCTAssertEqual(localizedGuide.title, "准备这台源端 Mac")
        XCTAssertEqual(localizedGuide.subtitle, "选择角色和目录，然后创建或检查迁移设置。")
        XCTAssertEqual(localizedGuide.steps.map(\.title), [
            "迁移配置文件",
            "选择目录",
            "迁移前验证",
        ])
        XCTAssertEqual(localizedGuide.steps.map(\.state), [.pending, .neutral, .pending])
        XCTAssertEqual(localizedGuide.steps[0].primaryActionTitle, "创建迁移设置")
        XCTAssertEqual(localizedGuide.steps[0].primaryTask, .profileInit)
        XCTAssertNil(localizedGuide.steps[0].secondaryActionTitle)
        XCTAssertEqual(localizedGuide.steps[0].detail, "选择目录后创建推荐设置；已有配置和自定义位置在高级选项里。")
        XCTAssertEqual(localizedGuide.steps[1].statusLabel, "创建或更新时可选")
        XCTAssertEqual(localizedGuide.steps[2].statusLabel, "未验证")

        XCTAssertEqual(store.setupGuide.title, "Prepare this Source")
        XCTAssertEqual(store.setupGuide.steps[0].primaryActionTitle, "Create Migration Setup")
        XCTAssertEqual(store.setupGuide.steps[0].primaryTask, .profileInit)
    }

    @MainActor
    func testSetupGuideShowsNewConfigDestinationAsCreationStep() throws {
        let store = AppStore()
        let sourceRoot = try makeTemporaryDirectory()
        let targetRoot = try makeTemporaryDirectory()
        defer {
            try? FileManager.default.removeItem(at: sourceRoot)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        store.selectedRole = .source
        store.sourceRootPath = sourceRoot.path
        store.targetRootPath = targetRoot.path
        store.applyProfileDestinationSelection("/tmp/studio.profile.json")

        let guide = store.setupGuide

        XCTAssertEqual(guide.steps[0].state, .review)
        XCTAssertEqual(guide.steps[0].primaryActionTitle, "Create New Config File")
        XCTAssertEqual(guide.steps[0].detail, "New destination selected. Write the migration config through the CLI before other tasks use it.")
        XCTAssertEqual(guide.steps[1].state, .pass)
        XCTAssertEqual(guide.steps[1].statusLabel, "source readable / target writable")
        XCTAssertEqual(guide.steps[2].state, .pending)
        XCTAssertEqual(guide.steps[2].primaryActionTitle, "Lint Config")
    }

    @MainActor
    func testSetupGuideShowsExistingTargetConfigWithoutCreationCTA() throws {
        let store = AppStore()
        let profileURL = makeTemporaryProfileURL(named: "target.profile.json")
        let targetRoot = try makeTemporaryDirectory()
        try "{}".write(to: profileURL, atomically: true, encoding: .utf8)
        defer {
            try? FileManager.default.removeItem(at: profileURL)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        store.selectedRole = .target
        store.profilePath = profileURL.path
        store.targetRootPath = targetRoot.path

        let guide = store.setupGuide

        XCTAssertEqual(guide.title, "Prepare this Target")
        XCTAssertEqual(guide.steps[0].state, .pass)
        XCTAssertEqual(guide.steps[0].primaryActionTitle, "Open Existing Config")
        XCTAssertNil(guide.steps[0].secondaryActionTitle)
        XCTAssertEqual(guide.steps[1].title, "Destination folder")
        XCTAssertEqual(guide.steps[1].state, .pass)
        XCTAssertEqual(guide.steps[2].detail, "Run Lint Config or Read Status to confirm the selected config still matches durable evidence.")
        XCTAssertEqual(guide.steps[2].primaryActionTitle, "Lint Existing Config")
        XCTAssertEqual(guide.steps[2].secondaryActionTitle, "Read Status")
    }

    @MainActor
    func testLocalizedSetupGuideShowsTargetValidationActions() throws {
        let store = AppStore()
        let profileURL = makeTemporaryProfileURL(named: "target.profile.json")
        let targetRoot = try makeTemporaryDirectory()
        try "{}".write(to: profileURL, atomically: true, encoding: .utf8)
        defer {
            try? FileManager.default.removeItem(at: profileURL)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        store.selectedRole = .target
        store.profilePath = profileURL.path
        store.targetRootPath = targetRoot.path
        let localization = AppChromeLocalization(language: .simplifiedChinese)

        let localizedGuide = store.localizedSetupGuide(using: localization)

        XCTAssertEqual(localizedGuide.title, "准备这台目标端 Mac")
        XCTAssertEqual(localizedGuide.steps[0].statusLabel, "现有配置文件")
        XCTAssertEqual(localizedGuide.steps[1].title, "目标目录")
        XCTAssertEqual(localizedGuide.steps[1].statusLabel, "目标端可写")
        XCTAssertEqual(localizedGuide.steps[2].detail, "运行 Lint Config 或 Read Status，确认所选配置仍匹配持久证据。")
        XCTAssertEqual(localizedGuide.steps[2].primaryActionTitle, "检查现有配置")
        XCTAssertEqual(localizedGuide.steps[2].secondaryActionTitle, "读取状态")
    }

    @MainActor
    func testLocalizedProfileSelectionDisplayDoesNotChangeRawConfigValues() throws {
        let store = AppStore()
        store.selectedRole = .source
        let localization = AppChromeLocalization(language: .simplifiedChinese)

        var display = store.localizedProfileSelectionContext(using: localization)
        XCTAssertEqual(display.title, "推荐设置")
        XCTAssertEqual(display.detail, "不用手动选择配置文件。选好目录后创建迁移设置即可。")
        XCTAssertEqual(display.rawPathLabel, "文件位置")
        XCTAssertNil(display.rawPath)
        XCTAssertNil(display.metadata)
        XCTAssertEqual(store.selectedProfileDisplayTitle, "Recommended setup")
        XCTAssertEqual(store.missingProfileCommandPreviewValue, "<Config File Required>")

        store.applyProfileDestinationSelection("/tmp/studio.profile.json")

        display = store.localizedProfileSelectionContext(using: localization)
        XCTAssertEqual(display.title, "自定义设置位置")
        XCTAssertEqual(display.detail, "选择目录，然后创建迁移设置。")
        XCTAssertEqual(display.metadata, "将通过所选文件创建。")
        XCTAssertEqual(display.rawPathLabel, "文件位置")
        XCTAssertEqual(display.rawPath, "/tmp/studio.profile.json")
        XCTAssertEqual(store.selectedProfileDisplayTitle, "Custom setup location")
        XCTAssertEqual(store.selectedProfileCommandPreviewValue, "<Selected Config: studio.profile.json>")
        XCTAssertEqual(
            store.commandPreviewArguments(for: .lintProfile),
            ["profile", "lint", "--profile", "<Selected Config: studio.profile.json>"]
        )
    }

    func testWorkbenchRoleLocalizedLabelsDoNotChangeRoleIdentity() {
        let simplifiedChinese = AppChromeLocalization(language: .simplifiedChinese)

        XCTAssertEqual(WorkbenchRole.source.localizedTitle(using: simplifiedChinese), "源端")
        XCTAssertEqual(WorkbenchRole.target.localizedTitle(using: simplifiedChinese), "目标端")
        XCTAssertEqual(WorkbenchRole.observer.localizedTitle(using: simplifiedChinese), "观察端")
        XCTAssertEqual(
            WorkbenchRole.allCases.map(\.rawValue),
            ["source", "target", "observer"]
        )
        XCTAssertEqual(WorkbenchRole.source.title, "Source")
        XCTAssertEqual(WorkbenchRole.source.allowedSetup, "create config, lint config, update target, dry-run preparation")
    }

    @MainActor
    func testSetupGuideCountsSuccessfulStatusAsObserverValidation() throws {
        let store = AppStore()
        let profileURL = makeTemporaryProfileURL(named: "observer.profile.json")
        try "{}".write(to: profileURL, atomically: true, encoding: .utf8)
        defer {
            try? FileManager.default.removeItem(at: profileURL)
        }
        store.selectedRole = .observer
        store.profilePath = profileURL.path
        let contextSignature = store.currentContextSignature(for: .status)

        store.recordSuccessfulCompletionForTesting(
            TaskRun(
                kind: .status,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: contextSignature,
                processIdentifier: nil,
                stdout: #"{"overall":{"status":"pass"}}"#,
                stderr: "",
                state: .finished(0)
            )
        )

        let guide = store.setupGuide

        XCTAssertEqual(guide.steps[2].state, .pass)
        XCTAssertEqual(guide.steps[2].statusLabel, "status read")
        XCTAssertEqual(guide.steps[2].primaryActionTitle, "Read Status")
    }

    @MainActor
    func testSetupGuideCountsSuccessfulStatusAsTargetValidation() throws {
        let store = AppStore()
        let profileURL = makeTemporaryProfileURL(named: "target.profile.json")
        let targetRoot = try makeTemporaryDirectory()
        try "{}".write(to: profileURL, atomically: true, encoding: .utf8)
        defer {
            try? FileManager.default.removeItem(at: profileURL)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        store.selectedRole = .target
        store.profilePath = profileURL.path
        store.targetRootPath = targetRoot.path
        let contextSignature = store.currentContextSignature(for: .status)

        store.recordSuccessfulCompletionForTesting(
            TaskRun(
                kind: .status,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: contextSignature,
                processIdentifier: nil,
                stdout: #"{"overall":{"status":"pass"}}"#,
                stderr: "",
                state: .finished(0)
            )
        )

        let guide = store.setupGuide

        XCTAssertEqual(guide.steps[2].state, .pass)
        XCTAssertEqual(guide.steps[2].statusLabel, "status read")
    }

    @MainActor
    func testSetupGuideClarifiesExistingConfigRootInputsAreNotLoadedRoots() throws {
        let store = AppStore()
        let profileURL = makeTemporaryProfileURL(named: "target.profile.json")
        try "{}".write(to: profileURL, atomically: true, encoding: .utf8)
        defer {
            try? FileManager.default.removeItem(at: profileURL)
        }
        store.selectedRole = .target
        store.profilePath = profileURL.path

        let guide = store.setupGuide

        XCTAssertEqual(guide.steps[1].title, "Destination folder")
        XCTAssertEqual(
            guide.steps[1].detail,
            "Use this folder only when explicitly updating the selected setup target. Lint and Status read the saved setup."
        )
        XCTAssertEqual(guide.steps[1].state, .neutral)
        XCTAssertEqual(guide.steps[1].statusLabel, "optional unless updating")
    }

    @MainActor
    func testApplyProfileDestinationSelectionDoesNotAutoLaunchProfileCreation() throws {
        let store = AppStore()
        let sourceRoot = try makeTemporaryDirectory()
        let targetRoot = try makeTemporaryDirectory()
        defer {
            try? FileManager.default.removeItem(at: sourceRoot)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        store.selectedRole = .source
        store.sourceRootPath = sourceRoot.path
        store.targetRootPath = targetRoot.path
        store.selectedTask = .status

        store.applyProfileDestinationSelection("/tmp/studio.profile.json")

        XCTAssertEqual(store.profilePath, "/tmp/studio.profile.json")
        XCTAssertEqual(store.selectedProfilePathState, .newDestination)
        XCTAssertEqual(store.selectedTask, .status)
        XCTAssertTrue(store.recentRuns.isEmpty)
        XCTAssertTrue(store.activeRuns.isEmpty)
        XCTAssertEqual(
            store.note,
            "New migration config destination selected. Review the current roots, then click Create Config File."
        )
    }

    @MainActor
    func testNewProfileDestinationDoesNotSatisfyExistingProfileTasks() throws {
        let store = AppStore()
        store.selectedRole = .source
        store.selectedTask = .status
        store.applyProfileDestinationSelection("/tmp/studio.profile.json")

        store.runSelectedTask()

        XCTAssertEqual(
            store.note,
            "Open an existing migration config file before running Status. A new config destination is only valid for Create Config File."
        )
        XCTAssertTrue(store.recentRuns.isEmpty)
        XCTAssertTrue(store.activeRuns.isEmpty)
    }

    @MainActor
    func testTaskRunGateBlocksExistingProfileTasksForNewProfileDestination() throws {
        let store = AppStore()
        store.selectedRole = .source
        store.selectedTask = .status
        store.applyProfileDestinationSelection("/tmp/studio.profile.json")

        let gate = store.taskRunGate()

        XCTAssertFalse(gate.isRunnable)
        XCTAssertEqual(
            gate.note,
            "Open an existing migration config file before running Status. A new config destination is only valid for Create Config File."
        )
    }

    @MainActor
    func testTaskRunGateBlocksPublishWithoutSessionID() throws {
        let store = AppStore()
        let profileURL = makeTemporaryProfileURL(named: "source.profile.json")
        try "{}".write(to: profileURL, atomically: true, encoding: .utf8)
        defer { try? FileManager.default.removeItem(at: profileURL) }
        store.selectedRole = .source
        store.selectedTask = .publish
        store.profilePath = profileURL.path

        let gate = store.taskRunGate()

        XCTAssertFalse(gate.isRunnable)
        XCTAssertEqual(
            gate.note,
            "Provide an explicit session id first. This app no longer creates hidden mutating session defaults."
        )
    }

    @MainActor
    func testTaskRunGateBlocksReconcileApplyWithoutReason() throws {
        let store = AppStore()
        let profileURL = makeTemporaryProfileURL(named: "source.profile.json")
        try "{}".write(to: profileURL, atomically: true, encoding: .utf8)
        defer { try? FileManager.default.removeItem(at: profileURL) }
        store.selectedRole = .source
        store.selectedTask = .reconcileApply
        store.profilePath = profileURL.path
        store.driftIDsInput = "drift-1"

        let gate = store.taskRunGate()

        XCTAssertFalse(gate.isRunnable)
        XCTAssertEqual(gate.note, "Provide a reason first.")
    }

    @MainActor
    func testNewProfileDestinationStillAllowsProfileInitPreflight() throws {
        let store = AppStore()
        store.selectedRole = .source
        store.selectedTask = .profileInit
        store.applyProfileDestinationSelection("/tmp/studio.profile.json")

        XCTAssertFalse(store.selectedTask.requiresExistingProfile)
        XCTAssertEqual(store.selectedProfilePathState, .newDestination)
        XCTAssertTrue(store.selectedProfileAllowsExplicitCreate)
    }

    @MainActor
    func testTaskRunGateBlocksProfileInitForExistingProfileFile() throws {
        let store = AppStore()
        let profileURL = makeTemporaryProfileURL(named: "existing.profile.json")
        try "{}".write(to: profileURL, atomically: true, encoding: .utf8)
        let sourceRoot = try makeTemporaryDirectory()
        let targetRoot = try makeTemporaryDirectory()
        defer {
            try? FileManager.default.removeItem(at: profileURL)
            try? FileManager.default.removeItem(at: sourceRoot)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        store.selectedRole = .source
        store.selectedTask = .profileInit
        store.profilePath = profileURL.path
        store.sourceRootPath = sourceRoot.path
        store.targetRootPath = targetRoot.path

        let gate = store.taskRunGate()

        XCTAssertFalse(gate.isRunnable)
        XCTAssertEqual(
            gate.note,
            "Choose a new migration config destination before running Create Config File. Existing config files must be opened, not overwritten."
        )
    }

    @MainActor
    func testCurrentInputBuildsServePreviewWithoutLeakingInternalReadyFile() {
        let store = AppStore()
        store.profilePath = "/tmp/profile.json"
        store.listenAddress = "127.0.0.1:4000"

        let contextSignature = store.currentContextSignature(for: .serve)
        store.installServeReadyFileForTesting("/tmp/internal-ready.json", contextSignature: contextSignature)

        XCTAssertEqual(
            SuperMoverTaskKind.serve.buildArguments(using: store.currentInput),
            ["serve", "--profile", "/tmp/profile.json", "--listen", "127.0.0.1:4000"]
        )
    }

    @MainActor
    func testCommandPreviewArgumentsMaskSelectedProfilePathWhileCurrentInputKeepsSSOTPath() {
        let store = AppStore()
        store.profilePath = "/tmp/source.profile.json"

        XCTAssertEqual(
            store.commandPreviewArguments(for: .status),
            ["status", "--profile", "<Selected Config: source.profile.json>", "--format", "json"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.status.buildArguments(using: store.currentInput),
            ["status", "--profile", "/tmp/source.profile.json", "--format", "json"]
        )
    }

    @MainActor
    func testCommandPreviewArgumentsShowProfileRequirementWhenNoProfileIsSelected() {
        let store = AppStore()

        XCTAssertEqual(
            store.commandPreviewArguments(for: .status),
            ["status", "--profile", "<Config File Required>", "--format", "json"]
        )
        XCTAssertEqual(
            SuperMoverTaskKind.status.buildArguments(using: store.currentInput),
            ["status", "--profile", "", "--format", "json"]
        )
    }

    @MainActor
    func testUIPreferencesDoNotChangeCommandInputsOrPreviewContracts() throws {
        let store = AppStore()
        let profileURL = makeTemporaryProfileURL(named: "ui-preferences.profile.json")
        try "{}".write(to: profileURL, atomically: true, encoding: .utf8)
        defer { try? FileManager.default.removeItem(at: profileURL) }
        store.selectedRole = .source
        store.selectedTask = .status
        store.profilePath = profileURL.path

        let representativeTasks: [SuperMoverTaskKind] = [
            .profileInit,
            .publish,
            .verify,
            .pair,
            .networkPush,
            .driftExpire,
            .pruneApprove,
            .syncQueueFail,
            .daemonRestart,
        ]
        let previewBefore = Dictionary(
            uniqueKeysWithValues: representativeTasks.map { ($0, store.commandPreviewArguments(for: $0)) }
        )
        let launchArgumentsBefore = Dictionary(
            uniqueKeysWithValues: representativeTasks.map { ($0, $0.buildArguments(using: store.currentInput)) }
        )
        let contextSignatureBefore = Dictionary(
            uniqueKeysWithValues: representativeTasks.map { ($0, store.currentContextSignature(for: $0)) }
        )
        let gateBefore = Dictionary(
            uniqueKeysWithValues: representativeTasks.map { ($0, store.taskRunGate(for: $0)) }
        )

        let previousAppearance = UserDefaults.standard.object(forKey: UIAppearancePreference.storageKey)
        let previousLanguage = UserDefaults.standard.object(forKey: UILanguagePreference.storageKey)
        defer {
            restoreStandardDefault(previousAppearance, forKey: UIAppearancePreference.storageKey)
            restoreStandardDefault(previousLanguage, forKey: UILanguagePreference.storageKey)
        }
        UserDefaults.standard.set(UIAppearancePreference.dark.rawValue, forKey: UIAppearancePreference.storageKey)
        UserDefaults.standard.set(UILanguagePreference.simplifiedChinese.rawValue, forKey: UILanguagePreference.storageKey)

        for task in representativeTasks {
            XCTAssertEqual(store.commandPreviewArguments(for: task), previewBefore[task], "\(task.rawValue) preview changed")
            XCTAssertEqual(task.buildArguments(using: store.currentInput), launchArgumentsBefore[task], "\(task.rawValue) arguments changed")
            XCTAssertEqual(store.currentContextSignature(for: task), contextSignatureBefore[task], "\(task.rawValue) context signature changed")
            XCTAssertEqual(store.taskRunGate(for: task), gateBefore[task], "\(task.rawValue) gate changed")
        }
    }

    @MainActor
    func testPairingReceiptDraftChangeClearsCurrentEvidence() throws {
        let store = AppStore()
        store.statusSnapshot = try decodeFixture(StatusSnapshot.self, from: statusReviewJSON())

        store.pairingReceipt.importReceiptFile = "/tmp/pairing-receipt.json"

        XCTAssertNil(store.statusSnapshot)
    }

    @MainActor
    func testProfileNetworkDraftChangeClearsCurrentEvidence() throws {
        let store = AppStore()
        store.reportSnapshot = try decodeFixture(ReportSnapshot.self, from: reportOKJSON())

        store.profileNetwork.receiverURL = "https://target.example.invalid:9443"

        XCTAssertNil(store.reportSnapshot)
    }

    @MainActor
    func testDriftRecordStructuredJSONIsRetainedAsReviewEvidence() {
        let store = AppStore()

        store.prepareStructuredEvidenceForLaunch(kind: .driftRecord)
        store.captureStructuredResult(
            for: .driftRecord,
            stdout: driftRecordReviewJSON(),
            stderr: "",
            exitCode: 1,
            contextSignature: store.currentContextSignature(for: .driftRecord)
        )

        XCTAssertEqual(store.driftRecordSnapshot?.detected, 1)
        XCTAssertEqual(store.driftRecordSnapshot?.records?.first?.id, "detected_file")
        XCTAssertEqual(store.evidenceEnvelopes[.driftRecord]?.exitCode, 1)
        XCTAssertEqual(store.evidenceEnvelopes[.driftRecord]?.freshness, .current)
    }

    @MainActor
    func testTaskContextTracksDiscoveryAndPairingInputs() {
        let store = AppStore()
        store.profilePath = "/tmp/profile.json"
        store.pairingTargetAddress = "10.0.0.20:39395"
        store.pairingVerificationCode = "123456"
        let firstPair = store.currentContextSignature(for: .pair)
        let pairRun = TaskRun(
            kind: .pair,
            slot: .foregroundAction,
            launchedAt: Date(),
            commandLine: [],
            contextSignature: firstPair,
            processIdentifier: nil,
            stdout: "pair: pinned target identity receipt=pairing-1 transfer=false\n",
            stderr: "",
            state: .finished(0)
        )
        XCTAssertTrue(store.isCurrentContext(pairRun))
        store.pairingVerificationCode = "654321"
        XCTAssertNotEqual(firstPair, store.currentContextSignature(for: .pair))
        XCTAssertFalse(store.isCurrentContext(pairRun))

        let firstBrowse = store.currentContextSignature(for: .discoverBrowse)
        store.discoveryBrowseListen = "127.0.0.1:0"
        XCTAssertNotEqual(firstBrowse, store.currentContextSignature(for: .discoverBrowse))

        let firstAdvertise = store.currentContextSignature(for: .discoverAdvertise)
        store.listenAddress = "127.0.0.1:5000"
        XCTAssertNotEqual(firstAdvertise, store.currentContextSignature(for: .discoverAdvertise))
        let secondAdvertise = store.currentContextSignature(for: .discoverAdvertise)
        store.discoveryAdvertiseListen = "0.0.0.0:0"
        XCTAssertNotEqual(secondAdvertise, store.currentContextSignature(for: .discoverAdvertise))
    }

    func testDiscoveryBrowseSnapshotDecodesCandidateClassification() throws {
        let json = """
        {
          "source": "lan_datagram",
          "listen": "127.0.0.1:39394",
          "candidate_count": 1,
          "invalid_packets": 0,
          "trusted": false,
          "candidates": [
            {
              "hint": {
                "address": "10.0.0.20:39395",
                "advertisement": {
                  "service_type": "_supermover._tcp",
                  "protocol_version": "supermover/1",
                  "ephemeral_nonce": "n123",
                  "capability_flags": ["pair"]
                },
                "seen_at": "2026-05-31T07:00:00Z",
                "expires_at": "2026-05-31T07:00:30Z",
                "trusted": false
              },
              "class": "ambiguous",
              "duplicate_count": 2,
              "ambiguity_reasons": ["same nonce seen from multiple addresses"]
            }
          ]
        }
        """.data(using: .utf8)!

        let decoded = try JSONDecoder().decode(DiscoveryBrowseSnapshot.self, from: json)

        XCTAssertEqual(decoded.candidate_count, 1)
        XCTAssertFalse(decoded.trusted)
        XCTAssertEqual(decoded.candidates.first?.classification, "ambiguous")
        XCTAssertEqual(decoded.candidates.first?.duplicate_count, 2)
        XCTAssertEqual(decoded.candidates.first?.hint.advertisement.capability_flags, ["pair"])
    }

    func testSyncQueueAndDiscoverRunSnapshotsDecodeReviewEvidence() throws {
        let queueJSON = """
        {
          "operation": "status",
          "mode": "queue_only",
          "state": "present",
          "summary": {
            "profile_id": "profile-local",
            "target_id": "local:profile-local",
            "queued": 1,
            "in_flight": 0,
            "backoff": 0,
            "canceled": 0,
            "done": 2,
            "failed": 0,
            "ready": 1,
            "total": 3,
            "warning_count": 0,
            "state_path": "/tmp/queue.json",
            "generated_at": "2026-05-31T07:00:00Z"
          },
          "entries": [
            {
              "id": "entry-1",
              "profile_id": "profile-local",
              "target_id": "local:profile-local",
              "root": "root-1",
              "path": ".hidden/secret.txt",
              "kind": "file",
              "mod_time": "2026-05-31T07:00:00Z",
              "enqueued_at": "2026-05-31T07:00:00Z",
              "status": "queued",
              "updated_at": "2026-05-31T07:00:00Z"
            }
          ]
        }
        """.data(using: .utf8)!
        let queue = try JSONDecoder().decode(SyncQueueSnapshot.self, from: queueJSON)
        XCTAssertEqual(queue.mode, "queue_only")
        XCTAssertEqual(queue.summary.ready, 1)
        XCTAssertEqual(queue.entries?.first?.path, ".hidden/secret.txt")

        let discoverJSON = """
        {
          "operation": "discover-run",
          "mode": "lan_discovery_gated_network_queue_consumer",
          "discovery": {
            "status": "no_matching_candidate",
            "reason": "timeout",
            "profile_address": "127.0.0.1:1",
            "candidate_count": 0,
            "invalid_packets": 0,
            "trusted": false
          }
        }
        """.data(using: .utf8)!
        let discover = try JSONDecoder().decode(SyncNetworkDiscoverRunSnapshot.self, from: discoverJSON)
        XCTAssertEqual(discover.discovery.status, "no_matching_candidate")
        XCTAssertFalse(discover.discovery.trusted)
        XCTAssertNil(discover.run)
        XCTAssertNil(discover.network)
    }

    func testDiscoverRunNoMatchWithZeroValuePayloadDoesNotCountAsExecutedRun() throws {
        let discover = try decodeFixture(
            SyncNetworkDiscoverRunSnapshot.self,
            from: """
            {
              "operation": "discover-run",
              "mode": "lan_discovery_gated_network_queue_consumer",
              "discovery": {
                "status": "no_matching_candidate",
                "reason": "timeout",
                "profile_address": "127.0.0.1:1",
                "candidate_count": 0,
                "invalid_packets": 0,
                "trusted": false
              },
              "enqueue": \(minimalQueueJSON(operation: "")),
              "run": \(minimalRunReceiptJSON(sessionID: "", status: "")),
              "network": {
                "profile_id": "",
                "target_id": "",
                "transfer": "",
                "encrypted_transfer": "",
                "files": 0,
                "bytes": 0,
                "warnings": 0
              }
            }
            """
        )

        XCTAssertEqual(discover.discovery.status, "no_matching_candidate")
        XCTAssertNotNil(discover.enqueue)
        XCTAssertNotNil(discover.run)
        XCTAssertNotNil(discover.network)
        XCTAssertFalse(discover.executedRun)
    }

    @MainActor
    func testRefreshAcceptanceBundleUsesProofAwareReviewStateWhenCollectedStatusIsStale() throws {
        let bundleRoot = try AcceptanceScriptHarness.makeDirectory(
            named: "appstore-refresh-acceptance-bundle-stale-collected-status"
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

        let store = AppStore()
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()

        XCTAssertEqual(store.acceptanceBundleSnapshot?.status, "evidence_collected")
        XCTAssertTrue(store.acceptanceBundleLoadError.isEmpty)
        XCTAssertTrue(store.note.contains("Loaded acceptance bundle evidence: review"))
        XCTAssertTrue(store.note.contains("meta.status=evidence_collected"))
        let event = try XCTUnwrap(store.appEvents.first)
        XCTAssertEqual(event.title, "acceptance bundle loaded")
        XCTAssertEqual(event.severity, .review)
        XCTAssertTrue(event.detail.contains("review"))
        XCTAssertTrue(event.detail.contains("meta.status=evidence_collected"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreflightBlocksBundledSourcePhaseOnBlockedAudit() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "blocked",
            targetAudit: "ready"
        )
        let profileURL = bundle.appendingPathComponent("source.profile.json")
        defer { try? FileManager.default.removeItem(at: bundle) }
        try #"{"schema":"supermover.profile.v1"}"#.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        let fakeResources = try makeFakePackagedResources(auditReady: false)
        defer { try? FileManager.default.removeItem(at: fakeResources.deletingLastPathComponent().deletingLastPathComponent()) }
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { fakeResources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { appBundleURL, outputURL in
                        try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
                            appBundleURL: appBundleURL,
                            outputURL: outputURL,
                            status: "blocked",
                            readiness: "blocked",
                            passReady: false,
                            blockingChecks: 6
                        )
                    }
                )
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .review,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "ad-hoc",
            gitDirty: true,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "local bundle review",
            detail: "review"
        )
        store.selectedRole = .source
        store.selectedTask = .pair
        store.profilePath = profileURL.path
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(error.contains("require ready source packaging audit"), "error:\n\(error)")
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreflightBlocksBundledTargetPhaseOnBlockedAudit() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "ready",
            targetAudit: "blocked"
        )
        let profileURL = bundle.appendingPathComponent("target.profile.json")
        defer { try? FileManager.default.removeItem(at: bundle) }
        try #"{"schema":"supermover.profile.v1"}"#.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        let fakeResources = try makeFakePackagedResources(auditReady: false)
        defer { try? FileManager.default.removeItem(at: fakeResources.deletingLastPathComponent().deletingLastPathComponent()) }
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { fakeResources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { appBundleURL, outputURL in
                        try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
                            appBundleURL: appBundleURL,
                            outputURL: outputURL,
                            status: "blocked",
                            readiness: "blocked",
                            passReady: false,
                            blockingChecks: 6
                        )
                    }
                )
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .review,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "ad-hoc",
            gitDirty: true,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "local bundle review",
            detail: "review"
        )
        store.selectedRole = .target
        store.selectedTask = .serve
        store.profilePath = profileURL.path
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .serve))

        XCTAssertTrue(error.contains("require ready target packaging audit"), "error:\n\(error)")
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreflightDoesNotBlockSameMachineBundle() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "same_machine",
            sourceAudit: "blocked",
            targetAudit: "blocked"
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .review,
            executablePath: "/definitely/missing/supermover",
            workingDirectoryPath: "/definitely/missing",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "ad-hoc",
            gitDirty: true,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "local bundle review",
            detail: "review"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .pair
        store.profilePath = "/tmp/profile.json"
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"

        store.runSelectedTask()

        XCTAssertFalse(store.note.contains("packaging audit"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreflightDoesNotBlockNonAcceptanceTask() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "blocked",
            targetAudit: "blocked"
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .review,
            executablePath: "/definitely/missing/supermover",
            workingDirectoryPath: "/definitely/missing",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "ad-hoc",
            gitDirty: true,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "local bundle review",
            detail: "review"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .status
        store.profilePath = "/tmp/profile.json"

        store.runSelectedTask()

        XCTAssertFalse(store.note.contains("packaging audit"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreflightBlocksWhenFreshPackagingEvidenceWouldMakeLocalNotarizationStale() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true,
            includeInstalledAppProof: true,
            includeStrictPhaseArtifacts: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        let fakeResources = try makeFakePackagedResources(auditReady: true, notarizationReady: true)
        defer { try? FileManager.default.removeItem(at: fakeResources.deletingLastPathComponent().deletingLastPathComponent()) }
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { fakeResources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { appBundleURL, outputURL in
                        try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
                            appBundleURL: appBundleURL,
                            outputURL: outputURL,
                            readiness: "ready"
                        )
                    }
                )
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: "/definitely/missing/supermover",
            workingDirectoryPath: "/definitely/missing",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .pair
        store.profilePath = "/tmp/profile.json"
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(
            error.contains("could not record source packaging evidence before phase execution"),
            "error:\n\(error)"
        )
        XCTAssertTrue(error.contains("does not match the current packaged app"), "error:\n\(error)")
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewBlocksWhenFreshPackagingEvidenceWouldMakeLocalNotarizationStale() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true,
            includeInstalledAppProof: true,
            includeStrictPhaseArtifacts: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        let fakeResources = try makeFakePackagedResources(auditReady: true, notarizationReady: true)
        defer { try? FileManager.default.removeItem(at: fakeResources.deletingLastPathComponent().deletingLastPathComponent()) }
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { fakeResources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { appBundleURL, outputURL in
                        try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
                            appBundleURL: appBundleURL,
                            outputURL: outputURL,
                            readiness: "ready"
                        )
                    }
                )
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: fakeResources.appendingPathComponent("bin/supermover").path,
            workingDirectoryPath: fakeResources.appendingPathComponent("bin").path,
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("does not match the current packaged app"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewBlocksContradictoryInstalledAppProof() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true,
            includeInstalledAppProof: true,
            contradictoryInstalledAppProof: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        let fakeResources = try makeFakePackagedResources(auditReady: true, notarizationReady: true)
        defer { try? FileManager.default.removeItem(at: fakeResources.deletingLastPathComponent().deletingLastPathComponent()) }
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { fakeResources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { appBundleURL, outputURL in
                        try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
                            appBundleURL: appBundleURL,
                            outputURL: outputURL
                        )
                    }
                )
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: "/definitely/missing/supermover",
            workingDirectoryPath: "/definitely/missing",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .pair

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("contradictory archive handoff evidence"))
        XCTAssertEqual(store.selectedTaskAcceptanceLaunchPreview, preview)
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewBlocksWhenOtherMachineReleaseEvidenceIsMissing() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "blocked",
            includeNotarizationEvidence: true,
            includeInstalledAppProof: true,
            includeStrictPhaseArtifacts: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        let fakeResources = try makeFakePackagedResources(auditReady: true, notarizationReady: true)
        defer { try? FileManager.default.removeItem(at: fakeResources.deletingLastPathComponent().deletingLastPathComponent()) }
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { fakeResources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { appBundleURL, outputURL in
                        try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
                            appBundleURL: appBundleURL,
                            outputURL: outputURL
                        )
                    }
                )
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: fakeResources.appendingPathComponent("bin/supermover").path,
            workingDirectoryPath: fakeResources.appendingPathComponent("bin").path,
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .pair

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("target packaging evidence"))
        XCTAssertTrue(preview.detail.contains("target.app-audit.json is not install-ready"))
        XCTAssertEqual(store.selectedTaskAcceptanceLaunchPreview, preview)
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreflightBlocksContradictoryInstalledAppProof() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true,
            includeInstalledAppProof: true,
            contradictoryInstalledAppProof: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        let fakeResources = try makeFakePackagedResources(auditReady: true, notarizationReady: true)
        defer { try? FileManager.default.removeItem(at: fakeResources.deletingLastPathComponent().deletingLastPathComponent()) }
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { fakeResources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { appBundleURL, outputURL in
                        try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
                            appBundleURL: appBundleURL,
                            outputURL: outputURL
                        )
                    }
                )
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: "/definitely/missing/supermover",
            workingDirectoryPath: "/definitely/missing",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .pair
        store.profilePath = "/tmp/profile.json"
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(error.contains("contradictory archive handoff evidence"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreflightBlocksWhenOtherMachineReleaseEvidenceIsMissing() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "blocked",
            includeNotarizationEvidence: true,
            includeInstalledAppProof: true,
            includeStrictPhaseArtifacts: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        let fakeResources = try makeFakePackagedResources(auditReady: true, notarizationReady: true)
        defer { try? FileManager.default.removeItem(at: fakeResources.deletingLastPathComponent().deletingLastPathComponent()) }
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { fakeResources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { appBundleURL, outputURL in
                        try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
                            appBundleURL: appBundleURL,
                            outputURL: outputURL
                        )
                    }
                )
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: fakeResources.appendingPathComponent("bin/supermover").path,
            workingDirectoryPath: fakeResources.appendingPathComponent("bin").path,
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .pair
        store.profilePath = "/tmp/profile.json"
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(error.contains("target packaging evidence"))
        XCTAssertTrue(error.contains("target.app-audit.json is not install-ready"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreflightAllowsDistributionReadyBundledAcceptanceTask() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        let fakeResources = try makeFakePackagedResources(auditReady: true, notarizationReady: true)
        defer { try? FileManager.default.removeItem(at: fakeResources.deletingLastPathComponent().deletingLastPathComponent()) }
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { fakeResources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { appBundleURL, outputURL in
                        try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
                            appBundleURL: appBundleURL,
                            outputURL: outputURL
                        )
                    }
                )
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: "/definitely/missing/supermover",
            workingDirectoryPath: "/definitely/missing",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .pair
        store.profilePath = "/tmp/profile.json"
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"

        let error = store.acceptanceInstalledAppLaunchPreflightError(for: .pair)

        XCTAssertNil(error)
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreflightAcceptsNotarizeScriptSidecarWorkflow() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let harness = try makeNotaryHarness(
            auditStatus: "pass",
            auditReadiness: "distribution_ready",
            auditPassReady: true,
            auditBlockingChecks: 0
        )
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }

        let notarize = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [
                "PATH": "\(harness.binURL.path):\(ProcessInfo.processInfo.environment["PATH"] ?? "")",
                "SUPERMOVER_NOTARY_KEYCHAIN_PROFILE": "supermover-ci",
                "SUPERMOVER_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
                "SUPERMOVER_TEST_NOTARY_ARGS_DIR": harness.argsURL.path,
                "SUPERMOVER_TEST_NOTARY_SUBMIT_JSON": harness.submitJSONURL.path,
                "SUPERMOVER_TEST_NOTARY_LOG_JSON": harness.logJSONURL.path,
            ],
            allowNonZeroExit: false
        )
        XCTAssertEqual(notarize.exitCode, 0)

        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let resources = app.appURL.appendingPathComponent("Contents/Resources", isDirectory: true)
        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { appBundleURL, outputURL in
                        try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
                            appBundleURL: appBundleURL,
                            outputURL: outputURL
                        )
                    }
                )
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: resources.appendingPathComponent("bin/supermover").path,
            workingDirectoryPath: resources.appendingPathComponent("bin").path,
            bundleIdentifier: "dev.supermover.macapp",
            appVersion: "0.1.0",
            provenancePath: resources.appendingPathComponent("supermover-provenance.json").path,
            provenanceStatus: "loaded",
            bundleCommit: head,
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "-",
            gitDirty: true,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "workflow-sidecar"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .pair
        store.profilePath = "/tmp/profile.json"
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"

        let error = store.acceptanceInstalledAppLaunchPreflightError(for: .pair)

        XCTAssertNil(error)
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarization?.output, "source.notarization.json")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarizationArtifact?.status, "pass")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarizationArtifact?.submission?.status, "Accepted")
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreflightBlocksDevelopmentCLI() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .development,
            readinessLevel: .review,
            executablePath: "/tmp/supermover-dev",
            workingDirectoryPath: "/tmp",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: nil,
            gitDirty: true,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "development launcher",
            detail: "development"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(error.contains("require a packaged app"))
        XCTAssertTrue(error.contains("development launcher"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreflightBlocksUnavailableCLI() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .unavailable,
            readinessLevel: .blocked,
            executablePath: "",
            workingDirectoryPath: "",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "missing",
            bundleCommit: nil,
            bundledCLIVersion: nil,
            buildProfile: nil,
            signing: nil,
            gitDirty: nil,
            builtAt: nil,
            readiness: "not available",
            detail: "unavailable"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(error.contains("require a packaged app"))
        XCTAssertTrue(error.contains("not available"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewBlocksDevelopmentCLI() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "ready",
            targetAudit: "ready"
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .development,
            readinessLevel: .review,
            executablePath: "/tmp/supermover-dev",
            workingDirectoryPath: "/tmp",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: nil,
            gitDirty: true,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "development launcher",
            detail: "development"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("require a packaged app"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewReviewsBlockedAudit() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "blocked",
            targetAudit: "ready"
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .review,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "ad-hoc",
            gitDirty: true,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "local bundle review",
            detail: "review"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .review, "detail:\n\(preview.detail)")
        XCTAssertTrue(preview.detail.contains("Loaded source packaging audit is blocked"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewReviewsMissingNotarizationEvidence() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready"
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .review)
        XCTAssertTrue(preview.detail.contains("No loaded source notarization evidence"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewDoesNotTrustStaleMetaWhenSourceAppAuditArtifactIsMissing() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }
        try FileManager.default.removeItem(at: bundle.appendingPathComponent("source.app-audit.json"))

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .review)
        XCTAssertTrue(preview.detail.contains("No loaded source packaging audit"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewReviewsReadyAuditWhenLoadedPackagingIsNotInstallReady() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "ready",
            targetAudit: "ready",
            includeNotarizationEvidence: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "ready",
            detail: "ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .review)
        XCTAssertTrue(preview.detail.contains("Loaded source packaging audit is ready"), preview.detail)
        XCTAssertTrue(
            preview.detail.contains("still require ready audit before phase execution"),
            preview.detail
        )
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewReviewsDistributionReadyAuditWhenDistinctMachineProofIsIncomplete() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .review)
        XCTAssertTrue(
            preview.detail.contains("installed-app machine identity correction"),
            preview.detail
        )
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewReviewsWhenDistinctMachineProofIsReadyButCurrentStrictEvaluationIsMissing() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true,
            includeInstalledAppProof: true,
            includeStrictPhaseArtifacts: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .review)
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewPassesWhenDistinctMachineProofAndCurrentStrictEvaluationAreReady() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true,
            includeInstalledAppProof: true,
            includeCurrentStrictEvaluation: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .pass, "detail:\n\(preview.detail)")
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewBlocksMissingLocalNotarizationEvidence() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let resources = try makeFakePackagedResources(auditReady: true, notarizationReady: false)
        defer { try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent()) }

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: resources.appendingPathComponent("bin/supermover").path,
            workingDirectoryPath: resources.appendingPathComponent("bin").path,
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("local source notarization evidence is missing"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewBlocksLocalNotarizationThatIsNotReleaseReady() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let resources = try makeFakePackagedResources(auditReady: true, notarizationReady: true)
        defer { try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent()) }
        let appURL = resources.deletingLastPathComponent().deletingLastPathComponent()
        let notaryDir = appURL.deletingLastPathComponent().appendingPathComponent("\(appURL.lastPathComponent).notary")
        let sidecarURL = notaryDir.appendingPathComponent("notarization.json")
        let auditURL = notaryDir.appendingPathComponent("post-staple.audit.json")
        let notaryLogURL = notaryDir.appendingPathComponent("notary-log.json")
        try """
        {
          "schema": "supermover.macos.notarization.v1",
          "checked_at": "2026-06-01T12:00:00Z",
          "status": "pass",
          "app_path": "\(appURL.path)",
          "auth_mode": "keychain_profile",
          "submission": {
            "id": "\(AcceptanceReleaseEvidenceFixtures.defaultSubmissionID)",
            "status": "In Progress"
          },
          "notary_log": {
            "path": "\(notaryLogURL.path)"
          },
          "audit": {
            "path": "\(auditURL.path)",
            "status": "pass",
            "readiness": "distribution_ready",
            "pass_ready": true
          }
        }
        """.write(to: sidecarURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: resources.appendingPathComponent("bin/supermover").path,
            workingDirectoryPath: resources.appendingPathComponent("bin").path,
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("not release-ready"), preview.detail)
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewBlocksMalformedLocalNotarizationEvidence() throws {
        let bundle = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready",
            includeNotarizationEvidence: true
        )
        defer { try? FileManager.default.removeItem(at: bundle) }

        let resources = try makeFakePackagedResources(auditReady: true, notarizationReady: true)
        defer { try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent()) }
        let appURL = resources.deletingLastPathComponent().deletingLastPathComponent()
        let sidecarURL = appURL.deletingLastPathComponent().appendingPathComponent("\(appURL.lastPathComponent).notary/notarization.json")
        try #"{"schema":"bad"}"#.write(to: sidecarURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources }
        )
        let executablePath = resources.appendingPathComponent("bin/supermover").path
        let workingDirectoryPath = resources.appendingPathComponent("bin").path
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: executablePath,
            workingDirectoryPath: workingDirectoryPath,
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "developer-id",
            gitDirty: false,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
        store.acceptanceBundlePath = bundle.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("notarization evidence is malformed"))
    }

    @MainActor
    func testAcceptanceTwoMachineLaunchPreviewSkipsSameMachineAndNonAcceptanceTasks() throws {
        let sameMachineBundle = try makeAcceptanceBundle(
            collectionMode: "same_machine",
            sourceAudit: "blocked",
            targetAudit: "blocked"
        )
        defer { try? FileManager.default.removeItem(at: sameMachineBundle) }

        let store = AppStore()
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .review,
            executablePath: "/tmp/SuperMover.app/Contents/Resources/bin/supermover",
            workingDirectoryPath: "/tmp/SuperMover.app/Contents/Resources/bin",
            bundleIdentifier: "dev.supermover",
            appVersion: "0.1",
            provenancePath: nil,
            provenanceStatus: "loaded",
            bundleCommit: "abcdef",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "ad-hoc",
            gitDirty: true,
            builtAt: "2026-06-01T00:00:00Z",
            readiness: "local bundle review",
            detail: "review"
        )
        store.acceptanceBundlePath = sameMachineBundle.path
        store.refreshAcceptanceBundle()

        XCTAssertNil(store.acceptanceInstalledAppLaunchPreview(for: .pair))
        XCTAssertNil(store.acceptanceInstalledAppLaunchPreview(for: .status))
    }

    @MainActor
    func testAcceptanceAutoRecordPairRewritesStaleSourceMachineIdentity() throws {
        let dir = try makeAcceptanceBundleWithMachineIdentityOnly(
            sourceMachineID: "stale-source-machine",
            sourceMachineLabel: "old-source",
            targetMachineID: "target-machine",
            targetMachineLabel: "target"
        )
        defer { try? FileManager.default.removeItem(at: dir) }

        let receiptDir = dir.appendingPathComponent("local-pairing-receipts", isDirectory: true)
        try FileManager.default.createDirectory(at: receiptDir, withIntermediateDirectories: true)
        let receiptURL = receiptDir.appendingPathComponent("pair-1.json")
        try """
        {
          "version": 1,
          "id": "pair-1",
          "profile_id": "profile-src",
          "target_id": "target-1",
          "source_device_id": "sha256:1111111111111111111111111111111111111111111111111111111111111111",
          "target_device_id": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
          "device_public_key": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
          "method": "sas",
          "verified_at": "2026-06-04T00:00:00Z",
          "verification_hash": "hash-1",
          "protocol_version": "supermover/v1"
        }
        """.write(to: receiptURL, atomically: true, encoding: .utf8)

        let profileURL = dir.appendingPathComponent("source.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1",
            "local_pairing_receipt_path": "\(receiptURL.path)"
          }
        }
        """.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"
        store.acceptanceBundleAuthoringCoordinator = AcceptanceBundleAuthoringCoordinator(
            writer: AcceptanceBundleArtifactWriter(
                machineIdentityResolver: AcceptanceMachineIdentityResolver(
                    resolveCurrentMachine: {
                        AcceptanceMachineIdentity(machineID: "source-machine", machineLabel: "source")
                    }
                )
            )
        )

        store.triggerAcceptanceAutoRecordForTesting(.pair)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.roles["source_pair"]?.machine_id, "source-machine")
        XCTAssertEqual(snapshot.sourceMachineFacts?.machine_id, "source-machine")
        XCTAssertEqual(snapshot.sourceMachineFactsArtifact?.machine_id, "source-machine")
        XCTAssertEqual(snapshot.sourcePairArtifact?.receipt_path, "exported-receipts/pair-1.json")
        XCTAssertFalse(snapshot.installedAppCollectionProof.requiresMachineIdentityCorrection)
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.finalEvaluationBundleHandoffDetail,
            "missing verified bundle_handoffs"
        )
    }

    @MainActor
    func testAcceptanceAutoRecordServeRewritesStaleTargetMachineIdentity() throws {
        let dir = try makeAcceptanceBundleWithMachineIdentityOnly(
            sourceMachineID: "source-machine",
            sourceMachineLabel: "source",
            targetMachineID: "stale-target-machine",
            targetMachineLabel: "old-target"
        )
        defer { try? FileManager.default.removeItem(at: dir) }

        let profileURL = dir.appendingPathComponent("target.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1"
          }
        }
        """.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.selectedRole = .target
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.acceptanceServePhase = "1"
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: "127.0.0.1:39395",
            verification_code: "123456",
            mode: "pairing",
            receiver_address: "127.0.0.1:9443",
            receiver_routes: true,
            push_network: true,
            trusted: true,
            transfer: true,
            expires_at: nil
        )
        store.acceptanceBundleAuthoringCoordinator = AcceptanceBundleAuthoringCoordinator(
            writer: AcceptanceBundleArtifactWriter(
                machineIdentityResolver: AcceptanceMachineIdentityResolver(
                    resolveCurrentMachine: {
                        AcceptanceMachineIdentity(machineID: "target-machine", machineLabel: "target")
                    }
                )
            )
        )

        store.triggerAcceptanceAutoRecordForTesting(.serve)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.roles["target"]?.machine_id, "target-machine")
        XCTAssertEqual(snapshot.targetMachineFacts?.machine_id, "target-machine")
        XCTAssertEqual(snapshot.targetMachineFactsArtifact?.machine_id, "target-machine")
        XCTAssertFalse(snapshot.installedAppCollectionProof.requiresMachineIdentityCorrection)
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.finalEvaluationBundleHandoffDetail,
            "missing verified bundle_handoffs"
        )
    }

    @MainActor
    func testSyncInputChangesClearPromotedSyncSnapshots() throws {
        let store = AppStore()
        store.syncQueueSnapshot = try decodeFixture(SyncQueueSnapshot.self, from: minimalQueueJSON(operation: "status"))
        store.syncRunSnapshot = try decodeFixture(SyncRunSnapshot.self, from: minimalSyncRunJSON())
        store.syncLoopSnapshot = try decodeFixture(
            SyncLoopSnapshot.self,
            from: """
            {
              "operation": "loop",
              "mode": "local_queue_consumer",
              "session_prefix": "sync-loop",
              "interval": "1m",
              "max_runs": 0,
              "status": "idle",
              "completed_runs": 0,
              "published_runs": 0,
              "idle_runs": 0,
              "retrying_runs": 0
            }
            """
        )
        store.syncNetworkDiscoverRunSnapshot = try decodeFixture(
            SyncNetworkDiscoverRunSnapshot.self,
            from: """
            {
              "operation": "discover-run",
              "mode": "lan_discovery_gated_network_queue_consumer",
              "discovery": {
                "status": "no_matching_candidate",
                "reason": "timeout",
                "profile_address": "127.0.0.1:1",
                "candidate_count": 0,
                "invalid_packets": 0,
                "trusted": false
              },
              "enqueue": \(minimalQueueJSON(operation: "")),
              "run": \(minimalRunReceiptJSON(sessionID: "", status: "")),
              "network": {
                "profile_id": "",
                "target_id": "",
                "transfer": "",
                "encrypted_transfer": "",
                "files": 0,
                "bytes": 0,
                "warnings": 0
              }
            }
            """
        )
        store.artifactReadProblems = [
            ArtifactReadProblem(
                occurredAt: Date(),
                artifactKind: .syncRun,
                task: .syncRun,
                problem: "sync sample",
                rawSample: "{}"
            ),
            ArtifactReadProblem(
                occurredAt: Date(),
                artifactKind: .status,
                task: .status,
                problem: "status sample",
                rawSample: "{}"
            ),
        ]

        store.syncRetryBackoff = "2m"

        XCTAssertNil(store.syncQueueSnapshot)
        XCTAssertNil(store.syncRunSnapshot)
        XCTAssertNil(store.syncLoopSnapshot)
        XCTAssertNil(store.syncNetworkDiscoverRunSnapshot)
        XCTAssertEqual(store.artifactReadProblems.map(\.artifactKind), [.status])
    }

    private func decodeFixture<T: Decodable>(_ type: T.Type, from json: String) throws -> T {
        let data = try XCTUnwrap(json.data(using: .utf8))
        return try JSONDecoder().decode(T.self, from: data)
    }

    private func makeAcceptanceBundle(
        collectionMode: String,
        sourceAudit: String,
        targetAudit: String,
        includeNotarizationEvidence: Bool = false,
        includeInstalledAppProof: Bool = false,
        contradictoryInstalledAppProof: Bool = false,
        includeStrictPhaseArtifacts: Bool = false,
        includeCurrentStrictEvaluation: Bool = false,
        sourceConsistencyJSON: String? = nil
    ) throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("acceptance-bundle-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let sourceAppPath = "/tmp/current-source/SuperMover.app"
        let targetAppPath = "/tmp/current-target/SuperMover.app"
        let roles = includeInstalledAppProof ? """
          "roles": {
            "source_pair": {
              "profile": "/tmp/source.profile.json",
              "status": "recorded",
              "machine_id": "source-machine",
              "machine_label": "source"
            },
            "target": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "target-machine",
              "machine_label": "target"
            }
          },
        """ : """
          "roles": {},
        """
        let bundleHandoffsJSON = contradictoryInstalledAppProof ? """
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
        """ : """
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
              }
            ]
        """
        let installedAppProofEvidence = includeInstalledAppProof ? """
            ,
            "machine_facts": {
              "source": {
                "output": "source.machine.json",
                "machine_id": "source-machine",
                "machine_label": "source"
              },
              "target": {
                "output": "target.machine.json",
                "machine_id": "target-machine",
                "machine_label": "target"
              }
            },
            "bundle_handoffs": \(bundleHandoffsJSON)
        """ : ""
        let notarizationEvidenceMeta = includeNotarizationEvidence ? """
            ,
            "notarization": {
              "source": {
                "collected_by": "test",
                "output": "source.notarization.json",
                "status": "\(sourceAudit == "blocked" ? "blocked" : "pass")",
                "audit_status": "\(sourceAudit == "blocked" ? "blocked" : "pass")",
                "audit_readiness": "\(sourceAudit == "blocked" ? "blocked" : sourceAudit)",
                "audit_pass_ready": \(sourceAudit != "blocked")
              },
              "target": {
                "collected_by": "test",
                "output": "target.notarization.json",
                "status": "\(targetAudit == "blocked" ? "blocked" : "pass")",
                "audit_status": "\(targetAudit == "blocked" ? "blocked" : "pass")",
                "audit_readiness": "\(targetAudit == "blocked" ? "blocked" : targetAudit)",
                "audit_pass_ready": \(targetAudit != "blocked")
              }
            }
        """ : ""
        let includeStrictAcceptanceFlow = includeCurrentStrictEvaluation || includeStrictPhaseArtifacts
        let effectiveSourceConsistencyJSON = sourceConsistencyJSON ?? (includeStrictAcceptanceFlow ? """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "status": "pass",
              "mode": "current_source_verified",
              "session_id": "session-1",
              "detail": "test fixture"
            }
            """ : nil)
        let acceptanceFlowMeta = includeStrictAcceptanceFlow ? """
            ,
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "discovery": {
              "source_browse": {
                "output": "source.browse.json",
                "trusted": false
              },
              "target_advertise": {
                "output": "target.advertise.json",
                "trusted": false
              }
            },
            "source_pair": {
              "output": "source.pair.json",
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "pair": "source.pair.txt"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "push": "source.network-push.txt",
              "verify": "source.verify.json",
              "status": "source.status.json",
              "report": "source.report.json",
              "health": "source.health.json"
            },
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted",
                "machine_id": "target-machine"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed",
                "machine_id": "target-machine"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched",
                "machine_id": "source-machine"
              }
            }
        """ : ""
        let sourceConsistencyMeta = effectiveSourceConsistencyJSON == nil ? "" : """
            ,
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "\(includeStrictAcceptanceFlow ? "pass" : "review")",
              "mode": "\(includeStrictAcceptanceFlow ? "current_source_verified" : "bundle_loaded")"
            }
        """
        let evaluationMeta = includeCurrentStrictEvaluation ? """
            ,
            "evaluation": {
              "pairing_receipt_id": "pair-1",
              "session_id": "session-1",
              "target_root": "/tmp/current-target",
              "output": "evaluation.json",
              "require_operator_evidence": true
            }
        """ : ""
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "\(includeCurrentStrictEvaluation ? "evidence_collected" : "in_progress")",
          "collection": {
            "mode": "\(collectionMode)",
            "machine_count": \(collectionMode == "two_machine" ? "2" : "1")
          },
          \(roles)
          "evidence": {
            "app_audit": {
              "source": {
                "collected_by": "test",
                "output": "source.app-audit.json",
                "exit_code": \(sourceAudit == "blocked" ? "1" : "0"),
                "status": "\(sourceAudit == "blocked" ? "blocked" : "pass")",
                "readiness": "\(sourceAudit == "blocked" ? "blocked" : sourceAudit)",
                "pass_ready": \(sourceAudit == "blocked" ? "false" : "true"),
                "blocking_checks": \(sourceAudit == "blocked" ? "6" : "0")
              },
              "target": {
                "collected_by": "test",
                "output": "target.app-audit.json",
                "exit_code": \(targetAudit == "blocked" ? "1" : "0"),
                "status": "\(targetAudit == "blocked" ? "blocked" : "pass")",
                "readiness": "\(targetAudit == "blocked" ? "blocked" : targetAudit)",
                "pass_ready": \(targetAudit == "blocked" ? "false" : "true"),
                "blocking_checks": \(targetAudit == "blocked" ? "6" : "0")
              }
            }\(notarizationEvidenceMeta)\(installedAppProofEvidence)\(acceptanceFlowMeta)\(sourceConsistencyMeta)\(evaluationMeta)
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(
            cliVersion: "supermover 0.1.0-dev"
        )
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest).write(
            to: dir.appendingPathComponent("source.provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest).write(
            to: dir.appendingPathComponent("target.provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: sourceAppPath,
            provenanceManifest: provenanceManifest,
            status: sourceAudit == "blocked" ? "blocked" : "pass",
            readiness: sourceAudit == "blocked" ? "blocked" : sourceAudit,
            passReady: sourceAudit != "blocked",
            blockingChecks: sourceAudit == "blocked" ? 6 : 0
        ).write(
            to: dir.appendingPathComponent("source.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: targetAppPath,
            provenanceManifest: provenanceManifest,
            status: targetAudit == "blocked" ? "blocked" : "pass",
            readiness: targetAudit == "blocked" ? "blocked" : targetAudit,
            passReady: targetAudit != "blocked",
            blockingChecks: targetAudit == "blocked" ? 6 : 0
        ).write(
            to: dir.appendingPathComponent("target.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        if includeNotarizationEvidence {
            try AcceptanceReleaseEvidenceFixtures.writeBundleNotaryLog(bundleRoot: dir, machine: "source")
            try AcceptanceReleaseEvidenceFixtures.writeBundleNotaryLog(bundleRoot: dir, machine: "target")
            try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
                appPath: sourceAppPath,
                auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(
                    appPath: sourceAppPath
                ),
                status: sourceAudit == "blocked" ? "blocked" : "pass",
                submissionStatus: sourceAudit == "blocked" ? "Invalid" : "Accepted",
                notaryLogPath: "source.notary-log.json",
                auditStatus: sourceAudit == "blocked" ? "blocked" : "pass",
                auditReadiness: sourceAudit == "blocked" ? "blocked" : sourceAudit,
                auditPassReady: sourceAudit != "blocked"
            ).write(
                to: dir.appendingPathComponent("source.notarization.json"),
                atomically: true,
                encoding: .utf8
            )
            try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
                appPath: targetAppPath,
                auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(
                    appPath: targetAppPath
                ),
                status: targetAudit == "blocked" ? "blocked" : "pass",
                submissionStatus: targetAudit == "blocked" ? "Invalid" : "Accepted",
                notaryLogPath: "target.notary-log.json",
                auditStatus: targetAudit == "blocked" ? "blocked" : "pass",
                auditReadiness: targetAudit == "blocked" ? "blocked" : targetAudit,
                auditPassReady: targetAudit != "blocked"
            ).write(
                to: dir.appendingPathComponent("target.notarization.json"),
                atomically: true,
                encoding: .utf8
            )
        }
        if includeInstalledAppProof {
            try """
            {
              "schema": "supermover.acceptance.machine_facts.v1",
              "machine_id": "source-machine",
              "machine_label": "source"
            }
            """.write(to: dir.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
            try """
            {
              "schema": "supermover.acceptance.machine_facts.v1",
              "machine_id": "target-machine",
              "machine_label": "target"
            }
            """.write(to: dir.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        }
        if let effectiveSourceConsistencyJSON {
            try effectiveSourceConsistencyJSON.write(
                to: dir.appendingPathComponent("source.consistency.json"),
                atomically: true,
                encoding: .utf8
            )
        }
        if includeStrictAcceptanceFlow {
            try """
            {
              "source": "browse",
              "listen": "0.0.0.0:39394",
              "candidate_count": 1,
              "invalid_packets": 0,
              "trusted": false,
              "candidates": [
                {
                  "hint": {
                    "address": "127.0.0.1:39395",
                    "advertisement": {
                      "service_type": "_supermover._udp",
                      "protocol_version": "1",
                      "ephemeral_nonce": "nonce-1",
                      "capability_flags": ["pairing"]
                    },
                    "seen_at": "2026-01-01T00:00:00Z",
                    "expires_at": "2026-01-01T00:00:30Z",
                    "trusted": false
                  },
                  "class": "candidate",
                  "duplicate_count": 1,
                  "ambiguity_reasons": []
                }
              ]
            }
            """.write(
                to: dir.appendingPathComponent("source.browse.json"),
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
                to: dir.appendingPathComponent("target.advertise.json"),
                atomically: true,
                encoding: .utf8
            )
            try """
            {
              "profile": "/tmp/source.profile.json",
              "target_address": "127.0.0.1:39395",
              "verification_code": "123456",
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json"
            }
            """.write(
                to: dir.appendingPathComponent("source.pair.json"),
                atomically: true,
                encoding: .utf8
            )
            try """
            {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing",
              "receiver_address": "127.0.0.1:9443",
              "receiver_routes": true,
              "push_network": true,
              "trusted": false,
              "transfer": true
            }
            """.write(
                to: dir.appendingPathComponent("target.ready.json"),
                atomically: true,
                encoding: .utf8
            )
            try FileManager.default.createDirectory(
                at: dir.appendingPathComponent("exported-receipts", isDirectory: true),
                withIntermediateDirectories: true
            )
            try """
            {
              "version": 1,
              "id": "pair-1",
              "profile_id": "profile-src",
              "target_id": "target-1",
              "source_device_id": "sha256:1111111111111111111111111111111111111111111111111111111111111111",
              "target_device_id": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
              "device_public_key": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
              "method": "sas",
              "verified_at": "2026-06-04T00:00:00Z",
              "verification_hash": "hash-1",
              "protocol_version": "supermover/v1"
            }
            """.write(
                to: dir.appendingPathComponent("exported-receipts/pair-1.json"),
                atomically: true,
                encoding: .utf8
            )
            try "pair ok".write(
                to: dir.appendingPathComponent("source.pair.txt"),
                atomically: true,
                encoding: .utf8
            )
            try """
            {
              "profile": "/tmp/source.profile.json",
              "session_id": "session-1",
              "target_address": "127.0.0.1:39395",
              "receiver_address": "127.0.0.1:9443",
              "target_mode": "pairing"
            }
            """.write(
                to: dir.appendingPathComponent("source.transfer.json"),
                atomically: true,
                encoding: .utf8
            )
            try "receipt adopted".write(
                to: dir.appendingPathComponent("target.adopt-pairing.txt"),
                atomically: true,
                encoding: .utf8
            )
            try "push ok".write(
                to: dir.appendingPathComponent("source.network-push.txt"),
                atomically: true,
                encoding: .utf8
            )
            try """
            {
              "schema": "supermover.acceptance.current_source_consistency.v1",
              "profile_id": "profile-1",
              "root_id": "root-1",
              "root_path": "/tmp/source",
              "session_id": "session-1",
              "created_at": "2026-01-01T00:00:00Z",
              "entries": []
            }
            """.write(
                to: dir.appendingPathComponent("source.baseline.json"),
                atomically: true,
                encoding: .utf8
            )
            try """
            {
              "target_root": "/tmp/current-target",
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
            """.write(
                to: dir.appendingPathComponent("source.verify.json"),
                atomically: true,
                encoding: .utf8
            )
            try """
            {
              "profile_id": "profile-local",
              "target_id": "local:profile-local",
              "target_root": "/tmp/current-target",
              "overall": {
                "status": "clean",
                "issues": []
              },
              "summary": {
                "warnings": 0,
                "soft_deletes": 0,
                "target_drifts": 0,
                "live_target_drifts": 0,
                "prune_candidates": 0,
                "prune_refusals": 0,
                "prune_approvals": 0,
                "network_transfers": 0,
                "artifact_problems": 0
              },
              "latest_session": {
                "id": "session-1",
                "manifest_id": "manifest-1",
                "created_at": "2026-05-31T07:00:00Z",
                "entries": 2,
                "files": 2,
                "completeness": {
                  "status": "complete",
                  "files_expected": 2,
                  "files_verified": 2,
                  "verification_errors": 0,
                  "verification_warnings": 0
                }
              },
              "prune_review": {
                "status": "clean",
                "approval_required": false,
                "apply": "none",
                "summary": {
                  "candidates": 0,
                  "refusals": 0,
                  "approvals": 0,
                  "unapplied_approvals": 0,
                  "receipt_issues": 0
                }
              },
              "pairing": {
                "status": "paired_receipt_valid",
                "receipt_id": "pair-1",
                "encrypted_transfer": "available",
                "issue": null
              },
              "privacy": {
                "status": "accepted",
                "claim": "bounded",
                "network_transfer": "encrypted"
              },
              "health": {
                "healthy": true,
                "summary": {
                  "incomplete_sessions": 0,
                  "invalid_records": 0,
                  "artifact_problems": 0,
                  "target_drifts": 0,
                  "network_transfers": 0
                }
              },
              "artifact_problems": [],
              "network_transfers": []
            }
            """.write(
                to: dir.appendingPathComponent("source.report.json"),
                atomically: true,
                encoding: .utf8
            )
            try """
            {
              "profile_id": "profile-local",
              "target_id": "local:profile-local",
              "target_root": "/tmp/current-target",
              "overall": {
                "status": "clean",
                "target_status": "ready"
              },
              "issues": [],
              "latest_session": {
                "id": "session-1",
                "manifest_id": "manifest-1",
                "created_at": "2026-05-31T07:00:00Z",
                "entries": 2,
                "completeness_status": "complete",
                "files_expected": 2,
                "files_verified": 2,
                "verification_errors": 0,
                "verification_warnings": 0
              },
              "counts": {
                "warnings": 0,
                "soft_deletes": 0,
                "target_drifts": 0,
                "live_target_drifts": 0,
                "live_target_drift_artifact_problems": 0,
                "prune_unapplied_approvals": 0,
                "prune_active_approvals": 0,
                "prune_stale_approvals": 0,
                "prune_expired_approvals": 0,
                "prune_consumed_approvals": 0,
                "prune_receipt_issues": 0,
                "recovery_issues": 0,
                "artifact_problems": 0,
                "network_transfers": 0
              },
              "prune_review": {
                "status": "clean",
                "action": "none"
              },
              "pairing": {
                "status": "paired",
                "encrypted_transfer": "available",
                "issue": null
              },
              "privacy": {
                "status": "accepted",
                "mode": "standard",
                "traffic_level": 1,
                "claim": "bounded",
                "local_push": "not applicable",
                "network_transfer": "encrypted",
                "residual_leakage": [],
                "configured_reductions": [],
                "overhead_status": "ok",
                "overhead_source": "profile"
              },
              "traffic_privacy_acceptance": {
                "status": "accepted",
                "claim": "bounded",
                "blockers": []
              },
              "network": {
                "status": "idle",
                "artifact_problems": 0,
                "transfers": []
              },
              "artifact_problem_sources": []
            }
            """.write(
                to: dir.appendingPathComponent("source.status.json"),
                atomically: true,
                encoding: .utf8
            )
            try """
            {
              "target_root": "/tmp/current-target",
              "healthy": true,
              "summary": {
                "incomplete_sessions": 0,
                "invalid_records": 0,
                "artifact_problems": 0,
                "target_drifts": 0,
                "network_transfers": 0
              },
              "items": [],
              "invalid": [],
              "artifacts": [],
              "network_transfers": [
                {
                  "session_id": "session-1",
                  "status": "complete",
                  "action": "publish"
                }
              ]
            }
            """.write(
                to: dir.appendingPathComponent("source.health.json"),
                atomically: true,
                encoding: .utf8
            )
        }
        if includeCurrentStrictEvaluation {
            try """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "evidence_collected",
              "pairing_receipt_id": "pair-1",
              "session_id": "session-1",
              "target_root": "/tmp/current-target",
              "require_operator_evidence": true
            }
            """.write(
                to: dir.appendingPathComponent("evaluation.json"),
                atomically: true,
                encoding: .utf8
            )
        }
        return dir
    }

    private func makeAcceptanceBundleWithMachineIdentityOnly(
        sourceMachineID: String,
        sourceMachineLabel: String,
        targetMachineID: String,
        targetMachineLabel: String
    ) throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(
            "acceptance-machine-identity-\(UUID().uuidString)",
            isDirectory: true
        )
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {
              "profile": "/tmp/source.profile.json",
              "status": "recorded",
              "machine_id": "\(sourceMachineID)",
              "machine_label": "\(sourceMachineLabel)"
            },
            "target": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "\(targetMachineID)",
              "machine_label": "\(targetMachineLabel)"
            }
          },
          "evidence": {
            "machine_facts": {
              "source": {
                "output": "source.machine.json",
                "machine_id": "\(sourceMachineID)",
                "machine_label": "\(sourceMachineLabel)"
              },
              "target": {
                "output": "target.machine.json",
                "machine_id": "\(targetMachineID)",
                "machine_label": "\(targetMachineLabel)"
              }
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "\(sourceMachineID)",
          "machine_label": "\(sourceMachineLabel)"
        }
        """.write(to: dir.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "\(targetMachineID)",
          "machine_label": "\(targetMachineLabel)"
        }
        """.write(to: dir.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        return dir
    }

    private func makeFakePackagedResources(auditReady: Bool, notarizationReady: Bool = false) throws -> URL {
        var provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(
            cliVersion: "supermover 0.1.0-dev"
        )
        provenanceManifest["signing"] = auditReady ? "developer-id" : "ad-hoc"
        if auditReady {
            return try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
                named: "SuperMover",
                cliVersion: "supermover 0.1.0-dev",
                provenanceManifest: provenanceManifest,
                includeNotarizationSidecar: notarizationReady
            ).resourcesURL
        }

        let appRoot = FileManager.default.temporaryDirectory.appendingPathComponent(
            "SuperMover-\(UUID().uuidString).app",
            isDirectory: true
        )
        let resources = appRoot.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resources.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest).write(
            to: resources.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        #!/bin/sh
        printf 'supermover 0.1.0-dev\\n'
        """.write(
            to: binDir.appendingPathComponent("supermover"),
            atomically: true,
            encoding: .utf8
        )
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: binDir.appendingPathComponent("supermover").path
        )
        try """
        #!/bin/sh
        set -eu
        cat <<'EOF'
        \(try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appRoot.path,
            provenanceManifest: provenanceManifest,
            status: "blocked",
            readiness: "blocked",
            passReady: false,
            blockingChecks: 6
        ))
        EOF
        """.write(
            to: binDir.appendingPathComponent("supermover-app-audit"),
            atomically: true,
            encoding: .utf8
        )
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: binDir.appendingPathComponent("supermover-app-audit").path
        )
        return resources
    }

    private func taskInput(profilePath: String = "/tmp/profile.json", sessionID: String = "") -> TaskInput {
        TaskInput(
            profilePath: profilePath,
            sourceRootPath: "/tmp/source",
            targetRootPath: "/tmp/target",
            profileID: "profile-local",
            profileName: "Local profile",
            targetID: "",
            targetName: "",
            sessionID: sessionID,
            sessionPrefix: "",
            queueEntryID: "",
            syncRetryBackoff: "1m",
            syncInterval: "1m",
            syncMaxRuns: "0",
            syncSettle: "250ms",
            syncMaxEvents: "0",
            syncDiscoveryListen: "0.0.0.0:39394",
            syncDiscoveryTimeout: "2s",
            listenAddress: "127.0.0.1:4000",
            pairingTargetAddress: "",
            pairingVerificationCode: "",
            pairingMethod: "sas",
            pairingTimeout: "5s",
            discoveryBrowseListen: "0.0.0.0:39394",
            discoveryBrowseTimeout: "2s",
            discoveryAdvertiseListen: "",
            discoveryAdvertiseDestination: "255.255.255.255:39394",
            discoveryAdvertiseDuration: "10s",
            discoveryAdvertiseInterval: "1s",
            driftIDsInput: "",
            approvalID: "",
            softDeleteIDsInput: "",
            expiresAt: "",
            reason: "",
            reviewer: ""
        )
    }

    private func verifyReviewJSON() -> String {
        """
        {
          "target_root": "/tmp/target",
          "session_id": "session-1",
          "manifest": {
            "id": "manifest-1",
            "session_id": "session-1",
            "root_id": "root-1",
            "created_at": "2026-05-31T07:00:00Z",
            "entries": 2,
            "files": 2
          },
          "summary": {
            "manifest_count": 1,
            "manifest_entries": 2,
            "files_expected": 2,
            "files_verified": 1,
            "warnings": 1,
            "soft_deletes": 1,
            "target_drifts": 1,
            "artifact_problems": 1,
            "error_findings": 1,
            "warning_findings": 1,
            "skipped_digest": 1
          },
          "findings": [
            {
              "kind": "digest_mismatch",
              "severity": "error",
              "session_id": "session-1",
              "path": "file.txt",
              "target_path": "file.txt",
              "message": "digest changed",
              "expected_digest": "sha256:expected",
              "actual_digest": "sha256:actual"
            }
          ],
          "warnings": [
            {
              "version": 1,
              "id": "warning-1",
              "session_id": "session-1",
              "code": "digest_missing",
              "message": "digest missing",
              "severity": "warning",
              "paths": ["file.txt"],
              "created_at": "2026-05-31T07:00:00Z"
            }
          ],
          "soft_deletes": [
            {
              "version": 1,
              "id": "soft-delete-1",
              "session_id": "session-1",
              "root_id": "root-1",
              "source_path": "old.txt",
              "target_path": "old.txt"
            }
          ],
          "target_drifts": [
            {
              "version": 1,
              "id": "drift-1",
              "session_id": "session-1",
              "root_id": "root-1",
              "path": "extra.txt",
              "detected_at": "2026-05-31T07:00:00Z",
              "change": "extra"
            }
          ],
          "artifact_problems": [
            {
              "session_id": "session-1",
              "path": ".supermover/bad.json",
              "error": "invalid json"
            }
          ],
          "manifests": [
            {
              "id": "manifest-1",
              "session_id": "session-1",
              "root_id": "root-1",
              "created_at": "2026-05-31T07:00:00Z",
              "entries": 2,
              "files": 2
            }
          ]
        }
        """
    }

    private func statusReviewJSON() -> String {
        """
        {
          "profile_id": "profile-local",
          "target_id": "local:profile-local",
          "target_root": "/tmp/target",
          "overall": {
            "status": "review",
            "target_status": "target drift"
          },
          "issues": ["target drift requires review"],
          "latest_session": {
            "id": "session-1",
            "manifest_id": "manifest-1",
            "created_at": "2026-05-31T07:00:00Z",
            "entries": 2,
            "completeness_status": "complete",
            "files_expected": 2,
            "files_verified": 2,
            "verification_errors": 0,
            "verification_warnings": 0
          },
          "counts": {
            "warnings": 0,
            "soft_deletes": 0,
            "target_drifts": 1,
            "live_target_drifts": 0,
            "live_target_drift_artifact_problems": 0,
            "prune_unapplied_approvals": 0,
            "prune_active_approvals": 0,
            "prune_stale_approvals": 0,
            "prune_expired_approvals": 0,
            "prune_consumed_approvals": 0,
            "prune_receipt_issues": 0,
            "recovery_issues": 0,
            "artifact_problems": 0,
            "network_transfers": 0
          },
          "prune_review": {
            "status": "clean",
            "action": "none"
          },
          "pairing": {
            "status": "paired",
            "encrypted_transfer": "available",
            "issue": null
          },
          "privacy": {
            "status": "accepted",
            "mode": "standard",
            "traffic_level": 1,
            "claim": "bounded",
            "local_push": "not applicable",
            "network_transfer": "encrypted",
            "residual_leakage": [],
            "configured_reductions": [],
            "overhead_status": "ok",
            "overhead_source": "profile"
          },
          "traffic_privacy_acceptance": {
            "status": "accepted",
            "claim": "bounded",
            "blockers": []
          },
          "network": {
            "status": "idle",
            "artifact_problems": 0,
            "transfers": []
          },
          "artifact_problem_sources": []
        }
        """
    }

    private func reportOKJSON() -> String {
        """
        {
          "profile_id": "profile-local",
          "target_id": "local:profile-local",
          "target_root": "/tmp/target",
          "overall": {
            "status": "clean",
            "issues": []
          },
          "summary": {
            "warnings": 0,
            "soft_deletes": 0,
            "target_drifts": 0,
            "live_target_drifts": 0,
            "prune_candidates": 0,
            "prune_refusals": 0,
            "prune_approvals": 0,
            "network_transfers": 0,
            "artifact_problems": 0
          },
          "latest_session": {
            "id": "session-1",
            "manifest_id": "manifest-1",
            "created_at": "2026-05-31T07:00:00Z",
            "entries": 2,
            "files": 2,
            "completeness": {
              "status": "complete",
              "files_expected": 2,
              "files_verified": 2,
              "verification_errors": 0,
              "verification_warnings": 0
            }
          },
          "prune_review": {
            "status": "clean",
            "approval_required": false,
            "apply": "none",
            "summary": {
              "candidates": 0,
              "refusals": 0,
              "approvals": 0,
              "unapplied_approvals": 0,
              "receipt_issues": 0
            }
          },
          "pairing": {
            "status": "paired",
            "encrypted_transfer": "available",
            "issue": null
          },
          "privacy": {
            "status": "accepted",
            "claim": "bounded",
            "network_transfer": "encrypted"
          },
          "health": {
            "healthy": true,
            "summary": {
              "incomplete_sessions": 0,
              "invalid_records": 0,
              "artifact_problems": 0,
              "target_drifts": 0,
              "network_transfers": 0
            }
          },
          "artifact_problems": [],
          "network_transfers": []
        }
        """
    }

    private func healthOKJSON() -> String {
        """
        {
          "target_root": "/tmp/target",
          "healthy": true,
          "summary": {
            "incomplete_sessions": 0,
            "invalid_records": 0,
            "artifact_problems": 0,
            "target_drifts": 0,
            "network_transfers": 0
          },
          "items": [],
          "invalid": [],
          "artifacts": [],
          "network_transfers": []
        }
        """
    }

    private func driftRecordReviewJSON() -> String {
        """
        {
          "target_root": "/tmp/target",
          "session_id": "session-1",
          "detected": 1,
          "recorded": 1,
          "existing": 0,
          "reopened": 0,
          "manifest_count": 1,
          "records": [
            {
              "id": "detected_file",
              "path": "file.txt",
              "change": "content_mismatch",
              "session_id": "session-1",
              "review_state": "needs_review",
              "recorded": true,
              "existing": false,
              "reopened": false
            }
          ]
        }
        """
    }

    private func minimalSyncRunJSON() -> String {
        """
        {
          "operation": "run",
          "mode": "local_queue_consumer",
          "enqueue": \(minimalQueueJSON(operation: "enqueue")),
          "run": \(minimalRunReceiptJSON(sessionID: "sync-run-1", status: "complete"))
        }
        """
    }

    private func minimalQueueJSON(operation: String) -> String {
        """
        {
          "operation": "\(operation)",
          "mode": "queue_only",
          "state": "present",
          "summary": \(minimalQueueSummaryJSON())
        }
        """
    }

    private func minimalRunReceiptJSON(sessionID: String, status: String) -> String {
        """
        {
          "session_id": "\(sessionID)",
          "status": "\(status)",
          "started_at": "",
          "finished_at": "",
          "state_path": "",
          "run_path": "",
          "summary": \(minimalQueueSummaryJSON())
        }
        """
    }

    private func minimalQueueSummaryJSON() -> String {
        """
        {
          "profile_id": "profile-local",
          "target_id": "local:profile-local",
          "queued": 0,
          "in_flight": 0,
          "backoff": 0,
          "canceled": 0,
          "done": 0,
          "failed": 0,
          "ready": 0,
          "total": 0,
          "warning_count": 0,
          "state_path": "/tmp/queue.json",
          "generated_at": "2026-05-31T07:00:00Z"
        }
        """
    }

    private func pruneReviewCandidateAndRefusalJSON() -> String {
        """
        {
          "schema": "supermover.prune.review.v1",
          "scope": "current",
          "target_root": "/tmp/target",
          "profile_id": "profile-local",
          "target_id": "local:profile-local",
          "session_filter": null,
          "latest_session_id": "session-1",
          "status": "review_required",
          "review_required": true,
          "action": "inspect_prune_review_before_release",
          "read_only": true,
          "authorization": {
            "approval_bypass": false,
            "approval_writing": "disabled",
            "receipt_writing": "disabled",
            "physical_pruning": "disabled",
            "target_deletion": "disabled",
            "apply_requires": "explicit_apply"
          },
          "prune_review": {
            "status": "review_required",
            "dry_run": true,
            "approval_required": true,
            "approval_authoring": "disabled",
            "physical_pruning": "disabled",
            "apply": "disabled",
            "approval_source": "fresh_dry_run",
            "receipt_source": "none",
            "summary": {
              "soft_deletes": 2,
              "candidates": 1,
              "refusals": 1,
              "approvals": 0,
              "unapplied_approvals": 0,
              "active_approvals": 0,
              "stale_approvals": 0,
              "expired_approvals": 0,
              "consumed_approvals": 0,
              "receipts": 0,
              "receipt_issues": 0,
              "artifact_problems": 0
            },
            "candidates": [
              {
                "soft_delete_id": "soft-delete-candidate",
                "detected_session_id": "session-1",
                "root_id": "root-1",
                "source_path": "old.txt",
                "target_path": "old.txt",
                "kind": "file",
                "detected_at": "2026-05-31T07:00:00Z",
                "intended_action": "prune",
                "physical_pruning": "disabled",
                "approval_writing": "disabled",
                "receipt_writing": "disabled",
                "review_required": true
              }
            ],
            "refusals": [
              {
                "soft_delete_id": "soft-delete-refusal",
                "detected_session_id": "session-1",
                "source_path": "blocked.txt",
                "target_path": "blocked.txt",
                "reason_code": "unsafe_path",
                "message": "unsafe path cannot be approved"
              }
            ],
            "approvals": [],
            "receipts": []
          },
          "artifact_problems": []
        }
        """
    }

    private func makeTemporaryProfileURL(named name: String) -> URL {
        FileManager.default.temporaryDirectory
            .appendingPathComponent("AppStoreTests-\(UUID().uuidString)-\(name)", isDirectory: false)
    }

    private func restoreStandardDefault(_ value: Any?, forKey key: String) {
        if let value {
            UserDefaults.standard.set(value, forKey: key)
        } else {
            UserDefaults.standard.removeObject(forKey: key)
        }
    }

    private func writeProvenanceManifest(
        in resourceURL: URL,
        signing: String?,
        gitDirty: Bool,
        cliRelativePath: String? = "Contents/Resources/bin/supermover"
    ) throws {
        let signingLine = signing.map { #""signing": "\#($0)","# } ?? ""
        let cliRelativePathLine = cliRelativePath.map { #""cli_relative_path": "\#($0)","# } ?? ""
        let manifest = """
        {
          "schema": "supermover.macos.provenance.v1",
          "app_bundle_id": "dev.supermover.macapp",
          "app_version": "0.1.0",
          "build_profile": "test",
          "git_commit": "abcdef123456",
          "git_dirty": \(gitDirty ? "true" : "false"),
          "cli_version": "supermover 0.1.0-dev",
          \(cliRelativePathLine)
          "built_at": "2026-05-31T00:00:00Z",
          \(signingLine)
          "test_only": true
        }
        """
        try manifest.write(
            to: resourceURL.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
    }
}
