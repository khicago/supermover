package incrementalsync

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/scan"
)

func TestOpenDoesNotCreateMissingStateDir(t *testing.T) {
	parent := t.TempDir()
	stateDir := filepath.Join(parent, "missing", "incremental-sync")

	scheduler, err := Open(Options{StateDir: stateDir, Now: fixedClock("2026-05-20T01:00:00Z")})
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	if _, err := os.Lstat(stateDir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Lstat(%q) error = %v, want fs.ErrNotExist", stateDir, err)
	}

	scope := Scope{ProfileID: "profile-a", TargetID: "target-a"}
	_, err = scheduler.Ready(scope)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Ready(missing state dir) error = %v, want fs.ErrNotExist", err)
	}
	_, err = scheduler.Summary(scope)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Summary(missing state dir) error = %v, want fs.ErrNotExist", err)
	}
	if _, err := os.Lstat(stateDir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Lstat(%q) after reads error = %v, want fs.ErrNotExist", stateDir, err)
	}
}

func TestScheduler_EnqueuePersistsChangedFiles(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, ".hidden", "note.txt"), []byte("secret"))
	mustWriteFile(t, filepath.Join(source, "docs", "a.txt"), []byte("alpha"))

	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:02:03Z"))
	p := testProfile("profile-a", source, "target-a")
	result, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}

	if len(result.Enqueued) != 4 {
		t.Fatalf("Enqueued length = %d, want 4 entries for dirs and files", len(result.Enqueued))
	}
	paths := entryPaths(result.Enqueued)
	wantPaths := []string{".hidden", ".hidden/note.txt", "docs", "docs/a.txt"}
	if strings.Join(paths, ",") != strings.Join(wantPaths, ",") {
		t.Fatalf("Enqueued paths = %#v, want %#v", paths, wantPaths)
	}
	for _, entry := range result.Enqueued {
		if entry.ProfileID != "profile-a" || entry.TargetID != "target-a" || entry.Root != "root" {
			t.Fatalf("entry scope = profile %q target %q root %q, want profile-a target-a root", entry.ProfileID, entry.TargetID, entry.Root)
		}
		if entry.Status != StatusQueued {
			t.Fatalf("entry status = %q, want queued", entry.Status)
		}
		if entry.Path == "docs/a.txt" && (entry.Digest == "" || entry.ModTime == "") {
			t.Fatalf("regular entry = %#v, want digest and modtime", entry)
		}
	}

	data, err := os.ReadFile(result.StatePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", result.StatePath, err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("state file does not end in newline: %q", string(data))
	}
	var persisted State
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("json.Unmarshal(state) error = %v, want nil", err)
	}
	if persisted.Schema != SchemaV1 || len(persisted.Entries) != 4 {
		t.Fatalf("persisted state = %#v, want schema and 4 entries", persisted)
	}
}

func TestScheduler_EnqueueDeduplicatesUnchangedEntries(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))

	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	first, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("first Enqueue() error = %v, want nil", err)
	}

	second, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("second Enqueue() error = %v, want nil", err)
	}

	if len(second.Enqueued) != 0 {
		t.Fatalf("second Enqueued length = %d, want 0", len(second.Enqueued))
	}
	if len(second.Skipped) != len(first.Enqueued) {
		t.Fatalf("second Skipped length = %d, want %d", len(second.Skipped), len(first.Enqueued))
	}
	for _, skipped := range second.Skipped {
		if skipped.Reason != "unchanged" {
			t.Fatalf("skipped reason = %q, want unchanged", skipped.Reason)
		}
	}
}

func TestScheduler_RecordRetryBackoffAffectsReadiness(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	now := parseTime(t, "2026-05-20T01:00:00Z")
	clock := func() time.Time { return now }
	scheduler := mustScheduler(t, stateDir, clock)
	p := testProfile("profile-a", source, "target-a")
	result, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	entry := findPath(t, result.Enqueued, "a.txt")

	retried, err := scheduler.RecordRetry(result.Scope, RetryOptions{
		EntryID: entry.ID,
		Err:     errors.New("temporary network refusal"),
		Backoff: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("RecordRetry() error = %v, want nil", err)
	}
	if retried.Status != StatusBackoff || retried.Attempts != 1 || retried.NextDueAt != "2026-05-20T01:10:00Z" {
		t.Fatalf("retried entry = %#v, want backoff attempt due at +10m", retried)
	}
	ready, err := scheduler.Ready(result.Scope)
	if err != nil {
		t.Fatalf("Ready() error = %v, want nil", err)
	}
	if containsID(ready, entry.ID) {
		t.Fatalf("Ready() contains retried entry before due time")
	}

	now = parseTime(t, "2026-05-20T01:10:00Z")
	ready, err = scheduler.Ready(result.Scope)
	if err != nil {
		t.Fatalf("Ready() after due error = %v, want nil", err)
	}
	if !containsID(ready, entry.ID) {
		t.Fatalf("Ready() after due does not contain retried entry")
	}
}

func TestScheduler_CancelHidesEntryUntilSourceChanges(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	path := filepath.Join(source, "a.txt")
	mustWriteFile(t, path, []byte("alpha"))
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	result, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	entry := findPath(t, result.Enqueued, "a.txt")

	canceled, err := scheduler.Cancel(result.Scope, entry.ID, "operator canceled")
	if err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	if canceled.Status != StatusCanceled || canceled.CanceledAt == "" || canceled.LastError != "operator canceled" {
		t.Fatalf("canceled entry = %#v, want canceled with reason", canceled)
	}
	ready, err := scheduler.Ready(result.Scope)
	if err != nil {
		t.Fatalf("Ready() error = %v, want nil", err)
	}
	if containsID(ready, entry.ID) {
		t.Fatalf("Ready() contains canceled entry")
	}

	repeated, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("repeated Enqueue() error = %v, want nil", err)
	}
	if containsID(repeated.Enqueued, entry.ID) {
		t.Fatalf("repeated Enqueue() requeued canceled unchanged entry")
	}

	mustWriteFile(t, path, []byte("changed"))
	changed, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("changed Enqueue() error = %v, want nil", err)
	}
	requeued := findPath(t, changed.Enqueued, "a.txt")
	if requeued.ID != entry.ID || requeued.Status != StatusQueued || requeued.CanceledAt != "" {
		t.Fatalf("changed requeued entry = %#v, want same id active queued entry", requeued)
	}
}

func TestScheduler_RejectsPaddedScopeAndRootIDs(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	tests := []struct {
		name   string
		mutate func(*Snapshot)
		want   string
	}{
		{
			name: "profile id",
			mutate: func(snapshot *Snapshot) {
				snapshot.Profile.ProfileID = "profile-a "
			},
			want: "profile_id must not be padded",
		},
		{
			name: "target id",
			mutate: func(snapshot *Snapshot) {
				snapshot.Profile.Target.TargetID = "target-a "
			},
			want: "target_id must not be padded",
		},
		{
			name: "root id",
			mutate: func(snapshot *Snapshot) {
				snapshot.Profile.Roots[0].ID = "root "
				snapshot.Root.ID = "root "
			},
			want: "root id must not be padded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
			p := testProfile("profile-a", source, "target-a")
			snapshot := mustSnapshot(t, p, p.Roots[0])
			tt.mutate(&snapshot)

			_, err := scheduler.Enqueue(snapshot)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Enqueue() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestScheduler_RejectsForeignScanRoot(t *testing.T) {
	source := t.TempDir()
	foreign := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	snapshot := mustSnapshot(t, p, p.Roots[0])
	snapshot.Scan.Root = foreign

	_, err := scheduler.Enqueue(snapshot)

	if err == nil || !strings.Contains(err.Error(), "does not match snapshot root") {
		t.Fatalf("Enqueue(foreign scan root) error = %v, want root mismatch", err)
	}
}

func TestScheduler_PersistsAndComparesSymlinkTargets(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	base := scan.Result{
		Root: p.Roots[0].Path,
		Entries: []scan.Entry{
			{Path: ".", Kind: scan.KindDir, ModTime: parseTime(t, "2026-05-20T01:00:00Z")},
			{Path: "link", Kind: scan.KindSymlink, SymlinkTarget: "target-v1", ModTime: parseTime(t, "2026-05-20T01:00:01Z")},
		},
	}

	first, err := scheduler.Enqueue(Snapshot{Profile: p, Root: p.Roots[0], Scan: base})
	if err != nil {
		t.Fatalf("first Enqueue() error = %v, want nil", err)
	}
	link := findPath(t, first.Enqueued, "link")
	if link.SymlinkTarget != "target-v1" {
		t.Fatalf("first symlink target = %q, want target-v1", link.SymlinkTarget)
	}
	var persisted State
	data, err := os.ReadFile(first.StatePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", first.StatePath, err)
	}
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("json.Unmarshal(state) error = %v, want nil", err)
	}
	if got := findPath(t, persisted.Entries, "link").SymlinkTarget; got != "target-v1" {
		t.Fatalf("persisted symlink target = %q, want target-v1", got)
	}

	base.Entries[1].SymlinkTarget = "target-v2"
	second, err := scheduler.Enqueue(Snapshot{Profile: p, Root: p.Roots[0], Scan: base})
	if err != nil {
		t.Fatalf("second Enqueue() error = %v, want nil", err)
	}
	requeued := findPath(t, second.Enqueued, "link")
	if requeued.ID != link.ID || requeued.SymlinkTarget != "target-v2" {
		t.Fatalf("requeued symlink = %#v, want same id with updated target-v2", requeued)
	}
}

func TestScheduler_EnqueueUpgradesLegacySymlinkQueueEntries(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	scope := Scope{ProfileID: p.ProfileID, TargetID: p.Target.TargetID}
	statePath := writeLegacySymlinkQueue(t, scheduler, scope, p.Roots[0].ID, "link")
	snapshot := Snapshot{
		Profile: p,
		Root:    p.Roots[0],
		Scan: scan.Result{
			Root: p.Roots[0].Path,
			Entries: []scan.Entry{
				{Path: ".", Kind: scan.KindDir, ModTime: parseTime(t, "2026-05-20T01:00:00Z")},
				{Path: "link", Kind: scan.KindSymlink, SymlinkTarget: "target-v2", ModTime: parseTime(t, "2026-05-20T01:00:01Z")},
			},
		},
	}

	result, err := scheduler.Enqueue(snapshot)
	if err != nil {
		t.Fatalf("Enqueue(legacy symlink queue) error = %v, want nil", err)
	}
	requeued := findPath(t, result.Enqueued, "link")
	if requeued.SymlinkTarget != "target-v2" {
		t.Fatalf("upgraded symlink target = %q, want target-v2", requeued.SymlinkTarget)
	}
	var persisted State
	persistedData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", statePath, err)
	}
	if err := json.Unmarshal(persistedData, &persisted); err != nil {
		t.Fatalf("json.Unmarshal(persisted) error = %v, want nil", err)
	}
	if got := findPath(t, persisted.Entries, "link").SymlinkTarget; got != "target-v2" {
		t.Fatalf("persisted upgraded symlink target = %q, want target-v2", got)
	}
}

func TestScheduler_DropsLegacySymlinkQueueEntryWhenNotRescanned(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	scope := Scope{ProfileID: p.ProfileID, TargetID: p.Target.TargetID}
	writeLegacySymlinkQueue(t, scheduler, scope, p.Roots[0].ID, "link")
	snapshot := Snapshot{
		Profile: p,
		Root:    p.Roots[0],
		Scan: scan.Result{
			Root: p.Roots[0].Path,
			Entries: []scan.Entry{
				{Path: ".", Kind: scan.KindDir, ModTime: parseTime(t, "2026-05-20T01:00:00Z")},
			},
		},
	}

	result, err := scheduler.Enqueue(snapshot)
	if err != nil {
		t.Fatalf("Enqueue(legacy missing symlink) error = %v, want nil", err)
	}
	if len(result.Enqueued) != 0 || len(result.Skipped) != 1 || result.Skipped[0].Reason != "legacy_missing_symlink_target" {
		t.Fatalf("Enqueue(legacy missing symlink) result = %#v, want skipped legacy entry", result)
	}
	summary, err := scheduler.Summary(scope)
	if err != nil {
		t.Fatalf("Summary(after dropping legacy symlink) error = %v, want nil", err)
	}
	if summary.Total != 0 {
		t.Fatalf("Summary(after dropping legacy symlink).Total = %d, want 0", summary.Total)
	}
}

func TestScheduler_ReadyHidesLegacySymlinkQueueEntries(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	scope := Scope{ProfileID: p.ProfileID, TargetID: p.Target.TargetID}
	writeLegacySymlinkQueue(t, scheduler, scope, p.Roots[0].ID, "link")

	ready, err := scheduler.Ready(scope)
	if err != nil {
		t.Fatalf("Ready(legacy symlink queue) error = %v, want nil", err)
	}
	if len(ready) != 0 {
		t.Fatalf("Ready(legacy symlink queue) = %#v, want no executable entries", ready)
	}
}

func TestScheduler_CorruptQueueHandling(t *testing.T) {
	stateDir := t.TempDir()
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	scope := Scope{ProfileID: "profile-a", TargetID: "target-a"}
	statePath, err := scheduler.StatePath(scope)
	if err != nil {
		t.Fatalf("StatePath() error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v, want nil", filepath.Dir(statePath), err)
	}
	if err := os.WriteFile(statePath, []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", statePath, err)
	}

	_, err = scheduler.Summary(scope)
	if !errors.Is(err, ErrCorruptQueue) {
		t.Fatalf("Summary() error = %v, want ErrCorruptQueue", err)
	}
}

func TestScheduler_ProfileTargetScopeSeparation(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))

	profileA := testProfile("profile-a", source, "target-a")
	profileB := testProfile("profile-b", source, "target-a")
	profileTargetB := testProfile("profile-a", source, "target-b")
	resultA, err := scheduler.Enqueue(mustSnapshot(t, profileA, profileA.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue(profile A) error = %v, want nil", err)
	}
	resultB, err := scheduler.Enqueue(mustSnapshot(t, profileB, profileB.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue(profile B) error = %v, want nil", err)
	}
	resultTargetB, err := scheduler.Enqueue(mustSnapshot(t, profileTargetB, profileTargetB.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue(target B) error = %v, want nil", err)
	}

	if resultA.StatePath == resultB.StatePath || resultA.StatePath == resultTargetB.StatePath || resultB.StatePath == resultTargetB.StatePath {
		t.Fatalf("scope state paths are not distinct: %q %q %q", resultA.StatePath, resultB.StatePath, resultTargetB.StatePath)
	}
	summaryA, err := scheduler.Summary(resultA.Scope)
	if err != nil {
		t.Fatalf("Summary(A) error = %v, want nil", err)
	}
	summaryB, err := scheduler.Summary(resultB.Scope)
	if err != nil {
		t.Fatalf("Summary(B) error = %v, want nil", err)
	}
	if summaryA.ProfileID != "profile-a" || summaryB.ProfileID != "profile-b" || summaryA.TargetID != summaryB.TargetID {
		t.Fatalf("summaries = %#v %#v, want separated profile scopes", summaryA, summaryB)
	}
}

func TestScheduler_RejectsUnsafeDataPaths(t *testing.T) {
	stateDir := t.TempDir()
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", t.TempDir(), "target-a")
	snapshot := Snapshot{
		Profile: p,
		Root:    p.Roots[0],
		Scan: scan.Result{
			Root: p.Roots[0].Path,
			Entries: []scan.Entry{
				{Path: ".", Kind: scan.KindDir, ModTime: parseTime(t, "2026-05-20T01:00:00Z")},
				{Path: "../escape.txt", Kind: scan.KindRegular, Digest: "sha256:abc", ModTime: parseTime(t, "2026-05-20T01:00:00Z")},
			},
		},
	}

	_, err := scheduler.Enqueue(snapshot)
	if !errors.Is(err, pathguard.ErrUnsafePath) {
		t.Fatalf("Enqueue(unsafe path) error = %v, want ErrUnsafePath", err)
	}
}

func TestScheduler_SkipsReservedControlAndUnsupportedEntries(t *testing.T) {
	stateDir := t.TempDir()
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", t.TempDir(), "target-a")
	snapshot := Snapshot{
		Profile: p,
		Root:    p.Roots[0],
		Scan: scan.Result{
			Root: p.Roots[0].Path,
			Entries: []scan.Entry{
				{Path: ".", Kind: scan.KindDir, ModTime: parseTime(t, "2026-05-20T01:00:00Z")},
				{Path: ".supermover/control.json", Kind: scan.KindRegular, Digest: "sha256:abc", ModTime: parseTime(t, "2026-05-20T01:00:00Z")},
				{Path: "pipe", Kind: scan.KindSpecial, ModTime: parseTime(t, "2026-05-20T01:00:00Z")},
			},
		},
	}

	result, err := scheduler.Enqueue(snapshot)
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	if len(result.Enqueued) != 0 || len(result.Skipped) != 2 {
		t.Fatalf("result = %#v, want no enqueued and two skipped entries", result)
	}
	reasons := map[string]string{}
	for _, skipped := range result.Skipped {
		reasons[skipped.Path] = skipped.Reason
	}
	if reasons[".supermover/control.json"] != "reserved_control_path" || reasons["pipe"] != "unsupported_kind" {
		t.Fatalf("skip reasons = %#v, want reserved_control_path and unsupported_kind", reasons)
	}
}

func mustScheduler(t *testing.T, stateDir string, clock Clock) *Scheduler {
	t.Helper()
	scheduler, err := New(Options{StateDir: stateDir, Now: clock})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	return scheduler
}

func mustSnapshot(t *testing.T, p profile.Profile, root profile.Root) Snapshot {
	t.Helper()
	result, err := scan.Scan(root.Path)
	if err != nil {
		t.Fatalf("Scan(%q) error = %v, want nil", root.Path, err)
	}
	return Snapshot{Profile: p, Root: root, Scan: result}
}

func writeLegacySymlinkQueue(t *testing.T, scheduler *Scheduler, scope Scope, rootID string, path string) string {
	t.Helper()
	legacy := State{
		Schema:    SchemaV1,
		Scope:     scope,
		UpdatedAt: "2026-05-20T00:00:00Z",
		Entries: []QueueEntry{
			{
				ID:         entryID(scope, rootID, path),
				ProfileID:  scope.ProfileID,
				TargetID:   scope.TargetID,
				Root:       rootID,
				Path:       path,
				Kind:       scan.KindSymlink,
				ModTime:    "2026-05-20T00:00:00Z",
				EnqueuedAt: "2026-05-20T00:00:00Z",
				Status:     StatusQueued,
				UpdatedAt:  "2026-05-20T00:00:00Z",
			},
		},
	}
	statePath, err := scheduler.StatePath(scope)
	if err != nil {
		t.Fatalf("StatePath() error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v, want nil", filepath.Dir(statePath), err)
	}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(legacy) error = %v, want nil", err)
	}
	if err := os.WriteFile(statePath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", statePath, err)
	}
	return statePath
}

func testProfile(profileID, sourceRoot, targetID string) profile.Profile {
	p := profile.NewDefault(profileID, profileID, sourceRoot, filepath.Join(os.TempDir(), targetID))
	p.Target.TargetID = targetID
	p.Target.LocalPath = filepath.Join(os.TempDir(), targetID)
	return p
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
}

func fixedClock(value string) Clock {
	t := mustParseTime(value)
	return func() time.Time { return t }
}

func parseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v, want nil", value, err)
	}
	return parsed
}

func mustParseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func entryPaths(entries []QueueEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	return paths
}

func findPath(t *testing.T, entries []QueueEntry, path string) QueueEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.Path == path {
			return entry
		}
	}
	t.Fatalf("path %q not found in entries %#v", path, entries)
	return QueueEntry{}
}

func containsID(entries []QueueEntry, id string) bool {
	for _, entry := range entries {
		if entry.ID == id {
			return true
		}
	}
	return false
}
