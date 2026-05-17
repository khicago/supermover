package verify

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
)

func TestDetectTargetDriftChoosesLatestPublishedManifest(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "latest.txt", []byte("latest"))

	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-older",
		SessionID: "older",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "older.txt", Kind: "file", Size: 5, Digest: digest([]byte("older")), TargetPath: "older.txt"},
		},
	})
	writePublishedReceipt(t, target, "older")
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-latest",
		SessionID: "latest",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:01Z",
		Entries: []control.ManifestEntry{
			{Path: "latest.txt", Kind: "file", Size: 6, Digest: digest([]byte("latest")), TargetPath: "latest.txt"},
		},
	})
	writePublishedReceipt(t, target, "latest")

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
	if err != nil {
		t.Fatalf("DetectTargetDrift(%q) error = %v, want nil", target, err)
	}
	if got.Manifest.SessionID != "latest" || got.Manifest.ID != "manifest-latest" {
		t.Fatalf("DetectTargetDrift(%q).Manifest = %+v, want latest manifest", target, got.Manifest)
	}
	if got.Summary.ManifestCount != 2 || len(got.Manifests) != 2 {
		t.Fatalf("DetectTargetDrift(%q) manifests summary=%+v manifests=%#v, want both published manifests summarized", target, got.Summary, got.Manifests)
	}
	if got.Summary.TargetDrifts != 0 || len(got.Drifts) != 0 {
		t.Fatalf("DetectTargetDrift(%q).Drifts = %#v summary=%+v, want latest manifest only with no drift", target, got.Drifts, got.Summary)
	}
}

func TestDetectTargetDriftUsesExplicitSession(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "selected.txt", []byte("selected"))
	writeTargetFile(t, target, "latest.txt", []byte("changed"))

	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-selected",
		SessionID: "selected",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "selected.txt", Kind: "file", Size: 8, Digest: digest([]byte("selected")), TargetPath: "selected.txt"},
		},
	})
	writePublishedReceipt(t, target, "selected")
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-latest",
		SessionID: "latest",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:01Z",
		Entries: []control.ManifestEntry{
			{Path: "latest.txt", Kind: "file", Size: 6, Digest: digest([]byte("latest")), TargetPath: "latest.txt"},
		},
	})
	writePublishedReceipt(t, target, "latest")

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, SessionID: "selected", Now: fixedDriftNow(t)})
	if err != nil {
		t.Fatalf("DetectTargetDrift(%q, selected) error = %v, want nil", target, err)
	}
	if got.Manifest.SessionID != "selected" || got.SessionID != "selected" {
		t.Fatalf("DetectTargetDrift(%q, selected) session=%q manifest=%+v, want selected session", target, got.SessionID, got.Manifest)
	}
	if got.Summary.ManifestCount != 1 || got.Summary.ManifestEntries != 1 {
		t.Fatalf("DetectTargetDrift(%q, selected) summary=%+v, want explicit session manifest only", target, got.Summary)
	}
	if got.Summary.TargetDrifts != 0 || len(got.Drifts) != 0 {
		t.Fatalf("DetectTargetDrift(%q, selected).Drifts = %#v summary=%+v, want no latest-session drift when selected is explicit", target, got.Drifts, got.Summary)
	}
}

func TestDetectTargetDriftRecordsManifestEntryDrifts(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "modified.txt", []byte("actual"))
	writeTargetFile(t, target, "metadata.txt", []byte("same"))
	writeTargetFile(t, target, "type-mismatch", []byte("not a directory"))
	writeTargetFile(t, target, "link", []byte("not a link"))
	if err := os.Symlink("actual-target", filepath.Join(target, "symlink-mismatch")); err != nil {
		t.Fatalf("Symlink(symlink-mismatch) error = %v, want nil", err)
	}
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "missing.txt", Kind: "file", Size: 7, Digest: digest([]byte("missing")), TargetPath: "missing.txt"},
			{Path: "modified.txt", Kind: "file", Size: 8, Digest: digest([]byte("expected")), TargetPath: "modified.txt"},
			{Path: "metadata.txt", Kind: "file", Size: 4, Digest: digest([]byte("same")), Mode: 0o600, TargetPath: "metadata.txt"},
			{Path: "type-mismatch", Kind: "dir", TargetPath: "type-mismatch"},
			{Path: "link", Kind: "symlink", TargetPath: "link", SymlinkTarget: "expected-target"},
			{Path: "symlink-mismatch", Kind: "symlink", TargetPath: "symlink-mismatch", SymlinkTarget: "expected-target"},
		},
	})
	writePublishedReceipt(t, target, "session")

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
	if err != nil {
		t.Fatalf("DetectTargetDrift(%q) error = %v, want nil", target, err)
	}
	assertGeneratedDrift(t, got.Drifts, driftExpectation{
		path:            "missing.txt",
		change:          "missing",
		expectedKind:    "file",
		expectedSize:    ptrInt64(7),
		expectedDigest:  digest([]byte("missing")),
		observedKind:    "missing",
		observedPresent: ptrBool(false),
		evidence:        []string{"target path is missing"},
	})
	assertGeneratedDrift(t, got.Drifts, driftExpectation{
		path:            "modified.txt",
		change:          "content_mismatch",
		expectedKind:    "file",
		expectedSize:    ptrInt64(8),
		expectedDigest:  digest([]byte("expected")),
		observedKind:    "file",
		observedPresent: ptrBool(true),
		observedSize:    ptrInt64(6),
		observedDigest:  digest([]byte("actual")),
		evidence:        []string{"digest mismatch"},
	})
	assertGeneratedDrift(t, got.Drifts, driftExpectation{
		path:            "metadata.txt",
		change:          "metadata_mismatch",
		expectedKind:    "file",
		expectedSize:    ptrInt64(4),
		expectedMode:    ptrUint32(0o600),
		expectedDigest:  digest([]byte("same")),
		observedKind:    "file",
		observedPresent: ptrBool(true),
		observedSize:    ptrInt64(4),
		observedMode:    ptrUint32(0o644),
		observedDigest:  digest([]byte("same")),
		evidence:        []string{"mode mismatch"},
	})
	assertGeneratedDrift(t, got.Drifts, driftExpectation{
		path:            "type-mismatch",
		change:          "type_mismatch",
		expectedKind:    "dir",
		observedKind:    "file",
		observedPresent: ptrBool(true),
		observedSize:    ptrInt64(15),
		evidence:        []string{`expected kind "dir" but found "file"`},
	})
	assertGeneratedDrift(t, got.Drifts, driftExpectation{
		path:            "link",
		change:          "type_mismatch",
		expectedKind:    "symlink",
		expectedLink:    "expected-target",
		observedKind:    "file",
		observedPresent: ptrBool(true),
		observedSize:    ptrInt64(10),
		evidence:        []string{`expected kind "symlink" but found "file"`},
	})
	assertGeneratedDrift(t, got.Drifts, driftExpectation{
		path:            "symlink-mismatch",
		change:          "symlink_mismatch",
		expectedKind:    "symlink",
		expectedLink:    "expected-target",
		observedKind:    "symlink",
		observedPresent: ptrBool(true),
		observedLink:    "actual-target",
		evidence:        []string{"symlink target mismatch"},
	})
	assertDriftSummary(t, got, 6, 0)
}

func TestDetectTargetDriftRecordsExtraTargetPathsAndSkipsControlPlane(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "keep.txt", []byte("keep"))
	writeTargetFile(t, target, ".hidden/secret.txt", []byte("secret"))
	writeTargetFile(t, target, "docs/.supermover/note.txt", []byte("nested control-looking data"))
	if err := os.Mkdir(filepath.Join(target, "empty-extra-dir"), 0o755); err != nil {
		t.Fatalf("Mkdir(empty-extra-dir) error = %v, want nil", err)
	}
	writeTargetFile(t, target, filepath.ToSlash(filepath.Join(control.DirName, "user-data.txt")), []byte("reserved top-level control data"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "keep.txt", Kind: "file", Size: 4, Digest: digest([]byte("keep")), TargetPath: "keep.txt"},
		},
	})
	writePublishedReceipt(t, target, "session")

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
	if err != nil {
		t.Fatalf("DetectTargetDrift(%q) error = %v, want nil", target, err)
	}
	assertGeneratedDrift(t, got.Drifts, driftExpectation{
		path:            ".hidden/secret.txt",
		change:          "extra",
		expectedEmpty:   true,
		observedKind:    "file",
		observedPresent: ptrBool(true),
		observedSize:    ptrInt64(6),
		observedDigest:  digest([]byte("secret")),
		evidence:        []string{"target path is not present in the selected manifest"},
	})
	assertGeneratedDrift(t, got.Drifts, driftExpectation{
		path:            "docs/.supermover/note.txt",
		change:          "extra",
		expectedEmpty:   true,
		observedKind:    "file",
		observedPresent: ptrBool(true),
		observedSize:    ptrInt64(27),
		observedDigest:  digest([]byte("nested control-looking data")),
		evidence:        []string{"target path is not present in the selected manifest"},
	})
	assertGeneratedDrift(t, got.Drifts, driftExpectation{
		path:            "empty-extra-dir",
		change:          "extra",
		expectedEmpty:   true,
		observedKind:    "dir",
		observedPresent: ptrBool(true),
		evidence:        []string{"target path is not present in the selected manifest"},
	})
	assertNoDriftPath(t, got.Drifts, ".supermover")
	assertNoDriftPath(t, got.Drifts, ".supermover/user-data.txt")
	assertDriftSummary(t, got, 3, 0)
}

func TestDetectTargetDriftExplicitLatestRecordsExtras(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "keep.txt", []byte("keep"))
	writeTargetFile(t, target, "extra.txt", []byte("extra"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "keep.txt", Kind: "file", Size: 4, Digest: digest([]byte("keep")), TargetPath: "keep.txt"},
		},
	})
	writePublishedReceipt(t, target, "session")

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, SessionID: "session", Now: fixedDriftNow(t)})
	if err != nil {
		t.Fatalf("DetectTargetDrift(%q, latest explicit) error = %v, want nil", target, err)
	}
	assertDrift(t, got.Drifts, "extra.txt", "extra")
	assertDriftSummary(t, got, 1, 0)
}

func TestDetectTargetDriftDoesNotDuplicateSoftDeletedTargetPaths(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "keep.txt", []byte("keep"))
	writeTargetFile(t, target, "deleted.txt", []byte("retained target copy"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "keep.txt", Kind: "file", Size: 4, Digest: digest([]byte("keep")), TargetPath: "keep.txt"},
		},
	})
	writePublishedReceipt(t, target, "session")
	writeSoftDelete(t, target, control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "session-del_001-deleted",
		SessionID:          "session",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "previous",
		PreviousManifestID: "manifest-previous",
		SourcePath:         "deleted.txt",
		TargetPath:         "deleted.txt",
		Kind:               "file",
		DetectedAt:         "2026-05-16T00:00:00Z",
		Reason:             "source_missing",
	})

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
	if err != nil {
		t.Fatalf("DetectTargetDrift(%q) error = %v, want nil", target, err)
	}
	assertNoDriftPath(t, got.Drifts, "deleted.txt")
	assertDriftSummary(t, got, 0, 0)
}

func TestDetectTargetDriftDoesNotFollowParentSymlink(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	writeTargetFile(t, outside, "file.txt", []byte("outside"))
	if err := os.Symlink(outside, filepath.Join(target, "parent-link")); err != nil {
		t.Skipf("Symlink(parent-link) unavailable: %v", err)
	}
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "parent-link/file.txt", Kind: "file", Size: 7, Digest: digest([]byte("outside")), TargetPath: "parent-link/file.txt"},
		},
	})
	writePublishedReceipt(t, target, "session")

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
	if err != nil {
		t.Fatalf("DetectTargetDrift(%q) error = %v, want nil", target, err)
	}
	assertDrift(t, got.Drifts, "parent-link/file.txt", "unsafe_parent")
	assertNoDriftPath(t, got.Drifts, "parent-link/file.txt/file.txt")
	assertDriftSummary(t, got, 1, 0)
}

func TestDetectTargetDriftRejectsSymlinkTargetRoot(t *testing.T) {
	parent := t.TempDir()
	realTarget := filepath.Join(parent, "real-target")
	if err := os.Mkdir(realTarget, 0o755); err != nil {
		t.Fatalf("Mkdir(real-target) error = %v, want nil", err)
	}
	symlinkTarget := filepath.Join(parent, "target-link")
	if err := os.Symlink(realTarget, symlinkTarget); err != nil {
		t.Skipf("Symlink(target root) unavailable: %v", err)
	}

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: symlinkTarget, Now: fixedDriftNow(t)})
	if err == nil {
		t.Fatalf("DetectTargetDrift(%q) = %+v, nil; want symlink target root error", symlinkTarget, got)
	}
	if !strings.Contains(err.Error(), "target root") || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("DetectTargetDrift(%q) error = %q, want target root symlink error", symlinkTarget, err.Error())
	}
}

func TestDetectTargetDriftRejectsSymlinkControlPlane(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(target, control.DirName)); err != nil {
		t.Skipf("Symlink(control plane) unavailable: %v", err)
	}

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
	if err == nil {
		t.Fatalf("DetectTargetDrift(%q) = %+v, nil; want symlink control-plane error", target, got)
	}
	if !strings.Contains(err.Error(), "control directory") || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("DetectTargetDrift(%q) error = %q, want control directory symlink error", target, err.Error())
	}
	if _, err := os.Stat(filepath.Join(outside, "sessions")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(outside sessions) error = %v, want os.ErrNotExist", err)
	}
}

func TestDetectTargetDriftRejectsNondirectoryControlPlane(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, control.DirName, []byte("not a directory"))

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
	if err == nil {
		t.Fatalf("DetectTargetDrift(%q) = %+v, nil; want non-directory control-plane error", target, got)
	}
	if !strings.Contains(err.Error(), "control path") || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("DetectTargetDrift(%q) error = %q, want control path directory error", target, err.Error())
	}
}

func TestDetectTargetDriftRejectsSymlinkControlArtifactSurfaces(t *testing.T) {
	tests := []struct {
		name    string
		linkRel string
	}{
		{name: "sessions directory", linkRel: filepath.Join(control.DirName, "sessions")},
		{name: "session directory", linkRel: filepath.Join(control.DirName, "sessions", "session")},
		{name: "receipt file", linkRel: filepath.Join(control.DirName, "sessions", "session", "receipt.json")},
		{name: "manifest file", linkRel: filepath.Join(control.DirName, "sessions", "session", "manifest.json")},
		{name: "warnings file", linkRel: filepath.Join(control.DirName, "warnings", "session-001-warning.json")},
		{name: "deleted file", linkRel: filepath.Join(control.DirName, "deleted", "session-del_001-file.json")},
		{name: "drift file", linkRel: filepath.Join(control.DirName, "drift", "drift.json")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := t.TempDir()
			outside := t.TempDir()
			linkPath := filepath.Join(target, tt.linkRel)
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatalf("MkdirAll(link parent) error = %v, want nil", err)
			}
			if err := os.Symlink(outside, linkPath); err != nil {
				t.Skipf("Symlink(%s) unavailable: %v", tt.linkRel, err)
			}

			got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
			if err == nil {
				t.Fatalf("DetectTargetDrift(%q) = %+v, nil; want control artifact symlink error", target, got)
			}
			if !strings.Contains(err.Error(), "control artifact path") || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("DetectTargetDrift(%q) error = %q, want control artifact symlink error", target, err.Error())
			}
			if _, err := os.Stat(filepath.Join(outside, "sessions")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Stat(outside sessions) error = %v, want os.ErrNotExist", err)
			}
		})
	}
}

func TestDetectTargetDriftAllowsSymlinkOutsideReadArtifactSurfaces(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	holdsPath := filepath.Join(target, control.DirName, "replacement-holds")
	if err := os.MkdirAll(filepath.Dir(holdsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(replacement holds parent) error = %v, want nil", err)
	}
	if err := os.Symlink(outside, holdsPath); err != nil {
		t.Skipf("Symlink(replacement-holds) unavailable: %v", err)
	}

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
	if err != nil {
		t.Fatalf("DetectTargetDrift(%q) error = %v, want nil for symlink outside detector artifact surfaces", target, err)
	}
	assertDriftSummary(t, got, 0, 0)
}

func TestDetectTargetDriftAllowsStagingSymlinkOutsideReadArtifactFiles(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	stagePath := filepath.Join(target, control.DirName, "sessions", "session", "stage", "link")
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(stage parent) error = %v, want nil", err)
	}
	if err := os.Symlink(outside, stagePath); err != nil {
		t.Skipf("Symlink(stage link) unavailable: %v", err)
	}

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
	if err != nil {
		t.Fatalf("DetectTargetDrift(%q) error = %v, want nil for staging symlink outside read artifact files", target, err)
	}
	assertDriftSummary(t, got, 0, 0)
}

func TestDetectTargetDriftKeepsMissingTargetRootBehavior(t *testing.T) {
	target := filepath.Join(t.TempDir(), "missing-target")

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
	if err != nil {
		t.Fatalf("DetectTargetDrift(%q) error = %v, want nil for missing target root", target, err)
	}
	if got.TargetRoot != filepath.ToSlash(target) {
		t.Fatalf("DetectTargetDrift(%q).TargetRoot = %q, want slash-normalized target root", target, got.TargetRoot)
	}
	if !got.NeedsReview() {
		t.Fatalf("DetectTargetDrift(%q).NeedsReview() = false, want true for missing published manifest", target)
	}
	assertDriftSummary(t, got, 0, 0)
}

func TestDriftReportNeedsReview(t *testing.T) {
	tests := []struct {
		name string
		in   DriftReport
		want bool
	}{
		{
			name: "clean published manifest",
			in:   DriftReport{Summary: DriftSummary{ManifestCount: 1}},
			want: false,
		},
		{
			name: "detected drift",
			in:   DriftReport{Summary: DriftSummary{ManifestCount: 1, TargetDrifts: 1}},
			want: true,
		},
		{
			name: "artifact problem",
			in:   DriftReport{Summary: DriftSummary{ManifestCount: 1, ArtifactProblems: 1}},
			want: true,
		},
		{
			name: "no published manifest",
			in:   DriftReport{Summary: DriftSummary{ManifestCount: 0}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.NeedsReview(); got != tt.want {
				t.Fatalf("DriftReport.NeedsReview() = %t, want %t for %+v", got, tt.want, tt.in.Summary)
			}
		})
	}
}

func TestDetectTargetDriftRejectsUnsafeManifestTargetPath(t *testing.T) {
	target := t.TempDir()
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "reserved.txt", Kind: "file", Size: 1, Digest: digest([]byte("x")), TargetPath: ".supermover/sessions/forged/receipt.json"},
			{Path: "windows.txt", Kind: "file", Size: 1, Digest: digest([]byte("x")), TargetPath: `C:/windows-temp/windows.txt`},
		},
	})
	writePublishedReceipt(t, target, "session")

	got, err := DetectTargetDrift(DriftOptions{TargetRoot: target, Now: fixedDriftNow(t)})
	if err != nil {
		t.Fatalf("DetectTargetDrift(%q) error = %v, want nil", target, err)
	}
	assertDriftSummary(t, got, 0, 2)
}

func TestSHA256ObservedRegularRejectsChangedFileObject(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "file.txt", []byte("expected"))
	path := filepath.Join(target, "file.txt")
	observed, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%q) error = %v, want nil", path, err)
	}
	outside := t.TempDir()
	writeTargetFile(t, outside, "outside.txt", []byte("outside"))
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove(%q) error = %v, want nil", path, err)
	}
	if err := os.Symlink(filepath.Join(outside, "outside.txt"), path); err != nil {
		t.Skipf("Symlink(replaced file) unavailable: %v", err)
	}

	got, err := sha256ObservedRegular(path, observed)
	if err == nil {
		t.Fatalf("sha256ObservedRegular(replaced symlink) = %q, nil; want changed-file error", got)
	}
}

func fixedDriftNow(t *testing.T) time.Time {
	t.Helper()
	now, err := time.Parse(time.RFC3339Nano, "2026-05-17T12:34:56Z")
	if err != nil {
		t.Fatalf("Parse fixed drift time error = %v", err)
	}
	return now
}

func assertDrift(t *testing.T, drifts []control.TargetDrift, path string, change string) {
	t.Helper()
	for _, drift := range drifts {
		if drift.Path == path && drift.Change == change {
			if drift.DetectedAt != fixedDriftNow(t).Format(time.RFC3339Nano) {
				t.Fatalf("drift %q/%q DetectedAt = %q, want fixed Now", path, change, drift.DetectedAt)
			}
			if drift.ReviewState != "needs_review" {
				t.Fatalf("drift %q/%q ReviewState = %q, want needs_review", path, change, drift.ReviewState)
			}
			return
		}
	}
	t.Fatalf("drifts = %#v, want path %q change %q", drifts, path, change)
}

type driftExpectation struct {
	path            string
	change          string
	expectedEmpty   bool
	expectedKind    string
	expectedSize    *int64
	expectedMode    *uint32
	expectedDigest  string
	expectedLink    string
	observedKind    string
	observedPresent *bool
	observedSize    *int64
	observedMode    *uint32
	observedDigest  string
	observedLink    string
	evidence        []string
}

func assertGeneratedDrift(t *testing.T, drifts []control.TargetDrift, want driftExpectation) {
	t.Helper()
	var found *control.TargetDrift
	for i := range drifts {
		if drifts[i].Path == want.path && drifts[i].Change == want.change {
			found = &drifts[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("drifts = %#v, want path %q change %q", drifts, want.path, want.change)
	}
	drift := *found
	wantID, err := detectedTargetDriftID(
		control.SessionReceipt{ProfileID: "profile-local", TargetID: "target-local"},
		control.Manifest{SessionID: "session", RootID: "root"},
		want.path,
		want.change,
		drift.Expected,
		drift.Observed,
		drift.Evidence,
	)
	if err != nil {
		t.Fatalf("detectedTargetDriftID(%q/%q) error = %v, want nil", want.path, want.change, err)
	}
	if drift.ID != wantID {
		t.Fatalf("drift %q/%q ID = %q, want deterministic detected ID", want.path, want.change, drift.ID)
	}
	if drift.SessionID != "session" || drift.ProfileID != "profile-local" || drift.TargetID != "target-local" || drift.RootID != "root" {
		t.Fatalf("drift %q/%q scope = session:%q profile:%q target:%q root:%q, want session/profile-local/target-local/root", want.path, want.change, drift.SessionID, drift.ProfileID, drift.TargetID, drift.RootID)
	}
	if drift.DetectedAt != fixedDriftNow(t).Format(time.RFC3339Nano) {
		t.Fatalf("drift %q/%q DetectedAt = %q, want fixed Now", want.path, want.change, drift.DetectedAt)
	}
	if drift.ReviewState != "needs_review" {
		t.Fatalf("drift %q/%q ReviewState = %q, want needs_review", want.path, want.change, drift.ReviewState)
	}
	if want.expectedEmpty {
		if drift.Expected.SessionID != "session" || drift.Expected.ManifestID != "manifest-session" || drift.Expected.Path != want.path || drift.Expected.Kind != "missing" {
			t.Fatalf("drift %q/%q Expected = %+v, want selected manifest missing-state baseline for extra target path", want.path, want.change, drift.Expected)
		}
	} else {
		if drift.Expected.SessionID != "session" || drift.Expected.ManifestID != "manifest-session" || drift.Expected.Path != want.path || drift.Expected.Kind != want.expectedKind {
			t.Fatalf("drift %q/%q Expected = %+v, want session manifest path/kind", want.path, want.change, drift.Expected)
		}
		if drift.Expected.Digest != want.expectedDigest || drift.Expected.SymlinkTarget != want.expectedLink {
			t.Fatalf("drift %q/%q Expected digest/link = %q/%q, want %q/%q", want.path, want.change, drift.Expected.Digest, drift.Expected.SymlinkTarget, want.expectedDigest, want.expectedLink)
		}
		assertOptionalSize(t, "expected.size", drift.Expected.HasSizeEvidence(), drift.Expected.Size, want.expectedSize)
		assertOptionalMode(t, "expected.mode", drift.Expected.HasModeEvidence(), drift.Expected.Mode, want.expectedMode)
	}
	if drift.Observed.Path != want.path || drift.Observed.Kind != want.observedKind {
		t.Fatalf("drift %q/%q Observed = %+v, want path/kind %q/%q", want.path, want.change, drift.Observed, want.path, want.observedKind)
	}
	if want.observedPresent != nil {
		if drift.Observed.Present == nil || *drift.Observed.Present != *want.observedPresent {
			t.Fatalf("drift %q/%q Observed.Present = %v, want %t", want.path, want.change, drift.Observed.Present, *want.observedPresent)
		}
	}
	if drift.Observed.Digest != want.observedDigest || drift.Observed.SymlinkTarget != want.observedLink {
		t.Fatalf("drift %q/%q Observed digest/link = %q/%q, want %q/%q", want.path, want.change, drift.Observed.Digest, drift.Observed.SymlinkTarget, want.observedDigest, want.observedLink)
	}
	assertOptionalSize(t, "observed.size", drift.Observed.HasSizeEvidence(), drift.Observed.Size, want.observedSize)
	assertOptionalMode(t, "observed.mode", drift.Observed.HasModeEvidence(), drift.Observed.Mode, want.observedMode)
	for _, wantEvidence := range want.evidence {
		if !containsSubstring(drift.Evidence, wantEvidence) {
			t.Fatalf("drift %q/%q Evidence = %#v, want substring %q", want.path, want.change, drift.Evidence, wantEvidence)
		}
	}
}

func assertOptionalSize(t *testing.T, field string, present bool, got int64, want *int64) {
	t.Helper()
	if want == nil {
		return
	}
	if !present || got != *want {
		t.Fatalf("%s = (%v, %d), want present %d", field, present, got, *want)
	}
}

func assertOptionalMode(t *testing.T, field string, present bool, got uint32, want *uint32) {
	t.Helper()
	if want == nil {
		return
	}
	if !present || got != *want {
		t.Fatalf("%s = (%v, %04o), want present %04o", field, present, got, *want)
	}
}

func containsSubstring(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func ptrBool(value bool) *bool {
	return &value
}

func ptrInt64(value int64) *int64 {
	return &value
}

func ptrUint32(value uint32) *uint32 {
	return &value
}

func assertNoDriftPath(t *testing.T, drifts []control.TargetDrift, path string) {
	t.Helper()
	for _, drift := range drifts {
		if drift.Path == path {
			t.Fatalf("drifts = %#v, did not want path %q", drifts, path)
		}
	}
}

func assertDriftSummary(t *testing.T, report DriftReport, targetDrifts int, artifactProblems int) {
	t.Helper()
	if len(report.Drifts) != targetDrifts || report.Summary.TargetDrifts != targetDrifts {
		t.Fatalf("Drifts = %#v summary=%+v, want %d target drifts", report.Drifts, report.Summary, targetDrifts)
	}
	if len(report.ArtifactProblems) != artifactProblems || report.Summary.ArtifactProblems != artifactProblems {
		t.Fatalf("ArtifactProblems = %#v summary=%+v, want %d artifact problems", report.ArtifactProblems, report.Summary, artifactProblems)
	}
}
