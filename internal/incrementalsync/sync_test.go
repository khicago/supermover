package incrementalsync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
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

func TestScheduler_HelperMarkDoneDoesNotOverrideCanceledStateAcrossProcesses(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	result, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	entry := findPath(t, result.Enqueued, "a.txt")
	if _, err := scheduler.MarkInFlight(result.Scope, entry.ID); err != nil {
		t.Fatalf("MarkInFlight() error = %v, want nil", err)
	}

	unlock, err := scheduler.lockScope(result.Scope)
	if err != nil {
		t.Fatalf("lockScope() error = %v, want nil", err)
	}
	cmd, output, err := startIncrementalSyncHelperProcess(t, "mark-done", stateDir, result.Scope.ProfileID, result.Scope.TargetID, entry.ID)
	if err != nil {
		unlock()
		t.Fatalf("startIncrementalSyncHelperProcess(mark-done) error = %v, want nil", err)
	}
	mustWriteQueueEntryStateUnderLock(t, scheduler, result.Scope, entry.ID, parseTime(t, "2026-05-20T01:05:00Z"), func(entry *QueueEntry, ts string) {
		entry.Status = StatusCanceled
		entry.CanceledAt = ts
		entry.DoneAt = ""
		entry.FailedAt = ""
		entry.NextDueAt = ""
		entry.LastError = "operator canceled while helper blocked"
		entry.UpdatedAt = ts
	})
	unlock()
	err = cmd.Wait()
	if err == nil {
		t.Fatalf("helper mark-done unexpectedly succeeded, output=%s", output.String())
	}
	if !strings.Contains(output.String(), "cannot transition to done") {
		t.Fatalf("helper mark-done output = %q, want transition refusal", output.String())
	}
	summary, err := scheduler.Summary(result.Scope)
	if err != nil {
		t.Fatalf("Summary() error = %v, want nil", err)
	}
	if summary.Canceled != 1 || summary.Done != 0 {
		t.Fatalf("Summary() = %#v, want canceled terminal state retained", summary)
	}
}

func TestScheduler_HelperRetryDoesNotOverrideFailedStateAcrossProcesses(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	result, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	entry := findPath(t, result.Enqueued, "a.txt")
	if _, err := scheduler.MarkInFlight(result.Scope, entry.ID); err != nil {
		t.Fatalf("MarkInFlight() error = %v, want nil", err)
	}

	unlock, err := scheduler.lockScope(result.Scope)
	if err != nil {
		t.Fatalf("lockScope() error = %v, want nil", err)
	}
	cmd, output, err := startIncrementalSyncHelperProcess(t, "record-retry", stateDir, result.Scope.ProfileID, result.Scope.TargetID, entry.ID)
	if err != nil {
		unlock()
		t.Fatalf("startIncrementalSyncHelperProcess(record-retry) error = %v, want nil", err)
	}
	mustWriteQueueEntryStateUnderLock(t, scheduler, result.Scope, entry.ID, parseTime(t, "2026-05-20T01:05:00Z"), func(entry *QueueEntry, ts string) {
		entry.Status = StatusFailed
		entry.FailedAt = ts
		entry.CanceledAt = ""
		entry.DoneAt = ""
		entry.NextDueAt = ""
		entry.LastError = "operator terminal failure while helper blocked"
		entry.UpdatedAt = ts
	})
	unlock()
	err = cmd.Wait()
	if err == nil {
		t.Fatalf("helper record-retry unexpectedly succeeded, output=%s", output.String())
	}
	if !strings.Contains(output.String(), "cannot transition to backoff") {
		t.Fatalf("helper record-retry output = %q, want transition refusal", output.String())
	}
	summary, err := scheduler.Summary(result.Scope)
	if err != nil {
		t.Fatalf("Summary() error = %v, want nil", err)
	}
	if summary.Failed != 1 || summary.Backoff != 0 {
		t.Fatalf("Summary() = %#v, want failed terminal state retained", summary)
	}
}

func TestIncrementalSyncHelperProcess(t *testing.T) {
	if os.Getenv("SUPERMOVER_INCREMENTALSYNC_HELPER") != "1" {
		t.Skip("helper process only")
	}
	args := helperProcessArgs(os.Args)
	if len(args) < 5 {
		fmt.Fprintln(os.Stderr, "missing helper args")
		os.Exit(2)
	}
	command := args[0]
	stateDir := args[1]
	scope := Scope{ProfileID: args[2], TargetID: args[3]}
	entryID := args[4]

	scheduler, err := New(Options{StateDir: stateDir, Now: fixedClock("2026-05-20T01:10:00Z")})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch command {
	case "mark-done":
		_, err = scheduler.MarkDone(scope, entryID)
	case "record-retry":
		_, err = scheduler.RecordRetry(scope, RetryOptions{
			EntryID: entryID,
			Err:     errors.New("late helper retry"),
			Backoff: time.Minute,
		})
	default:
		fmt.Fprintf(os.Stderr, "unknown helper command %q\n", command)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
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

func TestScheduler_RunOncePublishesReadyEntriesAndWritesReceipt(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, ".hidden", "note.txt"), []byte("secret"))
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	now := parseTime(t, "2026-05-20T01:00:00Z")
	clock := func() time.Time { return now }
	scheduler := mustScheduler(t, stateDir, clock)
	p := testProfile("profile-a", source, "target-a")
	queued, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	scope := queued.Scope
	var transferred []QueueEntry

	result, err := scheduler.RunOnce(context.Background(), scope, RunOptions{
		SessionID: "sync-run-1",
		Backoff:   time.Minute,
		Transfer: func(ctx context.Context, entries []QueueEntry) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			transferred = append(transferred, entries...)
			now = parseTime(t, "2026-05-20T01:00:01Z")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunOnce() error = %v, want nil", err)
	}
	if result.Status != RunStatusPublished || len(result.Published) != len(queued.Enqueued) || len(transferred) != len(queued.Enqueued) {
		t.Fatalf("RunOnce() result = %#v transferred=%#v, want all ready entries published", result, transferred)
	}
	if !containsPath(result.Published, ".hidden/note.txt") {
		t.Fatalf("RunOnce() published entries = %#v, want hidden file preserved", result.Published)
	}
	summary, err := scheduler.Summary(scope)
	if err != nil {
		t.Fatalf("Summary() error = %v, want nil", err)
	}
	if summary.Done != len(queued.Enqueued) || summary.Ready != 0 || summary.InFlight != 0 {
		t.Fatalf("Summary() = %#v, want all entries done", summary)
	}
	file, err := os.Open(result.RunPath)
	if err != nil {
		t.Fatalf("Open(%q) error = %v, want nil", result.RunPath, err)
	}
	defer file.Close()
	persisted, err := DecodeRunResult(file)
	if err != nil {
		t.Fatalf("DecodeRunResult() error = %v, want nil", err)
	}
	if persisted.Schema != RunSchemaV1 || persisted.Status != RunStatusPublished || len(persisted.Published) != len(queued.Enqueued) {
		t.Fatalf("persisted run result = %#v, want published run receipt", persisted)
	}
}

func TestScheduler_RunOnceRetriesInFlightEntriesOnTransferFailure(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	now := parseTime(t, "2026-05-20T01:00:00Z")
	clock := func() time.Time { return now }
	scheduler := mustScheduler(t, stateDir, clock)
	p := testProfile("profile-a", source, "target-a")
	queued, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}

	result, err := scheduler.RunOnce(context.Background(), queued.Scope, RunOptions{
		SessionID: "sync-run-2",
		Backoff:   10 * time.Minute,
		Transfer: func(context.Context, []QueueEntry) error {
			now = parseTime(t, "2026-05-20T01:00:01Z")
			return errors.New("target drift refused publish")
		},
	})
	if err != nil {
		t.Fatalf("RunOnce(failing transfer) error = %v, want nil retry result", err)
	}
	if result.Status != RunStatusRetrying || result.Error != "target drift refused publish" || len(result.Retried) != len(queued.Enqueued) {
		t.Fatalf("RunOnce(failing transfer) result = %#v, want retrying all ready entries", result)
	}
	for _, entry := range result.Retried {
		if entry.Status != StatusBackoff || entry.Attempts != 1 || entry.NextDueAt != "2026-05-20T01:10:01Z" {
			t.Fatalf("retried entry = %#v, want backoff attempt due at +10m", entry)
		}
	}
	ready, err := scheduler.Ready(queued.Scope)
	if err != nil {
		t.Fatalf("Ready() error = %v, want nil", err)
	}
	if len(ready) != 0 {
		t.Fatalf("Ready() before backoff due = %#v, want empty", ready)
	}
	file, err := os.Open(result.RunPath)
	if err != nil {
		t.Fatalf("Open(%q) error = %v, want nil", result.RunPath, err)
	}
	defer file.Close()
	persisted, err := DecodeRunResult(file)
	if err != nil {
		t.Fatalf("DecodeRunResult() error = %v, want nil", err)
	}
	if persisted.Status != RunStatusRetrying || persisted.Error != "target drift refused publish" || persisted.Summary.Backoff != len(queued.Enqueued) {
		t.Fatalf("persisted run result = %#v, want retrying receipt with backoff summary", persisted)
	}
}

func TestScheduler_RunOnceRequeuesStaleInFlightBeforeTransfer(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	queued, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	entry := findPath(t, queued.Enqueued, "a.txt")
	if _, err := scheduler.MarkInFlight(queued.Scope, entry.ID); err != nil {
		t.Fatalf("MarkInFlight() error = %v, want nil", err)
	}

	result, err := scheduler.RunOnce(context.Background(), queued.Scope, RunOptions{
		SessionID: "sync-run-3",
		Backoff:   time.Minute,
		Transfer: func(context.Context, []QueueEntry) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunOnce(recovering stale in-flight) error = %v, want nil", err)
	}
	if result.Recovered != 1 || !containsID(result.Published, entry.ID) {
		t.Fatalf("RunOnce(recovering stale in-flight) = %#v, want recovered entry published", result)
	}
}

func TestScheduler_RunOnceRefusesExistingRunReceipt(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	queued, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	runPath, err := scheduler.RunPath(queued.Scope, "sync-run-existing")
	if err != nil {
		t.Fatalf("RunPath() error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Dir(runPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v, want nil", filepath.Dir(runPath), err)
	}
	original := []byte(`{"preserve":"operator evidence"}`)
	if err := os.WriteFile(runPath, original, 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", runPath, err)
	}

	_, err = scheduler.RunOnce(context.Background(), queued.Scope, RunOptions{
		SessionID: "sync-run-existing",
		Backoff:   time.Minute,
		Transfer:  func(context.Context, []QueueEntry) error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "run receipt already exists") {
		t.Fatalf("RunOnce(existing receipt) error = %v, want already exists refusal", err)
	}
	got, err := os.ReadFile(runPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", runPath, err)
	}
	if string(got) != string(original) {
		t.Fatalf("existing run receipt changed: got %s want %s", string(got), string(original))
	}
}

func TestScheduler_RunResultsLoadsReceiptsAndReportsProblems(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	queued, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	result, err := scheduler.RunOnce(context.Background(), queued.Scope, RunOptions{
		SessionID: "sync-run-list",
		Backoff:   time.Minute,
		Transfer:  func(context.Context, []QueueEntry) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunOnce() error = %v, want nil", err)
	}

	runs, problems, err := scheduler.RunResults(queued.Scope)
	if err != nil {
		t.Fatalf("RunResults() error = %v, want nil", err)
	}
	if len(problems) != 0 || len(runs) != 1 || runs[0].SessionID != result.SessionID || runs[0].Status != RunStatusPublished {
		t.Fatalf("RunResults() runs=%#v problems=%#v, want one published run without problems", runs, problems)
	}

	runsDir, err := scheduler.RunsDir(queued.Scope)
	if err != nil {
		t.Fatalf("RunsDir() error = %v, want nil", err)
	}
	badPath := filepath.Join(runsDir, "sync-run-bad.json")
	if err := os.WriteFile(badPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", badPath, err)
	}

	runs, problems, err = scheduler.RunResults(queued.Scope)
	if err != nil {
		t.Fatalf("RunResults(bad receipt) error = %v, want nil", err)
	}
	if len(runs) != 1 || runs[0].SessionID != result.SessionID {
		t.Fatalf("RunResults(bad receipt) runs=%#v, want valid run preserved", runs)
	}
	if len(problems) != 1 || problems[0].Path != badPath || !strings.Contains(problems[0].Error, "unexpected EOF") {
		t.Fatalf("RunResults(bad receipt) problems=%#v, want corrupt receipt problem", problems)
	}

	result.RunPath = filepath.Join(runsDir, "somewhere-else.json")
	mismatchPath := filepath.Join(runsDir, "sync-run-mismatch.json")
	result.SessionID = "sync-run-mismatch"
	if err := scheduler.writeRunResult(mismatchPath, result); err != nil {
		t.Fatalf("writeRunResult(%q) error = %v, want nil", mismatchPath, err)
	}
	_, problems, err = scheduler.RunResults(queued.Scope)
	if err != nil {
		t.Fatalf("RunResults(mismatched run_path) error = %v, want nil", err)
	}
	if !containsArtifactProblem(problems, mismatchPath, "does not match artifact path") {
		t.Fatalf("RunResults(mismatched run_path) problems=%#v, want run_path mismatch problem", problems)
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

func TestScheduler_MarkFailedHidesEntryFromReady(t *testing.T) {
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

	now = parseTime(t, "2026-05-20T01:05:00Z")
	failed, err := scheduler.MarkFailed(result.Scope, entry.ID, "operator terminal failure")
	if err != nil {
		t.Fatalf("MarkFailed() error = %v, want nil", err)
	}
	if failed.Status != StatusFailed || failed.FailedAt != "2026-05-20T01:05:00Z" || failed.LastError != "operator terminal failure" || failed.NextDueAt != "" {
		t.Fatalf("failed entry = %#v, want failed status with durable reason and no retry due time", failed)
	}
	ready, err := scheduler.Ready(result.Scope)
	if err != nil {
		t.Fatalf("Ready() error = %v, want nil", err)
	}
	if containsID(ready, entry.ID) {
		t.Fatalf("Ready() contains failed entry")
	}
	summary, err := scheduler.Summary(result.Scope)
	if err != nil {
		t.Fatalf("Summary() error = %v, want nil", err)
	}
	if summary.Failed != 1 || summary.Ready != 0 || summary.Total != 1 || summary.Roots[0].Failed != 1 {
		t.Fatalf("Summary() = %#v, want one failed file and no ready entries", summary)
	}
}

func TestScheduler_MarkDoneRequiresInFlightState(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	result, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	entry := findPath(t, result.Enqueued, "a.txt")

	_, err = scheduler.MarkDone(result.Scope, entry.ID)

	if err == nil || !strings.Contains(err.Error(), "cannot transition to done") {
		t.Fatalf("MarkDone(queued) error = %v, want transition refusal", err)
	}
}

func TestScheduler_CancelBlocksLaterMarkDoneAndRetry(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	result, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	entry := findPath(t, result.Enqueued, "a.txt")
	if _, err := scheduler.MarkInFlight(result.Scope, entry.ID); err != nil {
		t.Fatalf("MarkInFlight() error = %v, want nil", err)
	}
	if _, err := scheduler.Cancel(result.Scope, entry.ID, "operator canceled during transfer"); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}

	_, err = scheduler.MarkDone(result.Scope, entry.ID)
	if err == nil || !strings.Contains(err.Error(), "cannot transition to done") {
		t.Fatalf("MarkDone(canceled) error = %v, want transition refusal", err)
	}
	_, err = scheduler.RecordRetry(result.Scope, RetryOptions{
		EntryID: entry.ID,
		Err:     errors.New("late retry"),
		Backoff: time.Minute,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot transition to backoff") {
		t.Fatalf("RecordRetry(canceled) error = %v, want transition refusal", err)
	}
	summary, err := scheduler.Summary(result.Scope)
	if err != nil {
		t.Fatalf("Summary() error = %v, want nil", err)
	}
	if summary.Canceled != 1 || summary.Done != 0 || summary.Backoff != 0 {
		t.Fatalf("Summary() = %#v, want canceled durable outcome retained", summary)
	}
}

func TestScheduler_MarkFailedBlocksLaterMarkDoneAndRetry(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	mustWriteFile(t, filepath.Join(source, "a.txt"), []byte("alpha"))
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	p := testProfile("profile-a", source, "target-a")
	result, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	entry := findPath(t, result.Enqueued, "a.txt")
	if _, err := scheduler.MarkInFlight(result.Scope, entry.ID); err != nil {
		t.Fatalf("MarkInFlight() error = %v, want nil", err)
	}
	if _, err := scheduler.MarkFailed(result.Scope, entry.ID, "operator terminal failure"); err != nil {
		t.Fatalf("MarkFailed() error = %v, want nil", err)
	}

	_, err = scheduler.MarkDone(result.Scope, entry.ID)
	if err == nil || !strings.Contains(err.Error(), "cannot transition to done") {
		t.Fatalf("MarkDone(failed) error = %v, want transition refusal", err)
	}
	_, err = scheduler.RecordRetry(result.Scope, RetryOptions{
		EntryID: entry.ID,
		Err:     errors.New("late retry"),
		Backoff: time.Minute,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot transition to backoff") {
		t.Fatalf("RecordRetry(failed) error = %v, want transition refusal", err)
	}
	summary, err := scheduler.Summary(result.Scope)
	if err != nil {
		t.Fatalf("Summary() error = %v, want nil", err)
	}
	if summary.Failed != 1 || summary.Done != 0 || summary.Backoff != 0 {
		t.Fatalf("Summary() = %#v, want failed durable outcome retained", summary)
	}
}

func TestScheduler_FailedEntryReopensWhenSourceChanges(t *testing.T) {
	source := t.TempDir()
	stateDir := t.TempDir()
	path := filepath.Join(source, "a.txt")
	mustWriteFile(t, path, []byte("alpha"))
	now := parseTime(t, "2026-05-20T01:00:00Z")
	clock := func() time.Time { return now }
	scheduler := mustScheduler(t, stateDir, clock)
	p := testProfile("profile-a", source, "target-a")
	result, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}
	entry := findPath(t, result.Enqueued, "a.txt")

	now = parseTime(t, "2026-05-20T01:01:00Z")
	if _, err := scheduler.MarkFailed(result.Scope, entry.ID, "operator terminal failure"); err != nil {
		t.Fatalf("MarkFailed() error = %v, want nil", err)
	}
	repeated, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("repeated Enqueue() error = %v, want nil", err)
	}
	if containsID(repeated.Enqueued, entry.ID) {
		t.Fatalf("repeated Enqueue() requeued failed unchanged entry")
	}

	now = parseTime(t, "2026-05-20T01:02:00Z")
	mustWriteFile(t, path, []byte("changed"))
	changed, err := scheduler.Enqueue(mustSnapshot(t, p, p.Roots[0]))
	if err != nil {
		t.Fatalf("changed Enqueue() error = %v, want nil", err)
	}
	requeued := findPath(t, changed.Enqueued, "a.txt")
	if requeued.ID != entry.ID || requeued.Status != StatusQueued || requeued.FailedAt != "" || requeued.LastError != "" || requeued.NextDueAt != "" || requeued.Attempts != 0 {
		t.Fatalf("changed requeued entry = %#v, want same id clean queued entry", requeued)
	}
	summary, err := scheduler.Summary(result.Scope)
	if err != nil {
		t.Fatalf("Summary() error = %v, want nil", err)
	}
	if summary.Failed != 0 || summary.Queued != 1 || summary.Ready != 1 {
		t.Fatalf("Summary(after changed source) = %#v, want failed evidence reopened as queued", summary)
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

func containsPath(entries []QueueEntry, path string) bool {
	for _, entry := range entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}

func containsArtifactProblem(problems []ArtifactProblem, path string, text string) bool {
	for _, problem := range problems {
		if problem.Path == path && strings.Contains(problem.Error, text) {
			return true
		}
	}
	return false
}

func startIncrementalSyncHelperProcess(t *testing.T, command, stateDir, profileID, targetID, entryID string) (*exec.Cmd, *bytes.Buffer, error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestIncrementalSyncHelperProcess", "--", command, stateDir, profileID, targetID, entryID)
	cmd.Env = append(os.Environ(), "SUPERMOVER_INCREMENTALSYNC_HELPER=1")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	waitForHelperLockAttempt(t, stateDir, profileID, targetID)
	return cmd, &output, nil
}

func waitForHelperLockAttempt(t *testing.T, stateDir, profileID, targetID string) {
	t.Helper()
	scope := Scope{ProfileID: profileID, TargetID: targetID}
	scheduler := mustScheduler(t, stateDir, fixedClock("2026-05-20T01:00:00Z"))
	statePath, err := scheduler.StatePath(scope)
	if err != nil {
		t.Fatalf("StatePath() error = %v, want nil", err)
	}
	lockPath := filepath.Join(filepath.Dir(statePath), "queue.lock")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(lockPath); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("helper did not create queue lock at %s", lockPath)
}

func mustWriteQueueEntryStateUnderLock(t *testing.T, scheduler *Scheduler, scope Scope, entryID string, now time.Time, mutate func(*QueueEntry, string)) {
	t.Helper()
	state, statePath, err := scheduler.loadExisting(scope)
	if err != nil {
		t.Fatalf("loadExisting() error = %v, want nil", err)
	}
	ts := formatTime(now)
	found := false
	for i := range state.Entries {
		if state.Entries[i].ID != entryID {
			continue
		}
		mutate(&state.Entries[i], ts)
		found = true
		break
	}
	if !found {
		t.Fatalf("entry %q not found in locked state", entryID)
	}
	state.UpdatedAt = ts
	if err := scheduler.writeState(statePath, state); err != nil {
		t.Fatalf("writeState() error = %v, want nil", err)
	}
}

func helperProcessArgs(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			return args[i+1:]
		}
	}
	return nil
}
