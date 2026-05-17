package networkpush

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/networkrun"
	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/protocolclient"
	"github.com/khicago/supermover/internal/receiver"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/tlsidentity"
	"github.com/khicago/supermover/internal/transport"
)

var testNow = time.Date(2026, 5, 19, 10, 11, 12, 0, time.UTC)

func TestRunLoopbackPushesFromProfileOnly(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.txt"), []byte("network push\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)

	server := newTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)

	got, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: "session-networkpush-loopback",
		Now:       func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatalf("Run(loopback) error = %v, want nil", err)
	}
	if got.SessionID != "session-networkpush-loopback" || got.Files != 1 || got.Bytes != int64(len("network push\n")) || got.Chunks != 1 {
		t.Fatalf("Run(loopback) result = %+v, want one transferred file", got)
	}
	if got.TransferStatus != control.NetworkTransferPublished || got.TransferStage != "commit" || got.TransferError != "" {
		t.Fatalf("Run(loopback) transfer result = %+v, want published commit", got)
	}
	payload, err := os.ReadFile(filepath.Join(targetRoot, "data.txt"))
	if err != nil {
		t.Fatalf("os.ReadFile(target) error = %v, want nil", err)
	}
	if string(payload) != "network push\n" {
		t.Fatalf("target payload = %q, want source payload", string(payload))
	}
	snapshot := readControlDoc[control.ProfileSnapshot](t, targetRoot, control.ArtifactProfileSnapshot, "profile-session-networkpush-loopback")
	if snapshot.ProfileID != prof.ProfileID || snapshot.SessionID != "session-networkpush-loopback" {
		t.Fatalf("profile snapshot = %+v, want receiver-side profile evidence", snapshot)
	}
	transfer := readNetworkTransfer(t, targetRoot, "session-networkpush-loopback")
	if transfer.Status != control.NetworkTransferPublished || transfer.Stage != "commit" || transfer.PrivacyOverhead == nil {
		t.Fatalf("network transfer artifact = %+v, want receiver-side published evidence", transfer)
	}
}

func TestRunGeneratesDeterministicSessionID(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.txt"), []byte("deterministic\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	server := newTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)

	got, err := Run(context.Background(), Options{
		Profile: prof,
		Now:     func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatalf("Run(generated session) error = %v, want nil", err)
	}
	if got.SessionID != "session-20260519T101112Z" {
		t.Fatalf("generated session id = %q, want deterministic timestamp id", got.SessionID)
	}
}

func TestRunResumesInterruptedUploadAfterReceiverRestart(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceFile := filepath.Join(sourceRoot, "large.bin")
	if err := writePatternFile(sourceFile, protocol.MaxChunkBytes+137); err != nil {
		t.Fatalf("writePatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	sessionID := "session-networkpush-resume-restart"
	begin := beginRequestForProfile(t, prof, sessionID)
	prefixSize := protocol.MaxChunkBytes
	store := receiver.FileStore{TargetRoot: targetRoot, Now: func() time.Time { return testNow }}
	if _, err := store.Begin(begin); err != nil {
		t.Fatalf("FileStore.Begin(partial) error = %v, want nil", err)
	}
	prefix := readFilePrefix(t, sourceFile, prefixSize)
	if _, err := store.AppendChunk(protocol.ChunkUploadRequest{
		SessionID: sessionID,
		Path:      "large.bin",
		Offset:    0,
		Data:      prefix,
		Digest:    digestBytes(prefix),
	}); err != nil {
		t.Fatalf("FileStore.AppendChunk(prefix) error = %v, want nil", err)
	}
	priorTransfer := networkTransferForBegin(begin, control.NetworkTransferFailed)
	if _, err := store.WriteNetworkTransfer(protocol.NetworkTransferArtifactRequest{
		SessionID: sessionID,
		Document:  marshalNetworkPushControlDoc(t, priorTransfer),
	}); err != nil {
		t.Fatalf("FileStore.WriteNetworkTransfer(prior partial evidence) error = %v, want nil", err)
	}

	firstServer, firstCounts := newCountingTLSReceiverServer(t, prof, targetCert)
	prof.Network = networkConfig(t, sourceCert, firstServer.URL)
	first, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: sessionID,
		Now:       func() time.Time { return testNow },
	})
	firstServer.Close()
	if err != nil {
		t.Fatalf("Run(resume existing partial) error = %v, want nil", err)
	}
	firstSnapshot := firstCounts.snapshot()
	if firstSnapshot.begin != 1 || firstSnapshot.status == 0 || firstSnapshot.commits != 1 {
		t.Fatalf("first receiver counts = %+v, want begin/status/commit calls", firstSnapshot)
	}
	if firstSnapshot.chunkRequests != 1 {
		t.Fatalf("first resumed chunk requests = %d, want one suffix chunk request", firstSnapshot.chunkRequests)
	}
	wantRemaining := int64(137)
	if first.Bytes != wantRemaining {
		t.Fatalf("Run(resume existing partial).Bytes = %d, want remaining bytes %d", first.Bytes, wantRemaining)
	}
	if first.TransferStatus != control.NetworkTransferPublished || first.TransferStage != "commit" {
		t.Fatalf("Run(resume existing partial) result = %+v, want published commit", first)
	}
	if hashFile(t, filepath.Join(targetRoot, "large.bin")) != hashFile(t, sourceFile) {
		t.Fatalf("published large file digest differs from source")
	}
	published := readNetworkTransfer(t, targetRoot, sessionID)
	if published.Status != control.NetworkTransferPublished || published.PrivacyOverhead == nil || published.PrivacyOverhead.PaddedChunks == 0 {
		t.Fatalf("published network transfer = %+v, want published evidence with resumed overhead", published)
	}

	secondServer, secondCounts := newCountingTLSReceiverServer(t, prof, targetCert)
	defer secondServer.Close()
	prof.Network = networkConfig(t, sourceCert, secondServer.URL)
	second, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: sessionID,
		Now:       func() time.Time { return testNow.Add(5 * time.Minute) },
	})
	if err != nil {
		t.Fatalf("Run(retry after receiver restart) error = %v, want nil", err)
	}
	if second.TransferStatus != control.NetworkTransferPublished || second.Bytes != 0 || second.Chunks != 0 {
		t.Fatalf("Run(retry after receiver restart) result = %+v, want published with no chunks", second)
	}
	if second.ResumeOutcome != "published_retry" {
		t.Fatalf("Run(retry after receiver restart).ResumeOutcome = %q, want published_retry", second.ResumeOutcome)
	}
	secondSnapshot := secondCounts.snapshot()
	if secondSnapshot.chunkRequests != 0 || secondSnapshot.commits != 1 {
		t.Fatalf("second receiver counts = %+v, want no chunks and one idempotent commit", secondSnapshot)
	}
	afterRetry := readNetworkTransfer(t, targetRoot, sessionID)
	if afterRetry.PrivacyOverhead == nil ||
		afterRetry.PrivacyOverhead.PaddedChunks != published.PrivacyOverhead.PaddedChunks ||
		afterRetry.PrivacyOverhead.BatchFrames != published.PrivacyOverhead.BatchFrames ||
		afterRetry.PrivacyOverhead.BatchedChunks != published.PrivacyOverhead.BatchedChunks {
		t.Fatalf("network transfer overhead after published retry = %+v, want preserved payload overhead %+v", afterRetry.PrivacyOverhead, published.PrivacyOverhead)
	}
	if len(afterRetry.Attempts) != len(published.Attempts)+1 {
		t.Fatalf("network transfer attempts after retry = %d, want %d", len(afterRetry.Attempts), len(published.Attempts)+1)
	}
}

func TestRunResumesStartedChunkTransferFromReceiverStatus(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceFile := filepath.Join(sourceRoot, "large.bin")
	if err := writePatternFile(sourceFile, protocol.MaxChunkBytes+137); err != nil {
		t.Fatalf("writePatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	sessionID := "session-networkpush-started-chunk-resume"
	begin := beginRequestForProfile(t, prof, sessionID)
	store := receiver.FileStore{TargetRoot: targetRoot, Now: func() time.Time { return testNow }}
	if _, err := store.Begin(begin); err != nil {
		t.Fatalf("FileStore.Begin(partial) error = %v, want nil", err)
	}
	prefix := readFilePrefix(t, sourceFile, protocol.MaxChunkBytes)
	if _, err := store.AppendChunk(protocol.ChunkUploadRequest{
		SessionID: sessionID,
		Path:      "large.bin",
		Offset:    0,
		Data:      prefix,
		Digest:    digestBytes(prefix),
	}); err != nil {
		t.Fatalf("FileStore.AppendChunk(prefix) error = %v, want nil", err)
	}
	priorTransfer := networkTransferForBegin(begin, control.NetworkTransferStarted)
	priorTransfer.Stage = "chunk"
	priorTransfer.ErrorCode = ""
	priorTransfer.Error = ""
	priorTransfer.UpdatedAt = testNow.Add(30 * time.Second).Format(time.RFC3339Nano)
	priorTransfer.Attempts[0].Stage = "chunk"
	priorTransfer.Attempts[0].Status = control.NetworkTransferStarted
	priorTransfer.Attempts[0].EndedAt = ""
	priorTransfer.Attempts[0].ErrorCode = ""
	priorTransfer.Attempts[0].Error = ""
	priorOverhead := *priorTransfer.PrivacyOverhead
	if _, err := store.WriteNetworkTransfer(protocol.NetworkTransferArtifactRequest{
		SessionID: sessionID,
		Document:  marshalNetworkPushControlDoc(t, priorTransfer),
	}); err != nil {
		t.Fatalf("FileStore.WriteNetworkTransfer(started chunk evidence) error = %v, want nil", err)
	}

	server, counts := newCountingTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)
	got, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: sessionID,
		Now:       func() time.Time { return testNow.Add(5 * time.Minute) },
	})
	if err != nil {
		t.Fatalf("Run(resume started chunk) error = %v, want nil", err)
	}
	wantRemaining := int64(137)
	if got.TransferStatus != control.NetworkTransferPublished || got.TransferStage != "commit" {
		t.Fatalf("Run(resume started chunk) result = %+v, want published commit", got)
	}
	if got.ResumeAuthority != "receiver_status" || got.ResumeOutcome != "resumed" || got.ResumedBytes != wantRemaining {
		t.Fatalf("Run(resume started chunk) resume = %s/%s/%d, want receiver_status/resumed/%d", got.ResumeAuthority, got.ResumeOutcome, got.ResumedBytes, wantRemaining)
	}
	if got.Bytes != wantRemaining || got.Chunks != 1 {
		t.Fatalf("Run(resume started chunk) payload = bytes %d chunks %d, want suffix only", got.Bytes, got.Chunks)
	}
	snapshot := counts.snapshot()
	if snapshot.begin != 1 || snapshot.status == 0 || snapshot.chunkRequests != 1 || snapshot.commits != 1 {
		t.Fatalf("receiver calls for started chunk resume = %+v, want begin/status plus one suffix chunk and commit", snapshot)
	}
	if hashFile(t, filepath.Join(targetRoot, "large.bin")) != hashFile(t, sourceFile) {
		t.Fatalf("published large file digest differs from source")
	}
	published := readNetworkTransfer(t, targetRoot, sessionID)
	if published.Status != control.NetworkTransferPublished || published.Stage != "commit" || published.PrivacyOverhead == nil {
		t.Fatalf("published transfer = %+v, want published commit with payload overhead", published)
	}
	if published.PrivacyOverhead.PaddedChunks <= priorOverhead.PaddedChunks ||
		published.PrivacyOverhead.BatchFrames <= priorOverhead.BatchFrames ||
		published.PrivacyOverhead.BatchedChunks <= priorOverhead.BatchedChunks {
		t.Fatalf("published overhead = %+v, want merged prior started/chunk overhead %+v plus resumed suffix overhead", published.PrivacyOverhead, priorOverhead)
	}
	if len(published.Attempts) != len(priorTransfer.Attempts)+1 {
		t.Fatalf("published attempts = %d, want prior started/chunk attempt plus current attempt", len(published.Attempts))
	}
	firstAttempt := published.Attempts[0]
	lastAttempt := published.Attempts[len(published.Attempts)-1]
	if firstAttempt.Status != control.NetworkTransferStarted || firstAttempt.Stage != "chunk" {
		t.Fatalf("first attempt = %+v, want prior in-flight started/chunk evidence preserved", firstAttempt)
	}
	if lastAttempt.Status != control.NetworkTransferPublished || lastAttempt.Stage != "commit" {
		t.Fatalf("last attempt = %+v, want current published commit attempt", lastAttempt)
	}
}

func TestRunFailsClosedWhenPartialReceiverStateLacksPriorPayloadEvidence(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceFile := filepath.Join(sourceRoot, "large.bin")
	if err := writePatternFile(sourceFile, protocol.MaxChunkBytes+137); err != nil {
		t.Fatalf("writePatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	sessionID := "session-networkpush-partial-missing-transfer"
	begin := beginRequestForProfile(t, prof, sessionID)
	store := receiver.FileStore{TargetRoot: targetRoot, Now: func() time.Time { return testNow }}
	if _, err := store.Begin(begin); err != nil {
		t.Fatalf("FileStore.Begin(partial) error = %v, want nil", err)
	}
	prefix := readFilePrefix(t, sourceFile, protocol.MaxChunkBytes)
	if _, err := store.AppendChunk(protocol.ChunkUploadRequest{
		SessionID: sessionID,
		Path:      "large.bin",
		Offset:    0,
		Data:      prefix,
		Digest:    digestBytes(prefix),
	}); err != nil {
		t.Fatalf("FileStore.AppendChunk(prefix) error = %v, want nil", err)
	}

	server, counts := newCountingTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)
	got, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: sessionID,
		Now:       func() time.Time { return testNow },
	})
	if !errors.Is(err, networkrun.ErrPayloadOverheadMissing) {
		t.Fatalf("Run(partial missing prior) error = %v, want ErrPayloadOverheadMissing", err)
	}
	if got.TransferStatus != control.NetworkTransferNeedsRepair || got.TransferStage != "network_transfer_artifact" || got.TransferCode != "payload_overhead_missing" || got.ResumeOutcome != "blocked" {
		t.Fatalf("Run(partial missing prior) result = %+v, want blocked needs_repair", got)
	}
	snapshot := counts.snapshot()
	if snapshot.chunkRequests != 0 || snapshot.commits != 0 {
		t.Fatalf("partial missing prior calls = %+v, want blocked before suffix upload or commit", snapshot)
	}
	if got.Bytes != 0 || got.Chunks != 0 {
		t.Fatalf("Run(partial missing prior) payload = bytes %d chunks %d, want blocked before upload", got.Bytes, got.Chunks)
	}
	transfer := readNetworkTransfer(t, targetRoot, sessionID)
	if transfer.Status != control.NetworkTransferNeedsRepair || transfer.PrivacyOverhead != nil {
		t.Fatalf("partial missing prior transfer = %+v, want needs_repair without fabricated overhead", transfer)
	}
}

func TestRunFailsClosedWhenFailedPartialRetryLacksPriorPayloadEvidence(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceFile := filepath.Join(sourceRoot, "large.bin")
	if err := writePatternFile(sourceFile, protocol.MaxChunkBytes+137); err != nil {
		t.Fatalf("writePatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	sessionID := "session-networkpush-failed-retry-missing-transfer"
	begin := beginRequestForProfile(t, prof, sessionID)
	store := receiver.FileStore{TargetRoot: targetRoot, Now: func() time.Time { return testNow }}
	if _, err := store.Begin(begin); err != nil {
		t.Fatalf("FileStore.Begin(partial) error = %v, want nil", err)
	}
	prefix := readFilePrefix(t, sourceFile, protocol.MaxChunkBytes)
	if _, err := store.AppendChunk(protocol.ChunkUploadRequest{
		SessionID: sessionID,
		Path:      "large.bin",
		Offset:    0,
		Data:      prefix,
		Digest:    digestBytes(prefix),
	}); err != nil {
		t.Fatalf("FileStore.AppendChunk(prefix) error = %v, want nil", err)
	}

	server, counts := newCountingTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)
	got, err := runWithClientDoer(context.Background(), prof, sessionID, func(base protocolclient.Doer) protocolclient.Doer {
		return &failBeforeCommitDoer{base: base}
	}, func() time.Time { return testNow }, protocol.MaxChunkBytes)
	if !errors.Is(err, networkrun.ErrPayloadOverheadMissing) {
		t.Fatalf("Run(failed partial missing prior) error = %v, want ErrPayloadOverheadMissing", err)
	}
	if got.TransferStatus != control.NetworkTransferNeedsRepair || got.TransferStage != "network_transfer_artifact" || got.TransferCode != "payload_overhead_missing" || got.ResumeOutcome != "blocked" {
		t.Fatalf("Run(failed partial missing prior) result = %+v, want blocked needs_repair", got)
	}
	if got.Bytes != 0 || got.Chunks != 0 {
		t.Fatalf("Run(failed partial missing prior) payload = bytes %d chunks %d, want blocked before upload", got.Bytes, got.Chunks)
	}
	snapshot := counts.snapshot()
	if snapshot.chunkRequests != 0 || snapshot.commits != 0 {
		t.Fatalf("failed partial missing prior calls = %+v, want blocked before suffix upload or commit", snapshot)
	}
	blocked := readNetworkTransfer(t, targetRoot, sessionID)
	if blocked.Status != control.NetworkTransferNeedsRepair || blocked.PrivacyOverhead != nil {
		t.Fatalf("failed partial missing prior transfer = %+v, want needs_repair without fabricated overhead", blocked)
	}

	counts.reset()
	second, secondErr := runWithClientDoer(context.Background(), prof, sessionID, nil, func() time.Time { return testNow.Add(5 * time.Minute) }, protocol.MaxChunkBytes)
	if !errors.Is(secondErr, networkrun.ErrPayloadOverheadMissing) {
		t.Fatalf("Run(commit-only after blocked partial) error = %v, want ErrPayloadOverheadMissing", secondErr)
	}
	if second.TransferStatus != control.NetworkTransferNeedsRepair || second.TransferStage != "network_transfer_artifact" || second.TransferCode != "payload_overhead_missing" || second.ResumeOutcome != "blocked" {
		t.Fatalf("Run(commit-only after blocked partial) result = %+v, want blocked needs_repair", second)
	}
	if snapshot := counts.snapshot(); snapshot.chunkRequests != 0 || snapshot.commits != 0 {
		t.Fatalf("commit-only after blocked partial calls = %+v, want blocked before commit retry", snapshot)
	}
	afterRetry := readNetworkTransfer(t, targetRoot, sessionID)
	if afterRetry.Status != control.NetworkTransferNeedsRepair || afterRetry.PrivacyOverhead != nil {
		t.Fatalf("commit-only after blocked partial transfer = %+v, want needs_repair without published overhead", afterRetry)
	}
}

func TestRunMergesInterruptedOperatorAttemptPrivacyOverhead(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceFile := filepath.Join(sourceRoot, "large.bin")
	if err := writePatternFile(sourceFile, 16*3+5); err != nil {
		t.Fatalf("writePatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	sessionID := "session-networkpush-operator-interrupted"
	server, counts := newCountingTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)
	prof.PrivacyPolicy.BatchMaxCount = 1

	first, err := runWithClientDoer(context.Background(), prof, sessionID, func(base protocolclient.Doer) protocolclient.Doer {
		return &failAfterAcceptedChunkDoer{base: base, failAfter: 1}
	}, func() time.Time { return testNow }, 16)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption after accepted chunk") {
		t.Fatalf("Run(interrupted operator push) error = %v, want simulated interruption", err)
	}
	if first.TransferStatus != control.NetworkTransferFailed || first.TransferStage != "transport" || first.TransferCode != "transfer_failed" {
		t.Fatalf("Run(interrupted operator push) result = %+v, want failed transport evidence", first)
	}
	interrupted := readNetworkTransfer(t, targetRoot, sessionID)
	if interrupted.PrivacyOverhead == nil || interrupted.PrivacyOverhead.PaddedChunks == 0 || interrupted.PrivacyOverhead.BatchFrames == 0 {
		t.Fatalf("interrupted transfer overhead = %+v, want accepted payload overhead", interrupted.PrivacyOverhead)
	}
	counts.reset()

	second, err := runWithClientDoer(context.Background(), prof, sessionID, nil, func() time.Time { return testNow.Add(5 * time.Minute) }, 16)
	if err != nil {
		t.Fatalf("Run(resume interrupted operator push) error = %v, want nil", err)
	}
	if second.TransferStatus != control.NetworkTransferPublished || second.ResumeOutcome != "resumed" || second.ResumedBytes == 0 {
		t.Fatalf("Run(resume interrupted operator push) result = %+v, want published resumed transfer", second)
	}
	if hashFile(t, filepath.Join(targetRoot, "large.bin")) != hashFile(t, sourceFile) {
		t.Fatalf("resumed target large file digest differs from source")
	}
	published := readNetworkTransfer(t, targetRoot, sessionID)
	if published.Status != control.NetworkTransferPublished || published.PrivacyOverhead == nil {
		t.Fatalf("published transfer = %+v, want published overhead", published)
	}
	if published.PrivacyOverhead.PaddedChunks <= interrupted.PrivacyOverhead.PaddedChunks ||
		published.PrivacyOverhead.BatchFrames <= interrupted.PrivacyOverhead.BatchFrames ||
		len(published.Attempts) != len(interrupted.Attempts)+1 {
		t.Fatalf("published transfer overhead/attempts = overhead %+v attempts %d, want merged with interrupted %+v attempts %d", published.PrivacyOverhead, len(published.Attempts), interrupted.PrivacyOverhead, len(interrupted.Attempts))
	}
	if counts.snapshot().chunkRequests == 0 {
		t.Fatalf("resume chunk requests = 0, want remaining payload upload")
	}
}

func TestRunMergesFailedRetryWithoutDroppingPriorPayloadEvidence(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceFile := filepath.Join(sourceRoot, "large.bin")
	if err := writePatternFile(sourceFile, 16*3+5); err != nil {
		t.Fatalf("writePatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	sessionID := "session-networkpush-failed-retry-preserves-evidence"
	server, counts := newCountingTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)
	prof.PrivacyPolicy.BatchMaxCount = 1

	first, err := runWithClientDoer(context.Background(), prof, sessionID, func(base protocolclient.Doer) protocolclient.Doer {
		return &failAfterAcceptedChunkDoer{base: base, failAfter: 1}
	}, func() time.Time { return testNow }, 16)
	if err == nil {
		t.Fatalf("Run(first interrupted push) error = nil, want simulated interruption")
	}
	if first.TransferStatus != control.NetworkTransferFailed {
		t.Fatalf("Run(first interrupted push) result = %+v, want failed evidence", first)
	}
	prior := readNetworkTransfer(t, targetRoot, sessionID)
	if prior.PrivacyOverhead == nil || prior.PrivacyOverhead.PaddedChunks == 0 {
		t.Fatalf("prior transfer overhead = %+v, want accepted payload evidence", prior.PrivacyOverhead)
	}
	counts.reset()

	second, err := runWithClientDoer(context.Background(), prof, sessionID, func(base protocolclient.Doer) protocolclient.Doer {
		return &failAfterAcceptedChunkDoer{base: base, failAfter: 1}
	}, func() time.Time { return testNow.Add(5 * time.Minute) }, 16)
	if err == nil {
		t.Fatalf("Run(second interrupted push) error = nil, want simulated interruption")
	}
	if second.TransferStatus != control.NetworkTransferFailed || second.TransferStage != "transport" || second.TransferCode != "transfer_failed" {
		t.Fatalf("Run(second interrupted push) result = %+v, want failed transport evidence", second)
	}
	merged := readNetworkTransfer(t, targetRoot, sessionID)
	if merged.PrivacyOverhead == nil ||
		merged.PrivacyOverhead.PaddedChunks <= prior.PrivacyOverhead.PaddedChunks ||
		len(merged.Attempts) != len(prior.Attempts)+1 {
		t.Fatalf("merged failed retry transfer = overhead %+v attempts %d, want preserved+merged prior %+v attempts %d", merged.PrivacyOverhead, len(merged.Attempts), prior.PrivacyOverhead, len(prior.Attempts))
	}
	if counts.snapshot().chunkRequests == 0 {
		t.Fatalf("second interrupted retry chunk requests = 0, want resumed payload attempt")
	}
}

func TestRunPreservesPayloadEvidenceWhenRetryOnlyCommits(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceFile := filepath.Join(sourceRoot, "large.bin")
	if err := writePatternFile(sourceFile, 16*2); err != nil {
		t.Fatalf("writePatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	sessionID := "session-networkpush-commit-only-retry"
	server, counts := newCountingTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)
	prof.PrivacyPolicy.BatchMaxCount = 1

	first, err := runWithClientDoer(context.Background(), prof, sessionID, func(base protocolclient.Doer) protocolclient.Doer {
		return &failBeforeCommitDoer{base: base}
	}, func() time.Time { return testNow }, 16)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption before commit") {
		t.Fatalf("Run(commit interrupted after payload) error = %v, want simulated commit interruption", err)
	}
	if first.TransferStatus != control.NetworkTransferFailed || first.TransferStage != "transport" || first.TransferCode != "transfer_failed" {
		t.Fatalf("Run(commit interrupted after payload) result = %+v, want failed evidence", first)
	}
	prior := readNetworkTransfer(t, targetRoot, sessionID)
	if prior.PrivacyOverhead == nil || prior.PrivacyOverhead.PaddedChunks == 0 {
		t.Fatalf("prior commit-only retry overhead = %+v, want payload evidence", prior.PrivacyOverhead)
	}
	counts.reset()

	second, err := runWithClientDoer(context.Background(), prof, sessionID, nil, func() time.Time { return testNow.Add(5 * time.Minute) }, 16)
	if err != nil {
		t.Fatalf("Run(commit-only retry) error = %v, want nil", err)
	}
	if second.TransferStatus != control.NetworkTransferPublished || second.Bytes != 0 || second.Chunks != 0 || second.ResumeOutcome != "receiver_status" {
		t.Fatalf("Run(commit-only retry) result = %+v, want published commit-only receiver_status retry", second)
	}
	snapshot := counts.snapshot()
	if snapshot.chunkRequests != 0 || snapshot.commits != 1 {
		t.Fatalf("commit-only retry counts = %+v, want no chunks and one commit", snapshot)
	}
	published := readNetworkTransfer(t, targetRoot, sessionID)
	if published.Status != control.NetworkTransferPublished || published.PrivacyOverhead == nil ||
		published.PrivacyOverhead.PaddedChunks != prior.PrivacyOverhead.PaddedChunks ||
		len(published.Attempts) != len(prior.Attempts)+1 {
		t.Fatalf("commit-only published transfer = overhead %+v attempts %d, want preserved prior %+v attempts %d", published.PrivacyOverhead, len(published.Attempts), prior.PrivacyOverhead, len(prior.Attempts))
	}
}

func TestRunPreservesZeroBytePayloadEvidenceWhenRetryOnlyCommits(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "empty.txt"), nil, 0o600); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	sessionID := "session-networkpush-zero-byte-commit-only-retry"
	server, counts := newCountingTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)
	prof.PrivacyPolicy.BatchMaxCount = 1

	first, err := runWithClientDoer(context.Background(), prof, sessionID, func(base protocolclient.Doer) protocolclient.Doer {
		return &failBeforeCommitDoer{base: base}
	}, func() time.Time { return testNow }, 16)
	if err == nil || !strings.Contains(err.Error(), "simulated interruption before commit") {
		t.Fatalf("Run(zero-byte commit interrupted) error = %v, want simulated commit interruption", err)
	}
	if first.TransferStatus != control.NetworkTransferFailed || first.TransferStage != "transport" || first.TransferCode != "transfer_failed" || first.Bytes != 0 || first.Chunks != 1 {
		t.Fatalf("Run(zero-byte commit interrupted) result = %+v, want failed evidence after one zero-byte completion chunk", first)
	}
	prior := readNetworkTransfer(t, targetRoot, sessionID)
	if prior.PrivacyOverhead == nil ||
		prior.PrivacyOverhead.PaddedChunks == 0 ||
		prior.PrivacyOverhead.BatchFrames == 0 ||
		prior.PrivacyOverhead.BatchedChunks == 0 {
		t.Fatalf("prior zero-byte commit-only retry overhead = %+v, want payload evidence", prior.PrivacyOverhead)
	}
	counts.reset()

	second, err := runWithClientDoer(context.Background(), prof, sessionID, nil, func() time.Time { return testNow.Add(5 * time.Minute) }, 16)
	if err != nil {
		t.Fatalf("Run(zero-byte commit-only retry) error = %v, want nil", err)
	}
	if second.TransferStatus != control.NetworkTransferPublished || second.Bytes != 0 || second.Chunks != 0 || second.ResumeOutcome != "receiver_status" {
		t.Fatalf("Run(zero-byte commit-only retry) result = %+v, want published commit-only receiver_status retry", second)
	}
	snapshot := counts.snapshot()
	if snapshot.chunkRequests != 0 || snapshot.commits != 1 {
		t.Fatalf("zero-byte commit-only retry counts = %+v, want no chunks and one commit", snapshot)
	}
	targetInfo, err := os.Lstat(filepath.Join(targetRoot, "empty.txt"))
	if err != nil {
		t.Fatalf("os.Lstat(target zero-byte file) error = %v, want nil", err)
	}
	if !targetInfo.Mode().IsRegular() || targetInfo.Size() != 0 {
		t.Fatalf("target zero-byte file mode/size = %s/%d, want regular size 0", targetInfo.Mode(), targetInfo.Size())
	}
	published := readNetworkTransfer(t, targetRoot, sessionID)
	if published.Status != control.NetworkTransferPublished || published.PrivacyOverhead == nil ||
		published.PrivacyOverhead.PaddedChunks != prior.PrivacyOverhead.PaddedChunks ||
		published.PrivacyOverhead.BatchFrames != prior.PrivacyOverhead.BatchFrames ||
		published.PrivacyOverhead.BatchedChunks != prior.PrivacyOverhead.BatchedChunks ||
		len(published.Attempts) != len(prior.Attempts)+1 {
		t.Fatalf("zero-byte commit-only published transfer = overhead %+v attempts %d, want preserved prior %+v attempts %d", published.PrivacyOverhead, len(published.Attempts), prior.PrivacyOverhead, len(prior.Attempts))
	}
	if published.ErrorCode == "payload_overhead_missing" {
		t.Fatalf("zero-byte commit-only published transfer error_code = %q, want preserved payload evidence", published.ErrorCode)
	}
}

func TestRunMergesCommitRemoteFailureWithoutDroppingPriorPayloadEvidence(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceFile := filepath.Join(sourceRoot, "large.bin")
	if err := writePatternFile(sourceFile, 16*2); err != nil {
		t.Fatalf("writePatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	sessionID := "session-networkpush-commit-remote-failure"
	server, counts := newCountingTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)
	prof.PrivacyPolicy.BatchMaxCount = 1

	first, err := runWithClientDoer(context.Background(), prof, sessionID, func(base protocolclient.Doer) protocolclient.Doer {
		return &failBeforeCommitDoer{base: base}
	}, func() time.Time { return testNow }, 16)
	if err == nil {
		t.Fatalf("Run(initial pre-commit interruption) error = nil, want failure")
	}
	prior := readNetworkTransfer(t, targetRoot, sessionID)
	if first.TransferStatus != control.NetworkTransferFailed || prior.PrivacyOverhead == nil || prior.PrivacyOverhead.PaddedChunks == 0 {
		t.Fatalf("initial interrupted result=%+v prior=%+v, want failed prior payload evidence", first, prior)
	}
	counts.reset()

	conflict := &commitConflictDoer{}
	second, err := runWithClientDoer(context.Background(), prof, sessionID, func(base protocolclient.Doer) protocolclient.Doer {
		conflict.base = base
		return conflict
	}, func() time.Time { return testNow.Add(5 * time.Minute) }, 16)
	if err == nil {
		t.Fatalf("Run(commit remote failure retry) error = nil, want commit conflict")
	}
	if second.TransferStatus != control.NetworkTransferPublishFailed || second.TransferStage != "commit" || second.TransferCode != "publish_failed" || second.ResumeOutcome != "blocked" {
		t.Fatalf("Run(commit remote failure retry) result = %+v, want blocked publish_failed commit evidence", second)
	}
	merged := readNetworkTransfer(t, targetRoot, sessionID)
	if merged.Status != control.NetworkTransferPublishFailed || merged.PrivacyOverhead == nil ||
		merged.PrivacyOverhead.PaddedChunks != prior.PrivacyOverhead.PaddedChunks ||
		len(merged.Attempts) != len(prior.Attempts)+1 {
		t.Fatalf("commit remote failure transfer = status=%q overhead=%+v attempts=%d, want publish_failed preserving prior overhead %+v attempts %d", merged.Status, merged.PrivacyOverhead, len(merged.Attempts), prior.PrivacyOverhead, len(prior.Attempts))
	}
	snapshot := counts.snapshot()
	if snapshot.chunkRequests != 0 || conflict.commits.Load() != 1 {
		t.Fatalf("commit remote failure counts = %+v synthetic_commits=%d, want no chunks and one commit error", snapshot, conflict.commits.Load())
	}
}

func TestRunFailsClosedWhenPublishedRetryCannotReadPayloadOverhead(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.txt"), []byte("commit retry\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	sessionID := "session-networkpush-commit-retry"
	server, counter := newCountingTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)

	if _, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: sessionID,
		Now:       func() time.Time { return testNow },
	}); err != nil {
		t.Fatalf("Run(first publish) error = %v, want nil", err)
	}
	transferPath, err := control.Path(targetRoot, control.ArtifactNetworkTransfer, sessionID)
	if err != nil {
		t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
	}
	if err := os.Remove(transferPath); err != nil {
		t.Fatalf("os.Remove(%q) error = %v, want nil", transferPath, err)
	}
	counter.reset()
	got, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: sessionID,
		Now:       func() time.Time { return testNow.Add(5 * time.Minute) },
	})
	if !errors.Is(err, networkrun.ErrPayloadOverheadMissing) {
		t.Fatalf("Run(published retry missing artifact) error = %v, want ErrPayloadOverheadMissing", err)
	}
	if got.TransferStatus != control.NetworkTransferNeedsRepair || got.TransferStage != "network_transfer_artifact" || got.TransferCode != "payload_overhead_missing" || got.Chunks != 0 || got.Bytes != 0 {
		t.Fatalf("Run(published retry missing artifact) result = %+v, want needs_repair without chunk upload", got)
	}
	if got.ResumeAuthority != "receiver_status" || got.ResumeOutcome != "blocked" {
		t.Fatalf("Run(published retry missing artifact) resume = %s/%s, want receiver_status/blocked", got.ResumeAuthority, got.ResumeOutcome)
	}
	snapshot := counter.snapshot()
	if snapshot.chunkRequests != 0 {
		t.Fatalf("chunk uploads during published retry = %d, want 0", snapshot.chunkRequests)
	}
	if snapshot.commits != 0 {
		t.Fatalf("commit calls during published retry = %d, want blocked before idempotent commit retry", snapshot.commits)
	}
	transfer := readNetworkTransfer(t, targetRoot, sessionID)
	if transfer.Status != control.NetworkTransferNeedsRepair || transfer.Stage != "network_transfer_artifact" || transfer.ErrorCode != "payload_overhead_missing" || transfer.PrivacyOverhead != nil {
		t.Fatalf("missing-overhead transfer = %+v, want needs_repair without fabricated privacy overhead", transfer)
	}
}

func TestRunFailsClosedWhenPublishedRetryHasBadPriorPayloadEvidence(t *testing.T) {
	tests := []struct {
		name string
		edit func(control.NetworkTransfer) control.NetworkTransfer
	}{
		{
			name: "corrupt JSON",
			edit: func(doc control.NetworkTransfer) control.NetworkTransfer {
				return doc
			},
		},
		{
			name: "scope mismatch",
			edit: func(doc control.NetworkTransfer) control.NetworkTransfer {
				doc.ProfileID = "profile.other"
				return doc
			},
		},
		{
			name: "non-published prior",
			edit: func(doc control.NetworkTransfer) control.NetworkTransfer {
				doc.Status = control.NetworkTransferFailed
				doc.Stage = "transport"
				doc.ErrorCode = "transfer_failed"
				last := len(doc.Attempts) - 1
				doc.Attempts[last].Status = control.NetworkTransferFailed
				doc.Attempts[last].Stage = "transport"
				doc.Attempts[last].ErrorCode = "transfer_failed"
				return doc
			},
		},
		{
			name: "missing payload overhead",
			edit: func(doc control.NetworkTransfer) control.NetworkTransfer {
				doc.PrivacyOverhead = nil
				return doc
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceRoot := t.TempDir()
			targetRoot := t.TempDir()
			if err := os.WriteFile(filepath.Join(sourceRoot, "data.txt"), []byte("retry bad prior\n"), 0o600); err != nil {
				t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
			}
			sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
			targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
			peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
			prof := pairedProfile(t, sourceRoot, targetRoot, peer)
			sessionID := "session-networkpush-bad-prior-" + strings.ReplaceAll(tt.name, " ", "-")
			server, counts := newCountingTLSReceiverServer(t, prof, targetCert)
			defer server.Close()
			prof.Network = networkConfig(t, sourceCert, server.URL)

			if _, err := Run(context.Background(), Options{
				Profile:   prof,
				SessionID: sessionID,
				Now:       func() time.Time { return testNow },
			}); err != nil {
				t.Fatalf("Run(first publish) error = %v, want nil", err)
			}
			transferPath, err := control.Path(targetRoot, control.ArtifactNetworkTransfer, sessionID)
			if err != nil {
				t.Fatalf("control.Path(network transfer) error = %v, want nil", err)
			}
			if tt.name == "corrupt JSON" {
				if err := os.WriteFile(transferPath, []byte(`{"not valid"`), 0o600); err != nil {
					t.Fatalf("os.WriteFile(corrupt transfer) error = %v, want nil", err)
				}
			} else {
				doc := tt.edit(readNetworkTransfer(t, targetRoot, sessionID))
				writeRawControlJSON(t, transferPath, doc)
			}
			counts.reset()

			got, err := Run(context.Background(), Options{
				Profile:   prof,
				SessionID: sessionID,
				Now:       func() time.Time { return testNow.Add(5 * time.Minute) },
			})
			if !errors.Is(err, networkrun.ErrPayloadOverheadMissing) {
				t.Fatalf("Run(published retry bad prior) error = %v, want ErrPayloadOverheadMissing", err)
			}
			if got.TransferStatus != control.NetworkTransferNeedsRepair || got.TransferStage != "network_transfer_artifact" || got.TransferCode != "payload_overhead_missing" || got.ResumeOutcome != "blocked" {
				t.Fatalf("Run(published retry bad prior) result = %+v, want blocked needs_repair", got)
			}
			snapshot := counts.snapshot()
			wantCommits := int64(0)
			if tt.name == "non-published prior" {
				wantCommits = 1
			}
			if snapshot.chunkRequests != 0 || snapshot.commits != wantCommits {
				t.Fatalf("bad-prior retry counts = %+v, want no chunks and %d commits", snapshot, wantCommits)
			}
			transfer := readNetworkTransfer(t, targetRoot, sessionID)
			if transfer.Status != control.NetworkTransferNeedsRepair || transfer.PrivacyOverhead != nil {
				t.Fatalf("bad-prior retry transfer = %+v, want needs_repair without fabricated overhead", transfer)
			}
		})
	}
}

func TestRunReportsNeedsRepairFromReceiverStatus(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.bin"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	sessionID := "session-networkpush-needs-repair"
	begin := beginRequestForProfile(t, prof, sessionID)
	store := receiver.FileStore{TargetRoot: targetRoot, Now: func() time.Time { return testNow }}
	if _, err := store.Begin(begin); err != nil {
		t.Fatalf("FileStore.Begin(needs repair seed) error = %v, want nil", err)
	}
	prefix := []byte("pay")
	if _, err := store.AppendChunk(protocol.ChunkUploadRequest{
		SessionID: sessionID,
		Path:      "data.bin",
		Offset:    0,
		Data:      prefix,
		Digest:    digestBytes(prefix),
	}); err != nil {
		t.Fatalf("FileStore.AppendChunk(partial) error = %v, want nil", err)
	}
	if _, err := store.Commit(protocol.CommitSessionRequest{SessionID: sessionID, EndedAt: testNow.Add(time.Minute)}); err == nil {
		t.Fatalf("FileStore.Commit(incomplete) error = nil, want integrity failure")
	}

	server, counts := newCountingTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)
	got, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: sessionID,
		Now:       func() time.Time { return testNow },
	})
	if !errors.Is(err, protocolclient.ErrReceiverNeedsRepair) {
		t.Fatalf("Run(needs repair) error = %v, want ErrReceiverNeedsRepair", err)
	}
	if got.TransferStatus != control.NetworkTransferNeedsRepair || got.TransferStage != "status" || got.TransferCode != "receiver_needs_repair" {
		t.Fatalf("Run(needs repair) result = %+v, want needs_repair status evidence", got)
	}
	snapshot := counts.snapshot()
	if snapshot.chunkRequests != 0 || snapshot.commits != 0 {
		t.Fatalf("receiver calls after needs_repair = %+v, want no chunk upload or commit", snapshot)
	}
	transfer := readNetworkTransfer(t, targetRoot, sessionID)
	if transfer.Status != control.NetworkTransferNeedsRepair || transfer.Stage != "status" || transfer.ErrorCode != "receiver_needs_repair" {
		t.Fatalf("network transfer = %+v, want receiver needs_repair evidence", transfer)
	}
}

func TestRunRefusesMissingNetworkAndPairing(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))

	tests := []struct {
		name    string
		prof    profile.Profile
		want    string
		isTrust error
	}{
		{
			name: "missing network",
			prof: profile.NewDefault("profile.default", "Profile", sourceRoot, targetRoot),
			want: "network.receiver_url is required",
		},
		{
			name: "missing pairing",
			prof: func() profile.Profile {
				p := profile.NewDefault("profile.default", "Profile", sourceRoot, targetRoot)
				p.Network = networkConfig(t, sourceCert, "https://127.0.0.1:1")
				return p
			}(),
			want:    "profile is not paired",
			isTrust: pairing.ErrUnpairedProfile,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Run(context.Background(), Options{
				Profile:   tt.prof,
				SessionID: "session-refused",
				Now:       func() time.Time { return testNow },
			})
			if err == nil {
				t.Fatalf("Run(%s) error = nil, want refusal", tt.name)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Run(%s) error = %v, want %q", tt.name, err, tt.want)
			}
			if tt.isTrust != nil && !errors.Is(err, tt.isTrust) {
				t.Fatalf("Run(%s) error = %v, want errors.Is(%v)", tt.name, err, tt.isTrust)
			}
		})
	}
}

func TestRunRejectsUnsafeLocalTLSIdentityFiles(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	prof.Network = networkConfig(t, sourceCert, "https://127.0.0.1:1")

	if err := os.Chmod(prof.Network.LocalTLSIdentity.PrivateKeyPath, 0o640); err != nil {
		t.Fatalf("os.Chmod(private key) error = %v, want nil", err)
	}
	_, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: "session-unsafe-identity",
		Now:       func() time.Time { return testNow },
	})
	if err == nil || !strings.Contains(err.Error(), "must not be readable") {
		t.Fatalf("Run(unsafe identity) error = %v, want private key permission refusal", err)
	}
}

func TestRunRecordsAuthRefusedEvidenceWhenReceiverPinsDiffer(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.txt"), []byte("auth failure\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)

	server := newTLSReceiverServerWithPeer(t, targetRoot, targetCert, peer, transport.AuthenticatedPeer{
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: certDeviceID(t, newTestCertificate(t, "other-target", testNow.Add(-time.Hour), testNow.Add(time.Hour))),
	})
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)

	got, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: "session-auth-refused",
		Now:       func() time.Time { return testNow },
	})
	if err == nil {
		t.Fatalf("Run(auth refused) error = nil, want receiver auth refusal")
	}
	if got.TransferStatus != control.NetworkTransferAuthRefused || got.TransferStage != "begin" || got.TransferCode != "auth_refused" {
		t.Fatalf("Run(auth refused) result = %+v, want auth_refused begin evidence", got)
	}
	if _, readErr := readNetworkTransferFromPath(targetRoot, "session-auth-refused"); !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("network transfer read error = %v, want absent because receiver rejected before storing a session", readErr)
	}
	if _, statErr := os.Lstat(filepath.Join(targetRoot, "data.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("target file stat error = %v, want no mutation", statErr)
	}
}

func TestRunRejectsWrongServerTLSCertificateBeforeTargetMutation(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.txt"), []byte("wrong server cert\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	pinnedTargetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	servedTargetCert := newTestCertificate(t, "other-target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, pinnedTargetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	server := newTLSReceiverServerWithPeer(t, targetRoot, servedTargetCert, transport.AuthenticatedPeer{
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: certDeviceID(t, servedTargetCert),
	}, transport.AuthenticatedPeer{
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
	})
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)

	got, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: "session-wrong-server-cert",
		Now:       func() time.Time { return testNow },
	})
	if err == nil || !strings.Contains(err.Error(), "tls peer identity mismatch") {
		t.Fatalf("Run(wrong server cert) error = %v, want TLS peer mismatch", err)
	}
	if got.TransferStatus != control.NetworkTransferFailed || got.TransferStage != "transport" || got.TransferCode != "transfer_failed" {
		t.Fatalf("Run(wrong server cert) result = %+v, want failed transport classification", got)
	}
	if _, statErr := os.Lstat(filepath.Join(targetRoot, "data.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("target file stat error = %v, want no mutation", statErr)
	}
	if _, readErr := readNetworkTransferFromPath(targetRoot, "session-wrong-server-cert"); !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("network transfer read error = %v, want absent before receiver session exists", readErr)
	}
}

func TestRunPersistsReservedControlWarning(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.txt"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	reserved := filepath.Join(sourceRoot, control.DirName)
	if err := os.Mkdir(reserved, 0o755); err != nil {
		t.Fatalf("os.Mkdir(reserved) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(reserved, "source-control.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(reserved file) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	server := newTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)

	got, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: "session-reserved-warning",
		Now:       func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatalf("Run(reserved warning) error = %v, want nil", err)
	}
	if got.Warnings != 1 {
		t.Fatalf("Run(reserved warning).Warnings = %d, want 1", got.Warnings)
	}
	warningsDir := filepath.Join(control.ControlDir(targetRoot), "warnings")
	entries, err := os.ReadDir(warningsDir)
	if err != nil {
		t.Fatalf("os.ReadDir(warnings) error = %v, want nil", err)
	}
	if len(entries) != 1 {
		t.Fatalf("warning artifacts = %d, want 1", len(entries))
	}
	warning, err := control.ReadFile[control.Warning](filepath.Join(warningsDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("control.ReadFile(warning) error = %v, want nil", err)
	}
	if warning.Code != "reserved_control_plane_skipped" || warning.SessionID != "session-reserved-warning" {
		t.Fatalf("warning = %+v, want reserved control warning", warning)
	}
}

func TestRunPublishesZeroByteFileWithPrivacyOverheadEvidence(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "empty.txt"), nil, 0o600); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	server, counts := newCountingTLSReceiverServer(t, prof, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)
	sessionID := "session-zero-byte"

	got, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: sessionID,
		Now:       func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatalf("Run(zero-byte) error = %v, want nil", err)
	}
	if got.SessionID != sessionID || got.Files != 1 || got.Bytes != 0 || got.Chunks != 1 {
		t.Fatalf("Run(zero-byte) result = %+v, want one published zero-byte file", got)
	}
	if got.TransferStatus != control.NetworkTransferPublished || got.TransferStage != "commit" || got.TransferCode != "" || got.TransferError != "" {
		t.Fatalf("Run(zero-byte) transfer result = %+v, want published commit", got)
	}
	targetInfo, err := os.Lstat(filepath.Join(targetRoot, "empty.txt"))
	if err != nil {
		t.Fatalf("os.Lstat(target zero-byte file) error = %v, want nil", err)
	}
	if !targetInfo.Mode().IsRegular() || targetInfo.Size() != 0 {
		t.Fatalf("target zero-byte file mode/size = %s/%d, want regular size 0", targetInfo.Mode(), targetInfo.Size())
	}
	sessionDir := filepath.Join(control.ControlDir(targetRoot), "sessions", sessionID)
	for _, path := range []string{
		sessionDir,
		filepath.Join(sessionDir, "network-session.json"),
		mustControlPath(t, targetRoot, control.ArtifactManifest, sessionID),
		mustControlPath(t, targetRoot, control.ArtifactSessionReceipt, sessionID),
		mustControlPath(t, targetRoot, control.ArtifactNetworkTransfer, sessionID),
		mustControlPath(t, targetRoot, control.ArtifactProfileSnapshot, "profile-"+sessionID),
	} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("receiver artifact %q stat error = %v, want present", path, err)
		}
	}
	snapshot := counts.snapshot()
	if snapshot.begin != 1 || snapshot.chunkRequests != 1 || snapshot.commits != 1 {
		t.Fatalf("zero-byte receiver counts = %+v, want begin/chunk/commit calls", snapshot)
	}
	transferPath := mustControlPath(t, targetRoot, control.ArtifactNetworkTransfer, sessionID)
	transferRaw, err := os.ReadFile(transferPath)
	if err != nil {
		t.Fatalf("os.ReadFile(network transfer) error = %v, want nil", err)
	}
	if strings.Contains(string(transferRaw), "payload_overhead_missing") {
		t.Fatalf("network-transfer.json contains payload_overhead_missing, want published payload evidence")
	}
	transfer := readNetworkTransfer(t, targetRoot, sessionID)
	if transfer.Status != control.NetworkTransferPublished || transfer.Stage != "commit" || transfer.ErrorCode != "" {
		t.Fatalf("network transfer artifact = %+v, want published commit", transfer)
	}
	if transfer.PrivacyOverhead == nil ||
		transfer.PrivacyOverhead.PaddedChunks == 0 ||
		transfer.PrivacyOverhead.BatchFrames == 0 ||
		transfer.PrivacyOverhead.BatchedChunks == 0 {
		t.Fatalf("network transfer privacy overhead = %+v, want padded chunk and batch counters", transfer.PrivacyOverhead)
	}

	counts.reset()
	second, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: sessionID,
		Now:       func() time.Time { return testNow.Add(5 * time.Minute) },
	})
	if err != nil {
		t.Fatalf("Run(zero-byte published retry) error = %v, want nil", err)
	}
	if second.TransferStatus != control.NetworkTransferPublished || second.Bytes != 0 || second.Chunks != 0 {
		t.Fatalf("Run(zero-byte published retry) result = %+v, want published retry with no payload upload", second)
	}
	if second.ResumeAuthority != "receiver_status" || second.ResumeOutcome != "published_retry" {
		t.Fatalf("Run(zero-byte published retry) resume = %s/%s, want receiver_status/published_retry", second.ResumeAuthority, second.ResumeOutcome)
	}
	retryCounts := counts.snapshot()
	if retryCounts.chunkRequests != 0 || retryCounts.commits != 1 {
		t.Fatalf("zero-byte retry receiver counts = %+v, want no chunks and one idempotent commit", retryCounts)
	}
	afterRetry := readNetworkTransfer(t, targetRoot, sessionID)
	if afterRetry.PrivacyOverhead == nil ||
		afterRetry.PrivacyOverhead.PaddedChunks != transfer.PrivacyOverhead.PaddedChunks ||
		afterRetry.PrivacyOverhead.BatchFrames != transfer.PrivacyOverhead.BatchFrames ||
		afterRetry.PrivacyOverhead.BatchedChunks != transfer.PrivacyOverhead.BatchedChunks {
		t.Fatalf("network transfer overhead after retry = %+v, want preserved payload overhead %+v", afterRetry.PrivacyOverhead, transfer.PrivacyOverhead)
	}
	if afterRetry.ErrorCode == "payload_overhead_missing" {
		t.Fatalf("network transfer after retry error_code = %q, want preserved published evidence", afterRetry.ErrorCode)
	}
}

func TestRunDoesNotRequireTargetLocalPathForTransferEvidence(t *testing.T) {
	sourceRoot := t.TempDir()
	receiptRoot := t.TempDir()
	remoteTargetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.txt"), []byte("remote evidence\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	serverProfile := pairedProfile(t, sourceRoot, remoteTargetRoot, peer)
	prof := pairedProfile(t, sourceRoot, receiptRoot, peer)
	beforeReceiptTree := snapshotTree(t, control.ControlDir(receiptRoot))
	server := newTLSReceiverServer(t, serverProfile, targetCert)
	defer server.Close()
	prof.Network = networkConfig(t, sourceCert, server.URL)
	if err := os.Chmod(receiptRoot, 0o500); err != nil {
		t.Fatalf("os.Chmod(receipt root) error = %v, want nil", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(receiptRoot, 0o700)
	})

	got, err := Run(context.Background(), Options{
		Profile:   prof,
		SessionID: "session-no-local-target",
		Now:       func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatalf("Run(no local target) error = %v, want nil because evidence goes through receiver", err)
	}
	if got.TransferStatus != control.NetworkTransferPublished {
		t.Fatalf("Run(no local target) result = %+v, want published", got)
	}
	if _, err := readNetworkTransferFromPath(receiptRoot, "session-no-local-target"); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source-side receipt root network transfer error = %v, want absent", err)
	}
	afterReceiptTree := snapshotTree(t, control.ControlDir(receiptRoot))
	if strings.Join(afterReceiptTree, "\n") != strings.Join(beforeReceiptTree, "\n") {
		t.Fatalf("source-side receipt root mutated\nbefore:\n%s\nafter:\n%s", strings.Join(beforeReceiptTree, "\n"), strings.Join(afterReceiptTree, "\n"))
	}
	transfer := readNetworkTransfer(t, remoteTargetRoot, "session-no-local-target")
	if transfer.Status != control.NetworkTransferPublished {
		t.Fatalf("remote network transfer = %+v, want published", transfer)
	}
}

func TestPreflightRejectsNonLevel2TrafficPolicy(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.txt"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	prof.Network = networkConfig(t, sourceCert, "https://127.0.0.1:1")
	prof.PrivacyPolicy.TrafficLevel = int(transport.PrivacyLevel1)

	_, err := Preflight(context.Background(), Options{
		Profile:   prof,
		SessionID: "session-level-one",
		Now:       func() time.Time { return testNow },
	})
	if err == nil || !strings.Contains(err.Error(), "only traffic level 2 is supported") {
		t.Fatalf("Preflight(level 1) error = %v, want traffic level refusal", err)
	}
	if _, statErr := os.Lstat(filepath.Join(control.ControlDir(targetRoot), "sessions", "session-level-one")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("level 1 session artifact stat error = %v, want no mutation", statErr)
	}
}

func newTLSReceiverServer(t *testing.T, prof profile.Profile, targetCert tls.Certificate) *httptest.Server {
	t.Helper()
	server, _ := newCountingTLSReceiverServer(t, prof, targetCert)
	return server
}

type receiverCounts struct {
	begin         atomic.Int64
	status        atomic.Int64
	chunkRequests atomic.Int64
	commits       atomic.Int64
}

type receiverCountsSnapshot struct {
	begin         int64
	status        int64
	chunkRequests int64
	commits       int64
}

func (c *receiverCounts) reset() {
	c.begin.Store(0)
	c.status.Store(0)
	c.chunkRequests.Store(0)
	c.commits.Store(0)
}

func (c *receiverCounts) snapshot() receiverCountsSnapshot {
	return receiverCountsSnapshot{
		begin:         c.begin.Load(),
		status:        c.status.Load(),
		chunkRequests: c.chunkRequests.Load(),
		commits:       c.commits.Load(),
	}
}

func newCountingTLSReceiverServer(t *testing.T, prof profile.Profile, targetCert tls.Certificate) (*httptest.Server, *receiverCounts) {
	t.Helper()
	receiverTLS, err := receiver.NewTLSReceiverFromProfile(receiver.TLSReceiverOptions{
		Profile:      prof,
		Certificates: []tls.Certificate{targetCert},
		Now:          func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatalf("NewTLSReceiverFromProfile error = %v, want nil", err)
	}
	counts := &receiverCounts{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
			counts.begin.Add(1)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/sessions/") && strings.HasSuffix(r.URL.Path, "/status"):
			counts.status.Add(1)
		case r.Method == http.MethodPost && (r.URL.Path == "/v1/chunks" || r.URL.Path == "/v1/chunk-batches"):
			counts.chunkRequests.Add(1)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/commit":
			counts.commits.Add(1)
		}
		receiverTLS.Handler.ServeHTTP(w, r)
	})
	server := httptest.NewUnstartedServer(handler)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = receiverTLS.TLSConfig
	server.StartTLS()
	return server, counts
}

func runWithClientDoer(ctx context.Context, prof profile.Profile, sessionID string, wrap func(protocolclient.Doer) protocolclient.Doer, now func() time.Time, chunkSize int) (Result, error) {
	prepared, err := prepare(ctx, Options{Profile: prof, SessionID: sessionID, Now: now}, true)
	if err != nil {
		return Result{}, err
	}
	certificate, err := tlsidentity.Load(prof.Network.LocalTLSIdentity)
	if err != nil {
		return Result{}, fmt.Errorf("load local TLS identity: %w", err)
	}
	tlsConfig, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{certificate},
		SourceDeviceID: prepared.trust.Receipt.SourceDeviceID,
		TargetDeviceID: prepared.trust.TargetDeviceID,
		Time:           now,
	})
	if err != nil {
		return Result{}, fmt.Errorf("build client TLS config: %w", err)
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport}
	doer := protocolclient.Doer(httpClient)
	if wrap != nil {
		doer = wrap(doer)
	}
	runResult, err := networkrun.Run(ctx, networkrun.Options{
		ArtifactWriter: networkrun.HTTPArtifactWriter{
			BaseURL: prof.Network.ReceiverURL,
			Doer:    doer,
		},
		ProfileSnapshot:      &prepared.profile,
		ProfilePrivacyPolicy: prof.PrivacyPolicy,
		Request:              prepared.request,
		Client: protocolclient.Client{
			BaseURL:   prof.Network.ReceiverURL,
			Doer:      doer,
			Now:       now,
			ChunkSize: chunkSize,
		},
		Now: now,
	})
	return resultFromNetworkRun(prepared.request.SessionID, runResult), err
}

type failAfterAcceptedChunkDoer struct {
	base      protocolclient.Doer
	failAfter int
	chunks    atomic.Int64
}

func (d *failAfterAcceptedChunkDoer) Do(req *http.Request) (*http.Response, error) {
	resp, err := d.base.Do(req)
	if err != nil {
		return resp, err
	}
	if req.Method == http.MethodPost && (req.URL.Path == "/v1/chunks" || req.URL.Path == "/v1/chunk-batches") && d.chunks.Add(1) >= int64(d.failAfter) {
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		return nil, errors.New("simulated interruption after accepted chunk")
	}
	return resp, nil
}

type failBeforeCommitDoer struct {
	base protocolclient.Doer
}

func (d *failBeforeCommitDoer) Do(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodPost && req.URL.Path == "/v1/commit" {
		return nil, errors.New("simulated interruption before commit")
	}
	resp, err := d.base.Do(req)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

type commitConflictDoer struct {
	base    protocolclient.Doer
	commits atomic.Int64
}

func (d *commitConflictDoer) Do(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodPost && req.URL.Path == "/v1/commit" {
		d.commits.Add(1)
		body := strings.NewReader(`{"code":"conflict","message":"simulated commit conflict"}`)
		return &http.Response{
			StatusCode: http.StatusConflict,
			Status:     "409 Conflict",
			Header:     make(http.Header),
			Body:       io.NopCloser(body),
			Request:    req,
		}, nil
	}
	return d.base.Do(req)
}

func newTLSReceiverServerWithPeer(
	t *testing.T,
	targetRoot string,
	targetCert tls.Certificate,
	tlsPeer transport.AuthenticatedPeer,
	handlerPeer transport.AuthenticatedPeer,
) *httptest.Server {
	t.Helper()
	handler := receiver.NewAuthenticatedHandler(receiver.FileStore{TargetRoot: targetRoot}, receiver.AuthenticatedHandlerOptions{
		ProfileID:      handlerPeer.ProfileID,
		TargetID:       handlerPeer.TargetID,
		SourceDeviceID: handlerPeer.SourceDeviceID,
		TargetDeviceID: handlerPeer.TargetDeviceID,
	})
	serverTLS, err := transport.ServerTLSConfig(transport.ServerTLSOptions{
		Certificates: []tls.Certificate{targetCert},
		Peer:         tlsPeer,
		Time:         func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatalf("ServerTLSConfig error = %v, want nil", err)
	}
	wrapped, err := transport.NewTLSAuthenticatedPeerHandler(handler, transport.AuthenticatedPeerTLSOptions{
		Peer: tlsPeer,
		Time: func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatalf("NewTLSAuthenticatedPeerHandler error = %v, want nil", err)
	}
	server := httptest.NewUnstartedServer(wrapped)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = serverTLS
	server.StartTLS()
	return server
}

func networkConfig(t *testing.T, sourceCert tls.Certificate, receiverURL string) *profile.NetworkConfig {
	t.Helper()
	certPath, keyPath := writeTLSIdentity(t, sourceCert)
	return &profile.NetworkConfig{
		ReceiverURL: receiverURL,
		LocalTLSIdentity: profile.TLSIdentityRef{
			CertificatePath: certPath,
			PrivateKeyPath:  keyPath,
		},
	}
}

func writeTLSIdentity(t *testing.T, cert tls.Certificate) (string, string) {
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
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatalf("os.WriteFile(cert) error = %v, want nil", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("os.WriteFile(key) error = %v, want nil", err)
	}
	return certPath, keyPath
}

func pairedProfile(t *testing.T, sourceRoot, targetRoot string, peer transport.AuthenticatedPeer) profile.Profile {
	t.Helper()
	p := profile.NewDefault(peer.ProfileID, "Profile", sourceRoot, targetRoot)
	p.Target.TargetID = peer.TargetID
	p.Target.DevicePublicKey = peer.TargetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = testNow.Format(time.RFC3339)
	receipt := control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               p.Target.PairingReceiptID,
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   peer.SourceDeviceID,
		TargetDeviceID:   peer.TargetDeviceID,
		DevicePublicKey:  peer.TargetDeviceID,
		Method:           string(transport.PairingMethodSAS),
		VerifiedAt:       p.Target.PairedAt,
		VerificationHash: "sha256:abcdef0123456789",
		ProtocolVersion:  protocol.Version,
	}
	receiptPath, err := control.Path(targetRoot, control.ArtifactPairingReceipt, receipt.ID)
	if err != nil {
		t.Fatalf("control.Path(pairing receipt) error = %v, want nil", err)
	}
	if err := control.WriteFile(receiptPath, receipt); err != nil {
		t.Fatalf("control.WriteFile(pairing receipt) error = %v, want nil", err)
	}
	return p
}

func authenticatedPeerForCerts(t *testing.T, sourceCert, targetCert tls.Certificate) transport.AuthenticatedPeer {
	t.Helper()
	return transport.AuthenticatedPeer{
		ProfileID:      "profile.default",
		TargetID:       "local:profile.default",
		SourceDeviceID: certDeviceID(t, sourceCert),
		TargetDeviceID: certDeviceID(t, targetCert),
	}
}

func certDeviceID(t *testing.T, cert tls.Certificate) string {
	t.Helper()
	id, err := transport.LeafSPKIDeviceID(cert.Leaf)
	if err != nil {
		t.Fatalf("LeafSPKIDeviceID(%q) error = %v, want nil", cert.Leaf.Subject.CommonName, err)
	}
	return id
}

func newTestCertificate(t *testing.T, commonName string, notBefore, notAfter time.Time) tls.Certificate {
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

func readNetworkTransfer(t *testing.T, targetRoot, sessionID string) control.NetworkTransfer {
	t.Helper()
	doc, err := readNetworkTransferFromPath(targetRoot, sessionID)
	if err != nil {
		t.Fatalf("control.ReadFile(network transfer) error = %v, want nil", err)
	}
	return doc
}

func readNetworkTransferFromPath(targetRoot, sessionID string) (control.NetworkTransfer, error) {
	path, err := control.Path(targetRoot, control.ArtifactNetworkTransfer, sessionID)
	if err != nil {
		return control.NetworkTransfer{}, err
	}
	doc, err := control.ReadFile[control.NetworkTransfer](path)
	if err != nil {
		return control.NetworkTransfer{}, err
	}
	return doc, nil
}

func mustControlPath(t *testing.T, targetRoot string, artifact control.ArtifactType, id string) string {
	t.Helper()
	path, err := control.Path(targetRoot, artifact, id)
	if err != nil {
		t.Fatalf("control.Path(%q, %q) error = %v, want nil", artifact, id, err)
	}
	return path
}

func writeRawControlJSON(t *testing.T, path string, doc control.Document) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatalf("os.OpenFile(%q) error = %v, want nil", path, err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(doc)
	closeErr := file.Close()
	if encodeErr != nil {
		t.Fatalf("json.Encode(%q) error = %v, want nil", path, encodeErr)
	}
	if closeErr != nil {
		t.Fatalf("close raw control JSON %q error = %v, want nil", path, closeErr)
	}
}

func marshalNetworkPushControlDoc(t *testing.T, doc control.Document) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := control.Write(&buf, doc); err != nil {
		t.Fatalf("control.Write(%T) error = %v, want nil", doc, err)
	}
	return buf.Bytes()
}

func networkTransferForBegin(begin protocol.BeginSessionRequest, status control.NetworkTransferStatus) control.NetworkTransfer {
	doc := control.NetworkTransfer{
		Version:         control.CurrentVersion,
		SessionID:       begin.SessionID,
		ProfileID:       begin.ProfileID,
		TargetID:        begin.TargetID,
		SourceDeviceID:  begin.SourceDeviceID,
		TargetDeviceID:  begin.TargetDeviceID,
		ProtocolVersion: protocol.Version,
		PrivacyPolicy:   begin.PrivacyPolicy,
		Status:          status,
		Stage:           "transport",
		ErrorCode:       "transfer_failed",
		StartedAt:       testNow.Format(time.RFC3339Nano),
		UpdatedAt:       testNow.Add(time.Minute).Format(time.RFC3339Nano),
		PrivacyOverhead: &control.NetworkTransferPrivacyOverhead{
			FramePlainBytes:    128,
			FrameWireBytes:     192,
			PaddingBytes:       64,
			PaddedChunks:       1,
			PaddingBucketBytes: begin.PrivacyPolicy.PaddingBucket,
			BatchFrames:        1,
			BatchedChunks:      1,
			MaxBatchCount:      begin.PrivacyPolicy.BatchMaxCount,
			MaxBatchPlainBytes: 128,
			JitteredRequests:   1,
			JitterBudgetMillis: begin.PrivacyPolicy.JitterBudget,
		},
		Attempts: []control.NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: testNow.Format(time.RFC3339Nano),
			EndedAt:   testNow.Add(time.Minute).Format(time.RFC3339Nano),
			Stage:     "transport",
			Status:    status,
			ErrorCode: "transfer_failed",
		}},
	}
	if status == control.NetworkTransferPublished {
		doc.Stage = "commit"
		doc.ErrorCode = ""
		doc.Attempts[0].Stage = "commit"
		doc.Attempts[0].ErrorCode = ""
	}
	return doc
}

func readControlDoc[T control.Document](t *testing.T, targetRoot string, artifact control.ArtifactType, id string) T {
	t.Helper()
	path, err := control.Path(targetRoot, artifact, id)
	if err != nil {
		t.Fatalf("control.Path(%q, %q) error = %v, want nil", artifact, id, err)
	}
	doc, err := control.ReadFile[T](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	return doc
}

func beginRequestForProfile(t *testing.T, prof profile.Profile, sessionID string) protocol.BeginSessionRequest {
	t.Helper()
	scanned, err := scan.Scan(prof.Roots[0].Path)
	if err != nil {
		t.Fatalf("scan.Scan(%q) error = %v, want nil", prof.Roots[0].Path, err)
	}
	trust, err := pairing.ValidateProfileTrust(prof)
	if err != nil {
		t.Fatalf("pairing.ValidateProfileTrust error = %v, want nil", err)
	}
	privacyPolicy, err := transportPrivacyPolicyFromProfile(prof.PrivacyPolicy)
	if err != nil {
		t.Fatalf("transportPrivacyPolicyFromProfile error = %v, want nil", err)
	}
	begin, _, err := protocolclient.BuildBeginRequest(protocolclient.TransferRequest{
		SourceRoot:     prof.Roots[0].Path,
		Scan:           scanned,
		SessionID:      sessionID,
		ManifestID:     sessionID,
		ProfileID:      prof.ProfileID,
		TargetID:       prof.Target.TargetID,
		SourceDeviceID: trust.Receipt.SourceDeviceID,
		TargetDeviceID: trust.TargetDeviceID,
		PrivacyPolicy:  privacyPolicy,
		RootID:         prof.Roots[0].ID,
		CreatedAt:      testNow,
	})
	if err != nil {
		t.Fatalf("protocolclient.BuildBeginRequest error = %v, want nil", err)
	}
	return begin
}

func readFilePrefix(t *testing.T, path string, size int) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	if len(data) < size {
		t.Fatalf("file %q size = %d, want at least %d", path, len(data), size)
	}
	return append([]byte(nil), data[:size]...)
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + fmt.Sprintf("%x", sum[:])
}

func writePatternFile(path string, size int) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	pattern := []byte("supermover-networkpush-recovery-test-pattern\n")
	written := 0
	for written < size {
		n := min(len(pattern), size-written)
		if _, err := file.Write(pattern[:n]); err != nil {
			return err
		}
		written += n
	}
	return nil
}

func hashFile(t *testing.T, path string) string {
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
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

func snapshotTree(t *testing.T, root string) []string {
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
