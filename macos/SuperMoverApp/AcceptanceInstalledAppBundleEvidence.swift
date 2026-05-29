import Foundation

// Installed-app collection and release evidence are derived from the loaded
// bundle snapshot in multiple advisory/final-gate surfaces. Keep those derived
// views in one place so app-side consumers share the same proof owner.
extension AcceptanceBundleLoadedSnapshot {
    var installedAppCollectionProof: AcceptanceInstalledAppCollectionProofSummary {
        AcceptanceInstalledAppCollectionProofSummary.evaluate(snapshot: self)
    }

    var installedAppReleaseEvidence: AcceptanceInstalledAppReleaseEvidenceSummary {
        AcceptanceInstalledAppReleaseEvidenceSummary.evaluate(snapshot: self)
    }

    var hasBlockedAppAudit: Bool {
        !installedAppReleaseEvidence.source.appAuditReady
            || !installedAppReleaseEvidence.target.appAuditReady
    }

    var installedAppMachinePairProof: InstalledAppMachinePairProof? {
        guard let pair = installedAppCollectionProof.installedAppMachinePair else {
            return nil
        }
        return InstalledAppMachinePairProof(
            sourceMachineID: pair.source,
            targetMachineID: pair.target
        )
    }

    var verifiedCrossMachineBundleHandoffs: [AcceptanceBundleSnapshot.BundleHandoffEvidence] {
        guard let machinePairProof = installedAppMachinePairProof else {
            return []
        }
        return bundleHandoffs.filter {
            guard $0.verified else { return false }
            let exportingID = cleanInstalledAppMachineID($0.exporting_machine_id)
            let importingID = cleanInstalledAppMachineID($0.importing_machine_id)
            guard let exportingID, let importingID, exportingID != importingID else {
                return false
            }
            return Set([exportingID, importingID]) == machinePairProof.machineIDs
        }
    }

    var verifiedInstalledAppMachinePairHandoff: VerifiedInstalledAppMachinePairHandoff? {
        guard let machinePairProof = installedAppMachinePairProof,
              installedAppCollectionProof.matchesRecordedMachinePair,
              let handoff = verifiedCrossMachineBundleHandoffs.first else {
            return nil
        }
        return VerifiedInstalledAppMachinePairHandoff(
            machinePair: machinePairProof,
            handoff: handoff
        )
    }

    private func cleanInstalledAppMachineID(_ value: String?) -> String? {
        guard let value else {
            return nil
        }
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }
}
