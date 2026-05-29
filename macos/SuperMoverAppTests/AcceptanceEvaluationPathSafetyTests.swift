import Foundation
import XCTest
@testable import SuperMoverApp

final class AcceptanceEvaluationPathSafetyTests: XCTestCase {
    func testEvaluationCoordinatorFailsClosedWhenCurrentSourceBaselinePathEscapesBundle() throws {
        let fixture = try makeFixture(
            named: "evaluation-baseline-escape",
            sourceConsistencyMeta: [
                "output": "source.consistency.json",
                "baseline": "../outside-baseline.json",
                "status": "pass",
                "mode": "current_source_verified",
                "session_id": "session-1",
            ]
        )
        defer { fixture.cleanup() }

        try makeBaselineJSON().write(
            to: fixture.bundleRootURL.deletingLastPathComponent().appendingPathComponent("outside-baseline.json"),
            atomically: true,
            encoding: .utf8
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: false
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("../outside-baseline.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenSourceProvenanceArtifactIsSymlinkedOutsideBundle() throws {
        let fixture = try makeFixture(named: "evaluation-provenance-symlink")
        defer { fixture.cleanup() }

        let outside = fixture.bundleRootURL.deletingLastPathComponent().appendingPathComponent("outside-source.provenance.json")
        try fixture.fileManager.removeItem(at: fixture.bundleRootURL.appendingPathComponent("source.provenance.json"))
        try AcceptanceReleaseEvidenceFixtures.jsonString(
            AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest()
        ).write(to: outside, atomically: true, encoding: .utf8)
        do {
            try fixture.fileManager.createSymbolicLink(
                at: fixture.bundleRootURL.appendingPathComponent("source.provenance.json"),
                withDestinationURL: outside
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("source.provenance.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenSourceConsistencyArtifactIsMalformedEvenIfMetaLooksReady() throws {
        let fixture = try makeFixture(
            named: "evaluation-source-consistency-malformed",
            sourceConsistencyArtifact: #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass""#
        )
        defer { fixture.cleanup() }

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("source.consistency.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenSourcePairReceiptPathEscapesBundle() throws {
        let fixture = try makeFixture(named: "evaluation-source-pair-receipt-escape")
        defer { fixture.cleanup() }

        try #"{"profile":"/tmp/source.profile.json","target_address":"127.0.0.1:39395","verification_code":"123456","pairing_receipt_id":"pair-1","receipt_path":"../outside-receipt.json"}"#
            .write(
                to: fixture.bundleRootURL.appendingPathComponent("source.pair.json"),
                atomically: true,
                encoding: .utf8
            )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("../outside-receipt.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenSourcePairReceiptArtifactIsMissing() throws {
        let fixture = try makeFixture(named: "evaluation-source-pair-receipt-missing")
        defer { fixture.cleanup() }

        try fixture.fileManager.removeItem(
            at: fixture.bundleRootURL.appendingPathComponent("exported-receipts/pair-1.json")
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingRequiredArtifact("exported-receipts/pair-1.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenSourcePairReceiptArtifactIsDirectory() throws {
        let fixture = try makeFixture(named: "evaluation-source-pair-receipt-directory")
        defer { fixture.cleanup() }

        try fixture.fileManager.removeItem(
            at: fixture.bundleRootURL.appendingPathComponent("exported-receipts/pair-1.json")
        )
        try fixture.fileManager.createDirectory(
            at: fixture.bundleRootURL.appendingPathComponent("exported-receipts/pair-1.json"),
            withIntermediateDirectories: true
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("exported-receipts/pair-1.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenSourcePairReceiptIsMalformed() throws {
        let fixture = try makeFixture(named: "evaluation-source-pair-receipt-malformed")
        defer { fixture.cleanup() }

        try "{}".write(
            to: fixture.bundleRootURL.appendingPathComponent("exported-receipts/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("exported-receipts/pair-1.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenSourcePairReceiptIsHardlink() throws {
        let fixture = try makeFixture(named: "evaluation-source-pair-receipt-hardlink")
        defer { fixture.cleanup() }

        let receiptURL = fixture.bundleRootURL.appendingPathComponent("exported-receipts/pair-1.json")
        let outside = fixture.workDirURL.appendingPathComponent("outside-source-pair-receipt.json")
        try targetPairingReceiptJSON().write(to: outside, atomically: true, encoding: .utf8)
        try fixture.fileManager.removeItem(at: receiptURL)
        do {
            try fixture.fileManager.linkItem(at: outside, to: receiptURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("exported-receipts/pair-1.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenPairingReceiptIDIsUnsafeControlPlanePathSegment() throws {
        let fixture = try makeFixture(named: "evaluation-unsafe-pairing-receipt-id")
        defer { fixture.cleanup() }

        try #"{"profile":"/tmp/source.profile.json","target_address":"127.0.0.1:39395","verification_code":"123456","pairing_receipt_id":"../pair-escape","receipt_path":"exported-receipts/pair-1.json"}"#
            .write(
                to: fixture.bundleRootURL.appendingPathComponent("source.pair.json"),
                atomically: true,
                encoding: .utf8
            )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidPairingReceiptID
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenSessionIDIsUnsafeControlPlanePathSegment() throws {
        let fixture = try makeFixture(named: "evaluation-unsafe-session-id")
        defer { fixture.cleanup() }

        try #"{"profile":"/tmp/source.profile.json","session_id":"../session-escape","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#
            .write(
                to: fixture.bundleRootURL.appendingPathComponent("source.transfer.json"),
                atomically: true,
                encoding: .utf8
            )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidSessionID
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedForUnsafeControlPlaneIDMatrix() throws {
        let unsafePairingReceiptIDs = [".", "..", "~pair", #"pair\id"#, "pair 1", "pair-1\n", "pair\u{0000}1", "pair\u{0007}1"]
        for (index, receiptID) in unsafePairingReceiptIDs.enumerated() {
            let fixture = try makeFixture(named: "evaluation-unsafe-pairing-id-matrix-\(index)")
            defer { fixture.cleanup() }
            try writeSourcePairJSON(pairingReceiptID: receiptID, to: fixture.bundleRootURL)

            XCTAssertThrowsError(
                try AcceptanceBundleEvaluationCoordinator().evaluate(
                    bundleRootURL: fixture.bundleRootURL,
                    targetRootURL: fixture.targetRootURL,
                    requireOperatorEvidence: true
                ),
                "pairing_receipt_id \(String(reflecting: receiptID)) should fail closed"
            ) { error in
                XCTAssertEqual(
                    error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                    .invalidPairingReceiptID
                )
            }
        }

        let unsafeSessionIDs = [".", "..", "~session", #"session\id"#, "session 1", "session-1\n", "session\u{0000}1", "session\u{0007}1"]
        for (index, sessionID) in unsafeSessionIDs.enumerated() {
            let fixture = try makeFixture(named: "evaluation-unsafe-session-id-matrix-\(index)")
            defer { fixture.cleanup() }
            try writeSourceTransferJSON(sessionID: sessionID, to: fixture.bundleRootURL)

            XCTAssertThrowsError(
                try AcceptanceBundleEvaluationCoordinator().evaluate(
                    bundleRootURL: fixture.bundleRootURL,
                    targetRootURL: fixture.targetRootURL,
                    requireOperatorEvidence: true
                ),
                "session_id \(String(reflecting: sessionID)) should fail closed"
            ) { error in
                XCTAssertEqual(
                    error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                    .invalidSessionID
                )
            }
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenTargetPairingReceiptIsDirectory() throws {
        let fixture = try makeFixture(named: "evaluation-target-pairing-receipt-directory")
        defer { fixture.cleanup() }

        let receiptURL = fixture.targetRootURL.appendingPathComponent(".supermover/pairings/pair-1.json")
        try fixture.fileManager.removeItem(at: receiptURL)
        try fixture.fileManager.createDirectory(at: receiptURL, withIntermediateDirectories: true)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidTargetPairingReceipt(".supermover/pairings/pair-1.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenTargetPairingReceiptIsEmptyObject() throws {
        let fixture = try makeFixture(named: "evaluation-target-pairing-receipt-empty-object")
        defer { fixture.cleanup() }

        try "{}".write(
            to: fixture.targetPairingReceiptURL,
            atomically: true,
            encoding: .utf8
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidTargetPairingReceipt(".supermover/pairings/pair-1.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenTargetPairingReceiptRequiredFieldsAreInvalid() throws {
        let invalidReceipts: [(name: String, payload: String)] = [
            ("missing-profile", try targetPairingReceiptJSON(profileID: nil)),
            ("blank-source-device", try targetPairingReceiptJSON(sourceDeviceID: "   ")),
            ("wrong-id", try targetPairingReceiptJSON(id: "pair-other")),
            ("missing-verification", try targetPairingReceiptJSON(verificationHash: nil, verificationPhrase: nil)),
            ("bad-verified-at", try targetPairingReceiptJSON(verifiedAt: "not-a-date")),
            ("device-public-key-mismatch", try targetPairingReceiptJSON(devicePublicKey: "other-device")),
        ]

        for invalidReceipt in invalidReceipts {
            let fixture = try makeFixture(named: "evaluation-target-pairing-receipt-\(invalidReceipt.name)")
            defer { fixture.cleanup() }

            try invalidReceipt.payload.write(
                to: fixture.targetPairingReceiptURL,
                atomically: true,
                encoding: .utf8
            )

            XCTAssertThrowsError(
                try AcceptanceBundleEvaluationCoordinator().evaluate(
                    bundleRootURL: fixture.bundleRootURL,
                    targetRootURL: fixture.targetRootURL,
                    requireOperatorEvidence: true
                ),
                "target pairing receipt \(invalidReceipt.name) should fail closed"
            ) { error in
                XCTAssertEqual(
                    error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                    .invalidTargetPairingReceipt(".supermover/pairings/pair-1.json")
                )
            }
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenTargetPairingReceiptIsHardlink() throws {
        let fixture = try makeFixture(named: "evaluation-target-pairing-receipt-hardlink")
        defer { fixture.cleanup() }

        let outside = fixture.workDirURL.appendingPathComponent("outside-pairing-receipt.json")
        try targetPairingReceiptJSON().write(to: outside, atomically: true, encoding: .utf8)
        try fixture.fileManager.removeItem(at: fixture.targetPairingReceiptURL)
        do {
            try fixture.fileManager.linkItem(at: outside, to: fixture.targetPairingReceiptURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidTargetPairingReceipt(".supermover/pairings/pair-1.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenTargetRootIsSymlink() throws {
        let fixture = try makeFixture(named: "evaluation-target-root-symlink")
        defer { fixture.cleanup() }

        let symlinkURL = fixture.workDirURL.appendingPathComponent("target-root-link", isDirectory: true)
        do {
            try fixture.fileManager.createSymbolicLink(at: symlinkURL, withDestinationURL: fixture.targetRootURL)
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: symlinkURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .unreadableTargetRoot(symlinkURL.path)
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenTargetNetworkTransferIsSymlink() throws {
        let fixture = try makeFixture(named: "evaluation-target-network-transfer-symlink")
        defer { fixture.cleanup() }

        let transferURL = fixture.targetRootURL
            .appendingPathComponent(".supermover/sessions/session-1/network-transfer.json")
        let outside = fixture.workDirURL.appendingPathComponent("outside-network-transfer.json")
        try fixture.fileManager.removeItem(at: transferURL)
        try #"{"status":"published","stage":"commit","encrypted_transfer":"tls13_mtls","source_device_id":"src-spki","target_device_id":"dst-spki"}"#
            .write(to: outside, atomically: true, encoding: .utf8)
        do {
            try fixture.fileManager.createSymbolicLink(at: transferURL, withDestinationURL: outside)
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidTargetNetworkTransfer(".supermover/sessions/session-1/network-transfer.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenTargetNetworkTransferContentIsMalformed() throws {
        let invalidTransfers: [(name: String, payload: String)] = [
            ("missing-session", try targetNetworkTransferJSON(sessionID: nil)),
            ("wrong-session", try targetNetworkTransferJSON(sessionID: "session-other")),
            ("missing-profile", try targetNetworkTransferJSON(profileID: nil)),
            ("same-devices", try targetNetworkTransferJSON(sourceDeviceID: "same", targetDeviceID: "same")),
            ("missing-protocol", try targetNetworkTransferJSON(protocolVersion: nil)),
            ("bad-started-at", try targetNetworkTransferJSON(startedAt: "not-a-date")),
            ("updated-before-started", try targetNetworkTransferJSON(startedAt: "2026-01-01T00:00:02Z", updatedAt: "2026-01-01T00:00:01Z")),
        ]

        for invalidTransfer in invalidTransfers {
            let fixture = try makeFixture(named: "evaluation-target-network-transfer-\(invalidTransfer.name)")
            defer { fixture.cleanup() }
            try invalidTransfer.payload.write(
                to: fixture.targetNetworkTransferURL,
                atomically: true,
                encoding: .utf8
            )

            XCTAssertThrowsError(
                try AcceptanceBundleEvaluationCoordinator().evaluate(
                    bundleRootURL: fixture.bundleRootURL,
                    targetRootURL: fixture.targetRootURL,
                    requireOperatorEvidence: true
                ),
                "target network-transfer \(invalidTransfer.name) should fail closed"
            ) { error in
                XCTAssertEqual(
                    error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                    .invalidTargetNetworkTransfer(".supermover/sessions/session-1/network-transfer.json")
                )
            }
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenTargetNetworkTransferIsHardlink() throws {
        let fixture = try makeFixture(named: "evaluation-target-network-transfer-hardlink")
        defer { fixture.cleanup() }

        let outside = fixture.workDirURL.appendingPathComponent("outside-network-transfer.json")
        try targetNetworkTransferJSON().write(to: outside, atomically: true, encoding: .utf8)
        try fixture.fileManager.removeItem(at: fixture.targetNetworkTransferURL)
        do {
            try fixture.fileManager.linkItem(at: outside, to: fixture.targetNetworkTransferURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: fixture.bundleRootURL,
                targetRootURL: fixture.targetRootURL,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidTargetNetworkTransfer(".supermover/sessions/session-1/network-transfer.json")
            )
        }
    }

    private func makeFixture(
        named name: String,
        sourceConsistencyMeta: [String: Any]? = nil,
        sourceConsistencyArtifact: String? = nil
    ) throws -> EvaluationFixture {
        let fileManager = FileManager.default
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: name)
        let bundleRootURL = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRootURL = workDir.appendingPathComponent("target-root", isDirectory: true)
        try fileManager.createDirectory(at: bundleRootURL, withIntermediateDirectories: true)
        try fileManager.createDirectory(at: targetRootURL, withIntermediateDirectories: true)

        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRootURL,
            machine: "source",
            appPath: "/Applications/Source/SuperMover.app"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundleRootURL,
            machine: "target",
            appPath: "/Applications/Target/SuperMover.app"
        )

        try #"{"profile":"/tmp/source.profile.json","target_address":"127.0.0.1:39395","verification_code":"123456","pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json"}"#.write(
            to: bundleRootURL.appendingPathComponent("source.pair.json"),
            atomically: true,
            encoding: .utf8
        )
        try fileManager.createDirectory(
            at: bundleRootURL.appendingPathComponent("exported-receipts"),
            withIntermediateDirectories: true
        )
        try targetPairingReceiptJSON().write(
            to: bundleRootURL.appendingPathComponent("exported-receipts/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )
        try "pair ok".write(
            to: bundleRootURL.appendingPathComponent("source.pair.txt"),
            atomically: true,
            encoding: .utf8
        )
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(
            to: bundleRootURL.appendingPathComponent("source.transfer.json"),
            atomically: true,
            encoding: .utf8
        )
        try "receipt adopted".write(
            to: bundleRootURL.appendingPathComponent("target.adopt-pairing.txt"),
            atomically: true,
            encoding: .utf8
        )
        try "push ok".write(
            to: bundleRootURL.appendingPathComponent("source.network-push.txt"),
            atomically: true,
            encoding: .utf8
        )
        try makeBaselineJSON().write(
            to: bundleRootURL.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try (sourceConsistencyArtifact ?? makeCurrentSourceConsistencyJSON()).write(
            to: bundleRootURL.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRootURL.path).write(
            to: bundleRootURL.appendingPathComponent("source.verify.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeReportJSON(targetRoot: targetRootURL.path).write(
            to: bundleRootURL.appendingPathComponent("source.report.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeStatusJSON(targetRoot: targetRootURL.path).write(
            to: bundleRootURL.appendingPathComponent("source.status.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeHealthJSON(targetRoot: targetRootURL.path).write(
            to: bundleRootURL.appendingPathComponent("source.health.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "schema": "supermover.acceptance.machine_facts.v1",
            "machine_id": "source-machine",
            "machine_label": "source",
        ]).write(
            to: bundleRootURL.appendingPathComponent("source.machine.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "schema": "supermover.acceptance.machine_facts.v1",
            "machine_id": "target-machine",
            "machine_label": "target",
        ]).write(
            to: bundleRootURL.appendingPathComponent("target.machine.json"),
            atomically: true,
            encoding: .utf8
        )

        let consistencyMeta = sourceConsistencyMeta ?? [
            "output": "source.consistency.json",
            "baseline": "source.baseline.json",
            "status": "pass",
            "mode": "current_source_verified",
            "session_id": "session-1",
        ]
        try AcceptanceReleaseEvidenceFixtures.jsonString(
            makeMetaDocument(sourceConsistencyMeta: consistencyMeta)
        ).write(
            to: bundleRootURL.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )

        let controlPlane = targetRootURL.appendingPathComponent(".supermover", isDirectory: true)
        try fileManager.createDirectory(
            at: controlPlane.appendingPathComponent("pairings"),
            withIntermediateDirectories: true
        )
        try fileManager.createDirectory(
            at: controlPlane.appendingPathComponent("sessions/session-1"),
            withIntermediateDirectories: true
        )
        try targetPairingReceiptJSON().write(
            to: controlPlane.appendingPathComponent("pairings/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )
        try targetNetworkTransferJSON().write(
            to: controlPlane.appendingPathComponent("sessions/session-1/network-transfer.json"),
            atomically: true,
            encoding: .utf8
        )

        return EvaluationFixture(
            fileManager: fileManager,
            workDirURL: workDir,
            bundleRootURL: bundleRootURL,
            targetRootURL: targetRootURL
        )
    }

    private func writeSourcePairJSON(pairingReceiptID: String, to bundleRootURL: URL) throws {
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "profile": "/tmp/source.profile.json",
            "target_address": "127.0.0.1:39395",
            "verification_code": "123456",
            "pairing_receipt_id": pairingReceiptID,
            "receipt_path": "exported-receipts/pair-1.json",
        ]).write(
            to: bundleRootURL.appendingPathComponent("source.pair.json"),
            atomically: true,
            encoding: .utf8
        )
    }

    private func writeSourceTransferJSON(sessionID: String, to bundleRootURL: URL) throws {
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "profile": "/tmp/source.profile.json",
            "session_id": sessionID,
            "target_address": "127.0.0.1:39395",
            "receiver_address": "127.0.0.1:9443",
            "target_mode": "pairing",
        ]).write(
            to: bundleRootURL.appendingPathComponent("source.transfer.json"),
            atomically: true,
            encoding: .utf8
        )
    }

    private static func targetPairingReceiptJSON(
        version: Int? = 1,
        id: String? = "pair-1",
        profileID: String? = "profile-1",
        targetID: String? = "target-1",
        sourceDeviceID: String? = "src-spki",
        targetDeviceID: String? = "dst-spki",
        devicePublicKey: String? = "dst-spki",
        method: String? = "verification_code",
        verifiedAt: String? = "2026-01-01T00:00:00Z",
        verificationHash: String? = "hash-1",
        verificationPhrase: String? = nil,
        protocolVersion: String? = "supermover/1"
    ) throws -> String {
        var receipt: [String: Any] = [:]
        if let version {
            receipt["version"] = version
        }
        if let id {
            receipt["id"] = id
        }
        if let profileID {
            receipt["profile_id"] = profileID
        }
        if let targetID {
            receipt["target_id"] = targetID
        }
        if let sourceDeviceID {
            receipt["source_device_id"] = sourceDeviceID
        }
        if let targetDeviceID {
            receipt["target_device_id"] = targetDeviceID
        }
        if let devicePublicKey {
            receipt["device_public_key"] = devicePublicKey
        }
        if let method {
            receipt["method"] = method
        }
        if let verifiedAt {
            receipt["verified_at"] = verifiedAt
        }
        if let verificationHash {
            receipt["verification_hash"] = verificationHash
        }
        if let verificationPhrase {
            receipt["verification_phrase"] = verificationPhrase
        }
        if let protocolVersion {
            receipt["protocol_version"] = protocolVersion
        }
        return try AcceptanceReleaseEvidenceFixtures.jsonString(receipt)
    }

    private static func targetNetworkTransferJSON(
        version: Int? = 1,
        sessionID: String? = "session-1",
        profileID: String? = "profile-1",
        targetID: String? = "target-1",
        sourceDeviceID: String? = "src-spki",
        targetDeviceID: String? = "dst-spki",
        protocolVersion: String? = "supermover/1",
        status: String? = "published",
        stage: String? = "commit",
        encryptedTransfer: String? = "tls13_mtls",
        startedAt: String? = "2026-01-01T00:00:00Z",
        updatedAt: String? = "2026-01-01T00:00:01Z"
    ) throws -> String {
        var transfer: [String: Any] = [:]
        if let version {
            transfer["version"] = version
        }
        if let sessionID {
            transfer["session_id"] = sessionID
        }
        if let profileID {
            transfer["profile_id"] = profileID
        }
        if let targetID {
            transfer["target_id"] = targetID
        }
        if let sourceDeviceID {
            transfer["source_device_id"] = sourceDeviceID
        }
        if let targetDeviceID {
            transfer["target_device_id"] = targetDeviceID
        }
        if let protocolVersion {
            transfer["protocol_version"] = protocolVersion
        }
        if let status {
            transfer["status"] = status
        }
        if let stage {
            transfer["stage"] = stage
        }
        if let encryptedTransfer {
            transfer["encrypted_transfer"] = encryptedTransfer
        }
        if let startedAt {
            transfer["started_at"] = startedAt
        }
        if let updatedAt {
            transfer["updated_at"] = updatedAt
        }
        return try AcceptanceReleaseEvidenceFixtures.jsonString(transfer)
    }

    private func targetNetworkTransferJSON(
        version: Int? = 1,
        sessionID: String? = "session-1",
        profileID: String? = "profile-1",
        targetID: String? = "target-1",
        sourceDeviceID: String? = "src-spki",
        targetDeviceID: String? = "dst-spki",
        protocolVersion: String? = "supermover/1",
        status: String? = "published",
        stage: String? = "commit",
        encryptedTransfer: String? = "tls13_mtls",
        startedAt: String? = "2026-01-01T00:00:00Z",
        updatedAt: String? = "2026-01-01T00:00:01Z"
    ) throws -> String {
        try Self.targetNetworkTransferJSON(
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

    private func targetPairingReceiptJSON(
        version: Int? = 1,
        id: String? = "pair-1",
        profileID: String? = "profile-1",
        targetID: String? = "target-1",
        sourceDeviceID: String? = "src-spki",
        targetDeviceID: String? = "dst-spki",
        devicePublicKey: String? = "dst-spki",
        method: String? = "verification_code",
        verifiedAt: String? = "2026-01-01T00:00:00Z",
        verificationHash: String? = "hash-1",
        verificationPhrase: String? = nil,
        protocolVersion: String? = "supermover/1"
    ) throws -> String {
        try Self.targetPairingReceiptJSON(
            version: version,
            id: id,
            profileID: profileID,
            targetID: targetID,
            sourceDeviceID: sourceDeviceID,
            targetDeviceID: targetDeviceID,
            devicePublicKey: devicePublicKey,
            method: method,
            verifiedAt: verifiedAt,
            verificationHash: verificationHash,
            verificationPhrase: verificationPhrase,
            protocolVersion: protocolVersion
        )
    }

    private func makeMetaDocument(sourceConsistencyMeta: [String: Any]) -> [String: Any] {
        [
            "schema": "supermover.acceptance.two_machine.v1",
            "status": "in_progress",
            "collection": [
                "mode": "two_machine",
                "machine_count": 2,
            ],
            "roles": [
                "source_pair": [
                    "profile": "/tmp/source.profile.json",
                    "status": "recorded",
                    "machine_id": "source-machine",
                    "machine_label": "source",
                ],
                "target": [
                    "profile": "/tmp/target.profile.json",
                    "status": "recorded",
                    "machine_id": "target-machine",
                    "machine_label": "target",
                ],
            ],
            "evidence": [
                "machine_facts": [
                    "source": [
                        "output": "source.machine.json",
                        "machine_id": "source-machine",
                        "machine_label": "source",
                    ],
                    "target": [
                        "output": "target.machine.json",
                        "machine_id": "target-machine",
                        "machine_label": "target",
                    ],
                ],
                "bundle_handoffs": [[
                    "archive": "source-to-target.tgz",
                    "manifest": "source-to-target.manifest.json",
                    "sha256": String(repeating: "1", count: 64),
                    "meta": "meta.json",
                    "verified": true,
                    "exporting_machine_id": "source-machine",
                    "exporting_machine_label": "source",
                    "importing_machine_id": "target-machine",
                    "importing_machine_label": "target",
                ]],
                "source_pair": [
                    "pairing_receipt_id": "pair-1",
                    "receipt_path": "exported-receipts/pair-1.json",
                    "target_address": "127.0.0.1:39395",
                    "output": "source.pair.json",
                    "pair": "source.pair.txt",
                ],
                "source_transfer": [
                    "session_id": "session-1",
                    "receiver_address": "127.0.0.1:9443",
                    "output": "source.transfer.json",
                    "verify": "source.verify.json",
                    "status": "source.status.json",
                    "report": "source.report.json",
                    "health": "source.health.json",
                    "push": "source.network-push.txt",
                ],
                "source_consistency": sourceConsistencyMeta,
                "target_import": [
                    "pairing_receipt_id": "pair-1",
                    "adopted": "target.adopt-pairing.txt",
                ],
                "operator": [
                    "local_network": [
                        "status": "pass",
                        "detail": "accepted prompt",
                    ],
                    "firewall": [
                        "status": "pass",
                        "detail": "allowed firewall access",
                    ],
                    "pairing_confirmation": [
                        "status": "pass",
                        "detail": "confirmed code",
                    ],
                ],
            ],
        ]
    }

    private func makeCurrentSourceConsistencyJSON() -> String {
        """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "status": "pass",
          "mode": "current_source_verified",
          "session_id": "session-1",
          "detail": "test fixture"
        }
        """
    }

    private func makeBaselineJSON() -> String {
        """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-1",
          "root_id": "root-1",
          "root_path": "/tmp/source",
          "session_id": "session-1",
          "created_at": "2026-01-01T00:00:00Z",
          "entries": []
        }
        """
    }

    private func makeVerifyJSON(targetRoot: String) -> String {
        """
        {
          "target_root": "\(targetRoot)",
          "session_id": "session-1",
          "manifest": {"id":"m1","session_id":"session-1","root_id":"root-1","created_at":"2026-01-01T00:00:00Z","entries":1,"files":1},
          "summary": {
            "manifest_count": 1,
            "manifest_entries": 1,
            "files_expected": 1,
            "files_verified": 1,
            "warnings": 0,
            "soft_deletes": 0,
            "target_drifts": 0,
            "artifact_problems": 0,
            "error_findings": 0,
            "warning_findings": 0,
            "skipped_digest": 0
          }
        }
        """
    }

    private func makeReportJSON(targetRoot: String) -> String {
        """
        {
          "target_root": "\(targetRoot)",
          "overall": {"status":"ok","issues":[]},
          "summary": {"warnings":0,"soft_deletes":0,"target_drifts":0,"live_target_drifts":0,"prune_candidates":0,"prune_refusals":0,"prune_approvals":0,"network_transfers":1,"artifact_problems":0},
          "latest_session": {"id":"session-1","manifest_id":"m1","created_at":"2026-01-01T00:00:00Z","entries":1,"files":1,"completeness":{"status":"verified","files_expected":1,"files_verified":1,"verification_errors":0,"verification_warnings":0}},
          "prune_review": {"status":"clear","approval_required":false,"apply":"none","summary":{"candidates":0,"refusals":0,"approvals":0,"unapplied_approvals":0,"receipt_issues":0}},
          "pairing": {"status":"paired_receipt_valid","receipt_id":"pair-1","target_device_id":"dst-spki","paired_at":"2026-01-01T00:00:00Z","method":"verification_code","verified_at":"2026-01-01T00:00:00Z","evidence":"receipt","receipt_source":"target_control","receipt_path":"p","source_receipt_path":"s","target_receipt_path":"t","encrypted_transfer":"required"},
          "privacy": {"status":"review","claim":"bounded","network_transfer":"published"},
          "health": {"healthy":true,"summary":{"incomplete_sessions":0,"invalid_records":0,"artifact_problems":0,"target_drifts":0,"network_transfers":1}}
        }
        """
    }

    private func makeStatusJSON(targetRoot: String) -> String {
        """
        {
          "profile_id": "profile-1",
          "target_id": "target-1",
          "target_root": "\(targetRoot)",
          "overall": {"status":"ok","target_status":"ok"},
          "issues": [],
          "latest_session": {"id":"session-1","manifest_id":"m1","created_at":"2026-01-01T00:00:00Z","entries":1,"completeness_status":"verified","files_expected":1,"files_verified":1,"verification_errors":0,"verification_warnings":0},
          "counts": {"warnings":0,"soft_deletes":0,"target_drifts":0,"live_target_drifts":0,"live_target_drift_artifact_problems":0,"prune_unapplied_approvals":0,"prune_active_approvals":0,"prune_stale_approvals":0,"prune_expired_approvals":0,"prune_consumed_approvals":0,"prune_receipt_issues":0,"recovery_issues":0,"artifact_problems":0,"network_transfers":1},
          "prune_review": {"status":"clear","action":"none"},
          "pairing": {"status":"paired_receipt_valid","receipt_id":"pair-1","target_device_id":"dst-spki","paired_at":"2026-01-01T00:00:00Z","method":"verification_code","verified_at":"2026-01-01T00:00:00Z","evidence":"receipt","receipt_source":"target_control","receipt_path":"p","source_receipt_path":"s","target_receipt_path":"t","encrypted_transfer":"required"},
          "privacy": {"status":"review","mode":"bounded","traffic_level":2,"claim":"bounded","local_push":"disabled","network_transfer":"published","residual_leakage":[],"configured_reductions":[],"overhead_status":"published","overhead_source":"network-transfer"},
          "traffic_privacy_acceptance": {"status":"review","claim":"bounded","blockers":[]},
          "network": {"status":"published","artifact_problems":0,"transfers":[{"session_id":"session-1","status":"published","stage":"commit","action":"preserved"}]},
          "artifact_problem_sources": []
        }
        """
    }

    private func makeHealthJSON(targetRoot: String) -> String {
        """
        {
          "target_root": "\(targetRoot)",
          "healthy": true,
          "summary": {"incomplete_sessions":0,"invalid_records":0,"artifact_problems":0,"target_drifts":0,"network_transfers":1},
          "items": [],
          "invalid": [],
          "artifacts": [],
          "network_transfers": [{"session_id":"session-1","status":"published","stage":"commit","action":"preserved"}]
        }
        """
    }
}

private struct EvaluationFixture {
    let fileManager: FileManager
    let workDirURL: URL
    let bundleRootURL: URL
    let targetRootURL: URL

    var targetPairingReceiptURL: URL {
        targetRootURL.appendingPathComponent(".supermover/pairings/pair-1.json")
    }

    var targetNetworkTransferURL: URL {
        targetRootURL.appendingPathComponent(".supermover/sessions/session-1/network-transfer.json")
    }

    func cleanup() {
        try? fileManager.removeItem(at: workDirURL)
    }
}
