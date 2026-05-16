package transaction

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClassifyRecoveryAction(t *testing.T) {
	tests := []struct {
		state      State
		wantAction RecoveryAction
	}{
		{state: StateReceived, wantAction: ActionRollback},
		{state: StateValidated, wantAction: ActionRollback},
		{state: StateStaged, wantAction: ActionRecover},
		{state: StatePublished, wantAction: ActionNone},
		{state: StateRolledBack, wantAction: ActionNone},
		{state: StateNeedsRepair, wantAction: ActionRepair},
		{state: State("future"), wantAction: ActionRepair},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			got, _ := ClassifyRecoveryAction(tt.state)
			if got != tt.wantAction {
				t.Errorf("ClassifyRecoveryAction(%q) = %q, want %q", tt.state, got, tt.wantAction)
			}
		})
	}
}

func TestScanRecoveryFindsIncompleteSessions(t *testing.T) {
	layout := NewLayout(t.TempDir())
	now := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)
	writeRecord(t, layout, "a-rollback", StateReceived, now)
	writeRecord(t, layout, "b-recover", StateStaged, now)
	writeRecord(t, layout, "c-published", StatePublished, now)
	writeRecord(t, layout, "d-repair", StateNeedsRepair, now)

	scan, err := ScanRecovery(layout)
	if err != nil {
		t.Fatalf("ScanRecovery(%+v) error = %v, want nil", layout, err)
	}

	got := map[string]RecoveryAction{}
	for _, item := range scan.Items {
		got[item.Record.ID] = item.Action
	}
	want := map[string]RecoveryAction{
		"a-rollback": ActionRollback,
		"b-recover":  ActionRecover,
		"d-repair":   ActionRepair,
	}
	if len(got) != len(want) {
		t.Fatalf("ScanRecovery(%+v) actions = %v, want %v", layout, got, want)
	}
	for id, wantAction := range want {
		if gotAction := got[id]; gotAction != wantAction {
			t.Errorf("ScanRecovery(%+v) action for %q = %q, want %q", layout, id, gotAction, wantAction)
		}
	}
	if len(scan.Invalid) != 0 {
		t.Errorf("ScanRecovery(%+v) invalid = %v, want empty", layout, scan.Invalid)
	}
}

func TestScanRecoveryReportsInvalidRecords(t *testing.T) {
	layout := NewLayout(t.TempDir())
	badDir := filepath.Join(layout.SessionsDir(), "bad")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", badDir, err)
	}
	badPath := filepath.Join(badDir, "session.json")
	if err := os.WriteFile(badPath, []byte(`{"id":"bad","state":"unknown"}`), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", badPath, err)
	}

	scan, err := ScanRecovery(layout)
	if err != nil {
		t.Fatalf("ScanRecovery(%+v) error = %v, want nil", layout, err)
	}
	if len(scan.Items) != 0 {
		t.Errorf("ScanRecovery(%+v) items = %v, want empty", layout, scan.Items)
	}
	if len(scan.Invalid) != 1 {
		t.Fatalf("ScanRecovery(%+v) invalid length = %d, want 1", layout, len(scan.Invalid))
	}
	if scan.Invalid[0].Path != badPath {
		t.Errorf("ScanRecovery(%+v) invalid path = %q, want %q", layout, scan.Invalid[0].Path, badPath)
	}
}

func writeRecord(t *testing.T, layout Layout, id string, state State, now time.Time) {
	t.Helper()
	record, err := NewSessionRecord(id, now)
	if err != nil {
		t.Fatalf("NewSessionRecord(%q, %v) error = %v, want nil", id, now, err)
	}
	record, err = record.WithState(state, now)
	if err != nil {
		t.Fatalf("SessionRecord.WithState(%q, %v) error = %v, want nil", state, now, err)
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		t.Fatalf("Layout.WriteSessionRecord(%+v) error = %v, want nil", record, err)
	}
}
