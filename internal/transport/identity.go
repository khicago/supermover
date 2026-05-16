package transport

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

const (
	maxDeviceIDLen    = 128
	maxProfileIDLen   = 128
	maxProtocolVerLen = 64
)

var (
	ErrInvalidDeviceID        = errors.New("invalid device id")
	ErrInvalidProfileID       = errors.New("invalid profile id")
	ErrInvalidProtocolVersion = errors.New("invalid protocol version")
)

// DeviceID identifies a pinned device identity at the schema layer.
//
// The value is intentionally a string so later transports can choose concrete
// public-key and fingerprint encodings without changing receipt schemas.
type DeviceID string

func (id DeviceID) Validate() error {
	if !validTokenString(string(id), maxDeviceIDLen) {
		return fmt.Errorf("%w: must be 8-%d printable token characters", ErrInvalidDeviceID, maxDeviceIDLen)
	}
	if !looksLikeKeyOrFingerprint(string(id)) {
		return fmt.Errorf("%w: must look like a public key or fingerprint", ErrInvalidDeviceID)
	}
	return nil
}

func ValidateProfileID(id string) error {
	if !validTokenString(id, maxProfileIDLen) {
		return fmt.Errorf("%w: must be 1-%d printable token characters", ErrInvalidProfileID, maxProfileIDLen)
	}
	return nil
}

func ValidateProtocolVersion(version string) error {
	if !validTokenString(version, maxProtocolVerLen) {
		return fmt.Errorf("%w: must be 1-%d printable token characters", ErrInvalidProtocolVersion, maxProtocolVerLen)
	}
	if !strings.Contains(version, "/") && !strings.HasPrefix(version, "v") {
		return fmt.Errorf("%w: must include a protocol name or v-prefixed version", ErrInvalidProtocolVersion)
	}
	return nil
}

func validTokenString(value string, maxLen int) bool {
	if value == "" || len(value) > maxLen || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._:/=-+", r)) {
			return false
		}
	}
	return true
}

func looksLikeKeyOrFingerprint(value string) bool {
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "sha256:") || strings.HasPrefix(lower, "ed25519:") || strings.HasPrefix(lower, "pubkey:") {
		return true
	}
	if strings.Count(value, ":") >= 3 {
		return true
	}
	if len(value) >= 32 {
		for _, r := range value {
			if !unicode.IsDigit(r) && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
				return false
			}
		}
		return true
	}
	return false
}
