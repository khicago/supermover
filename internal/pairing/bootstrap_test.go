package pairing

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/transport"
)

func TestBootstrapTargetDeviceIDUsesPinnedTransportCertificateSPKI(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	cert := newBootstrapTestCertificate(t, "target")
	p := profile.NewDefault("profile-local", "Target profile", source, target)
	p.Network = &profile.NetworkConfig{
		ReceiverURL:      "https://127.0.0.1:9443",
		LocalTLSIdentity: writeBootstrapIdentity(t, cert),
	}

	got, bound, err := BootstrapTargetDeviceID(p, time.Time{})
	if err != nil {
		t.Fatalf("BootstrapTargetDeviceID() error = %v, want nil", err)
	}
	want, err := transport.LeafSPKIDeviceID(cert.Leaf)
	if err != nil {
		t.Fatalf("LeafSPKIDeviceID() error = %v, want nil", err)
	}
	if !bound {
		t.Fatal("BootstrapTargetDeviceID() bound = false, want true")
	}
	if got != want {
		t.Fatalf("BootstrapTargetDeviceID() = %q, want cert SPKI %q", got, want)
	}
}

func TestBootstrapTargetDeviceIDFallsBackWithoutTransportIdentity(t *testing.T) {
	p := profile.NewDefault("profile-local", "Target profile", t.TempDir(), t.TempDir())

	got, bound, err := BootstrapTargetDeviceID(p, time.Time{})
	if err != nil {
		t.Fatalf("BootstrapTargetDeviceID() error = %v, want nil", err)
	}
	want, err := TargetDeviceID(p)
	if err != nil {
		t.Fatalf("TargetDeviceID() error = %v, want nil", err)
	}
	if bound {
		t.Fatal("BootstrapTargetDeviceID() bound = true, want false")
	}
	if got != want {
		t.Fatalf("BootstrapTargetDeviceID() = %q, want fallback target device id %q", got, want)
	}
}

func TestSourceTransportDeviceIDRequiresLocalTLSIdentity(t *testing.T) {
	p := profile.NewDefault("profile-local", "Source profile", t.TempDir(), t.TempDir())

	_, err := SourceTransportDeviceID(p, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "network.local_tls_identity") {
		t.Fatalf("SourceTransportDeviceID() error = %v, want missing local TLS identity", err)
	}
}

func TestSourceTransportDeviceIDUsesSourceCertificateSPKI(t *testing.T) {
	cert := newBootstrapTestCertificate(t, "source")
	p := profile.NewDefault("profile-local", "Source profile", t.TempDir(), t.TempDir())
	p.Network = &profile.NetworkConfig{
		ReceiverURL:      "https://127.0.0.1:9443",
		LocalTLSIdentity: writeBootstrapIdentity(t, cert),
	}

	got, err := SourceTransportDeviceID(p, time.Time{})
	if err != nil {
		t.Fatalf("SourceTransportDeviceID() error = %v, want nil", err)
	}
	want, err := transport.LeafSPKIDeviceID(cert.Leaf)
	if err != nil {
		t.Fatalf("LeafSPKIDeviceID() error = %v, want nil", err)
	}
	if got != want {
		t.Fatalf("SourceTransportDeviceID() = %q, want cert SPKI %q", got, want)
	}
}

func writeBootstrapIdentity(t *testing.T, cert tls.Certificate) profile.TLSIdentityRef {
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
	return profile.TLSIdentityRef{
		CertificatePath: certPath,
		PrivateKeyPath:  keyPath,
	}
}

func newBootstrapTestCertificate(t *testing.T, commonName string) tls.Certificate {
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
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
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
