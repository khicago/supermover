package transport

import (
	"testing"
	"time"
)

func TestDeviceIDValidate(t *testing.T) {
	tests := []struct {
		name    string
		id      DeviceID
		wantErr bool
	}{
		{name: "sha256 fingerprint", id: "sha256:abcdef0123456789", wantErr: false},
		{name: "hex fingerprint", id: "abcdef0123456789abcdef0123456789", wantErr: false},
		{name: "colon fingerprint", id: "aa:bb:cc:dd", wantErr: false},
		{name: "empty", id: "", wantErr: true},
		{name: "friendly name", id: "alice-laptop", wantErr: true},
		{name: "space", id: "sha256:abc def", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.id.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("DeviceID(%q).Validate() error = %v, want error presence = %t", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestPairingReceiptValidate(t *testing.T) {
	valid := PairingReceipt{
		SourceDeviceID:     "sha256:abcdef0123456789",
		TargetDeviceID:     "sha256:0123456789abcdef",
		ProfileID:          "profile.default",
		Method:             PairingMethodSAS,
		VerifiedAt:         time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC),
		VerificationPhrase: "alpha-bravo",
		ProtocolVersion:    "supermover/1",
	}
	tests := []struct {
		name    string
		receipt PairingReceipt
		wantErr bool
	}{
		{name: "valid", receipt: valid, wantErr: false},
		{name: "invalid method", receipt: withPairingMethod(valid, "sms"), wantErr: true},
		{name: "missing verified time", receipt: withVerifiedAt(valid, time.Time{}), wantErr: true},
		{name: "missing verification proof", receipt: withVerification(valid, "", ""), wantErr: true},
		{name: "same devices", receipt: withTarget(valid, valid.SourceDeviceID), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.receipt.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("PairingReceipt.Validate(%+v) error = %v, want error presence = %t", tt.receipt, err, tt.wantErr)
			}
		})
	}
}

func TestPrivacyPolicyValidate(t *testing.T) {
	tests := []struct {
		name    string
		policy  PrivacyPolicy
		wantErr bool
	}{
		{name: "level 2 default", policy: DefaultPrivacyPolicy(PrivacyLevel2), wantErr: false},
		{name: "level 2 no padding bucket", policy: PrivacyPolicy{Level: PrivacyLevel2, BatchMaxBytes: 1, BatchMaxCount: 1, DiscoveryLowInfo: true}, wantErr: true},
		{name: "level 2 padding disabled", policy: PrivacyPolicy{Level: PrivacyLevel2, PaddingBucket: 1, BatchMaxBytes: 1, BatchMaxCount: 1, DiscoveryLowInfo: true, DisablePadding: true}, wantErr: true},
		{name: "level 2 no batch bytes", policy: PrivacyPolicy{Level: PrivacyLevel2, PaddingBucket: 1, BatchMaxCount: 1, DiscoveryLowInfo: true}, wantErr: true},
		{name: "level 2 batching disabled", policy: PrivacyPolicy{Level: PrivacyLevel2, PaddingBucket: 1, BatchMaxBytes: 1, BatchMaxCount: 1, DiscoveryLowInfo: true, DisableBatching: true}, wantErr: true},
		{name: "level 2 high info discovery", policy: PrivacyPolicy{Level: PrivacyLevel2, PaddingBucket: 1, BatchMaxBytes: 1, BatchMaxCount: 1}, wantErr: true},
		{name: "level 2 no jitter budget", policy: PrivacyPolicy{Level: PrivacyLevel2, PaddingBucket: 1, BatchMaxBytes: 1, BatchMaxCount: 1, DiscoveryLowInfo: true}, wantErr: true},
		{name: "unsupported level", policy: PrivacyPolicy{Level: 9}, wantErr: true},
		{name: "level 1 accepts disabled protections", policy: PrivacyPolicy{Level: PrivacyLevel1}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("PrivacyPolicy.Validate(%+v) error = %v, want error presence = %t", tt.policy, err, tt.wantErr)
			}
		})
	}
}

func withPairingMethod(r PairingReceipt, method PairingMethod) PairingReceipt {
	r.Method = method
	return r
}

func withVerifiedAt(r PairingReceipt, verifiedAt time.Time) PairingReceipt {
	r.VerifiedAt = verifiedAt
	return r
}

func withVerification(r PairingReceipt, phrase, hash string) PairingReceipt {
	r.VerificationPhrase = phrase
	r.VerificationHash = hash
	return r
}

func withTarget(r PairingReceipt, target DeviceID) PairingReceipt {
	r.TargetDeviceID = target
	return r
}
