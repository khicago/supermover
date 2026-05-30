import XCTest
@testable import SuperMoverApp

final class AcceptanceBundleArtifactWriterTests: XCTestCase {
    @MainActor
    func testAppStoreManualArtifactRecordingWhenBundleConfigured() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)
        let receiptURL = try writePairingReceipt(id: "pair-1", under: dir)

        let profileURL = dir.appendingPathComponent("source.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1",
            "local_pairing_receipt_path": "\(receiptURL.path)"
          },
          "network": {
            "receiver_url": "https://127.0.0.1:9443"
          }
        }
        """.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"
        store.sessionID = "session-1"
        store.discoveryBrowseSnapshot = try XCTUnwrap(
            try? JSONDecoder().decode(
                DiscoveryBrowseSnapshot.self,
                from: Data("""
                {
                  "source": "lan_datagram",
                  "listen": "0.0.0.0:39394",
                  "candidate_count": 1,
                  "invalid_packets": 0,
                  "trusted": false,
                  "candidates": []
                }
                """.utf8)
            )
        )
        store.discoveryAdvertiseSnapshot = try XCTUnwrap(
            try? JSONDecoder().decode(
                DiscoveryAdvertiseSnapshot.self,
                from: Data("""
                {
                  "status": "advertised",
                  "listen": "0.0.0.0:39394",
                  "destination": "255.255.255.255:39394",
                  "service_type": "_supermover._udp",
                  "protocol_version": "v1",
                  "ephemeral_nonce": "nonce-1",
                  "capability_flags": ["pairing"],
                  "trusted": false,
                  "duration": "10s",
                  "interval": "1s"
                }
                """.utf8)
            )
        )
        store.acceptanceServePhase = "2"
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: "127.0.0.1:39395",
            verification_code: "123456",
            mode: "pairing",
            receiver_address: "127.0.0.1:9443",
            receiver_routes: true,
            push_network: true,
            trusted: true,
            transfer: true,
            expires_at: nil
        )
        store.recordAcceptanceDiscoveryBrowseArtifact()
        store.recordAcceptanceDiscoveryAdvertiseArtifact()
        store.recordAcceptanceServePhaseArtifact()
        store.recordAcceptanceSourcePairArtifact()
        store.recordAcceptanceSourceTransferArtifact()

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.sourceBrowseSnapshot?.candidate_count, 1)
        XCTAssertEqual(snapshot.targetAdvertiseSnapshot?.status, "advertised")
        XCTAssertEqual(snapshot.targetServePhaseArtifacts.first?.phase, 2)
        XCTAssertEqual(snapshot.sourcePairArtifact?.pairing_receipt_id, "pair-1")
        XCTAssertEqual(snapshot.sourcePairArtifact?.receipt_path, "exported-receipts/pair-1.json")
        XCTAssertEqual(snapshot.sourceTransferArtifact?.session_id, "session-1")
        XCTAssertEqual(snapshot.meta.evidence.target_ready?.mode, "pairing")
        XCTAssertTrue(FileManager.default.fileExists(atPath: dir.appendingPathComponent("target.ready.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: dir.appendingPathComponent("exported-receipts/pair-1.json").path))
    }

    @MainActor
    func testAppStoreManualServePhaseRecordingRequiresVerificationCodeForPairingMode() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)

        let profileURL = dir.appendingPathComponent("target.profile.json")
        try "{}\n".write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.acceptanceServePhase = "1"
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: "127.0.0.1:39395",
            verification_code: nil,
            mode: "pairing",
            receiver_address: "127.0.0.1:9443",
            receiver_routes: true,
            push_network: true,
            trusted: true,
            transfer: true,
            expires_at: nil
        )

        store.recordAcceptanceServePhaseArtifact()

        XCTAssertEqual(
            store.note,
            AcceptanceBundleArtifactWriter.WriteError.missingServeVerificationCode.localizedDescription
        )
        XCTAssertFalse(FileManager.default.fileExists(atPath: dir.appendingPathComponent("target.ready.json").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: dir.appendingPathComponent("target.ready.phase-1.json").path))

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertNil(snapshot.meta.evidence.target_ready)
    }

    @MainActor
    func testAppStoreManualServePhaseRecordingOmitsRuntimePairingApprovalFields() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)

        let profileURL = dir.appendingPathComponent("target.profile.json")
        try "{}\n".write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.acceptanceServePhase = "1"
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: "127.0.0.1:39395",
            verification_code: "123456",
            mode: "pairing",
            receiver_address: nil,
            receiver_routes: nil,
            push_network: nil,
            trusted: false,
            transfer: false,
            expires_at: nil,
            operator_token: "local-operator-token",
            pairing_request: PairingRequestSnapshot(
                protocol_version: "v1",
                id: "pair-request-1",
                status: "pending",
                source_profile_id: "profile-local",
                source_profile_name: "Source profile",
                source_device_id: "sha256:abcdef0123456789",
                requested_at: "2026-06-01T10:00:00Z",
                expires_at: "2026-06-01T10:02:00Z",
                decided_at: nil
            )
        )

        store.recordAcceptanceServePhaseArtifact()

        for artifact in ["target.ready.json", "target.ready.phase-1.json"] {
            let data = try Data(contentsOf: dir.appendingPathComponent(artifact))
            let object = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
            XCTAssertNil(object["operator_token"], "\(artifact) must not persist local operator token")
            XCTAssertNil(object["pairing_request"], "\(artifact) must not persist runtime pairing request state")
            XCTAssertEqual(object["verification_code"] as? String, "123456")
        }
    }

    @MainActor
    func testAppStoreManualSourceTransferRecordingExportsStructuredArtifacts() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)
        let baselineURL = dir.appendingPathComponent("source.baseline.capture.json")
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-src",
          "root_id": "root-src",
          "root_path": "/tmp/source",
          "session_id": "session-1",
          "created_at": "2026-06-01T10:00:00Z",
          "entries": []
        }
        """.write(to: baselineURL, atomically: true, encoding: .utf8)

        let profileURL = dir.appendingPathComponent("source.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1"
          },
          "network": {
            "receiver_url": "https://127.0.0.1:9443"
          }
        }
        """.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.sessionID = "session-1"
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: "127.0.0.1:39395",
            verification_code: "123456",
            mode: "pairing",
            receiver_address: "127.0.0.1:9443",
            receiver_routes: true,
            push_network: true,
            trusted: true,
            transfer: true,
            expires_at: nil
        )
        store.evidenceEnvelopes[.verify] = StructuredEvidenceEnvelope(
            artifactKind: .verify,
            task: .verify,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .verify),
            exitCode: 0,
            rawStdout: #"{"summary":{"files_verified":1,"files_expected":1}}"#,
            stderrSample: "",
            freshness: .current
        )
        store.evidenceEnvelopes[.status] = StructuredEvidenceEnvelope(
            artifactKind: .status,
            task: .status,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .status),
            exitCode: 0,
            rawStdout: #"{"pairing":{"status":"paired_receipt_valid"}}"#,
            stderrSample: "",
            freshness: .current
        )
        store.evidenceEnvelopes[.report] = StructuredEvidenceEnvelope(
            artifactKind: .report,
            task: .report,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .report),
            exitCode: 0,
            rawStdout: #"{"pairing":{"status":"paired_receipt_valid"}}"#,
            stderrSample: "",
            freshness: .current
        )
        store.evidenceEnvelopes[.health] = StructuredEvidenceEnvelope(
            artifactKind: .health,
            task: .health,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .health),
            exitCode: 0,
            rawStdout: #"{"healthy":true}"#,
            stderrSample: "",
            freshness: .current
        )
        store.evidenceEnvelopes[.sourceConsistency] = StructuredEvidenceEnvelope(
            artifactKind: .sourceConsistency,
            task: .networkPush,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .networkPush),
            exitCode: 0,
            rawStdout: #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#,
            stderrSample: "",
            freshness: .current
        )
        store.installNetworkPushBaselineFileForTesting(
            baselineURL.path,
            contextSignature: store.currentContextSignature(for: .networkPush)
        )
        store.recentRuns.insert(
            TaskRun(
                kind: .networkPush,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: store.currentContextSignature(for: .networkPush),
                processIdentifier: nil,
                stdout: #"{"status":"published"}"#,
                stderr: "",
                state: .finished(0)
            ),
            at: 0
        )

        store.recordAcceptanceSourceTransferArtifact()

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.verify, "source.verify.json")
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.status, "source.status.json")
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.report, "source.report.json")
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.health, "source.health.json")
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.push, "source.network-push.txt")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.output, "source.consistency.json")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.baseline, "source.baseline.json")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.status, "pass")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.mode, "current_source_verified")
        XCTAssertEqual(try String(contentsOf: dir.appendingPathComponent("source.network-push.txt")), #"{"status":"published"}"#)
        XCTAssertTrue(FileManager.default.fileExists(atPath: dir.appendingPathComponent("source.consistency.json").path))
        XCTAssertEqual(
            try String(contentsOf: dir.appendingPathComponent("source.baseline.json")).trimmingCharacters(in: .whitespacesAndNewlines),
            try String(contentsOf: baselineURL).trimmingCharacters(in: .whitespacesAndNewlines)
        )
    }

    @MainActor
    func testAppStoreManualSourceTransferRecordingBlocksMismatchedSourceConsistencySession() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)

        let profileURL = dir.appendingPathComponent("source.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1"
          },
          "network": {
            "receiver_url": "https://127.0.0.1:9443"
          }
        }
        """.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.sessionID = "session-1"
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: "127.0.0.1:39395",
            verification_code: "123456",
            mode: "pairing",
            receiver_address: "127.0.0.1:9443",
            receiver_routes: true,
            push_network: true,
            trusted: true,
            transfer: true,
            expires_at: nil
        )
        store.evidenceEnvelopes[.sourceConsistency] = StructuredEvidenceEnvelope(
            artifactKind: .sourceConsistency,
            task: .networkPush,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .networkPush),
            exitCode: 0,
            rawStdout: #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-other"}"#,
            stderrSample: "",
            freshness: .current
        )

        store.recordAcceptanceSourceTransferArtifact()

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.status, "blocked")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.mode, "session_mismatch")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.status, "blocked")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.mode, "session_mismatch")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.session_id, "session-1")
    }

    @MainActor
    func testAppStoreManualSourceTransferRecordingBlocksPassSourceConsistencyWithoutBaselineArtifact() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-src",
          "root_id": "root-src",
          "root_path": "/tmp/source",
          "session_id": "session-previous",
          "created_at": "2026-06-01T10:00:00Z",
          "entries": []
        }
        """.write(to: dir.appendingPathComponent("source.baseline.json"), atomically: true, encoding: .utf8)

        let profileURL = dir.appendingPathComponent("source.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1"
          },
          "network": {
            "receiver_url": "https://127.0.0.1:9443"
          }
        }
        """.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.sessionID = "session-1"
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: "127.0.0.1:39395",
            verification_code: "123456",
            mode: "pairing",
            receiver_address: "127.0.0.1:9443",
            receiver_routes: true,
            push_network: true,
            trusted: true,
            transfer: true,
            expires_at: nil
        )
        store.evidenceEnvelopes[.sourceConsistency] = StructuredEvidenceEnvelope(
            artifactKind: .sourceConsistency,
            task: .networkPush,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .networkPush),
            exitCode: 0,
            rawStdout: #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#,
            stderrSample: "",
            freshness: .current
        )

        store.recordAcceptanceSourceTransferArtifact()

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.status, "blocked")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.mode, "baseline_missing")
        XCTAssertNil(snapshot.meta.evidence.source_consistency?.baseline)
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.status, "blocked")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.mode, "baseline_missing")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.session_id, "session-1")
        XCTAssertFalse(FileManager.default.fileExists(atPath: dir.appendingPathComponent("source.baseline.json").path))
    }

    @MainActor
    func testNetworkPushAutoRecordExportsStructuredTransferArtifacts() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)
        let baselineURL = dir.appendingPathComponent("source.baseline.capture.json")
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-src",
          "root_id": "root-src",
          "root_path": "/tmp/source",
          "session_id": "session-1",
          "created_at": "2026-06-01T10:00:00Z",
          "entries": []
        }
        """.write(to: baselineURL, atomically: true, encoding: .utf8)

        let profileURL = dir.appendingPathComponent("source.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1"
          },
          "network": {
            "receiver_url": "https://127.0.0.1:9443"
          }
        }
        """.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.sessionID = "session-1"
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: "127.0.0.1:39395",
            verification_code: "123456",
            mode: "pairing",
            receiver_address: "127.0.0.1:9443",
            receiver_routes: true,
            push_network: true,
            trusted: true,
            transfer: true,
            expires_at: nil
        )
        store.evidenceEnvelopes[.verify] = StructuredEvidenceEnvelope(
            artifactKind: .verify,
            task: .verify,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .verify),
            exitCode: 0,
            rawStdout: #"{"summary":{"files_verified":1,"files_expected":1}}"#,
            stderrSample: "",
            freshness: .current
        )
        store.evidenceEnvelopes[.status] = StructuredEvidenceEnvelope(
            artifactKind: .status,
            task: .status,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .status),
            exitCode: 0,
            rawStdout: #"{"pairing":{"status":"paired_receipt_valid"}}"#,
            stderrSample: "",
            freshness: .current
        )
        store.evidenceEnvelopes[.report] = StructuredEvidenceEnvelope(
            artifactKind: .report,
            task: .report,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .report),
            exitCode: 0,
            rawStdout: #"{"pairing":{"status":"paired_receipt_valid"}}"#,
            stderrSample: "",
            freshness: .current
        )
        store.evidenceEnvelopes[.health] = StructuredEvidenceEnvelope(
            artifactKind: .health,
            task: .health,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .health),
            exitCode: 0,
            rawStdout: #"{"healthy":true}"#,
            stderrSample: "",
            freshness: .current
        )
        store.evidenceEnvelopes[.sourceConsistency] = StructuredEvidenceEnvelope(
            artifactKind: .sourceConsistency,
            task: .networkPush,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .networkPush),
            exitCode: 0,
            rawStdout: #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#,
            stderrSample: "",
            freshness: .current
        )
        store.installNetworkPushBaselineFileForTesting(
            baselineURL.path,
            contextSignature: store.currentContextSignature(for: .networkPush)
        )

        store.triggerAcceptanceAutoRecordForTesting(.networkPush)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.verify, "source.verify.json")
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.status, "source.status.json")
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.report, "source.report.json")
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.health, "source.health.json")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.output, "source.consistency.json")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.baseline, "source.baseline.json")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.status, "pass")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.mode, "current_source_verified")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.status, "pass")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.mode, "current_source_verified")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.baseline, "source.baseline.json")
    }

    @MainActor
    func testNetworkPushAutoRecordBlocksPassSourceConsistencyWithoutBaselineArtifact() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-src",
          "root_id": "root-src",
          "root_path": "/tmp/source",
          "session_id": "session-previous",
          "created_at": "2026-06-01T10:00:00Z",
          "entries": []
        }
        """.write(to: dir.appendingPathComponent("source.baseline.json"), atomically: true, encoding: .utf8)

        let profileURL = dir.appendingPathComponent("source.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1"
          },
          "network": {
            "receiver_url": "https://127.0.0.1:9443"
          }
        }
        """.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.sessionID = "session-1"
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
            address: "127.0.0.1:39395",
            verification_code: "123456",
            mode: "pairing",
            receiver_address: "127.0.0.1:9443",
            receiver_routes: true,
            push_network: true,
            trusted: true,
            transfer: true,
            expires_at: nil
        )
        store.evidenceEnvelopes[.sourceConsistency] = StructuredEvidenceEnvelope(
            artifactKind: .sourceConsistency,
            task: .networkPush,
            loadedAt: Date(),
            contextSignature: store.currentContextSignature(for: .networkPush),
            exitCode: 0,
            rawStdout: #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#,
            stderrSample: "",
            freshness: .current
        )

        store.triggerAcceptanceAutoRecordForTesting(.networkPush)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.status, "blocked")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.mode, "baseline_missing")
        XCTAssertNil(snapshot.meta.evidence.source_consistency?.baseline)
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.status, "blocked")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.mode, "baseline_missing")
        XCTAssertFalse(FileManager.default.fileExists(atPath: dir.appendingPathComponent("source.baseline.json").path))
    }

    func testWriterRecordsServePairAndTransferArtifacts() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)
        let receiptURL = try writePairingReceipt(id: "pair-1", under: dir)

        let writer = AcceptanceBundleArtifactWriter()
        try writer.writeServePhase(
            .init(
                bundleRootURL: dir,
                profilePath: "/tmp/target.profile.json",
                phase: 2,
                readiness: ServeReadinessSnapshot(
                    address: "127.0.0.1:39395",
                    verification_code: "123456",
                    mode: "pairing",
                    receiver_address: "127.0.0.1:9443",
                    receiver_routes: true,
                    push_network: true,
                    trusted: true,
                    transfer: true,
                    expires_at: nil
                )
            )
        )
        try writer.writeSourcePair(
            .init(
                bundleRootURL: dir,
                profilePath: "/tmp/source.profile.json",
                targetAddress: "127.0.0.1:39395",
                verificationCode: "123456",
                localPairingReceiptPath: receiptURL.path,
                pairingReceiptID: "pair-1",
                pairStdout: "pair completed"
            )
        )
        try writer.writeSourceTransfer(
            .init(
                bundleRootURL: dir,
                profilePath: "/tmp/source.profile.json",
                sessionID: "session-1",
                targetAddress: "127.0.0.1:39395",
                receiverAddress: "127.0.0.1:9443",
                targetMode: "pairing",
                sourceBaselineURL: nil,
                sourceConsistencyRawJSON: nil,
                verifyArtifactPath: nil,
                statusArtifactPath: nil,
                reportArtifactPath: nil,
                healthArtifactPath: nil,
                pushStdout: #"{"status":"ok"}"#
            )
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.targetServePhaseArtifacts.first?.phase, 2)
        XCTAssertEqual(snapshot.targetServePhaseArtifacts.first?.readiness.receiver_address, "127.0.0.1:9443")
        XCTAssertEqual(snapshot.meta.evidence.target_ready?.address, "127.0.0.1:39395")
        XCTAssertEqual(snapshot.meta.evidence.target_ready?.verification_code, "123456")
        XCTAssertEqual(snapshot.sourcePairArtifact?.pairing_receipt_id, "pair-1")
        XCTAssertEqual(snapshot.sourcePairArtifact?.receipt_path, "exported-receipts/pair-1.json")
        XCTAssertEqual(snapshot.sourceTransferArtifact?.session_id, "session-1")
        XCTAssertEqual(snapshot.meta.evidence.source_pair?.pair, "source.pair.txt")
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.push, "source.network-push.txt")
        XCTAssertEqual(try String(contentsOf: dir.appendingPathComponent("source.pair.txt")), "pair completed")
        XCTAssertEqual(try String(contentsOf: dir.appendingPathComponent("source.network-push.txt")), #"{"status":"ok"}"#)
        XCTAssertTrue(FileManager.default.fileExists(atPath: dir.appendingPathComponent("target.ready.json").path))
        XCTAssertEqual(
            try String(contentsOf: dir.appendingPathComponent("exported-receipts/pair-1.json")).trimmingCharacters(in: .whitespacesAndNewlines),
            try String(contentsOf: receiptURL).trimmingCharacters(in: .whitespacesAndNewlines)
        )
    }

    func testWriterRejectsHardlinkedSourcePairOutputLeafBeforeWritingPhaseArtifacts() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)
        let receiptURL = try writePairingReceipt(id: "pair-1", under: dir)
        let outsideOutputURL = dir.deletingLastPathComponent().appendingPathComponent("outside-source-pair.json")
        try "outside\n".write(to: outsideOutputURL, atomically: true, encoding: .utf8)
        let hardlinkedOutputURL = dir.appendingPathComponent("source.pair.json")
        do {
            try FileManager.default.linkItem(at: outsideOutputURL, to: hardlinkedOutputURL)
        } catch {
            throw XCTSkip("hardlink unavailable: \(error)")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleArtifactWriter().writeSourcePair(
                .init(
                    bundleRootURL: dir,
                    profilePath: "/tmp/source.profile.json",
                    targetAddress: "127.0.0.1:39395",
                    verificationCode: "123456",
                    localPairingReceiptPath: receiptURL.path,
                    pairingReceiptID: "pair-1",
                    pairStdout: "pair completed"
                )
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

        XCTAssertEqual(try? String(contentsOf: outsideOutputURL, encoding: .utf8), "outside\n")
        XCTAssertFalse(FileManager.default.fileExists(atPath: dir.appendingPathComponent("source.machine.json").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: dir.appendingPathComponent("source.pair.txt").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: dir.appendingPathComponent("exported-receipts/pair-1.json").path))

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertNil(snapshot.meta.evidence.machine_facts?["source"])
        XCTAssertNil(snapshot.meta.roles["source_pair"])
        XCTAssertNil(snapshot.meta.evidence.source_pair)
    }

    func testWriterSourcePairRewritesCanonicalSourceMachineIdentityForInstalledAppProof() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeInstalledAppMachineIdentityBundle(
            to: dir,
            sourceMachineID: "stale-source-machine",
            sourceMachineLabel: "old-source",
            targetMachineID: "target-machine",
            targetMachineLabel: "target"
        )
        let receiptURL = try writePairingReceipt(id: "pair-1", under: dir)

        let writer = AcceptanceBundleArtifactWriter(
            machineIdentityResolver: AcceptanceMachineIdentityResolver(
                resolveCurrentMachine: {
                    AcceptanceMachineIdentity(machineID: "current-source-machine", machineLabel: "source-now")
                }
            )
        )
        try writer.writeSourcePair(
            .init(
                bundleRootURL: dir,
                profilePath: "/tmp/source.profile.json",
                targetAddress: "127.0.0.1:39395",
                verificationCode: "123456",
                localPairingReceiptPath: receiptURL.path,
                pairingReceiptID: "pair-1",
                pairStdout: nil
            )
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.roles["source_pair"]?.machine_id, "current-source-machine")
        XCTAssertEqual(snapshot.sourceMachineFacts?.machine_id, "current-source-machine")
        XCTAssertEqual(snapshot.sourceMachineFactsArtifact?.machine_id, "current-source-machine")
        XCTAssertEqual(snapshot.sourceMachineFactsArtifact?.machine_label, "source-now")
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.finalEvaluationBundleHandoffDetail,
            "missing verified bundle_handoffs"
        )
        XCTAssertFalse(snapshot.installedAppCollectionProof.requiresMachineIdentityCorrection)
        XCTAssertNil(snapshot.installedAppCollectionProof.finalEvaluationCollectionDetail)
        XCTAssertNil(snapshot.installedAppCollectionProof.finalEvaluationMachineFactsDetail)
    }

    func testWriterServePhaseRewritesCanonicalTargetMachineIdentityForInstalledAppProof() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeInstalledAppMachineIdentityBundle(
            to: dir,
            sourceMachineID: "source-machine",
            sourceMachineLabel: "source",
            targetMachineID: "stale-target-machine",
            targetMachineLabel: "old-target"
        )

        let writer = AcceptanceBundleArtifactWriter(
            machineIdentityResolver: AcceptanceMachineIdentityResolver(
                resolveCurrentMachine: {
                    AcceptanceMachineIdentity(machineID: "current-target-machine", machineLabel: "target-now")
                }
            )
        )
        try writer.writeServePhase(
            .init(
                bundleRootURL: dir,
                profilePath: "/tmp/target.profile.json",
                phase: 1,
                readiness: ServeReadinessSnapshot(
                    address: "127.0.0.1:39395",
                    verification_code: "123456",
                    mode: "pairing",
                    receiver_address: "127.0.0.1:9443",
                    receiver_routes: true,
                    push_network: true,
                    trusted: true,
                    transfer: true,
                    expires_at: nil
                )
            )
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.roles["target"]?.machine_id, "current-target-machine")
        XCTAssertEqual(snapshot.targetMachineFacts?.machine_id, "current-target-machine")
        XCTAssertEqual(snapshot.targetMachineFactsArtifact?.machine_id, "current-target-machine")
        XCTAssertEqual(snapshot.targetMachineFactsArtifact?.machine_label, "target-now")
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.finalEvaluationBundleHandoffDetail,
            "missing verified bundle_handoffs"
        )
        XCTAssertFalse(snapshot.installedAppCollectionProof.requiresMachineIdentityCorrection)
        XCTAssertNil(snapshot.installedAppCollectionProof.finalEvaluationCollectionDetail)
        XCTAssertNil(snapshot.installedAppCollectionProof.finalEvaluationMachineFactsDetail)
    }

    func testWriterServePhaseRefreshesCurrentReadySurfaceWithoutStaleVerificationCode() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)

        let writer = AcceptanceBundleArtifactWriter()
        try writer.writeServePhase(
            .init(
                bundleRootURL: dir,
                profilePath: "/tmp/target.profile.json",
                phase: 1,
                readiness: ServeReadinessSnapshot(
                    address: "127.0.0.1:39395",
                    verification_code: "123456",
                    mode: "pairing-only",
                    receiver_address: nil,
                    receiver_routes: nil,
                    push_network: nil,
                    trusted: false,
                    transfer: false,
                    expires_at: nil
                )
            )
        )
        try writer.writeServePhase(
            .init(
                bundleRootURL: dir,
                profilePath: "/tmp/target.profile.json",
                phase: 2,
                readiness: ServeReadinessSnapshot(
                    address: "127.0.0.1:9443",
                    verification_code: nil,
                    mode: "receiver",
                    receiver_address: "127.0.0.1:9443",
                    receiver_routes: true,
                    push_network: true,
                    trusted: true,
                    transfer: true,
                    expires_at: nil
                )
            )
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.evidence.target_ready?.mode, "receiver")
        XCTAssertEqual(snapshot.meta.evidence.target_ready?.verification_code, "")

        let currentReadyData = try Data(contentsOf: dir.appendingPathComponent("target.ready.json"))
        let currentReady = try XCTUnwrap(JSONSerialization.jsonObject(with: currentReadyData) as? [String: Any])
        XCTAssertEqual(currentReady["mode"] as? String, "receiver")
        XCTAssertEqual(currentReady["verification_code"] as? String, "")
        XCTAssertEqual(currentReady["receiver_address"] as? String, "127.0.0.1:9443")
    }

    func testWriterBlocksMismatchedSourceConsistencySession() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)

        let writer = AcceptanceBundleArtifactWriter()
        try writer.writeSourceTransfer(
            .init(
                bundleRootURL: dir,
                profilePath: "/tmp/source.profile.json",
                sessionID: "session-1",
                targetAddress: "127.0.0.1:39395",
                receiverAddress: "127.0.0.1:9443",
                targetMode: "pairing",
                sourceBaselineURL: nil,
                sourceConsistencyRawJSON: #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-other"}"#,
                verifyArtifactPath: nil,
                statusArtifactPath: nil,
                reportArtifactPath: nil,
                healthArtifactPath: nil,
                pushStdout: nil
            )
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.status, "blocked")
        XCTAssertEqual(snapshot.meta.evidence.source_consistency?.mode, "session_mismatch")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.status, "blocked")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.mode, "session_mismatch")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.session_id, "session-1")
    }

    func testWriterRecordsTargetImportMeta() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)

        let writer = AcceptanceBundleArtifactWriter()
        try writer.writeTargetImport(
            .init(
                bundleRootURL: dir,
                profilePath: "/tmp/target.profile.json",
                pairingReceiptID: "pair-1",
                adoptedStdout: "receipt adopted"
            )
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.evidence.target_import?.pairing_receipt_id, "pair-1")
        XCTAssertEqual(snapshot.meta.evidence.target_import?.adopted, "target.adopt-pairing.txt")
        XCTAssertEqual(try String(contentsOf: dir.appendingPathComponent("target.adopt-pairing.txt")), "receipt adopted")
    }

    func testWriterRecordsDiscoveryArtifacts() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)

        let browse = try XCTUnwrap(
            try? JSONDecoder().decode(
                DiscoveryBrowseSnapshot.self,
                from: Data(
                    """
                    {
                      "source": "lan_datagram",
                      "listen": "0.0.0.0:39394",
                      "candidate_count": 1,
                      "invalid_packets": 0,
                      "trusted": false,
                      "candidates": []
                    }
                    """.utf8
                )
            )
        )
        let advertise = try XCTUnwrap(
            try? JSONDecoder().decode(
                DiscoveryAdvertiseSnapshot.self,
                from: Data(
                    """
                    {
                      "status": "advertised",
                      "listen": "0.0.0.0:39394",
                      "destination": "255.255.255.255:39394",
                      "service_type": "_supermover._udp",
                      "protocol_version": "v1",
                      "ephemeral_nonce": "nonce-1",
                      "capability_flags": ["pairing"],
                      "trusted": false,
                      "duration": "10s",
                      "interval": "1s"
                    }
                    """.utf8
                )
            )
        )

        let writer = AcceptanceBundleArtifactWriter()
        try writer.writeDiscoveryBrowse(.init(bundleRootURL: dir, snapshot: browse))
        try writer.writeDiscoveryAdvertise(
            .init(
                bundleRootURL: dir,
                profilePath: "/tmp/target.profile.json",
                snapshot: advertise
            )
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.sourceBrowseSnapshot?.candidate_count, 1)
        XCTAssertEqual(snapshot.targetAdvertiseSnapshot?.destination, "255.255.255.255:39394")
    }

    @MainActor
    func testPairAndImportAutoRecordExportTranscripts() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)
        let receiptURL = try writePairingReceipt(id: "pair-1", under: dir)

        let profileURL = dir.appendingPathComponent("pair.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1",
            "local_pairing_receipt_path": "\(receiptURL.path)"
          },
          "network": {
            "receiver_url": "https://127.0.0.1:9443"
          }
        }
        """.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.pairingTargetAddress = "127.0.0.1:39395"
        store.pairingVerificationCode = "123456"
        store.recentRuns.insert(
            TaskRun(
                kind: .pair,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: store.currentContextSignature(for: .pair),
                processIdentifier: nil,
                stdout: "pair transcript",
                stderr: "",
                state: .finished(0)
            ),
            at: 0
        )

        store.triggerAcceptanceAutoRecordForTesting(.pair)
        var snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.sourcePairArtifact?.receipt_path, "exported-receipts/pair-1.json")
        XCTAssertEqual(snapshot.meta.evidence.source_pair?.pair, "source.pair.txt")
        XCTAssertEqual(try String(contentsOf: dir.appendingPathComponent("source.pair.txt")), "pair transcript")
        XCTAssertTrue(FileManager.default.fileExists(atPath: dir.appendingPathComponent("exported-receipts/pair-1.json").path))

        store.recentRuns.insert(
            TaskRun(
                kind: .profileAdoptPairing,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: store.currentContextSignature(for: .profileAdoptPairing),
                processIdentifier: nil,
                stdout: "adopt transcript",
                stderr: "",
                state: .finished(0)
            ),
            at: 0
        )

        store.triggerAcceptanceAutoRecordForTesting(.profileAdoptPairing)
        snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.evidence.target_import?.adopted, "target.adopt-pairing.txt")
        XCTAssertEqual(try String(contentsOf: dir.appendingPathComponent("target.adopt-pairing.txt")), "adopt transcript")
    }

    @MainActor
    func testAppStoreManualTargetImportArtifactRecordingWhenBundleConfigured() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta(to: dir)

        let profileURL = dir.appendingPathComponent("target.profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1"
          }
        }
        """.write(to: profileURL, atomically: true, encoding: .utf8)

        let store = AppStore()
        store.selectedRole = .target
        store.acceptanceBundlePath = dir.path
        store.profilePath = profileURL.path
        store.recentRuns.insert(
            TaskRun(
                kind: .profileAdoptPairing,
                slot: .foregroundAction,
                launchedAt: Date(),
                commandLine: [],
                contextSignature: store.currentContextSignature(for: .profileAdoptPairing),
                processIdentifier: nil,
                stdout: "manual adopt transcript",
                stderr: "",
                state: .finished(0)
            ),
            at: 0
        )

        store.recordAcceptanceTargetImportArtifact()

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.meta.evidence.target_import?.pairing_receipt_id, "pair-1")
        XCTAssertEqual(snapshot.meta.evidence.target_import?.adopted, "target.adopt-pairing.txt")
        XCTAssertEqual(try String(contentsOf: dir.appendingPathComponent("target.adopt-pairing.txt")), "manual adopt transcript")
    }

    private func makeBundleDirectory() throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }

    private func writePairingReceipt(id: String, under dir: URL) throws -> URL {
        let receiptsDir = dir.appendingPathComponent("local-pairing-receipts", isDirectory: true)
        try FileManager.default.createDirectory(at: receiptsDir, withIntermediateDirectories: true)
        let receiptURL = receiptsDir.appendingPathComponent("\(id).json")
        try """
        {
          "version": 1,
          "id": "\(id)",
          "profile_id": "profile-src",
          "target_id": "target-1",
          "source_device_id": "sha256:1111111111111111111111111111111111111111111111111111111111111111",
          "target_device_id": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
          "device_public_key": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
          "method": "sas",
          "verified_at": "2026-06-04T00:00:00Z",
          "verification_hash": "hash-1",
          "protocol_version": "supermover/v1"
        }
        """.write(to: receiptURL, atomically: true, encoding: .utf8)
        return receiptURL
    }

    private func writeMeta(to dir: URL) throws {
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
    }

    private func writeInstalledAppMachineIdentityBundle(
        to dir: URL,
        sourceMachineID: String,
        sourceMachineLabel: String,
        targetMachineID: String,
        targetMachineLabel: String
    ) throws {
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
              "machine_id": "\(sourceMachineID)",
              "machine_label": "\(sourceMachineLabel)"
            },
            "target": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "\(targetMachineID)",
              "machine_label": "\(targetMachineLabel)"
            }
          },
          "evidence": {
            "machine_facts": {
              "source": {
                "output": "source.machine.json",
                "machine_id": "\(sourceMachineID)",
                "machine_label": "\(sourceMachineLabel)"
              },
              "target": {
                "output": "target.machine.json",
                "machine_id": "\(targetMachineID)",
                "machine_label": "\(targetMachineLabel)"
              }
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "\(sourceMachineID)",
          "machine_label": "\(sourceMachineLabel)"
        }
        """.write(to: dir.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)

        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "\(targetMachineID)",
          "machine_label": "\(targetMachineLabel)"
        }
        """.write(to: dir.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
    }
}
