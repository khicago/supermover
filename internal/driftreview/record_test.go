package driftreview

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/verify"
)

func TestRecordPersistsLiveDetectorDrift(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	writePublishedSession(t, target, "session-one", p.ProfileID, p.Target.TargetID, "root")
	if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("operator edit"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(target file) error = %v, want nil", err)
	}

	now := time.Date(2026, 5, 20, 1, 2, 3, 0, time.UTC)
	got, err := Record(RecordOptions{Profile: p, Now: now})
	if err != nil {
		t.Fatalf("Record() error = %v, want nil", err)
	}
	if got.Detected != 1 || got.Recorded != 1 || got.Existing != 0 || len(got.Records) != 1 {
		t.Fatalf("Record() = %+v, want one newly recorded drift", got)
	}
	record := got.Records[0]
	if !strings.HasPrefix(record.ID, "detected_") || record.Path != "file.txt" || record.Change != "content_mismatch" || record.SessionID != "session-one" || record.ReviewState != "needs_review" || !record.Recorded || record.Existing {
		t.Fatalf("Record().Records[0] = %+v, want recorded content mismatch", record)
	}

	path := targetDriftPath(t, target, record.ID)
	persisted, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	if persisted.ID != record.ID || persisted.Path != "file.txt" || persisted.Change != "content_mismatch" || persisted.DetectedAt != now.Format(time.RFC3339Nano) || persisted.ReviewState != "needs_review" {
		t.Fatalf("persisted drift = %+v, want live detector evidence with needs_review", persisted)
	}
	if persisted.Expected.SessionID != "session-one" || persisted.Expected.ManifestID != "manifest-session-one" || persisted.Expected.Path != "file.txt" {
		t.Fatalf("persisted expected evidence = %+v, want selected published manifest evidence", persisted.Expected)
	}
	if persisted.Observed.Path != "file.txt" || persisted.Observed.Kind != "file" || persisted.Observed.Digest == "" {
		t.Fatalf("persisted observed evidence = %+v, want target file evidence", persisted.Observed)
	}

	verifyReport, err := verify.BuildReport(verify.Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID})
	if err != nil {
		t.Fatalf("verify.BuildReport(%q) error = %v, want nil", target, err)
	}
	if verifyReport.Summary.TargetDrifts != 1 || len(verifyReport.TargetDrifts) != 1 || verifyReport.TargetDrifts[0].ID != record.ID {
		t.Fatalf("verify.BuildReport target drifts = %+v/%#v, want persisted record", verifyReport.Summary, verifyReport.TargetDrifts)
	}
}

func TestRecordIsIdempotentAndPreservesReviewMetadata(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	writePublishedSession(t, target, "session-one", p.ProfileID, p.Target.TargetID, "root")
	if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("operator edit"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(target file) error = %v, want nil", err)
	}

	first, err := Record(RecordOptions{
		Profile: p,
		Now:     time.Date(2026, 5, 20, 1, 2, 3, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Record(first) error = %v, want nil", err)
	}
	if first.Recorded != 1 || len(first.Records) != 1 {
		t.Fatalf("Record(first) = %+v, want one recorded drift", first)
	}
	id := first.Records[0].ID
	path := targetDriftPath(t, target, id)
	persisted, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(first drift) error = %v, want nil", err)
	}
	persisted.ReviewState = "acknowledged"
	persisted.ReviewAction = "acknowledge"
	persisted.ReviewedAt = "2026-05-20T02:00:00Z"
	persisted.ReviewedBy = "ops"
	persisted.ReviewReason = "operator accepted target-local edit"
	if err := control.WriteFile(path, persisted); err != nil {
		t.Fatalf("control.WriteFile(acknowledged drift) error = %v, want nil", err)
	}

	second, err := Record(RecordOptions{
		Profile: p,
		Now:     time.Date(2026, 5, 20, 3, 4, 5, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Record(second) error = %v, want nil", err)
	}
	if second.Recorded != 0 || second.Existing != 1 || len(second.Records) != 1 || second.Records[0].ID != id || second.Records[0].ReviewState != "acknowledged" {
		t.Fatalf("Record(second) = %+v, want existing acknowledged record", second)
	}
	again, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(second drift) error = %v, want nil", err)
	}
	if again.ReviewState != "acknowledged" || again.ReviewAction != "acknowledge" || again.ReviewedAt != "2026-05-20T02:00:00Z" || again.ReviewedBy != "ops" || again.ReviewReason != "operator accepted target-local edit" {
		t.Fatalf("persisted drift after second record = %+v, want review metadata preserved", again)
	}
	if again.DetectedAt != persisted.DetectedAt || !reflect.DeepEqual(again.Expected, persisted.Expected) || !reflect.DeepEqual(again.Observed, persisted.Observed) || !reflect.DeepEqual(again.Evidence, persisted.Evidence) {
		t.Fatalf("persisted drift after second record = %+v, want existing detector evidence preserved", again)
	}

	live, err := verify.DetectTargetDrift(verify.DriftOptions{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID})
	if err != nil {
		t.Fatalf("verify.DetectTargetDrift(%q) error = %v, want nil", target, err)
	}
	if live.Summary.TargetDrifts != 1 || len(live.Drifts) != 1 || live.Drifts[0].ID != id {
		t.Fatalf("live detector after acknowledge = %+v drifts=%#v, want unresolved target divergence still visible", live.Summary, live.Drifts)
	}
	if live.Drifts[0].ReviewState != "needs_review" || live.Drifts[0].Path != "file.txt" || live.Drifts[0].Change != "content_mismatch" {
		t.Fatalf("live detector drift after acknowledge = %+v, want fresh unreviewed live evidence", live.Drifts[0])
	}
}

func TestRecordReopensResolvedDriftWhenSameFindingReturns(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	writePublishedSession(t, target, "session-one", p.ProfileID, p.Target.TargetID, "root")
	if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("operator edit"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(target file) error = %v, want nil", err)
	}

	first, err := Record(RecordOptions{
		Profile: p,
		Now:     time.Date(2026, 5, 20, 1, 2, 3, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Record(first) error = %v, want nil", err)
	}
	if first.Recorded != 1 || len(first.Records) != 1 {
		t.Fatalf("Record(first) = %+v, want one recorded drift", first)
	}
	id := first.Records[0].ID
	path := targetDriftPath(t, target, id)
	persisted, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(first drift) error = %v, want nil", err)
	}
	persisted.ReviewState = "resolved"
	persisted.ReviewAction = "resolve"
	persisted.ReviewedAt = "2026-05-20T02:00:00Z"
	persisted.ReviewedBy = "ops"
	persisted.ReviewReason = "target restored"
	if err := control.WriteFile(path, persisted); err != nil {
		t.Fatalf("control.WriteFile(resolved drift) error = %v, want nil", err)
	}

	second, err := Record(RecordOptions{
		Profile: p,
		Now:     time.Date(2026, 5, 20, 3, 4, 5, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Record(second) error = %v, want nil", err)
	}
	if second.Recorded != 0 || second.Existing != 0 || second.Reopened != 1 || len(second.Records) != 1 {
		t.Fatalf("Record(second) = %+v, want one reopened drift", second)
	}
	if second.Records[0].ID != id || second.Records[0].ReviewState != "needs_review" || !second.Records[0].Reopened || second.Records[0].Recorded || second.Records[0].Existing {
		t.Fatalf("Record(second).Records[0] = %+v, want reopened needs_review record", second.Records[0])
	}
	again, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(reopened drift) error = %v, want nil", err)
	}
	if again.ReviewState != "needs_review" || again.ReviewAction != "" || again.ReviewedAt != "" || again.ReviewedBy != "" || again.ReviewReason != "" {
		t.Fatalf("persisted reopened drift = %+v, want current review reset to needs_review", again)
	}
	if again.DetectedAt != persisted.DetectedAt || again.LastDetectedAt != "2026-05-20T03:04:05Z" {
		t.Fatalf("persisted reopened detection times = detected_at %q last %q, want original and latest", again.DetectedAt, again.LastDetectedAt)
	}
	if len(again.ReviewHistory) != 1 {
		t.Fatalf("persisted reopened history = %+v, want prior resolve evidence", again.ReviewHistory)
	}
	event := again.ReviewHistory[0]
	if event.ReviewState != "resolved" || event.ReviewAction != "resolve" || event.ReviewedAt != "2026-05-20T02:00:00Z" || event.ReviewedBy != "ops" || event.ReviewReason != "target restored" || event.ReconciledAt != "2026-05-20T03:04:05Z" || event.ReconcileAction != "reopen" {
		t.Fatalf("persisted reopened history event = %+v, want prior resolve evidence plus reopen action", event)
	}

	verifyReport, err := verify.BuildReport(verify.Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID})
	if err != nil {
		t.Fatalf("verify.BuildReport(%q) error = %v, want nil", target, err)
	}
	if verifyReport.Summary.TargetDrifts != 1 || len(verifyReport.TargetDrifts) != 1 || verifyReport.TargetDrifts[0].ID != id {
		t.Fatalf("verify.BuildReport target drifts = %+v/%#v, want reopened drift review-required", verifyReport.Summary, verifyReport.TargetDrifts)
	}
}

func TestRecordRejectsExistingCollisionWithMismatchedEvidence(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*control.TargetDrift)
		wantErr string
	}{
		{
			name: "profile scope",
			mutate: func(drift *control.TargetDrift) {
				drift.ProfileID = "other-profile"
			},
			wantErr: "does not match detected scope",
		},
		{
			name: "target scope",
			mutate: func(drift *control.TargetDrift) {
				drift.TargetID = "other-target"
			},
			wantErr: "does not match detected scope",
		},
		{
			name: "root scope",
			mutate: func(drift *control.TargetDrift) {
				drift.RootID = "other-root"
			},
			wantErr: "does not match detected scope",
		},
		{
			name: "session",
			mutate: func(drift *control.TargetDrift) {
				drift.SessionID = "session-two"
			},
			wantErr: "session_id",
		},
		{
			name: "path",
			mutate: func(drift *control.TargetDrift) {
				drift.Path = "other.txt"
			},
			wantErr: "path/change",
		},
		{
			name: "change",
			mutate: func(drift *control.TargetDrift) {
				drift.Change = "metadata_mismatch"
			},
			wantErr: "path/change",
		},
		{
			name: "expected",
			mutate: func(drift *control.TargetDrift) {
				drift.Expected.Digest = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
			},
			wantErr: "expected evidence does not match current detection",
		},
		{
			name: "observed",
			mutate: func(drift *control.TargetDrift) {
				drift.Observed.Digest = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
			},
			wantErr: "observed evidence does not match current detection",
		},
		{
			name: "evidence",
			mutate: func(drift *control.TargetDrift) {
				drift.Evidence = append(append([]string(nil), drift.Evidence...), "tampered evidence")
			},
			wantErr: "evidence strings do not match current detection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "target")
			p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
			writePublishedSession(t, target, "session-one", p.ProfileID, p.Target.TargetID, "root")
			if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("operator edit"), 0o644); err != nil {
				t.Fatalf("os.WriteFile(target file) error = %v, want nil", err)
			}

			first, err := Record(RecordOptions{Profile: p})
			if err != nil {
				t.Fatalf("Record(first) error = %v, want nil", err)
			}
			if len(first.Records) != 1 {
				t.Fatalf("Record(first) = %+v, want one recorded drift", first)
			}
			id := first.Records[0].ID
			path := targetDriftPath(t, target, id)
			persisted, err := control.ReadFile[control.TargetDrift](path)
			if err != nil {
				t.Fatalf("control.ReadFile(first drift) error = %v, want nil", err)
			}
			tt.mutate(&persisted)
			if err := control.WriteFile(path, persisted); err != nil {
				t.Fatalf("control.WriteFile(tampered drift) error = %v, want nil", err)
			}

			_, err = Record(RecordOptions{Profile: p})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Record(tampered existing drift) error = %v, want %q", err, tt.wantErr)
			}
			again, readErr := control.ReadFile[control.TargetDrift](path)
			if readErr != nil {
				t.Fatalf("control.ReadFile(tampered drift) error = %v, want nil", readErr)
			}
			if !reflect.DeepEqual(again, persisted) {
				t.Fatalf("persisted collision drift = %+v, want no overwrite of mismatched existing artifact %+v", again, persisted)
			}
		})
	}
}

func TestRecordPreflightsExistingCollisionsBeforeWritingNewDrifts(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	writePublishedSession(t, target, "session-one", p.ProfileID, p.Target.TargetID, "root")
	if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("operator edit"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(target file) error = %v, want nil", err)
	}

	first, err := Record(RecordOptions{Profile: p})
	if err != nil {
		t.Fatalf("Record(first) error = %v, want nil", err)
	}
	if len(first.Records) != 1 {
		t.Fatalf("Record(first) = %+v, want one recorded drift", first)
	}
	id := first.Records[0].ID
	path := targetDriftPath(t, target, id)
	persisted, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(first drift) error = %v, want nil", err)
	}
	persisted.Evidence = append(append([]string(nil), persisted.Evidence...), "tampered evidence")
	if err := control.WriteFile(path, persisted); err != nil {
		t.Fatalf("control.WriteFile(tampered drift) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(target, "aaa-extra.txt"), []byte("extra"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(extra target file) error = %v, want nil", err)
	}
	live, err := verify.DetectTargetDrift(verify.DriftOptions{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID})
	if err != nil {
		t.Fatalf("verify.DetectTargetDrift(%q) error = %v, want nil", target, err)
	}
	var extraID string
	for _, drift := range live.Drifts {
		if drift.Path == "aaa-extra.txt" && drift.Change == "extra" {
			extraID = drift.ID
			break
		}
	}
	if extraID == "" {
		t.Fatalf("live detector drifts=%#v, want new extra drift before collision check", live.Drifts)
	}

	_, err = Record(RecordOptions{Profile: p})
	if err == nil || !strings.Contains(err.Error(), "evidence strings do not match current detection") {
		t.Fatalf("Record(tampered existing plus new drift) error = %v, want existing-collision refusal", err)
	}
	if _, statErr := os.Stat(targetDriftPath(t, target, extraID)); !os.IsNotExist(statErr) {
		t.Fatalf("Stat(new extra drift artifact) error = %v, want no partial write before existing collision is resolved", statErr)
	}
	again, readErr := control.ReadFile[control.TargetDrift](path)
	if readErr != nil {
		t.Fatalf("control.ReadFile(tampered drift) error = %v, want nil", readErr)
	}
	if !reflect.DeepEqual(again, persisted) {
		t.Fatalf("persisted collision drift = %+v, want no overwrite of mismatched existing artifact %+v", again, persisted)
	}
}

func TestRecordPersistsExtraDriftAsReviewableRecord(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	writePublishedSession(t, target, "session-one", p.ProfileID, p.Target.TargetID, "root")
	if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("payload!"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(target file) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(target, "extra.txt"), []byte("extra"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(extra file) error = %v, want nil", err)
	}

	got, err := Record(RecordOptions{Profile: p, Now: time.Date(2026, 5, 20, 1, 2, 3, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Record() error = %v, want nil", err)
	}
	if got.Detected != 2 || got.Recorded != 2 || len(got.Records) != 2 {
		t.Fatalf("Record() = %+v, want persisted file mismatch and extra drift", got)
	}
	var extraID string
	for _, record := range got.Records {
		if record.Path == "extra.txt" && record.Change == "extra" {
			extraID = record.ID
		}
	}
	if extraID == "" {
		t.Fatalf("Record() = %+v, want persisted extra drift", got)
	}
	persisted, err := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, extraID))
	if err != nil {
		t.Fatalf("control.ReadFile(extra drift) error = %v, want nil", err)
	}
	if persisted.Expected.SessionID != "session-one" || persisted.Expected.ManifestID != "manifest-session-one" || persisted.Expected.Kind != "missing" || persisted.Expected.Path != "extra.txt" {
		t.Fatalf("persisted extra expected = %+v, want selected manifest missing-state baseline", persisted.Expected)
	}
	if persisted.Observed.Path != "extra.txt" || persisted.Observed.Kind != "file" || persisted.Observed.Digest == "" {
		t.Fatalf("persisted extra observed = %+v, want target file evidence", persisted.Observed)
	}
	if len(persisted.Evidence) == 0 {
		t.Fatalf("persisted extra evidence = %+v, want durable live detector evidence", persisted.Evidence)
	}

	ack, err := Acknowledge(AcknowledgeOptions{
		Profile:  p,
		ID:       persisted.ID,
		Reason:   "reviewed target-local extra path",
		Reviewer: "ops",
		Now:      time.Date(2026, 5, 20, 2, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Acknowledge(extra persisted drift) error = %v, want nil", err)
	}
	if ack.ReviewState != "acknowledged" || ack.ID != persisted.ID {
		t.Fatalf("Acknowledge(extra persisted drift) = %+v, want acknowledged review metadata", ack)
	}
	afterAck, err := control.ReadFile[control.TargetDrift](targetDriftPath(t, target, extraID))
	if err != nil {
		t.Fatalf("control.ReadFile(acknowledged extra drift) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(afterAck.Expected, persisted.Expected) || !reflect.DeepEqual(afterAck.Observed, persisted.Observed) || !reflect.DeepEqual(afterAck.Evidence, persisted.Evidence) {
		t.Fatalf("acknowledged extra drift = %+v, want expected/observed/evidence preserved from %+v", afterAck, persisted)
	}
	if afterAck.ReviewState != "acknowledged" || afterAck.ReviewAction != "acknowledge" || afterAck.ReviewedAt != "2026-05-20T02:00:00Z" || afterAck.ReviewedBy != "ops" || afterAck.ReviewReason != "reviewed target-local extra path" {
		t.Fatalf("acknowledged extra drift = %+v, want acknowledgement metadata", afterAck)
	}

	live, err := verify.DetectTargetDrift(verify.DriftOptions{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID})
	if err != nil {
		t.Fatalf("verify.DetectTargetDrift(%q) error = %v, want nil", target, err)
	}
	var liveExtra *control.TargetDrift
	for i := range live.Drifts {
		if live.Drifts[i].Path == "extra.txt" && live.Drifts[i].Change == "extra" {
			liveExtra = &live.Drifts[i]
			break
		}
	}
	if liveExtra == nil {
		t.Fatalf("live detector after acknowledge = %+v drifts=%#v, want extra path still reported", live.Summary, live.Drifts)
	}
	if liveExtra.ID != persisted.ID || liveExtra.ReviewState != "needs_review" {
		t.Fatalf("live detector extra drift after acknowledge = %+v, want same extra path as fresh needs_review evidence", *liveExtra)
	}
	if !reflect.DeepEqual(liveExtra.Expected, persisted.Expected) || !reflect.DeepEqual(liveExtra.Observed, persisted.Observed) || !reflect.DeepEqual(liveExtra.Evidence, persisted.Evidence) {
		t.Fatalf("live detector extra drift after acknowledge = %+v, want same live evidence as persisted extra drift %+v", *liveExtra, persisted)
	}
}

func TestRecordRefusesUnsafeControlPlaneWithoutOutsideWrite(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	outside := filepath.Join(dir, "outside")
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), target)
	writePublishedSession(t, target, "session-one", p.ProfileID, p.Target.TargetID, "root")
	if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("operator edit"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(target file) error = %v, want nil", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(outside) error = %v, want nil", err)
	}
	driftDir := filepath.Join(target, control.DirName, "drift")
	if err := os.Symlink(outside, driftDir); err != nil {
		t.Skipf("os.Symlink(drift dir) unavailable: %v", err)
	}

	_, err := Record(RecordOptions{Profile: p})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Record(symlink drift dir) error = %v, want symlink refusal", err)
	}
	entries, readErr := os.ReadDir(outside)
	if readErr != nil {
		t.Fatalf("os.ReadDir(outside) error = %v, want nil", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("outside drift target entries = %#v, want no writes through symlink", entries)
	}
}
