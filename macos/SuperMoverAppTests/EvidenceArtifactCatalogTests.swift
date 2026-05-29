import XCTest
@testable import SuperMoverApp

final class EvidenceArtifactCatalogTests: XCTestCase {
    private let fileManager = FileManager.default

    func testClassifiesCurrentControlPlaneArtifactFamilies() throws {
        let targetRoot = try makeTargetRoot()
        let fixtures: [(path: String, family: EvidenceArtifactFamily, artifactID: String?)] = [
            ("profiles/source.json", .profile, "source"),
            ("pairings/target.json", .pairing, "target"),
            ("sessions/session-1/session.json", .session, "session-1"),
            ("sessions/session-1/receipt.json", .session, "session-1"),
            ("sessions/session-1/manifest.json", .session, "session-1"),
            ("sessions/session-1/network-transfer.json", .networkTransfer, "session-1"),
            ("warnings/warning-1.json", .warning, "warning-1"),
            ("deleted/delete-1.json", .deleted, "delete-1"),
            ("drift/drift-1.json", .drift, "drift-1"),
            ("prune/approvals/approval-1.json", .pruneApproval, "approval-1"),
            ("prune/receipts/prune-receipt-1.json", .pruneReceipt, "prune-receipt-1"),
            ("reconcile/receipts/reconcile-1.json", .reconcileReceipt, "reconcile-1"),
            ("daemon/state.json", .daemon, "state"),
            ("daemon/events/event-1.json", .daemonEvent, "event-1"),
            ("incremental-sync/profiles/profile-1/targets/target-1/queue.json", .incrementalSyncQueue, "profile-1/target-1"),
            ("incremental-sync/profiles/profile-1/targets/target-1/runs/run-1.json", .incrementalSyncRun, "profile-1/target-1/run-1"),
            ("agent/session-1-influence.json", .agentInfluence, "session-1-influence"),
            ("history/index.json", .historyIndex, "index"),
            ("recovery/state.json", .recoveryState, "state"),
        ]

        for fixture in fixtures {
            try writeArtifact(fixture.path, contents: #"{"id":"\#(fixture.artifactID ?? fixture.path)"}"#, under: targetRoot)
        }
        try writeArtifact("operator-notes.txt", contents: "plain operator notes", under: targetRoot)

        let catalog = EvidenceArtifactCatalogReader().read(targetRootURL: targetRoot)

        XCTAssertTrue(catalog.problems.isEmpty)
        for fixture in fixtures {
            let artifact = try XCTUnwrap(catalog.artifacts.first { $0.relativePath == ".supermover/\(fixture.path)" })
            XCTAssertEqual(artifact.family, fixture.family, fixture.path)
            XCTAssertEqual(artifact.artifactID, fixture.artifactID, fixture.path)
            XCTAssertEqual(artifact.fileName, URL(fileURLWithPath: fixture.path).lastPathComponent)
            XCTAssertEqual(artifact.jsonStatus, .valid, fixture.path)
            XCTAssertEqual(artifact.issueSeverity, .ok, fixture.path)
        }

        let unknown = try XCTUnwrap(catalog.artifacts.first { $0.relativePath == ".supermover/operator-notes.txt" })
        XCTAssertEqual(unknown.family, .unknownControl)
        XCTAssertEqual(unknown.jsonStatus, .notJSON)
        XCTAssertEqual(unknown.previewText, "plain operator notes")
    }

    func testHiddenArtifactsAndDotDirectoriesAreVisible() throws {
        let targetRoot = try makeTargetRoot()
        try writeArtifact("warnings/.hidden-warning.json", contents: #"{"code":"hidden"}"#, under: targetRoot)
        try writeArtifact(".operator-cache/evidence.json", contents: #"{"kind":"dot-dir"}"#, under: targetRoot)

        let catalog = EvidenceArtifactCatalogReader().read(targetRootURL: targetRoot)

        let hiddenWarning = try XCTUnwrap(catalog.artifacts.first { $0.relativePath == ".supermover/warnings/.hidden-warning.json" })
        XCTAssertEqual(hiddenWarning.family, .warning)
        XCTAssertEqual(hiddenWarning.artifactID, ".hidden-warning")

        let dotDirectoryArtifact = try XCTUnwrap(catalog.artifacts.first { $0.relativePath == ".supermover/.operator-cache/evidence.json" })
        XCTAssertEqual(dotDirectoryArtifact.family, .unknownControl)
        XCTAssertEqual(dotDirectoryArtifact.jsonStatus, .valid)
    }

    func testMalformedJSONIsVisibleAsCatalogProblem() throws {
        let targetRoot = try makeTargetRoot()
        try writeArtifact("drift/bad.json", contents: "{ not-json", under: targetRoot)

        let catalog = EvidenceArtifactCatalogReader().read(targetRootURL: targetRoot)

        let badArtifact = try XCTUnwrap(catalog.artifacts.first { $0.relativePath == ".supermover/drift/bad.json" })
        XCTAssertEqual(badArtifact.family, .drift)
        XCTAssertEqual(badArtifact.jsonStatus, .malformed)
        XCTAssertEqual(badArtifact.issueSeverity, .critical)

        let problem = try XCTUnwrap(catalog.problems.first { $0.relativePath == badArtifact.relativePath })
        XCTAssertEqual(problem.kind, .malformedJSON)
        XCTAssertEqual(problem.severity, .critical)
    }

    func testSymlinkArtifactIsRefusedAndNotRead() throws {
        let targetRoot = try makeTargetRoot()
        let outsideURL = targetRoot.appendingPathComponent("outside.json")
        try #"{"message":"must not be read"}"#.write(to: outsideURL, atomically: true, encoding: .utf8)

        let linkURL = targetRoot
            .appendingPathComponent(".supermover", isDirectory: true)
            .appendingPathComponent("warnings", isDirectory: true)
            .appendingPathComponent("link.json")
        try fileManager.createDirectory(
            at: linkURL.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
        try fileManager.createSymbolicLink(at: linkURL, withDestinationURL: outsideURL)

        let catalog = EvidenceArtifactCatalogReader().read(targetRootURL: targetRoot)

        let linkArtifact = try XCTUnwrap(catalog.artifacts.first { $0.relativePath == ".supermover/warnings/link.json" })
        XCTAssertEqual(linkArtifact.family, .warning)
        XCTAssertEqual(linkArtifact.jsonStatus, .symlink)
        XCTAssertEqual(linkArtifact.issueSeverity, .critical)
        XCTAssertEqual(linkArtifact.previewText, "")
        XCTAssertFalse(linkArtifact.searchText.contains("must not be read"))

        let problem = try XCTUnwrap(catalog.problems.first { $0.relativePath == linkArtifact.relativePath })
        XCTAssertEqual(problem.kind, .symlink)
    }

    func testFilteringAndSearchUseSharedCatalogLogic() throws {
        let targetRoot = try makeTargetRoot()
        try writeArtifact("profiles/main.json", contents: #"{"name":"Local profile"}"#, under: targetRoot)
        try writeArtifact("warnings/warning-1.json", contents: #"{"message":"Needs extra migration config"}"#, under: targetRoot)
        try writeArtifact("drift/drift-1.json", contents: #"{"change":"extra target file"}"#, under: targetRoot)

        let catalog = EvidenceArtifactCatalogReader().read(targetRootURL: targetRoot)

        XCTAssertEqual(
            catalog.filtered(families: [.warning]).map(\.relativePath),
            [".supermover/warnings/warning-1.json"]
        )
        XCTAssertEqual(
            catalog.filtered(query: "extra migration").map(\.relativePath),
            [".supermover/warnings/warning-1.json"]
        )
        XCTAssertEqual(
            catalog.filtered(families: [.profile], query: "LOCAL").map(\.relativePath),
            [".supermover/profiles/main.json"]
        )
        XCTAssertTrue(catalog.filtered(families: [.profile], query: "migration").isEmpty)
    }

    private func makeTargetRoot() throws -> URL {
        let targetRoot = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent("EvidenceArtifactCatalogTests-\(UUID().uuidString)", isDirectory: true)
        try fileManager.createDirectory(at: targetRoot, withIntermediateDirectories: true)
        addTeardownBlock {
            try? FileManager.default.removeItem(at: targetRoot)
        }
        return targetRoot
    }

    private func writeArtifact(
        _ relativePath: String,
        contents: String,
        under targetRoot: URL
    ) throws {
        let artifactURL = targetRoot
            .appendingPathComponent(".supermover", isDirectory: true)
            .appendingPathComponent(relativePath)
        try fileManager.createDirectory(
            at: artifactURL.deletingLastPathComponent(),
            withIntermediateDirectories: true
        )
        try contents.write(to: artifactURL, atomically: true, encoding: .utf8)
    }
}
