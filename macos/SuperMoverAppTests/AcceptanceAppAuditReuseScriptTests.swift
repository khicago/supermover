import XCTest
@testable import SuperMoverApp

final class AcceptanceAppAuditReuseScriptTests: XCTestCase {
    func testAcceptanceRecordAppAuditReusesCurrentMachineScopedEvidenceInsteadOfRewritingIt() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-app-audit-reuse")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        let app = try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
            named: "acceptance-app-audit-reuse"
        )
        defer { try? FileManager.default.removeItem(at: app.appURL) }

        let currentAudit = try makeAppAuditJSON(
            appPath: app.appURL.path,
            provenanceManifest: app.provenanceManifest,
            checkedAt: "2026-06-03T01:00:40Z"
        )
        try currentAudit.write(
            to: bundleRoot.appendingPathComponent("target.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        try writeBundleMeta(bundleRoot: bundleRoot, collectedBy: "target-serve")

        let counterURL = workDir.appendingPathComponent("audit-counter.txt")
        let replacementAudit = try makeAppAuditJSON(
            appPath: app.appURL.path,
            provenanceManifest: app.provenanceManifest,
            checkedAt: "2026-06-03T01:00:49Z"
        )
        let auditScriptURL = try writeAuditScript(workDir: workDir, auditJSON: replacementAudit)

        let result = try runAcceptanceRecordAppAudit(
            auditScriptURL: auditScriptURL,
            appURL: app.appURL,
            bundleRoot: bundleRoot,
            collectedBy: "target-import",
            environment: ["AUDIT_COUNTER_PATH": counterURL.path]
        )

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        XCTAssertEqual(
            try String(contentsOf: bundleRoot.appendingPathComponent("target.app-audit.json")),
            currentAudit
        )
        XCTAssertFalse(FileManager.default.fileExists(atPath: counterURL.path))
        XCTAssertEqual(try bundleCollectedBy(bundleRoot: bundleRoot), "target-serve")
    }

    func testAcceptanceRecordAppAuditRecollectsWhenExistingEvidenceHasStaleProvenance() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "acceptance-app-audit-stale")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("bundle", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot, withIntermediateDirectories: true)
        let app = try AcceptanceReleaseEvidenceFixtures.makeReleaseReadyPackagedApp(
            named: "acceptance-app-audit-stale"
        )
        defer { try? FileManager.default.removeItem(at: app.appURL) }

        var staleManifest = app.provenanceManifest
        staleManifest["git_commit"] = "stale00000000"
        let staleAudit = try makeAppAuditJSON(
            appPath: app.appURL.path,
            provenanceManifest: staleManifest,
            checkedAt: "2026-06-03T01:00:40Z"
        )
        try staleAudit.write(
            to: bundleRoot.appendingPathComponent("target.app-audit.json"),
            atomically: true,
            encoding: .utf8
        )
        try writeBundleMeta(bundleRoot: bundleRoot, collectedBy: "target-serve")

        let counterURL = workDir.appendingPathComponent("audit-counter.txt")
        let currentAudit = try makeAppAuditJSON(
            appPath: app.appURL.path,
            provenanceManifest: app.provenanceManifest,
            checkedAt: "2026-06-03T01:00:49Z"
        )
        let auditScriptURL = try writeAuditScript(workDir: workDir, auditJSON: currentAudit)

        let result = try runAcceptanceRecordAppAudit(
            auditScriptURL: auditScriptURL,
            appURL: app.appURL,
            bundleRoot: bundleRoot,
            collectedBy: "target-import",
            environment: ["AUDIT_COUNTER_PATH": counterURL.path]
        )

        XCTAssertEqual(result.exitCode, 0, "stderr:\n\(result.stderr)")
        let finalAudit = try bundleAuditDocument(bundleRoot: bundleRoot)
        let provenance = try XCTUnwrap(finalAudit["provenance"] as? [String: Any])
        let manifest = try XCTUnwrap(provenance["manifest"] as? [String: Any])
        XCTAssertEqual(finalAudit["app_path"] as? String, app.appURL.path)
        XCTAssertEqual(finalAudit["checked_at"] as? String, "2026-06-03T01:00:49Z")
        XCTAssertEqual(manifest["git_commit"] as? String, app.provenanceManifest["git_commit"] as? String)
        XCTAssertEqual(try String(contentsOf: counterURL), "1")
        XCTAssertEqual(try bundleCollectedBy(bundleRoot: bundleRoot), "target-import")
    }

    private func runAcceptanceRecordAppAudit(
        auditScriptURL: URL,
        appURL: URL,
        bundleRoot: URL,
        collectedBy: String,
        environment: [String: String]
    ) throws -> AcceptanceProcessResult {
        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        return try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: [
                "-c",
                ". \"$1\"; acceptance_record_app_audit \"$2\" \"$3\" \"$4\" target \"$5\"",
                "acceptance-common-record-app-audit",
                repoRoot.appendingPathComponent("macos/script/lib/acceptance-common.sh").path,
                auditScriptURL.path,
                appURL.path,
                bundleRoot.path,
                collectedBy,
            ],
            environment: environment,
            currentDirectoryURL: repoRoot
        )
    }

    private func writeBundleMeta(bundleRoot: URL, collectedBy: String) throws {
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "schema": "supermover.acceptance.two_machine.v1",
            "status": "in_progress",
            "collection": [
                "mode": "same_machine",
                "machine_count": 1,
            ],
            "roles": [:],
            "evidence": [
                "app_audit": [
                    "target": [
                        "collected_by": collectedBy,
                        "output": "target.app-audit.json",
                        "exit_code": 0,
                        "status": "pass",
                        "readiness": "distribution_ready",
                        "pass_ready": true,
                        "blocking_checks": 0,
                    ],
                ],
            ],
        ]).write(
            to: bundleRoot.appendingPathComponent("meta.json"),
            atomically: true,
            encoding: .utf8
        )
    }

    private func bundleCollectedBy(bundleRoot: URL) throws -> String? {
        let data = try Data(contentsOf: bundleRoot.appendingPathComponent("meta.json"))
        let document = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        let evidence = try XCTUnwrap(document["evidence"] as? [String: Any])
        let appAudit = try XCTUnwrap(evidence["app_audit"] as? [String: Any])
        let target = try XCTUnwrap(appAudit["target"] as? [String: Any])
        return target["collected_by"] as? String
    }

    private func bundleAuditDocument(bundleRoot: URL) throws -> [String: Any] {
        let data = try Data(contentsOf: bundleRoot.appendingPathComponent("target.app-audit.json"))
        return try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
    }

    private func writeAuditScript(workDir: URL, auditJSON: String) throws -> URL {
        let scriptURL = workDir.appendingPathComponent("supermover-app-audit")
        try """
        #!/bin/sh
        set -eu
        counter_path="${AUDIT_COUNTER_PATH:-}"
        if [ -n "$counter_path" ]; then
          count=0
          if [ -f "$counter_path" ]; then
            count=$(cat "$counter_path")
          fi
          count=$((count + 1))
          printf '%s' "$count" > "$counter_path"
        fi
        cat <<'EOF'
        \(auditJSON)
        EOF
        """.write(to: scriptURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: scriptURL.path)
        return scriptURL
    }

    private func makeAppAuditJSON(
        appPath: String,
        provenanceManifest: [String: Any],
        checkedAt: String
    ) throws -> String {
        try AcceptanceReleaseEvidenceFixtures.jsonString([
            "schema": "supermover.macos.app_audit.v1",
            "checked_at": checkedAt,
            "status": "pass",
            "readiness": "distribution_ready",
            "app_path": appPath,
            "summary": [
                "pass_ready": true,
                "blocking_checks": 0,
                "review_checks": 0,
            ],
            "provenance": [
                "path": "\(appPath)/Contents/Resources/supermover-provenance.json",
                "manifest": provenanceManifest,
            ],
        ])
    }
}
