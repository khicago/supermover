package driftstore

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strings"

	"github.com/khicago/supermover/internal/control"
)

// PlanItem is one target-drift artifact write after existing artifacts have
// been preflighted for compatibility.
type PlanItem struct {
	Path        string
	Drift       control.TargetDrift
	Existing    *control.TargetDrift
	controlRoot string
}

// WriteResult reports whether a target-drift artifact was created or reused.
type WriteResult struct {
	Drift    control.TargetDrift
	Created  bool
	Existing bool
	Reopened bool
}

// Plan validates all detected drift artifacts and preflights existing files
// before callers mutate the target control plane.
func Plan(targetRoot string, drifts []control.TargetDrift) ([]PlanItem, error) {
	controlRoot := control.ControlDir(targetRoot)
	seen := make(map[string]struct{}, len(drifts))
	plan := make([]PlanItem, 0, len(drifts))
	for _, drift := range drifts {
		normalized, err := normalizeNewDrift(drift)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[normalized.ID]; ok {
			return nil, fmt.Errorf("duplicate target drift id %q in write plan", normalized.ID)
		}
		seen[normalized.ID] = struct{}{}
		path, err := control.Path(targetRoot, control.ArtifactTargetDrift, normalized.ID)
		if err != nil {
			return nil, err
		}
		existing, err := control.ReadFileNoSymlinkUnderRoot[control.TargetDrift](controlRoot, path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				plan = append(plan, PlanItem{Path: path, Drift: normalized, controlRoot: controlRoot})
				continue
			}
			return nil, fmt.Errorf("read existing target drift: %w", err)
		}
		if existing.ID != normalized.ID {
			return nil, fmt.Errorf("existing target drift id %q does not match detected id %q", existing.ID, normalized.ID)
		}
		if err := RequireSameLogicalFinding(existing, normalized); err != nil {
			return nil, err
		}
		existingCopy := existing
		plan = append(plan, PlanItem{Path: path, Drift: normalized, Existing: &existingCopy, controlRoot: controlRoot})
	}
	return plan, nil
}

// Apply writes a preflighted target-drift plan with no-replace semantics.
func Apply(plan []PlanItem) ([]WriteResult, error) {
	refreshed, err := refreshApplyPlan(plan)
	if err != nil {
		return nil, err
	}
	results := make([]WriteResult, 0, len(refreshed))
	created := make([]PlanItem, 0, len(refreshed))
	for _, item := range refreshed {
		if item.Existing != nil {
			reconciled, reopened := reconcileExisting(*item.Existing, item.Drift)
			if reopened {
				if err := control.WriteFile(item.Path, reconciled); err != nil {
					return nil, errors.Join(err, removeCreated(created))
				}
				results = append(results, WriteResult{Drift: reconciled, Reopened: true})
				continue
			}
			results = append(results, WriteResult{Drift: reconciled, Existing: true})
			continue
		}
		if err := control.WriteNewFile(item.Path, item.Drift); err == nil {
			results = append(results, WriteResult{Drift: item.Drift, Created: true})
			created = append(created, item)
			continue
		} else if !errors.Is(err, control.ErrArtifactExists) {
			return nil, errors.Join(err, removeCreated(created))
		}
		existing, err := readCompatibleExisting(item)
		if err != nil {
			return nil, errors.Join(err, removeCreated(created))
		}
		reconciled, reopened := reconcileExisting(existing, item.Drift)
		if reopened {
			if err := control.WriteFile(item.Path, reconciled); err != nil {
				return nil, errors.Join(err, removeCreated(created))
			}
			results = append(results, WriteResult{Drift: reconciled, Reopened: true})
			continue
		}
		results = append(results, WriteResult{Drift: reconciled, Existing: true})
	}
	return results, nil
}

// Put writes or reuses a single target-drift artifact through the shared policy.
func Put(targetRoot string, drift control.TargetDrift) (WriteResult, error) {
	plan, err := Plan(targetRoot, []control.TargetDrift{drift})
	if err != nil {
		return WriteResult{}, err
	}
	results, err := Apply(plan)
	if err != nil {
		return WriteResult{}, err
	}
	if len(results) != 1 {
		return WriteResult{}, fmt.Errorf("target drift write produced %d results, want 1", len(results))
	}
	return results[0], nil
}

func refreshApplyPlan(plan []PlanItem) ([]PlanItem, error) {
	refreshed := make([]PlanItem, len(plan))
	copy(refreshed, plan)
	for i := range refreshed {
		existing, err := readCompatibleExisting(refreshed[i])
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && refreshed[i].Existing == nil {
				continue
			}
			return nil, err
		}
		existingCopy := existing
		refreshed[i].Existing = &existingCopy
	}
	return refreshed, nil
}

func readCompatibleExisting(item PlanItem) (control.TargetDrift, error) {
	existing, err := control.ReadFileNoSymlinkUnderRoot[control.TargetDrift](item.controlRoot, item.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return control.TargetDrift{}, err
		}
		return control.TargetDrift{}, fmt.Errorf("read existing target drift: %w", err)
	}
	if existing.ID != item.Drift.ID {
		return control.TargetDrift{}, fmt.Errorf("existing target drift id %q does not match detected id %q", existing.ID, item.Drift.ID)
	}
	if err := RequireSameLogicalFinding(existing, item.Drift); err != nil {
		return control.TargetDrift{}, err
	}
	return existing, nil
}

func reconcileExisting(existing control.TargetDrift, detected control.TargetDrift) (control.TargetDrift, bool) {
	if ReviewState(existing) != "resolved" {
		return existing, false
	}
	reconciled := detected
	reconciled.DetectedAt = existing.DetectedAt
	reconciled.LastDetectedAt = detected.DetectedAt
	reconciled.ReviewHistory = append([]control.TargetDriftReviewEvent(nil), existing.ReviewHistory...)
	reconciled.ReviewHistory = append(reconciled.ReviewHistory, control.TargetDriftReviewEvent{
		ReviewState:     existing.ReviewState,
		ReviewAction:    existing.ReviewAction,
		ReviewedAt:      existing.ReviewedAt,
		ReviewedBy:      existing.ReviewedBy,
		ReviewReason:    existing.ReviewReason,
		ReconciledAt:    detected.DetectedAt,
		ReconcileAction: "reopen",
	})
	reconciled.ReviewState = "needs_review"
	reconciled.ReviewedAt = ""
	reconciled.ReviewedBy = ""
	reconciled.ReviewReason = ""
	reconciled.ReviewAction = ""
	return reconciled, true
}

func removeCreated(created []PlanItem) error {
	var errs []error
	for i := len(created) - 1; i >= 0; i-- {
		item := created[i]
		current, err := control.ReadFileNoSymlinkUnderRoot[control.TargetDrift](item.controlRoot, item.Path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			errs = append(errs, fmt.Errorf("inspect created target drift %q for rollback: %w", item.Drift.ID, err))
			continue
		}
		if !reflect.DeepEqual(current, item.Drift) {
			errs = append(errs, fmt.Errorf("created target drift %q changed before rollback", item.Drift.ID))
			continue
		}
		if err := os.Remove(item.Path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove created target drift %q: %w", item.Drift.ID, err))
		}
	}
	return errors.Join(errs...)
}

func normalizeNewDrift(drift control.TargetDrift) (control.TargetDrift, error) {
	if err := control.ValidateArtifactID(drift.ID); err != nil {
		return control.TargetDrift{}, fmt.Errorf("target drift id is unsafe: %w", err)
	}
	drift.LastDetectedAt = ""
	drift.ReviewState = "needs_review"
	drift.ReviewedAt = ""
	drift.ReviewedBy = ""
	drift.ReviewReason = ""
	drift.ReviewAction = ""
	drift.ReviewHistory = nil
	if err := drift.Validate(); err != nil {
		return control.TargetDrift{}, err
	}
	return drift, nil
}

// RequireSameLogicalFinding rejects reuse when an existing artifact does not
// describe the same detected drift. Review metadata is intentionally ignored.
func RequireSameLogicalFinding(existing control.TargetDrift, detected control.TargetDrift) error {
	if existing.ProfileID != detected.ProfileID || existing.TargetID != detected.TargetID || existing.RootID != detected.RootID {
		return fmt.Errorf("existing target drift %q scope (%q/%q/%q) does not match detected scope (%q/%q/%q)",
			existing.ID,
			existing.ProfileID,
			existing.TargetID,
			existing.RootID,
			detected.ProfileID,
			detected.TargetID,
			detected.RootID,
		)
	}
	if existing.SessionID != detected.SessionID {
		return fmt.Errorf("existing target drift %q session_id %q does not match detected session_id %q", existing.ID, existing.SessionID, detected.SessionID)
	}
	if existing.Path != detected.Path || existing.Change != detected.Change {
		return fmt.Errorf("existing target drift %q path/change (%q/%q) does not match detected path/change (%q/%q)",
			existing.ID,
			existing.Path,
			existing.Change,
			detected.Path,
			detected.Change,
		)
	}
	if !reflect.DeepEqual(existing.Expected, detected.Expected) {
		return fmt.Errorf("existing target drift %q expected evidence does not match current detection", existing.ID)
	}
	if !reflect.DeepEqual(existing.Observed, detected.Observed) {
		return fmt.Errorf("existing target drift %q observed evidence does not match current detection", existing.ID)
	}
	if !reflect.DeepEqual(existing.Evidence, detected.Evidence) {
		return fmt.Errorf("existing target drift %q evidence strings do not match current detection", existing.ID)
	}
	return nil
}

// ReviewState returns the effective review state for a drift artifact.
func ReviewState(drift control.TargetDrift) string {
	reviewState := strings.TrimSpace(drift.ReviewState)
	if reviewState == "" {
		return "needs_review"
	}
	return reviewState
}
