import XCTest
import SuperMoverAppSupport
@testable import SuperMoverApp

final class PackagedAppAuditorTests: XCTestCase {
    func testAuditorDoesNotBlockWhenCanonicalSidecarIsMissing() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertFalse(report.notarizationSidecar.exists)
        XCTAssertFalse(blocked.contains("notarization.sidecar.exists"))
        XCTAssertFalse(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertFalse(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksStaleCanonicalSidecarWhenPresent() throws {
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

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksMalformedCanonicalSidecarEvenWhenAuditStillMatchesCurrentApp() throws {
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

        let sidecarDirectoryURL = app.appURL
            .deletingLastPathComponent()
            .appendingPathComponent("\(app.appURL.lastPathComponent).notary", isDirectory: true)
        let auditURL = sidecarDirectoryURL.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "schema": "supermover.macos.notarization.v0",
            "status": "pass",
            "app_path": app.appURL.path,
            "submission": [
                "status": "Accepted",
            ],
            "audit": [
                "path": auditURL.path,
                "status": "pass",
                "readiness": "distribution_ready",
                "pass_ready": true,
                "blocking_checks": 0,
            ],
        ]).write(
            to: sidecarDirectoryURL.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertTrue(blocked.contains("notarization.sidecar.schema"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenPostStapleAuditSchemaIsInvalid() throws {
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

        let sidecarDirectoryURL = app.appURL
            .deletingLastPathComponent()
            .appendingPathComponent("\(app.appURL.lastPathComponent).notary", isDirectory: true)
        let auditURL = sidecarDirectoryURL.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "schema": "supermover.macos.app_audit.v0",
            "status": "pass",
            "readiness": "distribution_ready",
            "app_path": app.appURL.path,
            "provenance": [
                "manifest": currentManifest,
            ],
            "summary": [
                "pass_ready": true,
                "blocking_checks": 0,
            ],
        ]).write(to: auditURL, atomically: true, encoding: .utf8)

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertTrue(blocked.contains("notarization.sidecar.audit.schema"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenPostStapleAuditEscapesSiblingSidecarDirectory() throws {
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

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenSubmissionIDIsMissing() throws {
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

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertTrue(blocked.contains("notarization.sidecar.notary_log"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenSubmissionIDIsNotUUID() throws {
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

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertTrue(blocked.contains("notarization.sidecar.notary_log"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenAuthModeIsMissing() throws {
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

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertTrue(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertFalse(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenFailureIsRecorded() throws {
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

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertTrue(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertFalse(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenNotaryLogIsNotAccepted() throws {
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

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertTrue(blocked.contains("notarization.sidecar.notary_log"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenNotaryLogIsMissing() throws {
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

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertTrue(blocked.contains("notarization.sidecar.notary_log"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenNotaryLogIsHardlinked() throws {
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

        let notaryLogURL = canonicalNotaryLogURL(for: app.appURL)
        let outsideURL = app.rootURL.appendingPathComponent("outside-notary-log.json")
        try FileManager.default.removeItem(at: notaryLogURL)
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(to: outsideURL, atomically: true, encoding: .utf8)
        do {
            try FileManager.default.linkItem(at: outsideURL, to: notaryLogURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertTrue(blocked.contains("notarization.sidecar.notary_log"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenSidecarLeafIsSymlinked() throws {
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

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertTrue(blocked.contains("notarization.sidecar.parse"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenPostStapleAuditLeafIsSymlinked() throws {
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

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertTrue(blocked.contains("notarization.sidecar.audit.exists"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorBlocksCanonicalSidecarWhenPostStapleAuditIsHardlinked() throws {
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

        let auditURL = canonicalPostStapleAuditURL(for: app.appURL)
        let outsideURL = app.rootURL.appendingPathComponent("outside-post-staple.audit.json")
        let auditJSON = try String(contentsOf: auditURL, encoding: .utf8)
        try FileManager.default.removeItem(at: auditURL)
        try auditJSON.write(to: outsideURL, atomically: true, encoding: .utf8)
        do {
            try FileManager.default.linkItem(at: outsideURL, to: auditURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))

        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(report.notarizationSidecar.current)
        XCTAssertFalse(report.notarizationSidecar.releaseReady)
        XCTAssertTrue(blocked.contains("notarization.sidecar.audit.exists"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertTrue(blocked.contains("notarization.sidecar.release_ready"))
    }

    func testAuditorTreatsSuccessfulSidecarAsCurrentAfterCustomWorkDirIsRemoved() throws {
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
        let result = try runNotarizeScript(
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

        XCTAssertEqual(result.exitCode, 0)
        try FileManager.default.removeItem(at: workDir)

        let report = PackagedAppAuditor().audit(appURL: app.appURL)
        let blocked = Set(report.checks.filter { $0.status == "blocked" }.map(\.id))
        let sidecarAuditPath = app.appURL
            .deletingLastPathComponent()
            .appendingPathComponent("\(app.appURL.lastPathComponent).notary", isDirectory: true)
            .appendingPathComponent("post-staple.audit.json")

        XCTAssertEqual(report.notarizationSidecar.audit?.path, sidecarAuditPath.path)
        XCTAssertTrue(report.notarizationSidecar.exists)
        XCTAssertFalse(blocked.contains("notarization.sidecar.currentness"))
        XCTAssertFalse(blocked.contains("notarization.sidecar.release_ready"))
    }
}
