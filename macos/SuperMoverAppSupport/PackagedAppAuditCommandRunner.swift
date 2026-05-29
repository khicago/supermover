import Foundation

public struct PackagedAppAuditCommandOutput: Equatable, Sendable {
    public let exitCode: Int32
    public let output: String

    public init(exitCode: Int32, output: String) {
        self.exitCode = exitCode
        self.output = output
    }
}

public struct PackagedAppAuditCommandRunner: Sendable {
    public init() {}

    public func run(executableURL: URL, arguments: [String]) throws -> PackagedAppAuditCommandOutput {
        let process = Process()
        let stdoutPipe = Pipe()
        let stderrPipe = Pipe()
        process.executableURL = executableURL
        process.arguments = arguments
        process.standardOutput = stdoutPipe
        process.standardError = stderrPipe
        try process.run()
        process.waitUntilExit()

        let stdout = String(decoding: stdoutPipe.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
        let stderr = String(decoding: stderrPipe.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
        return PackagedAppAuditCommandOutput(
            exitCode: process.terminationStatus,
            output: stdout + stderr
        )
    }
}
