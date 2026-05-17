package discovery

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

type CandidateClass string

const (
	CandidateClassUnique    CandidateClass = "unique"
	CandidateClassDuplicate CandidateClass = "duplicate"
	CandidateClassAmbiguous CandidateClass = "ambiguous"
)

type Candidate struct {
	Hint             AddressHint    `json:"hint"`
	Class            CandidateClass `json:"class"`
	DuplicateCount   int            `json:"duplicate_count"`
	AmbiguityReasons []string       `json:"ambiguity_reasons,omitempty"`
}

func Browse(ctx context.Context, source Source, now time.Time) ([]Candidate, error) {
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
	return ClassifyHints(hints, now)
}

func BrowseWithTimeout(ctx context.Context, source Source, now time.Time, timeout time.Duration) ([]Candidate, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return Browse(ctx, source, now)
}

func ClassifyHints(hints []AddressHint, now time.Time) ([]Candidate, error) {
	type candidateBucket struct {
		hint        AddressHint
		count       int
		capability  map[string]struct{}
		conflicting bool
	}

	byExact := map[string]*candidateBucket{}
	for _, hint := range hints {
		if err := hint.Validate(now); err != nil {
			if errors.Is(err, ErrStaleHint) {
				continue
			}
			return nil, err
		}
		key := candidateKey(hint)
		bucket := byExact[key]
		if bucket == nil {
			bucket = &candidateBucket{hint: hint, capability: map[string]struct{}{}}
			byExact[key] = bucket
		}
		bucket.count++
		bucket.capability[capabilityKey(hint.Advertisement.CapabilityFlags)] = struct{}{}
		if hint.SeenAt.After(bucket.hint.SeenAt) {
			bucket.hint = hint
		}
		if len(bucket.capability) > 1 {
			bucket.conflicting = true
		}
	}

	addressNonces := map[string]map[string]struct{}{}
	nonceAddresses := map[string]map[string]struct{}{}
	for _, bucket := range byExact {
		hint := bucket.hint
		addressKey := hint.Address + "\x00" + hint.Advertisement.ServiceType + "\x00" + hint.Advertisement.ProtocolVersion
		if addressNonces[addressKey] == nil {
			addressNonces[addressKey] = map[string]struct{}{}
		}
		addressNonces[addressKey][hint.Advertisement.EphemeralNonce] = struct{}{}

		nonceKey := hint.Advertisement.ServiceType + "\x00" + hint.Advertisement.ProtocolVersion + "\x00" + hint.Advertisement.EphemeralNonce
		if nonceAddresses[nonceKey] == nil {
			nonceAddresses[nonceKey] = map[string]struct{}{}
		}
		nonceAddresses[nonceKey][hint.Address] = struct{}{}
	}

	candidates := make([]Candidate, 0, len(byExact))
	for _, bucket := range byExact {
		hint := bucket.hint
		reasons := make([]string, 0, 3)
		addressKey := hint.Address + "\x00" + hint.Advertisement.ServiceType + "\x00" + hint.Advertisement.ProtocolVersion
		if len(addressNonces[addressKey]) > 1 {
			reasons = append(reasons, "same address advertised multiple nonces")
		}
		nonceKey := hint.Advertisement.ServiceType + "\x00" + hint.Advertisement.ProtocolVersion + "\x00" + hint.Advertisement.EphemeralNonce
		if len(nonceAddresses[nonceKey]) > 1 {
			reasons = append(reasons, "same nonce seen from multiple addresses")
		}
		if bucket.conflicting {
			reasons = append(reasons, "same address and nonce advertised conflicting capabilities")
		}
		class := CandidateClassUnique
		if len(reasons) > 0 {
			class = CandidateClassAmbiguous
		} else if bucket.count > 1 {
			class = CandidateClassDuplicate
		}
		candidates = append(candidates, Candidate{
			Hint:             hint,
			Class:            class,
			DuplicateCount:   bucket.count,
			AmbiguityReasons: reasons,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		left := candidates[i].Hint
		right := candidates[j].Hint
		if left.Address != right.Address {
			return left.Address < right.Address
		}
		if left.Advertisement.ServiceType != right.Advertisement.ServiceType {
			return left.Advertisement.ServiceType < right.Advertisement.ServiceType
		}
		if left.Advertisement.ProtocolVersion != right.Advertisement.ProtocolVersion {
			return left.Advertisement.ProtocolVersion < right.Advertisement.ProtocolVersion
		}
		return left.Advertisement.EphemeralNonce < right.Advertisement.EphemeralNonce
	})
	return candidates, nil
}

func candidateKey(hint AddressHint) string {
	return hint.Address + "\x00" +
		hint.Advertisement.ServiceType + "\x00" +
		hint.Advertisement.ProtocolVersion + "\x00" +
		hint.Advertisement.EphemeralNonce
}

func capabilityKey(capabilities []string) string {
	return strings.Join(sortedCopy(capabilities), ",")
}
