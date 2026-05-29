import Foundation

public struct PackagedAppAuditCheck: Codable, Equatable, Sendable {
    public let id: String
    public let status: String
    public let severity: String
    public let summary: String
    public let detail: String

    public init(id: String, status: String, severity: String, summary: String, detail: String) {
        self.id = id
        self.status = status
        self.severity = severity
        self.summary = summary
        self.detail = detail
    }
}

public struct PackagedAppAuditCommandResult: Codable, Equatable, Sendable {
    public let exitCode: Int32
    public let status: String
    public let output: String

    enum CodingKeys: String, CodingKey {
        case exitCode = "exit_code"
        case status
        case output
    }

    public init(exitCode: Int32, status: String, output: String) {
        self.exitCode = exitCode
        self.status = status
        self.output = output
    }
}

public struct PackagedAppAuditSummary: Codable, Equatable, Sendable {
    public let passReady: Bool
    public let blockingChecks: Int
    public let reviewChecks: Int
    public let expectedCLIRelativePath: String
    public let note: String

    enum CodingKeys: String, CodingKey {
        case passReady = "pass_ready"
        case blockingChecks = "blocking_checks"
        case reviewChecks = "review_checks"
        case expectedCLIRelativePath = "expected_cli_relative_path"
        case note
    }
}

public struct PackagedAppAuditPlist: Codable, Equatable, Sendable {
    public let path: String
    public let lint: PackagedAppAuditCommandResult
    public let bundleID: String?
    public let iconFile: String?
    public let shortVersion: String?
    public let bundleVersion: String?
    public let executable: String?
    public let packageType: String?
    public let localNetworkUsageDescription: String?

    enum CodingKeys: String, CodingKey {
        case path
        case lint
        case bundleID = "bundle_id"
        case iconFile = "icon_file"
        case shortVersion = "short_version"
        case bundleVersion = "bundle_version"
        case executable
        case packageType = "package_type"
        case localNetworkUsageDescription = "local_network_usage_description"
    }
}

public struct PackagedAppAuditProvenance: Codable, Equatable, Sendable {
    public let path: String
    public let exists: Bool
    public let loaded: Bool
    public let error: String?
    public let manifest: SuperMoverBundledProvenanceManifest?
}

public struct PackagedAppAuditCLI: Codable, Equatable, Sendable {
    public let path: String
    public let relativePath: String
    public let version: String?
    public let versionCommand: PackagedAppAuditCommandResult

    enum CodingKeys: String, CodingKey {
        case path
        case relativePath = "relative_path"
        case version
        case versionCommand = "version_command"
    }
}

public struct PackagedAppAuditHash: Codable, Equatable, Sendable {
    public let path: String
    public let sha256: String
    public let bytes: Int
}

public struct PackagedAppAuditCodesignSubject: Codable, Equatable, Sendable {
    public let subject: String
    public let path: String
    public let available: Bool
    public let verify: PackagedAppAuditCommandResult?
    public let details: PackagedAppAuditCommandResult?
    public let entitlementsDump: PackagedAppAuditCommandResult?
    public let adHoc: Bool?
    public let hardenedRuntime: Bool?
    public let developerIDApplication: Bool?
    public let teamIdentifier: String?
    public let authorities: [String]
    public let entitlements: [String: Bool]?

    enum CodingKeys: String, CodingKey {
        case subject
        case path
        case available
        case verify
        case details
        case entitlementsDump = "entitlements_dump"
        case adHoc = "ad_hoc"
        case hardenedRuntime = "hardened_runtime"
        case developerIDApplication = "developer_id_application"
        case teamIdentifier = "team_identifier"
        case authorities
        case entitlements
    }
}

public struct PackagedAppAuditAssessment: Codable, Equatable, Sendable {
    public let tool: String
    public let available: Bool
    public let status: String
    public let exitCode: Int32
    public let command: String
    public let output: String

    enum CodingKeys: String, CodingKey {
        case tool
        case available
        case status
        case exitCode = "exit_code"
        case command
        case output
    }
}

public struct PackagedAppAuditSigning: Codable, Equatable, Sendable {
    public struct Codesign: Codable, Equatable, Sendable {
        public let app: PackagedAppAuditCodesignSubject
        public let cli: PackagedAppAuditCodesignSubject
    }

    public let codesign: Codesign
    public let spctl: PackagedAppAuditAssessment
    public let stapler: PackagedAppAuditAssessment
}

public struct PackagedAppAuditNotarizationSidecar: Codable, Equatable, Sendable {
    public struct Audit: Codable, Equatable, Sendable {
        public let path: String?
        public let exists: Bool
        public let loaded: Bool
        public let error: String?
        public let status: String?
        public let readiness: String?
        public let passReady: Bool?
        public let currentApp: Bool
        public let currentProvenance: Bool
        public let consistentWithSidecar: Bool

        enum CodingKeys: String, CodingKey {
            case path
            case exists
            case loaded
            case error
            case status
            case readiness
            case passReady = "pass_ready"
            case currentApp = "current_app"
            case currentProvenance = "current_provenance"
            case consistentWithSidecar = "consistent_with_sidecar"
        }
    }

    public let path: String
    public let exists: Bool
    public let loaded: Bool
    public let error: String?
    public let current: Bool
    public let status: String?
    public let authMode: String?
    public let submissionID: String?
    public let submissionStatus: String?
    public let notaryLogPath: String?
    public let notaryLogAccepted: Bool
    public let failurePresent: Bool
    public let releaseReady: Bool
    public let audit: Audit?

    enum CodingKeys: String, CodingKey {
        case path
        case exists
        case loaded
        case error
        case current
        case status
        case authMode = "auth_mode"
        case submissionID = "submission_id"
        case submissionStatus = "submission_status"
        case notaryLogPath = "notary_log_path"
        case notaryLogAccepted = "notary_log_accepted"
        case failurePresent = "failure_present"
        case releaseReady = "release_ready"
        case audit
    }

    public init(
        path: String,
        exists: Bool,
        loaded: Bool,
        error: String?,
        current: Bool,
        status: String?,
        authMode: String?,
        submissionID: String?,
        submissionStatus: String?,
        notaryLogPath: String?,
        notaryLogAccepted: Bool,
        failurePresent: Bool,
        releaseReady: Bool,
        audit: Audit?
    ) {
        self.path = path
        self.exists = exists
        self.loaded = loaded
        self.error = error
        self.current = current
        self.status = status
        self.authMode = authMode
        self.submissionID = submissionID
        self.submissionStatus = submissionStatus
        self.notaryLogPath = notaryLogPath
        self.notaryLogAccepted = notaryLogAccepted
        self.failurePresent = failurePresent
        self.releaseReady = releaseReady
        self.audit = audit
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        path = try container.decode(String.self, forKey: .path)
        exists = try container.decode(Bool.self, forKey: .exists)
        loaded = try container.decode(Bool.self, forKey: .loaded)
        error = try container.decodeIfPresent(String.self, forKey: .error)
        current = try container.decode(Bool.self, forKey: .current)
        status = try container.decodeIfPresent(String.self, forKey: .status)
        authMode = try container.decodeIfPresent(String.self, forKey: .authMode)
        submissionID = try container.decodeIfPresent(String.self, forKey: .submissionID)
        submissionStatus = try container.decodeIfPresent(String.self, forKey: .submissionStatus)
        notaryLogPath = try container.decodeIfPresent(String.self, forKey: .notaryLogPath)
        notaryLogAccepted = try container.decodeIfPresent(Bool.self, forKey: .notaryLogAccepted) ?? false
        failurePresent = try container.decodeIfPresent(Bool.self, forKey: .failurePresent) ?? false
        releaseReady = try container.decode(Bool.self, forKey: .releaseReady)
        audit = try container.decodeIfPresent(Audit.self, forKey: .audit)
    }
}

public struct PackagedAppAuditReport: Codable, Equatable, Sendable {
    public static let schemaID = "supermover.macos.app_audit.v1"

    public let schema: String
    public let status: String
    public let readiness: String
    public let checkedAt: String
    public let appPath: String
    public let summary: PackagedAppAuditSummary
    public let plist: PackagedAppAuditPlist
    public let provenance: PackagedAppAuditProvenance
    public let cli: PackagedAppAuditCLI
    public let hashes: [PackagedAppAuditHash]
    public let signing: PackagedAppAuditSigning
    public let notarizationSidecar: PackagedAppAuditNotarizationSidecar
    public let checks: [PackagedAppAuditCheck]

    enum CodingKeys: String, CodingKey {
        case schema
        case status
        case readiness
        case checkedAt = "checked_at"
        case appPath = "app_path"
        case summary
        case plist
        case provenance
        case cli
        case hashes
        case signing
        case notarizationSidecar = "notarization_sidecar"
        case checks
    }
}
