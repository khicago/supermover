package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/protocol"
)

func TestProfileInitAndLint(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"profile", "init", "--profile", profilePath, "--source", source, "--target", target}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile init exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if p.Roots[0].Path != source {
		t.Errorf("profile root path = %q, want %q", p.Roots[0].Path, source)
	}
	if p.Target.LocalPath != target {
		t.Errorf("profile target local path = %q, want %q", p.Target.LocalPath, target)
	}
	if p.Target.TargetID == filepath.Clean(target) {
		t.Errorf("profile target id = %q, want identity separate from local path", p.Target.TargetID)
	}

	stdout.Reset()
	stderr.Reset()
	got = Run([]string{"profile", "lint", "--profile", profilePath}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile lint exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	for _, want := range []string{
		"profile ok",
		"privacy policy=status=profile_contract_only",
		"traffic_level=2",
		"claim=bounded_reduction_only",
		"configured_reductions=",
		"overhead_status=not_applied",
		"overhead_source=profile_contract",
		"residual_leakage=",
		"total_bytes",
		"duration",
		"peer_ip",
		"lan_presence",
		"supermover_use",
		"local_push=traffic_shaping_not_applied",
		"network_transfer=not_configured",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("profile lint stdout = %q, want %q", stdout.String(), want)
		}
	}
	for _, forbidden := range []string{"anonymous", "anonymity", "transfer_ready=true", "network_ready=true"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Errorf("profile lint stdout = %q, must not contain %q", stdout.String(), forbidden)
		}
	}
}

func TestProfileSetTargetUpdatesProfileSSOT(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	nextTarget := filepath.Join(dir, "next-target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)
	before, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) before set-target error = %v, want nil", profilePath, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"profile", "set-target", "--profile", profilePath, "--target", nextTarget, "--name", "Next target"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile set-target exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if p.Target.LocalPath != nextTarget {
		t.Errorf("profile target local path = %q, want %q", p.Target.LocalPath, nextTarget)
	}
	if p.Target.Name != "Next target" {
		t.Errorf("profile target name = %q, want %q", p.Target.Name, "Next target")
	}
	if p.Target.TargetID != before.Target.TargetID {
		t.Errorf("profile target id = %q, want unchanged %q without --target-id", p.Target.TargetID, before.Target.TargetID)
	}
}

func TestProfileSetTargetExplicitlyUpdatesTargetID(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	nextTarget := filepath.Join(dir, "next-target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	writeDefaultProfile(t, profilePath, source, target)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"profile", "set-target", "--profile", profilePath, "--target", nextTarget, "--target-id", "local:next-target"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile set-target --target-id exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	p, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if p.Target.TargetID != "local:next-target" {
		t.Errorf("profile target id = %q, want local:next-target", p.Target.TargetID)
	}
	if p.Target.LocalPath != nextTarget {
		t.Errorf("profile target local path = %q, want %q", p.Target.LocalPath, nextTarget)
	}
}

func TestProfileSetTargetRejectsTargetIDChangeForPairedProfile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	nextTarget := filepath.Join(dir, "next-target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v", profilePath, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"profile", "set-target", "--profile", profilePath, "--target", nextTarget, "--target-id", "local:next-target"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("profile set-target paired --target-id exit = %d, stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "cannot change target-id for a paired profile") {
		t.Fatalf("profile set-target paired --target-id stderr = %q, want paired profile refusal", stderr.String())
	}
}

func TestProfileSetTargetAllowsLocalPathChangeForPairedProfile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	nextTarget := filepath.Join(dir, "next-target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Target.DevicePublicKey = "sha256:0123456789abcdef"
	p.Target.PairingReceiptID = "pairing-1"
	p.Target.PairedAt = "2026-05-16T00:00:00Z"
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v", profilePath, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"profile", "set-target", "--profile", profilePath, "--target", nextTarget, "--name", "Mounted target"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile set-target paired local path exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	updated, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if updated.Target.LocalPath != filepath.Clean(nextTarget) || updated.Target.TargetID != p.Target.TargetID {
		t.Fatalf("paired profile target = %#v, want local path changed and target_id unchanged", updated.Target)
	}
	if updated.Target.DevicePublicKey != p.Target.DevicePublicKey || updated.Target.PairingReceiptID != p.Target.PairingReceiptID || updated.Target.PairedAt != p.Target.PairedAt {
		t.Fatalf("paired profile target = %#v, want pairing pins preserved", updated.Target)
	}
}

func TestProfileSetTargetRepairsLegacyPathTargetID(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	nextTarget := filepath.Join(dir, "next-target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	p := profile.NewDefault("profile-local", "Local profile", source, target)
	p.Target.TargetID = filepath.Clean(target)
	data := `{
  "version": 1,
  "profile_id": "` + p.ProfileID + `",
  "name": "` + p.Name + `",
  "roots": [{"id": "root", "path": "` + filepath.ToSlash(source) + `"}],
  "include": [{"pattern": "**"}],
  "consistency": "strict",
  "delete_policy": {"mode": "record", "require_review": true, "retention_days": 30},
  "metadata_policy": {"mode": "basic", "preserve_permissions": true, "preserve_mod_time": true},
  "privacy_policy": {"mode": "plaintext", "traffic_level": 2, "allow_plaintext_restore": true, "allow_hidden_files": true, "allow_sensitive_filenames": true, "padding_bucket_bytes": 65536, "batch_max_bytes": 1048576, "batch_max_count": 64, "jitter_budget_millis": 250, "discovery_low_info": true},
  "target": {"target_id": "` + filepath.ToSlash(target) + `", "name": "target", "local_path": "` + filepath.ToSlash(target) + `"},
  "agent_knowledge": {}
}
`
	if err := os.WriteFile(profilePath, []byte(data), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	if _, err := profile.ReadFile(profilePath); err == nil {
		t.Fatalf("profile.ReadFile(legacy path identity) error = nil, want validation error before repair")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{"profile", "set-target", "--profile", profilePath, "--target", nextTarget, "--target-id", "local:repaired"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile set-target repair exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	repaired, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) after repair error = %v, want nil", profilePath, err)
	}
	if repaired.Target.TargetID != "local:repaired" || repaired.Target.LocalPath != filepath.Clean(nextTarget) {
		t.Fatalf("repaired target = %#v, want explicit id and next target path", repaired.Target)
	}
}

func TestProfileSetNetworkUpdatesReceiverURLAndTLSIdentity(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)
	certPath := filepath.Join(dir, "device.crt")
	keyPath := filepath.Join(dir, "device.key")
	if err := os.WriteFile(certPath, []byte("cert"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(cert) error = %v, want nil", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(key) error = %v, want nil", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{
		"profile", "set-network",
		"--profile", profilePath,
		"--receiver-url", "https://127.0.0.1:9443",
		"--tls-cert", certPath,
		"--tls-key", keyPath,
	}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile set-network exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	updated, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if updated.Network == nil {
		t.Fatal("updated.Network = nil, want configured network")
	}
	if updated.Network.ReceiverURL != "https://127.0.0.1:9443" {
		t.Fatalf("receiver_url = %q, want https://127.0.0.1:9443", updated.Network.ReceiverURL)
	}
	if updated.Network.LocalTLSIdentity.CertificatePath != certPath || updated.Network.LocalTLSIdentity.PrivateKeyPath != keyPath {
		t.Fatalf("local tls identity = %#v, want cert/key paths", updated.Network.LocalTLSIdentity)
	}
	if !strings.Contains(stdout.String(), "updated profile network") || !strings.Contains(stdout.String(), "tls_identity=configured") {
		t.Fatalf("profile set-network stdout = %q, want summary line", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("profile set-network stderr = %q, want empty", stderr.String())
	}
}

func TestProfileSetNetworkClearsNetworkSectionWhenRequested(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	p := profile.NewDefault("profile-local", "Profile", source, target)
	p.Network = &profile.NetworkConfig{
		ReceiverURL: "https://127.0.0.1:9443",
		LocalTLSIdentity: profile.TLSIdentityRef{
			CertificatePath: filepath.Join(dir, "device.crt"),
			PrivateKeyPath:  filepath.Join(dir, "device.key"),
		},
	}
	if err := os.WriteFile(p.Network.LocalTLSIdentity.CertificatePath, []byte("cert"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(cert) error = %v, want nil", err)
	}
	if err := os.WriteFile(p.Network.LocalTLSIdentity.PrivateKeyPath, []byte("key"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(key) error = %v, want nil", err)
	}
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Run([]string{
		"profile", "set-network",
		"--profile", profilePath,
		"--clear-receiver-url",
		"--clear-tls-identity",
	}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("profile set-network clear exit = %d, stderr = %q, want 0", got, stderr.String())
	}
	updated, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if updated.Network != nil {
		t.Fatalf("updated.Network = %#v, want nil after clearing receiver_url and tls identity", updated.Network)
	}
}

func TestProfileSetNetworkRejectsPartialOrConflictingFlags(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	writeDefaultProfile(t, profilePath, source, target)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no changes",
			args: []string{"profile", "set-network", "--profile", profilePath},
			want: "provide at least one network change",
		},
		{
			name: "partial tls identity",
			args: []string{"profile", "set-network", "--profile", profilePath, "--tls-cert", filepath.Join(dir, "device.crt")},
			want: "--tls-cert and --tls-key must be provided together",
		},
		{
			name: "clear tls with explicit tls",
			args: []string{"profile", "set-network", "--profile", profilePath, "--clear-tls-identity", "--tls-cert", filepath.Join(dir, "device.crt"), "--tls-key", filepath.Join(dir, "device.key")},
			want: "--clear-tls-identity cannot be combined",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			got := Run(tt.args, &stdout, &stderr)
			if got != 2 {
				t.Fatalf("Run(%v) exit = %d, stderr = %q, want 2", tt.args, got, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("Run(%v) stderr = %q, want %q", tt.args, stderr.String(), tt.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("Run(%v) stdout = %q, want empty", tt.args, stdout.String())
			}
		})
	}
}

func TestProfileAdoptPairingUpdatesTargetPinsFromReceipt(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetDeviceID := certDeviceIDForCLI(t, targetCert)
	p := profile.NewDefault("profile-local", "Target profile", source, target)
	p.Network = networkConfigForCLI(t, targetCert, reserveTCPAddress(t))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, profile.Profile{
		ProfileID: p.ProfileID,
		Target: profile.TargetIdentity{
			TargetID:         p.Target.TargetID,
			DevicePublicKey:  targetDeviceID,
			PairingReceiptID: "pairing-1",
			PairedAt:         "2026-05-16T00:00:00Z",
		},
	}, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = "sha256:source0123456789abcdef"
		receipt.TargetDeviceID = targetDeviceID
		receipt.DevicePublicKey = targetDeviceID
		receipt.VerifiedAt = "2026-05-16T00:00:00Z"
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Runner{Now: cliTLSNow()}.Run([]string{"profile", "adopt-pairing", "--profile", profilePath, "--receipt-id", "pairing-1"}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("profile adopt-pairing exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	assertTextContainsAll(t, "profile adopt-pairing", stdout.String(), "adopted pairing", "receipt=pairing-1", "profile=profile-local", "target_device="+targetDeviceID)
	if stderr.Len() != 0 {
		t.Fatalf("profile adopt-pairing stderr = %q, want empty", stderr.String())
	}
	updated, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if updated.Target.DevicePublicKey != targetDeviceID || updated.Target.PairingReceiptID != "pairing-1" || updated.Target.PairedAt != "2026-05-16T00:00:00Z" {
		t.Fatalf("adopted target = %#v, want receipt-derived pins", updated.Target)
	}
}

func TestProfileAdoptPairingUpdatesTargetPinsFromExportedReceiptFile(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	receiptPath := filepath.Join(dir, "pairing-1.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetDeviceID := certDeviceIDForCLI(t, targetCert)
	p := profile.NewDefault("profile-local", "Target profile", source, target)
	p.Network = networkConfigForCLI(t, targetCert, reserveTCPAddress(t))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	receipt := control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               "pairing-1",
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   "sha256:source0123456789abcdef",
		TargetDeviceID:   targetDeviceID,
		DevicePublicKey:  targetDeviceID,
		Method:           "sas",
		VerifiedAt:       "2026-05-16T00:00:00Z",
		VerificationHash: "sha256:abcdef0123456789",
		ProtocolVersion:  protocol.Version,
	}
	if err := control.WriteFile(receiptPath, receipt); err != nil {
		t.Fatalf("control.WriteFile(%q) error = %v, want nil", receiptPath, err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Runner{Now: cliTLSNow()}.Run([]string{"profile", "adopt-pairing", "--profile", profilePath, "--receipt-file", receiptPath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("profile adopt-pairing --receipt-file exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	updated, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if updated.Target.DevicePublicKey != targetDeviceID || updated.Target.PairingReceiptID != "pairing-1" || updated.Target.PairedAt != "2026-05-16T00:00:00Z" {
		t.Fatalf("adopted exported receipt target = %#v, want receipt-derived pins", updated.Target)
	}
	persisted := readPairingReceiptDocForProfileTest(t, target, "pairing-1")
	if persisted != receipt {
		t.Fatalf("persisted exported receipt = %#v, want %#v", persisted, receipt)
	}
}

func TestProfileAdoptPairingReusesExistingMatchingTargetReceipt(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	receiptPath := filepath.Join(dir, "pairing-1.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetDeviceID := certDeviceIDForCLI(t, targetCert)
	p := profile.NewDefault("profile-local", "Target profile", source, target)
	p.Network = networkConfigForCLI(t, targetCert, reserveTCPAddress(t))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	receipt := control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               "pairing-1",
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   "sha256:source0123456789abcdef",
		TargetDeviceID:   targetDeviceID,
		DevicePublicKey:  targetDeviceID,
		Method:           "sas",
		VerifiedAt:       "2026-05-16T00:00:00Z",
		VerificationHash: "sha256:abcdef0123456789",
		ProtocolVersion:  protocol.Version,
	}
	if err := control.WriteFile(receiptPath, receipt); err != nil {
		t.Fatalf("control.WriteFile(%q) error = %v, want nil", receiptPath, err)
	}
	writePairingReceiptForCLI(t, target, profile.Profile{
		ProfileID: p.ProfileID,
		Target: profile.TargetIdentity{
			TargetID:         p.Target.TargetID,
			DevicePublicKey:  targetDeviceID,
			PairingReceiptID: receipt.ID,
			PairedAt:         receipt.VerifiedAt,
		},
	}, func(existing *control.PairingReceipt) {
		*existing = receipt
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Runner{Now: cliTLSNow()}.Run([]string{"profile", "adopt-pairing", "--profile", profilePath, "--receipt-file", receiptPath}, &stdout, &stderr)

	if got != 0 {
		t.Fatalf("profile adopt-pairing existing matching receipt exit = %d stderr = %q stdout = %q, want 0", got, stderr.String(), stdout.String())
	}
	persisted := readPairingReceiptDocForProfileTest(t, target, receipt.ID)
	if persisted != receipt {
		t.Fatalf("persisted matching receipt = %#v, want %#v", persisted, receipt)
	}
}

func TestProfileAdoptPairingRejectsConflictingExistingTargetReceipt(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	receiptPath := filepath.Join(dir, "pairing-1.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetDeviceID := certDeviceIDForCLI(t, targetCert)
	p := profile.NewDefault("profile-local", "Target profile", source, target)
	p.Network = networkConfigForCLI(t, targetCert, reserveTCPAddress(t))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	receipt := control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               "pairing-1",
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   "sha256:source0123456789abcdef",
		TargetDeviceID:   targetDeviceID,
		DevicePublicKey:  targetDeviceID,
		Method:           "sas",
		VerifiedAt:       "2026-05-16T00:00:00Z",
		VerificationHash: "sha256:abcdef0123456789",
		ProtocolVersion:  protocol.Version,
	}
	if err := control.WriteFile(receiptPath, receipt); err != nil {
		t.Fatalf("control.WriteFile(%q) error = %v, want nil", receiptPath, err)
	}
	writePairingReceiptForCLI(t, target, profile.Profile{
		ProfileID: p.ProfileID,
		Target: profile.TargetIdentity{
			TargetID:         p.Target.TargetID,
			DevicePublicKey:  targetDeviceID,
			PairingReceiptID: receipt.ID,
			PairedAt:         receipt.VerifiedAt,
		},
	}, func(existing *control.PairingReceipt) {
		existing.VerificationHash = "sha256:different0123456789"
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Runner{Now: cliTLSNow()}.Run([]string{"profile", "adopt-pairing", "--profile", profilePath, "--receipt-file", receiptPath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("profile adopt-pairing conflicting existing receipt exit = %d stderr = %q stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "already exists with different content") {
		t.Fatalf("profile adopt-pairing conflicting existing receipt stderr = %q, want conflict refusal", stderr.String())
	}
	updated, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if updated.Target.DevicePublicKey != "" || updated.Target.PairingReceiptID != "" || updated.Target.PairedAt != "" {
		t.Fatalf("conflicting receipt updated target = %#v, want no adopted pins", updated.Target)
	}
}

func TestProfileAdoptPairingRejectsSymlinkedProfileBeforeWritingTargetControl(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	realProfilePath := filepath.Join(dir, "real.target.profile.json")
	profilePath := filepath.Join(dir, "target.profile.json")
	receiptPath := filepath.Join(dir, "pairing-1.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	targetDeviceID := certDeviceIDForCLI(t, targetCert)
	p := profile.NewDefault("profile-local", "Target profile", source, target)
	p.Network = networkConfigForCLI(t, targetCert, reserveTCPAddress(t))
	if err := profile.WriteFile(realProfilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", realProfilePath, err)
	}
	if err := os.Symlink(realProfilePath, profilePath); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	receipt := control.PairingReceipt{
		Version:          control.CurrentVersion,
		ID:               "pairing-1",
		ProfileID:        p.ProfileID,
		TargetID:         p.Target.TargetID,
		SourceDeviceID:   "sha256:source0123456789abcdef",
		TargetDeviceID:   targetDeviceID,
		DevicePublicKey:  targetDeviceID,
		Method:           "sas",
		VerifiedAt:       "2026-05-16T00:00:00Z",
		VerificationHash: "sha256:abcdef0123456789",
		ProtocolVersion:  protocol.Version,
	}
	if err := control.WriteFile(receiptPath, receipt); err != nil {
		t.Fatalf("control.WriteFile(%q) error = %v, want nil", receiptPath, err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	got := Runner{Now: cliTLSNow()}.Run([]string{"profile", "adopt-pairing", "--profile", profilePath, "--receipt-file", receiptPath}, &stdout, &stderr)

	if got != 1 {
		t.Fatalf("profile adopt-pairing symlink profile exit = %d stderr = %q stdout = %q, want 1", got, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "profile file") || !strings.Contains(stderr.String(), "symlink") {
		t.Fatalf("profile adopt-pairing symlink profile stderr = %q, want profile symlink refusal", stderr.String())
	}
	updated := mustReadProfile(t, realProfilePath)
	if updated.Target.DevicePublicKey != "" || updated.Target.PairingReceiptID != "" || updated.Target.PairedAt != "" {
		t.Fatalf("symlink profile target = %#v, want no adopted pins", updated.Target)
	}
	if _, err := os.Lstat(filepath.Join(target, control.DirName)); !os.IsNotExist(err) {
		t.Fatalf("symlink profile target control dir error = %v, want not exist", err)
	}
}

func readPairingReceiptDocForProfileTest(t *testing.T, targetRoot, receiptID string) control.PairingReceipt {
	t.Helper()
	path, err := control.Path(targetRoot, control.ArtifactPairingReceipt, receiptID)
	if err != nil {
		t.Fatalf("control.Path(pairing receipt %q) error = %v, want nil", receiptID, err)
	}
	doc, err := control.ReadFile[control.PairingReceipt](path)
	if err != nil {
		t.Fatalf("control.ReadFile(%q) error = %v, want nil", path, err)
	}
	return doc
}

func TestProfileAdoptPairingRejectsTargetTLSIdentityMismatch(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	target := filepath.Join(dir, "target")
	profilePath := filepath.Join(dir, "target.profile.json")
	mustMkdir(t, source)
	mustMkdir(t, target)
	targetCert := newCLITestCertificate(t, "target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	otherCert := newCLITestCertificate(t, "other-target", cliTLSNow().Add(-time.Hour), cliTLSNow().Add(time.Hour))
	p := profile.NewDefault("profile-local", "Target profile", source, target)
	p.Network = networkConfigForCLI(t, targetCert, reserveTCPAddress(t))
	if err := profile.WriteFile(profilePath, p); err != nil {
		t.Fatalf("profile.WriteFile(%q) error = %v, want nil", profilePath, err)
	}
	writePairingReceiptForCLI(t, target, profile.Profile{
		ProfileID: p.ProfileID,
		Target: profile.TargetIdentity{
			TargetID:         p.Target.TargetID,
			DevicePublicKey:  certDeviceIDForCLI(t, otherCert),
			PairingReceiptID: "pairing-1",
			PairedAt:         "2026-05-16T00:00:00Z",
		},
	}, func(receipt *control.PairingReceipt) {
		receipt.SourceDeviceID = "sha256:source0123456789abcdef"
		receipt.TargetDeviceID = certDeviceIDForCLI(t, otherCert)
		receipt.DevicePublicKey = certDeviceIDForCLI(t, otherCert)
		receipt.VerifiedAt = "2026-05-16T00:00:00Z"
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	got := Runner{Now: cliTLSNow()}.Run([]string{"profile", "adopt-pairing", "--profile", profilePath, "--receipt-id", "pairing-1"}, &stdout, &stderr)

	if got != 2 {
		t.Fatalf("profile adopt-pairing target tls mismatch exit = %d stderr = %q, want 2", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "validate target local TLS identity files") || !strings.Contains(stderr.String(), "does not match pinned device id") {
		t.Fatalf("profile adopt-pairing target tls mismatch stderr = %q, want target TLS mismatch", stderr.String())
	}
	updated, err := profile.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile.ReadFile(%q) error = %v, want nil", profilePath, err)
	}
	if updated.Target.DevicePublicKey != "" || updated.Target.PairingReceiptID != "" || updated.Target.PairedAt != "" {
		t.Fatalf("target tls mismatch updated target = %#v, want no adopted pins", updated.Target)
	}
}
