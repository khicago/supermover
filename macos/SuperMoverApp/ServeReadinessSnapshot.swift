import Foundation

struct ServeReadinessSnapshot: Decodable, Equatable {
    let address: String
    let verification_code: String?
    let mode: String
    let receiver_address: String?
    let receiver_routes: Bool?
    let push_network: Bool?
    let trusted: Bool
    let transfer: Bool
    let expires_at: String?

    var summaryLine: String {
        if let receiver = receiver_address, receiver_routes == true, push_network == true {
            return "serve ready address=\(address) mode=\(mode) receiver_address=\(receiver) trusted=\(trusted) transfer=\(transfer)"
        }
        if let verification = verification_code, !verification.isEmpty {
            return "serve ready address=\(address) mode=\(mode) verification_code=\(verification) trusted=\(trusted) transfer=\(transfer)"
        }
        return "serve ready address=\(address) mode=\(mode) trusted=\(trusted) transfer=\(transfer)"
    }
}
