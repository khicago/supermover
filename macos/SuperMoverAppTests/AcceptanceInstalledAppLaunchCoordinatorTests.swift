import Foundation
import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppLaunchCoordinatorTests: XCTestCase {
    func testPreviewBlocksWhenRequiredPackagingOutputLeafIsSymlinked() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preview-required-output-symlink",
            symlinkedOutputName: "source.app-audit.json"
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preview-required-output-symlink",
            includeNotarizationSidecar: false
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let preview = try XCTUnwrap(
            coordinator.preview(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("unsafe symlink path"))
        XCTAssertTrue(preview.detail.contains("source.app-audit.json"))
    }

    func testPreflightBlocksBeforeWritingWhenRequiredPackagingOutputLeafIsSymlinked() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preflight-required-output-symlink",
            symlinkedOutputName: "source.app-audit.json"
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preflight-required-output-symlink",
            includeNotarizationSidecar: false
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let error = try XCTUnwrap(
            coordinator.preflightError(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )

        XCTAssertTrue(error.contains("unsafe symlink path"))
        XCTAssertTrue(error.contains("source.app-audit.json"))
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
    }

    func testPreviewBlocksWhenOptionalNotarizationOutputLeafIsSymlinked() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preview-notarization-output-symlink",
            symlinkedOutputName: "source.notarization.json"
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preview-notarization-output-symlink",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            includeNotarization: false
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let preview = try XCTUnwrap(
            coordinator.preview(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("unsafe symlink path"))
        XCTAssertTrue(preview.detail.contains("source.notarization.json"))
    }

    func testPreflightBlocksBeforeWritingWhenOptionalNotarizationOutputLeafIsSymlinked() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preflight-notarization-output-symlink",
            symlinkedOutputName: "source.notarization.json"
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preflight-notarization-output-symlink",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            includeNotarization: false
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let error = try XCTUnwrap(
            coordinator.preflightError(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )

        XCTAssertTrue(error.contains("unsafe symlink path"))
        XCTAssertTrue(error.contains("source.notarization.json"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.version.txt").path
            )
        )
        XCTAssertEqual(
            try? String(contentsOf: fixture.outsideOutputURL, encoding: .utf8),
            "outside\n"
        )
    }

    func testPreviewReviewsWhenPairCanCorrectInstalledAppMachineIdentity() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preview-installed-app-proof-machine-identity-correction",
            metaJSON: bundleMetaJSON(
                sourceMachineID: "same-machine",
                targetMachineID: "same-machine"
            )
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preview-installed-app-proof-machine-identity-correction",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "source",
            includeNotarization: true
        )
        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "target",
            includeNotarization: true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let preview = try XCTUnwrap(
            coordinator.preview(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .review)
        XCTAssertTrue(preview.detail.contains("machine identity correction"))
        XCTAssertTrue(preview.detail.contains("rewrite source_pair"))
    }

    func testPreflightAllowsPairWhenItCanCorrectInstalledAppMachineIdentity() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preflight-installed-app-proof-machine-identity-correction",
            metaJSON: bundleMetaJSON(
                sourceMachineID: "same-machine",
                targetMachineID: "same-machine"
            )
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preflight-installed-app-proof-machine-identity-correction",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "source",
            includeNotarization: true
        )
        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "target",
            includeNotarization: true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let error = coordinator.preflightError(
            for: .pair,
            using: makeDependencies(
                bundleRootURL: fixture.bundleRootURL,
                resourcesURL: resourcesURL,
                loadedSnapshot: loadedSnapshot
            )
        )

        XCTAssertNil(error)
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.version.txt").path
            )
        )
    }

    func testPreviewReviewsWhenServeCanCorrectInstalledAppMachineIdentity() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preview-target-serve-machine-identity-correction",
            metaJSON: bundleMetaJSON(
                sourceMachineID: "same-machine",
                targetMachineID: "same-machine"
            )
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preview-target-serve-machine-identity-correction",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "source",
            includeNotarization: true
        )
        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "target",
            includeNotarization: true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let preview = try XCTUnwrap(
            coordinator.preview(
                for: .serve,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot,
                    selectedRole: .target
                )
            )
        )

        XCTAssertEqual(preview.machine, "target")
        XCTAssertEqual(preview.state, .review)
        XCTAssertTrue(preview.detail.contains("machine identity correction"))
        XCTAssertTrue(preview.detail.contains("rewrite target role"))
    }

    func testPreflightAllowsServeWhenItCanCorrectInstalledAppMachineIdentity() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preflight-target-serve-machine-identity-correction",
            metaJSON: bundleMetaJSON(
                sourceMachineID: "same-machine",
                targetMachineID: "same-machine"
            )
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preflight-target-serve-machine-identity-correction",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "source",
            includeNotarization: true
        )
        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "target",
            includeNotarization: true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let error = coordinator.preflightError(
            for: .serve,
            using: makeDependencies(
                bundleRootURL: fixture.bundleRootURL,
                resourcesURL: resourcesURL,
                loadedSnapshot: loadedSnapshot,
                selectedRole: .target
            )
        )

        XCTAssertNil(error)
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("target.version.txt").path
            )
        )
    }

    func testPreflightStillBlocksUnrelatedTaskWhenInstalledAppMachineIdentityNeedsCorrection() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preflight-installed-app-proof-machine-identity-unrelated-task",
            metaJSON: bundleMetaJSON(
                sourceMachineID: "same-machine",
                targetMachineID: "same-machine"
            )
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preflight-installed-app-proof-machine-identity-unrelated-task",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            includeNotarization: true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let preview = try XCTUnwrap(
            coordinator.preview(
                for: .networkPush,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )
        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("same machine_id"))

        let error = try XCTUnwrap(
            coordinator.preflightError(
                for: .networkPush,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )

        XCTAssertTrue(error.contains("same machine_id"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.version.txt").path
            )
        )
    }

    func testPreviewReviewsPairAndServeWhenMachineFactsAreMissing() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preview-missing-machine-facts-correction"
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preview-missing-machine-facts-correction",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "source",
            includeNotarization: true
        )
        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "target",
            includeNotarization: true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)
        XCTAssertTrue(loadedSnapshot.installedAppCollectionProof.requiresMachineIdentityCorrection)
        XCTAssertNil(loadedSnapshot.installedAppCollectionProof.blockedReason)

        let pairPreview = try XCTUnwrap(
            coordinator.preview(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )
        XCTAssertEqual(pairPreview.machine, "source")
        XCTAssertEqual(pairPreview.state, .review)
        XCTAssertTrue(pairPreview.detail.contains("machine identity correction"))
        XCTAssertTrue(pairPreview.detail.contains("source.machine.json"))

        let servePreview = try XCTUnwrap(
            coordinator.preview(
                for: .serve,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot,
                    selectedRole: .target
                )
            )
        )
        XCTAssertEqual(servePreview.machine, "target")
        XCTAssertEqual(servePreview.state, .review)
        XCTAssertTrue(servePreview.detail.contains("machine identity correction"))
        XCTAssertTrue(servePreview.detail.contains("target.machine.json"))
    }

    func testPreflightAllowsPairAndServeWhenMachineFactsAreMissing() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preflight-missing-machine-facts-correction"
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preflight-missing-machine-facts-correction",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "source",
            includeNotarization: true
        )
        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "target",
            includeNotarization: true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        XCTAssertNil(
            coordinator.preflightError(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.version.txt").path
            )
        )

        let refreshedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)
        XCTAssertNil(
            coordinator.preflightError(
                for: .serve,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: refreshedSnapshot,
                    selectedRole: .target
                )
            )
        )
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("target.version.txt").path
            )
        )
    }

    func testPreflightBlocksUnrelatedTaskWhenMachineFactsAreMissing() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preflight-missing-machine-facts-unrelated-task"
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preflight-missing-machine-facts-unrelated-task",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "source",
            includeNotarization: true
        )
        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "target",
            includeNotarization: true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let error = try XCTUnwrap(
            coordinator.preflightError(
                for: .networkPush,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )

        XCTAssertTrue(error.contains("machine identity correction"))
        XCTAssertTrue(error.contains("source.machine.json"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.version.txt").path
            )
        )
    }

    func testPreflightBlocksBeforeWritingWhenInstalledAppProofRequiresBundleHandoff() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preflight-installed-app-proof-bundle-handoff",
            metaJSON: AcceptanceInstalledAppBundleFixtures.bundleMeta(
                includeReleaseEvidenceMeta: true,
                includeWorkflowSummaryArtifact: false,
                bundleHandoffsJSON: """
                [
                  {
                    "archive": "bundle-other.tgz",
                    "manifest": "bundle-other.manifest.json",
                    "sha256": "3333333333333333333333333333333333333333333333333333333333333333",
                    "meta": "meta.json",
                    "verified": true,
                    "exporting_machine_id": "other-source-machine",
                    "exporting_machine_label": "other-source",
                    "importing_machine_id": "other-target-machine",
                    "importing_machine_label": "other-target"
                  }
                ]
                """
            )
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preflight-installed-app-proof-bundle-handoff",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try AcceptanceInstalledAppBundleFixtures.writeInstalledAppMachineFacts(
            bundleRoot: fixture.bundleRootURL
        )
        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            includeNotarization: true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let error = try XCTUnwrap(
            coordinator.preflightError(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )

        XCTAssertTrue(error.contains("missing distinct-machine archive handoff proof"))
        XCTAssertTrue(error.contains("bundle_handoff pack/unpack/merge procedure"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.version.txt").path
            )
        )
    }

    func testPreviewBlocksWhenOtherMachineReleaseEvidenceIsMissingBeforeMachineIdentityCorrection() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preview-machine-identity-correction-missing-other-release",
            metaJSON: bundleMetaJSON(
                sourceMachineID: "same-machine",
                targetMachineID: "same-machine"
            )
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preview-machine-identity-correction-missing-other-release",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "source",
            includeNotarization: true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let preview = try XCTUnwrap(
            coordinator.preview(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("target packaging evidence"))
        XCTAssertTrue(preview.detail.contains("target.app-audit.json is not install-ready"))
    }

    func testPreflightBlocksWhenOtherMachineReleaseEvidenceIsMissingBeforeMachineIdentityCorrection() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preflight-machine-identity-correction-missing-other-release",
            metaJSON: bundleMetaJSON(
                sourceMachineID: "same-machine",
                targetMachineID: "same-machine"
            )
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preflight-machine-identity-correction-missing-other-release",
            includeNotarizationSidecar: true
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        try writeBundleReleaseEvidence(
            bundleRootURL: fixture.bundleRootURL,
            resourcesURL: resourcesURL,
            machine: "source",
            includeNotarization: true
        )

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let error = try XCTUnwrap(
            coordinator.preflightError(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot
                )
            )
        )

        XCTAssertTrue(error.contains("target packaging evidence"))
        XCTAssertTrue(error.contains("target.app-audit.json is not install-ready"))
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: fixture.bundleRootURL.appendingPathComponent("source.version.txt").path
            )
        )
    }

    func testPreviewBlocksWhenPackagingEvidenceAuditProbeFails() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preview-packaging-audit-probe-failure"
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preview-packaging-audit-probe-failure",
            includeNotarizationSidecar: false
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let preview = try XCTUnwrap(
            coordinator.preview(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot,
                    packagingCollectorFactory: {
                        AcceptancePackagingEvidenceCollector(
                            versionRunner: { _ in "supermover 0.1.0-dev\n" },
                            auditRunner: { _, _ in
                                throw AcceptancePackagingEvidenceCollector.CollectionError.auditScriptUnavailable(
                                    "/tmp/missing-supermover-app-audit"
                                )
                            }
                        )
                    }
                )
            )
        )

        XCTAssertEqual(preview.machine, "source")
        XCTAssertEqual(preview.state, .blocked)
        XCTAssertTrue(preview.detail.contains("Local app audit helper is unavailable"))
    }

    func testPreflightBlocksBeforeWritingWhenPackagingEvidenceAuditProbeFails() throws {
        let fixture = try makeBundleFixture(
            named: "launch-preflight-packaging-audit-probe-failure"
        )
        let resourcesURL = try makeFakePackagedResources(
            named: "launch-preflight-packaging-audit-probe-failure",
            includeNotarizationSidecar: false
        )
        defer {
            fixture.cleanup()
            try? FileManager.default.removeItem(
                at: resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
            )
        }

        let coordinator = AcceptanceInstalledAppLaunchCoordinator()
        let loadedSnapshot = try AcceptanceBundleReader().load(bundleRootURL: fixture.bundleRootURL)

        let error = try XCTUnwrap(
            coordinator.preflightError(
                for: .pair,
                using: makeDependencies(
                    bundleRootURL: fixture.bundleRootURL,
                    resourcesURL: resourcesURL,
                    loadedSnapshot: loadedSnapshot,
                    packagingCollectorFactory: {
                        AcceptancePackagingEvidenceCollector(
                            versionRunner: { _ in "supermover 0.1.0-dev\n" },
                            auditRunner: { _, _ in
                                throw AcceptancePackagingEvidenceCollector.CollectionError.auditScriptUnavailable(
                                    "/tmp/missing-supermover-app-audit"
                                )
                            }
                        )
                    }
                )
            )
        )

        XCTAssertTrue(error.contains("Local app audit helper is unavailable"))
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
    }

    private func makeDependencies(
        bundleRootURL: URL,
        resourcesURL: URL,
        loadedSnapshot: AcceptanceBundleLoadedSnapshot,
        selectedRole: WorkbenchRole = .source,
        packagingCollectorFactory: @escaping () -> AcceptancePackagingEvidenceCollector = {
            AcceptancePackagingEvidenceCollector(
                versionRunner: { _ in "supermover 0.1.0-dev\n" },
                auditRunner: { appBundleURL, outputURL in
                    try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
                        appPath: appBundleURL.path,
                        provenanceManifest: try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(
                            appBundleURL: appBundleURL
                        )
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
        }
    ) -> AcceptanceInstalledAppLaunchCoordinator.Dependencies {
        let snapshotBox = LoadedSnapshotBox(snapshot: loadedSnapshot)
        return AcceptanceInstalledAppLaunchCoordinator.Dependencies(
            currentContext: {
                .init(
                    selectedRole: selectedRole,
                    bundlePath: bundleRootURL.path,
                    loadedSnapshot: snapshotBox.snapshot,
                    cliProvenance: self.makeCLIProvenance(resourcesURL: resourcesURL)
                )
            },
            currentBundleURL: { bundleRootURL },
            refreshBundle: {
                snapshotBox.snapshot = (try? AcceptanceBundleReader().load(bundleRootURL: bundleRootURL)) ?? snapshotBox.snapshot
            },
            acceptanceBundleOperations: AcceptanceBundleAppOperations(
                resourceURLProvider: { resourcesURL },
                packagingCollectorFactory: packagingCollectorFactory
            )
        )
    }

    private func makeBundleFixture(
        named name: String,
        metaJSON: String? = nil,
        symlinkedOutputName: String? = nil
    ) throws -> LaunchBundleFixture {
        let workDirURL = try AcceptanceScriptHarness.makeDirectory(named: name)
        let bundleRootURL = workDirURL.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRootURL, withIntermediateDirectories: true)
        try (metaJSON ?? bundleMetaJSON()).write(
            to: bundleRootURL.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )

        let outsideOutputURL = workDirURL.appendingPathComponent("outside-output.json")
        try "outside\n".write(to: outsideOutputURL, atomically: true, encoding: .utf8)
        if let symlinkedOutputName {
            do {
                try FileManager.default.createSymbolicLink(
                    at: bundleRootURL.appendingPathComponent(symlinkedOutputName),
                    withDestinationURL: outsideOutputURL
                )
            } catch {
                throw XCTSkip("symlink unavailable: \(error)")
            }
        }

        return .init(
            workDirURL: workDirURL,
            bundleRootURL: bundleRootURL,
            outsideOutputURL: outsideOutputURL
        )
    }

    private func makeFakePackagedResources(
        named name: String,
        includeNotarizationSidecar: Bool
    ) throws -> URL {
        try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
            named: "SuperMover-\(name)",
            cliVersion: "supermover 0.1.0-dev",
            includeNotarizationSidecar: includeNotarizationSidecar
        )
            .resourcesURL
    }

    private func writeBundleReleaseEvidence(
        bundleRootURL: URL,
        resourcesURL: URL,
        machine: String = "source",
        includeNotarization: Bool
    ) throws {
        let appBundleURL = resourcesURL.deletingLastPathComponent().deletingLastPathComponent()
        let provenanceManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(
            appBundleURL: appBundleURL
        )
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest).write(
            to: bundleRootURL.appendingPathComponent("\(machine).provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: appBundleURL.path,
            provenanceManifest: provenanceManifest
        ).write(
            to: bundleRootURL.appendingPathComponent("\(machine).app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        guard includeNotarization else {
            return
        }
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: appBundleURL.path,
            auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(
                appPath: appBundleURL.path
            ),
            notaryLogPath: "\(machine).notary-log.json"
        ).write(
            to: bundleRootURL.appendingPathComponent("\(machine).notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.writeBundleNotaryLog(
            bundleRoot: bundleRootURL,
            machine: machine
        )
    }

    private func makeCLIProvenance(resourcesURL: URL) -> CLIProvenance {
        CLIProvenance(
            mode: .bundled,
            readinessLevel: .pass,
            executablePath: resourcesURL.appendingPathComponent("bin/supermover").path,
            workingDirectoryPath: resourcesURL.appendingPathComponent("bin").path,
            bundleIdentifier: "dev.supermover.macapp",
            appVersion: "0.1.0",
            provenancePath: resourcesURL.appendingPathComponent("supermover-provenance.json").path,
            provenanceStatus: "loaded",
            bundleCommit: "deadbeefdead",
            bundledCLIVersion: "supermover 0.1.0-dev",
            buildProfile: "test",
            signing: "Developer ID Application: Test (TEAMID1234)",
            gitDirty: false,
            builtAt: "2026-06-03T00:00:00Z",
            readiness: "distribution_ready",
            detail: "distribution_ready"
        )
    }

    private func bundleMetaJSON(
        sourceMachineID: String = "source-machine",
        targetMachineID: String = "target-machine"
    ) -> String {
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
              "machine_id": "\(sourceMachineID)",
              "machine_label": "source"
            },
            "target": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "\(targetMachineID)",
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
            }
          }
        }
        """
    }
}

private final class LoadedSnapshotBox {
    var snapshot: AcceptanceBundleLoadedSnapshot

    init(snapshot: AcceptanceBundleLoadedSnapshot) {
        self.snapshot = snapshot
    }
}

private struct LaunchBundleFixture {
    let workDirURL: URL
    let bundleRootURL: URL
    let outsideOutputURL: URL

    func cleanup() {
        try? FileManager.default.removeItem(at: workDirURL)
    }
}
