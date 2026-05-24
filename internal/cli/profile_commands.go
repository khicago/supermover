package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/report"
	"github.com/khicago/supermover/internal/tlsidentity"
)

func (r Runner) runProfile(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "profile: missing subcommand")
		printProfileUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		printProfileUsage(stdout)
		return 0
	case "init":
		return r.runProfileInit(args[1:], stdout, stderr)
	case "lint":
		return r.runProfileLint(args[1:], stdout, stderr)
	case "set-target":
		return r.runProfileSetTarget(args[1:], stdout, stderr)
	case "set-network":
		return r.runProfileSetNetwork(args[1:], stdout, stderr)
	case "adopt-pairing":
		return r.runProfileAdoptPairing(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "profile: unknown subcommand %q\n", args[0])
		printProfileUsage(stderr)
		return 2
	}
}

func (r Runner) runProfileInit(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("profile init", stderr)
	profilePath := fs.String("profile", "", "profile path to create")
	sourceRoot := fs.String("source", "", "source root to persist in the profile")
	targetRoot := fs.String("target", "", "trusted local target directory to persist")
	targetID := fs.String("target-id", "", "stable target identity to persist")
	profileID := fs.String("id", "profile-local", "profile id to persist")
	name := fs.String("name", "Local profile", "human-readable profile name")
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
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "profile init: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if *profilePath == "" || *sourceRoot == "" || *targetRoot == "" {
		fmt.Fprintln(stderr, "profile init: --profile, --source, and --target are required")
		return 2
	}
	if _, err := os.Stat(*profilePath); err == nil {
		fmt.Fprintf(stderr, "profile init: %s already exists\n", *profilePath)
		return 2
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "profile init: stat %s: %v\n", *profilePath, err)
		return 1
	}
	p := profile.NewDefault(*profileID, *name, *sourceRoot, *targetRoot)
	if strings.TrimSpace(*targetID) != "" {
		p.Target.TargetID = *targetID
	}
	if err := profile.WriteFile(*profilePath, p); err != nil {
		fmt.Fprintf(stderr, "profile init: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote profile %s\n", *profilePath)
	return 0
}

func (r Runner) runProfileLint(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("profile lint", stderr)
	profilePath := fs.String("profile", "", "profile path to lint")
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
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "profile lint: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if *profilePath == "" {
		fmt.Fprintln(stderr, "profile lint: --profile is required")
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "profile lint: %v\n", err)
		return 2
	}
	privacyState := report.PrivacyForProfile(&p)
	fmt.Fprintf(stdout, "profile ok: %s (%d roots)\n", p.ProfileID, len(p.Roots))
	fmt.Fprintf(stdout, "privacy policy=status=%s mode=%s traffic_level=%d claim=%s configured_reductions=%s overhead_status=%s overhead_source=%s overhead_padding_bucket_bytes=%d overhead_batch_max_bytes=%d overhead_batch_max_count=%d overhead_jitter_budget_millis=%d residual_leakage=%s local_push=%s network_transfer=%s\n",
		privacyState.Status,
		privacyState.Mode,
		privacyState.TrafficLevel,
		privacyState.Claim,
		formatStringList(privacyState.ConfiguredReduction),
		privacyState.Overhead.Status,
		privacyState.Overhead.Source,
		privacyState.Overhead.PaddingBucketBytes,
		privacyState.Overhead.BatchMaxBytes,
		privacyState.Overhead.BatchMaxCount,
		privacyState.Overhead.JitterBudgetMillis,
		formatStringList(privacyState.ResidualLeakage),
		privacyState.LocalPush,
		privacyState.NetworkTransfer,
	)
	return 0
}

func (r Runner) runProfileSetTarget(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("profile set-target", stderr)
	profilePath := fs.String("profile", "", "profile path to update")
	targetPath := fs.String("target", "", "trusted local target directory to persist")
	targetID := fs.String("target-id", "", "target identity override")
	name := fs.String("name", "", "human-readable target name override")
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
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "profile set-target: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if *profilePath == "" || *targetPath == "" {
		fmt.Fprintln(stderr, "profile set-target: --profile and --target are required")
		return 2
	}

	p, err := readProfileForSetTarget(*profilePath, *targetID)
	if err != nil {
		fmt.Fprintf(stderr, "profile set-target: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*targetID) != "" && strings.TrimSpace(p.Target.PairingReceiptID) != "" {
		fmt.Fprintln(stderr, "profile set-target: cannot change target-id for a paired profile; re-pair the target to rotate identity")
		return 2
	}
	oldLocalPath := p.Target.LocalPath
	cleanTarget := filepath.Clean(*targetPath)
	p.Target.LocalPath = cleanTarget
	if strings.TrimSpace(*targetID) != "" {
		p.Target.TargetID = *targetID
	}
	if strings.TrimSpace(*name) != "" {
		p.Target.Name = *name
	} else if strings.TrimSpace(p.Target.Name) == "" || p.Target.Name == filepath.Base(filepath.Clean(oldLocalPath)) {
		p.Target.Name = filepath.Base(cleanTarget)
	}
	if err := profile.WriteFile(*profilePath, p); err != nil {
		fmt.Fprintf(stderr, "profile set-target: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "updated profile target %s\n", *profilePath)
	return 0
}

func (r Runner) runProfileSetNetwork(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("profile set-network", stderr)
	profilePath := fs.String("profile", "", "profile path to update")
	receiverURL := fs.String("receiver-url", "", "profile-selected receiver base URL")
	tlsCert := fs.String("tls-cert", "", "absolute path to the local TLS certificate")
	tlsKey := fs.String("tls-key", "", "absolute path to the local TLS private key")
	clearReceiverURL := fs.Bool("clear-receiver-url", false, "clear profile network.receiver_url")
	clearTLSIdentity := fs.Bool("clear-tls-identity", false, "clear profile network.local_tls_identity")
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
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "profile set-network: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "profile set-network: --profile is required")
		return 2
	}
	if !*clearReceiverURL && strings.TrimSpace(*receiverURL) == "" && !*clearTLSIdentity && strings.TrimSpace(*tlsCert) == "" && strings.TrimSpace(*tlsKey) == "" {
		fmt.Fprintln(stderr, "profile set-network: provide at least one network change")
		return 2
	}
	if *clearTLSIdentity && (strings.TrimSpace(*tlsCert) != "" || strings.TrimSpace(*tlsKey) != "") {
		fmt.Fprintln(stderr, "profile set-network: --clear-tls-identity cannot be combined with --tls-cert or --tls-key")
		return 2
	}
	if (strings.TrimSpace(*tlsCert) == "") != (strings.TrimSpace(*tlsKey) == "") {
		fmt.Fprintln(stderr, "profile set-network: --tls-cert and --tls-key must be provided together")
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "profile set-network: %v\n", err)
		return 2
	}
	updated := p
	if updated.Network == nil {
		updated.Network = &profile.NetworkConfig{}
	}
	if *clearReceiverURL {
		updated.Network.ReceiverURL = ""
	} else if strings.TrimSpace(*receiverURL) != "" {
		updated.Network.ReceiverURL = strings.TrimSpace(*receiverURL)
	}
	if *clearTLSIdentity {
		updated.Network.LocalTLSIdentity = profile.TLSIdentityRef{}
	} else if strings.TrimSpace(*tlsCert) != "" && strings.TrimSpace(*tlsKey) != "" {
		updated.Network.LocalTLSIdentity = profile.TLSIdentityRef{
			CertificatePath: strings.TrimSpace(*tlsCert),
			PrivateKeyPath:  strings.TrimSpace(*tlsKey),
		}
	}
	if updated.Network != nil && strings.TrimSpace(updated.Network.ReceiverURL) == "" && !updated.Network.LocalTLSIdentity.Configured() {
		updated.Network = nil
	}
	if err := updated.Validate(); err != nil {
		fmt.Fprintf(stderr, "profile set-network: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if err := preflightProfileWrite(*profilePath); err != nil {
		fmt.Fprintf(stderr, "profile set-network: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if err := profile.WriteFile(*profilePath, updated); err != nil {
		fmt.Fprintf(stderr, "profile set-network: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	receiverValue := "-"
	if updated.Network != nil && strings.TrimSpace(updated.Network.ReceiverURL) != "" {
		receiverValue = updated.Network.ReceiverURL
	}
	tlsValue := "absent"
	if updated.Network != nil && updated.Network.LocalTLSIdentity.Configured() {
		tlsValue = "configured"
	}
	fmt.Fprintf(stdout, "updated profile network %s receiver_url=%s tls_identity=%s\n", *profilePath, receiverValue, tlsValue)
	return 0
}

func (r Runner) runProfileAdoptPairing(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := newFlagSet("profile adopt-pairing", stderr)
	profilePath := fs.String("profile", "", "profile path to update")
	receiptID := fs.String("receipt-id", "", "pairing receipt id under target .supermover/pairings")
	receiptPath := fs.String("receipt-file", "", "pairing receipt file exported from source pair")
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
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "profile adopt-pairing: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if strings.TrimSpace(*profilePath) == "" {
		fmt.Fprintln(stderr, "profile adopt-pairing: --profile is required")
		return 2
	}
	if (strings.TrimSpace(*receiptID) == "") == (strings.TrimSpace(*receiptPath) == "") {
		fmt.Fprintln(stderr, "profile adopt-pairing: exactly one of --receipt-id or --receipt-file is required")
		return 2
	}
	p, err := profile.ReadFile(*profilePath)
	if err != nil {
		fmt.Fprintf(stderr, "profile adopt-pairing: %v\n", err)
		return 2
	}
	receipt, err := readPairingReceiptForAdoption(p, strings.TrimSpace(*receiptID), strings.TrimSpace(*receiptPath))
	if err != nil {
		fmt.Fprintf(stderr, "profile adopt-pairing: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if receipt.ProfileID != p.ProfileID {
		fmt.Fprintf(stderr, "profile adopt-pairing: receipt profile_id %q does not match profile_id %q\n", safeDiagnosticLine(receipt.ProfileID), safeDiagnosticLine(p.ProfileID))
		return 2
	}
	if receipt.TargetID != p.Target.TargetID {
		fmt.Fprintf(stderr, "profile adopt-pairing: receipt target_id %q does not match profile target_id %q\n", safeDiagnosticLine(receipt.TargetID), safeDiagnosticLine(p.Target.TargetID))
		return 2
	}
	if p.Network != nil && p.Network.LocalTLSIdentity.Configured() {
		if err := tlsidentity.ValidatePinned(p.Network.LocalTLSIdentity, receipt.TargetDeviceID, r.nowFunc()); err != nil {
			fmt.Fprintf(stderr, "profile adopt-pairing: validate target local TLS identity files: %s\n", safeDiagnosticLine(err.Error()))
			return 2
		}
	}
	updated := p
	updated.Target.DevicePublicKey = receipt.TargetDeviceID
	updated.Target.PairingReceiptID = receipt.ID
	updated.Target.PairedAt = receipt.VerifiedAt
	if err := updated.Validate(); err != nil {
		fmt.Fprintf(stderr, "profile adopt-pairing: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if err := preflightProfileWrite(*profilePath); err != nil {
		fmt.Fprintf(stderr, "profile adopt-pairing: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	targetDir, err := targetDirFromProfile(p)
	if err != nil {
		fmt.Fprintf(stderr, "profile adopt-pairing: %s\n", safeDiagnosticLine(err.Error()))
		return 2
	}
	if _, err := preflightTargetPairingReceiptImport(targetDir, receipt); err != nil {
		fmt.Fprintf(stderr, "profile adopt-pairing: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if err := writeTargetPairingReceipt(targetDir, receipt); err != nil {
		fmt.Fprintf(stderr, "profile adopt-pairing: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	if err := profile.WriteFile(*profilePath, updated); err != nil {
		fmt.Fprintf(stderr, "profile adopt-pairing: %s\n", safeDiagnosticLine(err.Error()))
		return 1
	}
	fmt.Fprintf(stdout, "adopted pairing receipt=%s profile=%s target_device=%s\n", receipt.ID, updated.ProfileID, updated.Target.DevicePublicKey)
	return 0
}

func readProfileForSetTarget(path string, targetID string) (profile.Profile, error) {
	p, err := profile.ReadFile(path)
	if err == nil {
		return p, nil
	}
	if strings.TrimSpace(targetID) == "" {
		return profile.Profile{}, err
	}
	return profile.ReadFileForTargetRepair(path)
}

func readPairingReceiptForAdoption(p profile.Profile, receiptID string, receiptFile string) (control.PairingReceipt, error) {
	if strings.TrimSpace(receiptID) != "" {
		if err := control.ValidateArtifactID(strings.TrimSpace(receiptID)); err != nil {
			return control.PairingReceipt{}, fmt.Errorf("--receipt-id is invalid: %w", err)
		}
		targetDir, err := targetDirFromProfile(p)
		if err != nil {
			return control.PairingReceipt{}, err
		}
		receiptPath, err := control.Path(targetDir, control.ArtifactPairingReceipt, strings.TrimSpace(receiptID))
		if err != nil {
			return control.PairingReceipt{}, err
		}
		return control.ReadFileNoSymlinkUnderRoot[control.PairingReceipt](targetDir, receiptPath)
	}
	if strings.TrimSpace(receiptFile) == "" {
		return control.PairingReceipt{}, errors.New("missing pairing receipt source")
	}
	return pairing.ReadReceiptFile(receiptFile)
}

func printProfileUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  supermover profile init --profile <path> --source <path> --target <path> [--target-id <id>]
  supermover profile lint --profile <path>
  supermover profile set-target --profile <path> --target <path> [--target-id <id>]
  supermover profile set-network --profile <path> [--receiver-url <url>] [--tls-cert <path> --tls-key <path>] [--clear-receiver-url] [--clear-tls-identity]
  supermover profile adopt-pairing --profile <path> (--receipt-id <id> | --receipt-file <path>)`)
}
