package discovery

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClassifyHintsDeduplicatesAndClassifiesAmbiguity(t *testing.T) {
	now := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	ad1 := NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "abcdef0123456789", []string{"pair"})
	ad2 := NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "fedcba9876543210", []string{"pair"})
	older := mustAddressHint(t, "127.0.0.1:9000", ad1, now.Add(-time.Second), time.Minute)
	newer := mustAddressHint(t, "127.0.0.1:9000", ad1, now, time.Minute)
	ambiguous := mustAddressHint(t, "127.0.0.1:9000", ad2, now, time.Minute)
	sameNonceOtherAddress := mustAddressHint(t, "127.0.0.2:9000", ad1, now, time.Minute)

	got, err := ClassifyHints([]AddressHint{older, newer, ambiguous, sameNonceOtherAddress}, now)
	if err != nil {
		t.Fatalf("ClassifyHints() error = %v, want nil", err)
	}
	if len(got) != 3 {
		t.Fatalf("ClassifyHints() len = %d, want 3: %#v", len(got), got)
	}
	if got[0].Hint.Advertisement.EphemeralNonce != "abcdef0123456789" || got[0].DuplicateCount != 2 || got[0].Class != CandidateClassAmbiguous {
		t.Fatalf("first candidate = %+v, want deduplicated ambiguous newer candidate", got[0])
	}
	for _, candidate := range got {
		if candidate.Class != CandidateClassAmbiguous {
			t.Fatalf("candidate = %+v, want ambiguity classified for address/nonce conflicts", candidate)
		}
		if len(candidate.AmbiguityReasons) == 0 {
			t.Fatalf("candidate = %+v, want ambiguity reason", candidate)
		}
	}
}

func TestClassifyHintsMarksExactDuplicates(t *testing.T) {
	now := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	ad := NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "abcdef0123456789", []string{"pair"})
	older := mustAddressHint(t, "127.0.0.1:9000", ad, now.Add(-time.Second), time.Minute)
	newer := mustAddressHint(t, "127.0.0.1:9000", ad, now, time.Minute)

	got, err := ClassifyHints([]AddressHint{older, newer}, now)
	if err != nil {
		t.Fatalf("ClassifyHints() error = %v, want nil", err)
	}
	if len(got) != 1 {
		t.Fatalf("ClassifyHints() len = %d, want 1", len(got))
	}
	if got[0].Class != CandidateClassDuplicate || got[0].DuplicateCount != 2 {
		t.Fatalf("ClassifyHints() = %+v, want duplicate class with count 2", got[0])
	}
	if !got[0].Hint.SeenAt.Equal(newer.SeenAt) {
		t.Fatalf("ClassifyHints() kept SeenAt = %s, want newer %s", got[0].Hint.SeenAt, newer.SeenAt)
	}
}

func TestClassifyHintsSortsDeterministicallyAcrossServiceProtocolAndNonce(t *testing.T) {
	now := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	hints := []AddressHint{
		mustAddressHint(t, "127.0.0.1:9000", NewLowInfoAdvertisement("_supermover._tcp", "supermover/2", "bbbbbbbb", []string{"pair"}), now, time.Minute),
		mustAddressHint(t, "127.0.0.1:9000", NewLowInfoAdvertisement("_other._tcp", "supermover/1", "cccccccc", []string{"pair"}), now, time.Minute),
		mustAddressHint(t, "127.0.0.1:9000", NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "bbbbbbbb", []string{"pair"}), now, time.Minute),
		mustAddressHint(t, "127.0.0.1:9000", NewLowInfoAdvertisement("_supermover._tcp", "supermover/1", "aaaaaaaa", []string{"pair"}), now, time.Minute),
	}

	got, err := ClassifyHints(hints, now)
	if err != nil {
		t.Fatalf("ClassifyHints() error = %v, want nil", err)
	}
	gotOrder := make([]string, 0, len(got))
	for _, candidate := range got {
		ad := candidate.Hint.Advertisement
		gotOrder = append(gotOrder, ad.ServiceType+"|"+ad.ProtocolVersion+"|"+ad.EphemeralNonce)
	}
	wantOrder := []string{
		"_other._tcp|supermover/1|cccccccc",
		"_supermover._tcp|supermover/1|aaaaaaaa",
		"_supermover._tcp|supermover/1|bbbbbbbb",
		"_supermover._tcp|supermover/2|bbbbbbbb",
	}
	if strings.Join(gotOrder, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("ClassifyHints() order = %#v, want %#v", gotOrder, wantOrder)
	}
}

func TestBrowseWithTimeoutUsesContextDeadline(t *testing.T) {
	start := time.Now()
	got, err := BrowseWithTimeout(context.Background(), EmptySource{}, start, time.Millisecond)
	if err != nil {
		t.Fatalf("BrowseWithTimeout(empty) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("BrowseWithTimeout(empty) = %#v, want no candidates", got)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("BrowseWithTimeout(empty) took too long")
	}
}
