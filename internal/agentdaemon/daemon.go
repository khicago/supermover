package agentdaemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/durable"
	"github.com/khicago/supermover/internal/pathguard"
)

const (
	CurrentVersion = 1

	ServiceManagerNone = "none"
	RunModeForeground  = "foreground"

	StatusAbsent        = "absent"
	StatusInstalled     = "installed"
	StatusStarting      = "starting"
	StatusRunning       = "running"
	StatusStopping      = "stopping"
	StatusStopped       = "stopped"
	StatusFailed        = "failed"
	StatusScopeMismatch = "scope_mismatch"
)

type Install struct {
	Version        int      `json:"version"`
	ProfileID      string   `json:"profile_id"`
	TargetID       string   `json:"target_id"`
	ProfilePath    string   `json:"profile_path"`
	InstalledAt    string   `json:"installed_at"`
	RunMode        string   `json:"run_mode"`
	ServiceManager string   `json:"service_manager"`
	Command        []string `json:"command"`
}

type State struct {
	Version         int                `json:"version"`
	ProfileID       string             `json:"profile_id"`
	TargetID        string             `json:"target_id"`
	ProfilePath     string             `json:"profile_path"`
	Status          string             `json:"status"`
	RunMode         string             `json:"run_mode"`
	ServiceManager  string             `json:"service_manager"`
	PID             int                `json:"pid,omitempty"`
	Mode            string             `json:"mode,omitempty"`
	PairingAddress  string             `json:"pairing_address,omitempty"`
	ReceiverAddress string             `json:"receiver_address,omitempty"`
	StartedAt       string             `json:"started_at,omitempty"`
	UpdatedAt       string             `json:"updated_at"`
	StoppedAt       string             `json:"stopped_at,omitempty"`
	StopIntent      *StopIntentSummary `json:"stop_intent,omitempty"`
	LastError       string             `json:"last_error,omitempty"`
}

type StopIntent struct {
	Version        int    `json:"version"`
	ProfileID      string `json:"profile_id"`
	TargetID       string `json:"target_id"`
	RequestedAt    string `json:"requested_at"`
	Reason         string `json:"reason,omitempty"`
	RequestedByPID int    `json:"requested_by_pid,omitempty"`
}

type RestartIntent struct {
	Version        int    `json:"version"`
	ProfileID      string `json:"profile_id"`
	TargetID       string `json:"target_id"`
	RequestedAt    string `json:"requested_at"`
	Reason         string `json:"reason,omitempty"`
	RequestedByPID int    `json:"requested_by_pid,omitempty"`
}

type LifecycleEvent struct {
	Version    int               `json:"version"`
	ID         string            `json:"id"`
	ProfileID  string            `json:"profile_id"`
	TargetID   string            `json:"target_id"`
	Type       string            `json:"type"`
	RecordedAt string            `json:"recorded_at"`
	Message    string            `json:"message,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
}

type StopIntentSummary struct {
	RequestedAt    string `json:"requested_at"`
	Reason         string `json:"reason,omitempty"`
	RequestedByPID int    `json:"requested_by_pid,omitempty"`
}

type StatusReport struct {
	Version         int              `json:"version"`
	ProfileID       string           `json:"profile_id"`
	TargetID        string           `json:"target_id"`
	Installed       bool             `json:"installed"`
	State           string           `json:"state"`
	RunMode         string           `json:"run_mode,omitempty"`
	ServiceManager  string           `json:"service_manager,omitempty"`
	ScopeIssues     []string         `json:"scope_issues,omitempty"`
	Install         *Install         `json:"install,omitempty"`
	StateRecord     *State           `json:"state_record,omitempty"`
	StopIntent      *StopIntent      `json:"stop_intent,omitempty"`
	RestartIntent   *RestartIntent   `json:"restart_intent,omitempty"`
	LifecycleEvents []LifecycleEvent `json:"lifecycle_events,omitempty"`
}

func DaemonDir(targetRoot string) string {
	return filepath.Join(control.ControlDir(targetRoot), "daemon")
}

func InstallPath(targetRoot string) string {
	return filepath.Join(DaemonDir(targetRoot), "install.json")
}

func StatePath(targetRoot string) string {
	return filepath.Join(DaemonDir(targetRoot), "state.json")
}

func StopIntentPath(targetRoot string) string {
	return filepath.Join(DaemonDir(targetRoot), "stop-intent.json")
}

func RestartIntentPath(targetRoot string) string {
	return filepath.Join(DaemonDir(targetRoot), "restart-intent.json")
}

func LifecycleEventsDir(targetRoot string) string {
	return filepath.Join(DaemonDir(targetRoot), "events")
}

func LifecycleEventPath(targetRoot, eventID string) (string, error) {
	if strings.TrimSpace(eventID) == "" {
		return "", errors.New("event id is required")
	}
	if err := control.ValidateArtifactID(eventID); err != nil {
		return "", err
	}
	return filepath.Join(LifecycleEventsDir(targetRoot), eventID+".json"), nil
}

func NewInstall(profileID, targetID, profilePath string, installedAt time.Time) Install {
	return Install{
		Version:        CurrentVersion,
		ProfileID:      profileID,
		TargetID:       targetID,
		ProfilePath:    profilePath,
		InstalledAt:    formatTime(installedAt),
		RunMode:        RunModeForeground,
		ServiceManager: ServiceManagerNone,
		Command:        []string{"supermover", "daemon", "run", "--foreground", "--profile", profilePath},
	}
}

func NewState(profileID, targetID, profilePath, state string, pid int, now time.Time) State {
	return State{
		Version:        CurrentVersion,
		ProfileID:      profileID,
		TargetID:       targetID,
		ProfilePath:    profilePath,
		Status:         state,
		RunMode:        RunModeForeground,
		ServiceManager: ServiceManagerNone,
		PID:            pid,
		StartedAt:      formatTime(now),
		UpdatedAt:      formatTime(now),
	}
}

func NewStopIntent(profileID, targetID, reason string, requestedByPID int, now time.Time) StopIntent {
	return StopIntent{
		Version:        CurrentVersion,
		ProfileID:      profileID,
		TargetID:       targetID,
		RequestedAt:    formatTime(now),
		Reason:         strings.TrimSpace(reason),
		RequestedByPID: requestedByPID,
	}
}

func NewRestartIntent(profileID, targetID, reason string, requestedByPID int, now time.Time) RestartIntent {
	return RestartIntent{
		Version:        CurrentVersion,
		ProfileID:      profileID,
		TargetID:       targetID,
		RequestedAt:    formatTime(now),
		Reason:         strings.TrimSpace(reason),
		RequestedByPID: requestedByPID,
	}
}

func NewLifecycleEvent(profileID, targetID, eventType, message string, details map[string]string, now time.Time) LifecycleEvent {
	event := LifecycleEvent{
		Version:    CurrentVersion,
		ID:         newLifecycleEventID(now),
		ProfileID:  profileID,
		TargetID:   targetID,
		Type:       strings.TrimSpace(eventType),
		RecordedAt: formatTime(now),
		Message:    redactLifecycleText(message),
		Details:    redactLifecycleDetails(details),
	}
	return event
}

func WriteInstall(targetRoot string, install Install) error {
	return writeJSONFile(targetRoot, InstallPath(targetRoot), install)
}

func ReadInstall(targetRoot string) (Install, error) {
	return readJSONFile[Install](targetRoot, InstallPath(targetRoot))
}

func WriteState(targetRoot string, state State) error {
	return writeJSONFile(targetRoot, StatePath(targetRoot), state)
}

func ReadState(targetRoot string) (State, error) {
	return readJSONFile[State](targetRoot, StatePath(targetRoot))
}

func WriteStopIntent(targetRoot string, intent StopIntent) error {
	return writeJSONFile(targetRoot, StopIntentPath(targetRoot), intent)
}

func ReadStopIntent(targetRoot string) (StopIntent, error) {
	return readJSONFile[StopIntent](targetRoot, StopIntentPath(targetRoot))
}

func WriteRestartIntent(targetRoot string, intent RestartIntent) error {
	return writeJSONFile(targetRoot, RestartIntentPath(targetRoot), intent)
}

func ReadRestartIntent(targetRoot string) (RestartIntent, error) {
	return readJSONFile[RestartIntent](targetRoot, RestartIntentPath(targetRoot))
}

func RemoveStopIntent(targetRoot string) error {
	if err := ensureDaemonBoundary(targetRoot); err != nil {
		return err
	}
	err := os.Remove(StopIntentPath(targetRoot))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func RemoveRestartIntent(targetRoot string) error {
	if err := ensureDaemonBoundary(targetRoot); err != nil {
		return err
	}
	err := os.Remove(RestartIntentPath(targetRoot))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func AppendLifecycleEvent(targetRoot string, event LifecycleEvent) (LifecycleEvent, error) {
	if strings.TrimSpace(event.ID) == "" {
		event.ID = newLifecycleEventID(time.Now().UTC())
	}
	event.Message = redactLifecycleText(event.Message)
	event.Details = redactLifecycleDetails(event.Details)
	if err := event.Validate(); err != nil {
		return LifecycleEvent{}, err
	}
	if err := ensureDaemonBoundary(targetRoot); err != nil {
		return LifecycleEvent{}, err
	}
	eventsDir := LifecycleEventsDir(targetRoot)
	if err := pathguard.EnsurePlainDirectory(targetRoot, eventsDir, 0o700); err != nil {
		return LifecycleEvent{}, err
	}
	path, err := LifecycleEventPath(targetRoot, event.ID)
	if err != nil {
		return LifecycleEvent{}, err
	}
	temp, err := os.CreateTemp(eventsDir, ".daemon-event-*.tmp")
	if err != nil {
		return LifecycleEvent{}, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := writeJSON(temp, event); err != nil {
		temp.Close()
		return LifecycleEvent{}, err
	}
	if err := temp.Close(); err != nil {
		return LifecycleEvent{}, err
	}
	if err := durable.PromoteFileNoReplace(tempName, path); err != nil {
		return LifecycleEvent{}, err
	}
	return event, nil
}

func ListLifecycleEvents(targetRoot string) ([]LifecycleEvent, error) {
	if err := ensureDaemonBoundary(targetRoot); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(LifecycleEventsDir(targetRoot))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	events := make([]LifecycleEvent, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("daemon event artifact path must not be a symlink: %s", filepath.Join(LifecycleEventsDir(targetRoot), name))
		}
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		path, err := LifecycleEventPath(targetRoot, id)
		if err != nil {
			return nil, err
		}
		event, err := readJSONFile[LifecycleEvent](targetRoot, path)
		if err != nil {
			return nil, err
		}
		if event.ID != id {
			return nil, fmt.Errorf("daemon event id %q does not match path id %q", event.ID, id)
		}
		events = append(events, event)
	}
	sortLifecycleEvents(events)
	return events, nil
}

func TailLifecycleEvents(targetRoot string, limit int) ([]LifecycleEvent, error) {
	events, err := ListLifecycleEvents(targetRoot)
	if err != nil {
		return nil, err
	}
	return tailLifecycleEvents(events, limit), nil
}

func BuildStatus(targetRoot, profileID, targetID string) (StatusReport, error) {
	if err := ensureDaemonBoundary(targetRoot); err != nil {
		return StatusReport{}, err
	}
	report := StatusReport{
		Version:   CurrentVersion,
		ProfileID: profileID,
		TargetID:  targetID,
		State:     StatusAbsent,
	}
	if install, err := ReadInstall(targetRoot); err == nil {
		if daemonArtifactInScope(install.ProfileID, install.TargetID, profileID, targetID) {
			report.Installed = true
			report.Install = &install
			report.State = StatusInstalled
			report.RunMode = install.RunMode
			report.ServiceManager = install.ServiceManager
		} else {
			report.ScopeIssues = append(report.ScopeIssues, "install_scope_mismatch")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return StatusReport{}, err
	}
	if state, err := ReadState(targetRoot); err == nil {
		if daemonArtifactInScope(state.ProfileID, state.TargetID, profileID, targetID) {
			report.StateRecord = &state
			report.State = state.Status
			report.RunMode = state.RunMode
			report.ServiceManager = state.ServiceManager
		} else {
			report.ScopeIssues = append(report.ScopeIssues, "state_scope_mismatch")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return StatusReport{}, err
	}
	if intent, err := ReadStopIntent(targetRoot); err == nil {
		if daemonArtifactInScope(intent.ProfileID, intent.TargetID, profileID, targetID) {
			report.StopIntent = &intent
		} else {
			report.ScopeIssues = append(report.ScopeIssues, "stop_intent_scope_mismatch")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return StatusReport{}, err
	}
	if intent, err := ReadRestartIntent(targetRoot); err == nil {
		if daemonArtifactInScope(intent.ProfileID, intent.TargetID, profileID, targetID) {
			report.RestartIntent = &intent
		} else {
			report.ScopeIssues = append(report.ScopeIssues, "restart_intent_scope_mismatch")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return StatusReport{}, err
	}
	events, err := ListLifecycleEvents(targetRoot)
	if err != nil {
		return StatusReport{}, err
	}
	scopedEvents := make([]LifecycleEvent, 0, len(events))
	foreignLifecycleEvent := false
	for _, event := range events {
		if daemonArtifactInScope(event.ProfileID, event.TargetID, profileID, targetID) {
			scopedEvents = append(scopedEvents, event)
		} else {
			foreignLifecycleEvent = true
		}
	}
	if foreignLifecycleEvent {
		report.ScopeIssues = append(report.ScopeIssues, "lifecycle_event_scope_mismatch")
	}
	report.LifecycleEvents = tailLifecycleEvents(scopedEvents, 10)
	if len(report.ScopeIssues) > 0 && report.State == StatusAbsent {
		report.State = StatusScopeMismatch
	}
	return report, nil
}

func ArtifactInScope(docProfileID, docTargetID, profileID, targetID string) bool {
	return daemonArtifactInScope(docProfileID, docTargetID, profileID, targetID)
}

func daemonArtifactInScope(docProfileID, docTargetID, profileID, targetID string) bool {
	return docProfileID == profileID && docTargetID == targetID
}

func StopSummary(intent StopIntent) StopIntentSummary {
	return StopIntentSummary{
		RequestedAt:    intent.RequestedAt,
		Reason:         intent.Reason,
		RequestedByPID: intent.RequestedByPID,
	}
}

func (d Install) Validate() error {
	var errs []error
	validateCommon(d.Version, d.ProfileID, d.TargetID, &errs)
	if strings.TrimSpace(d.ProfilePath) == "" {
		errs = append(errs, errors.New("profile_path is required"))
	}
	if _, err := time.Parse(time.RFC3339Nano, d.InstalledAt); err != nil {
		errs = append(errs, fmt.Errorf("installed_at must be RFC3339 timestamp: %w", err))
	}
	if d.RunMode != RunModeForeground {
		errs = append(errs, fmt.Errorf("run_mode must be %q", RunModeForeground))
	}
	if d.ServiceManager != ServiceManagerNone {
		errs = append(errs, fmt.Errorf("service_manager must be %q", ServiceManagerNone))
	}
	if len(d.Command) == 0 {
		errs = append(errs, errors.New("command is required"))
	}
	return errors.Join(errs...)
}

func (s State) Validate() error {
	var errs []error
	validateCommon(s.Version, s.ProfileID, s.TargetID, &errs)
	if strings.TrimSpace(s.ProfilePath) == "" {
		errs = append(errs, errors.New("profile_path is required"))
	}
	if !validState(s.Status) {
		errs = append(errs, errors.New("status must be one of starting, running, stopping, stopped, failed"))
	}
	if s.RunMode != RunModeForeground {
		errs = append(errs, fmt.Errorf("run_mode must be %q", RunModeForeground))
	}
	if s.ServiceManager != ServiceManagerNone {
		errs = append(errs, fmt.Errorf("service_manager must be %q", ServiceManagerNone))
	}
	if s.PID < 0 {
		errs = append(errs, errors.New("pid cannot be negative"))
	}
	if strings.TrimSpace(s.StartedAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, s.StartedAt); err != nil {
			errs = append(errs, fmt.Errorf("started_at must be RFC3339 timestamp: %w", err))
		}
	}
	if _, err := time.Parse(time.RFC3339Nano, s.UpdatedAt); err != nil {
		errs = append(errs, fmt.Errorf("updated_at must be RFC3339 timestamp: %w", err))
	}
	if strings.TrimSpace(s.StoppedAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, s.StoppedAt); err != nil {
			errs = append(errs, fmt.Errorf("stopped_at must be RFC3339 timestamp: %w", err))
		}
	}
	if s.StopIntent != nil && strings.TrimSpace(s.StopIntent.RequestedAt) != "" {
		if _, err := time.Parse(time.RFC3339Nano, s.StopIntent.RequestedAt); err != nil {
			errs = append(errs, fmt.Errorf("stop_intent.requested_at must be RFC3339 timestamp: %w", err))
		}
	}
	return errors.Join(errs...)
}

func (s StopIntent) Validate() error {
	var errs []error
	validateCommon(s.Version, s.ProfileID, s.TargetID, &errs)
	if _, err := time.Parse(time.RFC3339Nano, s.RequestedAt); err != nil {
		errs = append(errs, fmt.Errorf("requested_at must be RFC3339 timestamp: %w", err))
	}
	if s.RequestedByPID < 0 {
		errs = append(errs, errors.New("requested_by_pid cannot be negative"))
	}
	return errors.Join(errs...)
}

func (r RestartIntent) Validate() error {
	var errs []error
	validateCommon(r.Version, r.ProfileID, r.TargetID, &errs)
	if _, err := time.Parse(time.RFC3339Nano, r.RequestedAt); err != nil {
		errs = append(errs, fmt.Errorf("requested_at must be RFC3339 timestamp: %w", err))
	}
	if r.RequestedByPID < 0 {
		errs = append(errs, errors.New("requested_by_pid cannot be negative"))
	}
	return errors.Join(errs...)
}

func (e LifecycleEvent) Validate() error {
	var errs []error
	validateCommon(e.Version, e.ProfileID, e.TargetID, &errs)
	if strings.TrimSpace(e.ID) == "" {
		errs = append(errs, errors.New("id is required"))
	} else if err := control.ValidateArtifactID(e.ID); err != nil {
		errs = append(errs, fmt.Errorf("id is invalid: %w", err))
	}
	if strings.TrimSpace(e.Type) == "" {
		errs = append(errs, errors.New("type is required"))
	} else if err := control.ValidateArtifactID(e.Type); err != nil {
		errs = append(errs, fmt.Errorf("type is invalid: %w", err))
	}
	if _, err := time.Parse(time.RFC3339Nano, e.RecordedAt); err != nil {
		errs = append(errs, fmt.Errorf("recorded_at must be RFC3339 timestamp: %w", err))
	}
	for key := range e.Details {
		if forbiddenLifecycleDetailKey(key) {
			errs = append(errs, fmt.Errorf("details contains forbidden key %q", key))
		}
		if strings.TrimSpace(key) == "" {
			errs = append(errs, errors.New("details key is required"))
		}
	}
	return errors.Join(errs...)
}

func validateCommon(version int, profileID, targetID string, errs *[]error) {
	if version != CurrentVersion {
		*errs = append(*errs, fmt.Errorf("version must be %d", CurrentVersion))
	}
	if strings.TrimSpace(profileID) == "" {
		*errs = append(*errs, errors.New("profile_id is required"))
	}
	if strings.TrimSpace(targetID) == "" {
		*errs = append(*errs, errors.New("target_id is required"))
	}
}

func validState(state string) bool {
	switch state {
	case StatusStarting, StatusRunning, StatusStopping, StatusStopped, StatusFailed:
		return true
	default:
		return false
	}
}

func writeJSONFile[T interface{ Validate() error }](targetRoot, path string, doc T) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	if err := ensureDaemonBoundary(targetRoot); err != nil {
		return err
	}
	if err := pathguard.EnsurePlainDirectory(targetRoot, filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".daemon-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := writeJSON(temp, doc); err != nil {
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
	if err := durable.PromoteFile(tempName, path); err != nil {
		return err
	}
	return nil
}

func readJSONFile[T interface{ Validate() error }](targetRoot, path string) (T, error) {
	var doc T
	if err := ensureDaemonBoundary(targetRoot); err != nil {
		return doc, err
	}
	file, err := os.Open(path)
	if err != nil {
		return doc, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return doc, err
	}
	if !info.Mode().IsRegular() {
		return doc, fmt.Errorf("daemon artifact %q is not a regular file", path)
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return doc, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("unexpected trailing JSON document")
		}
		return doc, err
	}
	if err := doc.Validate(); err != nil {
		return doc, err
	}
	return doc, nil
}

func writeJSON[T interface{ Validate() error }](w io.Writer, doc T) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

func ensureDaemonBoundary(targetRoot string) error {
	controlDir := control.ControlDir(targetRoot)
	info, err := os.Lstat(controlDir)
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
	daemonDir := DaemonDir(targetRoot)
	info, err = os.Lstat(daemonDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect daemon control directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("daemon control directory must be a directory, not a symlink: %s", daemonDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("daemon control path must be a directory: %s", daemonDir)
	}
	for _, name := range []string{"install.json", "state.json", "stop-intent.json", "restart-intent.json"} {
		path := filepath.Join(daemonDir, name)
		info, err := os.Lstat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("inspect daemon artifact path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("daemon artifact path must not be a symlink: %s", path)
		}
	}
	eventsDir := LifecycleEventsDir(targetRoot)
	info, err = os.Lstat(eventsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect daemon events directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("daemon events directory must be a directory, not a symlink: %s", eventsDir)
	}
	if !info.IsDir() {
		return fmt.Errorf("daemon events path must be a directory: %s", eventsDir)
	}
	return nil
}

var lifecycleEventCounter uint64

func newLifecycleEventID(now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stamp := strings.ToLower(now.UTC().Format("20060102T150405.000000000Z"))
	stamp = strings.ReplaceAll(stamp, ".", "")
	counter := atomic.AddUint64(&lifecycleEventCounter, 1)
	return fmt.Sprintf("event-%s-p%d-%016x", stamp, os.Getpid(), counter)
}

func redactLifecycleDetails(details map[string]string) map[string]string {
	if len(details) == 0 {
		return nil
	}
	redacted := make(map[string]string, len(details))
	for key, value := range details {
		key = strings.TrimSpace(key)
		if key == "" || forbiddenLifecycleDetailKey(key) {
			continue
		}
		redacted[key] = redactLifecycleText(value)
	}
	if len(redacted) == 0 {
		return nil
	}
	return redacted
}

func redactLifecycleText(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	lower := strings.ToLower(value)
	for _, phrase := range []string{"raw stderr", "verification code", "pairing code", "pairing verification code"} {
		if strings.Contains(lower, phrase) {
			return "[redacted]"
		}
	}
	if strings.Contains(lower, "private") && strings.Contains(lower, "key") {
		return "[redacted]"
	}
	if strings.Contains(value, string(os.PathSeparator)) || strings.Contains(value, "\\") {
		return "[redacted]"
	}
	return value
}

func forbiddenLifecycleDetailKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	switch normalized {
	case "stderr", "raw_stderr", "verification_code", "pairing_code", "pairing_verification_code":
		return true
	default:
		return false
	}
}

func sortLifecycleEvents(events []LifecycleEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		left, leftErr := time.Parse(time.RFC3339Nano, events[i].RecordedAt)
		right, rightErr := time.Parse(time.RFC3339Nano, events[j].RecordedAt)
		if leftErr == nil && rightErr == nil && !left.Equal(right) {
			return left.Before(right)
		}
		return events[i].ID < events[j].ID
	})
}

func tailLifecycleEvents(events []LifecycleEvent, limit int) []LifecycleEvent {
	if limit <= 0 || len(events) == 0 {
		return nil
	}
	sortLifecycleEvents(events)
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	tail := make([]LifecycleEvent, len(events))
	copy(tail, events)
	return tail
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		value = time.Now().UTC()
	}
	return value.UTC().Format(time.RFC3339Nano)
}
