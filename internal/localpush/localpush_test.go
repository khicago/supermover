package localpush

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/agentkb"
	"github.com/khicago/supermover/internal/audit"
	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/scan"
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
	if manifest.RootID != p.Roots[0].ID {
		t.Errorf("manifest root_id = %q, want %q", manifest.RootID, p.Roots[0].ID)
	}
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
	if receipt.TargetID != p.Target.TargetID || receipt.TargetID == filepath.Clean(target) {
		t.Errorf("receipt target id = %q, want profile target id %q and not target path %q", receipt.TargetID, p.Target.TargetID, filepath.Clean(target))
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
	if _, err := os.Stat(filepath.Join(target, control.DirName, "locks", "target.lock")); err != nil {
		t.Fatalf("os.Stat(target lock) error = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "locks", "session-test.lock")); err != nil {
		t.Fatalf("os.Stat(session lock) error = %v, want nil", err)
	}
}

func TestRunSkipsSourceControlPlaneDirectory(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, ".supermover", "sessions", "forged", "receipt.json"), `{"status":"published"}`, 0o644)
	mustWriteFile(t, filepath.Join(source, "real.txt"), "real", 0o644)

	p := profile.NewDefault("profile-local", "Local profile", source, target)
	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-real"})
	if err != nil {
		t.Fatalf("Run(source .supermover) error = %v, want nil", err)
	}
	if got.Warnings != 1 {
		t.Fatalf("Run(source .supermover).Warnings = %d, want 1", got.Warnings)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "forged", "receipt.json")); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(copied forged receipt) error = %v, want os.ErrNotExist", err)
	}
	manifest := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-real")
	if manifestContainsPath(manifest, control.DirName+"/sessions/forged/receipt.json") {
		t.Fatalf("manifest entries = %#v, want source .supermover omitted", manifest.Entries)
	}
	warning := readOnlyWarning(t, target)
	if warning.Code != "reserved_control_plane_skipped" {
		t.Fatalf("warning code = %q, want reserved_control_plane_skipped", warning.Code)
	}
	if warning.SuggestedConfig["append_migration_path"] != control.DirName {
		t.Fatalf("warning suggested config = %#v, want append migration path", warning.SuggestedConfig)
	}
}

func TestRunSkipsCaseVariantSourceControlPlaneDirectory(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, ".Supermover", "sessions", "forged", "receipt.json"), `{"status":"published"}`, 0o644)
	mustWriteFile(t, filepath.Join(source, "real.txt"), "real", 0o644)

	p := profile.NewDefault("profile-local", "Local profile", source, target)
	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-real"})
	if err != nil {
		t.Fatalf("Run(source .Supermover) error = %v, want nil", err)
	}
	if got.Warnings != 1 {
		t.Fatalf("Run(source .Supermover).Warnings = %d, want 1", got.Warnings)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "forged", "receipt.json")); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(copied forged receipt) error = %v, want os.ErrNotExist", err)
	}
	manifest := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-real")
	if manifestContainsPath(manifest, ".Supermover/sessions/forged/receipt.json") {
		t.Fatalf("manifest entries = %#v, want source .Supermover omitted", manifest.Entries)
	}
}

func TestRunRejectsTargetControlPlaneSymlink(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	outside := filepath.Join(dir, "outside-control")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "payload", 0o644)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", target, err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", outside, err)
	}
	if err := os.Symlink(outside, filepath.Join(target, control.DirName)); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"}); err == nil {
		t.Fatalf("Run(target .supermover symlink) error = nil, want control path error")
	}
	if _, err := os.Stat(filepath.Join(outside, "sessions", "session-test")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(outside session) error = %v, want os.ErrNotExist", err)
	}
	if _, err := os.Stat(filepath.Join(target, "file.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(target file) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunConcurrentSameSessionAllowsSinglePublisher(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "payload", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	existing := 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case strings.Contains(err.Error(), "already published"):
			existing++
		default:
			t.Fatalf("Run(concurrent same session) error = %v, want nil or already published", err)
		}
	}
	if successes != 1 || existing != 1 {
		t.Fatalf("Run(concurrent same session) successes=%d existing=%d, want 1/1", successes, existing)
	}
}

func TestLatestPublishedManifestUsesChronologicalTime(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "keep.txt"), "keep", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-early",
		SessionID: "early",
		RootID:    p.Roots[0].ID,
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "early.txt", Kind: "file", TargetPath: "early.txt"}},
	})
	writeReceipt(t, target, p, "early", "2026-05-16T00:00:00Z")
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-late",
		SessionID: "late",
		RootID:    p.Roots[0].ID,
		CreatedAt: "2026-05-16T00:00:00.5Z",
		Entries:   []control.ManifestEntry{{Path: "late.txt", Kind: "file", TargetPath: "late.txt"}},
	})
	writeReceipt(t, target, p, "late", "2026-05-16T00:00:00.5Z")

	got, ok, err := latestPublishedManifest(p, target)
	if err != nil {
		t.Fatalf("latestPublishedManifest(%q) error = %v, want nil", target, err)
	}
	if !ok || got.SessionID != "late" {
		t.Fatalf("latestPublishedManifest(%q) = (%q, %t), want late/true", target, got.SessionID, ok)
	}
}

func TestRunPublishesSymlinkTarget(t *testing.T) {
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
	if got.Warnings != 0 {
		t.Fatalf("Run() warnings = %d, want 0", got.Warnings)
	}
	manifest := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-test")
	if !manifestContainsSymlinkTarget(manifest, "link.txt", "real.txt") {
		t.Fatalf("manifest entries = %#v, want symlink target for link.txt", manifest.Entries)
	}
	gotTarget, err := os.Readlink(filepath.Join(target, "link.txt"))
	if err != nil {
		t.Fatalf("os.Readlink(target symlink) error = %v, want nil", err)
	}
	if gotTarget != "real.txt" {
		t.Fatalf("os.Readlink(target symlink) = %q, want real.txt", gotTarget)
	}
}

func TestRunRecordsWarningForUnsafeSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "real.txt"), "real", 0o644)
	if err := os.Symlink("../outside", filepath.Join(source, "link.txt")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	p := profile.NewDefault("profile-local", "Local profile", source, target)
	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
	if err != nil {
		t.Fatalf("Run(unsafe symlink) error = %v, want nil with warning", err)
	}
	if got.Warnings != 1 {
		t.Fatalf("Run(unsafe symlink).Warnings = %d, want 1", got.Warnings)
	}
	if _, err := os.Lstat(filepath.Join(target, "link.txt")); !os.IsNotExist(err) {
		t.Fatalf("os.Lstat(target unsafe symlink) error = %v, want os.ErrNotExist", err)
	}
	warning := readOnlyWarning(t, target)
	if warning.Code != "symlink_not_published" || warning.Detected["target"] != "../outside" {
		t.Fatalf("warning = %#v, want symlink_not_published with target evidence", warning)
	}
	manifest := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-test")
	if manifestContainsPath(manifest, "link.txt") {
		t.Fatalf("manifest entries = %#v, want unsafe symlink omitted", manifest.Entries)
	}
}

func TestReadStableSymlinkRejectsTargetChange(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", source, err)
	}
	link := filepath.Join(source, "link.txt")
	if err := os.Symlink("before.txt", link); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("os.Lstat(%q) error = %v, want nil", link, err)
	}
	entry := scan.Entry{Path: "link.txt", Kind: scan.KindSymlink, ModTime: info.ModTime(), SymlinkTarget: "before.txt"}
	if err := os.Remove(link); err != nil {
		t.Fatalf("os.Remove(%q) error = %v, want nil", link, err)
	}
	if err := os.Symlink("after.txt", link); err != nil {
		t.Fatalf("os.Symlink(after) error = %v, want nil", err)
	}

	if _, err := readStableSymlink(link, entry); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("readStableSymlink(changed target) error = %v, want changed target error", err)
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

func TestRunRejectsSymlinkedParentInsideTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "dir", "file.txt"), "payload", 0o644)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", target, err)
	}
	if err := os.Symlink(filepath.Join(source, "dir"), filepath.Join(target, "dir")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
	if err == nil {
		t.Fatalf("Run(symlinked target parent) error = nil, want unsafe path error")
	}
	if !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("Run(symlinked target parent) error = %q, want unsafe path error", err.Error())
	}
	if got, err := os.ReadFile(filepath.Join(source, "dir", "file.txt")); err != nil || string(got) != "payload" {
		t.Fatalf("source file after failed Run = (%q, %v), want unchanged payload", string(got), err)
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

func TestCopyRegularRejectsChangedSourceBeforePublish(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	target := filepath.Join(dir, "target.txt")
	mustWriteFile(t, source, "old", 0o644)
	initial := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)
	if err := os.Chtimes(source, initial, initial); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", source, err)
	}
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", target, err)
	}

	_, err := copyRegularWithPostCopy(source, target, 0o644, initial, func() error {
		next := initial.Add(time.Second)
		if err := os.WriteFile(source, []byte("new"), 0o644); err != nil {
			return err
		}
		return os.Chtimes(source, next, next)
	})
	if err == nil {
		t.Fatalf("copyRegularWithPostCopy(changed source) error = nil, want source changed error")
	}
	if !strings.Contains(err.Error(), "changed during copy") {
		t.Fatalf("copyRegularWithPostCopy(changed source) error = %q, want source changed error", err.Error())
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "existing" {
		t.Fatalf("target after failed copy = (%q, %v), want existing", string(got), err)
	}
}

func TestCopyRegularRejectsReplacedSourceBeforePublish(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	target := filepath.Join(dir, "target.txt")
	stamp := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)
	mustWriteFile(t, source, "old", 0o644)
	if err := os.Chtimes(source, stamp, stamp); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", source, err)
	}
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", target, err)
	}

	_, err := copyRegularWithPostCopy(source, target, 0o644, stamp, func() error {
		replacement := filepath.Join(dir, "replacement.txt")
		if err := os.WriteFile(replacement, []byte("old"), 0o644); err != nil {
			return err
		}
		if err := os.Chtimes(replacement, stamp, stamp); err != nil {
			return err
		}
		return os.Rename(replacement, source)
	})
	if err == nil {
		t.Fatalf("copyRegularWithPostCopy(replaced source) error = nil, want source changed error")
	}
	if !strings.Contains(err.Error(), "changed during copy") {
		t.Fatalf("copyRegularWithPostCopy(replaced source) error = %q, want source changed error", err.Error())
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "existing" {
		t.Fatalf("target after failed copy = (%q, %v), want existing", string(got), err)
	}
}

func TestCopyRegularRejectsChangedSourceSinceScan(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	target := filepath.Join(dir, "target.txt")
	stamp := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)
	mustWriteFile(t, source, "old", 0o644)
	if err := os.Chtimes(source, stamp, stamp); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", source, err)
	}
	result, err := scan.Scan(dir)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", dir, err)
	}
	entry := scanEntryByPath(t, result, "source.txt")
	replacement := filepath.Join(dir, "replacement.txt")
	mustWriteFile(t, replacement, "new", 0o644)
	if err := os.Chtimes(replacement, stamp, stamp); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", replacement, err)
	}
	if err := os.Rename(replacement, source); err != nil {
		t.Fatalf("os.Rename(%q, %q) error = %v, want nil", replacement, source, err)
	}
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", target, err)
	}

	_, err = copyRegularToStage(source, target+".stage", entry)
	if err == nil {
		t.Fatalf("copyRegularToStage(replaced since scan) error = nil, want source changed error")
	}
	if !strings.Contains(err.Error(), "changed since scan") {
		t.Fatalf("copyRegularToStage(replaced since scan) error = %q, want source changed error", err.Error())
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "existing" {
		t.Fatalf("target after failed copy = (%q, %v), want existing", string(got), err)
	}
}

func TestCopyRegularRejectsSymlinkSource(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside.txt")
	source := filepath.Join(dir, "source.txt")
	target := filepath.Join(dir, "target.txt")
	mustWriteFile(t, outside, "outside", 0o644)
	if err := os.Symlink(outside, source); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", target, err)
	}

	entry := scan.Entry{Path: "source.txt", Kind: scan.KindRegular, Size: int64(len("outside")), Mode: 0o644}
	_, err := copyRegularToStage(source, target+".stage", entry)
	if err == nil {
		t.Fatalf("copyRegularToStage(symlink source) error = nil, want non-regular source error")
	}
	if !strings.Contains(err.Error(), "changed since scan") {
		t.Fatalf("copyRegularToStage(symlink source) error = %q, want non-regular source error", err.Error())
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "existing" {
		t.Fatalf("target after symlink copy rejection = (%q, %v), want existing", string(got), err)
	}
}

func scanEntryByPath(t *testing.T, result scan.Result, path string) scan.Entry {
	t.Helper()
	for _, entry := range result.Entries {
		if entry.Path == path {
			return entry
		}
	}
	t.Fatalf("scan result entries = %#v, want path %q", result.Entries, path)
	return scan.Entry{}
}

func TestRunRecordsSoftDeleteWhenFileBecomesSymlink(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "item"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	if err := os.Remove(filepath.Join(source, "item")); err != nil {
		t.Fatalf("os.Remove(source item) error = %v, want nil", err)
	}
	if err := os.Symlink("real-target", filepath.Join(source, "item")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("second Run(file to symlink) error = %v, want nil with warning", err)
	}
	if got.Deleted != 1 || got.Warnings != 1 {
		t.Fatalf("second Run(file to symlink) deleted=%d warnings=%d, want 1/1", got.Deleted, got.Warnings)
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
	if record.SourcePath != "item" || !strings.Contains(record.Reason, "current source scan observes symlink") {
		t.Fatalf("soft-delete record = %#v, want kind-change evidence for item", record)
	}
	warningDir := filepath.Join(target, control.DirName, "warnings")
	warnings, err := os.ReadDir(warningDir)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil", warningDir, err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warning artifacts = %d, want 1", len(warnings))
	}
	warning, err := control.ReadFile[control.Warning](filepath.Join(warningDir, warnings[0].Name()))
	if err != nil {
		t.Fatalf("control.ReadFile(warning) error = %v, want nil", err)
	}
	if warning.Code != "symlink_not_published" {
		t.Fatalf("warning code = %q, want symlink_not_published", warning.Code)
	}

	preflight, err := Preflight(Options{Profile: p, TargetDir: target, SessionID: "session-three", Now: time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Preflight(file to symlink) error = %v, want nil", err)
	}
	if preflight.Deleted != 0 || preflight.Warnings != 1 {
		t.Fatalf("Preflight(file to symlink) deleted=%d warnings=%d, want 0/1 after session-two recorded soft delete", preflight.Deleted, preflight.Warnings)
	}
	if bytes, err := os.ReadFile(filepath.Join(target, "item")); err != nil || string(bytes) != "old" {
		t.Fatalf("target retained file = (%q, %v), want old retained for review", string(bytes), err)
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
	if record.ProfileID != p.ProfileID || record.TargetID != p.Target.TargetID || record.RootID != p.Roots[0].ID {
		t.Fatalf("soft-delete record scope = %#v, want profile/target/root evidence", record)
	}
	if record.PreviousSessionID != "session-one" || record.PreviousManifestID != "manifest-session-one" || record.Kind != "file" {
		t.Fatalf("soft-delete previous evidence = %#v, want previous manifest/session and file kind", record)
	}
}

func TestPreflightReportsSoftDeletes(t *testing.T) {
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

	got, err := Preflight(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Preflight(soft delete) error = %v, want nil", err)
	}
	if got.Deleted != 1 {
		t.Fatalf("Preflight(soft delete).Deleted = %d, want 1", got.Deleted)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "deleted")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(deleted dir after Preflight) error = %v, want os.ErrNotExist", err)
	}
}

func TestPreflightRejectsScanErrorsBeforeSoftDeletes(t *testing.T) {
	result := scan.Result{
		Audit: []audit.Record{{Kind: "scan_error", Path: "unreadable"}},
	}
	if err := rejectScanErrors(result); err == nil {
		t.Fatalf("rejectScanErrors(scan_error) error = nil, want scan error")
	}
}

func TestRunReadsLegacySymlinkManifestForSoftDeletes(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "keep.txt"), "keep", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if err := os.MkdirAll(filepath.Join(target, control.DirName, "sessions", "session-one"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(legacy session) error = %v, want nil", err)
	}
	receipt := control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        "session-one",
		ProfileID: p.ProfileID,
		TargetID:  p.Target.TargetID,
		StartedAt: "2026-05-15T00:00:00Z",
		Status:    "published",
	}
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, "session-one")
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	if err := control.WriteFile(receiptPath, receipt); err != nil {
		t.Fatalf("control.WriteFile(receipt) error = %v, want nil", err)
	}
	manifestPath, err := control.Path(target, control.ArtifactManifest, "session-one")
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v, want nil", err)
	}
	legacy := `{"version":1,"id":"manifest-session-one","session_id":"session-one","created_at":"2026-05-15T00:00:00Z","entries":[{"path":"deleted-link","kind":"symlink","target_path":"deleted-link"},{"path":"keep.txt","kind":"file","size":4,"target_path":"keep.txt"}]}`
	if err := os.WriteFile(manifestPath, []byte(legacy), 0o644); err != nil {
		t.Fatalf("os.WriteFile(legacy manifest) error = %v, want nil", err)
	}

	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Run(legacy symlink manifest) error = %v, want nil", err)
	}
	if got.Deleted != 1 {
		t.Fatalf("Run(legacy symlink manifest) deleted = %d, want 1", got.Deleted)
	}
}

func TestRunSkipsSoftDeletesFromDifferentProfile(t *testing.T) {
	dir := t.TempDir()
	sourceA := filepath.Join(dir, "source-a")
	sourceB := filepath.Join(dir, "source-b")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(sourceA, "old-a.txt"), "old", 0o644)
	mustWriteFile(t, filepath.Join(sourceB, "keep-b.txt"), "keep", 0o644)

	profileA := profile.NewDefault("profile-a", "Profile A", sourceA, target)
	if _, err := Run(Options{Profile: profileA, TargetDir: target, SessionID: "session-a", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("Run(profile-a) error = %v, want nil", err)
	}

	profileB := profile.NewDefault("profile-b", "Profile B", sourceB, target)
	got, err := Run(Options{Profile: profileB, TargetDir: target, SessionID: "session-b", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Run(profile-b) error = %v, want nil", err)
	}
	if got.Deleted != 0 {
		t.Fatalf("Run(profile-b) deleted = %d, want 0 because previous manifest belongs to a different profile", got.Deleted)
	}
}

func TestRunRefusesToOverwriteDifferentTargetFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "source", 0o644)
	mustWriteFile(t, filepath.Join(target, "file.txt"), "target", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
	if err == nil {
		t.Fatalf("Run(existing divergent target) error = nil, want overwrite refusal")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("Run(existing divergent target) error = %q, want refusing to overwrite", err.Error())
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "target" {
		t.Fatalf("target file after failed Run = (%q, %v), want unchanged target", string(got), err)
	}
}

func TestRunAllowsExistingIdenticalTargetFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "same", 0o600)
	mustWriteFile(t, filepath.Join(target, "file.txt"), "same", 0o600)
	sourceTime := time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), sourceTime, sourceTime); err != nil {
		t.Fatalf("os.Chtimes(source file) error = %v, want nil", err)
	}
	targetTime := time.Date(2026, 5, 15, 1, 2, 3, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(target, "file.txt"), targetTime, targetTime); err != nil {
		t.Fatalf("os.Chtimes(target file) error = %v, want nil", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)

	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"}); err != nil {
		t.Fatalf("Run(existing identical target) error = %v, want nil", err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "same" {
		t.Fatalf("target file after Run = (%q, %v), want same", string(got), err)
	}
	info, err := os.Stat(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("os.Stat(target file) error = %v, want nil", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("target mode after idempotent Run = %v, want existing 0600", info.Mode().Perm())
	}
	if !info.ModTime().Equal(targetTime) {
		t.Fatalf("target mtime after idempotent Run = %v, want existing %v", info.ModTime(), targetTime)
	}
}

func TestRunDoesNotMutateExistingTargetDirectoryMetadata(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "nested", "file.txt"), "payload", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	targetDir := filepath.Join(target, "nested")
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", targetDir, err)
	}
	oldTime := time.Date(2026, 5, 15, 1, 2, 3, 0, time.UTC)
	if err := os.Chtimes(targetDir, oldTime, oldTime); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", targetDir, err)
	}

	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"}); err != nil {
		t.Fatalf("Run(existing target directory) error = %v, want nil", err)
	}
	info, err := os.Stat(targetDir)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", targetDir, err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("target directory mode after Run = %v, want existing 0700 unchanged", info.Mode().Perm())
	}
	if !info.ModTime().Equal(oldTime) {
		t.Fatalf("target directory mtime after Run = %v, want existing %v unchanged", info.ModTime(), oldTime)
	}
}

func TestValidateProfileForLocalPushRejectsUnsupportedPolicyToggles(t *testing.T) {
	dir := t.TempDir()
	p := profile.NewDefault("profile-local", "Local profile", filepath.Join(dir, "source"), filepath.Join(dir, "target"))

	tests := []struct {
		name string
		edit func(*profile.Profile)
		want string
	}{
		{name: "hidden files disabled", edit: func(p *profile.Profile) { p.PrivacyPolicy.AllowHiddenFiles = false }, want: "allow_hidden_files"},
		{name: "sensitive names disabled", edit: func(p *profile.Profile) { p.PrivacyPolicy.AllowSensitiveFilenames = false }, want: "allow_sensitive_filenames"},
		{name: "redacted privacy", edit: func(p *profile.Profile) { p.PrivacyPolicy.Mode = profile.PrivacyModeRedacted }, want: "privacy_policy.mode"},
		{name: "custom traffic level", edit: func(p *profile.Profile) { p.PrivacyPolicy.TrafficLevel = 1 }, want: "custom privacy_policy"},
		{name: "prune mode", edit: func(p *profile.Profile) { p.DeletePolicy.Mode = profile.DeleteModePrune }, want: "physical prune"},
		{name: "physical prune enabled", edit: func(p *profile.Profile) { p.DeletePolicy.AllowPhysicalPrune = true }, want: "physical prune"},
		{name: "live consistency", edit: func(p *profile.Profile) { p.Consistency = profile.ConsistencyLive }, want: "consistency"},
		{name: "snapshot consistency", edit: func(p *profile.Profile) { p.Consistency = profile.ConsistencySnapshot }, want: "consistency"},
		{name: "xattrs enabled", edit: func(p *profile.Profile) { p.MetadataPolicy.PreserveExtendedAttr = true }, want: "preserve_extended_attr"},
		{name: "permissions disabled", edit: func(p *profile.Profile) { p.MetadataPolicy.PreservePermissions = false }, want: "preserve_permissions"},
		{name: "modtime disabled", edit: func(p *profile.Profile) { p.MetadataPolicy.PreserveModTime = false }, want: "preserve_mod_time"},
		{name: "custom agent knowledge", edit: func(p *profile.Profile) {
			p.AgentKnowledge.Categories = append(p.AgentKnowledge.Categories, profile.KnowledgeCategory{Name: "custom", Manifest: true})
		}, want: "agent_knowledge"},
		{name: "multiple roots", edit: func(p *profile.Profile) {
			p.Roots = append(p.Roots, profile.Root{ID: "root2", Path: filepath.Join(dir, "source2")})
		}, want: "exactly one root"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := p
			tt.edit(&next)
			err := ValidateProfileForLocalPush(next)
			if err == nil {
				t.Fatalf("ValidateProfileForLocalPush(%s) error = nil, want unsupported policy error", tt.name)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateProfileForLocalPush(%s) error = %q, want substring %q", tt.name, err.Error(), tt.want)
			}
		})
	}
}

func TestRunDetectsDefaultAgentKnowledgeCategories(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, ".github", "copilot-instructions.md"), "copilot", 0o644)
	mustWriteFile(t, filepath.Join(source, ".claude.json"), "{}", 0o600)

	p := profile.NewDefault("profile-local", "Local profile", source, target)
	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-agentkb", Now: time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Run(default agent knowledge) error = %v, want nil", err)
	}
	if got.Influences != 2 {
		t.Fatalf("Run(default agent knowledge) influences = %d, want 2", got.Influences)
	}

	doc := readAgentInfluence(t, target, "session-agentkb")
	byPath := map[string]string{}
	for _, influence := range doc.Influence {
		byPath[influence.Path] = string(influence.Category)
	}
	if byPath[".github/copilot-instructions.md"] != "tool_project_rules" {
		t.Errorf("Run(default agent knowledge) copilot influence category = %q, want tool_project_rules", byPath[".github/copilot-instructions.md"])
	}
	if byPath[".claude.json"] != "home_memories" {
		t.Errorf("Run(default agent knowledge) claude influence category = %q, want home_memories", byPath[".claude.json"])
	}
}

type agentInfluenceDocument struct {
	Version   int                 `json:"version"`
	SessionID string              `json:"session_id"`
	CreatedAt string              `json:"created_at"`
	Influence []agentkb.Influence `json:"influence"`
}

func readAgentInfluence(t *testing.T, target string, sessionID string) agentInfluenceDocument {
	t.Helper()
	path := filepath.Join(target, control.DirName, "agent", sessionID+"-influence.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	var got agentInfluenceDocument
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v, want nil", path, err)
	}
	return got
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

func manifestContainsSymlinkTarget(manifest control.Manifest, path string, target string) bool {
	for _, entry := range manifest.Entries {
		if entry.Path == path && entry.Kind == "symlink" && entry.SymlinkTarget == target {
			return true
		}
	}
	return false
}

func manifestContainsPath(manifest control.Manifest, path string) bool {
	for _, entry := range manifest.Entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}

func readOnlyWarning(t *testing.T, target string) control.Warning {
	t.Helper()
	warningDir := filepath.Join(target, control.DirName, "warnings")
	entries, err := os.ReadDir(warningDir)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil", warningDir, err)
	}
	if len(entries) != 1 {
		t.Fatalf("os.ReadDir(%q) entries = %#v, want one warning", warningDir, entries)
	}
	warning, err := control.ReadFile[control.Warning](filepath.Join(warningDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("control.ReadFile(warning) error = %v, want nil", err)
	}
	return warning
}

func writeManifest(t *testing.T, target string, manifest control.Manifest) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactManifest, manifest.SessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest %q) error = %v, want nil", manifest.SessionID, err)
	}
	if err := control.WriteFile(path, manifest); err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", path, err)
	}
}

func writeReceipt(t *testing.T, target string, p profile.Profile, sessionID string, startedAt string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt %q) error = %v, want nil", sessionID, err)
	}
	receipt := control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: p.ProfileID,
		TargetID:  p.Target.TargetID,
		StartedAt: startedAt,
		Status:    "published",
	}
	if err := control.WriteFile(path, receipt); err != nil {
		t.Fatalf("control.WriteFile(%q, receipt) error = %v, want nil", path, err)
	}
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
