import Foundation

struct AcceptancePackagingEvidenceCollector {
    struct AppAuditResult: Equatable {
        let exitCode: Int32
        let status: String
        let readiness: String
        let passReady: Bool
        let blockingChecks: Int
    }

    enum CollectionError: LocalizedError, Equatable {
        case missingBundlePath
        case invalidBundleRoot(String)
        case missingMeta(String)
        case symlinkRejected(String)
        case missingAppResources(String)
        case missingBundledProvenance(String)
        case malformedBundledProvenance(String)
        case auditScriptUnavailable(String)
        case auditExecutionFailed(String)
        case malformedAuditOutput(String)
        case malformedNotarizationOutput(String)
        case staleNotarizationOutput(String)
        case unsafeOutputArtifact(String)

        var errorDescription: String? {
            switch self {
            case .missingBundlePath:
                return "Choose an acceptance bundle directory first."
            case let .invalidBundleRoot(path):
                return "Acceptance bundle root is not a directory: \(path)"
            case let .missingMeta(path):
                return "Acceptance bundle is missing meta.json at \(path)."
            case let .symlinkRejected(path):
                return "Acceptance packaging evidence rejected an unsafe symlink path: \(path)"
            case let .missingAppResources(path):
                return "Current packaged app resources are unavailable: \(path)"
            case let .missingBundledProvenance(path):
                return "Bundled provenance manifest is missing: \(path)"
            case let .malformedBundledProvenance(path):
                return "Bundled provenance manifest is malformed or incomplete: \(path)"
            case let .auditScriptUnavailable(path):
                return "Local app audit helper is unavailable: \(path)"
            case let .auditExecutionFailed(detail):
                return "Local app audit failed unexpectedly: \(detail)"
            case let .malformedAuditOutput(path):
                return "Local app audit output is malformed: \(path)"
            case let .malformedNotarizationOutput(path):
                return "Local notarization evidence is malformed: \(path)"
            case let .staleNotarizationOutput(path):
                return "Local notarization evidence does not match the current packaged app: \(path)"
            case let .unsafeOutputArtifact(path):
                return "Acceptance packaging evidence rejected an unsafe output artifact: \(path)"
            }
        }
    }

    private typealias BundledProvenanceManifest = SuperMoverBundledProvenanceManifest

    private struct AppAuditArtifact: Decodable {
        struct Provenance: Decodable {
            let manifest: BundledProvenanceManifest?
        }

        let schema: String?
        let status: String?
        let readiness: String?
        let app_path: String?
        let provenance: Provenance?
        let summary: Summary?

        struct Summary: Decodable {
            let pass_ready: Bool?
            let blocking_checks: Int?
        }
    }

    struct CurrentAppAuditState: Equatable {
        let status: String
        let readiness: String
        let passReady: Bool
        let installReady: Bool
        let failureMessage: String?
    }

    struct CurrentMachineEvidenceInspection: Equatable {
        let audit: CurrentAppAuditState
        let notarization: CurrentAppNotarizationState
    }

    struct CurrentAppNotarizationArtifact: Decodable {
        struct Submission: Decodable {
            let id: String?
            let status: String?
        }

        struct NotaryLog: Decodable {
            let path: String?
        }

        struct Audit: Decodable {
            let path: String?
            let status: String?
            let readiness: String?
            let pass_ready: Bool?
        }

        struct Failure: Decodable {
            let id: String?
            let summary: String?
            let detail: String?
            let message: String?
        }

        let schema: String?
        let status: String?
        let app_path: String?
        let auth_mode: String?
        let submission: Submission?
        let notary_log: NotaryLog?
        let audit: Audit?
        let failure: Failure?
    }

    enum CurrentAppNotarizationState: Equatable {
        case missing
        case notReady(status: String)
        case ready
    }

    private struct PreparedCurrentMachineEvidence {
        struct PreparedNotarization {
            let outputName: String
            let notaryLogOutputName: String
            let status: String
            let submissionID: String?
            let submissionStatus: String?
            let authMode: String?
            let failurePresent: Bool
            let notaryLogPath: String?
            let auditStatus: String?
            let auditReadiness: String?
            let auditPassReady: Bool?
        }

        let stagingDirectoryURL: URL
        let versionOutputName: String
        let copiedProvenanceName: String
        let auditOutputName: String
        let audit: AppAuditResult
        let auditState: CurrentAppAuditState
        let notarization: PreparedNotarization?

        var outputNames: [String] {
            [versionOutputName, copiedProvenanceName, auditOutputName]
                + (notarization.map { [$0.outputName, $0.notaryLogOutputName] } ?? [])
        }

        var inspection: CurrentMachineEvidenceInspection {
            CurrentMachineEvidenceInspection(
                audit: auditState,
                notarization: currentAppNotarizationState(for: notarization)
            )
        }

        private func currentAppNotarizationState(
            for notarization: PreparedNotarization?
        ) -> CurrentAppNotarizationState {
            guard let notarization else {
                return .missing
            }
            if AcceptanceInstalledAppReleaseEvidenceSummary.notarizationIsReleaseReady(
                status: notarization.status,
                submissionID: notarization.submissionID,
                submissionStatus: notarization.submissionStatus,
                authMode: notarization.authMode,
                failurePresent: notarization.failurePresent,
                notaryLogPath: notarization.notaryLogPath,
                auditStatus: notarization.auditStatus,
                auditReadiness: notarization.auditReadiness,
                auditPassReady: notarization.auditPassReady
            ) {
                return .ready
            }
            return .notReady(status: notarization.status)
        }
    }

    let fileManager: FileManager
    let artifactAccess: AcceptanceBundleArtifactAccess
    let metaStore: AcceptanceBundleMetaStore
    let versionRunner: (_ resourceURL: URL?) throws -> String
    let auditRunner: (_ appBundleURL: URL, _ outputURL: URL) throws -> AppAuditResult

    init(
        fileManager: FileManager = .default,
        metaStore: AcceptanceBundleMetaStore = AcceptanceBundleMetaStore(),
        versionRunner: @escaping (_ resourceURL: URL?) throws -> String = Self.defaultVersionRunner,
        auditRunner: @escaping (_ appBundleURL: URL, _ outputURL: URL) throws -> AppAuditResult = Self.defaultAuditRunner
    ) {
        self.fileManager = fileManager
        artifactAccess = AcceptanceBundleArtifactAccess(fileManager: fileManager)
        self.metaStore = metaStore
        self.versionRunner = versionRunner
        self.auditRunner = auditRunner
    }

    func recordCurrentMachineEvidence(
        bundleRootURL: URL,
        machine: String,
        collectedBy: String,
        resourceURL: URL?
    ) throws -> [String] {
        try ensureBundleRoot(bundleRootURL)
        let prepared: PreparedCurrentMachineEvidence
        do {
            prepared = try prepareCurrentMachineEvidence(
                bundleRootURL: bundleRootURL,
                machine: machine,
                resourceURL: resourceURL
            )
        } catch {
            if shouldClearRecordedNotarizationEvidence(after: error) {
                try clearRecordedNotarizationEvidence(
                    bundleRootURL: bundleRootURL,
                    machine: machine
                )
            }
            throw error
        }
        defer {
            try? fileManager.removeItem(at: prepared.stagingDirectoryURL)
        }

        try publishPreparedCurrentMachineEvidence(
            prepared,
            bundleRootURL: bundleRootURL
        )

        try metaStore.withMutableMetaDocument(bundleRootURL: bundleRootURL) { document in
            var evidence = document["evidence"] as? [String: Any] ?? [:]
            var appAudit = evidence["app_audit"] as? [String: Any] ?? [:]
            appAudit[machine] = [
                "collected_by": collectedBy,
                "output": prepared.auditOutputName,
                "exit_code": prepared.audit.exitCode,
                "status": prepared.audit.status,
                "readiness": prepared.audit.readiness,
                "pass_ready": prepared.audit.passReady,
                "blocking_checks": prepared.audit.blockingChecks,
            ]
            evidence["app_audit"] = appAudit
            if let notarization = prepared.notarization {
                var notarizationEvidence = evidence["notarization"] as? [String: Any] ?? [:]
                var record: [String: Any] = [
                    "collected_by": collectedBy,
                    "output": notarization.outputName,
                    "notary_log": notarization.notaryLogOutputName,
                    "status": notarization.status,
                ]
                if let auditStatus = notarization.auditStatus {
                    record["audit_status"] = auditStatus
                }
                if let auditReadiness = notarization.auditReadiness {
                    record["audit_readiness"] = auditReadiness
                }
                if let auditPassReady = notarization.auditPassReady {
                    record["audit_pass_ready"] = auditPassReady
                }
                notarizationEvidence[machine] = record
                evidence["notarization"] = notarizationEvidence
            } else {
                removeStaleNotarizationEvidence(
                    from: &evidence,
                    machine: machine,
                    bundleRootURL: bundleRootURL
                )
            }
            document["evidence"] = evidence
        }

        return prepared.outputNames
    }

    func preflightCurrentMachineEvidenceWrite(
        bundleRootURL: URL,
        machine: String,
        resourceURL: URL?
    ) throws {
        try ensureBundleRoot(bundleRootURL)
        for outputName in mandatoryOutputNames(machine: machine) {
            _ = try safeBundleArtifactOutputURL(
                bundleRootURL: bundleRootURL,
                fileName: outputName
            )
        }
        if try notarizationEvidenceSourceURL(resourceURL: resourceURL) != nil {
            _ = try safeBundleArtifactOutputURL(
                bundleRootURL: bundleRootURL,
                fileName: "\(machine).notarization.json"
            )
            _ = try safeBundleArtifactOutputURL(
                bundleRootURL: bundleRootURL,
                fileName: "\(machine).notary-log.json"
            )
        }
    }

    func inspectCurrentMachineEvidence(
        bundleRootURL: URL,
        machine: String,
        resourceURL: URL?
    ) throws -> CurrentMachineEvidenceInspection {
        let prepared = try prepareCurrentMachineEvidence(
            bundleRootURL: bundleRootURL,
            machine: machine,
            resourceURL: resourceURL
        )
        defer {
            try? fileManager.removeItem(at: prepared.stagingDirectoryURL)
        }
        return prepared.inspection
    }

    private func ensureBundleRoot(_ bundleRootURL: URL) throws {
        do {
            _ = try artifactAccess.metaURL(bundleRootURL: bundleRootURL)
        } catch let AcceptanceBundleArtifactAccess.AccessError.invalidBundleRoot(path) {
            throw CollectionError.invalidBundleRoot(path)
        } catch AcceptanceBundleArtifactAccess.AccessError.missingArtifact {
            throw CollectionError.missingMeta(
                bundleRootURL.appendingPathComponent("meta.json").path
            )
        } catch let AcceptanceBundleArtifactAccess.AccessError.symlinkRejected(path) {
            throw CollectionError.symlinkRejected(path)
        } catch {
            throw CollectionError.invalidBundleRoot(bundleRootURL.path)
        }
    }

    private func mandatoryOutputNames(machine: String) -> [String] {
        [
            "\(machine).provenance.json",
            "\(machine).version.txt",
            "\(machine).app-audit.json",
        ]
    }

    private func prepareCurrentMachineEvidence(
        bundleRootURL: URL,
        machine: String,
        resourceURL: URL?
    ) throws -> PreparedCurrentMachineEvidence {
        let bundledProvenance = try bundledProvenanceRecord(resourceURL: resourceURL)
        try preflightCurrentMachineEvidenceWrite(
            bundleRootURL: bundleRootURL,
            machine: machine,
            resourceURL: resourceURL
        )

        let stagingDirectoryURL = try makeStagingDirectory(machine: machine)
        do {
            let copiedProvenanceName = "\(machine).provenance.json"
            try copyArtifactToStaging(
                sourceURL: bundledProvenance.url,
                stagingDirectoryURL: stagingDirectoryURL,
                fileName: copiedProvenanceName
            )

            let versionOutputName = "\(machine).version.txt"
            let versionText = try versionRunner(resourceURL)
            try Data(versionText.utf8).write(
                to: stagingDirectoryURL.appendingPathComponent(versionOutputName),
                options: .atomic
            )

            let auditOutputName = "\(machine).app-audit.json"
            let audit = try collectAuditIntoStaging(
                stagingDirectoryURL: stagingDirectoryURL,
                fileName: auditOutputName,
                resourceURL: resourceURL
            )
            let auditState = try inspectCurrentAudit(
                stagingDirectoryURL: stagingDirectoryURL,
                fileName: auditOutputName,
                resourceURL: resourceURL,
                currentProvenance: bundledProvenance.manifest,
                audit: audit
            )

            let notarization = try stageNotarizationEvidenceIfPresent(
                stagingDirectoryURL: stagingDirectoryURL,
                machine: machine,
                resourceURL: resourceURL,
                currentProvenance: bundledProvenance.manifest,
                currentAudit: auditState
            )

            return PreparedCurrentMachineEvidence(
                stagingDirectoryURL: stagingDirectoryURL,
                versionOutputName: versionOutputName,
                copiedProvenanceName: copiedProvenanceName,
                auditOutputName: auditOutputName,
                audit: audit,
                auditState: auditState,
                notarization: notarization
            )
        } catch {
            try? fileManager.removeItem(at: stagingDirectoryURL)
            throw error
        }
    }

    private func publishPreparedCurrentMachineEvidence(
        _ prepared: PreparedCurrentMachineEvidence,
        bundleRootURL: URL
    ) throws {
        for outputName in prepared.outputNames {
            let sourceURL = prepared.stagingDirectoryURL.appendingPathComponent(outputName)
            let destinationURL = try safeBundleArtifactOutputURL(
                bundleRootURL: bundleRootURL,
                fileName: outputName
            )
            try replaceCopiedArtifact(sourceURL: sourceURL, destinationURL: destinationURL)
        }
    }

    private func makeStagingDirectory(machine: String) throws -> URL {
        let directoryURL = fileManager.temporaryDirectory
            .appendingPathComponent("supermover-packaging-\(machine)-\(UUID().uuidString)", isDirectory: true)
        try fileManager.createDirectory(at: directoryURL, withIntermediateDirectories: true)
        return directoryURL
    }

    private func copyArtifactToStaging(
        sourceURL: URL,
        stagingDirectoryURL: URL,
        fileName: String
    ) throws {
        try fileManager.copyItem(
            at: sourceURL,
            to: stagingDirectoryURL.appendingPathComponent(fileName)
        )
    }

    private func collectAuditIntoStaging(
        stagingDirectoryURL: URL,
        fileName: String,
        resourceURL: URL?
    ) throws -> AppAuditResult {
        try auditRunner(
            appBundleURL(from: resourceURL),
            stagingDirectoryURL.appendingPathComponent(fileName)
        )
    }

    private struct BundledProvenanceRecord {
        let url: URL
        let manifest: BundledProvenanceManifest
    }

    private func bundledProvenanceRecord(resourceURL: URL?) throws -> BundledProvenanceRecord {
        guard let resourceURL else {
            throw CollectionError.missingAppResources("Bundle.main.resourceURL")
        }
        let provenanceURL = resourceURL.appendingPathComponent("supermover-provenance.json")
        guard fileManager.fileExists(atPath: provenanceURL.path) else {
            throw CollectionError.missingBundledProvenance(provenanceURL.path)
        }
        let data = try Data(contentsOf: provenanceURL)
        guard let manifest = try? JSONDecoder().decode(BundledProvenanceManifest.self, from: data),
              manifest.schema == BundledProvenanceManifest.schemaID,
              !(manifest.git_commit?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true),
              !(manifest.cli_version?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true),
              manifest.cli_relative_path == BundledProvenanceManifest.bundledCLIRelativePath,
              !(manifest.build_profile?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true),
              !(manifest.signing?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ?? true) else {
            throw CollectionError.malformedBundledProvenance(provenanceURL.path)
        }
        return BundledProvenanceRecord(url: provenanceURL, manifest: manifest)
    }

    private static func requireAuditScript(repoRoot: URL?) throws -> URL {
        guard let repoRoot else {
            throw CollectionError.auditScriptUnavailable("repo root not found")
        }
        let scriptURL = repoRoot.appendingPathComponent("macos/script/audit-app.sh")
        guard FileManager.default.isExecutableFile(atPath: scriptURL.path) else {
            throw CollectionError.auditScriptUnavailable(scriptURL.path)
        }
        return scriptURL
    }

    private static func bundledAuditExecutableURL(appBundleURL: URL) -> URL? {
        let helperURL = appBundleURL.appendingPathComponent("Contents/Resources/bin/supermover-app-audit")
        guard FileManager.default.isExecutableFile(atPath: helperURL.path) else {
            return nil
        }
        return helperURL
    }

    private static func findRepoRoot() -> URL? {
        let cwd = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
        let rootPath = cwd.pathComponents.first.map { "/\($0)" } ?? "/"
        var current = cwd
        while true {
            let goMod = current.appendingPathComponent("go.mod").path
            let cliMain = current.appendingPathComponent("cmd/supermover/main.go").path
            if FileManager.default.fileExists(atPath: goMod) && FileManager.default.fileExists(atPath: cliMain) {
                return current
            }
            let parent = current.deletingLastPathComponent()
            if parent.path == current.path || current.path == rootPath {
                return nil
            }
            current = parent
        }
    }

    private func appBundleURL(from resourceURL: URL?) throws -> URL {
        guard let resourceURL else {
            throw CollectionError.missingAppResources("Bundle.main.resourceURL")
        }
        let contentsURL = resourceURL.deletingLastPathComponent()
        let appURL = contentsURL.deletingLastPathComponent()
        guard appURL.lastPathComponent.hasSuffix(".app") else {
            throw CollectionError.missingAppResources(appURL.path)
        }
        return appURL
    }

    private func stageNotarizationEvidenceIfPresent(
        stagingDirectoryURL: URL,
        machine: String,
        resourceURL: URL?,
        currentProvenance: BundledProvenanceManifest,
        currentAudit: CurrentAppAuditState
    ) throws -> PreparedCurrentMachineEvidence.PreparedNotarization? {
        let sourceURL = try notarizationEvidenceSourceURL(resourceURL: resourceURL)
        guard let sourceURL else {
            return nil
        }
        guard fileManager.fileExists(atPath: sourceURL.path) else {
            return nil
        }
        let data = try Data(contentsOf: sourceURL)
        guard let artifact = try? JSONDecoder().decode(CurrentAppNotarizationArtifact.self, from: data),
              artifact.schema == "supermover.macos.notarization.v1",
              let status = artifact.status?.trimmingCharacters(in: .whitespacesAndNewlines),
              !status.isEmpty else {
            throw CollectionError.malformedNotarizationOutput(sourceURL.path)
        }
        guard try notarizationAuditMatchesCurrentApp(
            artifact: artifact,
            resourceURL: resourceURL,
            currentProvenance: currentProvenance,
            currentAudit: currentAudit
        ) else {
            throw CollectionError.staleNotarizationOutput(sourceURL.path)
        }
        let outputName = "\(machine).notarization.json"
        let notaryLogOutputName = "\(machine).notary-log.json"
        let notaryLogURL = try canonicalNotaryLogURL(
            artifact: artifact,
            resourceURL: resourceURL
        )
        guard let notaryLogData = try? Data(contentsOf: notaryLogURL),
              AcceptanceInstalledAppReleaseEvidenceSummary.acceptedNotaryLog(
                  data: notaryLogData,
                  submissionID: artifact.submission?.id
              ) else {
            throw CollectionError.staleNotarizationOutput(sourceURL.path)
        }
        try copyArtifactToStaging(
            sourceURL: notaryLogURL,
            stagingDirectoryURL: stagingDirectoryURL,
            fileName: notaryLogOutputName
        )
        try writeBundleLocalNotarizationArtifact(
            data: data,
            stagingDirectoryURL: stagingDirectoryURL,
            fileName: outputName,
            notaryLogOutputName: notaryLogOutputName
        )
        return PreparedCurrentMachineEvidence.PreparedNotarization(
            outputName: outputName,
            notaryLogOutputName: notaryLogOutputName,
            status: status,
            submissionID: artifact.submission?.id?.trimmingCharacters(in: .whitespacesAndNewlines),
            submissionStatus: artifact.submission?.status?.trimmingCharacters(in: .whitespacesAndNewlines),
            authMode: artifact.auth_mode?.trimmingCharacters(in: .whitespacesAndNewlines),
            failurePresent: artifact.failure != nil,
            notaryLogPath: notaryLogOutputName,
            auditStatus: artifact.audit?.status?.trimmingCharacters(in: .whitespacesAndNewlines),
            auditReadiness: artifact.audit?.readiness?.trimmingCharacters(in: .whitespacesAndNewlines),
            auditPassReady: artifact.audit?.pass_ready
        )
    }

    private func inspectCurrentAudit(
        stagingDirectoryURL: URL,
        fileName: String,
        resourceURL: URL?,
        currentProvenance: BundledProvenanceManifest,
        audit: AppAuditResult
    ) throws -> CurrentAppAuditState {
        let outputURL = stagingDirectoryURL.appendingPathComponent(fileName)
        let data = try Data(contentsOf: outputURL)
        guard let artifact = try? JSONDecoder().decode(AppAuditArtifact.self, from: data),
              artifact.schema == "supermover.macos.app_audit.v1" else {
            throw CollectionError.malformedAuditOutput(outputURL.path)
        }

        let currentAppPath = try appBundleURL(from: resourceURL).path
        let artifactAppPath = artifact.app_path?.trimmingCharacters(in: .whitespacesAndNewlines)
        let appPathMatches = artifactAppPath == currentAppPath
        let provenanceMatches = artifact.provenance?.manifest == currentProvenance
        let installReady = audit.status == "pass"
            && audit.readiness == "distribution_ready"
            && audit.passReady
            && appPathMatches
            && provenanceMatches

        let failureMessage: String?
        if installReady {
            failureMessage = nil
        } else if audit.status != "pass" || audit.readiness != "distribution_ready" || !audit.passReady {
            failureMessage = "Local app audit for the current packaged app is \(audit.readiness) and not install-ready."
        } else if artifactAppPath == nil {
            failureMessage = "Local app audit for the current packaged app does not record app_path."
        } else if !appPathMatches {
            failureMessage = "Local app audit for the current packaged app does not match the current app bundle path."
        } else {
            failureMessage = "Local app audit for the current packaged app does not match the current bundled provenance."
        }

        return CurrentAppAuditState(
            status: audit.status,
            readiness: audit.readiness,
            passReady: audit.passReady,
            installReady: installReady,
            failureMessage: failureMessage
        )
    }

    private func removeStaleNotarizationEvidence(
        from evidence: inout [String: Any],
        machine: String,
        bundleRootURL: URL
    ) {
        let staleOutputName = "\(machine).notarization.json"
        let staleOutputURL = bundleRootURL.appendingPathComponent(staleOutputName)
        if pathExistsOrIsSymlink(staleOutputURL) {
            try? fileManager.removeItem(at: staleOutputURL)
        }
        let staleNotaryLogName = "\(machine).notary-log.json"
        let staleNotaryLogURL = bundleRootURL.appendingPathComponent(staleNotaryLogName)
        if pathExistsOrIsSymlink(staleNotaryLogURL) {
            try? fileManager.removeItem(at: staleNotaryLogURL)
        }

        guard var notarizationEvidence = evidence["notarization"] as? [String: Any] else {
            return
        }
        notarizationEvidence.removeValue(forKey: machine)
        if notarizationEvidence.isEmpty {
            evidence.removeValue(forKey: "notarization")
        } else {
            evidence["notarization"] = notarizationEvidence
        }
    }

    private func clearRecordedNotarizationEvidence(
        bundleRootURL: URL,
        machine: String
    ) throws {
        try metaStore.withMutableMetaDocument(bundleRootURL: bundleRootURL) { document in
            var evidence = document["evidence"] as? [String: Any] ?? [:]
            removeStaleNotarizationEvidence(
                from: &evidence,
                machine: machine,
                bundleRootURL: bundleRootURL
            )
            document["evidence"] = evidence
        }
    }

    private func shouldClearRecordedNotarizationEvidence(after error: Error) -> Bool {
        guard let error = error as? CollectionError else {
            return false
        }
        switch error {
        case .symlinkRejected, .malformedNotarizationOutput, .staleNotarizationOutput, .unsafeOutputArtifact:
            return true
        default:
            return false
        }
    }

    private func bundleArtifactOutputURL(
        bundleRootURL: URL,
        fileName: String
    ) throws -> URL {
        do {
            return try artifactAccess.artifactURL(
                relativePath: fileName,
                bundleRootURL: bundleRootURL
            )
        } catch let AcceptanceBundleArtifactAccess.AccessError.invalidBundleRoot(path) {
            throw CollectionError.invalidBundleRoot(path)
        } catch let AcceptanceBundleArtifactAccess.AccessError.symlinkRejected(path) {
            throw CollectionError.symlinkRejected(path)
        } catch let AcceptanceBundleArtifactAccess.AccessError.unsafeArtifactPath(path) {
            throw CollectionError.symlinkRejected(
                bundleRootURL.appendingPathComponent(path).path
            )
        } catch {
            throw CollectionError.invalidBundleRoot(bundleRootURL.path)
        }
    }

    private func safeBundleArtifactOutputURL(
        bundleRootURL: URL,
        fileName: String
    ) throws -> URL {
        let url = try bundleArtifactOutputURL(
            bundleRootURL: bundleRootURL,
            fileName: fileName
        )
        try rejectUnsafeExistingOutputArtifact(url)
        return url
    }

    private func rejectUnsafeExistingOutputArtifact(_ url: URL) throws {
        guard pathExistsOrIsSymlink(url) else {
            return
        }
        guard let values = try? url.resourceValues(
            forKeys: [.isRegularFileKey, .isSymbolicLinkKey, .linkCountKey]
        ),
              values.isRegularFile == true,
              values.isSymbolicLink != true,
              (values.linkCount ?? 1) == 1 else {
            throw CollectionError.unsafeOutputArtifact(url.path)
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

    private func notarizationEvidenceSourceURL(resourceURL: URL?) throws -> URL? {
        guard let resourceURL else {
            return nil
        }
        let appBundleURL = try appBundleURL(from: resourceURL)
        let siblingNotaryDir = canonicalNotaryDirectoryURL(appBundleURL: appBundleURL)
        let siblingNotary = siblingNotaryDir.appendingPathComponent("notarization.json")
        if pathExistsOrIsSymlink(siblingNotary) {
            try rejectSymlinkIfPresent(siblingNotaryDir)
            try rejectSymlinkIfPresent(siblingNotary)
            return siblingNotary
        }
        return nil
    }

    private func notarizationAuditMatchesCurrentApp(
        artifact: CurrentAppNotarizationArtifact,
        resourceURL: URL?,
        currentProvenance: BundledProvenanceManifest,
        currentAudit: CurrentAppAuditState
    ) throws -> Bool {
        guard let appBundleURL = try? appBundleURL(from: resourceURL),
              artifact.app_path?.trimmingCharacters(in: .whitespacesAndNewlines) == appBundleURL.path else {
            return false
        }
        guard let auditPath = artifact.audit?.path?.trimmingCharacters(in: .whitespacesAndNewlines),
              !auditPath.isEmpty else {
            return false
        }
        let expectedAuditURL = canonicalPostStapleAuditURL(appBundleURL: appBundleURL)
            .standardizedFileURL
        let auditURL = URL(fileURLWithPath: auditPath).standardizedFileURL
        guard auditURL.path == expectedAuditURL.path else {
            return false
        }
        let expectedNotaryLogURL = canonicalNotaryLogURL(appBundleURL: appBundleURL)
            .standardizedFileURL
        let notaryLogPath = artifact.notary_log?.path?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        let notaryLogURL = URL(fileURLWithPath: notaryLogPath).standardizedFileURL
        guard !notaryLogPath.isEmpty,
              notaryLogURL.path == expectedNotaryLogURL.path else {
            return false
        }
        try rejectSymlinkIfPresent(canonicalNotaryDirectoryURL(appBundleURL: appBundleURL))
        try rejectSymlinkIfPresent(expectedAuditURL)
        try rejectSymlinkIfPresent(expectedNotaryLogURL)
        guard pathExistsOrIsSymlink(expectedAuditURL),
              let auditData = try? Data(contentsOf: auditURL),
              let auditArtifact = try? JSONDecoder().decode(AppAuditArtifact.self, from: auditData),
              auditArtifact.schema == "supermover.macos.app_audit.v1" else {
            return false
        }
        guard pathExistsOrIsSymlink(expectedNotaryLogURL),
              fileManager.isReadableFile(atPath: expectedNotaryLogURL.path) else {
            return false
        }
        guard auditArtifact.app_path?.trimmingCharacters(in: .whitespacesAndNewlines) == appBundleURL.path else {
            return false
        }

        let auditStatus = auditArtifact.status?.trimmingCharacters(in: .whitespacesAndNewlines)
        let auditReadiness = auditArtifact.readiness?.trimmingCharacters(in: .whitespacesAndNewlines)
        let artifactAuditStatus = artifact.audit?.status?.trimmingCharacters(in: .whitespacesAndNewlines)
        let artifactAuditReadiness = artifact.audit?.readiness?.trimmingCharacters(in: .whitespacesAndNewlines)

        return auditStatus == artifactAuditStatus
            && auditReadiness == artifactAuditReadiness
            && auditArtifact.summary?.pass_ready == artifact.audit?.pass_ready
            && artifactAuditStatus == currentAudit.status
            && artifactAuditReadiness == currentAudit.readiness
            && artifact.audit?.pass_ready == currentAudit.passReady
            && auditArtifact.provenance?.manifest == currentProvenance
    }

    private func canonicalNotaryDirectoryURL(appBundleURL: URL) -> URL {
        appBundleURL
            .deletingLastPathComponent()
            .appendingPathComponent("\(appBundleURL.lastPathComponent).notary", isDirectory: true)
    }

    private func canonicalPostStapleAuditURL(appBundleURL: URL) -> URL {
        canonicalNotaryDirectoryURL(appBundleURL: appBundleURL)
            .appendingPathComponent("post-staple.audit.json")
    }

    private func canonicalNotaryLogURL(appBundleURL: URL) -> URL {
        canonicalNotaryDirectoryURL(appBundleURL: appBundleURL)
            .appendingPathComponent("notary-log.json")
    }

    private func canonicalNotaryLogURL(
        artifact: CurrentAppNotarizationArtifact,
        resourceURL: URL?
    ) throws -> URL {
        let appBundleURL = try appBundleURL(from: resourceURL)
        let expectedURL = canonicalNotaryLogURL(appBundleURL: appBundleURL).standardizedFileURL
        let notaryLogPath = artifact.notary_log?.path?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        let actualURL = URL(fileURLWithPath: notaryLogPath).standardizedFileURL
        guard !notaryLogPath.isEmpty,
              actualURL.path == expectedURL.path,
              pathExistsOrIsSymlink(expectedURL) else {
            throw CollectionError.staleNotarizationOutput(
                canonicalNotarizationSidecarURL(appBundleURL: appBundleURL).path
            )
        }
        try rejectSymlinkIfPresent(expectedURL)
        return expectedURL
    }

    private func canonicalNotarizationSidecarURL(appBundleURL: URL) -> URL {
        canonicalNotaryDirectoryURL(appBundleURL: appBundleURL)
            .appendingPathComponent("notarization.json")
    }

    private func writeBundleLocalNotarizationArtifact(
        data: Data,
        stagingDirectoryURL: URL,
        fileName: String,
        notaryLogOutputName: String
    ) throws {
        guard var object = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw CollectionError.malformedNotarizationOutput(fileName)
        }
        var notaryLog = object["notary_log"] as? [String: Any] ?? [:]
        notaryLog["path"] = notaryLogOutputName
        object["notary_log"] = notaryLog
        let rewritten = try JSONSerialization.data(
            withJSONObject: object,
            options: [.prettyPrinted, .sortedKeys]
        )
        try rewritten.write(
            to: stagingDirectoryURL.appendingPathComponent(fileName),
            options: .atomic
        )
    }

    private func pathExistsOrIsSymlink(_ url: URL) -> Bool {
        if fileManager.fileExists(atPath: url.path) {
            return true
        }
        if let values = try? url.resourceValues(forKeys: [.isSymbolicLinkKey]) {
            return values.isSymbolicLink == true
        }
        return false
    }

    private func rejectSymlinkIfPresent(_ url: URL) throws {
        guard pathExistsOrIsSymlink(url) else {
            return
        }
        let values = try url.resourceValues(forKeys: [.isSymbolicLinkKey])
        if values.isSymbolicLink == true {
            throw CollectionError.symlinkRejected(url.path)
        }
    }

    private static func runAudit(
        executableURL: URL,
        appURL: URL,
        outputURL: URL
    ) throws -> AppAuditResult {
        let process = Process()
        let stdoutPipe = Pipe()
        let stderrPipe = Pipe()
        process.executableURL = executableURL
        process.arguments = [appURL.path]
        process.standardOutput = stdoutPipe
        process.standardError = stderrPipe
        try process.run()
        process.waitUntilExit()

        let stdoutData = stdoutPipe.fileHandleForReading.readDataToEndOfFile()
        let stderrData = stderrPipe.fileHandleForReading.readDataToEndOfFile()
        try stdoutData.write(to: outputURL, options: .atomic)
        let stderr = String(decoding: stderrData, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
        if process.terminationStatus != 0 && process.terminationStatus != 1 {
            throw CollectionError.auditExecutionFailed(stderr.isEmpty ? "\(process.terminationStatus)" : stderr)
        }
        guard let artifact = try? JSONDecoder().decode(AppAuditArtifact.self, from: stdoutData),
              artifact.schema == "supermover.macos.app_audit.v1",
              let status = artifact.status,
              let readiness = artifact.readiness else {
            throw CollectionError.malformedAuditOutput(outputURL.path)
        }
        return AppAuditResult(
            exitCode: process.terminationStatus,
            status: status,
            readiness: readiness,
            passReady: artifact.summary?.pass_ready ?? false,
            blockingChecks: artifact.summary?.blocking_checks ?? 0
        )
    }

    private static func defaultVersionRunner(resourceURL: URL?) throws -> String {
        guard let resourceURL else {
            throw CollectionError.missingAppResources("Bundle.main.resourceURL")
        }
        let executableURL = resourceURL.appendingPathComponent("bin/supermover")
        guard FileManager.default.isExecutableFile(atPath: executableURL.path) else {
            throw CollectionError.missingAppResources(executableURL.path)
        }
        let process = Process()
        let stdoutPipe = Pipe()
        let stderrPipe = Pipe()
        process.executableURL = executableURL
        process.arguments = ["version"]
        process.standardOutput = stdoutPipe
        process.standardError = stderrPipe
        try process.run()
        process.waitUntilExit()
        let stdoutData = stdoutPipe.fileHandleForReading.readDataToEndOfFile()
        let stderrData = stderrPipe.fileHandleForReading.readDataToEndOfFile()
        if process.terminationStatus != 0 {
            let stderr = String(decoding: stderrData, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
            throw CollectionError.auditExecutionFailed(stderr.isEmpty ? "version exit \(process.terminationStatus)" : stderr)
        }
        return String(decoding: stdoutData, as: UTF8.self)
    }

    private static func defaultAuditRunner(appBundleURL: URL, outputURL: URL) throws -> AppAuditResult {
        if let helperURL = bundledAuditExecutableURL(appBundleURL: appBundleURL) {
            return try runAudit(executableURL: helperURL, appURL: appBundleURL, outputURL: outputURL)
        }
        let scriptURL = try requireAuditScript(repoRoot: findRepoRoot())
        return try runAudit(executableURL: scriptURL, appURL: appBundleURL, outputURL: outputURL)
    }
}
