import XCTest
@testable import SuperMoverApp

final class AcceptanceInstalledAppReleaseEvidenceEvaluationScriptTests: XCTestCase {
    func testShellEvaluateRejectsSourceNotarizationThatDoesNotMatchBundledReleaseEvidence() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-evaluate-stale-notary")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        let targetRoot = workDir.appendingPathComponent("target-root", isDirectory: true)
        let controlPlane = targetRoot.appendingPathComponent(".supermover", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("pairings"), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("sessions/session-1"), withIntermediateDirectories: true)

        try evaluationBundleMeta.write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try machineFactsJSON(machineID: "source-machine", label: "source")
            .write(to: bundleRoot.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try machineFactsJSON(machineID: "target-machine", label: "target")
            .write(to: bundleRoot.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        let provenanceManifest = AcceptanceReleaseEvidenceFixtures.developerIDProvenanceManifest(cliVersion: "supermover v0")
        let sourceAppPath = "/Applications/SuperMover Source.app"
        let targetAppPath = "/Applications/SuperMover Target.app"

        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("source.provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.jsonString(provenanceManifest)
            .write(to: bundleRoot.appendingPathComponent("target.provenance.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: sourceAppPath,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("source.app-audit.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.appAuditJSON(
            appPath: targetAppPath,
            provenanceManifest: provenanceManifest
        ).write(to: bundleRoot.appendingPathComponent("target.app-audit.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(
            to: bundleRoot.appendingPathComponent("source.notary-log.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(
            to: bundleRoot.appendingPathComponent("target.notary-log.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: "/Applications/Stale Source.app",
            auditPath: "/Applications/Stale Source.app.audit.json",
            notaryLogPath: "source.notary-log.json"
        ).write(to: bundleRoot.appendingPathComponent("source.notarization.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.notarizationJSON(
            appPath: targetAppPath,
            auditPath: AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditPath(appPath: targetAppPath),
            notaryLogPath: "target.notary-log.json"
        ).write(to: bundleRoot.appendingPathComponent("target.notarization.json"), atomically: true, encoding: .utf8)

        try sourcePairJSON.write(to: bundleRoot.appendingPathComponent("source.pair.json"), atomically: true, encoding: .utf8)
        let exportedReceipts = bundleRoot.appendingPathComponent("exported-receipts", isDirectory: true)
        try FileManager.default.createDirectory(at: exportedReceipts, withIntermediateDirectories: true)
        try AcceptanceWorkflowFixtures.pairingReceiptJSON().write(
            to: exportedReceipts.appendingPathComponent("pair-1.json"),
            atomically: true,
            encoding: .utf8
        )
        try "pair ok\n".write(to: bundleRoot.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)
        try """
        {
          "address": "127.0.0.1:39395",
          "verification_code": "123456",
          "mode": "pairing",
          "receiver_address": "127.0.0.1:9443",
          "receiver_routes": true,
          "push_network": true,
          "trusted": false,
          "transfer": true
        }
        """.write(to: bundleRoot.appendingPathComponent("target.ready.json"), atomically: true, encoding: .utf8)
        try sourceTransferJSON.write(to: bundleRoot.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted\n".write(to: bundleRoot.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok\n".write(to: bundleRoot.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try sourceConsistencyJSON.write(to: bundleRoot.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)
        try #"{"baseline":"current"}"#.write(to: bundleRoot.appendingPathComponent("source.baseline.json"), atomically: true, encoding: .utf8)
        try verifyJSON(targetRoot: targetRoot).write(to: bundleRoot.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try reportJSON.write(to: bundleRoot.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try statusJSON(targetRoot: targetRoot).write(to: bundleRoot.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try healthJSON(targetRoot: targetRoot).write(to: bundleRoot.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.pairingReceiptJSON().write(
            to: controlPlane.appendingPathComponent("pairings/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )
        try AcceptanceWorkflowFixtures.targetNetworkTransferJSON().write(
            to: controlPlane.appendingPathComponent("sessions/session-1/network-transfer.json"),
            atomically: true,
            encoding: .utf8
        )

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundleRoot.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 1)
        XCTAssertTrue(
            result.stderr.contains("source.notarization.json does not match source.app-audit.json and source.provenance.json")
                || result.stderr.contains("source.notarization.json does not reference the current source post-staple audit"),
            "stderr:\n\(result.stderr)"
        )
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundleRoot.appendingPathComponent("evaluation.json").path))
    }

    private func machineFactsJSON(machineID: String, label: String) -> String {
        """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "\(machineID)",
          "machine_label": "\(label)"
        }
        """
    }

    private func verifyJSON(targetRoot: URL) -> String {
        #"{"summary":{"files_verified":1,"files_expected":1,"error_findings":0,"artifact_problems":0},"manifest":{"manifestID":"manifest-1"},"target_root":"\#(targetRoot.path)","session_id":"session-1","merkleRootProof":{"detail":"unavailable"}}"#
    }

    private func statusJSON(targetRoot: URL) -> String {
        AcceptanceWorkflowFixtures.statusJSON(targetRoot: targetRoot.path)
    }

    private func healthJSON(targetRoot: URL) -> String {
        AcceptanceWorkflowFixtures.healthJSON(targetRoot: targetRoot.path)
    }

    private var evaluationBundleMeta: String {
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
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
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
            ],
            "source_pair": {
              "output": "source.pair.json",
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "pair": "source.pair.txt"
            },
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "verify": "source.verify.json",
              "status": "source.status.json",
              "report": "source.report.json",
              "health": "source.health.json",
              "push": "source.network-push.txt"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "operator": {
              "local_network": {"status":"pass","detail":"ok"},
              "firewall": {"status":"pass","detail":"ok"},
              "pairing_confirmation": {"status":"pass","detail":"ok"}
            }
          }
        }
        """
    }

    private var sourcePairJSON: String {
        #"{"profile":"/tmp/source.profile.json","target_address":"127.0.0.1:39395","verification_code":"123456","pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json"}"#
    }

    private var sourceTransferJSON: String {
        #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#
    }

    private var sourceConsistencyJSON: String {
        #"{"schema":"supermover.acceptance.current_source_consistency.v1","status":"pass","mode":"current_source_verified","session_id":"session-1"}"#
    }

    private var reportJSON: String {
        #"{"pairing":{"receipt_id":"pair-1","status":"paired_receipt_valid"}}"#
    }

}
