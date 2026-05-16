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

func TestRecoverPublishesStagedSession(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	writeStagedLocalSession(t, layout, target, "session-recover", []control.ManifestEntry{
		{Path: "notes/a.txt", TargetPath: "notes/a.txt", Kind: "file", Mode: 0o640, Size: 7, ModTime: now.Format(time.RFC3339Nano), Digest: "sha256:239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"},
	})
	writeStageFile(t, layout, "session-recover", "notes/a.txt", "payload")

	got, err := Recover(RecoverOptions{Profile: p, TargetDir: target, SessionID: "session-recover", Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("Recover(staged session) error = %v, want nil", err)
	}
	if got.Recovered != 1 || got.RepairNeeded != 0 {
		t.Fatalf("Recover(staged session) = %#v, want one recovered item", got)
	}
	if bytes, err := os.ReadFile(filepath.Join(target, "notes", "a.txt")); err != nil || string(bytes) != "payload" {
		t.Fatalf("published file = (%q, %v), want payload", string(bytes), err)
	}
	record, err := transaction.ReadSessionRecord(layout.RecordPath("session-recover"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(recovered) error = %v, want nil", err)
	}
	if record.State != transaction.StatePublished {
		t.Fatalf("recovered state = %q, want %q", record.State, transaction.StatePublished)
	}
	receipt := readControlDoc[control.SessionReceipt](t, target, control.ArtifactSessionReceipt, "session-recover")
	if receipt.Status != "published" || receipt.StartedAt == receipt.EndedAt {
		t.Fatalf("recovered receipt = %#v, want published with distinct start/end", receipt)
	}
}

func TestRecoverCompletesAlreadyPublishedFileWithoutStage(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	entry := control.ManifestEntry{Path: "file.txt", TargetPath: "file.txt", Kind: "file", Mode: 0o644, Size: 7, ModTime: now.Format(time.RFC3339Nano), Digest: "sha256:239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"}
	writeStagedLocalSession(t, layout, target, "session-recover", []control.ManifestEntry{entry})
	writeTargetFile(t, filepath.Join(target, "file.txt"), "payload", 0o600)

	got, err := Recover(RecoverOptions{Profile: p, TargetDir: target, SessionID: "session-recover", Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("Recover(already published file) error = %v, want nil", err)
	}
	if got.Recovered != 1 {
		t.Fatalf("Recover(already published file).Recovered = %d, want 1", got.Recovered)
	}
	info, err := os.Stat(filepath.Join(target, "file.txt"))
	if err != nil {
		t.Fatalf("os.Stat(recovered existing file) error = %v, want nil", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("existing file mode after Recover = %v, want unchanged 0600", info.Mode().Perm())
	}
}

func TestRecoverMarksDivergentTargetNeedsRepair(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	writeStagedLocalSession(t, layout, target, "session-recover", []control.ManifestEntry{
		{Path: "file.txt", TargetPath: "file.txt", Kind: "file", Mode: 0o644, Size: 7, ModTime: now.Format(time.RFC3339Nano), Digest: "sha256:239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"},
	})
	writeStageFile(t, layout, "session-recover", "file.txt", "payload")
	writeTargetFile(t, filepath.Join(target, "file.txt"), "different", 0o644)

	got, err := Recover(RecoverOptions{Profile: p, TargetDir: target, SessionID: "session-recover", Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("Recover(divergent target) error = %v, want nil result with needs_repair", err)
	}
	if got.RepairNeeded != 1 {
		t.Fatalf("Recover(divergent target).RepairNeeded = %d, want 1", got.RepairNeeded)
	}
	record, err := transaction.ReadSessionRecord(layout.RecordPath("session-recover"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(repair) error = %v, want nil", err)
	}
	if record.State != transaction.StateNeedsRepair || !strings.Contains(record.Note, "refusing to overwrite") {
		t.Fatalf("repair record = %#v, want needs_repair with overwrite note", record)
	}
}

func TestRecoverRejectsCorruptStagedPayload(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	writeStagedLocalSession(t, layout, target, "session-recover", []control.ManifestEntry{
		{Path: "file.txt", TargetPath: "file.txt", Kind: "file", Mode: 0o644, Size: 7, ModTime: now.Format(time.RFC3339Nano), Digest: "sha256:239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"},
	})
	writeStageFile(t, layout, "session-recover", "file.txt", "damaged")

	got, err := Recover(RecoverOptions{Profile: p, TargetDir: target, SessionID: "session-recover", Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("Recover(corrupt staged payload) error = %v, want nil result with needs_repair", err)
	}
	if got.RepairNeeded != 1 {
		t.Fatalf("Recover(corrupt staged payload).RepairNeeded = %d, want 1", got.RepairNeeded)
	}
	if _, err := os.Stat(filepath.Join(target, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(final file after corrupt recover) error = %v, want os.ErrNotExist", err)
	}
	record, err := transaction.ReadSessionRecord(layout.RecordPath("session-recover"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(corrupt staged repair) error = %v, want nil", err)
	}
	if record.State != transaction.StateNeedsRepair || !strings.Contains(record.Note, "digest") {
		t.Fatalf("corrupt staged repair record = %#v, want needs_repair with digest note", record)
	}
}

func TestRecoverRollbackIncompleteRequiresExplicitOptIn(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecoverRecord(t, layout, "session-incomplete", transaction.StateValidated, time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC))

	dry, err := Recover(RecoverOptions{Profile: p, TargetDir: target, SessionID: "session-incomplete", DryRun: true})
	if err != nil {
		t.Fatalf("Recover(incomplete dry-run) error = %v, want nil", err)
	}
	if dry.Skipped != 1 || dry.Items[0].Status != "would_rollback" {
		t.Fatalf("Recover(incomplete dry-run) = %#v, want would_rollback", dry)
	}
	got, err := Recover(RecoverOptions{Profile: p, TargetDir: target, SessionID: "session-incomplete"})
	if err != nil {
		t.Fatalf("Recover(incomplete default) error = %v, want nil", err)
	}
	if got.Skipped != 1 || got.Items[0].Status != "skipped" {
		t.Fatalf("Recover(incomplete default) = %#v, want skipped", got)
	}
	applied, err := Recover(RecoverOptions{Profile: p, TargetDir: target, SessionID: "session-incomplete", RollbackIncomplete: true})
	if err != nil {
		t.Fatalf("Recover(incomplete rollback) error = %v, want nil", err)
	}
	if applied.Recovered != 1 || applied.Items[0].Status != "rolled_back" {
		t.Fatalf("Recover(incomplete rollback) = %#v, want rolled_back", applied)
	}
	record, err := transaction.ReadSessionRecord(layout.RecordPath("session-incomplete"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(rolled back) error = %v, want nil", err)
	}
	if record.State != transaction.StateRolledBack {
		t.Fatalf("record state = %q, want %q", record.State, transaction.StateRolledBack)
	}
}

func writeStagedLocalSession(t *testing.T, layout transaction.Layout, target string, sessionID string, entries []control.ManifestEntry) {
	t.Helper()
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	writeRecoverRecord(t, layout, sessionID, transaction.StateStaged, now)
	manifest := control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		RootID:    "root",
		CreatedAt: now.Format(time.RFC3339Nano),
		Entries:   entries,
	}
	path, err := control.Path(target, control.ArtifactManifest, sessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v, want nil", err)
	}
	if err := control.WriteFile(path, manifest); err != nil {
		t.Fatalf("control.WriteFile(%q) error = %v, want nil", path, err)
	}
}

func writeRecoverRecord(t *testing.T, layout transaction.Layout, sessionID string, state transaction.State, now time.Time) {
	t.Helper()
	record, err := transaction.NewSessionRecord(sessionID, now)
	if err != nil {
		t.Fatalf("transaction.NewSessionRecord(%q) error = %v, want nil", sessionID, err)
	}
	record, err = record.WithState(state, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("SessionRecord.WithState(%q) error = %v, want nil", state, err)
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		t.Fatalf("Layout.WriteSessionRecord(%+v) error = %v, want nil", record, err)
	}
}

func writeStageFile(t *testing.T, layout transaction.Layout, sessionID string, rel string, content string) {
	t.Helper()
	writeTargetFile(t, filepath.Join(layout.StagingDir(sessionID), filepath.FromSlash(rel)), content, 0o644)
}

func writeTargetFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
}
