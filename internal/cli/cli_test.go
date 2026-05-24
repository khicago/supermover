package cli

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/agentdaemon"
	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/incrementalsync"
	"github.com/khicago/supermover/internal/operatorui"
	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/pairserve"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/protocolclient"
	"github.com/khicago/supermover/internal/prune"
	"github.com/khicago/supermover/internal/receiver"
	"github.com/khicago/supermover/internal/receiverserve"
	"github.com/khicago/supermover/internal/reconcile"
	"github.com/khicago/supermover/internal/report"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/sourceconsistency"
	"github.com/khicago/supermover/internal/status"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
	"github.com/khicago/supermover/internal/verify"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

type blockingDaemonRunningWriter struct {
	mu             sync.Mutex
	buf            bytes.Buffer
	seenRunning    int
	blockOnRunning int
	blocked        chan struct{}
	release        chan struct{}
	unblockOnce    sync.Once
}

func newBlockingDaemonRunningWriter(blockOnRunning int) *blockingDaemonRunningWriter {
	return &blockingDaemonRunningWriter{
		blockOnRunning: blockOnRunning,
		blocked:        make(chan struct{}, 1),
		release:        make(chan struct{}),
	}
}

func (w *blockingDaemonRunningWriter) Write(p []byte) (int, error) {
	text := string(p)
	shouldBlock := false

	w.mu.Lock()
	if strings.Contains(text, "daemon: running") {
		w.seenRunning++
		shouldBlock = w.seenRunning == w.blockOnRunning
	}
	if !shouldBlock {
		n, err := w.buf.Write(p)
		w.mu.Unlock()
		return n, err
	}
	w.mu.Unlock()

	select {
	case w.blocked <- struct{}{}:
	default:
	}
	<-w.release

	w.mu.Lock()
	n, err := w.buf.Write(p)
	w.mu.Unlock()
	return n, err
}

func (w *blockingDaemonRunningWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func (w *blockingDaemonRunningWriter) Unblock() {
	w.unblockOnce.Do(func() {
		close(w.release)
	})
}

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
	assertTextContainsAll(t, "root help", stdout.String(), "bounded local/network runs, foreground loop, and watcher evidence")
	if strings.Contains(stdout.String(), "ongoing incremental sync") || strings.Contains(stdout.String(), "continuous sync") {
		t.Errorf("Run(%v) stdout = %q, should not present continuous incremental sync as current help description", []string{"help"}, stdout.String())
	}
	availableIndex := strings.Index(stdout.String(), "Available commands:")
	if availableIndex == -1 {
		t.Fatalf("Run(%v) stdout = %q, want available command section", []string{"help"}, stdout.String())
	}
	for _, command := range []string{"profile", "scan", "push", "verify", "drift", "deleted", "prune", "reconcile", "health", "report", "status", "recover", "serve", "daemon", "sync", "discover", "pair"} {
		commandIndex := strings.Index(stdout.String(), "\n  "+command+" ")
		if commandIndex == -1 {
			t.Errorf("Run(%v) stdout = %q, want available command %q", []string{"help"}, stdout.String(), command)
		} else if commandIndex < availableIndex {
			t.Errorf("Run(%v) stdout = %q, command %q should be listed as available", []string{"help"}, stdout.String(), command)
		}
	}
	if strings.Contains(stdout.String(), "Planned commands:") {
		t.Errorf("Run(%v) stdout = %q, should not retain planned command section once prune is wired as fail-closed contract", []string{"help"}, stdout.String())
	}
	if strings.Contains(stdout.String(), "Core commands:") {
		t.Errorf("Run(%v) stdout = %q, should not label planned commands as core", []string{"help"}, stdout.String())
	}
	if strings.Contains(stdout.String(), "Discover low-information address hints") {
		t.Errorf("Run(%v) stdout = %q, should not imply discover performs operational LAN discovery", []string{"help"}, stdout.String())
	}
	if strings.Contains(stdout.String(), "identity pinning later") {
		t.Errorf("Run(%v) stdout = %q, should not describe pair as future-only identity pinning", []string{"help"}, stdout.String())
	}
	if !strings.Contains(stdout.String(), "profile-backed TLS receiver") ||
		!strings.Contains(stdout.String(), "profile-backed mTLS") {
		t.Errorf("Run(%v) stdout = %q, want honest network status", []string{"help"}, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("Run(%v) stderr = %q, want empty", []string{"help"}, stderr.String())
	}
}

func TestPushNetworkHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"push", "--network", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("push --network --help exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	for _, want := range []string{
		"Usage of push --network",
		"--profile",
		"source-initiated network transfer contract",
		"fails closed",
		"receiver URL",
		"local TLS identity references",
		"local TLS identity",
		"without contacting the receiver",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("push --network --help stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{"--address", "--target-id", "--privacy", "--delete-policy", "encrypted transfer ready"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("push --network --help stdout = %q, must not expose override %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("push --network --help stderr = %q, want empty", stderr.String())
	}
}

func TestPruneHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"prune", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune --help exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	for _, want := range []string{
		"Usage of prune",
		"--profile",
		"prune review",
		"dry-run wiring reads published",
		"prune approval artifact",
		"without mutating target",
		"without writing approvals, receipts, or target files",
		"writes a prune receipt before",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("prune --help stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{
		"physical prune is available",
		"approval evidence written",
		"physical_pruning=ready",
		"receipt_writing=ready",
		"approved prune receipt",
		"release ready",
	} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("prune --help stdout = %q, must not contain %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune --help stderr = %q, want empty", stderr.String())
	}
}

func TestReportHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"report", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("report --help exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	for _, want := range []string{
		"Usage of report",
		"--profile",
		"read-only evidence",
		"profile-selected target",
		"persisted network-transfer artifacts",
		"incremental sync queue/run",
		"does not start daemon, transport",
		"watcher, or repair work",
		"persist live detector output",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report --help stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{"network transport status ready", "daemon readiness", "repair state automatically"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("report --help stdout = %q, must not contain %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("report --help stderr = %q, want empty", stderr.String())
	}
}

func TestDriftListHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"drift", "list", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("drift list --help exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	for _, want := range []string{
		"Usage of drift list",
		"--profile",
		"read-only live target drift detector",
		"profile-selected",
		"Output is not persisted",
		"use drift record to persist current findings",
		"before drift acknowledge can review them",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("drift list --help stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{"writes target_drifts", "writes .supermover/drift", "acknowledge live drift"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("drift list --help stdout = %q, must not contain %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift list --help stderr = %q, want empty", stderr.String())
	}
}

func TestDriftRecordHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"drift", "record", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("drift record --help exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	for _, want := range []string{
		"Usage of drift record",
		"--profile",
		"profile-selected target",
		"--session",
		"--format",
		"durable .supermover/drift review records",
		"records evidence only",
		"does not resolve, repair, prune",
		"suppress future detector output",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("drift record --help stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{"--target", "--policy", "background daemon", "clean target"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("drift record --help stdout = %q, must not contain %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift record --help stderr = %q, want empty", stderr.String())
	}
}

func TestDriftAcknowledgeHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"drift", "acknowledge", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("drift acknowledge --help exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	for _, want := range []string{
		"Usage of drift acknowledge",
		"--profile",
		"--id",
		"--reason",
		"existing durable .supermover/drift review record",
		"including drift record output",
		"live-only drift list/report.live_target_drift ids are refused",
		"records review metadata only",
		"does not resolve, repair, prune",
		"suppress future detector output",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("drift acknowledge --help stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{"--target", "--policy", "clean target"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("drift acknowledge --help stdout = %q, must not contain %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift acknowledge --help stderr = %q, want empty", stderr.String())
	}
}

func TestDriftExpireHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"drift", "expire", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("drift expire --help exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	for _, want := range []string{
		"Usage of drift expire",
		"--profile",
		"--id",
		"--reason",
		"Marks one existing durable .supermover/drift review record expired",
		"records expiry metadata only",
		"does not repair target files",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("drift expire --help stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{"--target", "clean target", "background daemon"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("drift expire --help stdout = %q, must not contain %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("drift expire --help stderr = %q, want empty", stderr.String())
	}
}

func TestPruneApprovalsHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"prune", "approvals", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune approvals --help exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	for _, want := range []string{
		"Usage of prune approvals",
		"--profile",
		"--format",
		"Lists current-scope prune approval artifacts",
		"read-only",
		"does not supersede approvals",
		"write prune receipts",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("prune approvals --help stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune approvals --help stderr = %q, want empty", stderr.String())
	}
}

func TestPruneSupersedeHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"prune", "supersede", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune supersede --help exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	for _, want := range []string{
		"Usage of prune supersede",
		"--profile",
		"--id",
		"--reason",
		"--reviewer",
		"Marks one existing current-scope prune approval artifact superseded",
		"updates durable approval review metadata only",
		"does not apply prune",
		"write prune receipts",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("prune supersede --help stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune supersede --help stderr = %q, want empty", stderr.String())
	}
}

func TestStatusHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"status", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("status --help exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	for _, want := range []string{"Usage of status", "--profile", "--format"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status --help stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, want := range []string{"target files needed for verification and live drift detection", "output is not persisted", "incremental sync fields report local", "does not start discovery"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status --help stdout = %q, want honesty note %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{"--session", "--target", "--policy", "--network"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("status --help stdout = %q, must not expose %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("status --help stderr = %q, want empty", stderr.String())
	}
}

func TestStatusUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing profile", args: []string{"status"}, want: "status: --profile is required"},
		{name: "blank profile", args: []string{"status", "--profile", "   "}, want: "status: --profile is required"},
		{name: "unsupported format", args: []string{"status", "--profile", "profile.json", "--format", "yaml"}, want: `status: unsupported format "yaml"`},
		{name: "unexpected args", args: []string{"status", "--profile", "profile.json", "extra\nvalue"}, want: `status: unexpected arguments: "extra\nvalue"`},
		{name: "session flag rejected", args: []string{"status", "--profile", "profile.json", "--session", "session-one"}, want: "flag provided but not defined"},
		{name: "unknown flag escaped", args: []string{"status", "--profile", "profile.json", "-bad\nflag"}, want: `status: flag provided but not defined: -bad\nflag`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d, stderr = %q, want 2", tt.args, got, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("Run(%v) stderr = %q, want %q", tt.args, stderr.String(), tt.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("Run(%v) stdout = %q, want empty", tt.args, stdout.String())
			}
			if strings.Contains(stderr.String(), "Usage of status") {
				t.Fatalf("Run(%v) stderr = %q, want diagnostic without flag package usage", tt.args, stderr.String())
			}
		})
	}
}

func TestStatusGenerationFailureReturnsUsageExitWithoutReport(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)
	mustMkdir(t, target)
	if err := os.Symlink(t.TempDir(), filepath.Join(target, control.DirName)); err != nil {
		t.Fatalf("os.Symlink(control dir) error = %v, want nil", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"status", "--profile", profilePath}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("status unsafe boundary exit = %d, stderr = %q, stdout = %q, want 2", got, stderr.String(), stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("status unsafe boundary stdout = %q, want no report", stdout.String())
	}
	if !strings.Contains(stderr.String(), "status:") {
		t.Fatalf("status unsafe boundary stderr = %q, want status diagnostic", stderr.String())
	}
}

func TestStatusCleanTargetTextAndJSONReturnSuccess(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-ok"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got := Run([]string{"status", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("status clean text exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"status: target=" + encodeTextValue(target),
		"profile_id=profile-local",
		"target_id=local:profile-local",
		"status=clean",
		"target_status=local_target_verified",
		"review_required=false",
		"latest_session=session-ok",
		"completeness_status=verified",
		"manifests=1",
		"files=1/1",
		"verification_errors=0",
		"verification_warnings=0",
		"warnings=0",
		"profile_suggestions=0",
		"soft_deletes=0",
		"target_drifts=0",
		"live_target_drifts=0",
		"live_target_drift_artifact_problems=0",
		"recovery_issues=0",
		"invalid_health_records=0",
		"artifact_problems=0",
		"pairing_issues=0",
		"network_transfers=0",
		"pairing status=unpaired encrypted_transfer=not_configured",
		"privacy network_transfer=not_configured local_push=traffic_shaping_not_applied",
		"network evidence_status=no_evidence artifact_problems=0",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status clean text stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("status clean text stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"status", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("status clean json exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var gotReport status.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(status stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Schema != status.SchemaV1 || gotReport.Overall.Status != status.OverallClean || gotReport.Overall.TargetStatus != "local_target_verified" || gotReport.ReviewRequired {
		t.Fatalf("status JSON report = %+v, want clean status.Report", gotReport)
	}
	if gotReport.LatestSession.ID != "session-ok" || gotReport.Counts.ManifestCount != 1 || gotReport.Network.Status != status.NetworkStatusNoEvidence {
		t.Fatalf("status JSON report latest/counts/network = %+v/%+v/%+v, want clean evidence", gotReport.LatestSession, gotReport.Counts, gotReport.Network)
	}
	if stderr.Len() != 0 {
		t.Fatalf("status clean json stderr = %q, want empty", stderr.String())
	}
}

func TestStatusJSONWriteFailureReturnsNoReportExit(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-ok"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}

	stderr.Reset()
	got := Run([]string{"status", "--profile", profilePath, "--format", "json"}, failingWriter{}, &stderr)
	if got != 2 {
		t.Fatalf("status json write failure exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "status: encode report:") {
		t.Fatalf("status json write failure stderr = %q, want encode diagnostic", stderr.String())
	}
}

func TestStatusReviewRequiredWarningReturnsOneWithReport(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-review"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	warning := control.Warning{
		Version:   control.CurrentVersion,
		ID:        "session-review-001-extra-config",
		SessionID: "session-review",
		Code:      "needs_profile_config",
		Message:   "path needs additional migration config",
		Severity:  "warning",
		Paths:     []string{"needs-extra"},
		CreatedAt: "2026-05-16T00:00:00Z",
	}
	warningPath, err := control.Path(target, control.ArtifactWarning, warning.ID)
	if err != nil {
		t.Fatalf("control.Path(warning) error = %v, want nil", err)
	}
	if err := control.WriteFile(warningPath, warning); err != nil {
		t.Fatalf("control.WriteFile(warning) error = %v, want nil", err)
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"status", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("status warning exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{"status=review_required", "target_status=local_target_attention", "review_required=true", "warnings=1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status warning stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("status warning stderr = %q, want empty", stderr.String())
	}
}

func TestStatusShowsPruneApprovalArtifactProblemSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)
	enablePrunePolicy(t, profilePath)
	approvalPath := filepath.Join(control.ControlDir(target), "prune", "approvals", "approval-damaged.json")
	mustWrite(t, approvalPath, "{")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"status", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("status damaged prune approval exit = %d, stderr = %q, stdout = %q, want review-needed 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"status=review_required",
		"target_status=local_target_unhealthy",
		"review_required=true",
		"artifact_problems=1",
		"artifact_problem_sources=prune_approval:1",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status damaged prune approval stdout = %q, want %q", stdout.String(), want)
		}
	}
	if strings.Contains(stdout.String(), "prune_approval id=") || strings.Contains(stdout.String(), "approval-damaged.json") {
		t.Fatalf("status damaged prune approval stdout = %q, want compact source count without detailed inventory", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("status damaged prune approval stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"status", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("status damaged prune approval json exit = %d, stderr = %q, stdout = %q, want review-needed 1", got, stderr.String(), stdout.String())
	}
	var gotReport status.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(status stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Counts.ArtifactProblems != 1 || len(gotReport.Counts.ArtifactProblemSources) != 1 || gotReport.Counts.ArtifactProblemSources[0] != (status.ArtifactProblemSourceCount{Source: "prune_approval", Count: 1}) {
		t.Fatalf("status damaged prune approval JSON counts = %+v, want prune approval source count", gotReport.Counts)
	}
	if stderr.Len() != 0 {
		t.Fatalf("status damaged prune approval json stderr = %q, want empty", stderr.String())
	}
}

func TestStatusTextEscapesControlledValues(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target\nwith=control")
	profilePath := filepath.Join(dir, "profile\nwith=control.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	p := profile.NewDefault("profile\nlocal", "Local profile", source, target)
	p.Target.TargetID = "target\nid"
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-one"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"status", "--profile", profilePath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("status escaped text exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"target=" + encodeTextValue(target),
		"profile_id=" + encodeTextValue(p.ProfileID),
		"target_id=" + encodeTextValue(p.Target.TargetID),
		"latest_session=session-one",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status escaped text stdout = %q, want %q", stdout.String(), want)
		}
	}
	firstLine := strings.SplitN(stdout.String(), "\n", 2)[0]
	for _, leaked := range []string{target, p.ProfileID, p.Target.TargetID} {
		if strings.Contains(firstLine, leaked) {
			t.Fatalf("status escaped text first line = %q, leaked raw value %q", firstLine, leaked)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("status escaped text stderr = %q, want empty", stderr.String())
	}
}

func TestStatusTextEscapesNetworkTransferValues(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-network"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	writeNetworkTransferForCLI(t, target, control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       "session-network",
		ProfileID:       "profile-local",
		TargetID:        "local:profile-local",
		SourceDeviceID:  "sha256:abcdef0123456789",
		TargetDeviceID:  "sha256:0123456789abcdef",
		ProtocolVersion: "supermover/1",
		EncryptedTransfer: control.NetworkTransferEncryptedTLS13MTLS,
		PrivacyPolicy:   transport.DefaultPrivacyPolicy(transport.PrivacyLevel2),
		Status:          control.NetworkTransferFailed,
		Stage:           "transport",
		StartedAt:       "2026-05-16T00:00:00Z",
		UpdatedAt:       "2026-05-16T00:00:01Z",
		ErrorCode:       "bad\ncode",
		Error:           "failure\nwith=field",
		Attempts: []control.NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: "2026-05-16T00:00:00Z",
			EndedAt:   "2026-05-16T00:00:01Z",
			Stage:     "transport",
			Status:    control.NetworkTransferFailed,
			ErrorCode: "bad\ncode",
			Error:     "failure\nwith=field",
		}},
	})
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"status", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("status network transfer exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"network evidence_status=review_required artifact_problems=0",
		"network_transfer session=session-network",
		"error_code=bad%0Acode",
		"error=failure%0Awith%3Dfield",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status network transfer stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if strings.Contains(line, "failure\nwith=field") || strings.Contains(line, "bad\ncode") {
			t.Fatalf("status network transfer line = %q, leaked raw network transfer value", line)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("status network transfer stderr = %q, want empty", stderr.String())
	}
}

func TestPushNetworkUsageErrors(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"push", "--network"}, want: "push --network: --profile is required"},
		{args: []string{"push", "--network", "--profile", "   "}, want: "push --network: --profile is required"},
		{args: []string{"push", "--network", "--profile", "profile.json", "extra"}, want: "push --network: unexpected arguments: extra"},
		{args: []string{"push", "--network", "--profile", "profile.json", "extra\ntransfer=ready"}, want: `push --network: unexpected arguments: "extra\ntransfer=ready"`},
		{args: []string{"push", "--network", "--profile", "profile.json", "--format", "yaml"}, want: `push --network: unsupported format "yaml"`},
		{args: []string{"push", "--network", "--profile", "profile.json", "--session", "bad session"}, want: "unsafe session id"},
		{args: []string{"push", "--network", "--profile", "profile.json", "--session", "bad\nsession"}, want: "unsafe session id"},
		{args: []string{"push", "--network", "--profile", "profile.json", "--session", "-"}, want: `session id "-" is reserved`},
		{args: []string{"push", "--network", "--profile", "profile.json", "--target-id", "target"}, want: "flag provided but not defined"},
		{args: []string{"push", "--network", "--profile", "profile.json", "--address", "127.0.0.1:9000"}, want: "flag provided but not defined"},
		{args: []string{"push", "--network", "--profile", "profile.json", "--privacy-level", "2"}, want: "flag provided but not defined"},
		{args: []string{"push", "--network", "--profile", "profile.json", "--privacy", "redacted"}, want: "flag provided but not defined"},
		{args: []string{"push", "--network", "--profile", "profile.json", "--padding-bucket-bytes", "65536"}, want: "flag provided but not defined"},
		{args: []string{"push", "--network", "--profile", "profile.json", "--jitter-budget-millis", "250"}, want: "flag provided but not defined"},
		{args: []string{"status", "--profile", "profile.json", "--privacy-level", "2"}, want: "flag provided but not defined"},
		{args: []string{"report", "--profile", "profile.json", "--padding-bucket-bytes", "65536"}, want: "flag provided but not defined"},
		{args: []string{"push", "--network", "--profile", "profile.json", "--delete-policy", "prune"}, want: "flag provided but not defined"},
		{args: []string{"push", "--network", "--profile", "profile.json", "--metadata-policy", "preserve"}, want: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d, stderr = %q, want 2", tt.args, got, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("Run(%v) stderr = %q, want %q", tt.args, stderr.String(), tt.want)
			}
			if strings.Count(stderr.String(), "\n") > 1 || strings.Contains(stderr.String(), "\ntransfer=ready") {
				t.Fatalf("Run(%v) stderr = %q, want single safe diagnostic line", tt.args, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("Run(%v) stdout = %q, want empty", tt.args, stdout.String())
			}
		})
	}
}

func TestPruneUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing profile", args: []string{"prune"}, want: "prune: --profile is required"},
		{name: "blank profile", args: []string{"prune", "--profile", "   "}, want: "prune: --profile is required"},
		{name: "unexpected positional arg", args: []string{"prune", "--profile", "profile.json", "extra"}, want: "prune: unexpected arguments: extra"},
		{name: "escaped positional arg", args: []string{"prune", "--profile", "profile.json", "extra\napply=true"}, want: `prune: unexpected arguments: "extra\napply=true"`},
		{name: "unsupported format", args: []string{"prune", "--profile", "profile.json", "--format", "yaml"}, want: `prune: unsupported format "yaml"`},
		{name: "mutually exclusive modes", args: []string{"prune", "--profile", "profile.json", "--dry-run", "--apply"}, want: "prune: --dry-run and --apply are mutually exclusive"},
		{name: "delete policy override", args: []string{"prune", "--profile", "profile.json", "--delete-policy", "prune"}, want: "flag provided but not defined"},
		{name: "review override", args: []string{"prune", "--profile", "profile.json", "--require-review=false"}, want: "flag provided but not defined"},
		{name: "physical prune override", args: []string{"prune", "--profile", "profile.json", "--allow-physical-prune"}, want: "flag provided but not defined"},
		{name: "target override", args: []string{"prune", "--profile", "profile.json", "--target", "/var/tmp/target"}, want: "flag provided but not defined"},
		{name: "approval without apply", args: []string{"prune", "--profile", "profile.json", "--approval", "approval1"}, want: "--approval is only valid with --apply"},
		{name: "review missing profile", args: []string{"prune", "review"}, want: "prune review: --profile is required"},
		{name: "review unsupported format", args: []string{"prune", "review", "--profile", "profile.json", "--format", "yaml"}, want: `prune review: unsupported format "yaml"`},
		{name: "review target override", args: []string{"prune", "review", "--profile", "profile.json", "--target", "/var/tmp/target"}, want: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d, stderr = %q, want 2", tt.args, got, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("Run(%v) stderr = %q, want %q", tt.args, stderr.String(), tt.want)
			}
			if strings.Count(stderr.String(), "\n") > 1 || strings.Contains(stderr.String(), "\napply=true") {
				t.Fatalf("Run(%v) stderr = %q, want single safe diagnostic line", tt.args, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("Run(%v) stdout = %q, want empty", tt.args, stdout.String())
			}
		})
	}
}

func TestLeafHelpReturnsSuccess(t *testing.T) {
	tests := [][]string{
		{"profile", "--help"},
		{"profile", "init", "--help"},
		{"profile", "lint", "--help"},
		{"profile", "set-target", "--help"},
		{"profile", "set-network", "--help"},
		{"scan", "--help"},
		{"push", "--help"},
		{"verify", "--help"},
		{"drift", "--help"},
		{"drift", "list", "--help"},
		{"drift", "record", "--help"},
		{"drift", "acknowledge", "--help"},
		{"deleted", "--help"},
		{"deleted", "list", "--help"},
		{"prune", "--help"},
		{"prune", "review", "--help"},
		{"prune", "approve", "--help"},
		{"health", "--help"},
		{"report", "--help"},
		{"recover", "--help"},
		{"serve", "--help"},
		{"discover", "--help"},
		{"pair", "--help"},
		{"push", "--network", "--help"},
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
			if !strings.Contains(stdout.String(), "Usage") {
				t.Fatalf("Run(%v) stdout = %q, want usage text", args, stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("Run(%v) stderr = %q, want empty", args, stderr.String())
			}
		})
	}
}

func TestLANTrustCommandHelpIsHonest(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		want      []string
		forbidden []string
	}{
		{
			name: "serve",
			args: []string{"serve", "--help"},
			want: []string{
				"Usage of serve",
				"--profile",
				"unpaired profiles serve pairing only",
				"paired receiver material must be complete",
				"receiver address comes from profile network.receiver_url",
			},
			forbidden: []string{"encrypted transfer ready", "anonymous", "--cert", "--key"},
		},
		{
			name: "daemon",
			args: []string{"daemon", "--help"},
			want: []string{
				"supermover daemon install",
				"supermover daemon run --foreground",
				"profile as the runtime SSOT",
				"does not install an OS service manager",
			},
			forbidden: []string{"background daemon ready", "LAN browsing ready", "--cert", "--key"},
		},
		{
			name: "discover",
			args: []string{"discover", "--help"},
			want: []string{
				"Usage of discover",
				"--timeout",
				"--address",
				"address hints",
				"not trust",
			},
			forbidden: []string{"trusted target", "identity verified"},
		},
		{
			name: "pair",
			args: []string{"pair", "--help"},
			want: []string{
				"Usage of pair",
				"--profile",
				"--target",
				"verification",
				"writes local pairing evidence",
			},
			forbidden: []string{"starts transfer", "syncs files"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 0 {
				t.Fatalf("Run(%v) exit = %d, stderr = %q, want 0", tt.args, got, stderr.String())
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("Run(%v) stdout = %q, want %q", tt.args, stdout.String(), want)
				}
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(stdout.String(), forbidden) {
					t.Fatalf("Run(%v) stdout = %q, must not contain %q", tt.args, stdout.String(), forbidden)
				}
			}
			if stderr.Len() != 0 {
				t.Fatalf("Run(%v) stderr = %q, want empty", tt.args, stderr.String())
			}
		})
	}
}

func TestServeStartsPairingOnlyServerAndCancelsCleanly(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile-local", "Secret Profile", source, target)
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan pairserve.ReadyInfo, 1)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{
		Context: ctx,
		ServePairingReady: func(info pairserve.ReadyInfo) {
			ready <- info
		},
	}

	go func() {
		done <- runner.Run([]string{"serve", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()

	var info pairserve.ReadyInfo
	select {
	case info = <-ready:
	case <-time.After(2 * time.Second):
		t.Fatalf("serve did not report ready; stderr=%q", stderr.String())
	}
	address := info.Address

	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + address + "/v1/discovery")
	if err != nil {
		t.Fatalf("GET /v1/discovery error = %v, want nil", err)
	}
	body := readHTTPBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/discovery status = %d body = %q, want 200", resp.StatusCode, body)
	}
	assertServeLowInfo(t, body, p, source, target)
	if !strings.Contains(body, `"trusted":false`) {
		t.Fatalf("GET /v1/discovery body = %q, want trusted=false", body)
	}
	resp, err = client.Get("http://" + address + "/v1/pairing")
	if err != nil {
		t.Fatalf("GET /v1/pairing without code error = %v, want nil", err)
	}
	body = readHTTPBody(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("GET /v1/pairing without code status = %d body = %q, want 403", resp.StatusCode, body)
	}
	assertServeLowInfo(t, body, p, source, target)
	reqPairing, err := http.NewRequest(http.MethodGet, "http://"+address+"/v1/pairing", nil)
	if err != nil {
		t.Fatalf("NewRequest(/v1/pairing) error = %v, want nil", err)
	}
	reqPairing.Header.Set(pairing.VerificationCodeHeader, info.VerificationCode)
	resp, err = client.Do(reqPairing)
	if err != nil {
		t.Fatalf("GET /v1/pairing with code error = %v, want nil", err)
	}
	body = readHTTPBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/pairing with code status = %d body = %q, want 200", resp.StatusCode, body)
	}
	if !strings.Contains(body, `"trusted":false`) {
		t.Fatalf("GET /v1/pairing with code body = %q, want trusted=false", body)
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+address+"/v1/sessions", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("NewRequest(/v1/sessions) error = %v, want nil", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/sessions error = %v, want nil", err)
	}
	body = readHTTPBody(t, resp)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST /v1/sessions status = %d body = %q, want 403", resp.StatusCode, body)
	}
	if !strings.Contains(body, "disabled") {
		t.Fatalf("POST /v1/sessions body = %q, want transfer disabled", body)
	}
	if strings.TrimSpace(info.VerificationCode) == "" || strings.TrimSpace(info.TargetDeviceID) == "" || info.ExpiresAt.IsZero() {
		t.Fatalf("serve ready info = %+v, want pairing code, target device id, and expiry", info)
	}

	cancel()
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("serve exit = %d stderr = %q, want 0", got, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("serve did not exit after cancel; stderr=%q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("serve stdout = %q, want empty", stdout.String())
	}
	assertServeOperatorOutputLowInfo(t, stderr.String(), p, source, target)
	if !strings.Contains(stderr.String(), "verification_code=") {
		t.Fatalf("serve stderr = %q, want operator verification code", stderr.String())
	}
	if _, err := os.Lstat(filepath.Join(target, ".supermover")); !os.IsNotExist(err) {
		t.Fatalf("serve .supermover state error = %v, want not exist", err)
	}
}

func TestDashboardStartsLoopbackReadOnlyPageAndCancelsCleanly(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile-local", "Target Dashboard", source, target)
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan operatorui.ReadyInfo, 1)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{
		Context: ctx,
		DashboardReady: func(info operatorui.ReadyInfo) {
			ready <- info
		},
	}
	go func() {
		done <- runner.Run([]string{"dashboard", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()
	var info operatorui.ReadyInfo
	select {
	case info = <-ready:
	case <-time.After(2 * time.Second):
		t.Fatalf("dashboard did not report ready; stderr=%q", stderr.String())
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(info.URL)
	if err != nil {
		t.Fatalf("GET dashboard page error = %v, want nil", err)
	}
	body := readHTTPBody(t, resp)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "Run full verification") {
		t.Fatalf("GET dashboard page status/body = %d/%q, want page", resp.StatusCode, body)
	}
	integrityURL := strings.Replace(info.URL, "/?token=", "/api/integrity?token=", 1)
	req, err := http.NewRequest(http.MethodGet, integrityURL, nil)
	if err != nil {
		t.Fatalf("NewRequest(dashboard integrity) error = %v, want nil", err)
	}
	req.Header.Set("X-Supermover-Dashboard", "1")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("GET dashboard integrity error = %v, want nil", err)
	}
	body = readHTTPBody(t, resp)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"read_only":true`) || !strings.Contains(body, `"status":"review_required"`) {
		t.Fatalf("GET dashboard integrity status/body = %d/%q, want read-only no-manifest review", resp.StatusCode, body)
	}
	cancel()
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("dashboard exit = %d stderr = %q, want 0", got, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("dashboard did not exit after cancel; stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "dashboard: url=http://") || !strings.Contains(stderr.String(), "loopback_only=true") {
		t.Fatalf("dashboard stderr = %q, want loopback operator URL", stderr.String())
	}
	if _, err := os.Lstat(filepath.Join(target, control.DirName)); !os.IsNotExist(err) {
		t.Fatalf("dashboard .supermover state error = %v, want no read-only artifact writes", err)
	}
}

func TestDashboardRejectsNonLoopbackListener(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	if err := profile.WriteFile(profilePath, profile.NewDefault("profile-local", "Target Dashboard", source, target)); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"dashboard", "--profile", profilePath, "--listen", "0.0.0.0:8787"}, &stdout, &stderr)
	if got != 2 || !strings.Contains(stderr.String(), "loopback") {
		t.Fatalf("dashboard non-loopback exit/stderr = %d/%q, want setup refusal", got, stderr.String())
	}
}

func TestServeStartsPairingAndProfileBackedTLSReceiver(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	p := profile.NewDefault(peer.ProfileID, "Secret Profile", source, target)
	p.Target.TargetID = peer.TargetID
	p.Target.DevicePublicKey = peer.TargetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	p.Network = networkConfigForCLI(t, targetCert, reserveTCPAddress(t))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = p.Target.PairedAt
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pairingReady := make(chan pairserve.ReadyInfo, 1)
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{
		Context: ctx,
		Now:     cliTLSNow(),
		ServePairingReady: func(info pairserve.ReadyInfo) {
			pairingReady <- info
		},
		ServeReceiverReady: func(info receiverserve.ReadyInfo) {
			receiverReady <- info
		},
	}

	go func() {
		done <- runner.Run([]string{"serve", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()

	pairingInfo := waitServePairingReady(t, pairingReady, &stderr)
	receiverInfo := waitServeReceiverReady(t, receiverReady, &stderr)
	if receiverInfo.Peer != peer {
		t.Fatalf("receiver peer = %+v, want %+v", receiverInfo.Peer, peer)
	}
	plainClient := http.Client{Timeout: 2 * time.Second}
	resp, err := plainClient.Get("http://" + pairingInfo.Address + "/v1/discovery")
	if err != nil {
		t.Fatalf("GET pairing discovery error = %v, want nil", err)
	}
	body := readHTTPBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET pairing discovery status = %d body = %q, want 200", resp.StatusCode, body)
	}
	assertServeLowInfo(t, body, p, source, target)
	begin := validCLIBeginRequest(peer, []byte("serve receiver route\n"))
	resp, err = plainClient.Do(newCLIBeginRequest(t, "http://"+receiverInfo.Address+"/v1/sessions", begin))
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode < 400 {
			t.Fatalf("plain receiver POST status = %d, want failure", resp.StatusCode)
		}
	}
	if _, err := os.Lstat(filepath.Join(target, control.DirName, "sessions", begin.SessionID)); !os.IsNotExist(err) {
		t.Fatalf("plain receiver session dir err = %v, want no session artifact", err)
	}
	client := pinnedCLIClient(t, sourceCert, peer)
	resp, err = client.Do(newCLIBeginRequest(t, "https://"+receiverInfo.Address+"/v1/sessions", begin))
	if err != nil {
		t.Fatalf("POST receiver /v1/sessions error = %v, want nil", err)
	}
	body = readHTTPBody(t, resp)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST receiver /v1/sessions status = %d body = %q, want 202", resp.StatusCode, body)
	}
	resp, err = client.Get("https://" + receiverInfo.Address + "/v1/sessions/" + begin.SessionID + "/status")
	if err != nil {
		t.Fatalf("GET receiver status error = %v, want nil", err)
	}
	body = readHTTPBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET receiver status = %d body = %q, want 200", resp.StatusCode, body)
	}
	cancel()
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("serve exit = %d stderr = %q, want 0", got, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("serve did not exit after cancel; stderr=%q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("serve stdout = %q, want empty", stdout.String())
	}
	assertServeOperatorOutputLowInfo(t, stderr.String(), p, source, target)
	if !strings.Contains(stderr.String(), "receiver_routes=true") || !strings.Contains(stderr.String(), "push_network=true") {
		t.Fatalf("serve stderr = %q, want receiver route readiness for push-network", stderr.String())
	}
}

func TestServeStartsPairingOnlyForUnpairedProfileWithReceiverMaterial(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Network = networkConfigForCLI(t, targetCert, reserveTCPAddress(t))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pairingReady := make(chan pairserve.ReadyInfo, 1)
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	go func() {
		done <- Runner{
			Context: ctx,
			ServePairingReady: func(info pairserve.ReadyInfo) {
				pairingReady <- info
			},
			ServeReceiverReady: func(info receiverserve.ReadyInfo) {
				receiverReady <- info
			},
		}.Run([]string{"serve", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()
	info := waitServePairingReady(t, pairingReady, &stderr)
	if info.Address == "" {
		t.Fatal("serve pairing address is empty")
	}
	select {
	case receiver := <-receiverReady:
		t.Fatalf("serve receiver ready = %+v, want no receiver for unpaired profile", receiver)
	default:
	}
	cancel()
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("serve exit = %d stderr = %q, want 0", got, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("serve did not exit after cancel; stderr=%q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("serve stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "mode=pairing-only") || strings.Contains(stderr.String(), "receiver_routes=true") {
		t.Fatalf("serve stderr = %q, want pairing-only without receiver routes", stderr.String())
	}
}

func TestServeRejectsPartialReceiverMaterialWithoutPairingFallback(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p)
	profileJSON := readFileString(t, profilePath)
	withPartialNetwork := strings.Replace(profileJSON, `"agent_knowledge"`, `"network": {"receiver_url":"https://127.0.0.1:9443"}, "agent_knowledge"`, 1)
	if withPartialNetwork == profileJSON {
		t.Fatal("test fixture did not inject partial network material")
	}
	mustWrite(t, profilePath, withPartialNetwork)
	ready := make(chan pairserve.ReadyInfo, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Runner{
		ServePairingReady: func(info pairserve.ReadyInfo) {
			ready <- info
		},
	}.Run([]string{"serve", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("serve partial receiver material exit = %d stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "network.local_tls_identity is required") {
		t.Fatalf("serve partial receiver material stderr = %q, want TLS identity refusal", stderr.String())
	}
	select {
	case info := <-ready:
		t.Fatalf("serve pairing ready = %+v, want no pairing fallback", info)
	default:
	}
	if stdout.Len() != 0 {
		t.Fatalf("serve partial receiver material stdout = %q, want empty", stdout.String())
	}
}

func TestServeReceiverListenFailureDoesNotStartPairingOrReceiver(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	blocked := listenOnReservedTCPAddress(t)
	defer blocked.Close()
	p := profile.NewDefault(peer.ProfileID, "Local profile", source, target)
	p.Target.TargetID = peer.TargetID
	p.Target.DevicePublicKey = peer.TargetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	p.Network = networkConfigForCLI(t, targetCert, blocked.Addr().String())
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = p.Target.PairedAt
	})
	pairingReady := make(chan pairserve.ReadyInfo, 1)
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{
		Now: cliTLSNow(),
		ServePairingReady: func(info pairserve.ReadyInfo) {
			pairingReady <- info
		},
		ServeReceiverReady: func(info receiverserve.ReadyInfo) {
			receiverReady <- info
		},
	}.Run([]string{"serve", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("serve receiver bind failure exit = %d stderr = %q, want 1", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "serve: receiver:") || !strings.Contains(stderr.String(), "bind") && !strings.Contains(stderr.String(), "address already in use") {
		t.Fatalf("serve receiver bind failure stderr = %q, want receiver listen diagnostic", stderr.String())
	}
	select {
	case info := <-pairingReady:
		t.Fatalf("serve pairing ready = %+v, want no readiness after receiver bind failure", info)
	default:
	}
	select {
	case info := <-receiverReady:
		t.Fatalf("serve receiver ready = %+v, want no readiness after receiver bind failure", info)
	default:
	}
	if stdout.Len() != 0 {
		t.Fatalf("serve receiver bind failure stdout = %q, want empty", stdout.String())
	}
}

func TestDaemonInstallStatusAndStopPersistLifecycleEvidence(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	runner := Runner{Now: now}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := runner.Run([]string{"daemon", "install", "--profile", profilePath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("daemon install exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "daemon install", stdout.String(),
		"daemon: installed",
		"profile=profile-local",
		"run_mode=foreground",
		"service_manager=none",
		"daemon%20run%20--foreground%20--profile",
	)
	if stderr.Len() != 0 {
		t.Fatalf("daemon install stderr = %q, want empty", stderr.String())
	}
	if _, err := os.Lstat(filepath.Join(target, control.DirName, "daemon", "install.json")); err != nil {
		t.Fatalf("daemon install artifact error = %v, want present", err)
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"daemon", "status", "--profile", profilePath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("daemon status exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "daemon status", stdout.String(),
		"daemon_status",
		"installed=true",
		"state=installed",
		"run_mode=foreground",
		"service_manager=none",
		"stop_requested=false",
		"restart_requested=false",
		"lifecycle_events=1",
	)
	if stderr.Len() != 0 {
		t.Fatalf("daemon status stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"daemon", "status", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("daemon status json exit = %d stderr = %q, want 0", got, stderr.String())
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("daemon status json = %q decode error = %v", stdout.String(), err)
	}
	if report["installed"] != true || report["state"] != "installed" || report["service_manager"] != "none" {
		t.Fatalf("daemon status json = %#v, want installed foreground state", report)
	}
	if stderr.Len() != 0 {
		t.Fatalf("daemon status json stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"daemon", "stop", "--profile", profilePath, "--reason", "operator review"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("daemon stop exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "daemon stop", stdout.String(), "daemon: stop_requested", "foreground_signal=stop-intent")
	if stderr.Len() != 0 {
		t.Fatalf("daemon stop stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	got = runner.Run([]string{"daemon", "status", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("daemon status after stop exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "daemon status after stop", stdout.String(), "stop_requested=true", "reason=operator%20review")
	if stderr.Len() != 0 {
		t.Fatalf("daemon status after stop stderr = %q, want empty", stderr.String())
	}
}

func TestDaemonHelpIncludesLogsAndRestartWithoutOverclaiming(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"daemon", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("daemon --help exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "daemon help", stdout.String(),
		"supermover daemon logs --profile <path>",
		"supermover daemon restart --profile <path>",
		"foreground agent lifecycle evidence",
		"profile as the runtime SSOT",
		"profile enables sync.local_polling",
		"profile enables repair.drift_recording",
		"without applying\nrepair or reconcile mutations",
		"profile enables repair.persisted_reconcile_apply",
		"stops after any refusal for operator review",
		"profile enables sync.network_polling",
		"source-side foreground\nnetwork queue worker",
		"per-entry profile-backed mTLS queue publication",
		"automatic\ndiscovery-selected sync",
		"select endpoints automatically",
	)
	assertTextContainsNone(t, "daemon help", stdout.String(), "background daemon ready", "LAN browsing ready", "starts ongoing incremental sync", "OS file watcher ready", "per-entry network transport ready", "retry policy ready")
	if stderr.Len() != 0 {
		t.Fatalf("daemon --help stderr = %q, want empty", stderr.String())
	}
}

func TestDaemonLogsShowsOnlyScopedRedactedLifecycleEvents(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	if _, err := agentdaemon.AppendLifecycleEvent(target, agentdaemon.NewLifecycleEvent("profile-other", "local:profile-local", "daemon_started", "foreign", nil, now)); err != nil {
		t.Fatalf("AppendLifecycleEvent(foreign) error = %v, want nil", err)
	}
	if _, err := agentdaemon.AppendLifecycleEvent(target, agentdaemon.NewLifecycleEvent("profile-local", "local:profile-local", "daemon_running", "pairing verification code 123456", map[string]string{
		"mode":         "pairing-only",
		"stderr":       "raw stderr",
		"pairing_code": "123456",
	}, now.Add(time.Second))); err != nil {
		t.Fatalf("AppendLifecycleEvent(scoped) error = %v, want nil", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"daemon", "logs", "--profile", profilePath, "--tail", "1"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("daemon logs exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "daemon logs", stdout.String(),
		"daemon_logs",
		"events=1",
		"scope_issues=lifecycle_event_scope_mismatch",
		"type=daemon_running",
		"message=%5Bredacted%5D",
		"details=mode=pairing-only",
	)
	assertTextContainsNone(t, "daemon logs", stdout.String(), "foreign", "123456", "raw%20stderr", "pairing_code", "stderr=")
	if stderr.Len() != 0 {
		t.Fatalf("daemon logs stderr = %q, want empty", stderr.String())
	}
}

func TestDaemonRestartRefusesAbsentOrStoppedForegroundState(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"daemon", "restart", "--profile", profilePath, "--reason", "operator review"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("daemon restart absent exit = %d stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no foreground daemon state is present") {
		t.Fatalf("daemon restart absent stderr = %q, want absent-state diagnostic", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("daemon restart absent stdout = %q, want empty", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	state := agentdaemon.NewState("profile-local", "local:profile-local", profilePath, agentdaemon.StatusStopped, 0, now)
	state.StoppedAt = now.Format(time.RFC3339Nano)
	if err := agentdaemon.WriteState(target, state); err != nil {
		t.Fatalf("WriteState(stopped) error = %v, want nil", err)
	}

	got = Run([]string{"daemon", "restart", "--profile", profilePath, "--reason", "operator review"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("daemon restart stopped exit = %d stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "state=stopped is not a running foreground daemon") {
		t.Fatalf("daemon restart stopped stderr = %q, want stopped-state diagnostic", stderr.String())
	}
}

func TestDaemonRestartOnStaleRunningStateIsPendingEvidenceOnly(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	state := agentdaemon.NewState("profile-local", "local:profile-local", profilePath, agentdaemon.StatusRunning, 12345, now)
	if err := agentdaemon.WriteState(target, state); err != nil {
		t.Fatalf("WriteState(running) error = %v, want nil", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: now.Add(time.Second)}.Run([]string{"daemon", "restart", "--profile", profilePath, "--reason", "operator review"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("daemon restart stale running exit = %d stderr = %q, want 0 pending evidence", got, stderr.String())
	}
	assertTextContainsAll(t, "daemon restart stale running", stdout.String(), "restart_requested", "foreground_signal=restart-intent", "consumption=pending")
	if strings.Contains(stdout.String(), "restarted") || strings.Contains(stdout.String(), "consumption=consumed") {
		t.Fatalf("daemon restart stale running stdout = %q, must not claim consumed restart", stdout.String())
	}
	intent, err := agentdaemon.ReadRestartIntent(target)
	if err != nil {
		t.Fatalf("ReadRestartIntent() error = %v, want nil", err)
	}
	if intent.Reason != "operator review" || intent.ProfileID != "profile-local" || intent.TargetID != "local:profile-local" {
		t.Fatalf("restart intent = %+v, want scoped pending evidence", intent)
	}
}

func TestSyncQueueHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"sync", "queue", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync queue --help exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "sync queue help", stdout.String(),
		"Usage of sync queue",
		"supermover sync queue enqueue --profile <path>",
		"supermover sync queue status --profile <path>",
		"supermover sync queue list --profile <path>",
		"supermover sync queue ready --profile <path>",
		"supermover sync queue cancel --profile <path> --id <entry-id> --reason <text>",
		"supermover sync queue fail --profile <path> --id <entry-id> --reason <text>",
		"durable changed-file queue evidence only",
		"do not watch roots",
		"copy\nfiles",
		"run a daemon",
		"perform ongoing sync",
		"profile-selected",
		"target. The profile remains the SSOT",
	)
	for _, forbidden := range []string{"--state-dir", "--target", "--network", "--watch", "background sync ready"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("sync queue --help stdout = %q, must not expose %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync queue --help stderr = %q, want empty", stderr.String())
	}
}

func TestSyncQueueUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing nested command", args: []string{"sync"}, want: "sync: missing subcommand"},
		{name: "missing queue action", args: []string{"sync", "queue"}, want: "sync queue: missing subcommand"},
		{name: "unknown queue action", args: []string{"sync", "queue", "run"}, want: `sync queue: unknown subcommand "run"`},
		{name: "missing profile", args: []string{"sync", "queue", "status"}, want: "sync queue status: --profile is required"},
		{name: "unsupported format", args: []string{"sync", "queue", "status", "--profile", "profile.json", "--format", "yaml"}, want: `sync queue status: unsupported format "yaml"`},
		{name: "list unexpected args", args: []string{"sync", "queue", "list", "--profile", "profile.json", "extra"}, want: `sync queue list: unexpected arguments: extra`},
		{name: "unexpected args", args: []string{"sync", "queue", "ready", "--profile", "profile.json", "extra\narg"}, want: `sync queue ready: unexpected arguments: "extra\narg"`},
		{name: "cancel missing id", args: []string{"sync", "queue", "cancel", "--profile", "profile.json", "--reason", "skip"}, want: "sync queue cancel: --id is required"},
		{name: "cancel missing reason", args: []string{"sync", "queue", "cancel", "--profile", "profile.json", "--id", "entry"}, want: "sync queue cancel: --reason is required"},
		{name: "fail missing id", args: []string{"sync", "queue", "fail", "--profile", "profile.json", "--reason", "terminal"}, want: "sync queue fail: --id is required"},
		{name: "fail missing reason", args: []string{"sync", "queue", "fail", "--profile", "profile.json", "--id", "entry"}, want: "sync queue fail: --reason is required"},
		{name: "state dir flag rejected", args: []string{"sync", "queue", "status", "--profile", "profile.json", "--state-dir", "tmp"}, want: "flag provided but not defined"},
		{name: "watch flag rejected", args: []string{"sync", "queue", "enqueue", "--profile", "profile.json", "--watch"}, want: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d stderr = %q, want 2", tt.args, got, stderr.String())
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

func TestSyncQueueEnqueueReadyStatusAndCancel(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, ".hidden", "secret.txt"), "secret")
	mustWrite(t, filepath.Join(source, "visible.txt"), "public")
	writeDefaultProfile(t, profilePath, source, target)
	runner := Runner{Now: time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := runner.Run([]string{"sync", "queue", "enqueue", "--profile", profilePath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync queue enqueue exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "sync queue enqueue", stdout.String(),
		"sync_queue_enqueue",
		"profile=profile-local",
		"target_id=local:profile-local",
		"queued=3",
		"ready=3",
		"total=3",
		"path=.hidden%2Fsecret.txt",
		"path=visible.txt",
		"mode=queue_only",
	)
	if strings.Contains(stdout.String(), "ongoing") || strings.Contains(stdout.String(), "watch") || strings.Contains(stdout.String(), "copied") {
		t.Fatalf("sync queue enqueue stdout = %q, must not imply execution loop", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync queue enqueue stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "ready", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue ready json exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var ready syncQueueEntriesResult
	if err := json.Unmarshal(stdout.Bytes(), &ready); err != nil {
		t.Fatalf("json.Unmarshal(sync queue ready stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if ready.Operation != "ready" || len(ready.Entries) != 3 || ready.Summary.Ready != 3 || ready.Mode != "queue_only" {
		t.Fatalf("sync queue ready result = %+v, want three ready queue-only entries", ready)
	}
	hiddenID := ""
	for _, entry := range ready.Entries {
		if entry.Path == ".hidden/secret.txt" {
			hiddenID = entry.ID
		}
	}
	if hiddenID == "" {
		t.Fatalf("sync queue ready entries = %+v, want hidden file entry", ready.Entries)
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "cancel", "--profile", profilePath, "--id", hiddenID, "--reason", "operator skip"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue cancel exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "sync queue cancel", stdout.String(),
		"sync_queue_cancel",
		"id="+hiddenID,
		"status=canceled",
		"reason=operator%20skip",
	)
	if stderr.Len() != 0 {
		t.Fatalf("sync queue cancel stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "status", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue status json exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var status syncQueueSummaryResult
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatalf("json.Unmarshal(sync queue status stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if status.Operation != "status" || status.Summary.Canceled != 1 || status.Summary.Ready != 2 || status.Summary.Total != 3 || status.Mode != "queue_only" {
		t.Fatalf("sync queue status result = %+v, want canceled hidden entry and two ready entries", status)
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "ready", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue ready text exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "path=.hidden%2Fsecret.txt") {
		t.Fatalf("sync queue ready after cancel stdout = %q, want canceled hidden entry excluded", stdout.String())
	}
	assertTextContainsAll(t, "sync queue ready after cancel", stdout.String(), "ready=2", "path=visible.txt")
	if stderr.Len() != 0 {
		t.Fatalf("sync queue ready after cancel stderr = %q, want empty", stderr.String())
	}
}

func TestSyncQueueListShowsAllEntryLifecycleDetails(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, ".hidden", "secret.txt"), "secret")
	mustWrite(t, filepath.Join(source, "visible.txt"), "public")
	writeDefaultProfile(t, profilePath, source, target)
	runner := Runner{Now: time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := runner.Run([]string{"sync", "queue", "enqueue", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue enqueue exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "ready", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue ready json exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var ready syncQueueEntriesResult
	if err := json.Unmarshal(stdout.Bytes(), &ready); err != nil {
		t.Fatalf("json.Unmarshal(sync queue ready stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	hiddenID := findCLIQueuePath(t, ready.Entries, ".hidden/secret.txt").ID
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "fail", "--profile", profilePath, "--id", hiddenID, "--reason", "operator terminal failure"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue fail exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "list", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue list text exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync queue list stderr = %q, want empty", stderr.String())
	}
	assertTextContainsAll(t, "sync queue list", stdout.String(),
		"sync_queue_list",
		"failed=1",
		"ready=2",
		"sync_queue_entry",
		"path=.hidden%2Fsecret.txt",
		"path=visible.txt",
		"status=failed",
		"last_error=operator%20terminal%20failure",
		"failed_at=2026-05-21T01:02:03Z",
		"next_due_at=-",
		"done_at=-",
		"enqueued_at=2026-05-21T01:02:03Z",
	)
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "list", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue list json exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var listed syncQueueEntriesResult
	if err := json.Unmarshal(stdout.Bytes(), &listed); err != nil {
		t.Fatalf("json.Unmarshal(sync queue list stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if listed.Operation != "list" || listed.Mode != "queue_only" || listed.Summary.Failed != 1 || listed.Summary.Ready != 2 || len(listed.Entries) != 3 {
		t.Fatalf("sync queue list json result = %+v, want all persisted entries with failed summary", listed)
	}
	failed := findCLIQueuePath(t, listed.Entries, ".hidden/secret.txt")
	if failed.Status != incrementalsync.StatusFailed || failed.LastError != "operator terminal failure" || failed.FailedAt == "" {
		t.Fatalf("sync queue list failed entry = %+v, want failed lifecycle detail", failed)
	}
}

func TestSyncQueueFailMarksTerminalReviewState(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, ".hidden", "secret.txt"), "secret")
	writeDefaultProfile(t, profilePath, source, target)
	runner := Runner{Now: time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := runner.Run([]string{"sync", "queue", "enqueue", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue enqueue exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "ready", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue ready json exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var ready syncQueueEntriesResult
	if err := json.Unmarshal(stdout.Bytes(), &ready); err != nil {
		t.Fatalf("json.Unmarshal(sync queue ready stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	hiddenID := findCLIQueuePath(t, ready.Entries, ".hidden/secret.txt").ID
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "fail", "--profile", profilePath, "--id", hiddenID, "--reason", "operator terminal failure", "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue fail json exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var failed syncQueueFailResult
	if err := json.Unmarshal(stdout.Bytes(), &failed); err != nil {
		t.Fatalf("json.Unmarshal(sync queue fail stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if failed.Operation != "fail" || failed.Mode != "queue_only" || failed.Entry.Status != incrementalsync.StatusFailed || failed.Entry.FailedAt == "" || failed.Entry.LastError != "operator terminal failure" {
		t.Fatalf("sync queue fail result = %+v, want failed terminal review evidence", failed)
	}
	if failed.Summary.Failed != 1 || failed.Summary.Ready != 1 || failed.Summary.Total != 2 {
		t.Fatalf("sync queue fail summary = %+v, want failed file and still-ready parent directory", failed.Summary)
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "ready", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue ready after fail exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "path=.hidden%2Fsecret.txt") {
		t.Fatalf("sync queue ready after fail stdout = %q, want failed file excluded", stdout.String())
	}
	assertTextContainsAll(t, "sync queue ready after fail", stdout.String(), "failed=1", "ready=1", "path=.hidden")
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "run", "--profile", profilePath, "--session", "sync-run-failed-terminal"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync run after fail exit = %d stderr = %q stdout = %q, want 0 for remaining ready directory", got, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "path=.hidden%2Fsecret.txt") {
		t.Fatalf("sync run after fail stdout = %q, must not publish failed file entry", stdout.String())
	}
	assertTextContainsAll(t, "sync run after fail", stdout.String(), "failed=1", "done=1")
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"status", "--profile", profilePath}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("status after failed queue exit = %d stderr = %q stdout = %q, want review-needed exit 1", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "status after failed queue", stdout.String(),
		"incremental_sync_status=review_required",
		"incremental_sync_failed=1",
		"incremental_sync_ready=0",
	)
}

func TestSyncQueueStatusMissingQueueIsReadOnlyEmpty(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	before := mustSnapshotTree(t, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"sync", "queue", "status", "--profile", profilePath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync queue status empty exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "sync queue empty status", stdout.String(), "sync_queue_status", "queued=0", "ready=0", "total=0", "state=missing")
	if stderr.Len() != 0 {
		t.Fatalf("sync queue status empty stderr = %q, want empty", stderr.String())
	}
	after := mustSnapshotTree(t, target)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("sync queue status empty changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

func TestSyncQueueReadyMissingQueueIsReadOnlyEmpty(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	before := mustSnapshotTree(t, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"sync", "queue", "ready", "--profile", profilePath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync queue ready empty exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "sync queue empty ready", stdout.String(), "sync_queue_ready", "ready=0", "total=0", "state=missing")
	if strings.Contains(stdout.String(), "sync_queue_ready_entry") {
		t.Fatalf("sync queue empty ready stdout = %q, want no entries", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync queue ready empty stderr = %q, want empty", stderr.String())
	}
	after := mustSnapshotTree(t, target)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("sync queue ready empty changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

func TestSyncQueueRefusesSymlinkControlSurface(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	outside := filepath.Join(dir, "outside")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustMkdir(t, outside)
	writeDefaultProfile(t, profilePath, source, target)
	linkPath := filepath.Join(target, control.DirName, "incremental-sync")
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(linkPath), err)
	}
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"sync", "queue", "enqueue", "--profile", profilePath}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("sync queue unsafe enqueue exit = %d stderr = %q stdout = %q, want 2", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "sync queue enqueue:") || !strings.Contains(stderr.String(), "symlink") {
		t.Fatalf("sync queue unsafe enqueue stderr = %q, want symlink diagnostic", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("sync queue unsafe enqueue stdout = %q, want empty", stdout.String())
	}
}

func TestSyncRunHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"sync", "run", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync run --help exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "sync run help", stdout.String(),
		"Usage of sync run",
		"supermover sync run --profile <path> --session <id>",
		"one bounded incremental sync pass",
		"profile-selected target",
		"durable changed-file queue",
		"existing local push safety path",
		"durable run receipt",
		"not a file watcher",
		"background daemon",
		"bidirectional sync engine",
		"daemon-integrated network sync",
		"LAN discovery\nexecutor",
	)
	for _, forbidden := range []string{"--state-dir", "--target", "--network", "--watch", "continuous sync ready"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("sync run --help stdout = %q, must not expose %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync run --help stderr = %q, want empty", stderr.String())
	}
}

func TestSyncLoopHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"sync", "loop", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync loop --help exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "sync loop help", stdout.String(),
		"Usage of sync loop",
		"supermover sync loop --profile <path> --session-prefix <id>",
		"foreground local polling loop",
		"profile-selected target",
		"durable changed-file queue",
		"existing local push safety path",
		"durable\nrun receipt",
		"--max-runs",
		"not an OS file watcher",
		"background daemon",
		"bidirectional sync engine",
		"daemon-integrated network sync",
		"LAN discovery\nexecutor",
	)
	for _, forbidden := range []string{"--state-dir", "--target", "--network", "--watch", "detached daemon", "background sync ready"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("sync loop --help stdout = %q, must not expose %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync loop --help stderr = %q, want empty", stderr.String())
	}
}

func TestSyncWatchHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"sync", "watch", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync watch --help exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "sync watch help", stdout.String(),
		"Usage of sync watch",
		"supermover sync watch --profile <path> --session-prefix <id>",
		"foreground OS file watcher",
		"profile-selected source roots",
		"existing local push safety path",
		"durable queue/run receipts",
		"--max-events",
		"not a background daemon",
		"bidirectional sync engine",
		"daemon-integrated network sync",
		"LAN discovery executor",
	)
	for _, forbidden := range []string{"--state-dir", "--target", "--network", "detached daemon", "network changed-file sync ready"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("sync watch --help stdout = %q, must not expose %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync watch --help stderr = %q, want empty", stderr.String())
	}
}

func TestSyncNetworkRunHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"sync", "network", "run", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync network run --help exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "sync network run help", stdout.String(),
		"Usage of sync network run",
		"supermover sync network run --profile <path> --session <id>",
		"one bounded network incremental pass",
		"profile-selected target\ncontrol plane",
		"durable queue/run receipts",
		"per-entry profile-backed mTLS network manifest",
		"previous published manifest evidence",
		"receiver-side\ntarget revalidation",
		"validates profile trust",
		"network material",
		"network push contract before queue mutation",
		"LAN discovery",
		"automatic endpoint selection",
		"foreground watcher",
		"background daemon",
		"broad\nrepair",
		"bidirectional sync",
	)
	for _, forbidden := range []string{"--state-dir", "--target", "--address", "--listen", "continuous network sync ready", "LAN browsing ready", "per-entry transport ready"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("sync network run --help stdout = %q, must not expose %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync network run --help stderr = %q, want empty", stderr.String())
	}
}

func TestSyncNetworkDiscoverRunHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"sync", "network", "discover-run", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync network discover-run --help exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "sync network discover-run help", stdout.String(),
		"Usage of sync network discover-run",
		"supermover sync network discover-run --profile <path> --session <id>",
		"LAN-discovery-gated network incremental pass",
		"low-info\navailability hint only",
		"match the profile-selected\nnetwork.receiver_url host:port",
		"per-entry profile-pinned mTLS transfer",
		"previous published\nevidence",
		"treats discovery as\ntrust",
		"not automatic endpoint selection",
		"not profile mutation",
		"not a foreground watcher",
		"not a background daemon",
		"not broad repair",
		"not bidirectional sync",
	)
	for _, forbidden := range []string{"--address", "--target", "trusted=true", "profile update", "per-entry transport ready"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("sync network discover-run --help stdout = %q, must not expose %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync network discover-run --help stderr = %q, want empty", stderr.String())
	}
}

func TestSyncNetworkLoopHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"sync", "network", "loop", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync network loop --help exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "sync network loop help", stdout.String(),
		"Usage of sync network loop",
		"supermover sync network loop --profile <path> --session-prefix <id>",
		"foreground network polling loop",
		"profile-selected target control\nplane",
		"durable queue/run receipts",
		"per-entry profile-backed mTLS network manifest",
		"previous published manifest evidence",
		"receiver-side target\nrevalidation",
		"--max-runs",
		"not LAN discovery",
		"not automatic endpoint selection",
		"not an OS file watcher",
		"not a background daemon",
		"not broad repair",
		"not bidirectional sync",
	)
	for _, forbidden := range []string{"--state-dir", "--target", "--address", "--listen", "LAN browsing ready", "per-entry transport ready"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("sync network loop --help stdout = %q, must not expose %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync network loop --help stderr = %q, want empty", stderr.String())
	}
}

func TestSyncRunUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing profile", args: []string{"sync", "run", "--session", "sync-run-1"}, want: "sync run: --profile is required"},
		{name: "missing session", args: []string{"sync", "run", "--profile", "profile.json"}, want: "sync run: --session is required"},
		{name: "padded session", args: []string{"sync", "run", "--profile", "profile.json", "--session", " sync-run-1"}, want: "sync run: --session must not be padded"},
		{name: "unsafe session", args: []string{"sync", "run", "--profile", "profile.json", "--session", "../x"}, want: "unsafe session id"},
		{name: "unsupported format", args: []string{"sync", "run", "--profile", "profile.json", "--session", "sync-run-1", "--format", "yaml"}, want: `sync run: unsupported format "yaml"`},
		{name: "negative backoff", args: []string{"sync", "run", "--profile", "profile.json", "--session", "sync-run-1", "--retry-backoff", "-1s"}, want: "sync run: --retry-backoff cannot be negative"},
		{name: "unexpected args", args: []string{"sync", "run", "--profile", "profile.json", "--session", "sync-run-1", "extra\narg"}, want: `sync run: unexpected arguments: "extra\narg"`},
		{name: "watch flag rejected", args: []string{"sync", "run", "--profile", "profile.json", "--session", "sync-run-1", "--watch"}, want: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d stderr = %q, want 2", tt.args, got, stderr.String())
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

func TestSyncLoopUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing profile", args: []string{"sync", "loop", "--session-prefix", "sync-loop"}, want: "sync loop: --profile is required"},
		{name: "missing session prefix", args: []string{"sync", "loop", "--profile", "profile.json"}, want: "sync loop: --session-prefix is required"},
		{name: "padded session prefix", args: []string{"sync", "loop", "--profile", "profile.json", "--session-prefix", " sync-loop"}, want: "sync loop: --session-prefix must not be padded"},
		{name: "unsafe session prefix", args: []string{"sync", "loop", "--profile", "profile.json", "--session-prefix", "../x"}, want: "unsafe session id"},
		{name: "unsupported format", args: []string{"sync", "loop", "--profile", "profile.json", "--session-prefix", "sync-loop", "--format", "yaml"}, want: `sync loop: unsupported format "yaml"`},
		{name: "zero interval", args: []string{"sync", "loop", "--profile", "profile.json", "--session-prefix", "sync-loop", "--interval", "0"}, want: "sync loop: --interval must be greater than zero"},
		{name: "negative max runs", args: []string{"sync", "loop", "--profile", "profile.json", "--session-prefix", "sync-loop", "--max-runs", "-1"}, want: "sync loop: --max-runs cannot be negative"},
		{name: "negative backoff", args: []string{"sync", "loop", "--profile", "profile.json", "--session-prefix", "sync-loop", "--retry-backoff", "-1s"}, want: "sync loop: --retry-backoff cannot be negative"},
		{name: "unexpected args", args: []string{"sync", "loop", "--profile", "profile.json", "--session-prefix", "sync-loop", "extra\narg"}, want: `sync loop: unexpected arguments: "extra\narg"`},
		{name: "watch flag rejected", args: []string{"sync", "loop", "--profile", "profile.json", "--session-prefix", "sync-loop", "--watch"}, want: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d stderr = %q, want 2", tt.args, got, stderr.String())
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

func TestSyncWatchUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing profile", args: []string{"sync", "watch", "--session-prefix", "sync-watch"}, want: "sync watch: --profile is required"},
		{name: "missing session prefix", args: []string{"sync", "watch", "--profile", "profile.json"}, want: "sync watch: --session-prefix is required"},
		{name: "padded session prefix", args: []string{"sync", "watch", "--profile", "profile.json", "--session-prefix", " sync-watch"}, want: "sync watch: --session-prefix must not be padded"},
		{name: "unsafe session prefix", args: []string{"sync", "watch", "--profile", "profile.json", "--session-prefix", "../x"}, want: "unsafe session id"},
		{name: "unsupported format", args: []string{"sync", "watch", "--profile", "profile.json", "--session-prefix", "sync-watch", "--format", "yaml"}, want: `sync watch: unsupported format "yaml"`},
		{name: "zero settle", args: []string{"sync", "watch", "--profile", "profile.json", "--session-prefix", "sync-watch", "--settle", "0"}, want: "sync watch: --settle must be greater than zero"},
		{name: "negative max events", args: []string{"sync", "watch", "--profile", "profile.json", "--session-prefix", "sync-watch", "--max-events", "-1"}, want: "sync watch: --max-events cannot be negative"},
		{name: "negative backoff", args: []string{"sync", "watch", "--profile", "profile.json", "--session-prefix", "sync-watch", "--retry-backoff", "-1s"}, want: "sync watch: --retry-backoff cannot be negative"},
		{name: "unexpected args", args: []string{"sync", "watch", "--profile", "profile.json", "--session-prefix", "sync-watch", "extra\narg"}, want: `sync watch: unexpected arguments: "extra\narg"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d stderr = %q, want 2", tt.args, got, stderr.String())
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

func TestSyncNetworkRunUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing network subcommand", args: []string{"sync", "network"}, want: "sync network: missing subcommand"},
		{name: "unknown network subcommand", args: []string{"sync", "network", "watch"}, want: `sync network: unknown subcommand "watch"`},
		{name: "missing profile", args: []string{"sync", "network", "run", "--session", "sync-net-1"}, want: "sync network run: --profile is required"},
		{name: "missing session", args: []string{"sync", "network", "run", "--profile", "profile.json"}, want: "sync network run: --session is required"},
		{name: "padded session", args: []string{"sync", "network", "run", "--profile", "profile.json", "--session", " sync-net-1"}, want: "sync network run: --session must not be padded"},
		{name: "unsafe session", args: []string{"sync", "network", "run", "--profile", "profile.json", "--session", "../x"}, want: "unsafe session id"},
		{name: "unsupported format", args: []string{"sync", "network", "run", "--profile", "profile.json", "--session", "sync-net-1", "--format", "yaml"}, want: `sync network run: unsupported format "yaml"`},
		{name: "negative backoff", args: []string{"sync", "network", "run", "--profile", "profile.json", "--session", "sync-net-1", "--retry-backoff", "-1s"}, want: "sync network run: --retry-backoff cannot be negative"},
		{name: "unexpected args", args: []string{"sync", "network", "run", "--profile", "profile.json", "--session", "sync-net-1", "extra\narg"}, want: `sync network run: unexpected arguments: "extra\narg"`},
		{name: "watch flag rejected", args: []string{"sync", "network", "run", "--profile", "profile.json", "--session", "sync-net-1", "--watch"}, want: "flag provided but not defined"},
		{name: "discover missing profile", args: []string{"sync", "network", "discover-run", "--session", "sync-net-1"}, want: "sync network discover-run: --profile is required"},
		{name: "discover missing session", args: []string{"sync", "network", "discover-run", "--profile", "profile.json"}, want: "sync network discover-run: --session is required"},
		{name: "discover padded session", args: []string{"sync", "network", "discover-run", "--profile", "profile.json", "--session", " sync-net-1"}, want: "sync network discover-run: --session must not be padded"},
		{name: "discover unsafe session", args: []string{"sync", "network", "discover-run", "--profile", "profile.json", "--session", "../x"}, want: "unsafe session id"},
		{name: "discover unsupported format", args: []string{"sync", "network", "discover-run", "--profile", "profile.json", "--session", "sync-net-1", "--format", "yaml"}, want: `sync network discover-run: unsupported format "yaml"`},
		{name: "discover zero timeout", args: []string{"sync", "network", "discover-run", "--profile", "profile.json", "--session", "sync-net-1", "--timeout", "0"}, want: "sync network discover-run: --timeout must be greater than zero"},
		{name: "discover negative backoff", args: []string{"sync", "network", "discover-run", "--profile", "profile.json", "--session", "sync-net-1", "--retry-backoff", "-1s"}, want: "sync network discover-run: --retry-backoff cannot be negative"},
		{name: "discover unexpected args", args: []string{"sync", "network", "discover-run", "--profile", "profile.json", "--session", "sync-net-1", "extra\narg"}, want: `sync network discover-run: unexpected arguments: "extra\narg"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d stderr = %q, want 2", tt.args, got, stderr.String())
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

func TestSyncNetworkLoopUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing profile", args: []string{"sync", "network", "loop", "--session-prefix", "sync-net-loop"}, want: "sync network loop: --profile is required"},
		{name: "missing session prefix", args: []string{"sync", "network", "loop", "--profile", "profile.json"}, want: "sync network loop: --session-prefix is required"},
		{name: "padded session prefix", args: []string{"sync", "network", "loop", "--profile", "profile.json", "--session-prefix", " sync-net-loop"}, want: "sync network loop: --session-prefix must not be padded"},
		{name: "unsafe session prefix", args: []string{"sync", "network", "loop", "--profile", "profile.json", "--session-prefix", "../x"}, want: "unsafe session id"},
		{name: "unsupported format", args: []string{"sync", "network", "loop", "--profile", "profile.json", "--session-prefix", "sync-net-loop", "--format", "yaml"}, want: `sync network loop: unsupported format "yaml"`},
		{name: "zero interval", args: []string{"sync", "network", "loop", "--profile", "profile.json", "--session-prefix", "sync-net-loop", "--interval", "0"}, want: "sync network loop: --interval must be greater than zero"},
		{name: "negative max runs", args: []string{"sync", "network", "loop", "--profile", "profile.json", "--session-prefix", "sync-net-loop", "--max-runs", "-1"}, want: "sync network loop: --max-runs cannot be negative"},
		{name: "negative backoff", args: []string{"sync", "network", "loop", "--profile", "profile.json", "--session-prefix", "sync-net-loop", "--retry-backoff", "-1s"}, want: "sync network loop: --retry-backoff cannot be negative"},
		{name: "unexpected args", args: []string{"sync", "network", "loop", "--profile", "profile.json", "--session-prefix", "sync-net-loop", "extra\narg"}, want: `sync network loop: unexpected arguments: "extra\narg"`},
		{name: "watch flag rejected", args: []string{"sync", "network", "loop", "--profile", "profile.json", "--session-prefix", "sync-net-loop", "--watch"}, want: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d stderr = %q, want 2", tt.args, got, stderr.String())
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

func TestSyncRunPublishesQueuedChangesAndMarksDone(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, ".hidden", "secret.txt"), "secret")
	mustWrite(t, filepath.Join(source, "visible.txt"), "public")
	writeDefaultProfile(t, profilePath, source, target)
	runner := Runner{Now: time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := runner.Run([]string{"sync", "run", "--profile", profilePath, "--session", "sync-run-1", "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync run exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var result syncRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(sync run stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if result.Mode != "bounded_queue_consumer" || result.Run.Status != incrementalsync.RunStatusPublished {
		t.Fatalf("sync run result = %+v, want bounded published run", result)
	}
	if result.Run.Summary.Done != 3 || result.Run.Summary.Ready != 0 || result.Run.Summary.InFlight != 0 || result.Run.Summary.Total != 3 {
		t.Fatalf("sync run summary = %+v, want three done entries", result.Run.Summary)
	}
	if !syncRunContainsPath(result.Run.Published, ".hidden/secret.txt") || !syncRunContainsPath(result.Run.Published, "visible.txt") {
		t.Fatalf("published entries = %+v, want hidden and visible files", result.Run.Published)
	}
	if gotContent := readFileString(t, filepath.Join(target, ".hidden", "secret.txt")); gotContent != "secret" {
		t.Fatalf("target hidden file = %q, want secret", gotContent)
	}
	if gotContent := readFileString(t, filepath.Join(target, "visible.txt")); gotContent != "public" {
		t.Fatalf("target visible file = %q, want public", gotContent)
	}
	if _, err := os.Stat(result.Run.RunPath); err != nil {
		t.Fatalf("os.Stat(run receipt %q) error = %v, want nil", result.Run.RunPath, err)
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"sync", "queue", "status", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue status after run exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "sync queue status after run", stdout.String(), "done=3", "ready=0", "in_flight=0", "mode=queue_only")
	if strings.Contains(stdout.String(), "watch") || strings.Contains(stdout.String(), "daemon") {
		t.Fatalf("sync queue status after run stdout = %q, must not imply watcher or daemon", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = runner.Run([]string{"status", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("status after sync run exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "status after sync run", stdout.String(),
		"incremental_sync_status=no_pending_review",
		"incremental_sync_action=no_incremental_sync_review_action_required",
		"incremental_sync_done=3",
		"incremental_sync_runs=1",
		"incremental_sync_retrying_runs=0",
	)
}

func TestSyncLoopPublishesChangedHiddenFileAcrossForegroundPasses(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustMkdir(t, filepath.Join(source, ".hidden"))
	mustWrite(t, filepath.Join(source, "visible.txt"), "first")
	writeDefaultProfile(t, profilePath, source, target)
	runner := Runner{
		Now: time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC),
	}
	mutated := false
	runner.syncLoopAfterRun = func(result syncRunResult) {
		if result.Run.SessionID == "sync-loop-000001" {
			mustWrite(t, filepath.Join(source, ".hidden", "secret.txt"), "second")
			mutated = true
		}
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := runner.Run([]string{"sync", "loop", "--profile", profilePath, "--session-prefix", "sync-loop", "--max-runs", "2", "--interval", "1ms", "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync loop exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var result syncLoopResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(sync loop stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if result.Mode != "foreground_polling_queue_consumer" || result.Status != "completed" || result.CompletedRuns != 2 || result.PublishedRuns != 2 || result.RetryingRuns != 0 {
		t.Fatalf("sync loop result = %+v, want two clean foreground polling passes", result)
	}
	if len(result.Runs) != 2 || result.Runs[0].Run.SessionID != "sync-loop-000001" || result.Runs[1].Run.SessionID != "sync-loop-000002" {
		t.Fatalf("sync loop runs = %+v, want generated session ids", result.Runs)
	}
	if !mutated {
		t.Fatalf("sync loop test did not mutate source between passes")
	}
	if !syncRunContainsPath(result.Runs[0].Run.Published, "visible.txt") {
		t.Fatalf("first sync loop pass published = %+v, want visible file", result.Runs[0].Run.Published)
	}
	if !syncRunContainsPath(result.Runs[1].Run.Published, ".hidden/secret.txt") {
		t.Fatalf("second sync loop pass published = %+v, want changed hidden file", result.Runs[1].Run.Published)
	}
	if gotContent := readFileString(t, filepath.Join(target, ".hidden", "secret.txt")); gotContent != "second" {
		t.Fatalf("target hidden file = %q, want second", gotContent)
	}
	for _, run := range result.Runs {
		if _, err := os.Stat(run.Run.RunPath); err != nil {
			t.Fatalf("os.Stat(run receipt %q) error = %v, want nil", run.Run.RunPath, err)
		}
		if run.Mode != "foreground_loop_pass" {
			t.Fatalf("sync loop run mode = %q, want foreground_loop_pass", run.Mode)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync loop stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"status", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("status after sync loop exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "status after sync loop",
		stdout.String(),
		"incremental_sync_status=no_pending_review",
		"incremental_sync_done=3",
		"incremental_sync_runs=2",
		"incremental_sync_retrying_runs=0",
	)
}

func TestSyncWatchPublishesChangedHiddenFileFromOSEvent(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustMkdir(t, filepath.Join(source, ".hidden"))
	mustWrite(t, filepath.Join(source, "visible.txt"), "first")
	writeDefaultProfile(t, profilePath, source, target)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ready := make(chan struct{}, 1)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{
		Context: ctx,
		Now:     time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC),
		syncWatchReady: func() {
			ready <- struct{}{}
		},
	}

	go func() {
		done <- runner.Run([]string{"sync", "watch", "--profile", profilePath, "--session-prefix", "sync-watch", "--max-events", "1", "--settle", "25ms", "--format", "json"}, &stdout, &stderr)
	}()

	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatalf("sync watch did not report ready; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	mustWrite(t, filepath.Join(source, ".hidden", "secret.txt"), "second")
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("sync watch exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("sync watch did not exit after one event batch; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	var result syncWatchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(sync watch stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if result.Mode != "foreground_os_watcher_queue_consumer" || result.Status != "completed" || result.EventBatches != 1 || result.EventsSeen == 0 {
		t.Fatalf("sync watch result = %+v, want one completed event batch", result)
	}
	if result.Baseline == nil || result.Baseline.Run.SessionID != "sync-watch-000001" || result.Baseline.Mode != "foreground_watch_baseline" {
		t.Fatalf("sync watch baseline = %+v, want baseline run sync-watch-000001", result.Baseline)
	}
	if len(result.Runs) != 1 || result.Runs[0].Run.SessionID != "sync-watch-000002" || result.Runs[0].Mode != "foreground_watch_event_pass" {
		t.Fatalf("sync watch event runs = %+v, want one generated event pass", result.Runs)
	}
	if !syncRunContainsPath(result.Baseline.Run.Published, "visible.txt") {
		t.Fatalf("sync watch baseline published = %+v, want visible file", result.Baseline.Run.Published)
	}
	if !syncRunContainsPath(result.Runs[0].Run.Published, ".hidden/secret.txt") {
		t.Fatalf("sync watch event pass published = %+v, want changed hidden file", result.Runs[0].Run.Published)
	}
	if gotContent := readFileString(t, filepath.Join(target, ".hidden", "secret.txt")); gotContent != "second" {
		t.Fatalf("target hidden file = %q, want second", gotContent)
	}
	if result.WatchedDirs < 1 || len(result.WatchedRoots) != 1 {
		t.Fatalf("sync watch watched dirs=%d roots=%#v, want at least source root", result.WatchedDirs, result.WatchedRoots)
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync watch stderr = %q, want empty", stderr.String())
	}
}

func TestSyncNetworkRunPublishesQueuedHiddenFileViaNetwork(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustMkdir(t, filepath.Join(source, ".hidden"))
	mustWrite(t, filepath.Join(source, ".hidden", "secret.txt"), "network secret")
	mustWrite(t, filepath.Join(source, "visible.txt"), "network public")
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	sourceProfile := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	sourceProfile.Target.TargetID = peer.TargetID
	sourceProfile.Target.DevicePublicKey = peer.TargetDeviceID
	sourceProfile.Target.PairingReceiptID = "pairing-1"
	sourceProfile.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	receiverListener := listenOnReservedTCPAddress(t)
	t.Cleanup(func() {
		_ = receiverListener.Close()
	})
	receiverAddress := receiverListener.Addr().String()
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, receiverAddress)
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, receiverAddress)
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	writePairingReceiptForCLI(t, target, sourceProfile, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = sourceProfile.Target.PairedAt
	})
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	pairingReady := make(chan pairserve.ReadyInfo, 1)
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	done := make(chan int, 1)
	var serveStdout bytes.Buffer
	var serveStderr bytes.Buffer
	serveRunner := Runner{
		Context:                 serveCtx,
		Now:                     cliTLSNow(),
		receiverListenerForTest: receiverListener,
		ServePairingReady: func(info pairserve.ReadyInfo) {
			pairingReady <- info
		},
		ServeReceiverReady: func(info receiverserve.ReadyInfo) {
			receiverReady <- info
		},
	}
	go func() {
		done <- serveRunner.Run([]string{"serve", "--profile", targetProfilePath, "--listen", "127.0.0.1:0"}, &serveStdout, &serveStderr)
	}()
	waitServePairingReady(t, pairingReady, &serveStderr)
	receiverInfo := waitServeReceiverReady(t, receiverReady, &serveStderr)
	if receiverInfo.Address != receiverAddress || receiverInfo.Peer != peer {
		t.Fatalf("serve receiver info = %+v, want address %q peer %+v", receiverInfo, receiverAddress, peer)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{"sync", "network", "run", "--profile", sourceProfilePath, "--session", "sync-net-1", "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync network run exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync network run stderr = %q, want empty", stderr.String())
	}
	var result syncNetworkRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(sync network run stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if result.Mode != "bounded_network_queue_consumer" || result.Run.Status != incrementalsync.RunStatusPublished {
		t.Fatalf("sync network run result = %+v, want bounded network published run", result)
	}
	if result.Network.Transfer != "published" || result.Network.EncryptedTransfer != "tls13_mtls" || result.Network.Status != "published" || result.Network.Stage != "commit" || result.Network.ResumeOutcome != "fresh" {
		t.Fatalf("sync network result = %+v, want published profile-backed mTLS transfer", result.Network)
	}
	if result.Run.Summary.Done != 3 || result.Run.Summary.Backoff != 0 || result.Run.Summary.Ready != 0 || result.Run.Summary.Total != 3 {
		t.Fatalf("sync network run summary = %+v, want three done entries", result.Run.Summary)
	}
	if !syncRunContainsPath(result.Run.Published, ".hidden/secret.txt") || !syncRunContainsPath(result.Run.Published, "visible.txt") {
		t.Fatalf("published entries = %+v, want hidden and visible files", result.Run.Published)
	}
	if gotContent := readFileString(t, filepath.Join(target, ".hidden", "secret.txt")); gotContent != "network secret" {
		t.Fatalf("target hidden file = %q, want network secret", gotContent)
	}
	if gotContent := readFileString(t, filepath.Join(target, "visible.txt")); gotContent != "network public" {
		t.Fatalf("target visible file = %q, want network public", gotContent)
	}
	if _, err := os.Stat(result.Run.RunPath); err != nil {
		t.Fatalf("os.Stat(run receipt %q) error = %v, want nil", result.Run.RunPath, err)
	}
	transfer := readNetworkTransferForCLI(t, target, "sync-net-1")
	if transfer.Status != control.NetworkTransferPublished || transfer.Stage != "commit" || transfer.PrivacyOverhead == nil {
		t.Fatalf("network transfer = %+v, want published commit with privacy overhead", transfer)
	}
	mustWrite(t, filepath.Join(source, ".hidden", "secret.txt"), "network secret v2")
	stdout.Reset()
	stderr.Reset()
	got = Runner{Now: cliTLSNow().Add(time.Minute)}.Run([]string{"sync", "network", "run", "--profile", sourceProfilePath, "--session", "sync-net-2", "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("second sync network run exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("second sync network run stderr = %q, want empty", stderr.String())
	}
	var second syncNetworkRunResult
	if err := json.Unmarshal(stdout.Bytes(), &second); err != nil {
		t.Fatalf("json.Unmarshal(second sync network run stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if second.Run.Status != incrementalsync.RunStatusPublished || second.Network.Transfer != "published" {
		t.Fatalf("second sync network run result = %+v, want published per-entry network run", second)
	}
	if len(second.Run.Published) != 1 || !syncRunContainsPath(second.Run.Published, ".hidden/secret.txt") {
		t.Fatalf("second published entries = %+v, want only changed hidden file", second.Run.Published)
	}
	if gotContent := readFileString(t, filepath.Join(target, ".hidden", "secret.txt")); gotContent != "network secret v2" {
		t.Fatalf("target hidden file after second run = %q, want network secret v2", gotContent)
	}
	manifestPath, err := control.Path(target, control.ArtifactManifest, "sync-net-2")
	if err != nil {
		t.Fatalf("control.Path(second manifest) error = %v, want nil", err)
	}
	manifest, err := control.ReadFile[control.Manifest](manifestPath)
	if err != nil {
		t.Fatalf("control.ReadFile(second manifest) error = %v, want nil", err)
	}
	if len(manifest.Entries) != 1 || manifest.Entries[0].Path != ".hidden/secret.txt" {
		t.Fatalf("second network manifest entries = %+v, want only hidden file ready entry", manifest.Entries)
	}
	if manifest.Entries[0].PreviousSessionID != "sync-net-1" || manifest.Entries[0].PreviousManifestID != "sync-net-1" || manifest.Entries[0].PreviousDigest == "" || !manifest.Entries[0].HasPreviousSizeEvidence() || !manifest.Entries[0].HasPreviousModeEvidence() {
		t.Fatalf("second network manifest previous evidence = %+v, want previous published file evidence", manifest.Entries[0])
	}
	if gotContent := readFileString(t, filepath.Join(target, "visible.txt")); gotContent != "network public" {
		t.Fatalf("target visible file after second run = %q, want unchanged network public", gotContent)
	}
	stdout.Reset()
	stderr.Reset()
	got = Runner{Now: cliTLSNow()}.Run([]string{"sync", "queue", "status", "--profile", sourceProfilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue status after network run exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "sync queue status after network run", stdout.String(), "done=3", "ready=0", "backoff=0", "mode=queue_only")
	cancelServe()
	select {
	case serveExit := <-done:
		if serveExit != 0 {
			t.Fatalf("serve exit after sync network runs = %d stderr = %q, want 0", serveExit, serveStderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("serve did not exit after cancel; stderr=%q", serveStderr.String())
	}
	if serveStdout.Len() != 0 {
		t.Fatalf("serve stdout = %q, want empty", serveStdout.String())
	}
	assertTextContainsAll(t, "serve sync network smoke", serveStderr.String(), "receiver_routes=true", "push_network=true")
}

func TestSyncNetworkDiscoverRunPublishesAfterMatchingLANCandidate(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustMkdir(t, filepath.Join(source, ".hidden"))
	mustWrite(t, filepath.Join(source, ".hidden", "secret.txt"), "discovery network secret")
	mustWrite(t, filepath.Join(source, "visible.txt"), "discovery network public")
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	sourceProfile := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	sourceProfile.Target.TargetID = peer.TargetID
	sourceProfile.Target.DevicePublicKey = peer.TargetDeviceID
	sourceProfile.Target.PairingReceiptID = "pairing-1"
	sourceProfile.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	receiverListener := listenOnReservedTCPAddress(t)
	t.Cleanup(func() {
		_ = receiverListener.Close()
	})
	receiverAddress := receiverListener.Addr().String()
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, receiverAddress)
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, receiverAddress)
	targetProfile.Discovery = &profile.DiscoveryConfig{AdvertiseReceiverHint: true}
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	writePairingReceiptForCLI(t, target, sourceProfile, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = sourceProfile.Target.PairedAt
	})
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	pairingReady := make(chan pairserve.ReadyInfo, 1)
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	serveDone := make(chan int, 1)
	var serveStdout bytes.Buffer
	var serveStderr bytes.Buffer
	serveRunner := Runner{
		Context:                 serveCtx,
		Now:                     cliTLSNow(),
		receiverListenerForTest: receiverListener,
		ServePairingReady: func(info pairserve.ReadyInfo) {
			pairingReady <- info
		},
		ServeReceiverReady: func(info receiverserve.ReadyInfo) {
			receiverReady <- info
		},
	}
	go func() {
		serveDone <- serveRunner.Run([]string{"serve", "--profile", targetProfilePath, "--listen", "127.0.0.1:0"}, &serveStdout, &serveStderr)
	}()
	waitServePairingReady(t, pairingReady, &serveStderr)
	receiverInfo := waitServeReceiverReady(t, receiverReady, &serveStderr)
	if receiverInfo.Address != receiverAddress || receiverInfo.Peer != peer {
		t.Fatalf("serve receiver info = %+v, want address %q peer %+v", receiverInfo, receiverAddress, peer)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	advertiseDone := make(chan int, 1)
	var advertiseStdout bytes.Buffer
	var advertiseStderr bytes.Buffer
	discoverRunner := Runner{
		Now: cliTLSNow(),
		DiscoverBrowseReady: func(address string) {
			go func() {
				advertiseDone <- Runner{Now: cliTLSNow()}.Run([]string{"discover", "advertise", "--profile", targetProfilePath, "--dest", address, "--duration", "150ms", "--interval", "10ms"}, &advertiseStdout, &advertiseStderr)
			}()
		},
	}

	got := discoverRunner.Run([]string{"sync", "network", "discover-run", "--profile", sourceProfilePath, "--session", "sync-net-discover-1", "--listen", "127.0.0.1:0", "--timeout", "250ms", "--format", "json"}, &stdout, &stderr)

	cancelServe()
	select {
	case serveExit := <-serveDone:
		if serveExit != 0 {
			t.Fatalf("serve exit after sync network discover-run = %d stderr = %q, want 0", serveExit, serveStderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("serve did not exit after cancel; stderr=%q", serveStderr.String())
	}
	if advertiseExit := <-advertiseDone; advertiseExit != 0 {
		t.Fatalf("discover advertise exit = %d stderr = %q stdout = %q, want 0", advertiseExit, advertiseStderr.String(), advertiseStdout.String())
	}
	if got != 0 {
		t.Fatalf("sync network discover-run exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync network discover-run stderr = %q, want empty", stderr.String())
	}
	var result syncNetworkDiscoverRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(sync network discover-run stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if result.Discovery.Status != "matched" || result.Discovery.ProfileAddress != receiverAddress || result.Discovery.MatchedAddress != receiverAddress || result.Discovery.Trusted {
		t.Fatalf("discovery gate = %+v, want matched untrusted candidate at receiver address", result.Discovery)
	}
	if result.Mode != "lan_discovery_gated_network_queue_consumer" || result.Run.Status != incrementalsync.RunStatusPublished {
		t.Fatalf("sync network discover-run result = %+v, want LAN-gated published run", result)
	}
	if result.Network.Transfer != "published" || result.Network.EncryptedTransfer != "tls13_mtls" || result.Network.Status != "published" || result.Network.Stage != "commit" {
		t.Fatalf("sync network discover-run network = %+v, want profile-pinned mTLS transfer", result.Network)
	}
	if !syncRunContainsPath(result.Run.Published, ".hidden/secret.txt") || !syncRunContainsPath(result.Run.Published, "visible.txt") {
		t.Fatalf("published entries = %+v, want hidden and visible files", result.Run.Published)
	}
	if gotContent := readFileString(t, filepath.Join(target, ".hidden", "secret.txt")); gotContent != "discovery network secret" {
		t.Fatalf("target hidden file = %q, want discovery network secret", gotContent)
	}
	if gotContent := readFileString(t, filepath.Join(target, "visible.txt")); gotContent != "discovery network public" {
		t.Fatalf("target visible file = %q, want discovery network public", gotContent)
	}
	raw := stdout.String() + advertiseStdout.String()
	for _, forbidden := range []string{"trusted=true", "hostname", "file_count"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("discovery-gated output = %q, must not contain %q", raw, forbidden)
		}
	}
}

func TestSyncNetworkDiscoverRunNoMatchDoesNotMutateQueue(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "source.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	p := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	p.Target.TargetID = peer.TargetID
	p.Target.DevicePublicKey = peer.TargetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	p.Network = networkConfigForCLI(t, sourceCert, net.JoinHostPort("127.0.0.1", "1"))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = p.Target.PairedAt
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{"sync", "network", "discover-run", "--profile", profilePath, "--session", "sync-net-no-match", "--listen", "127.0.0.1:0", "--timeout", "20ms", "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("sync network discover-run no-match exit = %d stderr = %q stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync network discover-run no-match stderr = %q, want empty", stderr.String())
	}
	var result syncNetworkDiscoverRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(sync network discover-run no-match stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if result.Discovery.Status != "no_matching_candidate" || result.Discovery.Trusted || result.Run.Status != "" {
		t.Fatalf("sync network discover-run no-match result = %+v, want discovery gate refusal without run", result)
	}
	if _, err := os.Stat(filepath.Join(control.ControlDir(target), "incremental-sync")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("incremental state dir stat error = %v, want no queue/run mutation", err)
	}
	if _, err := os.Stat(filepath.Join(control.ControlDir(target), "sessions", "sync-net-no-match")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("session dir stat error = %v, want no network transfer mutation", err)
	}
}

func TestSyncNetworkRunIdleDoesNotContactReceiverOrClaimFailure(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "source.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	p := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	p.Target.TargetID = peer.TargetID
	p.Target.DevicePublicKey = peer.TargetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	p.Network = networkConfigForCLI(t, sourceCert, net.JoinHostPort("127.0.0.1", "1"))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = p.Target.PairedAt
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{"sync", "network", "run", "--profile", profilePath, "--session", "sync-net-idle", "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("sync network idle exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync network idle stderr = %q, want empty", stderr.String())
	}
	var result syncNetworkRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(sync network idle stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if result.Run.Status != incrementalsync.RunStatusIdle || result.Run.Summary.Total != 0 || result.Run.Summary.Ready != 0 {
		t.Fatalf("sync network idle run = %+v, want empty idle queue", result.Run)
	}
	if result.Network.Transfer != "not_attempted" || result.Network.Status != "idle" || result.Network.Stage != "queue_idle" || result.Network.EncryptedTransfer != "profile_backed_mtls_validated" {
		t.Fatalf("sync network idle transfer = %+v, want not-attempted validated network plan", result.Network)
	}
	if _, err := os.Stat(filepath.Join(target, ".supermover", "sessions", "sync-net-idle", "network-transfer.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("network-transfer artifact error = %v, want not exist for idle queue", err)
	}
}

func TestSyncNetworkRunRejectsMissingNetworkMaterialBeforeQueueMutation(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "source.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	p := profile.NewDefault("profile-local", "Source profile", source, target)
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.DevicePublicKey = "sha256:target"
	p.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{"sync", "network", "run", "--profile", profilePath, "--session", "sync-net-invalid"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("sync network invalid exit = %d stderr = %q stdout = %q, want 2", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "network.receiver_url is required") {
		t.Fatalf("sync network invalid stderr = %q, want missing network material", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("sync network invalid stdout = %q, want empty", stdout.String())
	}
	if _, err := os.Stat(control.ControlDir(target)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("control dir stat error = %v, want no target control-plane mutation", err)
	}
}

func TestSyncNetworkLoopRunsBoundedForegroundPasses(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustMkdir(t, filepath.Join(source, ".hidden"))
	mustWrite(t, filepath.Join(source, ".hidden", "secret.txt"), "network loop secret")
	mustWrite(t, filepath.Join(source, "visible.txt"), "network loop public")
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	sourceProfile := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	sourceProfile.Target.TargetID = peer.TargetID
	sourceProfile.Target.DevicePublicKey = peer.TargetDeviceID
	sourceProfile.Target.PairingReceiptID = "pairing-1"
	sourceProfile.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	receiverListener := listenOnReservedTCPAddress(t)
	t.Cleanup(func() {
		_ = receiverListener.Close()
	})
	receiverAddress := receiverListener.Addr().String()
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, receiverAddress)
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, receiverAddress)
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	writePairingReceiptForCLI(t, target, sourceProfile, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = sourceProfile.Target.PairedAt
	})
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	pairingReady := make(chan pairserve.ReadyInfo, 1)
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	done := make(chan int, 1)
	var serveStdout bytes.Buffer
	var serveStderr bytes.Buffer
	serveRunner := Runner{
		Context:                 serveCtx,
		Now:                     cliTLSNow(),
		receiverListenerForTest: receiverListener,
		ServePairingReady: func(info pairserve.ReadyInfo) {
			pairingReady <- info
		},
		ServeReceiverReady: func(info receiverserve.ReadyInfo) {
			receiverReady <- info
		},
	}
	go func() {
		done <- serveRunner.Run([]string{"serve", "--profile", targetProfilePath, "--listen", "127.0.0.1:0"}, &serveStdout, &serveStderr)
	}()
	waitServePairingReady(t, pairingReady, &serveStderr)
	receiverInfo := waitServeReceiverReady(t, receiverReady, &serveStderr)
	if receiverInfo.Address != receiverAddress || receiverInfo.Peer != peer {
		t.Fatalf("serve receiver info = %+v, want address %q peer %+v", receiverInfo, receiverAddress, peer)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{"sync", "network", "loop", "--profile", sourceProfilePath, "--session-prefix", "sync-net-loop", "--max-runs", "2", "--interval", "1ms", "--format", "json"}, &stdout, &stderr)

	cancelServe()
	select {
	case serveExit := <-done:
		if serveExit != 0 {
			t.Fatalf("serve exit after sync network loop = %d stderr = %q, want 0", serveExit, serveStderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("serve did not exit after cancel; stderr=%q", serveStderr.String())
	}
	if got != 0 {
		t.Fatalf("sync network loop exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync network loop stderr = %q, want empty", stderr.String())
	}
	var result syncNetworkLoopResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(sync network loop stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if result.Mode != "foreground_network_queue_consumer" || result.Status != "completed" || result.CompletedRuns != 2 || result.PublishedRuns != 1 || result.IdleRuns != 1 || result.RetryingRuns != 0 {
		t.Fatalf("sync network loop result = %+v, want one published pass and one idle pass", result)
	}
	if result.NetworkAttempts != 1 || result.NetworkPublishedRuns != 1 || result.NetworkNotAttemptedRuns != 1 {
		t.Fatalf("sync network loop network counters = attempts=%d published=%d not_attempted=%d, want 1/1/1", result.NetworkAttempts, result.NetworkPublishedRuns, result.NetworkNotAttemptedRuns)
	}
	if len(result.Runs) != 2 || result.Runs[0].Run.SessionID != "sync-net-loop-000001" || result.Runs[1].Run.SessionID != "sync-net-loop-000002" {
		t.Fatalf("sync network loop runs = %+v, want generated session ids", result.Runs)
	}
	if result.Runs[0].Mode != "foreground_network_loop_pass" || result.Runs[0].Network.Transfer != "published" || result.Runs[0].Network.EncryptedTransfer != "tls13_mtls" {
		t.Fatalf("first sync network loop run = %+v, want published mTLS pass", result.Runs[0])
	}
	if result.Runs[1].Mode != "foreground_network_loop_pass" || result.Runs[1].Run.Status != incrementalsync.RunStatusIdle || result.Runs[1].Network.Transfer != "not_attempted" || result.Runs[1].Network.Stage != "queue_idle" {
		t.Fatalf("second sync network loop run = %+v, want idle not-attempted network pass", result.Runs[1])
	}
	if gotContent := readFileString(t, filepath.Join(target, ".hidden", "secret.txt")); gotContent != "network loop secret" {
		t.Fatalf("target hidden file = %q, want network loop secret", gotContent)
	}
	if gotContent := readFileString(t, filepath.Join(target, "visible.txt")); gotContent != "network loop public" {
		t.Fatalf("target visible file = %q, want network loop public", gotContent)
	}
	transfer := readNetworkTransferForCLI(t, target, "sync-net-loop-000001")
	if transfer.Status != control.NetworkTransferPublished || transfer.Stage != "commit" || transfer.PrivacyOverhead == nil {
		t.Fatalf("network transfer = %+v, want published first loop pass", transfer)
	}
	if _, err := os.Stat(filepath.Join(target, ".supermover", "sessions", "sync-net-loop-000002", "network-transfer.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("second network-transfer artifact error = %v, want not exist for idle queue pass", err)
	}
	stdout.Reset()
	stderr.Reset()
	got = Runner{Now: cliTLSNow()}.Run([]string{"sync", "queue", "status", "--profile", sourceProfilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("sync queue status after network loop exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "sync queue status after network loop", stdout.String(), "done=3", "ready=0", "backoff=0", "mode=queue_only")
}

func TestSyncRunRetriesQueueWhenLocalPushRefusesTargetDrift(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "source")
	mustWrite(t, filepath.Join(target, "file.txt"), "foreign target")
	writeDefaultProfile(t, profilePath, source, target)
	runner := Runner{Now: time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := runner.Run([]string{"sync", "run", "--profile", profilePath, "--session", "sync-run-drift", "--retry-backoff", "10m", "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("sync run drift exit = %d stderr = %q stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("sync run drift stderr = %q, want empty because retry result is stdout evidence", stderr.String())
	}
	var result syncRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(sync run drift stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if result.Run.Status != incrementalsync.RunStatusRetrying || len(result.Run.Retried) != 1 || result.Run.Summary.Backoff != 1 || result.Run.Summary.Ready != 0 {
		t.Fatalf("sync run drift result = %+v, want one backoff retry", result)
	}
	retried := result.Run.Retried[0]
	if retried.Path != "file.txt" || retried.Status != incrementalsync.StatusBackoff || retried.Attempts != 1 || retried.NextDueAt != "2026-05-21T01:12:03Z" {
		t.Fatalf("retried entry = %+v, want file.txt in 10m backoff", retried)
	}
	if gotContent := readFileString(t, filepath.Join(target, "file.txt")); gotContent != "foreign target" {
		t.Fatalf("target file after refused sync run = %q, want unchanged foreign target", gotContent)
	}
	if _, err := os.Stat(result.Run.RunPath); err != nil {
		t.Fatalf("os.Stat(run receipt %q) error = %v, want nil", result.Run.RunPath, err)
	}

	stdout.Reset()
	stderr.Reset()
	got = runner.Run([]string{"report", "--profile", profilePath}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("report after refused sync run exit = %d stderr = %q stdout = %q, want 1 for review-required retry evidence", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "report after refused sync run", stdout.String(),
		"incremental_sync_backoff=1",
		"incremental_sync_runs=1",
		"incremental_sync_retrying_runs=1",
		"incremental_sync_backoff",
		"incremental_sync_retrying_runs",
		"incremental_sync status=review_required action=inspect_incremental_sync_queue",
		"incremental_sync_run session=sync-run-drift status=retrying",
	)
}

func TestReconcileHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"reconcile", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("reconcile --help exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "reconcile help", stdout.String(),
		"Usage of reconcile",
		"supermover reconcile plan --profile <path>",
		"supermover reconcile review --profile <path>",
		"supermover reconcile apply --profile <path> --id <persisted-drift-id>",
		"supermover reconcile apply --profile <path> --all-persisted-planned",
		"supermover reconcile apply --profile <path> --record-live",
		"narrow persisted drift repair",
		"Review is read-only",
		"record-required inputs",
		"selects only currently planned persisted actions",
		"missing regular-file restores",
		"already-restored resolves",
		"does not run broad automatic reconcile",
		"Apply writes durable selected-ID receipts",
		"conflict_class",
		"retry_advice",
	)
	for _, forbidden := range []string{"--target", "--state-dir", "broad repair ready", "or persist apply receipts", "runs automatic retry policy"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("reconcile --help stdout = %q, must not expose %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("reconcile --help stderr = %q, want empty", stderr.String())
	}
}

func TestReconcileApplyHelpIsHonest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"reconcile", "apply", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("reconcile apply --help exit = %d stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "reconcile apply help", stdout.String(),
		"Usage of reconcile apply",
		"supermover reconcile apply --profile <path> --id <persisted-drift-id>",
		"supermover reconcile apply --profile <path> --all-persisted-planned",
		"supermover reconcile apply --profile <path> --record-live",
		"--record-live is an explicit gate",
		"first persists current live",
		"durable drift records",
		"Planned background boundaries remain non-apply-capable",
		"not a broad automatic repair or background retry runner",
	)
	for _, forbidden := range []string{"--target", "--state-dir", "background repair ready", "runs automatic retry policy"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("reconcile apply --help stdout = %q, must not expose %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("reconcile apply --help stderr = %q, want empty", stderr.String())
	}
}

func TestReconcileApplyRequiresExplicitSelectionIntentAndReason(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing profile", args: []string{"reconcile", "apply", "--id", "drift", "--apply", "--reason", "restore"}, want: "reconcile apply: --profile is required"},
		{name: "missing selection", args: []string{"reconcile", "apply", "--profile", "profile.json", "--apply", "--reason", "restore"}, want: "reconcile apply: at least one --id, --all-persisted-planned, or --record-live is required"},
		{name: "missing apply", args: []string{"reconcile", "apply", "--profile", "profile.json", "--id", "drift", "--reason", "restore"}, want: "reconcile apply: --apply is required"},
		{name: "missing reason", args: []string{"reconcile", "apply", "--profile", "profile.json", "--id", "drift", "--apply"}, want: "reconcile apply: --reason is required"},
		{name: "unsafe id", args: []string{"reconcile", "apply", "--profile", "profile.json", "--id", "../drift", "--apply", "--reason", "restore"}, want: "reconcile apply: --id is invalid"},
		{name: "unsupported format", args: []string{"reconcile", "apply", "--profile", "profile.json", "--id", "drift", "--apply", "--reason", "restore", "--format", "yaml"}, want: `reconcile apply: unsupported format "yaml"`},
		{name: "unexpected args", args: []string{"reconcile", "apply", "--profile", "profile.json", "--id", "drift", "--apply", "--reason", "restore", "extra\narg"}, want: `reconcile apply: unexpected arguments: "extra\narg"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d stderr = %q, want 2", tt.args, got, stderr.String())
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

func TestReconcileReviewReportsBoundariesWithoutMutation(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "aaaaaaa")
	writeDefaultProfile(t, profilePath, source, target)
	drift := cliMissingFileDrift("drift-reconcile-review")
	writePublishedSessionForReconcileCLI(t, target, drift.SessionID)
	writeTargetDriftForReconcileCLI(t, target, drift)
	before := mustSnapshotTree(t, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"reconcile", "review", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("reconcile review exit = %d stderr = %q stdout = %q, want review-required 1", got, stderr.String(), stdout.String())
	}
	var review reconcile.Review
	if err := json.Unmarshal(stdout.Bytes(), &review); err != nil {
		t.Fatalf("json.Unmarshal(reconcile review stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if review.Schema != reconcile.SchemaReview || review.Summary.PersistedPlanned != 1 || review.Summary.LiveTargetDrifts != 0 {
		t.Fatalf("reconcile review = %+v, want persisted plan with covered live detector evidence excluded from live-only inputs", review)
	}
	assertReviewBoundaryForCLI(t, review, reconcile.BoundaryPersistentDrift, reconcile.BoundaryStatusWiredReadOnly, true)
	assertReviewBoundaryForCLI(t, review, reconcile.BoundaryLiveOnlyDrift, reconcile.BoundaryStatusWiredReadOnly, false)
	assertReviewBoundaryForCLI(t, review, reconcile.BoundaryBackgroundScan, reconcile.BoundaryStatusPlanned, false)
	assertReviewBoundaryForCLI(t, review, reconcile.BoundaryManifestRewrite, reconcile.BoundaryStatusPlanned, false)
	assertReviewBoundaryForCLI(t, review, reconcile.BoundaryDaemonSync, reconcile.BoundaryStatusPlanned, false)
	assertReviewBoundaryForCLI(t, review, reconcile.BoundaryDriftToPrune, reconcile.BoundaryStatusPlanned, false)
	assertReviewBoundaryForCLI(t, review, reconcile.BoundaryAutomaticRetry, reconcile.BoundaryStatusPlanned, false)
	after := mustSnapshotTree(t, target)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("reconcile review changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
	if stderr.Len() != 0 {
		t.Fatalf("reconcile review stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"reconcile", "review", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("reconcile review text exit = %d stderr = %q stdout = %q, want review-required 1", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "reconcile review text", stdout.String(),
		"reconcile_review",
		"review_required=true",
		"persisted_planned=1",
		"live_target_drifts=0",
		"reconcile_live_input",
		"action=no_live_repair_inputs",
		"reconcile_boundary name=persisted_drift_reconcile status=wired_read_only",
		"reconcile_boundary name=live_only_repair_inputs status=wired_read_only",
		"reconcile_boundary name=background_scans status=planned",
		"reconcile_boundary name=manifest_rewrite status=planned",
		"reconcile_boundary name=daemon_ongoing_sync_integration status=planned",
		"reconcile_boundary name=drift_to_prune_handoff status=planned",
		"reconcile_boundary name=automatic_retry_policy status=planned",
	)
	if strings.Contains(stdout.String(), "status=applied") || strings.Contains(stdout.String(), "receipt_path=") {
		t.Fatalf("reconcile review stdout = %q, must not claim apply or receipt writes", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("reconcile review text stderr = %q, want empty", stderr.String())
	}
}

func TestReconcileReviewReportsLiveOnlyDriftAsRecordRequired(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "aaaaaaa")
	writeDefaultProfile(t, profilePath, source, target)
	writePublishedSessionForReconcileCLI(t, target, "session-one")
	before := mustSnapshotTree(t, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"reconcile", "review", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("reconcile review live-only exit = %d stderr = %q stdout = %q, want review-required 1", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "reconcile review live-only", stdout.String(),
		"reconcile_review",
		"persisted_planned=0",
		"live_target_drifts=1",
		"action=run_drift_record_before_reconcile_apply",
		"reconcile_live_drift",
		"reconcile_boundary name=live_only_repair_inputs status=record_required",
	)
	after := mustSnapshotTree(t, target)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("reconcile review live-only changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
	if stderr.Len() != 0 {
		t.Fatalf("reconcile review live-only stderr = %q, want empty", stderr.String())
	}
}

func TestReconcileApplyAllPersistedPlannedAppliesOnlyDurablePlannedActions(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "aaaaaaa")
	mustWrite(t, filepath.Join(source, "other.txt"), "bbbbbbb")
	writeDefaultProfile(t, profilePath, source, target)
	drift := cliMissingFileDrift("drift-reconcile-all")
	writePublishedSessionForReconcileCLI(t, target, drift.SessionID)
	writeTargetDriftForReconcileCLI(t, target, drift)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: time.Date(2026, 5, 21, 2, 3, 4, 0, time.UTC)}.Run([]string{"reconcile", "apply", "--profile", profilePath, "--all-persisted-planned", "--apply", "--reason", "restore all persisted planned", "--reviewer", "ops", "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("reconcile apply --all-persisted-planned exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var receipt reconcile.Receipt
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
		t.Fatalf("json.Unmarshal(reconcile apply all stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if receipt.Schema != reconcile.SchemaApplyReceipt || !receipt.ApplyIntent || receipt.Summary.Applied != 1 || receipt.Actions[0].DriftID != drift.ID {
		t.Fatalf("reconcile apply --all-persisted-planned receipt = %+v, want one applied persisted action", receipt)
	}
	if gotContent := readFileString(t, filepath.Join(target, "file.txt")); gotContent != "aaaaaaa" {
		t.Fatalf("restored target content = %q, want source payload", gotContent)
	}
	if _, err := os.Lstat(filepath.Join(target, "other.txt")); !os.IsNotExist(err) {
		t.Fatalf("live-only source file target state err = %v, want untouched missing file", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("reconcile apply --all-persisted-planned stderr = %q, want empty", stderr.String())
	}
}

func TestReconcileApplyAllPersistedPlannedRequiresDurablePlannedActions(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "aaaaaaa")
	writeDefaultProfile(t, profilePath, source, target)
	writePublishedSessionForReconcileCLI(t, target, "session-one")
	before := mustSnapshotTree(t, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"reconcile", "apply", "--profile", profilePath, "--all-persisted-planned", "--apply", "--reason", "restore all persisted planned"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("reconcile apply --all-persisted-planned exit = %d stderr = %q stdout = %q, want 1 for live-only inputs", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "reconcile apply all no persisted planned", stderr.String(),
		"reconcile apply: --all-persisted-planned found no persisted planned reconcile actions",
		"run drift record before reconcile apply",
	)
	after := mustSnapshotTree(t, target)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("reconcile apply all live-only changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
	if stdout.Len() != 0 {
		t.Fatalf("reconcile apply all live-only stdout = %q, want empty", stdout.String())
	}
}

func TestReconcileApplyAllPersistedPlannedRejectsMixedExplicitIDs(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"reconcile", "apply", "--profile", profilePath, "--id", "drift-one", "--all-persisted-planned", "--apply", "--reason", "restore all persisted planned"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("reconcile apply mixed ids exit = %d stderr = %q stdout = %q, want usage 2", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "reconcile apply mixed ids", stderr.String(), "reconcile apply: only one of --id, --all-persisted-planned, or --record-live is allowed")
	if stdout.Len() != 0 {
		t.Fatalf("reconcile apply mixed ids stdout = %q, want empty", stdout.String())
	}
}

func TestReconcileApplyRecordLivePersistsThenAppliesLiveOnlyMissingFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "aaaaaaa")
	writeDefaultProfile(t, profilePath, source, target)
	writePublishedSessionForReconcileCLI(t, target, "session-one")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: time.Date(2026, 5, 21, 2, 3, 4, 0, time.UTC)}.Run([]string{"reconcile", "apply", "--profile", profilePath, "--record-live", "--apply", "--reason", "record and restore live missing file", "--reviewer", "ops", "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("reconcile apply --record-live exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var receipt reconcile.Receipt
	if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
		t.Fatalf("json.Unmarshal(reconcile apply record-live stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if receipt.Schema != reconcile.SchemaApplyReceipt || !receipt.ApplyIntent || receipt.Summary.Applied != 1 || receipt.Summary.Refused != 0 || len(receipt.Actions) != 1 {
		t.Fatalf("reconcile apply --record-live receipt = %+v, want one applied persisted action", receipt)
	}
	if receipt.Actions[0].Path != "file.txt" || receipt.Actions[0].Action != reconcile.ActionRestoreFile {
		t.Fatalf("reconcile apply --record-live action = %+v, want file restore", receipt.Actions[0])
	}
	if gotContent := readFileString(t, filepath.Join(target, "file.txt")); gotContent != "aaaaaaa" {
		t.Fatalf("restored target content = %q, want source payload", gotContent)
	}
	persisted := readTargetDriftForReconcileCLI(t, target, receipt.Actions[0].DriftID)
	if persisted.ReviewState != "resolved" || persisted.ReviewAction != "resolve" || persisted.ReviewedBy != "ops" || persisted.ReviewReason != "record and restore live missing file" {
		t.Fatalf("persisted drift after record-live apply = %+v, want resolved review metadata", persisted)
	}
	if stderr.Len() != 0 {
		t.Fatalf("reconcile apply --record-live stderr = %q, want empty", stderr.String())
	}
}

func TestReconcileApplyRecordLiveRequiresLiveDrift(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "aaaaaaa")
	mustWriteWithModTime(t, filepath.Join(target, "file.txt"), "aaaaaaa", 0o644, "2026-05-18T00:00:00Z")
	writeDefaultProfile(t, profilePath, source, target)
	writePublishedSessionForReconcileCLI(t, target, "session-one")
	before := mustSnapshotTree(t, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"reconcile", "apply", "--profile", profilePath, "--record-live", "--apply", "--reason", "record and restore live drift"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("reconcile apply --record-live clean exit = %d stderr = %q stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "reconcile apply record-live clean", stderr.String(), "reconcile apply: --record-live found no live target drift records to persist before reconcile apply")
	after := mustSnapshotTree(t, target)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("reconcile apply record-live clean changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
	if stdout.Len() != 0 {
		t.Fatalf("reconcile apply record-live clean stdout = %q, want empty", stdout.String())
	}
}

func TestReconcileApplyRecordLiveRejectsMixedSelection(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"reconcile", "apply", "--profile", profilePath, "--record-live", "--all-persisted-planned", "--apply", "--reason", "record and restore live drift"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("reconcile apply record-live mixed exit = %d stderr = %q stdout = %q, want usage 2", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "reconcile apply record-live mixed", stderr.String(), "reconcile apply: only one of --id, --all-persisted-planned, or --record-live is allowed")
	if stdout.Len() != 0 {
		t.Fatalf("reconcile apply record-live mixed stdout = %q, want empty", stdout.String())
	}
}

func TestReconcilePlanAndApplyMissingFileRestore(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "aaaaaaa")
	writeDefaultProfile(t, profilePath, source, target)
	drift := cliMissingFileDrift("drift-reconcile-restore")
	writePublishedSessionForReconcileCLI(t, target, drift.SessionID)
	writeTargetDriftForReconcileCLI(t, target, drift)
	beforePlan := mustSnapshotTree(t, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"reconcile", "plan", "--profile", profilePath, "--id", drift.ID, "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("reconcile plan exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var plan reconcile.Receipt
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("json.Unmarshal(reconcile plan stdout) error = %v stdout = %q, want nil", err, stdout.String())
	}
	if plan.Schema != reconcile.SchemaPlanReceipt || plan.ApplyIntent || plan.Summary.Planned != 1 || plan.Summary.Refused != 0 || len(plan.Actions) != 1 {
		t.Fatalf("reconcile plan receipt = %+v, want one planned dry-run action", plan)
	}
	if plan.Actions[0].Action != reconcile.ActionRestoreFile || plan.Actions[0].Result != reconcile.ResultPlanned || plan.Actions[0].DriftID != drift.ID {
		t.Fatalf("reconcile plan action = %+v, want restore plan for selected drift", plan.Actions[0])
	}
	if _, err := os.Lstat(filepath.Join(target, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("target file after reconcile plan err = %v, want missing", err)
	}
	afterPlan := mustSnapshotTree(t, target)
	if strings.Join(afterPlan, "\n") != strings.Join(beforePlan, "\n") {
		t.Fatalf("reconcile plan changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(beforePlan, "\n"), strings.Join(afterPlan, "\n"))
	}
	if stderr.Len() != 0 {
		t.Fatalf("reconcile plan stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"reconcile", "plan", "--profile", profilePath, "--id", drift.ID}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("reconcile text plan exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "reconcile text plan", stdout.String(),
		"reconcile_plan",
		"planned=1",
		"refused=0",
	)
	if strings.Contains(stdout.String(), "conflict_class=") || strings.Contains(stdout.String(), "retry_advice=") {
		t.Fatalf("reconcile text plan stdout = %q, must not emit empty refusal taxonomy fields on planned actions", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()

	runner := Runner{Now: time.Date(2026, 5, 21, 2, 3, 4, 0, time.UTC)}
	got = runner.Run([]string{"reconcile", "apply", "--profile", profilePath, "--id", drift.ID, "--apply", "--reason", "restore from source evidence", "--reviewer", "ops"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("reconcile apply exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "reconcile apply", stdout.String(),
		"reconcile_apply",
		"apply_intent=true",
		"receipt=reconcile-",
		"status=applied",
		"receipt_path=",
		"applied=1",
		"refused=0",
		"action=restore_file",
		"result=applied",
		"drift=drift-reconcile-restore",
	)
	if gotContent := readFileString(t, filepath.Join(target, "file.txt")); gotContent != "aaaaaaa" {
		t.Fatalf("restored target content = %q, want source payload", gotContent)
	}
	persisted := readTargetDriftForReconcileCLI(t, target, drift.ID)
	if persisted.ReviewState != "resolved" || persisted.ReviewAction != "resolve" || persisted.ReviewedBy != "ops" || persisted.ReviewReason != "restore from source evidence" {
		t.Fatalf("persisted drift = %+v, want resolved review metadata", persisted)
	}
	receiptDir := filepath.Join(target, control.DirName, "reconcile", "receipts")
	entries, err := os.ReadDir(receiptDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v, want reconcile receipt", receiptDir, err)
	}
	if len(entries) != 1 {
		t.Fatalf("reconcile receipt count = %d, want one", len(entries))
	}
	artifact, err := control.ReadFile[reconcile.Receipt](filepath.Join(receiptDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("control.ReadFile(reconcile receipt) error = %v, want nil", err)
	}
	if artifact.Status != reconcile.ReceiptStatusApplied || artifact.Summary.Applied != 1 || artifact.Actions[0].DriftID != drift.ID {
		t.Fatalf("durable reconcile receipt = %+v, want applied drift receipt", artifact)
	}
	if stderr.Len() != 0 {
		t.Fatalf("reconcile apply stderr = %q, want empty", stderr.String())
	}
}

func TestReconcileTextRefusalIncludesConflictClassAndRetryAdvice(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "aaaaaaa")
	writeDefaultProfile(t, profilePath, source, target)
	drift := cliMissingFileDrift("drift-reconcile-conflict")
	writePublishedSessionForReconcileCLI(t, target, drift.SessionID)
	writeTargetDriftForReconcileCLI(t, target, drift)
	mustWrite(t, filepath.Join(target, "file.txt"), "operator")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"reconcile", "plan", "--profile", profilePath, "--id", drift.ID}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("reconcile conflict plan exit = %d stderr = %q stdout = %q, want review-required 1", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "reconcile conflict plan", stdout.String(),
		"reconcile_plan",
		"refused=1",
		"reconcile_refusal",
		"reason=ambiguous_state",
		"conflict_class=target_state_conflict",
		"retry_advice=review_target_state_before_retry",
	)
	if stderr.Len() != 0 {
		t.Fatalf("reconcile conflict plan stderr = %q, want empty", stderr.String())
	}
}

func TestDaemonStopDoesNotMutateForeignState(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	foreign := agentdaemon.NewState("profile-other", "target-other", "/profiles/other.json", agentdaemon.StatusRunning, 99, now)
	foreign.PairingAddress = "127.0.0.1:9000"
	if err := agentdaemon.WriteState(target, foreign); err != nil {
		t.Fatalf("WriteState(foreign) error = %v, want nil", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Runner{Now: now.Add(time.Minute)}.Run([]string{"daemon", "stop", "--profile", profilePath, "--reason", "operator review"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("daemon stop exit = %d stderr = %q, want 0", got, stderr.String())
	}
	after, err := agentdaemon.ReadState(target)
	if err != nil {
		t.Fatalf("ReadState(after stop) error = %v, want nil", err)
	}
	if after.Status != agentdaemon.StatusRunning || after.StopIntent != nil || after.UpdatedAt != foreign.UpdatedAt {
		t.Fatalf("foreign daemon state after stop = %+v, want unmodified foreign state %+v", after, foreign)
	}

	stdout.Reset()
	stderr.Reset()
	got = Runner{Now: now}.Run([]string{"daemon", "status", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("daemon status exit = %d stderr = %q, want 0", got, stderr.String())
	}
	statusLine := firstLineWithPrefix(stdout.String(), "daemon_status ")
	assertTextContainsAll(t, "daemon status foreign", statusLine, "state=scope_mismatch", "scope_issues=state_scope_mismatch", "state_profile=-", "pid=0")
	assertTextContainsNone(t, "daemon status foreign", statusLine, "state_profile=/profiles/other.json", "pid=99 ")
}

func TestDaemonRunScopedStartupCleanupOnlyRemovesOlderScopedStopIntent(t *testing.T) {
	target := t.TempDir()
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	foreign := agentdaemon.NewStopIntent("profile-other", "target-other", "foreign stop", 99, now)
	if err := agentdaemon.WriteStopIntent(target, foreign); err != nil {
		t.Fatalf("WriteStopIntent(foreign) error = %v, want nil", err)
	}

	if err := removeScopedDaemonStopIntentBefore(target, "profile-local", "target-local", now.Add(time.Second)); err != nil {
		t.Fatalf("removeScopedDaemonStopIntentBefore(foreign) error = %v, want nil", err)
	}
	got, err := agentdaemon.ReadStopIntent(target)
	if err != nil {
		t.Fatalf("ReadStopIntent(after foreign cleanup) error = %v, want nil", err)
	}
	if got.ProfileID != foreign.ProfileID || got.TargetID != foreign.TargetID || got.Reason != foreign.Reason {
		t.Fatalf("foreign stop intent after cleanup = %+v, want preserved %+v", got, foreign)
	}

	if err := agentdaemon.WriteStopIntent(target, agentdaemon.NewStopIntent("profile-local", "target-local", "fresh scoped stop", 100, now.Add(time.Second))); err != nil {
		t.Fatalf("WriteStopIntent(fresh scoped) error = %v, want nil", err)
	}
	if err := removeScopedDaemonStopIntentBefore(target, "profile-local", "target-local", now); err != nil {
		t.Fatalf("removeScopedDaemonStopIntentBefore(fresh scoped) error = %v, want nil", err)
	}
	got, err = agentdaemon.ReadStopIntent(target)
	if err != nil {
		t.Fatalf("ReadStopIntent(after fresh scoped cleanup) error = %v, want nil", err)
	}
	if got.Reason != "fresh scoped stop" {
		t.Fatalf("fresh scoped stop intent after cleanup = %+v, want preserved", got)
	}

	if err := agentdaemon.WriteStopIntent(target, agentdaemon.NewStopIntent("profile-local", "target-local", "old scoped stop", 100, now.Add(-time.Second))); err != nil {
		t.Fatalf("WriteStopIntent(old scoped) error = %v, want nil", err)
	}
	if err := removeScopedDaemonStopIntentBefore(target, "profile-local", "target-local", now); err != nil {
		t.Fatalf("removeScopedDaemonStopIntentBefore(old scoped) error = %v, want nil", err)
	}
	if _, err := agentdaemon.ReadStopIntent(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadStopIntent(after old scoped cleanup) error = %v, want not exist", err)
	}
}

func TestDaemonRunForegroundStopsFromPersistedIntent(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemonReady := make(chan agentdaemon.State, 1)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{
		Context: ctx,
		Now:     time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC),
		DaemonReady: func(state agentdaemon.State) {
			daemonReady <- state
		},
	}

	go func() {
		done <- runner.Run([]string{"daemon", "run", "--foreground", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()

	var state agentdaemon.State
	select {
	case state = <-daemonReady:
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not report ready; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if state.Status != agentdaemon.StatusRunning || state.Mode != "pairing-only" || state.PairingAddress == "" {
		t.Fatalf("daemon ready state = %+v, want running pairing-only with address", state)
	}
	var stopStdout bytes.Buffer
	var stopStderr bytes.Buffer
	stopRunner := Runner{Now: runner.Now}
	if got := stopRunner.Run([]string{"daemon", "stop", "--profile", profilePath, "--reason", "test stop"}, &stopStdout, &stopStderr); got != 0 {
		t.Fatalf("daemon stop exit = %d stderr = %q, want 0", got, stopStderr.String())
	}
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("daemon run exit = %d stderr = %q, want 0", got, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not exit after stop intent; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "daemon run", stdout.String(), "daemon: running", "state=running", "mode=pairing-only", "service_manager=none")
	gotState, err := agentdaemon.ReadState(target)
	if err != nil {
		t.Fatalf("agentdaemon.ReadState() error = %v, want nil", err)
	}
	if gotState.Status != agentdaemon.StatusStopped || gotState.StopIntent == nil || gotState.StopIntent.Reason != "test stop" {
		t.Fatalf("daemon final state = %+v, want stopped with stop intent", gotState)
	}
	events, err := agentdaemon.ListLifecycleEvents(target)
	if err != nil {
		t.Fatalf("ListLifecycleEvents() error = %v, want nil", err)
	}
	if !lifecycleEventTypesContain(events, "daemon_running", "daemon_stop_requested", "daemon_stopped") {
		t.Fatalf("daemon lifecycle event types = %#v, want running/stop/stopped", lifecycleEventTypes(events))
	}
}

func TestDaemonRunForegroundConsumesRestartIntentAndKeepsRunning(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemonReady := make(chan agentdaemon.State, 2)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	runner := Runner{
		Context: ctx,
		Now:     now,
		DaemonReady: func(state agentdaemon.State) {
			daemonReady <- state
		},
	}

	go func() {
		done <- runner.Run([]string{"daemon", "run", "--foreground", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()

	first := waitDaemonReadyState(t, daemonReady, &stdout, &stderr)
	var restartStdout bytes.Buffer
	var restartStderr bytes.Buffer
	if got := (Runner{Now: now.Add(time.Second)}).Run([]string{"daemon", "restart", "--profile", profilePath, "--reason", "test restart"}, &restartStdout, &restartStderr); got != 0 {
		t.Fatalf("daemon restart exit = %d stderr = %q, want 0", got, restartStderr.String())
	}
	assertTextContainsAll(t, "daemon restart", restartStdout.String(), "daemon: restart_requested", "foreground_signal=restart-intent", "service_manager=none", "consumption=pending")
	if restartStderr.Len() != 0 {
		t.Fatalf("daemon restart stderr = %q, want empty", restartStderr.String())
	}
	second := waitDaemonReadyState(t, daemonReady, &stdout, &stderr)
	if second.PairingAddress == "" || second.ProfileID != first.ProfileID || second.TargetID != first.TargetID {
		t.Fatalf("second daemon ready state = %+v, want same scope with new running address", second)
	}
	if _, err := agentdaemon.ReadRestartIntent(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadRestartIntent(after consumed) error = %v, want not exist", err)
	}
	select {
	case got := <-done:
		t.Fatalf("daemon run exited after restart with %d; stdout=%q stderr=%q", got, stdout.String(), stderr.String())
	case <-time.After(250 * time.Millisecond):
	}

	var stopStdout bytes.Buffer
	var stopStderr bytes.Buffer
	if got := (Runner{Now: now.Add(2 * time.Second)}).Run([]string{"daemon", "stop", "--profile", profilePath, "--reason", "test stop"}, &stopStdout, &stopStderr); got != 0 {
		t.Fatalf("daemon stop exit = %d stderr = %q, want 0", got, stopStderr.String())
	}
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("daemon run exit = %d stderr = %q, want 0", got, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not exit after stop intent; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	gotState, err := agentdaemon.ReadState(target)
	if err != nil {
		t.Fatalf("agentdaemon.ReadState() error = %v, want nil", err)
	}
	if gotState.Status != agentdaemon.StatusStopped || gotState.StopIntent == nil || gotState.StopIntent.Reason != "test stop" {
		t.Fatalf("daemon final state = %+v, want stopped with stop intent", gotState)
	}
	events, err := agentdaemon.ListLifecycleEvents(target)
	if err != nil {
		t.Fatalf("ListLifecycleEvents() error = %v, want nil", err)
	}
	if !lifecycleEventTypesContain(events, "daemon_restart_requested", "daemon_restart_consumed", "daemon_running", "daemon_stopped") {
		t.Fatalf("daemon lifecycle event types = %#v, want restart request/consume/running/stopped", lifecycleEventTypes(events))
	}
}

func TestDaemonRunForegroundSignalsReadyBeforeRunningStdoutAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemonReady := make(chan agentdaemon.State, 2)
	done := make(chan int, 1)
	stdout := newBlockingDaemonRunningWriter(2)
	defer stdout.Unblock()
	var stderr bytes.Buffer
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	runner := Runner{
		Context: ctx,
		Now:     now,
		DaemonReady: func(state agentdaemon.State) {
			daemonReady <- state
		},
	}

	go func() {
		done <- runner.Run([]string{"daemon", "run", "--foreground", "--profile", profilePath, "--listen", "127.0.0.1:0"}, stdout, &stderr)
	}()

	first := waitDaemonReadyState(t, daemonReady, stdout, &stderr)
	var restartStdout bytes.Buffer
	var restartStderr bytes.Buffer
	if got := (Runner{Now: now.Add(time.Second)}).Run([]string{"daemon", "restart", "--profile", profilePath, "--reason", "test restart"}, &restartStdout, &restartStderr); got != 0 {
		t.Fatalf("daemon restart exit = %d stderr = %q, want 0", got, restartStderr.String())
	}
	select {
	case <-stdout.blocked:
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not reach blocked second running stdout; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	select {
	case second := <-daemonReady:
		if second.PairingAddress == "" || second.ProfileID != first.ProfileID || second.TargetID != first.TargetID {
			t.Fatalf("second daemon ready state = %+v, want same scope with new running address", second)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("daemon ready hook waited behind running stdout; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Unblock()
	var stopStdout bytes.Buffer
	var stopStderr bytes.Buffer
	if got := (Runner{Now: now.Add(2 * time.Second)}).Run([]string{"daemon", "stop", "--profile", profilePath, "--reason", "test stop"}, &stopStdout, &stopStderr); got != 0 {
		t.Fatalf("daemon stop exit = %d stderr = %q, want 0", got, stopStderr.String())
	}
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("daemon run exit = %d stderr = %q, want 0", got, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not exit after stop intent; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestDaemonRunForegroundRunsProfileBackedLocalPollingSyncAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, ".hidden", "first.txt"), "first")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Sync = &profile.SyncConfig{LocalPolling: &profile.LocalPollingSyncConfig{
		Enabled:            true,
		IntervalMillis:     60000,
		RetryBackoffMillis: 10,
		SessionPrefix:      "daemon-sync",
	}}
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemonReady := make(chan agentdaemon.State, 2)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	runner := Runner{
		Context: ctx,
		Now:     now,
		DaemonReady: func(state agentdaemon.State) {
			daemonReady <- state
		},
	}

	go func() {
		done <- runner.Run([]string{"daemon", "run", "--foreground", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()

	first := waitDaemonReadyState(t, daemonReady, &stdout, &stderr)
	waitForFile(t, filepath.Join(target, ".hidden", "first.txt"), 2*time.Second, &stdout, &stderr)
	scheduler, err := incrementalsync.Open(incrementalsync.Options{
		StateDir: filepath.Join(control.ControlDir(target), "incremental-sync"),
		Now:      func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("incrementalsync.Open() error = %v, want nil", err)
	}
	waitForPublishedRunSessions(t, scheduler, incrementalsync.Scope{ProfileID: p.ProfileID, TargetID: p.Target.TargetID}, 2*time.Second, "daemon-sync-000001")

	mustWrite(t, filepath.Join(source, "after-restart.txt"), "second")
	var restartStdout bytes.Buffer
	var restartStderr bytes.Buffer
	if got := (Runner{Now: now.Add(time.Second)}).Run([]string{"daemon", "restart", "--profile", profilePath, "--reason", "sync restart"}, &restartStdout, &restartStderr); got != 0 {
		t.Fatalf("daemon restart exit = %d stderr = %q, want 0", got, restartStderr.String())
	}
	second := waitDaemonReadyState(t, daemonReady, &stdout, &stderr)
	if second.PairingAddress == "" || second.ProfileID != first.ProfileID || second.TargetID != first.TargetID {
		t.Fatalf("second daemon ready state = %+v, want same scope with new running address", second)
	}
	waitForFile(t, filepath.Join(target, "after-restart.txt"), 2*time.Second, &stdout, &stderr)
	waitForPublishedRunSessions(t, scheduler, incrementalsync.Scope{ProfileID: p.ProfileID, TargetID: p.Target.TargetID}, 2*time.Second, "daemon-sync-000001", "daemon-sync-000002")

	var stopStdout bytes.Buffer
	var stopStderr bytes.Buffer
	if got := (Runner{Now: now.Add(2 * time.Second)}).Run([]string{"daemon", "stop", "--profile", profilePath, "--reason", "test stop"}, &stopStdout, &stopStderr); got != 0 {
		t.Fatalf("daemon stop exit = %d stderr = %q, want 0", got, stopStderr.String())
	}
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("daemon run exit = %d stderr = %q, want 0", got, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not exit after stop intent; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "daemon run with local polling sync", stdout.String(), "daemon: sync_local_polling", "session=daemon-sync-000001")
	events, err := agentdaemon.ListLifecycleEvents(target)
	if err != nil {
		t.Fatalf("ListLifecycleEvents() error = %v, want nil", err)
	}
	if !lifecycleEventTypesContain(events, "daemon_sync_started", "daemon_sync_pass", "daemon_sync_stopped", "daemon_restart_consumed") {
		t.Fatalf("daemon lifecycle event types = %#v, want sync start/pass/stop and restart consume", lifecycleEventTypes(events))
	}
}

func TestDaemonRunForegroundRecordsProfileBackedLiveDriftWithoutRepair(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writePublishedSessionForReconcileCLI(t, target, "session-one")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Repair = &profile.RepairConfig{DriftRecording: &profile.DriftRecordingRepairConfig{
		Enabled:        true,
		IntervalMillis: 60000,
	}}
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemonReady := make(chan agentdaemon.State, 1)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	runner := Runner{
		Context: ctx,
		Now:     now,
		DaemonReady: func(state agentdaemon.State) {
			daemonReady <- state
		},
	}

	go func() {
		done <- runner.Run([]string{"daemon", "run", "--foreground", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()

	state := waitDaemonReadyState(t, daemonReady, &stdout, &stderr)
	if state.Status != agentdaemon.StatusRunning || state.Mode != "pairing-only" {
		t.Fatalf("daemon ready state = %+v, want running pairing-only", state)
	}
	drift := waitForTargetDriftByPath(t, target, "file.txt", "missing", 2*time.Second, &stdout, &stderr)
	if drift.ReviewState != "needs_review" || drift.ProfileID != p.ProfileID || drift.TargetID != p.Target.TargetID {
		t.Fatalf("recorded drift = %+v, want needs_review in profile scope", drift)
	}
	if _, err := os.Stat(filepath.Join(target, "file.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target file stat error = %v, want still missing because drift recording does not repair", err)
	}
	var stopStdout bytes.Buffer
	var stopStderr bytes.Buffer
	if got := (Runner{Now: now.Add(time.Second)}).Run([]string{"daemon", "stop", "--profile", profilePath, "--reason", "drift recording test stop"}, &stopStdout, &stopStderr); got != 0 {
		t.Fatalf("daemon stop exit = %d stderr = %q, want 0", got, stopStderr.String())
	}
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("daemon run exit = %d stderr = %q stdout=%q, want 0", got, stderr.String(), stdout.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not exit after stop intent; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "daemon drift recording", stdout.String(), "daemon: drift_recording", "detected=1", "recorded=1", "repair=not_applied", "reconcile=not_applied")
	assertTextContainsNone(t, "daemon drift recording stderr", stderr.String(), "drift recording:")
	events, err := agentdaemon.ListLifecycleEvents(target)
	if err != nil {
		t.Fatalf("ListLifecycleEvents() error = %v, want nil", err)
	}
	if !lifecycleEventTypesContain(events, "daemon_drift_recording_started", "daemon_drift_recording_pass", "daemon_drift_recording_stopped", "daemon_stopped") {
		t.Fatalf("daemon lifecycle event types = %#v, want drift recording start/pass/stop and daemon stop", lifecycleEventTypes(events))
	}
}

func TestDaemonRunForegroundAppliesProfileBackedPersistedReconcileWithoutLiveRecord(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWriteWithModTime(t, filepath.Join(source, "file.txt"), "aaaaaaa", 0o644, "2026-05-18T00:00:00Z")
	drift := cliMissingFileDrift("drift-daemon-reconcile")
	writePublishedSessionForReconcileCLI(t, target, drift.SessionID)
	writeTargetDriftForReconcileCLI(t, target, drift)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Repair = &profile.RepairConfig{PersistedReconcileApply: &profile.PersistedReconcileApplyRepairConfig{
		Enabled:        true,
		IntervalMillis: 60000,
		Reason:         "restore persisted drift from daemon profile policy",
		Reviewer:       "daemon-repair",
	}}
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemonReady := make(chan agentdaemon.State, 1)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	runner := Runner{
		Context: ctx,
		Now:     now,
		DaemonReady: func(state agentdaemon.State) {
			daemonReady <- state
		},
	}

	go func() {
		done <- runner.Run([]string{"daemon", "run", "--foreground", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()
	finished := false
	defer func() {
		if finished {
			return
		}
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("daemon run did not exit after test cleanup cancellation; stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}()

	_ = waitDaemonReadyState(t, daemonReady, &stdout, &stderr)
	receipt := waitForLatestReconcileReceipt(t, target, reconcile.ReceiptStatusApplied, 2*time.Second, &stdout, &stderr)
	waitForFile(t, filepath.Join(target, "file.txt"), 2*time.Second, &stdout, &stderr)
	persisted := readTargetDriftForReconcileCLI(t, target, drift.ID)
	if persisted.ReviewState != "resolved" || persisted.ReviewAction != "resolve" || persisted.ReviewedBy != "daemon-repair" || persisted.ReviewReason != "restore persisted drift from daemon profile policy" {
		t.Fatalf("daemon persisted reconcile drift = %+v, want resolved by profile-backed daemon apply", persisted)
	}
	if receipt.Summary.Applied != 1 || receipt.Summary.Refused != 0 || receipt.Actions[0].DriftID != drift.ID {
		t.Fatalf("daemon reconcile receipt = %+v, want one applied action for %q", receipt, drift.ID)
	}
	var stopStdout bytes.Buffer
	var stopStderr bytes.Buffer
	if got := (Runner{Now: now.Add(time.Second)}).Run([]string{"daemon", "stop", "--profile", profilePath, "--reason", "daemon reconcile test stop"}, &stopStdout, &stopStderr); got != 0 {
		t.Fatalf("daemon stop exit = %d stderr = %q, want 0", got, stopStderr.String())
	}
	select {
	case got := <-done:
		finished = true
		if got != 0 {
			t.Fatalf("daemon run exit = %d stderr = %q stdout=%q, want 0", got, stderr.String(), stdout.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not exit after stop intent; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "daemon reconcile apply", stdout.String(), "daemon: reconcile_apply", "selection=persisted_planned", "status=applied", "applied=1", "live_record=not_called", "manifest_rewrite=not_applied", "prune=not_applied")
	assertTextContainsNone(t, "daemon reconcile apply stderr", stderr.String(), "reconcile apply:")
	events, err := agentdaemon.ListLifecycleEvents(target)
	if err != nil {
		t.Fatalf("ListLifecycleEvents() error = %v, want nil", err)
	}
	if !lifecycleEventTypesContain(events, "daemon_reconcile_apply_started", "daemon_reconcile_apply_pass", "daemon_reconcile_apply_stopped", "daemon_stopped") {
		t.Fatalf("daemon lifecycle event types = %#v, want reconcile apply start/pass/stop and daemon stop", lifecycleEventTypes(events))
	}
}

func TestDaemonRunForegroundPersistedReconcileApplyDoesNotConsumeLiveOnlyDrift(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writePublishedSessionForReconcileCLI(t, target, "session-one")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Repair = &profile.RepairConfig{PersistedReconcileApply: &profile.PersistedReconcileApplyRepairConfig{
		Enabled:        true,
		IntervalMillis: 60000,
		Reason:         "restore persisted drift from daemon profile policy",
	}}
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemonReady := make(chan agentdaemon.State, 1)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	runner := Runner{
		Context: ctx,
		Now:     now,
		DaemonReady: func(state agentdaemon.State) {
			daemonReady <- state
		},
	}

	go func() {
		done <- runner.Run([]string{"daemon", "run", "--foreground", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()
	finished := false
	defer func() {
		if finished {
			return
		}
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("daemon run did not exit after test cleanup cancellation; stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}()

	_ = waitDaemonReadyState(t, daemonReady, &stdout, &stderr)
	waitForDaemonLifecycleEventType(t, target, "daemon_reconcile_apply_pass", 2*time.Second)
	if _, err := os.Stat(filepath.Join(target, control.DirName, "drift")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("drift directory stat error = %v, want no live-only drift recording", err)
	}
	if _, err := os.Stat(filepath.Join(target, "file.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target file stat error = %v, want live-only drift left unmodified", err)
	}
	var stopStdout bytes.Buffer
	var stopStderr bytes.Buffer
	if got := (Runner{Now: now.Add(time.Second)}).Run([]string{"daemon", "stop", "--profile", profilePath, "--reason", "daemon reconcile noop stop"}, &stopStdout, &stopStderr); got != 0 {
		t.Fatalf("daemon stop exit = %d stderr = %q, want 0", got, stopStderr.String())
	}
	select {
	case got := <-done:
		finished = true
		if got != 0 {
			t.Fatalf("daemon run exit = %d stderr = %q stdout=%q, want 0", got, stderr.String(), stdout.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not exit after stop intent; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "daemon reconcile no planned", stdout.String(), "daemon: reconcile_apply", "selection=persisted_planned", "planned=0", "status=noop")
	assertTextContainsNone(t, "daemon reconcile no planned stderr", stderr.String(), "reconcile apply:")
}

func TestDaemonRunForegroundRunsProfileBackedNetworkPollingSync(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustMkdir(t, filepath.Join(source, ".hidden"))
	mustWrite(t, filepath.Join(source, ".hidden", "daemon-network.txt"), "daemon network secret")
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	sourceProfile := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	sourceProfile.Target.TargetID = peer.TargetID
	sourceProfile.Target.DevicePublicKey = peer.TargetDeviceID
	sourceProfile.Target.PairingReceiptID = "pairing-1"
	sourceProfile.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	receiverListener := listenOnReservedTCPAddress(t)
	t.Cleanup(func() {
		_ = receiverListener.Close()
	})
	receiverAddress := receiverListener.Addr().String()
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, receiverAddress)
	sourceProfile.Sync = &profile.SyncConfig{NetworkPolling: &profile.NetworkPollingSyncConfig{
		Enabled:            true,
		IntervalMillis:     60000,
		RetryBackoffMillis: 10,
		SessionPrefix:      "daemon-network-sync",
	}}
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, receiverAddress)
	targetProfile.Sync = nil
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	writePairingReceiptForCLI(t, target, sourceProfile, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = sourceProfile.Target.PairedAt
	})
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	pairingReady := make(chan pairserve.ReadyInfo, 1)
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	serveDone := make(chan int, 1)
	var serveStdout bytes.Buffer
	var serveStderr bytes.Buffer
	serveRunner := Runner{
		Context:                 serveCtx,
		Now:                     cliTLSNow(),
		receiverListenerForTest: receiverListener,
		ServePairingReady: func(info pairserve.ReadyInfo) {
			pairingReady <- info
		},
		ServeReceiverReady: func(info receiverserve.ReadyInfo) {
			receiverReady <- info
		},
	}
	go func() {
		serveDone <- serveRunner.Run([]string{"serve", "--profile", targetProfilePath, "--listen", "127.0.0.1:0"}, &serveStdout, &serveStderr)
	}()
	waitServePairingReady(t, pairingReady, &serveStderr)
	receiverInfo := waitServeReceiverReady(t, receiverReady, &serveStderr)
	if receiverInfo.Address != receiverAddress || receiverInfo.Peer != peer {
		t.Fatalf("serve receiver info = %+v, want address %q peer %+v", receiverInfo, receiverAddress, peer)
	}
	ctx, cancelDaemon := context.WithCancel(context.Background())
	defer cancelDaemon()
	daemonReady := make(chan agentdaemon.State, 1)
	daemonDone := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	daemonRunner := Runner{
		Context: ctx,
		Now:     cliTLSNow(),
		DaemonReady: func(state agentdaemon.State) {
			daemonReady <- state
		},
	}
	go func() {
		daemonDone <- daemonRunner.Run([]string{"daemon", "run", "--foreground", "--profile", sourceProfilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()

	state := waitDaemonReadyState(t, daemonReady, &stdout, &stderr)
	if state.Status != agentdaemon.StatusRunning || state.Mode != "network-polling" || state.PairingAddress != "" || state.ReceiverAddress != "" {
		t.Fatalf("daemon network polling ready state = %+v, want network-polling without serve listeners", state)
	}
	waitForFile(t, filepath.Join(target, ".hidden", "daemon-network.txt"), 2*time.Second, &stdout, &stderr)
	scheduler, err := incrementalsync.Open(incrementalsync.Options{
		StateDir: filepath.Join(control.ControlDir(target), "incremental-sync"),
		Now:      func() time.Time { return cliTLSNow() },
	})
	if err != nil {
		t.Fatalf("incrementalsync.Open() error = %v, want nil", err)
	}
	waitForPublishedRunSessions(t, scheduler, incrementalsync.Scope{ProfileID: sourceProfile.ProfileID, TargetID: sourceProfile.Target.TargetID}, 2*time.Second, "daemon-network-sync-000001")
	var stopStdout bytes.Buffer
	var stopStderr bytes.Buffer
	if got := (Runner{Now: cliTLSNow().Add(time.Second)}).Run([]string{"daemon", "stop", "--profile", sourceProfilePath, "--reason", "test stop"}, &stopStdout, &stopStderr); got != 0 {
		t.Fatalf("daemon stop exit = %d stderr = %q, want 0", got, stopStderr.String())
	}
	select {
	case got := <-daemonDone:
		if got != 0 {
			t.Fatalf("daemon run exit = %d stderr = %q stdout=%q, want 0", got, stderr.String(), stdout.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not exit after stop intent; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "daemon run with network polling sync", stdout.String(), "daemon: running", "mode=network-polling", "daemon: sync_network_polling", "session=daemon-network-sync-000001", "network_transfer=published")
	if stderr.Len() != 0 {
		t.Fatalf("daemon network polling stderr = %q, want empty", stderr.String())
	}
	events, err := agentdaemon.ListLifecycleEvents(target)
	if err != nil {
		t.Fatalf("ListLifecycleEvents() error = %v, want nil", err)
	}
	if !lifecycleEventTypesContain(events, "daemon_sync_started", "daemon_sync_pass", "daemon_sync_stopped", "daemon_stopped") {
		t.Fatalf("daemon lifecycle event types = %#v, want network sync start/pass/stop and daemon stop", lifecycleEventTypes(events))
	}
	transfer := readNetworkTransferForCLI(t, target, "daemon-network-sync-000001")
	if transfer.Status != control.NetworkTransferPublished || transfer.Stage != "commit" {
		t.Fatalf("network transfer = %+v, want published commit", transfer)
	}

	mustWrite(t, filepath.Join(source, "after-network-restart.txt"), "after restart")
	ctx2, cancelDaemon2 := context.WithCancel(context.Background())
	defer cancelDaemon2()
	daemonReady2 := make(chan agentdaemon.State, 1)
	daemonDone2 := make(chan int, 1)
	var stdout2 bytes.Buffer
	var stderr2 bytes.Buffer
	daemonRunner2 := Runner{
		Context: ctx2,
		Now:     cliTLSNow().Add(2 * time.Second),
		DaemonReady: func(state agentdaemon.State) {
			daemonReady2 <- state
		},
	}
	go func() {
		daemonDone2 <- daemonRunner2.Run([]string{"daemon", "run", "--foreground", "--profile", sourceProfilePath, "--listen", "127.0.0.1:0"}, &stdout2, &stderr2)
	}()
	state2 := waitDaemonReadyState(t, daemonReady2, &stdout2, &stderr2)
	if state2.Status != agentdaemon.StatusRunning || state2.Mode != "network-polling" {
		t.Fatalf("second daemon network polling ready state = %+v, want network-polling", state2)
	}
	waitForFile(t, filepath.Join(target, "after-network-restart.txt"), 2*time.Second, &stdout2, &stderr2)
	waitForPublishedRunSessions(t, scheduler, incrementalsync.Scope{ProfileID: sourceProfile.ProfileID, TargetID: sourceProfile.Target.TargetID}, 2*time.Second, "daemon-network-sync-000001", "daemon-network-sync-000002")
	var stopStdout2 bytes.Buffer
	var stopStderr2 bytes.Buffer
	if got := (Runner{Now: cliTLSNow().Add(3 * time.Second)}).Run([]string{"daemon", "stop", "--profile", sourceProfilePath, "--reason", "second test stop"}, &stopStdout2, &stopStderr2); got != 0 {
		t.Fatalf("second daemon stop exit = %d stderr = %q, want 0", got, stopStderr2.String())
	}
	select {
	case got := <-daemonDone2:
		if got != 0 {
			t.Fatalf("second daemon run exit = %d stderr = %q stdout=%q, want 0", got, stderr2.String(), stdout2.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("second daemon run did not exit after stop intent; stdout=%q stderr=%q", stdout2.String(), stderr2.String())
	}
	assertTextContainsAll(t, "second daemon run with network polling sync", stdout2.String(), "daemon: running", "mode=network-polling", "daemon: sync_network_polling", "session=daemon-network-sync-000002", "network_transfer=published")
	if stderr2.Len() != 0 {
		t.Fatalf("second daemon network polling stderr = %q, want empty", stderr2.String())
	}
	transfer2 := readNetworkTransferForCLI(t, target, "daemon-network-sync-000002")
	if transfer2.Status != control.NetworkTransferPublished || transfer2.Stage != "commit" {
		t.Fatalf("second network transfer = %+v, want published commit", transfer2)
	}
	cancelServe()
	select {
	case serveExit := <-serveDone:
		if serveExit != 0 {
			t.Fatalf("serve exit after daemon network polling = %d stderr = %q, want 0", serveExit, serveStderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("serve did not exit after cancel; stderr=%q", serveStderr.String())
	}
}

func TestDaemonRunForegroundLocalPollingSyncResumesSessionSequenceFromReceipts(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "first.txt"), "first")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Sync = &profile.SyncConfig{LocalPolling: &profile.LocalPollingSyncConfig{
		Enabled:            true,
		IntervalMillis:     60000,
		RetryBackoffMillis: 10,
		SessionPrefix:      "daemon-sync",
	}}
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	runDaemonUntilSynced := func(label string, runNow time.Time, wantSession string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		daemonReady := make(chan agentdaemon.State, 1)
		done := make(chan int, 1)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		runner := Runner{
			Context: ctx,
			Now:     runNow,
			DaemonReady: func(state agentdaemon.State) {
				daemonReady <- state
			},
		}
		go func() {
			done <- runner.Run([]string{"daemon", "run", "--foreground", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
		}()
		_ = waitDaemonReadyState(t, daemonReady, &stdout, &stderr)
		scheduler, err := incrementalsync.Open(incrementalsync.Options{
			StateDir: filepath.Join(control.ControlDir(target), "incremental-sync"),
			Now:      func() time.Time { return now },
		})
		if err != nil {
			t.Fatalf("%s: incrementalsync.Open() error = %v, want nil", label, err)
		}
		waitForPublishedRunSessions(t, scheduler, incrementalsync.Scope{ProfileID: p.ProfileID, TargetID: p.Target.TargetID}, 2*time.Second, wantSession)
		var stopStdout bytes.Buffer
		var stopStderr bytes.Buffer
		if got := (Runner{Now: runNow.Add(time.Second)}).Run([]string{"daemon", "stop", "--profile", profilePath, "--reason", label}, &stopStdout, &stopStderr); got != 0 {
			t.Fatalf("%s: daemon stop exit = %d stderr = %q, want 0", label, got, stopStderr.String())
		}
		select {
		case got := <-done:
			if got != 0 {
				t.Fatalf("%s: daemon run exit = %d stderr = %q stdout=%q, want 0", label, got, stderr.String(), stdout.String())
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: daemon run did not exit after stop; stdout=%q stderr=%q", label, stdout.String(), stderr.String())
		}
	}

	runDaemonUntilSynced("first daemon run", now, "daemon-sync-000001")
	mustWrite(t, filepath.Join(source, "second.txt"), "second")
	runDaemonUntilSynced("second daemon run", now.Add(2*time.Second), "daemon-sync-000002")
}

func TestDaemonRunForegroundDoesNotDropStopDuringRestartWindow(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemonReady := make(chan agentdaemon.State, 2)
	restartConsumed := make(chan agentdaemon.State, 1)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	runner := Runner{
		Context: ctx,
		Now:     now,
		DaemonReady: func(state agentdaemon.State) {
			daemonReady <- state
		},
		DaemonRestartConsumed: func(state agentdaemon.State) {
			restartConsumed <- state
		},
	}

	go func() {
		done <- runner.Run([]string{"daemon", "run", "--foreground", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()

	_ = waitDaemonReadyState(t, daemonReady, &stdout, &stderr)
	var restartStdout bytes.Buffer
	var restartStderr bytes.Buffer
	if got := (Runner{Now: now.Add(time.Second)}).Run([]string{"daemon", "restart", "--profile", profilePath, "--reason", "test restart"}, &restartStdout, &restartStderr); got != 0 {
		t.Fatalf("daemon restart exit = %d stderr = %q, want 0", got, restartStderr.String())
	}
	select {
	case <-restartConsumed:
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not consume restart intent; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	var stopStdout bytes.Buffer
	var stopStderr bytes.Buffer
	if got := (Runner{Now: now.Add(2 * time.Second)}).Run([]string{"daemon", "stop", "--profile", profilePath, "--reason", "stop during restart"}, &stopStdout, &stopStderr); got != 0 {
		t.Fatalf("daemon stop exit = %d stderr = %q, want 0", got, stopStderr.String())
	}
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("daemon run exit = %d stderr = %q, want 0", got, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not exit after stop during restart window; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	gotState, err := agentdaemon.ReadState(target)
	if err != nil {
		t.Fatalf("agentdaemon.ReadState() error = %v, want nil", err)
	}
	if gotState.Status != agentdaemon.StatusStopped || gotState.StopIntent == nil || gotState.StopIntent.Reason != "stop during restart" {
		t.Fatalf("daemon final state = %+v, want stopped with stop intent from restart window", gotState)
	}
}

func TestDaemonRunIgnoresForeignStopIntent(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	now := time.Date(2026, 5, 21, 1, 2, 3, 0, time.UTC)
	if err := agentdaemon.WriteStopIntent(target, agentdaemon.NewStopIntent("profile-other", "target-other", "foreign stop", 99, now)); err != nil {
		t.Fatalf("WriteStopIntent(foreign) error = %v, want nil", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	daemonReady := make(chan agentdaemon.State, 1)
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := Runner{
		Context: ctx,
		Now:     now,
		DaemonReady: func(state agentdaemon.State) {
			daemonReady <- state
		},
	}

	go func() {
		done <- runner.Run([]string{"daemon", "run", "--foreground", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &stdout, &stderr)
	}()
	finished := false
	defer func() {
		if finished {
			return
		}
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("daemon run did not exit after test cancellation; stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}()

	var readyState agentdaemon.State
	select {
	case readyState = <-daemonReady:
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not report ready; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	select {
	case got := <-done:
		t.Fatalf("daemon run exited from foreign stop intent with %d; stdout=%q stderr=%q", got, stdout.String(), stderr.String())
	case <-time.After(250 * time.Millisecond):
	}
	if err := agentdaemon.WriteStopIntent(target, agentdaemon.NewStopIntent(readyState.ProfileID, readyState.TargetID, "scoped stop", 100, now.Add(time.Second))); err != nil {
		t.Fatalf("WriteStopIntent(scoped) error = %v, want nil", err)
	}
	select {
	case got := <-done:
		finished = true
		if got != 0 {
			t.Fatalf("daemon run exit = %d stderr = %q, want 0", got, stderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not exit after scoped stop intent; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	cancel()
	gotState, err := agentdaemon.ReadState(target)
	if err != nil {
		t.Fatalf("agentdaemon.ReadState() error = %v, want nil", err)
	}
	if gotState.Status != agentdaemon.StatusStopped || gotState.StopIntent == nil || gotState.StopIntent.Reason != "scoped stop" {
		t.Fatalf("daemon final state = %+v, want stopped with scoped stop intent", gotState)
	}
}

func TestDaemonRunRequiresForeground(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"daemon", "run", "--profile", profilePath}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("daemon run without foreground exit = %d stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--foreground is required") {
		t.Fatalf("daemon run without foreground stderr = %q, want foreground diagnostic", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("daemon run without foreground stdout = %q, want empty", stdout.String())
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

func TestScanRejectsUnsupportedAgentKnowledgeCategoriesBeforeListingPaths(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "AGENTS.md"), "default rules")
	mustWrite(t, filepath.Join(source, "TEAM.md"), "team rules")
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.AgentKnowledge.Categories = []profile.KnowledgeCategory{
		{Name: "repo_rules", Paths: []string{"TEAM.md"}, Manifest: true},
	}
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"scan", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got == 0 {
		t.Fatalf("scan --format json exit = 0, want unsupported agent_knowledge failure")
	}
	if stdout.Len() != 0 {
		t.Fatalf("scan --format json stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "custom agent_knowledge categories are not implemented") {
		t.Fatalf("scan --format json stderr = %q, want unsupported agent_knowledge error", stderr.String())
	}
	for _, leaked := range []string{"AGENTS.md", "TEAM.md", source} {
		if strings.Contains(stdout.String(), leaked) || strings.Contains(stderr.String(), leaked) {
			t.Fatalf("scan leaked path %q; stdout=%q stderr=%q", leaked, stdout.String(), stderr.String())
		}
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

func TestScanRejectsUnsupportedPrivacyPolicyWithoutListingPaths(t *testing.T) {
	tests := []struct {
		name string
		edit func(*profile.Profile)
		want string
	}{
		{
			name: "hidden files disabled",
			edit: func(p *profile.Profile) {
				p.PrivacyPolicy.AllowHiddenFiles = false
			},
			want: "allow_hidden_files=false",
		},
		{
			name: "sensitive filenames disabled",
			edit: func(p *profile.Profile) {
				p.PrivacyPolicy.AllowSensitiveFilenames = false
			},
			want: "allow_sensitive_filenames=false",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			target := filepath.Join(dir, "target")
			profilePath := filepath.Join(dir, "profile.json")
			mustMkdir(t, source)
			mustMkdir(t, target)
			mustWrite(t, filepath.Join(source, ".env"), "secret")
			mustWrite(t, filepath.Join(source, "visible.txt"), "public")
			p := profile.NewDefault("profile-local", "Local profile", source, target)
			tt.edit(&p)
			if err := profile.WriteFile(profilePath, p); err != nil {
				t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
			}

			for _, args := range [][]string{
				{"scan", "--profile", profilePath},
				{"scan", "--profile", profilePath, "--format", "json"},
			} {
				t.Run(strings.Join(args, " "), func(t *testing.T) {
					var stdout bytes.Buffer
					var stderr bytes.Buffer

					got := Run(args, &stdout, &stderr)

					if got == 0 {
						t.Fatalf("Run(%v) exit = 0, want nonzero", args)
					}
					if stdout.Len() != 0 {
						t.Fatalf("Run(%v) stdout = %q, want empty", args, stdout.String())
					}
					if !strings.Contains(stderr.String(), tt.want) {
						t.Fatalf("Run(%v) stderr = %q, want %q", args, stderr.String(), tt.want)
					}
					for _, leaked := range []string{".env", "visible.txt", source} {
						if strings.Contains(stdout.String(), leaked) || strings.Contains(stderr.String(), leaked) {
							t.Fatalf("Run(%v) leaked path %q; stdout=%q stderr=%q", args, leaked, stdout.String(), stderr.String())
						}
					}
				})
			}
		})
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

func TestPushNetworkRejectsUnpairedProfile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"push", "--network", "--profile", profilePath, "--dry-run"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("push --network unpaired exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "profile is not paired") {
		t.Fatalf("push --network unpaired stderr = %q, want unpaired refusal", stderr.String())
	}
	for _, forbidden := range []string{"trusted=true", "network_ready=true", "encrypted_transfer=ready"} {
		if strings.Contains(stdout.String()+stderr.String(), forbidden) {
			t.Fatalf("push --network unpaired output stdout=%q stderr=%q must not contain %q", stdout.String(), stderr.String(), forbidden)
		}
	}
	if stdout.Len() != 0 {
		t.Fatalf("push --network unpaired stdout = %q, want empty", stdout.String())
	}
}

func TestPushNetworkRoutesNetworkModeIndependentOfFlagPosition(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"push", "--profile", profilePath, "--network", "--dry-run"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("push --profile p --network exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "push --network: profile is not paired") {
		t.Fatalf("push --profile p --network stderr = %q, want network unpaired refusal", stderr.String())
	}
	if strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("push --profile p --network stderr = %q, should route to network mode", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("push --profile p --network stdout = %q, want empty", stdout.String())
	}
}

func TestPushNetworkDoesNotScanPastDoubleDash(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"push", "--profile", profilePath, "--", "--network"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("push --profile p -- --network exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "push: unexpected arguments: --network") {
		t.Fatalf("push --profile p -- --network stderr = %q, want local push unexpected arg", stderr.String())
	}
}

func TestPushNetworkEscapesProfileReadErrors(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"push", "--network", "--profile", "missing\ntransfer=ready.json"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("push --network missing profile exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if strings.Contains(stderr.String(), "\ntransfer=ready") || strings.Count(stderr.String(), "\n") > 1 {
		t.Fatalf("push --network missing profile stderr = %q, want single safe diagnostic line", stderr.String())
	}
	if !strings.Contains(stderr.String(), `missing\ntransfer=ready.json`) {
		t.Fatalf("push --network missing profile stderr = %q, want escaped path", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("push --network missing profile stdout = %q, want empty", stdout.String())
	}
}

func TestPushNetworkRejectsMismatchedPairingReceipt(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	setNetworkMaterial(&p, dir)
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p, func(receipt *control.PairingReceipt) {
		receipt.TargetID = "other-target"
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"push", "--network", "--profile", profilePath, "--dry-run"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("push --network mismatched receipt exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "validate local TLS identity files") {
		t.Fatalf("push --network mismatched receipt stderr = %q, want local TLS identity refusal", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("push --network mismatched receipt stdout = %q, want empty", stdout.String())
	}
}

func TestPushNetworkDryRunValidatesPairedProfileWithoutTransfer(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "data.txt"), "dry run\n")
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	p := profile.NewDefault(peer.ProfileID, "Local profile", source, target)
	p.Target.TargetID = peer.TargetID
	p.Target.DevicePublicKey = peer.TargetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	p.Network = networkConfigForCLI(t, sourceCert, net.JoinHostPort("127.0.0.1", "1"))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = p.Target.PairedAt
	})
	before := mustSnapshotTree(t, filepath.Join(target, control.DirName))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{"push", "--network", "--profile", profilePath, "--dry-run", "--session", "net-session"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("push --network dry-run exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"network push: profile=profile.default",
		"target_id=local:profile.default",
		"pairing_receipt=pairing-1",
		"target_device=" + peer.TargetDeviceID,
		"session=net-session",
		"transfer=dry_run",
		"encrypted_transfer=profile_backed_mtls_validated",
		"resume=not_attempted",
		"resume_authority=not_attempted",
		"resume_outcome=not_attempted",
		"files=1",
		"warnings=0",
		"status=dry_run",
		"stage=preflight",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("push --network dry-run stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("push --network dry-run stderr = %q, want empty", stderr.String())
	}
	for _, forbidden := range []string{"trusted=true", "network_ready=true", "sync ready", "transfer=not_wired"} {
		if strings.Contains(stdout.String()+stderr.String(), forbidden) {
			t.Fatalf("push --network dry-run output stdout=%q stderr=%q must not contain %q", stdout.String(), stderr.String(), forbidden)
		}
	}
	if _, err := os.Lstat(filepath.Join(target, control.DirName, "sessions", "net-session")); !os.IsNotExist(err) {
		t.Fatalf("push --network dry-run session artifact err = %v, want no session artifact", err)
	}
	if _, err := os.Lstat(filepath.Join(target, "data.txt")); !os.IsNotExist(err) {
		t.Fatalf("push --network dry-run target file err = %v, want no target file", err)
	}
	after := mustSnapshotTree(t, filepath.Join(target, control.DirName))
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("push --network dry-run mutated control tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

func TestPushNetworkPublishesZeroByteFileWithPrivacyEvidence(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "empty.txt"), "")
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	sourceProfile := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	sourceProfile.Target.TargetID = peer.TargetID
	sourceProfile.Target.DevicePublicKey = peer.TargetDeviceID
	sourceProfile.Target.PairingReceiptID = "pairing-1"
	sourceProfile.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	receiverListener := listenOnReservedTCPAddress(t)
	t.Cleanup(func() {
		_ = receiverListener.Close()
	})
	receiverAddress := receiverListener.Addr().String()
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, receiverAddress)
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, receiverAddress)
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	writePairingReceiptForCLI(t, target, sourceProfile, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = sourceProfile.Target.PairedAt
	})
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	done := make(chan int, 1)
	var serveStdout bytes.Buffer
	var serveStderr bytes.Buffer
	serveRunner := Runner{
		Context:                 serveCtx,
		Now:                     cliTLSNow(),
		receiverListenerForTest: receiverListener,
		ServeReceiverReady: func(info receiverserve.ReadyInfo) {
			receiverReady <- info
		},
	}
	go func() {
		done <- serveRunner.Run([]string{"serve", "--profile", targetProfilePath, "--listen", "127.0.0.1:0"}, &serveStdout, &serveStderr)
	}()
	receiverInfo := waitServeReceiverReady(t, receiverReady, &serveStderr)
	if receiverInfo.Address != receiverAddress || receiverInfo.Peer != peer {
		t.Fatalf("serve receiver info = %+v, want address %q peer %+v", receiverInfo, receiverAddress, peer)
	}

	sessionID := "session-zero-byte-cli"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{"push", "--network", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)

	cancelServe()
	select {
	case serveExit := <-done:
		if serveExit != 0 {
			t.Fatalf("serve exit after zero-byte network push = %d stderr = %q, want 0", serveExit, serveStderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("serve did not exit after cancel; stderr=%q", serveStderr.String())
	}
	if serveStdout.Len() != 0 {
		t.Fatalf("serve stdout = %q, want empty", serveStdout.String())
	}
	if got != 0 {
		t.Fatalf("push --network zero-byte exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "push --network zero-byte", stdout.String(),
		"transfer=published",
		"encrypted_transfer=tls13_mtls",
		"resume=receiver_status",
		"resume_authority=receiver_status",
		"resume_outcome=fresh",
		"files=1",
		"bytes=0",
		"chunks=1",
		"warnings=0",
		"status=published",
		"stage=commit",
	)
	if strings.Contains(stdout.String(), "payload_overhead_missing") {
		t.Fatalf("push --network zero-byte stdout = %q, want no payload_overhead_missing", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("push --network zero-byte stderr = %q, want empty", stderr.String())
	}
	info, err := os.Stat(filepath.Join(target, "empty.txt"))
	if err != nil {
		t.Fatalf("os.Stat(target zero-byte file) error = %v, want nil", err)
	}
	if !info.Mode().IsRegular() || info.Size() != 0 {
		t.Fatalf("target zero-byte file mode=%v size=%d, want regular size 0", info.Mode(), info.Size())
	}
	transfer := readNetworkTransferForCLI(t, target, sessionID)
	if transfer.Status != control.NetworkTransferPublished || transfer.Stage != "commit" || transfer.PrivacyOverhead == nil {
		t.Fatalf("network transfer = %+v, want published commit with privacy overhead", transfer)
	}
	if transfer.PrivacyPolicy.Level != transport.PrivacyLevel2 ||
		transfer.PrivacyOverhead.PaddedChunks == 0 ||
		transfer.PrivacyOverhead.BatchFrames == 0 ||
		transfer.PrivacyOverhead.BatchedChunks == 0 {
		t.Fatalf("network transfer privacy evidence = policy %+v overhead %+v, want level 2 zero-byte payload evidence", transfer.PrivacyPolicy, transfer.PrivacyOverhead)
	}
	if transfer.ErrorCode == "payload_overhead_missing" {
		t.Fatalf("network transfer error_code = %q, want published payload evidence", transfer.ErrorCode)
	}
}

func TestPushNetworkReleaseSmokePublishesAndReportsViaCLI(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "data.txt"), "network payload\n")
	certNotBefore := cliTLSNow().Add(-24 * time.Hour)
	certNotAfter := time.Now().Add(365 * 24 * time.Hour)
	if min := cliTLSNow().Add(365 * 24 * time.Hour); certNotAfter.Before(min) {
		certNotAfter = min
	}
	sourceCert := newCLITestCertificate(t, "source", certNotBefore, certNotAfter)
	targetCert := newCLITestCertificate(t, "target", certNotBefore, certNotAfter)
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	sourceProfile := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	sourceProfile.Target.TargetID = peer.TargetID
	sourceProfile.Target.DevicePublicKey = peer.TargetDeviceID
	sourceProfile.Target.PairingReceiptID = "pairing-1"
	sourceProfile.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	receiverListener := listenOnReservedTCPAddress(t)
	t.Cleanup(func() {
		_ = receiverListener.Close()
	})
	receiverAddress := receiverListener.Addr().String()
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, receiverAddress)
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, receiverAddress)
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	writePairingReceiptForCLI(t, target, sourceProfile, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = sourceProfile.Target.PairedAt
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"profile", "lint", "--profile", sourceProfilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile lint network smoke exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "profile lint network smoke", stdout.String(),
		"privacy policy=status=profile_contract_only",
		"traffic_level=2",
		"claim=bounded_reduction_only",
		"network_transfer=profile_backed_mtls_configured",
		"residual_leakage=",
		"total_bytes",
		"duration",
		"peer_ip",
		"lan_presence",
		"supermover_use",
	)
	assertTextContainsNone(t, "profile lint network smoke", stdout.String(), "anonymous", "anonymity", "transfer_ready=true", "network_ready=true")
	if stderr.Len() != 0 {
		t.Fatalf("profile lint network smoke stderr = %q, want empty", stderr.String())
	}

	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	pairingReady := make(chan pairserve.ReadyInfo, 1)
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	done := make(chan int, 1)
	var serveStdout bytes.Buffer
	var serveStderr bytes.Buffer
	serveRunner := Runner{
		Context:                 serveCtx,
		Now:                     cliTLSNow(),
		receiverListenerForTest: receiverListener,
		ServePairingReady: func(info pairserve.ReadyInfo) {
			pairingReady <- info
		},
		ServeReceiverReady: func(info receiverserve.ReadyInfo) {
			receiverReady <- info
		},
	}
	go func() {
		done <- serveRunner.Run([]string{"serve", "--profile", targetProfilePath, "--listen", "127.0.0.1:0"}, &serveStdout, &serveStderr)
	}()
	waitServePairingReady(t, pairingReady, &serveStderr)
	receiverInfo := waitServeReceiverReady(t, receiverReady, &serveStderr)
	if receiverInfo.Address != receiverAddress || receiverInfo.Peer != peer {
		t.Fatalf("serve receiver info = %+v, want address %q peer %+v", receiverInfo, receiverAddress, peer)
	}

	sessionID := "session-network-cli"
	stdout.Reset()
	stderr.Reset()
	got = Runner{Now: cliTLSNow()}.Run([]string{"push", "--network", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)

	cancelServe()
	select {
	case serveExit := <-done:
		if serveExit != 0 {
			t.Fatalf("serve exit after network push = %d stderr = %q, want 0", serveExit, serveStderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("serve did not exit after cancel; stderr=%q", serveStderr.String())
	}
	if serveStdout.Len() != 0 {
		t.Fatalf("serve stdout = %q, want empty", serveStdout.String())
	}
	assertTextContainsAll(t, "serve network smoke", serveStderr.String(), "receiver_routes=true", "push_network=true")
	if got != 0 {
		t.Fatalf("push --network exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "push --network release smoke", stdout.String(),
		"transfer=published",
		"encrypted_transfer=tls13_mtls",
		"resume=receiver_status",
		"resume_authority=receiver_status",
		"resume_outcome=fresh",
		"files=1",
		"chunks=1",
		"warnings=0",
		"status=published",
		"stage=commit",
	)
	if stderr.Len() != 0 {
		t.Fatalf("push --network stderr = %q, want empty", stderr.String())
	}
	if gotBytes, err := os.ReadFile(filepath.Join(target, "data.txt")); err != nil || string(gotBytes) != "network payload\n" {
		t.Fatalf("target file = (%q, %v), want network payload", string(gotBytes), err)
	}
	transfer := readNetworkTransferForCLI(t, target, sessionID)
	if transfer.Status != control.NetworkTransferPublished || transfer.Stage != "commit" || transfer.PrivacyOverhead == nil {
		t.Fatalf("network transfer = %+v, want published commit with privacy overhead", transfer)
	}
	if transfer.PrivacyPolicy.Level != transport.PrivacyLevel2 || transfer.PrivacyOverhead.PaddingBytes == 0 || transfer.PrivacyOverhead.BatchFrames == 0 || transfer.PrivacyOverhead.JitteredRequests == 0 {
		t.Fatalf("network transfer privacy evidence = policy %+v overhead %+v, want level 2 applied padding/batching/jitter", transfer.PrivacyPolicy, transfer.PrivacyOverhead)
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"verify", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("verify network smoke exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "verify network smoke", stdout.String(), "session="+sessionID, "files=1/1", "errors=0", "warnings=0")
	if stderr.Len() != 0 {
		t.Fatalf("verify network smoke stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"health", "--profile", sourceProfilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("health network smoke exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "health network smoke", stdout.String(), "healthy=true", "network_transfers=0")
	assertTextContainsNone(t, "health network smoke", stdout.String(), "network_transfer session=")
	if stderr.Len() != 0 {
		t.Fatalf("health network smoke stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"status", "--profile", sourceProfilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("status network smoke exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "status network smoke", stdout.String(),
		"status=clean",
		"target_status=local_target_verified",
		"latest_session="+sessionID,
		"network_transfers=0",
		"pairing status=paired_receipt_valid encrypted_transfer=profile_backed_mtls_configured",
		"privacy network_transfer=profile_backed_mtls_configured",
		"traffic_privacy_acceptance status=passed",
		"anonymity_claim=not_claimed",
		"session="+sessionID,
		"observed_padding_bytes=",
		"observed_jitter_budget_millis=250",
		"network evidence_status=no_evidence artifact_problems=0",
	)
	assertTextContainsNone(t, "status network smoke", stdout.String(), "network_ready=true", "transfer_ready=true", "anonymous", "anonymity_claim=claimed")
	if stderr.Len() != 0 {
		t.Fatalf("status network smoke stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"status", "--profile", sourceProfilePath, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("status JSON network smoke exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var gotStatus status.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotStatus); err != nil {
		t.Fatalf("json.Unmarshal(status stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotStatus.Overall.Status != status.OverallClean || gotStatus.Network.Status != status.NetworkStatusNoEvidence || gotStatus.Counts.NetworkTransfers != 0 {
		t.Fatalf("status JSON network smoke = %+v, want clean no-review network evidence", gotStatus)
	}
	if gotStatus.Privacy.NetworkTransfer != "profile_backed_mtls_configured" || !containsString(gotStatus.Privacy.ResidualLeakage, "supermover_use") {
		t.Fatalf("status JSON privacy = %+v, want configured network transfer and residual leakage", gotStatus.Privacy)
	}
	if gotStatus.TrafficPrivacy.Status != "passed" || gotStatus.TrafficPrivacy.SessionID != sessionID || gotStatus.TrafficPrivacy.AnonymityClaim != "not_claimed" || gotStatus.TrafficPrivacy.ObservedOverhead == nil {
		t.Fatalf("status JSON traffic privacy = %+v, want passed artifact-backed acceptance without anonymity claim", gotStatus.TrafficPrivacy)
	}
	if stderr.Len() != 0 {
		t.Fatalf("status JSON network smoke stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"report", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("report network smoke exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "report network smoke", stdout.String(),
		"status=local_target_verified",
		"session="+sessionID,
		"files=1/1",
		"pairing status=paired_receipt_valid",
		"encrypted_transfer=profile_backed_mtls_configured",
		"privacy status=profile_contract_only",
		"traffic_level=2",
		"claim=bounded_reduction_only",
		"network_transfer=profile_backed_mtls_configured",
		"traffic_privacy_acceptance status=passed",
		"anonymity_claim=not_claimed",
		"observed_padding_bytes=",
		"observed_jitter_budget_millis=250",
		"residual_leakage=",
		"total_bytes",
		"duration",
		"peer_ip",
		"lan_presence",
		"supermover_use",
		"profile_snapshot id=profile-"+sessionID,
		"privacy_traffic_level=2",
	)
	assertTextContainsNone(t, "report network smoke", stdout.String(), "network_transfer session=", "network_ready=true", "transfer_ready=true", "anonymous", "anonymity_claim=claimed")
	if stderr.Len() != 0 {
		t.Fatalf("report network smoke stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"report", "--profile", sourceProfilePath, "--session", sessionID, "--format", "json"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("report JSON network smoke exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var gotReport report.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(report stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Overall.Status != report.StatusVerified || gotReport.Summary.NetworkTransfers != 0 || len(gotReport.NetworkTransfers) != 0 {
		t.Fatalf("report JSON network smoke overall=%+v summary=%+v transfers=%#v, want clean published transfer omitted from review issues", gotReport.Overall, gotReport.Summary, gotReport.NetworkTransfers)
	}
	if gotReport.Pairing.EncryptedTransfer != "profile_backed_mtls_configured" || gotReport.Privacy.NetworkTransfer != "profile_backed_mtls_configured" {
		t.Fatalf("report JSON pairing/privacy = %+v/%+v, want profile-backed mTLS configured evidence", gotReport.Pairing, gotReport.Privacy)
	}
	if gotReport.TrafficPrivacy.Status != "passed" || gotReport.TrafficPrivacy.SessionID != sessionID || gotReport.TrafficPrivacy.AnonymityClaim != "not_claimed" || gotReport.TrafficPrivacy.ObservedOverhead == nil {
		t.Fatalf("report JSON traffic privacy = %+v, want passed artifact-backed acceptance without anonymity claim", gotReport.TrafficPrivacy)
	}
	for _, want := range []string{"total_bytes", "duration", "peer_ip", "lan_presence", "supermover_use"} {
		if !containsString(gotReport.Privacy.ResidualLeakage, want) {
			t.Fatalf("report JSON residual leakage = %#v, want %q", gotReport.Privacy.ResidualLeakage, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("report JSON network smoke stderr = %q, want empty", stderr.String())
	}
}

func TestPushNetworkRerunResumesFromReceiverStatus(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceFile := filepath.Join(source, "large.bin")
	writePatternFileForCLI(t, sourceFile, protocol.MaxChunkBytes+137)
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	sourceProfile := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	sourceProfile.Target.TargetID = peer.TargetID
	sourceProfile.Target.DevicePublicKey = peer.TargetDeviceID
	sourceProfile.Target.PairingReceiptID = "pairing-1"
	sourceProfile.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	receiverListener := listenOnReservedTCPAddress(t)
	defer receiverListener.Close()
	receiverAddress := receiverListener.Addr().String()
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, receiverAddress)
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, receiverAddress)
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	writePairingReceiptForCLI(t, target, sourceProfile, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = sourceProfile.Target.PairedAt
	})
	sessionID := "session-network-cli-resume"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	done := make(chan int, 1)
	receiverServer, err := receiverserve.New(receiverserve.Options{
		Profile: targetProfile,
		Now:     cliTLSNow,
		Ready: func(info receiverserve.ReadyInfo) {
			receiverReady <- info
		},
	})
	if err != nil {
		t.Fatalf("receiverserve.New error = %v, want nil", err)
	}
	go func() {
		if err := receiverServer.Serve(ctx, receiverListener); err != nil {
			done <- 1
			return
		}
		done <- 0
	}()
	waitServeReceiverReady(t, receiverReady, nil)
	seeded := seedPartialReceiverSessionForCLI(t, sourceProfile, sourceCert, peer, receiverAddress, sessionID)
	fileSize := fileSizeForCLI(t, sourceFile)
	if seeded <= 0 || seeded >= fileSize {
		t.Fatalf("seeded committed bytes = %d, want partial in (0,%d)", seeded, fileSize)
	}
	wantRemaining := fileSize - seeded

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Runner{Now: cliTLSNow().Add(5 * time.Minute)}.Run([]string{"push", "--network", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)

	cancel()
	select {
	case serveExit := <-done:
		if serveExit != 0 {
			t.Fatalf("receiver serve exit after network resume = %d, want 0", serveExit)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("receiver serve did not exit after cancel")
	}
	if got != 0 {
		t.Fatalf("push --network resume exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"transfer=published",
		"resume=receiver_status",
		"resume_authority=receiver_status",
		"resume_outcome=resumed",
		"resumed_bytes=" + fmt.Sprint(wantRemaining),
		"bytes=" + fmt.Sprint(wantRemaining),
		"status=published",
		"stage=commit",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("push --network resume stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("push --network resume stderr = %q, want empty", stderr.String())
	}
	if hashFileForCLI(t, filepath.Join(target, "large.bin")) != hashFileForCLI(t, sourceFile) {
		t.Fatalf("resumed target large file digest differs from source")
	}
	transfer := readNetworkTransferForCLI(t, target, sessionID)
	if transfer.Status != control.NetworkTransferPublished || transfer.Stage != "commit" || transfer.PrivacyOverhead == nil {
		t.Fatalf("network transfer = %+v, want published commit with privacy overhead", transfer)
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"health", "--profile", targetProfilePath}, &stdout, &stderr)
	if got != 0 || !strings.Contains(stdout.String(), "network_transfers=0") {
		t.Fatalf("health after network resume exit=%d stdout=%q stderr=%q, want clean published network state", got, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"status", "--profile", targetProfilePath}, &stdout, &stderr)
	if got != 0 || !strings.Contains(stdout.String(), "network_transfers=0") {
		t.Fatalf("status after network resume exit=%d stdout=%q stderr=%q, want no transfer review", got, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"report", "--profile", targetProfilePath, "--session", sessionID}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("report after network resume exit=%d stdout=%q stderr=%q, want clean published network state", got, stdout.String(), stderr.String())
	}
	for _, want := range []string{"status=local_target_verified", "session=" + sessionID, "profile_snapshot id=profile-" + sessionID} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report after network resume stdout=%q, want %q", stdout.String(), want)
		}
	}
	if strings.Contains(stdout.String(), "network_transfer session=") {
		t.Fatalf("report after network resume stdout=%q, want published transfer omitted from review issues", stdout.String())
	}
}

func TestPushNetworkInterruptedRunnerAttemptResumesSameSession(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceFile := filepath.Join(source, "large.bin")
	writePatternFileForCLI(t, sourceFile, protocol.MaxChunkBytes+137)
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	sourceProfile := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	sourceProfile.Target.TargetID = peer.TargetID
	sourceProfile.Target.DevicePublicKey = peer.TargetDeviceID
	sourceProfile.Target.PairingReceiptID = "pairing-1"
	sourceProfile.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	sourceProfile.PrivacyPolicy.BatchMaxCount = 1
	receiverListener := listenOnReservedTCPAddress(t)
	defer receiverListener.Close()
	receiverAddress := receiverListener.Addr().String()
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, receiverAddress)
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, receiverAddress)
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	writePairingReceiptForCLI(t, target, sourceProfile, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = sourceProfile.Target.PairedAt
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	done := make(chan int, 1)
	receiverServer := newInterruptingTLSReceiverForCLI(t, targetProfile, targetCert)
	go func() {
		if err := receiverServer.Serve(ctx, receiverListener); err != nil {
			done <- 1
			return
		}
		done <- 0
	}()
	// The test receiver is not the production receiverserve.Server, so the
	// readiness channel only synchronizes the reserved listener address.
	receiverReady <- receiverserve.ReadyInfo{Address: receiverAddress, Peer: peer}
	waitServeReceiverReady(t, receiverReady, nil)
	sessionID := "session-network-cli-interrupted"
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{"push", "--network", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("first interrupted push --network exit = %d, stderr = %q stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "first interrupted push --network", stdout.String(),
		"transfer=failed",
		"resume=receiver_status",
		"resume_authority=receiver_status",
		"resume_outcome=blocked",
		"status=failed",
		"stage=transport",
		"error_code=transfer_failed",
	)
	if !strings.Contains(stderr.String(), "push --network:") {
		t.Fatalf("first interrupted push --network stderr = %q, want push diagnostic", stderr.String())
	}
	failed := readNetworkTransferForCLI(t, target, sessionID)
	if failed.Status != control.NetworkTransferFailed || failed.Stage != "transport" || failed.ErrorCode != "transfer_failed" {
		t.Fatalf("failed network transfer = %+v, want failed transport evidence", failed)
	}
	if failed.PrivacyOverhead == nil || failed.PrivacyOverhead.PaddedChunks == 0 || failed.PrivacyOverhead.BatchFrames == 0 {
		t.Fatalf("failed network transfer overhead = %+v, want accepted chunk/batch privacy overhead", failed.PrivacyOverhead)
	}
	if len(failed.Attempts) != 1 || failed.Attempts[0].Error == "" {
		t.Fatalf("failed network transfer attempts = %+v, want failed attempt diagnostic", failed.Attempts)
	}
	status := readReceiverStatusForCLI(t, pinnedCLIClient(t, sourceCert, peer), receiverAddress, sessionID)
	var committed int64
	for _, file := range status.Files {
		if file.Path == "large.bin" {
			committed = file.CommittedSize
			if file.Complete {
				t.Fatalf("receiver status for large.bin = %+v, want partial", file)
			}
		}
	}
	if committed <= 0 || committed >= fileSizeForCLI(t, sourceFile) {
		t.Fatalf("receiver committed bytes after interrupted push = %d, want partial progress", committed)
	}
	wantRemaining := fileSizeForCLI(t, sourceFile) - committed
	stdout.Reset()
	stderr.Reset()

	receiverServer.failChunks.Store(false)
	got = Runner{Now: cliTLSNow().Add(5 * time.Minute)}.Run([]string{"push", "--network", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)

	cancel()
	select {
	case serveExit := <-done:
		if serveExit != 0 {
			t.Fatalf("receiver serve exit after interrupted network resume = %d, want 0", serveExit)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("receiver serve did not exit after cancel")
	}
	if got != 0 {
		t.Fatalf("second push --network resume exit = %d, stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "second push --network resume", stdout.String(),
		"transfer=published",
		"resume_authority=receiver_status",
		"resume_outcome=resumed",
		"resumed_bytes="+fmt.Sprint(wantRemaining),
		"status=published",
		"stage=commit",
	)
	if strings.Contains(stdout.String(), "resumed_bytes=0") {
		t.Fatalf("second push --network resume stdout = %q, want nonzero resumed bytes", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("second push --network resume stderr = %q, want empty", stderr.String())
	}
	if hashFileForCLI(t, filepath.Join(target, "large.bin")) != hashFileForCLI(t, sourceFile) {
		t.Fatalf("resumed target large file digest differs from source")
	}
	published := readNetworkTransferForCLI(t, target, sessionID)
	if published.Status != control.NetworkTransferPublished || published.Stage != "commit" || published.PrivacyOverhead == nil {
		t.Fatalf("published network transfer = %+v, want published commit evidence", published)
	}
	if len(published.Attempts) != len(failed.Attempts)+1 ||
		published.PrivacyOverhead.PaddedChunks <= failed.PrivacyOverhead.PaddedChunks ||
		published.PrivacyOverhead.BatchFrames <= failed.PrivacyOverhead.BatchFrames {
		t.Fatalf("published network transfer attempts/overhead = attempts %d overhead %+v, want merged from failed attempts %d overhead %+v", len(published.Attempts), published.PrivacyOverhead, len(failed.Attempts), failed.PrivacyOverhead)
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"health", "--profile", sourceProfilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("health after interrupted network resume exit=%d stdout=%q stderr=%q, want clean", got, stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "health after interrupted network resume", stdout.String(), "healthy=true", "network_transfers=0")

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"status", "--profile", sourceProfilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("status after interrupted network resume exit=%d stdout=%q stderr=%q, want clean", got, stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "status after interrupted network resume", stdout.String(), "status=clean", "network_transfers=0")

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"report", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("report after interrupted network resume exit=%d stdout=%q stderr=%q, want clean", got, stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "report after interrupted network resume", stdout.String(),
		"status=local_target_verified",
		"session="+sessionID,
		"privacy status=profile_contract_only",
		"residual_leakage=",
		"total_bytes",
		"duration",
		"peer_ip",
		"lan_presence",
		"supermover_use",
	)
	assertTextContainsNone(t, "report after interrupted network resume", stdout.String(), "network_transfer session=")
}

func TestPushNetworkReceiverRestartAcceptanceMatrix(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceFile := filepath.Join(source, "large.bin")
	writePatternFileForCLI(t, sourceFile, protocol.MaxChunkBytes+137)
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	sourceProfile := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	sourceProfile.Target.TargetID = peer.TargetID
	sourceProfile.Target.DevicePublicKey = peer.TargetDeviceID
	sourceProfile.Target.PairingReceiptID = "pairing-1"
	sourceProfile.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	receiverListener := listenOnReservedTCPAddress(t)
	defer receiverListener.Close()
	receiverAddress := receiverListener.Addr().String()
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, receiverAddress)
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, receiverAddress)
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	writePairingReceiptForCLI(t, target, sourceProfile, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = sourceProfile.Target.PairedAt
	})
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstReady := make(chan receiverserve.ReadyInfo, 1)
	firstDone := make(chan int, 1)
	firstServer, err := receiverserve.New(receiverserve.Options{
		Profile: targetProfile,
		Now:     cliTLSNow,
		Ready: func(info receiverserve.ReadyInfo) {
			firstReady <- info
		},
	})
	if err != nil {
		t.Fatalf("receiverserve.New(first) error = %v, want nil", err)
	}
	go func() {
		if err := firstServer.Serve(firstCtx, receiverListener); err != nil {
			firstDone <- 1
			return
		}
		firstDone <- 0
	}()
	waitServeReceiverReady(t, firstReady, nil)
	sessionID := "session-network-cli-receiver-restart"
	seeded := seedPartialReceiverSessionForCLI(t, sourceProfile, sourceCert, peer, receiverAddress, sessionID)
	fileSize := fileSizeForCLI(t, sourceFile)
	if seeded <= 0 || seeded >= fileSize {
		t.Fatalf("seeded committed bytes = %d, want partial in (0,%d)", seeded, fileSize)
	}
	wantRemaining := fileSize - seeded
	cancelFirst()
	select {
	case serveExit := <-firstDone:
		if serveExit != 0 {
			t.Fatalf("first receiver serve exit = %d, want 0", serveExit)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("first receiver serve did not exit after cancel")
	}

	secondListener := listenOnTCPAddress(t, receiverAddress)
	defer secondListener.Close()
	if secondListener.Addr().String() != receiverAddress {
		t.Fatalf("second receiver address = %q, want preserved receiver URL %q", secondListener.Addr().String(), receiverAddress)
	}
	secondCtx, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	secondReady := make(chan receiverserve.ReadyInfo, 1)
	secondDone := make(chan int, 1)
	secondServer, err := receiverserve.New(receiverserve.Options{
		Profile: targetProfile,
		Now:     cliTLSNow,
		Ready: func(info receiverserve.ReadyInfo) {
			secondReady <- info
		},
	})
	if err != nil {
		t.Fatalf("receiverserve.New(second) error = %v, want nil", err)
	}
	go func() {
		if err := secondServer.Serve(secondCtx, secondListener); err != nil {
			secondDone <- 1
			return
		}
		secondDone <- 0
	}()
	waitServeReceiverReady(t, secondReady, nil)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Runner{Now: cliTLSNow().Add(5 * time.Minute)}.Run([]string{"push", "--network", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("push --network after receiver restart exit = %d, stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "push --network after receiver restart", stdout.String(),
		"transfer=published",
		"resume_authority=receiver_status",
		"resume_outcome=resumed",
		"resumed_bytes="+fmt.Sprint(wantRemaining),
		"bytes="+fmt.Sprint(wantRemaining),
		"status=published",
		"stage=commit",
	)
	if stderr.Len() != 0 {
		t.Fatalf("push --network after receiver restart stderr = %q, want empty", stderr.String())
	}
	if hashFileForCLI(t, filepath.Join(target, "large.bin")) != hashFileForCLI(t, sourceFile) {
		t.Fatalf("resumed target large file digest differs from source")
	}
	published := readNetworkTransferForCLI(t, target, sessionID)
	if published.Status != control.NetworkTransferPublished || published.Stage != "commit" || published.PrivacyOverhead == nil {
		t.Fatalf("published network transfer = %+v, want published commit with privacy overhead", published)
	}
	if published.PrivacyOverhead.PaddedChunks == 0 || published.PrivacyOverhead.BatchFrames == 0 || len(published.Attempts) < 2 {
		t.Fatalf("published network transfer attempts/overhead = attempts %d overhead %+v, want merged retry evidence", len(published.Attempts), published.PrivacyOverhead)
	}

	stdout.Reset()
	stderr.Reset()
	got = Runner{Now: cliTLSNow().Add(10 * time.Minute)}.Run([]string{"push", "--network", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("push --network published retry after receiver restart exit = %d, stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "push --network published retry after receiver restart", stdout.String(),
		"transfer=published",
		"resume_authority=receiver_status",
		"resume_outcome=published_retry",
		"resumed_bytes=0",
		"bytes=0",
		"chunks=0",
	)
	afterRetry := readNetworkTransferForCLI(t, target, sessionID)
	if afterRetry.PrivacyOverhead == nil ||
		afterRetry.PrivacyOverhead.PaddedChunks != published.PrivacyOverhead.PaddedChunks ||
		afterRetry.PrivacyOverhead.BatchFrames != published.PrivacyOverhead.BatchFrames ||
		len(afterRetry.Attempts) != len(published.Attempts)+1 {
		t.Fatalf("network transfer after published retry = attempts %d overhead %+v, want preserved overhead from %+v and one extra attempt", len(afterRetry.Attempts), afterRetry.PrivacyOverhead, published.PrivacyOverhead)
	}
	cancelSecond()
	select {
	case serveExit := <-secondDone:
		if serveExit != 0 {
			t.Fatalf("second receiver serve exit = %d, want 0", serveExit)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("second receiver serve did not exit after cancel")
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"health", "--profile", sourceProfilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("health after receiver restart resume exit=%d stdout=%q stderr=%q, want clean", got, stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "health after receiver restart resume", stdout.String(), "healthy=true", "network_transfers=0")

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"status", "--profile", sourceProfilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("status after receiver restart resume exit=%d stdout=%q stderr=%q, want clean", got, stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "status after receiver restart resume", stdout.String(), "status=clean", "network_transfers=0")

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"report", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("report after receiver restart resume exit=%d stdout=%q stderr=%q, want clean", got, stdout.String(), stderr.String())
	}
	assertTextContainsAll(t, "report after receiver restart resume", stdout.String(), "status=local_target_verified", "session="+sessionID)
	assertTextContainsNone(t, "report after receiver restart resume", stdout.String(), "network_transfer session=")
}

func TestPushNetworkBlocksReceiverResumeWithoutPriorPayloadEvidence(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceFile := filepath.Join(source, "large.bin")
	writePatternFileForCLI(t, sourceFile, protocol.MaxChunkBytes+137)
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	sourceProfile := profile.NewDefault(peer.ProfileID, "Source profile", source, target)
	sourceProfile.Target.TargetID = peer.TargetID
	sourceProfile.Target.DevicePublicKey = peer.TargetDeviceID
	sourceProfile.Target.PairingReceiptID = "pairing-1"
	sourceProfile.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	receiverListener := listenOnReservedTCPAddress(t)
	defer receiverListener.Close()
	receiverAddress := receiverListener.Addr().String()
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, receiverAddress)
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, receiverAddress)
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	writePairingReceiptForCLI(t, target, sourceProfile, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = sourceProfile.Target.PairedAt
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	receiverReady := make(chan receiverserve.ReadyInfo, 1)
	done := make(chan int, 1)
	receiverServer, err := receiverserve.New(receiverserve.Options{
		Profile: targetProfile,
		Now:     cliTLSNow,
		Ready: func(info receiverserve.ReadyInfo) {
			receiverReady <- info
		},
	})
	if err != nil {
		t.Fatalf("receiverserve.New error = %v, want nil", err)
	}
	go func() {
		if err := receiverServer.Serve(ctx, receiverListener); err != nil {
			done <- 1
			return
		}
		done <- 0
	}()
	waitServeReceiverReady(t, receiverReady, nil)
	sessionID := "session-network-cli-missing-prior"
	scanned, err := scan.Scan(sourceProfile.Roots[0].Path)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", sourceProfile.Roots[0].Path, err)
	}
	trust, err := pairing.ValidateProfileTrust(sourceProfile)
	if err != nil {
		t.Fatalf("pairing.ValidateProfileTrust error = %v, want nil", err)
	}
	req := protocolclient.TransferRequest{
		SourceRoot:     sourceProfile.Roots[0].Path,
		Scan:           scanned,
		SessionID:      sessionID,
		ManifestID:     sessionID,
		ProfileID:      sourceProfile.ProfileID,
		TargetID:       sourceProfile.Target.TargetID,
		SourceDeviceID: trust.Receipt.SourceDeviceID,
		TargetDeviceID: trust.TargetDeviceID,
		PrivacyPolicy:  privacyPolicyForCLIProfile(sourceProfile),
		RootID:         sourceProfile.Roots[0].ID,
		CreatedAt:      cliTLSNow(),
	}
	client := pinnedCLIClient(t, sourceCert, peer)
	interrupting := &interruptAfterFirstChunkBatchDoer{next: client}
	_, err = protocolclient.Client{
		BaseURL: "https://" + receiverAddress,
		Doer:    interrupting,
		Now:     cliTLSNow,
	}.Run(context.Background(), req)
	if err == nil {
		t.Fatalf("protocolclient.Client.Run(partial without prior) error = nil, want interrupted upload")
	}
	if !interrupting.interrupted.Load() {
		t.Fatalf("partial seed did not interrupt after a chunk batch")
	}
	status := readReceiverStatusForCLI(t, client, receiverAddress, sessionID)
	var committed int64
	for _, file := range status.Files {
		if file.Path == "large.bin" {
			committed = file.CommittedSize
		}
	}
	if committed <= 0 || committed >= fileSizeForCLI(t, sourceFile) {
		t.Fatalf("receiver committed bytes after partial seed = %d, want partial progress", committed)
	}
	networkTransferPath, err := control.Path(target, control.ArtifactNetworkTransfer, sessionID)
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	if _, statErr := os.Lstat(networkTransferPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("network-transfer before retry stat error = %v, want missing prior evidence", statErr)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Runner{Now: cliTLSNow().Add(5 * time.Minute)}.Run([]string{"push", "--network", "--profile", sourceProfilePath, "--session", sessionID}, &stdout, &stderr)

	cancel()
	select {
	case serveExit := <-done:
		if serveExit != 0 {
			t.Fatalf("receiver serve exit after missing-prior retry = %d, want 0", serveExit)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("receiver serve did not exit after cancel")
	}
	if got != 1 {
		t.Fatalf("push --network missing prior exit = %d, stderr = %q stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "push --network missing prior", stdout.String(),
		"transfer=needs_repair",
		"resume_authority=receiver_status",
		"resume_outcome=blocked",
		"status=needs_repair",
		"stage=network_transfer_artifact",
		"error_code=payload_overhead_missing",
	)
	if !strings.Contains(stderr.String(), "payload privacy overhead evidence is missing") {
		t.Fatalf("push --network missing prior stderr = %q, want payload evidence diagnostic", stderr.String())
	}
	blocked := readNetworkTransferForCLI(t, target, sessionID)
	if blocked.Status != control.NetworkTransferNeedsRepair ||
		blocked.Stage != "network_transfer_artifact" ||
		blocked.ErrorCode != "payload_overhead_missing" ||
		blocked.PrivacyOverhead != nil {
		t.Fatalf("blocked network transfer = %+v, want needs_repair payload_overhead_missing without fabricated overhead", blocked)
	}
	if _, statErr := os.Lstat(filepath.Join(target, "large.bin")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("target file after missing-prior retry stat error = %v, want unpublished target", statErr)
	}
}

func TestRecoverHelpStatesNetworkRecoveryIsUnwired(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"recover", "--help"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("recover --help exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	assertTextContainsAll(t, "recover help", stdout.String(),
		"Reviews and repairs local target control-plane sessions only",
		"does not contact network receivers",
		"resume push --network uploads",
		"perform broad reconcile",
	)
	if stderr.Len() != 0 {
		t.Fatalf("recover --help stderr = %q, want empty", stderr.String())
	}
}

func TestPushNetworkRejectsPairedProfileWithoutNetworkMaterial(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p)
	before := mustSnapshotTree(t, filepath.Join(target, control.DirName))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"push", "--network", "--profile", profilePath, "--session", "net-session"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("push --network missing network material exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "network.receiver_url is required for network client transfer") {
		t.Fatalf("push --network missing network material stderr = %q, want profile network material refusal", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("push --network missing network material stdout = %q, want empty", stdout.String())
	}
	if _, err := os.Lstat(filepath.Join(target, control.DirName, "sessions", "net-session")); !os.IsNotExist(err) {
		t.Fatalf("push --network missing network material session artifact err = %v, want no session artifact", err)
	}
	after := mustSnapshotTree(t, filepath.Join(target, control.DirName))
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("push --network missing network material mutated control tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

func TestPushNetworkRejectsPartialNetworkMaterialWithoutMutatingTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	p.Network = &profile.NetworkConfig{
		ReceiverURL: "https://127.0.0.1:9443",
		LocalTLSIdentity: profile.TLSIdentityRef{
			CertificatePath: filepath.Join(dir, "source.crt"),
		},
	}
	if err := profile.WriteFile(profilePath, p); err == nil || !strings.Contains(err.Error(), "private_key_path is required") {
		t.Fatalf("profile.WriteFile(partial network material) error = %v, want private_key_path validation", err)
	}
	p.Network = nil
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p)
	before := mustSnapshotTree(t, filepath.Join(target, control.DirName))
	partialNetwork, err := json.Marshal(profile.NetworkConfig{
		ReceiverURL: "https://127.0.0.1:9443",
		LocalTLSIdentity: profile.TLSIdentityRef{
			CertificatePath: filepath.Join(dir, "source.crt"),
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal(partial network material) error = %v, want nil", err)
	}
	profileJSON := readFileString(t, profilePath)
	profileWithPartialNetwork := strings.Replace(profileJSON, `"agent_knowledge"`, `"network": `+string(partialNetwork)+`, "agent_knowledge"`, 1)
	if profileWithPartialNetwork == profileJSON {
		t.Fatalf("test fixture did not inject partial network material into profile JSON")
	}
	mustWrite(t, profilePath, profileWithPartialNetwork)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"push", "--network", "--profile", profilePath, "--session", "net-session"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("push --network partial network material exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "private_key_path is required") {
		t.Fatalf("push --network partial network material stderr = %q, want private_key_path validation", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("push --network partial network material stdout = %q, want empty", stdout.String())
	}
	if _, err := os.Lstat(filepath.Join(target, control.DirName, "sessions", "net-session")); !os.IsNotExist(err) {
		t.Fatalf("push --network partial network material session artifact err = %v, want no session artifact", err)
	}
	after := mustSnapshotTree(t, filepath.Join(target, control.DirName))
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("push --network partial network material mutated control tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

func TestPushNetworkTextEscapesProfileAndTargetFields(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "data.txt"), "escape\n")
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	p := profile.NewDefault(peer.ProfileID, "Local profile", source, target)
	p.Target.Name = "target local\ntransfer=ready"
	p.Target.TargetID = peer.TargetID
	p.Target.DevicePublicKey = peer.TargetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	p.Network = networkConfigForCLI(t, sourceCert, net.JoinHostPort("127.0.0.1", "1"))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = p.Target.PairedAt
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{"push", "--network", "--profile", profilePath, "--dry-run", "--session", "net-session"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("push --network escaped text exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{"profile=profile.default", "target_id=local:profile.default", "session=net-session"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("push --network escaped text stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{"\ntransfer=ready"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("push --network escaped text stdout = %q, must not contain %q", stdout.String(), forbidden)
		}
	}
}

func TestEncodeTextValueEscapesUnsafeBytes(t *testing.T) {
	got := encodeTextValue("profile local\ntransfer=ready")
	if got != "profile%20local%0Atransfer%3Dready" {
		t.Fatalf("encodeTextValue() = %q, want escaped whitespace and equals", got)
	}
}

func TestPushNetworkJSONReportsContractWithoutTransferReadiness(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "data.txt"), "json\n")
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	p := profile.NewDefault(peer.ProfileID, "Local profile", source, target)
	p.Target.TargetID = peer.TargetID
	p.Target.DevicePublicKey = peer.TargetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	p.Network = networkConfigForCLI(t, sourceCert, net.JoinHostPort("127.0.0.1", "1"))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = p.Target.PairedAt
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{"push", "--network", "--profile", profilePath, "--dry-run", "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("push --network json exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var plan networkPushPlan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("json.Unmarshal(push --network stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if plan.ProfileID != p.ProfileID || plan.TargetID != p.Target.TargetID || plan.PairingReceiptID != p.Target.PairingReceiptID {
		t.Fatalf("push --network json plan = %+v, want profile/target/receipt binding", plan)
	}
	if plan.Transfer != "dry_run" || plan.EncryptedTransfer != "profile_backed_mtls_validated" || plan.Resume != "not_attempted" || plan.ResumeAuthority != "not_attempted" || plan.ResumeOutcome != "not_attempted" || plan.Status != "dry_run" || plan.Stage != "preflight" {
		t.Fatalf("push --network json plan = %+v, want dry-run transfer fields", plan)
	}
	for _, forbidden := range []string{"trusted", "network_ready", "transfer_ready", "not_wired"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("push --network json stdout = %q, must not contain %q", stdout.String(), forbidden)
		}
	}
}

func TestPushNetworkJSONWritesSourceBaselineWhenRequested(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	baselinePath := filepath.Join(dir, "baseline.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "data.txt"), "json\n")
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	p := profile.NewDefault(peer.ProfileID, "Local profile", source, target)
	p.Target.TargetID = peer.TargetID
	p.Target.DevicePublicKey = peer.TargetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	p.Network = networkConfigForCLI(t, sourceCert, net.JoinHostPort("127.0.0.1", "1"))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = p.Target.PairedAt
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{
		"push", "--network",
		"--profile", profilePath,
		"--dry-run",
		"--format", "json",
		"--source-baseline", baselinePath,
	}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("push --network --source-baseline exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var plan networkPushPlan
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("json.Unmarshal(push --network --source-baseline stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if plan.SourceBaseline != baselinePath {
		t.Fatalf("push --network plan = %+v, want source_baseline=%q", plan, baselinePath)
	}
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", baselinePath, err)
	}
	var baseline sourceconsistency.Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatalf("json.Unmarshal(source baseline) error = %v, data = %q", err, string(data))
	}
	if baseline.Schema != sourceconsistency.Schema || baseline.ProfileID != p.ProfileID || baseline.RootID != p.Roots[0].ID || len(baseline.Entries) != 1 || baseline.Entries[0].Path != "data.txt" {
		t.Fatalf("source baseline = %+v, want exact transfer baseline for data.txt", baseline)
	}
}

func TestVerifySourceConsistencyJSONPassesAndFailsOnCurrentSourceChanges(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	baselinePath := filepath.Join(dir, "baseline.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustWrite(t, filepath.Join(source, "data.txt"), "json\n")
	p := profile.NewDefault("profile-source", "Local profile", source, target)
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	baseline, entries, err := buildSourceConsistencyBaseline(p, "session-source-1", Runner{Now: cliTLSNow()}.nowFunc())
	if err != nil {
		t.Fatalf("buildSourceConsistencyBaseline error = %v, want nil", err)
	}
	if len(entries) != 1 || entries[0].Path != "data.txt" {
		t.Fatalf("buildSourceConsistencyBaseline entries = %+v, want data.txt transfer entry", entries)
	}
	if err := writeSourceConsistencyBaseline(baselinePath, baseline); err != nil {
		t.Fatalf("writeSourceConsistencyBaseline(%q) error = %v, want nil", baselinePath, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Runner{Now: cliTLSNow()}.Run([]string{
		"verify", "source-consistency",
		"--profile", profilePath,
		"--baseline", baselinePath,
		"--format", "json",
	}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("verify source-consistency pass exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var report sourceconsistency.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal(verify source-consistency stdout) error = %v, stdout = %q", err, stdout.String())
	}
	if report.Status != sourceconsistency.StatusPass || report.Mode != sourceconsistency.ModeCurrentVerified || report.MismatchCount != 0 {
		t.Fatalf("verify source-consistency pass report = %+v, want pass/current_source_verified with zero mismatches", report)
	}
	if stderr.Len() != 0 {
		t.Fatalf("verify source-consistency pass stderr = %q, want empty", stderr.String())
	}

	mustWrite(t, filepath.Join(source, "data.txt"), "changed\n")
	stdout.Reset()
	stderr.Reset()
	got = Runner{Now: cliTLSNow().Add(time.Minute)}.Run([]string{
		"verify", "source-consistency",
		"--profile", profilePath,
		"--baseline", baselinePath,
		"--format", "json",
	}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("verify source-consistency changed exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal(changed verify source-consistency stdout) error = %v, stdout = %q", err, stdout.String())
	}
	if report.Status != sourceconsistency.StatusBlocked || report.Mode != sourceconsistency.ModeCurrentMismatch || report.MismatchCount == 0 {
		t.Fatalf("verify source-consistency changed report = %+v, want blocked/current_source_mismatch", report)
	}
	if stderr.Len() != 0 {
		t.Fatalf("verify source-consistency changed stderr = %q, want empty", stderr.String())
	}
}

func TestPruneValidatesProfilePolicyButDoesNotMutateTarget(t *testing.T) {
	tests := []struct {
		name     string
		modeFlag string
		wantExit int
		wantText string
		wantErr  string
	}{
		{name: "dry run", modeFlag: "--dry-run", wantExit: 0, wantText: "prune dry-run:"},
		{name: "apply", modeFlag: "--apply", wantExit: 2, wantText: "", wantErr: "--apply requires --approval"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			target := filepath.Join(dir, "target")
			profilePath := filepath.Join(dir, "profile.json")
			mustMkdir(t, source)
			mustMkdir(t, target)
			mustWrite(t, filepath.Join(target, "stale.txt"), "keep")
			p := profile.NewDefault("profile-local", "Local profile", source, target)
			p.DeletePolicy.Mode = profile.DeleteModePrune
			p.DeletePolicy.RequireReview = true
			p.DeletePolicy.AllowPhysicalPrune = true
			if err := profile.WriteFile(profilePath, p); err != nil {
				t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
			}
			if tt.modeFlag == "--dry-run" {
				writeEmptyPublishedSessionForCLI(t, target, "session-empty")
			}
			before := mustSnapshotTree(t, target)
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run([]string{"prune", "--profile", profilePath, tt.modeFlag}, &stdout, &stderr)

			if got != tt.wantExit {
				t.Fatalf("prune %s exit = %d, stderr = %q, stdout = %q, want %d", tt.modeFlag, got, stderr.String(), stdout.String(), tt.wantExit)
			}
			if tt.wantText != "" && !strings.Contains(stdout.String(), tt.wantText) {
				t.Fatalf("prune %s stdout = %q, want %q", tt.modeFlag, stdout.String(), tt.wantText)
			}
			if tt.wantErr != "" && !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("prune %s stderr = %q, want %q", tt.modeFlag, stderr.String(), tt.wantErr)
			}
			after := mustSnapshotTree(t, target)
			if strings.Join(after, "\n") != strings.Join(before, "\n") {
				t.Fatalf("prune %s changed target tree\nbefore:\n%s\nafter:\n%s", tt.modeFlag, strings.Join(before, "\n"), strings.Join(after, "\n"))
			}
		})
	}
}

func TestPruneDryRunTextReportsEmptyPlan(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.AllowPhysicalPrune = true
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writeEmptyPublishedSessionForCLI(t, target, "session-empty")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"prune", "--profile", profilePath, "--dry-run"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune dry-run exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"prune dry-run:",
		"profile=profile-local",
		"target_id=local:profile-local",
		"soft_deletes=0",
		"candidates=0",
		"refusals=0",
		"artifact_problems=0",
		"approval_required=true",
		"physical_pruning=not_applied",
		"approval_writing=not_written_by_dry_run",
		"receipt_writing=not_written_by_dry_run",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("prune dry-run stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune dry-run stderr = %q, want empty", stderr.String())
	}
}

func TestPruneTextEscapesProfileAndTargetFields(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile local\nstatus=ready", "Local profile", source, target)
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.AllowPhysicalPrune = true
	p.Target.TargetID = "target local\nprune=ready"
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writeEmptyPublishedSessionForCLI(t, target, "session-empty")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"prune", "--profile", profilePath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune escaped text exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{"profile=profile%20local%0Astatus%3Dready", "target_id=target%20local%0Aprune%3Dready"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("prune escaped text stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{"profile=profile local", "target_id=target local", "\nprune=ready"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("prune escaped text stdout = %q, must not contain %q", stdout.String(), forbidden)
		}
	}
}

func TestPruneApplyRequiresApproval(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.AllowPhysicalPrune = true
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"prune", "--profile", profilePath, "--format=json", "--apply"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("prune --apply without approval exit = %d, stderr = %q, stdout = %q, want 2", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "--apply requires --approval") {
		t.Fatalf("prune --apply without approval stderr = %q, want approval requirement", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("prune --apply without approval stdout = %q, want empty", stdout.String())
	}
}

func TestPruneDryRunShowsSoftDeleteCandidatesWithoutMutatingTarget(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	before := mustSnapshotTree(t, target)
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"prune", "--profile", profilePath, "--dry-run"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("prune dry-run exit = %d, stderr = %q, stdout = %q, want 1 for review-required candidates", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"prune dry-run:",
		"schema=supermover.prune_dry_run.v1",
		"policy_mode=prune",
		"policy_require_review=true",
		"policy_allow_physical_prune=true",
		"soft_deletes=1",
		"candidates=1",
		"refusals=0",
		"artifact_problems=0",
		"prune_candidate",
		"session=session-two",
		"previous_session=session-one",
		"previous_manifest=manifest-session-one",
		"source=gone.txt",
		"target=gone.txt",
		"kind=file",
		"size=4",
		"digest=sha256:",
		"previous_source=gone.txt",
		"previous_target=gone.txt",
		"previous_kind=file",
		"previous_size=4",
		"previous_digest=sha256:",
		"observed_present=true",
		"observed_path=gone.txt",
		"observed_kind=file",
		"observed_digest=sha256:",
		"action=delete_file",
		"physical_pruning=not_applied",
		"approval_writing=not_written_by_dry_run",
		"receipt_writing=not_written_by_dry_run",
		"review_required=true",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("prune dry-run stdout = %q, want %q", stdout.String(), want)
		}
	}
	if !strings.Contains(stderr.String(), "dry-run produced review-required evidence; no target files were deleted") {
		t.Fatalf("prune dry-run stderr = %q, want review-required diagnostic", stderr.String())
	}
	after := mustSnapshotTree(t, target)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("prune dry-run changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

func TestPruneDryRunJSONReportsRefusals(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	mustWrite(t, filepath.Join(target, "gone.txt"), "changed")
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"prune", "--profile", profilePath, "--dry-run", "--format=json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("prune dry-run json exit = %d, stderr = %q, stdout = %q, want 1 for refusal", got, stderr.String(), stdout.String())
	}
	var report struct {
		Schema              string `json:"schema"`
		DryRun              bool   `json:"dry_run"`
		ApprovalRequired    bool   `json:"approval_required"`
		ProfileDeletePolicy struct {
			Mode               string `json:"mode"`
			RequireReview      bool   `json:"require_review"`
			AllowPhysicalPrune bool   `json:"allow_physical_prune"`
		} `json:"profile_delete_policy"`
		Summary struct {
			SoftDeletes      int `json:"soft_deletes"`
			Candidates       int `json:"candidates"`
			Refusals         int `json:"refusals"`
			ArtifactProblems int `json:"artifact_problems"`
		} `json:"summary"`
		Refusals []struct {
			ReasonCode         string `json:"reason_code"`
			SoftDeleteID       string `json:"soft_delete_id"`
			CurrentTargetState struct {
				Present *bool  `json:"present"`
				Kind    string `json:"kind"`
				Digest  string `json:"digest"`
			} `json:"current_target_state"`
		} `json:"refusals"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal(prune dry-run stdout) error = %v, stdout = %q", err, stdout.String())
	}
	if report.Schema != "supermover.prune_dry_run.v1" || !report.DryRun || !report.ApprovalRequired {
		t.Fatalf("prune dry-run json metadata = %+v, want dry-run schema/review flags", report)
	}
	if report.ProfileDeletePolicy.Mode != "prune" || !report.ProfileDeletePolicy.RequireReview || !report.ProfileDeletePolicy.AllowPhysicalPrune {
		t.Fatalf("prune dry-run json policy = %+v, want profile prune policy", report.ProfileDeletePolicy)
	}
	if report.Summary.SoftDeletes != 1 || report.Summary.Candidates != 0 || report.Summary.Refusals != 1 || report.Summary.ArtifactProblems != 0 {
		t.Fatalf("prune dry-run json summary = %+v, want one refusal", report.Summary)
	}
	if len(report.Refusals) != 1 || report.Refusals[0].ReasonCode != "target_content_mismatch" || report.Refusals[0].SoftDeleteID == "" {
		t.Fatalf("prune dry-run json refusals = %+v, want content mismatch refusal", report.Refusals)
	}
	if report.Refusals[0].CurrentTargetState.Present == nil || !*report.Refusals[0].CurrentTargetState.Present || report.Refusals[0].CurrentTargetState.Kind != "file" || report.Refusals[0].CurrentTargetState.Digest == "" {
		t.Fatalf("prune dry-run json observed = %+v, want present file evidence", report.Refusals[0].CurrentTargetState)
	}
}

func TestPruneApplyDeletesApprovedFileAndWritesReceipt(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	approval := pruneApprovalForCLI(t, profilePath, target, "approval-cli", softDeleteIDForCLI(t, target, "gone.txt"))
	writePruneApprovalForCLI(t, target, approval)
	stdout.Reset()
	stderr.Reset()

	runner := Runner{Now: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)}
	got := runner.Run([]string{"prune", "--profile", profilePath, "--apply", "--approval", "approval-cli"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune apply exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if _, err := os.Lstat(filepath.Join(target, "gone.txt")); !os.IsNotExist(err) {
		t.Fatalf("Lstat(pruned target) error = %v, want not exist", err)
	}
	for _, want := range []string{"prune apply:", "approval=approval-cli", "status=applied", "prune_result", "target=gone.txt", "result=pruned"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("prune apply stdout = %q, want %q", stdout.String(), want)
		}
	}
	receipt, err := control.ReadFile[control.PruneReceipt](receiptPathForCLI(t, target, "approval-cli"))
	if err != nil {
		t.Fatalf("control.ReadFile(prune receipt) error = %v, want nil", err)
	}
	if receipt.Status != control.PruneReceiptApplied || receipt.ID != "approval-cli" || receipt.Items[0].Result != "pruned" {
		t.Fatalf("prune receipt = %+v, want applied receipt", receipt)
	}
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"report", "--profile", profilePath}, &stdout, &stderr); got != 1 {
		t.Fatalf("report after prune apply exit = %d, stderr = %q, stdout = %q, want 1 from verification/soft-delete evidence", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"prune_receipts=1",
		"prune_receipt_issues=0",
		"prune_review status=no_pending_review",
		"prune_receipt id=approval-cli",
		"status=applied",
		"action=inspect_applied_prune_receipt",
		"prune_receipt_item receipt=approval-cli",
		"target=gone.txt",
		"result=pruned",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report after prune apply stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("report after prune apply stderr = %q, want empty", stderr.String())
	}
}

func TestPruneApproveWritesApprovalWithoutDeletingTarget(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	softDeleteID := softDeleteIDForCLI(t, target, "gone.txt")
	stdout.Reset()
	stderr.Reset()

	runner := Runner{Now: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)}
	got := runner.Run([]string{"prune", "approve", "--profile", profilePath, "--id", "approval-authored", "--soft-delete", softDeleteID, "--reason", "reviewed stale file", "--reviewer", "cli-reviewer"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune approve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"prune_approval",
		"id=approval-authored",
		"profile=profile-local",
		"target_id=local:profile-local",
		"status=approved",
		"items=1",
		"approved_by=cli-reviewer",
		"approval_writing=written",
		"profile_snapshot_writing=written",
		"physical_pruning=not_applied",
		"receipt_writing=not_written_by_approval_authoring",
		"prune_approval_item",
		"soft_delete=" + softDeleteID,
		"target=gone.txt",
		"action=approve_for_prune",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("prune approve stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune approve stderr = %q, want empty", stderr.String())
	}
	if _, err := os.Lstat(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("Lstat(target after approve) error = %v, want retained target file", err)
	}
	if _, err := os.Lstat(receiptPathForCLI(t, target, "approval-authored")); !os.IsNotExist(err) {
		t.Fatalf("Lstat(receipt after approve) error = %v, want no receipt", err)
	}
	approval, err := control.ReadFile[control.PruneApproval](approvalPathForCLI(t, target, "approval-authored"))
	if err != nil {
		t.Fatalf("control.ReadFile(authored approval) error = %v, want nil", err)
	}
	if approval.ReviewTool != "supermover prune approve" || approval.ApprovalReason != "reviewed stale file" || len(approval.Items) != 1 || approval.Items[0].SoftDeleteID != softDeleteID {
		t.Fatalf("authored approval = %+v, want CLI review artifact", approval)
	}
	snapshotPath, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-approval-authored")
	if err != nil {
		t.Fatalf("control.Path(profile snapshot) error = %v, want nil", err)
	}
	if _, err := control.ReadFile[control.ProfileSnapshot](snapshotPath); err != nil {
		t.Fatalf("control.ReadFile(authored profile snapshot) error = %v, want nil", err)
	}
}

func TestPruneApprovalsListsCurrentScopeArtifacts(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	softDeleteID := softDeleteIDForCLI(t, target, "gone.txt")

	runner := Runner{Now: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)}
	stdout.Reset()
	stderr.Reset()
	if got := runner.Run([]string{"prune", "approve", "--profile", profilePath, "--id", "approval-authored", "--soft-delete", softDeleteID, "--reason", "reviewed stale file", "--reviewer", "cli-reviewer"}, &stdout, &stderr); got != 0 {
		t.Fatalf("prune approve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	got := Run([]string{"prune", "approvals", "--profile", profilePath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune approvals exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"prune_approvals",
		"approvals=1",
		"read_only=true",
		"prune_approval id=approval-authored",
		"status=approved",
		"approved_by=cli-reviewer",
		"approval_reason=reviewed%20stale%20file",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("prune approvals stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune approvals stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"prune", "approvals", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune approvals json exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var result prune.ListApprovalsResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(prune approvals stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if result.ProfileID != "profile-local" || result.TargetID != "local:profile-local" || len(result.Approvals) != 1 || result.Approvals[0].ID != "approval-authored" || result.Approvals[0].Status != "approved" {
		t.Fatalf("prune approvals json result = %+v, want one current-scope approval", result)
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune approvals json stderr = %q, want empty", stderr.String())
	}
}

func TestPruneSupersedeMarksApprovalWithoutDeletingTarget(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	softDeleteID := softDeleteIDForCLI(t, target, "gone.txt")

	runner := Runner{Now: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)}
	stdout.Reset()
	stderr.Reset()
	if got := runner.Run([]string{"prune", "approve", "--profile", profilePath, "--id", "approval-authored", "--soft-delete", softDeleteID, "--reason", "reviewed stale file", "--reviewer", "cli-reviewer"}, &stdout, &stderr); got != 0 {
		t.Fatalf("prune approve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}

	runner = Runner{Now: time.Date(2026, 5, 18, 11, 0, 0, 0, time.UTC)}
	stdout.Reset()
	stderr.Reset()
	got := runner.Run([]string{"prune", "supersede", "--profile", profilePath, "--id", "approval-authored", "--reason", "replaced by newer approval", "--reviewer", "release-reviewer"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune supersede exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"prune_approval_supersede",
		"id=approval-authored",
		"status=superseded",
		"approved_by=cli-reviewer",
		"superseded_by=release-reviewer",
		"superseded_at=2026-05-18T11:00:00Z",
		"review_tool=supermover%20prune%20supersede",
		"refusal_reason=replaced%20by%20newer%20approval",
		"physical_pruning=not_applied",
		"receipt_writing=not_written_by_supersede",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("prune supersede stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune supersede stderr = %q, want empty", stderr.String())
	}
	if _, err := os.Lstat(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("Lstat(target after supersede) error = %v, want retained target file", err)
	}
	if _, err := os.Lstat(receiptPathForCLI(t, target, "approval-authored")); !os.IsNotExist(err) {
		t.Fatalf("Lstat(receipt after supersede) error = %v, want no receipt", err)
	}
	approval, err := control.ReadFile[control.PruneApproval](approvalPathForCLI(t, target, "approval-authored"))
	if err != nil {
		t.Fatalf("control.ReadFile(superseded approval) error = %v, want nil", err)
	}
	if approval.Status != "superseded" || approval.RefusalReason != "replaced by newer approval" || approval.ApprovedBy != "cli-reviewer" || approval.SupersededBy != "release-reviewer" || approval.SupersededAt != "2026-05-18T11:00:00Z" || approval.ReviewTool != "supermover prune supersede" {
		t.Fatalf("superseded approval = %+v, want superseded review metadata", approval)
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"report", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("report after prune supersede exit = %d, stderr = %q, stdout = %q, want review-needed 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"prune_approval id=approval-authored",
		"status=superseded",
		"release_state=superseded",
		"release_action=inspect_superseded_prune_approval",
		"unapplied=false",
		"superseded_by=release-reviewer",
		"superseded_at=2026-05-18T11:00:00Z",
		"refusal_reason=replaced%20by%20newer%20approval",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report after prune supersede stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestPruneApprovalsRejectsMismatchedApprovalPath(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	softDeleteID := softDeleteIDForCLI(t, target, "gone.txt")
	approval := pruneApprovalForCLI(t, profilePath, target, "approval-real", softDeleteID)
	writePruneApprovalForCLI(t, target, approval)
	mismatchPath := filepath.Join(target, control.DirName, "prune", "approvals", "approval-path.json")
	if err := control.WriteNewFile(mismatchPath, approval); err != nil {
		t.Fatalf("control.WriteNewFile(mismatched approval path) error = %v, want nil", err)
	}
	if err := os.Remove(approvalPathForCLI(t, target, "approval-real")); err != nil {
		t.Fatalf("os.Remove(original approval path) error = %v, want nil", err)
	}

	stdout.Reset()
	stderr.Reset()
	got := Run([]string{"prune", "approvals", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("prune approvals mismatched path exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "does not match path id") {
		t.Fatalf("prune approvals mismatched path stderr = %q, want path/id mismatch", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("prune approvals mismatched path stdout = %q, want empty", stdout.String())
	}
}

func TestPruneSupersedeRejectsLinkedReceiptEvidence(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	softDeleteID := softDeleteIDForCLI(t, target, "gone.txt")

	runner := Runner{Now: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)}
	stdout.Reset()
	stderr.Reset()
	if got := runner.Run([]string{"prune", "approve", "--profile", profilePath, "--id", "approval-authored", "--soft-delete", softDeleteID, "--reason", "reviewed stale file", "--reviewer", "cli-reviewer"}, &stdout, &stderr); got != 0 {
		t.Fatalf("prune approve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if got := Run([]string{"prune", "--profile", profilePath, "--apply", "--approval", "approval-authored"}, &stdout, &stderr); got != 0 {
		t.Fatalf("prune apply exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}

	runner = Runner{Now: time.Date(2026, 5, 18, 11, 0, 0, 0, time.UTC)}
	stdout.Reset()
	stderr.Reset()
	got := runner.Run([]string{"prune", "supersede", "--profile", profilePath, "--id", "approval-authored", "--reason", "replaced by newer approval", "--reviewer", "release-reviewer"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("prune supersede linked receipt exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "already has prune receipt evidence") {
		t.Fatalf("prune supersede linked receipt stderr = %q, want linked receipt refusal", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("prune supersede linked receipt stdout = %q, want empty", stdout.String())
	}
}

func TestPruneSupersedeRequiresReviewer(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"prune", "supersede", "--profile", "profile.json", "--id", "approval", "--reason", "replaced"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("prune supersede missing reviewer exit = %d, stderr = %q, stdout = %q, want 2", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "--reviewer is required") {
		t.Fatalf("prune supersede missing reviewer stderr = %q, want reviewer-required", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("prune supersede missing reviewer stdout = %q, want empty", stdout.String())
	}
}

func TestReportAndStatusShowAuthoredPruneApprovalInventory(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	softDeleteID := softDeleteIDForCLI(t, target, "gone.txt")
	stdout.Reset()
	stderr.Reset()

	runner := Runner{Now: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)}
	got := runner.Run([]string{"prune", "approve", "--profile", profilePath, "--id", "approval-authored", "--soft-delete", softDeleteID, "--reason", "reviewed stale file", "--reviewer", "cli-reviewer"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("prune approve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"report", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("report after prune approve exit = %d, stderr = %q, stdout = %q, want review-needed 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"prune_approvals=1",
		"prune_unapplied_approvals=1",
		"issues=soft_deletes,prune_candidates,prune_unapplied_approvals",
		"prune_review status=review_required",
		"approval_source=control_plane_prune_approvals",
		"approvals=1",
		"unapplied_approvals=1",
		"prune_approval id=approval-authored",
		"status=approved",
		"items=1",
		"unapplied=true",
		"action=inspect_prune_approval_before_apply",
		"physical_pruning=not_applied",
		"approved_by=cli-reviewer",
		"prune_approval_item approval=approval-authored",
		"soft_delete=" + softDeleteID,
		"target=gone.txt",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report after prune approve stdout = %q, want %q", stdout.String(), want)
		}
	}
	if strings.Contains(stdout.String(), "prune_receipt id=approval-authored") {
		t.Fatalf("report after prune approve stdout = %q, want no applied prune receipt", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("report after prune approve stderr = %q, want empty", stderr.String())
	}
	if _, err := os.Lstat(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("Lstat(target after report) error = %v, want target file untouched", err)
	}
	if _, err := os.Lstat(receiptPathForCLI(t, target, "approval-authored")); !os.IsNotExist(err) {
		t.Fatalf("Lstat(receipt after report) error = %v, want no receipt", err)
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"report", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("report json after prune approve exit = %d, stderr = %q, stdout = %q, want review-needed 1", got, stderr.String(), stdout.String())
	}
	var reportJSON report.Report
	if err := json.Unmarshal(stdout.Bytes(), &reportJSON); err != nil {
		t.Fatalf("json.Unmarshal(report stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if reportJSON.Summary.PruneApprovals != 1 || reportJSON.Summary.PruneUnappliedApprovals != 1 {
		t.Fatalf("report json summary = %+v, want one unapplied approval", reportJSON.Summary)
	}
	if len(reportJSON.PruneReview.Approvals) != 1 || reportJSON.PruneReview.Approvals[0].ID != "approval-authored" || !reportJSON.PruneReview.Approvals[0].Unapplied || len(reportJSON.PruneReview.Approvals[0].Items) != 1 {
		t.Fatalf("report json prune approvals = %+v, want detailed approval inventory", reportJSON.PruneReview.Approvals)
	}
	if stderr.Len() != 0 {
		t.Fatalf("report json after prune approve stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"status", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("status after prune approve exit = %d, stderr = %q, stdout = %q, want review-needed 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"status:",
		"review_required=true",
		"prune_review_status=review_required",
		"prune_review_action=inspect_prune_review_before_release",
		"prune_approvals=1",
		"prune_unapplied_approvals=1",
		"prune_active_approvals=1",
		"prune_stale_approvals=0",
		"prune_expired_approvals=0",
		"prune_consumed_approvals=0",
		"prune_receipts=0",
		"prune_receipt_issues=0",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status after prune approve stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("status after prune approve stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"status", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("status json after prune approve exit = %d, stderr = %q, stdout = %q, want review-needed 1", got, stderr.String(), stdout.String())
	}
	var statusJSON status.Report
	if err := json.Unmarshal(stdout.Bytes(), &statusJSON); err != nil {
		t.Fatalf("json.Unmarshal(status stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if statusJSON.PruneReview.Status != string(report.PruneReviewReviewRequired) || statusJSON.PruneReview.Action != "inspect_prune_review_before_release" {
		t.Fatalf("status json prune review = %+v, want compact review-required action", statusJSON.PruneReview)
	}
	if statusJSON.Counts.PruneApprovals != 1 || statusJSON.Counts.PruneUnappliedApprovals != 1 || statusJSON.Counts.PruneActiveApprovals != 1 || statusJSON.Counts.PruneStaleApprovals != 0 || statusJSON.Counts.PruneExpiredApprovals != 0 || statusJSON.Counts.PruneConsumedApprovals != 0 || statusJSON.Counts.PruneReceipts != 0 || statusJSON.Counts.PruneReceiptIssues != 0 {
		t.Fatalf("status json counts = %+v, want compact prune readiness counts", statusJSON.Counts)
	}
	if stderr.Len() != 0 {
		t.Fatalf("status json after prune approve stderr = %q, want empty", stderr.String())
	}
}

func TestPruneReviewShowsReleaseInventoryWithoutMutatingTarget(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	softDeleteID := softDeleteIDForCLI(t, target, "gone.txt")
	stdout.Reset()
	stderr.Reset()

	runner := Runner{Now: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)}
	got := runner.Run([]string{"prune", "approve", "--profile", profilePath, "--id", "approval-authored", "--soft-delete", softDeleteID, "--reason", "reviewed stale file", "--reviewer", "cli-reviewer"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("prune approve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	before := mustSnapshotTree(t, target)
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"prune", "review", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("prune review exit = %d, stderr = %q, stdout = %q, want review-needed 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"prune release review:",
		"schema=supermover.prune_release_review.v1",
		"status=review_required",
		"review_required=true",
		"action=inspect_prune_review_before_release",
		"read_only=true",
		"approval_bypass=false",
		"approval_writing=not_performed",
		"physical_pruning=not_applied",
		"receipt_writing=not_performed",
		"target_deletion=not_applied",
		"apply_requires=prune%20--apply%20--approval%20%3Cid%3E",
		"prune_review status=review_required",
		"approvals=1",
		"unapplied_approvals=1",
		"active_approvals=1",
		"stale_approvals=0",
		"prune_approval id=approval-authored",
		"release_state=active",
		"release_blocker=false",
		"release_action=ready_for_prune_apply",
		"prune_approval_current_evidence approval=approval-authored",
		"state=current",
		"prune_approval_item approval=approval-authored",
		"soft_delete=" + softDeleteID,
		"target=gone.txt",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("prune review stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune review stderr = %q, want empty", stderr.String())
	}
	if _, err := os.Lstat(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("Lstat(target after prune review) error = %v, want target file untouched", err)
	}
	if _, err := os.Lstat(receiptPathForCLI(t, target, "approval-authored")); !os.IsNotExist(err) {
		t.Fatalf("Lstat(receipt after prune review) error = %v, want no receipt", err)
	}
	after := mustSnapshotTree(t, target)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("prune review changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"prune", "review", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("prune review json exit = %d, stderr = %q, stdout = %q, want review-needed 1", got, stderr.String(), stdout.String())
	}
	var result pruneReleaseReview
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(prune review stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if result.Schema != "supermover.prune_release_review.v1" || result.Status != string(report.PruneReviewReviewRequired) || !result.ReviewRequired || !result.ReadOnly {
		t.Fatalf("prune review json result = %+v, want read-only review-required envelope", result)
	}
	if result.SessionFilter != "" || result.LatestSessionID != "session-two" {
		t.Fatalf("prune review json scope = %+v, want no session filter and latest session evidence", result)
	}
	if result.Authorization.ApprovalBypass ||
		result.Authorization.ApprovalWriting != "not_performed" ||
		result.Authorization.PhysicalPruning != "not_applied" ||
		result.Authorization.ReceiptWriting != "not_performed" ||
		result.Authorization.TargetDeletion != "not_applied" ||
		result.Authorization.ApplyRequires != "prune --apply --approval <id>" {
		t.Fatalf("prune review json authorization = %+v, want no authorization bypass or mutations", result.Authorization)
	}
	if result.PruneReview.Summary.Approvals != 1 || result.PruneReview.Summary.UnappliedApprovals != 1 || len(result.PruneReview.Approvals) != 1 {
		t.Fatalf("prune review json prune_review = %+v, want one unapplied approval", result.PruneReview)
	}
	if result.PruneReview.Summary.ActiveApprovals != 1 || result.PruneReview.Approvals[0].ReleaseState != "active" || result.PruneReview.Approvals[0].ReleaseBlocker || len(result.PruneReview.Approvals[0].CurrentEvidence) != 1 || result.PruneReview.Approvals[0].CurrentEvidence[0].State != "current" {
		t.Fatalf("prune review json approvals = %+v summary=%+v, want active current approval evidence", result.PruneReview.Approvals, result.PruneReview.Summary)
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune review json stderr = %q, want empty", stderr.String())
	}
	afterJSON := mustSnapshotTree(t, target)
	if strings.Join(afterJSON, "\n") != strings.Join(before, "\n") {
		t.Fatalf("prune review json changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(afterJSON, "\n"))
	}
}

func TestPruneReviewCleanPlanExitsZero(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	enablePrunePolicy(t, profilePath)
	writeEmptyPublishedSessionForCLI(t, target, "session-empty")
	before := mustSnapshotTree(t, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"prune", "review", "--profile", profilePath, "--session", "session-empty", "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune review clean exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var result pruneReleaseReview
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(prune review clean stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if result.Status != string(report.PruneReviewNoPendingReview) || result.ReviewRequired || result.Action != "no_prune_review_action_required" {
		t.Fatalf("prune review clean result = %+v, want no pending review", result)
	}
	if result.SessionFilter != "session-empty" || result.LatestSessionID != "session-empty" {
		t.Fatalf("prune review clean scope = %+v, want explicit session filter", result)
	}
	if result.PruneReview.Summary.Candidates != 0 || result.PruneReview.Summary.UnappliedApprovals != 0 || result.PruneReview.Summary.Receipts != 0 {
		t.Fatalf("prune review clean summary = %+v, want no prune inventory", result.PruneReview.Summary)
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune review clean stderr = %q, want empty", stderr.String())
	}
	after := mustSnapshotTree(t, target)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("prune review clean changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

func TestStatusSurfacesStalePruneApprovalReadinessCounts(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	softDeleteID := softDeleteIDForCLI(t, target, "gone.txt")

	runner := Runner{Now: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)}
	stdout.Reset()
	stderr.Reset()
	if got := runner.Run([]string{"prune", "approve", "--profile", profilePath, "--id", "approval-stale", "--soft-delete", softDeleteID, "--reason", "reviewed stale file", "--reviewer", "cli-reviewer"}, &stdout, &stderr); got != 0 {
		t.Fatalf("prune approve exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if err := os.WriteFile(filepath.Join(target, "gone.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(changed target) error = %v, want nil", err)
	}

	stdout.Reset()
	stderr.Reset()
	got := Run([]string{"status", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("status stale prune approval exit = %d, stderr = %q, stdout = %q, want review-needed 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"status:",
		"review_required=true",
		"prune_review_status=review_required",
		"prune_review_action=inspect_prune_review_before_release",
		"prune_approvals=1",
		"prune_unapplied_approvals=1",
		"prune_active_approvals=0",
		"prune_stale_approvals=1",
		"prune_expired_approvals=0",
		"prune_consumed_approvals=0",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("status stale prune approval stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("status stale prune approval stderr = %q, want empty", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"status", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("status stale prune approval json exit = %d, stderr = %q, stdout = %q, want review-needed 1", got, stderr.String(), stdout.String())
	}
	var statusJSON status.Report
	if err := json.Unmarshal(stdout.Bytes(), &statusJSON); err != nil {
		t.Fatalf("json.Unmarshal(status stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if statusJSON.PruneReview.Status != string(report.PruneReviewReviewRequired) || statusJSON.PruneReview.Action != "inspect_prune_review_before_release" {
		t.Fatalf("status stale prune approval prune review = %+v, want review-required action", statusJSON.PruneReview)
	}
	if statusJSON.Counts.PruneApprovals != 1 || statusJSON.Counts.PruneUnappliedApprovals != 1 || statusJSON.Counts.PruneActiveApprovals != 0 || statusJSON.Counts.PruneStaleApprovals != 1 || statusJSON.Counts.PruneExpiredApprovals != 0 || statusJSON.Counts.PruneConsumedApprovals != 0 {
		t.Fatalf("status stale prune approval counts = %+v, want stale approval compact counts", statusJSON.Counts)
	}
	if stderr.Len() != 0 {
		t.Fatalf("status stale prune approval json stderr = %q, want empty", stderr.String())
	}
}

func TestPruneApproveJSONCanBeAppliedByExistingPruneApply(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	softDeleteID := softDeleteIDForCLI(t, target, "gone.txt")
	stdout.Reset()
	stderr.Reset()

	runner := Runner{Now: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)}
	got := runner.Run([]string{"prune", "approve", "--profile", profilePath, "--id", "approval-json", "--soft-delete", softDeleteID, "--reason", "reviewed stale file", "--approved-by", "json-reviewer", "--format", "json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune approve json exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	var result prune.AuthorApprovalResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("json.Unmarshal(prune approve stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if result.Schema != "supermover.prune_approval_authoring.v1" || result.ApprovalID != "approval-json" || result.Approval.ApprovedBy != "json-reviewer" || result.ApprovalWriting != "written" || result.ReceiptWriting != "not_written_by_approval_authoring" {
		t.Fatalf("prune approve json result = %+v, want authoring envelope", result)
	}
	if stderr.Len() != 0 {
		t.Fatalf("prune approve json stderr = %q, want empty", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got = runner.Run([]string{"prune", "--profile", profilePath, "--apply", "--approval", "approval-json"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("prune apply authored approval exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if _, err := os.Lstat(filepath.Join(target, "gone.txt")); !os.IsNotExist(err) {
		t.Fatalf("Lstat(pruned target) error = %v, want not exist", err)
	}
	receipt, err := control.ReadFile[control.PruneReceipt](receiptPathForCLI(t, target, "approval-json"))
	if err != nil {
		t.Fatalf("control.ReadFile(authored approval receipt) error = %v, want nil", err)
	}
	if receipt.Status != control.PruneReceiptApplied || receipt.ApprovalID != "approval-json" {
		t.Fatalf("authored approval receipt = %+v, want applied receipt", receipt)
	}
}

func TestPruneApproveRefusesActiveRetentionWindow(t *testing.T) {
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
	p := mustReadProfile(t, profilePath)
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 30
	p.DeletePolicy.AllowPhysicalPrune = true
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	softDeleteID := softDeleteIDForCLI(t, target, "gone.txt")
	before := mustSnapshotTree(t, target)
	stdout.Reset()
	stderr.Reset()

	runner := Runner{Now: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)}
	got := runner.Run([]string{"prune", "approve", "--profile", profilePath, "--id", "approval-retention", "--soft-delete", softDeleteID, "--reason", "reviewed stale file", "--reviewer", "cli-reviewer"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("prune approve retention exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "dry-run produced 1 refusal") {
		t.Fatalf("prune approve retention stderr = %q, want dry-run refusal", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("prune approve retention stdout = %q, want empty", stdout.String())
	}
	if _, err := os.Lstat(approvalPathForCLI(t, target, "approval-retention")); !os.IsNotExist(err) {
		t.Fatalf("Lstat(approval after retention refusal) error = %v, want no approval", err)
	}
	after := mustSnapshotTree(t, target)
	if strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("prune approve retention changed target tree\nbefore:\n%s\nafter:\n%s", strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
}

func TestPruneApproveRequiresExplicitSelectionAndReviewer(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing id", args: []string{"prune", "approve", "--profile", "profile.json", "--soft-delete", "sd", "--reason", "reviewed", "--reviewer", "r"}, want: "--id is required"},
		{name: "missing soft delete", args: []string{"prune", "approve", "--profile", "profile.json", "--id", "approval", "--reason", "reviewed", "--reviewer", "r"}, want: "at least one --soft-delete"},
		{name: "missing reviewer", args: []string{"prune", "approve", "--profile", "profile.json", "--id", "approval", "--soft-delete", "sd", "--reason", "reviewed"}, want: "--reviewer is required"},
		{name: "missing reason", args: []string{"prune", "approve", "--profile", "profile.json", "--id", "approval", "--soft-delete", "sd", "--reviewer", "r"}, want: "--reason is required"},
		{name: "unsafe id", args: []string{"prune", "approve", "--profile", "profile.json", "--id", "../approval", "--soft-delete", "sd", "--reason", "reviewed", "--reviewer", "r"}, want: "--id is invalid"},
		{name: "duplicate soft delete", args: []string{"prune", "approve", "--profile", "profile.json", "--id", "approval", "--soft-delete", "sd", "--soft-delete", "sd", "--reason", "reviewed", "--reviewer", "r"}, want: "duplicate --soft-delete"},
		{name: "invalid expiry", args: []string{"prune", "approve", "--profile", "profile.json", "--id", "approval", "--soft-delete", "sd", "--reason", "reviewed", "--reviewer", "r", "--expires-at", "not-time"}, want: "--expires-at must be RFC3339"},
		{name: "help as reason value", args: []string{"prune", "approve", "--profile", "profile.json", "--id", "approval", "--soft-delete", "sd", "--reason", "--help", "--reviewer", "r"}, want: "--help is only valid as the sole argument"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			got := Run(tt.args, &stdout, &stderr)

			if got != 2 {
				t.Fatalf("Run(%v) exit = %d, stderr = %q, want 2", tt.args, got, stderr.String())
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

func TestPruneRequiresProfilePolicyGate(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"prune", "--profile", profilePath, "--dry-run"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("prune default profile exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "profile delete_policy must use mode=prune with require_review=true and allow_physical_prune=true") {
		t.Fatalf("prune default profile stderr = %q, want policy gate refusal", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("prune default profile stdout = %q, want empty", stdout.String())
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

func TestHealthTextShowsNetworkTransferEvidence(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	writeNetworkTransferForCLI(t, target, control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       "session-network",
		ProfileID:       "profile-local",
		TargetID:        "local:profile-local",
		SourceDeviceID:  "sha256:abcdef0123456789",
		TargetDeviceID:  "sha256:0123456789abcdef",
		ProtocolVersion: "supermover/1",
		EncryptedTransfer: control.NetworkTransferEncryptedTLS13MTLS,
		PrivacyPolicy:   transport.DefaultPrivacyPolicy(transport.PrivacyLevel2),
		Status:          control.NetworkTransferAuthRefused,
		Stage:           "begin",
		StartedAt:       "2026-05-16T00:00:00Z",
		UpdatedAt:       "2026-05-16T00:00:01Z",
		ErrorCode:       "auth_refused",
		Error:           "receiver refused paired identity",
		Attempts: []control.NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: "2026-05-16T00:00:00Z",
			EndedAt:   "2026-05-16T00:00:01Z",
			Stage:     "begin",
			Status:    control.NetworkTransferAuthRefused,
			ErrorCode: "auth_refused",
			Error:     "receiver refused paired identity",
		}},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"health", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("health network transfer exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"healthy=false",
		"network_transfers=1",
		"network_transfer session=session-network status=auth_refused stage=begin action=review_pairing_and_profile_pins error_code=auth_refused error=receiver%20refused%20paired%20identity",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("health network transfer stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("health network transfer stderr = %q, want empty", stderr.String())
	}
}

func TestHealthTextShowsNetworkTransferArtifactSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	path := filepath.Join(control.ControlDir(target), "sessions", "session-network", "network-transfer.json")
	mustWrite(t, path, "{")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"health", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("health damaged network transfer exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"healthy=false",
		"artifact_problems=1",
		"network_transfers=0",
		"artifact source=network_transfer session=session-network",
		"path=" + encodeTextValue(path),
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("health damaged network transfer stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("health damaged network transfer stderr = %q, want empty", stderr.String())
	}
}

func TestReportJSONShowsEmptyTargetAsReviewState(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"report", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("report empty target exit = %d, stderr = %q, stdout = %q, want 1 with JSON report", got, stderr.String(), stdout.String())
	}
	var gotReport report.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(report stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Scope != report.ScopeLocalMigrationTarget || gotReport.Overall.Status != report.StatusEmpty {
		t.Fatalf("report empty target = %+v, want local empty report", gotReport.Overall)
	}
	if gotReport.Summary.ManifestCount != 0 || gotReport.LatestSession.Completeness.Status != report.CompletenessNoPublishedSession {
		t.Fatalf("report empty target summary=%+v latest=%+v, want no published session", gotReport.Summary, gotReport.LatestSession)
	}
	if stderr.Len() != 0 {
		t.Fatalf("report empty target stderr = %q, want empty", stderr.String())
	}
}

func TestReportCleanPublishedSessionReturnsSuccess(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-ok"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"report", "--profile", profilePath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("report clean target exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"status=local_target_verified",
		"session=session-ok",
		"files=1/1",
		"warnings=0",
		"soft_deletes=0",
		"artifact_problems=0",
		"pairing_issues=0",
		"pairing status=unpaired",
		"encrypted_transfer=not_configured",
		"privacy status=profile_contract_only",
		"traffic_level=2",
		"claim=bounded_reduction_only",
		"configured_reductions=",
		"overhead_status=not_applied",
		"overhead_source=profile_contract",
		"overhead_padding_bucket_bytes=65536",
		"overhead_batch_max_bytes=1048576",
		"overhead_batch_max_count=64",
		"overhead_jitter_budget_millis=250",
		"local_push=traffic_shaping_not_applied",
		"network_transfer=not_configured",
		"residual_leakage=",
		"total_bytes",
		"duration",
		"peer_ip",
		"lan_presence",
		"supermover_use",
		"profile_snapshot id=profile-session-ok",
		"privacy_status=profile_snapshot_contract",
		"privacy_traffic_level=2",
		"privacy_padding_bucket_bytes=65536",
		"privacy_batch_max_bytes=1048576",
		"privacy_batch_max_count=64",
		"privacy_jitter_budget_millis=250",
		"privacy_overhead_status=not_applied",
		"privacy_overhead_source=profile_snapshot",
		"privacy_overhead_padding_bucket_bytes=65536",
		"privacy_overhead_batch_max_bytes=1048576",
		"privacy_overhead_batch_max_count=64",
		"privacy_overhead_jitter_budget_millis=250",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report clean target stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{"transfer_ready=true", "network_ready=true", "trusted=true", "anonymous", "anonymity_claim=claimed"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Fatalf("report clean target stdout = %q, must not contain %q", stdout.String(), forbidden)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("report clean target stderr = %q, want empty", stderr.String())
	}
}

func TestReportShowsNetworkTransferPrivacyOverhead(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	p := mustReadProfile(t, profilePath)
	transfer := control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       "session-network",
		ProfileID:       p.ProfileID,
		TargetID:        p.Target.TargetID,
		SourceDeviceID:  "sha256:abcdef0123456789",
		TargetDeviceID:  "sha256:0123456789abcdef",
		ProtocolVersion: "supermover/1",
		EncryptedTransfer: control.NetworkTransferEncryptedTLS13MTLS,
		PrivacyPolicy:   transport.DefaultPrivacyPolicy(transport.PrivacyLevel2),
		PrivacyOverhead: &control.NetworkTransferPrivacyOverhead{
			FramePlainBytes:      512,
			FrameWireBytes:       640,
			PaddingBytes:         128,
			PaddedChunks:         2,
			PaddingBucketBytes:   64,
			BatchFrames:          1,
			BatchedChunks:        2,
			MaxBatchCount:        2,
			MaxBatchPlainBytes:   512,
			JitteredRequests:     3,
			JitterDelayMillis:    210,
			MaxJitterDelayMillis: 125,
			JitterBudgetMillis:   250,
		},
		Status:    control.NetworkTransferInterrupted,
		Stage:     "chunk",
		StartedAt: "2026-05-16T00:00:00Z",
		UpdatedAt: "2026-05-16T00:00:01Z",
		ErrorCode: "interrupted",
		Error:     "context canceled",
		Attempts: []control.NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: "2026-05-16T00:00:00Z",
			EndedAt:   "2026-05-16T00:00:01Z",
			Stage:     "chunk",
			Status:    control.NetworkTransferInterrupted,
			ErrorCode: "interrupted",
			Error:     "context canceled",
		}},
	}
	networkTransferPath, err := control.Path(target, control.ArtifactNetworkTransfer, transfer.SessionID)
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	if err := control.WriteFile(networkTransferPath, transfer); err != nil {
		t.Fatalf("control.WriteFile(network transfer) error = %v, want nil", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"report", "--profile", profilePath}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("report network transfer exit = %d, stderr = %q, stdout = %q, want 1 with issue", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"network_transfer session=session-network",
		"privacy_level=2",
		"privacy_frame_plain_bytes=512",
		"privacy_frame_wire_bytes=640",
		"privacy_padding_bytes=128",
		"privacy_padded_chunks=2",
		"privacy_overhead_padding_bucket_bytes=64",
		"privacy_batch_frames=1",
		"privacy_batched_chunks=2",
		"privacy_max_batch_count=2",
		"privacy_max_batch_plain_bytes=512",
		"privacy_jittered_requests=3",
		"privacy_jitter_delay_millis=210",
		"privacy_max_jitter_delay_millis=125",
		"privacy_overhead_jitter_budget_millis=250",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report network transfer stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("report network transfer stderr = %q, want empty", stderr.String())
	}
}

func TestReportJSONShowsNetworkTransferJitterOverhead(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	p := mustReadProfile(t, profilePath)
	transfer := control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       "session-network-json",
		ProfileID:       p.ProfileID,
		TargetID:        p.Target.TargetID,
		SourceDeviceID:  "sha256:abcdef0123456789",
		TargetDeviceID:  "sha256:0123456789abcdef",
		ProtocolVersion: "supermover/1",
		EncryptedTransfer: control.NetworkTransferEncryptedTLS13MTLS,
		PrivacyPolicy:   transport.DefaultPrivacyPolicy(transport.PrivacyLevel2),
		PrivacyOverhead: &control.NetworkTransferPrivacyOverhead{
			JitteredRequests:     4,
			JitterDelayMillis:    320,
			MaxJitterDelayMillis: 120,
			JitterBudgetMillis:   250,
		},
		Status:    control.NetworkTransferFailed,
		Stage:     "transport",
		StartedAt: "2026-05-16T00:00:00Z",
		UpdatedAt: "2026-05-16T00:00:01Z",
		ErrorCode: "transfer_failed",
		Error:     "receiver unavailable",
		Attempts: []control.NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: "2026-05-16T00:00:00Z",
			EndedAt:   "2026-05-16T00:00:01Z",
			Stage:     "transport",
			Status:    control.NetworkTransferFailed,
			ErrorCode: "transfer_failed",
			Error:     "receiver unavailable",
		}},
	}
	networkTransferPath, err := control.Path(target, control.ArtifactNetworkTransfer, transfer.SessionID)
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	if err := control.WriteFile(networkTransferPath, transfer); err != nil {
		t.Fatalf("control.WriteFile(network transfer) error = %v, want nil", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"report", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("report json network transfer exit = %d, stderr = %q, stdout = %q, want 1 with issue", got, stderr.String(), stdout.String())
	}
	var gotReport report.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(report stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if len(gotReport.NetworkTransfers) != 1 || gotReport.NetworkTransfers[0].Overhead == nil {
		t.Fatalf("report JSON network transfers = %#v, want one transfer with overhead", gotReport.NetworkTransfers)
	}
	overhead := gotReport.NetworkTransfers[0].Overhead
	if overhead.JitteredRequests != 4 || overhead.JitterDelayMillis != 320 || overhead.MaxJitterDelayMillis != 120 || overhead.JitterBudgetMillis != 250 {
		t.Fatalf("report JSON network transfer overhead = %+v, want persisted jitter overhead evidence", overhead)
	}
	for _, want := range []string{`"jittered_requests":4`, `"jitter_delay_millis":320`, `"max_jitter_delay_millis":120`, `"jitter_budget_millis":250`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report JSON stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("report JSON network transfer stderr = %q, want empty", stderr.String())
	}
}

func TestReportJSONShowsPairingEvidenceState(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"report", "--profile", profilePath, "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("report paired empty target exit = %d, stderr = %q, stdout = %q, want 1 for empty local target", got, stderr.String(), stdout.String())
	}
	var gotReport report.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(report stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Pairing.Status != report.PairingStatusValid || gotReport.Pairing.EncryptedTransfer != "not_configured" {
		t.Fatalf("report JSON pairing = %+v, want valid pairing evidence without configured transfer material", gotReport.Pairing)
	}
	if gotReport.Pairing.ReceiptID != p.Target.PairingReceiptID || gotReport.Pairing.TargetDeviceID != p.Target.DevicePublicKey {
		t.Fatalf("report JSON pairing = %+v, want receipt/device pins", gotReport.Pairing)
	}
	if gotReport.Privacy.Status != "profile_contract_only" || gotReport.Privacy.TrafficLevel != 2 || gotReport.Privacy.NetworkTransfer != "not_configured" {
		t.Fatalf("report JSON privacy = %+v, want profile contract level 2 without network material", gotReport.Privacy)
	}
	if gotReport.Privacy.Overhead.Status != "not_applied" || gotReport.Privacy.Overhead.Source != "profile_contract" {
		t.Fatalf("report JSON privacy overhead = %+v, want profile contract not_applied overhead", gotReport.Privacy.Overhead)
	}
	for _, want := range []string{"total_bytes", "duration", "peer_ip", "lan_presence", "supermover_use"} {
		if !containsString(gotReport.Privacy.ResidualLeakage, want) {
			t.Fatalf("report JSON privacy residual leakage = %#v, want %q", gotReport.Privacy.ResidualLeakage, want)
		}
	}
	if gotReport.Summary.PairingIssues != 0 || strings.Contains(stdout.String(), "transfer_ready") || strings.Contains(stdout.String(), "network_ready") {
		t.Fatalf("report JSON summary=%+v stdout=%q, want pairing evidence without transfer readiness", gotReport.Summary, stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("report paired stderr = %q, want empty", stderr.String())
	}
}

func TestReportTextShowsWarningsSuggestionsAndSoftDeletes(t *testing.T) {
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
	enablePrunePolicy(t, profilePath)
	warning := control.Warning{
		Version:   control.CurrentVersion,
		ID:        "session-two-001-extra-config",
		SessionID: "session-two",
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
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"report", "--profile", profilePath}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("report warnings/deletes exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"status=local_target_attention",
		"warnings=1",
		"profile_suggestions=1",
		"soft_deletes=1",
		"prune_candidates=1",
		"prune_refusals=0",
		"prune_review status=review_required",
		"approval_authoring=approval_artifact_authoring_wired",
		"physical_pruning=not_applied",
		"apply=existing_approval_apply_wired",
		"prune_candidate",
		"approval_writing=not_written_by_dry_run",
		"warning id=session-two-001-extra-config",
		"code=needs_profile_config",
		"profile_suggestion warning=session-two-001-extra-config",
		"patch=include.needs_extra=true",
		"soft_delete id=session-two-del_",
		"profile=profile-local",
		"target_id=local:profile-local",
		"root=root",
		"previous_session=session-one",
		"previous_manifest=manifest-session-one",
		"detected_at=",
		"source=gone.txt",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("report stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("report stderr = %q, want empty", stderr.String())
	}
}

func TestReportShowsRecoveryIssuesWithoutMutatingState(t *testing.T) {
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
	got := Run([]string{"report", "--profile", profilePath}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("report recovery issue exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "status=local_target_unhealthy") || !strings.Contains(stdout.String(), "recovery session=session-recover") {
		t.Fatalf("report recovery issue stdout = %q, want unhealthy recovery issue", stdout.String())
	}
	record, err := transaction.ReadSessionRecord(layout.RecordPath("session-recover"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(session-recover) error = %v, want nil", err)
	}
	if record.State != transaction.StateStaged {
		t.Fatalf("report mutated session state to %q, want %q", record.State, transaction.StateStaged)
	}
}

func TestRecoverCompletesStateAfterReceiptCrash(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	mustWrite(t, filepath.Join(source, "file.txt"), "payload")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if got := Run([]string{"push", "--profile", profilePath, "--session", "session-crash"}, &stdout, &stderr); got != 0 {
		t.Fatalf("push exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	layout := transaction.NewLayout(control.ControlDir(target))
	record, err := transaction.ReadSessionRecord(layout.RecordPath("session-crash"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(session-crash) error = %v, want nil", err)
	}
	staged, err := record.WithState(transaction.StateStaged, record.UpdatedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("SessionRecord.WithState(staged) error = %v, want nil", err)
	}
	if err := layout.WriteSessionRecord(staged); err != nil {
		t.Fatalf("Layout.WriteSessionRecord(staged) error = %v, want nil", err)
	}
	stdout.Reset()
	stderr.Reset()

	got := Run([]string{"recover", "--profile", profilePath, "--session", "session-crash", "--dry-run"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("recover dry-run receipt crash exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "status=would_complete_state") {
		t.Fatalf("recover dry-run receipt crash stdout = %q, want would_complete_state", stdout.String())
	}
	record, err = transaction.ReadSessionRecord(layout.RecordPath("session-crash"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(session-crash after dry-run) error = %v, want nil", err)
	}
	if record.State != transaction.StateStaged {
		t.Fatalf("recover dry-run mutated state to %q, want staged", record.State)
	}
	stdout.Reset()
	stderr.Reset()

	got = Run([]string{"recover", "--profile", profilePath, "--session", "session-crash"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("recover receipt crash exit = %d, stderr = %q, stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "status=completed_state") {
		t.Fatalf("recover receipt crash stdout = %q, want completed_state", stdout.String())
	}
	record, err = transaction.ReadSessionRecord(layout.RecordPath("session-crash"))
	if err != nil {
		t.Fatalf("transaction.ReadSessionRecord(session-crash final) error = %v, want nil", err)
	}
	if record.State != transaction.StatePublished {
		t.Fatalf("recover receipt crash state = %q, want published", record.State)
	}
}

func TestReportSessionCorruptManifestEmitsStructuredReport(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, "bad")
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
	}
	if err := control.WriteFile(receiptPath, control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        "bad",
		ProfileID: "profile-local",
		TargetID:  "local:profile-local",
		StartedAt: "2026-05-16T00:00:00Z",
		Status:    "published",
	}); err != nil {
		t.Fatalf("control.WriteFile(%q, receipt) error = %v, want nil", receiptPath, err)
	}
	manifestPath, err := control.Path(target, control.ArtifactManifest, "bad")
	if err != nil {
		t.Fatalf("control.Path(manifest) error = %v, want nil", err)
	}
	mustWrite(t, manifestPath, "{")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"report", "--profile", profilePath, "--session", "bad", "--format", "json"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("report corrupt manifest exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	var gotReport report.Report
	if err := json.Unmarshal(stdout.Bytes(), &gotReport); err != nil {
		t.Fatalf("json.Unmarshal(report stdout) error = %v, stdout = %q, want nil", err, stdout.String())
	}
	if gotReport.Summary.ArtifactProblems != 1 || len(gotReport.ArtifactProblems) != 1 || gotReport.ArtifactProblems[0].SessionID != "bad" {
		t.Fatalf("report corrupt manifest = %+v artifact_problems=%#v, want structured artifact problem", gotReport.Summary, gotReport.ArtifactProblems)
	}
	if stderr.Len() != 0 {
		t.Fatalf("report corrupt manifest stderr = %q, want empty", stderr.String())
	}
}

func TestReportTextShowsInvalidHealthRecordDetails(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	badPath := filepath.Join(control.ControlDir(target), "sessions", "bad", "session.json")
	mustWrite(t, badPath, "{")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Run([]string{"report", "--profile", profilePath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("report invalid record exit = %d, stderr = %q, stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "invalid_records=1") ||
		!strings.Contains(stdout.String(), "invalid_record session=bad") ||
		!strings.Contains(stdout.String(), "path="+encodeTextValue(badPath)) ||
		!strings.Contains(stdout.String(), "error=transaction%20validation%20failed") {
		t.Fatalf("report invalid record stdout = %q, want itemized invalid record", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("report invalid record stderr = %q, want empty", stderr.String())
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

func setNetworkMaterial(p *profile.Profile, dir string) {
	p.Network = &profile.NetworkConfig{
		ReceiverURL: "https://127.0.0.1:9443",
		LocalTLSIdentity: profile.TLSIdentityRef{
			CertificatePath: filepath.Join(dir, "source.crt"),
			PrivateKeyPath:  filepath.Join(dir, "source.key"),
		},
	}
}

func enablePrunePolicy(t *testing.T, path string) {
	t.Helper()
	p := mustReadProfile(t, path)
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	if err := profile.WriteFile(path, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", path, err)
	}
}

func pruneApprovalForCLI(t *testing.T, profilePath, target, approvalID, softDeleteID string) control.PruneApproval {
	t.Helper()
	p := mustReadProfile(t, profilePath)
	softDeletePath, err := control.Path(target, control.ArtifactSoftDelete, softDeleteID)
	if err != nil {
		t.Fatalf("control.Path(soft delete %q) error = %v, want nil", softDeleteID, err)
	}
	record, err := control.ReadFile[control.SoftDelete](softDeletePath)
	if err != nil {
		t.Fatalf("control.ReadFile(%q, soft delete) error = %v, want nil", softDeletePath, err)
	}
	snapshotID := "profile-" + approvalID
	snapshot := writePruneApprovalProfileSnapshotForCLI(t, target, snapshotID, p)
	snapshotPath, err := control.Path(target, control.ArtifactProfileSnapshot, snapshotID)
	if err != nil {
		t.Fatalf("control.Path(profile snapshot %q) error = %v, want nil", snapshotID, err)
	}
	snapshotDigest, err := prune.ProfileSnapshotDigest(snapshot.Profile)
	if err != nil {
		t.Fatalf("prune.ProfileSnapshotDigest(%q) error = %v, want nil", snapshotPath, err)
	}
	policy := p.DeletePolicy
	item := control.PruneApprovalItem{
		SoftDeleteID:       record.ID,
		SoftDeleteRef:      "deleted/" + record.ID + ".json",
		DetectedSessionID:  record.SessionID,
		PreviousSessionID:  record.PreviousSessionID,
		PreviousManifestID: record.PreviousManifestID,
		RootID:             record.RootID,
		SourcePath:         record.SourcePath,
		TargetPath:         record.TargetPath,
		Kind:               record.Kind,
		Size:               record.Size,
		Digest:             record.Digest,
		DetectedAt:         record.DetectedAt,
	}
	approval := control.PruneApproval{
		Version:               control.CurrentVersion,
		ID:                    approvalID,
		ProfileID:             p.ProfileID,
		TargetID:              p.Target.TargetID,
		RootID:                record.RootID,
		CreatedAt:             "2026-05-18T09:00:00Z",
		ApprovedBy:            "cli-test",
		ApprovedAt:            "2026-05-18T09:30:00Z",
		ReviewTool:            "unit-test",
		ProfileSnapshotID:     snapshotID,
		ProfileSnapshotDigest: snapshotDigest,
		ProfileDeletePolicy: control.PruneDeletePolicy{
			Mode:               string(policy.Mode),
			RequireReview:      policy.RequireReview,
			RetentionDays:      policy.RetentionDays,
			AllowPhysicalPrune: policy.AllowPhysicalPrune,
		},
		Items:          []control.PruneApprovalItem{item},
		ExpiresAt:      "2026-05-19T00:00:00Z",
		Status:         "approved",
		ApprovalReason: "reviewed",
	}
	approval.ApprovalScopeDigest = prune.ApprovalScopeDigest(approval.ProfileID, approval.TargetID, approval.RootID, approval.ProfileSnapshotID, approval.ProfileSnapshotDigest, policy, approval.Items)
	return approval
}

func writePruneApprovalProfileSnapshotForCLI(t *testing.T, target, snapshotID string, p profile.Profile) control.ProfileSnapshot {
	t.Helper()
	payload, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(profile) error = %v, want nil", err)
	}
	payload = append(payload, '\n')
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         snapshotID,
		ProfileID:  p.ProfileID,
		CapturedAt: "2026-05-18T09:00:00Z",
		Profile:    payload,
	}
	path, err := control.Path(target, control.ArtifactProfileSnapshot, snapshotID)
	if err != nil {
		t.Fatalf("control.Path(profile snapshot %q) error = %v, want nil", snapshotID, err)
	}
	if err := control.WriteNewFile(path, snapshot); err != nil {
		t.Fatalf("control.WriteNewFile(%q, profile snapshot) error = %v, want nil", path, err)
	}
	return snapshot
}

func softDeleteIDForCLI(t *testing.T, target, targetPath string) string {
	t.Helper()
	artifacts, err := verify.LoadArtifactsForScope(target, "profile-local", "local:profile-local")
	if err != nil {
		t.Fatalf("verify.LoadArtifactsForScope(%q) error = %v, want nil", target, err)
	}
	for _, record := range artifacts.SoftDeletes {
		if record.TargetPath == targetPath {
			return record.ID
		}
	}
	t.Fatalf("soft delete for target %q not found in %+v", targetPath, artifacts.SoftDeletes)
	return ""
}

func writePruneApprovalForCLI(t *testing.T, target string, approval control.PruneApproval) {
	t.Helper()
	path := approvalPathForCLI(t, target, approval.ID)
	if err := control.WriteNewFile(path, approval); err != nil {
		t.Fatalf("control.WriteNewFile(%q, prune approval) error = %v, want nil", path, err)
	}
}

func approvalPathForCLI(t *testing.T, target, id string) string {
	t.Helper()
	path, err := control.Path(target, control.ArtifactPruneApproval, id)
	if err != nil {
		t.Fatalf("control.Path(prune approval %q) error = %v, want nil", id, err)
	}
	return path
}

func receiptPathForCLI(t *testing.T, target, id string) string {
	t.Helper()
	path, err := control.Path(target, control.ArtifactPruneReceipt, id)
	if err != nil {
		t.Fatalf("control.Path(prune receipt %q) error = %v, want nil", id, err)
	}
	return path
}

func writeEmptyPublishedSessionForCLI(t *testing.T, target string, sessionID string) {
	t.Helper()
	manifestPath, err := control.Path(target, control.ArtifactManifest, sessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest %q) error = %v, want nil", sessionID, err)
	}
	err = control.WriteFile(manifestPath, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			cliExpectedManifestEntry(),
		},
	})
	if err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", manifestPath, err)
	}
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt %q) error = %v, want nil", sessionID, err)
	}
	err = control.WriteFile(receiptPath, control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: "profile-local",
		TargetID:  "local:profile-local",
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    "published",
	})
	if err != nil {
		t.Fatalf("control.WriteFile(%q, receipt) error = %v, want nil", receiptPath, err)
	}
}

func cliMissingFileDrift(id string) control.TargetDrift {
	expectedPayload := []byte("aaaaaaa")
	expected := control.TargetDriftExpectedState{
		SessionID:  "session-one",
		ManifestID: "manifest-session-one",
		Kind:       "file",
		Path:       "file.txt",
		Digest:     digestForCLI(expectedPayload),
		ModTime:    "2026-05-18T00:00:00Z",
	}
	expected.SetSizeEvidence(int64(len(expectedPayload)))
	expected.SetModeEvidence(0o644)
	missing := false
	return control.TargetDrift{
		Version:    control.CurrentVersion,
		ID:         id,
		SessionID:  "session-one",
		ProfileID:  "profile-local",
		TargetID:   "local:profile-local",
		RootID:     "root",
		Path:       "file.txt",
		DetectedAt: "2026-05-20T00:00:00Z",
		Change:     "missing",
		Expected:   expected,
		Observed: control.TargetDriftObservedState{
			Present: &missing,
			Kind:    "missing",
			Path:    "file.txt",
		},
		ReviewState: "needs_review",
		Evidence:    []string{"target path is missing"},
	}
}

func writePublishedSessionForReconcileCLI(t *testing.T, target string, sessionID string) {
	t.Helper()
	manifestPath, err := control.Path(target, control.ArtifactManifest, sessionID)
	if err != nil {
		t.Fatalf("control.Path(manifest %q) error = %v, want nil", sessionID, err)
	}
	if err := control.WriteFile(manifestPath, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{
			cliExpectedManifestEntry(),
		},
	}); err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", manifestPath, err)
	}
	receiptPath, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt %q) error = %v, want nil", sessionID, err)
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

func writeTargetDriftForReconcileCLI(t *testing.T, target string, drift control.TargetDrift) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactTargetDrift, drift.ID)
	if err != nil {
		t.Fatalf("control.Path(target drift %q) error = %v, want nil", drift.ID, err)
	}
	if err := control.WriteFile(path, drift); err != nil {
		t.Fatalf("control.WriteFile(%q, drift) error = %v, want nil", path, err)
	}
}

func assertReviewBoundaryForCLI(t *testing.T, review reconcile.Review, name string, status string, applyCapable bool) {
	t.Helper()
	var boundary reconcile.RepairBoundary
	found := false
	for _, candidate := range review.Boundaries {
		if candidate.Name == name {
			boundary = candidate
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("boundary %q not found in %+v", name, review.Boundaries)
	}
	if boundary.Status != status || boundary.ApplyCapable != applyCapable {
		t.Fatalf("boundary %q = %+v, want status=%s apply_capable=%t", name, boundary, status, applyCapable)
	}
}

func readTargetDriftForReconcileCLI(t *testing.T, target string, id string) control.TargetDrift {
	t.Helper()
	path, err := control.Path(target, control.ArtifactTargetDrift, id)
	if err != nil {
		t.Fatalf("control.Path(target drift %q) error = %v, want nil", id, err)
	}
	drift, err := control.ReadFile[control.TargetDrift](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	return drift
}

func cliExpectedManifestEntry() control.ManifestEntry {
	expectedPayload := []byte("aaaaaaa")
	entry := control.ManifestEntry{
		Path:       "file.txt",
		TargetPath: "file.txt",
		Kind:       "file",
		Digest:     testDigest(expectedPayload),
		ModTime:    "2026-05-18T00:00:00Z",
	}
	entry.SetSizeEvidence(int64(len(expectedPayload)))
	entry.SetModeEvidence(0o644)
	return entry
}

func mustReadProfile(t *testing.T, path string) profile.Profile {
	t.Helper()
	p, err := profile.ReadFile(path)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", path, err)
	}
	return p
}

func waitServePairingReady(t *testing.T, ready <-chan pairserve.ReadyInfo, stderr *bytes.Buffer) pairserve.ReadyInfo {
	t.Helper()
	select {
	case info := <-ready:
		return info
	case <-time.After(2 * time.Second):
		t.Fatalf("serve did not report pairing ready; stderr=%q", stderr.String())
	}
	return pairserve.ReadyInfo{}
}

func waitDaemonReadyState(t *testing.T, ready <-chan agentdaemon.State, stdout fmt.Stringer, stderr fmt.Stringer) agentdaemon.State {
	t.Helper()
	select {
	case state := <-ready:
		return state
	case <-time.After(2 * time.Second):
		t.Fatalf("daemon run did not report ready; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	return agentdaemon.State{}
}

func waitForFile(t *testing.T, path string, timeout time.Duration, stdout *bytes.Buffer, stderr *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("file %q was not published before timeout; stdout=%q stderr=%q", path, stdout.String(), stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForTargetDriftByPath(t *testing.T, target string, relPath string, change string, timeout time.Duration, stdout *bytes.Buffer, stderr *bytes.Buffer) control.TargetDrift {
	t.Helper()
	driftDir := filepath.Join(target, control.DirName, "drift")
	deadline := time.Now().Add(timeout)
	var lastIDs []string
	var lastErr error
	for {
		entries, err := os.ReadDir(driftDir)
		if err == nil {
			lastIDs = lastIDs[:0]
			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() || !strings.HasSuffix(name, ".json") {
					continue
				}
				id := strings.TrimSuffix(name, ".json")
				lastIDs = append(lastIDs, id)
				drift, readErr := readTargetDriftForReconcileCLINoFatal(target, id)
				if readErr != nil {
					lastErr = readErr
					continue
				}
				if drift.Path == relPath && drift.Change == change {
					return drift
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			lastErr = err
		}
		if time.Now().After(deadline) {
			t.Fatalf("target drift path=%q change=%q not recorded before timeout; ids=%v err=%v stdout=%q stderr=%q", relPath, change, lastIDs, lastErr, stdout.String(), stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForLatestReconcileReceipt(t *testing.T, target string, status string, timeout time.Duration, stdout *bytes.Buffer, stderr *bytes.Buffer) reconcile.Receipt {
	t.Helper()
	dir := filepath.Join(target, control.DirName, "reconcile", "receipts")
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		entries, err := os.ReadDir(dir)
		if err == nil {
			var receipts []reconcile.Receipt
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
					continue
				}
				path := filepath.Join(dir, entry.Name())
				receipt, readErr := control.ReadFile[reconcile.Receipt](path)
				if readErr != nil {
					lastErr = readErr
					continue
				}
				if receipt.Status == status {
					receipts = append(receipts, receipt)
				}
			}
			if len(receipts) > 0 {
				sort.Slice(receipts, func(i, j int) bool {
					if receipts[i].StartedAt == receipts[j].StartedAt {
						return receipts[i].ID < receipts[j].ID
					}
					return receipts[i].StartedAt < receipts[j].StartedAt
				})
				return receipts[len(receipts)-1]
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			lastErr = err
		}
		if time.Now().After(deadline) {
			t.Fatalf("reconcile receipt status=%q not written before timeout; err=%v stdout=%q stderr=%q", status, lastErr, stdout.String(), stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForDaemonLifecycleEventType(t *testing.T, target string, eventType string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastTypes []string
	var lastErr error
	for {
		events, err := agentdaemon.ListLifecycleEvents(target)
		if err == nil {
			lastTypes = lifecycleEventTypes(events)
			for _, got := range lastTypes {
				if got == eventType {
					return
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			lastErr = err
		}
		if time.Now().After(deadline) {
			t.Fatalf("daemon lifecycle event %q not observed before timeout; events=%v err=%v", eventType, lastTypes, lastErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func readTargetDriftForReconcileCLINoFatal(target string, id string) (control.TargetDrift, error) {
	path, err := control.Path(target, control.ArtifactTargetDrift, id)
	if err != nil {
		return control.TargetDrift{}, err
	}
	return control.ReadFile[control.TargetDrift](path)
}

func waitForPublishedRunSessions(t *testing.T, scheduler *incrementalsync.Scheduler, scope incrementalsync.Scope, timeout time.Duration, sessions ...string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastRuns []incrementalsync.RunResult
	var lastProblems []incrementalsync.ArtifactProblem
	var lastErr error
	for {
		lastRuns, lastProblems, lastErr = scheduler.RunResults(scope)
		if lastErr == nil && len(lastProblems) == 0 && runResultsContainPublishedSessions(lastRuns, sessions...) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("RunResults() sessions = %#v problems=%#v err=%v, want published %v", runResultSessions(lastRuns), lastProblems, lastErr, sessions)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func runResultsContainPublishedSessions(runs []incrementalsync.RunResult, sessions ...string) bool {
	seen := map[string]bool{}
	for _, run := range runs {
		if run.Status == incrementalsync.RunStatusPublished {
			seen[run.SessionID] = true
		}
	}
	for _, session := range sessions {
		if !seen[session] {
			return false
		}
	}
	return true
}

func runResultSessions(runs []incrementalsync.RunResult) []string {
	sessions := make([]string, 0, len(runs))
	for _, run := range runs {
		sessions = append(sessions, run.SessionID+":"+run.Status)
	}
	return sessions
}

func waitServeReceiverReady(t *testing.T, ready <-chan receiverserve.ReadyInfo, stderr *bytes.Buffer) receiverserve.ReadyInfo {
	t.Helper()
	select {
	case info := <-ready:
		return info
	case <-time.After(2 * time.Second):
		stderrText := ""
		if stderr != nil {
			stderrText = stderr.String()
		}
		t.Fatalf("serve receiver did not report ready; stderr=%q", stderrText)
	}
	return receiverserve.ReadyInfo{}
}

func httptestPairingServer(t *testing.T, bootstrap pairing.Bootstrap) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/pairing" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(bootstrap); err != nil {
			t.Fatalf("Encode(bootstrap) error = %v, want nil", err)
		}
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func cliTLSNow() time.Time {
	return time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
}

func reserveTCPAddress(t *testing.T) string {
	t.Helper()
	listener := listenOnReservedTCPAddress(t)
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("reserve listener Close error = %v, want nil", err)
	}
	return address
}

func listenOnReservedTCPAddress(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(reserve address) error = %v, want nil", err)
	}
	return listener
}

func listenOnTCPAddress(t *testing.T, address string) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("net.Listen(%q) error = %v, want nil", address, err)
	}
	return listener
}

func networkConfigForCLI(t *testing.T, cert tls.Certificate, address string) *profile.NetworkConfig {
	t.Helper()
	certPath, keyPath := writeCertificateFilesForCLI(t, cert)
	return &profile.NetworkConfig{
		ReceiverURL: "https://" + address,
		LocalTLSIdentity: profile.TLSIdentityRef{
			CertificatePath: certPath,
			PrivateKeyPath:  keyPath,
		},
	}
}

func writeCertificateFilesForCLI(t *testing.T, cert tls.Certificate) (string, string) {
	t.Helper()
	dir := t.TempDir()
	certPath := filepath.Join(dir, "identity.crt")
	keyPath := filepath.Join(dir, "identity.key")
	if len(cert.Certificate) == 0 {
		t.Fatal("test certificate chain is empty")
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey error = %v, want nil", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("os.WriteFile(cert) error = %v, want nil", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("os.WriteFile(key) error = %v, want nil", err)
	}
	return certPath, keyPath
}

func pinnedCLIClient(t *testing.T, sourceCert tls.Certificate, peer transport.AuthenticatedPeer) *http.Client {
	t.Helper()
	clientTLS, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{sourceCert},
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
		Time:           cliTLSNow,
	})
	if err != nil {
		t.Fatalf("ClientTLSConfig error = %v, want nil", err)
	}
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: clientTLS},
		Timeout:   2 * time.Second,
	}
}

func cliAuthenticatedPeerForCerts(t *testing.T, sourceCert, targetCert tls.Certificate) transport.AuthenticatedPeer {
	t.Helper()
	return transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: certDeviceIDForCLI(t, sourceCert),
		TargetDeviceID: certDeviceIDForCLI(t, targetCert),
	}
}

func certDeviceIDForCLI(t *testing.T, cert tls.Certificate) string {
	t.Helper()
	id, err := transport.LeafSPKIDeviceID(cert.Leaf)
	if err != nil {
		t.Fatalf("LeafSPKIDeviceID(%q) error = %v, want nil", cert.Leaf.Subject.CommonName, err)
	}
	return id
}

func newCLITestCertificate(t *testing.T, commonName string, notBefore, notAfter time.Time) tls.Certificate {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey error = %v, want nil", err)
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		t.Fatalf("rand.Int(serial) error = %v, want nil", err)
	}
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{commonName},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey, privateKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate error = %v, want nil", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("x509.ParseCertificate error = %v, want nil", err)
	}
	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  privateKey,
		Leaf:        leaf,
	}
}

func validCLIBeginRequest(peer transport.AuthenticatedPeer, data []byte) protocol.BeginSessionRequest {
	return protocol.BeginSessionRequest{
		ProtocolVersion: protocol.Version,
		SessionID:       "session-serve-cli",
		ProfileID:       peer.ProfileID,
		TargetID:        peer.TargetID,
		SourceDeviceID:  peer.SourceDeviceID,
		TargetDeviceID:  peer.TargetDeviceID,
		RootID:          "root1",
		CreatedAt:       cliTLSNow(),
		Manifest: protocol.TransferManifest{
			ID: "manifest-serve-cli",
			Entries: []protocol.ManifestEntry{
				{
					Path:    "a.txt",
					Kind:    protocol.FileKindFile,
					Mode:    0o600,
					Size:    int64(len(data)),
					Digest:  digestForCLI(data),
					ModTime: cliTLSNow(),
				},
			},
		},
		PrivacyPolicy: transport.DefaultPrivacyPolicy(transport.PrivacyLevel2),
	}
}

func newCLIBeginRequest(t *testing.T, url string, begin protocol.BeginSessionRequest) *http.Request {
	t.Helper()
	body, err := json.Marshal(begin)
	if err != nil {
		t.Fatalf("json.Marshal(begin) error = %v, want nil", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("http.NewRequest(begin) error = %v, want nil", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req
}

func digestForCLI(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func writePairingReceiptForCLI(t *testing.T, target string, p profile.Profile, mutate ...func(*control.PairingReceipt)) {
	t.Helper()
	receipt := control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               p.Target.PairingReceiptID,
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   "sha256:abcdef0123456789",
		TargetDeviceID:   p.Target.DevicePublicKey,
		DevicePublicKey:  p.Target.DevicePublicKey,
		Method:           "sas",
		VerifiedAt:       p.Target.PairedAt,
		VerificationHash: "sha256:abcdef0123456789",
		ProtocolVersion:  protocol.Version,
	}
	for _, fn := range mutate {
		fn(&receipt)
	}
	path, err := control.Path(target, control.ArtifactPairingReceipt, receipt.ID)
	if err != nil {
		t.Fatalf("control.Path(pairing receipt) error = %v, want nil", err)
	}
	if err := control.WriteFile(path, receipt); err != nil {
		t.Fatalf("control.WriteFile(pairing receipt) error = %v, want nil", err)
	}
}

func writeNetworkTransferForCLI(t *testing.T, target string, transfer control.NetworkTransfer) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactNetworkTransfer, transfer.SessionID)
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	if err := control.WriteFile(path, transfer); err != nil {
		t.Fatalf("control.WriteFile(network transfer) error = %v, want nil", err)
	}
}

func readNetworkTransferForCLI(t *testing.T, target, sessionID string) control.NetworkTransfer {
	t.Helper()
	path, err := control.Path(target, control.ArtifactNetworkTransfer, sessionID)
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	transfer, err := control.ReadFile[control.NetworkTransfer](path)
	if err != nil {
		t.Fatalf("control.ReadFile(network transfer) error = %v, want nil", err)
	}
	return transfer
}

func seedPartialReceiverSessionForCLI(t *testing.T, p profile.Profile, sourceCert tls.Certificate, peer transport.AuthenticatedPeer, receiverAddress string, sessionID string) int64 {
	t.Helper()
	scanned, err := scan.Scan(p.Roots[0].Path)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", p.Roots[0].Path, err)
	}
	trust, err := pairing.ValidateProfileTrust(p)
	if err != nil {
		t.Fatalf("pairing.ValidateProfileTrust error = %v, want nil", err)
	}
	req := protocolclient.TransferRequest{
		SourceRoot:     p.Roots[0].Path,
		Scan:           scanned,
		SessionID:      sessionID,
		ManifestID:     sessionID,
		ProfileID:      p.ProfileID,
		TargetID:       p.Target.TargetID,
		SourceDeviceID: trust.Receipt.SourceDeviceID,
		TargetDeviceID: trust.TargetDeviceID,
		PrivacyPolicy:  privacyPolicyForCLIProfile(p),
		RootID:         p.Roots[0].ID,
		CreatedAt:      cliTLSNow(),
	}
	client := pinnedCLIClient(t, sourceCert, peer)
	interrupting := &interruptAfterFirstChunkBatchDoer{next: client}
	_, err = protocolclient.Client{
		BaseURL: "https://" + receiverAddress,
		Doer:    interrupting,
		Now:     cliTLSNow,
	}.Run(context.Background(), req)
	if err == nil {
		t.Fatalf("protocolclient.Client.Run(partial seed) error = nil, want interrupted upload")
	}
	if !interrupting.interrupted.Load() {
		t.Fatalf("partial seed did not interrupt after a chunk batch")
	}
	writePriorNetworkTransferForCLI(t, client, receiverAddress, req)
	status := readReceiverStatusForCLI(t, client, receiverAddress, sessionID)
	for _, file := range status.Files {
		if file.Path == "large.bin" {
			return file.CommittedSize
		}
	}
	t.Fatalf("receiver status files = %+v, want large.bin", status.Files)
	return 0
}

func writePriorNetworkTransferForCLI(t *testing.T, client *http.Client, receiverAddress string, req protocolclient.TransferRequest) {
	t.Helper()
	started := cliTLSNow().UTC().Format(time.RFC3339Nano)
	ended := cliTLSNow().Add(time.Minute).UTC().Format(time.RFC3339Nano)
	transfer := control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       req.SessionID,
		ProfileID:       req.ProfileID,
		TargetID:        req.TargetID,
		SourceDeviceID:  req.SourceDeviceID,
		TargetDeviceID:  req.TargetDeviceID,
		ProtocolVersion: protocol.Version,
		PrivacyPolicy:   req.PrivacyPolicy,
		Status:          control.NetworkTransferFailed,
		Stage:           "transport",
		ErrorCode:       "transfer_failed",
		StartedAt:       started,
		UpdatedAt:       ended,
		PrivacyOverhead: &control.NetworkTransferPrivacyOverhead{
			FramePlainBytes:    128,
			FrameWireBytes:     192,
			PaddingBytes:       64,
			PaddedChunks:       1,
			PaddingBucketBytes: req.PrivacyPolicy.PaddingBucket,
			BatchFrames:        1,
			BatchedChunks:      1,
			MaxBatchCount:      req.PrivacyPolicy.BatchMaxCount,
			MaxBatchPlainBytes: 128,
			JitteredRequests:   1,
			JitterBudgetMillis: req.PrivacyPolicy.JitterBudget,
		},
		Attempts: []control.NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: started,
			EndedAt:   ended,
			Stage:     "transport",
			Status:    control.NetworkTransferFailed,
			ErrorCode: "transfer_failed",
		}},
	}
	var doc bytes.Buffer
	if err := control.Write(&doc, transfer); err != nil {
		t.Fatalf("control.Write(network transfer seed) error = %v, want nil", err)
	}
	body := protocol.NetworkTransferArtifactRequest{
		SessionID: req.SessionID,
		Document:  doc.Bytes(),
	}
	var payload bytes.Buffer
	if err := json.NewEncoder(&payload).Encode(body); err != nil {
		t.Fatalf("json.Encode(network transfer request) error = %v, want nil", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, "https://"+receiverAddress+"/v1/sessions/"+req.SessionID+"/artifacts/network-transfer", &payload)
	if err != nil {
		t.Fatalf("http.NewRequest(network transfer artifact) error = %v, want nil", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("POST network transfer artifact error = %v, want nil", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST network transfer artifact status = %d body=%q, want 200", resp.StatusCode, string(data))
	}
}

type interruptAfterFirstChunkBatchDoer struct {
	next        protocolclient.Doer
	batches     atomic.Int64
	interrupted atomic.Bool
}

func (d *interruptAfterFirstChunkBatchDoer) Do(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodPost && req.URL.Path == "/v1/chunk-batches" && d.batches.Add(1) > 1 {
		d.interrupted.Store(true)
		return nil, context.Canceled
	}
	return d.next.Do(req)
}

type interruptingTLSReceiverForCLI struct {
	handler    http.Handler
	tlsConfig  *tls.Config
	failChunks atomic.Bool
	failed     atomic.Bool
}

func newInterruptingTLSReceiverForCLI(t *testing.T, p profile.Profile, targetCert tls.Certificate) *interruptingTLSReceiverForCLI {
	t.Helper()
	receiverTLS, err := receiver.NewTLSReceiverFromProfile(receiver.TLSReceiverOptions{
		Profile:      p,
		Certificates: []tls.Certificate{targetCert},
		Now:          cliTLSNow,
	})
	if err != nil {
		t.Fatalf("NewTLSReceiverFromProfile error = %v, want nil", err)
	}
	server := &interruptingTLSReceiverForCLI{
		handler:   receiverTLS.Handler,
		tlsConfig: receiverTLS.TLSConfig,
	}
	server.failChunks.Store(true)
	return server
}

func (s *interruptingTLSReceiverForCLI) Serve(ctx context.Context, listener net.Listener) error {
	if ctx == nil {
		ctx = context.Background()
	}
	httpServer := &http.Server{
		Handler:  http.HandlerFunc(s.serveHTTP),
		ErrorLog: log.New(io.Discard, "", 0),
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(tls.NewListener(listener, s.tlsConfig))
	}()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			_ = httpServer.Close()
			return err
		}
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *interruptingTLSReceiverForCLI) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if s.failChunks.Load() && !s.failed.Load() &&
		r.Method == http.MethodPost &&
		(r.URL.Path == "/v1/chunks" || r.URL.Path == "/v1/chunk-batches") {
		rec := httptest.NewRecorder()
		s.handler.ServeHTTP(rec, r)
		if rec.Code == http.StatusAccepted && s.failed.CompareAndSwap(false, true) {
			panic(http.ErrAbortHandler)
		}
		copyRecordedResponse(w, rec)
		return
	}
	s.handler.ServeHTTP(w, r)
}

func copyRecordedResponse(w http.ResponseWriter, rec *httptest.ResponseRecorder) {
	for key, values := range rec.Header() {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	status := rec.Code
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(rec.Body.Bytes())
}

func readReceiverStatusForCLI(t *testing.T, client *http.Client, receiverAddress string, sessionID string) protocol.SessionStatusResponse {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "https://"+receiverAddress+"/v1/sessions/"+sessionID+"/status", nil)
	if err != nil {
		t.Fatalf("http.NewRequest(status) error = %v, want nil", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET receiver status error = %v, want nil", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET receiver status = %d body=%q, want 200", resp.StatusCode, string(body))
	}
	var status protocol.SessionStatusResponse
	decoder := json.NewDecoder(resp.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&status); err != nil {
		t.Fatalf("decode receiver status error = %v, want nil", err)
	}
	return status
}

func privacyPolicyForCLIProfile(p profile.Profile) transport.PrivacyPolicy {
	return transport.PrivacyPolicy{
		Level:            transport.PrivacyLevel(p.PrivacyPolicy.TrafficLevel),
		PaddingBucket:    p.PrivacyPolicy.PaddingBucketBytes,
		BatchMaxBytes:    p.PrivacyPolicy.BatchMaxBytes,
		BatchMaxCount:    p.PrivacyPolicy.BatchMaxCount,
		JitterBudget:     p.PrivacyPolicy.JitterBudgetMillis,
		DiscoveryLowInfo: p.PrivacyPolicy.DiscoveryLowInfo,
	}
}

func fileSizeForCLI(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", path, err)
	}
	return info.Size()
}

func writePatternFileForCLI(t *testing.T, path string, size int) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("os.OpenFile(%q) error = %v, want nil", path, err)
	}
	defer file.Close()
	pattern := []byte("supermover-cli-network-resume-test-pattern\n")
	written := 0
	for written < size {
		n := min(len(pattern), size-written)
		if _, err := file.Write(pattern[:n]); err != nil {
			t.Fatalf("file.Write(%q) error = %v, want nil", path, err)
		}
		written += n
	}
}

func hashFileForCLI(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open(%q) error = %v, want nil", path, err)
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		t.Fatalf("io.Copy(hash) error = %v, want nil", err)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
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

func mustWriteWithModTime(t *testing.T, path string, content string, mode os.FileMode, modTime string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
	ts, err := time.Parse(time.RFC3339Nano, modTime)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v, want nil", modTime, err)
	}
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatalf("os.Chtimes(%q) error = %v, want nil", path, err)
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	return string(data)
}

func syncRunContainsPath(entries []incrementalsync.QueueEntry, path string) bool {
	for _, entry := range entries {
		if entry.Path == path {
			return true
		}
	}
	return false
}

func findCLIQueuePath(t *testing.T, entries []incrementalsync.QueueEntry, path string) incrementalsync.QueueEntry {
	t.Helper()
	for _, entry := range entries {
		if entry.Path == path {
			return entry
		}
	}
	t.Fatalf("queue path %q not found in entries %+v", path, entries)
	return incrementalsync.QueueEntry{}
}

func mustSnapshotTree(t *testing.T, root string) []string {
	t.Helper()
	var entries []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		digest := "-"
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			digest = fmt.Sprintf("%x", sum[:])
		}
		entries = append(entries, fmt.Sprintf("%s %s %d %s", rel, info.Mode().String(), info.Size(), digest))
		return nil
	}); err != nil {
		t.Fatalf("filepath.WalkDir(%q) error = %v, want nil", root, err)
	}
	sort.Strings(entries)
	return entries
}

func readHTTPBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll(response body) error = %v, want nil", err)
	}
	return string(body)
}

func assertServeLowInfo(t *testing.T, text string, p profile.Profile, source string, target string) {
	t.Helper()
	for _, forbidden := range []string{
		p.ProfileID,
		p.Name,
		p.Target.TargetID,
		p.Target.LocalPath,
		p.Target.PairingReceiptID,
		source,
		target,
		"file_count",
		"manifest",
		"inventory",
		"hostname",
	} {
		if forbidden == "" {
			continue
		}
		if strings.Contains(text, forbidden) {
			t.Fatalf("serve text = %q, must not contain %q", text, forbidden)
		}
	}
}

func assertServeOperatorOutputLowInfo(t *testing.T, text string, p profile.Profile, source string, target string) {
	t.Helper()
	assertServeLowInfo(t, text, p, source, target)
	if p.Target.DevicePublicKey != "" && strings.Contains(text, p.Target.DevicePublicKey) {
		t.Fatalf("serve operator text = %q, must not contain pinned device public key", text)
	}
}

func assertTextContainsAll(t *testing.T, label string, text string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("%s text = %q, want %q", label, text, want)
		}
	}
}

func assertTextContainsNone(t *testing.T, label string, text string, forbidden ...string) {
	t.Helper()
	for _, value := range forbidden {
		if strings.Contains(text, value) {
			t.Fatalf("%s text = %q, must not contain %q", label, text, value)
		}
	}
}

func firstLineWithPrefix(text string, prefix string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}

func lifecycleEventTypes(events []agentdaemon.LifecycleEvent) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

func lifecycleEventTypesContain(events []agentdaemon.LifecycleEvent, wants ...string) bool {
	have := make(map[string]struct{}, len(events))
	for _, event := range events {
		have[event.Type] = struct{}{}
	}
	for _, want := range wants {
		if _, ok := have[want]; !ok {
			return false
		}
	}
	return true
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
