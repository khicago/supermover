package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/pairing"
	"github.com/khicago/supermover/internal/pairserve"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
)

func TestPairWritesReceiptAndUpdatesProfilePins(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceCert := newCLITestCertificate(t, "pair-source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "pair-target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	sourceProfile := profile.NewDefault("profile-local", "Source profile", source, target)
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, reserveTCPAddress(t))
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, reserveTCPAddress(t))
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	ready := make(chan pairserve.ReadyInfo, 1)
	done := make(chan int, 1)
	serverRunner := pairingServeRunnerWithAutoApprove(t, serveCtx, now, ready)
	var serveStdout bytes.Buffer
	var serveStderr bytes.Buffer
	go func() {
		done <- serverRunner.Run([]string{"serve", "--profile", targetProfilePath, "--listen", "127.0.0.1:0"}, &serveStdout, &serveStderr)
	}()
	info := waitServePairingReady(t, ready, &serveStderr)
	var pairStdout bytes.Buffer
	var pairStderr bytes.Buffer
	pairRunner := Runner{Now: now.Add(time.Second)}

	got := pairRunner.Run([]string{"pair", "--profile", sourceProfilePath, "--target", info.Address, "--verification-code", info.VerificationCode}, &pairStdout, &pairStderr)

	if got != 0 {
		t.Fatalf("pair exit = %d stderr = %q, want 0", got, pairStderr.String())
	}
	if !strings.Contains(pairStdout.String(), "pinned target identity") || !strings.Contains(pairStdout.String(), "transfer=false") {
		t.Fatalf("pair stdout = %q, want pinned identity with transfer=false", pairStdout.String())
	}
	for _, forbidden := range []string{"encrypted", "sync ready", "trusted=true"} {
		if strings.Contains(pairStdout.String(), forbidden) {
			t.Fatalf("pair stdout = %q, must not contain %q", pairStdout.String(), forbidden)
		}
	}
	updated, err := profile.ReadFile(sourceProfilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", sourceProfilePath, err)
	}
	if updated.Target.DevicePublicKey != info.TargetDeviceID || updated.Target.PairingReceiptID == "" || updated.Target.PairedAt == "" {
		t.Fatalf("updated target = %#v, want pinned target device/receipt/time", updated.Target)
	}
	if strings.TrimSpace(updated.Target.LocalPairingReceiptPath) == "" {
		t.Fatalf("updated target = %#v, want local pairing receipt path", updated.Target)
	}
	if info.TargetDeviceID != certDeviceIDForCLI(t, targetCert) {
		t.Fatalf("pair ready target device id = %q, want target certificate spki %q", info.TargetDeviceID, certDeviceIDForCLI(t, targetCert))
	}
	state, err := pairing.ValidateSourceProfileTrust(updated)
	if err != nil {
		t.Fatalf("ValidateSourceProfileTrust(updated) error = %v, want nil", err)
	}
	if state.TargetDeviceID != info.TargetDeviceID {
		t.Fatalf("ValidateSourceProfileTrust TargetDeviceID = %q, want %q", state.TargetDeviceID, info.TargetDeviceID)
	}
	if state.Receipt.Method != "sas" || state.Receipt.VerificationHash == "" || state.Receipt.SourceDeviceID == "" {
		t.Fatalf("pairing receipt = %#v, want method/hash/source evidence", state.Receipt)
	}
	if state.Receipt.SourceDeviceID != certDeviceIDForCLI(t, sourceCert) {
		t.Fatalf("pairing receipt source_device_id = %q, want source certificate spki %q", state.Receipt.SourceDeviceID, certDeviceIDForCLI(t, sourceCert))
	}
	receipt, err := pairing.ReadReceiptFile(updated.Target.LocalPairingReceiptPath)
	if err != nil {
		t.Fatalf("pairing.ReadReceiptFile(%q) error = %v, want nil", updated.Target.LocalPairingReceiptPath, err)
	}
	if receipt.ID != updated.Target.PairingReceiptID {
		t.Fatalf("local receipt id = %q, want %q", receipt.ID, updated.Target.PairingReceiptID)
	}
	if _, err := os.Lstat(filepath.Join(target, ".supermover")); !os.IsNotExist(err) {
		t.Fatalf("pair default target control plane state error = %v, want not exist", err)
	}
	cancelServe()
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("serve exit after cancel = %d stderr = %q, want 0", got, serveStderr.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not exit after cancel")
	}
}

func TestPairExportsReceiptWhenRequested(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	exportDir := filepath.Join(dir, "receipts")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustMkdir(t, exportDir)
	sourceCert := newCLITestCertificate(t, "pair-source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetCert := newCLITestCertificate(t, "pair-target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	sourceProfile := profile.NewDefault("profile-local", "Source profile", source, target)
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, reserveTCPAddress(t))
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = networkConfigForCLI(t, targetCert, reserveTCPAddress(t))
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	now := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	ready := make(chan pairserve.ReadyInfo, 1)
	done := make(chan int, 1)
	go func() {
		done <- pairingServeRunnerWithAutoApprove(t, serveCtx, now, ready).Run([]string{"serve", "--profile", targetProfilePath, "--listen", "127.0.0.1:0"}, &bytes.Buffer{}, &bytes.Buffer{})
	}()
	info := waitServePairingReady(t, ready, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: now.Add(time.Second)}.Run([]string{
		"pair",
		"--profile", sourceProfilePath,
		"--target", info.Address,
		"--verification-code", info.VerificationCode,
		"--receipt-out", exportDir,
	}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("pair --receipt-out exit = %d stderr = %q, want 0", got, stderr.String())
	}
	updated := mustReadProfile(t, sourceProfilePath)
	receiptPath := filepath.Join(exportDir, updated.Target.PairingReceiptID+".json")
	receipt, err := pairing.ReadReceiptFile(receiptPath)
	if err != nil {
		t.Fatalf("pairing.ReadReceiptFile(%q) error = %v, want nil", receiptPath, err)
	}
	if receipt.ID != updated.Target.PairingReceiptID || receipt.TargetDeviceID != updated.Target.DevicePublicKey {
		t.Fatalf("exported receipt = %#v, want profile pairing pins %#v", receipt, updated.Target)
	}
	if receipt.SourceDeviceID != certDeviceIDForCLI(t, sourceCert) {
		t.Fatalf("exported receipt source_device_id = %q, want %q", receipt.SourceDeviceID, certDeviceIDForCLI(t, sourceCert))
	}
	cancelServe()
	select {
	case got := <-done:
		if got != 0 {
			t.Fatalf("serve exit after cancel = %d, want 0", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not exit after cancel")
	}
}

func TestPairRejectsBootstrapWithoutTransportIdentityBindingWithoutMutatingProfile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	sourceProfilePath := filepath.Join(dir, "source.profile.json")
	targetProfilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceCert := newCLITestCertificate(t, "pair-source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	sourceProfile := profile.NewDefault("profile-local", "Source profile", source, target)
	sourceProfile.Network = networkConfigForCLI(t, sourceCert, reserveTCPAddress(t))
	targetProfile := sourceProfile
	targetProfile.Name = "Target profile"
	targetProfile.Network = nil
	if err := profile.WriteFile(sourceProfilePath, sourceProfile); err != nil {
		t.Fatalf("profile.WriteFile(source) error = %v, want nil", err)
	}
	if err := profile.WriteFile(targetProfilePath, targetProfile); err != nil {
		t.Fatalf("profile.WriteFile(target) error = %v, want nil", err)
	}
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	ready := make(chan pairserve.ReadyInfo, 1)
	done := make(chan int, 1)
	var serveStdout bytes.Buffer
	var serveStderr bytes.Buffer
	go func() {
		done <- pairingServeRunnerWithAutoApprove(t, serveCtx, time.Time{}, ready).Run([]string{"serve", "--profile", targetProfilePath, "--listen", "127.0.0.1:0"}, &serveStdout, &serveStderr)
	}()
	info := waitServePairingReady(t, ready, &serveStderr)
	var pairStdout bytes.Buffer
	var pairStderr bytes.Buffer

	got := Runner{}.Run([]string{"pair", "--profile", sourceProfilePath, "--target", info.Address, "--verification-code", info.VerificationCode}, &pairStdout, &pairStderr)

	if got != 2 {
		t.Fatalf("pair unbound bootstrap exit = %d stderr = %q, want 2", got, pairStderr.String())
	}
	if !strings.Contains(pairStderr.String(), "transport identity") {
		t.Fatalf("pair unbound bootstrap stderr = %q, want transport identity refusal", pairStderr.String())
	}
	updated, err := profile.ReadFile(sourceProfilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", sourceProfilePath, err)
	}
	if updated.Target.DevicePublicKey != "" || updated.Target.PairingReceiptID != "" || updated.Target.PairedAt != "" {
		t.Fatalf("unbound bootstrap target = %#v, want no pairing pins", updated.Target)
	}
	if _, err := os.Lstat(filepath.Join(target, ".supermover")); !os.IsNotExist(err) {
		t.Fatalf("unbound bootstrap .supermover state error = %v, want not exist", err)
	}
	cancelServe()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not exit after cancel")
	}
}

func TestPairRejectsWrongVerificationCodeWithoutMutatingProfile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "source.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceCert := newCLITestCertificate(t, "pair-source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Network = networkConfigForCLI(t, sourceCert, reserveTCPAddress(t))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	ready := make(chan pairserve.ReadyInfo, 1)
	done := make(chan int, 1)
	var serveStdout bytes.Buffer
	var serveStderr bytes.Buffer
	go func() {
		done <- pairingServeRunnerWithAutoApprove(t, serveCtx, time.Time{}, ready).Run([]string{"serve", "--profile", profilePath, "--listen", "127.0.0.1:0"}, &serveStdout, &serveStderr)
	}()
	info := waitServePairingReady(t, ready, &serveStderr)
	var pairStdout bytes.Buffer
	var pairStderr bytes.Buffer

	got := Runner{}.Run([]string{"pair", "--profile", profilePath, "--target", info.Address, "--verification-code", "000000"}, &pairStdout, &pairStderr)

	if got != 2 {
		t.Fatalf("pair wrong code exit = %d stderr = %q, want 2", got, pairStderr.String())
	}
	if !strings.Contains(pairStderr.String(), "pairing verification failed") {
		t.Fatalf("pair wrong code stderr = %q, want verification failure", pairStderr.String())
	}
	updated, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if updated.Target.DevicePublicKey != "" || updated.Target.PairingReceiptID != "" || updated.Target.PairedAt != "" {
		t.Fatalf("wrong code updated target = %#v, want no pairing pins", updated.Target)
	}
	if _, err := os.Lstat(filepath.Join(target, ".supermover")); !os.IsNotExist(err) {
		t.Fatalf("wrong code .supermover state error = %v, want not exist", err)
	}
	cancelServe()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not exit after cancel")
	}
}

func TestPairRejectsAlreadyPairedDifferentIdentity(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "source.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, p)
	otherBootstrap := pairing.Bootstrap{
		ProtocolVersion:        protocol.Version,
		Status:                 "pairing_ready",
		TargetDeviceID:         "sha256:fedcba9876543210",
		ChallengeID:            "pair-other",
		ExpiresAt:              time.Now().Add(time.Minute).UTC(),
		Trusted:                false,
		TransferEnabled:        false,
		TransportIdentityBound: true,
	}
	otherBootstrap.VerificationHash = pairing.VerificationHash(otherBootstrap.TargetDeviceID, otherBootstrap.ChallengeID, "123456")
	endpoint := httptestPairingServer(t, otherBootstrap)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{}.Run([]string{"pair", "--profile", profilePath, "--target", endpoint, "--verification-code", "123456"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("pair already paired exit = %d stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "target_device_id does not match profile target identity") {
		t.Fatalf("pair already paired stderr = %q, want target identity mismatch", stderr.String())
	}
	updated, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if updated.Target.DevicePublicKey != p.Target.DevicePublicKey || updated.Target.PairingReceiptID != p.Target.PairingReceiptID || updated.Target.PairedAt != p.Target.PairedAt {
		t.Fatalf("already paired target = %#v, want pins preserved", updated.Target)
	}
}

func TestPairRejectsMissingSourceTLSIdentityWithoutMutatingProfile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "source.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	targetCert := newCLITestCertificate(t, "pair-target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetDeviceID := certDeviceIDForCLI(t, targetCert)
	bootstrap := pairing.Bootstrap{
		ProtocolVersion:        protocol.Version,
		Status:                 "pairing_ready",
		TargetDeviceID:         targetDeviceID,
		ChallengeID:            "pair-source-missing-cert",
		ExpiresAt:              time.Now().Add(time.Minute).UTC(),
		Trusted:                false,
		TransferEnabled:        false,
		TransportIdentityBound: true,
	}
	bootstrap.VerificationHash = pairing.VerificationHash(bootstrap.TargetDeviceID, bootstrap.ChallengeID, "123456")
	endpoint := httptestPairingServer(t, bootstrap)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{}.Run([]string{"pair", "--profile", profilePath, "--target", endpoint, "--verification-code", "123456"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("pair missing source tls exit = %d stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "source") || !strings.Contains(stderr.String(), "network.local_tls_identity") {
		t.Fatalf("pair missing source tls stderr = %q, want source transport identity refusal", stderr.String())
	}
	updated, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if updated.Target.DevicePublicKey != "" || updated.Target.PairingReceiptID != "" || updated.Target.PairedAt != "" {
		t.Fatalf("missing source tls target = %#v, want no pairing pins", updated.Target)
	}
	if _, err := os.Lstat(filepath.Join(target, ".supermover")); !os.IsNotExist(err) {
		t.Fatalf("missing source tls .supermover state error = %v, want not exist", err)
	}
}

func TestPairRejectsExpiredBootstrapWithoutMutatingProfile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "source.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceCert := newCLITestCertificate(t, "pair-source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Network = networkConfigForCLI(t, sourceCert, reserveTCPAddress(t))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	targetDeviceID, err := pairing.TargetDeviceID(p)
	if err != nil {
		t.Fatalf("pairing.TargetDeviceID() error = %v, want nil", err)
	}
	bootstrap := pairing.Bootstrap{
		ProtocolVersion:        protocol.Version,
		Status:                 "pairing_ready",
		TargetDeviceID:         targetDeviceID,
		ChallengeID:            "pair-expired",
		ExpiresAt:              time.Date(2026, 5, 16, 9, 59, 0, 0, time.UTC),
		Trusted:                false,
		TransferEnabled:        false,
		TransportIdentityBound: true,
	}
	bootstrap.VerificationHash = pairing.VerificationHash(bootstrap.TargetDeviceID, bootstrap.ChallengeID, "123456")
	endpoint := httptestPairingServer(t, bootstrap)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)}.Run([]string{"pair", "--profile", profilePath, "--target", endpoint, "--verification-code", "123456"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("pair expired bootstrap exit = %d stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "challenge expired") {
		t.Fatalf("pair expired bootstrap stderr = %q, want challenge expired", stderr.String())
	}
	updated, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if updated.Target.DevicePublicKey != "" || updated.Target.PairingReceiptID != "" || updated.Target.PairedAt != "" {
		t.Fatalf("expired bootstrap target = %#v, want no pairing pins", updated.Target)
	}
	if _, err := os.Lstat(filepath.Join(target, ".supermover")); !os.IsNotExist(err) {
		t.Fatalf("expired bootstrap .supermover state error = %v, want not exist", err)
	}
}

func TestPairRejectsExistingExportedReceiptWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	exportDir := filepath.Join(dir, "receipts")
	profilePath := filepath.Join(dir, "source.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustMkdir(t, exportDir)
	sourceCert := newCLITestCertificate(t, "pair-source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Network = networkConfigForCLI(t, sourceCert, reserveTCPAddress(t))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	targetDeviceID, err := pairing.TargetDeviceID(p)
	if err != nil {
		t.Fatalf("pairing.TargetDeviceID() error = %v, want nil", err)
	}
	bootstrap := pairing.Bootstrap{
		ProtocolVersion:        protocol.Version,
		Status:                 "pairing_ready",
		TargetDeviceID:         targetDeviceID,
		ChallengeID:            "pair-collision",
		ExpiresAt:              time.Now().Add(time.Minute).UTC(),
		Trusted:                false,
		TransferEnabled:        false,
		TransportIdentityBound: true,
	}
	bootstrap.VerificationHash = pairing.VerificationHash(bootstrap.TargetDeviceID, bootstrap.ChallengeID, "123456")
	existingID := pairing.LocalReceiptID(p, bootstrap)
	receiptPath := filepath.Join(exportDir, existingID+".json")
	mustWrite(t, receiptPath, `{"preserve":"audit"}`)
	endpoint := httptestPairingServer(t, bootstrap)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{}.Run([]string{"pair", "--profile", profilePath, "--target", endpoint, "--verification-code", "123456", "--receipt-out", exportDir}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("pair existing exported receipt exit = %d stderr = %q, want 1", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("pair existing exported receipt stderr = %q, want no-replace refusal", stderr.String())
	}
	gotBytes, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", receiptPath, err)
	}
	if string(gotBytes) != `{"preserve":"audit"}` {
		t.Fatalf("existing receipt = %q, want preserved audit evidence", string(gotBytes))
	}
	updated := mustReadProfile(t, profilePath)
	if updated.Target.DevicePublicKey != "" || updated.Target.PairingReceiptID != "" || updated.Target.PairedAt != "" {
		t.Fatalf("existing exported receipt target = %#v, want no pairing pins", updated.Target)
	}
}

func TestPairRejectsSymlinkedExportPathWithoutMutatingProfile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	outside := filepath.Join(dir, "outside")
	exportPath := filepath.Join(dir, "receipt.json")
	profilePath := filepath.Join(dir, "source.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	mustMkdir(t, outside)
	sourceCert := newCLITestCertificate(t, "pair-source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Network = networkConfigForCLI(t, sourceCert, reserveTCPAddress(t))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	targetDeviceID, err := pairing.TargetDeviceID(p)
	if err != nil {
		t.Fatalf("pairing.TargetDeviceID() error = %v, want nil", err)
	}
	bootstrap := pairing.Bootstrap{
		ProtocolVersion:        protocol.Version,
		Status:                 "pairing_ready",
		TargetDeviceID:         targetDeviceID,
		ChallengeID:            "pair-snapshot-collision",
		ExpiresAt:              time.Now().Add(time.Minute).UTC(),
		Trusted:                false,
		TransferEnabled:        false,
		TransportIdentityBound: true,
	}
	bootstrap.VerificationHash = pairing.VerificationHash(bootstrap.TargetDeviceID, bootstrap.ChallengeID, "123456")
	realExport := filepath.Join(outside, "receipt.json")
	if err := os.WriteFile(realExport, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", realExport, err)
	}
	if err := os.Symlink(realExport, exportPath); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	endpoint := httptestPairingServer(t, bootstrap)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{}.Run([]string{"pair", "--profile", profilePath, "--target", endpoint, "--verification-code", "123456", "--receipt-out", exportPath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("pair symlinked export path exit = %d stderr = %q, want 1", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "symlink") {
		t.Fatalf("pair symlinked export path stderr = %q, want symlink refusal", stderr.String())
	}
	gotBytes, err := os.ReadFile(realExport)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", realExport, err)
	}
	if string(gotBytes) != "{}\n" {
		t.Fatalf("symlinked export target = %q, want preserved outside evidence", string(gotBytes))
	}
	updated := mustReadProfile(t, profilePath)
	if updated.Target.DevicePublicKey != "" || updated.Target.PairingReceiptID != "" || updated.Target.PairedAt != "" {
		t.Fatalf("symlinked export path target = %#v, want no pairing pins", updated.Target)
	}
}

func TestPairRejectsSymlinkedProfileBeforeWritingControlPlane(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	realProfilePath := filepath.Join(dir, "real.profile.json")
	profilePath := filepath.Join(dir, "source.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	sourceCert := newCLITestCertificate(t, "pair-source", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Network = networkConfigForCLI(t, sourceCert, reserveTCPAddress(t))
	if err := profile.WriteFile(realProfilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", realProfilePath, err)
	}
	if err := os.Symlink(realProfilePath, profilePath); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	targetDeviceID, err := pairing.TargetDeviceID(p)
	if err != nil {
		t.Fatalf("pairing.TargetDeviceID() error = %v, want nil", err)
	}
	bootstrap := pairing.Bootstrap{
		ProtocolVersion:        protocol.Version,
		Status:                 "pairing_ready",
		TargetDeviceID:         targetDeviceID,
		ChallengeID:            "pair-profile-symlink",
		ExpiresAt:              time.Now().Add(time.Minute).UTC(),
		Trusted:                false,
		TransferEnabled:        false,
		TransportIdentityBound: true,
	}
	bootstrap.VerificationHash = pairing.VerificationHash(bootstrap.TargetDeviceID, bootstrap.ChallengeID, "123456")
	endpoint := httptestPairingServer(t, bootstrap)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{}.Run([]string{"pair", "--profile", profilePath, "--target", endpoint, "--verification-code", "123456"}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("pair symlink profile exit = %d stderr = %q, want 1", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "profile file") || !strings.Contains(stderr.String(), "symlink") {
		t.Fatalf("pair symlink profile stderr = %q, want profile symlink refusal", stderr.String())
	}
	updated := mustReadProfile(t, realProfilePath)
	if updated.Target.DevicePublicKey != "" || updated.Target.PairingReceiptID != "" || updated.Target.PairedAt != "" {
		t.Fatalf("symlink profile target = %#v, want no pairing pins", updated.Target)
	}
	if _, err := os.Lstat(filepath.Join(target, ".supermover")); !os.IsNotExist(err) {
		t.Fatalf("symlink profile .supermover state error = %v, want not exist", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".supermover-pairings")); !os.IsNotExist(err) {
		t.Fatalf("symlink profile local receipt dir error = %v, want not exist", err)
	}
}
