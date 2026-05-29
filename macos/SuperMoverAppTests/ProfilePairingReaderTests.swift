import XCTest
@testable import SuperMoverApp

final class ProfilePairingReaderTests: XCTestCase {
    func testReaderLoadsPairingReceiptAndReceiverURL() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: dir) }
        let profileURL = dir.appendingPathComponent("profile.json")
        try """
        {
          "target": {
            "pairing_receipt_id": "pair-1",
            "device_public_key": "sha256:abc",
            "paired_at": "2026-05-16T10:00:00Z",
            "local_pairing_receipt_path": "/tmp/pair-1.json"
          },
          "network": {
            "receiver_url": "https://127.0.0.1:9443"
          }
        }
        """.write(to: profileURL, atomically: true, encoding: .utf8)

        let snapshot = try ProfilePairingReader().read(profileURL: profileURL)
        XCTAssertEqual(snapshot.target.pairing_receipt_id, "pair-1")
        XCTAssertEqual(snapshot.network?.receiver_url, "https://127.0.0.1:9443")
    }
}
