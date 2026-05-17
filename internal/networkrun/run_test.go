package networkrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/protocolclient"
	"github.com/khicago/supermover/internal/receiver"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/transport"
)

func TestRunRecordsAuthRefusedFromBegin(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.txt", []byte("payload"))
	target := t.TempDir()
	sessionID := "session-auth-refused"
	client := fastJitterClient("http://receiver.invalid", 0, doerFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/sessions" {
			return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
		}
		return jsonHTTPResponse(http.StatusForbidden, protocol.ErrorResponse{
			Code:    protocol.ErrorCodeForbidden,
			Message: "authenticated transport identity is required",
		}), nil
	}))

	_, err := Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: validProfilePrivacyPolicy(),
		Request:              validTransferRequest(source, scanResult, sessionID),
		Client:               client,
		Now:                  fixedNow,
	})
	if err == nil {
		t.Fatalf("Run(auth refused) error = nil, want receiver error")
	}
	var remote *protocolclient.RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("Run(auth refused) error = %T %v, want RemoteError", err, err)
	}
	if remote.Path != "/v1/sessions" || remote.StatusCode != http.StatusForbidden {
		t.Fatalf("RemoteError = %+v, want /v1/sessions 403", remote)
	}

	transfer := readNetworkTransfer(t, target, sessionID)
	assertNetworkTransferState(t, transfer, control.NetworkTransferAuthRefused, "begin", "auth_refused")
	wantPolicy := transport.DefaultPrivacyPolicy(transport.PrivacyLevel2)
	wantPolicy.JitterBudget = 1
	if transfer.PrivacyPolicy != wantPolicy {
		t.Fatalf("network transfer privacy policy = %+v, want request level 2 policy", transfer.PrivacyPolicy)
	}
	if transfer.Error == "" {
		t.Fatalf("network transfer error is empty, want receiver error detail")
	}
}

func TestRunRejectsMissingPrivacyPolicyBeforeWritingTransfer(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.txt", []byte("payload"))
	target := t.TempDir()
	sessionID := "session-missing-privacy"
	req := validTransferRequest(source, scanResult, sessionID)
	req.PrivacyPolicy = transport.PrivacyPolicy{}

	_, err := Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: validProfilePrivacyPolicy(),
		Request:              req,
		Client:               protocolclient.Client{},
		Now:                  fixedNow,
	})

	if err == nil || !strings.Contains(err.Error(), "privacy policy is required") {
		t.Fatalf("Run(missing privacy) error = %v, want privacy policy required", err)
	}
	path, pathErr := control.Path(target, control.ArtifactNetworkTransfer, sessionID)
	if pathErr != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", pathErr)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("network transfer artifact stat error = %v, want absent artifact", statErr)
	}
}

func TestRunRejectsPrivacyPolicyMismatchBeforeWritingTransfer(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.txt", []byte("payload"))
	target := t.TempDir()
	sessionID := "session-privacy-mismatch"
	req := validTransferRequest(source, scanResult, sessionID)
	profilePolicy := validProfilePrivacyPolicy()
	profilePolicy.JitterBudgetMillis = req.PrivacyPolicy.JitterBudget + 1

	_, err := Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: profilePolicy,
		Request:              req,
		Client:               protocolclient.Client{},
		Now:                  fixedNow,
	})

	if err == nil || !strings.Contains(err.Error(), "privacy policy does not match profile privacy policy") {
		t.Fatalf("Run(privacy mismatch) error = %v, want privacy mismatch", err)
	}
	path, pathErr := control.Path(target, control.ArtifactNetworkTransfer, sessionID)
	if pathErr != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", pathErr)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("network transfer artifact stat error = %v, want absent artifact", statErr)
	}
}

func TestRunRecordsInterruptedAndReturnsContextError(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.txt", []byte("payload"))
	target := t.TempDir()
	sessionID := "session-canceled"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := Run(ctx, Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: validProfilePrivacyPolicy(),
		Request:              validTransferRequest(source, scanResult, sessionID),
		Client:               protocolclient.Client{},
		Now:                  fixedNow,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run(canceled) error = %v, want context.Canceled", err)
	}
	if got.Transfer.Status != control.NetworkTransferInterrupted {
		t.Fatalf("Run(canceled).Transfer.Status = %q, want %q", got.Transfer.Status, control.NetworkTransferInterrupted)
	}

	transfer := readNetworkTransfer(t, target, sessionID)
	assertNetworkTransferState(t, transfer, control.NetworkTransferInterrupted, "transport", "interrupted")
	if transfer.Error != context.Canceled.Error() {
		t.Fatalf("network transfer error = %q, want %q", transfer.Error, context.Canceled.Error())
	}
}

func TestRunRecordsReceiverNeedsRepairFromStatus(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.txt", []byte("payload"))
	target := t.TempDir()
	sessionID := "session-needs-repair"
	client := fastJitterClient("http://receiver.invalid", 0, doerFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/sessions":
			return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
				SessionID: sessionID,
				State:     protocol.SessionStateValidated,
			}), nil
		case "/v1/sessions/" + sessionID + "/status":
			return jsonHTTPResponse(http.StatusOK, statusFromScan(scanResult, sessionID, protocol.SessionStateNeedsRepair)), nil
		default:
			return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
		}
	}))

	_, err := Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: validProfilePrivacyPolicy(),
		Request:              validTransferRequest(source, scanResult, sessionID),
		Client:               client,
		Now:                  fixedNow,
	})
	if !errors.Is(err, protocolclient.ErrReceiverNeedsRepair) {
		t.Fatalf("Run(receiver needs repair) error = %v, want ErrReceiverNeedsRepair", err)
	}

	transfer := readNetworkTransfer(t, target, sessionID)
	assertNetworkTransferState(t, transfer, control.NetworkTransferNeedsRepair, "status", "receiver_needs_repair")
}

func TestRunPersistsWarningsBeforePublishedTransfer(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "data.txt"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(data.txt) error = %v, want nil", err)
	}
	reservedDir := filepath.Join(source, control.DirName)
	if err := os.Mkdir(reservedDir, 0o755); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", reservedDir, err)
	}
	if err := os.WriteFile(filepath.Join(reservedDir, "source-control.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source control) error = %v, want nil", err)
	}
	scanResult, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}

	server := httptest.NewServer(receiver.NewHandler(receiver.FileStore{TargetRoot: target}))
	defer server.Close()
	sessionID := "session-success-warnings"

	got, err := Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: validProfilePrivacyPolicy(),
		Request:              validTransferRequest(source, scanResult, sessionID),
		Client:               fastJitterClient(server.URL, 4, server.Client()),
		Now:                  fixedNow,
	})
	if err != nil {
		t.Fatalf("Run(loopback warning success) error = %v, want nil", err)
	}
	if len(got.ClientResult.Warnings) != 1 {
		t.Fatalf("Run(loopback warning success).Warnings = %+v, want one reserved control warning", got.ClientResult.Warnings)
	}

	warningsDir := filepath.Join(control.ControlDir(target), "warnings")
	warningEntries, err := os.ReadDir(warningsDir)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil", warningsDir, err)
	}
	if len(warningEntries) != 1 {
		t.Fatalf("warning artifacts = %d, want 1", len(warningEntries))
	}
	warning, err := control.ReadFile[control.Warning](filepath.Join(warningsDir, warningEntries[0].Name()))
	if err != nil {
		t.Fatalf("control.ReadFile(warning) error = %v, want nil", err)
	}
	if warning.SessionID != sessionID || warning.Code != "reserved_control_plane_skipped" || warning.CreatedAt != fixedNow().UTC().Format(time.RFC3339Nano) {
		t.Fatalf("warning = %+v, want persisted reserved control warning for %s", warning, sessionID)
	}
	if warning.SuggestedConfig["append_migration_path"] != control.DirName {
		t.Fatalf("warning suggested config = %#v, want append_migration_path=%q", warning.SuggestedConfig, control.DirName)
	}

	transfer := readNetworkTransfer(t, target, sessionID)
	assertNetworkTransferState(t, transfer, control.NetworkTransferPublished, "commit", "")
	if transfer.Error != "" {
		t.Fatalf("published network transfer error = %q, want empty", transfer.Error)
	}
	if transfer.PrivacyOverhead == nil || transfer.PrivacyOverhead.PaddedChunks == 0 || transfer.PrivacyOverhead.PaddingBytes == 0 {
		t.Fatalf("published network transfer privacy overhead = %+v, want persisted padding overhead", transfer.PrivacyOverhead)
	}
	if transfer.PrivacyOverhead.PaddingBucketBytes != transport.DefaultPrivacyPolicy(transport.PrivacyLevel2).PaddingBucket {
		t.Fatalf("published network transfer padding bucket = %d, want default level 2 bucket", transfer.PrivacyOverhead.PaddingBucketBytes)
	}
	if transfer.PrivacyOverhead.BatchFrames == 0 || transfer.PrivacyOverhead.BatchedChunks == 0 || transfer.PrivacyOverhead.MaxBatchCount == 0 || transfer.PrivacyOverhead.MaxBatchPlainBytes == 0 {
		t.Fatalf("published network transfer privacy overhead = %+v, want persisted batch evidence", transfer.PrivacyOverhead)
	}
	if transfer.PrivacyOverhead.JitteredRequests == 0 || transfer.PrivacyOverhead.JitterBudgetMillis != transfer.PrivacyPolicy.JitterBudget {
		t.Fatalf("published network transfer privacy overhead = %+v, want persisted jitter evidence matching policy", transfer.PrivacyOverhead)
	}
	if transfer.PrivacyOverhead.MaxJitterDelayMillis > transfer.PrivacyPolicy.JitterBudget {
		t.Fatalf("published network transfer max jitter delay = %d, want <= policy budget %d", transfer.PrivacyOverhead.MaxJitterDelayMillis, transfer.PrivacyPolicy.JitterBudget)
	}
}

func TestRunPersistsArtifactsThroughRemoteWriterAfterBegin(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.txt", []byte("payload"))
	target := t.TempDir()
	server := httptest.NewServer(receiver.NewHandler(receiver.FileStore{TargetRoot: target}))
	defer server.Close()
	sessionID := "session-remote-artifacts"
	writer := HTTPArtifactWriter{BaseURL: server.URL, Doer: server.Client()}
	snapshot := profileSnapshotFixture(t, sessionID)

	got, err := Run(context.Background(), Options{
		ArtifactWriter:       writer,
		ProfileSnapshot:      &snapshot,
		ProfilePrivacyPolicy: validProfilePrivacyPolicy(),
		Request:              validTransferRequest(source, scanResult, sessionID),
		Client:               fastJitterClient(server.URL, 4, server.Client()),
		Now:                  fixedNow,
	})
	if err != nil {
		t.Fatalf("Run(remote artifact writer) error = %v, want nil", err)
	}
	if got.Transfer.Status != control.NetworkTransferPublished {
		t.Fatalf("Run(remote artifact writer).Transfer = %+v, want published", got.Transfer)
	}
	if readControlDoc[control.ProfileSnapshot](t, target, control.ArtifactProfileSnapshot, snapshot.ID).ID != snapshot.ID {
		t.Fatalf("remote profile snapshot was not persisted")
	}
	transfer := readNetworkTransfer(t, target, sessionID)
	assertNetworkTransferState(t, transfer, control.NetworkTransferPublished, "commit", "")
}

func TestLocalArtifactWriterRejectsCrossSessionNetworkTransfer(t *testing.T) {
	target := t.TempDir()
	writer := LocalArtifactWriter{TargetRoot: target}
	doc := validPublishedNetworkTransfer("session-other")
	path, err := control.Path(target, control.ArtifactNetworkTransfer, "session-local")
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	if err := control.WriteFile(path, doc); err != nil {
		t.Fatalf("control.WriteFile(cross-session transfer) error = %v, want nil", err)
	}

	if _, err := writer.ReadNetworkTransfer(context.Background(), "session-local"); err == nil || !strings.Contains(err.Error(), "session_id") {
		t.Fatalf("ReadNetworkTransfer(cross-session) error = %v, want session_id mismatch", err)
	}
}

func TestLocalArtifactWriterRejectsSymlinkNetworkTransfer(t *testing.T) {
	target := t.TempDir()
	writer := LocalArtifactWriter{TargetRoot: target}
	foreignPath := filepath.Join(t.TempDir(), "network-transfer.json")
	if err := control.WriteFile(foreignPath, validPublishedNetworkTransfer("session-local")); err != nil {
		t.Fatalf("control.WriteFile(foreign transfer) error = %v, want nil", err)
	}
	path, err := control.Path(target, control.ArtifactNetworkTransfer, "session-local")
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.Symlink(foreignPath, path); err != nil {
		t.Fatalf("os.Symlink(%q, %q) error = %v, want nil", foreignPath, path, err)
	}

	if _, err := writer.ReadNetworkTransfer(context.Background(), "session-local"); err == nil {
		t.Fatalf("ReadNetworkTransfer(symlink) error = nil, want failure")
	}
}

func TestRunRecordsInterruptedPrivacyOverheadAfterPaddedChunk(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "large.bin", []byte("abcdefghijklmnop"))
	target := t.TempDir()
	server := httptest.NewServer(receiver.NewHandler(receiver.FileStore{TargetRoot: target}))
	defer server.Close()
	sessionID := "session-interrupted-overhead"
	req := validTransferRequest(source, scanResult, sessionID)
	req.PrivacyPolicy.BatchMaxCount = 1

	_, err := Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: profilePrivacyPolicyForRequest(req),
		Request:              req,
		Client:               fastJitterClient(server.URL, 4, &failAfterAcceptedChunkDoer{base: server.Client(), failAfter: 1}),
		Now:                  fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "simulated interruption after accepted chunk") {
		t.Fatalf("Run(interrupted padded chunk) error = %v, want simulated interruption", err)
	}

	transfer := readNetworkTransfer(t, target, sessionID)
	assertNetworkTransferState(t, transfer, control.NetworkTransferFailed, "transport", "transfer_failed")
	if transfer.PrivacyOverhead == nil || transfer.PrivacyOverhead.PaddedChunks != 1 || transfer.PrivacyOverhead.PaddingBytes == 0 {
		t.Fatalf("failed network transfer privacy overhead = %+v, want one padded chunk overhead", transfer.PrivacyOverhead)
	}
	if transfer.PrivacyOverhead.JitteredRequests == 0 || transfer.PrivacyOverhead.JitterBudgetMillis != transfer.PrivacyPolicy.JitterBudget {
		t.Fatalf("failed network transfer privacy overhead = %+v, want retained jitter overhead", transfer.PrivacyOverhead)
	}
}

func TestRunWritesInFlightProgressEvidenceBeforeClientCompletion(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "large.bin", []byte("abcdefghijklmnop"))
	target := t.TempDir()
	server := httptest.NewServer(receiver.NewHandler(receiver.FileStore{TargetRoot: target}))
	defer server.Close()
	sessionID := "session-inflight-progress"
	req := validTransferRequest(source, scanResult, sessionID)
	req.PrivacyPolicy.BatchMaxCount = 1
	progressErr := errors.New("simulated source process stop after progress evidence")
	req.Progress = func(_ context.Context, event protocolclient.ProgressEvent) error {
		if event.Stage == protocolclient.ProgressStageChunk {
			inFlight := readNetworkTransfer(t, target, sessionID)
			if inFlight.Status != control.NetworkTransferStarted || inFlight.Stage != "chunk" ||
				inFlight.PrivacyOverhead == nil || inFlight.PrivacyOverhead.PaddedChunks != 1 || inFlight.PrivacyOverhead.BatchFrames != 1 {
				t.Fatalf("network transfer before caller progress callback = %+v, want in-flight chunk evidence", inFlight)
			}
			return progressErr
		}
		return nil
	}

	_, err := Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: profilePrivacyPolicyForRequest(req),
		Request:              req,
		Client:               fastJitterClient(server.URL, 4, server.Client()),
		Now:                  fixedNow,
	})
	if !errors.Is(err, progressErr) {
		t.Fatalf("Run(progress stop) error = %v, want progressErr", err)
	}

	transfer := readNetworkTransfer(t, target, sessionID)
	assertNetworkTransferState(t, transfer, control.NetworkTransferFailed, "transport", "transfer_failed")
	if transfer.PrivacyOverhead == nil || transfer.PrivacyOverhead.PaddedChunks != 1 || transfer.PrivacyOverhead.BatchFrames != 1 {
		t.Fatalf("final failed transfer privacy overhead = %+v, want retained one accepted chunk", transfer.PrivacyOverhead)
	}

}

func TestRunResumesAfterSourceStopWithInFlightProgressEvidence(t *testing.T) {
	sourcePayload := bytes.Repeat([]byte("source-stop-resume-"), 4)
	source, scanResult := scanSingleFileSource(t, "large.bin", sourcePayload)
	target := t.TempDir()
	server := httptest.NewServer(receiver.NewHandler(receiver.FileStore{TargetRoot: target}))
	defer server.Close()
	sessionID := "session-source-stop-resume"
	req := validTransferRequest(source, scanResult, sessionID)
	req.PrivacyPolicy.BatchMaxCount = 1
	chunkSize := 16
	progressErr := errors.New("simulated source stop after durable progress evidence")
	var stoppedAt int64
	req.Progress = func(_ context.Context, event protocolclient.ProgressEvent) error {
		if event.Stage != protocolclient.ProgressStageChunk {
			return nil
		}
		if len(event.Chunks) != 1 {
			t.Fatalf("chunk progress entries = %d, want one", len(event.Chunks))
		}
		stoppedAt = event.Chunks[0].ReceiverCommittedSize
		inFlight := readNetworkTransfer(t, target, sessionID)
		if inFlight.Status != control.NetworkTransferStarted || inFlight.Stage != "chunk" ||
			inFlight.PrivacyOverhead == nil || inFlight.PrivacyOverhead.PaddedChunks != 1 || inFlight.PrivacyOverhead.BatchFrames != 1 {
			t.Fatalf("network transfer before simulated source stop = %+v, want durable in-flight chunk evidence", inFlight)
		}
		return progressErr
	}

	first, err := Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: profilePrivacyPolicyForRequest(req),
		Request:              req,
		Client:               fastJitterClient(server.URL, chunkSize, server.Client()),
		Now:                  fixedNow,
	})
	if !errors.Is(err, progressErr) {
		t.Fatalf("Run(source stop) error = %v, want progressErr", err)
	}
	if stoppedAt <= 0 || stoppedAt >= int64(len(sourcePayload)) {
		t.Fatalf("stopped receiver offset = %d, want partial progress", stoppedAt)
	}
	assertNetworkTransferState(t, first.Transfer, control.NetworkTransferFailed, "transport", "transfer_failed")
	prior := readNetworkTransfer(t, target, sessionID)
	if prior.PrivacyOverhead == nil || prior.PrivacyOverhead.PaddedChunks != 1 || prior.PrivacyOverhead.BatchFrames != 1 {
		t.Fatalf("failed source-stop transfer overhead = %+v, want retained first chunk overhead", prior.PrivacyOverhead)
	}
	if _, statErr := os.Lstat(filepath.Join(target, "large.bin")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("os.Lstat(target before retry) error = %v, want no published file", statErr)
	}

	req.Progress = nil
	second, err := Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: profilePrivacyPolicyForRequest(req),
		Request:              req,
		Client:               fastJitterClient(server.URL, chunkSize, server.Client()),
		Now:                  func() time.Time { return fixedNow().Add(5 * time.Minute) },
	})
	if err != nil {
		t.Fatalf("Run(resume after source stop) error = %v, want nil", err)
	}
	wantRemaining := int64(len(sourcePayload)) - stoppedAt
	if second.ClientResult.Bytes != wantRemaining {
		t.Fatalf("Run(resume after source stop).Bytes = %d, want remaining bytes %d", second.ClientResult.Bytes, wantRemaining)
	}
	if len(second.ClientResult.ResumeFrom) != 1 || second.ClientResult.ResumeFrom[0].CommittedSize != stoppedAt {
		t.Fatalf("Run(resume after source stop).ResumeFrom = %+v, want receiver status offset %d", second.ClientResult.ResumeFrom, stoppedAt)
	}
	published := readNetworkTransfer(t, target, sessionID)
	if published.Status != control.NetworkTransferPublished || published.Stage != "commit" || published.PrivacyOverhead == nil {
		t.Fatalf("published source-stop retry transfer = %+v, want published commit evidence", published)
	}
	if published.PrivacyOverhead.PaddedChunks <= prior.PrivacyOverhead.PaddedChunks ||
		published.PrivacyOverhead.BatchFrames <= prior.PrivacyOverhead.BatchFrames ||
		len(published.Attempts) != len(prior.Attempts)+1 {
		t.Fatalf("published source-stop retry transfer = overhead %+v attempts %d, want merged prior %+v attempts %d", published.PrivacyOverhead, len(published.Attempts), prior.PrivacyOverhead, len(prior.Attempts))
	}
	sourceData, err := os.ReadFile(filepath.Join(source, "large.bin"))
	if err != nil {
		t.Fatalf("os.ReadFile(source) error = %v, want nil", err)
	}
	targetData, err := os.ReadFile(filepath.Join(target, "large.bin"))
	if err != nil {
		t.Fatalf("os.ReadFile(target) error = %v, want nil", err)
	}
	if !bytes.Equal(targetData, sourceData) {
		t.Fatalf("resumed target payload differs from source")
	}
}

func TestRunFailsClosedWhenFailedRetryLacksPriorPayloadEvidence(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "large.bin", []byte("abcdefghijklmnop"))
	target := t.TempDir()
	sessionID := "session-failed-retry-missing-prior"
	req := validTransferRequest(source, scanResult, sessionID)
	req.PrivacyPolicy.BatchMaxCount = 1
	remaining := []byte("ijklmnop")
	client := fastJitterClient("http://receiver.invalid", 8, doerFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/sessions":
			return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
				SessionID: sessionID,
				State:     protocol.SessionStateValidated,
				ResumeFrom: []protocol.FileStatus{{
					Path:           "large.bin",
					ExpectedSize:   16,
					ExpectedDigest: scanResult.Entries[0].Digest,
					CommittedSize:  int64(len("abcdefgh")),
					Complete:       false,
				}},
			}), nil
		case "/v1/sessions/" + sessionID + "/status":
			return jsonHTTPResponse(http.StatusOK, protocol.SessionStatusResponse{
				SessionID: sessionID,
				State:     protocol.SessionStateValidated,
				Files: []protocol.FileStatus{{
					Path:           "large.bin",
					ExpectedSize:   16,
					ExpectedDigest: scanResult.Entries[0].Digest,
					CommittedSize:  int64(len("abcdefgh")),
					Complete:       false,
				}},
			}), nil
		case "/v1/chunks":
			var chunk protocol.ChunkUploadRequest
			if err := json.NewDecoder(req.Body).Decode(&chunk); err != nil {
				t.Fatalf("json.Decode(chunk) error = %v, want nil", err)
			}
			if chunk.Offset != 8 || !bytes.Equal(chunk.Data, remaining) || !chunk.Final {
				t.Fatalf("chunk request = offset %d final %t data %q, want resumed suffix", chunk.Offset, chunk.Final, chunk.Data)
			}
			return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
				SessionID:     sessionID,
				Path:          "large.bin",
				CommittedSize: 16,
				ChunkState:    protocol.ChunkStateAccepted,
				Complete:      true,
			}), nil
		case "/v1/commit":
			return nil, errors.New("simulated interruption before commit")
		default:
			return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
		}
	}))

	got, err := Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: profilePrivacyPolicyForRequest(req),
		Request:              req,
		Client:               client,
		Now:                  fixedNow,
	})
	if !errors.Is(err, ErrPayloadOverheadMissing) {
		t.Fatalf("Run(failed retry missing prior) error = %v, want ErrPayloadOverheadMissing", err)
	}
	if got.ClientResult.Chunks != 0 || got.ClientResult.Bytes != 0 {
		t.Fatalf("Run(failed retry missing prior) payload = bytes %d chunks %d, want blocked before upload", got.ClientResult.Bytes, got.ClientResult.Chunks)
	}
	assertNetworkTransferState(t, got.Transfer, control.NetworkTransferNeedsRepair, "network_transfer_artifact", "payload_overhead_missing")

	transfer := readNetworkTransfer(t, target, sessionID)
	assertNetworkTransferState(t, transfer, control.NetworkTransferNeedsRepair, "network_transfer_artifact", "payload_overhead_missing")
	if transfer.PrivacyOverhead != nil {
		t.Fatalf("network transfer privacy overhead = %+v, want nil when prior payload evidence is missing", transfer.PrivacyOverhead)
	}
}

func TestRunFailsClosedWhenRemoteFailedRetryHasNoPriorArtifact(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "large.bin", []byte("abcdefghijklmnop"))
	sessionID := "session-remote-failed-retry-missing-prior"
	req := validTransferRequest(source, scanResult, sessionID)
	req.PrivacyPolicy.BatchMaxCount = 1
	remaining := []byte("ijklmnop")
	writer := &missingPriorArtifactWriter{}
	client := fastJitterClient("http://receiver.invalid", 8, doerFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/sessions":
			return jsonHTTPResponse(http.StatusAccepted, protocol.BeginSessionResponse{
				SessionID: sessionID,
				State:     protocol.SessionStateValidated,
				ResumeFrom: []protocol.FileStatus{{
					Path:           "large.bin",
					ExpectedSize:   16,
					ExpectedDigest: scanResult.Entries[0].Digest,
					CommittedSize:  int64(len("abcdefgh")),
					Complete:       false,
				}},
			}), nil
		case "/v1/sessions/" + sessionID + "/status":
			return jsonHTTPResponse(http.StatusOK, protocol.SessionStatusResponse{
				SessionID: sessionID,
				State:     protocol.SessionStateValidated,
				Files: []protocol.FileStatus{{
					Path:           "large.bin",
					ExpectedSize:   16,
					ExpectedDigest: scanResult.Entries[0].Digest,
					CommittedSize:  int64(len("abcdefgh")),
					Complete:       false,
				}},
			}), nil
		case "/v1/chunks":
			var chunk protocol.ChunkUploadRequest
			if err := json.NewDecoder(req.Body).Decode(&chunk); err != nil {
				t.Fatalf("json.Decode(chunk) error = %v, want nil", err)
			}
			if chunk.Offset != 8 || !bytes.Equal(chunk.Data, remaining) || !chunk.Final {
				t.Fatalf("chunk request = offset %d final %t data %q, want resumed suffix", chunk.Offset, chunk.Final, chunk.Data)
			}
			return jsonHTTPResponse(http.StatusAccepted, protocol.ChunkUploadResponse{
				SessionID:     sessionID,
				Path:          "large.bin",
				CommittedSize: 16,
				ChunkState:    protocol.ChunkStateAccepted,
				Complete:      true,
			}), nil
		case "/v1/commit":
			return nil, errors.New("simulated interruption before commit")
		default:
			return nil, fmt.Errorf("unexpected path %s", req.URL.Path)
		}
	}))

	got, err := Run(context.Background(), Options{
		ArtifactWriter:       writer,
		ProfilePrivacyPolicy: profilePrivacyPolicyForRequest(req),
		Request:              req,
		Client:               client,
		Now:                  fixedNow,
	})
	if !errors.Is(err, ErrPayloadOverheadMissing) {
		t.Fatalf("Run(remote failed retry missing prior) error = %v, want ErrPayloadOverheadMissing", err)
	}
	assertNetworkTransferState(t, got.Transfer, control.NetworkTransferNeedsRepair, "network_transfer_artifact", "payload_overhead_missing")
	if len(writer.transfers) != 1 {
		t.Fatalf("network transfer writes = %d, want one needs_repair artifact", len(writer.transfers))
	}
	transfer := writer.transfers[0]
	assertNetworkTransferState(t, transfer, control.NetworkTransferNeedsRepair, "network_transfer_artifact", "payload_overhead_missing")
	if transfer.PrivacyOverhead != nil {
		t.Fatalf("network transfer privacy overhead = %+v, want nil when prior artifact is missing", transfer.PrivacyOverhead)
	}
}

func TestRunRecordsPublishFailedWhenWarningArtifactCannotBeWritten(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "data.txt"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(data.txt) error = %v, want nil", err)
	}
	reservedDir := filepath.Join(source, control.DirName)
	if err := os.Mkdir(reservedDir, 0o755); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", reservedDir, err)
	}
	if err := os.WriteFile(filepath.Join(reservedDir, "source-control.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source control) error = %v, want nil", err)
	}
	scanResult, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}
	warningsPath := filepath.Join(control.ControlDir(target), "warnings")
	if err := os.MkdirAll(control.ControlDir(target), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(control dir) error = %v, want nil", err)
	}
	if err := os.WriteFile(warningsPath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", warningsPath, err)
	}

	server := httptest.NewServer(receiver.NewHandler(receiver.FileStore{TargetRoot: target}))
	defer server.Close()
	sessionID := "session-warning-write-failed"

	_, err = Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: validProfilePrivacyPolicy(),
		Request:              validTransferRequest(source, scanResult, sessionID),
		Client:               fastJitterClient(server.URL, 4, server.Client()),
		Now:                  fixedNow,
	})
	if err == nil {
		t.Fatalf("Run(warning write failure) error = nil, want warning artifact write error")
	}

	transfer := readNetworkTransfer(t, target, sessionID)
	assertNetworkTransferState(t, transfer, control.NetworkTransferPublishFailed, "warning_artifacts", "warning_artifact_write_failed")
	if transfer.Error == "" {
		t.Fatalf("publish failed network transfer error is empty, want warning artifact write detail")
	}
	if transfer.PrivacyOverhead == nil || transfer.PrivacyOverhead.PaddedChunks == 0 || transfer.PrivacyOverhead.PaddingBytes == 0 {
		t.Fatalf("publish failed network transfer privacy overhead = %+v, want retained padding overhead", transfer.PrivacyOverhead)
	}
	if transfer.PrivacyOverhead.PaddingBucketBytes != transport.DefaultPrivacyPolicy(transport.PrivacyLevel2).PaddingBucket {
		t.Fatalf("publish failed network transfer padding bucket = %d, want default level 2 bucket", transfer.PrivacyOverhead.PaddingBucketBytes)
	}
	if transfer.PrivacyOverhead.JitteredRequests == 0 || transfer.PrivacyOverhead.JitterBudgetMillis != transfer.PrivacyPolicy.JitterBudget {
		t.Fatalf("publish failed network transfer privacy overhead = %+v, want retained jitter overhead", transfer.PrivacyOverhead)
	}
}

func TestRunLeavesPublishFailedEvidenceWhenFinalTransferWriteFails(t *testing.T) {
	source, scanResult := scanSingleFileSource(t, "data.txt", []byte("payload"))
	target := t.TempDir()
	server := httptest.NewServer(receiver.NewHandler(receiver.FileStore{TargetRoot: target}))
	defer server.Close()
	sessionID := "session-final-transfer-write-failed"
	var networkTransferWrites int
	originalWrite := writeControlFile
	writeControlFile = func(path string, doc control.Document) error {
		if _, ok := doc.(control.NetworkTransfer); ok {
			networkTransferWrites++
			if networkTransferWrites == 5 {
				return errFinalTransferWrite
			}
		}
		return originalWrite(path, doc)
	}
	defer func() {
		writeControlFile = originalWrite
	}()

	got, err := Run(context.Background(), Options{
		TargetRoot:           target,
		ProfilePrivacyPolicy: validProfilePrivacyPolicy(),
		Request:              validTransferRequest(source, scanResult, sessionID),
		Client:               fastJitterClient(server.URL, 4, server.Client()),
		Now:                  fixedNow,
	})
	if !errors.Is(err, errFinalTransferWrite) {
		t.Fatalf("Run(final transfer write failure) error = %v, want errFinalTransferWrite", err)
	}
	assertNetworkTransferState(t, got.Transfer, control.NetworkTransferPublishFailed, "network_transfer_artifact", "network_transfer_artifact_write_failed")

	transfer := readNetworkTransfer(t, target, sessionID)
	assertNetworkTransferState(t, transfer, control.NetworkTransferPublishFailed, "network_transfer_artifact", "network_transfer_artifact_pending")
}

func assertNetworkTransferState(t *testing.T, transfer control.NetworkTransfer, status control.NetworkTransferStatus, stage, code string) {
	t.Helper()
	if transfer.Status != status || transfer.Stage != stage || transfer.ErrorCode != code {
		t.Fatalf("network transfer = %+v, want status=%q stage=%q error_code=%q", transfer, status, stage, code)
	}
	if len(transfer.Attempts) != 1 {
		t.Fatalf("network transfer attempts = %d, want 1", len(transfer.Attempts))
	}
	attempt := transfer.Attempts[0]
	if attempt.Status != status || attempt.Stage != stage || attempt.ErrorCode != code {
		t.Fatalf("network transfer attempt = %+v, want status=%q stage=%q error_code=%q", attempt, status, stage, code)
	}
}

func readNetworkTransfer(t *testing.T, targetRoot, sessionID string) control.NetworkTransfer {
	t.Helper()
	path, err := control.Path(targetRoot, control.ArtifactNetworkTransfer, sessionID)
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	doc, err := control.ReadFile[control.NetworkTransfer](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	return doc
}

func readControlDoc[T control.Document](t *testing.T, target string, artifact control.ArtifactType, id string) T {
	t.Helper()
	path, err := control.Path(target, artifact, id)
	if err != nil {
		t.Fatalf("control.Path(%q, %q) error = %v, want nil", artifact, id, err)
	}
	doc, err := control.ReadFile[T](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	return doc
}

func profileSnapshotFixture(t *testing.T, sessionID string) control.ProfileSnapshot {
	t.Helper()
	p := profile.NewDefault("profile.default", "Profile", t.TempDir(), t.TempDir())
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal(profile) error = %v, want nil", err)
	}
	return control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + sessionID,
		ProfileID:  p.ProfileID,
		SessionID:  sessionID,
		CapturedAt: fixedNow().UTC().Format(time.RFC3339Nano),
		Profile:    data,
	}
}

func validPublishedNetworkTransfer(sessionID string) control.NetworkTransfer {
	req := validTransferRequest("unused-source", scan.Result{}, sessionID)
	doc := newTransfer(req, fixedNow())
	doc.PrivacyOverhead = &control.NetworkTransferPrivacyOverhead{
		FramePlainBytes:    128,
		FrameWireBytes:     192,
		PaddingBytes:       64,
		PaddedChunks:       1,
		PaddingBucketBytes: req.PrivacyPolicy.PaddingBucket,
		BatchFrames:        1,
		BatchedChunks:      1,
		MaxBatchCount:      1,
		MaxBatchPlainBytes: 128,
		JitteredRequests:   1,
		JitterBudgetMillis: req.PrivacyPolicy.JitterBudget,
	}
	return finishTransfer(doc, outcome{
		status: control.NetworkTransferPublished,
		stage:  "commit",
	}, fixedNow().Add(time.Minute), nil)
}

func validTransferRequest(source string, result scan.Result, sessionID string) protocolclient.TransferRequest {
	now := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)
	privacyPolicy := transport.DefaultPrivacyPolicy(transport.PrivacyLevel2)
	privacyPolicy.JitterBudget = 1
	return protocolclient.TransferRequest{
		SourceRoot:     source,
		Scan:           result,
		SessionID:      sessionID,
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: "sha256:abcdef0123456789",
		TargetDeviceID: "sha256:0123456789abcdef",
		PrivacyPolicy:  privacyPolicy,
		RootID:         "root1",
		CreatedAt:      now,
		EndedAt:        now.Add(time.Minute),
	}
}

func validProfilePrivacyPolicy() profile.PrivacyPolicy {
	policy := profile.DefaultPrivacyPolicy()
	policy.JitterBudgetMillis = 1
	return policy
}

func profilePrivacyPolicyForRequest(req protocolclient.TransferRequest) profile.PrivacyPolicy {
	return profile.PrivacyPolicy{
		Mode:                    profile.PrivacyModePlaintext,
		TrafficLevel:            int(req.PrivacyPolicy.Level),
		AllowPlaintextRestore:   true,
		AllowHiddenFiles:        true,
		AllowSensitiveFilenames: true,
		PaddingBucketBytes:      req.PrivacyPolicy.PaddingBucket,
		BatchMaxBytes:           req.PrivacyPolicy.BatchMaxBytes,
		BatchMaxCount:           req.PrivacyPolicy.BatchMaxCount,
		JitterBudgetMillis:      req.PrivacyPolicy.JitterBudget,
		DiscoveryLowInfo:        req.PrivacyPolicy.DiscoveryLowInfo,
	}
}

func statusFromScan(result scan.Result, sessionID string, state protocol.SessionState) protocol.SessionStatusResponse {
	status := protocol.SessionStatusResponse{
		SessionID: sessionID,
		State:     state,
	}
	for _, entry := range result.Entries {
		if entry.Kind != scan.KindRegular {
			continue
		}
		status.Files = append(status.Files, protocol.FileStatus{
			Path:           entry.Path,
			ExpectedSize:   entry.Size,
			ExpectedDigest: entry.Digest,
			CommittedSize:  0,
			Complete:       false,
		})
	}
	return status
}

func scanSingleFileSource(t *testing.T, name string, data []byte) (string, scan.Result) {
	t.Helper()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, name), data, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", name, err)
	}
	result, err := scan.Scan(source)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", source, err)
	}
	return source, result
}

func jsonHTTPResponse(status int, value any) *http.Response {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(value)
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(&buf),
	}
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func fastJitterClient(baseURL string, chunkSize int, doer protocolclient.Doer) protocolclient.Client {
	return protocolclient.Client{
		BaseURL:   baseURL,
		Doer:      doer,
		ChunkSize: chunkSize,
	}
}

type failAfterAcceptedChunkDoer struct {
	base      protocolclient.Doer
	failAfter int
	chunks    int
}

func (d *failAfterAcceptedChunkDoer) Do(req *http.Request) (*http.Response, error) {
	resp, err := d.base.Do(req)
	if err != nil {
		return resp, err
	}
	if req.Method == http.MethodPost && (req.URL.Path == "/v1/chunks" || req.URL.Path == "/v1/chunk-batches") {
		d.chunks++
		if d.chunks >= d.failAfter {
			if resp != nil && resp.Body != nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
			return nil, errors.New("simulated interruption after accepted chunk")
		}
	}
	return resp, nil
}

type missingPriorArtifactWriter struct {
	transfers []control.NetworkTransfer
}

func (w *missingPriorArtifactWriter) CanWriteBeforeBegin() bool {
	return false
}

func (w *missingPriorArtifactWriter) WriteProfileSnapshot(context.Context, control.ProfileSnapshot) error {
	return nil
}

func (w *missingPriorArtifactWriter) WriteWarnings(context.Context, []control.Warning) error {
	return nil
}

func (w *missingPriorArtifactWriter) WriteNetworkTransfer(_ context.Context, doc control.NetworkTransfer) error {
	w.transfers = append(w.transfers, doc)
	return nil
}

func (w *missingPriorArtifactWriter) ReadNetworkTransfer(context.Context, string) (control.NetworkTransfer, error) {
	return control.NetworkTransfer{}, os.ErrNotExist
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 18, 9, 30, 0, 123, time.UTC)
}

var errFinalTransferWrite = errors.New("final network transfer artifact write failed")
