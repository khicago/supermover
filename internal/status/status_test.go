package status

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/prune"
	"github.com/khicago/supermover/internal/report"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
)

func TestBuildCleanTargetReturnsVerified(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	writeCompleteSession(t, target, "session-clean", "docs/a.txt", []byte("hello"))

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}
	if got.Schema != SchemaV1 {
		t.Fatalf("Build(%q).Schema = %q, want %q", target, got.Schema, SchemaV1)
	}
	if got.Overall.Status != OverallClean || got.Overall.TargetStatus != string(report.StatusVerified) || got.ReviewRequired || got.NeedsReview() || got.ExitCode() != ExitOK {
		t.Fatalf("Build(%q) status=%+v review=%t exit=%d, want clean verified", target, got.Overall, got.ReviewRequired, got.ExitCode())
	}
	if got.Scope != report.ScopeLocalMigrationTarget || got.ProfileID != p.ProfileID || got.TargetID != p.Target.TargetID {
		t.Fatalf("Build(%q) scope/profile/target = %q/%q/%q, want profile scoped local target", target, got.Scope, got.ProfileID, got.TargetID)
	}
	if got.LatestSession.ID != "session-clean" || got.LatestSession.CompletenessStatus != string(report.CompletenessVerified) {
		t.Fatalf("Build(%q).LatestSession = %+v, want verified session-clean", target, got.LatestSession)
	}
	if got.Counts.ManifestCount != 1 || got.Counts.ManifestEntries != 1 || got.Counts.FilesExpected != 1 || got.Counts.FilesVerified != 1 {
		t.Fatalf("Build(%q).Counts = %+v, want one verified manifest entry", target, got.Counts)
	}
	if got.Pairing.EncryptedTransfer != "not_configured" || got.Privacy.NetworkTransfer != "not_configured" {
		t.Fatalf("Build(%q) pairing=%+v privacy=%+v, want inherited not-configured evidence only", target, got.Pairing, got.Privacy)
	}
	if got.Network.Status != NetworkStatusNoEvidence || len(got.Network.Transfers) != 0 {
		t.Fatalf("Build(%q).Network = %+v, want no evidence instead of network health claim", target, got.Network)
	}
}

func TestBuildShowsProfileBackedMTLSConfigured(t *testing.T) {
	target := t.TempDir()
	p, receipt := testNetworkPairedProfile(t, target)
	writePairingReceipt(t, target, receipt)
	writeCompleteSession(t, target, "session-clean", "docs/a.txt", []byte("hello"))

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}
	if got.Pairing.EncryptedTransfer != "profile_backed_mtls_configured" || got.Privacy.NetworkTransfer != "profile_backed_mtls_configured" {
		t.Fatalf("Build(%q) pairing=%+v privacy=%+v, want configured profile-backed mTLS state", target, got.Pairing, got.Privacy)
	}
	if got.TrafficPrivacy.Status != "blocked" || got.TrafficPrivacy.AnonymityClaim != "not_claimed" || len(got.TrafficPrivacy.Blockers) != 1 || got.TrafficPrivacy.Blockers[0] != "applied_overhead_missing" {
		t.Fatalf("Build(%q).TrafficPrivacy = %+v, want missing applied overhead blocker without anonymity claim", target, got.TrafficPrivacy)
	}
}

func TestBuildTrafficPrivacyAcceptancePassesWithPublishedLevel2Overhead(t *testing.T) {
	target := t.TempDir()
	p, receipt := testNetworkPairedProfile(t, target)
	writePairingReceipt(t, target, receipt)
	writeCompleteSession(t, target, "session-network", "docs/a.txt", []byte("hello"))
	writeNetworkTransfer(t, target, control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       "session-network",
		ProfileID:       "profile-local",
		TargetID:        "target-local",
		SourceDeviceID:  receipt.SourceDeviceID,
		TargetDeviceID:  receipt.TargetDeviceID,
		ProtocolVersion: "supermover/1",
		PrivacyPolicy:   transport.DefaultPrivacyPolicy(transport.PrivacyLevel2),
		PrivacyOverhead: validStatusLevel2PrivacyOverhead(),
		Status:          control.NetworkTransferPublished,
		Stage:           "commit",
		StartedAt:       "2026-05-16T00:00:00Z",
		UpdatedAt:       "2026-05-16T00:01:00Z",
	})

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}

	if got.Overall.Status != OverallClean || got.Counts.NetworkTransfers != 0 || got.Network.Status != NetworkStatusNoEvidence {
		t.Fatalf("Build(%q) status=%+v counts=%+v network=%+v, want clean published transfer omitted from review evidence", target, got.Overall, got.Counts, got.Network)
	}
	if got.TrafficPrivacy.Status != "passed" || got.TrafficPrivacy.SessionID != "session-network" || got.TrafficPrivacy.AnonymityClaim != "not_claimed" {
		t.Fatalf("Build(%q).TrafficPrivacy = %+v, want passed acceptance without anonymity claim", target, got.TrafficPrivacy)
	}
	if got.TrafficPrivacy.ObservedOverhead == nil || got.TrafficPrivacy.ObservedOverhead.PaddingBytes != 128 || got.TrafficPrivacy.ObservedOverhead.JitterBudgetMillis != 250 {
		t.Fatalf("Build(%q).TrafficPrivacy.ObservedOverhead = %+v, want persisted padding and jitter evidence", target, got.TrafficPrivacy.ObservedOverhead)
	}
}

func TestBuildRequiresProfileSSOT(t *testing.T) {
	target := t.TempDir()

	got, err := Build(Options{TargetRoot: target})

	if err == nil {
		t.Fatalf("Build(%q) = %+v, nil; want profile-required error", target, got)
	}
	if !strings.Contains(err.Error(), "profile is required") {
		t.Fatalf("Build(%q) error = %q, want profile-required error", target, err.Error())
	}
}

func TestBuildEmptyTargetRequiresReview(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != OverallReviewRequired || got.Overall.TargetStatus != string(report.StatusEmpty) || !got.ReviewRequired || !got.NeedsReview() || got.ExitCode() != ExitReviewNeeded {
		t.Fatalf("Build(%q) status=%+v review=%t exit=%d, want empty target review", target, got.Overall, got.ReviewRequired, got.ExitCode())
	}
	if got.LatestSession.CompletenessStatus != string(report.CompletenessNoPublishedSession) {
		t.Fatalf("Build(%q).LatestSession = %+v, want no published session", target, got.LatestSession)
	}
	if got.Counts.ManifestCount != 0 || got.Counts.ArtifactProblems != 0 {
		t.Fatalf("Build(%q).Counts = %+v, want empty target without artifact problems", target, got.Counts)
	}
}

func TestBuildWarningAndSoftDeleteRequireReviewWithoutMutatingTarget(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	writeCompleteSession(t, target, "session-review", "keep.txt", []byte("keep"))
	writeWarning(t, target, control.Warning{
		Version:    control.CurrentVersion,
		ID:         "warning-review",
		SessionID:  "session-review",
		Code:       "special_file",
		Message:    "path needs additional migration config",
		Severity:   "warning",
		Paths:      []string{"socket"},
		TargetPath: "socket",
		CreatedAt:  "2026-05-16T00:02:00Z",
	})
	writeSoftDelete(t, target, control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "session-review-del_001",
		SessionID:          "session-review",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "session-previous",
		PreviousManifestID: "manifest-previous",
		SourcePath:         "gone.txt",
		TargetPath:         "gone.txt",
		Kind:               "file",
		DetectedAt:         "2026-05-16T00:03:00Z",
		Reason:             "missing_from_latest_source_scan",
	})
	before := snapshotControlPlane(t, target)

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}
	after := snapshotControlPlane(t, target)

	if got.Overall.Status != OverallReviewRequired || got.Overall.TargetStatus != string(report.StatusAttention) || !got.ReviewRequired {
		t.Fatalf("Build(%q) status=%+v review=%t, want attention required", target, got.Overall, got.ReviewRequired)
	}
	if got.Counts.Warnings != 1 || got.Counts.SoftDeletes != 1 {
		t.Fatalf("Build(%q).Counts = %+v, want warning and soft delete counts", target, got.Counts)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("Build(%q) mutated control plane\nbefore=%#v\nafter=%#v", target, before, after)
	}
}

func TestBuildPruneApprovalInventoryRequiresReviewWithoutMutatingTarget(t *testing.T) {
	target := t.TempDir()
	p := testPruneProfile(t, target)
	writeCompleteSession(t, target, "session-previous", "gone.txt", []byte("stale"))
	writeCompleteSession(t, target, "session-prune", "keep.txt", []byte("keep"))
	record := control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "session-prune-del-001",
		SessionID:          "session-prune",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "session-previous",
		PreviousManifestID: "manifest-session-previous",
		SourcePath:         "gone.txt",
		TargetPath:         "gone.txt",
		Kind:               "file",
		Size:               int64(len("stale")),
		Digest:             digest([]byte("stale")),
		DetectedAt:         "2026-05-16T00:03:00Z",
		Reason:             "missing_from_latest_source_scan",
	}
	writeSoftDelete(t, target, record)
	writePruneApproval(t, target, testPruneApproval(t, p, record))
	before := snapshotControlPlane(t, target)

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}
	after := snapshotControlPlane(t, target)

	if got.Overall.Status != OverallReviewRequired || got.Overall.TargetStatus != string(report.StatusAttention) || !got.ReviewRequired || got.ExitCode() != ExitReviewNeeded {
		t.Fatalf("Build(%q) status=%+v review=%t exit=%d, want unapplied prune approval review", target, got.Overall, got.ReviewRequired, got.ExitCode())
	}
	if got.Counts.PruneApprovals != 1 || got.Counts.PruneUnappliedApprovals != 1 {
		t.Fatalf("Build(%q).Counts = %+v, want one unapplied prune approval", target, got.Counts)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("Build(%q) mutated control plane\nbefore=%#v\nafter=%#v", target, before, after)
	}
}

func TestBuildPruneApprovalArtifactProblemShowsSourceBreakdown(t *testing.T) {
	target := t.TempDir()
	p := testPruneProfile(t, target)
	writeCompleteSession(t, target, "session-clean", "keep.txt", []byte("keep"))
	path, err := control.Path(target, control.ArtifactPruneApproval, "approval-damaged")
	if err != nil {
		t.Fatalf("control.Path(%q, prune approval) error = %v, want nil", target, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}

	if got.Overall.Status != OverallReviewRequired || got.Overall.TargetStatus != string(report.StatusUnhealthy) || !got.ReviewRequired || got.ExitCode() != ExitReviewNeeded {
		t.Fatalf("Build(%q) status=%+v review=%t exit=%d, want unhealthy prune approval artifact review", target, got.Overall, got.ReviewRequired, got.ExitCode())
	}
	if got.Counts.ArtifactProblems != 1 {
		t.Fatalf("Build(%q).Counts = %+v, want one artifact problem", target, got.Counts)
	}
	if got.Counts.ArtifactProblemSources == nil || len(got.Counts.ArtifactProblemSources) != 1 || got.Counts.ArtifactProblemSources[0] != (ArtifactProblemSourceCount{Source: "prune_approval", Count: 1}) {
		t.Fatalf("Build(%q).Counts.ArtifactProblemSources = %+v, want prune approval source count", target, got.Counts.ArtifactProblemSources)
	}
}

func TestBuildLiveTargetDriftRequiresReviewWithoutPersistingDrift(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	writeCompleteSession(t, target, "session-live", "keep.txt", []byte("keep"))
	writeTargetFile(t, target, "extra.txt", []byte("target-only"))
	before := snapshotControlPlane(t, target)

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != OverallReviewRequired || got.Overall.TargetStatus != string(report.StatusAttention) || !got.ReviewRequired {
		t.Fatalf("Build(%q) status=%+v review=%t, want attention for live drift", target, got.Overall, got.ReviewRequired)
	}
	if got.Counts.TargetDrifts != 0 || got.Counts.LiveTargetDrifts != 1 || got.Counts.LiveTargetDriftProblems != 0 {
		t.Fatalf("Build(%q).Counts = %+v, want one live drift and no persisted drift", target, got.Counts)
	}
	after := snapshotControlPlane(t, target)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("Build(%q) mutated control plane\nbefore=%#v\nafter=%#v", target, before, after)
	}
	driftDir := filepath.Join(target, control.DirName, "drift")
	entries, err := os.ReadDir(driftDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil or missing drift directory", driftDir, err)
	}
	if len(entries) != 0 {
		t.Fatalf("os.ReadDir(%q) = %d entries, want no persisted live drift artifacts", driftDir, len(entries))
	}
}

func TestBuildResolvedTargetDriftDoesNotRequireReview(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	writeCompleteSession(t, target, "session-resolved", "keep.txt", []byte("keep"))
	writeTargetDrift(t, target, control.TargetDrift{
		Version:      control.CurrentVersion,
		ID:           "session-resolved-drift",
		SessionID:    "session-resolved",
		ProfileID:    "profile-local",
		TargetID:     "target-local",
		RootID:       "root",
		Path:         "keep.txt",
		DetectedAt:   "2026-05-16T00:01:00Z",
		Change:       "content_mismatch",
		ReviewState:  "resolved",
		ReviewAction: "resolve",
		ReviewedAt:   "2026-05-20T00:00:00Z",
		ReviewReason: "target restored to expected manifest evidence",
		Evidence:     []string{"target content differed from staged manifest"},
	})

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != OverallClean || got.Overall.TargetStatus != string(report.StatusVerified) || got.ReviewRequired || got.ExitCode() != ExitOK {
		t.Fatalf("Build(%q) status=%+v review=%t exit=%d, want clean resolved target drift", target, got.Overall, got.ReviewRequired, got.ExitCode())
	}
	if got.Counts.TargetDrifts != 0 || got.Counts.LiveTargetDrifts != 0 {
		t.Fatalf("Build(%q).Counts = %+v, want resolved drift excluded and no live drift", target, got.Counts)
	}
}

func TestBuildCorruptArtifactDoesNotMarkClean(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	writeCompleteSession(t, target, "session-corrupt", "docs/a.txt", []byte("hello"))
	warningPath := filepath.Join(control.ControlDir(target), "warnings", "bad.json")
	if err := os.MkdirAll(filepath.Dir(warningPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(warningPath), err)
	}
	if err := os.WriteFile(warningPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", warningPath, err)
	}

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != OverallReviewRequired || got.Overall.TargetStatus == string(report.StatusVerified) || !got.ReviewRequired {
		t.Fatalf("Build(%q) status=%+v review=%t, want corrupt artifact review", target, got.Overall, got.ReviewRequired)
	}
	if got.Counts.ArtifactProblems == 0 {
		t.Fatalf("Build(%q).Counts = %+v, want artifact problem surfaced", target, got.Counts)
	}
}

func TestBuildDamagedNetworkTransferArtifactRequiresNetworkReview(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	writeCompleteSession(t, target, "session-network-damaged", "docs/a.txt", []byte("hello"))
	path, err := control.Path(target, control.ArtifactNetworkTransfer, "session-network-damaged")
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}

	if got.Overall.Status != OverallReviewRequired || got.Overall.TargetStatus != string(report.StatusUnhealthy) || !got.ReviewRequired {
		t.Fatalf("Build(%q) status=%+v review=%t, want damaged network artifact review", target, got.Overall, got.ReviewRequired)
	}
	if got.Counts.ArtifactProblems == 0 {
		t.Fatalf("Build(%q).Counts = %+v, want artifact problem surfaced", target, got.Counts)
	}
	if got.Network.Status != NetworkStatusReviewRequired || got.Network.ArtifactProblems != 1 || len(got.Network.Transfers) != 0 {
		t.Fatalf("Build(%q).Network = %+v, want review-required damaged network artifact evidence without transfer summary", target, got.Network)
	}
}

func TestBuildIgnoresDamagedForeignNetworkTransferArtifact(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	writeCompleteSession(t, target, "session-clean", "docs/a.txt", []byte("hello"))
	writeReceiptForScope(t, target, "session-foreign-corrupt", "received", "profile-other", "target-other")
	writeRawSessionArtifact(t, target, "session-foreign-corrupt", "network-transfer.json", `{"version":1,`)

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}

	if got.Overall.Status != OverallClean || got.ReviewRequired || got.ExitCode() != ExitOK {
		t.Fatalf("Build(%q) status=%+v review=%t exit=%d, want clean current profile despite foreign corrupt transfer", target, got.Overall, got.ReviewRequired, got.ExitCode())
	}
	if got.Counts.ArtifactProblems != 0 || got.Network.Status != NetworkStatusNoEvidence || got.Network.ArtifactProblems != 0 {
		t.Fatalf("Build(%q) counts=%+v network=%+v, want no current-profile network artifact problem", target, got.Counts, got.Network)
	}
}

func TestBuildKeepsDamagedNetworkTransferArtifactWhenReceiptIDMismatches(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	writeCompleteSession(t, target, "session-clean", "docs/a.txt", []byte("hello"))
	writeReceiptWithIDForScope(t, target, "session-local-corrupt", "session-other", "received", "profile-other", "target-other")
	writeRawSessionArtifact(t, target, "session-local-corrupt", "network-transfer.json", `{"version":1,`)

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}

	if got.Overall.Status != OverallReviewRequired || !got.ReviewRequired || got.ExitCode() != ExitReviewNeeded {
		t.Fatalf("Build(%q) status=%+v review=%t exit=%d, want review for corrupt transfer with mismatched receipt id", target, got.Overall, got.ReviewRequired, got.ExitCode())
	}
	if got.Counts.ArtifactProblems != 1 || got.Network.Status != NetworkStatusReviewRequired || got.Network.ArtifactProblems != 1 {
		t.Fatalf("Build(%q) counts=%+v network=%+v, want network artifact problem retained", target, got.Counts, got.Network)
	}
}

func TestBuildIncompleteSessionRequiresRecoveryReview(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	writeSessionRecord(t, target, "session-incomplete", transaction.StateStaged)

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}

	if got.Overall.Status != OverallReviewRequired || got.Overall.TargetStatus != string(report.StatusUnhealthy) || !got.ReviewRequired || got.ExitCode() != ExitReviewNeeded {
		t.Fatalf("Build(%q) status=%+v review=%t exit=%d, want recovery review", target, got.Overall, got.ReviewRequired, got.ExitCode())
	}
	if got.Counts.RecoveryIssues != 1 {
		t.Fatalf("Build(%q).Counts = %+v, want one recovery issue", target, got.Counts)
	}
}

func TestBuildNetworkTransferArtifactIsEvidenceOnlyReview(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	writeCompleteSession(t, target, "session-network", "docs/a.txt", []byte("hello"))
	writeNetworkTransfer(t, target, control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       "session-network",
		ProfileID:       "profile-local",
		TargetID:        "target-local",
		SourceDeviceID:  "sha256:abcdef0123456789",
		TargetDeviceID:  "sha256:0123456789abcdef",
		ProtocolVersion: "supermover/1",
		PrivacyPolicy:   transport.DefaultPrivacyPolicy(transport.PrivacyLevel2),
		Status:          control.NetworkTransferInterrupted,
		Stage:           "chunk",
		StartedAt:       "2026-05-16T00:00:00Z",
		UpdatedAt:       "2026-05-16T00:00:01Z",
		ErrorCode:       "interrupted",
		Error:           "context canceled",
	})

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q) error = %v, want nil", target, err)
	}

	if got.Overall.Status != OverallReviewRequired || got.Overall.TargetStatus != string(report.StatusUnhealthy) || !got.ReviewRequired {
		t.Fatalf("Build(%q) status=%+v review=%t, want network evidence review", target, got.Overall, got.ReviewRequired)
	}
	if got.Counts.NetworkTransfers != 1 {
		t.Fatalf("Build(%q).Counts = %+v, want one network transfer issue", target, got.Counts)
	}
	if got.Network.Status != NetworkStatusReviewRequired || len(got.Network.Transfers) != 1 {
		t.Fatalf("Build(%q).Network = %+v, want review-required local transfer artifact evidence", target, got.Network)
	}
	transfer := got.Network.Transfers[0]
	if transfer.SessionID != "session-network" || transfer.Status != string(control.NetworkTransferInterrupted) || transfer.Action == "" {
		t.Fatalf("Build(%q).Network.Transfers[0] = %+v, want compact interrupted transfer evidence", target, transfer)
	}
	if got.Pairing.EncryptedTransfer != "not_configured" || got.Privacy.NetworkTransfer != "not_configured" {
		t.Fatalf("Build(%q) pairing=%+v privacy=%+v, want network artifact evidence without configured transfer material", target, got.Pairing, got.Privacy)
	}
}

func TestBuildStartedChunkNetworkTransferRequiresReviewUntilPublished(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	writeCompleteSession(t, target, "session-network", "docs/a.txt", []byte("hello"))
	started := control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       "session-network",
		ProfileID:       "profile-local",
		TargetID:        "target-local",
		SourceDeviceID:  "sha256:abcdef0123456789",
		TargetDeviceID:  "sha256:0123456789abcdef",
		ProtocolVersion: "supermover/1",
		PrivacyPolicy:   transport.DefaultPrivacyPolicy(transport.PrivacyLevel2),
		PrivacyOverhead: validStatusLevel2PrivacyOverhead(),
		Status:          control.NetworkTransferStarted,
		Stage:           "chunk",
		StartedAt:       "2026-05-16T00:00:00Z",
		UpdatedAt:       "2026-05-16T00:00:01Z",
	}
	writeNetworkTransfer(t, target, started)

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q, started) error = %v, want nil", target, err)
	}
	if got.Overall.Status != OverallReviewRequired || got.Overall.TargetStatus != string(report.StatusUnhealthy) || !got.ReviewRequired {
		t.Fatalf("Build(%q, started) status=%+v review=%t, want started transfer review", target, got.Overall, got.ReviewRequired)
	}
	if got.Counts.NetworkTransfers != 1 || got.Network.Status != NetworkStatusReviewRequired || len(got.Network.Transfers) != 1 {
		t.Fatalf("Build(%q, started) counts=%+v network=%+v, want one review-required transfer", target, got.Counts, got.Network)
	}
	transfer := got.Network.Transfers[0]
	if transfer.SessionID != "session-network" || transfer.Status != string(control.NetworkTransferStarted) || transfer.Stage != "chunk" || transfer.Action != "retry_network_transfer" {
		t.Fatalf("Build(%q, started).Network.Transfers[0] = %+v, want started chunk retry evidence", target, transfer)
	}

	published := started
	published.Status = control.NetworkTransferPublished
	published.Stage = "commit"
	published.UpdatedAt = "2026-05-16T00:01:00Z"
	writeNetworkTransfer(t, target, published)

	got, err = Build(Options{TargetRoot: target, Profile: &p})
	if err != nil {
		t.Fatalf("Build(%q, published) error = %v, want nil", target, err)
	}
	if got.Overall.Status != OverallClean || got.Overall.TargetStatus != string(report.StatusVerified) || got.ReviewRequired {
		t.Fatalf("Build(%q, published) status=%+v review=%t, want clean published transfer", target, got.Overall, got.ReviewRequired)
	}
	if got.Counts.NetworkTransfers != 0 || got.Network.Status != NetworkStatusNoEvidence || len(got.Network.Transfers) != 0 {
		t.Fatalf("Build(%q, published) counts=%+v network=%+v, want published transfer omitted from review evidence", target, got.Counts, got.Network)
	}
}

func TestBuildUnsafeBoundaryReturnsReportError(t *testing.T) {
	target := t.TempDir()
	p := testProfile(t, target)
	controlPath := filepath.Join(target, control.DirName)
	if err := os.RemoveAll(controlPath); err != nil {
		t.Fatalf("os.RemoveAll(%q) error = %v, want nil", controlPath, err)
	}
	if err := os.Symlink(t.TempDir(), controlPath); err != nil {
		t.Fatalf("os.Symlink(control path) error = %v, want nil", err)
	}

	got, err := Build(Options{TargetRoot: target, Profile: &p})
	if err == nil {
		t.Fatalf("Build(%q) = %+v, nil; want unsafe boundary error", target, got)
	}
}

func testProfile(t *testing.T, target string) profile.Profile {
	t.Helper()
	p := profile.NewDefault("profile-local", "Profile", t.TempDir(), target)
	p.Target.TargetID = "target-local"
	return p
}

func testPruneProfile(t *testing.T, target string) profile.Profile {
	t.Helper()
	p := testProfile(t, target)
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 30
	p.DeletePolicy.AllowPhysicalPrune = true
	return p
}

func testNetworkPairedProfile(t *testing.T, target string) (profile.Profile, control.PairingReceipt) {
	t.Helper()
	now := time.Now().UTC()
	sourceCert := newStatusTestCertificate(t, "source", now.Add(-time.Hour), now.Add(time.Hour))
	targetCert := newStatusTestCertificate(t, "target", now.Add(-time.Hour), now.Add(time.Hour))
	sourceDeviceID := statusTestDeviceID(t, sourceCert)
	targetDeviceID := statusTestDeviceID(t, targetCert)
	p := profile.NewDefault("profile-local", "Profile", t.TempDir(), target)
	p.Target.TargetID = "target-local"
	p.Target.DevicePublicKey = targetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = now.Format(time.RFC3339)
	certPath, keyPath := writeStatusTLSIdentity(t, sourceCert)
	p.Network = &profile.NetworkConfig{
		ReceiverURL: "https://127.0.0.1:9443",
		LocalTLSIdentity: profile.TLSIdentityRef{
			CertificatePath: certPath,
			PrivateKeyPath:  keyPath,
		},
	}
	receipt := control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               p.Target.PairingReceiptID,
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   sourceDeviceID,
		TargetDeviceID:   targetDeviceID,
		DevicePublicKey:  targetDeviceID,
		Method:           "sas",
		VerifiedAt:       p.Target.PairedAt,
		VerificationHash: "sha256:abcdef0123456789",
		ProtocolVersion:  protocol.Version,
	}
	return p, receipt
}

func writePairingReceipt(t *testing.T, target string, receipt control.PairingReceipt) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactPairingReceipt, receipt.ID)
	if err != nil {
		t.Fatalf("control.Path(%q, pairing receipt, %q) error = %v, want nil", target, receipt.ID, err)
	}
	if err := control.WriteFile(path, receipt); err != nil {
		t.Fatalf("control.WriteFile(%q, pairing receipt) error = %v, want nil", path, err)
	}
}

func newStatusTestCertificate(t *testing.T, commonName string, notBefore, notAfter time.Time) tls.Certificate {
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
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: privateKey, Leaf: leaf}
}

func writeStatusTLSIdentity(t *testing.T, cert tls.Certificate) (string, string) {
	t.Helper()
	dir := t.TempDir()
	certPath := filepath.Join(dir, "identity.crt")
	keyPath := filepath.Join(dir, "identity.key")
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

func statusTestDeviceID(t *testing.T, cert tls.Certificate) string {
	t.Helper()
	id, err := transport.LeafSPKIDeviceID(cert.Leaf)
	if err != nil {
		t.Fatalf("LeafSPKIDeviceID error = %v, want nil", err)
	}
	return id
}

func writeCompleteSession(t *testing.T, target, sessionID, rel string, data []byte) {
	t.Helper()
	writeTargetFile(t, target, rel, data)
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{{
			Path:       rel,
			Kind:       "file",
			Size:       int64(len(data)),
			Digest:     digest(data),
			TargetPath: rel,
		}},
	})
	writePublishedReceipt(t, target, sessionID)
	writeProfileSnapshot(t, target, sessionID)
	writeSessionRecord(t, target, sessionID, transaction.StatePublished)
}

func writeTargetFile(t *testing.T, target string, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(target, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
}

func writeManifest(t *testing.T, target string, manifest control.Manifest) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactManifest, manifest.SessionID)
	if err != nil {
		t.Fatalf("control.Path(%q, manifest, %q) error = %v, want nil", target, manifest.SessionID, err)
	}
	if err := control.WriteFile(path, manifest); err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", path, err)
	}
}

func writePublishedReceipt(t *testing.T, target string, sessionID string) {
	t.Helper()
	writeReceiptForScope(t, target, sessionID, "published", "profile-local", "target-local")
}

func writeReceiptForScope(t *testing.T, target string, sessionID string, status string, profileID string, targetID string) {
	t.Helper()
	writeReceiptWithIDForScope(t, target, sessionID, sessionID, status, profileID, targetID)
}

func writeReceiptWithIDForScope(t *testing.T, target string, sessionID string, receiptID string, status string, profileID string, targetID string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(%q, receipt, %q) error = %v, want nil", target, sessionID, err)
	}
	receipt := control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        receiptID,
		ProfileID: profileID,
		TargetID:  targetID,
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    status,
	}
	if err := control.WriteFile(path, receipt); err != nil {
		t.Fatalf("control.WriteFile(%q, receipt) error = %v, want nil", path, err)
	}
}

func writeProfileSnapshot(t *testing.T, target string, sessionID string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-"+sessionID)
	if err != nil {
		t.Fatalf("control.Path(%q, profile snapshot, %q) error = %v, want nil", target, sessionID, err)
	}
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + sessionID,
		ProfileID:  "profile-local",
		SessionID:  sessionID,
		CapturedAt: "2026-05-16T00:00:00Z",
		Profile:    []byte(`{"profile_id":"profile-local","privacy_policy":{"mode":"plaintext","traffic_level":2,"allow_plaintext_restore":true,"allow_hidden_files":true,"allow_sensitive_filenames":true,"padding_bucket_bytes":65536,"batch_max_bytes":1048576,"batch_max_count":64,"jitter_budget_millis":250,"discovery_low_info":true},"roots":[{"id":"root"}],"target":{"target_id":"target-local"}}`),
	}
	if err := control.WriteFile(path, snapshot); err != nil {
		t.Fatalf("control.WriteFile(%q, profile snapshot) error = %v, want nil", path, err)
	}
}

func writeSessionRecord(t *testing.T, target string, sessionID string, state transaction.State) {
	t.Helper()
	layout := transaction.NewLayout(control.ControlDir(target))
	record := transaction.SessionRecord{
		ID:        sessionID,
		State:     state,
		CreatedAt: time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 16, 0, 1, 0, 0, time.UTC),
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		t.Fatalf("layout.WriteSessionRecord(%q) error = %v, want nil", sessionID, err)
	}
}

func writeWarning(t *testing.T, target string, warning control.Warning) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactWarning, warning.ID)
	if err != nil {
		t.Fatalf("control.Path(%q, warning, %q) error = %v, want nil", target, warning.ID, err)
	}
	if err := control.WriteFile(path, warning); err != nil {
		t.Fatalf("control.WriteFile(%q, warning) error = %v, want nil", path, err)
	}
}

func writeSoftDelete(t *testing.T, target string, softDelete control.SoftDelete) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSoftDelete, softDelete.ID)
	if err != nil {
		t.Fatalf("control.Path(%q, soft delete, %q) error = %v, want nil", target, softDelete.ID, err)
	}
	if err := control.WriteFile(path, softDelete); err != nil {
		t.Fatalf("control.WriteFile(%q, soft delete) error = %v, want nil", path, err)
	}
}

func writeTargetDrift(t *testing.T, target string, drift control.TargetDrift) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactTargetDrift, drift.ID)
	if err != nil {
		t.Fatalf("control.Path(%q, target drift, %q) error = %v, want nil", target, drift.ID, err)
	}
	if err := control.WriteFile(path, drift); err != nil {
		t.Fatalf("control.WriteFile(%q, target drift) error = %v, want nil", path, err)
	}
}

func writePruneApproval(t *testing.T, target string, approval control.PruneApproval) {
	t.Helper()
	writePruneApprovalProfileSnapshot(t, target, approval)
	path, err := control.Path(target, control.ArtifactPruneApproval, approval.ID)
	if err != nil {
		t.Fatalf("control.Path(%q, prune approval, %q) error = %v, want nil", target, approval.ID, err)
	}
	if err := control.WriteFile(path, approval); err != nil {
		t.Fatalf("control.WriteFile(%q, prune approval) error = %v, want nil", path, err)
	}
}

func writePruneApprovalProfileSnapshot(t *testing.T, target string, approval control.PruneApproval) {
	t.Helper()
	payload := pruneApprovalProfileSnapshotPayload(t, approval.ID)
	snapshotDigest, err := prune.ProfileSnapshotDigest(payload)
	if err != nil {
		t.Fatalf("prune.ProfileSnapshotDigest(%q) error = %v, want nil", approval.ID, err)
	}
	if snapshotDigest != approval.ProfileSnapshotDigest {
		t.Fatalf("profile snapshot digest for %q = %s, want approval digest %s", approval.ID, snapshotDigest, approval.ProfileSnapshotDigest)
	}
	path, err := control.Path(target, control.ArtifactProfileSnapshot, approval.ProfileSnapshotID)
	if err != nil {
		t.Fatalf("control.Path(%q, profile snapshot, %q) error = %v, want nil", target, approval.ProfileSnapshotID, err)
	}
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         approval.ProfileSnapshotID,
		ProfileID:  approval.ProfileID,
		CapturedAt: approval.CreatedAt,
		Profile:    payload,
	}
	if err := control.WriteFile(path, snapshot); err != nil {
		t.Fatalf("control.WriteFile(%q, profile snapshot) error = %v, want nil", path, err)
	}
}

func testPruneApproval(t *testing.T, p profile.Profile, record control.SoftDelete) control.PruneApproval {
	t.Helper()
	snapshotPayload := pruneApprovalProfileSnapshotPayload(t, "approval-status")
	snapshotDigest, err := prune.ProfileSnapshotDigest(snapshotPayload)
	if err != nil {
		t.Fatalf("prune.ProfileSnapshotDigest(approval-status) error = %v, want nil", err)
	}
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
		SymlinkTarget:      record.SymlinkTarget,
		DetectedAt:         record.DetectedAt,
	}
	approval := control.PruneApproval{
		Version:               control.CurrentVersion,
		ID:                    "approval-status",
		ProfileID:             p.ProfileID,
		TargetID:              p.Target.TargetID,
		RootID:                record.RootID,
		CreatedAt:             "2026-05-18T09:00:00Z",
		ApprovedBy:            "reviewer",
		ApprovedAt:            "2026-05-18T09:30:00Z",
		ReviewTool:            "unit-test",
		ProfileSnapshotID:     "profile-approval-status",
		ProfileSnapshotDigest: snapshotDigest,
		ProfileDeletePolicy: control.PruneDeletePolicy{
			Mode:               string(p.DeletePolicy.Mode),
			RequireReview:      p.DeletePolicy.RequireReview,
			RetentionDays:      p.DeletePolicy.RetentionDays,
			AllowPhysicalPrune: p.DeletePolicy.AllowPhysicalPrune,
		},
		Items:          []control.PruneApprovalItem{item},
		ExpiresAt:      "2026-05-19T00:00:00Z",
		Status:         "approved",
		ApprovalReason: "operator reviewed soft-delete evidence",
	}
	approval.ApprovalScopeDigest = prune.ApprovalScopeDigest(approval.ProfileID, approval.TargetID, approval.RootID, approval.ProfileSnapshotID, approval.ProfileSnapshotDigest, p.DeletePolicy, approval.Items)
	return approval
}

func pruneApprovalProfileSnapshotPayload(t *testing.T, approvalID string) []byte {
	t.Helper()
	p := profile.NewDefault("profile-local", "Profile", "/source", "/target")
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 30
	p.DeletePolicy.AllowPhysicalPrune = true
	payload, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(profile snapshot for %q) error = %v, want nil", approvalID, err)
	}
	return append(payload, '\n')
}

func writeNetworkTransfer(t *testing.T, target string, transfer control.NetworkTransfer) {
	t.Helper()
	transfer = normalizeNetworkTransferFixture(transfer)
	path, err := control.Path(target, control.ArtifactNetworkTransfer, transfer.SessionID)
	if err != nil {
		t.Fatalf("control.Path(%q, network transfer, %q) error = %v, want nil", target, transfer.SessionID, err)
	}
	if err := control.WriteFile(path, transfer); err != nil {
		t.Fatalf("control.WriteFile(%q, network transfer) error = %v, want nil", path, err)
	}
}

func writeRawSessionArtifact(t *testing.T, target string, sessionID string, name string, data string) {
	t.Helper()
	path := filepath.Join(target, control.DirName, "sessions", sessionID, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
}

func normalizeNetworkTransferFixture(transfer control.NetworkTransfer) control.NetworkTransfer {
	attempt := control.NetworkTransferAttempt{
		AttemptID: "attempt-1",
		StartedAt: transfer.StartedAt,
		EndedAt:   transfer.UpdatedAt,
		Stage:     transfer.Stage,
		Status:    transfer.Status,
		ErrorCode: transfer.ErrorCode,
		Error:     transfer.Error,
	}
	if transfer.Status == control.NetworkTransferStarted {
		attempt.EndedAt = ""
	}
	transfer.Attempts = []control.NetworkTransferAttempt{attempt}
	return transfer
}

func validStatusLevel2PrivacyOverhead() *control.NetworkTransferPrivacyOverhead {
	return &control.NetworkTransferPrivacyOverhead{
		FramePlainBytes:      512,
		FrameWireBytes:       640,
		PaddingBytes:         128,
		PaddedChunks:         2,
		PaddingBucketBytes:   64 * 1024,
		BatchFrames:          1,
		BatchedChunks:        2,
		MaxBatchCount:        2,
		MaxBatchPlainBytes:   512,
		JitteredRequests:     2,
		JitterDelayMillis:    50,
		MaxJitterDelayMillis: 25,
		JitterBudgetMillis:   250,
	}
}

func snapshotControlPlane(t *testing.T, target string) []string {
	t.Helper()
	var paths []string
	controlDir := control.ControlDir(target)
	err := filepath.WalkDir(controlDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(controlDir, path)
		if err != nil {
			return err
		}
		entry := filepath.ToSlash(rel)
		if !d.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			entry += " " + digest(data)
		}
		paths = append(paths, entry)
		return nil
	})
	if err != nil {
		t.Fatalf("filepath.WalkDir(%q) error = %v, want nil", controlDir, err)
	}
	return paths
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
