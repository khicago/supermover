package reconcile

import (
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
	"github.com/khicago/supermover/internal/driftstore"
	"github.com/khicago/supermover/internal/durable"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/targetlock"
	"github.com/khicago/supermover/internal/verify"
)

const (
	SchemaPlanReceipt  = "supermover.reconcile_plan.v1"
	SchemaApplyReceipt = "supermover.reconcile_apply.v1"

	ActionResolveNoop       = "resolve_noop"
	ActionRestoreFile       = "restore_file_from_source"
	ActionAlreadyResolved   = "already_resolved_noop"
	ActionRefuseUnsupported = "refuse_unsupported"

	ResultPlanned = "planned"
	ResultApplied = "applied"
	ResultRefused = "refused"
	ResultNoop    = "noop"

	ReceiptStatusStarted = "started"
	ReceiptStatusApplied = "applied"
	ReceiptStatusPartial = "partial"
	ReceiptStatusFailed  = "failed"

	ReasonAlreadyResolved       = "already_resolved"
	ReasonMissingApplyIntent    = "missing_apply_intent"
	ReasonProfileScopeMismatch  = "profile_scope_mismatch"
	ReasonRootScopeMismatch     = "root_scope_mismatch"
	ReasonControlPlanePath      = "control_plane_path"
	ReasonUnsafeTargetPath      = "unsafe_target_path"
	ReasonUnsafeTargetParent    = "unsafe_target_parent"
	ReasonUnsupportedChange     = "unsupported_change"
	ReasonUnsupportedKind       = "unsupported_kind"
	ReasonAmbiguousState        = "ambiguous_state"
	ReasonArtifactProblems      = "artifact_problems"
	ReasonPublishedEvidence     = "published_evidence_missing"
	ReasonSourceEvidenceMissing = "source_evidence_missing"
	ReasonSourceMismatch        = "source_evidence_mismatch"
	ReasonTargetChanged         = "target_changed_before_apply"
	ReasonMutationFailed        = "mutation_failed"

	ConflictClassOperatorIntent    = "operator_intent"
	ConflictClassScopeMismatch     = "scope_mismatch"
	ConflictClassUnsafePath        = "unsafe_path"
	ConflictClassUnsupportedDrift  = "unsupported_drift"
	ConflictClassTargetState       = "target_state_conflict"
	ConflictClassArtifactIntegrity = "artifact_integrity"
	ConflictClassPublishedEvidence = "published_evidence"
	ConflictClassSourceEvidence    = "source_evidence"
	ConflictClassMutationFailure   = "mutation_failure"

	RetryAdviceFixCommand                = "fix_command_and_retry"
	RetryAdviceSelectMatchingScope       = "select_matching_profile_or_record"
	RetryAdviceManualReview              = "manual_review_required"
	RetryAdviceReviewTargetState         = "review_target_state_before_retry"
	RetryAdviceRepairArtifacts           = "repair_artifacts_before_retry"
	RetryAdviceRestorePublishedEvidence  = "restore_published_evidence_or_republish"
	RetryAdviceRestoreSourceEvidence     = "restore_source_evidence_before_retry"
	RetryAdviceInspectMutationThenReplan = "inspect_mutation_result_then_replan"
)

var (
	chtimes              = os.Chtimes
	promoteFileNoReplace = durable.PromoteFileNoReplace
	afterSourcePreflight = func(string) {}
)

// Options selects profile-scoped durable drift records to plan.
type Options struct {
	Profile   profile.Profile
	IDs       []string
	SessionID string
	Now       time.Time
}

// ApplyOptions requires explicit mutation intent before any target write.
type ApplyOptions struct {
	Profile   profile.Profile
	IDs       []string
	SessionID string
	Apply     bool
	Reviewer  string
	Reason    string
	Now       time.Time
}

type Receipt struct {
	Schema           string            `json:"schema"`
	ID               string            `json:"id,omitempty"`
	Status           string            `json:"status,omitempty"`
	TargetRoot       string            `json:"target_root"`
	ProfileID        string            `json:"profile_id"`
	TargetID         string            `json:"target_id"`
	SessionID        string            `json:"session_id,omitempty"`
	GeneratedAt      string            `json:"generated_at"`
	StartedAt        string            `json:"started_at,omitempty"`
	EndedAt          string            `json:"ended_at,omitempty"`
	ReceiptPath      string            `json:"receipt_path,omitempty"`
	ApplyIntent      bool              `json:"apply_intent"`
	Summary          Summary           `json:"summary"`
	Actions          []Action          `json:"actions,omitempty"`
	Refusals         []Refusal         `json:"refusals,omitempty"`
	ArtifactProblems []ArtifactProblem `json:"artifact_problems,omitempty"`
}

type Summary struct {
	Records          int `json:"records"`
	Planned          int `json:"planned"`
	Applied          int `json:"applied"`
	Noop             int `json:"noop"`
	Refused          int `json:"refused"`
	ArtifactProblems int `json:"artifact_problems"`
}

type Action struct {
	DriftID        string          `json:"drift_id"`
	Path           string          `json:"path"`
	Change         string          `json:"change"`
	Action         string          `json:"action"`
	Result         string          `json:"result"`
	SessionID      string          `json:"session_id"`
	Expected       ExpectedState   `json:"expected"`
	ObservedBefore ObservedState   `json:"observed_before"`
	SourceEvidence *SourceEvidence `json:"source_evidence,omitempty"`
	ReviewedAt     string          `json:"reviewed_at,omitempty"`
	Reviewer       string          `json:"reviewer,omitempty"`
	Reason         string          `json:"reason,omitempty"`
}

type Refusal struct {
	DriftID        string        `json:"drift_id,omitempty"`
	Path           string        `json:"path,omitempty"`
	Change         string        `json:"change,omitempty"`
	SessionID      string        `json:"session_id,omitempty"`
	Action         string        `json:"action,omitempty"`
	ReasonCode     string        `json:"reason_code"`
	ConflictClass  string        `json:"conflict_class,omitempty"`
	RetryAdvice    string        `json:"retry_advice,omitempty"`
	Message        string        `json:"message"`
	ObservedBefore ObservedState `json:"observed_before,omitempty"`
}

type ArtifactProblem struct {
	SessionID string `json:"session_id,omitempty"`
	Path      string `json:"path"`
	Error     string `json:"error"`
}

type ExpectedState struct {
	SessionID  string `json:"session_id,omitempty"`
	ManifestID string `json:"manifest_id,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Path       string `json:"path,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Digest     string `json:"digest,omitempty"`
	Mode       uint32 `json:"mode,omitempty"`
	ModTime    string `json:"mod_time,omitempty"`
}

type ObservedState struct {
	Present *bool  `json:"present,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Path    string `json:"path,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Digest  string `json:"digest,omitempty"`
	Mode    uint32 `json:"mode,omitempty"`
	ModTime string `json:"mod_time,omitempty"`
}

type SourceEvidence struct {
	RootID string `json:"root_id"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
	Mode   uint32 `json:"mode,omitempty"`
}

type candidate struct {
	drift          control.TargetDrift
	action         Action
	refusal        Refusal
	refused        bool
	targetPath     string
	sourcePath     string
	targetObserved ObservedState
}

func Plan(opts Options) (Receipt, error) {
	targetRoot, now, err := normalizeOptions(opts.Profile, opts.Now)
	if err != nil {
		return Receipt{}, err
	}
	receipt, candidates, err := plan(targetRoot, opts.Profile, opts.IDs, opts.SessionID, now, false)
	if err != nil {
		return Receipt{}, err
	}
	_ = candidates
	receipt.Schema = SchemaPlanReceipt
	receipt.ApplyIntent = false
	return receipt, nil
}

func Apply(opts ApplyOptions) (Receipt, error) {
	targetRoot, now, err := normalizeOptions(opts.Profile, opts.Now)
	if err != nil {
		return Receipt{}, err
	}
	if opts.Apply && len(selectedDriftIDs(opts.IDs)) == 0 {
		return Receipt{}, errors.New("at least one persisted target drift id is required when apply intent is true")
	}
	receipt, _, err := plan(targetRoot, opts.Profile, opts.IDs, opts.SessionID, now, true)
	if err != nil {
		return Receipt{}, err
	}
	receipt.Schema = SchemaApplyReceipt
	receipt.ApplyIntent = opts.Apply
	if !opts.Apply {
		refusePlannedForMissingIntent(&receipt)
		return receipt, nil
	}
	if strings.TrimSpace(opts.Reason) == "" {
		return Receipt{}, errors.New("reason is required when apply intent is true")
	}
	unlock, err := targetlock.LockTarget(targetRoot)
	if err != nil {
		return Receipt{}, fmt.Errorf("lock target for reconcile apply: %w", err)
	}
	defer unlock()

	receipt, candidates, err := plan(targetRoot, opts.Profile, opts.IDs, opts.SessionID, now, true)
	if err != nil {
		return Receipt{}, err
	}
	receipt.Schema = SchemaApplyReceipt
	receipt.ApplyIntent = true
	receiptPath, err := writeStartedApplyReceipt(targetRoot, opts.Profile, opts.IDs, opts.SessionID, now, &receipt)
	if err != nil {
		return Receipt{}, err
	}
	if refuseApplySetConflicts(&receipt, candidates) {
		if err := writeFinalApplyReceipt(receiptPath, now, &receipt); err != nil {
			return Receipt{}, err
		}
		return receipt, nil
	}
	for _, item := range candidates {
		if item.refused || item.action.Result != ResultPlanned {
			continue
		}
		applied, refusal := applyCandidate(targetRoot, item, opts, now)
		if refusal.ReasonCode != "" {
			receipt.Refusals = append(receipt.Refusals, refusal)
			continue
		}
		receipt.Actions = replaceAction(receipt.Actions, applied)
	}
	sortReceipt(&receipt)
	summarize(&receipt)
	if err := writeFinalApplyReceipt(receiptPath, now, &receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func normalizeOptions(p profile.Profile, now time.Time) (string, time.Time, error) {
	if strings.TrimSpace(p.Target.LocalPath) == "" {
		return "", time.Time{}, errors.New("target.local_path is required; run profile set-target to persist the trusted target path")
	}
	targetRoot, err := filepath.Abs(p.Target.LocalPath)
	if err != nil {
		return "", time.Time{}, err
	}
	if err := control.ValidateArtifactLoadBoundary(targetRoot); err != nil {
		return "", time.Time{}, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	return targetRoot, now.UTC(), nil
}

func plan(targetRoot string, p profile.Profile, ids []string, sessionID string, now time.Time, forApply bool) (Receipt, []candidate, error) {
	artifacts, err := loadPlanningArtifacts(targetRoot, p, ids)
	if err != nil {
		return Receipt{}, nil, fmt.Errorf("load target artifacts: %w", err)
	}
	receipt := Receipt{
		Schema:      SchemaPlanReceipt,
		TargetRoot:  filepath.ToSlash(targetRoot),
		ProfileID:   p.ProfileID,
		TargetID:    p.Target.TargetID,
		SessionID:   strings.TrimSpace(sessionID),
		GeneratedAt: now.Format(time.RFC3339Nano),
	}
	for _, problem := range filterPlanningArtifactProblems(artifacts.ArtifactProblems, sessionID, ids, artifacts.TargetDrifts) {
		receipt.ArtifactProblems = append(receipt.ArtifactProblems, ArtifactProblem{
			SessionID: problem.SessionID,
			Path:      problem.Path,
			Error:     problem.Err,
		})
	}
	if len(receipt.ArtifactProblems) > 0 {
		receipt.Refusals = append(receipt.Refusals, Refusal{
			ReasonCode:    ReasonArtifactProblems,
			ConflictClass: conflictClassForReason(ReasonArtifactProblems),
			RetryAdvice:   retryAdviceForReason(ReasonArtifactProblems),
			Message:       "target control artifacts have load problems; reconcile planning is refused until artifacts are readable",
		})
		summarize(&receipt)
		return receipt, nil, nil
	}
	selected, err := selectDrifts(artifacts.TargetDrifts, ids, sessionID)
	if err != nil {
		return Receipt{}, nil, err
	}
	manifestBySession := map[string]control.Manifest{}
	for _, manifest := range artifacts.Manifests {
		manifestBySession[manifest.SessionID] = manifest
	}
	var candidates []candidate
	for _, drift := range selected {
		item := classify(targetRoot, p, drift, manifestBySession, now, forApply)
		if item.refused {
			receipt.Refusals = append(receipt.Refusals, item.refusal)
		} else {
			receipt.Actions = append(receipt.Actions, item.action)
		}
		candidates = append(candidates, item)
	}
	sortReceipt(&receipt)
	summarize(&receipt)
	return receipt, candidates, nil
}

func loadPlanningArtifacts(targetRoot string, p profile.Profile, ids []string) (verify.Artifacts, error) {
	artifacts, err := verify.LoadArtifactsForScope(targetRoot, p.ProfileID, p.Target.TargetID)
	if err != nil {
		return verify.Artifacts{}, err
	}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := control.ValidateArtifactID(id); err != nil {
			return verify.Artifacts{}, fmt.Errorf("drift id is unsafe: %w", err)
		}
		if hasTargetDriftID(artifacts.TargetDrifts, id) {
			continue
		}
		path, err := control.Path(targetRoot, control.ArtifactTargetDrift, id)
		if err != nil {
			return verify.Artifacts{}, err
		}
		drift, err := control.ReadFileNoSymlinkUnderRoot[control.TargetDrift](control.ControlDir(targetRoot), path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if !hasArtifactProblemPath(artifacts.ArtifactProblems, path) {
				artifacts.ArtifactProblems = append(artifacts.ArtifactProblems, verify.ArtifactProblem{
					Path: filepath.ToSlash(path),
					Err:  err.Error(),
				})
			}
			continue
		}
		artifacts.TargetDrifts = append(artifacts.TargetDrifts, drift)
	}
	sort.Slice(artifacts.TargetDrifts, func(i, j int) bool { return artifacts.TargetDrifts[i].ID < artifacts.TargetDrifts[j].ID })
	sort.Slice(artifacts.ArtifactProblems, func(i, j int) bool { return artifacts.ArtifactProblems[i].Path < artifacts.ArtifactProblems[j].Path })
	return artifacts, nil
}

func classify(targetRoot string, p profile.Profile, drift control.TargetDrift, manifests map[string]control.Manifest, now time.Time, forApply bool) candidate {
	item := candidate{drift: drift}
	item.action = Action{
		DriftID:   drift.ID,
		Path:      drift.Path,
		Change:    drift.Change,
		SessionID: drift.SessionID,
		Expected:  expectedState(drift.Expected),
	}
	item.refusal = Refusal{
		DriftID:   drift.ID,
		Path:      drift.Path,
		Change:    drift.Change,
		SessionID: drift.SessionID,
	}
	if drift.ProfileID != p.ProfileID || drift.TargetID != p.Target.TargetID {
		return refused(item, ReasonProfileScopeMismatch, fmt.Sprintf("drift scope %q/%q does not match profile scope %q/%q", drift.ProfileID, drift.TargetID, p.ProfileID, p.Target.TargetID))
	}
	if !profileHasRoot(p, drift.RootID) {
		return refused(item, ReasonRootScopeMismatch, fmt.Sprintf("drift root_id %q does not match profile roots", drift.RootID))
	}
	if err := safeDataPath(drift.Path); err != nil {
		return refused(item, refusalCodeForPathError(err), fmt.Sprintf("drift path %q is unsafe: %v", drift.Path, err))
	}
	targetPath, err := safeTargetPath(targetRoot, drift.Path)
	if err != nil {
		return refused(item, refusalCodeForPathError(err), fmt.Sprintf("drift target path %q is unsafe: %v", drift.Path, err))
	}
	item.targetPath = targetPath
	observed, err := observeTarget(targetRoot, drift.Path, drift.Expected.Digest != "")
	if err != nil {
		return refused(item, ReasonUnsafeTargetParent, fmt.Sprintf("inspect target path %q: %v", drift.Path, err))
	}
	item.targetObserved = observed
	item.action.ObservedBefore = observed
	item.refusal.ObservedBefore = observed

	switch driftstore.ReviewState(drift) {
	case "resolved", "expired":
		item.action.Action = ActionAlreadyResolved
		item.action.Result = ResultNoop
		item.action.ReviewedAt = drift.ReviewedAt
		item.action.Reviewer = drift.ReviewedBy
		item.action.Reason = drift.ReviewReason
		return item
	}
	if drift.Change == "missing" && drift.Expected.Kind == "file" {
		return classifyMissingFile(targetRoot, p, manifests, item, now, forApply)
	}
	if drift.Change == "extra" && drift.Expected.Kind == "missing" && observed.Present != nil && !*observed.Present {
		item.action.Action = ActionResolveNoop
		item.action.Result = ResultPlanned
		item.action.Reason = "target path is already absent"
		return item
	}
	return refused(item, ReasonUnsupportedChange, fmt.Sprintf("automatic reconcile does not support change %q with expected kind %q", drift.Change, drift.Expected.Kind))
}

func classifyMissingFile(targetRoot string, p profile.Profile, manifests map[string]control.Manifest, item candidate, now time.Time, forApply bool) candidate {
	drift := item.drift
	if item.targetObserved.Present == nil {
		return refused(item, ReasonAmbiguousState, "target path presence is unknown; record requires manual review")
	}
	if drift.Expected.Path != drift.Path {
		return refused(item, ReasonAmbiguousState, fmt.Sprintf("drift path %q does not match expected path %q", drift.Path, drift.Expected.Path))
	}
	if strings.TrimSpace(drift.Expected.Digest) == "" || !drift.Expected.HasSizeEvidence() {
		return refused(item, ReasonPublishedEvidence, "missing-file repair requires digest and size evidence in the drift expected state")
	}
	manifest, ok := manifests[drift.Expected.SessionID]
	if !ok || manifest.ID != drift.Expected.ManifestID {
		return refused(item, ReasonPublishedEvidence, "published expected manifest evidence is not available for the drift")
	}
	if !manifestHasExpectedEntry(manifest, drift.Expected) {
		return refused(item, ReasonPublishedEvidence, "published manifest entry does not match the drift expected evidence")
	}
	if *item.targetObserved.Present {
		if reason := restoredTargetMismatch(drift.Expected, item.targetObserved); reason != "" {
			return refused(item, ReasonAmbiguousState, "target path is no longer missing and does not match expected evidence: "+reason)
		}
		item.action.Action = ActionResolveNoop
		item.action.Result = ResultPlanned
		item.action.Reason = "target already matches expected evidence"
		return item
	}
	sourcePath, err := sourcePathForExpected(p, drift.RootID, drift.Expected.Path)
	if err != nil {
		return refused(item, ReasonSourceEvidenceMissing, err.Error())
	}
	source, err := sourceEvidence(sourcePath, drift.Expected)
	if err != nil {
		return refused(item, ReasonSourceEvidenceMissing, err.Error())
	}
	if source.Size != drift.Expected.Size || source.Digest != drift.Expected.Digest {
		return refused(item, ReasonSourceMismatch, "current source file does not match published manifest evidence")
	}
	source.RootID = drift.RootID
	source.Path = filepath.ToSlash(drift.Expected.Path)
	item.sourcePath = sourcePath
	item.action.Action = ActionRestoreFile
	item.action.Result = ResultPlanned
	item.action.SourceEvidence = &source
	if !forApply {
		item.action.Reason = "current source file matches published manifest evidence"
	} else {
		item.action.Reason = fmt.Sprintf("preflighted at %s", now.Format(time.RFC3339Nano))
	}
	return item
}

func applyCandidate(targetRoot string, item candidate, opts ApplyOptions, now time.Time) (Action, Refusal) {
	switch item.action.Action {
	case ActionRestoreFile:
		return applyRestoreFile(targetRoot, item, opts, now)
	case ActionResolveNoop:
		return applyResolveOnly(targetRoot, item, opts, now)
	default:
		return Action{}, Refusal{
			DriftID:    item.drift.ID,
			Path:       item.drift.Path,
			Change:     item.drift.Change,
			Action:     item.action.Action,
			ReasonCode: ReasonUnsupportedChange,
			Message:    "planned action is not apply-capable",
		}
	}
}

func applyRestoreFile(targetRoot string, item candidate, opts ApplyOptions, now time.Time) (Action, Refusal) {
	observed, err := observeTarget(targetRoot, item.drift.Path, false)
	if err != nil {
		return Action{}, applyRefusal(item, ReasonUnsafeTargetParent, fmt.Sprintf("inspect target path before apply: %v", err), observed)
	}
	if observed.Present == nil || *observed.Present {
		return Action{}, applyRefusal(item, ReasonTargetChanged, "target path changed before apply", observed)
	}
	if err := pathguard.EnsurePlainDirectory(targetRoot, filepath.Dir(item.targetPath), 0o755); err != nil {
		return Action{}, applyRefusal(item, ReasonUnsafeTargetParent, fmt.Sprintf("target parent is unsafe: %v", err), observed)
	}
	in, err := os.Open(item.sourcePath)
	if err != nil {
		return Action{}, applyRefusal(item, ReasonSourceEvidenceMissing, fmt.Sprintf("open source file: %v", err), observed)
	}
	defer in.Close()
	sourceInfo, err := in.Stat()
	if err != nil {
		return Action{}, applyRefusal(item, ReasonSourceEvidenceMissing, fmt.Sprintf("inspect source file: %v", err), observed)
	}
	sourceDigest, err := digestOpenFile(in)
	if err != nil {
		return Action{}, applyRefusal(item, ReasonSourceEvidenceMissing, fmt.Sprintf("digest source file: %v", err), observed)
	}
	if sourceInfo.Size() != item.drift.Expected.Size || sourceDigest != item.drift.Expected.Digest {
		return Action{}, applyRefusal(item, ReasonSourceMismatch, "source file changed after planning", observed)
	}
	afterSourcePreflight(item.sourcePath)
	temp, err := os.CreateTemp(filepath.Dir(item.targetPath), ".reconcile-*.tmp")
	if err != nil {
		return Action{}, applyRefusal(item, ReasonMutationFailed, fmt.Sprintf("create staged file: %v", err), observed)
	}
	tempName := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempName)
		}
	}()
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		temp.Close()
		return Action{}, applyRefusal(item, ReasonSourceEvidenceMissing, fmt.Sprintf("rewind source file: %v", err), observed)
	}
	if _, err := io.Copy(temp, in); err != nil {
		temp.Close()
		return Action{}, applyRefusal(item, ReasonMutationFailed, fmt.Sprintf("copy source to staged file: %v", err), observed)
	}
	mode := os.FileMode(item.drift.Expected.Mode).Perm()
	if mode == 0 {
		mode = sourceInfo.Mode().Perm()
	}
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return Action{}, applyRefusal(item, ReasonMutationFailed, fmt.Sprintf("set staged mode: %v", err), observed)
	}
	if item.drift.Expected.ModTime != "" {
		ts, err := time.Parse(time.RFC3339Nano, item.drift.Expected.ModTime)
		if err != nil {
			temp.Close()
			return Action{}, applyRefusal(item, ReasonMutationFailed, fmt.Sprintf("parse expected mod time: %v", err), observed)
		}
		if err := chtimes(tempName, ts, ts); err != nil {
			temp.Close()
			return Action{}, applyRefusal(item, ReasonMutationFailed, fmt.Sprintf("set staged mod time: %v", err), observed)
		}
	}
	if err := temp.Close(); err != nil {
		return Action{}, applyRefusal(item, ReasonMutationFailed, fmt.Sprintf("close staged file: %v", err), observed)
	}
	if reason, err := stagedFileMismatch(tempName, item.drift.Expected); err != nil {
		return Action{}, applyRefusal(item, ReasonMutationFailed, fmt.Sprintf("verify staged file: %v", err), observed)
	} else if reason != "" {
		return Action{}, applyRefusal(item, ReasonSourceMismatch, "staged file does not match expected evidence before publish: "+reason, observed)
	}
	if reason, err := currentSourceMismatch(item.sourcePath, item.drift.Expected); err != nil {
		return Action{}, applyRefusal(item, ReasonSourceMismatch, "current source file changed before publish: "+err.Error(), observed)
	} else if reason != "" {
		return Action{}, applyRefusal(item, ReasonSourceMismatch, "current source file changed before publish: "+reason, observed)
	}
	if err := promoteFileNoReplace(tempName, item.targetPath); err != nil {
		restored, reason, ok := restoredTargetAfterPublishError(targetRoot, item)
		if ok || reason != "" {
			observed = restored
		}
		code := ReasonMutationFailed
		if errors.Is(err, fs.ErrExist) || os.IsExist(err) {
			code = ReasonTargetChanged
		}
		message := fmt.Sprintf("publish restored file without replace: %v", err)
		if ok {
			message += "; target now matches expected evidence, rerun reconcile apply to resolve without restoring"
		} else if reason != "" {
			message += "; restored target check: " + reason
		}
		return Action{}, applyRefusal(item, code, message, observed)
	}
	removeTemp = false
	restored, err := observeTarget(targetRoot, item.drift.Path, true)
	if err != nil {
		return Action{}, applyRefusal(item, ReasonMutationFailed, fmt.Sprintf("verify restored target file: %v", err), observed)
	}
	if reason := restoredTargetMismatch(item.drift.Expected, restored); reason != "" {
		return Action{}, applyRefusal(item, ReasonMutationFailed, "restored target evidence does not match expected record: "+reason, restored)
	}
	return resolveDrift(targetRoot, item, opts, now)
}

func restoredTargetAfterPublishError(targetRoot string, item candidate) (ObservedState, string, bool) {
	restored, err := observeTarget(targetRoot, item.drift.Path, true)
	if err != nil {
		return restored, fmt.Sprintf("verify restored target file: %v", err), false
	}
	if reason := restoredTargetMismatch(item.drift.Expected, restored); reason != "" {
		return restored, "restored target evidence does not match expected record: " + reason, false
	}
	return restored, "", true
}

func stagedFileMismatch(path string, expected control.TargetDriftExpectedState) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return fmt.Sprintf("staged kind %q is not file", kindFromInfo(info)), nil
	}
	observed := ObservedState{
		Present: boolPtr(true),
		Kind:    "file",
		Path:    filepath.ToSlash(path),
		Size:    info.Size(),
		Mode:    uint32(info.Mode().Perm()),
		ModTime: info.ModTime().UTC().Format(time.RFC3339Nano),
	}
	if strings.TrimSpace(expected.Digest) != "" {
		digest, err := digestFile(path, info)
		if err != nil {
			return "", err
		}
		observed.Digest = digest
	}
	return restoredTargetMismatch(expected, observed), nil
}

func currentSourceMismatch(path string, expected control.TargetDriftExpectedState) (string, error) {
	source, err := sourceEvidence(path, expected)
	if err != nil {
		return "", err
	}
	if expected.HasSizeEvidence() && source.Size != expected.Size {
		return fmt.Sprintf("source size %d does not match expected size %d", source.Size, expected.Size), nil
	}
	if strings.TrimSpace(expected.Digest) != "" && source.Digest != expected.Digest {
		return "source digest does not match expected digest", nil
	}
	return "", nil
}

func applyResolveOnly(targetRoot string, item candidate, opts ApplyOptions, now time.Time) (Action, Refusal) {
	includeDigest := item.drift.Change == "missing" && item.drift.Expected.Digest != ""
	observed, err := observeTarget(targetRoot, item.drift.Path, includeDigest)
	if err != nil {
		return Action{}, applyRefusal(item, ReasonUnsafeTargetParent, fmt.Sprintf("inspect target path before resolve: %v", err), observed)
	}
	switch item.drift.Change {
	case "extra":
		if observed.Present == nil || *observed.Present {
			return Action{}, applyRefusal(item, ReasonTargetChanged, "target path changed before resolve", observed)
		}
	case "missing":
		if reason := restoredTargetMismatch(item.drift.Expected, observed); reason != "" {
			return Action{}, applyRefusal(item, ReasonTargetChanged, "target no longer matches expected evidence before resolve: "+reason, observed)
		}
		item.action.ObservedBefore = observed
		item.targetObserved = observed
	default:
		return Action{}, applyRefusal(item, ReasonUnsupportedChange, "planned resolve action does not support this drift change", observed)
	}
	return resolveDrift(targetRoot, item, opts, now)
}

func resolveDrift(targetRoot string, item candidate, opts ApplyOptions, now time.Time) (Action, Refusal) {
	path, err := control.Path(targetRoot, control.ArtifactTargetDrift, item.drift.ID)
	if err != nil {
		return Action{}, applyRefusal(item, ReasonMutationFailed, err.Error(), item.targetObserved)
	}
	current, err := control.ReadFileNoSymlinkUnderRoot[control.TargetDrift](control.ControlDir(targetRoot), path)
	if err != nil {
		return Action{}, applyRefusal(item, ReasonMutationFailed, fmt.Sprintf("read drift artifact before resolve: %v", err), item.targetObserved)
	}
	if current.ID != item.drift.ID || current.ProfileID != item.drift.ProfileID || current.TargetID != item.drift.TargetID || current.RootID != item.drift.RootID || current.Path != item.drift.Path || current.Change != item.drift.Change {
		return Action{}, applyRefusal(item, ReasonTargetChanged, "drift artifact changed before apply", item.targetObserved)
	}
	switch driftstore.ReviewState(current) {
	case "resolved", "expired":
		applied := item.action
		applied.Result = ResultNoop
		applied.ReviewedAt = current.ReviewedAt
		applied.Reviewer = current.ReviewedBy
		applied.Reason = current.ReviewReason
		return applied, Refusal{}
	}
	reviewedAt := now.Format(time.RFC3339Nano)
	current.ReviewState = "resolved"
	current.ReviewedAt = reviewedAt
	current.ReviewedBy = strings.TrimSpace(opts.Reviewer)
	current.ReviewReason = strings.TrimSpace(opts.Reason)
	current.ReviewAction = "resolve"
	if err := control.WriteFile(path, current); err != nil {
		return Action{}, applyRefusal(item, ReasonMutationFailed, fmt.Sprintf("write drift resolution: %v", err), item.targetObserved)
	}
	applied := item.action
	applied.Result = ResultApplied
	applied.ReviewedAt = reviewedAt
	applied.Reviewer = current.ReviewedBy
	applied.Reason = current.ReviewReason
	return applied, Refusal{}
}

func refuseApplySetConflicts(receipt *Receipt, candidates []candidate) bool {
	byTarget := map[string][]candidate{}
	for _, item := range candidates {
		if item.refused || item.action.Result != ResultPlanned {
			continue
		}
		switch item.action.Action {
		case ActionRestoreFile, ActionResolveNoop:
			key := filepath.ToSlash(item.targetPath)
			if key == "" {
				key = filepath.ToSlash(item.drift.Path)
			}
			byTarget[key] = append(byTarget[key], item)
		}
	}
	conflicted := map[string]struct{}{}
	for _, items := range byTarget {
		if len(items) < 2 {
			continue
		}
		for _, item := range items {
			conflicted[item.drift.ID] = struct{}{}
			receipt.Refusals = append(receipt.Refusals, Refusal{
				DriftID:        item.drift.ID,
				Path:           item.drift.Path,
				Change:         item.drift.Change,
				SessionID:      item.drift.SessionID,
				Action:         item.action.Action,
				ReasonCode:     ReasonAmbiguousState,
				ConflictClass:  conflictClassForReason(ReasonAmbiguousState),
				RetryAdvice:    retryAdviceForReason(ReasonAmbiguousState),
				Message:        "multiple selected drift records target the same path; reconcile apply refuses the whole plan before mutation",
				ObservedBefore: item.action.ObservedBefore,
			})
		}
	}
	if len(conflicted) == 0 {
		return false
	}
	kept := receipt.Actions[:0]
	for _, action := range receipt.Actions {
		if _, ok := conflicted[action.DriftID]; ok {
			continue
		}
		kept = append(kept, action)
	}
	receipt.Actions = kept
	sortReceipt(receipt)
	summarize(receipt)
	return true
}

func refused(item candidate, code string, message string) candidate {
	item.refused = true
	item.refusal.Action = ActionRefuseUnsupported
	item.refusal.ReasonCode = code
	item.refusal.ConflictClass = conflictClassForReason(code)
	item.refusal.RetryAdvice = retryAdviceForReason(code)
	item.refusal.Message = message
	return item
}

func applyRefusal(item candidate, code string, message string, observed ObservedState) Refusal {
	return Refusal{
		DriftID:        item.drift.ID,
		Path:           item.drift.Path,
		Change:         item.drift.Change,
		SessionID:      item.drift.SessionID,
		Action:         item.action.Action,
		ReasonCode:     code,
		ConflictClass:  conflictClassForReason(code),
		RetryAdvice:    retryAdviceForReason(code),
		Message:        message,
		ObservedBefore: observed,
	}
}

func conflictClassForReason(reason string) string {
	switch reason {
	case ReasonMissingApplyIntent:
		return ConflictClassOperatorIntent
	case ReasonProfileScopeMismatch, ReasonRootScopeMismatch:
		return ConflictClassScopeMismatch
	case ReasonControlPlanePath, ReasonUnsafeTargetPath, ReasonUnsafeTargetParent:
		return ConflictClassUnsafePath
	case ReasonUnsupportedChange, ReasonUnsupportedKind:
		return ConflictClassUnsupportedDrift
	case ReasonAlreadyResolved, ReasonAmbiguousState, ReasonTargetChanged:
		return ConflictClassTargetState
	case ReasonArtifactProblems:
		return ConflictClassArtifactIntegrity
	case ReasonPublishedEvidence:
		return ConflictClassPublishedEvidence
	case ReasonSourceEvidenceMissing, ReasonSourceMismatch:
		return ConflictClassSourceEvidence
	case ReasonMutationFailed:
		return ConflictClassMutationFailure
	default:
		return ConflictClassOperatorIntent
	}
}

func retryAdviceForReason(reason string) string {
	switch reason {
	case ReasonMissingApplyIntent:
		return RetryAdviceFixCommand
	case ReasonProfileScopeMismatch, ReasonRootScopeMismatch:
		return RetryAdviceSelectMatchingScope
	case ReasonControlPlanePath, ReasonUnsafeTargetPath, ReasonUnsafeTargetParent,
		ReasonUnsupportedChange, ReasonUnsupportedKind, ReasonAlreadyResolved:
		return RetryAdviceManualReview
	case ReasonAmbiguousState, ReasonTargetChanged:
		return RetryAdviceReviewTargetState
	case ReasonArtifactProblems:
		return RetryAdviceRepairArtifacts
	case ReasonPublishedEvidence:
		return RetryAdviceRestorePublishedEvidence
	case ReasonSourceEvidenceMissing, ReasonSourceMismatch:
		return RetryAdviceRestoreSourceEvidence
	case ReasonMutationFailed:
		return RetryAdviceInspectMutationThenReplan
	default:
		return RetryAdviceManualReview
	}
}

func refusePlannedForMissingIntent(receipt *Receipt) {
	var kept []Action
	for _, action := range receipt.Actions {
		if action.Result != ResultPlanned {
			kept = append(kept, action)
			continue
		}
		receipt.Refusals = append(receipt.Refusals, Refusal{
			DriftID:       action.DriftID,
			Path:          action.Path,
			Change:        action.Change,
			SessionID:     action.SessionID,
			Action:        action.Action,
			ReasonCode:    ReasonMissingApplyIntent,
			ConflictClass: conflictClassForReason(ReasonMissingApplyIntent),
			RetryAdvice:   retryAdviceForReason(ReasonMissingApplyIntent),
			Message:       "apply intent is required before target mutation",
		})
	}
	receipt.Actions = kept
	sortReceipt(receipt)
	summarize(receipt)
}

func selectDrifts(drifts []control.TargetDrift, ids []string, sessionID string) ([]control.TargetDrift, error) {
	want := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := control.ValidateArtifactID(id); err != nil {
			return nil, fmt.Errorf("drift id is unsafe: %w", err)
		}
		want[id] = struct{}{}
	}
	var out []control.TargetDrift
	for _, drift := range drifts {
		if strings.TrimSpace(sessionID) != "" && drift.SessionID != sessionID && drift.Expected.SessionID != sessionID {
			continue
		}
		if len(want) > 0 {
			if _, ok := want[drift.ID]; !ok {
				continue
			}
			delete(want, drift.ID)
		}
		out = append(out, drift)
	}
	if len(want) > 0 {
		missing := make([]string, 0, len(want))
		for id := range want {
			missing = append(missing, id)
		}
		sort.Strings(missing)
		return nil, fmt.Errorf("persisted target drift %q not found", strings.Join(missing, ","))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].ID < out[j].ID
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func filterArtifactProblems(problems []verify.ArtifactProblem, sessionID string) []verify.ArtifactProblem {
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

func filterPlanningArtifactProblems(problems []verify.ArtifactProblem, sessionID string, ids []string, drifts []control.TargetDrift) []verify.ArtifactProblem {
	want := selectedDriftIDs(ids)
	if len(want) == 0 {
		return filterArtifactProblems(problems, sessionID)
	}
	sessions := selectedDriftSessions(want, sessionID, drifts)
	var out []verify.ArtifactProblem
	for _, problem := range problems {
		if artifactProblemMatchesSelectedDrift(problem, want) || artifactProblemMatchesSelectedSession(problem, sessions) {
			out = append(out, problem)
		}
	}
	return out
}

func selectedDriftIDs(ids []string) map[string]struct{} {
	want := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			want[id] = struct{}{}
		}
	}
	return want
}

func selectedDriftSessions(want map[string]struct{}, sessionID string, drifts []control.TargetDrift) map[string]struct{} {
	sessions := map[string]struct{}{}
	for _, drift := range drifts {
		if _, ok := want[drift.ID]; !ok {
			continue
		}
		if strings.TrimSpace(sessionID) != "" && drift.SessionID != sessionID && drift.Expected.SessionID != sessionID {
			continue
		}
		if drift.SessionID != "" {
			sessions[drift.SessionID] = struct{}{}
		}
		if drift.Expected.SessionID != "" {
			sessions[drift.Expected.SessionID] = struct{}{}
		}
	}
	return sessions
}

func artifactProblemMatchesSelectedDrift(problem verify.ArtifactProblem, want map[string]struct{}) bool {
	if !strings.Contains(filepath.ToSlash(problem.Path), "/"+control.DirName+"/drift/") {
		return false
	}
	id := strings.TrimSuffix(filepath.Base(problem.Path), filepath.Ext(problem.Path))
	_, ok := want[id]
	return ok
}

func artifactProblemMatchesSelectedSession(problem verify.ArtifactProblem, sessions map[string]struct{}) bool {
	if problem.SessionID == "" {
		return false
	}
	if _, ok := sessions[problem.SessionID]; !ok {
		return false
	}
	path := filepath.ToSlash(problem.Path)
	return strings.Contains(path, "/"+control.DirName+"/sessions/"+problem.SessionID+"/")
}

func hasTargetDriftID(drifts []control.TargetDrift, id string) bool {
	for _, drift := range drifts {
		if drift.ID == id {
			return true
		}
	}
	return false
}

func hasArtifactProblemPath(problems []verify.ArtifactProblem, path string) bool {
	path = filepath.ToSlash(path)
	for _, existing := range problems {
		if existing.Path == path {
			return true
		}
	}
	return false
}

func sourcePathForExpected(p profile.Profile, rootID string, rel string) (string, error) {
	if err := safeDataPath(rel); err != nil {
		return "", fmt.Errorf("expected source path is unsafe: %w", err)
	}
	for _, root := range p.Roots {
		if root.ID != rootID {
			continue
		}
		if strings.TrimSpace(root.Path) == "" {
			return "", fmt.Errorf("profile root %q has no source path", rootID)
		}
		return filepath.Join(root.Path, filepath.FromSlash(rel)), nil
	}
	return "", fmt.Errorf("profile root %q was not found", rootID)
}

func sourceEvidence(path string, expected control.TargetDriftExpectedState) (SourceEvidence, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return SourceEvidence{}, fmt.Errorf("source file %q is missing", filepath.ToSlash(path))
		}
		return SourceEvidence{}, fmt.Errorf("inspect source file %q: %w", filepath.ToSlash(path), err)
	}
	if !info.Mode().IsRegular() {
		return SourceEvidence{}, fmt.Errorf("source path %q is not a regular file", filepath.ToSlash(path))
	}
	if expected.HasSizeEvidence() && info.Size() != expected.Size {
		return SourceEvidence{}, fmt.Errorf("source size %d does not match expected size %d", info.Size(), expected.Size)
	}
	digest, err := digestFile(path, info)
	if err != nil {
		return SourceEvidence{}, fmt.Errorf("digest source file %q: %w", filepath.ToSlash(path), err)
	}
	return SourceEvidence{
		Size:   info.Size(),
		Digest: digest,
		Mode:   uint32(info.Mode().Perm()),
	}, nil
}

func digestFile(path string, observed fs.FileInfo) (string, error) {
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
	digest, err := digestOpenFile(file)
	if err != nil {
		return "", err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !after.Mode().IsRegular() || !os.SameFile(observed, after) || after.Size() != observed.Size() || after.Mode().Perm() != observed.Mode().Perm() || !after.ModTime().Equal(observed.ModTime()) {
		return "", errors.New("regular file changed during digest")
	}
	return digest, nil
}

func digestOpenFile(file *os.File) (string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeStartedApplyReceipt(targetRoot string, p profile.Profile, ids []string, sessionID string, now time.Time, receipt *Receipt) (string, error) {
	id := reconcileReceiptID(p, ids, sessionID, now)
	path, err := control.Path(targetRoot, control.ArtifactReconcileReceipt, id)
	if err != nil {
		return "", err
	}
	receipt.ID = id
	receipt.Status = ReceiptStatusStarted
	receipt.StartedAt = now.Format(time.RFC3339Nano)
	receipt.EndedAt = ""
	receipt.ReceiptPath = filepath.ToSlash(path)
	if err := control.WriteNewFile(path, *receipt); err != nil {
		return "", fmt.Errorf("write started reconcile receipt: %w", err)
	}
	return path, nil
}

func writeFinalApplyReceipt(path string, now time.Time, receipt *Receipt) error {
	receipt.EndedAt = now.Format(time.RFC3339Nano)
	switch {
	case receipt.Summary.Applied > 0 && (receipt.Summary.Refused > 0 || receipt.Summary.ArtifactProblems > 0):
		receipt.Status = ReceiptStatusPartial
	case receipt.Summary.Refused > 0 || receipt.Summary.ArtifactProblems > 0:
		receipt.Status = ReceiptStatusFailed
	default:
		receipt.Status = ReceiptStatusApplied
	}
	if err := control.WriteFile(path, *receipt); err != nil {
		return fmt.Errorf("write final reconcile receipt: %w", err)
	}
	return nil
}

func reconcileReceiptID(p profile.Profile, ids []string, sessionID string, now time.Time) string {
	selected := make([]string, 0, len(ids))
	for _, id := range ids {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			selected = append(selected, trimmed)
		}
	}
	sort.Strings(selected)
	payload := strings.Join([]string{
		p.ProfileID,
		p.Target.TargetID,
		strings.TrimSpace(sessionID),
		now.Format(time.RFC3339Nano),
		strings.Join(selected, "\x00"),
	}, "\x1f")
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("reconcile-%s-%s", now.Format("20060102T150405.000000000Z"), hex.EncodeToString(sum[:])[:16])
}

func observeTarget(targetRoot string, rel string, includeDigest bool) (ObservedState, error) {
	if err := safeDataPath(rel); err != nil {
		return ObservedState{}, err
	}
	path, err := safeTargetPath(targetRoot, rel)
	if err != nil {
		return ObservedState{}, err
	}
	if err := pathguard.EnsureDirectory(targetRoot, filepath.Dir(path)); err != nil {
		return ObservedState{Present: boolPtr(true), Kind: "other", Path: rel}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ObservedState{Present: boolPtr(false), Kind: "missing", Path: rel}, nil
		}
		return ObservedState{}, err
	}
	state := ObservedState{
		Present: boolPtr(true),
		Kind:    kindFromInfo(info),
		Path:    rel,
		Mode:    uint32(info.Mode().Perm()),
		ModTime: info.ModTime().UTC().Format(time.RFC3339Nano),
	}
	if state.Kind == "file" {
		state.Size = info.Size()
		if includeDigest {
			digest, err := digestFile(path, info)
			if err != nil {
				return state, err
			}
			state.Digest = digest
		}
	}
	return state, nil
}

func restoredTargetMismatch(expected control.TargetDriftExpectedState, observed ObservedState) string {
	if observed.Present == nil || !*observed.Present {
		return "target file is missing"
	}
	if observed.Kind != "file" {
		return fmt.Sprintf("target kind %q is not file", observed.Kind)
	}
	if expected.HasSizeEvidence() && observed.Size != expected.Size {
		return fmt.Sprintf("size %d does not match expected size %d", observed.Size, expected.Size)
	}
	if strings.TrimSpace(expected.Digest) != "" && observed.Digest != expected.Digest {
		return "digest does not match expected digest"
	}
	if expected.HasModeEvidence() && observed.Mode != uint32(os.FileMode(expected.Mode).Perm()) {
		return fmt.Sprintf("mode %04o does not match expected mode %04o", observed.Mode, os.FileMode(expected.Mode).Perm())
	}
	if expected.ModTime != "" {
		ts, err := time.Parse(time.RFC3339Nano, expected.ModTime)
		if err != nil {
			return fmt.Sprintf("expected mod time is invalid: %v", err)
		}
		if observed.ModTime != ts.UTC().Format(time.RFC3339Nano) {
			return fmt.Sprintf("mod time %q does not match expected mod time %q", observed.ModTime, ts.UTC().Format(time.RFC3339Nano))
		}
	}
	return ""
}

func manifestHasExpectedEntry(manifest control.Manifest, expected control.TargetDriftExpectedState) bool {
	for _, entry := range manifest.Entries {
		if targetPath(entry) != expected.Path {
			continue
		}
		if filepath.ToSlash(entry.Path) != expected.Path {
			return false
		}
		if entry.Kind != expected.Kind || entry.Digest != expected.Digest || entry.ModTime != expected.ModTime {
			return false
		}
		if entry.HasSizeEvidence() != expected.HasSizeEvidence() || entry.Size != expected.Size {
			return false
		}
		if entry.HasModeEvidence() != expected.HasModeEvidence() || entry.Mode != expected.Mode {
			return false
		}
		return true
	}
	return false
}

func targetPath(entry control.ManifestEntry) string {
	if strings.TrimSpace(entry.TargetPath) != "" {
		return filepath.ToSlash(entry.TargetPath)
	}
	return filepath.ToSlash(entry.Path)
}

func profileHasRoot(p profile.Profile, rootID string) bool {
	for _, root := range p.Roots {
		if root.ID == rootID {
			return true
		}
	}
	return false
}

func safeDataPath(rel string) error {
	if err := pathguard.ValidateSlashRelativePath(rel, 0); err != nil {
		return err
	}
	if pathguard.IsReservedControlPath(rel) {
		return fmt.Errorf("reserved control-plane target path %q", rel)
	}
	return nil
}

func safeTargetPath(root, rel string) (string, error) {
	if err := safeDataPath(rel); err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

func refusalCodeForPathError(err error) string {
	if strings.Contains(err.Error(), "reserved control-plane") {
		return ReasonControlPlanePath
	}
	return ReasonUnsafeTargetPath
}

func expectedState(state control.TargetDriftExpectedState) ExpectedState {
	return ExpectedState{
		SessionID:  state.SessionID,
		ManifestID: state.ManifestID,
		Kind:       state.Kind,
		Path:       state.Path,
		Size:       state.Size,
		Digest:     state.Digest,
		Mode:       state.Mode,
		ModTime:    state.ModTime,
	}
}

func replaceAction(actions []Action, replacement Action) []Action {
	for i := range actions {
		if actions[i].DriftID == replacement.DriftID {
			actions[i] = replacement
			return actions
		}
	}
	return append(actions, replacement)
}

func sortReceipt(receipt *Receipt) {
	sort.Slice(receipt.Actions, func(i, j int) bool {
		if receipt.Actions[i].Path == receipt.Actions[j].Path {
			return receipt.Actions[i].DriftID < receipt.Actions[j].DriftID
		}
		return receipt.Actions[i].Path < receipt.Actions[j].Path
	})
	sort.Slice(receipt.Refusals, func(i, j int) bool {
		if receipt.Refusals[i].Path == receipt.Refusals[j].Path {
			return receipt.Refusals[i].DriftID < receipt.Refusals[j].DriftID
		}
		return receipt.Refusals[i].Path < receipt.Refusals[j].Path
	})
	sort.Slice(receipt.ArtifactProblems, func(i, j int) bool {
		if receipt.ArtifactProblems[i].Path == receipt.ArtifactProblems[j].Path {
			return receipt.ArtifactProblems[i].SessionID < receipt.ArtifactProblems[j].SessionID
		}
		return receipt.ArtifactProblems[i].Path < receipt.ArtifactProblems[j].Path
	})
}

func summarize(receipt *Receipt) {
	receipt.Summary = Summary{
		Records:          len(receipt.Actions) + len(receipt.Refusals),
		Refused:          len(receipt.Refusals),
		ArtifactProblems: len(receipt.ArtifactProblems),
	}
	for _, action := range receipt.Actions {
		switch action.Result {
		case ResultApplied:
			receipt.Summary.Applied++
		case ResultNoop:
			receipt.Summary.Noop++
		default:
			receipt.Summary.Planned++
		}
	}
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

func boolPtr(value bool) *bool {
	return &value
}

func (r Receipt) Validate() error {
	var errs []error
	if r.Schema != SchemaApplyReceipt {
		errs = append(errs, fmt.Errorf("schema must be %q for durable reconcile receipts", SchemaApplyReceipt))
	}
	if strings.TrimSpace(r.ID) == "" {
		errs = append(errs, errors.New("id is required"))
	} else if err := control.ValidateArtifactID(r.ID); err != nil {
		errs = append(errs, fmt.Errorf("id is unsafe: %w", err))
	}
	if !receiptStatusValid(r.Status) {
		errs = append(errs, errors.New("status must be one of started, applied, partial, failed"))
	}
	if strings.TrimSpace(r.TargetRoot) == "" {
		errs = append(errs, errors.New("target_root is required"))
	}
	if strings.TrimSpace(r.ProfileID) == "" {
		errs = append(errs, errors.New("profile_id is required"))
	}
	if strings.TrimSpace(r.TargetID) == "" {
		errs = append(errs, errors.New("target_id is required"))
	}
	if strings.TrimSpace(r.SessionID) != "" {
		if err := control.ValidateArtifactID(r.SessionID); err != nil {
			errs = append(errs, fmt.Errorf("session_id is unsafe: %w", err))
		}
	}
	generatedAt, generatedOK := parseReconcileTime("generated_at", r.GeneratedAt, true, &errs)
	startedAt, startedOK := parseReconcileTime("started_at", r.StartedAt, true, &errs)
	endedAt, endedOK := parseReconcileTime("ended_at", r.EndedAt, r.Status != ReceiptStatusStarted, &errs)
	if generatedOK && startedOK && startedAt.Before(generatedAt) {
		errs = append(errs, errors.New("started_at must be greater than or equal to generated_at"))
	}
	if startedOK && endedOK && endedAt.Before(startedAt) {
		errs = append(errs, errors.New("ended_at must be greater than or equal to started_at"))
	}
	if r.Status == ReceiptStatusStarted && strings.TrimSpace(r.EndedAt) != "" {
		errs = append(errs, errors.New("started receipts must not include ended_at"))
	}
	if strings.TrimSpace(r.ReceiptPath) == "" {
		errs = append(errs, errors.New("receipt_path is required"))
	}
	if !r.ApplyIntent {
		errs = append(errs, errors.New("apply_intent must be true for durable reconcile receipts"))
	}
	if len(r.Actions) == 0 && len(r.Refusals) == 0 && len(r.ArtifactProblems) == 0 {
		errs = append(errs, errors.New("receipt must contain actions, refusals, or artifact problems"))
	}
	for i, action := range r.Actions {
		validateReceiptAction(i, action, &errs)
	}
	for i, refusal := range r.Refusals {
		validateReceiptRefusal(i, refusal, &errs)
	}
	for i, problem := range r.ArtifactProblems {
		validateReceiptArtifactProblem(i, problem, &errs)
	}
	if r.Summary != summarizeCopy(r) {
		errs = append(errs, errors.New("summary does not match receipt contents"))
	}
	validateReceiptStatusOutcome(r, &errs)
	return errors.Join(errs...)
}

func receiptStatusValid(status string) bool {
	switch status {
	case ReceiptStatusStarted, ReceiptStatusApplied, ReceiptStatusPartial, ReceiptStatusFailed:
		return true
	default:
		return false
	}
}

func parseReconcileTime(name string, value string, required bool, errs *[]error) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		if required {
			*errs = append(*errs, fmt.Errorf("%s is required", name))
		}
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be RFC3339 timestamp: %w", name, err))
		return time.Time{}, false
	}
	return parsed, true
}

func validateReceiptAction(index int, action Action, errs *[]error) {
	prefix := fmt.Sprintf("actions[%d]", index)
	requireReconcileArtifactID(prefix+".drift_id", action.DriftID, errs)
	requireReconcileDataPath(prefix+".path", action.Path, errs)
	requireReconcileValue(prefix+".change", action.Change, errs)
	requireReconcileValue(prefix+".action", action.Action, errs)
	if action.Action != "" && !receiptActionValid(action.Action) {
		*errs = append(*errs, fmt.Errorf("%s.action must be one of resolve_noop, restore_file_from_source, already_resolved_noop", prefix))
	}
	requireReconcileValue(prefix+".result", action.Result, errs)
	if action.Result != "" && !receiptActionResultValid(action.Result) {
		*errs = append(*errs, fmt.Errorf("%s.result must be one of planned, applied, noop", prefix))
	}
	if action.SessionID != "" {
		requireReconcileArtifactID(prefix+".session_id", action.SessionID, errs)
	}
	validateExpectedState(prefix+".expected", action.Expected, errs)
	validateObservedState(prefix+".observed_before", action.ObservedBefore, errs)
	if action.SourceEvidence != nil {
		validateSourceEvidence(prefix+".source_evidence", *action.SourceEvidence, errs)
	}
	if action.ReviewedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, action.ReviewedAt); err != nil {
			*errs = append(*errs, fmt.Errorf("%s.reviewed_at must be RFC3339 timestamp: %w", prefix, err))
		}
	}
}

func validateReceiptRefusal(index int, refusal Refusal, errs *[]error) {
	prefix := fmt.Sprintf("refusals[%d]", index)
	if refusal.DriftID != "" {
		requireReconcileArtifactID(prefix+".drift_id", refusal.DriftID, errs)
	}
	if refusal.Path != "" {
		requireReconcileDataPath(prefix+".path", refusal.Path, errs)
	}
	if refusal.SessionID != "" {
		requireReconcileArtifactID(prefix+".session_id", refusal.SessionID, errs)
	}
	if refusal.Action != "" && refusal.Action != ActionRefuseUnsupported && !receiptActionValid(refusal.Action) {
		*errs = append(*errs, fmt.Errorf("%s.action must be a known reconcile action", prefix))
	}
	requireReconcileValue(prefix+".reason_code", refusal.ReasonCode, errs)
	if refusal.ReasonCode != "" {
		requireReconcileArtifactID(prefix+".reason_code", refusal.ReasonCode, errs)
	}
	if refusal.ConflictClass != "" && !conflictClassValid(refusal.ConflictClass) {
		*errs = append(*errs, fmt.Errorf("%s.conflict_class must be a known reconcile conflict class", prefix))
	}
	if refusal.RetryAdvice != "" && !retryAdviceValid(refusal.RetryAdvice) {
		*errs = append(*errs, fmt.Errorf("%s.retry_advice must be a known reconcile retry advice", prefix))
	}
	requireReconcileValue(prefix+".message", refusal.Message, errs)
	validateObservedState(prefix+".observed_before", refusal.ObservedBefore, errs)
}

func validateReceiptArtifactProblem(index int, problem ArtifactProblem, errs *[]error) {
	prefix := fmt.Sprintf("artifact_problems[%d]", index)
	if problem.SessionID != "" {
		requireReconcileArtifactID(prefix+".session_id", problem.SessionID, errs)
	}
	requireReconcileValue(prefix+".path", problem.Path, errs)
	requireReconcileValue(prefix+".error", problem.Error, errs)
}

func validateExpectedState(prefix string, state ExpectedState, errs *[]error) {
	if state.SessionID != "" {
		requireReconcileArtifactID(prefix+".session_id", state.SessionID, errs)
	}
	if state.ManifestID != "" {
		requireReconcileValue(prefix+".manifest_id", state.ManifestID, errs)
	}
	if state.Kind != "" && !receiptKindValid(state.Kind) {
		*errs = append(*errs, fmt.Errorf("%s.kind must be one of file, dir, symlink, special, missing, other", prefix))
	}
	if state.Path != "" {
		requireReconcileDataPath(prefix+".path", state.Path, errs)
	}
	if state.Size < 0 {
		*errs = append(*errs, fmt.Errorf("%s.size cannot be negative", prefix))
	}
	if state.Digest != "" && !receiptDigestValid(state.Digest) {
		*errs = append(*errs, fmt.Errorf("%s.digest must be sha256 hex", prefix))
	}
	if state.ModTime != "" {
		if _, err := time.Parse(time.RFC3339Nano, state.ModTime); err != nil {
			*errs = append(*errs, fmt.Errorf("%s.mod_time must be RFC3339 timestamp: %w", prefix, err))
		}
	}
}

func validateObservedState(prefix string, state ObservedState, errs *[]error) {
	if state.Present == nil && state.Kind == "" && state.Path == "" && state.Size == 0 && state.Digest == "" && state.Mode == 0 && state.ModTime == "" {
		return
	}
	if state.Present == nil {
		*errs = append(*errs, fmt.Errorf("%s.present is required", prefix))
	}
	if state.Kind == "" {
		*errs = append(*errs, fmt.Errorf("%s.kind is required", prefix))
	} else if !receiptKindValid(state.Kind) {
		*errs = append(*errs, fmt.Errorf("%s.kind must be one of file, dir, symlink, special, missing, other", prefix))
	}
	if state.Path != "" {
		requireReconcileDataPath(prefix+".path", state.Path, errs)
	}
	if state.Size < 0 {
		*errs = append(*errs, fmt.Errorf("%s.size cannot be negative", prefix))
	}
	if state.Digest != "" && !receiptDigestValid(state.Digest) {
		*errs = append(*errs, fmt.Errorf("%s.digest must be sha256 hex", prefix))
	}
	if state.ModTime != "" {
		if _, err := time.Parse(time.RFC3339Nano, state.ModTime); err != nil {
			*errs = append(*errs, fmt.Errorf("%s.mod_time must be RFC3339 timestamp: %w", prefix, err))
		}
	}
}

func validateSourceEvidence(prefix string, source SourceEvidence, errs *[]error) {
	requireReconcileValue(prefix+".root_id", source.RootID, errs)
	requireReconcileDataPath(prefix+".path", source.Path, errs)
	if source.Size < 0 {
		*errs = append(*errs, fmt.Errorf("%s.size cannot be negative", prefix))
	}
	requireReconcileValue(prefix+".digest", source.Digest, errs)
	if source.Digest != "" && !receiptDigestValid(source.Digest) {
		*errs = append(*errs, fmt.Errorf("%s.digest must be sha256 hex", prefix))
	}
}

func validateReceiptStatusOutcome(receipt Receipt, errs *[]error) {
	switch receipt.Status {
	case ReceiptStatusStarted:
		return
	case ReceiptStatusApplied:
		if receipt.Summary.Refused > 0 || receipt.Summary.ArtifactProblems > 0 {
			*errs = append(*errs, errors.New("status applied cannot include refusals or artifact problems"))
		}
		if len(receipt.Actions) == 0 {
			*errs = append(*errs, errors.New("status applied requires at least one action"))
		}
	case ReceiptStatusPartial:
		if receipt.Summary.Applied == 0 || (receipt.Summary.Refused+receipt.Summary.ArtifactProblems) == 0 {
			*errs = append(*errs, errors.New("status partial requires applied action plus refusal or artifact problem"))
		}
	case ReceiptStatusFailed:
		if receipt.Summary.Applied > 0 {
			*errs = append(*errs, errors.New("status failed cannot include applied actions"))
		}
		if receipt.Summary.Refused == 0 && receipt.Summary.ArtifactProblems == 0 {
			*errs = append(*errs, errors.New("status failed requires refusal or artifact problem"))
		}
	}
}

func receiptActionValid(action string) bool {
	switch action {
	case ActionResolveNoop, ActionRestoreFile, ActionAlreadyResolved:
		return true
	default:
		return false
	}
}

func receiptActionResultValid(result string) bool {
	switch result {
	case ResultPlanned, ResultApplied, ResultNoop:
		return true
	default:
		return false
	}
}

func receiptKindValid(kind string) bool {
	switch kind {
	case "file", "dir", "symlink", "special", "missing", "other":
		return true
	default:
		return false
	}
}

func conflictClassValid(value string) bool {
	switch value {
	case ConflictClassOperatorIntent,
		ConflictClassScopeMismatch,
		ConflictClassUnsafePath,
		ConflictClassUnsupportedDrift,
		ConflictClassTargetState,
		ConflictClassArtifactIntegrity,
		ConflictClassPublishedEvidence,
		ConflictClassSourceEvidence,
		ConflictClassMutationFailure:
		return true
	default:
		return false
	}
}

func retryAdviceValid(value string) bool {
	switch value {
	case RetryAdviceFixCommand,
		RetryAdviceSelectMatchingScope,
		RetryAdviceManualReview,
		RetryAdviceReviewTargetState,
		RetryAdviceRepairArtifacts,
		RetryAdviceRestorePublishedEvidence,
		RetryAdviceRestoreSourceEvidence,
		RetryAdviceInspectMutationThenReplan:
		return true
	default:
		return false
	}
}

func receiptDigestValid(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	for _, r := range strings.TrimPrefix(value, "sha256:") {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

func requireReconcileValue(name string, value string, errs *[]error) {
	if strings.TrimSpace(value) == "" {
		*errs = append(*errs, fmt.Errorf("%s is required", name))
	}
}

func requireReconcileArtifactID(name string, id string, errs *[]error) {
	requireReconcileValue(name, id, errs)
	if strings.TrimSpace(id) == "" {
		return
	}
	if err := control.ValidateArtifactID(id); err != nil {
		*errs = append(*errs, fmt.Errorf("%s is unsafe: %w", name, err))
	}
}

func requireReconcileDataPath(name string, rel string, errs *[]error) {
	requireReconcileValue(name, rel, errs)
	if strings.TrimSpace(rel) == "" {
		return
	}
	if err := safeDataPath(rel); err != nil {
		*errs = append(*errs, fmt.Errorf("%s is unsafe: %w", name, err))
	}
}

func summarizeCopy(receipt Receipt) Summary {
	out := Summary{
		Records:          len(receipt.Actions) + len(receipt.Refusals),
		Refused:          len(receipt.Refusals),
		ArtifactProblems: len(receipt.ArtifactProblems),
	}
	for _, action := range receipt.Actions {
		switch action.Result {
		case ResultApplied:
			out.Applied++
		case ResultNoop:
			out.Noop++
		default:
			out.Planned++
		}
	}
	return out
}

func CanonicalJSON(receipt Receipt) ([]byte, error) {
	return json.MarshalIndent(receipt, "", "  ")
}
