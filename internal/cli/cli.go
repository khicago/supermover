package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/khicago/supermover/internal/agentdaemon"
	"github.com/khicago/supermover/internal/agentkb"
	"github.com/khicago/supermover/internal/audit"
	"github.com/khicago/supermover/internal/buildinfo"
	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/discovery"
	"github.com/khicago/supermover/internal/driftreview"
	"github.com/khicago/supermover/internal/health"
	"github.com/khicago/supermover/internal/incrementalsync"
	"github.com/khicago/supermover/internal/localpush"
	"github.com/khicago/supermover/internal/networkpush"
	"github.com/khicago/supermover/internal/operatorui"
	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/pairserve"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/prune"
	"github.com/khicago/supermover/internal/receiverserve"
	"github.com/khicago/supermover/internal/reconcile"
	"github.com/khicago/supermover/internal/report"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/status"
	"github.com/khicago/supermover/internal/targetlock"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
	"github.com/khicago/supermover/internal/verify"
)

const maxPairingBootstrapBytes = 64 * 1024

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return RunContext(context.Background(), args, stdout, stderr)
}

func RunContext(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	runner := Runner{Context: ctx}
	return runner.Run(args, stdout, stderr)
}

type Runner struct {
	Now                time.Time
	SessionID          string
	Context            context.Context
	ServeReady         func(address string)
	ServePairingReady  func(pairserve.ReadyInfo)
	ServeReceiverReady func(receiverserve.ReadyInfo)
	DashboardReady     func(operatorui.ReadyInfo)
	DaemonReady        func(agentdaemon.State)
	// DaemonRestartConsumed lets tests synchronize with the narrow foreground restart window.
	DaemonRestartConsumed func(agentdaemon.State)
	// receiverListenerForTest lets CLI smoke tests hold a receiver port until serve starts.
	receiverListenerForTest net.Listener
}

func (r Runner) nowFunc() func() time.Time {
	return func() time.Time {
		if r.Now.IsZero() {
			return time.Now().UTC()
		}
		return r.Now.UTC()
	}
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
	case "dashboard":
		return r.runDashboard(args[1:], stdout, stderr)
	case "drift":
		return r.runDrift(args[1:], stdout, stderr)
	case "deleted":
		return r.runDeleted(args[1:], stdout, stderr)
	case "prune":
		return r.runPrune(args[1:], stdout, stderr)
	case "reconcile":
		return r.runReconcile(args[1:], stdout, stderr)
	case "health":
		return r.runHealth(args[1:], stdout, stderr)
	case "report":
		return r.runReport(args[1:], stdout, stderr)
	case "status":
		return r.runStatus(args[1:], stdout, stderr)
	case "recover":
		return r.runRecover(args[1:], stdout, stderr)
	case "serve":
		return r.runServe(args[1:], stdout, stderr)
	case "daemon":
		return r.runDaemon(args[1:], stdout, stderr)
	case "sync":
		return r.runSync(args[1:], stdout, stderr)
	case "discover":
		return r.runDiscover(args[1:], stdout, stderr)
	case "pair":
		return r.runPair(args[1:], stdout, stderr)
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
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
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
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
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
	privacyState := report.PrivacyForProfile(&p)
	fmt.Fprintf(stdout, "profile ok: %s (%d roots)\n", p.ProfileID, len(p.Roots))
	fmt.Fprintf(stdout, "privacy policy=status=%s mode=%s traffic_level=%d claim=%s configured_reductions=%s overhead_status=%s overhead_source=%s overhead_padding_bucket_bytes=%d overhead_batch_max_bytes=%d overhead_batch_max_count=%d overhead_jitter_budget_millis=%d residual_leakage=%s local_push=%s network_transfer=%s\n",
		privacyState.Status,
		privacyState.Mode,
		privacyState.TrafficLevel,
		privacyState.Claim,
		formatStringList(privacyState.ConfiguredReduction),
		privacyState.Overhead.Status,
		privacyState.Overhead.Source,
		privacyState.Overhead.PaddingBucketBytes,
		privacyState.Overhead.BatchMaxBytes,
		privacyState.Overhead.BatchMaxCount,
		privacyState.Overhead.JitterBudgetMillis,
		formatStringList(privacyState.ResidualLeakage),
		privacyState.LocalPush,
		privacyState.NetworkTransfer,
	)
	return 0
}

func (r Runner) runProfileSetTarget(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("profile set-target", stderr)
	profilePath := fs.String("profile", "", "profile path to update")
	targetPath := fs.String("target", "", "trusted local target directory to persist")
	targetID := fs.String("target-id", "", "target identity override")
	name := fs.String("name", "", "human-readable target name override")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
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
	if strings.TrimSpace(*targetID) != "" && strings.TrimSpace(p.Target.PairingReceiptID) != "" {
		fmt.Fprintln(stderr, "profile set-target: cannot change target-id for a paired profile; re-pair the target to rotate identity")
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
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
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
	if err := localpush.ValidateProfileForLocalPush(p); err != nil {
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
	if networkArgs, ok := extractNetworkPushArgs(args); ok {
		return r.runPushNetwork(networkArgs, stdout, stderr)
	}
	fs := newFlagSet("push", stderr)
	profilePath := fs.String("profile", "", "profile path")
	dryRun := fs.Bool("dry-run", false, "scan and report without writing target files or control-plane artifacts")
	sessionID := fs.String("session", "", "session id for deterministic tests and controlled reruns")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
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

func (r Runner) runPushNetwork(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("push --network", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of push --network:
  supermover push --network --profile <path> [--dry-run] [--session <id>] [--format text|json]

Validates the source-initiated network transfer contract and fails closed.
It reads target identity, pairing evidence, delete policy, metadata policy,
privacy policy, receiver URL, and local TLS identity references from the
profile SSOT. Without --dry-run it connects to the profile-selected pinned
mTLS receiver, transfers files, and writes receiver-side network evidence. The
--dry-run flag validates profile, pairing, local TLS identity, scan, and
manifest shape without contacting the receiver or writing target control-plane
artifacts.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	dryRun := fs.Bool("dry-run", false, "validate profile, pairing, local TLS identity, scan, and manifest without contacting the receiver")
	sessionID := fs.String("session", "", "session id for receiver resume context")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "push --network: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "push --network: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "push --network: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "push --network: unsupported format %q\n", *format)
		return 2
	}
	effectiveSessionID := strings.TrimSpace(*sessionID)
	if effectiveSessionID != "" {
		if effectiveSessionID == "-" {
			fmt.Fprintln(stderr, "push --network: session id \"-\" is reserved for absent text output")
			return 2
		}
		if err := transaction.ValidateSessionID(effectiveSessionID); err != nil {
			fmt.Fprintf(stderr, "push --network: %s\n", safeDiagnosticLine(err.Error()))
			return 2
		}
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "push --network: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	trust, err := pairing.ValidateProfileTrust(p)
	if err != nil {
		fmt.Fprintf(stderr, "push --network: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if err := p.ValidateNetworkClientMaterial(); err != nil {
		fmt.Fprintf(stderr, "push --network: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	ctx := r.Context
	if ctx == nil {
		ctx = context.Background()
	}
	opts := networkpush.Options{
		Profile:   p,
		SessionID: effectiveSessionID,
		Now:       r.nowFunc(),
	}
	var result networkpush.Result
	if *dryRun {
		result, err = networkpush.Preflight(ctx, opts)
	} else {
		result, err = networkpush.Run(ctx, opts)
	}
	if err != nil {
		code := 2
		if result.TransferStatus != "" {
			code = 1
		}
		if result.TransferStatus != "" {
			if printErr := printNetworkPushResult(stdout, *format, networkPushResultFromRun(p, trust, *dryRun, result)); printErr != nil {
				fmt.Fprintf(stderr, "push --network: encode result: %s\n", safeDiagnosticLine(printErr.Error()))
				return 1
			}
		}
		fmt.Fprintf(stderr, "push --network: %s\n", safeDiagnosticLine(err.Error()))
		return code
	}
	plan := networkPushResultFromRun(p, trust, *dryRun, result)
	switch *format {
	case "text":
		printNetworkPushPlanText(stdout, plan)
	case "json":
		if err := json.NewEncoder(stdout).Encode(plan); err != nil {
			fmt.Fprintf(stderr, "push --network: encode plan: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return 0
}

func extractNetworkPushArgs(args []string) ([]string, bool) {
	for i, arg := range args {
		if arg == "--" {
			return nil, false
		}
		if arg != "--network" {
			continue
		}
		networkArgs := make([]string, 0, len(args)-1)
		networkArgs = append(networkArgs, args[:i]...)
		networkArgs = append(networkArgs, args[i+1:]...)
		return networkArgs, true
	}
	return nil, false
}

func formatDiagnosticArgs(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, formatDiagnosticArg(arg))
	}
	return strings.Join(parts, " ")
}

func formatDiagnosticArg(arg string) string {
	if arg == "" || strings.ContainsAny(arg, " \t\r\n") {
		return strconv.QuoteToASCII(arg)
	}
	return safeDiagnosticLine(arg)
}

func safeDiagnosticLine(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		switch c {
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 0x20 || c == 0x7f {
				fmt.Fprintf(&b, "\\x%02X", c)
				continue
			}
			b.WriteByte(c)
		}
	}
	return b.String()
}

func (r Runner) runVerify(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("verify", stderr)
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "session id to verify")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "verify: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "verify: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "verify: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "verify: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	report, err := verify.BuildReport(verify.Options{
		TargetRoot: targetDir,
		SessionID:  *sessionID,
		ProfileID:  p.ProfileID,
		TargetID:   p.Target.TargetID,
	})
	if err != nil {
		fmt.Fprintf(stderr, "verify: %s\n", safeDiagnosticLine(err.Error()))
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
	if report.Summary.ErrorFindings > 0 ||
		report.Summary.WarningFindings > 0 ||
		report.Summary.Warnings > 0 ||
		report.Summary.SoftDeletes > 0 ||
		report.Summary.TargetDrifts > 0 ||
		report.Summary.ArtifactProblems > 0 {
		return 1
	}
	if report.Summary.ManifestCount == 0 {
		return 1
	}
	return 0
}

func (r Runner) runDashboard(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("dashboard", stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of dashboard:
  supermover dashboard --profile <path> [--listen <loopback-ip:port>]

Serves a local-only read-only operator page that verifies the profile-selected
target against the latest published manifest and scans for extra target paths.
Full verification reads target file content on page load or explicit refresh.
Open only the emitted access-token URL. Remote access must use a trusted local
forwarding mechanism such as SSH while preserving its token query.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "target profile path")
	listen := fs.String("listen", operatorui.DefaultListen, "loopback dashboard listen address")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "dashboard: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "dashboard: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "dashboard: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if _, err := targetDirFromProfile(p); err != nil {
		fmt.Fprintf(stderr, "dashboard: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	server, err := operatorui.New(operatorui.Options{
		Profile: p,
		Listen:  *listen,
		Now:     r.nowFunc(),
		Ready: func(info operatorui.ReadyInfo) {
			fmt.Fprintf(stderr, "dashboard: url=%s loopback_only=true read_only=true check=latest_published_snapshot\n", info.URL)
			if r.DashboardReady != nil {
				r.DashboardReady(info)
			}
		},
	})
	if err != nil {
		fmt.Fprintf(stderr, "dashboard: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	ctx := r.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := server.ListenAndServe(ctx); err != nil {
		fmt.Fprintf(stderr, "dashboard: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	return 0
}

func (r Runner) runDrift(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "drift: missing subcommand")
		printDriftUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printDriftUsage(stdout)
		return 0
	case "list":
		return r.runDriftList(args[1:], stdout, stderr)
	case "record":
		return r.runDriftRecord(args[1:], stdout, stderr)
	case "acknowledge":
		return r.runDriftAcknowledge(args[1:], stdout, stderr)
	case "resolve":
		return r.runDriftResolve(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "drift: unknown subcommand %q\n", args[0])
		printDriftUsage(stderr)
		return 2
	}
}

func (r Runner) runDriftRecord(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("drift record", stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of drift record:
  supermover drift record --profile <path> [--session <id>] [--format text|json]

Runs the live target drift detector against the profile-selected target and writes detected findings as durable .supermover/drift review records.
This records evidence only; it does not resolve, repair, prune, suppress future detector output, or run background scans.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "optional session id filter")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "drift record: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "drift record: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "drift record: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "drift record: unsupported format %q\n", *format)
		return 2
	}
	effectiveSessionID := *sessionID
	if effectiveSessionID != "" {
		if strings.TrimSpace(effectiveSessionID) == "" {
			fmt.Fprintln(stderr, "drift record: session id is required when --session is provided")
			return 2
		}
		if err := transaction.ValidateSessionID(effectiveSessionID); err != nil {
			fmt.Fprintf(stderr, "drift record: %s\n", safeDiagnosticLine(err.Error()))
			return 2
		}
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "drift record: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	result, err := driftreview.Record(driftreview.RecordOptions{
		Profile:   p,
		SessionID: effectiveSessionID,
		Now:       r.Now,
	})
	if err != nil {
		fmt.Fprintf(stderr, "drift record: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "text":
		printDriftRecordText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "drift record: encode result: %v\n", err)
			return 1
		}
	}
	if result.NeedsReview() {
		return 1
	}
	return 0
}

func (r Runner) runDriftList(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("drift list", stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of drift list:
  supermover drift list --profile <path> [--session <id>] [--format text|json]

Runs the read-only live target drift detector against the profile-selected target.
Output is not persisted; use drift record to persist current findings before drift acknowledge can review them.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "optional session id filter")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "drift list: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "drift list: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "drift list: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "drift list: unsupported format %q\n", *format)
		return 2
	}
	effectiveSessionID := *sessionID
	if effectiveSessionID != "" {
		if strings.TrimSpace(effectiveSessionID) == "" {
			fmt.Fprintln(stderr, "drift list: session id is required when --session is provided")
			return 2
		}
		if err := transaction.ValidateSessionID(effectiveSessionID); err != nil {
			fmt.Fprintf(stderr, "drift list: %s\n", safeDiagnosticLine(err.Error()))
			return 2
		}
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "drift list: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "drift list: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	report, err := verify.DetectTargetDrift(verify.DriftOptions{
		TargetRoot: targetDir,
		SessionID:  effectiveSessionID,
		ProfileID:  p.ProfileID,
		TargetID:   p.Target.TargetID,
		Now:        r.Now,
	})
	if err != nil {
		fmt.Fprintf(stderr, "drift list: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "text":
		printDriftText(stdout, report)
	case "json":
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			fmt.Fprintf(stderr, "drift list: encode report: %v\n", err)
			return 1
		}
	}
	if report.NeedsReview() {
		return 1
	}
	return 0
}

func (r Runner) runDriftAcknowledge(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("drift acknowledge", stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of drift acknowledge:
  supermover drift acknowledge --profile <path> --id <persisted-drift-id> --reason <text> [--reviewer <id>] [--format text|json]

Adds acknowledgement metadata to one existing durable .supermover/drift review record.
The id must come from persisted target_drifts evidence, including drift record output; live-only drift list/report.live_target_drift ids are refused.
This records review metadata only; it does not resolve, repair, prune, suppress future detector output, or make the target clean.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	id := fs.String("id", "", "persisted drift id")
	reason := fs.String("reason", "", "operator review reason")
	reviewer := fs.String("reviewer", "", "reviewer identity")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "drift acknowledge: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "drift acknowledge: --profile is required")
		return 2
	}
	if *id == "" {
		fmt.Fprintln(stderr, "drift acknowledge: --id is required")
		return 2
	}
	if err := control.ValidateArtifactID(*id); err != nil {
		fmt.Fprintf(stderr, "drift acknowledge: --id is invalid: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*reason) == "" {
		fmt.Fprintln(stderr, "drift acknowledge: --reason is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "drift acknowledge: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "drift acknowledge: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "drift acknowledge: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	result, err := driftreview.Acknowledge(driftreview.AcknowledgeOptions{
		Profile:  p,
		ID:       *id,
		Reason:   *reason,
		Reviewer: *reviewer,
		Now:      r.Now,
	})
	if err != nil {
		fmt.Fprintf(stderr, "drift acknowledge: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "text":
		printDriftAcknowledgeText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "drift acknowledge: encode result: %v\n", err)
			return 1
		}
	}
	return 0
}

func (r Runner) runDriftResolve(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("drift resolve", stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of drift resolve:
  supermover drift resolve --profile <path> --id <persisted-drift-id> --reason <text> [--reviewer <id>] [--format text|json]

Marks one existing durable .supermover/drift review record resolved only after a fresh live detector no longer reports drift for the persisted path and expected baseline.
The id must come from persisted target_drifts evidence, including drift record output; live-only drift list/report.live_target_drift ids are refused.
This records resolution metadata only; it does not repair target files, rewrite manifests, prune files, or suppress future detector output.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	id := fs.String("id", "", "persisted drift id")
	reason := fs.String("reason", "", "operator resolution reason")
	reviewer := fs.String("reviewer", "", "reviewer identity")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "drift resolve: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "drift resolve: --profile is required")
		return 2
	}
	if *id == "" {
		fmt.Fprintln(stderr, "drift resolve: --id is required")
		return 2
	}
	if err := control.ValidateArtifactID(*id); err != nil {
		fmt.Fprintf(stderr, "drift resolve: --id is invalid: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*reason) == "" {
		fmt.Fprintln(stderr, "drift resolve: --reason is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "drift resolve: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "drift resolve: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "drift resolve: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	result, err := driftreview.Resolve(driftreview.ResolveOptions{
		Profile:  p,
		ID:       *id,
		Reason:   *reason,
		Reviewer: *reviewer,
		Now:      r.Now,
	})
	if err != nil {
		fmt.Fprintf(stderr, "drift resolve: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "text":
		printDriftResolveText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "drift resolve: encode result: %v\n", err)
			return 1
		}
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
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
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

func (r Runner) runReconcile(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "reconcile: missing subcommand")
		printReconcileUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printReconcileUsage(stdout)
		return 0
	case "plan":
		return r.runReconcilePlan(args[1:], stdout, stderr)
	case "apply":
		return r.runReconcileApply(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "reconcile: unknown subcommand %q\n", args[0])
		printReconcileUsage(stderr)
		return 2
	}
}

func (r Runner) runReconcilePlan(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("reconcile plan", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of reconcile plan:
  supermover reconcile plan --profile <path> [--id <persisted-drift-id>...] [--session <id>] [--format text|json]

Builds a non-mutating reconcile plan from durable persisted target-drift
evidence on the profile-selected target.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	ids := multiFlag{}
	fs.Var(&ids, "id", "persisted target drift id to select; repeatable")
	sessionID := fs.String("session", "", "optional session id filter")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "reconcile plan: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	p, selectedIDs, selectedSession, ok := readReconcileCommandInputs("reconcile plan", fs, *profilePath, ids, *sessionID, *format, false, stderr)
	if !ok {
		return 2
	}
	receipt, err := reconcile.Plan(reconcile.Options{
		Profile:   p,
		IDs:       selectedIDs,
		SessionID: selectedSession,
		Now:       r.nowFunc()(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "reconcile plan: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if err := printReconcileReceipt(stdout, stderr, *format, receipt); err != nil {
		fmt.Fprintf(stderr, "reconcile plan: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if receipt.Summary.Refused > 0 || receipt.Summary.ArtifactProblems > 0 {
		return 1
	}
	return 0
}

func (r Runner) runReconcileApply(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("reconcile apply", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of reconcile apply:
  supermover reconcile apply --profile <path> --id <persisted-drift-id> [--id <persisted-drift-id>...] --apply --reason <text> [--reviewer <id>] [--session <id>] [--format text|json]

Applies only selected narrow persisted-drift reconcile actions after explicit
operator intent. This command is not a broad automatic repair runner.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	ids := multiFlag{}
	fs.Var(&ids, "id", "persisted target drift id to apply; repeatable")
	applyIntent := fs.Bool("apply", false, "explicit mutation intent")
	reason := fs.String("reason", "", "operator reason for resolving selected drift")
	reviewer := fs.String("reviewer", "", "reviewer/operator id")
	sessionID := fs.String("session", "", "optional session id filter")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "reconcile apply: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "reconcile apply: --profile is required")
		return 2
	}
	if len(ids) == 0 {
		fmt.Fprintln(stderr, "reconcile apply: at least one --id is required")
		return 2
	}
	if !*applyIntent {
		fmt.Fprintln(stderr, "reconcile apply: --apply is required")
		return 2
	}
	if strings.TrimSpace(*reason) == "" {
		fmt.Fprintln(stderr, "reconcile apply: --reason is required")
		return 2
	}
	p, selectedIDs, selectedSession, ok := readReconcileCommandInputs("reconcile apply", fs, *profilePath, ids, *sessionID, *format, true, stderr)
	if !ok {
		return 2
	}
	receipt, err := reconcile.Apply(reconcile.ApplyOptions{
		Profile:   p,
		IDs:       selectedIDs,
		SessionID: selectedSession,
		Apply:     true,
		Reviewer:  *reviewer,
		Reason:    *reason,
		Now:       r.nowFunc()(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "reconcile apply: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if err := printReconcileReceipt(stdout, stderr, *format, receipt); err != nil {
		fmt.Fprintf(stderr, "reconcile apply: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if receipt.Summary.Refused > 0 || receipt.Summary.ArtifactProblems > 0 {
		return 1
	}
	return 0
}

func readReconcileCommandInputs(command string, fs *flag.FlagSet, profilePath string, ids multiFlag, sessionID string, format string, requireIDs bool, stderr io.Writer) (profile.Profile, []string, string, bool) {
	if strings.TrimSpace(profilePath) == "" {
		fmt.Fprintf(stderr, "%s: --profile is required\n", command)
		return profile.Profile{}, nil, "", false
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "%s: unexpected arguments: %s\n", command, formatDiagnosticArgs(fs.Args()))
		return profile.Profile{}, nil, "", false
	}
	if format != "text" && format != "json" {
		fmt.Fprintf(stderr, "%s: unsupported format %q\n", command, format)
		return profile.Profile{}, nil, "", false
	}
	selectedIDs, ok := reconcileIDs(command, ids, requireIDs, stderr)
	if !ok {
		return profile.Profile{}, nil, "", false
	}
	selectedSession := strings.TrimSpace(sessionID)
	if selectedSession != "" {
		if err := transaction.ValidateSessionID(selectedSession); err != nil {
			fmt.Fprintf(stderr, "%s: --session is invalid: %s\n", command, safeDiagnosticLine(err.Error()))
			return profile.Profile{}, nil, "", false
		}
	}
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", command, safeDiagnosticLine(err.Error()))
		return profile.Profile{}, nil, "", false
	}
	return p, selectedIDs, selectedSession, true
}

func reconcileIDs(command string, ids multiFlag, requireIDs bool, stderr io.Writer) ([]string, bool) {
	if requireIDs && len(ids) == 0 {
		fmt.Fprintf(stderr, "%s: at least one --id is required\n", command)
		return nil, false
	}
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if err := control.ValidateArtifactID(id); err != nil {
			fmt.Fprintf(stderr, "%s: --id is invalid: %s\n", command, safeDiagnosticLine(err.Error()))
			return nil, false
		}
		if _, ok := seen[id]; ok {
			fmt.Fprintf(stderr, "%s: duplicate --id %q\n", command, id)
			return nil, false
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, true
}

func (r Runner) runPrune(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "help", "-h", "--help":
			printPruneUsage(stdout)
			return 0
		case "approvals":
			return r.runPruneApprovals(args[1:], stdout, stderr)
		case "approve":
			return r.runPruneApprove(args[1:], stdout, stderr)
		case "review":
			return r.runPruneReview(args[1:], stdout, stderr)
		case "supersede", "revoke":
			return r.runPruneSupersede(args[1:], stdout, stderr)
		}
	}
	fs := newFlagSet("prune", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		printPruneUsage(fs.Output())
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	dryRun := fs.Bool("dry-run", false, "emit non-mutating prune candidates and refusals from soft-delete evidence")
	apply := fs.Bool("apply", false, "apply a prune approval artifact and physically delete approved targets")
	approvalID := fs.String("approval", "", "approval id under target .supermover/prune/approvals")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "prune: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "prune: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "prune: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "prune: unsupported format %q\n", *format)
		return 2
	}
	if *dryRun && *apply {
		fmt.Fprintln(stderr, "prune: --dry-run and --apply are mutually exclusive")
		return 2
	}
	if !*apply && strings.TrimSpace(*approvalID) != "" {
		fmt.Fprintln(stderr, "prune: --approval is only valid with --apply")
		return 2
	}
	if *apply && strings.TrimSpace(*approvalID) == "" {
		fmt.Fprintln(stderr, "prune: --apply requires --approval <id>")
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "prune: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if p.DeletePolicy.Mode != profile.DeleteModePrune ||
		!p.DeletePolicy.RequireReview ||
		!p.DeletePolicy.AllowPhysicalPrune {
		fmt.Fprintln(stderr, "prune: profile delete_policy must use mode=prune with require_review=true and allow_physical_prune=true")
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "prune: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *dryRun || !*apply {
		report, err := prune.PlanDryRun(prune.Options{
			TargetRoot:   targetDir,
			ProfileID:    p.ProfileID,
			TargetID:     p.Target.TargetID,
			DeletePolicy: p.DeletePolicy,
			Now:          r.Now,
		})
		if err != nil {
			fmt.Fprintf(stderr, "prune: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
		switch *format {
		case "json":
			if err := json.NewEncoder(stdout).Encode(report); err != nil {
				fmt.Fprintf(stderr, "prune: encode dry-run report: %s\n", safeDiagnosticLine(err.Error()))
				return 1
			}
		case "text":
			printPruneDryRunText(stdout, report)
		}
		if report.NeedsReview() {
			fmt.Fprintln(stderr, "prune: dry-run produced review-required evidence; no target files were deleted")
			return 1
		}
		return 0
	}
	result, err := prune.Apply(prune.ApplyOptions{
		TargetRoot:   targetDir,
		ProfileID:    p.ProfileID,
		TargetID:     p.Target.TargetID,
		ApprovalID:   strings.TrimSpace(*approvalID),
		DeletePolicy: p.DeletePolicy,
		Now:          r.Now,
	})
	if err != nil {
		fmt.Fprintf(stderr, "prune: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "prune: encode apply result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	case "text":
		printPruneApplyText(stdout, result)
	}
	if result.Receipt.Status != control.PruneReceiptApplied {
		fmt.Fprintf(stderr, "prune: apply ended with status %s; inspect %s\n", result.Receipt.Status, result.ReceiptPath)
		return 1
	}
	return 0
}

func (r Runner) runPruneReview(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("prune review", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of prune review:
  supermover prune review --profile <path> [--session <id>] [--format text|json]

Builds current prune candidate/refusal evidence and reads approval inventory
and receipt evidence from the profile-selected target. This command is
read-only: it does not author approvals, apply prune decisions, write prune
receipts, or delete target files.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "optional session id to review")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "prune review: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "prune review: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "prune review: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "prune review: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "prune review: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "prune review: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	pruneReport, err := report.BuildPruneReview(report.Options{
		TargetRoot: targetDir,
		SessionID:  *sessionID,
		ProfileID:  p.ProfileID,
		TargetID:   p.Target.TargetID,
		Profile:    &p,
	})
	if err != nil {
		fmt.Fprintf(stderr, "prune review: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	result := pruneReleaseReviewFromReport(pruneReport)
	switch *format {
	case "text":
		printPruneReleaseReviewText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "prune review: encode review: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	if result.ReviewRequired {
		return 1
	}
	return 0
}

func (r Runner) runPruneApprovals(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("prune approvals", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of prune approvals:
  supermover prune approvals --profile <path> [--format text|json]

Lists current-scope prune approval artifacts for the profile-selected target.
This command is read-only: it does not supersede approvals, apply prune
decisions, write prune receipts, or delete target files.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "prune approvals: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "prune approvals: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "prune approvals: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "prune approvals: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "prune approvals: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "prune approvals: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	result, err := prune.ListApprovals(prune.ListApprovalsOptions{
		TargetRoot: targetDir,
		ProfileID:  p.ProfileID,
		TargetID:   p.Target.TargetID,
	})
	if err != nil {
		fmt.Fprintf(stderr, "prune approvals: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "text":
		printPruneApprovalsText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "prune approvals: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return 0
}

func (r Runner) runPruneApprove(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("prune approve", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of prune approve:
  supermover prune approve --profile <path> --id <approval-id> --soft-delete <id> [--soft-delete <id>...] --reason <text> [--reviewer <id>|--approved-by <id>] [--expires-at <rfc3339>] [--format text|json]

Writes a durable prune approval artifact from fresh dry-run evidence.
This command writes approval/profile-snapshot control-plane artifacts only; it
does not delete target files, write prune receipts, or apply approvals.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	approvalID := fs.String("id", "", "approval id under target .supermover/prune/approvals")
	softDeleteIDs := multiFlag{}
	fs.Var(&softDeleteIDs, "soft-delete", "soft-delete id to approve; repeatable")
	reason := fs.String("reason", "", "approval reason")
	reviewer := fs.String("reviewer", "", "reviewer/operator id")
	approvedBy := fs.String("approved-by", "", "reviewer/operator id alias")
	expiresAt := fs.String("expires-at", "", "optional approval expiry timestamp (RFC3339)")
	format := fs.String("format", "text", "output format: text or json")
	if firstArgIsHelp(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if hasHelpFlag(args) {
		fmt.Fprintln(stderr, "prune approve: --help is only valid as the sole argument")
		return 2
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stderr, "prune approve: --help is only valid as the sole argument")
			return 2
		}
		fmt.Fprintf(stderr, "prune approve: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "prune approve: --profile is required")
		return 2
	}
	if strings.TrimSpace(*approvalID) == "" {
		fmt.Fprintln(stderr, "prune approve: --id is required")
		return 2
	}
	if err := control.ValidateArtifactID(strings.TrimSpace(*approvalID)); err != nil {
		fmt.Fprintf(stderr, "prune approve: --id is invalid: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if len(softDeleteIDs) == 0 {
		fmt.Fprintln(stderr, "prune approve: at least one --soft-delete id is required")
		return 2
	}
	seenSoftDeletes := map[string]struct{}{}
	for _, id := range softDeleteIDs {
		id = strings.TrimSpace(id)
		if err := control.ValidateArtifactID(strings.TrimSpace(id)); err != nil {
			fmt.Fprintf(stderr, "prune approve: --soft-delete is invalid: %s\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		if _, ok := seenSoftDeletes[id]; ok {
			fmt.Fprintf(stderr, "prune approve: duplicate --soft-delete id %q\n", id)
			return 2
		}
		seenSoftDeletes[id] = struct{}{}
	}
	reviewerID := strings.TrimSpace(*reviewer)
	approvedByID := strings.TrimSpace(*approvedBy)
	if reviewerID != "" && approvedByID != "" && reviewerID != approvedByID {
		fmt.Fprintln(stderr, "prune approve: --reviewer and --approved-by must match when both are provided")
		return 2
	}
	if reviewerID == "" {
		reviewerID = approvedByID
	}
	if reviewerID == "" {
		fmt.Fprintln(stderr, "prune approve: --reviewer is required")
		return 2
	}
	if strings.TrimSpace(*reason) == "" {
		fmt.Fprintln(stderr, "prune approve: --reason is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "prune approve: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "prune approve: unsupported format %q\n", *format)
		return 2
	}
	normalizedNow := r.Now
	if normalizedNow.IsZero() {
		normalizedNow = time.Now().UTC()
	}
	normalizedNow = normalizedNow.UTC()
	if strings.TrimSpace(*expiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(*expiresAt))
		if err != nil {
			fmt.Fprintf(stderr, "prune approve: --expires-at must be RFC3339: %s\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		if !parsed.After(normalizedNow) {
			fmt.Fprintln(stderr, "prune approve: --expires-at must be after approved_at")
			return 2
		}
	}
	p, payload, err := readProfileFilePayload(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "prune approve: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if p.DeletePolicy.Mode != profile.DeleteModePrune ||
		!p.DeletePolicy.RequireReview ||
		!p.DeletePolicy.AllowPhysicalPrune {
		fmt.Fprintln(stderr, "prune approve: profile delete_policy must use mode=prune with require_review=true and allow_physical_prune=true")
		return 2
	}
	if _, err := targetDirFromProfile(p); err != nil {
		fmt.Fprintf(stderr, "prune approve: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	result, err := prune.AuthorApproval(prune.AuthorApprovalOptions{
		Profile:        p,
		ProfilePayload: payload,
		ApprovalID:     strings.TrimSpace(*approvalID),
		SoftDeleteIDs:  append([]string(nil), softDeleteIDs...),
		ApprovedBy:     reviewerID,
		Reason:         *reason,
		ExpiresAt:      *expiresAt,
		Now:            normalizedNow,
	})
	if err != nil {
		fmt.Fprintf(stderr, "prune approve: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "prune approve: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	case "text":
		printPruneApproveText(stdout, result)
	}
	return 0
}

func (r Runner) runPruneSupersede(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("prune supersede", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of prune supersede:
  supermover prune supersede --profile <path> --id <approval-id> --reason <text> --reviewer <id> [--format text|json]

Marks one existing current-scope prune approval artifact superseded.
This updates durable approval review metadata only; it does not apply prune
decisions, write prune receipts, or delete target files.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	id := fs.String("id", "", "approval id under target .supermover/prune/approvals")
	reason := fs.String("reason", "", "supersede reason")
	reviewer := fs.String("reviewer", "", "reviewer/operator id")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "prune supersede: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "prune supersede: --profile is required")
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "prune supersede: --id is required")
		return 2
	}
	if err := control.ValidateArtifactID(strings.TrimSpace(*id)); err != nil {
		fmt.Fprintf(stderr, "prune supersede: --id is invalid: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*reason) == "" {
		fmt.Fprintln(stderr, "prune supersede: --reason is required")
		return 2
	}
	if strings.TrimSpace(*reviewer) == "" {
		fmt.Fprintln(stderr, "prune supersede: --reviewer is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "prune supersede: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "prune supersede: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "prune supersede: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "prune supersede: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	result, err := prune.SupersedeApproval(prune.SupersedeApprovalOptions{
		TargetRoot: targetDir,
		ProfileID:  p.ProfileID,
		TargetID:   p.Target.TargetID,
		ApprovalID: strings.TrimSpace(*id),
		Reason:     *reason,
		Reviewer:   *reviewer,
		ReviewTool: "supermover prune supersede",
		Now:        r.Now,
	})
	if err != nil {
		fmt.Fprintf(stderr, "prune supersede: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "text":
		printPruneSupersedeText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "prune supersede: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return 0
}

func (r Runner) runHealth(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("health", stderr)
	profilePath := fs.String("profile", "", "profile path")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "health: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "health: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "health: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "health: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	report, err := health.BuildReport(health.Options{
		TargetRoot: targetDir,
		ProfileID:  p.ProfileID,
		TargetID:   p.Target.TargetID,
	})
	if err != nil {
		fmt.Fprintf(stderr, "health: %s\n", safeDiagnosticLine(err.Error()))
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

func (r Runner) runReport(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("report", stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of report:
  supermover report --profile <path> [--session <id>] [--format text|json]

Summarizes read-only evidence from the profile-selected target.
It may surface persisted network-transfer artifacts and live drift evidence, but it does not start daemon or transport work, repair state, or persist live detector output.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "optional session id to report")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "report: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "report: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "report: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "report: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "report: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "report: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	auditReport, err := report.BuildReport(report.Options{
		TargetRoot: targetDir,
		SessionID:  *sessionID,
		ProfileID:  p.ProfileID,
		TargetID:   p.Target.TargetID,
		Profile:    &p,
	})
	if err != nil {
		fmt.Fprintf(stderr, "report: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "text":
		printReportText(stdout, auditReport)
	case "json":
		if err := json.NewEncoder(stdout).Encode(auditReport); err != nil {
			fmt.Fprintf(stderr, "report: encode report: %v\n", err)
			return 1
		}
	}
	if auditReport.NeedsReview() {
		return 1
	}
	return 0
}

func (r Runner) runStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("status", stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of status:
  supermover status --profile <path> [--format text|json]

Reads the persisted profile SSOT, target control-plane artifacts, and local
target files needed for verification and live drift detection. Live detector
output is not persisted. Network fields report local artifact evidence only;
this command does not start discovery, LAN transfer, encrypted transfer, or
synchronization.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "status: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "status: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "status: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "status: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "status: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "status: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	statusReport, err := status.Build(status.Options{
		TargetRoot: targetDir,
		Profile:    &p,
	})
	if err != nil {
		fmt.Fprintf(stderr, "status: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	switch *format {
	case "text":
		printStatusText(stdout, statusReport)
	case "json":
		if err := json.NewEncoder(stdout).Encode(statusReport); err != nil {
			fmt.Fprintf(stderr, "status: encode report: %v\n", err)
			return 2
		}
	}
	return statusReport.ExitCode()
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "-help" || arg == "--help" {
			return true
		}
	}
	return false
}

func firstArgIsHelp(args []string) bool {
	return len(args) == 1 && (args[0] == "-h" || args[0] == "-help" || args[0] == "--help")
}

func (r Runner) runServe(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("serve", stderr)
	profilePath := fs.String("profile", "", "--profile target profile path; unpaired profiles serve pairing only, paired receiver material must be complete")
	listen := fs.String("listen", pairserve.DefaultListen, "--listen pairing listen address; receiver address comes from profile network.receiver_url")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "serve: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "serve: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if strings.TrimSpace(*listen) == "" {
		fmt.Fprintln(stderr, "serve: --listen is required")
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 2
	}
	if _, err := targetDirFromProfile(p); err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 2
	}
	enableReceiver := false
	if serveReceiverMaterialPresent(p) && profileHasPairingPins(p) {
		if _, err := pairing.ValidateProfileTrust(p); err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		if err := p.ValidateNetworkServerMaterial(); err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		enableReceiver = true
	}
	ctx := r.Context
	if ctx == nil {
		ctx = context.Background()
	}
	serveCtx, stopServe := context.WithCancel(ctx)
	defer stopServe()
	var outputMu sync.Mutex
	pairingServer, err := r.newPairingServe(p, *listen, stderr, &outputMu, enableReceiver)
	if err != nil {
		if errors.Is(err, pairserve.ErrInvalidOptions) {
			fmt.Fprintf(stderr, "serve: %v\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		fmt.Fprintf(stderr, "serve: %v\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	pairingListener, err := pairingServer.Listen()
	if err != nil {
		fmt.Fprintf(stderr, "serve: pairing: %v\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	defer pairingListener.Close()
	var receiverServer *receiverserve.Server
	var receiverListener net.Listener
	if enableReceiver {
		receiver, err := r.newReceiverServe(p, stderr, &outputMu)
		if err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		listener := r.receiverListenerForTest
		if listener == nil {
			var err error
			openedListener, err := receiver.Listen()
			if err != nil {
				fmt.Fprintf(stderr, "serve: receiver: %v\n", safeDiagnosticLine(err.Error()))
				return 1
			}
			listener = openedListener
		}
		defer listener.Close()
		receiverServer = receiver
		receiverListener = listener
	}
	serverCount := 1
	if enableReceiver {
		serverCount = 2
	}
	errCh := make(chan serveResult, serverCount)
	if receiverServer != nil {
		go func() {
			errCh <- serveResult{name: "receiver", err: receiverServer.Serve(serveCtx, receiverListener)}
		}()
	}
	go func() {
		errCh <- serveResult{name: "pairing", err: pairingServer.Serve(serveCtx, pairingListener)}
	}()
	var firstErr serveResult
	for completed := 0; completed < serverCount; completed++ {
		result := <-errCh
		if result.err != nil && firstErr.err == nil {
			firstErr = result
			stopServe()
		}
	}
	if firstErr.err != nil {
		if firstErr.name == "pairing" && errors.Is(firstErr.err, pairserve.ErrInvalidOptions) {
			fmt.Fprintf(stderr, "serve: %v\n", firstErr.err)
			return 2
		}
		fmt.Fprintf(stderr, "serve: %s: %v\n", firstErr.name, firstErr.err)
		return 1
	}
	return 0
}

func (r Runner) runDaemon(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "daemon: missing subcommand")
		printDaemonUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printDaemonUsage(stdout)
		return 0
	case "install":
		return r.runDaemonInstall(args[1:], stdout, stderr)
	case "run":
		return r.runDaemonRun(args[1:], stdout, stderr)
	case "status":
		return r.runDaemonStatus(args[1:], stdout, stderr)
	case "logs":
		return r.runDaemonLogs(args[1:], stdout, stderr)
	case "restart":
		return r.runDaemonRestart(args[1:], stdout, stderr)
	case "stop":
		return r.runDaemonStop(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "daemon: unknown subcommand %q\n", args[0])
		printDaemonUsage(stderr)
		return 2
	}
}

func (r Runner) runSync(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "sync: missing subcommand")
		printSyncUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printSyncUsage(stdout)
		return 0
	case "queue":
		return r.runSyncQueue(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "sync: unknown subcommand %q\n", args[0])
		printSyncUsage(stderr)
		return 2
	}
}

func (r Runner) runSyncQueue(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "sync queue: missing subcommand")
		printSyncQueueUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printSyncQueueUsage(stdout)
		return 0
	case "enqueue":
		return r.runSyncQueueEnqueue(args[1:], stdout, stderr)
	case "status":
		return r.runSyncQueueStatus(args[1:], stdout, stderr)
	case "ready":
		return r.runSyncQueueReady(args[1:], stdout, stderr)
	case "cancel":
		return r.runSyncQueueCancel(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "sync queue: unknown subcommand %q\n", args[0])
		printSyncQueueUsage(stderr)
		return 2
	}
}

func (r Runner) runSyncQueueEnqueue(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync queue enqueue", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync queue enqueue:
  supermover sync queue enqueue --profile <path> [--format text|json]

Snapshots profile roots and records durable changed-file queue evidence under the
profile-selected target. This queues evidence only; it does not watch roots,
copy files, run a daemon, or perform ongoing sync.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "sync queue enqueue: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync queue enqueue: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, scheduler, _, ok := r.syncQueueScheduler("sync queue enqueue", *profilePath, *format, true, stderr)
	if !ok {
		return 2
	}
	snapshots, err := incrementalsync.SnapshotProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "sync queue enqueue: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	result := syncQueueEnqueueResult{
		Operation: "enqueue",
		Mode:      "queue_only",
		Summary:   syncQueueEmptySummary(p, "", r.nowFunc()()),
	}
	for _, snapshot := range snapshots {
		enqueued, err := scheduler.Enqueue(snapshot)
		if err != nil {
			fmt.Fprintf(stderr, "sync queue enqueue: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
		result.Scope = enqueued.Scope
		result.StatePath = enqueued.StatePath
		result.Enqueued = append(result.Enqueued, enqueued.Enqueued...)
		result.Skipped = append(result.Skipped, enqueued.Skipped...)
		result.Audit = append(result.Audit, enqueued.Audit...)
		result.Summary = enqueued.Summary
	}
	if len(snapshots) == 0 {
		statePath, err := scheduler.StatePath(incrementalQueueScope(p))
		if err != nil {
			fmt.Fprintf(stderr, "sync queue enqueue: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
		result.Scope = incrementalQueueScope(p)
		result.StatePath = statePath
		result.Summary.StatePath = statePath
	}
	return printSyncQueueEnqueueResult(stdout, stderr, *format, result)
}

func (r Runner) runSyncQueueStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync queue status", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync queue status:
  supermover sync queue status --profile <path> [--format text|json]

Reads durable changed-file queue evidence from the profile-selected target.
This is a status view only; it does not watch roots, copy files, run a daemon,
or perform ongoing sync.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "sync queue status: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync queue status: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, scheduler, statePath, ok := r.syncQueueScheduler("sync queue status", *profilePath, *format, false, stderr)
	if !ok {
		return 2
	}
	summary, state, err := syncQueueSummaryOrMissing(scheduler, p, statePath, r.nowFunc()())
	if err != nil {
		fmt.Fprintf(stderr, "sync queue status: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	result := syncQueueSummaryResult{Operation: "status", Mode: "queue_only", State: state, Summary: summary}
	return printSyncQueueSummaryResult(stdout, stderr, *format, result)
}

func (r Runner) runSyncQueueReady(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync queue ready", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync queue ready:
  supermover sync queue ready --profile <path> [--format text|json]

Lists currently ready queue entries from durable changed-file evidence. This
does not execute the entries, copy files, watch roots, run a daemon, or perform
ongoing sync.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "sync queue ready: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync queue ready: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, scheduler, statePath, ok := r.syncQueueScheduler("sync queue ready", *profilePath, *format, false, stderr)
	if !ok {
		return 2
	}
	summary, state, err := syncQueueSummaryOrMissing(scheduler, p, statePath, r.nowFunc()())
	if err != nil {
		fmt.Fprintf(stderr, "sync queue ready: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	var entries []incrementalsync.QueueEntry
	if state == "present" {
		ready, err := scheduler.Ready(incrementalQueueScope(p))
		if err != nil {
			fmt.Fprintf(stderr, "sync queue ready: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
		entries = ready
	}
	result := syncQueueEntriesResult{Operation: "ready", Mode: "queue_only", State: state, Summary: summary, Entries: entries}
	return printSyncQueueEntriesResult(stdout, stderr, *format, result)
}

func (r Runner) runSyncQueueCancel(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync queue cancel", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync queue cancel:
  supermover sync queue cancel --profile <path> --id <entry-id> --reason <text> [--format text|json]

Marks one durable queue entry canceled. This records operator queue state only;
it does not delete source data, mutate target files, copy files, run a daemon,
or perform ongoing sync.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	entryID := fs.String("id", "", "queue entry id")
	reason := fs.String("reason", "", "operator cancellation reason")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "sync queue cancel: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*entryID) == "" {
		fmt.Fprintln(stderr, "sync queue cancel: --id is required")
		return 2
	}
	if strings.TrimSpace(*reason) == "" {
		fmt.Fprintln(stderr, "sync queue cancel: --reason is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync queue cancel: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, scheduler, _, ok := r.syncQueueScheduler("sync queue cancel", *profilePath, *format, true, stderr)
	if !ok {
		return 2
	}
	entry, err := scheduler.Cancel(incrementalQueueScope(p), strings.TrimSpace(*entryID), *reason)
	if err != nil {
		fmt.Fprintf(stderr, "sync queue cancel: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	summary, state, err := syncQueueSummaryOrMissing(scheduler, p, "", r.nowFunc()())
	if err != nil {
		fmt.Fprintf(stderr, "sync queue cancel: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	result := syncQueueCancelResult{Operation: "cancel", Mode: "queue_only", State: state, Entry: entry, Summary: summary, Reason: strings.TrimSpace(*reason)}
	return printSyncQueueCancelResult(stdout, stderr, *format, result)
}

func (r Runner) syncQueueScheduler(command string, profilePath string, format string, create bool, stderr io.Writer) (profile.Profile, *incrementalsync.Scheduler, string, bool) {
	if strings.TrimSpace(profilePath) == "" {
		fmt.Fprintf(stderr, "%s: --profile is required\n", command)
		return profile.Profile{}, nil, "", false
	}
	if format != "text" && format != "json" {
		fmt.Fprintf(stderr, "%s: unsupported format %q\n", command, format)
		return profile.Profile{}, nil, "", false
	}
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", command, safeDiagnosticLine(err.Error()))
		return profile.Profile{}, nil, "", false
	}
	stateDir, err := incrementalQueueStateDir(p, create)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", command, safeDiagnosticLine(err.Error()))
		return profile.Profile{}, nil, "", false
	}
	var scheduler *incrementalsync.Scheduler
	if create {
		scheduler, err = incrementalsync.New(incrementalsync.Options{StateDir: stateDir, Now: r.nowFunc()})
	} else {
		scheduler, err = incrementalsync.Open(incrementalsync.Options{StateDir: stateDir, Now: r.nowFunc()})
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", command, safeDiagnosticLine(err.Error()))
		return profile.Profile{}, nil, "", false
	}
	statePath, err := scheduler.StatePath(incrementalQueueScope(p))
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", command, safeDiagnosticLine(err.Error()))
		return profile.Profile{}, nil, "", false
	}
	return p, scheduler, statePath, true
}

func incrementalQueueScope(p profile.Profile) incrementalsync.Scope {
	return incrementalsync.Scope{ProfileID: p.ProfileID, TargetID: p.Target.TargetID}
}

func incrementalQueueStateDir(p profile.Profile, create bool) (string, error) {
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		return "", err
	}
	if err := control.ValidateArtifactLoadBoundary(targetDir); err != nil {
		return "", err
	}
	if create {
		if err := control.EnsureControlDir(targetDir); err != nil {
			return "", err
		}
	}
	return filepath.Join(control.ControlDir(targetDir), "incremental-sync"), nil
}

func (r Runner) runDaemonInstall(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("daemon install", stderr)
	profilePath := fs.String("profile", "", "--profile target profile path; profile remains the daemon SSOT")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "daemon install: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, targetDir, cleanProfilePath, err := readDaemonProfile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "daemon install: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	now := r.nowFunc()()
	install := agentdaemon.NewInstall(p.ProfileID, p.Target.TargetID, cleanProfilePath, now)
	if err := withLockedTarget(targetDir, func() error {
		return agentdaemon.WriteInstall(targetDir, install)
	}); err != nil {
		fmt.Fprintf(stderr, "daemon install: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if _, err := agentdaemon.AppendLifecycleEvent(targetDir, agentdaemon.NewLifecycleEvent(p.ProfileID, p.Target.TargetID, "daemon_installed", "foreground daemon lifecycle installed", map[string]string{
		"run_mode":        agentdaemon.RunModeForeground,
		"service_manager": agentdaemon.ServiceManagerNone,
	}, now)); err != nil {
		fmt.Fprintf(stderr, "daemon install: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	fmt.Fprintf(stdout, "daemon: installed profile=%s target=%s run_mode=foreground service_manager=none command=%s\n",
		encodeTextValue(p.ProfileID),
		encodeTextValue(p.Target.TargetID),
		encodeTextValue(strings.Join(install.Command, " ")),
	)
	return 0
}

func (r Runner) runDaemonRun(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("daemon run", stderr)
	profilePath := fs.String("profile", "", "--profile target profile path; profile remains the daemon SSOT")
	foreground := fs.Bool("foreground", false, "--foreground run supervised in this process; no OS service manager is installed")
	listen := fs.String("listen", pairserve.DefaultListen, "--listen pairing listen address; receiver address comes from profile network.receiver_url")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "daemon run: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if !*foreground {
		fmt.Fprintln(stderr, "daemon run: --foreground is required; background process management is not wired")
		return 2
	}
	if strings.TrimSpace(*listen) == "" {
		fmt.Fprintln(stderr, "daemon run: --listen is required")
		return 2
	}
	p, targetDir, cleanProfilePath, err := readDaemonProfile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	ctx := r.Context
	if ctx == nil {
		ctx = context.Background()
	}
	initialProfileID := p.ProfileID
	initialTargetID := p.Target.TargetID
	initialTargetDir := targetDir
	outputMu := &sync.Mutex{}
	cleanupOldStopIntent := true
	for {
		enableReceiver, err := validateDaemonReceiverMode(p)
		if err != nil {
			writeDaemonFailedState(targetDir, agentdaemon.NewState(initialProfileID, initialTargetID, cleanProfilePath, agentdaemon.StatusFailed, os.Getpid(), r.nowFunc()()), err.Error(), r.nowFunc()())
			appendDaemonLifecycleEvent(targetDir, initialProfileID, initialTargetID, "daemon_failed", "profile validation failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
			fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		result := r.runDaemonForegroundCycle(ctx, p, targetDir, cleanProfilePath, *listen, enableReceiver, cleanupOldStopIntent, stdout, stderr, outputMu)
		if result.exitCode != 0 || !result.restart {
			return result.exitCode
		}
		cleanupOldStopIntent = false
		nextProfile, nextTargetDir, nextCleanProfilePath, err := readDaemonProfile(cleanProfilePath)
		if err != nil {
			writeDaemonFailedState(targetDir, result.state, err.Error(), r.nowFunc()())
			appendDaemonLifecycleEvent(targetDir, result.state.ProfileID, result.state.TargetID, "daemon_failed", "profile reload failed after restart request", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
			fmt.Fprintf(stderr, "daemon run: reload profile after restart: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
		if nextProfile.ProfileID != initialProfileID || nextProfile.Target.TargetID != initialTargetID || filepath.Clean(nextTargetDir) != filepath.Clean(initialTargetDir) {
			err := fmt.Errorf("profile reload changed daemon scope; profile_id, target_id, and target.local_path must remain stable during foreground restart")
			writeDaemonFailedState(targetDir, result.state, err.Error(), r.nowFunc()())
			appendDaemonLifecycleEvent(targetDir, result.state.ProfileID, result.state.TargetID, "daemon_failed", "profile reload changed daemon scope", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
			fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
		p = nextProfile
		targetDir = nextTargetDir
		cleanProfilePath = nextCleanProfilePath
	}
}

type daemonRunCycleResult struct {
	state    agentdaemon.State
	exitCode int
	restart  bool
}

func (r Runner) runDaemonForegroundCycle(ctx context.Context, p profile.Profile, targetDir, cleanProfilePath, listen string, enableReceiver bool, cleanupOldStopIntent bool, stdout io.Writer, stderr io.Writer, outputMu *sync.Mutex) daemonRunCycleResult {
	serveCtx, stopServe := context.WithCancel(ctx)
	defer stopServe()
	now := r.nowFunc()()
	state := agentdaemon.NewState(p.ProfileID, p.Target.TargetID, cleanProfilePath, agentdaemon.StatusStarting, os.Getpid(), now)
	state.Mode = daemonMode(enableReceiver)
	appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_starting", "foreground daemon starting serve listeners", map[string]string{
		"mode":            state.Mode,
		"run_mode":        agentdaemon.RunModeForeground,
		"service_manager": agentdaemon.ServiceManagerNone,
	}, now)
	if err := writeLockedDaemonState(targetDir, state); err != nil {
		fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
		return daemonRunCycleResult{state: state, exitCode: 1}
	}
	if cleanupOldStopIntent {
		if err := removeScopedDaemonStopIntentBefore(targetDir, state.ProfileID, state.TargetID, now); err != nil {
			fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: state, exitCode: 1}
		}
	}
	pairingReady := make(chan pairserve.ReadyInfo, 1)
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	serveRunner := r
	serveRunner.ServePairingReady = func(info pairserve.ReadyInfo) {
		pairingReady <- info
		if r.ServePairingReady != nil {
			r.ServePairingReady(info)
		}
	}
	serveRunner.ServeReceiverReady = func(info receiverserve.ReadyInfo) {
		receiverReady <- info
		if r.ServeReceiverReady != nil {
			r.ServeReceiverReady(info)
		}
	}
	pairingServer, err := serveRunner.newPairingServe(p, listen, stderr, outputMu, enableReceiver)
	if err != nil {
		writeDaemonFailedState(targetDir, state, err.Error(), r.nowFunc()())
		appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_failed", "pairing serve setup failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
		if errors.Is(err, pairserve.ErrInvalidOptions) {
			fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: state, exitCode: 2}
		}
		fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
		return daemonRunCycleResult{state: state, exitCode: 1}
	}
	pairingListener, err := pairingServer.Listen()
	if err != nil {
		writeDaemonFailedState(targetDir, state, err.Error(), r.nowFunc()())
		appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_failed", "pairing listener failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
		fmt.Fprintf(stderr, "daemon run: pairing: %s\n", safeDiagnosticLine(err.Error()))
		return daemonRunCycleResult{state: state, exitCode: 1}
	}
	defer pairingListener.Close()
	var receiverServer *receiverserve.Server
	var receiverListener net.Listener
	if enableReceiver {
		receiver, err := serveRunner.newReceiverServe(p, stderr, outputMu)
		if err != nil {
			writeDaemonFailedState(targetDir, state, err.Error(), r.nowFunc()())
			appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_failed", "receiver serve setup failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
			fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: state, exitCode: 2}
		}
		listener := r.receiverListenerForTest
		if listener == nil {
			openedListener, err := receiver.Listen()
			if err != nil {
				writeDaemonFailedState(targetDir, state, err.Error(), r.nowFunc()())
				appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_failed", "receiver listener failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
				fmt.Fprintf(stderr, "daemon run: receiver: %s\n", safeDiagnosticLine(err.Error()))
				return daemonRunCycleResult{state: state, exitCode: 1}
			}
			listener = openedListener
		}
		defer listener.Close()
		receiverServer = receiver
		receiverListener = listener
	}
	serverCount := 1
	if enableReceiver {
		serverCount = 2
	}
	errCh := make(chan serveResult, serverCount)
	if receiverServer != nil {
		go func() {
			errCh <- serveResult{name: "receiver", err: receiverServer.Serve(serveCtx, receiverListener)}
		}()
	}
	go func() {
		errCh <- serveResult{name: "pairing", err: pairingServer.Serve(serveCtx, pairingListener)}
	}()
	state, readyErr := waitDaemonReady(targetDir, state, pairingReady, receiverReady, enableReceiver, errCh, r.nowFunc()())
	if readyErr.err != nil {
		stopServe()
		waitServeResults(serverCount-readyErr.consumedResults, errCh, serveResult{})
		appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_failed", "serve listener stopped before readiness", map[string]string{"error_class": daemonErrorClass(readyErr.err)}, r.nowFunc()())
		if readyErr.name != "" {
			fmt.Fprintf(stderr, "daemon run: %s: %s\n", readyErr.name, safeDiagnosticLine(readyErr.err.Error()))
		} else {
			fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(readyErr.err.Error()))
		}
		return daemonRunCycleResult{state: state, exitCode: readyErr.exitCode}
	}
	appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_running", "foreground daemon serve listeners are ready", map[string]string{
		"mode":             state.Mode,
		"pairing_address":  statusTextValueOrDash(state.PairingAddress),
		"receiver_address": statusTextValueOrDash(state.ReceiverAddress),
	}, r.nowFunc()())
	fmt.Fprintf(stdout, "daemon: running profile=%s target=%s state=running mode=%s pairing_address=%s receiver_address=%s foreground=true service_manager=none\n",
		encodeTextValue(state.ProfileID),
		encodeTextValue(state.TargetID),
		encodeTextValue(state.Mode),
		encodeTextValue(statusTextValueOrDash(state.PairingAddress)),
		encodeTextValue(statusTextValueOrDash(state.ReceiverAddress)),
	)
	if r.DaemonReady != nil {
		r.DaemonReady(state)
	}
	stopPoll := make(chan struct{})
	stopIntentCh := make(chan agentdaemon.StopIntent, 1)
	restartPoll := make(chan struct{})
	restartIntentCh := make(chan agentdaemon.RestartIntent, 1)
	go pollDaemonStopIntent(serveCtx, targetDir, state.ProfileID, state.TargetID, stopPoll, stopIntentCh, stopServe)
	go pollDaemonRestartIntent(serveCtx, targetDir, state.ProfileID, state.TargetID, restartPoll, restartIntentCh, stopServe)
	firstErr := waitServeResultsWithCancel(serverCount, errCh, stopServe)
	close(stopPoll)
	close(restartPoll)
	now = r.nowFunc()()
	finalState := state
	finalState.PID = os.Getpid()
	finalState.UpdatedAt = now.Format(time.RFC3339Nano)
	if firstErr.err != nil {
		finalState.Status = agentdaemon.StatusFailed
		finalState.LastError = firstErr.err.Error()
		if writeErr := writeLockedDaemonState(targetDir, finalState); writeErr != nil {
			fmt.Fprintf(stderr, "daemon run: write failed state: %s\n", safeDiagnosticLine(writeErr.Error()))
			return daemonRunCycleResult{state: finalState, exitCode: 1}
		}
		appendDaemonLifecycleEvent(targetDir, finalState.ProfileID, finalState.TargetID, "daemon_failed", "serve listener failed", map[string]string{"component": firstErr.name, "error_class": daemonErrorClass(firstErr.err)}, now)
		fmt.Fprintf(stderr, "daemon run: %s: %s\n", firstErr.name, safeDiagnosticLine(firstErr.err.Error()))
		return daemonRunCycleResult{state: finalState, exitCode: 1}
	}
	if intent, ok := receiveDaemonStopIntent(stopIntentCh); ok {
		finalState.Status = agentdaemon.StatusStopped
		finalState.StoppedAt = now.Format(time.RFC3339Nano)
		finalState.StopIntent = ptr(agentdaemon.StopSummary(intent))
		if err := writeLockedDaemonState(targetDir, finalState); err != nil {
			fmt.Fprintf(stderr, "daemon run: write stopped state: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: finalState, exitCode: 1}
		}
		appendDaemonLifecycleEvent(targetDir, finalState.ProfileID, finalState.TargetID, "daemon_stopped", "foreground daemon stopped from stop intent", map[string]string{"reason": intent.Reason}, now)
		return daemonRunCycleResult{state: finalState, exitCode: 0}
	}
	if intent, ok := receiveDaemonRestartIntent(restartIntentCh); ok {
		finalState.Status = agentdaemon.StatusStarting
		finalState.PairingAddress = ""
		finalState.ReceiverAddress = ""
		if err := withLockedTarget(targetDir, func() error {
			if err := agentdaemon.WriteState(targetDir, finalState); err != nil {
				return err
			}
			return agentdaemon.RemoveRestartIntent(targetDir)
		}); err != nil {
			fmt.Fprintf(stderr, "daemon run: write restarting state: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: finalState, exitCode: 1}
		}
		appendDaemonLifecycleEvent(targetDir, finalState.ProfileID, finalState.TargetID, "daemon_restart_consumed", "foreground daemon consumed restart intent", map[string]string{"reason": intent.Reason}, now)
		if r.DaemonRestartConsumed != nil {
			r.DaemonRestartConsumed(finalState)
		}
		return daemonRunCycleResult{state: finalState, exitCode: 0, restart: true}
	}
	finalState.Status = agentdaemon.StatusStopped
	finalState.StoppedAt = now.Format(time.RFC3339Nano)
	if err := writeLockedDaemonState(targetDir, finalState); err != nil {
		fmt.Fprintf(stderr, "daemon run: write stopped state: %s\n", safeDiagnosticLine(err.Error()))
		return daemonRunCycleResult{state: finalState, exitCode: 1}
	}
	appendDaemonLifecycleEvent(targetDir, finalState.ProfileID, finalState.TargetID, "daemon_stopped", "foreground daemon stopped", nil, now)
	return daemonRunCycleResult{state: finalState, exitCode: 0}
}

type daemonReadyError struct {
	name            string
	err             error
	exitCode        int
	consumedResults int
}

func waitDaemonReady(targetDir string, state agentdaemon.State, pairingReady <-chan pairserve.ReadyInfo, receiverReady <-chan receiverserve.ReadyInfo, enableReceiver bool, errCh <-chan serveResult, now time.Time) (agentdaemon.State, daemonReadyError) {
	state.Status = agentdaemon.StatusRunning
	pairingSeen := false
	receiverSeen := !enableReceiver
	for !(pairingSeen && receiverSeen) {
		select {
		case info := <-pairingReady:
			pairingSeen = true
			state.PairingAddress = info.Address
		case info := <-receiverReady:
			receiverSeen = true
			state.ReceiverAddress = info.Address
		case result := <-errCh:
			if result.err == nil {
				result.err = errors.New("daemon server stopped before readiness")
			}
			state.Status = agentdaemon.StatusFailed
			state.LastError = result.err.Error()
			state.UpdatedAt = now.Format(time.RFC3339Nano)
			_ = writeLockedDaemonState(targetDir, state)
			code := 1
			if result.name == "pairing" && errors.Is(result.err, pairserve.ErrInvalidOptions) {
				code = 2
			}
			return state, daemonReadyError{name: result.name, err: result.err, exitCode: code, consumedResults: 1}
		}
	}
	state.UpdatedAt = now.Format(time.RFC3339Nano)
	if err := writeLockedDaemonState(targetDir, state); err != nil {
		return state, daemonReadyError{err: err, exitCode: 1}
	}
	return state, daemonReadyError{}
}

func waitServeResults(count int, errCh <-chan serveResult, firstErr serveResult) serveResult {
	for completed := 0; completed < count; completed++ {
		result := <-errCh
		if result.err != nil && firstErr.err == nil {
			firstErr = result
		}
	}
	return firstErr
}

func waitServeResultsWithCancel(count int, errCh <-chan serveResult, stop func()) serveResult {
	var firstErr serveResult
	for completed := 0; completed < count; completed++ {
		result := <-errCh
		if result.err != nil && firstErr.err == nil {
			firstErr = result
			stop()
		}
	}
	return firstErr
}

func pollDaemonStopIntent(ctx context.Context, targetDir, profileID, targetID string, done <-chan struct{}, stopIntentCh chan<- agentdaemon.StopIntent, stop func()) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			if intent, ok, err := readScopedDaemonStopIntent(targetDir, profileID, targetID); err == nil && ok {
				select {
				case stopIntentCh <- intent:
				default:
				}
				stop()
				return
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
	}
}

func pollDaemonRestartIntent(ctx context.Context, targetDir, profileID, targetID string, done <-chan struct{}, restartIntentCh chan<- agentdaemon.RestartIntent, stop func()) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			if intent, ok, err := readScopedDaemonRestartIntent(targetDir, profileID, targetID); err == nil && ok {
				select {
				case restartIntentCh <- intent:
				default:
				}
				stop()
				return
			} else if err != nil && !errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
	}
}

func receiveDaemonStopIntent(stopIntentCh <-chan agentdaemon.StopIntent) (agentdaemon.StopIntent, bool) {
	select {
	case intent := <-stopIntentCh:
		return intent, true
	default:
		return agentdaemon.StopIntent{}, false
	}
}

func receiveDaemonRestartIntent(restartIntentCh <-chan agentdaemon.RestartIntent) (agentdaemon.RestartIntent, bool) {
	select {
	case intent := <-restartIntentCh:
		return intent, true
	default:
		return agentdaemon.RestartIntent{}, false
	}
}

func removeScopedDaemonStopIntentBefore(targetDir, profileID, targetID string, cutoff time.Time) error {
	intent, ok, err := readScopedDaemonStopIntent(targetDir, profileID, targetID)
	if errors.Is(err, os.ErrNotExist) || !ok {
		return nil
	}
	if err != nil {
		return err
	}
	requestedAt, err := time.Parse(time.RFC3339Nano, intent.RequestedAt)
	if err != nil {
		return err
	}
	if !requestedAt.Before(cutoff.UTC()) {
		return nil
	}
	return agentdaemon.RemoveStopIntent(targetDir)
}

func readScopedDaemonStopIntent(targetDir, profileID, targetID string) (agentdaemon.StopIntent, bool, error) {
	intent, err := agentdaemon.ReadStopIntent(targetDir)
	if err != nil {
		return agentdaemon.StopIntent{}, false, err
	}
	if !agentdaemon.ArtifactInScope(intent.ProfileID, intent.TargetID, profileID, targetID) {
		return intent, false, nil
	}
	return intent, true, nil
}

func readScopedDaemonRestartIntent(targetDir, profileID, targetID string) (agentdaemon.RestartIntent, bool, error) {
	intent, err := agentdaemon.ReadRestartIntent(targetDir)
	if err != nil {
		return agentdaemon.RestartIntent{}, false, err
	}
	if !agentdaemon.ArtifactInScope(intent.ProfileID, intent.TargetID, profileID, targetID) {
		return intent, false, nil
	}
	return intent, true, nil
}

func writeDaemonFailedState(targetDir string, state agentdaemon.State, message string, now time.Time) {
	state.Status = agentdaemon.StatusFailed
	state.LastError = message
	state.UpdatedAt = now.Format(time.RFC3339Nano)
	_ = writeLockedDaemonState(targetDir, state)
}

func appendDaemonLifecycleEvent(targetDir, profileID, targetID, eventType, message string, details map[string]string, now time.Time) {
	_, _ = agentdaemon.AppendLifecycleEvent(targetDir, agentdaemon.NewLifecycleEvent(profileID, targetID, eventType, message, details, now))
}

func daemonErrorClass(err error) string {
	if err == nil {
		return "none"
	}
	switch {
	case errors.Is(err, pairserve.ErrInvalidOptions):
		return "invalid_pairing_options"
	case errors.Is(err, os.ErrNotExist):
		return "not_found"
	case errors.Is(err, os.ErrPermission):
		return "permission"
	default:
		return "daemon_error"
	}
}

func writeLockedDaemonState(targetDir string, state agentdaemon.State) error {
	return withLockedTarget(targetDir, func() error {
		return agentdaemon.WriteState(targetDir, state)
	})
}

func withLockedTarget(targetDir string, fn func() error) error {
	unlock, err := targetlock.LockTarget(targetDir)
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

func readDaemonProfile(profilePath string) (profile.Profile, string, string, error) {
	if strings.TrimSpace(profilePath) == "" {
		return profile.Profile{}, "", "", errors.New("--profile is required")
	}
	cleanProfilePath, err := filepath.Abs(filepath.Clean(profilePath))
	if err != nil {
		return profile.Profile{}, "", "", err
	}
	p, err := profile.ReadFile(cleanProfilePath)
	if err != nil {
		return profile.Profile{}, "", "", err
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		return profile.Profile{}, "", "", err
	}
	if daemonTargetContainsReservedControl(targetDir) {
		return profile.Profile{}, "", "", fmt.Errorf("target.local_path must not be the reserved %s control directory", control.DirName)
	}
	info, err := os.Lstat(targetDir)
	if err != nil {
		return profile.Profile{}, "", "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return profile.Profile{}, "", "", fmt.Errorf("target.local_path %q is a symlink", targetDir)
	}
	if !info.IsDir() {
		return profile.Profile{}, "", "", fmt.Errorf("target.local_path %q is not a directory", targetDir)
	}
	if err := control.ValidateArtifactLoadBoundary(targetDir); err != nil {
		return profile.Profile{}, "", "", err
	}
	return p, targetDir, cleanProfilePath, nil
}

func daemonTargetContainsReservedControl(path string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(filepath.Clean(path)), "/") {
		if strings.EqualFold(segment, control.DirName) {
			return true
		}
	}
	return false
}

func validateDaemonReceiverMode(p profile.Profile) (bool, error) {
	if serveReceiverMaterialPresent(p) && profileHasPairingPins(p) {
		if _, err := pairing.ValidateProfileTrust(p); err != nil {
			return false, err
		}
		if err := p.ValidateNetworkServerMaterial(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func daemonMode(receiverEnabled bool) string {
	if receiverEnabled {
		return "pairing+receiver"
	}
	return "pairing-only"
}

func ptr[T any](value T) *T {
	return &value
}

func (r Runner) runDaemonStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("daemon status", stderr)
	profilePath := fs.String("profile", "", "--profile target profile path")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "daemon status: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "daemon status: unsupported format %q\n", *format)
		return 2
	}
	p, targetDir, _, err := readDaemonProfile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "daemon status: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	report, err := agentdaemon.BuildStatus(targetDir, p.ProfileID, p.Target.TargetID)
	if err != nil {
		fmt.Fprintf(stderr, "daemon status: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "text":
		printDaemonStatusText(stdout, report)
	case "json":
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			fmt.Fprintf(stderr, "daemon status: encode report: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return 0
}

func (r Runner) runDaemonLogs(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("daemon logs", stderr)
	profilePath := fs.String("profile", "", "--profile target profile path")
	tail := fs.Int("tail", 20, "--tail number of lifecycle events to show")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "daemon logs: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "daemon logs: unsupported format %q\n", *format)
		return 2
	}
	if *tail < 0 {
		fmt.Fprintln(stderr, "daemon logs: --tail cannot be negative")
		return 2
	}
	p, targetDir, _, err := readDaemonProfile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "daemon logs: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	events, err := agentdaemon.ListLifecycleEvents(targetDir)
	if err != nil {
		fmt.Fprintf(stderr, "daemon logs: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	scoped := make([]agentdaemon.LifecycleEvent, 0, len(events))
	scopeIssues := []string{}
	for _, event := range events {
		if agentdaemon.ArtifactInScope(event.ProfileID, event.TargetID, p.ProfileID, p.Target.TargetID) {
			scoped = append(scoped, event)
		} else {
			scopeIssues = append(scopeIssues, "lifecycle_event_scope_mismatch")
		}
	}
	if len(scoped) > *tail {
		scoped = scoped[len(scoped)-*tail:]
	}
	switch *format {
	case "text":
		fmt.Fprintf(stdout, "daemon_logs profile=%s target=%s events=%d scope_issues=%s\n",
			encodeTextValue(p.ProfileID),
			encodeTextValue(p.Target.TargetID),
			len(scoped),
			encodeTextList(uniqueStrings(scopeIssues)),
		)
		for _, event := range scoped {
			fmt.Fprintf(stdout, "daemon_event id=%s type=%s recorded_at=%s message=%s details=%s\n",
				encodeTextValue(event.ID),
				encodeTextValue(event.Type),
				encodeTextValue(event.RecordedAt),
				encodeTextValue(statusTextValueOrDash(event.Message)),
				encodeTextMap(event.Details),
			)
		}
	case "json":
		doc := struct {
			Version     int                          `json:"version"`
			ProfileID   string                       `json:"profile_id"`
			TargetID    string                       `json:"target_id"`
			Events      []agentdaemon.LifecycleEvent `json:"events"`
			ScopeIssues []string                     `json:"scope_issues,omitempty"`
		}{
			Version:     agentdaemon.CurrentVersion,
			ProfileID:   p.ProfileID,
			TargetID:    p.Target.TargetID,
			Events:      scoped,
			ScopeIssues: uniqueStrings(scopeIssues),
		}
		if err := json.NewEncoder(stdout).Encode(doc); err != nil {
			fmt.Fprintf(stderr, "daemon logs: encode report: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return 0
}

func (r Runner) runDaemonRestart(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("daemon restart", stderr)
	profilePath := fs.String("profile", "", "--profile target profile path")
	reason := fs.String("reason", "", "--reason optional operator reason to persist with restart intent")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "daemon restart: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "daemon restart: unsupported format %q\n", *format)
		return 2
	}
	p, targetDir, _, err := readDaemonProfile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "daemon restart: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	state, err := agentdaemon.ReadState(targetDir)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(stderr, "daemon restart: no foreground daemon state is present; run `supermover daemon run --foreground --profile <path>` first")
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "daemon restart: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if !agentdaemon.ArtifactInScope(state.ProfileID, state.TargetID, p.ProfileID, p.Target.TargetID) {
		fmt.Fprintln(stderr, "daemon restart: persisted daemon state belongs to a different profile or target")
		return 2
	}
	if state.Status != agentdaemon.StatusStarting && state.Status != agentdaemon.StatusRunning {
		fmt.Fprintf(stderr, "daemon restart: state=%s is not a running foreground daemon; run `supermover daemon run --foreground --profile <path>` first\n", safeDiagnosticLine(state.Status))
		return 2
	}
	now := r.nowFunc()()
	intent := agentdaemon.NewRestartIntent(p.ProfileID, p.Target.TargetID, *reason, os.Getpid(), now)
	if err := withLockedTarget(targetDir, func() error {
		return agentdaemon.WriteRestartIntent(targetDir, intent)
	}); err != nil {
		fmt.Fprintf(stderr, "daemon restart: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_restart_requested", "foreground daemon restart intent persisted", map[string]string{"reason": intent.Reason}, now)
	switch *format {
	case "text":
		fmt.Fprintf(stdout, "daemon: restart_requested profile=%s target=%s requested_at=%s foreground_signal=restart-intent service_manager=none consumption=pending\n",
			encodeTextValue(p.ProfileID),
			encodeTextValue(p.Target.TargetID),
			encodeTextValue(intent.RequestedAt),
		)
	case "json":
		doc := struct {
			Version int                       `json:"version"`
			Intent  agentdaemon.RestartIntent `json:"restart_intent"`
		}{Version: agentdaemon.CurrentVersion, Intent: intent}
		if err := json.NewEncoder(stdout).Encode(doc); err != nil {
			fmt.Fprintf(stderr, "daemon restart: encode report: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return 0
}

func (r Runner) runDaemonStop(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("daemon stop", stderr)
	profilePath := fs.String("profile", "", "--profile target profile path")
	reason := fs.String("reason", "", "--reason optional operator reason to persist with stop intent")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "daemon stop: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, targetDir, _, err := readDaemonProfile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "daemon stop: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	now := r.nowFunc()()
	intent := agentdaemon.NewStopIntent(p.ProfileID, p.Target.TargetID, *reason, os.Getpid(), now)
	if err := withLockedTarget(targetDir, func() error {
		if err := agentdaemon.WriteStopIntent(targetDir, intent); err != nil {
			return err
		}
		state, err := agentdaemon.ReadState(targetDir)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if agentdaemon.ArtifactInScope(state.ProfileID, state.TargetID, p.ProfileID, p.Target.TargetID) {
			if state.Status == agentdaemon.StatusStarting || state.Status == agentdaemon.StatusRunning {
				state.Status = agentdaemon.StatusStopping
			}
			state.StopIntent = ptr(agentdaemon.StopSummary(intent))
			state.UpdatedAt = now.Format(time.RFC3339Nano)
			return agentdaemon.WriteState(targetDir, state)
		}
		return nil
	}); err != nil {
		fmt.Fprintf(stderr, "daemon stop: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_stop_requested", "foreground daemon stop requested", map[string]string{"reason": intent.Reason}, now)
	fmt.Fprintf(stdout, "daemon: stop_requested profile=%s target=%s requested_at=%s foreground_signal=stop-intent\n",
		encodeTextValue(p.ProfileID),
		encodeTextValue(p.Target.TargetID),
		encodeTextValue(intent.RequestedAt),
	)
	return 0
}

func printDaemonStatusText(w io.Writer, report agentdaemon.StatusReport) {
	fmt.Fprintf(w, "daemon_status profile=%s target=%s installed=%t state=%s run_mode=%s service_manager=%s scope_issues=%s install_profile=%s state_profile=%s pid=%d mode=%s pairing_address=%s receiver_address=%s stop_requested=%t restart_requested=%t lifecycle_events=%d\n",
		encodeTextValue(report.ProfileID),
		encodeTextValue(report.TargetID),
		report.Installed,
		encodeTextValue(report.State),
		encodeTextValue(statusTextValueOrDash(report.RunMode)),
		encodeTextValue(statusTextValueOrDash(report.ServiceManager)),
		encodeTextList(report.ScopeIssues),
		encodeTextValue(daemonInstallProfilePath(report.Install)),
		encodeTextValue(daemonStateProfilePath(report.StateRecord)),
		daemonStatePID(report.StateRecord),
		encodeTextValue(daemonStateMode(report.StateRecord)),
		encodeTextValue(daemonStatePairingAddress(report.StateRecord)),
		encodeTextValue(daemonStateReceiverAddress(report.StateRecord)),
		report.StopIntent != nil,
		report.RestartIntent != nil,
		len(report.LifecycleEvents),
	)
	if report.StopIntent != nil {
		fmt.Fprintf(w, "daemon_stop_intent requested_at=%s reason=%s requested_by_pid=%d\n",
			encodeTextValue(report.StopIntent.RequestedAt),
			encodeTextValue(statusTextValueOrDash(report.StopIntent.Reason)),
			report.StopIntent.RequestedByPID,
		)
	}
	if report.RestartIntent != nil {
		fmt.Fprintf(w, "daemon_restart_intent requested_at=%s reason=%s requested_by_pid=%d\n",
			encodeTextValue(report.RestartIntent.RequestedAt),
			encodeTextValue(statusTextValueOrDash(report.RestartIntent.Reason)),
			report.RestartIntent.RequestedByPID,
		)
	}
	for _, event := range report.LifecycleEvents {
		fmt.Fprintf(w, "daemon_event id=%s type=%s recorded_at=%s message=%s details=%s\n",
			encodeTextValue(event.ID),
			encodeTextValue(event.Type),
			encodeTextValue(event.RecordedAt),
			encodeTextValue(statusTextValueOrDash(event.Message)),
			encodeTextMap(event.Details),
		)
	}
}

func daemonInstallProfilePath(install *agentdaemon.Install) string {
	if install == nil {
		return "-"
	}
	return statusTextValueOrDash(install.ProfilePath)
}

func daemonStateProfilePath(state *agentdaemon.State) string {
	if state == nil {
		return "-"
	}
	return statusTextValueOrDash(state.ProfilePath)
}

func daemonStatePID(state *agentdaemon.State) int {
	if state == nil {
		return 0
	}
	return state.PID
}

func daemonStateMode(state *agentdaemon.State) string {
	if state == nil {
		return "-"
	}
	return statusTextValueOrDash(state.Mode)
}

func daemonStatePairingAddress(state *agentdaemon.State) string {
	if state == nil {
		return "-"
	}
	return statusTextValueOrDash(state.PairingAddress)
}

func daemonStateReceiverAddress(state *agentdaemon.State) string {
	if state == nil {
		return "-"
	}
	return statusTextValueOrDash(state.ReceiverAddress)
}

type serveResult struct {
	name string
	err  error
}

func serveReceiverMaterialPresent(p profile.Profile) bool {
	if p.Network == nil {
		return false
	}
	return strings.TrimSpace(p.Network.ReceiverURL) != "" ||
		strings.TrimSpace(p.Network.LocalTLSIdentity.CertificatePath) != "" ||
		strings.TrimSpace(p.Network.LocalTLSIdentity.PrivateKeyPath) != ""
}

func (r Runner) newPairingServe(p profile.Profile, listen string, stderr io.Writer, outputMu *sync.Mutex, receiverEnabled bool) (*pairserve.Server, error) {
	return pairserve.New(pairserve.Options{
		Profile: p,
		Listen:  listen,
		Ready: func(info pairserve.ReadyInfo) {
			mode := "pairing-only"
			if receiverEnabled {
				mode = "pairing"
			}
			outputMu.Lock()
			fmt.Fprintf(stderr, "serve: listening address=%s mode=%s verification_code=%s expires_at=%s trusted=false transfer=false\n", info.Address, mode, info.VerificationCode, info.ExpiresAt.Format(time.RFC3339Nano))
			outputMu.Unlock()
			if r.ServePairingReady != nil {
				r.ServePairingReady(info)
			}
			if r.ServeReady != nil {
				r.ServeReady(info.Address)
			}
		},
	})
}

func (r Runner) newReceiverServe(p profile.Profile, stderr io.Writer, outputMu *sync.Mutex) (*receiverserve.Server, error) {
	return receiverserve.New(receiverserve.Options{
		Profile: p,
		Now: func() time.Time {
			if r.Now.IsZero() {
				return time.Now()
			}
			return r.Now
		},
		Ready: func(info receiverserve.ReadyInfo) {
			outputMu.Lock()
			fmt.Fprintf(stderr, "serve: receiver listening address=%s mode=receiver-tls trusted=true receiver_routes=true push_network=true\n", info.Address)
			outputMu.Unlock()
			if r.ServeReceiverReady != nil {
				r.ServeReceiverReady(info)
			}
		},
	})
}

func (r Runner) runDiscover(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("discover", stderr)
	timeout := fs.String("timeout", "2s", "--timeout for address hints; discovery is not trust")
	format := fs.String("format", "text", "output format: text or json")
	addresses := multiFlag{}
	fs.Var(&addresses, "address", "--address explicit host:port hint; repeatable; not trusted")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "discover: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	timeoutValue, err := time.ParseDuration(*timeout)
	if err != nil || timeoutValue <= 0 {
		fmt.Fprintf(stderr, "discover: invalid --timeout %q\n", *timeout)
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "discover: unsupported format %q\n", *format)
		return 2
	}
	for _, address := range addresses {
		if _, err := discovery.NewAddressHint(address, discovery.NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "abcdef0123456789", []string{"pair"}), time.Unix(0, 0).UTC(), discovery.DefaultHintTTL); err != nil {
			fmt.Fprintf(stderr, "discover: invalid address hint %q\n", address)
			return 2
		}
	}
	now := r.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutValue)
	defer cancel()
	source := discovery.Source(discovery.EmptySource{})
	if len(addresses) > 0 {
		source = discovery.StaticSource{
			Addresses:       addresses,
			ServiceType:     "_supermover._tcp",
			ProtocolVersion: "supermover/1",
			Nonce:           deterministicDiscoveryNonce(now, addresses),
			Capabilities:    []string{"pair"},
			TTL:             timeoutValue,
		}
	}
	hints, err := discovery.Collect(ctx, source, now)
	if err != nil {
		fmt.Fprintf(stderr, "discover: %v\n", err)
		return 1
	}
	switch *format {
	case "text":
		printDiscoveryText(stdout, hints)
	case "json":
		if err := json.NewEncoder(stdout).Encode(hints); err != nil {
			fmt.Fprintf(stderr, "discover: encode hints: %v\n", err)
			return 1
		}
	}
	return 0
}

func (r Runner) runPair(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("pair", stderr)
	profilePath := fs.String("profile", "", "--profile source profile path")
	target := fs.String("target", "", "--target host:port or http(s) pairing endpoint")
	verificationCode := fs.String("verification-code", "", "--verification-code shown by target serve")
	method := fs.String("method", "sas", "verification method: sas, short_code, qr, or tofu; writes local pairing evidence")
	timeout := fs.String("timeout", "5s", "--timeout for pairing bootstrap request")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" || strings.TrimSpace(*target) == "" {
		fmt.Fprintln(stderr, "pair: --profile and --target are required")
		return 2
	}
	if strings.TrimSpace(*verificationCode) == "" {
		fmt.Fprintln(stderr, "pair: --verification-code is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "pair: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	pairingMethod := transport.PairingMethod(*method)
	if err := pairingMethod.Validate(); err != nil {
		fmt.Fprintf(stderr, "pair: unsupported --method %q\n", *method)
		return 2
	}
	timeoutValue, err := time.ParseDuration(*timeout)
	if err != nil || timeoutValue <= 0 {
		fmt.Fprintf(stderr, "pair: invalid --timeout %q\n", *timeout)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 2
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 2
	}
	ctx := r.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutValue)
	defer cancel()
	expectedTargetDeviceID, err := pairing.TargetDeviceID(p)
	if err != nil {
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 2
	}
	bootstrap, err := fetchPairingBootstrap(ctx, *target, *verificationCode)
	if err != nil {
		if errors.Is(err, pairing.ErrVerificationCode) {
			fmt.Fprintf(stderr, "pair: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
	}
	now := r.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := pairing.ValidateBootstrap(bootstrap, expectedTargetDeviceID, *verificationCode, now); err != nil {
		if errors.Is(err, pairing.ErrVerificationCode) || errors.Is(err, pairing.ErrInvalidBootstrap) {
			fmt.Fprintf(stderr, "pair: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
	}
	if profileHasPairingPins(p) {
		state, err := pairing.ValidateProfileTrust(p)
		if err != nil {
			fmt.Fprintf(stderr, "pair: existing paired profile is invalid: %v\n", err)
			return 2
		}
		if state.TargetDeviceID != bootstrap.TargetDeviceID {
			fmt.Fprintln(stderr, "pair: profile is already paired to a different target identity; explicit re-pair/rotate is not implemented")
			return 2
		}
		fmt.Fprintf(stdout, "pair: identity already pinned receipt=%s transfer=false\n", p.Target.PairingReceiptID)
		return 0
	}
	result, err := writePairingArtifacts(*profilePath, targetDir, p, bootstrap, pairingMethod, now)
	if err != nil {
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "pair: pinned target identity receipt=%s transfer=false\n", result.Receipt.ID)
	return 0
}

type pairingWriteResult struct {
	Receipt control.PairingReceipt
	Profile profile.Profile
}

func fetchPairingBootstrap(ctx context.Context, target string, verificationCode string) (pairing.Bootstrap, error) {
	endpoint, err := pairingEndpointURL(target)
	if err != nil {
		return pairing.Bootstrap{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return pairing.Bootstrap{}, err
	}
	req.Header.Set(pairing.VerificationCodeHeader, strings.TrimSpace(verificationCode))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return pairing.Bootstrap{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return pairing.Bootstrap{}, pairing.ErrVerificationCode
	}
	if resp.StatusCode != http.StatusOK {
		return pairing.Bootstrap{}, fmt.Errorf("pairing endpoint returned HTTP %d", resp.StatusCode)
	}
	var bootstrap pairing.Bootstrap
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxPairingBootstrapBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bootstrap); err != nil {
		return pairing.Bootstrap{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("unexpected trailing JSON document")
		}
		return pairing.Bootstrap{}, err
	}
	return bootstrap, nil
}

func pairingEndpointURL(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", errors.New("target is required")
	}
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		parsed, err := url.Parse(target)
		if err != nil || parsed.Host == "" {
			return "", fmt.Errorf("invalid target %q", target)
		}
		if parsed.Path == "" || parsed.Path == "/" {
			parsed.Path = "/v1/pairing"
		}
		return parsed.String(), nil
	}
	if _, _, err := net.SplitHostPort(target); err != nil {
		return "", fmt.Errorf("invalid target address %q", target)
	}
	return "http://" + target + "/v1/pairing", nil
}

func profileHasPairingPins(p profile.Profile) bool {
	return strings.TrimSpace(p.Target.DevicePublicKey) != "" ||
		strings.TrimSpace(p.Target.PairingReceiptID) != "" ||
		strings.TrimSpace(p.Target.PairedAt) != ""
}

func writePairingArtifacts(profilePath string, targetDir string, p profile.Profile, bootstrap pairing.Bootstrap, method transport.PairingMethod, now time.Time) (pairingWriteResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	sourceDeviceID, err := pairing.SourceDeviceID(p)
	if err != nil {
		return pairingWriteResult{}, err
	}
	receiptID := localPairingReceiptID(p, bootstrap)
	verifiedAt := now.Format(time.RFC3339Nano)
	receipt := control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               receiptID,
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   sourceDeviceID,
		TargetDeviceID:   bootstrap.TargetDeviceID,
		DevicePublicKey:  bootstrap.TargetDeviceID,
		Method:           string(method),
		VerifiedAt:       verifiedAt,
		VerificationHash: bootstrap.VerificationHash,
		ProtocolVersion:  protocol.Version,
	}
	updated := p
	updated.Target.DevicePublicKey = bootstrap.TargetDeviceID
	updated.Target.PairingReceiptID = receipt.ID
	updated.Target.PairedAt = verifiedAt
	if err := updated.Validate(); err != nil {
		return pairingWriteResult{}, err
	}
	if err := preflightProfileWrite(profilePath); err != nil {
		return pairingWriteResult{}, err
	}
	receiptPath, err := control.Path(targetDir, control.ArtifactPairingReceipt, receipt.ID)
	if err != nil {
		return pairingWriteResult{}, err
	}
	snapshot, err := pairingProfileSnapshot(updated, receipt.ID, now)
	if err != nil {
		return pairingWriteResult{}, err
	}
	snapshotPath, err := control.Path(targetDir, control.ArtifactProfileSnapshot, "profile-"+receipt.ID)
	if err != nil {
		return pairingWriteResult{}, err
	}
	if err := preflightNewControlArtifact(receiptPath); err != nil {
		return pairingWriteResult{}, err
	}
	if err := preflightNewControlArtifact(snapshotPath); err != nil {
		return pairingWriteResult{}, err
	}
	if err := control.WriteNewFile(receiptPath, receipt); err != nil {
		return pairingWriteResult{}, err
	}
	if err := control.WriteNewFile(snapshotPath, snapshot); err != nil {
		return pairingWriteResult{}, err
	}
	if _, err := pairing.ValidateProfileTrust(updated); err != nil {
		return pairingWriteResult{}, err
	}
	if err := profile.WriteFile(profilePath, updated); err != nil {
		return pairingWriteResult{}, err
	}
	return pairingWriteResult{Receipt: receipt, Profile: updated}, nil
}

func localPairingReceiptID(p profile.Profile, bootstrap pairing.Bootstrap) string {
	sum := sha256.Sum256([]byte(protocol.Version + "\n" + p.ProfileID + "\n" + p.Target.TargetID + "\n" + bootstrap.TargetDeviceID + "\n" + bootstrap.ChallengeID))
	return "pair-" + hex.EncodeToString(sum[:8])
}

func preflightNewControlArtifact(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("control artifact %q exists as a symlink", path)
		}
		return fmt.Errorf("control artifact %q already exists", path)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func preflightProfileWrite(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("profile path is required")
	}
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("profile directory %q is a symlink", parent)
	}
	if !info.IsDir() {
		return fmt.Errorf("profile directory %q is not a directory", parent)
	}
	if existing, err := os.Lstat(path); err == nil {
		if existing.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("profile file %q is a symlink", path)
		}
		if existing.IsDir() {
			return fmt.Errorf("profile file %q is a directory", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temp, err := os.CreateTemp(parent, ".profile-preflight-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	if closeErr := temp.Close(); closeErr != nil {
		_ = os.Remove(name)
		return closeErr
	}
	return os.Remove(name)
}

func pairingProfileSnapshot(p profile.Profile, receiptID string, capturedAt time.Time) (control.ProfileSnapshot, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return control.ProfileSnapshot{}, err
	}
	return control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + receiptID,
		ProfileID:  p.ProfileID,
		CapturedAt: capturedAt.UTC().Format(time.RFC3339Nano),
		Profile:    payload,
	}, nil
}

func (r Runner) runRecover(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("recover", stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of recover:
  supermover recover --profile <path> [--session <id>] [--dry-run|--rollback-incomplete] [--format text|json]

Reviews and repairs local target control-plane sessions only. It can replay
safely staged local sessions or mark incomplete local sessions for review, but
it does not contact network receivers, resume push --network uploads, run a
network retry policy, or perform broad reconcile.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "optional session id to recover")
	dryRun := fs.Bool("dry-run", false, "report recovery actions without mutating target state")
	rollbackIncomplete := fs.Bool("rollback-incomplete", false, "mark received/validated sessions as rolled_back when they never reached durable staging")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
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

func readProfileFilePayload(path string) (profile.Profile, []byte, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return profile.Profile{}, nil, err
	}
	p, err := profile.Read(bytes.NewReader(payload))
	if err != nil {
		return profile.Profile{}, nil, err
	}
	return p, payload, nil
}

type scanReport struct {
	ProfileID      string              `json:"profile_id"`
	Roots          []scan.Result       `json:"roots"`
	EntryCount     int                 `json:"entry_count"`
	WarningCount   int                 `json:"warning_count"`
	InfluenceCount int                 `json:"influence_count"`
	Influence      []agentkb.Influence `json:"influence,omitempty"`
}

type networkPushPlan struct {
	ProfileID         string `json:"profile_id"`
	TargetID          string `json:"target_id"`
	SourceDeviceID    string `json:"source_device_id"`
	TargetDeviceID    string `json:"target_device_id"`
	PairingReceiptID  string `json:"pairing_receipt_id"`
	SessionID         string `json:"session_id,omitempty"`
	DryRun            bool   `json:"dry_run"`
	Transfer          string `json:"transfer"`
	EncryptedTransfer string `json:"encrypted_transfer"`
	Resume            string `json:"resume"`
	ResumeAuthority   string `json:"resume_authority"`
	ResumeOutcome     string `json:"resume_outcome"`
	ResumedBytes      int64  `json:"resumed_bytes,omitempty"`
	Files             int    `json:"files"`
	Bytes             int64  `json:"bytes"`
	Chunks            int    `json:"chunks,omitempty"`
	Warnings          int    `json:"warnings"`
	Status            string `json:"status,omitempty"`
	Stage             string `json:"stage,omitempty"`
	ErrorCode         string `json:"error_code,omitempty"`
}

type syncQueueSummaryResult struct {
	Operation string                  `json:"operation"`
	Mode      string                  `json:"mode"`
	State     string                  `json:"state"`
	Summary   incrementalsync.Summary `json:"summary"`
}

type syncQueueEntriesResult struct {
	Operation string                       `json:"operation"`
	Mode      string                       `json:"mode"`
	State     string                       `json:"state"`
	Summary   incrementalsync.Summary      `json:"summary"`
	Entries   []incrementalsync.QueueEntry `json:"entries,omitempty"`
}

type syncQueueEnqueueResult struct {
	Operation string                         `json:"operation"`
	Mode      string                         `json:"mode"`
	Scope     incrementalsync.Scope          `json:"scope"`
	StatePath string                         `json:"state_path"`
	Summary   incrementalsync.Summary        `json:"summary"`
	Enqueued  []incrementalsync.QueueEntry   `json:"enqueued,omitempty"`
	Skipped   []incrementalsync.SkippedEntry `json:"skipped,omitempty"`
	Audit     []audit.Record                 `json:"audit,omitempty"`
}

type syncQueueCancelResult struct {
	Operation string                     `json:"operation"`
	Mode      string                     `json:"mode"`
	State     string                     `json:"state"`
	Reason    string                     `json:"reason"`
	Entry     incrementalsync.QueueEntry `json:"entry"`
	Summary   incrementalsync.Summary    `json:"summary"`
}

type pruneReleaseReview struct {
	Schema           string                   `json:"schema"`
	Scope            string                   `json:"scope"`
	TargetRoot       string                   `json:"target_root"`
	ProfileID        string                   `json:"profile_id,omitempty"`
	TargetID         string                   `json:"target_id,omitempty"`
	SessionFilter    string                   `json:"session_filter,omitempty"`
	LatestSessionID  string                   `json:"latest_session_id,omitempty"`
	Status           string                   `json:"status"`
	ReviewRequired   bool                     `json:"review_required"`
	Action           string                   `json:"action"`
	ReadOnly         bool                     `json:"read_only"`
	Authorization    pruneReviewAuthorization `json:"authorization"`
	PruneReview      report.PruneReview       `json:"prune_review"`
	ArtifactProblems []report.ArtifactProblem `json:"artifact_problems,omitempty"`
}

type pruneReviewAuthorization struct {
	ApprovalBypass  bool   `json:"approval_bypass"`
	ApprovalWriting string `json:"approval_writing"`
	ReceiptWriting  string `json:"receipt_writing"`
	PhysicalPruning string `json:"physical_pruning"`
	TargetDeletion  string `json:"target_deletion"`
	ApplyRequires   string `json:"apply_requires"`
}

func scanProfile(p profile.Profile) (scanReport, error) {
	report := scanReport{ProfileID: p.ProfileID}
	categories := agentKnowledgeCategories(p.AgentKnowledge)
	for _, root := range p.Roots {
		result, err := scan.Scan(root.Path)
		if err != nil {
			return scanReport{}, err
		}
		report.EntryCount += len(result.Entries)
		report.WarningCount += len(result.Audit)
		report.Influence = append(report.Influence, agentkb.Detect(result.Entries, categories)...)
		report.Roots = append(report.Roots, result)
	}
	report.InfluenceCount = len(report.Influence)
	return report, nil
}

func agentKnowledgeCategories(config profile.AgentKnowledge) []agentkb.KnowledgeCategory {
	categories := make([]agentkb.KnowledgeCategory, 0, len(config.Categories))
	for _, category := range config.Categories {
		categories = append(categories, agentkb.KnowledgeCategory{
			Name:     agentkb.Category(category.Name),
			Paths:    append([]string(nil), category.Paths...),
			Manifest: category.Manifest,
		})
	}
	return categories
}

func printScanText(w io.Writer, report scanReport) {
	fmt.Fprintf(w, "profile=%s roots=%d entries=%d warnings=%d influences=%d\n", report.ProfileID, len(report.Roots), report.EntryCount, report.WarningCount, report.InfluenceCount)
	for _, root := range report.Roots {
		fmt.Fprintf(w, "root=%s entries=%d warnings=%d\n", root.Root, len(root.Entries), len(root.Audit))
	}
}

func printNetworkPushPlanText(w io.Writer, plan networkPushPlan) {
	session := plan.SessionID
	if session == "" {
		session = "-"
	}
	fmt.Fprintf(w, "network push: profile=%s target_id=%s source_device=%s target_device=%s pairing_receipt=%s session=%s dry_run=%t transfer=%s encrypted_transfer=%s resume=%s resume_authority=%s resume_outcome=%s resumed_bytes=%d files=%d bytes=%d chunks=%d warnings=%d status=%s stage=%s error_code=%s\n",
		encodeTextValue(plan.ProfileID),
		encodeTextValue(plan.TargetID),
		encodeTextValue(plan.SourceDeviceID),
		encodeTextValue(plan.TargetDeviceID),
		encodeTextValue(plan.PairingReceiptID),
		encodeTextValue(session),
		plan.DryRun,
		plan.Transfer,
		plan.EncryptedTransfer,
		plan.Resume,
		plan.ResumeAuthority,
		plan.ResumeOutcome,
		plan.ResumedBytes,
		plan.Files,
		plan.Bytes,
		plan.Chunks,
		plan.Warnings,
		encodeTextValue(defaultTextField(plan.Status)),
		encodeTextValue(defaultTextField(plan.Stage)),
		encodeTextValue(defaultTextField(plan.ErrorCode)),
	)
}

func networkPushResultFromRun(p profile.Profile, trust pairing.TrustState, dryRun bool, result networkpush.Result) networkPushPlan {
	transfer := string(result.TransferStatus)
	status := string(result.TransferStatus)
	stage := result.TransferStage
	resume := defaultTextField(result.ResumeAuthority)
	resumeAuthority := defaultTextField(result.ResumeAuthority)
	resumeOutcome := defaultTextField(result.ResumeOutcome)
	if dryRun {
		transfer = "dry_run"
		status = "dry_run"
		resume = "not_attempted"
		resumeAuthority = "not_attempted"
		resumeOutcome = "not_attempted"
	}
	if transfer == "" {
		transfer = "failed"
	}
	return networkPushPlan{
		ProfileID:         p.ProfileID,
		TargetID:          p.Target.TargetID,
		SourceDeviceID:    trust.Receipt.SourceDeviceID,
		TargetDeviceID:    trust.TargetDeviceID,
		PairingReceiptID:  trust.Receipt.ID,
		SessionID:         result.SessionID,
		DryRun:            dryRun,
		Transfer:          transfer,
		EncryptedTransfer: networkPushEncryptedTransfer(dryRun),
		Resume:            resume,
		ResumeAuthority:   resumeAuthority,
		ResumeOutcome:     resumeOutcome,
		ResumedBytes:      result.ResumedBytes,
		Files:             result.Files,
		Bytes:             result.Bytes,
		Chunks:            result.Chunks,
		Warnings:          result.Warnings,
		Status:            status,
		Stage:             stage,
		ErrorCode:         result.TransferCode,
	}
}

func networkPushEncryptedTransfer(dryRun bool) string {
	if dryRun {
		return "profile_backed_mtls_validated"
	}
	return "tls13_mtls"
}

func printNetworkPushResult(w io.Writer, format string, plan networkPushPlan) error {
	switch format {
	case "text":
		printNetworkPushPlanText(w, plan)
	case "json":
		return json.NewEncoder(w).Encode(plan)
	}
	return nil
}

func syncQueueSummaryOrMissing(scheduler *incrementalsync.Scheduler, p profile.Profile, statePath string, now time.Time) (incrementalsync.Summary, string, error) {
	summary, err := scheduler.Summary(incrementalQueueScope(p))
	if err == nil {
		return summary, "present", nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return incrementalsync.Summary{}, "", err
	}
	if statePath == "" {
		if computed, statePathErr := scheduler.StatePath(incrementalQueueScope(p)); statePathErr == nil {
			statePath = computed
		}
	}
	return syncQueueEmptySummary(p, statePath, now), "missing", nil
}

func syncQueueEmptySummary(p profile.Profile, statePath string, now time.Time) incrementalsync.Summary {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return incrementalsync.Summary{
		ProfileID:   p.ProfileID,
		TargetID:    p.Target.TargetID,
		StatePath:   statePath,
		GeneratedAt: now.UTC().Format(time.RFC3339Nano),
	}
}

func printSyncQueueEnqueueResult(stdout io.Writer, stderr io.Writer, format string, result syncQueueEnqueueResult) int {
	switch format {
	case "text":
		printSyncQueueSummaryLine(stdout, "sync_queue_enqueue", result.Mode, "present", result.Summary)
		for _, entry := range result.Enqueued {
			printSyncQueueEntry(stdout, "sync_queue_entry", entry)
		}
		for _, skipped := range result.Skipped {
			fmt.Fprintf(stdout, "sync_queue_skipped root=%s path=%s reason=%s\n",
				encodeTextValue(skipped.Root),
				encodeTextValue(skipped.Path),
				encodeTextValue(skipped.Reason),
			)
		}
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "sync queue enqueue: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return 0
}

func printSyncQueueSummaryResult(stdout io.Writer, stderr io.Writer, format string, result syncQueueSummaryResult) int {
	switch format {
	case "text":
		printSyncQueueSummaryLine(stdout, "sync_queue_status", result.Mode, result.State, result.Summary)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "sync queue status: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return 0
}

func printSyncQueueEntriesResult(stdout io.Writer, stderr io.Writer, format string, result syncQueueEntriesResult) int {
	switch format {
	case "text":
		printSyncQueueSummaryLine(stdout, "sync_queue_ready", result.Mode, result.State, result.Summary)
		for _, entry := range result.Entries {
			printSyncQueueEntry(stdout, "sync_queue_ready_entry", entry)
		}
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "sync queue ready: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return 0
}

func printSyncQueueCancelResult(stdout io.Writer, stderr io.Writer, format string, result syncQueueCancelResult) int {
	switch format {
	case "text":
		fmt.Fprintf(stdout, "sync_queue_cancel mode=%s state=%s id=%s status=%s reason=%s\n",
			encodeTextValue(result.Mode),
			encodeTextValue(result.State),
			encodeTextValue(result.Entry.ID),
			encodeTextValue(result.Entry.Status),
			encodeTextValue(result.Reason),
		)
		printSyncQueueSummaryLine(stdout, "sync_queue_status", result.Mode, result.State, result.Summary)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "sync queue cancel: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return 0
}

func printSyncQueueSummaryLine(w io.Writer, label string, mode string, state string, summary incrementalsync.Summary) {
	fmt.Fprintf(w, "%s mode=%s state=%s profile=%s target_id=%s queued=%d backoff=%d canceled=%d done=%d ready=%d total=%d warnings=%d state_path=%s generated_at=%s\n",
		label,
		encodeTextValue(mode),
		encodeTextValue(state),
		encodeTextValue(summary.ProfileID),
		encodeTextValue(summary.TargetID),
		summary.Queued,
		summary.Backoff,
		summary.Canceled,
		summary.Done,
		summary.Ready,
		summary.Total,
		summary.WarningCount,
		encodeTextValue(defaultTextField(summary.StatePath)),
		encodeTextValue(defaultTextField(summary.GeneratedAt)),
	)
}

func printSyncQueueEntry(w io.Writer, label string, entry incrementalsync.QueueEntry) {
	fmt.Fprintf(w, "%s id=%s profile=%s target_id=%s root=%s path=%s kind=%s status=%s attempts=%d digest=%s symlink_target=%s updated_at=%s\n",
		label,
		encodeTextValue(entry.ID),
		encodeTextValue(entry.ProfileID),
		encodeTextValue(entry.TargetID),
		encodeTextValue(entry.Root),
		encodeTextValue(entry.Path),
		encodeTextValue(string(entry.Kind)),
		encodeTextValue(entry.Status),
		entry.Attempts,
		encodeTextValue(defaultTextField(entry.Digest)),
		encodeTextValue(defaultTextField(entry.SymlinkTarget)),
		encodeTextValue(defaultTextField(entry.UpdatedAt)),
	)
}

func defaultTextField(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func printReconcileReceipt(stdout io.Writer, stderr io.Writer, format string, receipt reconcile.Receipt) error {
	switch format {
	case "text":
		printReconcileText(stdout, receipt)
	case "json":
		if err := json.NewEncoder(stdout).Encode(receipt); err != nil {
			return fmt.Errorf("encode receipt: %w", err)
		}
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
	_ = stderr
	return nil
}

func printReconcileText(w io.Writer, receipt reconcile.Receipt) {
	label := "reconcile_plan"
	if receipt.Schema == reconcile.SchemaApplyReceipt {
		label = "reconcile_apply"
	}
	fmt.Fprintf(w, "%s schema=%s target=%s profile=%s target_id=%s session=%s apply_intent=%t records=%d planned=%d applied=%d noop=%d refused=%d artifact_problems=%d\n",
		label,
		encodeTextValue(receipt.Schema),
		encodeTextValue(receipt.TargetRoot),
		encodeTextValue(receipt.ProfileID),
		encodeTextValue(receipt.TargetID),
		encodeTextValue(defaultTextField(receipt.SessionID)),
		receipt.ApplyIntent,
		receipt.Summary.Records,
		receipt.Summary.Planned,
		receipt.Summary.Applied,
		receipt.Summary.Noop,
		receipt.Summary.Refused,
		receipt.Summary.ArtifactProblems,
	)
	for _, action := range receipt.Actions {
		fmt.Fprintf(w, "reconcile_action drift=%s path=%s change=%s action=%s result=%s session=%s reviewed_at=%s reviewer=%s reason=%s source_root=%s source_path=%s source_size=%d source_digest=%s\n",
			encodeTextValue(action.DriftID),
			encodeTextValue(action.Path),
			encodeTextValue(action.Change),
			encodeTextValue(action.Action),
			encodeTextValue(action.Result),
			encodeTextValue(defaultTextField(action.SessionID)),
			encodeTextValue(defaultTextField(action.ReviewedAt)),
			encodeTextValue(defaultTextField(action.Reviewer)),
			encodeTextValue(defaultTextField(action.Reason)),
			encodeTextValue(defaultTextField(reconcileSourceRoot(action.SourceEvidence))),
			encodeTextValue(defaultTextField(reconcileSourcePath(action.SourceEvidence))),
			reconcileSourceSize(action.SourceEvidence),
			encodeTextValue(defaultTextField(reconcileSourceDigest(action.SourceEvidence))),
		)
	}
	for _, refusal := range receipt.Refusals {
		fmt.Fprintf(w, "reconcile_refusal drift=%s path=%s change=%s action=%s reason=%s message=%s observed_present=%t observed_kind=%s observed_digest=%s\n",
			encodeTextValue(defaultTextField(refusal.DriftID)),
			encodeTextValue(defaultTextField(refusal.Path)),
			encodeTextValue(defaultTextField(refusal.Change)),
			encodeTextValue(defaultTextField(refusal.Action)),
			encodeTextValue(refusal.ReasonCode),
			encodeTextValue(refusal.Message),
			boolValue(refusal.ObservedBefore.Present),
			encodeTextValue(defaultTextField(refusal.ObservedBefore.Kind)),
			encodeTextValue(defaultTextField(refusal.ObservedBefore.Digest)),
		)
	}
	for _, problem := range receipt.ArtifactProblems {
		fmt.Fprintf(w, "reconcile_artifact_problem session=%s path=%s error=%s\n",
			encodeTextValue(defaultTextField(problem.SessionID)),
			encodeTextValue(problem.Path),
			encodeTextValue(problem.Error),
		)
	}
}

func reconcileSourceRoot(source *reconcile.SourceEvidence) string {
	if source == nil {
		return ""
	}
	return source.RootID
}

func reconcileSourcePath(source *reconcile.SourceEvidence) string {
	if source == nil {
		return ""
	}
	return source.Path
}

func reconcileSourceSize(source *reconcile.SourceEvidence) int64 {
	if source == nil {
		return 0
	}
	return source.Size
}

func reconcileSourceDigest(source *reconcile.SourceEvidence) string {
	if source == nil {
		return ""
	}
	return source.Digest
}

func printPruneDryRunText(w io.Writer, report prune.DryRunReport) {
	fmt.Fprintf(w, "prune dry-run: schema=%s target=%s profile=%s target_id=%s policy_mode=%s policy_require_review=%t policy_allow_physical_prune=%t policy_retention_days=%d soft_deletes=%d candidates=%d refusals=%d artifact_problems=%d approval_required=%t physical_pruning=not_applied approval_writing=not_written_by_dry_run receipt_writing=not_written_by_dry_run\n",
		encodeTextValue(report.Schema),
		encodeTextValue(report.TargetRoot),
		encodeTextValue(report.ProfileID),
		encodeTextValue(report.TargetID),
		encodeTextValue(report.ProfileDeletePolicy.Mode),
		report.ProfileDeletePolicy.RequireReview,
		report.ProfileDeletePolicy.AllowPhysicalPrune,
		report.ProfileDeletePolicy.RetentionDays,
		report.Summary.SoftDeletes,
		report.Summary.Candidates,
		report.Summary.Refusals,
		report.Summary.ArtifactProblems,
		report.ApprovalRequired,
	)
	for _, candidate := range report.Candidates {
		fmt.Fprintf(w, "prune_candidate soft_delete=%s session=%s profile=%s target_id=%s root=%s previous_session=%s previous_manifest=%s source=%s target=%s kind=%s size=%d digest=%s previous_source=%s previous_target=%s previous_kind=%s previous_size=%d previous_digest=%s previous_mode=%d previous_mod_time=%s previous_symlink_target=%s observed_present=%t observed_path=%s observed_kind=%s observed_size=%d observed_digest=%s observed_mode=%d observed_mod_time=%s observed_symlink_target=%s action=%s physical_pruning=%s approval_writing=%s receipt_writing=%s review_required=%t\n",
			encodeTextValue(candidate.SoftDeleteID),
			encodeTextValue(candidate.DetectedSessionID),
			encodeTextValue(candidate.ProfileID),
			encodeTextValue(candidate.TargetID),
			encodeTextValue(candidate.RootID),
			encodeTextValue(candidate.PreviousSessionID),
			encodeTextValue(candidate.PreviousManifestID),
			encodeTextValue(candidate.SourcePath),
			encodeTextValue(candidate.TargetPath),
			encodeTextValue(candidate.Kind),
			candidate.Size,
			encodeTextValue(candidate.Digest),
			encodeTextValue(candidate.PreviousManifestEntry.SourcePath),
			encodeTextValue(candidate.PreviousManifestEntry.TargetPath),
			encodeTextValue(candidate.PreviousManifestEntry.Kind),
			candidate.PreviousManifestEntry.Size,
			encodeTextValue(candidate.PreviousManifestEntry.Digest),
			candidate.PreviousManifestEntry.Mode,
			encodeTextValue(candidate.PreviousManifestEntry.ModTime),
			encodeTextValue(candidate.PreviousManifestEntry.SymlinkTarget),
			boolValue(candidate.CurrentTargetState.Present),
			encodeTextValue(candidate.CurrentTargetState.Path),
			encodeTextValue(candidate.CurrentTargetState.Kind),
			candidate.CurrentTargetState.Size,
			encodeTextValue(candidate.CurrentTargetState.Digest),
			candidate.CurrentTargetState.Mode,
			encodeTextValue(candidate.CurrentTargetState.ModTime),
			encodeTextValue(candidate.CurrentTargetState.SymlinkTarget),
			encodeTextValue(candidate.IntendedAction),
			encodeTextValue(candidate.PhysicalPruning),
			encodeTextValue(candidate.ApprovalWriting),
			encodeTextValue(candidate.ReceiptWriting),
			candidate.ReviewRequired,
		)
	}
	for _, refusal := range report.Refusals {
		previous := prune.PreviousManifestEvidence{}
		if refusal.PreviousManifest != nil {
			previous = *refusal.PreviousManifest
		}
		softDeleteKind := ""
		softDeleteDigest := ""
		var softDeleteSize int64
		if refusal.SoftDeleteEvidence != nil {
			softDeleteKind = refusal.SoftDeleteEvidence.Kind
			softDeleteDigest = refusal.SoftDeleteEvidence.Digest
			softDeleteSize = refusal.SoftDeleteEvidence.Size
		}
		fmt.Fprintf(w, "prune_refusal soft_delete=%s session=%s source=%s target=%s soft_delete_kind=%s soft_delete_size=%d soft_delete_digest=%s previous_session=%s previous_manifest=%s previous_source=%s previous_target=%s previous_kind=%s previous_size=%d previous_digest=%s observed_present=%t observed_path=%s observed_kind=%s observed_size=%d observed_digest=%s observed_mode=%d observed_mod_time=%s observed_symlink_target=%s reason=%s message=%s\n",
			encodeTextValue(refusal.SoftDeleteID),
			encodeTextValue(refusal.DetectedSessionID),
			encodeTextValue(refusal.SourcePath),
			encodeTextValue(refusal.TargetPath),
			encodeTextValue(softDeleteKind),
			softDeleteSize,
			encodeTextValue(softDeleteDigest),
			encodeTextValue(previous.SessionID),
			encodeTextValue(previous.ManifestID),
			encodeTextValue(previous.SourcePath),
			encodeTextValue(previous.TargetPath),
			encodeTextValue(previous.Kind),
			previous.Size,
			encodeTextValue(previous.Digest),
			boolValue(refusal.CurrentTargetState.Present),
			encodeTextValue(refusal.CurrentTargetState.Path),
			encodeTextValue(refusal.CurrentTargetState.Kind),
			refusal.CurrentTargetState.Size,
			encodeTextValue(refusal.CurrentTargetState.Digest),
			refusal.CurrentTargetState.Mode,
			encodeTextValue(refusal.CurrentTargetState.ModTime),
			encodeTextValue(refusal.CurrentTargetState.SymlinkTarget),
			encodeTextValue(refusal.ReasonCode),
			encodeTextValue(refusal.Message),
		)
	}
	for _, problem := range report.ArtifactProblems {
		fmt.Fprintf(w, "artifact_problem session=%s path=%s error=%s\n",
			encodeTextValue(problem.SessionID),
			encodeTextValue(problem.Path),
			encodeTextValue(problem.Error),
		)
	}
}

func printPruneApplyText(w io.Writer, result prune.ApplyResult) {
	fmt.Fprintf(w, "prune apply: target=%s profile=%s target_id=%s approval=%s prune_session=%s receipt=%s existing_receipt=%t status=%s items=%d refusals=%d\n",
		encodeTextValue(result.TargetRoot),
		encodeTextValue(result.ProfileID),
		encodeTextValue(result.TargetID),
		encodeTextValue(result.ApprovalID),
		encodeTextValue(result.PruneSessionID),
		encodeTextValue(result.ReceiptPath),
		result.ExistingReceipt,
		result.Receipt.Status,
		len(result.Receipt.Items),
		len(result.Receipt.Refusals),
	)
	for _, item := range result.Receipt.Items {
		fmt.Fprintf(w, "prune_result soft_delete=%s target=%s action=%s result=%s error_code=%s error=%s pruned_at=%s observed_present=%t observed_kind=%s observed_digest=%s observed_symlink_target=%s\n",
			encodeTextValue(item.SoftDeleteID),
			encodeTextValue(item.TargetPath),
			encodeTextValue(item.IntendedAction),
			encodeTextValue(item.Result),
			encodeTextValue(item.ErrorCode),
			encodeTextValue(item.Error),
			encodeTextValue(item.PrunedAt),
			boolValue(item.PrePruneObserved.Present),
			encodeTextValue(item.PrePruneObserved.Kind),
			encodeTextValue(item.PrePruneObserved.Digest),
			encodeTextValue(item.PrePruneObserved.SymlinkTarget),
		)
	}
}

func printPruneApproveText(w io.Writer, result prune.AuthorApprovalResult) {
	fmt.Fprintf(w, "prune_approval id=%s profile=%s target_id=%s root=%s status=%s items=%d approval_path=%s profile_snapshot=%s profile_snapshot_path=%s profile_snapshot_digest=%s approval_scope_digest=%s approved_by=%s approved_at=%s expires_at=%s approval_writing=%s profile_snapshot_writing=%s physical_pruning=%s receipt_writing=%s\n",
		encodeTextValue(result.ApprovalID),
		encodeTextValue(result.ProfileID),
		encodeTextValue(result.TargetID),
		encodeTextValue(result.Approval.RootID),
		encodeTextValue(result.Approval.Status),
		len(result.Approval.Items),
		encodeTextValue(result.ApprovalPath),
		encodeTextValue(result.ProfileSnapshotID),
		encodeTextValue(result.ProfileSnapshotPath),
		encodeTextValue(result.ProfileSnapshotDigest),
		encodeTextValue(result.ApprovalScopeDigest),
		encodeTextValue(result.Approval.ApprovedBy),
		encodeTextValue(result.Approval.ApprovedAt),
		encodeTextValue(defaultTextField(result.Approval.ExpiresAt)),
		encodeTextValue(result.ApprovalWriting),
		encodeTextValue(result.ProfileSnapshotWriting),
		encodeTextValue(result.PhysicalPruning),
		encodeTextValue(result.ReceiptWriting),
	)
	for _, item := range result.Approval.Items {
		fmt.Fprintf(w, "prune_approval_item approval=%s soft_delete=%s session=%s previous_session=%s previous_manifest=%s source=%s target=%s kind=%s size=%d digest=%s symlink_target=%s action=approve_for_prune\n",
			encodeTextValue(result.ApprovalID),
			encodeTextValue(item.SoftDeleteID),
			encodeTextValue(item.DetectedSessionID),
			encodeTextValue(item.PreviousSessionID),
			encodeTextValue(item.PreviousManifestID),
			encodeTextValue(item.SourcePath),
			encodeTextValue(item.TargetPath),
			encodeTextValue(item.Kind),
			item.Size,
			encodeTextValue(defaultTextField(item.Digest)),
			encodeTextValue(defaultTextField(item.SymlinkTarget)),
		)
	}
}

func printPruneApprovalsText(w io.Writer, result prune.ListApprovalsResult) {
	fmt.Fprintf(w, "prune_approvals target=%s profile=%s target_id=%s approvals=%d read_only=%t\n",
		encodeTextValue(result.TargetRoot),
		encodeTextValue(result.ProfileID),
		encodeTextValue(result.TargetID),
		len(result.Approvals),
		true,
	)
	for _, approval := range result.Approvals {
		fmt.Fprintf(w, "prune_approval id=%s profile=%s target_id=%s root=%s status=%s items=%d approved_by=%s approved_at=%s superseded_by=%s superseded_at=%s created_at=%s expires_at=%s review_tool=%s approval_scope_digest=%s approval_reason=%s refusal_reason=%s\n",
			encodeTextValue(approval.ID),
			encodeTextValue(approval.ProfileID),
			encodeTextValue(approval.TargetID),
			encodeTextValue(approval.RootID),
			encodeTextValue(approval.Status),
			len(approval.Items),
			encodeTextValue(defaultTextField(approval.ApprovedBy)),
			encodeTextValue(defaultTextField(approval.ApprovedAt)),
			encodeTextValue(defaultTextField(approval.SupersededBy)),
			encodeTextValue(defaultTextField(approval.SupersededAt)),
			encodeTextValue(approval.CreatedAt),
			encodeTextValue(defaultTextField(approval.ExpiresAt)),
			encodeTextValue(approval.ReviewTool),
			encodeTextValue(defaultTextField(approval.ApprovalScopeDigest)),
			encodeTextValue(defaultTextField(approval.ApprovalReason)),
			encodeTextValue(defaultTextField(approval.RefusalReason)),
		)
	}
}

func printPruneSupersedeText(w io.Writer, result prune.SupersedeApprovalResult) {
	fmt.Fprintf(w, "prune_approval_supersede id=%s profile=%s target_id=%s approval_path=%s status=%s approved_by=%s approved_at=%s superseded_by=%s superseded_at=%s review_tool=%s refusal_reason=%s physical_pruning=%s receipt_writing=%s\n",
		encodeTextValue(result.ApprovalID),
		encodeTextValue(result.ProfileID),
		encodeTextValue(result.TargetID),
		encodeTextValue(result.ApprovalPath),
		encodeTextValue(result.Approval.Status),
		encodeTextValue(defaultTextField(result.Approval.ApprovedBy)),
		encodeTextValue(defaultTextField(result.Approval.ApprovedAt)),
		encodeTextValue(defaultTextField(result.Approval.SupersededBy)),
		encodeTextValue(defaultTextField(result.Approval.SupersededAt)),
		encodeTextValue(result.Approval.ReviewTool),
		encodeTextValue(defaultTextField(result.Approval.RefusalReason)),
		encodeTextValue("not_applied"),
		encodeTextValue("not_written_by_supersede"),
	)
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func encodeTextValue(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			strings.ContainsRune("._:-", rune(c)) {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

func encodeTextList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	encoded := make([]string, 0, len(values))
	for _, value := range sortedStrings(values) {
		encoded = append(encoded, encodeTextValue(value))
	}
	return strings.Join(encoded, ",")
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

func encodeTextMap(values map[string]string) string {
	if len(values) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, encodeTextValue(key)+"="+encodeTextValue(values[key]))
	}
	return strings.Join(parts, ",")
}

func pruneReleaseReviewFromReport(pruneReport report.PruneReviewReport) pruneReleaseReview {
	return pruneReleaseReview{
		Schema:          "supermover.prune_release_review.v1",
		Scope:           pruneReport.Scope,
		TargetRoot:      pruneReport.TargetRoot,
		ProfileID:       pruneReport.ProfileID,
		TargetID:        pruneReport.TargetID,
		SessionFilter:   pruneReport.SessionFilter,
		LatestSessionID: pruneReport.LatestSessionID,
		Status:          string(pruneReport.PruneReview.Status),
		ReviewRequired:  pruneReport.PruneReview.NeedsReview(),
		Action:          pruneReport.PruneReview.ReviewAction(),
		ReadOnly:        true,
		Authorization: pruneReviewAuthorization{
			ApprovalBypass:  false,
			ApprovalWriting: "not_performed",
			ReceiptWriting:  "not_performed",
			PhysicalPruning: "not_applied",
			TargetDeletion:  "not_applied",
			ApplyRequires:   "prune --apply --approval <id>",
		},
		PruneReview:      pruneReport.PruneReview,
		ArtifactProblems: append([]report.ArtifactProblem(nil), pruneReport.ArtifactProblems...),
	}
}

func printPruneReleaseReviewText(w io.Writer, result pruneReleaseReview) {
	fmt.Fprintf(w, "prune release review: schema=%s scope=%s target=%s profile=%s target_id=%s session_filter=%s latest_session=%s status=%s review_required=%t action=%s read_only=%t approval_bypass=%t approval_writing=%s physical_pruning=%s receipt_writing=%s target_deletion=%s apply_requires=%s\n",
		encodeTextValue(result.Schema),
		encodeTextValue(result.Scope),
		encodeTextValue(result.TargetRoot),
		encodeTextValue(result.ProfileID),
		encodeTextValue(result.TargetID),
		encodeTextValue(result.SessionFilter),
		encodeTextValue(result.LatestSessionID),
		encodeTextValue(result.Status),
		result.ReviewRequired,
		encodeTextValue(result.Action),
		result.ReadOnly,
		result.Authorization.ApprovalBypass,
		encodeTextValue(result.Authorization.ApprovalWriting),
		encodeTextValue(result.Authorization.PhysicalPruning),
		encodeTextValue(result.Authorization.ReceiptWriting),
		encodeTextValue(result.Authorization.TargetDeletion),
		encodeTextValue(result.Authorization.ApplyRequires),
	)
	printReportPruneReviewText(w, result.PruneReview)
	for _, problem := range result.ArtifactProblems {
		fmt.Fprintf(w, "prune_artifact_problem source=%s session=%s path=%s error=%s\n",
			encodeTextValue(problem.Source),
			encodeTextValue(problem.SessionID),
			encodeTextValue(problem.Path),
			encodeTextValue(problem.Error),
		)
	}
}

func printVerifyText(w io.Writer, report verify.Report) {
	fmt.Fprintf(w, "verify: target=%s session=%s manifests=%d files=%d/%d errors=%d warnings=%d soft_deletes=%d target_drifts=%d artifact_problems=%d\n",
		report.TargetRoot,
		report.Manifest.SessionID,
		report.Summary.ManifestCount,
		report.Summary.FilesVerified,
		report.Summary.FilesExpected,
		report.Summary.ErrorFindings,
		report.Summary.WarningFindings+report.Summary.Warnings,
		report.Summary.SoftDeletes,
		report.Summary.TargetDrifts,
		report.Summary.ArtifactProblems,
	)
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "%s %s path=%s target=%s message=%s\n", finding.Severity, finding.Kind, finding.Path, finding.TargetPath, finding.Message)
	}
	for _, problem := range report.ArtifactProblems {
		fmt.Fprintf(w, "error artifact_problem path=%s message=%s\n", problem.Path, problem.Err)
	}
}

func printDriftText(w io.Writer, report verify.DriftReport) {
	fmt.Fprintf(w, "drift: target=%s session=%s manifests=%d entries=%d target_drifts=%d artifact_problems=%d\n",
		encodeTextValue(report.TargetRoot),
		encodeTextValue(report.SessionID),
		report.Summary.ManifestCount,
		report.Summary.ManifestEntries,
		report.Summary.TargetDrifts,
		report.Summary.ArtifactProblems,
	)
	for _, drift := range report.Drifts {
		fmt.Fprintf(w, "target_drift id=%s session=%s profile=%s target_id=%s root=%s path=%s change=%s expected_kind=%s observed_kind=%s detected_at=%s review_state=%s durable=false acknowledgeable=false source=live_detector evidence=%s\n",
			encodeTextValue(drift.ID),
			encodeTextValue(drift.SessionID),
			encodeTextValue(drift.ProfileID),
			encodeTextValue(drift.TargetID),
			encodeTextValue(drift.RootID),
			encodeTextValue(drift.Path),
			encodeTextValue(drift.Change),
			encodeTextValue(drift.Expected.Kind),
			encodeTextValue(drift.Observed.Kind),
			encodeTextValue(drift.DetectedAt),
			encodeTextValue(drift.ReviewState),
			encodeTextValue(strings.Join(drift.Evidence, ",")),
		)
	}
	for _, problem := range report.ArtifactProblems {
		fmt.Fprintf(w, "artifact_problem session=%s path=%s error=%s\n",
			encodeTextValue(problem.SessionID),
			encodeTextValue(problem.Path),
			encodeTextValue(problem.Err),
		)
	}
}

func printDriftRecordText(w io.Writer, result driftreview.RecordResult) {
	fmt.Fprintf(w, "drift_record target=%s session=%s manifests=%d detected=%d recorded=%d existing=%d reopened=%d artifact_problems=%d action=record_only repair=not_applied resolve=not_applied prune=not_authorized\n",
		encodeTextValue(result.TargetRoot),
		encodeTextValue(result.SessionID),
		result.ManifestCount,
		result.Detected,
		result.Recorded,
		result.Existing,
		result.Reopened,
		len(result.ArtifactProblems),
	)
	for _, record := range result.Records {
		fmt.Fprintf(w, "drift_record_item id=%s session=%s path=%s change=%s review_state=%s recorded=%t existing=%t reopened=%t\n",
			encodeTextValue(record.ID),
			encodeTextValue(record.SessionID),
			encodeTextValue(record.Path),
			encodeTextValue(record.Change),
			encodeTextValue(record.ReviewState),
			record.Recorded,
			record.Existing,
			record.Reopened,
		)
	}
	for _, problem := range result.ArtifactProblems {
		fmt.Fprintf(w, "drift_record_artifact_problem session=%s path=%s error=%s\n",
			encodeTextValue(problem.SessionID),
			encodeTextValue(problem.Path),
			encodeTextValue(problem.Err),
		)
	}
}

func printDriftAcknowledgeText(w io.Writer, result driftreview.AcknowledgeResult) {
	fmt.Fprintf(w, "drift_acknowledge id=%s path=%s previous_state=%s review_state=%s reviewed_at=%s reviewer=%s reason=%s profile_id=%s target_id=%s session_id=%s\n",
		encodeTextValue(result.ID),
		encodeTextValue(result.Path),
		encodeTextValue(result.PreviousState),
		encodeTextValue(result.ReviewState),
		encodeTextValue(result.ReviewedAt),
		encodeTextValue(result.Reviewer),
		encodeTextValue(result.Reason),
		encodeTextValue(result.ProfileID),
		encodeTextValue(result.TargetID),
		encodeTextValue(result.SessionID),
	)
}

func printDriftResolveText(w io.Writer, result driftreview.ResolveResult) {
	fmt.Fprintf(w, "drift_resolve id=%s path=%s previous_state=%s review_state=%s reviewed_at=%s reviewer=%s reason=%s profile_id=%s target_id=%s session_id=%s repair=%s manifest_rewrite=%s prune=%s\n",
		encodeTextValue(result.ID),
		encodeTextValue(result.Path),
		encodeTextValue(result.PreviousState),
		encodeTextValue(result.ReviewState),
		encodeTextValue(result.ReviewedAt),
		encodeTextValue(result.Reviewer),
		encodeTextValue(result.Reason),
		encodeTextValue(result.ProfileID),
		encodeTextValue(result.TargetID),
		encodeTextValue(result.SessionID),
		encodeTextValue(result.Repair),
		encodeTextValue(result.ManifestRewrite),
		encodeTextValue(result.Prune),
	)
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
	fmt.Fprintf(w, "health: target=%s healthy=%t incomplete_sessions=%d invalid_records=%d artifact_problems=%d target_drifts=%d network_transfers=%d\n",
		encodeTextValue(report.TargetRoot),
		report.Healthy,
		report.Summary.IncompleteSessions,
		report.Summary.InvalidRecords,
		report.Summary.ArtifactProblems,
		report.Summary.TargetDrifts,
		report.Summary.NetworkTransfers,
	)
	for _, item := range report.Items {
		fmt.Fprintf(w, "%s state=%s action=%s reason=%s path=%s\n",
			encodeTextValue(item.SessionID),
			encodeTextValue(item.State),
			encodeTextValue(item.Action),
			encodeTextValue(item.Reason),
			encodeTextValue(item.Path),
		)
	}
	for _, invalid := range report.Invalid {
		fmt.Fprintf(w, "invalid session=%s path=%s error=%s\n",
			encodeTextValue(invalid.SessionID),
			encodeTextValue(invalid.Path),
			encodeTextValue(invalid.Error),
		)
	}
	for _, artifact := range report.Artifacts {
		fmt.Fprintf(w, "artifact source=%s session=%s path=%s error=%s\n",
			encodeTextValue(artifact.Source),
			encodeTextValue(artifact.SessionID),
			encodeTextValue(artifact.Path),
			encodeTextValue(artifact.Error),
		)
	}
	for _, drift := range report.TargetDrifts {
		fmt.Fprintf(w, "target_drift session=%s path=%s change=%s detected_at=%s evidence=%s\n",
			encodeTextValue(drift.SessionID),
			encodeTextValue(drift.Path),
			encodeTextValue(drift.Change),
			encodeTextValue(drift.DetectedAt),
			encodeTextValue(strings.Join(drift.Evidence, ",")),
		)
	}
	for _, transfer := range report.Transfers {
		fmt.Fprintf(w, "network_transfer session=%s status=%s stage=%s action=%s error_code=%s error=%s path=%s\n",
			encodeTextValue(transfer.SessionID),
			encodeTextValue(transfer.Status),
			encodeTextValue(transfer.Stage),
			encodeTextValue(transfer.Action),
			encodeTextValue(transfer.ErrorCode),
			encodeTextValue(transfer.Error),
			encodeTextValue(transfer.Path),
		)
	}
}

func printReportText(w io.Writer, report report.Report) {
	fmt.Fprintf(w, "report: target=%s status=%s session=%s manifests=%d files=%d/%d verification_errors=%d verification_warnings=%d warnings=%d profile_suggestions=%d soft_deletes=%d prune_candidates=%d prune_refusals=%d prune_approvals=%d prune_unapplied_approvals=%d prune_active_approvals=%d prune_stale_approvals=%d prune_expired_approvals=%d prune_consumed_approvals=%d prune_receipts=%d prune_receipt_issues=%d prune_artifact_problems=%d target_drifts=%d live_target_drifts=%d live_target_drift_artifact_problems=%d recovery_issues=%d invalid_records=%d artifact_problems=%d pairing_issues=%d scope=%s\n",
		encodeTextValue(report.TargetRoot),
		report.Overall.Status,
		encodeTextValue(report.LatestSession.ID),
		report.Summary.ManifestCount,
		report.Summary.FilesVerified,
		report.Summary.FilesExpected,
		report.Summary.VerificationErrors,
		report.Summary.VerificationWarnings,
		report.Summary.Warnings,
		report.Summary.ProfileSuggestions,
		report.Summary.SoftDeletes,
		report.Summary.PruneCandidates,
		report.Summary.PruneRefusals,
		report.Summary.PruneApprovals,
		report.Summary.PruneUnappliedApprovals,
		report.Summary.PruneActiveApprovals,
		report.Summary.PruneStaleApprovals,
		report.Summary.PruneExpiredApprovals,
		report.Summary.PruneConsumedApprovals,
		report.Summary.PruneReceipts,
		report.Summary.PruneReceiptIssues,
		report.Summary.PruneArtifactProblems,
		report.Summary.TargetDrifts,
		report.Summary.LiveTargetDrifts,
		report.Summary.LiveTargetDriftProblems,
		report.Summary.RecoveryIssues,
		report.Summary.InvalidHealthRecords,
		report.Summary.ArtifactProblems,
		report.Summary.PairingIssues,
		encodeTextValue(report.Scope),
	)
	if len(report.Overall.Issues) > 0 {
		fmt.Fprintf(w, "issues=%s\n", strings.Join(report.Overall.Issues, ","))
	}
	fmt.Fprintf(w, "pairing status=%s receipt=%s target_device=%s method=%s verified_at=%s evidence=%s encrypted_transfer=%s\n",
		encodeTextValue(string(report.Pairing.Status)),
		encodeTextValue(report.Pairing.ReceiptID),
		encodeTextValue(report.Pairing.TargetDeviceID),
		encodeTextValue(report.Pairing.Method),
		encodeTextValue(report.Pairing.VerifiedAt),
		encodeTextValue(report.Pairing.Evidence),
		encodeTextValue(report.Pairing.EncryptedTransfer),
	)
	fmt.Fprintf(w, "privacy status=%s mode=%s traffic_level=%d padding_bucket_bytes=%d batch_max_bytes=%d batch_max_count=%d jitter_budget_millis=%d discovery_low_info=%t claim=%s configured_reductions=%s overhead_status=%s overhead_source=%s overhead_padding_bucket_bytes=%d overhead_batch_max_bytes=%d overhead_batch_max_count=%d overhead_jitter_budget_millis=%d residual_leakage=%s local_push=%s network_transfer=%s\n",
		report.Privacy.Status,
		report.Privacy.Mode,
		report.Privacy.TrafficLevel,
		report.Privacy.PaddingBucketBytes,
		report.Privacy.BatchMaxBytes,
		report.Privacy.BatchMaxCount,
		report.Privacy.JitterBudgetMillis,
		report.Privacy.DiscoveryLowInfo,
		report.Privacy.Claim,
		formatStringList(report.Privacy.ConfiguredReduction),
		report.Privacy.Overhead.Status,
		report.Privacy.Overhead.Source,
		report.Privacy.Overhead.PaddingBucketBytes,
		report.Privacy.Overhead.BatchMaxBytes,
		report.Privacy.Overhead.BatchMaxCount,
		report.Privacy.Overhead.JitterBudgetMillis,
		formatStringList(report.Privacy.ResidualLeakage),
		report.Privacy.LocalPush,
		report.Privacy.NetworkTransfer,
	)
	printReportTrafficPrivacyAcceptanceText(w, report.TrafficPrivacy)
	for _, snapshot := range report.ProfileSnapshots {
		fmt.Fprintf(w, "profile_snapshot id=%s session=%s profile=%s captured_at=%s path=%s privacy_status=%s privacy_mode=%s privacy_traffic_level=%d privacy_padding_bucket_bytes=%d privacy_batch_max_bytes=%d privacy_batch_max_count=%d privacy_jitter_budget_millis=%d privacy_discovery_low_info=%t privacy_claim=%s privacy_configured_reductions=%s privacy_overhead_status=%s privacy_overhead_source=%s privacy_overhead_padding_bucket_bytes=%d privacy_overhead_batch_max_bytes=%d privacy_overhead_batch_max_count=%d privacy_overhead_jitter_budget_millis=%d privacy_local_push=%s privacy_network_transfer=%s\n",
			encodeTextValue(snapshot.ID),
			encodeTextValue(snapshot.SessionID),
			encodeTextValue(snapshot.ProfileID),
			encodeTextValue(snapshot.CapturedAt),
			encodeTextValue(snapshot.Path),
			encodeTextValue(snapshot.Privacy.Status),
			encodeTextValue(snapshot.Privacy.Mode),
			snapshot.Privacy.TrafficLevel,
			snapshot.Privacy.PaddingBucketBytes,
			snapshot.Privacy.BatchMaxBytes,
			snapshot.Privacy.BatchMaxCount,
			snapshot.Privacy.JitterBudgetMillis,
			snapshot.Privacy.DiscoveryLowInfo,
			encodeTextValue(snapshot.Privacy.Claim),
			encodeTextList(snapshot.Privacy.ConfiguredReduction),
			encodeTextValue(snapshot.Privacy.Overhead.Status),
			encodeTextValue(snapshot.Privacy.Overhead.Source),
			snapshot.Privacy.Overhead.PaddingBucketBytes,
			snapshot.Privacy.Overhead.BatchMaxBytes,
			snapshot.Privacy.Overhead.BatchMaxCount,
			snapshot.Privacy.Overhead.JitterBudgetMillis,
			encodeTextValue(snapshot.Privacy.LocalPush),
			encodeTextValue(snapshot.Privacy.NetworkTransfer),
		)
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(w, "warning id=%s session=%s severity=%s code=%s paths=%s target=%s message=%s\n",
			encodeTextValue(warning.ID),
			encodeTextValue(warning.SessionID),
			encodeTextValue(warning.Severity),
			encodeTextValue(warning.Code),
			encodeTextList(warning.Paths),
			encodeTextValue(warning.TargetPath),
			encodeTextValue(warning.Message),
		)
	}
	for _, suggestion := range report.ProfileSuggestions {
		fmt.Fprintf(w, "profile_suggestion warning=%s code=%s paths=%s target=%s patch=%s config=%s message=%s\n",
			encodeTextValue(suggestion.WarningID),
			encodeTextValue(suggestion.Code),
			encodeTextList(suggestion.Paths),
			encodeTextValue(suggestion.TargetPath),
			encodeTextMap(suggestion.SuggestedProfilePatch),
			encodeTextMap(suggestion.SuggestedConfig),
			encodeTextValue(suggestion.Message),
		)
	}
	for _, record := range report.SoftDeletes {
		fmt.Fprintf(w, "soft_delete id=%s session=%s profile=%s target_id=%s root=%s previous_session=%s previous_manifest=%s source=%s target=%s kind=%s size=%d digest=%s detected_at=%s reason=%s\n",
			encodeTextValue(record.ID),
			encodeTextValue(record.SessionID),
			encodeTextValue(record.ProfileID),
			encodeTextValue(record.TargetID),
			encodeTextValue(record.RootID),
			encodeTextValue(record.PreviousSessionID),
			encodeTextValue(record.PreviousManifestID),
			encodeTextValue(record.SourcePath),
			encodeTextValue(record.TargetPath),
			encodeTextValue(record.Kind),
			record.Size,
			encodeTextValue(record.Digest),
			encodeTextValue(record.DetectedAt),
			encodeTextValue(record.Reason),
		)
	}
	printReportPruneReviewText(w, report.PruneReview)
	for _, drift := range report.TargetDrifts {
		fmt.Fprintf(w, "target_drift id=%s session=%s path=%s change=%s detected_at=%s evidence=%s\n",
			encodeTextValue(drift.ID),
			encodeTextValue(drift.SessionID),
			encodeTextValue(drift.Path),
			encodeTextValue(drift.Change),
			encodeTextValue(drift.DetectedAt),
			encodeTextValue(strings.Join(drift.Evidence, ",")),
		)
	}
	fmt.Fprintf(w, "live_target_drift source=%s durable=%t session=%s manifests=%d entries=%d target_drifts=%d artifact_problems=%d\n",
		encodeTextValue(report.LiveTargetDrift.Source),
		report.LiveTargetDrift.Durable,
		encodeTextValue(report.LiveTargetDrift.SessionID),
		report.LiveTargetDrift.Summary.ManifestCount,
		report.LiveTargetDrift.Summary.ManifestEntries,
		report.LiveTargetDrift.Summary.TargetDrifts,
		report.LiveTargetDrift.Summary.ArtifactProblems,
	)
	for _, drift := range report.LiveTargetDrift.TargetDrifts {
		fmt.Fprintf(w, "live_target_drift_item id=%s session=%s profile=%s target_id=%s root=%s path=%s change=%s expected_kind=%s observed_kind=%s detected_at=%s review_state=%s evidence=%s\n",
			encodeTextValue(drift.ID),
			encodeTextValue(drift.SessionID),
			encodeTextValue(drift.ProfileID),
			encodeTextValue(drift.TargetID),
			encodeTextValue(drift.RootID),
			encodeTextValue(drift.Path),
			encodeTextValue(drift.Change),
			encodeTextValue(drift.Expected.Kind),
			encodeTextValue(drift.Observed.Kind),
			encodeTextValue(drift.DetectedAt),
			encodeTextValue(drift.ReviewState),
			encodeTextValue(strings.Join(drift.Evidence, ",")),
		)
	}
	for _, problem := range report.LiveTargetDrift.ArtifactProblems {
		fmt.Fprintf(w, "live_target_drift_artifact_problem session=%s path=%s error=%s\n",
			encodeTextValue(problem.SessionID),
			encodeTextValue(problem.Path),
			encodeTextValue(problem.Error),
		)
	}
	for _, item := range report.Health.RecoveryIssues {
		fmt.Fprintf(w, "recovery session=%s state=%s action=%s reason=%s path=%s\n",
			encodeTextValue(item.SessionID),
			encodeTextValue(item.State),
			encodeTextValue(item.Action),
			encodeTextValue(item.Reason),
			encodeTextValue(item.Path),
		)
	}
	for _, invalid := range report.Health.InvalidRecords {
		fmt.Fprintf(w, "invalid_record session=%s path=%s error=%s\n",
			encodeTextValue(invalid.SessionID),
			encodeTextValue(invalid.Path),
			encodeTextValue(invalid.Error),
		)
	}
	for _, transfer := range report.NetworkTransfers {
		overhead := control.NetworkTransferPrivacyOverhead{}
		if transfer.Overhead != nil {
			overhead = *transfer.Overhead
		}
		fmt.Fprintf(w, "network_transfer session=%s profile=%s target_id=%s status=%s stage=%s action=%s privacy_level=%d privacy_padding_bucket_bytes=%d privacy_batch_max_bytes=%d privacy_batch_max_count=%d privacy_jitter_budget_millis=%d privacy_discovery_low_info=%t privacy_frame_plain_bytes=%d privacy_frame_wire_bytes=%d privacy_padding_bytes=%d privacy_padded_chunks=%d privacy_overhead_padding_bucket_bytes=%d privacy_batch_frames=%d privacy_batched_chunks=%d privacy_max_batch_count=%d privacy_max_batch_plain_bytes=%d privacy_jittered_requests=%d privacy_jitter_delay_millis=%d privacy_max_jitter_delay_millis=%d privacy_overhead_jitter_budget_millis=%d path=%s error_code=%s error=%s\n",
			encodeTextValue(transfer.SessionID),
			encodeTextValue(transfer.ProfileID),
			encodeTextValue(transfer.TargetID),
			encodeTextValue(transfer.Status),
			encodeTextValue(transfer.Stage),
			encodeTextValue(transfer.Action),
			transfer.Privacy.Level,
			transfer.Privacy.PaddingBucket,
			transfer.Privacy.BatchMaxBytes,
			transfer.Privacy.BatchMaxCount,
			transfer.Privacy.JitterBudget,
			transfer.Privacy.DiscoveryLowInfo,
			overhead.FramePlainBytes,
			overhead.FrameWireBytes,
			overhead.PaddingBytes,
			overhead.PaddedChunks,
			overhead.PaddingBucketBytes,
			overhead.BatchFrames,
			overhead.BatchedChunks,
			overhead.MaxBatchCount,
			overhead.MaxBatchPlainBytes,
			overhead.JitteredRequests,
			overhead.JitterDelayMillis,
			overhead.MaxJitterDelayMillis,
			overhead.JitterBudgetMillis,
			encodeTextValue(transfer.Path),
			encodeTextValue(transfer.ErrorCode),
			encodeTextValue(transfer.Error),
		)
	}
	for _, problem := range report.ArtifactProblems {
		fmt.Fprintf(w, "artifact_problem source=%s session=%s path=%s error=%s\n",
			encodeTextValue(problem.Source),
			encodeTextValue(problem.SessionID),
			encodeTextValue(problem.Path),
			encodeTextValue(problem.Error),
		)
	}
	for _, finding := range report.VerificationFindings {
		fmt.Fprintf(w, "verification severity=%s kind=%s path=%s target=%s message=%s\n",
			encodeTextValue(string(finding.Severity)),
			encodeTextValue(string(finding.Kind)),
			encodeTextValue(finding.Path),
			encodeTextValue(finding.TargetPath),
			encodeTextValue(finding.Message),
		)
	}
}

func printReportTrafficPrivacyAcceptanceText(w io.Writer, acceptance report.TrafficPrivacyAcceptance) {
	overhead := control.NetworkTransferPrivacyOverhead{}
	if acceptance.ObservedOverhead != nil {
		overhead = *acceptance.ObservedOverhead
	}
	fmt.Fprintf(w, "traffic_privacy_acceptance status=%s scope=%s claim=%s anonymity_claim=%s evidence_source=%s session=%s blockers=%s configured_reductions=%s residual_leakage=%s padding_bucket_bytes=%d batch_max_bytes=%d batch_max_count=%d jitter_budget_millis=%d discovery_low_info=%t observed_frame_plain_bytes=%d observed_frame_wire_bytes=%d observed_padding_bytes=%d observed_padded_chunks=%d observed_padding_bucket_bytes=%d observed_batch_frames=%d observed_batched_chunks=%d observed_max_batch_count=%d observed_max_batch_plain_bytes=%d observed_jittered_requests=%d observed_jitter_delay_millis=%d observed_max_jitter_delay_millis=%d observed_jitter_budget_millis=%d\n",
		encodeTextValue(acceptance.Status),
		encodeTextValue(acceptance.Scope),
		encodeTextValue(acceptance.Claim),
		encodeTextValue(acceptance.AnonymityClaim),
		encodeTextValue(acceptance.EvidenceSource),
		encodeTextValue(defaultTextField(acceptance.SessionID)),
		encodeTextList(acceptance.Blockers),
		encodeTextList(acceptance.ConfiguredReductions),
		encodeTextList(acceptance.ResidualLeakage),
		acceptance.PaddingBucketBytes,
		acceptance.BatchMaxBytes,
		acceptance.BatchMaxCount,
		acceptance.JitterBudgetMillis,
		acceptance.DiscoveryLowInfo,
		overhead.FramePlainBytes,
		overhead.FrameWireBytes,
		overhead.PaddingBytes,
		overhead.PaddedChunks,
		overhead.PaddingBucketBytes,
		overhead.BatchFrames,
		overhead.BatchedChunks,
		overhead.MaxBatchCount,
		overhead.MaxBatchPlainBytes,
		overhead.JitteredRequests,
		overhead.JitterDelayMillis,
		overhead.MaxJitterDelayMillis,
		overhead.JitterBudgetMillis,
	)
}

func printReportPruneReviewText(w io.Writer, review report.PruneReview) {
	policy := prune.ProfileDeletePolicy{}
	if review.ProfileDeletePolicy != nil {
		policy = *review.ProfileDeletePolicy
	}
	fmt.Fprintf(w, "prune_review status=%s dry_run=%t approval_required=%t approval_authoring=%s physical_pruning=%s apply=%s approval_source=%s receipt_source=%s policy_mode=%s policy_require_review=%t policy_retention_days=%d policy_allow_physical_prune=%t soft_deletes=%d candidates=%d refusals=%d approvals=%d unapplied_approvals=%d active_approvals=%d stale_approvals=%d expired_approvals=%d consumed_approvals=%d receipts=%d receipt_issues=%d artifact_problems=%d\n",
		encodeTextValue(string(review.Status)),
		review.DryRun,
		review.ApprovalRequired,
		encodeTextValue(review.ApprovalAuthoring),
		encodeTextValue(review.PhysicalPruning),
		encodeTextValue(review.Apply),
		encodeTextValue(review.ApprovalSource),
		encodeTextValue(review.ReceiptSource),
		encodeTextValue(policy.Mode),
		policy.RequireReview,
		policy.RetentionDays,
		policy.AllowPhysicalPrune,
		review.Summary.SoftDeletes,
		review.Summary.Candidates,
		review.Summary.Refusals,
		review.Summary.Approvals,
		review.Summary.UnappliedApprovals,
		review.Summary.ActiveApprovals,
		review.Summary.StaleApprovals,
		review.Summary.ExpiredApprovals,
		review.Summary.ConsumedApprovals,
		review.Summary.Receipts,
		review.Summary.ReceiptIssues,
		review.Summary.ArtifactProblems,
	)
	for _, candidate := range review.Candidates {
		fmt.Fprintf(w, "prune_candidate soft_delete=%s session=%s profile=%s target_id=%s root=%s previous_session=%s previous_manifest=%s source=%s target=%s kind=%s size=%d digest=%s symlink_target=%s detected_at=%s previous_source=%s previous_target=%s previous_kind=%s previous_size=%d previous_digest=%s previous_symlink_target=%s observed_present=%t observed_path=%s observed_kind=%s observed_size=%d observed_digest=%s observed_symlink_target=%s action=%s physical_pruning=%s approval_writing=%s receipt_writing=%s review_required=%t\n",
			encodeTextValue(candidate.SoftDeleteID),
			encodeTextValue(candidate.DetectedSessionID),
			encodeTextValue(candidate.ProfileID),
			encodeTextValue(candidate.TargetID),
			encodeTextValue(candidate.RootID),
			encodeTextValue(candidate.PreviousSessionID),
			encodeTextValue(candidate.PreviousManifestID),
			encodeTextValue(candidate.SourcePath),
			encodeTextValue(candidate.TargetPath),
			encodeTextValue(candidate.Kind),
			candidate.Size,
			encodeTextValue(candidate.Digest),
			encodeTextValue(candidate.PreviousManifestEntry.SymlinkTarget),
			encodeTextValue(candidate.DetectedAt),
			encodeTextValue(candidate.PreviousManifestEntry.SourcePath),
			encodeTextValue(candidate.PreviousManifestEntry.TargetPath),
			encodeTextValue(candidate.PreviousManifestEntry.Kind),
			candidate.PreviousManifestEntry.Size,
			encodeTextValue(candidate.PreviousManifestEntry.Digest),
			encodeTextValue(candidate.PreviousManifestEntry.SymlinkTarget),
			boolValue(candidate.CurrentTargetState.Present),
			encodeTextValue(candidate.CurrentTargetState.Path),
			encodeTextValue(candidate.CurrentTargetState.Kind),
			candidate.CurrentTargetState.Size,
			encodeTextValue(candidate.CurrentTargetState.Digest),
			encodeTextValue(candidate.CurrentTargetState.SymlinkTarget),
			encodeTextValue(candidate.IntendedAction),
			encodeTextValue(candidate.PhysicalPruning),
			encodeTextValue(candidate.ApprovalWriting),
			encodeTextValue(candidate.ReceiptWriting),
			candidate.ReviewRequired,
		)
	}
	for _, refusal := range review.Refusals {
		fmt.Fprintf(w, "prune_refusal soft_delete=%s session=%s source=%s target=%s reason=%s message=%s observed_present=%t observed_path=%s observed_kind=%s observed_size=%d observed_digest=%s observed_symlink_target=%s\n",
			encodeTextValue(refusal.SoftDeleteID),
			encodeTextValue(refusal.DetectedSessionID),
			encodeTextValue(refusal.SourcePath),
			encodeTextValue(refusal.TargetPath),
			encodeTextValue(refusal.ReasonCode),
			encodeTextValue(refusal.Message),
			boolValue(refusal.CurrentTargetState.Present),
			encodeTextValue(refusal.CurrentTargetState.Path),
			encodeTextValue(refusal.CurrentTargetState.Kind),
			refusal.CurrentTargetState.Size,
			encodeTextValue(refusal.CurrentTargetState.Digest),
			encodeTextValue(refusal.CurrentTargetState.SymlinkTarget),
		)
	}
	for _, approval := range review.Approvals {
		fmt.Fprintf(w, "prune_approval id=%s profile=%s target_id=%s root=%s status=%s items=%d unapplied=%t release_state=%s release_blocker=%t release_reason=%s release_action=%s linked_receipt=%s linked_receipt_status=%s path=%s action=%s physical_pruning=%s created_at=%s approved_by=%s approved_at=%s superseded_by=%s superseded_at=%s expires_at=%s review_tool=%s profile_snapshot=%s profile_snapshot_path=%s profile_snapshot_digest=%s approval_scope_digest=%s approval_reason=%s refusal_reason=%s policy_mode=%s policy_require_review=%t policy_retention_days=%d policy_allow_physical_prune=%t\n",
			encodeTextValue(approval.ID),
			encodeTextValue(approval.ProfileID),
			encodeTextValue(approval.TargetID),
			encodeTextValue(approval.RootID),
			encodeTextValue(approval.Status),
			len(approval.Items),
			approval.Unapplied,
			encodeTextValue(defaultTextField(approval.ReleaseState)),
			approval.ReleaseBlocker,
			encodeTextValue(defaultTextField(approval.ReleaseReason)),
			encodeTextValue(defaultTextField(approval.ReleaseAction)),
			encodeTextValue(defaultTextField(approval.LinkedReceiptID)),
			encodeTextValue(defaultTextField(string(approval.LinkedReceiptStatus))),
			encodeTextValue(approval.Path),
			encodeTextValue(approval.Action),
			encodeTextValue(approval.PhysicalPruning),
			encodeTextValue(approval.CreatedAt),
			encodeTextValue(defaultTextField(approval.ApprovedBy)),
			encodeTextValue(defaultTextField(approval.ApprovedAt)),
			encodeTextValue(defaultTextField(approval.SupersededBy)),
			encodeTextValue(defaultTextField(approval.SupersededAt)),
			encodeTextValue(defaultTextField(approval.ExpiresAt)),
			encodeTextValue(approval.ReviewTool),
			encodeTextValue(defaultTextField(approval.ProfileSnapshotID)),
			encodeTextValue(defaultTextField(approval.ProfileSnapshotPath)),
			encodeTextValue(defaultTextField(approval.ProfileSnapshotDigest)),
			encodeTextValue(defaultTextField(approval.ApprovalScopeDigest)),
			encodeTextValue(defaultTextField(approval.ApprovalReason)),
			encodeTextValue(defaultTextField(approval.RefusalReason)),
			encodeTextValue(approval.ProfileDeletePolicy.Mode),
			approval.ProfileDeletePolicy.RequireReview,
			approval.ProfileDeletePolicy.RetentionDays,
			approval.ProfileDeletePolicy.AllowPhysicalPrune,
		)
		for _, evidence := range approval.CurrentEvidence {
			fmt.Fprintf(w, "prune_approval_current_evidence approval=%s soft_delete=%s target=%s state=%s reason_code=%s reason=%s\n",
				encodeTextValue(approval.ID),
				encodeTextValue(evidence.SoftDeleteID),
				encodeTextValue(evidence.TargetPath),
				encodeTextValue(evidence.State),
				encodeTextValue(defaultTextField(evidence.ReasonCode)),
				encodeTextValue(defaultTextField(evidence.Reason)),
			)
		}
		for _, item := range approval.Items {
			fmt.Fprintf(w, "prune_approval_item approval=%s soft_delete=%s session=%s previous_session=%s previous_manifest=%s root=%s source=%s target=%s kind=%s size=%d digest=%s symlink_target=%s detected_at=%s soft_delete_ref=%s action=%s physical_pruning=%s\n",
				encodeTextValue(approval.ID),
				encodeTextValue(item.SoftDeleteID),
				encodeTextValue(item.DetectedSessionID),
				encodeTextValue(item.PreviousSessionID),
				encodeTextValue(item.PreviousManifestID),
				encodeTextValue(item.RootID),
				encodeTextValue(item.SourcePath),
				encodeTextValue(item.TargetPath),
				encodeTextValue(item.Kind),
				item.Size,
				encodeTextValue(defaultTextField(item.Digest)),
				encodeTextValue(defaultTextField(item.SymlinkTarget)),
				encodeTextValue(item.DetectedAt),
				encodeTextValue(item.SoftDeleteRef),
				encodeTextValue(approval.Action),
				encodeTextValue(approval.PhysicalPruning),
			)
		}
	}
	for _, receipt := range review.Receipts {
		fmt.Fprintf(w, "prune_receipt id=%s prune_session=%s approval=%s profile=%s target_id=%s status=%s dry_run=%t items=%d refusals=%d action=%s started_at=%s ended_at=%s path=%s approval_scope_digest=%s\n",
			encodeTextValue(receipt.ID),
			encodeTextValue(receipt.PruneSessionID),
			encodeTextValue(receipt.ApprovalID),
			encodeTextValue(receipt.ProfileID),
			encodeTextValue(receipt.TargetID),
			encodeTextValue(string(receipt.Status)),
			receipt.DryRun,
			len(receipt.Items),
			len(receipt.Refusals),
			encodeTextValue(receipt.Action),
			encodeTextValue(receipt.StartedAt),
			encodeTextValue(receipt.EndedAt),
			encodeTextValue(receipt.Path),
			encodeTextValue(receipt.ApprovalScopeDigest),
		)
		for _, item := range receipt.Items {
			fmt.Fprintf(w, "prune_receipt_item receipt=%s soft_delete=%s target=%s action=%s result=%s error_code=%s error=%s pruned_at=%s observed_present=%t observed_path=%s observed_kind=%s observed_size=%d observed_digest=%s observed_symlink_target=%s\n",
				encodeTextValue(receipt.ID),
				encodeTextValue(item.SoftDeleteID),
				encodeTextValue(item.TargetPath),
				encodeTextValue(item.IntendedAction),
				encodeTextValue(item.Result),
				encodeTextValue(item.ErrorCode),
				encodeTextValue(item.Error),
				encodeTextValue(item.PrunedAt),
				boolValue(item.PrePruneObserved.Present),
				encodeTextValue(item.PrePruneObserved.Path),
				encodeTextValue(item.PrePruneObserved.Kind),
				item.PrePruneObserved.Size,
				encodeTextValue(item.PrePruneObserved.Digest),
				encodeTextValue(item.PrePruneObserved.SymlinkTarget),
			)
		}
		for _, refusal := range receipt.Refusals {
			fmt.Fprintf(w, "prune_receipt_refusal receipt=%s soft_delete=%s target=%s reason=%s message=%s\n",
				encodeTextValue(receipt.ID),
				encodeTextValue(refusal.SoftDeleteID),
				encodeTextValue(refusal.TargetPath),
				encodeTextValue(refusal.ReasonCode),
				encodeTextValue(refusal.Message),
			)
		}
	}
}

func printStatusText(w io.Writer, report status.Report) {
	fmt.Fprintf(w, "status: target=%s profile_id=%s target_id=%s status=%s target_status=%s review_required=%t latest_session=%s completeness_status=%s manifests=%d files=%d/%d verification_errors=%d verification_warnings=%d warnings=%d profile_suggestions=%d soft_deletes=%d prune_review_status=%s prune_review_action=%s prune_approvals=%d prune_unapplied_approvals=%d prune_active_approvals=%d prune_stale_approvals=%d prune_expired_approvals=%d prune_consumed_approvals=%d prune_receipts=%d prune_receipt_issues=%d target_drifts=%d live_target_drifts=%d live_target_drift_artifact_problems=%d recovery_issues=%d invalid_health_records=%d artifact_problems=%d artifact_problem_sources=%s pairing_issues=%d network_transfers=%d\n",
		encodeTextValue(report.TargetRoot),
		encodeTextValue(report.ProfileID),
		encodeTextValue(report.TargetID),
		encodeTextValue(report.Overall.Status),
		encodeTextValue(report.Overall.TargetStatus),
		report.ReviewRequired,
		encodeTextValue(statusTextValueOrDash(report.LatestSession.ID)),
		encodeTextValue(report.LatestSession.CompletenessStatus),
		report.Counts.ManifestCount,
		report.Counts.FilesVerified,
		report.Counts.FilesExpected,
		report.Counts.VerificationErrors,
		report.Counts.VerificationWarnings,
		report.Counts.Warnings,
		report.Counts.ProfileSuggestions,
		report.Counts.SoftDeletes,
		encodeTextValue(report.PruneReview.Status),
		encodeTextValue(report.PruneReview.Action),
		report.Counts.PruneApprovals,
		report.Counts.PruneUnappliedApprovals,
		report.Counts.PruneActiveApprovals,
		report.Counts.PruneStaleApprovals,
		report.Counts.PruneExpiredApprovals,
		report.Counts.PruneConsumedApprovals,
		report.Counts.PruneReceipts,
		report.Counts.PruneReceiptIssues,
		report.Counts.TargetDrifts,
		report.Counts.LiveTargetDrifts,
		report.Counts.LiveTargetDriftProblems,
		report.Counts.RecoveryIssues,
		report.Counts.InvalidHealthRecords,
		report.Counts.ArtifactProblems,
		formatStatusArtifactProblemSources(report.Counts.ArtifactProblemSources),
		report.Counts.PairingIssues,
		report.Counts.NetworkTransfers,
	)
	fmt.Fprintf(w, "pairing status=%s encrypted_transfer=%s\n",
		encodeTextValue(report.Pairing.Status),
		encodeTextValue(report.Pairing.EncryptedTransfer),
	)
	fmt.Fprintf(w, "privacy network_transfer=%s local_push=%s\n",
		encodeTextValue(report.Privacy.NetworkTransfer),
		encodeTextValue(report.Privacy.LocalPush),
	)
	printStatusTrafficPrivacyAcceptanceText(w, report.TrafficPrivacy)
	fmt.Fprintf(w, "network evidence_status=%s artifact_problems=%d\n",
		encodeTextValue(report.Network.Status),
		report.Network.ArtifactProblems,
	)
	for _, transfer := range report.Network.Transfers {
		fmt.Fprintf(w, "network_transfer session=%s status=%s stage=%s action=%s error_code=%s error=%s\n",
			encodeTextValue(transfer.SessionID),
			encodeTextValue(transfer.Status),
			encodeTextValue(transfer.Stage),
			encodeTextValue(transfer.Action),
			encodeTextValue(transfer.ErrorCode),
			encodeTextValue(transfer.Error),
		)
	}
}

func printStatusTrafficPrivacyAcceptanceText(w io.Writer, acceptance status.TrafficPrivacyAcceptance) {
	overhead := control.NetworkTransferPrivacyOverhead{}
	if acceptance.ObservedOverhead != nil {
		overhead = *acceptance.ObservedOverhead
	}
	fmt.Fprintf(w, "traffic_privacy_acceptance status=%s scope=%s claim=%s anonymity_claim=%s evidence_source=%s session=%s blockers=%s configured_reductions=%s residual_leakage=%s padding_bucket_bytes=%d batch_max_bytes=%d batch_max_count=%d jitter_budget_millis=%d discovery_low_info=%t observed_padding_bytes=%d observed_padded_chunks=%d observed_batch_frames=%d observed_batched_chunks=%d observed_jittered_requests=%d observed_jitter_budget_millis=%d\n",
		encodeTextValue(acceptance.Status),
		encodeTextValue(acceptance.Scope),
		encodeTextValue(acceptance.Claim),
		encodeTextValue(acceptance.AnonymityClaim),
		encodeTextValue(acceptance.EvidenceSource),
		encodeTextValue(defaultTextField(acceptance.SessionID)),
		encodeTextList(acceptance.Blockers),
		encodeTextList(acceptance.ConfiguredReductions),
		encodeTextList(acceptance.ResidualLeakage),
		acceptance.PaddingBucketBytes,
		acceptance.BatchMaxBytes,
		acceptance.BatchMaxCount,
		acceptance.JitterBudgetMillis,
		acceptance.DiscoveryLowInfo,
		overhead.PaddingBytes,
		overhead.PaddedChunks,
		overhead.BatchFrames,
		overhead.BatchedChunks,
		overhead.JitteredRequests,
		overhead.JitterBudgetMillis,
	)
}

func formatStatusArtifactProblemSources(sources []status.ArtifactProblemSourceCount) string {
	if len(sources) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		parts = append(parts, fmt.Sprintf("%s:%d", encodeTextValue(source.Source), source.Count))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func statusTextValueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func printDiscoveryText(w io.Writer, hints []discovery.AddressHint) {
	fmt.Fprintf(w, "discover: hints=%d trusted=false\n", len(hints))
	for _, hint := range hints {
		fmt.Fprintf(w, "hint address=%s service=%s protocol=%s nonce=%s caps=%s trusted=false expires_at=%s\n",
			hint.Address,
			hint.Advertisement.ServiceType,
			hint.Advertisement.ProtocolVersion,
			hint.Advertisement.EphemeralNonce,
			strings.Join(sortedStrings(hint.Advertisement.CapabilityFlags), ","),
			hint.ExpiresAt.Format(time.RFC3339Nano),
		)
	}
}

func formatStringList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(sortedStrings(values), ",")
}

func sortedStrings(values []string) []string {
	copied := append([]string(nil), values...)
	sort.Strings(copied)
	return copied
}

type multiFlag []string

func (m *multiFlag) String() string {
	if m == nil {
		return ""
	}
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("value is required")
	}
	*m = append(*m, value)
	return nil
}

func deterministicDiscoveryNonce(now time.Time, addresses []string) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	values := append([]string(nil), addresses...)
	sort.Strings(values)
	seed := now.UTC().UnixNano()
	for _, value := range values {
		for _, r := range value {
			seed = seed*33 + int64(r)
		}
	}
	if seed < 0 {
		seed = -seed
	}
	return "n" + strconv.FormatInt(seed, 36)
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

func printDaemonUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  supermover daemon install --profile <path>
  supermover daemon run --foreground --profile <path> [--listen <host:port>]
  supermover daemon status --profile <path> [--format text|json]
  supermover daemon logs --profile <path> [--tail <n>] [--format text|json]
  supermover daemon restart --profile <path> [--reason <text>] [--format text|json]
  supermover daemon stop --profile <path> [--reason <text>]

Manages foreground agent lifecycle evidence under the target .supermover control
plane, including scoped status, stop/restart intents, and redacted lifecycle
events. This wraps the existing profile-backed serve behavior and keeps the
profile as the runtime SSOT. It does not install an OS service manager, manage
detached background processes, browse the LAN, or run ongoing incremental sync.`)
}

func printSyncUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  supermover sync queue <enqueue|status|ready|cancel> [flags]

Sync commands expose durable changed-file queue evidence only. They do not run
a watcher, background daemon, transport loop, or ongoing sync executor.`)
}

func printSyncQueueUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage of sync queue:
  supermover sync queue enqueue --profile <path> [--format text|json]
  supermover sync queue status --profile <path> [--format text|json]
  supermover sync queue ready --profile <path> [--format text|json]
  supermover sync queue cancel --profile <path> --id <entry-id> --reason <text> [--format text|json]

Manages durable changed-file queue evidence only under the profile-selected
target. The profile remains the SSOT. These commands do not watch roots, copy
files, run a daemon, or perform ongoing sync.`)
}

func printDeletedUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  supermover deleted list --profile <path>`)
}

func printPruneUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage of prune:
  supermover prune --profile <path> [--dry-run|--apply --approval <id>] [--format text|json]
  supermover prune approvals --profile <path> [--format text|json]
  supermover prune review --profile <path> [--session <id>] [--format text|json]
  supermover prune approve --profile <path> --id <approval-id> --soft-delete <id> [--soft-delete <id>...] --reason <text> [--reviewer <id>|--approved-by <id>] [--expires-at <rfc3339>] [--format text|json]
  supermover prune supersede --profile <path> --id <approval-id> --reason <text> --reviewer <id> [--format text|json]

Reviews and applies approved physical-prune evidence.
The profile remains the policy SSOT. The default dry-run wiring reads published
soft-delete records and emits review candidates/refusals without mutating target
files. Approvals lists current-scope approval artifacts without mutating them.
Review reads candidates, approvals, and receipts as a focused release review
surface without writing approvals, receipts, or target files. Approve writes a
durable prune approval artifact without deleting target files or writing prune
receipts. Supersede updates one existing approval artifact to a superseded
review state without applying prune. Apply requires durable approval evidence
and writes a prune receipt before target mutation.`)
}

func printReconcileUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage of reconcile:
  supermover reconcile plan --profile <path> [--id <persisted-drift-id>...] [--session <id>] [--format text|json]
  supermover reconcile apply --profile <path> --id <persisted-drift-id> [--id <persisted-drift-id>...] --apply --reason <text> [--reviewer <id>] [--session <id>] [--format text|json]

Plans and applies narrow persisted drift repair from durable target-drift
evidence on the profile-selected target. Current apply support is limited to
missing regular-file restores from published/source evidence and
already-restored resolves. It does not run broad automatic reconcile,
automatic scanning, live-only repair, manifest rewrites, pruning, retry policy,
or persist apply receipts.`)
}

func printDriftUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  supermover drift list --profile <path> [--session <id>] [--format text|json]
  supermover drift record --profile <path> [--session <id>] [--format text|json]
  supermover drift acknowledge --profile <path> --id <persisted-drift-id> --reason <text> [--reviewer <id>] [--format text|json]
  supermover drift resolve --profile <path> --id <persisted-drift-id> --reason <text> [--reviewer <id>] [--format text|json]`)
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `%s - %s

Usage:
  supermover <command> [flags]

Available commands:
  profile     Manage profile SSOT configuration
  scan        Scan configured profile roots without writing target state
  push        Push source roots; --network uses paired profile-backed mTLS
  verify      Verify manifests and restored files
  dashboard   Serve local-only read-only target verification page
  drift       List target-local drift from published evidence
  deleted     Review source-side soft-delete records
  prune       Review soft-delete prune candidates; inspect/author/apply prune approval artifacts
  reconcile   Plan/apply narrow persisted drift repair
  health      Inspect target control-plane health
  report      Summarize local migration evidence for operator review
  status      Show compact local profile/target status
  recover     Resume safe local sessions or mark incomplete sessions
  serve       Pairing plus profile-backed TLS receiver for push --network
  daemon      Manage foreground agent lifecycle state around serve
  sync        Manage durable changed-file queue evidence only
  discover    Low-information explicit address hints; no LAN browsing or trust
  pair        Write local pairing receipt/profile pins after verification

Use "supermover help" for this overview.
`, buildinfo.Name, buildinfo.Description)
}
