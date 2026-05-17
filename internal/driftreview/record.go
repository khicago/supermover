package driftreview

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/driftstore"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/targetlock"
	"github.com/khicago/supermover/internal/verify"
)

// RecordOptions selects the profile-scoped target evidence to persist.
type RecordOptions struct {
	Profile   profile.Profile
	SessionID string
	Now       time.Time
}

// RecordResult summarizes durable live-detector record creation.
type RecordResult struct {
	TargetRoot       string                   `json:"target_root"`
	SessionID        string                   `json:"session_id,omitempty"`
	Detected         int                      `json:"detected"`
	Recorded         int                      `json:"recorded"`
	Existing         int                      `json:"existing"`
	Reopened         int                      `json:"reopened"`
	ManifestCount    int                      `json:"manifest_count"`
	ArtifactProblems []verify.ArtifactProblem `json:"artifact_problems,omitempty"`
	Records          []RecordedTargetDrift    `json:"records,omitempty"`
}

// RecordedTargetDrift is one detected finding and whether it was newly stored.
type RecordedTargetDrift struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Change      string `json:"change"`
	SessionID   string `json:"session_id"`
	ReviewState string `json:"review_state"`
	Recorded    bool   `json:"recorded"`
	Existing    bool   `json:"existing"`
	Reopened    bool   `json:"reopened"`
}

func (r RecordResult) NeedsReview() bool {
	return r.Detected > 0 || len(r.ArtifactProblems) > 0 || r.ManifestCount == 0
}

// Record persists current live detector findings as target-drift review records.
// It derives the target only from the profile, writes no-replace artifacts, and
// does not resolve, repair, prune, or suppress future detector output.
func Record(opts RecordOptions) (RecordResult, error) {
	targetRoot, err := targetRootFromProfile(opts.Profile)
	if err != nil {
		return RecordResult{}, err
	}
	if err := control.ValidateArtifactLoadBoundary(targetRoot); err != nil {
		return RecordResult{}, err
	}
	if _, err := os.Lstat(targetRoot); err != nil && errors.Is(err, os.ErrNotExist) {
		return RecordResult{TargetRoot: filepath.ToSlash(targetRoot)}, nil
	} else if err != nil {
		return RecordResult{}, fmt.Errorf("inspect target root: %w", err)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	report, err := detectRecordDrift(targetRoot, opts, now)
	if err != nil {
		return RecordResult{}, err
	}
	result := recordResultFromReport(report)
	if len(report.Drifts) == 0 {
		return result, nil
	}

	unlock, err := targetlock.LockTarget(targetRoot)
	if err != nil {
		return RecordResult{}, fmt.Errorf("lock target for drift record: %w", err)
	}
	defer unlock()

	report, err = detectRecordDrift(targetRoot, opts, now)
	if err != nil {
		return RecordResult{}, err
	}
	result = recordResultFromReport(report)
	if len(report.Drifts) == 0 {
		return result, nil
	}

	plan, err := driftstore.Plan(targetRoot, report.Drifts)
	if err != nil {
		return RecordResult{}, err
	}
	written, err := driftstore.Apply(plan)
	if err != nil {
		return RecordResult{}, err
	}
	for _, item := range written {
		if item.Created {
			result.Recorded++
		}
		if item.Existing {
			result.Existing++
		}
		if item.Reopened {
			result.Reopened++
		}
		result.Records = append(result.Records, recordedTargetDrift(item.Drift, item.Created, item.Existing, item.Reopened))
	}
	return result, nil
}

func detectRecordDrift(targetRoot string, opts RecordOptions, now time.Time) (verify.DriftReport, error) {
	return verify.DetectTargetDrift(verify.DriftOptions{
		TargetRoot: targetRoot,
		SessionID:  strings.TrimSpace(opts.SessionID),
		ProfileID:  opts.Profile.ProfileID,
		TargetID:   opts.Profile.Target.TargetID,
		Now:        now,
	})
}

func recordResultFromReport(report verify.DriftReport) RecordResult {
	return RecordResult{
		TargetRoot:       report.TargetRoot,
		SessionID:        report.SessionID,
		Detected:         len(report.Drifts),
		ManifestCount:    report.Summary.ManifestCount,
		ArtifactProblems: append([]verify.ArtifactProblem(nil), report.ArtifactProblems...),
	}
}

func recordedTargetDrift(drift control.TargetDrift, recorded bool, existing bool, reopened bool) RecordedTargetDrift {
	return RecordedTargetDrift{
		ID:          drift.ID,
		Path:        drift.Path,
		Change:      drift.Change,
		SessionID:   drift.SessionID,
		ReviewState: driftstore.ReviewState(drift),
		Recorded:    recorded,
		Existing:    existing,
		Reopened:    reopened,
	}
}
