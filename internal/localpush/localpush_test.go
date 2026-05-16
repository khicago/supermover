package localpush

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/transaction"
)

func TestRunCopiesFilesAndWritesControlArtifacts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, ".hidden"), "secret", 0o640)
	mustWriteFile(t, filepath.Join(source, "notes", "a.md"), "hello", 0o600)
	mustWriteFile(t, filepath.Join(source, "AGENTS.md"), "rules", 0o644)
	modTime := time.Date(2026, 5, 16, 7, 8, 9, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(source, "notes", "a.md"), modTime, modTime); err != nil {
		t.Fatalf("os.Chtimes(source file) error = %v, want nil", err)
	}

	p := profile.NewDefault("profile-local", "Local profile", source, target)
	now := time.Date(2026, 5, 16, 10, 11, 12, 0, time.UTC)
	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test", Now: now})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got.SessionID != "session-test" {
		t.Errorf("Run() session id = %q, want %q", got.SessionID, "session-test")
	}
	if got.Copied != 3 {
		t.Errorf("Run() copied = %d, want %d", got.Copied, 3)
	}
	if got.Influences != 1 {
		t.Errorf("Run() influences = %d, want %d", got.Influences, 1)
	}
	if gotBytes, err := os.ReadFile(filepath.Join(target, ".hidden")); err != nil || string(gotBytes) != "secret" {
		t.Fatalf("target hidden file = (%q, %v), want secret", string(gotBytes), err)
	}
	targetInfo, err := os.Stat(filepath.Join(target, "notes", "a.md"))
	if err != nil {
		t.Fatalf("os.Stat(target file) error = %v, want nil", err)
	}
	if targetInfo.Mode().Perm() != 0o600 {
		t.Errorf("target mode = %v, want 0600", targetInfo.Mode().Perm())
	}
	if !targetInfo.ModTime().Equal(modTime) {
		t.Errorf("target mtime = %v, want %v", targetInfo.ModTime(), modTime)
	}

	manifest := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-test")
	if len(manifest.Entries) != got.Entries {
		t.Fatalf("manifest entries = %d, want %d", len(manifest.Entries), got.Entries)
	}
	if !manifestContainsDigest(manifest, "notes/a.md") {
		t.Errorf("manifest entries = %#v, want digest for notes/a.md", manifest.Entries)
	}
	receipt := readControlDoc[control.SessionReceipt](t, target, control.ArtifactSessionReceipt, "session-test")
	if receipt.Status != "published" {
		t.Errorf("receipt status = %q, want published", receipt.Status)
	}
	snapshot := readControlDoc[control.ProfileSnapshot](t, target, control.ArtifactProfileSnapshot, "profile-session-test")
	if snapshot.ProfileID != p.ProfileID {
		t.Errorf("profile snapshot id = %q, want %q", snapshot.ProfileID, p.ProfileID)
	}
	record, err := transaction.ReadSessionRecord(filepath.Join(target, control.DirName, "sessions", "session-test", "session.json"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord() error = %v, want nil", err)
	}
	if record.State != transaction.StatePublished {
		t.Errorf("transaction state = %q, want %q", record.State, transaction.StatePublished)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "agent", "session-test-influence.json")); err != nil {
		t.Fatalf("os.Stat(agent influence) error = %v, want nil", err)
	}
}

func TestRunRecordsSymlinkWarningWithoutCopyingTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "real.txt"), "real", 0o644)
	if err := os.Symlink("real.txt", filepath.Join(source, "link.txt")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	p := profile.NewDefault("profile-local", "Local profile", source, target)
	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if got.Warnings != 1 {
		t.Fatalf("Run() warnings = %d, want %d", got.Warnings, 1)
	}
	warningPath := filepath.Join(target, control.DirName, "warnings")
	entries, err := os.ReadDir(warningPath)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil", warningPath, err)
	}
	if len(entries) != 1 {
		t.Fatalf("warnings = %#v, want one warning artifact", entries)
	}
	warning, err := control.ReadFile[control.Warning](filepath.Join(warningPath, entries[0].Name()))
	if err != nil {
		t.Fatalf("control.ReadFile(warning) error = %v, want nil", err)
	}
	if warning.Code != "symlink_not_copied" {
		t.Fatalf("warning code = %q, want symlink_not_copied", warning.Code)
	}
	if _, err := os.Lstat(filepath.Join(target, "link.txt")); !os.IsNotExist(err) {
		t.Fatalf("os.Lstat(target symlink) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunRejectsUnsupportedSelectionRules(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "payload", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Exclude = []profile.Rule{{Pattern: "*.tmp"}}

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
	if err == nil {
		t.Fatalf("Run(profile with exclude) error = nil, want unsupported rule error")
	}
	if !strings.Contains(err.Error(), "exclude rules are not implemented") {
		t.Fatalf("Run(profile with exclude) error = %q, want unsupported rule error", err.Error())
	}
}

func TestRunRejectsNestedSourceAndTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(source, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "payload", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
	if err == nil {
		t.Fatalf("Run(nested target) error = nil, want nested target error")
	}
	if !strings.Contains(err.Error(), "target directory must not be inside the source root") {
		t.Fatalf("Run(nested target) error = %q, want nested target error", err.Error())
	}
}

func TestRunRejectsSymlinkedTargetInsideSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	sourceLink := filepath.Join(dir, "source-link")
	target := filepath.Join(sourceLink, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "payload", 0o644)
	if err := os.Symlink(source, sourceLink); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
	if err == nil {
		t.Fatalf("Run(symlinked nested target) error = nil, want nested target error")
	}
	if !strings.Contains(err.Error(), "target directory must not be inside the source root") {
		t.Fatalf("Run(symlinked nested target) error = %q, want nested target error", err.Error())
	}
}

func TestRunRejectsExistingSession(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "payload", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)

	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
	if err == nil {
		t.Fatalf("second Run(existing session) error = nil, want existing session error")
	}
	if !strings.Contains(err.Error(), "already published") {
		t.Fatalf("second Run(existing session) error = %q, want already published error", err.Error())
	}
}

func TestRunHonorsDeletePolicyIgnore(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "keep.txt"), "keep", 0o644)
	mustWriteFile(t, filepath.Join(source, "gone.txt"), "gone", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.DeletePolicy.Mode = profile.DeleteModeIgnore
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	if err := os.Remove(filepath.Join(source, "gone.txt")); err != nil {
		t.Fatalf("os.Remove(source gone) error = %v, want nil", err)
	}

	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("second Run() error = %v, want nil", err)
	}
	if got.Deleted != 0 {
		t.Fatalf("second Run() deleted = %d, want 0", got.Deleted)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "deleted")); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(deleted dir) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunRecordsSoftDeleteWithoutRemovingTargetFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "keep.txt"), "keep", 0o644)
	mustWriteFile(t, filepath.Join(source, "gone.txt"), "gone", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	if err := os.Remove(filepath.Join(source, "gone.txt")); err != nil {
		t.Fatalf("os.Remove(source gone) error = %v, want nil", err)
	}

	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("second Run() error = %v, want nil", err)
	}
	if got.Deleted != 1 {
		t.Fatalf("second Run() deleted = %d, want 1", got.Deleted)
	}
	if _, err := os.Stat(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("os.Stat(target gone) error = %v, want file retained for review", err)
	}
	deletedDir := filepath.Join(target, control.DirName, "deleted")
	entries, err := os.ReadDir(deletedDir)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil", deletedDir, err)
	}
	if len(entries) != 1 {
		t.Fatalf("soft-delete artifacts = %d, want 1", len(entries))
	}
	record, err := control.ReadFile[control.SoftDelete](filepath.Join(deletedDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("control.ReadFile(soft delete) error = %v, want nil", err)
	}
	if record.SourcePath != "gone.txt" || record.SessionID != "session-two" {
		t.Fatalf("soft-delete record = %#v, want gone.txt in session-two", record)
	}
}

func readControlDoc[T control.Document](t *testing.T, target string, artifact control.ArtifactType, id string) T {
	t.Helper()
	path, err := control.Path(target, artifact, id)
	if err != nil {
		t.Fatalf("control.Path(%q, %q) error = %v, want nil", artifact, id, err)
	}
	doc, err := control.ReadFile[T](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	return doc
}

func manifestContainsDigest(manifest control.Manifest, path string) bool {
	for _, entry := range manifest.Entries {
		if entry.Path == path && strings.HasPrefix(entry.Digest, "sha256:") {
			return true
		}
	}
	return false
}

func mustWriteFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
}
