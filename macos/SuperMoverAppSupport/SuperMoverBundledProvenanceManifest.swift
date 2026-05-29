import Foundation

/// Shared shape for the packaged app provenance manifest carried by the bundle,
/// copied into acceptance bundles, and embedded into app-audit evidence.
public struct SuperMoverBundledProvenanceManifest: Codable, Equatable, Sendable {
    public static let schemaID = "supermover.macos.provenance.v1"
    public static let bundledCLIRelativePath = "Contents/Resources/bin/supermover"

    public let schema: String?
    public let app_bundle_id: String?
    public let app_version: String?
    public let build_profile: String?
    public let git_commit: String?
    public let git_dirty: Bool?
    public let cli_version: String?
    public let cli_relative_path: String?
    public let built_at: String?
    public let signing: String?

    public init(
        schema: String?,
        app_bundle_id: String?,
        app_version: String?,
        build_profile: String?,
        git_commit: String?,
        git_dirty: Bool?,
        cli_version: String?,
        cli_relative_path: String?,
        built_at: String?,
        signing: String?
    ) {
        self.schema = schema
        self.app_bundle_id = app_bundle_id
        self.app_version = app_version
        self.build_profile = build_profile
        self.git_commit = git_commit
        self.git_dirty = git_dirty
        self.cli_version = cli_version
        self.cli_relative_path = cli_relative_path
        self.built_at = built_at
        self.signing = signing
    }
}
