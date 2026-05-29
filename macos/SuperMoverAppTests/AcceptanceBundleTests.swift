import XCTest
@testable import SuperMoverApp

final class AcceptanceBundleTests: XCTestCase {
    func testAcceptanceBundleReaderLoadsEvidenceCollectedBundle() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "evidence_collected",
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
                "collected_by": "source-transfer",
                "output": "source.app-audit.json",
                "exit_code": 1,
                "status": "blocked",
                "readiness": "blocked",
                "pass_ready": false,
                "blocking_checks": 11
              }
            },
            "notarization": {
              "source": {
                "collected_by": "release-notary",
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
                "archive": "source-to-target.tgz",
                "manifest": "source-to-target.manifest.json",
                "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
                "meta": "meta.json",
                "verified": true
              }
            ],
            "discovery": {
              "source_browse": {
                "output": "source.browse.json",
                "trusted": false
              }
            },
            "target_serve_phases": [
              {"phase": 1, "ready": "target.ready.phase-1.json"}
            ],
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
            "evaluation": {
              "pairing_receipt_id": "pair-1",
              "session_id": "session-1",
              "target_root": "/tmp/target-root",
              "output": "evaluation.json",
              "require_operator_evidence": true
            },
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "prompt accepted",
                "machine_id": "target-machine",
                "machine_label": "target"
              },
              "firewall": {
                "status": "blocked",
                "detail": "prompt dismissed"
              }
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try """
        {
          "source": "lan_datagram",
          "listen": "0.0.0.0:39394",
          "candidate_count": 1,
          "invalid_packets": 0,
          "trusted": false,
          "candidates": []
        }
        """.write(to: dir.appendingPathComponent("source.browse.json"), atomically: true, encoding: .utf8)
        try """
        {
          "address": "127.0.0.1:39395",
          "verification_code": "123456",
          "mode": "pairing-only",
          "trusted": false,
          "transfer": false
        }
        """.write(to: dir.appendingPathComponent("target.ready.phase-1.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: dir.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: dir.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "profile": "/tmp/source.profile.json",
          "target_address": "127.0.0.1:39395",
          "verification_code": "123456",
          "pairing_receipt_id": "pair-1",
          "receipt_path": "exported-receipts/pair-1.json"
        }
        """.write(to: dir.appendingPathComponent("source.pair.json"), atomically: true, encoding: .utf8)
        try FileManager.default.createDirectory(
            at: dir.appendingPathComponent("exported-receipts"),
            withIntermediateDirectories: true
        )
        try AcceptanceWorkflowFixtures.pairingReceiptJSON().write(
            to: dir.appendingPathComponent("exported-receipts/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "profile": "/tmp/source.profile.json",
          "session_id": "session-1",
          "target_address": "127.0.0.1:39395",
          "receiver_address": "127.0.0.1:9443",
          "target_mode": "pairing"
        }
        """.write(to: dir.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "status": "pass",
          "mode": "current_source_verified",
          "session_id": "session-1",
          "entry_count": 3,
          "mismatch_count": 0,
          "detail": "Current source tree matches the transfer baseline used for the network push session."
        }
        """.write(to: dir.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-1",
          "root_id": "root-1",
          "root_path": "/tmp/source",
          "session_id": "session-1",
          "created_at": "2026-01-01T00:00:00Z",
          "entries": []
        }
        """.write(to: dir.appendingPathComponent("source.baseline.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.macos.notarization.v1",
          "checked_at": "2026-06-01T12:00:00Z",
          "status": "pass",
          "app_path": "/tmp/SuperMover.app",
          "work_dir": "/tmp/notary",
          "auth_mode": "keychain_profile",
          "archive_path": "/tmp/notary/SuperMover.app.zip",
          "submission": {
            "id": "11111111-1111-1111-1111-111111111111",
            "status": "Accepted",
            "message": "Ready for distribution"
          },
          "notary_log": {
            "path": "/tmp/notary/notary-log.json"
          },
          "audit": {
            "path": "/tmp/notary/post-staple.audit.json",
            "status": "pass",
            "readiness": "distribution_ready",
            "pass_ready": true,
            "blocking_checks": 0
          },
          "failure": null
        }
        """.write(to: dir.appendingPathComponent("source.notarization.json"), atomically: true, encoding: .utf8)
        try "pair ok".write(to: dir.appendingPathComponent("source.pair.txt"), atomically: true, encoding: .utf8)
        try "push ok".write(to: dir.appendingPathComponent("source.network-push.txt"), atomically: true, encoding: .utf8)
        try """
        {
          "target_root": "/tmp/target-root",
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
        """.write(to: dir.appendingPathComponent("source.verify.json"), atomically: true, encoding: .utf8)
        try """
        {
          "profile_id": "profile-1",
          "target_id": "target-1",
          "target_root": "/tmp/target-root",
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
        """.write(to: dir.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)
        try """
        {
          "target_root": "/tmp/target-root",
          "overall": {"status":"ok","issues":[]},
          "summary": {"warnings":0,"soft_deletes":0,"target_drifts":0,"live_target_drifts":0,"prune_candidates":0,"prune_refusals":0,"prune_approvals":0,"network_transfers":1,"artifact_problems":0},
          "latest_session": {"id":"session-1","manifest_id":"m1","created_at":"2026-01-01T00:00:00Z","entries":1,"files":1,"completeness":{"status":"verified","files_expected":1,"files_verified":1,"verification_errors":0,"verification_warnings":0}},
          "prune_review": {"status":"clear","approval_required":false,"apply":"none","summary":{"candidates":0,"refusals":0,"approvals":0,"unapplied_approvals":0,"receipt_issues":0}},
          "pairing": {"status":"paired_receipt_valid","receipt_id":"pair-1","target_device_id":"dst-spki","paired_at":"2026-01-01T00:00:00Z","method":"verification_code","verified_at":"2026-01-01T00:00:00Z","evidence":"receipt","receipt_source":"target_control","receipt_path":"p","source_receipt_path":"s","target_receipt_path":"t","encrypted_transfer":"required"},
          "privacy": {"status":"review","claim":"bounded","network_transfer":"published"},
          "health": {"healthy":true,"summary":{"incomplete_sessions":0,"invalid_records":0,"artifact_problems":0,"target_drifts":0,"network_transfers":1}}
        }
        """.write(to: dir.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)
        try """
        {
          "target_root": "/tmp/target-root",
          "healthy": true,
          "summary": {"incomplete_sessions":0,"invalid_records":0,"artifact_problems":0,"target_drifts":0,"network_transfers":1},
          "items": [],
          "invalid": [],
          "artifacts": [],
          "network_transfers": [{"session_id":"session-1","status":"published","stage":"commit","action":"preserved"}]
        }
        """.write(to: dir.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "evidence_collected",
          "pairing_receipt_id": "pair-1",
          "session_id": "session-1",
          "target_root": "/tmp/target-root"
        }
        """.write(to: dir.appendingPathComponent("evaluation.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)

        XCTAssertTrue(snapshot.isCollected)
        XCTAssertEqual(snapshot.meta.evidence.source_pair?.output, "source.pair.json")
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.output, "source.transfer.json")
        XCTAssertEqual(snapshot.meta.evidence.source_transfer?.push, "source.network-push.txt")
        XCTAssertEqual(snapshot.meta.evidence.evaluation?.output, "evaluation.json")
        XCTAssertFalse(snapshot.discoveryTrustedUnexpected)
        XCTAssertTrue(snapshot.hasBlockedAppAudit)
        XCTAssertEqual(snapshot.sourceBrowseSnapshot?.candidate_count, 1)
        XCTAssertEqual(snapshot.sourcePairArtifact?.pairing_receipt_id, "pair-1")
        XCTAssertEqual(snapshot.sourceTransferArtifact?.receiver_address, "127.0.0.1:9443")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.baseline, "source.baseline.json")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.session_id, "session-1")
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.entry_count, 3)
        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.detail, "Current source tree matches the transfer baseline used for the network push session.")
        XCTAssertEqual(snapshot.sourceNotarization?.output, "source.notarization.json")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.status, "pass")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.audit?.readiness, "distribution_ready")
        XCTAssertEqual(snapshot.sourceNotarizationArtifact?.submission?.id, "11111111-1111-1111-1111-111111111111")
        XCTAssertEqual(snapshot.bundleHandoffs.count, 1)
        XCTAssertEqual(snapshot.bundleHandoffs.first?.archive, "source-to-target.tgz")
        XCTAssertEqual(snapshot.bundleHandoffs.first?.manifest, "source-to-target.manifest.json")
        XCTAssertEqual(snapshot.bundleHandoffs.first?.verified, true)
        XCTAssertEqual(snapshot.evaluationArtifact?.target_root, "/tmp/target-root")
        XCTAssertEqual(snapshot.targetServePhaseArtifacts.first?.readiness.mode, "pairing-only")
        XCTAssertTrue(snapshot.issues.isEmpty)
        XCTAssertTrue(snapshot.persistedEvaluationRequiresOperatorEvidence)
        XCTAssertEqual(snapshot.collectionMode, "two_machine")
        XCTAssertEqual(snapshot.machineCount, 2)
        XCTAssertEqual(snapshot.meta.roles["source_pair"]?.machine_id, "source-machine")
        XCTAssertEqual(snapshot.meta.roles["target"]?.machine_id, "target-machine")
        XCTAssertEqual(snapshot.operatorEvidence["local_network"]?.status, "pass")
        XCTAssertEqual(snapshot.operatorEvidence["firewall"]?.status, "blocked")
        XCTAssertEqual(Set(snapshot.missingOperatorEvidence), Set(["firewall", "pairing_confirmation"]))
    }

    func testAcceptanceBundleReaderRejectsMalformedMeta() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        try "{".write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        XCTAssertThrowsError(try AcceptanceBundleReader().read(bundleRootURL: dir)) { error in
            XCTAssertEqual(error as? AcceptanceBundleReader.ReadError, .malformedMeta(dir.appendingPathComponent("meta.json")))
        }
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryStartsWithTargetServe() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {},
          "evidence": {}
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(summary.nextActions.first?.machine, "target")
        XCTAssertEqual(summary.nextActions.first?.step, "target_serve_phase_1")
        XCTAssertEqual(
            summary.nextActions.first?.commands.first,
            "sh macos/script/acceptance-two-machine.sh target-serve --profile '<target-profile>' --bundle-root '<bundle-root>'"
        )
    }

    func testAcceptanceBundleReaderLoadsWorkflowSummaryArtifact() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

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
            "workflow_summary": {
              "output": "workflow.summary.json"
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try """
        {
          "schema": "supermover.acceptance.workflow_summary.v1",
          "default": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "next_actions": [
              {
                "machine": "target",
                "step": "target_serve_phase_1",
                "action": "start target pairing serve",
                "commands": [
                  "sh macos/script/acceptance-two-machine.sh target-serve --profile '<target-profile>' --bundle-root '<bundle-root>'"
                ]
              }
            ],
            "steps": []
          },
          "require_operator_evidence": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "next_actions": [],
            "steps": []
          }
        }
        """.write(to: dir.appendingPathComponent("workflow.summary.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)

        XCTAssertEqual(snapshot.workflowSummaryArtifact?.schema, "supermover.acceptance.workflow_summary.v1")
        XCTAssertEqual(snapshot.workflowSummaryArtifact?.default?.next_actions.first?.step, "target_serve_phase_1")
        XCTAssertEqual(snapshot.workflowSummaryArtifact?.require_operator_evidence?.schema, "supermover.acceptance.workflow_status.v1")
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryRequiresMachineIdentityRewriteBeforeHandoff() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

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
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "discovery": {
              "source_browse": {
                "output": "source.browse.json",
                "trusted": false
              },
              "target_advertise": {
                "output": "target.advertise.json",
                "trusted": false
              }
            },
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched"
              }
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: dir,
            machine: "source",
            appPath: "/tmp/current-source/SuperMover.app"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: dir,
            machine: "target",
            appPath: "/tmp/current-target/SuperMover.app"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(summary.nextActions.map(\.machine), ["target", "source"])
        XCTAssertEqual(summary.nextActions.map(\.step), ["target_serve_phase_1", "source_pair"])
        XCTAssertTrue(summary.nextActions[0].action.contains("target.machine.json"))
        XCTAssertTrue(summary.nextActions[1].action.contains("source.machine.json"))
        XCTAssertTrue(summary.nextActions[0].commands.first?.contains("target-serve") == true)
        XCTAssertTrue(summary.nextActions[1].commands.first?.contains("source-pair") == true)
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryAdvancesToEvaluateWithoutOperatorEvidenceWhenInstalledAppProofIsMissing() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

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
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "discovery": {
              "source_browse": {
                "output": "source.browse.json",
                "trusted": false
              },
              "target_advertise": {
                "output": "target.advertise.json",
                "trusted": false
              }
            },
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(summary.nextActions.count, 1)
        XCTAssertEqual(summary.nextActions.first?.machine, "either")
        XCTAssertEqual(summary.nextActions.first?.step, "evaluate")
        XCTAssertEqual(
            summary.nextActions.first?.commands.first,
            "sh macos/script/acceptance-two-machine.sh evaluate --bundle-root '<bundle-root>' --target-root '<target-root>' --source-profile '/tmp/source.profile.json'"
        )
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMissing() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

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
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "discovery": {
              "source_browse": {
                "output": "source.browse.json",
                "trusted": false
              },
              "target_advertise": {
                "output": "target.advertise.json",
                "trusted": false
              }
            },
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try FileManager.default.removeItem(
            at: dir.appendingPathComponent("exported-receipts/pair-1.json")
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertFalse(snapshot.hasSourcePairReceiptArtifact)
        XCTAssertEqual(
            snapshot.issues.first(where: { $0.artifact == "exported-receipts/pair-1.json" })?.problem,
            "missing or unreadable artifact"
        )
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_pair" })?.done, false)
        XCTAssertEqual(summary.steps.first(where: { $0.id == "target_import" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_pair", "target_import"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMissing() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try FileManager.default.removeItem(at: dir.appendingPathComponent("target.ready.json"))

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(
            snapshot.issues.first(where: { $0.artifact == "target.ready.json" })?.problem,
            "missing or unreadable artifact"
        )
        XCTAssertEqual(summary.steps.first(where: { $0.id == "target_serve_phase_1" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["target_serve_phase_1"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetReadyArtifactIsMalformed() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try #"{"address":"127.0.0.1:39395","mode":"pairing"}"#.write(
            to: dir.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(
            snapshot.issues.first(where: { $0.artifact == "target.ready.json" })?.problem,
            "malformed JSON artifact"
        )
        XCTAssertEqual(summary.steps.first(where: { $0.id == "target_serve_phase_1" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["target_serve_phase_1"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenReferencedTargetImportArtifactIsMissing() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try FileManager.default.removeItem(at: dir.appendingPathComponent("target.adopt-pairing.txt"))

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(
            snapshot.issues.first(where: { $0.artifact == "target.adopt-pairing.txt" })?.problem,
            "missing or unreadable artifact"
        )
        XCTAssertEqual(summary.steps.first(where: { $0.id == "target_import" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["target_import"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenTargetImportAdoptedTranscriptIsMissing() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        let metaData = Data(AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).utf8)
        var root = try XCTUnwrap(JSONSerialization.jsonObject(with: metaData) as? [String: Any])
        var evidence = try XCTUnwrap(root["evidence"] as? [String: Any])
        var targetImport = try XCTUnwrap(evidence["target_import"] as? [String: Any])
        targetImport.removeValue(forKey: "adopted")
        evidence["target_import"] = targetImport
        root["evidence"] = evidence
        let updatedMeta = try JSONSerialization.data(withJSONObject: root, options: [.prettyPrinted, .sortedKeys])
        try updatedMeta.write(to: dir.appendingPathComponent("meta.json"), options: .atomic)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(summary.steps.first(where: { $0.id == "target_import" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["target_import"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairTargetAddressMismatchesTargetReady() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "profile": "/tmp/source.profile.json",
            "target_address": "127.0.0.1:49999",
            "verification_code": "123456",
            "pairing_receipt_id": "pair-1",
            "receipt_path": "exported-receipts/pair-1.json",
        ]).write(to: dir.appendingPathComponent("source.pair.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_pair" })?.done, false)
        XCTAssertEqual(summary.steps.first(where: { $0.id == "target_import" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_pair", "target_import"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferReceiverMismatchesTargetReady() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "profile": "/tmp/source.profile.json",
            "session_id": "session-1",
            "target_address": "127.0.0.1:39395",
            "receiver_address": "127.0.0.1:9555",
            "target_mode": "pairing",
        ]).write(to: dir.appendingPathComponent("source.transfer.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_pair" })?.done, true)
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferLacksReceiverReadyTargetArtifact() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "address": "127.0.0.1:39395",
            "verification_code": "123456",
            "mode": "pairing",
            "receiver_routes": false,
            "push_network": false,
            "trusted": false,
            "transfer": false,
        ]).write(to: dir.appendingPathComponent("target.ready.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(summary.steps.first(where: { $0.id == "target_serve_phase_1" })?.done, true)
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_pair" })?.done, true)
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsDirectory() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

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
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "discovery": {
              "source_browse": {
                "output": "source.browse.json",
                "trusted": false
              },
              "target_advertise": {
                "output": "target.advertise.json",
                "trusted": false
              }
            },
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try FileManager.default.removeItem(
            at: dir.appendingPathComponent("exported-receipts/pair-1.json")
        )
        try FileManager.default.createDirectory(
            at: dir.appendingPathComponent("exported-receipts/pair-1.json"),
            withIntermediateDirectories: true
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertFalse(snapshot.hasSourcePairReceiptArtifact)
        XCTAssertEqual(
            snapshot.issues.first(where: { $0.artifact == "exported-receipts/pair-1.json" })?.problem,
            "missing or unreadable artifact"
        )
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_pair" })?.done, false)
        XCTAssertEqual(summary.steps.first(where: { $0.id == "target_import" })?.done, false)
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairReceiptArtifactIsMalformed() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

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
              "machine_id": "source-machine"
            },
            "target": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "target-machine"
            }
          },
          "evidence": {
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json"
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try "{}".write(
            to: dir.appendingPathComponent("exported-receipts/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertTrue(snapshot.hasSourcePairReceiptArtifact)
        XCTAssertFalse(snapshot.hasValidSourcePairReceiptArtifact)
        XCTAssertEqual(
            snapshot.issues.first(where: { $0.artifact == "exported-receipts/pair-1.json" })?.problem,
            "invalid pairing receipt artifact"
        )
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_pair" })?.done, false)
        XCTAssertEqual(summary.steps.first(where: { $0.id == "target_import" })?.done, false)
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceReportReceiptMismatchesSourcePair() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try AcceptanceWorkflowFixtures.reportJSON(targetRoot: targetRoot.path)
            .replacingOccurrences(of: "\"receipt_id\":\"pair-1\"", with: "\"receipt_id\":\"pair-stale\"")
            .write(to: dir.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(snapshot.sourceReportArtifact?.pairing.receipt_id, "pair-stale")
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceReportReceiptHasWhitespace() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try AcceptanceWorkflowFixtures.reportJSON(targetRoot: targetRoot.path)
            .replacingOccurrences(of: "\"receipt_id\":\"pair-1\"", with: "\"receipt_id\":\"pair-1 \"")
            .write(to: dir.appendingPathComponent("source.report.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(snapshot.sourceReportArtifact?.pairing.receipt_id, "pair-1 ")
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceStatusSessionMismatchesTransfer() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try AcceptanceWorkflowFixtures.statusJSON(targetRoot: targetRoot.path)
            .replacingOccurrences(of: #""id":"session-1""#, with: #""id":"session-stale""#)
            .write(to: dir.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(snapshot.sourceStatusArtifact?.latest_session.id, "session-stale")
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceHealthLacksTransferSession() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try AcceptanceWorkflowFixtures.healthJSON(targetRoot: targetRoot.path)
            .replacingOccurrences(of: #""session_id":"session-1""#, with: #""session_id":"session-stale""#)
            .write(to: dir.appendingPathComponent("source.health.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(snapshot.sourceHealthArtifact?.network_transfers?.first?.session_id, "session-stale")
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferTargetRootsDisagree() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let otherTargetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: otherTargetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }
        defer { try? FileManager.default.removeItem(at: otherTargetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try AcceptanceWorkflowFixtures.statusJSON(targetRoot: otherTargetRoot.path)
            .write(to: dir.appendingPathComponent("source.status.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(snapshot.sourceVerifyArtifact?.target_root, targetRoot.path)
        XCTAssertEqual(snapshot.sourceStatusArtifact?.target_root, otherTargetRoot.path)
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenEvaluatedSourceTransferTargetRootsChange() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let otherTargetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: otherTargetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }
        defer { try? FileManager.default.removeItem(at: otherTargetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true,
            status: "evidence_collected"
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try rewriteJSONObject(dir.appendingPathComponent("meta.json")) { root in
            var evidence = try XCTUnwrap(root["evidence"] as? [String: Any])
            evidence["evaluation"] = [
                "pairing_receipt_id": "pair-1",
                "session_id": "session-1",
                "target_root": targetRoot.path,
                "output": "evaluation.json",
                "require_operator_evidence": false,
            ]
            root["evidence"] = evidence
        }
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "evidence_collected",
          "pairing_receipt_id": "pair-1",
          "session_id": "session-1",
          "target_root": "\(targetRoot.path)"
        }
        """.write(to: dir.appendingPathComponent("evaluation.json"), atomically: true, encoding: .utf8)
        for artifact in [
            "source.verify.json",
            "source.report.json",
            "source.status.json",
            "source.health.json",
        ] {
            try rewriteJSONObject(dir.appendingPathComponent(artifact)) { root in
                root["target_root"] = otherTargetRoot.path
            }
        }

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(snapshot.evaluationArtifact?.target_root, targetRoot.path)
        XCTAssertEqual(snapshot.sourceVerifyArtifact?.target_root, otherTargetRoot.path)
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
        XCTAssertEqual(snapshot.hasCurrentEvaluationPassState(requireOperatorEvidence: false), false)
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceConsistencySessionHasWhitespace() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "status": "pass",
          "mode": "current_source_verified",
          "session_id": "session-1 ",
          "detail": "test fixture"
        }
        """.write(to: dir.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.session_id, "session-1 ")
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourcePairIDIsUnsafeButTransferArtifactsLookReady() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try """
        {
          "profile": "/tmp/source.profile.json",
          "target_address": "127.0.0.1:39395",
          "verification_code": "123456",
          "pairing_receipt_id": "../pair-escape",
          "receipt_path": "exported-receipts/pair-1.json"
        }
        """.write(to: dir.appendingPathComponent("source.pair.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_pair" })?.done, false)
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_pair", "target_import", "source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryUsesSourceConsistencyArtifactBaselineBeforeMeta() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "baseline": "artifact.baseline.json",
          "status": "pass",
          "mode": "current_source_verified",
          "session_id": "session-1",
          "detail": "test fixture"
        }
        """.write(to: dir.appendingPathComponent("source.consistency.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(snapshot.sourceConsistencyArtifact?.baseline, "artifact.baseline.json")
        XCTAssertFalse(snapshot.hasSourceConsistencyBaselineArtifact)
        XCTAssertEqual(
            snapshot.issues.first(where: { $0.artifact == "artifact.baseline.json" })?.problem,
            "missing or unreadable artifact"
        )
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceTransferTranscriptIsDirectory() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try FileManager.default.removeItem(at: dir.appendingPathComponent("source.network-push.txt"))
        try FileManager.default.createDirectory(
            at: dir.appendingPathComponent("source.network-push.txt"),
            withIntermediateDirectories: true
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertFalse(snapshot.hasSourceNetworkPushTranscriptArtifact)
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryFailsClosedWhenSourceConsistencyBaselineIsDirectory() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        let targetRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        defer { try? FileManager.default.removeItem(at: targetRoot) }

        try AcceptanceInstalledAppBundleFixtures.bundleMeta(
            includeReleaseEvidenceMeta: false,
            includeWorkflowSummaryArtifact: false,
            includeEvaluateArtifactPaths: true
        ).write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)
        try AcceptanceWorkflowFixtures.writeReadyBundleArtifacts(bundleRoot: dir, targetRoot: targetRoot)
        try FileManager.default.removeItem(at: dir.appendingPathComponent("source.baseline.json"))
        try FileManager.default.createDirectory(
            at: dir.appendingPathComponent("source.baseline.json"),
            withIntermediateDirectories: true
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertFalse(snapshot.hasSourceConsistencyBaselineArtifact)
        XCTAssertEqual(
            snapshot.issues.first(where: { $0.artifact == "source.baseline.json" })?.problem,
            "missing or unreadable artifact"
        )
        XCTAssertEqual(summary.steps.first(where: { $0.id == "source_transfer" })?.done, false)
        XCTAssertEqual(summary.nextActions.map(\.step), ["source_transfer"])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryRequiresMachineIdentityRewriteWhenMachineFactArtifactsDisagreeWithMeta() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

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
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "discovery": {
              "source_browse": {
                "output": "source.browse.json",
                "trusted": false
              },
              "target_advertise": {
                "output": "target.advertise.json",
                "trusted": false
              }
            },
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
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
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched"
              }
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "other-source-machine",
          "machine_label": "other-source"
        }
        """.write(to: dir.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "other-target-machine",
          "machine_label": "other-target"
        }
        """.write(to: dir.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: dir,
            machine: "source",
            appPath: "/tmp/current-source/SuperMover.app"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: dir,
            machine: "target",
            appPath: "/tmp/current-target/SuperMover.app"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertNil(snapshot.installedAppMachinePairProof)
        XCTAssertNil(snapshot.verifiedInstalledAppMachinePairHandoff)
        XCTAssertEqual(summary.nextActions.map(\.machine), ["target", "source"])
        XCTAssertEqual(summary.nextActions.map(\.step), ["target_serve_phase_1", "source_pair"])
        XCTAssertTrue(summary.nextActions[0].commands.first?.contains("target-serve") == true)
        XCTAssertTrue(summary.nextActions[1].commands.first?.contains("source-pair") == true)
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryRequiresMachineIdentityRewriteWhenMachineFactArtifactSchemaIsInvalid() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

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
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "discovery": {
              "source_browse": {
                "output": "source.browse.json",
                "trusted": false
              },
              "target_advertise": {
                "output": "target.advertise.json",
                "trusted": false
              }
            },
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
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
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched"
              }
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try """
        {
          "schema": "supermover.acceptance.machine_facts.v0",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: dir.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: dir.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: dir,
            machine: "source",
            appPath: "/tmp/current-source/SuperMover.app"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: dir,
            machine: "target",
            appPath: "/tmp/current-target/SuperMover.app"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertNil(snapshot.installedAppMachinePairProof)
        XCTAssertNil(snapshot.verifiedInstalledAppMachinePairHandoff)
        XCTAssertEqual(summary.nextActions.map(\.machine), ["target", "source"])
        XCTAssertEqual(summary.nextActions.map(\.step), ["target_serve_phase_1", "source_pair"])
        XCTAssertTrue(summary.nextActions[0].commands.first?.contains("target-serve") == true)
        XCTAssertTrue(summary.nextActions[1].commands.first?.contains("source-pair") == true)
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryBlocksWhenVerifiedBundleHandoffsContainContradictoryMachinePairs() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

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
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "discovery": {
              "source_browse": {
                "output": "source.browse.json",
                "trusted": false
              },
              "target_advertise": {
                "output": "target.advertise.json",
                "trusted": false
              }
            },
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "bundle_handoffs": [
              {
                "archive": "bundle-good.tgz",
                "manifest": "bundle-good.manifest.json",
                "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
                "meta": "meta.json",
                "verified": true,
                "exporting_machine_id": "source-machine",
                "exporting_machine_label": "source",
                "importing_machine_id": "target-machine",
                "importing_machine_label": "target"
              },
              {
                "archive": "bundle-bad.tgz",
                "manifest": "bundle-bad.manifest.json",
                "sha256": "2222222222222222222222222222222222222222222222222222222222222222",
                "meta": "meta.json",
                "verified": true,
                "exporting_machine_id": "other-source-machine",
                "exporting_machine_label": "other-source",
                "importing_machine_id": "other-target-machine",
                "importing_machine_label": "other-target"
              }
            ],
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched"
              }
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: dir.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: dir.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: dir,
            machine: "source",
            appPath: "/tmp/current-source/SuperMover.app"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: dir,
            machine: "target",
            appPath: "/tmp/current-target/SuperMover.app"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertEqual(snapshot.verifiedCrossMachineBundleHandoffs.count, 1)
        XCTAssertNil(snapshot.verifiedInstalledAppMachinePairHandoff)
        XCTAssertFalse(snapshot.installedAppCollectionProof.ok)
        XCTAssertTrue(snapshot.installedAppCollectionProof.failures.contains("contradictory_verified_bundle_handoffs"))
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.failureMessage,
            "bundle_handoffs contain verified cross-machine archive handoff evidence for machine ids other than the recorded source/target pair"
        )
        XCTAssertEqual(summary.nextActions.count, 1)
        XCTAssertEqual(summary.nextActions.first?.machine, "either")
        XCTAssertEqual(summary.nextActions.first?.step, "review_bundle_handoff")
        XCTAssertTrue(summary.nextActions.first?.action.contains("contradictory") == true)
        XCTAssertEqual(summary.nextActions.first?.commands, [])
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryRequestsBundleHandoffWhenVerifiedHandoffDoesNotMatchRecordedPair() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

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
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "discovery": {
              "source_browse": {
                "output": "source.browse.json",
                "trusted": false
              },
              "target_advertise": {
                "output": "target.advertise.json",
                "trusted": false
              }
            },
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json",
              "pair": "source.pair.txt"
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443",
              "output": "source.transfer.json"
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "bundle_handoffs": [
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
            ],
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched"
              }
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "source-machine",
          "machine_label": "source"
        }
        """.write(to: dir.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "target-machine",
          "machine_label": "target"
        }
        """.write(to: dir.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: dir,
            machine: "source",
            appPath: "/tmp/current-source/SuperMover.app"
        )
        try AcceptanceReleaseEvidenceFixtures.writeCurrentBundleReleaseEvidence(
            bundleRoot: dir,
            machine: "target",
            appPath: "/tmp/current-target/SuperMover.app"
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: true)

        XCTAssertTrue(snapshot.verifiedCrossMachineBundleHandoffs.isEmpty)
        XCTAssertNil(snapshot.verifiedInstalledAppMachinePairHandoff)
        XCTAssertFalse(snapshot.installedAppCollectionProof.ok)
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.failures,
            ["handoff_does_not_match_recorded_machine_pair"]
        )
        XCTAssertNil(snapshot.installedAppCollectionProof.blockedReason)
        XCTAssertTrue(snapshot.installedAppCollectionProof.requiresBundleHandoffProof)
        XCTAssertEqual(
            snapshot.installedAppCollectionProof.failureMessage,
            "bundle_handoffs do not prove a verified cross-machine archive handoff between the recorded source/target machine ids"
        )
        XCTAssertEqual(summary.nextActions.count, 1)
        XCTAssertEqual(summary.nextActions.first?.machine, "either")
        XCTAssertEqual(summary.nextActions.first?.step, "bundle_handoff")
        XCTAssertTrue(summary.nextActions.first?.action.contains("pack/unpack/merge") == true)
        XCTAssertTrue(summary.nextActions.first?.commands.first?.contains("pack-bundle") == true)
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryIgnoresStaleArtifactWhenTwoMachineProofFieldsAreMissing() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

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
            "workflow_summary": {
              "output": "workflow.summary.json"
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        try """
        {
          "schema": "supermover.acceptance.workflow_summary.v1",
          "default": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "next_actions": [
              {
                "machine": "either",
                "step": "bundle_handoff",
                "action": "pack/unpack/merge bundle evidence across machines before installed-app evaluation",
                "commands": [
                  "sh macos/script/acceptance-two-machine.sh pack-bundle --bundle-root '<bundle-root>' --archive '<bundle.tgz>'"
                ]
              }
            ],
            "steps": [
              {
                "id": "target_serve_phase_1",
                "machine": "target",
                "description": "start target pairing serve",
                "done": true
              }
            ]
          },
          "require_operator_evidence": {
            "schema": "supermover.acceptance.workflow_status.v1",
            "next_actions": [],
            "steps": []
          }
        }
        """.write(to: dir.appendingPathComponent("workflow.summary.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertEqual(summary.nextActions.first?.step, "target_serve_phase_1")
        XCTAssertEqual(summary.nextActions.first?.machine, "target")
        XCTAssertEqual(summary.steps.first?.id, "target_serve_phase_1")
        XCTAssertFalse(summary.steps.first?.done == true)
    }

    func testAcceptanceBundleLoadedSnapshotWorkflowSummaryIgnoresStaleInlineMetaArtifactWhenTwoMachineProofFieldsAreMissing() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

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
            "workflow_summary": {
              "output": "workflow.summary.json",
              "default": {
                "schema": "supermover.acceptance.workflow_status.v1",
                "next_actions": [
                  {
                    "machine": "either",
                    "step": "bundle_handoff",
                    "action": "pack/unpack/merge bundle evidence across machines before installed-app evaluation",
                    "commands": [
                      "sh macos/script/acceptance-two-machine.sh pack-bundle --bundle-root '<bundle-root>' --archive '<bundle.tgz>'"
                    ]
                  }
                ],
                "steps": [
                  {
                    "id": "target_serve_phase_1",
                    "machine": "target",
                    "description": "start target pairing serve",
                    "done": true
                  }
                ]
              },
              "require_operator_evidence": {
                "schema": "supermover.acceptance.workflow_status.v1",
                "next_actions": [],
                "steps": []
              }
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let summary = snapshot.workflowSummary(requireOperatorEvidence: false)

        XCTAssertNil(snapshot.workflowSummaryArtifact)
        XCTAssertEqual(summary.nextActions.first?.step, "target_serve_phase_1")
        XCTAssertEqual(summary.nextActions.first?.machine, "target")
        XCTAssertEqual(summary.steps.first?.id, "target_serve_phase_1")
        XCTAssertFalse(summary.steps.first?.done == true)
    }

    func testAcceptanceBundleReaderRejectsUnsafeArtifactPaths() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

        let outside = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try #"{"pairing_receipt_id":"outside"}"#.write(to: outside, atomically: true, encoding: .utf8)
        defer { try? FileManager.default.removeItem(at: outside) }

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "/tmp/receipt.json",
              "target_address": "127.0.0.1:39395",
              "output": "../\(outside.lastPathComponent)"
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)

        XCTAssertNil(snapshot.sourcePairArtifact)
        XCTAssertEqual(snapshot.issues.count, 1)
        XCTAssertEqual(snapshot.issues.first?.artifact, "../\(outside.lastPathComponent)")
        XCTAssertEqual(snapshot.issues.first?.problem, "unsafe artifact path")
    }

    func testAcceptanceBundleReaderRejectsSymlinkArtifacts() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

        let outside = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try #"{"pairing_receipt_id":"outside"}"#.write(to: outside, atomically: true, encoding: .utf8)
        defer { try? FileManager.default.removeItem(at: outside) }
        let link = dir.appendingPathComponent("source.pair.json")
        do {
            try FileManager.default.createSymbolicLink(at: link, withDestinationURL: outside)
        } catch {
            throw XCTSkip("symlink unavailable: \(error)")
        }

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "in_progress",
          "roles": {},
          "evidence": {
            "source_pair": {
              "pairing_receipt_id": "pair-1",
              "receipt_path": "/tmp/receipt.json",
              "target_address": "127.0.0.1:39395",
              "output": "source.pair.json"
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)

        XCTAssertNil(snapshot.sourcePairArtifact)
        XCTAssertEqual(snapshot.issues.count, 1)
        XCTAssertEqual(snapshot.issues.first?.artifact, "source.pair.json")
        XCTAssertEqual(snapshot.issues.first?.problem, "unsafe artifact path")
    }

    @MainActor
    func testAppStoreRefreshAcceptanceBundleLoadsSnapshot() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "evidence_collected",
          "roles": {},
          "evidence": {
            "discovery": {
              "target_advertise": {
                "output": "target.advertise.json",
                "trusted": false
              }
            }
          }
        }
        """.write(to: dir.appendingPathComponent("meta.json"), atomically: true, encoding: .utf8)

        let store = AppStore()
        store.acceptanceBundlePath = dir.path
        store.refreshAcceptanceBundle()

        XCTAssertEqual(store.acceptanceBundleSnapshot?.status, "evidence_collected")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.meta.targetAdvertise?.output, "target.advertise.json")
        XCTAssertTrue(store.acceptanceBundleLoadError.isEmpty)
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
}
