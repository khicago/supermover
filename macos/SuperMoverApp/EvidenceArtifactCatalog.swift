import Foundation

enum EvidenceArtifactFamily: String, CaseIterable, Hashable, Identifiable {
    case profile
    case pairing
    case session
    case networkTransfer
    case warning
    case deleted
    case drift
    case pruneApproval
    case pruneReceipt
    case reconcileReceipt
    case daemon
    case daemonEvent
    case incrementalSyncQueue
    case incrementalSyncRun
    case agentInfluence
    case historyIndex
    case recoveryState
    case unknownControl

    var id: String { rawValue }

    var title: String {
        switch self {
        case .profile:
            return "Config"
        case .pairing:
            return "Pairing"
        case .session:
            return "Session"
        case .networkTransfer:
            return "Network transfer"
        case .warning:
            return "Warning"
        case .deleted:
            return "Soft delete"
        case .drift:
            return "Drift"
        case .pruneApproval:
            return "Prune approval"
        case .pruneReceipt:
            return "Prune receipt"
        case .reconcileReceipt:
            return "Reconcile receipt"
        case .daemon:
            return "Daemon"
        case .daemonEvent:
            return "Daemon event"
        case .incrementalSyncQueue:
            return "Incremental sync queue"
        case .incrementalSyncRun:
            return "Incremental sync run"
        case .agentInfluence:
            return "Agent influence"
        case .historyIndex:
            return "History index"
        case .recoveryState:
            return "Recovery state"
        case .unknownControl:
            return "Unknown control artifact"
        }
    }
}

enum EvidenceArtifactJSONStatus: String, Equatable, Hashable {
    case valid
    case malformed
    case notJSON
    case notCheckedLargeArtifact
    case unreadable
    case symlink
}

enum EvidenceArtifactIssueSeverity: Int, CaseIterable, Comparable, Hashable {
    case ok = 0
    case warning = 1
    case critical = 2

    static func < (lhs: EvidenceArtifactIssueSeverity, rhs: EvidenceArtifactIssueSeverity) -> Bool {
        lhs.rawValue < rhs.rawValue
    }
}

struct EvidenceArtifactCatalogProblem: Identifiable, Equatable, Hashable {
    enum Kind: String, Equatable, Hashable {
        case unsafePath
        case symlink
        case malformedJSON
        case readError
        case unsupportedFileType
    }

    let kind: Kind
    let relativePath: String
    let message: String
    let severity: EvidenceArtifactIssueSeverity

    var id: String { "\(kind.rawValue):\(relativePath)" }
}

struct EvidenceArtifactRecord: Identifiable, Equatable, Hashable {
    let id: String
    let family: EvidenceArtifactFamily
    let relativePath: String
    let fileName: String
    let artifactID: String?
    let size: Int64?
    let modifiedAt: Date?
    let jsonStatus: EvidenceArtifactJSONStatus
    let previewText: String
    let searchText: String
    let issueSeverity: EvidenceArtifactIssueSeverity
}

struct EvidenceArtifactFilter: Equatable {
    let families: Set<EvidenceArtifactFamily>
    let query: String

    init(families: Set<EvidenceArtifactFamily> = [], query: String = "") {
        self.families = families
        self.query = query
    }

    func matches(_ artifact: EvidenceArtifactRecord) -> Bool {
        if !families.isEmpty, !families.contains(artifact.family) {
            return false
        }

        let terms = query.evidenceCatalogSearchTerms
        guard !terms.isEmpty else {
            return true
        }

        let haystack = artifact.searchText.evidenceCatalogSearchNormalized
        return terms.allSatisfy { haystack.contains($0) }
    }
}

struct EvidenceArtifactCatalog: Equatable {
    let controlPlaneURL: URL
    let artifacts: [EvidenceArtifactRecord]
    let problems: [EvidenceArtifactCatalogProblem]

    var hasProblems: Bool {
        !problems.isEmpty
    }

    var familiesWithArtifacts: [EvidenceArtifactFamily] {
        Array(Set(artifacts.map(\.family))).sorted { lhs, rhs in
            lhs.rawValue < rhs.rawValue
        }
    }

    func filtered(
        families: Set<EvidenceArtifactFamily> = [],
        query: String = ""
    ) -> [EvidenceArtifactRecord] {
        filtered(EvidenceArtifactFilter(families: families, query: query))
    }

    func filtered(_ filter: EvidenceArtifactFilter) -> [EvidenceArtifactRecord] {
        artifacts.filter { filter.matches($0) }
    }

    func problems(for artifact: EvidenceArtifactRecord) -> [EvidenceArtifactCatalogProblem] {
        problems.filter { $0.relativePath == artifact.relativePath }
    }
}

struct EvidenceArtifactCatalogReader {
    static let defaultPreviewByteLimit = 8 * 1024
    static let maximumPreviewByteLimit = 64 * 1024

    let fileManager: FileManager
    let previewByteLimit: Int

    init(
        fileManager: FileManager = .default,
        previewByteLimit: Int = EvidenceArtifactCatalogReader.defaultPreviewByteLimit
    ) {
        self.fileManager = fileManager
        self.previewByteLimit = min(
            max(0, previewByteLimit),
            EvidenceArtifactCatalogReader.maximumPreviewByteLimit
        )
    }

    func read(targetRootURL: URL) -> EvidenceArtifactCatalog {
        read(
            controlPlaneURL: targetRootURL.appendingPathComponent(
                ".supermover",
                isDirectory: true
            )
        )
    }

    func read(controlPlaneURL: URL) -> EvidenceArtifactCatalog {
        EvidenceArtifactCatalogScanner(
            controlPlaneURL: controlPlaneURL,
            fileManager: fileManager,
            previewByteLimit: previewByteLimit
        ).read()
    }
}

private final class EvidenceArtifactCatalogScanner {
    private let controlPlaneURL: URL
    private let fileManager: FileManager
    private let previewByteLimit: Int
    private var artifacts: [EvidenceArtifactRecord] = []
    private var problems: [EvidenceArtifactCatalogProblem] = []

    private let resourceKeys: Set<URLResourceKey> = [
        .contentModificationDateKey,
        .fileSizeKey,
        .isDirectoryKey,
        .isRegularFileKey,
        .isSymbolicLinkKey,
    ]

    init(controlPlaneURL: URL, fileManager: FileManager, previewByteLimit: Int) {
        self.controlPlaneURL = controlPlaneURL
        self.fileManager = fileManager
        self.previewByteLimit = previewByteLimit
    }

    func read() -> EvidenceArtifactCatalog {
        scanRoot()

        return EvidenceArtifactCatalog(
            controlPlaneURL: controlPlaneURL,
            artifacts: artifacts.sorted { lhs, rhs in
                lhs.relativePath < rhs.relativePath
            },
            problems: problems.sorted { lhs, rhs in
                if lhs.relativePath == rhs.relativePath {
                    return lhs.kind.rawValue < rhs.kind.rawValue
                }
                return lhs.relativePath < rhs.relativePath
            }
        )
    }

    private func scanRoot() {
        do {
            let values = try controlPlaneURL.resourceValues(forKeys: resourceKeys)
            if values.isSymbolicLink == true {
                recordSymlink(at: controlPlaneURL, components: [], values: values)
                return
            }
            guard values.isDirectory == true else {
                let problem = makeProblem(
                    kind: .unsupportedFileType,
                    components: [],
                    message: ".supermover is not a directory."
                )
                problems.append(problem)
                artifacts.append(
                    makeRecord(
                        components: [],
                        size: values.fileSize.map(Int64.init),
                        modifiedAt: values.contentModificationDate,
                        jsonStatus: .unreadable,
                        previewText: "",
                        problemsForRecord: [problem]
                    )
                )
                return
            }
        } catch {
            problems.append(
                makeProblem(
                    kind: .readError,
                    components: [],
                    message: "Cannot read .supermover: \(error.localizedDescription)"
                )
            )
            return
        }

        scanDirectory(at: controlPlaneURL, components: [])
    }

    private func scanDirectory(at directoryURL: URL, components: [String]) {
        let children: [URL]
        do {
            children = try fileManager.contentsOfDirectory(
                at: directoryURL,
                includingPropertiesForKeys: Array(resourceKeys),
                options: []
            )
        } catch {
            problems.append(
                makeProblem(
                    kind: .readError,
                    components: components,
                    message: "Cannot list directory: \(error.localizedDescription)"
                )
            )
            return
        }

        for childURL in children.sorted(by: { $0.lastPathComponent < $1.lastPathComponent }) {
            let childComponents = components + [childURL.lastPathComponent]
            guard validateSafePath(childComponents) else {
                recordUnsafePath(components: childComponents)
                continue
            }

            let values: URLResourceValues
            do {
                values = try childURL.resourceValues(forKeys: resourceKeys)
            } catch {
                recordReadError(
                    at: childURL,
                    components: childComponents,
                    message: "Cannot read file metadata: \(error.localizedDescription)"
                )
                continue
            }

            if values.isSymbolicLink == true {
                recordSymlink(at: childURL, components: childComponents, values: values)
            } else if values.isDirectory == true {
                scanDirectory(at: childURL, components: childComponents)
            } else if values.isRegularFile == true {
                recordFile(at: childURL, components: childComponents, values: values)
            } else {
                let problem = makeProblem(
                    kind: .unsupportedFileType,
                    components: childComponents,
                    message: "Unsupported control-plane file type."
                )
                problems.append(problem)
                artifacts.append(
                    makeRecord(
                        components: childComponents,
                        size: values.fileSize.map(Int64.init),
                        modifiedAt: values.contentModificationDate,
                        jsonStatus: .unreadable,
                        previewText: "",
                        problemsForRecord: [problem]
                    )
                )
            }
        }
    }

    private func recordFile(
        at fileURL: URL,
        components: [String],
        values: URLResourceValues
    ) {
        var recordProblems: [EvidenceArtifactCatalogProblem] = []
        let preview: PreviewReadResult
        do {
            preview = try readPreviewData(from: fileURL)
        } catch {
            let problem = makeProblem(
                kind: .readError,
                components: components,
                message: "Cannot read artifact preview: \(error.localizedDescription)"
            )
            problems.append(problem)
            recordProblems.append(problem)
            artifacts.append(
                makeRecord(
                    components: components,
                    size: values.fileSize.map(Int64.init),
                    modifiedAt: values.contentModificationDate,
                    jsonStatus: .unreadable,
                    previewText: "",
                    problemsForRecord: recordProblems
                )
            )
            return
        }

        let fileName = components.last ?? ".supermover"
        let jsonStatus: EvidenceArtifactJSONStatus
        if isJSONFileName(fileName) {
            if preview.truncated {
                jsonStatus = .notCheckedLargeArtifact
            } else {
                do {
                    _ = try JSONSerialization.jsonObject(
                        with: preview.data,
                        options: [.fragmentsAllowed]
                    )
                    jsonStatus = .valid
                } catch {
                    let problem = makeProblem(
                        kind: .malformedJSON,
                        components: components,
                        message: "Malformed JSON: \(error.localizedDescription)"
                    )
                    problems.append(problem)
                    recordProblems.append(problem)
                    jsonStatus = .malformed
                }
            }
        } else {
            jsonStatus = .notJSON
        }

        artifacts.append(
            makeRecord(
                components: components,
                size: values.fileSize.map(Int64.init),
                modifiedAt: values.contentModificationDate,
                jsonStatus: jsonStatus,
                previewText: previewText(from: preview.data, truncated: preview.truncated),
                problemsForRecord: recordProblems
            )
        )
    }

    private func recordReadError(at _: URL, components: [String], message: String) {
        let problem = makeProblem(kind: .readError, components: components, message: message)
        problems.append(problem)
        artifacts.append(
            makeRecord(
                components: components,
                size: nil,
                modifiedAt: nil,
                jsonStatus: .unreadable,
                previewText: "",
                problemsForRecord: [problem]
            )
        )
    }

    private func recordUnsafePath(components: [String]) {
        let problem = makeProblem(
            kind: .unsafePath,
            components: components,
            message: "Unsafe control-plane path component."
        )
        problems.append(problem)
        artifacts.append(
            makeRecord(
                components: components,
                size: nil,
                modifiedAt: nil,
                jsonStatus: .unreadable,
                previewText: "",
                problemsForRecord: [problem]
            )
        )
    }

    private func recordSymlink(
        at _: URL,
        components: [String],
        values: URLResourceValues
    ) {
        let problem = makeProblem(
            kind: .symlink,
            components: components,
            message: "Refused to follow symlink in .supermover."
        )
        problems.append(problem)
        artifacts.append(
            makeRecord(
                components: components,
                size: values.fileSize.map(Int64.init),
                modifiedAt: values.contentModificationDate,
                jsonStatus: .symlink,
                previewText: "",
                problemsForRecord: [problem]
            )
        )
    }

    private func makeRecord(
        components: [String],
        size: Int64?,
        modifiedAt: Date?,
        jsonStatus: EvidenceArtifactJSONStatus,
        previewText: String,
        problemsForRecord: [EvidenceArtifactCatalogProblem]
    ) -> EvidenceArtifactRecord {
        let classification = classify(components)
        let relativePath = makeRelativePath(components)
        let fileName = components.last ?? ".supermover"
        let issueSeverity = problemsForRecord.map(\.severity).max() ?? .ok
        let searchText = [
            classification.family.rawValue,
            classification.family.title,
            relativePath,
            fileName,
            classification.artifactID,
            jsonStatus.rawValue,
            previewText,
            problemsForRecord.map(\.message).joined(separator: " "),
        ].compactMap { value -> String? in
            guard let value, !value.isEmpty else {
                return nil
            }
            return value
        }.joined(separator: " ")

        return EvidenceArtifactRecord(
            id: relativePath,
            family: classification.family,
            relativePath: relativePath,
            fileName: fileName,
            artifactID: classification.artifactID,
            size: size,
            modifiedAt: modifiedAt,
            jsonStatus: jsonStatus,
            previewText: previewText,
            searchText: searchText,
            issueSeverity: issueSeverity
        )
    }

    private func classify(_ components: [String]) -> EvidenceArtifactClassification {
        guard let first = components.first,
              let fileName = components.last else {
            return EvidenceArtifactClassification(family: .unknownControl, artifactID: nil)
        }

        let json = isJSONFileName(fileName)

        if components.count == 2, first == "profiles", json {
            return EvidenceArtifactClassification(family: .profile, artifactID: artifactStem(fileName))
        }
        if components.count == 2, first == "pairings", json {
            return EvidenceArtifactClassification(family: .pairing, artifactID: artifactStem(fileName))
        }
        if components.count == 3,
           first == "sessions",
           fileName == "network-transfer.json" {
            return EvidenceArtifactClassification(family: .networkTransfer, artifactID: components[1])
        }
        if components.count == 3,
           first == "sessions",
           ["session.json", "receipt.json", "manifest.json"].contains(fileName) {
            return EvidenceArtifactClassification(family: .session, artifactID: components[1])
        }
        if components.count == 2, first == "warnings", json {
            return EvidenceArtifactClassification(family: .warning, artifactID: artifactStem(fileName))
        }
        if components.count == 2, first == "deleted", json {
            return EvidenceArtifactClassification(family: .deleted, artifactID: artifactStem(fileName))
        }
        if components.count == 2, first == "drift", json {
            return EvidenceArtifactClassification(family: .drift, artifactID: artifactStem(fileName))
        }
        if components.count == 3, first == "prune", components[1] == "approvals", json {
            return EvidenceArtifactClassification(family: .pruneApproval, artifactID: artifactStem(fileName))
        }
        if components.count == 3, first == "prune", components[1] == "receipts", json {
            return EvidenceArtifactClassification(family: .pruneReceipt, artifactID: artifactStem(fileName))
        }
        if components.count == 3, first == "reconcile", components[1] == "receipts", json {
            return EvidenceArtifactClassification(family: .reconcileReceipt, artifactID: artifactStem(fileName))
        }
        if components.count == 2,
           first == "daemon",
           ["install.json", "state.json", "stop-intent.json", "restart-intent.json"].contains(fileName) {
            return EvidenceArtifactClassification(family: .daemon, artifactID: artifactStem(fileName))
        }
        if components.count == 3, first == "daemon", components[1] == "events", json {
            return EvidenceArtifactClassification(family: .daemonEvent, artifactID: artifactStem(fileName))
        }
        if components.count == 6,
           first == "incremental-sync",
           components[1] == "profiles",
           components[3] == "targets",
           fileName == "queue.json" {
            return EvidenceArtifactClassification(
                family: .incrementalSyncQueue,
                artifactID: "\(components[2])/\(components[4])"
            )
        }
        if components.count == 7,
           first == "incremental-sync",
           components[1] == "profiles",
           components[3] == "targets",
           components[5] == "runs",
           json {
            return EvidenceArtifactClassification(
                family: .incrementalSyncRun,
                artifactID: "\(components[2])/\(components[4])/\(artifactStem(fileName))"
            )
        }
        if components.count == 2, first == "agent", json {
            return EvidenceArtifactClassification(family: .agentInfluence, artifactID: artifactStem(fileName))
        }
        if components == ["history", "index.json"] {
            return EvidenceArtifactClassification(family: .historyIndex, artifactID: "index")
        }
        if components == ["recovery", "state.json"] {
            return EvidenceArtifactClassification(family: .recoveryState, artifactID: "state")
        }

        return EvidenceArtifactClassification(family: .unknownControl, artifactID: artifactStem(fileName))
    }

    private func makeProblem(
        kind: EvidenceArtifactCatalogProblem.Kind,
        components: [String],
        message: String,
        severity: EvidenceArtifactIssueSeverity = .critical
    ) -> EvidenceArtifactCatalogProblem {
        EvidenceArtifactCatalogProblem(
            kind: kind,
            relativePath: makeRelativePath(components),
            message: message,
            severity: severity
        )
    }

    private func readPreviewData(from fileURL: URL) throws -> PreviewReadResult {
        let readCount = previewByteLimit + 1
        let handle = try FileHandle(forReadingFrom: fileURL)
        defer {
            try? handle.close()
        }

        let data = try handle.read(upToCount: readCount) ?? Data()
        if data.count > previewByteLimit {
            return PreviewReadResult(
                data: Data(data.prefix(previewByteLimit)),
                truncated: true
            )
        }

        return PreviewReadResult(data: data, truncated: false)
    }

    private func previewText(from data: Data, truncated: Bool) -> String {
        guard !data.isEmpty else {
            return truncated ? "..." : ""
        }
        guard let rawText = String(data: data, encoding: .utf8) else {
            return truncated ? "<binary preview>..." : "<binary preview>"
        }

        let collapsed = rawText.replacingOccurrences(
            of: "\\s+",
            with: " ",
            options: .regularExpression
        ).trimmingCharacters(in: .whitespacesAndNewlines)
        let prefix = String(collapsed.prefix(512))
        return truncated ? "\(prefix)..." : prefix
    }

    private func makeRelativePath(_ components: [String]) -> String {
        guard !components.isEmpty else {
            return ".supermover"
        }
        return ".supermover/" + components.joined(separator: "/")
    }

    private func validateSafePath(_ components: [String]) -> Bool {
        components.allSatisfy { component in
            !component.isEmpty &&
                component != "." &&
                component != ".." &&
                !component.contains("/") &&
                !component.contains("\0")
        }
    }

    private func isJSONFileName(_ fileName: String) -> Bool {
        (fileName as NSString).pathExtension.lowercased() == "json"
    }

    private func artifactStem(_ fileName: String) -> String {
        (fileName as NSString).deletingPathExtension
    }
}

private struct EvidenceArtifactClassification {
    let family: EvidenceArtifactFamily
    let artifactID: String?
}

private struct PreviewReadResult {
    let data: Data
    let truncated: Bool
}

private extension String {
    var evidenceCatalogSearchNormalized: String {
        folding(
            options: [.caseInsensitive, .diacriticInsensitive],
            locale: Locale(identifier: "en_US_POSIX")
        ).trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var evidenceCatalogSearchTerms: [String] {
        evidenceCatalogSearchNormalized
            .split(whereSeparator: { $0.isWhitespace })
            .map(String.init)
    }
}
