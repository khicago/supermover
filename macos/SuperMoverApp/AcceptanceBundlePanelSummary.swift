import Foundation

struct AcceptanceBundlePanelSummary: Equatable {
    struct Metric: Equatable, Identifiable {
        enum Tone: Equatable {
            case positive
            case caution
            case informative
        }

        let label: String
        let value: String
        let tone: Tone

        var id: String { label }
    }

    let bundle: Metric
    let release: Metric
    let proof: Metric
    let `operator`: Metric

    var metrics: [Metric] {
        [bundle, release, proof, `operator`]
    }
}

extension AcceptanceBundleLoadedSnapshot {
    func panelSummary(requireOperatorEvidence: Bool) -> AcceptanceBundlePanelSummary {
        let workflow = workflowSummary(requireOperatorEvidence: requireOperatorEvidence)
        let releaseEvidence = installedAppReleaseEvidence
        let collectionProof = installedAppCollectionProof

        return AcceptanceBundlePanelSummary(
            bundle: bundleMetric(
                requireOperatorEvidence: requireOperatorEvidence,
                workflow: workflow
            ),
            release: releaseMetric(releaseEvidence),
            proof: proofMetric(collectionProof),
            operator: operatorMetric(requireOperatorEvidence: requireOperatorEvidence)
        )
    }

    private func bundleMetric(
        requireOperatorEvidence: Bool,
        workflow: (steps: [WorkflowStep], nextActions: [WorkflowAction])
    ) -> AcceptanceBundlePanelSummary.Metric {
        if hasCurrentEvaluationEvidence(requireOperatorEvidence: requireOperatorEvidence)
            && workflow.nextActions.isEmpty {
            return .init(label: "bundle", value: status, tone: .positive)
        }
        if isCollected {
            return .init(label: "bundle", value: "review", tone: .caution)
        }
        let normalizedStatus = status.trimmingCharacters(in: .whitespacesAndNewlines)
        return .init(
            label: "bundle",
            value: normalizedStatus.isEmpty ? "missing" : normalizedStatus,
            tone: .caution
        )
    }

    private func releaseMetric(
        _ releaseEvidence: AcceptanceInstalledAppReleaseEvidenceSummary
    ) -> AcceptanceBundlePanelSummary.Metric {
        .init(
            label: "release",
            value: releaseEvidence.ok ? "ready" : "review",
            tone: releaseEvidence.ok ? .positive : .caution
        )
    }

    private func proofMetric(
        _ collectionProof: AcceptanceInstalledAppCollectionProofSummary
    ) -> AcceptanceBundlePanelSummary.Metric {
        let value: String
        if collectionProof.ok {
            value = "proved"
        } else if collectionProof.blockedReason != nil {
            value = "blocked"
        } else if collectionProof.requiresBundleHandoffProof {
            value = "handoff"
        } else {
            value = "missing"
        }
        return .init(
            label: "proof",
            value: value,
            tone: collectionProof.ok ? .positive : .caution
        )
    }

    private func operatorMetric(
        requireOperatorEvidence: Bool
    ) -> AcceptanceBundlePanelSummary.Metric {
        let gate = manualEvidenceGate(requireOperatorEvidence: requireOperatorEvidence)
        if !gate.isRequired {
            return .init(label: "operator", value: "n/a", tone: .informative)
        }
        let ready = gate.missingEvidence.isEmpty
        return .init(
            label: "operator",
            value: ready ? "pass" : "missing",
            tone: ready ? .positive : .caution
        )
    }
}
