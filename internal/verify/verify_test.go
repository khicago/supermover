package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

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
	if got.Summary.ErrorFindings != 3 {
		t.Errorf("BuildReport(%q).Summary.ErrorFindings = %d, want 3", target, got.Summary.ErrorFindings)
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
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-new",
		SessionID: "new",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "new.txt", Kind: "file", Size: 3, Digest: digest([]byte("new"))}},
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

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !hasFinding(got.Findings, FindingUnsafeTargetPath, "escape.txt") {
		t.Fatalf("BuildReport(%q).Findings = %#v, want unsafe target path finding", target, got.Findings)
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

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.ManifestCount != 1 || len(got.ArtifactProblems) != 0 {
		t.Fatalf("BuildReport(%q) summary=%+v problems=%#v, want readable legacy manifest", target, got.Summary, got.ArtifactProblems)
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
