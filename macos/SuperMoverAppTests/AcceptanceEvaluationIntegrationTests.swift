import Darwin
import XCTest
@testable import SuperMoverApp

final class AcceptanceEvaluationIntegrationTests: XCTestCase {
    func testEvaluationCoordinatorRewritesSameMachineHarnessBundleWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: result.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        try clearHarnessEvaluation(bundleRootURL: bundleRoot)
        XCTAssertFalse(
            FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("evaluation.json").path)
        )

        try clearHarnessTargetImport(bundleRootURL: bundleRoot)
        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: false
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingTargetImportEvidence
            )
        }

        try restoreHarnessTargetImport(bundleRootURL: bundleRoot, targetRootURL: targetRoot)

        let authoring = try AcceptanceBundleEvaluationCoordinator().evaluate(
            bundleRootURL: bundleRoot,
            targetRootURL: targetRoot,
            requireOperatorEvidence: false
        )

        XCTAssertEqual(authoring.kind, .evaluation)
        XCTAssertEqual(authoring.detail, "evaluation.json -> \(bundleRoot.path)")

        let loaded = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertTrue(loaded.isCollected)
        XCTAssertEqual(loaded.evaluationArtifact?.status, "evidence_collected")
        XCTAssertEqual(loaded.evaluationArtifact?.target_root, targetRoot.path)
        XCTAssertEqual(loaded.meta.evidence.evaluation?.output, "evaluation.json")
        XCTAssertEqual(loaded.meta.evidence.evaluation?.session_id, loaded.sourceTransferArtifact?.session_id)
        XCTAssertEqual(loaded.meta.evidence.evaluation?.pairing_receipt_id, loaded.sourcePairArtifact?.pairing_receipt_id)
        XCTAssertEqual(loaded.meta.evidence.source_consistency?.status, "pass")
        XCTAssertEqual(loaded.meta.evidence.source_consistency?.mode, "current_source_verified")
        XCTAssertEqual(loaded.meta.evidence.source_consistency?.baseline, "source.baseline.json")
        XCTAssertEqual(loaded.sourceConsistencyArtifact?.status, "pass")
        XCTAssertEqual(loaded.sourceConsistencyArtifact?.mode, "current_source_verified")
        XCTAssertEqual(loaded.sourceConsistencyArtifact?.baseline, "source.baseline.json")
    }

    func testEvaluationCoordinatorRejectsSameMachineHarnessWhenOperatorEvidenceRequired() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: result.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)

        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        guard var document = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw IntegrationError.malformedMeta(metaURL.path)
        }
        var evidence = document["evidence"] as? [String: Any] ?? [:]
        evidence["operator"] = [
            "local_network": ["status": "pass", "detail": "accepted prompt"],
            "firewall": ["status": "pass", "detail": "allowed inbound"],
            "pairing_confirmation": ["status": "pass", "detail": "confirmed code"],
        ]
        document["evidence"] = evidence
        let updated = try JSONSerialization.data(withJSONObject: document, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: metaURL, options: .atomic)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .blockedAppAudit("source")
            )
        }
    }

    func testEvaluationCoordinatorRejectsBlockedAppAuditEvenIfSameMachineBundlePretendsToBeTwoMachine() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: result.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)

        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        guard var document = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw IntegrationError.malformedMeta(metaURL.path)
        }
        document["collection"] = [
            "mode": "two_machine",
            "machine_count": 2,
        ]
        document["roles"] = [
            "source_pair": [
                "profile": workDir.appendingPathComponent("source.profile.json").path,
                "status": "recorded",
                "machine_id": "source-machine",
                "machine_label": "source",
            ],
            "target": [
                "profile": workDir.appendingPathComponent("target.profile.json").path,
                "status": "recorded",
                "machine_id": "target-machine",
                "machine_label": "target",
            ],
        ]
        var evidence = document["evidence"] as? [String: Any] ?? [:]
        evidence["operator"] = [
            "local_network": ["status": "pass", "detail": "accepted prompt"],
            "firewall": ["status": "pass", "detail": "allowed inbound"],
            "pairing_confirmation": ["status": "pass", "detail": "confirmed code"],
        ]
        document["evidence"] = evidence
        let updated = try JSONSerialization.data(withJSONObject: document, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: metaURL, options: .atomic)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .blockedAppAudit("source")
            )
        }
    }

    func testSameMachineHarnessUsesConfiguredAuditScriptAcrossAllPhasesWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let fakeAuditDir = try makeDirectory(named: "same-machine-fake-audit")
        defer { try? FileManager.default.removeItem(at: fakeAuditDir) }
        let fakeAuditURL = fakeAuditDir.appendingPathComponent("fake-audit.sh")
        try """
        #!/bin/sh
        cat <<'EOF'
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "pass",
          "readiness": "distribution_ready",
          "summary": {
            "pass_ready": true,
            "blocking_checks": 0
          }
        }
        EOF
        """.write(to: fakeAuditURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditURL.path)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: result.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let sourceAuditData = try Data(contentsOf: bundleRoot.appendingPathComponent("source.app-audit.json"))
        let targetAuditData = try Data(contentsOf: bundleRoot.appendingPathComponent("target.app-audit.json"))
        let sourceAudit = try XCTUnwrap(JSONSerialization.jsonObject(with: sourceAuditData) as? [String: Any])
        let targetAudit = try XCTUnwrap(JSONSerialization.jsonObject(with: targetAuditData) as? [String: Any])

        XCTAssertEqual(sourceAudit["status"] as? String, "pass")
        XCTAssertEqual(sourceAudit["readiness"] as? String, "distribution_ready")
        XCTAssertEqual(targetAudit["status"] as? String, "pass")
        XCTAssertEqual(targetAudit["readiness"] as? String, "distribution_ready")
    }

    func testSameMachineHarnessHonorsConfiguredCopiedAppSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let scratch = try makeDirectory(named: "same-machine-copied-app")
        defer { try? FileManager.default.removeItem(at: scratch) }
        let copiedAppURL = scratch.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: copiedAppURL)

        let fakeAuditURL = scratch.appendingPathComponent("fake-audit.sh")
        try writeCurrentAuditScript(to: fakeAuditURL)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        try writeCurrentNotarizationSidecar(appBundleURL: copiedAppURL)

        let passing = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: passing.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }
        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("source.notarization.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
    }

    func testSameMachineHarnessHonorsPerMachineCopiedAppDirsWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let scratch = try makeDirectory(named: "same-machine-dual-copied-apps")
        defer { try? FileManager.default.removeItem(at: scratch) }

        let sourceAppURL = scratch.appendingPathComponent("Source.app", isDirectory: true)
        let targetAppURL = scratch.appendingPathComponent("Target.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: sourceAppURL)
        try FileManager.default.copyItem(at: builtAppURL, to: targetAppURL)

        let sourceWorkDir = "/tmp/source-app.notary"
        let targetWorkDir = "/tmp/target-app.notary"
        try writeCurrentNotarizationSidecar(appBundleURL: sourceAppURL, workDir: sourceWorkDir)
        try writeCurrentNotarizationSidecar(appBundleURL: targetAppURL, workDir: targetWorkDir)

        let fakeAuditURL = scratch.appendingPathComponent("fake-audit.sh")
        try writeCurrentAuditScript(to: fakeAuditURL)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_SOURCE_APP_DIR": sourceAppURL.path,
                "SUPERMOVER_ACCEPTANCE_TARGET_APP_DIR": targetAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: result.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }
        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)

        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.work_dir, sourceWorkDir)
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.work_dir, targetWorkDir)
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("source.notarization.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
    }

    func testSameMachineHarnessUsesSplitSourceAndTargetBundlesWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let scratch = try makeDirectory(named: "same-machine-split-bundles")
        defer { try? FileManager.default.removeItem(at: scratch) }

        let sourceAppURL = scratch.appendingPathComponent("Source.app", isDirectory: true)
        let targetAppURL = scratch.appendingPathComponent("Target.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: sourceAppURL)
        try FileManager.default.copyItem(at: builtAppURL, to: targetAppURL)

        try writeCurrentNotarizationSidecar(appBundleURL: sourceAppURL, workDir: "/tmp/source-split.notary")
        try writeCurrentNotarizationSidecar(appBundleURL: targetAppURL, workDir: "/tmp/target-split.notary")

        let fakeAuditURL = scratch.appendingPathComponent("fake-audit.sh")
        try writeCurrentAuditScript(to: fakeAuditURL)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_SOURCE_APP_DIR": sourceAppURL.path,
                "SUPERMOVER_ACCEPTANCE_TARGET_APP_DIR": targetAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: result.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let finalBundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let sourceBundleRoot = workDir.appendingPathComponent("source-bundle", isDirectory: true)
        let targetBundleRoot = workDir.appendingPathComponent("target-bundle", isDirectory: true)

        XCTAssertTrue(FileManager.default.fileExists(atPath: sourceBundleRoot.appendingPathComponent("source.browse.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: sourceBundleRoot.appendingPathComponent("source.transfer.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: targetBundleRoot.appendingPathComponent("target.advertise.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: targetBundleRoot.appendingPathComponent("target.ready.phase-2.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: finalBundleRoot.appendingPathComponent("source.transfer.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: finalBundleRoot.appendingPathComponent("target.advertise.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: finalBundleRoot.appendingPathComponent("evaluation.json").path))
    }

    func testSameMachineHarnessSupportsArchiveHandoffWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let scratch = try makeDirectory(named: "same-machine-archive-handoff")
        defer { try? FileManager.default.removeItem(at: scratch) }

        let sourceAppURL = scratch.appendingPathComponent("Source.app", isDirectory: true)
        let targetAppURL = scratch.appendingPathComponent("Target.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: sourceAppURL)
        try FileManager.default.copyItem(at: builtAppURL, to: targetAppURL)

        try writeCurrentNotarizationSidecar(appBundleURL: sourceAppURL)
        try writeCurrentNotarizationSidecar(appBundleURL: targetAppURL)

        let fakeAuditURL = scratch.appendingPathComponent("fake-audit.sh")
        try writeCurrentAuditScript(to: fakeAuditURL)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_SOURCE_APP_DIR": sourceAppURL.path,
                "SUPERMOVER_ACCEPTANCE_TARGET_APP_DIR": targetAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
                "SUPERMOVER_ACCEPTANCE_USE_ARCHIVE_HANDOFF": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: result.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let finalBundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        XCTAssertTrue(FileManager.default.fileExists(atPath: workDir.appendingPathComponent("target-to-final-phase-1.tgz").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: workDir.appendingPathComponent("source-to-final-phase-1.tgz").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: workDir.appendingPathComponent("target-to-source-phase-1.tgz").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: workDir.appendingPathComponent("source-to-target-pairing.tgz").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: workDir.appendingPathComponent("target-to-final-import.tgz").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: workDir.appendingPathComponent("target-to-final-ready.tgz").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: workDir.appendingPathComponent("target-to-source-ready.tgz").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: workDir.appendingPathComponent("source-to-final-transfer.tgz").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: finalBundleRoot.appendingPathComponent("evaluation.json").path))

        let metaData = try Data(contentsOf: finalBundleRoot.appendingPathComponent("meta.json"))
        let meta = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        let evidence = meta["evidence"] as? [String: Any] ?? [:]
        let handoffs = evidence["bundle_handoffs"] as? [[String: Any]] ?? []
        XCTAssertGreaterThanOrEqual(handoffs.count, 5)
        XCTAssertTrue(handoffs.contains(where: { ($0["archive"] as? String) == "target-to-final-phase-1.tgz" && ($0["verified"] as? Bool) == true }))
        XCTAssertTrue(handoffs.contains(where: { ($0["archive"] as? String) == "source-to-final-phase-1.tgz" && ($0["verified"] as? Bool) == true }))
        XCTAssertTrue(handoffs.contains(where: { ($0["archive"] as? String) == "target-to-source-phase-1.tgz" && ($0["verified"] as? Bool) == true }))
        XCTAssertTrue(handoffs.contains(where: { ($0["archive"] as? String) == "source-to-target-pairing.tgz" && ($0["verified"] as? Bool) == true }))
        XCTAssertTrue(handoffs.contains(where: { ($0["archive"] as? String) == "target-to-final-import.tgz" && ($0["verified"] as? Bool) == true }))
        XCTAssertTrue(handoffs.contains(where: { ($0["archive"] as? String) == "target-to-source-ready.tgz" && ($0["verified"] as? Bool) == true }))
        XCTAssertTrue(handoffs.contains(where: { ($0["archive"] as? String) == "source-to-final-transfer.tgz" && ($0["verified"] as? Bool) == true }))
    }

    func testSameMachineHarnessHonorsNotarizeScriptWorkflowSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let scratch = try makeDirectory(named: "same-machine-copied-app-workflow-sidecar")
        defer { try? FileManager.default.removeItem(at: scratch) }
        let copiedAppURL = scratch.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: copiedAppURL)

        let harness = try makeNotaryHarness(
            auditStatus: "pass",
            auditReadiness: "distribution_ready",
            auditPassReady: true,
            auditBlockingChecks: 0
        )
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }

        let workflowDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workflowDir) }

        let notarize = try runNotarizeScript(
            arguments: [
                "--app", copiedAppURL.path,
                "--work-dir", workflowDir.path,
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

        let fakeAuditURL = scratch.appendingPathComponent("fake-audit.sh")
        try """
        #!/bin/sh
        cat <<'EOF'
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "pass",
          "readiness": "distribution_ready",
          "summary": {
            "pass_ready": true,
            "blocking_checks": 0
          }
        }
        EOF
        """.write(to: fakeAuditURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditURL.path)

        let sidecarURL = copiedAppURL.deletingLastPathComponent().appendingPathComponent("\(copiedAppURL.lastPathComponent).notary/notarization.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarURL.path))

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let passing = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: passing.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }
        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("source.notarization.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.status, "pass")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.status, "pass")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.submission?.status, "Accepted")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.audit?.readiness, "distribution_ready")
    }

    func testSameMachineHarnessHonorsCanonicalBuiltAppWorkflowSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let sidecarURL = appURL.deletingLastPathComponent().appendingPathComponent("\(appURL.lastPathComponent).notary/notarization.json")
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

        let workflowDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workflowDir) }
        defer {
            try? FileManager.default.removeItem(at: sidecarURL)
            try? FileManager.default.removeItem(at: sidecarURL.deletingLastPathComponent())
        }

        let notarize = try runNotarizeScript(
            arguments: [
                "--app", appURL.path,
                "--work-dir", workflowDir.path,
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

        let fakeAuditURL = workflowDir.appendingPathComponent("fake-audit.sh")
        try """
        #!/bin/sh
        cat <<'EOF'
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "pass",
          "readiness": "distribution_ready",
          "summary": {
            "pass_ready": true,
            "blocking_checks": 0
          }
        }
        EOF
        """.write(to: fakeAuditURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditURL.path)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: result.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }
        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("source.notarization.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.status, "pass")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.status, "pass")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.submission?.status, "Accepted")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.audit?.readiness, "distribution_ready")
    }

    func testSameMachineHarnessClearsStaleNotarizationEvidenceWhenCopiedSidecarDisappearsWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let scratch = try makeDirectory(named: "same-machine-copied-app-stale-sidecar")
        defer { try? FileManager.default.removeItem(at: scratch) }
        let copiedAppURL = scratch.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: copiedAppURL)

        let fakeAuditURL = scratch.appendingPathComponent("fake-audit.sh")
        try """
        #!/bin/sh
        cat <<'EOF'
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "pass",
          "readiness": "distribution_ready",
          "summary": {
            "pass_ready": true,
            "blocking_checks": 0
          }
        }
        EOF
        """.write(to: fakeAuditURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditURL.path)

        let copiedSidecarDir = scratch.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        try FileManager.default.createDirectory(at: copiedSidecarDir, withIntermediateDirectories: true)
        let copiedSidecarURL = copiedSidecarDir.appendingPathComponent("notarization.json")
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
        """.write(to: copiedSidecarURL, atomically: true, encoding: .utf8)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let firstResult = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        let firstWorkDir = try extractWorkDirectory(from: firstResult.stdout)
        defer { try? FileManager.default.removeItem(at: firstWorkDir) }
        let firstBundleRoot = firstWorkDir.appendingPathComponent("bundle", isDirectory: true)
        XCTAssertTrue(FileManager.default.fileExists(atPath: firstBundleRoot.appendingPathComponent("source.notarization.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: firstBundleRoot.appendingPathComponent("target.notarization.json").path))

        try FileManager.default.removeItem(at: copiedSidecarURL)

        let secondResult = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        let secondWorkDir = try extractWorkDirectory(from: secondResult.stdout)
        defer { try? FileManager.default.removeItem(at: secondWorkDir) }
        let secondBundleRoot = secondWorkDir.appendingPathComponent("bundle", isDirectory: true)
        XCTAssertFalse(FileManager.default.fileExists(atPath: secondBundleRoot.appendingPathComponent("source.notarization.json").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: secondBundleRoot.appendingPathComponent("target.notarization.json").path))

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: secondBundleRoot)
        XCTAssertNil(snapshot.sourceNotarization)
        XCTAssertNil(snapshot.sourceNotarizationArtifact)
        XCTAssertNil(snapshot.targetNotarization)
        XCTAssertNil(snapshot.targetNotarizationArtifact)
    }

    func testSameMachineHarnessRejectsMalformedCopiedSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let scratch = try makeDirectory(named: "same-machine-copied-app-malformed-sidecar")
        defer { try? FileManager.default.removeItem(at: scratch) }
        let copiedAppURL = scratch.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: copiedAppURL)

        let fakeAuditURL = scratch.appendingPathComponent("fake-audit.sh")
        try """
        #!/bin/sh
        cat <<'EOF'
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "pass",
          "readiness": "distribution_ready",
          "summary": {
            "pass_ready": true,
            "blocking_checks": 0
          }
        }
        EOF
        """.write(to: fakeAuditURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditURL.path)

        let copiedSidecarDir = scratch.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        try FileManager.default.createDirectory(at: copiedSidecarDir, withIntermediateDirectories: true)
        try #"{"schema":"bad"}"#.write(to: copiedSidecarDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )
        let stdout = result.stdout
        let stderr = result.stderr

        XCTAssertEqual(result.exitCode, 5, "stdout:\n\(stdout)\nstderr:\n\(stderr)")
        XCTAssertTrue(stderr.contains("target serve exited before readiness was recorded"), "stderr:\n\(stderr)")
        XCTAssertTrue(stderr.contains("malformed target notarization evidence"), "stderr:\n\(stderr)")
        XCTAssertTrue(stderr.contains("target pairing serve readiness failed"), "stderr:\n\(stderr)")
        XCTAssertTrue(stderr.contains("same-machine two-machine acceptance failed"), "stderr:\n\(stderr)")
    }

    func testSameMachineHarnessCleansUpTargetServeProcessWhenSourceBrowseFailsAfterReadyWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let scratch = try makeDirectory(named: "same-machine-cleanup-after-source-browse-failure")
        defer { try? FileManager.default.removeItem(at: scratch) }

        let sourceAppURL = scratch.appendingPathComponent("Source.app", isDirectory: true)
        let targetAppURL = scratch.appendingPathComponent("Target.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: sourceAppURL)
        try FileManager.default.copyItem(at: builtAppURL, to: targetAppURL)
        try writeCurrentNotarizationSidecar(appBundleURL: targetAppURL)
        try writeCurrentNotarizationSidecar(appBundleURL: sourceAppURL)
        try #"{"schema":"bad"}"#.write(
            to: AcceptanceReleaseEvidenceFixtures.canonicalNotarizationSidecarURL(appBundleURL: sourceAppURL),
            atomically: true,
            encoding: .utf8
        )

        let fakeAuditURL = scratch.appendingPathComponent("fake-audit.sh")
        try writeCurrentAuditScript(to: fakeAuditURL)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_SOURCE_APP_DIR": sourceAppURL.path,
                "SUPERMOVER_ACCEPTANCE_TARGET_APP_DIR": targetAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )
        let stdout = result.stdout
        let stderr = result.stderr

        XCTAssertEqual(result.exitCode, 5, "stdout:\n\(stdout)\nstderr:\n\(stderr)")
        XCTAssertTrue(stderr.contains("same-machine two-machine acceptance failed"), "stderr:\n\(stderr)")

        let failedWorkDir = try extractFailedWorkDirectory(from: stderr)
        let targetBundleRoot = failedWorkDir.appendingPathComponent("target-bundle", isDirectory: true)
        XCTAssertTrue(FileManager.default.fileExists(atPath: targetBundleRoot.appendingPathComponent("target.ready.phase-1.json").path))
        let sourceBrowseStderr = try String(contentsOf: failedWorkDir.appendingPathComponent("source-browse.err"), encoding: .utf8)
        XCTAssertTrue(sourceBrowseStderr.contains("malformed source notarization evidence"), "source-browse.err:\n\(sourceBrowseStderr)")

        let pidText = try String(contentsOf: targetBundleRoot.appendingPathComponent("target.serve.phase-1.pid"), encoding: .utf8)
        let pidValue = try XCTUnwrap(Int32(pidText.trimmingCharacters(in: .whitespacesAndNewlines)))
        defer {
            if processIsRunning(pidValue) {
                _ = kill(pidValue, SIGTERM)
            }
        }
        XCTAssertFalse(processIsRunning(pidValue), "target serve process should be cleaned up after source-browse failure; pid=\(pidValue)\nstderr:\n\(stderr)")
    }

    func testSameMachineHarnessRejectsMalformedCanonicalBuiltAppWorkflowSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let sidecarURL = appURL.deletingLastPathComponent().appendingPathComponent("\(appURL.lastPathComponent).notary/notarization.json")
        if FileManager.default.fileExists(atPath: sidecarURL.path) {
            throw XCTSkip("canonical built app already has sidecar at \(sidecarURL.path); workflow-sidecar injection case is not applicable")
        }

        let harness = try makeNotaryHarness(
            auditStatus: "pass",
            auditReadiness: "distribution_ready",
            auditPassReady: true,
            auditBlockingChecks: 0
        )
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }

        let workflowDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workflowDir) }
        defer {
            try? FileManager.default.removeItem(at: sidecarURL)
            try? FileManager.default.removeItem(at: sidecarURL.deletingLastPathComponent())
        }

        let notarize = try runNotarizeScript(
            arguments: [
                "--app", appURL.path,
                "--work-dir", workflowDir.path,
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

        let fakeAuditURL = workflowDir.appendingPathComponent("fake-audit.sh")
        try """
        #!/bin/sh
        cat <<'EOF'
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "pass",
          "readiness": "distribution_ready",
          "summary": {
            "pass_ready": true,
            "blocking_checks": 0
          }
        }
        EOF
        """.write(to: fakeAuditURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditURL.path)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )
        let stdout = result.stdout
        let stderr = result.stderr

        XCTAssertEqual(result.exitCode, 5, "stdout:\n\(stdout)\nstderr:\n\(stderr)")
        XCTAssertTrue(stderr.contains("target serve exited before readiness was recorded"), "stderr:\n\(stderr)")
        XCTAssertTrue(stderr.contains("malformed target notarization evidence"), "stderr:\n\(stderr)")
        XCTAssertTrue(stderr.contains("target pairing serve readiness failed"), "stderr:\n\(stderr)")
        XCTAssertTrue(stderr.contains("same-machine two-machine acceptance failed"), "stderr:\n\(stderr)")
    }

    func testEvaluationCoordinatorRejectsSameMachineCopiedAppBundleAtDistinctMachineGateWhenReleaseEvidencePresent() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let scratch = try makeDirectory(named: "same-machine-copied-app-eval")
        defer { try? FileManager.default.removeItem(at: scratch) }
        let copiedAppURL = scratch.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: copiedAppURL)

        let fakeAuditURL = scratch.appendingPathComponent("fake-audit.sh")
        try """
        #!/bin/sh
        cat <<'EOF'
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "pass",
          "readiness": "distribution_ready",
          "summary": {
            "pass_ready": true,
            "blocking_checks": 0
          }
        }
        EOF
        """.write(to: fakeAuditURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditURL.path)

        let copiedSidecarDir = scratch.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        try FileManager.default.createDirectory(at: copiedSidecarDir, withIntermediateDirectories: true)
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
        """.write(to: copiedSidecarDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: result.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }
        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)

        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        guard var document = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw IntegrationError.malformedMeta(metaURL.path)
        }
        var evidence = document["evidence"] as? [String: Any] ?? [:]
        evidence["operator"] = [
            "local_network": ["status": "pass", "detail": "accepted prompt"],
            "firewall": ["status": "pass", "detail": "allowed inbound"],
            "pairing_confirmation": ["status": "pass", "detail": "confirmed code"],
        ]
        document["evidence"] = evidence
        let updated = try JSONSerialization.data(withJSONObject: document, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: metaURL, options: .atomic)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection("collection.mode=same_machine")
            )
        }
    }

    func testEvaluationCoordinatorRejectsSameMachineHarnessWhenReleaseNotarizationEvidenceIsMissing() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: result.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)

        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        guard var document = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw IntegrationError.malformedMeta(metaURL.path)
        }
        document["collection"] = [
            "mode": "two_machine",
            "machine_count": 2,
        ]
        document["roles"] = [
            "source_pair": [
                "profile": workDir.appendingPathComponent("source.profile.json").path,
                "status": "recorded",
                "machine_id": "source-machine",
                "machine_label": "source",
            ],
            "target": [
                "profile": workDir.appendingPathComponent("target.profile.json").path,
                "status": "recorded",
                "machine_id": "target-machine",
                "machine_label": "target",
            ],
        ]
        var evidence = document["evidence"] as? [String: Any] ?? [:]
        evidence["operator"] = [
            "local_network": ["status": "pass", "detail": "accepted prompt"],
            "firewall": ["status": "pass", "detail": "allowed inbound"],
            "pairing_confirmation": ["status": "pass", "detail": "confirmed code"],
        ]
        document["evidence"] = evidence
        let updated = try JSONSerialization.data(withJSONObject: document, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: metaURL, options: .atomic)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingRequiredArtifact("source.notarization.json")
            )
        }
    }

    func testEvaluationCoordinatorRejectsSameMachineHarnessWhenCurrentSourceBaselineArtifactIsMissing() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_SAME_MACHINE_EVALUATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [repoRoot.appendingPathComponent("macos/script/acceptance-two-machine-same-machine.sh").path],
            environment: [
                "SUPERMOVER_ACCEPTANCE_SKIP_BUILD": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        let workDir = try extractWorkDirectory(from: result.stdout)
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)

        let baselineURL = bundleRoot.appendingPathComponent("source.baseline.json")
        if FileManager.default.fileExists(atPath: baselineURL.path) {
            try FileManager.default.removeItem(at: baselineURL)
        }

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundleRoot,
                targetRootURL: targetRoot,
                requireOperatorEvidence: false
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingRequiredArtifact("source.baseline.json")
            )
        }
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

    private func restoreHarnessTargetImport(bundleRootURL: URL, targetRootURL: URL) throws {
        let pairingDir = targetRootURL.appendingPathComponent(".supermover/pairings", isDirectory: true)
        let pairingReceiptURL = try XCTUnwrap(
            try FileManager.default.contentsOfDirectory(at: pairingDir, includingPropertiesForKeys: nil)
                .first(where: { $0.pathExtension == "json" })
        )
        let pairingReceiptID = pairingReceiptURL.deletingPathExtension().lastPathComponent

        let metaURL = bundleRootURL.appendingPathComponent("meta.json")
        let data = try Data(contentsOf: metaURL)
        guard var document = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw IntegrationError.malformedMeta(metaURL.path)
        }
        var evidence = document["evidence"] as? [String: Any] ?? [:]
        evidence["target_import"] = [
            "pairing_receipt_id": pairingReceiptID,
            "adopted": "target.adopt-pairing.txt",
        ]
        document["evidence"] = evidence
        let updated = try JSONSerialization.data(withJSONObject: document, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: metaURL, options: .atomic)

        try "restored adopt transcript".write(
            to: bundleRootURL.appendingPathComponent("target.adopt-pairing.txt"),
            atomically: true,
            encoding: .utf8
        )
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

    private func extractFailedWorkDirectory(from stderr: String) throws -> URL {
        let prefix = "same-machine two-machine acceptance failed "
        guard let line = stderr
            .split(whereSeparator: \.isNewline)
            .reversed()
            .first(where: { $0.hasPrefix(prefix) }),
              let separator = line.lastIndex(of: ":") else {
            throw IntegrationError.missingWorkDirectory(stderr)
        }
        let path = line[line.index(after: separator)...].trimmingCharacters(in: .whitespacesAndNewlines)
        guard !path.isEmpty else {
            throw IntegrationError.missingWorkDirectory(stderr)
        }
        return URL(fileURLWithPath: path, isDirectory: true)
    }

    private func writeCurrentAuditScript(to scriptURL: URL) throws {
        try """
        #!/bin/sh
        set -eu
        app_path=$1
        python3 - "$app_path" <<'PY'
        import json
        import pathlib
        import sys

        app_path = pathlib.Path(sys.argv[1])
        provenance_path = app_path / "Contents" / "Resources" / "supermover-provenance.json"
        with provenance_path.open() as handle:
            provenance_manifest = json.load(handle)
        print(json.dumps({
            "schema": "supermover.macos.app_audit.v1",
            "status": "pass",
            "readiness": "distribution_ready",
            "app_path": str(app_path),
            "summary": {
                "pass_ready": True,
                "blocking_checks": 0,
                "review_checks": 0
            },
            "provenance": {
                "path": str(provenance_path),
                "manifest": provenance_manifest
            }
        }, indent=2, sort_keys=True))
        PY
        """.write(to: scriptURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: scriptURL.path)
    }

    private func writeCurrentNotarizationSidecar(appBundleURL: URL, workDir: String? = nil) throws {
        let sidecarDir = AcceptanceReleaseEvidenceFixtures.canonicalNotaryDirectoryURL(appBundleURL: appBundleURL)
        try FileManager.default.createDirectory(at: sidecarDir, withIntermediateDirectories: true)

        let provenanceManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: appBundleURL)
        let auditURL = AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditURL(appBundleURL: appBundleURL)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appBundleURL.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditURL, atomically: true, encoding: .utf8)

        var notarizationDocument: [String: Any] = [
            "schema": "supermover.macos.notarization.v1",
            "status": "pass",
            "app_path": appBundleURL.path,
            "submission": [
                "status": "Accepted"
            ],
            "audit": [
                "path": auditURL.path,
                "status": "pass",
                "readiness": "distribution_ready",
                "pass_ready": true,
                "blocking_checks": 0
            ]
        ]
        if let workDir {
            notarizationDocument["work_dir"] = workDir
        }
        let notarizationJSON = try AcceptanceReleaseEvidenceFixtures.jsonString(notarizationDocument)
        try notarizationJSON.write(
            to: AcceptanceReleaseEvidenceFixtures.canonicalNotarizationSidecarURL(appBundleURL: appBundleURL),
            atomically: true,
            encoding: .utf8
        )
    }

    private func processIsRunning(_ pid: Int32) -> Bool {
        kill(pid, 0) == 0
    }

    private func runProcess(
        executableURL: URL,
        arguments: [String],
        environment: [String: String],
        currentDirectoryURL: URL
    ) throws -> ProcessResult {
        let result = try runProcessAllowFailure(
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
        return result
    }

    private func runProcessAllowFailure(
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
        return ProcessResult(stdout: result.stdout, stderr: result.stderr, exitCode: result.exitCode)
    }

    private func repoRootURL(file: StaticString = #filePath) -> URL {
        URL(fileURLWithPath: "\(file)")
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
    }

    private func makeDirectory(named name: String) throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("\(name)-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }
}

private struct ProcessResult {
    let stdout: String
    let stderr: String
    let exitCode: Int32
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
