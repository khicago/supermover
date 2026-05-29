import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppNotarizationSourceTests: XCTestCase {
    func testCollectorIgnoresBundledFallbackNotarizationWithoutSiblingSidecar() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithBundledFallbackNotarization()
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        let outputs = try AcceptancePackagingEvidenceCollector(
            versionRunner: { _ in "supermover 0.1.0-dev\n" },
            auditRunner: { _, outputURL in
                try self.writeCurrentAudit(outputURL: outputURL, resources: resources)
                return self.currentAuditResult()
            }
        ).recordCurrentMachineEvidence(
            bundleRootURL: bundleRoot,
            machine: "source",
            collectedBy: "test",
            resourceURL: resources
        )

        XCTAssertEqual(outputs, ["source.version.txt", "source.provenance.json", "source.app-audit.json"])
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertNil(snapshot.sourceNotarization)
        XCTAssertNil(snapshot.sourceNotarizationArtifact)
    }

    func testCollectorRejectsSiblingNotarizationWhenAuditProvenanceDoesNotMatchCurrentApp() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "stale00000000"
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        XCTAssertThrowsError(
            try AcceptancePackagingEvidenceCollector(
                versionRunner: { _ in "supermover 0.1.0-dev\n" },
                auditRunner: { _, outputURL in
                    try self.writeCurrentAudit(outputURL: outputURL, resources: resources)
                    return self.currentAuditResult()
                }
            ).recordCurrentMachineEvidence(
                bundleRootURL: bundleRoot,
                machine: "source",
                collectedBy: "test",
                resourceURL: resources
            )
        ) { error in
            let appRoot = resources.deletingLastPathComponent().deletingLastPathComponent()
            let expectedSidecar = appRoot
                .deletingLastPathComponent()
                .appendingPathComponent("\(appRoot.lastPathComponent).notary/notarization.json")

            XCTAssertEqual(
                error as? AcceptancePackagingEvidenceCollector.CollectionError,
                .staleNotarizationOutput(expectedSidecar.path)
            )
        }
    }

    func testCollectorRejectsSiblingNotarizationWhenSidecarAppPathDoesNotMatchCurrentApp() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "abcdef123456",
            sidecarAppPath: "/tmp/stale/SuperMover.app"
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        XCTAssertThrowsError(
            try AcceptancePackagingEvidenceCollector(
                versionRunner: { _ in "supermover 0.1.0-dev\n" },
                auditRunner: { _, outputURL in
                    try self.writeCurrentAudit(outputURL: outputURL, resources: resources)
                    return self.currentAuditResult()
                }
            ).recordCurrentMachineEvidence(
                bundleRootURL: bundleRoot,
                machine: "source",
                collectedBy: "test",
                resourceURL: resources
            )
        ) { error in
            let appRoot = resources.deletingLastPathComponent().deletingLastPathComponent()
            let expectedSidecar = appRoot
                .deletingLastPathComponent()
                .appendingPathComponent("\(appRoot.lastPathComponent).notary/notarization.json")

            XCTAssertEqual(
                error as? AcceptancePackagingEvidenceCollector.CollectionError,
                .staleNotarizationOutput(expectedSidecar.path)
            )
        }
    }

    func testCollectorRejectsSiblingNotarizationWhenAuditPathEscapesSiblingSidecarDirectory() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let externalAuditURL = FileManager.default.temporaryDirectory
            .appendingPathComponent("external-post-staple-\(UUID().uuidString).audit.json")
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "abcdef123456",
            sidecarAuditPath: externalAuditURL.path
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
            try? FileManager.default.removeItem(at: externalAuditURL)
        }

        XCTAssertThrowsError(
            try AcceptancePackagingEvidenceCollector(
                versionRunner: { _ in "supermover 0.1.0-dev\n" },
                auditRunner: { _, outputURL in
                    try self.writeCurrentAudit(outputURL: outputURL, resources: resources)
                    return self.currentAuditResult()
                }
            ).recordCurrentMachineEvidence(
                bundleRootURL: bundleRoot,
                machine: "source",
                collectedBy: "test",
                resourceURL: resources
            )
        ) { error in
            let appRoot = resources.deletingLastPathComponent().deletingLastPathComponent()
            let expectedSidecar = appRoot
                .deletingLastPathComponent()
                .appendingPathComponent("\(appRoot.lastPathComponent).notary/notarization.json")

            XCTAssertEqual(
                error as? AcceptancePackagingEvidenceCollector.CollectionError,
                .staleNotarizationOutput(expectedSidecar.path)
            )
        }
    }

    func testCollectorRejectsSiblingNotarizationWhenCanonicalSidecarLeafIsSymlinked() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "abcdef123456"
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        let appRoot = resources.deletingLastPathComponent().deletingLastPathComponent()
        let externalSidecarURL = appRoot.deletingLastPathComponent().appendingPathComponent("external-notarization.json")
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appRoot.path,
            auditPath: canonicalPostStapleAuditURL(for: appRoot).path
        ).write(to: externalSidecarURL, atomically: true, encoding: .utf8)
        try replaceItemWithSymlink(
            at: canonicalNotarizationSidecarURL(for: appRoot),
            destination: externalSidecarURL
        )

        XCTAssertThrowsError(
            try AcceptancePackagingEvidenceCollector(
                versionRunner: { _ in "supermover 0.1.0-dev\n" },
                auditRunner: { _, outputURL in
                    try self.writeCurrentAudit(outputURL: outputURL, resources: resources)
                    return self.currentAuditResult()
                }
            ).recordCurrentMachineEvidence(
                bundleRootURL: bundleRoot,
                machine: "source",
                collectedBy: "test",
                resourceURL: resources
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptancePackagingEvidenceCollector.CollectionError,
                .symlinkRejected(canonicalNotarizationSidecarURL(for: appRoot).path)
            )
        }
    }

    func testCollectorRejectsSiblingNotarizationWhenCanonicalAuditLeafIsSymlinkedAndClearsStaleCopiedEvidence() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "abcdef123456"
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        let appRoot = resources.deletingLastPathComponent().deletingLastPathComponent()
        let externalAuditURL = appRoot.deletingLastPathComponent().appendingPathComponent("external-post-staple.audit.json")
        let provenanceObject = try JSONSerialization.jsonObject(
            with: try Data(contentsOf: resources.appendingPathComponent("supermover-provenance.json"))
        )
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appRoot.path,
            provenanceManifest: try XCTUnwrap(provenanceObject as? [String: Any])
        ).write(to: externalAuditURL, atomically: true, encoding: .utf8)
        try replaceItemWithSymlink(
            at: canonicalPostStapleAuditURL(for: appRoot),
            destination: externalAuditURL
        )

        XCTAssertThrowsError(
            try AcceptancePackagingEvidenceCollector(
                versionRunner: { _ in "supermover 0.1.0-dev\n" },
                auditRunner: { _, outputURL in
                    try self.writeCurrentAudit(outputURL: outputURL, resources: resources)
                    return self.currentAuditResult()
                }
            ).recordCurrentMachineEvidence(
                bundleRootURL: bundleRoot,
                machine: "source",
                collectedBy: "test",
                resourceURL: resources
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptancePackagingEvidenceCollector.CollectionError,
                .symlinkRejected(canonicalPostStapleAuditURL(for: appRoot).path)
            )
        }

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundleRoot)
        XCTAssertNil(snapshot.sourceNotarization)
        XCTAssertNil(snapshot.sourceNotarizationArtifact)
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("source.notarization.json").path))
    }

    @MainActor
    func testLaunchPreviewBlocksWhenBundledFallbackNotarizationIsMissingLocalEvidence() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithBundledFallbackNotarization()
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
        XCTAssertTrue(preview.detail.contains("local source notarization evidence is missing"))
    }

    @MainActor
    func testLaunchPreviewBlocksWhenSiblingNotarizationDoesNotMatchCurrentApp() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "stale00000000"
        )
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
        XCTAssertTrue(preview.detail.contains("does not match the current packaged app"))
    }

    @MainActor
    func testLaunchPreviewBlocksWhenSiblingNotarizationAuditPathEscapesCanonicalSidecarDirectory() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let externalAuditURL = FileManager.default.temporaryDirectory
            .appendingPathComponent("external-post-staple-\(UUID().uuidString).audit.json")
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "abcdef123456",
            sidecarAuditPath: externalAuditURL.path
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
            try? FileManager.default.removeItem(at: externalAuditURL)
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
        XCTAssertTrue(preview.detail.contains("does not match the current packaged app"))
    }

    @MainActor
    func testLaunchPreviewBlocksWhenSiblingNotarizationAppPathDoesNotMatchCurrentApp() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "abcdef123456",
            sidecarAppPath: "/tmp/stale/SuperMover.app"
        )
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
        XCTAssertTrue(preview.detail.contains("does not match the current packaged app"))
    }

    @MainActor
    func testLaunchPreviewBlocksWhenSiblingNotarizationUsesUnsafeSymlink() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "abcdef123456"
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        let appRoot = resources.deletingLastPathComponent().deletingLastPathComponent()
        let externalSidecarURL = appRoot.deletingLastPathComponent().appendingPathComponent("external-notarization.json")
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appRoot.path,
            auditPath: canonicalPostStapleAuditURL(for: appRoot).path
        ).write(to: externalSidecarURL, atomically: true, encoding: .utf8)
        try replaceItemWithSymlink(
            at: canonicalNotarizationSidecarURL(for: appRoot),
            destination: externalSidecarURL
        )

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
        XCTAssertTrue(preview.detail.contains("unsafe symlink"))
    }

    @MainActor
    func testLaunchPreflightBlocksWithoutWritingWhenSiblingNotarizationUsesUnsafeSymlink() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "abcdef123456"
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        let appRoot = resources.deletingLastPathComponent().deletingLastPathComponent()
        let externalSidecarURL = appRoot.deletingLastPathComponent().appendingPathComponent("external-notarization.json")
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appRoot.path,
            auditPath: canonicalPostStapleAuditURL(for: appRoot).path
        ).write(to: externalSidecarURL, atomically: true, encoding: .utf8)
        try replaceItemWithSymlink(
            at: canonicalNotarizationSidecarURL(for: appRoot),
            destination: externalSidecarURL
        )

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { _, outputURL in
                        try self.writeCurrentAudit(outputURL: outputURL, resources: resources)
                        return self.currentAuditResult()
                    }
                )
            }
        )
        store.cliProvenance = makeCLIProvenance(resources: resources)
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(error.contains("unsafe symlink"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: bundleRoot.appendingPathComponent("source.version.txt").path
            )
        )
    }

    @MainActor
    func testLaunchPreflightFailsClosedWhenOnlyBundledFallbackNotarizationExists() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithBundledFallbackNotarization()
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { _, outputURL in
                        try self.writeCurrentAudit(outputURL: outputURL, resources: resources)
                        return self.currentAuditResult()
                    }
                )
            }
        )
        store.cliProvenance = makeCLIProvenance(resources: resources)
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(error.contains("require release-ready source notarization evidence"))
        XCTAssertTrue(error.contains("source.notarization.json"))
    }

    @MainActor
    func testLaunchPreflightFailsClosedWhenSiblingNotarizationAuditDoesNotMatchCurrentApp() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "stale00000000"
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { _, outputURL in
                        try self.writeCurrentAudit(outputURL: outputURL, resources: resources)
                        return self.currentAuditResult()
                    }
                )
            }
        )
        store.cliProvenance = makeCLIProvenance(resources: resources)
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(error.contains("does not match the current packaged app"))
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarization?.output, "source.notarization.json")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarizationArtifact?.status, "pass")
    }

    @MainActor
    func testLaunchPreflightFailsClosedWhenSiblingNotarizationAuditPathEscapesSiblingSidecarDirectory() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let externalAuditURL = FileManager.default.temporaryDirectory
            .appendingPathComponent("external-post-staple-\(UUID().uuidString).audit.json")
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "abcdef123456",
            sidecarAuditPath: externalAuditURL.path
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
            try? FileManager.default.removeItem(at: externalAuditURL)
        }

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { _, outputURL in
                        try self.writeCurrentAudit(outputURL: outputURL, resources: resources)
                        return self.currentAuditResult()
                    }
                )
            }
        )
        store.cliProvenance = makeCLIProvenance(resources: resources)
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(error.contains("does not match the current packaged app"))
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarization?.output, "source.notarization.json")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarizationArtifact?.status, "pass")
    }

    @MainActor
    func testLaunchPreflightFailsClosedWhenSiblingNotarizationAppPathDoesNotMatchCurrentApp() throws {
        let bundleRoot = try makeAcceptanceBundle(includeLoadedNotarizationEvidence: true)
        let resources = try makeFakePackagedResourcesWithSiblingNotarization(
            currentGitCommit: "abcdef123456",
            auditGitCommit: "abcdef123456",
            sidecarAppPath: "/tmp/stale/SuperMover.app"
        )
        defer {
            try? FileManager.default.removeItem(at: bundleRoot)
            try? FileManager.default.removeItem(at: resources.deletingLastPathComponent().deletingLastPathComponent())
        }

        let store = AppStore()
        store.acceptanceBundleOperations = AcceptanceBundleAppOperations(
            resourceURLProvider: { resources },
            packagingCollectorFactory: {
                AcceptancePackagingEvidenceCollector(
                    versionRunner: { _ in "supermover 0.1.0-dev\n" },
                    auditRunner: { _, outputURL in
                        try self.writeCurrentAudit(outputURL: outputURL, resources: resources)
                        return self.currentAuditResult()
                    }
                )
            }
        )
        store.cliProvenance = makeCLIProvenance(resources: resources)
        store.acceptanceBundlePath = bundleRoot.path
        store.refreshAcceptanceBundle()

        let error = try XCTUnwrap(store.acceptanceInstalledAppLaunchPreflightError(for: .pair))

        XCTAssertTrue(error.contains("does not match the current packaged app"))
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarization?.output, "source.notarization.json")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.sourceNotarizationArtifact?.status, "pass")
    }

    private func makeAcceptanceBundle(includeLoadedNotarizationEvidence: Bool) throws -> URL {
        let bundleRoot = FileManager.default.temporaryDirectory.appendingPathComponent(
            "acceptance-bundle-\(UUID().uuidString)",
            isDirectory: true
        )
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)

        let notarizationMeta = includeLoadedNotarizationEvidence ? """
            ,
            "notarization": {
              "source": {
                "collected_by": "loaded",
                "output": "source.notarization.json",
                "status": "pass",
                "audit_status": "pass",
                "audit_readiness": "distribution_ready",
                "audit_pass_ready": true
              }
            }
        """ : ""

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
            }\(notarizationMeta)
          }
        }
        """.write(to: bundleRoot.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let bundleAppPath = "/tmp/current-bundle/SuperMover.app"
        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest).write(
            to: bundleRoot.appendingPathComponent("source.provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: bundleAppPath,
            provenanceManifest: provenanceManifest
        ).write(
            to: bundleRoot.appendingPathComponent("source.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )

        if includeLoadedNotarizationEvidence {
            try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
                appPath: bundleAppPath,
                auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(
                    appPath: bundleAppPath
                ),
                notaryLogPath: "source.notary-log.json"
            ).write(
                to: bundleRoot.appendingPathComponent("source.notarization.json"),
                atomically: true,
                encoding: .utf8
            )
            try AcceptanceReleaseEvidenceFixtures.writeBundleNotaryLog(
                bundleRoot: bundleRoot,
                machine: "source"
            )
        }

        return bundleRoot
    }

    private func makeFakePackagedResourcesWithBundledFallbackNotarization() throws -> URL {
        let fixture = try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
            named: "SuperMover",
            cliVersion: "supermover 0.1.0-dev",
            provenanceManifest: [
                "schema": "supermover.macos.provenance.v1",
                "git_commit": "abcdef123456",
                "cli_version": "supermover 0.1.0-dev",
                "cli_relative_path": "Contents/Resources/bin/supermover",
                "build_profile": "test",
                "signing": "developer-id",
            ],
            includeNotarizationSidecar: false
        )
        let resources = fixture.resourcesURL
        let fallbackDir = resources.appendingPathComponent("release/notary", isDirectory: true)
        try FileManager.default.createDirectory(at: fallbackDir, withIntermediateDirectories: true)

        try """
        {
          "schema": "supermover.macos.notarization.v1",
          "checked_at": "2026-06-01T12:00:00Z",
          "status": "pass",
          "submission": {
            "status": "Accepted"
          },
          "audit": {
            "status": "pass",
            "readiness": "distribution_ready",
            "pass_ready": true,
            "blocking_checks": 0
          }
        }
        """.write(to: fallbackDir.appendingPathComponent("notarization.json"), atomically: true, encoding: .utf8)

        return resources
    }

    private func makeFakePackagedResourcesWithSiblingNotarization(
        currentGitCommit: String,
        auditGitCommit: String,
        sidecarAppPath: String? = nil,
        sidecarAuditPath: String? = nil
    ) throws -> URL {
        let fixture = try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
            named: "SuperMover",
            cliVersion: "supermover 0.1.0-dev",
            provenanceManifest: [
                "schema": "supermover.macos.provenance.v1",
                "git_commit": currentGitCommit,
                "cli_version": "supermover 0.1.0-dev",
                "cli_relative_path": "Contents/Resources/bin/supermover",
                "build_profile": "test",
                "signing": "developer-id",
            ],
            includeNotarizationSidecar: false
        )
        let appRoot = fixture.appURL
        let resources = fixture.resourcesURL
        let notaryDir = fixture.canonicalNotaryDirectoryURL
        try FileManager.default.createDirectory(at: notaryDir, withIntermediateDirectories: true)

        let auditURL = sidecarAuditPath.map { URL(fileURLWithPath: $0) }
            ?? fixture.canonicalPostStapleAuditURL
        try FileManager.default.createDirectory(at: auditURL.deletingLastPathComponent(), withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "pass",
          "readiness": "distribution_ready",
          "app_path": "\(appRoot.path)",
          "provenance": {
            "manifest": {
              "schema": "supermover.macos.provenance.v1",
              "git_commit": "\(auditGitCommit)",
              "cli_version": "supermover 0.1.0-dev",
              "cli_relative_path": "Contents/Resources/bin/supermover",
              "build_profile": "test",
              "signing": "developer-id"
            }
          },
          "summary": {
            "pass_ready": true,
            "blocking_checks": 0
          }
        }
        """.write(to: auditURL, atomically: true, encoding: .utf8)

        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(
            to: fixture.canonicalNotaryLogURL,
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: sidecarAppPath ?? appRoot.path,
            auditPath: auditURL.path,
            notaryLogPath: fixture.canonicalNotaryLogURL.path
        ).write(to: fixture.canonicalNotarizationSidecarURL, atomically: true, encoding: .utf8)

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

    private func currentAuditResult() -> AcceptancePackagingEvidenceCollector.AppAuditResult {
        .init(
            exitCode: 0,
            status: "pass",
            readiness: "distribution_ready",
            passReady: true,
            blockingChecks: 0
        )
    }

    private func writeCurrentAudit(outputURL: URL, resources: URL) throws {
        let provenanceURL = resources.appendingPathComponent("supermover-provenance.json")
        let provenanceData = try Data(contentsOf: provenanceURL)
        let provenanceObject = try JSONSerialization.jsonObject(with: provenanceData)
        guard let provenanceManifest = provenanceObject as? [String: Any] else {
            throw NSError(domain: "AcceptanceInstalledAppNotarizationSourceTests", code: 1)
        }
        let appRoot = resources.deletingLastPathComponent().deletingLastPathComponent()
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appRoot.path,
            provenanceManifest: provenanceManifest
        ).write(
            to: outputURL,
            atomically: true,
            encoding: .utf8
        )
    }
}
