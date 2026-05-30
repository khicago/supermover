import SwiftUI

struct AcceptanceOperatorEvidencePanel: View {
    @Binding var draft: AcceptanceOperatorEvidenceDraft
    let role: WorkbenchRole
    let bundlePath: String
    let snapshot: AcceptanceBundleLoadedSnapshot?
    let requireOperatorEvidence: Bool
    let requireOperatorEvidenceLocked: Bool
    let chooseArtifact: () -> Void
    let clearArtifact: () -> Void
    let recordEvidence: () -> Void
    let refreshBundle: () -> Void
    var localization: AppChromeLocalization = AppChromeLocalization(language: .english)

    private var currentRecord: AcceptanceBundleSnapshot.OperatorEvidence? {
        snapshot?.operatorEvidence[draft.kind.rawValue]
    }

    private var currentRecordGateStatus: AcceptanceManualEvidenceRecordGateStatus? {
        snapshot?.manualEvidenceRecordGateStatus(
            kind: draft.kind.rawValue,
            requireOperatorEvidence: requireOperatorEvidence
        )
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 12) {
                VStack(alignment: .leading, spacing: 6) {
                    Text(localization.text("Manual Evidence"))
                        .font(.system(size: 12, weight: .semibold))
                        .foregroundStyle(SMColor.secondaryText)
                    Text(localization.text("Record real-device prompt and code-confirmation evidence into the same acceptance bundle contract used by the two-machine harness. This writes durable `meta.json` operator records under the bundle lock; it does not mark the run complete by itself."))
                        .font(.system(size: 12))
                        .foregroundStyle(SMColor.secondaryText)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Spacer()
                if let snapshot {
                    let manualEvidenceGate = snapshot.manualEvidenceGate(
                        requireOperatorEvidence: requireOperatorEvidence
                    )
                    EvidenceChip(
                        label: localization.text("manual gate"),
                        value: manualEvidenceGate.chipValue,
                        tint: manualEvidenceGate.isRequired ? SMColor.amber : SMColor.blue
                    )
                }
            }

            kindStatusFields

            VStack(alignment: .leading, spacing: 6) {
                Text(localization.text("Detail"))
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundStyle(SMColor.secondaryText)
                TextField(localization.text("One durable sentence about what the operator observed"), text: $draft.detail)
                    .textFieldStyle(.plain)
                    .font(.system(size: 13, design: .monospaced))
                    .padding(10)
                    .background(SMColor.card)
                    .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
                    .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(SMColor.hairline))
            }

            artifactPathRow

            if let currentRecord {
                VStack(alignment: .leading, spacing: 6) {
                    acceptanceOperatorLine(localization.text("current"), "\(currentRecord.status) · \(currentRecord.detail)")
                    if let artifact = currentRecord.artifact, !artifact.isEmpty {
                        acceptanceOperatorLine(localization.text("artifact"), artifact)
                    }
                    if let currentRecordGateStatus {
                        acceptanceOperatorLine(
                            currentRecordGateStatus.localizedLabel(using: localization),
                            currentRecordGateStatus.message
                        )
                    }
                }
            } else {
                Text(localization.text(draft.kind.guidance))
                    .font(.system(size: 12))
                    .foregroundStyle(SMColor.secondaryText)
                    .fixedSize(horizontal: false, vertical: true)
            }

            if requireOperatorEvidenceLocked {
                Text(localization.text("Two-machine installed-app bundles always require pass Local Network, firewall, and pairing-confirmation evidence before final evaluate."))
                    .font(.system(size: 12))
                    .foregroundStyle(SMColor.secondaryText)
                    .fixedSize(horizontal: false, vertical: true)
            }

            if role == .observer {
                Text(localization.text("Observer role can inspect acceptance bundles but cannot author manual evidence."))
                    .font(.system(size: 12))
                    .foregroundStyle(SMColor.amber)
                    .fixedSize(horizontal: false, vertical: true)
            } else if bundlePath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                Text(localization.text("Load or select an acceptance bundle directory before recording manual evidence."))
                    .font(.system(size: 12))
                    .foregroundStyle(SMColor.secondaryText)
                    .fixedSize(horizontal: false, vertical: true)
            }

            HStack(spacing: 12) {
                if role != .observer {
                    PrimaryActionButton(localization.text("Record Manual Evidence"), systemImage: "checkmark.shield") {
                        recordEvidence()
                    }
                }
                ActionButton(localization.text("Refresh Bundle"), systemImage: "arrow.clockwise") {
                    refreshBundle()
                }
            }
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(SMColor.input)
        .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).stroke(SMColor.hairline))
    }

    private var kindStatusFields: some View {
        ViewThatFits(in: .horizontal) {
            HStack(alignment: .top, spacing: 14) {
                evidenceKindField
                evidenceStatusField
            }
            VStack(alignment: .leading, spacing: 12) {
                evidenceKindField
                evidenceStatusField
            }
        }
    }

    private var evidenceKindField: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(localization.text("Evidence Kind"))
                .font(.system(size: 12, weight: .semibold))
                .foregroundStyle(SMColor.secondaryText)
            Picker(localization.text("Evidence Kind"), selection: $draft.kind) {
                ForEach(AcceptanceOperatorEvidenceKind.allCases) { kind in
                    Text(localization.text(kind.title)).tag(kind)
                }
            }
            .pickerStyle(.menu)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var evidenceStatusField: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(localization.text("Status"))
                .font(.system(size: 12, weight: .semibold))
                .foregroundStyle(SMColor.secondaryText)
            Picker(localization.text("Status"), selection: $draft.status) {
                ForEach(AcceptanceOperatorEvidenceStatus.allCases) { status in
                    Text(localization.text(status.title)).tag(status)
                }
            }
            .pickerStyle(.segmented)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var artifactPathRow: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: 10) {
                artifactPathField
                artifactButtons
            }
            VStack(alignment: .leading, spacing: 10) {
                artifactPathField
                HStack(spacing: 10) {
                    artifactButtons
                    Spacer(minLength: 0)
                }
            }
        }
    }

    private var artifactPathField: some View {
        acceptanceOperatorField(
            text: $draft.artifactPath,
            placeholder: localization.text("Optional screenshot or log artifact path")
        )
    }

    @ViewBuilder
    private var artifactButtons: some View {
        CompactActionButton(localization.text("Browse operator artifact"), systemImage: "folder") {
            chooseArtifact()
        }
        if !draft.trimmedArtifactPath.isEmpty {
            CompactActionButton(localization.text("Clear operator artifact"), systemImage: "xmark") {
                clearArtifact()
            }
        }
    }
}

private extension AcceptanceManualEvidenceRecordGateStatus {
    func localizedLabel(using localization: AppChromeLocalization) -> String {
        switch state {
        case .missing:
            return localization.text("strict gate")
        case .valid:
            return localization.text("gate valid")
        case .invalid:
            return localization.text("gate invalid")
        case .optional:
            return localization.text("gate note")
        }
    }
}

private func acceptanceOperatorField(text: Binding<String>, placeholder: String) -> some View {
    TextField(placeholder, text: text)
        .textFieldStyle(.plain)
        .font(.system(size: 13, design: .monospaced))
        .padding(10)
        .background(SMColor.card)
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(SMColor.hairline))
        .frame(maxWidth: .infinity, alignment: .leading)
}

private func acceptanceOperatorLine(_ label: String, _ value: String) -> some View {
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
