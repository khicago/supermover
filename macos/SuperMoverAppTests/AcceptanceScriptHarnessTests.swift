import XCTest

final class AcceptanceScriptHarnessTests: XCTestCase {
    func testBuildAppResourceCopyIncludesProcessedLocalizationDirectories() throws {
        let workDir = try AcceptanceScriptHarness.makeDirectory(named: "build-app-resources")
        defer { try? FileManager.default.removeItem(at: workDir) }

        let bundleRoot = workDir.appendingPathComponent("SuperMoverApp_SuperMoverApp.bundle", isDirectory: true)
        let bundleResources = bundleRoot.appendingPathComponent("Resources", isDirectory: true)
        let outputResources = workDir.appendingPathComponent("SuperMover.app/Contents/Resources", isDirectory: true)
        try FileManager.default.createDirectory(at: bundleRoot.appendingPathComponent("en.lproj", isDirectory: true), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: bundleRoot.appendingPathComponent("zh-hans.lproj", isDirectory: true), withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: bundleResources, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: outputResources, withIntermediateDirectories: true)
        try "Info".write(to: bundleRoot.appendingPathComponent("Info.plist"), atomically: true, encoding: .utf8)
        try "\"settings.title\" = \"Settings\";\n".write(
            to: bundleRoot.appendingPathComponent("en.lproj/Localizable.strings"),
            atomically: true,
            encoding: .utf8
        )
        try "\"settings.title\" = \"设置\";\n".write(
            to: bundleRoot.appendingPathComponent("zh-hans.lproj/Localizable.strings"),
            atomically: true,
            encoding: .utf8
        )
        try "nested".write(to: bundleResources.appendingPathComponent("Nested.txt"), atomically: true, encoding: .utf8)

        let repoRoot = AcceptanceScriptHarness.repoRootURL(file: #filePath)
        let script = """
        . "\(repoRoot.appendingPathComponent("macos/script/lib/app-build-resources.sh").path)"
        copy_swiftpm_app_resources "\(bundleRoot.path)" "\(outputResources.path)"
        """

        let result = try AcceptanceScriptHarness.runProcess(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: ["-c", script],
            environment: [:],
            currentDirectoryURL: repoRoot
        )

        XCTAssertTrue(result.stdout.isEmpty)
        XCTAssertTrue(FileManager.default.fileExists(atPath: outputResources.appendingPathComponent("en.lproj/Localizable.strings").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: outputResources.appendingPathComponent("zh-hans.lproj/Localizable.strings").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: outputResources.appendingPathComponent("Nested.txt").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: outputResources.appendingPathComponent("Info.plist").path))
    }

    func testRunProcessAllowFailureDrainsStdoutAndStderrWhileProcessRuns() throws {
        let script = """
        ( sleep 2; kill -KILL $$ ) &
        killer=$!
        chunk=$(printf '%4096s' '')
        i=0
        while [ "$i" -lt 1024 ]; do
          printf '%s\\n' "$chunk"
          printf '%s\\n' "$chunk" >&2
          i=$((i + 1))
        done
        printf 'stdout-finished\\n'
        printf 'stderr-finished\\n' >&2
        kill "$killer" 2>/dev/null || true
        wait "$killer" 2>/dev/null || true
        """

        let result = try AcceptanceScriptHarness.runProcessAllowFailure(
            executableURL: URL(fileURLWithPath: "/bin/sh"),
            arguments: ["-c", script],
            environment: [:],
            currentDirectoryURL: AcceptanceScriptHarness.repoRootURL(file: #filePath)
        )

        XCTAssertEqual(
            result.exitCode,
            0,
            "stdoutBytes=\(result.stdout.utf8.count) stderrBytes=\(result.stderr.utf8.count)"
        )
        XCTAssertTrue(result.stdout.contains("stdout-finished"))
        XCTAssertTrue(result.stderr.contains("stderr-finished"))
        XCTAssertGreaterThan(result.stdout.utf8.count, 4 * 1024 * 1024)
        XCTAssertGreaterThan(result.stderr.utf8.count, 4 * 1024 * 1024)
    }
}
