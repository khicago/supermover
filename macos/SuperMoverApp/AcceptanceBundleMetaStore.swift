import Foundation

struct AcceptanceBundleOperatorEvidenceRecord: Equatable {
    let kind: String
    let status: String
    let detail: String
    let artifact: String?
    let machineID: String?
    let machineLabel: String?

    init(
        kind: String,
        status: String,
        detail: String,
        artifact: String?,
        machineID: String? = nil,
        machineLabel: String? = nil
    ) {
        self.kind = kind
        self.status = status
        self.detail = detail
        self.artifact = artifact
        self.machineID = machineID
        self.machineLabel = machineLabel
    }
}

struct AcceptanceBundleMetaStore {
    enum MutationError: LocalizedError, Equatable {
        case invalidBundleRoot(URL)
        case missingMeta(URL)
        case malformedMeta(URL)
        case lockTimeout(URL)
        case invalidLock(URL)
        case symlinkRejected(URL)
        case unsafeOutputArtifact(URL)
        case invalidRecord(String)

        var errorDescription: String? {
            switch self {
            case let .invalidBundleRoot(url):
                return "Acceptance bundle root is not a directory: \(url.path)"
            case let .missingMeta(url):
                return "Acceptance bundle is missing meta.json at \(url.path)."
            case let .malformedMeta(url):
                return "Acceptance bundle meta.json is malformed at \(url.path)."
            case let .lockTimeout(url):
                return "Timed out waiting for acceptance bundle lock at \(url.path)."
            case let .invalidLock(url):
                return "Acceptance bundle lock path is not a directory: \(url.path)"
            case let .symlinkRejected(url):
                return "Acceptance bundle mutation rejected symlink path: \(url.path)"
            case let .unsafeOutputArtifact(url):
                return "Acceptance bundle mutation rejected unsafe output artifact: \(url.path)"
            case let .invalidRecord(message):
                return message
            }
        }
    }

    let fileManager: FileManager
    let pollInterval: TimeInterval

    init(fileManager: FileManager = .default, pollInterval: TimeInterval = 0.1) {
        self.fileManager = fileManager
        self.pollInterval = max(0.01, pollInterval)
    }

    func recordOperatorEvidence(
        bundleRootURL: URL,
        record: AcceptanceBundleOperatorEvidenceRecord,
        timeoutSeconds: TimeInterval = 30
    ) throws {
        let normalizedRecord = try normalized(record: record)
        try ensureDirectory(bundleRootURL)
        try rejectSymlinkIfPresent(bundleRootURL)

        let metaURL = bundleRootURL.appendingPathComponent("meta.json")
        guard fileManager.fileExists(atPath: metaURL.path) else {
            throw MutationError.missingMeta(metaURL)
        }
        try ensureRegularMetaFile(metaURL)
        try withMutableMetaDocument(
            bundleRootURL: bundleRootURL,
            timeoutSeconds: timeoutSeconds
        ) { document in
            applyOperatorEvidence(record: normalizedRecord, to: &document)
        }
    }

    private func normalized(
        record: AcceptanceBundleOperatorEvidenceRecord
    ) throws -> AcceptanceBundleOperatorEvidenceRecord {
        let kind = record.kind.trimmingCharacters(in: .whitespacesAndNewlines)
        let status = record.status.trimmingCharacters(in: .whitespacesAndNewlines)
        let detail = record.detail.trimmingCharacters(in: .whitespacesAndNewlines)
        let artifact = record.artifact?.trimmingCharacters(in: .whitespacesAndNewlines)
        let machineID = record.machineID?.trimmingCharacters(in: .whitespacesAndNewlines)
        let machineLabel = record.machineLabel?.trimmingCharacters(in: .whitespacesAndNewlines)

        guard !kind.isEmpty else {
            throw MutationError.invalidRecord("Operator evidence kind is required.")
        }
        guard status == "pass" || status == "blocked" else {
            throw MutationError.invalidRecord("Operator evidence status must be pass or blocked.")
        }
        guard !detail.isEmpty else {
            throw MutationError.invalidRecord("Operator evidence detail is required.")
        }

        return AcceptanceBundleOperatorEvidenceRecord(
            kind: kind,
            status: status,
            detail: detail,
            artifact: artifact?.isEmpty == true ? nil : artifact,
            machineID: machineID?.isEmpty == true ? nil : machineID,
            machineLabel: machineLabel?.isEmpty == true ? nil : machineLabel
        )
    }

    private func ensureDirectory(_ url: URL) throws {
        var isDirectory: ObjCBool = false
        guard fileManager.fileExists(atPath: url.path, isDirectory: &isDirectory), isDirectory.boolValue else {
            throw MutationError.invalidBundleRoot(url)
        }
    }

    private func rejectSymlinkIfPresent(_ url: URL) throws {
        guard fileManager.fileExists(atPath: url.path) else {
            return
        }
        let values = try url.resourceValues(forKeys: [.isSymbolicLinkKey])
        if values.isSymbolicLink == true {
            throw MutationError.symlinkRejected(url)
        }
    }

    private func ensureRegularMetaFile(_ url: URL) throws {
        try rejectSymlinkIfPresent(url)
        let values = try url.resourceValues(forKeys: [.isRegularFileKey, .isSymbolicLinkKey, .linkCountKey])
        if values.isSymbolicLink == true {
            throw MutationError.symlinkRejected(url)
        }
        guard values.isRegularFile == true, (values.linkCount ?? 1) == 1 else {
            throw MutationError.malformedMeta(url)
        }
    }

    func withBundleLock(
        lockURL: URL,
        timeoutSeconds: TimeInterval,
        body: () throws -> Void
    ) throws {
        let deadline = Date().addingTimeInterval(timeoutSeconds)
        while true {
            do {
                try fileManager.createDirectory(
                    at: lockURL,
                    withIntermediateDirectories: false
                )
                break
            } catch {
                if fileManager.fileExists(atPath: lockURL.path) {
                    let values = try? lockURL.resourceValues(forKeys: [.isDirectoryKey, .isSymbolicLinkKey])
                    if values?.isSymbolicLink == true {
                        throw MutationError.symlinkRejected(lockURL)
                    }
                    if values?.isDirectory == false {
                        throw MutationError.invalidLock(lockURL)
                    }
                }
                if Date() >= deadline {
                    throw MutationError.lockTimeout(lockURL)
                }
                Thread.sleep(forTimeInterval: pollInterval)
            }
        }

        defer {
            try? fileManager.removeItem(at: lockURL)
        }
        try body()
    }

    func loadMetaDocument(metaURL: URL) throws -> [String: Any] {
        let data = try Data(contentsOf: metaURL)
        let object: Any
        do {
            object = try JSONSerialization.jsonObject(with: data)
        } catch {
            throw MutationError.malformedMeta(metaURL)
        }
        guard let document = object as? [String: Any] else {
            throw MutationError.malformedMeta(metaURL)
        }
        return document
    }

    func withMutableMetaDocument(
        bundleRootURL: URL,
        timeoutSeconds: TimeInterval = 30,
        mutate: (inout [String: Any]) throws -> Void
    ) throws {
        try ensureDirectory(bundleRootURL)
        try rejectSymlinkIfPresent(bundleRootURL)
        let metaURL = bundleRootURL.appendingPathComponent("meta.json")
        guard fileManager.fileExists(atPath: metaURL.path) else {
            throw MutationError.missingMeta(metaURL)
        }
        try ensureRegularMetaFile(metaURL)
        let lockURL = bundleRootURL.appendingPathComponent(".meta.lock", isDirectory: true)
        try withBundleLock(lockURL: lockURL, timeoutSeconds: timeoutSeconds) {
            var document = try loadMetaDocument(metaURL: metaURL)
            try mutate(&document)
            let data = try JSONSerialization.data(
                withJSONObject: document,
                options: [.prettyPrinted, .sortedKeys]
            )
            try data.write(to: metaURL, options: [.atomic])
        }
    }

    private func applyOperatorEvidence(
        record: AcceptanceBundleOperatorEvidenceRecord,
        to document: inout [String: Any]
    ) {
        var evidence = document["evidence"] as? [String: Any] ?? [:]
        var operatorEvidence = evidence["operator"] as? [String: Any] ?? [:]
        var entry: [String: Any] = [
            "status": record.status,
            "detail": record.detail,
        ]
        if let artifact = record.artifact {
            entry["artifact"] = artifact
        }
        if let machineID = record.machineID {
            entry["machine_id"] = machineID
        }
        if let machineLabel = record.machineLabel {
            entry["machine_label"] = machineLabel
        }
        operatorEvidence[record.kind] = entry
        evidence["operator"] = operatorEvidence
        document["evidence"] = evidence
    }
}
