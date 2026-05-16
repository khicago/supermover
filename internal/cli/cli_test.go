package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/transaction"
)

func TestRunHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"help"}, &stdout, &stderr)

	if got != 0 {
		t.Errorf("Run(%v) exit = %d, want %d", []string{"help"}, got, 0)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("Run(%v) stdout = %q, want usage text", []string{"help"}, stdout.String())
	}
	if !strings.Contains(stdout.String(), "Available commands:") {
		t.Errorf("Run(%v) stdout = %q, want available command section", []string{"help"}, stdout.String())
	}
	if !strings.Contains(stdout.String(), "Planned commands:") {
		t.Errorf("Run(%v) stdout = %q, want planned command section", []string{"help"}, stdout.String())
	}
	availableIndex := strings.Index(stdout.String(), "Available commands:")
	plannedIndex := strings.Index(stdout.String(), "Planned commands:")
	for _, command := range []string{"profile", "scan", "push", "verify", "deleted", "health"} {
		commandIndex := strings.Index(stdout.String(), "\n  "+command+" ")
		if commandIndex == -1 {
			t.Errorf("Run(%v) stdout = %q, want available command %q", []string{"help"}, stdout.String(), command)
		} else if commandIndex < availableIndex || commandIndex > plannedIndex {
			t.Errorf("Run(%v) stdout = %q, command %q should be listed as available", []string{"help"}, stdout.String(), command)
		}
	}
	for _, command := range []string{"serve", "pair", "prune", "recover", "status"} {
		commandIndex := strings.Index(stdout.String(), "\n  "+command+" ")
		if commandIndex == -1 {
			t.Errorf("Run(%v) stdout = %q, want planned command %q", []string{"help"}, stdout.String(), command)
		} else if commandIndex < plannedIndex {
			t.Errorf("Run(%v) stdout = %q, command %q should be listed as planned", []string{"help"}, stdout.String(), command)
		}
	}
	if strings.Contains(stdout.String(), "Core commands:") {
		t.Errorf("Run(%v) stdout = %q, should not label planned commands as core", []string{"help"}, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("Run(%v) stderr = %q, want empty", []string{"help"}, stderr.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"missing"}, &stdout, &stderr)

	if got != 2 {
		t.Errorf("Run(%v) exit = %d, want %d", []string{"missing"}, got, 2)
	}
	if !strings.Contains(stderr.String(), `unknown command "missing"`) {
		t.Errorf("Run(%v) stderr = %q, want unknown command message", []string{"missing"}, stderr.String())
	}
}

func TestProfileInitAndLint(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"profile", "init", "--profile", profilePath, "--source", source, "--target", target}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile init exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if p.Roots[0].Path != source {
		t.Errorf("profile root path = %q, want %q", p.Roots[0].Path, source)
	}
	if p.Target.LocalPath != target {
		t.Errorf("profile target local path = %q, want %q", p.Target.LocalPath, target)
	}
	if p.Target.TargetID == filepath.Clean(target) {
		t.Errorf("profile target id = %q, want identity separate from local path", p.Target.TargetID)
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"profile", "lint", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile lint exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "profile ok") {
		t.Errorf("profile lint stdout = %q, want ok summary", stdout.String())
	}
}

func TestProfileSetTargetUpdatesProfileSSOT(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	nextTarget := filepath.Join(dir, "next-target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)
	before, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) before set-target error = %v, want nil", profilePath, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"profile", "set-target", "--profile", profilePath, "--target", nextTarget, "--name", "Next target"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile set-target exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if p.Target.LocalPath != nextTarget {
		t.Errorf("profile target local path = %q, want %q", p.Target.LocalPath, nextTarget)
	}
	if p.Target.Name != "Next target" {
		t.Errorf("profile target name = %q, want %q", p.Target.Name, "Next target")
	}
	if p.Target.TargetID != before.Target.TargetID {
		t.Errorf("profile target id = %q, want unchanged %q without --target-id", p.Target.TargetID, before.Target.TargetID)
	}
}

func TestProfileSetTargetExplicitlyUpdatesTargetID(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	nextTarget := filepath.Join(dir, "next-target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"profile", "set-target", "--profile", profilePath, "--target", nextTarget, "--target-id", "local:next-target"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile set-target --target-id exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if p.Target.TargetID != "local:next-target" {
		t.Errorf("profile target id = %q, want local:next-target", p.Target.TargetID)
	}
	if p.Target.LocalPath != nextTarget {
		t.Errorf("profile target local path = %q, want %q", p.Target.LocalPath, nextTarget)
	}
}

func TestProfileSetTargetRepairsLegacyPathTargetID(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	nextTarget := filepath.Join(dir, "next-target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Target.TargetID = filepath.Clean(target)
	data := `{
  "version": 1,
  "profile_id": "` + p.ProfileID + `",
  "name": "` + p.Name + `",
  "roots": [{"id": "root", "path": "` + filepath.ToSlash(source) + `"}],
  "include": [{"pattern": "**"}],
  "consistency": "strict",
  "delete_policy": {"mode": "record", "require_review": true, "retention_days": 30},
  "metadata_policy": {"mode": "basic", "preserve_permissions": true, "preserve_mod_time": true},
  "privacy_policy": {"mode": "plaintext", "traffic_level": 2, "allow_plaintext_restore": true, "allow_hidden_files": true, "allow_sensitive_filenames": true, "padding_bucket_bytes": 65536, "batch_max_bytes": 1048576, "batch_max_count": 64, "jitter_budget_millis": 250, "discovery_low_info": true},
  "target": {"target_id": "` + filepath.ToSlash(target) + `", "name": "target", "local_path": "` + filepath.ToSlash(target) + `"},
  "agent_knowledge": {}
}
`
	if err := os.WriteFile(profilePath, []byte(data), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	if _, err := profile.ReadFile(profilePath); err == nil {
		t.Fatalf("profile.ReadFile(legacy path identity) error = nil, want validation error before repair")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"profile", "set-target", "--profile", profilePath, "--target", nextTarget, "--target-id", "local:repaired"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile set-target repair exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	repaired, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) after repair error = %v, want nil", profilePath, err)
	}
	if repaired.Target.TargetID != "local:repaired" || repaired.Target.LocalPath != filepath.Clean(nextTarget) {
		t.Fatalf("repaired target = %#v, want explicit id and next target path", repaired.Target)
	}
}

func TestScanUsesProfileRoots(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "AGENTS.md"), "rules")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"scan", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("scan exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "entries=2") {
		t.Errorf("scan stdout = %q, want entry count", stdout.String())
	}
	if !strings.Contains(stdout.String(), "influences=1") {
		t.Errorf("scan stdout = %q, want influence count", stdout.String())
	}
}

func TestScanRejectsUnsupportedSelectionRules(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Exclude = []profile.Rule{{Pattern: "*.tmp"}}
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"scan", "--profile", profilePath}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("scan exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "exclude rules are not implemented") {
		t.Fatalf("scan stderr = %q, want unsupported exclude error", stderr.String())
	}
}

func TestPushLocalTargetWritesFilesAndControlArtifacts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "notes", "a.md"), "hello")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"push", "--profile", profilePath, "--session", "session-test"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if gotBytes, err := os.ReadFile(filepath.Join(target, "notes", "a.md")); err != nil || string(gotBytes) != "hello" {
		t.Fatalf("target file = (%q, %v), want hello", string(gotBytes), err)
	}
	manifestPath, err := control.Path(target, control.ArtifactManifest, "session-test")
	if err != nil {
		t.Fatalf("control.Path() error = %v, want nil", err)
	}
	manifest, err := control.ReadFile[control.Manifest](manifestPath)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", manifestPath, err)
	}
	if len(manifest.Entries) == 0 {
		t.Fatalf("manifest entries = 0, want copied file entry")
	}
}

func TestPushDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"push", "--profile", profilePath, "--dry-run"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("push --dry-run exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName)); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(.supermover) error = %v, want os.ErrNotExist", err)
	}
}

func TestPushDryRunRejectsNestedTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(source, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"push", "--profile", profilePath, "--dry-run"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("push --dry-run nested target exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "target directory must not be inside the source root") {
		t.Fatalf("push --dry-run stderr = %q, want nested target error", stderr.String())
	}
}

func TestVerifyReportsPublishedSession(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-test"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"verify", "--profile", profilePath, "--session", "session-test"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("verify exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "files=1/1") {
		t.Fatalf("verify stdout = %q, want file verification summary", stdout.String())
	}
}

func TestVerifyReturnsFailureForMissingFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-test"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if err := os.Remove(filepath.Join(target, "file.txt")); err != nil {
		t.Fatalf("os.Remove(target file) error = %v, want nil", err)
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"verify", "--profile", profilePath, "--session", "session-test"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("verify exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "missing_file") {
		t.Fatalf("verify stdout = %q, want missing_file finding", stdout.String())
	}
}

func TestDeletedListShowsSoftDeleteRecords(t *testing.T) {
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
	got := Run([]string{"push", "--profile", profilePath, "--session", "session-two"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("second push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "deleted=1") {
		t.Fatalf("second push stdout = %q, want deleted=1", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"deleted", "list", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("deleted list exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "source=gone.txt") {
		t.Fatalf("deleted list stdout = %q, want gone.txt soft delete", stdout.String())
	}
}

func TestHealthReportsHealthyTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"health", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("health exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "healthy=true") {
		t.Fatalf("health stdout = %q, want healthy=true", stdout.String())
	}
}

func TestHealthReturnsFailureForIncompleteSessions(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	writeSessionRecord(t, layout, "session-recover", transaction.StateStaged)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"health", "--profile", profilePath}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("health incomplete exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "session-recover") || !strings.Contains(stdout.String(), "action=recover") {
		t.Fatalf("health stdout = %q, want recovery item", stdout.String())
	}
}

func writeDefaultProfile(t *testing.T, path string, source string, target string) {
	t.Helper()
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	if err := profile.WriteFile(path, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", path, err)
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
}

func writeSessionRecord(t *testing.T, layout transaction.Layout, id string, state transaction.State) {
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
