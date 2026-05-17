package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
)

func TestBuildReportVerifiesLatestManifest(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "ok.txt", []byte("ok"))
	writeTargetFile(t, target, "changed.txt", []byte("changed"))

	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-old",
		SessionID: "old",
		CreatedAt: "2026-05-15T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "old.txt", Kind: "file", Size: 3, Digest: digest([]byte("old")), TargetPath: "old.txt"},
		},
	})
	writePublishedReceipt(t, target, "old")
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-new",
		SessionID: "new",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "ok.txt", Kind: "file", Size: 2, Digest: digest([]byte("ok")), TargetPath: "ok.txt"},
			{Path: "missing.txt", Kind: "file", Size: 7, Digest: digest([]byte("missing")), TargetPath: "missing.txt"},
			{Path: "changed.txt", Kind: "file", Size: 8, Digest: digest([]byte("expected")), TargetPath: "changed.txt"},
			{Path: "nodigest.txt", Kind: "file", Size: 2, TargetPath: "ok.txt"},
			{Path: "dir", Kind: "dir", TargetPath: "dir"},
		},
	})
	writePublishedReceipt(t, target, "new")
	writeWarning(t, target, control.Warning{
		Version:   control.CurrentVersion,
		ID:        "warning-new",
		SessionID: "new",
		Code:      "symlink_not_copied",
		Message:   "symlink copy is not implemented",
		Severity:  "warning",
		Paths:     []string{"link.txt"},
		CreatedAt: "2026-05-16T00:00:00Z",
	})
	writeSoftDelete(t, target, control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "del-one",
		SessionID:          "new",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "old",
		PreviousManifestID: "manifest-old",
		SourcePath:         "gone.txt",
		TargetPath:         "gone.txt",
		Kind:               "file",
		DetectedAt:         "2026-05-16T00:00:00Z",
	})
	writeSoftDelete(t, target, control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "del-old",
		SessionID:          "old",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "older",
		PreviousManifestID: "manifest-older",
		SourcePath:         "old-gone.txt",
		TargetPath:         "old-gone.txt",
		Kind:               "file",
		DetectedAt:         "2026-05-15T00:00:00Z",
	})

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Manifest.SessionID != "new" {
		t.Fatalf("BuildReport(%q).Manifest.SessionID = %q, want new", target, got.Manifest.SessionID)
	}
	if got.Summary.ManifestCount != 2 {
		t.Errorf("BuildReport(%q).Summary.ManifestCount = %d, want 2", target, got.Summary.ManifestCount)
	}
	if got.Summary.FilesExpected != 4 {
		t.Errorf("BuildReport(%q).Summary.FilesExpected = %d, want 4", target, got.Summary.FilesExpected)
	}
	if got.Summary.FilesVerified != 1 {
		t.Errorf("BuildReport(%q).Summary.FilesVerified = %d, want 1", target, got.Summary.FilesVerified)
	}
	if got.Summary.ErrorFindings != 4 {
		t.Errorf("BuildReport(%q).Summary.ErrorFindings = %d, want 4", target, got.Summary.ErrorFindings)
	}
	if got.Summary.WarningFindings != 1 {
		t.Errorf("BuildReport(%q).Summary.WarningFindings = %d, want 1", target, got.Summary.WarningFindings)
	}
	if got.Summary.Warnings != 1 {
		t.Errorf("BuildReport(%q).Summary.Warnings = %d, want 1", target, got.Summary.Warnings)
	}
	if got.Summary.SoftDeletes != 2 {
		t.Errorf("BuildReport(%q).Summary.SoftDeletes = %d, want 2", target, got.Summary.SoftDeletes)
	}
	if got.Summary.TargetDrifts != 0 {
		t.Errorf("BuildReport(%q).Summary.TargetDrifts = %d, want 0", target, got.Summary.TargetDrifts)
	}
	if !hasFinding(got.Findings, FindingMissingFile, "missing.txt") {
		t.Errorf("BuildReport(%q).Findings missing %s for missing.txt: %#v", target, FindingMissingFile, got.Findings)
	}
	if !hasFinding(got.Findings, FindingSizeMismatch, "changed.txt") {
		t.Errorf("BuildReport(%q).Findings missing %s for changed.txt: %#v", target, FindingSizeMismatch, got.Findings)
	}
	if !hasFinding(got.Findings, FindingDigestMismatch, "changed.txt") {
		t.Errorf("BuildReport(%q).Findings missing %s for changed.txt: %#v", target, FindingDigestMismatch, got.Findings)
	}
	if !hasFinding(got.Findings, FindingDigestMissing, "nodigest.txt") {
		t.Errorf("BuildReport(%q).Findings missing %s for nodigest.txt: %#v", target, FindingDigestMissing, got.Findings)
	}
}

func TestBuildReportSurfacesTargetDriftArtifacts(t *testing.T) {
	target := t.TempDir()
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
	})
	writePublishedReceipt(t, target, "session")
	writeTargetDrift(t, target, control.TargetDrift{
		Version:    control.CurrentVersion,
		ID:         "session-drift_file",
		SessionID:  "session",
		ProfileID:  "profile-local",
		TargetID:   "target-local",
		RootID:     "root",
		Path:       "file.txt",
		DetectedAt: "2026-05-16T00:01:00Z",
		Change:     "content_mismatch",
		Evidence:   []string{"target content differs from staged manifest"},
	})

	got, err := BuildReport(Options{TargetRoot: target, SessionID: "session"})
	if err != nil {
		t.Fatalf("BuildReport(%q, session) error = %v, want nil", target, err)
	}
	if got.Summary.TargetDrifts != 1 || len(got.TargetDrifts) != 1 {
		t.Fatalf("BuildReport(%q).TargetDrifts = %#v summary=%+v, want one drift artifact", target, got.TargetDrifts, got.Summary)
	}
	if got.TargetDrifts[0].Path != "file.txt" || got.TargetDrifts[0].Change != "content_mismatch" || got.TargetDrifts[0].ReviewState != "needs_review" {
		t.Fatalf("BuildReport(%q).TargetDrifts[0] = %#v, want target path, change, and normalized review state", target, got.TargetDrifts[0])
	}
}

func TestBuildReportIgnoresResolvedTargetDriftArtifacts(t *testing.T) {
	target := t.TempDir()
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
	})
	writePublishedReceipt(t, target, "session")
	writeTargetDrift(t, target, control.TargetDrift{
		Version:      control.CurrentVersion,
		ID:           "session-drift-resolved",
		SessionID:    "session",
		ProfileID:    "profile-local",
		TargetID:     "target-local",
		RootID:       "root",
		Path:         "file.txt",
		DetectedAt:   "2026-05-16T00:01:00Z",
		Change:       "content_mismatch",
		ReviewState:  "resolved",
		ReviewAction: "resolve",
		ReviewedAt:   "2026-05-20T00:00:00Z",
		ReviewReason: "target restored to expected manifest evidence",
		Evidence:     []string{"target content differed from staged manifest"},
	})

	got, err := BuildReport(Options{TargetRoot: target, SessionID: "session"})
	if err != nil {
		t.Fatalf("BuildReport(%q, session) error = %v, want nil", target, err)
	}
	if got.Summary.TargetDrifts != 0 || len(got.TargetDrifts) != 0 {
		t.Fatalf("BuildReport(%q).TargetDrifts = %#v summary=%+v, want resolved drift excluded from review count", target, got.TargetDrifts, got.Summary)
	}
}

func TestBuildReportReportsBareResolvedTargetDriftAsArtifactProblem(t *testing.T) {
	target := t.TempDir()
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
	})
	writePublishedReceipt(t, target, "session")
	writeRawArtifact(t, target, "drift", "bare-resolved.json", `{"version":1,"id":"bare-resolved","session_id":"session","profile_id":"profile-local","target_id":"target-local","root_id":"root","path":"file.txt","detected_at":"2026-05-16T00:01:00Z","change":"content_mismatch","review_state":"resolved"}`)

	got, err := BuildReport(Options{TargetRoot: target, SessionID: "session"})
	if err != nil {
		t.Fatalf("BuildReport(%q, session) error = %v, want nil", target, err)
	}
	if got.Summary.TargetDrifts != 0 || len(got.TargetDrifts) != 0 {
		t.Fatalf("BuildReport(%q).TargetDrifts = %#v summary=%+v, want invalid bare resolved drift excluded from review list", target, got.TargetDrifts, got.Summary)
	}
	if got.Summary.ArtifactProblems != 1 || len(got.ArtifactProblems) != 1 || !strings.Contains(got.ArtifactProblems[0].Err, `review_action "resolve" is required`) {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v summary=%+v, want bare resolved review-action problem", target, got.ArtifactProblems, got.Summary)
	}
}

func TestBuildReportFiltersTargetDriftsByProfileScope(t *testing.T) {
	target := t.TempDir()
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
	})
	writePublishedReceipt(t, target, "session")
	writeTargetDrift(t, target, control.TargetDrift{
		Version:    control.CurrentVersion,
		ID:         "foreign-drift",
		SessionID:  "foreign-session",
		ProfileID:  "foreign-profile",
		TargetID:   "foreign-target",
		RootID:     "root",
		Path:       "file.txt",
		DetectedAt: "2026-05-16T00:01:00Z",
		Change:     "content_mismatch",
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.TargetDrifts != 0 || len(got.TargetDrifts) != 0 {
		t.Fatalf("BuildReport(%q).TargetDrifts = %#v summary=%+v, want foreign drift filtered", target, got.TargetDrifts, got.Summary)
	}
}

func TestBuildReportRecordsInvalidTargetDriftArtifact(t *testing.T) {
	target := t.TempDir()
	writeRawArtifact(t, target, "drift", "bad.json", `{"version":1,"id":"bad","path":"file.txt","detected_at":"2026-05-16T00:00:00Z","change":"content_mismatch"}`)

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.TargetDrifts != 0 || len(got.TargetDrifts) != 0 {
		t.Fatalf("BuildReport(%q).TargetDrifts = %#v summary=%+v, want invalid drift excluded", target, got.TargetDrifts, got.Summary)
	}
	if got.Summary.ArtifactProblems != 1 || len(got.ArtifactProblems) != 1 || !strings.Contains(got.ArtifactProblems[0].Err, "session_id") {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v summary=%+v, want missing session_id problem", target, got.ArtifactProblems, got.Summary)
	}
}

func TestBuildReportSessionTargetDriftArtifactProblemReturnsStructuredReport(t *testing.T) {
	target := t.TempDir()
	writeRawArtifact(t, target, "drift", "bad-drift.json", `{"version":1,"id":"bad-drift","session_id":"bad-drift","profile_id":"profile-local","target_id":"target-local","root_id":"root","path":"../file.txt","detected_at":"2026-05-16T00:00:00Z","change":"content_mismatch"}`)

	got, err := BuildReport(Options{TargetRoot: target, SessionID: "bad-drift"})
	if err != nil {
		t.Fatalf("BuildReport(%q, bad-drift) error = %v, want structured report", target, err)
	}
	if got.Summary.ManifestCount != 0 || got.Summary.ArtifactProblems != 1 {
		t.Fatalf("BuildReport(%q, bad-drift) summary=%+v, want no manifest and one artifact problem", target, got.Summary)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].SessionID != "bad-drift" || !strings.Contains(got.ArtifactProblems[0].Err, "path is unsafe") {
		t.Fatalf("BuildReport(%q, bad-drift).ArtifactProblems = %#v, want bad-drift unsafe-path problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportSessionTruncatedTargetDriftArtifactProblemReturnsStructuredReport(t *testing.T) {
	target := t.TempDir()
	writeRawArtifact(t, target, "drift", "bad-drift.json", `{"version":1,"id":"bad-drift","session_id":"bad-drift","profile_id":"profile-local","target_id":"target-local","root_id":"root","path":"file.txt",`)

	got, err := BuildReport(Options{TargetRoot: target, SessionID: "bad-drift"})
	if err != nil {
		t.Fatalf("BuildReport(%q, bad-drift) error = %v, want structured report", target, err)
	}
	if got.Summary.ManifestCount != 0 || got.Summary.ArtifactProblems != 1 {
		t.Fatalf("BuildReport(%q, bad-drift) summary=%+v, want no manifest and one artifact problem", target, got.Summary)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].SessionID != "bad-drift" {
		t.Fatalf("BuildReport(%q, bad-drift).ArtifactProblems = %#v, want bad-drift problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportScopeIgnoresMalformedForeignTargetDriftArtifact(t *testing.T) {
	target := t.TempDir()
	writeRawArtifact(t, target, "drift", "foreign-drift.json", `{"version":1,"id":"foreign-drift","session_id":"foreign-session","profile_id":"foreign-profile","target_id":"foreign-target","root_id":"root","path":"../file.txt","detected_at":"2026-05-16T00:00:00Z","change":"content_mismatch"}`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.ArtifactProblems != 0 || len(got.ArtifactProblems) != 0 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v summary=%+v, want foreign malformed drift filtered", target, got.ArtifactProblems, got.Summary)
	}
}

func TestBuildReportScopeIgnoresTruncatedForeignTargetDriftArtifactByReceipt(t *testing.T) {
	target := t.TempDir()
	writePublishedReceiptForScope(t, target, "foreign-session", "foreign-profile", "foreign-target")
	writeRawArtifact(t, target, "drift", "foreign-drift.json", `{"version":1,"id":"foreign-drift","session_id":"foreign-session","root_id":"root","path":"file.txt",`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.ArtifactProblems != 0 || len(got.ArtifactProblems) != 0 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v summary=%+v, want foreign truncated drift filtered by receipt", target, got.ArtifactProblems, got.Summary)
	}
}

func TestBuildReportScopeIgnoresMisleadingTruncatedTargetDriftArtifactByReceipt(t *testing.T) {
	target := t.TempDir()
	writePublishedReceiptForScope(t, target, "foreign-session", "foreign-profile", "foreign-target")
	writeRawArtifact(t, target, "drift", "foreign-drift.json", `{"version":1,"id":"foreign-drift","session_id":"foreign-session","profile_id":"profile-local","target_id":"target-local","root_id":"root","path":"file.txt",`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.ArtifactProblems != 0 || len(got.ArtifactProblems) != 0 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v summary=%+v, want misleading foreign drift filtered by receipt", target, got.ArtifactProblems, got.Summary)
	}
}

func TestBuildReportUnsafeTargetDriftSessionHintDoesNotReadReceiptPath(t *testing.T) {
	target := t.TempDir()
	writeRawArtifact(t, target, "drift", "unsafe-drift.json", `{"version":1,"id":"unsafe-drift","session_id":"../sessions/local-session","profile_id":"profile-local","target_id":"target-local","root_id":"root","path":"file.txt",`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.ArtifactProblems != 1 || len(got.ArtifactProblems) != 1 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v summary=%+v, want unsafe session hint artifact problem", target, got.ArtifactProblems, got.Summary)
	}
	if got.ArtifactProblems[0].SessionID != "" {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want unsafe session hint not trusted as session id", target, got.ArtifactProblems)
	}
}

func TestBuildReportNestedTargetDriftSessionHintDoesNotReadReceiptPath(t *testing.T) {
	target := t.TempDir()
	writeRawArtifact(t, target, "drift", "nested-drift.json", `{"version":1,"id":"nested-drift","session_id":"a/b","profile_id":"profile-local","target_id":"target-local","root_id":"root","path":"file.txt",`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.ArtifactProblems != 1 || len(got.ArtifactProblems) != 1 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v summary=%+v, want nested session hint artifact problem", target, got.ArtifactProblems, got.Summary)
	}
	if got.ArtifactProblems[0].SessionID != "" {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want nested session hint not trusted as session id", target, got.ArtifactProblems)
	}
}

func TestBuildReportDotTargetDriftSessionHintDoesNotReadReceiptPath(t *testing.T) {
	target := t.TempDir()
	writeRawArtifact(t, target, "drift", "dot-drift.json", `{"version":1,"id":"dot-drift","session_id":".","profile_id":"profile-local","target_id":"target-local","root_id":"root","path":"file.txt",`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.ArtifactProblems != 1 || len(got.ArtifactProblems) != 1 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v summary=%+v, want dot session hint artifact problem", target, got.ArtifactProblems, got.Summary)
	}
	if got.ArtifactProblems[0].SessionID != "" {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want dot session hint not trusted as session id", target, got.ArtifactProblems)
	}
}

func TestBuildReportScopeKeepsTruncatedTargetDriftWhenReceiptIDMismatchesSession(t *testing.T) {
	target := t.TempDir()
	writeSessionReceiptFile(t, target, "local-session", control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        "other-session",
		ProfileID: "foreign-profile",
		TargetID:  "foreign-target",
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    "published",
	})
	writeRawArtifact(t, target, "drift", "local-drift.json", `{"version":1,"id":"local-drift","session_id":"local-session","profile_id":"profile-local","target_id":"target-local","root_id":"root","path":"file.txt",`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	foundDriftProblem := false
	foundReceiptMismatch := false
	for _, problem := range got.ArtifactProblems {
		if problem.SessionID == "local-session" && strings.Contains(problem.Path, filepath.Join(control.DirName, "drift")) {
			foundDriftProblem = true
		}
		if problem.SessionID == "local-session" && strings.Contains(problem.Err, `receipt id "other-session" does not match session directory "local-session"`) {
			foundReceiptMismatch = true
		}
	}
	if !foundDriftProblem || !foundReceiptMismatch {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v summary=%+v, want local drift problem and receipt mismatch evidence", target, got.ArtifactProblems, got.Summary)
	}
}

func TestBuildReportRecordsScopedSessionTargetDriftMismatch(t *testing.T) {
	target := t.TempDir()
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
	})
	writePublishedReceipt(t, target, "session")
	writeTargetDrift(t, target, control.TargetDrift{
		Version:    control.CurrentVersion,
		ID:         "session-drift-bad-scope",
		SessionID:  "session",
		ProfileID:  "profile-other",
		TargetID:   "target-other",
		RootID:     "root",
		Path:       "file.txt",
		DetectedAt: "2026-05-16T00:01:00Z",
		Change:     "content_mismatch",
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.TargetDrifts != 0 || len(got.TargetDrifts) != 0 {
		t.Fatalf("BuildReport(%q).TargetDrifts = %#v summary=%+v, want mismatched drift excluded", target, got.TargetDrifts, got.Summary)
	}
	if got.Summary.ArtifactProblems != 1 || len(got.ArtifactProblems) != 1 || !strings.Contains(got.ArtifactProblems[0].Err, "profile_id") {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v summary=%+v, want drift scope mismatch problem", target, got.ArtifactProblems, got.Summary)
	}
}

func TestBuildReportChoosesLatestManifestChronologically(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "early.txt", []byte("early"))
	writeTargetFile(t, target, "late.txt", []byte("late"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-early",
		SessionID: "early",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "early.txt", Kind: "file", Size: 5, Digest: digest([]byte("early"))}},
	})
	writePublishedReceipt(t, target, "early")
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-late",
		SessionID: "late",
		CreatedAt: "2026-05-16T00:00:00.5Z",
		Entries:   []control.ManifestEntry{{Path: "late.txt", Kind: "file", Size: 4, Digest: digest([]byte("late"))}},
	})
	writePublishedReceipt(t, target, "late")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Manifest.SessionID != "late" {
		t.Fatalf("BuildReport(%q).Manifest.SessionID = %q, want late", target, got.Manifest.SessionID)
	}
}

func TestBuildReportFiltersSession(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "old.txt", []byte("old"))
	writeTargetFile(t, target, "new.txt", []byte("new"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-old",
		SessionID: "old",
		CreatedAt: "2026-05-15T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "old.txt", Kind: "file", Size: 3, Digest: digest([]byte("old"))}},
	})
	writePublishedReceipt(t, target, "old")
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-new",
		SessionID: "new",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "new.txt", Kind: "file", Size: 3, Digest: digest([]byte("new"))}},
	})
	writePublishedReceipt(t, target, "new")
	writeSoftDelete(t, target, control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "del-old",
		SessionID:          "old",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "older",
		PreviousManifestID: "manifest-older",
		SourcePath:         "old-gone.txt",
		TargetPath:         "old-gone.txt",
		Kind:               "file",
		DetectedAt:         "2026-05-15T00:00:00Z",
	})
	writeSoftDelete(t, target, control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "del-new",
		SessionID:          "new",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "old",
		PreviousManifestID: "manifest-old",
		SourcePath:         "new-gone.txt",
		TargetPath:         "new-gone.txt",
		Kind:               "file",
		DetectedAt:         "2026-05-16T00:00:00Z",
	})

	got, err := BuildReport(Options{TargetRoot: target, SessionID: "old"})
	if err != nil {
		t.Fatalf("BuildReport(%q, old) error = %v, want nil", target, err)
	}
	if got.Manifest.SessionID != "old" {
		t.Fatalf("BuildReport(%q, old).Manifest.SessionID = %q, want old", target, got.Manifest.SessionID)
	}
	if got.Summary.FilesVerified != 1 {
		t.Errorf("BuildReport(%q, old).Summary.FilesVerified = %d, want 1", target, got.Summary.FilesVerified)
	}
	if got.Summary.SoftDeletes != 1 || got.SoftDeletes[0].SessionID != "old" {
		t.Errorf("BuildReport(%q, old).SoftDeletes = %#v, want only old records", target, got.SoftDeletes)
	}
}

func TestBuildReportSessionManifestArtifactProblemReturnsStructuredReport(t *testing.T) {
	target := t.TempDir()
	writePublishedReceipt(t, target, "bad")
	path, err := control.Path(target, control.ArtifactManifest, "bad")
	if err != nil {
		t.Fatalf("Path(%q, manifest, bad) error = %v", target, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, SessionID: "bad"})
	if err != nil {
		t.Fatalf("BuildReport(%q, bad) error = %v, want structured report", target, err)
	}
	if got.Summary.ManifestCount != 0 || got.Summary.ArtifactProblems != 1 {
		t.Fatalf("BuildReport(%q, bad) summary=%+v, want no manifest and one artifact problem", target, got.Summary)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].SessionID != "bad" {
		t.Fatalf("BuildReport(%q, bad).ArtifactProblems = %#v, want bad session problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportFlagsManifestInvalidCreatedAt(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "ok.txt", []byte("ok"))
	writeRawArtifact(t, target, filepath.Join("sessions", "bad"), "manifest.json", `{"version":1,"id":"manifest-bad","session_id":"bad","created_at":"not-time","entries":[{"path":"ok.txt","kind":"file","size":2,"digest":"`+digest([]byte("ok"))+`","target_path":"ok.txt"}]}`)
	writePublishedReceipt(t, target, "bad")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.ManifestCount != 0 || got.Summary.ArtifactProblems != 1 {
		t.Fatalf("BuildReport(%q) summary=%+v, want malformed manifest as artifact problem", target, got.Summary)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].SessionID != "bad" || !strings.Contains(got.ArtifactProblems[0].Err, "created_at") {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want created_at artifact problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportFlagsSoftDeleteScopeMismatch(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "ok.txt", []byte("ok"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "ok.txt", Kind: "file", Size: 2, Digest: digest([]byte("ok")), TargetPath: "ok.txt"}},
	})
	writePublishedReceipt(t, target, "session")
	writeSoftDelete(t, target, control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "session-del_bad",
		SessionID:          "session",
		ProfileID:          "other-profile",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "previous",
		PreviousManifestID: "manifest-previous",
		SourcePath:         "gone.txt",
		TargetPath:         "gone.txt",
		Kind:               "file",
		DetectedAt:         "2026-05-16T00:00:00Z",
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.SoftDeletes != 0 || len(got.SoftDeletes) != 0 {
		t.Fatalf("BuildReport(%q).SoftDeletes = %#v summary=%+v, want mismatched record excluded", target, got.SoftDeletes, got.Summary)
	}
	if got.Summary.ArtifactProblems != 1 || len(got.ArtifactProblems) != 1 || !strings.Contains(got.ArtifactProblems[0].Err, "profile_id") {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want soft delete profile mismatch problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportSoftDeleteScopeMismatchUsesActualArtifactPath(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "ok.txt", []byte("ok"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "ok.txt", Kind: "file", Size: 2, Digest: digest([]byte("ok")), TargetPath: "ok.txt"}},
	})
	writePublishedReceipt(t, target, "session")
	record := control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "session-del_bad",
		SessionID:          "session",
		ProfileID:          "other-profile",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "previous",
		PreviousManifestID: "manifest-previous",
		SourcePath:         "gone.txt",
		TargetPath:         "gone.txt",
		Kind:               "file",
		DetectedAt:         "2026-05-16T00:00:00Z",
	}
	actualPath := filepath.Join(target, control.DirName, "deleted", "custom-soft-delete-name.json")
	if err := control.WriteFile(actualPath, record); err != nil {
		t.Fatalf("control.WriteFile(%q, softDelete) error = %v, want nil", actualPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if len(got.ArtifactProblems) != 1 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want one soft-delete scope problem", target, got.ArtifactProblems)
	}
	if got.ArtifactProblems[0].Path != filepath.ToSlash(actualPath) {
		t.Fatalf("BuildReport(%q).ArtifactProblems[0].Path = %q, want actual artifact path %q", target, got.ArtifactProblems[0].Path, filepath.ToSlash(actualPath))
	}
}

func TestBuildReportFlagsSoftDeleteRootMismatch(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "ok.txt", []byte("ok"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "ok.txt", Kind: "file", Size: 2, Digest: digest([]byte("ok")), TargetPath: "ok.txt"}},
	})
	writePublishedReceipt(t, target, "session")
	writeSoftDelete(t, target, control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "session-del_bad_root",
		SessionID:          "session",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "wrong-root",
		PreviousSessionID:  "previous",
		PreviousManifestID: "manifest-previous",
		SourcePath:         "gone.txt",
		TargetPath:         "gone.txt",
		Kind:               "file",
		DetectedAt:         "2026-05-16T00:00:00Z",
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.SoftDeletes != 0 || len(got.SoftDeletes) != 0 {
		t.Fatalf("BuildReport(%q).SoftDeletes = %#v summary=%+v, want root-mismatched record excluded", target, got.SoftDeletes, got.Summary)
	}
	if got.Summary.ArtifactProblems != 1 || len(got.ArtifactProblems) != 1 || !strings.Contains(got.ArtifactProblems[0].Err, "root_id") {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want soft delete root mismatch problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportRejectsUnsafeTargetPath(t *testing.T) {
	target := t.TempDir()
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-one",
		SessionID: "one",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "escape.txt", Kind: "file", Size: 1, Digest: digest([]byte("x")), TargetPath: "../escape.txt"},
		},
	})
	writePublishedReceipt(t, target, "one")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !hasFinding(got.Findings, FindingUnsafeTargetPath, "escape.txt") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want unsafe target path finding", target, got.Findings)
	}
}

func TestBuildReportRejectsReservedControlPlaneTargetPath(t *testing.T) {
	target := t.TempDir()
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-one",
		SessionID: "one",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "forged.txt", Kind: "file", Size: 1, Digest: digest([]byte("x")), TargetPath: ".supermover/sessions/forged/receipt.json"},
		},
	})
	writePublishedReceipt(t, target, "one")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !hasFinding(got.Findings, FindingUnsafeTargetPath, "forged.txt") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want unsafe target path finding", target, got.Findings)
	}
}

func TestBuildReportRejectsNormalizedReservedControlPlaneTargetPath(t *testing.T) {
	target := t.TempDir()
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-one",
		SessionID: "one",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "forged.txt", Kind: "file", Size: 1, Digest: digest([]byte("x")), TargetPath: "safe/../.supermover/sessions/forged/receipt.json"},
		},
	})
	writePublishedReceipt(t, target, "one")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !hasFinding(got.Findings, FindingUnsafeTargetPath, "forged.txt") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want unsafe target path finding", target, got.Findings)
	}
}

func TestBuildReportRejectsWindowsShapedTargetPath(t *testing.T) {
	target := t.TempDir()
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-one",
		SessionID: "one",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "drive.txt", Kind: "file", Size: 1, Digest: digest([]byte("x")), TargetPath: "C:/windows-temp/drive.txt"},
			{Path: "backslash.txt", Kind: "file", Size: 1, Digest: digest([]byte("x")), TargetPath: `dir\backslash.txt`},
		},
	})
	writePublishedReceipt(t, target, "one")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !hasFinding(got.Findings, FindingUnsafeTargetPath, "drive.txt") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want unsafe Windows drive path finding", target, got.Findings)
	}
	if !hasFinding(got.Findings, FindingUnsafeTargetPath, "backslash.txt") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want unsafe backslash path finding", target, got.Findings)
	}
}

func TestBuildReportRejectsSymlinkParentTargetPath(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll(outside docs) error = %v, want nil", err)
	}
	writeTargetFile(t, outside, "docs/a.txt", []byte("x"))
	if err := os.Symlink(filepath.Join(outside, "docs"), filepath.Join(target, "docs")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-one",
		SessionID: "one",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "docs/a.txt", Kind: "file", Size: 1, Digest: digest([]byte("x")), TargetPath: "docs/a.txt"},
		},
	})
	writePublishedReceipt(t, target, "one")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !hasFinding(got.Findings, FindingUnsafeTargetPath, "docs/a.txt") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want unsafe target path finding", target, got.Findings)
	}
	if got.Summary.FilesVerified != 0 {
		t.Fatalf("BuildReport(%q).Summary.FilesVerified = %d, want 0", target, got.Summary.FilesVerified)
	}
}

func TestBuildReportFindsMetadataMismatch(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "file.txt", []byte("same"))
	path := filepath.Join(target, "file.txt")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("os.Chmod(%q) error = %v, want nil", path, err)
	}
	actualTime := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, actualTime, actualTime); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", path, err)
	}
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{{
			Path:       "file.txt",
			Kind:       "file",
			Mode:       0o600,
			Size:       4,
			Digest:     digest([]byte("same")),
			ModTime:    "2026-05-16T00:00:00Z",
			TargetPath: "file.txt",
		}},
	})
	writePublishedReceipt(t, target, "session")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !hasFinding(got.Findings, FindingModeMismatch, "file.txt") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want mode mismatch", target, got.Findings)
	}
	if !hasFinding(got.Findings, FindingModTimeMismatch, "file.txt") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want mtime mismatch", target, got.Findings)
	}
}

func TestBuildReportVerifiesDirectoryAndSymlinkEntries(t *testing.T) {
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(target, "dir"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(dir) error = %v, want nil", err)
	}
	if err := os.Symlink("dir", filepath.Join(target, "link")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "dir", Kind: "dir", TargetPath: "dir"},
			{Path: "link", Kind: "symlink", TargetPath: "link", SymlinkTarget: "dir"},
		},
	})
	writePublishedReceipt(t, target, "session")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.ErrorFindings != 0 || len(got.Findings) != 0 {
		t.Fatalf("BuildReport(%q).Findings = %#v summary=%+v, want no findings", target, got.Findings, got.Summary)
	}
}

func TestBuildReportFindsDirectoryAndSymlinkMismatch(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "dir", []byte("not-dir"))
	if err := os.Symlink("other", filepath.Join(target, "link")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "dir", Kind: "dir", TargetPath: "dir"},
			{Path: "link", Kind: "symlink", TargetPath: "link", SymlinkTarget: "dir"},
			{Path: "missing-link", Kind: "symlink", TargetPath: "missing-link", SymlinkTarget: "dir"},
		},
	})
	writePublishedReceipt(t, target, "session")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !hasFinding(got.Findings, FindingNotDirectory, "dir") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want not_directory", target, got.Findings)
	}
	if !hasFinding(got.Findings, FindingSymlinkMismatch, "link") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want symlink_mismatch", target, got.Findings)
	}
	if !hasFinding(got.Findings, FindingMissingSymlink, "missing-link") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want missing_symlink", target, got.Findings)
	}
}

func TestBuildReportFindsUnsupportedManifestKind(t *testing.T) {
	target := t.TempDir()
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "socket", Kind: "socket", TargetPath: "socket"},
		},
	})
	writePublishedReceipt(t, target, "session")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !hasFinding(got.Findings, FindingUnsupportedKind, "socket") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want unsupported_kind", target, got.Findings)
	}
}

func TestBuildReportFiltersReceiptScope(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "ok.txt", []byte("ok"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-one",
		SessionID: "one",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "ok.txt", Kind: "file", Size: 2, Digest: digest([]byte("ok")), TargetPath: "ok.txt"}},
	})
	writePublishedReceipt(t, target, "one")

	got, err := BuildReport(Options{TargetRoot: target, SessionID: "one", ProfileID: "other-profile", TargetID: "target-local"})
	if err == nil {
		t.Fatalf("BuildReport(wrong profile) error = nil, report=%#v, want mismatch error", got)
	}
	if !strings.Contains(err.Error(), "does not match requested profile/target") {
		t.Fatalf("BuildReport(wrong profile) error = %v, want profile/target mismatch", err)
	}
}

func TestLoadArtifactsRecordsBadJSON(t *testing.T) {
	target := t.TempDir()
	warningsDir := filepath.Join(target, control.DirName, "warnings")
	if err := os.MkdirAll(warningsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", warningsDir, err)
	}
	badPath := filepath.Join(warningsDir, "bad.json")
	if err := os.WriteFile(badPath, []byte(`{"version":1`), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", badPath, err)
	}

	got, err := LoadArtifacts(target)
	if err != nil {
		t.Fatalf("LoadArtifacts(%q) error = %v, want nil", target, err)
	}
	if len(got.ArtifactProblems) != 1 {
		t.Fatalf("LoadArtifacts(%q).ArtifactProblems length = %d, want 1", target, len(got.ArtifactProblems))
	}
}

func TestLoadArtifactsRejectsUnsafeControlArtifactBoundary(t *testing.T) {
	tests := []struct {
		name    string
		linkRel string
	}{
		{name: "target root", linkRel: ""},
		{name: "control directory", linkRel: control.DirName},
		{name: "sessions directory", linkRel: filepath.Join(control.DirName, "sessions")},
		{name: "session directory", linkRel: filepath.Join(control.DirName, "sessions", "session")},
		{name: "receipt file", linkRel: filepath.Join(control.DirName, "sessions", "session", "receipt.json")},
		{name: "manifest file", linkRel: filepath.Join(control.DirName, "sessions", "session", "manifest.json")},
		{name: "session record file", linkRel: filepath.Join(control.DirName, "sessions", "session", "session.json")},
		{name: "network transfer file", linkRel: filepath.Join(control.DirName, "sessions", "session", "network-transfer.json")},
		{name: "profiles directory", linkRel: filepath.Join(control.DirName, "profiles")},
		{name: "profile snapshot file", linkRel: filepath.Join(control.DirName, "profiles", "profile-session.json")},
		{name: "pairings directory", linkRel: filepath.Join(control.DirName, "pairings")},
		{name: "pairing receipt file", linkRel: filepath.Join(control.DirName, "pairings", "pairing.json")},
		{name: "warnings file", linkRel: filepath.Join(control.DirName, "warnings", "warning.json")},
		{name: "deleted file", linkRel: filepath.Join(control.DirName, "deleted", "deleted.json")},
		{name: "drift file", linkRel: filepath.Join(control.DirName, "drift", "drift.json")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := t.TempDir()
			target := filepath.Join(parent, "target")
			outside := filepath.Join(parent, "outside")
			if err := os.MkdirAll(outside, 0o755); err != nil {
				t.Fatalf("MkdirAll(outside) error = %v, want nil", err)
			}
			if tt.linkRel == "" {
				if err := os.Symlink(outside, target); err != nil {
					t.Skipf("Symlink(target root) unavailable: %v", err)
				}
			} else {
				if err := os.MkdirAll(target, 0o755); err != nil {
					t.Fatalf("MkdirAll(target) error = %v, want nil", err)
				}
				linkPath := filepath.Join(target, tt.linkRel)
				if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
					t.Fatalf("MkdirAll(link parent) error = %v, want nil", err)
				}
				if err := os.Symlink(outside, linkPath); err != nil {
					t.Skipf("Symlink(%s) unavailable: %v", tt.linkRel, err)
				}
			}

			got, err := LoadArtifacts(target)
			if err == nil {
				t.Fatalf("LoadArtifacts(%q) = %+v, nil; want unsafe boundary error", target, got)
			}
			if !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("LoadArtifacts(%q) error = %q, want symlink boundary error", target, err.Error())
			}
			if _, err := os.Stat(filepath.Join(outside, "sessions")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Stat(outside sessions) error = %v, want os.ErrNotExist", err)
			}
		})
	}
}

func TestLoadArtifactsAllowsStagingSymlinkOutsideReadArtifactFiles(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	stagePath := filepath.Join(target, control.DirName, "sessions", "session", "stage", "link")
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(stage parent) error = %v, want nil", err)
	}
	if err := os.Symlink(outside, stagePath); err != nil {
		t.Skipf("Symlink(stage link) unavailable: %v", err)
	}

	got, err := LoadArtifacts(target)
	if err != nil {
		t.Fatalf("LoadArtifacts(%q) error = %v, want nil for staging symlink outside read artifact files", target, err)
	}
	if len(got.ArtifactProblems) != 0 {
		t.Fatalf("LoadArtifacts(%q).ArtifactProblems = %#v, want none for staging symlink outside read artifact files", target, got.ArtifactProblems)
	}
}

func TestBuildReportReadsLegacySymlinkManifest(t *testing.T) {
	target := t.TempDir()
	path, err := control.Path(target, control.ArtifactManifest, "legacy")
	if err != nil {
		t.Fatalf("control.Path(%q, manifest, legacy) error = %v, want nil", target, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	legacy := `{"version":1,"id":"manifest-legacy","session_id":"legacy","created_at":"2026-05-15T00:00:00Z","entries":[{"path":"link","kind":"symlink","target_path":"link"}]}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
	writePublishedReceipt(t, target, "legacy")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.ManifestCount != 1 || len(got.ArtifactProblems) != 0 {
		t.Fatalf("BuildReport(%q) summary=%+v problems=%#v, want readable legacy manifest", target, got.Summary, got.ArtifactProblems)
	}
}

func TestBuildReportIgnoresUnpublishedArtifacts(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "published.txt", []byte("ok"))
	writeTargetFile(t, target, "draft.txt", []byte("draft"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-published",
		SessionID: "published",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "published.txt", Kind: "file", Size: 2, Digest: digest([]byte("ok"))}},
	})
	writePublishedReceipt(t, target, "published")
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-draft",
		SessionID: "draft",
		CreatedAt: "2026-05-17T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "draft.txt", Kind: "file", Size: 5, Digest: digest([]byte("draft"))}},
	})
	writeWarning(t, target, control.Warning{
		Version:   control.CurrentVersion,
		ID:        "warning-draft",
		SessionID: "draft",
		Code:      "draft_warning",
		Message:   "draft warning",
		Severity:  "warning",
		Paths:     []string{"draft.txt"},
		CreatedAt: "2026-05-17T00:00:00Z",
	})
	writeSoftDelete(t, target, control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "draft-del-one",
		SessionID:          "draft",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "published",
		PreviousManifestID: "manifest-published",
		SourcePath:         "draft-gone.txt",
		TargetPath:         "draft-gone.txt",
		Kind:               "file",
		DetectedAt:         "2026-05-17T00:00:00Z",
	})
	writeRawArtifact(t, target, "warnings", "draft-999-bad.json", `{"version":1`)
	writeRawArtifact(t, target, "deleted", "draft-del_bad.json", `{"version":1`)

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.ManifestCount != 1 || got.Manifest.SessionID != "published" {
		t.Fatalf("BuildReport(%q) manifest summary = %+v, want only published session", target, got.Summary)
	}
	if got.Summary.Warnings != 0 {
		t.Fatalf("BuildReport(%q).Summary.Warnings = %d, want 0 unpublished warnings", target, got.Summary.Warnings)
	}
	if got.Summary.SoftDeletes != 0 {
		t.Fatalf("BuildReport(%q).Summary.SoftDeletes = %d, want 0 unpublished soft deletes", target, got.Summary.SoftDeletes)
	}
	if len(got.ArtifactProblems) != 0 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want unpublished global artifacts skipped before parsing", target, got.ArtifactProblems)
	}
}

func TestBuildReportRecordsMalformedPublishedGlobalArtifact(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "published.txt", []byte("ok"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-published",
		SessionID: "published",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "published.txt", Kind: "file", Size: 2, Digest: digest([]byte("ok"))}},
	})
	writePublishedReceipt(t, target, "published")
	writeRawArtifact(t, target, "warnings", "published-999-bad.json", `{"version":1`)

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if len(got.ArtifactProblems) != 1 {
		t.Fatalf("BuildReport(%q).ArtifactProblems length = %d, want 1", target, len(got.ArtifactProblems))
	}
}

func TestBuildReportSessionFilterKeepsUnscopedArtifactProblems(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "selected.txt", []byte("ok"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-selected",
		SessionID: "selected",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "selected.txt", Kind: "file", Size: 2, Digest: digest([]byte("ok"))}},
	})
	writePublishedReceipt(t, target, "selected")
	writeRawArtifact(t, target, "warnings", "bad-global.json", `{"version":1`)

	got, err := BuildReport(Options{TargetRoot: target, SessionID: "selected"})
	if err != nil {
		t.Fatalf("BuildReport(%q, selected) error = %v, want nil", target, err)
	}
	if len(got.ArtifactProblems) != 1 {
		t.Fatalf("BuildReport(%q, selected).ArtifactProblems = %#v, want unscoped global artifact problem retained", target, got.ArtifactProblems)
	}
	if got.ArtifactProblems[0].SessionID != "" || !strings.Contains(got.ArtifactProblems[0].Path, "bad-global.json") {
		t.Fatalf("BuildReport(%q, selected).ArtifactProblems = %#v, want unscoped bad-global problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportMalformedPublishedGlobalArtifactIsNotHiddenByLongerUnpublishedPrefix(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "published.txt", []byte("ok"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-published",
		SessionID: "published",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "published.txt", Kind: "file", Size: 2, Digest: digest([]byte("ok"))}},
	})
	writePublishedReceipt(t, target, "published")
	if err := os.MkdirAll(filepath.Join(target, control.DirName, "sessions", "published-999"), 0o755); err != nil {
		t.Fatalf("MkdirAll(unpublished longer prefix session) error = %v, want nil", err)
	}
	writeRawArtifact(t, target, "warnings", "published-999-bad.json", `{"version":1`)

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if len(got.ArtifactProblems) != 1 {
		t.Fatalf("BuildReport(%q).ArtifactProblems length = %d, want 1", target, len(got.ArtifactProblems))
	}
}

func writeManifest(t *testing.T, target string, manifest control.Manifest) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactManifest, manifest.SessionID)
	if err != nil {
		t.Fatalf("Path(%q, manifest, %q) error = %v", target, manifest.SessionID, err)
	}
	if err := control.WriteFile(path, manifest); err != nil {
		t.Fatalf("WriteFile(%q, manifest) error = %v", path, err)
	}
}

func writePublishedReceipt(t *testing.T, target string, sessionID string) {
	t.Helper()
	writePublishedReceiptForScope(t, target, sessionID, "profile-local", "target-local")
}

func writePublishedReceiptForScope(t *testing.T, target string, sessionID string, profileID string, targetID string) {
	t.Helper()
	writeSessionReceiptFile(t, target, sessionID, control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: profileID,
		TargetID:  targetID,
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    "published",
	})
}

func writeSessionReceiptFile(t *testing.T, target string, sessionID string, receipt control.SessionReceipt) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("Path(%q, receipt, %q) error = %v", target, sessionID, err)
	}
	if err := control.WriteFile(path, receipt); err != nil {
		t.Fatalf("WriteFile(%q, receipt) error = %v", path, err)
	}
}

func writeWarning(t *testing.T, target string, warning control.Warning) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactWarning, warning.ID)
	if err != nil {
		t.Fatalf("Path(%q, warning, %q) error = %v", target, warning.ID, err)
	}
	if err := control.WriteFile(path, warning); err != nil {
		t.Fatalf("WriteFile(%q, warning) error = %v", path, err)
	}
}

func writeSoftDelete(t *testing.T, target string, softDelete control.SoftDelete) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSoftDelete, softDelete.ID)
	if err != nil {
		t.Fatalf("Path(%q, soft_delete, %q) error = %v", target, softDelete.ID, err)
	}
	if err := control.WriteFile(path, softDelete); err != nil {
		t.Fatalf("WriteFile(%q, softDelete) error = %v", path, err)
	}
}

func writeTargetDrift(t *testing.T, target string, drift control.TargetDrift) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactTargetDrift, drift.ID)
	if err != nil {
		t.Fatalf("Path(%q, target_drift, %q) error = %v", target, drift.ID, err)
	}
	if err := control.WriteFile(path, drift); err != nil {
		t.Fatalf("WriteFile(%q, targetDrift) error = %v", path, err)
	}
}

func writeTargetFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func writeRawArtifact(t *testing.T, target, dir, name string, data string) {
	t.Helper()
	path := filepath.Join(target, control.DirName, dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func hasFinding(findings []Finding, kind FindingKind, path string) bool {
	for _, finding := range findings {
		if finding.Kind == kind && finding.Path == path {
			return true
		}
	}
	return false
}
