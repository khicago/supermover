import Foundation

struct PairingReceiptDraft: Equatable {
    var exportTarget = ""
    var importReceiptFile = ""

    var trimmedExportTarget: String {
        exportTarget.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var trimmedImportReceiptFile: String {
        importReceiptFile.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    var exportArguments: [String] {
        guard !trimmedExportTarget.isEmpty else {
            return []
        }
        return ["--receipt-out", trimmedExportTarget]
    }

    var adoptArguments: [String] {
        guard !trimmedImportReceiptFile.isEmpty else {
            return []
        }
        return ["--receipt-file", trimmedImportReceiptFile]
    }

    var contextInputs: [String] {
        [trimmedExportTarget, trimmedImportReceiptFile]
    }
}
