import SwiftUI

struct ServeReadinessPanel: View {
    let snapshot: ServeReadinessSnapshot?

    var body: some View {
        if let snapshot {
            VStack(alignment: .leading, spacing: 8) {
                serveLine("mode", snapshot.mode)
                serveLine("address", snapshot.address)
                if let verificationCode = snapshot.verification_code, !verificationCode.isEmpty {
                    serveLine("verification code", verificationCode)
                }
                if let receiverAddress = snapshot.receiver_address, !receiverAddress.isEmpty {
                    serveLine("receiver", receiverAddress)
                }
                if let expiresAt = snapshot.expires_at, !expiresAt.isEmpty {
                    serveLine("expires", expiresAt)
                }
                serveLine("trusted", snapshot.trusted ? "true" : "false")
                serveLine("transfer", snapshot.transfer ? "true" : "false")
                if let receiverRoutes = snapshot.receiver_routes {
                    serveLine("receiver routes", receiverRoutes ? "true" : "false")
                }
                if let pushNetwork = snapshot.push_network {
                    serveLine("push network", pushNetwork ? "true" : "false")
                }
            }
            .padding(12)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(SMColor.input)
            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
            .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).stroke(SMColor.hairline))
        }
    }
}

private func serveLine(_ label: String, _ value: String) -> some View {
    HStack(alignment: .top, spacing: 10) {
        Text(label.uppercased())
            .font(.system(size: 10, weight: .semibold))
            .foregroundStyle(SMColor.secondaryText)
            .frame(width: 110, alignment: .leading)
        Text(value)
            .font(.system(size: 12, design: .monospaced))
            .foregroundStyle(SMColor.primaryText)
            .textSelection(.enabled)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}
