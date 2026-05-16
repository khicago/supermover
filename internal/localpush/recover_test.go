package localpush

import (
	"encoding/json"
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
	writeStagedLocalSession(t, layout, target, p, "session-recover", []control.ManifestEntry{
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
	writeStagedLocalSession(t, layout, target, p, "session-recover", []control.ManifestEntry{entry})
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
		t.Fatalf("existing file mode after Recover = %v, want existing 0600", info.Mode().Perm())
	}
}

func TestRecoverMarksDivergentTargetNeedsRepair(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	writeStagedLocalSession(t, layout, target, p, "session-recover", []control.ManifestEntry{
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
	writeStagedLocalSession(t, layout, target, p, "session-recover", []control.ManifestEntry{
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

func TestRecoverPublishesStagedSymlinkEntry(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	writeStagedLocalSession(t, layout, target, p, "session-recover", []control.ManifestEntry{
		{Path: "link.txt", TargetPath: "link.txt", Kind: "symlink", SymlinkTarget: "real.txt"},
	})

	got, err := Recover(RecoverOptions{Profile: p, TargetDir: target, SessionID: "session-recover", Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("Recover(symlink-only staged session) error = %v, want nil", err)
	}
	if got.Recovered != 1 || got.RepairNeeded != 0 {
		t.Fatalf("Recover(symlink-only staged session) = %#v, want recovered", got)
	}
	gotTarget, err := os.Readlink(filepath.Join(target, "link.txt"))
	if err != nil {
		t.Fatalf("os.Readlink(recovered symlink target) error = %v, want nil", err)
	}
	if gotTarget != "real.txt" {
		t.Fatalf("os.Readlink(recovered symlink target) = %q, want real.txt", gotTarget)
	}
}

func TestRecoverPreflightRejectsUnsafeSymlinkBeforePartialPublish(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	writeStagedLocalSession(t, layout, target, p, "session-recover", []control.ManifestEntry{
		{Path: "ok.txt", TargetPath: "ok.txt", Kind: "file", Mode: 0o644, Size: 7, ModTime: now.Format(time.RFC3339Nano), Digest: "sha256:239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"},
	})
	appendRawManifestEntry(t, target, "session-recover", `{"path":"link.txt","target_path":"link.txt","kind":"symlink","symlink_target":"../outside"}`)
	writeStageFile(t, layout, "session-recover", "ok.txt", "payload")

	got, err := Recover(RecoverOptions{Profile: p, TargetDir: target, SessionID: "session-recover", Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("Recover(unsafe symlink) error = %v, want nil result with needs_repair", err)
	}
	if got.RepairNeeded != 1 {
		t.Fatalf("Recover(unsafe symlink).RepairNeeded = %d, want 1", got.RepairNeeded)
	}
	if _, err := os.Stat(filepath.Join(target, "ok.txt")); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(partially published file) error = %v, want os.ErrNotExist", err)
	}
	if _, err := os.Lstat(filepath.Join(target, "link.txt")); !os.IsNotExist(err) {
		t.Fatalf("os.Lstat(unsafe symlink) error = %v, want os.ErrNotExist", err)
	}
}

func TestRecoverRejectsReservedControlPlaneTargetPath(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	writeStagedLocalSession(t, layout, target, p, "session-recover", []control.ManifestEntry{
		{Path: "payload.json", TargetPath: ".supermover/sessions/forged/receipt.json", Kind: "file", Mode: 0o644, Size: 7, ModTime: now.Format(time.RFC3339Nano), Digest: "sha256:239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"},
	})
	writeStageFile(t, layout, "session-recover", "payload.json", "payload")

	got, err := Recover(RecoverOptions{Profile: p, TargetDir: target, SessionID: "session-recover", Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("Recover(reserved target path) error = %v, want nil result with needs_repair", err)
	}
	if got.RepairNeeded != 1 {
		t.Fatalf("Recover(reserved target path).RepairNeeded = %d, want 1", got.RepairNeeded)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "sessions", "forged", "receipt.json")); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(forged receipt) error = %v, want os.ErrNotExist", err)
	}
}

func TestRecoverRejectsWrongProfileSnapshot(t *testing.T) {
	dir := t.TempDir()
	sourceA := filepath.Join(dir, "source-a")
	sourceB := filepath.Join(dir, "source-b")
	target := filepath.Join(dir, "target")
	profileA := profile.NewDefault("profile-a", "Profile A", sourceA, target)
	profileB := profile.NewDefault("profile-b", "Profile B", sourceB, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	writeStagedLocalSession(t, layout, target, profileA, "session-recover", []control.ManifestEntry{
		{Path: "file.txt", TargetPath: "file.txt", Kind: "file", Mode: 0o644, Size: 7, ModTime: now.Format(time.RFC3339Nano), Digest: "sha256:239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"},
	})
	writeStageFile(t, layout, "session-recover", "file.txt", "payload")

	got, err := Recover(RecoverOptions{Profile: profileB, TargetDir: target, SessionID: "session-recover", Now: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("Recover(wrong profile) error = %v, want nil result with needs_repair", err)
	}
	if got.RepairNeeded != 1 {
		t.Fatalf("Recover(wrong profile).RepairNeeded = %d, want 1", got.RepairNeeded)
	}
	if _, err := os.Stat(filepath.Join(target, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(wrong profile recovered file) error = %v, want os.ErrNotExist", err)
	}
	record, err := transaction.ReadSessionRecord(layout.RecordPath("session-recover"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(wrong profile repair) error = %v, want nil", err)
	}
	if record.State != transaction.StateNeedsRepair || !strings.Contains(record.Note, "profile") {
		t.Fatalf("wrong profile repair record = %#v, want needs_repair with profile note", record)
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

func TestRecoverReportsInvalidSessionRecords(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	badPath := layout.RecordPath("session-bad")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(badPath), err)
	}
	if err := os.WriteFile(badPath, []byte(`{"id":"session-bad","state":"unknown"}`), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", badPath, err)
	}

	got, err := Recover(RecoverOptions{Profile: p, TargetDir: target})
	if err != nil {
		t.Fatalf("Recover(invalid session record) error = %v, want nil", err)
	}
	if got.RepairNeeded != 1 || got.Inspected != 1 {
		t.Fatalf("Recover(invalid session record) = %#v, want one repair-needed item", got)
	}
	if len(got.Items) != 1 || got.Items[0].SessionID != "session-bad" || got.Items[0].Status != "needs_repair" || got.Items[0].State != "invalid" {
		t.Fatalf("Recover(invalid session record).Items = %#v, want invalid needs_repair item", got.Items)
	}
}

func writeStagedLocalSession(t *testing.T, layout transaction.Layout, target string, p profile.Profile, sessionID string, entries []control.ManifestEntry) {
	t.Helper()
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	writeRecoverRecord(t, layout, sessionID, transaction.StateStaged, now)
	profilePayload, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal(profile) error = %v, want nil", err)
	}
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + sessionID,
		ProfileID:  p.ProfileID,
		SessionID:  sessionID,
		CapturedAt: now.Format(time.RFC3339Nano),
		Profile:    profilePayload,
	}
	snapshotPath, err := control.Path(target, control.ArtifactProfileSnapshot, snapshot.ID)
	if err != nil {
		t.Fatalf("control.Path(profile snapshot) error = %v, want nil", err)
	}
	if err := control.WriteFile(snapshotPath, snapshot); err != nil {
		t.Fatalf("control.WriteFile(%q) error = %v, want nil", snapshotPath, err)
	}
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

func appendRawManifestEntry(t *testing.T, target string, sessionID string, entryJSON string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactManifest, sessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v, want nil", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	needle := []byte("\n  ]")
	replacement := []byte(",\n    " + entryJSON + "\n  ]")
	next := strings.Replace(string(data), string(needle), string(replacement), 1)
	if next == string(data) {
		t.Fatalf("appendRawManifestEntry(%q) could not find entries terminator in %s", entryJSON, path)
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
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
