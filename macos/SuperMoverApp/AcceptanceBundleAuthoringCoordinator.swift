import Foundation

enum AcceptanceBundleArtifactKind: String {
    case sourceBrowse = "source.browse.json"
    case targetAdvertise = "target.advertise.json"
    case targetServePhase = "target.ready.phase"
    case sourcePair = "source.pair.json"
    case sourceTransfer = "source.transfer.json"
    case targetImport = "target_import"
    case evaluation = "evaluation.json"
    case packagingEvidence = "packaging evidence"
}

struct AcceptanceBundleArtifactAuthoringResult {
    let kind: AcceptanceBundleArtifactKind
    let detail: String
}

struct AcceptanceBundleAuthoringCoordinator {
    let writer: AcceptanceBundleArtifactWriter
    let profileReader: ProfilePairingReader

    init(
        writer: AcceptanceBundleArtifactWriter = AcceptanceBundleArtifactWriter(),
        profileReader: ProfilePairingReader = ProfilePairingReader()
    ) {
        self.writer = writer
        self.profileReader = profileReader
    }

    func writeBrowse(
        bundleRootURL: URL,
        snapshot: DiscoveryBrowseSnapshot
    ) throws -> AcceptanceBundleArtifactAuthoringResult {
        try writer.writeDiscoveryBrowse(.init(bundleRootURL: bundleRootURL, snapshot: snapshot))
        return .init(kind: .sourceBrowse, detail: "source.browse.json -> \(bundleRootURL.path)")
    }

    func writeAdvertise(
        bundleRootURL: URL,
        snapshot: DiscoveryAdvertiseSnapshot,
        profilePath: String
    ) throws -> AcceptanceBundleArtifactAuthoringResult {
        guard !profilePath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingProfilePath
        }
        try writer.writeDiscoveryAdvertise(
            .init(
                bundleRootURL: bundleRootURL,
                profilePath: profilePath,
                snapshot: snapshot
            )
        )
        return .init(kind: .targetAdvertise, detail: "target.advertise.json -> \(bundleRootURL.path)")
    }

    func writeServePhase(
        bundleRootURL: URL,
        phase: Int,
        readiness: ServeReadinessSnapshot,
        profilePath: String
    ) throws -> AcceptanceBundleArtifactAuthoringResult {
        guard !profilePath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingProfilePath
        }
        try writer.writeServePhase(
            .init(
                bundleRootURL: bundleRootURL,
                profilePath: profilePath,
                phase: phase,
                readiness: readiness
            )
        )
        return .init(kind: .targetServePhase, detail: "target.ready.phase-\(phase).json -> \(bundleRootURL.path)")
    }

    func writeSourcePair(
        bundleRootURL: URL,
        input: TaskInput,
        pairStdout: String? = nil
    ) throws -> AcceptanceBundleArtifactAuthoringResult {
        guard !input.profilePath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingProfilePath
        }
        guard !input.requiredPairingTargetAddress.isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingPairingTargetAddress
        }
        guard !input.requiredPairingVerificationCode.isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingPairingVerificationCode
        }
        let profileURL = URL(fileURLWithPath: input.profilePath)
        let profile = try profileReader.read(profileURL: profileURL)
        let receiptID = profile.target.pairing_receipt_id?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !receiptID.isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingPairingReceiptID
        }
        let localReceiptPath = resolvedProfileArtifactPath(
            profile.target.local_pairing_receipt_path,
            profileURL: profileURL
        )
        guard !localReceiptPath.isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingLocalPairingReceiptPath
        }
        try writer.writeSourcePair(
            .init(
                bundleRootURL: bundleRootURL,
                profilePath: input.profilePath,
                targetAddress: input.requiredPairingTargetAddress,
                verificationCode: input.requiredPairingVerificationCode,
                localPairingReceiptPath: localReceiptPath,
                pairingReceiptID: receiptID,
                pairStdout: pairStdout
            )
        )
        return .init(kind: .sourcePair, detail: "source.pair.json -> \(bundleRootURL.path)")
    }

    func writeSourceTransfer(
        bundleRootURL: URL,
        input: TaskInput,
        fallbackTargetMode: String,
        sourceBaselineURL: URL? = nil,
        sourceConsistencyEnvelope: StructuredEvidenceEnvelope? = nil,
        verifyArtifactPath: String? = nil,
        statusArtifactPath: String? = nil,
        reportArtifactPath: String? = nil,
        healthArtifactPath: String? = nil,
        pushStdout: String? = nil
    ) throws -> AcceptanceBundleArtifactAuthoringResult {
        guard !input.profilePath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingProfilePath
        }
        guard !input.requiredSessionID.isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingSessionID
        }
        guard !input.requiredPairingTargetAddress.isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingPairingTargetAddress
        }
        let profile = try profileReader.read(profileURL: URL(fileURLWithPath: input.profilePath))
        let receiverURL = profile.network?.receiver_url?.trimmingCharacters(in: .whitespacesAndNewlines) ?? input.profileNetwork.trimmedReceiverURL
        guard !receiverURL.isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingReceiverAddress
        }
        try writer.writeSourceTransfer(
            .init(
                bundleRootURL: bundleRootURL,
                profilePath: input.profilePath,
                sessionID: input.requiredSessionID,
                targetAddress: input.requiredPairingTargetAddress,
                receiverAddress: receiverURL.replacingOccurrences(of: "https://", with: ""),
                targetMode: fallbackTargetMode,
                sourceBaselineURL: sourceBaselineURL,
                sourceConsistencyRawJSON: sourceConsistencyEnvelope?.rawStdout,
                verifyArtifactPath: verifyArtifactPath,
                statusArtifactPath: statusArtifactPath,
                reportArtifactPath: reportArtifactPath,
                healthArtifactPath: healthArtifactPath,
                pushStdout: pushStdout
            )
        )
        return .init(kind: .sourceTransfer, detail: "source.transfer.json -> \(bundleRootURL.path)")
    }

    func writeTargetImport(
        bundleRootURL: URL,
        input: TaskInput,
        adoptedStdout: String? = nil
    ) throws -> AcceptanceBundleArtifactAuthoringResult {
        guard !input.profilePath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingProfilePath
        }
        let profile = try profileReader.read(profileURL: URL(fileURLWithPath: input.profilePath))
        let receiptID = profile.target.pairing_receipt_id?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !receiptID.isEmpty else {
            throw AcceptanceBundleArtifactWriter.WriteError.missingPairingReceiptID
        }
        try writer.writeTargetImport(
            .init(
                bundleRootURL: bundleRootURL,
                profilePath: input.profilePath,
                pairingReceiptID: receiptID,
                adoptedStdout: adoptedStdout
            )
        )
        return .init(kind: .targetImport, detail: "target_import -> \(bundleRootURL.path)")
    }

    func writeStructuredTransferArtifacts(
        bundleRootURL: URL,
        verifyEnvelope: StructuredEvidenceEnvelope?,
        statusEnvelope: StructuredEvidenceEnvelope?,
        reportEnvelope: StructuredEvidenceEnvelope?,
        healthEnvelope: StructuredEvidenceEnvelope?
    ) throws -> (verify: String?, status: String?, report: String?, health: String?) {
        let writer = self.writer
        let verify = try verifyEnvelope.map {
            try writer.writeStructuredJSONArtifact(
                .init(bundleRootURL: bundleRootURL, fileName: "source.verify.json", rawJSON: $0.rawStdout)
            )
        }
        let status = try statusEnvelope.map {
            try writer.writeStructuredJSONArtifact(
                .init(bundleRootURL: bundleRootURL, fileName: "source.status.json", rawJSON: $0.rawStdout)
            )
        }
        let report = try reportEnvelope.map {
            try writer.writeStructuredJSONArtifact(
                .init(bundleRootURL: bundleRootURL, fileName: "source.report.json", rawJSON: $0.rawStdout)
            )
        }
        let health = try healthEnvelope.map {
            try writer.writeStructuredJSONArtifact(
                .init(bundleRootURL: bundleRootURL, fileName: "source.health.json", rawJSON: $0.rawStdout)
            )
        }
        return (verify, status, report, health)
    }

    func transcriptArtifactPath(for kind: AcceptanceBundleArtifactKind) -> String? {
        switch kind {
        case .sourcePair:
            return "source.pair.txt"
        case .targetImport:
            return "target.adopt-pairing.txt"
        case .sourceTransfer:
            return "source.network-push.txt"
        default:
            return nil
        }
    }

    private func resolvedProfileArtifactPath(
        _ rawPath: String?,
        profileURL: URL
    ) -> String {
        let trimmed = rawPath?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !trimmed.isEmpty else {
            return ""
        }
        let expanded = (trimmed as NSString).expandingTildeInPath
        guard !expanded.isEmpty else {
            return ""
        }
        if expanded.hasPrefix("/") {
            return URL(fileURLWithPath: expanded).standardizedFileURL.path
        }
        return profileURL
            .deletingLastPathComponent()
            .appendingPathComponent(expanded)
            .standardizedFileURL
            .path
    }
}
