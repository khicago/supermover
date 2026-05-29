import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppReleaseEvidenceScriptTests: XCTestCase {
    func testShellRecordPackagingEvidenceWritesReadyArtifactsForSourceMachine() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-record-packaging-evidence")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("Fake.app", isDirectory: true)
        let sidecarDir = workDir.appendingPathComponent("Fake.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resourcesDir.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sidecarDir, withIntermediateDirectories: true)
        try minimalBundleMeta.write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        let smBin = binDir.appendingPathComponent("supermover")
        try stubVersionScript.write(to: smBin, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: smBin.path
        )
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        let auditPath = sidecarDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path
        ).write(
            to: sidecarDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let auditScript = workDir.appendingPathComponent("audit-app.sh")
        try AcceptanceReleaseEvidenceFixtures.readyAuditScript(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: auditScript.path
        )

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "record-packaging-evidence",
                "--bundle-root", bundleRoot.path,
                "--machine", "source",
                "--app", appDir.path,
            ],
            environment: ["SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": auditScript.path],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.sourceAppAudit?.output, "source.app-audit.json")
        XCTAssertEqual(snapshot.sourceAppAudit?.status, "pass")
        XCTAssertEqual(snapshot.sourceNotarization?.output, "source.notarization.json")
        XCTAssertEqual(snapshot.sourceNotarization?.collected_by, "record-packaging-evidence")
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("workflow.summary.json").path))
    }

    func testShellRecordPackagingEvidenceUsesBundledAuditHelperByDefault() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-record-packaging-evidence-bundled-helper")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("Fake.app", isDirectory: true)
        let sidecarDir = workDir.appendingPathComponent("Fake.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resourcesDir.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sidecarDir, withIntermediateDirectories: true)
        try minimalBundleMeta.write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )

        let smBin = binDir.appendingPathComponent("supermover")
        try stubVersionScript.write(to: smBin, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: smBin.path
        )

        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        let auditPath = sidecarDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path
        ).write(
            to: sidecarDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        let helperPath = binDir.appendingPathComponent("supermover-app-audit")
        try AcceptanceReleaseEvidenceFixtures.readyAuditScript(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: helperPath, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: helperPath.path
        )

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "record-packaging-evidence",
                "--bundle-root", bundleRoot.path,
                "--machine", "source",
                "--app", appDir.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.sourceAppAudit?.status, "pass")
        XCTAssertEqual(snapshot.sourceAppAudit?.readiness, "distribution_ready")
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("workflow.summary.json").path))
    }

    func testShellRecordPackagingEvidenceAcceptsAuditProvenanceWithDifferentKeyOrder() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-record-packaging-evidence-reordered-provenance")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("Fake.app", isDirectory: true)
        let sidecarDir = workDir.appendingPathComponent("Fake.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resourcesDir.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sidecarDir, withIntermediateDirectories: true)
        try minimalBundleMeta.write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )

        let smBin = binDir.appendingPathComponent("supermover")
        try stubVersionScript.write(to: smBin, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: smBin.path
        )

        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )

        let schema = try XCTUnwrap(provenanceManifest["schema"] as? String)
        let appBundleID = try XCTUnwrap(provenanceManifest["app_bundle_id"] as? String)
        let appVersion = try XCTUnwrap(provenanceManifest["app_version"] as? String)
        let buildProfile = try XCTUnwrap(provenanceManifest["build_profile"] as? String)
        let gitCommit = try XCTUnwrap(provenanceManifest["git_commit"] as? String)
        let gitDirty = try XCTUnwrap(provenanceManifest["git_dirty"] as? Bool)
        let cliVersion = try XCTUnwrap(provenanceManifest["cli_version"] as? String)
        let cliRelativePath = try XCTUnwrap(provenanceManifest["cli_relative_path"] as? String)
        let builtAt = try XCTUnwrap(provenanceManifest["built_at"] as? String)
        let signing = try XCTUnwrap(provenanceManifest["signing"] as? String)

        let auditPath = sidecarDir.appendingPathComponent("post-staple.audit.json")
        try """
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "pass",
          "readiness": "distribution_ready",
          "app_path": "\(appDir.path)",
          "provenance": {
            "manifest": {
              "signing": "\(signing)",
              "built_at": "\(builtAt)",
              "cli_relative_path": "\(cliRelativePath)",
              "cli_version": "\(cliVersion)",
              "git_dirty": \(gitDirty ? "true" : "false"),
              "git_commit": "\(gitCommit)",
              "build_profile": "\(buildProfile)",
              "app_version": "\(appVersion)",
              "app_bundle_id": "\(appBundleID)",
              "schema": "\(schema)"
            }
          },
          "summary": {
            "pass_ready": true,
            "blocking_checks": 0
          }
        }
        """.write(to: auditPath, atomically: true, encoding: .utf8)

        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path
        ).write(
            to: sidecarDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let auditScript = workDir.appendingPathComponent("audit-app.sh")
        try AcceptanceReleaseEvidenceFixtures.readyAuditScript(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: auditScript.path
        )

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "record-packaging-evidence",
                "--bundle-root", bundleRoot.path,
                "--machine", "source",
                "--app", appDir.path,
            ],
            environment: ["SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": auditScript.path],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.sourceNotarization?.output, "source.notarization.json")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.audit?.path, auditPath.path)
    }

    func testShellRecordPackagingEvidenceRejectsStaleSiblingNotarizationWhoseAuditDoesNotMatchCurrentApp() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-record-packaging-evidence-stale-notary")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("Fake.app", isDirectory: true)
        let sidecarDir = workDir.appendingPathComponent("Fake.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resourcesDir.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sidecarDir, withIntermediateDirectories: true)
        try minimalBundleMeta.write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )

        let smBin = binDir.appendingPathComponent("supermover")
        try stubVersionScript.write(to: smBin, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: smBin.path
        )

        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        let staleAuditPath = workDir.appendingPathComponent("Fake.stale.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: "/tmp/old/Fake.app",
            provenanceManifest: provenanceManifest
        ).write(to: staleAuditPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(
            to: sidecarDir.appendingPathComponent("notary-log.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: "/tmp/old/Fake.app",
            auditPath: staleAuditPath.path
        ).write(
            to: sidecarDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let auditScript = workDir.appendingPathComponent("audit-app.sh")
        try AcceptanceReleaseEvidenceFixtures.readyAuditScript(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: auditScript.path
        )

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "record-packaging-evidence",
                "--bundle-root", bundleRoot.path,
                "--machine", "source",
                "--app", appDir.path,
            ],
            environment: ["SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": auditScript.path],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("stale source notarization evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("source.notarization.json").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("workflow.summary.json").path))
    }

    func testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotarizationOutputLeaf() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-record-packaging-evidence-output-symlink")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("Fake.app", isDirectory: true)
        let sidecarDir = workDir.appendingPathComponent("Fake.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resourcesDir.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sidecarDir, withIntermediateDirectories: true)
        try minimalBundleMeta.write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )

        let smBin = binDir.appendingPathComponent("supermover")
        try stubVersionScript.write(to: smBin, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: smBin.path
        )

        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        let auditPath = sidecarDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path
        ).write(
            to: sidecarDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let auditScript = workDir.appendingPathComponent("audit-app.sh")
        try AcceptanceReleaseEvidenceFixtures.readyAuditScript(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: auditScript.path
        )

        let outsideOutputURL = workDir.appendingPathComponent("outside-notarization.json")
        try "outside\n".write(to: outsideOutputURL, atomically: true, encoding: .utf8)
        do {
            try FileManager.default.createSymbolicLink(
                at: bundleRoot.appendingPathComponent("source.notarization.json"),
                withDestinationURL: outsideOutputURL
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "record-packaging-evidence",
                "--bundle-root", bundleRoot.path,
                "--machine", "source",
                "--app", appDir.path,
            ],
            environment: ["SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": auditScript.path],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe source notarization evidence"))
        XCTAssertEqual(
            try String(contentsOf: outsideOutputURL, encoding: .utf8),
            "outside\n"
        )
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("workflow.summary.json").path))
    }

    func testShellRecordPackagingEvidenceRejectsSymlinkedBundleNotaryLogOutputLeaf() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-record-packaging-evidence-notary-log-output-symlink")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("Fake.app", isDirectory: true)
        let sidecarDir = workDir.appendingPathComponent("Fake.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resourcesDir.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sidecarDir, withIntermediateDirectories: true)
        try minimalBundleMeta.write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )

        let smBin = binDir.appendingPathComponent("supermover")
        try stubVersionScript.write(to: smBin, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: smBin.path
        )

        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        let auditPath = sidecarDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path
        ).write(
            to: sidecarDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let auditScript = workDir.appendingPathComponent("audit-app.sh")
        try AcceptanceReleaseEvidenceFixtures.readyAuditScript(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: auditScript.path
        )

        let outsideOutputURL = workDir.appendingPathComponent("outside-notary-log.json")
        try "outside\n".write(to: outsideOutputURL, atomically: true, encoding: .utf8)
        do {
            try FileManager.default.createSymbolicLink(
                at: bundleRoot.appendingPathComponent("source.notary-log.json"),
                withDestinationURL: outsideOutputURL
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "record-packaging-evidence",
                "--bundle-root", bundleRoot.path,
                "--machine", "source",
                "--app", appDir.path,
            ],
            environment: ["SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": auditScript.path],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe source notarization evidence"))
        XCTAssertEqual(
            try String(contentsOf: outsideOutputURL, encoding: .utf8),
            "outside\n"
        )
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("workflow.summary.json").path))
    }

    func testShellRecordPackagingEvidenceRejectsHardlinkedBundleAppAuditOutputLeaf() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-record-packaging-evidence-app-audit-output-hardlink")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("Fake.app", isDirectory: true)
        let sidecarDir = workDir.appendingPathComponent("Fake.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resourcesDir.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: sidecarDir, withIntermediateDirectories: true)
        try minimalBundleMeta.write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )

        let smBin = binDir.appendingPathComponent("supermover")
        try stubVersionScript.write(to: smBin, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: smBin.path
        )

        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        let auditPath = sidecarDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path
        ).write(
            to: sidecarDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let auditScript = workDir.appendingPathComponent("audit-app.sh")
        try AcceptanceReleaseEvidenceFixtures.readyAuditScript(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditScript, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: auditScript.path
        )

        let outsideOutputURL = workDir.appendingPathComponent("outside-app-audit.json")
        try "outside\n".write(to: outsideOutputURL, atomically: true, encoding: .utf8)
        do {
            try FileManager.default.linkItem(
                at: outsideOutputURL,
                to: bundleRoot.appendingPathComponent("source.app-audit.json")
            )
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "record-packaging-evidence",
                "--bundle-root", bundleRoot.path,
                "--machine", "source",
                "--app", appDir.path,
            ],
            environment: ["SUPERMOVER_ACCEPTANCE_AUDIT_APP_SCRIPT": auditScript.path],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe source packaging evidence"))
        XCTAssertEqual(
            try String(contentsOf: outsideOutputURL, encoding: .utf8),
            "outside\n"
        )
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("workflow.summary.json").path))
    }

    func testShellWorkflowStatusRequiresPackagingEvidenceBeforeEvaluateWhenStrictTwoMachineReleaseEvidenceIsMissing() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-release-evidence-script")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try workflowBundleMeta.write(
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
        """.write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
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

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["ok"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, true)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(
            failures,
            [
                "source.app-audit.json is not install-ready",
                "target.app-audit.json is not install-ready",
                "source.notarization.json is not release-ready",
                "target.notarization.json is not release-ready",
            ]
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence", "target_packaging_evidence"])
        let firstCommands = try XCTUnwrap(nextActions.first?["commands"] as? [String])
        XCTAssertTrue(firstCommands.first?.contains("record-packaging-evidence") == true)
    }

    func testShellWorkflowStatusTreatsSymlinkedSourceAppAuditAsMissingReleaseEvidence() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(
            named: "acceptance-workflow-release-evidence-source-app-audit-symlink"
        )
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try workflowBundleMeta.write(
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
        """.write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        _ = try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/Applications/SuperMover Source.app",
            cliVersion: "supermover v0"
        )
        _ = try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "target",
            appPath: "/Applications/SuperMover Target.app",
            cliVersion: "supermover v0"
        )

        let outsideAuditURL = workDir.appendingPathComponent("outside-source.app-audit.json")
        let currentAudit = try String(
            contentsOf: bundleRoot.appendingPathComponent("source.app-audit.json"),
            encoding: .utf8
        )
        try currentAudit.write(to: outsideAuditURL, atomically: true, encoding: .utf8)
        try FileManager.default.removeItem(at: bundleRoot.appendingPathComponent("source.app-audit.json"))
        do {
            try FileManager.default.createSymbolicLink(
                at: bundleRoot.appendingPathComponent("source.app-audit.json"),
                withDestinationURL: outsideAuditURL
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
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

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, true)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertTrue(failures.contains("source.app-audit.json is not install-ready"))
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])
    }

    func testShellWorkflowStatusRequiresFreshSourceNotarizationBeforeEvaluateWhenStrictTwoMachineReleaseEvidenceIsStale() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-release-evidence-stale-notary")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try workflowBundleMeta.write(
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
        """.write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        let sourceAppPath = "/Applications/SuperMover Source.app"
        let targetAppPath = "/Applications/SuperMover Target.app"
        let sourceAuditPath = "/Applications/SuperMover Source.app.audit.json"
        let targetAuditPath = AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(appPath: targetAppPath)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: "supermover v0")

        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("source.provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("target.provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: sourceAppPath,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("source.app-audit.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: targetAppPath,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: "/Applications/Stale Source.app",
            auditPath: sourceAuditPath,
            notaryLogPath: "source.notary-log.json"
        ).write(to: bundleRoot.appendingPathComponent("source.notarization.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: targetAppPath,
            auditPath: targetAuditPath,
            notaryLogPath: "target.notary-log.json"
        ).write(to: bundleRoot.appendingPathComponent("target.notarization.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.writeBundleNotaryLog(bundleRoot: bundleRoot, machine: "source")
        try AcceptanceReleaseEvidenceFixtures.writeBundleNotaryLog(bundleRoot: bundleRoot, machine: "target")

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
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

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["ok"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, true)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(failures, ["source.notarization.json is not release-ready"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])
    }

    func testShellWorkflowStatusRequiresFreshSourceNotarizationBeforeEvaluateWhenStrictTwoMachineAuditProvenanceIsStale() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-release-evidence-stale-audit-provenance")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try workflowBundleMeta.write(
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
        """.write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        let sourceAppPath = "/Applications/SuperMover Source.app"
        let targetAppPath = "/Applications/SuperMover Target.app"
        let sourceAuditPath = "/Applications/SuperMover Source.app.audit.json"
        let targetAuditPath = AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(appPath: targetAppPath)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: "supermover v0")
        var staleProvenanceManifest = provenanceManifest
        staleProvenanceManifest["git_commit"] = "stale00000000"

        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("source.provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("target.provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: sourceAppPath,
            provenanceManifest: staleProvenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("source.app-audit.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: targetAppPath,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: sourceAppPath,
            auditPath: sourceAuditPath,
            notaryLogPath: "source.notary-log.json"
        ).write(to: bundleRoot.appendingPathComponent("source.notarization.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: targetAppPath,
            auditPath: targetAuditPath,
            notaryLogPath: "target.notary-log.json"
        ).write(to: bundleRoot.appendingPathComponent("target.notarization.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.writeBundleNotaryLog(bundleRoot: bundleRoot, machine: "source")
        try AcceptanceReleaseEvidenceFixtures.writeBundleNotaryLog(bundleRoot: bundleRoot, machine: "target")

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
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

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["ok"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, true)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(
            failures,
            [
                "source.app-audit.json is not install-ready",
                "source.notarization.json is not release-ready",
            ]
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])
    }

    func testShellWorkflowStatusRejectsSourceAppAuditThatPassesButIsReviewOnly() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-release-evidence-review-only-audit")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try workflowBundleMeta.write(
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
        """.write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        let sourceAppPath = "/Applications/SuperMover Source.app"
        let targetAppPath = "/Applications/SuperMover Target.app"
        let sourceAuditPath = AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(appPath: sourceAppPath)
        let targetAuditPath = AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(appPath: targetAppPath)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: "supermover v0")

        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("source.provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("target.provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: sourceAppPath,
            provenanceManifest: provenanceManifest,
            status: "pass",
            readiness: "review_only",
            passReady: true
        ).write(to: bundleRoot.appendingPathComponent("source.app-audit.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: targetAppPath,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: sourceAppPath,
            auditPath: sourceAuditPath,
            notaryLogPath: "source.notary-log.json",
            auditStatus: "pass",
            auditReadiness: "review_only",
            auditPassReady: true
        ).write(to: bundleRoot.appendingPathComponent("source.notarization.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: targetAppPath,
            auditPath: targetAuditPath,
            notaryLogPath: "target.notary-log.json"
        ).write(to: bundleRoot.appendingPathComponent("target.notarization.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.writeBundleNotaryLog(bundleRoot: bundleRoot, machine: "source")
        try AcceptanceReleaseEvidenceFixtures.writeBundleNotaryLog(bundleRoot: bundleRoot, machine: "target")

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
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

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["ok"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, true)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(
            failures,
            [
                "source.app-audit.json is not install-ready",
                "source.notarization.json is not release-ready",
            ]
        )
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])
    }

    func testShellWorkflowStatusRejectsReleaseReadyNotarizationWithoutNotaryLogEvidence() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-workflow-release-evidence-missing-notary-log")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try workflowBundleMeta.write(
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
        """.write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        let sourceAppPath = "/Applications/SuperMover Source.app"
        let targetAppPath = "/Applications/SuperMover Target.app"
        let sourceAuditPath = "/Applications/SuperMover Source.app.audit.json"
        let targetAuditPath = AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(appPath: targetAppPath)
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: "supermover v0")

        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("source.provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("target.provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: sourceAppPath,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("source.app-audit.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: targetAppPath,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(
            to: bundleRoot.appendingPathComponent("target.notary-log.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: sourceAppPath,
            auditPath: sourceAuditPath,
            includeNotaryLog: false
        ).write(to: bundleRoot.appendingPathComponent("source.notarization.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: targetAppPath,
            auditPath: targetAuditPath,
            notaryLogPath: "target.notary-log.json"
        ).write(to: bundleRoot.appendingPathComponent("target.notarization.json"), atomically: true, encoding: .utf8)

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
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

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        let data = Data(result.stdout.utf8)
        let status = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        XCTAssertEqual(status["ok"] as? Bool, false)
        XCTAssertEqual(status["installed_app_proof_ok"] as? Bool, true)
        XCTAssertEqual(status["installed_app_release_evidence_ok"] as? Bool, false)
        let failures = try XCTUnwrap(status["installed_app_release_evidence_failures"] as? [String])
        XCTAssertEqual(failures, ["source.notarization.json is not release-ready"])
        let nextActions = try XCTUnwrap(status["next_actions"] as? [[String: Any]])
        XCTAssertEqual(nextActions.map { $0["step"] as? String }, ["source_packaging_evidence"])
    }

    private var minimalBundleMeta: String {
        """
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
        """
    }

    private var workflowBundleMeta: String {
        """
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
        """
    }

    private var stubVersionScript: String {
        """
        #!/bin/sh
        set -eu
        printf 'supermover test-build\\n'
        """
    }
}
