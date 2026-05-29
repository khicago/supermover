import Foundation

struct AcceptanceBundleArtifactWriter {
    enum WriteError: LocalizedError, Equatable {
        case missingBundlePath
        case missingProfilePath
        case missingLocalPairingReceiptPath
        case missingServeVerificationCode
        case missingPairingTargetAddress
        case missingPairingVerificationCode
        case missingPairingReceiptID
        case missingSessionID
        case missingReceiverAddress
        case missingLocalPairingReceiptArtifact(String)
        case invalidLocalPairingReceiptArtifact(String)
        case invalidPhase(Int)
        case missingServeReadiness
        case missingDiscoveryBrowse
        case missingDiscoveryAdvertise

        var errorDescription: String? {
            switch self {
            case .missingBundlePath:
                return "Choose an acceptance bundle directory first."
            case .missingProfilePath:
                return "Select a migration config file first."
            case .missingLocalPairingReceiptPath:
                return "Current migration config does not point to a durable local pairing receipt yet."
            case .missingServeVerificationCode:
                return "Current target serve readiness does not expose a pairing verification code yet."
            case .missingPairingTargetAddress:
                return "Pairing target address is required for source pair evidence."
            case .missingPairingVerificationCode:
                return "Pairing verification code is required for source pair evidence."
            case .missingPairingReceiptID:
                return "Current migration config does not contain a durable pairing_receipt_id yet."
            case .missingSessionID:
                return "Provide an explicit session id before writing source transfer evidence."
            case .missingReceiverAddress:
                return "Current migration config network receiver URL is required for source transfer evidence."
            case let .missingLocalPairingReceiptArtifact(path):
                return "Current migration config pairing receipt artifact is missing at \(path)."
            case let .invalidLocalPairingReceiptArtifact(detail):
                return "Current migration config pairing receipt artifact is invalid: \(detail)"
            case let .invalidPhase(phase):
                return "Serve phase must be a positive integer, got \(phase)."
            case .missingServeReadiness:
                return "Current serve readiness snapshot is unavailable."
            case .missingDiscoveryBrowse:
                return "Current discovery browse snapshot is unavailable."
            case .missingDiscoveryAdvertise:
                return "Current discovery advertise snapshot is unavailable."
            }
        }
    }

    struct SourcePairRecord {
        let bundleRootURL: URL
        let profilePath: String
        let targetAddress: String
        let verificationCode: String
        let localPairingReceiptPath: String
        let pairingReceiptID: String
        let pairStdout: String?
    }

    struct SourceTransferRecord {
        let bundleRootURL: URL
        let profilePath: String
        let sessionID: String
        let targetAddress: String
        let receiverAddress: String
        let targetMode: String
        let sourceBaselineURL: URL?
        let sourceConsistencyRawJSON: String?
        let verifyArtifactPath: String?
        let statusArtifactPath: String?
        let reportArtifactPath: String?
        let healthArtifactPath: String?
        let pushStdout: String?
    }

    struct TargetImportRecord {
        let bundleRootURL: URL
        let profilePath: String
        let pairingReceiptID: String
        let adoptedStdout: String?
    }

    struct StructuredJSONArtifactRecord {
        let bundleRootURL: URL
        let fileName: String
        let rawJSON: String
    }

    struct EvaluationRecord {
        let bundleRootURL: URL
        let pairingReceiptID: String
        let sessionID: String
        let targetRoot: String
        let requireOperatorEvidence: Bool
    }

    struct ServePhaseRecord {
        let bundleRootURL: URL
        let profilePath: String
        let phase: Int
        let readiness: ServeReadinessSnapshot
    }

    struct DiscoveryBrowseRecord {
        let bundleRootURL: URL
        let snapshot: DiscoveryBrowseSnapshot
    }

    struct DiscoveryAdvertiseRecord {
        let bundleRootURL: URL
        let profilePath: String
        let snapshot: DiscoveryAdvertiseSnapshot
    }

    let fileManager: FileManager
    let artifactAccess: AcceptanceBundleArtifactAccess
    let metaStore: AcceptanceBundleMetaStore
    let machineIdentityResolver: AcceptanceMachineIdentityResolver

    init(
        fileManager: FileManager = .default,
        metaStore: AcceptanceBundleMetaStore = AcceptanceBundleMetaStore(),
        machineIdentityResolver: AcceptanceMachineIdentityResolver = AcceptanceMachineIdentityResolver()
    ) {
        self.fileManager = fileManager
        artifactAccess = AcceptanceBundleArtifactAccess(fileManager: fileManager)
        self.metaStore = metaStore
        self.machineIdentityResolver = machineIdentityResolver
    }

    func writeSourcePair(_ record: SourcePairRecord) throws {
        try ensureBundle(record.bundleRootURL)
        try preflightArtifactOutputs(
            bundleRootURL: record.bundleRootURL,
            fileNames: [
                "source.machine.json",
                "exported-receipts/\(record.pairingReceiptID).json",
                "source.pair.json",
            ] + optionalTextArtifactNames([
                ("source.pair.txt", record.pairStdout),
            ])
        )
        let machineIdentity = try writeCanonicalMachineFactsArtifact(
            bundleRootURL: record.bundleRootURL,
            machine: "source"
        )
        let receiptPath = try stagePairingReceiptArtifact(
            bundleRootURL: record.bundleRootURL,
            localPairingReceiptPath: record.localPairingReceiptPath,
            pairingReceiptID: record.pairingReceiptID
        )
        let pairArtifactPath = try recordTextArtifact(
            bundleRootURL: record.bundleRootURL,
            fileName: "source.pair.txt",
            contents: record.pairStdout
        )
        try writeJSONObject(
            [
                "profile": record.profilePath,
                "target_address": record.targetAddress,
                "verification_code": record.verificationCode,
                "pairing_receipt_id": record.pairingReceiptID,
                "receipt_path": receiptPath,
            ],
            toArtifact: "source.pair.json",
            bundleRootURL: record.bundleRootURL
        )
        try mutateBundleMeta(record.bundleRootURL) { document in
            var document = try AcceptanceBundleMetaDocument(document)
            document.recordMachineIdentity(
                machine: "source",
                role: "source_pair",
                profilePath: record.profilePath,
                identity: machineIdentity
            )
            var sourcePair = document.evidenceSourcePair
            sourcePair["pairing_receipt_id"] = record.pairingReceiptID
            sourcePair["receipt_path"] = receiptPath
            sourcePair["target_address"] = record.targetAddress
            sourcePair["output"] = "source.pair.json"
            if let pairArtifactPath {
                sourcePair["pair"] = pairArtifactPath
            }
            document.evidenceSourcePair = sourcePair
            return document.object
        }
    }

    func writeSourceTransfer(_ record: SourceTransferRecord) throws {
        try ensureBundle(record.bundleRootURL)
        try preflightArtifactOutputs(
            bundleRootURL: record.bundleRootURL,
            fileNames: [
                "source.machine.json",
                "source.consistency.json",
                "source.transfer.json",
            ] + optionalTextArtifactNames([
                ("source.network-push.txt", record.pushStdout),
            ]) + optionalCopiedArtifactNames([
                ("source.baseline.json", record.sourceBaselineURL),
            ])
        )
        let machineIdentity = try writeCanonicalMachineFactsArtifact(
            bundleRootURL: record.bundleRootURL,
            machine: "source"
        )
        let pushArtifactPath = try recordTextArtifact(
            bundleRootURL: record.bundleRootURL,
            fileName: "source.network-push.txt",
            contents: record.pushStdout
        )
        let baselineArtifactPath = try recordCopiedArtifact(
            bundleRootURL: record.bundleRootURL,
            fileName: "source.baseline.json",
            sourceURL: record.sourceBaselineURL
        )
        if baselineArtifactPath == nil {
            try removeArtifactIfPresent(
                bundleRootURL: record.bundleRootURL,
                fileName: "source.baseline.json"
            )
        }
        let sourceConsistencyState = try writeSourceConsistencyArtifact(
            bundleRootURL: record.bundleRootURL,
            profilePath: record.profilePath,
            sessionID: record.sessionID,
            rawJSON: record.sourceConsistencyRawJSON,
            hasBaselineArtifact: baselineArtifactPath != nil
        )
        try writeJSONObject(
            [
                "profile": record.profilePath,
                "session_id": record.sessionID,
                "target_address": record.targetAddress,
                "receiver_address": record.receiverAddress,
                "target_mode": record.targetMode,
            ],
            toArtifact: "source.transfer.json",
            bundleRootURL: record.bundleRootURL
        )
        try mutateBundleMeta(record.bundleRootURL) { document in
            var document = try AcceptanceBundleMetaDocument(document)
            document.recordMachineIdentity(
                machine: "source",
                role: "source_transfer",
                profilePath: record.profilePath,
                identity: machineIdentity
            )
            var sourceTransfer = document.evidenceSourceTransfer
            sourceTransfer["session_id"] = record.sessionID
            sourceTransfer["receiver_address"] = record.receiverAddress
            sourceTransfer["output"] = "source.transfer.json"
            if let verifyArtifactPath = record.verifyArtifactPath {
                sourceTransfer["verify"] = verifyArtifactPath
            }
            if let statusArtifactPath = record.statusArtifactPath {
                sourceTransfer["status"] = statusArtifactPath
            }
            if let reportArtifactPath = record.reportArtifactPath {
                sourceTransfer["report"] = reportArtifactPath
            }
            if let healthArtifactPath = record.healthArtifactPath {
                sourceTransfer["health"] = healthArtifactPath
            }
            if let pushArtifactPath {
                sourceTransfer["push"] = pushArtifactPath
            }
            document.evidenceSourceTransfer = sourceTransfer
            document.evidenceSourceConsistency = [
                "output": "source.consistency.json",
                "status": sourceConsistencyState.status,
                "mode": sourceConsistencyState.mode,
            ]
            if let baselineArtifactPath {
                document.evidenceSourceConsistency["baseline"] = baselineArtifactPath
            }
            return document.object
        }
    }

    func writeTargetImport(_ record: TargetImportRecord) throws {
        try ensureBundle(record.bundleRootURL)
        try preflightArtifactOutputs(
            bundleRootURL: record.bundleRootURL,
            fileNames: [
                "target.machine.json",
            ] + optionalTextArtifactNames([
                ("target.adopt-pairing.txt", record.adoptedStdout),
            ])
        )
        let machineIdentity = try writeCanonicalMachineFactsArtifact(
            bundleRootURL: record.bundleRootURL,
            machine: "target"
        )
        let adoptedArtifactPath = try recordTextArtifact(
            bundleRootURL: record.bundleRootURL,
            fileName: "target.adopt-pairing.txt",
            contents: record.adoptedStdout
        )
        try mutateBundleMeta(record.bundleRootURL) { document in
            var document = try AcceptanceBundleMetaDocument(document)
            document.recordMachineIdentity(
                machine: "target",
                role: "target_import",
                profilePath: record.profilePath,
                identity: machineIdentity
            )
            var targetImport: [String: Any] = [
                "pairing_receipt_id": record.pairingReceiptID,
            ]
            if let adoptedArtifactPath {
                targetImport["adopted"] = adoptedArtifactPath
            }
            document.evidenceTargetImport = targetImport
            return document.object
        }
    }

    func writeStructuredJSONArtifact(_ record: StructuredJSONArtifactRecord) throws -> String {
        try ensureBundle(record.bundleRootURL)
        let trimmed = record.rawJSON.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            throw AcceptanceBundleMetaStore.MutationError.invalidRecord("Structured JSON artifact is empty for \(record.fileName).")
        }
        guard let data = trimmed.data(using: .utf8),
              (try? JSONSerialization.jsonObject(with: data)) != nil else {
            throw AcceptanceBundleMetaStore.MutationError.invalidRecord("Structured JSON artifact is malformed for \(record.fileName).")
        }
        let url = try writableArtifactURL(
            bundleRootURL: record.bundleRootURL,
            fileName: record.fileName
        )
        try data.write(to: url, options: Data.WritingOptions.atomic)
        return record.fileName
    }

    private func writeSourceConsistencyArtifact(
        bundleRootURL: URL,
        profilePath: String,
        sessionID: String,
        rawJSON: String?,
        hasBaselineArtifact: Bool
    ) throws -> (status: String, mode: String) {
        let trimmed = rawJSON?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if let data = trimmed.data(using: .utf8),
           let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
           let status = (object["status"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines),
           let mode = (object["mode"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines),
           !status.isEmpty,
           !mode.isEmpty {
            let proofSessionID = (object["session_id"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            if !proofSessionID.isEmpty, proofSessionID != sessionID {
                try writeJSONObject(
                    [
                        "schema": "supermover.acceptance.current_source_consistency.v1",
                        "status": "blocked",
                        "mode": "session_mismatch",
                        "profile": profilePath,
                        "session_id": sessionID,
                        "detail": "Current-source proof session_id does not match the transfer session being written into this acceptance bundle.",
                    ],
                    toArtifact: "source.consistency.json",
                    bundleRootURL: bundleRootURL
                )
                return ("blocked", "session_mismatch")
            }
            if status == "pass", mode == "current_source_verified", !hasBaselineArtifact {
                try writeJSONObject(
                    [
                        "schema": "supermover.acceptance.current_source_consistency.v1",
                        "status": "blocked",
                        "mode": "baseline_missing",
                        "profile": profilePath,
                        "session_id": sessionID,
                        "detail": "Current-source pass proof requires the transfer baseline artifact captured during network push.",
                    ],
                    toArtifact: "source.consistency.json",
                    bundleRootURL: bundleRootURL
                )
                return ("blocked", "baseline_missing")
            }
            let outputURL = try writableArtifactURL(
                bundleRootURL: bundleRootURL,
                fileName: "source.consistency.json"
            )
            try data.write(to: outputURL, options: .atomic)
            return (status, mode)
        }
        try writeJSONObject(
            [
                "schema": "supermover.acceptance.current_source_consistency.v1",
                "status": "blocked",
                "mode": "app_first_replay_not_verified",
                "profile": profilePath,
                "session_id": sessionID,
                "detail": "App-first Write Transfer replay has not yet regenerated a current-source baseline/proof; use CLI source-consistency evidence for final acceptance.",
            ],
            toArtifact: "source.consistency.json",
            bundleRootURL: bundleRootURL
        )
        return ("blocked", "app_first_replay_not_verified")
    }

    func writeEvaluation(_ record: EvaluationRecord) throws {
        try ensureBundle(record.bundleRootURL)
        try preflightArtifactOutputs(
            bundleRootURL: record.bundleRootURL,
            fileNames: ["evaluation.json"]
        )
        try writeJSONObject(
            [
                "schema": "supermover.acceptance.two_machine.v1",
                "status": "evidence_collected",
                "pairing_receipt_id": record.pairingReceiptID,
                "session_id": record.sessionID,
                "target_root": record.targetRoot,
                "require_operator_evidence": record.requireOperatorEvidence,
            ],
            toArtifact: "evaluation.json",
            bundleRootURL: record.bundleRootURL
        )
        try mutateBundleMeta(record.bundleRootURL) { document in
            var document = try AcceptanceBundleMetaDocument(document)
            document.object["status"] = "evidence_collected"
            document.evidenceEvaluation = [
                "pairing_receipt_id": record.pairingReceiptID,
                "session_id": record.sessionID,
                "target_root": record.targetRoot,
                "output": "evaluation.json",
                "require_operator_evidence": record.requireOperatorEvidence,
            ]
            return document.object
        }
    }

    private func recordTextArtifact(
        bundleRootURL: URL,
        fileName: String,
        contents: String?
    ) throws -> String? {
        let trimmed = contents?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !trimmed.isEmpty else {
            return nil
        }
        let url = try writableArtifactURL(bundleRootURL: bundleRootURL, fileName: fileName)
        try Data(trimmed.utf8).write(to: url, options: Data.WritingOptions.atomic)
        return fileName
    }

    private func recordCopiedArtifact(
        bundleRootURL: URL,
        fileName: String,
        sourceURL: URL?
    ) throws -> String? {
        guard let sourceURL else {
            return nil
        }
        guard fileManager.fileExists(atPath: sourceURL.path) else {
            return nil
        }
        let destinationURL = try writableArtifactURL(bundleRootURL: bundleRootURL, fileName: fileName)
        try replaceCopiedArtifact(sourceURL: sourceURL, destinationURL: destinationURL)
        return fileName
    }

    private func removeArtifactIfPresent(
        bundleRootURL: URL,
        fileName: String
    ) throws {
        let url = bundleRootURL.appendingPathComponent(fileName)
        guard fileManager.fileExists(atPath: url.path) else {
            return
        }
        try fileManager.removeItem(at: url)
    }

    private func writeCanonicalMachineFactsArtifact(
        bundleRootURL: URL,
        machine: String
    ) throws -> AcceptanceMachineIdentity {
        let identity = machineIdentityResolver.resolve()
        try writeJSONObject(
            machineFactsJSONObject(identity),
            toArtifact: "\(machine).machine.json",
            bundleRootURL: bundleRootURL
        )
        return identity
    }

    func writeServePhase(_ record: ServePhaseRecord) throws {
        try ensureBundle(record.bundleRootURL)
        guard record.phase > 0 else {
            throw WriteError.invalidPhase(record.phase)
        }
        if serveModeRequiresVerificationCode(record.readiness.mode),
           normalizedVerificationCode(record.readiness.verification_code) == nil {
            throw WriteError.missingServeVerificationCode
        }
        try preflightArtifactOutputs(
            bundleRootURL: record.bundleRootURL,
            fileNames: [
                "target.machine.json",
                "target.ready.phase-\(record.phase).json",
                "target.ready.json",
            ]
        )
        let machineIdentity = try writeCanonicalMachineFactsArtifact(
            bundleRootURL: record.bundleRootURL,
            machine: "target"
        )
        let fileName = "target.ready.phase-\(record.phase).json"
        let readinessObject = serveReadinessJSONObject(record.readiness)
        try writeJSONObject(
            readinessObject,
            toArtifact: fileName,
            bundleRootURL: record.bundleRootURL
        )
        try writeJSONObject(
            currentTargetReadyJSONObject(
                snapshot: record.readiness,
                profilePath: record.profilePath
            ),
            toArtifact: "target.ready.json",
            bundleRootURL: record.bundleRootURL
        )
        try mutateBundleMeta(record.bundleRootURL) { document in
            var document = try AcceptanceBundleMetaDocument(document)
            document.recordMachineIdentity(
                machine: "target",
                role: "target",
                profilePath: record.profilePath,
                identity: machineIdentity
            )
            var phases = document.targetServePhases
            phases.removeAll { $0["phase"] as? Int == record.phase }
            phases.append([
                "phase": record.phase,
                "ready": fileName,
            ])
            phases.sort { lhs, rhs in
                (lhs["phase"] as? Int ?? 0) < (rhs["phase"] as? Int ?? 0)
            }
            document.targetServePhases = phases
            document.targetReadyEvidence = [
                "address": record.readiness.address,
                "verification_code": normalizedVerificationCode(record.readiness.verification_code) ?? "",
                "mode": record.readiness.mode,
            ]
            return document.object
        }
    }

    func writeDiscoveryBrowse(_ record: DiscoveryBrowseRecord) throws {
        try ensureBundle(record.bundleRootURL)
        try preflightArtifactOutputs(
            bundleRootURL: record.bundleRootURL,
            fileNames: [
                "source.machine.json",
                "source.browse.json",
            ]
        )
        let machineIdentity = try writeCanonicalMachineFactsArtifact(
            bundleRootURL: record.bundleRootURL,
            machine: "source"
        )
        try writeJSONObject(
            discoveryBrowseJSONObject(record.snapshot),
            toArtifact: "source.browse.json",
            bundleRootURL: record.bundleRootURL
        )
        try mutateBundleMeta(record.bundleRootURL) { document in
            var document = try AcceptanceBundleMetaDocument(document)
            document.recordMachineIdentity(
                machine: "source",
                role: "source_browse",
                profilePath: "-",
                identity: machineIdentity
            )
            var discovery = document.discoveryEvidence
            discovery["source_browse"] = [
                "output": "source.browse.json",
                "trusted": false,
            ]
            document.discoveryEvidence = discovery
            return document.object
        }
    }

    func writeDiscoveryAdvertise(_ record: DiscoveryAdvertiseRecord) throws {
        try ensureBundle(record.bundleRootURL)
        try preflightArtifactOutputs(
            bundleRootURL: record.bundleRootURL,
            fileNames: [
                "target.machine.json",
                "target.advertise.json",
            ]
        )
        let machineIdentity = try writeCanonicalMachineFactsArtifact(
            bundleRootURL: record.bundleRootURL,
            machine: "target"
        )
        try writeJSONObject(
            discoveryAdvertiseJSONObject(record.snapshot),
            toArtifact: "target.advertise.json",
            bundleRootURL: record.bundleRootURL
        )
        try mutateBundleMeta(record.bundleRootURL) { document in
            var document = try AcceptanceBundleMetaDocument(document)
            document.recordMachineIdentity(
                machine: "target",
                role: "target_advertise",
                profilePath: record.profilePath,
                identity: machineIdentity
            )
            var discovery = document.discoveryEvidence
            discovery["target_advertise"] = [
                "output": "target.advertise.json",
                "trusted": false,
            ]
            document.discoveryEvidence = discovery
            return document.object
        }
    }

    private func ensureBundle(_ bundleRootURL: URL) throws {
        do {
            _ = try artifactAccess.metaURL(bundleRootURL: bundleRootURL)
        } catch let AcceptanceBundleArtifactAccess.AccessError.invalidBundleRoot(path) {
            throw AcceptanceBundleMetaStore.MutationError.invalidBundleRoot(
                URL(fileURLWithPath: path, isDirectory: true)
            )
        } catch let AcceptanceBundleArtifactAccess.AccessError.symlinkRejected(path) {
            throw AcceptanceBundleMetaStore.MutationError.symlinkRejected(URL(fileURLWithPath: path))
        } catch AcceptanceBundleArtifactAccess.AccessError.missingArtifact {
            throw AcceptanceBundleMetaStore.MutationError.missingMeta(
                bundleRootURL.appendingPathComponent("meta.json")
            )
        } catch let AcceptanceBundleArtifactAccess.AccessError.unsafeOutputArtifact(path) {
            throw AcceptanceBundleMetaStore.MutationError.unsafeOutputArtifact(URL(fileURLWithPath: path))
        } catch {
            throw AcceptanceBundleMetaStore.MutationError.invalidBundleRoot(bundleRootURL)
        }
    }

    private func stagePairingReceiptArtifact(
        bundleRootURL: URL,
        localPairingReceiptPath: String,
        pairingReceiptID: String
    ) throws -> String {
        let trimmed = localPairingReceiptPath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            throw WriteError.missingLocalPairingReceiptPath
        }
        let sourceURL = URL(fileURLWithPath: trimmed)
        var isDirectory: ObjCBool = false
        guard fileManager.fileExists(atPath: sourceURL.path, isDirectory: &isDirectory),
              !isDirectory.boolValue else {
            throw WriteError.missingLocalPairingReceiptArtifact(sourceURL.path)
        }
        let sourceValues = try sourceURL.resourceValues(forKeys: [.isSymbolicLinkKey])
        if sourceValues.isSymbolicLink == true {
            throw WriteError.invalidLocalPairingReceiptArtifact(
                "pairing receipt path resolves through a symlink: \(sourceURL.path)"
            )
        }

        let data: Data
        do {
            data = try Data(contentsOf: sourceURL)
        } catch {
            throw WriteError.missingLocalPairingReceiptArtifact(sourceURL.path)
        }
        try validatePairingReceiptData(
            data,
            expectedPairingReceiptID: pairingReceiptID,
            sourcePath: sourceURL.path
        )

        let directoryURL = try artifactAccess.artifactURL(
            relativePath: "exported-receipts",
            bundleRootURL: bundleRootURL
        )
        try fileManager.createDirectory(at: directoryURL, withIntermediateDirectories: true)

        let relativePath = "exported-receipts/\(pairingReceiptID).json"
        let destinationURL = try writableArtifactURL(
            bundleRootURL: bundleRootURL,
            fileName: relativePath
        )
        try data.write(to: destinationURL, options: .atomic)
        return relativePath
    }

    private func validatePairingReceiptData(
        _ data: Data,
        expectedPairingReceiptID: String,
        sourcePath: String
    ) throws {
        let object: Any
        do {
            object = try JSONSerialization.jsonObject(with: data)
        } catch {
            throw WriteError.invalidLocalPairingReceiptArtifact(
                "pairing receipt is malformed at \(sourcePath)"
            )
        }
        guard let dictionary = object as? [String: Any] else {
            throw WriteError.invalidLocalPairingReceiptArtifact(
                "pairing receipt is not a JSON object at \(sourcePath)"
            )
        }
        let receiptID = (dictionary["id"] as? String)?
            .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !receiptID.isEmpty else {
            throw WriteError.invalidLocalPairingReceiptArtifact(
                "pairing receipt is missing id at \(sourcePath)"
            )
        }
        guard receiptID == expectedPairingReceiptID else {
            throw WriteError.invalidLocalPairingReceiptArtifact(
                "pairing receipt id \(receiptID) does not match profile pairing_receipt_id \(expectedPairingReceiptID)"
            )
        }
    }

    private func preflightArtifactOutputs(
        bundleRootURL: URL,
        fileNames: [String]
    ) throws {
        for fileName in fileNames {
            _ = try writableArtifactURL(bundleRootURL: bundleRootURL, fileName: fileName)
        }
    }

    private func optionalTextArtifactNames(_ artifacts: [(String, String?)]) -> [String] {
        artifacts.compactMap { fileName, contents in
            let trimmed = contents?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            return trimmed.isEmpty ? nil : fileName
        }
    }

    private func optionalCopiedArtifactNames(_ artifacts: [(String, URL?)]) -> [String] {
        artifacts.compactMap { fileName, sourceURL in
            guard let sourceURL, fileManager.fileExists(atPath: sourceURL.path) else {
                return nil
            }
            return fileName
        }
    }

    private func writableArtifactURL(
        bundleRootURL: URL,
        fileName: String
    ) throws -> URL {
        do {
            return try artifactAccess.writableArtifactURL(
                relativePath: fileName,
                bundleRootURL: bundleRootURL
            )
        } catch let AcceptanceBundleArtifactAccess.AccessError.invalidBundleRoot(path) {
            throw AcceptanceBundleMetaStore.MutationError.invalidBundleRoot(
                URL(fileURLWithPath: path, isDirectory: true)
            )
        } catch let AcceptanceBundleArtifactAccess.AccessError.symlinkRejected(path) {
            throw AcceptanceBundleMetaStore.MutationError.symlinkRejected(URL(fileURLWithPath: path))
        } catch let AcceptanceBundleArtifactAccess.AccessError.unsafeArtifactPath(path) {
            throw AcceptanceBundleMetaStore.MutationError.invalidRecord(
                "Acceptance bundle artifact path is unsafe: \(path)"
            )
        } catch let AcceptanceBundleArtifactAccess.AccessError.unsafeOutputArtifact(path) {
            throw AcceptanceBundleMetaStore.MutationError.unsafeOutputArtifact(URL(fileURLWithPath: path))
        } catch let AcceptanceBundleArtifactAccess.AccessError.unreadableArtifact(path) {
            throw AcceptanceBundleMetaStore.MutationError.unsafeOutputArtifact(
                bundleRootURL.appendingPathComponent(path)
            )
        } catch let AcceptanceBundleArtifactAccess.AccessError.missingArtifact(path) {
            throw AcceptanceBundleMetaStore.MutationError.invalidRecord(
                "Acceptance bundle artifact path is missing: \(path)"
            )
        }
    }

    private func replaceCopiedArtifact(sourceURL: URL, destinationURL: URL) throws {
        let tempURL = destinationURL.deletingLastPathComponent().appendingPathComponent(
            ".\(UUID().uuidString).\(destinationURL.lastPathComponent)",
            isDirectory: false
        )
        do {
            try fileManager.copyItem(at: sourceURL, to: tempURL)
            if fileManager.fileExists(atPath: destinationURL.path) {
                _ = try fileManager.replaceItemAt(destinationURL, withItemAt: tempURL)
            } else {
                try fileManager.moveItem(at: tempURL, to: destinationURL)
            }
        } catch {
            try? fileManager.removeItem(at: tempURL)
            throw error
        }
    }

    private func writeJSONObject(
        _ object: [String: Any],
        toArtifact fileName: String,
        bundleRootURL: URL
    ) throws {
        let data = try JSONSerialization.data(
            withJSONObject: object,
            options: [.prettyPrinted, .sortedKeys]
        )
        let url = try writableArtifactURL(bundleRootURL: bundleRootURL, fileName: fileName)
        try data.write(to: url, options: Data.WritingOptions.atomic)
    }

    private func mutateBundleMeta(
        _ bundleRootURL: URL,
        mutate: ([String: Any]) throws -> [String: Any]
    ) throws {
        try metaStore.withMutableMetaDocument(bundleRootURL: bundleRootURL) { document in
            document = try mutate(document)
        }
    }

    private func normalizedVerificationCode(_ value: String?) -> String? {
        let trimmed = value?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return trimmed.isEmpty ? nil : trimmed
    }

    private func serveModeRequiresVerificationCode(_ mode: String) -> Bool {
        let normalized = mode.trimmingCharacters(in: .whitespacesAndNewlines)
        return normalized == "pairing" || normalized == "pairing-only"
    }

    private func currentTargetReadyJSONObject(
        snapshot: ServeReadinessSnapshot,
        profilePath: String
    ) -> [String: Any] {
        var object = serveReadinessJSONObject(snapshot)
        object["profile"] = profilePath
        object["address"] = snapshot.address
        object["verification_code"] = normalizedVerificationCode(snapshot.verification_code) ?? ""
        object["mode"] = snapshot.mode
        return object
    }

    private func serveReadinessJSONObject(_ snapshot: ServeReadinessSnapshot) -> [String: Any] {
        var object: [String: Any] = [
            "address": snapshot.address,
            "mode": snapshot.mode,
            "trusted": snapshot.trusted,
            "transfer": snapshot.transfer,
        ]
        if let verification = snapshot.verification_code {
            object["verification_code"] = verification
        }
        if let receiverAddress = snapshot.receiver_address {
            object["receiver_address"] = receiverAddress
        }
        if let receiverRoutes = snapshot.receiver_routes {
            object["receiver_routes"] = receiverRoutes
        }
        if let pushNetwork = snapshot.push_network {
            object["push_network"] = pushNetwork
        }
        if let expiresAt = snapshot.expires_at {
            object["expires_at"] = expiresAt
        }
        return object
    }

    private func discoveryBrowseJSONObject(_ snapshot: DiscoveryBrowseSnapshot) -> [String: Any] {
        [
            "source": snapshot.source,
            "listen": snapshot.listen,
            "candidate_count": snapshot.candidate_count,
            "invalid_packets": snapshot.invalid_packets,
            "trusted": snapshot.trusted,
            "candidates": snapshot.candidates.map { candidate in
                var object: [String: Any] = [
                    "hint": [
                        "address": candidate.hint.address,
                        "advertisement": [
                            "service_type": candidate.hint.advertisement.service_type,
                            "protocol_version": candidate.hint.advertisement.protocol_version,
                            "ephemeral_nonce": candidate.hint.advertisement.ephemeral_nonce,
                            "capability_flags": candidate.hint.advertisement.capability_flags,
                        ],
                        "seen_at": candidate.hint.seen_at,
                        "expires_at": candidate.hint.expires_at,
                        "trusted": candidate.hint.trusted,
                    ],
                    "class": candidate.classification,
                    "duplicate_count": candidate.duplicate_count,
                ]
                if let ambiguity = candidate.ambiguity_reasons {
                    object["ambiguity_reasons"] = ambiguity
                }
                return object
            },
        ]
    }

    private func discoveryAdvertiseJSONObject(_ snapshot: DiscoveryAdvertiseSnapshot) -> [String: Any] {
        [
            "status": snapshot.status,
            "listen": snapshot.listen,
            "destination": snapshot.destination,
            "service_type": snapshot.service_type,
            "protocol_version": snapshot.protocol_version,
            "ephemeral_nonce": snapshot.ephemeral_nonce,
            "capability_flags": snapshot.capability_flags,
            "trusted": snapshot.trusted,
            "duration": snapshot.duration,
            "interval": snapshot.interval,
        ]
    }

    private func machineFactsJSONObject(_ identity: AcceptanceMachineIdentity) -> [String: Any] {
        [
            "schema": AcceptanceInstalledAppCollectionProofConstants.machineFactsSchema,
            "machine_id": identity.machineID,
            "machine_label": identity.machineLabel ?? NSNull(),
        ]
    }
}

private struct AcceptanceBundleMetaDocument {
    var object: [String: Any]

    init(_ object: [String: Any]) throws {
        guard object["schema"] != nil else {
            throw AcceptanceBundleMetaStore.MutationError.invalidRecord("Acceptance bundle meta.json is missing schema.")
        }
        self.object = object
    }

    var evidence: [String: Any] {
        get { object["evidence"] as? [String: Any] ?? [:] }
        set { object["evidence"] = newValue }
    }

    var evidenceSourcePair: [String: Any] {
        get { evidence["source_pair"] as? [String: Any] ?? [:] }
        set {
            var evidence = self.evidence
            evidence["source_pair"] = newValue
            self.evidence = evidence
        }
    }

    var evidenceSourceTransfer: [String: Any] {
        get { evidence["source_transfer"] as? [String: Any] ?? [:] }
        set {
            var evidence = self.evidence
            evidence["source_transfer"] = newValue
            self.evidence = evidence
        }
    }

    var evidenceSourceConsistency: [String: Any] {
        get { evidence["source_consistency"] as? [String: Any] ?? [:] }
        set {
            var evidence = self.evidence
            evidence["source_consistency"] = newValue
            self.evidence = evidence
        }
    }

    var evidenceTargetImport: [String: Any] {
        get { evidence["target_import"] as? [String: Any] ?? [:] }
        set {
            var evidence = self.evidence
            evidence["target_import"] = newValue
            self.evidence = evidence
        }
    }

    var targetServePhases: [[String: Any]] {
        get { evidence["target_serve_phases"] as? [[String: Any]] ?? [] }
        set {
            var evidence = self.evidence
            evidence["target_serve_phases"] = newValue
            self.evidence = evidence
        }
    }

    var discoveryEvidence: [String: Any] {
        get { evidence["discovery"] as? [String: Any] ?? [:] }
        set {
            var evidence = self.evidence
            evidence["discovery"] = newValue
            self.evidence = evidence
        }
    }

    var evidenceEvaluation: [String: Any] {
        get { evidence["evaluation"] as? [String: Any] ?? [:] }
        set {
            var evidence = self.evidence
            evidence["evaluation"] = newValue
            self.evidence = evidence
        }
    }

    var targetReadyEvidence: [String: Any] {
        get { evidence["target_ready"] as? [String: Any] ?? [:] }
        set {
            var evidence = self.evidence
            evidence["target_ready"] = newValue
            self.evidence = evidence
        }
    }

    mutating func recordMachineIdentity(
        machine: String,
        role: String,
        profilePath: String,
        identity: AcceptanceMachineIdentity
    ) {
        var roles = object["roles"] as? [String: Any] ?? [:]
        let roleRecord: [String: Any] = [
            "profile": profilePath,
            "status": "recorded",
            "machine_id": identity.machineID,
            "machine_label": identity.machineLabel ?? NSNull(),
        ]
        roles[role] = roleRecord
        object["roles"] = roles

        var evidence = self.evidence
        var machineFacts = evidence["machine_facts"] as? [String: Any] ?? [:]
        let machineFactsRecord: [String: Any] = [
            "output": "\(machine).machine.json",
            "machine_id": identity.machineID,
            "machine_label": identity.machineLabel ?? NSNull(),
        ]
        machineFacts[machine] = machineFactsRecord
        evidence["machine_facts"] = machineFacts
        self.evidence = evidence
    }
}
