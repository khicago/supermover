package protocol

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/transport"
)

const (
	Version = "supermover/1"

	MaxSessionIDLen = 128
	MaxRootIDLen    = 128
	MaxPathLen      = 4096
	MaxDigestLen    = 128
	MaxChunkBytes   = 4 * 1024 * 1024
)

type FileKind string

const (
	FileKindFile    FileKind = "file"
	FileKindDir     FileKind = "dir"
	FileKindSymlink FileKind = "symlink"
)

type ChunkState string

const (
	ChunkStateAccepted  ChunkState = "accepted"
	ChunkStateDuplicate ChunkState = "duplicate"
)

type SessionState string

const (
	SessionStateReceived    SessionState = "received"
	SessionStateValidated   SessionState = "validated"
	SessionStateStaged      SessionState = "staged"
	SessionStatePublished   SessionState = "published"
	SessionStateRolledBack  SessionState = "rolled_back"
	SessionStateNeedsRepair SessionState = "needs_repair"
)

type ErrorCode string

const (
	ErrorCodeBadRequest ErrorCode = "bad_request"
	ErrorCodeConflict   ErrorCode = "conflict"
	ErrorCodeNotFound   ErrorCode = "not_found"
	ErrorCodeIO         ErrorCode = "io_error"
	ErrorCodeIntegrity  ErrorCode = "integrity_failure"
)

var ErrValidation = errors.New("protocol validation failed")

type BeginSessionRequest struct {
	ProtocolVersion string           `json:"protocol_version"`
	SessionID       string           `json:"session_id"`
	ProfileID       string           `json:"profile_id"`
	SourceDeviceID  string           `json:"source_device_id"`
	TargetDeviceID  string           `json:"target_device_id"`
	RootID          string           `json:"root_id,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	Manifest        TransferManifest `json:"manifest"`
}

type BeginSessionResponse struct {
	SessionID  string       `json:"session_id"`
	State      SessionState `json:"state"`
	ResumeFrom []FileStatus `json:"resume_from,omitempty"`
}

type TransferManifest struct {
	ID      string          `json:"id"`
	Entries []ManifestEntry `json:"entries,omitempty"`
}

type ManifestEntry struct {
	Path          string    `json:"path"`
	TargetPath    string    `json:"target_path,omitempty"`
	Kind          FileKind  `json:"kind"`
	Size          int64     `json:"size,omitempty"`
	Digest        string    `json:"digest,omitempty"`
	ModTime       time.Time `json:"mod_time,omitempty"`
	SymlinkTarget string    `json:"symlink_target,omitempty"`
}

type ChunkUploadRequest struct {
	SessionID string `json:"session_id"`
	Path      string `json:"path"`
	Offset    int64  `json:"offset"`
	Data      []byte `json:"data"`
	Digest    string `json:"digest,omitempty"`
	Final     bool   `json:"final,omitempty"`
}

type ChunkUploadResponse struct {
	SessionID     string     `json:"session_id"`
	Path          string     `json:"path"`
	CommittedSize int64      `json:"committed_size"`
	ChunkState    ChunkState `json:"chunk_state"`
	Complete      bool       `json:"complete"`
}

type CommitSessionRequest struct {
	SessionID string    `json:"session_id"`
	EndedAt   time.Time `json:"ended_at"`
}

type CommitSessionResponse struct {
	SessionID string       `json:"session_id"`
	State     SessionState `json:"state"`
	ReceiptID string       `json:"receipt_id"`
}

type SessionStatusResponse struct {
	SessionID string       `json:"session_id"`
	State     SessionState `json:"state"`
	Files     []FileStatus `json:"files,omitempty"`
}

type FileStatus struct {
	Path           string `json:"path"`
	ExpectedSize   int64  `json:"expected_size,omitempty"`
	CommittedSize  int64  `json:"committed_size"`
	ExpectedDigest string `json:"expected_digest,omitempty"`
	Complete       bool   `json:"complete"`
}

type ErrorResponse struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (r BeginSessionRequest) Validate() error {
	var errs []error
	if err := transport.ValidateProtocolVersion(r.ProtocolVersion); err != nil || r.ProtocolVersion != Version {
		errs = append(errs, fmt.Errorf("protocol_version must be %q", Version))
	}
	validateToken("session_id", r.SessionID, MaxSessionIDLen, &errs)
	if err := transport.ValidateProfileID(r.ProfileID); err != nil {
		errs = append(errs, fmt.Errorf("profile_id: %v", err))
	}
	if err := transport.DeviceID(r.SourceDeviceID).Validate(); err != nil {
		errs = append(errs, fmt.Errorf("source_device_id: %v", err))
	}
	if err := transport.DeviceID(r.TargetDeviceID).Validate(); err != nil {
		errs = append(errs, fmt.Errorf("target_device_id: %v", err))
	}
	if r.SourceDeviceID == r.TargetDeviceID {
		errs = append(errs, errors.New("source_device_id and target_device_id must differ"))
	}
	if r.RootID != "" {
		validateToken("root_id", r.RootID, MaxRootIDLen, &errs)
	}
	if r.CreatedAt.IsZero() {
		errs = append(errs, errors.New("created_at is required"))
	}
	if err := r.Manifest.Validate(); err != nil {
		errs = append(errs, err)
	}
	return joinValidation(errs)
}

func (m TransferManifest) Validate() error {
	var errs []error
	validateToken("manifest.id", m.ID, MaxSessionIDLen, &errs)
	seen := map[string]struct{}{}
	seenTargets := map[string]string{}
	for i, entry := range m.Entries {
		if err := entry.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("manifest.entries[%d]: %w", i, err))
		}
		if _, ok := seen[entry.Path]; ok {
			errs = append(errs, fmt.Errorf("manifest.entries[%d]: duplicate path %q", i, entry.Path))
		}
		seen[entry.Path] = struct{}{}
		targetPath := entry.Path
		if entry.TargetPath != "" {
			targetPath = entry.TargetPath
		}
		if firstPath, ok := seenTargets[targetPath]; ok && firstPath != entry.Path {
			errs = append(errs, fmt.Errorf("manifest.entries[%d]: duplicate target path %q also used by %q", i, targetPath, firstPath))
		}
		seenTargets[targetPath] = entry.Path
	}
	return joinValidation(errs)
}

func (e ManifestEntry) Validate() error {
	var errs []error
	validateRelativePath("path", e.Path, &errs)
	if e.TargetPath != "" {
		validateRelativePath("target_path", e.TargetPath, &errs)
	}
	switch e.Kind {
	case FileKindFile:
		if e.Size < 0 {
			errs = append(errs, errors.New("size cannot be negative"))
		}
		validateDigest("digest", e.Digest, true, &errs)
	case FileKindDir:
		if e.Size != 0 {
			errs = append(errs, errors.New("directory size must be zero"))
		}
	case FileKindSymlink:
		if strings.TrimSpace(e.SymlinkTarget) == "" {
			errs = append(errs, errors.New("symlink_target is required for symlinks"))
		}
	default:
		errs = append(errs, fmt.Errorf("kind must be one of %s, %s, %s", FileKindFile, FileKindDir, FileKindSymlink))
	}
	return joinValidation(errs)
}

func (r ChunkUploadRequest) Validate() error {
	var errs []error
	validateToken("session_id", r.SessionID, MaxSessionIDLen, &errs)
	validateRelativePath("path", r.Path, &errs)
	if r.Offset < 0 {
		errs = append(errs, errors.New("offset cannot be negative"))
	}
	if len(r.Data) == 0 {
		errs = append(errs, errors.New("data is required"))
	}
	if len(r.Data) > MaxChunkBytes {
		errs = append(errs, fmt.Errorf("data exceeds maximum chunk size %d", MaxChunkBytes))
	}
	validateDigest("digest", r.Digest, false, &errs)
	return joinValidation(errs)
}

func (r CommitSessionRequest) Validate() error {
	var errs []error
	validateToken("session_id", r.SessionID, MaxSessionIDLen, &errs)
	if r.EndedAt.IsZero() {
		errs = append(errs, errors.New("ended_at is required"))
	}
	return joinValidation(errs)
}

func (s SessionState) Valid() bool {
	switch s {
	case SessionStateReceived, SessionStateValidated, SessionStateStaged, SessionStatePublished, SessionStateRolledBack, SessionStateNeedsRepair:
		return true
	default:
		return false
	}
}

func validateToken(field, value string, maxLen int, errs *[]error) {
	if strings.TrimSpace(value) == "" {
		*errs = append(*errs, fmt.Errorf("%s is required", field))
		return
	}
	if len(value) > maxLen {
		*errs = append(*errs, fmt.Errorf("%s is too long", field))
		return
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && !strings.ContainsRune("._-", r) {
			*errs = append(*errs, fmt.Errorf("%s contains unsafe characters", field))
			return
		}
	}
}

func validateRelativePath(field, value string, errs *[]error) {
	if strings.TrimSpace(value) == "" {
		*errs = append(*errs, fmt.Errorf("%s is required", field))
		return
	}
	if len(value) > MaxPathLen {
		*errs = append(*errs, fmt.Errorf("%s is too long", field))
		return
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`) || strings.Contains(value, `\`) {
		*errs = append(*errs, fmt.Errorf("%s must be a slash-separated relative path", field))
		return
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			*errs = append(*errs, fmt.Errorf("%s contains unsafe segment %q", field, part))
			return
		}
	}
}

func validateDigest(field, value string, required bool, errs *[]error) {
	if value == "" {
		if required {
			*errs = append(*errs, fmt.Errorf("%s is required", field))
		}
		return
	}
	if len(value) > MaxDigestLen || !strings.HasPrefix(value, "sha256:") {
		*errs = append(*errs, fmt.Errorf("%s must be a sha256 digest", field))
		return
	}
	hexValue := strings.TrimPrefix(value, "sha256:")
	if len(hexValue) != 64 {
		*errs = append(*errs, fmt.Errorf("%s must contain 64 hex characters", field))
		return
	}
	for _, r := range hexValue {
		if !(r >= 'a' && r <= 'f') && !(r >= 'A' && r <= 'F') && !(r >= '0' && r <= '9') {
			*errs = append(*errs, fmt.Errorf("%s must be hexadecimal", field))
			return
		}
	}
}

func joinValidation(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrValidation, errors.Join(errs...))
}
