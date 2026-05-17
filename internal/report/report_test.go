package report

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/transaction"
)

func TestBuildReportEmptyTarget(t *testing.T) {
	target := t.TempDir()

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Scope != ScopeLocalMigrationTarget {
		t.Fatalf("BuildReport(%q).Scope = %q, want %q", target, got.Scope, ScopeLocalMigrationTarget)
	}
	if got.Overall.Status != StatusEmpty {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusEmpty)
	}
	if got.LatestSession.Completeness.Status != CompletenessNoPublishedSession {
		t.Fatalf("BuildReport(%q).LatestSession.Completeness.Status = %q, want %q", target, got.LatestSession.Completeness.Status, CompletenessNoPublishedSession)
	}
	if got.Summary.ManifestCount != 0 || got.Summary.ArtifactProblems != 0 || !got.Health.Healthy {
		t.Fatalf("BuildReport(%q) summary=%+v health=%+v, want empty healthy target", target, got.Summary, got.Health)
	}
}

func TestBuildReportPublishedSessionComplete(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "docs/a.txt", []byte("hello"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session-success",
		SessionID: "session-success",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{{
			Path:       "docs/a.txt",
			Kind:       "file",
			Size:       5,
			Digest:     digest([]byte("hello")),
			TargetPath: "docs/a.txt",
		}},
	})
	writePublishedReceipt(t, target, "session-success")
	writeProfileSnapshot(t, target, "session-success")
	writeSessionRecord(t, target, "session-success", transaction.StatePublished)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusVerified {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q; report=%+v", target, got.Overall.Status, StatusVerified, got)
	}
	if got.LatestSession.ID != "session-success" || got.LatestSession.Completeness.Status != CompletenessVerified {
		t.Fatalf("BuildReport(%q).LatestSession = %+v, want verified session-success", target, got.LatestSession)
	}
	if got.Summary.FilesExpected != 1 || got.Summary.FilesVerified != 1 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want one verified file", target, got.Summary)
	}
	if len(got.ArtifactProblems) != 0 || len(got.VerificationFindings) != 0 {
		t.Fatalf("BuildReport(%q) artifact_problems=%#v findings=%#v, want none", target, got.ArtifactProblems, got.VerificationFindings)
	}
	if len(got.ProfileSnapshots) != 1 || got.ProfileSnapshots[0].ID != "profile-session-success" {
		t.Fatalf("BuildReport(%q).ProfileSnapshots = %#v, want session profile snapshot", target, got.ProfileSnapshots)
	}
}

func TestBuildReportFlagsMissingProfileSnapshot(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-missing-snapshot", "docs/a.txt", []byte("hello"))
	path, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-session-missing-snapshot")
	if err != nil {
		t.Fatalf("control.Path(profile snapshot) error = %v, want nil", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("os.Remove(%q) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	if len(got.ProfileSnapshots) != 0 {
		t.Fatalf("BuildReport(%q).ProfileSnapshots = %#v, want no valid snapshots", target, got.ProfileSnapshots)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "profile_snapshot" {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want profile snapshot problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportFlagsCorruptProfileSnapshot(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-corrupt-snapshot", "docs/a.txt", []byte("hello"))
	path, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-session-corrupt-snapshot")
	if err != nil {
		t.Fatalf("control.Path(profile snapshot) error = %v, want nil", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy || len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "profile_snapshot" {
		t.Fatalf("BuildReport(%q) status=%q artifact_problems=%#v, want corrupt profile snapshot problem", target, got.Overall.Status, got.ArtifactProblems)
	}
}

func TestBuildReportFlagsEmbeddedProfileSnapshotTargetMismatch(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-wrong-target", "docs/a.txt", []byte("hello"))
	path, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-session-wrong-target")
	if err != nil {
		t.Fatalf("control.Path(profile snapshot) error = %v, want nil", err)
	}
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-session-wrong-target",
		ProfileID:  "profile-local",
		SessionID:  "session-wrong-target",
		CapturedAt: "2026-05-16T00:00:00Z",
		Profile:    []byte(`{"profile_id":"profile-local","roots":[{"id":"root"}],"target":{"target_id":"target-other"}}`),
	}
	if err := control.WriteFile(path, snapshot); err != nil {
		t.Fatalf("control.WriteFile(%q, snapshot) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy || len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "profile_snapshot" {
		t.Fatalf("BuildReport(%q) status=%q artifact_problems=%#v, want embedded snapshot target problem", target, got.Overall.Status, got.ArtifactProblems)
	}
	if !strings.Contains(got.ArtifactProblems[0].Error, "embedded target_id") {
		t.Fatalf("BuildReport(%q).ArtifactProblems[0].Error = %q, want embedded target_id mismatch", target, got.ArtifactProblems[0].Error)
	}
}

func TestBuildReportFlagsForeignPublishedReceipts(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "docs/a.txt", []byte("hello"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-foreign",
		SessionID: "session-foreign",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{{
			Path:       "docs/a.txt",
			Kind:       "file",
			Size:       5,
			Digest:     digest([]byte("hello")),
			TargetPath: "docs/a.txt",
		}},
	})
	writePublishedReceiptForScope(t, target, "session-foreign", "profile-other", "target-other")

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q, profile-local) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q, profile-local).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	if got.Summary.ManifestCount != 0 {
		t.Fatalf("BuildReport(%q, profile-local).Summary.ManifestCount = %d, want 0 matching manifests", target, got.Summary.ManifestCount)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "scope" {
		t.Fatalf("BuildReport(%q, profile-local).ArtifactProblems = %#v, want foreign scope problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportWarningSuggestions(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-warning", "docs/a.txt", []byte("hello"))
	writeWarning(t, target, control.Warning{
		Version:    control.CurrentVersion,
		ID:         "warning-suggestion",
		SessionID:  "session-warning",
		Code:       "special_file",
		Message:    "path needs additional migration config",
		Severity:   "warning",
		Paths:      []string{"docs/socket"},
		TargetPath: "docs/socket",
		SuggestedProfilePatch: map[string]string{
			"include_special_files": "true",
		},
		SuggestedConfig: map[string]string{
			"special_files": "manual_review",
		},
		CreatedAt: "2026-05-16T00:02:00Z",
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusAttention {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusAttention)
	}
	if len(got.Warnings) != 1 || got.Warnings[0].SuggestedProfilePatch["include_special_files"] != "true" {
		t.Fatalf("BuildReport(%q).Warnings = %#v, want warning with suggested profile patch", target, got.Warnings)
	}
	if len(got.ProfileSuggestions) != 1 {
		t.Fatalf("BuildReport(%q).ProfileSuggestions length = %d, want 1", target, len(got.ProfileSuggestions))
	}
	suggestion := got.ProfileSuggestions[0]
	if suggestion.WarningID != "warning-suggestion" || suggestion.Code != "special_file" || suggestion.SuggestedConfig["special_files"] != "manual_review" {
		t.Fatalf("BuildReport(%q).ProfileSuggestions[0] = %+v, want filterable warning suggestion", target, suggestion)
	}
}

func TestBuildReportSoftDeletes(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-soft-delete", "keep.txt", []byte("keep"))
	writeSoftDelete(t, target, control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "session-soft-delete-del_001",
		SessionID:          "session-soft-delete",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "session-previous",
		PreviousManifestID: "manifest-previous",
		SourcePath:         "gone.txt",
		TargetPath:         "gone.txt",
		Kind:               "file",
		DetectedAt:         "2026-05-16T00:03:00Z",
		Reason:             "missing_from_latest_source_scan",
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusAttention {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusAttention)
	}
	if got.Summary.SoftDeletes != 1 || len(got.SoftDeletes) != 1 || got.SoftDeletes[0].TargetPath != "gone.txt" {
		t.Fatalf("BuildReport(%q).SoftDeletes = %#v summary=%+v, want inspectable soft delete", target, got.SoftDeletes, got.Summary)
	}
}

func TestBuildReportHealthIssuesAndDamagedArtifact(t *testing.T) {
	target := t.TempDir()
	writeSessionRecord(t, target, "session-recover", transaction.StateStaged)
	warningPath := filepath.Join(control.ControlDir(target), "warnings", "bad.json")
	if err := os.MkdirAll(filepath.Dir(warningPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(warningPath), err)
	}
	if err := os.WriteFile(warningPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", warningPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	if got.Health.Healthy || len(got.Health.RecoveryIssues) != 1 {
		t.Fatalf("BuildReport(%q).Health = %+v, want one recovery issue", target, got.Health)
	}
	if len(got.ArtifactProblems) != 1 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want damaged artifact problem", target, got.ArtifactProblems)
	}
	if got.ArtifactProblems[0].Path != filepath.ToSlash(warningPath) {
		t.Fatalf("BuildReport(%q).ArtifactProblems[0].Path = %q, want %q", target, got.ArtifactProblems[0].Path, filepath.ToSlash(warningPath))
	}
	if len(got.Health.ArtifactIssues) != 1 {
		t.Fatalf("BuildReport(%q).Health.ArtifactIssues = %#v, want damaged artifact in health", target, got.Health.ArtifactIssues)
	}
}

func TestBuildReportShowsReceiptSessionMismatch(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-crash", "docs/a.txt", []byte("hello"))
	writeSessionRecord(t, target, "session-crash", transaction.StateStaged)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	if got.Summary.RecoveryIssues != 1 || got.Summary.ArtifactProblems == 0 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want recovery and artifact issue", target, got.Summary)
	}
	found := false
	for _, problem := range got.ArtifactProblems {
		if problem.Source == "health" && problem.SessionID == "session-crash" && strings.Contains(problem.Error, "receipt status") {
			found = true
		}
	}
	if !found {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want receipt/session mismatch", target, got.ArtifactProblems)
	}
}

func TestBuildReportShowsPartialControlArtifact(t *testing.T) {
	target := t.TempDir()
	writeSessionRecord(t, target, "session-partial", transaction.StateReceived)
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session-partial",
		SessionID: "session-partial",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	found := false
	for _, problem := range got.ArtifactProblems {
		if problem.Source == "health" && problem.SessionID == "session-partial" && strings.Contains(problem.Error, "non-staged session") {
			found = true
		}
	}
	if !found {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want partial control artifact", target, got.ArtifactProblems)
	}
}

func TestBuildReportDeduplicatesManifestArtifactProblems(t *testing.T) {
	target := t.TempDir()
	writePublishedReceipt(t, target, "session-missing-manifest")
	writeSessionRecord(t, target, "session-missing-manifest", transaction.StatePublished)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	if len(got.ArtifactProblems) != 1 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want one deduplicated manifest problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportSessionFilterIgnoresOtherSessionArtifactProblem(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-selected", "docs/a.txt", []byte("hello"))
	writePublishedReceipt(t, target, "session-other")
	writeSessionRecord(t, target, "session-other", transaction.StatePublished)
	otherManifestPath, err := control.Path(target, control.ArtifactManifest, "session-other")
	if err != nil {
		t.Fatalf("control.Path(other manifest) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Dir(otherManifestPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(otherManifestPath), err)
	}
	if err := os.WriteFile(otherManifestPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", otherManifestPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local", SessionID: "session-selected"})
	if err != nil {
		t.Fatalf("BuildReport(%q, selected) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusVerified {
		t.Fatalf("BuildReport(%q, selected).Overall.Status = %q, artifact_problems=%#v health=%+v, want %q", target, got.Overall.Status, got.ArtifactProblems, got.Health, StatusVerified)
	}
	if len(got.ArtifactProblems) != 0 || len(got.Health.ArtifactIssues) != 0 {
		t.Fatalf("BuildReport(%q, selected) artifact_problems=%#v health_artifacts=%#v, want unrelated artifact filtered", target, got.ArtifactProblems, got.Health.ArtifactIssues)
	}
}

func TestBuildReportSessionFilterKeepsUnscopedArtifactProblem(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-selected", "docs/a.txt", []byte("hello"))
	badPath := filepath.Join(control.ControlDir(target), "warnings", "bad-global.json")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(badPath), err)
	}
	if err := os.WriteFile(badPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", badPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local", SessionID: "session-selected"})
	if err != nil {
		t.Fatalf("BuildReport(%q, selected) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q, selected).Overall.Status = %q, artifact_problems=%#v, want %q", target, got.Overall.Status, got.ArtifactProblems, StatusUnhealthy)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].SessionID != "" || got.ArtifactProblems[0].Path != filepath.ToSlash(badPath) {
		t.Fatalf("BuildReport(%q, selected).ArtifactProblems = %#v, want unscoped bad-global problem", target, got.ArtifactProblems)
	}
	if len(got.Health.ArtifactIssues) != 1 || got.Health.ArtifactIssues[0].SessionID != "" {
		t.Fatalf("BuildReport(%q, selected).Health.ArtifactIssues = %#v, want unscoped health artifact retained", target, got.Health.ArtifactIssues)
	}
}

func TestBuildReportAggregateIncludesOlderSessionArtifactProblem(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-new", "docs/new.txt", []byte("new"))
	writePublishedReceipt(t, target, "session-old")
	writeSessionRecord(t, target, "session-old", transaction.StatePublished)
	oldManifestPath, err := control.Path(target, control.ArtifactManifest, "session-old")
	if err != nil {
		t.Fatalf("control.Path(old manifest) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Dir(oldManifestPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(oldManifestPath), err)
	}
	if err := os.WriteFile(oldManifestPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", oldManifestPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.SessionID != "session-new" {
		t.Fatalf("BuildReport(%q).SessionID = %q, want latest session-new", target, got.SessionID)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, artifact_problems=%#v health=%+v, want %q", target, got.Overall.Status, got.ArtifactProblems, got.Health, StatusUnhealthy)
	}
	if len(got.ArtifactProblems) == 0 || len(got.Health.ArtifactIssues) == 0 {
		t.Fatalf("BuildReport(%q) artifact_problems=%#v health_artifacts=%#v, want old session artifact retained in aggregate", target, got.ArtifactProblems, got.Health.ArtifactIssues)
	}
	if got.ArtifactProblems[0].SessionID != "session-old" {
		t.Fatalf("BuildReport(%q).ArtifactProblems[0].SessionID = %q, want session-old", target, got.ArtifactProblems[0].SessionID)
	}
}

func TestBuildReportAggregateIncludesOlderSessionProfileSnapshotProblem(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-old", "docs/old.txt", []byte("old"))
	writeCompleteSession(t, target, "session-new", "docs/new.txt", []byte("new"))
	rewriteManifestCreatedAt(t, target, "session-old", "2026-05-15T00:00:00Z")
	rewriteManifestCreatedAt(t, target, "session-new", "2026-05-16T00:00:00Z")
	path, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-session-old")
	if err != nil {
		t.Fatalf("control.Path(old profile snapshot) error = %v, want nil", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("os.Remove(%q) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.SessionID != "session-new" {
		t.Fatalf("BuildReport(%q).SessionID = %q, want latest session-new", target, got.SessionID)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, artifact_problems=%#v, want %q", target, got.Overall.Status, got.ArtifactProblems, StatusUnhealthy)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "profile_snapshot" || got.ArtifactProblems[0].SessionID != "session-old" {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want old profile snapshot problem", target, got.ArtifactProblems)
	}
	if len(got.ProfileSnapshots) != 1 || got.ProfileSnapshots[0].SessionID != "session-new" {
		t.Fatalf("BuildReport(%q).ProfileSnapshots = %#v, want valid latest profile snapshot still visible", target, got.ProfileSnapshots)
	}
}

func TestBuildReportSessionFilterIgnoresOtherSessionRecoveryIssue(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-selected", "docs/a.txt", []byte("hello"))
	writeSessionRecord(t, target, "session-other", transaction.StateStaged)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local", SessionID: "session-selected"})
	if err != nil {
		t.Fatalf("BuildReport(%q, selected) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusVerified {
		t.Fatalf("BuildReport(%q, selected).Overall.Status = %q, want %q; health=%+v", target, got.Overall.Status, StatusVerified, got.Health)
	}
	if got.Summary.RecoveryIssues != 0 || len(got.Health.RecoveryIssues) != 0 {
		t.Fatalf("BuildReport(%q, selected).Health = %+v, want unrelated recovery issue filtered", target, got.Health)
	}
}

func TestBuildReportSessionFilterIgnoresOtherSessionInvalidRecord(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "sessions", "docs/a.txt", []byte("hello"))
	badPath := filepath.Join(control.ControlDir(target), "sessions", "bad", "session.json")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(badPath), err)
	}
	if err := os.WriteFile(badPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", badPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local", SessionID: "sessions"})
	if err != nil {
		t.Fatalf("BuildReport(%q, sessions) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusVerified {
		t.Fatalf("BuildReport(%q, sessions).Overall.Status = %q, health=%+v, want %q", target, got.Overall.Status, got.Health, StatusVerified)
	}
	if len(got.Health.InvalidRecords) != 0 || got.Summary.InvalidHealthRecords != 0 {
		t.Fatalf("BuildReport(%q, sessions).InvalidRecords = %#v, want unrelated invalid record filtered", target, got.Health.InvalidRecords)
	}
}

func TestBuildReportMissingTargetReturnsError(t *testing.T) {
	_, err := BuildReport(Options{TargetRoot: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatalf("BuildReport(missing target) error = nil, want error")
	}
}

func writeCompleteSession(t *testing.T, target, sessionID, rel string, data []byte) {
	t.Helper()
	writeTargetFile(t, target, rel, data)
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{{
			Path:       rel,
			Kind:       "file",
			Size:       int64(len(data)),
			Digest:     digest(data),
			TargetPath: rel,
		}},
	})
	writePublishedReceipt(t, target, sessionID)
	writeProfileSnapshot(t, target, sessionID)
	writeSessionRecord(t, target, sessionID, transaction.StatePublished)
}

func rewriteManifestCreatedAt(t *testing.T, target string, sessionID string, createdAt string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactManifest, sessionID)
	if err != nil {
		t.Fatalf("control.Path(%q, manifest, %q) error = %v, want nil", target, sessionID, err)
	}
	manifest, err := control.ReadFile[control.Manifest](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q, manifest) error = %v, want nil", path, err)
	}
	manifest.CreatedAt = createdAt
	if err := control.WriteFile(path, manifest); err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", path, err)
	}
}

func writeManifest(t *testing.T, target string, manifest control.Manifest) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactManifest, manifest.SessionID)
	if err != nil {
		t.Fatalf("control.Path(%q, manifest, %q) error = %v, want nil", target, manifest.SessionID, err)
	}
	if err := control.WriteFile(path, manifest); err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", path, err)
	}
}

func writePublishedReceipt(t *testing.T, target string, sessionID string) {
	t.Helper()
	writePublishedReceiptForScope(t, target, sessionID, "profile-local", "target-local")
}

func writePublishedReceiptForScope(t *testing.T, target string, sessionID string, profileID string, targetID string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(%q, receipt, %q) error = %v, want nil", target, sessionID, err)
	}
	receipt := control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: profileID,
		TargetID:  targetID,
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    "published",
	}
	if err := control.WriteFile(path, receipt); err != nil {
		t.Fatalf("control.WriteFile(%q, receipt) error = %v, want nil", path, err)
	}
}

func writeProfileSnapshot(t *testing.T, target string, sessionID string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-"+sessionID)
	if err != nil {
		t.Fatalf("control.Path(%q, profile snapshot, %q) error = %v, want nil", target, sessionID, err)
	}
	payload := []byte(`{"profile_id":"profile-local","roots":[{"id":"root"}],"target":{"target_id":"target-local"}}`)
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + sessionID,
		ProfileID:  "profile-local",
		SessionID:  sessionID,
		CapturedAt: "2026-05-16T00:00:00Z",
		Profile:    payload,
	}
	if err := control.WriteFile(path, snapshot); err != nil {
		t.Fatalf("control.WriteFile(%q, profile snapshot) error = %v, want nil", path, err)
	}
}

func writeWarning(t *testing.T, target string, warning control.Warning) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactWarning, warning.ID)
	if err != nil {
		t.Fatalf("control.Path(%q, warning, %q) error = %v, want nil", target, warning.ID, err)
	}
	if err := control.WriteFile(path, warning); err != nil {
		t.Fatalf("control.WriteFile(%q, warning) error = %v, want nil", path, err)
	}
}

func writeSoftDelete(t *testing.T, target string, softDelete control.SoftDelete) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSoftDelete, softDelete.ID)
	if err != nil {
		t.Fatalf("control.Path(%q, soft delete, %q) error = %v, want nil", target, softDelete.ID, err)
	}
	if err := control.WriteFile(path, softDelete); err != nil {
		t.Fatalf("control.WriteFile(%q, soft delete) error = %v, want nil", path, err)
	}
}

func writeTargetFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
}

func writeSessionRecord(t *testing.T, target, sessionID string, state transaction.State) {
	t.Helper()
	now := time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC)
	record, err := transaction.NewSessionRecord(sessionID, now)
	if err != nil {
		t.Fatalf("transaction.NewSessionRecord(%q) error = %v, want nil", sessionID, err)
	}
	record, err = record.WithState(state, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("SessionRecord.WithState(%q) error = %v, want nil", state, err)
	}
	if err := transaction.NewLayout(control.ControlDir(target)).WriteSessionRecord(record); err != nil {
		t.Fatalf("Layout.WriteSessionRecord(%+v) error = %v, want nil", record, err)
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
