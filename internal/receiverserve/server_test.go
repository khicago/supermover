package receiverserve_test

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
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/receiverserve"
	"github.com/khicago/supermover/internal/transport"
)

var testNow = time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)

func TestListenAndServeAcceptsPinnedMutualTLSBeginAndStatus(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	prof.Network = networkConfig(t, targetCert, reserveAddress(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readyCh := make(chan receiverserve.ReadyInfo, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- receiverserve.ListenAndServe(ctx, receiverserve.Options{
			Profile: prof,
			Now:     func() time.Time { return testNow },
			Ready: func(info receiverserve.ReadyInfo) {
				readyCh <- info
			},
		})
	}()
	ready := waitReady(t, readyCh)
	if ready.Peer != peer {
		t.Fatalf("Ready peer = %+v, want %+v", ready.Peer, peer)
	}
	if ready.Address == "" {
		t.Fatal("Ready address is empty")
	}
	client := pinnedClient(t, sourceCert, peer)
	begin := validBeginRequest(peer, []byte("hello receiver serve\n"))
	resp, err := client.Do(newBeginRequest(t, "https://"+ready.Address+"/v1/sessions", begin))
	if err != nil {
		t.Fatalf("POST /v1/sessions error = %v, want nil", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST /v1/sessions status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}
	statusResp, err := client.Get("https://" + ready.Address + "/v1/sessions/" + begin.SessionID + "/status")
	if err != nil {
		t.Fatalf("GET status error = %v, want nil", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", statusResp.StatusCode, http.StatusOK)
	}
	cancel()
	if err := waitServeDone(t, errCh); err != nil {
		t.Fatalf("ListenAndServe after cancel error = %v, want nil", err)
	}
}

func TestListenAndServeRejectsPlainHTTPWithoutSessionArtifacts(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	prof.Network = networkConfig(t, targetCert, reserveAddress(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readyCh := make(chan receiverserve.ReadyInfo, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- receiverserve.ListenAndServe(ctx, receiverserve.Options{
			Profile: prof,
			Now:     func() time.Time { return testNow },
			Ready: func(info receiverserve.ReadyInfo) {
				readyCh <- info
			},
		})
	}()
	ready := waitReady(t, readyCh)
	begin := validBeginRequest(peer, []byte("plain http must not land"))
	resp, err := http.DefaultClient.Do(newBeginRequest(t, "http://"+ready.Address+"/v1/sessions", begin))
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode < 400 {
			t.Fatalf("plain HTTP status = %d, want failure", resp.StatusCode)
		}
	}
	assertNoSessionArtifacts(t, targetRoot, begin.SessionID)
	cancel()
	if err := waitServeDone(t, errCh); err != nil {
		t.Fatalf("ListenAndServe after cancel error = %v, want nil", err)
	}
}

func TestListenAndServeRejectsWrongClientCertWithoutSessionArtifacts(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	wrongSourceCert := newTestCertificate(t, "source-wrong", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	prof.Network = networkConfig(t, targetCert, reserveAddress(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readyCh := make(chan receiverserve.ReadyInfo, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- receiverserve.ListenAndServe(ctx, receiverserve.Options{
			Profile: prof,
			Now:     func() time.Time { return testNow },
			Ready: func(info receiverserve.ReadyInfo) {
				readyCh <- info
			},
		})
	}()
	ready := waitReady(t, readyCh)
	client := pinnedClient(t, wrongSourceCert, transport.AuthenticatedPeer{
		ProfileID:      peer.ProfileID,
		TargetID:       peer.TargetID,
		SourceDeviceID: certDeviceID(t, wrongSourceCert),
		TargetDeviceID: peer.TargetDeviceID,
	})
	begin := validBeginRequest(peer, []byte("wrong cert must not land"))
	resp, err := client.Do(newBeginRequest(t, "https://"+ready.Address+"/v1/sessions", begin))
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode < 400 {
			t.Fatalf("wrong client cert status = %d, want failure", resp.StatusCode)
		}
	}
	assertNoSessionArtifacts(t, targetRoot, begin.SessionID)
	cancel()
	if err := waitServeDone(t, errCh); err != nil {
		t.Fatalf("ListenAndServe after cancel error = %v, want nil", err)
	}
}

func TestListenAndServeReturnsErrorBeforeReadyForInvalidMaterial(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	validProfile := pairedProfile(t, sourceRoot, targetRoot, peer)
	validProfile.Network = networkConfig(t, targetCert, reserveAddress(t))

	tests := []struct {
		name   string
		mutate func(*profile.Profile)
	}{
		{
			name: "missing receiver url",
			mutate: func(p *profile.Profile) {
				p.Network.ReceiverURL = ""
			},
		},
		{
			name: "missing network",
			mutate: func(p *profile.Profile) {
				p.Network = nil
			},
		},
		{
			name: "unpaired profile",
			mutate: func(p *profile.Profile) {
				p.Target.DevicePublicKey = ""
				p.Target.PairingReceiptID = ""
				p.Target.PairedAt = ""
			},
		},
		{
			name: "mismatched pairing receipt",
			mutate: func(p *profile.Profile) {
				p.Target.DevicePublicKey = certDeviceID(t, newTestCertificate(t, "target-other", testNow.Add(-time.Hour), testNow.Add(time.Hour)))
			},
		},
		{
			name: "mismatched local TLS identity",
			mutate: func(p *profile.Profile) {
				wrongTargetCert := newTestCertificate(t, "target-wrong", testNow.Add(-time.Hour), testNow.Add(time.Hour))
				p.Network = networkConfig(t, wrongTargetCert, reserveAddress(t))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prof := validProfile
			tt.mutate(&prof)
			var ready bool
			err := receiverserve.ListenAndServe(context.Background(), receiverserve.Options{
				Profile: prof,
				Now:     func() time.Time { return testNow },
				Ready: func(receiverserve.ReadyInfo) {
					ready = true
				},
			})
			if err == nil {
				t.Fatal("ListenAndServe error = nil, want failure")
			}
			if ready {
				t.Fatal("Ready fired for invalid material")
			}
		})
	}
}

func TestListenAndServeRejectsUnsafeTargetRootBeforeReady(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := filepath.Join(t.TempDir(), pathguard.ReservedControlDir)
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetRoot) error = %v, want nil", err)
	}
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	prof := pairedProfile(t, sourceRoot, targetRoot, peer)
	prof.Network = networkConfig(t, targetCert, reserveAddress(t))

	var ready bool
	err := receiverserve.ListenAndServe(context.Background(), receiverserve.Options{
		Profile: prof,
		Now:     func() time.Time { return testNow },
		Ready: func(receiverserve.ReadyInfo) {
			ready = true
		},
	})
	if err == nil {
		t.Fatal("ListenAndServe error = nil, want unsafe target root failure")
	}
	if ready {
		t.Fatal("Ready fired for unsafe target root")
	}
}

func TestNewRejectsUnsafeLocalTLSIdentityFiles(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	sourceCert := newTestCertificate(t, "source", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	targetCert := newTestCertificate(t, "target", testNow.Add(-time.Hour), testNow.Add(time.Hour))
	peer := authenticatedPeerForCerts(t, sourceCert, targetCert)
	validProfile := pairedProfile(t, sourceRoot, targetRoot, peer)
	validProfile.Network = networkConfig(t, targetCert, reserveAddress(t))
	tests := []struct {
		name   string
		mutate func(*profile.Profile)
		want   string
	}{
		{
			name: "certificate symlink",
			mutate: func(p *profile.Profile) {
				realCert := p.Network.LocalTLSIdentity.CertificatePath
				linkPath := filepath.Join(t.TempDir(), "identity-link.crt")
				if err := os.Symlink(realCert, linkPath); err != nil {
					t.Skipf("os.Symlink(cert) unavailable: %v", err)
				}
				p.Network.LocalTLSIdentity.CertificatePath = linkPath
			},
			want: "must not be a symlink",
		},
		{
			name: "private key symlink",
			mutate: func(p *profile.Profile) {
				realKey := p.Network.LocalTLSIdentity.PrivateKeyPath
				linkPath := filepath.Join(t.TempDir(), "identity-link.key")
				if err := os.Symlink(realKey, linkPath); err != nil {
					t.Skipf("os.Symlink(key) unavailable: %v", err)
				}
				p.Network.LocalTLSIdentity.PrivateKeyPath = linkPath
			},
			want: "must not be a symlink",
		},
		{
			name: "private key group readable",
			mutate: func(p *profile.Profile) {
				if err := os.Chmod(p.Network.LocalTLSIdentity.PrivateKeyPath, 0o640); err != nil {
					t.Fatalf("os.Chmod(private key) error = %v, want nil", err)
				}
			},
			want: "must not be readable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prof := validProfile
			prof.Network = networkConfig(t, targetCert, reserveAddress(t))
			tt.mutate(&prof)
			_, err := receiverserve.New(receiverserve.Options{
				Profile: prof,
				Now:     func() time.Time { return testNow },
			})
			if err == nil {
				t.Fatal("New() error = nil, want unsafe identity file failure")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("New() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func reserveAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen reserve address error = %v, want nil", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("reserve listener Close error = %v, want nil", err)
	}
	return address
}

func networkConfig(t *testing.T, cert tls.Certificate, address string) *profile.NetworkConfig {
	t.Helper()
	certPath, keyPath := writeCertificateFiles(t, cert)
	return &profile.NetworkConfig{
		ReceiverURL: "https://" + address,
		LocalTLSIdentity: profile.TLSIdentityRef{
			CertificatePath: certPath,
			PrivateKeyPath:  keyPath,
		},
	}
}

func writeCertificateFiles(t *testing.T, cert tls.Certificate) (string, string) {
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
		t.Fatalf("WriteFile(cert) error = %v, want nil", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("WriteFile(key) error = %v, want nil", err)
	}
	return certPath, keyPath
}

func waitReady(t *testing.T, readyCh <-chan receiverserve.ReadyInfo) receiverserve.ReadyInfo {
	t.Helper()
	select {
	case ready := <-readyCh:
		return ready
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Ready")
		return receiverserve.ReadyInfo{}
	}
}

func waitServeDone(t *testing.T, errCh <-chan error) error {
	t.Helper()
	select {
	case err := <-errCh:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ListenAndServe to exit")
		return nil
	}
}

func pinnedClient(t *testing.T, sourceCert tls.Certificate, peer transport.AuthenticatedPeer) *http.Client {
	t.Helper()
	clientTLS, err := transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{sourceCert},
		SourceDeviceID: peer.SourceDeviceID,
		TargetDeviceID: peer.TargetDeviceID,
		Time:           func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatalf("ClientTLSConfig error = %v, want nil", err)
	}
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: clientTLS},
		Timeout:   2 * time.Second,
	}
}

func assertNoSessionArtifacts(t *testing.T, targetRoot string, sessionID string) {
	t.Helper()
	sessionDir := filepath.Join(targetRoot, control.DirName, "sessions", sessionID)
	entries, err := os.ReadDir(sessionDir)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v, want nil or not-exist", sessionDir, err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("session artifacts for %q = %s, want none", sessionID, strings.Join(names, ","))
	}
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

func validBeginRequest(peer transport.AuthenticatedPeer, data []byte) protocol.BeginSessionRequest {
	return protocol.BeginSessionRequest{
		ProtocolVersion: protocol.Version,
		SessionID:       "session-1",
		ProfileID:       peer.ProfileID,
		TargetID:        peer.TargetID,
		SourceDeviceID:  peer.SourceDeviceID,
		TargetDeviceID:  peer.TargetDeviceID,
		RootID:          "root1",
		CreatedAt:       testNow,
		Manifest: protocol.TransferManifest{
			ID: "manifest-1",
			Entries: []protocol.ManifestEntry{
				{
					Path:    "a.txt",
					Kind:    protocol.FileKindFile,
					Mode:    0o600,
					Size:    int64(len(data)),
					Digest:  digest(data),
					ModTime: testNow,
				},
			},
		},
		PrivacyPolicy: transport.DefaultPrivacyPolicy(transport.PrivacyLevel2),
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
