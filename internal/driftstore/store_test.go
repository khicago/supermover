package driftstore

import (
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/khicago/supermover/internal/control"
)

func TestPlanRejectsDuplicateDriftID(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target")
	drift := testTargetDrift("detected_file_txt")

	_, err := Plan(target, []control.TargetDrift{drift, drift})
	if err == nil || !strings.Contains(err.Error(), `duplicate target drift id "detected_file_txt"`) {
		t.Fatalf("Plan(duplicate ids) error = %v, want duplicate id refusal", err)
	}
}

func TestPutReusesExistingLogicalFindingAndPreservesReviewMetadata(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target")
	existing := testTargetDrift("detected_file_txt")
	existing.ReviewState = "acknowledged"
	existing.ReviewAction = "acknowledge"
	existing.ReviewedAt = "2026-05-20T02:00:00Z"
	existing.ReviewedBy = "ops"
	existing.ReviewReason = "operator accepted target-local edit"
	path := writeTargetDrift(t, target, existing)

	detected := existing
	detected.DetectedAt = "2026-05-20T03:04:05Z"
	detected.ReviewState = "resolved"
	detected.ReviewAction = "resolve"
	detected.ReviewedAt = "2026-05-20T04:00:00Z"
	detected.ReviewedBy = "other-ops"
	detected.ReviewReason = "new write metadata must not replace existing review"

	result, err := Put(target, detected)
	if err != nil {
		t.Fatalf("Put(existing logical finding) error = %v, want nil", err)
	}
	if result.Created || !result.Existing || !reflect.DeepEqual(result.Drift, existing) {
		t.Fatalf("Put(existing logical finding) = %+v, want existing preserved drift %+v", result, existing)
	}
	persisted, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(existing drift) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(persisted, existing) {
		t.Fatalf("persisted drift after Put = %+v, want existing review metadata preserved %+v", persisted, existing)
	}
}

func TestPutReopensResolvedExistingLogicalFindingWithHistory(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target")
	existing := testTargetDrift("detected_file_txt")
	existing.DetectedAt = "2026-05-20T01:02:03Z"
	existing.ReviewState = "resolved"
	existing.ReviewAction = "resolve"
	existing.ReviewedAt = "2026-05-20T02:00:00Z"
	existing.ReviewedBy = "ops"
	existing.ReviewReason = "target restored"
	path := writeTargetDrift(t, target, existing)

	detected := existing
	detected.DetectedAt = "2026-05-20T03:04:05Z"
	detected.ReviewState = "needs_review"
	detected.ReviewAction = ""
	detected.ReviewedAt = ""
	detected.ReviewedBy = ""
	detected.ReviewReason = ""

	result, err := Put(target, detected)
	if err != nil {
		t.Fatalf("Put(resolved existing logical finding) error = %v, want nil", err)
	}
	if result.Created || result.Existing || !result.Reopened {
		t.Fatalf("Put(resolved existing logical finding) = %+v, want reopened result", result)
	}
	if result.Drift.ReviewState != "needs_review" || result.Drift.ReviewAction != "" || result.Drift.ReviewedAt != "" || result.Drift.ReviewedBy != "" || result.Drift.ReviewReason != "" {
		t.Fatalf("reopened drift review fields = %+v, want active needs_review without current review evidence", result.Drift)
	}
	if result.Drift.DetectedAt != existing.DetectedAt || result.Drift.LastDetectedAt != detected.DetectedAt {
		t.Fatalf("reopened drift detection times = detected_at %q last %q, want original %q and latest %q", result.Drift.DetectedAt, result.Drift.LastDetectedAt, existing.DetectedAt, detected.DetectedAt)
	}
	if len(result.Drift.ReviewHistory) != 1 {
		t.Fatalf("reopened drift history = %+v, want one prior review event", result.Drift.ReviewHistory)
	}
	event := result.Drift.ReviewHistory[0]
	if event.ReviewState != "resolved" || event.ReviewAction != "resolve" || event.ReviewedAt != existing.ReviewedAt || event.ReviewedBy != existing.ReviewedBy || event.ReviewReason != existing.ReviewReason || event.ReconciledAt != detected.DetectedAt || event.ReconcileAction != "reopen" {
		t.Fatalf("reopened drift history event = %+v, want prior resolve evidence plus reopen reconcile action", event)
	}
	persisted, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(reopened drift) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(persisted, result.Drift) {
		t.Fatalf("persisted reopened drift = %+v, want returned drift %+v", persisted, result.Drift)
	}
}

func TestPlanRejectsExistingSameIDWithMismatchedObservedOrEvidence(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*control.TargetDrift)
		wantErr string
	}{
		{
			name: "observed evidence",
			mutate: func(drift *control.TargetDrift) {
				drift.Observed.Digest = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
			},
			wantErr: "observed evidence does not match current detection",
		},
		{
			name: "evidence strings",
			mutate: func(drift *control.TargetDrift) {
				drift.Evidence = append(drift.Evidence, "tampered evidence")
			},
			wantErr: "evidence strings do not match current detection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "target")
			detected := testTargetDrift("detected_file_txt")
			existing := detected
			tt.mutate(&existing)
			writeTargetDrift(t, target, existing)

			_, err := Plan(target, []control.TargetDrift{detected})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Plan(mismatched existing drift) error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestPutNormalizesReviewMetadataForNewWrites(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target")
	drift := testTargetDrift("detected_file_txt")
	drift.ReviewState = "acknowledged"
	drift.ReviewAction = "acknowledge"
	drift.ReviewedAt = "2026-05-20T02:00:00Z"
	drift.ReviewedBy = "ops"
	drift.ReviewReason = "caller-supplied metadata must not survive new write"

	result, err := Put(target, drift)
	if err != nil {
		t.Fatalf("Put(new drift) error = %v, want nil", err)
	}
	if !result.Created || result.Existing {
		t.Fatalf("Put(new drift) = %+v, want created result", result)
	}
	if result.Drift.ReviewState != "needs_review" || result.Drift.ReviewAction != "" || result.Drift.ReviewedAt != "" || result.Drift.ReviewedBy != "" || result.Drift.ReviewReason != "" {
		t.Fatalf("Put(new drift).Drift = %+v, want normalized needs_review without review metadata", result.Drift)
	}

	path, err := control.Path(target, control.ArtifactTargetDrift, drift.ID)
	if err != nil {
		t.Fatalf("control.Path(target drift) error = %v, want nil", err)
	}
	persisted, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(new drift) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(persisted, result.Drift) {
		t.Fatalf("persisted new drift = %+v, want result drift %+v", persisted, result.Drift)
	}
}

func TestApplyRepreflightsPlannedNewDriftsBeforeCreatingAnyArtifact(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target")
	first := testTargetDrift("detected_first")
	first.Path = "first.txt"
	first.Expected.Path = "first.txt"
	first.Observed.Path = "first.txt"
	second := testTargetDrift("detected_second")
	second.Path = "second.txt"
	second.Expected.Path = "second.txt"
	second.Observed.Path = "second.txt"

	plan, err := Plan(target, []control.TargetDrift{first, second})
	if err != nil {
		t.Fatalf("Plan(two new drifts) error = %v, want nil", err)
	}
	if len(plan) != 2 || plan[0].Existing != nil || plan[1].Existing != nil {
		t.Fatalf("Plan(two new drifts) = %+v, want two planned-new artifacts", plan)
	}

	conflict := second
	conflict.Evidence = append(conflict.Evidence, "external conflicting evidence")
	writeTargetDrift(t, target, conflict)

	_, err = Apply(plan)
	if err == nil || !strings.Contains(err.Error(), "evidence strings do not match current detection") {
		t.Fatalf("Apply(plan with conflicting second artifact) error = %v, want conflict refusal before writes", err)
	}
	assertTargetDriftNotExist(t, target, first.ID)
}

func testTargetDrift(id string) control.TargetDrift {
	present := true
	expected := control.TargetDriftExpectedState{
		SessionID:  "session-one",
		ManifestID: "manifest-session-one",
		Kind:       "file",
		Path:       "file.txt",
		Digest:     "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ModTime:    "2026-05-20T01:00:00Z",
	}
	expected.SetSizeEvidence(4)
	expected.SetModeEvidence(0o644)
	observed := control.TargetDriftObservedState{
		Present: &present,
		Kind:    "file",
		Path:    "file.txt",
		Digest:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ModTime: "2026-05-20T01:01:00Z",
	}
	observed.SetSizeEvidence(13)
	observed.SetModeEvidence(0o644)
	return control.TargetDrift{
		Version:     control.CurrentVersion,
		ID:          id,
		SessionID:   "session-one",
		ProfileID:   "profile-local",
		TargetID:    "local:profile-local",
		RootID:      "root",
		Path:        "file.txt",
		DetectedAt:  "2026-05-20T01:02:03Z",
		Change:      "content_mismatch",
		Expected:    expected,
		Observed:    observed,
		ReviewState: "needs_review",
		Evidence: []string{
			"expected digest sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"observed digest sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}
}

func writeTargetDrift(t *testing.T, target string, drift control.TargetDrift) string {
	t.Helper()
	path := targetDriftPath(t, target, drift.ID)
	if err := control.WriteFile(path, drift); err != nil {
		t.Fatalf("control.WriteFile(target drift) error = %v, want nil", err)
	}
	return path
}

func assertTargetDriftNotExist(t *testing.T, target string, id string) {
	t.Helper()
	path := targetDriftPath(t, target, id)
	if _, err := control.ReadFile[control.TargetDrift](path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("control.ReadFile(%q) error = %v, want not exist", path, err)
	}
}

func targetDriftPath(t *testing.T, target string, id string) string {
	t.Helper()
	path, err := control.Path(target, control.ArtifactTargetDrift, id)
	if err != nil {
		t.Fatalf("control.Path(target drift) error = %v, want nil", err)
	}
	return path
}
