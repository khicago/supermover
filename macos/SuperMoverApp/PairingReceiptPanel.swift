import SwiftUI

struct PairingReceiptPanel: View {
    @Binding var draft: PairingReceiptDraft
    let role: WorkbenchRole
    let runTask: (SuperMoverTaskKind) -> Void
    let chooseExportTarget: () -> Void
    let browseImportReceipt: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            VStack(alignment: .leading, spacing: 6) {
                Text("Pairing Receipt")
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundStyle(SMColor.secondaryText)
                Text("Source pairing can export a durable receipt for transfer. Target pairing import writes that receipt into target `.supermover/pairings` plus target config pins.")
                    .font(.system(size: 12))
                    .foregroundStyle(SMColor.secondaryText)
                    .fixedSize(horizontal: false, vertical: true)
            }

            if role == .source {
                HStack(spacing: 10) {
                    pairingPathField("Receipt Export", text: $draft.exportTarget, placeholder: "Optional file path or directory for exported receipt")
                    CompactActionButton("Choose receipt export path", systemImage: "folder.badge.plus") {
                        chooseExportTarget()
                    }
                }
                HStack(spacing: 12) {
                    PrimaryActionButton("Pair and Export", systemImage: "square.and.arrow.up") {
                        runTask(.pair)
                    }
                    ActionButton("Read Status", systemImage: "waveform.path.ecg") {
                        runTask(.status)
                    }
                }
            } else if role == .target {
                HStack(spacing: 10) {
                    pairingPathField("Receipt File", text: $draft.importReceiptFile, placeholder: "Exported pairing receipt JSON from source")
                    CompactActionButton("Browse pairing receipt", systemImage: "folder") {
                        browseImportReceipt()
                    }
                }
                HStack(spacing: 12) {
                    PrimaryActionButton("Import Pairing Receipt", systemImage: "square.and.arrow.down") {
                        runTask(.profileAdoptPairing)
                    }
                    ActionButton("Read Status", systemImage: "waveform.path.ecg") {
                        runTask(.status)
                    }
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

private func pairingPathField(_ label: String, text: Binding<String>, placeholder: String) -> some View {
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
