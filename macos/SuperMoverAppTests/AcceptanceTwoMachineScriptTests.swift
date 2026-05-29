import XCTest
@testable import SuperMoverApp

final class AcceptanceTwoMachineScriptTests: XCTestCase {
    private static let machineIdentityCorrectionFailureMessage =
        "roles.source_pair/target machine_id do not match source.machine.json and target.machine.json"

    func testAcceptanceRequireReadyAppAuditForCollectionAcceptsDistributionReadyAudit() throws {
        let workDir = try makeDirectory(named: "acceptance-common-distribution-ready")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let notaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
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
              "machine_id": "source-machine",
              "machine_label": "source"
            }
          },
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: "supermover target-cli")
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: resourcesDir.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        let auditPath = notaryDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        let notaryLogPath = notaryDir.appendingPathComponent("notary-log.json")
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(to: notaryLogPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path,
            notaryLogPath: notaryLogPath.path
        ).write(to: notaryDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_require_ready_app_audit_for_collection \"$2\" target \"$3\"",
                "acceptance-common-check",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-common.sh").path,
                bundleRoot.path,
                appDir.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        XCTAssertEqual(result.stderr, "")
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
    }

    func testAcceptanceRequireReadyAppAuditForCollectionRejectsReviewOnlyAuditBeforeNotarizationGate() throws {
        let workDir = try makeDirectory(named: "acceptance-common-review-only-audit")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let notaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "target": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "target-machine",
              "machine_label": "target"
            }
          },
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: "supermover target-cli")
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: resourcesDir.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("target.provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest,
            status: "pass",
            readiness: "review_only",
            passReady: true,
            blockingChecks: 0,
            reviewChecks: 1
        ).write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        let auditPath = notaryDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest,
            status: "pass",
            readiness: "review_only",
            passReady: true,
            blockingChecks: 0,
            reviewChecks: 1
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        let notaryLogPath = notaryDir.appendingPathComponent("notary-log.json")
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(to: notaryLogPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path,
            notaryLogPath: notaryLogPath.path,
            auditStatus: "pass",
            auditReadiness: "review_only",
            auditPassReady: true
        ).write(to: notaryDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_require_ready_app_audit_for_collection \"$2\" target \"$3\"",
                "acceptance-common-check",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-common.sh").path,
                bundleRoot.path,
                appDir.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("installed-app acceptance requires install-ready target app audit before phase execution"))
        XCTAssertFalse(result.stderr.contains("requires release-ready target notarization evidence"))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.target.appAuditReady)
    }

    func testAcceptanceRequireReadyAppAuditForCollectionRejectsNotarizationWithoutSubmissionID() throws {
        let workDir = try makeDirectory(named: "acceptance-common-notarization-missing-submission-id")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let notaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "target": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "target-machine",
              "machine_label": "target"
            }
          },
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: "supermover target-cli")
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: resourcesDir.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        let auditPath = notaryDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        let notaryLogPath = notaryDir.appendingPathComponent("notary-log.json")
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(to: notaryLogPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path,
            submissionID: nil,
            notaryLogPath: notaryLogPath.path
        ).write(to: notaryDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_require_ready_app_audit_for_collection \"$2\" target \"$3\"",
                "acceptance-common-check",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-common.sh").path,
                bundleRoot.path,
                appDir.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("target.notary-log.json is not accepted notarization log evidence"))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notary-log.json").path))
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.target.notarizationReady)
    }

    func testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedSubmissionID() throws {
        let workDir = try makeDirectory(named: "acceptance-common-notarization-malformed-submission-id")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let notaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "target": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "target-machine",
              "machine_label": "target"
            }
          },
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: "supermover target-cli")
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: resourcesDir.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        let auditPath = notaryDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        let notaryLogPath = notaryDir.appendingPathComponent("notary-log.json")
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(to: notaryLogPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path,
            submissionID: "manual-pass",
            notaryLogPath: notaryLogPath.path
        ).write(to: notaryDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_require_ready_app_audit_for_collection \"$2\" target \"$3\"",
                "acceptance-common-check",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-common.sh").path,
                bundleRoot.path,
                appDir.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("target.notary-log.json is not accepted notarization log evidence"))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notary-log.json").path))
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.target.notarizationReady)
    }

    func testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedNotaryLog() throws {
        let workDir = try makeDirectory(named: "acceptance-common-notarization-malformed-notary-log")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let notaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "target": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "target-machine",
              "machine_label": "target"
            }
          },
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: "supermover target-cli")
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: resourcesDir.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        let auditPath = notaryDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        let notaryLogPath = notaryDir.appendingPathComponent("notary-log.json")
        try #"{"status":"Accepted","issues":"none"}"#.write(to: notaryLogPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path,
            notaryLogPath: notaryLogPath.path
        ).write(to: notaryDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_require_ready_app_audit_for_collection \"$2\" target \"$3\"",
                "acceptance-common-check",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-common.sh").path,
                bundleRoot.path,
                appDir.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("target.notary-log.json is not accepted notarization log evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notary-log.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertFalse(snapshot.installedAppReleaseEvidence.target.notarizationReady)
    }

    func testAcceptanceRequireReadyAppAuditForCollectionRejectsAcceptedNotaryLogFromDifferentSubmission() throws {
        let workDir = try makeDirectory(named: "acceptance-common-notarization-mismatched-notary-log")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let notaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "target": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "target-machine",
              "machine_label": "target"
            }
          },
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: "supermover target-cli")
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: resourcesDir.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        let auditPath = notaryDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        let notaryLogPath = notaryDir.appendingPathComponent("notary-log.json")
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON(
            submissionID: "22222222-2222-2222-2222-222222222222"
        ).write(to: notaryLogPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path,
            submissionID: "11111111-1111-1111-1111-111111111111",
            notaryLogPath: notaryLogPath.path
        ).write(to: notaryDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_require_ready_app_audit_for_collection \"$2\" target \"$3\"",
                "acceptance-common-check",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-common.sh").path,
                bundleRoot.path,
                appDir.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("target.notary-log.json is not accepted notarization log evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notary-log.json").path))
    }

    func testMachineFactsResolverUsesExplicitOverrideFirst() throws {
        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_two_machine_resolve_machine_id \"$2\"",
                "acceptance-machine-id",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-two-machine.sh").path,
                "source-machine-explicit",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.stdout.trimmingCharacters(in: .whitespacesAndNewlines), "source-machine-explicit")
    }

    func testMachineFactsResolverUsesSystemDerivedIDWhenOverrideMissing() throws {
        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_two_machine_resolve_machine_id \"\"",
                "acceptance-machine-id",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-two-machine.sh").path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let machineID = result.stdout.trimmingCharacters(in: .whitespacesAndNewlines)
        XCTAssertFalse(machineID.isEmpty)
        XCTAssertNotEqual(machineID, "macos-machine-unknown")
        XCTAssertTrue(
            machineID.hasPrefix("macos-platformuuid-") || machineID.hasPrefix("macos-hostname-"),
            "unexpected machine id: \(machineID)"
        )
    }

    func testAcceptanceRequireReadyAppAuditForCollectionRejectsMissingNotarizationEvidence() throws {
        let workDir = try makeDirectory(named: "acceptance-common-missing-notarization")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: appDir, withIntermediateDirectories: true)
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
              "machine_id": "source-machine",
              "machine_label": "source"
            }
          },
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
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
        """.write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_require_ready_app_audit_for_collection \"$2\" target \"$3\"",
                "acceptance-common-check",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-common.sh").path,
                bundleRoot.path,
                appDir.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("requires release-ready target notarization evidence"))
    }

    func testAcceptanceRequireReadyAppAuditForCollectionClearsStaleNotarizationEvidenceWhenSidecarMissingInSameMachineMode() throws {
        let workDir = try makeDirectory(named: "acceptance-common-stale-notarization")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: appDir, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "same_machine",
            "machine_count": 1
          },
          "roles": {},
          "evidence": {
            "notarization": {
              "target": {
                "collected_by": "stale",
                "output": "target.notarization.json",
                "status": "pass",
                "audit_status": "pass",
                "audit_readiness": "distribution_ready",
                "audit_pass_ready": true
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
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
        """.write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
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
        """.write(to: bundleRoot.appendingPathComponent("target.notarization.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_require_ready_app_audit_for_collection \"$2\" target \"$3\"",
                "acceptance-common-check",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-common.sh").path,
                bundleRoot.path,
                appDir.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))

        let data = try Data(contentsOf: bundleRoot.appendingPathComponent("meta.json"))
        let document = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let evidence = document["evidence"] as? [String: Any] ?? [:]
        let notarization = evidence["notarization"] as? [String: Any]
        XCTAssertNil(notarization?["target"])
    }

    func testAcceptanceRequireReadyAppAuditForCollectionRejectsMalformedNotarizationEvidenceInSameMachineMode() throws {
        let workDir = try makeDirectory(named: "acceptance-common-malformed-notarization")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let notaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: appDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "same_machine",
            "machine_count": 1
          },
          "roles": {},
          "evidence": {
            "notarization": {
              "target": {
                "collected_by": "stale",
                "output": "target.notarization.json",
                "status": "pass"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
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
        """.write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"bad"}"#.write(to: notaryDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"stale"}"#.write(to: bundleRoot.appendingPathComponent("target.notarization.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_require_ready_app_audit_for_collection \"$2\" target \"$3\"",
                "acceptance-common-check",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-common.sh").path,
                bundleRoot.path,
                appDir.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("malformed target notarization evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))

        let data = try Data(contentsOf: bundleRoot.appendingPathComponent("meta.json"))
        let document = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let evidence = document["evidence"] as? [String: Any] ?? [:]
        let notarization = evidence["notarization"] as? [String: Any]
        XCTAssertNil(notarization?["target"])
    }

    func testTargetServeWaitRejectsStaleReadyFileWithoutFreshRunnerEvidence() throws {
        let workDir = try makeDirectory(named: "target-serve-stale-ready")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "address": "127.0.0.1:40000",
          "verification_code": "123456",
          "mode": "pairing",
          "receiver_address": "https://127.0.0.1:41000",
          "receiver_routes": true,
          "push_network": true,
          "trusted": true,
          "transfer": true
        }
        """.write(to: bundleRoot.appendingPathComponent("target.ready.json"), atomically: true, encoding: .utf8)
        try "stale stderr".write(to: bundleRoot.appendingPathComponent("phase.stderr"), atomically: true, encoding: .utf8)
        try "stale stdout".write(to: bundleRoot.appendingPathComponent("phase.stdout"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                "sleep 5 & runner=$!; kill \"$runner\"; . \"$1\"; acceptance_two_machine_wait_for_target_ready \"$2\" /tmp/profile.json 1 \"$3\" \"$4\" \"$runner\"",
                "acceptance-two-machine-wait",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-two-machine.sh").path,
                bundleRoot.path,
                bundleRoot.appendingPathComponent("phase.stderr").path,
                bundleRoot.appendingPathComponent("phase.stdout").path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertNotEqual(result.exitCode, 0)
        XCTAssertTrue(result.stderr.contains("target serve exited before readiness was recorded"))
        XCTAssertTrue(result.stderr.contains("stale stderr"))
    }

    func testTwoMachineTargetServePropagatesEarlyServeFailureBeforeFreshReadiness() throws {
        let workDir = try makeDirectory(named: "target-serve-early-exit")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "same_machine",
            "machine_count": 1
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let fakeAppDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let resources = fakeAppDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resources.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        let fakeCLI = binDir.appendingPathComponent("supermover")
        try """
        #!/bin/sh
        case "$1 $2" in
          "version ")
            printf 'supermover 0.1.0-dev\\n'
            ;;
          "profile lint")
            exit 0
            ;;
          "serve --help")
            printf 'Usage: serve --help\\n-ready-file string\\n'
            ;;
          "serve --profile")
            printf 'serve failed before ready\\n' >&2
            exit 7
            ;;
          *)
            exit 0
            ;;
        esac
        """.write(to: fakeCLI, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeCLI.path)
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
        let fakeAuditScript = workDir.appendingPathComponent("fake-audit.sh")
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
        """.write(to: fakeAuditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditScript.path)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-serve",
                "--profile", "/tmp/profile.json",
                "--bundle-root", bundleRoot.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_APP_DIR": fakeAppDir.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
                "SUPERMOVER_ACCEPTANCE_COLLECTION_MODE": "same_machine",
                "SUPERMOVER_ACCEPTANCE_MACHINE_COUNT": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 7)
        XCTAssertTrue(result.stderr.contains("target serve exited before readiness was recorded"))
        XCTAssertTrue(result.stderr.contains("serve failed before ready"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.ready.phase-1.json").path))
    }

    func testTwoMachineTargetServeFailsEarlyOnBlockedAppAuditWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let appURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: appURL.path) else {
            throw XCTSkip("missing built app at \(appURL.path); run sh macos/script/build-app.sh first")
        }

        let workDir = try makeDirectory(named: "two-machine-script")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        let sourceRoot = workDir.appendingPathComponent("source-root", isDirectory: true)
        try FileManager.default.createDirectory(at: sourceRoot, withIntermediateDirectories: true)
        let profile = workDir.appendingPathComponent("target.profile.json")

        let initResult = try runProcess(
            executableURL: appURL.appendingPathComponent("Contents/Resources/bin/supermover"),
            arguments: [
                "profile", "init",
                "--profile", profile.path,
                "--source", sourceRoot.path,
                "--target", targetRoot.path,
                "--id", "two-machine-script",
                "--name", "Two Machine Script",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(initResult.stderr, "")

        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-serve",
                "--profile", profile.path,
                "--bundle-root", bundleRoot.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("installed-app acceptance requires install-ready target app audit before phase execution"))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.app-audit.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.version.txt").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.provenance.json").path))
    }

    func testTwoMachinePhaseWrappersHonorPerMachineAppDirOverrides() throws {
        let workDir = try makeDirectory(named: "two-machine-role-specific-app-dirs")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)

        let sourceAppDir = workDir.appendingPathComponent("Source.app", isDirectory: true)
        let targetAppDir = workDir.appendingPathComponent("Target.app", isDirectory: true)

        func writeFakeApp(at appDir: URL, version: String, commit: String, buildProfile: String) throws {
            let resources = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
            let binDir = resources.appendingPathComponent("bin", isDirectory: true)
            try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)

            let cliURL = binDir.appendingPathComponent("supermover")
            try """
            #!/bin/sh
            cmd1=${1:-}
            cmd2=${2:-}
            cmd3=${3:-}
            case "$cmd1" in
              version)
                printf '\(version)\\n'
                ;;
              discover)
                case "$cmd2" in
                  browse)
                    if [ "$cmd3" = "--help" ]; then
                      printf 'Usage: discover browse --help\\n-timeout string\\n'
                    else
                      cat <<'EOF'
            {
              "trusted": false,
              "candidate_count": 1,
              "candidates": [
                {
                  "id": "candidate-1"
                }
              ]
            }
            EOF
                    fi
                    ;;
                  advertise)
                    if [ "$cmd3" = "--help" ]; then
                      printf 'Usage: discover advertise --help\\n-profile string\\n'
                    else
                      cat <<'EOF'
            {
              "status": "advertised",
              "trusted": false,
              "listen": "127.0.0.1:41000",
              "destination": "127.0.0.1:41001",
              "capability_flags": ["pairing"]
            }
            EOF
                    fi
                    ;;
                  *)
                    exit 9
                    ;;
                esac
                ;;
              *)
                exit 9
                ;;
            esac
            """.write(to: cliURL, atomically: true, encoding: .utf8)
            try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cliURL.path)

            try """
            {
              "schema": "supermover.macos.provenance.v1",
              "git_commit": "\(commit)",
              "cli_version": "\(version)",
              "cli_relative_path": "Contents/Resources/bin/supermover",
              "build_profile": "\(buildProfile)",
              "signing": "developer_id"
            }
            """.write(to: resources.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        }

        try writeFakeApp(
            at: sourceAppDir,
            version: "supermover source-cli",
            commit: "source123456",
            buildProfile: "source-app"
        )
        try writeFakeApp(
            at: targetAppDir,
            version: "supermover target-cli",
            commit: "target123456",
            buildProfile: "target-app"
        )

        let fakeAuditScript = workDir.appendingPathComponent("fake-audit.sh")
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
        """.write(to: fakeAuditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditScript.path)

        let targetProfile = workDir.appendingPathComponent("target.profile.json")
        try "{}\n".write(to: targetProfile, atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let sharedEnvironment = [
            "SUPERMOVER_ACCEPTANCE_APP_DIR": workDir.appendingPathComponent("missing.app").path,
            "SUPERMOVER_ACCEPTANCE_SOURCE_APP_DIR": sourceAppDir.path,
            "SUPERMOVER_ACCEPTANCE_TARGET_APP_DIR": targetAppDir.path,
            "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
            "SUPERMOVER_ACCEPTANCE_COLLECTION_MODE": "same_machine",
            "SUPERMOVER_ACCEPTANCE_MACHINE_COUNT": "1",
        ]

        let advertise = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-advertise",
                "--profile", targetProfile.path,
                "--bundle-root", bundleRoot.path,
                "--listen", "127.0.0.1:0",
                "--dest", "127.0.0.1:41001",
                "--duration", "1s",
                "--interval", "200ms",
            ],
            environment: sharedEnvironment,
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(advertise.stderr, "")

        let browse = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "source-browse",
                "--bundle-root", bundleRoot.path,
                "--listen", "127.0.0.1:41002",
                "--timeout", "600ms",
            ],
            environment: sharedEnvironment,
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(browse.stderr, "")

        XCTAssertEqual(
            try String(contentsOf: bundleRoot.appendingPathComponent("target.version.txt")).trimmingCharacters(in: .whitespacesAndNewlines),
            "supermover target-cli"
        )
        XCTAssertEqual(
            try String(contentsOf: bundleRoot.appendingPathComponent("source.version.txt")).trimmingCharacters(in: .whitespacesAndNewlines),
            "supermover source-cli"
        )

        let targetProvenance = try String(contentsOf: bundleRoot.appendingPathComponent("target.provenance.json"))
        XCTAssertTrue(targetProvenance.contains("\"git_commit\": \"target123456\""))
        XCTAssertTrue(targetProvenance.contains("\"build_profile\": \"target-app\""))

        let sourceProvenance = try String(contentsOf: bundleRoot.appendingPathComponent("source.provenance.json"))
        XCTAssertTrue(sourceProvenance.contains("\"git_commit\": \"source123456\""))
        XCTAssertTrue(sourceProvenance.contains("\"build_profile\": \"source-app\""))
    }

    func testTwoMachineSourcePairWritesBundleRelativeReceiptPath() throws {
        let workDir = try makeDirectory(named: "two-machine-source-pair-relative-receipt")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "same_machine",
            "machine_count": 1
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "address": "127.0.0.1:39395",
          "verification_code": "123456",
          "mode": "pairing-only"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.ready.json"), atomically: true, encoding: .utf8)

        let sourceAppDir = workDir.appendingPathComponent("Source.app", isDirectory: true)
        let resources = sourceAppDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resources.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        let cliURL = binDir.appendingPathComponent("supermover")
        try """
        #!/bin/sh
        cmd1=${1:-}
        cmd2=${2:-}
        cmd3=${3:-}
        case "$cmd1 $cmd2" in
          "version ")
            printf 'supermover source-cli\\n'
            ;;
          "profile lint")
            exit 0
            ;;
          "pair --help")
            printf 'Usage: pair --help\\n-receipt-out string\\n'
            ;;
          "pair --profile")
            receipt_dir=""
            while [ "$#" -gt 0 ]; do
              case "$1" in
                --receipt-out)
                  receipt_dir=$2
                  shift 2
                  ;;
                *)
                  shift 1
                  ;;
              esac
            done
            mkdir -p "$receipt_dir"
            cat <<'EOF' > "$receipt_dir/pair-1.json"
        {"version":1,"id":"pair-1","profile_id":"profile-src","target_id":"target-1","source_device_id":"src-spki","target_device_id":"dst-spki","device_public_key":"dst-spki","method":"sas","verified_at":"2026-06-04T00:00:00Z","verification_hash":"hash-1","protocol_version":"supermover/v1"}
        EOF
            printf 'pair ok\\n'
            ;;
          *)
            exit 9
            ;;
        esac
        """.write(to: cliURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cliURL.path)
        try """
        {
          "schema": "supermover.macos.provenance.v1",
          "git_commit": "source123456",
          "cli_version": "supermover source-cli",
          "cli_relative_path": "Contents/Resources/bin/supermover",
          "build_profile": "source-app",
          "signing": "developer_id"
        }
        """.write(to: resources.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)

        let fakeAuditScript = workDir.appendingPathComponent("fake-audit.sh")
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
        """.write(to: fakeAuditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditScript.path)

        let sourceProfile = workDir.appendingPathComponent("source.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1"
          }
        }
        """.write(to: sourceProfile, atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "source-pair",
                "--profile", sourceProfile.path,
                "--bundle-root", bundleRoot.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_APP_DIR": sourceAppDir.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
                "SUPERMOVER_ACCEPTANCE_COLLECTION_MODE": "same_machine",
                "SUPERMOVER_ACCEPTANCE_MACHINE_COUNT": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.stderr, "")
        let sourcePairData = try Data(contentsOf: bundleRoot.appendingPathComponent("source.pair.json"))
        let sourcePair = try XCTUnwrap(JSONSerialization.jsonObject(with: sourcePairData) as? [String: Any])
        XCTAssertEqual(sourcePair["receipt_path"] as? String, "exported-receipts/pair-1.json")
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("exported-receipts/pair-1.json").path
            )
        )
    }

    func testAppAuthoredTargetReadySupportsShellSourcePairWithoutExplicitAddressFlags() throws {
        let workDir = try makeDirectory(named: "app-authored-target-ready-source-pair")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "same_machine",
            "machine_count": 1
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try AcceptanceBundleArtifactWriter().writeServePhase(
            .init(
                bundleRootURL: bundleRoot,
                profilePath: "/tmp/target.profile.json",
                phase: 1,
                readiness: ServeReadinessSnapshot(
                    address: "127.0.0.1:39395",
                    verification_code: "123456",
                    mode: "pairing",
                    receiver_address: nil,
                    receiver_routes: nil,
                    push_network: nil,
                    trusted: true,
                    transfer: true,
                    expires_at: nil
                )
            )
        )

        let sourceAppDir = workDir.appendingPathComponent("Source.app", isDirectory: true)
        let resources = sourceAppDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resources.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        let cliURL = binDir.appendingPathComponent("supermover")
        try """
        #!/bin/sh
        cmd1=${1:-}
        cmd2=${2:-}
        case "$cmd1 $cmd2" in
          "version ")
            printf 'supermover source-cli\\n'
            ;;
          "profile lint")
            exit 0
            ;;
          "pair --help")
            printf 'Usage: pair --help\\n-receipt-out string\\n'
            ;;
          "pair --profile")
            receipt_dir=""
            while [ "$#" -gt 0 ]; do
              case "$1" in
                --receipt-out)
                  receipt_dir=$2
                  shift 2
                  ;;
                *)
                  shift 1
                  ;;
              esac
            done
            mkdir -p "$receipt_dir"
            cat <<'EOF' > "$receipt_dir/pair-1.json"
        {"version":1,"id":"pair-1","profile_id":"profile-1","target_id":"target-1","source_device_id":"source-device","target_device_id":"target-device","device_public_key":"target-device","method":"sas","verified_at":"2026-06-04T00:00:00Z","verification_hash":"hash-1","protocol_version":"v1"}
        EOF
            printf 'pair ok\\n'
            ;;
          *)
            exit 9
            ;;
        esac
        """.write(to: cliURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cliURL.path)
        try """
        {
          "schema": "supermover.macos.provenance.v1",
          "git_commit": "source123456",
          "cli_version": "supermover source-cli",
          "cli_relative_path": "Contents/Resources/bin/supermover",
          "build_profile": "source-app",
          "signing": "developer_id"
        }
        """.write(to: resources.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)

        let fakeAuditScript = workDir.appendingPathComponent("fake-audit.sh")
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
        """.write(to: fakeAuditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditScript.path)

        let sourceProfile = workDir.appendingPathComponent("source.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1"
          }
        }
        """.write(to: sourceProfile, atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "source-pair",
                "--profile", sourceProfile.path,
                "--bundle-root", bundleRoot.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_APP_DIR": sourceAppDir.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
                "SUPERMOVER_ACCEPTANCE_COLLECTION_MODE": "same_machine",
                "SUPERMOVER_ACCEPTANCE_MACHINE_COUNT": "1",
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.stderr, "")
        let sourcePairData = try Data(contentsOf: bundleRoot.appendingPathComponent("source.pair.json"))
        let sourcePair = try XCTUnwrap(JSONSerialization.jsonObject(with: sourcePairData) as? [String: Any])
        XCTAssertEqual(sourcePair["target_address"] as? String, "127.0.0.1:39395")
        XCTAssertEqual(sourcePair["verification_code"] as? String, "123456")
        XCTAssertEqual(sourcePair["receipt_path"] as? String, "exported-receipts/pair-1.json")
    }

    func testTwoMachineMergeBundleFailsClosedOnCollectionConflict() throws {
        let workDir = try makeDirectory(named: "two-machine-merge-bundle-conflict")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: destinationBundle, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: incomingBundle, withIntermediateDirectories: true)

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "same_machine",
            "machine_count": 1
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: destinationBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: incomingBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "merge-bundle",
                "--bundle-root", destinationBundle.path,
                "--incoming-bundle-root", incomingBundle.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("acceptance bundle merge conflict"))
        let mergedMeta = try String(contentsOf: destinationBundle.appendingPathComponent("meta.json"))
        XCTAssertTrue(mergedMeta.contains("\"mode\": \"same_machine\""))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: destinationBundle.appendingPathComponent("source.pair.json").path
            )
        )
    }

    func testTwoMachineMergeBundleFailsClosedOnArtifactConflict() throws {
        let workDir = try makeDirectory(named: "two-machine-merge-bundle-artifact-conflict")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: destinationBundle, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: incomingBundle, withIntermediateDirectories: true)

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {},
          "evidence": {}
        }
        """.write(to: destinationBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {},
          "evidence": {}
        }
        """.write(to: incomingBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try "destination\n".write(to: destinationBundle.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)
        try "incoming\n".write(to: incomingBundle.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "merge-bundle",
                "--bundle-root", destinationBundle.path,
                "--incoming-bundle-root", incomingBundle.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("acceptance bundle merge conflict"))
        let destinationPayload = try String(contentsOf: destinationBundle.appendingPathComponent("source.pair.txt"))
        XCTAssertEqual(destinationPayload, "destination\n")
    }

    func testTwoMachineMergeBundleArtifactConflictDoesNotPublishNovelIncomingArtifacts() throws {
        let workDir = try makeDirectory(named: "two-machine-merge-bundle-artifact-conflict-atomic")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try writeMergeBundleMeta(at: destinationBundle)
        try writeMergeBundleMeta(at: incomingBundle)
        let novelDirectory = incomingBundle.appendingPathComponent("000-dir", isDirectory: true)
        try FileManager.default.createDirectory(at: novelDirectory, withIntermediateDirectories: true)
        try "novel incoming\n".write(to: novelDirectory.appendingPathComponent("novel.txt"), atomically: true, encoding: .utf8)
        try "destination\n".write(to: destinationBundle.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)
        try "incoming\n".write(to: incomingBundle.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)

        let result = try runMergeBundleAllowFailure(
            destinationBundle: destinationBundle,
            incomingBundle: incomingBundle
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("acceptance bundle merge conflict"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: destinationBundle.appendingPathComponent("000-dir").path
            )
        )
        let destinationPayload = try String(contentsOf: destinationBundle.appendingPathComponent("source.pair.txt"))
        XCTAssertEqual(destinationPayload, "destination\n")
    }

    func testTwoMachineMergeBundleFailsClosedOnMetaConflict() throws {
        let workDir = try makeDirectory(named: "two-machine-merge-bundle-meta-conflict")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: destinationBundle, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: incomingBundle, withIntermediateDirectories: true)

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {
            "source_pair": { "machine_id": "source-machine", "machine_label": "source" }
          },
          "evidence": {}
        }
        """.write(to: destinationBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {
            "source_pair": { "machine_id": "different-source-machine", "machine_label": "other-source" }
          },
          "evidence": {}
        }
        """.write(to: incomingBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "merge-bundle",
                "--bundle-root", destinationBundle.path,
                "--incoming-bundle-root", incomingBundle.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("meta.json contains overlapping non-identical values"))
        let mergedMeta = try String(contentsOf: destinationBundle.appendingPathComponent("meta.json"))
        XCTAssertTrue(mergedMeta.contains("source-machine"))
        XCTAssertFalse(mergedMeta.contains("different-source-machine"))
    }

    func testTwoMachineMergeBundleMetaConflictDoesNotPublishNovelIncomingArtifacts() throws {
        let workDir = try makeDirectory(named: "two-machine-merge-bundle-meta-conflict-atomic")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: destinationBundle, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: incomingBundle, withIntermediateDirectories: true)

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {
            "source_pair": { "machine_id": "source-machine", "machine_label": "source" }
          },
          "evidence": {}
        }
        """.write(to: destinationBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {
            "source_pair": { "machine_id": "different-source-machine", "machine_label": "other-source" }
          },
          "evidence": {}
        }
        """.write(to: incomingBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try "novel incoming\n".write(to: incomingBundle.appendingPathComponent("000-novel.txt"), atomically: true, encoding: .utf8)

        let result = try runMergeBundleAllowFailure(
            destinationBundle: destinationBundle,
            incomingBundle: incomingBundle
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("meta.json contains overlapping non-identical values"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: destinationBundle.appendingPathComponent("000-novel.txt").path
            )
        )
        let mergedMeta = try String(contentsOf: destinationBundle.appendingPathComponent("meta.json"))
        XCTAssertTrue(mergedMeta.contains("source-machine"))
        XCTAssertFalse(mergedMeta.contains("different-source-machine"))
    }

    func testTwoMachineMergeBundleTargetReadyConflictDoesNotPublishMetaOrNovelArtifacts() throws {
        let workDir = try makeDirectory(named: "two-machine-merge-bundle-target-ready-conflict-atomic")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: destinationBundle, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: incomingBundle, withIntermediateDirectories: true)

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {},
          "evidence": {
            "target_serve_phases": [{ "phase": 1 }]
          }
        }
        """.write(to: destinationBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {
            "target": { "machine_id": "target-machine", "machine_label": "target" }
          },
          "evidence": {
            "target_serve_phases": [{ "phase": 1 }]
          }
        }
        """.write(to: incomingBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try #"{"phase":1,"ready":"destination"}"#.write(
            to: destinationBundle.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )
        try #"{"phase":1,"ready":"incoming"}"#.write(
            to: incomingBundle.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )
        try "novel incoming\n".write(to: incomingBundle.appendingPathComponent("000-novel.txt"), atomically: true, encoding: .utf8)

        let result = try runMergeBundleAllowFailure(
            destinationBundle: destinationBundle,
            incomingBundle: incomingBundle
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("target.ready.json differs"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: destinationBundle.appendingPathComponent("000-novel.txt").path
            )
        )
        let mergedMeta = try String(contentsOf: destinationBundle.appendingPathComponent("meta.json"))
        XCTAssertFalse(mergedMeta.contains("target-machine"))
        let targetReady = try String(contentsOf: destinationBundle.appendingPathComponent("target.ready.json"))
        XCTAssertTrue(targetReady.contains("destination"))
        XCTAssertFalse(targetReady.contains("incoming"))
    }

    func testTwoMachineMergeBundleRejectsDestinationSymlinkDirectoryBeforePublish() throws {
        let workDir = try makeDirectory(named: "two-machine-merge-bundle-destination-symlink-dir")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try writeMergeBundleMeta(at: destinationBundle)
        try writeMergeBundleMeta(at: incomingBundle)
        let outsideDirectory = workDir.appendingPathComponent("outside-dir", isDirectory: true)
        try FileManager.default.createDirectory(at: outsideDirectory, withIntermediateDirectories: true)
        do {
            try FileManager.default.createSymbolicLink(
                at: destinationBundle.appendingPathComponent("incoming-dir"),
                withDestinationURL: outsideDirectory
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }
        let incomingDirectory = incomingBundle.appendingPathComponent("incoming-dir", isDirectory: true)
        try FileManager.default.createDirectory(at: incomingDirectory, withIntermediateDirectories: true)
        try "novel incoming\n".write(to: incomingDirectory.appendingPathComponent("novel.txt"), atomically: true, encoding: .utf8)

        let result = try runMergeBundleAllowFailure(
            destinationBundle: destinationBundle,
            incomingBundle: incomingBundle
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("acceptance bundle merge conflict"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: outsideDirectory.appendingPathComponent("novel.txt").path
            )
        )
    }

    func testTwoMachineMergeBundleRollsBackNovelArtifactsOnPublishCopyFailure() throws {
        let workDir = try makeDirectory(named: "two-machine-merge-bundle-publish-copy-failure")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try writeMergeBundleMeta(at: destinationBundle)
        try FileManager.default.createDirectory(at: incomingBundle, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {
            "target": { "machine_id": "target-machine", "machine_label": "target" }
          },
          "evidence": {}
        }
        """.write(to: incomingBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        let nestedDirectory = incomingBundle.appendingPathComponent("000-dir", isDirectory: true)
        try FileManager.default.createDirectory(at: nestedDirectory, withIntermediateDirectories: true)
        try "nested novel\n".write(to: nestedDirectory.appendingPathComponent("nested.txt"), atomically: true, encoding: .utf8)
        try "first novel\n".write(to: incomingBundle.appendingPathComponent("000-novel.txt"), atomically: true, encoding: .utf8)
        try "fail novel\n".write(to: incomingBundle.appendingPathComponent("001-fail.txt"), atomically: true, encoding: .utf8)

        let binDir = workDir.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        let fakeCP = binDir.appendingPathComponent("cp")
        try """
        #!/bin/sh
        case "$2" in
          *001-fail.txt)
            printf 'forced publish copy failure\\n' >&2
            exit 1
            ;;
        esac
        exec /bin/cp "$@"
        """.write(to: fakeCP, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeCP.path)
        let inheritedPath = ProcessInfo.processInfo.environment["PATH"] ?? "/usr/bin:/bin:/usr/sbin:/sbin"

        let result = try runMergeBundleAllowFailure(
            destinationBundle: destinationBundle,
            incomingBundle: incomingBundle,
            environment: ["PATH": "\(binDir.path):\(inheritedPath)"]
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("failed to publish merged acceptance artifact"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: destinationBundle.appendingPathComponent("000-dir").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: destinationBundle.appendingPathComponent("000-novel.txt").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: destinationBundle.appendingPathComponent("001-fail.txt").path))
        let mergedMeta = try String(contentsOf: destinationBundle.appendingPathComponent("meta.json"))
        XCTAssertFalse(mergedMeta.contains("target-machine"))
    }

    func testTwoMachineMergeBundleRejectsSymlinkedIncomingEntriesBeforePublish() throws {
        let fixture = try makeUnsafeMergeBundleFixture(named: "two-machine-merge-bundle-symlink-entry") { incomingBundle in
            do {
                try FileManager.default.createSymbolicLink(
                    at: incomingBundle.appendingPathComponent("000-unsafe.json"),
                    withDestinationURL: URL(fileURLWithPath: "/tmp/supermover-merge-escape.json")
                )
            } catch {
                throw XCTSkip("symlink unavailable: \(error)")
            }
        }
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }

        let result = try runMergeBundleAllowFailure(
            destinationBundle: fixture.destinationBundle,
            incomingBundle: fixture.incomingBundle
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe acceptance bundle archive entry"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.destinationBundle.appendingPathComponent("safe-before-unsafe.txt").path
            )
        )
    }

    func testTwoMachineMergeBundleRejectsSymlinkedIncomingMetaBeforeParsing() throws {
        let workDir = try makeDirectory(named: "two-machine-merge-bundle-symlink-meta")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try writeMergeBundleMeta(at: destinationBundle)
        try writeMergeBundleMeta(at: incomingBundle)
        try FileManager.default.removeItem(at: incomingBundle.appendingPathComponent("meta.json"))
        let outsideMeta = workDir.appendingPathComponent("outside-meta.json")
        try "not json\n".write(to: outsideMeta, atomically: true, encoding: .utf8)
        do {
            try FileManager.default.createSymbolicLink(
                at: incomingBundle.appendingPathComponent("meta.json"),
                withDestinationURL: outsideMeta
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        let result = try runMergeBundleAllowFailure(
            destinationBundle: destinationBundle,
            incomingBundle: incomingBundle
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe acceptance bundle archive entry"))
        XCTAssertFalse(result.stderr.contains("parse error"))
    }

    func testTwoMachineMergeBundleRejectsHardlinkedIncomingEntriesBeforePublish() throws {
        let fixture = try makeUnsafeMergeBundleFixture(named: "two-machine-merge-bundle-hardlink-entry") { incomingBundle in
            let original = incomingBundle.appendingPathComponent("unsafe.original.json")
            let hardlink = incomingBundle.appendingPathComponent("000-unsafe.json")
            try #"{"pair":"original"}"#.write(to: original, atomically: true, encoding: .utf8)
            do {
                try FileManager.default.linkItem(at: original, to: hardlink)
            } catch {
                throw XCTSkip("hardlink unavailable: \(error)")
            }
        }
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }

        let result = try runMergeBundleAllowFailure(
            destinationBundle: fixture.destinationBundle,
            incomingBundle: fixture.incomingBundle
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe acceptance bundle archive entry"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.destinationBundle.appendingPathComponent("safe-before-unsafe.txt").path
            )
        )
    }

    func testTwoMachineMergeBundleRejectsSpecialIncomingEntriesBeforePublish() throws {
        let fifoName = "000-unsafe.json"
        let fixture = try makeUnsafeMergeBundleFixture(named: "two-machine-merge-bundle-special-entry") { incomingBundle in
            let fifo = incomingBundle.appendingPathComponent(fifoName)
            let result = try runProcessAllowFailure(
                executableURL: URL(fileURLWithPath: "/usr/bin/mkfifo"),
                arguments: [fifo.path],
                environment: [:],
                currentDirectoryURL: incomingBundle
            )
            if result.exitCode != 0 {
                throw XCTSkip("mkfifo unavailable: \(result.stderr)")
            }
        }
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }

        let result = try runMergeBundleWithFIFOWriterAllowFailure(
            destinationBundle: fixture.destinationBundle,
            incomingBundle: fixture.incomingBundle,
            fifo: fixture.incomingBundle.appendingPathComponent(fifoName)
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe acceptance bundle archive entry"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.destinationBundle.appendingPathComponent("safe-before-unsafe.txt").path
            )
        )
    }

    private func writeMergeBundleMeta(at bundleRoot: URL) throws {
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
    }

    private func runMergeBundleAllowFailure(
        destinationBundle: URL,
        incomingBundle: URL,
        environment: [String: String] = [:]
    ) throws -> ProcessResult {
        let repoRoot = repoRootURL()
        return try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "merge-bundle",
                "--bundle-root", destinationBundle.path,
                "--incoming-bundle-root", incomingBundle.path,
            ],
            environment: environment,
            currentDirectoryURL: repoRoot
        )
    }

    private func runMergeBundleWithFIFOWriterAllowFailure(
        destinationBundle: URL,
        incomingBundle: URL,
        fifo: URL
    ) throws -> ProcessResult {
        let repoRoot = repoRootURL()
        return try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                """
                fifo=$1
                shift
                (printf 'fifo incoming\\n' > "$fifo") &
                writer=$!
                "$@"
                status=$?
                kill "$writer" 2>/dev/null || true
                wait "$writer" 2>/dev/null || true
                exit "$status"
                """,
                "merge-bundle-fifo-wrapper",
                fifo.path,
                "/bin/sh",
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "merge-bundle",
                "--bundle-root", destinationBundle.path,
                "--incoming-bundle-root", incomingBundle.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
    }

    private func makeUnsafeMergeBundleFixture(
        named name: String,
        addUnsafeEntry: (URL) throws -> Void
    ) throws -> (workDir: URL, destinationBundle: URL, incomingBundle: URL) {
        let workDir = try makeDirectory(named: name)
        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try writeMergeBundleMeta(at: destinationBundle)
        try writeMergeBundleMeta(at: incomingBundle)
        try "safe incoming\n".write(
            to: incomingBundle.appendingPathComponent("safe-before-unsafe.txt"),
            atomically: true,
            encoding: .utf8
        )
        try addUnsafeEntry(incomingBundle)
        return (workDir, destinationBundle, incomingBundle)
    }

    func testTwoMachineMergeBundleRecomputesWorkflowSummaryInsteadOfConflictingOnDerivedArtifact() throws {
        let workDir = try makeDirectory(named: "two-machine-merge-bundle-workflow-summary")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: destinationBundle, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: incomingBundle, withIntermediateDirectories: true)

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {},
          "evidence": {
            "workflow_summary": {
              "output": "workflow.summary.json",
              "default": {
                "schema": "supermover.acceptance.workflow_status.v1",
                "next_actions": [{ "machine": "source", "step": "stale-destination" }],
                "steps": []
              },
              "require_operator_evidence": {
                "schema": "supermover.acceptance.workflow_status.v1",
                "next_actions": [{ "machine": "source", "step": "stale-destination" }],
                "steps": []
              }
            }
          }
        }
        """.write(to: destinationBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.workflow_summary.v1",
          "default": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "next_actions": [{ "machine": "source", "step": "stale-destination" }],
            "steps": []
          },
          "require_operator_evidence": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "next_actions": [{ "machine": "source", "step": "stale-destination" }],
            "steps": []
          }
        }
        """.write(to: destinationBundle.appendingPathComponent("workflow.summary.json"), atomically: true, encoding: .utf8)

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": { "mode": "two_machine", "machine_count": 2 },
          "roles": {},
          "evidence": {
            "workflow_summary": {
              "output": "workflow.summary.json",
              "default": {
                "schema": "supermover.acceptance.workflow_status.v1",
                "next_actions": [{ "machine": "target", "step": "stale-incoming" }],
                "steps": []
              },
              "require_operator_evidence": {
                "schema": "supermover.acceptance.workflow_status.v1",
                "next_actions": [{ "machine": "target", "step": "stale-incoming" }],
                "steps": []
              }
            }
          }
        }
        """.write(to: incomingBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.workflow_summary.v1",
          "default": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "next_actions": [{ "machine": "target", "step": "stale-incoming" }],
            "steps": []
          },
          "require_operator_evidence": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "next_actions": [{ "machine": "target", "step": "stale-incoming" }],
            "steps": []
          }
        }
        """.write(to: incomingBundle.appendingPathComponent("workflow.summary.json"), atomically: true, encoding: .utf8)
        try "pair ok\n".write(to: incomingBundle.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "merge-bundle",
                "--bundle-root", destinationBundle.path,
                "--incoming-bundle-root", incomingBundle.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        XCTAssertTrue(FileManager.default.fileExists(atPath: destinationBundle.appendingPathComponent("source.pair.txt").path))

        let mergedMetaData = try Data(contentsOf: destinationBundle.appendingPathComponent("meta.json"))
        let mergedMeta = try XCTUnwrap(JSONSerialization.jsonObject(with: mergedMetaData) as? [String: Any])
        let mergedEvidence = mergedMeta["evidence"] as? [String: Any] ?? [:]
        let mergedWorkflowSummary = try XCTUnwrap(mergedEvidence["workflow_summary"] as? [String: Any])
        XCTAssertEqual(mergedWorkflowSummary["output"] as? String, "workflow.summary.json")

        let workflowSummaryText = try String(contentsOf: destinationBundle.appendingPathComponent("workflow.summary.json"))
        XCTAssertFalse(workflowSummaryText.contains("stale-destination"))
        XCTAssertFalse(workflowSummaryText.contains("stale-incoming"))
    }

    func testTwoMachineTargetImportUsesMergedBundleRelativeReceiptPath() throws {
        let workDir = try makeDirectory(named: "two-machine-target-import-merged-bundle")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let sourceBundle = workDir.appendingPathComponent("source-bundle", isDirectory: true)
        let targetBundle = workDir.appendingPathComponent("target-bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: sourceBundle, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetBundle, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(
            at: sourceBundle.appendingPathComponent("exported-receipts", isDirectory: true),
            withIntermediateDirectories: true
        )

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
              "machine_id": "source-machine",
              "machine_label": "source"
            }
          },
          "evidence": {
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            }
          }
        }
        """.write(to: sourceBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "profile": "/tmp/source.profile.json",
          "target_address": "127.0.0.1:39395",
          "verification_code": "123456",
          "pairing_receipt_id": "pair-1",
          "receipt_path": "exported-receipts/pair-1.json"
        }
        """.write(to: sourceBundle.appendingPathComponent("source.pair.json"), atomically: true, encoding: .utf8)
        try "pair ok\n".write(to: sourceBundle.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.pairingReceiptJSON().write(
            to: sourceBundle.appendingPathComponent("exported-receipts/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "target_import": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "target-machine",
              "machine_label": "target"
            }
          },
          "evidence": {}
        }
        """.write(to: targetBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let targetAppDir = workDir.appendingPathComponent("Target.app", isDirectory: true)
        let resources = targetAppDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resources.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        let capturedReceiptPath = workDir.appendingPathComponent("captured-receipt-path.txt")
        let cliURL = binDir.appendingPathComponent("supermover")
        try """
        #!/bin/sh
        cmd1=${1:-}
        cmd2=${2:-}
        cmd3=${3:-}
        case "$cmd1 $cmd2" in
          "version ")
            printf 'supermover target-cli\\n'
            ;;
          "profile adopt-pairing")
            if [ "$cmd3" = "--help" ]; then
              printf 'Usage: profile adopt-pairing --help\\n-receipt-file string\\n'
            else
              receipt_file=""
              while [ "$#" -gt 0 ]; do
                case "$1" in
                  --receipt-file)
                    receipt_file=$2
                    shift 2
                    ;;
                  *)
                    shift 1
                    ;;
                esac
              done
              printf '%s\\n' "$receipt_file" > "\(capturedReceiptPath.path)"
              printf 'receipt adopted\\n'
            fi
            ;;
          *)
            exit 9
            ;;
        esac
        """.write(to: cliURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cliURL.path)

        let targetNotaryDir = workDir.appendingPathComponent("Target.app.notary", isDirectory: true)
        try FileManager.default.createDirectory(at: targetNotaryDir, withIntermediateDirectories: true)
        let provenanceManifest = try writeCurrentAppSidecarReleaseEvidence(
            appDir: targetAppDir,
            sidecarDir: targetNotaryDir,
            cliVersion: "supermover target-cli"
        )

        let fakeAuditScript = workDir.appendingPathComponent("fake-audit.sh")
        try makeCurrentAuditScript(
            appPath: targetAppDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: fakeAuditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditScript.path)

        let targetProfile = workDir.appendingPathComponent("target.profile.json")
        try "{}\n".write(to: targetProfile, atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let merge = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "merge-bundle",
                "--bundle-root", targetBundle.path,
                "--incoming-bundle-root", sourceBundle.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(merge.stderr, "")
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: targetBundle.appendingPathComponent("exported-receipts/pair-1.json").path
            )
        )

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-import",
                "--profile", targetProfile.path,
                "--bundle-root", targetBundle.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_APP_DIR": targetAppDir.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.stderr, "")
        XCTAssertEqual(
            try String(contentsOf: capturedReceiptPath).trimmingCharacters(in: .whitespacesAndNewlines),
            targetBundle.appendingPathComponent("exported-receipts/pair-1.json").path
        )
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: targetBundle.appendingPathComponent("target.adopt-pairing.txt").path
            )
        )
    }

    func testAppAuthoredSourcePairUsesBundleRelativeReceiptPathForShellTargetImport() throws {
        let workDir = try makeDirectory(named: "app-authored-source-pair-target-import")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let sourceBundle = workDir.appendingPathComponent("source-bundle", isDirectory: true)
        let targetBundle = workDir.appendingPathComponent("target-bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: sourceBundle, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetBundle, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "machine_count": 2,
            "mode": "two_machine"
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: sourceBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "machine_count": 2,
            "mode": "two_machine"
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: targetBundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let localReceiptURL = try writeLocalPairingReceipt(id: "pair-1", under: workDir)
        try AcceptanceBundleArtifactWriter().writeSourcePair(
            .init(
                bundleRootURL: sourceBundle,
                profilePath: "/tmp/source.profile.json",
                targetAddress: "127.0.0.1:39395",
                verificationCode: "123456",
                localPairingReceiptPath: localReceiptURL.path,
                pairingReceiptID: "pair-1",
                pairStdout: "pair ok"
            )
        )

        let targetAppDir = workDir.appendingPathComponent("Target.app", isDirectory: true)
        let resources = targetAppDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resources.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        let capturedReceiptPath = workDir.appendingPathComponent("captured-receipt-path.txt")
        let cliURL = binDir.appendingPathComponent("supermover")
        try """
        #!/bin/sh
        cmd1=${1:-}
        cmd2=${2:-}
        cmd3=${3:-}
        case "$cmd1 $cmd2" in
          "version ")
            printf 'supermover target-cli\\n'
            ;;
          "profile adopt-pairing")
            if [ "$cmd3" = "--help" ]; then
              printf 'Usage: profile adopt-pairing --help\\n-receipt-file string\\n'
            else
              receipt_file=""
              while [ "$#" -gt 0 ]; do
                case "$1" in
                  --receipt-file)
                    receipt_file=$2
                    shift 2
                    ;;
                  *)
                    shift 1
                    ;;
                esac
              done
              printf '%s\\n' "$receipt_file" > "\(capturedReceiptPath.path)"
              printf 'receipt adopted\\n'
            fi
            ;;
          *)
            exit 9
            ;;
        esac
        """.write(to: cliURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cliURL.path)

        let targetNotaryDir = workDir.appendingPathComponent("Target.app.notary", isDirectory: true)
        try FileManager.default.createDirectory(at: targetNotaryDir, withIntermediateDirectories: true)
        let provenanceManifest = try writeCurrentAppSidecarReleaseEvidence(
            appDir: targetAppDir,
            sidecarDir: targetNotaryDir,
            cliVersion: "supermover target-cli"
        )

        let fakeAuditScript = workDir.appendingPathComponent("fake-audit.sh")
        try makeCurrentAuditScript(
            appPath: targetAppDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: fakeAuditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditScript.path)

        let targetProfile = workDir.appendingPathComponent("target.profile.json")
        try "{}\n".write(to: targetProfile, atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "merge-bundle",
                "--bundle-root", targetBundle.path,
                "--incoming-bundle-root", sourceBundle.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-import",
                "--profile", targetProfile.path,
                "--bundle-root", targetBundle.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_APP_DIR": targetAppDir.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.stderr, "")
        XCTAssertEqual(
            try String(contentsOf: capturedReceiptPath).trimmingCharacters(in: .whitespacesAndNewlines),
            targetBundle.appendingPathComponent("exported-receipts/pair-1.json").path
        )
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: targetBundle.appendingPathComponent("target.adopt-pairing.txt").path
            )
        )
    }

    func testTwoMachinePackAndUnpackBundleRoundTripsEvidence() throws {
        let workDir = try makeDirectory(named: "two-machine-pack-unpack-bundle")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(
            at: bundleRoot.appendingPathComponent("exported-receipts", isDirectory: true),
            withIntermediateDirectories: true
        )
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
              "machine_id": "source-machine",
              "machine_label": "source"
            }
          },
          "evidence": {
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "profile": "/tmp/source.profile.json",
          "target_address": "127.0.0.1:39395",
          "verification_code": "123456",
          "pairing_receipt_id": "pair-1",
          "receipt_path": "exported-receipts/pair-1.json"
        }
        """.write(to: bundleRoot.appendingPathComponent("source.pair.json"), atomically: true, encoding: .utf8)
        try "pair ok\n".write(to: bundleRoot.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.pairing.receipt.v1","id":"pair-1"}"#.write(
            to: bundleRoot.appendingPathComponent("exported-receipts/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )

        let archivePath = workDir.appendingPathComponent("source-bundle.tgz")
        let unpackedRoot = workDir.appendingPathComponent("unpacked-bundle", isDirectory: true)

        let repoRoot = repoRootURL()
        let pack = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "pack-bundle",
                "--bundle-root", bundleRoot.path,
                "--archive", archivePath.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID": "source-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_LABEL": "source",
            ],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(pack.stderr, "")
        XCTAssertTrue(FileManager.default.fileExists(atPath: archivePath.path))
        let manifestPath = archivePath.deletingPathExtension().appendingPathExtension("manifest.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: manifestPath.path))
        let manifestData = try Data(contentsOf: manifestPath)
        let manifest = try XCTUnwrap(JSONSerialization.jsonObject(with: manifestData) as? [String: Any])
        XCTAssertEqual(manifest["schema"] as? String, "supermover.acceptance.bundle_archive.v1")
        XCTAssertEqual(manifest["archive"] as? String, archivePath.lastPathComponent)
        XCTAssertEqual(manifest["bundle_status"] as? String, "in_progress")
        XCTAssertEqual(manifest["collection_mode"] as? String, "two_machine")
        XCTAssertEqual(manifest["machine_count"] as? Int, 2)
        XCTAssertEqual(manifest["meta"] as? String, "meta.json")
        XCTAssertEqual(manifest["exporting_machine_id"] as? String, "source-machine")
        XCTAssertEqual(manifest["exporting_machine_label"] as? String, "source")
        let workflowSummary = try XCTUnwrap(manifest["workflow_summary"] as? [String: Any])
        let workflowNextActions = try XCTUnwrap(workflowSummary["next_actions"] as? [[String: Any]])
        XCTAssertEqual(workflowNextActions.first?["machine"] as? String, "target")
        XCTAssertEqual(workflowNextActions.first?["step"] as? String, "target_serve_phase_1")
        let sha256 = try XCTUnwrap(manifest["sha256"] as? String)
        XCTAssertEqual(sha256.count, 64)

        let metaData = try Data(contentsOf: bundleRoot.appendingPathComponent("meta.json"))
        let meta = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        let evidence = meta["evidence"] as? [String: Any] ?? [:]
        let persistedWorkflowSummary = try XCTUnwrap((evidence["workflow_summary"] as? [String: Any])?["default"] as? [String: Any])
        let persistedNextActions = try XCTUnwrap(persistedWorkflowSummary["next_actions"] as? [[String: Any]])
        XCTAssertEqual(persistedNextActions.first?["step"] as? String, workflowNextActions.first?["step"] as? String)
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("workflow.summary.json").path))
        let workflowArtifactData = try Data(contentsOf: bundleRoot.appendingPathComponent("workflow.summary.json"))
        let workflowArtifact = try XCTUnwrap(JSONSerialization.jsonObject(with: workflowArtifactData) as? [String: Any])
        XCTAssertEqual(workflowArtifact["schema"] as? String, "supermover.acceptance.workflow_summary.v1")
        let artifactDefault = try XCTUnwrap(workflowArtifact["default"] as? [String: Any])
        let artifactNextActions = try XCTUnwrap(artifactDefault["next_actions"] as? [[String: Any]])
        XCTAssertEqual(artifactNextActions.first?["step"] as? String, workflowNextActions.first?["step"] as? String)

        let unpack = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "unpack-bundle",
                "--archive", archivePath.path,
                "--manifest", manifestPath.path,
                "--bundle-root", unpackedRoot.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_ID": "target-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_LABEL": "target",
            ],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(unpack.stderr, "")
        XCTAssertTrue(FileManager.default.fileExists(atPath: unpackedRoot.appendingPathComponent("meta.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: unpackedRoot.appendingPathComponent("source.pair.json").path))
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: unpackedRoot.appendingPathComponent("exported-receipts/pair-1.json").path
            )
        )

        let unpackedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: unpackedRoot)
        XCTAssertEqual(unpackedSnapshot.sourcePairArtifact?.pairing_receipt_id, "pair-1")
        XCTAssertEqual(unpackedSnapshot.sourcePairArtifact?.receipt_path, "exported-receipts/pair-1.json")
    }

    func testTwoMachineUnpackBundleRecordsVerifiedArchiveHandoffEvidence() throws {
        let workDir = try makeDirectory(named: "two-machine-unpack-records-handoff")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
              "machine_id": "source-machine",
              "machine_label": "source"
            }
          },
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try "hello\n".write(to: bundleRoot.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )

        let archivePath = workDir.appendingPathComponent("bundle.tgz")
        let manifestPath = workDir.appendingPathComponent("bundle.manifest.json")
        let unpackedRoot = workDir.appendingPathComponent("unpacked-bundle", isDirectory: true)

        let repoRoot = repoRootURL()
        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "pack-bundle",
                "--bundle-root", bundleRoot.path,
                "--archive", archivePath.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID": "source-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_LABEL": "source",
            ],
            currentDirectoryURL: repoRoot
        )

        let unpack = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "unpack-bundle",
                "--archive", archivePath.path,
                "--manifest", manifestPath.path,
                "--bundle-root", unpackedRoot.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_ID": "target-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_LABEL": "target",
            ],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(unpack.stderr, "")

        let metaData = try Data(contentsOf: unpackedRoot.appendingPathComponent("meta.json"))
        let meta = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        let evidence = meta["evidence"] as? [String: Any] ?? [:]
        let handoffs = evidence["bundle_handoffs"] as? [[String: Any]] ?? []
        XCTAssertEqual(handoffs.count, 1)
        let handoff = try XCTUnwrap(handoffs.first)
        XCTAssertEqual(handoff["archive"] as? String, "bundle.tgz")
        XCTAssertEqual(handoff["manifest"] as? String, "bundle.manifest.json")
        XCTAssertEqual(handoff["verified"] as? Bool, true)
        XCTAssertEqual(handoff["meta"] as? String, "meta.json")
        XCTAssertEqual(handoff["exporting_machine_id"] as? String, "source-machine")
        XCTAssertEqual(handoff["exporting_machine_label"] as? String, "source")
        XCTAssertEqual(handoff["importing_machine_id"] as? String, "target-machine")
        XCTAssertEqual(handoff["importing_machine_label"] as? String, "target")
        let sha256 = try XCTUnwrap(handoff["sha256"] as? String)
        XCTAssertEqual(sha256.count, 64)
    }

    func testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityIsTampered() throws {
        let workDir = try makeDirectory(named: "two-machine-unpack-tampered-manifest-export-identity")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
              "machine_id": "source-machine",
              "machine_label": "source"
            }
          },
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try "hello\n".write(to: bundleRoot.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)

        let archivePath = workDir.appendingPathComponent("bundle.tgz")
        let manifestPath = workDir.appendingPathComponent("bundle.manifest.json")
        let unpackedRoot = workDir.appendingPathComponent("unpacked-bundle", isDirectory: true)

        let repoRoot = repoRootURL()
        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "pack-bundle",
                "--bundle-root", bundleRoot.path,
                "--archive", archivePath.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID": "source-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_LABEL": "source",
            ],
            currentDirectoryURL: repoRoot
        )

        let tamperedManifest = try String(contentsOf: manifestPath)
            .replacingOccurrences(of: "\"exporting_machine_id\": \"source-machine\"", with: "\"exporting_machine_id\": \"target-machine\"")
            .replacingOccurrences(of: "\"exporting_machine_label\": \"source\"", with: "\"exporting_machine_label\": \"target\"")
        try tamperedManifest.write(to: manifestPath, atomically: true, encoding: .utf8)

        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "unpack-bundle",
                "--archive", archivePath.path,
                "--manifest", manifestPath.path,
                "--bundle-root", unpackedRoot.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_ID": "target-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_LABEL": "target",
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("acceptance bundle manifest exporting_machine_id mismatch"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: unpackedRoot.appendingPathComponent("meta.json").path))
    }

    func testTwoMachineUnpackBundleFailsClosedWhenManifestExportIdentityFieldsAreMissing() throws {
        let workDir = try makeDirectory(named: "two-machine-unpack-missing-manifest-export-identity")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
              "machine_id": "source-machine",
              "machine_label": "source"
            }
          },
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try "hello\n".write(to: bundleRoot.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)

        let archivePath = workDir.appendingPathComponent("bundle.tgz")
        let manifestPath = workDir.appendingPathComponent("bundle.manifest.json")
        let unpackedRoot = workDir.appendingPathComponent("unpacked-bundle", isDirectory: true)

        let repoRoot = repoRootURL()
        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "pack-bundle",
                "--bundle-root", bundleRoot.path,
                "--archive", archivePath.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID": "source-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_LABEL": "source",
            ],
            currentDirectoryURL: repoRoot
        )

        let manifestData = try Data(contentsOf: manifestPath)
        var manifest = try XCTUnwrap(JSONSerialization.jsonObject(with: manifestData) as? [String: Any])
        manifest.removeValue(forKey: "exporting_machine_id")
        manifest.removeValue(forKey: "exporting_machine_label")
        let strippedManifestData = try JSONSerialization.data(withJSONObject: manifest, options: [.prettyPrinted, .sortedKeys])
        try strippedManifestData.write(to: manifestPath)

        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "unpack-bundle",
                "--archive", archivePath.path,
                "--manifest", manifestPath.path,
                "--bundle-root", unpackedRoot.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_ID": "target-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_LABEL": "target",
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("malformed acceptance bundle manifest"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: unpackedRoot.appendingPathComponent("meta.json").path))
    }

    func testTwoMachineWorkflowStatusStartsWithTargetServeNextAction() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-initial")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["schema"] as? String, "supermover.acceptance.workflow_status.v1")
        XCTAssertEqual(status["ok"] as? Bool, false)
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.first?["machine"] as? String, "target")
        XCTAssertEqual(nextActions.first?["step"] as? String, "target_serve_phase_1")
        let commands = try XCTUnwrap(nextActions.first?["commands"] as? [String])
        XCTAssertEqual(commands.first, "sh macos/script/acceptance-two-machine.sh target-serve --profile '<target-profile>' --bundle-root '\(bundleRoot.path)'")

        let metaData = try Data(contentsOf: bundleRoot.appendingPathComponent("meta.json"))
        let meta = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        let evidence = meta["evidence"] as? [String: Any] ?? [:]
        let workflowSummary = try XCTUnwrap(evidence["workflow_summary"] as? [String: Any])
        let defaultSummary = try XCTUnwrap(workflowSummary["default"] as? [String: Any])
        let persistedNextActions = try XCTUnwrap(defaultSummary["next_actions"] as? [[String: Any]])
        XCTAssertEqual(persistedNextActions.first?["step"] as? String, "target_serve_phase_1")
    }

    func testTwoMachinePackBundleUsesCanonicalMachineFactsForManifestIdentityWhenRolesDisagree() throws {
        let workDir = try makeDirectory(named: "two-machine-pack-bundle-canonical-machine-facts")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
              "machine_id": "stale-role-machine",
              "machine_label": "stale-role"
            }
          },
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )

        let archivePath = workDir.appendingPathComponent("bundle.tgz")
        let repoRoot = repoRootURL()
        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "pack-bundle",
                "--bundle-root", bundleRoot.path,
                "--archive", archivePath.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID": "source-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_LABEL": "source",
            ],
            currentDirectoryURL: repoRoot
        )

        let manifestPath = archivePath.deletingPathExtension().appendingPathExtension("manifest.json")
        let manifestData = try Data(contentsOf: manifestPath)
        let manifest = try XCTUnwrap(JSONSerialization.jsonObject(with: manifestData) as? [String: Any])
        XCTAssertEqual(manifest["exporting_machine_id"] as? String, "source-machine")
        XCTAssertEqual(manifest["exporting_machine_label"] as? String, "source")
    }

    func testTwoMachinePackBundleFailsClosedWhenExportMachineDoesNotMatchCanonicalMachineFacts() throws {
        let workDir = try makeDirectory(named: "two-machine-pack-bundle-export-machine-mismatch")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )

        let archivePath = workDir.appendingPathComponent("bundle.tgz")
        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "pack-bundle",
                "--bundle-root", bundleRoot.path,
                "--archive", archivePath.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID": "target-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_LABEL": "target",
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(
            result.stderr.contains(
                "canonical source.machine.json / target.machine.json do not match exporting machine_id=target-machine"
            )
        )
        XCTAssertFalse(FileManager.default.fileExists(atPath: archivePath.path))
    }

    func testTwoMachinePackBundleFailsClosedWhenCanonicalSourceMachineFactsArtifactIsSymlinkedOutsideBundle() throws {
        let workDir = try makeDirectory(named: "two-machine-pack-bundle-source-machine-facts-symlink")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let outsideMachineFactsURL = workDir.appendingPathComponent("outside-source.machine.json")
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: outsideMachineFactsURL, atomically: true, encoding: .utf8)
        do {
            try FileManager.default.createSymbolicLink(
                at: bundleRoot.appendingPathComponent("source.machine.json"),
                withDestinationURL: outsideMachineFactsURL
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        let archivePath = workDir.appendingPathComponent("bundle.tgz")
        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "pack-bundle",
                "--bundle-root", bundleRoot.path,
                "--archive", archivePath.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID": "source-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_LABEL": "source",
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(
            result.stderr.contains(
                "missing valid canonical source.machine.json / target.machine.json export identity evidence"
            )
        )
        XCTAssertFalse(FileManager.default.fileExists(atPath: archivePath.path))
    }

    func testTwoMachinePackBundleFailsClosedWhenCanonicalMachineFactsNeedLabelDisambiguation() throws {
        let workDir = try makeDirectory(named: "two-machine-pack-bundle-export-machine-ambiguous")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "same_machine",
            "machine_count": 1
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.json",
            machineID: "same-machine",
            machineLabel: "same-machine source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "target.machine.json",
            machineID: "same-machine",
            machineLabel: "same-machine target"
        )

        let archivePath = workDir.appendingPathComponent("bundle.tgz")
        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "pack-bundle",
                "--bundle-root", bundleRoot.path,
                "--archive", archivePath.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID": "same-machine",
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(
            result.stderr.contains(
                "canonical source.machine.json / target.machine.json both match exporting machine_id=same-machine"
            )
        )
        XCTAssertFalse(FileManager.default.fileExists(atPath: archivePath.path))
    }

    func testTwoMachineWorkflowStatusRequiresVerifiedHandoffBeforeEvaluate() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-handoff")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
          "evidence": {
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
              "receipt_path": "exported-receipts/pair-1.json"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted",
                "machine_id": "target-machine",
                "machine_label": "target"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed",
                "machine_id": "target-machine",
                "machine_label": "target"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched",
                "machine_id": "source-machine",
                "machine_label": "source"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"source-machine","machine_label":"source"}"#
            .write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"target-machine","machine_label":"target"}"#
            .write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 0)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 0)
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 1)
        XCTAssertEqual(nextActions.first?["machine"] as? String, "either")
        XCTAssertEqual(nextActions.first?["step"] as? String, "bundle_handoff")
        let commands = try XCTUnwrap(nextActions.first?["commands"] as? [String])
        XCTAssertEqual(commands.count, 3)
        XCTAssertTrue(commands[0].contains("pack-bundle"))
        XCTAssertTrue(commands[1].contains("unpack-bundle"))
        XCTAssertTrue(commands[2].contains("merge-bundle"))
    }

    func testTwoMachineWorkflowStatusDoesNotTreatWrongMachinePairHandoffAsDistinctMachineProof() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-wrong-machine-pair-handoff")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
          "evidence": {
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
              "receipt_path": "exported-receipts/pair-1.json"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "bundle_handoffs": [
              {
                "archive": "bundle.tgz",
                "manifest": "bundle.manifest.json",
                "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
                "meta": "meta.json",
                "verified": true,
                "exporting_machine_id": "third-source-machine",
                "exporting_machine_label": "other-source",
                "importing_machine_id": "third-target-machine",
                "importing_machine_label": "other-target"
              }
            ],
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted",
                "machine_id": "target-machine",
                "machine_label": "target"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed",
                "machine_id": "target-machine",
                "machine_label": "target"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched",
                "machine_id": "source-machine",
                "machine_label": "source"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 0)
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, false)
        XCTAssertEqual(
            status["failure_message"] as? String,
            "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 1)
        XCTAssertEqual(nextActions.first?["machine"] as? String, "either")
        XCTAssertEqual(nextActions.first?["step"] as? String, "bundle_handoff")
    }

    func testTwoMachineWorkflowStatusDoesNotTreatSameMachineHandoffAsDistinctMachineProof() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-same-machine-handoff")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
          "evidence": {
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
              "receipt_path": "exported-receipts/pair-1.json"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "bundle_handoffs": [
              {
                "archive": "bundle.tgz",
                "manifest": "bundle.manifest.json",
                "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
                "meta": "meta.json",
                "verified": true,
                "exporting_machine_id": "source-machine",
                "exporting_machine_label": "source",
                "importing_machine_id": "source-machine",
                "importing_machine_label": "source"
              }
            ],
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 0)
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, false)
        XCTAssertEqual(
            status["failure_message"] as? String,
            "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 1)
        XCTAssertEqual(nextActions.first?["machine"] as? String, "either")
        XCTAssertEqual(nextActions.first?["step"] as? String, "bundle_handoff")
    }

    func testTwoMachineWorkflowStatusAdvancesToEvaluateWhenVerifiedMachinePairHandoffExists() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-ready-to-evaluate")
        let targetRoot = try makeDirectory(named: "two-machine-workflow-status-ready-to-evaluate-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
          "evidence": {
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
              "receipt_path": "exported-receipts/pair-1.json"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "bundle_handoffs": [
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
            ],
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted",
                "machine_id": "target-machine",
                "machine_label": "target"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed",
                "machine_id": "target-machine",
                "machine_label": "target"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched",
                "machine_id": "source-machine",
                "machine_label": "source"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["has_installed_app_machine_pair_proof"] as? Bool, true)
        XCTAssertEqual(status["ok"] as? Bool, false)
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 1)
        XCTAssertEqual(nextActions.first?["step"] as? String, "evaluate")
        let commands = try XCTUnwrap(nextActions.first?["commands"] as? [String])
        XCTAssertEqual(commands.count, 1)
        XCTAssertTrue(commands[0].contains("evaluate"))
        XCTAssertTrue(commands[0].contains("--require-operator-evidence"))
    }

    func testTwoMachineWorkflowStatusRejectsMalformedSourceVerifyCounts() throws {
        let cases: [(name: String, summary: [String: Any])] = [
            (
                "string-files-verified",
                ["files_verified": "1", "files_expected": 1, "error_findings": 0, "artifact_problems": 0]
            ),
            (
                "nonnumeric-files-verified",
                ["files_verified": "many", "files_expected": 1, "error_findings": 0, "artifact_problems": 0]
            ),
            (
                "bool-files-verified",
                ["files_verified": true, "files_expected": 1, "error_findings": 0, "artifact_problems": 0]
            ),
            (
                "string-error-findings",
                ["files_verified": 1, "files_expected": 1, "error_findings": "0", "artifact_problems": 0]
            ),
            (
                "string-artifact-problems",
                ["files_verified": 1, "files_expected": 1, "error_findings": 0, "artifact_problems": "0"]
            ),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-verify-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            try writeSourceVerifySummary(testCase.summary, bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

            let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
            let steps = try XCTUnwrap(status["steps"] as? [[String: Any]], testCase.name)
            XCTAssertEqual(
                steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
                false,
                testCase.name
            )
            let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]], testCase.name)
            XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"], testCase.name)
        }
    }

    func testTwoMachineWorkflowStatusRejectsOutOfRangeSourceStatusAndHealthCounters() throws {
        let cases: [(name: String, artifact: String, path: [String], value: String)] = [
            ("status-files-expected-too-large", "source.status.json", ["latest_session", "files_expected"], "9223372036854775808"),
            ("status-files-expected-decimal-intmax", "source.status.json", ["latest_session", "files_expected"], "9223372036854775807.0"),
            ("status-network-transfers-scientific", "source.status.json", ["counts", "network_transfers"], "1e100"),
            ("health-incomplete-sessions-too-large", "source.health.json", ["summary", "incomplete_sessions"], "9223372036854775808"),
            ("health-incomplete-sessions-decimal-intmax", "source.health.json", ["summary", "incomplete_sessions"], "9223372036854775807.0"),
            ("health-network-transfers-scientific", "source.health.json", ["summary", "network_transfers"], "1e100"),
            ("health-network-transfers-intmax-scientific", "source.health.json", ["summary", "network_transfers"], "9.223372036854775807e18"),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-counter-range-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            try rewriteJSONObjectString(
                fixture.bundleRoot.appendingPathComponent(testCase.artifact),
                path: testCase.path,
                rawValue: testCase.value
            )

            let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
            let steps = try XCTUnwrap(status["steps"] as? [[String: Any]], testCase.name)
            XCTAssertEqual(
                steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
                false,
                testCase.name
            )
            let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]], testCase.name)
            XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"], testCase.name)
        }
    }

    func testTwoMachineWorkflowStatusRejectsControlCharacterControlPlaneIDs() throws {
        let cases: [(name: String, pairingReceiptID: String?, sessionID: String?)] = [
            ("pair-nul", "pair-1\u{0000}", nil),
            ("pair-bel", "pair-1\u{0007}", nil),
            ("session-nul", nil, "session-1\u{0000}"),
            ("session-bel", nil, "session-1\u{0007}"),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-control-id-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            if let pairingReceiptID = testCase.pairingReceiptID {
                try writeSourcePairBundleArtifacts(to: fixture.bundleRoot, pairingReceiptID: pairingReceiptID)
            }
            if let sessionID = testCase.sessionID {
                try AcceptanceReleaseEvidenceFixtures.jsonString([
                    "profile": "/tmp/source.profile.json",
                    "session_id": sessionID,
                    "target_address": "127.0.0.1:39395",
                    "receiver_address": "127.0.0.1:9443",
                    "target_mode": "pairing",
                ]).write(to: fixture.bundleRoot.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
                try AcceptanceReleaseEvidenceFixtures.jsonString([
                    "schema": "supermover.acceptance.current_source_consistency.v1",
                    "status": "pass",
                    "mode": "current_source_verified",
                    "session_id": sessionID,
                ]).write(to: fixture.bundleRoot.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)
            }

            let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
            let steps = try XCTUnwrap(status["steps"] as? [[String: Any]], testCase.name)
            if testCase.pairingReceiptID != nil {
                XCTAssertEqual(
                    steps.first(where: { ($0["id"] as? String) == "source_pair" })?["done"] as? Bool,
                    false,
                    testCase.name
                )
            }
            XCTAssertEqual(
                steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
                false,
                testCase.name
            )
            let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]], testCase.name)
            XCTAssertTrue(nextActions.contains { ($0["step"] as? String) == "source_transfer" }, testCase.name)
        }
    }

    func testTwoMachineWorkflowStatusRequiresNonBlankOperatorEvidenceDetail() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-blank-operator-detail")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try setOperatorEvidenceDetail(kind: "local_network", detail: " \n\t ", bundleRoot: fixture.bundleRoot)

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "operator_local_network" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertTrue(
            nextActions.contains(where: { ($0["step"] as? String) == "operator_local_network" }),
            "next_actions:\n\(nextActions)"
        )
    }

    func testTwoMachineWorkflowStatusRequiresOperatorEvidenceMachineBinding() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-operator-machine-binding")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try setOperatorEvidenceMachineID(kind: "local_network", machineID: nil, bundleRoot: fixture.bundleRoot)
        try setOperatorEvidenceMachineID(kind: "pairing_confirmation", machineID: "target-machine", bundleRoot: fixture.bundleRoot)

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "operator_local_network" })?["done"] as? Bool,
            false
        )
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "operator_pairing_confirmation" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertTrue(
            nextActions.contains(where: { ($0["step"] as? String) == "operator_local_network" }),
            "next_actions:\n\(nextActions)"
        )
        XCTAssertTrue(
            nextActions.contains(where: { ($0["step"] as? String) == "operator_pairing_confirmation" }),
            "next_actions:\n\(nextActions)"
        )
    }

    func testTwoMachineWorkflowStatusUsesRawOperatorMachineBinding() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-operator-raw-machine-binding")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try rewriteMachineFacts(machine: "target", machineID: "target-machine ", bundleRoot: fixture.bundleRoot)
        try setOperatorEvidenceMachineID(kind: "local_network", machineID: "target-machine", bundleRoot: fixture.bundleRoot)
        try setOperatorEvidenceMachineID(kind: "firewall", machineID: "target-machine ", bundleRoot: fixture.bundleRoot)

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "operator_local_network" })?["done"] as? Bool,
            false
        )
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "operator_firewall" })?["done"] as? Bool,
            true
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["operator_local_network"])
    }

    func testTwoMachineEvaluateRequiresNonBlankOperatorEvidenceDetail() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-blank-operator-detail")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try setOperatorEvidenceDetail(kind: "local_network", detail: " \n\t ", bundleRoot: fixture.bundleRoot)

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRequiresOperatorEvidenceMachineBinding() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-operator-machine-binding")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try setOperatorEvidenceMachineID(kind: "firewall", machineID: "source-machine", bundleRoot: fixture.bundleRoot)

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("operator evidence must be pass with non-empty detail and machine_id bound to source/target machine facts"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateUsesRawOperatorMachineBinding() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-operator-raw-machine-binding")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try rewriteMachineFacts(machine: "target", machineID: "target-machine ", bundleRoot: fixture.bundleRoot)
        try setOperatorEvidenceMachineID(kind: "local_network", machineID: "target-machine", bundleRoot: fixture.bundleRoot)
        try setOperatorEvidenceMachineID(kind: "firewall", machineID: "target-machine ", bundleRoot: fixture.bundleRoot)

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("operator evidence must be pass with non-empty detail and machine_id bound to source/target machine facts"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineStrictAcceptanceRejectsMissingBundleLocalNotaryLog() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-missing-notary-log")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try FileManager.default.removeItem(
            at: fixture.bundleRoot.appendingPathComponent("source.notary-log.json")
        )

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(failures, ["source.notarization.json is not release-ready"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("source.notarization.json is not release-ready")
                || result.stderr.contains("source.notary-log.json"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineStrictAcceptanceRejectsMalformedBundleLocalNotaryLog() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-malformed-notary-log")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try "not json\n".write(
            to: fixture.bundleRoot.appendingPathComponent("source.notary-log.json"),
            atomically: true,
            encoding: .utf8
        )

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(failures, ["source.notarization.json is not release-ready"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("source.notarization.json is not release-ready")
                || result.stderr.contains("source.notary-log.json"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineStrictAcceptanceRejectsBundleLocalNotaryLogFromDifferentSubmission() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-mismatched-notary-log")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON(
            submissionID: "22222222-2222-2222-2222-222222222222"
        ).write(
            to: fixture.bundleRoot.appendingPathComponent("source.notary-log.json"),
            atomically: true,
            encoding: .utf8
        )

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(failures, ["source.notarization.json is not release-ready"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("source.notarization.json is not release-ready")
                || result.stderr.contains("source.notary-log.json"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineStrictAcceptanceRejectsSourceNotarizationWithoutAuthMode() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-missing-notary-auth-mode")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try rewriteJSONObject(
            fixture.bundleRoot.appendingPathComponent("source.notarization.json")
        ) { root in
            root.removeValue(forKey: "auth_mode")
        }

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(failures, ["source.notarization.json is not release-ready"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("source.notarization.json is not release-ready")
                || result.stderr.contains("auth_mode"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineStrictAcceptanceRejectsSourceNotarizationWithFailureRecord() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-notary-failure-record")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try rewriteJSONObject(
            fixture.bundleRoot.appendingPathComponent("source.notarization.json")
        ) { root in
            root["failure"] = [
                "id": "notary_rejected",
                "message": "notarytool rejected the submission",
            ]
        }

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(failures, ["source.notarization.json is not release-ready"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("source.notarization.json is not release-ready")
                || result.stderr.contains("failure"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineStrictAcceptanceRejectsMalformedSourceNotarizationSubmissionID() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-notary-malformed-submission-id")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try rewriteJSONObject(
            fixture.bundleRoot.appendingPathComponent("source.notarization.json")
        ) { root in
            var submission = try XCTUnwrap(root["submission"] as? [String: Any])
            submission["id"] = "manual-pass"
            root["submission"] = submission
        }

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(failures, ["source.notarization.json is not release-ready"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("source.notarization.json is not release-ready")
                || result.stderr.contains("submission"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineStrictAcceptanceRejectsNotarizationBoundToOtherMachineNotaryLog() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-cross-bound-notary-log")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }

        let sourceAppPath = "/Applications/SuperMover Source.app"
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: sourceAppPath,
            auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(appPath: sourceAppPath),
            notaryLogPath: "target.notary-log.json"
        ).write(
            to: fixture.bundleRoot.appendingPathComponent("source.notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(failures, ["source.notarization.json is not release-ready"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("source.notarization.json is not release-ready")
                || result.stderr.contains("source.notary-log.json"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineStrictAcceptanceRejectsNotarizationBoundToOtherMachineAuditPath() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-cross-bound-notary-audit")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }

        let sourceAppPath = "/Applications/SuperMover Source.app"
        let targetAppPath = "/Applications/SuperMover Target.app"
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: sourceAppPath,
            auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(appPath: targetAppPath),
            notaryLogPath: "source.notary-log.json"
        ).write(
            to: fixture.bundleRoot.appendingPathComponent("source.notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(failures, ["source.notarization.json is not release-ready"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("source.notarization.json is not release-ready")
                || result.stderr.contains("source post-staple audit")
                || result.stderr.contains("post-staple.audit.json"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineRecordOperatorEvidenceRejectsBlankDetail() throws {
        let workDir = try makeDirectory(named: "two-machine-record-operator-evidence-blank-detail")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "record-operator-evidence",
                "--bundle-root", bundleRoot.path,
                "--kind", "local_network",
                "--status", "pass",
                "--detail", " \n\t ",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 2)
        XCTAssertTrue(result.stderr.contains("operator evidence detail is required"))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.operatorEvidence, [:])
    }

    func testTwoMachineRecordOperatorEvidenceBindsTargetMachineFacts() throws {
        let workDir = try makeDirectory(named: "two-machine-record-operator-evidence-machine-binding")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false
        ).write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(bundleRoot: bundleRoot)

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "record-operator-evidence",
                "--bundle-root", bundleRoot.path,
                "--kind", "firewall",
                "--status", "pass",
                "--detail", "allowed firewall access on target",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 0)
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.operatorEvidence["firewall"]?.machine_id, "target-machine")
        XCTAssertEqual(snapshot.operatorEvidence["firewall"]?.machine_label, "target")
    }

    func testTwoMachineRecordOperatorEvidenceRejectsPassWithoutMachineFacts() throws {
        let workDir = try makeDirectory(named: "two-machine-record-operator-evidence-no-machine-facts")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "record-operator-evidence",
                "--bundle-root", bundleRoot.path,
                "--kind", "local_network",
                "--status", "pass",
                "--detail", "accepted prompt",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 2)
        XCTAssertTrue(result.stderr.contains("requires canonical target.machine.json before pass recording"))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.operatorEvidence, [:])
    }

    func testTwoMachineWorkflowStatusFailsClosedWhenSourcePairReceiptArtifactIsMissing() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-missing-receipt")
        let targetRoot = try makeDirectory(named: "two-machine-workflow-status-missing-receipt-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try FileManager.default.removeItem(
            at: bundleRoot.appendingPathComponent("exported-receipts/pair-1.json")
        )

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_pair" })?["done"] as? Bool,
            false
        )
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "target_import" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_pair", "target_import"])
    }

    func testTwoMachineWorkflowStatusFailsClosedWhenSourcePairReceiptArtifactIsMalformed() throws {
        let cases: [(name: String, payload: String)] = [
            ("empty-object", "{}"),
            ("wrong-id", try targetPairingReceiptJSON(id: "pair-other")),
            (
                "missing-verification",
                try AcceptanceWorkflowFixtures.pairingReceiptJSON(verificationHash: nil, verificationPhrase: nil)
            ),
            (
                "bad-verified-at",
                try AcceptanceWorkflowFixtures.pairingReceiptJSON(verifiedAt: "not-a-date")
            ),
            (
                "device-key-mismatch",
                try AcceptanceWorkflowFixtures.pairingReceiptJSON(devicePublicKey: "other-device")
            ),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-malformed-source-receipt-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            try testCase.payload.write(
                to: fixture.bundleRoot.appendingPathComponent("exported-receipts/pair-1.json"),
                atomically: true,
                encoding: .utf8
            )

            let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)
            let steps = try XCTUnwrap(status["steps"] as? [[String: Any]], testCase.name)
            XCTAssertEqual(
                steps.first(where: { ($0["id"] as? String) == "source_pair" })?["done"] as? Bool,
                false,
                testCase.name
            )
            XCTAssertEqual(
                steps.first(where: { ($0["id"] as? String) == "target_import" })?["done"] as? Bool,
                false,
                testCase.name
            )
            let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]], testCase.name)
            XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_pair", "target_import"], testCase.name)
        }
    }

    func testTwoMachineWorkflowStatusRejectsMissingReferencedTargetImportArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-missing-target-import-artifact")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try FileManager.default.removeItem(at: fixture.bundleRoot.appendingPathComponent("target.adopt-pairing.txt"))

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "target_import" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["target_import"])
    }

    func testTwoMachineWorkflowStatusFailsClosedWhenSourceReportReceiptMismatchesSourcePair() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-report-receipt-mismatch")
        let targetRoot = try makeDirectory(named: "two-machine-workflow-status-report-receipt-mismatch-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try #"{"pairing":{"receipt_id":"pair-stale","status":"paired_receipt_valid"}}"#.write(
            to: bundleRoot.appendingPathComponent("source.report.json"),
            atomically: true,
            encoding: .utf8
        )

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"])
    }

    func testTwoMachineWorkflowStatusFailsClosedWhenSourceReportReceiptHasWhitespace() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-report-receipt-whitespace")
        let targetRoot = try makeDirectory(named: "two-machine-workflow-status-report-receipt-whitespace-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try #"{"pairing":{"receipt_id":"pair-1 ","status":"paired_receipt_valid"}}"#.write(
            to: bundleRoot.appendingPathComponent("source.report.json"),
            atomically: true,
            encoding: .utf8
        )

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"])
    }

    func testTwoMachineWorkflowStatusRejectsSourceStatusSessionMismatch() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-source-status-session-mismatch")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try AcceptanceWorkflowFixtures.statusJSON(targetRoot: fixture.targetRoot.path)
            .replacingOccurrences(of: #""id":"session-1""#, with: #""id":"session-stale""#)
            .write(to: fixture.bundleRoot.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"])
    }

    func testTwoMachineWorkflowStatusRejectsSourceHealthWithoutTransferSession() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-source-health-session-missing")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try AcceptanceWorkflowFixtures.healthJSON(targetRoot: fixture.targetRoot.path)
            .replacingOccurrences(of: #""session_id":"session-1""#, with: #""session_id":"session-stale""#)
            .write(to: fixture.bundleRoot.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"])
    }

    func testTwoMachineWorkflowStatusRejectsMixedSourceTransferTargetRoots() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-mixed-transfer-target-roots")
        let otherTargetRoot = try makeDirectory(named: "two-machine-workflow-status-other-target-root")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        defer { try? FileManager.default.removeItem(at: otherTargetRoot) }
        try AcceptanceWorkflowFixtures.statusJSON(targetRoot: otherTargetRoot.path)
            .write(to: fixture.bundleRoot.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"])
    }

    func testTwoMachineWorkflowStatusRejectsEvaluatedSourceTransferTargetRootsFromDifferentTargetRoot() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-evaluated-transfer-target-root")
        let otherTargetRoot = try makeDirectory(named: "two-machine-workflow-status-other-evaluated-target-root")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        defer { try? FileManager.default.removeItem(at: otherTargetRoot) }

        let evaluationResult = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
        XCTAssertEqual(evaluationResult.exitCode, 0, "stderr:\n\(evaluationResult.stderr)")
        try rewriteAllSourceTransferTargetRoots(bundleRoot: fixture.bundleRoot, targetRoot: otherTargetRoot)

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        XCTAssertEqual(status["ok"] as? Bool, false)
        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"])
    }

    func testTwoMachineWorkflowStatusRejectsMalformedSourceBrowseArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-source-browse-malformed")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try #"{"trusted":false,"candidate_count":0,"candidates":[]}"#.write(
            to: fixture.bundleRoot.appendingPathComponent("source.browse.json"),
            atomically: true,
            encoding: .utf8
        )

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_browse" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_browse"])
    }

    func testTwoMachineWorkflowStatusRejectsMalformedTargetAdvertiseArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-target-advertise-malformed")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try #"{"status":"advertised","trusted":false}"#.write(
            to: fixture.bundleRoot.appendingPathComponent("target.advertise.json"),
            atomically: true,
            encoding: .utf8
        )

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "target_advertise" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["target_advertise"])
    }

    func testTwoMachineWorkflowStatusRejectsMissingTargetReadyArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-target-ready-missing")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try FileManager.default.removeItem(at: fixture.bundleRoot.appendingPathComponent("target.ready.json"))

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "target_serve_phase_1" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["target_serve_phase_1"])
    }

    func testTwoMachineWorkflowStatusRejectsMalformedTargetReadyArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-target-ready-malformed")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try #"{"address":"127.0.0.1:39395","mode":"pairing"}"#.write(
            to: fixture.bundleRoot.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "target_serve_phase_1" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["target_serve_phase_1"])
    }

    func testTwoMachineWorkflowStatusRejectsSourcePairTargetAddressMismatchWithTargetReady() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-source-pair-target-ready-mismatch")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "profile": "/tmp/source.profile.json",
            "target_address": "127.0.0.1:49999",
            "verification_code": "123456",
            "pairing_receipt_id": "pair-1",
            "receipt_path": "exported-receipts/pair-1.json",
        ]).write(
            to: fixture.bundleRoot.appendingPathComponent("source.pair.json"),
            atomically: true,
            encoding: .utf8
        )

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_pair" })?["done"] as? Bool,
            false
        )
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "target_import" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_pair", "target_import"])
    }

    func testTwoMachineWorkflowStatusRejectsSourceTransferReceiverMismatchWithTargetReady() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-source-transfer-target-ready-mismatch")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "profile": "/tmp/source.profile.json",
            "session_id": "session-1",
            "target_address": "127.0.0.1:39395",
            "receiver_address": "127.0.0.1:9555",
            "target_mode": "pairing",
        ]).write(
            to: fixture.bundleRoot.appendingPathComponent("source.transfer.json"),
            atomically: true,
            encoding: .utf8
        )

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_pair" })?["done"] as? Bool,
            true
        )
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"])
    }

    func testTwoMachineWorkflowStatusRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-target-ready-lacks-receiver-transfer")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "address": "127.0.0.1:39395",
            "verification_code": "123456",
            "mode": "pairing",
            "receiver_routes": false,
            "push_network": false,
            "trusted": false,
            "transfer": false,
        ]).write(
            to: fixture.bundleRoot.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "target_serve_phase_1" })?["done"] as? Bool,
            true
        )
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_pair" })?["done"] as? Bool,
            true
        )
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"])
    }

    func testTwoMachineWorkflowStatusFailsClosedWhenSourceConsistencySessionHasWhitespace() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-consistency-session-whitespace")
        let targetRoot = try makeDirectory(named: "two-machine-workflow-status-consistency-session-whitespace-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "status": "pass",
          "mode": "current_source_verified",
          "session_id": "session-1 "
        }
        """.write(to: bundleRoot.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"])
    }

    func testTwoMachineWorkflowStatusFailsClosedWhenSourcePairIDIsUnsafeButTransferArtifactsLookReady() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-unsafe-pairing-id")
        let targetRoot = try makeDirectory(named: "two-machine-workflow-status-unsafe-pairing-id-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try writeSourcePairBundleArtifacts(to: bundleRoot, pairingReceiptID: "../pair-escape")

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(steps.first(where: { ($0["id"] as? String) == "source_pair" })?["done"] as? Bool, false)
        XCTAssertEqual(steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool, false)
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_pair", "target_import", "source_transfer"])
    }

    func testTwoMachineWorkflowStatusUsesSourceConsistencyArtifactBaselineBeforeMeta() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-consistency-baseline")
        let targetRoot = try makeDirectory(named: "two-machine-workflow-status-consistency-baseline-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "baseline": "artifact.baseline.json",
          "status": "pass",
          "mode": "current_source_verified",
          "session_id": "session-1"
        }
        """.write(to: bundleRoot.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let steps = try XCTUnwrap(status["steps"] as? [[String: Any]])
        XCTAssertEqual(
            steps.first(where: { ($0["id"] as? String) == "source_transfer" })?["done"] as? Bool,
            false
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_transfer"])
    }

    func testTwoMachineWorkflowStatusRequiresCollectionReviewBeforeEvaluateWhenStrictEvidenceCollectionIsInvalid() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-invalid-collection")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "same_machine",
            "machine_count": 1
          },
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
          "evidence": {
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
              "receipt_path": "exported-receipts/pair-1.json"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "bundle_handoffs": [
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
            ],
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["has_installed_app_machine_pair_proof"] as? Bool, true)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_failures"] as? [String], ["invalid_collection"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 1)
        XCTAssertEqual(nextActions.first?["step"] as? String, "review_collection")
        let commands = try XCTUnwrap(nextActions.first?["commands"] as? [String])
        XCTAssertEqual(commands, [])
    }

    func testTwoMachineWorkflowStatusFailsClosedWhenVerifiedHandoffsAlsoContainAnotherCrossMachinePair() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-contradictory-verified-handoffs")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
          "evidence": {
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
              "receipt_path": "exported-receipts/pair-1.json"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "bundle_handoffs": [
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
                "archive": "bundle.other.tgz",
                "manifest": "bundle.other.manifest.json",
                "sha256": "2222222222222222222222222222222222222222222222222222222222222222",
                "meta": "meta.other.json",
                "verified": true,
                "exporting_machine_id": "third-source-machine",
                "exporting_machine_label": "other-source",
                "importing_machine_id": "third-target-machine",
                "importing_machine_label": "other-target"
              }
            ],
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 2)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["matches_recorded_machine_pair"] as? Bool, false)
        XCTAssertEqual(status["has_installed_app_machine_pair_proof"] as? Bool, true)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_proof_failures"] as? [String])
        XCTAssertEqual(failures, ["contradictory_verified_bundle_handoffs"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 1)
        XCTAssertEqual(nextActions.first?["machine"] as? String, "either")
        XCTAssertEqual(nextActions.first?["step"] as? String, "review_bundle_handoff")
        let commands = try XCTUnwrap(nextActions.first?["commands"] as? [String])
        XCTAssertEqual(commands, [])
    }

    func testTwoMachineWorkflowStatusDoesNotTreatRoleMatchedHandoffAsProofWhenMachineFactsDisagree() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-machine-facts-mismatch")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
          "evidence": {
            "machine_facts": {
              "source": {
                "output": "source.machine.json",
                "machine_id": "other-source-machine",
                "machine_label": "other-source"
              },
              "target": {
                "output": "target.machine.json",
                "machine_id": "other-target-machine",
                "machine_label": "other-target"
              }
            },
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
              "receipt_path": "exported-receipts/pair-1.json"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "bundle_handoffs": [
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
            ],
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 0)
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, true)
        XCTAssertEqual(
            status["failure_message"] as? String,
            Self.machineIdentityCorrectionFailureMessage
        )
        try assertMachineIdentityCorrectionNextActions(status)
    }

    func testTwoMachineWorkflowStatusDoesNotTreatMetaMatchedHandoffAsProofWhenMachineFactArtifactsDisagree() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-machine-fact-artifact-mismatch")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
          "evidence": {
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
              "receipt_path": "exported-receipts/pair-1.json"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "bundle_handoffs": [
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
            ],
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"other-source-machine","machine_label":"other-source"}"#
            .write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"other-target-machine","machine_label":"other-target"}"#
            .write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 0)
        XCTAssertEqual(status["matches_recorded_machine_pair"] as? Bool, false)
        XCTAssertEqual(status["machine_facts_consistent"] as? Bool, false)
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, true)
        XCTAssertEqual(
            status["failure_message"] as? String,
            Self.machineIdentityCorrectionFailureMessage
        )
        try assertMachineIdentityCorrectionNextActions(status)
    }

    func testTwoMachineWorkflowStatusDoesNotTreatAlternateMetaMachineFactOutputsAsProofWhenCanonicalArtifactsDisagree() throws {
        let workDir = try makeDirectory(named: "two-machine-workflow-status-selected-machine-fact-output-mismatch")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
          "evidence": {
            "machine_facts": {
              "source": {
                "output": "source.machine.selected.json",
                "machine_id": "source-machine",
                "machine_label": "source"
              },
              "target": {
                "output": "target.machine.selected.json",
                "machine_id": "target-machine",
                "machine_label": "target"
              }
            },
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
              "receipt_path": "exported-receipts/pair-1.json"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "bundle_handoffs": [
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
            ],
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.json",
            machineID: "other-source-machine",
            machineLabel: "other-source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "target.machine.json",
            machineID: "other-target-machine",
            machineLabel: "other-target"
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["verified_bundle_handoffs"] as? Int, 1)
        XCTAssertEqual(status["verified_cross_machine_bundle_handoffs"] as? Int, 0)
        XCTAssertEqual(status["matches_recorded_machine_pair"] as? Bool, false)
        XCTAssertEqual(status["has_installed_app_machine_pair_proof"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, false)
        XCTAssertEqual(status["machine_facts_consistent"] as? Bool, false)
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, true)
        XCTAssertEqual(
            status["failure_message"] as? String,
            Self.machineIdentityCorrectionFailureMessage
        )
        try assertMachineIdentityCorrectionNextActions(status)
    }

    func testTwoMachineEvaluateRejectsWrongMachinePairHandoff() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-wrong-machine-pair-handoff")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let controlPlane = targetRoot.appendingPathComponent(".supermover", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("pairings"), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("sessions/session-1"), withIntermediateDirectories: true)

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
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
              {
                "archive": "bundle.tgz",
                "manifest": "bundle.manifest.json",
                "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
                "meta": "meta.json",
                "verified": true,
                "exporting_machine_id": "third-source-machine",
                "exporting_machine_label": "other-source",
                "importing_machine_id": "third-target-machine",
                "importing_machine_label": "other-target"
              }
            ],
            "source_pair": {
              "output": "source.pair.json",
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "pair": "source.pair.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "verify": "source.verify.json",
              "status": "source.status.json",
              "report": "source.report.json",
              "health": "source.health.json",
              "push": "source.network-push.txt"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "operator": {
              "local_network": {"status":"pass","detail":"ok"},
              "firewall": {"status":"pass","detail":"ok"},
              "pairing_confirmation": {"status":"pass","detail":"ok"}
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"source-machine","machine_label":"source"}"#
            .write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"target-machine","machine_label":"target"}"#
            .write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeTargetReadyBundleArtifact(to: bundleRoot)
        try writeSourcePairBundleArtifacts(to: bundleRoot)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#
            .write(to: bundleRoot.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted\n".write(to: bundleRoot.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok\n".write(to: bundleRoot.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#
            .write(to: bundleRoot.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)
        try #"{"baseline":"current"}"#.write(to: bundleRoot.appendingPathComponent("source.baseline.json"), atomically: true, encoding: .utf8)
        try #"{"summary":{"files_verified":1,"files_expected":1,"error_findings":0,"artifact_problems":0},"manifest":{"manifestID":"manifest-1"},"target_root":"\#(targetRoot.path)","session_id":"session-1","merkleRootProof":{"detail":"unavailable"}}"#
            .write(to: bundleRoot.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.reportJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.statusJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.healthJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try targetPairingReceiptJSON()
            .write(to: controlPlane.appendingPathComponent("pairings/pair-1.json"), atomically: true, encoding: .utf8)
        try targetNetworkTransferJSON()
            .write(to: controlPlane.appendingPathComponent("sessions/session-1/network-transfer.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains(
                "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"
            ),
            "stderr:\n\(result.stderr)"
        )
    }

    func testTwoMachineEvaluateRejectsMissingSourcePairReceiptArtifact() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-missing-receipt")
        let targetRoot = try makeDirectory(named: "two-machine-evaluate-missing-receipt-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try writeReadyTargetControlPlane(to: targetRoot)
        try FileManager.default.removeItem(
            at: bundleRoot.appendingPathComponent("exported-receipts/pair-1.json")
        )

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains(
                "missing exported receipt: \(bundleRoot.appendingPathComponent("exported-receipts/pair-1.json").path)"
            )
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRejectsSourceReportReceiptMismatch() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-report-receipt-mismatch")
        let targetRoot = try makeDirectory(named: "two-machine-evaluate-report-receipt-mismatch-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try writeReadyTargetControlPlane(to: targetRoot)
        try #"{"pairing":{"receipt_id":"pair-stale","status":"paired_receipt_valid"}}"#.write(
            to: bundleRoot.appendingPathComponent("source.report.json"),
            atomically: true,
            encoding: .utf8
        )

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("source report pairing receipt_id mismatch: expected pair-1"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRejectsSourceConsistencyArtifactBaselineEscape() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-consistency-baseline-escape")
        let targetRoot = try makeDirectory(named: "two-machine-evaluate-consistency-baseline-escape-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try writeReadyTargetControlPlane(to: targetRoot)
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "output": "source.consistency.json",
          "baseline": "../outside-baseline.json",
          "status": "pass",
          "mode": "current_source_verified",
          "session_id": "session-1"
        }
        """.write(
            to: bundleRoot.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try "{}".write(
            to: bundleRoot.deletingLastPathComponent().appendingPathComponent("outside-baseline.json"),
            atomically: true,
            encoding: .utf8
        )

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("bundle artifact path contains unsafe traversal: ../outside-baseline.json"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRejectsUnsafePairingReceiptIDControlPlaneSegment() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-unsafe-pairing-id")
        let targetRoot = try makeDirectory(named: "two-machine-evaluate-unsafe-pairing-id-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try writeReadyTargetControlPlane(to: targetRoot)
        try #"{"profile":"/tmp/source.profile.json","target_address":"127.0.0.1:39395","verification_code":"123456","pairing_receipt_id":"../pair-escape","receipt_path":"exported-receipts/pair-1.json"}"#
            .write(to: bundleRoot.appendingPathComponent("source.pair.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("invalid pairing_receipt_id path segment") && result.stderr.contains("../pair-escape"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRejectsPairingReceiptIDWithWhitespaceControlPlaneSegment() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-pairing-id-whitespace")
        let targetRoot = try makeDirectory(named: "two-machine-evaluate-pairing-id-whitespace-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try writeReadyTargetControlPlane(to: targetRoot)
        try writeSourcePairBundleArtifacts(to: bundleRoot, pairingReceiptID: "pair-1\n")

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("invalid pairing_receipt_id path segment") && result.stderr.contains("pair-1"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRejectsUnsafeSessionIDControlPlaneSegment() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-unsafe-session-id")
        let targetRoot = try makeDirectory(named: "two-machine-evaluate-unsafe-session-id-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try writeReadyTargetControlPlane(to: targetRoot)
        try #"{"profile":"/tmp/source.profile.json","session_id":"../session-escape","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#
            .write(to: bundleRoot.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("invalid session_id path segment") && result.stderr.contains("../session-escape"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRejectsSessionIDWithWhitespaceControlPlaneSegment() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-session-id-whitespace")
        let targetRoot = try makeDirectory(named: "two-machine-evaluate-session-id-whitespace-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try writeReadyTargetControlPlane(to: targetRoot)
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "profile": "/tmp/source.profile.json",
            "session_id": "session-1\n",
            "target_address": "127.0.0.1:39395",
            "receiver_address": "127.0.0.1:9443",
            "target_mode": "pairing",
        ]).write(to: bundleRoot.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("invalid session_id path segment") && result.stderr.contains("session-1"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRejectsTargetPairingReceiptDirectory() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-target-pairing-directory")
        let targetRoot = try makeDirectory(named: "two-machine-evaluate-target-pairing-directory-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try writeReadyTargetControlPlane(to: targetRoot)
        let receiptURL = targetRoot.appendingPathComponent(".supermover/pairings/pair-1.json")
        try FileManager.default.removeItem(at: receiptURL)
        try FileManager.default.createDirectory(at: receiptURL, withIntermediateDirectories: true)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("invalid target pairing receipt: .supermover/pairings/pair-1.json"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRejectsMalformedTargetPairingReceiptEvidence() throws {
        let wrongIDReceipt = try targetPairingReceiptJSON(id: "pair-stale")
        let cases: [(name: String, payload: String)] = [
            ("empty-object", "{}"),
            (
                "missing-profile",
                """
                {"version":1,"id":"pair-1","target_id":"target-1","source_device_id":"src-spki","target_device_id":"dst-spki","device_public_key":"dst-spki","method":"sas","verified_at":"2026-06-04T00:00:00Z","protocol_version":"supermover/v1"}
                """
            ),
            (
                "blank-source-device",
                """
                {"version":1,"id":"pair-1","profile_id":"profile-src","target_id":"target-1","source_device_id":"   ","target_device_id":"dst-spki","device_public_key":"dst-spki","method":"sas","verified_at":"2026-06-04T00:00:00Z","protocol_version":"supermover/v1"}
                """
            ),
            ("wrong-id", wrongIDReceipt),
            (
                "string-version",
                """
                {"version":"1","id":"pair-1","profile_id":"profile-src","target_id":"target-1","source_device_id":"src-spki","target_device_id":"dst-spki","device_public_key":"dst-spki","method":"sas","verified_at":"2026-06-04T00:00:00Z","protocol_version":"supermover/v1"}
                """
            ),
            (
                "missing-verification",
                try AcceptanceWorkflowFixtures.pairingReceiptJSON(verificationHash: nil, verificationPhrase: nil)
            ),
            (
                "bad-verified-at",
                try AcceptanceWorkflowFixtures.pairingReceiptJSON(verifiedAt: "not-a-date")
            ),
            (
                "device-public-key-mismatch",
                try AcceptanceWorkflowFixtures.pairingReceiptJSON(devicePublicKey: "other-device")
            ),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-target-receipt-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            try testCase.payload.write(
                to: fixture.targetRoot.appendingPathComponent(".supermover/pairings/pair-1.json"),
                atomically: true,
                encoding: .utf8
            )

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains("invalid target pairing receipt evidence"), testCase.name)
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsMalformedExportedPairingReceiptEvidence() throws {
        let cases: [(name: String, payload: String)] = [
            ("empty-object", "{}"),
            ("wrong-id", try targetPairingReceiptJSON(id: "pair-other")),
            (
                "missing-verification",
                try AcceptanceWorkflowFixtures.pairingReceiptJSON(verificationHash: nil, verificationPhrase: nil)
            ),
            (
                "bad-verified-at",
                try AcceptanceWorkflowFixtures.pairingReceiptJSON(verifiedAt: "not-a-date")
            ),
            (
                "device-public-key-mismatch",
                try AcceptanceWorkflowFixtures.pairingReceiptJSON(devicePublicKey: "other-device")
            ),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-exported-receipt-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            try testCase.payload.write(
                to: fixture.bundleRoot.appendingPathComponent("exported-receipts/pair-1.json"),
                atomically: true,
                encoding: .utf8
            )

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains("invalid exported receipt evidence"), testCase.name)
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsNonStringSourcePairReceiptPath() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-non-string-receipt-path")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }

        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "profile": "/tmp/source.profile.json",
            "target_address": "127.0.0.1:39395",
            "verification_code": "123456",
            "pairing_receipt_id": "pair-1",
            "receipt_path": true,
        ]).write(
            to: fixture.bundleRoot.appendingPathComponent("source.pair.json"),
            atomically: true,
            encoding: .utf8
        )
        try targetPairingReceiptJSON().write(
            to: fixture.bundleRoot.appendingPathComponent("true"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("invalid source pairing receipt artifact: expected string"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRejectsSymlinkedTargetRoot() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-symlinked-target-root")
        let linkedRoot = fixture.workDir.appendingPathComponent("target-root-link")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        do {
            try FileManager.default.createSymbolicLink(at: linkedRoot, withDestinationURL: fixture.targetRoot)
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: linkedRoot)
        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("invalid target root for target pairing receipt"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRejectsTargetNetworkTransferSymlink() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-target-transfer-symlink")
        let targetRoot = try makeDirectory(named: "two-machine-evaluate-target-transfer-symlink-target")
        defer { try? FileManager.default.removeItem(at: workDir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try writeReadyTargetControlPlane(to: targetRoot)
        let transferURL = targetRoot.appendingPathComponent(".supermover/sessions/session-1/network-transfer.json")
        let outside = workDir.appendingPathComponent("outside-network-transfer.json")
        try FileManager.default.removeItem(at: transferURL)
        try #"{"status":"published","stage":"commit","encrypted_transfer":"tls13_mtls","source_device_id":"src-spki","target_device_id":"dst-spki"}"#
            .write(to: outside, atomically: true, encoding: .utf8)
        do {
            try FileManager.default.createSymbolicLink(at: transferURL, withDestinationURL: outside)
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("invalid target network-transfer evidence: .supermover/sessions/session-1/network-transfer.json"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("evaluation.json").path
            )
        )
    }

    func testTwoMachineEvaluateRejectsTargetControlPlaneHardlinks() throws {
        let cases: [(name: String, relativePath: String, payload: String, expectedMessage: String)] = [
            (
                "pairing-receipt",
                ".supermover/pairings/pair-1.json",
                try targetPairingReceiptJSON(),
                "invalid target pairing receipt: .supermover/pairings/pair-1.json"
            ),
            (
                "network-transfer",
                ".supermover/sessions/session-1/network-transfer.json",
                try targetNetworkTransferJSON(),
                "invalid target network-transfer evidence: .supermover/sessions/session-1/network-transfer.json"
            ),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-target-hardlink-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            let controlURL = fixture.targetRoot.appendingPathComponent(testCase.relativePath)
            let outside = fixture.workDir.appendingPathComponent("\(testCase.name).json")
            try testCase.payload.write(to: outside, atomically: true, encoding: .utf8)
            try FileManager.default.removeItem(at: controlURL)
            do {
                try FileManager.default.linkItem(at: outside, to: controlURL)
            } catch {
                throw XCTSkip("hardlink unavailable: \(error)")
            }

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains(testCase.expectedMessage), testCase.name)
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsBundleLocalArtifactHardlinks() throws {
        let cases: [(name: String, relativePath: String, expectedMessage: String)] = [
            ("source-provenance", "source.provenance.json", "invalid source provenance"),
            ("source-verify", "source.verify.json", "invalid source verify artifact"),
            ("source-baseline", "source.baseline.json", "invalid source consistency baseline"),
            ("source-machine-facts", "source.machine.json", "invalid source machine facts"),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-bundle-hardlink-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            let artifactURL = fixture.bundleRoot.appendingPathComponent(testCase.relativePath)
            let outside = fixture.workDir.appendingPathComponent("\(testCase.name).json")
            try FileManager.default.copyItem(at: artifactURL, to: outside)
            try FileManager.default.removeItem(at: artifactURL)
            do {
                try FileManager.default.linkItem(at: outside, to: artifactURL)
            } catch {
                throw XCTSkip("hardlink unavailable: \(error)")
            }

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains(testCase.expectedMessage), testCase.name)
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsLinkedBundleMeta() throws {
        enum LinkKind {
            case hardlink
            case symlink
        }
        let cases: [(name: String, kind: LinkKind)] = [
            ("hardlink", .hardlink),
            ("symlink", .symlink),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-meta-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            let metaURL = fixture.bundleRoot.appendingPathComponent("meta.json")
            let outside = fixture.workDir.appendingPathComponent("outside-meta.json")
            try FileManager.default.copyItem(at: metaURL, to: outside)
            try FileManager.default.removeItem(at: metaURL)
            do {
                switch testCase.kind {
                case .hardlink:
                    try FileManager.default.linkItem(at: outside, to: metaURL)
                case .symlink:
                    try FileManager.default.createSymbolicLink(at: metaURL, withDestinationURL: outside)
                }
            } catch {
                throw XCTSkip("\(testCase.name) unavailable: \(error)")
            }

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains("invalid bundle meta"), testCase.name)
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineWorkflowStatusRejectsLinkedBundleMeta() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-meta-hardlink")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        let metaURL = fixture.bundleRoot.appendingPathComponent("meta.json")
        let outside = fixture.workDir.appendingPathComponent("outside-meta.json")
        try FileManager.default.copyItem(at: metaURL, to: outside)
        try FileManager.default.removeItem(at: metaURL)
        do {
            try FileManager.default.linkItem(at: outside, to: metaURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", fixture.bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("invalid bundle meta"))
        XCTAssertTrue(result.stdout.isEmpty)
    }

    func testTwoMachineWorkflowStatusDoesNotUseHardlinkedMachineFactArtifactsAsProof() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-machine-hardlink")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        let machineFactsURL = fixture.bundleRoot.appendingPathComponent("source.machine.json")
        let outside = fixture.workDir.appendingPathComponent("outside-source-machine.json")
        try FileManager.default.copyItem(at: machineFactsURL, to: outside)
        try FileManager.default.removeItem(at: machineFactsURL)
        do {
            try FileManager.default.linkItem(at: outside, to: machineFactsURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        XCTAssertEqual(status["has_installed_app_machine_pair_proof"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, false)
        XCTAssertEqual(status["machine_facts_consistent"] as? Bool, false)
        XCTAssertEqual(status["requires_machine_identity_correction"] as? Bool, true)
    }

    func testTwoMachineWorkflowStatusDoesNotUseHardlinkedReleaseArtifactsAsReady() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-workflow-status-release-hardlink")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        let auditURL = fixture.bundleRoot.appendingPathComponent("source.app-audit.json")
        let outside = fixture.workDir.appendingPathComponent("outside-source-app-audit.json")
        try FileManager.default.copyItem(at: auditURL, to: outside)
        try FileManager.default.removeItem(at: auditURL)
        do {
            try FileManager.default.linkItem(at: outside, to: auditURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        let status = try runWorkflowStatus(bundleRoot: fixture.bundleRoot)

        XCTAssertEqual(status["source_app_audit_ready"] as? Bool, false)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertTrue(failures.contains("source.app-audit.json is not install-ready"))
    }

    func testTwoMachineEvaluateRejectsMalformedTargetNetworkTransferDeviceIDs() throws {
        let cases: [(name: String, sourceDeviceID: Any?, targetDeviceID: Any?)] = [
            ("missing-source", nil, "dst-spki"),
            ("null-source", NSNull(), "dst-spki"),
            ("number-source", 123, "dst-spki"),
            ("blank-source", "   ", "dst-spki"),
            ("missing-target", "src-spki", nil),
            ("null-target", "src-spki", NSNull()),
            ("number-target", "src-spki", 123),
            ("blank-target", "src-spki", "   "),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-network-transfer-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            try writeNetworkTransferEvidence(
                sourceDeviceID: testCase.sourceDeviceID,
                targetDeviceID: testCase.targetDeviceID,
                targetRoot: fixture.targetRoot
            )

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains("invalid target network-transfer evidence"), testCase.name)
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsMalformedTargetNetworkTransferSchema() throws {
        let cases: [(name: String, payload: String)] = [
            ("missing-session", try targetNetworkTransferJSON(sessionID: nil)),
            ("wrong-session", try targetNetworkTransferJSON(sessionID: "session-other")),
            ("missing-profile", try targetNetworkTransferJSON(profileID: nil)),
            ("missing-target", try targetNetworkTransferJSON(targetID: nil)),
            ("same-devices", try targetNetworkTransferJSON(sourceDeviceID: "same", targetDeviceID: "same")),
            ("missing-protocol", try targetNetworkTransferJSON(protocolVersion: nil)),
            ("bad-started-at", try targetNetworkTransferJSON(startedAt: "not-a-date")),
            ("updated-before-started", try targetNetworkTransferJSON(startedAt: "2026-01-01T00:00:02Z", updatedAt: "2026-01-01T00:00:01Z")),
            ("missing-version", try targetNetworkTransferJSON(version: nil)),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-network-transfer-schema-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            try testCase.payload.write(
                to: fixture.targetRoot.appendingPathComponent(".supermover/sessions/session-1/network-transfer.json"),
                atomically: true,
                encoding: .utf8
            )

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains("invalid target network-transfer evidence"), testCase.name)
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsMalformedSourceVerifyCounts() throws {
        let cases: [(name: String, summary: [String: Any])] = [
            (
                "string-files-verified",
                ["files_verified": "1", "files_expected": 1, "error_findings": 0, "artifact_problems": 0]
            ),
            (
                "nonnumeric-files-verified",
                ["files_verified": "many", "files_expected": 1, "error_findings": 0, "artifact_problems": 0]
            ),
            (
                "bool-files-verified",
                ["files_verified": true, "files_expected": 1, "error_findings": 0, "artifact_problems": 0]
            ),
            (
                "string-error-findings",
                ["files_verified": 1, "files_expected": 1, "error_findings": "0", "artifact_problems": 0]
            ),
            (
                "string-artifact-problems",
                ["files_verified": 1, "files_expected": 1, "error_findings": 0, "artifact_problems": "0"]
            ),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-verify-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            try writeSourceVerifySummary(testCase.summary, bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains("invalid source verify summary evidence"), testCase.name)
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsSourceTransferEvidenceFromDifferentTargetRoot() throws {
        let cases: [(name: String, artifact: String, failure: String)] = [
            (
                "verify",
                "source.verify.json",
                "source verify target_root does not match selected target root"
            ),
            (
                "report",
                "source.report.json",
                "source report target_root does not match selected target root"
            ),
            (
                "status",
                "source.status.json",
                "source status target_root does not match selected target root"
            ),
            (
                "health",
                "source.health.json",
                "source health target_root does not match selected target root"
            ),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-target-root-\(testCase.name)")
            let otherTargetRoot = try makeDirectory(named: "two-machine-evaluate-other-target-root-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            defer { try? FileManager.default.removeItem(at: otherTargetRoot) }
            try rewriteJSONObject(fixture.bundleRoot.appendingPathComponent(testCase.artifact)) { root in
                root["target_root"] = otherTargetRoot.path
            }

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains(testCase.failure), "\(testCase.name) stderr:\n\(result.stderr)")
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsMalformedSourceHealthArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-malformed-source-health")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try #"{"status":"ok"}"#.write(
            to: fixture.bundleRoot.appendingPathComponent("source.health.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("invalid source health artifact"), "stderr:\n\(result.stderr)")
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path))
    }

    func testTwoMachineEvaluateRejectsMalformedSourceStatusArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-malformed-source-status")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try #"{"target_root":"\#(fixture.targetRoot.path)"}"#.write(
            to: fixture.bundleRoot.appendingPathComponent("source.status.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("invalid source status artifact"), "stderr:\n\(result.stderr)")
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path))
    }

    func testTwoMachineEvaluateRejectsNonIntegralOrNegativeSourceStatusCounters() throws {
        let cases: [(name: String, path: [String], value: Any)] = [
            ("negative-files-expected", ["latest_session", "files_expected"], -1),
            ("fractional-files-verified", ["latest_session", "files_verified"], 1.5),
            ("negative-verification-errors", ["latest_session", "verification_errors"], -1),
            ("fractional-artifact-problems", ["counts", "artifact_problems"], 1.5),
            ("negative-network-transfers", ["counts", "network_transfers"], -1),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-source-status-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            try rewriteJSONObject(fixture.bundleRoot.appendingPathComponent("source.status.json")) { root in
                try setNestedJSONValue(testCase.value, path: testCase.path, root: &root)
            }

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains("invalid source status artifact"), "\(testCase.name) stderr:\n\(result.stderr)")
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsNonIntegralOrNegativeSourceHealthCounters() throws {
        let cases: [(name: String, field: String, value: Any)] = [
            ("negative-incomplete-sessions", "incomplete_sessions", -1),
            ("fractional-invalid-records", "invalid_records", 1.5),
            ("negative-artifact-problems", "artifact_problems", -1),
            ("fractional-target-drifts", "target_drifts", 1.5),
            ("negative-network-transfers", "network_transfers", -1),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-source-health-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            try rewriteJSONObject(fixture.bundleRoot.appendingPathComponent("source.health.json")) { root in
                try setNestedJSONValue(testCase.value, path: ["summary", testCase.field], root: &root)
            }

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains("invalid source health artifact"), "\(testCase.name) stderr:\n\(result.stderr)")
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsOutOfRangeSourceStatusAndHealthCounters() throws {
        let cases: [(name: String, artifact: String, path: [String], value: String, stderr: String)] = [
            (
                "status-files-expected-too-large",
                "source.status.json",
                ["latest_session", "files_expected"],
                "9223372036854775808",
                "invalid source status artifact"
            ),
            (
                "status-files-expected-decimal-intmax",
                "source.status.json",
                ["latest_session", "files_expected"],
                "9223372036854775807.0",
                "invalid source status artifact"
            ),
            (
                "status-network-transfers-scientific",
                "source.status.json",
                ["counts", "network_transfers"],
                "1e100",
                "invalid source status artifact"
            ),
            (
                "health-incomplete-sessions-too-large",
                "source.health.json",
                ["summary", "incomplete_sessions"],
                "9223372036854775808",
                "invalid source health artifact"
            ),
            (
                "health-incomplete-sessions-decimal-intmax",
                "source.health.json",
                ["summary", "incomplete_sessions"],
                "9223372036854775807.0",
                "invalid source health artifact"
            ),
            (
                "health-network-transfers-scientific",
                "source.health.json",
                ["summary", "network_transfers"],
                "1e100",
                "invalid source health artifact"
            ),
            (
                "health-network-transfers-intmax-scientific",
                "source.health.json",
                ["summary", "network_transfers"],
                "9.223372036854775807e18",
                "invalid source health artifact"
            ),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-counter-range-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            try rewriteJSONObjectString(
                fixture.bundleRoot.appendingPathComponent(testCase.artifact),
                path: testCase.path,
                rawValue: testCase.value
            )

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

            XCTAssertEqual(result.exitCode, 1, testCase.name)
            XCTAssertTrue(result.stderr.contains(testCase.stderr), "\(testCase.name) stderr:\n\(result.stderr)")
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsMalformedSourceBrowseArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-malformed-source-browse")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try #"{"trusted":false,"candidate_count":0,"candidates":[]}"#.write(
            to: fixture.bundleRoot.appendingPathComponent("source.browse.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("invalid source browse artifact"), "stderr:\n\(result.stderr)")
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path))
    }

    func testTwoMachineEvaluateRejectsMalformedTargetAdvertiseArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-malformed-target-advertise")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try #"{"status":"advertised","trusted":false}"#.write(
            to: fixture.bundleRoot.appendingPathComponent("target.advertise.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("invalid target advertise artifact"), "stderr:\n\(result.stderr)")
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path))
    }

    func testTwoMachineEvaluateRejectsMissingTargetReadyArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-missing-target-ready")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try FileManager.default.removeItem(at: fixture.bundleRoot.appendingPathComponent("target.ready.json"))

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("missing target ready artifact"), "stderr:\n\(result.stderr)")
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path))
    }

    func testTwoMachineEvaluateRejectsMalformedTargetReadyArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-malformed-target-ready")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try #"{"address":"127.0.0.1:39395","mode":"pairing"}"#.write(
            to: fixture.bundleRoot.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("invalid target ready artifact"), "stderr:\n\(result.stderr)")
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path))
    }

    func testTwoMachineEvaluateRejectsSourcePairTargetAddressMismatchWithTargetReady() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-source-pair-target-ready-mismatch")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "profile": "/tmp/source.profile.json",
            "target_address": "127.0.0.1:49999",
            "verification_code": "123456",
            "pairing_receipt_id": "pair-1",
            "receipt_path": "exported-receipts/pair-1.json",
        ]).write(
            to: fixture.bundleRoot.appendingPathComponent("source.pair.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("source pair target address does not match target.ready.json"), "stderr:\n\(result.stderr)")
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path))
    }

    func testTwoMachineEvaluateRejectsSourceTransferReceiverMismatchWithTargetReady() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-source-transfer-target-ready-mismatch")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "profile": "/tmp/source.profile.json",
            "session_id": "session-1",
            "target_address": "127.0.0.1:39395",
            "receiver_address": "127.0.0.1:9555",
            "target_mode": "pairing",
        ]).write(
            to: fixture.bundleRoot.appendingPathComponent("source.transfer.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("source transfer receiver does not match target.ready.json"), "stderr:\n\(result.stderr)")
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path))
    }

    func testTwoMachineEvaluateRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-target-ready-lacks-receiver-transfer")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "address": "127.0.0.1:39395",
            "verification_code": "123456",
            "mode": "pairing",
            "receiver_routes": false,
            "push_network": false,
            "trusted": false,
            "transfer": false,
        ]).write(
            to: fixture.bundleRoot.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("target ready artifact does not prove receiver transfer readiness"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path))
    }

    func testTwoMachineEvaluateRejectsMissingReferencedTargetImportArtifact() throws {
        let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-missing-target-import-artifact")
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }
        defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
        try FileManager.default.removeItem(at: fixture.bundleRoot.appendingPathComponent("target.adopt-pairing.txt"))

        let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(result.stderr.contains("missing target_import adopted artifact"), "stderr:\n\(result.stderr)")
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path))
    }

    func testTwoMachineEvaluateRejectsControlCharacterControlPlaneIDsBeforePathUse() throws {
        let cases: [(name: String, pairingReceiptID: String?, sessionID: String?)] = [
            ("pair-nul", "pair-1\u{0000}", nil),
            ("pair-bel", "pair-1\u{0007}", nil),
            ("session-nul", nil, "session-1\u{0000}"),
            ("session-bel", nil, "session-1\u{0007}"),
        ]

        for testCase in cases {
            let fixture = try makeReadyTwoMachineFixture(named: "two-machine-evaluate-control-id-\(testCase.name)")
            defer { try? FileManager.default.removeItem(at: fixture.workDir) }
            defer { try? FileManager.default.removeItem(at: fixture.targetRoot) }
            if let pairingReceiptID = testCase.pairingReceiptID {
                try writeSourcePairBundleArtifacts(to: fixture.bundleRoot, pairingReceiptID: pairingReceiptID)
            }
            if let sessionID = testCase.sessionID {
                try AcceptanceReleaseEvidenceFixtures.jsonString([
                    "profile": "/tmp/source.profile.json",
                    "session_id": sessionID,
                    "target_address": "127.0.0.1:39395",
                    "receiver_address": "127.0.0.1:9443",
                    "target_mode": "pairing",
                ]).write(to: fixture.bundleRoot.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
            }

            let result = try runEvaluate(bundleRoot: fixture.bundleRoot, targetRoot: fixture.targetRoot)
            XCTAssertEqual(result.exitCode, 1, testCase.name)
            let label = testCase.pairingReceiptID == nil ? "session_id" : "pairing_receipt_id"
            XCTAssertTrue(result.stderr.contains("invalid \(label) path segment"), testCase.name)
            XCTAssertFalse(
                FileManager.default.fileExists(
                    atPath: fixture.bundleRoot.appendingPathComponent("evaluation.json").path
                ),
                testCase.name
            )
        }
    }

    func testTwoMachineEvaluateRejectsContradictoryVerifiedHandoffsEvenWhenOneMatchesRecordedMachinePair() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-contradictory-verified-handoffs")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let controlPlane = targetRoot.appendingPathComponent(".supermover", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("pairings"), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("sessions/session-1"), withIntermediateDirectories: true)

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
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
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
                "archive": "bundle.other.tgz",
                "manifest": "bundle.other.manifest.json",
                "sha256": "2222222222222222222222222222222222222222222222222222222222222222",
                "meta": "meta.other.json",
                "verified": true,
                "exporting_machine_id": "third-source-machine",
                "exporting_machine_label": "other-source",
                "importing_machine_id": "third-target-machine",
                "importing_machine_label": "other-target"
              }
            ],
            "source_pair": {
              "output": "source.pair.json",
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "pair": "source.pair.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "verify": "source.verify.json",
              "status": "source.status.json",
              "report": "source.report.json",
              "health": "source.health.json",
              "push": "source.network-push.txt"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "operator": {
              "local_network": {"status":"pass","detail":"ok"},
              "firewall": {"status":"pass","detail":"ok"},
              "pairing_confirmation": {"status":"pass","detail":"ok"}
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"source-machine","machine_label":"source"}"#
            .write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"target-machine","machine_label":"target"}"#
            .write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeTargetReadyBundleArtifact(to: bundleRoot)
        try writeSourcePairBundleArtifacts(to: bundleRoot)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#
            .write(to: bundleRoot.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted\n".write(to: bundleRoot.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok\n".write(to: bundleRoot.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#
            .write(to: bundleRoot.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)
        try #"{"baseline":"current"}"#.write(to: bundleRoot.appendingPathComponent("source.baseline.json"), atomically: true, encoding: .utf8)
        try #"{"summary":{"files_verified":1,"files_expected":1,"error_findings":0,"artifact_problems":0},"manifest":{"manifestID":"manifest-1"},"target_root":"\#(targetRoot.path)","session_id":"session-1","merkleRootProof":{"detail":"unavailable"}}"#
            .write(to: bundleRoot.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.reportJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.statusJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.healthJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try targetPairingReceiptJSON()
            .write(to: controlPlane.appendingPathComponent("pairings/pair-1.json"), atomically: true, encoding: .utf8)
        try targetNetworkTransferJSON()
            .write(to: controlPlane.appendingPathComponent("sessions/session-1/network-transfer.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains(
                "bundle_handoffs contain verified cross-machine archive handoff evidence for machine ids other than the recorded source/target pair"
            ),
            "stderr:\n\(result.stderr)"
        )
    }

    func testTwoMachineEvaluateRejectsRoleMatchedHandoffWhenMetaMachineFactsDisagree() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-meta-machine-facts-mismatch")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let controlPlane = targetRoot.appendingPathComponent(".supermover", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("pairings"), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("sessions/session-1"), withIntermediateDirectories: true)

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
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"other-source-machine","machine_label":"other-source"},
              "target": {"output":"target.machine.json","machine_id":"other-target-machine","machine_label":"other-target"}
            },
            "bundle_handoffs": [
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
            ],
            "source_pair": {
              "output": "source.pair.json",
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "pair": "source.pair.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "verify": "source.verify.json",
              "status": "source.status.json",
              "report": "source.report.json",
              "health": "source.health.json",
              "push": "source.network-push.txt"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "operator": {
              "local_network": {"status":"pass","detail":"ok"},
              "firewall": {"status":"pass","detail":"ok"},
              "pairing_confirmation": {"status":"pass","detail":"ok"}
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"other-source-machine","machine_label":"other-source"}"#
            .write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"other-target-machine","machine_label":"other-target"}"#
            .write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeTargetReadyBundleArtifact(to: bundleRoot)
        try writeSourcePairBundleArtifacts(to: bundleRoot)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#
            .write(to: bundleRoot.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted\n".write(to: bundleRoot.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok\n".write(to: bundleRoot.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#
            .write(to: bundleRoot.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)
        try #"{"baseline":"current"}"#.write(to: bundleRoot.appendingPathComponent("source.baseline.json"), atomically: true, encoding: .utf8)
        try #"{"summary":{"files_verified":1,"files_expected":1,"error_findings":0,"artifact_problems":0},"manifest":{"manifestID":"manifest-1"},"target_root":"\#(targetRoot.path)","session_id":"session-1","merkleRootProof":{"detail":"unavailable"}}"#
            .write(to: bundleRoot.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.reportJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.statusJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.healthJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try targetPairingReceiptJSON()
            .write(to: controlPlane.appendingPathComponent("pairings/pair-1.json"), atomically: true, encoding: .utf8)
        try targetNetworkTransferJSON()
            .write(to: controlPlane.appendingPathComponent("sessions/session-1/network-transfer.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains(
                Self.machineIdentityCorrectionFailureMessage
            ),
            "stderr:\n\(result.stderr)"
        )
    }

    func testTwoMachineEvaluateRejectsAlternateMetaMachineFactOutputsThatMaskCanonicalMismatch() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-selected-machine-fact-output-mismatch")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let controlPlane = targetRoot.appendingPathComponent(".supermover", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("pairings"), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("sessions/session-1"), withIntermediateDirectories: true)

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
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.selected.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.selected.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
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
            ],
            "source_pair": {
              "output": "source.pair.json",
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "pair": "source.pair.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "verify": "source.verify.json",
              "status": "source.status.json",
              "report": "source.report.json",
              "health": "source.health.json",
              "push": "source.network-push.txt"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "operator": {
              "local_network": {"status":"pass","detail":"ok"},
              "firewall": {"status":"pass","detail":"ok"},
              "pairing_confirmation": {"status":"pass","detail":"ok"}
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.json",
            machineID: "other-source-machine",
            machineLabel: "other-source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "target.machine.json",
            machineID: "other-target-machine",
            machineLabel: "other-target"
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
        try writeTargetReadyBundleArtifact(to: bundleRoot)
        try writeSourcePairBundleArtifacts(to: bundleRoot)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#
            .write(to: bundleRoot.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted\n".write(to: bundleRoot.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok\n".write(to: bundleRoot.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#
            .write(to: bundleRoot.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)
        try #"{"baseline":"current"}"#.write(to: bundleRoot.appendingPathComponent("source.baseline.json"), atomically: true, encoding: .utf8)
        try #"{"summary":{"files_verified":1,"files_expected":1,"error_findings":0,"artifact_problems":0},"manifest":{"manifestID":"manifest-1"},"target_root":"\#(targetRoot.path)","session_id":"session-1","merkleRootProof":{"detail":"unavailable"}}"#
            .write(to: bundleRoot.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.reportJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.statusJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.healthJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try targetPairingReceiptJSON()
            .write(to: controlPlane.appendingPathComponent("pairings/pair-1.json"), atomically: true, encoding: .utf8)
        try targetNetworkTransferJSON()
            .write(to: controlPlane.appendingPathComponent("sessions/session-1/network-transfer.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains(
                Self.machineIdentityCorrectionFailureMessage
            ),
            "stderr:\n\(result.stderr)"
        )
    }

    func testTwoMachineEvaluateRejectsMachineFactArtifactMismatch() throws {
        let workDir = try makeDirectory(named: "two-machine-evaluate-machine-fact-artifact-mismatch")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let controlPlane = targetRoot.appendingPathComponent(".supermover", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("pairings"), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("sessions/session-1"), withIntermediateDirectories: true)

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
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
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
            ],
            "source_pair": {
              "output": "source.pair.json",
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "pair": "source.pair.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "verify": "source.verify.json",
              "status": "source.status.json",
              "report": "source.report.json",
              "health": "source.health.json",
              "push": "source.network-push.txt"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "operator": {
              "local_network": {"status":"pass","detail":"ok"},
              "firewall": {"status":"pass","detail":"ok"},
              "pairing_confirmation": {"status":"pass","detail":"ok"}
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"other-source-machine","machine_label":"other-source"}"#
            .write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.machine_facts.v1","machine_id":"other-target-machine","machine_label":"other-target"}"#
            .write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeTargetReadyBundleArtifact(to: bundleRoot)
        try writeSourcePairBundleArtifacts(to: bundleRoot)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#
            .write(to: bundleRoot.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted\n".write(to: bundleRoot.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok\n".write(to: bundleRoot.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#
            .write(to: bundleRoot.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)
        try #"{"baseline":"current"}"#.write(to: bundleRoot.appendingPathComponent("source.baseline.json"), atomically: true, encoding: .utf8)
        try #"{"summary":{"files_verified":1,"files_expected":1,"error_findings":0,"artifact_problems":0},"manifest":{"manifestID":"manifest-1"},"target_root":"\#(targetRoot.path)","session_id":"session-1","merkleRootProof":{"detail":"unavailable"}}"#
            .write(to: bundleRoot.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.reportJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.statusJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.healthJSON(targetRoot: targetRoot.path)
            .write(to: bundleRoot.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try targetPairingReceiptJSON()
            .write(to: controlPlane.appendingPathComponent("pairings/pair-1.json"), atomically: true, encoding: .utf8)
        try targetNetworkTransferJSON()
            .write(to: controlPlane.appendingPathComponent("sessions/session-1/network-transfer.json"), atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains(
                Self.machineIdentityCorrectionFailureMessage
            ),
            "stderr:\n\(result.stderr)"
        )
    }

    func testTwoMachineUnpackBundleFailsClosedOnMalformedArchive() throws {
        let workDir = try makeDirectory(named: "two-machine-unpack-malformed-archive")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let archivePath = workDir.appendingPathComponent("bad-bundle.tgz")
        try "not a tar archive\n".write(to: archivePath, atomically: true, encoding: .utf8)
        let manifestPath = workDir.appendingPathComponent("bad-bundle.manifest.json")
        try """
        {
          "schema": "supermover.acceptance.bundle_archive.v1",
          "archive": "bad-bundle.tgz",
          "sha256": "ec9bd04b698bd732a96d65ce9bd4cd4b06fe0b817bd5cf84227d53a086cb95fc",
          "meta": "meta.json",
          "bundle_status": "in_progress",
          "collection_mode": "two_machine",
          "machine_count": 2,
          "exporting_machine_id": "source-machine",
          "exporting_machine_label": "source"
        }
        """.write(to: manifestPath, atomically: true, encoding: .utf8)
        let unpackedRoot = workDir.appendingPathComponent("unpacked-bundle", isDirectory: true)

        let repoRoot = repoRootURL()
        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "unpack-bundle",
                "--archive", archivePath.path,
                "--manifest", manifestPath.path,
                "--bundle-root", unpackedRoot.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("failed to unpack acceptance bundle archive"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: unpackedRoot.appendingPathComponent("meta.json").path))
    }

    func testTwoMachineUnpackBundlePreservesExistingBundleWhenArchiveMissingExportIdentity() throws {
        let workDir = try makeDirectory(named: "two-machine-unpack-preserves-existing-bundle")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let archiveRoot = workDir.appendingPathComponent("archive-root", isDirectory: true)
        try FileManager.default.createDirectory(at: archiveRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: archiveRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let unpackedRoot = workDir.appendingPathComponent("unpacked-bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: unpackedRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "existing",
          "evidence": {}
        }
        """.write(to: unpackedRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try "existing\n".write(
            to: unpackedRoot.appendingPathComponent("existing-marker.txt"),
            atomically: true,
            encoding: .utf8
        )

        let archivePath = workDir.appendingPathComponent("bundle.tgz")
        let manifestPath = workDir.appendingPathComponent("bundle.manifest.json")
        let repoRoot = repoRootURL()
        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/usr/bin/tar"),
            arguments: ["-C", archiveRoot.path, "-czf", archivePath.path, "."],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        let digestResult = try runProcess(
            executableURL: URL(fileURLWithPath: "/usr/bin/shasum"),
            arguments: ["-a", "256", archivePath.path],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        let digest = try XCTUnwrap(digestResult.stdout.split(separator: " ").first).description
        try """
        {
          "schema": "supermover.acceptance.bundle_archive.v1",
          "archive": "bundle.tgz",
          "sha256": "\(digest)",
          "meta": "meta.json",
          "bundle_status": "in_progress",
          "collection_mode": "two_machine",
          "machine_count": 2,
          "exporting_machine_id": "source-machine",
          "exporting_machine_label": "source"
        }
        """.write(to: manifestPath, atomically: true, encoding: .utf8)

        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "unpack-bundle",
                "--archive", archivePath.path,
                "--manifest", manifestPath.path,
                "--bundle-root", unpackedRoot.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_ID": "target-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_LABEL": "target",
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("missing export identity artifact"))
        let preservedMeta = try String(contentsOf: unpackedRoot.appendingPathComponent("meta.json"), encoding: .utf8)
        XCTAssertTrue(preservedMeta.contains(#""status": "existing""#), "meta.json:\n\(preservedMeta)")
        XCTAssertTrue(FileManager.default.fileExists(atPath: unpackedRoot.appendingPathComponent("existing-marker.txt").path))
        let leftoverStages = try FileManager.default.contentsOfDirectory(atPath: workDir.path)
            .filter { $0.hasPrefix(".supermover-unpack.") }
        XCTAssertEqual(leftoverStages, [])
    }

    func testTwoMachineUnpackBundleFailsClosedOnArchiveDigestMismatch() throws {
        let workDir = try makeDirectory(named: "two-machine-unpack-digest-mismatch")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try "original\n".write(to: bundleRoot.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)
        try writeMachineFactsArtifact(
            bundleRoot: bundleRoot,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )

        let archivePath = workDir.appendingPathComponent("bundle.tgz")
        let manifestPath = workDir.appendingPathComponent("bundle.manifest.json")
        let unpackedRoot = workDir.appendingPathComponent("unpacked-bundle", isDirectory: true)

        let repoRoot = repoRootURL()
        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "pack-bundle",
                "--bundle-root", bundleRoot.path,
                "--archive", archivePath.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_ID": "source-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_EXPORT_MACHINE_LABEL": "source",
            ],
            currentDirectoryURL: repoRoot
        )

        try "tampered archive bytes\n".write(to: archivePath, atomically: true, encoding: .utf8)
        XCTAssertTrue(FileManager.default.fileExists(atPath: manifestPath.path))

        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "unpack-bundle",
                "--archive", archivePath.path,
                "--manifest", manifestPath.path,
                "--bundle-root", unpackedRoot.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("acceptance bundle archive digest mismatch"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: unpackedRoot.appendingPathComponent("meta.json").path))
    }

    func testTwoMachineUnpackBundleRejectsSymlinkedArchiveEntriesBeforePublish() throws {
        let fixture = try makeUnsafeArchiveFixture(named: "two-machine-unpack-symlink-entry") { archiveRoot in
            do {
                try FileManager.default.createSymbolicLink(
                    at: archiveRoot.appendingPathComponent("source.pair.json"),
                    withDestinationURL: URL(fileURLWithPath: "/tmp/supermover-archive-escape.json")
                )
            } catch {
                throw XCTSkip("symlink unavailable: \(error)")
            }
        }
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }

        let result = try unpackUnsafeArchiveFixture(fixture)

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe acceptance bundle archive entry"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.unpackedRoot.appendingPathComponent("meta.json").path))
        let leftoverStages = try FileManager.default.contentsOfDirectory(atPath: fixture.workDir.path)
            .filter { $0.hasPrefix(".supermover-unpack.") }
        XCTAssertEqual(leftoverStages, [])
    }

    func testTwoMachineUnpackBundleRejectsHardlinkedArchiveEntriesBeforePublish() throws {
        let fixture = try makeUnsafeArchiveFixture(named: "two-machine-unpack-hardlink-entry") { archiveRoot in
            let original = archiveRoot.appendingPathComponent("source.pair.original.json")
            let hardlink = archiveRoot.appendingPathComponent("source.pair.json")
            try #"{"pair":"original"}"#.write(to: original, atomically: true, encoding: .utf8)
            do {
                try FileManager.default.linkItem(at: original, to: hardlink)
            } catch {
                throw XCTSkip("hardlink unavailable: \(error)")
            }
        }
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }

        let result = try unpackUnsafeArchiveFixture(fixture)

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe acceptance bundle archive entry"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.unpackedRoot.appendingPathComponent("meta.json").path))
        let leftoverStages = try FileManager.default.contentsOfDirectory(atPath: fixture.workDir.path)
            .filter { $0.hasPrefix(".supermover-unpack.") }
        XCTAssertEqual(leftoverStages, [])
    }

    func testTwoMachineUnpackBundleRejectsSpecialArchiveEntriesBeforePublish() throws {
        let fixture = try makeUnsafeArchiveFixture(named: "two-machine-unpack-special-entry") { archiveRoot in
            let fifo = archiveRoot.appendingPathComponent("source.pair.json")
            let result = try runProcessAllowFailure(
                executableURL: URL(fileURLWithPath: "/usr/bin/mkfifo"),
                arguments: [fifo.path],
                environment: [:],
                currentDirectoryURL: archiveRoot
            )
            if result.exitCode != 0 {
                throw XCTSkip("mkfifo unavailable: \(result.stderr)")
            }
        }
        defer { try? FileManager.default.removeItem(at: fixture.workDir) }

        let result = try unpackUnsafeArchiveFixture(fixture)

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe acceptance bundle archive entry"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: fixture.unpackedRoot.appendingPathComponent("meta.json").path))
        let leftoverStages = try FileManager.default.contentsOfDirectory(atPath: fixture.workDir.path)
            .filter { $0.hasPrefix(".supermover-unpack.") }
        XCTAssertEqual(leftoverStages, [])
    }

    private func makeUnsafeArchiveFixture(
        named name: String,
        addUnsafeEntry: (URL) throws -> Void
    ) throws -> (workDir: URL, archivePath: URL, manifestPath: URL, unpackedRoot: URL) {
        let workDir = try makeDirectory(named: name)

        let archiveRoot = workDir.appendingPathComponent("archive-root", isDirectory: true)
        try FileManager.default.createDirectory(at: archiveRoot, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: archiveRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.bundle_export_identity.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: archiveRoot.appendingPathComponent("__supermover-bundle-export-identity.json"), atomically: true, encoding: .utf8)
        try addUnsafeEntry(archiveRoot)

        let archivePath = workDir.appendingPathComponent("bundle.tgz")
        let manifestPath = workDir.appendingPathComponent("bundle.manifest.json")
        let repoRoot = repoRootURL()
        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/usr/bin/tar"),
            arguments: ["-C", archiveRoot.path, "-czf", archivePath.path, "."],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        let digestResult = try runProcess(
            executableURL: URL(fileURLWithPath: "/usr/bin/shasum"),
            arguments: ["-a", "256", archivePath.path],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        let digest = try XCTUnwrap(digestResult.stdout.split(separator: " ").first).description
        try """
        {
          "schema": "supermover.acceptance.bundle_archive.v1",
          "archive": "bundle.tgz",
          "sha256": "\(digest)",
          "meta": "meta.json",
          "bundle_status": "in_progress",
          "collection_mode": "two_machine",
          "machine_count": 2,
          "exporting_machine_id": "source-machine",
          "exporting_machine_label": "source"
        }
        """.write(to: manifestPath, atomically: true, encoding: .utf8)

        let unpackedRoot = workDir.appendingPathComponent("unpacked-bundle", isDirectory: true)
        return (workDir, archivePath, manifestPath, unpackedRoot)
    }

    private func unpackUnsafeArchiveFixture(
        _ fixture: (workDir: URL, archivePath: URL, manifestPath: URL, unpackedRoot: URL)
    ) throws -> ProcessResult {
        let repoRoot = repoRootURL()
        return try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "unpack-bundle",
                "--archive", fixture.archivePath.path,
                "--manifest", fixture.manifestPath.path,
                "--bundle-root", fixture.unpackedRoot.path,
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_ID": "target-machine",
                "SUPERMOVER_ACCEPTANCE_BUNDLE_IMPORT_MACHINE_LABEL": "target",
            ],
            currentDirectoryURL: repoRoot
        )
    }

    func testTwoMachineTargetAdvertiseWritesMachineFactsArtifact() throws {
        let workDir = try makeDirectory(named: "two-machine-machine-facts")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)

        let targetAppDir = workDir.appendingPathComponent("Target.app", isDirectory: true)
        let resources = targetAppDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resources.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        let cliURL = binDir.appendingPathComponent("supermover")
        try """
        #!/bin/sh
        cmd1=${1:-}
        cmd2=${2:-}
        cmd3=${3:-}
        case "$cmd1" in
          version)
            printf 'supermover target-cli\\n'
            ;;
          discover)
            if [ "$cmd2" = "advertise" ] && [ "$cmd3" = "--help" ]; then
              printf 'Usage: discover advertise --help\\n-profile string\\n'
            elif [ "$cmd2" = "advertise" ]; then
              cat <<'EOF'
        {
          "status": "advertised",
          "trusted": false,
          "listen": "127.0.0.1:42000",
          "destination": "127.0.0.1:42001",
          "capability_flags": ["pairing"]
        }
        EOF
            else
              exit 9
            fi
            ;;
          *)
            exit 9
            ;;
        esac
        """.write(to: cliURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cliURL.path)

        let notaryDir = workDir.appendingPathComponent("Target.app.notary", isDirectory: true)
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)
        let provenanceManifest = try writeCurrentAppSidecarReleaseEvidence(
            appDir: targetAppDir,
            sidecarDir: notaryDir,
            cliVersion: "supermover target-cli"
        )

        let fakeAuditScript = workDir.appendingPathComponent("fake-audit.sh")
        try makeCurrentAuditScript(
            appPath: targetAppDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: fakeAuditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditScript.path)

        let targetProfile = workDir.appendingPathComponent("target.profile.json")
        try "{}\n".write(to: targetProfile, atomically: true, encoding: .utf8)

        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-advertise",
                "--profile", targetProfile.path,
                "--bundle-root", bundleRoot.path,
                "--listen", "127.0.0.1:0",
                "--dest", "127.0.0.1:42001",
                "--duration", "1s",
                "--interval", "200ms",
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_TARGET_APP_DIR": targetAppDir.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
                "SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_ID": "target-machine-01",
                "SUPERMOVER_ACCEPTANCE_TARGET_MACHINE_LABEL": "target-lab",
                "SUPERMOVER_ACCEPTANCE_COLLECTION_MODE": "same_machine",
                "SUPERMOVER_ACCEPTANCE_MACHINE_COUNT": "1",
            ],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(result.stderr, "")

        let machineArtifact = try String(contentsOf: bundleRoot.appendingPathComponent("target.machine.json"))
        XCTAssertTrue(machineArtifact.contains("\"schema\": \"supermover.acceptance.machine_facts.v1\""))
        XCTAssertTrue(machineArtifact.contains("\"machine_id\": \"target-machine-01\""))
        XCTAssertTrue(machineArtifact.contains("\"machine_label\": \"target-lab\""))

        let metaData = try Data(contentsOf: bundleRoot.appendingPathComponent("meta.json"))
        let meta = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        let evidence = meta["evidence"] as? [String: Any] ?? [:]
        let machineFacts = (evidence["machine_facts"] as? [String: Any])?["target"] as? [String: Any]
        XCTAssertEqual(machineFacts?["output"] as? String, "target.machine.json")
        XCTAssertEqual(machineFacts?["machine_id"] as? String, "target-machine-01")
        XCTAssertEqual(machineFacts?["machine_label"] as? String, "target-lab")
    }

    func testTwoMachineTargetAdvertiseHonorsConfiguredCopiedAppSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let workDir = try makeDirectory(named: "two-machine-advertise-copied-app")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let copiedAppURL = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: copiedAppURL)
        let copiedCLI = copiedAppURL.appendingPathComponent("Contents/Resources/bin/supermover")
        let fakeAuditScript = workDir.appendingPathComponent("fake-audit.sh")
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
        """.write(to: fakeAuditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditScript.path)

        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let sourceRoot = workDir.appendingPathComponent("source-root", isDirectory: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sourceRoot, withIntermediateDirectories: true)
        let profile = workDir.appendingPathComponent("target.profile.json")

        let initResult = try runProcess(
            executableURL: copiedCLI,
            arguments: [
                "profile", "init",
                "--profile", profile.path,
                "--source", sourceRoot.path,
                "--target", targetRoot.path,
                "--id", "two-machine-advertise",
                "--name", "Two Machine Advertise",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(initResult.stderr, "")

        let blockedBundleRoot = workDir.appendingPathComponent("bundle-blocked", isDirectory: true)
        try FileManager.default.createDirectory(at: blockedBundleRoot, withIntermediateDirectories: true)
        let blocked = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-advertise",
                "--profile", profile.path,
                "--bundle-root", blockedBundleRoot.path,
                "--listen", "127.0.0.1:0",
                "--dest", "127.0.0.1:39394",
                "--duration", "1s",
                "--interval", "200ms",
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(blocked.exitCode, 5)
        XCTAssertTrue(blocked.stderr.contains("requires release-ready target notarization evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: blockedBundleRoot.appendingPathComponent("target.notarization.json").path))

        let copiedSidecarDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
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

        let passingBundleRoot = workDir.appendingPathComponent("bundle-pass", isDirectory: true)
        try FileManager.default.createDirectory(at: passingBundleRoot, withIntermediateDirectories: true)
        let passing = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-advertise",
                "--profile", profile.path,
                "--bundle-root", passingBundleRoot.path,
                "--listen", "127.0.0.1:0",
                "--dest", "127.0.0.1:39394",
                "--duration", "1s",
                "--interval", "200ms",
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(passing.stderr, "")
        XCTAssertTrue(FileManager.default.fileExists(atPath: passingBundleRoot.appendingPathComponent("target.notarization.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: passingBundleRoot.appendingPathComponent("target.advertise.json").path))
    }

    func testTwoMachineTargetAdvertiseClearsStaleNotarizationEvidenceAfterCopiedSidecarDisappearsWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let workDir = try makeDirectory(named: "two-machine-advertise-stale-sidecar")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let copiedAppURL = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: copiedAppURL)
        let copiedCLI = copiedAppURL.appendingPathComponent("Contents/Resources/bin/supermover")
        let fakeAuditScript = workDir.appendingPathComponent("fake-audit.sh")
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
        """.write(to: fakeAuditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditScript.path)

        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let sourceRoot = workDir.appendingPathComponent("source-root", isDirectory: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sourceRoot, withIntermediateDirectories: true)
        let profile = workDir.appendingPathComponent("target.profile.json")

        let initResult = try runProcess(
            executableURL: copiedCLI,
            arguments: [
                "profile", "init",
                "--profile", profile.path,
                "--source", sourceRoot.path,
                "--target", targetRoot.path,
                "--id", "two-machine-advertise-stale",
                "--name", "Two Machine Advertise Stale",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(initResult.stderr, "")

        let copiedSidecarDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
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

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)

        let firstPass = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-advertise",
                "--profile", profile.path,
                "--bundle-root", bundleRoot.path,
                "--listen", "127.0.0.1:0",
                "--dest", "127.0.0.1:39394",
                "--duration", "1s",
                "--interval", "200ms",
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
            ],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(firstPass.stderr, "")
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))

        try FileManager.default.removeItem(at: copiedSidecarDir.appendingPathComponent("notarization.json"))

        let blocked = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-advertise",
                "--profile", profile.path,
                "--bundle-root", bundleRoot.path,
                "--listen", "127.0.0.1:0",
                "--dest", "127.0.0.1:39395",
                "--duration", "1s",
                "--interval", "200ms",
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(blocked.exitCode, 5)
        XCTAssertTrue(blocked.stderr.contains("requires release-ready target notarization evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))

        let data = try Data(contentsOf: bundleRoot.appendingPathComponent("meta.json"))
        let document = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let evidence = document["evidence"] as? [String: Any] ?? [:]
        let notarization = evidence["notarization"] as? [String: Any]
        XCTAssertNil(notarization?["target"])
    }

    func testTwoMachineTargetAdvertiseAcceptsNotarizeScriptWorkflowSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let workDir = try makeDirectory(named: "two-machine-advertise-workflow-sidecar")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let copiedAppURL = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: copiedAppURL)
        let copiedCLI = copiedAppURL.appendingPathComponent("Contents/Resources/bin/supermover")

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

        let sidecarURL = copiedAppURL.deletingLastPathComponent().appendingPathComponent("\(copiedAppURL.lastPathComponent).notary/notarization.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarURL.path))

        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let sourceRoot = workDir.appendingPathComponent("source-root", isDirectory: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sourceRoot, withIntermediateDirectories: true)
        let profile = workDir.appendingPathComponent("target.profile.json")

        let initResult = try runProcess(
            executableURL: copiedCLI,
            arguments: [
                "profile", "init",
                "--profile", profile.path,
                "--source", sourceRoot.path,
                "--target", targetRoot.path,
                "--id", "two-machine-advertise-workflow-sidecar",
                "--name", "Two Machine Advertise Workflow Sidecar",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(initResult.stderr, "")

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-advertise",
                "--profile", profile.path,
                "--bundle-root", bundleRoot.path,
                "--listen", "127.0.0.1:0",
                "--dest", "127.0.0.1:39394",
                "--duration", "1s",
                "--interval", "200ms",
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.stderr, "")
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.advertise.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.targetNotarization?.output, "target.notarization.json")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.status, "pass")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.submission?.status, "Accepted")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.audit?.readiness, "distribution_ready")
    }

    func testTwoMachineTargetAdvertiseAcceptsCanonicalBuiltAppWorkflowSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let sidecarURL = builtAppURL.deletingLastPathComponent().appendingPathComponent("\(builtAppURL.lastPathComponent).notary/notarization.json")
        if FileManager.default.fileExists(atPath: sidecarURL.path) {
            throw XCTSkip("canonical built app already has sidecar at \(sidecarURL.path); workflow-sidecar injection case is not applicable")
        }
        defer {
            try? FileManager.default.removeItem(at: sidecarURL.deletingLastPathComponent())
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

        let notarize = try runNotarizeScript(
            arguments: [
                "--app", builtAppURL.path,
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

        let workDir = try makeDirectory(named: "two-machine-advertise-canonical-workflow-sidecar")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let builtCLI = builtAppURL.appendingPathComponent("Contents/Resources/bin/supermover")
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let sourceRoot = workDir.appendingPathComponent("source-root", isDirectory: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sourceRoot, withIntermediateDirectories: true)
        let profile = workDir.appendingPathComponent("target.profile.json")

        let initResult = try runProcess(
            executableURL: builtCLI,
            arguments: [
                "profile", "init",
                "--profile", profile.path,
                "--source", sourceRoot.path,
                "--target", targetRoot.path,
                "--id", "two-machine-advertise-canonical-workflow-sidecar",
                "--name", "Two Machine Advertise Canonical Workflow Sidecar",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(initResult.stderr, "")

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)

        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-advertise",
                "--profile", profile.path,
                "--bundle-root", bundleRoot.path,
                "--listen", "127.0.0.1:0",
                "--dest", "127.0.0.1:39394",
                "--duration", "1s",
                "--interval", "200ms",
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.stderr, "")
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.advertise.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.targetNotarization?.output, "target.notarization.json")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.status, "pass")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.submission?.status, "Accepted")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.audit?.readiness, "distribution_ready")
    }

    func testTwoMachineTargetAdvertiseRejectsMalformedCopiedSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let workDir = try makeDirectory(named: "two-machine-advertise-malformed-sidecar")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let copiedAppURL = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        try FileManager.default.copyItem(at: builtAppURL, to: copiedAppURL)
        let copiedCLI = copiedAppURL.appendingPathComponent("Contents/Resources/bin/supermover")
        let fakeAuditScript = workDir.appendingPathComponent("fake-audit.sh")
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
        """.write(to: fakeAuditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: fakeAuditScript.path)

        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let sourceRoot = workDir.appendingPathComponent("source-root", isDirectory: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sourceRoot, withIntermediateDirectories: true)
        let profile = workDir.appendingPathComponent("target.profile.json")

        let initResult = try runProcess(
            executableURL: copiedCLI,
            arguments: [
                "profile", "init",
                "--profile", profile.path,
                "--source", sourceRoot.path,
                "--target", targetRoot.path,
                "--id", "two-machine-advertise-malformed",
                "--name", "Two Machine Advertise Malformed",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(initResult.stderr, "")

        let copiedSidecarDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        try FileManager.default.createDirectory(at: copiedSidecarDir, withIntermediateDirectories: true)
        try #"{"schema":"bad"}"#.write(to: copiedSidecarDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try #"{"schema":"stale"}"#.write(to: bundleRoot.appendingPathComponent("target.notarization.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {
            "notarization": {
              "target": {
                "collected_by": "stale",
                "output": "target.notarization.json",
                "status": "pass"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-advertise",
                "--profile", profile.path,
                "--bundle-root", bundleRoot.path,
                "--listen", "127.0.0.1:0",
                "--dest", "127.0.0.1:39394",
                "--duration", "1s",
                "--interval", "200ms",
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_APP_DIR": copiedAppURL.path,
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": fakeAuditScript.path,
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("malformed target notarization evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.advertise.json").path))
    }

    func testTwoMachineTargetAdvertiseRejectsMalformedCanonicalBuiltAppWorkflowSidecarWhenEnabled() throws {
        guard ProcessInfo.processInfo.environment["SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION"] == "1" else {
            throw XCTSkip("set SUPERMOVER_RUN_REAL_PACKAGING_INTEGRATION=1 after building macos/dist/SuperMover.app")
        }

        let repoRoot = repoRootURL()
        let builtAppURL = repoRoot.appendingPathComponent("macos/dist/SuperMover.app", isDirectory: true)
        guard FileManager.default.fileExists(atPath: builtAppURL.path) else {
            throw XCTSkip("missing built app at \(builtAppURL.path); run sh macos/script/build-app.sh first")
        }

        let sidecarURL = builtAppURL.deletingLastPathComponent().appendingPathComponent("\(builtAppURL.lastPathComponent).notary/notarization.json")
        if FileManager.default.fileExists(atPath: sidecarURL.path) {
            throw XCTSkip("canonical built app already has sidecar at \(sidecarURL.path); workflow-sidecar injection case is not applicable")
        }
        defer {
            try? FileManager.default.removeItem(at: sidecarURL.deletingLastPathComponent())
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

        let notarize = try runNotarizeScript(
            arguments: [
                "--app", builtAppURL.path,
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

        let workDir = try makeDirectory(named: "two-machine-advertise-canonical-malformed-sidecar")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let builtCLI = builtAppURL.appendingPathComponent("Contents/Resources/bin/supermover")
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let sourceRoot = workDir.appendingPathComponent("source-root", isDirectory: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sourceRoot, withIntermediateDirectories: true)
        let profile = workDir.appendingPathComponent("target.profile.json")

        let initResult = try runProcess(
            executableURL: builtCLI,
            arguments: [
                "profile", "init",
                "--profile", profile.path,
                "--source", sourceRoot.path,
                "--target", targetRoot.path,
                "--id", "two-machine-advertise-canonical-malformed",
                "--name", "Two Machine Advertise Canonical Malformed",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        XCTAssertEqual(initResult.stderr, "")

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try #"{"schema":"stale"}"#.write(to: bundleRoot.appendingPathComponent("target.notarization.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {
            "notarization": {
              "target": {
                "collected_by": "stale",
                "output": "target.notarization.json",
                "status": "pass"
              }
            }
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let result = try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "target-advertise",
                "--profile", profile.path,
                "--bundle-root", bundleRoot.path,
                "--listen", "127.0.0.1:0",
                "--dest", "127.0.0.1:39394",
                "--duration", "1s",
                "--interval", "200ms",
            ],
            environment: [
                "SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
            ],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("malformed target notarization evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.advertise.json").path))
    }

    private func writeCurrentBundleReleaseEvidence(
        bundleRoot: URL,
        machine: String,
        appPath: String,
        cliVersion: String = "supermover v0"
    ) throws {
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: cliVersion)
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("\(machine).provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appPath,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("\(machine).app-audit.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appPath,
            auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(appPath: appPath),
            notaryLogPath: "\(machine).notary-log.json"
        ).write(to: bundleRoot.appendingPathComponent("\(machine).notarization.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(
            to: bundleRoot.appendingPathComponent("\(machine).notary-log.json"),
            atomically: true,
            encoding: .utf8
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

    private func assertMachineIdentityCorrectionNextActions(_ status: [String: Any]) throws {
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.count, 2)
        guard nextActions.count == 2 else {
            return
        }
        XCTAssertEqual(nextActions[0]["machine"] as? String, "target")
        XCTAssertEqual(nextActions[0]["step"] as? String, "target_serve_phase_1")
        XCTAssertEqual(nextActions[1]["machine"] as? String, "source")
        XCTAssertEqual(nextActions[1]["step"] as? String, "source_pair")
    }

    private func writeCurrentAppSidecarReleaseEvidence(
        appDir: URL,
        sidecarDir: URL,
        cliVersion: String
    ) throws -> [String: Any] {
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sidecarDir, withIntermediateDirectories: true)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: cliVersion)
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: resourcesDir.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        let auditPath = sidecarDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path
        ).write(to: sidecarDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)
        return provenanceManifest
    }

    private func makeCurrentAuditScript(appPath: String, provenanceManifest: [String: Any]) throws -> String {
        try AcceptanceReleaseEvidenceFixtures.readyAuditScript(
            appPath: appPath,
            provenanceManifest: provenanceManifest
        )
    }

    private func writeLocalPairingReceipt(id: String, under dir: URL) throws -> URL {
        let receiptsDir = dir.appendingPathComponent("local-pairing-receipts", isDirectory: true)
        try FileManager.default.createDirectory(at: receiptsDir, withIntermediateDirectories: true)
        let receiptURL = receiptsDir.appendingPathComponent("\(id).json")
        try targetPairingReceiptJSON(id: id).write(to: receiptURL, atomically: true, encoding: .utf8)
        return receiptURL
    }

    private func targetPairingReceiptJSON(id: String = "pair-1") throws -> String {
        try AcceptanceWorkflowFixtures.pairingReceiptJSON(id: id)
    }

    private func targetNetworkTransferJSON(
        version: Int? = 1,
        sessionID: String? = "session-1",
        profileID: String? = "profile-1",
        targetID: String? = "target-1",
        sourceDeviceID: Any? = "src-spki",
        targetDeviceID: Any? = "dst-spki",
        protocolVersion: String? = "supermover/1",
        status: String? = "published",
        stage: String? = "commit",
        encryptedTransfer: String? = "tls13_mtls",
        startedAt: String? = "2026-01-01T00:00:00Z",
        updatedAt: String? = "2026-01-01T00:00:01Z"
    ) throws -> String {
        try AcceptanceWorkflowFixtures.targetNetworkTransferJSON(
            version: version,
            sessionID: sessionID,
            profileID: profileID,
            targetID: targetID,
            sourceDeviceID: sourceDeviceID,
            targetDeviceID: targetDeviceID,
            protocolVersion: protocolVersion,
            status: status,
            stage: stage,
            encryptedTransfer: encryptedTransfer,
            startedAt: startedAt,
            updatedAt: updatedAt
        )
    }

    private func writeReadyTargetControlPlane(
        to targetRoot: URL,
        pairingReceiptID: String = "pair-1",
        sessionID: String = "session-1"
    ) throws {
        let controlPlane = targetRoot.appendingPathComponent(".supermover", isDirectory: true)
        try FileManager.default.createDirectory(
            at: controlPlane.appendingPathComponent("pairings"),
            withIntermediateDirectories: true
        )
        try FileManager.default.createDirectory(
            at: controlPlane.appendingPathComponent("sessions/\(sessionID)"),
            withIntermediateDirectories: true
        )
        try targetPairingReceiptJSON(id: pairingReceiptID).write(
            to: controlPlane.appendingPathComponent("pairings/\(pairingReceiptID).json"),
            atomically: true,
                encoding: .utf8
            )
        try targetNetworkTransferJSON(sessionID: sessionID)
            .write(
                to: controlPlane.appendingPathComponent("sessions/\(sessionID)/network-transfer.json"),
                atomically: true,
                encoding: .utf8
            )
    }

    private func writeSourcePairBundleArtifacts(
        to bundleRoot: URL,
        pairingReceiptID: String = "pair-1"
    ) throws {
        let exportedReceipts = bundleRoot.appendingPathComponent("exported-receipts", isDirectory: true)
        try FileManager.default.createDirectory(at: exportedReceipts, withIntermediateDirectories: true)
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "profile": "/tmp/source.profile.json",
            "target_address": "127.0.0.1:39395",
            "verification_code": "123456",
            "pairing_receipt_id": pairingReceiptID,
            "receipt_path": "exported-receipts/pair-1.json",
        ])
            .write(to: bundleRoot.appendingPathComponent("source.pair.json"), atomically: true, encoding: .utf8)
        try targetPairingReceiptJSON(id: pairingReceiptID).write(
            to: bundleRoot.appendingPathComponent("exported-receipts/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )
        try "pair ok\n".write(
            to: bundleRoot.appendingPathComponent("source.pair.txt"),
            atomically: true,
            encoding: .utf8
        )
    }

    private func writeTargetReadyBundleArtifact(to bundleRoot: URL) throws {
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
        """.write(to: bundleRoot.appendingPathComponent("target.ready.json"), atomically: true, encoding: .utf8)

        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let metaData = try Data(contentsOf: metaURL)
        var root = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        var evidence = try XCTUnwrap(root["evidence"] as? [String: Any])
        evidence["target_ready"] = [
            "address": "127.0.0.1:39395",
            "verification_code": "123456",
            "mode": "pairing",
        ]
        root["evidence"] = evidence
        let updated = try JSONSerialization.data(withJSONObject: root, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: metaURL)
    }

    private func makeReadyTwoMachineFixture(named name: String) throws -> (
        workDir: URL,
        bundleRoot: URL,
        targetRoot: URL
    ) {
        let workDir = try makeDirectory(named: name)
        let targetRoot = try makeDirectory(named: "\(name)-target")
        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
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
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app"
        )
        try writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app"
        )
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: bundleRoot, targetRoot: targetRoot)
        try writeReadyTargetControlPlane(to: targetRoot)
        return (workDir, bundleRoot, targetRoot)
    }

    private func writeSourceVerifySummary(
        _ summary: [String: Any],
        bundleRoot: URL,
        targetRoot: URL
    ) throws {
        let verify: [String: Any] = [
            "summary": summary,
            "manifest": ["manifestID": "manifest-1"],
            "target_root": targetRoot.path,
            "session_id": "session-1",
            "merkleRootProof": ["detail": "unavailable"],
        ]
        try AcceptanceReleaseEvidenceFixtures.jsonString(verify)
            .write(to: bundleRoot.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
    }

    private func setOperatorEvidenceDetail(
        kind: String,
        detail: String,
        bundleRoot: URL
    ) throws {
        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let metaData = try Data(contentsOf: metaURL)
        var root = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        var evidence = try XCTUnwrap(root["evidence"] as? [String: Any])
        var operatorEvidence = try XCTUnwrap(evidence["operator"] as? [String: Any])
        var record = try XCTUnwrap(operatorEvidence[kind] as? [String: Any])
        record["detail"] = detail
        operatorEvidence[kind] = record
        evidence["operator"] = operatorEvidence
        root["evidence"] = evidence

        let updatedMeta = try JSONSerialization.data(withJSONObject: root, options: [.prettyPrinted, .sortedKeys])
        try updatedMeta.write(to: metaURL)
    }

    private func setOperatorEvidenceMachineID(
        kind: String,
        machineID: String?,
        bundleRoot: URL
    ) throws {
        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let metaData = try Data(contentsOf: metaURL)
        var root = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        var evidence = try XCTUnwrap(root["evidence"] as? [String: Any])
        var operatorEvidence = try XCTUnwrap(evidence["operator"] as? [String: Any])
        var record = try XCTUnwrap(operatorEvidence[kind] as? [String: Any])
        if let machineID {
            record["machine_id"] = machineID
        } else {
            record.removeValue(forKey: "machine_id")
        }
        operatorEvidence[kind] = record
        evidence["operator"] = operatorEvidence
        root["evidence"] = evidence

        let updatedMeta = try JSONSerialization.data(withJSONObject: root, options: [.prettyPrinted, .sortedKeys])
        try updatedMeta.write(to: metaURL)
    }

    private func rewriteMachineFacts(
        machine: String,
        schema: String = "supermover.acceptance.machine_facts.v1",
        machineID: String,
        bundleRoot: URL
    ) throws {
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "schema": schema,
            "machine_id": machineID,
            "machine_label": machine,
        ]).write(
            to: bundleRoot.appendingPathComponent("\(machine).machine.json"),
            atomically: true,
            encoding: .utf8
        )
    }

    private func rewriteJSONObject(
        _ url: URL,
        transform: (inout [String: Any]) throws -> Void
    ) throws {
        let data = try Data(contentsOf: url)
        var root = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        try transform(&root)
        let updated = try JSONSerialization.data(withJSONObject: root, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: url)
    }

    private func rewriteAllSourceTransferTargetRoots(bundleRoot: URL, targetRoot: URL) throws {
        for artifact in [
            "source.verify.json",
            "source.report.json",
            "source.status.json",
            "source.health.json",
        ] {
            try rewriteJSONObject(bundleRoot.appendingPathComponent(artifact)) { root in
                root["target_root"] = targetRoot.path
            }
        }
    }

    private func setNestedJSONValue(
        _ value: Any,
        path: [String],
        root: inout [String: Any]
    ) throws {
        guard let key = path.first else {
            return
        }
        guard path.count > 1 else {
            root[key] = value
            return
        }
        var child = try XCTUnwrap(root[key] as? [String: Any])
        try setNestedJSONValue(value, path: Array(path.dropFirst()), root: &child)
        root[key] = child
    }

    private func rewriteJSONObjectString(
        _ url: URL,
        path: [String],
        rawValue: String
    ) throws {
        var text = try String(contentsOf: url, encoding: .utf8)
        try replaceNestedJSONNumberString(path: path, rawValue: rawValue, text: &text)
        try text.write(to: url, atomically: true, encoding: .utf8)
    }

    private func replaceNestedJSONNumberString(
        path: [String],
        rawValue: String,
        text: inout String
    ) throws {
        let key = try XCTUnwrap(path.first)
        guard path.count > 1 else {
            let pattern = #""\#(NSRegularExpression.escapedPattern(for: key))"\s*:\s*-?(?:\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)"#
            let regex = try NSRegularExpression(pattern: pattern)
            let range = NSRange(text.startIndex..<text.endIndex, in: text)
            guard let match = regex.firstMatch(in: text, range: range),
                  let matchRange = Range(match.range, in: text) else {
                XCTFail("missing JSON number key \(key)")
                return
            }
            text.replaceSubrange(matchRange, with: "\"\(key)\": \(rawValue)")
            return
        }

        let keyPattern = #""\#(NSRegularExpression.escapedPattern(for: key))"\s*:\s*\{"#
        let regex = try NSRegularExpression(pattern: keyPattern)
        let range = NSRange(text.startIndex..<text.endIndex, in: text)
        guard let match = regex.firstMatch(in: text, range: range),
              let matchRange = Range(match.range, in: text) else {
            XCTFail("missing JSON object key \(key)")
            return
        }
        let objectStart = text.index(before: matchRange.upperBound)
        let objectEnd = try XCTUnwrap(matchingBraceIndex(in: text, openingBraceIndex: objectStart))
        var objectText = String(text[objectStart...objectEnd])
        try replaceNestedJSONNumberString(path: Array(path.dropFirst()), rawValue: rawValue, text: &objectText)
        text.replaceSubrange(objectStart...objectEnd, with: objectText)
    }

    private func matchingBraceIndex(in text: String, openingBraceIndex: String.Index) -> String.Index? {
        var depth = 0
        var inString = false
        var isEscaped = false
        var index = openingBraceIndex
        while index < text.endIndex {
            let character = text[index]
            if inString {
                if isEscaped {
                    isEscaped = false
                } else if character == "\\" {
                    isEscaped = true
                } else if character == "\"" {
                    inString = false
                }
            } else if character == "\"" {
                inString = true
            } else if character == "{" {
                depth += 1
            } else if character == "}" {
                depth -= 1
                if depth == 0 {
                    return index
                }
            }
            index = text.index(after: index)
        }
        return nil
    }

    private func writeNetworkTransferEvidence(
        sourceDeviceID: Any?,
        targetDeviceID: Any?,
        targetRoot: URL
    ) throws {
        try targetNetworkTransferJSON(
            sourceDeviceID: sourceDeviceID,
            targetDeviceID: targetDeviceID
        )
            .write(
                to: targetRoot.appendingPathComponent(".supermover/sessions/session-1/network-transfer.json"),
                atomically: true,
                encoding: .utf8
            )
    }

    private func runWorkflowStatus(bundleRoot: URL) throws -> [String: Any] {
        let repoRoot = repoRootURL()
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "workflow-status",
                "--bundle-root", bundleRoot.path,
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
        let data = Data(result.stdout.utf8)
        return try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
    }

    private func runEvaluate(bundleRoot: URL, targetRoot: URL) throws -> ProcessResult {
        let repoRoot = repoRootURL()
        return try runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )
    }

    private func makeDirectory(named name: String) throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("\(name)-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
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
            throw ScriptIntegrationError.commandFailed(
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
        return ProcessResult(
            stdout: result.stdout,
            stderr: result.stderr,
            exitCode: result.exitCode
        )
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
    let exitCode: Int32
}

private enum ScriptIntegrationError: LocalizedError {
    case commandFailed(command: String, stdout: String, stderr: String, exitCode: Int32)

    var errorDescription: String? {
        switch self {
        case let .commandFailed(command, stdout, stderr, exitCode):
            return "command failed (\(exitCode)): \(command)\nstdout:\n\(stdout)\nstderr:\n\(stderr)"
        }
    }
}
