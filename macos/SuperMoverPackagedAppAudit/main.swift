import Foundation
import SuperMoverAppSupport

@main
struct SuperMoverPackagedAppAuditMain {
    static func main() throws {
        let arguments = Array(CommandLine.arguments.dropFirst())
        let appURL: URL
        switch arguments.count {
        case 0:
            appURL = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
        case 1:
            appURL = URL(fileURLWithPath: arguments[0])
        default:
            FileHandle.standardError.write(Data("usage: SuperMoverPackagedAppAudit [path/to/SuperMover.app]\n".utf8))
            exit(2)
        }

        let report = PackagedAppAuditor().audit(appURL: appURL)
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        let data = try encoder.encode(report)
        FileHandle.standardOutput.write(data)
        FileHandle.standardOutput.write(Data("\n".utf8))
        exit(report.status == "pass" ? 0 : 1)
    }
}
