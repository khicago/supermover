import Foundation

struct ProfilePairingSnapshot: Decodable, Equatable {
    struct Target: Decodable, Equatable {
        let pairing_receipt_id: String?
        let device_public_key: String?
        let paired_at: String?
        let local_pairing_receipt_path: String?
    }

    struct Network: Decodable, Equatable {
        let receiver_url: String?
    }

    let target: Target
    let network: Network?
}

struct ProfilePairingReader {
    enum ReadError: LocalizedError, Equatable {
        case missingFile(URL)
        case malformedProfile(URL)

        var errorDescription: String? {
            switch self {
            case let .missingFile(url):
                return "Migration config file is missing at \(url.path)."
            case let .malformedProfile(url):
                return "Migration config file is malformed at \(url.path)."
            }
        }
    }

    func read(profileURL: URL) throws -> ProfilePairingSnapshot {
        guard FileManager.default.fileExists(atPath: profileURL.path) else {
            throw ReadError.missingFile(profileURL)
        }
        let data = try Data(contentsOf: profileURL)
        guard let decoded = try? JSONDecoder().decode(ProfilePairingSnapshot.self, from: data) else {
            throw ReadError.malformedProfile(profileURL)
        }
        return decoded
    }
}
