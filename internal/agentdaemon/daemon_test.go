package agentdaemon

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDaemonLifecycleArtifactsRoundTripAndStatus(t *testing.T) {
	target := t.TempDir()
	now := time.Date(2026, 5, 21, 1, 2, 3, 4, time.UTC)
	install := NewInstall("profile-local", "target-local", "/profiles/target.json", now)
	if err := WriteInstall(target, install); err != nil {
		t.Fatalf("WriteInstall() error = %v, want nil", err)
	}
	state := NewState("profile-local", "target-local", "/profiles/target.json", StatusRunning, 123, now)
	state.Mode = "pairing-only"
	state.PairingAddress = "127.0.0.1:9000"
	if err := WriteState(target, state); err != nil {
		t.Fatalf("WriteState() error = %v, want nil", err)
	}
	intent := NewStopIntent("profile-local", "target-local", "operator stop", 456, now)
	if err := WriteStopIntent(target, intent); err != nil {
		t.Fatalf("WriteStopIntent() error = %v, want nil", err)
	}
	restart := NewRestartIntent("profile-local", "target-local", "operator restart", 789, now)
	if err := WriteRestartIntent(target, restart); err != nil {
		t.Fatalf("WriteRestartIntent() error = %v, want nil", err)
	}
	event, err := AppendLifecycleEvent(target, NewLifecycleEvent(
		"profile-local",
		"target-local",
		"daemon_started",
		"pairing verification code 123456",
		map[string]string{"mode": "pairing-only", "stderr": "raw stderr must not persist", "pairing_code": "123456"},
		now,
	))
	if err != nil {
		t.Fatalf("AppendLifecycleEvent() error = %v, want nil", err)
	}

	report, err := BuildStatus(target, "profile-local", "target-local")
	if err != nil {
		t.Fatalf("BuildStatus() error = %v, want nil", err)
	}
	if !report.Installed || report.State != StatusRunning || report.RunMode != RunModeForeground || report.ServiceManager != ServiceManagerNone {
		t.Fatalf("BuildStatus() = %+v, want installed running foreground status", report)
	}
	if report.StateRecord == nil || report.StateRecord.PairingAddress != "127.0.0.1:9000" {
		t.Fatalf("BuildStatus().StateRecord = %+v, want persisted state", report.StateRecord)
	}
	if report.StopIntent == nil || report.StopIntent.Reason != "operator stop" || report.StopIntent.RequestedByPID != 456 {
		t.Fatalf("BuildStatus().StopIntent = %+v, want persisted stop intent", report.StopIntent)
	}
	if report.RestartIntent == nil || report.RestartIntent.Reason != "operator restart" || report.RestartIntent.RequestedByPID != 789 {
		t.Fatalf("BuildStatus().RestartIntent = %+v, want persisted restart intent", report.RestartIntent)
	}
	if len(report.LifecycleEvents) != 1 || report.LifecycleEvents[0].ID != event.ID || report.LifecycleEvents[0].Message != "[redacted]" || report.LifecycleEvents[0].Details["stderr"] != "" || report.LifecycleEvents[0].Details["pairing_code"] != "" {
		t.Fatalf("BuildStatus().LifecycleEvents = %+v, want one scoped redacted event", report.LifecycleEvents)
	}
}

func TestBuildStatusDoesNotTrustForeignDaemonArtifacts(t *testing.T) {
	target := t.TempDir()
	now := time.Date(2026, 5, 21, 1, 2, 3, 4, time.UTC)
	if err := WriteInstall(target, NewInstall("profile-other", "target-local", "/profiles/other.json", now)); err != nil {
		t.Fatalf("WriteInstall(foreign) error = %v, want nil", err)
	}
	if err := WriteState(target, NewState("profile-local", "target-other", "/profiles/target.json", StatusRunning, 123, now)); err != nil {
		t.Fatalf("WriteState(foreign) error = %v, want nil", err)
	}
	if err := WriteStopIntent(target, NewStopIntent("profile-other", "target-other", "foreign stop", 456, now)); err != nil {
		t.Fatalf("WriteStopIntent(foreign) error = %v, want nil", err)
	}
	if err := WriteRestartIntent(target, NewRestartIntent("profile-other", "target-other", "foreign restart", 789, now)); err != nil {
		t.Fatalf("WriteRestartIntent(foreign) error = %v, want nil", err)
	}
	if _, err := AppendLifecycleEvent(target, NewLifecycleEvent("profile-other", "target-other", "daemon_started", "foreign event", nil, now)); err != nil {
		t.Fatalf("AppendLifecycleEvent(foreign) error = %v, want nil", err)
	}

	report, err := BuildStatus(target, "profile-local", "target-local")
	if err != nil {
		t.Fatalf("BuildStatus() error = %v, want nil", err)
	}

	if report.Installed || report.Install != nil || report.StateRecord != nil || report.StopIntent != nil || report.RestartIntent != nil || len(report.LifecycleEvents) != 0 {
		t.Fatalf("BuildStatus() = %+v, want foreign artifacts excluded from trusted status evidence", report)
	}
	if report.State != StatusScopeMismatch {
		t.Fatalf("BuildStatus().State = %q, want %q for only foreign artifacts", report.State, StatusScopeMismatch)
	}
	want := []string{"install_scope_mismatch", "state_scope_mismatch", "stop_intent_scope_mismatch", "restart_intent_scope_mismatch", "lifecycle_event_scope_mismatch"}
	if len(report.ScopeIssues) != len(want) {
		t.Fatalf("BuildStatus().ScopeIssues = %#v, want %#v", report.ScopeIssues, want)
	}
	for i, issue := range want {
		if report.ScopeIssues[i] != issue {
			t.Fatalf("BuildStatus().ScopeIssues = %#v, want %#v", report.ScopeIssues, want)
		}
	}
}

func TestBuildStatusRequiresExactScopeIdentity(t *testing.T) {
	target := t.TempDir()
	now := time.Date(2026, 5, 21, 1, 2, 3, 4, time.UTC)
	if err := WriteInstall(target, NewInstall("profile-local ", "target-local", "/profiles/other.json", now)); err != nil {
		t.Fatalf("WriteInstall(space suffix) error = %v, want nil", err)
	}
	if err := WriteRestartIntent(target, NewRestartIntent("profile-local", "target-local ", "space suffix", 7, now)); err != nil {
		t.Fatalf("WriteRestartIntent(space suffix) error = %v, want nil", err)
	}
	if _, err := AppendLifecycleEvent(target, NewLifecycleEvent("profile-local ", "target-local", "daemon_started", "space suffix", nil, now)); err != nil {
		t.Fatalf("AppendLifecycleEvent(space suffix) error = %v, want nil", err)
	}

	report, err := BuildStatus(target, "profile-local", "target-local")
	if err != nil {
		t.Fatalf("BuildStatus() error = %v, want nil", err)
	}
	if report.Installed || report.Install != nil || report.RestartIntent != nil || len(report.LifecycleEvents) != 0 {
		t.Fatalf("BuildStatus() = %+v, want space-suffixed profile_id treated as out of scope", report)
	}
	want := []string{"install_scope_mismatch", "restart_intent_scope_mismatch", "lifecycle_event_scope_mismatch"}
	if len(report.ScopeIssues) != len(want) {
		t.Fatalf("BuildStatus().ScopeIssues = %#v, want exact scope mismatch", report.ScopeIssues)
	}
	for i, issue := range want {
		if report.ScopeIssues[i] != issue {
			t.Fatalf("BuildStatus().ScopeIssues = %#v, want %#v", report.ScopeIssues, want)
		}
	}
}

func TestRemoveStopIntentIsIdempotent(t *testing.T) {
	target := t.TempDir()
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	if err := WriteStopIntent(target, NewStopIntent("profile-local", "target-local", "", 1, now)); err != nil {
		t.Fatalf("WriteStopIntent() error = %v, want nil", err)
	}

	if err := RemoveStopIntent(target); err != nil {
		t.Fatalf("RemoveStopIntent(first) error = %v, want nil", err)
	}
	if err := RemoveStopIntent(target); err != nil {
		t.Fatalf("RemoveStopIntent(second) error = %v, want nil", err)
	}
	if _, err := os.Lstat(StopIntentPath(target)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat(stop intent) error = %v, want not exist", err)
	}
}

func TestRemoveRestartIntentIsIdempotent(t *testing.T) {
	target := t.TempDir()
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	if err := WriteRestartIntent(target, NewRestartIntent("profile-local", "target-local", "", 1, now)); err != nil {
		t.Fatalf("WriteRestartIntent() error = %v, want nil", err)
	}

	if err := RemoveRestartIntent(target); err != nil {
		t.Fatalf("RemoveRestartIntent(first) error = %v, want nil", err)
	}
	if err := RemoveRestartIntent(target); err != nil {
		t.Fatalf("RemoveRestartIntent(second) error = %v, want nil", err)
	}
	if _, err := os.Lstat(RestartIntentPath(target)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat(restart intent) error = %v, want not exist", err)
	}
}

func TestDaemonArtifactsRejectSymlinkSurface(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(target, ".supermover"), 0o755); err != nil {
		t.Fatalf("MkdirAll(control) error = %v, want nil", err)
	}
	if err := os.Symlink(outside, DaemonDir(target)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	err := WriteState(target, NewState("profile-local", "target-local", "/profiles/target.json", StatusRunning, 123, time.Now()))
	if err == nil {
		t.Fatal("WriteState() error = nil, want symlink surface refusal")
	}
}

func TestDaemonArtifactsRejectSymlinkRestartIntentAndEventsSurfaces(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	if err := WriteInstall(target, NewInstall("profile-local", "target-local", "/profiles/target.json", now)); err != nil {
		t.Fatalf("WriteInstall() error = %v, want nil", err)
	}
	if err := os.Remove(InstallPath(target)); err != nil {
		t.Fatalf("Remove(install) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "restart-intent.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(outside restart) error = %v, want nil", err)
	}
	if err := os.Symlink(filepath.Join(outside, "restart-intent.json"), RestartIntentPath(target)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := WriteRestartIntent(target, NewRestartIntent("profile-local", "target-local", "", 1, now)); err == nil {
		t.Fatal("WriteRestartIntent() error = nil, want symlink artifact refusal")
	}
	if err := os.Remove(RestartIntentPath(target)); err != nil {
		t.Fatalf("Remove(restart symlink) error = %v, want nil", err)
	}
	if err := os.Symlink(outside, LifecycleEventsDir(target)); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := AppendLifecycleEvent(target, NewLifecycleEvent("profile-local", "target-local", "daemon_started", "started", nil, now)); err == nil {
		t.Fatal("AppendLifecycleEvent() error = nil, want symlink events dir refusal")
	}
}

func TestLifecycleEventsTailOrderingAndLimit(t *testing.T) {
	target := t.TempDir()
	base := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	events := []LifecycleEvent{
		NewLifecycleEvent("profile-local", "target-local", "daemon_started", "first", nil, base.Add(2*time.Second)),
		NewLifecycleEvent("profile-local", "target-local", "daemon_ready", "second", nil, base.Add(time.Second)),
		NewLifecycleEvent("profile-local", "target-local", "daemon_stopped", "third", nil, base.Add(3*time.Second)),
	}
	for i := range events {
		events[i].ID = []string{"event-c", "event-b", "event-d"}[i]
		if _, err := AppendLifecycleEvent(target, events[i]); err != nil {
			t.Fatalf("AppendLifecycleEvent(%d) error = %v, want nil", i, err)
		}
	}

	list, err := ListLifecycleEvents(target)
	if err != nil {
		t.Fatalf("ListLifecycleEvents() error = %v, want nil", err)
	}
	if got := eventMessages(list); len(got) != 3 || got[0] != "second" || got[1] != "first" || got[2] != "third" {
		t.Fatalf("ListLifecycleEvents() messages = %#v, want recorded_at ordering", got)
	}
	tail, err := TailLifecycleEvents(target, 2)
	if err != nil {
		t.Fatalf("TailLifecycleEvents() error = %v, want nil", err)
	}
	if got := eventMessages(tail); len(got) != 2 || got[0] != "first" || got[1] != "third" {
		t.Fatalf("TailLifecycleEvents(limit=2) messages = %#v, want newest two in order", got)
	}
	empty, err := TailLifecycleEvents(target, 0)
	if err != nil {
		t.Fatalf("TailLifecycleEvents(limit=0) error = %v, want nil", err)
	}
	if len(empty) != 0 {
		t.Fatalf("TailLifecycleEvents(limit=0) = %+v, want empty", empty)
	}
}

func eventMessages(events []LifecycleEvent) []string {
	messages := make([]string, 0, len(events))
	for _, event := range events {
		messages = append(messages, event.Message)
	}
	return messages
}
