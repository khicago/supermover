package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/health"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/report"
	"github.com/khicago/supermover/internal/status"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/verify"
)

func TestVerifyJSONReturnsReviewExitAndMachineReadableMissingEvidence(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-missing"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if err := os.Remove(filepath.Join(target, "file.txt")); err != nil {
		t.Fatalf("os.Remove(target file) error = %v, want nil", err)
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"verify", "--profile", profilePath, "--session", "session-missing", "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("verify JSON missing target exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var gotReport verify.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(verify stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Summary.ErrorFindings != 1 || len(gotReport.Findings) != 1 {
		t.Fatalf("verify JSON summary=%+v findings=%#v, want one review finding", gotReport.Summary, gotReport.Findings)
	}
	finding := gotReport.Findings[0]
	if finding.Kind != verify.FindingMissingFile || finding.Severity != verify.SeverityError || finding.SessionID != "session-missing" || finding.TargetPath != "file.txt" {
		t.Fatalf("verify JSON finding = %+v, want durable missing-file evidence for session target path", finding)
	}
	if stderr.Len() != 0 {
		t.Fatalf("verify JSON missing target stderr = %q, want empty", stderr.String())
	}
}

func TestReportJSONReturnsReviewExitAndMachineReadableDivergenceEvidence(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-divergent"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(target, "file.txt"), "changed")
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"report", "--profile", profilePath, "--session", "session-divergent", "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("report JSON divergent target exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var gotReport report.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(report stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Overall.Status != report.StatusFailed || !gotReport.NeedsReview() {
		t.Fatalf("report JSON overall=%+v, want verification failure requiring review", gotReport.Overall)
	}
	if gotReport.Summary.VerificationErrors == 0 || len(gotReport.VerificationFindings) == 0 {
		t.Fatalf("report JSON summary=%+v findings=%#v, want machine-readable verification findings", gotReport.Summary, gotReport.VerificationFindings)
	}
	var hasDigestMismatch bool
	for _, finding := range gotReport.VerificationFindings {
		if finding.Kind == verify.FindingDigestMismatch && finding.Severity == verify.SeverityError && finding.SessionID == "session-divergent" && finding.TargetPath == "file.txt" {
			hasDigestMismatch = true
		}
	}
	if !hasDigestMismatch {
		t.Fatalf("report JSON verification findings = %+v, want digest divergence evidence for session target path", gotReport.VerificationFindings)
	}
	if gotReport.LatestSession.Completeness.Status != report.CompletenessFailed {
		t.Fatalf("report JSON completeness=%+v, want verification_failed", gotReport.LatestSession.Completeness)
	}
	if stderr.Len() != 0 {
		t.Fatalf("report JSON divergent target stderr = %q, want empty", stderr.String())
	}
}

func TestVerifyReturnsReviewExitForPersistedWarningAndSoftDeleteArtifacts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "keep.txt"), "keep")
	mustWrite(t, filepath.Join(source, "gone.txt"), "gone")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("first push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if err := os.Remove(filepath.Join(source, "gone.txt")); err != nil {
		t.Fatalf("os.Remove(source gone) error = %v, want nil", err)
	}
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-two"}, &stdout, &stderr); got != 0 {
		t.Fatalf("second push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	writeReviewWarning(t, target, "session-two")
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"verify", "--profile", profilePath, "--session", "session-two", "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("verify review artifacts exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var gotReport verify.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(verify stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Summary.Warnings != 1 || gotReport.Summary.SoftDeletes != 1 {
		t.Fatalf("verify JSON summary=%+v, want persisted warning and soft-delete review evidence", gotReport.Summary)
	}
	if len(gotReport.Warnings) != 1 || gotReport.Warnings[0].ID != "session-two-001-extra-config" {
		t.Fatalf("verify JSON warnings=%#v, want persisted warning evidence", gotReport.Warnings)
	}
	if len(gotReport.SoftDeletes) != 1 || gotReport.SoftDeletes[0].TargetPath != "gone.txt" {
		t.Fatalf("verify JSON soft_deletes=%#v, want inspectable soft-delete evidence", gotReport.SoftDeletes)
	}
	if stderr.Len() != 0 {
		t.Fatalf("verify review artifacts stderr = %q, want empty", stderr.String())
	}
}

func TestReportJSONShowsWarningsSuggestionsAndSoftDeletes(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "keep.txt"), "keep")
	mustWrite(t, filepath.Join(source, "gone.txt"), "gone")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("first push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if err := os.Remove(filepath.Join(source, "gone.txt")); err != nil {
		t.Fatalf("os.Remove(source gone) error = %v, want nil", err)
	}
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-two"}, &stdout, &stderr); got != 0 {
		t.Fatalf("second push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	writeReviewWarning(t, target, "session-two")
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"report", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("report review artifacts exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var gotReport report.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(report stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Overall.Status != report.StatusAttention || !gotReport.NeedsReview() {
		t.Fatalf("report JSON overall=%+v, want review-required attention status", gotReport.Overall)
	}
	if gotReport.Summary.Warnings != 1 || gotReport.Summary.ProfileSuggestions != 1 || gotReport.Summary.SoftDeletes != 1 {
		t.Fatalf("report JSON summary=%+v, want warning, suggestion, and soft-delete counts", gotReport.Summary)
	}
	if len(gotReport.ProfileSuggestions) != 1 || gotReport.ProfileSuggestions[0].SuggestedProfilePatch["include.needs_extra"] != "true" {
		t.Fatalf("report JSON profile_suggestions=%#v, want durable suggested profile patch", gotReport.ProfileSuggestions)
	}
	if len(gotReport.SoftDeletes) != 1 || gotReport.SoftDeletes[0].PreviousSessionID != "session-one" || gotReport.SoftDeletes[0].TargetPath != "gone.txt" {
		t.Fatalf("report JSON soft_deletes=%#v, want previous-session deletion evidence", gotReport.SoftDeletes)
	}
	if stderr.Len() != 0 {
		t.Fatalf("report review artifacts stderr = %q, want empty", stderr.String())
	}
}

func TestDriftListCleanTargetReturnsSuccess(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-clean"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "list", "--profile", profilePath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("drift list clean exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{"drift: target=", "session=session-clean", "manifests=1", "entries=1", "target_drifts=0", "artifact_problems=0"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("drift list clean stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list clean stderr = %q, want empty", stderr.String())
	}
}

func TestDriftListJSONCleanTargetReturnsSuccess(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-clean-json"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "list", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("drift list clean JSON exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var gotReport verify.DriftReport
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(drift clean stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.SessionID != "session-clean-json" || gotReport.Summary.TargetDrifts != 0 || gotReport.Summary.ArtifactProblems != 0 || len(gotReport.Drifts) != 0 {
		t.Fatalf("drift list clean JSON report=%+v drifts=%#v, want clean selected session", gotReport.Summary, gotReport.Drifts)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list clean JSON stderr = %q, want empty", stderr.String())
	}
}

func TestDriftListJSONReturnsReviewExitAndMachineReadableTargetDrift(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "old")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-drift"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(target, "file.txt"), "manual target edit")
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "list", "--profile", profilePath, "--session", "session-drift", "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list JSON drift exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var gotReport verify.DriftReport
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(drift stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.SessionID != "session-drift" || gotReport.Summary.TargetDrifts != 1 || len(gotReport.Drifts) != 1 {
		t.Fatalf("drift list JSON report=%+v drifts=%#v, want one session-drift target drift", gotReport.Summary, gotReport.Drifts)
	}
	drift := gotReport.Drifts[0]
	if drift.Path != "file.txt" || drift.Change != "content_mismatch" || drift.Expected.SessionID != "session-drift" || drift.Observed.Path != "file.txt" || drift.ReviewState != "needs_review" {
		t.Fatalf("drift list JSON drift = %#v, want structured target-local content mismatch evidence", drift)
	}
	if !strings.Contains(stdout.String(), `"target_drifts"`) {
		t.Fatalf("drift list JSON stdout = %q, want target_drifts field", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list JSON drift stderr = %q, want empty", stderr.String())
	}
}

func TestDriftListDoesNotPersistDetectorOutput(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "old")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-readonly"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(target, "file.txt"), "manual target edit")
	controlDir := filepath.Join(target, control.DirName)
	before := snapshotTree(t, controlDir)
	if _, err := os.Stat(filepath.Join(controlDir, "drift")); !os.IsNotExist(err) {
		t.Fatalf("Stat(control drift dir) error = %v, want os.ErrNotExist before read-only drift list", err)
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "list", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list read-only exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var gotReport verify.DriftReport
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(read-only drift stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Summary.TargetDrifts != 1 {
		t.Fatalf("drift list read-only summary=%+v drifts=%#v, want one live detector drift", gotReport.Summary, gotReport.Drifts)
	}
	after := snapshotTree(t, controlDir)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("drift list changed control plane\nbefore=%#v\nafter=%#v", before, after)
	}
	if _, err := os.Stat(filepath.Join(controlDir, "drift")); !os.IsNotExist(err) {
		t.Fatalf("Stat(control drift dir) error = %v, want os.ErrNotExist after read-only drift list", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list read-only stderr = %q, want empty", stderr.String())
	}
}

func TestDriftRecordPersistsLiveDetectorOutput(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "old")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(target, "file.txt"), "manual target edit")
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "record", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift record exit = %d, stderr = %q, stdout = %q, want 1 for recorded review evidence", got, stderr.String(), stdout.String())
	}
	var result struct {
		Detected int `json:"detected"`
		Recorded int `json:"recorded"`
		Existing int `json:"existing"`
		Records  []struct {
			ID          string `json:"id"`
			Path        string `json:"path"`
			Change      string `json:"change"`
			SessionID   string `json:"session_id"`
			ReviewState string `json:"review_state"`
			Recorded    bool   `json:"recorded"`
			Existing    bool   `json:"existing"`
		} `json:"records"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(drift record stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if result.Detected != 1 || result.Recorded != 1 || result.Existing != 0 || len(result.Records) != 1 {
		t.Fatalf("drift record result = %+v, want one newly recorded drift", result)
	}
	record := result.Records[0]
	if !strings.HasPrefix(record.ID, "detected_") || record.Path != "file.txt" || record.Change != "content_mismatch" || record.SessionID != "session-one" || record.ReviewState != "needs_review" || !record.Recorded || record.Existing {
		t.Fatalf("drift record item = %+v, want durable live content mismatch evidence", record)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift record stderr = %q, want empty", stderr.String())
	}
	persisted, err := control.ReadFile[control.TargetDrift](targetDriftArtifactPath(t, target, record.ID))
	if err != nil {
		t.Fatalf("control.ReadFile(persisted drift) error = %v, want nil", err)
	}
	if persisted.ID != record.ID || persisted.ReviewState != "needs_review" || persisted.Expected.SessionID != "session-one" || persisted.Observed.Digest == "" {
		t.Fatalf("persisted drift = %+v, want live detector evidence persisted as needs_review", persisted)
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"report", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("report after drift record exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var gotReport report.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(report after drift record) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Summary.TargetDrifts != 1 || len(gotReport.TargetDrifts) != 1 || gotReport.TargetDrifts[0].ID != record.ID {
		t.Fatalf("report after drift record target_drifts=%+v/%#v, want persisted drift", gotReport.Summary, gotReport.TargetDrifts)
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"status", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("status after drift record exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var gotStatus status.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotStatus); err != nil {
		t.Fatalf("json.Unmarshal(status after drift record) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotStatus.Counts.TargetDrifts != 1 {
		t.Fatalf("status after drift record counts = %+v, want persisted target drift count", gotStatus.Counts)
	}
}

func TestDriftRecordEmptyTargetRequiresReviewWithoutWritingRecords(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"drift", "record", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift record empty target exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var result struct {
		ManifestCount int `json:"manifest_count"`
		Detected      int `json:"detected"`
		Recorded      int `json:"recorded"`
		Existing      int `json:"existing"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(drift record empty target) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if result.ManifestCount != 0 || result.Detected != 0 || result.Recorded != 0 || result.Existing != 0 {
		t.Fatalf("drift record empty target = %+v, want missing-manifest review without records", result)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "drift")); !os.IsNotExist(err) {
		t.Fatalf("Stat(control drift dir) error = %v, want no drift records written for empty target", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift record empty target stderr = %q, want empty", stderr.String())
	}
}

func TestDriftRecordMissingTargetRequiresReviewWithoutCreatingTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "missing-target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"drift", "record", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift record missing target exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var result struct {
		ManifestCount int `json:"manifest_count"`
		Detected      int `json:"detected"`
		Recorded      int `json:"recorded"`
		Existing      int `json:"existing"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(drift record missing target) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if result.ManifestCount != 0 || result.Detected != 0 || result.Recorded != 0 || result.Existing != 0 {
		t.Fatalf("drift record missing target = %+v, want missing-manifest review without records", result)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("Stat(missing target) error = %v, want target not created", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift record missing target stderr = %q, want empty", stderr.String())
	}
}

func TestDriftRecordIsIdempotentAndDoesNotSuppressLiveDetector(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "old")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(target, "file.txt"), "manual target edit")
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"drift", "record", "--profile", profilePath, "--format", "json"}, &stdout, &stderr); got != 1 {
		t.Fatalf("first drift record exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var first struct {
		Records []struct {
			ID string `json:"id"`
		} `json:"records"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &first); err != nil {
		t.Fatalf("json.Unmarshal(first drift record) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if len(first.Records) != 1 {
		t.Fatalf("first drift record = %+v, want one record", first)
	}
	id := first.Records[0].ID

	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"drift", "acknowledge", "--profile", profilePath, "--id", id, "--reason", "reviewed live record"}, &stdout, &stderr); got != 0 {
		t.Fatalf("drift acknowledge recorded live drift exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	got := Run([]string{"drift", "record", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("second drift record exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var second struct {
		Recorded int `json:"recorded"`
		Existing int `json:"existing"`
		Records  []struct {
			ID          string `json:"id"`
			ReviewState string `json:"review_state"`
			Recorded    bool   `json:"recorded"`
			Existing    bool   `json:"existing"`
		} `json:"records"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &second); err != nil {
		t.Fatalf("json.Unmarshal(second drift record) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if second.Recorded != 0 || second.Existing != 1 || len(second.Records) != 1 || second.Records[0].ID != id || second.Records[0].ReviewState != "acknowledged" || second.Records[0].Recorded || !second.Records[0].Existing {
		t.Fatalf("second drift record = %+v, want existing acknowledged record preserved", second)
	}
	persisted, err := control.ReadFile[control.TargetDrift](targetDriftArtifactPath(t, target, id))
	if err != nil {
		t.Fatalf("control.ReadFile(persisted drift) error = %v, want nil", err)
	}
	if persisted.ReviewState != "acknowledged" || persisted.ReviewReason != "reviewed live record" {
		t.Fatalf("persisted drift after second record = %+v, want acknowledgement preserved", persisted)
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"drift", "list", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("drift list after acknowledge exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var live verify.DriftReport
	if err := json.Unmarshal(stdout.Bytes(), &live); err != nil {
		t.Fatalf("json.Unmarshal(live drift after acknowledge) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if live.Summary.TargetDrifts != 1 || len(live.Drifts) != 1 || live.Drifts[0].ID != id {
		t.Fatalf("drift list after acknowledge = %+v drifts=%#v, want live detector still reporting target divergence", live.Summary, live.Drifts)
	}
	if live.Drifts[0].ReviewState != "needs_review" || live.Drifts[0].Path != "file.txt" || live.Drifts[0].Change != "content_mismatch" {
		t.Fatalf("drift list after acknowledge drift = %+v, want fresh unreviewed live evidence", live.Drifts[0])
	}
}

func TestDriftRecordCreatesNewRecordWhenObservedEvidenceChanges(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "old")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(target, "file.txt"), "manual target edit")
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"drift", "record", "--profile", profilePath, "--format", "json"}, &stdout, &stderr); got != 1 {
		t.Fatalf("first drift record exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var first struct {
		Records []struct {
			ID string `json:"id"`
		} `json:"records"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &first); err != nil {
		t.Fatalf("json.Unmarshal(first drift record) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if len(first.Records) != 1 {
		t.Fatalf("first drift record = %+v, want one record", first)
	}
	firstID := first.Records[0].ID

	mustWrite(t, filepath.Join(target, "file.txt"), "different target edit")
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"drift", "record", "--profile", profilePath, "--format", "json"}, &stdout, &stderr); got != 1 {
		t.Fatalf("second drift record exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var second struct {
		Recorded int `json:"recorded"`
		Existing int `json:"existing"`
		Records  []struct {
			ID string `json:"id"`
		} `json:"records"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &second); err != nil {
		t.Fatalf("json.Unmarshal(second drift record) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if second.Recorded != 1 || second.Existing != 0 || len(second.Records) != 1 || second.Records[0].ID == firstID {
		t.Fatalf("second drift record = %+v, want one new record for changed observed evidence", second)
	}
	firstPersisted, err := control.ReadFile[control.TargetDrift](targetDriftArtifactPath(t, target, firstID))
	if err != nil {
		t.Fatalf("control.ReadFile(first persisted drift) error = %v, want nil", err)
	}
	secondPersisted, err := control.ReadFile[control.TargetDrift](targetDriftArtifactPath(t, target, second.Records[0].ID))
	if err != nil {
		t.Fatalf("control.ReadFile(second persisted drift) error = %v, want nil", err)
	}
	if firstPersisted.Observed.Digest == secondPersisted.Observed.Digest || secondPersisted.Observed.Digest != testDigest([]byte("different target edit")) {
		t.Fatalf("persisted observed digests first=%q second=%q, want separate current evidence", firstPersisted.Observed.Digest, secondPersisted.Observed.Digest)
	}
}

func TestDriftRecordHistoricalSessionDoesNotIncludeLatestExtraPaths(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("first push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-two"}, &stdout, &stderr); got != 0 {
		t.Fatalf("second push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(target, "extra.txt"), "target-only")
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "record", "--profile", profilePath, "--session", "session-one", "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("historical drift record exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var historical struct {
		SessionID     string `json:"session_id"`
		ManifestCount int    `json:"manifest_count"`
		Detected      int    `json:"detected"`
		Recorded      int    `json:"recorded"`
		Existing      int    `json:"existing"`
		Records       []struct {
			Path string `json:"path"`
		} `json:"records"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &historical); err != nil {
		t.Fatalf("json.Unmarshal(historical drift record) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if historical.SessionID != "session-one" || historical.ManifestCount != 1 || historical.Detected != 0 || historical.Recorded != 0 || historical.Existing != 0 || len(historical.Records) != 0 {
		t.Fatalf("historical drift record = %+v, want selected clean historical manifest without latest extra drift", historical)
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "drift")); !os.IsNotExist(err) {
		t.Fatalf("Stat(control drift dir) error = %v, want no record writes for clean historical session", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("historical drift record stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"drift", "record", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("latest drift record exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var latest struct {
		SessionID string `json:"session_id"`
		Detected  int    `json:"detected"`
		Recorded  int    `json:"recorded"`
		Records   []struct {
			ID       string `json:"id"`
			Path     string `json:"path"`
			Change   string `json:"change"`
			Existing bool   `json:"existing"`
		} `json:"records"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &latest); err != nil {
		t.Fatalf("json.Unmarshal(latest drift record) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if latest.SessionID != "session-two" || latest.Detected != 1 || latest.Recorded != 1 || len(latest.Records) != 1 || latest.Records[0].Path != "extra.txt" || latest.Records[0].Change != "extra" || latest.Records[0].Existing {
		t.Fatalf("latest drift record = %+v, want latest extra path persisted", latest)
	}
	if _, err := control.ReadFile[control.TargetDrift](targetDriftArtifactPath(t, target, latest.Records[0].ID)); err != nil {
		t.Fatalf("control.ReadFile(latest extra drift) error = %v, want nil", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("latest drift record stderr = %q, want empty", stderr.String())
	}
}

func TestDriftAcknowledgeJSONUpdatesPersistedRecord(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)
	drift := cliTargetDrift("drift-ack")
	writeEmptyPublishedSessionForCLI(t, target, drift.SessionID)
	writeTargetDriftArtifact(t, target, drift)

	runner := Runner{Now: time.Date(2026, 5, 19, 10, 11, 12, 0, time.UTC)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := runner.Run([]string{"drift", "acknowledge", "--profile", profilePath, "--id", drift.ID, "--reason", "accepted operator target edit", "--reviewer", "ops", "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("drift acknowledge exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var result struct {
		ID            string `json:"id"`
		Path          string `json:"path"`
		PreviousState string `json:"previous_state"`
		ReviewState   string `json:"review_state"`
		ReviewedAt    string `json:"reviewed_at"`
		Reviewer      string `json:"reviewer"`
		Reason        string `json:"reason"`
		ProfileID     string `json:"profile_id"`
		TargetID      string `json:"target_id"`
		SessionID     string `json:"session_id"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(drift acknowledge stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if result.ID != drift.ID || result.Path != drift.Path || result.PreviousState != "needs_review" || result.ReviewState != "acknowledged" || result.ReviewedAt != "2026-05-19T10:11:12Z" || result.Reviewer != "ops" || result.Reason != "accepted operator target edit" || result.ProfileID != "profile-local" || result.TargetID != "local:profile-local" || result.SessionID != drift.SessionID {
		t.Fatalf("drift acknowledge JSON = %+v, want durable review evidence", result)
	}
	path := targetDriftArtifactPath(t, target, drift.ID)
	persisted, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	if persisted.ReviewState != "acknowledged" || persisted.ReviewedAt != "2026-05-19T10:11:12Z" || persisted.ReviewedBy != "ops" || persisted.ReviewReason != "accepted operator target edit" || persisted.ReviewAction != "acknowledge" {
		t.Fatalf("persisted drift = %+v, want acknowledgement metadata", persisted)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift acknowledge stderr = %q, want empty", stderr.String())
	}
}

func TestDriftAcknowledgeAcceptsPushRecordedTargetDrift(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "old\n")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("first push exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	mustWrite(t, filepath.Join(source, "file.txt"), "new\n")
	mustWrite(t, filepath.Join(target, "file.txt"), "operator target edit\n")
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-two"}, &stdout, &stderr); got != 1 {
		t.Fatalf("second push exit = %d, stderr = %q, stdout = %q, want target drift refusal", got, stderr.String(), stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"verify", "--profile", profilePath, "--session", "session-two", "--format", "json"}, &stdout, &stderr); got != 1 {
		t.Fatalf("verify drift exit = %d, stderr = %q, stdout = %q, want target drift evidence", got, stderr.String(), stdout.String())
	}
	var verifyReport verify.Report
	if err := json.Unmarshal(stdout.Bytes(), &verifyReport); err != nil {
		t.Fatalf("json.Unmarshal(verify drift stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if len(verifyReport.TargetDrifts) != 1 {
		t.Fatalf("verify target_drifts = %#v, want one persisted drift", verifyReport.TargetDrifts)
	}
	drift := verifyReport.TargetDrifts[0]
	if drift.SessionID != "session-two" || drift.Expected.SessionID != "session-one" || drift.Expected.ManifestID != "manifest-session-one" {
		t.Fatalf("push-recorded drift = %#v, want failed attempt with published expected baseline", drift)
	}

	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"drift", "acknowledge", "--profile", profilePath, "--id", drift.ID, "--reason", "reviewed push-recorded drift", "--reviewer", "ops", "--format", "json"}, &stdout, &stderr); got != 0 {
		t.Fatalf("drift acknowledge exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var result struct {
		ID          string `json:"id"`
		SessionID   string `json:"session_id"`
		ReviewState string `json:"review_state"`
		Reason      string `json:"reason"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(drift acknowledge stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if result.ID != drift.ID || result.SessionID != "session-two" || result.ReviewState != "acknowledged" || result.Reason != "reviewed push-recorded drift" {
		t.Fatalf("drift acknowledge result = %+v, want acknowledged push-recorded drift", result)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift acknowledge stderr = %q, want empty", stderr.String())
	}
}

func TestDriftResolveJSONUpdatesPersistedRecordAfterTargetRestored(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)
	drift := cliTargetDrift("drift-resolve")
	writeEmptyPublishedSessionForCLI(t, target, drift.SessionID)
	writeSessionRecord(t, transaction.NewLayout(control.ControlDir(target)), drift.SessionID, transaction.StatePublished)
	writeSessionProfileSnapshotForCLI(t, profilePath, target, drift.SessionID)
	writeTargetFileForCLIDrift(t, target, "file.txt", []byte("aaaaaaa"), 0o644)
	writeTargetDriftArtifact(t, target, drift)

	runner := Runner{Now: time.Date(2026, 5, 20, 10, 11, 12, 0, time.UTC)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := runner.Run([]string{"drift", "resolve", "--profile", profilePath, "--id", drift.ID, "--reason", "target restored", "--reviewer", "ops", "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("drift resolve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var result struct {
		ID              string `json:"id"`
		Path            string `json:"path"`
		PreviousState   string `json:"previous_state"`
		ReviewState     string `json:"review_state"`
		ReviewedAt      string `json:"reviewed_at"`
		Reviewer        string `json:"reviewer"`
		Reason          string `json:"reason"`
		ProfileID       string `json:"profile_id"`
		TargetID        string `json:"target_id"`
		SessionID       string `json:"session_id"`
		Repair          string `json:"repair"`
		ManifestRewrite string `json:"manifest_rewrite"`
		Prune           string `json:"prune"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(drift resolve stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if result.ID != drift.ID || result.Path != drift.Path || result.PreviousState != "needs_review" || result.ReviewState != "resolved" || result.ReviewedAt != "2026-05-20T10:11:12Z" || result.Reviewer != "ops" || result.Reason != "target restored" || result.ProfileID != "profile-local" || result.TargetID != "local:profile-local" || result.SessionID != drift.SessionID || result.Repair != "not_applied" || result.ManifestRewrite != "not_applied" || result.Prune != "not_authorized" {
		t.Fatalf("drift resolve JSON = %+v, want durable resolved review evidence", result)
	}
	persisted, err := control.ReadFile[control.TargetDrift](targetDriftArtifactPath(t, target, drift.ID))
	if err != nil {
		t.Fatalf("control.ReadFile(target drift) error = %v, want nil", err)
	}
	if persisted.ReviewState != "resolved" || persisted.ReviewAction != "resolve" || persisted.ReviewedBy != "ops" || persisted.ReviewReason != "target restored" {
		t.Fatalf("persisted drift = %+v, want resolved metadata", persisted)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift resolve stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = runner.Run([]string{"status", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("status after drift resolve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var statusReport status.Report
	if err := json.Unmarshal(stdout.Bytes(), &statusReport); err != nil {
		t.Fatalf("json.Unmarshal(status after resolve) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if statusReport.Counts.TargetDrifts != 0 || statusReport.Counts.LiveTargetDrifts != 0 || statusReport.ReviewRequired {
		t.Fatalf("status after resolve = %+v, want clean drift counts", statusReport)
	}
	if stderr.Len() != 0 {
		t.Fatalf("status after resolve stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = runner.Run([]string{"verify", "--profile", profilePath, "--session", drift.SessionID, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("verify after drift resolve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var verifyReport verify.Report
	if err := json.Unmarshal(stdout.Bytes(), &verifyReport); err != nil {
		t.Fatalf("json.Unmarshal(verify after resolve) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if verifyReport.Summary.TargetDrifts != 0 || len(verifyReport.TargetDrifts) != 0 {
		t.Fatalf("verify after resolve target drifts=%+v/%#v, want clean persisted drift counts", verifyReport.Summary, verifyReport.TargetDrifts)
	}
	if stderr.Len() != 0 {
		t.Fatalf("verify after resolve stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = runner.Run([]string{"health", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("health after drift resolve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var healthReport health.Report
	if err := json.Unmarshal(stdout.Bytes(), &healthReport); err != nil {
		t.Fatalf("json.Unmarshal(health after resolve) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if !healthReport.Healthy || healthReport.Summary.TargetDrifts != 0 || len(healthReport.TargetDrifts) != 0 {
		t.Fatalf("health after resolve = %+v, want clean persisted drift counts", healthReport)
	}
	if stderr.Len() != 0 {
		t.Fatalf("health after resolve stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = runner.Run([]string{"report", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("report after drift resolve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var reportJSON report.Report
	if err := json.Unmarshal(stdout.Bytes(), &reportJSON); err != nil {
		t.Fatalf("json.Unmarshal(report after resolve) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if reportJSON.Summary.TargetDrifts != 0 || len(reportJSON.TargetDrifts) != 0 || reportJSON.Summary.LiveTargetDrifts != 0 || reportJSON.Overall.Status != report.StatusVerified {
		t.Fatalf("report after resolve = %+v target_drifts=%#v live=%+v, want clean drift counts", reportJSON.Summary, reportJSON.TargetDrifts, reportJSON.LiveTargetDrift)
	}
	if stderr.Len() != 0 {
		t.Fatalf("report after resolve stderr = %q, want empty", stderr.String())
	}
}

func TestDriftResolveRefusesLiveOnlyIDAndStillDetectedDrift(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)
	drift := cliTargetDrift("drift-resolve-refusal")
	writeEmptyPublishedSessionForCLI(t, target, drift.SessionID)
	writeTargetFileForCLIDrift(t, target, "file.txt", []byte("bbbbbbbbb"), 0o644)
	writeTargetDriftArtifact(t, target, drift)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"drift", "list", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("drift list live exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var live verify.DriftReport
	if err := json.Unmarshal(stdout.Bytes(), &live); err != nil {
		t.Fatalf("json.Unmarshal(drift list stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if len(live.Drifts) != 1 || !strings.HasPrefix(live.Drifts[0].ID, "detected_") {
		t.Fatalf("drift list drifts=%#v, want one live detector id", live.Drifts)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list live stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"drift", "resolve", "--profile", profilePath, "--id", live.Drifts[0].ID, "--reason", "reviewed"}, &stdout, &stderr)
	if got != 1 || !strings.Contains(stderr.String(), "not found") || stdout.Len() != 0 {
		t.Fatalf("drift resolve live id exit=%d stdout=%q stderr=%q, want persisted-not-found refusal", got, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"drift", "resolve", "--profile", profilePath, "--id", drift.ID, "--reason", "reviewed"}, &stdout, &stderr)
	if got != 1 || !strings.Contains(stderr.String(), "still reports drift") || stdout.Len() != 0 {
		t.Fatalf("drift resolve still-detected exit=%d stdout=%q stderr=%q, want still-detected refusal", got, stdout.String(), stderr.String())
	}
	persisted, err := control.ReadFile[control.TargetDrift](targetDriftArtifactPath(t, target, drift.ID))
	if err != nil {
		t.Fatalf("control.ReadFile(target drift) error = %v, want nil", err)
	}
	if persisted.ReviewState != "needs_review" || persisted.ReviewAction != "" {
		t.Fatalf("persisted drift after resolve refusal = %+v, want unchanged", persisted)
	}
}

func TestBareResolvedTargetDriftArtifactRequiresReviewAcrossSurfaces(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)
	writeEmptyPublishedSessionForCLI(t, target, "session-one")
	writeSessionRecord(t, transaction.NewLayout(control.ControlDir(target)), "session-one", transaction.StatePublished)
	writeTargetFileForCLIDrift(t, target, "file.txt", []byte("aaaaaaa"), 0o644)
	writeSessionProfileSnapshotForCLI(t, profilePath, target, "session-one")
	bareResolved := `{"version":1,"id":"bare-resolved","session_id":"session-one","profile_id":"profile-local","target_id":"local:profile-local","root_id":"root","path":"file.txt","detected_at":"2026-05-16T00:01:00Z","change":"content_mismatch","review_state":"resolved"}`
	mustWrite(t, targetDriftArtifactPath(t, target, "bare-resolved"), bareResolved)

	tests := []struct {
		name     string
		args     []string
		wantText string
		wantErr  string
	}{
		{
			name:     "status",
			args:     []string{"status", "--profile", profilePath},
			wantText: "artifact_problems=1",
			wantErr:  "",
		},
		{
			name:     "verify",
			args:     []string{"verify", "--profile", profilePath, "--session", "session-one"},
			wantText: "artifact_problems=1",
			wantErr:  "review_action",
		},
		{
			name:     "health",
			args:     []string{"health", "--profile", profilePath},
			wantText: "artifact_problems=1",
			wantErr:  "review_action",
		},
		{
			name:     "report",
			args:     []string{"report", "--profile", profilePath},
			wantText: "artifact_problems=1",
			wantErr:  "review_action",
		},
		{
			name:     "status json",
			args:     []string{"status", "--profile", profilePath, "--format", "json"},
			wantText: `"artifact_problems":1`,
			wantErr:  "",
		},
		{
			name:     "verify json",
			args:     []string{"verify", "--profile", profilePath, "--session", "session-one", "--format", "json"},
			wantText: `"artifact_problems":1`,
			wantErr:  `review_action \"resolve\" is required`,
		},
		{
			name:     "health json",
			args:     []string{"health", "--profile", profilePath, "--format", "json"},
			wantText: `"artifact_problems":1`,
			wantErr:  `review_action \"resolve\" is required`,
		},
		{
			name:     "report json",
			args:     []string{"report", "--profile", profilePath, "--format", "json"},
			wantText: `"artifact_problems":1`,
			wantErr:  `review_action \"resolve\" is required`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			got := Run(tt.args, &stdout, &stderr)
			if got != 1 || stderr.Len() != 0 {
				t.Fatalf("%s exit=%d stdout=%q stderr=%q, want review exit with stdout-only artifact problem", tt.name, got, stdout.String(), stderr.String())
			}
			out := stdout.String()
			if !strings.Contains(out, tt.wantText) || (tt.wantErr != "" && !strings.Contains(out, tt.wantErr)) {
				t.Fatalf("%s stdout=%q, want artifact problem count and bare-resolved validation error", tt.name, out)
			}
			if strings.Contains(out, "target_drifts=1") || strings.Contains(out, `"target_drifts":1`) {
				t.Fatalf("%s stdout=%q, want bare resolved artifact problem instead of active target drift", tt.name, out)
			}
		})
	}
}

func TestDriftAcknowledgeRejectsLiveDetectorID(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "old")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(target, "file.txt"), "manual target edit")
	stdout.Reset()
	stderr.Reset()
	got := Run([]string{"drift", "list", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("drift list live exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var driftReport verify.DriftReport
	if err := json.Unmarshal(stdout.Bytes(), &driftReport); err != nil {
		t.Fatalf("json.Unmarshal(drift list stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if len(driftReport.Drifts) != 1 || !strings.HasPrefix(driftReport.Drifts[0].ID, "detected_") {
		t.Fatalf("drift list drifts=%#v, want one live detector id", driftReport.Drifts)
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"drift", "acknowledge", "--profile", profilePath, "--id", driftReport.Drifts[0].ID, "--reason", "reviewed live output"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("drift acknowledge live id exit = %d, stdout = %q, stderr = %q, want 1", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Fatalf("drift acknowledge live id stderr = %q, want persisted-not-found refusal", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("drift acknowledge live id stdout = %q, want empty", stdout.String())
	}
}

func TestDriftAcknowledgeRefusals(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)
	drift := cliTargetDrift("drift-refusal")
	writeEmptyPublishedSessionForCLI(t, target, drift.SessionID)
	writeTargetDriftArtifact(t, target, drift)
	foreign := cliTargetDrift("foreign-drift")
	foreign.ProfileID = "other-profile"
	writeTargetDriftArtifact(t, target, foreign)
	foreignTarget := cliTargetDrift("foreign-target-drift")
	foreignTarget.TargetID = "other-target"
	writeTargetDriftArtifact(t, target, foreignTarget)

	tests := []struct {
		name       string
		args       []string
		wantExit   int
		wantStderr string
	}{
		{
			name:       "missing reason",
			args:       []string{"drift", "acknowledge", "--profile", profilePath, "--id", drift.ID},
			wantExit:   2,
			wantStderr: "--reason is required",
		},
		{
			name:       "unsafe id",
			args:       []string{"drift", "acknowledge", "--profile", profilePath, "--id", "../drift", "--reason", "reviewed"},
			wantExit:   2,
			wantStderr: "--id is invalid",
		},
		{
			name:       "scope mismatch",
			args:       []string{"drift", "acknowledge", "--profile", profilePath, "--id", foreign.ID, "--reason", "reviewed"},
			wantExit:   1,
			wantStderr: "does not match profile scope",
		},
		{
			name:       "target scope mismatch",
			args:       []string{"drift", "acknowledge", "--profile", profilePath, "--id", foreignTarget.ID, "--reason", "reviewed"},
			wantExit:   1,
			wantStderr: "does not match profile scope",
		},
		{
			name:       "missing persisted record",
			args:       []string{"drift", "acknowledge", "--profile", profilePath, "--id", "live-only", "--reason", "reviewed"},
			wantExit:   1,
			wantStderr: "not found",
		},
		{
			name:       "unsupported format",
			args:       []string{"drift", "acknowledge", "--profile", profilePath, "--id", drift.ID, "--reason", "reviewed", "--format", "yaml"},
			wantExit:   2,
			wantStderr: "unsupported format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			got := Run(tt.args, &stdout, &stderr)
			if got != tt.wantExit {
				t.Fatalf("Run(%v) exit = %d, stdout = %q, stderr = %q, want %d", tt.args, got, stdout.String(), stderr.String(), tt.wantExit)
			}
			if stdout.Len() != 0 {
				t.Fatalf("Run(%v) stdout = %q, want empty", tt.args, stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("Run(%v) stderr = %q, want %q", tt.args, stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestDriftListRemainsReadOnlyWithPersistedDrift(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "old")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	drift := cliTargetDrift("drift-readonly")
	writeEmptyPublishedSessionForCLI(t, target, drift.SessionID)
	writeTargetDriftArtifact(t, target, drift)
	mustWrite(t, filepath.Join(target, "file.txt"), "manual target edit")
	controlDir := filepath.Join(target, control.DirName)
	before := snapshotTree(t, controlDir)

	stdout.Reset()
	stderr.Reset()
	got := Run([]string{"drift", "list", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list persisted exit = %d, stdout = %q, stderr = %q, want 1", got, stdout.String(), stderr.String())
	}
	var report verify.DriftReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal(drift list persisted stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if report.Summary.TargetDrifts != 1 || len(report.Drifts) != 1 {
		t.Fatalf("drift list persisted summary = %+v drifts=%#v, want one live detector drift", report.Summary, report.Drifts)
	}
	after := snapshotTree(t, controlDir)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("drift list changed persisted control plane\nbefore=%#v\nafter=%#v", before, after)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list persisted stderr = %q, want empty", stderr.String())
	}
}

func TestReportAndStatusRemainReadOnlyAfterAcknowledge(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)
	drift := cliTargetDrift("drift-report-status-readonly")
	writeEmptyPublishedSessionForCLI(t, target, drift.SessionID)
	writeTargetDriftArtifact(t, target, drift)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"drift", "acknowledge", "--profile", profilePath, "--id", drift.ID, "--reason", "reviewed"}, &stdout, &stderr); got != 0 {
		t.Fatalf("drift acknowledge exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	controlDir := filepath.Join(target, control.DirName)
	before := snapshotTree(t, controlDir)

	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "report", args: []string{"report", "--profile", profilePath, "--format", "json"}},
		{name: "status", args: []string{"status", "--profile", profilePath, "--format", "json"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			got := Run(tt.args, &stdout, &stderr)
			if got != 1 {
				t.Fatalf("Run(%v) exit = %d, stderr = %q, stdout = %q, want 1", tt.args, got, stderr.String(), stdout.String())
			}
			if stdout.Len() == 0 {
				t.Fatalf("Run(%v) stdout empty, want JSON review surface", tt.args)
			}
			if stderr.Len() != 0 {
				t.Fatalf("Run(%v) stderr = %q, want empty", tt.args, stderr.String())
			}
			after := snapshotTree(t, controlDir)
			if !reflect.DeepEqual(after, before) {
				t.Fatalf("Run(%v) changed control plane\nbefore=%#v\nafter=%#v", tt.args, before, after)
			}
		})
	}
}

func TestDriftListHistoricalSessionDoesNotIncludeLatestExtraPaths(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("first push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-two"}, &stdout, &stderr); got != 0 {
		t.Fatalf("second push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(target, "extra.txt"), "target-only")
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "list", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("historical drift list exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "extra.txt") || !strings.Contains(stdout.String(), "target_drifts=0") {
		t.Fatalf("historical drift list stdout = %q, want no latest extra-path drift", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("historical drift list stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"drift", "list", "--profile", profilePath}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("latest drift list exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "path=extra.txt") || !strings.Contains(stdout.String(), "change=extra") {
		t.Fatalf("latest drift list stdout = %q, want latest extra-path drift", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("latest drift list stderr = %q, want empty", stderr.String())
	}
}

func TestHealthAndReportTextShowTargetDriftArtifacts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "old")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("first push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(source, "file.txt"), "new")
	mustWrite(t, filepath.Join(target, "file.txt"), "manual target edit")
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-two"}, &stdout, &stderr); got != 1 {
		t.Fatalf("second push drift exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "refusing") {
		t.Fatalf("second push drift stderr = %q, want refusal", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "list", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("drift list scoped drift exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{"target_drifts=1", "target_drift id=detected_", "session=session-one", "path=file.txt", "change=content_mismatch", "review_state=needs_review"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("drift list scoped drift stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list scoped drift stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"health", "--profile", profilePath}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("health drift exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{"healthy=false", "target_drifts=1", "target_drift session=session-two", "path=file.txt", "change=content_mismatch"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("health drift stdout = %q, want %q", stdout.String(), want)
		}
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"report", "--profile", profilePath}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("report drift exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{"status=local_target_unhealthy", "target_drifts=1", "live_target_drifts=1", "issues=recovery_issues,verification_errors,target_drifts,live_target_drifts", "target_drift id=session-two-drift_", "live_target_drift source=live_detector durable=false", "live_target_drift_item id=detected_", "path=file.txt", "change=content_mismatch"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report drift stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("report drift stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"verify", "--profile", profilePath, "--session", "session-two", "--format", "json"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("verify scoped drift exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var verifyReport verify.Report
	if err := json.Unmarshal(stdout.Bytes(), &verifyReport); err != nil {
		t.Fatalf("json.Unmarshal(verify scoped drift stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if verifyReport.Summary.TargetDrifts != 1 || len(verifyReport.TargetDrifts) != 1 || verifyReport.TargetDrifts[0].SessionID != "session-two" {
		t.Fatalf("verify scoped drift report=%+v drifts=%#v, want session-two drift evidence", verifyReport.Summary, verifyReport.TargetDrifts)
	}
	verifyDrift := verifyReport.TargetDrifts[0]
	if verifyDrift.ReviewState != "needs_review" || verifyDrift.Expected.SessionID != "session-one" || verifyDrift.Expected.ManifestID != "manifest-session-one" || verifyDrift.Observed.Path != "file.txt" {
		t.Fatalf("verify scoped drift = %#v, want structured expected/observed needs-review evidence", verifyDrift)
	}
	if stderr.Len() != 0 {
		t.Fatalf("verify scoped drift stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"report", "--profile", profilePath, "--session", "session-two", "--format", "json"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("report scoped drift exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var reportJSON report.Report
	if err := json.Unmarshal(stdout.Bytes(), &reportJSON); err != nil {
		t.Fatalf("json.Unmarshal(report scoped drift stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if reportJSON.Summary.TargetDrifts != 1 || len(reportJSON.TargetDrifts) != 1 || reportJSON.TargetDrifts[0].SessionID != "session-two" {
		t.Fatalf("report scoped persisted drift report=%+v drifts=%#v, want session-two persisted drift evidence", reportJSON.Summary, reportJSON.TargetDrifts)
	}
	reportDrift := reportJSON.TargetDrifts[0]
	if reportDrift.ReviewState != "needs_review" || reportDrift.Expected.SessionID != "session-one" || reportDrift.Expected.ManifestID != "manifest-session-one" || reportDrift.Observed.Path != "file.txt" {
		t.Fatalf("report scoped drift = %#v, want structured expected/observed needs-review evidence", reportDrift)
	}
	if reportJSON.Summary.LiveTargetDrifts != 0 || reportJSON.Summary.LiveTargetDriftProblems != 1 || reportJSON.LiveTargetDrift.Source != "live_detector" || reportJSON.LiveTargetDrift.Durable {
		t.Fatalf("report scoped live drift report=%+v live=%+v, want non-durable live detector artifact problem for missing selected manifest", reportJSON.Summary, reportJSON.LiveTargetDrift)
	}
	if len(reportJSON.LiveTargetDrift.ArtifactProblems) != 1 || !strings.Contains(reportJSON.LiveTargetDrift.ArtifactProblems[0].Error, "manifest for session") {
		t.Fatalf("report scoped live artifact problems=%#v, want selected-manifest detector problem", reportJSON.LiveTargetDrift.ArtifactProblems)
	}
	if stderr.Len() != 0 {
		t.Fatalf("report scoped drift stderr = %q, want empty", stderr.String())
	}
}

func TestDriftListRejectsSessionFromDifferentProfile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profileA := filepath.Join(dir, "a.json")
	profileB := filepath.Join(dir, "b.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	if err := profile.WriteFile(profileA, profile.NewDefault("profile-a", "Profile A", source, target)); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profileA, err)
	}
	if err := profile.WriteFile(profileB, profile.NewDefault("profile-b", "Profile B", source, target)); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profileB, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profileA, "--session", "session-test"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push profile-a exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "list", "--profile", profileB, "--session", "session-test"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list profile-b exit = %d, stdout = %q, stderr = %q, want 1", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "does not match requested profile/target") {
		t.Fatalf("drift list profile-b stderr = %q, want profile/target mismatch", stderr.String())
	}
}

func TestDriftListCorruptManifestReturnsArtifactProblemJSON(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	writeDriftListPublishedReceipt(t, target, "bad")
	manifestPath, err := control.Path(target, control.ArtifactManifest, "bad")
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v, want nil", err)
	}
	mustWrite(t, manifestPath, "{")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"drift", "list", "--profile", profilePath, "--session", "bad", "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list corrupt manifest exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var gotReport verify.DriftReport
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(drift corrupt manifest stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Summary.ManifestCount != 0 || gotReport.Summary.ArtifactProblems != 1 || len(gotReport.ArtifactProblems) != 1 || gotReport.ArtifactProblems[0].SessionID != "bad" {
		t.Fatalf("drift list corrupt manifest report=%+v problems=%#v, want structured artifact problem", gotReport.Summary, gotReport.ArtifactProblems)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list corrupt manifest stderr = %q, want empty", stderr.String())
	}
}

func TestDriftListCorruptReceiptReturnsArtifactProblemJSON(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, "bad-receipt")
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	mustWrite(t, receiptPath, "{")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"drift", "list", "--profile", profilePath, "--session", "bad-receipt", "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list corrupt receipt exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var gotReport verify.DriftReport
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(drift corrupt receipt stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Summary.ManifestCount != 0 || gotReport.Summary.ArtifactProblems != 1 || len(gotReport.ArtifactProblems) != 1 || gotReport.ArtifactProblems[0].SessionID != "bad-receipt" {
		t.Fatalf("drift list corrupt receipt report=%+v problems=%#v, want structured receipt artifact problem", gotReport.Summary, gotReport.ArtifactProblems)
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list corrupt receipt stderr = %q, want empty", stderr.String())
	}
}

func TestDriftListMissingSessionReturnsOperatorErrorWithoutJSON(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-existing"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "list", "--profile", profilePath, "--session", "session-missing", "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list missing session exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), `manifest for session "session-missing" not found`) {
		t.Fatalf("drift list missing session stderr = %q, want missing manifest error", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("drift list missing session stdout = %q, want empty because runtime selection errors are not JSON reports", stdout.String())
	}
}

func TestReportMissingSessionReturnsSelectionErrorWithoutJSON(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-existing"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"report", "--profile", profilePath, "--session", "session-missing", "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("report missing session exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), `manifest for session "session-missing" not found`) {
		t.Fatalf("report missing session stderr = %q, want missing manifest error", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("report missing session stdout = %q, want empty because runtime selection errors are not JSON reports", stdout.String())
	}
}

func TestDriftListRejectsInvalidUsage(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "missing-target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing subcommand",
			args: []string{"drift"},
			want: "drift: missing subcommand",
		},
		{
			name: "unknown subcommand",
			args: []string{"drift", "show"},
			want: `drift: unknown subcommand "show"`,
		},
		{
			name: "missing profile",
			args: []string{"drift", "list"},
			want: "drift list: --profile is required",
		},
		{
			name: "unexpected argument",
			args: []string{"drift", "list", "--profile", profilePath, "extra"},
			want: "drift list: unexpected arguments: extra",
		},
		{
			name: "unexpected argument with newline",
			args: []string{"drift", "list", "--profile", profilePath, "extra\ntransfer=ready"},
			want: `drift list: unexpected arguments: "extra\ntransfer=ready"`,
		},
		{
			name: "unsupported format before target read",
			args: []string{"drift", "list", "--profile", profilePath, "--format", "yaml"},
			want: `drift list: unsupported format "yaml"`,
		},
		{
			name: "record missing profile",
			args: []string{"drift", "record"},
			want: "drift record: --profile is required",
		},
		{
			name: "record unexpected argument",
			args: []string{"drift", "record", "--profile", profilePath, "extra"},
			want: "drift record: unexpected arguments: extra",
		},
		{
			name: "record unexpected argument with newline",
			args: []string{"drift", "record", "--profile", profilePath, "extra\nrecorded=true"},
			want: `drift record: unexpected arguments: "extra\nrecorded=true"`,
		},
		{
			name: "record unsupported format",
			args: []string{"drift", "record", "--profile", profilePath, "--format", "yaml"},
			want: `drift record: unsupported format "yaml"`,
		},
		{
			name: "record target override unavailable",
			args: []string{"drift", "record", "--profile", profilePath, "--target", target},
			want: "drift record: flag provided but not defined: -target",
		},
		{
			name: "record policy override unavailable",
			args: []string{"drift", "record", "--profile", profilePath, "--policy", "fast"},
			want: "drift record: flag provided but not defined: -policy",
		},
		{
			name: "record network override unavailable",
			args: []string{"drift", "record", "--profile", profilePath, "--network"},
			want: "drift record: flag provided but not defined: -network",
		},
		{
			name: "record unsafe session id",
			args: []string{"drift", "record", "--profile", profilePath, "--session", "../bad"},
			want: `unsafe session id "../bad"`,
		},
		{
			name: "record whitespace session id",
			args: []string{"drift", "record", "--profile", profilePath, "--session", "   "},
			want: "session id is required when --session is provided",
		},
		{
			name: "target override unavailable",
			args: []string{"drift", "list", "--profile", profilePath, "--target", target},
			want: "drift list: flag provided but not defined: -target",
		},
		{
			name: "unsafe session id",
			args: []string{"drift", "list", "--profile", profilePath, "--session", "../bad"},
			want: `unsafe session id "../bad"`,
		},
		{
			name: "whitespace session id",
			args: []string{"drift", "list", "--profile", profilePath, "--session", "   "},
			want: "session id is required when --session is provided",
		},
		{
			name: "acknowledge missing profile",
			args: []string{"drift", "acknowledge", "--id", "drift-one", "--reason", "reviewed"},
			want: "drift acknowledge: --profile is required",
		},
		{
			name: "acknowledge missing id",
			args: []string{"drift", "acknowledge", "--profile", profilePath, "--reason", "reviewed"},
			want: "drift acknowledge: --id is required",
		},
		{
			name: "acknowledge missing reason",
			args: []string{"drift", "acknowledge", "--profile", profilePath, "--id", "drift-one"},
			want: "drift acknowledge: --reason is required",
		},
		{
			name: "acknowledge unsafe id",
			args: []string{"drift", "acknowledge", "--profile", profilePath, "--id", "../drift", "--reason", "reviewed"},
			want: "drift acknowledge: --id is invalid",
		},
		{
			name: "acknowledge unexpected argument",
			args: []string{"drift", "acknowledge", "--profile", profilePath, "--id", "drift-one", "--reason", "reviewed", "extra"},
			want: "drift acknowledge: unexpected arguments: extra",
		},
		{
			name: "acknowledge unsupported format",
			args: []string{"drift", "acknowledge", "--profile", profilePath, "--id", "drift-one", "--reason", "reviewed", "--format", "yaml"},
			want: `drift acknowledge: unsupported format "yaml"`,
		},
		{
			name: "acknowledge target override unavailable",
			args: []string{"drift", "acknowledge", "--profile", profilePath, "--id", "drift-one", "--reason", "reviewed", "--target", target},
			want: "drift acknowledge: flag provided but not defined: -target",
		},
		{
			name: "resolve missing profile",
			args: []string{"drift", "resolve", "--id", "drift-one", "--reason", "reviewed"},
			want: "drift resolve: --profile is required",
		},
		{
			name: "resolve missing id",
			args: []string{"drift", "resolve", "--profile", profilePath, "--reason", "reviewed"},
			want: "drift resolve: --id is required",
		},
		{
			name: "resolve missing reason",
			args: []string{"drift", "resolve", "--profile", profilePath, "--id", "drift-one"},
			want: "drift resolve: --reason is required",
		},
		{
			name: "resolve unsafe id",
			args: []string{"drift", "resolve", "--profile", profilePath, "--id", "../drift", "--reason", "reviewed"},
			want: "drift resolve: --id is invalid",
		},
		{
			name: "resolve unexpected argument",
			args: []string{"drift", "resolve", "--profile", profilePath, "--id", "drift-one", "--reason", "reviewed", "extra"},
			want: "drift resolve: unexpected arguments: extra",
		},
		{
			name: "resolve unsupported format",
			args: []string{"drift", "resolve", "--profile", profilePath, "--id", "drift-one", "--reason", "reviewed", "--format", "yaml"},
			want: `drift resolve: unsupported format "yaml"`,
		},
		{
			name: "resolve target override unavailable",
			args: []string{"drift", "resolve", "--profile", profilePath, "--id", "drift-one", "--reason", "reviewed", "--target", target},
			want: "drift resolve: flag provided but not defined: -target",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d, stdout = %q, stderr = %q, want 2", tt.args, got, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("Run(%v) stderr = %q, want %q", tt.args, stderr.String(), tt.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("Run(%v) stdout = %q, want empty", tt.args, stdout.String())
			}
		})
	}
}

func TestDriftCommandsEscapeUnknownFlagErrors(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.json")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "record",
			args: []string{"drift", "record", "--profile", profilePath, "-bad\nflag"},
			want: `drift record: flag provided but not defined: -bad\nflag`,
		},
		{
			name: "list",
			args: []string{"drift", "list", "--profile", profilePath, "-bad\nflag"},
			want: `drift list: flag provided but not defined: -bad\nflag`,
		},
		{
			name: "acknowledge",
			args: []string{"drift", "acknowledge", "--profile", profilePath, "--id", "drift-one", "--reason", "reviewed", "-bad\nflag"},
			want: `drift acknowledge: flag provided but not defined: -bad\nflag`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d, stderr = %q, stdout = %q, want 2", tt.args, got, stderr.String(), stdout.String())
			}
			if strings.Contains(stderr.String(), "bad\nflag") {
				t.Fatalf("Run(%v) stderr = %q, must not contain raw newline flag", tt.args, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("Run(%v) stderr = %q, want %q", tt.args, stderr.String(), tt.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("Run(%v) stdout = %q, want empty", tt.args, stdout.String())
			}
		})
	}
}

func TestDriftListTextEscapesTargetControlledValues(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-safe-text"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(target, "line\nbreak.txt"), "target-only")
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "list", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list text escape exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "path=line\nbreak.txt") {
		t.Fatalf("drift list text escape stdout = %q, must not contain raw newline path value", stdout.String())
	}
	if !strings.Contains(stdout.String(), "path=line%0Abreak.txt") {
		t.Fatalf("drift list text escape stdout = %q, want percent-encoded path value", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list text escape stderr = %q, want empty", stderr.String())
	}
}

func TestDriftAcknowledgeTextEscapesOperatorControlledValues(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)
	drift := cliTargetDrift("drift-text-escape")
	writeEmptyPublishedSessionForCLI(t, target, drift.SessionID)
	writeTargetDriftArtifact(t, target, drift)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"drift", "acknowledge", "--profile", profilePath, "--id", drift.ID, "--reason", "line\nbreak", "--reviewer", "ops\nteam"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("drift acknowledge text escape exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "line\nbreak") || strings.Contains(stdout.String(), "ops\nteam") {
		t.Fatalf("drift acknowledge text stdout = %q, must not contain raw newline values", stdout.String())
	}
	for _, want := range []string{"reason=line%0Abreak", "reviewer=ops%0Ateam"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("drift acknowledge text stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift acknowledge text stderr = %q, want empty", stderr.String())
	}
}

func TestReportTextEscapesLiveDriftTargetControlledValues(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-live-text"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	mustWrite(t, filepath.Join(target, "line\nbreak.txt"), "target-only")
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"report", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("report text escape exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "path=line\nbreak.txt") {
		t.Fatalf("report text escape stdout = %q, must not contain raw newline path value", stdout.String())
	}
	for _, want := range []string{"live_target_drifts=1", "live_target_drift source=live_detector durable=false", "path=line%0Abreak.txt"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report text escape stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("report text escape stderr = %q, want empty", stderr.String())
	}
}

func TestReportTextEscapesPersistedReviewValues(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "keep.txt"), "keep")
	mustWrite(t, filepath.Join(source, "gone.txt"), "gone")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("first push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if err := os.Remove(filepath.Join(source, "gone.txt")); err != nil {
		t.Fatalf("os.Remove(source gone) error = %v, want nil", err)
	}
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-two"}, &stdout, &stderr); got != 0 {
		t.Fatalf("second push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	writeReviewWarning(t, target, "session-two")
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"report", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("report persisted text escape exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	for _, forbidden := range []string{"path needs additional", "present in previous manifest"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("report persisted text stdout = %q, must not contain raw value %q", stdout.String(), forbidden)
		}
	}
	for _, want := range []string{"message=path%20needs%20additional%20migration%20config", "patch=include.needs_extra=true", "target=gone.txt", "reason=present%20in%20previous%20manifest"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report persisted text stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("report persisted text escape stderr = %q, want empty", stderr.String())
	}
}

func TestDriftListTextEscapesEvidenceValues(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	if err := os.Symlink("expected-target", filepath.Join(source, "link")); err != nil {
		t.Skipf("Symlink(source link) unavailable: %v", err)
	}
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-evidence-text"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if err := os.Remove(filepath.Join(target, "link")); err != nil {
		t.Fatalf("os.Remove(target link) error = %v, want nil", err)
	}
	if err := os.Symlink("actual\nfield=value", filepath.Join(target, "link")); err != nil {
		t.Skipf("Symlink(target link) unavailable: %v", err)
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"drift", "list", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list evidence escape exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "field=value") || strings.Contains(stdout.String(), "\nfield=value") {
		t.Fatalf("drift list evidence escape stdout = %q, must not contain raw evidence field injection", stdout.String())
	}
	if !strings.Contains(stdout.String(), "field%3Dvalue") {
		t.Fatalf("drift list evidence escape stdout = %q, want encoded evidence value", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list evidence escape stderr = %q, want empty", stderr.String())
	}
}

func TestDriftListRejectsSymlinkTargetRoot(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	realTarget := filepath.Join(dir, "real-target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, realTarget)
	if err := os.Symlink(realTarget, target); err != nil {
		t.Skipf("Symlink(target root) unavailable: %v", err)
	}
	writeDefaultProfile(t, profilePath, source, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"drift", "list", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list symlink target root exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "target root") || !strings.Contains(stderr.String(), "symlink") {
		t.Fatalf("drift list symlink target root stderr = %q, want target root symlink error", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("drift list symlink target root stdout = %q, want empty", stdout.String())
	}
}

func TestDriftListRejectsSymlinkControlPlane(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	outside := filepath.Join(dir, "outside-control")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustMkdir(t, outside)
	if err := os.Symlink(outside, filepath.Join(target, control.DirName)); err != nil {
		t.Skipf("Symlink(control plane) unavailable: %v", err)
	}
	writeDefaultProfile(t, profilePath, source, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"drift", "list", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list symlink control-plane exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "control directory") || !strings.Contains(stderr.String(), "symlink") {
		t.Fatalf("drift list symlink control-plane stderr = %q, want control directory symlink error", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("drift list symlink control-plane stdout = %q, want empty", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(outside, "sessions")); !os.IsNotExist(err) {
		t.Fatalf("Stat(outside sessions) error = %v, want os.ErrNotExist", err)
	}
}

func TestDriftListRejectsNestedControlPlaneSymlink(t *testing.T) {
	tests := []struct {
		name    string
		linkRel string
	}{
		{name: "sessions directory", linkRel: filepath.Join(control.DirName, "sessions")},
		{name: "session directory", linkRel: filepath.Join(control.DirName, "sessions", "session-link")},
		{name: "receipt file", linkRel: filepath.Join(control.DirName, "sessions", "session-link", "receipt.json")},
		{name: "session record file", linkRel: filepath.Join(control.DirName, "sessions", "session-link", "session.json")},
		{name: "network transfer file", linkRel: filepath.Join(control.DirName, "sessions", "session-link", "network-transfer.json")},
		{name: "profile snapshot file", linkRel: filepath.Join(control.DirName, "profiles", "profile-session-link.json")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			target := filepath.Join(dir, "target")
			outside := filepath.Join(dir, "outside-control")
			profilePath := filepath.Join(dir, "profile.json")
			mustMkdir(t, source)
			mustMkdir(t, target)
			mustMkdir(t, outside)
			linkPath := filepath.Join(target, tt.linkRel)
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatalf("os.MkdirAll(link parent) error = %v, want nil", err)
			}
			if err := os.Symlink(outside, linkPath); err != nil {
				t.Skipf("Symlink(%s) unavailable: %v", tt.linkRel, err)
			}
			writeDefaultProfile(t, profilePath, source, target)
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run([]string{"drift", "list", "--profile", profilePath}, &stdout, &stderr)

			if got != 1 {
				t.Fatalf("drift list nested control symlink exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
			}
			if !strings.Contains(stderr.String(), "control artifact path") || !strings.Contains(stderr.String(), "symlink") {
				t.Fatalf("drift list nested control symlink stderr = %q, want control artifact symlink error", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("drift list nested control symlink stdout = %q, want empty", stdout.String())
			}
			if _, err := os.Stat(filepath.Join(outside, "sessions")); !os.IsNotExist(err) {
				t.Fatalf("Stat(outside sessions) error = %v, want os.ErrNotExist", err)
			}
		})
	}
}

func TestAuditCommandsEscapeUnsafeControlBoundaryErrors(t *testing.T) {
	tests := []struct {
		name string
		args func(profilePath string) []string
	}{
		{
			name: "verify",
			args: func(profilePath string) []string {
				return []string{"verify", "--profile", profilePath}
			},
		},
		{
			name: "health",
			args: func(profilePath string) []string {
				return []string{"health", "--profile", profilePath}
			},
		},
		{
			name: "report",
			args: func(profilePath string) []string {
				return []string{"report", "--profile", profilePath}
			},
		},
		{
			name: "drift list",
			args: func(profilePath string) []string {
				return []string{"drift", "list", "--profile", profilePath}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			target := filepath.Join(dir, "target")
			outside := filepath.Join(dir, "outside-control")
			profilePath := filepath.Join(dir, "profile.json")
			mustMkdir(t, source)
			mustMkdir(t, target)
			mustMkdir(t, outside)
			linkPath := filepath.Join(target, control.DirName, "sessions", "line\nbreak")
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatalf("os.MkdirAll(link parent) error = %v, want nil", err)
			}
			if err := os.Symlink(outside, linkPath); err != nil {
				t.Skipf("Symlink(newline control artifact path) unavailable: %v", err)
			}
			writeDefaultProfile(t, profilePath, source, target)
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args(profilePath), &stdout, &stderr)

			if got != 1 {
				t.Fatalf("%s unsafe control boundary exit = %d, stderr = %q, stdout = %q, want 1", tt.name, got, stderr.String(), stdout.String())
			}
			if strings.Contains(stderr.String(), "line\nbreak") {
				t.Fatalf("%s unsafe control boundary stderr = %q, must not contain raw newline path", tt.name, stderr.String())
			}
			if !strings.Contains(stderr.String(), `line\nbreak`) {
				t.Fatalf("%s unsafe control boundary stderr = %q, want escaped newline path", tt.name, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("%s unsafe control boundary stdout = %q, want empty", tt.name, stdout.String())
			}
			if _, err := os.Stat(filepath.Join(outside, "sessions")); !os.IsNotExist(err) {
				t.Fatalf("Stat(outside sessions) error = %v, want os.ErrNotExist", err)
			}
		})
	}
}

func TestDriftListEscapesProfileReadErrors(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "bad\nprofile.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"drift", "list", "--profile", profilePath}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("drift list profile read error exit = %d, stderr = %q, stdout = %q, want 2", got, stderr.String(), stdout.String())
	}
	if strings.Contains(stderr.String(), "bad\nprofile.json") {
		t.Fatalf("drift list profile read stderr = %q, must not contain raw newline path", stderr.String())
	}
	if !strings.Contains(stderr.String(), `bad\nprofile.json`) {
		t.Fatalf("drift list profile read stderr = %q, want escaped newline path", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("drift list profile read stdout = %q, want empty", stdout.String())
	}
}

func TestDriftListEmptyTargetRequiresReview(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"drift", "list", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("drift list empty target exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "manifests=0") || !strings.Contains(stdout.String(), "target_drifts=0") {
		t.Fatalf("drift list empty target stdout = %q, want empty review summary", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list empty target stderr = %q, want empty", stderr.String())
	}
}

func TestHealthIgnoresForeignTargetDriftArtifacts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	writeTargetDriftArtifact(t, target, control.TargetDrift{
		Version:    control.CurrentVersion,
		ID:         "foreign-drift",
		SessionID:  "foreign-session",
		ProfileID:  "foreign-profile",
		TargetID:   "foreign-target",
		RootID:     "root",
		Path:       "file.txt",
		DetectedAt: "2026-05-16T00:01:00Z",
		Change:     "content_mismatch",
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"health", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("health foreign drift exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{"healthy=true", "target_drifts=0"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("health foreign drift stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("health foreign drift stderr = %q, want empty", stderr.String())
	}
}

func writeReviewWarning(t *testing.T, target string, sessionID string) {
	t.Helper()
	warning := control.Warning{
		Version:   control.CurrentVersion,
		ID:        sessionID + "-001-extra-config",
		SessionID: sessionID,
		Code:      "needs_profile_config",
		Message:   "path needs additional migration config",
		Severity:  "warning",
		Paths:     []string{"needs-extra"},
		SuggestedProfilePatch: map[string]string{
			"include.needs_extra": "true",
		},
		CreatedAt: "2026-05-16T00:00:00Z",
	}
	warningPath, err := control.Path(target, control.ArtifactWarning, warning.ID)
	if err != nil {
		t.Fatalf("control.Path(warning) error = %v, want nil", err)
	}
	if err := control.WriteFile(warningPath, warning); err != nil {
		t.Fatalf("control.WriteFile(warning) error = %v, want nil", err)
	}
}

func writeTargetDriftArtifact(t *testing.T, target string, drift control.TargetDrift) {
	t.Helper()
	path := targetDriftArtifactPath(t, target, drift.ID)
	if err := control.WriteFile(path, drift); err != nil {
		t.Fatalf("control.WriteFile(%q, target drift) error = %v, want nil", path, err)
	}
}

func writeTargetFileForCLIDrift(t *testing.T, target string, rel string, data []byte, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(target, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
	modTime, err := time.Parse(time.RFC3339Nano, "2026-05-18T00:00:00Z")
	if err != nil {
		t.Fatalf("time.Parse(modtime) error = %v, want nil", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", path, err)
	}
}

func writeSessionProfileSnapshotForCLI(t *testing.T, profilePath string, target string, sessionID string) {
	t.Helper()
	p, payload, err := readProfileFilePayload(profilePath)
	if err != nil {
		t.Fatalf("readProfileFilePayload(%q) error = %v, want nil", profilePath, err)
	}
	snapshotID := "profile-" + sessionID
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         snapshotID,
		ProfileID:  p.ProfileID,
		SessionID:  sessionID,
		CapturedAt: "2026-05-16T00:00:00Z",
		Profile:    payload,
	}
	path, err := control.Path(target, control.ArtifactProfileSnapshot, snapshotID)
	if err != nil {
		t.Fatalf("control.Path(profile snapshot %q) error = %v, want nil", snapshotID, err)
	}
	if err := control.WriteFile(path, snapshot); err != nil {
		t.Fatalf("control.WriteFile(%q, profile snapshot) error = %v, want nil", path, err)
	}
}

func targetDriftArtifactPath(t *testing.T, target string, id string) string {
	t.Helper()
	path, err := control.Path(target, control.ArtifactTargetDrift, id)
	if err != nil {
		t.Fatalf("control.Path(target drift) error = %v, want nil", err)
	}
	return path
}

func testDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cliTargetDrift(id string) control.TargetDrift {
	present := true
	expectedPayload := []byte("aaaaaaa")
	expected := control.TargetDriftExpectedState{
		SessionID:  "session-one",
		ManifestID: "manifest-session-one",
		Kind:       "file",
		Path:       "file.txt",
		Digest:     testDigest(expectedPayload),
		ModTime:    "2026-05-18T00:00:00Z",
	}
	expected.SetSizeEvidence(int64(len(expectedPayload)))
	expected.SetModeEvidence(0o644)
	observed := control.TargetDriftObservedState{
		Present: &present,
		Kind:    "file",
		Path:    "file.txt",
		Digest:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ModTime: "2026-05-19T00:00:00Z",
	}
	observed.SetSizeEvidence(9)
	observed.SetModeEvidence(0o644)
	return control.TargetDrift{
		Version:     control.CurrentVersion,
		ID:          id,
		SessionID:   "session-one",
		ProfileID:   "profile-local",
		TargetID:    "local:profile-local",
		RootID:      "root",
		Path:        "file.txt",
		DetectedAt:  "2026-05-19T00:00:00Z",
		Change:      "content_mismatch",
		Expected:    expected,
		Observed:    observed,
		ReviewState: "needs_review",
		Evidence:    []string{"target content differs from manifest evidence"},
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if _, err := os.Lstat(root); err != nil {
		if os.IsNotExist(err) {
			return out
		}
		t.Fatalf("os.Lstat(%q) error = %v, want nil or not-exist", root, err)
	}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			out[rel] = "symlink:" + target
		case info.IsDir():
			out[rel] = "dir"
		default:
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[rel] = "file:" + string(content)
		}
		return nil
	}); err != nil {
		t.Fatalf("filepath.WalkDir(%q) error = %v, want nil", root, err)
	}
	return out
}

func writeDriftListPublishedReceipt(t *testing.T, target string, sessionID string) {
	t.Helper()
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	if err := control.WriteFile(receiptPath, control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: "profile-local",
		TargetID:  "local:profile-local",
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    "published",
	}); err != nil {
		t.Fatalf("control.WriteFile(%q, receipt) error = %v, want nil", receiptPath, err)
	}
}
