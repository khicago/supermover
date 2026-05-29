import Foundation

enum EvidenceRunwayState: Equatable {
    case pass
    case pending
    case review
}

struct EvidenceGateEvaluation: Equatable {
    let hasStatusEvidence: Bool
    let statusNeedsReview: Bool
    let hasReportEvidence: Bool
    let reportNeedsReview: Bool
    let hasHealthEvidence: Bool
    let healthNeedsReview: Bool
    let hasVerifyEvidence: Bool
    let verifyNeedsReview: Bool

    var targetPreflightState: EvidenceRunwayState {
        guard hasTargetPreflightEvidence else {
            return .pending
        }
        return targetPreflightNeedsReview ? .review : .pass
    }

    var aggregateEvidenceState: EvidenceRunwayState {
        guard hasAnyEvidence else {
            return .pending
        }
        return anyEvidenceNeedsReview ? .review : .pass
    }

    var verificationState: EvidenceRunwayState {
        if hasVerifyEvidence {
            return verifyNeedsReview ? .review : .pass
        }
        if hasAnyEvidence {
            return .review
        }
        return .pending
    }

    var hasAnyEvidence: Bool {
        hasTargetPreflightEvidence || hasVerifyEvidence
    }

    private var hasTargetPreflightEvidence: Bool {
        hasStatusEvidence || hasReportEvidence || hasHealthEvidence
    }

    private var targetPreflightNeedsReview: Bool {
        (hasStatusEvidence && statusNeedsReview) ||
            (hasReportEvidence && reportNeedsReview) ||
            (hasHealthEvidence && healthNeedsReview)
    }

    private var anyEvidenceNeedsReview: Bool {
        targetPreflightNeedsReview || (hasVerifyEvidence && verifyNeedsReview)
    }
}
