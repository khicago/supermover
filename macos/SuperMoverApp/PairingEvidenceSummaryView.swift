import SwiftUI

struct PairingEvidenceSummaryView: View {
    private let pairing: PairingEvidenceSummaryModel?

    init(
        statusPairing: StatusSnapshot.Pairing?,
        reportPairing: ReportSnapshot.Pairing?
  ) {
    if let statusPairing {
      pairing = PairingEvidenceSummaryModel(statusPairing)
    } else if let reportPairing {
      pairing = PairingEvidenceSummaryModel(reportPairing)
    } else {
      pairing = nil
    }
  }

  var body: some View {
    if let pairing {
      VStack(alignment: .leading, spacing: 8) {
        evidenceLine("pairing status", pairing.status)
        evidenceLine("receipt source", pairing.receiptSource ?? "unavailable")
        evidenceLine("receipt id", pairing.receiptID ?? "unavailable")
        evidenceLine("receipt path", pairing.receiptPath ?? "unavailable")
        if let sourcePath = pairing.sourceReceiptPath, !sourcePath.isEmpty {
          evidenceLine("source receipt", sourcePath)
        }
        if let targetPath = pairing.targetReceiptPath, !targetPath.isEmpty {
          evidenceLine("target receipt", targetPath)
        }
        if let verifiedAt = pairing.verifiedAt, !verifiedAt.isEmpty {
          evidenceLine("verified", verifiedAt)
        }
        if let method = pairing.method, !method.isEmpty {
          evidenceLine("method", method)
        }
      }
      .padding(12)
      .frame(maxWidth: .infinity, alignment: .leading)
      .background(SMColor.input)
      .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
      .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).stroke(SMColor.hairline))
    }
  }

  private func evidenceLine(_ label: String, _ value: String) -> some View {
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
}

private struct PairingEvidenceSummaryModel {
  let status: String
  let receiptSource: String?
  let receiptID: String?
  let receiptPath: String?
  let sourceReceiptPath: String?
  let targetReceiptPath: String?
  let verifiedAt: String?
  let method: String?

    init(_ pairing: StatusSnapshot.Pairing) {
    status = pairing.status
    receiptSource = pairing.receipt_source
    receiptID = pairing.receipt_id
    receiptPath = pairing.receipt_path
    sourceReceiptPath = pairing.source_receipt_path
    targetReceiptPath = pairing.target_receipt_path
    verifiedAt = pairing.verified_at
    method = pairing.method
  }

    init(_ pairing: ReportSnapshot.Pairing) {
    status = pairing.status
    receiptSource = pairing.receipt_source
    receiptID = pairing.receipt_id
    receiptPath = pairing.receipt_path
    sourceReceiptPath = pairing.source_receipt_path
    targetReceiptPath = pairing.target_receipt_path
    verifiedAt = pairing.verified_at
    method = pairing.method
  }
}
