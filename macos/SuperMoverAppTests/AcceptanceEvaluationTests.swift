import XCTest
@testable import SuperMoverApp

final class AcceptanceEvaluationTests: XCTestCase {
    func testEvaluationCoordinatorWritesEvaluationArtifact() throws {
        let bundle = try makeDirectory(named: "bundle")
        let targetRoot = try makeDirectory(named: "target-root")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try writeTargetReadyFixture(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(
            to: bundle.appendingPathComponent("source.transfer.json"),
            atomically: true,
            encoding: .utf8
        )
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
              {
                "archive":"source-to-target.tgz",
                "manifest":"source-to-target.manifest.json",
                "sha256":"1111111111111111111111111111111111111111111111111111111111111111",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"source-machine",
                "exporting_machine_label":"source",
                "importing_machine_id":"target-machine",
                "importing_machine_label":"target"
              }
            ],
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json",
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
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        let controlPlane = targetRoot.appendingPathComponent(".supermover", isDirectory: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("pairings"), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("sessions/session-1"), withIntermediateDirectories: true)
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

        let result = try AcceptanceBundleEvaluationCoordinator().evaluate(
            bundleRootURL: bundle,
            targetRootURL: targetRoot,
            requireOperatorEvidence: true
        )

        XCTAssertEqual(result.kind, .evaluation)
        let loaded = try AcceptanceBundleReader().load(bundleRootURL: bundle)
        XCTAssertEqual(loaded.status, "evidence_collected")
        XCTAssertEqual(loaded.meta.evidence.evaluation?.output, "evaluation.json")
        XCTAssertEqual(loaded.meta.evidence.evaluation?.require_operator_evidence, true)
        XCTAssertEqual(loaded.evaluationArtifact?.session_id, "session-1")
    }

    func testEvaluationCoordinatorAcceptsDistributionReadyInstalledAppAudit() throws {
        let bundle = try makeDirectory(named: "bundle-distribution-ready")
        let targetRoot = try makeDirectory(named: "target-root-distribution-ready")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeDistributionReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeDistributionReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try writeTargetReadyFixture(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(
            to: bundle.appendingPathComponent("source.transfer.json"),
            atomically: true,
            encoding: .utf8
        )
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
              {
                "archive":"source-to-target.tgz",
                "manifest":"source-to-target.manifest.json",
                "sha256":"1111111111111111111111111111111111111111111111111111111111111111",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"source-machine",
                "exporting_machine_label":"source",
                "importing_machine_id":"target-machine",
                "importing_machine_label":"target"
              }
            ],
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json",
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
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertNoThrow(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        )
    }

    func testEvaluationCoordinatorFailsClosedWhenReleaseNotarizationEvidenceIsMissing() throws {
        let bundle = try makeDirectory(named: "bundle-missing-notarization")
        let targetRoot = try makeDirectory(named: "target-root-missing-notarization")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeDistributionReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeDistributionReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(
            to: bundle.appendingPathComponent("source.transfer.json"),
            atomically: true,
            encoding: .utf8
        )
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json",
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
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingRequiredArtifact("source.notarization.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWithoutOperatorEvidence() throws {
        let bundle = try makeDirectory(named: "bundle-missing-operator")
        let targetRoot = try makeDirectory(named: "target-root-missing-operator")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
              {
                "archive":"source-to-target.tgz",
                "manifest":"source-to-target.manifest.json",
                "sha256":"1111111111111111111111111111111111111111111111111111111111111111",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"source-machine",
                "exporting_machine_label":"source",
                "importing_machine_id":"target-machine",
                "importing_machine_label":"target"
              }
            ],
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"blocked","detail":"dismissed"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingOperatorEvidence("firewall")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenOperatorEvidenceDetailIsBlankInStrictLane() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-blank-operator-detail",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try setOperatorEvidenceDetail(
            kind: "local_network",
            detail: " \n\t ",
            bundleRoot: bundle
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingOperatorEvidence("local_network")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenInstalledAppMachineFactsAreMissing() throws {
        let bundle = try makeDirectory(named: "bundle-missing-machine-facts")
        let targetRoot = try makeDirectory(named: "target-root-missing-machine-facts")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"same-machine","machine_label":"shared"},
              "target": {"output":"target.machine.json","machine_id":"same-machine","machine_label":"shared"}
            },
            "bundle_handoffs": [
              {
                "archive":"source-to-target.tgz",
                "manifest":"source-to-target.manifest.json",
                "sha256":"1111111111111111111111111111111111111111111111111111111111111111",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"source-machine",
                "exporting_machine_label":"source",
                "importing_machine_id":"target-machine",
                "importing_machine_label":"target"
              }
            ],
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingRequiredArtifact("source.machine.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenInstalledAppMachineFactsMatch() throws {
        let bundle = try makeDirectory(named: "bundle-matching-machine-facts")
        let targetRoot = try makeDirectory(named: "target-root-matching-machine-facts")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "same-machine",
          "machine_label": "shared"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "same-machine",
          "machine_label": "shared"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"same-machine","machine_label":"shared"},
              "target": {"output":"target.machine.json","machine_id":"same-machine","machine_label":"shared"}
            },
            "bundle_handoffs": [
              {
                "archive":"source-to-target.tgz",
                "manifest":"source-to-target.manifest.json",
                "sha256":"1111111111111111111111111111111111111111111111111111111111111111",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"source-machine",
                "exporting_machine_label":"source",
                "importing_machine_id":"target-machine",
                "importing_machine_label":"target"
              }
            ],
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection("source and target machine facts share machine_id=same-machine")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenInstalledAppBundleHandoffsAreMissing() throws {
        let bundle = try makeDirectory(named: "bundle-missing-bundle-handoffs")
        let targetRoot = try makeDirectory(named: "target-root-missing-bundle-handoffs")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection("missing verified bundle_handoffs")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenRoleMachineIDsDoNotMatchMachineFacts() throws {
        let bundle = try makeDirectory(named: "bundle-machine-facts-role-mismatch")
        let targetRoot = try makeDirectory(named: "target-root-machine-facts-role-mismatch")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-role-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-role-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
              {
                "archive":"source-to-target.tgz",
                "manifest":"source-to-target.manifest.json",
                "sha256":"1111111111111111111111111111111111111111111111111111111111111111",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"source-machine",
                "exporting_machine_label":"source",
                "importing_machine_id":"target-machine",
                "importing_machine_label":"target"
              }
            ],
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection(
                    "roles.source_pair/target machine_id do not match source.machine.json and target.machine.json"
                )
            )
        }
    }

    func testEvaluationCoordinatorCanonicalMachineFactsPreferCanonicalArtifactsOverAlternateMetaOutputs() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-canonical-machine-facts-preferred",
            sourceMachineFactsOutput: "source.machine.selected.json",
            targetMachineFactsOutput: "target.machine.selected.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.selected.json",
            machineID: "other-source-machine",
            machineLabel: "other-source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.selected.json",
            machineID: "other-target-machine",
            machineLabel: "other-target"
        )

        let result = try AcceptanceBundleEvaluationCoordinator().evaluate(
            bundleRootURL: bundle,
            targetRootURL: targetRoot,
            requireOperatorEvidence: true
        )

        XCTAssertEqual(result.kind, .evaluation)
    }

    func testEvaluationCoordinatorRejectsMalformedSourceStatusArtifact() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-malformed-source-status",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try #"{"target_root":"\#(targetRoot.path)"}"#.write(
            to: bundle.appendingPathComponent("source.status.json"),
            atomically: true,
            encoding: .utf8
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidStatusEvidence
            )
        }
    }

    func testEvaluationCoordinatorRejectsMalformedSourceHealthArtifact() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-malformed-source-health",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try #"{"status":"ok"}"#.write(
            to: bundle.appendingPathComponent("source.health.json"),
            atomically: true,
            encoding: .utf8
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidHealthEvidence
            )
        }
    }

    func testEvaluationCoordinatorRejectsSourceTransferEvidenceFromDifferentTargetRoot() throws {
        let cases: [(name: String, artifact: String, payload: (String) -> String, expected: AcceptanceBundleEvaluationCoordinator.EvaluationError)] = [
            ("verify", "source.verify.json", makeVerifyJSON, .invalidVerifyEvidence),
            ("report", "source.report.json", makeReportJSON, .invalidReportEvidence),
            ("status", "source.status.json", makeStatusJSON, .invalidStatusEvidence),
            ("health", "source.health.json", makeHealthJSON, .invalidHealthEvidence),
        ]

        for testCase in cases {
            let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
                named: "bundle-\(testCase.name)-wrong-target-root",
                sourceMachineFactsOutput: "source.machine.json",
                targetMachineFactsOutput: "target.machine.json"
            )
            let otherTargetRoot = try makeDirectory(named: "bundle-\(testCase.name)-other-target-root")
            defer {
                try? FileManager.default.removeItem(at: bundle)
                try? FileManager.default.removeItem(at: targetRoot)
                try? FileManager.default.removeItem(at: otherTargetRoot)
            }
            try writeMachineFactsArtifact(
                bundleRoot: bundle,
                relativePath: "source.machine.json",
                machineID: "source-machine",
                machineLabel: "source"
            )
            try writeMachineFactsArtifact(
                bundleRoot: bundle,
                relativePath: "target.machine.json",
                machineID: "target-machine",
                machineLabel: "target"
            )
            try testCase.payload(otherTargetRoot.path).write(
                to: bundle.appendingPathComponent(testCase.artifact),
                atomically: true,
                encoding: .utf8
            )

            XCTAssertThrowsError(
                try AcceptanceBundleEvaluationCoordinator().evaluate(
                    bundleRootURL: bundle,
                    targetRootURL: targetRoot,
                    requireOperatorEvidence: true
                ),
                testCase.name
            ) { error in
                XCTAssertEqual(
                    error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                    testCase.expected,
                    testCase.name
                )
            }
        }
    }

    func testEvaluationCoordinatorRejectsMissingTargetReadyArtifact() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-missing-target-ready",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try FileManager.default.removeItem(at: bundle.appendingPathComponent("target.ready.json"))

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingRequiredArtifact("target.ready.json")
            )
        }
    }

    func testEvaluationCoordinatorRejectsMalformedTargetReadyArtifact() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-malformed-target-ready",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try #"{"address":"127.0.0.1:39395","mode":"pairing"}"#.write(
            to: bundle.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("target.ready.json")
            )
        }
    }

    func testEvaluationCoordinatorRejectsMissingReferencedTargetImportArtifact() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-missing-target-import-artifact",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try FileManager.default.removeItem(at: bundle.appendingPathComponent("target.adopt-pairing.txt"))

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingRequiredArtifact("target.adopt-pairing.txt")
            )
        }
    }

    func testEvaluationCoordinatorRejectsMissingTargetImportAdoptedTranscript() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-missing-target-import-adopted-transcript",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        let metaURL = bundle.appendingPathComponent("meta.json")
        let metaData = try Data(contentsOf: metaURL)
        var root = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        var evidence = try XCTUnwrap(root["evidence"] as? [String: Any])
        var targetImport = try XCTUnwrap(evidence["target_import"] as? [String: Any])
        targetImport.removeValue(forKey: "adopted")
        evidence["target_import"] = targetImport
        root["evidence"] = evidence
        let updatedMeta = try JSONSerialization.data(withJSONObject: root, options: [.prettyPrinted, .sortedKeys])
        try updatedMeta.write(to: metaURL, options: .atomic)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingRequiredArtifact("target_import adopted artifact")
            )
        }
    }

    func testEvaluationCoordinatorRejectsSourcePairTargetAddressMismatchWithTargetReady() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-source-pair-target-ready-address-mismatch",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try """
        {
          "profile": "/tmp/source.profile.json",
          "target_address": "127.0.0.1:49999",
          "verification_code": "123456",
          "pairing_receipt_id": "pair-1",
          "receipt_path": "\(sourcePairReceiptRelativePath)"
        }
        """.write(
            to: bundle.appendingPathComponent("source.pair.json"),
            atomically: true,
            encoding: .utf8
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("source.pair.json")
            )
        }
    }

    func testEvaluationCoordinatorRejectsSourceTransferReceiverMismatchWithTargetReady() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-source-transfer-target-ready-receiver-mismatch",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try """
        {
          "profile": "/tmp/source.profile.json",
          "session_id": "session-1",
          "target_address": "127.0.0.1:39395",
          "receiver_address": "127.0.0.1:9555",
          "target_mode": "pairing"
        }
        """.write(
            to: bundle.appendingPathComponent("source.transfer.json"),
            atomically: true,
            encoding: .utf8
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("source.transfer.json")
            )
        }
    }

    func testEvaluationCoordinatorRejectsSourceTransferWhenTargetReadyLacksReceiverTransferProof() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-target-ready-lacks-receiver-transfer",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try """
        {
          "address": "127.0.0.1:39395",
          "verification_code": "123456",
          "mode": "pairing",
          "receiver_routes": false,
          "push_network": false,
          "trusted": false,
          "transfer": false
        }
        """.write(
            to: bundle.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("target.ready.json")
            )
        }
    }

    func testEvaluationCoordinatorCanonicalMachineFactsRejectAlternateMetaOutputsThatMaskCanonicalMismatch() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-canonical-machine-facts-mismatch",
            sourceMachineFactsOutput: "source.machine.selected.json",
            targetMachineFactsOutput: "target.machine.selected.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "other-source-machine",
            machineLabel: "other-source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "other-target-machine",
            machineLabel: "other-target"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.selected.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.selected.json",
            machineID: "target-machine",
            machineLabel: "target"
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection(
                    "roles.source_pair/target machine_id do not match source.machine.json and target.machine.json"
                )
            )
        }
    }

    func testEvaluationCoordinatorCanonicalMachineFactsRejectValidAlternateMetaOutputsWhenCanonicalArtifactsAreMalformed() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-canonical-machine-facts-malformed",
            sourceMachineFactsOutput: "source.machine.selected.json",
            targetMachineFactsOutput: "target.machine.selected.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_label": "source"
        }
        """.write(
            to: bundle.appendingPathComponent("source.machine.json"),
            atomically: true,
            encoding: .utf8
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.selected.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.selected.json",
            machineID: "target-machine",
            machineLabel: "target"
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .malformedArtifact("source.machine.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenBundleHandoffsDoNotProveCrossMachineTransfer() throws {
        let bundle = try makeDirectory(named: "bundle-same-machine-bundle-handoff")
        let targetRoot = try makeDirectory(named: "target-root-same-machine-bundle-handoff")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
              {
                "archive":"source-to-target.tgz",
                "manifest":"source-to-target.manifest.json",
                "sha256":"1111111111111111111111111111111111111111111111111111111111111111",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"source-machine",
                "exporting_machine_label":"source",
                "importing_machine_id":"source-machine",
                "importing_machine_label":"source"
              }
            ],
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection(
                    "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"
                )
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenBundleHandoffsProveTheWrongMachinePair() throws {
        let bundle = try makeDirectory(named: "bundle-wrong-machine-pair-bundle-handoff")
        let targetRoot = try makeDirectory(named: "target-root-wrong-machine-pair-bundle-handoff")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
              {
                "archive":"source-to-target.tgz",
                "manifest":"source-to-target.manifest.json",
                "sha256":"1111111111111111111111111111111111111111111111111111111111111111",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"third-source-machine",
                "exporting_machine_label":"other-source",
                "importing_machine_id":"third-target-machine",
                "importing_machine_label":"other-target"
              }
            ],
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection(
                    "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"
                )
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenBundleHandoffsContainContradictoryMachinePairs() throws {
        let bundle = try makeDirectory(named: "bundle-contradictory-bundle-handoffs")
        let targetRoot = try makeDirectory(named: "target-root-contradictory-bundle-handoffs")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
              {
                "archive":"source-to-target.tgz",
                "manifest":"source-to-target.manifest.json",
                "sha256":"1111111111111111111111111111111111111111111111111111111111111111",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"source-machine",
                "exporting_machine_label":"source",
                "importing_machine_id":"target-machine",
                "importing_machine_label":"target"
              },
              {
                "archive":"wrong-pair.tgz",
                "manifest":"wrong-pair.manifest.json",
                "sha256":"2222222222222222222222222222222222222222222222222222222222222222",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"third-source-machine",
                "exporting_machine_label":"other-source",
                "importing_machine_id":"third-target-machine",
                "importing_machine_label":"other-target"
              }
            ],
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidInstalledAppCollection(
                    "bundle_handoffs contain verified cross-machine archive handoff evidence for machine ids other than the recorded source/target pair"
                )
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWithoutTargetImportEvidence() throws {
        let bundle = try makeDirectory(named: "bundle-missing-target-import")
        let targetRoot = try makeDirectory(named: "target-root-missing-target-import")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","status":"pass","mode":"current_source_verified"}
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: false
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingTargetImportEvidence
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenCurrentSourceBaselineArtifactIsMissing() throws {
        let bundle = try makeDirectory(named: "bundle-missing-source-baseline")
        let targetRoot = try makeDirectory(named: "target-root-missing-source-baseline")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"}
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: false
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .missingRequiredArtifact("source.baseline.json")
            )
        }
    }

    @MainActor
    func testAppStoreSourceRoleCannotRecordEvaluation() {
        let store = AppStore()
        store.selectedRole = .source
        store.targetRootPath = "/tmp/target-root"

        store.recordAcceptanceEvaluationArtifact()

        XCTAssertTrue(store.note.contains("Source role cannot finalize acceptance evaluation"))
    }

    func testEvaluationCoordinatorFailsClosedForSameMachineCollectionWhenOperatorEvidenceRequired() throws {
        let bundle = try makeDirectory(named: "bundle-same-machine-operator")
        let targetRoot = try makeDirectory(named: "target-root-same-machine-operator")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "blocked", mode: "not_wired").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "same_machine",
            "machine_count": 1
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "same-machine", "machine_label": "same-machine source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "same-machine", "machine_label": "same-machine target"}
          },
          "evidence": {
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"blocked","mode":"not_wired"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .blockedCurrentSourceConsistency("not_wired")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenInstalledAppAuditIsBlocked() throws {
        let bundle = try makeDirectory(named: "bundle-blocked-app-audit")
        let targetRoot = try makeDirectory(named: "target-root-blocked-app-audit")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeBlockedAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .blockedAppAudit("source")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenInstalledAppNotarizationDoesNotMatchBundledReleaseEvidence() throws {
        let bundle = try makeDirectory(named: "bundle-stale-installed-app-notarization")
        let targetRoot = try makeDirectory(named: "target-root-stale-installed-app-notarization")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeDistributionReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeDistributionReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(
            to: bundle.appendingPathComponent("source.notary-log.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "schema": "supermover.macos.notarization.v1",
          "checked_at": "2026-06-01T12:00:00Z",
          "status": "pass",
          "app_path": "/tmp/stale-source/SuperMover.app",
          "work_dir": "/tmp/stale-source/SuperMover.app.notary",
          "auth_mode": "keychain_profile",
          "archive_path": "/tmp/stale-source/SuperMover.app.notary/SuperMover.app.zip",
          "submission": {
            "id": "11111111-1111-1111-1111-111111111111",
            "status": "Accepted",
            "message": "Ready for distribution"
          },
          "notary_log": {
            "path": "source.notary-log.json"
          },
          "audit": {
            "path": "/tmp/stale-source/SuperMover.app.notary/post-staple.audit.json",
            "status": "pass",
            "readiness": "distribution_ready",
            "pass_ready": true,
            "blocking_checks": 0
          },
          "failure": null
        }
        """.write(
            to: bundle.appendingPathComponent("source.notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
              {
                "archive":"source-to-target.tgz",
                "manifest":"source-to-target.manifest.json",
                "sha256":"1111111111111111111111111111111111111111111111111111111111111111",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"source-machine",
                "exporting_machine_label":"source",
                "importing_machine_id":"target-machine",
                "importing_machine_label":"target"
              }
            ],
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidNotarizationEvidence("source")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenBundleLocalNotaryLogIsMalformed() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-malformed-notary-log",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try "not json\n".write(
            to: bundle.appendingPathComponent("source.notary-log.json"),
            atomically: true,
            encoding: .utf8
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidNotarizationEvidence("source")
            )
        }
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("evaluation.json").path))
    }

    func testEvaluationCoordinatorFailsClosedWhenInstalledAppAuditProvenanceDoesNotMatchBundledReleaseEvidence() throws {
        let bundle = try makeDirectory(named: "bundle-stale-installed-app-audit-provenance")
        let targetRoot = try makeDirectory(named: "target-root-stale-installed-app-audit-provenance")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        let sourceProvenance = try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundle,
            machine: "source",
            appPath: "/tmp/current-source/SuperMover.app"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: bundle,
            machine: "target",
            appPath: "/tmp/current-target/SuperMover.app"
        )
        var staleSourceProvenance = sourceProvenance
        staleSourceProvenance["git_dirty"] = true
        try AcceptanceReleaseEvidenceFixtures.jsonString(staleSourceProvenance).write(
            to: bundle.appendingPathComponent("source.provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "machine_facts": {
              "source": {"output":"source.machine.json","machine_id":"source-machine","machine_label":"source"},
              "target": {"output":"target.machine.json","machine_id":"target-machine","machine_label":"target"}
            },
            "bundle_handoffs": [
              {
                "archive":"source-to-target.tgz",
                "manifest":"source-to-target.manifest.json",
                "sha256":"1111111111111111111111111111111111111111111111111111111111111111",
                "meta":"meta.json",
                "verified":true,
                "exporting_machine_id":"source-machine",
                "exporting_machine_label":"source",
                "importing_machine_id":"target-machine",
                "importing_machine_label":"target"
              }
            ],
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"},
            "operator": {
              "local_network": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "firewall": {"status":"pass","detail":"ok","machine_id":"target-machine","machine_label":"target"},
              "pairing_confirmation": {"status":"pass","detail":"ok","machine_id":"source-machine","machine_label":"source"}
            }
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: bundle.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: bundle.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .blockedAppAudit("source")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenTargetTransferIsNotDurableMTLSEvidence() throws {
        let bundle = try makeDirectory(named: "bundle-invalid-transfer-encryption")
        let targetRoot = try makeDirectory(named: "target-root-invalid-transfer-encryption")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"}
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let controlPlane = targetRoot.appendingPathComponent(".supermover", isDirectory: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("pairings"), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: controlPlane.appendingPathComponent("sessions/session-1"), withIntermediateDirectories: true)
        try AcceptanceWorkflowFixtures.pairingReceiptJSON().write(
            to: controlPlane.appendingPathComponent("pairings/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )
        try #"{"status":"published","stage":"commit","encrypted_transfer":"profile_backed_mtls_validated","source_device_id":"src-spki","target_device_id":"dst-spki"}"#.write(
            to: controlPlane.appendingPathComponent("sessions/session-1/network-transfer.json"),
            atomically: true,
            encoding: .utf8
        )

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: false
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidTargetNetworkTransfer(".supermover/sessions/session-1/network-transfer.json")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenCurrentSourceProofSessionMismatchesTransferSession() throws {
        let bundle = try makeDirectory(named: "bundle-source-consistency-session-mismatch")
        let targetRoot = try makeDirectory(named: "target-root-source-consistency-session-mismatch")
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try writeBundleMeta(to: bundle)
        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeSourcePairFixtures(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(to: bundle.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "status": "pass",
          "mode": "current_source_verified",
          "session_id": "session-other",
          "detail": "mismatched session fixture"
        }
        """.write(to: bundle.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)
        try makeVerifyJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try makeReportJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try makeStatusJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try makeHealthJSON(targetRoot: targetRoot.path).write(to: bundle.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {"profile": "/tmp/source.profile.json", "status": "recorded", "machine_id": "source-machine", "machine_label": "source"},
            "target": {"profile": "/tmp/target.profile.json", "status": "recorded", "machine_id": "target-machine", "machine_label": "target"}
          },
          "evidence": {
            "source_pair": {"pairing_receipt_id":"pair-1","receipt_path":"exported-receipts/pair-1.json","target_address":"127.0.0.1:39395","output":"source.pair.json","pair":"source.pair.txt"},
            "source_transfer": {"session_id":"session-1","receiver_address":"127.0.0.1:9443","output":"source.transfer.json","verify":"source.verify.json","status":"source.status.json","report":"source.report.json","health":"source.health.json","push":"source.network-push.txt"},
            "source_consistency": {"output":"source.consistency.json","baseline":"source.baseline.json","status":"pass","mode":"current_source_verified"},
            "target_import": {"pairing_receipt_id":"pair-1","adopted":"target.adopt-pairing.txt"}
          }
        }
        """.write(to: bundle.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: false
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .blockedCurrentSourceConsistency("session_mismatch")
            )
        }
    }

    func testEvaluationCoordinatorExplicitNonCanonicalSourceArtifactsStayAlignedWithShellEvaluate() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-explicit-noncanonical-source-artifacts",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        let shellBundle = FileManager.default.temporaryDirectory.appendingPathComponent(
            "bundle-explicit-noncanonical-source-artifacts-shell-\(UUID().uuidString)",
            isDirectory: true
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
            try? FileManager.default.removeItem(at: shellBundle)
        }

        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try relocateSourceEvaluationArtifactsToNonCanonicalPaths(bundleRoot: bundle)
        try FileManager.default.copyItem(at: bundle, to: shellBundle)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: bundle)
        XCTAssertEqual(snapshot.sourcePairArtifact?.pairing_receipt_id, "pair-1")
        XCTAssertEqual(snapshot.sourceTransferArtifact?.session_id, "session-1")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.session_id, "session-1")

        XCTAssertNoThrow(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        )

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let shellResult = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", shellBundle.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
                "--require-operator-evidence",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(shellResult.exitCode, 0, "stderr:\n\(shellResult.stderr)")
        XCTAssertTrue(FileManager.default.fileExists(atPath: shellBundle.appendingPathComponent("evaluation.json").path))
    }

    func testShellEvaluateFailsClosedWhenCanonicalSourceProvenanceIsMalformed() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-malformed-canonical-source-provenance",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        try #"{"schema":"supermover.macos.provenance.v1","git_commit":""}"#.write(
            to: bundle.appendingPathComponent("source.provenance.json"),
            atomically: true,
            encoding: .utf8
        )

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundle.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertNotEqual(result.exitCode, 0)
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("evaluation.json").path))
    }

    func testEvaluationCoordinatorFailsClosedWhenSourceNotarizationAuthModeIsMissing() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-missing-source-notary-auth-mode",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try rewriteJSONObject(bundle.appendingPathComponent("source.notarization.json")) { root in
            root.removeValue(forKey: "auth_mode")
        }

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidNotarizationEvidence("source")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenSourceNotarizationHasFailureRecord() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-source-notary-failure-record",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try rewriteJSONObject(bundle.appendingPathComponent("source.notarization.json")) { root in
            root["failure"] = [
                "id": "notary_rejected",
                "message": "notarytool rejected the submission",
            ]
        }

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidNotarizationEvidence("source")
            )
        }
    }

    func testEvaluationCoordinatorFailsClosedWhenSourceNotarizationSubmissionIDIsMalformed() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-source-notary-malformed-submission-id",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "source.machine.json",
            machineID: "source-machine",
            machineLabel: "source"
        )
        try writeMachineFactsArtifact(
            bundleRoot: bundle,
            relativePath: "target.machine.json",
            machineID: "target-machine",
            machineLabel: "target"
        )
        try rewriteJSONObject(bundle.appendingPathComponent("source.notarization.json")) { root in
            var submission = try XCTUnwrap(root["submission"] as? [String: Any])
            submission["id"] = "manual-pass"
            root["submission"] = submission
        }

        XCTAssertThrowsError(
            try AcceptanceBundleEvaluationCoordinator().evaluate(
                bundleRootURL: bundle,
                targetRootURL: targetRoot,
                requireOperatorEvidence: true
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleEvaluationCoordinator.EvaluationError,
                .invalidNotarizationEvidence("source")
            )
        }
    }

    func testShellEvaluateFailsClosedWhenCanonicalSourceAppAuditArtifactIsSymlinkedOutsideBundle() throws {
        let (bundle, targetRoot) = try makeInstalledAppEvaluationFixture(
            named: "bundle-symlinked-canonical-source-app-audit",
            sourceMachineFactsOutput: "source.machine.json",
            targetMachineFactsOutput: "target.machine.json"
        )
        defer {
            try? FileManager.default.removeItem(at: bundle)
            try? FileManager.default.removeItem(at: targetRoot)
        }

        let outsideAuditURL = bundle.deletingLastPathComponent().appendingPathComponent(
            "outside-source.app-audit.json"
        )
        try writeReadyAppAudit(to: outsideAuditURL)
        try FileManager.default.removeItem(at: bundle.appendingPathComponent("source.app-audit.json"))
        do {
            try FileManager.default.createSymbolicLink(
                at: bundle.appendingPathComponent("source.app-audit.json"),
                withDestinationURL: outsideAuditURL
            )
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                repoRoot.appendingPathComponent("macos/script/acceptance-two-machine.sh").path,
                "evaluate",
                "--bundle-root", bundle.path,
                "--target-root", targetRoot.path,
                "--source-profile", "/tmp/source.profile.json",
            ],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertEqual(result.exitCode, 5)
        XCTAssertTrue(result.stderr.contains("bundle artifact path resolves through symlink: source.app-audit.json"))
        XCTAssertFalse(FileManager.default.fileExists(atPath: bundle.appendingPathComponent("evaluation.json").path))
    }

    private func makeInstalledAppEvaluationFixture(
        named name: String,
        sourceMachineFactsOutput: String,
        targetMachineFactsOutput: String
    ) throws -> (bundle: URL, targetRoot: URL) {
        let bundle = try makeDirectory(named: name)
        let targetRoot = try makeDirectory(named: "target-root-\(name)")

        try writeProvenance(to: bundle.appendingPathComponent("source.provenance.json"))
        try writeProvenance(to: bundle.appendingPathComponent("target.provenance.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("source.app-audit.json"))
        try writeReadyAppAudit(to: bundle.appendingPathComponent("target.app-audit.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("source.notarization.json"))
        try writeReleaseNotarization(to: bundle.appendingPathComponent("target.notarization.json"))
        try writeSourcePairFixtures(to: bundle)
        try writeTargetReadyFixture(to: bundle)
        try #"{"profile":"/tmp/source.profile.json","session_id":"session-1","target_address":"127.0.0.1:39395","receiver_address":"127.0.0.1:9443","target_mode":"pairing"}"#.write(
            to: bundle.appendingPathComponent("source.transfer.json"),
            atomically: true,
            encoding: .utf8
        )
        try "receipt adopted".write(to: bundle.appendingPathComponent("target.adopt-pairing.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: bundle.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try makeSourceBaselineJSON().write(
            to: bundle.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeCurrentSourceConsistencyJSON(status: "pass", mode: "current_source_verified").write(
            to: bundle.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeVerifyJSON(targetRoot: targetRoot.path).write(
            to: bundle.appendingPathComponent("source.verify.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeReportJSON(targetRoot: targetRoot.path).write(
            to: bundle.appendingPathComponent("source.report.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeStatusJSON(targetRoot: targetRoot.path).write(
            to: bundle.appendingPathComponent("source.status.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeHealthJSON(targetRoot: targetRoot.path).write(
            to: bundle.appendingPathComponent("source.health.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "source": "test-source",
          "listen": "127.0.0.1:9443",
          "candidate_count": 0,
          "invalid_packets": 0,
          "trusted": false,
          "candidates": []
        }
        """.write(
            to: bundle.appendingPathComponent("source.browse.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "status": "advertised",
          "listen": "127.0.0.1:39395",
          "destination": "239.255.255.250:1900",
          "service_type": "_supermover._udp",
          "protocol_version": "1",
          "ephemeral_nonce": "nonce-1",
          "capability_flags": ["pairing"],
          "trusted": false,
          "duration": "30s",
          "interval": "1s"
        }
        """.write(
            to: bundle.appendingPathComponent("target.advertise.json"),
            atomically: true,
            encoding: .utf8
        )
        try makeInstalledAppEvaluationMeta(
            sourceMachineFactsOutput: sourceMachineFactsOutput,
            targetMachineFactsOutput: targetMachineFactsOutput
        ).write(
            to: bundle.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
        try writeInstalledAppEvaluationControlPlane(to: targetRoot)

        return (bundle, targetRoot)
    }

    private func makeDirectory(named name: String) throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("\(name)-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }

    private var sourcePairReceiptRelativePath: String {
        "exported-receipts/pair-1.json"
    }

    private func writeSourcePairFixtures(to bundle: URL) throws {
        try """
        {
          "profile": "/tmp/source.profile.json",
          "target_address": "127.0.0.1:39395",
          "verification_code": "123456",
          "pairing_receipt_id": "pair-1",
          "receipt_path": "\(sourcePairReceiptRelativePath)"
        }
        """.write(
            to: bundle.appendingPathComponent("source.pair.json"),
            atomically: true,
            encoding: .utf8
        )
        let exportedReceipts = bundle.appendingPathComponent("exported-receipts", isDirectory: true)
        try FileManager.default.createDirectory(at: exportedReceipts, withIntermediateDirectories: true)
        try AcceptanceWorkflowFixtures.pairingReceiptJSON().write(
            to: bundle.appendingPathComponent(sourcePairReceiptRelativePath),
            atomically: true,
            encoding: .utf8
        )
        try "pair ok".write(
            to: bundle.appendingPathComponent("source.pair.txt"),
            atomically: true,
            encoding: .utf8
        )
    }

    private func writeTargetReadyFixture(to bundle: URL) throws {
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
        """.write(to: bundle.appendingPathComponent("target.ready.json"), atomically: true, encoding: .utf8)
    }

    private func writeBundleMeta(to dir: URL) throws {
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {}
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
    }

    private func writeProvenance(to url: URL) throws {
        try """
        {
          "schema": "supermover.macos.provenance.v1",
          "git_commit": "abcdef123456",
          "cli_version": "supermover 0.1.0-dev",
          "cli_relative_path": "Contents/Resources/bin/supermover",
          "build_profile": "test",
          "signing": "unsigned"
        }
        """.write(to: url, atomically: true, encoding: .utf8)
    }

    private func writeAppAudit(to url: URL) throws {
        try writeBlockedAppAudit(to: url)
    }

    private func writeBlockedAppAudit(to url: URL) throws {
        try """
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "blocked",
          "readiness": "blocked",
          "summary": {
            "pass_ready": false
          }
        }
        """.write(to: url, atomically: true, encoding: .utf8)
    }

    private func writeReadyAppAudit(to url: URL) throws {
        try """
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "pass",
          "readiness": "distribution_ready",
          "app_path": "/tmp/SuperMover.app",
          "provenance": {
            "path": "/tmp/SuperMover.app/Contents/Resources/supermover-provenance.json",
            "manifest": {
              "schema": "supermover.macos.provenance.v1",
              "git_commit": "abcdef123456",
              "cli_version": "supermover 0.1.0-dev",
              "cli_relative_path": "Contents/Resources/bin/supermover",
              "build_profile": "test",
              "signing": "unsigned"
            }
          },
          "summary": {
            "pass_ready": true
          }
        }
        """.write(to: url, atomically: true, encoding: .utf8)
    }

    private func writeDistributionReadyAppAudit(to url: URL) throws {
        try """
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "pass",
          "readiness": "distribution_ready",
          "app_path": "/tmp/SuperMover.app",
          "provenance": {
            "path": "/tmp/SuperMover.app/Contents/Resources/supermover-provenance.json",
            "manifest": {
              "schema": "supermover.macos.provenance.v1",
              "git_commit": "abcdef123456",
              "cli_version": "supermover 0.1.0-dev",
              "cli_relative_path": "Contents/Resources/bin/supermover",
              "build_profile": "test",
              "signing": "unsigned"
            }
          },
          "summary": {
            "pass_ready": true
          }
        }
        """.write(to: url, atomically: true, encoding: .utf8)
    }

    private func writeReleaseNotarization(to url: URL) throws {
        let machine = url.lastPathComponent.replacingOccurrences(of: ".notarization.json", with: "")
        let notaryLogName = "\(machine).notary-log.json"
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON().write(
            to: url.deletingLastPathComponent().appendingPathComponent(notaryLogName),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "schema": "supermover.macos.notarization.v1",
          "checked_at": "2026-06-01T12:00:00Z",
          "status": "pass",
          "app_path": "/tmp/SuperMover.app",
          "work_dir": "/tmp/SuperMover.app.notary",
          "auth_mode": "keychain_profile",
          "archive_path": "/tmp/SuperMover.app.notary/SuperMover.app.zip",
          "submission": {
            "id": "11111111-1111-1111-1111-111111111111",
            "status": "Accepted",
            "message": "Ready for distribution"
          },
          "notary_log": {
            "path": "\(notaryLogName)"
          },
          "audit": {
            "path": "/tmp/SuperMover.app.notary/post-staple.audit.json",
            "status": "pass",
            "readiness": "distribution_ready",
            "pass_ready": true,
            "blocking_checks": 0
          },
          "failure": null
        }
        """.write(to: url, atomically: true, encoding: .utf8)
    }

    private func makeCurrentSourceConsistencyJSON(status: String, mode: String) -> String {
        """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "status": "\(status)",
          "mode": "\(mode)",
          "session_id": "session-1",
          "detail": "test fixture"
        }
        """
    }

    private func makeSourceBaselineJSON() -> String {
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

    private func makeInstalledAppEvaluationMeta(
        sourceMachineFactsOutput: String,
        targetMachineFactsOutput: String
    ) -> String {
        AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        )
        .replacingOccurrences(
            of: "\"output\": \"source.machine.json\"",
            with: "\"output\": \"\(sourceMachineFactsOutput)\""
        )
        .replacingOccurrences(
            of: "\"output\": \"target.machine.json\"",
            with: "\"output\": \"\(targetMachineFactsOutput)\""
        )
    }

    private func relocateSourceEvaluationArtifactsToNonCanonicalPaths(bundleRoot: URL) throws {
        try moveBundleArtifact(
            bundleRoot: bundleRoot,
            from: "source.pair.json",
            to: "artifacts/source.pair.explicit.json"
        )
        try moveBundleArtifact(
            bundleRoot: bundleRoot,
            from: "source.pair.txt",
            to: "artifacts/source.pair.explicit.txt"
        )
        try moveBundleArtifact(
            bundleRoot: bundleRoot,
            from: "source.transfer.json",
            to: "artifacts/source.transfer.explicit.json"
        )
        try moveBundleArtifact(
            bundleRoot: bundleRoot,
            from: "source.network-push.txt",
            to: "artifacts/source.network-push.explicit.txt"
        )
        try moveBundleArtifact(
            bundleRoot: bundleRoot,
            from: "source.verify.json",
            to: "artifacts/source.verify.explicit.json"
        )
        try moveBundleArtifact(
            bundleRoot: bundleRoot,
            from: "source.status.json",
            to: "artifacts/source.status.explicit.json"
        )
        try moveBundleArtifact(
            bundleRoot: bundleRoot,
            from: "source.report.json",
            to: "artifacts/source.report.explicit.json"
        )
        try moveBundleArtifact(
            bundleRoot: bundleRoot,
            from: "source.health.json",
            to: "artifacts/source.health.explicit.json"
        )
        try moveBundleArtifact(
            bundleRoot: bundleRoot,
            from: "source.consistency.json",
            to: "artifacts/source.consistency.explicit.json"
        )
        try moveBundleArtifact(
            bundleRoot: bundleRoot,
            from: "source.baseline.json",
            to: "artifacts/source.baseline.explicit.json"
        )

        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let metaData = try Data(contentsOf: metaURL)
        var root = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        var evidence = try XCTUnwrap(root["evidence"] as? [String: Any])
        var sourcePair = try XCTUnwrap(evidence["source_pair"] as? [String: Any])
        var sourceTransfer = try XCTUnwrap(evidence["source_transfer"] as? [String: Any])
        var sourceConsistency = try XCTUnwrap(evidence["source_consistency"] as? [String: Any])

        sourcePair["output"] = "artifacts/source.pair.explicit.json"
        sourcePair["pair"] = "artifacts/source.pair.explicit.txt"
        sourceTransfer["output"] = "artifacts/source.transfer.explicit.json"
        sourceTransfer["push"] = "artifacts/source.network-push.explicit.txt"
        sourceTransfer["verify"] = "artifacts/source.verify.explicit.json"
        sourceTransfer["status"] = "artifacts/source.status.explicit.json"
        sourceTransfer["report"] = "artifacts/source.report.explicit.json"
        sourceTransfer["health"] = "artifacts/source.health.explicit.json"
        sourceConsistency["output"] = "artifacts/source.consistency.explicit.json"
        sourceConsistency["baseline"] = "artifacts/source.baseline.explicit.json"

        evidence["source_pair"] = sourcePair
        evidence["source_transfer"] = sourceTransfer
        evidence["source_consistency"] = sourceConsistency
        root["evidence"] = evidence

        let updatedMeta = try JSONSerialization.data(withJSONObject: root, options: [.prettyPrinted, .sortedKeys])
        try updatedMeta.write(to: metaURL)
    }

    private func setOperatorEvidenceDetail(
        kind: String,
        detail: String,
        bundleRoot: URL
    ) throws {
        let metaURL = bundleRoot.appendingPathComponent("meta.json")
        let metaData = try Data(contentsOf: metaURL)
        var root = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        var evidence = try XCTUnwrap(root["evidence"] as? [String: Any])
        var operatorEvidence = try XCTUnwrap(evidence["operator"] as? [String: Any])
        var record = try XCTUnwrap(operatorEvidence[kind] as? [String: Any])
        record["detail"] = detail
        operatorEvidence[kind] = record
        evidence["operator"] = operatorEvidence
        root["evidence"] = evidence

        let updatedMeta = try JSONSerialization.data(withJSONObject: root, options: [.prettyPrinted, .sortedKeys])
        try updatedMeta.write(to: metaURL)
    }

    private func rewriteJSONObject(
        _ url: URL,
        transform: (inout [String: Any]) throws -> Void
    ) throws {
        let data = try Data(contentsOf: url)
        var root = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        try transform(&root)
        let updated = try JSONSerialization.data(withJSONObject: root, options: [.prettyPrinted, .sortedKeys])
        try updated.write(to: url)
    }

    private func moveBundleArtifact(
        bundleRoot: URL,
        from sourceRelativePath: String,
        to destinationRelativePath: String
    ) throws {
        let sourceURL = bundleRoot.appendingPathComponent(sourceRelativePath)
        let destinationURL = bundleRoot.appendingPathComponent(destinationRelativePath)
        try FileManager.default.createDirectory(
            at: destinationURL.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
        try FileManager.default.moveItem(at: sourceURL, to: destinationURL)
    }

    private func writeMachineFactsArtifact(
        bundleRoot: URL,
        relativePath: String,
        machineID: String,
        machineLabel: String
    ) throws {
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "\(machineID)",
          "machine_label": "\(machineLabel)"
        }
        """.write(
            to: bundleRoot.appendingPathComponent(relativePath),
            atomically: true,
            encoding: .utf8
        )
    }

    private func writeInstalledAppEvaluationControlPlane(to targetRoot: URL) throws {
        let controlPlane = targetRoot.appendingPathComponent(".supermover", isDirectory: true)
        try FileManager.default.createDirectory(
            at: controlPlane.appendingPathComponent("pairings"),
            withIntermediateDirectories: true
        )
        try FileManager.default.createDirectory(
            at: controlPlane.appendingPathComponent("sessions/session-1"),
            withIntermediateDirectories: true
        )
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
    }
}
