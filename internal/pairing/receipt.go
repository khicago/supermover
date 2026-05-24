package pairing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
	"github.com/khicago/supermover/internal/transport"
)

type TrustedPairing struct {
	Receipt        control.PairingReceipt
	TargetDeviceID string
}

func LocalReceiptID(p profile.Profile, bootstrap Bootstrap) string {
	sum := sha256.Sum256([]byte(protocol.Version + "\n" + p.ProfileID + "\n" + p.Target.TargetID + "\n" + bootstrap.TargetDeviceID + "\n" + bootstrap.ChallengeID))
	return "pair-" + hex.EncodeToString(sum[:8])
}

func BuildLocalReceipt(p profile.Profile, bootstrap Bootstrap, sourceDeviceID string, method transport.PairingMethod, now time.Time) control.PairingReceipt {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	return control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               LocalReceiptID(p, bootstrap),
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   sourceDeviceID,
		TargetDeviceID:   bootstrap.TargetDeviceID,
		DevicePublicKey:  bootstrap.TargetDeviceID,
		Method:           string(method),
		VerifiedAt:       now.Format(time.RFC3339Nano),
		VerificationHash: bootstrap.VerificationHash,
		ProtocolVersion:  protocol.Version,
	}
}

func TrustedFromProfilePins(p profile.Profile) (TrustedPairing, error) {
	if err := p.Validate(); err != nil {
		return TrustedPairing{}, fmt.Errorf("%w: %v", ErrPairingProfileInvalid, err)
	}
	if strings.TrimSpace(p.Target.PairingReceiptID) == "" || strings.TrimSpace(p.Target.DevicePublicKey) == "" || strings.TrimSpace(p.Target.PairedAt) == "" {
		return TrustedPairing{}, ErrUnpairedProfile
	}
	receipt := control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               p.Target.PairingReceiptID,
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		TargetDeviceID:   p.Target.DevicePublicKey,
		DevicePublicKey:  p.Target.DevicePublicKey,
		VerifiedAt:       p.Target.PairedAt,
		Method:           string(transport.PairingMethodSAS),
		VerificationHash: "profile_pins_only",
		ProtocolVersion:  protocol.Version,
	}
	return TrustedPairing{
		Receipt:        receipt,
		TargetDeviceID: p.Target.DevicePublicKey,
	}, nil
}

func ReadReceiptFile(path string) (control.PairingReceipt, error) {
	file, err := os.Open(path)
	if err != nil {
		return control.PairingReceipt{}, err
	}
	defer file.Close()
	return DecodeReceipt(file)
}

func DecodeReceipt(r io.Reader) (control.PairingReceipt, error) {
	var receipt control.PairingReceipt
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return control.PairingReceipt{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("unexpected trailing JSON document")
		}
		return control.PairingReceipt{}, err
	}
	if err := receipt.Validate(); err != nil {
		return control.PairingReceipt{}, err
	}
	return receipt, nil
}

func ReceiptExportPath(dir string, receipt control.PairingReceipt) string {
	return filepath.Join(filepath.Clean(dir), receipt.ID+".json")
}
