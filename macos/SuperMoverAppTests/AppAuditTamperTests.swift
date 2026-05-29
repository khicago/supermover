import XCTest
@testable import SuperMoverApp

final class AppAuditTamperTests: XCTestCase {
    func testAppAuditDoesNotBlockWhenCanonicalNotarizationSidecarIsAbsent() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertFalse(audit.blockedCheckIDs.contains("notarization.sidecar.parse"))
        XCTAssertFalse(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertFalse(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhoseAppPathDoesNotMatchCurrentApp() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: "/tmp/old/SuperMover.app",
            auditAppPath: "/tmp/old/SuperMover.app",
            auditProvenanceManifest: currentManifest
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhoseAuditProvenanceDoesNotMatchCurrentApp() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        var staleManifest = currentManifest
        staleManifest["git_commit"] = "stale00000000"
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: staleManifest
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhoseAuditPathEscapesSiblingDirectory() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest
        )

        let externalAuditURL = app.rootURL.appendingPathComponent("external-post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: app.appURL.path,
            provenanceManifest: currentManifest
        ).write(to: externalAuditURL, atomically: true, encoding: .utf8)

        let sidecarDirectoryURL = app.appURL
            .deletingLastPathComponent()
            .appendingPathComponent("\(app.appURL.lastPathComponent).notary", isDirectory: true)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: app.appURL.path,
            auditPath: externalAuditURL.path
        ).write(
            to: sidecarDirectoryURL.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhenSubmissionIDIsMissing() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest,
            submissionID: nil
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.notary_log"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhenSubmissionIDIsNotUUID() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest,
            submissionID: "not-a-uuid"
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.notary_log"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhenAuthModeIsMissing() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest,
            authMode: nil
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertFalse(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhenFailureIsRecorded() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest,
            failure: ["reason": "stapler failed"]
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertFalse(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhenNotaryLogIsNotAccepted() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest
        )
        try #"{"status":"Accepted","issues":"not-an-array"}"#.write(
            to: canonicalNotaryLogURL(for: app.appURL),
            atomically: true,
            encoding: .utf8
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.notary_log"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhenNotaryLogIsMissing() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest,
            includeNotaryLog: false
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.notary_log"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhenSidecarLeafIsSymlinked() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest
        )

        let externalSidecarURL = app.rootURL.appendingPathComponent("external-notarization.json")
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: app.appURL.path,
            auditPath: canonicalPostStapleAuditURL(for: app.appURL).path
        ).write(to: externalSidecarURL, atomically: true, encoding: .utf8)
        try replaceItemWithSymlink(
            at: canonicalNotarizationSidecarURL(for: app.appURL),
            destination: externalSidecarURL
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.parse"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhenSidecarLeafIsBrokenSymlink() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest
        )

        try replaceItemWithSymlink(
            at: canonicalNotarizationSidecarURL(for: app.appURL),
            destination: app.rootURL.appendingPathComponent("missing-notarization.json")
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.parse"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhenSidecarLeafIsHardlinked() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest
        )

        let sidecarURL = canonicalNotarizationSidecarURL(for: app.appURL)
        let outsideURL = app.rootURL.appendingPathComponent("outside-notarization.json")
        let sidecarJSON = try String(contentsOf: sidecarURL, encoding: .utf8)
        try FileManager.default.removeItem(at: sidecarURL)
        try sidecarJSON.write(to: outsideURL, atomically: true, encoding: .utf8)
        do {
            try FileManager.default.linkItem(at: outsideURL, to: sidecarURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.parse"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhenPostStapleAuditLeafIsSymlinked() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest
        )

        let externalAuditURL = app.rootURL.appendingPathComponent("external-post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: app.appURL.path,
            provenanceManifest: currentManifest
        ).write(to: externalAuditURL, atomically: true, encoding: .utf8)
        try replaceItemWithSymlink(
            at: canonicalPostStapleAuditURL(for: app.appURL),
            destination: externalAuditURL
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksCanonicalNotarizationSidecarWhenPostStapleAuditLeafIsBrokenSymlink() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: app.appURL.path,
            auditAppPath: app.appURL.path,
            auditProvenanceManifest: currentManifest
        )

        try replaceItemWithSymlink(
            at: canonicalPostStapleAuditURL(for: app.appURL),
            destination: app.rootURL.appendingPathComponent("missing-post-staple.audit.json")
        )

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    func testAppAuditBlocksTamperedBundles() throws {
        let head = try currentGitHead()

        let missingLocalNetworkUsage = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: missingLocalNetworkUsage.rootURL) }
        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/usr/bin/plutil"),
            arguments: ["-remove", "NSLocalNetworkUsageDescription", missingLocalNetworkUsage.appURL.appendingPathComponent("Contents/Info.plist").path]
        )
        let missingLocalNetworkUsageAudit = try runAppAudit(appURL: missingLocalNetworkUsage.appURL)
        XCTAssertEqual(missingLocalNetworkUsageAudit.exitCode, 1)
        XCTAssertEqual(missingLocalNetworkUsageAudit.status, "blocked")
        XCTAssertTrue(missingLocalNetworkUsageAudit.blockedCheckIDs.contains("plist.local_network_usage"))

        let missingEntitlement = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head,
            appEntitlements: [
                "com.apple.security.files.user-selected.read-write": true,
                "com.apple.security.network.client": true,
            ]
        )
        defer { try? FileManager.default.removeItem(at: missingEntitlement.rootURL) }
        let missingEntitlementAudit = try runAppAudit(appURL: missingEntitlement.appURL)
        XCTAssertEqual(missingEntitlementAudit.exitCode, 1)
        XCTAssertEqual(missingEntitlementAudit.status, "blocked")
        XCTAssertTrue(missingEntitlementAudit.blockedCheckIDs.contains("codesign.app.entitlement.com.apple.security.network.server"))

        let wrongCLIPath = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/not-supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: wrongCLIPath.rootURL) }
        let wrongCLIPathAudit = try runAppAudit(appURL: wrongCLIPath.appURL)
        XCTAssertEqual(wrongCLIPathAudit.exitCode, 1)
        XCTAssertTrue(wrongCLIPathAudit.blockedCheckIDs.contains("provenance.cli_relative_path"))

        let wrongCLIVersion = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 9.9.9",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: wrongCLIVersion.rootURL) }
        let wrongCLIVersionAudit = try runAppAudit(appURL: wrongCLIVersion.appURL)
        XCTAssertEqual(wrongCLIVersionAudit.exitCode, 1)
        XCTAssertTrue(wrongCLIVersionAudit.blockedCheckIDs.contains("cli.version.provenance"))

        let staleProvenance = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: "stale00000000"
        )
        defer { try? FileManager.default.removeItem(at: staleProvenance.rootURL) }
        let staleProvenanceAudit = try runAppAudit(appURL: staleProvenance.appURL)
        XCTAssertEqual(staleProvenanceAudit.exitCode, 1)
        XCTAssertTrue(staleProvenanceAudit.blockedCheckIDs.contains("git.provenance_head"))

        let malformedProvenance = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceJSON: "not-json\n"
        )
        defer { try? FileManager.default.removeItem(at: malformedProvenance.rootURL) }
        let malformedProvenanceAudit = try runAppAudit(appURL: malformedProvenance.appURL)
        XCTAssertEqual(malformedProvenanceAudit.exitCode, 1)
        XCTAssertTrue(malformedProvenanceAudit.blockedCheckIDs.contains("provenance.parse"))

        let missingIcon = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: missingIcon.rootURL) }
        try FileManager.default.removeItem(
            at: missingIcon.appURL.appendingPathComponent("Contents/Resources/SuperMover.icns")
        )
        let missingIconAudit = try runAppAudit(appURL: missingIcon.appURL)
        XCTAssertEqual(missingIconAudit.exitCode, 1)
        XCTAssertTrue(missingIconAudit.blockedCheckIDs.contains("app.icon.exists"))

        let missingIconPlistKey = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: missingIconPlistKey.rootURL) }
        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/usr/bin/plutil"),
            arguments: ["-remove", "CFBundleIconFile", missingIconPlistKey.appURL.appendingPathComponent("Contents/Info.plist").path]
        )
        let missingIconPlistKeyAudit = try runAppAudit(appURL: missingIconPlistKey.appURL)
        XCTAssertEqual(missingIconPlistKeyAudit.exitCode, 1)
        XCTAssertTrue(missingIconPlistKeyAudit.blockedCheckIDs.contains("plist.icon_file"))
    }
}

extension XCTestCase {
    func makeTemporaryDirectory() throws -> URL {
        let url = URL(fileURLWithPath: NSTemporaryDirectory(), isDirectory: true)
            .appendingPathComponent("supermover-app-tests-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true)
        return url
    }

    func currentGitHead(file: StaticString = #filePath, line: UInt = #line) throws -> String {
        let result = try runProcess(
            executableURL: URL(fileURLWithPath: "/usr/bin/git"),
            arguments: ["rev-parse", "--short=12", "HEAD"],
            currentDirectoryURL: repositoryRootURL(),
            file: file,
            line: line
        )
        let head = result.stdout.trimmingCharacters(in: .whitespacesAndNewlines)
        XCTAssertFalse(head.isEmpty, file: file, line: line)
        return head
    }

    func repositoryRootURL(file: StaticString = #filePath, line: UInt = #line) -> URL {
        var current = URL(fileURLWithPath: String(describing: file))
            .deletingLastPathComponent()
        while current.path != "/" {
            if FileManager.default.fileExists(atPath: current.appendingPathComponent(".git").path) {
                return current
            }
            current.deleteLastPathComponent()
        }
        XCTFail("could not locate repository root from \(String(describing: file))", file: file, line: line)
        return URL(fileURLWithPath: "/")
    }

    func makeExecutableBundledCLI(in resourceURL: URL) throws {
        let binURL = resourceURL.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binURL, withIntermediateDirectories: true)
        let cliURL = binURL.appendingPathComponent("supermover")
        try "#!/bin/sh\nprintf 'supermover 0.1.0-dev\\n'\n".write(to: cliURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cliURL.path)
    }

    func makeSignedTestApp(
        cliVersionOutput: String = "supermover 0.1.0-dev",
        provenanceCLIPath: String = "Contents/Resources/bin/supermover",
        provenanceCLIVersion: String = "supermover 0.1.0-dev",
        provenanceGitCommit: String = "abcdef123456",
        provenanceJSON: String? = nil,
        appEntitlements: [String: Bool] = [
            "com.apple.security.files.user-selected.read-write": true,
            "com.apple.security.network.client": true,
            "com.apple.security.network.server": true,
        ],
        cliEntitlements: [String: Bool] = [
            "com.apple.security.files.user-selected.read-write": true,
            "com.apple.security.network.client": true,
            "com.apple.security.network.server": true,
        ],
        file: StaticString = #filePath,
        line: UInt = #line
    ) throws -> (rootURL: URL, appURL: URL) {
        let rootURL = try makeTemporaryDirectory()
        let appURL = rootURL.appendingPathComponent("SuperMover.app", isDirectory: true)
        let contentsURL = appURL.appendingPathComponent("Contents", isDirectory: true)
        let macOSURL = contentsURL.appendingPathComponent("MacOS", isDirectory: true)
        let resourcesURL = contentsURL.appendingPathComponent("Resources", isDirectory: true)
        let binURL = resourcesURL.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: macOSURL, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: binURL, withIntermediateDirectories: true)

        let repoRoot = repositoryRootURL(file: file, line: line)
        try FileManager.default.copyItem(
            at: repoRoot.appendingPathComponent("macos/script/Info.plist"),
            to: contentsURL.appendingPathComponent("Info.plist")
        )

        let appExecutableURL = macOSURL.appendingPathComponent("SuperMoverApp")
        try "#!/bin/sh\nexit 0\n".write(to: appExecutableURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: appExecutableURL.path)

        let cliURL = binURL.appendingPathComponent("supermover")
        let cliScript = """
        #!/bin/sh
        if [ "${1:-}" = "version" ]; then
          printf '%s\n' "\(cliVersionOutput)"
          exit 0
        fi
        printf 'unexpected args\n' >&2
        exit 64
        """
        try cliScript.write(to: cliURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: cliURL.path)

        let provenancePath = resourcesURL.appendingPathComponent("supermover-provenance.json")
        if let provenanceJSON {
            try provenanceJSON.write(to: provenancePath, atomically: true, encoding: .utf8)
        } else {
            let manifest = """
            {
              "schema": "supermover.macos.provenance.v1",
              "app_bundle_id": "dev.supermover.macapp",
              "app_version": "0.1.0",
              "build_profile": "test",
              "git_commit": "\(provenanceGitCommit)",
              "git_dirty": true,
              "cli_version": "\(provenanceCLIVersion)",
              "cli_relative_path": "\(provenanceCLIPath)",
              "built_at": "2026-06-01T00:00:00Z",
              "signing": "-"
            }
            """
            try manifest.write(to: provenancePath, atomically: true, encoding: .utf8)
        }

        let iconURL = resourcesURL.appendingPathComponent("SuperMover.icns")
        try Data(repeating: 0, count: 16).write(to: iconURL)

        let appEntitlementsURL = try writeEntitlements(
            appEntitlements,
            name: "app.entitlements",
            in: rootURL
        )
        let cliEntitlementsURL = try writeEntitlements(
            cliEntitlements,
            name: "cli.entitlements",
            in: rootURL
        )

        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/usr/bin/codesign"),
            arguments: ["--force", "--options", "runtime", "--entitlements", cliEntitlementsURL.path, "--sign", "-", cliURL.path],
            file: file,
            line: line
        )
        _ = try runProcess(
            executableURL: URL(fileURLWithPath: "/usr/bin/codesign"),
            arguments: ["--force", "--options", "runtime", "--entitlements", appEntitlementsURL.path, "--sign", "-", appURL.path],
            file: file,
            line: line
        )

        return (rootURL, appURL)
    }

    func writeEntitlements(_ values: [String: Bool], name: String, in directory: URL) throws -> URL {
        let entitlementsURL = directory.appendingPathComponent(name)
        let body = values.keys.sorted().map { key in
            let value = values[key] == true ? "<true/>" : "<false/>"
            return "    <key>\(key)</key>\n    \(value)"
        }.joined(separator: "\n")
        let plist = """
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
        <dict>
        \(body)
        </dict>
        </plist>
        """
        try plist.write(to: entitlementsURL, atomically: true, encoding: .utf8)
        return entitlementsURL
    }

    func writeCanonicalNotarizationSidecar(
        appURL: URL,
        sidecarAppPath: String,
        auditAppPath: String,
        auditProvenanceManifest: [String: Any],
        status: String = "pass",
        submissionID: String? = "11111111-1111-1111-1111-111111111111",
        submissionStatus: String = "Accepted",
        authMode: String? = "keychain_profile",
        failure: [String: Any]? = nil,
        includeNotaryLog: Bool = true,
        auditStatus: String = "pass",
        auditReadiness: String = "distribution_ready",
        auditPassReady: Bool = true
    ) throws {
        let sidecarDirectoryURL = appURL
            .deletingLastPathComponent()
            .appendingPathComponent("\(appURL.lastPathComponent).notary", isDirectory: true)
        try FileManager.default.createDirectory(at: sidecarDirectoryURL, withIntermediateDirectories: true)

        let auditURL = sidecarDirectoryURL.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: auditAppPath,
            provenanceManifest: auditProvenanceManifest,
            status: auditStatus,
            readiness: auditReadiness,
            passReady: auditPassReady
        ).write(to: auditURL, atomically: true, encoding: .utf8)

        let notarizationJSON = try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: sidecarAppPath,
            auditPath: auditURL.path,
            status: status,
            submissionID: submissionID,
            submissionStatus: submissionStatus,
            authMode: authMode,
            includeNotaryLog: includeNotaryLog,
            auditStatus: auditStatus,
            auditReadiness: auditReadiness,
            auditPassReady: auditPassReady
        )
        if let failure {
            var document = try JSONSerialization.jsonObject(
                with: Data(notarizationJSON.utf8)
            ) as? [String: Any]
            document?["failure"] = failure
            try AcceptanceReleaseEvidenceFixtures.jsonString(document ?? [:]).write(
                to: sidecarDirectoryURL.appendingPathComponent("notarization.json"),
                atomically: true,
                encoding: .utf8
            )
        } else {
            try notarizationJSON.write(
                to: sidecarDirectoryURL.appendingPathComponent("notarization.json"),
                atomically: true,
                encoding: .utf8
            )
        }
    }

    func canonicalNotaryDirectoryURL(for appURL: URL) -> URL {
        appURL
            .deletingLastPathComponent()
            .appendingPathComponent("\(appURL.lastPathComponent).notary", isDirectory: true)
    }

    func canonicalNotarizationSidecarURL(for appURL: URL) -> URL {
        canonicalNotaryDirectoryURL(for: appURL).appendingPathComponent("notarization.json")
    }

    func canonicalPostStapleAuditURL(for appURL: URL) -> URL {
        canonicalNotaryDirectoryURL(for: appURL).appendingPathComponent("post-staple.audit.json")
    }

    func canonicalNotaryLogURL(for appURL: URL) -> URL {
        canonicalNotaryDirectoryURL(for: appURL).appendingPathComponent("notary-log.json")
    }

    func replaceItemWithSymlink(at url: URL, destination: URL) throws {
        try FileManager.default.removeItem(at: url)
        try FileManager.default.createSymbolicLink(at: url, withDestinationURL: destination)
    }

    func runAppAudit(
        appURL: URL,
        file: StaticString = #filePath,
        line: UInt = #line
    ) throws -> (exitCode: Int32, status: String, blockedCheckIDs: Set<String>) {
        let repoRoot = repositoryRootURL(file: file, line: line)
        let result = try runProcess(
            executableURL: repoRoot.appendingPathComponent("macos/script/audit-app.sh"),
            arguments: [appURL.path],
            currentDirectoryURL: repoRoot,
            allowNonZeroExit: true,
            file: file,
            line: line
        )
        let data = try XCTUnwrap(result.stdout.data(using: .utf8), file: file, line: line)
        let payload = try JSONSerialization.jsonObject(with: data) as? [String: Any]
        let json = try XCTUnwrap(payload, file: file, line: line)
        let status = try XCTUnwrap(json["status"] as? String, file: file, line: line)
        let checks = try XCTUnwrap(json["checks"] as? [[String: Any]], file: file, line: line)
        let blocked = Set(checks.compactMap { check -> String? in
            guard let checkStatus = check["status"] as? String, checkStatus == "blocked" else {
                return nil
            }
            return check["id"] as? String
        })
        return (result.exitCode, status, blocked)
    }

    func runProcess(
        executableURL: URL,
        arguments: [String],
        currentDirectoryURL: URL? = nil,
        allowNonZeroExit: Bool = false,
        file: StaticString = #filePath,
        line: UInt = #line
    ) throws -> (stdout: String, stderr: String, exitCode: Int32) {
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: executableURL,
            arguments: arguments,
            environment: [:],
            currentDirectoryURL: currentDirectoryURL ?? repositoryRootURL(file: file, line: line)
        )
        if !allowNonZeroExit {
            XCTAssertEqual(
                result.exitCode,
                0,
                "command failed: \(executableURL.path) \(arguments.joined(separator: " "))\nstdout:\n\(result.stdout)\nstderr:\n\(result.stderr)",
                file: file,
                line: line
            )
        }
        return (result.stdout, result.stderr, result.exitCode)
    }
}
