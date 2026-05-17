package transport

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var (
	ErrTLSConfig          = errors.New("invalid tls transport config")
	ErrTLSPeerCertificate = errors.New("invalid tls peer certificate")
	ErrTLSPeerMismatch    = errors.New("tls peer identity mismatch")
)

type ServerTLSOptions struct {
	Certificates []tls.Certificate
	Peer         AuthenticatedPeer
	Time         func() time.Time
}

type ClientTLSOptions struct {
	Certificates   []tls.Certificate
	SourceDeviceID string
	TargetDeviceID string
	ServerName     string
	RootCAs        *x509.CertPool
	Time           func() time.Time
}

type AuthenticatedPeerTLSOptions struct {
	Peer AuthenticatedPeer
	Time func() time.Time
}

func LeafSPKIDeviceID(cert *x509.Certificate) (string, error) {
	if cert == nil {
		return "", fmt.Errorf("%w: certificate is nil", ErrTLSPeerCertificate)
	}
	if len(cert.RawSubjectPublicKeyInfo) == 0 {
		return "", fmt.Errorf("%w: certificate has no subject public key info", ErrTLSPeerCertificate)
	}
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	id := "sha256:" + hex.EncodeToString(sum[:])
	if err := DeviceID(id).Validate(); err != nil {
		return "", fmt.Errorf("%w: derived spki device id: %v", ErrTLSPeerCertificate, err)
	}
	return id, nil
}

func ServerTLSConfig(opts ServerTLSOptions) (*tls.Config, error) {
	if err := opts.Peer.Validate(); err != nil {
		return nil, fmt.Errorf("%w: authenticated peer: %v", ErrTLSConfig, err)
	}
	certificates, leaf, err := validateLocalCertificate(opts.Certificates, opts.Peer.TargetDeviceID, now(opts.Time))
	if err != nil {
		return nil, err
	}
	if err := validateCertificateTime(leaf, now(opts.Time)); err != nil {
		return nil, err
	}
	expectedSource := opts.Peer.SourceDeviceID
	nowFunc := nowFunc(opts.Time)
	return &tls.Config{
		MinVersion:             tls.VersionTLS13,
		Certificates:           certificates,
		ClientAuth:             tls.RequireAnyClientCert,
		SessionTicketsDisabled: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			return validatePinnedPeer(state, expectedSource, nowFunc())
		},
	}, nil
}

func ClientTLSConfig(opts ClientTLSOptions) (*tls.Config, error) {
	if err := DeviceID(opts.SourceDeviceID).Validate(); err != nil {
		return nil, fmt.Errorf("%w: source device id: %v", ErrTLSConfig, err)
	}
	if err := DeviceID(opts.TargetDeviceID).Validate(); err != nil {
		return nil, fmt.Errorf("%w: target device id: %v", ErrTLSConfig, err)
	}
	if opts.SourceDeviceID == opts.TargetDeviceID {
		return nil, fmt.Errorf("%w: source and target device ids must differ", ErrTLSConfig)
	}
	certificates, leaf, err := validateLocalCertificate(opts.Certificates, opts.SourceDeviceID, now(opts.Time))
	if err != nil {
		return nil, err
	}
	if err := validateCertificateTime(leaf, now(opts.Time)); err != nil {
		return nil, err
	}
	expectedTarget := opts.TargetDeviceID
	nowFunc := nowFunc(opts.Time)
	cfg := &tls.Config{
		MinVersion:             tls.VersionTLS13,
		Certificates:           certificates,
		RootCAs:                opts.RootCAs,
		ServerName:             opts.ServerName,
		SessionTicketsDisabled: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			return validatePinnedPeer(state, expectedTarget, nowFunc())
		},
	}
	if opts.RootCAs == nil {
		// Direct SPKI pinning is the trust boundary for paired Supermover devices.
		// Go's normal verifier needs a CA pool, so VerifyConnection above performs
		// the required peer certificate, validity-window, and pinned identity checks.
		cfg.InsecureSkipVerify = true
	}
	return cfg, nil
}

func NewTLSAuthenticatedPeerHandler(next http.Handler, opts AuthenticatedPeerTLSOptions) (http.Handler, error) {
	if next == nil {
		return nil, fmt.Errorf("%w: handler is nil", ErrTLSConfig)
	}
	if err := opts.Peer.Validate(); err != nil {
		return nil, fmt.Errorf("%w: authenticated peer: %v", ErrTLSConfig, err)
	}
	nowFunc := nowFunc(opts.Time)
	expectedSource := opts.Peer.SourceDeviceID
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			http.Error(w, "authenticated TLS transport is required", http.StatusForbidden)
			return
		}
		if err := validatePinnedPeer(*r.TLS, expectedSource, nowFunc()); err != nil {
			http.Error(w, "authenticated TLS transport is required", http.StatusForbidden)
			return
		}
		ctx := ContextWithAuthenticatedPeer(r.Context(), opts.Peer)
		next.ServeHTTP(w, r.WithContext(ctx))
	}), nil
}

func validateLocalCertificate(certs []tls.Certificate, expectedDeviceID string, at time.Time) ([]tls.Certificate, *x509.Certificate, error) {
	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("%w: certificate is required", ErrTLSConfig)
	}
	if len(certs) != 1 {
		return nil, nil, fmt.Errorf("%w: exactly one pinned certificate is required", ErrTLSConfig)
	}
	copied := []tls.Certificate{cloneCertificate(certs[0])}
	leaf, err := parsedLeafCertificate(copied[0])
	if err != nil {
		return nil, nil, err
	}
	if err := validateCertificateTime(leaf, at); err != nil {
		return nil, nil, err
	}
	deviceID, err := LeafSPKIDeviceID(leaf)
	if err != nil {
		return nil, nil, err
	}
	if deviceID != expectedDeviceID {
		return nil, nil, fmt.Errorf("%w: local certificate spki %q does not match pinned device id %q", ErrTLSPeerMismatch, deviceID, expectedDeviceID)
	}
	copied[0].Leaf = leaf
	return copied, leaf, nil
}

func parsedLeafCertificate(cert tls.Certificate) (*x509.Certificate, error) {
	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("%w: certificate chain is empty", ErrTLSConfig)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("%w: parse leaf certificate: %v", ErrTLSConfig, err)
	}
	if cert.Leaf != nil && !cert.Leaf.Equal(leaf) {
		return nil, fmt.Errorf("%w: leaf certificate does not match certificate chain", ErrTLSConfig)
	}
	return leaf, nil
}

func cloneCertificate(cert tls.Certificate) tls.Certificate {
	cloned := cert
	cloned.Certificate = cloneBytesSlice(cert.Certificate)
	cloned.OCSPStaple = append([]byte(nil), cert.OCSPStaple...)
	cloned.SignedCertificateTimestamps = cloneBytesSlice(cert.SignedCertificateTimestamps)
	cloned.Leaf = cert.Leaf
	return cloned
}

func cloneBytesSlice(values [][]byte) [][]byte {
	if values == nil {
		return nil
	}
	cloned := make([][]byte, len(values))
	for i := range values {
		cloned[i] = append([]byte(nil), values[i]...)
	}
	return cloned
}

func validatePinnedPeer(state tls.ConnectionState, expectedDeviceID string, at time.Time) error {
	if state.Version != tls.VersionTLS13 {
		return fmt.Errorf("%w: negotiated tls version is not TLS 1.3", ErrTLSPeerCertificate)
	}
	if len(state.PeerCertificates) == 0 {
		return fmt.Errorf("%w: peer certificate is required", ErrTLSPeerCertificate)
	}
	leaf := state.PeerCertificates[0]
	if err := validateCertificateTime(leaf, at); err != nil {
		return err
	}
	deviceID, err := LeafSPKIDeviceID(leaf)
	if err != nil {
		return err
	}
	if deviceID != expectedDeviceID {
		return fmt.Errorf("%w: peer spki %q does not match pinned device id %q", ErrTLSPeerMismatch, deviceID, expectedDeviceID)
	}
	return nil
}

func validateCertificateTime(cert *x509.Certificate, at time.Time) error {
	if cert == nil {
		return fmt.Errorf("%w: certificate is nil", ErrTLSPeerCertificate)
	}
	if at.Before(cert.NotBefore) {
		return fmt.Errorf("%w: certificate is not valid before %s", ErrTLSPeerCertificate, cert.NotBefore.UTC().Format(time.RFC3339))
	}
	if !at.Before(cert.NotAfter) {
		return fmt.Errorf("%w: certificate expired at %s", ErrTLSPeerCertificate, cert.NotAfter.UTC().Format(time.RFC3339))
	}
	return nil
}

func nowFunc(fn func() time.Time) func() time.Time {
	if fn != nil {
		return fn
	}
	return time.Now
}

func now(fn func() time.Time) time.Time {
	return nowFunc(fn)()
}
