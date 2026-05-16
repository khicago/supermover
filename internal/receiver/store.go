package receiver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/durable"
	"github.com/khicago/supermover/internal/filelock"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/transaction"
)

var (
	ErrSessionNotFound = errors.New("receiver session not found")
	ErrConflict        = errors.New("receiver conflict")
	ErrIntegrity       = errors.New("receiver integrity failure")
)

type Clock func() time.Time

type Store interface {
	Begin(req protocol.BeginSessionRequest) (protocol.BeginSessionResponse, error)
	Status(sessionID string) (protocol.SessionStatusResponse, error)
	AppendChunk(req protocol.ChunkUploadRequest) (protocol.ChunkUploadResponse, error)
	Commit(req protocol.CommitSessionRequest) (protocol.CommitSessionResponse, error)
}

type FileStore struct {
	TargetRoot string
	Now        Clock
}

var receiverLocks sync.Map

type sessionMeta struct {
	ProtocolVersion string                    `json:"protocol_version"`
	SessionID       string                    `json:"session_id"`
	ProfileID       string                    `json:"profile_id"`
	SourceDeviceID  string                    `json:"source_device_id"`
	TargetDeviceID  string                    `json:"target_device_id"`
	RootID          string                    `json:"root_id,omitempty"`
	CreatedAt       time.Time                 `json:"created_at"`
	Manifest        protocol.TransferManifest `json:"manifest"`
}

func (s FileStore) Begin(req protocol.BeginSessionRequest) (protocol.BeginSessionResponse, error) {
	if err := req.Validate(); err != nil {
		return protocol.BeginSessionResponse{}, err
	}
	if err := s.validate(); err != nil {
		return protocol.BeginSessionResponse{}, err
	}
	unlock, err := s.lockSession(req.SessionID)
	if err != nil {
		return protocol.BeginSessionResponse{}, err
	}
	defer unlock()

	layout := s.layout()
	if err := layout.EnsureSessionDirs(req.SessionID); err != nil {
		return protocol.BeginSessionResponse{}, err
	}

	if existing, err := readMeta(s.metaPath(req.SessionID)); err == nil {
		if !sameManifest(existing, req) {
			return protocol.BeginSessionResponse{}, fmt.Errorf("%w: session %q already exists with different metadata", ErrConflict, req.SessionID)
		}
		if err := s.reconcileBegin(existing); err != nil {
			return protocol.BeginSessionResponse{}, err
		}
		status, err := s.statusLocked(req.SessionID)
		if err != nil {
			return protocol.BeginSessionResponse{}, err
		}
		return protocol.BeginSessionResponse{SessionID: req.SessionID, State: status.State, ResumeFrom: status.Files}, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return protocol.BeginSessionResponse{}, err
	}

	record, err := transaction.NewSessionRecord(req.SessionID, req.CreatedAt)
	if err != nil {
		return protocol.BeginSessionResponse{}, err
	}
	record, err = record.WithState(transaction.StateValidated, s.now())
	if err != nil {
		return protocol.BeginSessionResponse{}, err
	}
	meta := sessionMeta{
		ProtocolVersion: req.ProtocolVersion,
		SessionID:       req.SessionID,
		ProfileID:       req.ProfileID,
		SourceDeviceID:  req.SourceDeviceID,
		TargetDeviceID:  req.TargetDeviceID,
		RootID:          req.RootID,
		CreatedAt:       req.CreatedAt.UTC(),
		Manifest:        req.Manifest,
	}
	if err := writeJSONAtomic(s.metaPath(req.SessionID), meta); err != nil {
		return protocol.BeginSessionResponse{}, err
	}
	if err := s.writeManifestArtifact(req); err != nil {
		return protocol.BeginSessionResponse{}, err
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		return protocol.BeginSessionResponse{}, err
	}
	return protocol.BeginSessionResponse{SessionID: req.SessionID, State: protocol.SessionStateValidated}, nil
}

func (s FileStore) Status(sessionID string) (protocol.SessionStatusResponse, error) {
	if err := s.validate(); err != nil {
		return protocol.SessionStatusResponse{}, err
	}
	if err := transaction.ValidateSessionID(sessionID); err != nil {
		return protocol.SessionStatusResponse{}, err
	}
	unlock, err := s.lockSession(sessionID)
	if err != nil {
		return protocol.SessionStatusResponse{}, err
	}
	defer unlock()

	return s.statusLocked(sessionID)
}

func (s FileStore) statusLocked(sessionID string) (protocol.SessionStatusResponse, error) {
	meta, record, err := s.loadSession(sessionID)
	if err != nil {
		return protocol.SessionStatusResponse{}, err
	}
	files, err := s.fileStatuses(meta)
	if err != nil {
		return protocol.SessionStatusResponse{}, err
	}
	return protocol.SessionStatusResponse{
		SessionID: sessionID,
		State:     fromTransactionState(record.State),
		Files:     files,
	}, nil
}

func (s FileStore) AppendChunk(req protocol.ChunkUploadRequest) (protocol.ChunkUploadResponse, error) {
	if err := req.Validate(); err != nil {
		return protocol.ChunkUploadResponse{}, err
	}
	if err := s.validate(); err != nil {
		return protocol.ChunkUploadResponse{}, err
	}
	if req.Digest != "" && req.Digest != sha256Digest(req.Data) {
		return protocol.ChunkUploadResponse{}, fmt.Errorf("%w: chunk digest mismatch for %q", ErrIntegrity, req.Path)
	}

	unlock, err := s.lockSession(req.SessionID)
	if err != nil {
		return protocol.ChunkUploadResponse{}, err
	}
	defer unlock()

	meta, record, err := s.loadSession(req.SessionID)
	if err != nil {
		return protocol.ChunkUploadResponse{}, err
	}
	if record.State == transaction.StatePublished || record.State == transaction.StateRolledBack || record.State == transaction.StateNeedsRepair {
		return protocol.ChunkUploadResponse{}, fmt.Errorf("%w: session %q is terminal", ErrConflict, req.SessionID)
	}

	entry, ok := findEntry(meta.Manifest, req.Path)
	if !ok {
		return protocol.ChunkUploadResponse{}, fmt.Errorf("%w: manifest has no file %q", ErrConflict, req.Path)
	}
	if entry.Kind != protocol.FileKindFile {
		return protocol.ChunkUploadResponse{}, fmt.Errorf("%w: %q is not a file", ErrConflict, req.Path)
	}
	if req.Offset+int64(len(req.Data)) > entry.Size {
		return protocol.ChunkUploadResponse{}, fmt.Errorf("%w: chunk exceeds declared size for %q", ErrConflict, req.Path)
	}

	path, err := s.stageFilePath(req.SessionID, req.Path)
	if err != nil {
		return protocol.ChunkUploadResponse{}, err
	}
	size, err := fileSize(path)
	if err != nil {
		return protocol.ChunkUploadResponse{}, err
	}
	chunkState := protocol.ChunkStateAccepted
	switch {
	case req.Offset == size:
		if err := appendAt(s.layout().StagingDir(req.SessionID), path, req.Data); err != nil {
			return protocol.ChunkUploadResponse{}, err
		}
		size += int64(len(req.Data))
	case req.Offset+int64(len(req.Data)) <= size:
		matches, err := chunkMatches(path, req.Offset, req.Data)
		if err != nil {
			return protocol.ChunkUploadResponse{}, err
		}
		if !matches {
			return protocol.ChunkUploadResponse{}, fmt.Errorf("%w: replayed chunk differs at %q offset %d", ErrConflict, req.Path, req.Offset)
		}
		chunkState = protocol.ChunkStateDuplicate
	default:
		return protocol.ChunkUploadResponse{}, fmt.Errorf("%w: expected offset %d for %q", ErrConflict, size, req.Path)
	}

	complete := size == entry.Size
	if req.Final && !complete {
		return protocol.ChunkUploadResponse{}, fmt.Errorf("%w: final chunk leaves %q incomplete", ErrConflict, req.Path)
	}
	return protocol.ChunkUploadResponse{
		SessionID:     req.SessionID,
		Path:          req.Path,
		CommittedSize: size,
		ChunkState:    chunkState,
		Complete:      complete,
	}, nil
}

func (s FileStore) Commit(req protocol.CommitSessionRequest) (protocol.CommitSessionResponse, error) {
	if err := req.Validate(); err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	if err := s.validate(); err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	unlock, err := s.lockSession(req.SessionID)
	if err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	defer unlock()

	meta, record, err := s.loadSession(req.SessionID)
	if err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	if record.State == transaction.StatePublished {
		if err := s.ensurePublishedArtifacts(meta); err != nil {
			s.markNeedsRepair(record)
			return protocol.CommitSessionResponse{}, err
		}
		return protocol.CommitSessionResponse{
			SessionID: req.SessionID,
			State:     protocol.SessionStatePublished,
			ReceiptID: req.SessionID,
		}, nil
	}
	if record.State == transaction.StateRolledBack {
		return protocol.CommitSessionResponse{}, fmt.Errorf("%w: session %q is rolled back", ErrConflict, req.SessionID)
	}
	if record.State == transaction.StateNeedsRepair {
		return protocol.CommitSessionResponse{}, fmt.Errorf("%w: session %q needs repair", ErrConflict, req.SessionID)
	}
	staged := record
	if record.State != transaction.StateStaged {
		if err := s.stageNonRegularEntries(meta); err != nil {
			return protocol.CommitSessionResponse{}, err
		}
		if err := s.verifyFiles(meta); err != nil {
			s.markNeedsRepair(record)
			return protocol.CommitSessionResponse{}, err
		}
		var err error
		staged, err = record.WithState(transaction.StateStaged, s.now())
		if err != nil {
			return protocol.CommitSessionResponse{}, err
		}
		if err := s.layout().WriteSessionRecord(staged); err != nil {
			return protocol.CommitSessionResponse{}, err
		}
	} else if err := s.reconcileStagedFiles(meta); err != nil {
		s.markNeedsRepair(staged)
		return protocol.CommitSessionResponse{}, err
	}
	if err := s.publish(meta); err != nil {
		s.markNeedsRepair(staged)
		return protocol.CommitSessionResponse{}, err
	}
	if err := s.writeReceipt(meta, req.EndedAt); err != nil {
		s.markNeedsRepair(staged)
		return protocol.CommitSessionResponse{}, err
	}
	published, err := staged.WithState(transaction.StatePublished, s.now())
	if err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	if err := s.layout().WriteSessionRecord(published); err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	return protocol.CommitSessionResponse{
		SessionID: req.SessionID,
		State:     protocol.SessionStatePublished,
		ReceiptID: req.SessionID,
	}, nil
}

func (s FileStore) markNeedsRepair(record transaction.SessionRecord) {
	repair, err := record.WithState(transaction.StateNeedsRepair, s.now())
	if err != nil {
		return
	}
	_ = s.layout().WriteSessionRecord(repair)
}

func (s FileStore) layout() transaction.Layout {
	return transaction.NewLayout(control.ControlDir(s.TargetRoot))
}

func (s FileStore) validate() error {
	if strings.TrimSpace(s.TargetRoot) == "" {
		return fmt.Errorf("%w: target root is required", protocol.ErrValidation)
	}
	return nil
}

func (s FileStore) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s FileStore) lockSession(sessionID string) (func(), error) {
	target, err := pathguard.CanonicalPath(s.TargetRoot)
	if err != nil {
		return nil, err
	}
	locksDir := filepath.Join(control.ControlDir(s.TargetRoot), "locks")
	if err := pathguard.EnsurePlainDirectory(s.TargetRoot, locksDir, 0o700); err != nil {
		return nil, err
	}

	targetValue, _ := receiverLocks.LoadOrStore(target+"\x00target", &sync.Mutex{})
	targetMu := targetValue.(*sync.Mutex)
	targetMu.Lock()
	unlockTargetFile, err := filelock.LockInDir(locksDir, "target.lock")
	if err != nil {
		targetMu.Unlock()
		return nil, err
	}

	sessionValue, _ := receiverLocks.LoadOrStore(target+"\x00session\x00"+sessionID, &sync.Mutex{})
	sessionMu := sessionValue.(*sync.Mutex)
	sessionMu.Lock()
	unlockSessionFile, err := filelock.LockInDir(locksDir, sessionID+".lock")
	if err != nil {
		sessionMu.Unlock()
		unlockTargetFile()
		targetMu.Unlock()
		return nil, err
	}
	return func() {
		unlockSessionFile()
		sessionMu.Unlock()
		unlockTargetFile()
		targetMu.Unlock()
	}, nil
}

func (s FileStore) metaPath(sessionID string) string {
	return filepath.Join(s.layout().SessionDir(sessionID), "network-session.json")
}

func (s FileStore) stageFilePath(sessionID, rel string) (string, error) {
	path, err := pathguard.SafeJoin(s.layout().StagingDir(sessionID), rel)
	if err != nil {
		return "", protocolPathError(err)
	}
	return path, nil
}

func (s FileStore) finalPath(entry protocol.ManifestEntry) (string, error) {
	target := entry.Path
	if entry.TargetPath != "" {
		target = entry.TargetPath
	}
	var (
		path string
		err  error
	)
	if entry.Kind == protocol.FileKindDir {
		path, err = pathguard.SafeJoinDirectory(s.TargetRoot, target)
	} else {
		path, err = pathguard.SafeJoinParent(s.TargetRoot, target)
	}
	if err != nil {
		return "", protocolPathError(err)
	}
	return path, nil
}

func (s FileStore) loadSession(sessionID string) (sessionMeta, transaction.SessionRecord, error) {
	meta, err := readMeta(s.metaPath(sessionID))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return sessionMeta{}, transaction.SessionRecord{}, ErrSessionNotFound
		}
		return sessionMeta{}, transaction.SessionRecord{}, err
	}
	record, err := transaction.ReadSessionRecord(s.layout().RecordPath(sessionID))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return sessionMeta{}, transaction.SessionRecord{}, ErrSessionNotFound
		}
		return sessionMeta{}, transaction.SessionRecord{}, err
	}
	return meta, record, nil
}

func (s FileStore) reconcileBegin(meta sessionMeta) error {
	record, err := transaction.ReadSessionRecord(s.layout().RecordPath(meta.SessionID))
	if err == nil {
		if record.ID != meta.SessionID {
			return fmt.Errorf("%w: session record id %q does not match metadata %q", ErrConflict, record.ID, meta.SessionID)
		}
		if record.State == transaction.StatePublished {
			if err := s.ensurePublishedArtifacts(meta); err != nil {
				s.markNeedsRepair(record)
				return err
			}
			return nil
		}
		req := beginRequestFromMeta(meta)
		if err := s.writeManifestArtifact(req); err != nil {
			return err
		}
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	published, err := s.hasPublishedReceipt(meta)
	if err != nil {
		return err
	}
	if published {
		if err := s.ensurePublishedArtifacts(meta); err != nil {
			return err
		}
		return s.rebuildSessionRecord(meta, transaction.StatePublished)
	}
	req := beginRequestFromMeta(meta)
	if err := s.writeManifestArtifact(req); err != nil {
		return err
	}
	return s.rebuildSessionRecord(meta, transaction.StateValidated)
}

func (s FileStore) rebuildSessionRecord(meta sessionMeta, state transaction.State) error {
	record, err := transaction.NewSessionRecord(meta.SessionID, meta.CreatedAt)
	if err != nil {
		return err
	}
	record, err = record.WithState(state, s.now())
	if err != nil {
		return err
	}
	return s.layout().WriteSessionRecord(record)
}

func beginRequestFromMeta(meta sessionMeta) protocol.BeginSessionRequest {
	return protocol.BeginSessionRequest{
		ProtocolVersion: meta.ProtocolVersion,
		SessionID:       meta.SessionID,
		ProfileID:       meta.ProfileID,
		SourceDeviceID:  meta.SourceDeviceID,
		TargetDeviceID:  meta.TargetDeviceID,
		RootID:          meta.RootID,
		CreatedAt:       meta.CreatedAt,
		Manifest:        meta.Manifest,
	}
}

func (s FileStore) fileStatuses(meta sessionMeta) ([]protocol.FileStatus, error) {
	files := make([]protocol.FileStatus, 0)
	for _, entry := range meta.Manifest.Entries {
		if entry.Kind != protocol.FileKindFile {
			continue
		}
		path, err := s.stageFilePath(meta.SessionID, entry.Path)
		if err != nil {
			return nil, err
		}
		size, err := fileSize(path)
		if err != nil {
			return nil, err
		}
		files = append(files, protocol.FileStatus{
			Path:           entry.Path,
			ExpectedSize:   entry.Size,
			CommittedSize:  size,
			ExpectedDigest: entry.Digest,
			Complete:       size == entry.Size,
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func (s FileStore) stageNonRegularEntries(meta sessionMeta) error {
	for _, entry := range meta.Manifest.Entries {
		path, err := s.stageFilePath(meta.SessionID, entry.Path)
		if err != nil {
			return err
		}
		switch entry.Kind {
		case protocol.FileKindDir:
			if err := makeDirectoryInside(s.layout().StagingDir(meta.SessionID), path, 0o755); err != nil {
				return fmt.Errorf("stage directory %q: %w", entry.Path, err)
			}
		}
	}
	return nil
}

func (s FileStore) verifyFiles(meta sessionMeta) error {
	for _, entry := range meta.Manifest.Entries {
		if entry.Kind != protocol.FileKindFile {
			continue
		}
		path, err := s.stageFilePath(meta.SessionID, entry.Path)
		if err != nil {
			return err
		}
		size, err := fileSize(path)
		if err != nil {
			return err
		}
		if size != entry.Size {
			return fmt.Errorf("%w: %q size = %d, want %d", ErrIntegrity, entry.Path, size, entry.Size)
		}
		got, err := fileDigest(path)
		if err != nil {
			return err
		}
		if got != entry.Digest {
			return fmt.Errorf("%w: %q digest = %s, want %s", ErrIntegrity, entry.Path, got, entry.Digest)
		}
	}
	return nil
}

func (s FileStore) reconcileStagedFiles(meta sessionMeta) error {
	for _, entry := range meta.Manifest.Entries {
		switch entry.Kind {
		case protocol.FileKindFile:
			if err := s.reconcileStagedFile(meta, entry); err != nil {
				return err
			}
		case protocol.FileKindDir:
			final, err := s.finalPath(entry)
			if err != nil {
				return err
			}
			if _, exists, err := finalDirectoryState(final); err != nil || exists {
				if err != nil {
					return err
				}
				continue
			}
			stage, err := s.stageFilePath(meta.SessionID, entry.Path)
			if err != nil {
				return err
			}
			if _, err := os.Lstat(stage); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return err
			}
		case protocol.FileKindSymlink:
			final, err := s.finalPath(entry)
			if err != nil {
				return err
			}
			same, exists, err := symlinkTargetState(final, entry.SymlinkTarget)
			if err != nil {
				return err
			}
			if exists && !same {
				return fmt.Errorf("%w: target symlink %q already exists with different target; refusing to overwrite", ErrConflict, publishPath(entry))
			}
		}
	}
	return nil
}

func (s FileStore) reconcileStagedFile(meta sessionMeta, entry protocol.ManifestEntry) error {
	final, err := s.finalPath(entry)
	if err != nil {
		return err
	}
	same, exists, err := finalFileState(final, entry.Size, entry.Digest)
	if err != nil {
		return err
	}
	if exists {
		if same {
			return nil
		}
		return fmt.Errorf("%w: target file %q already exists with different content; refusing to overwrite", ErrConflict, publishPath(entry))
	}
	stage, err := s.stageFilePath(meta.SessionID, entry.Path)
	if err != nil {
		return err
	}
	size, err := fileSize(stage)
	if err != nil {
		return err
	}
	if size != entry.Size {
		return fmt.Errorf("%w: %q size = %d, want %d", ErrIntegrity, entry.Path, size, entry.Size)
	}
	got, err := fileDigest(stage)
	if err != nil {
		return err
	}
	if got != entry.Digest {
		return fmt.Errorf("%w: %q digest = %s, want %s", ErrIntegrity, entry.Path, got, entry.Digest)
	}
	return nil
}

func (s FileStore) publish(meta sessionMeta) error {
	for _, entry := range meta.Manifest.Entries {
		final, err := s.finalPath(entry)
		if err != nil {
			return err
		}
		switch entry.Kind {
		case protocol.FileKindDir:
			if err := publishDirectory(s.TargetRoot, final, entry.Path); err != nil {
				return err
			}
		case protocol.FileKindSymlink:
			if err := publishSymlinkNoReplace(s.TargetRoot, final, entry); err != nil {
				return err
			}
		case protocol.FileKindFile:
			same, exists, err := finalFileState(final, entry.Size, entry.Digest)
			if err != nil {
				return err
			}
			stage, err := s.stageFilePath(meta.SessionID, entry.Path)
			if err != nil {
				return err
			}
			if exists {
				if same {
					if err := os.Remove(stage); err != nil && !errors.Is(err, fs.ErrNotExist) {
						return fmt.Errorf("remove duplicate staged file %q: %w", entry.Path, err)
					}
					continue
				}
				return fmt.Errorf("%w: target file %q already exists with different content; refusing to overwrite", ErrConflict, publishPath(entry))
			}
			if err := pathguard.EnsurePlainDirectory(s.TargetRoot, filepath.Dir(final), 0o755); err != nil {
				return fmt.Errorf("publish file parent %q: %w", entry.Path, err)
			}
			if err := applyReceiverFileMetadata(stage, entry); err != nil {
				return err
			}
			if err := durable.PromoteFileNoReplace(stage, final); err != nil {
				return fmt.Errorf("publish file %q: %w", entry.Path, err)
			}
		}
	}
	return nil
}

func applyReceiverFileMetadata(path string, entry protocol.ManifestEntry) error {
	mode := os.FileMode(entry.Mode).Perm()
	if mode != 0 {
		if err := os.Chmod(path, mode); err != nil {
			return fmt.Errorf("apply mode %q: %w", entry.Path, err)
		}
	}
	if !entry.ModTime.IsZero() {
		if err := os.Chtimes(path, entry.ModTime, entry.ModTime); err != nil {
			return fmt.Errorf("apply mtime %q: %w", entry.Path, err)
		}
	}
	return nil
}

func publishPath(entry protocol.ManifestEntry) string {
	if entry.TargetPath != "" {
		return entry.TargetPath
	}
	return entry.Path
}

func publishSymlinkNoReplace(root, final string, entry protocol.ManifestEntry) error {
	if err := pathguard.ValidateRelativeSymlinkTarget(entry.SymlinkTarget); err != nil {
		return fmt.Errorf("publish symlink %q: %w", entry.Path, err)
	}
	if err := pathguard.EnsurePlainDirectory(root, filepath.Dir(final), 0o755); err != nil {
		return fmt.Errorf("publish symlink parent %q: %w", entry.Path, err)
	}
	same, exists, err := symlinkTargetState(final, entry.SymlinkTarget)
	if err != nil {
		return err
	}
	if exists {
		if same {
			return nil
		}
		return fmt.Errorf("%w: target symlink %q already exists with different target; refusing to overwrite", ErrConflict, publishPath(entry))
	}
	if err := os.Symlink(entry.SymlinkTarget, final); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("%w: target symlink %q appeared before publish; refusing to overwrite", ErrConflict, publishPath(entry))
		}
		return fmt.Errorf("publish symlink %q: %w", entry.Path, err)
	}
	if err := durable.SyncDirBestEffort(filepath.Dir(final)); err != nil {
		return err
	}
	return nil
}

func publishDirectory(root, final, path string) error {
	info, err := os.Lstat(final)
	if err == nil {
		if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		return fmt.Errorf("%w: target directory %q already exists as non-directory; refusing to overwrite", ErrConflict, path)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat target directory %q: %w", path, err)
	}
	if err := pathguard.EnsurePlainDirectory(root, final, 0o755); err != nil {
		return fmt.Errorf("publish directory %q: %w", path, err)
	}
	return nil
}

func finalDirectoryState(path string) (same bool, exists bool, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("stat target directory %q: %w", path, err)
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return true, true, nil
	}
	return false, true, fmt.Errorf("%w: target directory %q already exists as non-directory; refusing to overwrite", ErrConflict, path)
}

func finalFileState(path string, size int64, digest string) (same bool, exists bool, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("stat target file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return false, true, fmt.Errorf("%w: target file %q already exists as non-regular; refusing to overwrite", ErrConflict, path)
	}
	if info.Size() != size {
		return false, true, nil
	}
	got, err := fileDigest(path)
	if err != nil {
		return false, true, err
	}
	return strings.EqualFold(got, digest), true, nil
}

func symlinkTargetState(path string, target string) (same bool, exists bool, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("stat target symlink %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, true, fmt.Errorf("%w: target symlink %q already exists as non-symlink; refusing to overwrite", ErrConflict, path)
	}
	got, err := os.Readlink(path)
	if err != nil {
		return false, true, fmt.Errorf("read target symlink %q: %w", path, err)
	}
	return got == target, true, nil
}

func (s FileStore) hasPublishedReceipt(meta sessionMeta) (bool, error) {
	receiptPath, err := control.Path(s.TargetRoot, control.ArtifactSessionReceipt, meta.SessionID)
	if err != nil {
		return false, err
	}
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return receipt.Status == string(protocol.SessionStatePublished), nil
}

func (s FileStore) ensurePublishedArtifacts(meta sessionMeta) error {
	manifestPath, err := control.Path(s.TargetRoot, control.ArtifactManifest, meta.SessionID)
	if err != nil {
		return err
	}
	manifest, err := control.ReadFile[control.Manifest](manifestPath)
	if err != nil {
		return fmt.Errorf("%w: published session %q is missing manifest: %v", ErrConflict, meta.SessionID, err)
	}
	if err := ensureManifestMatchesMeta(manifest, meta); err != nil {
		return err
	}
	receiptPath, err := control.Path(s.TargetRoot, control.ArtifactSessionReceipt, meta.SessionID)
	if err != nil {
		return err
	}
	receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
	if err != nil {
		return fmt.Errorf("%w: published session %q is missing receipt: %v", ErrConflict, meta.SessionID, err)
	}
	if receipt.ID != meta.SessionID {
		return fmt.Errorf("%w: published session %q receipt id = %q", ErrConflict, meta.SessionID, receipt.ID)
	}
	if receipt.Status != string(protocol.SessionStatePublished) {
		return fmt.Errorf("%w: published session %q receipt status = %q", ErrConflict, meta.SessionID, receipt.Status)
	}
	if receipt.ProfileID != meta.ProfileID || receipt.TargetID != meta.TargetDeviceID {
		return fmt.Errorf("%w: published session %q receipt scope = %q/%q, want %q/%q", ErrConflict, meta.SessionID, receipt.ProfileID, receipt.TargetID, meta.ProfileID, meta.TargetDeviceID)
	}
	if receipt.StartedAt != meta.CreatedAt.UTC().Format(time.RFC3339) {
		return fmt.Errorf("%w: published session %q receipt started_at = %q, want %q", ErrConflict, meta.SessionID, receipt.StartedAt, meta.CreatedAt.UTC().Format(time.RFC3339))
	}
	return nil
}

func ensureManifestMatchesMeta(manifest control.Manifest, meta sessionMeta) error {
	if manifest.ID != meta.Manifest.ID {
		return fmt.Errorf("%w: published session %q manifest id = %q, want %q", ErrConflict, meta.SessionID, manifest.ID, meta.Manifest.ID)
	}
	if manifest.SessionID != meta.SessionID {
		return fmt.Errorf("%w: published session %q manifest session_id = %q", ErrConflict, meta.SessionID, manifest.SessionID)
	}
	if manifest.RootID != meta.RootID {
		return fmt.Errorf("%w: published session %q manifest root_id = %q, want %q", ErrConflict, meta.SessionID, manifest.RootID, meta.RootID)
	}
	if manifest.CreatedAt != meta.CreatedAt.UTC().Format(time.RFC3339) {
		return fmt.Errorf("%w: published session %q manifest created_at = %q, want %q", ErrConflict, meta.SessionID, manifest.CreatedAt, meta.CreatedAt.UTC().Format(time.RFC3339))
	}
	wantEntries := controlEntriesFromProtocol(meta.Manifest.Entries)
	if !reflect.DeepEqual(manifest.Entries, wantEntries) {
		return fmt.Errorf("%w: published session %q manifest entries do not match session metadata", ErrConflict, meta.SessionID)
	}
	return nil
}

func controlEntriesFromProtocol(entries []protocol.ManifestEntry) []control.ManifestEntry {
	out := make([]control.ManifestEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, control.ManifestEntry{
			Path:          entry.Path,
			Kind:          string(entry.Kind),
			Mode:          entry.Mode,
			Size:          entry.Size,
			ModTime:       formatOptionalTime(entry.ModTime),
			Digest:        entry.Digest,
			TargetPath:    entry.TargetPath,
			SymlinkTarget: entry.SymlinkTarget,
		})
	}
	return out
}

func protocolPathError(err error) error {
	if errors.Is(err, pathguard.ErrUnsafePath) {
		return fmt.Errorf("%w: %v", protocol.ErrValidation, err)
	}
	return err
}

func (s FileStore) writeManifestArtifact(req protocol.BeginSessionRequest) error {
	manifest := control.Manifest{
		Version:   control.CurrentVersion,
		ID:        req.Manifest.ID,
		SessionID: req.SessionID,
		RootID:    req.RootID,
		CreatedAt: req.CreatedAt.UTC().Format(time.RFC3339),
		Entries:   make([]control.ManifestEntry, 0, len(req.Manifest.Entries)),
	}
	manifest.Entries = controlEntriesFromProtocol(req.Manifest.Entries)
	path, err := control.Path(s.TargetRoot, control.ArtifactManifest, req.SessionID)
	if err != nil {
		return err
	}
	return control.WriteFile(path, manifest)
}

func (s FileStore) writeReceipt(meta sessionMeta, endedAt time.Time) error {
	receipt := control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        meta.SessionID,
		ProfileID: meta.ProfileID,
		TargetID:  meta.TargetDeviceID,
		StartedAt: meta.CreatedAt.UTC().Format(time.RFC3339),
		EndedAt:   endedAt.UTC().Format(time.RFC3339),
		Status:    string(protocol.SessionStatePublished),
	}
	path, err := control.Path(s.TargetRoot, control.ArtifactSessionReceipt, meta.SessionID)
	if err != nil {
		return err
	}
	return control.WriteFile(path, receipt)
}

func readMeta(path string) (sessionMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return sessionMeta{}, err
	}
	var meta sessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return sessionMeta{}, err
	}
	if err := beginRequestFromMeta(meta).Validate(); err != nil {
		return sessionMeta{}, err
	}
	return meta, nil
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".receiver-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if err := durable.SyncDirBestEffort(filepath.Dir(path)); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func appendAt(root, path string, data []byte) error {
	if err := makeDirectoryInside(root, filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return protocolPathError(fmt.Errorf("%w: staged file %q is a symlink", pathguard.ErrUnsafePath, path))
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: staged file %q is not regular", ErrConflict, path)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func makeDirectoryInside(root, dir string, mode os.FileMode) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	dirAbs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, dirAbs)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return protocolPathError(fmt.Errorf("%w: path escapes root", pathguard.ErrUnsafePath))
	}
	info, err := os.Lstat(rootAbs)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return protocolPathError(fmt.Errorf("%w: root is not a plain directory", pathguard.ErrUnsafePath))
	}
	if rel == "." {
		return nil
	}
	current := rootAbs
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			if err := os.Mkdir(current, mode); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
			info, err = os.Lstat(current)
			if err != nil {
				return err
			}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return protocolPathError(fmt.Errorf("%w: directory component %q is a symlink", pathguard.ErrUnsafePath, current))
		}
		if !info.IsDir() {
			return protocolPathError(fmt.Errorf("%w: directory component %q is not a directory", pathguard.ErrUnsafePath, current))
		}
	}
	return nil
}

func chunkMatches(path string, offset int64, data []byte) (bool, error) {
	if err := ensurePlainFile(path); err != nil {
		return false, err
	}
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	buf := make([]byte, len(data))
	n, err := file.ReadAt(buf, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return n == len(data) && string(buf) == string(data), nil
}

func fileSize(path string) (int64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 0, protocolPathError(fmt.Errorf("%w: staged file %q is a symlink", pathguard.ErrUnsafePath, path))
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("%w: staged path %q is not a regular file", ErrConflict, path)
	}
	return info.Size(), nil
}

func fileDigest(path string) (string, error) {
	if err := ensurePlainFile(path); err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func ensurePlainFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return protocolPathError(fmt.Errorf("%w: file %q is a symlink", pathguard.ErrUnsafePath, path))
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: file %q is not regular", ErrConflict, path)
	}
	return nil
}

func sha256Digest(data []byte) string {
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func findEntry(manifest protocol.TransferManifest, path string) (protocol.ManifestEntry, bool) {
	for _, entry := range manifest.Entries {
		if entry.Path == path {
			return entry, true
		}
	}
	return protocol.ManifestEntry{}, false
}

func sameManifest(meta sessionMeta, req protocol.BeginSessionRequest) bool {
	return meta.ProtocolVersion == req.ProtocolVersion &&
		meta.SessionID == req.SessionID &&
		meta.ProfileID == req.ProfileID &&
		meta.SourceDeviceID == req.SourceDeviceID &&
		meta.TargetDeviceID == req.TargetDeviceID &&
		meta.RootID == req.RootID &&
		meta.CreatedAt.Equal(req.CreatedAt) &&
		reflect.DeepEqual(meta.Manifest, req.Manifest)
}

func fromTransactionState(state transaction.State) protocol.SessionState {
	switch state {
	case transaction.StateReceived:
		return protocol.SessionStateReceived
	case transaction.StateValidated:
		return protocol.SessionStateValidated
	case transaction.StateStaged:
		return protocol.SessionStateStaged
	case transaction.StatePublished:
		return protocol.SessionStatePublished
	case transaction.StateRolledBack:
		return protocol.SessionStateRolledBack
	case transaction.StateNeedsRepair:
		return protocol.SessionStateNeedsRepair
	default:
		return protocol.SessionStateNeedsRepair
	}
}

func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
