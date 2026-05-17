package tlsidentity

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/transport"
)

const validationPeerDeviceID = "sha256:0000000000000000"

func Load(ref profile.TLSIdentityRef) (tls.Certificate, error) {
	if err := ValidateFiles(ref); err != nil {
		return tls.Certificate{}, err
	}
	certificate, err := tls.LoadX509KeyPair(ref.CertificatePath, ref.PrivateKeyPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load key pair: %w", err)
	}
	return certificate, nil
}

func ValidatePinned(ref profile.TLSIdentityRef, expectedDeviceID string, at func() time.Time) error {
	certificate, err := Load(ref)
	if err != nil {
		return err
	}
	_, err = transport.ClientTLSConfig(transport.ClientTLSOptions{
		Certificates:   []tls.Certificate{certificate},
		SourceDeviceID: expectedDeviceID,
		TargetDeviceID: validationPeerDeviceID,
		Time:           at,
	})
	if err != nil {
		return err
	}
	return nil
}

func ValidateFiles(ref profile.TLSIdentityRef) error {
	if err := validateFile("certificate", ref.CertificatePath, false); err != nil {
		return err
	}
	if err := validateFile("private key", ref.PrivateKeyPath, true); err != nil {
		return err
	}
	return nil
}

func validateFile(label string, path string, privateKey bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%s file: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s file %q must not be a symlink", label, path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s file %q must be a regular file", label, path)
	}
	if privateKey && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s file %q must not be readable, writable, or executable by group or others", label, path)
	}
	return nil
}
