package control

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/khicago/supermover/internal/durable"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
)

const (
	DirName        = ".supermover"
	CurrentVersion = 1
)

// ErrArtifactExists reports a refused no-replace control artifact write.
var ErrArtifactExists = errors.New("control artifact already exists")

type ArtifactType string

const (
	ArtifactProfileSnapshot ArtifactType = "profile_snapshot"
	ArtifactPairingReceipt  ArtifactType = "pairing_receipt"
	ArtifactSessionReceipt  ArtifactType = "session_receipt"
	ArtifactManifest        ArtifactType = "manifest"
	ArtifactWarning         ArtifactType = "warning"
	ArtifactTargetDrift     ArtifactType = "target_drift"
	ArtifactSoftDelete      ArtifactType = "soft_delete"
	ArtifactPruneApproval   ArtifactType = "prune_approval"
	ArtifactPruneReceipt    ArtifactType = "prune_receipt"
	ArtifactHistoryIndex    ArtifactType = "history_index"
	ArtifactRecoveryState   ArtifactType = "recovery_state"
	ArtifactNetworkTransfer ArtifactType = "network_transfer"
)

type ProfileSnapshot struct {
	Version    int             `json:"version"`
	ID         string          `json:"id"`
	ProfileID  string          `json:"profile_id"`
	SessionID  string          `json:"session_id,omitempty"`
	CapturedAt string          `json:"captured_at"`
	Profile    json.RawMessage `json:"profile"`
}

type PairingReceipt struct {
	Version            int    `json:"version"`
	ID                 string `json:"id"`
	ProfileID          string `json:"profile_id"`
	TargetID           string `json:"target_id"`
	SourceDeviceID     string `json:"source_device_id"`
	TargetDeviceID     string `json:"target_device_id"`
	DevicePublicKey    string `json:"device_public_key"`
	Method             string `json:"method"`
	VerifiedAt         string `json:"verified_at"`
	VerificationPhrase string `json:"verification_phrase,omitempty"`
	VerificationHash   string `json:"verification_hash,omitempty"`
	ProtocolVersion    string `json:"protocol_version"`
}

type SessionReceipt struct {
	Version   int    `json:"version"`
	ID        string `json:"id"`
	ProfileID string `json:"profile_id"`
	TargetID  string `json:"target_id"`
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at,omitempty"`
	Status    string `json:"status"`
}

type Manifest struct {
	Version   int             `json:"version"`
	ID        string          `json:"id"`
	SessionID string          `json:"session_id"`
	RootID    string          `json:"root_id,omitempty"`
	CreatedAt string          `json:"created_at"`
	Entries   []ManifestEntry `json:"entries,omitempty"`
}

type ManifestEntry struct {
	Path               string `json:"path"`
	Kind               string `json:"kind"`
	Mode               uint32 `json:"mode,omitempty"`
	Size               int64  `json:"size,omitempty"`
	ModTime            string `json:"mod_time,omitempty"`
	Digest             string `json:"digest,omitempty"`
	TargetPath         string `json:"target_path,omitempty"`
	SymlinkTarget      string `json:"symlink_target,omitempty"`
	PreviousSessionID  string `json:"previous_session_id,omitempty"`
	PreviousManifestID string `json:"previous_manifest_id,omitempty"`
	PreviousSize       int64  `json:"previous_size,omitempty"`
	PreviousDigest     string `json:"previous_digest,omitempty"`
	PreviousMode       uint32 `json:"previous_mode,omitempty"`
	PreviousModTime    string `json:"previous_mod_time,omitempty"`

	modePresent         bool
	sizePresent         bool
	previousModePresent bool
	previousSizePresent bool
}

type manifestEntryJSON struct {
	Path               string  `json:"path"`
	Kind               string  `json:"kind"`
	Mode               *uint32 `json:"mode,omitempty"`
	Size               *int64  `json:"size,omitempty"`
	ModTime            string  `json:"mod_time,omitempty"`
	Digest             string  `json:"digest,omitempty"`
	TargetPath         string  `json:"target_path,omitempty"`
	SymlinkTarget      string  `json:"symlink_target,omitempty"`
	PreviousSessionID  string  `json:"previous_session_id,omitempty"`
	PreviousManifestID string  `json:"previous_manifest_id,omitempty"`
	PreviousSize       *int64  `json:"previous_size,omitempty"`
	PreviousDigest     string  `json:"previous_digest,omitempty"`
	PreviousMode       *uint32 `json:"previous_mode,omitempty"`
	PreviousModTime    string  `json:"previous_mod_time,omitempty"`
}

func (e ManifestEntry) MarshalJSON() ([]byte, error) {
	wire := manifestEntryJSON{
		Path:               e.Path,
		Kind:               e.Kind,
		ModTime:            e.ModTime,
		Digest:             e.Digest,
		TargetPath:         e.TargetPath,
		SymlinkTarget:      e.SymlinkTarget,
		PreviousSessionID:  e.PreviousSessionID,
		PreviousManifestID: e.PreviousManifestID,
		PreviousDigest:     e.PreviousDigest,
		PreviousModTime:    e.PreviousModTime,
	}
	if e.modePresent || e.Mode != 0 {
		mode := e.Mode
		wire.Mode = &mode
	}
	if e.sizePresent || e.Size != 0 {
		size := e.Size
		wire.Size = &size
	}
	if e.previousSizePresent || e.PreviousSize != 0 {
		size := e.PreviousSize
		wire.PreviousSize = &size
	}
	if e.previousModePresent || e.PreviousMode != 0 {
		mode := e.PreviousMode
		wire.PreviousMode = &mode
	}
	return json.Marshal(wire)
}

func (e *ManifestEntry) UnmarshalJSON(data []byte) error {
	var wire manifestEntryJSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*e = ManifestEntry{
		Path:                wire.Path,
		Kind:                wire.Kind,
		ModTime:             wire.ModTime,
		Digest:              wire.Digest,
		TargetPath:          wire.TargetPath,
		SymlinkTarget:       wire.SymlinkTarget,
		PreviousSessionID:   wire.PreviousSessionID,
		PreviousManifestID:  wire.PreviousManifestID,
		PreviousDigest:      wire.PreviousDigest,
		PreviousModTime:     wire.PreviousModTime,
		modePresent:         raw["mode"] != nil,
		sizePresent:         raw["size"] != nil,
		previousModePresent: raw["previous_mode"] != nil,
		previousSizePresent: raw["previous_size"] != nil,
	}
	if wire.Mode != nil {
		e.Mode = *wire.Mode
	}
	if wire.Size != nil {
		e.Size = *wire.Size
	}
	if wire.PreviousMode != nil {
		e.PreviousMode = *wire.PreviousMode
	}
	if wire.PreviousSize != nil {
		e.PreviousSize = *wire.PreviousSize
	}
	return nil
}

func (e *ManifestEntry) SetModeEvidence(mode uint32) {
	e.Mode = mode
	e.modePresent = true
}

func (e *ManifestEntry) SetSizeEvidence(size int64) {
	e.Size = size
	e.sizePresent = true
}

func (e *ManifestEntry) SetPreviousModeEvidence(mode uint32) {
	e.PreviousMode = mode
	e.previousModePresent = true
}

func (e *ManifestEntry) SetPreviousSizeEvidence(size int64) {
	e.PreviousSize = size
	e.previousSizePresent = true
}

func (e ManifestEntry) HasModeEvidence() bool {
	return e.modePresent || e.Mode != 0
}

func (e ManifestEntry) HasSizeEvidence() bool {
	return e.sizePresent || e.Size != 0
}

func (e ManifestEntry) HasPreviousModeEvidence() bool {
	return e.previousModePresent || e.PreviousMode != 0
}

func (e ManifestEntry) HasPreviousSizeEvidence() bool {
	return e.previousSizePresent || e.PreviousSize != 0
}

func (e ManifestEntry) EqualManifestEvidence(other ManifestEntry) bool {
	return e.Path == other.Path &&
		e.Kind == other.Kind &&
		e.Mode == other.Mode &&
		e.Size == other.Size &&
		e.ModTime == other.ModTime &&
		e.Digest == other.Digest &&
		e.TargetPath == other.TargetPath &&
		e.SymlinkTarget == other.SymlinkTarget &&
		e.PreviousSessionID == other.PreviousSessionID &&
		e.PreviousManifestID == other.PreviousManifestID &&
		e.PreviousSize == other.PreviousSize &&
		e.PreviousDigest == other.PreviousDigest &&
		e.PreviousMode == other.PreviousMode &&
		e.PreviousModTime == other.PreviousModTime &&
		e.HasModeEvidence() == other.HasModeEvidence() &&
		e.HasSizeEvidence() == other.HasSizeEvidence() &&
		e.HasPreviousModeEvidence() == other.HasPreviousModeEvidence() &&
		e.HasPreviousSizeEvidence() == other.HasPreviousSizeEvidence()
}

func EqualManifestEntries(a, b []ManifestEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].EqualManifestEvidence(b[i]) {
			return false
		}
	}
	return true
}

type Warning struct {
	Version               int               `json:"version"`
	ID                    string            `json:"id"`
	SessionID             string            `json:"session_id,omitempty"`
	Code                  string            `json:"code"`
	Message               string            `json:"message"`
	Severity              string            `json:"severity,omitempty"`
	Paths                 []string          `json:"paths,omitempty"`
	TargetPath            string            `json:"target_path,omitempty"`
	Detected              map[string]string `json:"detected,omitempty"`
	SuggestedProfilePatch map[string]string `json:"suggested_profile_patch,omitempty"`
	SuggestedConfig       map[string]string `json:"suggested_config,omitempty"`
	CreatedAt             string            `json:"created_at"`
}

type TargetDrift struct {
	Version        int                      `json:"version"`
	ID             string                   `json:"id"`
	SessionID      string                   `json:"session_id,omitempty"`
	ProfileID      string                   `json:"profile_id,omitempty"`
	TargetID       string                   `json:"target_id,omitempty"`
	RootID         string                   `json:"root_id,omitempty"`
	Path           string                   `json:"path"`
	DetectedAt     string                   `json:"detected_at"`
	LastDetectedAt string                   `json:"last_detected_at,omitempty"`
	Change         string                   `json:"change"`
	Expected       TargetDriftExpectedState `json:"expected,omitempty"`
	Observed       TargetDriftObservedState `json:"observed,omitempty"`
	ReviewState    string                   `json:"review_state,omitempty"`
	ReviewedAt     string                   `json:"reviewed_at,omitempty"`
	ReviewedBy     string                   `json:"reviewed_by,omitempty"`
	ReviewReason   string                   `json:"review_reason,omitempty"`
	ReviewAction   string                   `json:"review_action,omitempty"`
	ReviewHistory  []TargetDriftReviewEvent `json:"review_history,omitempty"`
	Evidence       []string                 `json:"evidence,omitempty"`
}

type TargetDriftReviewEvent struct {
	ReviewState     string `json:"review_state"`
	ReviewAction    string `json:"review_action,omitempty"`
	ReviewedAt      string `json:"reviewed_at,omitempty"`
	ReviewedBy      string `json:"reviewed_by,omitempty"`
	ReviewReason    string `json:"review_reason,omitempty"`
	ReconciledAt    string `json:"reconciled_at"`
	ReconcileAction string `json:"reconcile_action"`
}

type TargetDriftExpectedState struct {
	SessionID     string `json:"session_id,omitempty"`
	ManifestID    string `json:"manifest_id,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Path          string `json:"path,omitempty"`
	Size          int64  `json:"size,omitempty"`
	Digest        string `json:"digest,omitempty"`
	Mode          uint32 `json:"mode,omitempty"`
	ModTime       string `json:"mod_time,omitempty"`
	SymlinkTarget string `json:"symlink_target,omitempty"`

	sizePresent bool
	modePresent bool
}

type targetDriftExpectedStateJSON struct {
	SessionID     string  `json:"session_id,omitempty"`
	ManifestID    string  `json:"manifest_id,omitempty"`
	Kind          string  `json:"kind,omitempty"`
	Path          string  `json:"path,omitempty"`
	Size          *int64  `json:"size,omitempty"`
	Digest        string  `json:"digest,omitempty"`
	Mode          *uint32 `json:"mode,omitempty"`
	ModTime       string  `json:"mod_time,omitempty"`
	SymlinkTarget string  `json:"symlink_target,omitempty"`
}

func (s TargetDriftExpectedState) MarshalJSON() ([]byte, error) {
	wire := targetDriftExpectedStateJSON{
		SessionID:     s.SessionID,
		ManifestID:    s.ManifestID,
		Kind:          s.Kind,
		Path:          s.Path,
		Digest:        s.Digest,
		ModTime:       s.ModTime,
		SymlinkTarget: s.SymlinkTarget,
	}
	if s.sizePresent || s.Size != 0 {
		wire.Size = &s.Size
	}
	if s.modePresent || s.Mode != 0 {
		wire.Mode = &s.Mode
	}
	return json.Marshal(wire)
}

func (s *TargetDriftExpectedState) UnmarshalJSON(data []byte) error {
	var wire targetDriftExpectedStateJSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	s.SessionID = wire.SessionID
	s.ManifestID = wire.ManifestID
	s.Kind = wire.Kind
	s.Path = wire.Path
	if wire.Size != nil {
		s.Size = *wire.Size
		s.sizePresent = true
	}
	s.Digest = wire.Digest
	if wire.Mode != nil {
		s.Mode = *wire.Mode
		s.modePresent = true
	}
	s.ModTime = wire.ModTime
	s.SymlinkTarget = wire.SymlinkTarget
	return nil
}

func (s *TargetDriftExpectedState) SetSizeEvidence(size int64) {
	s.Size = size
	s.sizePresent = true
}

func (s TargetDriftExpectedState) HasSizeEvidence() bool {
	return s.sizePresent || s.Size != 0
}

func (s *TargetDriftExpectedState) SetModeEvidence(mode uint32) {
	s.Mode = mode
	s.modePresent = true
}

func (s TargetDriftExpectedState) HasModeEvidence() bool {
	return s.modePresent || s.Mode != 0
}

func (s TargetDriftExpectedState) empty() bool {
	return strings.TrimSpace(s.SessionID) == "" &&
		strings.TrimSpace(s.ManifestID) == "" &&
		strings.TrimSpace(s.Kind) == "" &&
		strings.TrimSpace(s.Path) == "" &&
		!s.HasSizeEvidence() &&
		strings.TrimSpace(s.Digest) == "" &&
		!s.HasModeEvidence() &&
		strings.TrimSpace(s.ModTime) == "" &&
		strings.TrimSpace(s.SymlinkTarget) == ""
}

type TargetDriftObservedState struct {
	Present       *bool  `json:"present,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Path          string `json:"path,omitempty"`
	Size          int64  `json:"size,omitempty"`
	Digest        string `json:"digest,omitempty"`
	Mode          uint32 `json:"mode,omitempty"`
	ModTime       string `json:"mod_time,omitempty"`
	SymlinkTarget string `json:"symlink_target,omitempty"`

	sizePresent bool
	modePresent bool
}

type targetDriftObservedStateJSON struct {
	Present       *bool   `json:"present,omitempty"`
	Kind          string  `json:"kind,omitempty"`
	Path          string  `json:"path,omitempty"`
	Size          *int64  `json:"size,omitempty"`
	Digest        string  `json:"digest,omitempty"`
	Mode          *uint32 `json:"mode,omitempty"`
	ModTime       string  `json:"mod_time,omitempty"`
	SymlinkTarget string  `json:"symlink_target,omitempty"`
}

func (s TargetDriftObservedState) MarshalJSON() ([]byte, error) {
	wire := targetDriftObservedStateJSON{
		Present:       s.Present,
		Kind:          s.Kind,
		Path:          s.Path,
		Digest:        s.Digest,
		ModTime:       s.ModTime,
		SymlinkTarget: s.SymlinkTarget,
	}
	if s.sizePresent || s.Size != 0 {
		wire.Size = &s.Size
	}
	if s.modePresent || s.Mode != 0 {
		wire.Mode = &s.Mode
	}
	return json.Marshal(wire)
}

func (s *TargetDriftObservedState) UnmarshalJSON(data []byte) error {
	var wire targetDriftObservedStateJSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	s.Present = wire.Present
	s.Kind = wire.Kind
	s.Path = wire.Path
	if wire.Size != nil {
		s.Size = *wire.Size
		s.sizePresent = true
	}
	s.Digest = wire.Digest
	if wire.Mode != nil {
		s.Mode = *wire.Mode
		s.modePresent = true
	}
	s.ModTime = wire.ModTime
	s.SymlinkTarget = wire.SymlinkTarget
	return nil
}

func (s *TargetDriftObservedState) SetSizeEvidence(size int64) {
	s.Size = size
	s.sizePresent = true
}

func (s TargetDriftObservedState) HasSizeEvidence() bool {
	return s.sizePresent || s.Size != 0
}

func (s *TargetDriftObservedState) SetModeEvidence(mode uint32) {
	s.Mode = mode
	s.modePresent = true
}

func (s TargetDriftObservedState) HasModeEvidence() bool {
	return s.modePresent || s.Mode != 0
}

func (s TargetDriftObservedState) empty() bool {
	return s.Present == nil &&
		strings.TrimSpace(s.Kind) == "" &&
		strings.TrimSpace(s.Path) == "" &&
		!s.HasSizeEvidence() &&
		strings.TrimSpace(s.Digest) == "" &&
		!s.HasModeEvidence() &&
		strings.TrimSpace(s.ModTime) == "" &&
		strings.TrimSpace(s.SymlinkTarget) == ""
}

type SoftDelete struct {
	Version            int    `json:"version"`
	ID                 string `json:"id"`
	SessionID          string `json:"session_id,omitempty"`
	ProfileID          string `json:"profile_id,omitempty"`
	TargetID           string `json:"target_id,omitempty"`
	RootID             string `json:"root_id,omitempty"`
	PreviousSessionID  string `json:"previous_session_id,omitempty"`
	PreviousManifestID string `json:"previous_manifest_id,omitempty"`
	SourcePath         string `json:"source_path"`
	TargetPath         string `json:"target_path"`
	Kind               string `json:"kind,omitempty"`
	Size               int64  `json:"size,omitempty"`
	Digest             string `json:"digest,omitempty"`
	SymlinkTarget      string `json:"symlink_target,omitempty"`
	DetectedAt         string `json:"detected_at"`
	Reason             string `json:"reason,omitempty"`
}

type PruneDeletePolicy struct {
	Mode               string `json:"mode"`
	RequireReview      bool   `json:"require_review"`
	RetentionDays      int    `json:"retention_days,omitempty"`
	AllowPhysicalPrune bool   `json:"allow_physical_prune"`
}

type PruneApproval struct {
	Version               int                 `json:"version"`
	ID                    string              `json:"id"`
	ProfileID             string              `json:"profile_id"`
	TargetID              string              `json:"target_id"`
	RootID                string              `json:"root_id"`
	CreatedAt             string              `json:"created_at"`
	ApprovedBy            string              `json:"approved_by,omitempty"`
	ApprovedAt            string              `json:"approved_at,omitempty"`
	ReviewTool            string              `json:"review_tool"`
	ProfileSnapshotID     string              `json:"profile_snapshot_id"`
	ProfileSnapshotDigest string              `json:"profile_snapshot_digest,omitempty"`
	ProfileDeletePolicy   PruneDeletePolicy   `json:"profile_delete_policy"`
	Items                 []PruneApprovalItem `json:"items"`
	ApprovalScopeDigest   string              `json:"approval_scope_digest"`
	ExpiresAt             string              `json:"expires_at,omitempty"`
	Status                string              `json:"status"`
	ApprovalReason        string              `json:"approval_reason,omitempty"`
	RefusalReason         string              `json:"refusal_reason,omitempty"`
	SupersededBy          string              `json:"superseded_by,omitempty"`
	SupersededAt          string              `json:"superseded_at,omitempty"`
}

type PruneApprovalItem struct {
	SoftDeleteID       string `json:"soft_delete_id"`
	SoftDeleteRef      string `json:"soft_delete_ref"`
	DetectedSessionID  string `json:"detected_session_id"`
	PreviousSessionID  string `json:"previous_session_id"`
	PreviousManifestID string `json:"previous_manifest_id"`
	RootID             string `json:"root_id"`
	SourcePath         string `json:"source_path"`
	TargetPath         string `json:"target_path"`
	Kind               string `json:"kind"`
	Size               int64  `json:"size,omitempty"`
	Digest             string `json:"digest,omitempty"`
	SymlinkTarget      string `json:"symlink_target,omitempty"`
	DetectedAt         string `json:"detected_at"`
}

type PruneReceiptStatus string

const (
	PruneReceiptPlanned PruneReceiptStatus = "planned"
	PruneReceiptStarted PruneReceiptStatus = "started"
	PruneReceiptApplied PruneReceiptStatus = "applied"
	PruneReceiptPartial PruneReceiptStatus = "partial"
	PruneReceiptFailed  PruneReceiptStatus = "failed"
)

type PruneReceipt struct {
	Version             int                `json:"version"`
	ID                  string             `json:"id"`
	PruneSessionID      string             `json:"prune_session_id"`
	ApprovalID          string             `json:"approval_id"`
	ProfileID           string             `json:"profile_id"`
	TargetID            string             `json:"target_id"`
	StartedAt           string             `json:"started_at"`
	EndedAt             string             `json:"ended_at,omitempty"`
	Status              PruneReceiptStatus `json:"status"`
	DryRun              bool               `json:"dry_run"`
	ApprovalScopeDigest string             `json:"approval_scope_digest"`
	Items               []PruneReceiptItem `json:"items"`
	Refusals            []PruneRefusal     `json:"refusals,omitempty"`
}

type PruneReceiptItem struct {
	SoftDeleteID     string                   `json:"soft_delete_id"`
	TargetPath       string                   `json:"target_path"`
	IntendedAction   string                   `json:"intended_action"`
	PrePruneObserved PruneObservedTargetState `json:"pre_prune_observed,omitempty"`
	Result           string                   `json:"result"`
	ErrorCode        string                   `json:"error_code,omitempty"`
	Error            string                   `json:"error,omitempty"`
	PrunedAt         string                   `json:"pruned_at,omitempty"`
}

type PruneObservedTargetState struct {
	Present       *bool  `json:"present,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Path          string `json:"path,omitempty"`
	Size          int64  `json:"size,omitempty"`
	Digest        string `json:"digest,omitempty"`
	Mode          uint32 `json:"mode,omitempty"`
	ModTime       string `json:"mod_time,omitempty"`
	SymlinkTarget string `json:"symlink_target,omitempty"`

	sizePresent bool
	modePresent bool
}

type pruneObservedTargetStateJSON struct {
	Present       *bool   `json:"present,omitempty"`
	Kind          string  `json:"kind,omitempty"`
	Path          string  `json:"path,omitempty"`
	Size          *int64  `json:"size,omitempty"`
	Digest        string  `json:"digest,omitempty"`
	Mode          *uint32 `json:"mode,omitempty"`
	ModTime       string  `json:"mod_time,omitempty"`
	SymlinkTarget string  `json:"symlink_target,omitempty"`
}

func (s PruneObservedTargetState) MarshalJSON() ([]byte, error) {
	wire := pruneObservedTargetStateJSON{
		Present:       s.Present,
		Kind:          s.Kind,
		Path:          s.Path,
		Digest:        s.Digest,
		ModTime:       s.ModTime,
		SymlinkTarget: s.SymlinkTarget,
	}
	if s.sizePresent || s.Size != 0 {
		wire.Size = &s.Size
	}
	if s.modePresent || s.Mode != 0 {
		wire.Mode = &s.Mode
	}
	return json.Marshal(wire)
}

func (s *PruneObservedTargetState) UnmarshalJSON(data []byte) error {
	var wire pruneObservedTargetStateJSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	s.Present = wire.Present
	s.Kind = wire.Kind
	s.Path = wire.Path
	if wire.Size != nil {
		s.Size = *wire.Size
		s.sizePresent = true
	}
	s.Digest = wire.Digest
	if wire.Mode != nil {
		s.Mode = *wire.Mode
		s.modePresent = true
	}
	s.ModTime = wire.ModTime
	s.SymlinkTarget = wire.SymlinkTarget
	return nil
}

func (s *PruneObservedTargetState) SetSizeEvidence(size int64) {
	s.Size = size
	s.sizePresent = true
}

func (s PruneObservedTargetState) HasSizeEvidence() bool {
	return s.sizePresent || s.Size != 0
}

func (s *PruneObservedTargetState) SetModeEvidence(mode uint32) {
	s.Mode = mode
	s.modePresent = true
}

func (s PruneObservedTargetState) HasModeEvidence() bool {
	return s.modePresent || s.Mode != 0
}

func (s PruneObservedTargetState) empty() bool {
	return s.Present == nil &&
		strings.TrimSpace(s.Kind) == "" &&
		strings.TrimSpace(s.Path) == "" &&
		!s.HasSizeEvidence() &&
		strings.TrimSpace(s.Digest) == "" &&
		!s.HasModeEvidence() &&
		strings.TrimSpace(s.ModTime) == "" &&
		strings.TrimSpace(s.SymlinkTarget) == ""
}

type PruneRefusal struct {
	SoftDeleteID string `json:"soft_delete_id,omitempty"`
	TargetPath   string `json:"target_path,omitempty"`
	ReasonCode   string `json:"reason_code"`
	Message      string `json:"message"`
}

type HistoryIndex struct {
	Version   int            `json:"version"`
	UpdatedAt string         `json:"updated_at"`
	Sessions  []HistoryEntry `json:"sessions,omitempty"`
	Latest    string         `json:"latest,omitempty"`
}

type HistoryEntry struct {
	SessionID  string `json:"session_id"`
	StartedAt  string `json:"started_at"`
	ReceiptRef string `json:"receipt_ref"`
}

type RecoveryStatus string

const (
	RecoveryClean       RecoveryStatus = "clean"
	RecoveryInterrupted RecoveryStatus = "interrupted"
	RecoveryRepairing   RecoveryStatus = "repairing"
)

type RecoveryState struct {
	Version     int            `json:"version"`
	SessionID   string         `json:"session_id,omitempty"`
	Status      RecoveryStatus `json:"status"`
	UpdatedAt   string         `json:"updated_at"`
	Checkpoints []string       `json:"checkpoints,omitempty"`
}

type NetworkTransferStatus string

const (
	NetworkTransferStarted       NetworkTransferStatus = "started"
	NetworkTransferPublished     NetworkTransferStatus = "published"
	NetworkTransferInterrupted   NetworkTransferStatus = "interrupted"
	NetworkTransferAuthRefused   NetworkTransferStatus = "auth_refused"
	NetworkTransferNeedsRepair   NetworkTransferStatus = "needs_repair"
	NetworkTransferPublishFailed NetworkTransferStatus = "publish_failed"
	NetworkTransferFailed        NetworkTransferStatus = "failed"
)

type NetworkTransferAttempt struct {
	AttemptID string                `json:"attempt_id"`
	StartedAt string                `json:"started_at"`
	EndedAt   string                `json:"ended_at,omitempty"`
	Stage     string                `json:"stage"`
	Status    NetworkTransferStatus `json:"status"`
	ErrorCode string                `json:"error_code,omitempty"`
	Error     string                `json:"error,omitempty"`
}

type NetworkTransferPrivacyOverhead struct {
	FramePlainBytes      int64 `json:"frame_plain_bytes,omitempty"`
	FrameWireBytes       int64 `json:"frame_wire_bytes,omitempty"`
	PaddingBytes         int64 `json:"padding_bytes,omitempty"`
	PaddedChunks         int   `json:"padded_chunks,omitempty"`
	PaddingBucketBytes   int   `json:"padding_bucket_bytes,omitempty"`
	BatchFrames          int   `json:"batch_frames,omitempty"`
	BatchedChunks        int   `json:"batched_chunks,omitempty"`
	MaxBatchCount        int   `json:"max_batch_count,omitempty"`
	MaxBatchPlainBytes   int   `json:"max_batch_plain_bytes,omitempty"`
	JitteredRequests     int   `json:"jittered_requests,omitempty"`
	JitterDelayMillis    int64 `json:"jitter_delay_millis,omitempty"`
	MaxJitterDelayMillis int   `json:"max_jitter_delay_millis,omitempty"`
	JitterBudgetMillis   int   `json:"jitter_budget_millis,omitempty"`
}

func (o NetworkTransferPrivacyOverhead) Empty() bool {
	return o.FramePlainBytes == 0 &&
		o.FrameWireBytes == 0 &&
		o.PaddingBytes == 0 &&
		o.PaddedChunks == 0 &&
		o.PaddingBucketBytes == 0 &&
		o.BatchFrames == 0 &&
		o.BatchedChunks == 0 &&
		o.MaxBatchCount == 0 &&
		o.MaxBatchPlainBytes == 0 &&
		o.JitteredRequests == 0 &&
		o.JitterDelayMillis == 0 &&
		o.MaxJitterDelayMillis == 0 &&
		o.JitterBudgetMillis == 0
}

func (o NetworkTransferPrivacyOverhead) Validate() error {
	var errs []error
	if o.Empty() {
		errs = append(errs, errors.New("privacy_overhead must contain applied overhead counters"))
	}
	if o.FramePlainBytes < 0 {
		errs = append(errs, errors.New("frame_plain_bytes cannot be negative"))
	}
	if o.FrameWireBytes < 0 {
		errs = append(errs, errors.New("frame_wire_bytes cannot be negative"))
	}
	if o.PaddingBytes < 0 {
		errs = append(errs, errors.New("padding_bytes cannot be negative"))
	}
	if o.PaddedChunks < 0 {
		errs = append(errs, errors.New("padded_chunks cannot be negative"))
	}
	if o.PaddingBucketBytes < 0 {
		errs = append(errs, errors.New("padding_bucket_bytes cannot be negative"))
	}
	if o.BatchFrames < 0 {
		errs = append(errs, errors.New("batch_frames cannot be negative"))
	}
	if o.BatchedChunks < 0 {
		errs = append(errs, errors.New("batched_chunks cannot be negative"))
	}
	if o.MaxBatchCount < 0 {
		errs = append(errs, errors.New("max_batch_count cannot be negative"))
	}
	if o.MaxBatchPlainBytes < 0 {
		errs = append(errs, errors.New("max_batch_plain_bytes cannot be negative"))
	}
	if o.JitteredRequests < 0 {
		errs = append(errs, errors.New("jittered_requests cannot be negative"))
	}
	if o.JitterDelayMillis < 0 {
		errs = append(errs, errors.New("jitter_delay_millis cannot be negative"))
	}
	if o.MaxJitterDelayMillis < 0 {
		errs = append(errs, errors.New("max_jitter_delay_millis cannot be negative"))
	}
	if o.JitterBudgetMillis < 0 {
		errs = append(errs, errors.New("jitter_budget_millis cannot be negative"))
	}
	if o.FrameWireBytes != 0 && o.FramePlainBytes > o.FrameWireBytes {
		errs = append(errs, errors.New("frame_plain_bytes cannot exceed frame_wire_bytes"))
	}
	if o.PaddingBytes != 0 && o.FrameWireBytes == 0 {
		errs = append(errs, errors.New("padding_bytes requires frame_wire_bytes"))
	}
	if o.PaddedChunks != 0 && o.PaddingBucketBytes == 0 {
		errs = append(errs, errors.New("padded_chunks requires padding_bucket_bytes"))
	}
	if o.BatchedChunks != 0 && o.BatchFrames == 0 {
		errs = append(errs, errors.New("batched_chunks requires batch_frames"))
	}
	if o.MaxBatchCount != 0 && o.BatchFrames == 0 {
		errs = append(errs, errors.New("max_batch_count requires batch_frames"))
	}
	if o.MaxBatchPlainBytes != 0 && o.BatchFrames == 0 {
		errs = append(errs, errors.New("max_batch_plain_bytes requires batch_frames"))
	}
	jitterApplied := o.JitterDelayMillis != 0 || o.MaxJitterDelayMillis != 0
	jitterConfigured := o.JitteredRequests != 0 || o.JitterBudgetMillis != 0
	if jitterApplied && o.JitteredRequests == 0 {
		errs = append(errs, errors.New("jitter delay evidence requires jittered_requests"))
	}
	if jitterApplied && o.JitterBudgetMillis == 0 {
		errs = append(errs, errors.New("jitter delay evidence requires jitter_budget_millis"))
	}
	if jitterConfigured && o.JitteredRequests == 0 {
		errs = append(errs, errors.New("jitter overhead evidence requires jittered_requests"))
	}
	if jitterConfigured && o.JitterBudgetMillis == 0 {
		errs = append(errs, errors.New("jitter overhead evidence requires jitter_budget_millis"))
	}
	if o.MaxJitterDelayMillis > o.JitterBudgetMillis {
		errs = append(errs, errors.New("max_jitter_delay_millis cannot exceed jitter_budget_millis"))
	}
	if o.JitteredRequests > 0 && o.JitterBudgetMillis > 0 && o.JitterDelayMillis > int64(o.JitteredRequests)*int64(o.JitterBudgetMillis) {
		errs = append(errs, errors.New("jitter_delay_millis cannot exceed jittered_requests * jitter_budget_millis"))
	}
	return errors.Join(errs...)
}

type NetworkTransfer struct {
	Version         int                             `json:"version"`
	SessionID       string                          `json:"session_id"`
	ProfileID       string                          `json:"profile_id"`
	TargetID        string                          `json:"target_id"`
	SourceDeviceID  string                          `json:"source_device_id"`
	TargetDeviceID  string                          `json:"target_device_id"`
	ProtocolVersion string                          `json:"protocol_version"`
	PrivacyPolicy   transport.PrivacyPolicy         `json:"privacy_policy,omitempty"`
	PrivacyOverhead *NetworkTransferPrivacyOverhead `json:"privacy_overhead,omitempty"`
	Status          NetworkTransferStatus           `json:"status"`
	Stage           string                          `json:"stage"`
	StartedAt       string                          `json:"started_at"`
	UpdatedAt       string                          `json:"updated_at"`
	ErrorCode       string                          `json:"error_code,omitempty"`
	Error           string                          `json:"error,omitempty"`
	Attempts        []NetworkTransferAttempt        `json:"attempts,omitempty"`
}

type Document interface {
	Validate() error
}

func ControlDir(targetRoot string) string {
	return filepath.Join(targetRoot, DirName)
}

func EnsureControlDir(targetRoot string) error {
	return pathguard.EnsurePlainDirectory(targetRoot, ControlDir(targetRoot), 0o755)
}

func ValidateArtifactLoadBoundary(targetRoot string) error {
	info, err := os.Lstat(targetRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect target root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("target root must be a directory, not a symlink: %s", targetRoot)
	}
	if !info.IsDir() {
		return fmt.Errorf("target root must be a directory: %s", targetRoot)
	}

	controlDir := ControlDir(targetRoot)
	info, err = os.Lstat(controlDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect control directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("control directory must be a directory, not a symlink: %s", controlDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("control path must be a directory: %s", controlDir)
	}
	if err := validateSessionArtifactSurface(filepath.Join(controlDir, "sessions")); err != nil {
		return err
	}
	if err := validateProfileArtifactSurface(filepath.Join(controlDir, "profiles")); err != nil {
		return err
	}
	for _, dir := range []string{"pairings", "warnings", "deleted", "drift"} {
		if err := validateFlatArtifactSurface(filepath.Join(controlDir, dir)); err != nil {
			return err
		}
	}
	if err := validatePruneArtifactSurface(filepath.Join(controlDir, "prune")); err != nil {
		return err
	}
	if err := validateDaemonArtifactSurface(filepath.Join(controlDir, "daemon")); err != nil {
		return err
	}
	if err := validateIncrementalSyncArtifactSurface(filepath.Join(controlDir, "incremental-sync")); err != nil {
		return err
	}
	return nil
}

func validateSessionArtifactSurface(sessionsDir string) error {
	info, err := os.Lstat(sessionsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect control artifact path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("control artifact path must not be a symlink: %s", sessionsDir)
	}
	if !info.IsDir() {
		return nil
	}
	sessions, err := os.ReadDir(sessionsDir)
	if err != nil {
		return fmt.Errorf("inspect control artifact path %s: %w", sessionsDir, err)
	}
	for _, session := range sessions {
		if session.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("control artifact path must not be a symlink: %s", filepath.Join(sessionsDir, session.Name()))
		}
		if !session.IsDir() {
			continue
		}
		sessionDir := filepath.Join(sessionsDir, session.Name())
		info, err := os.Lstat(sessionDir)
		if err != nil {
			return fmt.Errorf("inspect control artifact path %s: %w", sessionDir, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("control artifact path must not be a symlink: %s", sessionDir)
		}
		for _, name := range []string{"receipt.json", "manifest.json", "session.json", "network-transfer.json"} {
			if err := validateOptionalControlArtifactFile(filepath.Join(sessionDir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFlatArtifactSurface(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect control artifact path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("control artifact path must not be a symlink: %s", root)
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("inspect control artifact path %s: %w", root, err)
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect control artifact path %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("control artifact path must not be a symlink: %s", path)
		}
	}
	return nil
}

func validateProfileArtifactSurface(root string) error {
	return validateFlatArtifactSurface(root)
}

func validatePruneArtifactSurface(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect control artifact path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("control artifact path must not be a symlink: %s", root)
	}
	if !info.IsDir() {
		return nil
	}
	for _, dir := range []string{"approvals", "receipts"} {
		if err := validateFlatArtifactSurface(filepath.Join(root, dir)); err != nil {
			return err
		}
	}
	return nil
}

func validateDaemonArtifactSurface(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect control artifact path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("control artifact path must not be a symlink: %s", root)
	}
	if !info.IsDir() {
		return nil
	}
	for _, name := range []string{"install.json", "state.json", "stop-intent.json", "restart-intent.json"} {
		if err := validateOptionalControlArtifactFile(filepath.Join(root, name)); err != nil {
			return err
		}
	}
	if err := validateFlatArtifactSurface(filepath.Join(root, "events")); err != nil {
		return err
	}
	return nil
}

func validateIncrementalSyncArtifactSurface(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect control artifact path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("control artifact path must not be a symlink: %s", root)
	}
	if !info.IsDir() {
		return nil
	}

	profilesDir := filepath.Join(root, "profiles")
	info, err = os.Lstat(profilesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect control artifact path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("control artifact path must not be a symlink: %s", profilesDir)
	}
	if !info.IsDir() {
		return nil
	}
	profiles, err := os.ReadDir(profilesDir)
	if err != nil {
		return fmt.Errorf("inspect control artifact path %s: %w", profilesDir, err)
	}
	for _, profile := range profiles {
		profileDir := filepath.Join(profilesDir, profile.Name())
		if profile.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("control artifact path must not be a symlink: %s", profileDir)
		}
		if !profile.IsDir() {
			continue
		}
		info, err := os.Lstat(profileDir)
		if err != nil {
			return fmt.Errorf("inspect control artifact path %s: %w", profileDir, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("control artifact path must not be a symlink: %s", profileDir)
		}
		targetsDir := filepath.Join(profileDir, "targets")
		info, err = os.Lstat(targetsDir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("inspect control artifact path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("control artifact path must not be a symlink: %s", targetsDir)
		}
		if !info.IsDir() {
			continue
		}
		targets, err := os.ReadDir(targetsDir)
		if err != nil {
			return fmt.Errorf("inspect control artifact path %s: %w", targetsDir, err)
		}
		for _, target := range targets {
			targetDir := filepath.Join(targetsDir, target.Name())
			if target.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("control artifact path must not be a symlink: %s", targetDir)
			}
			if !target.IsDir() {
				continue
			}
			info, err := os.Lstat(targetDir)
			if err != nil {
				return fmt.Errorf("inspect control artifact path %s: %w", targetDir, err)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("control artifact path must not be a symlink: %s", targetDir)
			}
			if err := validateOptionalControlArtifactFile(filepath.Join(targetDir, "queue.json")); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateOptionalControlArtifactFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect control artifact path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("control artifact path must not be a symlink: %s", path)
	}
	return nil
}

func Path(targetRoot string, artifact ArtifactType, id string) (string, error) {
	if strings.TrimSpace(id) == "" && artifact != ArtifactHistoryIndex && artifact != ArtifactRecoveryState {
		return "", errors.New("id is required")
	}
	if id != "" {
		if err := validateArtifactPathID(artifact, id); err != nil {
			return "", err
		}
	}

	base := ControlDir(targetRoot)
	switch artifact {
	case ArtifactProfileSnapshot:
		return filepath.Join(base, "profiles", id+".json"), nil
	case ArtifactPairingReceipt:
		return filepath.Join(base, "pairings", id+".json"), nil
	case ArtifactSessionReceipt:
		return filepath.Join(base, "sessions", id, "receipt.json"), nil
	case ArtifactManifest:
		return filepath.Join(base, "sessions", id, "manifest.json"), nil
	case ArtifactWarning:
		return filepath.Join(base, "warnings", id+".json"), nil
	case ArtifactTargetDrift:
		return filepath.Join(base, "drift", id+".json"), nil
	case ArtifactSoftDelete:
		return filepath.Join(base, "deleted", id+".json"), nil
	case ArtifactPruneApproval:
		return filepath.Join(base, "prune", "approvals", id+".json"), nil
	case ArtifactPruneReceipt:
		return filepath.Join(base, "prune", "receipts", id+".json"), nil
	case ArtifactHistoryIndex:
		return filepath.Join(base, "history", "index.json"), nil
	case ArtifactRecoveryState:
		return filepath.Join(base, "recovery", "state.json"), nil
	case ArtifactNetworkTransfer:
		return filepath.Join(base, "sessions", id, "network-transfer.json"), nil
	default:
		return "", fmt.Errorf("unknown artifact type %q", artifact)
	}
}

func validateArtifactPathID(artifact ArtifactType, id string) error {
	switch artifact {
	case ArtifactSessionReceipt, ArtifactManifest, ArtifactNetworkTransfer:
		return transaction.ValidateSessionID(id)
	default:
		return ValidateArtifactID(id)
	}
}

// ValidateArtifactID validates ids used as filenames under the target control plane.
func ValidateArtifactID(id string) error {
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._-:", r) {
			continue
		}
		return fmt.Errorf("id %q contains unsafe character %q", id, r)
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("id %q contains unsafe path segment", id)
	}
	return nil
}

func Read[T Document](r io.Reader) (T, error) {
	var doc T
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return doc, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return doc, err
	}
	if err := doc.Validate(); err != nil {
		return doc, err
	}
	return doc, nil
}

func ReadFile[T Document](path string) (T, error) {
	file, err := os.Open(path)
	if err != nil {
		var zero T
		return zero, err
	}
	defer file.Close()
	return Read[T](file)
}

func ReadManifestCompatFile(path string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()
	return ReadManifestCompat(file)
}

func ReadManifestCompat(r io.Reader) (Manifest, error) {
	var doc Manifest
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return doc, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return doc, err
	}
	if err := doc.validateWithOptions(manifestValidationOptions{allowLegacySymlinkTarget: true}); err != nil {
		return doc, err
	}
	return doc, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON document")
		}
		return err
	}
	return nil
}

func Write(w io.Writer, doc Document) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

func WriteFile(path string, doc Document) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	return writeFile(path, doc, true)
}

func WriteNewFile(path string, doc Document) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	return writeFile(path, doc, false)
}

func writeFile(path string, doc Document, replace bool) error {
	controlDir := enclosingControlDir(path)
	if err := pathguard.EnsurePlainDirectory(filepath.Dir(controlDir), filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if !replace {
		if info, err := os.Lstat(path); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("control artifact %q exists as a symlink", path)
			}
			return fmt.Errorf("%w: %q", ErrArtifactExists, path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".control-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := Write(temp, doc); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if replace {
		if err := os.Rename(tempName, path); err != nil {
			return err
		}
	} else {
		if err := os.Link(tempName, path); err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("%w: %q", ErrArtifactExists, path)
			}
			return err
		}
	}
	if err := durable.SyncDirBestEffort(filepath.Dir(path)); err != nil {
		return err
	}
	return nil
}

func enclosingControlDir(path string) string {
	current := filepath.Clean(path)
	for {
		if filepath.Base(current) == DirName {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Dir(path)
		}
		current = parent
	}
}

func (d ProfileSnapshot) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	require("profile_id", d.ProfileID, &errs)
	validateSessionIDField("session_id", d.SessionID, &errs)
	require("captured_at", d.CapturedAt, &errs)
	if len(d.Profile) == 0 || !json.Valid(d.Profile) {
		errs = append(errs, errors.New("profile must contain valid JSON"))
	}
	return errors.Join(errs...)
}

func (d PairingReceipt) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	if strings.TrimSpace(d.ID) != "" {
		if err := ValidateArtifactID(d.ID); err != nil {
			errs = append(errs, fmt.Errorf("id is unsafe: %w", err))
		}
	}
	require("profile_id", d.ProfileID, &errs)
	require("target_id", d.TargetID, &errs)
	require("source_device_id", d.SourceDeviceID, &errs)
	require("target_device_id", d.TargetDeviceID, &errs)
	require("device_public_key", d.DevicePublicKey, &errs)
	require("method", d.Method, &errs)
	require("verified_at", d.VerifiedAt, &errs)
	require("protocol_version", d.ProtocolVersion, &errs)
	if d.VerificationPhrase == "" && d.VerificationHash == "" {
		errs = append(errs, errors.New("verification phrase or hash is required"))
	}
	verifiedAt, err := time.Parse(time.RFC3339Nano, d.VerifiedAt)
	if strings.TrimSpace(d.VerifiedAt) != "" && err != nil {
		errs = append(errs, fmt.Errorf("verified_at must be RFC3339 timestamp: %w", err))
	}
	if d.DevicePublicKey != "" && d.TargetDeviceID != "" && d.DevicePublicKey != d.TargetDeviceID {
		errs = append(errs, errors.New("device_public_key must match target_device_id"))
	}
	if verifiedAt.IsZero() && err == nil {
		errs = append(errs, errors.New("verified_at is required"))
	}
	if pairingReceiptReadyForTransportValidation(d, verifiedAt, err) {
		transportReceipt := transport.PairingReceipt{
			SourceDeviceID:     transport.DeviceID(d.SourceDeviceID),
			TargetDeviceID:     transport.DeviceID(d.TargetDeviceID),
			ProfileID:          d.ProfileID,
			Method:             transport.PairingMethod(d.Method),
			VerifiedAt:         verifiedAt,
			VerificationPhrase: d.VerificationPhrase,
			VerificationHash:   d.VerificationHash,
			ProtocolVersion:    d.ProtocolVersion,
		}
		if err := transportReceipt.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func pairingReceiptReadyForTransportValidation(d PairingReceipt, verifiedAt time.Time, parseErr error) bool {
	if parseErr != nil || verifiedAt.IsZero() {
		return false
	}
	if strings.TrimSpace(d.ProfileID) == "" ||
		strings.TrimSpace(d.SourceDeviceID) == "" ||
		strings.TrimSpace(d.TargetDeviceID) == "" ||
		strings.TrimSpace(d.DevicePublicKey) == "" ||
		strings.TrimSpace(d.Method) == "" ||
		strings.TrimSpace(d.ProtocolVersion) == "" {
		return false
	}
	if d.VerificationPhrase == "" && d.VerificationHash == "" {
		return false
	}
	if d.DevicePublicKey != d.TargetDeviceID {
		return false
	}
	return true
}

func (d SessionReceipt) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	validateSessionIDField("id", d.ID, &errs)
	require("profile_id", d.ProfileID, &errs)
	require("target_id", d.TargetID, &errs)
	require("started_at", d.StartedAt, &errs)
	require("status", d.Status, &errs)
	return errors.Join(errs...)
}

func (d Manifest) Validate() error {
	return d.validateWithOptions(manifestValidationOptions{})
}

type manifestValidationOptions struct {
	allowLegacySymlinkTarget bool
}

func (d Manifest) validateWithOptions(opts manifestValidationOptions) error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	require("session_id", d.SessionID, &errs)
	validateSessionIDField("session_id", d.SessionID, &errs)
	require("created_at", d.CreatedAt, &errs)
	if strings.TrimSpace(d.CreatedAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, d.CreatedAt); err != nil {
			errs = append(errs, fmt.Errorf("created_at must be RFC3339 timestamp: %w", err))
		}
	}
	for i, entry := range d.Entries {
		if strings.TrimSpace(entry.Path) == "" {
			errs = append(errs, fmt.Errorf("entries[%d].path is required", i))
		}
		if strings.TrimSpace(entry.Kind) == "" {
			errs = append(errs, fmt.Errorf("entries[%d].kind is required", i))
		}
		if entry.Kind == "symlink" && strings.TrimSpace(entry.SymlinkTarget) == "" && !opts.allowLegacySymlinkTarget {
			errs = append(errs, fmt.Errorf("entries[%d].symlink_target is required for symlinks", i))
		}
		if entry.Kind == "symlink" && strings.TrimSpace(entry.SymlinkTarget) != "" {
			if err := pathguard.ValidateRelativeSymlinkTarget(entry.SymlinkTarget); err != nil {
				errs = append(errs, fmt.Errorf("entries[%d].symlink_target is unsafe: %w", i, err))
			}
		}
		if entry.Size < 0 {
			errs = append(errs, fmt.Errorf("entries[%d].size cannot be negative", i))
		}
		if entry.PreviousSize < 0 {
			errs = append(errs, fmt.Errorf("entries[%d].previous_size cannot be negative", i))
		}
		previousCoreFields := []string{
			entry.PreviousSessionID,
			entry.PreviousManifestID,
			entry.PreviousDigest,
		}
		previousPresent := 0
		for _, value := range previousCoreFields {
			if strings.TrimSpace(value) != "" {
				previousPresent++
			}
		}
		if previousPresent > 0 && previousPresent != len(previousCoreFields) {
			errs = append(errs, fmt.Errorf("entries[%d].previous evidence must include session_id, manifest_id, and digest together", i))
		}
		if previousPresent > 0 && entry.Kind != "file" {
			errs = append(errs, fmt.Errorf("entries[%d].previous evidence is only valid for file entries", i))
		}
		if previousPresent > 0 && !entry.HasPreviousSizeEvidence() {
			errs = append(errs, fmt.Errorf("entries[%d].previous evidence must include previous_size", i))
		}
		if strings.TrimSpace(entry.PreviousSessionID) != "" {
			if err := transaction.ValidateSessionID(entry.PreviousSessionID); err != nil {
				errs = append(errs, fmt.Errorf("entries[%d].previous_session_id is invalid: %w", i, err))
			}
		}
		if previousPresent > 0 && !entry.HasPreviousModeEvidence() {
			errs = append(errs, fmt.Errorf("entries[%d].previous evidence must include previous_mode", i))
		}
		if previousPresent > 0 && strings.TrimSpace(entry.PreviousModTime) == "" {
			errs = append(errs, fmt.Errorf("entries[%d].previous evidence must include previous_mod_time", i))
		}
		if strings.TrimSpace(entry.PreviousModTime) != "" {
			if _, err := time.Parse(time.RFC3339Nano, entry.PreviousModTime); err != nil {
				errs = append(errs, fmt.Errorf("entries[%d].previous_mod_time must be RFC3339 timestamp: %w", i, err))
			}
		}
		if previousPresent == 0 && (entry.HasPreviousSizeEvidence() || entry.HasPreviousModeEvidence() || strings.TrimSpace(entry.PreviousModTime) != "") {
			errs = append(errs, fmt.Errorf("entries[%d].previous metadata requires previous evidence", i))
		}
		if strings.TrimSpace(entry.PreviousDigest) != "" && !isSHA256Digest(entry.PreviousDigest) {
			errs = append(errs, fmt.Errorf("entries[%d].previous_digest must be sha256 hex", i))
		}
	}
	return errors.Join(errs...)
}

func isSHA256Digest(value string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	hexPart := strings.TrimPrefix(value, prefix)
	if len(hexPart) != 64 {
		return false
	}
	for _, r := range hexPart {
		if ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F') {
			continue
		}
		return false
	}
	return true
}

func (d Warning) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	require("session_id", d.SessionID, &errs)
	validateSessionIDField("session_id", d.SessionID, &errs)
	require("code", d.Code, &errs)
	require("message", d.Message, &errs)
	require("severity", d.Severity, &errs)
	if len(d.Paths) == 0 {
		errs = append(errs, errors.New("paths must contain at least one path"))
	}
	require("created_at", d.CreatedAt, &errs)
	return errors.Join(errs...)
}

func (d TargetDrift) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	if strings.TrimSpace(d.ID) != "" {
		if err := ValidateArtifactID(d.ID); err != nil {
			errs = append(errs, fmt.Errorf("id is unsafe: %w", err))
		}
	}
	require("session_id", d.SessionID, &errs)
	validateSessionIDField("session_id", d.SessionID, &errs)
	require("profile_id", d.ProfileID, &errs)
	require("target_id", d.TargetID, &errs)
	require("root_id", d.RootID, &errs)
	requireSafeControlRelativePath("path", d.Path, &errs)
	require("detected_at", d.DetectedAt, &errs)
	if strings.TrimSpace(d.DetectedAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, d.DetectedAt); err != nil {
			errs = append(errs, fmt.Errorf("detected_at must be RFC3339 timestamp: %w", err))
		}
	}
	if strings.TrimSpace(d.LastDetectedAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, d.LastDetectedAt); err != nil {
			errs = append(errs, fmt.Errorf("last_detected_at must be RFC3339 timestamp: %w", err))
		}
	}
	require("change", d.Change, &errs)
	if strings.TrimSpace(d.ReviewState) != "" && !targetDriftReviewStateValid(d.ReviewState) {
		errs = append(errs, fmt.Errorf("review_state must be one of needs_review, acknowledged, resolved"))
	}
	if strings.TrimSpace(d.ReviewedAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, d.ReviewedAt); err != nil {
			errs = append(errs, fmt.Errorf("reviewed_at must be RFC3339 timestamp: %w", err))
		}
	}
	if strings.TrimSpace(d.ReviewAction) != "" && !targetDriftReviewActionValid(d.ReviewAction) {
		errs = append(errs, fmt.Errorf("review_action must be one of acknowledge, resolve"))
	}
	validateTargetDriftReviewEvidence(d, &errs)
	for i, event := range d.ReviewHistory {
		validateTargetDriftReviewEvent(i, event, &errs)
	}
	validateTargetDriftExpected(d.Expected, &errs)
	validateTargetDriftObserved(d.Observed, &errs)
	return errors.Join(errs...)
}

func validateTargetDriftReviewEvidence(d TargetDrift, errs *[]error) {
	reviewAction := strings.TrimSpace(d.ReviewAction)
	reviewState := strings.TrimSpace(d.ReviewState)
	reviewedAt := strings.TrimSpace(d.ReviewedAt)
	reviewedBy := strings.TrimSpace(d.ReviewedBy)
	reviewReason := strings.TrimSpace(d.ReviewReason)
	if reviewAction == "" && reviewedAt == "" && reviewedBy == "" && reviewReason == "" {
		if reviewState == "resolved" {
			*errs = append(*errs, errors.New(`review_action "resolve" is required when review_state is "resolved"`))
		}
		return
	}
	if reviewAction == "" {
		*errs = append(*errs, errors.New("review_action is required when review evidence is present"))
		return
	}
	if reviewedAt == "" {
		*errs = append(*errs, errors.New("reviewed_at is required when review_action is present"))
	}
	if reviewReason == "" {
		*errs = append(*errs, errors.New("review_reason is required when review_action is present"))
	}
	switch reviewAction {
	case "acknowledge":
		if reviewState != "acknowledged" {
			*errs = append(*errs, errors.New(`review_state must be "acknowledged" when review_action is "acknowledge"`))
		}
	case "resolve":
		if reviewState != "resolved" {
			*errs = append(*errs, errors.New(`review_state must be "resolved" when review_action is "resolve"`))
		}
	}
}

func validateTargetDriftReviewEvent(index int, event TargetDriftReviewEvent, errs *[]error) {
	prefix := fmt.Sprintf("review_history[%d]", index)
	reviewState := strings.TrimSpace(event.ReviewState)
	if reviewState == "" {
		*errs = append(*errs, fmt.Errorf("%s.review_state is required", prefix))
	} else if !targetDriftReviewStateValid(reviewState) {
		*errs = append(*errs, fmt.Errorf("%s.review_state must be one of needs_review, acknowledged, resolved", prefix))
	}
	if strings.TrimSpace(event.ReviewAction) != "" && !targetDriftReviewActionValid(event.ReviewAction) {
		*errs = append(*errs, fmt.Errorf("%s.review_action must be one of acknowledge, resolve", prefix))
	}
	if strings.TrimSpace(event.ReviewedAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, event.ReviewedAt); err != nil {
			*errs = append(*errs, fmt.Errorf("%s.reviewed_at must be RFC3339 timestamp: %w", prefix, err))
		}
	}
	require(prefix+".reconciled_at", event.ReconciledAt, errs)
	if strings.TrimSpace(event.ReconciledAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, event.ReconciledAt); err != nil {
			*errs = append(*errs, fmt.Errorf("%s.reconciled_at must be RFC3339 timestamp: %w", prefix, err))
		}
	}
	require(prefix+".reconcile_action", event.ReconcileAction, errs)
	if action := strings.TrimSpace(event.ReconcileAction); action != "" && action != "reopen" {
		*errs = append(*errs, fmt.Errorf("%s.reconcile_action must be reopen", prefix))
	}
}

func validateTargetDriftExpected(state TargetDriftExpectedState, errs *[]error) {
	if state.empty() {
		return
	}
	require("expected.session_id", state.SessionID, errs)
	validateSessionIDField("expected.session_id", state.SessionID, errs)
	require("expected.manifest_id", state.ManifestID, errs)
	require("expected.kind", state.Kind, errs)
	if strings.TrimSpace(state.Kind) != "" && !targetDriftKindValid(state.Kind) {
		*errs = append(*errs, fmt.Errorf("expected.kind must be one of file, dir, symlink, special, missing, other"))
	}
	requireSafeControlRelativePath("expected.path", state.Path, errs)
	if state.HasSizeEvidence() && state.Size < 0 {
		*errs = append(*errs, errors.New("expected.size cannot be negative"))
	}
	if strings.TrimSpace(state.Digest) != "" && !isSHA256Digest(state.Digest) {
		*errs = append(*errs, errors.New("expected.digest must be sha256 hex"))
	}
	if strings.TrimSpace(state.ModTime) != "" {
		if _, err := time.Parse(time.RFC3339Nano, state.ModTime); err != nil {
			*errs = append(*errs, fmt.Errorf("expected.mod_time must be RFC3339 timestamp: %w", err))
		}
	}
	if strings.TrimSpace(state.Digest) != "" && state.Kind != "file" {
		*errs = append(*errs, errors.New("expected.digest is only valid for file entries"))
	}
	if strings.TrimSpace(state.SymlinkTarget) != "" {
		if err := pathguard.ValidateRelativeSymlinkTarget(state.SymlinkTarget); err != nil {
			*errs = append(*errs, fmt.Errorf("expected.symlink_target is unsafe: %w", err))
		}
		if state.Kind != "symlink" {
			*errs = append(*errs, errors.New("expected.symlink_target is only valid for symlink entries"))
		}
	}
	if (state.HasSizeEvidence() || state.HasModeEvidence() || strings.TrimSpace(state.Digest) != "" || strings.TrimSpace(state.ModTime) != "" || strings.TrimSpace(state.SymlinkTarget) != "") && state.Kind == "missing" {
		*errs = append(*errs, errors.New("expected missing state cannot include file metadata"))
	}
}

func validateTargetDriftObserved(state TargetDriftObservedState, errs *[]error) {
	if state.empty() {
		return
	}
	if state.Present == nil {
		*errs = append(*errs, errors.New("observed.present is required"))
	}
	require("observed.kind", state.Kind, errs)
	if strings.TrimSpace(state.Kind) != "" && !targetDriftKindValid(state.Kind) {
		*errs = append(*errs, fmt.Errorf("observed.kind must be one of file, dir, symlink, special, missing, other"))
	}
	requireSafeControlRelativePath("observed.path", state.Path, errs)
	if state.HasSizeEvidence() && state.Size < 0 {
		*errs = append(*errs, errors.New("observed.size cannot be negative"))
	}
	if strings.TrimSpace(state.Digest) != "" && !isSHA256Digest(state.Digest) {
		*errs = append(*errs, errors.New("observed.digest must be sha256 hex"))
	}
	if strings.TrimSpace(state.ModTime) != "" {
		if _, err := time.Parse(time.RFC3339Nano, state.ModTime); err != nil {
			*errs = append(*errs, fmt.Errorf("observed.mod_time must be RFC3339 timestamp: %w", err))
		}
	}
	if strings.TrimSpace(state.Digest) != "" && state.Kind != "file" {
		*errs = append(*errs, errors.New("observed.digest is only valid for file entries"))
	}
	if strings.TrimSpace(state.SymlinkTarget) != "" {
		if err := pathguard.ValidateRelativeSymlinkTarget(state.SymlinkTarget); err != nil {
			*errs = append(*errs, fmt.Errorf("observed.symlink_target is unsafe: %w", err))
		}
	}
	if state.Present != nil && !*state.Present && state.Kind != "missing" {
		*errs = append(*errs, errors.New("observed.kind must be missing when observed.present is false"))
	}
	if state.Kind == "missing" && (state.HasSizeEvidence() || state.HasModeEvidence() || strings.TrimSpace(state.Digest) != "" || strings.TrimSpace(state.ModTime) != "" || strings.TrimSpace(state.SymlinkTarget) != "") {
		*errs = append(*errs, errors.New("observed missing state cannot include file metadata"))
	}
}

func (d SoftDelete) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	if strings.TrimSpace(d.ID) != "" {
		if err := ValidateArtifactID(d.ID); err != nil {
			errs = append(errs, fmt.Errorf("id is unsafe: %w", err))
		}
	}
	require("session_id", d.SessionID, &errs)
	validateSessionIDField("session_id", d.SessionID, &errs)
	require("profile_id", d.ProfileID, &errs)
	require("target_id", d.TargetID, &errs)
	require("root_id", d.RootID, &errs)
	require("previous_session_id", d.PreviousSessionID, &errs)
	validateSessionIDField("previous_session_id", d.PreviousSessionID, &errs)
	require("previous_manifest_id", d.PreviousManifestID, &errs)
	requireSafeControlRelativePath("source_path", d.SourcePath, &errs)
	requireSafeControlRelativePath("target_path", d.TargetPath, &errs)
	require("kind", d.Kind, &errs)
	if strings.TrimSpace(d.Kind) != "" && !pruneTargetKindValid(d.Kind) {
		errs = append(errs, fmt.Errorf("kind must be one of file, dir, symlink, special, other"))
	}
	if d.Size < 0 {
		errs = append(errs, errors.New("size cannot be negative"))
	}
	if strings.TrimSpace(d.Digest) != "" && !isSHA256Digest(d.Digest) {
		errs = append(errs, errors.New("digest must be sha256 hex"))
	}
	if strings.TrimSpace(d.Digest) != "" && d.Kind != "file" {
		errs = append(errs, errors.New("digest is only valid for file soft deletes"))
	}
	if strings.TrimSpace(d.SymlinkTarget) != "" {
		if err := pathguard.ValidateRelativeSymlinkTarget(d.SymlinkTarget); err != nil {
			errs = append(errs, fmt.Errorf("symlink_target is unsafe: %w", err))
		}
	}
	if strings.TrimSpace(d.SymlinkTarget) != "" && d.Kind != "symlink" {
		errs = append(errs, errors.New("symlink_target is only valid for symlink soft deletes"))
	}
	require("detected_at", d.DetectedAt, &errs)
	if strings.TrimSpace(d.DetectedAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, d.DetectedAt); err != nil {
			errs = append(errs, fmt.Errorf("detected_at must be RFC3339 timestamp: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (d PruneApproval) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	validateArtifactIDField("id", d.ID, &errs)
	require("profile_id", d.ProfileID, &errs)
	require("target_id", d.TargetID, &errs)
	require("root_id", d.RootID, &errs)
	createdAt, createdAtOK := requireRFC3339("created_at", d.CreatedAt, &errs)
	var approvedAt time.Time
	var approvedAtOK bool
	if d.Status == "approved" {
		require("approved_by", d.ApprovedBy, &errs)
		approvedAt, approvedAtOK = requireRFC3339("approved_at", d.ApprovedAt, &errs)
	} else {
		approvedAt, approvedAtOK = parseOptionalRFC3339("approved_at", d.ApprovedAt, &errs)
	}
	if createdAtOK && approvedAtOK && approvedAt.Before(createdAt) {
		errs = append(errs, errors.New("approved_at must be greater than or equal to created_at"))
	}
	require("review_tool", d.ReviewTool, &errs)
	if d.Status == "approved" {
		require("profile_snapshot_id", d.ProfileSnapshotID, &errs)
		requireDigest("profile_snapshot_digest", d.ProfileSnapshotDigest, &errs)
	}
	validateArtifactIDField("profile_snapshot_id", d.ProfileSnapshotID, &errs)
	if strings.TrimSpace(d.ExpiresAt) != "" {
		expiresAt, expiresAtOK := parseOptionalRFC3339("expires_at", d.ExpiresAt, &errs)
		if approvedAtOK && expiresAtOK && expiresAt.Before(approvedAt) {
			errs = append(errs, errors.New("expires_at must be greater than or equal to approved_at"))
		}
		if createdAtOK && expiresAtOK && expiresAt.Before(createdAt) {
			errs = append(errs, errors.New("expires_at must be greater than or equal to created_at"))
		}
	}
	require("status", d.Status, &errs)
	if strings.TrimSpace(d.Status) != "" && !pruneApprovalStatusValid(d.Status) {
		errs = append(errs, errors.New("status must be one of approved, refused, superseded"))
	}
	var supersededAt time.Time
	var supersededAtOK bool
	if d.Status == "superseded" {
		require("superseded_by", d.SupersededBy, &errs)
		supersededAt, supersededAtOK = requireRFC3339("superseded_at", d.SupersededAt, &errs)
	} else {
		supersededAt, supersededAtOK = parseOptionalRFC3339("superseded_at", d.SupersededAt, &errs)
		if strings.TrimSpace(d.SupersededBy) != "" {
			errs = append(errs, errors.New("superseded_by is only valid when status is superseded"))
		}
		if strings.TrimSpace(d.SupersededAt) != "" {
			errs = append(errs, errors.New("superseded_at is only valid when status is superseded"))
		}
	}
	if approvedAtOK && supersededAtOK && supersededAt.Before(approvedAt) {
		errs = append(errs, errors.New("superseded_at must be greater than or equal to approved_at"))
	}
	if createdAtOK && supersededAtOK && supersededAt.Before(createdAt) {
		errs = append(errs, errors.New("superseded_at must be greater than or equal to created_at"))
	}
	if d.Status == "approved" && strings.TrimSpace(d.RefusalReason) != "" {
		errs = append(errs, errors.New("refusal_reason must be empty when status is approved"))
	}
	if d.Status != "" && d.Status != "approved" && strings.TrimSpace(d.RefusalReason) == "" {
		errs = append(errs, errors.New("refusal_reason is required unless status is approved"))
	}
	if d.Status == "approved" {
		validatePruneDeletePolicy(d.ProfileDeletePolicy, &errs)
		if len(d.Items) == 0 {
			errs = append(errs, errors.New("items must contain at least one approved soft delete"))
		}
		requireDigest("approval_scope_digest", d.ApprovalScopeDigest, &errs)
	} else if strings.TrimSpace(d.ApprovalScopeDigest) != "" && !isSHA256Digest(d.ApprovalScopeDigest) {
		errs = append(errs, errors.New("approval_scope_digest must be sha256 hex"))
	}
	for i, item := range d.Items {
		validatePruneApprovalItem(i, item, &errs)
	}
	return errors.Join(errs...)
}

func validatePruneApprovalItem(index int, item PruneApprovalItem, errs *[]error) {
	prefix := fmt.Sprintf("items[%d]", index)
	require(prefix+".soft_delete_id", item.SoftDeleteID, errs)
	validateArtifactIDField(prefix+".soft_delete_id", item.SoftDeleteID, errs)
	require(prefix+".soft_delete_ref", item.SoftDeleteRef, errs)
	if strings.TrimSpace(item.SoftDeleteID) != "" && strings.TrimSpace(item.SoftDeleteRef) != "" {
		want := "deleted/" + item.SoftDeleteID + ".json"
		if item.SoftDeleteRef != want {
			*errs = append(*errs, fmt.Errorf("%s.soft_delete_ref must match soft_delete_id", prefix))
		}
	}
	require(prefix+".detected_session_id", item.DetectedSessionID, errs)
	validateSessionIDField(prefix+".detected_session_id", item.DetectedSessionID, errs)
	require(prefix+".previous_session_id", item.PreviousSessionID, errs)
	validateSessionIDField(prefix+".previous_session_id", item.PreviousSessionID, errs)
	require(prefix+".previous_manifest_id", item.PreviousManifestID, errs)
	require(prefix+".root_id", item.RootID, errs)
	requireSafeControlRelativePath(prefix+".source_path", item.SourcePath, errs)
	requireSafeControlRelativePath(prefix+".target_path", item.TargetPath, errs)
	require(prefix+".kind", item.Kind, errs)
	if strings.TrimSpace(item.Kind) != "" && !pruneTargetKindValid(item.Kind) {
		*errs = append(*errs, fmt.Errorf("%s.kind must be one of file, dir, symlink, special, other", prefix))
	}
	if item.Size < 0 {
		*errs = append(*errs, fmt.Errorf("%s.size cannot be negative", prefix))
	}
	if strings.TrimSpace(item.Digest) != "" && !isSHA256Digest(item.Digest) {
		*errs = append(*errs, fmt.Errorf("%s.digest must be sha256 hex", prefix))
	}
	if strings.TrimSpace(item.Digest) != "" && item.Kind != "file" {
		*errs = append(*errs, fmt.Errorf("%s.digest is only valid for file approvals", prefix))
	}
	if strings.TrimSpace(item.SymlinkTarget) != "" {
		if err := pathguard.ValidateRelativeSymlinkTarget(item.SymlinkTarget); err != nil {
			*errs = append(*errs, fmt.Errorf("%s.symlink_target is unsafe: %w", prefix, err))
		}
	}
	if item.Kind == "symlink" && strings.TrimSpace(item.SymlinkTarget) == "" {
		*errs = append(*errs, fmt.Errorf("%s.symlink_target is required for symlink approvals", prefix))
	}
	if strings.TrimSpace(item.SymlinkTarget) != "" && item.Kind != "symlink" {
		*errs = append(*errs, fmt.Errorf("%s.symlink_target is only valid for symlink approvals", prefix))
	}
	requireRFC3339(prefix+".detected_at", item.DetectedAt, errs)
}

func validatePruneDeletePolicy(policy PruneDeletePolicy, errs *[]error) {
	require("profile_delete_policy.mode", policy.Mode, errs)
	if strings.TrimSpace(policy.Mode) != "" && policy.Mode != "prune" {
		*errs = append(*errs, errors.New("profile_delete_policy.mode must be prune"))
	}
	if !policy.RequireReview {
		*errs = append(*errs, errors.New("profile_delete_policy.require_review must be true"))
	}
	if !policy.AllowPhysicalPrune {
		*errs = append(*errs, errors.New("profile_delete_policy.allow_physical_prune must be true"))
	}
	if policy.RetentionDays < 0 {
		*errs = append(*errs, errors.New("profile_delete_policy.retention_days cannot be negative"))
	}
}

func (s PruneReceiptStatus) Valid() bool {
	switch s {
	case PruneReceiptPlanned, PruneReceiptStarted, PruneReceiptApplied, PruneReceiptPartial, PruneReceiptFailed:
		return true
	default:
		return false
	}
}

func (d PruneReceipt) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	validateArtifactIDField("id", d.ID, &errs)
	require("prune_session_id", d.PruneSessionID, &errs)
	validateArtifactIDField("prune_session_id", d.PruneSessionID, &errs)
	require("approval_id", d.ApprovalID, &errs)
	validateArtifactIDField("approval_id", d.ApprovalID, &errs)
	require("profile_id", d.ProfileID, &errs)
	require("target_id", d.TargetID, &errs)
	startedAt, startedOK := requireRFC3339("started_at", d.StartedAt, &errs)
	endedAt, endedOK := parseOptionalRFC3339("ended_at", d.EndedAt, &errs)
	if startedOK && endedOK && endedAt.Before(startedAt) {
		errs = append(errs, errors.New("ended_at must be greater than or equal to started_at"))
	}
	require("status", string(d.Status), &errs)
	if strings.TrimSpace(string(d.Status)) != "" && !d.Status.Valid() {
		errs = append(errs, errors.New("status must be one of planned, started, applied, partial, failed"))
	}
	if d.DryRun && d.Status != PruneReceiptPlanned {
		errs = append(errs, errors.New("dry_run receipts must use status planned"))
	}
	if !d.DryRun && d.Status == PruneReceiptPlanned {
		errs = append(errs, errors.New("apply receipts must not use status planned"))
	}
	if !d.DryRun && d.Status != PruneReceiptPlanned && d.Status != PruneReceiptStarted && strings.TrimSpace(d.EndedAt) == "" {
		errs = append(errs, errors.New("ended_at is required for apply receipts"))
	}
	requireDigest("approval_scope_digest", d.ApprovalScopeDigest, &errs)
	if len(d.Items) == 0 {
		errs = append(errs, errors.New("items must contain at least one prune result"))
	}
	for i, item := range d.Items {
		validatePruneReceiptItem(i, item, d, &errs)
	}
	validatePruneReceiptOutcome(d, &errs)
	for i, refusal := range d.Refusals {
		validatePruneRefusal(i, refusal, &errs)
	}
	return errors.Join(errs...)
}

func validatePruneReceiptItem(index int, item PruneReceiptItem, d PruneReceipt, errs *[]error) {
	prefix := fmt.Sprintf("items[%d]", index)
	require(prefix+".soft_delete_id", item.SoftDeleteID, errs)
	validateArtifactIDField(prefix+".soft_delete_id", item.SoftDeleteID, errs)
	requireSafeControlRelativePath(prefix+".target_path", item.TargetPath, errs)
	require(prefix+".intended_action", item.IntendedAction, errs)
	if strings.TrimSpace(item.IntendedAction) != "" && !pruneReceiptIntendedActionValid(item.IntendedAction) {
		*errs = append(*errs, fmt.Errorf("%s.intended_action must be one of delete_file, delete_symlink", prefix))
	}
	validatePruneObservedTargetState(prefix+".pre_prune_observed", item.PrePruneObserved, errs)
	require(prefix+".result", item.Result, errs)
	if strings.TrimSpace(item.Result) != "" && !pruneReceiptItemResultValid(item.Result) {
		*errs = append(*errs, fmt.Errorf("%s.result must be one of would_prune, pruned, skipped, refused, failed", prefix))
	}
	if d.DryRun && item.Result != "" && item.Result != "would_prune" && item.Result != "refused" && item.Result != "skipped" {
		*errs = append(*errs, fmt.Errorf("%s.result is invalid for dry_run receipt", prefix))
	}
	if !d.DryRun && d.Status != PruneReceiptStarted && item.Result == "would_prune" {
		*errs = append(*errs, fmt.Errorf("%s.result would_prune is only valid for dry_run or started apply receipt", prefix))
	}
	if strings.TrimSpace(item.ErrorCode) != "" {
		validateArtifactIDField(prefix+".error_code", item.ErrorCode, errs)
	}
	if (item.Result == "refused" || item.Result == "failed") && strings.TrimSpace(item.ErrorCode) == "" {
		*errs = append(*errs, fmt.Errorf("%s.error_code is required for refused or failed result", prefix))
	}
	if strings.TrimSpace(item.PrunedAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, item.PrunedAt); err != nil {
			*errs = append(*errs, fmt.Errorf("%s.pruned_at must be RFC3339 timestamp: %w", prefix, err))
		}
	}
	if d.DryRun && strings.TrimSpace(item.PrunedAt) != "" {
		*errs = append(*errs, fmt.Errorf("%s.pruned_at must be empty for dry_run receipt", prefix))
	}
	if !d.DryRun && item.Result != "pruned" && strings.TrimSpace(item.PrunedAt) != "" {
		*errs = append(*errs, fmt.Errorf("%s.pruned_at is only valid when result is pruned", prefix))
	}
	if item.Result == "pruned" && strings.TrimSpace(item.PrunedAt) == "" {
		*errs = append(*errs, fmt.Errorf("%s.pruned_at is required when result is pruned", prefix))
	}
	if item.Result == "pruned" && item.PrePruneObserved.empty() {
		*errs = append(*errs, fmt.Errorf("%s.pre_prune_observed is required when result is pruned", prefix))
	}
	if item.Result == "pruned" && item.PrePruneObserved.Present != nil && !*item.PrePruneObserved.Present {
		*errs = append(*errs, fmt.Errorf("%s.pre_prune_observed.present must be true when result is pruned", prefix))
	}
}

func validatePruneReceiptOutcome(d PruneReceipt, errs *[]error) {
	if len(d.Items) == 0 || strings.TrimSpace(string(d.Status)) == "" || !d.Status.Valid() {
		return
	}
	counts := map[string]int{}
	for _, item := range d.Items {
		counts[item.Result]++
	}
	if d.DryRun {
		if d.Status != PruneReceiptPlanned {
			return
		}
		if counts["pruned"] > 0 || counts["failed"] > 0 {
			*errs = append(*errs, errors.New("dry_run receipts cannot contain pruned or failed results"))
		}
		return
	}
	switch d.Status {
	case PruneReceiptStarted:
		if d.DryRun {
			return
		}
		if strings.TrimSpace(d.EndedAt) != "" {
			*errs = append(*errs, errors.New("status started must not include ended_at"))
		}
		if counts["pruned"] > 0 || counts["failed"] > 0 {
			*errs = append(*errs, errors.New("status started cannot contain pruned or failed results"))
		}
		for _, item := range d.Items {
			if item.Result != "would_prune" && item.Result != "refused" && item.Result != "skipped" {
				*errs = append(*errs, errors.New("status started items must be would_prune, refused, or skipped"))
				break
			}
		}
	case PruneReceiptApplied:
		if counts["failed"] > 0 || counts["refused"] > 0 || counts["pruned"] == 0 {
			*errs = append(*errs, errors.New("status applied requires at least one pruned result and no failed or refused results"))
		}
	case PruneReceiptPartial:
		if counts["pruned"] == 0 || (counts["failed"]+counts["refused"]) == 0 {
			*errs = append(*errs, errors.New("status partial requires pruned result plus failed or refused result"))
		}
	case PruneReceiptFailed:
		if counts["pruned"] > 0 || (counts["failed"]+counts["refused"]) == 0 {
			*errs = append(*errs, errors.New("status failed requires failed or refused results and no pruned results"))
		}
	}
}

func validatePruneObservedTargetState(name string, state PruneObservedTargetState, errs *[]error) {
	if state.empty() {
		return
	}
	if state.Present == nil {
		*errs = append(*errs, fmt.Errorf("%s.present is required", name))
	}
	require(name+".kind", state.Kind, errs)
	if strings.TrimSpace(state.Kind) != "" && !targetDriftKindValid(state.Kind) {
		*errs = append(*errs, fmt.Errorf("%s.kind must be one of file, dir, symlink, special, missing, other", name))
	}
	requireSafeControlRelativePath(name+".path", state.Path, errs)
	if state.HasSizeEvidence() && state.Size < 0 {
		*errs = append(*errs, fmt.Errorf("%s.size cannot be negative", name))
	}
	if strings.TrimSpace(state.Digest) != "" && !isSHA256Digest(state.Digest) {
		*errs = append(*errs, fmt.Errorf("%s.digest must be sha256 hex", name))
	}
	if strings.TrimSpace(state.ModTime) != "" {
		if _, err := time.Parse(time.RFC3339Nano, state.ModTime); err != nil {
			*errs = append(*errs, fmt.Errorf("%s.mod_time must be RFC3339 timestamp: %w", name, err))
		}
	}
	if strings.TrimSpace(state.Digest) != "" && state.Kind != "file" {
		*errs = append(*errs, fmt.Errorf("%s.digest is only valid for file entries", name))
	}
	if strings.TrimSpace(state.SymlinkTarget) != "" {
		if err := pathguard.ValidateRelativeSymlinkTarget(state.SymlinkTarget); err != nil {
			*errs = append(*errs, fmt.Errorf("%s.symlink_target is unsafe: %w", name, err))
		}
		if state.Kind != "symlink" {
			*errs = append(*errs, fmt.Errorf("%s.symlink_target is only valid for symlink entries", name))
		}
	}
	if state.Present != nil && !*state.Present && state.Kind != "missing" {
		*errs = append(*errs, fmt.Errorf("%s.kind must be missing when present is false", name))
	}
	if state.Present != nil && *state.Present && state.Kind == "missing" {
		*errs = append(*errs, fmt.Errorf("%s.kind cannot be missing when present is true", name))
	}
	if state.Kind == "missing" && (state.HasSizeEvidence() || state.HasModeEvidence() || strings.TrimSpace(state.Digest) != "" || strings.TrimSpace(state.ModTime) != "" || strings.TrimSpace(state.SymlinkTarget) != "") {
		*errs = append(*errs, fmt.Errorf("%s missing state cannot include file metadata", name))
	}
}

func validatePruneRefusal(index int, refusal PruneRefusal, errs *[]error) {
	prefix := fmt.Sprintf("refusals[%d]", index)
	validateArtifactIDField(prefix+".soft_delete_id", refusal.SoftDeleteID, errs)
	if strings.TrimSpace(refusal.TargetPath) != "" {
		requireSafeControlRelativePath(prefix+".target_path", refusal.TargetPath, errs)
	}
	require(prefix+".reason_code", refusal.ReasonCode, errs)
	validateArtifactIDField(prefix+".reason_code", refusal.ReasonCode, errs)
	require(prefix+".message", refusal.Message, errs)
}

func (d HistoryIndex) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("updated_at", d.UpdatedAt, &errs)
	validateSessionIDField("latest", d.Latest, &errs)
	for i, entry := range d.Sessions {
		if strings.TrimSpace(entry.SessionID) == "" {
			errs = append(errs, fmt.Errorf("sessions[%d].session_id is required", i))
		} else if err := transaction.ValidateSessionID(entry.SessionID); err != nil {
			errs = append(errs, fmt.Errorf("sessions[%d].session_id is invalid: %w", i, err))
		}
		if strings.TrimSpace(entry.StartedAt) == "" {
			errs = append(errs, fmt.Errorf("sessions[%d].started_at is required", i))
		}
		if strings.TrimSpace(entry.ReceiptRef) == "" {
			errs = append(errs, fmt.Errorf("sessions[%d].receipt_ref is required", i))
		} else if entry.ReceiptRef != "sessions/"+entry.SessionID+"/receipt.json" {
			errs = append(errs, fmt.Errorf("sessions[%d].receipt_ref must match session_id receipt path", i))
		}
	}
	return errors.Join(errs...)
}

func (d RecoveryState) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("updated_at", d.UpdatedAt, &errs)
	switch d.Status {
	case RecoveryClean, RecoveryInterrupted, RecoveryRepairing:
	default:
		errs = append(errs, errors.New("status must be one of clean, interrupted, repairing"))
	}
	if d.Status != RecoveryClean && strings.TrimSpace(d.SessionID) == "" {
		errs = append(errs, errors.New("session_id is required unless status is clean"))
	}
	if strings.TrimSpace(d.SessionID) != "" {
		validateSessionIDField("session_id", d.SessionID, &errs)
	}
	return errors.Join(errs...)
}

func (s NetworkTransferStatus) Valid() bool {
	switch s {
	case NetworkTransferStarted,
		NetworkTransferPublished,
		NetworkTransferInterrupted,
		NetworkTransferAuthRefused,
		NetworkTransferNeedsRepair,
		NetworkTransferPublishFailed,
		NetworkTransferFailed:
		return true
	default:
		return false
	}
}

func (d NetworkTransfer) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("session_id", d.SessionID, &errs)
	validateSessionIDField("session_id", d.SessionID, &errs)
	require("profile_id", d.ProfileID, &errs)
	if strings.TrimSpace(d.ProfileID) != "" {
		if err := transport.ValidateProfileID(d.ProfileID); err != nil {
			errs = append(errs, fmt.Errorf("profile_id is invalid: %w", err))
		}
	}
	require("target_id", d.TargetID, &errs)
	if strings.TrimSpace(d.TargetID) != "" {
		if err := transport.ValidateProfileID(d.TargetID); err != nil {
			errs = append(errs, fmt.Errorf("target_id is invalid: %w", err))
		}
	}
	require("source_device_id", d.SourceDeviceID, &errs)
	if strings.TrimSpace(d.SourceDeviceID) != "" {
		if err := transport.DeviceID(d.SourceDeviceID).Validate(); err != nil {
			errs = append(errs, fmt.Errorf("source_device_id is invalid: %w", err))
		}
	}
	require("target_device_id", d.TargetDeviceID, &errs)
	if strings.TrimSpace(d.TargetDeviceID) != "" {
		if err := transport.DeviceID(d.TargetDeviceID).Validate(); err != nil {
			errs = append(errs, fmt.Errorf("target_device_id is invalid: %w", err))
		}
	}
	if strings.TrimSpace(d.SourceDeviceID) != "" && d.SourceDeviceID == d.TargetDeviceID {
		errs = append(errs, errors.New("source_device_id and target_device_id must differ"))
	}
	require("protocol_version", d.ProtocolVersion, &errs)
	if strings.TrimSpace(d.ProtocolVersion) != "" {
		if err := transport.ValidateProtocolVersion(d.ProtocolVersion); err != nil {
			errs = append(errs, fmt.Errorf("protocol_version is invalid: %w", err))
		}
	}
	if d.PrivacyPolicy.Level != 0 {
		if err := d.PrivacyPolicy.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("privacy_policy is invalid: %w", err))
		}
	}
	if d.PrivacyOverhead != nil {
		if err := d.PrivacyOverhead.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("privacy_overhead is invalid: %w", err))
		}
		if d.PrivacyOverhead.JitterBudgetMillis != 0 {
			if d.PrivacyPolicy.Level != transport.PrivacyLevel2 {
				errs = append(errs, errors.New("privacy_overhead.jitter_budget_millis requires privacy_policy.level 2"))
			} else if d.PrivacyPolicy.JitterBudget == 0 {
				errs = append(errs, errors.New("privacy_overhead.jitter_budget_millis requires privacy_policy.jitter_budget_millis"))
			} else if d.PrivacyOverhead.JitterBudgetMillis != d.PrivacyPolicy.JitterBudget {
				errs = append(errs, errors.New("privacy_overhead.jitter_budget_millis must match privacy_policy.jitter_budget_millis"))
			}
		}
	}
	if d.Status == NetworkTransferPublished && d.PrivacyPolicy.Level == transport.PrivacyLevel2 && d.PrivacyOverhead == nil {
		errs = append(errs, errors.New("privacy_overhead is required for published level 2 network transfer"))
	}
	require("status", string(d.Status), &errs)
	if strings.TrimSpace(string(d.Status)) != "" && !d.Status.Valid() {
		errs = append(errs, fmt.Errorf("status must be one of started, published, interrupted, auth_refused, needs_repair, publish_failed, failed"))
	}
	require("stage", d.Stage, &errs)
	if strings.TrimSpace(d.Stage) != "" && !networkTransferStageValid(d.Stage) {
		errs = append(errs, errors.New("stage must be one of begin, status, chunk, commit, warning_artifacts, network_transfer_artifact, transport"))
	}
	require("started_at", d.StartedAt, &errs)
	require("updated_at", d.UpdatedAt, &errs)
	var startedAt time.Time
	var startedAtOK bool
	if strings.TrimSpace(d.StartedAt) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, d.StartedAt)
		if err != nil {
			errs = append(errs, fmt.Errorf("started_at must be RFC3339 timestamp: %w", err))
		} else {
			startedAt = parsed
			startedAtOK = true
		}
	}
	var updatedAt time.Time
	var updatedAtOK bool
	if strings.TrimSpace(d.UpdatedAt) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, d.UpdatedAt)
		if err != nil {
			errs = append(errs, fmt.Errorf("updated_at must be RFC3339 timestamp: %w", err))
		} else {
			updatedAt = parsed
			updatedAtOK = true
		}
	}
	if startedAtOK && updatedAtOK && updatedAt.Before(startedAt) {
		errs = append(errs, errors.New("updated_at must be greater than or equal to started_at"))
	}
	if len(d.Attempts) == 0 {
		errs = append(errs, errors.New("attempts must contain at least one attempt"))
	}
	for i, attempt := range d.Attempts {
		if strings.TrimSpace(attempt.AttemptID) == "" {
			errs = append(errs, fmt.Errorf("attempts[%d].attempt_id is required", i))
		} else if err := ValidateArtifactID(attempt.AttemptID); err != nil {
			errs = append(errs, fmt.Errorf("attempts[%d].attempt_id is unsafe: %w", i, err))
		}
		var attemptStartedAt time.Time
		var attemptStartedAtOK bool
		if strings.TrimSpace(attempt.StartedAt) == "" {
			errs = append(errs, fmt.Errorf("attempts[%d].started_at is required", i))
		} else {
			parsed, err := time.Parse(time.RFC3339Nano, attempt.StartedAt)
			if err != nil {
				errs = append(errs, fmt.Errorf("attempts[%d].started_at must be RFC3339 timestamp: %w", i, err))
			} else {
				attemptStartedAt = parsed
				attemptStartedAtOK = true
			}
		}
		var attemptEndedAt time.Time
		var attemptEndedAtOK bool
		if strings.TrimSpace(attempt.EndedAt) != "" {
			parsed, err := time.Parse(time.RFC3339Nano, attempt.EndedAt)
			if err != nil {
				errs = append(errs, fmt.Errorf("attempts[%d].ended_at must be RFC3339 timestamp: %w", i, err))
			} else {
				attemptEndedAt = parsed
				attemptEndedAtOK = true
			}
		}
		if attemptStartedAtOK && attemptEndedAtOK && attemptEndedAt.Before(attemptStartedAt) {
			errs = append(errs, fmt.Errorf("attempts[%d].ended_at must be greater than or equal to started_at", i))
		}
		if strings.TrimSpace(attempt.Stage) == "" {
			errs = append(errs, fmt.Errorf("attempts[%d].stage is required", i))
		} else if !networkTransferStageValid(attempt.Stage) {
			errs = append(errs, fmt.Errorf("attempts[%d].stage must be one of begin, status, chunk, commit, warning_artifacts, network_transfer_artifact, transport", i))
		}
		if strings.TrimSpace(string(attempt.Status)) == "" {
			errs = append(errs, fmt.Errorf("attempts[%d].status is required", i))
		} else if !attempt.Status.Valid() {
			errs = append(errs, fmt.Errorf("attempts[%d].status must be one of started, published, interrupted, auth_refused, needs_repair, publish_failed, failed", i))
		}
		if attempt.Status != NetworkTransferStarted && strings.TrimSpace(attempt.EndedAt) == "" {
			errs = append(errs, fmt.Errorf("attempts[%d].ended_at is required for terminal attempt status", i))
		}
	}
	if len(d.Attempts) > 0 {
		last := d.Attempts[len(d.Attempts)-1]
		if last.Status != d.Status {
			errs = append(errs, errors.New("last attempt status must match top-level status"))
		}
		if last.Stage != d.Stage {
			errs = append(errs, errors.New("last attempt stage must match top-level stage"))
		}
	}
	return errors.Join(errs...)
}

func networkTransferStageValid(stage string) bool {
	switch stage {
	case "begin", "status", "chunk", "commit", "warning_artifacts", "network_transfer_artifact", "transport":
		return true
	default:
		return false
	}
}

func pruneTargetKindValid(kind string) bool {
	switch kind {
	case "file", "dir", "symlink", "special", "other":
		return true
	default:
		return false
	}
}

func pruneApprovalStatusValid(status string) bool {
	switch status {
	case "approved", "refused", "superseded":
		return true
	default:
		return false
	}
}

func pruneReceiptItemResultValid(result string) bool {
	switch result {
	case "would_prune", "pruned", "skipped", "refused", "failed":
		return true
	default:
		return false
	}
}

func pruneReceiptIntendedActionValid(action string) bool {
	switch action {
	case "delete_file", "delete_symlink":
		return true
	default:
		return false
	}
}

func validateArtifactIDField(name string, id string, errs *[]error) {
	if strings.TrimSpace(id) == "" {
		return
	}
	if err := ValidateArtifactID(id); err != nil {
		*errs = append(*errs, fmt.Errorf("%s is unsafe: %w", name, err))
	}
}

func requireDigest(name string, digest string, errs *[]error) {
	require(name, digest, errs)
	if strings.TrimSpace(digest) == "" {
		return
	}
	if !isSHA256Digest(digest) {
		*errs = append(*errs, fmt.Errorf("%s must be sha256 hex", name))
	}
}

func requireRFC3339(name string, value string, errs *[]error) (time.Time, bool) {
	require(name, value, errs)
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be RFC3339 timestamp: %w", name, err))
		return time.Time{}, false
	}
	return parsed, true
}

func parseOptionalRFC3339(name string, value string, errs *[]error) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s must be RFC3339 timestamp: %w", name, err))
		return time.Time{}, false
	}
	return parsed, true
}

func requireSafeControlRelativePath(name string, value string, errs *[]error) {
	require(name, value, errs)
	if strings.TrimSpace(value) == "" {
		return
	}
	if err := pathguard.ValidateSlashRelativePath(value, 0); err != nil {
		*errs = append(*errs, fmt.Errorf("%s is unsafe: %w", name, err))
	}
	if pathguard.IsReservedControlPath(value) {
		*errs = append(*errs, fmt.Errorf("%s is unsafe: reserved control directory", name))
	}
}

func targetDriftReviewStateValid(state string) bool {
	switch state {
	case "needs_review", "acknowledged", "resolved":
		return true
	default:
		return false
	}
}

func targetDriftReviewActionValid(action string) bool {
	switch action {
	case "acknowledge", "resolve":
		return true
	default:
		return false
	}
}

func targetDriftKindValid(kind string) bool {
	switch kind {
	case "file", "dir", "symlink", "special", "missing", "other":
		return true
	default:
		return false
	}
}

func validateSessionIDField(name string, id string, errs *[]error) {
	if strings.TrimSpace(id) == "" {
		return
	}
	if err := transaction.ValidateSessionID(id); err != nil {
		*errs = append(*errs, fmt.Errorf("%s is invalid: %w", name, err))
	}
}

func requireVersion(version int, errs *[]error) {
	if version != CurrentVersion {
		*errs = append(*errs, fmt.Errorf("version must be %d", CurrentVersion))
	}
}

func require(field string, value string, errs *[]error) {
	if strings.TrimSpace(value) == "" {
		*errs = append(*errs, fmt.Errorf("%s is required", field))
	}
}
