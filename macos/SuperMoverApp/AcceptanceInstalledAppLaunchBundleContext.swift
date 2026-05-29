import Foundation

struct AcceptanceInstalledAppLaunchBundleContext {
    let state: AcceptanceInstalledAppLaunchGate.BundleState
    let snapshot: AcceptanceBundleLoadedSnapshot?
    let installedAppCollectionProof: AcceptanceInstalledAppCollectionProofSummary?
    let installedAppReleaseEvidence: AcceptanceInstalledAppReleaseEvidenceSummary?

    static func resolve(
        bundlePath: String,
        loadedSnapshot: AcceptanceBundleLoadedSnapshot?,
        reader: AcceptanceBundleReader = AcceptanceBundleReader()
    ) -> AcceptanceInstalledAppLaunchBundleContext {
        let trimmedPath = bundlePath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedPath.isEmpty else {
            return .init(
                state: .notConfigured,
                snapshot: nil,
                installedAppCollectionProof: nil,
                installedAppReleaseEvidence: nil
            )
        }
        if let loadedSnapshot, matchesBundlePath(loadedSnapshot.bundleRootPath, trimmedPath) {
            return .init(
                state: .loaded(collectionMode: loadedSnapshot.collectionMode),
                snapshot: loadedSnapshot,
                installedAppCollectionProof: loadedSnapshot.installedAppCollectionProof,
                installedAppReleaseEvidence: loadedSnapshot.installedAppReleaseEvidence
            )
        }

        let bundleRootURL = URL(fileURLWithPath: trimmedPath, isDirectory: true)
        do {
            let snapshot = try reader.load(bundleRootURL: bundleRootURL)
            return .init(
                state: .loaded(collectionMode: snapshot.collectionMode),
                snapshot: snapshot,
                installedAppCollectionProof: snapshot.installedAppCollectionProof,
                installedAppReleaseEvidence: snapshot.installedAppReleaseEvidence
            )
        } catch {
            return .init(
                state: .invalid(path: trimmedPath, detail: error.localizedDescription),
                snapshot: nil,
                installedAppCollectionProof: nil,
                installedAppReleaseEvidence: nil
            )
        }
    }

    private static func matchesBundlePath(_ storedPath: String, _ bundlePath: String) -> Bool {
        URL(fileURLWithPath: storedPath, isDirectory: true).standardizedFileURL.path
            == URL(fileURLWithPath: bundlePath, isDirectory: true).standardizedFileURL.path
    }
}
