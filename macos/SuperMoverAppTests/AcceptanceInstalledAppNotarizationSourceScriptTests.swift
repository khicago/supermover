import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppNotarizationSourceScriptTests: XCTestCase {
    func testShellInstalledAppAuditAcceptsSiblingNotarizationSidecar() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-installed-app-notary-sibling")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let siblingNotaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: siblingNotaryDir, withIntermediateDirectories: true)

        try makeBundleMeta(withStaleNotarization: false).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        let auditPath = siblingNotaryDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(
            to: bundleRoot.appendingPathComponent("target.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: auditPath.path
        ).write(
            to: siblingNotaryDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runAcceptanceRequireReadyAppAudit(bundleRoot: bundleRoot, appDir: appDir)

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        XCTAssertEqual(result.stderr, "")
        XCTAssertTrue(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertEqual(snapshot.targetNotarization?.output, "target.notarization.json")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.status, "pass")
        XCTAssertEqual(snapshot.targetNotarizationArtifact?.submission?.status, "Accepted")
    }

    func testShellInstalledAppAuditRejectsSiblingNotarizationSidecarWhenAuditDoesNotMatchCurrentApp() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-installed-app-notary-stale-sibling")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let siblingNotaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: siblingNotaryDir, withIntermediateDirectories: true)

        try makeBundleMeta(withStaleNotarization: false).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(
            to: bundleRoot.appendingPathComponent("target.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        let staleAuditPath = workDir.appendingPathComponent("SuperMover.stale.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: "/tmp/old/SuperMover.app",
            provenanceManifest: provenanceManifest
        ).write(to: staleAuditPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(
            to: siblingNotaryDir.appendingPathComponent("notary-log.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: "/tmp/old/SuperMover.app",
            auditPath: staleAuditPath.path
        ).write(
            to: siblingNotaryDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runAcceptanceRequireReadyAppAudit(bundleRoot: bundleRoot, appDir: appDir)

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("stale target notarization evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertNil(snapshot.targetNotarization)
        XCTAssertNil(snapshot.targetNotarizationArtifact)
    }

    func testShellInstalledAppAuditRejectsBundledFallbackNotarizationPath() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-installed-app-notary-fallback")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources/release/notary", isDirectory: true)
        let rootResourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: rootResourcesDir, withIntermediateDirectories: true)

        try makeBundleMeta(withStaleNotarization: true).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: rootResourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(
            to: bundleRoot.appendingPathComponent("target.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        let fallbackAuditPath = workDir.appendingPathComponent("SuperMover.fallback.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: fallbackAuditPath, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: fallbackAuditPath.path
        ).write(
            to: resourcesDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        try #"{"schema":"stale"}"#.write(
            to: bundleRoot.appendingPathComponent("target.notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runAcceptanceRequireReadyAppAudit(bundleRoot: bundleRoot, appDir: appDir)

        XCTAssertEqual(result.exitCode, 5)
        let expectedSiblingPath = workDir
            .appendingPathComponent("SuperMover.app.notary", isDirectory: true)
            .appendingPathComponent("notarization.json")
            .path
        XCTAssertTrue(result.stderr.contains("release-ready target notarization evidence before phase execution"))
        XCTAssertTrue(result.stderr.contains(expectedSiblingPath))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertNil(snapshot.targetNotarization)
        XCTAssertNil(snapshot.targetNotarizationArtifact)
    }

    func testShellInstalledAppAuditRejectsSiblingNotarizationSidecarWhenCanonicalLeafIsSymlinked() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-installed-app-notary-sidecar-symlink")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let siblingNotaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: siblingNotaryDir, withIntermediateDirectories: true)

        try makeBundleMeta(withStaleNotarization: true).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(
            to: bundleRoot.appendingPathComponent("target.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        let canonicalAuditPath = siblingNotaryDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: canonicalAuditPath, atomically: true, encoding: .utf8)
        let externalSidecarURL = workDir.appendingPathComponent("external-notarization.json")
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: canonicalAuditPath.path
        ).write(to: externalSidecarURL, atomically: true, encoding: .utf8)
        try FileManager.default.createSymbolicLink(
            at: siblingNotaryDir.appendingPathComponent("notarization.json"),
            withDestinationURL: externalSidecarURL
        )
        try #"{"schema":"stale"}"#.write(
            to: bundleRoot.appendingPathComponent("target.notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runAcceptanceRequireReadyAppAudit(bundleRoot: bundleRoot, appDir: appDir)

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe target notarization evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertNil(snapshot.targetNotarization)
        XCTAssertNil(snapshot.targetNotarizationArtifact)
    }

    func testShellInstalledAppAuditRejectsSiblingNotarizationSidecarWhenCanonicalLeafIsHardlinked() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-installed-app-notary-sidecar-hardlink")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let siblingNotaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: siblingNotaryDir, withIntermediateDirectories: true)

        try makeBundleMeta(withStaleNotarization: true).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(
            to: bundleRoot.appendingPathComponent("target.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        let canonicalAuditPath = siblingNotaryDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: canonicalAuditPath, atomically: true, encoding: .utf8)
        let canonicalSidecarURL = siblingNotaryDir.appendingPathComponent("notarization.json")
        let externalSidecarURL = workDir.appendingPathComponent("external-notarization.json")
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: canonicalAuditPath.path
        ).write(to: externalSidecarURL, atomically: true, encoding: .utf8)
        do {
            try FileManager.default.linkItem(at: externalSidecarURL, to: canonicalSidecarURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }
        try #"{"schema":"stale"}"#.write(
            to: bundleRoot.appendingPathComponent("target.notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runAcceptanceRequireReadyAppAudit(bundleRoot: bundleRoot, appDir: appDir)

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe target notarization evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertNil(snapshot.targetNotarization)
        XCTAssertNil(snapshot.targetNotarizationArtifact)
    }

    func testShellInstalledAppAuditRejectsSiblingNotarizationSidecarWhenCanonicalAuditLeafIsSymlinked() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-installed-app-notary-audit-symlink")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let appDir = workDir.appendingPathComponent("SuperMover.app", isDirectory: true)
        let siblingNotaryDir = workDir.appendingPathComponent("SuperMover.app.notary", isDirectory: true)
        let resourcesDir = appDir.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: resourcesDir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: siblingNotaryDir, withIntermediateDirectories: true)

        try makeBundleMeta(withStaleNotarization: true).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.bundledProvenanceJSON().write(
            to: resourcesDir.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(
            to: bundleRoot.appendingPathComponent("target.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        let externalAuditURL = workDir.appendingPathComponent("external-post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appDir.path,
            provenanceManifest: provenanceManifest
        ).write(to: externalAuditURL, atomically: true, encoding: .utf8)
        let canonicalAuditPath = siblingNotaryDir.appendingPathComponent("post-staple.audit.json")
        try FileManager.default.createSymbolicLink(at: canonicalAuditPath, withDestinationURL: externalAuditURL)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appDir.path,
            auditPath: canonicalAuditPath.path
        ).write(
            to: siblingNotaryDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        try #"{"schema":"stale"}"#.write(
            to: bundleRoot.appendingPathComponent("target.notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let result = try runAcceptanceRequireReadyAppAudit(bundleRoot: bundleRoot, appDir: appDir)

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("unsafe target notarization evidence"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("target.notarization.json").path))
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertNil(snapshot.targetNotarization)
        XCTAssertNil(snapshot.targetNotarizationArtifact)
    }

    private func runAcceptanceRequireReadyAppAudit(bundleRoot: URL, appDir: URL) throws -> AcceptanceProcessResult {
        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        return try AcceptanceScriptHarness.runProcessAllowFailure(
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
    }

    private func makeBundleMeta(withStaleNotarization: Bool) -> String {
        let staleNotarization = withStaleNotarization ? """
            ,
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
        """ : ""
        return """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {
            "app_audit": {
              "target": {
                "collected_by": "test",
                "output": "target.app-audit.json",
                "exit_code": 0,
                "status": "pass",
                "readiness": "distribution_ready",
                "pass_ready": true,
                "blocking_checks": 0
              }
            }\(staleNotarization)
          }
        }
        """
    }
}
