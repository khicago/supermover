package pairing

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/transport"
)

const (
	DefaultChallengeTTL    = 5 * time.Minute
	VerificationCodeHeader = "X-Supermover-Verification-Code"
	verificationTokenBytes = 20
)

var (
	ErrInvalidBootstrap = errors.New("invalid pairing bootstrap")
	ErrVerificationCode = errors.New("pairing verification failed")
)

type Bootstrap struct {
	ProtocolVersion  string    `json:"protocol_version"`
	Status           string    `json:"status"`
	TargetDeviceID   string    `json:"target_device_id"`
	ChallengeID      string    `json:"challenge_id"`
	VerificationHash string    `json:"verification_hash"`
	ExpiresAt        time.Time `json:"expires_at"`
	Trusted          bool      `json:"trusted"`
	TransferEnabled  bool      `json:"transfer_enabled"`
}

func SourceDeviceID(p profile.Profile) (string, error) {
	return fingerprint("source", p.ProfileID)
}

func TargetDeviceID(p profile.Profile) (string, error) {
	if strings.TrimSpace(p.Target.DevicePublicKey) != "" {
		if err := transport.DeviceID(p.Target.DevicePublicKey).Validate(); err != nil {
			return "", err
		}
		return p.Target.DevicePublicKey, nil
	}
	return fingerprint("target", p.Target.TargetID)
}

func NewVerificationCode() (string, error) {
	var bytes [verificationTokenBytes]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func NewChallengeID() (string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return "pair-" + hex.EncodeToString(bytes[:]), nil
}

func VerificationHash(targetDeviceID, challengeID, code string) string {
	sum := sha256.Sum256([]byte(protocol.Version + "\n" + targetDeviceID + "\n" + challengeID + "\n" + strings.TrimSpace(code)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ValidateBootstrap(info Bootstrap, expectedTargetDeviceID string, code string, now time.Time) error {
	if strings.TrimSpace(info.ProtocolVersion) != protocol.Version {
		return fmt.Errorf("%w: protocol_version must be %q", ErrInvalidBootstrap, protocol.Version)
	}
	if info.Status != "pairing_ready" {
		return fmt.Errorf("%w: status must be pairing_ready", ErrInvalidBootstrap)
	}
	if err := transport.DeviceID(expectedTargetDeviceID).Validate(); err != nil {
		return fmt.Errorf("%w: expected target_device_id: %v", ErrInvalidBootstrap, err)
	}
	if err := transport.DeviceID(info.TargetDeviceID).Validate(); err != nil {
		return fmt.Errorf("%w: target_device_id: %v", ErrInvalidBootstrap, err)
	}
	if info.TargetDeviceID != expectedTargetDeviceID {
		return fmt.Errorf("%w: target_device_id does not match profile target identity", ErrInvalidBootstrap)
	}
	if strings.TrimSpace(info.ChallengeID) == "" {
		return fmt.Errorf("%w: challenge_id is required", ErrInvalidBootstrap)
	}
	if strings.TrimSpace(info.VerificationHash) == "" {
		return fmt.Errorf("%w: verification_hash is required", ErrInvalidBootstrap)
	}
	if info.Trusted {
		return fmt.Errorf("%w: unauthenticated pairing endpoint must not claim trust", ErrInvalidBootstrap)
	}
	if info.TransferEnabled {
		return fmt.Errorf("%w: pairing endpoint must not enable transfer", ErrInvalidBootstrap)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if info.ExpiresAt.IsZero() || !info.ExpiresAt.After(now) {
		return fmt.Errorf("%w: challenge expired", ErrInvalidBootstrap)
	}
	if got := VerificationHash(info.TargetDeviceID, info.ChallengeID, code); got != info.VerificationHash {
		return ErrVerificationCode
	}
	return nil
}

func fingerprint(kind string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("cannot derive %s device id from empty value", kind)
	}
	sum := sha256.Sum256([]byte("supermover/" + kind + "-device/v1\x00" + value))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
