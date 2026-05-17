package tlsidentity_test

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
	"github.com/khicago/supermover/internal/tlsidentity"
)

func TestLoadRejectsUnsafeFiles(t *testing.T) {
	ref := writeIdentity(t)
	tests := []struct {
		name   string
		mutate func(profile.TLSIdentityRef) profile.TLSIdentityRef
		want   string
	}{
		{
			name: "missing certificate",
			mutate: func(ref profile.TLSIdentityRef) profile.TLSIdentityRef {
				ref.CertificatePath = filepath.Join(t.TempDir(), "missing.crt")
				return ref
			},
			want: "certificate file",
		},
		{
			name: "certificate symlink",
			mutate: func(ref profile.TLSIdentityRef) profile.TLSIdentityRef {
				link := filepath.Join(t.TempDir(), "identity-link.crt")
				if err := os.Symlink(ref.CertificatePath, link); err != nil {
					t.Skipf("os.Symlink(cert) unavailable: %v", err)
				}
				ref.CertificatePath = link
				return ref
			},
			want: "must not be a symlink",
		},
		{
			name: "private key symlink",
			mutate: func(ref profile.TLSIdentityRef) profile.TLSIdentityRef {
				link := filepath.Join(t.TempDir(), "identity-link.key")
				if err := os.Symlink(ref.PrivateKeyPath, link); err != nil {
					t.Skipf("os.Symlink(key) unavailable: %v", err)
				}
				ref.PrivateKeyPath = link
				return ref
			},
			want: "must not be a symlink",
		},
		{
			name: "private key group readable",
			mutate: func(ref profile.TLSIdentityRef) profile.TLSIdentityRef {
				if err := os.Chmod(ref.PrivateKeyPath, 0o640); err != nil {
					t.Fatalf("os.Chmod(private key) error = %v, want nil", err)
				}
				return ref
			},
			want: "must not be readable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tlsidentity.Load(tt.mutate(ref))
			if err == nil {
				t.Fatal("Load() error = nil, want unsafe identity refusal")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestLoadAcceptsRegularPrivateIdentityFiles(t *testing.T) {
	cert, err := tlsidentity.Load(writeIdentity(t))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("Load() certificate chain is empty")
	}
}

func writeIdentity(t *testing.T) profile.TLSIdentityRef {
	t.Helper()
	cert := newCertificate(t)
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

func newCertificate(t *testing.T) tls.Certificate {
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
		Subject:               pkix.Name{CommonName: "identity"},
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
