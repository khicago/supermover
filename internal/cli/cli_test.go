package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/verify"
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
	for _, command := range []string{"profile", "scan", "push", "verify", "deleted", "health", "recover"} {
		commandIndex := strings.Index(stdout.String(), "\n  "+command+" ")
		if commandIndex == -1 {
			t.Errorf("Run(%v) stdout = %q, want available command %q", []string{"help"}, stdout.String(), command)
		} else if commandIndex < availableIndex || commandIndex > plannedIndex {
			t.Errorf("Run(%v) stdout = %q, command %q should be listed as available", []string{"help"}, stdout.String(), command)
		}
	}
	for _, command := range []string{"serve", "pair", "prune", "status", "discover", "drift"} {
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

func TestLeafHelpReturnsSuccess(t *testing.T) {
	tests := [][]string{
		{"profile", "--help"},
		{"profile", "init", "--help"},
		{"profile", "lint", "--help"},
		{"profile", "set-target", "--help"},
		{"scan", "--help"},
		{"push", "--help"},
		{"verify", "--help"},
		{"deleted", "--help"},
		{"deleted", "list", "--help"},
		{"health", "--help"},
		{"recover", "--help"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(args, &stdout, &stderr)

			if got != 0 {
				t.Fatalf("Run(%v) exit = %d, want 0; stdout=%q stderr=%q", args, got, stdout.String(), stderr.String())
			}
			if stdout.Len() == 0 && stderr.Len() == 0 {
				t.Fatalf("Run(%v) produced no help output, want usage text", args)
			}
		})
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

func TestScanJSONDoesNotExposeObservedIdentity(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"scan", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("scan --format json exit = %d, stderr = %q, want 0", got, stderr.String())
	}

	var report struct {
		Roots []struct {
			Entries []map[string]any `json:"entries"`
		} `json:"roots"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal(scan stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if len(report.Roots) != 1 || len(report.Roots[0].Entries) == 0 {
		t.Fatalf("scan --format json roots = %#v, want one root with entries", report.Roots)
	}
	for _, entry := range report.Roots[0].Entries {
		if _, ok := entry["observed"]; ok {
			t.Fatalf("scan --format json entry keys = %#v, want no observed field", entry)
		}
		if _, ok := entry["Observed"]; ok {
			t.Fatalf("scan --format json entry keys = %#v, want no Observed field", entry)
		}
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

func TestPushDryRunReportsSoftDeletes(t *testing.T) {
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

	got := Run([]string{"push", "--profile", profilePath, "--dry-run", "--session", "session-two"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("push --dry-run soft delete exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "deleted=1") {
		t.Fatalf("push --dry-run stdout = %q, want deleted=1", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(target, control.DirName, "deleted")); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(deleted dir after dry-run) error = %v, want os.ErrNotExist", err)
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

func TestPushDryRunRejectsDivergentExistingTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "source")
	mustWrite(t, filepath.Join(target, "file.txt"), "target")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"push", "--profile", profilePath, "--dry-run"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("push --dry-run divergent target exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "refusing to overwrite") {
		t.Fatalf("push --dry-run divergent target stderr = %q, want refusing to overwrite", stderr.String())
	}
}

func TestPushDryRunRejectsMultipleRoots(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	sourceTwo := filepath.Join(dir, "source-two")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, sourceTwo)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Roots = append(p.Roots, profile.Root{ID: "root-two", Path: sourceTwo})
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"push", "--profile", profilePath, "--dry-run"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("push --dry-run multi-root exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "exactly one root") {
		t.Fatalf("push --dry-run multi-root stderr = %q, want exactly one root error", stderr.String())
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

func TestVerifyRejectsSessionFromDifferentProfile(t *testing.T) {
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

	got := Run([]string{"verify", "--profile", profileB, "--session", "session-test"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("verify profile-b exit = %d, stdout = %q, stderr = %q, want 1", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "does not match requested profile/target") {
		t.Fatalf("verify profile-b stderr = %q, want profile/target mismatch", stderr.String())
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

func TestVerifyReturnsFailureForWarningFinding(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(target, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)
	manifest := control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session",
		SessionID: "session",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{{Path: "file.txt", TargetPath: "file.txt", Kind: "file", Size: 7}},
	}
	manifestPath, err := control.Path(target, control.ArtifactManifest, "session")
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v, want nil", err)
	}
	if err := control.WriteFile(manifestPath, manifest); err != nil {
		t.Fatalf("control.WriteFile(manifest) error = %v, want nil", err)
	}
	receipt := control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        "session",
		ProfileID: "profile-local",
		TargetID:  "local:profile-local",
		StartedAt: "2026-05-16T00:00:00Z",
		Status:    "published",
	}
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, "session")
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	if err := control.WriteFile(receiptPath, receipt); err != nil {
		t.Fatalf("control.WriteFile(receipt) error = %v, want nil", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"verify", "--profile", profilePath, "--session", "session"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("verify warning finding exit = %d, stdout = %q, stderr = %q, want 1", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), string(verify.FindingDigestMissing)) {
		t.Fatalf("verify warning finding stdout = %q, want digest_missing", stdout.String())
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
	for _, want := range []string{"profile=profile-local", "target_id=local:profile-local", "root=root", "previous_session=session-one", "previous_manifest=manifest-session-one", "kind=file", "size=4", "digest=sha256:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("deleted list stdout = %q, want evidence field %q", stdout.String(), want)
		}
	}
}

func TestDeletedListRejectsSessionFromDifferentProfile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profileA := filepath.Join(dir, "a.json")
	profileB := filepath.Join(dir, "b.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "keep.txt"), "keep")
	mustWrite(t, filepath.Join(source, "gone.txt"), "gone")
	if err := profile.WriteFile(profileA, profile.NewDefault("profile-a", "Profile A", source, target)); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profileA, err)
	}
	if err := profile.WriteFile(profileB, profile.NewDefault("profile-b", "Profile B", source, target)); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profileB, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profileA, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("first push profile-a exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if err := os.Remove(filepath.Join(source, "gone.txt")); err != nil {
		t.Fatalf("os.Remove(source gone) error = %v, want nil", err)
	}
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"push", "--profile", profileA, "--session", "session-two"}, &stdout, &stderr); got != 0 {
		t.Fatalf("second push profile-a exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"deleted", "list", "--profile", profileB, "--session", "session-two"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("deleted list profile-b exit = %d, stdout = %q, stderr = %q, want 1", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "does not match requested profile/target") {
		t.Fatalf("deleted list profile-b stderr = %q, want profile/target mismatch", stderr.String())
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

func TestRecoverDryRunReportsIncompleteSession(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	writeSessionRecord(t, layout, "session-incomplete", transaction.StateValidated)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"recover", "--profile", profilePath, "--session", "session-incomplete", "--dry-run"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("recover --dry-run exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "session-incomplete") || !strings.Contains(stdout.String(), "status=would_rollback") {
		t.Fatalf("recover --dry-run stdout = %q, want would_rollback item", stdout.String())
	}
	record, err := transaction.ReadSessionRecord(layout.RecordPath("session-incomplete"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(%q) error = %v, want nil", "session-incomplete", err)
	}
	if record.State != transaction.StateValidated {
		t.Fatalf("recover --dry-run state = %q, want unchanged validated", record.State)
	}
}

func TestRecoverRollbackIncompleteUpdatesSession(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	layout := transaction.NewLayout(control.ControlDir(target))
	writeSessionRecord(t, layout, "session-incomplete", transaction.StateValidated)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"recover", "--profile", profilePath, "--session", "session-incomplete", "--rollback-incomplete"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("recover --rollback-incomplete exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "status=rolled_back") {
		t.Fatalf("recover --rollback-incomplete stdout = %q, want rolled_back", stdout.String())
	}
	record, err := transaction.ReadSessionRecord(layout.RecordPath("session-incomplete"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(%q) error = %v, want nil", "session-incomplete", err)
	}
	if record.State != transaction.StateRolledBack {
		t.Fatalf("recover --rollback-incomplete state = %q, want rolled_back", record.State)
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
