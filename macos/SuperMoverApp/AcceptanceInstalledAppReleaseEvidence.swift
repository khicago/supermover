import Foundation

struct AcceptanceInstalledAppReleaseEvidenceSummary: Equatable {
    struct Machine: Equatable {
        let name: String
        let appAuditPresent: Bool
        let appAuditReady: Bool
        let appAuditReadiness: String?
        let appAuditFailureMessage: String?
        let notarizationPresent: Bool
        let notarizationReady: Bool
        let notarizationStatus: String?
        let notarizationFailureMessage: String?

        var ready: Bool {
            appAuditReady && notarizationReady
        }

        var failures: [String] {
            var failures: [String] = []
            if !appAuditReady {
                failures.append("\(name).app-audit.json is not install-ready")
            }
            if !notarizationReady {
                failures.append("\(name).notarization.json is not release-ready")
            }
            return failures
        }
    }

    let source: Machine
    let target: Machine

    var ok: Bool {
        source.ready && target.ready
    }

    var failures: [String] {
        source.failures + target.failures
    }

    var missingMachines: [String] {
        [source, target].filter { !$0.ready }.map(\.name)
    }

    static func notarizationIsReleaseReady(
        status: String,
        submissionID: String?,
        submissionStatus: String?,
        authMode: String?,
        failurePresent: Bool,
        notaryLogPath: String?,
        auditStatus: String?,
        auditReadiness: String?,
        auditPassReady: Bool?
    ) -> Bool {
        status == "pass"
            && notarizationSubmissionIDIsReleaseReady(submissionID)
            && submissionStatus == "Accepted"
            && notarizationAuthModeIsReleaseReady(authMode)
            && !failurePresent
            && cleanText(notaryLogPath) != nil
            && auditStatus == "pass"
            && auditReadiness == "distribution_ready"
            && auditPassReady == true
    }

    static func notarizationAuthModeIsReleaseReady(_ authMode: String?) -> Bool {
        switch cleanText(authMode) {
        case "keychain_profile", "api_key", "apple_id":
            return true
        default:
            return false
        }
    }

    static func notarizationSubmissionIDIsReleaseReady(_ submissionID: String?) -> Bool {
        guard let submissionID = cleanText(submissionID) else {
            return false
        }
        return UUID(uuidString: submissionID) != nil
    }

    static func evaluate(snapshot: AcceptanceBundleLoadedSnapshot) -> AcceptanceInstalledAppReleaseEvidenceSummary {
        let bundleRootURL = URL(fileURLWithPath: snapshot.bundleRootPath, isDirectory: true)
        return AcceptanceInstalledAppReleaseEvidenceSummary(
            source: machine(
                name: "source",
                audit: snapshot.sourceAppAuditArtifact,
                provenance: snapshot.sourceProvenanceArtifact,
                notarization: snapshot.sourceNotarizationArtifact,
                bundleRootURL: bundleRootURL
            ),
            target: machine(
                name: "target",
                audit: snapshot.targetAppAuditArtifact,
                provenance: snapshot.targetProvenanceArtifact,
                notarization: snapshot.targetNotarizationArtifact,
                bundleRootURL: bundleRootURL
            )
        )
    }

    private struct AuditState {
        let present: Bool
        let ready: Bool
        let appPath: String?
        let status: String?
        let readiness: String?
        let passReady: Bool
        let failureMessage: String?
    }

    private struct NotarizationState {
        let present: Bool
        let ready: Bool
        let status: String?
        let failureMessage: String?
    }

    private static func machine(
        name: String,
        audit: AcceptanceBundleSnapshot.AppAuditArtifact?,
        provenance: AcceptanceBundleSnapshot.ProvenanceArtifact?,
        notarization: AcceptanceBundleSnapshot.NotarizationArtifact?,
        bundleRootURL: URL?
    ) -> Machine {
        let auditState = appAuditState(name: name, audit: audit, provenance: provenance)
        let notarizationState = notarizationState(
            name: name,
            notarization: notarization,
            auditState: auditState,
            bundleRootURL: bundleRootURL
        )
        return Machine(
            name: name,
            appAuditPresent: auditState.present,
            appAuditReady: auditState.ready,
            appAuditReadiness: auditState.readiness,
            appAuditFailureMessage: auditState.failureMessage,
            notarizationPresent: notarizationState.present,
            notarizationReady: notarizationState.ready,
            notarizationStatus: notarizationState.status,
            notarizationFailureMessage: notarizationState.failureMessage
        )
    }

    private static func appAuditState(
        name: String,
        audit: AcceptanceBundleSnapshot.AppAuditArtifact?,
        provenance: AcceptanceBundleSnapshot.ProvenanceArtifact?
    ) -> AuditState {
        let status = cleanText(audit?.status)
        let readiness = cleanText(audit?.readiness)
        let passReady = audit?.summary?.pass_ready == true
        let installReady =
            audit?.schema == "supermover.macos.app_audit.v1"
            && status == "pass"
            && readiness == "distribution_ready"
            && passReady
        let appPath = cleanText(audit?.app_path)
        let provenanceMatches =
            provenance?.schema == "supermover.macos.provenance.v1"
            && audit?.provenance?.manifest == provenance
        let ready = installReady && appPath != nil && provenanceMatches

        let failureMessage: String?
        if ready {
            failureMessage = nil
        } else if !installReady {
            failureMessage = "\(name).app-audit.json is not install-ready"
        } else if appPath == nil {
            failureMessage = "\(name).app-audit.json does not record app_path"
        } else {
            failureMessage = "\(name).app-audit.json does not match \(name).provenance.json"
        }
        return AuditState(
            present: audit != nil,
            ready: ready,
            appPath: appPath,
            status: status,
            readiness: readiness,
            passReady: passReady,
            failureMessage: failureMessage
        )
    }

    private static func notarizationState(
        name: String,
        notarization: AcceptanceBundleSnapshot.NotarizationArtifact?,
        auditState: AuditState,
        bundleRootURL: URL?
    ) -> NotarizationState {
        let status = cleanText(notarization?.status)
        let notaryLogPath = cleanText(notarization?.notary_log?.path)
        let expectedNotaryLogPath = "\(name).notary-log.json"
        let auditPath = cleanText(notarization?.audit?.path)
        let expectedAuditPath = auditState.appPath.map { "\($0).notary/post-staple.audit.json" }
        let notaryLogReady = notaryLogPath.flatMap {
            bundleLocalAcceptedNotaryLogExists(
                relativePath: $0,
                submissionID: notarization?.submission?.id,
                bundleRootURL: bundleRootURL
            )
        } == true
        let statusReady =
            notarization?.schema == "supermover.macos.notarization.v1"
            && status != nil
            && AcceptanceInstalledAppReleaseEvidenceSummary.notarizationIsReleaseReady(
                status: notarization?.status ?? "",
                submissionID: notarization?.submission?.id,
                submissionStatus: notarization?.submission?.status,
                authMode: notarization?.auth_mode,
                failurePresent: notarization?.failure != nil,
                notaryLogPath: notaryLogPath,
                auditStatus: notarization?.audit?.status,
                auditReadiness: notarization?.audit?.readiness,
                auditPassReady: notarization?.audit?.pass_ready
            )
            && notaryLogPath == expectedNotaryLogPath
            && notaryLogReady
        let currentMatches =
            statusReady
            && auditState.ready
            && cleanText(notarization?.app_path) == auditState.appPath
            && auditPath != nil
            && auditPath == expectedAuditPath
            && cleanText(notarization?.audit?.status) == auditState.status
            && cleanText(notarization?.audit?.readiness) == auditState.readiness
            && notarization?.audit?.pass_ready == auditState.passReady
        let ready = currentMatches

        let failureMessage: String?
        if ready {
            failureMessage = nil
        } else if !statusReady {
            failureMessage = "\(name).notarization.json is not release-ready"
        } else if auditPath == nil {
            failureMessage = "\(name).notarization.json does not record audit.path"
        } else {
            failureMessage = "\(name).notarization.json does not match \(name).app-audit.json and \(name).provenance.json"
        }
        return NotarizationState(
            present: notarization != nil,
            ready: ready,
            status: status,
            failureMessage: failureMessage
        )
    }

    private static func cleanText(_ value: String?) -> String? {
        guard let value else {
            return nil
        }
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    static func acceptedNotaryLog(data: Data, submissionID: String?) -> Bool {
        guard let expectedJobID = normalizedSubmissionUUID(submissionID) else {
            return false
        }
        guard let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              object["status"] as? String == "Accepted" else {
            return false
        }
        if let issues = object["issues"], !(issues is [Any]) && !(issues is NSNull) {
            return false
        }
        return normalizedSubmissionUUID(object["jobId"] as? String) == expectedJobID
    }

    private static func normalizedSubmissionUUID(_ submissionID: String?) -> String? {
        guard let submissionID = cleanText(submissionID),
              let uuid = UUID(uuidString: submissionID) else {
            return nil
        }
        return uuid.uuidString.lowercased()
    }

    private static func bundleLocalAcceptedNotaryLogExists(
        relativePath: String,
        submissionID: String?,
        bundleRootURL: URL?
    ) -> Bool {
        guard let bundleRootURL else {
            return false
        }
        guard let data = try? AcceptanceBundleArtifactAccess().requiredArtifactData(
            relativePath: relativePath,
            bundleRootURL: bundleRootURL
        ) else {
            return false
        }
        return acceptedNotaryLog(data: data, submissionID: submissionID)
    }
}
