package status

import (
	"errors"
	"sort"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/report"
)

const (
	SchemaV1         = "supermover.status/v1"
	ExitOK           = 0
	ExitReviewNeeded = 1

	NetworkStatusNoEvidence     = "no_evidence"
	NetworkStatusReviewRequired = "review_required"

	OverallClean          = "clean"
	OverallReviewRequired = "review_required"
)

type Options struct {
	TargetRoot string
	Profile    *profile.Profile
	SessionID  string
}

type Report struct {
	Schema         string                   `json:"schema"`
	Scope          string                   `json:"scope"`
	TargetRoot     string                   `json:"target_root"`
	ProfileID      string                   `json:"profile_id,omitempty"`
	TargetID       string                   `json:"target_id,omitempty"`
	Overall        Overall                  `json:"overall"`
	Issues         []string                 `json:"issues,omitempty"`
	LatestSession  LatestSession            `json:"latest_session"`
	Counts         Counts                   `json:"counts"`
	PruneReview    PruneReview              `json:"prune_review"`
	Pairing        PairingEvidence          `json:"pairing"`
	Privacy        PrivacyEvidence          `json:"privacy"`
	TrafficPrivacy TrafficPrivacyAcceptance `json:"traffic_privacy_acceptance"`
	Network        NetworkEvidence          `json:"network"`
	ReviewRequired bool                     `json:"review_required"`
}

type Overall struct {
	Status       string `json:"status"`
	TargetStatus string `json:"target_status"`
}

type LatestSession struct {
	ID                   string `json:"id,omitempty"`
	ManifestID           string `json:"manifest_id,omitempty"`
	CreatedAt            string `json:"created_at,omitempty"`
	Entries              int    `json:"entries"`
	Files                int    `json:"files"`
	CompletenessStatus   string `json:"completeness_status"`
	FilesExpected        int    `json:"files_expected"`
	FilesVerified        int    `json:"files_verified"`
	VerificationErrors   int    `json:"verification_errors"`
	VerificationWarnings int    `json:"verification_warnings"`
}

type Counts struct {
	ManifestCount           int                          `json:"manifest_count"`
	ManifestEntries         int                          `json:"manifest_entries"`
	FilesExpected           int                          `json:"files_expected"`
	FilesVerified           int                          `json:"files_verified"`
	VerificationErrors      int                          `json:"verification_errors"`
	VerificationWarnings    int                          `json:"verification_warnings"`
	Warnings                int                          `json:"warnings"`
	ProfileSuggestions      int                          `json:"profile_suggestions"`
	SoftDeletes             int                          `json:"soft_deletes"`
	PruneApprovals          int                          `json:"prune_approvals"`
	PruneUnappliedApprovals int                          `json:"prune_unapplied_approvals"`
	PruneActiveApprovals    int                          `json:"prune_active_approvals"`
	PruneStaleApprovals     int                          `json:"prune_stale_approvals"`
	PruneExpiredApprovals   int                          `json:"prune_expired_approvals"`
	PruneConsumedApprovals  int                          `json:"prune_consumed_approvals"`
	PruneReceipts           int                          `json:"prune_receipts"`
	PruneReceiptIssues      int                          `json:"prune_receipt_issues"`
	TargetDrifts            int                          `json:"target_drifts"`
	LiveTargetDrifts        int                          `json:"live_target_drifts"`
	LiveTargetDriftProblems int                          `json:"live_target_drift_artifact_problems"`
	RecoveryIssues          int                          `json:"recovery_issues"`
	InvalidHealthRecords    int                          `json:"invalid_health_records"`
	ArtifactProblems        int                          `json:"artifact_problems"`
	ArtifactProblemSources  []ArtifactProblemSourceCount `json:"artifact_problem_sources,omitempty"`
	PairingIssues           int                          `json:"pairing_issues"`
	NetworkTransfers        int                          `json:"network_transfers"`
}

type PruneReview struct {
	Status string `json:"status"`
	Action string `json:"action"`
}

type ArtifactProblemSourceCount struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
}

type PairingEvidence struct {
	Status            string `json:"status"`
	ReceiptID         string `json:"receipt_id,omitempty"`
	TargetDeviceID    string `json:"target_device_id,omitempty"`
	PairedAt          string `json:"paired_at,omitempty"`
	Method            string `json:"method,omitempty"`
	VerifiedAt        string `json:"verified_at,omitempty"`
	Evidence          string `json:"evidence,omitempty"`
	EncryptedTransfer string `json:"encrypted_transfer"`
	Issue             string `json:"issue,omitempty"`
}

type PrivacyEvidence struct {
	Status               string   `json:"status"`
	Mode                 string   `json:"mode,omitempty"`
	TrafficLevel         int      `json:"traffic_level,omitempty"`
	Claim                string   `json:"claim"`
	LocalPush            string   `json:"local_push"`
	NetworkTransfer      string   `json:"network_transfer"`
	ResidualLeakage      []string `json:"residual_leakage"`
	ConfiguredReductions []string `json:"configured_reductions,omitempty"`
	OverheadStatus       string   `json:"overhead_status"`
	OverheadSource       string   `json:"overhead_source"`
}

type TrafficPrivacyAcceptance struct {
	Status               string                                  `json:"status"`
	Scope                string                                  `json:"scope"`
	Claim                string                                  `json:"claim"`
	AnonymityClaim       string                                  `json:"anonymity_claim"`
	EvidenceSource       string                                  `json:"evidence_source"`
	SessionID            string                                  `json:"session_id,omitempty"`
	Blockers             []string                                `json:"blockers,omitempty"`
	ConfiguredReductions []string                                `json:"configured_reductions,omitempty"`
	ResidualLeakage      []string                                `json:"residual_leakage,omitempty"`
	PaddingBucketBytes   int                                     `json:"padding_bucket_bytes,omitempty"`
	BatchMaxBytes        int                                     `json:"batch_max_bytes,omitempty"`
	BatchMaxCount        int                                     `json:"batch_max_count,omitempty"`
	JitterBudgetMillis   int                                     `json:"jitter_budget_millis,omitempty"`
	DiscoveryLowInfo     bool                                    `json:"discovery_low_info,omitempty"`
	ObservedOverhead     *control.NetworkTransferPrivacyOverhead `json:"observed_overhead,omitempty"`
}

type NetworkEvidence struct {
	Status           string                    `json:"status"`
	ArtifactProblems int                       `json:"artifact_problems"`
	Transfers        []NetworkTransferEvidence `json:"transfers,omitempty"`
}

type NetworkTransferEvidence struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Stage     string `json:"stage,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	Error     string `json:"error,omitempty"`
	Action    string `json:"action"`
}

func Build(opts Options) (Report, error) {
	if opts.Profile == nil {
		return Report{}, errors.New("profile is required")
	}

	full, err := report.BuildReport(report.Options{
		TargetRoot: opts.TargetRoot,
		ProfileID:  opts.Profile.ProfileID,
		TargetID:   opts.Profile.Target.TargetID,
		SessionID:  opts.SessionID,
		Profile:    opts.Profile,
	})
	if err != nil {
		return Report{}, err
	}

	out := Report{
		Schema:         SchemaV1,
		Scope:          full.Scope,
		TargetRoot:     full.TargetRoot,
		ProfileID:      full.ProfileID,
		TargetID:       full.TargetID,
		Overall:        overall(full),
		Issues:         append([]string(nil), full.Overall.Issues...),
		LatestSession:  latestSession(full.LatestSession),
		Counts:         counts(full.Summary, full.ArtifactProblems),
		PruneReview:    pruneReview(full.PruneReview),
		Pairing:        pairingEvidence(full.Pairing),
		Privacy:        privacyEvidence(full.Privacy),
		TrafficPrivacy: trafficPrivacyAcceptance(full.TrafficPrivacy),
		Network:        networkEvidence(full.NetworkTransfers, full.ArtifactProblems),
	}
	out.ReviewRequired = out.NeedsReview()
	return out, nil
}

func (r Report) NeedsReview() bool {
	return r.Overall.Status != OverallClean
}

func (r Report) ExitCode() int {
	if r.NeedsReview() {
		return ExitReviewNeeded
	}
	return ExitOK
}

func overall(in report.Report) Overall {
	out := Overall{
		Status:       OverallReviewRequired,
		TargetStatus: string(in.Overall.Status),
	}
	if in.Overall.Status == report.StatusVerified {
		out.Status = OverallClean
	}
	return out
}

func latestSession(in report.LatestSession) LatestSession {
	return LatestSession{
		ID:                   in.ID,
		ManifestID:           in.ManifestID,
		CreatedAt:            in.CreatedAt,
		Entries:              in.Entries,
		Files:                in.Files,
		CompletenessStatus:   string(in.Completeness.Status),
		FilesExpected:        in.Completeness.FilesExpected,
		FilesVerified:        in.Completeness.FilesVerified,
		VerificationErrors:   in.Completeness.VerificationErrors,
		VerificationWarnings: in.Completeness.VerificationWarnings,
	}
}

func counts(in report.Summary, artifactProblems []report.ArtifactProblem) Counts {
	return Counts{
		ManifestCount:           in.ManifestCount,
		ManifestEntries:         in.ManifestEntries,
		FilesExpected:           in.FilesExpected,
		FilesVerified:           in.FilesVerified,
		VerificationErrors:      in.VerificationErrors,
		VerificationWarnings:    in.VerificationWarnings,
		Warnings:                in.Warnings,
		ProfileSuggestions:      in.ProfileSuggestions,
		SoftDeletes:             in.SoftDeletes,
		PruneApprovals:          in.PruneApprovals,
		PruneUnappliedApprovals: in.PruneUnappliedApprovals,
		PruneActiveApprovals:    in.PruneActiveApprovals,
		PruneStaleApprovals:     in.PruneStaleApprovals,
		PruneExpiredApprovals:   in.PruneExpiredApprovals,
		PruneConsumedApprovals:  in.PruneConsumedApprovals,
		PruneReceipts:           in.PruneReceipts,
		PruneReceiptIssues:      in.PruneReceiptIssues,
		TargetDrifts:            in.TargetDrifts,
		LiveTargetDrifts:        in.LiveTargetDrifts,
		LiveTargetDriftProblems: in.LiveTargetDriftProblems,
		RecoveryIssues:          in.RecoveryIssues,
		InvalidHealthRecords:    in.InvalidHealthRecords,
		ArtifactProblems:        in.ArtifactProblems,
		ArtifactProblemSources:  artifactProblemSourceCounts(artifactProblems),
		PairingIssues:           in.PairingIssues,
		NetworkTransfers:        in.NetworkTransfers,
	}
}

func pruneReview(in report.PruneReview) PruneReview {
	return PruneReview{
		Status: string(in.Status),
		Action: in.ReviewAction(),
	}
}

func artifactProblemSourceCounts(problems []report.ArtifactProblem) []ArtifactProblemSourceCount {
	if len(problems) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, problem := range problems {
		source := problem.Source
		if source == "" {
			source = "unknown"
		}
		counts[source]++
	}
	sources := make([]string, 0, len(counts))
	for source := range counts {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	out := make([]ArtifactProblemSourceCount, 0, len(sources))
	for _, source := range sources {
		out = append(out, ArtifactProblemSourceCount{
			Source: source,
			Count:  counts[source],
		})
	}
	return out
}

func pairingEvidence(in report.PairingState) PairingEvidence {
	return PairingEvidence{
		Status:            string(in.Status),
		ReceiptID:         in.ReceiptID,
		TargetDeviceID:    in.TargetDeviceID,
		PairedAt:          in.PairedAt,
		Method:            in.Method,
		VerifiedAt:        in.VerifiedAt,
		Evidence:          in.Evidence,
		EncryptedTransfer: in.EncryptedTransfer,
		Issue:             in.Issue,
	}
}

func privacyEvidence(in report.PrivacyState) PrivacyEvidence {
	return PrivacyEvidence{
		Status:               in.Status,
		Mode:                 in.Mode,
		TrafficLevel:         in.TrafficLevel,
		Claim:                in.Claim,
		LocalPush:            in.LocalPush,
		NetworkTransfer:      in.NetworkTransfer,
		ResidualLeakage:      append([]string(nil), in.ResidualLeakage...),
		ConfiguredReductions: append([]string(nil), in.ConfiguredReduction...),
		OverheadStatus:       in.Overhead.Status,
		OverheadSource:       in.Overhead.Source,
	}
}

func trafficPrivacyAcceptance(in report.TrafficPrivacyAcceptance) TrafficPrivacyAcceptance {
	return TrafficPrivacyAcceptance{
		Status:               in.Status,
		Scope:                in.Scope,
		Claim:                in.Claim,
		AnonymityClaim:       in.AnonymityClaim,
		EvidenceSource:       in.EvidenceSource,
		SessionID:            in.SessionID,
		Blockers:             append([]string(nil), in.Blockers...),
		ConfiguredReductions: append([]string(nil), in.ConfiguredReductions...),
		ResidualLeakage:      append([]string(nil), in.ResidualLeakage...),
		PaddingBucketBytes:   in.PaddingBucketBytes,
		BatchMaxBytes:        in.BatchMaxBytes,
		BatchMaxCount:        in.BatchMaxCount,
		JitterBudgetMillis:   in.JitterBudgetMillis,
		DiscoveryLowInfo:     in.DiscoveryLowInfo,
		ObservedOverhead:     cloneNetworkPrivacyOverhead(in.ObservedOverhead),
	}
}

func cloneNetworkPrivacyOverhead(in *control.NetworkTransferPrivacyOverhead) *control.NetworkTransferPrivacyOverhead {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func networkEvidence(transfers []report.NetworkTransfer, artifactProblems []report.ArtifactProblem) NetworkEvidence {
	out := NetworkEvidence{
		Status:           NetworkStatusNoEvidence,
		ArtifactProblems: networkTransferArtifactProblems(artifactProblems),
	}
	if len(transfers) > 0 || out.ArtifactProblems > 0 {
		out.Status = NetworkStatusReviewRequired
	}
	for _, transfer := range transfers {
		out.Transfers = append(out.Transfers, NetworkTransferEvidence{
			SessionID: transfer.SessionID,
			Status:    transfer.Status,
			Stage:     transfer.Stage,
			ErrorCode: transfer.ErrorCode,
			Error:     transfer.Error,
			Action:    transfer.Action,
		})
	}
	return out
}

func networkTransferArtifactProblems(problems []report.ArtifactProblem) int {
	count := 0
	for _, problem := range problems {
		if problem.Source == "network_transfer" {
			count++
		}
	}
	return count
}
