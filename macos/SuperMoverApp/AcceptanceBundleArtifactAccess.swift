import Foundation

struct AcceptanceBundleArtifactAccess {
    enum AccessError: Error, Equatable {
        case invalidBundleRoot(String)
        case symlinkRejected(String)
        case unsafeArtifactPath(String)
        case unsafeOutputArtifact(String)
        case unreadableArtifact(String)
        case missingArtifact(String)
    }

    let fileManager: FileManager

    init(fileManager: FileManager = .default) {
        self.fileManager = fileManager
    }

    func metaURL(bundleRootURL: URL) throws -> URL {
        let validatedBundleRootURL = try validatedBundleRootURL(bundleRootURL)
        let metaURL = validatedBundleRootURL.appendingPathComponent("meta.json")
        guard fileManager.fileExists(atPath: metaURL.path) else {
            throw AccessError.missingArtifact("meta.json")
        }
        let values = try metaURL.resourceValues(forKeys: [.isSymbolicLinkKey])
        if values.isSymbolicLink == true {
            throw AccessError.symlinkRejected(metaURL.path)
        }
        guard try isSingleLinkRegularFile(metaURL) else {
            throw AccessError.unreadableArtifact("meta.json")
        }
        return metaURL
    }

    func validatedBundleRootURL(_ bundleRootURL: URL) throws -> URL {
        var isDirectory: ObjCBool = false
        guard fileManager.fileExists(atPath: bundleRootURL.path, isDirectory: &isDirectory), isDirectory.boolValue else {
            throw AccessError.invalidBundleRoot(bundleRootURL.path)
        }
        let values = try bundleRootURL.resourceValues(forKeys: [.isSymbolicLinkKey])
        if values.isSymbolicLink == true {
            throw AccessError.symlinkRejected(bundleRootURL.path)
        }
        return bundleRootURL
    }

    func artifactURL(relativePath: String, bundleRootURL: URL) throws -> URL {
        let validatedBundleRootURL = try validatedBundleRootURL(bundleRootURL)
        let trimmed = relativePath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty,
              !trimmed.hasPrefix("/"),
              !trimmed.hasPrefix("~") else {
            throw AccessError.unsafeArtifactPath(relativePath)
        }

        let parts = trimmed.split(separator: "/", omittingEmptySubsequences: false).map(String.init)
        guard !parts.isEmpty else {
            throw AccessError.unsafeArtifactPath(relativePath)
        }

        var current = validatedBundleRootURL
        for part in parts {
            guard !part.isEmpty, part != ".", part != ".." else {
                throw AccessError.unsafeArtifactPath(relativePath)
            }
            current = current.appendingPathComponent(part)
            if fileManager.fileExists(atPath: current.path) {
                let values = try current.resourceValues(forKeys: [.isSymbolicLinkKey])
                if values.isSymbolicLink == true {
                    throw AccessError.unsafeArtifactPath(relativePath)
                }
            }
        }

        let rootPath = validatedBundleRootURL.standardizedFileURL.path
        let artifactPath = current.standardizedFileURL.path
        guard artifactPath == rootPath || artifactPath.hasPrefix(rootPath + "/") else {
            throw AccessError.unsafeArtifactPath(relativePath)
        }
        return current
    }

    func writableArtifactURL(relativePath: String, bundleRootURL: URL) throws -> URL {
        let url = try artifactURL(relativePath: relativePath, bundleRootURL: bundleRootURL)
        guard pathExistsOrIsSymlink(url) else {
            return url
        }
        do {
            guard try isSingleLinkRegularFile(url) else {
                throw AccessError.unsafeOutputArtifact(url.path)
            }
        } catch AccessError.unsafeOutputArtifact {
            throw AccessError.unsafeOutputArtifact(url.path)
        } catch {
            throw AccessError.unsafeOutputArtifact(url.path)
        }
        return url
    }

    func artifactDataIfPresent(relativePath: String, bundleRootURL: URL) throws -> Data? {
        let url = try artifactURL(relativePath: relativePath, bundleRootURL: bundleRootURL)
        guard fileManager.fileExists(atPath: url.path) else {
            return nil
        }
        guard try isSingleLinkRegularFile(url) else {
            throw AccessError.unreadableArtifact(relativePath)
        }
        do {
            return try Data(contentsOf: url)
        } catch {
            throw AccessError.unreadableArtifact(relativePath)
        }
    }

    func requiredArtifactData(relativePath: String, bundleRootURL: URL) throws -> Data {
        guard let data = try artifactDataIfPresent(relativePath: relativePath, bundleRootURL: bundleRootURL) else {
            throw AccessError.missingArtifact(relativePath)
        }
        return data
    }

    func regularArtifactExists(relativePath: String, bundleRootURL: URL) throws -> Bool {
        let url = try artifactURL(relativePath: relativePath, bundleRootURL: bundleRootURL)
        guard fileManager.fileExists(atPath: url.path) else {
            return false
        }
        return try isSingleLinkRegularFile(url)
    }

    private func isSingleLinkRegularFile(_ url: URL) throws -> Bool {
        let values = try url.resourceValues(forKeys: [.isRegularFileKey, .isSymbolicLinkKey, .linkCountKey])
        return values.isRegularFile == true && values.isSymbolicLink != true && (values.linkCount ?? 1) == 1
    }

    private func pathExistsOrIsSymlink(_ url: URL) -> Bool {
        if fileManager.fileExists(atPath: url.path) {
            return true
        }
        if let values = try? url.resourceValues(forKeys: [.isSymbolicLinkKey]) {
            return values.isSymbolicLink == true
        }
        return false
    }
}
