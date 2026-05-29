import XCTest

final class NotarizeAppScriptTests: XCTestCase {
    func testNotarizeScriptFailsClosedWithoutCredentials() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }

        let result = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [:],
            allowNonZeroExit: true
        )

        XCTAssertEqual(result.exitCode, 1)
        let json = try parseJSON(result.stdout)
        XCTAssertEqual(json["status"] as? String, "blocked")
        XCTAssertEqual((json["failure"] as? [String: Any])?["id"] as? String, "missing_credentials")
    }

    func testNotarizeScriptClearsStaleSidecarWhenJQIsMissing() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }

        let sidecarURL = app.appURL.deletingLastPathComponent().appendingPathComponent("\(app.appURL.lastPathComponent).notary/notarization.json")
        try FileManager.default.createDirectory(at: sidecarURL.deletingLastPathComponent(), withIntermediateDirectories: true)
        try """
        {
          "schema": "supermover.macos.notarization.v1",
          "status": "pass",
          "submission": {"status": "Accepted"},
          "audit": {"status": "pass", "readiness": "distribution_ready", "pass_ready": true}
        }
        """.write(to: sidecarURL, atomically: true, encoding: .utf8)

        let result = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [
                "PATH": "/usr/bin:/bin",
            ],
            allowNonZeroExit: true
        )

        XCTAssertEqual(result.exitCode, 1)
        let json = try parseJSON(result.stdout)
        XCTAssertEqual(json["status"] as? String, "blocked")
        XCTAssertEqual((json["failure"] as? [String: Any])?["id"] as? String, "missing_tool_jq")

        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarURL.path))
        let sidecarJSON = try parseJSON(String(contentsOf: sidecarURL))
        XCTAssertEqual(sidecarJSON["status"] as? String, "blocked")
        XCTAssertEqual((sidecarJSON["failure"] as? [String: Any])?["id"] as? String, "missing_tool_jq")
    }

    func testNotarizeScriptProducesDurableEvidenceOnSuccessfulFakeNotaryFlow() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let harness = try makeNotaryHarness(auditStatus: "pass", auditReadiness: "distribution_ready", auditPassReady: true, auditBlockingChecks: 0)
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }

        let result = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [
                "PATH": "\(harness.binURL.path):\(ProcessInfo.processInfo.environment["PATH"] ?? "")",
                "SUPERMOVER_NOTARY_KEYCHAIN_PROFILE": "supermover-ci",
                "SUPERMOVER_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
                "SUPERMOVER_TEST_NOTARY_ARGS_DIR": harness.argsURL.path,
                "SUPERMOVER_TEST_NOTARY_SUBMIT_JSON": harness.submitJSONURL.path,
                "SUPERMOVER_TEST_NOTARY_LOG_JSON": harness.logJSONURL.path,
            ],
            allowNonZeroExit: false
        )

        XCTAssertEqual(result.exitCode, 0)
        let json = try parseJSON(result.stdout)
        XCTAssertEqual(json["status"] as? String, "pass")
        XCTAssertEqual(json["auth_mode"] as? String, "keychain_profile")
        XCTAssertEqual((json["submission"] as? [String: Any])?["id"] as? String, "11111111-1111-1111-1111-111111111111")
        XCTAssertEqual((json["submission"] as? [String: Any])?["status"] as? String, "Accepted")
        XCTAssertEqual((json["audit"] as? [String: Any])?["status"] as? String, "pass")
        XCTAssertEqual((json["audit"] as? [String: Any])?["readiness"] as? String, "distribution_ready")

        let archivePath = try XCTUnwrap(json["archive_path"] as? String)
        let logPath = try XCTUnwrap((json["notary_log"] as? [String: Any])?["path"] as? String)
        XCTAssertTrue(FileManager.default.fileExists(atPath: archivePath))
        XCTAssertTrue(FileManager.default.fileExists(atPath: logPath))
        XCTAssertTrue(FileManager.default.fileExists(atPath: workDir.appendingPathComponent("post-staple.audit.json").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: harness.argsURL.appendingPathComponent("submit.args").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: harness.argsURL.appendingPathComponent("stapler-validate.args").path))
    }

    func testNotarizeScriptFailsClosedWhenSubmissionIDIsNotUUID() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let harness = try makeNotaryHarness(auditStatus: "pass", auditReadiness: "distribution_ready", auditPassReady: true, auditBlockingChecks: 0)
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }
        try """
        {"id":"manual-pass","status":"Accepted","message":"Ready for distribution"}
        """.write(to: harness.submitJSONURL, atomically: true, encoding: .utf8)

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }

        let result = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [
                "PATH": "\(harness.binURL.path):\(ProcessInfo.processInfo.environment["PATH"] ?? "")",
                "SUPERMOVER_NOTARY_KEYCHAIN_PROFILE": "supermover-ci",
                "SUPERMOVER_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
                "SUPERMOVER_TEST_NOTARY_ARGS_DIR": harness.argsURL.path,
                "SUPERMOVER_TEST_NOTARY_SUBMIT_JSON": harness.submitJSONURL.path,
                "SUPERMOVER_TEST_NOTARY_LOG_JSON": harness.logJSONURL.path,
            ],
            allowNonZeroExit: true
        )

        XCTAssertEqual(result.exitCode, 1)
        let json = try parseJSON(result.stdout)
        XCTAssertEqual(json["status"] as? String, "blocked")
        XCTAssertEqual((json["failure"] as? [String: Any])?["id"] as? String, "malformed_submit_response")
        XCTAssertEqual((json["submission"] as? [String: Any])?["id"] as? String, "manual-pass")
        XCTAssertFalse(FileManager.default.fileExists(atPath: harness.argsURL.appendingPathComponent("log.args").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: harness.argsURL.appendingPathComponent("stapler-staple.args").path))

        let sidecarURL = app.appURL.deletingLastPathComponent().appendingPathComponent("\(app.appURL.lastPathComponent).notary/notarization.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarURL.path))
        let sidecarJSON = try parseJSON(String(contentsOf: sidecarURL))
        XCTAssertEqual(sidecarJSON["status"] as? String, "blocked")
        XCTAssertEqual((sidecarJSON["failure"] as? [String: Any])?["id"] as? String, "malformed_submit_response")
    }

    func testNotarizeScriptFailsClosedWhenNotaryLogIsNotAccepted() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let harness = try makeNotaryHarness(auditStatus: "pass", auditReadiness: "distribution_ready", auditPassReady: true, auditBlockingChecks: 0)
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }
        try """
        {"status":"Invalid","issues":[{"severity":"error","path":"SuperMover.app","message":"rejected"}]}
        """.write(to: harness.logJSONURL, atomically: true, encoding: .utf8)

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }

        let result = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [
                "PATH": "\(harness.binURL.path):\(ProcessInfo.processInfo.environment["PATH"] ?? "")",
                "SUPERMOVER_NOTARY_KEYCHAIN_PROFILE": "supermover-ci",
                "SUPERMOVER_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
                "SUPERMOVER_TEST_NOTARY_ARGS_DIR": harness.argsURL.path,
                "SUPERMOVER_TEST_NOTARY_SUBMIT_JSON": harness.submitJSONURL.path,
                "SUPERMOVER_TEST_NOTARY_LOG_JSON": harness.logJSONURL.path,
            ],
            allowNonZeroExit: true
        )

        XCTAssertEqual(result.exitCode, 1)
        let json = try parseJSON(result.stdout)
        XCTAssertEqual(json["status"] as? String, "blocked")
        XCTAssertEqual((json["failure"] as? [String: Any])?["id"] as? String, "notary_log_not_accepted")
        XCTAssertEqual((json["notary_log"] as? [String: Any])?["path"] as? String, workDir.appendingPathComponent("notary-log.json").path)
        XCTAssertTrue(FileManager.default.fileExists(atPath: harness.argsURL.appendingPathComponent("log.args").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: harness.argsURL.appendingPathComponent("stapler-staple.args").path))

        let sidecarURL = app.appURL.deletingLastPathComponent().appendingPathComponent("\(app.appURL.lastPathComponent).notary/notarization.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarURL.path))
        let sidecarJSON = try parseJSON(String(contentsOf: sidecarURL))
        XCTAssertEqual(sidecarJSON["status"] as? String, "blocked")
        XCTAssertEqual((sidecarJSON["failure"] as? [String: Any])?["id"] as? String, "notary_log_not_accepted")
    }

    func testNotarizeScriptFailsClosedWhenNotaryLogJobIDDoesNotMatchSubmission() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let harness = try makeNotaryHarness(auditStatus: "pass", auditReadiness: "distribution_ready", auditPassReady: true, auditBlockingChecks: 0)
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON(
            submissionID: "22222222-2222-2222-2222-222222222222"
        ).write(to: harness.logJSONURL, atomically: true, encoding: .utf8)

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }

        let result = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [
                "PATH": "\(harness.binURL.path):\(ProcessInfo.processInfo.environment["PATH"] ?? "")",
                "SUPERMOVER_NOTARY_KEYCHAIN_PROFILE": "supermover-ci",
                "SUPERMOVER_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
                "SUPERMOVER_TEST_NOTARY_ARGS_DIR": harness.argsURL.path,
                "SUPERMOVER_TEST_NOTARY_SUBMIT_JSON": harness.submitJSONURL.path,
                "SUPERMOVER_TEST_NOTARY_LOG_JSON": harness.logJSONURL.path,
            ],
            allowNonZeroExit: true
        )

        XCTAssertEqual(result.exitCode, 1)
        let json = try parseJSON(result.stdout)
        XCTAssertEqual(json["status"] as? String, "blocked")
        XCTAssertEqual((json["failure"] as? [String: Any])?["id"] as? String, "notary_log_not_accepted")
        XCTAssertTrue(FileManager.default.fileExists(atPath: harness.argsURL.appendingPathComponent("log.args").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: harness.argsURL.appendingPathComponent("stapler-staple.args").path))
    }

    func testNotarizeScriptFailsClosedWhenNotaryLogIssuesIsMalformed() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let harness = try makeNotaryHarness(auditStatus: "pass", auditReadiness: "distribution_ready", auditPassReady: true, auditBlockingChecks: 0)
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }
        try """
        {"status":"Accepted","issues":"none"}
        """.write(to: harness.logJSONURL, atomically: true, encoding: .utf8)

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }

        let result = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [
                "PATH": "\(harness.binURL.path):\(ProcessInfo.processInfo.environment["PATH"] ?? "")",
                "SUPERMOVER_NOTARY_KEYCHAIN_PROFILE": "supermover-ci",
                "SUPERMOVER_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
                "SUPERMOVER_TEST_NOTARY_ARGS_DIR": harness.argsURL.path,
                "SUPERMOVER_TEST_NOTARY_SUBMIT_JSON": harness.submitJSONURL.path,
                "SUPERMOVER_TEST_NOTARY_LOG_JSON": harness.logJSONURL.path,
            ],
            allowNonZeroExit: true
        )

        XCTAssertEqual(result.exitCode, 1)
        let json = try parseJSON(result.stdout)
        XCTAssertEqual(json["status"] as? String, "blocked")
        XCTAssertEqual((json["failure"] as? [String: Any])?["id"] as? String, "notary_log_not_accepted")
        XCTAssertTrue(FileManager.default.fileExists(atPath: harness.argsURL.appendingPathComponent("log.args").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: harness.argsURL.appendingPathComponent("stapler-staple.args").path))

        let sidecarURL = app.appURL.deletingLastPathComponent().appendingPathComponent("\(app.appURL.lastPathComponent).notary/notarization.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarURL.path))
        let sidecarJSON = try parseJSON(String(contentsOf: sidecarURL))
        XCTAssertEqual(sidecarJSON["status"] as? String, "blocked")
        XCTAssertEqual((sidecarJSON["failure"] as? [String: Any])?["id"] as? String, "notary_log_not_accepted")
    }

    func testNotarizeScriptPersistsStructuredSidecarNextToAppOnSuccessfulFakeNotaryFlow() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let harness = try makeNotaryHarness(auditStatus: "pass", auditReadiness: "distribution_ready", auditPassReady: true, auditBlockingChecks: 0)
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }

        let result = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [
                "PATH": "\(harness.binURL.path):\(ProcessInfo.processInfo.environment["PATH"] ?? "")",
                "SUPERMOVER_NOTARY_KEYCHAIN_PROFILE": "supermover-ci",
                "SUPERMOVER_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
                "SUPERMOVER_TEST_NOTARY_ARGS_DIR": harness.argsURL.path,
                "SUPERMOVER_TEST_NOTARY_SUBMIT_JSON": harness.submitJSONURL.path,
                "SUPERMOVER_TEST_NOTARY_LOG_JSON": harness.logJSONURL.path,
            ],
            allowNonZeroExit: false
        )

        XCTAssertEqual(result.exitCode, 0)
        let sidecarURL = app.appURL.deletingLastPathComponent().appendingPathComponent("\(app.appURL.lastPathComponent).notary/notarization.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarURL.path))
        let sidecarJSON = try parseJSON(String(contentsOf: sidecarURL))
        XCTAssertEqual(sidecarJSON["status"] as? String, "pass")
        XCTAssertEqual(
            (sidecarJSON["audit"] as? [String: Any])?["path"] as? String,
            sidecarURL.deletingLastPathComponent().appendingPathComponent("post-staple.audit.json").path
        )
        XCTAssertEqual((sidecarJSON["submission"] as? [String: Any])?["status"] as? String, "Accepted")
        XCTAssertEqual((sidecarJSON["audit"] as? [String: Any])?["readiness"] as? String, "distribution_ready")
        let sidecarLogURL = sidecarURL.deletingLastPathComponent().appendingPathComponent("notary-log.json")
        XCTAssertEqual((sidecarJSON["notary_log"] as? [String: Any])?["path"] as? String, sidecarLogURL.path)
        XCTAssertTrue(
            FileManager.default.fileExists(
                atPath: sidecarURL.deletingLastPathComponent().appendingPathComponent("post-staple.audit.json").path
            )
        )
        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarLogURL.path))
    }

    func testNotarizeScriptFailsClosedWhenPostStapleAuditRemainsBlocked() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let harness = try makeNotaryHarness(auditStatus: "blocked", auditReadiness: "blocked", auditPassReady: false, auditBlockingChecks: 3)
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }

        let result = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [
                "PATH": "\(harness.binURL.path):\(ProcessInfo.processInfo.environment["PATH"] ?? "")",
                "SUPERMOVER_NOTARY_KEYCHAIN_PROFILE": "supermover-ci",
                "SUPERMOVER_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
                "SUPERMOVER_TEST_NOTARY_ARGS_DIR": harness.argsURL.path,
                "SUPERMOVER_TEST_NOTARY_SUBMIT_JSON": harness.submitJSONURL.path,
                "SUPERMOVER_TEST_NOTARY_LOG_JSON": harness.logJSONURL.path,
            ],
            allowNonZeroExit: true
        )

        XCTAssertEqual(result.exitCode, 1)
        let json = try parseJSON(result.stdout)
        XCTAssertEqual(json["status"] as? String, "blocked")
        XCTAssertEqual((json["failure"] as? [String: Any])?["id"] as? String, "post_staple_audit_blocked")
        XCTAssertEqual((json["audit"] as? [String: Any])?["status"] as? String, "blocked")
        XCTAssertEqual((json["audit"] as? [String: Any])?["blocking_checks"] as? Int, 3)
        let sidecarURL = app.appURL.deletingLastPathComponent().appendingPathComponent("\(app.appURL.lastPathComponent).notary/notarization.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarURL.path))
        let sidecarJSON = try parseJSON(String(contentsOf: sidecarURL))
        XCTAssertEqual(sidecarJSON["status"] as? String, "blocked")
        XCTAssertEqual((sidecarJSON["failure"] as? [String: Any])?["id"] as? String, "post_staple_audit_blocked")
    }

    func testNotarizeScriptOverwritesStaleCanonicalSidecarBeforePostStapleAudit() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let currentManifest = try AcceptanceReleaseEvidenceFixtures.bundledProvenanceManifest(appBundleURL: app.appURL)
        try writeCanonicalNotarizationSidecar(
            appURL: app.appURL,
            sidecarAppPath: "/tmp/old/SuperMover.app",
            auditAppPath: "/tmp/old/SuperMover.app",
            auditProvenanceManifest: currentManifest
        )

        let harness = try makeNotaryHarness(auditStatus: "pass", auditReadiness: "distribution_ready", auditPassReady: true, auditBlockingChecks: 0)
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }

        let workDir = try makeTemporaryDirectory()
        defer { try? FileManager.default.removeItem(at: workDir) }

        let result = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [
                "PATH": "\(harness.binURL.path):\(ProcessInfo.processInfo.environment["PATH"] ?? "")",
                "SUPERMOVER_NOTARY_KEYCHAIN_PROFILE": "supermover-ci",
                "SUPERMOVER_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
                "SUPERMOVER_TEST_NOTARY_ARGS_DIR": harness.argsURL.path,
                "SUPERMOVER_TEST_NOTARY_SUBMIT_JSON": harness.submitJSONURL.path,
                "SUPERMOVER_TEST_NOTARY_LOG_JSON": harness.logJSONURL.path,
            ],
            allowNonZeroExit: false
        )

        XCTAssertEqual(result.exitCode, 0)
        let sidecarURL = app.appURL.deletingLastPathComponent().appendingPathComponent("\(app.appURL.lastPathComponent).notary/notarization.json")
        XCTAssertTrue(FileManager.default.fileExists(atPath: sidecarURL.path))
        let sidecarJSON = try parseJSON(String(contentsOf: sidecarURL))
        XCTAssertEqual(sidecarJSON["status"] as? String, "pass")
        XCTAssertEqual(sidecarJSON["app_path"] as? String, app.appURL.path)
        XCTAssertEqual((sidecarJSON["submission"] as? [String: Any])?["status"] as? String, "Accepted")
        XCTAssertEqual((sidecarJSON["audit"] as? [String: Any])?["readiness"] as? String, "distribution_ready")
    }

    func testNotarizeScriptKeepsSidecarCurrentWhenCustomWorkDirIsRemoved() throws {
        let head = try currentGitHead()
        let app = try makeSignedTestApp(
            cliVersionOutput: "supermover 0.1.0-dev",
            provenanceCLIPath: "Contents/Resources/bin/supermover",
            provenanceCLIVersion: "supermover 0.1.0-dev",
            provenanceGitCommit: head
        )
        defer { try? FileManager.default.removeItem(at: app.rootURL) }

        let harness = try makeNotaryHarness(auditStatus: "pass", auditReadiness: "distribution_ready", auditPassReady: true, auditBlockingChecks: 0)
        defer { try? FileManager.default.removeItem(at: harness.rootURL) }

        let workDir = try makeTemporaryDirectory()
        let result = try runNotarizeScript(
            arguments: [
                "--app", app.appURL.path,
                "--work-dir", workDir.path,
            ],
            environment: [
                "PATH": "\(harness.binURL.path):\(ProcessInfo.processInfo.environment["PATH"] ?? "")",
                "SUPERMOVER_NOTARY_KEYCHAIN_PROFILE": "supermover-ci",
                "SUPERMOVER_AUDIT_APP_SCRIPT": harness.auditScriptURL.path,
                "SUPERMOVER_TEST_NOTARY_ARGS_DIR": harness.argsURL.path,
                "SUPERMOVER_TEST_NOTARY_SUBMIT_JSON": harness.submitJSONURL.path,
                "SUPERMOVER_TEST_NOTARY_LOG_JSON": harness.logJSONURL.path,
            ],
            allowNonZeroExit: false
        )

        XCTAssertEqual(result.exitCode, 0)
        try FileManager.default.removeItem(at: workDir)

        let audit = try runAppAudit(appURL: app.appURL)
        XCTAssertEqual(audit.exitCode, 1)
        XCTAssertEqual(audit.status, "blocked")
        XCTAssertFalse(audit.blockedCheckIDs.contains("notarization.sidecar.currentness"))
        XCTAssertFalse(audit.blockedCheckIDs.contains("notarization.sidecar.release_ready"))
    }

    private func parseJSON(_ stdout: String, file: StaticString = #filePath, line: UInt = #line) throws -> [String: Any] {
        let data = try XCTUnwrap(stdout.data(using: .utf8), file: file, line: line)
        return try XCTUnwrap(try JSONSerialization.jsonObject(with: data) as? [String: Any], file: file, line: line)
    }
}

extension XCTestCase {
    func runNotarizeScript(
        arguments: [String],
        environment: [String: String],
        allowNonZeroExit: Bool,
        file: StaticString = #filePath,
        line: UInt = #line
    ) throws -> (stdout: String, stderr: String, exitCode: Int32) {
        let repoRoot = repositoryRootURL(file: file, line: line)
        let executableURL = repoRoot.appendingPathComponent("macos/script/notarize-app.sh")
        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: executableURL,
            arguments: arguments,
            environment: environment,
            currentDirectoryURL: repoRoot
        )
        if !allowNonZeroExit {
            XCTAssertEqual(
                result.exitCode,
                0,
                "command failed: \(executableURL.path) \(arguments.joined(separator: " "))\nstdout:\n\(result.stdout)\nstderr:\n\(result.stderr)",
                file: file,
                line: line
            )
        }
        return (result.stdout, result.stderr, result.exitCode)
    }

    func makeNotaryHarness(
        auditStatus: String,
        auditReadiness: String,
        auditPassReady: Bool,
        auditBlockingChecks: Int
    ) throws -> (rootURL: URL, binURL: URL, argsURL: URL, submitJSONURL: URL, logJSONURL: URL, auditScriptURL: URL) {
        let rootURL = try makeTemporaryDirectory()
        let binURL = rootURL.appendingPathComponent("bin", isDirectory: true)
        let argsURL = rootURL.appendingPathComponent("args", isDirectory: true)
        try FileManager.default.createDirectory(at: binURL, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: argsURL, withIntermediateDirectories: true)

        let submitJSONURL = rootURL.appendingPathComponent("submit.json")
        try """
        {"id":"11111111-1111-1111-1111-111111111111","status":"Accepted","message":"Ready for distribution"}
        """.write(to: submitJSONURL, atomically: true, encoding: .utf8)

        let logJSONURL = rootURL.appendingPathComponent("log.json")
        try AcceptanceReleaseEvidenceFixtures.notaryLogJSON()
            .write(to: logJSONURL, atomically: true, encoding: .utf8)

        let xcrunURL = binURL.appendingPathComponent("xcrun")
        try """
        #!/bin/sh
        set -eu
        args_dir=${SUPERMOVER_TEST_NOTARY_ARGS_DIR:?}
        submit_json=${SUPERMOVER_TEST_NOTARY_SUBMIT_JSON:?}
        log_json=${SUPERMOVER_TEST_NOTARY_LOG_JSON:?}
        case "${1:-}" in
          notarytool)
            shift
            subcommand=${1:-}
            shift
            case "$subcommand" in
              submit)
                printf '%s\n' "$@" > "$args_dir/submit.args"
                cat "$submit_json"
                ;;
              log)
                printf '%s\n' "$@" > "$args_dir/log.args"
                prev=""
                last=""
                for arg in "$@"; do
                  prev=$last
                  last=$arg
                done
                cat "$log_json" > "$last"
                ;;
              *)
                printf 'unsupported notarytool subcommand: %s\n' "$subcommand" >&2
                exit 64
                ;;
            esac
            ;;
          stapler)
            shift
            subcommand=${1:-}
            shift
            printf '%s\n' "$@" > "$args_dir/stapler-$subcommand.args"
            ;;
          *)
            printf 'unsupported xcrun invocation: %s\n' "${1:-}" >&2
            exit 64
            ;;
        esac
        """.write(to: xcrunURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: xcrunURL.path)

        let dittoURL = binURL.appendingPathComponent("ditto")
        try """
        #!/bin/sh
        set -eu
        args_dir=${SUPERMOVER_TEST_NOTARY_ARGS_DIR:?}
        printf '%s\n' "$@" > "$args_dir/ditto.args"
        last=""
        for arg in "$@"; do
          last=$arg
        done
        mkdir -p "$(dirname "$last")"
        : > "$last"
        """.write(to: dittoURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: dittoURL.path)

        let auditScriptURL = rootURL.appendingPathComponent("audit-app.sh")
        try """
        #!/bin/sh
        set -eu
        app_path=${1:?missing app path}
        provenance_path="$app_path/Contents/Resources/supermover-provenance.json"
        provenance_json=$(cat "$provenance_path")
        cat <<EOF
        {
          "schema": "supermover.macos.app_audit.v1",
          "status": "\(auditStatus)",
          "readiness": "\(auditReadiness)",
          "app_path": "$app_path",
          "provenance": {
            "path": "$provenance_path",
            "exists": true,
            "loaded": true,
            "error": null,
            "manifest": $provenance_json
          },
          "summary": {
            "pass_ready": \(auditPassReady ? "true" : "false"),
            "blocking_checks": \(auditBlockingChecks)
          },
          "checks": []
        }
        EOF
        if [ "\(auditStatus)" = "pass" ]; then
          exit 0
        fi
        exit 1
        """.write(to: auditScriptURL, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: auditScriptURL.path)

        return (rootURL, binURL, argsURL, submitJSONURL, logJSONURL, auditScriptURL)
    }
}
