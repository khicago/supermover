import XCTest
@testable import SuperMoverApp

final class AcceptanceTwoMachineCurrentStateMergeTests: XCTestCase {
    func testMergeBundleAdvancesTargetCurrentReadyStateWhilePreservingPhaseArtifacts() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "two-machine-merge-current-target-ready")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let destinationBundle = workDir.appendingPathComponent("destination-bundle", isDirectory: true)
        let incomingBundle = workDir.appendingPathComponent("incoming-bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: destinationBundle, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: incomingBundle, withIntermediateDirectories: true)

        let phase1Ready = try makeReadyJSON(
            address: "[::]:58968",
            verificationCode: "phase-1-code",
            mode: "pairing-only"
        )
        let phase2Ready = try makeReadyJSON(
            address: "[::]:58982",
            verificationCode: "phase-2-code",
            mode: "pairing",
            receiverAddress: "127.0.0.1:58954",
            receiverRoutes: true,
            pushNetwork: true,
            trusted: true,
            transfer: true
        )

        try writeMeta(
            to: destinationBundle.appendingPathComponent("meta.json"),
            targetReady: [
                "address": "[::]:58968",
                "verification_code": "phase-1-code",
                "mode": "pairing-only",
            ],
            targetServePhases: [
                ["phase": 1, "ready": "target.ready.phase-1.json"],
            ]
        )
        try phase1Ready.write(
            to: destinationBundle.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )
        try phase1Ready.write(
            to: destinationBundle.appendingPathComponent("target.ready.phase-1.json"),
            atomically: true,
            encoding: .utf8
        )

        try writeMeta(
            to: incomingBundle.appendingPathComponent("meta.json"),
            targetReady: [
                "address": "[::]:58982",
                "verification_code": "phase-2-code",
                "mode": "pairing",
            ],
            targetServePhases: [
                ["phase": 1, "ready": "target.ready.phase-1.json"],
                ["phase": 2, "ready": "target.ready.phase-2.json"],
            ]
        )
        try phase1Ready.write(
            to: incomingBundle.appendingPathComponent("target.ready.phase-1.json"),
            atomically: true,
            encoding: .utf8
        )
        try phase2Ready.write(
            to: incomingBundle.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )
        try phase2Ready.write(
            to: incomingBundle.appendingPathComponent("target.ready.phase-2.json"),
            atomically: true,
            encoding: .utf8
        )
        try "3\n".write(
            to: incomingBundle.appendingPathComponent("target.serve.next-phase"),
            atomically: true,
            encoding: .utf8
        )
        try "99999\n".write(
            to: incomingBundle.appendingPathComponent("target.serve.phase-2.pid"),
            atomically: true,
            encoding: .utf8
        )

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh"),
            arguments: [
                "merge-bundle",
                "--bundle-root", destinationBundle.path,
                "--incoming-bundle-root", incomingBundle.path,
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: destinationBundle.appendingPathComponent("target.ready.phase-2.json").path
            )
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: destinationBundle.appendingPathComponent("target.serve.next-phase").path
            )
        )
        XCTAssertFalse(
            FileManager.default.fileExists(
                atPath: destinationBundle.appendingPathComponent("target.serve.phase-2.pid").path
            )
        )

        let targetReadyData = try Data(contentsOf: destinationBundle.appendingPathComponent("target.ready.json"))
        let targetReady = try XCTUnwrap(JSONSerialization.jsonObject(with: targetReadyData) as? [String: Any])
        XCTAssertEqual(targetReady["mode"] as? String, "pairing")
        XCTAssertEqual(targetReady["receiver_address"] as? String, "127.0.0.1:58954")

        let metaData = try Data(contentsOf: destinationBundle.appendingPathComponent("meta.json"))
        let meta = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        let evidence = try XCTUnwrap(meta["evidence"] as? [String: Any])
        let targetReadyEvidence = try XCTUnwrap(evidence["target_ready"] as? [String: Any])
        XCTAssertEqual(targetReadyEvidence["mode"] as? String, "pairing")
        let targetServePhases = try XCTUnwrap(evidence["target_serve_phases"] as? [[String: Any]])
        XCTAssertEqual(targetServePhases.count, 2)
        XCTAssertEqual(targetServePhases.compactMap { $0["phase"] as? Int }, [1, 2])
    }

    private func writeMeta(
        to url: URL,
        targetReady: [String: Any],
        targetServePhases: [[String: Any]]
    ) throws {
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "schema": "supermover.acceptance.two_machine.v1",
            "status": "in_progress",
            "collection": [
                "mode": "two_machine",
                "machine_count": 2,
            ],
            "roles": [:],
            "evidence": [
                "target_ready": targetReady,
                "target_serve_phases": targetServePhases,
            ],
        ]).write(to: url, atomically: true, encoding: .utf8)
    }

    private func makeReadyJSON(
        address: String,
        verificationCode: String,
        mode: String,
        receiverAddress: String? = nil,
        receiverRoutes: Bool? = nil,
        pushNetwork: Bool? = nil,
        trusted: Bool? = nil,
        transfer: Bool? = nil
    ) throws -> String {
        var object: [String: Any] = [
            "address": address,
            "verification_code": verificationCode,
            "mode": mode,
        ]
        if let receiverAddress {
            object["receiver_address"] = receiverAddress
        }
        if let receiverRoutes {
            object["receiver_routes"] = receiverRoutes
        }
        if let pushNetwork {
            object["push_network"] = pushNetwork
        }
        if let trusted {
            object["trusted"] = trusted
        }
        if let transfer {
            object["transfer"] = transfer
        }
        return try AcceptanceReleaseEvidenceFixtures.jsonString(object)
    }
}
