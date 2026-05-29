import Foundation

struct AcceptanceEvaluationMode: Equatable {
    let requireOperatorEvidence: Bool
    let isLockedForTwoMachineCollection: Bool

    static func resolve(
        snapshot: AcceptanceBundleLoadedSnapshot?,
        draft: AcceptanceEvaluationDraft
    ) -> AcceptanceEvaluationMode {
        let locked = snapshot?.collectionMode == "two_machine"
        return AcceptanceEvaluationMode(
            requireOperatorEvidence: locked || draft.requireOperatorEvidence,
            isLockedForTwoMachineCollection: locked
        )
    }
}
