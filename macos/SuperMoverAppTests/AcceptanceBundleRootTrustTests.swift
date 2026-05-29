import Foundation
import XCTest
@testable import SuperMoverApp

final class AcceptanceBundleRootTrustTests: XCTestCase {
    func testAcceptanceBundleReaderRejectsSymlinkedMetaFile() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-meta-symlink")
        defer { fixture.cleanup() }

        let outsideMetaURL = fixture.workDirURL.appendingPathComponent("outside-meta.json")
        try validMetaJSON().write(to: outsideMetaURL, atomically: true, encoding: .utf8)
        try FileManager.default.removeItem(at: fixture.bundleRootURL.appendingPathComponent("meta.json"))
        do {
            try FileManager.default.createSymbolicLink(
                at: fixture.bundleRootURL.appendingPathComponent("meta.json"),
                withDestinationURL: outsideMetaURL
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleReader.ReadError,
                .symlinkRejected(fixture.bundleRootURL.appendingPathComponent("meta.json"))
            )
        }
    }

    func testAcceptanceBundleReaderRejectsHardlinkedMetaFile() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-meta-hardlink")
        defer { fixture.cleanup() }

        let outsideMetaURL = fixture.workDirURL.appendingPathComponent("outside-meta.json")
        try validMetaJSON().write(to: outsideMetaURL, atomically: true, encoding: .utf8)
        let metaURL = fixture.bundleRootURL.appendingPathComponent("meta.json")
        try FileManager.default.removeItem(at: metaURL)
        do {
            try FileManager.default.linkItem(at: outsideMetaURL, to: metaURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleReader.ReadError,
                .malformedMeta(metaURL)
            )
        }
    }

    func testAcceptanceBundleReaderRejectsSpecialMetaFile() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-meta-special")
        defer { fixture.cleanup() }

        let metaURL = fixture.bundleRootURL.appendingPathComponent("meta.json")
        try FileManager.default.removeItem(at: metaURL)
        try makeFIFO(at: metaURL)

        XCTAssertThrowsError(
            try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleReader.ReadError,
                .malformedMeta(metaURL)
            )
        }
    }

    func testOperatorEvidenceStoreRejectsHardlinkedMetaBeforeWriting() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-operator-meta-hardlink")
        defer { fixture.cleanup() }

        let outsideMetaURL = fixture.workDirURL.appendingPathComponent("outside-meta.json")
        try validMetaJSON().write(to: outsideMetaURL, atomically: true, encoding: .utf8)
        let metaURL = fixture.bundleRootURL.appendingPathComponent("meta.json")
        try FileManager.default.removeItem(at: metaURL)
        do {
            try FileManager.default.linkItem(at: outsideMetaURL, to: metaURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleMetaStore().recordOperatorEvidence(
                bundleRootURL: fixture.bundleRootURL,
                record: .init(
                    kind: "local_network",
                    status: "pass",
                    detail: "operator confirmed prompt",
                    artifact: nil
                )
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleMetaStore.MutationError,
                .malformedMeta(metaURL)
            )
        }

        let outsideMeta = try String(contentsOf: outsideMetaURL)
        XCTAssertFalse(outsideMeta.contains("local_network"))
    }

    func testOperatorEvidenceStoreRejectsSpecialMetaBeforeWriting() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-operator-meta-special")
        defer { fixture.cleanup() }

        let metaURL = fixture.bundleRootURL.appendingPathComponent("meta.json")
        try FileManager.default.removeItem(at: metaURL)
        try makeFIFO(at: metaURL)

        XCTAssertThrowsError(
            try AcceptanceBundleMetaStore().recordOperatorEvidence(
                bundleRootURL: fixture.bundleRootURL,
                record: .init(
                    kind: "local_network",
                    status: "pass",
                    detail: "operator confirmed prompt",
                    artifact: nil
                )
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleMetaStore.MutationError,
                .malformedMeta(metaURL)
            )
        }
    }

    func testAcceptanceBundleReaderRejectsSymlinkedBundleRoot() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-root-symlink")
        defer { fixture.cleanup() }

        let symlinkURL = fixture.workDirURL.appendingPathComponent("bundle-link", isDirectory: true)
        do {
            try FileManager.default.createSymbolicLink(at: symlinkURL, withDestinationURL: fixture.bundleRootURL)
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleReader().load(bundleRootURL: symlinkURL)
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleReader.ReadError,
                .symlinkRejected(symlinkURL)
            )
        }
    }

    func testLaunchBundleContextFailsClosedWhenAcceptanceBundleRootIsSymlinked() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-context-root-symlink")
        defer { fixture.cleanup() }

        let symlinkURL = fixture.workDirURL.appendingPathComponent("bundle-link", isDirectory: true)
        do {
            try FileManager.default.createSymbolicLink(at: symlinkURL, withDestinationURL: fixture.bundleRootURL)
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        let context = AcceptanceInstalledAppLaunchBundleContext.resolve(
            bundlePath: symlinkURL.path,
            loadedSnapshot: nil,
            reader: AcceptanceBundleReader()
        )

        guard case let .invalid(path, detail) = context.state else {
            return XCTFail("expected invalid bundle state, got \(context.state)")
        }
        XCTAssertEqual(path, symlinkURL.path)
        XCTAssertTrue(detail.localizedCaseInsensitiveContains("symlink"))
    }

    func testAcceptanceBundleArtifactWriterRejectsSymlinkedBundleRootBeforeWriting() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-writer-root-symlink")
        defer { fixture.cleanup() }

        let symlinkURL = fixture.workDirURL.appendingPathComponent("bundle-link", isDirectory: true)
        do {
            try FileManager.default.createSymbolicLink(at: symlinkURL, withDestinationURL: fixture.bundleRootURL)
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleArtifactWriter().writeStructuredJSONArtifact(
                .init(bundleRootURL: symlinkURL, fileName: "artifact.json", rawJSON: #"{"ok":true}"#)
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleMetaStore.MutationError,
                .symlinkRejected(symlinkURL)
            )
        }

        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("artifact.json").path
            )
        )
    }

    func testAcceptanceBundleArtifactWriterRejectsSymlinkedMetaBeforeWriting() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-writer-meta-symlink")
        defer { fixture.cleanup() }

        let outsideMetaURL = fixture.workDirURL.appendingPathComponent("outside-meta.json")
        try validMetaJSON().write(to: outsideMetaURL, atomically: true, encoding: .utf8)
        try FileManager.default.removeItem(at: fixture.bundleRootURL.appendingPathComponent("meta.json"))
        do {
            try FileManager.default.createSymbolicLink(
                at: fixture.bundleRootURL.appendingPathComponent("meta.json"),
                withDestinationURL: outsideMetaURL
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleArtifactWriter().writeStructuredJSONArtifact(
                .init(bundleRootURL: fixture.bundleRootURL, fileName: "artifact.json", rawJSON: #"{"ok":true}"#)
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleMetaStore.MutationError,
                .symlinkRejected(fixture.bundleRootURL.appendingPathComponent("meta.json"))
            )
        }

        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("artifact.json").path
            )
        )
    }

    func testPackagingEvidenceCollectorRejectsSymlinkedBundleRootBeforeWritingOutputs() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-collector-root-symlink")
        let resourcesURL = try makeFakePackagedResources(named: "packaged-resources")
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        let symlinkURL = fixture.workDirURL.appendingPathComponent("bundle-link", isDirectory: true)
        do {
            try FileManager.default.createSymbolicLink(at: symlinkURL, withDestinationURL: fixture.bundleRootURL)
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

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
                    "blocking_checks": 1
                  }
                }
                """.write(to: outputURL, atomically: true, encoding: .utf8)
                return .init(
                    exitCode: 1,
                    status: "blocked",
                    readiness: "blocked",
                    passReady: false,
                    blockingChecks: 1
                )
            }
        )

        XCTAssertThrowsError(
            try collector.recordCurrentMachineEvidence(
                bundleRootURL: symlinkURL,
                machine: "source",
                collectedBy: "test",
                resourceURL: resourcesURL
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptancePackagingEvidenceCollector.CollectionError,
                .symlinkRejected(symlinkURL.path)
            )
        }

        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.version.txt").path
            )
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.provenance.json").path
            )
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.app-audit.json").path
            )
        )
    }

    func testPackagingEvidenceCollectorRejectsSymlinkedMetaBeforeWritingOutputs() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-collector-meta-symlink")
        let resourcesURL = try makeFakePackagedResources(named: "packaged-resources-meta")
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        let outsideMetaURL = fixture.workDirURL.appendingPathComponent("outside-meta.json")
        try validMetaJSON().write(to: outsideMetaURL, atomically: true, encoding: .utf8)
        try FileManager.default.removeItem(at: fixture.bundleRootURL.appendingPathComponent("meta.json"))
        do {
            try FileManager.default.createSymbolicLink(
                at: fixture.bundleRootURL.appendingPathComponent("meta.json"),
                withDestinationURL: outsideMetaURL
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

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
                    "blocking_checks": 1
                  }
                }
                """.write(to: outputURL, atomically: true, encoding: .utf8)
                return .init(
                    exitCode: 1,
                    status: "blocked",
                    readiness: "blocked",
                    passReady: false,
                    blockingChecks: 1
                )
            }
        )

        XCTAssertThrowsError(
            try collector.recordCurrentMachineEvidence(
                bundleRootURL: fixture.bundleRootURL,
                machine: "source",
                collectedBy: "test",
                resourceURL: resourcesURL
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptancePackagingEvidenceCollector.CollectionError,
                .symlinkRejected(fixture.bundleRootURL.appendingPathComponent("meta.json").path)
            )
        }

        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.version.txt").path
            )
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.provenance.json").path
            )
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.app-audit.json").path
            )
        )
    }

    func testPackagingEvidenceCollectorRejectsSymlinkedNotarizationOutputLeaf() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-collector-notary-output-symlink")
        let resourcesURL = try makeFakePackagedResources(named: "packaged-resources-notary-output")
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        let appBundleURL = resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
        let sidecarDir = appBundleURL
            .deletingLastPathComponent()
            .appendingPathComponent("\(appBundleURL.lastPathComponent).notary", isDirectory: true)
        try FileManager.default.createDirectory(at: sidecarDir, withIntermediateDirectories: true)
        let provenanceManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(
            appBundleURL: appBundleURL
        )
        let auditURL = sidecarDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appBundleURL.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditURL, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appBundleURL.path,
            auditPath: auditURL.path
        ).write(
            to: sidecarDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let outsideOutputURL = fixture.workDirURL.appendingPathComponent("outside-notarization.json")
        try "outside\n".write(to: outsideOutputURL, atomically: true, encoding: .utf8)
        do {
            try FileManager.default.createSymbolicLink(
                at: fixture.bundleRootURL.appendingPathComponent("source.notarization.json"),
                withDestinationURL: outsideOutputURL
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        let collector = AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
                    appPath: appBundleURL.path,
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
                bundleRootURL: fixture.bundleRootURL,
                machine: "source",
                collectedBy: "test",
                resourceURL: resourcesURL
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptancePackagingEvidenceCollector.CollectionError,
                .symlinkRejected(fixture.bundleRootURL.appendingPathComponent("source.notarization.json").path)
            )
        }

        XCTAssertEqual(
            try? String(contentsOf: outsideOutputURL, encoding: .utf8),
            "outside\n"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("workflow.summary.json").path
            )
        )
    }

    func testPackagingEvidenceCollectorRejectsHardlinkedNotaryLogOutputLeafBeforeWriting() throws {
        let fixture = try makeBundleRootFixture(named: "bundle-collector-notary-log-output-hardlink")
        let resourcesURL = try makeFakePackagedResources(named: "packaged-resources-notary-log-output")
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        let appBundleURL = resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
        let sidecarDir = appBundleURL
            .deletingLastPathComponent()
            .appendingPathComponent("\(appBundleURL.lastPathComponent).notary", isDirectory: true)
        try FileManager.default.createDirectory(at: sidecarDir, withIntermediateDirectories: true)
        let provenanceManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(
            appBundleURL: appBundleURL
        )
        let auditURL = sidecarDir.appendingPathComponent("post-staple.audit.json")
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appBundleURL.path,
            provenanceManifest: provenanceManifest
        ).write(to: auditURL, atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appBundleURL.path,
            auditPath: auditURL.path
        ).write(
            to: sidecarDir.appendingPathComponent("notarization.json"),
            atomically: true,
            encoding: .utf8
        )

        let outsideOutputURL = fixture.workDirURL.appendingPathComponent("outside-notary-log.json")
        try "outside\n".write(to: outsideOutputURL, atomically: true, encoding: .utf8)
        let hardlinkedOutputURL = fixture.bundleRootURL.appendingPathComponent("source.notary-log.json")
        do {
            try FileManager.default.linkItem(at: outsideOutputURL, to: hardlinkedOutputURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        let collector = AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
                    appPath: appBundleURL.path,
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
                bundleRootURL: fixture.bundleRootURL,
                machine: "source",
                collectedBy: "test",
                resourceURL: resourcesURL
            )
        ) { error in
            XCTAssertTrue(
                error.localizedDescription.contains("unsafe output artifact"),
                "unexpected error: \(error)"
            )
            XCTAssertTrue(
                error.localizedDescription.contains(hardlinkedOutputURL.path),
                "unexpected error: \(error)"
            )
        }

        XCTAssertEqual(
            try? String(contentsOf: outsideOutputURL, encoding: .utf8),
            "outside\n"
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.version.txt").path
            )
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.provenance.json").path
            )
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.app-audit.json").path
            )
        )
    }

    private func makeBundleRootFixture(named name: String) throws -> BundleRootFixture {
        let workDirURL = try AcceptanceScriptHarness.makeDirectory(named: name)
        let bundleRootURL = workDirURL.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRootURL, withIntermediateDirectories: true)
        try validMetaJSON().write(
            to: bundleRootURL.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        return .init(workDirURL: workDirURL, bundleRootURL: bundleRootURL)
    }

    private func makeFakePackagedResources(named name: String) throws -> URL {
        let appRoot = FileManager.default.temporaryDirectory.appendingPathComponent(
            "SuperMover-\(name)-\(UUID().uuidString).app",
            isDirectory: true
        )
        let resourcesURL = appRoot.appendingPathComponent("Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: resourcesURL, withIntermediateDirectories: true)
        try AcceptanceReleaseEvidenceFixtures.jsonString(
            AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(
                cliVersion: "supermover 0.1.0-dev"
            )
        ).write(
            to: resourcesURL.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        return resourcesURL
    }

    private func validMetaJSON() -> String {
        """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """
    }

    private func makeFIFO(at url: URL) throws {
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/usr/bin/mkfifo"),
            arguments: [url.path],
            environment: [:],
            currentDirectoryURL: url.deletingLastPathComponent()
        )
        if result.exitCode != 0 {
            throw XCTSkip("mkfifo unavailable: \(result.stderr)")
        }
    }
}

private struct BundleRootFixture {
    let workDirURL: URL
    let bundleRootURL: URL

    func cleanup() {
        try? FileManager.default.removeItem(at: workDirURL)
    }
}
