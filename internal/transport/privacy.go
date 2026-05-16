package transport

import (
	"errors"
	"fmt"
)

type PrivacyLevel int

const (
	PrivacyLevel1 PrivacyLevel = 1
	PrivacyLevel2 PrivacyLevel = 2
	PrivacyLevel3 PrivacyLevel = 3
)

var ErrInvalidPrivacyPolicy = errors.New("invalid privacy policy")

type PrivacyPolicy struct {
	Level            PrivacyLevel `json:"level"`
	PaddingBucket    int          `json:"padding_bucket_bytes,omitempty"`
	BatchMaxBytes    int          `json:"batch_max_bytes,omitempty"`
	BatchMaxCount    int          `json:"batch_max_count,omitempty"`
	JitterBudget     int          `json:"jitter_budget_millis,omitempty"`
	DiscoveryLowInfo bool         `json:"discovery_low_info,omitempty"`
	DisablePadding   bool         `json:"disable_padding,omitempty"`
	DisableBatching  bool         `json:"disable_batching,omitempty"`
}

func (p PrivacyPolicy) Validate() error {
	switch p.Level {
	case PrivacyLevel1, PrivacyLevel2, PrivacyLevel3:
	default:
		return fmt.Errorf("%w: unsupported level %d", ErrInvalidPrivacyPolicy, p.Level)
	}
	if p.PaddingBucket < 0 || p.BatchMaxBytes < 0 || p.BatchMaxCount < 0 || p.JitterBudget < 0 {
		return fmt.Errorf("%w: numeric settings must be non-negative", ErrInvalidPrivacyPolicy)
	}
	if p.Level != PrivacyLevel2 {
		return nil
	}
	if p.DisablePadding || p.PaddingBucket == 0 {
		return fmt.Errorf("%w: level 2 requires record padding", ErrInvalidPrivacyPolicy)
	}
	if p.DisableBatching || p.BatchMaxBytes == 0 || p.BatchMaxCount == 0 {
		return fmt.Errorf("%w: level 2 requires batching", ErrInvalidPrivacyPolicy)
	}
	if !p.DiscoveryLowInfo {
		return fmt.Errorf("%w: level 2 requires low-information discovery", ErrInvalidPrivacyPolicy)
	}
	return nil
}

func DefaultPrivacyPolicy(level PrivacyLevel) PrivacyPolicy {
	p := PrivacyPolicy{Level: level}
	if level == PrivacyLevel2 {
		p.PaddingBucket = 64 * 1024
		p.BatchMaxBytes = 1024 * 1024
		p.BatchMaxCount = 64
		p.JitterBudget = 250
		p.DiscoveryLowInfo = true
	}
	return p
}
