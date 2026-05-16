package transaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/durable"
	"github.com/khicago/supermover/internal/pathguard"
)

type State string

const (
	StateReceived    State = "received"
	StateValidated   State = "validated"
	StateStaged      State = "staged"
	StatePublished   State = "published"
	StateRolledBack  State = "rolled_back"
	StateNeedsRepair State = "needs_repair"
)

var ErrValidation = errors.New("transaction validation failed")

var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type SessionRecord struct {
	ID        string    `json:"id"`
	State     State     `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Note      string    `json:"note,omitempty"`
}

func NewSessionRecord(id string, now time.Time) (SessionRecord, error) {
	if err := ValidateSessionID(id); err != nil {
		return SessionRecord{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return SessionRecord{
		ID:        id,
		State:     StateReceived,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}, nil
}

func (r SessionRecord) Validate() error {
	if err := ValidateSessionID(r.ID); err != nil {
		return err
	}
	if !r.State.Valid() {
		return fmt.Errorf("%w: unknown state %q", ErrValidation, r.State)
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at is required", ErrValidation)
	}
	if r.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: updated_at is required", ErrValidation)
	}
	return nil
}

func (r SessionRecord) WithState(state State, now time.Time) (SessionRecord, error) {
	if !state.Valid() {
		return SessionRecord{}, fmt.Errorf("%w: unknown state %q", ErrValidation, state)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	r.State = state
	r.UpdatedAt = now.UTC()
	return r, nil
}

func (s State) Valid() bool {
	switch s {
	case StateReceived, StateValidated, StateStaged, StatePublished, StateRolledBack, StateNeedsRepair:
		return true
	default:
		return false
	}
}

func ValidateSessionID(id string) error {
	if id == "" {
		return fmt.Errorf("%w: session id is required", ErrValidation)
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) || !sessionIDPattern.MatchString(id) {
		return fmt.Errorf("%w: unsafe session id %q", ErrValidation, id)
	}
	return nil
}

type Layout struct {
	ControlDir string
}

func NewLayout(controlDir string) Layout {
	return Layout{ControlDir: controlDir}
}

func (l Layout) SessionsDir() string {
	return filepath.Join(l.ControlDir, "sessions")
}

func (l Layout) SessionDir(id string) string {
	return filepath.Join(l.SessionsDir(), id)
}

func (l Layout) StagingDir(id string) string {
	return filepath.Join(l.SessionDir(id), "stage")
}

func (l Layout) ManifestPath(id string) string {
	return filepath.Join(l.SessionDir(id), "manifest.json")
}

func (l Layout) RecordPath(id string) string {
	return filepath.Join(l.SessionDir(id), "session.json")
}

func (l Layout) EnsureSessionDirs(id string) error {
	if err := ValidateSessionID(id); err != nil {
		return err
	}
	if err := pathguard.EnsurePlainDirectory(l.StagingDir(id), 0o755); err != nil {
		return fmt.Errorf("create session directories: %w", err)
	}
	return nil
}

func (l Layout) WriteSessionRecord(record SessionRecord) error {
	if err := record.Validate(); err != nil {
		return err
	}
	if err := l.EnsureSessionDirs(record.ID); err != nil {
		return err
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session record: %w", err)
	}
	data = append(data, '\n')

	path := l.RecordPath(record.ID)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("create session record temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write session record temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync session record temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close session record temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("publish session record: %w", err)
	}
	if err := durable.SyncDirBestEffort(filepath.Dir(path)); err != nil {
		return fmt.Errorf("sync session record directory: %w", err)
	}
	cleanup = false
	return nil
}

func ReadSessionRecord(path string) (SessionRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SessionRecord{}, fmt.Errorf("read session record: %w", err)
	}
	var record SessionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return SessionRecord{}, fmt.Errorf("%w: decode %s: %v", ErrValidation, path, err)
	}
	if err := record.Validate(); err != nil {
		return SessionRecord{}, err
	}
	return record, nil
}
