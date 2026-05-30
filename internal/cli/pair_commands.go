package cli

import (
	"bytes"
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
	"github.com/khicago/supermover/internal/pairserve"
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
	var existingPairing pairing.TrustState
	if alreadyPaired {
		existingPairing, err = pairing.ValidateSourceProfileTrust(p)
		if err != nil {
			fmt.Fprintf(stderr, "pair: existing paired profile is invalid: %v\n", err)
			return 2
		}
		expectedTargetDeviceID = existingPairing.TargetDeviceID
	}
	now := r.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sourceDeviceID, err := pairing.SourceTransportDeviceID(p, now)
	if err != nil {
		if !alreadyPaired {
			fmt.Fprintf(stderr, "pair: %v\n", err)
			return 2
		}
		sourceDeviceID = strings.TrimSpace(existingPairing.Receipt.SourceDeviceID)
		if sourceDeviceID == "" {
			sourceDeviceID, err = pairing.SourceDeviceID(p)
			if err != nil {
				fmt.Fprintf(stderr, "pair: %v\n", err)
				return 2
			}
		}
	}
	request, err := createPairingRequest(ctx, *target, *verificationCode, p, sourceDeviceID)
	if err != nil {
		if errors.Is(err, pairing.ErrVerificationCode) {
			fmt.Fprintf(stderr, "pair: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
	}
	bootstrap, err := waitPairingApproval(ctx, *target, *verificationCode, request.ID)
	if err != nil {
		if errors.Is(err, pairing.ErrVerificationCode) {
			fmt.Fprintf(stderr, "pair: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "pair: %v\n", err)
		return 1
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
	fmt.Fprintf(stdout, "pair: request approved id=%s pinned target identity receipt=%s transfer=false\n", request.ID, result.Receipt.ID)
	return 0
}

func createPairingRequest(ctx context.Context, target string, verificationCode string, p profile.Profile, sourceDeviceID string) (pairserve.PairingRequestSnapshot, error) {
	endpoint, err := pairingRequestEndpointURL(target)
	if err != nil {
		return pairserve.PairingRequestSnapshot{}, err
	}
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(pairserve.PairingRequestCreate{
		SourceProfileID:   p.ProfileID,
		SourceProfileName: p.Name,
		SourceDeviceID:    sourceDeviceID,
	}); err != nil {
		return pairserve.PairingRequestSnapshot{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return pairserve.PairingRequestSnapshot{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(pairing.VerificationCodeHeader, strings.TrimSpace(verificationCode))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return pairserve.PairingRequestSnapshot{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return pairserve.PairingRequestSnapshot{}, pairing.ErrVerificationCode
	}
	if resp.StatusCode != http.StatusAccepted {
		return pairserve.PairingRequestSnapshot{}, fmt.Errorf("pairing request endpoint returned HTTP %d", resp.StatusCode)
	}
	var response pairserve.PairingRequestResponse
	if err := decodePairingRequestResponse(resp.Body, &response); err != nil {
		return pairserve.PairingRequestSnapshot{}, err
	}
	if response.Request.Status != "pending" || strings.TrimSpace(response.Request.ID) == "" {
		return pairserve.PairingRequestSnapshot{}, errors.New("pairing request response did not enter pending state")
	}
	return response.Request, nil
}

func waitPairingApproval(ctx context.Context, target string, verificationCode string, requestID string) (pairing.Bootstrap, error) {
	endpoint, err := pairingRequestStatusEndpointURL(target, requestID)
	if err != nil {
		return pairing.Bootstrap{}, err
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		bootstrap, status, err := fetchPairingRequestStatus(ctx, endpoint, verificationCode)
		if err != nil {
			return pairing.Bootstrap{}, err
		}
		switch status {
		case "approved":
			if bootstrap == nil {
				return pairing.Bootstrap{}, errors.New("approved pairing request did not include bootstrap")
			}
			return *bootstrap, nil
		case "pending":
			select {
			case <-ctx.Done():
				return pairing.Bootstrap{}, ctx.Err()
			case <-ticker.C:
				continue
			}
		case "rejected":
			return pairing.Bootstrap{}, errors.New("pairing request rejected by target operator")
		case "expired":
			return pairing.Bootstrap{}, errors.New("pairing request expired before target approval")
		default:
			return pairing.Bootstrap{}, fmt.Errorf("pairing request returned unexpected status %q", status)
		}
	}
}

func fetchPairingRequestStatus(ctx context.Context, endpoint string, verificationCode string) (*pairing.Bootstrap, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set(pairing.VerificationCodeHeader, strings.TrimSpace(verificationCode))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return nil, "", pairing.ErrVerificationCode
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("pairing request status endpoint returned HTTP %d", resp.StatusCode)
	}
	var response pairserve.PairingRequestResponse
	if err := decodePairingRequestResponse(resp.Body, &response); err != nil {
		return nil, "", err
	}
	return response.Bootstrap, response.Request.Status, nil
}

func decodePairingRequestResponse(reader io.Reader, response *pairserve.PairingRequestResponse) error {
	decoder := json.NewDecoder(io.LimitReader(reader, maxPairingBootstrapBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(response); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("unexpected trailing JSON document")
		}
		return err
	}
	return nil
}

func pairingRequestEndpointURL(target string) (string, error) {
	base, err := pairingBaseURL(target)
	if err != nil {
		return "", err
	}
	base.Path = "/v1/pairing/requests"
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func pairingRequestStatusEndpointURL(target string, requestID string) (string, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return "", errors.New("pairing request id is required")
	}
	base, err := pairingBaseURL(target)
	if err != nil {
		return "", err
	}
	base.Path = "/v1/pairing/requests/" + url.PathEscape(requestID)
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func pairingBaseURL(target string) (*url.URL, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("target is required")
	}
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		parsed, err := url.Parse(target)
		if err != nil || parsed.Host == "" {
			return nil, fmt.Errorf("invalid target %q", target)
		}
		return parsed, nil
	}
	if _, _, err := net.SplitHostPort(target); err != nil {
		return nil, fmt.Errorf("invalid target address %q", target)
	}
	return &url.URL{Scheme: "http", Host: target}, nil
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
