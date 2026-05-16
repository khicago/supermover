package deleted

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/scan"
)

func TestGenerateSoftDeleteRecords(t *testing.T) {
	now := time.Date(2026, 5, 16, 1, 2, 3, 4, time.UTC)
	previous := control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-one",
		SessionID: "session-one",
		CreatedAt: "2026-05-15T00:00:00Z",
		Entries: []control.ManifestEntry{
			{Path: "keep.txt", Kind: "file", TargetPath: "keep.txt"},
			{Path: "deleted.txt", Kind: "file", TargetPath: "deleted.txt", Size: 7, Digest: "sha256:abcdef"},
			{Path: "deleted-link", Kind: "symlink", TargetPath: "deleted-link", SymlinkTarget: "deleted.txt"},
			{Path: "empty-dir", Kind: "dir", TargetPath: "empty-dir"},
		},
	}
	current := scan.Result{
		Entries: []scan.Entry{
			{Path: ".", Kind: scan.KindDir},
			{Path: "keep.txt", Kind: scan.KindRegular},
		},
	}

	got, err := Generate(Options{
		PreviousManifest: previous,
		CurrentScan:      current,
		SessionID:        "session-two",
		ProfileID:        "profile-local",
		TargetID:         "local:profile-local",
		RootID:           "root",
		DetectedAt:       now,
	})
	if err != nil {
		t.Fatalf("Generate(%#v) error = %v, want nil", previous, err)
	}
	if len(got.Records) != 2 {
		t.Fatalf("Generate(%#v).Records length = %d, want 2", previous, len(got.Records))
	}
	if got.Records[0].SourcePath != "deleted-link" {
		t.Errorf("Generate(%#v).Records[0].SourcePath = %q, want deleted-link", previous, got.Records[0].SourcePath)
	}
	if got.Records[1].SourcePath != "deleted.txt" {
		t.Errorf("Generate(%#v).Records[1].SourcePath = %q, want deleted.txt", previous, got.Records[1].SourcePath)
	}
	for _, record := range got.Records {
		if record.Version != control.CurrentVersion {
			t.Errorf("Generate(%#v) record version = %d, want %d", previous, record.Version, control.CurrentVersion)
		}
		if record.SessionID != "session-two" {
			t.Errorf("Generate(%#v) record session = %q, want session-two", previous, record.SessionID)
		}
		if record.ProfileID != "profile-local" || record.TargetID != "local:profile-local" || record.RootID != "root" {
			t.Errorf("Generate(%#v) record scope = (%q, %q, %q), want profile-local/local:profile-local/root", previous, record.ProfileID, record.TargetID, record.RootID)
		}
		if record.PreviousSessionID != "session-one" || record.PreviousManifestID != "manifest-one" {
			t.Errorf("Generate(%#v) previous evidence = (%q, %q), want session-one/manifest-one", previous, record.PreviousSessionID, record.PreviousManifestID)
		}
		if record.DetectedAt != "2026-05-16T01:02:03.000000004Z" {
			t.Errorf("Generate(%#v) record detected_at = %q, want 2026-05-16T01:02:03.000000004Z", previous, record.DetectedAt)
		}
		if record.Reason == "" {
			t.Errorf("Generate(%#v) record reason = empty, want explanation", previous)
		}
	}
	if got.Records[1].Kind != "file" || got.Records[1].Size != 7 || got.Records[1].Digest != "sha256:abcdef" {
		t.Errorf("Generate(%#v).Records[1] evidence = (%q, %d, %q), want file/7/sha256:abcdef", previous, got.Records[1].Kind, got.Records[1].Size, got.Records[1].Digest)
	}
}

func TestGenerateDoesNotPhysicallyDelete(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "deleted.txt")
	if err := os.WriteFile(path, []byte("still here"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	previous := control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-one",
		SessionID: "session-one",
		CreatedAt: "2026-05-15T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "deleted.txt", Kind: "file", TargetPath: "deleted.txt"}},
	}

	_, err := Generate(Options{
		PreviousManifest: previous,
		CurrentScan:      scan.Result{Root: root, Entries: []scan.Entry{{Path: ".", Kind: scan.KindDir}}},
		ProfileID:        "profile-local",
		TargetID:         "local:profile-local",
		RootID:           "root",
		DetectedAt:       time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Generate(%#v) error = %v, want nil", previous, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat(%q) error = %v, want file to remain", path, err)
	}
}

func TestStableIDDeterministic(t *testing.T) {
	got := StableID("session-one", "a/../deleted.txt", "./target.txt")
	want := StableID("session-one", "deleted.txt", "target.txt")
	if got != want {
		t.Fatalf("StableID() = %q, want %q", got, want)
	}
}

func TestGenerateRequiresPreviousSessionID(t *testing.T) {
	_, err := Generate(Options{PreviousManifest: control.Manifest{ID: "manifest-one"}})
	if err == nil {
		t.Fatalf("Generate(missing session_id) error = nil, want error")
	}
}
