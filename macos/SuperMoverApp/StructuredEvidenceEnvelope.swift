import Foundation

enum StructuredEvidenceFreshness: String, Equatable {
    case current
    case stale
}

struct StructuredEvidenceEnvelope: Identifiable {
    let id = UUID()
    let artifactKind: StructuredArtifactKind
    let task: SuperMoverTaskKind
    let loadedAt: Date
    let contextSignature: String
    let exitCode: Int32
    let rawStdout: String
    let stderrSample: String
    var freshness: StructuredEvidenceFreshness
}
