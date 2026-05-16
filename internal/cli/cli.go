package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/agentkb"
	"github.com/khicago/supermover/internal/buildinfo"
	"github.com/khicago/supermover/internal/health"
	"github.com/khicago/supermover/internal/localpush"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/verify"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	runner := Runner{}
	return runner.Run(args, stdout, stderr)
}

type Runner struct {
	Now       time.Time
	SessionID string
}

func (r Runner) Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version", "--version":
		fmt.Fprintf(stdout, "%s %s\n", buildinfo.Name, buildinfo.Version)
		return 0
	case "profile":
		return r.runProfile(args[1:], stdout, stderr)
	case "scan":
		return r.runScan(args[1:], stdout, stderr)
	case "push":
		return r.runPush(args[1:], stdout, stderr)
	case "verify":
		return r.runVerify(args[1:], stdout, stderr)
	case "deleted":
		return r.runDeleted(args[1:], stdout, stderr)
	case "health":
		return r.runHealth(args[1:], stdout, stderr)
	case "recover":
		return r.runRecover(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "%s: unknown command %q\n", buildinfo.Name, args[0])
		printUsage(stderr)
		return 2
	}
}

func (r Runner) runProfile(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "profile: missing subcommand")
		printProfileUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printProfileUsage(stdout)
		return 0
	case "init":
		return r.runProfileInit(args[1:], stdout, stderr)
	case "lint":
		return r.runProfileLint(args[1:], stdout, stderr)
	case "set-target":
		return r.runProfileSetTarget(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "profile: unknown subcommand %q\n", args[0])
		printProfileUsage(stderr)
		return 2
	}
}

func (r Runner) runProfileInit(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("profile init", stderr)
	profilePath := fs.String("profile", "", "profile path to create")
	sourceRoot := fs.String("source", "", "source root to persist in the profile")
	targetRoot := fs.String("target", "", "trusted local target directory to persist")
	targetID := fs.String("target-id", "", "stable target identity to persist")
	profileID := fs.String("id", "profile-local", "profile id to persist")
	name := fs.String("name", "Local profile", "human-readable profile name")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "profile init: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if *profilePath == "" || *sourceRoot == "" || *targetRoot == "" {
		fmt.Fprintln(stderr, "profile init: --profile, --source, and --target are required")
		return 2
	}
	if _, err := os.Stat(*profilePath); err == nil {
		fmt.Fprintf(stderr, "profile init: %s already exists\n", *profilePath)
		return 2
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "profile init: stat %s: %v\n", *profilePath, err)
		return 1
	}
	p := profile.NewDefault(*profileID, *name, *sourceRoot, *targetRoot)
	if strings.TrimSpace(*targetID) != "" {
		p.Target.TargetID = *targetID
	}
	if err := profile.WriteFile(*profilePath, p); err != nil {
		fmt.Fprintf(stderr, "profile init: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote profile %s\n", *profilePath)
	return 0
}

func (r Runner) runProfileLint(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("profile lint", stderr)
	profilePath := fs.String("profile", "", "profile path to lint")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "profile lint: --profile is required")
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "profile lint: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "profile ok: %s (%d roots)\n", p.ProfileID, len(p.Roots))
	return 0
}

func (r Runner) runProfileSetTarget(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("profile set-target", stderr)
	profilePath := fs.String("profile", "", "profile path to update")
	targetPath := fs.String("target", "", "trusted local target directory to persist")
	targetID := fs.String("target-id", "", "target identity override")
	name := fs.String("name", "", "human-readable target name override")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "profile set-target: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if *profilePath == "" || *targetPath == "" {
		fmt.Fprintln(stderr, "profile set-target: --profile and --target are required")
		return 2
	}

	p, err := readProfileForSetTarget(*profilePath, *targetID)
	if err != nil {
		fmt.Fprintf(stderr, "profile set-target: %v\n", err)
		return 2
	}
	oldLocalPath := p.Target.LocalPath
	cleanTarget := filepath.Clean(*targetPath)
	p.Target.LocalPath = cleanTarget
	if strings.TrimSpace(*targetID) != "" {
		p.Target.TargetID = *targetID
	}
	if strings.TrimSpace(*name) != "" {
		p.Target.Name = *name
	} else if strings.TrimSpace(p.Target.Name) == "" || p.Target.Name == filepath.Base(filepath.Clean(oldLocalPath)) {
		p.Target.Name = filepath.Base(cleanTarget)
	}
	if err := profile.WriteFile(*profilePath, p); err != nil {
		fmt.Fprintf(stderr, "profile set-target: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "updated profile target %s\n", *profilePath)
	return 0
}

func readProfileForSetTarget(path string, targetID string) (profile.Profile, error) {
	p, err := profile.ReadFile(path)
	if err == nil {
		return p, nil
	}
	if strings.TrimSpace(targetID) == "" {
		return profile.Profile{}, err
	}
	return profile.ReadFileForTargetRepair(path)
}

func (r Runner) runScan(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("scan", stderr)
	profilePath := fs.String("profile", "", "profile path")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "scan: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "scan: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "scan: %v\n", err)
		return 2
	}
	if err := localpush.ValidateSupportedRules(p); err != nil {
		fmt.Fprintf(stderr, "scan: %v\n", err)
		return 2
	}
	report, err := scanProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "scan: %v\n", err)
		return 1
	}
	switch *format {
	case "text":
		printScanText(stdout, report)
	case "json":
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			fmt.Fprintf(stderr, "scan: encode report: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "scan: unsupported format %q\n", *format)
		return 2
	}
	return 0
}

func (r Runner) runPush(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("push", stderr)
	profilePath := fs.String("profile", "", "profile path")
	dryRun := fs.Bool("dry-run", false, "scan and report without writing target files or control-plane artifacts")
	sessionID := fs.String("session", "", "session id for deterministic tests and controlled reruns")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "push: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "push: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "push: %v\n", err)
		return 2
	}
	if err := localpush.ValidateProfileForLocalPush(p); err != nil {
		fmt.Fprintf(stderr, "push: %v\n", err)
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "push: %v\n", err)
		return 2
	}
	if *dryRun {
		result, err := localpush.Preflight(localpush.Options{Profile: p, TargetDir: targetDir, SessionID: *sessionID, Now: r.Now})
		if err != nil {
			fmt.Fprintf(stderr, "push: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "dry run: profile=%s roots=%d entries=%d warnings=%d influences=%d deleted=%d target=%s\n", p.ProfileID, len(p.Roots), result.Entries, result.Warnings, result.Influences, result.Deleted, targetDir)
		return 0
	}
	effectiveSessionID := *sessionID
	if effectiveSessionID == "" {
		effectiveSessionID = r.SessionID
	}
	result, err := localpush.Run(localpush.Options{Profile: p, TargetDir: targetDir, SessionID: effectiveSessionID, Now: r.Now})
	if err != nil {
		fmt.Fprintf(stderr, "push: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "published session %s: entries=%d copied=%d warnings=%d influences=%d deleted=%d\n", result.SessionID, result.Entries, result.Copied, result.Warnings, result.Influences, result.Deleted)
	return 0
}

func (r Runner) runVerify(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("verify", stderr)
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "session id to verify")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "verify: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "verify: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "verify: %v\n", err)
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "verify: %v\n", err)
		return 2
	}
	report, err := verify.BuildReport(verify.Options{
		TargetRoot: targetDir,
		SessionID:  *sessionID,
		ProfileID:  p.ProfileID,
		TargetID:   p.Target.TargetID,
	})
	if err != nil {
		fmt.Fprintf(stderr, "verify: %v\n", err)
		return 1
	}
	switch *format {
	case "text":
		printVerifyText(stdout, report)
	case "json":
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			fmt.Fprintf(stderr, "verify: encode report: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "verify: unsupported format %q\n", *format)
		return 2
	}
	if report.Summary.ErrorFindings > 0 || report.Summary.ArtifactProblems > 0 {
		return 1
	}
	if report.Summary.ManifestCount == 0 {
		return 1
	}
	return 0
}

func (r Runner) runDeleted(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "deleted: missing subcommand")
		printDeletedUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printDeletedUsage(stdout)
		return 0
	case "list":
		return r.runDeletedList(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "deleted: unknown subcommand %q\n", args[0])
		printDeletedUsage(stderr)
		return 2
	}
}

func (r Runner) runDeletedList(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("deleted list", stderr)
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "optional session id filter")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "deleted list: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "deleted list: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "deleted list: %v\n", err)
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "deleted list: %v\n", err)
		return 2
	}
	report, err := verify.BuildReport(verify.Options{
		TargetRoot: targetDir,
		SessionID:  *sessionID,
		ProfileID:  p.ProfileID,
		TargetID:   p.Target.TargetID,
	})
	if err != nil {
		fmt.Fprintf(stderr, "deleted list: %v\n", err)
		return 1
	}
	switch *format {
	case "text":
		printDeletedText(stdout, report)
	case "json":
		if err := json.NewEncoder(stdout).Encode(report.SoftDeletes); err != nil {
			fmt.Fprintf(stderr, "deleted list: encode report: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "deleted list: unsupported format %q\n", *format)
		return 2
	}
	return 0
}

func (r Runner) runHealth(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("health", stderr)
	profilePath := fs.String("profile", "", "profile path")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "health: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "health: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "health: %v\n", err)
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "health: %v\n", err)
		return 2
	}
	report, err := health.BuildReport(health.Options{TargetRoot: targetDir})
	if err != nil {
		fmt.Fprintf(stderr, "health: %v\n", err)
		return 1
	}
	switch *format {
	case "text":
		printHealthText(stdout, report)
	case "json":
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			fmt.Fprintf(stderr, "health: encode report: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "health: unsupported format %q\n", *format)
		return 2
	}
	if !report.Healthy {
		return 1
	}
	return 0
}

func (r Runner) runRecover(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("recover", stderr)
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "optional session id to recover")
	dryRun := fs.Bool("dry-run", false, "report recovery actions without mutating target state")
	rollbackIncomplete := fs.Bool("rollback-incomplete", false, "mark received/validated sessions as rolled_back when they never reached durable staging")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "recover: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "recover: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "recover: %v\n", err)
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "recover: %v\n", err)
		return 2
	}
	result, err := localpush.Recover(localpush.RecoverOptions{
		Profile:            p,
		TargetDir:          targetDir,
		SessionID:          *sessionID,
		DryRun:             *dryRun,
		RollbackIncomplete: *rollbackIncomplete,
		Now:                r.Now,
	})
	if err != nil {
		fmt.Fprintf(stderr, "recover: %v\n", err)
		return 1
	}
	switch *format {
	case "text":
		printRecoverText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "recover: encode result: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "recover: unsupported format %q\n", *format)
		return 2
	}
	if result.RepairNeeded > 0 {
		return 1
	}
	return 0
}

func targetDirFromProfile(p profile.Profile) (string, error) {
	if strings.TrimSpace(p.Target.LocalPath) != "" {
		return p.Target.LocalPath, nil
	}
	return "", fmt.Errorf("target.local_path is required; run profile set-target to persist the trusted target path")
}

type scanReport struct {
	ProfileID      string              `json:"profile_id"`
	Roots          []scan.Result       `json:"roots"`
	EntryCount     int                 `json:"entry_count"`
	WarningCount   int                 `json:"warning_count"`
	InfluenceCount int                 `json:"influence_count"`
	Influence      []agentkb.Influence `json:"influence,omitempty"`
}

func scanProfile(p profile.Profile) (scanReport, error) {
	report := scanReport{ProfileID: p.ProfileID}
	for _, root := range p.Roots {
		result, err := scan.Scan(root.Path)
		if err != nil {
			return scanReport{}, err
		}
		report.EntryCount += len(result.Entries)
		report.WarningCount += len(result.Audit)
		report.Influence = append(report.Influence, agentkb.Detect(result.Entries)...)
		report.Roots = append(report.Roots, result)
	}
	report.InfluenceCount = len(report.Influence)
	return report, nil
}

func printScanText(w io.Writer, report scanReport) {
	fmt.Fprintf(w, "profile=%s roots=%d entries=%d warnings=%d influences=%d\n", report.ProfileID, len(report.Roots), report.EntryCount, report.WarningCount, report.InfluenceCount)
	for _, root := range report.Roots {
		fmt.Fprintf(w, "root=%s entries=%d warnings=%d\n", root.Root, len(root.Entries), len(root.Audit))
	}
}

func printVerifyText(w io.Writer, report verify.Report) {
	fmt.Fprintf(w, "verify: target=%s session=%s manifests=%d files=%d/%d errors=%d warnings=%d soft_deletes=%d artifact_problems=%d\n",
		report.TargetRoot,
		report.Manifest.SessionID,
		report.Summary.ManifestCount,
		report.Summary.FilesVerified,
		report.Summary.FilesExpected,
		report.Summary.ErrorFindings,
		report.Summary.WarningFindings+report.Summary.Warnings,
		report.Summary.SoftDeletes,
		report.Summary.ArtifactProblems,
	)
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "%s %s path=%s target=%s message=%s\n", finding.Severity, finding.Kind, finding.Path, finding.TargetPath, finding.Message)
	}
	for _, problem := range report.ArtifactProblems {
		fmt.Fprintf(w, "error artifact_problem path=%s message=%s\n", problem.Path, problem.Err)
	}
}

func printDeletedText(w io.Writer, report verify.Report) {
	fmt.Fprintf(w, "soft deletes: count=%d target=%s\n", len(report.SoftDeletes), report.TargetRoot)
	for _, record := range report.SoftDeletes {
		fmt.Fprintf(w, "%s session=%s profile=%s target_id=%s root=%s previous_session=%s previous_manifest=%s source=%s target=%s kind=%s size=%d digest=%s detected_at=%s\n",
			record.ID,
			record.SessionID,
			record.ProfileID,
			record.TargetID,
			record.RootID,
			record.PreviousSessionID,
			record.PreviousManifestID,
			record.SourcePath,
			record.TargetPath,
			record.Kind,
			record.Size,
			record.Digest,
			record.DetectedAt,
		)
	}
}

func printHealthText(w io.Writer, report health.Report) {
	fmt.Fprintf(w, "health: target=%s healthy=%t incomplete_sessions=%d invalid_records=%d artifact_problems=%d\n",
		report.TargetRoot,
		report.Healthy,
		report.Summary.IncompleteSessions,
		report.Summary.InvalidRecords,
		report.Summary.ArtifactProblems,
	)
	for _, item := range report.Items {
		fmt.Fprintf(w, "%s state=%s action=%s reason=%s path=%s\n", item.SessionID, item.State, item.Action, item.Reason, item.Path)
	}
	for _, invalid := range report.Invalid {
		fmt.Fprintf(w, "invalid path=%s error=%s\n", invalid.Path, invalid.Error)
	}
	for _, artifact := range report.Artifacts {
		fmt.Fprintf(w, "artifact session=%s path=%s error=%s\n", artifact.SessionID, artifact.Path, artifact.Error)
	}
}

func printRecoverText(w io.Writer, result localpush.RecoverResult) {
	fmt.Fprintf(w, "recover: target=%s dry_run=%t inspected=%d recovered=%d skipped=%d repair_needed=%d\n",
		result.TargetDir,
		result.DryRun,
		result.Inspected,
		result.Recovered,
		result.Skipped,
		result.RepairNeeded,
	)
	for _, item := range result.Items {
		fmt.Fprintf(w, "%s state=%s action=%s status=%s message=%s\n", item.SessionID, item.State, item.Action, item.Status, item.Message)
	}
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func printProfileUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  supermover profile init --profile <path> --source <path> --target <path> [--target-id <id>]
  supermover profile lint --profile <path>
  supermover profile set-target --profile <path> --target <path> [--target-id <id>]`)
}

func printDeletedUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  supermover deleted list --profile <path>`)
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `%s - %s

Usage:
  supermover <command> [flags]

Available commands:
  profile     Manage profile SSOT configuration
  scan        Scan configured profile roots without writing target state
  push        Push source roots to the local target recorded in the profile
  verify      Verify manifests and restored files
  deleted     Review source-side soft-delete records
  health      Inspect target control-plane health
  recover     Resume safe local sessions or mark incomplete sessions

Planned commands:
  serve       Run a trusted network target receiver
  pair        Pair with a target by explicit verification
  prune       Apply reviewed physical pruning after policy checks
  status      Show local profile/session status
  discover    Find local targets without trusting them
  drift       Review target-local drift

Use "supermover help" for this overview.
`, buildinfo.Name, buildinfo.Description)
}
