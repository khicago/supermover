import Foundation

struct AcceptanceBundleReader {
    enum ReadError: LocalizedError, Equatable {
        case invalidBundleRoot(URL)
        case missingMeta(URL)
        case malformedMeta(URL)
        case symlinkRejected(URL)

        var errorDescription: String? {
            switch self {
            case let .invalidBundleRoot(url):
                return "Acceptance bundle root is not a directory: \(url.path)"
            case let .missingMeta(url):
                return "Acceptance bundle is missing meta.json at \(url.path)."
            case let .malformedMeta(url):
                return "Acceptance bundle meta.json is malformed at \(url.path)."
            case let .symlinkRejected(url):
                return "Acceptance bundle read rejected symlink path: \(url.path)"
            }
        }
    }

    let artifactAccess: AcceptanceBundleArtifactAccess

    init(fileManager: FileManager = .default) {
        artifactAccess = AcceptanceBundleArtifactAccess(fileManager: fileManager)
    }

    func read(bundleRootURL: URL) throws -> AcceptanceBundleSnapshot {
        let metaURL = try validatedMetaURL(bundleRootURL)
        let data: Data
        do {
            data = try Data(contentsOf: metaURL)
        } catch {
            throw ReadError.malformedMeta(metaURL)
        }
        guard let decoded = try? JSONDecoder().decode(AcceptanceBundleSnapshot.self, from: data) else {
            throw ReadError.malformedMeta(metaURL)
        }
        return decoded
    }

    func load(bundleRootURL: URL) throws -> AcceptanceBundleLoadedSnapshot {
        let meta = try read(bundleRootURL: bundleRootURL)
        var issues: [AcceptanceBundleArtifactIssue] = []

        let sourceProvenance = decodeIfPresent(
            AcceptanceBundleSnapshot.ProvenanceArtifact.self,
            relativePath: "source.provenance.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let targetProvenance = decodeIfPresent(
            AcceptanceBundleSnapshot.ProvenanceArtifact.self,
            relativePath: "target.provenance.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let sourceBrowse = decodeOptional(
            DiscoveryBrowseSnapshot.self,
            relativePath: meta.sourceBrowse?.output,
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let sourceAppAudit = decodeIfPresent(
            AcceptanceBundleSnapshot.AppAuditArtifact.self,
            relativePath: "source.app-audit.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let targetAppAudit = decodeIfPresent(
            AcceptanceBundleSnapshot.AppAuditArtifact.self,
            relativePath: "target.app-audit.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let sourceNotarization = decodeIfPresent(
            AcceptanceBundleSnapshot.NotarizationArtifact.self,
            relativePath: "source.notarization.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let targetNotarization = decodeIfPresent(
            AcceptanceBundleSnapshot.NotarizationArtifact.self,
            relativePath: "target.notarization.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let sourceMachineFacts = decodeIfPresent(
            AcceptanceBundleSnapshot.MachineFactsArtifact.self,
            relativePath: AcceptanceInstalledAppCollectionProofConstants.sourceMachineFactsArtifact,
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let targetMachineFacts = decodeIfPresent(
            AcceptanceBundleSnapshot.MachineFactsArtifact.self,
            relativePath: AcceptanceInstalledAppCollectionProofConstants.targetMachineFactsArtifact,
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let workflowSummary = decodeOptional(
            AcceptanceBundleSnapshot.WorkflowSummaryArtifact.self,
            relativePath: meta.evidence.workflow_summary?.output,
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let targetAdvertise = decodeOptional(
            DiscoveryAdvertiseSnapshot.self,
            relativePath: meta.targetAdvertise?.output,
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let targetReady = decodeOptional(
            ServeReadinessSnapshot.self,
            relativePath: meta.evidence.target_ready == nil ? nil : "target.ready.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let sourcePair = decodeOptionalWithFallback(
            AcceptanceSourcePairArtifact.self,
            explicitRelativePath: meta.evidence.source_pair?.output,
            fallbackRelativePath: "source.pair.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let hasSourcePairReceiptArtifact = validatedArtifactExists(
            relativePath: sourcePair?.receipt_path,
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let hasValidSourcePairReceiptArtifact = validatedPairingReceiptArtifact(
            relativePath: sourcePair?.receipt_path,
            expectedPairingReceiptID: sourcePair?.pairing_receipt_id,
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let hasSourcePairTranscriptArtifact = artifactExists(
            explicitRelativePath: meta.evidence.source_pair?.pair,
            fallbackRelativePath: "source.pair.txt",
            bundleRootURL: bundleRootURL
        )
        let sourceTransfer = decodeOptionalWithFallback(
            AcceptanceSourceTransferArtifact.self,
            explicitRelativePath: meta.evidence.source_transfer?.output,
            fallbackRelativePath: "source.transfer.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let hasSourceNetworkPushTranscriptArtifact = artifactExists(
            explicitRelativePath: meta.evidence.source_transfer?.push,
            fallbackRelativePath: "source.network-push.txt",
            bundleRootURL: bundleRootURL
        )
        let decodedSourceConsistency = decodeOptionalWithFallback(
            AcceptanceBundleSnapshot.SourceConsistencyEvidence.self,
            explicitRelativePath: meta.evidence.source_consistency?.output,
            fallbackRelativePath: "source.consistency.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let sourceConsistency = mergedSourceConsistency(
            artifact: decodedSourceConsistency,
            meta: meta.evidence.source_consistency
        )
        let sourceConsistencyBaselinePath = sourceConsistencyBaselinePath(
            artifact: decodedSourceConsistency,
            merged: sourceConsistency
        )
        let hasSourceConsistencyBaselineArtifact: Bool
        if sourceConsistency != nil {
            hasSourceConsistencyBaselineArtifact = validatedArtifactExists(
                relativePath: sourceConsistencyBaselinePath ?? "source.baseline.json",
                bundleRootURL: bundleRootURL,
                issues: &issues
            )
        } else {
            hasSourceConsistencyBaselineArtifact = false
        }
        let sourceVerify = decodeOptionalWithFallback(
            VerifySnapshot.self,
            explicitRelativePath: meta.evidence.source_transfer?.verify,
            fallbackRelativePath: "source.verify.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let sourceStatus = decodeOptionalWithFallback(
            StatusSnapshot.self,
            explicitRelativePath: meta.evidence.source_transfer?.status,
            fallbackRelativePath: "source.status.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let sourceReport = decodeOptionalWithFallback(
            ReportSnapshot.self,
            explicitRelativePath: meta.evidence.source_transfer?.report,
            fallbackRelativePath: "source.report.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let sourceHealth = decodeOptionalWithFallback(
            HealthSnapshot.self,
            explicitRelativePath: meta.evidence.source_transfer?.health,
            fallbackRelativePath: "source.health.json",
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let evaluation = decodeOptional(
            AcceptanceEvaluationArtifact.self,
            relativePath: meta.evidence.evaluation?.output,
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let hasTargetImportTranscriptArtifact = validatedArtifactExists(
            relativePath: meta.evidence.target_import?.adopted,
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
        let servePhases = meta.evidence.target_serve_phases?.compactMap { phase in
            decodeServePhase(phase, bundleRootURL: bundleRootURL, issues: &issues)
        } ?? []

        return AcceptanceBundleLoadedSnapshot(
            bundleRootPath: bundleRootURL.standardizedFileURL.path,
            meta: meta,
            sourceBrowseSnapshot: sourceBrowse,
            targetAdvertiseSnapshot: targetAdvertise,
            targetReadyArtifact: targetReady,
            sourceProvenanceArtifact: sourceProvenance,
            targetProvenanceArtifact: targetProvenance,
            sourceAppAuditArtifact: sourceAppAudit,
            targetAppAuditArtifact: targetAppAudit,
            sourceNotarizationArtifact: sourceNotarization,
            targetNotarizationArtifact: targetNotarization,
            sourceMachineFactsArtifact: sourceMachineFacts,
            targetMachineFactsArtifact: targetMachineFacts,
            workflowSummaryArtifact: workflowSummary,
            sourcePairArtifact: sourcePair,
            hasSourcePairReceiptArtifact: hasSourcePairReceiptArtifact,
            hasValidSourcePairReceiptArtifact: hasValidSourcePairReceiptArtifact,
            hasSourcePairTranscriptArtifact: hasSourcePairTranscriptArtifact,
            sourceTransferArtifact: sourceTransfer,
            hasSourceNetworkPushTranscriptArtifact: hasSourceNetworkPushTranscriptArtifact,
            sourceConsistencyArtifact: sourceConsistency,
            hasDecodedSourceConsistencyArtifact: decodedSourceConsistency != nil,
            hasSourceConsistencyBaselineArtifact: hasSourceConsistencyBaselineArtifact,
            sourceVerifyArtifact: sourceVerify,
            sourceStatusArtifact: sourceStatus,
            sourceReportArtifact: sourceReport,
            sourceHealthArtifact: sourceHealth,
            evaluationArtifact: evaluation,
            hasTargetImportTranscriptArtifact: hasTargetImportTranscriptArtifact,
            targetServePhaseArtifacts: servePhases,
            operatorEvidence: meta.operatorEvidence,
            issues: issues
        )
    }

    private func decodeServePhase(
        _ phase: AcceptanceBundleSnapshot.TargetServePhase,
        bundleRootURL: URL,
        issues: inout [AcceptanceBundleArtifactIssue]
    ) -> AcceptanceServePhaseArtifact? {
        guard let readiness = decodeOptional(
            ServeReadinessSnapshot.self,
            relativePath: phase.ready,
            bundleRootURL: bundleRootURL,
            issues: &issues
        ) else {
            return nil
        }
        return AcceptanceServePhaseArtifact(
            phase: phase.phase,
            path: phase.ready,
            readiness: readiness
        )
    }

    private func decodeOptional<T: Decodable>(
        _ type: T.Type,
        relativePath: String?,
        bundleRootURL: URL,
        issues: inout [AcceptanceBundleArtifactIssue]
    ) -> T? {
        let trimmed = relativePath?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !trimmed.isEmpty else {
            return nil
        }
        do {
            guard let data = try artifactAccess.artifactDataIfPresent(
                relativePath: trimmed,
                bundleRootURL: bundleRootURL
            ) else {
                issues.append(
                    AcceptanceBundleArtifactIssue(
                        artifact: trimmed,
                        problem: "missing or unreadable artifact"
                    )
                )
                return nil
            }
            guard let decoded = try? JSONDecoder().decode(T.self, from: data) else {
                issues.append(
                    AcceptanceBundleArtifactIssue(
                        artifact: trimmed,
                        problem: "malformed JSON artifact"
                    )
                )
                return nil
            }
            return decoded
        } catch AcceptanceBundleArtifactAccess.AccessError.unsafeArtifactPath {
            issues.append(
                AcceptanceBundleArtifactIssue(
                    artifact: trimmed,
                    problem: "unsafe artifact path"
                )
            )
            return nil
        } catch {
            issues.append(
                AcceptanceBundleArtifactIssue(
                    artifact: trimmed,
                    problem: "missing or unreadable artifact"
                )
            )
            return nil
        }
    }

    private func decodeOptionalWithFallback<T: Decodable>(
        _ type: T.Type,
        explicitRelativePath: String?,
        fallbackRelativePath: String,
        bundleRootURL: URL,
        issues: inout [AcceptanceBundleArtifactIssue]
    ) -> T? {
        let explicit = explicitRelativePath?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if explicit.isEmpty {
            return decodeIfPresent(
                type,
                relativePath: fallbackRelativePath,
                bundleRootURL: bundleRootURL,
                issues: &issues
            )
        }
        return decodeOptional(
            type,
            relativePath: explicit,
            bundleRootURL: bundleRootURL,
            issues: &issues
        )
    }

    private func decodeIfPresent<T: Decodable>(
        _ type: T.Type,
        relativePath: String,
        bundleRootURL: URL,
        issues: inout [AcceptanceBundleArtifactIssue]
    ) -> T? {
        do {
            guard let data = try artifactAccess.artifactDataIfPresent(
                relativePath: relativePath,
                bundleRootURL: bundleRootURL
            ) else {
                return nil
            }
            guard let decoded = try? JSONDecoder().decode(T.self, from: data) else {
                issues.append(
                    AcceptanceBundleArtifactIssue(
                        artifact: relativePath,
                        problem: "malformed JSON artifact"
                    )
                )
                return nil
            }
            return decoded
        } catch AcceptanceBundleArtifactAccess.AccessError.unsafeArtifactPath {
            issues.append(
                AcceptanceBundleArtifactIssue(
                    artifact: relativePath,
                    problem: "unsafe artifact path"
                )
            )
            return nil
        } catch {
            issues.append(
                AcceptanceBundleArtifactIssue(
                    artifact: relativePath,
                    problem: "missing or unreadable artifact"
                )
            )
            return nil
        }
    }

    private func mergedSourceConsistency(
        artifact: AcceptanceBundleSnapshot.SourceConsistencyEvidence?,
        meta: AcceptanceBundleSnapshot.SourceConsistencyEvidence?
    ) -> AcceptanceBundleSnapshot.SourceConsistencyEvidence? {
        if let artifact {
            return AcceptanceBundleSnapshot.SourceConsistencyEvidence(
                schema: artifact.schema ?? meta?.schema,
                output: artifact.output ?? meta?.output,
                baseline: artifact.baseline ?? meta?.baseline,
                status: artifact.status,
                mode: artifact.mode,
                session_id: artifact.session_id ?? meta?.session_id,
                entry_count: artifact.entry_count ?? meta?.entry_count,
                mismatch_count: artifact.mismatch_count ?? meta?.mismatch_count,
                detail: artifact.detail ?? meta?.detail
            )
        }
        return meta
    }

    private func sourceConsistencyBaselinePath(
        artifact: AcceptanceBundleSnapshot.SourceConsistencyEvidence?,
        merged: AcceptanceBundleSnapshot.SourceConsistencyEvidence?
    ) -> String? {
        if let artifactBaseline = artifact?.baseline?.trimmingCharacters(in: .whitespacesAndNewlines),
           !artifactBaseline.isEmpty {
            return artifactBaseline
        }
        if let mergedBaseline = merged?.baseline?.trimmingCharacters(in: .whitespacesAndNewlines),
           !mergedBaseline.isEmpty {
            return mergedBaseline
        }
        return nil
    }

    private func artifactExists(relativePath: String?, bundleRootURL: URL) -> Bool {
        guard let relativePath else {
            return false
        }
        do {
            return try artifactAccess.regularArtifactExists(
                relativePath: relativePath,
                bundleRootURL: bundleRootURL
            )
        } catch {
            return false
        }
    }

    private func artifactExists(
        explicitRelativePath: String?,
        fallbackRelativePath: String,
        bundleRootURL: URL
    ) -> Bool {
        let explicit = explicitRelativePath?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if explicit.isEmpty {
            return artifactExists(
                relativePath: fallbackRelativePath,
                bundleRootURL: bundleRootURL
            )
        }
        return artifactExists(
            relativePath: explicit,
            bundleRootURL: bundleRootURL
        )
    }

    private func validatedArtifactExists(
        relativePath: String?,
        bundleRootURL: URL,
        issues: inout [AcceptanceBundleArtifactIssue]
    ) -> Bool {
        let trimmed = relativePath?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !trimmed.isEmpty else {
            return false
        }
        do {
            guard try artifactAccess.regularArtifactExists(
                relativePath: trimmed,
                bundleRootURL: bundleRootURL
            ) else {
                issues.append(
                    AcceptanceBundleArtifactIssue(
                        artifact: trimmed,
                        problem: "missing or unreadable artifact"
                    )
                )
                return false
            }
            return true
        } catch AcceptanceBundleArtifactAccess.AccessError.unsafeArtifactPath {
            issues.append(
                AcceptanceBundleArtifactIssue(
                    artifact: trimmed,
                    problem: "unsafe artifact path"
                )
            )
            return false
        } catch {
            issues.append(
                AcceptanceBundleArtifactIssue(
                    artifact: trimmed,
                    problem: "missing or unreadable artifact"
                )
            )
            return false
        }
    }

    private func validatedPairingReceiptArtifact(
        relativePath: String?,
        expectedPairingReceiptID: String?,
        bundleRootURL: URL,
        issues: inout [AcceptanceBundleArtifactIssue]
    ) -> Bool {
        let trimmed = relativePath?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !trimmed.isEmpty else {
            return false
        }
        let expectedID = expectedPairingReceiptID?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !expectedID.isEmpty else {
            return false
        }
        do {
            guard let data = try artifactAccess.artifactDataIfPresent(
                relativePath: trimmed,
                bundleRootURL: bundleRootURL
            ) else {
                return false
            }
            guard let receipt = try? JSONDecoder().decode(PairingReceiptArtifact.self, from: data),
                  receipt.isValid(expectedID: expectedID) else {
                issues.append(
                    AcceptanceBundleArtifactIssue(
                        artifact: trimmed,
                        problem: "invalid pairing receipt artifact"
                    )
                )
                return false
            }
            return true
        } catch AcceptanceBundleArtifactAccess.AccessError.unsafeArtifactPath {
            return false
        } catch {
            return false
        }
    }

    private func validatedMetaURL(_ bundleRootURL: URL) throws -> URL {
        do {
            return try artifactAccess.metaURL(bundleRootURL: bundleRootURL)
        } catch let AcceptanceBundleArtifactAccess.AccessError.invalidBundleRoot(path) {
            throw ReadError.invalidBundleRoot(URL(fileURLWithPath: path, isDirectory: true))
        } catch let AcceptanceBundleArtifactAccess.AccessError.symlinkRejected(path) {
            throw ReadError.symlinkRejected(URL(fileURLWithPath: path))
        } catch AcceptanceBundleArtifactAccess.AccessError.missingArtifact {
            throw ReadError.missingMeta(bundleRootURL.appendingPathComponent("meta.json"))
        } catch AcceptanceBundleArtifactAccess.AccessError.unreadableArtifact {
            throw ReadError.malformedMeta(bundleRootURL.appendingPathComponent("meta.json"))
        } catch {
            throw ReadError.malformedMeta(bundleRootURL.appendingPathComponent("meta.json"))
        }
    }

}

private struct PairingReceiptArtifact: Decodable {
    let version: Int?
    let id: String?
    let profile_id: String?
    let target_id: String?
    let source_device_id: String?
    let target_device_id: String?
    let device_public_key: String?
    let method: String?
    let verified_at: String?
    let verification_hash: String?
    let verification_phrase: String?
    let protocol_version: String?

    func isValid(expectedID: String) -> Bool {
        guard let version, version > 0,
              clean(id) == expectedID,
              clean(profile_id) != nil,
              clean(target_id) != nil,
              clean(source_device_id) != nil,
              clean(target_device_id) != nil,
              clean(device_public_key) != nil,
              clean(device_public_key) == clean(target_device_id),
              clean(method) != nil,
              rfc3339Date(verified_at) != nil,
              clean(verification_hash) != nil || clean(verification_phrase) != nil,
              clean(protocol_version) != nil else {
            return false
        }
        return true
    }

    private func clean(_ value: String?) -> String? {
        let trimmed = value?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return trimmed.isEmpty ? nil : trimmed
    }

    private func rfc3339Date(_ value: String?) -> Date? {
        guard let value = clean(value) else {
            return nil
        }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]
        if let date = formatter.date(from: value) {
            return date
        }
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter.date(from: value)
    }
}
