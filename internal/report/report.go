package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/health"
	"github.com/khicago/supermover/internal/networkpush"
	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/prune"
	"github.com/khicago/supermover/internal/tlsidentity"
	"github.com/khicago/supermover/internal/transport"
	"github.com/khicago/supermover/internal/verify"
)

const ScopeLocalMigrationTarget = "local_migration_target"

type Status string

const (
	StatusEmpty     Status = "local_target_empty"
	StatusVerified  Status = "local_target_verified"
	StatusAttention Status = "local_target_attention"
	StatusFailed    Status = "local_target_verification_failed"
	StatusUnhealthy Status = "local_target_unhealthy"
)

type CompletenessStatus string

const (
	CompletenessNoPublishedSession CompletenessStatus = "no_published_session"
	CompletenessVerified           CompletenessStatus = "verified_at_report_time"
	CompletenessNeedsAttention     CompletenessStatus = "needs_attention"
	CompletenessFailed             CompletenessStatus = "verification_failed"
)

type Options struct {
	TargetRoot string
	ProfileID  string
	TargetID   string
	SessionID  string
	Profile    *profile.Profile
}

type Report struct {
	Scope                string                   `json:"scope"`
	TargetRoot           string                   `json:"target_root"`
	ProfileID            string                   `json:"profile_id,omitempty"`
	TargetID             string                   `json:"target_id,omitempty"`
	SessionID            string                   `json:"session_id,omitempty"`
	Overall              Overall                  `json:"overall"`
	Summary              Summary                  `json:"summary"`
	LatestSession        LatestSession            `json:"latest_session"`
	Warnings             []control.Warning        `json:"warnings,omitempty"`
	ProfileSuggestions   []ProfileSuggestion      `json:"profile_suggestions,omitempty"`
	SoftDeletes          []control.SoftDelete     `json:"soft_deletes,omitempty"`
	PruneReview          PruneReview              `json:"prune_review"`
	TargetDrifts         []control.TargetDrift    `json:"target_drifts,omitempty"`
	LiveTargetDrift      LiveTargetDrift          `json:"live_target_drift"`
	NetworkTransfers     []NetworkTransfer        `json:"network_transfers,omitempty"`
	Pairing              PairingState             `json:"pairing"`
	Privacy              PrivacyState             `json:"privacy"`
	TrafficPrivacy       TrafficPrivacyAcceptance `json:"traffic_privacy_acceptance"`
	Health               Health                   `json:"health"`
	ArtifactProblems     []ArtifactProblem        `json:"artifact_problems,omitempty"`
	ProfileSnapshots     []ProfileSnapshot        `json:"profile_snapshots,omitempty"`
	VerificationFindings []verify.Finding         `json:"verification_findings,omitempty"`
}

type PruneReviewReport struct {
	Scope            string            `json:"scope"`
	TargetRoot       string            `json:"target_root"`
	ProfileID        string            `json:"profile_id,omitempty"`
	TargetID         string            `json:"target_id,omitempty"`
	SessionFilter    string            `json:"session_filter,omitempty"`
	LatestSessionID  string            `json:"latest_session_id,omitempty"`
	PruneReview      PruneReview       `json:"prune_review"`
	ArtifactProblems []ArtifactProblem `json:"artifact_problems,omitempty"`
}

type Overall struct {
	Scope  string   `json:"scope"`
	Status Status   `json:"status"`
	Issues []string `json:"issues,omitempty"`
}

type Summary struct {
	ManifestCount           int `json:"manifest_count"`
	ManifestEntries         int `json:"manifest_entries"`
	FilesExpected           int `json:"files_expected"`
	FilesVerified           int `json:"files_verified"`
	VerificationErrors      int `json:"verification_errors"`
	VerificationWarnings    int `json:"verification_warnings"`
	Warnings                int `json:"warnings"`
	ProfileSuggestions      int `json:"profile_suggestions"`
	SoftDeletes             int `json:"soft_deletes"`
	PruneCandidates         int `json:"prune_candidates"`
	PruneRefusals           int `json:"prune_refusals"`
	PruneApprovals          int `json:"prune_approvals"`
	PruneUnappliedApprovals int `json:"prune_unapplied_approvals"`
	PruneActiveApprovals    int `json:"prune_active_approvals"`
	PruneStaleApprovals     int `json:"prune_stale_approvals"`
	PruneExpiredApprovals   int `json:"prune_expired_approvals"`
	PruneConsumedApprovals  int `json:"prune_consumed_approvals"`
	PruneReceipts           int `json:"prune_receipts"`
	PruneReceiptIssues      int `json:"prune_receipt_issues"`
	PruneArtifactProblems   int `json:"prune_artifact_problems"`
	TargetDrifts            int `json:"target_drifts"`
	LiveTargetDrifts        int `json:"live_target_drifts"`
	LiveTargetDriftProblems int `json:"live_target_drift_artifact_problems"`
	RecoveryIssues          int `json:"recovery_issues"`
	InvalidHealthRecords    int `json:"invalid_health_records"`
	ArtifactProblems        int `json:"artifact_problems"`
	PairingIssues           int `json:"pairing_issues"`
	NetworkTransfers        int `json:"network_transfers"`
}

type PairingStatus string

const (
	PairingStatusProfileUnavailable PairingStatus = "profile_unavailable"
	PairingStatusUnpaired           PairingStatus = "unpaired"
	PairingStatusValid              PairingStatus = "paired_receipt_valid"
	PairingStatusProfileInvalid     PairingStatus = "profile_invalid"
	PairingStatusTargetMissing      PairingStatus = "paired_target_missing"
	PairingStatusReceiptMissing     PairingStatus = "paired_receipt_missing"
	PairingStatusReceiptInvalid     PairingStatus = "paired_receipt_invalid"
	PairingStatusMismatch           PairingStatus = "paired_receipt_mismatch"
)

type PruneReviewStatus string

const (
	PruneReviewProfileUnavailable PruneReviewStatus = "profile_unavailable"
	PruneReviewPolicyNotEnabled   PruneReviewStatus = "policy_not_enabled"
	PruneReviewNoPendingReview    PruneReviewStatus = "no_pending_review"
	PruneReviewReviewRequired     PruneReviewStatus = "review_required"
	PruneReviewReceiptAttention   PruneReviewStatus = "receipt_attention"
	PruneReviewArtifactProblems   PruneReviewStatus = "artifact_problems"
)

type PruneReview struct {
	Status              PruneReviewStatus          `json:"status"`
	DryRun              bool                       `json:"dry_run"`
	ApprovalRequired    bool                       `json:"approval_required"`
	ApprovalAuthoring   string                     `json:"approval_authoring"`
	PhysicalPruning     string                     `json:"physical_pruning"`
	Apply               string                     `json:"apply"`
	ApprovalSource      string                     `json:"approval_source"`
	ReceiptSource       string                     `json:"receipt_source"`
	ProfileDeletePolicy *prune.ProfileDeletePolicy `json:"profile_delete_policy,omitempty"`
	Summary             PruneReviewSummary         `json:"summary"`
	Candidates          []prune.Candidate          `json:"candidates,omitempty"`
	Refusals            []prune.Refusal            `json:"refusals,omitempty"`
	Approvals           []PruneApproval            `json:"approvals,omitempty"`
	Receipts            []PruneReceipt             `json:"receipts,omitempty"`
}

type PruneReviewSummary struct {
	SoftDeletes        int `json:"soft_deletes"`
	Candidates         int `json:"candidates"`
	Refusals           int `json:"refusals"`
	Approvals          int `json:"approvals"`
	UnappliedApprovals int `json:"unapplied_approvals"`
	ActiveApprovals    int `json:"active_approvals"`
	StaleApprovals     int `json:"stale_approvals"`
	ExpiredApprovals   int `json:"expired_approvals"`
	ConsumedApprovals  int `json:"consumed_approvals"`
	Receipts           int `json:"receipts"`
	ReceiptIssues      int `json:"receipt_issues"`
	ArtifactProblems   int `json:"artifact_problems"`
}

type PruneApproval struct {
	Path                  string                      `json:"path"`
	Action                string                      `json:"action"`
	PhysicalPruning       string                      `json:"physical_pruning"`
	Unapplied             bool                        `json:"unapplied"`
	ReleaseState          string                      `json:"release_state"`
	ReleaseBlocker        bool                        `json:"release_blocker"`
	ReleaseReason         string                      `json:"release_reason,omitempty"`
	ReleaseAction         string                      `json:"release_action"`
	CurrentEvidence       []PruneApprovalItemEvidence `json:"current_evidence,omitempty"`
	Version               int                         `json:"version"`
	ID                    string                      `json:"id"`
	ProfileID             string                      `json:"profile_id"`
	TargetID              string                      `json:"target_id"`
	RootID                string                      `json:"root_id"`
	CreatedAt             string                      `json:"created_at"`
	ApprovedBy            string                      `json:"approved_by,omitempty"`
	ApprovedAt            string                      `json:"approved_at,omitempty"`
	ReviewTool            string                      `json:"review_tool"`
	ProfileSnapshotID     string                      `json:"profile_snapshot_id,omitempty"`
	ProfileSnapshotPath   string                      `json:"profile_snapshot_path,omitempty"`
	ProfileSnapshotDigest string                      `json:"profile_snapshot_digest,omitempty"`
	ProfileDeletePolicy   control.PruneDeletePolicy   `json:"profile_delete_policy"`
	Items                 []control.PruneApprovalItem `json:"items,omitempty"`
	ApprovalScopeDigest   string                      `json:"approval_scope_digest,omitempty"`
	ExpiresAt             string                      `json:"expires_at,omitempty"`
	Status                string                      `json:"status"`
	ApprovalReason        string                      `json:"approval_reason,omitempty"`
	RefusalReason         string                      `json:"refusal_reason,omitempty"`
	SupersededBy          string                      `json:"superseded_by,omitempty"`
	SupersededAt          string                      `json:"superseded_at,omitempty"`
	LinkedReceiptID       string                      `json:"linked_receipt_id,omitempty"`
	LinkedReceiptStatus   control.PruneReceiptStatus  `json:"linked_receipt_status,omitempty"`
}

type PruneApprovalItemEvidence struct {
	SoftDeleteID string `json:"soft_delete_id"`
	TargetPath   string `json:"target_path"`
	State        string `json:"state"`
	Reason       string `json:"reason,omitempty"`
	ReasonCode   string `json:"reason_code,omitempty"`
}

type PruneReceipt struct {
	Path                string                     `json:"path"`
	Action              string                     `json:"action"`
	Version             int                        `json:"version"`
	ID                  string                     `json:"id"`
	PruneSessionID      string                     `json:"prune_session_id"`
	ApprovalID          string                     `json:"approval_id"`
	ProfileID           string                     `json:"profile_id"`
	TargetID            string                     `json:"target_id"`
	StartedAt           string                     `json:"started_at"`
	EndedAt             string                     `json:"ended_at,omitempty"`
	Status              control.PruneReceiptStatus `json:"status"`
	DryRun              bool                       `json:"dry_run"`
	ApprovalScopeDigest string                     `json:"approval_scope_digest"`
	Items               []control.PruneReceiptItem `json:"items"`
	Refusals            []control.PruneRefusal     `json:"refusals,omitempty"`
}

type PairingState struct {
	Status            PairingStatus `json:"status"`
	ReceiptID         string        `json:"receipt_id,omitempty"`
	TargetDeviceID    string        `json:"target_device_id,omitempty"`
	PairedAt          string        `json:"paired_at,omitempty"`
	Method            string        `json:"method,omitempty"`
	VerifiedAt        string        `json:"verified_at,omitempty"`
	Evidence          string        `json:"evidence,omitempty"`
	EncryptedTransfer string        `json:"encrypted_transfer"`
	Issue             string        `json:"issue,omitempty"`
}

type PrivacyState struct {
	Status              string          `json:"status"`
	Mode                string          `json:"mode,omitempty"`
	TrafficLevel        int             `json:"traffic_level,omitempty"`
	PaddingBucketBytes  int             `json:"padding_bucket_bytes,omitempty"`
	BatchMaxBytes       int             `json:"batch_max_bytes,omitempty"`
	BatchMaxCount       int             `json:"batch_max_count,omitempty"`
	JitterBudgetMillis  int             `json:"jitter_budget_millis,omitempty"`
	DiscoveryLowInfo    bool            `json:"discovery_low_info,omitempty"`
	Claim               string          `json:"claim"`
	LocalPush           string          `json:"local_push"`
	NetworkTransfer     string          `json:"network_transfer"`
	ResidualLeakage     []string        `json:"residual_leakage"`
	ConfiguredReduction []string        `json:"configured_reductions,omitempty"`
	Overhead            PrivacyOverhead `json:"overhead"`
}

type PrivacyOverhead struct {
	Status             string `json:"status"`
	Source             string `json:"source"`
	PaddingBucketBytes int    `json:"padding_bucket_bytes,omitempty"`
	BatchMaxBytes      int    `json:"batch_max_bytes,omitempty"`
	BatchMaxCount      int    `json:"batch_max_count,omitempty"`
	JitterBudgetMillis int    `json:"jitter_budget_millis,omitempty"`
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

type LatestSession struct {
	ID           string       `json:"id,omitempty"`
	ManifestID   string       `json:"manifest_id,omitempty"`
	CreatedAt    string       `json:"created_at,omitempty"`
	Entries      int          `json:"entries"`
	Files        int          `json:"files"`
	Completeness Completeness `json:"completeness"`
}

type Completeness struct {
	Status               CompletenessStatus `json:"status"`
	FilesExpected        int                `json:"files_expected"`
	FilesVerified        int                `json:"files_verified"`
	VerificationErrors   int                `json:"verification_errors"`
	VerificationWarnings int                `json:"verification_warnings"`
}

type ProfileSuggestion struct {
	WarningID             string            `json:"warning_id"`
	SessionID             string            `json:"session_id,omitempty"`
	Code                  string            `json:"code"`
	Severity              string            `json:"severity,omitempty"`
	Message               string            `json:"message"`
	Paths                 []string          `json:"paths,omitempty"`
	TargetPath            string            `json:"target_path,omitempty"`
	SuggestedProfilePatch map[string]string `json:"suggested_profile_patch,omitempty"`
	SuggestedConfig       map[string]string `json:"suggested_config,omitempty"`
	CreatedAt             string            `json:"created_at,omitempty"`
}

type Health struct {
	Healthy        bool              `json:"healthy"`
	Summary        HealthSummary     `json:"summary"`
	RecoveryIssues []RecoveryIssue   `json:"recovery_issues,omitempty"`
	InvalidRecords []InvalidRecord   `json:"invalid_records,omitempty"`
	ArtifactIssues []ArtifactProblem `json:"artifact_issues,omitempty"`
	Transfers      []NetworkTransfer `json:"network_transfers,omitempty"`
}

type HealthSummary struct {
	IncompleteSessions int `json:"incomplete_sessions"`
	InvalidRecords     int `json:"invalid_records"`
	ArtifactProblems   int `json:"artifact_problems"`
	TargetDrifts       int `json:"target_drifts"`
	NetworkTransfers   int `json:"network_transfers"`
}

type RecoveryIssue struct {
	SessionID string `json:"session_id"`
	State     string `json:"state"`
	Action    string `json:"action"`
	Reason    string `json:"reason"`
	Path      string `json:"path"`
	UpdatedAt string `json:"updated_at"`
}

type InvalidRecord struct {
	SessionID string `json:"session_id,omitempty"`
	Path      string `json:"path"`
	Error     string `json:"error"`
}

type ArtifactProblem struct {
	Source    string `json:"source,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Path      string `json:"path"`
	Error     string `json:"error"`
}

type NetworkTransfer struct {
	SessionID string                                  `json:"session_id"`
	ProfileID string                                  `json:"profile_id,omitempty"`
	TargetID  string                                  `json:"target_id,omitempty"`
	Status    string                                  `json:"status"`
	Stage     string                                  `json:"stage,omitempty"`
	ErrorCode string                                  `json:"error_code,omitempty"`
	Error     string                                  `json:"error,omitempty"`
	Path      string                                  `json:"path"`
	UpdatedAt string                                  `json:"updated_at,omitempty"`
	Action    string                                  `json:"action"`
	Privacy   transport.PrivacyPolicy                 `json:"privacy_policy,omitempty"`
	Overhead  *control.NetworkTransferPrivacyOverhead `json:"privacy_overhead,omitempty"`
}

type ProfileSnapshot struct {
	ID         string       `json:"id"`
	SessionID  string       `json:"session_id,omitempty"`
	ProfileID  string       `json:"profile_id"`
	CapturedAt string       `json:"captured_at"`
	Path       string       `json:"path"`
	Privacy    PrivacyState `json:"privacy"`
}

type LiveTargetDrift struct {
	Source           string                   `json:"source"`
	Durable          bool                     `json:"durable"`
	SessionID        string                   `json:"session_id,omitempty"`
	Manifest         verify.ManifestSummary   `json:"manifest"`
	Summary          LiveTargetDriftSummary   `json:"summary"`
	TargetDrifts     []control.TargetDrift    `json:"target_drifts,omitempty"`
	ArtifactProblems []ArtifactProblem        `json:"artifact_problems,omitempty"`
	Manifests        []verify.ManifestSummary `json:"manifests,omitempty"`
}

type LiveTargetDriftSummary struct {
	ManifestCount    int `json:"manifest_count"`
	ManifestEntries  int `json:"manifest_entries"`
	TargetDrifts     int `json:"target_drifts"`
	ArtifactProblems int `json:"artifact_problems"`
}

func BuildReport(opts Options) (Report, error) {
	targetRoot := strings.TrimSpace(opts.TargetRoot)
	if targetRoot == "" {
		return Report{}, errors.New("target root is required")
	}
	absRoot, err := filepath.Abs(targetRoot)
	if err != nil {
		return Report{}, err
	}
	profileID, targetID, err := normalizeScope(opts, absRoot)
	if err != nil {
		return Report{}, err
	}

	healthReport, err := health.BuildReport(health.Options{
		TargetRoot: absRoot,
		ProfileID:  profileID,
		TargetID:   targetID,
	})
	if err != nil {
		return Report{}, err
	}
	verifyReport, err := verify.BuildReport(verify.Options{
		TargetRoot: absRoot,
		SessionID:  opts.SessionID,
		ProfileID:  profileID,
		TargetID:   targetID,
	})
	if err != nil {
		return Report{}, err
	}
	liveDriftSessionID := opts.SessionID
	if liveDriftSessionID == "" {
		liveDriftSessionID = verifyReport.Manifest.SessionID
	}
	liveDriftReport, liveDriftErr := verify.DetectTargetDrift(verify.DriftOptions{
		TargetRoot: absRoot,
		SessionID:  liveDriftSessionID,
		ProfileID:  profileID,
		TargetID:   targetID,
	})
	liveTargetDrift := summarizeLiveTargetDrift(liveDriftReport, liveDriftErr, absRoot, liveDriftSessionID)

	filterSessionID := opts.SessionID
	displaySessionID := opts.SessionID
	if displaySessionID == "" {
		displaySessionID = verifyReport.Manifest.SessionID
	}

	healthView := summarizeHealth(healthReport, filterSessionID, verifyReport.Summary.TargetDrifts)
	snapshots, snapshotProblems := readProfileSnapshots(absRoot, verifyReport.Manifests, profileID, targetID, filterSessionID)
	scopeProblems := readForeignPublishedReceipts(absRoot, profileID, targetID, opts.SessionID, verifyReport.Summary.ManifestCount)
	profileSuggestions := collectProfileSuggestions(verifyReport.Warnings)
	pairingState, pairingProblems := evaluatePairing(opts.Profile)
	pruneReview, pruneProblems := buildPruneReview(absRoot, profileID, targetID, filterSessionID, opts.Profile, verifyReport.SoftDeletes)
	artifactProblems := mergeArtifactProblems(filterSessionID, healthView.ArtifactIssues, verifyReport.ArtifactProblems, snapshotProblems, scopeProblems, pairingProblems, pruneProblems)

	privacyState := PrivacyForProfile(opts.Profile)
	report := Report{
		Scope:                ScopeLocalMigrationTarget,
		TargetRoot:           verifyReport.TargetRoot,
		ProfileID:            profileID,
		TargetID:             targetID,
		SessionID:            displaySessionID,
		Overall:              Overall{Scope: ScopeLocalMigrationTarget},
		Summary:              summarizeVerify(verifyReport, healthView, len(profileSuggestions), len(artifactProblems), len(pairingProblems)),
		LatestSession:        summarizeLatestSession(verifyReport),
		Warnings:             copyWarnings(verifyReport.Warnings),
		ProfileSuggestions:   profileSuggestions,
		SoftDeletes:          append([]control.SoftDelete(nil), verifyReport.SoftDeletes...),
		PruneReview:          pruneReview,
		TargetDrifts:         copyTargetDrifts(verifyReport.TargetDrifts),
		LiveTargetDrift:      liveTargetDrift,
		NetworkTransfers:     append([]NetworkTransfer(nil), healthView.Transfers...),
		Pairing:              pairingState,
		Privacy:              privacyState,
		TrafficPrivacy:       trafficPrivacyAcceptance(absRoot, profileID, targetID, filterSessionID, privacyState, opts.Profile),
		Health:               healthView,
		ArtifactProblems:     artifactProblems,
		ProfileSnapshots:     snapshots,
		VerificationFindings: append([]verify.Finding(nil), verifyReport.Findings...),
	}
	report.Summary.LiveTargetDrifts = report.LiveTargetDrift.Summary.TargetDrifts
	report.Summary.LiveTargetDriftProblems = report.LiveTargetDrift.Summary.ArtifactProblems
	report.Summary.PruneCandidates = report.PruneReview.Summary.Candidates
	report.Summary.PruneRefusals = report.PruneReview.Summary.Refusals
	report.Summary.PruneApprovals = report.PruneReview.Summary.Approvals
	report.Summary.PruneUnappliedApprovals = report.PruneReview.Summary.UnappliedApprovals
	report.Summary.PruneActiveApprovals = report.PruneReview.Summary.ActiveApprovals
	report.Summary.PruneStaleApprovals = report.PruneReview.Summary.StaleApprovals
	report.Summary.PruneExpiredApprovals = report.PruneReview.Summary.ExpiredApprovals
	report.Summary.PruneConsumedApprovals = report.PruneReview.Summary.ConsumedApprovals
	report.Summary.PruneReceipts = report.PruneReview.Summary.Receipts
	report.Summary.PruneReceiptIssues = report.PruneReview.Summary.ReceiptIssues
	report.Summary.PruneArtifactProblems = report.PruneReview.Summary.ArtifactProblems
	report.Summary.ArtifactProblems = len(report.ArtifactProblems)
	report.Overall.Status = classify(report)
	report.Overall.Issues = summarizeIssues(report)
	return report, nil
}

func BuildPruneReview(opts Options) (PruneReviewReport, error) {
	targetRoot := strings.TrimSpace(opts.TargetRoot)
	if targetRoot == "" {
		return PruneReviewReport{}, errors.New("target root is required")
	}
	absRoot, err := filepath.Abs(targetRoot)
	if err != nil {
		return PruneReviewReport{}, err
	}
	profileID, targetID, err := normalizeScope(opts, absRoot)
	if err != nil {
		return PruneReviewReport{}, err
	}
	verifyReport, err := verify.BuildReport(verify.Options{
		TargetRoot: absRoot,
		SessionID:  opts.SessionID,
		ProfileID:  profileID,
		TargetID:   targetID,
	})
	if err != nil {
		return PruneReviewReport{}, err
	}
	pruneReview, pruneProblems := buildPruneReview(absRoot, profileID, targetID, opts.SessionID, opts.Profile, verifyReport.SoftDeletes)
	return PruneReviewReport{
		Scope:            ScopeLocalMigrationTarget,
		TargetRoot:       verifyReport.TargetRoot,
		ProfileID:        profileID,
		TargetID:         targetID,
		SessionFilter:    opts.SessionID,
		LatestSessionID:  verifyReport.Manifest.SessionID,
		PruneReview:      pruneReview,
		ArtifactProblems: pruneProblems,
	}, nil
}

func normalizeScope(opts Options, absRoot string) (string, string, error) {
	profileID := strings.TrimSpace(opts.ProfileID)
	targetID := strings.TrimSpace(opts.TargetID)
	if opts.Profile == nil {
		return profileID, targetID, nil
	}
	if profileID != "" && opts.Profile.ProfileID != profileID {
		return "", "", fmt.Errorf("profile_id %q does not match profile %q", profileID, opts.Profile.ProfileID)
	}
	if targetID != "" && opts.Profile.Target.TargetID != targetID {
		return "", "", fmt.Errorf("target_id %q does not match profile target %q", targetID, opts.Profile.Target.TargetID)
	}
	profileTargetRoot, err := filepath.Abs(opts.Profile.Target.LocalPath)
	if err != nil {
		return "", "", err
	}
	if filepath.Clean(profileTargetRoot) != filepath.Clean(absRoot) {
		return "", "", fmt.Errorf("target root %q does not match profile target.local_path %q", absRoot, profileTargetRoot)
	}
	return opts.Profile.ProfileID, opts.Profile.Target.TargetID, nil
}

func summarizeVerify(report verify.Report, health Health, suggestionCount int, artifactProblems int, pairingIssues int) Summary {
	return Summary{
		ManifestCount:        report.Summary.ManifestCount,
		ManifestEntries:      report.Summary.ManifestEntries,
		FilesExpected:        report.Summary.FilesExpected,
		FilesVerified:        report.Summary.FilesVerified,
		VerificationErrors:   report.Summary.ErrorFindings,
		VerificationWarnings: report.Summary.WarningFindings,
		Warnings:             report.Summary.Warnings,
		ProfileSuggestions:   suggestionCount,
		SoftDeletes:          report.Summary.SoftDeletes,
		TargetDrifts:         report.Summary.TargetDrifts,
		RecoveryIssues:       len(health.RecoveryIssues),
		InvalidHealthRecords: len(health.InvalidRecords),
		ArtifactProblems:     artifactProblems,
		PairingIssues:        pairingIssues,
		NetworkTransfers:     len(health.Transfers),
	}
}

func buildPruneReview(targetRoot string, profileID string, targetID string, sessionID string, p *profile.Profile, softDeletes []control.SoftDelete) (PruneReview, []ArtifactProblem) {
	review := PruneReview{
		Status:            PruneReviewProfileUnavailable,
		DryRun:            true,
		ApprovalRequired:  true,
		ApprovalAuthoring: "approval_artifact_authoring_wired",
		PhysicalPruning:   "not_applied",
		Apply:             "existing_approval_apply_wired",
		ApprovalSource:    "control_plane_prune_approvals",
		ReceiptSource:     "control_plane_prune_receipts",
		Summary: PruneReviewSummary{
			SoftDeletes: len(softDeletes),
		},
	}

	softDeleteIDs := softDeleteIDSet(softDeletes)
	receipts, receiptProblems := readPruneReceipts(targetRoot, profileID, targetID, sessionID, softDeleteIDs)
	review.Receipts = receipts
	review.Summary.Receipts = len(receipts)
	review.Summary.ReceiptIssues = countPruneReceiptIssues(receipts)
	approvals, approvalProblems := readPruneApprovals(targetRoot, profileID, targetID, sessionID, softDeleteIDs, appliedPruneReceipts(receipts))
	review.Approvals = approvals
	review.Summary.Approvals = len(approvals)
	updatePruneApprovalSummary(&review)
	resolved := resolvedPruneSoftDeletes(receipts)

	var problems []ArtifactProblem
	problems = append(problems, receiptProblems...)
	problems = append(problems, approvalProblems...)
	if p == nil {
		review.Summary.ArtifactProblems = len(problems)
		review.Status = pruneReviewStatus(review)
		return review, problems
	}

	policy := prune.ProfileDeletePolicy{
		Mode:               string(p.DeletePolicy.Mode),
		RequireReview:      p.DeletePolicy.RequireReview,
		RetentionDays:      p.DeletePolicy.RetentionDays,
		AllowPhysicalPrune: p.DeletePolicy.AllowPhysicalPrune,
	}
	review.ProfileDeletePolicy = &policy
	if !prunePolicyEnabled(p.DeletePolicy) {
		review.Status = PruneReviewPolicyNotEnabled
		review.Summary.ArtifactProblems = len(problems)
		review.Status = pruneReviewStatus(review)
		return review, problems
	}
	if !targetControlPlanePresent(targetRoot) {
		review.Status = pruneReviewStatus(review)
		review.Summary.ArtifactProblems = len(problems)
		return review, problems
	}

	dryRun, err := prune.PlanDryRun(prune.Options{
		TargetRoot:   targetRoot,
		ProfileID:    profileID,
		TargetID:     targetID,
		SessionID:    sessionID,
		DeletePolicy: p.DeletePolicy,
	})
	if err != nil {
		problems = append(problems, ArtifactProblem{
			Source:    "prune_review",
			SessionID: sessionID,
			Path:      control.ControlDir(targetRoot),
			Error:     err.Error(),
		})
		review.Summary.ArtifactProblems = len(problems)
		review.Status = pruneReviewStatus(review)
		return review, problems
	}
	for _, problem := range dryRun.ArtifactProblems {
		problems = append(problems, ArtifactProblem{
			Source:    "prune_review",
			SessionID: problem.SessionID,
			Path:      problem.Path,
			Error:     problem.Error,
		})
	}
	review.Candidates = append([]prune.Candidate(nil), dryRun.Candidates...)
	review.Refusals = filterResolvedTargetMissingPruneRefusals(dryRun.Refusals, resolved)
	classifyPruneApprovals(&review, dryRun)
	review.Summary.Candidates = len(review.Candidates)
	review.Summary.Refusals = len(review.Refusals)
	updatePruneApprovalSummary(&review)
	review.Summary.ArtifactProblems = len(problems)
	review.Status = pruneReviewStatus(review)
	return review, problems
}

func prunePolicyEnabled(policy profile.DeletePolicy) bool {
	return policy.Mode == profile.DeleteModePrune && policy.RequireReview && policy.AllowPhysicalPrune
}

func targetControlPlanePresent(targetRoot string) bool {
	info, err := os.Lstat(control.ControlDir(targetRoot))
	if err != nil {
		return false
	}
	return info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

func pruneReviewStatus(review PruneReview) PruneReviewStatus {
	switch {
	case review.Summary.ArtifactProblems > 0:
		return PruneReviewArtifactProblems
	case review.Summary.Candidates > 0 || review.Summary.Refusals > 0 || review.Summary.UnappliedApprovals > 0 || review.Summary.StaleApprovals > 0 || review.Summary.ExpiredApprovals > 0:
		return PruneReviewReviewRequired
	case review.Summary.ReceiptIssues > 0:
		return PruneReviewReceiptAttention
	case review.Status == PruneReviewPolicyNotEnabled:
		return PruneReviewPolicyNotEnabled
	case review.Status == PruneReviewProfileUnavailable && review.ProfileDeletePolicy == nil:
		return PruneReviewProfileUnavailable
	default:
		return PruneReviewNoPendingReview
	}
}

func (review PruneReview) NeedsReview() bool {
	switch review.Status {
	case PruneReviewReviewRequired,
		PruneReviewReceiptAttention,
		PruneReviewArtifactProblems,
		PruneReviewProfileUnavailable:
		return true
	default:
		return false
	}
}

func (review PruneReview) ReviewAction() string {
	switch review.Status {
	case PruneReviewReviewRequired:
		return "inspect_prune_review_before_release"
	case PruneReviewReceiptAttention:
		return "inspect_prune_receipts_before_release"
	case PruneReviewArtifactProblems:
		return "inspect_prune_artifact_problems_before_release"
	case PruneReviewProfileUnavailable:
		return "provide_profile_before_release_review"
	case PruneReviewPolicyNotEnabled:
		return "prune_policy_not_enabled"
	default:
		return "no_prune_review_action_required"
	}
}

func softDeleteIDSet(records []control.SoftDelete) map[string]struct{} {
	out := make(map[string]struct{}, len(records))
	for _, record := range records {
		out[record.ID] = struct{}{}
	}
	return out
}

func readPruneReceipts(targetRoot string, profileID string, targetID string, sessionID string, softDeleteIDs map[string]struct{}) ([]PruneReceipt, []ArtifactProblem) {
	dir := filepath.Join(control.ControlDir(targetRoot), "prune", "receipts")
	if err := pathguard.EnsureDirectory(targetRoot, dir); err != nil {
		return nil, []ArtifactProblem{{Source: "prune_receipt", Path: dir, Error: err.Error()}}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, []ArtifactProblem{{Source: "prune_receipt", Path: dir, Error: err.Error()}}
	}

	var receipts []PruneReceipt
	var problems []ArtifactProblem
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			problems = append(problems, ArtifactProblem{
				Source: "prune_receipt",
				Path:   path,
				Error:  "prune receipt is not a regular file",
			})
			continue
		}
		receipt, err := control.ReadFileNoSymlinkUnderRoot[control.PruneReceipt](targetRoot, path)
		if err != nil {
			problems = append(problems, ArtifactProblem{
				Source: "prune_receipt",
				Path:   path,
				Error:  err.Error(),
			})
			continue
		}
		pathID := strings.TrimSuffix(entry.Name(), ".json")
		if receipt.ID != pathID {
			problems = append(problems, ArtifactProblem{
				Source: "prune_receipt",
				Path:   path,
				Error:  fmt.Sprintf("prune receipt id %q does not match path id %q", receipt.ID, pathID),
			})
			continue
		}
		if strings.TrimSpace(profileID) != "" && receipt.ProfileID != profileID {
			continue
		}
		if strings.TrimSpace(targetID) != "" && receipt.TargetID != targetID {
			continue
		}
		if sessionID != "" {
			receipt = filterPruneReceiptForSession(receipt, softDeleteIDs)
			if len(receipt.Items) == 0 && len(receipt.Refusals) == 0 {
				continue
			}
		}
		receipts = append(receipts, viewPruneReceipt(path, receipt))
	}
	sort.Slice(receipts, func(i, j int) bool {
		if receipts[i].StartedAt == receipts[j].StartedAt {
			return receipts[i].ID < receipts[j].ID
		}
		return receipts[i].StartedAt < receipts[j].StartedAt
	})
	return receipts, problems
}

type appliedPruneReceipt struct {
	ID        string
	Status    control.PruneReceiptStatus
	StartedAt string
}

func readPruneApprovals(targetRoot string, profileID string, targetID string, sessionID string, softDeleteIDs map[string]struct{}, appliedReceipts map[string]appliedPruneReceipt) ([]PruneApproval, []ArtifactProblem) {
	dir := filepath.Join(control.ControlDir(targetRoot), "prune", "approvals")
	if err := pathguard.EnsureDirectory(targetRoot, dir); err != nil {
		return nil, []ArtifactProblem{{Source: "prune_approval", Path: dir, Error: err.Error()}}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, []ArtifactProblem{{Source: "prune_approval", Path: dir, Error: err.Error()}}
	}

	var approvals []PruneApproval
	var problems []ArtifactProblem
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			problems = append(problems, ArtifactProblem{
				Source: "prune_approval",
				Path:   path,
				Error:  "prune approval is not a regular file",
			})
			continue
		}
		scope, err := readPruneApprovalScope(targetRoot, path)
		if err != nil {
			problems = append(problems, ArtifactProblem{
				Source: "prune_approval",
				Path:   path,
				Error:  err.Error(),
			})
			continue
		}
		if scope.excludedBy(profileID, targetID) {
			continue
		}
		approval, err := control.ReadFileNoSymlinkUnderRoot[control.PruneApproval](targetRoot, path)
		if err != nil {
			problems = append(problems, ArtifactProblem{
				Source: "prune_approval",
				Path:   path,
				Error:  err.Error(),
			})
			continue
		}
		pathID := strings.TrimSuffix(entry.Name(), ".json")
		if approval.ID != pathID {
			problems = append(problems, ArtifactProblem{
				Source: "prune_approval",
				Path:   path,
				Error:  fmt.Sprintf("prune approval id %q does not match path id %q", approval.ID, pathID),
			})
			continue
		}
		if strings.TrimSpace(profileID) != "" && approval.ProfileID != profileID {
			continue
		}
		if strings.TrimSpace(targetID) != "" && approval.TargetID != targetID {
			continue
		}
		if sessionID != "" {
			approval = filterPruneApprovalForSession(approval, softDeleteIDs)
			if len(approval.Items) == 0 {
				continue
			}
		}
		approvals = append(approvals, viewPruneApproval(targetRoot, path, approval, appliedReceipts))
	}
	sort.Slice(approvals, func(i, j int) bool {
		left := approvalSortTime(approvals[i])
		right := approvalSortTime(approvals[j])
		if left == right {
			return approvals[i].ID < approvals[j].ID
		}
		return left < right
	})
	return approvals, problems
}

type pruneApprovalScopeDocument map[string]json.RawMessage

func (d pruneApprovalScopeDocument) Validate() error {
	return nil
}

type pruneApprovalScope struct {
	profileID    string
	profileKnown bool
	targetID     string
	targetKnown  bool
}

func readPruneApprovalScope(targetRoot string, path string) (pruneApprovalScope, error) {
	doc, err := control.ReadFileNoSymlinkUnderRoot[pruneApprovalScopeDocument](targetRoot, path)
	if err != nil {
		return pruneApprovalScope{}, err
	}
	profileID, profileKnown := rawOptionalJSONString(doc, "profile_id")
	targetID, targetKnown := rawOptionalJSONString(doc, "target_id")
	return pruneApprovalScope{
		profileID:    profileID,
		profileKnown: profileKnown,
		targetID:     targetID,
		targetKnown:  targetKnown,
	}, nil
}

func rawOptionalJSONString(doc pruneApprovalScopeDocument, field string) (string, bool) {
	raw, ok := doc[field]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func (scope pruneApprovalScope) excludedBy(profileID string, targetID string) bool {
	if strings.TrimSpace(profileID) != "" && scope.profileKnown && scope.profileID != profileID {
		return true
	}
	if strings.TrimSpace(targetID) != "" && scope.targetKnown && scope.targetID != targetID {
		return true
	}
	return false
}

func filterPruneApprovalForSession(approval control.PruneApproval, softDeleteIDs map[string]struct{}) control.PruneApproval {
	filtered := approval
	filtered.Items = nil
	for _, item := range approval.Items {
		if _, ok := softDeleteIDs[item.SoftDeleteID]; ok {
			filtered.Items = append(filtered.Items, item)
		}
	}
	return filtered
}

func viewPruneApproval(targetRoot string, path string, approval control.PruneApproval, linkedReceipts map[string]appliedPruneReceipt) PruneApproval {
	out := PruneApproval{
		Path:                  filepath.ToSlash(path),
		Action:                "inspect_prune_approval",
		PhysicalPruning:       "not_applied",
		ReleaseState:          "not_classified",
		ReleaseAction:         "inspect_prune_approval",
		Version:               approval.Version,
		ID:                    approval.ID,
		ProfileID:             approval.ProfileID,
		TargetID:              approval.TargetID,
		RootID:                approval.RootID,
		CreatedAt:             approval.CreatedAt,
		ApprovedBy:            approval.ApprovedBy,
		ApprovedAt:            approval.ApprovedAt,
		ReviewTool:            approval.ReviewTool,
		ProfileSnapshotID:     approval.ProfileSnapshotID,
		ProfileSnapshotDigest: approval.ProfileSnapshotDigest,
		ProfileDeletePolicy:   approval.ProfileDeletePolicy,
		Items:                 append([]control.PruneApprovalItem(nil), approval.Items...),
		ApprovalScopeDigest:   approval.ApprovalScopeDigest,
		ExpiresAt:             approval.ExpiresAt,
		Status:                approval.Status,
		ApprovalReason:        approval.ApprovalReason,
		RefusalReason:         approval.RefusalReason,
		SupersededBy:          approval.SupersededBy,
		SupersededAt:          approval.SupersededAt,
	}
	if approval.ProfileSnapshotID != "" {
		if snapshotPath, err := control.Path(targetRoot, control.ArtifactProfileSnapshot, approval.ProfileSnapshotID); err == nil {
			out.ProfileSnapshotPath = filepath.ToSlash(snapshotPath)
		}
	}
	if linked, ok := linkedReceipts[pruneApprovalReceiptKey(approval.ID, approval.ApprovalScopeDigest)]; ok {
		out.LinkedReceiptID = linked.ID
		out.LinkedReceiptStatus = linked.Status
		out.Action = "inspect_prune_receipt"
		switch linked.Status {
		case control.PruneReceiptApplied, control.PruneReceiptPartial:
			out.ReleaseState = "consumed"
			out.ReleaseReason = "approval has applied or partial prune receipt evidence"
			out.ReleaseAction = "inspect_prune_receipt"
			return out
		case control.PruneReceiptStarted:
			out.ReleaseState = "pending_receipt"
			out.ReleaseReason = "approval has started prune receipt evidence that still requires operator review"
			out.ReleaseAction = "inspect_prune_receipt"
			return out
		case control.PruneReceiptFailed:
			out.ReleaseState = "failed_receipt"
			out.ReleaseReason = "approval has failed prune receipt evidence that requires operator review"
			out.ReleaseAction = "inspect_prune_receipt"
			return out
		default:
			out.ReleaseState = "receipt_attention"
			out.ReleaseReason = "approval has prune receipt evidence that requires operator review"
			out.ReleaseAction = "inspect_prune_receipt"
			return out
		}
	}
	if approval.Status == "superseded" {
		out.ReleaseState = "superseded"
		out.ReleaseReason = approval.RefusalReason
		out.ReleaseAction = "inspect_superseded_prune_approval"
		return out
	}
	if approval.Status == "approved" {
		out.Unapplied = true
		out.Action = "inspect_prune_approval_before_apply"
		out.ReleaseState = "pending_current_evidence"
		out.ReleaseAction = "inspect_prune_approval_before_apply"
	}
	return out
}

func approvalSortTime(approval PruneApproval) string {
	if approval.ApprovedAt != "" {
		return approval.ApprovedAt
	}
	return approval.CreatedAt
}

func appliedPruneReceipts(receipts []PruneReceipt) map[string]appliedPruneReceipt {
	out := map[string]appliedPruneReceipt{}
	for _, receipt := range receipts {
		key := pruneApprovalReceiptKey(receipt.ApprovalID, receipt.ApprovalScopeDigest)
		current := appliedPruneReceipt{
			ID:        receipt.ID,
			Status:    receipt.Status,
			StartedAt: receipt.StartedAt,
		}
		existing, ok := out[key]
		if !ok || pruneReceiptSortKey(current) >= pruneReceiptSortKey(existing) {
			out[key] = current
		}
	}
	return out
}

func pruneReceiptSortKey(receipt appliedPruneReceipt) string {
	return receipt.StartedAt + "\x00" + receipt.ID
}

func pruneApprovalReceiptKey(approvalID string, approvalScopeDigest string) string {
	return approvalID + "\x00" + approvalScopeDigest
}

func classifyPruneApprovals(review *PruneReview, dryRun prune.DryRunReport) {
	now := time.Now().UTC()
	if generatedAt, err := time.Parse(time.RFC3339Nano, dryRun.GeneratedAt); err == nil {
		now = generatedAt.UTC()
	}
	candidates := make(map[string]prune.Candidate, len(dryRun.Candidates))
	for _, candidate := range dryRun.Candidates {
		candidates[candidate.SoftDeleteID] = candidate
	}
	refusals := make(map[string]prune.Refusal, len(dryRun.Refusals))
	for _, refusal := range dryRun.Refusals {
		if refusal.SoftDeleteID != "" {
			refusals[refusal.SoftDeleteID] = refusal
		}
	}
	for i := range review.Approvals {
		classifyPruneApproval(&review.Approvals[i], candidates, refusals, now)
	}
}

func classifyPruneApproval(approval *PruneApproval, candidates map[string]prune.Candidate, refusals map[string]prune.Refusal, now time.Time) {
	if approval.LinkedReceiptID != "" {
		switch approval.LinkedReceiptStatus {
		case control.PruneReceiptApplied, control.PruneReceiptPartial:
			approval.ReleaseState = "consumed"
			approval.ReleaseReason = "approval has applied or partial prune receipt evidence"
			approval.ReleaseAction = "inspect_prune_receipt"
			approval.ReleaseBlocker = false
			return
		case control.PruneReceiptStarted:
			approval.Unapplied = false
			approval.ReleaseState = "pending_receipt"
			approval.ReleaseReason = "approval has started prune receipt evidence that still requires operator review"
			approval.ReleaseAction = "inspect_prune_receipt"
			approval.ReleaseBlocker = true
			return
		case control.PruneReceiptFailed:
			approval.Unapplied = false
			approval.ReleaseState = "failed_receipt"
			approval.ReleaseReason = "approval has failed prune receipt evidence that requires operator review"
			approval.ReleaseAction = "inspect_prune_receipt"
			approval.ReleaseBlocker = true
			return
		default:
			approval.Unapplied = false
			approval.ReleaseState = "receipt_attention"
			approval.ReleaseReason = "approval has prune receipt evidence that requires operator review"
			approval.ReleaseAction = "inspect_prune_receipt"
			approval.ReleaseBlocker = true
			return
		}
	}
	if approval.Status == "superseded" {
		approval.Unapplied = false
		approval.ReleaseState = "superseded"
		approval.ReleaseAction = "inspect_superseded_prune_approval"
		approval.ReleaseBlocker = false
		if approval.ReleaseReason == "" {
			approval.ReleaseReason = approval.RefusalReason
		}
		return
	}
	if approval.Status != "approved" {
		approval.Unapplied = false
		approval.ReleaseState = "not_approved"
		approval.ReleaseReason = "approval status is not approved"
		approval.ReleaseAction = "inspect_prune_approval"
		approval.ReleaseBlocker = true
		return
	}
	approval.Unapplied = true
	if approvalExpired(approval.ExpiresAt, now) {
		approval.ReleaseState = "expired"
		approval.ReleaseReason = "approval expires_at is not in the future"
		approval.ReleaseAction = "author_new_prune_approval"
		approval.ReleaseBlocker = true
	}
	for _, item := range approval.Items {
		evidence := PruneApprovalItemEvidence{
			SoftDeleteID: item.SoftDeleteID,
			TargetPath:   item.TargetPath,
		}
		if candidate, ok := candidates[item.SoftDeleteID]; ok {
			if reason := prune.ApprovalItemMismatch(item, candidate); reason != "" {
				evidence.State = "stale"
				evidence.Reason = reason
				markStalePruneApproval(approval, reason)
			} else {
				evidence.State = "current"
			}
			approval.CurrentEvidence = append(approval.CurrentEvidence, evidence)
			continue
		}
		if refusal, ok := refusals[item.SoftDeleteID]; ok {
			evidence.State = "refused"
			evidence.ReasonCode = refusal.ReasonCode
			evidence.Reason = refusal.Message
			markStalePruneApproval(approval, refusal.Message)
			approval.CurrentEvidence = append(approval.CurrentEvidence, evidence)
			continue
		}
		evidence.State = "missing"
		evidence.ReasonCode = prune.ReasonCurrentCandidateMissing
		evidence.Reason = "approved soft-delete item is not a current prune candidate"
		markStalePruneApproval(approval, evidence.Reason)
		approval.CurrentEvidence = append(approval.CurrentEvidence, evidence)
	}
	if approval.ReleaseState == "" || approval.ReleaseState == "pending_current_evidence" || approval.ReleaseState == "not_classified" {
		approval.ReleaseState = "active"
		approval.ReleaseAction = "ready_for_prune_apply"
		approval.ReleaseBlocker = false
		approval.ReleaseReason = "approved items match current prune candidates"
	}
}

func markStalePruneApproval(approval *PruneApproval, reason string) {
	if approval.ReleaseState == "expired" {
		return
	}
	approval.ReleaseState = "stale"
	approval.ReleaseReason = reason
	approval.ReleaseAction = "author_new_prune_approval"
	approval.ReleaseBlocker = true
}

func approvalExpired(raw string, now time.Time) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return true
	}
	return !now.UTC().Before(expiresAt)
}

func updatePruneApprovalSummary(review *PruneReview) {
	review.Summary.UnappliedApprovals = 0
	review.Summary.ActiveApprovals = 0
	review.Summary.StaleApprovals = 0
	review.Summary.ExpiredApprovals = 0
	review.Summary.ConsumedApprovals = 0
	for _, approval := range review.Approvals {
		if approval.Unapplied {
			review.Summary.UnappliedApprovals++
		}
		switch approval.ReleaseState {
		case "active":
			review.Summary.ActiveApprovals++
		case "stale":
			review.Summary.StaleApprovals++
		case "expired":
			review.Summary.ExpiredApprovals++
		case "consumed":
			review.Summary.ConsumedApprovals++
		}
	}
}

func filterPruneReceiptForSession(receipt control.PruneReceipt, softDeleteIDs map[string]struct{}) control.PruneReceipt {
	filtered := receipt
	filtered.Items = nil
	for _, item := range receipt.Items {
		if _, ok := softDeleteIDs[item.SoftDeleteID]; ok {
			filtered.Items = append(filtered.Items, item)
		}
	}
	filtered.Refusals = nil
	for _, refusal := range receipt.Refusals {
		if _, ok := softDeleteIDs[refusal.SoftDeleteID]; ok {
			filtered.Refusals = append(filtered.Refusals, refusal)
		}
	}
	return filtered
}

func viewPruneReceipt(path string, receipt control.PruneReceipt) PruneReceipt {
	return PruneReceipt{
		Path:                filepath.ToSlash(path),
		Action:              pruneReceiptAction(receipt.Status),
		Version:             receipt.Version,
		ID:                  receipt.ID,
		PruneSessionID:      receipt.PruneSessionID,
		ApprovalID:          receipt.ApprovalID,
		ProfileID:           receipt.ProfileID,
		TargetID:            receipt.TargetID,
		StartedAt:           receipt.StartedAt,
		EndedAt:             receipt.EndedAt,
		Status:              receipt.Status,
		DryRun:              receipt.DryRun,
		ApprovalScopeDigest: receipt.ApprovalScopeDigest,
		Items:               append([]control.PruneReceiptItem(nil), receipt.Items...),
		Refusals:            append([]control.PruneRefusal(nil), receipt.Refusals...),
	}
}

func pruneReceiptAction(status control.PruneReceiptStatus) string {
	switch status {
	case control.PruneReceiptApplied:
		return "inspect_applied_prune_receipt"
	case control.PruneReceiptPartial, control.PruneReceiptFailed, control.PruneReceiptStarted:
		return "inspect_prune_receipt"
	default:
		return "inspect_prune_receipt"
	}
}

func countPruneReceiptIssues(receipts []PruneReceipt) int {
	issues := 0
	for _, receipt := range receipts {
		switch receipt.Status {
		case control.PruneReceiptApplied:
		default:
			issues++
		}
	}
	return issues
}

func resolvedPruneSoftDeletes(receipts []PruneReceipt) map[string]struct{} {
	resolved := map[string]struct{}{}
	for _, receipt := range receipts {
		switch receipt.Status {
		case control.PruneReceiptApplied, control.PruneReceiptPartial:
		default:
			continue
		}
		for _, item := range receipt.Items {
			if item.Result == "pruned" {
				resolved[item.SoftDeleteID] = struct{}{}
			}
		}
	}
	return resolved
}

func filterResolvedTargetMissingPruneRefusals(refusals []prune.Refusal, resolved map[string]struct{}) []prune.Refusal {
	if len(resolved) == 0 {
		return append([]prune.Refusal(nil), refusals...)
	}
	out := make([]prune.Refusal, 0, len(refusals))
	for _, refusal := range refusals {
		if _, ok := resolved[refusal.SoftDeleteID]; ok && refusal.ReasonCode == prune.ReasonTargetMissing {
			continue
		}
		out = append(out, refusal)
	}
	return out
}

func summarizeLiveTargetDrift(report verify.DriftReport, detectorErr error, targetRoot string, sessionID string) LiveTargetDrift {
	out := LiveTargetDrift{
		Source:    "live_detector",
		Durable:   false,
		SessionID: report.SessionID,
		Manifest:  report.Manifest,
		Summary: LiveTargetDriftSummary{
			ManifestCount:    report.Summary.ManifestCount,
			ManifestEntries:  report.Summary.ManifestEntries,
			TargetDrifts:     report.Summary.TargetDrifts,
			ArtifactProblems: report.Summary.ArtifactProblems,
		},
		TargetDrifts: copyTargetDrifts(report.Drifts),
		Manifests:    append([]verify.ManifestSummary(nil), report.Manifests...),
	}
	for _, problem := range report.ArtifactProblems {
		out.ArtifactProblems = append(out.ArtifactProblems, ArtifactProblem{
			Source:    "live_target_drift",
			SessionID: problem.SessionID,
			Path:      filepath.ToSlash(problem.Path),
			Error:     problem.Err,
		})
	}
	if detectorErr != nil {
		out.ArtifactProblems = append(out.ArtifactProblems, ArtifactProblem{
			Source:    "live_target_drift",
			SessionID: sessionID,
			Path:      filepath.ToSlash(targetRoot),
			Error:     detectorErr.Error(),
		})
	}
	out.Summary.ArtifactProblems = len(out.ArtifactProblems)
	return out
}

func evaluatePairing(p *profile.Profile) (PairingState, []ArtifactProblem) {
	if p == nil {
		return PairingState{
			Status:            PairingStatusProfileUnavailable,
			EncryptedTransfer: "not_wired",
		}, nil
	}
	state := PairingState{
		ReceiptID:         strings.TrimSpace(p.Target.PairingReceiptID),
		TargetDeviceID:    strings.TrimSpace(p.Target.DevicePublicKey),
		PairedAt:          strings.TrimSpace(p.Target.PairedAt),
		EncryptedTransfer: "not_configured",
	}
	trust, err := pairing.ValidateProfileTrust(*p)
	if err == nil {
		state.Status = PairingStatusValid
		state.Evidence = "profile_pins_match_pairing_receipt"
		state.ReceiptID = trust.Receipt.ID
		state.TargetDeviceID = trust.TargetDeviceID
		state.Method = trust.Receipt.Method
		state.VerifiedAt = trust.Receipt.VerifiedAt
		state.PairedAt = p.Target.PairedAt
		if p.ValidateNetworkClientMaterial() == nil &&
			networkpush.ValidateProfileForNetworkPush(*p) == nil &&
			tlsidentity.ValidatePinned(p.Network.LocalTLSIdentity, trust.Receipt.SourceDeviceID, time.Now) == nil {
			state.EncryptedTransfer = "profile_backed_mtls_configured"
		}
		return state, nil
	}
	state.Issue = err.Error()
	switch {
	case errors.Is(err, pairing.ErrUnpairedProfile):
		state.Status = PairingStatusUnpaired
		state.Issue = ""
		state.Evidence = "profile_has_no_complete_pairing_pins"
		return state, nil
	case errors.Is(err, pairing.ErrPairingProfileInvalid):
		state.Status = PairingStatusProfileInvalid
	case errors.Is(err, pairing.ErrPairingTargetMissing):
		state.Status = PairingStatusTargetMissing
	case errors.Is(err, pairing.ErrPairingReceiptMissing):
		state.Status = PairingStatusReceiptMissing
	case errors.Is(err, pairing.ErrPairingReceiptInvalid):
		state.Status = PairingStatusReceiptInvalid
	case errors.Is(err, pairing.ErrPairingMismatch):
		state.Status = PairingStatusMismatch
	default:
		state.Status = PairingStatusReceiptInvalid
	}
	return state, []ArtifactProblem{{
		Source: "pairing_receipt",
		Path:   pairingReceiptProblemPath(*p),
		Error:  err.Error(),
	}}
}

func pairingReceiptProblemPath(p profile.Profile) string {
	if strings.TrimSpace(p.Target.LocalPath) == "" || strings.TrimSpace(p.Target.PairingReceiptID) == "" {
		return ""
	}
	path, err := control.Path(p.Target.LocalPath, control.ArtifactPairingReceipt, p.Target.PairingReceiptID)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(path)
}

func PrivacyForProfile(p *profile.Profile) PrivacyState {
	state := PrivacyState{
		Status:          "profile_unavailable",
		Claim:           "not_available",
		LocalPush:       "traffic_shaping_not_applied",
		NetworkTransfer: "not_available",
		Overhead: PrivacyOverhead{
			Status: "not_available",
			Source: "profile_unavailable",
		},
		ResidualLeakage: []string{
			"total_bytes",
			"duration",
			"peer_ip",
			"lan_presence",
			"supermover_use",
		},
	}
	if p == nil {
		return state
	}
	state.Status = "profile_contract_only"
	state = privacyForPolicy(state, p.PrivacyPolicy, "profile_contract")
	if trust, err := pairing.ValidateProfileTrust(*p); err == nil &&
		p.ValidateNetworkClientMaterial() == nil &&
		networkpush.ValidateProfileForNetworkPush(*p) == nil &&
		tlsidentity.ValidatePinned(p.Network.LocalTLSIdentity, trust.Receipt.SourceDeviceID, time.Now) == nil {
		state.NetworkTransfer = "profile_backed_mtls_configured"
	}
	return state
}

func privacyForProfileSnapshot(policy profile.PrivacyPolicy) PrivacyState {
	state := PrivacyState{
		Status:          "profile_snapshot_unavailable",
		Claim:           "not_available",
		LocalPush:       "traffic_shaping_not_applied",
		NetworkTransfer: "not_wired",
		Overhead: PrivacyOverhead{
			Status: "not_available",
			Source: "profile_snapshot",
		},
		ResidualLeakage: []string{
			"total_bytes",
			"duration",
			"peer_ip",
			"lan_presence",
			"supermover_use",
		},
	}
	if policy.TrafficLevel == 0 && policy.Mode == "" {
		return state
	}
	state.Status = "profile_snapshot_contract"
	return privacyForPolicy(state, policy, "profile_snapshot")
}

func privacyForPolicy(state PrivacyState, policy profile.PrivacyPolicy, overheadSource string) PrivacyState {
	state.Mode = string(policy.Mode)
	state.TrafficLevel = policy.TrafficLevel
	state.PaddingBucketBytes = policy.PaddingBucketBytes
	state.BatchMaxBytes = policy.BatchMaxBytes
	state.BatchMaxCount = policy.BatchMaxCount
	state.JitterBudgetMillis = policy.JitterBudgetMillis
	state.DiscoveryLowInfo = policy.DiscoveryLowInfo
	state.LocalPush = localPushPrivacyStatus(policy)
	state.NetworkTransfer = "not_configured"
	overheadStatus := "not_applied"
	if policy.TrafficLevel != 2 {
		overheadStatus = "not_applicable"
	}
	state.Overhead = PrivacyOverhead{
		Status:             overheadStatus,
		Source:             overheadSource,
		PaddingBucketBytes: policy.PaddingBucketBytes,
		BatchMaxBytes:      policy.BatchMaxBytes,
		BatchMaxCount:      policy.BatchMaxCount,
		JitterBudgetMillis: policy.JitterBudgetMillis,
	}
	if policy.TrafficLevel != 2 {
		state.Claim = "not_applicable"
		return state
	}
	state.Claim = "bounded_reduction_only"
	state.ConfiguredReduction = []string{
		"padding_buckets",
		"batch_bounds",
		"bounded_jitter",
		"low_information_discovery",
	}
	return state
}

func localPushPrivacyStatus(policy profile.PrivacyPolicy) string {
	if policy == profile.DefaultPrivacyPolicy() {
		return "traffic_shaping_not_applied"
	}
	return "unsupported_privacy_policy"
}

func trafficPrivacyAcceptance(targetRoot string, profileID string, targetID string, sessionID string, privacy PrivacyState, p *profile.Profile) TrafficPrivacyAcceptance {
	acceptance := TrafficPrivacyAcceptance{
		Status:               "not_applicable",
		Scope:                "profile_backed_network_path",
		Claim:                privacy.Claim,
		AnonymityClaim:       "not_claimed",
		EvidenceSource:       "network_transfer_artifact",
		ConfiguredReductions: append([]string(nil), privacy.ConfiguredReduction...),
		ResidualLeakage:      append([]string(nil), privacy.ResidualLeakage...),
		PaddingBucketBytes:   privacy.PaddingBucketBytes,
		BatchMaxBytes:        privacy.BatchMaxBytes,
		BatchMaxCount:        privacy.BatchMaxCount,
		JitterBudgetMillis:   privacy.JitterBudgetMillis,
		DiscoveryLowInfo:     privacy.DiscoveryLowInfo,
	}
	if privacy.TrafficLevel != int(transport.PrivacyLevel2) {
		acceptance.Blockers = []string{"traffic_level_not_2"}
		return acceptance
	}
	if privacy.NetworkTransfer != "profile_backed_mtls_configured" {
		acceptance.Status = "blocked"
		acceptance.Blockers = []string{"profile_backed_network_transfer_not_configured"}
		return acceptance
	}
	if p == nil {
		acceptance.Status = "blocked"
		acceptance.Blockers = []string{"profile_backed_network_transfer_not_configured"}
		return acceptance
	}
	expectedPolicy, err := transportPrivacyPolicyFromProfile(p.PrivacyPolicy)
	if err != nil {
		acceptance.Status = "blocked"
		acceptance.Blockers = []string{"profile_privacy_policy_invalid"}
		return acceptance
	}
	trust, err := pairing.ValidateProfileTrust(*p)
	if err != nil {
		acceptance.Status = "blocked"
		acceptance.Blockers = []string{"pairing_receipt_unavailable"}
		return acceptance
	}

	transfers, blockers := publishedLevel2NetworkTransferEvidence(targetRoot, profileID, targetID, sessionID, expectedPolicy, trust.Receipt.SourceDeviceID, trust.TargetDeviceID)
	if len(blockers) > 0 {
		acceptance.Status = "blocked"
		acceptance.Blockers = blockers
		return acceptance
	}
	if len(transfers) == 0 {
		acceptance.Status = "blocked"
		acceptance.Blockers = []string{"applied_overhead_missing"}
		return acceptance
	}
	transfer := transfers[len(transfers)-1]
	acceptance.Status = "passed"
	acceptance.SessionID = transfer.SessionID
	acceptance.ObservedOverhead = clonePrivacyOverhead(transfer.PrivacyOverhead)
	return acceptance
}

func publishedLevel2NetworkTransferEvidence(targetRoot string, profileID string, targetID string, sessionID string, expectedPolicy transport.PrivacyPolicy, sourceDeviceID string, targetDeviceID string) ([]control.NetworkTransfer, []string) {
	sessionsDir := filepath.Join(control.ControlDir(targetRoot), "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, []string{"network_transfer_artifact_unreadable"}
	}
	var transfers []control.NetworkTransfer
	var blockers []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		currentSessionID := entry.Name()
		if sessionID != "" && currentSessionID != sessionID {
			continue
		}
		if networkTransferReceiptOutOfScopeForAcceptance(targetRoot, currentSessionID, profileID, targetID) {
			continue
		}
		path, err := control.Path(targetRoot, control.ArtifactNetworkTransfer, currentSessionID)
		if err != nil {
			blockers = appendMissing(blockers, "network_transfer_artifact_unreadable")
			continue
		}
		transfer, err := control.ReadFileNoSymlinkUnderRoot[control.NetworkTransfer](control.ControlDir(targetRoot), path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if strings.Contains(err.Error(), "privacy_overhead") {
				blockers = appendMissing(blockers, "applied_overhead_missing")
				continue
			}
			blockers = appendMissing(blockers, "network_transfer_artifact_unreadable")
			continue
		}
		if transfer.SessionID != currentSessionID {
			blockers = appendMissing(blockers, "network_transfer_scope_mismatch")
			continue
		}
		if strings.TrimSpace(profileID) != "" && transfer.ProfileID != profileID {
			blockers = appendMissing(blockers, "network_transfer_scope_mismatch")
			continue
		}
		if strings.TrimSpace(targetID) != "" && transfer.TargetID != targetID {
			blockers = appendMissing(blockers, "network_transfer_scope_mismatch")
			continue
		}
		if strings.TrimSpace(sourceDeviceID) != "" && transfer.SourceDeviceID != sourceDeviceID {
			blockers = appendMissing(blockers, "network_transfer_identity_mismatch")
			continue
		}
		if strings.TrimSpace(targetDeviceID) != "" && transfer.TargetDeviceID != targetDeviceID {
			blockers = appendMissing(blockers, "network_transfer_identity_mismatch")
			continue
		}
		if transfer.PrivacyPolicy.Level != transport.PrivacyLevel2 {
			continue
		}
		if transfer.PrivacyPolicy != expectedPolicy {
			blockers = appendMissing(blockers, "privacy_policy_mismatch")
			continue
		}
		if transfer.Status != control.NetworkTransferPublished {
			continue
		}
		if transfer.PrivacyOverhead == nil || transfer.PrivacyOverhead.Empty() {
			blockers = appendMissing(blockers, "applied_overhead_missing")
			continue
		}
		if blocker := trafficPrivacyOverheadBlocker(expectedPolicy, *transfer.PrivacyOverhead); blocker != "" {
			blockers = appendMissing(blockers, blocker)
			continue
		}
		transfers = append(transfers, transfer)
	}
	sort.Slice(transfers, func(i, j int) bool {
		if transfers[i].UpdatedAt == transfers[j].UpdatedAt {
			return transfers[i].SessionID < transfers[j].SessionID
		}
		return transfers[i].UpdatedAt < transfers[j].UpdatedAt
	})
	return transfers, blockers
}

func transportPrivacyPolicyFromProfile(policy profile.PrivacyPolicy) (transport.PrivacyPolicy, error) {
	out := transport.PrivacyPolicy{
		Level:            transport.PrivacyLevel(policy.TrafficLevel),
		PaddingBucket:    policy.PaddingBucketBytes,
		BatchMaxBytes:    policy.BatchMaxBytes,
		BatchMaxCount:    policy.BatchMaxCount,
		JitterBudget:     policy.JitterBudgetMillis,
		DiscoveryLowInfo: policy.DiscoveryLowInfo,
	}
	if err := out.Validate(); err != nil {
		return transport.PrivacyPolicy{}, err
	}
	return out, nil
}

func trafficPrivacyOverheadBlocker(policy transport.PrivacyPolicy, overhead control.NetworkTransferPrivacyOverhead) string {
	if overhead.PaddingBucketBytes != policy.PaddingBucket || overhead.PaddedChunks <= 0 {
		return "applied_padding_overhead_missing"
	}
	if overhead.BatchFrames <= 0 || overhead.BatchedChunks <= 0 || overhead.MaxBatchCount <= 0 || overhead.MaxBatchCount > policy.BatchMaxCount || overhead.MaxBatchPlainBytes <= 0 || overhead.MaxBatchPlainBytes > policy.BatchMaxBytes {
		return "applied_batch_overhead_missing"
	}
	if overhead.JitterBudgetMillis != policy.JitterBudget || overhead.JitteredRequests <= 0 || overhead.MaxJitterDelayMillis > policy.JitterBudget {
		return "applied_jitter_overhead_missing"
	}
	return ""
}

func networkTransferReceiptOutOfScopeForAcceptance(targetRoot string, sessionID string, profileID string, targetID string) bool {
	receiptPath, err := control.Path(targetRoot, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		return false
	}
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		return false
	}
	if receipt.ID != sessionID || receipt.Status != "published" {
		return false
	}
	if strings.TrimSpace(profileID) != "" && receipt.ProfileID != profileID {
		return true
	}
	if strings.TrimSpace(targetID) != "" && receipt.TargetID != targetID {
		return true
	}
	return false
}

func clonePrivacyOverhead(overhead *control.NetworkTransferPrivacyOverhead) *control.NetworkTransferPrivacyOverhead {
	if overhead == nil {
		return nil
	}
	cloned := *overhead
	return &cloned
}

func appendMissing(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func summarizeLatestSession(report verify.Report) LatestSession {
	latest := LatestSession{
		ID:         report.Manifest.SessionID,
		ManifestID: report.Manifest.ID,
		CreatedAt:  report.Manifest.CreatedAt,
		Entries:    report.Manifest.Entries,
		Files:      report.Manifest.Files,
		Completeness: Completeness{
			FilesExpected:        report.Summary.FilesExpected,
			FilesVerified:        report.Summary.FilesVerified,
			VerificationErrors:   report.Summary.ErrorFindings,
			VerificationWarnings: report.Summary.WarningFindings,
		},
	}
	switch {
	case latest.ID == "":
		latest.Completeness.Status = CompletenessNoPublishedSession
	case report.Summary.ErrorFindings > 0:
		latest.Completeness.Status = CompletenessFailed
	case report.Summary.WarningFindings > 0:
		latest.Completeness.Status = CompletenessNeedsAttention
	default:
		latest.Completeness.Status = CompletenessVerified
	}
	return latest
}

func summarizeHealth(report health.Report, sessionID string, targetDrifts int) Health {
	out := Health{
		Summary: HealthSummary{
			ArtifactProblems: report.Summary.ArtifactProblems,
			TargetDrifts:     targetDrifts,
		},
	}
	for _, item := range report.Items {
		if sessionID != "" && item.SessionID != sessionID {
			continue
		}
		out.RecoveryIssues = append(out.RecoveryIssues, RecoveryIssue{
			SessionID: item.SessionID,
			State:     item.State,
			Action:    item.Action,
			Reason:    item.Reason,
			Path:      filepath.ToSlash(item.Path),
			UpdatedAt: item.UpdatedAt,
		})
	}
	for _, invalid := range report.Invalid {
		if sessionID != "" && invalid.SessionID != sessionID {
			continue
		}
		out.InvalidRecords = append(out.InvalidRecords, InvalidRecord{
			SessionID: invalid.SessionID,
			Path:      filepath.ToSlash(invalid.Path),
			Error:     invalid.Error,
		})
	}
	for _, issue := range report.Artifacts {
		if sessionID != "" && issue.SessionID != "" && issue.SessionID != sessionID {
			continue
		}
		source := issue.Source
		if source == "" {
			source = "health"
		}
		out.ArtifactIssues = append(out.ArtifactIssues, ArtifactProblem{
			Source:    source,
			SessionID: issue.SessionID,
			Path:      filepath.ToSlash(issue.Path),
			Error:     issue.Error,
		})
	}
	for _, transfer := range report.Transfers {
		if sessionID != "" && transfer.SessionID != sessionID {
			continue
		}
		out.Transfers = append(out.Transfers, NetworkTransfer{
			SessionID: transfer.SessionID,
			ProfileID: transfer.ProfileID,
			TargetID:  transfer.TargetID,
			Status:    transfer.Status,
			Stage:     transfer.Stage,
			ErrorCode: transfer.ErrorCode,
			Error:     transfer.Error,
			Path:      filepath.ToSlash(transfer.Path),
			UpdatedAt: transfer.UpdatedAt,
			Action:    transfer.Action,
			Privacy:   transfer.Privacy,
			Overhead:  transfer.Overhead,
		})
	}
	out.Summary.IncompleteSessions = len(out.RecoveryIssues)
	out.Summary.InvalidRecords = len(out.InvalidRecords)
	out.Summary.ArtifactProblems = len(out.ArtifactIssues)
	out.Summary.NetworkTransfers = len(out.Transfers)
	out.Healthy = out.Summary.IncompleteSessions == 0 &&
		out.Summary.InvalidRecords == 0 &&
		out.Summary.ArtifactProblems == 0 &&
		out.Summary.TargetDrifts == 0 &&
		out.Summary.NetworkTransfers == 0
	return out
}

func readProfileSnapshots(targetRoot string, manifests []verify.ManifestSummary, profileID string, targetID string, sessionID string) ([]ProfileSnapshot, []ArtifactProblem) {
	var snapshots []ProfileSnapshot
	var problems []ArtifactProblem
	for _, manifest := range manifests {
		if sessionID != "" && manifest.SessionID != sessionID {
			continue
		}
		id := "profile-" + manifest.SessionID
		path, err := control.Path(targetRoot, control.ArtifactProfileSnapshot, id)
		if err != nil {
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Error: err.Error()})
			continue
		}
		snapshot, err := control.ReadFile[control.ProfileSnapshot](path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				err = fmt.Errorf("profile snapshot %q is missing", id)
			}
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Path: path, Error: err.Error()})
			continue
		}
		if snapshot.ID != id {
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Path: path, Error: fmt.Sprintf("profile snapshot id %q does not match expected %q", snapshot.ID, id)})
			continue
		}
		if snapshot.SessionID != manifest.SessionID {
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Path: path, Error: fmt.Sprintf("profile snapshot session_id %q does not match manifest session %q", snapshot.SessionID, manifest.SessionID)})
			continue
		}
		if strings.TrimSpace(profileID) != "" && snapshot.ProfileID != profileID {
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Path: path, Error: fmt.Sprintf("profile snapshot profile_id %q does not match requested profile %q", snapshot.ProfileID, profileID)})
			continue
		}
		embedded, err := decodeProfileSnapshot(snapshot, profileID, targetID, manifest.RootID)
		if err != nil {
			problems = append(problems, ArtifactProblem{Source: "profile_snapshot", SessionID: manifest.SessionID, Path: path, Error: err.Error()})
			continue
		}
		snapshots = append(snapshots, ProfileSnapshot{
			ID:         snapshot.ID,
			SessionID:  snapshot.SessionID,
			ProfileID:  embedded.ProfileID,
			CapturedAt: snapshot.CapturedAt,
			Path:       filepath.ToSlash(path),
			Privacy:    privacyForProfileSnapshot(embedded.PrivacyPolicy),
		})
	}
	return snapshots, problems
}

type snapshotProfile struct {
	ProfileID     string                `json:"profile_id"`
	PrivacyPolicy profile.PrivacyPolicy `json:"privacy_policy"`
	Roots         []struct {
		ID string `json:"id"`
	} `json:"roots"`
	Target struct {
		TargetID string `json:"target_id"`
	} `json:"target"`
}

func decodeProfileSnapshot(snapshot control.ProfileSnapshot, expectedProfileID string, expectedTargetID string, manifestRootID string) (snapshotProfile, error) {
	var embedded snapshotProfile
	if err := json.Unmarshal(snapshot.Profile, &embedded); err != nil {
		return embedded, fmt.Errorf("decode embedded profile snapshot: %w", err)
	}
	if embedded.ProfileID != snapshot.ProfileID {
		return embedded, fmt.Errorf("embedded profile_id %q does not match snapshot profile_id %q", embedded.ProfileID, snapshot.ProfileID)
	}
	if strings.TrimSpace(expectedProfileID) != "" && embedded.ProfileID != expectedProfileID {
		return embedded, fmt.Errorf("embedded profile_id %q does not match requested profile %q", embedded.ProfileID, expectedProfileID)
	}
	if strings.TrimSpace(embedded.Target.TargetID) == "" {
		return embedded, errors.New("embedded target.target_id is required")
	}
	if strings.TrimSpace(expectedTargetID) != "" && embedded.Target.TargetID != expectedTargetID {
		return embedded, fmt.Errorf("embedded target_id %q does not match requested target %q", embedded.Target.TargetID, expectedTargetID)
	}
	if strings.TrimSpace(manifestRootID) != "" && !snapshotContainsRoot(embedded.Roots, manifestRootID) {
		return embedded, fmt.Errorf("manifest root_id %q is not present in embedded profile snapshot", manifestRootID)
	}
	return embedded, nil
}

func snapshotContainsRoot(roots []struct {
	ID string `json:"id"`
}, rootID string) bool {
	for _, root := range roots {
		if root.ID == rootID {
			return true
		}
	}
	return false
}

func readForeignPublishedReceipts(targetRoot string, profileID string, targetID string, sessionID string, manifestCount int) []ArtifactProblem {
	if sessionID != "" || manifestCount > 0 || (strings.TrimSpace(profileID) == "" && strings.TrimSpace(targetID) == "") {
		return nil
	}
	sessionsDir := filepath.Join(control.ControlDir(targetRoot), "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return []ArtifactProblem{{Source: "scope", Path: sessionsDir, Error: err.Error()}}
	}

	var problems []ArtifactProblem
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		receiptPath := filepath.Join(sessionsDir, entry.Name(), "receipt.json")
		receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
		if err != nil || receipt.Status != "published" || receipt.ID != entry.Name() {
			continue
		}
		if strings.TrimSpace(profileID) != "" && receipt.ProfileID == profileID && strings.TrimSpace(targetID) != "" && receipt.TargetID == targetID {
			continue
		}
		if strings.TrimSpace(profileID) != "" && receipt.ProfileID == profileID && strings.TrimSpace(targetID) == "" {
			continue
		}
		if strings.TrimSpace(targetID) != "" && receipt.TargetID == targetID && strings.TrimSpace(profileID) == "" {
			continue
		}
		problems = append(problems, ArtifactProblem{
			Source:    "scope",
			SessionID: receipt.ID,
			Path:      receiptPath,
			Error:     fmt.Sprintf("published session belongs to profile_id/target_id (%q/%q), not requested profile_id/target_id (%q/%q)", receipt.ProfileID, receipt.TargetID, profileID, targetID),
		})
	}
	return problems
}

func collectProfileSuggestions(warnings []control.Warning) []ProfileSuggestion {
	var suggestions []ProfileSuggestion
	for _, warning := range warnings {
		if len(warning.SuggestedProfilePatch) == 0 && len(warning.SuggestedConfig) == 0 {
			continue
		}
		suggestions = append(suggestions, ProfileSuggestion{
			WarningID:             warning.ID,
			SessionID:             warning.SessionID,
			Code:                  warning.Code,
			Severity:              warning.Severity,
			Message:               warning.Message,
			Paths:                 append([]string(nil), warning.Paths...),
			TargetPath:            warning.TargetPath,
			SuggestedProfilePatch: copyStringMap(warning.SuggestedProfilePatch),
			SuggestedConfig:       copyStringMap(warning.SuggestedConfig),
			CreatedAt:             warning.CreatedAt,
		})
	}
	return suggestions
}

func mergeArtifactProblems(sessionID string, healthIssues []ArtifactProblem, verifyProblems []verify.ArtifactProblem, extraProblemSets ...[]ArtifactProblem) []ArtifactProblem {
	var out []ArtifactProblem
	seen := map[string]struct{}{}
	add := func(problem ArtifactProblem) {
		if sessionID != "" && problem.SessionID != "" && problem.SessionID != sessionID {
			return
		}
		problem.Path = filepath.ToSlash(problem.Path)
		key := problem.Source + "\x00" + problem.SessionID + "\x00" + problem.Error
		if problem.Path != "" {
			key = "path\x00" + problem.Path
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, problem)
	}
	for _, issue := range healthIssues {
		add(issue)
	}
	for _, problem := range verifyProblems {
		add(ArtifactProblem{
			Source:    "verify",
			SessionID: problem.SessionID,
			Path:      problem.Path,
			Error:     problem.Err,
		})
	}
	for _, problems := range extraProblemSets {
		for _, problem := range problems {
			add(problem)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			if out[i].SessionID == out[j].SessionID {
				return out[i].Error < out[j].Error
			}
			return out[i].SessionID < out[j].SessionID
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func classify(report Report) Status {
	switch {
	case !report.Health.Healthy || report.Summary.ArtifactProblems > 0:
		return StatusUnhealthy
	case report.Summary.ManifestCount == 0:
		return StatusEmpty
	case report.Summary.VerificationErrors > 0:
		return StatusFailed
	case report.Summary.VerificationWarnings > 0 ||
		report.Summary.Warnings > 0 ||
		report.Summary.SoftDeletes > 0 ||
		report.Summary.PruneCandidates > 0 ||
		report.Summary.PruneRefusals > 0 ||
		report.Summary.PruneUnappliedApprovals > 0 ||
		report.Summary.PruneReceiptIssues > 0 ||
		report.Summary.TargetDrifts > 0 ||
		report.Summary.LiveTargetDrifts > 0 ||
		report.Summary.LiveTargetDriftProblems > 0:
		return StatusAttention
	default:
		return StatusVerified
	}
}

func (r Report) NeedsReview() bool {
	return r.Overall.Status != StatusVerified
}

func summarizeIssues(report Report) []string {
	var issues []string
	if report.Summary.ArtifactProblems > 0 {
		issues = append(issues, "artifact_problems")
	}
	if report.Summary.PairingIssues > 0 {
		issues = append(issues, "pairing_issues")
	}
	if report.Summary.NetworkTransfers > 0 {
		issues = append(issues, "network_transfers")
	}
	if report.Summary.RecoveryIssues > 0 {
		issues = append(issues, "recovery_issues")
	}
	if report.Summary.InvalidHealthRecords > 0 {
		issues = append(issues, "invalid_health_records")
	}
	if report.Summary.VerificationErrors > 0 {
		issues = append(issues, "verification_errors")
	}
	if report.Summary.VerificationWarnings > 0 {
		issues = append(issues, "verification_warnings")
	}
	if report.Summary.Warnings > 0 {
		issues = append(issues, "warnings")
	}
	if report.Summary.ProfileSuggestions > 0 {
		issues = append(issues, "profile_suggestions")
	}
	if report.Summary.SoftDeletes > 0 {
		issues = append(issues, "soft_deletes")
	}
	if report.Summary.PruneCandidates > 0 {
		issues = append(issues, "prune_candidates")
	}
	if report.Summary.PruneRefusals > 0 {
		issues = append(issues, "prune_refusals")
	}
	if report.Summary.PruneUnappliedApprovals > 0 {
		issues = append(issues, "prune_unapplied_approvals")
	}
	if report.Summary.PruneStaleApprovals > 0 {
		issues = append(issues, "prune_stale_approvals")
	}
	if report.Summary.PruneExpiredApprovals > 0 {
		issues = append(issues, "prune_expired_approvals")
	}
	if report.Summary.PruneReceiptIssues > 0 {
		issues = append(issues, "prune_receipt_issues")
	}
	if report.Summary.TargetDrifts > 0 {
		issues = append(issues, "target_drifts")
	}
	if report.Summary.LiveTargetDrifts > 0 {
		issues = append(issues, "live_target_drifts")
	}
	if report.Summary.LiveTargetDriftProblems > 0 {
		issues = append(issues, "live_target_drift_artifact_problems")
	}
	return issues
}

func copyWarnings(warnings []control.Warning) []control.Warning {
	out := append([]control.Warning(nil), warnings...)
	for i := range out {
		out[i].Paths = append([]string(nil), out[i].Paths...)
		out[i].Detected = copyStringMap(out[i].Detected)
		out[i].SuggestedProfilePatch = copyStringMap(out[i].SuggestedProfilePatch)
		out[i].SuggestedConfig = copyStringMap(out[i].SuggestedConfig)
	}
	return out
}

func copyTargetDrifts(drifts []control.TargetDrift) []control.TargetDrift {
	out := append([]control.TargetDrift(nil), drifts...)
	for i := range out {
		out[i].Evidence = append([]string(nil), out[i].Evidence...)
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
