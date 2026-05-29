import Foundation
import XCTest
@testable import SuperMoverApp

struct AcceptanceProcessResult {
    let stdout: String
    let stderr: String
    let exitCode: Int32
}

private final class AcceptanceProcessOutputData: @unchecked Sendable {
    private let lock = NSLock()
    private var data = Data()

    func set(_ newData: Data) {
        lock.lock()
        data = newData
        lock.unlock()
    }

    func value() -> Data {
        lock.lock()
        let current = data
        lock.unlock()
        return current
    }
}

struct AcceptancePackagedAppFixture {
    let appURL: URL
    let resourcesURL: URL
    let binURL: URL
    let provenanceManifest: [String: Any]

    var canonicalNotaryDirectoryURL: URL {
        AcceptanceReleaseEvidenceFixtures.canonicalNotaryDirectoryURL(appBundleURL: appURL)
    }

    var canonicalPostStapleAuditURL: URL {
        AcceptanceReleaseEvidenceFixtures.canonicalPostStapleAuditURL(appBundleURL: appURL)
    }

    var canonicalNotarizationSidecarURL: URL {
        AcceptanceReleaseEvidenceFixtures.canonicalNotarizationSidecarURL(appBundleURL: appURL)
    }

    var canonicalNotaryLogURL: URL {
        AcceptanceReleaseEvidenceFixtures.canonicalNotaryLogURL(appBundleURL: appURL)
    }
}

enum AcceptanceScriptHarness {
    static func makeDirectory(named name: String) throws -> URL {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(
            "\(name)-\(UUID().uuidString)",
            isDirectory: true
        )
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }

    static func repoRootURL(file: StaticString = #filePath) -> URL {
        URL(fileURLWithPath: "\(file)")
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
    }

    static func runProcessAllowFailure(
        executableURL: URL,
        arguments: [String],
        environment: [String: String],
        currentDirectoryURL: URL
    ) throws -> AcceptanceProcessResult {
        let process = Process()
        process.executableURL = executableURL
        process.arguments = arguments
        process.currentDirectoryURL = currentDirectoryURL

        var mergedEnvironment = ProcessInfo.processInfo.environment
        for (key, value) in environment {
            mergedEnvironment[key] = value
        }
        process.environment = mergedEnvironment

        let stdoutPipe = Pipe()
        let stderrPipe = Pipe()
        process.standardOutput = stdoutPipe
        process.standardError = stderrPipe

        let outputGroup = DispatchGroup()
        let stdoutData = AcceptanceProcessOutputData()
        let stderrData = AcceptanceProcessOutputData()

        try process.run()

        outputGroup.enter()
        DispatchQueue.global(qos: .userInitiated).async {
            let data = stdoutPipe.fileHandleForReading.readDataToEndOfFile()
            stdoutData.set(data)
            outputGroup.leave()
        }

        outputGroup.enter()
        DispatchQueue.global(qos: .userInitiated).async {
            let data = stderrPipe.fileHandleForReading.readDataToEndOfFile()
            stderrData.set(data)
            outputGroup.leave()
        }

        process.waitUntilExit()
        outputGroup.wait()

        return AcceptanceProcessResult(
            stdout: String(decoding: stdoutData.value(), as: UTF8.self),
            stderr: String(decoding: stderrData.value(), as: UTF8.self),
            exitCode: process.terminationStatus
        )
    }

    static func runProcess(
        executableURL: URL,
        arguments: [String],
        environment: [String: String],
        currentDirectoryURL: URL
    ) throws -> AcceptanceProcessResult {
        let result = try runProcessAllowFailure(
            executableURL: executableURL,
            arguments: arguments,
            environment: environment,
            currentDirectoryURL: currentDirectoryURL
        )
        if result.exitCode != 0 {
            XCTFail("stderr:\n\(result.stderr)")
        }
        return result
    }
}

enum AcceptanceReleaseEvidenceFixtures {
    static let defaultSubmissionID = "11111111-1111-1111-1111-111111111111"

    static func canonicalNotaryDirectoryURL(appBundleURL: URL) -> URL {
        appBundleURL
            .deletingLastPathComponent()
            .appendingPathComponent("\(appBundleURL.lastPathComponent).notary", isDirectory: true)
    }

    static func canonicalPostStapleAuditURL(appBundleURL: URL) -> URL {
        canonicalNotaryDirectoryURL(appBundleURL: appBundleURL)
            .appendingPathComponent("post-staple.audit.json")
    }

    static func canonicalNotarizationSidecarURL(appBundleURL: URL) -> URL {
        canonicalNotaryDirectoryURL(appBundleURL: appBundleURL)
            .appendingPathComponent("notarization.json")
    }

    static func canonicalNotaryLogURL(appBundleURL: URL) -> URL {
        canonicalNotaryDirectoryURL(appBundleURL: appBundleURL)
            .appendingPathComponent("notary-log.json")
    }

    static func canonicalPostStapleAuditPath(appPath: String) -> String {
        canonicalPostStapleAuditURL(
            appBundleURL: URL(fileURLWithPath: appPath, isDirectory: true)
        ).path
    }

    static func developerIDProvenanceManifest(
        cliVersion: String = "supermover test-build"
    ) -> [String: Any] {
        [
            "schema": "supermover.macos.provenance.v1",
            "app_bundle_id": "dev.supermover.macapp",
            "app_version": "0.1.0",
            "build_profile": "test",
            "git_commit": "deadbeefdead",
            "git_dirty": false,
            "cli_version": cliVersion,
            "cli_relative_path": "Contents/Resources/bin/supermover",
            "built_at": "2026-06-03T00:00:00Z",
            "signing": "Developer ID Application: Test (TEAMID1234)",
        ]
    }

    static func bundledProvenanceJSON(
        cliVersion: String = "supermover test-build"
    ) throws -> String {
        try jsonString(developerIDProvenanceManifest(cliVersion: cliVersion))
    }

    static func appAuditJSON(
        appPath: String,
        provenanceManifest: [String: Any]? = nil,
        status: String = "pass",
        readiness: String = "distribution_ready",
        passReady: Bool = true,
        blockingChecks: Int = 0,
        reviewChecks: Int = 0
    ) throws -> String {
        var root: [String: Any] = [
            "schema": "supermover.macos.app_audit.v1",
            "status": status,
            "readiness": readiness,
            "app_path": appPath,
            "summary": [
                "pass_ready": passReady,
                "blocking_checks": blockingChecks,
                "review_checks": reviewChecks,
            ],
        ]
        if let provenanceManifest {
            root["provenance"] = [
                "path": "\(appPath)/Contents/Resources/supermover-provenance.json",
                "manifest": provenanceManifest,
            ]
        }
        return try jsonString(root)
    }

    static func notarizationJSON(
        appPath: String,
        auditPath: String,
        status: String = "pass",
        submissionID: String? = defaultSubmissionID,
        submissionStatus: String = "Accepted",
        authMode: String? = "keychain_profile",
        notaryLogPath: String? = nil,
        includeNotaryLog: Bool = true,
        auditStatus: String = "pass",
        auditReadiness: String = "distribution_ready",
        auditPassReady: Bool = true
    ) throws -> String {
        let resolvedNotaryLogPath = notaryLogPath ?? defaultNotaryLogPath(auditPath: auditPath)
        var submission: [String: Any] = [
            "status": submissionStatus,
        ]
        if let submissionID {
            submission["id"] = submissionID
        }
        var root: [String: Any] = [
            "schema": "supermover.macos.notarization.v1",
            "status": status,
            "app_path": appPath,
            "submission": submission,
            "audit": [
                "path": auditPath,
                "status": auditStatus,
                "readiness": auditReadiness,
                "pass_ready": auditPassReady,
                "blocking_checks": 0,
            ],
        ]
        if let authMode {
            root["auth_mode"] = authMode
        }
        if includeNotaryLog, let resolvedNotaryLogPath {
            try writeDefaultNotaryLogIfPossible(path: resolvedNotaryLogPath, submissionID: submissionID)
            root["notary_log"] = [
                "path": resolvedNotaryLogPath,
            ]
        }
        return try jsonString(root)
    }

    private static func defaultNotaryLogPath(auditPath: String) -> String? {
        let trimmed = auditPath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            return nil
        }
        if trimmed.hasPrefix("/") {
            return URL(fileURLWithPath: trimmed)
                .deletingLastPathComponent()
                .appendingPathComponent("notary-log.json")
                .path
        }
        return "notary-log.json"
    }

    static func notaryLogJSON(submissionID: String = defaultSubmissionID) -> String {
        #"{"jobId":"\#(submissionID)","status":"Accepted","issues":[]}"#
    }

    private static func writeDefaultNotaryLogIfPossible(path: String, submissionID: String?) throws {
        guard path.hasPrefix("/"), let submissionID else {
            return
        }
        let url = URL(fileURLWithPath: path)
        var isDirectory: ObjCBool = false
        guard FileManager.default.fileExists(
            atPath: url.deletingLastPathComponent().path,
            isDirectory: &isDirectory
        ), isDirectory.boolValue else {
            return
        }
        guard !FileManager.default.fileExists(atPath: url.path) else {
            return
        }
        try notaryLogJSON(submissionID: submissionID).write(
            to: url,
            atomically: true,
            encoding: .utf8
        )
    }

    static func writeBundleNotaryLog(
        bundleRoot: URL,
        machine: String,
        submissionID: String = defaultSubmissionID
    ) throws {
        try notaryLogJSON(submissionID: submissionID).write(
            to: bundleRoot.appendingPathComponent("\(machine).notary-log.json"),
            atomically: true,
            encoding: .utf8
        )
    }

    @discardableResult
    static func writeCurrentBundleReleaseEvidence(
        bundleRoot: URL,
        machine: String,
        appPath: String,
        cliVersion: String = "supermover test-build"
    ) throws -> [String: Any] {
        let provenanceManifest = developerIDProvenanceManifest(cliVersion: cliVersion)
        try jsonString(provenanceManifest).write(
            to: bundleRoot.appendingPathComponent("\(machine).provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try appAuditJSON(
            appPath: appPath,
            provenanceManifest: provenanceManifest
        ).write(
            to: bundleRoot.appendingPathComponent("\(machine).app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        try notarizationJSON(
            appPath: appPath,
            auditPath: canonicalPostStapleAuditPath(appPath: appPath),
            notaryLogPath: "\(machine).notary-log.json"
        ).write(
            to: bundleRoot.appendingPathComponent("\(machine).notarization.json"),
            atomically: true,
            encoding: .utf8
        )
        try writeBundleNotaryLog(bundleRoot: bundleRoot, machine: machine)
        return provenanceManifest
    }

    static func makeReleaseReadyPackagedApp(
        named name: String,
        cliVersion: String = "supermover 0.1.0-dev",
        provenanceManifest: [String: Any]? = nil,
        includeNotarizationSidecar: Bool = false
    ) throws -> AcceptancePackagedAppFixture {
        let appURL = FileManager.default.temporaryDirectory.appendingPathComponent(
            "\(name)-\(UUID().uuidString).app",
            isDirectory: true
        )
        let resourcesURL = appURL.appendingPathComponent("Contents/Resources", isDirectory: true)
        let binURL = resourcesURL.appendingPathComponent("bin", isDirectory: true)
        try FileManager.default.createDirectory(at: binURL, withIntermediateDirectories: true)

        let manifest = provenanceManifest ?? developerIDProvenanceManifest(cliVersion: cliVersion)
        try jsonString(manifest).write(
            to: resourcesURL.appendingPathComponent("supermover-provenance.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        #!/bin/sh
        printf '\(cliVersion)\\n'
        """.write(
            to: binURL.appendingPathComponent("supermover"),
            atomically: true,
            encoding: .utf8
        )
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: binURL.appendingPathComponent("supermover").path
        )

        try readyAuditScript(
            appPath: appURL.path,
            provenanceManifest: manifest
        ).write(
            to: binURL.appendingPathComponent("supermover-app-audit"),
            atomically: true,
            encoding: .utf8
        )
        try FileManager.default.setAttributes(
            [.posixPermissions: 0o755],
            ofItemAtPath: binURL.appendingPathComponent("supermover-app-audit").path
        )

        let fixture = AcceptancePackagedAppFixture(
            appURL: appURL,
            resourcesURL: resourcesURL,
            binURL: binURL,
            provenanceManifest: manifest
        )
        if includeNotarizationSidecar {
            try FileManager.default.createDirectory(
                at: fixture.canonicalNotaryDirectoryURL,
                withIntermediateDirectories: true
            )
            try appAuditJSON(
                appPath: appURL.path,
                provenanceManifest: manifest
            ).write(
                to: fixture.canonicalPostStapleAuditURL,
                atomically: true,
                encoding: .utf8
            )
            try notaryLogJSON().write(
                to: canonicalNotaryLogURL(appBundleURL: appURL),
                atomically: true,
                encoding: .utf8
            )
            try notarizationJSON(
                appPath: appURL.path,
                auditPath: fixture.canonicalPostStapleAuditURL.path,
                notaryLogPath: canonicalNotaryLogURL(appBundleURL: appURL).path
            ).write(
                to: fixture.canonicalNotarizationSidecarURL,
                atomically: true,
                encoding: .utf8
            )
        }
        return fixture
    }

    static func bundledProvenanceManifest(appBundleURL: URL) throws -> [String: Any] {
        let provenanceURL = appBundleURL
            .appendingPathComponent("Contents/Resources", isDirectory: true)
            .appendingPathComponent("supermover-provenance.json")
        let data = try Data(contentsOf: provenanceURL)
        guard let object = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw NSError(
                domain: "AcceptanceReleaseEvidenceFixtures",
                code: 2,
                userInfo: [NSLocalizedDescriptionKey: "malformed bundled provenance manifest at \(provenanceURL.path)"]
            )
        }
        return object
    }

    @discardableResult
    static func writeAppAuditResult(
        appBundleURL: URL,
        outputURL: URL,
        status: String = "pass",
        readiness: String = "distribution_ready",
        passReady: Bool = true,
        blockingChecks: Int = 0,
        reviewChecks: Int = 0
    ) throws -> AcceptancePackagingEvidenceCollector.AppAuditResult {
        let provenanceManifest = try bundledProvenanceManifest(appBundleURL: appBundleURL)
        try appAuditJSON(
            appPath: appBundleURL.path,
            provenanceManifest: provenanceManifest,
            status: status,
            readiness: readiness,
            passReady: passReady,
            blockingChecks: blockingChecks,
            reviewChecks: reviewChecks
        ).write(
            to: outputURL,
            atomically: true,
            encoding: .utf8
        )
        return .init(
            exitCode: status == "pass" ? 0 : 1,
            status: status,
            readiness: readiness,
            passReady: passReady,
            blockingChecks: blockingChecks
        )
    }

    static func readyAuditScript(
        appPath: String,
        provenanceManifest: [String: Any]
    ) throws -> String {
        let auditJSON = try appAuditJSON(
            appPath: appPath,
            provenanceManifest: provenanceManifest
        )
        return """
        #!/bin/sh
        set -eu
        cat <<'EOF'
        \(auditJSON)
        EOF
        """
    }

    static func jsonString(_ object: Any) throws -> String {
        let data = try JSONSerialization.data(
            withJSONObject: object,
            options: [.prettyPrinted, .sortedKeys]
        )
        guard var string = String(data: data, encoding: .utf8) else {
            throw NSError(domain: "AcceptanceReleaseEvidenceFixtures", code: 1)
        }
        if !string.hasSuffix("\n") {
            string.append("\n")
        }
        return string
    }
}

enum AcceptanceWorkflowFixtures {
    static func writeReadyBundleArtifacts(bundleRoot: URL, targetRoot: URL) throws {
        try FileManager.default.createDirectory(
            at: bundleRoot.appendingPathComponent("exported-receipts", isDirectory: true),
            withIntermediateDirectories: true
        )

        try """
        {
          "source": "browse",
          "listen": "0.0.0.0:39394",
          "candidate_count": 1,
          "invalid_packets": 0,
          "trusted": false,
          "candidates": [
            {
              "hint": {
                "address": "127.0.0.1:39395",
                "advertisement": {
                  "service_type": "_supermover._udp",
                  "protocol_version": "1",
                  "ephemeral_nonce": "nonce-1",
                  "capability_flags": ["pairing"]
                },
                "seen_at": "2026-01-01T00:00:00Z",
                "expires_at": "2026-01-01T00:00:30Z",
                "trusted": false
              },
              "class": "candidate",
              "duplicate_count": 1,
              "ambiguity_reasons": []
            }
          ]
        }
        """.write(
            to: bundleRoot.appendingPathComponent("source.browse.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "status": "advertised",
          "listen": "0.0.0.0:39394",
          "destination": "255.255.255.255:39394",
          "service_type": "_supermover._udp",
          "protocol_version": "1",
          "ephemeral_nonce": "nonce-1",
          "capability_flags": ["pairing"],
          "trusted": false,
          "duration": "5s",
          "interval": "1s"
        }
        """.write(
            to: bundleRoot.appendingPathComponent("target.advertise.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "address": "127.0.0.1:39395",
          "verification_code": "123456",
          "mode": "pairing",
          "receiver_address": "127.0.0.1:9443",
          "receiver_routes": true,
          "push_network": true,
          "trusted": false,
          "transfer": true
        }
        """.write(
            to: bundleRoot.appendingPathComponent("target.ready.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "profile": "/tmp/source.profile.json",
          "target_address": "127.0.0.1:39395",
          "verification_code": "123456",
          "pairing_receipt_id": "pair-1",
          "receipt_path": "exported-receipts/pair-1.json"
        }
        """.write(
            to: bundleRoot.appendingPathComponent("source.pair.json"),
            atomically: true,
            encoding: .utf8
        )
        try pairingReceiptJSON().write(
            to: bundleRoot.appendingPathComponent("exported-receipts/pair-1.json"),
            atomically: true,
            encoding: .utf8
        )
        try "pair ok\n".write(
            to: bundleRoot.appendingPathComponent("source.pair.txt"),
            atomically: true,
            encoding: .utf8
        )
        try "receipt adopted\n".write(
            to: bundleRoot.appendingPathComponent("target.adopt-pairing.txt"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "profile": "/tmp/source.profile.json",
          "session_id": "session-1",
          "target_address": "127.0.0.1:39395",
          "receiver_address": "127.0.0.1:9443",
          "target_mode": "pairing"
        }
        """.write(
            to: bundleRoot.appendingPathComponent("source.transfer.json"),
            atomically: true,
            encoding: .utf8
        )
        try "push ok\n".write(
            to: bundleRoot.appendingPathComponent("source.network-push.txt"),
            atomically: true,
            encoding: .utf8
        )
        try sourceBaselineJSON().write(
            to: bundleRoot.appendingPathComponent("source.baseline.json"),
            atomically: true,
            encoding: .utf8
        )
        try currentSourceConsistencyJSON().write(
            to: bundleRoot.appendingPathComponent("source.consistency.json"),
            atomically: true,
            encoding: .utf8
        )
        try verifyJSON(targetRoot: targetRoot.path).write(
            to: bundleRoot.appendingPathComponent("source.verify.json"),
            atomically: true,
            encoding: .utf8
        )
        try reportJSON(targetRoot: targetRoot.path).write(
            to: bundleRoot.appendingPathComponent("source.report.json"),
            atomically: true,
            encoding: .utf8
        )
        try statusJSON(targetRoot: targetRoot.path).write(
            to: bundleRoot.appendingPathComponent("source.status.json"),
            atomically: true,
            encoding: .utf8
        )
        try healthJSON(targetRoot: targetRoot.path).write(
            to: bundleRoot.appendingPathComponent("source.health.json"),
            atomically: true,
            encoding: .utf8
        )
    }

    static func pairingReceiptJSON(
        id: String = "pair-1",
        profileID: String? = "profile-src",
        targetID: String? = "target-1",
        sourceDeviceID: String? = "src-spki",
        targetDeviceID: String? = "dst-spki",
        devicePublicKey: String? = "dst-spki",
        method: String? = "sas",
        verifiedAt: String? = "2026-06-04T00:00:00Z",
        verificationHash: String? = "hash-1",
        verificationPhrase: String? = nil,
        protocolVersion: String? = "supermover/v1",
        version: Int? = 1
    ) throws -> String {
        var receipt: [String: Any] = [:]
        if let version {
            receipt["version"] = version
        }
        receipt["id"] = id
        if let profileID {
            receipt["profile_id"] = profileID
        }
        if let targetID {
            receipt["target_id"] = targetID
        }
        if let sourceDeviceID {
            receipt["source_device_id"] = sourceDeviceID
        }
        if let targetDeviceID {
            receipt["target_device_id"] = targetDeviceID
        }
        if let devicePublicKey {
            receipt["device_public_key"] = devicePublicKey
        }
        if let method {
            receipt["method"] = method
        }
        if let verifiedAt {
            receipt["verified_at"] = verifiedAt
        }
        if let verificationHash {
            receipt["verification_hash"] = verificationHash
        }
        if let verificationPhrase {
            receipt["verification_phrase"] = verificationPhrase
        }
        if let protocolVersion {
            receipt["protocol_version"] = protocolVersion
        }
        return try AcceptanceReleaseEvidenceFixtures.jsonString(receipt)
    }

    static func targetNetworkTransferJSON(
        version: Int? = 1,
        sessionID: String? = "session-1",
        profileID: String? = "profile-1",
        targetID: String? = "target-1",
        sourceDeviceID: Any? = "src-spki",
        targetDeviceID: Any? = "dst-spki",
        protocolVersion: String? = "supermover/1",
        status: String? = "published",
        stage: String? = "commit",
        encryptedTransfer: String? = "tls13_mtls",
        startedAt: String? = "2026-01-01T00:00:00Z",
        updatedAt: String? = "2026-01-01T00:00:01Z"
    ) throws -> String {
        var transfer: [String: Any] = [:]
        if let version {
            transfer["version"] = version
        }
        if let sessionID {
            transfer["session_id"] = sessionID
        }
        if let profileID {
            transfer["profile_id"] = profileID
        }
        if let targetID {
            transfer["target_id"] = targetID
        }
        if let sourceDeviceID {
            transfer["source_device_id"] = sourceDeviceID
        }
        if let targetDeviceID {
            transfer["target_device_id"] = targetDeviceID
        }
        if let protocolVersion {
            transfer["protocol_version"] = protocolVersion
        }
        if let status {
            transfer["status"] = status
        }
        if let stage {
            transfer["stage"] = stage
        }
        if let encryptedTransfer {
            transfer["encrypted_transfer"] = encryptedTransfer
        }
        if let startedAt {
            transfer["started_at"] = startedAt
        }
        if let updatedAt {
            transfer["updated_at"] = updatedAt
        }
        return try AcceptanceReleaseEvidenceFixtures.jsonString(transfer)
    }

    static func currentSourceConsistencyJSON() -> String {
        """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "status": "pass",
          "mode": "current_source_verified",
          "session_id": "session-1",
          "detail": "test fixture"
        }
        """
    }

    static func sourceBaselineJSON() -> String {
        """
        {
          "schema": "supermover.acceptance.current_source_consistency.v1",
          "profile_id": "profile-1",
          "root_id": "root-1",
          "root_path": "/tmp/source",
          "session_id": "session-1",
          "created_at": "2026-01-01T00:00:00Z",
          "entries": []
        }
        """
    }

    static func verifyJSON(targetRoot: String) -> String {
        """
        {
          "target_root": "\(targetRoot)",
          "session_id": "session-1",
          "manifest": {"id":"m1","session_id":"session-1","root_id":"root-1","created_at":"2026-01-01T00:00:00Z","entries":1,"files":1},
          "summary": {
            "manifest_count": 1,
            "manifest_entries": 1,
            "files_expected": 1,
            "files_verified": 1,
            "warnings": 0,
            "soft_deletes": 0,
            "target_drifts": 0,
            "artifact_problems": 0,
            "error_findings": 0,
            "warning_findings": 0,
            "skipped_digest": 0
          }
        }
        """
    }

    static func reportJSON(targetRoot: String) -> String {
        """
        {
          "target_root": "\(targetRoot)",
          "overall": {"status":"ok","issues":[]},
          "summary": {"warnings":0,"soft_deletes":0,"target_drifts":0,"live_target_drifts":0,"prune_candidates":0,"prune_refusals":0,"prune_approvals":0,"network_transfers":1,"artifact_problems":0},
          "latest_session": {"id":"session-1","manifest_id":"m1","created_at":"2026-01-01T00:00:00Z","entries":1,"files":1,"completeness":{"status":"verified","files_expected":1,"files_verified":1,"verification_errors":0,"verification_warnings":0}},
          "prune_review": {"status":"clear","approval_required":false,"apply":"none","summary":{"candidates":0,"refusals":0,"approvals":0,"unapplied_approvals":0,"receipt_issues":0}},
          "pairing": {"status":"paired_receipt_valid","receipt_id":"pair-1","target_device_id":"dst-spki","paired_at":"2026-01-01T00:00:00Z","method":"verification_code","verified_at":"2026-01-01T00:00:00Z","evidence":"receipt","receipt_source":"target_control","receipt_path":"p","source_receipt_path":"s","target_receipt_path":"t","encrypted_transfer":"required"},
          "privacy": {"status":"review","claim":"bounded","network_transfer":"published"},
          "health": {"healthy":true,"summary":{"incomplete_sessions":0,"invalid_records":0,"artifact_problems":0,"target_drifts":0,"network_transfers":1}}
        }
        """
    }

    static func statusJSON(targetRoot: String) -> String {
        """
        {
          "profile_id": "profile-1",
          "target_id": "target-1",
          "target_root": "\(targetRoot)",
          "overall": {"status":"ok","target_status":"ok"},
          "issues": [],
          "latest_session": {"id":"session-1","manifest_id":"m1","created_at":"2026-01-01T00:00:00Z","entries":1,"completeness_status":"verified","files_expected":1,"files_verified":1,"verification_errors":0,"verification_warnings":0},
          "counts": {"warnings":0,"soft_deletes":0,"target_drifts":0,"live_target_drifts":0,"live_target_drift_artifact_problems":0,"prune_unapplied_approvals":0,"prune_active_approvals":0,"prune_stale_approvals":0,"prune_expired_approvals":0,"prune_consumed_approvals":0,"prune_receipt_issues":0,"recovery_issues":0,"artifact_problems":0,"network_transfers":1},
          "prune_review": {"status":"clear","action":"none"},
          "pairing": {"status":"paired_receipt_valid","receipt_id":"pair-1","target_device_id":"dst-spki","paired_at":"2026-01-01T00:00:00Z","method":"verification_code","verified_at":"2026-01-01T00:00:00Z","evidence":"receipt","receipt_source":"target_control","receipt_path":"p","source_receipt_path":"s","target_receipt_path":"t","encrypted_transfer":"required"},
          "privacy": {"status":"review","mode":"bounded","traffic_level":2,"claim":"bounded","local_push":"disabled","network_transfer":"published","residual_leakage":[],"configured_reductions":[],"overhead_status":"published","overhead_source":"network-transfer"},
          "traffic_privacy_acceptance": {"status":"review","claim":"bounded","blockers":[]},
          "network": {"status":"published","artifact_problems":0,"transfers":[{"session_id":"session-1","status":"published","stage":"commit","action":"preserved"}]},
          "artifact_problem_sources": []
        }
        """
    }

    static func healthJSON(targetRoot: String) -> String {
        """
        {
          "target_root": "\(targetRoot)",
          "healthy": true,
          "summary": {"incomplete_sessions":0,"invalid_records":0,"artifact_problems":0,"target_drifts":0,"network_transfers":1},
          "items": [],
          "invalid": [],
          "artifacts": [],
          "network_transfers": [{"session_id":"session-1","status":"published","stage":"commit","action":"preserved"}]
        }
        """
    }
}

enum AcceptanceInstalledAppBundleFixtures {
    static let defaultVerifiedBundleHandoffsJSON = """
        [
          {
            "archive": "bundle.tgz",
            "manifest": "bundle.manifest.json",
            "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
            "meta": "meta.json",
            "verified": true,
            "exporting_machine_id": "source-machine",
            "exporting_machine_label": "source",
            "importing_machine_id": "target-machine",
            "importing_machine_label": "target"
          }
        ]
    """

    static func writeInstalledAppMachineFacts(
        bundleRoot: URL,
        sourceMachineID: String = "source-machine",
        sourceMachineLabel: String = "source",
        targetMachineID: String = "target-machine",
        targetMachineLabel: String = "target"
    ) throws {
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "\(sourceMachineID)",
          "machine_label": "\(sourceMachineLabel)"
        }
        """.write(
            to: bundleRoot.appendingPathComponent("source.machine.json"),
            atomically: true,
            encoding: .utf8
        )
        try """
        {
          "schema": "supermover.acceptance.machine_facts.v1",
          "machine_id": "\(targetMachineID)",
          "machine_label": "\(targetMachineLabel)"
        }
        """.write(
            to: bundleRoot.appendingPathComponent("target.machine.json"),
            atomically: true,
            encoding: .utf8
        )
    }

    static func bundleMeta(
        includeReleaseEvidenceMeta: Bool,
        includeWorkflowSummaryArtifact: Bool,
        includeEvaluateArtifactPaths: Bool = false,
        status: String = "in_progress",
        sourceMachineID: String = "source-machine",
        sourceMachineLabel: String = "source",
        targetMachineID: String = "target-machine",
        targetMachineLabel: String = "target",
        bundleHandoffsJSON: String = defaultVerifiedBundleHandoffsJSON
    ) -> String {
        let releaseEvidence = includeReleaseEvidenceMeta ? """
            ,
            "app_audit": {
              "source": {
                "collected_by": "test",
                "output": "source.app-audit.json",
                "exit_code": 0,
                "status": "pass",
                "readiness": "distribution_ready",
                "pass_ready": true,
                "blocking_checks": 0
              },
              "target": {
                "collected_by": "test",
                "output": "target.app-audit.json",
                "exit_code": 0,
                "status": "pass",
                "readiness": "distribution_ready",
                "pass_ready": true,
                "blocking_checks": 0
              }
            },
            "notarization": {
              "source": {
                "collected_by": "test",
                "output": "source.notarization.json",
                "status": "pass",
                "audit_status": "pass",
                "audit_readiness": "distribution_ready",
                "audit_pass_ready": true
              },
              "target": {
                "collected_by": "test",
                "output": "target.notarization.json",
                "status": "pass",
                "audit_status": "pass",
                "audit_readiness": "distribution_ready",
                "audit_pass_ready": true
              }
            }
        """ : ""
        let workflowSummary = includeWorkflowSummaryArtifact ? """
            ,
            "workflow_summary": {
              "output": "workflow.summary.json"
            }
        """ : ""
        let sourcePairArtifacts = includeEvaluateArtifactPaths ? """
            ,
              "pair": "source.pair.txt"
        """ : ""
        let sourceTransferArtifacts = includeEvaluateArtifactPaths ? """
            ,
              "push": "source.network-push.txt",
              "verify": "source.verify.json",
              "status": "source.status.json",
              "report": "source.report.json",
              "health": "source.health.json"
        """ : ""
        return """
        {
          "schema": "supermover.acceptance.two_machine.v1",
          "status": "\(status)",
          "collection": {
            "mode": "two_machine",
            "machine_count": 2
          },
          "roles": {
            "source_pair": {
              "profile": "/tmp/source.profile.json",
              "status": "recorded",
              "machine_id": "\(sourceMachineID)",
              "machine_label": "\(sourceMachineLabel)"
            },
            "target": {
              "profile": "/tmp/target.profile.json",
              "status": "recorded",
              "machine_id": "\(targetMachineID)",
              "machine_label": "\(targetMachineLabel)"
            }
          },
          "evidence": {
            "machine_facts": {
              "source": {
                "output": "source.machine.json",
                "machine_id": "\(sourceMachineID)",
                "machine_label": "\(sourceMachineLabel)"
              },
              "target": {
                "output": "target.machine.json",
                "machine_id": "\(targetMachineID)",
                "machine_label": "\(targetMachineLabel)"
              }
            }\(workflowSummary)\(releaseEvidence),
            "target_ready": {
              "address": "127.0.0.1:39395",
              "verification_code": "123456",
              "mode": "pairing"
            },
            "discovery": {
              "source_browse": {
                "output": "source.browse.json",
                "trusted": false
              },
              "target_advertise": {
                "output": "target.advertise.json",
                "trusted": false
              }
            },
            "source_pair": {
              "output": "source.pair.json",
              "pairing_receipt_id": "pair-1",
              "receipt_path": "exported-receipts/pair-1.json",
              "target_address": "127.0.0.1:39395"\(sourcePairArtifacts)
            },
            "target_import": {
              "pairing_receipt_id": "pair-1",
              "adopted": "target.adopt-pairing.txt"
            },
            "source_transfer": {
              "output": "source.transfer.json",
              "session_id": "session-1",
              "receiver_address": "127.0.0.1:9443"\(sourceTransferArtifacts)
            },
            "source_consistency": {
              "output": "source.consistency.json",
              "baseline": "source.baseline.json",
              "status": "pass",
              "mode": "current_source_verified"
            },
            "bundle_handoffs": \(bundleHandoffsJSON),
            "operator": {
              "local_network": {
                "status": "pass",
                "detail": "accepted",
                "machine_id": "\(targetMachineID)",
                "machine_label": "\(targetMachineLabel)"
              },
              "firewall": {
                "status": "pass",
                "detail": "allowed",
                "machine_id": "\(targetMachineID)",
                "machine_label": "\(targetMachineLabel)"
              },
              "pairing_confirmation": {
                "status": "pass",
                "detail": "matched",
                "machine_id": "\(sourceMachineID)",
                "machine_label": "\(sourceMachineLabel)"
              }
            }
          }
        }
        """
    }
}
