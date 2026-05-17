package transport_test

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
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/networkrun"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/protocolclient"
	"github.com/khicago/supermover/internal/receiver"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/transport"
)

var tlsTestNow = time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)

func TestLeafSPKIDeviceIDDerivesStablePinnedIdentity(t *testing.T) {
	cert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	other := newTestCertificate(t, "source-rotated", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))

	id, err := transport.LeafSPKIDeviceID(cert.Leaf)
	if err != nil {
		t.Fatalf("LeafSPKIDeviceID(source) error = %v, want nil", err)
	}
	again, err := transport.LeafSPKIDeviceID(cert.Leaf)
	if err != nil {
		t.Fatalf("LeafSPKIDeviceID(source again) error = %v, want nil", err)
	}
	otherID, err := transport.LeafSPKIDeviceID(other.Leaf)
	if err != nil {
		t.Fatalf("LeafSPKIDeviceID(other) error = %v, want nil", err)
	}

	if id != again {
		t.Fatalf("LeafSPKIDeviceID not stable: %q then %q", id, again)
	}
	if id == otherID {
		t.Fatalf("LeafSPKIDeviceID collision for different test certs: %q", id)
	}
	if err := transport.DeviceID(id).Validate(); err != nil {
		t.Fatalf("derived device id %q validation error = %v, want nil", id, err)
	}
	if len(id) != len("sha256:")+sha256.Size*2 {
		t.Fatalf("derived device id len = %d, want %d", len(id), len("sha256:")+sha256.Size*2)
	}
}

func TestPinnedTLSReceiverAllowsMatchingMutualTLSProtocolClient(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceRoot, "docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll(source docs) error = %v, want nil", err)
	}
	const payload = "hello over pinned tls\n"
	if err := os.WriteFile(filepath.Join(sourceRoot, "docs", "a.txt"), []byte(payload), 0o600); err != nil {
		t.Fatalf("WriteFile(source file) error = %v, want nil", err)
	}
	scanned, err := scan.Scan(sourceRoot)
	if err != nil {
		t.Fatalf("scan.Scan(sourceRoot) error = %v, want nil", err)
	}

	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	handler := receiver.NewAuthenticatedHandler(receiver.FileStore{TargetRoot: targetRoot}, receiver.AuthenticatedHandlerOptions{
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
	})
	server, client := newPinnedTLSServerAndClient(t, handler, peer, sourceCert, targetCert)
	defer server.Close()

	result, err := protocolclient.Client{
		BaseURL:   server.URL,
		Doer:      client,
		ChunkSize: 5,
		Now:       func() time.Time { return tlsTestNow },
	}.Run(context.Background(), protocolclient.TransferRequest{
		SourceRoot:     sourceRoot,
		Scan:           scanned,
		SessionID:      "session-tls-ok",
		ManifestID:     "manifest-tls-ok",
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
		PrivacyPolicy:  fastJitterPrivacyPolicy(),
		RootID:         "root1",
		CreatedAt:      tlsTestNow,
		EndedAt:        tlsTestNow.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("protocolclient.Run over pinned TLS error = %v, want nil", err)
	}
	if result.Commit.State != protocol.SessionStatePublished {
		t.Fatalf("commit state = %q, want %q", result.Commit.State, protocol.SessionStatePublished)
	}
	got, err := os.ReadFile(filepath.Join(targetRoot, "docs", "a.txt"))
	if err != nil {
		t.Fatalf("ReadFile(target file) error = %v, want nil", err)
	}
	if string(got) != payload {
		t.Fatalf("target payload = %q, want %q", string(got), payload)
	}
}

func TestTLSReceiverFromProfileBindsPairingReceiptAndCertificatePins(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "a.txt"), []byte("profile tls\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(source file) error = %v, want nil", err)
	}
	scanned, err := scan.Scan(sourceRoot)
	if err != nil {
		t.Fatalf("scan.Scan(sourceRoot) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)

	receiverTLS, err := receiver.NewTLSReceiverFromProfile(receiver.TLSReceiverOptions{
		Profile:      prof,
		Certificates: []tls.Certificate{targetCert},
		Now:          func() time.Time { return tlsTestNow },
	})
	if err != nil {
		t.Fatalf("NewTLSReceiverFromProfile error = %v, want nil", err)
	}
	if receiverTLS.Peer != peer {
		t.Fatalf("receiver peer = %+v, want %+v", receiverTLS.Peer, peer)
	}
	server := httptest.NewUnstartedServer(receiverTLS.Handler)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = receiverTLS.TLSConfig
	server.StartTLS()
	defer server.Close()
	clientTLS, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{sourceCert},
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
		Time:           func() time.Time { return tlsTestNow },
	})
	if err != nil {
		t.Fatalf("ClientTLSConfig error = %v, want nil", err)
	}

	_, err = protocolclient.Client{
		BaseURL:   server.URL,
		Doer:      &http.Client{Transport: &http.Transport{TLSClientConfig: clientTLS}},
		ChunkSize: 4,
		Now:       func() time.Time { return tlsTestNow },
	}.Run(context.Background(), protocolclient.TransferRequest{
		SourceRoot:     sourceRoot,
		Scan:           scanned,
		SessionID:      "session-profile-tls",
		ManifestID:     "manifest-profile-tls",
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
		PrivacyPolicy:  fastJitterPrivacyPolicy(),
		RootID:         "root1",
		CreatedAt:      tlsTestNow,
		EndedAt:        tlsTestNow.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("protocolclient.Run through profile-bound TLS receiver error = %v, want nil", err)
	}
}

func TestTLSReceiverFromProfileRejectsStalePairingReceipt(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	prof.Target.PairedAt = tlsTestNow.Add(-time.Hour).Format(time.RFC3339)

	_, err := receiver.NewTLSReceiverFromProfile(receiver.TLSReceiverOptions{
		Profile:      prof,
		Certificates: []tls.Certificate{targetCert},
		Now:          func() time.Time { return tlsTestNow },
	})
	if err == nil {
		t.Fatalf("NewTLSReceiverFromProfile(stale receipt) error = nil, want failure")
	}
}

func TestTLSReceiverFromProfileRejectsWrongTargetCertificate(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	wrongTargetCert := newTestCertificate(t, "target-wrong", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)

	_, err := receiver.NewTLSReceiverFromProfile(receiver.TLSReceiverOptions{
		Profile:      prof,
		Certificates: []tls.Certificate{wrongTargetCert},
		Now:          func() time.Time { return tlsTestNow },
	})
	if err == nil {
		t.Fatalf("NewTLSReceiverFromProfile(wrong target certificate) error = nil, want failure")
	}
	if !errors.Is(err, transport.ErrTLSPeerMismatch) {
		t.Fatalf("NewTLSReceiverFromProfile(wrong target certificate) error = %v, want ErrTLSPeerMismatch", err)
	}
}

func TestPinnedTLSNetworkRunRecordsAuthRefusedForWrongRequestIdentity(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.txt"), []byte("auth refused\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(source file) error = %v, want nil", err)
	}
	scanned, err := scan.Scan(sourceRoot)
	if err != nil {
		t.Fatalf("scan.Scan(sourceRoot) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	wrongSourceCert := newTestCertificate(t, "source-wrong", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	handler := receiver.NewAuthenticatedHandler(receiver.FileStore{TargetRoot: targetRoot}, receiver.AuthenticatedHandlerOptions{
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
	})
	server, client := newPinnedTLSServerAndClient(t, handler, peer, sourceCert, targetCert)
	defer server.Close()

	sessionID := "session-tls-auth-refused"
	req := validTLSNetworkTransferRequest(sourceRoot, scanned, sessionID, peer)
	req.SourceDeviceID = certDeviceID(t, wrongSourceCert)
	_, err = networkrun.Run(context.Background(), networkrun.Options{
		TargetRoot:           targetRoot,
		ProfilePrivacyPolicy: profilePrivacyPolicyFromTransport(req.PrivacyPolicy),
		Request:              req,
		Client:               fastJitterProtocolClient(server.URL, 4, client),
		Now:                  func() time.Time { return tlsTestNow },
	})
	if err == nil {
		t.Fatalf("networkrun.Run(wrong request identity) error = nil, want auth refusal")
	}
	var remote *protocolclient.RemoteError
	if !errors.As(err, &remote) {
		t.Fatalf("networkrun.Run(wrong request identity) error = %T %v, want RemoteError", err, err)
	}
	if remote.Path != "/v1/sessions" || remote.StatusCode != http.StatusForbidden {
		t.Fatalf("RemoteError = %+v, want /v1/sessions 403", remote)
	}
	transfer := readTLSNetworkTransfer(t, targetRoot, sessionID)
	assertTLSNetworkTransferState(t, transfer, control.NetworkTransferAuthRefused, "begin", "auth_refused")
	assertNoPublishedFile(t, targetRoot, "data.txt")
}

func TestPinnedTLSNetworkRunResumesAfterInterruptedTransferAndReceiverRestart(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	chunkSize := 1024
	sourceFile := filepath.Join(sourceRoot, "large.bin")
	if err := writeTLSPatternFile(sourceFile, chunkSize*9+137); err != nil {
		t.Fatalf("writeTLSPatternFile(%q) error = %v, want nil", sourceFile, err)
	}
	scanned, err := scan.Scan(sourceRoot)
	if err != nil {
		t.Fatalf("scan.Scan(sourceRoot) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	sessionID := "session-tls-resume-restart"
	req := validTLSNetworkTransferRequest(sourceRoot, scanned, sessionID, peer)
	req.PrivacyPolicy.BatchMaxCount = 1

	firstServer, firstClient := newPinnedTLSReceiverServerAndClient(t, targetRoot, peer, sourceCert, targetCert)
	firstDoer := &failAfterAcceptedTLSChunkDoer{
		base:      firstClient,
		failAfter: 2,
	}
	_, err = networkrun.Run(context.Background(), networkrun.Options{
		TargetRoot:           targetRoot,
		ProfilePrivacyPolicy: profilePrivacyPolicyFromTransport(req.PrivacyPolicy),
		Request:              req,
		Client:               fastJitterProtocolClient(firstServer.URL, chunkSize, firstDoer),
		Now:                  func() time.Time { return tlsTestNow },
	})
	firstServer.Close()
	if err == nil || !strings.Contains(err.Error(), "simulated interruption after accepted TLS chunk") {
		t.Fatalf("first networkrun.Run(interrupted) error = %v, want simulated interruption", err)
	}
	if firstDoer.chunks != 2 {
		t.Fatalf("accepted chunks before interruption = %d, want 2", firstDoer.chunks)
	}
	interrupted := readTLSNetworkTransfer(t, targetRoot, sessionID)
	assertTLSNetworkTransferState(t, interrupted, control.NetworkTransferFailed, "transport", "transfer_failed")

	secondServer, secondClient := newPinnedTLSReceiverServerAndClient(t, targetRoot, peer, sourceCert, targetCert)
	defer secondServer.Close()
	got, err := networkrun.Run(context.Background(), networkrun.Options{
		TargetRoot:           targetRoot,
		ProfilePrivacyPolicy: profilePrivacyPolicyFromTransport(req.PrivacyPolicy),
		Request:              req,
		Client:               fastJitterProtocolClient(secondServer.URL, chunkSize, secondClient),
		Now:                  func() time.Time { return tlsTestNow },
	})
	if err != nil {
		t.Fatalf("second networkrun.Run(resume after restart) error = %v, want nil", err)
	}
	wantRemaining := int64(chunkSize*7 + 137)
	if got.ClientResult.Bytes != wantRemaining {
		t.Fatalf("second networkrun.Run(resume).Bytes = %d, want remaining bytes %d", got.ClientResult.Bytes, wantRemaining)
	}
	if got.ClientResult.Chunks != 8 {
		t.Fatalf("second networkrun.Run(resume).Chunks = %d, want 8 remaining chunks", got.ClientResult.Chunks)
	}
	transfer := readTLSNetworkTransfer(t, targetRoot, sessionID)
	assertTLSNetworkTransferState(t, transfer, control.NetworkTransferPublished, "commit", "")
	if len(transfer.Attempts) != 2 {
		t.Fatalf("network transfer attempts = %d, want interrupted attempt plus resumed publish attempt", len(transfer.Attempts))
	}
	if transfer.PrivacyOverhead == nil || interrupted.PrivacyOverhead == nil ||
		transfer.PrivacyOverhead.PaddedChunks <= interrupted.PrivacyOverhead.PaddedChunks ||
		transfer.PrivacyOverhead.BatchFrames <= interrupted.PrivacyOverhead.BatchFrames {
		t.Fatalf("published network transfer overhead = %+v, want merged with interrupted overhead %+v", transfer.PrivacyOverhead, interrupted.PrivacyOverhead)
	}
	if hashTLSFile(t, filepath.Join(targetRoot, "large.bin")) != hashTLSFile(t, sourceFile) {
		t.Fatalf("published large file digest differs from source")
	}
}

func TestPinnedTLSNetworkRunReportsNeedsRepairForCorruptedReceiverArtifact(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.bin"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("WriteFile(source file) error = %v, want nil", err)
	}
	scanned, err := scan.Scan(sourceRoot)
	if err != nil {
		t.Fatalf("scan.Scan(sourceRoot) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	sessionID := "session-tls-corrupt-artifact"
	req := validTLSNetworkTransferRequest(sourceRoot, scanned, sessionID, peer)
	store := receiver.FileStore{TargetRoot: targetRoot}
	begin, _, err := protocolclient.BuildBeginRequest(req)
	if err != nil {
		t.Fatalf("BuildBeginRequest(%+v) error = %v, want nil", req, err)
	}
	if _, err := store.Begin(begin); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", begin, err)
	}
	if _, err := store.AppendChunk(protocol.ChunkUploadRequest{
		SessionID: sessionID,
		Path:      "data.bin",
		Offset:    0,
		Data:      []byte("pay"),
		Digest:    digest([]byte("pay")),
	}); err != nil {
		t.Fatalf("FileStore.AppendChunk(partial) error = %v, want nil", err)
	}
	if _, err := store.Commit(protocol.CommitSessionRequest{SessionID: sessionID, EndedAt: tlsTestNow.Add(time.Minute)}); err == nil {
		t.Fatalf("FileStore.Commit(incomplete) error = nil, want integrity failure")
	}

	server, client := newPinnedTLSReceiverServerAndClient(t, targetRoot, peer, sourceCert, targetCert)
	defer server.Close()
	_, err = networkrun.Run(context.Background(), networkrun.Options{
		TargetRoot:           targetRoot,
		ProfilePrivacyPolicy: profilePrivacyPolicyFromTransport(req.PrivacyPolicy),
		Request:              req,
		Client:               fastJitterProtocolClient(server.URL, 3, client),
		Now:                  func() time.Time { return tlsTestNow },
	})
	if !errors.Is(err, protocolclient.ErrReceiverNeedsRepair) {
		t.Fatalf("networkrun.Run(needs repair receiver artifact) error = %v, want ErrReceiverNeedsRepair", err)
	}
	transfer := readTLSNetworkTransfer(t, targetRoot, sessionID)
	assertTLSNetworkTransferState(t, transfer, control.NetworkTransferNeedsRepair, "status", "receiver_needs_repair")
}

func TestPinnedTLSNetworkRunRecordsFailedEvidenceForCorruptedReceiverMetadata(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceRoot, "data.bin"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("WriteFile(source file) error = %v, want nil", err)
	}
	scanned, err := scan.Scan(sourceRoot)
	if err != nil {
		t.Fatalf("scan.Scan(sourceRoot) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	sessionID := "session-tls-corrupt-metadata"
	req := validTLSNetworkTransferRequest(sourceRoot, scanned, sessionID, peer)
	store := receiver.FileStore{TargetRoot: targetRoot}
	begin, _, err := protocolclient.BuildBeginRequest(req)
	if err != nil {
		t.Fatalf("BuildBeginRequest(%+v) error = %v, want nil", req, err)
	}
	if _, err := store.Begin(begin); err != nil {
		t.Fatalf("FileStore.Begin(%+v) error = %v, want nil", begin, err)
	}
	metaPath := filepath.Join(control.ControlDir(targetRoot), "sessions", sessionID, "network-session.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", metaPath, err)
	}
	corrupted := strings.Replace(string(data), "{", `{"unexpected":true,`, 1)
	if err := os.WriteFile(metaPath, []byte(corrupted), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", metaPath, err)
	}

	server, client := newPinnedTLSReceiverServerAndClient(t, targetRoot, peer, sourceCert, targetCert)
	defer server.Close()
	_, err = networkrun.Run(context.Background(), networkrun.Options{
		TargetRoot:           targetRoot,
		ProfilePrivacyPolicy: profilePrivacyPolicyFromTransport(req.PrivacyPolicy),
		Request:              req,
		Client:               fastJitterProtocolClient(server.URL, 3, client),
		Now:                  func() time.Time { return tlsTestNow },
	})
	if err == nil {
		t.Fatalf("networkrun.Run(corrupt receiver metadata) error = nil, want receiver artifact error")
	}
	transfer := readTLSNetworkTransfer(t, targetRoot, sessionID)
	assertTLSNetworkTransferState(t, transfer, control.NetworkTransferFailed, "transport", "transfer_failed")
	if !strings.Contains(transfer.Error, "unknown field") {
		t.Fatalf("network transfer error = %q, want corrupted metadata detail", transfer.Error)
	}
}

func TestPinnedTLSServerRejectsWrongClientCertificate(t *testing.T) {
	targetRoot := t.TempDir()
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	wrongSourceCert := newTestCertificate(t, "source-wrong", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	handler := receiver.NewAuthenticatedHandler(receiver.FileStore{TargetRoot: targetRoot}, receiver.AuthenticatedHandlerOptions{
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
	})
	server, _ := newPinnedTLSServerAndClient(t, handler, peer, sourceCert, targetCert)
	defer server.Close()

	wrongSourceID := certDeviceID(t, wrongSourceCert)
	clientTLS, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{wrongSourceCert},
		SourceDeviceID: wrongSourceID,
		TargetDeviceID: peer.TargetDeviceID,
		Time:           func() time.Time { return tlsTestNow },
	})
	if err != nil {
		t.Fatalf("ClientTLSConfig(wrong source) error = %v, want nil", err)
	}
	req := newBeginRequest(t, server.URL+"/v1/sessions", validBeginRequest(peer, []byte("abc")))
	_, err = (&http.Client{Transport: &http.Transport{TLSClientConfig: clientTLS}}).Do(req)
	if err == nil {
		t.Fatalf("Do with wrong client certificate error = nil, want TLS failure")
	}
	assertNoSessionsDir(t, targetRoot)
}

func TestPinnedTLSClientRejectsWrongServerIdentity(t *testing.T) {
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	wrongTargetCert := newTestCertificate(t, "target-wrong", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	expectedPeer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	servedPeer := expectedPeer
	servedPeer.TargetDeviceID = certDeviceID(t, wrongTargetCert)
	var reached atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	server, _ := newPinnedTLSServerAndClient(t, handler, servedPeer, sourceCert, wrongTargetCert)
	defer server.Close()

	clientTLS, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{sourceCert},
		SourceDeviceID: expectedPeer.SourceDeviceID,
		TargetDeviceID: expectedPeer.TargetDeviceID,
		Time:           func() time.Time { return tlsTestNow },
	})
	if err != nil {
		t.Fatalf("ClientTLSConfig(expected target) error = %v, want nil", err)
	}
	resp, err := (&http.Client{Transport: &http.Transport{TLSClientConfig: clientTLS}}).Get(server.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatalf("Get with wrong server identity error = nil status = %s, want TLS failure", resp.Status)
	}
	if reached.Load() != 0 {
		t.Fatalf("handler reached after wrong server identity = %d, want 0", reached.Load())
	}
}

func TestPinnedTLSReceiverRejectsWrongProfileTargetAfterTLS(t *testing.T) {
	targetRoot := t.TempDir()
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	handler := receiver.NewAuthenticatedHandler(receiver.FileStore{TargetRoot: targetRoot}, receiver.AuthenticatedHandlerOptions{
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
	})
	server, client := newPinnedTLSServerAndClient(t, handler, peer, sourceCert, targetCert)
	defer server.Close()

	begin := validBeginRequest(peer, []byte("abc"))
	begin.TargetID = "local:other-target"
	resp, err := client.Do(newBeginRequest(t, server.URL+"/v1/sessions", begin))
	if err != nil {
		t.Fatalf("Do wrong target begin error = %v, want HTTP forbidden", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("wrong target begin status = %d, want %d body %s", resp.StatusCode, http.StatusForbidden, string(body))
	}
	assertNoSessionsDir(t, targetRoot)
}

func TestPinnedTLSConfigRejectsExpiredLocalCertificate(t *testing.T) {
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-2*time.Hour), tlsTestNow.Add(-time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	sourceID := certDeviceID(t, sourceCert)
	targetID := certDeviceID(t, targetCert)

	_, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{sourceCert},
		SourceDeviceID: sourceID,
		TargetDeviceID: targetID,
		Time:           func() time.Time { return tlsTestNow },
	})
	if err == nil {
		t.Fatalf("ClientTLSConfig(expired local cert) error = nil, want failure")
	}
	if !errors.Is(err, transport.ErrTLSPeerCertificate) {
		t.Fatalf("ClientTLSConfig(expired local cert) error = %v, want ErrTLSPeerCertificate", err)
	}
}

func TestPinnedTLSConfigRejectsMultipleLocalCertificates(t *testing.T) {
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	sourceID := certDeviceID(t, sourceCert)
	targetID := certDeviceID(t, targetCert)

	_, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{sourceCert, targetCert},
		SourceDeviceID: sourceID,
		TargetDeviceID: targetID,
		Time:           func() time.Time { return tlsTestNow },
	})
	if err == nil {
		t.Fatalf("ClientTLSConfig(multiple local certs) error = nil, want failure")
	}
	if !errors.Is(err, transport.ErrTLSConfig) {
		t.Fatalf("ClientTLSConfig(multiple local certs) error = %v, want ErrTLSConfig", err)
	}
}

func TestPinnedTLSConfigRejectsMismatchedLeafCertificate(t *testing.T) {
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	mismatched := sourceCert
	mismatched.Leaf = targetCert.Leaf
	sourceID := certDeviceID(t, sourceCert)
	targetID := certDeviceID(t, targetCert)

	_, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{mismatched},
		SourceDeviceID: sourceID,
		TargetDeviceID: targetID,
		Time:           func() time.Time { return tlsTestNow },
	})
	if err == nil {
		t.Fatalf("ClientTLSConfig(mismatched Leaf) error = nil, want failure")
	}
	if !errors.Is(err, transport.ErrTLSConfig) {
		t.Fatalf("ClientTLSConfig(mismatched Leaf) error = %v, want ErrTLSConfig", err)
	}
}

func TestPinnedTLSServerRejectsCorruptedTLSFrame(t *testing.T) {
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	var reached atomic.Int32
	server, _ := newPinnedTLSServerAndClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}), peer, sourceCert, targetCert)
	defer server.Close()

	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("net.Dial(%s) error = %v, want nil", server.Listener.Addr(), err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetDeadline error = %v, want nil", err)
	}
	if _, err := conn.Write([]byte("not a TLS record\r\n\r\n")); err != nil {
		t.Fatalf("Write(corrupted TLS frame) error = %v, want nil", err)
	}
	_, _ = conn.Read(make([]byte, 1))
	if reached.Load() != 0 {
		t.Fatalf("handler reached after corrupted TLS frame = %d, want 0", reached.Load())
	}
}

func TestPinnedTLSClientHonorsCancelledContext(t *testing.T) {
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	var reached atomic.Int32
	server, client := newPinnedTLSServerAndClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}), peer, sourceCert, targetCert)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext error = %v, want nil", err)
	}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Fatalf("client.Do(canceled request) error = nil, want context canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("client.Do(canceled request) error = %v, want context.Canceled", err)
	}
	if reached.Load() != 0 {
		t.Fatalf("handler reached after canceled request = %d, want 0", reached.Load())
	}
}

func TestPinnedTLSClientHonorsTimeoutAfterAuthentication(t *testing.T) {
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	started := make(chan struct{})
	released := make(chan struct{})
	handlerErr := make(chan error, 1)
	blockingApp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := transport.AuthenticatedPeerFromContext(r.Context())
		if !ok {
			handlerErr <- errors.New("authenticated peer context missing after TLS")
		} else if got != peer {
			handlerErr <- fmt.Errorf("authenticated peer = %+v, want %+v", got, peer)
		}
		close(started)
		<-r.Context().Done()
		close(released)
	})
	wrapped, err := transport.NewTLSAuthenticatedPeerHandler(blockingApp, transport.AuthenticatedPeerTLSOptions{
		Peer: peer,
		Time: func() time.Time { return tlsTestNow },
	})
	if err != nil {
		t.Fatalf("NewTLSAuthenticatedPeerHandler error = %v, want nil", err)
	}
	serverTLS, err := transport.ServerTLSConfig(transport.ServerTLSOptions{
		Certificates: []tls.Certificate{targetCert},
		Peer:         peer,
		Time:         func() time.Time { return tlsTestNow },
	})
	if err != nil {
		t.Fatalf("ServerTLSConfig error = %v, want nil", err)
	}
	server := httptest.NewUnstartedServer(wrapped)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()
	clientTLS, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{sourceCert},
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
		Time:           func() time.Time { return tlsTestNow },
	})
	if err != nil {
		t.Fatalf("ClientTLSConfig error = %v, want nil", err)
	}
	done := make(chan struct {
		resp *http.Response
		err  error
	}, 1)
	go func() {
		client := &http.Client{
			Transport: &http.Transport{TLSClientConfig: clientTLS},
			Timeout:   100 * time.Millisecond,
		}
		resp, err := client.Get(server.URL)
		done <- struct {
			resp *http.Response
			err  error
		}{resp: resp, err: err}
	}()

	select {
	case <-started:
	case result := <-done:
		if result.resp != nil {
			result.resp.Body.Close()
		}
		t.Fatalf("client.Get finished before authenticated handler reached: %v", result.err)
	case <-time.After(3 * time.Second):
		t.Fatal("authenticated handler was not reached before test timeout")
	}
	select {
	case err := <-handlerErr:
		t.Fatalf("authenticated handler context error: %v", err)
	default:
	}
	select {
	case result := <-done:
		if result.resp != nil {
			result.resp.Body.Close()
		}
		if result.err == nil {
			t.Fatalf("client.Get timeout error = nil, want timeout")
		}
		var netErr net.Error
		if !errors.As(result.err, &netErr) || !netErr.Timeout() {
			t.Fatalf("client.Get timeout error = %v, want net.Error timeout", result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client.Get did not return after timeout")
	}
	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("server handler did not observe request context cancellation")
	}
}

func TestTLSAuthenticatedPeerHandlerRequiresTLS(t *testing.T) {
	sourceCert := newTestCertificate(t, "source", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", tlsTestNow.Add(-time.Hour), tlsTestNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	var reached atomic.Int32
	handler, err := transport.NewTLSAuthenticatedPeerHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}), transport.AuthenticatedPeerTLSOptions{
		Peer: peer,
		Time: func() time.Time { return tlsTestNow },
	})
	if err != nil {
		t.Fatalf("NewTLSAuthenticatedPeerHandler error = %v, want nil", err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-TLS request status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if reached.Load() != 0 {
		t.Fatalf("wrapped handler reached without TLS = %d, want 0", reached.Load())
	}
}

func newPinnedTLSServerAndClient(
	t *testing.T,
	handler http.Handler,
	peer transport.AuthenticatedPeer,
	sourceCert tls.Certificate,
	targetCert tls.Certificate,
) (*httptest.Server, *http.Client) {
	t.Helper()
	wrapped, err := transport.NewTLSAuthenticatedPeerHandler(handler, transport.AuthenticatedPeerTLSOptions{
		Peer: peer,
		Time: func() time.Time { return tlsTestNow },
	})
	if err != nil {
		t.Fatalf("NewTLSAuthenticatedPeerHandler error = %v, want nil", err)
	}
	serverTLS, err := transport.ServerTLSConfig(transport.ServerTLSOptions{
		Certificates: []tls.Certificate{targetCert},
		Peer:         peer,
		Time:         func() time.Time { return tlsTestNow },
	})
	if err != nil {
		t.Fatalf("ServerTLSConfig error = %v, want nil", err)
	}
	server := httptest.NewUnstartedServer(wrapped)
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = serverTLS
	server.StartTLS()
	clientTLS, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{sourceCert},
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
		Time:           func() time.Time { return tlsTestNow },
	})
	if err != nil {
		server.Close()
		t.Fatalf("ClientTLSConfig error = %v, want nil", err)
	}
	return server, &http.Client{Transport: &http.Transport{TLSClientConfig: clientTLS}}
}

func newPinnedTLSReceiverServerAndClient(
	t *testing.T,
	targetRoot string,
	peer transport.AuthenticatedPeer,
	sourceCert tls.Certificate,
	targetCert tls.Certificate,
) (*httptest.Server, *http.Client) {
	t.Helper()
	handler := receiver.NewAuthenticatedHandler(receiver.FileStore{TargetRoot: targetRoot}, receiver.AuthenticatedHandlerOptions{
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
	})
	return newPinnedTLSServerAndClient(t, handler, peer, sourceCert, targetCert)
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

func validTLSNetworkTransferRequest(source string, result scan.Result, sessionID string, peer transport.AuthenticatedPeer) protocolclient.TransferRequest {
	return protocolclient.TransferRequest{
		SourceRoot:     source,
		Scan:           result,
		SessionID:      sessionID,
		ManifestID:     "manifest-" + sessionID,
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
		PrivacyPolicy:  fastJitterPrivacyPolicy(),
		RootID:         "root1",
		CreatedAt:      tlsTestNow,
		EndedAt:        tlsTestNow.Add(time.Minute),
	}
}

func fastJitterPrivacyPolicy() transport.PrivacyPolicy {
	policy := transport.DefaultPrivacyPolicy(transport.PrivacyLevel2)
	policy.JitterBudget = 1
	return policy
}

func profilePrivacyPolicyFromTransport(policy transport.PrivacyPolicy) profile.PrivacyPolicy {
	return profile.PrivacyPolicy{
		Mode:                    profile.PrivacyModePlaintext,
		TrafficLevel:            int(policy.Level),
		AllowPlaintextRestore:   true,
		AllowHiddenFiles:        true,
		AllowSensitiveFilenames: true,
		PaddingBucketBytes:      policy.PaddingBucket,
		BatchMaxBytes:           policy.BatchMaxBytes,
		BatchMaxCount:           policy.BatchMaxCount,
		JitterBudgetMillis:      policy.JitterBudget,
		DiscoveryLowInfo:        policy.DiscoveryLowInfo,
	}
}

func readTLSNetworkTransfer(t *testing.T, targetRoot, sessionID string) control.NetworkTransfer {
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

func assertTLSNetworkTransferState(t *testing.T, transfer control.NetworkTransfer, status control.NetworkTransferStatus, stage, code string) {
	t.Helper()
	if transfer.Status != status || transfer.Stage != stage || transfer.ErrorCode != code {
		t.Fatalf("network transfer = %+v, want status=%q stage=%q error_code=%q", transfer, status, stage, code)
	}
	if len(transfer.Attempts) == 0 {
		t.Fatalf("network transfer attempts = 0, want at least 1")
	}
	attempt := transfer.Attempts[len(transfer.Attempts)-1]
	if attempt.Status != status || attempt.Stage != stage || attempt.ErrorCode != code {
		t.Fatalf("network transfer last attempt = %+v, want status=%q stage=%q error_code=%q", attempt, status, stage, code)
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

func pairedProfile(t *testing.T, sourceRoot, targetRoot string, peer transport.AuthenticatedPeer) profile.Profile {
	t.Helper()
	p := profile.NewDefault(peer.ProfileID, "Profile", sourceRoot, targetRoot)
	p.Target.TargetID = peer.TargetID
	p.Target.DevicePublicKey = peer.TargetDeviceID
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = tlsTestNow.Format(time.RFC3339)
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

func validBeginRequest(peer transport.AuthenticatedPeer, data []byte) protocol.BeginSessionRequest {
	return protocol.BeginSessionRequest{
		ProtocolVersion: protocol.Version,
		SessionID:       "session-1",
		ProfileID:       peer.ProfileID,
		TargetID:        peer.TargetID,
		SourceDeviceID:  peer.SourceDeviceID,
		TargetDeviceID:  peer.TargetDeviceID,
		RootID:          "root1",
		CreatedAt:       tlsTestNow,
		Manifest: protocol.TransferManifest{
			ID: "manifest-1",
			Entries: []protocol.ManifestEntry{
				{Path: "docs", Kind: protocol.FileKindDir},
				{
					Path:    "docs/a.txt",
					Kind:    protocol.FileKindFile,
					Mode:    0o600,
					Size:    int64(len(data)),
					Digest:  digest(data),
					ModTime: tlsTestNow,
				},
			},
		},
	}
}

func newBeginRequest(t *testing.T, url string, begin protocol.BeginSessionRequest) *http.Request {
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

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func writeTLSPatternFile(path string, size int) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	pattern := []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ\n")
	remaining := size
	for remaining > 0 {
		n := min(remaining, len(pattern))
		if _, err := file.Write(pattern[:n]); err != nil {
			return err
		}
		remaining -= n
	}
	return file.Sync()
}

func hashTLSFile(t *testing.T, path string) string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open(%q) error = %v, want nil", path, err)
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		t.Fatalf("hash %q error = %v, want nil", path, err)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func assertNoPublishedFile(t *testing.T, targetRoot, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(targetRoot, name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target file %q error = %v, want not exist", name, err)
	}
}

func assertNoSessionsDir(t *testing.T, targetRoot string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(targetRoot, ".supermover", "sessions")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sessions dir error = %v, want not exist", err)
	}
}

func fastJitterProtocolClient(baseURL string, chunkSize int, doer protocolclient.Doer) protocolclient.Client {
	return protocolclient.Client{
		BaseURL:   baseURL,
		Doer:      doer,
		ChunkSize: chunkSize,
		Now:       func() time.Time { return tlsTestNow },
	}
}

type failAfterAcceptedTLSChunkDoer struct {
	base      protocolclient.Doer
	failAfter int
	chunks    int
}

func (d *failAfterAcceptedTLSChunkDoer) Do(req *http.Request) (*http.Response, error) {
	resp, err := d.base.Do(req)
	if err != nil {
		return resp, err
	}
	if req.Method == http.MethodPost && (req.URL.Path == "/v1/chunks" || req.URL.Path == "/v1/chunk-batches") && resp != nil && resp.StatusCode == http.StatusAccepted {
		d.chunks++
		if d.chunks >= d.failAfter {
			if resp != nil && resp.Body != nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
			return nil, errors.New("simulated interruption after accepted TLS chunk")
		}
	}
	return resp, nil
}
