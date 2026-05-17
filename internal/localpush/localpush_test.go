package localpush

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/agentkb"
	"github.com/khicago/supermover/internal/audit"
	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/verify"
)

func setBeforeReadStableSymlinkHook(hook func(sourcePath string, entry scan.Entry) error) {
	beforeReadStableSymlink = hook
}

func setBeforePublishStagedHook(hook func(entry control.ManifestEntry, targetPath string) error) {
	beforePublishStaged = hook
}

func setBeforeManagedReplacePromoteHook(hook func(entry control.ManifestEntry, targetPath string) error) {
	beforeManagedReplacePromote = hook
}

func setBeforeManagedReplaceCurrentHoldHook(hook func(entry control.ManifestEntry, targetPath string, holdPath string) error) {
	beforeManagedReplaceCurrentHold = hook
}

func setAfterManagedReplaceHoldHook(hook func(entry control.ManifestEntry, targetPath string, holdPath string) error) {
	afterManagedReplaceHold = hook
}

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

func TestLatestPublishedManifestRejectsReceiptIDMismatch(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-one",
		SessionID: "one",
		RootID:    p.Roots[0].ID,
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "one.txt", Kind: "file", TargetPath: "one.txt"}},
	})
	writeReceipt(t, target, p, "one", "2026-05-16T00:00:00Z")
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, "one")
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	receipt := readControlDoc[control.SessionReceipt](t, target, control.ArtifactSessionReceipt, "one")
	receipt.ID = "other"
	if err := control.WriteFile(receiptPath, receipt); err != nil {
		t.Fatalf("control.WriteFile(receipt mismatch) error = %v, want nil", err)
	}

	if _, _, err := latestPublishedManifest(p, target); err == nil || !strings.Contains(err.Error(), "receipt") {
		t.Fatalf("latestPublishedManifest(receipt mismatch) error = %v, want receipt mismatch", err)
	}
}

func TestLatestPublishedManifestRejectsManifestSessionMismatch(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	manifest := control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-other",
		SessionID: "other",
		RootID:    p.Roots[0].ID,
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "one.txt", Kind: "file", TargetPath: "one.txt"}},
	}
	writeManifest(t, target, manifest)
	manifestPath, err := control.Path(target, control.ArtifactManifest, "one")
	if err != nil {
		t.Fatalf("control.Path(manifest one) error = %v, want nil", err)
	}
	otherPath, err := control.Path(target, control.ArtifactManifest, "other")
	if err != nil {
		t.Fatalf("control.Path(manifest other) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(manifest one parent) error = %v, want nil", err)
	}
	data, err := os.ReadFile(otherPath)
	if err != nil {
		t.Fatalf("os.ReadFile(manifest other) error = %v, want nil", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile(manifest one) error = %v, want nil", err)
	}
	writeReceipt(t, target, p, "one", "2026-05-16T00:00:00Z")

	if _, _, err := latestPublishedManifest(p, target); err == nil || !strings.Contains(err.Error(), "manifest") {
		t.Fatalf("latestPublishedManifest(manifest mismatch) error = %v, want manifest mismatch", err)
	}
}

func TestRunPreflightsAllTargetsBeforePublish(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "a.txt"), "a", 0o644)
	mustWriteFile(t, filepath.Join(source, "z.txt"), "z", 0o644)
	mustWriteFile(t, filepath.Join(target, "z.txt"), "different", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
	if err == nil || !strings.Contains(err.Error(), "different content") {
		t.Fatalf("Run(preflight target conflict) error = %v, want different content error", err)
	}
	if _, err := os.Stat(filepath.Join(target, "a.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(a.txt after failed preflight) error = %v, want os.ErrNotExist", err)
	}
}

func TestPreflightAndRunRefuseDivergentTargetFile(t *testing.T) {
	tests := []struct {
		name string
		push func(Options) (Result, error)
	}{
		{name: "preflight", push: Preflight},
		{name: "run", push: Run},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			target := filepath.Join(dir, "target")
			mustWriteFile(t, filepath.Join(source, "file.txt"), "source", 0o644)
			mustWriteFile(t, filepath.Join(target, "file.txt"), "target", 0o600)
			p := profile.NewDefault("profile-local", "Local profile", source, target)

			_, err := tt.push(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
			if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
				t.Fatalf("%s(divergent target) error = %v, want overwrite refusal", tt.name, err)
			}
			got, err := os.ReadFile(filepath.Join(target, "file.txt"))
			if err != nil {
				t.Fatalf("os.ReadFile(target after failed %s) error = %v, want nil", tt.name, err)
			}
			if string(got) != "target" {
				t.Fatalf("target after failed %s = %q, want original target", tt.name, string(got))
			}
			info, err := os.Stat(filepath.Join(target, "file.txt"))
			if err != nil {
				t.Fatalf("os.Stat(target after failed %s) error = %v, want nil", tt.name, err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("target mode after failed %s = %v, want original 0600", tt.name, info.Mode().Perm())
			}
			if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "session-test", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("os.Stat(receipt after failed %s) error = %v, want os.ErrNotExist", tt.name, err)
			}
		})
	}
}

func TestEnsureDirectoryPreflightClassifiesExistingTarget(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	if err := ensureDirectoryPreflight(missing); err != nil {
		t.Fatalf("ensureDirectoryPreflight(%q) error = %v, want nil for missing target", missing, err)
	}

	plainDir := filepath.Join(dir, "plain")
	if err := os.Mkdir(plainDir, 0o755); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", plainDir, err)
	}
	if err := ensureDirectoryPreflight(plainDir); err != nil {
		t.Fatalf("ensureDirectoryPreflight(%q) error = %v, want nil for plain directory", plainDir, err)
	}

	filePath := filepath.Join(dir, "file")
	if err := os.WriteFile(filePath, []byte("file"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", filePath, err)
	}
	if err := ensureDirectoryPreflight(filePath); err == nil || !strings.Contains(err.Error(), "non-directory") {
		t.Fatalf("ensureDirectoryPreflight(%q) error = %v, want non-directory refusal", filePath, err)
	}

	linkPath := filepath.Join(dir, "link")
	if err := os.Symlink(plainDir, linkPath); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	if err := ensureDirectoryPreflight(linkPath); err == nil || !strings.Contains(err.Error(), "non-directory") {
		t.Fatalf("ensureDirectoryPreflight(%q) error = %v, want symlink refusal", linkPath, err)
	}
}

func TestSymlinkTargetStateClassifiesTargets(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	same, exists, err := symlinkTargetState(missing, "target.txt")
	if err != nil || same || exists {
		t.Fatalf("symlinkTargetState(%q, target.txt) = same=%t exists=%t err=%v, want missing false/false/nil", missing, same, exists, err)
	}

	linkPath := filepath.Join(dir, "link")
	if err := os.Symlink("target.txt", linkPath); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	same, exists, err = symlinkTargetState(linkPath, "target.txt")
	if err != nil || !same || !exists {
		t.Fatalf("symlinkTargetState(%q, target.txt) = same=%t exists=%t err=%v, want same existing symlink", linkPath, same, exists, err)
	}
	same, exists, err = symlinkTargetState(linkPath, "other.txt")
	if err != nil || same || !exists {
		t.Fatalf("symlinkTargetState(%q, other.txt) = same=%t exists=%t err=%v, want different existing symlink", linkPath, same, exists, err)
	}

	filePath := filepath.Join(dir, "file")
	if err := os.WriteFile(filePath, []byte("file"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", filePath, err)
	}
	same, exists, err = symlinkTargetState(filePath, "target.txt")
	if !errors.Is(err, errSymlinkTargetConflict) || same || !exists {
		t.Fatalf("symlinkTargetState(%q, target.txt) = same=%t exists=%t err=%v, want conflict on non-symlink", filePath, same, exists, err)
	}
}

func TestRunRejectsNormalizedReservedControlPlaneTargetPathBeforePublish(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "payload.json"), "payload", 0o644)
	layout := transaction.NewLayout(control.ControlDir(target))
	entries := []control.ManifestEntry{
		{Path: "payload.json", TargetPath: "safe/../.supermover/sessions/forged/receipt.json", Kind: "file", Mode: 0o644, Size: 7, Digest: "sha256:239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"},
	}
	stagePath, err := pathguard.SafeJoinParent(layout.StagingDir("session-test"), "payload.json")
	if err != nil {
		t.Fatalf("pathguard.SafeJoinParent(stage) error = %v, want nil", err)
	}
	mustWriteFile(t, stagePath, "payload", 0o644)

	if err := preflightPublishPlan(layout, target, "session-test", entries, publishModeRun); err == nil || !strings.Contains(err.Error(), "reserved control") {
		t.Fatalf("preflightPublishPlan(normalized reserved path) error = %v, want reserved control error", err)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "forged", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(forged receipt) error = %v, want os.ErrNotExist", err)
	}
}

func TestPreflightPublishPlanRejectsFileTargetWithDescendant(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	layout := transaction.NewLayout(control.ControlDir(target))
	entries := []control.ManifestEntry{
		{Path: "first.txt", TargetPath: "a", Kind: "file", Mode: 0o644, Size: 1, Digest: "sha256:ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"},
		{Path: "second.txt", TargetPath: "a/b", Kind: "file", Mode: 0o644, Size: 1, Digest: "sha256:3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d"},
	}
	first, err := pathguard.SafeJoinParent(layout.StagingDir("session-test"), "first.txt")
	if err != nil {
		t.Fatalf("pathguard.SafeJoinParent(first) error = %v, want nil", err)
	}
	second, err := pathguard.SafeJoinParent(layout.StagingDir("session-test"), "second.txt")
	if err != nil {
		t.Fatalf("pathguard.SafeJoinParent(second) error = %v, want nil", err)
	}
	mustWriteFile(t, first, "a", 0o644)
	mustWriteFile(t, second, "b", 0o644)

	if err := preflightPublishPlan(layout, target, "session-test", entries, publishModeRun); err == nil || !strings.Contains(err.Error(), "non-directory") {
		t.Fatalf("preflightPublishPlan(file target with descendant) error = %v, want non-directory target error", err)
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

func TestRunRejectsChangedSymlinkBeforePublish(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "real.txt"), "real", 0o644)
	link := filepath.Join(source, "link.txt")
	if err := os.Symlink("real.txt", link); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	setBeforeReadStableSymlinkHook(func(sourcePath string, entry scan.Entry) error {
		if entry.Path != "link.txt" {
			return nil
		}
		if err := os.Remove(sourcePath); err != nil {
			return err
		}
		return os.Symlink("other.txt", sourcePath)
	})
	t.Cleanup(func() { setBeforeReadStableSymlinkHook(nil) })

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-test"})
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("Run(changed symlink) error = %v, want source changed error", err)
	}
	if _, err := control.ReadFile[control.SessionReceipt](filepath.Join(target, control.DirName, "sessions", "session-test", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("control.ReadFile(receipt after changed symlink) error = %v, want os.ErrNotExist", err)
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

func TestCopyRegularToStageRejectsChangedSourceWithoutPublishingStage(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	stage := filepath.Join(dir, "stage", "file.txt")
	stamp := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)
	mustWriteFile(t, source, "trusted", 0o640)
	if err := os.Chtimes(source, stamp, stamp); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", source, err)
	}
	entry := scan.Entry{
		Path:    "file.txt",
		Kind:    scan.KindRegular,
		Size:    int64(len("trusted")),
		Mode:    0o640,
		ModTime: stamp,
	}

	_, err := copyRegularToStageWithPostCopy(source, stage, entry, func() error {
		next := stamp.Add(time.Second)
		if err := os.WriteFile(source, []byte("changed"), 0o640); err != nil {
			return err
		}
		return os.Chtimes(source, next, next)
	})
	if err == nil || !strings.Contains(err.Error(), "changed during copy") {
		t.Fatalf("copyRegularToStageWithPostCopy(changed source) error = %v, want changed during copy", err)
	}
	if _, err := os.Stat(stage); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(staged payload after changed source) error = %v, want os.ErrNotExist", err)
	}
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(stage), ".supermover-*.tmp"))
	if err != nil {
		t.Fatalf("filepath.Glob(stage temp files) error = %v, want nil", err)
	}
	if len(temps) != 0 {
		t.Fatalf("stage temp files after changed source = %#v, want none", temps)
	}
}

func TestCopyRegularToStageRejectsSameMetadataContentChange(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	stage := filepath.Join(dir, "stage", "file.txt")
	stamp := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)
	mustWriteFile(t, source, "trusted", 0o640)
	if err := os.Chtimes(source, stamp, stamp); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", source, err)
	}
	entry := scan.Entry{
		Path:    "file.txt",
		Kind:    scan.KindRegular,
		Size:    int64(len("trusted")),
		Mode:    0o640,
		ModTime: stamp,
	}

	_, err := copyRegularToStageWithPostCopy(source, stage, entry, func() error {
		if err := os.WriteFile(source, []byte("changed"), 0o640); err != nil {
			return err
		}
		return os.Chtimes(source, stamp, stamp)
	})
	if err == nil || !strings.Contains(err.Error(), "changed during copy") {
		t.Fatalf("copyRegularToStageWithPostCopy(same metadata changed source) error = %v, want changed during copy", err)
	}
	if _, err := os.Stat(stage); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(staged payload after changed source) error = %v, want os.ErrNotExist", err)
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

func TestCopyRegularRejectsSameMetadataContentChangeSinceScan(t *testing.T) {
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
	if err := os.WriteFile(source, []byte("new"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", source, err)
	}
	if err := os.Chtimes(source, stamp, stamp); err != nil {
		t.Fatalf("os.Chtimes(%q) after rewrite error = %v, want nil", source, err)
	}
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", target, err)
	}

	_, err = copyRegularToStage(source, target+".stage", entry)
	if err == nil {
		t.Fatalf("copyRegularToStage(same metadata changed since scan) error = nil, want source changed error")
	}
	if !strings.Contains(err.Error(), "changed since scan") {
		t.Fatalf("copyRegularToStage(same metadata changed since scan) error = %q, want source changed error", err.Error())
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

func TestRecoverRejectsStagedSessionWithMismatchedProfileSnapshot(t *testing.T) {
	tests := []struct {
		name string
		edit func(*control.ProfileSnapshot)
		want string
	}{
		{
			name: "snapshot profile id mismatch",
			edit: func(snapshot *control.ProfileSnapshot) {
				snapshot.ProfileID = "other-profile"
			},
			want: "profile_id",
		},
		{
			name: "embedded target id mismatch",
			edit: func(snapshot *control.ProfileSnapshot) {
				var embedded profile.Profile
				if err := json.Unmarshal(snapshot.Profile, &embedded); err != nil {
					panic(err)
				}
				embedded.Target.TargetID = "other-target"
				data, err := json.Marshal(embedded)
				if err != nil {
					panic(err)
				}
				snapshot.Profile = data
			},
			want: "target_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, target, p, sessionID := prepareStagedRecoverySession(t)
			snapshotPath, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-"+sessionID)
			if err != nil {
				t.Fatalf("control.Path(profile snapshot) error = %v, want nil", err)
			}
			snapshot := readControlDoc[control.ProfileSnapshot](t, target, control.ArtifactProfileSnapshot, "profile-"+sessionID)
			tt.edit(&snapshot)
			if err := control.WriteFile(snapshotPath, snapshot); err != nil {
				t.Fatalf("control.WriteFile(%q, edited snapshot) error = %v, want nil", snapshotPath, err)
			}

			result, err := Recover(RecoverOptions{
				Profile:   p,
				TargetDir: target,
				SessionID: sessionID,
				Now:       time.Date(2026, 5, 16, 5, 0, 0, 0, time.UTC),
			})
			if err != nil {
				t.Fatalf("Recover(%s) error = %v, want nil result with needs_repair", tt.name, err)
			}
			if result.RepairNeeded != 1 || result.Recovered != 0 || len(result.Items) != 1 {
				t.Fatalf("Recover(%s) result = %+v, want one repair item and no recovered sessions", tt.name, result)
			}
			if result.Items[0].Status != "needs_repair" || !strings.Contains(result.Items[0].Message, tt.want) {
				t.Fatalf("Recover(%s) item = %+v, want needs_repair message containing %q", tt.name, result.Items[0], tt.want)
			}
			if _, err := os.Stat(filepath.Join(target, "file.txt")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("os.Stat(published target after rejected recovery) error = %v, want os.ErrNotExist", err)
			}
			if _, err := os.Stat(filepath.Join(source, "file.txt")); err != nil {
				t.Fatalf("os.Stat(source after rejected recovery) error = %v, want source retained", err)
			}
			record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(target)).RecordPath(sessionID))
			if err != nil {
				t.Fatalf("ReadSessionRecord(%q) error = %v, want nil", sessionID, err)
			}
			if record.State != transaction.StateNeedsRepair {
				t.Fatalf("session state after rejected recovery = %q, want %q", record.State, transaction.StateNeedsRepair)
			}
		})
	}
}

func TestRunRefusesExistingRecoveryStateBeforeCopying(t *testing.T) {
	tests := []struct {
		name  string
		state transaction.State
	}{
		{name: "received", state: transaction.StateReceived},
		{name: "staged", state: transaction.StateStaged},
		{name: "needs repair", state: transaction.StateNeedsRepair},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			target := filepath.Join(dir, "target")
			mustWriteFile(t, filepath.Join(source, "new.txt"), "new", 0o644)
			p := profile.NewDefault("profile-local", "Local profile", source, target)
			layout := transaction.NewLayout(control.ControlDir(target))
			record, err := transaction.NewSessionRecord("session-old", time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatalf("transaction.NewSessionRecord() error = %v, want nil", err)
			}
			record, err = record.WithState(tt.state, time.Date(2026, 5, 16, 1, 1, 0, 0, time.UTC))
			if err != nil {
				t.Fatalf("SessionRecord.WithState(%q) error = %v, want nil", tt.state, err)
			}
			if err := layout.WriteSessionRecord(record); err != nil {
				t.Fatalf("Layout.WriteSessionRecord(%+v) error = %v, want nil", record, err)
			}

			_, err = Run(Options{Profile: p, TargetDir: target, SessionID: "session-new", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
			if err == nil {
				t.Fatalf("Run(existing %s recovery state) error = nil, want refusal", tt.name)
			}
			if !strings.Contains(err.Error(), "health") || !strings.Contains(err.Error(), "recover") {
				t.Fatalf("Run(existing %s recovery state) error = %v, want health/recover guidance", tt.name, err)
			}
			if _, err := os.Stat(filepath.Join(target, "new.txt")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("os.Stat(target data after refusal) error = %v, want os.ErrNotExist", err)
			}
		})
	}
}

func TestRunRefusesOrphanedSessionDirectoryBeforeCopying(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "new.txt"), "new", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	orphanedStage := filepath.Join(control.ControlDir(target), "sessions", "session-orphan", "stage")
	if err := os.MkdirAll(orphanedStage, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", orphanedStage, err)
	}

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-new", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	if err == nil {
		t.Fatalf("Run(orphaned session directory) error = nil, want invalid recovery refusal")
	}
	if !strings.Contains(err.Error(), "invalid recovery state") {
		t.Fatalf("Run(orphaned session directory) error = %v, want invalid recovery state", err)
	}
	if _, err := os.Stat(filepath.Join(target, "new.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(target data after refusal) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunRefusesLegacyPublishedSessionFromDifferentScopeBeforeCopying(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "new.txt"), "new", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	other := profile.NewDefault("profile-other", "Other profile", source, target)
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session-legacy",
		SessionID: "session-legacy",
		RootID:    p.Roots[0].ID,
		CreatedAt: "2026-05-15T00:00:00Z",
	})
	writeReceipt(t, target, other, "session-legacy", "2026-05-15T00:00:00Z")

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-new", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	if err == nil {
		t.Fatalf("Run(legacy session different scope) error = nil, want invalid recovery refusal")
	}
	if !strings.Contains(err.Error(), "invalid recovery state") {
		t.Fatalf("Run(legacy session different scope) error = %v, want invalid recovery state", err)
	}
	if _, err := os.Stat(filepath.Join(target, "new.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(target data after different-scope legacy refusal) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunAllowsLegacyPublishedSessionForSameScope(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "new.txt"), "new", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session-legacy",
		SessionID: "session-legacy",
		RootID:    p.Roots[0].ID,
		CreatedAt: "2026-05-15T00:00:00Z",
	})
	writeReceipt(t, target, p, "session-legacy", "2026-05-15T00:00:00Z")

	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-new", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("Run(same-scope legacy published session) error = %v, want nil", err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "new.txt")); err != nil || string(got) != "new" {
		t.Fatalf("target data after same-scope legacy run = (%q, %v), want new", string(got), err)
	}
}

func TestRunReplacesChangedManagedTargetFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	oldTime := time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC)
	newTime := time.Date(2026, 5, 16, 4, 5, 6, 0, time.UTC)
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), oldTime, oldTime); err != nil {
		t.Fatalf("os.Chtimes(source old file) error = %v, want nil", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	previousManifest := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-one")
	previousEntry := manifestEntryByPath(t, previousManifest, "file.txt")
	if previousEntry.Digest != testDigest("old") {
		t.Fatalf("session-one digest = %q, want digest for old content", previousEntry.Digest)
	}

	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new content", 0o600)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), newTime, newTime); err != nil {
		t.Fatalf("os.Chtimes(source new file) error = %v, want nil", err)
	}
	preflight, err := Preflight(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Preflight(changed managed file) error = %v, want nil", err)
	}
	if preflight.Copied != 1 {
		t.Fatalf("Preflight(changed managed file).Copied = %d, want 1", preflight.Copied)
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "old" {
		t.Fatalf("target after Preflight = (%q, %v), want old", string(got), err)
	}

	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 4, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("second Run(changed managed file) error = %v, want nil", err)
	}
	if got.Copied != 1 {
		t.Fatalf("second Run(changed managed file).Copied = %d, want 1", got.Copied)
	}
	bytes, err := os.ReadFile(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile(target changed file) error = %v, want nil", err)
	}
	if string(bytes) != "new content" {
		t.Fatalf("target changed file = %q, want new content", string(bytes))
	}
	info, err := os.Stat(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("os.Stat(target changed file) error = %v, want nil", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("target changed file mode = %v, want 0600", info.Mode().Perm())
	}
	if !info.ModTime().Equal(newTime) {
		t.Fatalf("target changed file mtime = %v, want %v", info.ModTime(), newTime)
	}
	manifest := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-two")
	entry := manifestEntryByPath(t, manifest, "file.txt")
	if entry.Size != int64(len("new content")) || !strings.HasPrefix(entry.Digest, "sha256:") {
		t.Fatalf("session-two manifest entry = %#v, want new size and digest", entry)
	}
	if entry.Digest != testDigest("new content") {
		t.Fatalf("session-two digest = %q, want digest for new content", entry.Digest)
	}
	if entry.PreviousSessionID != "session-one" || entry.PreviousManifestID != "manifest-session-one" || !strings.HasPrefix(entry.PreviousDigest, "sha256:") {
		t.Fatalf("session-two manifest entry = %#v, want previous evidence", entry)
	}
	if entry.PreviousSize != previousEntry.Size || entry.PreviousDigest != previousEntry.Digest || entry.PreviousMode != previousEntry.Mode || entry.PreviousModTime != previousEntry.ModTime {
		t.Fatalf("session-two previous evidence = %#v, want previous entry %#v", entry, previousEntry)
	}
	if entry.Digest == entry.PreviousDigest {
		t.Fatalf("session-two digest = previous digest %q, want changed file evidence", entry.Digest)
	}
	holdPath, err := replacementHoldPath(target, "session-two", "previous", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath() error = %v, want nil", err)
	}
	if _, err := os.Stat(holdPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(successful replacement hold) error = %v, want os.ErrNotExist", err)
	}
	verifyReport, err := verify.BuildReport(verify.Options{TargetRoot: target, SessionID: "session-two", ProfileID: p.ProfileID, TargetID: p.Target.TargetID})
	if err != nil {
		t.Fatalf("verify.BuildReport(session-two) error = %v, want nil", err)
	}
	if len(verifyReport.Findings) != 0 || verifyReport.Summary.FilesVerified != 1 {
		t.Fatalf("verify.BuildReport(session-two) findings=%#v summary=%+v, want one verified file", verifyReport.Findings, verifyReport.Summary)
	}
}

func TestRunUpdatesManagedFileWhenOnlyMetadataChanges(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	oldTime := time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC)
	newTime := time.Date(2026, 5, 16, 4, 5, 6, 0, time.UTC)
	mustWriteFile(t, filepath.Join(source, "file.txt"), "same content", 0o644)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), oldTime, oldTime); err != nil {
		t.Fatalf("os.Chtimes(source old file) error = %v, want nil", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}

	mustReplaceFile(t, filepath.Join(source, "file.txt"), "same content", 0o600)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), newTime, newTime); err != nil {
		t.Fatalf("os.Chtimes(source new metadata) error = %v, want nil", err)
	}
	preflight, err := Preflight(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("Preflight(metadata-only changed managed file) error = %v, want nil", err)
	}
	if preflight.Copied != 1 {
		t.Fatalf("Preflight(metadata-only changed managed file).Copied = %d, want 1", preflight.Copied)
	}
	info, err := os.Stat(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("os.Stat(target after metadata preflight) error = %v, want nil", err)
	}
	if info.Mode().Perm() != 0o644 || !info.ModTime().Equal(oldTime) {
		t.Fatalf("target metadata after preflight = mode %v mtime %v, want old metadata", info.Mode().Perm(), info.ModTime())
	}

	got, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 4, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("second Run(metadata-only changed managed file) error = %v, want nil", err)
	}
	if got.Copied != 1 {
		t.Fatalf("second Run(metadata-only changed managed file).Copied = %d, want 1", got.Copied)
	}
	if bytes, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(bytes) != "same content" {
		t.Fatalf("target metadata-only file = (%q, %v), want same content", string(bytes), err)
	}
	info, err = os.Stat(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("os.Stat(target metadata-only file) error = %v, want nil", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("target metadata-only file mode = %v, want 0600", info.Mode().Perm())
	}
	if !info.ModTime().Equal(newTime) {
		t.Fatalf("target metadata-only file mtime = %v, want %v", info.ModTime(), newTime)
	}
	manifest := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-two")
	entry := manifestEntryByPath(t, manifest, "file.txt")
	if entry.PreviousSessionID != "session-one" || entry.PreviousDigest != entry.Digest {
		t.Fatalf("metadata-only manifest entry = %#v, want previous evidence with same digest", entry)
	}
}

func TestPublishStagedPreservesExplicitZeroModeFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	layout := transaction.NewLayout(control.ControlDir(target))
	sessionID := "session-zero-mode"
	stagePath, err := pathguard.SafeJoinParent(layout.StagingDir(sessionID), "secret.txt")
	if err != nil {
		t.Fatalf("pathguard.SafeJoinParent(stage, secret.txt) error = %v, want nil", err)
	}
	mustWriteFile(t, stagePath, "secret", 0o644)
	modTime := time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)
	entry := control.ManifestEntry{
		Path:       "secret.txt",
		Kind:       "file",
		TargetPath: "secret.txt",
		ModTime:    modTime.Format(time.RFC3339Nano),
		Digest:     testDigest("secret"),
	}
	entry.SetModeEvidence(0)
	entry.SetSizeEvidence(int64(len("secret")))

	if err := publishStaged(layout, target, sessionID, []control.ManifestEntry{entry}, nil, publishModeRun); err != nil {
		t.Fatalf("publishStaged(explicit zero mode) error = %v, want nil", err)
	}
	info, err := os.Stat(filepath.Join(target, "secret.txt"))
	if err != nil {
		t.Fatalf("os.Stat(target explicit zero mode) error = %v, want nil", err)
	}
	if info.Mode().Perm() != 0 {
		t.Fatalf("target explicit zero mode = %v, want 0000", info.Mode().Perm())
	}
	if !info.ModTime().Equal(modTime) {
		t.Fatalf("target explicit zero mode mtime = %v, want %v", info.ModTime(), modTime)
	}
}

func TestRunReplacesChangedHiddenManagedTargetFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, ".env"), "TOKEN=old", 0o600)
	mustWriteFile(t, filepath.Join(source, ".config", "settings.json"), `{"mode":"old"}`, 0o640)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run(hidden file) error = %v, want nil", err)
	}

	mustReplaceFile(t, filepath.Join(source, ".env"), "TOKEN=new", 0o600)
	mustReplaceFile(t, filepath.Join(source, ".config", "settings.json"), `{"mode":"new"}`, 0o640)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("second Run(hidden changed file) error = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(target, ".env"))
	if err != nil {
		t.Fatalf("os.ReadFile(target hidden file) error = %v, want nil", err)
	}
	if string(got) != "TOKEN=new" {
		t.Fatalf("target hidden file = %q, want TOKEN=new", string(got))
	}
	got, err = os.ReadFile(filepath.Join(target, ".config", "settings.json"))
	if err != nil {
		t.Fatalf("os.ReadFile(target hidden directory child) error = %v, want nil", err)
	}
	if string(got) != `{"mode":"new"}` {
		t.Fatalf("target hidden directory child = %q, want new settings", string(got))
	}
	manifest := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-two")
	entry := manifestEntryByPath(t, manifest, ".config/settings.json")
	if entry.PreviousSessionID != "session-one" || !strings.HasPrefix(entry.PreviousDigest, "sha256:") {
		t.Fatalf("hidden directory child manifest entry = %#v, want previous evidence", entry)
	}
}

func TestPreflightAndRunRefuseChangedManagedFileAfterTargetDrift(t *testing.T) {
	tests := []struct {
		name  string
		drift func(t *testing.T, path string)
		want  string
	}{
		{name: "content", drift: func(t *testing.T, path string) {
			t.Helper()
			mustReplaceFile(t, path, "manual target edit", 0o600)
		}, want: "manual target edit"},
		{name: "delete", drift: func(t *testing.T, path string) {
			t.Helper()
			if err := os.Remove(path); err != nil {
				t.Fatalf("os.Remove(%q) error = %v, want nil", path, err)
			}
		}},
		{name: "mode", drift: func(t *testing.T, path string) {
			t.Helper()
			if err := os.Chmod(path, 0o600); err != nil {
				t.Fatalf("os.Chmod(%q) error = %v, want nil", path, err)
			}
		}, want: "old"},
		{name: "mtime", drift: func(t *testing.T, path string) {
			t.Helper()
			driftTime := time.Date(2026, 5, 16, 9, 0, 0, 0, time.UTC)
			if err := os.Chtimes(path, driftTime, driftTime); err != nil {
				t.Fatalf("os.Chtimes(%q) error = %v, want nil", path, err)
			}
		}, want: "old"},
		{name: "external replacement", drift: func(t *testing.T, path string) {
			t.Helper()
			mustReplaceFile(t, path, "new source", 0o644)
		}, want: "new source"},
	}
	pushes := []struct {
		name string
		push func(Options) (Result, error)
	}{
		{name: "preflight", push: Preflight},
		{name: "run", push: Run},
	}

	for _, drift := range tests {
		for _, push := range pushes {
			t.Run(drift.name+"/"+push.name, func(t *testing.T) {
				dir := t.TempDir()
				source := filepath.Join(dir, "source")
				target := filepath.Join(dir, "target")
				mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
				oldTime := time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC)
				if err := os.Chtimes(filepath.Join(source, "file.txt"), oldTime, oldTime); err != nil {
					t.Fatalf("os.Chtimes(source old file) error = %v, want nil", err)
				}
				p := profile.NewDefault("profile-local", "Local profile", source, target)
				if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
					t.Fatalf("first Run(%s/%s) error = %v, want nil", drift.name, push.name, err)
				}
				drift.drift(t, filepath.Join(target, "file.txt"))
				mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

				_, err := push.push(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
				if err == nil || !strings.Contains(err.Error(), "refusing") {
					t.Fatalf("%s(changed source with target drift %s) error = %v, want refusal", push.name, drift.name, err)
				}
				got, err := os.ReadFile(filepath.Join(target, "file.txt"))
				if drift.want == "" {
					if !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("os.ReadFile(target after failed %s/%s) error = %v, want os.ErrNotExist", drift.name, push.name, err)
					}
				} else if err != nil || string(got) != drift.want {
					t.Fatalf("target after failed %s/%s = (%q, %v), want %q", drift.name, push.name, string(got), err, drift.want)
				}
				if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "session-two", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("os.Stat(receipt after failed %s/%s) error = %v, want os.ErrNotExist", drift.name, push.name, err)
				}
			})
		}
	}
}

func TestRunRefusesManagedReplacementWhenTargetChangesBeforePublish(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

	triggered := false
	setBeforePublishStagedHook(func(entry control.ManifestEntry, targetPath string) error {
		if entry.Path != "file.txt" || triggered {
			return nil
		}
		triggered = true
		mustReplaceFile(t, targetPath, "manual target edit", 0o644)
		return nil
	})
	t.Cleanup(func() { setBeforePublishStagedHook(nil) })

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setBeforePublishStagedHook(nil)
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("Run(changed target before publish) error = %v, want overwrite refusal", err)
	}
	if !triggered {
		t.Fatalf("beforePublishStaged was not triggered")
	}
	got, err := os.ReadFile(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile(target after refused publish) error = %v, want nil", err)
	}
	if string(got) != "manual target edit" {
		t.Fatalf("target after refused publish = %q, want manual target edit", string(got))
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "session-two", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(receipt after refused publish) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunRefusesManagedReplacementWhenTargetChangesAfterFirstPreviousCheck(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

	triggered := false
	setBeforeManagedReplacePromoteHook(func(entry control.ManifestEntry, targetPath string) error {
		if entry.Path != "file.txt" || triggered {
			return nil
		}
		triggered = true
		mustReplaceFile(t, targetPath, "manual target edit", 0o644)
		return nil
	})
	t.Cleanup(func() { setBeforeManagedReplacePromoteHook(nil) })

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setBeforeManagedReplacePromoteHook(nil)
	if err == nil || !errors.Is(err, errManagedReplaceTargetChanged) {
		t.Fatalf("Run(target changed after managed replace check) error = %v, want %v", err, errManagedReplaceTargetChanged)
	}
	if !triggered {
		t.Fatalf("beforeManagedReplacePromote was not triggered")
	}
	got, err := os.ReadFile(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile(target after refused managed replace) error = %v, want nil", err)
	}
	if string(got) != "manual target edit" {
		t.Fatalf("target after refused managed replace = %q, want manual target edit", string(got))
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "session-two", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(receipt after refused managed replace) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunRetainsReplacementHoldWhenTargetAppearsAfterHold(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

	triggered := false
	setAfterManagedReplaceHoldHook(func(entry control.ManifestEntry, targetPath string, _ string) error {
		if entry.Path != "file.txt" || triggered {
			return nil
		}
		triggered = true
		mustWriteFile(t, targetPath, "external winner", 0o644)
		return nil
	})
	t.Cleanup(func() { setAfterManagedReplaceHoldHook(nil) })

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setAfterManagedReplaceHoldHook(nil)
	if err == nil || !strings.Contains(err.Error(), "without replace") {
		t.Fatalf("Run(target appeared after replacement hold) error = %v, want no-replace publish refusal", err)
	}
	if !triggered {
		t.Fatalf("afterManagedReplaceHold was not triggered")
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "external winner" {
		t.Fatalf("target after refused managed replace = (%q, %v), want external winner", string(got), err)
	}
	holdPath, err := replacementHoldPath(target, "session-two", "previous", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath() error = %v, want nil", err)
	}
	if got, err := os.ReadFile(holdPath); err != nil || string(got) != "old" {
		t.Fatalf("replacement hold = (%q, %v), want old", string(got), err)
	}
	currentHoldPath, err := replacementHoldPath(target, "session-two", "current", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath(current) error = %v, want nil", err)
	}
	if got, err := os.ReadFile(currentHoldPath); err != nil || string(got) != "old" {
		t.Fatalf("current replacement hold = (%q, %v), want old", string(got), err)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "session-two", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(receipt after refused managed replace) error = %v, want os.ErrNotExist", err)
	}

	recovered, err := Recover(RecoverOptions{
		Profile:   p,
		TargetDir: target,
		SessionID: "session-two",
		Now:       time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Recover(target appeared after replacement hold) error = %v, want nil result with needs_repair", err)
	}
	if recovered.RepairNeeded != 1 || recovered.Recovered != 0 {
		t.Fatalf("Recover(target appeared after replacement hold) result = %+v, want needs_repair", recovered)
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "external winner" {
		t.Fatalf("target after refused recover = (%q, %v), want external winner", string(got), err)
	}
	if got, err := os.ReadFile(holdPath); err != nil || string(got) != "old" {
		t.Fatalf("replacement hold after refused recover = (%q, %v), want old", string(got), err)
	}
	if got, err := os.ReadFile(currentHoldPath); err != nil || string(got) != "old" {
		t.Fatalf("current replacement hold after refused recover = (%q, %v), want old", string(got), err)
	}
}

func TestRunMovesExternalReplacementToCurrentHoldWhenTargetChangesBeforeRename(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

	triggered := false
	setBeforeManagedReplaceCurrentHoldHook(func(entry control.ManifestEntry, targetPath string, _ string) error {
		if entry.Path != "file.txt" || triggered {
			return nil
		}
		triggered = true
		mustReplaceFile(t, targetPath, "external replacement", 0o644)
		return nil
	})
	t.Cleanup(func() { setBeforeManagedReplaceCurrentHoldHook(nil) })

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setBeforeManagedReplaceCurrentHoldHook(nil)
	if err == nil || !errors.Is(err, errManagedReplaceTargetChanged) {
		t.Fatalf("Run(target replaced before rename) error = %v, want %v", err, errManagedReplaceTargetChanged)
	}
	if !triggered {
		t.Fatalf("beforeManagedReplaceCurrentHold was not triggered")
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "external replacement" {
		t.Fatalf("target after current hold mismatch = (%q, %v), want external replacement restored", string(got), err)
	}
	previousHold, err := replacementHoldPath(target, "session-two", "previous", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath(previous) error = %v, want nil", err)
	}
	if got, err := os.ReadFile(previousHold); err != nil || string(got) != "old" {
		t.Fatalf("previous replacement hold = (%q, %v), want old", string(got), err)
	}
	currentHold, err := replacementHoldPath(target, "session-two", "current", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath(current) error = %v, want nil", err)
	}
	if _, err := os.Stat(currentHold); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(current replacement hold after restore) error = %v, want os.ErrNotExist", err)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "session-two", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(receipt after current hold mismatch) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunRefusesExternalTargetThatAlreadyMatchesNewManifest(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	oldTime := time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), oldTime, oldTime); err != nil {
		t.Fatalf("os.Chtimes(source old file) error = %v, want nil", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 30, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o600)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), newTime, newTime); err != nil {
		t.Fatalf("os.Chtimes(source new file) error = %v, want nil", err)
	}

	triggered := false
	setBeforePublishStagedHook(func(entry control.ManifestEntry, targetPath string) error {
		if entry.Path != "file.txt" || triggered {
			return nil
		}
		triggered = true
		mustReplaceFile(t, targetPath, "new source", 0o600)
		if err := os.Chtimes(targetPath, newTime, newTime); err != nil {
			t.Fatalf("os.Chtimes(external target) error = %v, want nil", err)
		}
		return nil
	})
	t.Cleanup(func() { setBeforePublishStagedHook(nil) })

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 30, 0, 0, time.UTC)})
	setBeforePublishStagedHook(nil)
	if err == nil || !strings.Contains(err.Error(), "refusing to accept external replacement") {
		t.Fatalf("Run(external target matches new manifest) error = %v, want external replacement refusal", err)
	}
	if !triggered {
		t.Fatalf("beforePublishStaged was not triggered")
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "new source" {
		t.Fatalf("target after external replacement refusal = (%q, %v), want external new source left intact", string(got), err)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "session-two", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(receipt after external replacement refusal) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunRefusesToOverwriteConcurrentCurrentReplacementHold(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

	triggered := false
	setBeforeManagedReplaceCurrentHoldHook(func(entry control.ManifestEntry, _ string, holdPath string) error {
		if entry.Path != "file.txt" || triggered {
			return nil
		}
		triggered = true
		mustWriteFile(t, holdPath, "external current hold", 0o644)
		return nil
	})
	t.Cleanup(func() { setBeforeManagedReplaceCurrentHoldHook(nil) })

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setBeforeManagedReplaceCurrentHoldHook(nil)
	if err == nil || !strings.Contains(err.Error(), "without replace") {
		t.Fatalf("Run(concurrent current hold) error = %v, want no-replace refusal", err)
	}
	if !triggered {
		t.Fatalf("beforeManagedReplaceCurrentHold was not triggered")
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "old" {
		t.Fatalf("target after current hold collision = (%q, %v), want old", string(got), err)
	}
	currentHold, err := replacementHoldPath(target, "session-two", "current", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath(current) error = %v, want nil", err)
	}
	if got, err := os.ReadFile(currentHold); err != nil || string(got) != "external current hold" {
		t.Fatalf("current replacement hold after collision = (%q, %v), want external current hold", string(got), err)
	}
}

func TestRecoverPublishesManagedReplacementHeldAfterInterruption(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

	triggered := false
	setAfterManagedReplaceHoldHook(func(entry control.ManifestEntry, _ string, holdPath string) error {
		if entry.Path != "file.txt" || triggered {
			return nil
		}
		triggered = true
		if got, err := os.ReadFile(holdPath); err != nil || string(got) != "old" {
			t.Fatalf("replacement hold during interruption = (%q, %v), want old", string(got), err)
		}
		return errors.New("simulated interruption after replacement hold")
	})
	t.Cleanup(func() { setAfterManagedReplaceHoldHook(nil) })

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setAfterManagedReplaceHoldHook(nil)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption") {
		t.Fatalf("Run(interrupted after replacement hold) error = %v, want simulated interruption", err)
	}
	if !triggered {
		t.Fatalf("afterManagedReplaceHold was not triggered")
	}
	if _, err := os.Stat(filepath.Join(target, "file.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(target after held interruption) error = %v, want os.ErrNotExist", err)
	}
	holdPath, err := replacementHoldPath(target, "session-two", "previous", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath() error = %v, want nil", err)
	}
	if got, err := os.ReadFile(holdPath); err != nil || string(got) != "old" {
		t.Fatalf("replacement hold after interruption = (%q, %v), want old", string(got), err)
	}
	currentHoldPath, err := replacementHoldPath(target, "session-two", "current", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath(current) error = %v, want nil", err)
	}
	if got, err := os.ReadFile(currentHoldPath); err != nil || string(got) != "old" {
		t.Fatalf("current replacement hold after interruption = (%q, %v), want old", string(got), err)
	}

	recovered, err := Recover(RecoverOptions{
		Profile:   p,
		TargetDir: target,
		SessionID: "session-two",
		Now:       time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Recover(held managed replacement) error = %v, want nil", err)
	}
	if recovered.Recovered != 1 || recovered.RepairNeeded != 0 {
		t.Fatalf("Recover(held managed replacement) result = %+v, want one recovered session", recovered)
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "new source" {
		t.Fatalf("target after held replacement recover = (%q, %v), want new source", string(got), err)
	}
	if _, err := os.Stat(holdPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(replacement hold after recover) error = %v, want os.ErrNotExist", err)
	}
	if _, err := os.Stat(currentHoldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(current replacement hold after recover) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunRestoresHeldTargetWhenManagedReplacementPromoteFails(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

	triggered := false
	setAfterManagedReplaceHoldHook(func(entry control.ManifestEntry, _ string, _ string) error {
		if entry.Path != "file.txt" || triggered {
			return nil
		}
		triggered = true
		stagePath, err := pathguard.SafeJoin(transaction.NewLayout(control.ControlDir(target)).StagingDir("session-two"), entry.Path)
		if err != nil {
			t.Fatalf("pathguard.SafeJoin(stage, %q) error = %v, want nil", entry.Path, err)
		}
		if err := os.Remove(stagePath); err != nil {
			t.Fatalf("os.Remove(stage %q) error = %v, want nil", stagePath, err)
		}
		return nil
	})
	t.Cleanup(func() { setAfterManagedReplaceHoldHook(nil) })

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setAfterManagedReplaceHoldHook(nil)
	if err == nil {
		t.Fatalf("Run(stage removed after replacement hold) error = nil, want promote failure")
	}
	if !triggered {
		t.Fatalf("afterManagedReplaceHold was not triggered")
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "old" {
		t.Fatalf("target after failed promote = (%q, %v), want restored old", string(got), err)
	}
	holdPath, err := replacementHoldPath(target, "session-two", "previous", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath() error = %v, want nil", err)
	}
	if _, err := os.Stat(holdPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(replacement hold after failed promote restore) error = %v, want os.ErrNotExist", err)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "session-two", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(receipt after failed promote restore) error = %v, want os.ErrNotExist", err)
	}
}

func TestRunRetainsReplacementHoldWhenHeldTargetMutatesBeforeCleanup(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

	triggered := false
	setAfterManagedReplaceHoldHook(func(entry control.ManifestEntry, _ string, holdPath string) error {
		if entry.Path != "file.txt" || triggered {
			return nil
		}
		triggered = true
		mustReplaceFile(t, holdPath, "external old-file write", 0o644)
		stagePath, err := pathguard.SafeJoin(transaction.NewLayout(control.ControlDir(target)).StagingDir("session-two"), entry.Path)
		if err != nil {
			t.Fatalf("pathguard.SafeJoin(stage, %q) error = %v, want nil", entry.Path, err)
		}
		if err := os.Remove(stagePath); err != nil {
			t.Fatalf("os.Remove(stage %q) error = %v, want nil", stagePath, err)
		}
		return nil
	})
	t.Cleanup(func() { setAfterManagedReplaceHoldHook(nil) })

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setAfterManagedReplaceHoldHook(nil)
	if err == nil || !errors.Is(err, errManagedReplaceTargetChanged) {
		t.Fatalf("Run(mutated hold with failed promote) error = %v, want %v", err, errManagedReplaceTargetChanged)
	}
	if !triggered {
		t.Fatalf("afterManagedReplaceHold was not triggered")
	}
	if _, err := os.Stat(filepath.Join(target, "file.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(target after mutated hold failed promote) error = %v, want os.ErrNotExist", err)
	}
	holdPath, err := replacementHoldPath(target, "session-two", "previous", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath() error = %v, want nil", err)
	}
	if got, err := os.ReadFile(holdPath); err != nil || string(got) != "external old-file write" {
		t.Fatalf("replacement hold after held mutation = (%q, %v), want external old-file write", string(got), err)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "session-two", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(receipt after held mutation) error = %v, want os.ErrNotExist", err)
	}
}

func TestRecoverRefusesManagedReplacementWhenExistingTargetMetadataDiffers(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)
	newTime := time.Date(2026, 5, 16, 2, 30, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), newTime, newTime); err != nil {
		t.Fatalf("os.Chtimes(source new file) error = %v, want nil", err)
	}

	setBeforePublishStagedHook(func(entry control.ManifestEntry, _ string) error {
		if entry.Path == "file.txt" {
			return errors.New("simulated interruption before publish")
		}
		return nil
	})
	t.Cleanup(func() { setBeforePublishStagedHook(nil) })
	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setBeforePublishStagedHook(nil)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption") {
		t.Fatalf("second Run(simulated interruption) error = %v, want simulated interruption", err)
	}
	mustReplaceFile(t, filepath.Join(target, "file.txt"), "new source", 0o600)
	manualTime := time.Date(2026, 5, 16, 9, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(target, "file.txt"), manualTime, manualTime); err != nil {
		t.Fatalf("os.Chtimes(manual target) error = %v, want nil", err)
	}

	recovered, err := Recover(RecoverOptions{
		Profile:   p,
		TargetDir: target,
		SessionID: "session-two",
		Now:       time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Recover(existing target with mismatched metadata) error = %v, want nil result with needs_repair", err)
	}
	if recovered.RepairNeeded != 1 || recovered.Recovered != 0 {
		t.Fatalf("Recover(existing target with mismatched metadata) result = %+v, want needs_repair", recovered)
	}
	info, err := os.Stat(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("os.Stat(target after refused metadata recover) error = %v, want nil", err)
	}
	if info.Mode().Perm() != 0o600 || !info.ModTime().Equal(manualTime) {
		t.Fatalf("target metadata after refused recover = mode %v mtime %v, want manual metadata", info.Mode().Perm(), info.ModTime())
	}
}

func TestRecoverCleansReplacementHoldAfterPromoteSucceededBeforeCleanup(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	oldTime := time.Date(2026, 5, 16, 1, 30, 0, 0, time.UTC)
	newTime := time.Date(2026, 5, 16, 2, 30, 0, 0, time.UTC)
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), oldTime, oldTime); err != nil {
		t.Fatalf("os.Chtimes(source old file) error = %v, want nil", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o600)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), newTime, newTime); err != nil {
		t.Fatalf("os.Chtimes(source new file) error = %v, want nil", err)
	}

	setBeforePublishStagedHook(func(entry control.ManifestEntry, _ string) error {
		if entry.Path == "file.txt" {
			return errors.New("simulated interruption before publish")
		}
		return nil
	})
	t.Cleanup(func() { setBeforePublishStagedHook(nil) })
	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setBeforePublishStagedHook(nil)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption") {
		t.Fatalf("second Run(simulated interruption) error = %v, want simulated interruption", err)
	}

	layout := transaction.NewLayout(control.ControlDir(target))
	holdPath, err := replacementHoldPath(target, "session-two", "previous", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath() error = %v, want nil", err)
	}
	if err := pathguard.EnsurePlainDirectory(control.ControlDir(target), filepath.Dir(holdPath), 0o700); err != nil {
		t.Fatalf("EnsurePlainDirectory(hold parent) error = %v, want nil", err)
	}
	if err := os.Link(filepath.Join(target, "file.txt"), holdPath); err != nil {
		t.Fatalf("os.Link(target old, hold) error = %v, want nil", err)
	}
	currentHoldPath, err := replacementHoldPath(target, "session-two", "current", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath(current) error = %v, want nil", err)
	}
	if err := pathguard.EnsurePlainDirectory(control.ControlDir(target), filepath.Dir(currentHoldPath), 0o700); err != nil {
		t.Fatalf("EnsurePlainDirectory(current hold parent) error = %v, want nil", err)
	}
	if err := os.Link(filepath.Join(target, "file.txt"), currentHoldPath); err != nil {
		t.Fatalf("os.Link(target old, current hold) error = %v, want nil", err)
	}
	if err := os.Remove(filepath.Join(target, "file.txt")); err != nil {
		t.Fatalf("os.Remove(target old) error = %v, want nil", err)
	}
	mustWriteFile(t, filepath.Join(target, "file.txt"), "new source", 0o600)
	if err := os.Chtimes(filepath.Join(target, "file.txt"), newTime, newTime); err != nil {
		t.Fatalf("os.Chtimes(target new file) error = %v, want nil", err)
	}
	stagePath, err := pathguard.SafeJoin(layout.StagingDir("session-two"), "file.txt")
	if err != nil {
		t.Fatalf("pathguard.SafeJoin(stage, file.txt) error = %v, want nil", err)
	}
	if _, err := os.Stat(stagePath); err != nil {
		t.Fatalf("os.Stat(stage before recover) error = %v, want nil", err)
	}

	recovered, err := Recover(RecoverOptions{
		Profile:   p,
		TargetDir: target,
		SessionID: "session-two",
		Now:       time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Recover(promoted before hold cleanup) error = %v, want nil", err)
	}
	if recovered.Recovered != 1 || recovered.RepairNeeded != 0 {
		t.Fatalf("Recover(promoted before hold cleanup) result = %+v, want recovered", recovered)
	}
	if _, err := os.Stat(holdPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(replacement hold after cleanup recover) error = %v, want os.ErrNotExist", err)
	}
	if _, err := os.Stat(currentHoldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(current replacement hold after cleanup recover) error = %v, want os.ErrNotExist", err)
	}
	if _, err := os.Stat(stagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(stage after cleanup recover) error = %v, want os.ErrNotExist", err)
	}
}

func TestRecoverRetainsPreviousHoldWhenCurrentHoldMutatesBeforeCleanup(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	oldTime := time.Date(2026, 5, 16, 1, 30, 0, 0, time.UTC)
	newTime := time.Date(2026, 5, 16, 2, 30, 0, 0, time.UTC)
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), oldTime, oldTime); err != nil {
		t.Fatalf("os.Chtimes(source old file) error = %v, want nil", err)
	}
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o600)
	if err := os.Chtimes(filepath.Join(source, "file.txt"), newTime, newTime); err != nil {
		t.Fatalf("os.Chtimes(source new file) error = %v, want nil", err)
	}

	setBeforePublishStagedHook(func(entry control.ManifestEntry, _ string) error {
		if entry.Path == "file.txt" {
			return errors.New("simulated interruption before publish")
		}
		return nil
	})
	t.Cleanup(func() { setBeforePublishStagedHook(nil) })
	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setBeforePublishStagedHook(nil)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption") {
		t.Fatalf("second Run(simulated interruption) error = %v, want simulated interruption", err)
	}

	layout := transaction.NewLayout(control.ControlDir(target))
	previousHold, err := replacementHoldPath(target, "session-two", "previous", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath(previous) error = %v, want nil", err)
	}
	if err := pathguard.EnsurePlainDirectory(control.ControlDir(target), filepath.Dir(previousHold), 0o700); err != nil {
		t.Fatalf("EnsurePlainDirectory(previous hold parent) error = %v, want nil", err)
	}
	if err := os.Link(filepath.Join(target, "file.txt"), previousHold); err != nil {
		t.Fatalf("os.Link(target old, previous hold) error = %v, want nil", err)
	}
	currentHold, err := replacementHoldPath(target, "session-two", "current", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath(current) error = %v, want nil", err)
	}
	if err := pathguard.EnsurePlainDirectory(control.ControlDir(target), filepath.Dir(currentHold), 0o700); err != nil {
		t.Fatalf("EnsurePlainDirectory(current hold parent) error = %v, want nil", err)
	}
	mustWriteFile(t, currentHold, "external current hold mutation", 0o644)
	if err := os.Remove(filepath.Join(target, "file.txt")); err != nil {
		t.Fatalf("os.Remove(target old) error = %v, want nil", err)
	}
	mustWriteFile(t, filepath.Join(target, "file.txt"), "new source", 0o600)
	if err := os.Chtimes(filepath.Join(target, "file.txt"), newTime, newTime); err != nil {
		t.Fatalf("os.Chtimes(target new file) error = %v, want nil", err)
	}
	stagePath, err := pathguard.SafeJoin(layout.StagingDir("session-two"), "file.txt")
	if err != nil {
		t.Fatalf("pathguard.SafeJoin(stage, file.txt) error = %v, want nil", err)
	}

	recovered, err := Recover(RecoverOptions{
		Profile:   p,
		TargetDir: target,
		SessionID: "session-two",
		Now:       time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Recover(mutated current hold) error = %v, want nil result with needs_repair", err)
	}
	if recovered.RepairNeeded != 1 || recovered.Recovered != 0 {
		t.Fatalf("Recover(mutated current hold) result = %+v, want needs_repair", recovered)
	}
	if got, err := os.ReadFile(previousHold); err != nil || string(got) != "old" {
		t.Fatalf("previous hold after current mutation recover = (%q, %v), want old", string(got), err)
	}
	if got, err := os.ReadFile(currentHold); err != nil || string(got) != "external current hold mutation" {
		t.Fatalf("current hold after current mutation recover = (%q, %v), want mutated content", string(got), err)
	}
	if _, err := os.Stat(stagePath); err != nil {
		t.Fatalf("os.Stat(stage after refused mutated current hold recover) error = %v, want staged file retained", err)
	}
}

func TestRunRefusesReplacementHoldThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	outside := filepath.Join(dir, "outside")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(outside) error = %v, want nil", err)
	}
	if err := os.Symlink(outside, filepath.Join(target, control.DirName, "replacement-holds")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	if err == nil || !errors.Is(err, pathguard.ErrUnsafePath) {
		t.Fatalf("Run(replacement hold symlink) error = %v, want ErrUnsafePath", err)
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "old" {
		t.Fatalf("target after hold symlink refusal = (%q, %v), want old", string(got), err)
	}
}

func TestRecoverRefusesPreviousOnlyReplacementHoldForMissingTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}

	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)
	setBeforePublishStagedHook(func(entry control.ManifestEntry, _ string) error {
		if entry.Path == "file.txt" {
			return errors.New("simulated interruption before publish")
		}
		return nil
	})
	t.Cleanup(func() { setBeforePublishStagedHook(nil) })
	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setBeforePublishStagedHook(nil)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption") {
		t.Fatalf("second Run(simulated interruption) error = %v, want simulated interruption", err)
	}

	previousHold, err := replacementHoldPath(target, "session-two", "previous", "file.txt")
	if err != nil {
		t.Fatalf("replacementHoldPath(previous) error = %v, want nil", err)
	}
	if err := pathguard.EnsurePlainDirectory(control.ControlDir(target), filepath.Dir(previousHold), 0o700); err != nil {
		t.Fatalf("EnsurePlainDirectory(previous hold parent) error = %v, want nil", err)
	}
	if err := createReplacementHold(filepath.Join(target, "file.txt"), previousHold); err != nil {
		t.Fatalf("createReplacementHold(previous only) error = %v, want nil", err)
	}
	if err := os.Remove(filepath.Join(target, "file.txt")); err != nil {
		t.Fatalf("os.Remove(target previous file) error = %v, want nil", err)
	}

	recovered, err := Recover(RecoverOptions{
		Profile:   p,
		TargetDir: target,
		SessionID: "session-two",
		Now:       time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Recover(previous-only replacement hold) error = %v, want nil result with needs_repair", err)
	}
	if recovered.RepairNeeded != 1 || recovered.Recovered != 0 {
		t.Fatalf("Recover(previous-only replacement hold) result = %+v, want needs_repair", recovered)
	}
	if _, err := os.Stat(filepath.Join(target, "file.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(target after previous-only recover) error = %v, want os.ErrNotExist", err)
	}
	if got, err := os.ReadFile(previousHold); err != nil || string(got) != "old" {
		t.Fatalf("previous-only hold after recover refusal = (%q, %v), want old", string(got), err)
	}
}

func TestPushRefusesChangedFileWhenPreviousManifestLacksCompleteMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*control.ManifestEntry)
	}{
		{
			name: "missing modtime",
			mutate: func(entry *control.ManifestEntry) {
				entry.ModTime = ""
			},
		},
	}
	pushes := []struct {
		name string
		push func(Options) (Result, error)
	}{
		{name: "preflight", push: Preflight},
		{name: "run", push: Run},
	}

	for _, tt := range tests {
		for _, push := range pushes {
			t.Run(tt.name+"/"+push.name, func(t *testing.T) {
				dir := t.TempDir()
				source := filepath.Join(dir, "source")
				target := filepath.Join(dir, "target")
				mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
				p := profile.NewDefault("profile-local", "Local profile", source, target)
				if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
					t.Fatalf("first Run(%s/%s) error = %v, want nil", tt.name, push.name, err)
				}
				previous := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-one")
				for i := range previous.Entries {
					if previous.Entries[i].Path == "file.txt" {
						tt.mutate(&previous.Entries[i])
					}
				}
				writeManifest(t, target, previous)
				mustReplaceFile(t, filepath.Join(source, "file.txt"), "new source", 0o644)

				_, err := push.push(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
				if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
					t.Fatalf("%s(changed file with %s previous manifest) error = %v, want overwrite refusal", push.name, tt.name, err)
				}
				if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "old" {
					t.Fatalf("target after %s/%s refusal = (%q, %v), want old", tt.name, push.name, string(got), err)
				}
				if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "session-two", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("os.Stat(receipt after %s/%s refusal) error = %v, want os.ErrNotExist", tt.name, push.name, err)
				}
			})
		}
	}
}

func TestPushRefusesChangedFileWhenPreviousManifestBelongsToDifferentTarget(t *testing.T) {
	tests := []struct {
		name string
		push func(Options) (Result, error)
	}{
		{name: "preflight", push: Preflight},
		{name: "run", push: Run},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			sourceA := filepath.Join(dir, "source-a")
			sourceB := filepath.Join(dir, "source-b")
			target := filepath.Join(dir, "target")
			mustWriteFile(t, filepath.Join(sourceA, "file.txt"), "old", 0o644)
			profileA := profile.NewDefault("profile-a", "Profile A", sourceA, target)
			if _, err := Run(Options{Profile: profileA, TargetDir: target, SessionID: "session-a", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
				t.Fatalf("Run(profile-a) error = %v, want nil", err)
			}
			mustWriteFile(t, filepath.Join(sourceB, "file.txt"), "new", 0o644)
			profileB := profile.NewDefault("profile-b", "Profile B", sourceB, target)

			_, err := tt.push(Options{Profile: profileB, TargetDir: target, SessionID: "session-b", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
			if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
				t.Fatalf("%s(profile-b changed file) error = %v, want overwrite refusal without profile-b ownership proof", tt.name, err)
			}
			got, err := os.ReadFile(filepath.Join(target, "file.txt"))
			if err != nil {
				t.Fatalf("os.ReadFile(target after profile-b failure) error = %v, want nil", err)
			}
			if string(got) != "old" {
				t.Fatalf("target after profile-b %s failure = %q, want old", tt.name, string(got))
			}
			if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "session-b", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("os.Stat(receipt after profile-b %s failure) error = %v, want os.ErrNotExist", tt.name, err)
			}
		})
	}
}

func TestRecoverPublishesStagedManagedReplacement(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new", 0o644)

	triggered := false
	setBeforePublishStagedHook(func(entry control.ManifestEntry, _ string) error {
		if entry.Path != "file.txt" || triggered {
			return nil
		}
		triggered = true
		return errors.New("simulated interruption before publish")
	})
	t.Cleanup(func() { setBeforePublishStagedHook(nil) })
	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setBeforePublishStagedHook(nil)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption") {
		t.Fatalf("second Run(simulated interruption) error = %v, want simulated interruption", err)
	}
	if !triggered {
		t.Fatalf("beforePublishStaged was not triggered")
	}
	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(target)).RecordPath("session-two"))
	if err != nil {
		t.Fatalf("ReadSessionRecord(session-two) error = %v, want nil", err)
	}
	if record.State != transaction.StateStaged {
		t.Fatalf("session-two state after interrupted publish = %q, want %q", record.State, transaction.StateStaged)
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "old" {
		t.Fatalf("target after interrupted publish = (%q, %v), want old", string(got), err)
	}
	manifest := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-two")
	entry := manifestEntryByPath(t, manifest, "file.txt")
	if entry.PreviousSessionID != "session-one" || entry.PreviousManifestID != "manifest-session-one" || !strings.HasPrefix(entry.PreviousDigest, "sha256:") {
		t.Fatalf("interrupted manifest entry = %#v, want previous evidence for recovery", entry)
	}

	recovered, err := Recover(RecoverOptions{
		Profile:   p,
		TargetDir: target,
		SessionID: "session-two",
		Now:       time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Recover(staged managed replacement) error = %v, want nil", err)
	}
	if recovered.Recovered != 1 || recovered.RepairNeeded != 0 {
		t.Fatalf("Recover(staged managed replacement) result = %+v, want one recovered session", recovered)
	}
	got, err := os.ReadFile(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile(target after recover) error = %v, want nil", err)
	}
	if string(got) != "new" {
		t.Fatalf("target after recover = %q, want new", string(got))
	}
	finalRecord, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(target)).RecordPath("session-two"))
	if err != nil {
		t.Fatalf("ReadSessionRecord(session-two final) error = %v, want nil", err)
	}
	if finalRecord.State != transaction.StatePublished {
		t.Fatalf("session-two state after recover = %q, want %q", finalRecord.State, transaction.StatePublished)
	}
}

func TestRecoverRefusesStagedManagedReplacementAfterTargetDrift(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new", 0o644)

	setBeforePublishStagedHook(func(entry control.ManifestEntry, _ string) error {
		if entry.Path == "file.txt" {
			return errors.New("simulated interruption before publish")
		}
		return nil
	})
	t.Cleanup(func() { setBeforePublishStagedHook(nil) })
	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setBeforePublishStagedHook(nil)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption") {
		t.Fatalf("second Run(simulated interruption) error = %v, want simulated interruption", err)
	}
	mustReplaceFile(t, filepath.Join(target, "file.txt"), "manual target edit", 0o644)

	recovered, err := Recover(RecoverOptions{
		Profile:   p,
		TargetDir: target,
		SessionID: "session-two",
		Now:       time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Recover(staged managed replacement after target drift) error = %v, want nil result with needs_repair", err)
	}
	if recovered.RepairNeeded != 1 || recovered.Recovered != 0 {
		t.Fatalf("Recover(staged managed replacement after target drift) result = %+v, want needs_repair", recovered)
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "manual target edit" {
		t.Fatalf("target after refused recover = (%q, %v), want manual target edit", string(got), err)
	}
	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(target)).RecordPath("session-two"))
	if err != nil {
		t.Fatalf("ReadSessionRecord(session-two) error = %v, want nil", err)
	}
	if record.State != transaction.StateNeedsRepair {
		t.Fatalf("session-two state after refused recover = %q, want %q", record.State, transaction.StateNeedsRepair)
	}
}

func TestRecoverRejectsStagedManagedReplacementWithForgedPreviousManifestID(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatalf("first Run() error = %v, want nil", err)
	}
	mustReplaceFile(t, filepath.Join(source, "file.txt"), "new", 0o644)

	setBeforePublishStagedHook(func(entry control.ManifestEntry, _ string) error {
		if entry.Path == "file.txt" {
			return errors.New("simulated interruption before publish")
		}
		return nil
	})
	t.Cleanup(func() { setBeforePublishStagedHook(nil) })
	_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
	setBeforePublishStagedHook(nil)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption") {
		t.Fatalf("second Run(simulated interruption) error = %v, want simulated interruption", err)
	}
	manifestPath, err := control.Path(target, control.ArtifactManifest, "session-two")
	if err != nil {
		t.Fatalf("control.Path(session-two manifest) error = %v, want nil", err)
	}
	manifest := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-two")
	for i := range manifest.Entries {
		if manifest.Entries[i].Path == "file.txt" {
			manifest.Entries[i].PreviousManifestID = "manifest-forged"
		}
	}
	if err := control.WriteFile(manifestPath, manifest); err != nil {
		t.Fatalf("control.WriteFile(forged manifest) error = %v, want nil", err)
	}

	recovered, err := Recover(RecoverOptions{
		Profile:   p,
		TargetDir: target,
		SessionID: "session-two",
		Now:       time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Recover(forged previous evidence) error = %v, want nil result with needs_repair", err)
	}
	if recovered.RepairNeeded != 1 || recovered.Recovered != 0 {
		t.Fatalf("Recover(forged previous evidence) result = %+v, want needs_repair", recovered)
	}
	if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "old" {
		t.Fatalf("target after forged evidence recover = (%q, %v), want old", string(got), err)
	}
	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(target)).RecordPath("session-two"))
	if err != nil {
		t.Fatalf("ReadSessionRecord(session-two) error = %v, want nil", err)
	}
	if record.State != transaction.StateNeedsRepair || !strings.Contains(record.Note, "previous manifest") {
		t.Fatalf("session-two state after forged evidence recover = %#v, want needs_repair with previous manifest note", record)
	}
}

func TestRecoverRejectsStagedManagedReplacementWithForgedPreviousMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*control.ManifestEntry)
	}{
		{
			name: "mode",
			mutate: func(entry *control.ManifestEntry) {
				entry.Mode = 0
			},
		},
		{
			name: "modtime",
			mutate: func(entry *control.ManifestEntry) {
				entry.ModTime = "2026-05-16T09:00:00Z"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			target := filepath.Join(dir, "target")
			mustWriteFile(t, filepath.Join(source, "file.txt"), "old", 0o644)
			p := profile.NewDefault("profile-local", "Local profile", source, target)
			if _, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-one", Now: time.Date(2026, 5, 16, 1, 0, 0, 0, time.UTC)}); err != nil {
				t.Fatalf("first Run(%s) error = %v, want nil", tt.name, err)
			}
			mustReplaceFile(t, filepath.Join(source, "file.txt"), "new", 0o644)

			setBeforePublishStagedHook(func(entry control.ManifestEntry, _ string) error {
				if entry.Path == "file.txt" {
					return errors.New("simulated interruption before publish")
				}
				return nil
			})
			t.Cleanup(func() { setBeforePublishStagedHook(nil) })
			_, err := Run(Options{Profile: p, TargetDir: target, SessionID: "session-two", Now: time.Date(2026, 5, 16, 2, 0, 0, 0, time.UTC)})
			setBeforePublishStagedHook(nil)
			if err == nil || !strings.Contains(err.Error(), "simulated interruption") {
				t.Fatalf("second Run(%s simulated interruption) error = %v, want simulated interruption", tt.name, err)
			}

			previous := readControlDoc[control.Manifest](t, target, control.ArtifactManifest, "session-one")
			for i := range previous.Entries {
				if previous.Entries[i].Path == "file.txt" {
					tt.mutate(&previous.Entries[i])
				}
			}
			writeManifest(t, target, previous)

			recovered, err := Recover(RecoverOptions{
				Profile:   p,
				TargetDir: target,
				SessionID: "session-two",
				Now:       time.Date(2026, 5, 16, 3, 0, 0, 0, time.UTC),
			})
			if err != nil {
				t.Fatalf("Recover(forged previous %s) error = %v, want nil result with needs_repair", tt.name, err)
			}
			if recovered.RepairNeeded != 1 || recovered.Recovered != 0 {
				t.Fatalf("Recover(forged previous %s) result = %+v, want needs_repair", tt.name, recovered)
			}
			if got, err := os.ReadFile(filepath.Join(target, "file.txt")); err != nil || string(got) != "old" {
				t.Fatalf("target after forged previous %s recover = (%q, %v), want old", tt.name, string(got), err)
			}
			record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(target)).RecordPath("session-two"))
			if err != nil {
				t.Fatalf("ReadSessionRecord(session-two) error = %v, want nil", err)
			}
			if record.State != transaction.StateNeedsRepair || !strings.Contains(record.Note, "metadata") {
				t.Fatalf("session-two state after forged previous %s recover = %#v, want needs_repair with metadata note", tt.name, record)
			}
		})
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

func manifestEntryByPath(t *testing.T, manifest control.Manifest, path string) control.ManifestEntry {
	t.Helper()
	for _, entry := range manifest.Entries {
		if entry.Path == path {
			return entry
		}
	}
	t.Fatalf("manifest entries = %#v, want path %q", manifest.Entries, path)
	return control.ManifestEntry{}
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

func prepareStagedRecoverySession(t *testing.T) (source string, target string, p profile.Profile, sessionID string) {
	t.Helper()
	dir := t.TempDir()
	source = filepath.Join(dir, "source")
	target = filepath.Join(dir, "target")
	sessionID = "session-recover"
	mustWriteFile(t, filepath.Join(source, "file.txt"), "payload", 0o644)
	p = profile.NewDefault("profile-local", "Local profile", source, target)
	if _, err := Run(Options{
		Profile:   p,
		TargetDir: target,
		SessionID: sessionID,
		Now:       time.Date(2026, 5, 16, 4, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Run(%q) error = %v, want nil", sessionID, err)
	}
	layout := transaction.NewLayout(control.ControlDir(target))
	record, err := transaction.ReadSessionRecord(layout.RecordPath(sessionID))
	if err != nil {
		t.Fatalf("ReadSessionRecord(%q) error = %v, want nil", sessionID, err)
	}
	staged, err := record.WithState(transaction.StateStaged, time.Date(2026, 5, 16, 4, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SessionRecord.WithState(staged) error = %v, want nil", err)
	}
	if err := layout.WriteSessionRecord(staged); err != nil {
		t.Fatalf("WriteSessionRecord(staged) error = %v, want nil", err)
	}
	stagePath, err := pathguard.SafeJoinParent(layout.StagingDir(sessionID), "file.txt")
	if err != nil {
		t.Fatalf("pathguard.SafeJoinParent(stage, file.txt) error = %v, want nil", err)
	}
	mustWriteFile(t, stagePath, "payload", 0o644)
	if err := os.Remove(filepath.Join(target, "file.txt")); err != nil {
		t.Fatalf("os.Remove(published file) error = %v, want nil", err)
	}
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	if err := os.Remove(receiptPath); err != nil {
		t.Fatalf("os.Remove(receipt) error = %v, want nil", err)
	}
	return source, target, p, sessionID
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

func mustReplaceFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Remove(%q) error = %v, want nil", path, err)
	}
	mustWriteFile(t, path, content, mode)
}

func testDigest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + fmt.Sprintf("%x", sum[:])
}
