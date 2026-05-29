import Foundation

enum AcceptanceControlPlaneID {
    static func safeRawSegment(_ value: String?) -> String? {
        guard let value, isSafeRawSegment(value) else {
            return nil
        }
        return value
    }

    static func isSafeRawSegment(_ value: String) -> Bool {
        !value.isEmpty
            && value.rangeOfCharacter(from: .whitespacesAndNewlines) == nil
            && value.rangeOfCharacter(from: .controlCharacters) == nil
            && value != "."
            && value != ".."
            && !value.hasPrefix("~")
            && !value.contains("/")
            && !value.contains("\\")
    }
}
