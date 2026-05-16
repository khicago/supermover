package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/khicago/supermover/internal/durable"
)

const (
	DirName        = ".supermover"
	CurrentVersion = 1
)

type ArtifactType string

const (
	ArtifactProfileSnapshot ArtifactType = "profile_snapshot"
	ArtifactPairingReceipt  ArtifactType = "pairing_receipt"
	ArtifactSessionReceipt  ArtifactType = "session_receipt"
	ArtifactManifest        ArtifactType = "manifest"
	ArtifactWarning         ArtifactType = "warning"
	ArtifactTargetDrift     ArtifactType = "target_drift"
	ArtifactSoftDelete      ArtifactType = "soft_delete"
	ArtifactHistoryIndex    ArtifactType = "history_index"
	ArtifactRecoveryState   ArtifactType = "recovery_state"
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
	Version         int    `json:"version"`
	ID              string `json:"id"`
	ProfileID       string `json:"profile_id"`
	TargetID        string `json:"target_id"`
	DevicePublicKey string `json:"device_public_key"`
	VerifiedAt      string `json:"verified_at"`
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
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	Size          int64  `json:"size,omitempty"`
	ModTime       string `json:"mod_time,omitempty"`
	Digest        string `json:"digest,omitempty"`
	TargetPath    string `json:"target_path,omitempty"`
	SymlinkTarget string `json:"symlink_target,omitempty"`
}

type Warning struct {
	Version   int      `json:"version"`
	ID        string   `json:"id"`
	SessionID string   `json:"session_id,omitempty"`
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	Paths     []string `json:"paths,omitempty"`
	CreatedAt string   `json:"created_at"`
}

type TargetDrift struct {
	Version    int      `json:"version"`
	ID         string   `json:"id"`
	SessionID  string   `json:"session_id,omitempty"`
	Path       string   `json:"path"`
	DetectedAt string   `json:"detected_at"`
	Change     string   `json:"change"`
	Evidence   []string `json:"evidence,omitempty"`
}

type SoftDelete struct {
	Version    int    `json:"version"`
	ID         string `json:"id"`
	SessionID  string `json:"session_id,omitempty"`
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
	DetectedAt string `json:"detected_at"`
	Reason     string `json:"reason,omitempty"`
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

type Document interface {
	Validate() error
}

func ControlDir(targetRoot string) string {
	return filepath.Join(targetRoot, DirName)
}

func Path(targetRoot string, artifact ArtifactType, id string) (string, error) {
	if strings.TrimSpace(id) == "" && artifact != ArtifactHistoryIndex && artifact != ArtifactRecoveryState {
		return "", errors.New("id is required")
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
	case ArtifactHistoryIndex:
		return filepath.Join(base, "history", "index.json"), nil
	case ArtifactRecoveryState:
		return filepath.Join(base, "recovery", "state.json"), nil
	default:
		return "", fmt.Errorf("unknown artifact type %q", artifact)
	}
}

func Read[T Document](r io.Reader) (T, error) {
	var doc T
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
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
	if err := doc.validateWithOptions(manifestValidationOptions{allowLegacySymlinkTarget: true}); err != nil {
		return doc, err
	}
	return doc, nil
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
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
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	if err := durable.SyncDirBestEffort(filepath.Dir(path)); err != nil {
		return err
	}
	return nil
}

func (d ProfileSnapshot) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	require("profile_id", d.ProfileID, &errs)
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
	require("profile_id", d.ProfileID, &errs)
	require("target_id", d.TargetID, &errs)
	require("device_public_key", d.DevicePublicKey, &errs)
	require("verified_at", d.VerifiedAt, &errs)
	return errors.Join(errs...)
}

func (d SessionReceipt) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
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
	require("created_at", d.CreatedAt, &errs)
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
		if entry.Size < 0 {
			errs = append(errs, fmt.Errorf("entries[%d].size cannot be negative", i))
		}
	}
	return errors.Join(errs...)
}

func (d Warning) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	require("code", d.Code, &errs)
	require("message", d.Message, &errs)
	require("created_at", d.CreatedAt, &errs)
	return errors.Join(errs...)
}

func (d TargetDrift) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	require("path", d.Path, &errs)
	require("detected_at", d.DetectedAt, &errs)
	require("change", d.Change, &errs)
	return errors.Join(errs...)
}

func (d SoftDelete) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("id", d.ID, &errs)
	require("source_path", d.SourcePath, &errs)
	require("target_path", d.TargetPath, &errs)
	require("detected_at", d.DetectedAt, &errs)
	return errors.Join(errs...)
}

func (d HistoryIndex) Validate() error {
	var errs []error
	requireVersion(d.Version, &errs)
	require("updated_at", d.UpdatedAt, &errs)
	for i, entry := range d.Sessions {
		if strings.TrimSpace(entry.SessionID) == "" {
			errs = append(errs, fmt.Errorf("sessions[%d].session_id is required", i))
		}
		if strings.TrimSpace(entry.StartedAt) == "" {
			errs = append(errs, fmt.Errorf("sessions[%d].started_at is required", i))
		}
		if strings.TrimSpace(entry.ReceiptRef) == "" {
			errs = append(errs, fmt.Errorf("sessions[%d].receipt_ref is required", i))
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
	return errors.Join(errs...)
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
