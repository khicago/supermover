import XCTest
@testable import SuperMoverApp

final class AcceptancePackagingEvidenceTests: XCTestCase {
    func testCollectorDefaultRunnersAgainstBuiltAppWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }
        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let bundle = try makeDirectory(named: "bundle-real")
        defer { try? FileManager.default.removeItem(at: bundle) }
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let outputs = try AcceptancePackagingEvidenceCollector().recordCurrentMachineEvidence(
            bundleRootURL: bundle,
            machine: "source",
            collectedBy: "integration",
            resourceURL: appURL.appendingPathComponent("Contents/Resources", isDirectory: true)
        )

        let sidecarURL = appURL.deletingLastPathComponent().appendingPathComponent("\(appURL.lastPathComponent).notary/notarization.json")
        let expectedOutputs = ["source.version.txt", "source.provenance.json", "source.app-audit.json"] + (FileManager.default.fileExists(atPath: sidecarURL.path) ? ["source.notarization.json"] : [])
        XCTAssertEqual(outputs, expectedOutputs)
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("source.version.txt").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("source.provenance.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("source.app-audit.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundle)
        XCTAssertEqual(snapshot.sourceAppAudit?.output, "source.app-audit.json")
        XCTAssertEqual(snapshot.sourceAppAudit?.collected_by, "integration")
        XCTAssertNotNil(snapshot.sourceAppAudit?.readiness)
        if expectedOutputs.contains("source.notarization.json") {
            XCTAssertTrue(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("source.notarization.json").path))
            XCTAssertEqual(snapshot.sourceNotarization?.output, "source.notarization.json")
            XCTAssertEqual(snapshot.sourceNotarizationArtifact?.status, "pass")
        } else {
            XCTAssertFalse(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("source.notarization.json").path))
            XCTAssertNil(snapshot.sourceNotarization)
            XCTAssertNil(snapshot.sourceNotarizationArtifact)
        }
    }

    func testCollectorDefaultRunnerUsesBundledAuditHelperWhenPresent() throws {
        let bundle = try makeDirectory(named: "bundle-default-helper")
        let app = FileManager.default.temporaryDirectory.appendingPathComponent("SuperMover-\(UUID().uuidString).app", isDirectory: true)
        try FileManager.default.createDirectory(at: app, withIntermediateDirectories: true)
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: app)
        }

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let resources = app.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resources.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)

        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(
            cliVersion: "supermover 0.1.0-dev"
        )
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest).write(
            to: resources.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try stubVersionScript.write(
            to: binDir.appendingPathComponent("supermover"),
            atomically: true,
            encoding: .utf8
        )
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: binDir.appendingPathComponent("supermover").path
        )
        try AcceptanceReleaseEvidenceFixtures.readyAuditScript(
            appPath: app.path,
            provenanceManifest: provenanceManifest
        ).write(
            to: binDir.appendingPathComponent("supermover-app-audit"),
            atomically: true,
            encoding: .utf8
        )
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: binDir.appendingPathComponent("supermover-app-audit").path
        )

        let outputs = try AcceptancePackagingEvidenceCollector().recordCurrentMachineEvidence(
            bundleRootURL: bundle,
            machine: "source",
            collectedBy: "test",
            resourceURL: resources
        )

        XCTAssertEqual(outputs, ["source.version.txt", "source.provenance.json", "source.app-audit.json"])
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundle)
        XCTAssertEqual(snapshot.sourceAppAudit?.status, "pass")
        XCTAssertEqual(snapshot.sourceAppAudit?.readiness, "distribution_ready")
    }

    func testCollectorWritesVersionProvenanceAndAppAudit() throws {
        let bundle = try makeDirectory(named: "bundle")
        let app = FileManager.default.temporaryDirectory.appendingPathComponent("SuperMover-\(UUID().uuidString).app", isDirectory: true)
        try FileManager.default.createDirectory(at: app, withIntermediateDirectories: true)
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: app)
        }

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let resources = app.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: resources, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.macos.provenance.v1",
          "git_commit": "abcdef123456",
          "cli_version": "supermover 0.1.0-dev",
          "cli_relative_path": "Contents/Resources/bin/supermover",
          "build_profile": "test",
          "signing": "unsigned"
        }
        """.write(to: resources.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)

        let appContents = app.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: appContents, withIntermediateDirectories: true)
        let collector = AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try """
                {
                  "schema": "supermover.macos.app_audit.v1",
                  "status": "blocked",
                  "readiness": "blocked",
                  "summary": {
                    "pass_ready": false,
                    "blocking_checks": 3
                  }
                }
                """.write(to: outputURL, atomically: true, encoding: .utf8)
                return .init(
                    exitCode: 1,
                    status: "blocked",
                    readiness: "blocked",
                    passReady: false,
                    blockingChecks: 3
                )
            }
        )

        let outputs = try collector.recordCurrentMachineEvidence(
            bundleRootURL: bundle,
            machine: "source",
            collectedBy: "test",
            resourceURL: appContents
        )

        XCTAssertEqual(outputs, ["source.version.txt", "source.provenance.json", "source.app-audit.json"])
        XCTAssertEqual(try String(contentsOf: bundle.appendingPathComponent("source.version.txt")), "supermover 0.1.0-dev\n")
        XCTAssertEqual(
            try String(contentsOf: bundle.appendingPathComponent("source.provenance.json")),
            try String(contentsOf: resources.appendingPathComponent("supermover-provenance.json"))
        )
        let snapshot = try AcceptanceBundleReader().read(bundleRootURL: bundle)
        XCTAssertEqual(snapshot.sourceAppAudit?.output, "source.app-audit.json")
        XCTAssertEqual(snapshot.sourceAppAudit?.exit_code, 1)
        XCTAssertEqual(snapshot.sourceAppAudit?.blocking_checks, 3)
    }

    func testCollectorInspectionTreatsReviewOnlyAppAuditAsNotInstallReady() throws {
        let bundle = try makeDirectory(named: "bundle-review-only-audit")
        let app = FileManager.default.temporaryDirectory.appendingPathComponent("SuperMover-\(UUID().uuidString).app", isDirectory: true)
        try FileManager.default.createDirectory(at: app, withIntermediateDirectories: true)
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: app)
        }

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let resources = app.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: resources, withIntermediateDirectories: true)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(
            cliVersion: "supermover 0.1.0-dev"
        )
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest).write(
            to: resources.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        let collector = AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
                    appPath: app.path,
                    provenanceManifest: provenanceManifest,
                    status: "pass",
                    readiness: "review_only",
                    passReady: true,
                    blockingChecks: 0,
                    reviewChecks: 1
                ).write(to: outputURL, atomically: true, encoding: .utf8)
                return .init(
                    exitCode: 0,
                    status: "pass",
                    readiness: "review_only",
                    passReady: true,
                    blockingChecks: 0
                )
            }
        )

        let inspection = try collector.inspectCurrentMachineEvidence(
            bundleRootURL: bundle,
            machine: "source",
            resourceURL: resources
        )

        XCTAssertFalse(inspection.audit.installReady)
        XCTAssertEqual(inspection.audit.readiness, "review_only")
        XCTAssertEqual(
            inspection.audit.failureMessage,
            "Local app audit for the current packaged app is review_only and not install-ready."
        )
    }

    func testCollectorCopiesStructuredNotarizationEvidenceWhenPresent() throws {
        let bundle = try makeDirectory(named: "bundle")
        let app = FileManager.default.temporaryDirectory.appendingPathComponent("SuperMover-\(UUID().uuidString).app", isDirectory: true)
        try FileManager.default.createDirectory(at: app, withIntermediateDirectories: true)
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: app)
        }

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let resources = app.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: resources, withIntermediateDirectories: true)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(
            cliVersion: "supermover 0.1.0-dev"
        )
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest).write(
            to: resources.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        let notaryDir = app.deletingLastPathComponent().appendingPathComponent("\(app.lastPathComponent).notary", isDirectory: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
        let postStapleAuditURL = notaryDir.appendingPathComponent("post-staple.audit.json")
        let notaryLogURL = notaryDir.appendingPathComponent("notary-log.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: app.path,
            provenanceManifest: provenanceManifest
        ).write(
            to: postStapleAuditURL,
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(
            to: notaryLogURL,
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "schema": "supermover.macos.notarization.v1",
          "checked_at": "2026-06-01T12:00:00Z",
          "status": "pass",
          "app_path": "\(app.path)",
          "work_dir": "/tmp/notary",
          "auth_mode": "keychain_profile",
          "archive_path": "/tmp/notary/SuperMover.app.zip",
          "submission": {
            "id": "11111111-1111-1111-1111-111111111111",
            "status": "Accepted",
            "message": "Ready for distribution"
          },
          "notary_log": {
            "path": "\(notaryLogURL.path)"
          },
          "audit": {
            "path": "\(postStapleAuditURL.path)",
            "status": "pass",
            "readiness": "distribution_ready",
            "pass_ready": true,
            "blocking_checks": 0
          },
          "failure": null
        }
        """.write(to: notaryDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        let collector = AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try """
                {
                  "schema": "supermover.macos.app_audit.v1",
                  "status": "pass",
                  "readiness": "distribution_ready",
                  "summary": {
                    "pass_ready": true,
                    "blocking_checks": 0
                  }
                }
                """.write(to: outputURL, atomically: true, encoding: .utf8)
                return .init(
                    exitCode: 0,
                    status: "pass",
                    readiness: "distribution_ready",
                    passReady: true,
                    blockingChecks: 0
                )
            }
        )

        let outputs = try collector.recordCurrentMachineEvidence(
            bundleRootURL: bundle,
            machine: "source",
            collectedBy: "test",
            resourceURL: resources
        )

        XCTAssertEqual(outputs, ["source.version.txt", "source.provenance.json", "source.app-audit.json", "source.notarization.json", "source.notary-log.json"])
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("source.notarization.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("source.notary-log.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundle)
        XCTAssertEqual(snapshot.sourceNotarization?.output, "source.notarization.json")
        XCTAssertEqual(snapshot.sourceNotarization?.notary_log, "source.notary-log.json")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.status, "pass")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.notary_log?.path, "source.notary-log.json")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.audit?.readiness, "distribution_ready")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.submission?.status, "Accepted")
    }

    func testCollectorRejectsMalformedBundledProvenance() throws {
        let bundle = try makeDirectory(named: "bundle")
        let app = FileManager.default.temporaryDirectory.appendingPathComponent("SuperMover-\(UUID().uuidString).app", isDirectory: true)
        try FileManager.default.createDirectory(at: app, withIntermediateDirectories: true)
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: app)
        }
        let resources = app.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: resources, withIntermediateDirectories: true)

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"bad"}"#.write(to: resources.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)

        XCTAssertThrowsError(
            try AcceptancePackagingEvidenceCollector().recordCurrentMachineEvidence(
                bundleRootURL: bundle,
                machine: "source",
                collectedBy: "test",
                resourceURL: resources
            )
        ) { error in
            guard case .malformedBundledProvenance = error as? AcceptancePackagingEvidenceCollector.CollectionError else {
                return XCTFail("unexpected error: \(error)")
            }
        }
    }

    func testCollectorRejectsMalformedStructuredNotarizationEvidenceWithoutPublishingFreshPackagingArtifacts() throws {
        let bundle = try makeDirectory(named: "bundle")
        let app = FileManager.default.temporaryDirectory.appendingPathComponent("SuperMover-\(UUID().uuidString).app", isDirectory: true)
        try FileManager.default.createDirectory(at: app, withIntermediateDirectories: true)
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: app)
        }
        let resources = app.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: resources, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.macos.provenance.v1",
          "git_commit": "abcdef123456",
          "cli_version": "supermover 0.1.0-dev",
          "cli_relative_path": "Contents/Resources/bin/supermover",
          "build_profile": "test",
          "signing": "developer_id"
        }
        """.write(to: resources.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        let notaryDir = app.deletingLastPathComponent().appendingPathComponent("\(app.lastPathComponent).notary", isDirectory: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
        try #"{"schema":"bad"}"#.write(to: notaryDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {
            "notarization": {
              "source": {
                "collected_by": "stale",
                "output": "source.notarization.json",
                "status": "pass",
                "audit_status": "pass",
                "audit_readiness": "distribution_ready",
                "audit_pass_ready": true
              }
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"stale"}"#.write(to: bundle.appendingPathComponent("source.notarization.json"), atomically: true, encoding: .utf8)

        let collector = AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try """
                {
                  "schema": "supermover.macos.app_audit.v1",
                  "status": "pass",
                  "readiness": "distribution_ready",
                  "summary": {
                    "pass_ready": true,
                    "blocking_checks": 0
                  }
                }
                """.write(to: outputURL, atomically: true, encoding: .utf8)
                return .init(
                    exitCode: 0,
                    status: "pass",
                    readiness: "distribution_ready",
                    passReady: true,
                    blockingChecks: 0
                )
            }
        )

        XCTAssertThrowsError(
            try collector.recordCurrentMachineEvidence(
                bundleRootURL: bundle,
                machine: "source",
                collectedBy: "test",
                resourceURL: resources
            )
        ) { error in
            guard case .malformedNotarizationOutput = error as? AcceptancePackagingEvidenceCollector.CollectionError else {
                return XCTFail("unexpected error: \(error)")
            }
        }
        try assertNoPublishedPackagingArtifacts(bundleRootURL: bundle)
    }

    func testCollectorRejectsStaleStructuredNotarizationEvidenceWithoutPublishingFreshPackagingArtifacts() throws {
        let bundle = try makeDirectory(named: "bundle")
        let app = FileManager.default.temporaryDirectory.appendingPathComponent("SuperMover-\(UUID().uuidString).app", isDirectory: true)
        try FileManager.default.createDirectory(at: app, withIntermediateDirectories: true)
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: app)
        }
        let resources = app.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: resources, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.macos.provenance.v1",
          "git_commit": "abcdef123456",
          "cli_version": "supermover 0.1.0-dev",
          "cli_relative_path": "Contents/Resources/bin/supermover",
          "build_profile": "test",
          "signing": "developer_id"
        }
        """.write(to: resources.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        let notaryDir = app.deletingLastPathComponent().appendingPathComponent("\(app.lastPathComponent).notary", isDirectory: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.macos.notarization.v1",
          "checked_at": "2026-06-01T12:00:00Z",
          "status": "pass",
          "app_path": "/tmp/Other.app",
          "submission": {
            "status": "Accepted"
          },
          "audit": {
            "path": "\(notaryDir.appendingPathComponent("post-staple.audit.json").path)",
            "status": "pass",
            "readiness": "distribution_ready",
            "pass_ready": true
          }
        }
        """.write(to: notaryDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {
            "notarization": {
              "source": {
                "collected_by": "stale",
                "output": "source.notarization.json",
                "status": "pass",
                "audit_status": "pass",
                "audit_readiness": "distribution_ready",
                "audit_pass_ready": true
              }
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"stale"}"#.write(to: bundle.appendingPathComponent("source.notarization.json"), atomically: true, encoding: .utf8)

        let collector = AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try """
                {
                  "schema": "supermover.macos.app_audit.v1",
                  "status": "pass",
                  "readiness": "distribution_ready",
                  "summary": {
                    "pass_ready": true,
                    "blocking_checks": 0
                  }
                }
                """.write(to: outputURL, atomically: true, encoding: .utf8)
                return .init(
                    exitCode: 0,
                    status: "pass",
                    readiness: "distribution_ready",
                    passReady: true,
                    blockingChecks: 0
                )
            }
        )

        XCTAssertThrowsError(
            try collector.recordCurrentMachineEvidence(
                bundleRootURL: bundle,
                machine: "source",
                collectedBy: "test",
                resourceURL: resources
            )
        ) { error in
            guard case .staleNotarizationOutput = error as? AcceptancePackagingEvidenceCollector.CollectionError else {
                return XCTFail("unexpected error: \(error)")
            }
        }
        try assertNoPublishedPackagingArtifacts(bundleRootURL: bundle)
    }

    func testCollectorRejectsMalformedSiblingNotaryLogWithoutPublishingFreshPackagingArtifacts() throws {
        let bundle = try makeDirectory(named: "bundle")
        let app = FileManager.default.temporaryDirectory.appendingPathComponent("SuperMover-\(UUID().uuidString).app", isDirectory: true)
        try FileManager.default.createDirectory(at: app, withIntermediateDirectories: true)
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: app)
        }
        let resources = app.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: resources, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {
            "notarization": {
              "source": {
                "collected_by": "stale",
                "output": "source.notarization.json",
                "notary_log": "source.notary-log.json",
                "status": "pass",
                "audit_status": "pass",
                "audit_readiness": "distribution_ready",
                "audit_pass_ready": true
              }
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(
            cliVersion: "supermover 0.1.0-dev"
        )
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest).write(
            to: resources.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        let notaryDir = app.deletingLastPathComponent().appendingPathComponent("\(app.lastPathComponent).notary", isDirectory: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
        let postStapleAuditURL = notaryDir.appendingPathComponent("post-staple.audit.json")
        let notaryLogURL = notaryDir.appendingPathComponent("notary-log.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: app.path,
            provenanceManifest: provenanceManifest
        ).write(to: postStapleAuditURL, atomically: true, encoding: .utf8)
        try #"{"status":"Accepted","issues":"none"}"#.write(to: notaryLogURL, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: app.path,
            auditPath: postStapleAuditURL.path,
            notaryLogPath: notaryLogURL.path
        ).write(to: notaryDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"stale"}"#.write(to: bundle.appendingPathComponent("source.notarization.json"), atomically: true, encoding: .utf8)
        try #"{"status":"Accepted","issues":"stale"}"#.write(to: bundle.appendingPathComponent("source.notary-log.json"), atomically: true, encoding: .utf8)

        let collector = AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
                    appPath: app.path,
                    provenanceManifest: provenanceManifest
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

        XCTAssertThrowsError(
            try collector.recordCurrentMachineEvidence(
                bundleRootURL: bundle,
                machine: "source",
                collectedBy: "test",
                resourceURL: resources
            )
        ) { error in
            guard case .staleNotarizationOutput = error as? AcceptancePackagingEvidenceCollector.CollectionError else {
                return XCTFail("unexpected error: \(error)")
            }
        }
        try assertNoPublishedPackagingArtifacts(bundleRootURL: bundle)
    }

    func testCollectorClearsStaleNotarizationEvidenceWhenLocalSidecarMissing() throws {
        let bundle = try makeDirectory(named: "bundle")
        let app = FileManager.default.temporaryDirectory.appendingPathComponent("SuperMover-\(UUID().uuidString).app", isDirectory: true)
        try FileManager.default.createDirectory(at: app, withIntermediateDirectories: true)
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: app)
        }
        let resources = app.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: resources, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {
            "notarization": {
              "source": {
                "collected_by": "stale",
                "output": "source.notarization.json",
                "status": "pass",
                "audit_status": "pass",
                "audit_readiness": "distribution_ready",
                "audit_pass_ready": true
              }
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.macos.provenance.v1",
          "git_commit": "abcdef123456",
          "cli_version": "supermover 0.1.0-dev",
          "cli_relative_path": "Contents/Resources/bin/supermover",
          "build_profile": "test",
          "signing": "developer_id"
        }
        """.write(to: resources.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
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
        """.write(to: bundle.appendingPathComponent("source.notarization.json"), atomically: true, encoding: .utf8)
        try FileManager.default.createSymbolicLink(
            atPath: bundle.appendingPathComponent("source.notary-log.json").path,
            withDestinationPath: bundle.appendingPathComponent("missing-stale-notary-log.json").path
        )

        let collector = AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try """
                {
                  "schema": "supermover.macos.app_audit.v1",
                  "status": "pass",
                  "readiness": "distribution_ready",
                  "summary": {
                    "pass_ready": true,
                    "blocking_checks": 0
                  }
                }
                """.write(to: outputURL, atomically: true, encoding: .utf8)
                return .init(
                    exitCode: 0,
                    status: "pass",
                    readiness: "distribution_ready",
                    passReady: true,
                    blockingChecks: 0
                )
            }
        )

        let outputs = try collector.recordCurrentMachineEvidence(
            bundleRootURL: bundle,
            machine: "source",
            collectedBy: "test",
            resourceURL: resources
        )

        XCTAssertEqual(outputs, ["source.version.txt", "source.provenance.json", "source.app-audit.json"])
        XCTAssertFalse(pathExistsOrIsSymlink(bundle.appendingPathComponent("source.notarization.json")))
        XCTAssertFalse(pathExistsOrIsSymlink(bundle.appendingPathComponent("source.notary-log.json")))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundle)
        XCTAssertNil(snapshot.sourceNotarization)
        XCTAssertNil(snapshot.sourceNotarizationArtifact)
    }

    func testCollectorUsesSiblingNotarizationSidecarForCopiedBuiltAppWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }
        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let scratch = try makeDirectory(named: "copied-built-app")
        defer { try? FileManager.default.removeItem(at: scratch) }
        let copiedAppURL = scratch.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: copiedAppURL)
        let copiedResourcesURL = copiedAppURL.appendingPathComponent("Contents/Resources", isDirectory: true)
        let copiedProvenanceManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: copiedAppURL)
        let copiedSidecarDir = scratch.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        try FileManager.default.createDirectory(at: copiedSidecarDir, withIntermediateDirectories: true)
        let postStapleAuditURL = copiedSidecarDir.appendingPathComponent("post-staple.audit.json")
        let notaryLogURL = copiedSidecarDir.appendingPathComponent("notary-log.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: copiedAppURL.path,
            provenanceManifest: copiedProvenanceManifest
        ).write(to: postStapleAuditURL, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(to: notaryLogURL, atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.macos.notarization.v1",
          "checked_at": "2026-06-01T12:00:00Z",
          "status": "pass",
          "app_path": "\(copiedAppURL.path)",
          "work_dir": "\(copiedSidecarDir.path)",
          "auth_mode": "keychain_profile",
          "archive_path": "\(copiedSidecarDir.appendingPathComponent("SuperMover.app.zip").path)",
          "submission": {
            "id": "11111111-1111-1111-1111-111111111111",
            "status": "Accepted",
            "message": "Ready for distribution"
          },
          "notary_log": {
            "path": "\(notaryLogURL.path)"
          },
          "audit": {
            "path": "\(postStapleAuditURL.path)",
            "status": "pass",
            "readiness": "distribution_ready",
            "pass_ready": true,
            "blocking_checks": 0
          },
          "failure": null
        }
        """.write(to: copiedSidecarDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        let bundle = try makeDirectory(named: "bundle-real-sidecar")
        defer { try? FileManager.default.removeItem(at: bundle) }
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let originalCWD = FileManager.default.currentDirectoryPath
        XCTAssertTrue(FileManager.default.changeCurrentDirectoryPath(repoRoot.path))
        defer { _ = FileManager.default.changeCurrentDirectoryPath(originalCWD) }

        let outputs = try AcceptancePackagingEvidenceCollector().recordCurrentMachineEvidence(
            bundleRootURL: bundle,
            machine: "source",
            collectedBy: "integration",
            resourceURL: copiedResourcesURL
        )

        XCTAssertEqual(outputs, ["source.version.txt", "source.provenance.json", "source.app-audit.json", "source.notarization.json", "source.notary-log.json"])
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundle)
        XCTAssertEqual(snapshot.sourceNotarization?.output, "source.notarization.json")
        XCTAssertEqual(snapshot.sourceNotarization?.notary_log, "source.notary-log.json")
        XCTAssertEqual(snapshot.sourceAppAuditArtifact?.app_path, copiedAppURL.path)
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.app_path, copiedAppURL.path)
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.audit?.path, postStapleAuditURL.path)
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.notary_log?.path, "source.notary-log.json")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.status, "pass")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.submission?.status, "Accepted")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.audit?.readiness, "distribution_ready")
    }

    func testCollectorConsumesNotarizeScriptSidecarFromSignedAppWorkflow() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let harness = try makeNotaryHarness(auditStatus: "pass", auditReadiness: "distribution_ready", auditPassReady: true, auditBlockingChecks: 0)
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

        let bundle = try makeDirectory(named: "bundle-script-sidecar")
        defer { try? FileManager.default.removeItem(at: bundle) }
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let outputs = try AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try """
                {
                  "schema": "supermover.macos.app_audit.v1",
                  "status": "pass",
                  "readiness": "distribution_ready",
                  "summary": {
                    "pass_ready": true,
                    "blocking_checks": 0
                  }
                }
                """.write(to: outputURL, atomically: true, encoding: .utf8)
                return .init(
                    exitCode: 0,
                    status: "pass",
                    readiness: "distribution_ready",
                    passReady: true,
                    blockingChecks: 0
                )
            }
        ).recordCurrentMachineEvidence(
            bundleRootURL: bundle,
            machine: "source",
            collectedBy: "workflow",
            resourceURL: app.appURL.appendingPathComponent("Contents/Resources", isDirectory: true)
        )

        XCTAssertEqual(outputs, ["source.version.txt", "source.provenance.json", "source.app-audit.json", "source.notarization.json", "source.notary-log.json"])
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundle)
        XCTAssertEqual(snapshot.sourceNotarization?.collected_by, "workflow")
        XCTAssertEqual(snapshot.sourceNotarization?.output, "source.notarization.json")
        XCTAssertEqual(snapshot.sourceNotarization?.notary_log, "source.notary-log.json")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.status, "pass")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.notary_log?.path, "source.notary-log.json")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.submission?.status, "Accepted")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.audit?.readiness, "distribution_ready")
    }

    @MainActor
    func testObserverCannotRecordPackagingEvidence() {
        let store = AppStore()
        store.selectedRole = .observer

        store.recordAcceptancePackagingEvidence()

        XCTAssertTrue(store.note.contains("Observer role cannot record local packaging evidence"))
    }

    private func makeDirectory(named name: String) throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("\(name)-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }

    private func repoRootURL(file: StaticString = #filePath) -> URL {
        URL(fileURLWithPath: "\(file)")
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
    }

    private var stubVersionScript: String {
        """
        #!/bin/sh
        printf 'supermover 0.1.0-dev\n'
        """
    }

    private func assertNoPublishedPackagingArtifacts(
        bundleRootURL: URL,
        file: StaticString = #filePath,
        line: UInt = #line
    ) throws {
        XCTAssertFalse(
            FileManager.default.fileExists(atPath: bundleRootURL.appendingPathComponent("source.version.txt").path),
            file: file,
            line: line
        )
        XCTAssertFalse(
            FileManager.default.fileExists(atPath: bundleRootURL.appendingPathComponent("source.provenance.json").path),
            file: file,
            line: line
        )
        XCTAssertFalse(
            FileManager.default.fileExists(atPath: bundleRootURL.appendingPathComponent("source.app-audit.json").path),
            file: file,
            line: line
        )
        XCTAssertFalse(
            pathExistsOrIsSymlink(bundleRootURL.appendingPathComponent("source.notarization.json")),
            file: file,
            line: line
        )
        XCTAssertFalse(
            pathExistsOrIsSymlink(bundleRootURL.appendingPathComponent("source.notary-log.json")),
            file: file,
            line: line
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRootURL)
        XCTAssertNil(snapshot.sourceAppAudit, file: file, line: line)
        XCTAssertNil(snapshot.sourceAppAuditArtifact, file: file, line: line)
        XCTAssertNil(snapshot.sourceProvenanceArtifact, file: file, line: line)
        XCTAssertNil(snapshot.sourceNotarization, file: file, line: line)
        XCTAssertNil(snapshot.sourceNotarizationArtifact, file: file, line: line)
    }

    private func pathExistsOrIsSymlink(_ url: URL) -> Bool {
        if FileManager.default.fileExists(atPath: url.path) {
            return true
        }
        if let values = try? url.resourceValues(forKeys: [.isSymbolicLinkKey]) {
            return values.isSymbolicLink == true
        }
        return false
    }
}
