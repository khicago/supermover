import CryptoKit
import Darwin
import Foundation

public struct PackagedAppAuditor {
    private static let requiredEntitlements = [
        "com.apple.security.files.user-selected.read-write",
        "com.apple.security.network.client",
        "com.apple.security.network.server",
    ]

    private enum NotarizationArtifactAccessError: Error {
        case symlinkRejected(String)
        case nonRegularArtifact(String)
    }

    private let fileManager: FileManager
    private let commandRunner: PackagedAppAuditCommandRunner

    public init(
        fileManager: FileManager = .default,
        commandRunner: PackagedAppAuditCommandRunner = .init()
    ) {
        self.fileManager = fileManager
        self.commandRunner = commandRunner
    }

    public func audit(appURL: URL) -> PackagedAppAuditReport {
        var state = AuditState()
        let checkedAt = ISO8601DateFormatter().string(from: Date())
        let expectedCLIRelativePath = SuperMoverBundledProvenanceManifest.bundledCLIRelativePath

        let contentsURL = appURL.appendingPathComponent("Contents", isDirectory: true)
        let infoPlistURL = contentsURL.appendingPathComponent("Info.plist")
        let resourcesURL = contentsURL.appendingPathComponent("Resources", isDirectory: true)
        let provenanceURL = resourcesURL.appendingPathComponent("supermover-provenance.json")

        if directoryExists(appURL) {
            state.addCheck(id: "app.exists", status: "pass", severity: "required", summary: "app bundle exists", detail: appURL.path)
        } else {
            state.addCheck(id: "app.exists", status: "blocked", severity: "blocking", summary: "app bundle is missing", detail: appURL.path)
        }

        let plist = inspectPlist(infoPlistURL: infoPlistURL, state: &state)
        let iconName = iconFileName(plist.iconFile)
        let iconURL = resourcesURL.appendingPathComponent(iconName)
        if let iconFile = plist.iconFile, !iconFile.isEmpty, fileExists(iconURL) {
            state.addCheck(id: "app.icon.exists", status: "pass", severity: "required", summary: "app icon resource exists", detail: "Contents/Resources/\(iconName)")
        } else {
            state.addCheck(id: "app.icon.exists", status: "blocked", severity: "blocking", summary: "app icon resource is missing", detail: "Contents/Resources/\(iconName)")
        }

        let executablePath = plist.executable.map { contentsURL.appendingPathComponent("MacOS/\($0)") }
        if let executablePath, fileManager.isExecutableFile(atPath: executablePath.path) {
            state.addCheck(id: "app.executable", status: "pass", severity: "required", summary: "app executable exists and is executable", detail: "Contents/MacOS/\(plist.executable ?? "")")
        } else {
            state.addCheck(id: "app.executable", status: "blocked", severity: "blocking", summary: "app executable is missing or not executable", detail: "Contents/MacOS/\(plist.executable ?? "")")
        }

        let provenance = inspectProvenance(
            provenanceURL: provenanceURL,
            plist: plist,
            state: &state
        )
        let cli = inspectCLI(
            resourcesURL: resourcesURL,
            provenance: provenance,
            expectedCLIRelativePath: expectedCLIRelativePath,
            state: &state
        )

        let hashes = recordHashes(
            infoPlistURL: infoPlistURL,
            iconURL: iconURL,
            executableURL: executablePath,
            cliURL: URL(fileURLWithPath: cli.path),
            provenanceURL: provenanceURL,
            state: &state
        )
        let appCodesign = inspectCodesign(
            subject: "app",
            targetURL: appURL,
            state: &state
        )
        let cliCodesign = inspectCodesign(
            subject: "cli",
            targetURL: URL(fileURLWithPath: cli.path),
            state: &state
        )
        let spctl = inspectAssessment(
            tool: "spctl",
            commandDescription: "spctl --assess --type execute --verbose=4",
            executableURL: URL(fileURLWithPath: "/usr/sbin/spctl"),
            arguments: ["--assess", "--type", "execute", "--verbose=4", appURL.path],
            requiredPath: appURL,
            checkID: "spctl.assess",
            passSummary: "spctl assessment passed",
            passDetail: "Gatekeeper accepted the app for execution.",
            blockedSummary: "spctl assessment failed",
            unavailableSummary: "spctl is unavailable",
            unavailableDetail: "Cannot assess Gatekeeper execution readiness.",
            state: &state
        )
        let stapler = inspectStapler(appURL: appURL, state: &state)
        let notarizationSidecar = inspectNotarizationSidecar(
            appURL: appURL,
            currentProvenance: provenance,
            state: &state
        )

        let overallStatus: String
        let readiness: String
        if state.blockingCount > 0 {
            overallStatus = "blocked"
            readiness = "blocked"
        } else if state.reviewCount > 0 {
            overallStatus = "review_only"
            readiness = "review_only"
        } else {
            overallStatus = "pass"
            readiness = "distribution_ready"
        }

        return PackagedAppAuditReport(
            schema: PackagedAppAuditReport.schemaID,
            status: overallStatus,
            readiness: readiness,
            checkedAt: checkedAt,
            appPath: appURL.path,
            summary: PackagedAppAuditSummary(
                passReady: overallStatus == "pass",
                blockingChecks: state.blockingCount,
                reviewChecks: state.reviewCount,
                expectedCLIRelativePath: expectedCLIRelativePath,
                note: overallStatus == "pass"
                    ? "Developer ID signing, clean provenance, Gatekeeper assessment, and stapled notarization evidence are present."
                    : "This audit is local evidence only and does not prove notarized distribution readiness."
            ),
            plist: plist,
            provenance: provenance,
            cli: cli,
            hashes: hashes,
            signing: PackagedAppAuditSigning(
                codesign: .init(app: appCodesign, cli: cliCodesign),
                spctl: spctl,
                stapler: stapler
            ),
            notarizationSidecar: notarizationSidecar,
            checks: state.checks
        )
    }

    private func inspectPlist(infoPlistURL: URL, state: inout AuditState) -> PackagedAppAuditPlist {
        let lint = runTool(executableURL: URL(fileURLWithPath: "/usr/bin/plutil"), arguments: ["-lint", infoPlistURL.path])
        if fileExists(infoPlistURL) {
            state.addCheck(id: "plist.exists", status: "pass", severity: "required", summary: "Info.plist exists", detail: infoPlistURL.path)
        } else {
            state.addCheck(id: "plist.exists", status: "blocked", severity: "blocking", summary: "Info.plist is missing", detail: infoPlistURL.path)
        }
        state.addCheck(
            id: "plist.lint",
            status: lint.status,
            severity: "blocking",
            summary: lint.status == "pass" ? "Info.plist is valid" : "Info.plist is invalid",
            detail: lint.output.trimmingCharacters(in: .whitespacesAndNewlines)
        )

        let plistObject = (try? Data(contentsOf: infoPlistURL))
            .flatMap { try? PropertyListSerialization.propertyList(from: $0, format: nil) as? [String: Any] } ?? [:]
        let plist = PackagedAppAuditPlist(
            path: infoPlistURL.path,
            lint: lint.result,
            bundleID: cleanText(plistObject["CFBundleIdentifier"] as? String),
            iconFile: cleanText(plistObject["CFBundleIconFile"] as? String),
            shortVersion: cleanText(plistObject["CFBundleShortVersionString"] as? String),
            bundleVersion: cleanText(plistObject["CFBundleVersion"] as? String),
            executable: cleanText(plistObject["CFBundleExecutable"] as? String),
            packageType: cleanText(plistObject["CFBundlePackageType"] as? String),
            localNetworkUsageDescription: cleanText(plistObject["NSLocalNetworkUsageDescription"] as? String)
        )

        state.require(plist.bundleID, id: "plist.bundle_id", summary: "Info.plist bundle identifier is present", blockedSummary: "Info.plist is missing CFBundleIdentifier", detail: plist.bundleID ?? "CFBundleIdentifier is required for provenance comparison.")
        state.require(plist.iconFile, id: "plist.icon_file", summary: "Info.plist app icon reference is present", blockedSummary: "Info.plist is missing CFBundleIconFile", detail: plist.iconFile ?? "CFBundleIconFile is required for the packaged app icon.")
        state.require(plist.shortVersion, id: "plist.short_version", summary: "Info.plist short version is present", blockedSummary: "Info.plist is missing CFBundleShortVersionString", detail: plist.shortVersion ?? "CFBundleShortVersionString is required for provenance comparison.")
        state.require(plist.executable, id: "plist.executable", summary: "Info.plist executable is present", blockedSummary: "Info.plist is missing CFBundleExecutable", detail: plist.executable ?? "CFBundleExecutable is required to locate the app executable.")
        state.require(plist.localNetworkUsageDescription, id: "plist.local_network_usage", summary: "Info.plist Local Network usage description is present", blockedSummary: "Info.plist is missing NSLocalNetworkUsageDescription", detail: plist.localNetworkUsageDescription ?? "Installed-app LAN discovery, pairing, and serve flows require an explicit Local Network usage description.")
        return plist
    }

    private func inspectProvenance(provenanceURL: URL, plist: PackagedAppAuditPlist, state: inout AuditState) -> PackagedAppAuditProvenance {
        guard fileExists(provenanceURL) else {
            state.addCheck(id: "provenance.exists", status: "blocked", severity: "blocking", summary: "provenance manifest is missing", detail: provenanceURL.path)
            return PackagedAppAuditProvenance(path: provenanceURL.path, exists: false, loaded: false, error: nil, manifest: nil)
        }
        let manifest: SuperMoverBundledProvenanceManifest?
        let parseError: String?
        do {
            manifest = try JSONDecoder().decode(SuperMoverBundledProvenanceManifest.self, from: Data(contentsOf: provenanceURL))
            parseError = nil
            state.addCheck(id: "provenance.parse", status: "pass", severity: "required", summary: "provenance JSON parses", detail: provenanceURL.path)
        } catch {
            manifest = nil
            parseError = String(describing: error)
            state.addCheck(id: "provenance.parse", status: "blocked", severity: "blocking", summary: "provenance JSON is malformed", detail: parseError ?? provenanceURL.path)
        }
        let record = PackagedAppAuditProvenance(path: provenanceURL.path, exists: true, loaded: manifest != nil, error: parseError, manifest: manifest)
        guard let manifest else { return record }

        state.expect(manifest.schema == SuperMoverBundledProvenanceManifest.schemaID, id: "provenance.schema", passSummary: "provenance schema matches", blockedSummary: "provenance schema mismatch", detail: manifest.schema ?? "missing schema")
        state.expect(cleanText(manifest.app_bundle_id) == plist.bundleID && plist.bundleID != nil, id: "provenance.app_bundle_id", passSummary: "provenance bundle id matches Info.plist", blockedSummary: "provenance bundle id does not match Info.plist", detail: "provenance='\(manifest.app_bundle_id ?? "")' plist='\(plist.bundleID ?? "")'")
        state.expect(cleanText(manifest.app_version) == plist.shortVersion && plist.shortVersion != nil, id: "provenance.app_version", passSummary: "provenance app version matches Info.plist", blockedSummary: "provenance app version does not match Info.plist", detail: "provenance='\(manifest.app_version ?? "")' plist='\(plist.shortVersion ?? "")'")
        state.expect(cleanText(manifest.cli_relative_path) == SuperMoverBundledProvenanceManifest.bundledCLIRelativePath, id: "provenance.cli_relative_path", passSummary: "provenance CLI path is expected", blockedSummary: "provenance CLI path is unexpected", detail: manifest.cli_relative_path ?? "missing cli_relative_path")
        state.expect(cleanText(manifest.build_profile) != nil, id: "provenance.build_profile", passSummary: "provenance build profile is present", blockedSummary: "provenance build profile is missing", detail: manifest.build_profile ?? "missing build_profile")
        state.expect(cleanText(manifest.git_commit) != nil, id: "provenance.git_commit", passSummary: "provenance git commit is present", blockedSummary: "provenance git commit is missing", detail: manifest.git_commit ?? "missing git_commit")
        state.expect(cleanText(manifest.cli_version) != nil, id: "provenance.cli_version", passSummary: "provenance CLI version is present", blockedSummary: "provenance CLI version is missing", detail: manifest.cli_version ?? "missing cli_version")
        state.expect(cleanText(manifest.built_at) != nil, id: "provenance.built_at", passSummary: "provenance build timestamp is present", blockedSummary: "provenance build timestamp is missing", detail: manifest.built_at ?? "missing built_at")
        state.expect(manifest.git_dirty == false, id: "provenance.git_dirty", passSummary: "provenance dirty bit records a clean build", blockedSummary: "provenance dirty bit is not release-ready", detail: String(describing: manifest.git_dirty))
        let signing = cleanText(manifest.signing) ?? ""
        let signingReady = !signing.isEmpty && signing != "unsigned" && signing != "-"
        state.expect(signingReady, id: "provenance.signing", passSummary: "provenance records a signing identity", blockedSummary: "provenance records a non-release signing mode", detail: signing)
        return record
    }

    private func inspectCLI(resourcesURL: URL, provenance: PackagedAppAuditProvenance, expectedCLIRelativePath: String, state: inout AuditState) -> PackagedAppAuditCLI {
        let cliURL = resourcesURL.appendingPathComponent("bin/supermover")
        if fileManager.isExecutableFile(atPath: cliURL.path) {
            state.addCheck(id: "cli.exists", status: "pass", severity: "required", summary: "bundled CLI exists and is executable", detail: expectedCLIRelativePath)
        } else {
            state.addCheck(id: "cli.exists", status: "blocked", severity: "blocking", summary: "bundled CLI is missing or not executable", detail: expectedCLIRelativePath)
        }
        let versionRun = runTool(executableURL: cliURL, arguments: ["version"])
        state.addCheck(
            id: "cli.version.command",
            status: versionRun.status,
            severity: "blocking",
            summary: versionRun.status == "pass" ? "bundled CLI version command succeeded" : "bundled CLI version command failed",
            detail: versionRun.output.trimmingCharacters(in: .whitespacesAndNewlines)
        )
        let version = cleanText(versionRun.output.components(separatedBy: .newlines).first)
        let manifestVersion = provenance.manifest?.cli_version
        state.expect(versionRun.result.exitCode == 0 && version == manifestVersion && manifestVersion != nil, id: "cli.version.provenance", passSummary: "bundled CLI version matches provenance", blockedSummary: "bundled CLI version does not match provenance", detail: "cli='\(version ?? "")' provenance='\(manifestVersion ?? "")'")
        return PackagedAppAuditCLI(
            path: cliURL.path,
            relativePath: expectedCLIRelativePath,
            version: version,
            versionCommand: versionRun.result
        )
    }

    private func recordHashes(infoPlistURL: URL, iconURL: URL, executableURL: URL?, cliURL: URL, provenanceURL: URL, state: inout AuditState) -> [PackagedAppAuditHash] {
        let candidates: [(String, URL?)] = [
            ("Contents/Info.plist", infoPlistURL),
            ("Contents/Resources/\(iconURL.lastPathComponent)", iconURL),
            (executableURL.map { "Contents/MacOS/\($0.lastPathComponent)" } ?? "", executableURL),
            (SuperMoverBundledProvenanceManifest.bundledCLIRelativePath, cliURL),
            ("Contents/Resources/supermover-provenance.json", provenanceURL),
        ]
        let hashes: [PackagedAppAuditHash] = candidates.compactMap { relativePath, url in
            guard let url, fileExists(url), let data = try? Data(contentsOf: url) else { return nil }
            let digest = SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
            return PackagedAppAuditHash(path: relativePath, sha256: digest, bytes: data.count)
        }
        state.addCheck(
            id: "hashes.basic",
            status: hashes.isEmpty ? "blocked" : "pass",
            severity: hashes.isEmpty ? "blocking" : "required",
            summary: hashes.isEmpty ? "no bundle file hashes were recorded" : "basic bundle file hashes were recorded",
            detail: hashes.isEmpty ? "Expected at least Info.plist, app icon, app executable, CLI, or provenance." : "Info.plist, app icon, app executable, CLI, and provenance are hashed when present."
        )
        return hashes
    }

    private func inspectCodesign(subject: String, targetURL: URL, state: inout AuditState) -> PackagedAppAuditCodesignSubject {
        guard fileExists(targetURL) else {
            state.addCheck(id: "codesign.\(subject).exists", status: "blocked", severity: "blocking", summary: "\(subject) path is missing", detail: targetURL.path)
            return PackagedAppAuditCodesignSubject(subject: subject, path: targetURL.path, available: false, verify: nil, details: nil, entitlementsDump: nil, adHoc: nil, hardenedRuntime: nil, developerIDApplication: nil, teamIdentifier: nil, authorities: [], entitlements: nil)
        }

        let verify = runTool(executableURL: URL(fileURLWithPath: "/usr/bin/codesign"), arguments: ["--verify", "--strict", "--verbose=4", targetURL.path])
        let details = runTool(executableURL: URL(fileURLWithPath: "/usr/bin/codesign"), arguments: ["-dv", "--verbose=4", targetURL.path])
        let entitlementsDump = runTool(executableURL: URL(fileURLWithPath: "/usr/bin/codesign"), arguments: ["-d", "--entitlements", "-", "--xml", targetURL.path])
        let authorities = details.output.split(separator: "\n").compactMap { line -> String? in
            let text = String(line)
            guard text.hasPrefix("Authority=") else { return nil }
            return String(text.dropFirst("Authority=".count))
        }
        let adHoc = details.output.contains("Signature=adhoc")
        let hardenedRuntime = details.output.range(of: #"CodeDirectory .*flags=.*runtime"#, options: .regularExpression) != nil
        let developerID = authorities.contains { $0.hasPrefix("Developer ID Application") }
        let teamIdentifier = details.output.split(separator: "\n").first { $0.hasPrefix("TeamIdentifier=") }.map { String($0.dropFirst("TeamIdentifier=".count)) }.flatMap(cleanText)
        let entitlements = parseEntitlements(from: entitlementsDump.output)

        state.addCheck(id: "codesign.\(subject).verify", status: verify.status, severity: "blocking", summary: verify.status == "pass" ? "\(subject) code signature verifies" : "\(subject) code signature does not verify", detail: verify.output.trimmingCharacters(in: .whitespacesAndNewlines))
        state.expect(details.result.exitCode == 0 && developerID, id: "codesign.\(subject).identity", passSummary: "\(subject) has Developer ID Application authority", blockedSummary: "\(subject) is not signed for Developer ID distribution", detail: details.output.trimmingCharacters(in: .whitespacesAndNewlines))
        state.expect(details.result.exitCode == 0 && !adHoc, id: "codesign.\(subject).ad_hoc", passSummary: "\(subject) is not ad-hoc signed", blockedSummary: "\(subject) is ad-hoc signed", detail: details.output.trimmingCharacters(in: .whitespacesAndNewlines))
        state.expect(details.result.exitCode == 0 && hardenedRuntime, id: "codesign.\(subject).runtime", passSummary: "\(subject) has hardened runtime enabled", blockedSummary: "\(subject) is missing hardened runtime", detail: details.output.trimmingCharacters(in: .whitespacesAndNewlines))
        state.addCheck(id: "codesign.\(subject).entitlements", status: entitlements != nil ? "pass" : "blocked", severity: entitlements != nil ? "required" : "blocking", summary: entitlements != nil ? "\(subject) entitlements were dumped" : "\(subject) entitlements are unavailable", detail: entitlementsDump.output.trimmingCharacters(in: .whitespacesAndNewlines))
        for key in Self.requiredEntitlements {
            let present = entitlements?[key] == true
            state.expect(present, id: "codesign.\(subject).entitlement.\(key)", passSummary: "\(subject) entitlement is present", blockedSummary: "\(subject) entitlement is missing", detail: key)
        }

        return PackagedAppAuditCodesignSubject(
            subject: subject,
            path: targetURL.path,
            available: true,
            verify: verify.result,
            details: details.result,
            entitlementsDump: entitlementsDump.result,
            adHoc: adHoc,
            hardenedRuntime: hardenedRuntime,
            developerIDApplication: developerID,
            teamIdentifier: teamIdentifier,
            authorities: authorities,
            entitlements: entitlements
        )
    }

    private func inspectAssessment(
        tool: String,
        commandDescription: String,
        executableURL: URL,
        arguments: [String],
        requiredPath: URL,
        checkID: String,
        passSummary: String,
        passDetail: String,
        blockedSummary: String,
        unavailableSummary: String,
        unavailableDetail: String,
        state: inout AuditState
    ) -> PackagedAppAuditAssessment {
        guard fileExists(requiredPath) else {
            state.addCheck(id: checkID, status: "blocked", severity: "blocking", summary: "\(tool) assessment cannot run", detail: "App bundle path is missing.")
            return PackagedAppAuditAssessment(tool: tool, available: false, status: "blocked", exitCode: 127, command: commandDescription, output: "")
        }
        let run = runTool(executableURL: executableURL, arguments: arguments)
        state.addCheck(
            id: checkID,
            status: run.status,
            severity: "blocking",
            summary: run.status == "pass" ? passSummary : (run.result.exitCode == 127 ? unavailableSummary : blockedSummary),
            detail: run.result.exitCode == 127 ? unavailableDetail : run.output.trimmingCharacters(in: .whitespacesAndNewlines)
        )
        return PackagedAppAuditAssessment(tool: tool, available: run.result.exitCode != 127, status: run.status, exitCode: run.result.exitCode, command: commandDescription, output: run.output)
    }

    private func inspectStapler(appURL: URL, state: inout AuditState) -> PackagedAppAuditAssessment {
        let lookup = runTool(executableURL: URL(fileURLWithPath: "/usr/bin/xcrun"), arguments: ["-f", "stapler"])
        guard lookup.result.exitCode == 0 else {
            state.addCheck(id: "stapler.validate", status: "blocked", severity: "blocking", summary: "stapler is unavailable", detail: "Cannot validate a notarization ticket staple.")
            return PackagedAppAuditAssessment(tool: "stapler", available: false, status: "blocked", exitCode: lookup.result.exitCode, command: "xcrun stapler validate", output: lookup.output)
        }
        return inspectAssessment(
            tool: "stapler",
            commandDescription: "xcrun stapler validate",
            executableURL: URL(fileURLWithPath: "/usr/bin/xcrun"),
            arguments: ["stapler", "validate", appURL.path],
            requiredPath: appURL,
            checkID: "stapler.validate",
            passSummary: "stapler validation passed",
            passDetail: "A stapled notarization ticket was validated.",
            blockedSummary: "stapler validation failed",
            unavailableSummary: "stapler is unavailable",
            unavailableDetail: "Cannot validate a notarization ticket staple.",
            state: &state
        )
    }

    private func inspectNotarizationSidecar(
        appURL: URL,
        currentProvenance: PackagedAppAuditProvenance,
        state: inout AuditState
    ) -> PackagedAppAuditNotarizationSidecar {
        let sidecarURL = canonicalNotarizationSidecarURL(appURL: appURL)
        guard pathExistsOrIsSymlink(sidecarURL) else {
            return PackagedAppAuditNotarizationSidecar(
                path: sidecarURL.path,
                exists: false,
                loaded: false,
                error: nil,
                current: false,
                status: nil,
                authMode: nil,
                submissionID: nil,
                submissionStatus: nil,
                notaryLogPath: nil,
                notaryLogAccepted: false,
                failurePresent: false,
                releaseReady: false,
                audit: nil
            )
        }

        state.addCheck(
            id: "notarization.sidecar.exists",
            status: "pass",
            severity: "required",
            summary: "canonical notarization sidecar exists",
            detail: sidecarURL.path
        )

        let sidecarData: Data
        let sidecarDocument: NotarizationSidecarDocument
        do {
            try rejectSymlinkIfPresent(canonicalNotaryDirectoryURL(appURL: appURL))
            try rejectSymlinkIfPresent(sidecarURL)
            sidecarData = try Data(contentsOf: sidecarURL)
            sidecarDocument = try JSONDecoder().decode(NotarizationSidecarDocument.self, from: sidecarData)
            state.addCheck(
                id: "notarization.sidecar.parse",
                status: "pass",
                severity: "required",
                summary: "canonical notarization sidecar parses",
                detail: sidecarURL.path
            )
        } catch let NotarizationArtifactAccessError.symlinkRejected(path) {
            let detail = "unsafe symlink path: \(path)"
            state.addCheck(
                id: "notarization.sidecar.parse",
                status: "blocked",
                severity: "blocking",
                summary: "canonical notarization sidecar uses an unsafe symlink path",
                detail: detail
            )
            state.addCheck(
                id: "notarization.sidecar.currentness",
                status: "blocked",
                severity: "blocking",
                summary: "canonical notarization sidecar does not match the current app and bundled provenance",
                detail: sidecarURL.path
            )
            state.addCheck(
                id: "notarization.sidecar.release_ready",
                status: "blocked",
                severity: "blocking",
                summary: "canonical notarization sidecar does not record release-ready notarization evidence",
                detail: detail
            )
            return PackagedAppAuditNotarizationSidecar(
                path: sidecarURL.path,
                exists: true,
                loaded: false,
                error: detail,
                current: false,
                status: nil,
                authMode: nil,
                submissionID: nil,
                submissionStatus: nil,
                notaryLogPath: nil,
                notaryLogAccepted: false,
                failurePresent: false,
                releaseReady: false,
                audit: nil
            )
        } catch {
            let detail = String(describing: error)
            state.addCheck(
                id: "notarization.sidecar.parse",
                status: "blocked",
                severity: "blocking",
                summary: "canonical notarization sidecar is malformed",
                detail: detail
            )
            return PackagedAppAuditNotarizationSidecar(
                path: sidecarURL.path,
                exists: true,
                loaded: false,
                error: detail,
                current: false,
                status: nil,
                authMode: nil,
                submissionID: nil,
                submissionStatus: nil,
                notaryLogPath: nil,
                notaryLogAccepted: false,
                failurePresent: false,
                releaseReady: false,
                audit: nil
            )
        }

        let sidecarStatus = cleanText(sidecarDocument.status)
        let sidecarAuthMode = cleanText(sidecarDocument.auth_mode)
        let submissionID = cleanText(sidecarDocument.submission?.id)
        let submissionStatus = cleanText(sidecarDocument.submission?.status)
        let notaryLogPath = cleanText(sidecarDocument.notary_log?.path)
        let sidecarAuditStatus = cleanText(sidecarDocument.audit?.status)
        let sidecarAuditReadiness = cleanText(sidecarDocument.audit?.readiness)
        let sidecarAuditPassReady = sidecarDocument.audit?.pass_ready
        let sidecarAppMatches = cleanText(sidecarDocument.app_path) == appURL.path

        state.expect(
            sidecarDocument.schema == "supermover.macos.notarization.v1",
            id: "notarization.sidecar.schema",
            passSummary: "canonical notarization sidecar schema matches",
            blockedSummary: "canonical notarization sidecar schema is invalid",
            detail: sidecarDocument.schema ?? "missing schema"
        )
        state.expect(
            sidecarStatus != nil,
            id: "notarization.sidecar.status",
            passSummary: "canonical notarization sidecar status is present",
            blockedSummary: "canonical notarization sidecar status is missing",
            detail: sidecarDocument.status ?? "missing status"
        )
        state.expect(
            sidecarAppMatches,
            id: "notarization.sidecar.app_path",
            passSummary: "canonical notarization sidecar app path matches the current app",
            blockedSummary: "canonical notarization sidecar app path does not match the current app",
            detail: "sidecar='\(sidecarDocument.app_path ?? "")' app='\(appURL.path)'"
        )
        state.expect(
            submissionID != nil,
            id: "notarization.sidecar.submission_id",
            passSummary: "canonical notarization sidecar submission id is present",
            blockedSummary: "canonical notarization sidecar submission id is missing",
            detail: submissionID ?? "missing submission id"
        )
        state.expect(
            submissionStatus == "Accepted",
            id: "notarization.sidecar.submission",
            passSummary: "canonical notarization sidecar submission is Accepted",
            blockedSummary: "canonical notarization sidecar submission is not Accepted",
            detail: submissionStatus ?? "missing submission status"
        )

        let auditPath = cleanText(sidecarDocument.audit?.path)
        let expectedAuditURL = canonicalPostStapleAuditURL(appURL: appURL)
            .standardizedFileURL
        let expectedNotaryLogURL = canonicalNotaryLogURL(appURL: appURL)
            .standardizedFileURL
        let auditURL = auditPath.map { URL(fileURLWithPath: $0).standardizedFileURL }
        let notaryLogURL = notaryLogPath.map { URL(fileURLWithPath: $0).standardizedFileURL }
        let auditPathMatchesCanonicalSidecar = auditURL?.path == expectedAuditURL.path
        let notaryLogMatchesCanonicalSidecar = notaryLogURL?.path == expectedNotaryLogURL.path
        let notaryLogAccepted: Bool
        let notaryLogArtifactDetail: String?
        if notaryLogMatchesCanonicalSidecar {
            do {
                try rejectSymlinkIfPresent(canonicalNotaryDirectoryURL(appURL: appURL))
                try requireSingleLinkRegularArtifact(expectedNotaryLogURL)
                let notaryLogData = try Data(contentsOf: expectedNotaryLogURL)
                if notaryLogIsAccepted(notaryLogData, submissionID: submissionID) {
                    notaryLogAccepted = true
                    notaryLogArtifactDetail = nil
                } else {
                    notaryLogAccepted = false
                    notaryLogArtifactDetail = "not accepted notarization log evidence: \(expectedNotaryLogURL.path)"
                }
            } catch let NotarizationArtifactAccessError.symlinkRejected(path) {
                notaryLogAccepted = false
                notaryLogArtifactDetail = "unsafe symlink path: \(path)"
            } catch let NotarizationArtifactAccessError.nonRegularArtifact(path) {
                notaryLogAccepted = false
                notaryLogArtifactDetail = "not a single-link regular artifact: \(path)"
            } catch {
                notaryLogAccepted = false
                notaryLogArtifactDetail = String(describing: error)
            }
        } else {
            notaryLogAccepted = false
            notaryLogArtifactDetail = nil
        }
        state.expect(
            notaryLogMatchesCanonicalSidecar && notaryLogAccepted,
            id: "notarization.sidecar.notary_log",
            passSummary: "canonical notarization sidecar references accepted current notary log evidence",
            blockedSummary: "canonical notarization sidecar notary log is missing or not accepted",
            detail: notaryLogArtifactDetail ?? notaryLogPath ?? "missing notary_log.path"
        )
        let auditExists: Bool
        let auditArtifactDetail: String?
        if auditPathMatchesCanonicalSidecar {
            do {
                try rejectSymlinkIfPresent(canonicalNotaryDirectoryURL(appURL: appURL))
                try requireSingleLinkRegularArtifact(expectedAuditURL)
                auditExists = true
                auditArtifactDetail = nil
            } catch let NotarizationArtifactAccessError.symlinkRejected(path) {
                auditExists = false
                auditArtifactDetail = "unsafe symlink path: \(path)"
            } catch let NotarizationArtifactAccessError.nonRegularArtifact(path) {
                auditExists = false
                auditArtifactDetail = "not a single-link regular artifact: \(path)"
            } catch {
                auditExists = false
                auditArtifactDetail = String(describing: error)
            }
        } else {
            auditExists = false
            auditArtifactDetail = nil
        }
        state.expect(
            auditExists,
            id: "notarization.sidecar.audit.exists",
            passSummary: "canonical notarization sidecar references an existing post-staple audit",
            blockedSummary: "canonical notarization sidecar post-staple audit is missing",
            detail: auditArtifactDetail ?? auditPath ?? "missing audit.path"
        )

        var auditRecord = PackagedAppAuditNotarizationSidecar.Audit(
            path: auditPath,
            exists: auditExists,
            loaded: false,
            error: nil,
            status: nil,
            readiness: nil,
            passReady: nil,
            currentApp: false,
            currentProvenance: false,
            consistentWithSidecar: false
        )
        var auditSchemaValid = false

        if let auditURL, auditExists {
            do {
                let auditDocument = try JSONDecoder().decode(AuditSummaryDocument.self, from: Data(contentsOf: auditURL))
                auditRecord = PackagedAppAuditNotarizationSidecar.Audit(
                    path: auditPath,
                    exists: true,
                    loaded: true,
                    error: nil,
                    status: cleanText(auditDocument.status),
                    readiness: cleanText(auditDocument.readiness),
                    passReady: auditDocument.summary?.pass_ready,
                    currentApp: cleanText(auditDocument.app_path) == appURL.path,
                    currentProvenance: auditDocument.provenance?.manifest == currentProvenance.manifest && currentProvenance.manifest != nil,
                    consistentWithSidecar:
                        cleanText(auditDocument.status) == sidecarAuditStatus
                        && cleanText(auditDocument.readiness) == sidecarAuditReadiness
                        && auditDocument.summary?.pass_ready == sidecarAuditPassReady
                )
                state.addCheck(
                    id: "notarization.sidecar.audit.parse",
                    status: "pass",
                    severity: "required",
                    summary: "canonical notarization sidecar post-staple audit parses",
                    detail: auditURL.path
                )
                auditSchemaValid = auditDocument.schema == PackagedAppAuditReport.schemaID
                state.expect(
                    auditSchemaValid,
                    id: "notarization.sidecar.audit.schema",
                    passSummary: "canonical notarization sidecar post-staple audit schema matches",
                    blockedSummary: "canonical notarization sidecar post-staple audit schema is invalid",
                    detail: auditDocument.schema ?? "missing schema"
                )
                state.expect(
                    auditRecord.currentApp,
                    id: "notarization.sidecar.audit.app_path",
                    passSummary: "canonical notarization sidecar post-staple audit app path matches the current app",
                    blockedSummary: "canonical notarization sidecar post-staple audit app path does not match the current app",
                    detail: "audit='\(auditDocument.app_path ?? "")' app='\(appURL.path)'"
                )
                state.expect(
                    auditRecord.consistentWithSidecar,
                    id: "notarization.sidecar.audit.consistency",
                    passSummary: "canonical notarization sidecar matches its post-staple audit readiness",
                    blockedSummary: "canonical notarization sidecar does not match its post-staple audit readiness",
                    detail: "sidecar status='\(sidecarAuditStatus ?? "")' readiness='\(sidecarAuditReadiness ?? "")' pass_ready='\(String(describing: sidecarAuditPassReady))'"
                )
                state.expect(
                    auditRecord.currentProvenance,
                    id: "notarization.sidecar.audit.provenance",
                    passSummary: "canonical notarization sidecar post-staple audit matches current bundled provenance",
                    blockedSummary: "canonical notarization sidecar post-staple audit does not match current bundled provenance",
                    detail: auditURL.path
                )
            } catch {
                let detail = String(describing: error)
                auditRecord = PackagedAppAuditNotarizationSidecar.Audit(
                    path: auditPath,
                    exists: true,
                    loaded: false,
                    error: detail,
                    status: nil,
                    readiness: nil,
                    passReady: nil,
                    currentApp: false,
                    currentProvenance: false,
                    consistentWithSidecar: false
                )
                state.addCheck(
                    id: "notarization.sidecar.audit.parse",
                    status: "blocked",
                    severity: "blocking",
                    summary: "canonical notarization sidecar post-staple audit is malformed",
                    detail: detail
                )
            }
        } else if let auditArtifactDetail {
            auditRecord = PackagedAppAuditNotarizationSidecar.Audit(
                path: auditPath,
                exists: false,
                loaded: false,
                error: auditArtifactDetail,
                status: nil,
                readiness: nil,
                passReady: nil,
                currentApp: false,
                currentProvenance: false,
                consistentWithSidecar: false
            )
            state.addCheck(
                id: "notarization.sidecar.audit.parse",
                status: "blocked",
                severity: "blocking",
                summary: "canonical notarization sidecar post-staple audit uses an unsafe artifact path",
                detail: auditArtifactDetail
            )
        } else {
            state.addCheck(
                id: "notarization.sidecar.audit.parse",
                status: "blocked",
                severity: "blocking",
                summary: "canonical notarization sidecar post-staple audit is malformed",
                detail: auditPath ?? "missing audit.path"
            )
        }

        let sidecarWellFormed =
            sidecarDocument.schema == "supermover.macos.notarization.v1"
            && sidecarStatus != nil
        let current = sidecarWellFormed
            && auditPathMatchesCanonicalSidecar
            && notaryLogMatchesCanonicalSidecar
            && notaryLogAccepted
            && auditSchemaValid
            && sidecarAppMatches
            && auditRecord.loaded
            && auditRecord.currentApp
            && auditRecord.currentProvenance
            && auditRecord.consistentWithSidecar
        state.expect(
            current,
            id: "notarization.sidecar.currentness",
            passSummary: "canonical notarization sidecar matches the current app and bundled provenance",
            blockedSummary: "canonical notarization sidecar does not match the current app and bundled provenance",
            detail: sidecarURL.path
        )

        let releaseReady = current
            && sidecarStatus == "pass"
            && notarizationSubmissionIDIsReleaseReady(submissionID)
            && submissionStatus == "Accepted"
            && notarizationAuthModeIsReleaseReady(sidecarAuthMode)
            && !sidecarDocument.failurePresent
            && notaryLogPath != nil
            && notaryLogAccepted
            && sidecarAuditStatus == "pass"
            && sidecarAuditReadiness == "distribution_ready"
            && sidecarAuditPassReady == true
        state.expect(
            releaseReady,
            id: "notarization.sidecar.release_ready",
            passSummary: "canonical notarization sidecar records release-ready notarization evidence",
            blockedSummary: "canonical notarization sidecar does not record release-ready notarization evidence",
            detail: "status='\(sidecarStatus ?? "")' auth_mode='\(sidecarAuthMode ?? "")' submission.id='\(submissionID ?? "")' submission='\(submissionStatus ?? "")' failure_present='\(sidecarDocument.failurePresent)' notary_log.path='\(notaryLogPath ?? "")' notary_log_accepted='\(notaryLogAccepted)' audit.status='\(sidecarAuditStatus ?? "")' audit.readiness='\(sidecarAuditReadiness ?? "")' audit.pass_ready='\(String(describing: sidecarAuditPassReady))'"
        )

        return PackagedAppAuditNotarizationSidecar(
            path: sidecarURL.path,
            exists: true,
            loaded: true,
            error: nil,
            current: current,
            status: sidecarStatus,
            authMode: sidecarAuthMode,
            submissionID: submissionID,
            submissionStatus: submissionStatus,
            notaryLogPath: notaryLogPath,
            notaryLogAccepted: notaryLogAccepted,
            failurePresent: sidecarDocument.failurePresent,
            releaseReady: releaseReady,
            audit: auditRecord
        )
    }

    private func runTool(executableURL: URL, arguments: [String]) -> (result: PackagedAppAuditCommandResult, status: String, output: String) {
        guard fileExists(executableURL) || fileManager.isExecutableFile(atPath: executableURL.path) else {
            return (.init(exitCode: 127, status: "blocked", output: ""), "blocked", "")
        }
        do {
            let output = try commandRunner.run(executableURL: executableURL, arguments: arguments)
            let status = output.exitCode == 0 ? "pass" : "blocked"
            return (.init(exitCode: output.exitCode, status: status, output: output.output), status, output.output)
        } catch {
            return (.init(exitCode: 127, status: "blocked", output: String(describing: error)), "blocked", String(describing: error))
        }
    }

    private func parseEntitlements(from output: String) -> [String: Bool]? {
        guard let start = output.range(of: "<?xml") ?? output.range(of: "<plist") else {
            return nil
        }
        guard let data = String(output[start.lowerBound...]).data(using: .utf8),
              let plist = try? PropertyListSerialization.propertyList(from: data, format: nil) as? [String: Any] else {
            return nil
        }
        var result: [String: Bool] = [:]
        for (key, value) in plist {
            if let flag = value as? Bool {
                result[key] = flag
            }
        }
        return result
    }

    private func iconFileName(_ iconFile: String?) -> String {
        guard let iconFile = cleanText(iconFile) else {
            return ".missing.icns"
        }
        return iconFile.hasSuffix(".icns") ? iconFile : "\(iconFile).icns"
    }

    private func cleanText(_ value: String?) -> String? {
        guard let value else { return nil }
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    private func notarizationAuthModeIsReleaseReady(_ authMode: String?) -> Bool {
        switch cleanText(authMode) {
        case "keychain_profile", "api_key", "apple_id":
            return true
        default:
            return false
        }
    }

    private func notarizationSubmissionIDIsReleaseReady(_ submissionID: String?) -> Bool {
        guard let submissionID = cleanText(submissionID) else {
            return false
        }
        return UUID(uuidString: submissionID) != nil
    }

    private func notaryLogIsAccepted(_ data: Data, submissionID: String?) -> Bool {
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

    private func normalizedSubmissionUUID(_ submissionID: String?) -> String? {
        guard let submissionID = cleanText(submissionID),
              let uuid = UUID(uuidString: submissionID) else {
            return nil
        }
        return uuid.uuidString.lowercased()
    }

    private func canonicalNotaryDirectoryURL(appURL: URL) -> URL {
        appURL
            .deletingLastPathComponent()
            .appendingPathComponent("\(appURL.lastPathComponent).notary", isDirectory: true)
    }

    private func canonicalNotarizationSidecarURL(appURL: URL) -> URL {
        canonicalNotaryDirectoryURL(appURL: appURL).appendingPathComponent("notarization.json")
    }

    private func canonicalPostStapleAuditURL(appURL: URL) -> URL {
        canonicalNotaryDirectoryURL(appURL: appURL).appendingPathComponent("post-staple.audit.json")
    }

    private func canonicalNotaryLogURL(appURL: URL) -> URL {
        canonicalNotaryDirectoryURL(appURL: appURL).appendingPathComponent("notary-log.json")
    }

    private func fileExists(_ url: URL) -> Bool {
        fileManager.fileExists(atPath: url.path)
    }

    private func pathExistsOrIsSymlink(_ url: URL) -> Bool {
        if fileExists(url) {
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
            throw NotarizationArtifactAccessError.symlinkRejected(url.path)
        }
    }

    private func requireSingleLinkRegularArtifact(_ url: URL) throws {
        try rejectSymlinkIfPresent(url)
        var statBuffer = stat()
        guard lstat(url.path, &statBuffer) == 0 else {
            throw NotarizationArtifactAccessError.nonRegularArtifact(url.path)
        }
        let mode = statBuffer.st_mode & S_IFMT
        guard mode == S_IFREG, statBuffer.st_nlink == 1 else {
            throw NotarizationArtifactAccessError.nonRegularArtifact(url.path)
        }
    }

    private func directoryExists(_ url: URL) -> Bool {
        var isDirectory: ObjCBool = false
        return fileManager.fileExists(atPath: url.path, isDirectory: &isDirectory) && isDirectory.boolValue
    }
}

private struct NotarizationSidecarDocument: Decodable {
    struct NotaryLog: Decodable {
        let path: String?
    }

    struct Submission: Decodable {
        let id: String?
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
    let app_path: String?
    let submission: Submission?
    let notary_log: NotaryLog?
    let audit: Audit?
    let failurePresent: Bool

    enum CodingKeys: String, CodingKey {
        case schema
        case status
        case auth_mode
        case app_path
        case submission
        case notary_log
        case audit
        case failure
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        schema = try container.decodeIfPresent(String.self, forKey: .schema)
        status = try container.decodeIfPresent(String.self, forKey: .status)
        auth_mode = try container.decodeIfPresent(String.self, forKey: .auth_mode)
        app_path = try container.decodeIfPresent(String.self, forKey: .app_path)
        submission = try container.decodeIfPresent(Submission.self, forKey: .submission)
        notary_log = try container.decodeIfPresent(NotaryLog.self, forKey: .notary_log)
        audit = try container.decodeIfPresent(Audit.self, forKey: .audit)
        if container.contains(.failure) {
            failurePresent = !(try container.decodeNil(forKey: .failure))
        } else {
            failurePresent = false
        }
    }
}

private struct AuditSummaryDocument: Decodable {
    struct Summary: Decodable {
        let pass_ready: Bool?
    }

    struct Provenance: Decodable {
        let manifest: SuperMoverBundledProvenanceManifest?
    }

    let schema: String?
    let status: String?
    let readiness: String?
    let app_path: String?
    let summary: Summary?
    let provenance: Provenance?
}

private struct AuditState {
    var checks: [PackagedAppAuditCheck] = []
    var blockingCount = 0
    var reviewCount = 0

    mutating func addCheck(id: String, status: String, severity: String, summary: String, detail: String) {
        checks.append(.init(id: id, status: status, severity: severity, summary: summary, detail: detail))
        switch status {
        case "blocked":
            blockingCount += 1
        case "review":
            reviewCount += 1
        default:
            break
        }
    }

    mutating func expect(_ condition: Bool, id: String, passSummary: String, blockedSummary: String, detail: String) {
        addCheck(
            id: id,
            status: condition ? "pass" : "blocked",
            severity: condition ? "required" : "blocking",
            summary: condition ? passSummary : blockedSummary,
            detail: detail
        )
    }

    mutating func require(_ value: String?, id: String, summary: String, blockedSummary: String, detail: String) {
        expect(value != nil, id: id, passSummary: summary, blockedSummary: blockedSummary, detail: detail)
    }
}
