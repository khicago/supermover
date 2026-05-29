import XCTest
@testable import SuperMoverApp

final class AcceptanceBundleAppOperationsIntegrationTests: XCTestCase {
    @MainActor
    func testAppStoreBlocksMismatchedSourceConsistencyAgainstSameMachineBundleWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let harness = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: harness.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let sourceProfile = workDir.appendingPathComponent("source.profile.json")
        let originalTargetAddress = try loadJSONString(
            bundleRoot.appendingPathComponent("source.transfer.json"),
            key: "target_address"
        )
        let originalReceiverAddress = try loadJSONString(
            bundleRoot.appendingPathComponent("source.transfer.json"),
            key: "receiver_address"
        )
        let originalVerifyJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.verify.json"))
        let originalStatusJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.status.json"))
        let originalReportJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.report.json"))
        let originalHealthJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.health.json"))
        let originalConsistencyJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.consistency.json"))
        let originalPushStdout = try String(contentsOf: bundleRoot.appendingPathComponent("source.network-push.txt"))
        try clearHarnessSourceTransfer(bundleRootURL: bundleRoot)

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { appURL.appendingPathComponent("Contents/Resources", isDirectory: true) }
        )
        store.acceptanceBundlePath = bundleRoot.path
        store.profilePath = sourceProfile.path
        store.selectedRole = .source
        store.sessionID = "same-machine-five-phase-002"
        store.pairingTargetAddress = originalTargetAddress
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: store.pairingTargetAddress,
            verification_code: nil,
            mode: "pairing",
            receiver_address: originalReceiverAddress,
            receiver_routes: true,
            push_network: true,
            trusted: true,
            transfer: true,
            expires_at: nil
        )
        store.evidenceEnvelopes[.verify] = try makeStructuredEnvelope(
            artifactKind: .verify,
            task: .verify,
            contextSignature: store.currentContextSignature(for: .verify),
            rawStdout: originalVerifyJSON
        )
        store.evidenceEnvelopes[.status] = try makeStructuredEnvelope(
            artifactKind: .status,
            task: .status,
            contextSignature: store.currentContextSignature(for: .status),
            rawStdout: originalStatusJSON
        )
        store.evidenceEnvelopes[.report] = try makeStructuredEnvelope(
            artifactKind: .report,
            task: .report,
            contextSignature: store.currentContextSignature(for: .report),
            rawStdout: originalReportJSON
        )
        store.evidenceEnvelopes[.health] = try makeStructuredEnvelope(
            artifactKind: .health,
            task: .health,
            contextSignature: store.currentContextSignature(for: .health),
            rawStdout: originalHealthJSON
        )
        store.evidenceEnvelopes[.sourceConsistency] = try makeStructuredEnvelope(
            artifactKind: .sourceConsistency,
            task: .networkPush,
            contextSignature: store.currentContextSignature(for: .networkPush),
            rawStdout: originalConsistencyJSON
        )
        store.recentRuns.insert(
            TaskRun(
                kind: .networkPush,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: store.currentContextSignature(for: .networkPush),
                processIdentifier: nil,
                stdout: originalPushStdout,
                stderr: "",
                state: .finished(0)
            ),
            at: 0
        )

        store.recordAcceptanceSourceTransferArtifact()

        XCTAssertTrue(store.note.contains("Recorded source.transfer.json into acceptance bundle."))
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceTransferArtifact?.session_id, "same-machine-five-phase-002")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_consistency?.status, "blocked")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_consistency?.mode, "session_mismatch")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceConsistencyArtifact?.status, "blocked")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceConsistencyArtifact?.mode, "session_mismatch")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceConsistencyArtifact?.session_id, "same-machine-five-phase-002")
        XCTAssertEqual(
            store.acceptanceBundleSnapshot?.sourceConsistencyArtifact?.detail,
            "Current-source proof session_id does not match the transfer session being written into this acceptance bundle."
        )
    }

    @MainActor
    func testAppStoreClearsStaleBaselineWhenSourceConsistencyPassReplayLacksBaselineSidecar() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let harness = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: harness.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let sourceProfile = workDir.appendingPathComponent("source.profile.json")
        let originalSessionID = try loadJSONString(
            bundleRoot.appendingPathComponent("source.transfer.json"),
            key: "session_id"
        )
        let originalTargetAddress = try loadJSONString(
            bundleRoot.appendingPathComponent("source.transfer.json"),
            key: "target_address"
        )
        let originalReceiverAddress = try loadJSONString(
            bundleRoot.appendingPathComponent("source.transfer.json"),
            key: "receiver_address"
        )
        let originalConsistencyJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.consistency.json"))
        let originalPushStdout = try String(contentsOf: bundleRoot.appendingPathComponent("source.network-push.txt"))
        try clearHarnessSourceTransfer(bundleRootURL: bundleRoot)

        let staleBaselineURL = bundleRoot.appendingPathComponent("source.baseline.json")
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-src",
          "root_id": "root-src",
          "root_path": "/tmp/source",
          "session_id": "stale-session",
          "created_at": "2026-06-01T10:00:00Z",
          "entries": []
        }
        """.write(to: staleBaselineURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { appURL.appendingPathComponent("Contents/Resources", isDirectory: true) }
        )
        store.acceptanceBundlePath = bundleRoot.path
        store.profilePath = sourceProfile.path
        store.selectedRole = .source
        store.sessionID = originalSessionID
        store.pairingTargetAddress = originalTargetAddress
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: store.pairingTargetAddress,
            verification_code: nil,
            mode: "pairing",
            receiver_address: originalReceiverAddress,
            receiver_routes: true,
            push_network: true,
            trusted: true,
            transfer: true,
            expires_at: nil
        )
        store.evidenceEnvelopes[.sourceConsistency] = try makeStructuredEnvelope(
            artifactKind: .sourceConsistency,
            task: .networkPush,
            contextSignature: store.currentContextSignature(for: .networkPush),
            rawStdout: originalConsistencyJSON
        )
        store.recentRuns.insert(
            TaskRun(
                kind: .networkPush,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: store.currentContextSignature(for: .networkPush),
                processIdentifier: nil,
                stdout: originalPushStdout,
                stderr: "",
                state: .finished(0)
            ),
            at: 0
        )

        store.recordAcceptanceSourceTransferArtifact()

        XCTAssertTrue(store.note.contains("Recorded source.transfer.json into acceptance bundle."))
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_consistency?.status, "blocked")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_consistency?.mode, "baseline_missing")
        XCTAssertNil(store.acceptanceBundleSnapshot?.meta.evidence.source_consistency?.baseline)
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceConsistencyArtifact?.status, "blocked")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceConsistencyArtifact?.mode, "baseline_missing")
        XCTAssertFalse(FileManager.default.fileExists(atPath: staleBaselineURL.path))
    }

    @MainActor
    func testAppStoreClearsStaleNotarizationEvidenceWhenBuiltAppHasNoSidecar() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }
        let sidecarURL = appURL.deletingLastPathComponent().appendingPathComponent("\(appURL.lastPathComponent).notary/notarization.json")
        guard !FileManager.default.fileExists(atPath: sidecarURL.path) else {
            throw XCTSkip("built app already has real notarization sidecar at \(sidecarURL.path); stale cleanup case is not applicable")
        }

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let harness = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: harness.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let sourceProfile = workDir.appendingPathComponent("source.profile.json")
        let staleNotarizationURL = bundleRoot.appendingPathComponent("source.notarization.json")
        try """
        {
          "schema": "supermover.macos.notarization.v1",
          "status": "pass",
          "submission": {
            "status": "Accepted"
          },
          "audit": {
            "status": "pass",
            "readiness": "distribution_ready",
            "pass_ready": true
          }
        }
        """.write(to: staleNotarizationURL, atomically: true, encoding: .utf8)

        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        guard var document = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw IntegrationError.malformedMeta(metaURL.path)
        }
        var evidence = document["evidence"] as? [String: Any] ?? [:]
        evidence["notarization"] = [
            "source": [
                "collected_by": "stale",
                "output": "source.notarization.json",
                "status": "pass",
                "audit_status": "pass",
                "audit_readiness": "distribution_ready",
                "audit_pass_ready": true,
            ]
        ]
        document["evidence"] = evidence
        let updated = try JSONSerialization.data(withJSONObject: document, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: metaURL, options: .atomic)

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { appURL.appendingPathComponent("Contents/Resources", isDirectory: true) }
        )
        store.acceptanceBundlePath = bundleRoot.path
        store.profilePath = sourceProfile.path
        store.selectedRole = .source
        store.refreshAcceptanceBundle()

        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarization?.output, "source.notarization.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: staleNotarizationURL.path))

        store.recordAcceptancePackagingEvidence()

        XCTAssertTrue(store.note.contains("Recorded packaging evidence into acceptance bundle."))
        XCTAssertFalse(FileManager.default.fileExists(atPath: staleNotarizationURL.path))
        XCTAssertNil(store.acceptanceBundleSnapshot?.sourceNotarization)
        XCTAssertNil(store.acceptanceBundleSnapshot?.sourceNotarizationArtifact)
    }

    @MainActor
    func testAppStoreBuiltAppLaunchPreflightHonorsCopiedSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let bundleRoot = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready"
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        let scratch = try makeDirectory(named: "copied-built-app-preflight")
        defer { try? FileManager.default.removeItem(at: scratch) }
        let copiedAppURL = scratch.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: copiedAppURL)
        let copiedResourcesURL = copiedAppURL.appendingPathComponent("Contents/Resources", isDirectory: true)

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { copiedResourcesURL },
            packagingCollectorFactory: {
                self.makePassingPackagingCollector(appBundleURL: copiedAppURL)
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: copiedResourcesURL.appendingPathComponent("bin/supermover").path,
            workingDirectoryPath: copiedResourcesURL.appendingPathComponent("bin").path,
            bundleIdentifier: "dev.supermover.macapp",
            appVersion: "0.1.0",
            provenancePath: copiedResourcesURL.appendingPathComponent("supermover-provenance.json").path,
            provenanceStatus: "loaded",
            bundleCommit: "9662f01cb0db",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "local-release",
            signing: "-",
            gitDirty: true,
            builtAt: "2026-06-01T13:06:40Z",
            readiness: "local bundle review",
            detail: "copied built app"
        )
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .pair
        store.profilePath = "/tmp/profile.json"
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"

        let blockedError = store.acceptanceInstalledAppLaunchPreflightError(for: .pair)
        XCTAssertNotNil(blockedError)
        XCTAssertTrue(blockedError?.contains("no source.notarization.json") == true)

        let copiedSidecarDir = scratch.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        try writeReadyNotarizationSidecar(appBundleURL: copiedAppURL, notaryDirectoryURL: copiedSidecarDir)

        let allowedError = store.acceptanceInstalledAppLaunchPreflightError(for: .pair)
        XCTAssertNil(allowedError)
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarization?.output, "source.notarization.json")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarizationArtifact?.status, "pass")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarizationArtifact?.submission?.status, "Accepted")
    }

    @MainActor
    func testAppStoreBuiltAppLaunchPreflightHonorsCanonicalWorkflowSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let sidecarURL = builtAppURL.deletingLastPathComponent().appendingPathComponent("\(builtAppURL.lastPathComponent).notary/notarization.json")
        guard !FileManager.default.fileExists(atPath: sidecarURL.path) else {
            throw XCTSkip("canonical built app already has sidecar at \(sidecarURL.path); workflow-sidecar injection case is not applicable")
        }

        let harness = try makeNotaryHarness(
            auditStatus: "pass",
            auditReadiness: "distribution_ready",
            auditPassReady: true,
            auditBlockingChecks: 0
        )
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer {
            try? FileManager.default.removeItem(at: sidecarURL)
            try? FileManager.default.removeItem(at: sidecarURL.deletingLastPathComponent())
        }

        let notarize = try runNotarizeScript(
            arguments: [
                "--app", builtAppURL.path,
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
        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarURL.path))

        let bundleRoot = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready"
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        let resourcesURL = builtAppURL.appendingPathComponent("Contents/Resources", isDirectory: true)
        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resourcesURL },
            packagingCollectorFactory: {
                self.makePassingPackagingCollector(appBundleURL: builtAppURL)
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: resourcesURL.appendingPathComponent("bin/supermover").path,
            workingDirectoryPath: resourcesURL.appendingPathComponent("bin").path,
            bundleIdentifier: "dev.supermover.macapp",
            appVersion: "0.1.0",
            provenancePath: resourcesURL.appendingPathComponent("supermover-provenance.json").path,
            provenanceStatus: "loaded",
            bundleCommit: "9662f01cb0db",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "local-release",
            signing: "-",
            gitDirty: true,
            builtAt: "2026-06-01T13:06:40Z",
            readiness: "local bundle review",
            detail: "canonical built app workflow sidecar"
        )
        store.acceptanceBundlePath = bundleRoot.path
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
    func testAppStoreBuiltAppLaunchPreflightRejectsMalformedCanonicalWorkflowSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let sidecarURL = builtAppURL.deletingLastPathComponent().appendingPathComponent("\(builtAppURL.lastPathComponent).notary/notarization.json")
        guard !FileManager.default.fileExists(atPath: sidecarURL.path) else {
            throw XCTSkip("canonical built app already has sidecar at \(sidecarURL.path); workflow-sidecar injection case is not applicable")
        }

        let harness = try makeNotaryHarness(
            auditStatus: "pass",
            auditReadiness: "distribution_ready",
            auditPassReady: true,
            auditBlockingChecks: 0
        )
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer {
            try? FileManager.default.removeItem(at: sidecarURL)
            try? FileManager.default.removeItem(at: sidecarURL.deletingLastPathComponent())
        }

        let notarize = try runNotarizeScript(
            arguments: [
                "--app", builtAppURL.path,
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
        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarURL.path))

        try #"{"schema":"bad"}"#.write(to: sidecarURL, atomically: true, encoding: .utf8)

        let bundleRoot = try makeAcceptanceBundle(
            collectionMode: "two_machine",
            sourceAudit: "distribution_ready",
            targetAudit: "distribution_ready"
        )
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        let staleNotarizationURL = bundleRoot.appendingPathComponent("source.notarization.json")
        try #"{"schema":"stale"}"#.write(to: staleNotarizationURL, atomically: true, encoding: .utf8)
        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let metaData = try Data(contentsOf: metaURL)
        guard var metaDocument = try JSONSerialization.jsonObject(with: metaData) as? [String: Any] else {
            throw IntegrationError.malformedMeta(metaURL.path)
        }
        var evidence = metaDocument["evidence"] as? [String: Any] ?? [:]
        evidence["notarization"] = [
            "source": [
                "collected_by": "stale",
                "output": "source.notarization.json",
                "status": "pass",
                "audit_status": "pass",
                "audit_readiness": "distribution_ready",
                "audit_pass_ready": true,
            ]
        ]
        metaDocument["evidence"] = evidence
        let updatedMeta = try JSONSerialization.data(withJSONObject: metaDocument, options: [.prettyPrinted, .sortedKeys])
        try updatedMeta.write(to: metaURL, options: .atomic)

        let resourcesURL = builtAppURL.appendingPathComponent("Contents/Resources", isDirectory: true)
        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resourcesURL },
            packagingCollectorFactory: {
                self.makePassingPackagingCollector(appBundleURL: builtAppURL)
            }
        )
        store.cliProvenance = CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: resourcesURL.appendingPathComponent("bin/supermover").path,
            workingDirectoryPath: resourcesURL.appendingPathComponent("bin").path,
            bundleIdentifier: "dev.supermover.macapp",
            appVersion: "0.1.0",
            provenancePath: resourcesURL.appendingPathComponent("supermover-provenance.json").path,
            provenanceStatus: "loaded",
            bundleCommit: "9662f01cb0db",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "local-release",
            signing: "-",
            gitDirty: true,
            builtAt: "2026-06-01T13:06:40Z",
            readiness: "local bundle review",
            detail: "canonical built app malformed workflow sidecar"
        )
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .pair
        store.profilePath = "/tmp/profile.json"
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"

        let error = store.acceptanceInstalledAppLaunchPreflightError(for: .pair)

        XCTAssertNotNil(error)
        XCTAssertTrue(error?.contains("Local notarization evidence is malformed") == true)
        XCTAssertFalse(FileManager.default.fileExists(atPath: staleNotarizationURL.path))
        XCTAssertNil(store.acceptanceBundleSnapshot?.sourceNotarization)
        XCTAssertNil(store.acceptanceBundleSnapshot?.sourceNotarizationArtifact)
    }

    @MainActor
    func testAppStoreRecordsPackagingAndEvaluationAgainstSameMachineBundleWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_APPSTORE_ACCEPTANCE_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let harness = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: harness.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let sourceProfile = workDir.appendingPathComponent("source.profile.json")
        let targetProfile = workDir.appendingPathComponent("target.profile.json")
        let expectedReceiptID = try ProfilePairingReader().read(profileURL: targetProfile).target.pairing_receipt_id
        let originalTargetAddress = try loadJSONString(
            bundleRoot.appendingPathComponent("source.transfer.json"),
            key: "target_address"
        )
        let originalReceiverAddress = try loadJSONString(
            bundleRoot.appendingPathComponent("source.transfer.json"),
            key: "receiver_address"
        )
        let originalVerifyJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.verify.json"))
        let originalStatusJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.status.json"))
        let originalReportJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.report.json"))
        let originalHealthJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.health.json"))
        let originalConsistencyJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.consistency.json"))
        let originalBaselineJSON = try String(contentsOf: bundleRoot.appendingPathComponent("source.baseline.json"))
        let originalPushStdout = try String(contentsOf: bundleRoot.appendingPathComponent("source.network-push.txt"))
        try clearHarnessEvaluation(bundleRootURL: bundleRoot)
        try clearHarnessTargetImport(bundleRootURL: bundleRoot)
        try removeSourcePackagingEvidence(bundleRootURL: bundleRoot)
        try clearHarnessSourceTransfer(bundleRootURL: bundleRoot)

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { appURL.appendingPathComponent("Contents/Resources", isDirectory: true) }
        )
        store.acceptanceBundlePath = bundleRoot.path
        store.profilePath = targetProfile.path
        store.targetRootPath = targetRoot.path
        store.selectedRole = .source
        store.refreshAcceptanceBundle()

        store.recordAcceptancePackagingEvidence()
        XCTAssertTrue(store.note.contains("Recorded packaging evidence into acceptance bundle."))
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceAppAudit?.output, "source.app-audit.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("source.version.txt").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("source.provenance.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("source.app-audit.json").path))

        store.profilePath = sourceProfile.path
        store.sessionID = "same-machine-five-phase-001"
        store.pairingTargetAddress = originalTargetAddress
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: store.pairingTargetAddress,
            verification_code: nil,
            mode: "pairing",
            receiver_address: originalReceiverAddress,
            receiver_routes: true,
            push_network: true,
            trusted: true,
            transfer: true,
            expires_at: nil
        )
        store.evidenceEnvelopes[.verify] = try makeStructuredEnvelope(
            artifactKind: .verify,
            task: .verify,
            contextSignature: store.currentContextSignature(for: .verify),
            rawStdout: originalVerifyJSON
        )
        store.evidenceEnvelopes[.status] = try makeStructuredEnvelope(
            artifactKind: .status,
            task: .status,
            contextSignature: store.currentContextSignature(for: .status),
            rawStdout: originalStatusJSON
        )
        store.evidenceEnvelopes[.report] = try makeStructuredEnvelope(
            artifactKind: .report,
            task: .report,
            contextSignature: store.currentContextSignature(for: .report),
            rawStdout: originalReportJSON
        )
        store.evidenceEnvelopes[.health] = try makeStructuredEnvelope(
            artifactKind: .health,
            task: .health,
            contextSignature: store.currentContextSignature(for: .health),
            rawStdout: originalHealthJSON
        )
        store.evidenceEnvelopes[.sourceConsistency] = try makeStructuredEnvelope(
            artifactKind: .sourceConsistency,
            task: .networkPush,
            contextSignature: store.currentContextSignature(for: .networkPush),
            rawStdout: originalConsistencyJSON
        )
        let baselineCaptureURL = workDir.appendingPathComponent("source.baseline.capture.json")
        try originalBaselineJSON.write(to: baselineCaptureURL, atomically: true, encoding: .utf8)
        store.installNetworkPushBaselineFileForTesting(
            baselineCaptureURL.path,
            contextSignature: store.currentContextSignature(for: .networkPush)
        )
        store.recentRuns.insert(
            TaskRun(
                kind: .networkPush,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: store.currentContextSignature(for: .networkPush),
                processIdentifier: nil,
                stdout: originalPushStdout,
                stderr: "",
                state: .finished(0)
            ),
            at: 0
        )
        store.recordAcceptanceSourceTransferArtifact()
        XCTAssertTrue(store.note.contains("Recorded source.transfer.json into acceptance bundle."))
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceTransferArtifact?.session_id, "same-machine-five-phase-001")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_transfer?.verify, "source.verify.json")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_transfer?.status, "source.status.json")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_transfer?.report, "source.report.json")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_transfer?.health, "source.health.json")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_transfer?.push, "source.network-push.txt")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_consistency?.output, "source.consistency.json")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_consistency?.baseline, "source.baseline.json")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_consistency?.status, "pass")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.source_consistency?.mode, "current_source_verified")
        XCTAssertEqual(
            try String(contentsOf: bundleRoot.appendingPathComponent("source.network-push.txt")).trimmingCharacters(in: .whitespacesAndNewlines),
            originalPushStdout.trimmingCharacters(in: .whitespacesAndNewlines)
        )
        XCTAssertEqual(
            try String(contentsOf: bundleRoot.appendingPathComponent("source.baseline.json")).trimmingCharacters(in: .whitespacesAndNewlines),
            originalBaselineJSON.trimmingCharacters(in: .whitespacesAndNewlines)
        )
        XCTAssertEqual(
            try String(contentsOf: bundleRoot.appendingPathComponent("source.consistency.json")).trimmingCharacters(in: .whitespacesAndNewlines),
            originalConsistencyJSON.trimmingCharacters(in: .whitespacesAndNewlines)
        )

        store.selectedRole = .target
        store.profilePath = targetProfile.path
        store.recentRuns.insert(
            TaskRun(
                kind: .profileAdoptPairing,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: store.currentContextSignature(for: .profileAdoptPairing),
                processIdentifier: nil,
                stdout: "app-first adopt transcript",
                stderr: "",
                state: .finished(0)
            ),
            at: 0
        )
        store.recordAcceptanceTargetImportArtifact()
        XCTAssertTrue(store.note.contains("Recorded target_import into acceptance bundle."))
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.target_import?.pairing_receipt_id, expectedReceiptID)
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.target_import?.adopted, "target.adopt-pairing.txt")
        XCTAssertEqual(
            try String(contentsOf: bundleRoot.appendingPathComponent("target.adopt-pairing.txt")),
            "app-first adopt transcript"
        )

        store.recordAcceptanceEvaluationArtifact()
        XCTAssertTrue(store.note.contains("Recorded evaluation.json into acceptance bundle."))
        XCTAssertEqual(store.acceptanceBundleSnapshot?.evaluationArtifact?.status, "evidence_collected")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.evaluationArtifact?.target_root, targetRoot.path)
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.evidence.evaluation?.output, "evaluation.json")
    }

    private func removeSourcePackagingEvidence(bundleRootURL: URL) throws {
        let filenames = [
            "source.version.txt",
            "source.provenance.json",
            "source.app-audit.json",
        ]
        for filename in filenames {
            let url = bundleRootURL.appendingPathComponent(filename)
            if FileManager.default.fileExists(atPath: url.path) {
                try FileManager.default.removeItem(at: url)
            }
        }

        let metaURL = bundleRootURL.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        guard var document = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw IntegrationError.malformedMeta(metaURL.path)
        }
        var evidence = document["evidence"] as? [String: Any] ?? [:]
        if var appAudit = evidence["app_audit"] as? [String: Any] {
            appAudit.removeValue(forKey: "source")
            evidence["app_audit"] = appAudit
        }
        document["evidence"] = evidence
        let updated = try JSONSerialization.data(withJSONObject: document, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: metaURL, options: .atomic)
    }

    private func clearHarnessEvaluation(bundleRootURL: URL) throws {
        let metaURL = bundleRootURL.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        guard var document = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw IntegrationError.malformedMeta(metaURL.path)
        }
        document["status"] = "in_progress"
        var evidence = document["evidence"] as? [String: Any] ?? [:]
        evidence.removeValue(forKey: "evaluation")
        document["evidence"] = evidence
        let updated = try JSONSerialization.data(withJSONObject: document, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: metaURL, options: .atomic)

        let evaluationURL = bundleRootURL.appendingPathComponent("evaluation.json")
        if FileManager.default.fileExists(atPath: evaluationURL.path) {
            try FileManager.default.removeItem(at: evaluationURL)
        }
    }

    private func clearHarnessTargetImport(bundleRootURL: URL) throws {
        let metaURL = bundleRootURL.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        guard var document = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw IntegrationError.malformedMeta(metaURL.path)
        }
        var evidence = document["evidence"] as? [String: Any] ?? [:]
        evidence.removeValue(forKey: "target_import")
        document["evidence"] = evidence
        let updated = try JSONSerialization.data(withJSONObject: document, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: metaURL, options: .atomic)

        let adoptedURL = bundleRootURL.appendingPathComponent("target.adopt-pairing.txt")
        if FileManager.default.fileExists(atPath: adoptedURL.path) {
            try FileManager.default.removeItem(at: adoptedURL)
        }
    }

    private func clearHarnessSourceTransfer(bundleRootURL: URL) throws {
        let filenames = [
            "source.transfer.json",
            "source.verify.json",
            "source.status.json",
            "source.report.json",
            "source.health.json",
            "source.consistency.json",
            "source.baseline.json",
            "source.network-push.txt",
        ]
        for filename in filenames {
            let url = bundleRootURL.appendingPathComponent(filename)
            if FileManager.default.fileExists(atPath: url.path) {
                try FileManager.default.removeItem(at: url)
            }
        }

        let metaURL = bundleRootURL.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        guard var document = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw IntegrationError.malformedMeta(metaURL.path)
        }
        var evidence = document["evidence"] as? [String: Any] ?? [:]
        evidence.removeValue(forKey: "source_transfer")
        evidence.removeValue(forKey: "source_consistency")
        document["evidence"] = evidence
        let updated = try JSONSerialization.data(withJSONObject: document, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: metaURL, options: .atomic)
    }

    private func loadJSONString(_ fileURL: URL, key: String) throws -> String {
        let data = try Data(contentsOf: fileURL)
        guard let object = try JSONSerialization.jsonObject(with: data) as? [String: Any],
              let value = object[key] as? String,
              !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw IntegrationError.malformedMeta(fileURL.path)
        }
        return value
    }

    private func makeStructuredEnvelope(
        artifactKind: StructuredArtifactKind,
        task: SuperMoverTaskKind,
        contextSignature: String,
        rawStdout: String
    ) throws -> StructuredEvidenceEnvelope {
        StructuredEvidenceEnvelope(
            artifactKind: artifactKind,
            task: task,
            loadedAt: Date(),
            contextSignature: contextSignature,
            exitCode: 0,
            rawStdout: rawStdout,
            stderrSample: "",
            freshness: .current
        )
    }

    private func makePassingPackagingCollector(appBundleURL: URL) -> AcceptancePackagingEvidenceCollector {
        AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
                    appBundleURL: appBundleURL,
                    outputURL: outputURL
                )
            }
        )
    }

    private func writeReadyNotarizationSidecar(
        appBundleURL: URL,
        notaryDirectoryURL: URL
    ) throws {
        try FileManager.default.createDirectory(
            at: notaryDirectoryURL,
            withIntermediateDirectories: true
        )
        let auditURL = notaryDirectoryURL.appendingPathComponent("post-staple.audit.json")
        _ = try AcceptanceReleaseEvidenceFixtures.writeAppAuditResult(
            appBundleURL: appBundleURL,
            outputURL: auditURL
        )
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appBundleURL.path,
            auditPath: auditURL.path
        ).write(
            to: notaryDirectoryURL.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )
    }

    private func makeDirectory(named name: String) throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("\(name)-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }

    private func makeAcceptanceBundle(
        collectionMode: String,
        sourceAudit: String,
        targetAudit: String
    ) throws -> URL {
        let dir = try makeDirectory(named: "acceptance-bundle")
        let sourceAppPath = "/tmp/current-source/SuperMover.app"
        let targetAppPath = "/tmp/current-target/SuperMover.app"
        let sourceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(
            cliVersion: "supermover source-test-build"
        )
        let targetManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(
            cliVersion: "supermover target-test-build"
        )
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "\(collectionMode)",
            "machine_count": \(collectionMode == "two_machine" ? "2" : "1")
          },
          "roles": {},
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
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.jsonString(sourceManifest).write(
            to: dir.appendingPathComponent("source.provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.jsonString(targetManifest).write(
            to: dir.appendingPathComponent("target.provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: sourceAppPath,
            provenanceManifest: sourceManifest,
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
            provenanceManifest: targetManifest,
            status: targetAudit == "blocked" ? "blocked" : "pass",
            readiness: targetAudit == "blocked" ? "blocked" : targetAudit,
            passReady: targetAudit != "blocked",
            blockingChecks: targetAudit == "blocked" ? 6 : 0
        ).write(
            to: dir.appendingPathComponent("target.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        return dir
    }


    private func extractWorkDirectory(from stdout: String) throws -> URL {
        let prefix = "same-machine two-machine acceptance passed: "
        guard let line = stdout
            .split(whereSeparator: \.isNewline)
            .reversed()
            .first(where: { $0.hasPrefix(prefix) }) else {
            throw IntegrationError.missingWorkDirectory(stdout)
        }
        let path = String(line.dropFirst(prefix.count)).trimmingCharacters(in: .whitespacesAndNewlines)
        guard !path.isEmpty else {
            throw IntegrationError.missingWorkDirectory(stdout)
        }
        return URL(fileURLWithPath: path, isDirectory: true)
    }

    private func runProcess(
        executableURL: URL,
        arguments: [String],
        environment: [String: String],
        currentDirectoryURL: URL
    ) throws -> ProcessResult {
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: executableURL,
            arguments: arguments,
            environment: environment,
            currentDirectoryURL: currentDirectoryURL
        )
        guard result.exitCode == 0 else {
            throw IntegrationError.commandFailed(
                command: ([executableURL.path] + arguments).joined(separator: " "),
                stdout: result.stdout,
                stderr: result.stderr,
                exitCode: result.exitCode
            )
        }
        return ProcessResult(stdout: result.stdout, stderr: result.stderr)
    }

    private func repoRootURL(file: StaticString = #filePath) -> URL {
        URL(fileURLWithPath: "\(file)")
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
    }
}

private struct ProcessResult {
    let stdout: String
    let stderr: String
}

private enum IntegrationError: LocalizedError {
    case missingWorkDirectory(String)
    case malformedMeta(String)
    case commandFailed(command: String, stdout: String, stderr: String, exitCode: Int32)

    var errorDescription: String? {
        switch self {
        case let .missingWorkDirectory(stdout):
            return "same-machine harness output did not include the work directory path.\nstdout:\n\(stdout)"
        case let .malformedMeta(path):
            return "acceptance bundle meta.json is malformed: \(path)"
        case let .commandFailed(command, stdout, stderr, exitCode):
            return "command failed (\(exitCode)): \(command)\nstdout:\n\(stdout)\nstderr:\n\(stderr)"
        }
    }
}
