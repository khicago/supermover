import Foundation

enum EvidenceSeverity: Int, CaseIterable, Comparable {
    case ok = 0
    case unavailable = 1
    case warning = 2
    case review = 3
    case critical = 4

    static func < (lhs: EvidenceSeverity, rhs: EvidenceSeverity) -> Bool {
        lhs.rawValue < rhs.rawValue
    }

    var label: String {
        switch self {
        case .ok:
            return "ok"
        case .unavailable:
            return "not checked"
        case .warning:
            return "warning"
        case .review:
            return "review"
        case .critical:
            return "critical"
        }
    }
}

enum EvidenceCategory: String, CaseIterable, Identifiable, Hashable {
    case verify
    case currentSourceConsistency
    case status
    case report
    case health

    var id: String { rawValue }

    var title: String {
        switch self {
        case .verify:
            return "Verify"
        case .currentSourceConsistency:
            return "Current Source"
        case .status:
            return "Status"
        case .report:
            return "Report"
        case .health:
            return "Health"
        }
    }

    var rawSurfaceID: String { rawValue }

    fileprivate var sortOrder: Int {
        switch self {
        case .verify:
            return 0
        case .currentSourceConsistency:
            return 1
        case .status:
            return 2
        case .report:
            return 3
        case .health:
            return 4
        }
    }

    fileprivate var notCheckedDetail: String {
        "No \(rawSurfaceID) evidence snapshot has been read for the current context."
    }
}

enum EvidenceCardActionMode: Equatable {
    case readOnly
}

struct EvidenceCardAction: Equatable {
    let label: String
    let rawSurfaceID: String
    let mode: EvidenceCardActionMode

    init(label: String, rawSurfaceID: String, mode: EvidenceCardActionMode = .readOnly) {
        self.label = label
        self.rawSurfaceID = rawSurfaceID
        self.mode = mode
    }
}

struct EvidenceFact: Identifiable, Equatable, Hashable {
    let key: String
    let label: String
    let value: String
    let severity: EvidenceSeverity
    let detail: String?

    var id: String { key }

    init(
        key: String,
        label: String,
        value: String,
        severity: EvidenceSeverity,
        detail: String? = nil
    ) {
        self.key = key
        self.label = label
        self.value = value
        self.severity = severity
        self.detail = detail
    }

    static let unavailable = EvidenceFact(
        key: "availability",
        label: "Availability",
        value: "not checked",
        severity: .unavailable,
        detail: "No evidence has been read yet."
    )
}

struct EvidenceCardInput: Equatable {
    let category: EvidenceCategory
    let status: String
    let detail: String
    let severity: EvidenceSeverity
    let facts: [EvidenceFact]
    let rawSurfaceID: String?
    let nextAction: EvidenceCardAction?

    init(
        category: EvidenceCategory,
        status: String,
        detail: String,
        severity: EvidenceSeverity,
        facts: [EvidenceFact] = [],
        rawSurfaceID: String? = nil,
        nextAction: EvidenceCardAction? = nil
    ) {
        self.category = category
        self.status = status
        self.detail = detail
        self.severity = severity
        self.facts = facts
        self.rawSurfaceID = rawSurfaceID
        self.nextAction = nextAction
    }
}

struct EvidenceCard: Identifiable, Equatable {
    let category: EvidenceCategory
    let title: String
    let status: String
    let detail: String
    let severity: EvidenceSeverity
    let facts: [EvidenceFact]
    let rawSurfaceID: String
    let nextAction: EvidenceCardAction?

    var id: EvidenceCategory { category }

    init(input: EvidenceCardInput) {
        category = input.category
        title = input.category.title
        status = input.status
        detail = input.detail
        facts = input.facts
        rawSurfaceID = input.rawSurfaceID ?? input.category.rawSurfaceID

        let strongestFactSeverity = input.facts.map(\.severity).max() ?? input.severity
        severity = max(input.severity, strongestFactSeverity)
        nextAction = input.nextAction ?? EvidenceCard.defaultNextAction(
            for: input.category,
            severity: severity
        )
    }

    static func unavailable(for category: EvidenceCategory) -> EvidenceCard {
        EvidenceCard(
            input: EvidenceCardInput(
                category: category,
                status: "not checked",
                detail: category.notCheckedDetail,
                severity: .unavailable,
                facts: [.unavailable],
                rawSurfaceID: category.rawSurfaceID,
                nextAction: EvidenceCardAction(
                    label: "Read \(category.title.lowercased()) evidence",
                    rawSurfaceID: category.rawSurfaceID
                )
            )
        )
    }

    func displayFacts(maxCount: Int = 7) -> [EvidenceFact] {
        guard maxCount > 0 else { return [] }
        guard facts.count > maxCount else { return facts }

        let identityKeys: Set<String> = [
            "target_root",
            "profile_id",
            "target_id",
            "manifest_id",
            "session_id",
            "latest_session",
            "overall_status",
            "target_status",
        ]

        var selected: [EvidenceFact] = []
        var selectedKeys = Set<String>()

        func appendIfNeeded(_ fact: EvidenceFact) {
            guard selected.count < maxCount, selectedKeys.insert(fact.key).inserted else {
                return
            }
            selected.append(fact)
        }

        facts.filter { identityKeys.contains($0.key) }
            .prefix(3)
            .forEach(appendIfNeeded)

        facts.filter { $0.key == "cli_exit" }
            .forEach(appendIfNeeded)

        facts.enumerated()
            .filter { _, fact in fact.severity > .ok && !selectedKeys.contains(fact.key) }
            .sorted { lhs, rhs in
                if lhs.element.severity != rhs.element.severity {
                    return lhs.element.severity > rhs.element.severity
                }
                return lhs.offset < rhs.offset
            }
            .map(\.element)
            .forEach(appendIfNeeded)

        facts.filter { !selectedKeys.contains($0.key) }
            .forEach(appendIfNeeded)

        return selected
    }

    func hiddenFactCount(maxCount: Int = 7) -> Int {
        max(0, facts.count - displayFacts(maxCount: maxCount).count)
    }

    private static func defaultNextAction(
        for category: EvidenceCategory,
        severity: EvidenceSeverity
    ) -> EvidenceCardAction? {
        switch severity {
        case .ok:
            return nil
        case .unavailable:
            return EvidenceCardAction(
                label: "Read \(category.title.lowercased()) evidence",
                rawSurfaceID: category.rawSurfaceID
            )
        case .warning, .review, .critical:
            return EvidenceCardAction(
                label: "Inspect \(category.title.lowercased()) evidence",
                rawSurfaceID: category.rawSurfaceID
            )
        }
    }
}

struct EvidenceVaultBuilder {
    var verify: VerifySnapshot?
    var sourceConsistency: AcceptanceBundleSnapshot.SourceConsistencyEvidence?
    var status: StatusSnapshot?
    var report: ReportSnapshot?
    var health: HealthSnapshot?
    var envelopes: [StructuredArtifactKind: StructuredEvidenceEnvelope]

    init(
        verify: VerifySnapshot? = nil,
        sourceConsistency: AcceptanceBundleSnapshot.SourceConsistencyEvidence? = nil,
        status: StatusSnapshot? = nil,
        report: ReportSnapshot? = nil,
        health: HealthSnapshot? = nil,
        envelopes: [StructuredArtifactKind: StructuredEvidenceEnvelope] = [:]
    ) {
        self.verify = verify
        self.sourceConsistency = sourceConsistency
        self.status = status
        self.report = report
        self.health = health
        self.envelopes = envelopes
    }

    func cards() -> [EvidenceCard] {
        Self.cards(
            from: [
                verify.map { Self.withEnvelope(Self.verifyInput($0), envelope: envelopes[.verify]) },
                sourceConsistency.map { Self.withEnvelope(Self.currentSourceConsistencyInput($0), envelope: envelopes[.sourceConsistency]) },
                status.map { Self.withEnvelope(Self.statusInput($0), envelope: envelopes[.status]) },
                report.map { Self.withEnvelope(Self.reportInput($0), envelope: envelopes[.report]) },
                health.map { Self.withEnvelope(Self.healthInput($0), envelope: envelopes[.health]) },
            ].compactMap { $0 }
        )
    }

    static func cards(
        from inputs: [EvidenceCardInput],
        categories: [EvidenceCategory] = EvidenceCategory.allCases
    ) -> [EvidenceCard] {
        var cardsByCategory: [EvidenceCategory: EvidenceCard] = [:]
        for input in inputs {
            cardsByCategory[input.category] = EvidenceCard(input: input)
        }

        let cards = categories.map { category in
            cardsByCategory[category] ?? EvidenceCard.unavailable(for: category)
        }
        return sorted(cards)
    }

    private static func sorted(_ cards: [EvidenceCard]) -> [EvidenceCard] {
        cards.sorted { lhs, rhs in
            if lhs.severity != rhs.severity {
                return lhs.severity > rhs.severity
            }
            return lhs.category.sortOrder < rhs.category.sortOrder
        }
    }

    private static func withEnvelope(
        _ input: EvidenceCardInput,
        envelope: StructuredEvidenceEnvelope?
    ) -> EvidenceCardInput {
        guard let envelope else {
            return input
        }
        let facts = input.facts + [cliExitFact(envelope)]
        return EvidenceCardInput(
            category: input.category,
            status: input.status,
            detail: input.detail,
            severity: max(input.severity, strongestSeverity(in: facts)),
            facts: facts,
            rawSurfaceID: input.rawSurfaceID,
            nextAction: input.nextAction
        )
    }

    private static func cliExitFact(_ envelope: StructuredEvidenceEnvelope) -> EvidenceFact {
        let severity: EvidenceSeverity
        if envelope.exitCode == 0 {
            severity = .ok
        } else if envelope.exitCode == 1 {
            severity = .review
        } else {
            severity = .critical
        }
        return EvidenceFact(
            key: "cli_exit",
            label: "CLI exit",
            value: "\(envelope.exitCode)",
            severity: severity,
            detail: envelope.exitCode == 0 ? "Structured stdout came from a successful CLI run." : "Structured stdout came from a non-zero CLI run and must be treated as review evidence."
        )
    }

    private static func verifyInput(_ verify: VerifySnapshot) -> EvidenceCardInput {
        let facts: [EvidenceFact] = [
            textFact(key: "target_root", label: "Target root", value: verify.target_root),
            textFact(key: "manifest_id", label: "Manifest", value: emptyAsNone(verify.manifest.manifestID), severity: verify.summary.manifest_count == 0 ? .critical : .ok),
            textFact(key: "session_id", label: "Session", value: emptyAsNone(verify.session_id)),
            countFact(key: "manifest_count", label: "Manifest count", count: verify.summary.manifest_count, issueSeverity: .critical, reviewWhenZero: true),
            countFact(key: "files_expected", label: "Files expected", count: verify.summary.files_expected, reviewWhenNonZero: false),
            countFact(key: "files_verified", label: "Files verified", count: verify.summary.files_verified, reviewWhenNonZero: false),
            countFact(key: "error_findings", label: "Error findings", count: verify.summary.error_findings, issueSeverity: .critical),
            countFact(key: "warning_findings", label: "Warning findings", count: verify.summary.warning_findings, issueSeverity: .review),
            countFact(key: "warnings", label: "Warnings", count: verify.summary.warnings, issueSeverity: .review),
            countFact(key: "soft_deletes", label: "Soft deletes", count: verify.summary.soft_deletes, issueSeverity: .review),
            countFact(key: "target_drifts", label: "Target drifts", count: verify.summary.target_drifts, issueSeverity: .review),
            countFact(key: "artifact_problems", label: "Artifact problems", count: verify.summary.artifact_problems, issueSeverity: .critical),
            countFact(key: "skipped_digest", label: "Skipped digest", count: verify.summary.skipped_digest, issueSeverity: .review),
        ]

        return EvidenceCardInput(
            category: .verify,
            status: verify.statusLabel,
            detail: "\(verify.summary.files_verified) of \(verify.summary.files_expected) expected files verified against manifest evidence.",
            severity: strongestSeverity(in: facts),
            facts: facts,
            rawSurfaceID: EvidenceCategory.verify.rawSurfaceID
        )
    }

    private static func statusInput(_ status: StatusSnapshot) -> EvidenceCardInput {
        let issueCount = status.issues?.count ?? 0
        let facts: [EvidenceFact] = [
            textFact(key: "profile_id", label: "Config ID", value: status.profile_id),
            textFact(key: "target_id", label: "Target", value: status.target_id),
            textFact(key: "target_root", label: "Target root", value: status.target_root),
            textFact(key: "overall_status", label: "Overall", value: status.overall.status),
            textFact(key: "target_status", label: "Target status", value: status.overall.target_status),
            textFact(key: "pairing_status", label: "Pairing", value: status.pairing.status),
            textFact(key: "pairing_receipt_id", label: "Pairing receipt", value: emptyAsNone(status.pairing.receipt_id)),
            textFact(key: "pairing_receipt_source", label: "Receipt source", value: emptyAsNone(status.pairing.receipt_source)),
            textFact(key: "pairing_receipt_path", label: "Receipt path", value: emptyAsNone(status.pairing.receipt_path)),
            textFact(key: "latest_session", label: "Latest session", value: emptyAsNone(status.latest_session.id)),
            textFact(key: "completeness", label: "Completeness", value: status.latest_session.completeness_status),
            countFact(key: "issues", label: "Issues", count: issueCount, issueSeverity: .review),
            countFact(key: "verification_errors", label: "Verification errors", count: status.latest_session.verification_errors, issueSeverity: .critical),
            countFact(key: "verification_warnings", label: "Verification warnings", count: status.latest_session.verification_warnings, issueSeverity: .review),
            countFact(key: "warnings", label: "Warnings", count: status.counts.warnings, issueSeverity: .review),
            countFact(key: "target_drifts", label: "Target drifts", count: status.counts.target_drifts, issueSeverity: .review),
            countFact(key: "live_target_drifts", label: "Live target drifts", count: status.counts.live_target_drifts, issueSeverity: .review),
            countFact(key: "recovery_issues", label: "Recovery issues", count: status.counts.recovery_issues, issueSeverity: .critical),
            countFact(key: "artifact_problems", label: "Artifact problems", count: status.counts.artifact_problems + status.network.artifact_problems, issueSeverity: .critical),
            countFact(key: "prune_unapplied_approvals", label: "Unapplied prune approvals", count: status.counts.prune_unapplied_approvals, issueSeverity: .review),
            countFact(key: "prune_stale_approvals", label: "Stale prune approvals", count: status.counts.prune_stale_approvals, issueSeverity: .review),
            countFact(key: "prune_expired_approvals", label: "Expired prune approvals", count: status.counts.prune_expired_approvals, issueSeverity: .review),
            countFact(key: "prune_receipt_issues", label: "Prune receipt issues", count: status.counts.prune_receipt_issues, issueSeverity: .critical),
        ]

        return EvidenceCardInput(
            category: .status,
            status: status.overall.status,
            detail: "Target status: \(status.overall.target_status). Latest completeness: \(status.latest_session.completeness_status).",
            severity: strongestSeverity(in: facts),
            facts: facts,
            rawSurfaceID: EvidenceCategory.status.rawSurfaceID
        )
    }

    private static func currentSourceConsistencyInput(
        _ consistency: AcceptanceBundleSnapshot.SourceConsistencyEvidence
    ) -> EvidenceCardInput {
        let normalizedStatus = consistency.status.trimmingCharacters(in: .whitespacesAndNewlines)
        let normalizedMode = consistency.mode.trimmingCharacters(in: .whitespacesAndNewlines)
        let sessionID = consistency.session_id?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        let entryCount = consistency.entry_count ?? 0
        let mismatchCount = consistency.mismatch_count ?? 0
        let severity: EvidenceSeverity = (normalizedStatus == "pass" && normalizedMode == "current_source_verified") ? .ok : .review
        let detail = consistency.detail?.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty == false
            ? consistency.detail!.trimmingCharacters(in: .whitespacesAndNewlines)
            : "Current source proof has not been captured for the current acceptance transfer context."
        let facts: [EvidenceFact] = [
            textFact(key: "status", label: "Status", value: emptyAsNone(normalizedStatus), severity: severity),
            textFact(key: "mode", label: "Mode", value: emptyAsNone(normalizedMode), severity: severity),
            textFact(key: "session_id", label: "Session", value: emptyAsNone(sessionID)),
            countFact(key: "entry_count", label: "Entries", count: entryCount, reviewWhenNonZero: false),
            countFact(key: "mismatch_count", label: "Mismatches", count: mismatchCount, issueSeverity: .review),
        ]
        let statusLabel: String
        if normalizedStatus == "pass" && normalizedMode == "current_source_verified" {
            statusLabel = "current source verified"
        } else if normalizedMode.isEmpty {
            statusLabel = normalizedStatus.isEmpty ? "not checked" : normalizedStatus
        } else {
            statusLabel = normalizedMode.replacingOccurrences(of: "_", with: " ")
        }

        return EvidenceCardInput(
            category: .currentSourceConsistency,
            status: statusLabel,
            detail: detail,
            severity: max(severity, strongestSeverity(in: facts)),
            facts: facts,
            rawSurfaceID: "source-consistency"
        )
    }

    private static func reportInput(_ report: ReportSnapshot) -> EvidenceCardInput {
        let issueCount = report.overall.issues?.count ?? 0
        let facts: [EvidenceFact] = [
            textFact(key: "profile_id", label: "Config ID", value: emptyAsNone(report.profile_id)),
            textFact(key: "target_id", label: "Target", value: emptyAsNone(report.target_id)),
            textFact(key: "target_root", label: "Target root", value: report.target_root),
            textFact(key: "overall_status", label: "Overall", value: report.overall.status),
            textFact(key: "pairing_status", label: "Pairing", value: report.pairing.status),
            textFact(key: "pairing_receipt_id", label: "Pairing receipt", value: emptyAsNone(report.pairing.receipt_id)),
            textFact(key: "pairing_receipt_source", label: "Receipt source", value: emptyAsNone(report.pairing.receipt_source)),
            textFact(key: "pairing_receipt_path", label: "Receipt path", value: emptyAsNone(report.pairing.receipt_path)),
            countFact(key: "issues", label: "Issues", count: issueCount, issueSeverity: .review),
            textFact(key: "latest_session", label: "Latest session", value: emptyAsNone(report.latest_session.id)),
            textFact(key: "completeness", label: "Completeness", value: report.latest_session.completeness.status),
            countFact(key: "verification_errors", label: "Verification errors", count: report.latest_session.completeness.verification_errors, issueSeverity: .critical),
            countFact(key: "verification_warnings", label: "Verification warnings", count: report.latest_session.completeness.verification_warnings, issueSeverity: .review),
            countFact(key: "warnings", label: "Warnings", count: report.summary.warnings, issueSeverity: .review),
            countFact(key: "soft_deletes", label: "Soft deletes", count: report.summary.soft_deletes, issueSeverity: .review),
            countFact(key: "target_drifts", label: "Target drifts", count: report.summary.target_drifts, issueSeverity: .review),
            countFact(key: "live_target_drifts", label: "Live target drifts", count: report.summary.live_target_drifts, issueSeverity: .review),
            countFact(key: "artifact_problems", label: "Artifact problems", count: report.summary.artifact_problems, issueSeverity: .critical),
            boolFact(key: "prune_approval_required", label: "Prune approval required", value: report.prune_review.approval_required, trueSeverity: .review),
            countFact(key: "prune_candidates", label: "Prune candidates", count: report.prune_review.summary.candidates, issueSeverity: .review),
            countFact(key: "prune_refusals", label: "Prune refusals", count: report.prune_review.summary.refusals, issueSeverity: .review),
            countFact(key: "prune_unapplied_approvals", label: "Unapplied prune approvals", count: report.prune_review.summary.unapplied_approvals, issueSeverity: .review),
            countFact(key: "prune_receipt_issues", label: "Prune receipt issues", count: report.prune_review.summary.receipt_issues, issueSeverity: .critical),
            boolFact(key: "health_healthy", label: "Health", value: report.health.healthy, trueSeverity: .ok, falseSeverity: .critical),
            countFact(key: "health_incomplete_sessions", label: "Incomplete sessions", count: report.health.summary.incomplete_sessions, issueSeverity: .critical),
            countFact(key: "health_invalid_records", label: "Invalid records", count: report.health.summary.invalid_records, issueSeverity: .critical),
            countFact(key: "health_artifact_problems", label: "Health artifact problems", count: report.health.summary.artifact_problems, issueSeverity: .critical),
            countFact(key: "health_target_drifts", label: "Health target drifts", count: report.health.summary.target_drifts, issueSeverity: .review),
        ]

        return EvidenceCardInput(
            category: .report,
            status: report.overall.status,
            detail: "Latest completeness: \(report.latest_session.completeness.status). Prune review: \(report.prune_review.status).",
            severity: strongestSeverity(in: facts),
            facts: facts,
            rawSurfaceID: EvidenceCategory.report.rawSurfaceID
        )
    }

    private static func healthInput(_ health: HealthSnapshot) -> EvidenceCardInput {
        let facts: [EvidenceFact] = [
            textFact(key: "target_root", label: "Target root", value: health.target_root),
            boolFact(key: "healthy", label: "Healthy", value: health.healthy, trueSeverity: .ok, falseSeverity: .critical),
            countFact(key: "incomplete_sessions", label: "Incomplete sessions", count: health.summary.incomplete_sessions, issueSeverity: .critical),
            countFact(key: "invalid_records", label: "Invalid records", count: health.summary.invalid_records, issueSeverity: .critical),
            countFact(key: "artifact_problems", label: "Artifact problems", count: health.summary.artifact_problems, issueSeverity: .critical),
            countFact(key: "target_drifts", label: "Target drifts", count: health.summary.target_drifts, issueSeverity: .review),
            countFact(key: "network_transfers", label: "Network transfers", count: health.summary.network_transfers, reviewWhenNonZero: false),
        ]

        return EvidenceCardInput(
            category: .health,
            status: health.healthy ? "healthy" : "unhealthy",
            detail: "Recovery evidence has \(health.summary.incomplete_sessions) incomplete sessions, \(health.summary.invalid_records) invalid records, and \(health.summary.artifact_problems) artifact problems.",
            severity: strongestSeverity(in: facts),
            facts: facts,
            rawSurfaceID: EvidenceCategory.health.rawSurfaceID
        )
    }

    private static func strongestSeverity(in facts: [EvidenceFact]) -> EvidenceSeverity {
        facts.map(\.severity).max() ?? .ok
    }

    private static func textFact(
        key: String,
        label: String,
        value: String,
        severity: EvidenceSeverity? = nil
    ) -> EvidenceFact {
        EvidenceFact(
            key: key,
            label: label,
            value: value,
            severity: severity ?? severityForStatusText(value)
        )
    }

    private static func countFact(
        key: String,
        label: String,
        count: Int,
        issueSeverity: EvidenceSeverity = .review,
        reviewWhenNonZero: Bool = true,
        reviewWhenZero: Bool = false
    ) -> EvidenceFact {
        let severity: EvidenceSeverity
        if reviewWhenZero && count == 0 {
            severity = issueSeverity
        } else if reviewWhenNonZero && count > 0 {
            severity = issueSeverity
        } else {
            severity = .ok
        }
        return EvidenceFact(key: key, label: label, value: "\(count)", severity: severity)
    }

    private static func boolFact(
        key: String,
        label: String,
        value: Bool,
        trueSeverity: EvidenceSeverity = .review,
        falseSeverity: EvidenceSeverity = .ok
    ) -> EvidenceFact {
        let severity = value ? trueSeverity : falseSeverity
        return EvidenceFact(key: key, label: label, value: value ? "yes" : "no", severity: severity)
    }

    private static func severityForStatusText(_ value: String) -> EvidenceSeverity {
        let normalized = value.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        if normalized.isEmpty || normalized == "none" {
            return .ok
        }

        let criticalTerms = [
            "error",
            "failed",
            "invalid",
            "mismatch",
            "unhealthy",
            "blocked",
        ]
        if criticalTerms.contains(where: normalized.contains) {
            return .critical
        }

        let reviewTerms = [
            "review",
            "drift",
            "warning",
            "attention",
            "pending",
            "missing",
            "unavailable",
            "unpaired",
            "artifact",
            "recovery",
            "stale",
            "incomplete",
        ]
        if reviewTerms.contains(where: normalized.contains) {
            return .review
        }

        return .ok
    }

    private static func emptyAsNone(_ value: String?) -> String {
        let trimmed = value?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return trimmed.isEmpty ? "none" : trimmed
    }
}
