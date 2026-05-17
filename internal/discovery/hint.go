package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

const DefaultHintTTL = 30 * time.Second

type AddressHint struct {
	Address       string        `json:"address"`
	Advertisement Advertisement `json:"advertisement"`
	SeenAt        time.Time     `json:"seen_at"`
	ExpiresAt     time.Time     `json:"expires_at"`
	Trusted       bool          `json:"trusted"`
}

type Source interface {
	Discover(context.Context, time.Time) ([]AddressHint, error)
}

type EmptySource struct{}

func (EmptySource) Discover(ctx context.Context, _ time.Time) ([]AddressHint, error) {
	<-ctx.Done()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
		return nil, nil
	}
	return nil, ctx.Err()
}

type StaticSource struct {
	Addresses       []string
	ServiceType     string
	ProtocolVersion string
	Nonce           string
	Capabilities    []string
	TTL             time.Duration
}

func (s StaticSource) Discover(ctx context.Context, now time.Time) ([]AddressHint, error) {
	select {
	case <-ctx.Done():
		return nil, nil
	default:
	}
	ttl := s.TTL
	if ttl <= 0 {
		ttl = DefaultHintTTL
	}
	ad := NewLowInfoAdvertisement(s.ServiceType, s.ProtocolVersion, s.Nonce, s.Capabilities)
	hints := make([]AddressHint, 0, len(s.Addresses))
	for _, address := range s.Addresses {
		hint, err := NewAddressHint(address, ad, now, ttl)
		if err != nil {
			return nil, err
		}
		hints = append(hints, hint)
	}
	return hints, nil
}

func NewAddressHint(address string, ad Advertisement, seenAt time.Time, ttl time.Duration) (AddressHint, error) {
	if seenAt.IsZero() {
		seenAt = time.Now().UTC()
	}
	if ttl <= 0 {
		ttl = DefaultHintTTL
	}
	hint := AddressHint{
		Address:       strings.TrimSpace(address),
		Advertisement: ad,
		SeenAt:        seenAt.UTC(),
		ExpiresAt:     seenAt.Add(ttl).UTC(),
		Trusted:       false,
	}
	if err := hint.Validate(seenAt); err != nil {
		return AddressHint{}, err
	}
	return hint, nil
}

func (h AddressHint) Validate(now time.Time) error {
	if !validAddress(h.Address) {
		return fmt.Errorf("%w: invalid address hint", ErrInvalidAdvertisement)
	}
	if h.Trusted {
		return fmt.Errorf("%w: discovery hints must not be trusted", ErrInvalidAdvertisement)
	}
	if h.SeenAt.IsZero() || h.ExpiresAt.IsZero() || !h.ExpiresAt.After(h.SeenAt) {
		return fmt.Errorf("%w: invalid hint lifetime", ErrInvalidAdvertisement)
	}
	if !now.IsZero() && !h.ExpiresAt.After(now) {
		return fmt.Errorf("%w: %w", ErrInvalidAdvertisement, ErrStaleHint)
	}
	if err := h.Advertisement.Validate(); err != nil {
		return err
	}
	return nil
}

func Collect(ctx context.Context, source Source, now time.Time) ([]AddressHint, error) {
	if source == nil {
		source = EmptySource{}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	hints, err := source.Discover(ctx, now)
	if err != nil {
		return nil, err
	}
	return FilterHints(hints, now)
}

func FilterHints(hints []AddressHint, now time.Time) ([]AddressHint, error) {
	byKey := map[string]AddressHint{}
	for _, hint := range hints {
		if err := hint.Validate(now); err != nil {
			if errors.Is(err, ErrStaleHint) {
				continue
			}
			return nil, err
		}
		key := hint.Address + "\x00" + hint.Advertisement.ProtocolVersion + "\x00" + hint.Advertisement.EphemeralNonce
		if existing, ok := byKey[key]; !ok || hint.SeenAt.After(existing.SeenAt) {
			byKey[key] = hint
		}
	}
	out := make([]AddressHint, 0, len(byKey))
	for _, hint := range byKey {
		out = append(out, hint)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Address == out[j].Address {
			return out[i].Advertisement.EphemeralNonce < out[j].Advertisement.EphemeralNonce
		}
		return out[i].Address < out[j].Address
	})
	return out, nil
}

func ParseTXT(txt map[string]string) (Advertisement, error) {
	for _, key := range []string{"svc", "proto", "nonce", "caps"} {
		if strings.TrimSpace(txt[key]) == "" {
			return Advertisement{}, fmt.Errorf("%w: missing txt %q", ErrInvalidAdvertisement, key)
		}
	}
	for key, value := range txt {
		if err := ValidateUnauthenticatedTXTField(key, value); err != nil {
			return Advertisement{}, fmt.Errorf("%w: %v", ErrInvalidAdvertisement, err)
		}
	}
	caps, err := parseStrictCaps(txt["caps"])
	if err != nil {
		return Advertisement{}, fmt.Errorf("%w: %v", ErrInvalidAdvertisement, err)
	}
	sort.Strings(caps)
	ad := NewLowInfoAdvertisement(txt["svc"], txt["proto"], txt["nonce"], caps)
	if err := ad.Validate(); err != nil {
		return Advertisement{}, err
	}
	return ad, nil
}

func validAddress(address string) bool {
	host, port, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return false
	}
	if net.ParseIP(host) == nil {
		return false
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return false
		}
	}
	portNumber, err := strconv.Atoi(port)
	return err == nil && portNumber > 0 && portNumber <= 65535
}
