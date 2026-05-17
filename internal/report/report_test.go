package report

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
	"fmt"
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
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
)

func TestBuildReportEmptyTarget(t *testing.T) {
	target := t.TempDir()

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Scope != ScopeLocalMigrationTarget {
		t.Fatalf("BuildReport(%q).Scope = %q, want %q", target, got.Scope, ScopeLocalMigrationTarget)
	}
	if got.Overall.Status != StatusEmpty {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusEmpty)
	}
	if got.LatestSession.Completeness.Status != CompletenessNoPublishedSession {
		t.Fatalf("BuildReport(%q).LatestSession.Completeness.Status = %q, want %q", target, got.LatestSession.Completeness.Status, CompletenessNoPublishedSession)
	}
	if got.Summary.ManifestCount != 0 || got.Summary.ArtifactProblems != 0 || !got.Health.Healthy {
		t.Fatalf("BuildReport(%q) summary=%+v health=%+v, want empty healthy target", target, got.Summary, got.Health)
	}
}

func TestBuildReportPairingStateUnpaired(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Pairing.Status != PairingStatusUnpaired || got.Pairing.EncryptedTransfer != "not_configured" {
		t.Fatalf("BuildReport(%q).Pairing = %+v, want unpaired/not_configured", target, got.Pairing)
	}
	if got.Summary.PairingIssues != 0 || len(got.ArtifactProblems) != 0 {
		t.Fatalf("BuildReport(%q) pairing_issues=%d artifacts=%#v, want unpaired without review problem", target, got.Summary.PairingIssues, got.ArtifactProblems)
	}
	if got.Privacy.Status != "profile_contract_only" || got.Privacy.TrafficLevel != 2 || got.Privacy.Claim != "bounded_reduction_only" {
		t.Fatalf("BuildReport(%q).Privacy = %+v, want level 2 bounded-reduction profile contract", target, got.Privacy)
	}
	if got.Privacy.LocalPush != "traffic_shaping_not_applied" || got.Privacy.NetworkTransfer != "not_configured" {
		t.Fatalf("BuildReport(%q).Privacy = %+v, want local push not applied and network transfer not configured", target, got.Privacy)
	}
	privacyJSON, err := json.Marshal(got.Privacy)
	if err != nil {
		t.Fatalf("json.Marshal(Privacy) error = %v, want nil", err)
	}
	if !strings.Contains(string(privacyJSON), `"configured_reductions"`) || strings.Contains(string(privacyJSON), `"configured_reduction"`) {
		t.Fatalf("Privacy JSON = %s, want configured_reductions key without singular configured_reduction", privacyJSON)
	}
	for _, want := range []string{"total_bytes", "duration", "peer_ip", "lan_presence", "supermover_use"} {
		if !containsString(got.Privacy.ResidualLeakage, want) {
			t.Fatalf("BuildReport(%q).Privacy.ResidualLeakage = %#v, want %q", target, got.Privacy.ResidualLeakage, want)
		}
	}
}

func TestPrivacyForProfileCoversUnavailableAndNonLevel2Profiles(t *testing.T) {
	unavailable := PrivacyForProfile(nil)
	if unavailable.Status != "profile_unavailable" || unavailable.Claim != "not_available" || unavailable.NetworkTransfer != "not_available" {
		t.Fatalf("PrivacyForProfile(nil) = %+v, want unavailable contract without level 2 reduction claim", unavailable)
	}
	if !containsString(unavailable.ResidualLeakage, "total_bytes") || !containsString(unavailable.ResidualLeakage, "supermover_use") {
		t.Fatalf("PrivacyForProfile(nil).ResidualLeakage = %#v, want residual leakage terms", unavailable.ResidualLeakage)
	}

	p := profile.NewDefault("profile-local", "Profile", t.TempDir(), t.TempDir())
	p.PrivacyPolicy.TrafficLevel = 1
	p.PrivacyPolicy.PaddingBucketBytes = 0
	p.PrivacyPolicy.BatchMaxBytes = 0
	p.PrivacyPolicy.BatchMaxCount = 0
	p.PrivacyPolicy.JitterBudgetMillis = 0
	p.PrivacyPolicy.DiscoveryLowInfo = false
	got := PrivacyForProfile(&p)
	if got.Status != "profile_contract_only" || got.TrafficLevel != 1 || got.Claim != "not_applicable" {
		t.Fatalf("PrivacyForProfile(level 1) = %+v, want profile contract level 1", got)
	}
	if len(got.ConfiguredReduction) != 0 {
		t.Fatalf("PrivacyForProfile(level 1).ConfiguredReduction = %#v, want none", got.ConfiguredReduction)
	}
}

func TestPrivacyForProfileMarksCustomLevel2UnsupportedByLocalPush(t *testing.T) {
	p := profile.NewDefault("profile-local", "Profile", t.TempDir(), t.TempDir())
	p.PrivacyPolicy.PaddingBucketBytes = 128 * 1024

	got := PrivacyForProfile(&p)

	if got.Status != "profile_contract_only" || got.TrafficLevel != 2 || got.Claim != "bounded_reduction_only" {
		t.Fatalf("PrivacyForProfile(custom level 2) = %+v, want level 2 profile contract", got)
	}
	if got.LocalPush != "unsupported_privacy_policy" || got.Overhead.Status != "not_applied" {
		t.Fatalf("PrivacyForProfile(custom level 2) = %+v, want unsupported local push with not-applied overhead", got)
	}
}

func TestBuildReportPairingStateValidReceipt(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := validReportPairedProfile(source, target)
	writeReportPairingReceipt(t, target, validReportPairingReceipt(p))

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Pairing.Status != PairingStatusValid || got.Pairing.Evidence != "profile_pins_match_pairing_receipt" {
		t.Fatalf("BuildReport(%q).Pairing = %+v, want valid pairing evidence", target, got.Pairing)
	}
	if got.Pairing.ReceiptID != p.Target.PairingReceiptID || got.Pairing.TargetDeviceID != p.Target.DevicePublicKey || got.Pairing.Method != "sas" || got.Pairing.VerifiedAt != p.Target.PairedAt {
		t.Fatalf("BuildReport(%q).Pairing = %+v, want receipt/method/device/timestamp", target, got.Pairing)
	}
	if got.Pairing.EncryptedTransfer != "not_configured" || got.Summary.PairingIssues != 0 || len(got.ArtifactProblems) != 0 {
		t.Fatalf("BuildReport(%q) pairing=%+v summary=%+v artifacts=%#v, want valid evidence without transfer readiness", target, got.Pairing, got.Summary, got.ArtifactProblems)
	}
}

func TestBuildReportPairingStateWithProfileBackedMTLSConfigured(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p, receipt := validReportNetworkPairedProfile(t, source, target)
	writeReportPairingReceipt(t, target, receipt)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Pairing.Status != PairingStatusValid || got.Pairing.EncryptedTransfer != "profile_backed_mtls_configured" {
		t.Fatalf("BuildReport(%q).Pairing = %+v, want valid profile-backed mTLS configuration", target, got.Pairing)
	}
	if got.Privacy.NetworkTransfer != "profile_backed_mtls_configured" || got.Privacy.Overhead.Status != "not_applied" {
		t.Fatalf("BuildReport(%q).Privacy = %+v, want configured network transfer without applied overhead", target, got.Privacy)
	}
	if got.TrafficPrivacy.Status != "blocked" || got.TrafficPrivacy.AnonymityClaim != "not_claimed" || !containsString(got.TrafficPrivacy.Blockers, "applied_overhead_missing") {
		t.Fatalf("BuildReport(%q).TrafficPrivacy = %+v, want explicit level 2 acceptance blocker without anonymity claim", target, got.TrafficPrivacy)
	}
}

func TestBuildReportTrafficPrivacyAcceptancePassesWithPublishedLevel2Overhead(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p, receipt := validReportNetworkPairedProfile(t, source, target)
	writeReportPairingReceipt(t, target, receipt)
	writeCompleteSession(t, target, "session-network", "docs/a.txt", []byte("hello"))
	transfer := validReportNetworkTransfer("session-network")
	transfer.SourceDeviceID = receipt.SourceDeviceID
	transfer.TargetDeviceID = receipt.TargetDeviceID
	transfer.Status = control.NetworkTransferPublished
	transfer.Stage = "commit"
	transfer.UpdatedAt = "2026-05-16T00:01:00Z"
	transfer.ErrorCode = ""
	transfer.Error = ""
	transfer.PrivacyOverhead = validReportLevel2PrivacyOverhead()
	transfer.PrivacyOverhead.PaddingBucketBytes = p.PrivacyPolicy.PaddingBucketBytes
	writeNetworkTransfer(t, target, transfer)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, SessionID: "session-network", Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}

	if got.Overall.Status != StatusVerified || got.Summary.NetworkTransfers != 0 || len(got.NetworkTransfers) != 0 {
		t.Fatalf("BuildReport(%q) overall=%+v summary=%+v transfers=%#v, want clean published network transfer omitted from review issues", target, got.Overall, got.Summary, got.NetworkTransfers)
	}
	acceptance := got.TrafficPrivacy
	if acceptance.Status != "passed" || acceptance.SessionID != "session-network" || acceptance.Scope != "profile_backed_network_path" || acceptance.EvidenceSource != "network_transfer_artifact" {
		t.Fatalf("BuildReport(%q).TrafficPrivacy = %+v, want passed acceptance tied to published network transfer artifact", target, acceptance)
	}
	if acceptance.Claim != "bounded_reduction_only" || acceptance.AnonymityClaim != "not_claimed" {
		t.Fatalf("BuildReport(%q).TrafficPrivacy claim=%q anonymity=%q, want bounded reduction with no anonymity claim", target, acceptance.Claim, acceptance.AnonymityClaim)
	}
	if len(acceptance.Blockers) != 0 || acceptance.ObservedOverhead == nil {
		t.Fatalf("BuildReport(%q).TrafficPrivacy = %+v, want no blockers and observed overhead", target, acceptance)
	}
	if acceptance.ObservedOverhead.PaddingBytes != 128 || acceptance.ObservedOverhead.PaddedChunks != 2 || acceptance.ObservedOverhead.JitterBudgetMillis != 250 {
		t.Fatalf("BuildReport(%q).TrafficPrivacy.ObservedOverhead = %+v, want persisted padding and jitter evidence", target, acceptance.ObservedOverhead)
	}
	for _, want := range []string{"total_bytes", "duration", "peer_ip", "lan_presence", "supermover_use"} {
		if !containsString(acceptance.ResidualLeakage, want) {
			t.Fatalf("BuildReport(%q).TrafficPrivacy.ResidualLeakage = %#v, want %q", target, acceptance.ResidualLeakage, want)
		}
	}
}

func TestBuildReportTrafficPrivacyAcceptanceBlocksInvalidPublishedLevel2Overhead(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p, receipt := validReportNetworkPairedProfile(t, source, target)
	writeReportPairingReceipt(t, target, receipt)
	writeCompleteSession(t, target, "session-network", "docs/a.txt", []byte("hello"))
	transfer := validReportNetworkTransfer("session-network")
	transfer.SourceDeviceID = receipt.SourceDeviceID
	transfer.TargetDeviceID = receipt.TargetDeviceID
	transfer.Status = control.NetworkTransferPublished
	transfer.Stage = "commit"
	transfer.UpdatedAt = "2026-05-16T00:01:00Z"
	transfer.ErrorCode = ""
	transfer.Error = ""
	transfer.PrivacyOverhead = nil
	writeRawNetworkTransfer(t, target, transfer)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}

	if got.TrafficPrivacy.Status != "blocked" || got.TrafficPrivacy.AnonymityClaim != "not_claimed" || !containsString(got.TrafficPrivacy.Blockers, "applied_overhead_missing") {
		t.Fatalf("BuildReport(%q).TrafficPrivacy = %+v, want missing applied overhead blocker without anonymity claim", target, got.TrafficPrivacy)
	}
	if got.TrafficPrivacy.ObservedOverhead != nil {
		t.Fatalf("BuildReport(%q).TrafficPrivacy.ObservedOverhead = %+v, want nil when artifact lacks valid overhead", target, got.TrafficPrivacy.ObservedOverhead)
	}
}

func TestBuildReportTrafficPrivacyAcceptanceBlocksMismatchedProfilePolicy(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p, receipt := validReportNetworkPairedProfile(t, source, target)
	p.PrivacyPolicy.PaddingBucketBytes = 128 * 1024
	writeReportPairingReceipt(t, target, receipt)
	writeCompleteSession(t, target, "session-network", "docs/a.txt", []byte("hello"))
	transfer := validReportNetworkTransfer("session-network")
	transfer.SourceDeviceID = receipt.SourceDeviceID
	transfer.TargetDeviceID = receipt.TargetDeviceID
	transfer.Status = control.NetworkTransferPublished
	transfer.Stage = "commit"
	transfer.UpdatedAt = "2026-05-16T00:01:00Z"
	transfer.ErrorCode = ""
	transfer.Error = ""
	transfer.PrivacyOverhead = validReportLevel2PrivacyOverhead()
	writeNetworkTransfer(t, target, transfer)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}

	if got.TrafficPrivacy.Status != "blocked" || !containsString(got.TrafficPrivacy.Blockers, "privacy_policy_mismatch") {
		t.Fatalf("BuildReport(%q).TrafficPrivacy = %+v, want profile/artifact privacy policy mismatch blocker", target, got.TrafficPrivacy)
	}
}

func TestBuildReportTrafficPrivacyAcceptanceBlocksPartialAppliedOverhead(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p, receipt := validReportNetworkPairedProfile(t, source, target)
	writeReportPairingReceipt(t, target, receipt)
	writeCompleteSession(t, target, "session-network", "docs/a.txt", []byte("hello"))
	transfer := validReportNetworkTransfer("session-network")
	transfer.SourceDeviceID = receipt.SourceDeviceID
	transfer.TargetDeviceID = receipt.TargetDeviceID
	transfer.Status = control.NetworkTransferPublished
	transfer.Stage = "commit"
	transfer.UpdatedAt = "2026-05-16T00:01:00Z"
	transfer.ErrorCode = ""
	transfer.Error = ""
	transfer.PrivacyOverhead = &control.NetworkTransferPrivacyOverhead{
		JitteredRequests:   1,
		JitterDelayMillis:  10,
		JitterBudgetMillis: 250,
	}
	writeNetworkTransfer(t, target, transfer)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}

	if got.TrafficPrivacy.Status != "blocked" || !containsString(got.TrafficPrivacy.Blockers, "applied_padding_overhead_missing") {
		t.Fatalf("BuildReport(%q).TrafficPrivacy = %+v, want partial overhead blocker", target, got.TrafficPrivacy)
	}
}

func TestBuildReportTrafficPrivacyAcceptanceBlocksDeviceMismatch(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p, receipt := validReportNetworkPairedProfile(t, source, target)
	writeReportPairingReceipt(t, target, receipt)
	writeCompleteSession(t, target, "session-network", "docs/a.txt", []byte("hello"))
	transfer := validReportNetworkTransfer("session-network")
	transfer.SourceDeviceID = receipt.SourceDeviceID
	transfer.TargetDeviceID = receipt.TargetDeviceID
	transfer.Status = control.NetworkTransferPublished
	transfer.Stage = "commit"
	transfer.UpdatedAt = "2026-05-16T00:01:00Z"
	transfer.ErrorCode = ""
	transfer.Error = ""
	transfer.SourceDeviceID = "sha256:fedcba9876543210"
	transfer.PrivacyOverhead = validReportLevel2PrivacyOverhead()
	writeNetworkTransfer(t, target, transfer)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}

	if got.TrafficPrivacy.Status != "blocked" || !containsString(got.TrafficPrivacy.Blockers, "network_transfer_identity_mismatch") {
		t.Fatalf("BuildReport(%q).TrafficPrivacy = %+v, want pairing/device mismatch blocker", target, got.TrafficPrivacy)
	}
}

func TestBuildReportPairingStateReceiptMismatchRequiresReview(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := validReportPairedProfile(source, target)
	receipt := validReportPairingReceipt(p)
	receipt.TargetID = "target-other"
	writeReportPairingReceipt(t, target, receipt)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Pairing.Status != PairingStatusMismatch || got.Summary.PairingIssues != 1 {
		t.Fatalf("BuildReport(%q).Pairing=%+v summary=%+v, want mismatch pairing issue", target, got.Pairing, got.Summary)
	}
	if got.Overall.Status != StatusUnhealthy || !containsIssue(got.Overall.Issues, "pairing_issues") {
		t.Fatalf("BuildReport(%q).Overall=%+v, want unhealthy pairing issue", target, got.Overall)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "pairing_receipt" || !strings.Contains(got.ArtifactProblems[0].Error, "target_id") {
		t.Fatalf("BuildReport(%q).ArtifactProblems=%#v, want pairing receipt mismatch problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportPairingStateMissingReceiptRequiresReview(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := validReportPairedProfile(source, target)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Pairing.Status != PairingStatusReceiptMissing || got.Summary.PairingIssues != 1 {
		t.Fatalf("BuildReport(%q).Pairing=%+v summary=%+v, want missing receipt issue", target, got.Pairing, got.Summary)
	}
	if got.Overall.Status != StatusUnhealthy || len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "pairing_receipt" {
		t.Fatalf("BuildReport(%q) overall=%+v artifacts=%#v, want unhealthy pairing receipt problem", target, got.Overall, got.ArtifactProblems)
	}
}

func TestBuildReportRejectsProfileScopeMismatch(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	otherTarget := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, otherTarget)

	_, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})

	if err == nil {
		t.Fatalf("BuildReport(profile target mismatch) error = nil, want mismatch error")
	}
	if !strings.Contains(err.Error(), "target root") || !strings.Contains(err.Error(), "target.local_path") {
		t.Fatalf("BuildReport(profile target mismatch) error = %v, want target root/local path mismatch", err)
	}
}

func TestBuildReportPublishedSessionComplete(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "docs/a.txt", []byte("hello"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session-success",
		SessionID: "session-success",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{{
			Path:       "docs/a.txt",
			Kind:       "file",
			Size:       5,
			Digest:     digest([]byte("hello")),
			TargetPath: "docs/a.txt",
		}},
	})
	writePublishedReceipt(t, target, "session-success")
	writeProfileSnapshot(t, target, "session-success")
	writeSessionRecord(t, target, "session-success", transaction.StatePublished)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusVerified {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q; report=%+v", target, got.Overall.Status, StatusVerified, got)
	}
	if got.LatestSession.ID != "session-success" || got.LatestSession.Completeness.Status != CompletenessVerified {
		t.Fatalf("BuildReport(%q).LatestSession = %+v, want verified session-success", target, got.LatestSession)
	}
	if got.Summary.FilesExpected != 1 || got.Summary.FilesVerified != 1 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want one verified file", target, got.Summary)
	}
	if len(got.ArtifactProblems) != 0 || len(got.VerificationFindings) != 0 {
		t.Fatalf("BuildReport(%q) artifact_problems=%#v findings=%#v, want none", target, got.ArtifactProblems, got.VerificationFindings)
	}
	if len(got.ProfileSnapshots) != 1 || got.ProfileSnapshots[0].ID != "profile-session-success" {
		t.Fatalf("BuildReport(%q).ProfileSnapshots = %#v, want session profile snapshot", target, got.ProfileSnapshots)
	}
	snapshotPrivacy := got.ProfileSnapshots[0].Privacy
	if snapshotPrivacy.Status != "profile_snapshot_contract" ||
		snapshotPrivacy.TrafficLevel != 2 ||
		snapshotPrivacy.PaddingBucketBytes != 65536 ||
		snapshotPrivacy.BatchMaxBytes != 1048576 ||
		snapshotPrivacy.BatchMaxCount != 64 ||
		snapshotPrivacy.JitterBudgetMillis != 250 ||
		!snapshotPrivacy.DiscoveryLowInfo {
		t.Fatalf("BuildReport(%q).ProfileSnapshots[0].Privacy = %+v, want persisted level 2 bounds", target, snapshotPrivacy)
	}
	if snapshotPrivacy.Overhead.Status != "not_applied" || snapshotPrivacy.Overhead.Source != "profile_snapshot" {
		t.Fatalf("BuildReport(%q).ProfileSnapshots[0].Privacy.Overhead = %+v, want profile snapshot not_applied overhead", target, snapshotPrivacy.Overhead)
	}
}

func TestBuildReportKeepsLegacyProfileSnapshotWithoutPrivacyPolicyHealthy(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-legacy-snapshot", "docs/a.txt", []byte("hello"))
	path, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-session-legacy-snapshot")
	if err != nil {
		t.Fatalf("control.Path(profile snapshot) error = %v, want nil", err)
	}
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-session-legacy-snapshot",
		ProfileID:  "profile-local",
		SessionID:  "session-legacy-snapshot",
		CapturedAt: "2026-05-16T00:00:00Z",
		Profile:    []byte(`{"profile_id":"profile-local","roots":[{"id":"root"}],"target":{"target_id":"target-local"}}`),
	}
	if err := control.WriteFile(path, snapshot); err != nil {
		t.Fatalf("control.WriteFile(%q, snapshot) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusVerified || len(got.ArtifactProblems) != 0 {
		t.Fatalf("BuildReport(%q) status=%q artifact_problems=%#v, want legacy snapshot accepted", target, got.Overall.Status, got.ArtifactProblems)
	}
	if len(got.ProfileSnapshots) != 1 || got.ProfileSnapshots[0].Privacy.Status != "profile_snapshot_unavailable" {
		t.Fatalf("BuildReport(%q).ProfileSnapshots = %#v, want legacy snapshot with unavailable privacy evidence", target, got.ProfileSnapshots)
	}
}

func TestBuildReportFlagsMissingProfileSnapshot(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-missing-snapshot", "docs/a.txt", []byte("hello"))
	path, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-session-missing-snapshot")
	if err != nil {
		t.Fatalf("control.Path(profile snapshot) error = %v, want nil", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("os.Remove(%q) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	if len(got.ProfileSnapshots) != 0 {
		t.Fatalf("BuildReport(%q).ProfileSnapshots = %#v, want no valid snapshots", target, got.ProfileSnapshots)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "profile_snapshot" {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want profile snapshot problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportFlagsCorruptProfileSnapshot(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-corrupt-snapshot", "docs/a.txt", []byte("hello"))
	path, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-session-corrupt-snapshot")
	if err != nil {
		t.Fatalf("control.Path(profile snapshot) error = %v, want nil", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy || len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "profile_snapshot" {
		t.Fatalf("BuildReport(%q) status=%q artifact_problems=%#v, want corrupt profile snapshot problem", target, got.Overall.Status, got.ArtifactProblems)
	}
}

func TestBuildReportFlagsEmbeddedProfileSnapshotTargetMismatch(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-wrong-target", "docs/a.txt", []byte("hello"))
	path, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-session-wrong-target")
	if err != nil {
		t.Fatalf("control.Path(profile snapshot) error = %v, want nil", err)
	}
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-session-wrong-target",
		ProfileID:  "profile-local",
		SessionID:  "session-wrong-target",
		CapturedAt: "2026-05-16T00:00:00Z",
		Profile:    []byte(`{"profile_id":"profile-local","roots":[{"id":"root"}],"target":{"target_id":"target-other"}}`),
	}
	if err := control.WriteFile(path, snapshot); err != nil {
		t.Fatalf("control.WriteFile(%q, snapshot) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy || len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "profile_snapshot" {
		t.Fatalf("BuildReport(%q) status=%q artifact_problems=%#v, want embedded snapshot target problem", target, got.Overall.Status, got.ArtifactProblems)
	}
	if !strings.Contains(got.ArtifactProblems[0].Error, "embedded target_id") {
		t.Fatalf("BuildReport(%q).ArtifactProblems[0].Error = %q, want embedded target_id mismatch", target, got.ArtifactProblems[0].Error)
	}
}

func TestBuildReportFlagsForeignPublishedReceipts(t *testing.T) {
	target := t.TempDir()
	writeTargetFile(t, target, "docs/a.txt", []byte("hello"))
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-foreign",
		SessionID: "session-foreign",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []control.ManifestEntry{{
			Path:       "docs/a.txt",
			Kind:       "file",
			Size:       5,
			Digest:     digest([]byte("hello")),
			TargetPath: "docs/a.txt",
		}},
	})
	writePublishedReceiptForScope(t, target, "session-foreign", "profile-other", "target-other")

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q, profile-local) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q, profile-local).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	if got.Summary.ManifestCount != 0 {
		t.Fatalf("BuildReport(%q, profile-local).Summary.ManifestCount = %d, want 0 matching manifests", target, got.Summary.ManifestCount)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "scope" {
		t.Fatalf("BuildReport(%q, profile-local).ArtifactProblems = %#v, want foreign scope problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportWarningSuggestions(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-warning", "docs/a.txt", []byte("hello"))
	writeWarning(t, target, control.Warning{
		Version:    control.CurrentVersion,
		ID:         "warning-suggestion",
		SessionID:  "session-warning",
		Code:       "special_file",
		Message:    "path needs additional migration config",
		Severity:   "warning",
		Paths:      []string{"docs/socket"},
		TargetPath: "docs/socket",
		SuggestedProfilePatch: map[string]string{
			"include_special_files": "true",
		},
		SuggestedConfig: map[string]string{
			"special_files": "manual_review",
		},
		CreatedAt: "2026-05-16T00:02:00Z",
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusAttention {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusAttention)
	}
	if len(got.Warnings) != 1 || got.Warnings[0].SuggestedProfilePatch["include_special_files"] != "true" {
		t.Fatalf("BuildReport(%q).Warnings = %#v, want warning with suggested profile patch", target, got.Warnings)
	}
	if len(got.ProfileSuggestions) != 1 {
		t.Fatalf("BuildReport(%q).ProfileSuggestions length = %d, want 1", target, len(got.ProfileSuggestions))
	}
	suggestion := got.ProfileSuggestions[0]
	if suggestion.WarningID != "warning-suggestion" || suggestion.Code != "special_file" || suggestion.SuggestedConfig["special_files"] != "manual_review" {
		t.Fatalf("BuildReport(%q).ProfileSuggestions[0] = %+v, want filterable warning suggestion", target, suggestion)
	}
}

func TestBuildReportSoftDeletes(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-soft-delete", "keep.txt", []byte("keep"))
	writeSoftDelete(t, target, control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "session-soft-delete-del_001",
		SessionID:          "session-soft-delete",
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

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusAttention {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusAttention)
	}
	if got.Summary.SoftDeletes != 1 || len(got.SoftDeletes) != 1 || got.SoftDeletes[0].TargetPath != "gone.txt" {
		t.Fatalf("BuildReport(%q).SoftDeletes = %#v summary=%+v, want inspectable soft delete", target, got.SoftDeletes, got.Summary)
	}
}

func TestBuildReportPruneReviewShowsCandidatesAndPolicy(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	record := writeReportPruneCandidateFixture(t, target)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.PruneReview.Status != PruneReviewReviewRequired || got.PruneReview.ProfileDeletePolicy == nil {
		t.Fatalf("BuildReport(%q).PruneReview = %+v, want review-required policy surface", target, got.PruneReview)
	}
	if got.PruneReview.ProfileDeletePolicy.Mode != "prune" || !got.PruneReview.ProfileDeletePolicy.RequireReview || !got.PruneReview.ProfileDeletePolicy.AllowPhysicalPrune {
		t.Fatalf("BuildReport(%q).PruneReview.ProfileDeletePolicy = %+v, want profile prune policy", target, got.PruneReview.ProfileDeletePolicy)
	}
	if got.Summary.PruneCandidates != 1 || got.Summary.PruneRefusals != 0 || got.Summary.PruneReceipts != 0 || got.Summary.PruneArtifactProblems != 0 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want one prune candidate", target, got.Summary)
	}
	if len(got.PruneReview.Candidates) != 1 || got.PruneReview.Candidates[0].SoftDeleteID != record.ID || got.PruneReview.Candidates[0].ApprovalWriting != "not_written_by_dry_run" {
		t.Fatalf("BuildReport(%q).PruneReview.Candidates = %+v, want dry-run candidate evidence", target, got.PruneReview.Candidates)
	}
	if !containsIssue(got.Overall.Issues, "prune_candidates") {
		t.Fatalf("BuildReport(%q).Overall.Issues = %#v, want prune_candidates issue", target, got.Overall.Issues)
	}
}

func TestBuildReportPruneReviewShowsApprovalInventoryWithoutMutatingTarget(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	record := writeReportPruneCandidateFixture(t, target)
	approval := reportPruneApproval(t, target, "approval-report", p, record)
	writeReportPruneApproval(t, target, approval)
	before := snapshotControlPlane(t, target)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	after := snapshotControlPlane(t, target)

	if got.PruneReview.Status != PruneReviewReviewRequired || got.Summary.PruneApprovals != 1 || got.Summary.PruneUnappliedApprovals != 1 {
		t.Fatalf("BuildReport(%q).PruneReview = %+v summary=%+v, want one unapplied approval requiring review", target, got.PruneReview, got.Summary)
	}
	if len(got.PruneReview.Approvals) != 1 {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals = %+v, want one approval", target, got.PruneReview.Approvals)
	}
	gotApproval := got.PruneReview.Approvals[0]
	if gotApproval.ID != "approval-report" || gotApproval.Status != "approved" || !gotApproval.Unapplied || gotApproval.PhysicalPruning != "not_applied" || gotApproval.Action != "inspect_prune_approval_before_apply" {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals[0] = %+v, want pending read-only approval evidence", target, gotApproval)
	}
	if gotApproval.ReleaseState != "active" || gotApproval.ReleaseBlocker || gotApproval.ReleaseAction != "ready_for_prune_apply" || len(gotApproval.CurrentEvidence) != 1 || gotApproval.CurrentEvidence[0].State != "current" {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals[0] release evidence = %+v current=%+v, want active current approval", target, gotApproval, gotApproval.CurrentEvidence)
	}
	if got.PruneReview.Summary.ActiveApprovals != 1 || got.PruneReview.Summary.StaleApprovals != 0 || got.PruneReview.Summary.ExpiredApprovals != 0 || got.PruneReview.Summary.ConsumedApprovals != 0 {
		t.Fatalf("BuildReport(%q).PruneReview.Summary = %+v, want active approval count", target, got.PruneReview.Summary)
	}
	if gotApproval.ApprovedBy != "report-reviewer" || gotApproval.ProfileSnapshotID == "" || gotApproval.ProfileSnapshotPath == "" || len(gotApproval.Items) != 1 || gotApproval.Items[0].SoftDeleteID != record.ID {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals[0] = %+v, want scoped approval item evidence", target, gotApproval)
	}
	if got.Summary.PruneReceipts != 0 {
		t.Fatalf("BuildReport(%q).Summary.PruneReceipts = %d, want no receipt written", target, got.Summary.PruneReceipts)
	}
	receiptPath, err := control.Path(target, control.ArtifactPruneReceipt, "approval-report")
	if err != nil {
		t.Fatalf("control.Path(prune receipt) error = %v, want nil", err)
	}
	if _, err := os.Lstat(receiptPath); !os.IsNotExist(err) {
		t.Fatalf("Lstat(%q) error = %v, want no receipt", receiptPath, err)
	}
	if _, err := os.Lstat(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("Lstat(target gone.txt) error = %v, want target file untouched", err)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("BuildReport(%q) mutated control plane\nbefore=%#v\nafter=%#v", target, before, after)
	}
	if !containsIssue(got.Overall.Issues, "prune_unapplied_approvals") {
		t.Fatalf("BuildReport(%q).Overall.Issues = %#v, want prune_unapplied_approvals issue", target, got.Overall.Issues)
	}
}

func TestBuildReportPruneReviewMarksStaleApprovalWithoutMutatingTarget(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	record := writeReportPruneCandidateFixture(t, target)
	approval := reportPruneApproval(t, target, "approval-stale", p, record)
	writeReportPruneApproval(t, target, approval)
	if err := os.WriteFile(filepath.Join(target, "gone.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(changed target) error = %v, want nil", err)
	}
	before := snapshotControlPlane(t, target)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	after := snapshotControlPlane(t, target)

	if got.PruneReview.Status != PruneReviewReviewRequired || got.Summary.PruneStaleApprovals != 1 || got.PruneReview.Summary.StaleApprovals != 1 {
		t.Fatalf("BuildReport(%q).PruneReview = %+v summary=%+v, want stale approval review", target, got.PruneReview, got.Summary)
	}
	if len(got.PruneReview.Approvals) != 1 {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals = %+v, want one approval", target, got.PruneReview.Approvals)
	}
	gotApproval := got.PruneReview.Approvals[0]
	if gotApproval.ReleaseState != "stale" || !gotApproval.ReleaseBlocker || gotApproval.ReleaseAction != "author_new_prune_approval" || gotApproval.ReleaseReason == "" {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals[0] = %+v, want stale release blocker", target, gotApproval)
	}
	if len(gotApproval.CurrentEvidence) != 1 || gotApproval.CurrentEvidence[0].State != "refused" || gotApproval.CurrentEvidence[0].ReasonCode != prune.ReasonTargetContentMismatch {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals[0].CurrentEvidence = %+v, want current refusal evidence", target, gotApproval.CurrentEvidence)
	}
	if !containsIssue(got.Overall.Issues, "prune_stale_approvals") {
		t.Fatalf("BuildReport(%q).Overall.Issues = %#v, want prune_stale_approvals issue", target, got.Overall.Issues)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("BuildReport(%q) mutated control plane\nbefore=%#v\nafter=%#v", target, before, after)
	}
}

func TestBuildReportPruneReviewMarksApprovalStaleWhenCandidateDisappears(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	record := writeReportPruneCandidateFixture(t, target)
	approval := reportPruneApproval(t, target, "approval-missing-current", p, record)
	writeReportPruneApproval(t, target, approval)
	if err := os.Remove(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("os.Remove(target gone) error = %v, want nil", err)
	}
	before := snapshotControlPlane(t, target)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	after := snapshotControlPlane(t, target)

	if got.PruneReview.Summary.StaleApprovals != 1 {
		t.Fatalf("BuildReport(%q).PruneReview.Summary = %+v, want stale approval count", target, got.PruneReview.Summary)
	}
	if len(got.PruneReview.Approvals) != 1 {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals = %+v, want one approval", target, got.PruneReview.Approvals)
	}
	approvalView := got.PruneReview.Approvals[0]
	if approvalView.ReleaseState != "stale" || !approvalView.ReleaseBlocker || len(approvalView.CurrentEvidence) != 1 || approvalView.CurrentEvidence[0].State != "refused" || approvalView.CurrentEvidence[0].ReasonCode != prune.ReasonTargetMissing {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals[0] = %+v current=%+v, want stale target-missing evidence", target, approvalView, approvalView.CurrentEvidence)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("BuildReport(%q) mutated control plane\nbefore=%#v\nafter=%#v", target, before, after)
	}
}

func TestBuildPruneReviewKeepsSessionFilterSeparateFromLatestSession(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	record := writeReportPruneCandidateFixture(t, target)
	before := snapshotControlPlane(t, target)

	got, err := BuildPruneReview(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildPruneReview(%q) error = %v, want nil", target, err)
	}
	after := snapshotControlPlane(t, target)

	if got.SessionFilter != "" || got.LatestSessionID != "session-two" {
		t.Fatalf("BuildPruneReview(%q) scope = %+v, want no filter and latest session evidence", target, got)
	}
	if got.PruneReview.Status != PruneReviewReviewRequired || !got.PruneReview.NeedsReview() || got.PruneReview.ReviewAction() != "inspect_prune_review_before_release" {
		t.Fatalf("BuildPruneReview(%q).PruneReview = %+v, want review-required action", target, got.PruneReview)
	}
	if len(got.PruneReview.Candidates) != 1 || got.PruneReview.Candidates[0].SoftDeleteID != record.ID {
		t.Fatalf("BuildPruneReview(%q).PruneReview.Candidates = %+v, want current candidate evidence", target, got.PruneReview.Candidates)
	}
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("BuildPruneReview(%q) mutated control plane\nbefore=%#v\nafter=%#v", target, before, after)
	}
}

func TestBuildReportPruneReviewLinksAppliedApprovalInventory(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	record := writeReportPruneCandidateFixture(t, target)
	approval := reportPruneApproval(t, target, "approval-report", p, record)
	writeReportPruneApproval(t, target, approval)
	writeReportPruneReceiptWithApproval(t, target, "receipt-report", control.PruneReceiptApplied, approval.ID, approval.ApprovalScopeDigest, []control.PruneReceiptItem{appliedReportPruneItem(record)})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.PruneApprovals != 1 || got.Summary.PruneUnappliedApprovals != 0 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want applied approval inventoried but not pending", target, got.Summary)
	}
	if len(got.PruneReview.Approvals) != 1 || got.PruneReview.Approvals[0].Unapplied || got.PruneReview.Approvals[0].LinkedReceiptID != "receipt-report" || got.PruneReview.Approvals[0].LinkedReceiptStatus != control.PruneReceiptApplied {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals = %+v, want approval linked to applied receipt", target, got.PruneReview.Approvals)
	}
	if got.PruneReview.Approvals[0].ReleaseState != "consumed" || got.PruneReview.Approvals[0].ReleaseBlocker || got.PruneReview.Summary.ConsumedApprovals != 1 {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals = %+v summary=%+v, want consumed approval evidence", target, got.PruneReview.Approvals, got.PruneReview.Summary)
	}
}

func TestBuildReportPruneReviewLinksFailedReceiptToApproval(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	record := writeReportPruneCandidateFixture(t, target)
	approval := reportPruneApproval(t, target, "approval-report", p, record)
	writeReportPruneApproval(t, target, approval)
	writeReportPruneReceiptWithApproval(t, target, "receipt-report-failed", control.PruneReceiptFailed, approval.ID, approval.ApprovalScopeDigest, []control.PruneReceiptItem{failedReportPruneItem(record)})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.PruneApprovals != 1 || got.Summary.PruneUnappliedApprovals != 0 || got.Summary.PruneReceipts != 1 || got.Summary.PruneReceiptIssues != 1 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want failed linked receipt counted as receipt attention", target, got.Summary)
	}
	if len(got.PruneReview.Approvals) != 1 {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals = %+v, want one approval", target, got.PruneReview.Approvals)
	}
	view := got.PruneReview.Approvals[0]
	if view.LinkedReceiptID != "receipt-report-failed" || view.LinkedReceiptStatus != control.PruneReceiptFailed || view.Unapplied || view.ReleaseState != "failed_receipt" || !view.ReleaseBlocker || view.ReleaseAction != "inspect_prune_receipt" {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals[0] = %+v, want failed receipt-linked approval attention", target, view)
	}
	if view.SupersededBy != "" || view.SupersededAt != "" {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals[0] supersede metadata = %+v, want empty for non-superseded approval", target, view)
	}
	if !containsIssue(got.Overall.Issues, "prune_receipt_issues") {
		t.Fatalf("BuildReport(%q).Overall.Issues = %#v, want prune_receipt_issues issue", target, got.Overall.Issues)
	}
}

func TestBuildReportSkipsOutOfScopePruneApprovalBeforePathIDProblems(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	record := writeReportPruneCandidateFixture(t, target)
	approval := reportPruneApproval(t, target, "approval-foreign", p, record)
	approval.ID = "approval-foreign-actual"
	approval.ProfileID = "profile-foreign"
	approval.TargetID = "target-foreign"
	path := filepath.Join(target, control.DirName, "prune", "approvals", "approval-foreign-path.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := control.WriteNewFile(path, approval); err != nil {
		t.Fatalf("control.WriteNewFile(%q, foreign approval) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.PruneApprovals != 0 || len(got.PruneReview.Approvals) != 0 {
		t.Fatalf("BuildReport(%q).PruneReview.Approvals = %+v summary=%+v, want out-of-scope approval skipped", target, got.PruneReview.Approvals, got.Summary)
	}
	for _, problem := range got.ArtifactProblems {
		if problem.Source == "prune_approval" {
			t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want foreign path/id mismatch skipped before current-scope problem reporting", target, got.ArtifactProblems)
		}
	}
}

func TestBuildReportPruneReviewSurfacesReceiptsAndFiltersResolvedCandidates(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	record := writeReportPruneCandidateFixture(t, target)
	writeReportPruneReceipt(t, target, control.PruneReceiptApplied, []control.PruneReceiptItem{appliedReportPruneItem(record)})
	if err := os.Remove(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("os.Remove(target gone) error = %v, want nil", err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.PruneReceipts != 1 || got.Summary.PruneCandidates != 0 || got.Summary.PruneRefusals != 0 || got.Summary.PruneReceiptIssues != 0 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want applied receipt without pending target-missing prune review", target, got.Summary)
	}
	if got.PruneReview.Status != PruneReviewNoPendingReview {
		t.Fatalf("BuildReport(%q).PruneReview.Status = %q, want %q", target, got.PruneReview.Status, PruneReviewNoPendingReview)
	}
	if len(got.PruneReview.Receipts) != 1 || got.PruneReview.Receipts[0].Status != control.PruneReceiptApplied || got.PruneReview.Receipts[0].Action != "inspect_applied_prune_receipt" {
		t.Fatalf("BuildReport(%q).PruneReview.Receipts = %+v, want applied receipt evidence", target, got.PruneReview.Receipts)
	}
}

func TestBuildReportAppliedPruneReceiptDoesNotSuppressRecreatedTarget(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	record := writeReportPruneCandidateFixture(t, target)
	writeReportPruneReceipt(t, target, control.PruneReceiptApplied, []control.PruneReceiptItem{appliedReportPruneItem(record)})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.PruneCandidates != 1 || got.PruneReview.Status != PruneReviewReviewRequired {
		t.Fatalf("BuildReport(%q).PruneReview = %+v summary=%+v, want recreated target still pending review", target, got.PruneReview, got.Summary)
	}
}

func TestBuildReportPruneReviewFlagsFailedReceiptsAndManualDeletion(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	record := writeReportPruneCandidateFixture(t, target)
	failed := failedReportPruneItem(record)
	writeReportPruneReceipt(t, target, control.PruneReceiptFailed, []control.PruneReceiptItem{failed})
	if err := os.Remove(filepath.Join(target, "gone.txt")); err != nil {
		t.Fatalf("os.Remove(target gone) error = %v, want nil", err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.PruneReceipts != 1 || got.Summary.PruneReceiptIssues != 1 || got.Summary.PruneRefusals != 1 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want failed receipt plus missing-target refusal", target, got.Summary)
	}
	if got.PruneReview.Status != PruneReviewReviewRequired {
		t.Fatalf("BuildReport(%q).PruneReview.Status = %q, want review-required manual deletion surface", target, got.PruneReview.Status)
	}
	if got.PruneReview.Refusals[0].ReasonCode != prune.ReasonTargetMissing {
		t.Fatalf("BuildReport(%q).PruneReview.Refusals = %+v, want target_missing refusal", target, got.PruneReview.Refusals)
	}
	if !containsIssue(got.Overall.Issues, "prune_refusals") || !containsIssue(got.Overall.Issues, "prune_receipt_issues") {
		t.Fatalf("BuildReport(%q).Overall.Issues = %#v, want prune refusal and receipt issue", target, got.Overall.Issues)
	}
}

func TestBuildReportPruneApprovalArtifactProblems(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	writeReportPruneCandidateFixture(t, target)
	path := filepath.Join(target, control.DirName, "prune", "approvals", "bad.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy || got.Summary.PruneArtifactProblems != 1 || got.Summary.ArtifactProblems == 0 {
		t.Fatalf("BuildReport(%q) status=%q summary=%+v artifacts=%#v, want prune approval artifact problem", target, got.Overall.Status, got.Summary, got.ArtifactProblems)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "prune_approval" {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want prune_approval problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportPruneReceiptArtifactProblems(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	writeReportPruneCandidateFixture(t, target)
	path := filepath.Join(target, control.DirName, "prune", "receipts", "bad.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy || got.Summary.PruneArtifactProblems != 1 || got.Summary.ArtifactProblems == 0 {
		t.Fatalf("BuildReport(%q) status=%q summary=%+v artifacts=%#v, want prune artifact problem", target, got.Overall.Status, got.Summary, got.ArtifactProblems)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "prune_receipt" {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want prune_receipt problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportRejectsPruneReceiptDirectorySymlinkBoundary(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	outside := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	writeReportPruneCandidateFixture(t, target)
	receiptDir := filepath.Join(target, control.DirName, "prune", "receipts")
	if err := os.MkdirAll(filepath.Join(target, control.DirName, "prune"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(prune dir) error = %v, want nil", err)
	}
	if err := os.RemoveAll(receiptDir); err != nil {
		t.Fatalf("os.RemoveAll(receipt dir) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Join(outside, "receipts"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(outside receipts) error = %v, want nil", err)
	}
	if err := os.Symlink(filepath.Join(outside, "receipts"), receiptDir); err != nil {
		t.Fatalf("os.Symlink(receipt dir) error = %v, want nil", err)
	}

	_, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, Profile: &p})
	if err == nil || !strings.Contains(err.Error(), "control artifact path must not be a symlink") {
		t.Fatalf("BuildReport(%q) error = %v, want unsafe prune receipt boundary error", target, err)
	}
}

func TestBuildReportPruneReceiptSessionFilterUsesSoftDeleteEvidence(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.DeletePolicy.Mode = profile.DeleteModePrune
	p.DeletePolicy.RequireReview = true
	p.DeletePolicy.RetentionDays = 0
	p.DeletePolicy.AllowPhysicalPrune = true
	selected := writeReportPruneCandidateFixture(t, target)
	other := selected
	other.ID = "session-other-del_001"
	other.SessionID = "session-other"
	other.TargetPath = "other.txt"
	other.SourcePath = "other.txt"
	writeSoftDelete(t, target, other)
	writeReportPruneReceipt(t, target, control.PruneReceiptApplied, []control.PruneReceiptItem{appliedReportPruneItem(selected)})
	otherReceipt := appliedReportPruneItem(other)
	otherReceipt.SoftDeleteID = other.ID
	otherReceipt.TargetPath = other.TargetPath
	otherReceipt.PrePruneObserved.Path = other.TargetPath
	writeReportPruneReceiptWithID(t, target, "prune-other", control.PruneReceiptApplied, []control.PruneReceiptItem{otherReceipt})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: p.ProfileID, TargetID: p.Target.TargetID, SessionID: selected.SessionID, Profile: &p})
	if err != nil {
		t.Fatalf("BuildReport(%q, selected) error = %v, want nil", target, err)
	}
	if got.Summary.PruneReceipts != 1 || len(got.PruneReview.Receipts) != 1 || got.PruneReview.Receipts[0].ID != "prune-apply" || len(got.PruneReview.Receipts[0].Items) != 1 {
		t.Fatalf("BuildReport(%q, selected).PruneReview.Receipts = %+v summary=%+v, want selected receipt only", target, got.PruneReview.Receipts, got.Summary)
	}
	if got.PruneReview.Receipts[0].Items[0].SoftDeleteID != selected.ID {
		t.Fatalf("BuildReport(%q, selected).PruneReview.Receipts[0].Items = %+v, want selected soft delete only", target, got.PruneReview.Receipts[0].Items)
	}
}

func TestBuildReportTargetDrifts(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-drift", "keep.txt", []byte("keep"))
	writeTargetDrift(t, target, control.TargetDrift{
		Version:    control.CurrentVersion,
		ID:         "session-drift-drift_keep",
		SessionID:  "session-drift",
		ProfileID:  "profile-local",
		TargetID:   "target-local",
		RootID:     "root",
		Path:       "keep.txt",
		DetectedAt: "2026-05-16T00:01:00Z",
		Change:     "content_mismatch",
		Evidence:   []string{"target content differs from staged manifest"},
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy || !containsIssue(got.Overall.Issues, "target_drifts") {
		t.Fatalf("BuildReport(%q) overall=%+v, want unhealthy target drift review", target, got.Overall)
	}
	if got.Summary.TargetDrifts != 1 || len(got.TargetDrifts) != 1 || got.TargetDrifts[0].Path != "keep.txt" {
		t.Fatalf("BuildReport(%q).TargetDrifts = %#v summary=%+v, want inspectable drift evidence", target, got.TargetDrifts, got.Summary)
	}
	if got.LatestSession.Completeness.Status != CompletenessVerified {
		t.Fatalf("BuildReport(%q).LatestSession.Completeness = %+v, want manifest verification separate from drift review", target, got.LatestSession.Completeness)
	}
}

func TestBuildReportResolvedTargetDriftDoesNotRequireReview(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-drift", "keep.txt", []byte("keep"))
	writeTargetDrift(t, target, control.TargetDrift{
		Version:      control.CurrentVersion,
		ID:           "session-drift-resolved_keep",
		SessionID:    "session-drift",
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

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.TargetDrifts != 0 || len(got.TargetDrifts) != 0 || got.Health.Summary.TargetDrifts != 0 {
		t.Fatalf("BuildReport(%q) target drifts summary=%+v health=%+v drifts=%#v, want resolved drift excluded from review counts", target, got.Summary, got.Health.Summary, got.TargetDrifts)
	}
	if got.Overall.Status != StatusVerified || containsIssue(got.Overall.Issues, "target_drifts") {
		t.Fatalf("BuildReport(%q) overall=%+v, want verified without target_drifts issue", target, got.Overall)
	}
}

func TestBuildReportLiveTargetDriftIsSeparateAndNonDurable(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-live-drift", "keep.txt", []byte("keep"))
	writeTargetFile(t, target, "extra.txt", []byte("target-only"))
	before := snapshotControlPlane(t, target)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusAttention || !containsIssue(got.Overall.Issues, "live_target_drifts") {
		t.Fatalf("BuildReport(%q) overall=%+v live=%+v, want review-required live drift issue", target, got.Overall, got.LiveTargetDrift)
	}
	if got.Summary.TargetDrifts != 0 || len(got.TargetDrifts) != 0 {
		t.Fatalf("BuildReport(%q) persisted target drift summary=%+v drifts=%#v, want no persisted drift evidence", target, got.Summary, got.TargetDrifts)
	}
	if got.Summary.LiveTargetDrifts != 1 || got.Summary.LiveTargetDriftProblems != 0 {
		t.Fatalf("BuildReport(%q) live summary=%+v live=%+v, want one live drift and no live artifact problems", target, got.Summary, got.LiveTargetDrift)
	}
	if got.LiveTargetDrift.Source != "live_detector" || got.LiveTargetDrift.Durable {
		t.Fatalf("BuildReport(%q).LiveTargetDrift=%+v, want non-durable live detector source", target, got.LiveTargetDrift)
	}
	if len(got.LiveTargetDrift.TargetDrifts) != 1 || got.LiveTargetDrift.TargetDrifts[0].Path != "extra.txt" || got.LiveTargetDrift.TargetDrifts[0].Change != "extra" {
		t.Fatalf("BuildReport(%q).LiveTargetDrift.TargetDrifts=%#v, want extra.txt live drift", target, got.LiveTargetDrift.TargetDrifts)
	}
	after := snapshotControlPlane(t, target)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("BuildReport(%q) mutated control plane\nbefore=%#v\nafter=%#v", target, before, after)
	}
	driftDir := filepath.Join(target, control.DirName, "drift")
	entries, err := os.ReadDir(driftDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil or missing drift directory", driftDir, err)
	}
	if len(entries) != 0 {
		t.Fatalf("os.ReadDir(%q) = %d entries, want report live detector to remain non-persistent", driftDir, len(entries))
	}
}

func TestBuildReportLiveTargetDriftKeepsPersistedDriftSeparate(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-drift", "keep.txt", []byte("keep"))
	writeTargetFile(t, target, "extra.txt", []byte("target-only"))
	writeTargetDrift(t, target, control.TargetDrift{
		Version:    control.CurrentVersion,
		ID:         "session-drift-persisted_keep",
		SessionID:  "session-drift",
		ProfileID:  "profile-local",
		TargetID:   "target-local",
		RootID:     "root",
		Path:       "keep.txt",
		DetectedAt: "2026-05-16T00:01:00Z",
		Change:     "content_mismatch",
		Evidence:   []string{"persisted drift evidence"},
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.TargetDrifts != 1 || len(got.TargetDrifts) != 1 {
		t.Fatalf("BuildReport(%q) persisted drifts summary=%+v drifts=%#v, want one persisted drift", target, got.Summary, got.TargetDrifts)
	}
	if got.Summary.LiveTargetDrifts != 1 || len(got.LiveTargetDrift.TargetDrifts) != 1 {
		t.Fatalf("BuildReport(%q) live drift summary=%+v live=%+v, want one separate live drift", target, got.Summary, got.LiveTargetDrift)
	}
	if !containsIssue(got.Overall.Issues, "target_drifts") || !containsIssue(got.Overall.Issues, "live_target_drifts") {
		t.Fatalf("BuildReport(%q) issues=%#v, want persisted and live drift issues", target, got.Overall.Issues)
	}
}

func TestBuildReportFiltersForeignHealthTargetDrifts(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-clean", "keep.txt", []byte("keep"))
	writeTargetDrift(t, target, control.TargetDrift{
		Version:    control.CurrentVersion,
		ID:         "session-foreign-drift",
		SessionID:  "session-foreign",
		ProfileID:  "foreign-profile",
		TargetID:   "foreign-target",
		RootID:     "root",
		Path:       "foreign.txt",
		DetectedAt: "2026-05-16T00:01:00Z",
		Change:     "content_mismatch",
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Summary.TargetDrifts != 0 || len(got.TargetDrifts) != 0 || got.Health.Summary.TargetDrifts != 0 {
		t.Fatalf("BuildReport(%q) target drifts summary=%+v health=%+v drifts=%#v, want foreign drift filtered", target, got.Summary, got.Health.Summary, got.TargetDrifts)
	}
	if got.Overall.Status != StatusVerified || !got.Health.Healthy {
		t.Fatalf("BuildReport(%q) overall=%+v health=%+v, want verified scoped report", target, got.Overall, got.Health)
	}
}

func TestBuildReportShowsNetworkTransferIssue(t *testing.T) {
	target := t.TempDir()
	writeNetworkTransfer(t, target, control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       "session-network",
		ProfileID:       "profile-local",
		TargetID:        "target-local",
		SourceDeviceID:  "sha256:abcdef0123456789",
		TargetDeviceID:  "sha256:0123456789abcdef",
		ProtocolVersion: "supermover/1",
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
		Status:    control.NetworkTransferNeedsRepair,
		Stage:     "status",
		StartedAt: "2026-05-16T00:00:00Z",
		UpdatedAt: "2026-05-16T00:00:01Z",
		ErrorCode: "receiver_needs_repair",
		Error:     "receiver reported needs_repair",
		Attempts: []control.NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: "2026-05-16T00:00:00Z",
			EndedAt:   "2026-05-16T00:00:01Z",
			Stage:     "status",
			Status:    control.NetworkTransferNeedsRepair,
			ErrorCode: "receiver_needs_repair",
			Error:     "receiver reported needs_repair",
		}},
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy || !containsIssue(got.Overall.Issues, "network_transfers") {
		t.Fatalf("BuildReport(%q).Overall = %+v, want unhealthy network transfer issue", target, got.Overall)
	}
	if got.Summary.NetworkTransfers != 1 || got.Health.Summary.NetworkTransfers != 1 || len(got.NetworkTransfers) != 1 || len(got.Health.Transfers) != 1 {
		t.Fatalf("BuildReport(%q) summary=%+v health=%+v transfers=%#v, want one surfaced network transfer", target, got.Summary, got.Health, got.NetworkTransfers)
	}
	transfer := got.NetworkTransfers[0]
	if transfer.Status != "needs_repair" || transfer.Action != "review_receiver_repair_state_before_retry" || transfer.Path == "" {
		t.Fatalf("BuildReport(%q).NetworkTransfers[0] = %+v, want receiver repair review action and artifact path", target, transfer)
	}
	if transfer.Privacy != transport.DefaultPrivacyPolicy(transport.PrivacyLevel2) {
		t.Fatalf("BuildReport(%q).NetworkTransfers[0].Privacy = %+v, want level 2 transfer policy evidence", target, transfer.Privacy)
	}
	if transfer.Overhead == nil || transfer.Overhead.PaddingBytes != 128 || transfer.Overhead.PaddedChunks != 2 {
		t.Fatalf("BuildReport(%q).NetworkTransfers[0].Overhead = %+v, want persisted padding overhead evidence", target, transfer.Overhead)
	}
	if transfer.Overhead.BatchFrames != 1 || transfer.Overhead.BatchedChunks != 2 || transfer.Overhead.MaxBatchCount != 2 || transfer.Overhead.MaxBatchPlainBytes != 512 {
		t.Fatalf("BuildReport(%q).NetworkTransfers[0].Overhead = %+v, want persisted batch overhead evidence", target, transfer.Overhead)
	}
	if transfer.Overhead.JitteredRequests != 3 || transfer.Overhead.JitterDelayMillis != 210 || transfer.Overhead.MaxJitterDelayMillis != 125 || transfer.Overhead.JitterBudgetMillis != 250 {
		t.Fatalf("BuildReport(%q).NetworkTransfers[0].Overhead = %+v, want persisted jitter overhead evidence", target, transfer.Overhead)
	}
	if got.Health.Transfers[0].Overhead == nil || got.Health.Transfers[0].Overhead.JitteredRequests != 3 || got.Health.Transfers[0].Overhead.JitterBudgetMillis != 250 {
		t.Fatalf("BuildReport(%q).Health.Transfers[0].Overhead = %+v, want persisted jitter overhead evidence", target, got.Health.Transfers[0].Overhead)
	}
}

func TestBuildReportSessionFilterAppliesToNetworkTransfers(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-selected", "docs/a.txt", []byte("hello"))
	writeNetworkTransfer(t, target, validReportNetworkTransfer("session-other"))

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local", SessionID: "session-selected"})
	if err != nil {
		t.Fatalf("BuildReport(%q, selected) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusVerified || got.Summary.NetworkTransfers != 0 || len(got.NetworkTransfers) != 0 || len(got.Health.Transfers) != 0 {
		t.Fatalf("BuildReport(%q, selected) overall=%+v summary=%+v transfers=%#v health=%+v, want selected session verified without foreign transfer", target, got.Overall, got.Summary, got.NetworkTransfers, got.Health)
	}
}

func TestBuildReportStartedChunkTransferRequiresReviewUntilPublished(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-network", "docs/a.txt", []byte("hello"))
	started := validReportNetworkTransfer("session-network")
	started.Status = control.NetworkTransferStarted
	started.Stage = "chunk"
	started.ErrorCode = ""
	started.Error = ""
	started.PrivacyOverhead = validReportLevel2PrivacyOverhead()
	writeNetworkTransfer(t, target, started)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q, started) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy || !containsIssue(got.Overall.Issues, "network_transfers") {
		t.Fatalf("BuildReport(%q, started).Overall = %+v, want unhealthy network transfer review", target, got.Overall)
	}
	if got.Summary.NetworkTransfers != 1 || len(got.NetworkTransfers) != 1 || len(got.Health.Transfers) != 1 {
		t.Fatalf("BuildReport(%q, started) summary=%+v transfers=%#v health=%+v, want one started transfer issue", target, got.Summary, got.NetworkTransfers, got.Health)
	}
	transfer := got.NetworkTransfers[0]
	if transfer.Status != string(control.NetworkTransferStarted) || transfer.Stage != "chunk" || transfer.Action != "retry_network_transfer" {
		t.Fatalf("BuildReport(%q, started).NetworkTransfers[0] = %+v, want started chunk retry evidence", target, transfer)
	}
	if transfer.Overhead == nil || transfer.Overhead.PaddingBytes != 128 || transfer.Overhead.PaddedChunks != 2 {
		t.Fatalf("BuildReport(%q, started).NetworkTransfers[0].Overhead = %+v, want persisted privacy overhead evidence", target, transfer.Overhead)
	}

	published := started
	published.Status = control.NetworkTransferPublished
	published.Stage = "commit"
	published.UpdatedAt = "2026-05-16T00:01:00Z"
	published.ErrorCode = ""
	published.Error = ""
	writeNetworkTransfer(t, target, published)

	got, err = BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q, published) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusVerified || got.Summary.NetworkTransfers != 0 || len(got.NetworkTransfers) != 0 || len(got.Health.Transfers) != 0 {
		t.Fatalf("BuildReport(%q, published) overall=%+v summary=%+v transfers=%#v health=%+v, want clean published transfer omitted from review issues", target, got.Overall, got.Summary, got.NetworkTransfers, got.Health)
	}
}

func TestBuildReportHealthIssuesAndDamagedArtifact(t *testing.T) {
	target := t.TempDir()
	writeSessionRecord(t, target, "session-recover", transaction.StateStaged)
	warningPath := filepath.Join(control.ControlDir(target), "warnings", "bad.json")
	if err := os.MkdirAll(filepath.Dir(warningPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(warningPath), err)
	}
	if err := os.WriteFile(warningPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", warningPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	if got.Health.Healthy || len(got.Health.RecoveryIssues) != 1 {
		t.Fatalf("BuildReport(%q).Health = %+v, want one recovery issue", target, got.Health)
	}
	if len(got.ArtifactProblems) != 1 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want damaged artifact problem", target, got.ArtifactProblems)
	}
	if got.ArtifactProblems[0].Path != filepath.ToSlash(warningPath) {
		t.Fatalf("BuildReport(%q).ArtifactProblems[0].Path = %q, want %q", target, got.ArtifactProblems[0].Path, filepath.ToSlash(warningPath))
	}
	if len(got.Health.ArtifactIssues) != 1 {
		t.Fatalf("BuildReport(%q).Health.ArtifactIssues = %#v, want damaged artifact in health", target, got.Health.ArtifactIssues)
	}
}

func TestBuildReportShowsReceiptSessionMismatch(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-crash", "docs/a.txt", []byte("hello"))
	writeSessionRecord(t, target, "session-crash", transaction.StateStaged)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	if got.Summary.RecoveryIssues != 1 || got.Summary.ArtifactProblems == 0 {
		t.Fatalf("BuildReport(%q).Summary = %+v, want recovery and artifact issue", target, got.Summary)
	}
	found := false
	for _, problem := range got.ArtifactProblems {
		if problem.Source == "health" && problem.SessionID == "session-crash" && strings.Contains(problem.Error, "receipt status") {
			found = true
		}
	}
	if !found {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want receipt/session mismatch", target, got.ArtifactProblems)
	}
}

func TestBuildReportShowsPartialControlArtifact(t *testing.T) {
	target := t.TempDir()
	writeSessionRecord(t, target, "session-partial", transaction.StateReceived)
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session-partial",
		SessionID: "session-partial",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
	})

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	found := false
	for _, problem := range got.ArtifactProblems {
		if problem.Source == "health" && problem.SessionID == "session-partial" && strings.Contains(problem.Error, "non-staged session") {
			found = true
		}
	}
	if !found {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want partial control artifact", target, got.ArtifactProblems)
	}
}

func TestBuildReportShowsNonPublishedReceiptArtifact(t *testing.T) {
	target := t.TempDir()
	writeSessionRecord(t, target, "session-receipt", transaction.StateStaged)
	writeReceiptForStatus(t, target, "session-receipt", "received")

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	found := false
	for _, problem := range got.ArtifactProblems {
		if problem.Source == "health" && problem.SessionID == "session-receipt" && strings.Contains(problem.Error, `receipt status "received"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want non-published receipt artifact", target, got.ArtifactProblems)
	}
}

func TestBuildReportDeduplicatesManifestArtifactProblems(t *testing.T) {
	target := t.TempDir()
	writePublishedReceipt(t, target, "session-missing-manifest")
	writeSessionRecord(t, target, "session-missing-manifest", transaction.StatePublished)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, want %q", target, got.Overall.Status, StatusUnhealthy)
	}
	if len(got.ArtifactProblems) != 1 {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want one deduplicated manifest problem", target, got.ArtifactProblems)
	}
}

func TestBuildReportSessionFilterIgnoresOtherSessionArtifactProblem(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-selected", "docs/a.txt", []byte("hello"))
	writePublishedReceipt(t, target, "session-other")
	writeSessionRecord(t, target, "session-other", transaction.StatePublished)
	otherManifestPath, err := control.Path(target, control.ArtifactManifest, "session-other")
	if err != nil {
		t.Fatalf("control.Path(other manifest) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Dir(otherManifestPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(otherManifestPath), err)
	}
	if err := os.WriteFile(otherManifestPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", otherManifestPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local", SessionID: "session-selected"})
	if err != nil {
		t.Fatalf("BuildReport(%q, selected) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusVerified {
		t.Fatalf("BuildReport(%q, selected).Overall.Status = %q, artifact_problems=%#v health=%+v, want %q", target, got.Overall.Status, got.ArtifactProblems, got.Health, StatusVerified)
	}
	if len(got.ArtifactProblems) != 0 || len(got.Health.ArtifactIssues) != 0 {
		t.Fatalf("BuildReport(%q, selected) artifact_problems=%#v health_artifacts=%#v, want unrelated artifact filtered", target, got.ArtifactProblems, got.Health.ArtifactIssues)
	}
}

func TestBuildReportSessionFilterKeepsUnscopedArtifactProblem(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-selected", "docs/a.txt", []byte("hello"))
	badPath := filepath.Join(control.ControlDir(target), "warnings", "bad-global.json")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(badPath), err)
	}
	if err := os.WriteFile(badPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", badPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local", SessionID: "session-selected"})
	if err != nil {
		t.Fatalf("BuildReport(%q, selected) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q, selected).Overall.Status = %q, artifact_problems=%#v, want %q", target, got.Overall.Status, got.ArtifactProblems, StatusUnhealthy)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].SessionID != "" || got.ArtifactProblems[0].Path != filepath.ToSlash(badPath) {
		t.Fatalf("BuildReport(%q, selected).ArtifactProblems = %#v, want unscoped bad-global problem", target, got.ArtifactProblems)
	}
	if len(got.Health.ArtifactIssues) != 1 || got.Health.ArtifactIssues[0].SessionID != "" {
		t.Fatalf("BuildReport(%q, selected).Health.ArtifactIssues = %#v, want unscoped health artifact retained", target, got.Health.ArtifactIssues)
	}
}

func TestBuildReportAggregateIncludesOlderSessionArtifactProblem(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-new", "docs/new.txt", []byte("new"))
	writePublishedReceipt(t, target, "session-old")
	writeSessionRecord(t, target, "session-old", transaction.StatePublished)
	oldManifestPath, err := control.Path(target, control.ArtifactManifest, "session-old")
	if err != nil {
		t.Fatalf("control.Path(old manifest) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Dir(oldManifestPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(oldManifestPath), err)
	}
	if err := os.WriteFile(oldManifestPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", oldManifestPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.SessionID != "session-new" {
		t.Fatalf("BuildReport(%q).SessionID = %q, want latest session-new", target, got.SessionID)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, artifact_problems=%#v health=%+v, want %q", target, got.Overall.Status, got.ArtifactProblems, got.Health, StatusUnhealthy)
	}
	if len(got.ArtifactProblems) == 0 || len(got.Health.ArtifactIssues) == 0 {
		t.Fatalf("BuildReport(%q) artifact_problems=%#v health_artifacts=%#v, want old session artifact retained in aggregate", target, got.ArtifactProblems, got.Health.ArtifactIssues)
	}
	if got.ArtifactProblems[0].SessionID != "session-old" {
		t.Fatalf("BuildReport(%q).ArtifactProblems[0].SessionID = %q, want session-old", target, got.ArtifactProblems[0].SessionID)
	}
}

func TestBuildReportAggregateIncludesOlderSessionProfileSnapshotProblem(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-old", "docs/old.txt", []byte("old"))
	writeCompleteSession(t, target, "session-new", "docs/new.txt", []byte("new"))
	rewriteManifestCreatedAt(t, target, "session-old", "2026-05-15T00:00:00Z")
	rewriteManifestCreatedAt(t, target, "session-new", "2026-05-16T00:00:00Z")
	path, err := control.Path(target, control.ArtifactProfileSnapshot, "profile-session-old")
	if err != nil {
		t.Fatalf("control.Path(old profile snapshot) error = %v, want nil", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("os.Remove(%q) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.SessionID != "session-new" {
		t.Fatalf("BuildReport(%q).SessionID = %q, want latest session-new", target, got.SessionID)
	}
	if got.Overall.Status != StatusUnhealthy {
		t.Fatalf("BuildReport(%q).Overall.Status = %q, artifact_problems=%#v, want %q", target, got.Overall.Status, got.ArtifactProblems, StatusUnhealthy)
	}
	if len(got.ArtifactProblems) != 1 || got.ArtifactProblems[0].Source != "profile_snapshot" || got.ArtifactProblems[0].SessionID != "session-old" {
		t.Fatalf("BuildReport(%q).ArtifactProblems = %#v, want old profile snapshot problem", target, got.ArtifactProblems)
	}
	if len(got.ProfileSnapshots) != 1 || got.ProfileSnapshots[0].SessionID != "session-new" {
		t.Fatalf("BuildReport(%q).ProfileSnapshots = %#v, want valid latest profile snapshot still visible", target, got.ProfileSnapshots)
	}
}

func TestBuildReportSessionFilterIgnoresOtherSessionRecoveryIssue(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "session-selected", "docs/a.txt", []byte("hello"))
	writeSessionRecord(t, target, "session-other", transaction.StateStaged)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local", SessionID: "session-selected"})
	if err != nil {
		t.Fatalf("BuildReport(%q, selected) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusVerified {
		t.Fatalf("BuildReport(%q, selected).Overall.Status = %q, want %q; health=%+v", target, got.Overall.Status, StatusVerified, got.Health)
	}
	if got.Summary.RecoveryIssues != 0 || len(got.Health.RecoveryIssues) != 0 {
		t.Fatalf("BuildReport(%q, selected).Health = %+v, want unrelated recovery issue filtered", target, got.Health)
	}
}

func TestBuildReportSessionFilterIgnoresOtherSessionInvalidRecord(t *testing.T) {
	target := t.TempDir()
	writeCompleteSession(t, target, "sessions", "docs/a.txt", []byte("hello"))
	badPath := filepath.Join(control.ControlDir(target), "sessions", "bad", "session.json")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(badPath), err)
	}
	if err := os.WriteFile(badPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", badPath, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local", SessionID: "sessions"})
	if err != nil {
		t.Fatalf("BuildReport(%q, sessions) error = %v, want nil", target, err)
	}
	if got.Overall.Status != StatusVerified {
		t.Fatalf("BuildReport(%q, sessions).Overall.Status = %q, health=%+v, want %q", target, got.Overall.Status, got.Health, StatusVerified)
	}
	if len(got.Health.InvalidRecords) != 0 || got.Summary.InvalidHealthRecords != 0 {
		t.Fatalf("BuildReport(%q, sessions).InvalidRecords = %#v, want unrelated invalid record filtered", target, got.Health.InvalidRecords)
	}
}

func TestBuildReportMissingTargetReturnsError(t *testing.T) {
	_, err := BuildReport(Options{TargetRoot: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatalf("BuildReport(missing target) error = nil, want error")
	}
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

func rewriteManifestCreatedAt(t *testing.T, target string, sessionID string, createdAt string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactManifest, sessionID)
	if err != nil {
		t.Fatalf("control.Path(%q, manifest, %q) error = %v, want nil", target, sessionID, err)
	}
	manifest, err := control.ReadFile[control.Manifest](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q, manifest) error = %v, want nil", path, err)
	}
	manifest.CreatedAt = createdAt
	if err := control.WriteFile(path, manifest); err != nil {
		t.Fatalf("control.WriteFile(%q, manifest) error = %v, want nil", path, err)
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
	writePublishedReceiptForScope(t, target, sessionID, "profile-local", "target-local")
}

func writePublishedReceiptForScope(t *testing.T, target string, sessionID string, profileID string, targetID string) {
	t.Helper()
	writeReceiptDocument(t, target, control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: profileID,
		TargetID:  targetID,
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    "published",
	})
}

func writeReceiptForStatus(t *testing.T, target string, sessionID string, status string) {
	t.Helper()
	writeReceiptDocument(t, target, control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: "profile-local",
		TargetID:  "target-local",
		StartedAt: "2026-05-16T00:00:00Z",
		EndedAt:   "2026-05-16T00:01:00Z",
		Status:    status,
	})
}

func writeReceiptDocument(t *testing.T, target string, receipt control.SessionReceipt) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSessionReceipt, receipt.ID)
	if err != nil {
		t.Fatalf("control.Path(%q, receipt, %q) error = %v, want nil", target, receipt.ID, err)
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
	payload := []byte(`{"profile_id":"profile-local","privacy_policy":{"mode":"plaintext","traffic_level":2,"allow_plaintext_restore":true,"allow_hidden_files":true,"allow_sensitive_filenames":true,"padding_bucket_bytes":65536,"batch_max_bytes":1048576,"batch_max_count":64,"jitter_budget_millis":250,"discovery_low_info":true},"roots":[{"id":"root"}],"target":{"target_id":"target-local"}}`)
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + sessionID,
		ProfileID:  "profile-local",
		SessionID:  sessionID,
		CapturedAt: "2026-05-16T00:00:00Z",
		Profile:    payload,
	}
	if err := control.WriteFile(path, snapshot); err != nil {
		t.Fatalf("control.WriteFile(%q, profile snapshot) error = %v, want nil", path, err)
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

func writeReportPruneCandidateFixture(t *testing.T, target string) control.SoftDelete {
	t.Helper()
	data := []byte("gone")
	writeTargetFile(t, target, "gone.txt", data)
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session-one",
		SessionID: "session-one",
		RootID:    "root",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []control.ManifestEntry{reportPruneFileEntry("gone.txt", data)},
	})
	writePublishedReceipt(t, target, "session-one")
	writeProfileSnapshot(t, target, "session-one")
	writeSessionRecord(t, target, "session-one", transaction.StatePublished)
	record := control.SoftDelete{
		Version:            control.CurrentVersion,
		ID:                 "session-two-del_001",
		SessionID:          "session-two",
		ProfileID:          "profile-local",
		TargetID:           "target-local",
		RootID:             "root",
		PreviousSessionID:  "session-one",
		PreviousManifestID: "manifest-session-one",
		SourcePath:         "gone.txt",
		TargetPath:         "gone.txt",
		Kind:               "file",
		Size:               int64(len(data)),
		Digest:             digest(data),
		DetectedAt:         "2026-05-16T00:03:00Z",
		Reason:             "missing_from_latest_source_scan",
	}
	writeManifest(t, target, control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-session-two",
		SessionID: "session-two",
		RootID:    "root",
		CreatedAt: "2026-05-17T00:00:00Z",
		Entries:   []control.ManifestEntry{reportPruneFileEntry("keep.txt", []byte("keep"))},
	})
	writeTargetFile(t, target, "keep.txt", []byte("keep"))
	writePublishedReceipt(t, target, "session-two")
	writeProfileSnapshot(t, target, "session-two")
	writeSessionRecord(t, target, "session-two", transaction.StatePublished)
	writeSoftDelete(t, target, record)
	return record
}

func reportPruneFileEntry(rel string, data []byte) control.ManifestEntry {
	entry := control.ManifestEntry{
		Path:       rel,
		Kind:       "file",
		ModTime:    "2026-05-16T00:00:00Z",
		Digest:     digest(data),
		TargetPath: rel,
	}
	entry.SetSizeEvidence(int64(len(data)))
	entry.SetModeEvidence(0o644)
	return entry
}

func writeReportPruneReceipt(t *testing.T, target string, status control.PruneReceiptStatus, items []control.PruneReceiptItem) {
	t.Helper()
	writeReportPruneReceiptWithID(t, target, "prune-apply", status, items)
}

func writeReportPruneReceiptWithID(t *testing.T, target string, id string, status control.PruneReceiptStatus, items []control.PruneReceiptItem) {
	t.Helper()
	writeReportPruneReceiptWithApproval(t, target, id, status, "approval-"+id, fmt.Sprintf("sha256:%064x", len(id)+len(items)), items)
}

func writeReportPruneReceiptWithApproval(t *testing.T, target string, id string, status control.PruneReceiptStatus, approvalID string, approvalScopeDigest string, items []control.PruneReceiptItem) {
	t.Helper()
	receipt := control.PruneReceipt{
		Version:             control.CurrentVersion,
		ID:                  id,
		PruneSessionID:      id,
		ApprovalID:          approvalID,
		ProfileID:           "profile-local",
		TargetID:            "target-local",
		StartedAt:           "2026-05-18T09:30:00Z",
		EndedAt:             "2026-05-18T09:31:00Z",
		Status:              status,
		DryRun:              false,
		ApprovalScopeDigest: approvalScopeDigest,
		Items:               items,
	}
	path, err := control.Path(target, control.ArtifactPruneReceipt, id)
	if err != nil {
		t.Fatalf("control.Path(%q, prune receipt, %q) error = %v, want nil", target, id, err)
	}
	if err := control.WriteFile(path, receipt); err != nil {
		t.Fatalf("control.WriteFile(%q, prune receipt) error = %v, want nil", path, err)
	}
}

func reportPruneApproval(t *testing.T, target string, approvalID string, p profile.Profile, record control.SoftDelete) control.PruneApproval {
	t.Helper()
	snapshotID := "profile-" + approvalID
	snapshot := writeReportPruneApprovalProfileSnapshot(t, target, snapshotID, p)
	snapshotDigest, err := prune.ProfileSnapshotDigest(snapshot.Profile)
	if err != nil {
		t.Fatalf("prune.ProfileSnapshotDigest(profile snapshot) error = %v, want nil", err)
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
		ID:                    approvalID,
		ProfileID:             p.ProfileID,
		TargetID:              p.Target.TargetID,
		RootID:                record.RootID,
		CreatedAt:             "2026-05-18T09:00:00Z",
		ApprovedBy:            "report-reviewer",
		ApprovedAt:            "2026-05-18T09:30:00Z",
		ReviewTool:            "unit-test",
		ProfileSnapshotID:     snapshotID,
		ProfileSnapshotDigest: snapshotDigest,
		ProfileDeletePolicy: control.PruneDeletePolicy{
			Mode:               string(p.DeletePolicy.Mode),
			RequireReview:      p.DeletePolicy.RequireReview,
			RetentionDays:      p.DeletePolicy.RetentionDays,
			AllowPhysicalPrune: p.DeletePolicy.AllowPhysicalPrune,
		},
		Items:          []control.PruneApprovalItem{item},
		ExpiresAt:      "2026-06-19T00:00:00Z",
		Status:         "approved",
		ApprovalReason: "reviewed stale target",
	}
	approval.ApprovalScopeDigest = prune.ApprovalScopeDigest(approval.ProfileID, approval.TargetID, approval.RootID, approval.ProfileSnapshotID, approval.ProfileSnapshotDigest, p.DeletePolicy, approval.Items)
	return approval
}

func writeReportPruneApprovalProfileSnapshot(t *testing.T, target string, snapshotID string, p profile.Profile) control.ProfileSnapshot {
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

func writeReportPruneApproval(t *testing.T, target string, approval control.PruneApproval) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactPruneApproval, approval.ID)
	if err != nil {
		t.Fatalf("control.Path(prune approval %q) error = %v, want nil", approval.ID, err)
	}
	if err := control.WriteNewFile(path, approval); err != nil {
		t.Fatalf("control.WriteNewFile(%q, prune approval) error = %v, want nil", path, err)
	}
}

func appliedReportPruneItem(record control.SoftDelete) control.PruneReceiptItem {
	present := true
	observed := control.PruneObservedTargetState{
		Present: &present,
		Kind:    "file",
		Path:    record.TargetPath,
		Digest:  record.Digest,
	}
	observed.SetSizeEvidence(record.Size)
	return control.PruneReceiptItem{
		SoftDeleteID:     record.ID,
		TargetPath:       record.TargetPath,
		IntendedAction:   "delete_file",
		PrePruneObserved: observed,
		Result:           "pruned",
		PrunedAt:         "2026-05-18T09:30:30Z",
	}
}

func failedReportPruneItem(record control.SoftDelete) control.PruneReceiptItem {
	item := appliedReportPruneItem(record)
	item.Result = "failed"
	item.ErrorCode = "remove_failed"
	item.Error = "injected remove failure"
	item.PrunedAt = ""
	return item
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

func writeRawNetworkTransfer(t *testing.T, target string, transfer control.NetworkTransfer) {
	t.Helper()
	transfer = normalizeNetworkTransferFixture(transfer)
	path, err := control.Path(target, control.ArtifactNetworkTransfer, transfer.SessionID)
	if err != nil {
		t.Fatalf("control.Path(%q, network transfer, %q) error = %v, want nil", target, transfer.SessionID, err)
	}
	data, err := json.MarshalIndent(transfer, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(network transfer) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q, network transfer) error = %v, want nil", path, err)
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

func validReportNetworkTransfer(sessionID string) control.NetworkTransfer {
	return control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       sessionID,
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
}

func validReportLevel2PrivacyOverhead() *control.NetworkTransferPrivacyOverhead {
	return &control.NetworkTransferPrivacyOverhead{
		FramePlainBytes:      512,
		FrameWireBytes:       640,
		PaddingBytes:         128,
		PaddedChunks:         2,
		PaddingBucketBytes:   64,
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

func validReportPairedProfile(source string, target string) profile.Profile {
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	return p
}

func validReportNetworkPairedProfile(t *testing.T, source string, target string) (profile.Profile, control.PairingReceipt) {
	t.Helper()
	now := time.Now().UTC()
	sourceCert := newReportTestCertificate(t, "source", now.Add(-time.Hour), now.Add(time.Hour))
	targetCert := newReportTestCertificate(t, "target", now.Add(-time.Hour), now.Add(time.Hour))
	sourceDeviceID := reportTestDeviceID(t, sourceCert)
	targetDeviceID := reportTestDeviceID(t, targetCert)
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.TargetID = "target-local"
	p.Target.DevicePublicKey = targetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = now.Format(time.RFC3339)
	certPath, keyPath := writeReportTLSIdentity(t, sourceCert)
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

func validReportPairingReceipt(p profile.Profile) control.PairingReceipt {
	return control.PairingReceipt{
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
		ProtocolVersion:  "supermover/1",
	}
}

func newReportTestCertificate(t *testing.T, commonName string, notBefore, notAfter time.Time) tls.Certificate {
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

func writeReportTLSIdentity(t *testing.T, cert tls.Certificate) (string, string) {
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

func reportTestDeviceID(t *testing.T, cert tls.Certificate) string {
	t.Helper()
	id, err := transport.LeafSPKIDeviceID(cert.Leaf)
	if err != nil {
		t.Fatalf("LeafSPKIDeviceID error = %v, want nil", err)
	}
	return id
}

func writeReportPairingReceipt(t *testing.T, target string, receipt control.PairingReceipt) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactPairingReceipt, receipt.ID)
	if err != nil {
		t.Fatalf("control.Path(%q, pairing receipt, %q) error = %v, want nil", target, receipt.ID, err)
	}
	if err := control.WriteFile(path, receipt); err != nil {
		t.Fatalf("control.WriteFile(%q, pairing receipt) error = %v, want nil", path, err)
	}
}

func containsIssue(issues []string, want string) bool {
	return containsString(issues, want)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func writeTargetFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
}

func writeSessionRecord(t *testing.T, target, sessionID string, state transaction.State) {
	t.Helper()
	now := time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC)
	record, err := transaction.NewSessionRecord(sessionID, now)
	if err != nil {
		t.Fatalf("transaction.NewSessionRecord(%q) error = %v, want nil", sessionID, err)
	}
	record, err = record.WithState(state, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("SessionRecord.WithState(%q) error = %v, want nil", state, err)
	}
	if err := transaction.NewLayout(control.ControlDir(target)).WriteSessionRecord(record); err != nil {
		t.Fatalf("Layout.WriteSessionRecord(%+v) error = %v, want nil", record, err)
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
