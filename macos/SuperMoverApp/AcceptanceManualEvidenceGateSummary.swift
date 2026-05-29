import Foundation

struct AcceptanceManualEvidenceRequirement: Equatable, Identifiable {
    let kind: String
    let machine: String
    let step: String
    let description: String

    var id: String { kind }

    static let strictTwoMachineRequirements: [Self] = [
        .init(
            kind: "local_network",
            machine: "target",
            step: "operator_local_network",
            description: "record Local Network prompt evidence"
        ),
        .init(
            kind: "firewall",
            machine: "target",
            step: "operator_firewall",
            description: "record firewall evidence"
        ),
        .init(
            kind: "pairing_confirmation",
            machine: "source",
            step: "operator_pairing_confirmation",
            description: "record physical pairing confirmation evidence"
        ),
    ]
}

struct AcceptanceManualEvidenceGateSummary: Equatable {
    let isRequired: Bool
    let missingEvidence: [String]

    var chipValue: String {
        isRequired ? "required" : "optional"
    }
}

struct AcceptanceManualEvidenceRecordGateStatus: Equatable {
    enum State: String, Equatable {
        case missing
        case valid
        case invalid
        case optional
    }

    let state: State
    let message: String
}

extension AcceptanceBundleLoadedSnapshot {
    func manualEvidenceRequirements(requireOperatorEvidence: Bool) -> [AcceptanceManualEvidenceRequirement] {
        requireOperatorEvidence ? AcceptanceManualEvidenceRequirement.strictTwoMachineRequirements : []
    }

    func manualEvidenceGate(requireOperatorEvidence: Bool) -> AcceptanceManualEvidenceGateSummary {
        let requirements = manualEvidenceRequirements(requireOperatorEvidence: requireOperatorEvidence)
        return AcceptanceManualEvidenceGateSummary(
            isRequired: requireOperatorEvidence,
            missingEvidence: requirements.compactMap { requirement in
                hasRecordedManualEvidence(requirement) ? nil : requirement.kind
            }
        )
    }

    func hasRecordedManualEvidence(_ requirement: AcceptanceManualEvidenceRequirement) -> Bool {
        hasValidManualEvidence(requirement)
    }

    func hasValidManualEvidence(_ requirement: AcceptanceManualEvidenceRequirement) -> Bool {
        guard let record = operatorEvidence[requirement.kind] else {
            return false
        }
        guard record.status == "pass"
            && !record.detail.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        else {
            return false
        }

        guard let expectedMachineID = expectedOperatorEvidenceMachineID(for: requirement) else {
            return false
        }
        return record.machine_id == expectedMachineID
    }

    func manualEvidenceRecordGateStatus(
        kind: String,
        requireOperatorEvidence: Bool
    ) -> AcceptanceManualEvidenceRecordGateStatus {
        guard let record = operatorEvidence[kind] else {
            return .init(
                state: .missing,
                message: "No manual evidence is recorded for this gate."
            )
        }
        guard requireOperatorEvidence else {
            return .init(
                state: .optional,
                message: "Recorded \(record.status); strict operator-evidence gate is not active."
            )
        }
        guard let requirement = AcceptanceManualEvidenceRequirement.strictTwoMachineRequirements.first(where: { $0.kind == kind }) else {
            return .init(
                state: .optional,
                message: "Recorded \(record.status); this kind is outside the strict final-evaluate gate."
            )
        }
        guard record.status == "pass" else {
            return .init(
                state: .invalid,
                message: "Recorded \(record.status); strict final evaluate requires pass."
            )
        }
        guard !record.detail.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            return .init(
                state: .invalid,
                message: "Recorded pass has blank detail; strict final evaluate will reject it."
            )
        }
        guard let expectedMachineID = expectedOperatorEvidenceMachineID(for: requirement) else {
            return .init(
                state: .invalid,
                message: "Recorded pass cannot be validated until \(requirement.machine) machine facts are present."
            )
        }
        guard let recordedMachineID = record.machine_id,
              !recordedMachineID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            return .init(
                state: .invalid,
                message: "Recorded pass is missing \(requirement.machine) machine_id; strict final evaluate will reject it."
            )
        }
        guard recordedMachineID == expectedMachineID else {
            return .init(
                state: .invalid,
                message: "Recorded pass is bound to machine_id=\(recordedMachineID); strict final evaluate expects \(expectedMachineID)."
            )
        }
        return .init(
            state: .valid,
            message: "Recorded pass is bound to \(requirement.machine) machine_id=\(expectedMachineID)."
        )
    }

    private func expectedOperatorEvidenceMachineID(
        for requirement: AcceptanceManualEvidenceRequirement
    ) -> String? {
        let machineID: String?
        switch requirement.machine {
        case "source":
            guard sourceMachineFactsArtifact?.schema == "supermover.acceptance.machine_facts.v1" else {
                return nil
            }
            machineID = sourceMachineFactsArtifact?.machine_id
        case "target":
            guard targetMachineFactsArtifact?.schema == "supermover.acceptance.machine_facts.v1" else {
                return nil
            }
            machineID = targetMachineFactsArtifact?.machine_id
        default:
            machineID = nil
        }
        guard let rawMachineID = machineID,
              !rawMachineID.isEmpty else {
            return nil
        }
        return rawMachineID
    }
}
