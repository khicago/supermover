package driftreview

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
)

func TestAcknowledgeUpdatesPersistedReviewMetadataOnly(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	drift := validTargetDrift()
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetDrift(t, target, drift)

	got, err := Acknowledge(AcknowledgeOptions{
		Profile:  p,
		ID:       drift.ID,
		Reason:   "accepted as operator-owned target edit",
		Reviewer: "ops@example.com",
		Now:      now,
	})
	if err != nil {
		t.Fatalf("Acknowledge() error = %v, want nil", err)
	}
	if got.ID != drift.ID || got.Path != drift.Path || got.PreviousState != "needs_review" || got.ReviewState != "acknowledged" || got.ReviewedAt != now.Format(time.RFC3339Nano) || got.Reviewer != "ops@example.com" || got.Reason != "accepted as operator-owned target edit" || got.ProfileID != p.ProfileID || got.TargetID != p.Target.TargetID || got.SessionID != drift.SessionID {
		t.Fatalf("Acknowledge() = %+v, want review evidence for persisted drift", got)
	}

	path := targetDriftPath(t, target, drift.ID)
	persisted, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	if persisted.ReviewState != "acknowledged" || persisted.ReviewedAt != now.Format(time.RFC3339Nano) || persisted.ReviewedBy != "ops@example.com" || persisted.ReviewReason != "accepted as operator-owned target edit" || persisted.ReviewAction != "acknowledge" {
		t.Fatalf("persisted drift review metadata = %+v, want acknowledgement fields", persisted)
	}
	if persisted.Change != drift.Change || persisted.Expected.Kind != drift.Expected.Kind || persisted.Observed.Kind != drift.Observed.Kind {
		t.Fatalf("persisted drift changed non-review evidence = %+v, want original drift evidence preserved", persisted)
	}
}

func TestResolveMarksPersistedDriftResolvedAfterTargetMatchesExpected(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	drift := validTargetDrift()
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetFile(t, target, "file.txt", []byte("aaaaaaa"), 0o644)
	writeTargetDrift(t, target, drift)

	got, err := Resolve(ResolveOptions{
		Profile:  p,
		ID:       drift.ID,
		Reason:   "target restored to published manifest evidence",
		Reviewer: "ops@example.com",
		Now:      now,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if got.ID != drift.ID || got.Path != drift.Path || got.PreviousState != "needs_review" || got.ReviewState != "resolved" || got.ReviewedAt != now.Format(time.RFC3339Nano) || got.Reviewer != "ops@example.com" || got.Reason != "target restored to published manifest evidence" || got.ProfileID != p.ProfileID || got.TargetID != p.Target.TargetID || got.SessionID != drift.SessionID {
		t.Fatalf("Resolve() = %+v, want resolved review evidence for persisted drift", got)
	}

	path := targetDriftPath(t, target, drift.ID)
	persisted, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	if persisted.ReviewState != "resolved" || persisted.ReviewedAt != now.Format(time.RFC3339Nano) || persisted.ReviewedBy != "ops@example.com" || persisted.ReviewReason != "target restored to published manifest evidence" || persisted.ReviewAction != "resolve" {
		t.Fatalf("persisted drift review metadata = %+v, want resolve fields", persisted)
	}
	if persisted.Change != drift.Change || persisted.Expected.Kind != drift.Expected.Kind || persisted.Observed.Kind != drift.Observed.Kind || persisted.Observed.Digest != drift.Observed.Digest {
		t.Fatalf("persisted drift changed non-review evidence = %+v, want original drift evidence preserved", persisted)
	}
}

func TestResolveRejectsPersistedDriftStillDetected(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	drift := validTargetDrift()
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetFile(t, target, "file.txt", []byte("bbbbbbbbb"), 0o644)
	writeTargetDrift(t, target, drift)

	_, err := Resolve(ResolveOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "still reports drift") {
		t.Fatalf("Resolve(still detected) error = %v, want still-detected refusal", err)
	}
	persisted, readErr := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, drift.ID))
	if readErr != nil {
		t.Fatalf("control.ReadFile(target drift) error = %v, want nil", readErr)
	}
	if persisted.ReviewState != "needs_review" || persisted.ReviewAction != "" || persisted.ReviewReason != "" {
		t.Fatalf("persisted review evidence = %+v, want refusal before metadata write", persisted)
	}
}

func TestResolveRejectsDifferentCurrentDriftForSamePathAndExpectedBaseline(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	drift := validTargetDrift()
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetFile(t, target, "file.txt", []byte("different"), 0o600)
	writeTargetDrift(t, target, drift)

	_, err := Resolve(ResolveOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "still reports drift") {
		t.Fatalf("Resolve(different current drift) error = %v, want same-path baseline refusal", err)
	}
	persisted, readErr := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, drift.ID))
	if readErr != nil {
		t.Fatalf("control.ReadFile(target drift) error = %v, want nil", readErr)
	}
	if persisted.ReviewState != "needs_review" || persisted.ReviewAction != "" || persisted.ReviewReason != "" {
		t.Fatalf("persisted review evidence = %+v, want refusal before metadata write", persisted)
	}
}

func TestResolveRejectsAgedExtraDriftWhilePathStillExists(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	drift := extraTargetDrift("drift-aged-extra", "session-one", "extra.txt")
	writePublishedSession(t, target, "session-one", p.ProfileID, p.Target.TargetID, drift.RootID)
	writePublishedSession(t, target, "session-two", p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetFile(t, target, "file.txt", []byte("aaaaaaa"), 0o644)
	writeTargetFile(t, target, "extra.txt", []byte("still here"), 0o644)
	writeTargetDrift(t, target, drift)

	_, err := Resolve(ResolveOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "still reports drift for extra path") {
		t.Fatalf("Resolve(aged extra still present) error = %v, want extra-path refusal", err)
	}
	persisted, readErr := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, drift.ID))
	if readErr != nil {
		t.Fatalf("control.ReadFile(target drift) error = %v, want nil", readErr)
	}
	if persisted.ReviewState != "needs_review" || persisted.ReviewAction != "" || persisted.ReviewReason != "" {
		t.Fatalf("persisted review evidence = %+v, want refusal before metadata write", persisted)
	}
}

func TestResolveAcceptsExtraDriftAfterPathRemoved(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	drift := extraTargetDrift("drift-extra-removed", "session-one", "extra.txt")
	writePublishedSession(t, target, "session-one", p.ProfileID, p.Target.TargetID, drift.RootID)
	writePublishedSession(t, target, "session-two", p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetFile(t, target, "file.txt", []byte("aaaaaaa"), 0o644)
	writeTargetDrift(t, target, drift)

	got, err := Resolve(ResolveOptions{Profile: p, ID: drift.ID, Reason: "extra path removed"})
	if err != nil {
		t.Fatalf("Resolve(extra removed) error = %v, want nil", err)
	}
	if got.ReviewState != "resolved" || got.ID != drift.ID {
		t.Fatalf("Resolve(extra removed) = %+v, want resolved extra drift", got)
	}
}

func TestAcknowledgeAcceptsFailedAttemptDriftWithPublishedExpectedBaseline(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	drift := failedAttemptTargetDrift()
	writePublishedSession(t, target, drift.Expected.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetDrift(t, target, drift)

	got, err := Acknowledge(AcknowledgeOptions{
		Profile:  p,
		ID:       drift.ID,
		Reason:   "accepted failed attempt drift against published baseline",
		Reviewer: "ops@example.com",
		Now:      now,
	})
	if err != nil {
		t.Fatalf("Acknowledge() error = %v, want nil", err)
	}
	if got.SessionID != "session-two" || got.ReviewState != "acknowledged" || got.ReviewedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("Acknowledge() = %+v, want acknowledged failed attempted session drift", got)
	}

	persisted, err := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, drift.ID))
	if err != nil {
		t.Fatalf("control.ReadFile(target drift) error = %v, want nil", err)
	}
	if persisted.SessionID != "session-two" || persisted.Expected.SessionID != "session-one" || persisted.Expected.ManifestID != "manifest-session-one" || persisted.ReviewState != "acknowledged" {
		t.Fatalf("persisted failed attempt drift = %+v, want original session evidence with acknowledgement", persisted)
	}
}

func TestResolveAcceptsFailedAttemptDriftWithRestoredPublishedBaseline(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	now := time.Date(2026, 5, 20, 12, 30, 0, 0, time.UTC)
	drift := failedAttemptTargetDrift()
	writePublishedSession(t, target, drift.Expected.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetFile(t, target, "file.txt", []byte("aaaaaaa"), 0o644)
	writeTargetDrift(t, target, drift)

	got, err := Resolve(ResolveOptions{
		Profile:  p,
		ID:       drift.ID,
		Reason:   "failed attempt drift restored to published baseline",
		Reviewer: "ops@example.com",
		Now:      now,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if got.SessionID != "session-two" || got.ReviewState != "resolved" || got.ReviewedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("Resolve() = %+v, want resolved failed attempted session drift", got)
	}

	persisted, err := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, drift.ID))
	if err != nil {
		t.Fatalf("control.ReadFile(target drift) error = %v, want nil", err)
	}
	if persisted.SessionID != "session-two" || persisted.Expected.SessionID != "session-one" || persisted.Expected.ManifestID != "manifest-session-one" || persisted.ReviewState != "resolved" || persisted.ReviewAction != "resolve" {
		t.Fatalf("persisted failed attempt drift = %+v, want original session evidence with resolution", persisted)
	}
}

func TestAcknowledgeRejectsTamperedExpectedManifestEvidence(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*control.TargetDrift)
		wantErr string
	}{
		{
			name: "digest",
			mutate: func(drift *control.TargetDrift) {
				drift.Expected.Digest = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
			},
			wantErr: "expected digest",
		},
		{
			name: "size",
			mutate: func(drift *control.TargetDrift) {
				drift.Expected.SetSizeEvidence(8)
			},
			wantErr: "expected size",
		},
		{
			name: "mode",
			mutate: func(drift *control.TargetDrift) {
				drift.Expected.SetModeEvidence(0o600)
			},
			wantErr: "expected mode",
		},
		{
			name: "path",
			mutate: func(drift *control.TargetDrift) {
				drift.Expected.Path = "other.txt"
			},
			wantErr: "does not match expected path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target")
			p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
			drift := validTargetDrift()
			writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
			tt.mutate(&drift)
			writeTargetDrift(t, target, drift)

			_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Acknowledge() error = %v, want %q", err, tt.wantErr)
			}
			persisted, readErr := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, drift.ID))
			if readErr != nil {
				t.Fatalf("control.ReadFile(target drift) error = %v, want nil", readErr)
			}
			if persisted.ReviewState != "needs_review" || persisted.ReviewAction != "" || persisted.ReviewReason != "" {
				t.Fatalf("persisted review evidence = %+v, want refusal before metadata write", persisted)
			}
		})
	}
}

func TestResolveRejectsMissingReasonAndAlreadyResolved(t *testing.T) {
	tests := []struct {
		name       string
		reason     string
		state      string
		action     string
		reviewedAt string
		reviewer   string
		wantErr    string
	}{
		{
			name:    "missing reason",
			reason:  "",
			state:   "needs_review",
			wantErr: "reason is required",
		},
		{
			name:       "already resolved",
			reason:     "reviewed",
			state:      "resolved",
			action:     "resolve",
			reviewedAt: "2026-05-19T00:00:00Z",
			reviewer:   "previous-reviewer",
			wantErr:    "already resolved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target")
			p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
			drift := validTargetDrift()
			drift.ReviewState = tt.state
			drift.ReviewAction = tt.action
			drift.ReviewedAt = tt.reviewedAt
			drift.ReviewedBy = tt.reviewer
			if tt.action != "" {
				drift.ReviewReason = "previous review"
			}
			writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
			writeTargetFile(t, target, "file.txt", []byte("aaaaaaa"), 0o644)
			writeTargetDrift(t, target, drift)
			path := targetDriftPath(t, target, drift.ID)
			before, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("os.ReadFile(%q) before resolve refusal error = %v, want nil", path, readErr)
			}

			_, err := Resolve(ResolveOptions{Profile: p, ID: drift.ID, Reason: tt.reason})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Resolve() error = %v, want %q", err, tt.wantErr)
			}
			persisted, readErr := control.ReadFile[control.TargetDrift](path)
			if readErr != nil {
				t.Fatalf("control.ReadFile(target drift) error = %v, want nil", readErr)
			}
			if persisted.ReviewState != tt.state || persisted.ReviewAction != tt.action || persisted.ReviewedAt != tt.reviewedAt || persisted.ReviewedBy != tt.reviewer || persisted.ReviewReason != drift.ReviewReason {
				t.Fatalf("persisted drift after resolve refusal = %+v, want previous review metadata unchanged", persisted)
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("os.ReadFile(%q) after resolve refusal error = %v, want nil", path, readErr)
			}
			if string(before) != string(after) {
				t.Fatalf("target drift artifact changed after resolve refusal\nbefore=%s\nafter=%s", before, after)
			}
		})
	}
}

func TestResolveRejectsScopeAndEvidenceFailuresWithoutMutation(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*control.TargetDrift)
		writeScope func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift)
		wantErr    string
	}{
		{
			name: "scope mismatch",
			mutate: func(drift *control.TargetDrift) {
				drift.ProfileID = "other-profile"
			},
			writeScope: func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift) {
				t.Helper()
				writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
			},
			wantErr: "does not match profile scope",
		},
		{
			name: "missing published evidence",
			writeScope: func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift) {
				t.Helper()
			},
			wantErr: "not valid for the selected profile/session scope",
		},
		{
			name: "tampered expected digest",
			mutate: func(drift *control.TargetDrift) {
				drift.Expected.Digest = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
			},
			writeScope: func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift) {
				t.Helper()
				writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
			},
			wantErr: "expected digest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target")
			p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
			drift := validTargetDrift()
			if tt.mutate != nil {
				tt.mutate(&drift)
			}
			tt.writeScope(t, target, p, drift)
			writeTargetFile(t, target, "file.txt", []byte("aaaaaaa"), 0o644)
			writeTargetDrift(t, target, drift)

			_, err := Resolve(ResolveOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Resolve() error = %v, want %q", err, tt.wantErr)
			}
			persisted, readErr := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, drift.ID))
			if readErr != nil {
				t.Fatalf("control.ReadFile(target drift) error = %v, want nil", readErr)
			}
			if persisted.ReviewState != drift.ReviewState || persisted.ReviewAction != drift.ReviewAction || persisted.ReviewReason != drift.ReviewReason {
				t.Fatalf("persisted review evidence = %+v, want refusal before metadata write", persisted)
			}
		})
	}
}

func TestAcknowledgeRejectsMissingExpectedBaselineForNonExtraDrift(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	drift := validTargetDrift()
	drift.ID = "drift-missing-baseline"
	drift.Change = "content_mismatch"
	drift.Expected = control.TargetDriftExpectedState{
		SessionID:  drift.SessionID,
		ManifestID: "manifest-" + drift.SessionID,
		Kind:       "missing",
		Path:       "extra.txt",
	}
	drift.Path = "extra.txt"
	drift.Observed.Path = "extra.txt"
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetDrift(t, target, drift)

	_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "only valid for extra target paths") {
		t.Fatalf("Acknowledge(non-extra missing baseline) error = %v, want missing-baseline refusal", err)
	}
	persisted, readErr := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, drift.ID))
	if readErr != nil {
		t.Fatalf("control.ReadFile(target drift) error = %v, want nil", readErr)
	}
	if persisted.ReviewState != "needs_review" || persisted.ReviewAction != "" || persisted.ReviewReason != "" {
		t.Fatalf("persisted review evidence = %+v, want refusal before metadata write", persisted)
	}
}

func TestAcknowledgeRejectsExtraMissingBaselineForManifestPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	drift := validTargetDrift()
	drift.ID = "drift-extra-existing-manifest-path"
	drift.Change = "extra"
	drift.Expected = control.TargetDriftExpectedState{
		SessionID:  drift.SessionID,
		ManifestID: "manifest-" + drift.SessionID,
		Kind:       "missing",
		Path:       "file.txt",
	}
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetDrift(t, target, drift)

	_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "is present in published manifest") {
		t.Fatalf("Acknowledge(extra missing baseline for manifest path) error = %v, want manifest-presence refusal", err)
	}
	persisted, readErr := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, drift.ID))
	if readErr != nil {
		t.Fatalf("control.ReadFile(target drift) error = %v, want nil", readErr)
	}
	if persisted.ReviewState != "needs_review" || persisted.ReviewAction != "" || persisted.ReviewReason != "" {
		t.Fatalf("persisted review evidence = %+v, want refusal before metadata write", persisted)
	}
}

func TestAcknowledgeRejectsExtraMissingBaselineWithInconsistentObservedEvidence(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	drift := validTargetDrift()
	drift.ID = "drift-extra-observed-mismatch"
	drift.Change = "extra"
	drift.Path = "extra.txt"
	drift.Expected = control.TargetDriftExpectedState{
		SessionID:  drift.SessionID,
		ManifestID: "manifest-" + drift.SessionID,
		Kind:       "missing",
		Path:       "extra.txt",
	}
	drift.Observed.Path = "other.txt"
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetDrift(t, target, drift)

	_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "observed path") {
		t.Fatalf("Acknowledge(extra observed mismatch) error = %v, want observed evidence refusal", err)
	}
	persisted, readErr := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, drift.ID))
	if readErr != nil {
		t.Fatalf("control.ReadFile(target drift) error = %v, want nil", readErr)
	}
	if persisted.ReviewState != "needs_review" || persisted.ReviewAction != "" || persisted.ReviewReason != "" {
		t.Fatalf("persisted review evidence = %+v, want refusal before metadata write", persisted)
	}
}

func TestAcknowledgeRequiresReason(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	drift := validTargetDrift()
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetDrift(t, target, drift)

	_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: drift.ID, Reason: " \t"})
	if err == nil || !strings.Contains(err.Error(), "reason is required") {
		t.Fatalf("Acknowledge(missing reason) error = %v, want reason required", err)
	}
}

func TestAcknowledgeRejectsUnsafeID(t *testing.T) {
	dir := t.TempDir()
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), filepath.Join(dir, "target"))

	_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: "../drift", Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "drift id is unsafe") {
		t.Fatalf("Acknowledge(unsafe id) error = %v, want unsafe id refusal", err)
	}
}

func TestAcknowledgeRejectsScopeMismatch(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	drift := validTargetDrift()
	drift.ProfileID = "other-profile"
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetDrift(t, target, drift)

	_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "does not match profile scope") {
		t.Fatalf("Acknowledge(scope mismatch) error = %v, want scope mismatch refusal", err)
	}
}

func TestAcknowledgeRejectsMissingPersistedRecord(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", target, err)
	}
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)

	_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: "live-detector-id", Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "persisted target drift") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Acknowledge(missing persisted record) error = %v, want live-only refusal", err)
	}
	if _, statErr := os.Stat(filepath.Join(target, ".supermover", "locks")); !os.IsNotExist(statErr) {
		t.Fatalf("Acknowledge(missing persisted record) created lock state, stat err = %v, want not exist", statErr)
	}
}

func TestAcknowledgeRejectsMissingTargetWithoutCreatingLockArtifacts(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "missing-target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)

	_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: "missing-drift", Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "persisted target drift") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Acknowledge(missing target) error = %v, want persisted-not-found refusal", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("Acknowledge(missing target) created target state, stat err = %v, want not exist", statErr)
	}
}

func TestAcknowledgeRejectsRecordsNotVisibleToVerifyScope(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*control.TargetDrift)
		writeScope func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift)
		wantErr    string
	}{
		{
			name: "missing published session evidence",
			writeScope: func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift) {
				t.Helper()
			},
			wantErr: "not valid for the selected profile/session scope",
		},
		{
			name: "manifest root mismatch",
			writeScope: func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift) {
				t.Helper()
				writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, "other-root")
			},
			wantErr: "does not match expected session manifest root_id",
		},
		{
			name: "missing published manifest evidence",
			writeScope: func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift) {
				t.Helper()
				writePublishedReceipt(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID)
			},
			wantErr: "published expected session manifest is required",
		},
		{
			name: "profile root mismatch",
			mutate: func(drift *control.TargetDrift) {
				drift.RootID = "other-root"
			},
			writeScope: func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift) {
				t.Helper()
				writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
			},
			wantErr: "does not match profile roots",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target")
			p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
			drift := validTargetDrift()
			if tt.mutate != nil {
				tt.mutate(&drift)
			}
			tt.writeScope(t, target, p, drift)
			writeTargetDrift(t, target, drift)

			_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Acknowledge() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestAcknowledgeRejectsFailedAttemptDriftWithoutPublishedExpectedBaseline(t *testing.T) {
	tests := []struct {
		name       string
		writeScope func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift)
		wantErr    string
	}{
		{
			name: "missing expected published receipt",
			writeScope: func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift) {
				t.Helper()
			},
			wantErr: "published expected session receipt is required",
		},
		{
			name: "missing expected published manifest",
			writeScope: func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift) {
				t.Helper()
				writePublishedReceipt(t, target, drift.Expected.SessionID, p.ProfileID, p.Target.TargetID)
			},
			wantErr: "published expected session manifest is required",
		},
		{
			name: "expected manifest root mismatch",
			writeScope: func(t *testing.T, target string, p profile.Profile, drift control.TargetDrift) {
				t.Helper()
				writePublishedSession(t, target, drift.Expected.SessionID, p.ProfileID, p.Target.TargetID, "other-root")
			},
			wantErr: "does not match expected session manifest root_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target")
			p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
			drift := failedAttemptTargetDrift()
			tt.writeScope(t, target, p, drift)
			writeTargetDrift(t, target, drift)

			_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Acknowledge() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestAcknowledgeRejectsTargetScopeMismatch(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	drift := validTargetDrift()
	drift.TargetID = "other-target"
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	writeTargetDrift(t, target, drift)

	_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "does not match profile scope") {
		t.Fatalf("Acknowledge(target mismatch) error = %v, want scope mismatch refusal", err)
	}
}

func TestAcknowledgeRejectsRepeatedOrResolvedReview(t *testing.T) {
	tests := []struct {
		name       string
		state      string
		action     string
		wantErr    string
		reason     string
		reviewedAt string
	}{
		{
			name:       "already acknowledged",
			state:      "acknowledged",
			action:     "acknowledge",
			reason:     "already reviewed",
			reviewedAt: "2026-05-18T00:00:00Z",
			wantErr:    "already acknowledged",
		},
		{
			name:       "already resolved",
			state:      "resolved",
			action:     "resolve",
			reason:     "already resolved",
			reviewedAt: "2026-05-18T00:00:00Z",
			wantErr:    "already resolved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target")
			p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
			drift := validTargetDrift()
			drift.ReviewState = tt.state
			drift.ReviewAction = tt.action
			drift.ReviewReason = tt.reason
			drift.ReviewedAt = tt.reviewedAt
			writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
			writeTargetDrift(t, target, drift)

			_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: drift.ID, Reason: "new reason"})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Acknowledge(%s) error = %v, want %q", tt.name, err, tt.wantErr)
			}
			persisted, readErr := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, drift.ID))
			if readErr != nil {
				t.Fatalf("control.ReadFile(target drift) error = %v, want nil", readErr)
			}
			if persisted.ReviewReason != tt.reason || persisted.ReviewedAt != tt.reviewedAt {
				t.Fatalf("persisted review evidence = %+v, want original evidence preserved", persisted)
			}
		})
	}
}

func TestAcknowledgeRejectsSymlinkDriftArtifact(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	outside := filepath.Join(dir, "outside.json")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	drift := validTargetDrift()
	writePublishedSession(t, target, drift.SessionID, p.ProfileID, p.Target.TargetID, drift.RootID)
	if err := os.MkdirAll(filepath.Dir(targetDriftPath(t, target, drift.ID)), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(drift dir) error = %v, want nil", err)
	}
	if err := os.WriteFile(outside, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatalf("os.WriteFile(outside) error = %v, want nil", err)
	}
	if err := os.Symlink(outside, targetDriftPath(t, target, drift.ID)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := Acknowledge(AcknowledgeOptions{Profile: p, ID: drift.ID, Reason: "reviewed"})
	if err == nil || !strings.Contains(err.Error(), "control artifact path") || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Acknowledge(symlink drift artifact) error = %v, want symlink artifact refusal", err)
	}
}

func validTargetDrift() control.TargetDrift {
	present := true
	expectedPayload := []byte("aaaaaaa")
	expected := control.TargetDriftExpectedState{
		SessionID:  "session-one",
		ManifestID: "manifest-session-one",
		Kind:       "file",
		Path:       "file.txt",
		Digest:     digest(expectedPayload),
		ModTime:    "2026-05-18T00:00:00Z",
	}
	expected.SetSizeEvidence(int64(len(expectedPayload)))
	expected.SetModeEvidence(0o644)
	observed := control.TargetDriftObservedState{
		Present: &present,
		Kind:    "file",
		Path:    "file.txt",
		Digest:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ModTime: "2026-05-19T00:00:00Z",
	}
	observed.SetSizeEvidence(9)
	observed.SetModeEvidence(0o644)
	return control.TargetDrift{
		Version:     control.CurrentVersion,
		ID:          "drift-one",
		SessionID:   "session-one",
		ProfileID:   "profile-local",
		TargetID:    "local:profile-local",
		RootID:      "root",
		Path:        "file.txt",
		DetectedAt:  "2026-05-19T00:00:00Z",
		Change:      "content_mismatch",
		Expected:    expected,
		Observed:    observed,
		ReviewState: "needs_review",
		Evidence:    []string{"target content differs from manifest evidence"},
	}
}

func failedAttemptTargetDrift() control.TargetDrift {
	drift := validTargetDrift()
	drift.ID = "drift-failed-attempt"
	drift.SessionID = "session-two"
	return drift
}

func extraTargetDrift(id string, sessionID string, path string) control.TargetDrift {
	present := true
	expected := control.TargetDriftExpectedState{
		SessionID:  sessionID,
		ManifestID: "manifest-" + sessionID,
		Kind:       "missing",
		Path:       path,
	}
	observed := control.TargetDriftObservedState{
		Present: &present,
		Kind:    "file",
		Path:    path,
		Digest:  digest([]byte("still here")),
		ModTime: "2026-05-18T00:00:00Z",
	}
	observed.SetSizeEvidence(int64(len([]byte("still here"))))
	observed.SetModeEvidence(0o644)
	return control.TargetDrift{
		Version:     control.CurrentVersion,
		ID:          id,
		SessionID:   sessionID,
		ProfileID:   "profile-local",
		TargetID:    "local:profile-local",
		RootID:      "root",
		Path:        path,
		DetectedAt:  "2026-05-19T00:00:00Z",
		Change:      "extra",
		Expected:    expected,
		Observed:    observed,
		ReviewState: "needs_review",
		Evidence:    []string{"target path is not present in the selected manifest"},
	}
}

func writePublishedSession(t *testing.T, target string, sessionID string, profileID string, targetID string, rootID string) {
	t.Helper()
	manifestPath, err := control.Path(target, control.ArtifactManifest, sessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest %q) error = %v, want nil", sessionID, err)
	}
	if err := control.WriteFile(manifestPath, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		RootID:    rootID,
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			validManifestEntry(),
		},
	}); err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", manifestPath, err)
	}
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt %q) error = %v, want nil", sessionID, err)
	}
	if err := control.WriteFile(receiptPath, control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: profileID,
		TargetID:  targetID,
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    "published",
	}); err != nil {
		t.Fatalf("control.WriteFile(%q, receipt) error = %v, want nil", receiptPath, err)
	}
}

func validManifestEntry() control.ManifestEntry {
	entry := control.ManifestEntry{
		Path:       "file.txt",
		TargetPath: "file.txt",
		Kind:       "file",
		Digest:     digest([]byte("aaaaaaa")),
		ModTime:    "2026-05-18T00:00:00Z",
	}
	entry.SetSizeEvidence(7)
	entry.SetModeEvidence(0o644)
	return entry
}

func writePublishedReceipt(t *testing.T, target string, sessionID string, profileID string, targetID string) {
	t.Helper()
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt %q) error = %v, want nil", sessionID, err)
	}
	if err := control.WriteFile(receiptPath, control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: profileID,
		TargetID:  targetID,
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    "published",
	}); err != nil {
		t.Fatalf("control.WriteFile(%q, receipt) error = %v, want nil", receiptPath, err)
	}
}

func writeTargetDrift(t *testing.T, target string, drift control.TargetDrift) {
	t.Helper()
	path := targetDriftPath(t, target, drift.ID)
	if err := control.WriteFile(path, drift); err != nil {
		t.Fatalf("control.WriteFile(%q) error = %v, want nil", path, err)
	}
}

func writeTargetFile(t *testing.T, target string, rel string, data []byte, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(target, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
	modTime, err := time.Parse(time.RFC3339Nano, "2026-05-18T00:00:00Z")
	if err != nil {
		t.Fatalf("time.Parse(modtime) error = %v, want nil", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", path, err)
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func targetDriftPath(t *testing.T, target string, id string) string {
	t.Helper()
	path, err := control.Path(target, control.ArtifactTargetDrift, id)
	if err != nil {
		t.Fatalf("control.Path(target drift) error = %v, want nil", err)
	}
	return path
}
