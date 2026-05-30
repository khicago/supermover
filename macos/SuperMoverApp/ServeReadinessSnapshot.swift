import Foundation

struct ServeReadinessSnapshot: Decodable, Equatable {
    let address: String
    let verification_code: String?
    let operator_token: String?
    let mode: String
    let receiver_address: String?
    let receiver_routes: Bool?
    let push_network: Bool?
    let trusted: Bool
    let transfer: Bool
    let expires_at: String?
    let pairing_request: PairingRequestSnapshot?

    init(
        address: String,
        verification_code: String?,
        mode: String,
        receiver_address: String?,
        receiver_routes: Bool?,
        push_network: Bool?,
        trusted: Bool,
        transfer: Bool,
        expires_at: String?,
        operator_token: String? = nil,
        pairing_request: PairingRequestSnapshot? = nil
    ) {
        self.address = address
        self.verification_code = verification_code
        self.operator_token = operator_token
        self.mode = mode
        self.receiver_address = receiver_address
        self.receiver_routes = receiver_routes
        self.push_network = push_network
        self.trusted = trusted
        self.transfer = transfer
        self.expires_at = expires_at
        self.pairing_request = pairing_request
    }

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

struct PairingRequestSnapshot: Decodable, Equatable {
    let protocol_version: String
    let id: String
    let status: String
    let source_profile_id: String
    let source_profile_name: String?
    let source_device_id: String
    let requested_at: String
    let expires_at: String
    let decided_at: String?

    var sourceLabel: String {
        if let name = source_profile_name?.trimmingCharacters(in: .whitespacesAndNewlines), !name.isEmpty {
            return "\(name) (\(source_profile_id))"
        }
        return source_profile_id
    }
}
