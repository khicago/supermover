package discovery

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseTXT(t *testing.T) {
	txt := map[string]string{
		"svc":   "_supermover._tcp",
		"proto": "supermover/1",
		"nonce": "abcdef0123456789",
		"caps":  "l2,pair",
	}

	got, err := ParseTXT(txt)
	if err != nil {
		t.Fatalf("ParseTXT() error = %v, want nil", err)
	}
	if got.ServiceType != "_supermover._tcp" || got.ProtocolVersion != "supermover/1" || got.EphemeralNonce != "abcdef0123456789" {
		t.Fatalf("ParseTXT() = %+v, want canonical low-info fields", got)
	}
	if len(got.CapabilityFlags) != 2 || got.CapabilityFlags[0] != "l2" || got.CapabilityFlags[1] != "pair" {
		t.Fatalf("ParseTXT() capabilities = %#v, want sorted l2,pair", got.CapabilityFlags)
	}
}

func TestParseTXTRejectsMalformedOrHighInfoFields(t *testing.T) {
	valid := map[string]string{"svc": "_supermover._tcp", "proto": "supermover/1", "nonce": "abcdef0123456789", "caps": "pair"}
	tests := []struct {
		name string
		txt  map[string]string
	}{
		{name: "missing service", txt: withoutTXT(valid, "svc")},
		{name: "missing protocol", txt: withoutTXT(valid, "proto")},
		{name: "missing nonce", txt: withoutTXT(valid, "nonce")},
		{name: "missing caps", txt: withoutTXT(valid, "caps")},
		{name: "unknown hostname field", txt: withTXTValue(valid, "hostname", "alice-mbp.local")},
		{name: "profile in canonical value", txt: withTXTValue(valid, "caps", "profile-default")},
		{name: "trailing empty cap", txt: withTXTValue(valid, "caps", "pair,")},
		{name: "leading empty cap", txt: withTXTValue(valid, "caps", ",pair")},
		{name: "only empty caps", txt: withTXTValue(valid, "caps", ",")},
		{name: "spaced caps", txt: withTXTValue(valid, "caps", "pair, l2")},
		{name: "bad nonce", txt: withTXTValue(valid, "nonce", "abc")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTXT(tt.txt)
			if !errors.Is(err, ErrInvalidAdvertisement) {
				t.Fatalf("ParseTXT(%#v) error = %v, want ErrInvalidAdvertisement", tt.txt, err)
			}
		})
	}
}

func TestNewAddressHintRejectsNonIPOrTrustedHints(t *testing.T) {
	now := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)
	ad := NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "abcdef0123456789", []string{"pair"})
	tests := []struct {
		name string
		hint AddressHint
	}{
		{name: "hostname address", hint: AddressHint{Address: "alice-mbp.local:9000", Advertisement: ad, SeenAt: now, ExpiresAt: now.Add(time.Second)}},
		{name: "zero port", hint: AddressHint{Address: "127.0.0.1:0", Advertisement: ad, SeenAt: now, ExpiresAt: now.Add(time.Second)}},
		{name: "port too large", hint: AddressHint{Address: "127.0.0.1:70000", Advertisement: ad, SeenAt: now, ExpiresAt: now.Add(time.Second)}},
		{name: "trusted", hint: AddressHint{Address: "127.0.0.1:9000", Advertisement: ad, SeenAt: now, ExpiresAt: now.Add(time.Second), Trusted: true}},
		{name: "stale", hint: AddressHint{Address: "127.0.0.1:9000", Advertisement: ad, SeenAt: now, ExpiresAt: now}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.hint.Validate(now); err == nil {
				t.Fatalf("AddressHint.Validate(%+v) error = nil, want invalid hint", tt.hint)
			}
		})
	}
}

func TestFilterHintsDropsStaleAndDeduplicates(t *testing.T) {
	now := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)
	ad := NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "abcdef0123456789", []string{"pair"})
	older := mustAddressHint(t, "127.0.0.1:9000", ad, now.Add(-time.Second), time.Minute)
	newer := mustAddressHint(t, "127.0.0.1:9000", ad, now, time.Minute)
	stale := mustAddressHint(t, "127.0.0.2:9000", ad, now.Add(-2*time.Minute), time.Second)

	got, err := FilterHints([]AddressHint{older, stale, newer}, now)
	if err != nil {
		t.Fatalf("FilterHints() error = %v, want nil", err)
	}
	if len(got) != 1 {
		t.Fatalf("FilterHints() len = %d, want 1: %#v", len(got), got)
	}
	if !got[0].SeenAt.Equal(newer.SeenAt) {
		t.Fatalf("FilterHints() kept SeenAt = %s, want newer %s", got[0].SeenAt, newer.SeenAt)
	}
}

func TestCollectWaitsForContextWhenNoSourceHints(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	got, err := Collect(ctx, EmptySource{}, time.Now())
	if err != nil {
		t.Fatalf("Collect(empty) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("Collect(empty) = %#v, want no hints", got)
	}
}

func TestStaticSourceReturnsExplicitAddressHints(t *testing.T) {
	now := time.Date(2026, 5, 16, 8, 0, 0, 0, time.UTC)
	source := StaticSource{
		Addresses:       []string{"127.0.0.1:9000"},
		ServiceType:     "_supermover._tcp",
		ProtocolVersion: "supermover/1",
		Capabilities:    []string{"pair", "l2"},
		Nonce:           "abcdef0123456789",
		TTL:             time.Minute,
	}

	got, err := Collect(context.Background(), source, now)
	if err != nil {
		t.Fatalf("Collect(static) error = %v, want nil", err)
	}
	if len(got) != 1 {
		t.Fatalf("Collect(static) len = %d, want 1", len(got))
	}
	if got[0].Address != "127.0.0.1:9000" || got[0].Trusted {
		t.Fatalf("Collect(static) = %+v, want untrusted address hint", got[0])
	}
	txt, err := got[0].Advertisement.TXT()
	if err != nil {
		t.Fatalf("Advertisement.TXT() error = %v, want nil", err)
	}
	for _, forbidden := range []string{"profile", "path", "hostname", "target", "device", "file_count"} {
		if strings.Contains(strings.Join(mapValues(txt), ","), forbidden) {
			t.Fatalf("TXT() = %#v, must not contain %q", txt, forbidden)
		}
	}
}

func mustAddressHint(t *testing.T, address string, ad Advertisement, seenAt time.Time, ttl time.Duration) AddressHint {
	t.Helper()
	hint, err := NewAddressHint(address, ad, seenAt, ttl)
	if err != nil {
		t.Fatalf("NewAddressHint(%q) error = %v", address, err)
	}
	return hint
}

func withoutTXT(txt map[string]string, key string) map[string]string {
	copied := copyTXT(txt)
	delete(copied, key)
	return copied
}

func withTXTValue(txt map[string]string, key string, value string) map[string]string {
	copied := copyTXT(txt)
	copied[key] = value
	return copied
}

func copyTXT(txt map[string]string) map[string]string {
	copied := make(map[string]string, len(txt))
	for key, value := range txt {
		copied[key] = value
	}
	return copied
}

func mapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
