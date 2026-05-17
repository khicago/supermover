package pairing

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
)

var (
	ErrPairingProfileInvalid = errors.New("pairing profile is invalid")
	ErrUnpairedProfile       = errors.New("profile is not paired")
	ErrPairingTargetMissing  = errors.New("paired target local path is missing")
	ErrPairingReceiptMissing = errors.New("pairing receipt is missing")
	ErrPairingReceiptInvalid = errors.New("pairing receipt is invalid")
	ErrPairingMismatch       = errors.New("pairing receipt does not match profile")
)

type TrustState struct {
	Receipt        control.PairingReceipt
	TargetDeviceID string
}

func ValidateProfileTrust(p profile.Profile) (TrustState, error) {
	if err := p.Validate(); err != nil {
		return TrustState{}, fmt.Errorf("%w: %v", ErrPairingProfileInvalid, err)
	}
	if strings.TrimSpace(p.Target.PairingReceiptID) == "" || strings.TrimSpace(p.Target.DevicePublicKey) == "" || strings.TrimSpace(p.Target.PairedAt) == "" {
		return TrustState{}, ErrUnpairedProfile
	}
	if strings.TrimSpace(p.Target.LocalPath) == "" {
		return TrustState{}, ErrPairingTargetMissing
	}
	receiptPath, err := control.Path(p.Target.LocalPath, control.ArtifactPairingReceipt, p.Target.PairingReceiptID)
	if err != nil {
		return TrustState{}, fmt.Errorf("%w: %v", ErrPairingReceiptInvalid, err)
	}
	if err := pathguard.EnsureDirectory(p.Target.LocalPath, filepath.Dir(receiptPath)); err != nil {
		return TrustState{}, fmt.Errorf("%w: pairings directory: %v", ErrPairingReceiptInvalid, err)
	}
	if err := validatePlainReceiptFile(receiptPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return TrustState{}, fmt.Errorf("%w: %s", ErrPairingReceiptMissing, p.Target.PairingReceiptID)
		}
		return TrustState{}, fmt.Errorf("%w: receipt file: %v", ErrPairingReceiptInvalid, err)
	}
	receipt, err := control.ReadFile[control.PairingReceipt](receiptPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return TrustState{}, fmt.Errorf("%w: %s", ErrPairingReceiptMissing, p.Target.PairingReceiptID)
		}
		return TrustState{}, fmt.Errorf("%w: %v", ErrPairingReceiptInvalid, err)
	}
	if receipt.ID != p.Target.PairingReceiptID {
		return TrustState{}, fmt.Errorf("%w: receipt id %q does not match profile pairing_receipt_id %q", ErrPairingMismatch, receipt.ID, p.Target.PairingReceiptID)
	}
	if receipt.ProfileID != p.ProfileID {
		return TrustState{}, fmt.Errorf("%w: receipt profile_id %q does not match profile_id %q", ErrPairingMismatch, receipt.ProfileID, p.ProfileID)
	}
	if receipt.TargetID != p.Target.TargetID {
		return TrustState{}, fmt.Errorf("%w: receipt target_id %q does not match profile target_id %q", ErrPairingMismatch, receipt.TargetID, p.Target.TargetID)
	}
	if receipt.DevicePublicKey != p.Target.DevicePublicKey || receipt.TargetDeviceID != p.Target.DevicePublicKey {
		return TrustState{}, fmt.Errorf("%w: receipt device_public_key/target_device_id does not match pinned profile device_public_key", ErrPairingMismatch)
	}
	if receipt.VerifiedAt != p.Target.PairedAt {
		return TrustState{}, fmt.Errorf("%w: receipt verified_at %q does not match profile paired_at %q", ErrPairingMismatch, receipt.VerifiedAt, p.Target.PairedAt)
	}
	return TrustState{Receipt: receipt, TargetDeviceID: receipt.TargetDeviceID}, nil
}

func validatePlainReceiptFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("receipt file %q is a symlink", path)
	}
	if info.IsDir() {
		return fmt.Errorf("receipt file %q is a directory", path)
	}
	return nil
}
