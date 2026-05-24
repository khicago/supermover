package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pairserve"
	"github.com/khicago/supermover/internal/profile"
)

func TestServeWritesStructuredReadyFileForPairingOnly(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	readyPath := filepath.Join(dir, "serve-ready.json")
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
	done := make(chan int, 1)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	go func() {
		done <- Runner{
			Context: ctx,
			ServePairingReady: func(info pairserve.ReadyInfo) {
				pairingReady <- info
			},
		}.Run([]string{"serve", "--profile", profilePath, "--listen", "127.0.0.1:0", "--ready-file", readyPath}, &stdout, &stderr)
	}()
	info := waitServePairingReady(t, pairingReady, &stderr)
	waitForFile(t, readyPath, 2*time.Second, &stdout, &stderr)
	var ready serveReadyEvidence
	data, err := os.ReadFile(readyPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", readyPath, err)
	}
	if err := json.Unmarshal(data, &ready); err != nil {
		t.Fatalf("json.Unmarshal(ready file) error = %v, want nil", err)
	}
	if ready.Address != info.Address {
		t.Fatalf("ready file address = %q, want %q", ready.Address, info.Address)
	}
	if ready.VerificationCode != info.VerificationCode {
		t.Fatalf("ready file verification_code = %q, want %q", ready.VerificationCode, info.VerificationCode)
	}
	if ready.Mode != "pairing-only" {
		t.Fatalf("ready file mode = %q, want pairing-only", ready.Mode)
	}
	if ready.Trusted || ready.Transfer {
		t.Fatalf("ready file = %+v, want trusted=false transfer=false", ready)
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
}

func TestServeWritesStructuredReadyFileForPairedReceiver(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	readyPath := filepath.Join(dir, "receiver-ready.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceCert := newCLITestCertificate(t, "source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	peer := cliAuthenticatedPeerForCerts(t, sourceCert, targetCert)
	receiverListener := listenOnReservedTCPAddress(t)
	t.Cleanup(func() {
		_ = receiverListener.Close()
	})
	receiverAddress := receiverListener.Addr().String()
	targetProfile := profile.NewDefault(peer.ProfileID, "Target profile", source, target)
	targetProfile.Target.TargetID = peer.TargetID
	targetProfile.Target.DevicePublicKey = peer.TargetDeviceID
	targetProfile.Target.PairingReceiptID = "pairing-1"
	targetProfile.Target.PairedAt = cliTLSNow().Format(time.RFC3339)
	targetProfile.Network = networkConfigForCLI(t, targetCert, receiverAddress)
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	writePairingReceiptForCLI(t, target, targetProfile, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = peer.SourceDeviceID
		receipt.TargetDeviceID = peer.TargetDeviceID
		receipt.DevicePublicKey = peer.TargetDeviceID
		receipt.VerifiedAt = targetProfile.Target.PairedAt
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	receiverReady := make(chan string, 1)
	done := make(chan int, 1)
	var serveStdout bytes.Buffer
	var serveStderr bytes.Buffer
	go func() {
		done <- Runner{
			Context:                 ctx,
			Now:                     cliTLSNow(),
			receiverListenerForTest: receiverListener,
			ServeReady: func(address string) {
				receiverReady <- address
			},
		}.Run([]string{"serve", "--profile", targetProfilePath, "--listen", "127.0.0.1:0", "--ready-file", readyPath}, &serveStdout, &serveStderr)
	}()
	waitForFile(t, readyPath, 2*time.Second, &serveStdout, &serveStderr)
	select {
	case <-receiverReady:
	case <-time.After(2 * time.Second):
		t.Fatalf("serve receiver did not report ready; stderr=%q", serveStderr.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	var ready serveReadyEvidence
	for {
		data, err := os.ReadFile(readyPath)
		if err == nil && json.Unmarshal(data, &ready) == nil && ready.ReceiverRoutes && ready.ReceiverAddress != "" {
			break
		}
		if time.Now().After(deadline) {
			raw, _ := os.ReadFile(readyPath)
			t.Fatalf("receiver ready file not updated before timeout; ready=%q stderr=%q", string(raw), serveStderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ready.Address == "" || ready.ReceiverAddress == "" {
		t.Fatalf("receiver ready file = %+v, want receiver addresses", ready)
	}
	if ready.Mode != "pairing" || !ready.Trusted || !ready.Transfer || !ready.ReceiverRoutes || !ready.PushNetwork {
		t.Fatalf("receiver ready file = %+v, want paired serve evidence with receiver routes", ready)
	}
	cancel()
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("paired serve exit = %d stderr = %q, want 0", got, serveStderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("paired serve did not exit after cancel; stderr=%q", serveStderr.String())
	}
}

func TestServeRejectsSymlinkReadyFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	realPath := filepath.Join(dir, "real-ready.json")
	linkPath := filepath.Join(dir, "ready-link.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	if err := os.WriteFile(realPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", realPath, err)
	}
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("os.Symlink(%q, %q) error = %v, want nil", realPath, linkPath, err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"serve", "--profile", profilePath, "--listen", "127.0.0.1:0", "--ready-file", linkPath}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("serve symlink ready-file exit = %d stderr = %q, want 2", got, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("ready-file")) || !bytes.Contains(stderr.Bytes(), []byte("symlink")) {
		t.Fatalf("serve symlink ready-file stderr = %q, want symlink refusal", stderr.String())
	}
}

func TestServeReadyStateDoesNotOverwriteReceiverEvidenceWithStalePairingSnapshot(t *testing.T) {
	var (
		writes   []serveReadyEvidence
		writesMu sync.Mutex
	)
	pairingWriteStarted := make(chan struct{})
	allowPairingWrite := make(chan struct{})
	state := &serveReadyState{
		path: "ignored",
		writer: func(_ string, doc serveReadyEvidence) error {
			if !doc.Transfer {
				close(pairingWriteStarted)
				<-allowPairingWrite
			}
			writesMu.Lock()
			writes = append(writes, doc)
			writesMu.Unlock()
			return nil
		},
	}

	pairingDone := make(chan struct{})
	go func() {
		defer close(pairingDone)
		state.recordPairing(serveReadyEvidence{
			Address:          "[::]:41000",
			VerificationCode: "verify-1",
			Mode:             "pairing",
			Trusted:          false,
			Transfer:         false,
			ExpiresAt:        "2026-06-01T00:00:00Z",
		})
	}()

	<-pairingWriteStarted
	receiverDone := make(chan struct{})
	go func() {
		defer close(receiverDone)
		state.recordReceiver(serveReadyEvidence{
			Address:         "127.0.0.1:9443",
			Mode:            "receiver-tls",
			ReceiverAddress: "127.0.0.1:9443",
			ReceiverRoutes:  true,
			PushNetwork:     true,
			Trusted:         true,
			Transfer:        true,
		})
	}()
	select {
	case <-receiverDone:
		t.Fatal("receiver write completed before pairing write released")
	default:
	}
	close(allowPairingWrite)
	<-pairingDone
	<-receiverDone

	writesMu.Lock()
	defer writesMu.Unlock()
	if len(writes) != 2 {
		t.Fatalf("writes = %d, want 2", len(writes))
	}
	final := writes[len(writes)-1]
	if !final.Transfer || !final.Trusted || !final.ReceiverRoutes || final.ReceiverAddress == "" {
		t.Fatalf("final ready evidence = %+v, want receiver-ready state to win", final)
	}
}
