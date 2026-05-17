package receiver

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/transaction"
)

func TestFileStoreBeginStatusChunkCommit(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)
	store := FileStore{TargetRoot: root, Now: func() time.Time { return now }}
	req := validBeginRequest([]byte("hello world"))

	begin, err := store.Begin(req)
	if err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	if begin.State != protocol.SessionStateValidated {
		t.Errorf("FileStore.Begin(%+v).State = %q, want %q", req, begin.State, protocol.SessionStateValidated)
	}

	first := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello ")}
	resp, err := store.AppendChunk(first)
	if err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", first, err)
	}
	if resp.CommittedSize != 6 || resp.Complete {
		t.Errorf("FileStore.AppendChunk(%+v) = %+v, want committed size 6 and incomplete", first, resp)
	}

	replay, err := store.AppendChunk(first)
	if err != nil {
		t.Fatalf("FileStore.AppendChunk(replay %+v) error = %v, want nil", first, err)
	}
	if replay.ChunkState != protocol.ChunkStateDuplicate || replay.CommittedSize != 6 {
		t.Errorf("FileStore.AppendChunk(replay %+v) = %+v, want duplicate at size 6", first, replay)
	}

	second := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 6, Data: []byte("world"), Final: true}
	resp, err = store.AppendChunk(second)
	if err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", second, err)
	}
	if !resp.Complete || resp.CommittedSize != 11 {
		t.Errorf("FileStore.AppendChunk(%+v) = %+v, want complete size 11", second, resp)
	}

	status, err := store.Status(req.SessionID)
	if err != nil {
		t.Fatalf("FileStore.Status(%q) error = %v, want nil", req.SessionID, err)
	}
	if len(status.Files) != 1 || !status.Files[0].Complete {
		t.Fatalf("FileStore.Status(%q).Files = %+v, want one complete file", req.SessionID, status.Files)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: now.Add(time.Minute)}
	commit, err := store.Commit(commitReq)
	if err != nil {
		t.Fatalf("FileStore.Commit(%+v) error = %v, want nil", commitReq, err)
	}
	if commit.State != protocol.SessionStatePublished {
		t.Errorf("FileStore.Commit(%+v).State = %q, want %q", commitReq, commit.State, protocol.SessionStatePublished)
	}

	got, err := os.ReadFile(filepath.Join(root, "docs", "a.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile(published file) error = %v, want nil", err)
	}
	if string(got) != "hello world" {
		t.Errorf("published file = %q, want %q", got, "hello world")
	}
	info, err := os.Stat(filepath.Join(root, "docs", "a.txt"))
	if err != nil {
		t.Fatalf("os.Stat(published file) error = %v, want nil", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("published file mode = %v, want 0600 from manifest", info.Mode().Perm())
	}

	layout := transaction.NewLayout(control.ControlDir(root))
	record, err := transaction.ReadSessionRecord(layout.RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(%q) error = %v, want nil", layout.RecordPath(req.SessionID), err)
	}
	if record.State != transaction.StatePublished {
		t.Errorf("transaction.ReadSessionRecord(%q).State = %q, want %q", layout.RecordPath(req.SessionID), record.State, transaction.StatePublished)
	}
	if _, err := os.Stat(filepath.Join(control.ControlDir(root), "sessions", req.SessionID, "manifest.json")); err != nil {
		t.Errorf("os.Stat(manifest artifact) error = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(control.ControlDir(root), "sessions", req.SessionID, "receipt.json")); err != nil {
		t.Errorf("os.Stat(receipt artifact) error = %v, want nil", err)
	}
	manifest := readControlDoc[control.Manifest](t, root, control.ArtifactManifest, req.SessionID)
	if len(manifest.Entries) < 2 || manifest.Entries[1].Mode != 0o600 {
		t.Fatalf("manifest file mode = %#v, want 0600", manifest.Entries)
	}
	if _, err := os.Stat(filepath.Join(control.ControlDir(root), "locks", "target.lock")); err != nil {
		t.Fatalf("os.Stat(target lock) error = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(control.ControlDir(root), "locks", req.SessionID+".lock")); err != nil {
		t.Fatalf("os.Stat(session lock) error = %v, want nil", err)
	}
}

func TestFileStoreRequiresTargetRoot(t *testing.T) {
	store := FileStore{}
	req := validBeginRequest([]byte("hello"))

	if _, err := store.Begin(req); !errors.Is(err, protocol.ErrValidation) {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want protocol.ErrValidation", req, err)
	}
}

func TestFileStoreConcurrentBeginRejectsDifferentMetadata(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	first := validBeginRequest([]byte("hello"))
	second := validBeginRequest([]byte("hello"))
	second.Manifest.ID = "manifest-2"
	second.Manifest.Entries[1].Path = "docs/b.txt"

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, req := range []protocol.BeginSessionRequest{first, second} {
		wg.Add(1)
		go func(req protocol.BeginSessionRequest) {
			defer wg.Done()
			<-start
			_, err := store.Begin(req)
			errs <- err
		}(req)
	}
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	conflicts := 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrConflict):
			conflicts++
		default:
			t.Fatalf("FileStore.Begin(concurrent different metadata) error = %v, want nil or ErrConflict", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent Begin results successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}
}

func TestFileStoreBeginReplaySameMetadataReturnsResumeState(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("he")}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}

	replay, err := store.Begin(req)
	if err != nil {
		t.Fatalf("FileStore.Begin(replay same metadata) error = %v, want nil", err)
	}
	if replay.State != protocol.SessionStateValidated {
		t.Fatalf("FileStore.Begin(replay same metadata).State = %q, want %q", replay.State, protocol.SessionStateValidated)
	}
	if len(replay.ResumeFrom) != 1 || replay.ResumeFrom[0].Path != "docs/a.txt" || replay.ResumeFrom[0].CommittedSize != 2 || replay.ResumeFrom[0].Complete {
		t.Fatalf("FileStore.Begin(replay same metadata).ResumeFrom = %+v, want docs/a.txt committed size 2 incomplete", replay.ResumeFrom)
	}
}

func TestFileStoreBeginReplayDifferentMetadataConflicts(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	replay := req
	replay.Manifest.Entries[1].Digest = digest([]byte("other"))

	if _, err := store.Begin(replay); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Begin(replay different metadata) error = %v, want ErrConflict", err)
	}
}

func TestFileStoreConcurrentBeginLocksCanonicalTargetRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "target")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", root, err)
	}
	stores := []FileStore{
		{TargetRoot: root},
		{TargetRoot: filepath.Join(base, ".", "target")},
	}
	first := validBeginRequest([]byte("hello"))
	second := validBeginRequest([]byte("hello"))
	second.Manifest.ID = "manifest-2"
	second.Manifest.Entries[1].Path = "docs/b.txt"

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i, req := range []protocol.BeginSessionRequest{first, second} {
		wg.Add(1)
		go func(store FileStore, req protocol.BeginSessionRequest) {
			defer wg.Done()
			<-start
			_, err := store.Begin(req)
			errs <- err
		}(stores[i], req)
	}
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	conflicts := 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrConflict):
			conflicts++
		default:
			t.Fatalf("FileStore.Begin(canonical target concurrent different metadata) error = %v, want nil or ErrConflict", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("canonical target concurrent Begin results successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}
}

func TestFileStoreStatusReturnsResumeOffset(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello world"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello")}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}

	status, err := store.Status(req.SessionID)
	if err != nil {
		t.Fatalf("FileStore.Status(%q) error = %v, want nil", req.SessionID, err)
	}
	if len(status.Files) != 1 || status.Files[0].CommittedSize != 5 || status.Files[0].Complete {
		t.Errorf("FileStore.Status(%q).Files = %+v, want committed size 5 and incomplete", req.SessionID, status.Files)
	}
}

func TestFileStoreBeginRebuildsMissingRecordFromMeta(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	layout := transaction.NewLayout(control.ControlDir(root))
	if err := os.Remove(layout.RecordPath(req.SessionID)); err != nil {
		t.Fatalf("os.Remove(session record) error = %v, want nil", err)
	}

	begin, err := store.Begin(req)
	if err != nil {
		t.Fatalf("FileStore.Begin(rebuild record) error = %v, want nil", err)
	}
	if begin.State != protocol.SessionStateValidated {
		t.Fatalf("FileStore.Begin(rebuild record).State = %q, want validated", begin.State)
	}
	if _, err := transaction.ReadSessionRecord(layout.RecordPath(req.SessionID)); err != nil {
		t.Fatalf("transaction.ReadSessionRecord(rebuilt) error = %v, want nil", err)
	}
}

func TestFileStoreBeginRewritesMissingManifestFromMeta(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	manifestPath, err := control.Path(root, control.ArtifactManifest, req.SessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v, want nil", err)
	}
	if err := os.Remove(manifestPath); err != nil {
		t.Fatalf("os.Remove(manifest) error = %v, want nil", err)
	}

	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(rewrite manifest) error = %v, want nil", err)
	}
	if _, err := control.ReadFile[control.Manifest](manifestPath); err != nil {
		t.Fatalf("control.ReadFile(rewritten manifest) error = %v, want nil", err)
	}
}

func TestFileStoreBeginDoesNotRewritePublishedManifestDrift(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err != nil {
		t.Fatalf("FileStore.Commit(%+v) error = %v, want nil", commitReq, err)
	}
	manifestPath, err := control.Path(root, control.ArtifactManifest, req.SessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v, want nil", err)
	}
	manifest, err := control.ReadFile[control.Manifest](manifestPath)
	if err != nil {
		t.Fatalf("control.ReadFile(manifest) error = %v, want nil", err)
	}
	manifest.Entries[1].Digest = digest([]byte("other"))
	if err := control.WriteFile(manifestPath, manifest); err != nil {
		t.Fatalf("control.WriteFile(manifest drift) error = %v, want nil", err)
	}

	if _, err := store.Begin(req); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Begin(published manifest drift) error = %v, want ErrConflict", err)
	}
	drifted, err := control.ReadFile[control.Manifest](manifestPath)
	if err != nil {
		t.Fatalf("control.ReadFile(drifted manifest) error = %v, want nil", err)
	}
	if drifted.Entries[1].Digest != digest([]byte("other")) {
		t.Fatalf("manifest digest after Begin = %q, want drift retained for audit", drifted.Entries[1].Digest)
	}
	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(root)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(%q) error = %v, want nil", req.SessionID, err)
	}
	if record.State != transaction.StateNeedsRepair {
		t.Fatalf("session state after Begin manifest drift = %q, want %q", record.State, transaction.StateNeedsRepair)
	}
}

func TestFileStoreBeginRejectsUnsafeTargetPath(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest([]byte("hello"))
	req.Manifest.Entries[1].TargetPath = "../escape.txt"

	if _, err := store.Begin(req); !errors.Is(err, protocol.ErrValidation) {
		t.Fatalf("FileStore.Begin(unsafe target path) error = %v, want protocol.ErrValidation", err)
	}
}

func TestFileStoreBeginRejectsReservedControlPlaneTargetPath(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	req.Manifest.Entries[1].TargetPath = ".supermover/sessions/fake/receipt.json"

	if _, err := store.Begin(req); !errors.Is(err, protocol.ErrValidation) {
		t.Fatalf("FileStore.Begin(reserved control path) error = %v, want protocol.ErrValidation", err)
	}
	if _, err := os.Stat(filepath.Join(root, control.DirName, "sessions", "fake", "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(forged control artifact) error = %v, want os.ErrNotExist", err)
	}
}

func TestFileStoreBeginRejectsUnsafeSymlinkTarget(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest(nil)
	req.Manifest.Entries = []protocol.ManifestEntry{
		{Path: "docs/link.txt", Kind: protocol.FileKindSymlink, SymlinkTarget: "../outside"},
	}

	if _, err := store.Begin(req); !errors.Is(err, protocol.ErrValidation) {
		t.Fatalf("FileStore.Begin(unsafe symlink target) error = %v, want protocol.ErrValidation", err)
	}
}

func TestFileStoreBeginRejectsManifestEntryLimit(t *testing.T) {
	req := validBeginRequest(nil)
	req.Manifest.Entries = make([]protocol.ManifestEntry, protocol.MaxManifestEntries+1)
	for i := range req.Manifest.Entries {
		req.Manifest.Entries[i] = protocol.ManifestEntry{Path: "f" + strconv.Itoa(i), Kind: protocol.FileKindFile, Size: 0, Digest: digest(nil)}
	}

	if err := req.Validate(); !errors.Is(err, protocol.ErrValidation) {
		t.Fatalf("BeginSessionRequest.Validate(too many entries) error = %v, want protocol.ErrValidation", err)
	}
}

func TestFileStoreAppendChunkRejectsGapAndMismatchReplay(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}

	gap := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 2, Data: []byte("ll")}
	if _, err := store.AppendChunk(gap); !errors.Is(err, ErrConflict) {
		t.Errorf("FileStore.AppendChunk(%+v) error = %v, want ErrConflict", gap, err)
	}

	first := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("he")}
	if _, err := store.AppendChunk(first); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", first, err)
	}
	badReplay := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("HE")}
	if _, err := store.AppendChunk(badReplay); !errors.Is(err, ErrConflict) {
		t.Errorf("FileStore.AppendChunk(%+v) error = %v, want ErrConflict", badReplay, err)
	}
}

func TestFileStoreAppendChunkClassifiesDuplicateAndRejectsOverlap(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest([]byte("hello world"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	first := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello")}
	if _, err := store.AppendChunk(first); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", first, err)
	}

	duplicate, err := store.AppendChunk(first)
	if err != nil {
		t.Fatalf("FileStore.AppendChunk(duplicate %+v) error = %v, want nil", first, err)
	}
	if duplicate.ChunkState != protocol.ChunkStateDuplicate || duplicate.CommittedSize != 5 {
		t.Fatalf("FileStore.AppendChunk(duplicate %+v) = %+v, want duplicate at size 5", first, duplicate)
	}

	differentReplay := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 1, Data: []byte("ELL")}
	if _, err := store.AppendChunk(differentReplay); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.AppendChunk(different replay %+v) error = %v, want ErrConflict", differentReplay, err)
	}
	partialOverlap := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 3, Data: []byte("lo w")}
	if _, err := store.AppendChunk(partialOverlap); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.AppendChunk(partial overlap %+v) error = %v, want ErrConflict", partialOverlap, err)
	}
}

func TestFileStoreAppendChunkRejectsStagedLeafSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	outsideFile := filepath.Join(outside, "a.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", outsideFile, err)
	}
	stagePath := filepath.Join(control.ControlDir(root), "sessions", req.SessionID, "stage", "docs", "a.txt")
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(stagePath), err)
	}
	if err := os.Symlink(outsideFile, stagePath); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); !errors.Is(err, protocol.ErrValidation) {
		t.Fatalf("FileStore.AppendChunk(staged leaf symlink) error = %v, want protocol.ErrValidation", err)
	}
	got, err := os.ReadFile(outsideFile)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", outsideFile, err)
	}
	if string(got) != "outside" {
		t.Fatalf("outside file after AppendChunk = %q, want outside", got)
	}
}

func TestFileStoreCommitRejectsIncompleteDigest(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hell")}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); !errors.Is(err, ErrIntegrity) {
		t.Errorf("FileStore.Commit(%+v) error = %v, want ErrIntegrity", commitReq, err)
	}

	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(store.TargetRoot)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(%q) error = %v, want nil", req.SessionID, err)
	}
	if record.State != transaction.StateNeedsRepair {
		t.Errorf("transaction.ReadSessionRecord(%q).State = %q, want %q", req.SessionID, record.State, transaction.StateNeedsRepair)
	}
}

func TestFileStoreCommitRejectsStagedDigestMismatch(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("HELLO"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("FileStore.Commit(%+v) error = %v, want ErrIntegrity", commitReq, err)
	}
	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(store.TargetRoot)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(%q) error = %v, want nil", req.SessionID, err)
	}
	if record.State != transaction.StateNeedsRepair {
		t.Fatalf("session state after digest mismatch = %q, want %q", record.State, transaction.StateNeedsRepair)
	}
	if _, err := os.Stat(filepath.Join(store.TargetRoot, "docs", "a.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(published file after digest mismatch) error = %v, want os.ErrNotExist", err)
	}
}

func TestFileStoreNeedsRepairRejectsNormalTraffic(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hell")}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("FileStore.Commit(%+v) error = %v, want ErrIntegrity", commitReq, err)
	}

	fixup := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 4, Data: []byte("o"), Final: true}
	if _, err := store.AppendChunk(fixup); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.AppendChunk(needs repair) error = %v, want ErrConflict", err)
	}
	if _, err := store.Commit(commitReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Commit(needs repair) error = %v, want ErrConflict", err)
	}
}

func TestFileStoreCommitRejectsSymlinkParentEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); !errors.Is(err, protocol.ErrValidation) {
		t.Fatalf("FileStore.Commit(symlink parent) error = %v, want protocol.ErrValidation", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "a.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(outside file) error = %v, want os.ErrNotExist", err)
	}
}

func TestFileStoreCommitPreflightsAllTargetsBeforePublish(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("a"))
	req.Manifest.Entries = []protocol.ManifestEntry{
		{Path: "a.txt", Kind: protocol.FileKindFile, Size: 1, Digest: digest([]byte("a"))},
		{Path: "z.txt", Kind: protocol.FileKindFile, Size: 1, Digest: digest([]byte("z"))},
	}
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	first := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "a.txt", Offset: 0, Data: []byte("a"), Final: true}
	if _, err := store.AppendChunk(first); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", first, err)
	}
	second := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "z.txt", Offset: 0, Data: []byte("z"), Final: true}
	if _, err := store.AppendChunk(second); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", second, err)
	}
	if err := os.WriteFile(filepath.Join(root, "z.txt"), []byte("different"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(existing z) error = %v, want nil", err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Commit(preflight target conflict) error = %v, want ErrConflict", err)
	}
	if _, err := os.Stat(filepath.Join(root, "a.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(a.txt after failed commit preflight) error = %v, want os.ErrNotExist", err)
	}
}

func TestFileStoreCommitRejectsManifestFileTargetWithDescendantBeforePublish(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest(nil)
	req.Manifest.Entries = []protocol.ManifestEntry{
		{Path: "src/first.txt", TargetPath: "a", Kind: protocol.FileKindFile, Size: 1, Digest: digest([]byte("a"))},
		{Path: "src/second.txt", TargetPath: "a/b", Kind: protocol.FileKindFile, Size: 1, Digest: digest([]byte("b"))},
	}
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	first := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "src/first.txt", Offset: 0, Data: []byte("a"), Final: true}
	if _, err := store.AppendChunk(first); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", first, err)
	}
	second := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "src/second.txt", Offset: 0, Data: []byte("b"), Final: true}
	if _, err := store.AppendChunk(second); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", second, err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Commit(file target with descendant) error = %v, want ErrConflict", err)
	}
	if _, err := os.Stat(filepath.Join(root, "a")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(a after rejected target plan) error = %v, want os.ErrNotExist", err)
	}
}

func TestFileStorePublishedSessionRequiresReceipt(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err != nil {
		t.Fatalf("FileStore.Commit(%+v) error = %v, want nil", commitReq, err)
	}
	receiptPath, err := control.Path(root, control.ArtifactSessionReceipt, req.SessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	if err := os.Remove(receiptPath); err != nil {
		t.Fatalf("os.Remove(%q) error = %v, want nil", receiptPath, err)
	}

	if _, err := store.Commit(commitReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Commit(missing receipt) error = %v, want ErrConflict", err)
	}
	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(root)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(%q) error = %v, want nil", req.SessionID, err)
	}
	if record.State != transaction.StateNeedsRepair {
		t.Fatalf("session state after missing receipt = %q, want %q", record.State, transaction.StateNeedsRepair)
	}
}

func TestFileStoreCommitRecoversAfterPublishBeforeReceipt(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	meta, _, err := store.loadSession(req.SessionID)
	if err != nil {
		t.Fatalf("FileStore.loadSession(%q) error = %v, want nil", req.SessionID, err)
	}
	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(root)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("ReadSessionRecord(%q) error = %v, want nil", req.SessionID, err)
	}
	staged, err := record.WithState(transaction.StateStaged, time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SessionRecord.WithState(staged) error = %v, want nil", err)
	}
	if err := store.layout().WriteSessionRecord(staged); err != nil {
		t.Fatalf("Layout.WriteSessionRecord(staged) error = %v, want nil", err)
	}
	if err := store.publish(meta); err != nil {
		t.Fatalf("FileStore.publish(%q) error = %v, want nil", req.SessionID, err)
	}
	if _, err := os.Lstat(filepath.Join(control.ControlDir(root), "sessions", req.SessionID, "stage", "docs", "a.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Lstat(staged file after simulated publish) error = %v, want os.ErrNotExist", err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 2, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err != nil {
		t.Fatalf("FileStore.Commit(recover after publish before receipt) error = %v, want nil", err)
	}
	final, err := os.ReadFile(filepath.Join(root, "docs", "a.txt"))
	if err != nil || string(final) != "hello" {
		t.Fatalf("published file after recovered commit = (%q, %v), want hello", string(final), err)
	}
	finalRecord, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(root)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("ReadSessionRecord(final) error = %v, want nil", err)
	}
	if finalRecord.State != transaction.StatePublished {
		t.Fatalf("session state after recovered commit = %q, want published", finalRecord.State)
	}
}

func TestFileStoreCommitMarksStagedReconcileFailureNeedsRepair(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(root)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("ReadSessionRecord(%q) error = %v, want nil", req.SessionID, err)
	}
	staged, err := record.WithState(transaction.StateStaged, time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SessionRecord.WithState(staged) error = %v, want nil", err)
	}
	if err := store.layout().WriteSessionRecord(staged); err != nil {
		t.Fatalf("Layout.WriteSessionRecord(staged) error = %v, want nil", err)
	}
	stagePath := filepath.Join(control.ControlDir(root), "sessions", req.SessionID, "stage", "docs", "a.txt")
	if err := os.WriteFile(stagePath, []byte("corrupt"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", stagePath, err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 2, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("FileStore.Commit(corrupt staged retry) error = %v, want ErrIntegrity", err)
	}
	finalRecord, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(root)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("ReadSessionRecord(final) error = %v, want nil", err)
	}
	if finalRecord.State != transaction.StateNeedsRepair {
		t.Fatalf("session state after corrupt staged retry = %q, want %q", finalRecord.State, transaction.StateNeedsRepair)
	}
}

func TestFileStoreCommitMarksReceiptWriteFailureNeedsRepair(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	receiptPath, err := control.Path(root, control.ArtifactSessionReceipt, req.SessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	if err := os.MkdirAll(receiptPath, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", receiptPath, err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 2, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err == nil {
		t.Fatalf("FileStore.Commit(receipt path directory) error = nil, want receipt write failure")
	}
	finalRecord, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(root)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("ReadSessionRecord(final) error = %v, want nil", err)
	}
	if finalRecord.State != transaction.StateNeedsRepair {
		t.Fatalf("session state after receipt write failure = %q, want %q", finalRecord.State, transaction.StateNeedsRepair)
	}
	if !strings.Contains(finalRecord.Note, "receipt") {
		t.Fatalf("session note after receipt write failure = %q, want receipt cause", finalRecord.Note)
	}
}

func TestFileStoreStatusRejectsUnknownMetadataField(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	metaPath := filepath.Join(control.ControlDir(root), "sessions", req.SessionID, "network-session.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", metaPath, err)
	}
	next := strings.Replace(string(data), "{", `{"unexpected":true,`, 1)
	if err := os.WriteFile(metaPath, []byte(next), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", metaPath, err)
	}

	if _, err := store.Status(req.SessionID); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("FileStore.Status(unknown metadata field) error = %v, want unknown field", err)
	}
}

func TestFileStorePublishedSessionRequiresReceiptScope(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err != nil {
		t.Fatalf("FileStore.Commit(%+v) error = %v, want nil", commitReq, err)
	}
	receiptPath, err := control.Path(root, control.ArtifactSessionReceipt, req.SessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		t.Fatalf("control.ReadFile(receipt) error = %v, want nil", err)
	}
	receipt.ProfileID = "profile.other"
	if err := control.WriteFile(receiptPath, receipt); err != nil {
		t.Fatalf("control.WriteFile(receipt scope drift) error = %v, want nil", err)
	}

	if _, err := store.Commit(commitReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Commit(receipt scope drift) error = %v, want ErrConflict", err)
	}
	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(root)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(%q) error = %v, want nil", req.SessionID, err)
	}
	if record.State != transaction.StateNeedsRepair {
		t.Fatalf("session state after receipt scope drift = %q, want %q", record.State, transaction.StateNeedsRepair)
	}
}

func TestFileStorePublishedSessionRequiresReceiptStartedAt(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err != nil {
		t.Fatalf("FileStore.Commit(%+v) error = %v, want nil", commitReq, err)
	}
	receiptPath, err := control.Path(root, control.ArtifactSessionReceipt, req.SessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		t.Fatalf("control.ReadFile(receipt) error = %v, want nil", err)
	}
	receipt.StartedAt = "2026-05-16T00:00:00Z"
	if err := control.WriteFile(receiptPath, receipt); err != nil {
		t.Fatalf("control.WriteFile(receipt time drift) error = %v, want nil", err)
	}

	if _, err := store.Commit(commitReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Commit(receipt time drift) error = %v, want ErrConflict", err)
	}
}

func TestFileStorePublishedSessionRequiresManifestEntriesMatch(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err != nil {
		t.Fatalf("FileStore.Commit(%+v) error = %v, want nil", commitReq, err)
	}
	manifestPath, err := control.Path(root, control.ArtifactManifest, req.SessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v, want nil", err)
	}
	manifest, err := control.ReadFile[control.Manifest](manifestPath)
	if err != nil {
		t.Fatalf("control.ReadFile(manifest) error = %v, want nil", err)
	}
	manifest.Entries[1].Digest = digest([]byte("other"))
	if err := control.WriteFile(manifestPath, manifest); err != nil {
		t.Fatalf("control.WriteFile(manifest drift) error = %v, want nil", err)
	}

	if _, err := store.Commit(commitReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Commit(manifest drift) error = %v, want ErrConflict", err)
	}
	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(root)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(%q) error = %v, want nil", req.SessionID, err)
	}
	if record.State != transaction.StateNeedsRepair {
		t.Fatalf("session state after manifest drift = %q, want %q", record.State, transaction.StateNeedsRepair)
	}
}

func TestFileStorePublishedSessionReplayIsIdempotentAndRejectsChunks(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err != nil {
		t.Fatalf("FileStore.Commit(%+v) error = %v, want nil", commitReq, err)
	}

	beginReplay, err := store.Begin(req)
	if err != nil {
		t.Fatalf("FileStore.Begin(published replay) error = %v, want nil", err)
	}
	if beginReplay.State != protocol.SessionStatePublished {
		t.Fatalf("FileStore.Begin(published replay).State = %q, want %q", beginReplay.State, protocol.SessionStatePublished)
	}
	commitReplay, err := store.Commit(commitReq)
	if err != nil {
		t.Fatalf("FileStore.Commit(published replay) error = %v, want nil", err)
	}
	if commitReplay.State != protocol.SessionStatePublished || commitReplay.ReceiptID != req.SessionID {
		t.Fatalf("FileStore.Commit(published replay) = %+v, want published receipt %q", commitReplay, req.SessionID)
	}
	if _, err := store.AppendChunk(chunk); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.AppendChunk(published session) error = %v, want ErrConflict", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "a.txt"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("published file after replay = (%q, %v), want hello", string(got), err)
	}
}

func TestFileStorePublishedSessionRequiresManifestCreatedAt(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}
	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err != nil {
		t.Fatalf("FileStore.Commit(%+v) error = %v, want nil", commitReq, err)
	}
	manifestPath, err := control.Path(root, control.ArtifactManifest, req.SessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v, want nil", err)
	}
	manifest, err := control.ReadFile[control.Manifest](manifestPath)
	if err != nil {
		t.Fatalf("control.ReadFile(manifest) error = %v, want nil", err)
	}
	manifest.CreatedAt = "2026-05-16T00:00:00Z"
	if err := control.WriteFile(manifestPath, manifest); err != nil {
		t.Fatalf("control.WriteFile(manifest time drift) error = %v, want nil", err)
	}

	if _, err := store.Commit(commitReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Commit(manifest time drift) error = %v, want ErrConflict", err)
	}
}

func TestFileStoreBeginRejectsEntriesBelowSymlinkPath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest(nil)
	req.Manifest.Entries = []protocol.ManifestEntry{
		{Path: "linkdir", Kind: protocol.FileKindSymlink, SymlinkTarget: outside},
		{Path: "linkdir/pwn", Kind: protocol.FileKindSymlink, SymlinkTarget: "victim"},
	}
	if _, err := store.Begin(req); !errors.Is(err, protocol.ErrValidation) {
		t.Fatalf("FileStore.Begin(entries below symlink path) error = %v, want protocol.ErrValidation", err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "pwn")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Lstat(outside pwn) error = %v, want os.ErrNotExist", err)
	}
}

func TestFileStoreBeginRejectsFileBelowSymlinkWithTargetPath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest(nil)
	req.Manifest.Entries = []protocol.ManifestEntry{
		{Path: "linkdir/pwn", Kind: protocol.FileKindFile, Size: 0, Digest: digest(nil)},
		{Path: "linkdir", TargetPath: "published-link", Kind: protocol.FileKindSymlink, SymlinkTarget: outside},
	}
	if _, err := store.Begin(req); !errors.Is(err, protocol.ErrValidation) {
		t.Fatalf("FileStore.Begin(file below symlink with target path) error = %v, want protocol.ErrValidation", err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "pwn")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Lstat(outside pwn) error = %v, want os.ErrNotExist", err)
	}
}

func TestFileStoreManifestPreservesSymlinkTarget(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	req.Manifest.Entries = append(req.Manifest.Entries, protocol.ManifestEntry{
		Path:          "docs/link.txt",
		Kind:          protocol.FileKindSymlink,
		SymlinkTarget: "a.txt",
	})
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}

	manifest := readControlDoc[control.Manifest](t, root, control.ArtifactManifest, req.SessionID)
	for _, entry := range manifest.Entries {
		if entry.Path == "docs/link.txt" {
			if entry.SymlinkTarget != "a.txt" {
				t.Fatalf("manifest symlink target = %q, want a.txt", entry.SymlinkTarget)
			}
			return
		}
	}
	t.Fatalf("manifest entries = %#v, want docs/link.txt", manifest.Entries)
}

func TestFileStoreCommitRefusesDivergentExistingTargetFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(docs) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "a.txt"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(existing target) error = %v, want nil", err)
	}
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Commit(existing divergent target) error = %v, want ErrConflict", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "a.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile(existing target) error = %v, want nil", err)
	}
	if string(got) != "existing" {
		t.Fatalf("target file after failed Commit = %q, want existing", got)
	}
}

func TestFileStoreCommitAllowsIdenticalExistingTargetFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(docs) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(existing target) error = %v, want nil", err)
	}
	existingTime := time.Date(2026, 5, 15, 8, 1, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(root, "docs", "a.txt"), existingTime, existingTime); err != nil {
		t.Fatalf("os.Chtimes(existing target) error = %v, want nil", err)
	}
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	req.Manifest.Entries[0].Mode = 0o700
	req.Manifest.Entries[0].ModTime = time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err != nil {
		t.Fatalf("FileStore.Commit(existing identical target) error = %v, want nil", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "a.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile(existing target) error = %v, want nil", err)
	}
	if string(got) != "hello" {
		t.Fatalf("target file after Commit = %q, want hello", got)
	}
	info, err := os.Stat(filepath.Join(root, "docs", "a.txt"))
	if err != nil {
		t.Fatalf("os.Stat(existing target) error = %v, want nil", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("target mode after idempotent Commit = %v, want existing 0600", info.Mode().Perm())
	}
	if !info.ModTime().Equal(existingTime) {
		t.Fatalf("target mtime after idempotent Commit = %v, want existing %v", info.ModTime(), existingTime)
	}
}

func TestFileStoreCommitRefusesDivergentExistingTargetSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(docs) error = %v, want nil", err)
	}
	if err := os.Symlink("other.txt", filepath.Join(root, "docs", "link.txt")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	req.Manifest.Entries = []protocol.ManifestEntry{
		{Path: "docs", Kind: protocol.FileKindDir},
		{Path: "docs/link.txt", Kind: protocol.FileKindSymlink, SymlinkTarget: "a.txt"},
	}
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); !errors.Is(err, ErrConflict) {
		t.Fatalf("FileStore.Commit(existing divergent symlink) error = %v, want ErrConflict", err)
	}
	got, err := os.Readlink(filepath.Join(root, "docs", "link.txt"))
	if err != nil {
		t.Fatalf("os.Readlink(existing symlink) error = %v, want nil", err)
	}
	if got != "other.txt" {
		t.Fatalf("target symlink after failed Commit = %q, want other.txt", got)
	}
}

func TestFileStoreCommitPublishesNewSymlinkWithoutStagedPlaceholder(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	req.Manifest.Entries = []protocol.ManifestEntry{
		{Path: "docs", Kind: protocol.FileKindDir},
		{Path: "docs/link.txt", Kind: protocol.FileKindSymlink, SymlinkTarget: "a.txt"},
	}
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err != nil {
		t.Fatalf("FileStore.Commit(new symlink) error = %v, want nil", err)
	}
	got, err := os.Readlink(filepath.Join(root, "docs", "link.txt"))
	if err != nil {
		t.Fatalf("os.Readlink(new symlink) error = %v, want nil", err)
	}
	if got != "a.txt" {
		t.Fatalf("target symlink after Commit = %q, want a.txt", got)
	}
	stagePath := filepath.Join(control.ControlDir(root), "sessions", req.SessionID, "stage", "docs", "link.txt")
	if _, err := os.Lstat(stagePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Lstat(%q) error = %v, want os.ErrNotExist", stagePath, err)
	}
}

func TestFileStoreCommitAllowsIdenticalExistingTargetSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(docs) error = %v, want nil", err)
	}
	if err := os.Symlink("a.txt", filepath.Join(root, "docs", "link.txt")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	req.Manifest.Entries = []protocol.ManifestEntry{
		{Path: "docs", Kind: protocol.FileKindDir},
		{Path: "docs/link.txt", Kind: protocol.FileKindSymlink, SymlinkTarget: "a.txt"},
	}
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}

	commitReq := protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)}
	if _, err := store.Commit(commitReq); err != nil {
		t.Fatalf("FileStore.Commit(existing identical symlink) error = %v, want nil", err)
	}
	got, err := os.Readlink(filepath.Join(root, "docs", "link.txt"))
	if err != nil {
		t.Fatalf("os.Readlink(existing symlink) error = %v, want nil", err)
	}
	if got != "a.txt" {
		t.Fatalf("target symlink after Commit = %q, want a.txt", got)
	}
}

func TestFileStoreManifestPreservesEntryModTimeNanos(t *testing.T) {
	root := t.TempDir()
	store := FileStore{TargetRoot: root}
	req := validBeginRequest([]byte("hello"))
	req.Manifest.Entries[1].ModTime = time.Date(2026, 5, 16, 8, 1, 2, 123456789, time.UTC)
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	manifest := readControlDoc[control.Manifest](t, root, control.ArtifactManifest, req.SessionID)
	if got := manifest.Entries[1].ModTime; got != "2026-05-16T08:01:02.123456789Z" {
		t.Fatalf("manifest file mod_time = %q, want RFC3339Nano", got)
	}
}

func TestFileStoreConcurrentDuplicateChunkSerialized(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}

	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("he")}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	responses := make(chan protocol.ChunkUploadResponse, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := store.AppendChunk(chunk)
			if err != nil {
				errs <- err
				return
			}
			responses <- resp
		}()
	}
	wg.Wait()
	close(errs)
	close(responses)

	for err := range errs {
		t.Errorf("FileStore.AppendChunk(concurrent duplicate) error = %v, want nil", err)
	}
	states := map[protocol.ChunkState]int{}
	for resp := range responses {
		if resp.CommittedSize != 2 {
			t.Errorf("FileStore.AppendChunk(concurrent duplicate).CommittedSize = %d, want 2", resp.CommittedSize)
		}
		states[resp.ChunkState]++
	}
	if states[protocol.ChunkStateAccepted] != 1 || states[protocol.ChunkStateDuplicate] != 1 {
		t.Fatalf("concurrent duplicate chunk states = %#v, want one accepted and one duplicate", states)
	}
}

func TestFileStoreConcurrentCommitIsIdempotent(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest([]byte("hello"))
	if _, err := store.Begin(req); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", req, err)
	}
	chunk := protocol.ChunkUploadRequest{SessionID: req.SessionID, Path: "docs/a.txt", Offset: 0, Data: []byte("hello"), Final: true}
	if _, err := store.AppendChunk(chunk); err != nil {
		t.Fatalf("FileStore.AppendChunk(%+v) error = %v, want nil", chunk, err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := store.Commit(protocol.CommitSessionRequest{SessionID: req.SessionID, EndedAt: time.Date(2026, 5, 16, 8, 1, 0, 0, time.UTC)})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("FileStore.Commit(concurrent idempotent) error = %v, want nil", err)
		}
	}
	record, err := transaction.ReadSessionRecord(transaction.NewLayout(control.ControlDir(store.TargetRoot)).RecordPath(req.SessionID))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(%q) error = %v, want nil", req.SessionID, err)
	}
	if record.State != transaction.StatePublished {
		t.Fatalf("session state after concurrent Commit = %q, want %q", record.State, transaction.StatePublished)
	}
}

func validBeginRequest(data []byte) protocol.BeginSessionRequest {
	now := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)
	return protocol.BeginSessionRequest{
		ProtocolVersion: protocol.Version,
		SessionID:       "session-1",
		ProfileID:       "profile.default",
		SourceDeviceID:  "sha256:abcdef0123456789",
		TargetDeviceID:  "sha256:0123456789abcdef",
		RootID:          "root1",
		CreatedAt:       now,
		Manifest: protocol.TransferManifest{
			ID: "manifest-1",
			Entries: []protocol.ManifestEntry{
				{Path: "docs", Kind: protocol.FileKindDir},
				{Path: "docs/a.txt", Kind: protocol.FileKindFile, Mode: 0o600, Size: int64(len(data)), Digest: digest(data), ModTime: now},
			},
		},
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
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
