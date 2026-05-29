import Foundation

struct AcceptanceBundleEvaluationCoordinator {
    enum EvaluationError: LocalizedError, Equatable {
        case missingTargetRoot
        case unreadableTargetRoot(String)
        case missingBundleMeta
        case malformedProvenance(String)
        case missingRequiredArtifact(String)
        case malformedArtifact(String)
        case invalidAppAudit(String)
        case invalidPairingReceiptID
        case invalidSessionID
        case missingTargetImportEvidence
        case invalidTargetImportEvidence
        case missingTargetPairingReceipt(String)
        case invalidTargetPairingReceipt(String)
        case missingTargetNetworkTransfer(String)
        case invalidTargetNetworkTransfer(String)
        case invalidVerifyEvidence
        case invalidReportEvidence
        case invalidStatusEvidence
        case invalidHealthEvidence
        case missingCurrentSourceConsistencyEvidence
        case blockedCurrentSourceConsistency(String)
        case missingOperatorEvidence(String)
        case invalidInstalledAppCollection(String)
        case blockedAppAudit(String)
        case invalidNotarizationEvidence(String)

        var errorDescription: String? {
            switch self {
            case .missingTargetRoot:
                return "Choose a target root before writing acceptance evaluation."
            case let .unreadableTargetRoot(path):
                return "Target root is not readable: \(path)"
            case .missingBundleMeta:
                return "Acceptance bundle meta.json is missing or unreadable."
            case let .malformedProvenance(path):
                return "Acceptance provenance is malformed or incomplete: \(path)"
            case let .missingRequiredArtifact(path):
                return "Acceptance bundle is missing required artifact: \(path)"
            case let .malformedArtifact(path):
                return "Acceptance artifact is malformed: \(path)"
            case let .invalidAppAudit(path):
                return "App audit artifact is missing required schema/readiness fields: \(path)"
            case .invalidPairingReceiptID:
                return "Acceptance bundle is missing a durable pairing_receipt_id."
            case .invalidSessionID:
                return "Acceptance bundle is missing a durable session_id."
            case .missingTargetImportEvidence:
                return "Acceptance bundle is missing required target_import evidence."
            case .invalidTargetImportEvidence:
                return "Acceptance bundle target_import evidence does not match the paired receipt."
            case let .missingTargetPairingReceipt(path):
                return "Target control-plane pairing receipt is missing: \(path)"
            case let .invalidTargetPairingReceipt(path):
                return "Target control-plane pairing receipt is not a regular non-symlink artifact: \(path)"
            case let .missingTargetNetworkTransfer(path):
                return "Target control-plane network-transfer evidence is missing: \(path)"
            case let .invalidTargetNetworkTransfer(path):
                return "Target network-transfer evidence is not published commit-stage mTLS evidence: \(path)"
            case .invalidVerifyEvidence:
                return "Current verify evidence does not prove a clean published transfer result."
            case .invalidReportEvidence:
                return "Current report evidence does not show paired_receipt_valid."
            case .invalidStatusEvidence:
                return "Current status evidence is malformed or does not match the accepted transfer."
            case .invalidHealthEvidence:
                return "Current health evidence is malformed or does not match the accepted transfer."
            case .missingCurrentSourceConsistencyEvidence:
                return "Acceptance bundle is missing required current-source consistency evidence."
            case let .blockedCurrentSourceConsistency(mode):
                return "Current-source consistency proof is not verified; acceptance bundle reports \(mode)."
            case let .missingOperatorEvidence(kind):
                return "Required manual evidence is missing pass status for \(kind)."
            case let .invalidInstalledAppCollection(reason):
                return "Acceptance bundle does not prove real two-machine installed-app collection: \(reason)"
            case let .blockedAppAudit(machine):
                return "Acceptance bundle app audit is not install-ready for \(machine)."
            case let .invalidNotarizationEvidence(machine):
                return "Acceptance bundle notarization evidence is not release-ready for \(machine)."
            }
        }
    }

    private struct ProvenanceManifest: Decodable {
        let schema: String?
        let git_commit: String?
        let cli_version: String?
        let cli_relative_path: String?
        let build_profile: String?
        let signing: String?
    }

    private struct AppAuditArtifact: Decodable {
        struct Summary: Decodable {
            let pass_ready: Bool?
        }

        let schema: String?
        let status: String?
        let readiness: String?
        let summary: Summary?
    }

    private struct TargetNetworkTransferArtifact: Decodable {
        let version: Int?
        let session_id: String?
        let profile_id: String?
        let target_id: String?
        let status: String?
        let stage: String?
        let encrypted_transfer: String?
        let source_device_id: String?
        let target_device_id: String?
        let protocol_version: String?
        let started_at: String?
        let updated_at: String?
    }

    private struct TargetPairingReceiptArtifact: Decodable {
        let version: Int?
        let id: String?
        let profile_id: String?
        let target_id: String?
        let source_device_id: String?
        let target_device_id: String?
        let device_public_key: String?
        let method: String?
        let verified_at: String?
        let verification_hash: String?
        let verification_phrase: String?
        let protocol_version: String?
    }

    private struct NotarizationArtifact: Decodable {
        struct Submission: Decodable {
            let status: String?
        }

        struct Audit: Decodable {
            let path: String?
            let status: String?
            let readiness: String?
            let pass_ready: Bool?
        }

        let schema: String?
        let status: String?
        let auth_mode: String?
        let submission: Submission?
        let audit: Audit?
    }

    private struct MachineFactsArtifact: Decodable {
        let schema: String?
        let machine_id: String?
        let machine_label: String?
    }

    let fileManager: FileManager
    let artifactAccess: AcceptanceBundleArtifactAccess
    let bundleReader: AcceptanceBundleReader
    let writer: AcceptanceBundleArtifactWriter

    init(
        fileManager: FileManager = .default,
        bundleReader: AcceptanceBundleReader = AcceptanceBundleReader(),
        writer: AcceptanceBundleArtifactWriter = AcceptanceBundleArtifactWriter()
    ) {
        self.fileManager = fileManager
        artifactAccess = AcceptanceBundleArtifactAccess(fileManager: fileManager)
        self.bundleReader = bundleReader
        self.writer = writer
    }

    func evaluate(
        bundleRootURL: URL,
        targetRootURL: URL,
        requireOperatorEvidence: Bool
    ) throws -> AcceptanceBundleArtifactAuthoringResult {
        try ensureReadableDirectory(targetRootURL)
        let snapshot = try bundleReader.load(bundleRootURL: bundleRootURL)

        try validateBundle(snapshot: snapshot, bundleRootURL: bundleRootURL)
        let pairingReceiptID = try extractPairingReceiptID(snapshot: snapshot)
        let sessionID = try extractSessionID(snapshot: snapshot)
        try validateSourcePairReceipt(
            snapshot: snapshot,
            bundleRootURL: bundleRootURL,
            pairingReceiptID: pairingReceiptID
        )
        try validateTargetImport(snapshot: snapshot, pairingReceiptID: pairingReceiptID)
        try validateTargetControlPlane(
            targetRootURL: targetRootURL,
            snapshot: snapshot,
            pairingReceiptID: pairingReceiptID,
            sessionID: sessionID
        )
        try validateSnapshots(
            snapshot: snapshot,
            pairingReceiptID: pairingReceiptID,
            sessionID: sessionID,
            targetRootURL: targetRootURL,
            bundleRootURL: bundleRootURL
        )
        if requireOperatorEvidence {
            try requireRegularArtifact("source.notarization.json", bundleRootURL: bundleRootURL)
            try requireRegularArtifact("target.notarization.json", bundleRootURL: bundleRootURL)
            try requireRegularArtifact("source.machine.json", bundleRootURL: bundleRootURL)
            try requireRegularArtifact("target.machine.json", bundleRootURL: bundleRootURL)
            try validateInstalledAppReleaseEvidence(snapshot: snapshot)
            try validateNotaryLogArtifact(
                snapshot.sourceNotarizationArtifact,
                machine: "source",
                appAudit: snapshot.sourceAppAuditArtifact,
                bundleRootURL: bundleRootURL
            )
            try validateNotaryLogArtifact(
                snapshot.targetNotarizationArtifact,
                machine: "target",
                appAudit: snapshot.targetAppAuditArtifact,
                bundleRootURL: bundleRootURL
            )
            try validateInstalledAppMachineFacts(snapshot: snapshot, bundleRootURL: bundleRootURL)
            try validateInstalledAppBundleHandoffs(snapshot: snapshot)
            try validateOperatorEvidence(snapshot: snapshot)
        }
        try validateTargetReady(snapshot: snapshot, bundleRootURL: bundleRootURL)
        try validateSourceEvidenceMatchesTargetReady(snapshot: snapshot)

        try writer.writeEvaluation(
            .init(
                bundleRootURL: bundleRootURL,
                pairingReceiptID: pairingReceiptID,
                sessionID: sessionID,
                targetRoot: targetRootURL.path,
                requireOperatorEvidence: requireOperatorEvidence
            )
        )
        return .init(kind: .evaluation, detail: "evaluation.json -> \(bundleRootURL.path)")
    }

    private func ensureReadableDirectory(_ url: URL) throws {
        var isDirectory: ObjCBool = false
        guard fileManager.fileExists(atPath: url.path, isDirectory: &isDirectory), isDirectory.boolValue else {
            throw EvaluationError.unreadableTargetRoot(url.path)
        }
        let values = try url.resourceValues(forKeys: [.isSymbolicLinkKey])
        if values.isSymbolicLink == true {
            throw EvaluationError.unreadableTargetRoot(url.path)
        }
    }

    private func validateBundle(
        snapshot: AcceptanceBundleLoadedSnapshot,
        bundleRootURL: URL
    ) throws {
        try validateProvenance(bundleRootURL: bundleRootURL, name: "source.provenance.json")
        try validateProvenance(bundleRootURL: bundleRootURL, name: "target.provenance.json")
        try validateAppAudit(bundleRootURL: bundleRootURL, name: "source.app-audit.json")
        try validateAppAudit(bundleRootURL: bundleRootURL, name: "target.app-audit.json")
        try requireRegularArtifact(snapshot.meta.evidence.source_pair?.output ?? "source.pair.json", bundleRootURL: bundleRootURL)
        try requireRegularArtifact(snapshot.meta.evidence.source_transfer?.output ?? "source.transfer.json", bundleRootURL: bundleRootURL)
        try requireRegularArtifact(snapshot.meta.evidence.source_pair?.pair ?? "source.pair.txt", bundleRootURL: bundleRootURL)
        try requireRegularArtifact(snapshot.meta.evidence.source_transfer?.push ?? "source.network-push.txt", bundleRootURL: bundleRootURL)
        try requireRegularArtifact(snapshot.meta.evidence.source_transfer?.verify ?? "source.verify.json", bundleRootURL: bundleRootURL)
        try requireRegularArtifact(snapshot.meta.evidence.source_transfer?.status ?? "source.status.json", bundleRootURL: bundleRootURL)
        try requireRegularArtifact(snapshot.meta.evidence.source_transfer?.report ?? "source.report.json", bundleRootURL: bundleRootURL)
        try requireRegularArtifact(snapshot.meta.evidence.source_transfer?.health ?? "source.health.json", bundleRootURL: bundleRootURL)
        try requireRegularArtifact(snapshot.meta.evidence.source_consistency?.output ?? "source.consistency.json", bundleRootURL: bundleRootURL)
        if let targetImport = snapshot.meta.evidence.target_import {
            let adopted = targetImport.adopted?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            guard !adopted.isEmpty else {
                throw EvaluationError.missingRequiredArtifact("target_import adopted artifact")
            }
            try requireRegularArtifact(adopted, bundleRootURL: bundleRootURL)
        }
        if let sourceBrowse = snapshot.meta.sourceBrowse?.output {
            try validateDiscoveryBrowse(bundleRootURL: bundleRootURL, relativePath: sourceBrowse)
        }
        if let targetAdvertise = snapshot.meta.targetAdvertise?.output {
            try validateDiscoveryAdvertise(bundleRootURL: bundleRootURL, relativePath: targetAdvertise)
        }
    }

    private func validateProvenance(bundleRootURL: URL, name: String) throws {
        let data = try requireBundleArtifactData(name, bundleRootURL: bundleRootURL)
        guard let manifest = try? JSONDecoder().decode(ProvenanceManifest.self, from: data),
              manifest.schema == "supermover.macos.provenance.v1",
              !(manifest.git_commit?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true),
              !(manifest.cli_version?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true),
              manifest.cli_relative_path == "Contents/Resources/bin/supermover",
              !(manifest.build_profile?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true),
              !(manifest.signing?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true) else {
            throw EvaluationError.malformedProvenance(name)
        }
    }

    private func validateAppAudit(bundleRootURL: URL, name: String) throws {
        let data = try requireBundleArtifactData(name, bundleRootURL: bundleRootURL)
        guard let audit = try? JSONDecoder().decode(AppAuditArtifact.self, from: data),
              audit.schema == "supermover.macos.app_audit.v1",
              !(audit.status?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true),
              !(audit.readiness?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true) else {
            throw EvaluationError.invalidAppAudit(name)
        }
    }

    private func validateDiscoveryBrowse(bundleRootURL: URL, relativePath: String) throws {
        let data = try requireBundleArtifactData(relativePath, bundleRootURL: bundleRootURL)
        guard let snapshot = try? JSONDecoder().decode(DiscoveryBrowseSnapshot.self, from: data),
              snapshot.trusted == false else {
            throw EvaluationError.malformedArtifact(relativePath)
        }
    }

    private func validateDiscoveryAdvertise(bundleRootURL: URL, relativePath: String) throws {
        let data = try requireBundleArtifactData(relativePath, bundleRootURL: bundleRootURL)
        guard let snapshot = try? JSONDecoder().decode(DiscoveryAdvertiseSnapshot.self, from: data),
              snapshot.status == "advertised",
              snapshot.trusted == false else {
            throw EvaluationError.malformedArtifact(relativePath)
        }
    }

    private func validateTargetReady(
        snapshot: AcceptanceBundleLoadedSnapshot,
        bundleRootURL: URL
    ) throws {
        let data = try requireBundleArtifactData("target.ready.json", bundleRootURL: bundleRootURL)
        guard let readiness = try? JSONDecoder().decode(ServeReadinessSnapshot.self, from: data),
              let metaReady = snapshot.meta.evidence.target_ready,
              requiredPairingReceiptField(readiness.address) != nil,
              requiredPairingReceiptField(readiness.mode) != nil,
              readiness.address == metaReady.address,
              readiness.mode == metaReady.mode else {
            throw EvaluationError.malformedArtifact("target.ready.json")
        }

        let artifactVerification = readiness.verification_code?
            .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if readiness.mode == "pairing" || readiness.mode == "pairing-only" {
            guard requiredPairingReceiptField(metaReady.verification_code) != nil,
                  metaReady.verification_code == artifactVerification else {
                throw EvaluationError.malformedArtifact("target.ready.json")
            }
        } else if !artifactVerification.isEmpty && metaReady.verification_code != artifactVerification {
            throw EvaluationError.malformedArtifact("target.ready.json")
        }
    }

    private func validateSourceEvidenceMatchesTargetReady(
        snapshot: AcceptanceBundleLoadedSnapshot
    ) throws {
        guard let readiness = snapshot.targetReadyArtifact else {
            throw EvaluationError.malformedArtifact("target.ready.json")
        }
        guard let sourcePair = snapshot.sourcePairArtifact,
              sourcePair.target_address == readiness.address else {
            throw EvaluationError.malformedArtifact("source.pair.json")
        }
        guard let sourceTransfer = snapshot.sourceTransferArtifact,
              sourceTransfer.target_address == readiness.address,
              sourceTransfer.target_mode == readiness.mode else {
            throw EvaluationError.malformedArtifact("source.transfer.json")
        }

        let expectedReceiver = readiness.receiver_address?
            .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !expectedReceiver.isEmpty,
              readiness.receiver_routes == true,
              readiness.push_network == true,
              readiness.transfer == true else {
            throw EvaluationError.malformedArtifact("target.ready.json")
        }
        if !expectedReceiver.isEmpty, sourceTransfer.receiver_address != expectedReceiver {
            throw EvaluationError.malformedArtifact("source.transfer.json")
        }
    }

    private func extractPairingReceiptID(snapshot: AcceptanceBundleLoadedSnapshot) throws -> String {
        guard let receiptID = AcceptanceControlPlaneID.safeRawSegment(
            snapshot.sourcePairArtifact?.pairing_receipt_id
        ) else {
            throw EvaluationError.invalidPairingReceiptID
        }
        return receiptID
    }

    private func extractSessionID(snapshot: AcceptanceBundleLoadedSnapshot) throws -> String {
        guard let sessionID = AcceptanceControlPlaneID.safeRawSegment(
            snapshot.sourceTransferArtifact?.session_id
        ) else {
            throw EvaluationError.invalidSessionID
        }
        return sessionID
    }

    private func validateTargetImport(
        snapshot: AcceptanceBundleLoadedSnapshot,
        pairingReceiptID: String
    ) throws {
        guard let targetImport = snapshot.meta.evidence.target_import else {
            throw EvaluationError.missingTargetImportEvidence
        }
        guard targetImport.pairing_receipt_id == pairingReceiptID else {
            throw EvaluationError.invalidTargetImportEvidence
        }
    }

    private func validateSourcePairReceipt(
        snapshot: AcceptanceBundleLoadedSnapshot,
        bundleRootURL: URL,
        pairingReceiptID: String
    ) throws {
        let receiptPath = snapshot.sourcePairArtifact?.receipt_path
            .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !receiptPath.isEmpty else {
            throw EvaluationError.missingRequiredArtifact("source pairing receipt artifact")
        }
        let data = try requireBundleArtifactData(receiptPath, bundleRootURL: bundleRootURL)
        try validatePairingReceipt(data, pairingReceiptID: pairingReceiptID, invalidError: .malformedArtifact(receiptPath))
    }

    private func validateTargetControlPlane(
        targetRootURL: URL,
        snapshot: AcceptanceBundleLoadedSnapshot,
        pairingReceiptID: String,
        sessionID: String
    ) throws {
        let pairingPath = ".supermover/pairings/\(pairingReceiptID).json"
        let pairingData = try requireTargetControlPlaneArtifactData(
            relativePath: pairingPath,
            targetRootURL: targetRootURL,
            missingError: .missingTargetPairingReceipt(pairingPath),
            invalidError: .invalidTargetPairingReceipt(pairingPath)
        )
        try validatePairingReceipt(
            pairingData,
            pairingReceiptID: pairingReceiptID,
            invalidError: .invalidTargetPairingReceipt(pairingPath)
        )

        let transferPath = ".supermover/sessions/\(sessionID)/network-transfer.json"
        let data = try requireTargetControlPlaneArtifactData(
            relativePath: transferPath,
            targetRootURL: targetRootURL,
            missingError: .missingTargetNetworkTransfer(transferPath),
            invalidError: .invalidTargetNetworkTransfer(transferPath)
        )
        guard let transfer = try? JSONDecoder().decode(TargetNetworkTransferArtifact.self, from: data),
              let version = transfer.version,
              version > 0,
              requiredPairingReceiptField(transfer.session_id) == sessionID,
              requiredPairingReceiptField(transfer.profile_id) != nil,
              requiredPairingReceiptField(transfer.target_id) != nil,
              requiredPairingReceiptField(transfer.source_device_id) != nil,
              requiredPairingReceiptField(transfer.target_device_id) != nil,
              requiredPairingReceiptField(transfer.source_device_id) != requiredPairingReceiptField(transfer.target_device_id),
              requiredPairingReceiptField(transfer.protocol_version) != nil,
              transfer.status == "published",
              transfer.stage == "commit",
              transfer.encrypted_transfer == "tls13_mtls",
              rfc3339Date(transfer.started_at) != nil,
              let startedAt = rfc3339Date(transfer.started_at),
              let updatedAt = rfc3339Date(transfer.updated_at),
              updatedAt >= startedAt else {
            throw EvaluationError.invalidTargetNetworkTransfer(transferPath)
        }
        if let statusProfileID = requiredPairingReceiptField(snapshot.sourceStatusArtifact?.profile_id),
           statusProfileID != requiredPairingReceiptField(transfer.profile_id) {
            throw EvaluationError.invalidTargetNetworkTransfer(transferPath)
        }
        if let statusTargetID = requiredPairingReceiptField(snapshot.sourceStatusArtifact?.target_id),
           statusTargetID != requiredPairingReceiptField(transfer.target_id) {
            throw EvaluationError.invalidTargetNetworkTransfer(transferPath)
        }
    }

    private func validatePairingReceipt(
        _ data: Data,
        pairingReceiptID: String,
        invalidError: EvaluationError
    ) throws {
        guard let receipt = try? JSONDecoder().decode(TargetPairingReceiptArtifact.self, from: data),
              let version = receipt.version,
              version > 0,
              requiredPairingReceiptField(receipt.id) == pairingReceiptID,
              requiredPairingReceiptField(receipt.profile_id) != nil,
              requiredPairingReceiptField(receipt.target_id) != nil,
              requiredPairingReceiptField(receipt.source_device_id) != nil,
              requiredPairingReceiptField(receipt.target_device_id) != nil,
              requiredPairingReceiptField(receipt.device_public_key) != nil,
              requiredPairingReceiptField(receipt.device_public_key) == requiredPairingReceiptField(receipt.target_device_id),
              requiredPairingReceiptField(receipt.method) != nil,
              rfc3339Date(receipt.verified_at) != nil,
              requiredPairingReceiptField(receipt.verification_hash) != nil || requiredPairingReceiptField(receipt.verification_phrase) != nil,
              requiredPairingReceiptField(receipt.protocol_version) != nil else {
            throw invalidError
        }
    }

    private func requiredPairingReceiptField(_ value: String?) -> String? {
        let trimmed = value?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return trimmed.isEmpty ? nil : trimmed
    }

    private func rfc3339Date(_ value: String?) -> Date? {
        guard let value = requiredPairingReceiptField(value) else {
            return nil
        }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]
        if let date = formatter.date(from: value) {
            return date
        }
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter.date(from: value)
    }

    private func validateSnapshots(
        snapshot: AcceptanceBundleLoadedSnapshot,
        pairingReceiptID: String,
        sessionID: String,
        targetRootURL: URL,
        bundleRootURL: URL
    ) throws {
        try validateCurrentSourceConsistency(snapshot: snapshot, bundleRootURL: bundleRootURL)
        guard let verifySnapshot = snapshot.sourceVerifyArtifact,
              sourceEvidenceTargetRootMatches(verifySnapshot.target_root, targetRootURL: targetRootURL),
              verifySnapshot.summary.files_verified >= 1,
              verifySnapshot.summary.error_findings == 0,
              verifySnapshot.summary.artifact_problems == 0 else {
            throw EvaluationError.invalidVerifyEvidence
        }
        guard let reportSnapshot = snapshot.sourceReportArtifact,
              sourceEvidenceTargetRootMatches(reportSnapshot.target_root, targetRootURL: targetRootURL),
              reportSnapshot.pairing.status == "paired_receipt_valid",
              !(reportSnapshot.pairing.receipt_id?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true),
              reportSnapshot.pairing.receipt_id == pairingReceiptID else {
            throw EvaluationError.invalidReportEvidence
        }
        guard let statusSnapshot = snapshot.sourceStatusArtifact,
              !statusSnapshot.profile_id.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
              !statusSnapshot.target_id.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
              sourceEvidenceTargetRootMatches(statusSnapshot.target_root, targetRootURL: targetRootURL),
              !statusSnapshot.overall.status.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
              !statusSnapshot.overall.target_status.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
              statusSnapshot.latest_session.id == sessionID,
              !statusSnapshot.latest_session.completeness_status.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
              statusSnapshot.latest_session.files_expected >= 0,
              statusSnapshot.latest_session.files_verified >= 0,
              statusSnapshot.latest_session.verification_errors >= 0,
              statusSnapshot.counts.artifact_problems >= 0,
              statusSnapshot.counts.network_transfers >= 0 else {
            throw EvaluationError.invalidStatusEvidence
        }
        guard let healthSnapshot = snapshot.sourceHealthArtifact,
              sourceEvidenceTargetRootMatches(healthSnapshot.target_root, targetRootURL: targetRootURL),
              healthSnapshot.summary.incomplete_sessions >= 0,
              healthSnapshot.summary.invalid_records >= 0,
              healthSnapshot.summary.artifact_problems >= 0,
              healthSnapshot.summary.target_drifts >= 0,
              healthSnapshot.summary.network_transfers >= 0,
              healthSnapshot.network_transfers?.contains(where: { transfer in
                  transfer.session_id == sessionID
                      && !transfer.status.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
              }) == true else {
            throw EvaluationError.invalidHealthEvidence
        }
    }

    private func sourceEvidenceTargetRootMatches(_ targetRoot: String, targetRootURL: URL) -> Bool {
        let evidencePath = targetRoot.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !evidencePath.isEmpty else {
            return false
        }
        return URL(fileURLWithPath: evidencePath).standardizedFileURL.path == targetRootURL.standardizedFileURL.path
    }

    private func validateCurrentSourceConsistency(
        snapshot: AcceptanceBundleLoadedSnapshot,
        bundleRootURL: URL
    ) throws {
        let consistency = try loadRequiredSourceConsistencyArtifact(
            snapshot: snapshot,
            bundleRootURL: bundleRootURL
        )
        let status = consistency.status.trimmingCharacters(in: .whitespacesAndNewlines)
        let mode = consistency.mode.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !status.isEmpty, !mode.isEmpty else {
            throw EvaluationError.missingCurrentSourceConsistencyEvidence
        }
        guard status == "pass", mode == "current_source_verified" else {
            throw EvaluationError.blockedCurrentSourceConsistency(mode.isEmpty ? status : mode)
        }
        let baselinePath = (
            consistency.baseline?.trimmingCharacters(in: .whitespacesAndNewlines)
                ?? snapshot.meta.evidence.source_consistency?.baseline?.trimmingCharacters(in: .whitespacesAndNewlines)
                ?? "source.baseline.json"
        )
        guard !baselinePath.isEmpty else {
            throw EvaluationError.missingRequiredArtifact("source.baseline.json")
        }
        try requireRegularArtifact(baselinePath, bundleRootURL: bundleRootURL)
        guard let transferSessionID = AcceptanceControlPlaneID.safeRawSegment(
            snapshot.sourceTransferArtifact?.session_id
        ),
            consistency.session_id == transferSessionID
        else {
            throw EvaluationError.blockedCurrentSourceConsistency("session_mismatch")
        }
    }

    private func loadRequiredSourceConsistencyArtifact(
        snapshot: AcceptanceBundleLoadedSnapshot,
        bundleRootURL: URL
    ) throws -> AcceptanceBundleSnapshot.SourceConsistencyEvidence {
        let relativePath = snapshot.meta.evidence.source_consistency?.output?.trimmingCharacters(
            in: .whitespacesAndNewlines
        ) ?? "source.consistency.json"
        guard !relativePath.isEmpty else {
            throw EvaluationError.missingCurrentSourceConsistencyEvidence
        }
        let data = try requireBundleArtifactData(relativePath, bundleRootURL: bundleRootURL)
        guard let consistency = try? JSONDecoder().decode(
            AcceptanceBundleSnapshot.SourceConsistencyEvidence.self,
            from: data
        ) else {
            throw EvaluationError.malformedArtifact(relativePath)
        }
        return consistency
    }

    private func validateOperatorEvidence(snapshot: AcceptanceBundleLoadedSnapshot) throws {
        if let detail = snapshot.installedAppCollectionProof.finalEvaluationCollectionDetail {
            throw EvaluationError.invalidInstalledAppCollection(detail)
        }
        for requirement in AcceptanceManualEvidenceRequirement.strictTwoMachineRequirements {
            if !snapshot.hasValidManualEvidence(requirement) {
                throw EvaluationError.missingOperatorEvidence(requirement.kind)
            }
        }
    }

    private func validateInstalledAppMachineFacts(
        snapshot: AcceptanceBundleLoadedSnapshot,
        bundleRootURL: URL
    ) throws {
        let sourceRelative = AcceptanceInstalledAppCollectionProofConstants.sourceMachineFactsArtifact
        let targetRelative = AcceptanceInstalledAppCollectionProofConstants.targetMachineFactsArtifact

        let sourceData = try requireBundleArtifactData(sourceRelative, bundleRootURL: bundleRootURL)
        let targetData = try requireBundleArtifactData(targetRelative, bundleRootURL: bundleRootURL)
        guard let sourceFacts = try? JSONDecoder().decode(MachineFactsArtifact.self, from: sourceData),
              sourceFacts.schema == "supermover.acceptance.machine_facts.v1",
              let sourceMachineID = sourceFacts.machine_id?.trimmingCharacters(in: CharacterSet.whitespacesAndNewlines),
              !sourceMachineID.isEmpty else {
            throw EvaluationError.malformedArtifact(sourceRelative)
        }
        guard let targetFacts = try? JSONDecoder().decode(MachineFactsArtifact.self, from: targetData),
              targetFacts.schema == "supermover.acceptance.machine_facts.v1",
              let targetMachineID = targetFacts.machine_id?.trimmingCharacters(in: CharacterSet.whitespacesAndNewlines),
              !targetMachineID.isEmpty else {
            throw EvaluationError.malformedArtifact(targetRelative)
        }
        if let detail = snapshot.installedAppCollectionProof.finalEvaluationCollectionDetail {
            throw EvaluationError.invalidInstalledAppCollection(detail)
        }
        guard sourceMachineID != targetMachineID else {
            throw EvaluationError.invalidInstalledAppCollection("source and target machine facts share machine_id=\(sourceMachineID)")
        }
        if let detail = snapshot.installedAppCollectionProof.finalEvaluationMachineFactsDetail {
            throw EvaluationError.invalidInstalledAppCollection(detail)
        }
        guard snapshot.installedAppCollectionProof.hasInstalledAppMachinePairProof else {
            throw EvaluationError.invalidInstalledAppCollection(
                "roles.source_pair/target machine_id do not match source.machine.json and target.machine.json"
            )
        }
    }

    private func validateInstalledAppBundleHandoffs(
        snapshot: AcceptanceBundleLoadedSnapshot
    ) throws {
        let proof = snapshot.installedAppCollectionProof
        if let detail = proof.finalEvaluationBundleHandoffDetail {
            throw EvaluationError.invalidInstalledAppCollection(detail)
        }
    }

    private func validateNotaryLogArtifact(
        _ artifact: AcceptanceBundleSnapshot.NotarizationArtifact?,
        machine: String,
        appAudit: AcceptanceBundleSnapshot.AppAuditArtifact?,
        bundleRootURL: URL
    ) throws {
        guard let auditPath = artifact?.audit?.path?.trimmingCharacters(in: .whitespacesAndNewlines),
              !auditPath.isEmpty,
              let appPath = appAudit?.app_path?.trimmingCharacters(in: .whitespacesAndNewlines),
              !appPath.isEmpty,
              auditPath == "\(appPath).notary/post-staple.audit.json" else {
            throw EvaluationError.invalidNotarizationEvidence(machine)
        }
        guard let notaryLogPath = artifact?.notary_log?.path?.trimmingCharacters(in: .whitespacesAndNewlines),
              !notaryLogPath.isEmpty else {
            throw EvaluationError.invalidNotarizationEvidence(machine)
        }
        guard AcceptanceInstalledAppReleaseEvidenceSummary.notarizationAuthModeIsReleaseReady(
            artifact?.auth_mode
        ) else {
            throw EvaluationError.invalidNotarizationEvidence(machine)
        }
        guard AcceptanceInstalledAppReleaseEvidenceSummary.notarizationSubmissionIDIsReleaseReady(
            artifact?.submission?.id
        ) else {
            throw EvaluationError.invalidNotarizationEvidence(machine)
        }
        guard artifact?.failure == nil else {
            throw EvaluationError.invalidNotarizationEvidence(machine)
        }
        guard notaryLogPath == "\(machine).notary-log.json" else {
            throw EvaluationError.invalidNotarizationEvidence(machine)
        }
        let notaryLogData = try requireBundleArtifactData(notaryLogPath, bundleRootURL: bundleRootURL)
        guard AcceptanceInstalledAppReleaseEvidenceSummary.acceptedNotaryLog(
            data: notaryLogData,
            submissionID: artifact?.submission?.id
        ) else {
            throw EvaluationError.invalidNotarizationEvidence(machine)
        }
    }

    private func validateInstalledAppReleaseEvidence(
        snapshot: AcceptanceBundleLoadedSnapshot
    ) throws {
        let releaseEvidence = snapshot.installedAppReleaseEvidence
        guard releaseEvidence.source.appAuditReady else {
            throw EvaluationError.blockedAppAudit("source")
        }
        guard releaseEvidence.target.appAuditReady else {
            throw EvaluationError.blockedAppAudit("target")
        }
        guard releaseEvidence.source.notarizationReady else {
            throw EvaluationError.invalidNotarizationEvidence("source")
        }
        guard releaseEvidence.target.notarizationReady else {
            throw EvaluationError.invalidNotarizationEvidence("target")
        }
    }

    private func requireRegularArtifact(_ relativePath: String, bundleRootURL: URL) throws {
        _ = try requireBundleArtifactData(relativePath, bundleRootURL: bundleRootURL)
    }

    private func requireBundleArtifactData(_ relativePath: String, bundleRootURL: URL) throws -> Data {
        do {
            return try artifactAccess.requiredArtifactData(
                relativePath: relativePath,
                bundleRootURL: bundleRootURL
            )
        } catch AcceptanceBundleArtifactAccess.AccessError.missingArtifact {
            throw EvaluationError.missingRequiredArtifact(relativePath)
        } catch AcceptanceBundleArtifactAccess.AccessError.unsafeArtifactPath,
                AcceptanceBundleArtifactAccess.AccessError.unreadableArtifact {
            throw EvaluationError.malformedArtifact(relativePath)
        } catch {
            throw EvaluationError.malformedArtifact(relativePath)
        }
    }

    private func requireTargetControlPlaneArtifact(
        relativePath: String,
        targetRootURL: URL,
        missingError: EvaluationError,
        invalidError: EvaluationError
    ) throws {
        _ = try requireTargetControlPlaneArtifactData(
            relativePath: relativePath,
            targetRootURL: targetRootURL,
            missingError: missingError,
            invalidError: invalidError
        )
    }

    private func requireTargetControlPlaneArtifactData(
        relativePath: String,
        targetRootURL: URL,
        missingError: EvaluationError,
        invalidError: EvaluationError
    ) throws -> Data {
        let url = try targetControlPlaneArtifactURL(
            relativePath: relativePath,
            targetRootURL: targetRootURL,
            invalidError: invalidError
        )
        guard fileManager.fileExists(atPath: url.path) else {
            throw missingError
        }
        guard try isRegularNonSymlinkFile(url) else {
            throw invalidError
        }
        do {
            return try Data(contentsOf: url)
        } catch {
            throw invalidError
        }
    }

    private func targetControlPlaneArtifactURL(
        relativePath: String,
        targetRootURL: URL,
        invalidError: EvaluationError
    ) throws -> URL {
        guard relativePath.hasPrefix(".supermover/") else {
            throw invalidError
        }
        let parts = relativePath.split(separator: "/", omittingEmptySubsequences: false).map(String.init)
        guard !parts.isEmpty else {
            throw invalidError
        }
        var current = targetRootURL
        for part in parts {
            guard !part.isEmpty, part != ".", part != ".." else {
                throw invalidError
            }
            current = current.appendingPathComponent(part)
            if fileManager.fileExists(atPath: current.path) {
                let values = try current.resourceValues(forKeys: [.isSymbolicLinkKey])
                if values.isSymbolicLink == true {
                    throw invalidError
                }
            }
        }
        let rootPath = targetRootURL.standardizedFileURL.path
        let artifactPath = current.standardizedFileURL.path
        guard artifactPath.hasPrefix(rootPath + "/") else {
            throw invalidError
        }
        return current
    }

    private func isRegularNonSymlinkFile(_ url: URL) throws -> Bool {
        let values = try url.resourceValues(forKeys: [.isRegularFileKey, .isSymbolicLinkKey, .linkCountKey])
        return values.isRegularFile == true && values.isSymbolicLink != true && (values.linkCount ?? 1) == 1
    }
}
