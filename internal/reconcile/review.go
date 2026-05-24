package reconcile

import (
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/verify"
)

const (
	SchemaReview = "supermover.reconcile_review.v1"

	BoundaryStatusWiredReadOnly  = "wired_read_only"
	BoundaryStatusRecordRequired = "record_required"
	BoundaryStatusPlanned        = "planned"
	BoundaryStatusBlocked        = "blocked"

	BoundaryPersistentDrift = "persisted_drift_reconcile"
	BoundaryLiveOnlyDrift   = "live_only_repair_inputs"
	BoundaryBackgroundScan  = "background_scans"
	BoundaryManifestRewrite = "manifest_rewrite"
	BoundaryDaemonSync      = "daemon_ongoing_sync_integration"
	BoundaryDriftToPrune    = "drift_to_prune_handoff"
	BoundaryAutomaticRetry  = "automatic_retry_policy"
)

// ReviewOptions selects the profile-scoped repair/reconcile evidence to review.
type ReviewOptions struct {
	Profile   profile.Profile
	SessionID string
	Now       time.Time
}

// Review is a non-mutating broad repair boundary inventory.
type Review struct {
	Schema           string            `json:"schema"`
	TargetRoot       string            `json:"target_root"`
	ProfileID        string            `json:"profile_id"`
	TargetID         string            `json:"target_id"`
	SessionID        string            `json:"session_id,omitempty"`
	GeneratedAt      string            `json:"generated_at"`
	Summary          ReviewSummary     `json:"summary"`
	PersistedPlan    Receipt           `json:"persisted_plan"`
	LiveRepairInput  LiveRepairInput   `json:"live_repair_input"`
	Boundaries       []RepairBoundary  `json:"boundaries"`
	ArtifactProblems []ArtifactProblem `json:"artifact_problems,omitempty"`
}

type ReviewSummary struct {
	PersistedRecords          int  `json:"persisted_records"`
	PersistedPlanned          int  `json:"persisted_planned"`
	PersistedNoop             int  `json:"persisted_noop"`
	PersistedRefused          int  `json:"persisted_refused"`
	PersistedArtifactProblems int  `json:"persisted_artifact_problems"`
	LiveManifestCount         int  `json:"live_manifest_count"`
	LiveTargetDrifts          int  `json:"live_target_drifts"`
	LiveArtifactProblems      int  `json:"live_artifact_problems"`
	Boundaries                int  `json:"boundaries"`
	ApplyCapableBoundaries    int  `json:"apply_capable_boundaries"`
	RecordRequiredBoundaries  int  `json:"record_required_boundaries"`
	PlannedBoundaries         int  `json:"planned_boundaries"`
	ReviewRequired            bool `json:"review_required"`
}

type LiveRepairInput struct {
	Source           string                 `json:"source"`
	Durable          bool                   `json:"durable"`
	SessionID        string                 `json:"session_id,omitempty"`
	Manifest         verify.ManifestSummary `json:"manifest"`
	Summary          LiveRepairSummary      `json:"summary"`
	Action           string                 `json:"action"`
	Reason           string                 `json:"reason"`
	TargetDrifts     []control.TargetDrift  `json:"target_drifts,omitempty"`
	ArtifactProblems []ArtifactProblem      `json:"artifact_problems,omitempty"`
}

type LiveRepairSummary struct {
	ManifestCount    int `json:"manifest_count"`
	ManifestEntries  int `json:"manifest_entries"`
	TargetDrifts     int `json:"target_drifts"`
	ArtifactProblems int `json:"artifact_problems"`
}

type RepairBoundary struct {
	Name         string          `json:"name"`
	Status       string          `json:"status"`
	Source       string          `json:"source"`
	Action       string          `json:"action"`
	ApplyCapable bool            `json:"apply_capable"`
	Summary      BoundarySummary `json:"summary"`
	Reason       string          `json:"reason"`
}

type BoundarySummary struct {
	Records          int `json:"records,omitempty"`
	Planned          int `json:"planned,omitempty"`
	Noop             int `json:"noop,omitempty"`
	Refused          int `json:"refused,omitempty"`
	TargetDrifts     int `json:"target_drifts,omitempty"`
	ArtifactProblems int `json:"artifact_problems,omitempty"`
}

func ReviewBoundaries(opts ReviewOptions) (Review, error) {
	targetRoot, now, err := normalizeOptions(opts.Profile, opts.Now)
	if err != nil {
		return Review{}, err
	}
	sessionID := strings.TrimSpace(opts.SessionID)
	persisted, err := Plan(Options{
		Profile:   opts.Profile,
		SessionID: sessionID,
		Now:       now,
	})
	if err != nil {
		return Review{}, err
	}
	liveReport, liveErr := verify.DetectTargetDrift(verify.DriftOptions{
		TargetRoot: targetRoot,
		SessionID:  sessionID,
		ProfileID:  opts.Profile.ProfileID,
		TargetID:   opts.Profile.Target.TargetID,
		Now:        now,
	})
	liveInput := liveRepairInput(liveReport, liveErr, targetRoot, sessionID, persisted)
	out := Review{
		Schema:          SchemaReview,
		TargetRoot:      filepath.ToSlash(targetRoot),
		ProfileID:       opts.Profile.ProfileID,
		TargetID:        opts.Profile.Target.TargetID,
		SessionID:       sessionID,
		GeneratedAt:     now.Format(time.RFC3339Nano),
		PersistedPlan:   persisted,
		LiveRepairInput: liveInput,
		Boundaries:      reviewBoundaries(persisted, liveInput),
	}
	out.ArtifactProblems = append(out.ArtifactProblems, persisted.ArtifactProblems...)
	out.ArtifactProblems = append(out.ArtifactProblems, liveInput.ArtifactProblems...)
	summarizeReview(&out)
	return out, nil
}

func (r Review) NeedsReview() bool {
	return r.Summary.ReviewRequired
}

func liveRepairInput(report verify.DriftReport, detectorErr error, targetRoot string, sessionID string, persisted Receipt) LiveRepairInput {
	liveOnly := liveOnlyTargetDrifts(report.Drifts, persisted)
	out := LiveRepairInput{
		Source:    "live_detector",
		Durable:   false,
		SessionID: report.SessionID,
		Manifest:  report.Manifest,
		Summary: LiveRepairSummary{
			ManifestCount:    report.Summary.ManifestCount,
			ManifestEntries:  report.Summary.ManifestEntries,
			TargetDrifts:     len(liveOnly),
			ArtifactProblems: report.Summary.ArtifactProblems,
		},
		TargetDrifts: liveOnly,
	}
	switch {
	case detectorErr != nil:
		out.Action = "repair_live_detector_artifacts_before_review"
		out.Reason = "live target drift detector could not complete"
	case report.Summary.ManifestCount == 0:
		out.Action = "publish_or_select_session_before_repair"
		out.Reason = "no published manifest is available for live drift repair review"
	case len(liveOnly) > 0:
		out.Action = "run_drift_record_before_reconcile_apply"
		out.Reason = "live-only detector findings are ephemeral and must be persisted before selected reconcile apply"
	default:
		out.Action = "no_live_repair_inputs"
		out.Reason = "live detector did not find unpersisted repair inputs for the selected scope"
	}
	for _, problem := range report.ArtifactProblems {
		out.ArtifactProblems = append(out.ArtifactProblems, ArtifactProblem{
			SessionID: problem.SessionID,
			Path:      filepath.ToSlash(problem.Path),
			Error:     problem.Err,
		})
	}
	if detectorErr != nil {
		out.ArtifactProblems = append(out.ArtifactProblems, ArtifactProblem{
			SessionID: sessionID,
			Path:      filepath.ToSlash(targetRoot),
			Error:     detectorErr.Error(),
		})
	}
	out.Summary.ArtifactProblems = len(out.ArtifactProblems)
	return out
}

func liveOnlyTargetDrifts(drifts []control.TargetDrift, persisted Receipt) []control.TargetDrift {
	persistedFindings := persistedTargetDriftFindings(persisted)
	out := make([]control.TargetDrift, 0, len(drifts))
	for _, drift := range drifts {
		if persistedOpenFindingCoversLiveDrift(drift, persistedFindings) {
			continue
		}
		out = append(out, drift)
	}
	return out
}

func persistedTargetDriftFindings(persisted Receipt) []control.TargetDrift {
	out := make([]control.TargetDrift, 0, len(persisted.Actions)+len(persisted.Refusals))
	for _, action := range persisted.Actions {
		out = append(out, control.TargetDrift{
			ID:          action.DriftID,
			SessionID:   action.SessionID,
			Path:        action.Path,
			Change:      action.Change,
			ReviewState: reviewStateForAction(action),
			Expected:    targetDriftExpectedFromAction(action.Expected),
			Observed:    targetDriftObservedFromAction(action.ObservedBefore),
		})
	}
	for _, refusal := range persisted.Refusals {
		out = append(out, control.TargetDrift{
			ID:          refusal.DriftID,
			SessionID:   refusal.SessionID,
			Path:        refusal.Path,
			Change:      refusal.Change,
			ReviewState: "needs_review",
			Observed:    targetDriftObservedFromAction(refusal.ObservedBefore),
		})
	}
	return out
}

func persistedOpenFindingCoversLiveDrift(live control.TargetDrift, persisted []control.TargetDrift) bool {
	for _, record := range persisted {
		switch strings.TrimSpace(record.ReviewState) {
		case "resolved", "expired":
			continue
		}
		if strings.TrimSpace(record.ID) != "" && record.ID == live.ID {
			return true
		}
		if record.SessionID != live.SessionID || record.Path != live.Path || record.Change != live.Change {
			continue
		}
		if !targetDriftExpectedBlank(record.Expected) && !targetDriftExpectedCovers(record.Expected, live.Expected) {
			continue
		}
		if !targetDriftObservedCovers(record.Observed, live.Observed) {
			continue
		}
		return true
	}
	return false
}

func reviewStateForAction(action Action) string {
	switch action.Action {
	case ActionAlreadyResolved:
		return "resolved"
	default:
		return "needs_review"
	}
}

func targetDriftExpectedFromAction(state ExpectedState) control.TargetDriftExpectedState {
	out := control.TargetDriftExpectedState{
		SessionID:  state.SessionID,
		ManifestID: state.ManifestID,
		Kind:       state.Kind,
		Path:       state.Path,
		Digest:     state.Digest,
		ModTime:    state.ModTime,
	}
	if state.Size != 0 {
		out.SetSizeEvidence(state.Size)
	}
	if state.Mode != 0 {
		out.SetModeEvidence(state.Mode)
	}
	return out
}

func targetDriftObservedFromAction(state ObservedState) control.TargetDriftObservedState {
	out := control.TargetDriftObservedState{
		Present: state.Present,
		Kind:    state.Kind,
		Path:    state.Path,
		Digest:  state.Digest,
		ModTime: state.ModTime,
	}
	if state.Size != 0 {
		out.SetSizeEvidence(state.Size)
	}
	if state.Mode != 0 {
		out.SetModeEvidence(state.Mode)
	}
	return out
}

func targetDriftExpectedCovers(persisted control.TargetDriftExpectedState, live control.TargetDriftExpectedState) bool {
	return persisted.SessionID == live.SessionID &&
		persisted.ManifestID == live.ManifestID &&
		persisted.Kind == live.Kind &&
		persisted.Path == live.Path &&
		persisted.Digest == live.Digest &&
		persisted.ModTime == live.ModTime &&
		persisted.Size == live.Size &&
		persisted.HasSizeEvidence() == live.HasSizeEvidence() &&
		persisted.Mode == live.Mode &&
		persisted.HasModeEvidence() == live.HasModeEvidence() &&
		persisted.SymlinkTarget == live.SymlinkTarget
}

func targetDriftExpectedBlank(state control.TargetDriftExpectedState) bool {
	return state.SessionID == "" &&
		state.ManifestID == "" &&
		state.Kind == "" &&
		state.Path == "" &&
		state.Digest == "" &&
		state.ModTime == "" &&
		state.Size == 0 &&
		!state.HasSizeEvidence() &&
		state.Mode == 0 &&
		!state.HasModeEvidence() &&
		state.SymlinkTarget == ""
}

func targetDriftObservedCovers(persisted control.TargetDriftObservedState, live control.TargetDriftObservedState) bool {
	return reflect.DeepEqual(persisted.Present, live.Present) &&
		persisted.Kind == live.Kind &&
		persisted.Path == live.Path &&
		persisted.Digest == live.Digest &&
		persisted.ModTime == live.ModTime &&
		persisted.Size == live.Size &&
		persisted.HasSizeEvidence() == live.HasSizeEvidence() &&
		persisted.Mode == live.Mode &&
		persisted.HasModeEvidence() == live.HasModeEvidence() &&
		persisted.SymlinkTarget == live.SymlinkTarget
}

func reviewBoundaries(persisted Receipt, live LiveRepairInput) []RepairBoundary {
	boundaries := []RepairBoundary{
		{
			Name:         BoundaryPersistentDrift,
			Status:       BoundaryStatusWiredReadOnly,
			Source:       "persisted_target_drift",
			Action:       "run_reconcile_plan_then_selected_apply",
			ApplyCapable: persisted.Summary.Planned > 0,
			Summary: BoundarySummary{
				Records:          persisted.Summary.Records,
				Planned:          persisted.Summary.Planned,
				Noop:             persisted.Summary.Noop,
				Refused:          persisted.Summary.Refused,
				ArtifactProblems: persisted.Summary.ArtifactProblems,
			},
			Reason: "only selected persisted drift records can reach reconcile apply; apply still requires --id, --apply, --reason, preflight, and a durable receipt",
		},
		liveOnlyBoundary(live),
		plannedBoundary(BoundaryBackgroundScan, "detector_scheduler", "keep_background_scans_planned", "no automatic background detector loop persists repair evidence yet"),
		plannedBoundary(BoundaryManifestRewrite, "manifest_control_plane", "keep_manifest_rewrite_planned", "manifest rewrite decisions are not wired and must not be inferred from repair review"),
		plannedBoundary(BoundaryDaemonSync, "daemon_incremental_sync", "keep_daemon_sync_integration_planned", "foreground daemon and ongoing sync do not run broad repair"),
		plannedBoundary(BoundaryDriftToPrune, "prune_review", "keep_drift_to_prune_handoff_planned", "drift-to-prune handoff is not wired; prune remains approval-gated"),
		plannedBoundary(BoundaryAutomaticRetry, "retry_policy", "keep_automatic_retry_planned", "conflict retry advice is operator guidance, not automatic retry"),
	}
	return boundaries
}

func liveOnlyBoundary(live LiveRepairInput) RepairBoundary {
	status := BoundaryStatusWiredReadOnly
	if live.Summary.TargetDrifts > 0 {
		status = BoundaryStatusRecordRequired
	} else if live.Summary.ArtifactProblems > 0 || live.Summary.ManifestCount == 0 {
		status = BoundaryStatusBlocked
	}
	return RepairBoundary{
		Name:         BoundaryLiveOnlyDrift,
		Status:       status,
		Source:       "live_detector",
		Action:       live.Action,
		ApplyCapable: false,
		Summary: BoundarySummary{
			TargetDrifts:     live.Summary.TargetDrifts,
			ArtifactProblems: live.Summary.ArtifactProblems,
		},
		Reason: live.Reason,
	}
}

func plannedBoundary(name string, source string, action string, reason string) RepairBoundary {
	return RepairBoundary{
		Name:         name,
		Status:       BoundaryStatusPlanned,
		Source:       source,
		Action:       action,
		ApplyCapable: false,
		Reason:       reason,
	}
}

func summarizeReview(review *Review) {
	review.Summary.PersistedRecords = review.PersistedPlan.Summary.Records
	review.Summary.PersistedPlanned = review.PersistedPlan.Summary.Planned
	review.Summary.PersistedNoop = review.PersistedPlan.Summary.Noop
	review.Summary.PersistedRefused = review.PersistedPlan.Summary.Refused
	review.Summary.PersistedArtifactProblems = review.PersistedPlan.Summary.ArtifactProblems
	review.Summary.LiveManifestCount = review.LiveRepairInput.Summary.ManifestCount
	review.Summary.LiveTargetDrifts = review.LiveRepairInput.Summary.TargetDrifts
	review.Summary.LiveArtifactProblems = review.LiveRepairInput.Summary.ArtifactProblems
	review.Summary.Boundaries = len(review.Boundaries)
	for _, boundary := range review.Boundaries {
		if boundary.ApplyCapable {
			review.Summary.ApplyCapableBoundaries++
		}
		switch boundary.Status {
		case BoundaryStatusRecordRequired:
			review.Summary.RecordRequiredBoundaries++
		case BoundaryStatusPlanned:
			review.Summary.PlannedBoundaries++
		}
	}
	review.Summary.ReviewRequired = review.Summary.PersistedRefused > 0 ||
		review.Summary.PersistedArtifactProblems > 0 ||
		review.Summary.PersistedPlanned > 0 ||
		review.Summary.LiveTargetDrifts > 0 ||
		review.Summary.LiveArtifactProblems > 0 ||
		review.Summary.LiveManifestCount == 0
}
