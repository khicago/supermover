import SwiftUI

struct ProfileNetworkPanel: View {
    @Binding var draft: ProfileNetworkDraft
    let role: WorkbenchRole
    let networkStatus: String
    let networkTint: Color
    let runTask: (SuperMoverTaskKind) -> Void
    var localization: AppChromeLocalization = AppChromeLocalization(language: .english)

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 12) {
                VStack(alignment: .leading, spacing: 6) {
                    Text(localization.text("Migration Config Network"))
                        .font(.system(size: 12, weight: .semibold))
                        .foregroundStyle(SMColor.secondaryText)
                    Text(localization.text("Receiver address and local TLS identity stay in the migration config file. This app writes reviewed SSOT material through `profile set-network`, not transient overrides."))
                        .font(.system(size: 12))
                        .foregroundStyle(SMColor.secondaryText)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Spacer()
                EvidenceChip(label: localization.text("status"), value: networkStatus, tint: networkTint)
            }

            HStack(spacing: 14) {
                profileField(localization.text("Receiver URL"), text: $draft.receiverURL, placeholder: "https://127.0.0.1:9443")
                profileField(localization.text("TLS Certificate"), text: $draft.tlsCertificatePath, placeholder: localization.text("Path to device certificate"))
                profileField(localization.text("TLS Private Key"), text: $draft.tlsPrivateKeyPath, placeholder: localization.text("Path to device private key"))
            }

            HStack(spacing: 18) {
                Toggle(localization.text("Clear Receiver URL"), isOn: $draft.clearReceiverURL)
                    .toggleStyle(.checkbox)
                Toggle(localization.text("Clear TLS Identity"), isOn: $draft.clearTLSIdentity)
                    .toggleStyle(.checkbox)
            }
            .font(.system(size: 12))
            .foregroundStyle(SMColor.secondaryText)

            if draft.hasPartialTLSIdentity {
                Text(localization.text("TLS certificate and private key paths are atomic. Provide both together or clear the stored TLS identity."))
                    .font(.system(size: 12))
                    .foregroundStyle(SMColor.amber)
                    .fixedSize(horizontal: false, vertical: true)
            } else if !draft.hasRequestedChange {
                Text(localization.text("Leave fields empty to keep current migration config network material unchanged."))
                    .font(.system(size: 12))
                    .foregroundStyle(SMColor.secondaryText)
                    .fixedSize(horizontal: false, vertical: true)
            }

            HStack(spacing: 12) {
                if role != .observer {
                    ActionButton(localization.text("Update Config Network"), systemImage: "network") {
                        runTask(.profileSetNetwork)
                    }
                }
                ActionButton(localization.text("Lint Config"), systemImage: "checklist") {
                    runTask(.lintProfile)
                }
                ActionButton(localization.text("Read Status"), systemImage: "waveform.path.ecg") {
                    runTask(.status)
                }
            }
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(SMColor.input)
        .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).stroke(SMColor.hairline))
    }
}

private func profileField(_ label: String, text: Binding<String>, placeholder: String) -> some View {
    VStack(alignment: .leading, spacing: 6) {
        Text(label)
            .font(.system(size: 12, weight: .semibold))
            .foregroundStyle(SMColor.secondaryText)
        TextField(placeholder, text: text)
            .textFieldStyle(.plain)
            .font(.system(size: 13, design: .monospaced))
            .padding(10)
            .background(SMColor.card)
            .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
            .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(SMColor.hairline))
    }
    .frame(maxWidth: .infinity, alignment: .leading)
}
