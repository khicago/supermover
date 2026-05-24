package discovery

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	MaxServiceTypeLen     = 64
	MaxProtocolVersionLen = 64
	MinNonceLen           = 8
	MaxNonceLen           = 128
	MaxCapabilityLen      = 32
)

var allowedCapabilityFlags = map[string]struct{}{
	"l2":   {},
	"pair": {},
}

var (
	ErrInvalidAdvertisement = errors.New("invalid discovery advertisement")
	ErrForbiddenTXTField    = errors.New("forbidden unauthenticated txt field")
	ErrStaleHint            = errors.New("stale discovery hint")
)

type Advertisement struct {
	ServiceType        string            `json:"service_type"`
	ProtocolVersion    string            `json:"protocol_version"`
	EphemeralNonce     string            `json:"ephemeral_nonce"`
	CapabilityFlags    []string          `json:"capability_flags,omitempty"`
	UnauthenticatedTXT map[string]string `json:"unauthenticated_txt,omitempty"`
}

func NewLowInfoAdvertisement(serviceType, protocolVersion, nonce string, capabilities []string) Advertisement {
	return Advertisement{
		ServiceType:     serviceType,
		ProtocolVersion: protocolVersion,
		EphemeralNonce:  nonce,
		CapabilityFlags: append([]string(nil), capabilities...),
	}
}

func (a Advertisement) Validate() error {
	if !validToken(a.ServiceType, MaxServiceTypeLen) {
		return fmt.Errorf("%w: invalid service type", ErrInvalidAdvertisement)
	}
	if !validProtocolVersion(a.ProtocolVersion) {
		return fmt.Errorf("%w: invalid protocol version", ErrInvalidAdvertisement)
	}
	if !validNonce(a.EphemeralNonce) {
		return fmt.Errorf("%w: invalid ephemeral nonce", ErrInvalidAdvertisement)
	}
	for _, flag := range a.CapabilityFlags {
		if !validCapabilityFlag(flag) {
			return fmt.Errorf("%w: invalid capability flag %q", ErrInvalidAdvertisement, flag)
		}
	}
	for key, value := range a.UnauthenticatedTXT {
		if err := ValidateUnauthenticatedTXTField(key, value); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidAdvertisement, err)
		}
	}
	return nil
}

func (a Advertisement) TXT() (map[string]string, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	txt := map[string]string{
		"caps":  strings.Join(sortedCopy(a.CapabilityFlags), ","),
		"nonce": a.EphemeralNonce,
		"proto": a.ProtocolVersion,
		"svc":   a.ServiceType,
	}
	for key, value := range a.UnauthenticatedTXT {
		if _, exists := txt[key]; exists {
			continue
		}
		txt[key] = value
	}
	return txt, nil
}

func ValidateUnauthenticatedTXTField(key, value string) error {
	switch key {
	case "svc", "proto", "nonce", "caps":
	default:
		return fmt.Errorf("%w: key %q is not in low-info allowlist", ErrForbiddenTXTField, key)
	}
	if containsForbiddenInformation(key) || containsForbiddenInformation(value) {
		return fmt.Errorf("%w: %q", ErrForbiddenTXTField, key)
	}
	if !validTXTValue(value) {
		return fmt.Errorf("%w: invalid value for %q", ErrForbiddenTXTField, key)
	}
	if key == "caps" {
		flags, err := parseStrictCaps(value)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrForbiddenTXTField, err)
		}
		for _, flag := range flags {
			if !validCapabilityFlag(flag) {
				return fmt.Errorf("%w: invalid capability flag %q", ErrForbiddenTXTField, flag)
			}
		}
	}
	return nil
}

func parseStrictCaps(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	caps := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || strings.TrimSpace(part) != part {
			return nil, errors.New("malformed caps value")
		}
		caps = append(caps, part)
	}
	return caps, nil
}

func containsForbiddenInformation(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"user", "username", "path", "hostname", "host", "profile",
		"label", "files", "file_count", "count", "friendly", "name",
		"home", "/users/", "/home/", "\\users\\",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func validProtocolVersion(value string) bool {
	return validToken(value, MaxProtocolVersionLen) && (strings.Contains(value, "/") || strings.HasPrefix(value, "v"))
}

func validNonce(value string) bool {
	if len(value) < MinNonceLen || len(value) > MaxNonceLen {
		return false
	}
	return validToken(value, MaxNonceLen)
}

func validCapabilityFlag(value string) bool {
	if !validToken(value, MaxCapabilityLen) || containsForbiddenInformation(value) {
		return false
	}
	_, ok := allowedCapabilityFlags[value]
	return ok
}

func validTXTValue(value string) bool {
	if value == "" || len(value) > 256 || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._:/=+,-", r)) {
			return false
		}
	}
	return true
}

func validToken(value string, maxLen int) bool {
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

func sortedCopy(values []string) []string {
	copied := append([]string(nil), values...)
	sort.Strings(copied)
	return copied
}
