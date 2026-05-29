import XCTest
@testable import SuperMoverApp

final class ServeReadinessTaskTests: XCTestCase {
    func testServeBuildArgumentsIgnoreRuntimeReadyFile() {
        let input = TaskInput(
            profilePath: "/tmp/profile.json",
            sourceRootPath: "/tmp/source",
            targetRootPath: "/tmp/target",
            profileID: "profile-local",
            profileName: "Local profile",
            targetID: "",
            targetName: "",
            sessionID: "",
            sessionPrefix: "",
            queueEntryID: "",
            syncRetryBackoff: "1m",
            syncInterval: "1m",
            syncMaxRuns: "0",
            syncSettle: "250ms",
            syncMaxEvents: "0",
            syncDiscoveryListen: "0.0.0.0:39394",
            syncDiscoveryTimeout: "2s",
            listenAddress: "127.0.0.1:4000",
            pairingTargetAddress: "",
            pairingVerificationCode: "",
            pairingMethod: "sas",
            pairingTimeout: "5s",
            discoveryBrowseListen: "0.0.0.0:39394",
            discoveryBrowseTimeout: "2s",
            discoveryAdvertiseListen: "",
            discoveryAdvertiseDestination: "255.255.255.255:39394",
            discoveryAdvertiseDuration: "10s",
            discoveryAdvertiseInterval: "1s",
            driftIDsInput: "",
            approvalID: "",
            softDeleteIDsInput: "",
            expiresAt: "",
            reason: "",
            reviewer: ""
        )

        XCTAssertEqual(
            SuperMoverTaskKind.serve.buildArguments(using: input),
            ["serve", "--profile", "/tmp/profile.json", "--listen", "127.0.0.1:4000"]
        )
    }

    func testServeReadinessSnapshotDecodesReceiverEvidence() throws {
        let json = """
        {
          "address": "127.0.0.1:39395",
          "verification_code": "123456",
          "mode": "pairing",
          "receiver_address": "127.0.0.1:9443",
          "receiver_routes": true,
          "push_network": true,
          "trusted": true,
          "transfer": true,
          "expires_at": "2026-06-01T00:00:00Z"
        }
        """
        let decoded = try JSONDecoder().decode(ServeReadinessSnapshot.self, from: XCTUnwrap(json.data(using: .utf8)))

        XCTAssertEqual(decoded.address, "127.0.0.1:39395")
        XCTAssertEqual(decoded.receiver_address, "127.0.0.1:9443")
        XCTAssertEqual(decoded.mode, "pairing")
        XCTAssertEqual(decoded.receiver_routes, true)
        XCTAssertEqual(decoded.push_network, true)
        XCTAssertTrue(decoded.trusted)
        XCTAssertTrue(decoded.transfer)
    }

    @MainActor
    func testServeReadinessRefreshDecodesCurrentReadyFile() throws {
        let store = AppStore()
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        let readyURL = dir.appendingPathComponent("serve-ready.json")
        try """
        {
          "address": "127.0.0.1:39395",
          "verification_code": "123456",
          "mode": "pairing-only",
          "trusted": false,
          "transfer": false
        }
        """.write(to: readyURL, atomically: true, encoding: .utf8)

        store.installServeReadyFileForTesting(readyURL.path, contextSignature: store.currentContextSignature(for: .serve))
        store.refreshServeReadinessFromFileIfCurrent(slot: .targetServe, kind: .serve)

        XCTAssertEqual(store.serveReadinessSnapshot?.address, "127.0.0.1:39395")
        XCTAssertEqual(store.serveReadinessSnapshot?.verification_code, "123456")
        XCTAssertEqual(store.serveReadinessSnapshot?.mode, "pairing-only")
    }

    @MainActor
    func testServeReadinessRefreshClearsCurrentSnapshotWhenReadyFileIsMissing() {
        let store = AppStore()
        store.serveReadinessSnapshot = ServeReadinessSnapshot(
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

        let missingURL = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        store.installServeReadyFileForTesting(
            missingURL.path,
            contextSignature: store.currentContextSignature(for: .serve)
        )
        store.refreshServeReadinessFromFileIfCurrent(slot: .targetServe, kind: .serve)

        XCTAssertNil(store.serveReadinessSnapshot)
    }

    @MainActor
    func testServeReadinessRefreshClearsCurrentSnapshotWhenReadyFileIsMalformed() throws {
        let store = AppStore()
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        let readyURL = dir.appendingPathComponent("serve-ready.json")
        try "not-json".write(to: readyURL, atomically: true, encoding: .utf8)

        store.serveReadinessSnapshot = ServeReadinessSnapshot(
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
        store.installServeReadyFileForTesting(
            readyURL.path,
            contextSignature: store.currentContextSignature(for: .serve)
        )
        store.refreshServeReadinessFromFileIfCurrent(slot: .targetServe, kind: .serve)

        XCTAssertNil(store.serveReadinessSnapshot)
    }
}
