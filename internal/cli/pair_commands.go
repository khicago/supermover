package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/pathguard"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/transport"
)

type pairingWriteResult struct {
	Receipt control.PairingReceipt
	Profile profile.Profile
}

func (r Runner) runPair(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("pair", stderr)
	profilePath := fs.String("profile", "", "--profile source profile path")
	target := fs.String("target", "", "--target host:port or http(s) pairing endpoint")
	verificationCode := fs.String("verification-code", "", "--verification-code shown by target serve")
	method := fs.String("method", "sas", "verification method: sas, short_code, qr, or tofu; writes local pairing evidence")
	exportPath := fs.String("receipt-out", "", "--receipt-out optional file path or directory for exported pairing receipt")
	timeout := fs.String("timeout", "5s", "--timeout for pairing bootstrap request")
	if hasHelpFlag(args) {
		fs.SetOutput(stdout)
		fs.Usage()
		return 0
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			fs.Usage()
			return 0
		}
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" || strings.TrimSpace(*target) == "" {
		fmt.Fprintln(stderr, "pair: --profile and --target are required")
		return 2
	}
	if strings.TrimSpace(*verificationCode) == "" {
		fmt.Fprintln(stderr, "pair: --verification-code is required")
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "pair: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	pairingMethod := transport.PairingMethod(*method)
	if err := pairingMethod.Validate(); err != nil {
		fmt.Fprintf(stderr, "pair: unsupported --method %q\n", *method)
		return 2
	}
	timeoutValue, err := time.ParseDuration(*timeout)
	if err != nil || timeoutValue <= 0 {
		fmt.Fprintf(stderr, "pair: invalid --timeout %q\n", *timeout)
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 2
	}
	ctx := r.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutValue)
	defer cancel()
	alreadyPaired := profileHasPairingPins(p)
	expectedTargetDeviceID := ""
	if alreadyPaired {
		existingPairing, err := pairing.ValidateSourceProfileTrust(p)
		if err != nil {
			fmt.Fprintf(stderr, "pair: existing paired profile is invalid: %v\n", err)
			return 2
		}
		expectedTargetDeviceID = existingPairing.TargetDeviceID
	}
	bootstrap, err := fetchPairingBootstrap(ctx, *target, *verificationCode)
	if err != nil {
		if errors.Is(err, pairing.ErrVerificationCode) {
			fmt.Fprintf(stderr, "pair: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
	}
	now := r.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := pairing.ValidateBootstrap(bootstrap, expectedTargetDeviceID, *verificationCode, now); err != nil {
		if errors.Is(err, pairing.ErrVerificationCode) || errors.Is(err, pairing.ErrInvalidBootstrap) {
			fmt.Fprintf(stderr, "pair: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
	}
	if !bootstrap.TransportIdentityBound {
		fmt.Fprintf(stderr, "pair: %v\n", fmt.Errorf("%w: target transport identity is not bound to authenticated transport certificate", pairing.ErrInvalidBootstrap))
		return 2
	}
	sourceDeviceID, err := pairing.SourceTransportDeviceID(p, now)
	if err != nil {
		if alreadyPaired {
			fmt.Fprintf(stdout, "pair: identity already pinned receipt=%s transfer=false\n", p.Target.PairingReceiptID)
			return 0
		}
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 2
	}
	if alreadyPaired {
		fmt.Fprintf(stdout, "pair: identity already pinned receipt=%s transfer=false\n", p.Target.PairingReceiptID)
		return 0
	}
	receipt := pairing.BuildLocalReceipt(p, bootstrap, sourceDeviceID, pairingMethod, now)
	receiptOutputPath, err := resolvedPairingReceiptOutputPath(*profilePath, *exportPath, receipt)
	if err != nil {
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
	}
	if err := preflightProfileWrite(*profilePath); err != nil {
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
	}
	if err := exportPairingReceipt(receiptOutputPath, receipt, true); err != nil {
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
	}
	if err := exportPairingReceipt(receiptOutputPath, receipt, false); err != nil {
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
	}
	result, err := writePairingArtifacts(*profilePath, p, bootstrap, sourceDeviceID, pairingMethod, now, receiptOutputPath)
	if err != nil {
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "pair: pinned target identity receipt=%s transfer=false\n", result.Receipt.ID)
	return 0
}

func fetchPairingBootstrap(ctx context.Context, target string, verificationCode string) (pairing.Bootstrap, error) {
	endpoint, err := pairingEndpointURL(target)
	if err != nil {
		return pairing.Bootstrap{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return pairing.Bootstrap{}, err
	}
	req.Header.Set(pairing.VerificationCodeHeader, strings.TrimSpace(verificationCode))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return pairing.Bootstrap{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return pairing.Bootstrap{}, pairing.ErrVerificationCode
	}
	if resp.StatusCode != http.StatusOK {
		return pairing.Bootstrap{}, fmt.Errorf("pairing endpoint returned HTTP %d", resp.StatusCode)
	}
	var bootstrap pairing.Bootstrap
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxPairingBootstrapBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bootstrap); err != nil {
		return pairing.Bootstrap{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("unexpected trailing JSON document")
		}
		return pairing.Bootstrap{}, err
	}
	return bootstrap, nil
}

func pairingEndpointURL(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", errors.New("target is required")
	}
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		parsed, err := url.Parse(target)
		if err != nil || parsed.Host == "" {
			return "", fmt.Errorf("invalid target %q", target)
		}
		if parsed.Path == "" || parsed.Path == "/" {
			parsed.Path = "/v1/pairing"
		}
		return parsed.String(), nil
	}
	if _, _, err := net.SplitHostPort(target); err != nil {
		return "", fmt.Errorf("invalid target address %q", target)
	}
	return "http://" + target + "/v1/pairing", nil
}

func profileHasPairingPins(p profile.Profile) bool {
	return strings.TrimSpace(p.Target.DevicePublicKey) != "" ||
		strings.TrimSpace(p.Target.PairingReceiptID) != "" ||
		strings.TrimSpace(p.Target.PairedAt) != ""
}

func writePairingArtifacts(profilePath string, p profile.Profile, bootstrap pairing.Bootstrap, sourceDeviceID string, method transport.PairingMethod, now time.Time, receiptPath string) (pairingWriteResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	receipt := pairing.BuildLocalReceipt(p, bootstrap, sourceDeviceID, method, now)
	updated := p
	updated.Target.DevicePublicKey = bootstrap.TargetDeviceID
	updated.Target.PairingReceiptID = receipt.ID
	updated.Target.PairedAt = receipt.VerifiedAt
	updated.Target.LocalPairingReceiptPath = receiptPath
	if err := updated.Validate(); err != nil {
		return pairingWriteResult{}, err
	}
	if err := preflightProfileWrite(profilePath); err != nil {
		return pairingWriteResult{}, err
	}
	if err := updated.Validate(); err != nil {
		return pairingWriteResult{}, err
	}
	if err := profile.WriteFile(profilePath, updated); err != nil {
		return pairingWriteResult{}, err
	}
	return pairingWriteResult{Receipt: receipt, Profile: updated}, nil
}

func exportPairingReceipt(path string, receipt control.PairingReceipt, preflightOnly bool) error {
	if err := pathguard.EnsurePlainDirectory(filepath.Dir(path), filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := preflightProfileWrite(path); err != nil {
		return err
	}
	if preflightOnly {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("%w: %q", control.ErrArtifactExists, path)
		} else if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return control.WriteNewFile(path, receipt)
}

func resolveReceiptExportPath(target string, receipt control.PairingReceipt) (string, error) {
	path := filepath.Clean(target)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return pairing.ReceiptExportPath(path, receipt), nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return path, nil
}

func resolvedPairingReceiptOutputPath(profilePath string, receiptTarget string, receipt control.PairingReceipt) (string, error) {
	if strings.TrimSpace(receiptTarget) != "" {
		return resolveReceiptExportPath(receiptTarget, receipt)
	}
	baseDir := filepath.Join(filepath.Dir(filepath.Clean(profilePath)), ".supermover-pairings")
	return pairing.ReceiptExportPath(baseDir, receipt), nil
}
