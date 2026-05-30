import SwiftUI

struct AcceptanceBundlePanel: View {
    let bundlePath: String
    let snapshot: AcceptanceBundleLoadedSnapshot?
    let loadError: String?
    let browseBundle: () -> Void
    let refreshBundle: () -> Void
    let servePhase: Binding<String>
    let requireOperatorEvidence: Binding<Bool>
    let requireOperatorEvidenceLocked: Bool
    let role: WorkbenchRole
    let recordBrowse: () -> Void
    let recordAdvertise: () -> Void
    let recordServePhase: () -> Void
    let recordSourcePair: () -> Void
    let recordSourceTransfer: () -> Void
    let recordTargetImport: () -> Void
    let recordEvaluation: () -> Void
    let recordPackagingEvidence: () -> Void
    var localization: AppChromeLocalization = AppChromeLocalization(language: .english)

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 12) {
                VStack(alignment: .leading, spacing: 6) {
                    Text(localization.text("Acceptance Bundle"))
                        .font(.system(size: 12, weight: .semibold))
                        .foregroundStyle(SMColor.secondaryText)
                    Text(localization.text("Read a same-machine or two-machine installed-app evidence bundle. This is a read-only surface for packaged acceptance artifacts, not a runtime override."))
                        .font(.system(size: 12))
                        .foregroundStyle(SMColor.secondaryText)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Spacer()
                if let snapshot {
                    let panelSummary = snapshot.panelSummary(
                        requireOperatorEvidence: requireOperatorEvidence.wrappedValue
                    )
                    EvidenceChip(
                        label: localization.text("status"),
                        value: panelSummary.bundle.value,
                        tint: metricTint(panelSummary.bundle.tone)
                    )
                }
            }

            bundlePathRow

            if let loadError, !loadError.isEmpty {
                Text(loadError)
                    .font(.system(size: 12))
                    .foregroundStyle(SMColor.amber)
                    .fixedSize(horizontal: false, vertical: true)
            } else if let snapshot {
                metrics(snapshot)
                workflow(snapshot)
                evidenceFacts(snapshot)
                artifactAuthoring
                loadedArtifacts(snapshot)
            } else {
                Text(localization.text("No acceptance bundle loaded. Choose a bundle directory that contains durable `meta.json` and phase artifacts."))
                    .font(.system(size: 12))
                    .foregroundStyle(SMColor.secondaryText)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .padding(14)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(SMColor.input)
        .clipShape(RoundedRectangle(cornerRadius: 16, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous).stroke(SMColor.hairline))
    }

    private var bundlePathRow: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: 10) {
                bundlePathField
                bundlePathButtons
            }
            VStack(alignment: .leading, spacing: 10) {
                bundlePathField
                HStack(spacing: 10) {
                    bundlePathButtons
                    Spacer(minLength: 0)
                }
            }
        }
    }

    private var bundlePathField: some View {
        acceptancePathField(text: .constant(bundlePath), placeholder: localization.text("Path to acceptance bundle directory"))
    }

    @ViewBuilder
    private var bundlePathButtons: some View {
        CompactActionButton(localization.text("Browse acceptance bundle"), systemImage: "folder") {
            browseBundle()
        }
        CompactActionButton(localization.text("Refresh acceptance bundle"), systemImage: "arrow.clockwise") {
            refreshBundle()
        }
    }

    private func metrics(_ snapshot: AcceptanceBundleLoadedSnapshot) -> some View {
        let summary = snapshot.panelSummary(requireOperatorEvidence: requireOperatorEvidence.wrappedValue)
        return HStack(spacing: 12) {
            ForEach(summary.metrics) { metric in
                acceptanceMetricTile(metric.label, value: metric.value, tint: metricTint(metric.tone))
            }
        }
    }

    @ViewBuilder
    private func workflow(_ snapshot: AcceptanceBundleLoadedSnapshot) -> some View {
        let summary = snapshot.workflowSummary(requireOperatorEvidence: requireOperatorEvidence.wrappedValue)
        VStack(alignment: .leading, spacing: 10) {
            Text(localization.text("Workflow"))
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(SMColor.secondaryText)
            if summary.nextActions.isEmpty {
                evidenceLine("next action", "none")
            } else {
                ForEach(summary.nextActions) { action in
                    evidenceLine("next \(action.machine)", "\(action.step) · \(action.action)")
                    ForEach(Array(action.commands.enumerated()), id: \.offset) { index, command in
                        evidenceLine("cmd \(index + 1)", command)
                    }
                }
            }
            VStack(alignment: .leading, spacing: 6) {
                ForEach(summary.steps) { step in
                    evidenceLine(
                        "\(step.machine) \(step.id)",
                        "\(step.done ? "done" : "pending") · \(step.description)"
                    )
                }
            }
        }
    }

    @ViewBuilder
    private func evidenceFacts(_ snapshot: AcceptanceBundleLoadedSnapshot) -> some View {
        let manualEvidenceGate = snapshot.manualEvidenceGate(
            requireOperatorEvidence: requireOperatorEvidence.wrappedValue
        )
        VStack(alignment: .leading, spacing: 8) {
            evidenceLine("schema", snapshot.schema)
            evidenceLine("bundle path", bundlePath)
            if let sourcePair = snapshot.sourcePairArtifact {
                evidenceLine("pairing receipt", sourcePair.pairing_receipt_id)
                evidenceLine("target address", sourcePair.target_address)
                evidenceLine("source pair artifact", snapshot.meta.evidence.source_pair?.output ?? "source.pair.json")
            }
            if let transfer = snapshot.sourceTransferArtifact {
                evidenceLine("session", transfer.session_id)
                evidenceLine("receiver", transfer.receiver_address)
                evidenceLine("source transfer artifact", snapshot.meta.evidence.source_transfer?.output ?? "source.transfer.json")
            }
            if let consistency = snapshot.sourceConsistencyArtifact {
                evidenceLine("source consistency", "\(consistency.status) · \(consistency.mode)")
                if let baseline = consistency.baseline, !baseline.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    evidenceLine("consistency baseline", baseline)
                }
                if let sessionID = consistency.session_id, !sessionID.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    evidenceLine("consistency session", sessionID)
                }
                if let mismatchCount = consistency.mismatch_count {
                    evidenceLine("consistency mismatches", "\(mismatchCount)")
                }
                if let detail = consistency.detail, !detail.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    evidenceLine("consistency detail", detail)
                }
            }
            if let evaluation = snapshot.evaluationArtifact {
                evidenceLine("evaluation", snapshot.meta.evidence.evaluation?.output ?? "evaluation.json")
                evidenceLine("target root", evaluation.target_root)
                evidenceLine("manual gate", manualEvidenceGate.isRequired ? "required" : "not required")
            }
            if let phaseOne = snapshot.targetServePhaseArtifacts.first(where: { $0.phase == 1 }) {
                evidenceLine("serve phase 1", phaseOne.path)
            }
            if let phaseTwo = snapshot.targetServePhaseArtifacts.first(where: { $0.phase == 2 }) {
                evidenceLine("serve phase 2", phaseTwo.path)
            }
            if let advertise = snapshot.targetAdvertiseSnapshot {
                evidenceLine("advertise artifact", snapshot.meta.targetAdvertise?.output ?? "target.advertise.json")
                evidenceLine("advertise trusted", String(advertise.trusted))
                evidenceLine("advertise destination", advertise.destination)
            }
            if let browse = snapshot.sourceBrowseSnapshot {
                evidenceLine("browse artifact", snapshot.meta.sourceBrowse?.output ?? "source.browse.json")
                evidenceLine("browse trusted", String(browse.trusted))
                evidenceLine("browse candidates", "\(browse.candidate_count)")
            }
            if let targetAudit = snapshot.targetAppAuditArtifact {
                evidenceLine("target audit", "\(targetAudit.readiness ?? "unknown") via \(snapshot.meta.targetAppAudit?.output ?? "target.app-audit.json")")
            }
            if let targetNotarization = snapshot.targetNotarization {
                evidenceLine("target notarization", "\(targetNotarization.status) via \(targetNotarization.output)")
                if let readiness = targetNotarization.audit_readiness, !readiness.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    evidenceLine("target notarization audit", readiness)
                }
            }
            if let sourceAudit = snapshot.sourceAppAuditArtifact {
                evidenceLine("source audit", "\(sourceAudit.readiness ?? "unknown") via \(snapshot.meta.sourceAppAudit?.output ?? "source.app-audit.json")")
            }
            if let sourceNotarization = snapshot.sourceNotarization {
                evidenceLine("source notarization", "\(sourceNotarization.status) via \(sourceNotarization.output)")
                if let readiness = sourceNotarization.audit_readiness, !readiness.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                    evidenceLine("source notarization audit", readiness)
                }
            }
            if !snapshot.operatorEvidence.isEmpty {
                ForEach(snapshot.operatorEvidence.keys.sorted(), id: \.self) { key in
                    if let record = snapshot.operatorEvidence[key] {
                        evidenceLine("operator \(key)", "\(record.status) · \(record.detail)")
                    }
                }
            }
            if !snapshot.bundleHandoffs.isEmpty {
                evidenceLine("bundle handoffs", "\(snapshot.bundleHandoffs.count)")
                if let latestHandoff = snapshot.bundleHandoffs.last {
                    evidenceLine("latest handoff", "\(latestHandoff.archive) · \(latestHandoff.verified ? "verified" : "unverified")")
                    evidenceLine("latest manifest", latestHandoff.manifest)
                }
            }
        }
    }

    @ViewBuilder
    private func loadedArtifacts(_ snapshot: AcceptanceBundleLoadedSnapshot) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            if !snapshot.targetServePhaseArtifacts.isEmpty {
                Text(localization.text("Serve Phases"))
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(SMColor.secondaryText)
                ForEach(snapshot.targetServePhaseArtifacts) { phase in
                    VStack(alignment: .leading, spacing: 4) {
                        evidenceLine("phase", "\(phase.phase)")
                        evidenceLine("mode", phase.readiness.mode)
                        evidenceLine("address", phase.readiness.address)
                        if let verification = phase.readiness.verification_code, !verification.isEmpty {
                            evidenceLine("verification", verification)
                        }
                        if let receiver = phase.readiness.receiver_address, !receiver.isEmpty {
                            evidenceLine("receiver", receiver)
                        }
                    }
                    .padding(10)
                    .background(SMColor.card)
                    .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
                    .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).stroke(SMColor.hairline))
                }
            }
            if !snapshot.issues.isEmpty {
                Text(localization.text("Artifact Issues"))
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(SMColor.secondaryText)
                ForEach(snapshot.issues) { issue in
                    evidenceLine(issue.artifact, issue.problem)
                }
            }
            let manualEvidenceGate = snapshot.manualEvidenceGate(
                requireOperatorEvidence: requireOperatorEvidence.wrappedValue
            )
            if !manualEvidenceGate.missingEvidence.isEmpty {
                Text(localization.text("Missing Manual Evidence"))
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(SMColor.secondaryText)
                ForEach(manualEvidenceGate.missingEvidence, id: \.self) { key in
                    evidenceLine(key, "required pass evidence not recorded")
                }
            }
            if !snapshot.bundleHandoffs.isEmpty {
                Text(localization.text("Bundle Handoffs"))
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(SMColor.secondaryText)
                ForEach(Array(snapshot.bundleHandoffs.enumerated()), id: \.offset) { _, handoff in
                    VStack(alignment: .leading, spacing: 4) {
                        evidenceLine("archive", handoff.archive)
                        evidenceLine("manifest", handoff.manifest)
                        evidenceLine("sha256", handoff.sha256)
                        evidenceLine("verified", handoff.verified ? "true" : "false")
                    }
                    .padding(10)
                    .background(SMColor.card)
                    .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
                    .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).stroke(SMColor.hairline))
                }
            }
            if let sourceNotarizationArtifact = snapshot.sourceNotarizationArtifact {
                Text(localization.text("Source Notarization"))
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(SMColor.secondaryText)
                evidenceLine("status", sourceNotarizationArtifact.status)
                if let authMode = sourceNotarizationArtifact.auth_mode, !authMode.isEmpty {
                    evidenceLine("auth mode", authMode)
                }
                if let submissionID = sourceNotarizationArtifact.submission?.id, !submissionID.isEmpty {
                    evidenceLine("submission", submissionID)
                }
                if let submissionStatus = sourceNotarizationArtifact.submission?.status, !submissionStatus.isEmpty {
                    evidenceLine("submission status", submissionStatus)
                }
            }
            if let targetNotarizationArtifact = snapshot.targetNotarizationArtifact {
                Text(localization.text("Target Notarization"))
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(SMColor.secondaryText)
                evidenceLine("status", targetNotarizationArtifact.status)
                if let authMode = targetNotarizationArtifact.auth_mode, !authMode.isEmpty {
                    evidenceLine("auth mode", authMode)
                }
                if let submissionID = targetNotarizationArtifact.submission?.id, !submissionID.isEmpty {
                    evidenceLine("submission", submissionID)
                }
                if let submissionStatus = targetNotarizationArtifact.submission?.status, !submissionStatus.isEmpty {
                    evidenceLine("submission status", submissionStatus)
                }
            }
        }
    }

    private var artifactAuthoring: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(localization.text("Write Current Artifacts"))
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(SMColor.secondaryText)

            HStack(spacing: 12) {
                if role == .source {
                    ActionButton(localization.text("Write Packaging"), systemImage: "shippingbox") {
                        recordPackagingEvidence()
                    }
                    ActionButton(localization.text("Write Browse"), systemImage: "dot.radiowaves.left.and.right") {
                        recordBrowse()
                    }
                    ActionButton(localization.text("Write Pair"), systemImage: "link.badge.plus") {
                        recordSourcePair()
                    }
                    ActionButton(localization.text("Write Transfer"), systemImage: "square.and.arrow.up") {
                        recordSourceTransfer()
                    }
                }
                if role == .target {
                    ActionButton(localization.text("Write Packaging"), systemImage: "shippingbox") {
                        recordPackagingEvidence()
                    }
                    ActionButton(localization.text("Write Advertise"), systemImage: "antenna.radiowaves.left.and.right") {
                        recordAdvertise()
                    }
                    ActionButton(localization.text("Write Import"), systemImage: "square.and.arrow.down") {
                        recordTargetImport()
                    }
                    HStack(spacing: 8) {
                        TextField(localization.text("phase"), text: servePhase)
                            .textFieldStyle(.plain)
                            .font(.system(size: 12, design: .monospaced))
                            .frame(width: 52)
                            .padding(.horizontal, 10)
                            .padding(.vertical, 8)
                            .background(SMColor.card)
                            .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
                            .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(SMColor.hairline))
                        ActionButton(localization.text("Write Serve"), systemImage: "play.rectangle") {
                            recordServePhase()
                        }
                    }
                }
                if role != .source {
                    VStack(alignment: .leading, spacing: 8) {
                        Toggle(isOn: requireOperatorEvidence) {
                            Text(localization.text("Require operator evidence"))
                                .font(.system(size: 12))
                                .foregroundStyle(SMColor.primaryText)
                        }
                        .toggleStyle(.checkbox)
                        .disabled(requireOperatorEvidenceLocked)
                        if requireOperatorEvidenceLocked {
                            Text(localization.text("Two-machine installed-app bundles always evaluate in the stricter operator-evidence lane."))
                                .font(.system(size: 12))
                                .foregroundStyle(SMColor.secondaryText)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                        ActionButton(localization.text("Write Evaluation"), systemImage: "checkmark.seal") {
                            recordEvaluation()
                        }
                    }
                }
            }
            Text(localization.text("These buttons write durable phase artifacts into the selected acceptance bundle from the current app snapshots. They do not replace running the underlying command or make the bundle green by themselves."))
                .font(.system(size: 12))
                .foregroundStyle(SMColor.secondaryText)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private func evidenceLine(_ label: String, _ value: String) -> some View {
        HStack(alignment: .top, spacing: 10) {
            Text(label.uppercased())
                .font(.system(size: 10, weight: .semibold))
                .foregroundStyle(SMColor.secondaryText)
                .frame(width: 120, alignment: .leading)
            Text(value)
                .font(.system(size: 12, design: .monospaced))
                .foregroundStyle(SMColor.primaryText)
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
    }
}

private func acceptancePathField(text: Binding<String>, placeholder: String) -> some View {
    TextField(placeholder, text: text)
        .textFieldStyle(.plain)
        .font(.system(size: 13, design: .monospaced))
        .padding(10)
        .background(SMColor.card)
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 10, style: .continuous).stroke(SMColor.hairline))
        .frame(maxWidth: .infinity, alignment: .leading)
}

private func metricTint(_ tone: AcceptanceBundlePanelSummary.Metric.Tone) -> Color {
    switch tone {
    case .positive:
        return SMColor.green
    case .caution:
        return SMColor.amber
    case .informative:
        return SMColor.blue
    }
}

private func acceptanceMetricTile(_ label: String, value: String, tint: Color) -> some View {
    VStack(alignment: .leading, spacing: 6) {
        Text(label.uppercased())
            .font(.system(size: 10, weight: .semibold))
            .foregroundStyle(SMColor.secondaryText)
        Text(value)
            .font(.system(size: 14, weight: .semibold, design: .monospaced))
            .foregroundStyle(tint)
            .lineLimit(1)
            .minimumScaleFactor(0.8)
    }
    .padding(12)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(SMColor.card)
    .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
    .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).stroke(SMColor.hairline))
}
