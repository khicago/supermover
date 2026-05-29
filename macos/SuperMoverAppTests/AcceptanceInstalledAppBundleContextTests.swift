import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppBundleContextTests: XCTestCase {
    @MainActor
    func testLaunchPreviewBlocksUnreadableAcceptanceBundle() throws {
        let bundleRoot = try makeMalformedAcceptanceBundle()
        let resources = try makeFakePackagedResources()
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources }
        )
        store.cliProvenance = makeCLIProvenance(resources: resources)
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("require a readable acceptance bundle"))
        XCTAssertTrue(preview.detail.contains("meta.json is malformed"))
    }

    @MainActor
    func testLaunchPreflightBlocksUnreadableAcceptanceBundle() throws {
        let bundleRoot = try makeMalformedAcceptanceBundle()
        let resources = try makeFakePackagedResources()
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources }
        )
        store.cliProvenance = makeCLIProvenance(resources: resources)
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(error.contains("require a readable acceptance bundle"))
        XCTAssertTrue(error.contains("meta.json is malformed"))
    }

    @MainActor
    func testRunSelectedTaskDoesNotLaunchWhenAcceptanceBundleIsUnreadable() throws {
        let bundleRoot = try makeMalformedAcceptanceBundle()
        let profileURL = bundleRoot.appendingPathComponent("profile.json")
        let resources = try makeFakePackagedResources()
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources }
        )
        store.cliProvenance = makeCLIProvenance(resources: resources)
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()
        store.selectedRole = .source
        store.selectedTask = .pair
        try #"{"schema":"supermover.profile.v1"}"#.write(to: profileURL, atomically: true, encoding: .utf8)
        store.profilePath = profileURL.path
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"

        store.runSelectedTask()

        XCTAssertTrue(store.note.contains("require a readable acceptance bundle"))
        XCTAssertTrue(store.recentRuns.isEmpty)
    }

    @MainActor
    func testLaunchGateStillSkipsWhenAcceptanceBundleIsNotConfigured() throws {
        let resources = try makeFakePackagedResources()
        defer { try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent()) }

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources }
        )
        store.cliProvenance = makeCLIProvenance(resources: resources)

        XCTAssertNil(store.acceptanceInstalledAppLaunchPreview(for: .pair))
        XCTAssertNil(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))
    }

    @MainActor
    func testLaunchPreviewReviewsLoadedNotarizationThatDoesNotMatchBundledReleaseEvidence() throws {
        let bundleRoot = try makeAcceptanceBundleWithStaleLoadedSourceNotarization()
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { nil }
        )
        store.cliProvenance = makeCLIProvenance(resources: bundleRoot)
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .review)
        XCTAssertTrue(preview.detail.contains("release-ready"))
    }

    @MainActor
    func testLaunchPreviewReviewsLoadedAuditWhenBundledProvenanceDoesNotMatch() throws {
        let bundleRoot = try makeAcceptanceBundleWithStaleLoadedSourceAuditProvenance()
        defer { try? FileManager.default.removeItem(at: bundleRoot) }

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { nil }
        )
        store.cliProvenance = makeCLIProvenance(resources: bundleRoot)
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()

        let preview = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreview(for: .pair))

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .review)
        XCTAssertTrue(preview.detail.contains("fresh packaging evidence"))
    }

    func testBundleContextReloadsWhenLoadedSnapshotDoesNotMatchCurrentBundlePath() throws {
        let loadedBundle = try makeAcceptanceBundleWithStaleLoadedSourceNotarization()
        let malformedBundle = try makeMalformedAcceptanceBundle()
        defer {
            try? FileManager.default.removeItem(at: loadedBundle)
            try? FileManager.default.removeItem(at: malformedBundle)
        }

        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: loadedBundle)

        let context = AcceptanceInstalledAppLaunchBundleContext.resolve(
            bundlePath: malformedBundle.path,
            loadedSnapshot: loadedSnapshot,
            reader: AcceptanceBundleReader()
        )

        guard case let .invalid(path, detail) = context.state else {
            return XCTFail("expected invalid bundle state, got \(context.state)")
        }
        XCTAssertEqual(path, malformedBundle.path)
        XCTAssertTrue(detail.contains("meta.json is malformed"))
        XCTAssertNil(context.snapshot)
        XCTAssertNil(context.installedAppCollectionProof)
    }

    private func makeMalformedAcceptanceBundle() throws -> URL {
        let bundleRoot = FileManager.default.temporaryDirectory.appendingPathComponent(
            "acceptance-bundle-\(UUID().uuidString)",
            isDirectory: true
        )
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try #"{"schema":"supermover.acceptance.two_machine.v1""#.write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        return bundleRoot
    }

    private func makeAcceptanceBundleWithStaleLoadedSourceNotarization() throws -> URL {
        let bundleRoot = FileManager.default.temporaryDirectory.appendingPathComponent(
            "acceptance-bundle-\(UUID().uuidString)",
            isDirectory: true
        )
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
            "app_audit": {
              "source": {
                "collected_by": "loaded",
                "output": "source.app-audit.json",
                "exit_code": 0,
                "status": "pass",
                "readiness": "distribution_ready",
                "pass_ready": true,
                "blocking_checks": 0
              }
            },
            "notarization": {
              "source": {
                "collected_by": "loaded",
                "output": "source.notarization.json",
                "status": "pass",
                "audit_status": "pass",
                "audit_readiness": "distribution_ready",
                "audit_pass_ready": true
              }
            },
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
            ]
          }
        }
        """.write(
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
        """.write(
            to: bundleRoot.appendingPathComponent("source.machine.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(
            to: bundleRoot.appendingPathComponent("target.machine.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRoot,
            machine: "source",
            appPath: "/tmp/current-source/SuperMover.app"
        )
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: "/tmp/stale-source/SuperMover.app",
            auditPath: "/tmp/stale-source/SuperMover.app.audit.json"
        ).write(
            to: bundleRoot.appendingPathComponent("source.notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        return bundleRoot
    }

    private func makeAcceptanceBundleWithStaleLoadedSourceAuditProvenance() throws -> URL {
        let bundleRoot = try makeAcceptanceBundleWithStaleLoadedSourceNotarization()
        let currentProvenance = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        var staleProvenance = currentProvenance
        staleProvenance["built_at"] = "2026-06-04T00:00:00Z"
        try AcceptanceReleaseEvidenceFixtures.jsonString(staleProvenance).write(
            to: bundleRoot.appendingPathComponent("source.provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: "/tmp/current-source/SuperMover.app",
            auditPath: "/tmp/current-source/SuperMover.app.audit.json"
        ).write(
            to: bundleRoot.appendingPathComponent("source.notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        return bundleRoot
    }

    private func makeFakePackagedResources() throws -> URL {
        let appRoot = FileManager.default.temporaryDirectory.appendingPathComponent(
            "SuperMover-\(UUID().uuidString).app",
            isDirectory: true
        )
        let resources = appRoot.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binDir = resources.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binDir, withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.macos.provenance.v1",
          "git_commit": "abcdef123456",
          "cli_version": "supermover 0.1.0-dev",
          "cli_relative_path": "Contents/Resources/bin/supermover",
          "build_profile": "test",
          "signing": "developer-id"
        }
        """.write(to: resources.appendingPathComponent("supermover-provenance.json"), atomically: true, encoding: .utf8)
        return resources
    }

    private func makeCLIProvenance(resources: URL) -> CLIProvenance {
        CLIProvenance(
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
    }
}
