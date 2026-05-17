package pairing

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
)

func TestValidateProfileTrustAcceptsPinnedReceipt(t *testing.T) {
	target := t.TempDir()
	p := validPairedProfile(target)
	writePairingReceipt(t, target, control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               p.Target.PairingReceiptID,
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   "sha256:abcdef0123456789",
		TargetDeviceID:   p.Target.DevicePublicKey,
		DevicePublicKey:  p.Target.DevicePublicKey,
		Method:           "sas",
		VerifiedAt:       p.Target.PairedAt,
		VerificationHash: "sha256:abcdef0123456789",
		ProtocolVersion:  "supermover/1",
	})

	got, err := ValidateProfileTrust(p)
	if err != nil {
		t.Fatalf("ValidateProfileTrust() error = %v, want nil", err)
	}
	if got.Receipt.ID != p.Target.PairingReceiptID || got.TargetDeviceID != p.Target.DevicePublicKey {
		t.Fatalf("ValidateProfileTrust() = %+v, want pinned receipt and target device", got)
	}
}

func TestValidateProfileTrustRejectsUnpairedProfile(t *testing.T) {
	p := profile.NewDefault("profile-local", "Profile", t.TempDir(), t.TempDir())

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrUnpairedProfile) {
		t.Fatalf("ValidateProfileTrust(unpaired) error = %v, want ErrUnpairedProfile", err)
	}
}

func TestValidateProfileTrustRejectsInvalidProfileSeparatelyFromReceipt(t *testing.T) {
	p := validPairedProfile(t.TempDir())
	p.Target.DevicePublicKey = "not-a-device-id"

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrPairingProfileInvalid) {
		t.Fatalf("ValidateProfileTrust(invalid profile) error = %v, want ErrPairingProfileInvalid", err)
	}
	if errors.Is(err, ErrPairingReceiptInvalid) {
		t.Fatalf("ValidateProfileTrust(invalid profile) error = %v, must not report receipt invalid", err)
	}
}

func TestValidateProfileTrustRejectsMissingReceipt(t *testing.T) {
	p := validPairedProfile(t.TempDir())

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrPairingReceiptMissing) {
		t.Fatalf("ValidateProfileTrust(missing receipt) error = %v, want ErrPairingReceiptMissing", err)
	}
}

func TestValidateProfileTrustRejectsMissingLocalTargetPath(t *testing.T) {
	p := validPairedProfile(t.TempDir())
	p.Target.LocalPath = ""

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrPairingTargetMissing) {
		t.Fatalf("ValidateProfileTrust(missing local path) error = %v, want ErrPairingTargetMissing", err)
	}
}

func TestValidateProfileTrustRejectsReceiptProfileMismatch(t *testing.T) {
	target := t.TempDir()
	p := validPairedProfile(target)
	receipt := validPairingReceipt(p)
	receipt.ProfileID = "other-profile"
	writePairingReceipt(t, target, receipt)

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrPairingMismatch) || !strings.Contains(err.Error(), "profile_id") {
		t.Fatalf("ValidateProfileTrust(profile mismatch) error = %v, want profile_id mismatch", err)
	}
}

func TestValidateProfileTrustRejectsReceiptTargetMismatch(t *testing.T) {
	target := t.TempDir()
	p := validPairedProfile(target)
	receipt := validPairingReceipt(p)
	receipt.TargetID = "other-target"
	writePairingReceipt(t, target, receipt)

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrPairingMismatch) || !strings.Contains(err.Error(), "target_id") {
		t.Fatalf("ValidateProfileTrust(target mismatch) error = %v, want target_id mismatch", err)
	}
}

func TestValidateProfileTrustRejectsPinnedKeyMismatch(t *testing.T) {
	target := t.TempDir()
	p := validPairedProfile(target)
	receipt := validPairingReceipt(p)
	receipt.DevicePublicKey = "sha256:fedcba9876543210"
	receipt.TargetDeviceID = "sha256:fedcba9876543210"
	writePairingReceipt(t, target, receipt)

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrPairingMismatch) || !strings.Contains(err.Error(), "device_public_key") {
		t.Fatalf("ValidateProfileTrust(key mismatch) error = %v, want device_public_key mismatch", err)
	}
}

func TestValidateProfileTrustRejectsReceiptIDMismatch(t *testing.T) {
	target := t.TempDir()
	p := validPairedProfile(target)
	receipt := validPairingReceipt(p)
	receipt.ID = "other-pairing"
	writePairingReceiptAtID(t, target, p.Target.PairingReceiptID, receipt)

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrPairingMismatch) || !strings.Contains(err.Error(), "receipt id") {
		t.Fatalf("ValidateProfileTrust(receipt id mismatch) error = %v, want receipt id mismatch", err)
	}
}

func TestValidateProfileTrustRejectsMalformedReceipt(t *testing.T) {
	target := t.TempDir()
	p := validPairedProfile(target)
	writeRawPairingReceipt(t, target, p.Target.PairingReceiptID, `{"version":1,"id":"pairing-1","profile_id":"profile-local","target_id":"target-local","source_device_id":"sha256:abcdef0123456789","target_device_id":"sha256:0123456789abcdef","device_public_key":"sha256:0123456789abcdef","method":"sas","verified_at":"2026-05-16T00:00:00Z","protocol_version":"supermover/1"}`)

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrPairingReceiptInvalid) {
		t.Fatalf("ValidateProfileTrust(malformed receipt) error = %v, want ErrPairingReceiptInvalid", err)
	}
}

func TestValidateProfileTrustRejectsSymlinkedControlPlane(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	p := validPairedProfile(target)
	writePairingReceipt(t, outside, validPairingReceipt(p))
	if err := os.Symlink(filepath.Join(outside, control.DirName), filepath.Join(target, control.DirName)); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrPairingReceiptInvalid) {
		t.Fatalf("ValidateProfileTrust(symlinked control plane) error = %v, want ErrPairingReceiptInvalid", err)
	}
}

func TestValidateProfileTrustRejectsSymlinkedPairingsDirectory(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	p := validPairedProfile(target)
	writePairingReceipt(t, outside, validPairingReceipt(p))
	controlDir := filepath.Join(target, control.DirName)
	if err := os.Mkdir(controlDir, 0o755); err != nil {
		t.Fatalf("os.Mkdir(control dir) error = %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, control.DirName, "pairings"), filepath.Join(controlDir, "pairings")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrPairingReceiptInvalid) {
		t.Fatalf("ValidateProfileTrust(symlinked pairings dir) error = %v, want ErrPairingReceiptInvalid", err)
	}
}

func TestValidateProfileTrustRejectsSymlinkedPairingReceipt(t *testing.T) {
	target := t.TempDir()
	outside := t.TempDir()
	p := validPairedProfile(target)
	writePairingReceipt(t, outside, validPairingReceipt(p))
	pairingsDir := filepath.Join(target, control.DirName, "pairings")
	if err := os.MkdirAll(pairingsDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(pairings dir) error = %v", err)
	}
	receiptPath := filepath.Join(pairingsDir, p.Target.PairingReceiptID+".json")
	outsideReceipt := filepath.Join(outside, control.DirName, "pairings", p.Target.PairingReceiptID+".json")
	if err := os.Symlink(outsideReceipt, receiptPath); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	_, err := ValidateProfileTrust(p)
	if !errors.Is(err, ErrPairingReceiptInvalid) {
		t.Fatalf("ValidateProfileTrust(symlinked receipt) error = %v, want ErrPairingReceiptInvalid", err)
	}
}

func validPairedProfile(target string) profile.Profile {
	p := profile.NewDefault("profile-local", "Profile", tTempSource(target), target)
	p.Target.TargetID = "target-local"
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	return p
}

func tTempSource(target string) string {
	return target + "-source"
}

func validPairingReceipt(p profile.Profile) control.PairingReceipt {
	return control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               p.Target.PairingReceiptID,
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   "sha256:abcdef0123456789",
		TargetDeviceID:   p.Target.DevicePublicKey,
		DevicePublicKey:  p.Target.DevicePublicKey,
		Method:           "sas",
		VerifiedAt:       p.Target.PairedAt,
		VerificationHash: "sha256:abcdef0123456789",
		ProtocolVersion:  "supermover/1",
	}
}

func writePairingReceipt(t *testing.T, target string, receipt control.PairingReceipt) {
	t.Helper()
	writePairingReceiptAtID(t, target, receipt.ID, receipt)
}

func writePairingReceiptAtID(t *testing.T, target string, id string, receipt control.PairingReceipt) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactPairingReceipt, id)
	if err != nil {
		t.Fatalf("Path(pairing) error = %v", err)
	}
	if err := control.WriteFile(path, receipt); err != nil {
		t.Fatalf("WriteFile(pairing) error = %v", err)
	}
}

func writeRawPairingReceipt(t *testing.T, target string, id string, payload string) {
	t.Helper()
	path, err := control.Path(target, control.ArtifactPairingReceipt, id)
	if err != nil {
		t.Fatalf("Path(pairing) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(pairing dir) error = %v", err)
	}
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("os.WriteFile(pairing) error = %v", err)
	}
}
