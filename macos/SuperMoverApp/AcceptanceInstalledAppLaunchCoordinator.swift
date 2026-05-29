import Foundation

// Owns the app-side installed-app launch flow so AppStore can delegate to one
// proof-aware module instead of restating gate wiring across store methods.
struct AcceptanceInstalledAppLaunchCoordinator {
    private enum CurrentEvaluationRepair {
        case review(detail: String)
        case blocked(detail: String)
    }

    struct Context {
        let selectedRole: WorkbenchRole
        let bundlePath: String
        let loadedSnapshot: AcceptanceBundleLoadedSnapshot?
        let cliProvenance: CLIProvenance
    }

    struct Dependencies {
        let currentContext: () -> Context
        let currentBundleURL: () throws -> URL
        let refreshBundle: () -> Void
        let acceptanceBundleOperations: AcceptanceBundleAppOperations
    }

    private struct Evaluation {
        let bundleContext: AcceptanceInstalledAppLaunchBundleContext
        let input: AcceptanceInstalledAppLaunchGate.Input
        let verdict: AcceptanceInstalledAppLaunchGate.Verdict

        var releaseEvidence: AcceptanceInstalledAppReleaseEvidenceSummary? {
            bundleContext.installedAppReleaseEvidence
        }
    }

    func preview(
        for kind: SuperMoverTaskKind,
        using dependencies: Dependencies
    ) -> AcceptanceInstalledAppLaunchPreview? {
        guard let evaluation = evaluate(for: kind, context: dependencies.currentContext()) else {
            return nil
        }
        let machineIdentityCorrectionDetail = machineIdentityCorrectionLaunchDetail(
            for: kind,
            evaluation: evaluation
        )
        switch evaluation.verdict {
        case .invalidAcceptanceBundle, .requiresPackagedApp:
            return evaluation.verdict.preview()
        default:
            break
        }
        if let blockedDetail = installedAppProofCannotBeRepairedByLaunchDetail(
            evaluation.verdict,
            proof: evaluation.input.installedAppCollectionProof,
            machineIdentityCorrectionDetail: machineIdentityCorrectionDetail
        ) {
            return AcceptanceInstalledAppLaunchPreview(
                machine: evaluation.input.machine,
                state: .blocked,
                detail: blockedDetail
            )
        }
        let packagingProbe = currentAppPackagingProbe(
            machine: evaluation.input.machine,
            cliProvenance: evaluation.input.cliProvenance,
            using: dependencies
        )
        if let blockedDetail = packagingProbe.blockedDetail(machine: evaluation.input.machine) {
            return AcceptanceInstalledAppLaunchPreview(
                machine: evaluation.input.machine,
                state: .blocked,
                detail: blockedDetail
            )
        }
        let localNotarizationBlockingDetail = packagingProbe.localNotarizationBlockingDetail(
            machine: evaluation.input.machine
        )
        if let localNotarizationBlockingDetail {
            return AcceptanceInstalledAppLaunchPreview(
                machine: evaluation.input.machine,
                state: .blocked,
                detail: localNotarizationBlockingDetail
            )
        }
        if let releaseEvidenceBlockingDetail = otherMachineReleaseEvidenceBlockingDetail(for: evaluation) {
            return AcceptanceInstalledAppLaunchPreview(
                machine: evaluation.input.machine,
                state: .blocked,
                detail: releaseEvidenceBlockingDetail
            )
        }
        if let machineIdentityCorrectionDetail {
            return AcceptanceInstalledAppLaunchPreview(
                machine: evaluation.input.machine,
                state: .review,
                detail: machineIdentityCorrectionDetail
            )
        }
        let currentEvaluationRepair = currentEvaluationRepair(for: kind, evaluation: evaluation)
        if case let .blocked(detail)? = currentEvaluationRepair {
            return AcceptanceInstalledAppLaunchPreview(
                machine: evaluation.input.machine,
                state: .blocked,
                detail: detail
            )
        }
        if case let .review(detail)? = currentEvaluationRepair {
            return AcceptanceInstalledAppLaunchPreview(
                machine: evaluation.input.machine,
                state: .review,
                detail: detail
            )
        }
        return evaluation.verdict.preview(localNotarizationBlockingDetail: localNotarizationBlockingDetail)
    }

    func preflightError(
        for kind: SuperMoverTaskKind,
        using dependencies: Dependencies
    ) -> String? {
        guard let initialEvaluation = evaluate(for: kind, context: dependencies.currentContext()) else {
            return nil
        }
        let machineIdentityCorrectionDetail = machineIdentityCorrectionLaunchDetail(
            for: kind,
            evaluation: initialEvaluation
        )
        switch initialEvaluation.verdict {
        case .invalidAcceptanceBundle, .requiresPackagedApp:
            return initialEvaluation.verdict.preflightError
        default:
            break
        }
        if let blockedDetail = installedAppProofCannotBeRepairedByLaunchDetail(
            initialEvaluation.verdict,
            proof: initialEvaluation.input.installedAppCollectionProof,
            machineIdentityCorrectionDetail: machineIdentityCorrectionDetail
        ) {
            return blockedDetail
        }

        let packagingProbe = currentAppPackagingProbe(
            machine: initialEvaluation.input.machine,
            cliProvenance: initialEvaluation.input.cliProvenance,
            using: dependencies
        )
        if let packagingWriteBlockedDetail = packagingProbe.blockedDetail(machine: initialEvaluation.input.machine) {
            return packagingWriteBlockedDetail
        }
        let hasLocalNotarizationBlockingDetail = packagingProbe.localNotarizationBlockingDetail(
            machine: initialEvaluation.input.machine
        ) != nil
        if !hasLocalNotarizationBlockingDetail {
            if let releaseEvidenceBlockingDetail = otherMachineReleaseEvidenceBlockingDetail(
                for: initialEvaluation
            ) {
                return releaseEvidenceBlockingDetail
            }
        }

        let initialCurrentEvaluationRepair = currentEvaluationRepair(
            for: kind,
            evaluation: initialEvaluation
        )
        if case let .blocked(detail)? = initialCurrentEvaluationRepair,
           !hasLocalNotarizationBlockingDetail {
            return detail
        }

        let currentContext = dependencies.currentContext()
        do {
            _ = try dependencies.acceptanceBundleOperations.recordPackagingEvidence(
                bundleRootURL: try dependencies.currentBundleURL(),
                machine: initialEvaluation.input.machine,
                collectedBy: "app-preflight-\(currentContext.selectedRole.rawValue)"
            )
            dependencies.refreshBundle()
        } catch {
            dependencies.refreshBundle()
            return "Real two-machine installed-app acceptance tasks could not record \(initialEvaluation.input.machine) packaging evidence before phase execution: \(error.localizedDescription)"
        }

        guard let refreshedEvaluation = evaluate(for: kind, context: dependencies.currentContext()) else {
            return nil
        }
        if let releaseEvidencePreflightError = currentMachineReleaseEvidencePreflightError(
            for: refreshedEvaluation.verdict
        ) {
            return releaseEvidencePreflightError
        }
        if let releaseEvidenceBlockingDetail = otherMachineReleaseEvidenceBlockingDetail(
            for: refreshedEvaluation
        ) {
            return releaseEvidenceBlockingDetail
        }
        if machineIdentityCorrectionLaunchDetail(for: kind, evaluation: refreshedEvaluation) != nil {
            return nil
        }
        if case let .blocked(detail)? = currentEvaluationRepair(for: kind, evaluation: refreshedEvaluation) {
            return detail
        }
        if case .review = currentEvaluationRepair(for: kind, evaluation: refreshedEvaluation) {
            return nil
        }
        switch refreshedEvaluation.verdict {
        case .ready:
            return nil
        default:
            return refreshedEvaluation.verdict.preflightError
        }
    }

    private func installedAppProofCannotBeRepairedByLaunchDetail(
        _ verdict: AcceptanceInstalledAppLaunchGate.Verdict,
        proof: AcceptanceInstalledAppCollectionProofSummary?,
        machineIdentityCorrectionDetail: String?
    ) -> String? {
        guard machineIdentityCorrectionDetail == nil else {
            return nil
        }
        switch verdict {
        case .installedAppProofBlocked:
            return verdict.preflightError
        case .installedAppProofIncomplete where proof?.requiresMachineIdentityCorrection == true:
            return "Current acceptance bundle needs installed-app machine identity correction before this task can run. Launch Source Pair or Target Serve to rewrite source.machine.json / target.machine.json evidence from the installed apps; unrelated tasks remain blocked until that machine identity evidence is current."
        default:
            return nil
        }
    }

    private func currentMachineReleaseEvidencePreflightError(
        for verdict: AcceptanceInstalledAppLaunchGate.Verdict
    ) -> String? {
        switch verdict {
        case .missingPackagingAudit,
             .packagingAuditNotReady,
             .missingNotarization,
             .notarizationNotReady:
            return verdict.preflightError
        default:
            return nil
        }
    }

    private func machineIdentityCorrectionLaunchDetail(
        for kind: SuperMoverTaskKind,
        evaluation: Evaluation
    ) -> String? {
        switch evaluation.verdict {
        case .installedAppProofBlocked, .installedAppProofIncomplete:
            break
        default:
            return nil
        }
        guard let proof = evaluation.input.installedAppCollectionProof,
              let correctionLaunch = proof.machineIdentityCorrectionLaunch(for: kind),
              correctionLaunch.machine == evaluation.input.machine else {
            return nil
        }
        return correctionLaunch.detail
    }

    private func evaluate(
        for kind: SuperMoverTaskKind,
        context: Context
    ) -> Evaluation? {
        let bundleContext = AcceptanceInstalledAppLaunchBundleContext.resolve(
            bundlePath: context.bundlePath,
            loadedSnapshot: context.loadedSnapshot
        )
        guard let input = gateInput(for: kind, context: context, bundleContext: bundleContext),
              let verdict = AcceptanceInstalledAppLaunchGate.evaluate(input) else {
            return nil
        }
        return Evaluation(
            bundleContext: bundleContext,
            input: input,
            verdict: verdict
        )
    }

    private func currentAppPackagingProbe(
        machine: String,
        cliProvenance: CLIProvenance,
        using dependencies: Dependencies
    ) -> AcceptanceBundleAppOperations.CurrentAppPackagingProbe {
        do {
            return dependencies.acceptanceBundleOperations.currentAppPackagingProbe(
                bundleRootURL: try dependencies.currentBundleURL(),
                machine: machine,
                cliProvenance: cliProvenance
            )
        } catch {
            return .blocked(.unexpected(error.localizedDescription))
        }
    }

    private func gateInput(
        for kind: SuperMoverTaskKind,
        context: Context,
        bundleContext: AcceptanceInstalledAppLaunchBundleContext
    ) -> AcceptanceInstalledAppLaunchGate.Input? {
        guard let machine = machine(for: kind) else {
            return nil
        }
        let releaseEvidenceMachine = bundleContext.snapshot.map {
            let summary = $0.installedAppReleaseEvidence
            return machine == "source" ? summary.source : summary.target
        }
        return AcceptanceInstalledAppLaunchGate.Input(
            machine: machine,
            bundleState: bundleContext.state,
            cliProvenance: context.cliProvenance,
            installedAppCollectionProof: bundleContext.installedAppCollectionProof,
            releaseEvidenceMachine: releaseEvidenceMachine,
            hasCurrentEvaluationPassState: bundleContext.snapshot?.hasCurrentEvaluationPassState(
                requireOperatorEvidence: true
            ) ?? false
        )
    }

    private func currentEvaluationRepair(
        for kind: SuperMoverTaskKind,
        evaluation: Evaluation
    ) -> CurrentEvaluationRepair? {
        guard case .finalEvaluationPending = evaluation.verdict,
              let snapshot = evaluation.bundleContext.snapshot else {
            return nil
        }

        let nextActions = snapshot.workflowSummary(requireOperatorEvidence: true).nextActions
        guard !nextActions.isEmpty else {
            return nil
        }
        if nextActions.count == 1, nextActions[0].step == "evaluate" {
            return nil
        }

        if nextActions.count == 1,
           let currentStep = workflowStep(for: kind),
           let matchingAction = nextActions.first(where: {
               $0.step == currentStep && ($0.machine == evaluation.input.machine || $0.machine == "either")
           }) {
            return .review(
                detail: "Current acceptance bundle no longer satisfies strict final evaluation (\(matchingAction.action)). Launch can refresh this step from the current installed app, but final evaluate remains blocked until the merged bundle is re-evaluated."
            )
        }

        let nextAction = nextActions[0]
        return .blocked(
            detail: "Real two-machine installed-app acceptance tasks require current strict bundle proof before phase execution. Current acceptance bundle now needs \(nextAction.action)."
        )
    }

    private func otherMachineReleaseEvidenceBlockingDetail(
        for evaluation: Evaluation
    ) -> String? {
        switch evaluation.verdict {
        case .ready, .finalEvaluationPending, .installedAppProofIncomplete, .installedAppProofBlocked:
            break
        default:
            return nil
        }
        guard let releaseEvidence = evaluation.releaseEvidence else {
            return nil
        }
        let missingMachines = releaseEvidence.missingMachines.filter { $0 != evaluation.input.machine }
        guard !missingMachines.isEmpty else {
            return nil
        }

        let failureDetail = missingMachines.compactMap { machine in
            releaseEvidenceMachine(named: machine, from: releaseEvidence)?
                .failures
                .joined(separator: "; ")
        }
        .filter { !$0.isEmpty }
        .joined(separator: " ")

        let machineList = humanReadableMachineList(missingMachines)
        if failureDetail.isEmpty {
            return "Real two-machine installed-app acceptance tasks require release-ready \(machineList) packaging evidence before phase execution. Record \(machineList) release packaging evidence before launching this \(evaluation.input.machine) task."
        }
        return "Real two-machine installed-app acceptance tasks require release-ready \(machineList) packaging evidence before phase execution. Current acceptance bundle still needs \(failureDetail)."
    }

    private func releaseEvidenceMachine(
        named machine: String,
        from releaseEvidence: AcceptanceInstalledAppReleaseEvidenceSummary
    ) -> AcceptanceInstalledAppReleaseEvidenceSummary.Machine? {
        switch machine {
        case "source":
            return releaseEvidence.source
        case "target":
            return releaseEvidence.target
        default:
            return nil
        }
    }

    private func humanReadableMachineList(_ machines: [String]) -> String {
        switch machines.count {
        case 0:
            return "source and target"
        case 1:
            return machines[0]
        case 2:
            return "\(machines[0]) and \(machines[1])"
        default:
            return machines.dropLast().joined(separator: ", ") + ", and " + (machines.last ?? "")
        }
    }

    private func machine(for kind: SuperMoverTaskKind) -> String? {
        switch kind {
        case .discoverBrowse, .pair, .networkDryRun, .networkPush:
            return "source"
        case .discoverAdvertise, .serve, .profileAdoptPairing:
            return "target"
        default:
            return nil
        }
    }

    private func workflowStep(for kind: SuperMoverTaskKind) -> String? {
        switch kind {
        case .discoverBrowse:
            return "source_browse"
        case .discoverAdvertise:
            return "target_advertise"
        case .pair:
            return "source_pair"
        case .profileAdoptPairing:
            return "target_import"
        case .networkPush:
            return "source_transfer"
        case .serve:
            return "target_serve_phase_1"
        default:
            return nil
        }
    }
}
