package prune

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/targetlock"
	"github.com/khicago/supermover/internal/verify"
)

const (
	ReasonPreviousManifestMissing       = "previous_manifest_missing"
	ReasonPreviousManifestIDMismatch    = "previous_manifest_id_mismatch"
	ReasonPreviousManifestEntryMissing  = "previous_manifest_entry_missing"
	ReasonPreviousManifestEntryMismatch = "previous_manifest_entry_mismatch"
	ReasonUnsafeTargetPath              = "unsafe_target_path"
	ReasonUnsafeTargetParent            = "unsafe_target_parent"
	ReasonTargetMissing                 = "target_missing"
	ReasonTargetReadError               = "target_read_error"
	ReasonTargetKindMismatch            = "target_kind_mismatch"
	ReasonTargetContentMismatch         = "target_content_mismatch"
	ReasonTargetReappeared              = "target_reappeared"
	ReasonRetentionWindowActive         = "retention_window_active"
	ReasonWeakPreviousEvidence          = "weak_previous_evidence"
	ReasonUnsupportedKind               = "unsupported_kind"
	ReasonApprovalScopeMismatch         = "approval_scope_mismatch"
	ReasonApprovalItemMismatch          = "approval_item_mismatch"
	ReasonArtifactProblems              = "artifact_problems"
	ReasonCurrentCandidateMissing       = "current_candidate_missing"
	ReasonTargetChangedBeforePrune      = "target_changed_before_prune"
	ReasonRemoveFailed                  = "remove_failed"
)

var beforeRemoveApprovedTarget func(targetRoot, targetRel string) error

type Options struct {
	TargetRoot   string
	ProfileID    string
	TargetID     string
	SessionID    string
	DeletePolicy profile.DeletePolicy
	Now          time.Time
}

type ApplyOptions struct {
	TargetRoot     string
	ProfileID      string
	TargetID       string
	ApprovalID     string
	ApprovalPath   string
	PruneSessionID string
	DeletePolicy   profile.DeletePolicy
	Now            time.Time
}

type AuthorApprovalOptions struct {
	Profile        profile.Profile
	ProfilePayload []byte
	ApprovalID     string
	SoftDeleteIDs  []string
	ApprovedBy     string
	Reason         string
	ExpiresAt      string
	Now            time.Time
}

type ListApprovalsOptions struct {
	TargetRoot string
	ProfileID  string
	TargetID   string
}

type SupersedeApprovalOptions struct {
	TargetRoot string
	ProfileID  string
	TargetID   string
	ApprovalID string
	Reason     string
	Reviewer   string
	ReviewTool string
	Now        time.Time
}

type ApplyResult struct {
	TargetRoot       string               `json:"target_root"`
	ProfileID        string               `json:"profile_id"`
	TargetID         string               `json:"target_id"`
	ApprovalID       string               `json:"approval_id"`
	ApprovalPath     string               `json:"approval_path"`
	PruneSessionID   string               `json:"prune_session_id"`
	ReceiptPath      string               `json:"receipt_path"`
	ExistingReceipt  bool                 `json:"existing_receipt"`
	Receipt          control.PruneReceipt `json:"receipt"`
	ArtifactProblems []ArtifactProblem    `json:"artifact_problems,omitempty"`
}

type AuthorApprovalResult struct {
	Schema                 string                      `json:"schema"`
	TargetRoot             string                      `json:"target_root"`
	ProfileID              string                      `json:"profile_id"`
	TargetID               string                      `json:"target_id"`
	ApprovalID             string                      `json:"approval_id"`
	ApprovalPath           string                      `json:"approval_path"`
	ProfileSnapshotID      string                      `json:"profile_snapshot_id"`
	ProfileSnapshotPath    string                      `json:"profile_snapshot_path"`
	ProfileSnapshotDigest  string                      `json:"profile_snapshot_digest"`
	ApprovalScopeDigest    string                      `json:"approval_scope_digest"`
	ApprovalWriting        string                      `json:"approval_writing"`
	ProfileSnapshotWriting string                      `json:"profile_snapshot_writing"`
	PhysicalPruning        string                      `json:"physical_pruning"`
	ReceiptWriting         string                      `json:"receipt_writing"`
	Approval               control.PruneApproval       `json:"approval"`
	Items                  []control.PruneApprovalItem `json:"items"`
	DryRunSummary          DryRunSummary               `json:"dry_run_summary"`
}

type ListApprovalsResult struct {
	TargetRoot string                  `json:"target_root"`
	ProfileID  string                  `json:"profile_id"`
	TargetID   string                  `json:"target_id"`
	Approvals  []control.PruneApproval `json:"approvals,omitempty"`
}

type SupersedeApprovalResult struct {
	TargetRoot   string                `json:"target_root"`
	ProfileID    string                `json:"profile_id"`
	TargetID     string                `json:"target_id"`
	ApprovalID   string                `json:"approval_id"`
	ApprovalPath string                `json:"approval_path"`
	Approval     control.PruneApproval `json:"approval"`
}

type DryRunReport struct {
	Schema              string              `json:"schema"`
	TargetRoot          string              `json:"target_root"`
	ProfileID           string              `json:"profile_id"`
	TargetID            string              `json:"target_id"`
	SessionID           string              `json:"session_id,omitempty"`
	DryRun              bool                `json:"dry_run"`
	ApprovalRequired    bool                `json:"approval_required"`
	ProfileDeletePolicy ProfileDeletePolicy `json:"profile_delete_policy"`
	GeneratedAt         string              `json:"generated_at"`
	Summary             DryRunSummary       `json:"summary"`
	Candidates          []Candidate         `json:"candidates,omitempty"`
	Refusals            []Refusal           `json:"refusals,omitempty"`
	ArtifactProblems    []ArtifactProblem   `json:"artifact_problems,omitempty"`
}

type DryRunSummary struct {
	SoftDeletes      int `json:"soft_deletes"`
	Candidates       int `json:"candidates"`
	Refusals         int `json:"refusals"`
	ArtifactProblems int `json:"artifact_problems"`
}

type ProfileDeletePolicy struct {
	Mode               string `json:"mode"`
	RequireReview      bool   `json:"require_review"`
	RetentionDays      int    `json:"retention_days,omitempty"`
	AllowPhysicalPrune bool   `json:"allow_physical_prune"`
}

type ArtifactProblem struct {
	SessionID string `json:"session_id,omitempty"`
	Path      string `json:"path"`
	Error     string `json:"error"`
}

type Candidate struct {
	SoftDeleteID          string                           `json:"soft_delete_id"`
	DetectedSessionID     string                           `json:"detected_session_id"`
	ProfileID             string                           `json:"profile_id"`
	TargetID              string                           `json:"target_id"`
	RootID                string                           `json:"root_id"`
	SourcePath            string                           `json:"source_path"`
	TargetPath            string                           `json:"target_path"`
	Kind                  string                           `json:"kind"`
	Size                  int64                            `json:"size,omitempty"`
	Digest                string                           `json:"digest,omitempty"`
	DetectedAt            string                           `json:"detected_at"`
	PreviousSessionID     string                           `json:"previous_session_id"`
	PreviousManifestID    string                           `json:"previous_manifest_id"`
	PreviousManifestEntry PreviousManifestEvidence         `json:"previous_manifest_entry"`
	CurrentTargetState    control.PruneObservedTargetState `json:"current_target_state"`
	IntendedAction        string                           `json:"intended_action"`
	PhysicalPruning       string                           `json:"physical_pruning"`
	ApprovalWriting       string                           `json:"approval_writing"`
	ReceiptWriting        string                           `json:"receipt_writing"`
	ReviewRequired        bool                             `json:"review_required"`
}

type PreviousManifestEvidence struct {
	SessionID     string `json:"session_id"`
	ManifestID    string `json:"manifest_id"`
	RootID        string `json:"root_id,omitempty"`
	SourcePath    string `json:"source_path"`
	TargetPath    string `json:"target_path"`
	Kind          string `json:"kind"`
	Size          int64  `json:"size,omitempty"`
	Digest        string `json:"digest,omitempty"`
	Mode          uint32 `json:"mode,omitempty"`
	ModTime       string `json:"mod_time,omitempty"`
	SymlinkTarget string `json:"symlink_target,omitempty"`
}

type Refusal struct {
	SoftDeleteID       string                           `json:"soft_delete_id,omitempty"`
	DetectedSessionID  string                           `json:"detected_session_id,omitempty"`
	SourcePath         string                           `json:"source_path,omitempty"`
	TargetPath         string                           `json:"target_path,omitempty"`
	ReasonCode         string                           `json:"reason_code"`
	Message            string                           `json:"message"`
	SoftDeleteEvidence *control.SoftDelete              `json:"soft_delete_evidence,omitempty"`
	PreviousManifest   *PreviousManifestEvidence        `json:"previous_manifest_entry,omitempty"`
	CurrentTargetState control.PruneObservedTargetState `json:"current_target_state,omitempty"`
}

func AuthorApproval(opts AuthorApprovalOptions) (AuthorApprovalResult, error) {
	if strings.TrimSpace(opts.Profile.Target.LocalPath) == "" {
		return AuthorApprovalResult{}, errors.New("profile target.local_path is required")
	}
	targetRoot, err := filepath.Abs(opts.Profile.Target.LocalPath)
	if err != nil {
		return AuthorApprovalResult{}, err
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	if err := requireExistingTargetRoot(targetRoot); err != nil {
		return AuthorApprovalResult{}, err
	}
	if err := validateAuthorApprovalOptions(opts, now); err != nil {
		return AuthorApprovalResult{}, err
	}

	unlock, err := targetlock.LockTarget(targetRoot)
	if err != nil {
		return AuthorApprovalResult{}, fmt.Errorf("lock target for prune approval: %w", err)
	}
	defer unlock()

	report, err := PlanDryRun(Options{
		TargetRoot:   targetRoot,
		ProfileID:    opts.Profile.ProfileID,
		TargetID:     opts.Profile.Target.TargetID,
		DeletePolicy: opts.Profile.DeletePolicy,
		Now:          now,
	})
	if err != nil {
		return AuthorApprovalResult{}, err
	}
	if len(report.ArtifactProblems) > 0 {
		return AuthorApprovalResult{}, fmt.Errorf("%s: dry-run produced %d artifact problem(s)", ReasonArtifactProblems, len(report.ArtifactProblems))
	}
	if len(report.Refusals) > 0 {
		return AuthorApprovalResult{}, fmt.Errorf("dry-run produced %d refusal(s); approval was not written", len(report.Refusals))
	}

	selectedItems, rootID, err := selectedApprovalItems(opts.SoftDeleteIDs, report.Candidates)
	if err != nil {
		return AuthorApprovalResult{}, err
	}
	if !profileHasRoot(opts.Profile, rootID) {
		return AuthorApprovalResult{}, fmt.Errorf("approval root_id %q is not present in profile", rootID)
	}
	approvalID := strings.TrimSpace(opts.ApprovalID)
	snapshotID := "profile-" + approvalID
	if err := control.ValidateArtifactID(snapshotID); err != nil {
		return AuthorApprovalResult{}, fmt.Errorf("profile snapshot id is invalid: %w", err)
	}
	snapshotDigest, err := ProfileSnapshotDigest(opts.ProfilePayload)
	if err != nil {
		return AuthorApprovalResult{}, fmt.Errorf("profile snapshot digest: %w", err)
	}
	approvedAt := now.Format(time.RFC3339Nano)
	expiresAt, err := normalizedFutureTime(opts.ExpiresAt, now, "expires_at")
	if err != nil {
		return AuthorApprovalResult{}, err
	}

	approval := control.PruneApproval{
		Version:               control.CurrentVersion,
		ID:                    approvalID,
		ProfileID:             opts.Profile.ProfileID,
		TargetID:              opts.Profile.Target.TargetID,
		RootID:                rootID,
		CreatedAt:             approvedAt,
		ApprovedBy:            strings.TrimSpace(opts.ApprovedBy),
		ApprovedAt:            approvedAt,
		ReviewTool:            "supermover prune approve",
		ProfileSnapshotID:     snapshotID,
		ProfileSnapshotDigest: snapshotDigest,
		ProfileDeletePolicy:   controlPruneDeletePolicy(opts.Profile.DeletePolicy),
		Items:                 selectedItems,
		ExpiresAt:             expiresAt,
		Status:                "approved",
		ApprovalReason:        strings.TrimSpace(opts.Reason),
	}
	approval.ApprovalScopeDigest = ApprovalScopeDigest(approval.ProfileID, approval.TargetID, approval.RootID, approval.ProfileSnapshotID, approval.ProfileSnapshotDigest, opts.Profile.DeletePolicy, approval.Items)

	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         snapshotID,
		ProfileID:  opts.Profile.ProfileID,
		CapturedAt: approvedAt,
		Profile:    append(json.RawMessage(nil), opts.ProfilePayload...),
	}
	snapshotPath, err := control.Path(targetRoot, control.ArtifactProfileSnapshot, snapshot.ID)
	if err != nil {
		return AuthorApprovalResult{}, err
	}
	approvalPath, err := control.Path(targetRoot, control.ArtifactPruneApproval, approval.ID)
	if err != nil {
		return AuthorApprovalResult{}, err
	}
	if err := ensureNewControlArtifactPath(snapshotPath, "profile snapshot"); err != nil {
		return AuthorApprovalResult{}, err
	}
	if err := ensureNewControlArtifactPath(approvalPath, "prune approval"); err != nil {
		return AuthorApprovalResult{}, err
	}
	if err := control.WriteNewFile(snapshotPath, snapshot); err != nil {
		return AuthorApprovalResult{}, fmt.Errorf("write profile snapshot: %w", err)
	}
	if err := control.WriteNewFile(approvalPath, approval); err != nil {
		return AuthorApprovalResult{}, fmt.Errorf("write prune approval: %w", err)
	}
	return AuthorApprovalResult{
		Schema:                 "supermover.prune_approval_authoring.v1",
		TargetRoot:             filepath.ToSlash(targetRoot),
		ProfileID:              approval.ProfileID,
		TargetID:               approval.TargetID,
		ApprovalID:             approval.ID,
		ApprovalPath:           filepath.ToSlash(approvalPath),
		ProfileSnapshotID:      snapshot.ID,
		ProfileSnapshotPath:    filepath.ToSlash(snapshotPath),
		ProfileSnapshotDigest:  snapshotDigest,
		ApprovalScopeDigest:    approval.ApprovalScopeDigest,
		ApprovalWriting:        "written",
		ProfileSnapshotWriting: "written",
		PhysicalPruning:        "not_applied",
		ReceiptWriting:         "not_written_by_approval_authoring",
		Approval:               approval,
		Items:                  append([]control.PruneApprovalItem(nil), approval.Items...),
		DryRunSummary:          report.Summary,
	}, nil
}

func ListApprovals(opts ListApprovalsOptions) (ListApprovalsResult, error) {
	if strings.TrimSpace(opts.TargetRoot) == "" {
		return ListApprovalsResult{}, errors.New("target root is required")
	}
	targetRoot, err := filepath.Abs(opts.TargetRoot)
	if err != nil {
		return ListApprovalsResult{}, err
	}
	if err := requireExistingTargetRoot(targetRoot); err != nil {
		return ListApprovalsResult{}, err
	}

	dir := filepath.Join(control.ControlDir(targetRoot), "prune", "approvals")
	if err := pathguard.EnsureDirectory(targetRoot, dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ListApprovalsResult{
				TargetRoot: filepath.ToSlash(targetRoot),
				ProfileID:  opts.ProfileID,
				TargetID:   opts.TargetID,
			}, nil
		}
		return ListApprovalsResult{}, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ListApprovalsResult{
				TargetRoot: filepath.ToSlash(targetRoot),
				ProfileID:  opts.ProfileID,
				TargetID:   opts.TargetID,
			}, nil
		}
		return ListApprovalsResult{}, err
	}

	approvals := make([]control.PruneApproval, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		scope, err := readPruneApprovalScopeDocument(targetRoot, path)
		if err != nil {
			return ListApprovalsResult{}, fmt.Errorf("read prune approval scope %q: %w", path, err)
		}
		if scope.excludedBy(opts.ProfileID, opts.TargetID) {
			continue
		}
		approval, err := control.ReadFileNoSymlinkUnderRoot[control.PruneApproval](targetRoot, path)
		if err != nil {
			return ListApprovalsResult{}, fmt.Errorf("read prune approval %q: %w", path, err)
		}
		pathID := strings.TrimSuffix(entry.Name(), ".json")
		if approval.ID != pathID {
			return ListApprovalsResult{}, fmt.Errorf("prune approval id %q does not match path id %q", approval.ID, pathID)
		}
		approvals = append(approvals, approval)
	}
	sort.Slice(approvals, func(i, j int) bool {
		left := approvals[i].ApprovedAt
		if left == "" {
			left = approvals[i].CreatedAt
		}
		right := approvals[j].ApprovedAt
		if right == "" {
			right = approvals[j].CreatedAt
		}
		if left == right {
			return approvals[i].ID < approvals[j].ID
		}
		return left < right
	})

	return ListApprovalsResult{
		TargetRoot: filepath.ToSlash(targetRoot),
		ProfileID:  opts.ProfileID,
		TargetID:   opts.TargetID,
		Approvals:  approvals,
	}, nil
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

func readPruneApprovalScopeDocument(targetRoot string, path string) (pruneApprovalScope, error) {
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

func SupersedeApproval(opts SupersedeApprovalOptions) (SupersedeApprovalResult, error) {
	if strings.TrimSpace(opts.TargetRoot) == "" {
		return SupersedeApprovalResult{}, errors.New("target root is required")
	}
	targetRoot, err := filepath.Abs(opts.TargetRoot)
	if err != nil {
		return SupersedeApprovalResult{}, err
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	if err := requireExistingTargetRoot(targetRoot); err != nil {
		return SupersedeApprovalResult{}, err
	}
	if strings.TrimSpace(opts.ApprovalID) == "" {
		return SupersedeApprovalResult{}, errors.New("approval id is required")
	}
	if err := control.ValidateArtifactID(strings.TrimSpace(opts.ApprovalID)); err != nil {
		return SupersedeApprovalResult{}, fmt.Errorf("approval id is invalid: %w", err)
	}
	if strings.TrimSpace(opts.Reason) == "" {
		return SupersedeApprovalResult{}, errors.New("supersede reason is required")
	}
	reviewTool := strings.TrimSpace(opts.ReviewTool)
	if reviewTool == "" {
		reviewTool = "supermover prune supersede"
	}

	unlock, err := targetlock.LockTarget(targetRoot)
	if err != nil {
		return SupersedeApprovalResult{}, fmt.Errorf("lock target for prune approval supersede: %w", err)
	}
	defer unlock()

	approvalPath, err := control.Path(targetRoot, control.ArtifactPruneApproval, strings.TrimSpace(opts.ApprovalID))
	if err != nil {
		return SupersedeApprovalResult{}, err
	}
	approval, err := readControlFileNoSymlink[control.PruneApproval](targetRoot, approvalPath, "prune approval")
	if err != nil {
		return SupersedeApprovalResult{}, fmt.Errorf("read prune approval: %w", err)
	}
	if strings.TrimSpace(opts.ProfileID) != "" && approval.ProfileID != opts.ProfileID {
		return SupersedeApprovalResult{}, fmt.Errorf("approval profile_id %q does not match profile %q", approval.ProfileID, opts.ProfileID)
	}
	if strings.TrimSpace(opts.TargetID) != "" && approval.TargetID != opts.TargetID {
		return SupersedeApprovalResult{}, fmt.Errorf("approval target_id %q does not match target %q", approval.TargetID, opts.TargetID)
	}
	if linkedReceipts, err := appliedPruneReceiptsForTarget(targetRoot, approval.ProfileID, approval.TargetID, approval.ID, approval.ApprovalScopeDigest); err != nil {
		return SupersedeApprovalResult{}, err
	} else if len(linkedReceipts) > 0 {
		return SupersedeApprovalResult{}, errors.New("approval already has prune receipt evidence and cannot be superseded")
	}
	if approval.Status == "superseded" {
		return SupersedeApprovalResult{
			TargetRoot:   filepath.ToSlash(targetRoot),
			ProfileID:    approval.ProfileID,
			TargetID:     approval.TargetID,
			ApprovalID:   approval.ID,
			ApprovalPath: filepath.ToSlash(approvalPath),
			Approval:     approval,
		}, nil
	}
	if approval.Status != "approved" {
		return SupersedeApprovalResult{}, fmt.Errorf("approval status %q cannot be superseded", approval.Status)
	}

	approval.Status = "superseded"
	approval.RefusalReason = strings.TrimSpace(opts.Reason)
	approval.ReviewTool = reviewTool
	approval.SupersededBy = strings.TrimSpace(opts.Reviewer)
	approval.SupersededAt = now.Format(time.RFC3339Nano)

	if err := control.WriteFile(approvalPath, approval); err != nil {
		return SupersedeApprovalResult{}, fmt.Errorf("write superseded prune approval: %w", err)
	}
	return SupersedeApprovalResult{
		TargetRoot:   filepath.ToSlash(targetRoot),
		ProfileID:    approval.ProfileID,
		TargetID:     approval.TargetID,
		ApprovalID:   approval.ID,
		ApprovalPath: filepath.ToSlash(approvalPath),
		Approval:     approval,
	}, nil
}

func appliedPruneReceiptsForTarget(targetRoot string, profileID string, targetID string, approvalID string, scopeDigest string) ([]control.PruneReceipt, error) {
	dir := filepath.Join(control.ControlDir(targetRoot), "prune", "receipts")
	if err := pathguard.EnsureDirectory(targetRoot, dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var receipts []control.PruneReceipt
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		receipt, err := control.ReadFileNoSymlinkUnderRoot[control.PruneReceipt](targetRoot, path)
		if err != nil {
			return nil, fmt.Errorf("read prune receipt %q: %w", path, err)
		}
		pathID := strings.TrimSuffix(entry.Name(), ".json")
		if receipt.ID != pathID {
			return nil, fmt.Errorf("prune receipt id %q does not match path id %q", receipt.ID, pathID)
		}
		if receipt.ProfileID != profileID || receipt.TargetID != targetID || receipt.ApprovalID != approvalID || receipt.ApprovalScopeDigest != scopeDigest {
			continue
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func Apply(opts ApplyOptions) (ApplyResult, error) {
	if strings.TrimSpace(opts.TargetRoot) == "" {
		return ApplyResult{}, errors.New("target root is required")
	}
	targetRoot, err := filepath.Abs(opts.TargetRoot)
	if err != nil {
		return ApplyResult{}, err
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	if err := requireExistingTargetRoot(targetRoot); err != nil {
		return ApplyResult{}, err
	}
	unlock, err := targetlock.LockTarget(targetRoot)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("lock target for prune apply: %w", err)
	}
	defer unlock()

	targetRootInfo, err := verifiedTargetRootInfo(targetRoot)
	if err != nil {
		return ApplyResult{}, err
	}

	approvalPath, expectedApprovalID, err := resolveApprovalPath(targetRoot, opts)
	if err != nil {
		return ApplyResult{}, err
	}
	approval, err := readControlFileNoSymlink[control.PruneApproval](targetRoot, approvalPath, "prune approval")
	if err != nil {
		return ApplyResult{}, fmt.Errorf("read prune approval: %w", err)
	}
	if err := validateApprovalBinding(targetRoot, expectedApprovalID, approval, opts, now); err != nil {
		return ApplyResult{}, err
	}

	pruneSessionID := strings.TrimSpace(opts.PruneSessionID)
	if pruneSessionID == "" {
		pruneSessionID = approval.ID
	}
	receiptPath, err := control.Path(targetRoot, control.ArtifactPruneReceipt, pruneSessionID)
	if err != nil {
		return ApplyResult{}, err
	}
	if receipt, ok, err := readExistingReceipt(receiptPath, pruneSessionID, approval, opts); ok || err != nil {
		if err != nil {
			return ApplyResult{}, err
		}
		return ApplyResult{
			TargetRoot:      filepath.ToSlash(targetRoot),
			ProfileID:       opts.ProfileID,
			TargetID:        opts.TargetID,
			ApprovalID:      approval.ID,
			ApprovalPath:    filepath.ToSlash(approvalPath),
			PruneSessionID:  pruneSessionID,
			ReceiptPath:     filepath.ToSlash(receiptPath),
			ExistingReceipt: true,
			Receipt:         receipt,
		}, nil
	}

	report, err := PlanDryRun(Options{
		TargetRoot:   targetRoot,
		ProfileID:    opts.ProfileID,
		TargetID:     opts.TargetID,
		DeletePolicy: opts.DeletePolicy,
		Now:          now,
	})
	if err != nil {
		return ApplyResult{}, err
	}

	startReceipt := startedReceipt(pruneSessionID, approval, report, now)
	if err := control.WriteNewFile(receiptPath, startReceipt); err != nil {
		if existing, ok, readErr := readExistingReceipt(receiptPath, pruneSessionID, approval, opts); ok || readErr != nil {
			if readErr != nil {
				return ApplyResult{}, fmt.Errorf("write started prune receipt: %w", err)
			}
			return ApplyResult{
				TargetRoot:      filepath.ToSlash(targetRoot),
				ProfileID:       opts.ProfileID,
				TargetID:        opts.TargetID,
				ApprovalID:      approval.ID,
				ApprovalPath:    filepath.ToSlash(approvalPath),
				PruneSessionID:  pruneSessionID,
				ReceiptPath:     filepath.ToSlash(receiptPath),
				ExistingReceipt: true,
				Receipt:         existing,
			}, nil
		}
		return ApplyResult{}, fmt.Errorf("write started prune receipt: %w", err)
	}

	items, refusals := applyApprovedItems(targetRoot, targetRootInfo, approval, report, now)
	finalReceipt := control.PruneReceipt{
		Version:             control.CurrentVersion,
		ID:                  pruneSessionID,
		PruneSessionID:      pruneSessionID,
		ApprovalID:          approval.ID,
		ProfileID:           approval.ProfileID,
		TargetID:            approval.TargetID,
		StartedAt:           now.Format(time.RFC3339Nano),
		EndedAt:             now.Format(time.RFC3339Nano),
		Status:              receiptStatus(items),
		DryRun:              false,
		ApprovalScopeDigest: approval.ApprovalScopeDigest,
		Items:               items,
		Refusals:            refusals,
	}
	if err := control.WriteFile(receiptPath, finalReceipt); err != nil {
		return ApplyResult{}, fmt.Errorf("write final prune receipt: %w", err)
	}
	return ApplyResult{
		TargetRoot:       filepath.ToSlash(targetRoot),
		ProfileID:        opts.ProfileID,
		TargetID:         opts.TargetID,
		ApprovalID:       approval.ID,
		ApprovalPath:     filepath.ToSlash(approvalPath),
		PruneSessionID:   pruneSessionID,
		ReceiptPath:      filepath.ToSlash(receiptPath),
		Receipt:          finalReceipt,
		ArtifactProblems: report.ArtifactProblems,
	}, nil
}

func PlanDryRun(opts Options) (DryRunReport, error) {
	if strings.TrimSpace(opts.TargetRoot) == "" {
		return DryRunReport{}, errors.New("target root is required")
	}
	targetRoot, err := filepath.Abs(opts.TargetRoot)
	if err != nil {
		return DryRunReport{}, err
	}
	generatedAt := opts.Now
	if generatedAt.IsZero() {
		generatedAt = time.Now()
	}
	generatedAt = generatedAt.UTC()

	if err := requireExistingTargetRoot(targetRoot); err != nil {
		return DryRunReport{}, err
	}
	artifacts, err := verify.LoadArtifactsForScope(targetRoot, opts.ProfileID, opts.TargetID)
	if err != nil {
		return DryRunReport{}, err
	}
	softDeletes := filterSoftDeletes(artifacts.SoftDeletes, opts.SessionID)
	report := DryRunReport{
		TargetRoot:          filepath.ToSlash(targetRoot),
		ProfileID:           opts.ProfileID,
		TargetID:            opts.TargetID,
		SessionID:           opts.SessionID,
		Schema:              "supermover.prune_dry_run.v1",
		DryRun:              true,
		ApprovalRequired:    true,
		ProfileDeletePolicy: profileDeletePolicy(opts.DeletePolicy),
		GeneratedAt:         generatedAt.Format(time.RFC3339Nano),
		ArtifactProblems:    artifactProblems(filterVerifyArtifactProblems(artifacts.ArtifactProblems, opts.SessionID)),
	}
	manifestBySession := make(map[string]control.Manifest, len(artifacts.Manifests))
	for _, manifest := range artifacts.Manifests {
		manifestBySession[manifest.SessionID] = manifest
	}
	for _, record := range softDeletes {
		candidate, refusal := planRecord(targetRoot, record, manifestBySession, artifacts.Manifests, opts.DeletePolicy, generatedAt)
		if refusal != nil {
			report.Refusals = append(report.Refusals, *refusal)
			continue
		}
		report.Candidates = append(report.Candidates, candidate)
	}
	sort.Slice(report.Candidates, func(i, j int) bool {
		if report.Candidates[i].TargetPath == report.Candidates[j].TargetPath {
			return report.Candidates[i].SoftDeleteID < report.Candidates[j].SoftDeleteID
		}
		return report.Candidates[i].TargetPath < report.Candidates[j].TargetPath
	})
	sort.Slice(report.Refusals, func(i, j int) bool {
		if report.Refusals[i].TargetPath == report.Refusals[j].TargetPath {
			if report.Refusals[i].ReasonCode == report.Refusals[j].ReasonCode {
				return report.Refusals[i].SoftDeleteID < report.Refusals[j].SoftDeleteID
			}
			return report.Refusals[i].ReasonCode < report.Refusals[j].ReasonCode
		}
		return report.Refusals[i].TargetPath < report.Refusals[j].TargetPath
	})
	report.Summary = DryRunSummary{
		SoftDeletes:      len(softDeletes),
		Candidates:       len(report.Candidates),
		Refusals:         len(report.Refusals),
		ArtifactProblems: len(report.ArtifactProblems),
	}
	return report, nil
}

func (r DryRunReport) NeedsReview() bool {
	return r.Summary.Candidates > 0 ||
		r.Summary.Refusals > 0 ||
		r.Summary.ArtifactProblems > 0
}

func validateAuthorApprovalOptions(opts AuthorApprovalOptions, now time.Time) error {
	if err := opts.Profile.Validate(); err != nil {
		return fmt.Errorf("profile is invalid: %w", err)
	}
	if err := validateProfilePayloadMatches(opts.ProfilePayload, opts.Profile); err != nil {
		return err
	}
	if opts.Profile.DeletePolicy.Mode != profile.DeleteModePrune ||
		!opts.Profile.DeletePolicy.RequireReview ||
		!opts.Profile.DeletePolicy.AllowPhysicalPrune {
		return errors.New("profile delete_policy must use mode=prune with require_review=true and allow_physical_prune=true")
	}
	if strings.TrimSpace(opts.ApprovalID) == "" {
		return errors.New("approval id is required")
	}
	if err := control.ValidateArtifactID(strings.TrimSpace(opts.ApprovalID)); err != nil {
		return fmt.Errorf("approval id is invalid: %w", err)
	}
	if strings.TrimSpace(opts.ApprovedBy) == "" {
		return errors.New("approved by is required")
	}
	if strings.TrimSpace(opts.Reason) == "" {
		return errors.New("approval reason is required")
	}
	if len(opts.SoftDeleteIDs) == 0 {
		return errors.New("at least one soft-delete id is required")
	}
	seen := map[string]struct{}{}
	for _, raw := range opts.SoftDeleteIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			return errors.New("soft-delete id is required")
		}
		if err := control.ValidateArtifactID(id); err != nil {
			return fmt.Errorf("soft-delete id %q is invalid: %w", id, err)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate soft-delete id %q", id)
		}
		seen[id] = struct{}{}
	}
	if _, err := normalizedFutureTime(opts.ExpiresAt, now, "expires_at"); err != nil {
		return err
	}
	return nil
}

func validateProfilePayloadMatches(payload []byte, want profile.Profile) error {
	if len(payload) == 0 || !json.Valid(payload) {
		return errors.New("profile payload must contain valid JSON")
	}
	payloadProfile, err := profile.Read(bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("profile payload is invalid: %w", err)
	}
	wantData, err := json.Marshal(want)
	if err != nil {
		return fmt.Errorf("profile payload comparison: %w", err)
	}
	gotData, err := json.Marshal(payloadProfile)
	if err != nil {
		return fmt.Errorf("profile payload comparison: %w", err)
	}
	if !bytes.Equal(gotData, wantData) {
		return errors.New("profile payload does not match profile approval scope")
	}
	return nil
}

func normalizedFutureTime(raw string, now time.Time, label string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", label, err)
	}
	if !parsed.After(now) {
		return "", fmt.Errorf("%s must be after approved_at", label)
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
}

func selectedApprovalItems(ids []string, candidates []Candidate) ([]control.PruneApprovalItem, string, error) {
	byID := make(map[string]Candidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.SoftDeleteID] = candidate
	}
	items := make([]control.PruneApprovalItem, 0, len(ids))
	var rootID string
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		candidate, ok := byID[id]
		if !ok {
			return nil, "", fmt.Errorf("%s: selected soft-delete id %q is not a current prune candidate", ReasonCurrentCandidateMissing, id)
		}
		if rootID == "" {
			rootID = candidate.RootID
		} else if candidate.RootID != rootID {
			return nil, "", errors.New("selected soft-delete candidates span multiple roots")
		}
		items = append(items, approvalItemFromCandidate(candidate))
	}
	if len(items) == 0 {
		return nil, "", errors.New("no prune candidates selected")
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].SoftDeleteID == items[j].SoftDeleteID {
			return items[i].TargetPath < items[j].TargetPath
		}
		return items[i].SoftDeleteID < items[j].SoftDeleteID
	})
	return items, rootID, nil
}

func approvalItemFromCandidate(candidate Candidate) control.PruneApprovalItem {
	return control.PruneApprovalItem{
		SoftDeleteID:       candidate.SoftDeleteID,
		SoftDeleteRef:      "deleted/" + candidate.SoftDeleteID + ".json",
		DetectedSessionID:  candidate.DetectedSessionID,
		PreviousSessionID:  candidate.PreviousSessionID,
		PreviousManifestID: candidate.PreviousManifestID,
		RootID:             candidate.RootID,
		SourcePath:         candidate.SourcePath,
		TargetPath:         candidate.TargetPath,
		Kind:               candidate.Kind,
		Size:               candidate.Size,
		Digest:             candidate.Digest,
		SymlinkTarget:      candidate.PreviousManifestEntry.SymlinkTarget,
		DetectedAt:         candidate.DetectedAt,
	}
}

func ApprovalScopeDigest(profileID, targetID, rootID, profileSnapshotID, profileSnapshotDigest string, policy profile.DeletePolicy, items []control.PruneApprovalItem) string {
	scope := approvalScopeDigestDocument{
		Schema:                "supermover.prune_approval_scope.v1",
		ProfileID:             profileID,
		TargetID:              targetID,
		RootID:                rootID,
		ProfileSnapshotID:     profileSnapshotID,
		ProfileSnapshotDigest: profileSnapshotDigest,
		ProfileDeletePolicy:   controlPruneDeletePolicy(policy),
		Items:                 canonicalApprovalItems(items),
	}
	data, err := json.Marshal(scope)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

type approvalScopeDigestDocument struct {
	Schema                string                      `json:"schema"`
	ProfileID             string                      `json:"profile_id"`
	TargetID              string                      `json:"target_id"`
	RootID                string                      `json:"root_id"`
	ProfileSnapshotID     string                      `json:"profile_snapshot_id"`
	ProfileSnapshotDigest string                      `json:"profile_snapshot_digest"`
	ProfileDeletePolicy   control.PruneDeletePolicy   `json:"profile_delete_policy"`
	Items                 []control.PruneApprovalItem `json:"items"`
}

func canonicalApprovalItems(items []control.PruneApprovalItem) []control.PruneApprovalItem {
	canonical := append([]control.PruneApprovalItem(nil), items...)
	sort.Slice(canonical, func(i, j int) bool {
		if canonical[i].SoftDeleteID == canonical[j].SoftDeleteID {
			return canonical[i].TargetPath < canonical[j].TargetPath
		}
		return canonical[i].SoftDeleteID < canonical[j].SoftDeleteID
	})
	return canonical
}

func readControlFileNoSymlink[T control.Document](targetRoot string, path string, label string) (T, error) {
	doc, err := control.ReadFileNoSymlinkUnderRoot[T](targetRoot, path)
	if err != nil {
		var zero T
		if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return zero, fmt.Errorf("%s %q exists as a symlink", label, path)
		}
		return zero, fmt.Errorf("%s %q: %w", label, path, err)
	}
	return doc, nil
}

func ensureNewControlArtifactPath(path string, label string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s %q exists as a symlink", label, path)
		}
		return fmt.Errorf("%w: %q", control.ErrArtifactExists, path)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("inspect %s path: %w", label, err)
}

func profileDeletePolicy(policy profile.DeletePolicy) ProfileDeletePolicy {
	return ProfileDeletePolicy{
		Mode:               string(policy.Mode),
		RequireReview:      policy.RequireReview,
		RetentionDays:      policy.RetentionDays,
		AllowPhysicalPrune: policy.AllowPhysicalPrune,
	}
}

func controlPruneDeletePolicy(policy profile.DeletePolicy) control.PruneDeletePolicy {
	return control.PruneDeletePolicy{
		Mode:               string(policy.Mode),
		RequireReview:      policy.RequireReview,
		RetentionDays:      policy.RetentionDays,
		AllowPhysicalPrune: policy.AllowPhysicalPrune,
	}
}

func resolveApprovalPath(targetRoot string, opts ApplyOptions) (string, string, error) {
	approvalID := strings.TrimSpace(opts.ApprovalID)
	approvalPath := strings.TrimSpace(opts.ApprovalPath)
	switch {
	case approvalID == "" && approvalPath == "":
		return "", "", errors.New("exactly one explicit approval id or path is required")
	case approvalID != "" && approvalPath != "":
		return "", "", errors.New("approval id and approval path are mutually exclusive")
	case approvalID != "":
		path, err := control.Path(targetRoot, control.ArtifactPruneApproval, approvalID)
		return path, approvalID, err
	default:
		path, err := filepath.Abs(approvalPath)
		if err != nil {
			return "", "", err
		}
		if filepath.Dir(path) != filepath.Join(control.ControlDir(targetRoot), "prune", "approvals") {
			return "", "", fmt.Errorf("approval path must be inside target control-plane prune approvals: %s", approvalPath)
		}
		base := filepath.Base(path)
		if !strings.HasSuffix(base, ".json") {
			return "", "", fmt.Errorf("approval path must end in .json: %s", approvalPath)
		}
		expectedID := strings.TrimSuffix(base, ".json")
		if err := control.ValidateArtifactID(expectedID); err != nil {
			return "", "", fmt.Errorf("approval path id is unsafe: %w", err)
		}
		return path, expectedID, nil
	}
}

func validateApprovalBinding(targetRoot string, expectedApprovalID string, approval control.PruneApproval, opts ApplyOptions, now time.Time) error {
	if approval.ID != expectedApprovalID {
		return fmt.Errorf("%s: approval id %q does not match requested id %q", ReasonApprovalScopeMismatch, approval.ID, expectedApprovalID)
	}
	if approval.Status != "approved" {
		return fmt.Errorf("%s: approval status %q is not approved", ReasonApprovalScopeMismatch, approval.Status)
	}
	if approval.ProfileID != opts.ProfileID {
		return fmt.Errorf("%s: approval profile_id %q does not match requested profile_id %q", ReasonApprovalScopeMismatch, approval.ProfileID, opts.ProfileID)
	}
	if approval.TargetID != opts.TargetID {
		return fmt.Errorf("%s: approval target_id %q does not match requested target_id %q", ReasonApprovalScopeMismatch, approval.TargetID, opts.TargetID)
	}
	wantPolicy := controlPruneDeletePolicy(opts.DeletePolicy)
	if approval.ProfileDeletePolicy != wantPolicy {
		return fmt.Errorf("%s: approval profile delete policy does not match profile policy", ReasonApprovalScopeMismatch)
	}
	approvedAt, err := time.Parse(time.RFC3339Nano, approval.ApprovedAt)
	if err != nil {
		return fmt.Errorf("parse approval approved_at: %w", err)
	}
	if now.Before(approvedAt) {
		return fmt.Errorf("%s: approval approved_at %s is in the future", ReasonApprovalScopeMismatch, approval.ApprovedAt)
	}
	if strings.TrimSpace(approval.ExpiresAt) != "" {
		expiresAt, err := time.Parse(time.RFC3339Nano, approval.ExpiresAt)
		if err != nil {
			return fmt.Errorf("parse approval expires_at: %w", err)
		}
		if !now.Before(expiresAt) {
			return fmt.Errorf("%s: approval expired at %s", ReasonApprovalScopeMismatch, approval.ExpiresAt)
		}
	}
	if len(approval.Items) == 0 {
		return fmt.Errorf("%s: approval has no items", ReasonApprovalScopeMismatch)
	}
	snapshotPath, err := control.Path(targetRoot, control.ArtifactProfileSnapshot, approval.ProfileSnapshotID)
	if err != nil {
		return err
	}
	snapshot, err := readControlFileNoSymlink[control.ProfileSnapshot](targetRoot, snapshotPath, "approval profile snapshot")
	if err != nil {
		return fmt.Errorf("%s: read approval profile snapshot: %w", ReasonApprovalScopeMismatch, err)
	}
	snapshotDigest, err := ProfileSnapshotDigest(snapshot.Profile)
	if err != nil {
		return fmt.Errorf("%s: approval profile snapshot payload is invalid: %w", ReasonApprovalScopeMismatch, err)
	}
	if snapshotDigest != approval.ProfileSnapshotDigest {
		return fmt.Errorf("%s: approval profile snapshot digest does not match snapshot payload", ReasonApprovalScopeMismatch)
	}
	if snapshot.ID != approval.ProfileSnapshotID {
		return fmt.Errorf("%s: approval profile snapshot id %q does not match requested snapshot id %q", ReasonApprovalScopeMismatch, snapshot.ID, approval.ProfileSnapshotID)
	}
	if snapshot.ProfileID != opts.ProfileID {
		return fmt.Errorf("%s: approval profile snapshot profile_id %q does not match requested profile_id %q", ReasonApprovalScopeMismatch, snapshot.ProfileID, opts.ProfileID)
	}
	snapshotProfile, err := profile.Read(bytes.NewReader(snapshot.Profile))
	if err != nil {
		return fmt.Errorf("%s: approval profile snapshot payload is invalid: %w", ReasonApprovalScopeMismatch, err)
	}
	if snapshotProfile.ProfileID != opts.ProfileID {
		return fmt.Errorf("%s: approval profile snapshot payload profile_id %q does not match requested profile_id %q", ReasonApprovalScopeMismatch, snapshotProfile.ProfileID, opts.ProfileID)
	}
	if snapshotProfile.Target.TargetID != opts.TargetID {
		return fmt.Errorf("%s: approval profile snapshot target_id %q does not match requested target_id %q", ReasonApprovalScopeMismatch, snapshotProfile.Target.TargetID, opts.TargetID)
	}
	if controlPruneDeletePolicy(snapshotProfile.DeletePolicy) != wantPolicy {
		return fmt.Errorf("%s: approval profile snapshot delete policy does not match requested profile policy", ReasonApprovalScopeMismatch)
	}
	if !profileHasRoot(snapshotProfile, approval.RootID) {
		return fmt.Errorf("%s: approval root_id %q is not present in profile snapshot", ReasonApprovalScopeMismatch, approval.RootID)
	}
	if err := validateApprovalItems(approval); err != nil {
		return err
	}
	wantDigest := ApprovalScopeDigest(approval.ProfileID, approval.TargetID, approval.RootID, approval.ProfileSnapshotID, approval.ProfileSnapshotDigest, opts.DeletePolicy, approval.Items)
	if approval.ApprovalScopeDigest != wantDigest {
		return fmt.Errorf("%s: approval_scope_digest does not match approved items and profile policy", ReasonApprovalScopeMismatch)
	}
	return nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ProfileSnapshotDigest(profilePayload json.RawMessage) (string, error) {
	var canonical any
	if err := json.Unmarshal(profilePayload, &canonical); err != nil {
		return "", err
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	return digestBytes(data), nil
}

func validateApprovalItems(approval control.PruneApproval) error {
	seenSoftDeletes := map[string]struct{}{}
	seenTargets := map[string]struct{}{}
	for _, item := range approval.Items {
		if item.RootID != approval.RootID {
			return fmt.Errorf("%s: approval item %q root_id %q does not match approval root_id %q", ReasonApprovalScopeMismatch, item.SoftDeleteID, item.RootID, approval.RootID)
		}
		if _, ok := seenSoftDeletes[item.SoftDeleteID]; ok {
			return fmt.Errorf("%s: duplicate approved soft_delete_id %q", ReasonApprovalScopeMismatch, item.SoftDeleteID)
		}
		seenSoftDeletes[item.SoftDeleteID] = struct{}{}
		if _, ok := seenTargets[item.TargetPath]; ok {
			return fmt.Errorf("%s: duplicate approved target_path %q", ReasonApprovalScopeMismatch, item.TargetPath)
		}
		seenTargets[item.TargetPath] = struct{}{}
	}
	return nil
}

func profileHasRoot(p profile.Profile, rootID string) bool {
	for _, root := range p.Roots {
		if root.ID == rootID {
			return true
		}
	}
	return false
}

func startedReceipt(pruneSessionID string, approval control.PruneApproval, report DryRunReport, now time.Time) control.PruneReceipt {
	items, refusals := plannedReceiptItems(approval, report)
	return control.PruneReceipt{
		Version:             control.CurrentVersion,
		ID:                  pruneSessionID,
		PruneSessionID:      pruneSessionID,
		ApprovalID:          approval.ID,
		ProfileID:           approval.ProfileID,
		TargetID:            approval.TargetID,
		StartedAt:           now.Format(time.RFC3339Nano),
		Status:              control.PruneReceiptStarted,
		DryRun:              false,
		ApprovalScopeDigest: approval.ApprovalScopeDigest,
		Items:               items,
		Refusals:            refusals,
	}
}

func plannedReceiptItems(approval control.PruneApproval, report DryRunReport) ([]control.PruneReceiptItem, []control.PruneRefusal) {
	candidates := make(map[string]Candidate, len(report.Candidates))
	for _, candidate := range report.Candidates {
		candidates[candidate.SoftDeleteID] = candidate
	}
	refusalsBySoftDelete := make(map[string]Refusal, len(report.Refusals))
	for _, refusal := range report.Refusals {
		if refusal.SoftDeleteID != "" {
			refusalsBySoftDelete[refusal.SoftDeleteID] = refusal
		}
	}
	items := make([]control.PruneReceiptItem, 0, len(approval.Items))
	refusals := make([]control.PruneRefusal, 0)
	for _, approved := range approval.Items {
		item := control.PruneReceiptItem{
			SoftDeleteID:   approved.SoftDeleteID,
			TargetPath:     approved.TargetPath,
			IntendedAction: receiptIntendedAction(approved.Kind),
			Result:         "would_prune",
		}
		if candidate, ok := candidates[approved.SoftDeleteID]; ok {
			item.PrePruneObserved = candidate.CurrentTargetState
		}
		if currentRefusal, ok := refusalsBySoftDelete[approved.SoftDeleteID]; ok {
			item.Result = "refused"
			item.ErrorCode = currentRefusal.ReasonCode
			item.Error = currentRefusal.Message
			item.PrePruneObserved = currentRefusal.CurrentTargetState
			refusals = append(refusals, control.PruneRefusal{
				SoftDeleteID: approved.SoftDeleteID,
				TargetPath:   approved.TargetPath,
				ReasonCode:   currentRefusal.ReasonCode,
				Message:      currentRefusal.Message,
			})
		}
		items = append(items, item)
	}
	return items, refusals
}

func readExistingReceipt(path string, pruneSessionID string, approval control.PruneApproval, opts ApplyOptions) (control.PruneReceipt, bool, error) {
	receipt, err := readControlFileNoSymlink[control.PruneReceipt](opts.TargetRoot, path, "prune receipt")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return control.PruneReceipt{}, false, nil
		}
		return control.PruneReceipt{}, false, fmt.Errorf("read existing prune receipt: %w", err)
	}
	if receipt.ID != pruneSessionID ||
		receipt.PruneSessionID != pruneSessionID ||
		receipt.DryRun ||
		receipt.ApprovalID != approval.ID ||
		receipt.ProfileID != opts.ProfileID ||
		receipt.TargetID != opts.TargetID ||
		receipt.ApprovalScopeDigest != approval.ApprovalScopeDigest {
		return control.PruneReceipt{}, false, fmt.Errorf("existing prune receipt %q does not match requested approval", path)
	}
	if receipt.Status == control.PruneReceiptStarted {
		return control.PruneReceipt{}, false, fmt.Errorf("existing prune receipt %q is started; inspect interrupted prune before retry", path)
	}
	return receipt, true, nil
}

func applyApprovedItems(targetRoot string, targetRootInfo fs.FileInfo, approval control.PruneApproval, report DryRunReport, now time.Time) ([]control.PruneReceiptItem, []control.PruneRefusal) {
	candidates := make(map[string]Candidate, len(report.Candidates))
	for _, candidate := range report.Candidates {
		candidates[candidate.SoftDeleteID] = candidate
	}
	refusalsBySoftDelete := make(map[string]Refusal, len(report.Refusals))
	for _, refusal := range report.Refusals {
		if refusal.SoftDeleteID != "" {
			refusalsBySoftDelete[refusal.SoftDeleteID] = refusal
		}
	}

	items := make([]control.PruneReceiptItem, 0, len(approval.Items))
	refusals := make([]control.PruneRefusal, 0)
	blockedByArtifactProblems := len(report.ArtifactProblems) > 0
	for _, approved := range approval.Items {
		if blockedByArtifactProblems {
			item, refusal := refusedReceiptItem(targetRoot, approved, ReasonArtifactProblems, "artifact problems exist in current prune evidence", control.PruneObservedTargetState{})
			items = append(items, item)
			refusals = append(refusals, refusal)
			continue
		}
		if currentRefusal, ok := refusalsBySoftDelete[approved.SoftDeleteID]; ok {
			observed := currentRefusal.CurrentTargetState
			if pruneObservedStateEmpty(observed) {
				observed = observeApprovedItem(targetRoot, approved)
			}
			item, refusal := refusedReceiptItem(targetRoot, approved, currentRefusal.ReasonCode, currentRefusal.Message, observed)
			items = append(items, item)
			refusals = append(refusals, refusal)
			continue
		}
		candidate, ok := candidates[approved.SoftDeleteID]
		if !ok {
			item, refusal := refusedReceiptItem(targetRoot, approved, ReasonCurrentCandidateMissing, "approved soft-delete item is not a current prune candidate", observeApprovedItem(targetRoot, approved))
			items = append(items, item)
			refusals = append(refusals, refusal)
			continue
		}
		if reason := approvalItemMismatch(approved, candidate); reason != "" {
			item, refusal := refusedReceiptItem(targetRoot, approved, ReasonApprovalItemMismatch, reason, candidate.CurrentTargetState)
			items = append(items, item)
			refusals = append(refusals, refusal)
			continue
		}
		item := pruneCandidate(targetRoot, targetRootInfo, candidate, approved, now)
		items = append(items, item)
		if item.Result == "refused" || item.Result == "failed" {
			refusals = append(refusals, control.PruneRefusal{
				SoftDeleteID: approved.SoftDeleteID,
				TargetPath:   approved.TargetPath,
				ReasonCode:   item.ErrorCode,
				Message:      item.Error,
			})
		}
	}
	return items, refusals
}

func approvalItemMismatch(approved control.PruneApprovalItem, candidate Candidate) string {
	if approved.DetectedSessionID != candidate.DetectedSessionID {
		return fmt.Sprintf("approved detected_session_id %q does not match current candidate %q", approved.DetectedSessionID, candidate.DetectedSessionID)
	}
	if approved.PreviousSessionID != candidate.PreviousSessionID {
		return fmt.Sprintf("approved previous_session_id %q does not match current candidate %q", approved.PreviousSessionID, candidate.PreviousSessionID)
	}
	if approved.PreviousManifestID != candidate.PreviousManifestID {
		return fmt.Sprintf("approved previous_manifest_id %q does not match current candidate %q", approved.PreviousManifestID, candidate.PreviousManifestID)
	}
	if approved.RootID != candidate.RootID {
		return fmt.Sprintf("approved root_id %q does not match current candidate %q", approved.RootID, candidate.RootID)
	}
	if approved.SourcePath != candidate.SourcePath {
		return fmt.Sprintf("approved source_path %q does not match current candidate %q", approved.SourcePath, candidate.SourcePath)
	}
	if approved.TargetPath != candidate.TargetPath {
		return fmt.Sprintf("approved target_path %q does not match current candidate %q", approved.TargetPath, candidate.TargetPath)
	}
	if approved.Kind != candidate.Kind {
		return fmt.Sprintf("approved kind %q does not match current candidate %q", approved.Kind, candidate.Kind)
	}
	if approved.Size != candidate.Size {
		return fmt.Sprintf("approved size %d does not match current candidate %d", approved.Size, candidate.Size)
	}
	if approved.Digest != candidate.Digest {
		return "approved digest does not match current candidate digest"
	}
	if approved.SymlinkTarget != candidate.PreviousManifestEntry.SymlinkTarget {
		return "approved symlink target does not match current candidate previous manifest symlink target"
	}
	if approved.DetectedAt != candidate.DetectedAt {
		return fmt.Sprintf("approved detected_at %q does not match current candidate %q", approved.DetectedAt, candidate.DetectedAt)
	}
	return ""
}

// ApprovalItemMismatch reports why an approved item no longer matches a
// current dry-run candidate. It is shared by apply and read-only release review.
func ApprovalItemMismatch(approved control.PruneApprovalItem, candidate Candidate) string {
	return approvalItemMismatch(approved, candidate)
}

func pruneCandidate(targetRoot string, targetRootInfo fs.FileInfo, candidate Candidate, approved control.PruneApprovalItem, now time.Time) control.PruneReceiptItem {
	item := control.PruneReceiptItem{
		SoftDeleteID:     approved.SoftDeleteID,
		TargetPath:       approved.TargetPath,
		IntendedAction:   receiptIntendedAction(candidate.Kind),
		PrePruneObserved: candidate.CurrentTargetState,
	}
	if candidate.Kind != "file" && candidate.Kind != "symlink" {
		item.Result = "refused"
		item.ErrorCode = ReasonUnsupportedKind
		item.Error = fmt.Sprintf("physical prune supports file and symlink targets, got %q", candidate.Kind)
		return item
	}
	fullPath, err := safeTargetPath(targetRoot, candidate.TargetPath)
	if err != nil {
		item.Result = "refused"
		item.ErrorCode = ReasonUnsafeTargetPath
		item.Error = err.Error()
		return item
	}
	if err := pathguard.EnsureDirectory(targetRoot, filepath.Dir(fullPath)); err != nil {
		item.Result = "refused"
		item.ErrorCode = ReasonUnsafeTargetParent
		item.Error = err.Error()
		return item
	}
	observed, err := inspectTarget(fullPath, candidate.TargetPath, candidate.Kind == "file")
	if err != nil {
		item.PrePruneObserved = observed
		item.Result = "refused"
		if errors.Is(err, fs.ErrNotExist) {
			item.ErrorCode = ReasonTargetMissing
		} else {
			item.ErrorCode = ReasonTargetReadError
		}
		item.Error = err.Error()
		return item
	}
	item.PrePruneObserved = observed
	if reason := observedStateMismatch(candidate.CurrentTargetState, observed); reason != "" {
		item.Result = "refused"
		item.ErrorCode = ReasonTargetChangedBeforePrune
		item.Error = reason
		return item
	}
	if err := removeApprovedTarget(targetRoot, targetRootInfo, candidate.TargetPath, candidate.Kind, observed); err != nil {
		item.Result = "failed"
		item.ErrorCode = ReasonRemoveFailed
		item.Error = err.Error()
		return item
	}
	item.Result = "pruned"
	item.PrunedAt = now.Format(time.RFC3339Nano)
	return item
}

func removeApprovedTarget(targetRoot string, targetRootInfo fs.FileInfo, targetRel, kind string, expected control.PruneObservedTargetState) error {
	if err := pathguard.ValidateSlashRelativePath(targetRel, 0); err != nil {
		return err
	}
	if pathguard.IsReservedControlPath(targetRel) {
		return fmt.Errorf("reserved control-plane target path %q", targetRel)
	}
	if beforeRemoveApprovedTarget != nil {
		if err := beforeRemoveApprovedTarget(targetRoot, targetRel); err != nil {
			return err
		}
	}
	root, err := openVerifiedTargetRoot(targetRoot, targetRootInfo)
	if err != nil {
		return err
	}
	defer root.Close()
	observed, err := inspectRootedTarget(root, targetRel, kind == "file")
	if err != nil {
		return err
	}
	if reason := observedStateMismatch(expected, observed); reason != "" {
		return errors.New(reason)
	}
	if err := root.Remove(targetRel); err != nil {
		return err
	}
	return syncRootedParentDirBestEffort(root, targetRel)
}

func syncRootedParentDirBestEffort(root *os.Root, targetRel string) error {
	parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(targetRel)))
	if parent == "." {
		parent = "."
	}
	dir, err := root.Open(parent)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return nil
	}
	return nil
}

func receiptIntendedAction(kind string) string {
	switch kind {
	case "symlink":
		return "delete_symlink"
	default:
		return "delete_file"
	}
}

func observedStateMismatch(expected, observed control.PruneObservedTargetState) string {
	if expected.Present == nil || observed.Present == nil || *expected.Present != *observed.Present {
		return "target presence changed before prune"
	}
	if expected.Kind != observed.Kind {
		return fmt.Sprintf("target kind changed from %q to %q before prune", expected.Kind, observed.Kind)
	}
	if expected.Path != observed.Path {
		return fmt.Sprintf("target path evidence changed from %q to %q before prune", expected.Path, observed.Path)
	}
	if expected.HasSizeEvidence() != observed.HasSizeEvidence() || (expected.HasSizeEvidence() && expected.Size != observed.Size) {
		return "target size evidence changed before prune"
	}
	if expected.Digest != observed.Digest {
		return "target digest changed before prune"
	}
	if expected.HasModeEvidence() != observed.HasModeEvidence() || (expected.HasModeEvidence() && expected.Mode != observed.Mode) {
		return "target mode evidence changed before prune"
	}
	if expected.ModTime != observed.ModTime {
		return "target mod_time changed before prune"
	}
	if expected.SymlinkTarget != observed.SymlinkTarget {
		return "target symlink target changed before prune"
	}
	return ""
}

func refusedReceiptItem(targetRoot string, approved control.PruneApprovalItem, reasonCode, message string, observed control.PruneObservedTargetState) (control.PruneReceiptItem, control.PruneRefusal) {
	if pruneObservedStateEmpty(observed) {
		observed = observeApprovedItem(targetRoot, approved)
	}
	return control.PruneReceiptItem{
			SoftDeleteID:     approved.SoftDeleteID,
			TargetPath:       approved.TargetPath,
			IntendedAction:   receiptIntendedAction(approved.Kind),
			PrePruneObserved: observed,
			Result:           "refused",
			ErrorCode:        reasonCode,
			Error:            message,
		}, control.PruneRefusal{
			SoftDeleteID: approved.SoftDeleteID,
			TargetPath:   approved.TargetPath,
			ReasonCode:   reasonCode,
			Message:      message,
		}
}

func observeApprovedItem(targetRoot string, approved control.PruneApprovalItem) control.PruneObservedTargetState {
	fullPath, err := safeTargetPath(targetRoot, approved.TargetPath)
	if err != nil {
		return control.PruneObservedTargetState{}
	}
	if err := pathguard.EnsureDirectory(targetRoot, filepath.Dir(fullPath)); err != nil {
		return presentObserved(approved.TargetPath, "other")
	}
	observed, err := inspectTarget(fullPath, approved.TargetPath, approved.Kind == "file")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return missingObserved(approved.TargetPath)
		}
		return observed
	}
	return observed
}

func receiptStatus(items []control.PruneReceiptItem) control.PruneReceiptStatus {
	pruned := 0
	refusedOrFailed := 0
	for _, item := range items {
		switch item.Result {
		case "pruned":
			pruned++
		case "refused", "failed":
			refusedOrFailed++
		}
	}
	if pruned > 0 && refusedOrFailed == 0 {
		return control.PruneReceiptApplied
	}
	if pruned > 0 {
		return control.PruneReceiptPartial
	}
	return control.PruneReceiptFailed
}

func requireExistingTargetRoot(targetRoot string) error {
	info, err := os.Lstat(targetRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("target root does not exist: %s", targetRoot)
		}
		return fmt.Errorf("inspect target root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("target root must be a directory, not a symlink: %s", targetRoot)
	}
	if !info.IsDir() {
		return fmt.Errorf("target root must be a directory: %s", targetRoot)
	}
	controlDir := control.ControlDir(targetRoot)
	info, err = os.Lstat(controlDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("target control plane is missing: %s", controlDir)
		}
		return fmt.Errorf("inspect target control plane: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("target control plane must be a directory, not a symlink: %s", controlDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("target control plane must be a directory: %s", controlDir)
	}
	return nil
}

func verifiedTargetRootInfo(targetRoot string) (fs.FileInfo, error) {
	info, err := os.Lstat(targetRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect target root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("target root must be a directory, not a symlink: %s", targetRoot)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("target root must be a directory: %s", targetRoot)
	}
	return info, nil
}

func openVerifiedTargetRoot(targetRoot string, expected fs.FileInfo) (*os.Root, error) {
	beforeOpen, err := verifiedTargetRootInfo(targetRoot)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(expected, beforeOpen) {
		return nil, fmt.Errorf("target root changed before prune: %s", targetRoot)
	}
	root, err := os.OpenRoot(targetRoot)
	if err != nil {
		return nil, err
	}
	opened, err := root.Lstat(".")
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("inspect opened target root: %w", err)
	}
	if !os.SameFile(expected, opened) {
		_ = root.Close()
		return nil, fmt.Errorf("target root changed before prune: %s", targetRoot)
	}
	return root, nil
}

func planRecord(targetRoot string, record control.SoftDelete, manifestBySession map[string]control.Manifest, manifests []control.Manifest, policy profile.DeletePolicy, now time.Time) (Candidate, *Refusal) {
	manifest, ok := manifestBySession[record.PreviousSessionID]
	if !ok {
		return Candidate{}, recordRefusal(record, ReasonPreviousManifestMissing, fmt.Sprintf("previous manifest session %q is missing", record.PreviousSessionID), nil, control.PruneObservedTargetState{})
	}
	if manifest.ID != record.PreviousManifestID {
		return Candidate{}, recordRefusal(record, ReasonPreviousManifestIDMismatch, fmt.Sprintf("previous manifest id %q does not match soft-delete evidence %q", manifest.ID, record.PreviousManifestID), nil, control.PruneObservedTargetState{})
	}
	entry, ok := previousManifestEntry(record, manifest)
	if !ok {
		return Candidate{}, recordRefusal(record, ReasonPreviousManifestEntryMissing, "previous manifest entry for soft-delete target path is missing", nil, control.PruneObservedTargetState{})
	}
	previous := previousEvidence(manifest, entry)
	if reason := previousEvidenceMismatch(record, previous); reason != "" {
		return Candidate{}, recordRefusal(record, ReasonPreviousManifestEntryMismatch, reason, &previous, control.PruneObservedTargetState{})
	}
	if entry.Kind != "file" && entry.Kind != "symlink" {
		return Candidate{}, recordRefusal(record, ReasonUnsupportedKind, fmt.Sprintf("prune dry-run supports file and symlink targets, got %q", entry.Kind), &previous, control.PruneObservedTargetState{})
	}
	if reason := weakPreviousEvidence(entry); reason != "" {
		return Candidate{}, recordRefusal(record, ReasonWeakPreviousEvidence, reason, &previous, control.PruneObservedTargetState{})
	}
	if laterManifest, ok := laterManifestContainsTarget(record, manifests); ok {
		return Candidate{}, recordRefusal(record, ReasonTargetReappeared, fmt.Sprintf("target path %q appears in later published manifest %q", record.TargetPath, laterManifest.ID), &previous, control.PruneObservedTargetState{})
	}

	fullPath, err := safeTargetPath(targetRoot, record.TargetPath)
	if err != nil {
		return Candidate{}, recordRefusal(record, ReasonUnsafeTargetPath, fmt.Sprintf("target path is unsafe: %v", err), &previous, control.PruneObservedTargetState{})
	}
	if err := pathguard.EnsureDirectory(targetRoot, filepath.Dir(fullPath)); err != nil {
		observed := presentObserved(record.TargetPath, "other")
		return Candidate{}, recordRefusal(record, ReasonUnsafeTargetParent, fmt.Sprintf("target parent is unsafe: %v", err), &previous, observed)
	}
	observed, err := inspectTarget(fullPath, record.TargetPath, true)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Candidate{}, recordRefusal(record, ReasonTargetMissing, "target path is already missing or already pruned", &previous, missingObserved(record.TargetPath))
		}
		return Candidate{}, recordRefusal(record, ReasonTargetReadError, fmt.Sprintf("could not inspect target path: %v", err), &previous, observed)
	}
	if observed.Kind != entry.Kind {
		return Candidate{}, recordRefusal(record, ReasonTargetKindMismatch, fmt.Sprintf("target kind %q does not match previous manifest kind %q", observed.Kind, entry.Kind), &previous, observed)
	}
	if reason := observedMismatch(entry, observed); reason != "" {
		return Candidate{}, recordRefusal(record, ReasonTargetContentMismatch, reason, &previous, observed)
	}
	if refusal := retentionWindowRefusal(record, policy, now, &previous, observed); refusal != nil {
		return Candidate{}, refusal
	}

	return Candidate{
		SoftDeleteID:          record.ID,
		DetectedSessionID:     record.SessionID,
		ProfileID:             record.ProfileID,
		TargetID:              record.TargetID,
		RootID:                record.RootID,
		SourcePath:            record.SourcePath,
		TargetPath:            record.TargetPath,
		Kind:                  record.Kind,
		Size:                  record.Size,
		Digest:                record.Digest,
		DetectedAt:            record.DetectedAt,
		PreviousSessionID:     record.PreviousSessionID,
		PreviousManifestID:    record.PreviousManifestID,
		PreviousManifestEntry: previous,
		CurrentTargetState:    observed,
		IntendedAction:        intendedAction(record.Kind),
		PhysicalPruning:       "not_applied",
		ApprovalWriting:       "not_written_by_dry_run",
		ReceiptWriting:        "not_written_by_dry_run",
		ReviewRequired:        true,
	}, nil
}

func retentionWindowRefusal(record control.SoftDelete, policy profile.DeletePolicy, now time.Time, previous *PreviousManifestEvidence, observed control.PruneObservedTargetState) *Refusal {
	if policy.RetentionDays < 0 {
		return recordRefusal(record, ReasonRetentionWindowActive, fmt.Sprintf("profile delete_policy.retention_days is invalid: retention_days cannot be negative"), previous, observed)
	}
	if policy.RetentionDays == 0 {
		return nil
	}
	detectedAt, err := time.Parse(time.RFC3339Nano, record.DetectedAt)
	if err != nil {
		return recordRefusal(record, ReasonRetentionWindowActive, fmt.Sprintf("soft-delete detected_at is invalid: %v", err), previous, observed)
	}
	duration, err := retentionDuration(policy.RetentionDays)
	if err != nil {
		return recordRefusal(record, ReasonRetentionWindowActive, fmt.Sprintf("profile delete_policy.retention_days is invalid: %v", err), previous, observed)
	}
	eligibleAt := detectedAt.UTC().Add(duration)
	if now.Before(eligibleAt) {
		return recordRefusal(record, ReasonRetentionWindowActive, fmt.Sprintf("soft-delete retention window active until %s", eligibleAt.Format(time.RFC3339Nano)), previous, observed)
	}
	return nil
}

func retentionDuration(days int) (time.Duration, error) {
	if days < 0 {
		return 0, errors.New("retention_days cannot be negative")
	}
	const day = 24 * time.Hour
	const maxDays = int64(1<<63-1) / int64(day)
	if int64(days) > maxDays {
		return 0, fmt.Errorf("retention_days %d is too large", days)
	}
	return time.Duration(days) * day, nil
}

func weakPreviousEvidence(entry control.ManifestEntry) string {
	if entry.Kind != "file" {
		return ""
	}
	if strings.TrimSpace(entry.Digest) == "" {
		return "previous manifest file entry has no digest evidence"
	}
	if !entry.HasSizeEvidence() {
		return "previous manifest file entry has no size evidence"
	}
	return ""
}

func laterManifestContainsTarget(record control.SoftDelete, manifests []control.Manifest) (control.Manifest, bool) {
	afterRecord := false
	for _, manifest := range manifests {
		if manifest.SessionID == record.SessionID {
			afterRecord = true
			continue
		}
		if !afterRecord {
			continue
		}
		if manifestContainsTarget(manifest, record.TargetPath) {
			return manifest, true
		}
	}
	return control.Manifest{}, false
}

func manifestContainsTarget(manifest control.Manifest, targetRel string) bool {
	for _, entry := range manifest.Entries {
		if targetPath(entry) == targetRel {
			return true
		}
	}
	return false
}

func previousManifestEntry(record control.SoftDelete, manifest control.Manifest) (control.ManifestEntry, bool) {
	for _, entry := range manifest.Entries {
		if targetPath(entry) == record.TargetPath && filepath.ToSlash(entry.Path) == record.SourcePath {
			return entry, true
		}
	}
	for _, entry := range manifest.Entries {
		if targetPath(entry) == record.TargetPath {
			return entry, true
		}
	}
	return control.ManifestEntry{}, false
}

func previousEvidence(manifest control.Manifest, entry control.ManifestEntry) PreviousManifestEvidence {
	evidence := PreviousManifestEvidence{
		SessionID:     manifest.SessionID,
		ManifestID:    manifest.ID,
		RootID:        manifest.RootID,
		SourcePath:    filepath.ToSlash(entry.Path),
		TargetPath:    targetPath(entry),
		Kind:          entry.Kind,
		Digest:        entry.Digest,
		ModTime:       entry.ModTime,
		SymlinkTarget: entry.SymlinkTarget,
	}
	if entry.HasSizeEvidence() {
		evidence.Size = entry.Size
	}
	if entry.HasModeEvidence() {
		evidence.Mode = entry.Mode
	}
	return evidence
}

func previousEvidenceMismatch(record control.SoftDelete, previous PreviousManifestEvidence) string {
	if previous.SourcePath != record.SourcePath {
		return fmt.Sprintf("previous manifest source path %q does not match soft-delete source path %q", previous.SourcePath, record.SourcePath)
	}
	if previous.TargetPath != record.TargetPath {
		return fmt.Sprintf("previous manifest target path %q does not match soft-delete target path %q", previous.TargetPath, record.TargetPath)
	}
	if previous.Kind != record.Kind {
		return fmt.Sprintf("previous manifest kind %q does not match soft-delete kind %q", previous.Kind, record.Kind)
	}
	if record.Kind == "file" {
		if record.Size != 0 && previous.Size != 0 && previous.Size != record.Size {
			return fmt.Sprintf("previous manifest size %d does not match soft-delete size %d", previous.Size, record.Size)
		}
		if record.Digest != "" && previous.Digest != "" && previous.Digest != record.Digest {
			return "previous manifest digest does not match soft-delete digest"
		}
	}
	return ""
}

func observedMismatch(entry control.ManifestEntry, observed control.PruneObservedTargetState) string {
	switch entry.Kind {
	case "file":
		if entry.HasSizeEvidence() && observed.HasSizeEvidence() && entry.Size != observed.Size {
			return fmt.Sprintf("target size %d does not match previous manifest size %d", observed.Size, entry.Size)
		}
		if strings.TrimSpace(entry.Digest) != "" && strings.TrimSpace(observed.Digest) != "" && entry.Digest != observed.Digest {
			return "target digest does not match previous manifest digest"
		}
	case "symlink":
		if entry.SymlinkTarget != observed.SymlinkTarget {
			return "target symlink target does not match previous manifest symlink target"
		}
	}
	return ""
}

func inspectTarget(path, targetRel string, includeDigest bool) (control.PruneObservedTargetState, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return missingObserved(targetRel), err
	}
	observed := control.PruneObservedTargetState{
		Present: boolPtr(true),
		Kind:    kindFromInfo(info),
		Path:    targetRel,
		ModTime: info.ModTime().UTC().Format(time.RFC3339Nano),
	}
	observed.SetModeEvidence(uint32(info.Mode().Perm()))
	switch observed.Kind {
	case "file":
		observed.SetSizeEvidence(info.Size())
		if includeDigest {
			digest, err := stableFileDigest(path, info)
			if err != nil {
				return observed, err
			}
			observed.Digest = digest
		}
	case "symlink":
		target, err := os.Readlink(path)
		if err != nil {
			return observed, err
		}
		if err := pathguard.ValidateRelativeSymlinkTarget(target); err != nil {
			return observed, err
		}
		observed.SymlinkTarget = target
	}
	return observed, nil
}

func inspectRootedTarget(root *os.Root, targetRel string, includeDigest bool) (control.PruneObservedTargetState, error) {
	info, err := root.Lstat(targetRel)
	if err != nil {
		return missingObserved(targetRel), err
	}
	observed := control.PruneObservedTargetState{
		Present: boolPtr(true),
		Kind:    kindFromInfo(info),
		Path:    targetRel,
		ModTime: info.ModTime().UTC().Format(time.RFC3339Nano),
	}
	observed.SetModeEvidence(uint32(info.Mode().Perm()))
	switch observed.Kind {
	case "file":
		observed.SetSizeEvidence(info.Size())
		if includeDigest {
			digest, err := stableRootedFileDigest(root, targetRel, info)
			if err != nil {
				return observed, err
			}
			observed.Digest = digest
		}
	case "symlink":
		target, err := root.Readlink(targetRel)
		if err != nil {
			return observed, err
		}
		if err := pathguard.ValidateRelativeSymlinkTarget(target); err != nil {
			return observed, err
		}
		observed.SymlinkTarget = target
	}
	return observed, nil
}

func stableFileDigest(path string, observed fs.FileInfo) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(observed, opened) {
		return "", errors.New("regular file changed before digest")
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !after.Mode().IsRegular() || !os.SameFile(observed, after) || after.Size() != observed.Size() || after.Mode().Perm() != observed.Mode().Perm() || !after.ModTime().Equal(observed.ModTime()) {
		return "", errors.New("regular file changed during digest")
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func stableRootedFileDigest(root *os.Root, targetRel string, observed fs.FileInfo) (string, error) {
	file, err := root.Open(targetRel)
	if err != nil {
		return "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(observed, opened) {
		return "", errors.New("regular file changed before digest")
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	after, err := root.Lstat(targetRel)
	if err != nil {
		return "", err
	}
	if !after.Mode().IsRegular() || !os.SameFile(observed, after) || after.Size() != observed.Size() || after.Mode().Perm() != observed.Mode().Perm() || !after.ModTime().Equal(observed.ModTime()) {
		return "", errors.New("regular file changed during digest")
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func safeTargetPath(root, rel string) (string, error) {
	if err := pathguard.ValidateSlashRelativePath(rel, 0); err != nil {
		return "", err
	}
	if pathguard.IsReservedControlPath(rel) {
		return "", fmt.Errorf("reserved control-plane target path %q", rel)
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

func kindFromInfo(info fs.FileInfo) string {
	if info.Mode()&os.ModeSymlink != 0 {
		return "symlink"
	}
	if info.Mode().IsRegular() {
		return "file"
	}
	if info.IsDir() {
		return "dir"
	}
	return "special"
}

func intendedAction(kind string) string {
	switch kind {
	case "file":
		return "delete_file"
	case "symlink":
		return "delete_symlink"
	default:
		return "review_only"
	}
}

func pruneObservedStateEmpty(state control.PruneObservedTargetState) bool {
	return state.Present == nil &&
		strings.TrimSpace(state.Kind) == "" &&
		strings.TrimSpace(state.Path) == "" &&
		!state.HasSizeEvidence() &&
		strings.TrimSpace(state.Digest) == "" &&
		!state.HasModeEvidence() &&
		strings.TrimSpace(state.ModTime) == "" &&
		strings.TrimSpace(state.SymlinkTarget) == ""
}

func targetPath(entry control.ManifestEntry) string {
	if strings.TrimSpace(entry.TargetPath) != "" {
		return filepath.ToSlash(entry.TargetPath)
	}
	return filepath.ToSlash(entry.Path)
}

func filterSoftDeletes(records []control.SoftDelete, sessionID string) []control.SoftDelete {
	if strings.TrimSpace(sessionID) == "" {
		return append([]control.SoftDelete(nil), records...)
	}
	var out []control.SoftDelete
	for _, record := range records {
		if record.SessionID == sessionID {
			out = append(out, record)
		}
	}
	return out
}

func filterVerifyArtifactProblems(problems []verify.ArtifactProblem, sessionID string) []verify.ArtifactProblem {
	if strings.TrimSpace(sessionID) == "" {
		return append([]verify.ArtifactProblem(nil), problems...)
	}
	var out []verify.ArtifactProblem
	for _, problem := range problems {
		if problem.SessionID == "" || problem.SessionID == sessionID {
			out = append(out, problem)
		}
	}
	return out
}

func artifactProblems(problems []verify.ArtifactProblem) []ArtifactProblem {
	out := make([]ArtifactProblem, 0, len(problems))
	for _, problem := range problems {
		out = append(out, ArtifactProblem{
			SessionID: problem.SessionID,
			Path:      problem.Path,
			Error:     problem.Err,
		})
	}
	return out
}

func recordRefusal(record control.SoftDelete, reasonCode string, message string, previous *PreviousManifestEvidence, observed control.PruneObservedTargetState) *Refusal {
	refusal := Refusal{
		SoftDeleteID:       record.ID,
		DetectedSessionID:  record.SessionID,
		SourcePath:         record.SourcePath,
		TargetPath:         record.TargetPath,
		ReasonCode:         reasonCode,
		Message:            message,
		SoftDeleteEvidence: &record,
		CurrentTargetState: observed,
	}
	if previous != nil {
		refusal.PreviousManifest = previous
	}
	return &refusal
}

func missingObserved(targetRel string) control.PruneObservedTargetState {
	return control.PruneObservedTargetState{
		Present: boolPtr(false),
		Kind:    "missing",
		Path:    targetRel,
	}
}

func presentObserved(targetRel, kind string) control.PruneObservedTargetState {
	return control.PruneObservedTargetState{
		Present: boolPtr(true),
		Kind:    kind,
		Path:    targetRel,
	}
}

func boolPtr(value bool) *bool {
	return &value
}
