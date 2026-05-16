package transport

import (
	"errors"
	"fmt"
	"time"
)

type PairingMethod string

const (
	PairingMethodSAS       PairingMethod = "sas"
	PairingMethodShortCode PairingMethod = "short_code"
	PairingMethodQR        PairingMethod = "qr"
	PairingMethodTOFU      PairingMethod = "tofu"
)

var (
	ErrInvalidPairingMethod  = errors.New("invalid pairing method")
	ErrInvalidPairingReceipt = errors.New("invalid pairing receipt")
)

type PairingReceipt struct {
	SourceDeviceID     DeviceID      `json:"source_device_id"`
	TargetDeviceID     DeviceID      `json:"target_device_id"`
	ProfileID          string        `json:"profile_id"`
	Method             PairingMethod `json:"method"`
	VerifiedAt         time.Time     `json:"verified_at"`
	VerificationPhrase string        `json:"verification_phrase,omitempty"`
	VerificationHash   string        `json:"verification_hash,omitempty"`
	ProtocolVersion    string        `json:"protocol_version"`
}

func (r PairingReceipt) Validate() error {
	if err := r.SourceDeviceID.Validate(); err != nil {
		return fmt.Errorf("%w: source device id: %v", ErrInvalidPairingReceipt, err)
	}
	if err := r.TargetDeviceID.Validate(); err != nil {
		return fmt.Errorf("%w: target device id: %v", ErrInvalidPairingReceipt, err)
	}
	if r.SourceDeviceID == r.TargetDeviceID {
		return fmt.Errorf("%w: source and target device ids must differ", ErrInvalidPairingReceipt)
	}
	if err := ValidateProfileID(r.ProfileID); err != nil {
		return fmt.Errorf("%w: profile id: %v", ErrInvalidPairingReceipt, err)
	}
	if err := r.Method.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPairingReceipt, err)
	}
	if r.VerifiedAt.IsZero() {
		return fmt.Errorf("%w: verified time is required", ErrInvalidPairingReceipt)
	}
	if r.VerificationPhrase == "" && r.VerificationHash == "" {
		return fmt.Errorf("%w: verification phrase or hash is required", ErrInvalidPairingReceipt)
	}
	if r.VerificationPhrase != "" && !validTokenString(r.VerificationPhrase, 256) {
		return fmt.Errorf("%w: invalid verification phrase", ErrInvalidPairingReceipt)
	}
	if r.VerificationHash != "" && !validTokenString(r.VerificationHash, 256) {
		return fmt.Errorf("%w: invalid verification hash", ErrInvalidPairingReceipt)
	}
	if err := ValidateProtocolVersion(r.ProtocolVersion); err != nil {
		return fmt.Errorf("%w: protocol version: %v", ErrInvalidPairingReceipt, err)
	}
	return nil
}

func (m PairingMethod) Validate() error {
	switch m {
	case PairingMethodSAS, PairingMethodShortCode, PairingMethodQR, PairingMethodTOFU:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidPairingMethod, m)
	}
}
