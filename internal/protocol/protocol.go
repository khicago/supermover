package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/privacy/padding"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
)

const (
	Version = "supermover/1"

	EmptySHA256Digest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	MaxSessionIDLen = 128
	MaxRootIDLen    = 128
	MaxPathLen      = 4096
	MaxDigestLen    = 128
	MaxChunkBytes   = 4 * 1024 * 1024

	MaxChunkRequestBodyBytes       = ((MaxChunkBytes + 2) / 3 * 4) + 256*1024
	MaxPaddingBucketBytes          = 1024 * 1024
	MaxPaddedChunkRequestBodyBytes = MaxChunkRequestBodyBytes + MaxPaddingBucketBytes + 1024
	MaxBatchChunks                 = 1024
	MaxBatchPlainBodyBytes         = MaxChunkRequestBodyBytes
	MaxPaddedBatchRequestBodyBytes = MaxBatchPlainBodyBytes + MaxPaddingBucketBytes + 1024

	MaxManifestEntries             = 100_000
	MaxTotalDeclaredBytes          = 1 << 50
	MaxArtifactDocumentBytes       = 1024 * 1024
	MaxSymlinkTargetLen            = pathguard.MaxSymlinkTargetLen
	MaxFileMode              int64 = 0o777
)

const (
	FrameEncodingHeader    = "X-Supermover-Frame-Encoding"
	FrameEncodingPaddingV1 = "padding-v1"
	FrameSessionIDHeader   = "X-Supermover-Frame-Session-ID"
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
	ErrorCodeForbidden  ErrorCode = "forbidden"
	ErrorCodeNotFound   ErrorCode = "not_found"
	ErrorCodeIO         ErrorCode = "io_error"
	ErrorCodeIntegrity  ErrorCode = "integrity_failure"
)

var ErrValidation = errors.New("protocol validation failed")

type BeginSessionRequest struct {
	ProtocolVersion string                  `json:"protocol_version"`
	SessionID       string                  `json:"session_id"`
	ProfileID       string                  `json:"profile_id"`
	TargetID        string                  `json:"target_id"`
	SourceDeviceID  string                  `json:"source_device_id"`
	TargetDeviceID  string                  `json:"target_device_id"`
	PrivacyPolicy   transport.PrivacyPolicy `json:"privacy_policy,omitempty"`
	RootID          string                  `json:"root_id,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	Manifest        TransferManifest        `json:"manifest"`
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
	Mode          uint32    `json:"mode,omitempty"`
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

type ChunkBatchUploadRequest struct {
	SessionID string               `json:"session_id"`
	Chunks    []ChunkUploadRequest `json:"chunks"`
}

type ChunkUploadResponse struct {
	SessionID     string     `json:"session_id"`
	Path          string     `json:"path"`
	CommittedSize int64      `json:"committed_size"`
	ChunkState    ChunkState `json:"chunk_state"`
	Complete      bool       `json:"complete"`
}

type ChunkBatchUploadResponse struct {
	SessionID string                `json:"session_id"`
	Chunks    []ChunkUploadResponse `json:"chunks"`
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

type ProfileSnapshotArtifactRequest struct {
	SessionID string `json:"session_id"`
	Document  []byte `json:"document"`
}

type WarningArtifactRequest struct {
	SessionID string   `json:"session_id"`
	Documents [][]byte `json:"documents"`
}

type NetworkTransferArtifactRequest struct {
	SessionID string `json:"session_id"`
	Document  []byte `json:"document"`
}

type ArtifactWriteResponse struct {
	SessionID string `json:"session_id"`
	Written   int    `json:"written"`
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
	validateSessionID("session_id", r.SessionID, &errs)
	if err := transport.ValidateProfileID(r.ProfileID); err != nil {
		errs = append(errs, fmt.Errorf("profile_id: %v", err))
	}
	if err := transport.ValidateProfileID(r.TargetID); err != nil {
		errs = append(errs, fmt.Errorf("target_id: %v", err))
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
	if r.PrivacyPolicy.Level != 0 {
		if err := r.PrivacyPolicy.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("privacy_policy: %w", err))
		}
		if err := r.validateProtocolPrivacyBounds(); err != nil {
			errs = append(errs, fmt.Errorf("privacy_policy: %w", err))
		}
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

func (r BeginSessionRequest) validateProtocolPrivacyBounds() error {
	policy := r.PrivacyPolicy
	if policy.Level != transport.PrivacyLevel2 {
		return nil
	}
	var errs []error
	if policy.PaddingBucket > MaxPaddingBucketBytes {
		errs = append(errs, fmt.Errorf("padding_bucket_bytes %d exceeds maximum %d", policy.PaddingBucket, MaxPaddingBucketBytes))
	}
	if policy.BatchMaxBytes > MaxBatchPlainBodyBytes {
		errs = append(errs, fmt.Errorf("batch_max_bytes %d exceeds maximum %d", policy.BatchMaxBytes, MaxBatchPlainBodyBytes))
	}
	if policy.BatchMaxCount > MaxBatchChunks {
		errs = append(errs, fmt.Errorf("batch_max_count %d exceeds maximum %d", policy.BatchMaxCount, MaxBatchChunks))
	}
	if policy.PaddingBucket > 0 && policy.BatchMaxBytes > 0 {
		if _, _, err := padding.PaddedLen(policy.BatchMaxBytes, padding.Config{
			BucketBytes:   policy.PaddingBucket,
			MaxFrameBytes: MaxPaddedBatchRequestBodyBytes,
		}); err != nil {
			errs = append(errs, fmt.Errorf("padded batch frame for batch_max_bytes %d: %w", policy.BatchMaxBytes, err))
		}
	}
	return joinValidation(errs)
}

func (m TransferManifest) Validate() error {
	var errs []error
	validateToken("manifest.id", m.ID, MaxSessionIDLen, &errs)
	if len(m.Entries) > MaxManifestEntries {
		errs = append(errs, fmt.Errorf("manifest.entries exceeds maximum count %d", MaxManifestEntries))
	}
	seen := map[string]struct{}{}
	seenTargets := map[string]string{}
	var totalDeclared int64
	var sourceRefs []pathRef
	var targetRefs []pathRef
	for i, entry := range m.Entries {
		if err := entry.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("manifest.entries[%d]: %w", i, err))
		}
		if entry.Kind == FileKindFile {
			if entry.Size > MaxTotalDeclaredBytes-totalDeclared {
				errs = append(errs, fmt.Errorf("manifest.entries[%d]: total declared file size exceeds maximum %d", i, MaxTotalDeclaredBytes))
			} else {
				totalDeclared += entry.Size
			}
		}
		sourceRefs = append(sourceRefs, pathRef{index: i, path: entry.Path, symlink: entry.Kind == FileKindSymlink})
		if _, ok := seen[entry.Path]; ok {
			errs = append(errs, fmt.Errorf("manifest.entries[%d]: duplicate path %q", i, entry.Path))
		}
		seen[entry.Path] = struct{}{}
		targetPath := entryTargetPath(entry)
		targetRefs = append(targetRefs, pathRef{index: i, path: targetPath, symlink: entry.Kind == FileKindSymlink})
		if pathguard.IsReservedControlPath(targetPath) {
			errs = append(errs, fmt.Errorf("manifest.entries[%d]: target path %q uses reserved control directory", i, targetPath))
		}
		if firstPath, ok := seenTargets[targetPath]; ok && firstPath != entry.Path {
			errs = append(errs, fmt.Errorf("manifest.entries[%d]: duplicate target path %q also used by %q", i, targetPath, firstPath))
		}
		seenTargets[targetPath] = entry.Path
	}
	errs = append(errs, validateNoEntriesBelowSymlinks(sourceRefs, "path")...)
	errs = append(errs, validateNoEntriesBelowSymlinks(targetRefs, "target path")...)
	return joinValidation(errs)
}

func entryTargetPath(entry ManifestEntry) string {
	if entry.TargetPath != "" {
		return entry.TargetPath
	}
	return entry.Path
}

type pathRef struct {
	index   int
	path    string
	symlink bool
}

func validateNoEntriesBelowSymlinks(refs []pathRef, label string) []error {
	sorted := append([]pathRef(nil), refs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].path == sorted[j].path {
			return sorted[i].index < sorted[j].index
		}
		return sorted[i].path < sorted[j].path
	})
	var errs []error
	var stack []pathRef
	for _, ref := range sorted {
		for len(stack) > 0 && !strings.HasPrefix(ref.path, stack[len(stack)-1].path+"/") {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 && ref.index != stack[len(stack)-1].index {
			active := stack[len(stack)-1]
			errs = append(errs, fmt.Errorf("manifest.entries[%d]: %s %q is below symlink %s %q", ref.index, label, ref.path, label, active.path))
		}
		if ref.symlink {
			stack = append(stack, ref)
		}
	}
	return errs
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
		validateMode("mode", e.Mode, &errs)
		validateDigest("digest", e.Digest, true, &errs)
		if e.Size == 0 && e.Digest != "" && e.Digest != EmptySHA256Digest {
			errs = append(errs, fmt.Errorf("digest must be %s for zero-byte files", EmptySHA256Digest))
		}
	case FileKindDir:
		if e.Size != 0 {
			errs = append(errs, errors.New("directory size must be zero"))
		}
		validateMode("mode", e.Mode, &errs)
	case FileKindSymlink:
		if strings.TrimSpace(e.SymlinkTarget) == "" {
			errs = append(errs, errors.New("symlink_target is required for symlinks"))
		}
		validateSymlinkTarget("symlink_target", e.SymlinkTarget, &errs)
	default:
		errs = append(errs, fmt.Errorf("kind must be one of %s, %s, %s", FileKindFile, FileKindDir, FileKindSymlink))
	}
	return joinValidation(errs)
}

func (r ChunkUploadRequest) Validate() error {
	var errs []error
	validateSessionID("session_id", r.SessionID, &errs)
	validateRelativePath("path", r.Path, &errs)
	if r.Offset < 0 {
		errs = append(errs, errors.New("offset cannot be negative"))
	}
	if len(r.Data) == 0 {
		if !r.Final {
			errs = append(errs, errors.New("zero-byte completion must be final"))
		}
		if r.Offset != 0 {
			errs = append(errs, errors.New("zero-byte completion offset must be zero"))
		}
		if r.Digest != "" && r.Digest != EmptySHA256Digest {
			errs = append(errs, fmt.Errorf("zero-byte completion digest must be %s", EmptySHA256Digest))
		}
	}
	if len(r.Data) > MaxChunkBytes {
		errs = append(errs, fmt.Errorf("data exceeds maximum chunk size %d", MaxChunkBytes))
	}
	validateDigest("digest", r.Digest, false, &errs)
	return joinValidation(errs)
}

func (r ChunkBatchUploadRequest) Validate() error {
	var errs []error
	validateSessionID("session_id", r.SessionID, &errs)
	if len(r.Chunks) == 0 {
		errs = append(errs, errors.New("chunks must not be empty"))
	}
	if len(r.Chunks) > MaxBatchChunks {
		errs = append(errs, fmt.Errorf("chunks exceeds maximum %d", MaxBatchChunks))
	}
	for i, chunk := range r.Chunks {
		if err := chunk.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("chunks[%d]: %w", i, err))
			continue
		}
		if chunk.SessionID != r.SessionID {
			errs = append(errs, fmt.Errorf("chunks[%d].session_id must match batch session_id", i))
		}
	}
	return joinValidation(errs)
}

func (r CommitSessionRequest) Validate() error {
	var errs []error
	validateSessionID("session_id", r.SessionID, &errs)
	if r.EndedAt.IsZero() {
		errs = append(errs, errors.New("ended_at is required"))
	}
	return joinValidation(errs)
}

func (r ProfileSnapshotArtifactRequest) Validate() error {
	var errs []error
	validateSessionID("session_id", r.SessionID, &errs)
	validateDocument("document", r.Document, &errs)
	return joinValidation(errs)
}

func (r WarningArtifactRequest) Validate() error {
	var errs []error
	validateSessionID("session_id", r.SessionID, &errs)
	if len(r.Documents) == 0 {
		errs = append(errs, errors.New("documents must not be empty"))
	}
	if len(r.Documents) > MaxManifestEntries {
		errs = append(errs, fmt.Errorf("documents exceeds maximum count %d", MaxManifestEntries))
	}
	for i, doc := range r.Documents {
		validateDocument(fmt.Sprintf("documents[%d]", i), doc, &errs)
	}
	return joinValidation(errs)
}

func (r NetworkTransferArtifactRequest) Validate() error {
	var errs []error
	validateSessionID("session_id", r.SessionID, &errs)
	validateDocument("document", r.Document, &errs)
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

func validateSessionID(field, value string, errs *[]error) {
	if strings.TrimSpace(value) == "" {
		*errs = append(*errs, fmt.Errorf("%s is required", field))
		return
	}
	if len(value) > MaxSessionIDLen {
		*errs = append(*errs, fmt.Errorf("%s is too long", field))
		return
	}
	if err := transaction.ValidateSessionID(value); err != nil {
		*errs = append(*errs, fmt.Errorf("%s is unsafe: %v", field, err))
	}
}

func validateDocument(field string, data []byte, errs *[]error) {
	if len(data) == 0 {
		*errs = append(*errs, fmt.Errorf("%s is required", field))
		return
	}
	if len(data) > MaxArtifactDocumentBytes {
		*errs = append(*errs, fmt.Errorf("%s exceeds maximum size %d", field, MaxArtifactDocumentBytes))
		return
	}
	if !json.Valid(data) {
		*errs = append(*errs, fmt.Errorf("%s must contain valid JSON", field))
	}
}

func validateRelativePath(field, value string, errs *[]error) {
	if err := pathguard.ValidateSlashRelativePath(value, MaxPathLen); err != nil {
		switch {
		case strings.Contains(err.Error(), "is required"):
			*errs = append(*errs, fmt.Errorf("%s is required", field))
		case strings.Contains(err.Error(), "is too long"):
			*errs = append(*errs, fmt.Errorf("%s is too long", field))
		case strings.Contains(err.Error(), "unsafe segment"):
			_, segment, _ := strings.Cut(err.Error(), "unsafe segment ")
			*errs = append(*errs, fmt.Errorf("%s contains unsafe segment %s", field, segment))
		default:
			*errs = append(*errs, fmt.Errorf("%s must be a slash-separated relative path", field))
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

func validateMode(field string, value uint32, errs *[]error) {
	if value == 0 {
		return
	}
	if value&^uint32(MaxFileMode) != 0 {
		*errs = append(*errs, fmt.Errorf("%s must contain only permission bits", field))
	}
}

func validateSymlinkTarget(field, value string, errs *[]error) {
	if err := pathguard.ValidateRelativeSymlinkTarget(value); err != nil {
		*errs = append(*errs, fmt.Errorf("%s is unsafe: %w", field, err))
	}
}

func joinValidation(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrValidation, errors.Join(errs...))
}
