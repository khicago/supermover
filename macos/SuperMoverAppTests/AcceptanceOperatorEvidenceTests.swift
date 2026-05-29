import XCTest
@testable import SuperMoverApp

final class AcceptanceOperatorEvidenceTests: XCTestCase {
    func testManualEvidenceGateUsesCurrentStrictLaneInsteadOfStoredEvaluationFlag() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "in_progress",
              "collection": {
                "mode": "two_machine",
                "machine_count": 2
              },
              "roles": {},
              "evidence": {
                "operator": {
                  "local_network": {
                    "status": "pass",
                    "detail": "accepted prompt",
                    "machine_id": "target-machine"
                  }
                }
              }
            }
            """,
            to: dir
        )
        try writeMachineFacts(to: dir)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertFalse(snapshot.persistedEvaluationRequiresOperatorEvidence)

        let gate = snapshot.manualEvidenceGate(requireOperatorEvidence: true)

        XCTAssertTrue(gate.isRequired)
        XCTAssertEqual(gate.chipValue, "required")
        XCTAssertEqual(Set(gate.missingEvidence), Set(["firewall", "pairing_confirmation"]))
    }

    func testManualEvidenceGateDoesNotTreatUnboundPassAsRecorded() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "in_progress",
              "collection": {
                "mode": "two_machine",
                "machine_count": 2
              },
              "roles": {},
              "evidence": {
                "operator": {
                  "local_network": {
                    "status": "pass",
                    "detail": "accepted prompt"
                  },
                  "firewall": {
                    "status": "pass",
                    "detail": "allowed firewall access",
                    "machine_id": "target-machine"
                  },
                  "pairing_confirmation": {
                    "status": "pass",
                    "detail": "confirmed code",
                    "machine_id": "source-machine"
                  }
                }
              }
            }
            """,
            to: dir
        )
        try writeMachineFacts(to: dir)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let gate = snapshot.manualEvidenceGate(requireOperatorEvidence: true)

        XCTAssertEqual(gate.missingEvidence, ["local_network"])
    }

    func testManualEvidenceGateRejectsWrongMachineBinding() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "in_progress",
              "collection": {
                "mode": "two_machine",
                "machine_count": 2
              },
              "roles": {},
              "evidence": {
                "operator": {
                  "local_network": {
                    "status": "pass",
                    "detail": "accepted prompt",
                    "machine_id": "source-machine"
                  },
                  "firewall": {
                    "status": "pass",
                    "detail": "allowed firewall access",
                    "machine_id": "target-machine"
                  },
                  "pairing_confirmation": {
                    "status": "pass",
                    "detail": "confirmed code",
                    "machine_id": "source-machine"
                  }
                }
              }
            }
            """,
            to: dir
        )
        try writeMachineFacts(to: dir)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let gate = snapshot.manualEvidenceGate(requireOperatorEvidence: true)

        XCTAssertEqual(gate.missingEvidence, ["local_network"])
    }

    func testManualEvidenceRecordGateStatusExplainsInvalidPassBinding() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "in_progress",
              "collection": {
                "mode": "two_machine",
                "machine_count": 2
              },
              "roles": {},
              "evidence": {
                "operator": {
                  "local_network": {
                    "status": "pass",
                    "detail": "accepted prompt"
                  },
                  "firewall": {
                    "status": "pass",
                    "detail": "allowed firewall access",
                    "machine_id": "source-machine"
                  }
                }
              }
            }
            """,
            to: dir
        )
        try writeMachineFacts(to: dir)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)

        XCTAssertEqual(
            snapshot.manualEvidenceRecordGateStatus(
                kind: "local_network",
                requireOperatorEvidence: true
            ),
            AcceptanceManualEvidenceRecordGateStatus(
                state: .invalid,
                message: "Recorded pass is missing target machine_id; strict final evaluate will reject it."
            )
        )
        XCTAssertEqual(
            snapshot.manualEvidenceRecordGateStatus(
                kind: "firewall",
                requireOperatorEvidence: true
            ),
            AcceptanceManualEvidenceRecordGateStatus(
                state: .invalid,
                message: "Recorded pass is bound to machine_id=source-machine; strict final evaluate expects target-machine."
            )
        )
    }

    func testManualEvidenceRecordGateStatusUsesRawMachineIDBinding() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "in_progress",
              "collection": {
                "mode": "two_machine",
                "machine_count": 2
              },
              "roles": {},
              "evidence": {
                "operator": {
                  "local_network": {
                    "status": "pass",
                    "detail": "accepted prompt",
                    "machine_id": "target-machine "
                  }
                }
              }
            }
            """,
            to: dir
        )
        try writeMachineFacts(to: dir)

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)

        XCTAssertEqual(
            snapshot.manualEvidenceRecordGateStatus(
                kind: "local_network",
                requireOperatorEvidence: true
            ),
            AcceptanceManualEvidenceRecordGateStatus(
                state: .invalid,
                message: "Recorded pass is bound to machine_id=target-machine ; strict final evaluate expects target-machine."
            )
        )
    }

    func testManualEvidenceRecordGateStatusUsesRawMachineFactsMachineID() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "in_progress",
              "collection": {
                "mode": "two_machine",
                "machine_count": 2
              },
              "roles": {},
              "evidence": {
                "operator": {
                  "local_network": {
                    "status": "pass",
                    "detail": "accepted prompt",
                    "machine_id": "target-machine"
                  },
                  "firewall": {
                    "status": "pass",
                    "detail": "allowed firewall access",
                    "machine_id": "target-machine "
                  }
                }
              }
            }
            """,
            to: dir
        )
        try writeMachineFacts(to: dir, targetMachineID: "target-machine ")

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)

        XCTAssertEqual(
            snapshot.manualEvidenceRecordGateStatus(
                kind: "local_network",
                requireOperatorEvidence: true
            ),
            AcceptanceManualEvidenceRecordGateStatus(
                state: .invalid,
                message: "Recorded pass is bound to machine_id=target-machine; strict final evaluate expects target-machine ."
            )
        )
        XCTAssertEqual(
            snapshot.manualEvidenceRecordGateStatus(
                kind: "firewall",
                requireOperatorEvidence: true
            ),
            AcceptanceManualEvidenceRecordGateStatus(
                state: .valid,
                message: "Recorded pass is bound to target machine_id=target-machine ."
            )
        )
    }

    func testManualEvidenceRecordGateStatusRejectsMalformedMachineFactsSchema() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "in_progress",
              "collection": {
                "mode": "two_machine",
                "machine_count": 2
              },
              "roles": {},
              "evidence": {
                "operator": {
                  "local_network": {
                    "status": "pass",
                    "detail": "accepted prompt",
                    "machine_id": "target-machine"
                  }
                }
              }
            }
            """,
            to: dir
        )
        try writeMachineFacts(to: dir, targetSchema: "supermover.acceptance.machine_facts.v0")

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)

        XCTAssertEqual(
            snapshot.manualEvidenceRecordGateStatus(
                kind: "local_network",
                requireOperatorEvidence: true
            ),
            AcceptanceManualEvidenceRecordGateStatus(
                state: .invalid,
                message: "Recorded pass cannot be validated until target machine facts are present."
            )
        )
    }

    func testManualEvidenceGateCanRemainOptionalOutsideStrictLane() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "in_progress",
              "roles": {},
              "evidence": {}
            }
            """,
            to: dir
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        let gate = snapshot.manualEvidenceGate(requireOperatorEvidence: false)

        XCTAssertFalse(gate.isRequired)
        XCTAssertEqual(gate.chipValue, "optional")
        XCTAssertEqual(gate.missingEvidence, [])
    }

    func testManualEvidenceGateUsesCurrentOptionalLaneWhenStoredEvaluationRequiresOperatorEvidence() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "evidence_collected",
              "roles": {},
              "evidence": {
                "evaluation": {
                  "pairing_receipt_id": "pair-1",
                  "session_id": "session-1",
                  "target_root": "/tmp/target-root",
                  "output": "evaluation.json",
                  "require_operator_evidence": true
                }
              }
            }
            """,
            to: dir
        )

        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "evidence_collected",
          "pairing_receipt_id": "pair-1",
          "session_id": "session-1",
          "target_root": "/tmp/target-root",
          "require_operator_evidence": true
        }
        """.write(
            to: dir.appendingPathComponent("evaluation.json"),
            atomically: true,
            encoding: .utf8
        )

        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertTrue(snapshot.persistedEvaluationRequiresOperatorEvidence)

        let gate = snapshot.manualEvidenceGate(requireOperatorEvidence: false)

        XCTAssertFalse(gate.isRequired)
        XCTAssertEqual(gate.chipValue, "optional")
        XCTAssertEqual(gate.missingEvidence, [])
    }

    func testMetaStoreRecordsOperatorEvidenceIntoBundleMeta() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "in_progress",
              "roles": {},
              "evidence": {}
            }
            """,
            to: dir
        )

        try AcceptanceBundleMetaStore().recordOperatorEvidence(
            bundleRootURL: dir,
            record: AcceptanceBundleOperatorEvidenceRecord(
                kind: "local_network",
                status: "pass",
                detail: "accepted prompt on target machine",
                artifact: "Screenshots/local-network.png",
                machineID: "target-machine",
                machineLabel: "target"
            )
        )

        let loaded = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(loaded.operatorEvidence["local_network"]?.status, "pass")
        XCTAssertEqual(loaded.operatorEvidence["local_network"]?.detail, "accepted prompt on target machine")
        XCTAssertEqual(loaded.operatorEvidence["local_network"]?.artifact, "Screenshots/local-network.png")
        XCTAssertEqual(loaded.operatorEvidence["local_network"]?.machine_id, "target-machine")
        XCTAssertEqual(loaded.operatorEvidence["local_network"]?.machine_label, "target")
    }

    func testMetaStoreRejectsMalformedMeta() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }
        try writeMeta("{", to: dir)

        XCTAssertThrowsError(
            try AcceptanceBundleMetaStore().recordOperatorEvidence(
                bundleRootURL: dir,
                record: AcceptanceBundleOperatorEvidenceRecord(
                    kind: "firewall",
                    status: "blocked",
                    detail: "prompt dismissed",
                    artifact: nil
                )
            )
        ) { error in
            XCTAssertEqual(
                error as? AcceptanceBundleMetaStore.MutationError,
                .malformedMeta(dir.appendingPathComponent("meta.json"))
            )
        }
    }

    @MainActor
    func testAppStoreRecordsAcceptanceOperatorEvidenceAndRefreshesBundle() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "evidence_collected",
              "roles": {},
              "evidence": {
                "evaluation": {
                  "pairing_receipt_id": "pair-1",
                  "session_id": "session-1",
                  "target_root": "/tmp/target-root",
                  "output": "evaluation.json",
                  "require_operator_evidence": true
                }
              }
            }
            """,
            to: dir
        )
        try writeMachineFacts(to: dir)
        try """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "evidence_collected",
          "pairing_receipt_id": "pair-1",
          "session_id": "session-1",
          "target_root": "/tmp/target-root",
          "require_operator_evidence": true
        }
        """.write(to: dir.appendingPathComponent("evaluation.json"), atomically: true, encoding: .utf8)

        let store = AppStore()
        store.selectedRole = .target
        store.acceptanceBundlePath = dir.path
        store.refreshAcceptanceBundle()
        store.acceptanceOperatorEvidence.kind = .pairingConfirmation
        store.acceptanceOperatorEvidence.status = .pass
        store.acceptanceOperatorEvidence.detail = "confirmed six digits on both devices"
        store.acceptanceOperatorEvidence.artifactPath = "Photos/pairing-code.jpg"

        store.recordAcceptanceOperatorEvidence()

        XCTAssertEqual(store.acceptanceBundleSnapshot?.operatorEvidence["pairing_confirmation"]?.status, "pass")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.operatorEvidence["pairing_confirmation"]?.artifact, "Photos/pairing-code.jpg")
        XCTAssertEqual(store.acceptanceBundleSnapshot?.operatorEvidence["pairing_confirmation"]?.machine_id, "source-machine")
        XCTAssertTrue(store.acceptanceBundleLoadError.isEmpty)
        XCTAssertEqual(store.note, "Recorded manual evidence for Pairing Code.")
    }

    @MainActor
    func testAppStoreRejectsPassOperatorEvidenceBeforeMachineFacts() throws {
        let dir = try makeBundleDirectory()
        defer { try? FileManager.default.removeItem(at: dir) }

        try writeMeta(
            """
            {
              "schema": "supermover.acceptance.two_machine.v1",
              "status": "in_progress",
              "roles": {},
              "evidence": {}
            }
            """,
            to: dir
        )

        let store = AppStore()
        store.selectedRole = .source
        store.acceptanceBundlePath = dir.path
        store.refreshAcceptanceBundle()
        store.acceptanceOperatorEvidence.kind = .localNetwork
        store.acceptanceOperatorEvidence.status = .pass
        store.acceptanceOperatorEvidence.detail = "accepted prompt"

        store.recordAcceptanceOperatorEvidence()

        XCTAssertEqual(store.note, "Record source/target machine facts before writing pass manual evidence.")
        let snapshot = try AcceptanceBundleReader().load(bundleRootURL: dir)
        XCTAssertEqual(snapshot.operatorEvidence, [:])
    }

    private func makeBundleDirectory() throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }

    private func writeMeta(_ text: String, to dir: URL) throws {
        try text.write(
            to: dir.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
    }

    private func writeMachineFacts(
        to dir: URL,
        sourceSchema: String = "supermover.acceptance.machine_facts.v1",
        targetSchema: String = "supermover.acceptance.machine_facts.v1",
        sourceMachineID: String = "source-machine",
        targetMachineID: String = "target-machine"
    ) throws {
        try """
        {
          "schema": "\(sourceSchema)",
          "machine_id": "\(sourceMachineID)",
          "machine_label": "source"
        }
        """.write(to: dir.appendingPathComponent("source.machine.json"), atomically: true, encoding: .utf8)
        try """
        {
          "schema": "\(targetSchema)",
          "machine_id": "\(targetMachineID)",
          "machine_label": "target"
        }
        """.write(to: dir.appendingPathComponent("target.machine.json"), atomically: true, encoding: .utf8)
    }
}
