package health

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/transaction"
	"github.com/khicago/supermover/internal/transport"
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

func TestBuildReportRejectsUnsafeControlBoundaryBeforeRecoveryScan(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(control.ControlDir(target), "sessions")); err != nil {
		t.Skipf("Symlink(control sessions) unavailable: %v", err)
	}

	got, err := BuildReport(Options{TargetRoot: target})
	if err == nil {
		t.Fatalf("BuildReport(%q) = %+v, nil; want unsafe control boundary error", target, got)
	}
	if !strings.Contains(err.Error(), "control artifact path") || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("BuildReport(%q) error = %q, want control artifact symlink error", target, err.Error())
	}
	if _, err := os.Stat(filepath.Join(outside, "session.json")); !os.IsNotExist(err) {
		t.Fatalf("Stat(outside session.json) error = %v, want os.ErrNotExist", err)
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

func TestBuildReportMarksTargetDriftReviewUnhealthy(t *testing.T) {
	target := t.TempDir()
	writeTargetDrift(t, target, control.TargetDrift{
		Version:    control.CurrentVersion,
		ID:         "session-drift-file",
		SessionID:  "session-drift",
		ProfileID:  "profile-local",
		TargetID:   "target-local",
		RootID:     "root",
		Path:       "file.txt",
		DetectedAt: "2026-05-16T00:01:00Z",
		Change:     "content_mismatch",
		Evidence:   []string{"target content differs from staged manifest"},
	})

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy {
		t.Fatalf("BuildReport(%q).Healthy = true, want false for target drift review", target)
	}
	if got.Summary.TargetDrifts != 1 || len(got.TargetDrifts) != 1 || got.TargetDrifts[0].Path != "file.txt" {
		t.Fatalf("BuildReport(%q).TargetDrifts = %#v summary=%+v, want target drift review evidence", target, got.TargetDrifts, got.Summary)
	}
}

func TestBuildReportResolvedTargetDriftDoesNotRequireReview(t *testing.T) {
	target := t.TempDir()
	writeTargetDrift(t, target, control.TargetDrift{
		Version:      control.CurrentVersion,
		ID:           "session-drift-resolved",
		SessionID:    "session-drift",
		ProfileID:    "profile-local",
		TargetID:     "target-local",
		RootID:       "root",
		Path:         "file.txt",
		DetectedAt:   "2026-05-16T00:01:00Z",
		Change:       "content_mismatch",
		ReviewState:  "resolved",
		ReviewAction: "resolve",
		ReviewedAt:   "2026-05-20T00:00:00Z",
		ReviewReason: "target restored to expected manifest evidence",
		Evidence:     []string{"target content differs from staged manifest"},
	})

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !got.Healthy || got.Summary.TargetDrifts != 0 || len(got.TargetDrifts) != 0 {
		t.Fatalf("BuildReport(%q) healthy=%v target drifts=%#v summary=%+v, want resolved drift excluded from review counts", target, got.Healthy, got.TargetDrifts, got.Summary)
	}
}

func TestBuildReportMarksNetworkTransferIssueUnhealthy(t *testing.T) {
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
			FramePlainBytes:    512,
			FrameWireBytes:     640,
			PaddingBytes:       128,
			PaddedChunks:       2,
			PaddingBucketBytes: 64,
			JitteredRequests:   2,
			JitterBudgetMillis: 250,
		},
		Status:    control.NetworkTransferAuthRefused,
		Stage:     "begin",
		StartedAt: "2026-05-16T00:00:00Z",
		UpdatedAt: "2026-05-16T00:00:01Z",
		ErrorCode: "auth_refused",
		Error:     "receiver refused paired identity",
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

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy {
		t.Fatalf("BuildReport(%q).Healthy = true, want false for network transfer issue", target)
	}
	if got.Summary.NetworkTransfers != 1 || len(got.Transfers) != 1 {
		t.Fatalf("BuildReport(%q).Transfers = %#v summary=%+v, want one network transfer issue", target, got.Transfers, got.Summary)
	}
	transfer := got.Transfers[0]
	if transfer.SessionID != "session-network" || transfer.Status != "auth_refused" || transfer.Action != "review_pairing_and_profile_pins" {
		t.Fatalf("BuildReport(%q).Transfers[0] = %+v, want auth refusal operator action", target, transfer)
	}
	if transfer.Privacy != transport.DefaultPrivacyPolicy(transport.PrivacyLevel2) {
		t.Fatalf("BuildReport(%q).Transfers[0].Privacy = %+v, want level 2 transfer policy evidence", target, transfer.Privacy)
	}
	if transfer.Overhead == nil || transfer.Overhead.PaddingBytes != 128 || transfer.Overhead.PaddedChunks != 2 {
		t.Fatalf("BuildReport(%q).Transfers[0].Overhead = %+v, want persisted padding overhead evidence", target, transfer.Overhead)
	}
	if transfer.Overhead.JitteredRequests != 2 || transfer.Overhead.JitterDelayMillis != 0 || transfer.Overhead.MaxJitterDelayMillis != 0 || transfer.Overhead.JitterBudgetMillis != 250 {
		t.Fatalf("BuildReport(%q).Transfers[0].Overhead = %+v, want persisted zero-delay jitter overhead evidence", target, transfer.Overhead)
	}
}

func TestBuildReportKeepsNetworkTransferWithoutAuthoritativeReceipt(t *testing.T) {
	target := t.TempDir()
	foreign := validHealthNetworkTransfer("session-foreign")
	foreign.ProfileID = "profile-other"
	writeNetworkTransfer(t, target, foreign)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy || got.Summary.NetworkTransfers != 1 || len(got.Transfers) != 1 {
		t.Fatalf("BuildReport(%q) healthy=%v transfers=%#v summary=%+v, want transfer retained without authoritative foreign receipt", target, got.Healthy, got.Transfers, got.Summary)
	}
}

func TestBuildReportIgnoresForeignNetworkTransferWithReceipt(t *testing.T) {
	target := t.TempDir()
	writeReceiptForScope(t, target, "session-foreign", "received", "profile-other", "target-other")
	foreign := validHealthNetworkTransfer("session-foreign")
	foreign.ProfileID = "profile-other"
	foreign.TargetID = "target-other"
	writeNetworkTransfer(t, target, foreign)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !got.Healthy || got.Summary.NetworkTransfers != 0 || len(got.Transfers) != 0 {
		t.Fatalf("BuildReport(%q) healthy=%v transfers=%#v summary=%+v, want authoritative foreign receipt to filter transfer", target, got.Healthy, got.Transfers, got.Summary)
	}
}

func TestBuildReportKeepsRecoveryItemWhenNetworkTransferHasNoAuthoritativeReceipt(t *testing.T) {
	target := t.TempDir()
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecord(t, layout, "session-foreign", transaction.StateStaged)
	foreign := validHealthNetworkTransfer("session-foreign")
	foreign.ProfileID = "profile-other"
	foreign.TargetID = "target-other"
	writeNetworkTransfer(t, target, foreign)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy || got.Summary.IncompleteSessions != 1 || len(got.Items) != 1 {
		t.Fatalf("BuildReport(%q) healthy=%v summary=%+v items=%#v transfers=%#v, want recovery item retained without authoritative foreign receipt", target, got.Healthy, got.Summary, got.Items, got.Transfers)
	}
}

func TestBuildReportKeepsInvalidSessionRecordWhenNetworkTransferHasNoAuthoritativeReceipt(t *testing.T) {
	target := t.TempDir()
	foreign := validHealthNetworkTransfer("session-missing-record")
	foreign.ProfileID = "profile-other"
	foreign.TargetID = "target-other"
	writeNetworkTransfer(t, target, foreign)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy || got.Summary.InvalidRecords != 1 || len(got.Invalid) != 1 {
		t.Fatalf("BuildReport(%q) healthy=%v summary=%+v invalid=%#v transfers=%#v, want missing session record retained without authoritative foreign receipt", target, got.Healthy, got.Summary, got.Invalid, got.Transfers)
	}
}

func TestBuildReportMarksInScopeReceiptWithMismatchedNetworkTransferScopeUnhealthy(t *testing.T) {
	target := t.TempDir()
	writeReceiptForScope(t, target, "session-mismatch", "received", "profile-local", "target-local")
	transfer := validHealthNetworkTransfer("session-mismatch")
	transfer.ProfileID = "profile-other"
	transfer.TargetID = "target-other"
	writeNetworkTransfer(t, target, transfer)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	found := false
	for _, issue := range got.Artifacts {
		if issue.SessionID == "session-mismatch" && strings.Contains(issue.Error, "does not match session receipt") {
			found = true
		}
	}
	if got.Healthy || !found {
		t.Fatalf("BuildReport(%q) healthy=%v artifacts=%#v, want scope mismatch artifact problem", target, got.Healthy, got.Artifacts)
	}
}

func TestBuildReportTrustsForeignReceiptOverClaimedInScopeNetworkTransfer(t *testing.T) {
	target := t.TempDir()
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecord(t, layout, "session-foreign-claimed", transaction.StateStaged)
	writeReceiptForScope(t, target, "session-foreign-claimed", "received", "profile-other", "target-other")
	transfer := validHealthNetworkTransfer("session-foreign-claimed")
	transfer.ProfileID = "profile-local"
	transfer.TargetID = "target-local"
	writeNetworkTransfer(t, target, transfer)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !got.Healthy || got.Summary.IncompleteSessions != 0 || got.Summary.NetworkTransfers != 0 || got.Summary.ArtifactProblems != 0 {
		t.Fatalf("BuildReport(%q) healthy=%v summary=%+v items=%#v transfers=%#v artifacts=%#v, want foreign receipt to exclude claimed in-scope transfer", target, got.Healthy, got.Summary, got.Items, got.Transfers, got.Artifacts)
	}
}

func TestBuildReportIgnoresForeignNetworkTransferRecoveryItemWithReceipt(t *testing.T) {
	target := t.TempDir()
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecord(t, layout, "session-foreign", transaction.StateStaged)
	writeReceiptForScope(t, target, "session-foreign", "received", "profile-other", "target-other")
	foreign := validHealthNetworkTransfer("session-foreign")
	foreign.ProfileID = "profile-other"
	foreign.TargetID = "target-other"
	writeNetworkTransfer(t, target, foreign)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !got.Healthy || got.Summary.IncompleteSessions != 0 || got.Summary.NetworkTransfers != 0 {
		t.Fatalf("BuildReport(%q) healthy=%v summary=%+v items=%#v transfers=%#v, want authoritative foreign receipt to filter network session", target, got.Healthy, got.Summary, got.Items, got.Transfers)
	}
}

func TestBuildReportIgnoresForeignInvalidSessionRecordWithReceipt(t *testing.T) {
	target := t.TempDir()
	writeReceiptForScope(t, target, "session-missing-record", "received", "profile-other", "target-other")
	foreign := validHealthNetworkTransfer("session-missing-record")
	foreign.ProfileID = "profile-other"
	foreign.TargetID = "target-other"
	writeNetworkTransfer(t, target, foreign)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !got.Healthy || got.Summary.InvalidRecords != 0 || len(got.Invalid) != 0 || got.Summary.NetworkTransfers != 0 {
		t.Fatalf("BuildReport(%q) healthy=%v summary=%+v invalid=%#v transfers=%#v, want authoritative foreign receipt to filter invalid session record", target, got.Healthy, got.Summary, got.Invalid, got.Transfers)
	}
}

func TestBuildReportIgnoresCorruptForeignNetworkTransferByReceiptScope(t *testing.T) {
	target := t.TempDir()
	writeReceiptForScope(t, target, "session-foreign-corrupt", "received", "profile-other", "target-other")
	writeRawSessionArtifact(t, target, "session-foreign-corrupt", "network-transfer.json", `{"version":1,`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !got.Healthy || got.Summary.ArtifactProblems != 0 || len(got.Artifacts) != 0 || got.Summary.NetworkTransfers != 0 {
		t.Fatalf("BuildReport(%q) healthy=%v summary=%+v artifacts=%#v transfers=%#v, want corrupt foreign transfer ignored by receipt scope", target, got.Healthy, got.Summary, got.Artifacts, got.Transfers)
	}
}

func TestBuildReportKeepsCorruptInScopeNetworkTransfer(t *testing.T) {
	target := t.TempDir()
	writeReceiptForScope(t, target, "session-local-corrupt", "received", "profile-local", "target-local")
	writeRawSessionArtifact(t, target, "session-local-corrupt", "network-transfer.json", `{"version":1,`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy || got.Summary.ArtifactProblems != 1 || len(got.Artifacts) != 1 || got.Artifacts[0].Source != "network_transfer" {
		t.Fatalf("BuildReport(%q) healthy=%v summary=%+v artifacts=%#v, want corrupt in-scope transfer artifact problem", target, got.Healthy, got.Summary, got.Artifacts)
	}
}

func TestBuildReportKeepsCorruptNetworkTransferWhenReceiptIDMismatches(t *testing.T) {
	target := t.TempDir()
	writeReceiptWithIDForScope(t, target, "session-local-corrupt", "session-other", "received", "profile-other", "target-other")
	writeRawSessionArtifact(t, target, "session-local-corrupt", "network-transfer.json", `{"version":1,`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy || got.Summary.ArtifactProblems != 1 || len(got.Artifacts) != 1 || got.Artifacts[0].Source != "network_transfer" {
		t.Fatalf("BuildReport(%q) healthy=%v summary=%+v artifacts=%#v, want corrupt transfer retained when receipt id mismatches session", target, got.Healthy, got.Summary, got.Artifacts)
	}
}

func TestBuildReportIgnoresDecodedInvalidForeignNetworkTransferByReceiptScope(t *testing.T) {
	target := t.TempDir()
	writeReceiptForScope(t, target, "session-foreign-mismatch", "received", "profile-other", "target-other")
	transfer := validHealthNetworkTransfer("session-foreign-mismatch")
	transfer.SessionID = "session-other"
	transfer.ProfileID = "profile-other"
	transfer.TargetID = "target-other"
	writeNetworkTransferAtSession(t, target, "session-foreign-mismatch", transfer)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if !got.Healthy || got.Summary.ArtifactProblems != 0 || got.Summary.NetworkTransfers != 0 || len(got.Artifacts) != 0 {
		t.Fatalf("BuildReport(%q) healthy=%v summary=%+v artifacts=%#v transfers=%#v, want decoded invalid foreign transfer filtered by receipt scope", target, got.Healthy, got.Summary, got.Artifacts, got.Transfers)
	}
}

func TestBuildReportPublishedNetworkTransferStillRequiresSessionRecordButIsNotTransferIssue(t *testing.T) {
	target := t.TempDir()
	published := validHealthNetworkTransfer("session-published")
	published.Status = control.NetworkTransferPublished
	published.Stage = "commit"
	published.ErrorCode = ""
	published.Error = ""
	published.PrivacyOverhead = validLevel2PrivacyOverhead()
	writeNetworkTransfer(t, target, published)

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy {
		t.Fatalf("BuildReport(%q).Healthy = true, want false for published transfer without session record", target)
	}
	if got.Summary.NetworkTransfers != 0 || len(got.Transfers) != 0 {
		t.Fatalf("BuildReport(%q).Transfers = %#v summary=%+v, want published transfer excluded from network transfer issues", target, got.Transfers, got.Summary)
	}
	if got.Summary.InvalidRecords != 1 || len(got.Invalid) != 1 {
		t.Fatalf("BuildReport(%q).Invalid = %#v summary=%+v, want missing published session record", target, got.Invalid, got.Summary)
	}
}

func TestBuildReportMarksPublishedLevel2TransferWithoutOverheadUnhealthy(t *testing.T) {
	target := t.TempDir()
	published := normalizeNetworkTransferFixture(validHealthNetworkTransfer("session-published-no-overhead"))
	published.Status = control.NetworkTransferPublished
	published.Stage = "commit"
	published.ErrorCode = ""
	published.Error = ""
	published.PrivacyOverhead = nil
	published.Attempts[0].Status = control.NetworkTransferPublished
	published.Attempts[0].Stage = "commit"
	published.Attempts[0].ErrorCode = ""
	published.Attempts[0].Error = ""
	path, err := control.Path(target, control.ArtifactNetworkTransfer, published.SessionID)
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	payload, err := json.Marshal(published)
	if err != nil {
		t.Fatalf("json.Marshal(published transfer) error = %v, want nil", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy || got.Summary.ArtifactProblems != 1 || len(got.Artifacts) != 1 {
		t.Fatalf("BuildReport(%q) healthy=%v summary=%+v artifacts=%#v, want one invalid network transfer artifact", target, got.Healthy, got.Summary, got.Artifacts)
	}
	if !strings.Contains(got.Artifacts[0].Error, "privacy_overhead is required for published level 2 network transfer") {
		t.Fatalf("BuildReport(%q).Artifacts[0] = %+v, want missing overhead validation error", target, got.Artifacts[0])
	}
}

func TestBuildReportMarksPublishedNetworkTransferStateMismatchUnhealthy(t *testing.T) {
	target := t.TempDir()
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecord(t, layout, "session-rolled-back", transaction.StateRolledBack)
	published := validHealthNetworkTransfer("session-rolled-back")
	published.Status = control.NetworkTransferPublished
	published.Stage = "commit"
	published.ErrorCode = ""
	published.Error = ""
	published.PrivacyOverhead = validLevel2PrivacyOverhead()
	writeNetworkTransfer(t, target, published)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	found := false
	for _, issue := range got.Artifacts {
		if issue.SessionID == "session-rolled-back" && strings.Contains(issue.Error, "network transfer status") && strings.Contains(issue.Error, "session state") {
			found = true
		}
	}
	if got.Healthy || !found {
		t.Fatalf("BuildReport(%q) healthy=%v artifacts=%#v, want published network transfer/session state mismatch", target, got.Healthy, got.Artifacts)
	}
}

func TestBuildReportMarksDamagedNetworkTransferArtifactUnhealthy(t *testing.T) {
	target := t.TempDir()
	path := filepath.Join(control.ControlDir(target), "sessions", "session-network", "network-transfer.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy {
		t.Fatalf("BuildReport(%q).Healthy = true, want false for damaged network transfer artifact", target)
	}
	if got.Summary.ArtifactProblems != 1 || len(got.Artifacts) != 1 || got.Artifacts[0].Path != path {
		t.Fatalf("BuildReport(%q).Artifacts = %#v summary=%+v, want one damaged network transfer artifact", target, got.Artifacts, got.Summary)
	}
}

func TestBuildReportIgnoresTruncatedForeignTargetDriftArtifactByReceipt(t *testing.T) {
	target := t.TempDir()
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecord(t, layout, "foreign-session", transaction.StateReceived)
	writeReceiptForScope(t, target, "foreign-session", "received", "foreign-profile", "foreign-target")
	writeRawArtifact(t, target, "drift", "foreign-drift.json", `{"version":1,"id":"foreign-drift","session_id":"foreign-session","root_id":"root","path":"file.txt",`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	foundDriftProblem := false
	for _, artifact := range got.Artifacts {
		if strings.Contains(artifact.Path, filepath.Join(control.DirName, "drift")) {
			foundDriftProblem = true
		}
	}
	if foundDriftProblem {
		t.Fatalf("BuildReport(%q).Artifacts = %#v, want foreign truncated drift ignored", target, got.Artifacts)
	}
}

func TestBuildReportIgnoresMisleadingTruncatedForeignTargetDriftArtifactByReceipt(t *testing.T) {
	target := t.TempDir()
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecord(t, layout, "foreign-session", transaction.StateReceived)
	writeReceiptForScope(t, target, "foreign-session", "received", "foreign-profile", "foreign-target")
	writeRawArtifact(t, target, "drift", "foreign-drift.json", `{"version":1,"id":"foreign-drift","session_id":"foreign-session","profile_id":"profile-local","target_id":"target-local","root_id":"root","path":"file.txt",`)

	got, err := BuildReport(Options{TargetRoot: target, ProfileID: "profile-local", TargetID: "target-local"})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	foundDriftProblem := false
	for _, artifact := range got.Artifacts {
		if strings.Contains(artifact.Path, filepath.Join(control.DirName, "drift")) {
			foundDriftProblem = true
		}
	}
	if foundDriftProblem {
		t.Fatalf("BuildReport(%q).Artifacts = %#v, want misleading foreign truncated drift ignored", target, got.Artifacts)
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

func TestBuildReportMarksNonPublishedReceiptWithStagedSessionUnhealthy(t *testing.T) {
	target := t.TempDir()
	layout := transaction.NewLayout(control.ControlDir(target))
	writeRecord(t, layout, "session-receipt", transaction.StateStaged)
	writeReceipt(t, target, "session-receipt", "received")

	got, err := BuildReport(Options{TargetRoot: target})
	if err != nil {
		t.Fatalf("BuildReport(%q) error = %v, want nil", target, err)
	}
	if got.Healthy {
		t.Fatalf("BuildReport(%q).Healthy = true, want false for non-published receipt on staged session", target)
	}
	found := false
	for _, issue := range got.Artifacts {
		if issue.SessionID == "session-receipt" && strings.Contains(issue.Error, `receipt status "received"`) && strings.Contains(issue.Error, "non-published session state") {
			found = true
		}
	}
	if !found {
		t.Fatalf("BuildReport(%q).Artifacts = %#v, want non-published receipt issue", target, got.Artifacts)
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
	writeReceiptForScope(t, target, sessionID, status, "profile-local", "target-local")
}

func writeReceiptForScope(t *testing.T, target string, sessionID string, status string, profileID string, targetID string) {
	t.Helper()
	writeReceiptWithIDForScope(t, target, sessionID, sessionID, status, profileID, targetID)
}

func writeReceiptWithIDForScope(t *testing.T, target string, sessionID string, receiptID string, status string, profileID string, targetID string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactSessionReceipt, sessionID)
	if err != nil {
		t.Fatalf("control.Path(receipt) error = %v, want nil", err)
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

func writeRawArtifact(t *testing.T, target, dir, name string, data string) {
	t.Helper()
	path := filepath.Join(target, control.DirName, dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
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

func writeTargetDrift(t *testing.T, target string, drift control.TargetDrift) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactTargetDrift, drift.ID)
	if err != nil {
		t.Fatalf("control.Path(target drift) error = %v, want nil", err)
	}
	if err := control.WriteFile(path, drift); err != nil {
		t.Fatalf("control.WriteFile(%q, target drift) error = %v, want nil", path, err)
	}
}

func writeNetworkTransfer(t *testing.T, target string, transfer control.NetworkTransfer) {
	t.Helper()
	transfer = normalizeNetworkTransferFixture(transfer)
	writeNetworkTransferAtSession(t, target, transfer.SessionID, transfer)
}

func writeNetworkTransferAtSession(t *testing.T, target string, sessionID string, transfer control.NetworkTransfer) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactNetworkTransfer, sessionID)
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	if err := control.WriteFile(path, transfer); err != nil {
		t.Fatalf("control.WriteFile(%q, network transfer) error = %v, want nil", path, err)
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

func validHealthNetworkTransfer(sessionID string) control.NetworkTransfer {
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

func validLevel2PrivacyOverhead() *control.NetworkTransferPrivacyOverhead {
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
