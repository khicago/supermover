import Foundation

enum AcceptanceOperatorEvidenceKind: String, CaseIterable, Identifiable {
    case localNetwork = "local_network"
    case firewall
    case pairingConfirmation = "pairing_confirmation"

    var id: String { rawValue }

    var title: String {
        switch self {
        case .localNetwork:
            return "Local Network"
        case .firewall:
            return "Firewall"
        case .pairingConfirmation:
            return "Pairing Code"
        }
    }

    var guidance: String {
        switch self {
        case .localNetwork:
            return "Record whether the machine accepted the macOS Local Network prompt required for discovery, pairing, or receiver access."
        case .firewall:
            return "Record whether the machine allowed inbound listen/firewall access for the installed app during the real-device run."
        case .pairingConfirmation:
            return "Record that the operator physically confirmed the target verification code before treating pairing as trusted."
        }
    }
}

enum AcceptanceOperatorEvidenceStatus: String, CaseIterable, Identifiable {
    case pass
    case blocked

    var id: String { rawValue }

    var title: String {
        rawValue.capitalized
    }
}

struct AcceptanceOperatorEvidenceDraft: Equatable {
    var kind: AcceptanceOperatorEvidenceKind = .localNetwork
    var status: AcceptanceOperatorEvidenceStatus = .pass
    var detail = ""
    var artifactPath = ""

    var trimmedDetail: String {
        detail.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var trimmedArtifactPath: String {
        artifactPath.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var record: AcceptanceBundleOperatorEvidenceRecord {
        makeRecord(machineIdentity: nil)
    }

    func makeRecord(machineIdentity: AcceptanceOperatorEvidenceMachineIdentity?) -> AcceptanceBundleOperatorEvidenceRecord {
        AcceptanceBundleOperatorEvidenceRecord(
            kind: kind.rawValue,
            status: status.rawValue,
            detail: trimmedDetail,
            artifact: trimmedArtifactPath.isEmpty ? nil : trimmedArtifactPath,
            machineID: machineIdentity?.id,
            machineLabel: machineIdentity?.label
        )
    }
}

struct AcceptanceOperatorEvidenceMachineIdentity: Equatable {
    let id: String
    let label: String?
}
