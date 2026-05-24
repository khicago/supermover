package cli

import (
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
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/khicago/supermover/internal/agentdaemon"
	"github.com/khicago/supermover/internal/agentkb"
	"github.com/khicago/supermover/internal/audit"
	"github.com/khicago/supermover/internal/buildinfo"
	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/discovery"
	"github.com/khicago/supermover/internal/driftreview"
	"github.com/khicago/supermover/internal/durable"
	"github.com/khicago/supermover/internal/health"
	"github.com/khicago/supermover/internal/incrementalsync"
	"github.com/khicago/supermover/internal/localpush"
	"github.com/khicago/supermover/internal/networkpush"
	"github.com/khicago/supermover/internal/operatorui"
	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/pairserve"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/prune"
	"github.com/khicago/supermover/internal/receiverserve"
	"github.com/khicago/supermover/internal/reconcile"
	"github.com/khicago/supermover/internal/report"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/sourceconsistency"
	"github.com/khicago/supermover/internal/status"
	"github.com/khicago/supermover/internal/targetlock"
	"github.com/khicago/supermover/internal/tlsidentity"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/verify"
)

const (
	maxPairingBootstrapBytes = 64 * 1024

	defaultDiscoveryBrowseListen         = "0.0.0.0:39394"
	defaultDiscoveryAdvertiseDestination = "255.255.255.255:39394"
	defaultDiscoveryAdvertiseDuration    = 10 * time.Second
	defaultDiscoveryAdvertiseListen      = "0.0.0.0:0"
	defaultDiscoveryAdvertiseInterval    = 1 * time.Second
	defaultSyncNetworkDiscoverTimeout    = 2 * time.Second
)

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
	// DiscoverBrowseReady lets LAN discovery tests send datagrams after the browse socket is bound.
	DiscoverBrowseReady func(address string)
	// DiscoverAdvertiseReady lets LAN discovery tests observe the advertisement socket before the first write.
	DiscoverAdvertiseReady func(address string)
	// syncLoopAfterRun lets tests mutate source state between foreground loop passes.
	syncLoopAfterRun func(syncRunResult)
	// syncWatchReady lets tests mutate source state after the OS watcher is armed.
	syncWatchReady func()
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

func (r Runner) baseContext() context.Context {
	if r.Context != nil {
		return r.Context
	}
	return context.Background()
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
  supermover push --network --profile <path> [--dry-run] [--session <id>] [--format text|json] [--source-baseline <path>]

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
	sourceBaselinePath := fs.String("source-baseline", "", "optional path to write the exact source baseline used for this transfer")
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
	trust, err := pairing.ValidateSourceProfileTrust(p)
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
	var transferEntries []scan.Entry
	if strings.TrimSpace(*sourceBaselinePath) != "" {
		baseline, entries, err := buildSourceConsistencyBaseline(p, effectiveSessionID, r.nowFunc())
		if err != nil {
			fmt.Fprintf(stderr, "push --network: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
		if err := writeSourceConsistencyBaseline(*sourceBaselinePath, baseline); err != nil {
			fmt.Fprintf(stderr, "push --network: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
		transferEntries = entries
	}
	opts := networkpush.Options{
		Profile:   p,
		SessionID: effectiveSessionID,
		Now:       r.nowFunc(),
		TransferEntries: transferEntries,
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
	if strings.TrimSpace(*sourceBaselinePath) != "" {
		plan.SourceBaseline = *sourceBaselinePath
	}
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
	if len(args) > 0 && args[0] == "source-consistency" {
		return r.runVerifySourceConsistency(args[1:], stdout, stderr)
	}
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

func (r Runner) runVerifySourceConsistency(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("verify source-consistency", stderr)
	profilePath := fs.String("profile", "", "profile path")
	baselinePath := fs.String("baseline", "", "source consistency baseline JSON path")
	format := fs.String("format", "text", "output format: text or json")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage = func() {
			fmt.Fprintln(fs.Output(), `Usage of verify source-consistency:
  supermover verify source-consistency --profile <path> --baseline <path> [--format text|json]

Reads a source-side baseline captured from the transfer session itself and
compares the current source tree against that exact baseline. This is a
read-only current-source proof surface for acceptance and audit workflows.`)
			fs.PrintDefaults()
		}
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "verify source-consistency: --profile is required")
		return 2
	}
	if strings.TrimSpace(*baselinePath) == "" {
		fmt.Fprintln(stderr, "verify source-consistency: --baseline is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "verify source-consistency: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "verify source-consistency: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "verify source-consistency: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	baselineFile, err := os.Open(*baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "verify source-consistency: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	defer baselineFile.Close()
	baseline, err := control.Read[sourceconsistency.Baseline](baselineFile)
	if err != nil {
		fmt.Fprintf(stderr, "verify source-consistency: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if len(p.Roots) != 1 {
		fmt.Fprintf(stderr, "verify source-consistency: %s\n", safeDiagnosticLine("source consistency requires exactly one profile root"))
		return 2
	}
	rootPath, err := filepath.Abs(p.Roots[0].Path)
	if err != nil {
		fmt.Fprintf(stderr, "verify source-consistency: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	baselineRoot, err := filepath.Abs(filepath.FromSlash(baseline.RootPath))
	if err != nil {
		fmt.Fprintf(stderr, "verify source-consistency: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if rootPath != baselineRoot {
		fmt.Fprintf(stderr, "verify source-consistency: %s\n", safeDiagnosticLine(fmt.Sprintf("profile root %q does not match baseline root %q", filepath.ToSlash(rootPath), filepath.ToSlash(baselineRoot))))
		return 2
	}
	if strings.TrimSpace(baseline.ProfileID) != "" && baseline.ProfileID != p.ProfileID {
		fmt.Fprintf(stderr, "verify source-consistency: %s\n", safeDiagnosticLine(fmt.Sprintf("baseline profile_id %q does not match profile_id %q", baseline.ProfileID, p.ProfileID)))
		return 2
	}
	if strings.TrimSpace(baseline.RootID) != "" && baseline.RootID != p.Roots[0].ID {
		fmt.Fprintf(stderr, "verify source-consistency: %s\n", safeDiagnosticLine(fmt.Sprintf("baseline root_id %q does not match profile root_id %q", baseline.RootID, p.Roots[0].ID)))
		return 2
	}
	report, err := sourceconsistency.Compare(sourceconsistency.CompareOptions{
		ProfilePath: *profilePath,
		Baseline:    baseline,
		Now:         r.nowFunc(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "verify source-consistency: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "text":
		printSourceConsistencyText(stdout, report)
	case "json":
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			fmt.Fprintf(stderr, "verify source-consistency: encode report: %v\n", err)
			return 1
		}
	}
	if report.Status != sourceconsistency.StatusPass {
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
	case "expire":
		return r.runDriftExpire(args[1:], stdout, stderr)
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

func (r Runner) runDriftExpire(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("drift expire", stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of drift expire:
  supermover drift expire --profile <path> --id <persisted-drift-id> --reason <text> [--reviewer <id>] [--format text|json]

Marks one existing durable .supermover/drift review record expired when the operator wants to retire stale review evidence without claiming the target is restored.
The id must come from persisted target_drifts evidence, including drift record output; live-only drift list/report.live_target_drift ids are refused.
This records expiry metadata only; it does not repair target files, rewrite manifests, prune files, or suppress future detector output.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	id := fs.String("id", "", "persisted drift id")
	reason := fs.String("reason", "", "operator expiry reason")
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
		fmt.Fprintf(stderr, "drift expire: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "drift expire: --profile is required")
		return 2
	}
	if *id == "" {
		fmt.Fprintln(stderr, "drift expire: --id is required")
		return 2
	}
	if err := control.ValidateArtifactID(*id); err != nil {
		fmt.Fprintf(stderr, "drift expire: --id is invalid: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*reason) == "" {
		fmt.Fprintln(stderr, "drift expire: --reason is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "drift expire: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "drift expire: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "drift expire: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	result, err := driftreview.Expire(driftreview.ExpireOptions{
		Profile:  p,
		ID:       *id,
		Reason:   *reason,
		Reviewer: *reviewer,
		Now:      r.Now,
	})
	if err != nil {
		fmt.Fprintf(stderr, "drift expire: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	switch *format {
	case "text":
		printDriftExpireText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "drift expire: encode result: %v\n", err)
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
	case "review":
		return r.runReconcileReview(args[1:], stdout, stderr)
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
evidence on the profile-selected target. Refusals include conflict_class and
retry_advice review fields; they do not trigger automatic retry.`)
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

func (r Runner) runReconcileReview(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("reconcile review", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of reconcile review:
  supermover reconcile review --profile <path> [--session <id>] [--format text|json]

Reviews broad repair boundaries without mutating the target. It shows the
current persisted-drift reconcile plan, live-only detector inputs that must be
recorded before selected apply, and planned boundaries for background scans,
manifest rewrite, daemon/ongoing sync integration, drift-to-prune handoff, and
automatic retry policy.`)
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
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		fmt.Fprintf(stderr, "reconcile review: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	p, _, selectedSession, ok := readReconcileCommandInputs("reconcile review", fs, *profilePath, nil, *sessionID, *format, false, stderr)
	if !ok {
		return 2
	}
	review, err := reconcile.ReviewBoundaries(reconcile.ReviewOptions{
		Profile:   p,
		SessionID: selectedSession,
		Now:       r.nowFunc()(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "reconcile review: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if err := printReconcileReview(stdout, stderr, *format, review); err != nil {
		fmt.Fprintf(stderr, "reconcile review: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if review.NeedsReview() {
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
  supermover reconcile apply --profile <path> --all-persisted-planned --apply --reason <text> [--reviewer <id>] [--session <id>] [--format text|json]
  supermover reconcile apply --profile <path> --record-live --apply --reason <text> [--reviewer <id>] [--session <id>] [--format text|json]

Applies only selected narrow persisted-drift reconcile actions after explicit
operator intent. --all-persisted-planned is an explicit gate that first reviews
durable persisted drift evidence and then selects only planned persisted
actions. --record-live is an explicit gate that first persists current live
detector findings as durable drift records, then applies only those persisted
planned actions. Planned background boundaries remain non-apply-capable.
Refusals include conflict_class and retry_advice review fields. This command is
not a broad automatic repair or background retry runner.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	ids := multiFlag{}
	fs.Var(&ids, "id", "persisted target drift id to apply; repeatable")
	allPersistedPlanned := fs.Bool("all-persisted-planned", false, "apply every currently planned persisted target drift action after review")
	recordLive := fs.Bool("record-live", false, "persist current live detector findings before applying planned persisted actions")
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
	selectionModes := 0
	if len(ids) > 0 {
		selectionModes++
	}
	if *allPersistedPlanned {
		selectionModes++
	}
	if *recordLive {
		selectionModes++
	}
	if selectionModes == 0 {
		fmt.Fprintln(stderr, "reconcile apply: at least one --id, --all-persisted-planned, or --record-live is required")
		return 2
	}
	if selectionModes > 1 {
		fmt.Fprintln(stderr, "reconcile apply: only one of --id, --all-persisted-planned, or --record-live is allowed")
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
	p, selectedIDs, selectedSession, ok := readReconcileCommandInputs("reconcile apply", fs, *profilePath, ids, *sessionID, *format, false, stderr)
	if !ok {
		return 2
	}
	if *allPersistedPlanned {
		var err error
		selectedIDs, err = allPersistedPlannedReconcileIDs(p, selectedSession, r.nowFunc()())
		if err != nil {
			fmt.Fprintf(stderr, "reconcile apply: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	if *recordLive {
		var err error
		selectedIDs, err = recordLiveReconcileIDs(p, selectedSession, r.nowFunc()())
		if err != nil {
			fmt.Fprintf(stderr, "reconcile apply: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
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

func allPersistedPlannedReconcileIDs(p profile.Profile, sessionID string, now time.Time) ([]string, error) {
	ids, err := persistedPlannedReconcileIDs(p, sessionID, now)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errors.New("--all-persisted-planned found no persisted planned reconcile actions; run drift record before reconcile apply for live-only findings")
	}
	return ids, nil
}

func persistedPlannedReconcileIDs(p profile.Profile, sessionID string, now time.Time) ([]string, error) {
	review, err := reconcile.ReviewBoundaries(reconcile.ReviewOptions{
		Profile:   p,
		SessionID: sessionID,
		Now:       now,
	})
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, action := range review.PersistedPlan.Actions {
		if action.Result == reconcile.ResultPlanned && strings.TrimSpace(action.DriftID) != "" {
			ids = append(ids, action.DriftID)
		}
	}
	return ids, nil
}

func recordLiveReconcileIDs(p profile.Profile, sessionID string, now time.Time) ([]string, error) {
	recorded, err := driftreview.Record(driftreview.RecordOptions{
		Profile:   p,
		SessionID: sessionID,
		Now:       now,
	})
	if err != nil {
		return nil, err
	}
	if len(recorded.Records) == 0 {
		return nil, errors.New("--record-live found no live target drift records to persist before reconcile apply")
	}
	ids := make([]string, 0, len(recorded.Records))
	for _, record := range recorded.Records {
		if strings.TrimSpace(record.ID) != "" {
			ids = append(ids, record.ID)
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("--record-live did not produce persisted drift ids before reconcile apply")
	}
	return ids, nil
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
It may surface persisted network-transfer artifacts, incremental sync queue/run
receipts, and live drift evidence, but it does not start daemon, transport,
watcher, or repair work, and it does not persist live detector output.`)
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
output is not persisted. Network and incremental sync fields report local
artifact evidence only; this command does not start discovery, LAN transfer,
encrypted transfer, watcher work, or synchronization.`)
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

func flagProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
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
	case "run":
		return r.runSyncRun(args[1:], stdout, stderr)
	case "loop":
		return r.runSyncLoop(args[1:], stdout, stderr)
	case "watch":
		return r.runSyncWatch(args[1:], stdout, stderr)
	case "network":
		return r.runSyncNetwork(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "sync: unknown subcommand %q\n", args[0])
		printSyncUsage(stderr)
		return 2
	}
}

func (r Runner) runSyncNetwork(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "sync network: missing subcommand")
		printSyncNetworkUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printSyncNetworkUsage(stdout)
		return 0
	case "run":
		return r.runSyncNetworkRun(args[1:], stdout, stderr)
	case "discover-run":
		return r.runSyncNetworkDiscoverRun(args[1:], stdout, stderr)
	case "loop":
		return r.runSyncNetworkLoop(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "sync network: unknown subcommand %q\n", args[0])
		printSyncNetworkUsage(stderr)
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
	case "list":
		return r.runSyncQueueList(args[1:], stdout, stderr)
	case "ready":
		return r.runSyncQueueReady(args[1:], stdout, stderr)
	case "cancel":
		return r.runSyncQueueCancel(args[1:], stdout, stderr)
	case "fail":
		return r.runSyncQueueFail(args[1:], stdout, stderr)
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
	result, err := enqueueProfileSnapshots(p, scheduler, r.nowFunc()())
	if err != nil {
		fmt.Fprintf(stderr, "sync queue enqueue: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	return printSyncQueueEnqueueResult(stdout, stderr, *format, result)
}

func enqueueProfileSnapshots(p profile.Profile, scheduler *incrementalsync.Scheduler, now time.Time) (syncQueueEnqueueResult, error) {
	snapshots, err := incrementalsync.SnapshotProfile(p)
	if err != nil {
		return syncQueueEnqueueResult{}, err
	}
	result := syncQueueEnqueueResult{
		Operation: "enqueue",
		Mode:      "queue_only",
		Summary:   syncQueueEmptySummary(p, "", now),
	}
	for _, snapshot := range snapshots {
		enqueued, err := scheduler.Enqueue(snapshot)
		if err != nil {
			return syncQueueEnqueueResult{}, err
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
			return syncQueueEnqueueResult{}, err
		}
		result.Scope = incrementalQueueScope(p)
		result.StatePath = statePath
		result.Summary.StatePath = statePath
	}
	return result, nil
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

func (r Runner) runSyncQueueList(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync queue list", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync queue list:
  supermover sync queue list --profile <path> [--format text|json]

Lists every persisted queue entry from durable changed-file evidence, including
terminal and retry states. This is a read-only detail view; it does not execute
entries, copy files, watch roots, run a daemon, or perform ongoing sync.`)
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
		fmt.Fprintf(stderr, "sync queue list: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync queue list: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, scheduler, statePath, ok := r.syncQueueScheduler("sync queue list", *profilePath, *format, false, stderr)
	if !ok {
		return 2
	}
	summary, queueState, err := syncQueueSummaryOrMissing(scheduler, p, statePath, r.nowFunc()())
	if err != nil {
		fmt.Fprintf(stderr, "sync queue list: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	var entries []incrementalsync.QueueEntry
	if queueState == "present" {
		persisted, _, err := scheduler.State(incrementalQueueScope(p))
		if err != nil {
			fmt.Fprintf(stderr, "sync queue list: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
		entries = persisted.Entries
	}
	result := syncQueueEntriesResult{Operation: "list", Mode: "queue_only", State: queueState, Summary: summary, Entries: entries}
	return printSyncQueueEntriesResult(stdout, stderr, *format, result)
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

func (r Runner) runSyncQueueFail(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync queue fail", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync queue fail:
  supermover sync queue fail --profile <path> --id <entry-id> --reason <text> [--format text|json]

Marks one durable queue entry failed as terminal operator review evidence. This
does not delete source data, mutate target files, retry work, run a daemon, or
perform ongoing sync. A later changed source observation can enqueue new work
for the same path.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	entryID := fs.String("id", "", "queue entry id")
	reason := fs.String("reason", "", "operator failure reason")
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
		fmt.Fprintf(stderr, "sync queue fail: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*entryID) == "" {
		fmt.Fprintln(stderr, "sync queue fail: --id is required")
		return 2
	}
	if strings.TrimSpace(*reason) == "" {
		fmt.Fprintln(stderr, "sync queue fail: --reason is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync queue fail: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, scheduler, _, ok := r.syncQueueScheduler("sync queue fail", *profilePath, *format, true, stderr)
	if !ok {
		return 2
	}
	entry, err := scheduler.MarkFailed(incrementalQueueScope(p), strings.TrimSpace(*entryID), *reason)
	if err != nil {
		fmt.Fprintf(stderr, "sync queue fail: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	summary, state, err := syncQueueSummaryOrMissing(scheduler, p, "", r.nowFunc()())
	if err != nil {
		fmt.Fprintf(stderr, "sync queue fail: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	result := syncQueueFailResult{Operation: "fail", Mode: "queue_only", State: state, Entry: entry, Summary: summary, Reason: strings.TrimSpace(*reason)}
	return printSyncQueueFailResult(stdout, stderr, *format, result)
}

func (r Runner) runSyncRun(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync run", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync run:
  supermover sync run --profile <path> --session <id> [--retry-backoff <duration>] [--format text|json]

Runs one bounded incremental sync pass for the profile-selected target. It first
snapshots profile roots into the durable changed-file queue, then publishes the
currently ready queue entries through the existing local push safety path and
records a durable run receipt. This is not a file watcher, background daemon,
bidirectional sync engine, daemon-integrated network sync, or LAN discovery
executor.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "session id for this bounded run")
	retryBackoff := fs.Duration("retry-backoff", time.Minute, "duration before retry-backoff entries are ready again")
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
		fmt.Fprintf(stderr, "sync run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*sessionID) == "" {
		fmt.Fprintln(stderr, "sync run: --session is required")
		return 2
	}
	if strings.TrimSpace(*sessionID) != *sessionID {
		fmt.Fprintln(stderr, "sync run: --session must not be padded")
		return 2
	}
	if err := transaction.ValidateSessionID(*sessionID); err != nil {
		fmt.Fprintf(stderr, "sync run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *retryBackoff < 0 {
		fmt.Fprintln(stderr, "sync run: --retry-backoff cannot be negative")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync run: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, scheduler, _, ok := r.syncQueueScheduler("sync run", *profilePath, *format, true, stderr)
	if !ok {
		return 2
	}
	ctx := r.baseContext()
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "sync run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	runResult, err := r.runSyncPass(ctx, p, targetDir, scheduler, *sessionID, *retryBackoff, "bounded_queue_consumer")
	if err != nil {
		fmt.Fprintf(stderr, "sync run: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	return printSyncRunResult(stdout, stderr, *format, runResult)
}

func (r Runner) runSyncLoop(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync loop", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync loop:
  supermover sync loop --profile <path> --session-prefix <id> [--interval <duration>] [--max-runs <n>] [--retry-backoff <duration>] [--format text|json]

Runs a foreground local polling loop for the profile-selected target. Each pass
snapshots profile roots into the durable changed-file queue, publishes currently
ready entries through the existing local push safety path, and writes a durable
run receipt with a generated session id. Use --max-runs for bounded smoke and
release checks. This is not an OS file watcher, background daemon,
bidirectional sync engine, daemon-integrated network sync, or LAN discovery
executor.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	sessionPrefix := fs.String("session-prefix", "", "session id prefix for generated loop run receipts")
	interval := fs.Duration("interval", time.Minute, "duration to wait between passes")
	maxRuns := fs.Int("max-runs", 0, "maximum passes before exit; 0 runs until interrupted")
	retryBackoff := fs.Duration("retry-backoff", time.Minute, "duration before retry-backoff entries are ready again")
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
		fmt.Fprintf(stderr, "sync loop: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*sessionPrefix) == "" {
		fmt.Fprintln(stderr, "sync loop: --session-prefix is required")
		return 2
	}
	if strings.TrimSpace(*sessionPrefix) != *sessionPrefix {
		fmt.Fprintln(stderr, "sync loop: --session-prefix must not be padded")
		return 2
	}
	if err := transaction.ValidateSessionID(*sessionPrefix); err != nil {
		fmt.Fprintf(stderr, "sync loop: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if err := transaction.ValidateSessionID(syncLoopSessionID(*sessionPrefix, 1)); err != nil {
		fmt.Fprintf(stderr, "sync loop: generated session id is invalid: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *interval <= 0 {
		fmt.Fprintln(stderr, "sync loop: --interval must be greater than zero")
		return 2
	}
	if *maxRuns < 0 {
		fmt.Fprintln(stderr, "sync loop: --max-runs cannot be negative")
		return 2
	}
	if *retryBackoff < 0 {
		fmt.Fprintln(stderr, "sync loop: --retry-backoff cannot be negative")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync loop: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, scheduler, _, ok := r.syncQueueScheduler("sync loop", *profilePath, *format, true, stderr)
	if !ok {
		return 2
	}
	ctx := r.baseContext()
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "sync loop: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	result := syncLoopResult{
		Operation:     "loop",
		Mode:          "foreground_polling_queue_consumer",
		SessionPrefix: *sessionPrefix,
		Interval:      interval.String(),
		MaxRuns:       *maxRuns,
		Status:        "completed",
	}
	exitCode := 0
	includeRuns := *maxRuns > 0
	for runNumber := 1; *maxRuns == 0 || runNumber <= *maxRuns; runNumber++ {
		if err := ctx.Err(); err != nil {
			result.Status = "canceled"
			exitCode = 1
			break
		}
		runResult, err := r.runSyncPass(ctx, p, targetDir, scheduler, syncLoopSessionID(*sessionPrefix, runNumber), *retryBackoff, "foreground_loop_pass")
		if err != nil {
			fmt.Fprintf(stderr, "sync loop: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
		result.CompletedRuns++
		if includeRuns {
			result.Runs = append(result.Runs, runResult)
		}
		switch runResult.Run.Status {
		case incrementalsync.RunStatusIdle:
			result.IdleRuns++
		case incrementalsync.RunStatusPublished:
			result.PublishedRuns++
		case incrementalsync.RunStatusRetrying:
			result.RetryingRuns++
		}
		if runResult.Run.Status == incrementalsync.RunStatusRetrying {
			result.Status = "retrying"
			exitCode = 1
		}
		if r.syncLoopAfterRun != nil {
			r.syncLoopAfterRun(runResult)
		}
		if *maxRuns != 0 && runNumber >= *maxRuns {
			break
		}
		if err := waitSyncLoopInterval(ctx, *interval); err != nil {
			result.Status = "canceled"
			exitCode = 1
			break
		}
	}
	return printSyncLoopResult(stdout, stderr, *format, result, exitCode)
}

func (r Runner) runSyncWatch(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync watch", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync watch:
  supermover sync watch --profile <path> --session-prefix <id> [--settle <duration>] [--max-events <n>] [--retry-backoff <duration>] [--format text|json]

Runs a foreground OS file watcher for the profile-selected source roots. It
arms recursive watchers for existing directories, runs one baseline local queue
consumer pass, then coalesces OS file events into durable queue/run receipts
through the existing local push safety path. Use --max-events for bounded smoke
and release checks. This is not a background daemon, bidirectional sync engine,
daemon-integrated network sync, or LAN discovery executor.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	sessionPrefix := fs.String("session-prefix", "", "session id prefix for generated watcher run receipts")
	settle := fs.Duration("settle", 250*time.Millisecond, "duration to coalesce file events before each watcher pass")
	maxEvents := fs.Int("max-events", 0, "maximum coalesced event batches before exit; 0 runs until interrupted")
	retryBackoff := fs.Duration("retry-backoff", time.Minute, "duration before retry-backoff entries are ready again")
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
		fmt.Fprintf(stderr, "sync watch: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*sessionPrefix) == "" {
		fmt.Fprintln(stderr, "sync watch: --session-prefix is required")
		return 2
	}
	if strings.TrimSpace(*sessionPrefix) != *sessionPrefix {
		fmt.Fprintln(stderr, "sync watch: --session-prefix must not be padded")
		return 2
	}
	if err := transaction.ValidateSessionID(*sessionPrefix); err != nil {
		fmt.Fprintf(stderr, "sync watch: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if err := transaction.ValidateSessionID(syncLoopSessionID(*sessionPrefix, 1)); err != nil {
		fmt.Fprintf(stderr, "sync watch: generated session id is invalid: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *settle <= 0 {
		fmt.Fprintln(stderr, "sync watch: --settle must be greater than zero")
		return 2
	}
	if *maxEvents < 0 {
		fmt.Fprintln(stderr, "sync watch: --max-events cannot be negative")
		return 2
	}
	if *retryBackoff < 0 {
		fmt.Fprintln(stderr, "sync watch: --retry-backoff cannot be negative")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync watch: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	p, scheduler, _, ok := r.syncQueueScheduler("sync watch", *profilePath, *format, true, stderr)
	if !ok {
		return 2
	}
	ctx := r.baseContext()
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "sync watch: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	result, err := r.runSyncWatchLoop(ctx, p, targetDir, scheduler, *sessionPrefix, *settle, *maxEvents, *retryBackoff)
	if err != nil {
		fmt.Fprintf(stderr, "sync watch: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	return printSyncWatchResult(stdout, stderr, *format, result)
}

func (r Runner) runSyncNetworkRun(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync network run", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync network run:
  supermover sync network run --profile <path> --session <id> [--retry-backoff <duration>] [--format text|json]

Runs one bounded network incremental pass using the profile-selected target
control plane for durable queue/run receipts. Ready queue entries are published
through a per-entry profile-backed mTLS network manifest; regular-file
replacements require previous published manifest evidence and receiver-side
target revalidation. It validates profile trust, network material, and the
network push contract before queue mutation. This is not LAN discovery,
automatic endpoint selection, a foreground watcher, a background daemon, broad
repair, or bidirectional sync.`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "session id for this bounded network run")
	retryBackoff := fs.Duration("retry-backoff", time.Minute, "duration before retry-backoff entries are ready again")
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
		fmt.Fprintf(stderr, "sync network run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*sessionID) == "" {
		fmt.Fprintln(stderr, "sync network run: --session is required")
		return 2
	}
	if strings.TrimSpace(*sessionID) != *sessionID {
		fmt.Fprintln(stderr, "sync network run: --session must not be padded")
		return 2
	}
	if err := transaction.ValidateSessionID(*sessionID); err != nil {
		fmt.Fprintf(stderr, "sync network run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *retryBackoff < 0 {
		fmt.Fprintln(stderr, "sync network run: --retry-backoff cannot be negative")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync network run: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "sync network run: --profile is required")
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "sync network run: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "sync network run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	trust, err := validateSyncNetworkRunProfile(p, r.nowFunc())
	if err != nil {
		fmt.Fprintf(stderr, "sync network run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	stateDir, err := incrementalQueueStateDir(p, true)
	if err != nil {
		fmt.Fprintf(stderr, "sync network run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	scheduler, err := incrementalsync.New(incrementalsync.Options{StateDir: stateDir, Now: r.nowFunc()})
	if err != nil {
		fmt.Fprintf(stderr, "sync network run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	result, err := r.runSyncNetworkPass(r.baseContext(), p, scheduler, trust, *sessionID, *retryBackoff, "bounded_network_queue_consumer")
	if err != nil {
		fmt.Fprintf(stderr, "sync network run: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	return printSyncNetworkRunResult(stdout, stderr, *format, result)
}

func (r Runner) runSyncNetworkDiscoverRun(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync network discover-run", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync network discover-run:
  supermover sync network discover-run --profile <path> --session <id> [--listen <udp-host:port>] [--timeout <duration>] [--retry-backoff <duration>] [--format text|json]

Runs one LAN-discovery-gated network incremental pass. Discovery is a low-info
availability hint only: the selected candidate must match the profile-selected
network.receiver_url host:port. Once that gate passes, ready queue entries use
the same per-entry profile-pinned mTLS transfer, including previous published
evidence for regular-file replacement. The command never treats discovery as
trust and does not write queue/run receipts unless the profile-backed discovery
gate passes.
Non-goals:
- not automatic endpoint selection
- not profile mutation
- not a foreground watcher
- not a background daemon
- not broad repair
- not bidirectional sync`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	sessionID := fs.String("session", "", "session id for this bounded discovery-gated network run")
	listen := fs.String("listen", defaultDiscoveryBrowseListen, "--listen UDP host:port for low-information LAN advertisements")
	timeout := fs.Duration("timeout", defaultSyncNetworkDiscoverTimeout, "duration to wait for LAN discovery before running")
	retryBackoff := fs.Duration("retry-backoff", time.Minute, "duration before retry-backoff entries are ready again")
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
		fmt.Fprintf(stderr, "sync network discover-run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*sessionID) == "" {
		fmt.Fprintln(stderr, "sync network discover-run: --session is required")
		return 2
	}
	if strings.TrimSpace(*sessionID) != *sessionID {
		fmt.Fprintln(stderr, "sync network discover-run: --session must not be padded")
		return 2
	}
	if err := transaction.ValidateSessionID(*sessionID); err != nil {
		fmt.Fprintf(stderr, "sync network discover-run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(stderr, "sync network discover-run: --timeout must be greater than zero")
		return 2
	}
	if *retryBackoff < 0 {
		fmt.Fprintln(stderr, "sync network discover-run: --retry-backoff cannot be negative")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync network discover-run: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "sync network discover-run: --profile is required")
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "sync network discover-run: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "sync network discover-run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	trust, err := validateSyncNetworkRunProfile(p, r.nowFunc())
	if err != nil {
		fmt.Fprintf(stderr, "sync network discover-run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	conn, err := listenDiscoveryUDP(*listen)
	if err != nil {
		if errors.Is(err, errInvalidDiscoveryAddress) {
			fmt.Fprintf(stderr, "sync network discover-run: invalid --listen %q\n", *listen)
			return 2
		}
		fmt.Fprintf(stderr, "sync network discover-run: listen: %v\n", err)
		return 1
	}
	defer conn.Close()
	if r.DiscoverBrowseReady != nil {
		r.DiscoverBrowseReady(conn.LocalAddr().String())
	}
	candidates, gate, err := r.syncNetworkDiscoveryGate(conn, p, *timeout)
	if err != nil {
		fmt.Fprintf(stderr, "sync network discover-run: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if gate.Status != "matched" {
		result := syncNetworkDiscoverRunResult{
			Operation: "discover-run",
			Mode:      "lan_discovery_gated_network_queue_consumer",
			Discovery: gate,
		}
		return printSyncNetworkDiscoverRunResult(stdout, stderr, *format, result, 1)
	}
	stateDir, err := incrementalQueueStateDir(p, true)
	if err != nil {
		fmt.Fprintf(stderr, "sync network discover-run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	scheduler, err := incrementalsync.New(incrementalsync.Options{StateDir: stateDir, Now: r.nowFunc()})
	if err != nil {
		fmt.Fprintf(stderr, "sync network discover-run: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	runResult, err := r.runSyncNetworkPass(r.baseContext(), p, scheduler, trust, *sessionID, *retryBackoff, "lan_discovery_gated_network_queue_consumer")
	if err != nil {
		fmt.Fprintf(stderr, "sync network discover-run: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	gate.CandidateCount = len(candidates)
	result := syncNetworkDiscoverRunResult{
		Operation: "discover-run",
		Mode:      "lan_discovery_gated_network_queue_consumer",
		Discovery: gate,
		Enqueue:   runResult.Enqueue,
		Run:       runResult.Run,
		Network:   runResult.Network,
	}
	return printSyncNetworkDiscoverRunResult(stdout, stderr, *format, result, syncNetworkRunExitCode(runResult))
}

func (r Runner) runSyncNetworkLoop(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("sync network loop", stderr)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `Usage of sync network loop:
  supermover sync network loop --profile <path> --session-prefix <id> [--interval <duration>] [--max-runs <n>] [--retry-backoff <duration>] [--format text|json]

Runs a foreground network polling loop using the profile-selected target control
plane for durable queue/run receipts. Ready queue entries are published through
a per-entry profile-backed mTLS network manifest; regular-file replacements
require previous published manifest evidence and receiver-side target
revalidation. Each pass validates profile trust, network material, and the
network push contract before queue mutation. Use --max-runs for bounded smoke
and release checks.
Non-goals:
- not LAN discovery
- not automatic endpoint selection
- not an OS file watcher
- not a background daemon
- not broad repair
- not bidirectional sync`)
		fs.PrintDefaults()
	}
	profilePath := fs.String("profile", "", "profile path")
	sessionPrefix := fs.String("session-prefix", "", "session id prefix for generated network loop run receipts")
	interval := fs.Duration("interval", time.Minute, "duration to wait between network passes")
	maxRuns := fs.Int("max-runs", 0, "maximum passes before exit; 0 runs until interrupted")
	retryBackoff := fs.Duration("retry-backoff", time.Minute, "duration before retry-backoff entries are ready again")
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
		fmt.Fprintf(stderr, "sync network loop: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if strings.TrimSpace(*sessionPrefix) == "" {
		fmt.Fprintln(stderr, "sync network loop: --session-prefix is required")
		return 2
	}
	if strings.TrimSpace(*sessionPrefix) != *sessionPrefix {
		fmt.Fprintln(stderr, "sync network loop: --session-prefix must not be padded")
		return 2
	}
	if err := transaction.ValidateSessionID(*sessionPrefix); err != nil {
		fmt.Fprintf(stderr, "sync network loop: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if err := transaction.ValidateSessionID(syncLoopSessionID(*sessionPrefix, 1)); err != nil {
		fmt.Fprintf(stderr, "sync network loop: generated session id is invalid: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if *interval <= 0 {
		fmt.Fprintln(stderr, "sync network loop: --interval must be greater than zero")
		return 2
	}
	if *maxRuns < 0 {
		fmt.Fprintln(stderr, "sync network loop: --max-runs cannot be negative")
		return 2
	}
	if *retryBackoff < 0 {
		fmt.Fprintln(stderr, "sync network loop: --retry-backoff cannot be negative")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "sync network loop: unexpected arguments: %s\n", formatDiagnosticArgs(fs.Args()))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "sync network loop: --profile is required")
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "sync network loop: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "sync network loop: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	trust, err := validateSyncNetworkRunProfile(p, r.nowFunc())
	if err != nil {
		fmt.Fprintf(stderr, "sync network loop: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	stateDir, err := incrementalQueueStateDir(p, true)
	if err != nil {
		fmt.Fprintf(stderr, "sync network loop: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	scheduler, err := incrementalsync.New(incrementalsync.Options{StateDir: stateDir, Now: r.nowFunc()})
	if err != nil {
		fmt.Fprintf(stderr, "sync network loop: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	result, exitCode, err := r.runSyncNetworkLoopPasses(r.baseContext(), p, scheduler, trust, *sessionPrefix, *interval, *maxRuns, *retryBackoff)
	if err != nil {
		fmt.Fprintf(stderr, "sync network loop: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	return printSyncNetworkLoopResult(stdout, stderr, *format, result, exitCode)
}

func validateSyncNetworkRunProfile(p profile.Profile, now func() time.Time) (pairing.TrustState, error) {
	if err := p.ValidateNetworkClientMaterial(); err != nil {
		return pairing.TrustState{}, err
	}
	if err := networkpush.ValidateProfileForNetworkPush(p); err != nil {
		return pairing.TrustState{}, err
	}
	trust, err := pairing.ValidateSourceProfileTrust(p)
	if err != nil {
		return pairing.TrustState{}, err
	}
	sourceDeviceID, err := pairing.SourceTransportDeviceID(p, now())
	if err != nil {
		return pairing.TrustState{}, fmt.Errorf("validate local TLS identity files: %w", err)
	}
	trust.Receipt.SourceDeviceID = sourceDeviceID
	if err := tlsidentity.ValidatePinned(p.Network.LocalTLSIdentity, sourceDeviceID, now); err != nil {
		return pairing.TrustState{}, fmt.Errorf("validate local TLS identity files: %w", err)
	}
	return trust, nil
}

func (r Runner) syncNetworkDiscoveryGate(conn discovery.DatagramConn, p profile.Profile, timeout time.Duration) ([]discovery.Candidate, syncNetworkDiscoveryGate, error) {
	now := r.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ctx, cancel := context.WithTimeout(r.baseContext(), timeout)
	defer cancel()
	invalidPackets := 0
	candidates, err := discovery.Browse(ctx, discovery.DatagramSource{
		Conn:            conn,
		ServiceType:     pairserve.ServiceType,
		ProtocolVersion: protocol.Version,
		TTL:             discovery.DefaultHintTTL,
		InvalidPackets:  &invalidPackets,
	}, now)
	if err != nil {
		return nil, syncNetworkDiscoveryGate{}, err
	}
	profileAddress, err := receiverURLHostPort(p)
	if err != nil {
		return candidates, syncNetworkDiscoveryGate{}, err
	}
	gate := syncNetworkDiscoveryGate{
		Status:         "no_matching_candidate",
		Reason:         "no LAN candidate matched profile network.receiver_url",
		ProfileAddress: profileAddress,
		CandidateCount: len(candidates),
		InvalidPackets: invalidPackets,
		Trusted:        false,
	}
	for _, candidate := range candidates {
		if candidate.Class == discovery.CandidateClassAmbiguous {
			continue
		}
		if candidate.Hint.Address != profileAddress {
			continue
		}
		gate.Status = "matched"
		gate.Reason = "candidate matched profile network.receiver_url"
		gate.MatchedAddress = candidate.Hint.Address
		gate.MatchedClass = string(candidate.Class)
		gate.Capabilities = sortedStrings(candidate.Hint.Advertisement.CapabilityFlags)
		gate.ExpiresAt = candidate.Hint.ExpiresAt.Format(time.RFC3339Nano)
		return candidates, gate, nil
	}
	return candidates, gate, nil
}

func receiverURLHostPort(p profile.Profile) (string, error) {
	if p.Network == nil {
		return "", errors.New("network.receiver_url is required")
	}
	parsed, err := url.Parse(p.Network.ReceiverURL)
	if err != nil {
		return "", fmt.Errorf("parse network.receiver_url: %w", err)
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if host == "" || port == "" {
		return "", errors.New("network.receiver_url host and explicit port are required")
	}
	return net.JoinHostPort(host, port), nil
}

func (r Runner) runSyncNetworkPass(ctx context.Context, p profile.Profile, scheduler *incrementalsync.Scheduler, trust pairing.TrustState, sessionID string, retryBackoff time.Duration, mode string) (syncNetworkRunResult, error) {
	enqueued, err := enqueueProfileSnapshots(p, scheduler, r.nowFunc()())
	if err != nil {
		return syncNetworkRunResult{}, err
	}
	networkResult := networkpush.Result{SessionID: sessionID}
	networkAttempted := false
	result, err := scheduler.RunOnce(ctx, incrementalQueueScope(p), incrementalsync.RunOptions{
		SessionID: sessionID,
		Backoff:   retryBackoff,
		Transfer: func(ctx context.Context, entries []incrementalsync.QueueEntry) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			transferEntries, err := networkTransferEntriesFromQueue(p, entries)
			if err != nil {
				return err
			}
			networkAttempted = true
			runResult, err := networkpush.Run(ctx, networkpush.Options{
				Profile:         p,
				SessionID:       sessionID,
				Now:             r.nowFunc(),
				TransferEntries: transferEntries,
			})
			if strings.TrimSpace(runResult.SessionID) == "" {
				runResult.SessionID = sessionID
			}
			networkResult = runResult
			return err
		},
	})
	if err != nil {
		return syncNetworkRunResult{}, err
	}
	return syncNetworkRunResult{
		Operation: "run",
		Mode:      mode,
		Enqueue:   enqueued,
		Run:       result,
		Network:   syncNetworkPushPlan(p, trust, sessionID, networkAttempted, networkResult),
	}, nil
}

func networkTransferEntriesFromQueue(p profile.Profile, entries []incrementalsync.QueueEntry) ([]scan.Entry, error) {
	if len(p.Roots) != 1 {
		return nil, fmt.Errorf("network queue transfer requires exactly one profile root")
	}
	if len(entries) == 0 {
		return nil, nil
	}
	previous, err := previousNetworkManifestEvidence(p)
	if err != nil {
		return nil, err
	}
	out := make([]scan.Entry, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.Root != p.Roots[0].ID {
			return nil, fmt.Errorf("queued entry %q root %q does not match profile root %q", entry.Path, entry.Root, p.Roots[0].ID)
		}
		next, err := networkTransferEntryFromQueue(entry)
		if err != nil {
			return nil, err
		}
		if previousEntry, ok := previous[next.Path]; ok {
			next.PreviousSessionID = previousEntry.PreviousSessionID
			next.PreviousManifestID = previousEntry.PreviousManifestID
			next.PreviousSize = previousEntry.PreviousSize
			next.PreviousDigest = previousEntry.PreviousDigest
			next.PreviousMode = previousEntry.PreviousMode
			next.PreviousModTime = previousEntry.PreviousModTime
		}
		if _, ok := seen[next.Path]; ok {
			return nil, fmt.Errorf("duplicate queued transfer path %q", next.Path)
		}
		seen[next.Path] = struct{}{}
		out = append(out, next)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out, nil
}

type networkPreviousManifestEntry struct {
	PreviousSessionID  string
	PreviousManifestID string
	PreviousSize       *int64
	PreviousDigest     string
	PreviousMode       *uint32
	PreviousModTime    string
}

func previousNetworkManifestEvidence(p profile.Profile) (map[string]networkPreviousManifestEntry, error) {
	out := map[string]networkPreviousManifestEntry{}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		return nil, err
	}
	sessionsDir := filepath.Join(control.ControlDir(targetDir), "sessions")
	sessionDirs, err := os.ReadDir(sessionsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return out, nil
		}
		return nil, fmt.Errorf("read previous network sessions: %w", err)
	}
	type pathEvidence struct {
		entry networkPreviousManifestEntry
		stamp time.Time
	}
	latestByPath := map[string]pathEvidence{}
	for _, sessionDir := range sessionDirs {
		if !sessionDir.IsDir() {
			continue
		}
		sessionID := sessionDir.Name()
		receiptPath, err := control.Path(targetDir, control.ArtifactSessionReceipt, sessionID)
		if err != nil {
			return nil, err
		}
		receipt, err := control.ReadFile[control.SessionReceipt](receiptPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read previous network receipt %q: %w", sessionID, err)
		}
		if receipt.Status != "published" {
			continue
		}
		if receipt.ID != sessionID {
			return nil, fmt.Errorf("published network receipt %q id = %q", sessionID, receipt.ID)
		}
		if receipt.ProfileID != p.ProfileID || receipt.TargetID != p.Target.TargetID {
			continue
		}
		manifestPath, err := control.Path(targetDir, control.ArtifactManifest, sessionID)
		if err != nil {
			return nil, err
		}
		manifest, err := control.ReadManifestCompatFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("read previous network manifest %q: %w", sessionID, err)
		}
		if manifest.SessionID != sessionID {
			return nil, fmt.Errorf("published network manifest %q session_id = %q", sessionID, manifest.SessionID)
		}
		rootID := p.Roots[0].ID
		if manifest.RootID != rootID && !(manifest.RootID == "" && len(p.Roots) == 1) {
			continue
		}
		stamp := manifest.CreatedAt
		if stamp == "" {
			stamp = receipt.StartedAt
		}
		parsedStamp, err := time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			return nil, fmt.Errorf("parse previous network manifest time %q for session %q: %w", stamp, sessionID, err)
		}
		for _, entry := range manifest.Entries {
			if entry.Kind != "file" || !entry.HasSizeEvidence() || !entry.HasModeEvidence() || strings.TrimSpace(entry.Digest) == "" || strings.TrimSpace(entry.ModTime) == "" {
				continue
			}
			path := networkTransferTargetPath(entry.Path, entry.TargetPath)
			current, ok := latestByPath[path]
			if ok && (current.stamp.After(parsedStamp) || (current.stamp.Equal(parsedStamp) && current.entry.PreviousSessionID > manifest.SessionID)) {
				continue
			}
			size := entry.Size
			mode := entry.Mode
			latestByPath[path] = pathEvidence{
				stamp: parsedStamp,
				entry: networkPreviousManifestEntry{
					PreviousSessionID:  manifest.SessionID,
					PreviousManifestID: manifest.ID,
					PreviousSize:       &size,
					PreviousDigest:     entry.Digest,
					PreviousMode:       &mode,
					PreviousModTime:    entry.ModTime,
				},
			}
		}
	}
	for path, evidence := range latestByPath {
		out[path] = evidence.entry
	}
	return out, nil
}

func networkTransferTargetPath(path, targetPath string) string {
	if strings.TrimSpace(targetPath) != "" {
		return filepath.ToSlash(filepath.Clean(filepath.FromSlash(targetPath)))
	}
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
}

func networkTransferEntryFromQueue(entry incrementalsync.QueueEntry) (scan.Entry, error) {
	modTime, err := time.Parse(time.RFC3339Nano, entry.ModTime)
	if err != nil {
		return scan.Entry{}, fmt.Errorf("queued entry %q mod_time: %w", entry.Path, err)
	}
	out := scan.Entry{
		Path:          entry.Path,
		Kind:          entry.Kind,
		Digest:        entry.Digest,
		SymlinkTarget: entry.SymlinkTarget,
		ModTime:       modTime,
	}
	switch entry.Kind {
	case scan.KindRegular:
		if strings.TrimSpace(entry.Digest) == "" {
			return scan.Entry{}, fmt.Errorf("queued regular entry %q is missing digest", entry.Path)
		}
		out.Size = entry.Size
		out.Mode = fs.FileMode(entry.Mode)
		out.Executable = out.Mode.Perm()&0o111 != 0
	case scan.KindDir:
		if entry.Mode == 0 {
			return scan.Entry{}, fmt.Errorf("queued directory entry %q is missing mode evidence", entry.Path)
		}
		out.Mode = fs.FileMode(entry.Mode) | fs.ModeDir
	case scan.KindSymlink:
		if strings.TrimSpace(entry.SymlinkTarget) == "" {
			return scan.Entry{}, fmt.Errorf("queued symlink entry %q is missing symlink_target", entry.Path)
		}
		out.Mode = fs.ModeSymlink
	default:
		return scan.Entry{}, fmt.Errorf("queued entry %q uses unsupported kind %q", entry.Path, entry.Kind)
	}
	return out, nil
}

func (r Runner) runSyncNetworkLoopPasses(ctx context.Context, p profile.Profile, scheduler *incrementalsync.Scheduler, trust pairing.TrustState, sessionPrefix string, interval time.Duration, maxRuns int, retryBackoff time.Duration) (syncNetworkLoopResult, int, error) {
	result := syncNetworkLoopResult{
		Operation:     "loop",
		Mode:          "foreground_network_queue_consumer",
		SessionPrefix: sessionPrefix,
		Interval:      interval.String(),
		MaxRuns:       maxRuns,
		Status:        "completed",
	}
	exitCode := 0
	includeRuns := maxRuns > 0
	for runNumber := 1; maxRuns == 0 || runNumber <= maxRuns; runNumber++ {
		if err := ctx.Err(); err != nil {
			result.Status = "canceled"
			exitCode = 1
			break
		}
		runResult, err := r.runSyncNetworkPass(ctx, p, scheduler, trust, syncLoopSessionID(sessionPrefix, runNumber), retryBackoff, "foreground_network_loop_pass")
		if err != nil {
			return syncNetworkLoopResult{}, 1, err
		}
		result.CompletedRuns++
		if includeRuns {
			result.Runs = append(result.Runs, runResult)
		}
		switch runResult.Run.Status {
		case incrementalsync.RunStatusIdle:
			result.IdleRuns++
		case incrementalsync.RunStatusPublished:
			result.PublishedRuns++
		case incrementalsync.RunStatusRetrying:
			result.RetryingRuns++
		}
		if runResult.Network.Transfer != "not_attempted" {
			result.NetworkAttempts++
		}
		if runResult.Network.Transfer == string(control.NetworkTransferPublished) {
			result.NetworkPublishedRuns++
		}
		if runResult.Network.Transfer == "not_attempted" {
			result.NetworkNotAttemptedRuns++
		}
		if runResult.Run.Status == incrementalsync.RunStatusRetrying {
			result.Status = "retrying"
			exitCode = 1
		}
		if maxRuns != 0 && runNumber >= maxRuns {
			break
		}
		if err := waitSyncLoopInterval(ctx, interval); err != nil {
			result.Status = "canceled"
			exitCode = 1
			break
		}
	}
	return result, exitCode, nil
}

func syncNetworkPushPlan(p profile.Profile, trust pairing.TrustState, sessionID string, attempted bool, result networkpush.Result) networkPushPlan {
	if attempted {
		return networkPushResultFromRun(p, trust, false, result)
	}
	return networkPushPlan{
		ProfileID:         p.ProfileID,
		TargetID:          p.Target.TargetID,
		SourceDeviceID:    trust.Receipt.SourceDeviceID,
		TargetDeviceID:    trust.TargetDeviceID,
		PairingReceiptID:  trust.Receipt.ID,
		SessionID:         sessionID,
		Transfer:          "not_attempted",
		EncryptedTransfer: "profile_backed_mtls_validated",
		Resume:            "not_attempted",
		ResumeAuthority:   "not_attempted",
		ResumeOutcome:     "not_attempted",
		Status:            "idle",
		Stage:             "queue_idle",
	}
}

func (r Runner) runSyncWatchLoop(ctx context.Context, p profile.Profile, targetDir string, scheduler *incrementalsync.Scheduler, sessionPrefix string, settle time.Duration, maxEvents int, retryBackoff time.Duration) (syncWatchResult, error) {
	watcher, err := newSyncWatchBackend(p)
	if err != nil {
		return syncWatchResult{}, err
	}
	defer watcher.Close()
	result := syncWatchResult{
		Operation:     "watch",
		Mode:          "foreground_os_watcher_queue_consumer",
		SessionPrefix: sessionPrefix,
		Settle:        settle.String(),
		MaxEvents:     maxEvents,
		Status:        "completed",
		WatchedRoots:  watcher.WatchedRoots(),
		WatchedDirs:   watcher.WatchedDirs(),
	}
	runNumber := 1
	baseline, err := r.runSyncPass(ctx, p, targetDir, scheduler, syncLoopSessionID(sessionPrefix, runNumber), retryBackoff, "foreground_watch_baseline")
	if err != nil {
		return syncWatchResult{}, err
	}
	result.Baseline = &baseline
	result.CompletedRuns++
	result.PublishedRuns += syncRunPublishedCount(baseline)
	result.IdleRuns += syncRunIdleCount(baseline)
	result.RetryingRuns += syncRunRetryingCount(baseline)
	if baseline.Run.Status == incrementalsync.RunStatusRetrying {
		result.Status = "retrying"
		return result, nil
	}
	if r.syncWatchReady != nil {
		r.syncWatchReady()
	}
	runNumber++
	for maxEvents == 0 || result.EventBatches < maxEvents {
		events, err := watcher.Next(ctx, settle)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				result.Status = "canceled"
				return result, nil
			}
			return syncWatchResult{}, err
		}
		if len(events) == 0 {
			result.Status = "canceled"
			return result, nil
		}
		result.EventBatches++
		result.EventsSeen += len(events)
		runResult, err := r.runSyncPass(ctx, p, targetDir, scheduler, syncLoopSessionID(sessionPrefix, runNumber), retryBackoff, "foreground_watch_event_pass")
		if err != nil {
			return syncWatchResult{}, err
		}
		result.Runs = append(result.Runs, runResult)
		result.CompletedRuns++
		result.PublishedRuns += syncRunPublishedCount(runResult)
		result.IdleRuns += syncRunIdleCount(runResult)
		result.RetryingRuns += syncRunRetryingCount(runResult)
		if runResult.Run.Status == incrementalsync.RunStatusRetrying {
			result.Status = "retrying"
			return result, nil
		}
		runNumber++
	}
	return result, nil
}

func (r Runner) runSyncPass(ctx context.Context, p profile.Profile, targetDir string, scheduler *incrementalsync.Scheduler, sessionID string, retryBackoff time.Duration, mode string) (syncRunResult, error) {
	enqueued, err := enqueueProfileSnapshots(p, scheduler, r.nowFunc()())
	if err != nil {
		return syncRunResult{}, err
	}
	result, err := scheduler.RunOnce(ctx, incrementalQueueScope(p), incrementalsync.RunOptions{
		SessionID: sessionID,
		Backoff:   retryBackoff,
		Transfer: func(ctx context.Context, entries []incrementalsync.QueueEntry) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			_, err := localpush.Run(localpush.Options{Profile: p, TargetDir: targetDir, SessionID: sessionID, Now: r.nowFunc()()})
			return err
		},
	})
	if err != nil {
		return syncRunResult{}, err
	}
	return syncRunResult{
		Operation: "run",
		Mode:      mode,
		Enqueue:   enqueued,
		Run:       result,
	}, nil
}

func syncLoopSessionID(prefix string, runNumber int) string {
	return fmt.Sprintf("%s-%06d", prefix, runNumber)
}

func syncRunPublishedCount(result syncRunResult) int {
	if result.Run.Status == incrementalsync.RunStatusPublished {
		return 1
	}
	return 0
}

func syncRunIdleCount(result syncRunResult) int {
	if result.Run.Status == incrementalsync.RunStatusIdle {
		return 1
	}
	return 0
}

func syncRunRetryingCount(result syncRunResult) int {
	if result.Run.Status == incrementalsync.RunStatusRetrying {
		return 1
	}
	return 0
}

type syncWatchEvent struct {
	Path string
	Op   string
}

type syncWatchBackend struct {
	watcher      *fsnotify.Watcher
	watchedRoots []string
	watchedDirs  map[string]struct{}
}

func newSyncWatchBackend(p profile.Profile) (*syncWatchBackend, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	backend := &syncWatchBackend{
		watcher:     watcher,
		watchedDirs: map[string]struct{}{},
	}
	for _, root := range p.Roots {
		cleanRoot, err := filepath.Abs(root.Path)
		if err != nil {
			backend.Close()
			return nil, err
		}
		if err := backend.addRecursive(cleanRoot); err != nil {
			backend.Close()
			return nil, err
		}
		backend.watchedRoots = append(backend.watchedRoots, cleanRoot)
	}
	sort.Strings(backend.watchedRoots)
	return backend, nil
}

func (b *syncWatchBackend) Close() error {
	if b == nil || b.watcher == nil {
		return nil
	}
	return b.watcher.Close()
}

func (b *syncWatchBackend) WatchedRoots() []string {
	if b == nil {
		return nil
	}
	return append([]string(nil), b.watchedRoots...)
}

func (b *syncWatchBackend) WatchedDirs() int {
	if b == nil {
		return 0
	}
	return len(b.watchedDirs)
}

func (b *syncWatchBackend) Next(ctx context.Context, settle time.Duration) ([]syncWatchEvent, error) {
	if b == nil || b.watcher == nil {
		return nil, errors.New("sync watcher is nil")
	}
	var events []syncWatchEvent
	for {
		select {
		case <-ctx.Done():
			return events, ctx.Err()
		case err, ok := <-b.watcher.Errors:
			if !ok {
				return events, context.Canceled
			}
			return events, err
		case event, ok := <-b.watcher.Events:
			if !ok {
				return events, context.Canceled
			}
			if ignoreSyncWatchEvent(event) {
				continue
			}
			events = append(events, syncWatchEvent{Path: event.Name, Op: event.Op.String()})
			b.addNewDirectoryIfNeeded(event)
			return b.collectSettled(ctx, settle, events)
		}
	}
}

func (b *syncWatchBackend) collectSettled(ctx context.Context, settle time.Duration, events []syncWatchEvent) ([]syncWatchEvent, error) {
	timer := time.NewTimer(settle)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return events, ctx.Err()
		case err, ok := <-b.watcher.Errors:
			if !ok {
				return events, nil
			}
			return events, err
		case event, ok := <-b.watcher.Events:
			if !ok {
				return events, nil
			}
			if ignoreSyncWatchEvent(event) {
				continue
			}
			events = append(events, syncWatchEvent{Path: event.Name, Op: event.Op.String()})
			b.addNewDirectoryIfNeeded(event)
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(settle)
		case <-timer.C:
			return events, nil
		}
	}
}

func (b *syncWatchBackend) addNewDirectoryIfNeeded(event fsnotify.Event) {
	if event.Op&(fsnotify.Create|fsnotify.Rename) == 0 {
		return
	}
	info, err := os.Stat(event.Name)
	if err != nil || !info.IsDir() {
		return
	}
	_ = b.addRecursive(event.Name)
}

func (b *syncWatchBackend) addRecursive(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: sync watch root %q is a symlink", pathguard.ErrUnsafePath, root)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: sync watch root %q is not a directory", pathguard.ErrUnsafePath, root)
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return filepath.SkipDir
		}
		if _, ok := b.watchedDirs[path]; ok {
			return nil
		}
		if err := b.watcher.Add(path); err != nil {
			return err
		}
		b.watchedDirs[path] = struct{}{}
		return nil
	})
}

func ignoreSyncWatchEvent(event fsnotify.Event) bool {
	if event.Name == "" {
		return true
	}
	for _, segment := range strings.Split(filepath.ToSlash(filepath.Clean(event.Name)), "/") {
		if strings.EqualFold(segment, control.DirName) {
			return true
		}
	}
	return false
}

func waitSyncLoopInterval(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func daemonLocalPollingSyncConfig(p profile.Profile) (profile.LocalPollingSyncConfig, bool) {
	if p.Sync == nil || p.Sync.LocalPolling == nil || !p.Sync.LocalPolling.Enabled {
		return profile.LocalPollingSyncConfig{}, false
	}
	return *p.Sync.LocalPolling, true
}

func daemonNetworkPollingSyncConfig(p profile.Profile) (profile.NetworkPollingSyncConfig, bool) {
	if p.Sync == nil || p.Sync.NetworkPolling == nil || !p.Sync.NetworkPolling.Enabled {
		return profile.NetworkPollingSyncConfig{}, false
	}
	return *p.Sync.NetworkPolling, true
}

func daemonDriftRecordingConfig(p profile.Profile) (profile.DriftRecordingRepairConfig, bool) {
	if p.Repair == nil || p.Repair.DriftRecording == nil || !p.Repair.DriftRecording.Enabled {
		return profile.DriftRecordingRepairConfig{}, false
	}
	return *p.Repair.DriftRecording, true
}

func daemonPersistedReconcileApplyConfig(p profile.Profile) (profile.PersistedReconcileApplyRepairConfig, bool) {
	if p.Repair == nil || p.Repair.PersistedReconcileApply == nil || !p.Repair.PersistedReconcileApply.Enabled {
		return profile.PersistedReconcileApplyRepairConfig{}, false
	}
	return *p.Repair.PersistedReconcileApply, true
}

func (r Runner) runDaemonLocalPollingSync(ctx context.Context, p profile.Profile, targetDir string, scheduler *incrementalsync.Scheduler, config profile.LocalPollingSyncConfig, nextRunNumber func() int, stdout io.Writer, stderr io.Writer, outputMu *sync.Mutex) {
	if scheduler == nil || nextRunNumber == nil {
		return
	}
	interval := time.Duration(config.IntervalMillis) * time.Millisecond
	retryBackoff := time.Duration(config.RetryBackoffMillis) * time.Millisecond
	appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_sync_started", "profile-backed local polling sync started", map[string]string{
		"mode":            "local_polling",
		"session_prefix":  config.SessionPrefix,
		"interval_millis": strconv.Itoa(config.IntervalMillis),
	}, r.nowFunc()())
	defer appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_sync_stopped", "profile-backed local polling sync stopped", map[string]string{
		"mode":           "local_polling",
		"session_prefix": config.SessionPrefix,
	}, r.nowFunc()())
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		runNumber := nextRunNumber()
		sessionID := syncLoopSessionID(config.SessionPrefix, runNumber)
		runResult, err := r.runSyncPass(ctx, p, targetDir, scheduler, sessionID, retryBackoff, "daemon_foreground_local_polling")
		if err != nil {
			appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_sync_pass_failed", "profile-backed local polling sync pass failed", map[string]string{
				"error_class": daemonErrorClass(err),
			}, r.nowFunc()())
			writeLocked(outputMu, stderr, "daemon run: local polling sync: %s\n", safeDiagnosticLine(err.Error()))
			return
		}
		appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_sync_pass", "profile-backed local polling sync pass completed", map[string]string{
			"session": sessionID,
			"status":  runResult.Run.Status,
		}, r.nowFunc()())
		writeLocked(outputMu, stdout, "daemon: sync_local_polling profile=%s target=%s session=%s status=%s published=%d retried=%d queued=%d backoff=%d failed=%d\n",
			encodeTextValue(p.ProfileID),
			encodeTextValue(p.Target.TargetID),
			encodeTextValue(sessionID),
			encodeTextValue(runResult.Run.Status),
			len(runResult.Run.Published),
			len(runResult.Run.Retried),
			runResult.Run.Summary.Queued,
			runResult.Run.Summary.Backoff,
			runResult.Run.Summary.Failed,
		)
		if err := waitSyncLoopInterval(ctx, interval); err != nil {
			return
		}
	}
}

func (r Runner) runDaemonDriftRecording(ctx context.Context, p profile.Profile, targetDir string, config profile.DriftRecordingRepairConfig, stdout io.Writer, stderr io.Writer, outputMu *sync.Mutex) {
	interval := time.Duration(config.IntervalMillis) * time.Millisecond
	appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_drift_recording_started", "profile-backed drift recording started", map[string]string{
		"interval_millis": strconv.Itoa(config.IntervalMillis),
		"repair_applied":  "false",
	}, r.nowFunc()())
	defer appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_drift_recording_stopped", "profile-backed drift recording stopped", map[string]string{
		"repair_applied": "false",
	}, r.nowFunc()())
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		result, err := driftreview.Record(driftreview.RecordOptions{
			Profile: p,
			Now:     r.nowFunc()(),
		})
		if err != nil {
			appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_drift_recording_failed", "profile-backed drift recording pass failed", map[string]string{
				"error_class":    daemonErrorClass(err),
				"repair_applied": "false",
			}, r.nowFunc()())
			writeLocked(outputMu, stderr, "daemon run: drift recording: %s\n", safeDiagnosticLine(err.Error()))
			return
		}
		appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_drift_recording_pass", "profile-backed drift recording pass completed", map[string]string{
			"detected":          strconv.Itoa(result.Detected),
			"recorded":          strconv.Itoa(result.Recorded),
			"existing":          strconv.Itoa(result.Existing),
			"reopened":          strconv.Itoa(result.Reopened),
			"artifact_problems": strconv.Itoa(len(result.ArtifactProblems)),
			"manifest_count":    strconv.Itoa(result.ManifestCount),
			"repair_applied":    "false",
		}, r.nowFunc()())
		writeLocked(outputMu, stdout, "daemon: drift_recording profile=%s target=%s detected=%d recorded=%d existing=%d reopened=%d artifact_problems=%d manifest_count=%d repair=not_applied reconcile=not_applied\n",
			encodeTextValue(p.ProfileID),
			encodeTextValue(p.Target.TargetID),
			result.Detected,
			result.Recorded,
			result.Existing,
			result.Reopened,
			len(result.ArtifactProblems),
			result.ManifestCount,
		)
		if err := waitSyncLoopInterval(ctx, interval); err != nil {
			return
		}
	}
}

func (r Runner) runDaemonPersistedReconcileApply(ctx context.Context, p profile.Profile, targetDir string, config profile.PersistedReconcileApplyRepairConfig, stdout io.Writer, stderr io.Writer, outputMu *sync.Mutex) {
	interval := time.Duration(config.IntervalMillis) * time.Millisecond
	appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_reconcile_apply_started", "profile-backed persisted reconcile apply started", map[string]string{
		"interval_millis": strconv.Itoa(config.IntervalMillis),
		"selection":       "persisted_planned",
	}, r.nowFunc()())
	defer appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_reconcile_apply_stopped", "profile-backed persisted reconcile apply stopped", map[string]string{
		"selection": "persisted_planned",
	}, r.nowFunc()())
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		now := r.nowFunc()()
		ids, err := persistedPlannedReconcileIDs(p, "", now)
		if err != nil {
			appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_reconcile_apply_failed", "profile-backed persisted reconcile apply selection failed", map[string]string{
				"selection":    "persisted_planned",
				"error_class":  daemonErrorClass(err),
				"apply_called": "false",
			}, r.nowFunc()())
			writeLocked(outputMu, stderr, "daemon run: reconcile apply: %s\n", safeDiagnosticLine(err.Error()))
			return
		}
		if len(ids) == 0 {
			appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_reconcile_apply_pass", "profile-backed persisted reconcile apply pass found no planned actions", map[string]string{
				"selection": "persisted_planned",
				"planned":   "0",
				"applied":   "0",
				"refused":   "0",
			}, r.nowFunc()())
			writeLocked(outputMu, stdout, "daemon: reconcile_apply profile=%s target=%s selection=persisted_planned planned=0 applied=0 noop=0 refused=0 artifact_problems=0 status=noop\n",
				encodeTextValue(p.ProfileID),
				encodeTextValue(p.Target.TargetID),
			)
			if err := waitSyncLoopInterval(ctx, interval); err != nil {
				return
			}
			continue
		}
		receipt, err := reconcile.Apply(reconcile.ApplyOptions{
			Profile:  p,
			IDs:      ids,
			Apply:    true,
			Reviewer: config.Reviewer,
			Reason:   config.Reason,
			Now:      now,
		})
		if err != nil {
			appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_reconcile_apply_failed", "profile-backed persisted reconcile apply failed", map[string]string{
				"selection":   "persisted_planned",
				"error_class": daemonErrorClass(err),
			}, r.nowFunc()())
			writeLocked(outputMu, stderr, "daemon run: reconcile apply: %s\n", safeDiagnosticLine(err.Error()))
			return
		}
		appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_reconcile_apply_pass", "profile-backed persisted reconcile apply pass completed", map[string]string{
			"selection":          "persisted_planned",
			"planned":            strconv.Itoa(receipt.Summary.Planned),
			"applied":            strconv.Itoa(receipt.Summary.Applied),
			"noop":               strconv.Itoa(receipt.Summary.Noop),
			"refused":            strconv.Itoa(receipt.Summary.Refused),
			"artifact_problems":  strconv.Itoa(receipt.Summary.ArtifactProblems),
			"receipt_status":     receipt.Status,
			"receipt_id":         statusTextValueOrDash(receipt.ID),
			"live_record_called": "false",
		}, r.nowFunc()())
		writeLocked(outputMu, stdout, "daemon: reconcile_apply profile=%s target=%s selection=persisted_planned receipt=%s status=%s planned=%d applied=%d noop=%d refused=%d artifact_problems=%d live_record=not_called manifest_rewrite=not_applied prune=not_applied\n",
			encodeTextValue(p.ProfileID),
			encodeTextValue(p.Target.TargetID),
			encodeTextValue(statusTextValueOrDash(receipt.ID)),
			encodeTextValue(statusTextValueOrDash(receipt.Status)),
			receipt.Summary.Planned,
			receipt.Summary.Applied,
			receipt.Summary.Noop,
			receipt.Summary.Refused,
			receipt.Summary.ArtifactProblems,
		)
		if receipt.Summary.Refused > 0 || receipt.Summary.ArtifactProblems > 0 {
			writeLocked(outputMu, stderr, "daemon run: reconcile apply: stopped after refused persisted planned action; inspect receipt=%s before retry\n", safeDiagnosticLine(statusTextValueOrDash(receipt.ID)))
			return
		}
		if err := waitSyncLoopInterval(ctx, interval); err != nil {
			return
		}
	}
}

func (r Runner) runDaemonNetworkPollingSync(ctx context.Context, p profile.Profile, targetDir string, scheduler *incrementalsync.Scheduler, config profile.NetworkPollingSyncConfig, trust pairing.TrustState, nextRunNumber func() int, stdout io.Writer, stderr io.Writer, outputMu *sync.Mutex) {
	if scheduler == nil || nextRunNumber == nil {
		return
	}
	interval := time.Duration(config.IntervalMillis) * time.Millisecond
	retryBackoff := time.Duration(config.RetryBackoffMillis) * time.Millisecond
	appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_sync_started", "profile-backed network polling sync started", map[string]string{
		"mode":            "network_polling",
		"session_prefix":  config.SessionPrefix,
		"interval_millis": strconv.Itoa(config.IntervalMillis),
	}, r.nowFunc()())
	defer appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_sync_stopped", "profile-backed network polling sync stopped", map[string]string{
		"mode":           "network_polling",
		"session_prefix": config.SessionPrefix,
	}, r.nowFunc()())
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		runNumber := nextRunNumber()
		sessionID := syncLoopSessionID(config.SessionPrefix, runNumber)
		runResult, err := r.runSyncNetworkPass(ctx, p, scheduler, trust, sessionID, retryBackoff, "daemon_foreground_network_polling")
		if err != nil {
			appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_sync_pass_failed", "profile-backed network polling sync pass failed", map[string]string{
				"mode":        "network_polling",
				"error_class": daemonErrorClass(err),
			}, r.nowFunc()())
			writeLocked(outputMu, stderr, "daemon run: network polling sync: %s\n", safeDiagnosticLine(err.Error()))
			return
		}
		appendDaemonLifecycleEvent(targetDir, p.ProfileID, p.Target.TargetID, "daemon_sync_pass", "profile-backed network polling sync pass completed", map[string]string{
			"mode":             "network_polling",
			"session":          sessionID,
			"status":           runResult.Run.Status,
			"network_transfer": runResult.Network.Transfer,
		}, r.nowFunc()())
		writeLocked(outputMu, stdout, "daemon: sync_network_polling profile=%s target=%s session=%s status=%s network_transfer=%s published=%d retried=%d queued=%d backoff=%d failed=%d\n",
			encodeTextValue(p.ProfileID),
			encodeTextValue(p.Target.TargetID),
			encodeTextValue(sessionID),
			encodeTextValue(runResult.Run.Status),
			encodeTextValue(runResult.Network.Transfer),
			len(runResult.Run.Published),
			len(runResult.Run.Retried),
			runResult.Run.Summary.Queued,
			runResult.Run.Summary.Backoff,
			runResult.Run.Summary.Failed,
		)
		if err := waitSyncLoopInterval(ctx, interval); err != nil {
			return
		}
	}
}

func (r Runner) daemonLocalPollingLastRunNumber(p profile.Profile, config profile.LocalPollingSyncConfig) (int, error) {
	stateDir, err := incrementalQueueStateDir(p, false)
	if err != nil {
		return 0, err
	}
	scheduler, err := incrementalsync.Open(incrementalsync.Options{StateDir: stateDir, Now: r.nowFunc()})
	if err != nil {
		return 0, err
	}
	return lastSyncRunNumber(scheduler, incrementalQueueScope(p), config.SessionPrefix)
}

func (r Runner) daemonNetworkPollingLastRunNumber(p profile.Profile, config profile.NetworkPollingSyncConfig) (int, error) {
	stateDir, err := incrementalQueueStateDir(p, false)
	if err != nil {
		return 0, err
	}
	scheduler, err := incrementalsync.Open(incrementalsync.Options{StateDir: stateDir, Now: r.nowFunc()})
	if err != nil {
		return 0, err
	}
	return lastSyncRunNumber(scheduler, incrementalQueueScope(p), config.SessionPrefix)
}

func lastSyncRunNumber(scheduler *incrementalsync.Scheduler, scope incrementalsync.Scope, sessionPrefix string) (int, error) {
	runs, problems, err := scheduler.RunResults(scope)
	if err != nil {
		return 0, err
	}
	if len(problems) != 0 {
		return 0, fmt.Errorf("incremental sync run receipt has artifact problems: %s", problems[0].Error)
	}
	last := 0
	prefix := sessionPrefix + "-"
	for _, run := range runs {
		suffix, ok := strings.CutPrefix(run.SessionID, prefix)
		if !ok {
			continue
		}
		number, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		if number > last {
			last = number
		}
	}
	return last, nil
}

func writeLocked(mu *sync.Mutex, w io.Writer, format string, args ...any) {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	fmt.Fprintf(w, format, args...)
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
	profilePath := fs.String("profile", "", "--profile profile path; profile remains the daemon SSOT")
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
	profilePath := fs.String("profile", "", "--profile profile path; profile remains the daemon SSOT")
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
	syncRunNumber := 0
	nextSyncRunNumber := func() int {
		syncRunNumber++
		return syncRunNumber
	}
	networkSyncRunNumber := 0
	nextNetworkSyncRunNumber := func() int {
		networkSyncRunNumber++
		return networkSyncRunNumber
	}
	for {
		networkSyncConfig, networkSyncEnabled := daemonNetworkPollingSyncConfig(p)
		if networkSyncEnabled {
			trust, err := validateSyncNetworkRunProfile(p, r.nowFunc())
			if err != nil {
				writeDaemonFailedState(targetDir, agentdaemon.NewState(initialProfileID, initialTargetID, cleanProfilePath, agentdaemon.StatusFailed, os.Getpid(), r.nowFunc()()), err.Error(), r.nowFunc()())
				appendDaemonLifecycleEvent(targetDir, initialProfileID, initialTargetID, "daemon_failed", "network polling sync profile validation failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
				fmt.Fprintf(stderr, "daemon run: network polling sync: %s\n", safeDiagnosticLine(err.Error()))
				return 2
			}
			if networkSyncRunNumber == 0 {
				lastRunNumber, err := r.daemonNetworkPollingLastRunNumber(p, networkSyncConfig)
				if err != nil {
					writeDaemonFailedState(targetDir, agentdaemon.NewState(initialProfileID, initialTargetID, cleanProfilePath, agentdaemon.StatusFailed, os.Getpid(), r.nowFunc()()), err.Error(), r.nowFunc()())
					appendDaemonLifecycleEvent(targetDir, initialProfileID, initialTargetID, "daemon_failed", "network polling sync receipt scan failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
					fmt.Fprintf(stderr, "daemon run: network polling sync: %s\n", safeDiagnosticLine(err.Error()))
					return 1
				}
				networkSyncRunNumber = lastRunNumber
			}
			result := r.runDaemonNetworkPollingCycle(ctx, p, targetDir, cleanProfilePath, cleanupOldStopIntent, networkSyncConfig, trust, nextNetworkSyncRunNumber, stdout, stderr, outputMu)
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
			continue
		}
		enableReceiver, err := validateDaemonReceiverMode(p)
		if err != nil {
			writeDaemonFailedState(targetDir, agentdaemon.NewState(initialProfileID, initialTargetID, cleanProfilePath, agentdaemon.StatusFailed, os.Getpid(), r.nowFunc()()), err.Error(), r.nowFunc()())
			appendDaemonLifecycleEvent(targetDir, initialProfileID, initialTargetID, "daemon_failed", "profile validation failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
			fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		syncConfig, syncEnabled := daemonLocalPollingSyncConfig(p)
		driftRecordingConfig, driftRecordingEnabled := daemonDriftRecordingConfig(p)
		reconcileApplyConfig, reconcileApplyEnabled := daemonPersistedReconcileApplyConfig(p)
		if syncEnabled && syncRunNumber == 0 {
			lastRunNumber, err := r.daemonLocalPollingLastRunNumber(p, syncConfig)
			if err != nil {
				writeDaemonFailedState(targetDir, agentdaemon.NewState(initialProfileID, initialTargetID, cleanProfilePath, agentdaemon.StatusFailed, os.Getpid(), r.nowFunc()()), err.Error(), r.nowFunc()())
				appendDaemonLifecycleEvent(targetDir, initialProfileID, initialTargetID, "daemon_failed", "local polling sync receipt scan failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
				fmt.Fprintf(stderr, "daemon run: local polling sync: %s\n", safeDiagnosticLine(err.Error()))
				return 1
			}
			syncRunNumber = lastRunNumber
		}
		result := r.runDaemonForegroundCycle(ctx, p, targetDir, cleanProfilePath, *listen, enableReceiver, cleanupOldStopIntent, syncConfig, syncEnabled, nextSyncRunNumber, driftRecordingConfig, driftRecordingEnabled, reconcileApplyConfig, reconcileApplyEnabled, stdout, stderr, outputMu)
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

func (r Runner) runDaemonForegroundCycle(ctx context.Context, p profile.Profile, targetDir, cleanProfilePath, listen string, enableReceiver bool, cleanupOldStopIntent bool, syncConfig profile.LocalPollingSyncConfig, syncEnabled bool, nextSyncRunNumber func() int, driftRecordingConfig profile.DriftRecordingRepairConfig, driftRecordingEnabled bool, reconcileApplyConfig profile.PersistedReconcileApplyRepairConfig, reconcileApplyEnabled bool, stdout io.Writer, stderr io.Writer, outputMu *sync.Mutex) daemonRunCycleResult {
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
	var syncScheduler *incrementalsync.Scheduler
	if syncEnabled {
		stateDir, err := incrementalQueueStateDir(p, true)
		if err != nil {
			writeDaemonFailedState(targetDir, state, err.Error(), r.nowFunc()())
			appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_failed", "local polling sync state setup failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
			fmt.Fprintf(stderr, "daemon run: local polling sync: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: state, exitCode: 1}
		}
		scheduler, err := incrementalsync.New(incrementalsync.Options{StateDir: stateDir, Now: r.nowFunc()})
		if err != nil {
			writeDaemonFailedState(targetDir, state, err.Error(), r.nowFunc()())
			appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_failed", "local polling sync scheduler setup failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
			fmt.Fprintf(stderr, "daemon run: local polling sync: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: state, exitCode: 1}
		}
		syncScheduler = scheduler
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
	pairingServer, err := serveRunner.newPairingServe(p, listen, stderr, outputMu, enableReceiver, nil)
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
		receiver, err := serveRunner.newReceiverServe(p, stderr, outputMu, nil)
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
	if r.DaemonReady != nil {
		r.DaemonReady(state)
	}
	fmt.Fprintf(stdout, "daemon: running profile=%s target=%s state=running mode=%s pairing_address=%s receiver_address=%s foreground=true service_manager=none\n",
		encodeTextValue(state.ProfileID),
		encodeTextValue(state.TargetID),
		encodeTextValue(state.Mode),
		encodeTextValue(statusTextValueOrDash(state.PairingAddress)),
		encodeTextValue(statusTextValueOrDash(state.ReceiverAddress)),
	)
	var workers sync.WaitGroup
	if syncEnabled {
		workers.Add(1)
		go func() {
			defer workers.Done()
			r.runDaemonLocalPollingSync(serveCtx, p, targetDir, syncScheduler, syncConfig, nextSyncRunNumber, stdout, stderr, outputMu)
		}()
	}
	if driftRecordingEnabled {
		workers.Add(1)
		go func() {
			defer workers.Done()
			r.runDaemonDriftRecording(serveCtx, p, targetDir, driftRecordingConfig, stdout, stderr, outputMu)
		}()
	}
	if reconcileApplyEnabled {
		workers.Add(1)
		go func() {
			defer workers.Done()
			r.runDaemonPersistedReconcileApply(serveCtx, p, targetDir, reconcileApplyConfig, stdout, stderr, outputMu)
		}()
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
	workers.Wait()
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

func (r Runner) runDaemonNetworkPollingCycle(ctx context.Context, p profile.Profile, targetDir, cleanProfilePath string, cleanupOldStopIntent bool, syncConfig profile.NetworkPollingSyncConfig, trust pairing.TrustState, nextSyncRunNumber func() int, stdout io.Writer, stderr io.Writer, outputMu *sync.Mutex) daemonRunCycleResult {
	workerCtx, stopWorker := context.WithCancel(ctx)
	defer stopWorker()
	now := r.nowFunc()()
	state := agentdaemon.NewState(p.ProfileID, p.Target.TargetID, cleanProfilePath, agentdaemon.StatusStarting, os.Getpid(), now)
	state.Mode = "network-polling"
	appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_starting", "foreground daemon starting network polling sync", map[string]string{
		"mode":            state.Mode,
		"run_mode":        agentdaemon.RunModeForeground,
		"service_manager": agentdaemon.ServiceManagerNone,
	}, now)
	if err := writeLockedDaemonState(targetDir, state); err != nil {
		fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
		return daemonRunCycleResult{state: state, exitCode: 1}
	}
	stateDir, err := incrementalQueueStateDir(p, true)
	if err != nil {
		writeDaemonFailedState(targetDir, state, err.Error(), r.nowFunc()())
		appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_failed", "network polling sync state setup failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
		fmt.Fprintf(stderr, "daemon run: network polling sync: %s\n", safeDiagnosticLine(err.Error()))
		return daemonRunCycleResult{state: state, exitCode: 1}
	}
	scheduler, err := incrementalsync.New(incrementalsync.Options{StateDir: stateDir, Now: r.nowFunc()})
	if err != nil {
		writeDaemonFailedState(targetDir, state, err.Error(), r.nowFunc()())
		appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_failed", "network polling sync scheduler setup failed", map[string]string{"error_class": daemonErrorClass(err)}, r.nowFunc()())
		fmt.Fprintf(stderr, "daemon run: network polling sync: %s\n", safeDiagnosticLine(err.Error()))
		return daemonRunCycleResult{state: state, exitCode: 1}
	}
	if cleanupOldStopIntent {
		if err := removeScopedDaemonStopIntentBefore(targetDir, state.ProfileID, state.TargetID, now); err != nil {
			fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: state, exitCode: 1}
		}
	}
	state.Status = agentdaemon.StatusRunning
	state.UpdatedAt = r.nowFunc()().Format(time.RFC3339Nano)
	if err := writeLockedDaemonState(targetDir, state); err != nil {
		fmt.Fprintf(stderr, "daemon run: %s\n", safeDiagnosticLine(err.Error()))
		return daemonRunCycleResult{state: state, exitCode: 1}
	}
	appendDaemonLifecycleEvent(targetDir, state.ProfileID, state.TargetID, "daemon_running", "foreground daemon network polling sync is running", map[string]string{
		"mode":           state.Mode,
		"session_prefix": syncConfig.SessionPrefix,
	}, r.nowFunc()())
	if r.DaemonReady != nil {
		r.DaemonReady(state)
	}
	fmt.Fprintf(stdout, "daemon: running profile=%s target=%s state=running mode=%s pairing_address=- receiver_address=- foreground=true service_manager=none\n",
		encodeTextValue(state.ProfileID),
		encodeTextValue(state.TargetID),
		encodeTextValue(state.Mode),
	)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		r.runDaemonNetworkPollingSync(workerCtx, p, targetDir, scheduler, syncConfig, trust, nextSyncRunNumber, stdout, stderr, outputMu)
	}()
	stopPoll := make(chan struct{})
	stopIntentCh := make(chan agentdaemon.StopIntent, 1)
	restartPoll := make(chan struct{})
	restartIntentCh := make(chan agentdaemon.RestartIntent, 1)
	go pollDaemonStopIntent(workerCtx, targetDir, state.ProfileID, state.TargetID, stopPoll, stopIntentCh, stopWorker)
	go pollDaemonRestartIntent(workerCtx, targetDir, state.ProfileID, state.TargetID, restartPoll, restartIntentCh, stopWorker)
	workerStoppedUnexpectedly := false
	select {
	case <-workerCtx.Done():
	case <-workerDone:
		workerStoppedUnexpectedly = true
		stopWorker()
	}
	close(stopPoll)
	close(restartPoll)
	<-workerDone
	now = r.nowFunc()()
	finalState := state
	finalState.PID = os.Getpid()
	finalState.UpdatedAt = now.Format(time.RFC3339Nano)
	if intent, ok := receiveDaemonStopIntent(stopIntentCh); ok {
		finalState.Status = agentdaemon.StatusStopped
		finalState.StoppedAt = now.Format(time.RFC3339Nano)
		finalState.StopIntent = ptr(agentdaemon.StopSummary(intent))
		if err := writeLockedDaemonState(targetDir, finalState); err != nil {
			fmt.Fprintf(stderr, "daemon run: write stopped state: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: finalState, exitCode: 1}
		}
		appendDaemonLifecycleEvent(targetDir, finalState.ProfileID, finalState.TargetID, "daemon_stopped", "foreground daemon stopped from stop intent", map[string]string{"reason": intent.Reason, "mode": "network_polling"}, now)
		return daemonRunCycleResult{state: finalState, exitCode: 0}
	}
	if intent, ok := receiveDaemonRestartIntent(restartIntentCh); ok {
		finalState.Status = agentdaemon.StatusStarting
		if err := withLockedTarget(targetDir, func() error {
			if err := agentdaemon.WriteState(targetDir, finalState); err != nil {
				return err
			}
			return agentdaemon.RemoveRestartIntent(targetDir)
		}); err != nil {
			fmt.Fprintf(stderr, "daemon run: write restarting state: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: finalState, exitCode: 1}
		}
		appendDaemonLifecycleEvent(targetDir, finalState.ProfileID, finalState.TargetID, "daemon_restart_consumed", "foreground daemon consumed restart intent", map[string]string{"reason": intent.Reason, "mode": "network_polling"}, now)
		if r.DaemonRestartConsumed != nil {
			r.DaemonRestartConsumed(finalState)
		}
		return daemonRunCycleResult{state: finalState, exitCode: 0, restart: true}
	}
	if err := ctx.Err(); err != nil {
		finalState.Status = agentdaemon.StatusStopped
		finalState.StoppedAt = now.Format(time.RFC3339Nano)
		if err := writeLockedDaemonState(targetDir, finalState); err != nil {
			fmt.Fprintf(stderr, "daemon run: write stopped state: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: finalState, exitCode: 1}
		}
		appendDaemonLifecycleEvent(targetDir, finalState.ProfileID, finalState.TargetID, "daemon_stopped", "foreground daemon stopped", map[string]string{"mode": "network_polling"}, now)
		return daemonRunCycleResult{state: finalState, exitCode: 0}
	}
	if !workerStoppedUnexpectedly {
		finalState.Status = agentdaemon.StatusStopped
		finalState.StoppedAt = now.Format(time.RFC3339Nano)
		if err := writeLockedDaemonState(targetDir, finalState); err != nil {
			fmt.Fprintf(stderr, "daemon run: write stopped state: %s\n", safeDiagnosticLine(err.Error()))
			return daemonRunCycleResult{state: finalState, exitCode: 1}
		}
		appendDaemonLifecycleEvent(targetDir, finalState.ProfileID, finalState.TargetID, "daemon_stopped", "foreground daemon stopped", map[string]string{"mode": "network_polling"}, now)
		return daemonRunCycleResult{state: finalState, exitCode: 0}
	}
	finalState.Status = agentdaemon.StatusFailed
	finalState.LastError = "network polling sync stopped unexpectedly"
	if err := writeLockedDaemonState(targetDir, finalState); err != nil {
		fmt.Fprintf(stderr, "daemon run: write failed state: %s\n", safeDiagnosticLine(err.Error()))
		return daemonRunCycleResult{state: finalState, exitCode: 1}
	}
	appendDaemonLifecycleEvent(targetDir, finalState.ProfileID, finalState.TargetID, "daemon_failed", "network polling sync stopped unexpectedly", map[string]string{"mode": "network_polling"}, now)
	fmt.Fprintln(stderr, "daemon run: network polling sync stopped unexpectedly")
	return daemonRunCycleResult{state: finalState, exitCode: 1}
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

func (r Runner) runDiscover(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "help":
			printDiscoverUsage(stdout)
			return 0
		case "browse":
			return r.runDiscoverBrowse(args[1:], stdout, stderr)
		case "advertise":
			return r.runDiscoverAdvertise(args[1:], stdout, stderr)
		}
	}
	fs := newFlagSet("discover", stderr)
	fs.Usage = func() { printDiscoverUsage(fs.Output()) }
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
		if _, err := discovery.NewAddressHint(address, discovery.NewLowInfoAdvertisement(pairserve.ServiceType, protocol.Version, "abcdef0123456789", []string{"pair"}), time.Unix(0, 0).UTC(), discovery.DefaultHintTTL); err != nil {
			fmt.Fprintf(stderr, "discover: invalid address hint %q\n", address)
			return 2
		}
	}
	now := r.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ctx, cancel := context.WithTimeout(r.baseContext(), timeoutValue)
	defer cancel()
	source := discovery.Source(discovery.EmptySource{})
	if len(addresses) > 0 {
		source = discovery.StaticSource{
			Addresses:       addresses,
			ServiceType:     pairserve.ServiceType,
			ProtocolVersion: protocol.Version,
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

func (r Runner) runDiscoverBrowse(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("discover browse", stderr)
	listen := fs.String("listen", defaultDiscoveryBrowseListen, "--listen UDP host:port for low-information LAN advertisements")
	timeout := fs.String("timeout", "2s", "--timeout for LAN browse; discovery is not trust")
	format := fs.String("format", "text", "output format: text or json")
	strict := fs.Bool("strict", false, "--strict fails on malformed or high-information datagrams instead of dropping them")
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
		fmt.Fprintf(stderr, "discover browse: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	timeoutValue, err := time.ParseDuration(*timeout)
	if err != nil || timeoutValue <= 0 {
		fmt.Fprintf(stderr, "discover browse: invalid --timeout %q\n", *timeout)
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "discover browse: unsupported format %q\n", *format)
		return 2
	}
	conn, err := listenDiscoveryUDP(*listen)
	if err != nil {
		if errors.Is(err, errInvalidDiscoveryAddress) {
			fmt.Fprintf(stderr, "discover browse: invalid --listen %q\n", *listen)
			return 2
		}
		fmt.Fprintf(stderr, "discover browse: listen: %v\n", err)
		return 1
	}
	defer conn.Close()
	if r.DiscoverBrowseReady != nil {
		r.DiscoverBrowseReady(conn.LocalAddr().String())
	}
	now := r.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ctx, cancel := context.WithTimeout(r.baseContext(), timeoutValue)
	defer cancel()
	invalidPackets := 0
	candidates, err := discovery.Browse(ctx, discovery.DatagramSource{
		Conn:            conn,
		ServiceType:     pairserve.ServiceType,
		ProtocolVersion: protocol.Version,
		TTL:             discovery.DefaultHintTTL,
		Strict:          *strict,
		InvalidPackets:  &invalidPackets,
	}, now)
	if err != nil {
		fmt.Fprintf(stderr, "discover browse: %v\n", err)
		return 1
	}
	result := discoveryBrowseResult{
		Source:         "lan_datagram",
		Listen:         conn.LocalAddr().String(),
		CandidateCount: len(candidates),
		InvalidPackets: invalidPackets,
		Trusted:        false,
		Candidates:     candidates,
	}
	switch *format {
	case "text":
		printDiscoveryBrowseText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "discover browse: encode candidates: %v\n", err)
			return 1
		}
	}
	return 0
}

func (r Runner) runDiscoverAdvertise(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("discover advertise", stderr)
	profilePath := fs.String("profile", "", "--profile target profile path; profile privacy policy is the advertisement SSOT")
	listen := fs.String("listen", defaultDiscoveryAdvertiseListen, "--listen UDP host:port for the advertised source address")
	dest := fs.String("dest", defaultDiscoveryAdvertiseDestination, "--dest UDP host:port that receives low-information advertisements")
	interval := fs.String("interval", defaultDiscoveryAdvertiseInterval.String(), "--interval between advertisement datagrams")
	duration := fs.String("duration", defaultDiscoveryAdvertiseDuration.String(), "--duration to advertise before exiting")
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
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "discover advertise: --profile is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "discover advertise: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	durationValue, err := time.ParseDuration(*duration)
	if err != nil || durationValue <= 0 {
		fmt.Fprintf(stderr, "discover advertise: invalid --duration %q\n", *duration)
		return 2
	}
	intervalValue, err := time.ParseDuration(*interval)
	if err != nil || intervalValue <= 0 {
		fmt.Fprintf(stderr, "discover advertise: invalid --interval %q\n", *interval)
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "discover advertise: unsupported format %q\n", *format)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "discover advertise: %v\n", err)
		return 2
	}
	if !p.PrivacyPolicy.DiscoveryLowInfo {
		fmt.Fprintln(stderr, "discover advertise: privacy_policy.discovery_low_info must be true for LAN advertisement")
		return 2
	}
	if p.Discovery != nil && p.Discovery.AdvertiseReceiverHint {
		trust, err := pairing.ValidateProfileTrust(p)
		if err != nil {
			fmt.Fprintf(stderr, "discover advertise: %v\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		if err := p.ValidateNetworkServerMaterial(); err != nil {
			fmt.Fprintf(stderr, "discover advertise: %v\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		if err := tlsidentity.ValidatePinned(p.Network.LocalTLSIdentity, trust.TargetDeviceID, r.nowFunc()); err != nil {
			fmt.Fprintf(stderr, "discover advertise: validate local TLS identity files: %v\n", safeDiagnosticLine(err.Error()))
			return 2
		}
		receiverAddress, err := receiverURLHostPort(p)
		if err != nil {
			fmt.Fprintf(stderr, "discover advertise: %v\n", err)
			return 2
		}
		if flagProvided(fs, "listen") && strings.TrimSpace(*listen) != receiverAddress {
			fmt.Fprintln(stderr, "discover advertise: --listen must match profile network.receiver_url when discovery.advertise_receiver_hint is true")
			return 2
		}
		*listen = receiverAddress
	}
	destination, err := resolveDiscoveryUDPAddr(*dest)
	if err != nil {
		fmt.Fprintf(stderr, "discover advertise: invalid --dest %q\n", *dest)
		return 2
	}
	conn, err := listenDiscoveryUDP(*listen)
	if err != nil {
		if errors.Is(err, errInvalidDiscoveryAddress) {
			fmt.Fprintf(stderr, "discover advertise: invalid --listen %q\n", *listen)
			return 2
		}
		fmt.Fprintf(stderr, "discover advertise: listen: %v\n", err)
		return 1
	}
	defer conn.Close()
	if err := enableUDPBroadcast(conn); err != nil {
		fmt.Fprintf(stderr, "discover advertise: enable broadcast: %v\n", err)
		return 1
	}
	now := r.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ad := discovery.NewLowInfoAdvertisement(pairserve.ServiceType, protocol.Version, deterministicLANDiscoveryNonce(now, p), discoveryCapabilitiesForProfile(p))
	if err := ad.Validate(); err != nil {
		fmt.Fprintf(stderr, "discover advertise: %v\n", err)
		return 2
	}
	if r.DiscoverAdvertiseReady != nil {
		r.DiscoverAdvertiseReady(conn.LocalAddr().String())
	}
	ctx, cancel := context.WithTimeout(r.baseContext(), durationValue)
	defer cancel()
	result := discoveryAdvertiseResult{
		Status:          "advertised",
		Listen:          conn.LocalAddr().String(),
		Destination:     destination.String(),
		ServiceType:     ad.ServiceType,
		ProtocolVersion: ad.ProtocolVersion,
		EphemeralNonce:  ad.EphemeralNonce,
		Capabilities:    sortedStrings(ad.CapabilityFlags),
		Trusted:         false,
		Duration:        durationValue.String(),
		Interval:        intervalValue.String(),
	}
	err = discovery.DatagramAdvertiser{
		Conn:          conn,
		Destination:   destination,
		Advertisement: ad,
		Interval:      intervalValue,
	}.Advertise(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "discover advertise: %v\n", err)
		return 1
	}
	switch *format {
	case "text":
		printDiscoveryAdvertiseText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "discover advertise: encode result: %v\n", err)
			return 1
		}
	}
	return 0
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
	SourceBaseline    string `json:"source_baseline,omitempty"`
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

type syncQueueFailResult struct {
	Operation string                     `json:"operation"`
	Mode      string                     `json:"mode"`
	State     string                     `json:"state"`
	Reason    string                     `json:"reason"`
	Entry     incrementalsync.QueueEntry `json:"entry"`
	Summary   incrementalsync.Summary    `json:"summary"`
}

type syncRunResult struct {
	Operation string                    `json:"operation"`
	Mode      string                    `json:"mode"`
	Enqueue   syncQueueEnqueueResult    `json:"enqueue"`
	Run       incrementalsync.RunResult `json:"run"`
}

type syncNetworkRunResult struct {
	Operation string                    `json:"operation"`
	Mode      string                    `json:"mode"`
	Enqueue   syncQueueEnqueueResult    `json:"enqueue"`
	Run       incrementalsync.RunResult `json:"run"`
	Network   networkPushPlan           `json:"network"`
}

type syncNetworkDiscoveryGate struct {
	Status         string   `json:"status"`
	Reason         string   `json:"reason"`
	ProfileAddress string   `json:"profile_address"`
	MatchedAddress string   `json:"matched_address,omitempty"`
	MatchedClass   string   `json:"matched_class,omitempty"`
	Capabilities   []string `json:"capability_flags,omitempty"`
	ExpiresAt      string   `json:"expires_at,omitempty"`
	CandidateCount int      `json:"candidate_count"`
	InvalidPackets int      `json:"invalid_packets"`
	Trusted        bool     `json:"trusted"`
}

type syncNetworkDiscoverRunResult struct {
	Operation string                    `json:"operation"`
	Mode      string                    `json:"mode"`
	Discovery syncNetworkDiscoveryGate  `json:"discovery"`
	Enqueue   syncQueueEnqueueResult    `json:"enqueue,omitempty"`
	Run       incrementalsync.RunResult `json:"run,omitempty"`
	Network   networkPushPlan           `json:"network,omitempty"`
}

type syncNetworkLoopResult struct {
	Operation               string                 `json:"operation"`
	Mode                    string                 `json:"mode"`
	SessionPrefix           string                 `json:"session_prefix"`
	Interval                string                 `json:"interval"`
	MaxRuns                 int                    `json:"max_runs"`
	Status                  string                 `json:"status"`
	CompletedRuns           int                    `json:"completed_runs"`
	PublishedRuns           int                    `json:"published_runs"`
	IdleRuns                int                    `json:"idle_runs"`
	RetryingRuns            int                    `json:"retrying_runs"`
	NetworkAttempts         int                    `json:"network_attempts"`
	NetworkPublishedRuns    int                    `json:"network_published_runs"`
	NetworkNotAttemptedRuns int                    `json:"network_not_attempted_runs"`
	Runs                    []syncNetworkRunResult `json:"runs,omitempty"`
}

type syncLoopResult struct {
	Operation     string          `json:"operation"`
	Mode          string          `json:"mode"`
	SessionPrefix string          `json:"session_prefix"`
	Interval      string          `json:"interval"`
	MaxRuns       int             `json:"max_runs"`
	Status        string          `json:"status"`
	CompletedRuns int             `json:"completed_runs"`
	PublishedRuns int             `json:"published_runs"`
	IdleRuns      int             `json:"idle_runs"`
	RetryingRuns  int             `json:"retrying_runs"`
	Runs          []syncRunResult `json:"runs,omitempty"`
}

type syncWatchResult struct {
	Operation     string          `json:"operation"`
	Mode          string          `json:"mode"`
	SessionPrefix string          `json:"session_prefix"`
	Settle        string          `json:"settle"`
	MaxEvents     int             `json:"max_events"`
	Status        string          `json:"status"`
	WatchedRoots  []string        `json:"watched_roots,omitempty"`
	WatchedDirs   int             `json:"watched_dirs"`
	EventBatches  int             `json:"event_batches"`
	EventsSeen    int             `json:"events_seen"`
	CompletedRuns int             `json:"completed_runs"`
	PublishedRuns int             `json:"published_runs"`
	IdleRuns      int             `json:"idle_runs"`
	RetryingRuns  int             `json:"retrying_runs"`
	Baseline      *syncRunResult  `json:"baseline,omitempty"`
	Runs          []syncRunResult `json:"runs,omitempty"`
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
	fmt.Fprintf(w, "network push: profile=%s target_id=%s source_device=%s target_device=%s pairing_receipt=%s session=%s dry_run=%t transfer=%s encrypted_transfer=%s resume=%s resume_authority=%s resume_outcome=%s resumed_bytes=%d files=%d bytes=%d chunks=%d warnings=%d status=%s stage=%s error_code=%s source_baseline=%s\n",
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
		encodeTextValue(defaultTextField(plan.SourceBaseline)),
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

func printSourceConsistencyText(w io.Writer, report sourceconsistency.Report) {
	fmt.Fprintf(w, "source_consistency status=%s mode=%s profile=%s session=%s entries=%d mismatches=%d root=%s\n",
		encodeTextValue(report.Status),
		encodeTextValue(report.Mode),
		encodeTextValue(report.ProfileID),
		encodeTextValue(defaultTextField(report.SessionID)),
		report.EntryCount,
		report.MismatchCount,
		encodeTextValue(report.RootPath),
	)
	for _, mismatch := range report.Mismatches {
		fmt.Fprintf(w, "mismatch path=%s kind=%s expected=%s actual=%s message=%s\n",
			encodeTextValue(mismatch.Path),
			encodeTextValue(mismatch.Kind),
			encodeTextValue(defaultTextField(mismatch.Expected)),
			encodeTextValue(defaultTextField(mismatch.Actual)),
			encodeTextValue(mismatch.Message),
		)
	}
}

func buildSourceConsistencyBaseline(p profile.Profile, sessionID string, now func() time.Time) (sourceconsistency.Baseline, []scan.Entry, error) {
	if len(p.Roots) != 1 {
		return sourceconsistency.Baseline{}, nil, fmt.Errorf("source consistency requires exactly one profile root")
	}
	result, err := scan.Scan(p.Roots[0].Path)
	if err != nil {
		return sourceconsistency.Baseline{}, nil, err
	}
	baseline, err := sourceconsistency.BuildBaseline(sourceconsistency.BuildBaselineOptions{
		Profile:   p,
		SessionID: sessionID,
		Now:       now,
		Entries:   result.Entries,
	})
	if err != nil {
		return sourceconsistency.Baseline{}, nil, err
	}
	entries := make([]scan.Entry, 0, len(baseline.Entries))
	for _, entry := range baseline.Entries {
		entries = append(entries, baselineEntryToScanEntry(entry))
	}
	return baseline, entries, nil
}

func baselineEntryToScanEntry(entry sourceconsistency.BaselineEntry) scan.Entry {
	switch entry.Kind {
	case "file":
		modTime, _ := time.Parse(time.RFC3339Nano, entry.ModTime)
		return scan.Entry{
			Path:       entry.Path,
			Kind:       scan.KindRegular,
			Size:       entry.Size,
			Mode:       fs.FileMode(entry.Mode),
			ModTime:    modTime.UTC(),
			Digest:     entry.Digest,
			Executable: entry.Mode&0o111 != 0,
		}
	case "dir":
		modTime, _ := time.Parse(time.RFC3339Nano, entry.ModTime)
		return scan.Entry{
			Path:    entry.Path,
			Kind:    scan.KindDir,
			Mode:    fs.ModeDir | fs.FileMode(entry.Mode),
			ModTime: modTime.UTC(),
		}
	case "symlink":
		return scan.Entry{
			Path:          entry.Path,
			Kind:          scan.KindSymlink,
			Mode:          os.ModeSymlink,
			SymlinkTarget: entry.SymlinkTarget,
		}
	default:
		return scan.Entry{Path: entry.Path}
	}
}

func writeSourceConsistencyBaseline(path string, baseline sourceconsistency.Baseline) error {
	if err := preflightOutputFile(path, "source consistency baseline"); err != nil {
		return err
	}
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return err
	}
	parent := filepath.Dir(path)
	temp, err := os.CreateTemp(parent, ".source-consistency-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := durable.PromoteFile(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func preflightOutputFile(path string, label string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("%s path is required", label)
	}
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s directory %q is a symlink", label, parent)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s directory %q is not a directory", label, parent)
	}
	if existing, err := os.Lstat(path); err == nil {
		if existing.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s %q is a symlink", label, path)
		}
		if existing.IsDir() {
			return fmt.Errorf("%s %q is a directory", label, path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
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
		label := "sync_queue_ready"
		entryLabel := "sync_queue_ready_entry"
		if result.Operation == "list" {
			label = "sync_queue_list"
			entryLabel = "sync_queue_entry"
		}
		printSyncQueueSummaryLine(stdout, label, result.Mode, result.State, result.Summary)
		for _, entry := range result.Entries {
			printSyncQueueEntry(stdout, entryLabel, entry)
		}
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			encodeErr := "sync queue ready"
			if result.Operation == "list" {
				encodeErr = "sync queue list"
			}
			fmt.Fprintf(stderr, "%s: encode result: %s\n", encodeErr, safeDiagnosticLine(err.Error()))
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

func printSyncQueueFailResult(stdout io.Writer, stderr io.Writer, format string, result syncQueueFailResult) int {
	switch format {
	case "text":
		fmt.Fprintf(stdout, "sync_queue_fail mode=%s state=%s id=%s status=%s reason=%s\n",
			encodeTextValue(result.Mode),
			encodeTextValue(result.State),
			encodeTextValue(result.Entry.ID),
			encodeTextValue(result.Entry.Status),
			encodeTextValue(result.Reason),
		)
		printSyncQueueSummaryLine(stdout, "sync_queue_status", result.Mode, result.State, result.Summary)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "sync queue fail: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return 0
}

func printSyncRunResult(stdout io.Writer, stderr io.Writer, format string, result syncRunResult) int {
	switch format {
	case "text":
		printSyncRunText(stdout, "sync_run", result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "sync run: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	if result.Run.Status == incrementalsync.RunStatusRetrying {
		return 1
	}
	return 0
}

func printSyncNetworkRunResult(stdout io.Writer, stderr io.Writer, format string, result syncNetworkRunResult) int {
	switch format {
	case "text":
		printSyncNetworkRunText(stdout, result)
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "sync network run: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	if result.Run.Status == incrementalsync.RunStatusRetrying {
		return 1
	}
	return 0
}

func printSyncNetworkDiscoverRunResult(stdout io.Writer, stderr io.Writer, format string, result syncNetworkDiscoverRunResult, exitCode int) int {
	switch format {
	case "text":
		printSyncNetworkDiscoveryGateText(stdout, result.Discovery)
		if result.Discovery.Status == "matched" {
			printSyncNetworkRunText(stdout, syncNetworkRunResult{
				Operation: "discover-run",
				Mode:      result.Mode,
				Enqueue:   result.Enqueue,
				Run:       result.Run,
				Network:   result.Network,
			})
		}
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "sync network discover-run: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return exitCode
}

func syncNetworkRunExitCode(result syncNetworkRunResult) int {
	if result.Run.Status == incrementalsync.RunStatusRetrying {
		return 1
	}
	return 0
}

func printSyncNetworkLoopResult(stdout io.Writer, stderr io.Writer, format string, result syncNetworkLoopResult, exitCode int) int {
	switch format {
	case "text":
		fmt.Fprintf(stdout, "sync_network_loop mode=%s status=%s session_prefix=%s interval=%s max_runs=%d completed_runs=%d published_runs=%d idle_runs=%d retrying_runs=%d network_attempts=%d network_published_runs=%d network_not_attempted_runs=%d\n",
			encodeTextValue(result.Mode),
			encodeTextValue(result.Status),
			encodeTextValue(result.SessionPrefix),
			encodeTextValue(result.Interval),
			result.MaxRuns,
			result.CompletedRuns,
			result.PublishedRuns,
			result.IdleRuns,
			result.RetryingRuns,
			result.NetworkAttempts,
			result.NetworkPublishedRuns,
			result.NetworkNotAttemptedRuns,
		)
		for _, run := range result.Runs {
			printSyncNetworkRunText(stdout, run)
		}
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "sync network loop: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return exitCode
}

func printSyncLoopResult(stdout io.Writer, stderr io.Writer, format string, result syncLoopResult, exitCode int) int {
	switch format {
	case "text":
		fmt.Fprintf(stdout, "sync_loop mode=%s status=%s session_prefix=%s interval=%s max_runs=%d completed_runs=%d published_runs=%d idle_runs=%d retrying_runs=%d\n",
			encodeTextValue(result.Mode),
			encodeTextValue(result.Status),
			encodeTextValue(result.SessionPrefix),
			encodeTextValue(result.Interval),
			result.MaxRuns,
			result.CompletedRuns,
			result.PublishedRuns,
			result.IdleRuns,
			result.RetryingRuns,
		)
		for _, run := range result.Runs {
			printSyncRunText(stdout, "sync_loop_run", run)
		}
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "sync loop: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	return exitCode
}

func printSyncWatchResult(stdout io.Writer, stderr io.Writer, format string, result syncWatchResult) int {
	switch format {
	case "text":
		fmt.Fprintf(stdout, "sync_watch mode=%s status=%s session_prefix=%s settle=%s max_events=%d watched_roots=%d watched_dirs=%d event_batches=%d events_seen=%d completed_runs=%d published_runs=%d idle_runs=%d retrying_runs=%d\n",
			encodeTextValue(result.Mode),
			encodeTextValue(result.Status),
			encodeTextValue(result.SessionPrefix),
			encodeTextValue(result.Settle),
			result.MaxEvents,
			len(result.WatchedRoots),
			result.WatchedDirs,
			result.EventBatches,
			result.EventsSeen,
			result.CompletedRuns,
			result.PublishedRuns,
			result.IdleRuns,
			result.RetryingRuns,
		)
		if result.Baseline != nil {
			printSyncRunText(stdout, "sync_watch_baseline", *result.Baseline)
		}
		for _, run := range result.Runs {
			printSyncRunText(stdout, "sync_watch_run", run)
		}
	case "json":
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			fmt.Fprintf(stderr, "sync watch: encode result: %s\n", safeDiagnosticLine(err.Error()))
			return 1
		}
	}
	if result.Status == "retrying" {
		return 1
	}
	if result.Status == "canceled" {
		return 1
	}
	return 0
}

func printSyncRunText(stdout io.Writer, label string, result syncRunResult) {
	run := result.Run
	fmt.Fprintf(stdout, "%s mode=%s status=%s session=%s profile=%s target_id=%s enqueued=%d ready=%d in_flight=%d published=%d retried=%d recovered=%d queued=%d backoff=%d failed=%d done=%d total=%d state_path=%s run_path=%s error=%s\n",
		label,
		encodeTextValue(result.Mode),
		encodeTextValue(run.Status),
		encodeTextValue(run.SessionID),
		encodeTextValue(run.Scope.ProfileID),
		encodeTextValue(run.Scope.TargetID),
		len(result.Enqueue.Enqueued),
		len(run.Ready),
		len(run.InFlight),
		len(run.Published),
		len(run.Retried),
		run.Recovered,
		run.Summary.Queued,
		run.Summary.Backoff,
		run.Summary.Failed,
		run.Summary.Done,
		run.Summary.Total,
		encodeTextValue(defaultTextField(run.StatePath)),
		encodeTextValue(defaultTextField(run.RunPath)),
		encodeTextValue(defaultTextField(run.Error)),
	)
	for _, entry := range run.Published {
		printSyncQueueEntry(stdout, label+"_published_entry", entry)
	}
	for _, entry := range run.Retried {
		printSyncQueueEntry(stdout, label+"_retried_entry", entry)
	}
}

func printSyncNetworkRunText(stdout io.Writer, result syncNetworkRunResult) {
	printSyncRunText(stdout, "sync_network_run", syncRunResult{
		Operation: result.Operation,
		Mode:      result.Mode,
		Enqueue:   result.Enqueue,
		Run:       result.Run,
	})
	network := result.Network
	session := network.SessionID
	if session == "" {
		session = "-"
	}
	fmt.Fprintf(stdout, "sync_network_transfer transfer=%s encrypted_transfer=%s session=%s profile=%s target_id=%s source_device=%s target_device=%s pairing_receipt=%s resume_authority=%s resume_outcome=%s resumed_bytes=%d files=%d bytes=%d chunks=%d warnings=%d status=%s stage=%s error_code=%s\n",
		encodeTextValue(network.Transfer),
		encodeTextValue(network.EncryptedTransfer),
		encodeTextValue(session),
		encodeTextValue(network.ProfileID),
		encodeTextValue(network.TargetID),
		encodeTextValue(network.SourceDeviceID),
		encodeTextValue(network.TargetDeviceID),
		encodeTextValue(network.PairingReceiptID),
		encodeTextValue(network.ResumeAuthority),
		encodeTextValue(network.ResumeOutcome),
		network.ResumedBytes,
		network.Files,
		network.Bytes,
		network.Chunks,
		network.Warnings,
		encodeTextValue(defaultTextField(network.Status)),
		encodeTextValue(defaultTextField(network.Stage)),
		encodeTextValue(defaultTextField(network.ErrorCode)),
	)
}

func printSyncNetworkDiscoveryGateText(stdout io.Writer, gate syncNetworkDiscoveryGate) {
	fmt.Fprintf(stdout, "sync_network_discovery_gate status=%s reason=%s profile_address=%s matched_address=%s matched_class=%s candidates=%d invalid_packets=%d trusted=false caps=%s expires_at=%s\n",
		encodeTextValue(gate.Status),
		encodeTextValue(gate.Reason),
		encodeTextValue(gate.ProfileAddress),
		encodeTextValue(defaultTextField(gate.MatchedAddress)),
		encodeTextValue(defaultTextField(gate.MatchedClass)),
		gate.CandidateCount,
		gate.InvalidPackets,
		encodeTextValue(formatStringList(gate.Capabilities)),
		encodeTextValue(defaultTextField(gate.ExpiresAt)),
	)
}

func printSyncQueueSummaryLine(w io.Writer, label string, mode string, state string, summary incrementalsync.Summary) {
	fmt.Fprintf(w, "%s mode=%s state=%s profile=%s target_id=%s queued=%d in_flight=%d backoff=%d failed=%d canceled=%d done=%d ready=%d total=%d warnings=%d state_path=%s generated_at=%s\n",
		label,
		encodeTextValue(mode),
		encodeTextValue(state),
		encodeTextValue(summary.ProfileID),
		encodeTextValue(summary.TargetID),
		summary.Queued,
		summary.InFlight,
		summary.Backoff,
		summary.Failed,
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
	fmt.Fprintf(w, "%s id=%s profile=%s target_id=%s root=%s path=%s kind=%s status=%s attempts=%d digest=%s symlink_target=%s size=%d mode=%#o last_error=%s next_due_at=%s failed_at=%s canceled_at=%s done_at=%s enqueued_at=%s updated_at=%s\n",
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
		entry.Size,
		entry.Mode,
		encodeTextValue(defaultTextField(entry.LastError)),
		encodeTextValue(defaultTextField(entry.NextDueAt)),
		encodeTextValue(defaultTextField(entry.FailedAt)),
		encodeTextValue(defaultTextField(entry.CanceledAt)),
		encodeTextValue(defaultTextField(entry.DoneAt)),
		encodeTextValue(defaultTextField(entry.EnqueuedAt)),
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

func printReconcileReview(stdout io.Writer, stderr io.Writer, format string, review reconcile.Review) error {
	switch format {
	case "text":
		printReconcileReviewText(stdout, review)
	case "json":
		if err := json.NewEncoder(stdout).Encode(review); err != nil {
			return fmt.Errorf("encode review: %w", err)
		}
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
	_ = stderr
	return nil
}

func printReconcileReviewText(w io.Writer, review reconcile.Review) {
	fmt.Fprintf(w, "reconcile_review schema=%s target=%s profile=%s target_id=%s session=%s review_required=%t persisted_records=%d persisted_planned=%d persisted_noop=%d persisted_refused=%d persisted_artifact_problems=%d live_manifest_count=%d live_target_drifts=%d live_artifact_problems=%d boundaries=%d apply_capable_boundaries=%d record_required_boundaries=%d planned_boundaries=%d\n",
		encodeTextValue(review.Schema),
		encodeTextValue(review.TargetRoot),
		encodeTextValue(review.ProfileID),
		encodeTextValue(review.TargetID),
		encodeTextValue(defaultTextField(review.SessionID)),
		review.Summary.ReviewRequired,
		review.Summary.PersistedRecords,
		review.Summary.PersistedPlanned,
		review.Summary.PersistedNoop,
		review.Summary.PersistedRefused,
		review.Summary.PersistedArtifactProblems,
		review.Summary.LiveManifestCount,
		review.Summary.LiveTargetDrifts,
		review.Summary.LiveArtifactProblems,
		review.Summary.Boundaries,
		review.Summary.ApplyCapableBoundaries,
		review.Summary.RecordRequiredBoundaries,
		review.Summary.PlannedBoundaries,
	)
	live := review.LiveRepairInput
	fmt.Fprintf(w, "reconcile_live_input source=%s durable=%t session=%s action=%s manifest_count=%d manifest_entries=%d target_drifts=%d artifact_problems=%d reason=%s\n",
		encodeTextValue(live.Source),
		live.Durable,
		encodeTextValue(defaultTextField(live.SessionID)),
		encodeTextValue(live.Action),
		live.Summary.ManifestCount,
		live.Summary.ManifestEntries,
		live.Summary.TargetDrifts,
		live.Summary.ArtifactProblems,
		encodeTextValue(live.Reason),
	)
	for _, drift := range live.TargetDrifts {
		fmt.Fprintf(w, "reconcile_live_drift id=%s path=%s change=%s session=%s durable=false action=record_before_apply\n",
			encodeTextValue(drift.ID),
			encodeTextValue(drift.Path),
			encodeTextValue(drift.Change),
			encodeTextValue(defaultTextField(drift.SessionID)),
		)
	}
	for _, boundary := range review.Boundaries {
		fmt.Fprintf(w, "reconcile_boundary name=%s status=%s source=%s action=%s apply_capable=%t records=%d planned=%d noop=%d refused=%d target_drifts=%d artifact_problems=%d reason=%s\n",
			encodeTextValue(boundary.Name),
			encodeTextValue(boundary.Status),
			encodeTextValue(boundary.Source),
			encodeTextValue(boundary.Action),
			boundary.ApplyCapable,
			boundary.Summary.Records,
			boundary.Summary.Planned,
			boundary.Summary.Noop,
			boundary.Summary.Refused,
			boundary.Summary.TargetDrifts,
			boundary.Summary.ArtifactProblems,
			encodeTextValue(boundary.Reason),
		)
	}
	for _, problem := range review.ArtifactProblems {
		fmt.Fprintf(w, "reconcile_review_artifact_problem session=%s path=%s error=%s\n",
			encodeTextValue(defaultTextField(problem.SessionID)),
			encodeTextValue(problem.Path),
			encodeTextValue(problem.Error),
		)
	}
}

func printReconcileText(w io.Writer, receipt reconcile.Receipt) {
	label := "reconcile_plan"
	if receipt.Schema == reconcile.SchemaApplyReceipt {
		label = "reconcile_apply"
	}
	fmt.Fprintf(w, "%s schema=%s receipt=%s status=%s receipt_path=%s target=%s profile=%s target_id=%s session=%s apply_intent=%t records=%d planned=%d applied=%d noop=%d refused=%d artifact_problems=%d\n",
		label,
		encodeTextValue(receipt.Schema),
		encodeTextValue(defaultTextField(receipt.ID)),
		encodeTextValue(defaultTextField(receipt.Status)),
		encodeTextValue(defaultTextField(receipt.ReceiptPath)),
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
		fmt.Fprintf(w, "reconcile_refusal drift=%s path=%s change=%s action=%s reason=%s conflict_class=%s retry_advice=%s message=%s observed_present=%t observed_kind=%s observed_digest=%s\n",
			encodeTextValue(defaultTextField(refusal.DriftID)),
			encodeTextValue(defaultTextField(refusal.Path)),
			encodeTextValue(defaultTextField(refusal.Change)),
			encodeTextValue(defaultTextField(refusal.Action)),
			encodeTextValue(refusal.ReasonCode),
			encodeTextValue(defaultTextField(refusal.ConflictClass)),
			encodeTextValue(defaultTextField(refusal.RetryAdvice)),
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

func printDriftExpireText(w io.Writer, result driftreview.ExpireResult) {
	fmt.Fprintf(w, "drift_expire id=%s path=%s previous_state=%s review_state=%s reviewed_at=%s reviewer=%s reason=%s profile_id=%s target_id=%s session_id=%s repair=not_applied manifest_rewrite=not_applied prune=not_authorized\n",
		encodeTextValue(result.ID),
		encodeTextValue(result.Path),
		encodeTextValue(result.PreviousState),
		encodeTextValue(result.ReviewState),
		encodeTextValue(result.ReviewedAt),
		encodeTextValue(statusTextValueOrDash(result.Reviewer)),
		encodeTextValue(result.Reason),
		encodeTextValue(result.ProfileID),
		encodeTextValue(result.TargetID),
		encodeTextValue(result.SessionID),
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
	fmt.Fprintf(w, "report: target=%s status=%s session=%s manifests=%d files=%d/%d verification_errors=%d verification_warnings=%d warnings=%d profile_suggestions=%d soft_deletes=%d prune_candidates=%d prune_refusals=%d prune_approvals=%d prune_unapplied_approvals=%d prune_active_approvals=%d prune_stale_approvals=%d prune_expired_approvals=%d prune_consumed_approvals=%d prune_receipts=%d prune_receipt_issues=%d prune_artifact_problems=%d reconcile_receipts=%d reconcile_receipt_issues=%d incremental_sync_queued=%d incremental_sync_in_flight=%d incremental_sync_backoff=%d incremental_sync_failed=%d incremental_sync_done=%d incremental_sync_ready=%d incremental_sync_runs=%d incremental_sync_retrying_runs=%d incremental_sync_artifact_problems=%d target_drifts=%d live_target_drifts=%d live_target_drift_artifact_problems=%d recovery_issues=%d invalid_records=%d artifact_problems=%d pairing_issues=%d scope=%s\n",
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
		report.Summary.ReconcileReceipts,
		report.Summary.ReconcileReceiptIssues,
		report.Summary.IncrementalSyncQueued,
		report.Summary.IncrementalSyncInFlight,
		report.Summary.IncrementalSyncBackoff,
		report.Summary.IncrementalSyncFailed,
		report.Summary.IncrementalSyncDone,
		report.Summary.IncrementalSyncReady,
		report.Summary.IncrementalSyncRuns,
		report.Summary.IncrementalSyncRetryingRuns,
		report.Summary.IncrementalSyncArtifactProblems,
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
	printReportIncrementalSyncText(w, report.IncrementalSync)
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

func printReportIncrementalSyncText(w io.Writer, review report.IncrementalSyncReview) {
	fmt.Fprintf(w, "incremental_sync status=%s action=%s queue_state=%s profile=%s target_id=%s queued=%d in_flight=%d backoff=%d failed=%d canceled=%d done=%d ready=%d total=%d runs=%d artifact_problems=%d state_path=%s\n",
		encodeTextValue(string(review.Status)),
		encodeTextValue(review.Action),
		encodeTextValue(review.Queue.State),
		encodeTextValue(review.Queue.ProfileID),
		encodeTextValue(review.Queue.TargetID),
		review.Queue.Queued,
		review.Queue.InFlight,
		review.Queue.Backoff,
		review.Queue.Failed,
		review.Queue.Canceled,
		review.Queue.Done,
		review.Queue.Ready,
		review.Queue.Total,
		len(review.Runs),
		len(review.ArtifactProblems),
		encodeTextValue(review.Queue.StatePath),
	)
	for _, run := range review.Runs {
		fmt.Fprintf(w, "incremental_sync_run session=%s status=%s action=%s ready=%d in_flight=%d published=%d retried=%d recovered=%d started_at=%s finished_at=%s path=%s error=%s\n",
			encodeTextValue(run.SessionID),
			encodeTextValue(run.Status),
			encodeTextValue(run.Action),
			run.Ready,
			run.InFlight,
			run.Published,
			run.Retried,
			run.Recovered,
			encodeTextValue(run.StartedAt),
			encodeTextValue(run.FinishedAt),
			encodeTextValue(run.Path),
			encodeTextValue(run.Error),
		)
	}
}

func printStatusText(w io.Writer, report status.Report) {
	fmt.Fprintf(w, "status: target=%s profile_id=%s target_id=%s status=%s target_status=%s review_required=%t latest_session=%s completeness_status=%s manifests=%d files=%d/%d verification_errors=%d verification_warnings=%d warnings=%d profile_suggestions=%d soft_deletes=%d prune_review_status=%s prune_review_action=%s prune_approvals=%d prune_unapplied_approvals=%d prune_active_approvals=%d prune_stale_approvals=%d prune_expired_approvals=%d prune_consumed_approvals=%d prune_receipts=%d prune_receipt_issues=%d reconcile_receipts=%d reconcile_receipt_issues=%d incremental_sync_status=%s incremental_sync_action=%s incremental_sync_queued=%d incremental_sync_in_flight=%d incremental_sync_backoff=%d incremental_sync_failed=%d incremental_sync_done=%d incremental_sync_ready=%d incremental_sync_runs=%d incremental_sync_retrying_runs=%d incremental_sync_artifact_problems=%d target_drifts=%d live_target_drifts=%d live_target_drift_artifact_problems=%d recovery_issues=%d invalid_health_records=%d artifact_problems=%d artifact_problem_sources=%s pairing_issues=%d network_transfers=%d\n",
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
		report.Counts.ReconcileReceipts,
		report.Counts.ReconcileReceiptIssues,
		encodeTextValue(report.IncrementalSync.Status),
		encodeTextValue(report.IncrementalSync.Action),
		report.Counts.IncrementalSyncQueued,
		report.Counts.IncrementalSyncInFlight,
		report.Counts.IncrementalSyncBackoff,
		report.Counts.IncrementalSyncFailed,
		report.Counts.IncrementalSyncDone,
		report.Counts.IncrementalSyncReady,
		report.Counts.IncrementalSyncRuns,
		report.Counts.IncrementalSyncRetryingRuns,
		report.Counts.IncrementalSyncArtifactProblems,
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

type discoveryBrowseResult struct {
	Source         string                `json:"source"`
	Listen         string                `json:"listen"`
	CandidateCount int                   `json:"candidate_count"`
	InvalidPackets int                   `json:"invalid_packets"`
	Trusted        bool                  `json:"trusted"`
	Candidates     []discovery.Candidate `json:"candidates"`
}

type discoveryAdvertiseResult struct {
	Status          string   `json:"status"`
	Listen          string   `json:"listen"`
	Destination     string   `json:"destination"`
	ServiceType     string   `json:"service_type"`
	ProtocolVersion string   `json:"protocol_version"`
	EphemeralNonce  string   `json:"ephemeral_nonce"`
	Capabilities    []string `json:"capability_flags"`
	Trusted         bool     `json:"trusted"`
	Duration        string   `json:"duration"`
	Interval        string   `json:"interval"`
}

func printDiscoveryBrowseText(w io.Writer, result discoveryBrowseResult) {
	fmt.Fprintf(w, "discover browse: candidates=%d trusted=false source=%s listen=%s invalid_packets=%d\n",
		result.CandidateCount,
		encodeTextValue(result.Source),
		encodeTextValue(result.Listen),
		result.InvalidPackets,
	)
	for _, candidate := range result.Candidates {
		hint := candidate.Hint
		fmt.Fprintf(w, "candidate address=%s class=%s duplicates=%d service=%s protocol=%s nonce=%s caps=%s trusted=false expires_at=%s ambiguity=%s\n",
			hint.Address,
			candidate.Class,
			candidate.DuplicateCount,
			hint.Advertisement.ServiceType,
			hint.Advertisement.ProtocolVersion,
			hint.Advertisement.EphemeralNonce,
			strings.Join(sortedStrings(hint.Advertisement.CapabilityFlags), ","),
			hint.ExpiresAt.Format(time.RFC3339Nano),
			formatDiscoveryReasonList(candidate.AmbiguityReasons),
		)
	}
}

func printDiscoveryAdvertiseText(w io.Writer, result discoveryAdvertiseResult) {
	fmt.Fprintf(w, "discover advertise: status=%s listen=%s destination=%s service=%s protocol=%s caps=%s trusted=false duration=%s interval=%s\n",
		result.Status,
		result.Listen,
		result.Destination,
		result.ServiceType,
		result.ProtocolVersion,
		strings.Join(sortedStrings(result.Capabilities), ","),
		encodeTextValue(result.Duration),
		encodeTextValue(result.Interval),
	)
}

func formatStringList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(sortedStrings(values), ",")
}

func formatDiscoveryReasonList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return encodeTextValue(strings.Join(sortedStrings(values), ","))
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

func deterministicLANDiscoveryNonce(now time.Time, p profile.Profile) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sum := sha256.Sum256([]byte(protocol.Version + "\n" + p.ProfileID + "\n" + p.Target.TargetID + "\n" + strconv.FormatInt(now.UTC().UnixNano(), 10)))
	return "n" + hex.EncodeToString(sum[:8])
}

func discoveryCapabilitiesForProfile(p profile.Profile) []string {
	caps := []string{"pair"}
	if p.PrivacyPolicy.TrafficLevel == 2 && p.PrivacyPolicy.DiscoveryLowInfo {
		caps = append(caps, "l2")
	}
	return sortedStrings(caps)
}

var errInvalidDiscoveryAddress = errors.New("invalid discovery address")

func listenDiscoveryUDP(address string) (*net.UDPConn, error) {
	udpAddr, err := resolveDiscoveryListenUDPAddr(address)
	if err != nil {
		return nil, err
	}
	return net.ListenUDP("udp4", udpAddr)
}

func resolveDiscoveryListenUDPAddr(address string) (*net.UDPAddr, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errInvalidDiscoveryAddress
	}
	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil || udpAddr == nil || udpAddr.Port < 0 || udpAddr.Port > 65535 {
		return nil, errInvalidDiscoveryAddress
	}
	return udpAddr, nil
}

func resolveDiscoveryUDPAddr(address string) (*net.UDPAddr, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errInvalidDiscoveryAddress
	}
	udpAddr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil || udpAddr == nil || udpAddr.IP == nil || udpAddr.IP.IsUnspecified() || udpAddr.Port <= 0 || udpAddr.Port > 65535 {
		return nil, errInvalidDiscoveryAddress
	}
	return udpAddr, nil
}

func enableUDPBroadcast(conn *net.UDPConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	return sockErr
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

func printDiscoverUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage of discover:
  supermover discover [--timeout <duration>] [--address <host:port> ...] [--format text|json]
  supermover discover browse [--listen <udp-host:port>] [--timeout <duration>] [--strict] [--format text|json]
  supermover discover advertise --profile <path> [--listen <udp-host:port>] [--dest <udp-host:port>] [--interval <duration>] [--duration <duration>] [--format text|json]

Emits unauthenticated, low-information address hints and LAN browse candidates.
Discovery is not trust: pair with explicit verification before pushing data.`)
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
profile as the runtime SSOT. When the profile enables sync.local_polling, the
foreground process also runs the local queue consumer and writes sync receipts.
When the profile enables repair.drift_recording, the foreground process records
current live detector findings as durable drift review evidence without applying
repair or reconcile mutations.
When the profile enables repair.persisted_reconcile_apply, the foreground
process applies only currently planned persisted drift records through the
existing reconcile receipt path and stops after any refusal for operator review.
When the profile enables sync.network_polling, it runs a source-side foreground
network queue worker using per-entry profile-backed mTLS queue publication
instead of serving receiver routes.
It does not install an OS service manager, manage detached background
processes, browse the LAN, run an OS file watcher, execute automatic
discovery-selected sync, select endpoints automatically, run broad repair,
record live-only drift before apply, rewrite manifests, hand drift to prune, or
perform bidirectional sync. The current profile-backed reconcile worker is not a
retry policy.`)
}

func printSyncUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  supermover sync queue <enqueue|status|list|ready|cancel|fail> [flags]
  supermover sync run --profile <path> --session <id> [--retry-backoff <duration>] [--format text|json]
  supermover sync loop --profile <path> --session-prefix <id> [--interval <duration>] [--max-runs <n>] [--retry-backoff <duration>] [--format text|json]
  supermover sync watch --profile <path> --session-prefix <id> [--settle <duration>] [--max-events <n>] [--retry-backoff <duration>] [--format text|json]
  supermover sync network run --profile <path> --session <id> [--retry-backoff <duration>] [--format text|json]
  supermover sync network discover-run --profile <path> --session <id> [--listen <udp-host:port>] [--timeout <duration>] [--retry-backoff <duration>] [--format text|json]
  supermover sync network loop --profile <path> --session-prefix <id> [--interval <duration>] [--max-runs <n>] [--retry-backoff <duration>] [--format text|json]

Sync commands expose durable changed-file queue evidence, a bounded one-pass
local queue consumer, a foreground polling loop, and a foreground OS watcher
that feeds the same local queue consumer. The bounded network run and foreground
network loop publish ready queue entries through a per-entry profile-backed mTLS
network manifest, and discover-run adds a low-information LAN discovery gate
before the same profile-pinned transfer. Queue receipts remain under the
profile-selected target control plane. These commands do not run a background
daemon, bidirectional sync engine, automatic endpoint selection, or broad
repair.`)
}

func printSyncNetworkUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage of sync network:
  supermover sync network run --profile <path> --session <id> [--retry-backoff <duration>] [--format text|json]
  supermover sync network discover-run --profile <path> --session <id> [--listen <udp-host:port>] [--timeout <duration>] [--retry-backoff <duration>] [--format text|json]
  supermover sync network loop --profile <path> --session-prefix <id> [--interval <duration>] [--max-runs <n>] [--retry-backoff <duration>] [--format text|json]

Runs bounded network queue publication and a foreground network polling loop
through per-entry profile-backed mTLS transfer. discover-run first requires a
low-information LAN candidate that matches the profile-selected
network.receiver_url before running that same mTLS path. Regular-file
replacements require previous published manifest evidence and receiver-side
target revalidation. Queue and run receipts still live under the
profile-selected target .supermover control plane. This is not automatic
endpoint selection, an OS file watcher, a detached daemon, broad repair, or
bidirectional sync.`)
}

func printSyncQueueUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage of sync queue:
  supermover sync queue enqueue --profile <path> [--format text|json]
  supermover sync queue status --profile <path> [--format text|json]
  supermover sync queue list --profile <path> [--format text|json]
  supermover sync queue ready --profile <path> [--format text|json]
  supermover sync queue cancel --profile <path> --id <entry-id> --reason <text> [--format text|json]
  supermover sync queue fail --profile <path> --id <entry-id> --reason <text> [--format text|json]

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
  supermover reconcile review --profile <path> [--session <id>] [--format text|json]
  supermover reconcile apply --profile <path> --id <persisted-drift-id> [--id <persisted-drift-id>...] --apply --reason <text> [--reviewer <id>] [--session <id>] [--format text|json]
  supermover reconcile apply --profile <path> --all-persisted-planned --apply --reason <text> [--reviewer <id>] [--session <id>] [--format text|json]
  supermover reconcile apply --profile <path> --record-live --apply --reason <text> [--reviewer <id>] [--session <id>] [--format text|json]

Plans, reviews, and applies reconcile evidence on the profile-selected target.
Current apply support is narrow persisted drift repair: selected persisted-drift
missing regular-file restores from published/source evidence and
already-restored resolves. Review is read-only: it exposes persisted plan
readiness, live-only record-required inputs, and planned broad boundaries
without target mutation. Apply writes durable selected-ID receipts; the
--all-persisted-planned gate first reviews durable persisted evidence and then
selects only currently planned persisted actions. The --record-live gate first
persists current live detector findings, then applies only the resulting
persisted planned actions.
Apply does not run broad automatic reconcile, automatic scanning, manifest
rewrites, pruning, automatic retry policy, or background repair. Refusals
include conflict_class and retry_advice fields for operator review.`)
}

func printDriftUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  supermover drift list --profile <path> [--session <id>] [--format text|json]
  supermover drift record --profile <path> [--session <id>] [--format text|json]
  supermover drift acknowledge --profile <path> --id <persisted-drift-id> --reason <text> [--reviewer <id>] [--format text|json]
  supermover drift expire --profile <path> --id <persisted-drift-id> --reason <text> [--reviewer <id>] [--format text|json]
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
  reconcile   Plan/review/apply narrow persisted drift repair
  health      Inspect target control-plane health
  report      Summarize local migration evidence for operator review
  status      Show compact local profile/target status
  recover     Resume safe local sessions or mark incomplete sessions
  serve       Pairing plus profile-backed TLS receiver for push --network
  daemon      Manage foreground agent lifecycle state around serve
  sync        Manage durable changed-file queue, bounded local/network runs, foreground loop, and watcher evidence
  discover    Low-information explicit and LAN browse hints; no trust
  pair        Write local pairing receipt/profile pins after verification

Use "supermover help" for this overview.
`, buildinfo.Name, buildinfo.Description)
}
