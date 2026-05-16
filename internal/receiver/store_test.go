package receiver

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
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
}

func TestFileStoreRequiresTargetRoot(t *testing.T) {
	store := FileStore{}
	req := validBeginRequest([]byte("hello"))

	if _, err := store.Begin(req); !errors.Is(err, protocol.ErrValidation) {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want protocol.ErrValidation", req, err)
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

func TestFileStoreBeginRejectsUnsafeTargetPath(t *testing.T) {
	store := FileStore{TargetRoot: t.TempDir()}
	req := validBeginRequest([]byte("hello"))
	req.Manifest.Entries[1].TargetPath = "../escape.txt"

	if _, err := store.Begin(req); !errors.Is(err, protocol.ErrValidation) {
		t.Fatalf("FileStore.Begin(unsafe target path) error = %v, want protocol.ErrValidation", err)
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
				{Path: "docs/a.txt", Kind: protocol.FileKindFile, Size: int64(len(data)), Digest: digest(data), ModTime: now},
			},
		},
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
