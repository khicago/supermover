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
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/durable"
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
	layout := s.layout()
	if err := layout.EnsureSessionDirs(req.SessionID); err != nil {
		return protocol.BeginSessionResponse{}, err
	}

	if existing, err := readMeta(s.metaPath(req.SessionID)); err == nil {
		if !sameManifest(existing, req) {
			return protocol.BeginSessionResponse{}, fmt.Errorf("%w: session %q already exists with different metadata", ErrConflict, req.SessionID)
		}
		status, err := s.Status(req.SessionID)
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
	if err := writeJSONAtomic(s.metaPath(req.SessionID), sessionMeta{
		ProtocolVersion: req.ProtocolVersion,
		SessionID:       req.SessionID,
		ProfileID:       req.ProfileID,
		SourceDeviceID:  req.SourceDeviceID,
		TargetDeviceID:  req.TargetDeviceID,
		RootID:          req.RootID,
		CreatedAt:       req.CreatedAt.UTC(),
		Manifest:        req.Manifest,
	}); err != nil {
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

	meta, record, err := s.loadSession(req.SessionID)
	if err != nil {
		return protocol.ChunkUploadResponse{}, err
	}
	if record.State == transaction.StatePublished || record.State == transaction.StateRolledBack {
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
		if err := appendAt(path, req.Data); err != nil {
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
	meta, record, err := s.loadSession(req.SessionID)
	if err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	if record.State == transaction.StatePublished {
		return protocol.CommitSessionResponse{
			SessionID: req.SessionID,
			State:     protocol.SessionStatePublished,
			ReceiptID: req.SessionID,
		}, nil
	}
	if record.State == transaction.StateRolledBack {
		return protocol.CommitSessionResponse{}, fmt.Errorf("%w: session %q is rolled back", ErrConflict, req.SessionID)
	}
	if err := s.stageNonRegularEntries(meta); err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	if err := s.verifyFiles(meta); err != nil {
		marked, markErr := record.WithState(transaction.StateNeedsRepair, s.now())
		if markErr == nil {
			_ = s.layout().WriteSessionRecord(marked)
		}
		return protocol.CommitSessionResponse{}, err
	}
	staged, err := record.WithState(transaction.StateStaged, s.now())
	if err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	if err := s.layout().WriteSessionRecord(staged); err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	if err := s.publish(meta); err != nil {
		repair, markErr := staged.WithState(transaction.StateNeedsRepair, s.now())
		if markErr == nil {
			_ = s.layout().WriteSessionRecord(repair)
		}
		return protocol.CommitSessionResponse{}, err
	}
	published, err := staged.WithState(transaction.StatePublished, s.now())
	if err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	if err := s.layout().WriteSessionRecord(published); err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	if err := s.writeReceipt(meta, req.EndedAt); err != nil {
		return protocol.CommitSessionResponse{}, err
	}
	return protocol.CommitSessionResponse{
		SessionID: req.SessionID,
		State:     protocol.SessionStatePublished,
		ReceiptID: req.SessionID,
	}, nil
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

func (s FileStore) metaPath(sessionID string) string {
	return filepath.Join(s.layout().SessionDir(sessionID), "network-session.json")
}

func (s FileStore) stageFilePath(sessionID, rel string) (string, error) {
	return safeJoin(s.layout().StagingDir(sessionID), rel)
}

func (s FileStore) finalPath(entry protocol.ManifestEntry) (string, error) {
	target := entry.Path
	if entry.TargetPath != "" {
		target = entry.TargetPath
	}
	return safeJoin(s.TargetRoot, target)
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
			if err := os.MkdirAll(path, 0o755); err != nil {
				return fmt.Errorf("stage directory %q: %w", entry.Path, err)
			}
		case protocol.FileKindSymlink:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("stage symlink parent %q: %w", entry.Path, err)
			}
			_ = os.Remove(path)
			if err := os.Symlink(entry.SymlinkTarget, path); err != nil {
				return fmt.Errorf("stage symlink %q: %w", entry.Path, err)
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

func (s FileStore) publish(meta sessionMeta) error {
	for _, entry := range meta.Manifest.Entries {
		stage, err := s.stageFilePath(meta.SessionID, entry.Path)
		if err != nil {
			return err
		}
		final, err := s.finalPath(entry)
		if err != nil {
			return err
		}
		switch entry.Kind {
		case protocol.FileKindDir:
			if err := os.MkdirAll(final, 0o755); err != nil {
				return fmt.Errorf("publish directory %q: %w", entry.Path, err)
			}
		case protocol.FileKindSymlink:
			if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
				return fmt.Errorf("publish symlink parent %q: %w", entry.Path, err)
			}
			_ = os.Remove(final)
			if err := os.Rename(stage, final); err != nil {
				return fmt.Errorf("publish symlink %q: %w", entry.Path, err)
			}
		case protocol.FileKindFile:
			if err := durable.PromoteFile(stage, final); err != nil {
				return fmt.Errorf("publish file %q: %w", entry.Path, err)
			}
			if !entry.ModTime.IsZero() {
				if err := os.Chtimes(final, entry.ModTime, entry.ModTime); err != nil {
					return fmt.Errorf("apply mtime %q: %w", entry.Path, err)
				}
			}
		}
	}
	return nil
}

func safeJoin(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: absolute path %q", protocol.ErrValidation, rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: unsafe relative path %q", protocol.ErrValidation, rel)
	}
	return filepath.Join(root, clean), nil
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
	for _, entry := range req.Manifest.Entries {
		manifest.Entries = append(manifest.Entries, control.ManifestEntry{
			Path:       entry.Path,
			Kind:       string(entry.Kind),
			Size:       entry.Size,
			ModTime:    formatOptionalTime(entry.ModTime),
			Digest:     entry.Digest,
			TargetPath: entry.TargetPath,
		})
	}
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
	if err := (protocol.BeginSessionRequest{
		ProtocolVersion: meta.ProtocolVersion,
		SessionID:       meta.SessionID,
		ProfileID:       meta.ProfileID,
		SourceDeviceID:  meta.SourceDeviceID,
		TargetDeviceID:  meta.TargetDeviceID,
		RootID:          meta.RootID,
		CreatedAt:       meta.CreatedAt,
		Manifest:        meta.Manifest,
	}).Validate(); err != nil {
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

func appendAt(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func chunkMatches(path string, offset int64, data []byte) (bool, error) {
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
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	if info.IsDir() {
		return 0, fmt.Errorf("%w: staged path %q is a directory", ErrConflict, path)
	}
	return info.Size(), nil
}

func fileDigest(path string) (string, error) {
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
	return t.UTC().Format(time.RFC3339)
}
