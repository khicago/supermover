package health

import (
	"os"
	"path/filepath"
	"strings"
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
	if len(got.Invalid) != 1 || got.Invalid[0].Path != badPath || got.Invalid[0].SessionID != "bad" {
		t.Fatalf("BuildReport(%q).Invalid = %#v, want bad record path %q", target, got.Invalid, badPath)
	}
}

func TestBuildReportMarksPublishedSessionMissingArtifactsUnhealthy(t *testing.T) {
	target := t.TempDir()
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecord(t, layout, "session-published", transaction.StatePublished)

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy {
		t.Fatalf("BuildReport(%q).Healthy = true, want false for missing published artifacts", target)
	}
	if got.Summary.ArtifactProblems != 2 {
		t.Fatalf("BuildReport(%q).Summary.ArtifactProblems = %d, want 2", target, got.Summary.ArtifactProblems)
	}
	if len(got.Artifacts) != 2 {
		t.Fatalf("BuildReport(%q).Artifacts = %#v, want manifest and receipt problems", target, got.Artifacts)
	}
}

func TestBuildReportMarksDamagedReviewArtifactsUnhealthy(t *testing.T) {
	target := t.TempDir()
	warningsDir := filepath.Join(control.ControlDir(target), "warnings")
	if err := os.MkdirAll(warningsDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", warningsDir, err)
	}
	warningPath := filepath.Join(warningsDir, "bad.json")
	if err := os.WriteFile(warningPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", warningPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy {
		t.Fatalf("BuildReport(%q).Healthy = true, want false for damaged warning artifact", target)
	}
	if got.Summary.ArtifactProblems != 1 || len(got.Artifacts) != 1 || got.Artifacts[0].Path != warningPath {
		t.Fatalf("BuildReport(%q).Artifacts = %#v summary=%+v, want one damaged warning artifact", target, got.Artifacts, got.Summary)
	}
}

func TestBuildReportMarksPublishedReceiptWithStagedSessionUnhealthy(t *testing.T) {
	target := t.TempDir()
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecord(t, layout, "session-crash", transaction.StateStaged)
	writeReceipt(t, target, "session-crash", "published")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy {
		t.Fatalf("BuildReport(%q).Healthy = true, want false for receipt/session mismatch", target)
	}
	if got.Summary.IncompleteSessions != 1 || got.Summary.ArtifactProblems == 0 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want incomplete session and artifact problem", target, got.Summary)
	}
	found := false
	for _, issue := range got.Artifacts {
		if issue.SessionID == "session-crash" && strings.Contains(issue.Error, "session state") && strings.Contains(issue.Error, "receipt status") {
			found = true
		}
	}
	if !found {
		t.Fatalf("BuildReport(%q).Artifacts = %#v, want receipt/session mismatch issue", target, got.Artifacts)
	}
	foundManifest := false
	for _, issue := range got.Artifacts {
		if issue.SessionID == "session-crash" && strings.HasSuffix(issue.Path, filepath.Join("session-crash", "manifest.json")) {
			foundManifest = true
		}
	}
	if !foundManifest {
		t.Fatalf("BuildReport(%q).Artifacts = %#v, want missing manifest issue for published receipt", target, got.Artifacts)
	}
}

func TestBuildReportMarksPartialControlArtifactsUnhealthy(t *testing.T) {
	target := t.TempDir()
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecord(t, layout, "session-partial", transaction.StateReceived)
	writeManifest(t, target, "session-partial")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy {
		t.Fatalf("BuildReport(%q).Healthy = true, want false for partial control artifacts", target)
	}
	found := false
	for _, issue := range got.Artifacts {
		if issue.SessionID == "session-partial" && strings.Contains(issue.Error, "non-staged session") {
			found = true
		}
	}
	if !found {
		t.Fatalf("BuildReport(%q).Artifacts = %#v, want partial control artifact issue", target, got.Artifacts)
	}
}

func TestBuildReportRejectsMissingTarget(t *testing.T) {
	_, err := BuildReport(Options{TargetRoot: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatalf("BuildReport(missing target) error = nil, want error")
	}
}

func writeReceipt(t *testing.T, target string, sessionID string, status string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	receipt := control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: "profile-local",
		TargetID:  "target-local",
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    status,
	}
	if err := control.WriteFile(path, receipt); err != nil {
		t.Fatalf("control.WriteFile(%q, receipt) error = %v, want nil", path, err)
	}
}

func writeManifest(t *testing.T, target string, sessionID string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactManifest, sessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v, want nil", err)
	}
	manifest := control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
	}
	if err := control.WriteFile(path, manifest); err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", path, err)
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
