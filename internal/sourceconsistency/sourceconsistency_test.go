package sourceconsistency

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/scan"
)

func TestComparePassesWhenCurrentSourceMatchesBaseline(t *testing.T) {
	root := t.TempDir()
	mustWriteSourceFile(t, filepath.Join(root, "file.txt"), "hello\n")
	mustMkdir(t, filepath.Join(root, "dir"))
	mustSymlink(t, "file.txt", filepath.Join(root, "link"))

	p := profile.NewDefault("profile-src", "Source", root, filepath.Join(t.TempDir(), "target"))
	result, err := scan.Scan(root)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", root, err)
	}
	baseline, err := BuildBaseline(BuildBaselineOptions{
		Profile:   p,
		SessionID: "session-1",
		Now:       fixedNow,
		Entries:   result.Entries,
	})
	if err != nil {
		t.Fatalf("BuildBaseline error = %v, want nil", err)
	}

	report, err := Compare(CompareOptions{
		ProfilePath: filepath.Join(root, "profile.json"),
		Baseline:    baseline,
		Now:         fixedNow,
	})
	if err != nil {
		t.Fatalf("Compare error = %v, want nil", err)
	}
	if report.Status != StatusPass || report.Mode != ModeCurrentVerified || report.MismatchCount != 0 {
		t.Fatalf("Compare report = %+v, want pass/current_source_verified with zero mismatches", report)
	}
}

func TestCompareBlocksWhenCurrentSourceChanges(t *testing.T) {
	root := t.TempDir()
	mustWriteSourceFile(t, filepath.Join(root, "file.txt"), "hello\n")

	p := profile.NewDefault("profile-src", "Source", root, filepath.Join(t.TempDir(), "target"))
	result, err := scan.Scan(root)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", root, err)
	}
	baseline, err := BuildBaseline(BuildBaselineOptions{
		Profile:   p,
		SessionID: "session-2",
		Now:       fixedNow,
		Entries:   result.Entries,
	})
	if err != nil {
		t.Fatalf("BuildBaseline error = %v, want nil", err)
	}
	mustWriteSourceFile(t, filepath.Join(root, "file.txt"), "changed\n")
	mustWriteSourceFile(t, filepath.Join(root, "extra.txt"), "new\n")

	report, err := Compare(CompareOptions{
		ProfilePath: filepath.Join(root, "profile.json"),
		Baseline:    baseline,
		Now:         fixedNow,
	})
	if err != nil {
		t.Fatalf("Compare error = %v, want nil", err)
	}
	if report.Status != StatusBlocked || report.Mode != ModeCurrentMismatch || report.MismatchCount != 2 {
		t.Fatalf("Compare report = %+v, want blocked/current_source_mismatch with two mismatches", report)
	}
}

func TestBuildBaselineSkipsReservedAndSpecialEntries(t *testing.T) {
	root := t.TempDir()
	mustWriteSourceFile(t, filepath.Join(root, "file.txt"), "hello\n")
	mustMkdir(t, filepath.Join(root, ".supermover"))
	mustWriteSourceFile(t, filepath.Join(root, ".supermover", "ignored.json"), "{}")
	p := profile.NewDefault("profile-src", "Source", root, filepath.Join(t.TempDir(), "target"))
	result, err := scan.Scan(root)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", root, err)
	}
	result.Entries = append(result.Entries, scan.Entry{
		Path: "fifo",
		Kind: scan.KindSpecial,
		Mode: os.ModeNamedPipe | 0o600,
	})
	baseline, err := BuildBaseline(BuildBaselineOptions{
		Profile: p,
		Now:     fixedNow,
		Entries: result.Entries,
	})
	if err != nil {
		t.Fatalf("BuildBaseline error = %v, want nil", err)
	}
	if len(baseline.Entries) != 1 || baseline.Entries[0].Path != "file.txt" {
		t.Fatalf("BuildBaseline entries = %+v, want only transferable file", baseline.Entries)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
}

func mustWriteSourceFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", path, err)
	}
}

func mustSymlink(t *testing.T, target string, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("os.Symlink(%q, %q) error = %v", target, path, err)
	}
}
