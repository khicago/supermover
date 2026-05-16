package health

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/transaction"
)

func TestBuildReportHealthyWhenNoIncompleteSessions(t *testing.T) {
	target := t.TempDir()

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !got.Healthy {
		t.Fatalf("BuildReport(%q).Healthy = false, want true", target)
	}
	if got.Summary.IncompleteSessions != 0 || got.Summary.InvalidRecords != 0 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want zero counts", target, got.Summary)
	}
}

func TestBuildReportListsRecoveryItemsAndInvalidRecords(t *testing.T) {
	target := t.TempDir()
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecord(t, layout, "session-recover", transaction.StateStaged)
	badPath := filepath.Join(layout.SessionsDir(), "bad", "session.json")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(badPath), err)
	}
	if err := os.WriteFile(badPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", badPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy {
		t.Fatalf("BuildReport(%q).Healthy = true, want false", target)
	}
	if got.Summary.IncompleteSessions != 1 || got.Summary.InvalidRecords != 1 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want one incomplete and one invalid", target, got.Summary)
	}
	if len(got.Items) != 1 || got.Items[0].SessionID != "session-recover" || got.Items[0].Action != string(transaction.ActionRecover) {
		t.Fatalf("BuildReport(%q).Items = %#v, want session-recover recover item", target, got.Items)
	}
	if len(got.Invalid) != 1 || got.Invalid[0].Path != badPath {
		t.Fatalf("BuildReport(%q).Invalid = %#v, want bad record path %q", target, got.Invalid, badPath)
	}
}

func TestBuildReportRejectsMissingTarget(t *testing.T) {
	_, err := BuildReport(Options{TargetRoot: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatalf("BuildReport(missing target) error = nil, want error")
	}
}

func writeRecord(t *testing.T, layout transaction.Layout, id string, state transaction.State) {
	t.Helper()
	now := time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC)
	record, err := transaction.NewSessionRecord(id, now)
	if err != nil {
		t.Fatalf("transaction.NewSessionRecord(%q) error = %v, want nil", id, err)
	}
	record, err = record.WithState(state, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("SessionRecord.WithState(%q) error = %v, want nil", state, err)
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		t.Fatalf("Layout.WriteSessionRecord(%+v) error = %v, want nil", record, err)
	}
}
