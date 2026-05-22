package incrementalsync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/audit"
	"github.com/khicago/supermover/internal/durable"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/scan"
)

const (
	SchemaV1       = "supermover.incremental_sync.queue/v1"
	StateFileName  = "queue.json"
	StatusQueued   = "queued"
	StatusBackoff  = "backoff"
	StatusCanceled = "canceled"
	StatusDone     = "done"
)

var ErrCorruptQueue = errors.New("corrupt incremental sync queue")

type Clock func() time.Time

type Scheduler struct {
	dir   string
	clock Clock
}

type Options struct {
	StateDir string
	Now      Clock
}

type Scope struct {
	ProfileID string `json:"profile_id"`
	TargetID  string `json:"target_id"`
}

type Snapshot struct {
	Profile profile.Profile
	Root    profile.Root
	Scan    scan.Result
}

type EnqueueResult struct {
	Scope     Scope          `json:"scope"`
	StatePath string         `json:"state_path"`
	Enqueued  []QueueEntry   `json:"enqueued,omitempty"`
	Skipped   []SkippedEntry `json:"skipped,omitempty"`
	Audit     []audit.Record `json:"audit,omitempty"`
	Summary   Summary        `json:"summary"`
}

type SkippedEntry struct {
	Root   string `json:"root"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type QueueEntry struct {
	ID            string    `json:"id"`
	ProfileID     string    `json:"profile_id"`
	TargetID      string    `json:"target_id"`
	Root          string    `json:"root"`
	Path          string    `json:"path"`
	Kind          scan.Kind `json:"kind"`
	Digest        string    `json:"digest,omitempty"`
	SymlinkTarget string    `json:"symlink_target,omitempty"`
	ModTime       string    `json:"mod_time"`
	Size          int64     `json:"size,omitempty"`
	Mode          uint32    `json:"mode,omitempty"`
	EnqueuedAt    string    `json:"enqueued_at"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	NextDueAt     string    `json:"next_due_at,omitempty"`
	CanceledAt    string    `json:"canceled_at,omitempty"`
	DoneAt        string    `json:"done_at,omitempty"`
	UpdatedAt     string    `json:"updated_at"`
	hadSize       bool
	hadMode       bool
}

type queueEntryJSON struct {
	ID            string    `json:"id"`
	ProfileID     string    `json:"profile_id"`
	TargetID      string    `json:"target_id"`
	Root          string    `json:"root"`
	Path          string    `json:"path"`
	Kind          scan.Kind `json:"kind"`
	Digest        string    `json:"digest,omitempty"`
	SymlinkTarget string    `json:"symlink_target,omitempty"`
	ModTime       string    `json:"mod_time"`
	Size          *int64    `json:"size,omitempty"`
	Mode          *uint32   `json:"mode,omitempty"`
	EnqueuedAt    string    `json:"enqueued_at"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	NextDueAt     string    `json:"next_due_at,omitempty"`
	CanceledAt    string    `json:"canceled_at,omitempty"`
	DoneAt        string    `json:"done_at,omitempty"`
	UpdatedAt     string    `json:"updated_at"`
}

func (e QueueEntry) MarshalJSON() ([]byte, error) {
	wire := queueEntryJSON{
		ID:            e.ID,
		ProfileID:     e.ProfileID,
		TargetID:      e.TargetID,
		Root:          e.Root,
		Path:          e.Path,
		Kind:          e.Kind,
		Digest:        e.Digest,
		SymlinkTarget: e.SymlinkTarget,
		ModTime:       e.ModTime,
		EnqueuedAt:    e.EnqueuedAt,
		Status:        e.Status,
		Attempts:      e.Attempts,
		LastError:     e.LastError,
		NextDueAt:     e.NextDueAt,
		CanceledAt:    e.CanceledAt,
		DoneAt:        e.DoneAt,
		UpdatedAt:     e.UpdatedAt,
	}
	if e.hadSize || e.Size != 0 {
		size := e.Size
		wire.Size = &size
	}
	if e.hadMode || e.Mode != 0 {
		mode := e.Mode
		wire.Mode = &mode
	}
	return json.Marshal(wire)
}

func (e *QueueEntry) UnmarshalJSON(data []byte) error {
	var wire queueEntryJSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	*e = QueueEntry{
		ID:            wire.ID,
		ProfileID:     wire.ProfileID,
		TargetID:      wire.TargetID,
		Root:          wire.Root,
		Path:          wire.Path,
		Kind:          wire.Kind,
		Digest:        wire.Digest,
		SymlinkTarget: wire.SymlinkTarget,
		ModTime:       wire.ModTime,
		EnqueuedAt:    wire.EnqueuedAt,
		Status:        wire.Status,
		Attempts:      wire.Attempts,
		LastError:     wire.LastError,
		NextDueAt:     wire.NextDueAt,
		CanceledAt:    wire.CanceledAt,
		DoneAt:        wire.DoneAt,
		UpdatedAt:     wire.UpdatedAt,
		hadSize:       wire.Size != nil,
		hadMode:       wire.Mode != nil,
	}
	if wire.Size != nil {
		e.Size = *wire.Size
	}
	if wire.Mode != nil {
		e.Mode = *wire.Mode
	}
	return nil
}

type State struct {
	Schema    string       `json:"schema"`
	Scope     Scope        `json:"scope"`
	UpdatedAt string       `json:"updated_at"`
	Entries   []QueueEntry `json:"entries,omitempty"`
}

type Summary struct {
	ProfileID    string         `json:"profile_id"`
	TargetID     string         `json:"target_id"`
	Queued       int            `json:"queued"`
	Backoff      int            `json:"backoff"`
	Canceled     int            `json:"canceled"`
	Done         int            `json:"done"`
	Ready        int            `json:"ready"`
	Total        int            `json:"total"`
	Roots        []RootSummary  `json:"roots,omitempty"`
	ByStatus     map[string]int `json:"by_status,omitempty"`
	WarningCount int            `json:"warning_count,omitempty"`
	Warnings     []audit.Record `json:"warnings,omitempty"`
	StatePath    string         `json:"state_path,omitempty"`
	GeneratedAt  string         `json:"generated_at"`
}

type RootSummary struct {
	Root     string `json:"root"`
	Queued   int    `json:"queued"`
	Backoff  int    `json:"backoff"`
	Canceled int    `json:"canceled"`
	Done     int    `json:"done"`
	Ready    int    `json:"ready"`
	Total    int    `json:"total"`
}

type RetryOptions struct {
	EntryID string
	Err     error
	Backoff time.Duration
}

func New(opts Options) (*Scheduler, error) {
	stateDir, clock, err := schedulerOptions(opts)
	if err != nil {
		return nil, err
	}
	if err := pathguard.EnsurePlainDirectory(stateDir, stateDir, 0o755); err != nil {
		return nil, err
	}
	return &Scheduler{dir: stateDir, clock: clock}, nil
}

func Open(opts Options) (*Scheduler, error) {
	stateDir, clock, err := schedulerOptions(opts)
	if err != nil {
		return nil, err
	}
	if err := validateReadOnlyStateDir(stateDir); err != nil {
		return nil, err
	}
	return &Scheduler{dir: stateDir, clock: clock}, nil
}

func schedulerOptions(opts Options) (string, Clock, error) {
	if strings.TrimSpace(opts.StateDir) == "" {
		return "", nil, errors.New("state directory is required")
	}
	stateDir, err := filepath.Abs(opts.StateDir)
	if err != nil {
		return "", nil, err
	}
	clock := opts.Now
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return stateDir, clock, nil
}

func validateReadOnlyStateDir(stateDir string) error {
	info, err := os.Lstat(stateDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: state directory %q is a symlink", pathguard.ErrUnsafePath, stateDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: state directory %q is not a directory", pathguard.ErrUnsafePath, stateDir)
	}
	return nil
}

func SnapshotProfile(p profile.Profile) ([]Snapshot, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	snapshots := make([]Snapshot, 0, len(p.Roots))
	for _, root := range p.Roots {
		result, err := scan.Scan(root.Path)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, Snapshot{Profile: p, Root: root, Scan: result})
	}
	return snapshots, nil
}

func (s *Scheduler) Enqueue(snapshot Snapshot) (EnqueueResult, error) {
	if s == nil {
		return EnqueueResult{}, errors.New("scheduler is nil")
	}
	if err := snapshot.Profile.Validate(); err != nil {
		return EnqueueResult{}, err
	}
	rootID := snapshot.Root.ID
	if strings.TrimSpace(rootID) == "" {
		return EnqueueResult{}, errors.New("root id is required")
	}
	if strings.TrimSpace(rootID) != rootID {
		return EnqueueResult{}, errors.New("root id must not be padded")
	}
	if !profileHasRoot(snapshot.Profile, snapshot.Root) {
		return EnqueueResult{}, fmt.Errorf("root %q is not part of profile %q", rootID, snapshot.Profile.ProfileID)
	}
	if err := validateSnapshotScanRoot(snapshot); err != nil {
		return EnqueueResult{}, err
	}
	scope, err := validateScope(Scope{ProfileID: snapshot.Profile.ProfileID, TargetID: snapshot.Profile.Target.TargetID})
	if err != nil {
		return EnqueueResult{}, err
	}
	state, statePath, err := s.loadOrInit(scope)
	if err != nil {
		return EnqueueResult{}, err
	}

	now := canonicalTime(s.clock())
	index := indexEntries(state.Entries)
	result := EnqueueResult{
		Scope:     scope,
		StatePath: statePath,
		Audit:     append([]audit.Record(nil), snapshot.Scan.Audit...),
	}

	for _, entry := range snapshot.Scan.Entries {
		if entry.Path == "." {
			continue
		}
		decision, reason, err := enqueueDecision(scope, rootID, entry)
		if err != nil {
			return EnqueueResult{}, err
		}
		if !decision {
			result.Skipped = append(result.Skipped, SkippedEntry{Root: rootID, Path: entry.Path, Reason: reason})
			continue
		}
		next := queueEntry(scope, rootID, entry, now)
		current, ok := index[next.ID]
		if ok && sameObservedChange(current, next) {
			result.Skipped = append(result.Skipped, SkippedEntry{Root: rootID, Path: entry.Path, Reason: "unchanged"})
			continue
		}
		if ok {
			next.EnqueuedAt = current.EnqueuedAt
			next.Attempts = current.Attempts
			next.LastError = current.LastError
			next.NextDueAt = current.NextDueAt
			if current.Status == StatusBackoff {
				next.Status = StatusBackoff
			}
		}
		index[next.ID] = next
		result.Enqueued = append(result.Enqueued, next)
	}

	var dropped []SkippedEntry
	state.Entries, dropped = dropLegacyTargetlessSymlinks(sortedEntries(index))
	result.Skipped = append(result.Skipped, dropped...)
	state.UpdatedAt = formatTime(now)
	if err := s.writeState(statePath, state); err != nil {
		return EnqueueResult{}, err
	}
	result.Summary = buildSummary(state, statePath, now, result.Audit)
	return result, nil
}

func (s *Scheduler) Ready(scope Scope) ([]QueueEntry, error) {
	state, _, err := s.loadExisting(scope)
	if err != nil {
		return nil, err
	}
	now := s.clock().UTC()
	var ready []QueueEntry
	for _, entry := range state.Entries {
		if legacyTargetlessSymlink(entry) {
			continue
		}
		if !isReady(entry, now) {
			continue
		}
		ready = append(ready, entry)
	}
	sortQueue(ready)
	return ready, nil
}

func (s *Scheduler) RecordRetry(scope Scope, opts RetryOptions) (QueueEntry, error) {
	if s == nil {
		return QueueEntry{}, errors.New("scheduler is nil")
	}
	if strings.TrimSpace(opts.EntryID) == "" {
		return QueueEntry{}, errors.New("entry id is required")
	}
	if opts.Backoff < 0 {
		return QueueEntry{}, errors.New("backoff cannot be negative")
	}
	state, statePath, err := s.loadExisting(scope)
	if err != nil {
		return QueueEntry{}, err
	}
	state.Entries, _ = dropLegacyTargetlessSymlinks(state.Entries)
	now := canonicalTime(s.clock())
	for i := range state.Entries {
		if state.Entries[i].ID != opts.EntryID {
			continue
		}
		state.Entries[i].Status = StatusBackoff
		state.Entries[i].Attempts++
		state.Entries[i].LastError = retryError(opts.Err)
		state.Entries[i].NextDueAt = formatTime(now.Add(opts.Backoff))
		state.Entries[i].UpdatedAt = formatTime(now)
		state.UpdatedAt = formatTime(now)
		if err := s.writeState(statePath, state); err != nil {
			return QueueEntry{}, err
		}
		return state.Entries[i], nil
	}
	return QueueEntry{}, fmt.Errorf("entry %q not found", opts.EntryID)
}

func (s *Scheduler) Cancel(scope Scope, entryID, reason string) (QueueEntry, error) {
	if s == nil {
		return QueueEntry{}, errors.New("scheduler is nil")
	}
	if strings.TrimSpace(entryID) == "" {
		return QueueEntry{}, errors.New("entry id is required")
	}
	state, statePath, err := s.loadExisting(scope)
	if err != nil {
		return QueueEntry{}, err
	}
	state.Entries, _ = dropLegacyTargetlessSymlinks(state.Entries)
	now := canonicalTime(s.clock())
	for i := range state.Entries {
		if state.Entries[i].ID != entryID {
			continue
		}
		state.Entries[i].Status = StatusCanceled
		state.Entries[i].CanceledAt = formatTime(now)
		state.Entries[i].UpdatedAt = formatTime(now)
		if strings.TrimSpace(reason) != "" {
			state.Entries[i].LastError = strings.TrimSpace(reason)
		}
		state.UpdatedAt = formatTime(now)
		if err := s.writeState(statePath, state); err != nil {
			return QueueEntry{}, err
		}
		return state.Entries[i], nil
	}
	return QueueEntry{}, fmt.Errorf("entry %q not found", entryID)
}

func (s *Scheduler) MarkDone(scope Scope, entryID string) (QueueEntry, error) {
	if s == nil {
		return QueueEntry{}, errors.New("scheduler is nil")
	}
	if strings.TrimSpace(entryID) == "" {
		return QueueEntry{}, errors.New("entry id is required")
	}
	state, statePath, err := s.loadExisting(scope)
	if err != nil {
		return QueueEntry{}, err
	}
	state.Entries, _ = dropLegacyTargetlessSymlinks(state.Entries)
	now := canonicalTime(s.clock())
	for i := range state.Entries {
		if state.Entries[i].ID != entryID {
			continue
		}
		state.Entries[i].Status = StatusDone
		state.Entries[i].DoneAt = formatTime(now)
		state.Entries[i].UpdatedAt = formatTime(now)
		state.Entries[i].LastError = ""
		state.Entries[i].NextDueAt = ""
		state.UpdatedAt = formatTime(now)
		if err := s.writeState(statePath, state); err != nil {
			return QueueEntry{}, err
		}
		return state.Entries[i], nil
	}
	return QueueEntry{}, fmt.Errorf("entry %q not found", entryID)
}

func (s *Scheduler) Summary(scope Scope) (Summary, error) {
	state, statePath, err := s.loadExisting(scope)
	if err != nil {
		return Summary{}, err
	}
	state.Entries, _ = dropLegacyTargetlessSymlinks(state.Entries)
	return buildSummary(state, statePath, canonicalTime(s.clock()), nil), nil
}

func (s *Scheduler) StatePath(scope Scope) (string, error) {
	if s == nil {
		return "", errors.New("scheduler is nil")
	}
	scope, err := validateScope(scope)
	if err != nil {
		return "", err
	}
	profileDir, err := scopePathComponent(scope.ProfileID)
	if err != nil {
		return "", fmt.Errorf("profile scope: %w", err)
	}
	targetDir, err := scopePathComponent(scope.TargetID)
	if err != nil {
		return "", fmt.Errorf("target scope: %w", err)
	}
	return filepath.Join(s.dir, "profiles", profileDir, "targets", targetDir, StateFileName), nil
}

func (s *Scheduler) loadOrInit(scope Scope) (State, string, error) {
	var err error
	scope, err = validateScope(scope)
	if err != nil {
		return State{}, "", err
	}
	state, statePath, err := s.loadExisting(scope)
	if err == nil {
		return state, statePath, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return State{Schema: SchemaV1, Scope: scope}, statePath, nil
	}
	return State{}, "", err
}

func (s *Scheduler) loadExisting(scope Scope) (State, string, error) {
	var err error
	scope, err = validateScope(scope)
	if err != nil {
		return State{}, "", err
	}
	statePath, err := s.StatePath(scope)
	if err != nil {
		return State{}, "", err
	}
	file, err := os.Open(statePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return State{}, statePath, err
		}
		return State{}, statePath, err
	}
	defer file.Close()
	state, err := decodeState(file)
	if err != nil {
		return State{}, statePath, fmt.Errorf("%w: %w", ErrCorruptQueue, err)
	}
	if state.Scope != scope {
		return State{}, statePath, fmt.Errorf("%w: queue scope profile=%q target=%q does not match requested profile=%q target=%q", ErrCorruptQueue, state.Scope.ProfileID, state.Scope.TargetID, scope.ProfileID, scope.TargetID)
	}
	return state, statePath, nil
}

func (s *Scheduler) writeState(statePath string, state State) error {
	if err := validateState(state); err != nil {
		return err
	}
	if err := pathguard.EnsurePlainDirectory(s.dir, filepath.Dir(statePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(filepath.Dir(statePath), ".queue-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(data); err != nil {
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
	if err := os.Rename(tempName, statePath); err != nil {
		return err
	}
	return durable.SyncDirBestEffort(filepath.Dir(statePath))
}

func decodeState(r io.Reader) (State, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	var state State
	if err := decoder.Decode(&state); err != nil {
		return State{}, err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return State{}, err
	}
	if err := validateStateForRead(state); err != nil {
		return State{}, err
	}
	return state, nil
}

func validateState(state State) error {
	return validateStateWithOptions(state, stateValidationOptions{})
}

func validateStateForRead(state State) error {
	return validateStateWithOptions(state, stateValidationOptions{allowLegacyMissingSymlinkTarget: true})
}

type stateValidationOptions struct {
	allowLegacyMissingSymlinkTarget bool
}

func validateStateWithOptions(state State, opts stateValidationOptions) error {
	if state.Schema != SchemaV1 {
		return fmt.Errorf("schema %q is not supported", state.Schema)
	}
	if state.UpdatedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, state.UpdatedAt); err != nil {
			return fmt.Errorf("updated_at must be RFC3339 timestamp: %w", err)
		}
	}
	scope, err := validateScope(state.Scope)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(state.Entries))
	for i, entry := range state.Entries {
		if entry.ProfileID != scope.ProfileID || entry.TargetID != scope.TargetID {
			return fmt.Errorf("entries[%d] scope profile=%q target=%q does not match queue scope profile=%q target=%q", i, entry.ProfileID, entry.TargetID, scope.ProfileID, scope.TargetID)
		}
		if _, ok := seen[entry.ID]; ok {
			return fmt.Errorf("duplicate entry id %q", entry.ID)
		}
		seen[entry.ID] = struct{}{}
		if err := validateEntryWithOptions(entry, opts); err != nil {
			return fmt.Errorf("entries[%d]: %w", i, err)
		}
	}
	return nil
}

func validateEntryWithOptions(entry QueueEntry, opts stateValidationOptions) error {
	if strings.TrimSpace(entry.ID) == "" {
		return errors.New("entry id is required")
	}
	if strings.TrimSpace(entry.Root) == "" {
		return errors.New("root is required")
	}
	if strings.TrimSpace(entry.Root) != entry.Root {
		return errors.New("root must not be padded")
	}
	if err := pathguard.ValidateSlashRelativePath(entry.Path, 0); err != nil {
		return err
	}
	if pathguard.IsReservedControlPath(entry.Path) {
		return fmt.Errorf("%w: data path uses reserved control directory", pathguard.ErrUnsafePath)
	}
	if _, err := time.Parse(time.RFC3339Nano, entry.ModTime); err != nil {
		return fmt.Errorf("mod_time must be RFC3339 timestamp: %w", err)
	}
	for name, value := range map[string]string{
		"enqueued_at": entry.EnqueuedAt,
		"updated_at":  entry.UpdatedAt,
		"next_due_at": entry.NextDueAt,
		"canceled_at": entry.CanceledAt,
		"done_at":     entry.DoneAt,
	} {
		if value == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
			return fmt.Errorf("%s must be RFC3339 timestamp: %w", name, err)
		}
	}
	switch entry.Status {
	case StatusQueued, StatusBackoff, StatusCanceled, StatusDone:
	default:
		return fmt.Errorf("unsupported status %q", entry.Status)
	}
	if entry.Kind == scan.KindSymlink {
		if opts.allowLegacyMissingSymlinkTarget && entry.SymlinkTarget == "" {
			return nil
		}
		if err := pathguard.ValidateRelativeSymlinkTarget(entry.SymlinkTarget); err != nil {
			return err
		}
	} else if strings.TrimSpace(entry.SymlinkTarget) != "" {
		return errors.New("symlink_target is only valid for symlink entries")
	}
	return nil
}

func validateScope(scope Scope) (Scope, error) {
	if strings.TrimSpace(scope.ProfileID) == "" {
		return Scope{}, errors.New("profile_id is required")
	}
	if strings.TrimSpace(scope.ProfileID) != scope.ProfileID {
		return Scope{}, errors.New("profile_id must not be padded")
	}
	if strings.TrimSpace(scope.TargetID) == "" {
		return Scope{}, errors.New("target_id is required")
	}
	if strings.TrimSpace(scope.TargetID) != scope.TargetID {
		return Scope{}, errors.New("target_id must not be padded")
	}
	return scope, nil
}

func profileHasRoot(p profile.Profile, root profile.Root) bool {
	for _, candidate := range p.Roots {
		if candidate.ID == root.ID && filepath.Clean(candidate.Path) == filepath.Clean(root.Path) {
			return true
		}
	}
	return false
}

func validateSnapshotScanRoot(snapshot Snapshot) error {
	if strings.TrimSpace(snapshot.Scan.Root) == "" {
		return errors.New("scan root is required")
	}
	scanRoot, err := filepath.Abs(filepath.FromSlash(snapshot.Scan.Root))
	if err != nil {
		return fmt.Errorf("scan root: %w", err)
	}
	rootPath, err := filepath.Abs(snapshot.Root.Path)
	if err != nil {
		return fmt.Errorf("snapshot root path: %w", err)
	}
	if filepath.Clean(scanRoot) != filepath.Clean(rootPath) {
		return fmt.Errorf("scan root %q does not match snapshot root %q", snapshot.Scan.Root, snapshot.Root.Path)
	}
	return nil
}

func enqueueDecision(scope Scope, root string, entry scan.Entry) (bool, string, error) {
	if err := pathguard.ValidateSlashRelativePath(entry.Path, 0); err != nil {
		return false, "", err
	}
	if pathguard.IsReservedControlPath(entry.Path) {
		return false, "reserved_control_path", nil
	}
	switch entry.Kind {
	case scan.KindRegular:
		if strings.TrimSpace(entry.Digest) == "" {
			return false, "", fmt.Errorf("regular path %q has no digest", entry.Path)
		}
		return true, "", nil
	case scan.KindSymlink:
		if err := pathguard.ValidateRelativeSymlinkTarget(entry.SymlinkTarget); err != nil {
			return false, "unsafe_symlink_target", nil
		}
		return true, "", nil
	case scan.KindDir:
		return true, "", nil
	default:
		return false, "unsupported_kind", nil
	}
}

func queueEntry(scope Scope, root string, entry scan.Entry, now time.Time) QueueEntry {
	out := QueueEntry{
		ID:         entryID(scope, root, entry.Path),
		ProfileID:  scope.ProfileID,
		TargetID:   scope.TargetID,
		Root:       root,
		Path:       entry.Path,
		Kind:       entry.Kind,
		Digest:     entry.Digest,
		ModTime:    formatTime(entry.ModTime),
		EnqueuedAt: formatTime(now),
		Status:     StatusQueued,
		UpdatedAt:  formatTime(now),
	}
	if entry.Kind == scan.KindRegular {
		out.Size = entry.Size
		out.Mode = uint32(entry.Mode.Perm())
		out.hadSize = true
		out.hadMode = true
	}
	if entry.Kind == scan.KindSymlink {
		out.SymlinkTarget = entry.SymlinkTarget
	}
	return out
}

func sameObservedChange(a, b QueueEntry) bool {
	return a.ProfileID == b.ProfileID &&
		a.TargetID == b.TargetID &&
		a.Root == b.Root &&
		a.Path == b.Path &&
		a.Kind == b.Kind &&
		a.Digest == b.Digest &&
		a.SymlinkTarget == b.SymlinkTarget &&
		a.ModTime == b.ModTime &&
		a.Size == b.Size &&
		a.Mode == b.Mode
}

func indexEntries(entries []QueueEntry) map[string]QueueEntry {
	out := make(map[string]QueueEntry, len(entries))
	for _, entry := range entries {
		out[entry.ID] = entry
	}
	return out
}

func sortedEntries(index map[string]QueueEntry) []QueueEntry {
	entries := make([]QueueEntry, 0, len(index))
	for _, entry := range index {
		entries = append(entries, entry)
	}
	sortQueue(entries)
	return entries
}

func dropLegacyTargetlessSymlinks(entries []QueueEntry) ([]QueueEntry, []SkippedEntry) {
	out := make([]QueueEntry, 0, len(entries))
	var skipped []SkippedEntry
	for _, entry := range entries {
		if legacyTargetlessSymlink(entry) {
			skipped = append(skipped, SkippedEntry{Root: entry.Root, Path: entry.Path, Reason: "legacy_missing_symlink_target"})
			continue
		}
		out = append(out, entry)
	}
	return out, skipped
}

func legacyTargetlessSymlink(entry QueueEntry) bool {
	return entry.Kind == scan.KindSymlink && entry.SymlinkTarget == ""
}

func sortQueue(entries []QueueEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Root != entries[j].Root {
			return entries[i].Root < entries[j].Root
		}
		if entries[i].Path != entries[j].Path {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].ID < entries[j].ID
	})
}

func buildSummary(state State, statePath string, now time.Time, warnings []audit.Record) Summary {
	byRoot := make(map[string]*RootSummary)
	summary := Summary{
		ProfileID:    state.Scope.ProfileID,
		TargetID:     state.Scope.TargetID,
		ByStatus:     make(map[string]int),
		WarningCount: len(warnings),
		Warnings:     append([]audit.Record(nil), warnings...),
		StatePath:    statePath,
		GeneratedAt:  formatTime(now),
	}
	for _, entry := range state.Entries {
		root := byRoot[entry.Root]
		if root == nil {
			root = &RootSummary{Root: entry.Root}
			byRoot[entry.Root] = root
		}
		summary.Total++
		root.Total++
		summary.ByStatus[entry.Status]++
		switch entry.Status {
		case StatusQueued:
			summary.Queued++
			root.Queued++
		case StatusBackoff:
			summary.Backoff++
			root.Backoff++
		case StatusCanceled:
			summary.Canceled++
			root.Canceled++
		case StatusDone:
			summary.Done++
			root.Done++
		}
		if isReady(entry, now) {
			summary.Ready++
			root.Ready++
		}
	}
	roots := make([]RootSummary, 0, len(byRoot))
	for _, root := range byRoot {
		roots = append(roots, *root)
	}
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].Root < roots[j].Root
	})
	summary.Roots = roots
	if len(summary.ByStatus) == 0 {
		summary.ByStatus = nil
	}
	return summary
}

func isReady(entry QueueEntry, now time.Time) bool {
	switch entry.Status {
	case StatusQueued:
		return true
	case StatusBackoff:
		if strings.TrimSpace(entry.NextDueAt) == "" {
			return true
		}
		due, err := time.Parse(time.RFC3339Nano, entry.NextDueAt)
		return err == nil && !due.After(now)
	default:
		return false
	}
}

func entryID(scope Scope, root, path string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{scope.ProfileID, scope.TargetID, root, path}, "\x00")))
	return "isq_" + hex.EncodeToString(sum[:8])
}

func scopePathComponent(value string) (string, error) {
	if strings.TrimSpace(value) != value || value == "" {
		return "", errors.New("id must not be empty or padded")
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8]), nil
}

func formatTime(t time.Time) string {
	return canonicalTime(t).Format(time.RFC3339Nano)
}

func canonicalTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t.UTC()
}

func retryError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
