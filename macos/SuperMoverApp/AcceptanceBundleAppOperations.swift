import Foundation

struct AcceptanceBundleAppOperations {
    enum CurrentAppPackagingBlockReason: Equatable {
        case collection(AcceptancePackagingEvidenceCollector.CollectionError)
        case currentAppAuditNotInstallReady(String)
        case unexpected(String)

        func detail(machine: String) -> String {
            let prefix = "Real two-machine installed-app acceptance tasks could not record \(machine) packaging evidence before phase execution:"
            switch self {
            case let .collection(error):
                return "\(prefix) \(error.localizedDescription)"
            case let .currentAppAuditNotInstallReady(detail):
                return "\(prefix) \(detail)"
            case let .unexpected(detail):
                return "\(prefix) \(detail)"
            }
        }
    }

    enum CurrentAppPackagingProbe: Equatable {
        case skipped
        case blocked(CurrentAppPackagingBlockReason)
        case recordable(localNotarization: AcceptancePackagingEvidenceCollector.CurrentAppNotarizationState)

        func blockedDetail(machine: String) -> String? {
            guard case let .blocked(reason) = self else {
                return nil
            }
            return reason.detail(machine: machine)
        }

        func localNotarizationBlockingDetail(machine: String) -> String? {
            guard case let .recordable(localNotarization) = self else {
                return nil
            }
            switch localNotarization {
            case .missing:
                return "Loaded \(machine) packaging audit is accepted, but local \(machine) notarization evidence is missing and launch will fail closed until it is replaced."
            case let .notReady(status):
                return "Loaded \(machine) packaging audit is accepted, but local \(machine) notarization evidence is \(status) and not release-ready; launch will fail closed until it is replaced."
            case .ready:
                return nil
            }
        }
    }

    let resourceURLProvider: () -> URL?
    let packagingCollectorFactory: () -> AcceptancePackagingEvidenceCollector
    let evaluationCoordinatorFactory: () -> AcceptanceBundleEvaluationCoordinator

    init(
        resourceURLProvider: @escaping () -> URL? = { Bundle.main.resourceURL },
        packagingCollectorFactory: @escaping () -> AcceptancePackagingEvidenceCollector = {
            AcceptancePackagingEvidenceCollector()
        },
        evaluationCoordinatorFactory: @escaping () -> AcceptanceBundleEvaluationCoordinator = {
            AcceptanceBundleEvaluationCoordinator()
        }
    ) {
        self.resourceURLProvider = resourceURLProvider
        self.packagingCollectorFactory = packagingCollectorFactory
        self.evaluationCoordinatorFactory = evaluationCoordinatorFactory
    }

    func recordPackagingEvidence(
        bundleRootURL: URL,
        machine: String,
        collectedBy: String
    ) throws -> AcceptanceBundleArtifactAuthoringResult {
        let outputs = try packagingCollectorFactory().recordCurrentMachineEvidence(
            bundleRootURL: bundleRootURL,
            machine: machine,
            collectedBy: collectedBy,
            resourceURL: resourceURLProvider()
        )
        return .init(
            kind: .packagingEvidence,
            detail: "\(machine) packaging evidence -> \(outputs.joined(separator: ", "))"
        )
    }

    func currentAppPackagingProbe(
        bundleRootURL: URL,
        machine: String,
        cliProvenance: CLIProvenance
    ) -> CurrentAppPackagingProbe {
        guard let resourceURL = currentPackagedResourceURL(matching: cliProvenance) else {
            return .skipped
        }
        do {
            let inspection = try packagingCollectorFactory().inspectCurrentMachineEvidence(
                bundleRootURL: bundleRootURL,
                machine: machine,
                resourceURL: resourceURL
            )
            guard inspection.audit.installReady else {
                return .blocked(
                    .currentAppAuditNotInstallReady(
                        inspection.audit.failureMessage
                            ?? "Local app audit for the current packaged app is not install-ready."
                    )
                )
            }
            return .recordable(localNotarization: inspection.notarization)
        } catch let error as AcceptancePackagingEvidenceCollector.CollectionError {
            return .blocked(.collection(error))
        } catch {
            return .blocked(.unexpected(error.localizedDescription))
        }
    }

    func recordEvaluation(
        bundleRootURL: URL,
        targetRootURL: URL,
        requireOperatorEvidence: Bool
    ) throws -> AcceptanceBundleArtifactAuthoringResult {
        try evaluationCoordinatorFactory().evaluate(
            bundleRootURL: bundleRootURL,
            targetRootURL: targetRootURL,
            requireOperatorEvidence: requireOperatorEvidence
        )
    }

    private func appBundleURL(for resourceURL: URL) throws -> URL {
        let contentsURL = resourceURL.deletingLastPathComponent()
        let appURL = contentsURL.deletingLastPathComponent()
        guard appURL.lastPathComponent.hasSuffix(".app") else {
            throw SuperMoverCLIError.bundledBinaryMissing(appURL.path)
        }
        return appURL
    }

    private func appBundleURL(fromExecutablePath executablePath: String) -> URL? {
        let trimmed = executablePath.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            return nil
        }
        let executableURL = URL(fileURLWithPath: trimmed)
        let appURL = executableURL
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        guard appURL.lastPathComponent.hasSuffix(".app") else {
            return nil
        }
        return appURL
    }

    private func currentPackagedResourceURL(matching cliProvenance: CLIProvenance) -> URL? {
        guard let resourceURL = resourceURLProvider(),
              let resourceBundleURL = try? appBundleURL(for: resourceURL),
              let executableBundleURL = appBundleURL(fromExecutablePath: cliProvenance.executablePath) else {
            return nil
        }
        guard resourceBundleURL.standardizedFileURL.path == executableBundleURL.standardizedFileURL.path else {
            return nil
        }
        return resourceURL
    }
}
