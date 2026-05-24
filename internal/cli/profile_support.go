package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
)

func preflightNewControlArtifact(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("control artifact %q exists as a symlink", path)
		}
		return fmt.Errorf("control artifact %q already exists", path)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func preflightProfileWrite(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("profile path is required")
	}
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("profile directory %q is a symlink", parent)
	}
	if !info.IsDir() {
		return fmt.Errorf("profile directory %q is not a directory", parent)
	}
	if existing, err := os.Lstat(path); err == nil {
		if existing.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("profile file %q is a symlink", path)
		}
		if existing.IsDir() {
			return fmt.Errorf("profile file %q is a directory", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temp, err := os.CreateTemp(parent, ".profile-preflight-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	if closeErr := temp.Close(); closeErr != nil {
		_ = os.Remove(name)
		return closeErr
	}
	return os.Remove(name)
}

func pairingProfileSnapshot(p profile.Profile, receiptID string, capturedAt time.Time) (control.ProfileSnapshot, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return control.ProfileSnapshot{}, err
	}
	return control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + receiptID,
		ProfileID:  p.ProfileID,
		CapturedAt: capturedAt.UTC().Format(time.RFC3339Nano),
		Profile:    payload,
	}, nil
}

func targetDirFromProfile(p profile.Profile) (string, error) {
	if strings.TrimSpace(p.Target.LocalPath) != "" {
		return p.Target.LocalPath, nil
	}
	return "", fmt.Errorf("target.local_path is required; run profile set-target to persist the trusted target path")
}

func readProfileFilePayload(path string) (profile.Profile, []byte, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return profile.Profile{}, nil, err
	}
	p, err := profile.Read(bytes.NewReader(payload))
	if err != nil {
		return profile.Profile{}, nil, err
	}
	return p, payload, nil
}

func targetPairingReceiptPath(targetDir string, receipt control.PairingReceipt) (string, error) {
	return control.Path(targetDir, control.ArtifactPairingReceipt, receipt.ID)
}

func preflightTargetPairingReceiptImport(targetDir string, receipt control.PairingReceipt) (string, error) {
	receiptPath, err := targetPairingReceiptPath(targetDir, receipt)
	if err != nil {
		return "", err
	}
	if err := pathguard.EnsureDirectory(targetDir, filepath.Dir(receiptPath)); err != nil {
		return "", err
	}
	if existing, err := control.ReadFileNoSymlinkUnderRoot[control.PairingReceipt](targetDir, receiptPath); err == nil {
		if existing != receipt {
			return "", fmt.Errorf("pairing receipt %q already exists with different content", receipt.ID)
		}
		return receiptPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := preflightNewControlArtifact(receiptPath); err != nil {
		return "", err
	}
	return receiptPath, nil
}

func writeTargetPairingReceipt(targetDir string, receipt control.PairingReceipt) error {
	receiptPath, err := preflightTargetPairingReceiptImport(targetDir, receipt)
	if err != nil {
		return err
	}
	if existing, err := control.ReadFileNoSymlinkUnderRoot[control.PairingReceipt](targetDir, receiptPath); err == nil {
		if existing == receipt {
			return nil
		}
		return fmt.Errorf("pairing receipt %q already exists with different content", receipt.ID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return control.WriteNewFile(receiptPath, receipt)
}
