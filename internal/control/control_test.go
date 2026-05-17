package control

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khicago/supermover/internal/transport"
)

func TestPath(t *testing.T) {
	root := filepath.Join("tmp", "target")
	tests := []struct {
		name     string
		artifact ArtifactType
		id       string
		want     string
	}{
		{"profile", ArtifactProfileSnapshot, "p1", filepath.Join(root, DirName, "profiles", "p1.json")},
		{"pairing", ArtifactPairingReceipt, "pair1", filepath.Join(root, DirName, "pairings", "pair1.json")},
		{"session", ArtifactSessionReceipt, "s1", filepath.Join(root, DirName, "sessions", "s1", "receipt.json")},
		{"manifest", ArtifactManifest, "s1", filepath.Join(root, DirName, "sessions", "s1", "manifest.json")},
		{"warning", ArtifactWarning, "w1", filepath.Join(root, DirName, "warnings", "w1.json")},
		{"drift", ArtifactTargetDrift, "d1", filepath.Join(root, DirName, "drift", "d1.json")},
		{"delete", ArtifactSoftDelete, "del1", filepath.Join(root, DirName, "deleted", "del1.json")},
		{"prune approval", ArtifactPruneApproval, "approval1", filepath.Join(root, DirName, "prune", "approvals", "approval1.json")},
		{"prune receipt", ArtifactPruneReceipt, "prune1", filepath.Join(root, DirName, "prune", "receipts", "prune1.json")},
		{"history", ArtifactHistoryIndex, "", filepath.Join(root, DirName, "history", "index.json")},
		{"recovery", ArtifactRecoveryState, "", filepath.Join(root, DirName, "recovery", "state.json")},
		{"network transfer", ArtifactNetworkTransfer, "s1", filepath.Join(root, DirName, "sessions", "s1", "network-transfer.json")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Path(root, tt.artifact, tt.id)
			if err != nil {
				t.Fatalf("Path() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Path() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadFileNoSymlinkRejectsSymlinkArtifact(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := WriteFile(target, validProfileSnapshot()); err != nil {
		t.Fatalf("WriteFile(target) error = %v, want nil", err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink(link) error = %v, want nil", err)
	}

	_, err := ReadFileNoSymlink[ProfileSnapshot](link)
	if err == nil {
		t.Fatalf("ReadFileNoSymlink(link) error = nil, want symlink rejection")
	}
}

func TestReadFileNoSymlinkUnderRootRejectsSymlinkParent(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "target")
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(filepath.Join(root, DirName), 0o755); err != nil {
		t.Fatalf("MkdirAll(control dir) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Join(outside, "profiles"), 0o755); err != nil {
		t.Fatalf("MkdirAll(outside profiles) error = %v, want nil", err)
	}
	outsideSnapshot := filepath.Join(outside, "profiles", "profile-snapshot.json")
	if err := WriteFile(outsideSnapshot, validProfileSnapshot()); err != nil {
		t.Fatalf("WriteFile(outside snapshot) error = %v, want nil", err)
	}
	profilesDir := filepath.Join(root, DirName, "profiles")
	if err := os.Symlink(filepath.Join(outside, "profiles"), profilesDir); err != nil {
		t.Fatalf("Symlink(profiles dir) error = %v, want nil", err)
	}

	_, err := ReadFileNoSymlinkUnderRoot[ProfileSnapshot](root, filepath.Join(profilesDir, "profile-snapshot.json"))
	if err == nil {
		t.Fatalf("ReadFileNoSymlinkUnderRoot(symlink parent) error = nil, want symlink rejection")
	}
}

func TestPathRejectsMissingID(t *testing.T) {
	_, err := Path("/target", ArtifactManifest, "")
	if err == nil {
		t.Fatalf("Path() error = nil, want missing id error")
	}
	if !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("Path() error = %q, want missing id error", err.Error())
	}
}

func TestPathRejectsUnsafeID(t *testing.T) {
	for _, id := range []string{"../escape", "bad/id", `bad\id`} {
		t.Run(id, func(t *testing.T) {
			if _, err := Path("/target", ArtifactManifest, id); err == nil {
				t.Fatalf("Path(%q) error = nil, want unsafe id error", id)
			}
		})
	}
}

func TestPathRejectsDotOnlySessionID(t *testing.T) {
	for _, artifact := range []ArtifactType{ArtifactSessionReceipt, ArtifactManifest} {
		t.Run(string(artifact), func(t *testing.T) {
			if _, err := Path("/target", artifact, "."); err == nil {
				t.Fatalf("Path(%q, %q) error = nil, want unsafe session id error", artifact, ".")
			}
		})
	}
}

func TestValidateArtifactLoadBoundaryRejectsUnsafeControlArtifacts(t *testing.T) {
	tests := []struct {
		name    string
		linkRel string
	}{
		{name: "target root", linkRel: ""},
		{name: "control directory", linkRel: DirName},
		{name: "sessions directory", linkRel: filepath.Join(DirName, "sessions")},
		{name: "session directory", linkRel: filepath.Join(DirName, "sessions", "session")},
		{name: "receipt file", linkRel: filepath.Join(DirName, "sessions", "session", "receipt.json")},
		{name: "manifest file", linkRel: filepath.Join(DirName, "sessions", "session", "manifest.json")},
		{name: "session record file", linkRel: filepath.Join(DirName, "sessions", "session", "session.json")},
		{name: "network transfer file", linkRel: filepath.Join(DirName, "sessions", "session", "network-transfer.json")},
		{name: "profiles directory", linkRel: filepath.Join(DirName, "profiles")},
		{name: "profile snapshot file", linkRel: filepath.Join(DirName, "profiles", "profile-session.json")},
		{name: "pairings directory", linkRel: filepath.Join(DirName, "pairings")},
		{name: "pairing receipt file", linkRel: filepath.Join(DirName, "pairings", "pairing.json")},
		{name: "warnings file", linkRel: filepath.Join(DirName, "warnings", "warning.json")},
		{name: "deleted file", linkRel: filepath.Join(DirName, "deleted", "deleted.json")},
		{name: "drift file", linkRel: filepath.Join(DirName, "drift", "drift.json")},
		{name: "daemon directory", linkRel: filepath.Join(DirName, "daemon")},
		{name: "daemon install file", linkRel: filepath.Join(DirName, "daemon", "install.json")},
		{name: "daemon state file", linkRel: filepath.Join(DirName, "daemon", "state.json")},
		{name: "daemon stop intent file", linkRel: filepath.Join(DirName, "daemon", "stop-intent.json")},
		{name: "daemon restart intent file", linkRel: filepath.Join(DirName, "daemon", "restart-intent.json")},
		{name: "daemon events directory", linkRel: filepath.Join(DirName, "daemon", "events")},
		{name: "daemon event file", linkRel: filepath.Join(DirName, "daemon", "events", "event.json")},
		{name: "prune directory", linkRel: filepath.Join(DirName, "prune")},
		{name: "prune approvals directory", linkRel: filepath.Join(DirName, "prune", "approvals")},
		{name: "prune approval file", linkRel: filepath.Join(DirName, "prune", "approvals", "approval.json")},
		{name: "prune receipts directory", linkRel: filepath.Join(DirName, "prune", "receipts")},
		{name: "prune receipt file", linkRel: filepath.Join(DirName, "prune", "receipts", "receipt.json")},
		{name: "incremental sync directory", linkRel: filepath.Join(DirName, "incremental-sync")},
		{name: "incremental sync profiles directory", linkRel: filepath.Join(DirName, "incremental-sync", "profiles")},
		{name: "incremental sync profile scope directory", linkRel: filepath.Join(DirName, "incremental-sync", "profiles", "profile-scope")},
		{name: "incremental sync targets directory", linkRel: filepath.Join(DirName, "incremental-sync", "profiles", "profile-scope", "targets")},
		{name: "incremental sync target scope directory", linkRel: filepath.Join(DirName, "incremental-sync", "profiles", "profile-scope", "targets", "target-scope")},
		{name: "incremental sync queue file", linkRel: filepath.Join(DirName, "incremental-sync", "profiles", "profile-scope", "targets", "target-scope", "queue.json")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := t.TempDir()
			target := filepath.Join(parent, "target")
			outside := filepath.Join(parent, "outside")
			if err := os.MkdirAll(outside, 0o755); err != nil {
				t.Fatalf("os.MkdirAll(outside) error = %v, want nil", err)
			}
			if tt.linkRel == "" {
				if err := os.Symlink(outside, target); err != nil {
					t.Skipf("symlink target root unavailable: %v", err)
				}
			} else {
				if err := os.MkdirAll(target, 0o755); err != nil {
					t.Fatalf("os.MkdirAll(target) error = %v, want nil", err)
				}
				linkPath := filepath.Join(target, tt.linkRel)
				if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
					t.Fatalf("os.MkdirAll(link parent) error = %v, want nil", err)
				}
				if err := os.Symlink(outside, linkPath); err != nil {
					t.Skipf("symlink %s unavailable: %v", tt.linkRel, err)
				}
			}

			err := ValidateArtifactLoadBoundary(target)
			if err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("ValidateArtifactLoadBoundary(%q) error = %v, want symlink refusal", target, err)
			}
		})
	}
}

func TestValidateDocuments(t *testing.T) {
	tests := []struct {
		name    string
		doc     Document
		wantErr string
	}{
		{name: "profile snapshot", doc: validProfileSnapshot()},
		{name: "pairing receipt", doc: validPairingReceipt()},
		{name: "session receipt", doc: validSessionReceipt()},
		{name: "manifest", doc: validManifest()},
		{name: "warning", doc: validWarning()},
		{name: "target drift", doc: validTargetDrift()},
		{name: "soft delete", doc: validSoftDelete()},
		{name: "prune approval", doc: validPruneApproval()},
		{name: "prune receipt", doc: validPruneReceipt()},
		{name: "history index", doc: validHistoryIndex()},
		{name: "recovery state", doc: validRecoveryState()},
		{name: "network transfer", doc: validNetworkTransfer()},
		{name: "legacy network transfer without privacy policy", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.PrivacyPolicy = transport.PrivacyPolicy{}
			return doc
		}()},
		{name: "invalid pairing receipt unsafe id", doc: func() PairingReceipt {
			receipt := validPairingReceipt()
			receipt.ID = "../pair1"
			return receipt
		}(), wantErr: "id is unsafe"},
		{name: "invalid pairing receipt missing source", doc: PairingReceipt{Version: CurrentVersion, ID: "pair1", ProfileID: "profile1", TargetID: "target1", TargetDeviceID: "sha256:0123456789abcdef", DevicePublicKey: "sha256:0123456789abcdef", Method: "sas", VerifiedAt: "2026-05-16T00:00:00Z", VerificationHash: "sha256:abcdef0123456789", ProtocolVersion: "supermover/1"}, wantErr: "source_device_id"},
		{name: "invalid pairing receipt mismatched target device", doc: PairingReceipt{Version: CurrentVersion, ID: "pair1", ProfileID: "profile1", TargetID: "target1", SourceDeviceID: "sha256:abcdef0123456789", TargetDeviceID: "sha256:0123456789abcdef", DevicePublicKey: "sha256:different012345", Method: "sas", VerifiedAt: "2026-05-16T00:00:00Z", VerificationHash: "sha256:abcdef0123456789", ProtocolVersion: "supermover/1"}, wantErr: "device_public_key must match target_device_id"},
		{name: "invalid pairing receipt method", doc: PairingReceipt{Version: CurrentVersion, ID: "pair1", ProfileID: "profile1", TargetID: "target1", SourceDeviceID: "sha256:abcdef0123456789", TargetDeviceID: "sha256:0123456789abcdef", DevicePublicKey: "sha256:0123456789abcdef", Method: "sms", VerifiedAt: "2026-05-16T00:00:00Z", VerificationHash: "sha256:abcdef0123456789", ProtocolVersion: "supermover/1"}, wantErr: "invalid pairing method"},
		{name: "invalid pairing receipt missing proof", doc: PairingReceipt{Version: CurrentVersion, ID: "pair1", ProfileID: "profile1", TargetID: "target1", SourceDeviceID: "sha256:abcdef0123456789", TargetDeviceID: "sha256:0123456789abcdef", DevicePublicKey: "sha256:0123456789abcdef", Method: "sas", VerifiedAt: "2026-05-16T00:00:00Z", ProtocolVersion: "supermover/1"}, wantErr: "verification phrase or hash is required"},
		{name: "invalid pairing receipt bad verified time", doc: PairingReceipt{Version: CurrentVersion, ID: "pair1", ProfileID: "profile1", TargetID: "target1", SourceDeviceID: "sha256:abcdef0123456789", TargetDeviceID: "sha256:0123456789abcdef", DevicePublicKey: "sha256:0123456789abcdef", Method: "sas", VerifiedAt: "not-a-time", VerificationHash: "sha256:abcdef0123456789", ProtocolVersion: "supermover/1"}, wantErr: "verified_at must be RFC3339"},
		{name: "invalid pairing receipt bad protocol", doc: PairingReceipt{Version: CurrentVersion, ID: "pair1", ProfileID: "profile1", TargetID: "target1", SourceDeviceID: "sha256:abcdef0123456789", TargetDeviceID: "sha256:0123456789abcdef", DevicePublicKey: "sha256:0123456789abcdef", Method: "sas", VerifiedAt: "2026-05-16T00:00:00Z", VerificationHash: "sha256:abcdef0123456789", ProtocolVersion: "one"}, wantErr: "protocol version"},
		{name: "invalid recovery", doc: RecoveryState{Version: CurrentVersion, Status: "unknown", UpdatedAt: "2026-05-16T00:00:00Z"}, wantErr: "status must be one of"},
		{name: "invalid manifest entry", doc: Manifest{Version: CurrentVersion, ID: "m1", SessionID: "s1", CreatedAt: "2026-05-16T00:00:00Z", Entries: []ManifestEntry{{Kind: "file"}}}, wantErr: "entries[0].path is required"},
		{name: "invalid symlink manifest entry", doc: Manifest{Version: CurrentVersion, ID: "m1", SessionID: "s1", CreatedAt: "2026-05-16T00:00:00Z", Entries: []ManifestEntry{{Path: "link", Kind: "symlink"}}}, wantErr: "entries[0].symlink_target is required"},
		{name: "unsafe symlink manifest entry", doc: Manifest{Version: CurrentVersion, ID: "m1", SessionID: "s1", CreatedAt: "2026-05-16T00:00:00Z", Entries: []ManifestEntry{{Path: "link", Kind: "symlink", SymlinkTarget: "../outside"}}}, wantErr: "entries[0].symlink_target is unsafe"},
		{name: "invalid manifest partial previous evidence", doc: Manifest{Version: CurrentVersion, ID: "m1", SessionID: "s1", CreatedAt: "2026-05-16T00:00:00Z", Entries: []ManifestEntry{{Path: "file", Kind: "file", PreviousSessionID: "s0", PreviousDigest: "sha256:abc"}}}, wantErr: "previous evidence must include"},
		{name: "invalid manifest unsupported previous digest", doc: Manifest{Version: CurrentVersion, ID: "m1", SessionID: "s1", CreatedAt: "2026-05-16T00:00:00Z", Entries: []ManifestEntry{{Path: "file", Kind: "file", PreviousSessionID: "s0", PreviousManifestID: "m0", PreviousDigest: "md5:abc"}}}, wantErr: "previous_digest must be sha256 hex"},
		{name: "invalid manifest short previous digest", doc: Manifest{Version: CurrentVersion, ID: "m1", SessionID: "s1", CreatedAt: "2026-05-16T00:00:00Z", Entries: []ManifestEntry{{Path: "file", Kind: "file", PreviousSessionID: "s0", PreviousManifestID: "m0", PreviousDigest: "sha256:abc"}}}, wantErr: "previous_digest must be sha256 hex"},
		{name: "invalid manifest previous evidence without previous_size", doc: manifestWithPreviousEvidence(func(entry *ManifestEntry) { entry.previousSizePresent = false }), wantErr: "previous evidence must include previous_size"},
		{name: "invalid manifest previous evidence without previous_mode", doc: manifestWithPreviousEvidence(func(entry *ManifestEntry) {
			entry.PreviousMode = 0
			entry.previousModePresent = false
		}), wantErr: "previous evidence must include previous_mode"},
		{name: "invalid manifest previous evidence without modtime", doc: manifestWithPreviousEvidence(func(entry *ManifestEntry) { entry.PreviousModTime = "" }), wantErr: "previous evidence must include previous_mod_time"},
		{name: "invalid manifest malformed previous modtime", doc: manifestWithPreviousEvidence(func(entry *ManifestEntry) { entry.PreviousModTime = "not-a-time" }), wantErr: "previous_mod_time must be RFC3339 timestamp"},
		{name: "invalid manifest metadata without previous evidence", doc: Manifest{Version: CurrentVersion, ID: "m1", SessionID: "s1", CreatedAt: "2026-05-16T00:00:00Z", Entries: []ManifestEntry{{Path: "file", Kind: "file", PreviousMode: 0o644}}}, wantErr: "previous metadata requires previous evidence"},
		{name: "invalid manifest previous evidence on directory", doc: Manifest{Version: CurrentVersion, ID: "m1", SessionID: "s1", CreatedAt: "2026-05-16T00:00:00Z", Entries: []ManifestEntry{{Path: "dir", Kind: "dir", PreviousSessionID: "s0", PreviousManifestID: "m0", PreviousDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}, wantErr: "previous evidence is only valid for file entries"},
		{name: "invalid warning missing session", doc: Warning{Version: CurrentVersion, ID: "w1", Severity: "warning", Code: "c", Message: "m", Paths: []string{"p"}, CreatedAt: "2026-05-16T00:00:00Z"}, wantErr: "session_id is required"},
		{name: "invalid warning missing severity", doc: Warning{Version: CurrentVersion, ID: "w1", SessionID: "s1", Code: "c", Message: "m", Paths: []string{"p"}, CreatedAt: "2026-05-16T00:00:00Z"}, wantErr: "severity is required"},
		{name: "invalid warning missing paths", doc: Warning{Version: CurrentVersion, ID: "w1", SessionID: "s1", Severity: "warning", Code: "c", Message: "m", CreatedAt: "2026-05-16T00:00:00Z"}, wantErr: "paths must contain"},
		{name: "invalid target drift unsafe path", doc: TargetDrift{Version: CurrentVersion, ID: "d1", SessionID: "s1", ProfileID: "p1", TargetID: "t1", RootID: "root", Path: "../file", DetectedAt: "2026-05-16T00:00:00Z", Change: "content_mismatch"}, wantErr: "path is unsafe"},
		{name: "invalid target drift reserved path", doc: TargetDrift{Version: CurrentVersion, ID: "d1", SessionID: "s1", ProfileID: "p1", TargetID: "t1", RootID: "root", Path: ".supermover/drift/file.json", DetectedAt: "2026-05-16T00:00:00Z", Change: "content_mismatch"}, wantErr: "reserved control directory"},
		{name: "invalid target drift unsafe id", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.ID = "../drift"
			return doc
		}(), wantErr: "id is unsafe"},
		{name: "invalid target drift bad timestamp", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.DetectedAt = "not-a-time"
			return doc
		}(), wantErr: "detected_at must be RFC3339"},
		{name: "invalid target drift unsupported review state", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.ReviewState = "ignored"
			return doc
		}(), wantErr: "review_state must be one of"},
		{name: "invalid target drift bad reviewed timestamp", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.ReviewedAt = "not-a-time"
			return doc
		}(), wantErr: "reviewed_at must be RFC3339"},
		{name: "invalid target drift unsupported review action", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.ReviewAction = "ignore"
			return doc
		}(), wantErr: "review_action must be one of"},
		{name: "invalid target drift review action without timestamp", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.ReviewState = "acknowledged"
			doc.ReviewAction = "acknowledge"
			doc.ReviewReason = "reviewed"
			return doc
		}(), wantErr: "reviewed_at is required when review_action is present"},
		{name: "invalid target drift review action without reason", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.ReviewState = "acknowledged"
			doc.ReviewAction = "acknowledge"
			doc.ReviewedAt = "2026-05-19T00:00:00Z"
			return doc
		}(), wantErr: "review_reason is required when review_action is present"},
		{name: "invalid target drift acknowledge action state mismatch", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.ReviewState = "resolved"
			doc.ReviewAction = "acknowledge"
			doc.ReviewedAt = "2026-05-19T00:00:00Z"
			doc.ReviewReason = "reviewed"
			return doc
		}(), wantErr: `review_state must be "acknowledged"`},
		{name: "invalid target drift resolved state without resolve action", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.ReviewState = "resolved"
			return doc
		}(), wantErr: `review_action "resolve" is required`},
		{name: "invalid target drift evidence without action", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.ReviewedAt = "2026-05-19T00:00:00Z"
			doc.ReviewReason = "reviewed"
			return doc
		}(), wantErr: "review_action is required when review evidence is present"},
		{name: "invalid target drift reviewer without action", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.ReviewedBy = "ops"
			return doc
		}(), wantErr: "review_action is required when review evidence is present"},
		{name: "invalid target drift partial expected evidence", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.Expected.ManifestID = ""
			return doc
		}(), wantErr: "expected.manifest_id is required"},
		{name: "invalid target drift unsafe expected path", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.Expected.Path = ".supermover/file"
			return doc
		}(), wantErr: "expected.path is unsafe"},
		{name: "invalid target drift bad expected digest", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.Expected.Digest = "md5:abc"
			return doc
		}(), wantErr: "expected.digest must be sha256 hex"},
		{name: "invalid target drift negative observed size", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.Observed.SetSizeEvidence(-1)
			return doc
		}(), wantErr: "observed.size cannot be negative"},
		{name: "invalid target drift unsafe observed path", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.Observed.Path = "../file"
			return doc
		}(), wantErr: "observed.path is unsafe"},
		{name: "invalid target drift unsafe observed symlink", doc: func() TargetDrift {
			doc := validTargetDrift()
			doc.Observed.Kind = "symlink"
			doc.Observed.SymlinkTarget = ".supermover/file"
			return doc
		}(), wantErr: "observed.symlink_target is unsafe"},
		{name: "invalid target drift observed false not missing", doc: func() TargetDrift {
			doc := validTargetDrift()
			present := false
			doc.Observed.Present = &present
			doc.Observed.Kind = "file"
			return doc
		}(), wantErr: "observed.kind must be missing"},
		{name: "invalid soft delete missing profile", doc: SoftDelete{Version: CurrentVersion, ID: "d1", SessionID: "s1", TargetID: "t1", RootID: "root", PreviousSessionID: "s0", PreviousManifestID: "m0", SourcePath: "a", TargetPath: "a", Kind: "file", DetectedAt: "2026-05-16T00:00:00Z"}, wantErr: "profile_id is required"},
		{name: "invalid soft delete missing previous evidence", doc: SoftDelete{Version: CurrentVersion, ID: "d1", SessionID: "s1", ProfileID: "p1", TargetID: "t1", RootID: "root", SourcePath: "a", TargetPath: "a", Kind: "file", DetectedAt: "2026-05-16T00:00:00Z"}, wantErr: "previous_session_id is required"},
		{name: "invalid soft delete unsafe source path", doc: func() SoftDelete {
			doc := validSoftDelete()
			doc.SourcePath = "../escape"
			return doc
		}(), wantErr: "source_path is unsafe"},
		{name: "invalid soft delete reserved target path", doc: func() SoftDelete {
			doc := validSoftDelete()
			doc.TargetPath = ".supermover/sessions/forged/receipt.json"
			return doc
		}(), wantErr: "target_path is unsafe"},
		{name: "invalid soft delete short digest", doc: func() SoftDelete {
			doc := validSoftDelete()
			doc.Digest = "sha256:abc"
			return doc
		}(), wantErr: "digest must be sha256 hex"},
		{name: "valid soft delete symlink target evidence", doc: func() SoftDelete {
			doc := validSoftDelete()
			doc.Kind = "symlink"
			doc.Size = 0
			doc.Digest = ""
			doc.SymlinkTarget = "notes/a-target.md"
			return doc
		}()},
		{name: "invalid soft delete unsafe symlink target", doc: func() SoftDelete {
			doc := validSoftDelete()
			doc.Kind = "symlink"
			doc.Size = 0
			doc.Digest = ""
			doc.SymlinkTarget = "../outside"
			return doc
		}(), wantErr: "symlink_target is unsafe"},
		{name: "invalid soft delete symlink target on file", doc: func() SoftDelete {
			doc := validSoftDelete()
			doc.SymlinkTarget = "notes/a-target.md"
			return doc
		}(), wantErr: "symlink_target is only valid for symlink soft deletes"},
		{name: "invalid prune approval unsafe id", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.ID = "../approval"
			return doc
		}(), wantErr: "id is unsafe"},
		{name: "invalid prune approval missing item", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Items = nil
			return doc
		}(), wantErr: "items must contain at least one approved soft delete"},
		{name: "invalid prune approval policy without physical opt in", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.ProfileDeletePolicy.AllowPhysicalPrune = false
			return doc
		}(), wantErr: "profile_delete_policy.allow_physical_prune must be true"},
		{name: "valid refused prune approval preserves refusal without full evidence", doc: PruneApproval{
			Version:       CurrentVersion,
			ID:            "approval-refused",
			ProfileID:     "profile1",
			TargetID:      "target1",
			RootID:        "root",
			CreatedAt:     "2026-05-16T00:01:00Z",
			ReviewTool:    "prune dry-run",
			Status:        "refused",
			RefusalReason: "delete_policy.allow_physical_prune is false",
		}},
		{name: "valid superseded prune approval preserves approval plus supersede metadata", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Status = "superseded"
			doc.RefusalReason = "replaced by newer approval"
			doc.SupersededBy = "reviewer@example.com"
			doc.SupersededAt = "2026-05-16T00:03:00Z"
			return doc
		}()},
		{name: "invalid prune approval approved before created", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.ApprovedAt = "2026-05-15T00:00:00Z"
			return doc
		}(), wantErr: "approved_at must be greater than or equal to created_at"},
		{name: "invalid prune approval expires before approval", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.ExpiresAt = "2026-05-16T00:01:30Z"
			return doc
		}(), wantErr: "expires_at must be greater than or equal to approved_at"},
		{name: "invalid prune approval bad soft delete ref", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Items[0].SoftDeleteRef = "deleted/other.json"
			return doc
		}(), wantErr: "items[0].soft_delete_ref must match soft_delete_id"},
		{name: "invalid prune approval reserved target path", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Items[0].TargetPath = ".supermover/deleted/forged.json"
			return doc
		}(), wantErr: "items[0].target_path is unsafe"},
		{name: "valid prune approval symlink target evidence", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Items[0].Kind = "symlink"
			doc.Items[0].Size = 0
			doc.Items[0].Digest = ""
			doc.Items[0].SymlinkTarget = "notes/a-target.md"
			return doc
		}()},
		{name: "invalid prune approval missing symlink target", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Items[0].Kind = "symlink"
			doc.Items[0].Size = 0
			doc.Items[0].Digest = ""
			return doc
		}(), wantErr: "items[0].symlink_target is required"},
		{name: "invalid prune approval unsafe symlink target", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Items[0].Kind = "symlink"
			doc.Items[0].Size = 0
			doc.Items[0].Digest = ""
			doc.Items[0].SymlinkTarget = "../outside"
			return doc
		}(), wantErr: "items[0].symlink_target is unsafe"},
		{name: "invalid prune approval symlink target on file", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Items[0].SymlinkTarget = "notes/a-target.md"
			return doc
		}(), wantErr: "items[0].symlink_target is only valid for symlink approvals"},
		{name: "invalid prune approval refused without reason", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Status = "refused"
			return doc
		}(), wantErr: "refusal_reason is required"},
		{name: "invalid prune approval superseded without supersede reviewer", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Status = "superseded"
			doc.RefusalReason = "replaced by newer approval"
			doc.SupersededAt = "2026-05-16T00:03:00Z"
			doc.SupersededBy = ""
			return doc
		}(), wantErr: "superseded_by"},
		{name: "invalid prune approval superseded without superseded_at", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Status = "superseded"
			doc.RefusalReason = "replaced by newer approval"
			doc.SupersededBy = "reviewer@example.com"
			doc.SupersededAt = ""
			return doc
		}(), wantErr: "superseded_at"},
		{name: "invalid prune approval superseded before approval", doc: func() PruneApproval {
			doc := validPruneApproval()
			doc.Status = "superseded"
			doc.RefusalReason = "replaced by newer approval"
			doc.SupersededBy = "reviewer@example.com"
			doc.SupersededAt = "2026-05-16T00:01:30Z"
			return doc
		}(), wantErr: "superseded_at must be greater than or equal to approved_at"},
		{name: "invalid prune receipt dry run applied", doc: func() PruneReceipt {
			doc := validPruneReceipt()
			doc.Status = PruneReceiptApplied
			return doc
		}(), wantErr: "dry_run receipts must use status planned"},
		{name: "invalid prune receipt unsafe observed path", doc: func() PruneReceipt {
			doc := validPruneReceipt()
			doc.Items[0].PrePruneObserved.Path = "../escape"
			return doc
		}(), wantErr: "items[0].pre_prune_observed.path is unsafe"},
		{name: "invalid prune receipt refused without error code", doc: func() PruneReceipt {
			doc := validPruneReceipt()
			doc.Items[0].Result = "refused"
			doc.Items[0].ErrorCode = ""
			return doc
		}(), wantErr: "items[0].error_code is required"},
		{name: "invalid prune receipt apply would prune", doc: func() PruneReceipt {
			doc := validPruneReceipt()
			doc.DryRun = false
			doc.Status = PruneReceiptApplied
			doc.EndedAt = "2026-05-16T00:05:00Z"
			return doc
		}(), wantErr: "items[0].result would_prune is only valid for dry_run or started apply receipt"},
		{name: "valid prune receipt started apply", doc: validStartedPruneReceipt()},
		{name: "invalid prune receipt started with ended_at", doc: validStartedPruneReceipt(func(doc *PruneReceipt) {
			doc.EndedAt = "2026-05-16T00:04:00Z"
		}), wantErr: "status started must not include ended_at"},
		{name: "valid prune receipt symlink intended action", doc: validAppliedPruneReceipt(func(doc *PruneReceipt) {
			doc.Items[0].IntendedAction = "delete_symlink"
		})},
		{name: "invalid prune receipt unknown intended action", doc: func() PruneReceipt {
			doc := validAppliedPruneReceipt()
			doc.Items[0].IntendedAction = "delete_directory"
			return doc
		}(), wantErr: "items[0].intended_action must be one of delete_file, delete_symlink"},
		{name: "invalid prune receipt pruned without observed evidence", doc: func() PruneReceipt {
			doc := validAppliedPruneReceipt()
			doc.Items[0].PrePruneObserved = PruneObservedTargetState{}
			return doc
		}(), wantErr: "items[0].pre_prune_observed is required when result is pruned"},
		{name: "invalid prune receipt pruned with missing observed target", doc: func() PruneReceipt {
			doc := validAppliedPruneReceipt()
			present := false
			doc.Items[0].PrePruneObserved = PruneObservedTargetState{
				Present: &present,
				Kind:    "missing",
				Path:    "notes/a.md",
			}
			return doc
		}(), wantErr: "items[0].pre_prune_observed.present must be true when result is pruned"},
		{name: "invalid prune receipt observed target present but missing kind", doc: func() PruneReceipt {
			doc := validPruneReceipt()
			present := true
			doc.Items[0].PrePruneObserved = PruneObservedTargetState{
				Present: &present,
				Kind:    "missing",
				Path:    "notes/a.md",
			}
			return doc
		}(), wantErr: "items[0].pre_prune_observed.kind cannot be missing when present is true"},
		{name: "invalid prune receipt failed status with pruned item", doc: func() PruneReceipt {
			doc := validAppliedPruneReceipt()
			doc.Status = PruneReceiptFailed
			return doc
		}(), wantErr: "status failed requires failed or refused results and no pruned results"},
		{name: "invalid prune receipt applied status with failed item", doc: func() PruneReceipt {
			doc := validFailedPruneReceipt()
			doc.Status = PruneReceiptApplied
			return doc
		}(), wantErr: "status applied requires at least one pruned result and no failed or refused results"},
		{name: "invalid prune receipt partial without pruned item", doc: validFailedPruneReceipt(func(doc *PruneReceipt) {
			doc.Status = PruneReceiptPartial
		}), wantErr: "status partial requires pruned result plus failed or refused result"},
		{name: "invalid prune receipt apply missing ended_at", doc: func() PruneReceipt {
			doc := validAppliedPruneReceipt()
			doc.EndedAt = ""
			return doc
		}(), wantErr: "ended_at is required for apply receipts"},
		{name: "invalid prune receipt pruned_at on refused item", doc: validFailedPruneReceipt(func(doc *PruneReceipt) {
			doc.Items[0].PrunedAt = "2026-05-16T00:04:00Z"
		}), wantErr: "items[0].pruned_at is only valid when result is pruned"},
		{name: "invalid snapshot payload", doc: ProfileSnapshot{Version: CurrentVersion, ID: "snap1", ProfileID: "p1", CapturedAt: "2026-05-16T00:00:00Z", Profile: []byte(`{`)}, wantErr: "profile must contain valid JSON"},
		{name: "invalid profile snapshot dot session", doc: ProfileSnapshot{Version: CurrentVersion, ID: "snap1", ProfileID: "p1", SessionID: ".", CapturedAt: "2026-05-16T00:00:00Z", Profile: []byte(`{"version":1}`)}, wantErr: "session_id is invalid"},
		{name: "invalid manifest entry dot previous session", doc: manifestWithPreviousEvidence(func(entry *ManifestEntry) { entry.PreviousSessionID = "." }), wantErr: "previous_session_id is invalid"},
		{name: "invalid session receipt dot id", doc: SessionReceipt{Version: CurrentVersion, ID: ".", ProfileID: "p1", TargetID: "t1", StartedAt: "2026-05-16T00:00:00Z", Status: "published"}, wantErr: "id is invalid"},
		{name: "invalid manifest dot session", doc: Manifest{Version: CurrentVersion, ID: "m1", SessionID: ".", CreatedAt: "2026-05-16T00:00:00Z"}, wantErr: "session_id is invalid"},
		{name: "invalid warning dot session", doc: Warning{Version: CurrentVersion, ID: "w1", SessionID: ".", Code: "c", Message: "m", Severity: "warning", Paths: []string{"p"}, CreatedAt: "2026-05-16T00:00:00Z"}, wantErr: "session_id is invalid"},
		{name: "invalid target drift dot session", doc: TargetDrift{Version: CurrentVersion, ID: "d1", SessionID: ".", ProfileID: "p1", TargetID: "t1", RootID: "root", Path: "file", DetectedAt: "2026-05-16T00:00:00Z", Change: "content_mismatch"}, wantErr: "session_id is invalid"},
		{name: "invalid soft delete dot session", doc: SoftDelete{Version: CurrentVersion, ID: "d1", SessionID: ".", ProfileID: "p1", TargetID: "t1", RootID: "root", PreviousSessionID: "s0", PreviousManifestID: "m0", SourcePath: "a", TargetPath: "a", Kind: "file", DetectedAt: "2026-05-16T00:00:00Z"}, wantErr: "session_id is invalid"},
		{name: "invalid soft delete dot previous session", doc: SoftDelete{Version: CurrentVersion, ID: "d1", SessionID: "s1", ProfileID: "p1", TargetID: "t1", RootID: "root", PreviousSessionID: ".", PreviousManifestID: "m0", SourcePath: "a", TargetPath: "a", Kind: "file", DetectedAt: "2026-05-16T00:00:00Z"}, wantErr: "previous_session_id is invalid"},
		{name: "invalid history dot latest", doc: HistoryIndex{Version: CurrentVersion, UpdatedAt: "2026-05-16T00:00:00Z", Latest: "."}, wantErr: "latest is invalid"},
		{name: "invalid history dot session", doc: HistoryIndex{Version: CurrentVersion, UpdatedAt: "2026-05-16T00:00:00Z", Sessions: []HistoryEntry{{SessionID: ".", StartedAt: "2026-05-16T00:00:00Z", ReceiptRef: "sessions/./receipt.json"}}}, wantErr: "sessions[0].session_id is invalid"},
		{name: "invalid history mismatched receipt ref", doc: HistoryIndex{Version: CurrentVersion, UpdatedAt: "2026-05-16T00:00:00Z", Sessions: []HistoryEntry{{SessionID: "session1", StartedAt: "2026-05-16T00:00:00Z", ReceiptRef: "sessions/./receipt.json"}}}, wantErr: "sessions[0].receipt_ref must match session_id"},
		{name: "invalid recovery dot session", doc: RecoveryState{Version: CurrentVersion, Status: RecoveryInterrupted, SessionID: ".", UpdatedAt: "2026-05-16T00:00:00Z"}, wantErr: "session_id is invalid"},
		{name: "invalid network transfer status", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.Status = "unknown"
			return doc
		}(), wantErr: "status must be one of"},
		{name: "invalid network transfer bad source device", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.SourceDeviceID = "bad"
			return doc
		}(), wantErr: "source_device_id is invalid"},
		{name: "invalid network transfer equal devices", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.TargetDeviceID = doc.SourceDeviceID
			return doc
		}(), wantErr: "source_device_id and target_device_id must differ"},
		{name: "invalid network transfer bad target id", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.TargetID = "bad target"
			return doc
		}(), wantErr: "target_id is invalid"},
		{name: "invalid network transfer bad protocol", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.ProtocolVersion = "one"
			return doc
		}(), wantErr: "protocol_version is invalid"},
		{name: "invalid network transfer missing attempts", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.Attempts = nil
			return doc
		}(), wantErr: "attempts must contain"},
		{name: "invalid network transfer updated before started", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.UpdatedAt = "2026-05-15T00:00:00Z"
			return doc
		}(), wantErr: "updated_at must be greater than or equal to started_at"},
		{name: "invalid network transfer stage", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.Stage = "publish"
			return doc
		}(), wantErr: "stage must be one of"},
		{name: "invalid network transfer attempt status", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.Attempts[0].Status = "unknown"
			return doc
		}(), wantErr: "attempts[0].status must be one of"},
		{name: "invalid network transfer attempt ended before started", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.Attempts[0].EndedAt = "2026-05-15T00:00:00Z"
			return doc
		}(), wantErr: "attempts[0].ended_at must be greater"},
		{name: "invalid network transfer attempt mismatch", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.Attempts[0].Status = NetworkTransferFailed
			return doc
		}(), wantErr: "last attempt status must match"},
		{name: "invalid network transfer dot session", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.SessionID = "."
			return doc
		}(), wantErr: "session_id is invalid"},
		{name: "invalid published level 2 network transfer missing privacy overhead", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.Status = NetworkTransferPublished
			doc.Stage = "commit"
			doc.ErrorCode = ""
			doc.Error = ""
			doc.Attempts[0].Status = NetworkTransferPublished
			doc.Attempts[0].Stage = "commit"
			doc.Attempts[0].ErrorCode = ""
			doc.Attempts[0].Error = ""
			return doc
		}(), wantErr: "privacy_overhead is required for published level 2 network transfer"},
		{name: "invalid network transfer empty privacy overhead", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.PrivacyOverhead = &NetworkTransferPrivacyOverhead{}
			return doc
		}(), wantErr: "privacy_overhead must contain applied overhead counters"},
		{name: "invalid network transfer impossible privacy overhead", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.PrivacyOverhead = &NetworkTransferPrivacyOverhead{FramePlainBytes: 10, FrameWireBytes: 5}
			return doc
		}(), wantErr: "frame_plain_bytes cannot exceed frame_wire_bytes"},
		{name: "valid network transfer jitter-only privacy overhead", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.PrivacyOverhead = &NetworkTransferPrivacyOverhead{
				JitteredRequests:   3,
				JitterBudgetMillis: 250,
			}
			return doc
		}()},
		{name: "invalid network transfer mismatched jitter overhead budget", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.PrivacyOverhead = &NetworkTransferPrivacyOverhead{
				JitteredRequests:   3,
				JitterBudgetMillis: 125,
			}
			return doc
		}(), wantErr: "privacy_overhead.jitter_budget_millis must match privacy_policy.jitter_budget_millis"},
		{name: "invalid network transfer jitter overhead without policy budget", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.PrivacyPolicy = transport.PrivacyPolicy{}
			doc.PrivacyOverhead = &NetworkTransferPrivacyOverhead{
				JitteredRequests:   3,
				JitterBudgetMillis: 250,
			}
			return doc
		}(), wantErr: "privacy_overhead.jitter_budget_millis requires privacy_policy.level 2"},
		{name: "invalid network transfer level 1 jitter overhead", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.PrivacyPolicy = transport.PrivacyPolicy{
				Level:        transport.PrivacyLevel1,
				JitterBudget: 250,
			}
			doc.PrivacyOverhead = &NetworkTransferPrivacyOverhead{
				JitteredRequests:   3,
				JitterBudgetMillis: 250,
			}
			return doc
		}(), wantErr: "privacy_overhead.jitter_budget_millis requires privacy_policy.level 2"},
		{name: "invalid network transfer level 3 jitter overhead", doc: func() NetworkTransfer {
			doc := validNetworkTransfer()
			doc.PrivacyPolicy = transport.PrivacyPolicy{
				Level:        transport.PrivacyLevel3,
				JitterBudget: 250,
			}
			doc.PrivacyOverhead = &NetworkTransferPrivacyOverhead{
				JitteredRequests:   3,
				JitterBudgetMillis: 250,
			}
			return doc
		}(), wantErr: "privacy_overhead.jitter_budget_millis requires privacy_policy.level 2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.doc.Validate()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Validate() error = nil, want %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestNetworkTransferPrivacyOverheadEmptyIncludesJitterEvidence(t *testing.T) {
	tests := []struct {
		name     string
		overhead NetworkTransferPrivacyOverhead
		want     bool
	}{
		{
			name: "empty",
			want: true,
		},
		{
			name: "jitter requests and budget only",
			overhead: NetworkTransferPrivacyOverhead{
				JitteredRequests:   1,
				JitterBudgetMillis: 250,
			},
		},
		{
			name: "jitter applied delay only",
			overhead: NetworkTransferPrivacyOverhead{
				JitteredRequests:   1,
				JitterDelayMillis:  25,
				JitterBudgetMillis: 250,
			},
		},
		{
			name: "jitter sampled zero delays",
			overhead: NetworkTransferPrivacyOverhead{
				JitteredRequests:   2,
				JitterBudgetMillis: 250,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.overhead.Empty(); got != tt.want {
				t.Fatalf("NetworkTransferPrivacyOverhead.Empty() = %t, want %t for %+v", got, tt.want, tt.overhead)
			}
		})
	}
}

func TestNetworkTransferPrivacyOverheadValidateJitter(t *testing.T) {
	tests := []struct {
		name     string
		overhead NetworkTransferPrivacyOverhead
		wantErr  string
	}{
		{
			name: "sampled zero delays records requests and budget",
			overhead: NetworkTransferPrivacyOverhead{
				JitteredRequests:   4,
				JitterBudgetMillis: 250,
			},
		},
		{
			name: "applied bounded jitter",
			overhead: NetworkTransferPrivacyOverhead{
				JitteredRequests:     4,
				JitterDelayMillis:    375,
				MaxJitterDelayMillis: 125,
				JitterBudgetMillis:   250,
			},
		},
		{
			name: "negative requests",
			overhead: NetworkTransferPrivacyOverhead{
				JitteredRequests: -1,
			},
			wantErr: "jittered_requests cannot be negative",
		},
		{
			name: "negative total delay",
			overhead: NetworkTransferPrivacyOverhead{
				JitterDelayMillis: -1,
			},
			wantErr: "jitter_delay_millis cannot be negative",
		},
		{
			name: "negative max delay",
			overhead: NetworkTransferPrivacyOverhead{
				MaxJitterDelayMillis: -1,
			},
			wantErr: "max_jitter_delay_millis cannot be negative",
		},
		{
			name: "negative budget",
			overhead: NetworkTransferPrivacyOverhead{
				JitterBudgetMillis: -1,
			},
			wantErr: "jitter_budget_millis cannot be negative",
		},
		{
			name: "delay requires requests",
			overhead: NetworkTransferPrivacyOverhead{
				JitterDelayMillis:  1,
				JitterBudgetMillis: 250,
			},
			wantErr: "jitter delay evidence requires jittered_requests",
		},
		{
			name: "delay requires budget",
			overhead: NetworkTransferPrivacyOverhead{
				JitteredRequests:  1,
				JitterDelayMillis: 1,
			},
			wantErr: "jitter delay evidence requires jitter_budget_millis",
		},
		{
			name: "max requires requests",
			overhead: NetworkTransferPrivacyOverhead{
				MaxJitterDelayMillis: 1,
				JitterBudgetMillis:   250,
			},
			wantErr: "jitter delay evidence requires jittered_requests",
		},
		{
			name: "requests require budget",
			overhead: NetworkTransferPrivacyOverhead{
				JitteredRequests: 1,
			},
			wantErr: "jitter overhead evidence requires jitter_budget_millis",
		},
		{
			name: "budget requires requests",
			overhead: NetworkTransferPrivacyOverhead{
				JitterBudgetMillis: 250,
			},
			wantErr: "jitter overhead evidence requires jittered_requests",
		},
		{
			name: "max cannot exceed budget",
			overhead: NetworkTransferPrivacyOverhead{
				JitteredRequests:     1,
				MaxJitterDelayMillis: 251,
				JitterBudgetMillis:   250,
			},
			wantErr: "max_jitter_delay_millis cannot exceed jitter_budget_millis",
		},
		{
			name: "total delay cannot exceed request budget",
			overhead: NetworkTransferPrivacyOverhead{
				JitteredRequests:   2,
				JitterDelayMillis:  501,
				JitterBudgetMillis: 250,
			},
			wantErr: "jitter_delay_millis cannot exceed jittered_requests * jitter_budget_millis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.overhead.Validate()
			if tt.wantErr == "" && err != nil {
				t.Fatalf("NetworkTransferPrivacyOverhead.Validate() error = %v, want nil", err)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NetworkTransferPrivacyOverhead.Validate() error = nil, want %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NetworkTransferPrivacyOverhead.Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestReadWriteRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := validManifest()

	if err := Write(&buf, want); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, err := Read[Manifest](&buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got.ID != want.ID {
		t.Fatalf("Read() id = %q, want %q", got.ID, want.ID)
	}
	if len(got.Entries) != 1 || got.Entries[0].Path != "notes/a.md" {
		t.Fatalf("Read() entries = %#v, want notes/a.md entry", got.Entries)
	}
}

func TestManifestEntryRoundTripPreservesZeroEvidenceFields(t *testing.T) {
	want := validManifest()
	want.Entries = []ManifestEntry{{Path: "secret.txt", Kind: "file", ModTime: "2026-05-16T00:00:00Z", Digest: "sha256:abc"}}
	want.Entries[0].SetModeEvidence(0)
	want.Entries[0].SetSizeEvidence(0)
	want.Entries[0].PreviousSessionID = "previous"
	want.Entries[0].PreviousManifestID = "previous-manifest"
	want.Entries[0].PreviousDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	want.Entries[0].PreviousModTime = "2026-05-15T00:00:00Z"
	want.Entries[0].SetPreviousSizeEvidence(0)
	want.Entries[0].SetPreviousModeEvidence(0)

	var buf bytes.Buffer
	if err := Write(&buf, want); err != nil {
		t.Fatalf("Write(zero evidence manifest) error = %v, want nil", err)
	}
	payload := buf.String()
	for _, wantField := range []string{`"mode": 0`, `"size": 0`, `"previous_mode": 0`, `"previous_size": 0`} {
		if !strings.Contains(payload, wantField) {
			t.Fatalf("Write(zero evidence manifest) payload = %s, want field %s", payload, wantField)
		}
	}

	got, err := Read[Manifest](strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Read(zero evidence manifest) error = %v, want nil", err)
	}
	entry := got.Entries[0]
	if !entry.HasModeEvidence() || entry.Mode != 0 {
		t.Fatalf("Read(zero evidence).mode = (%v, %d), want present 0", entry.HasModeEvidence(), entry.Mode)
	}
	if !entry.HasSizeEvidence() || entry.Size != 0 {
		t.Fatalf("Read(zero evidence).size = (%v, %d), want present 0", entry.HasSizeEvidence(), entry.Size)
	}
	if !entry.HasPreviousModeEvidence() || entry.PreviousMode != 0 {
		t.Fatalf("Read(zero evidence).previous_mode = (%v, %d), want present 0", entry.HasPreviousModeEvidence(), entry.PreviousMode)
	}
	if !entry.HasPreviousSizeEvidence() || entry.PreviousSize != 0 {
		t.Fatalf("Read(zero evidence).previous_size = (%v, %d), want present 0", entry.HasPreviousSizeEvidence(), entry.PreviousSize)
	}
}

func TestTargetDriftRoundTripPreservesZeroEvidenceFields(t *testing.T) {
	present := true
	want := validTargetDrift()
	want.Expected.SetSizeEvidence(0)
	want.Expected.SetModeEvidence(0)
	want.Observed.Present = &present
	want.Observed.Kind = "file"
	want.Observed.Path = "notes/a.md"
	want.Observed.SetSizeEvidence(0)
	want.Observed.SetModeEvidence(0)

	var buf bytes.Buffer
	if err := Write(&buf, want); err != nil {
		t.Fatalf("Write(zero evidence target drift) error = %v, want nil", err)
	}
	payload := buf.String()
	for _, wantField := range []string{`"size": 0`, `"mode": 0`} {
		if !strings.Contains(payload, wantField) {
			t.Fatalf("Write(zero evidence target drift) payload = %s, want field %s", payload, wantField)
		}
	}

	got, err := Read[TargetDrift](strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Read(zero evidence target drift) error = %v, want nil", err)
	}
	if !got.Expected.HasSizeEvidence() || got.Expected.Size != 0 {
		t.Fatalf("Read(zero evidence).expected.size = (%v, %d), want present 0", got.Expected.HasSizeEvidence(), got.Expected.Size)
	}
	if !got.Expected.HasModeEvidence() || got.Expected.Mode != 0 {
		t.Fatalf("Read(zero evidence).expected.mode = (%v, %d), want present 0", got.Expected.HasModeEvidence(), got.Expected.Mode)
	}
	if !got.Observed.HasSizeEvidence() || got.Observed.Size != 0 {
		t.Fatalf("Read(zero evidence).observed.size = (%v, %d), want present 0", got.Observed.HasSizeEvidence(), got.Observed.Size)
	}
	if !got.Observed.HasModeEvidence() || got.Observed.Mode != 0 {
		t.Fatalf("Read(zero evidence).observed.mode = (%v, %d), want present 0", got.Observed.HasModeEvidence(), got.Observed.Mode)
	}
}

func TestTargetDriftRoundTripPreservesDirectNonZeroEvidenceFields(t *testing.T) {
	present := true
	want := validTargetDrift()
	want.Expected.Size = 123
	want.Expected.Mode = 0o600
	want.Observed.Present = &present
	want.Observed.Kind = "file"
	want.Observed.Path = "notes/a.md"
	want.Observed.Size = 456
	want.Observed.Mode = 0o644

	var buf bytes.Buffer
	if err := Write(&buf, want); err != nil {
		t.Fatalf("Write(non-zero evidence target drift) error = %v, want nil", err)
	}
	payload := buf.String()
	for _, wantField := range []string{`"size": 123`, `"mode": 384`, `"size": 456`, `"mode": 420`} {
		if !strings.Contains(payload, wantField) {
			t.Fatalf("Write(non-zero evidence target drift) payload = %s, want field %s", payload, wantField)
		}
	}

	got, err := Read[TargetDrift](strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Read(non-zero evidence target drift) error = %v, want nil", err)
	}
	if !got.Expected.HasSizeEvidence() || got.Expected.Size != 123 || !got.Expected.HasModeEvidence() || got.Expected.Mode != 0o600 {
		t.Fatalf("Read(non-zero evidence).expected = %+v, want size/mode evidence", got.Expected)
	}
	if !got.Observed.HasSizeEvidence() || got.Observed.Size != 456 || !got.Observed.HasModeEvidence() || got.Observed.Mode != 0o644 {
		t.Fatalf("Read(non-zero evidence).observed = %+v, want size/mode evidence", got.Observed)
	}
}

func TestTargetDriftLegacyChangeRemainsValid(t *testing.T) {
	doc := validTargetDrift()
	doc.Change = "modified"

	if err := doc.Validate(); err != nil {
		t.Fatalf("TargetDrift.Validate(legacy change) error = %v, want nil", err)
	}
}

func TestTargetDriftExpectedSymlinkTargetRoundTrip(t *testing.T) {
	doc := validTargetDrift()
	doc.Path = "links/current"
	doc.Change = "symlink_mismatch"
	doc.Expected = TargetDriftExpectedState{
		SessionID:     "session1",
		ManifestID:    "manifest-session1",
		Kind:          "symlink",
		Path:          "links/current",
		SymlinkTarget: "expected-target",
	}
	doc.Observed = TargetDriftObservedState{
		Present:       doc.Observed.Present,
		Kind:          "symlink",
		Path:          "links/current",
		SymlinkTarget: "actual-target",
	}

	var buf bytes.Buffer
	if err := Write(&buf, doc); err != nil {
		t.Fatalf("Write(symlink target drift) error = %v, want nil", err)
	}
	payload := buf.String()
	if !strings.Contains(payload, `"symlink_target": "expected-target"`) {
		t.Fatalf("Write(symlink target drift) payload = %s, want expected symlink_target", payload)
	}

	got, err := Read[TargetDrift](strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Read(symlink target drift) error = %v, want nil", err)
	}
	if got.Expected.SymlinkTarget != "expected-target" || got.Observed.SymlinkTarget != "actual-target" {
		t.Fatalf("Read(symlink target drift) expected=%+v observed=%+v, want symlink targets preserved", got.Expected, got.Observed)
	}
}

func TestTargetDriftExpectedSymlinkTargetValidation(t *testing.T) {
	tests := []struct {
		name string
		edit func(*TargetDrift)
		want string
	}{
		{
			name: "unsafe target",
			edit: func(doc *TargetDrift) {
				doc.Expected = TargetDriftExpectedState{
					SessionID:     "session1",
					ManifestID:    "manifest-session1",
					Kind:          "symlink",
					Path:          "links/current",
					SymlinkTarget: "../outside",
				}
			},
			want: "expected.symlink_target is unsafe",
		},
		{
			name: "non symlink kind",
			edit: func(doc *TargetDrift) {
				doc.Expected.SymlinkTarget = "elsewhere"
			},
			want: "expected.symlink_target is only valid for symlink entries",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validTargetDrift()
			tt.edit(&doc)
			err := doc.Validate()
			if err == nil {
				t.Fatalf("TargetDrift.Validate() error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("TargetDrift.Validate() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestReadTargetDriftRejectsUnknownNestedField(t *testing.T) {
	raw := `{"version":1,"id":"drift1","session_id":"session1","profile_id":"profile1","target_id":"target1","root_id":"root","path":"notes/a.md","detected_at":"2026-05-16T00:00:00Z","change":"content_mismatch","expected":{"session_id":"session0","manifest_id":"manifest-session0","kind":"file","path":"notes/a.md","unexpected":true}}`

	_, err := Read[TargetDrift](strings.NewReader(raw))
	if err == nil {
		t.Fatalf("Read(target drift unknown nested field) error = nil, want unknown field error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Read(target drift unknown nested field) error = %q, want unknown field error", err.Error())
	}
}

func TestReadTargetDriftRejectsTrailingNestedJSON(t *testing.T) {
	raw := `{"version":1,"id":"drift1","session_id":"session1","profile_id":"profile1","target_id":"target1","root_id":"root","path":"notes/a.md","detected_at":"2026-05-16T00:00:00Z","change":"content_mismatch","expected":{"session_id":"session0","manifest_id":"manifest-session0","kind":"file","path":"notes/a.md"}{"ignored":true}}`

	_, err := Read[TargetDrift](strings.NewReader(raw))
	if err == nil {
		t.Fatalf("Read(target drift trailing nested JSON) error = nil, want malformed JSON error")
	}
	if !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("Read(target drift trailing nested JSON) error = %q, want malformed JSON error", err.Error())
	}
}

func TestReadRejectsTrailingJSONDocument(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, validWarning()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	buf.WriteString(`{"ignored":true}`)

	_, err := Read[Warning](&buf)
	if err == nil {
		t.Fatalf("Read() error = nil, want trailing JSON error")
	}
	if !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("Read() error = %q, want trailing JSON error", err.Error())
	}
}

func TestWriteFileRejectsControlPathSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, DirName)); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	doc := validManifest()
	path, err := Path(root, ArtifactManifest, doc.SessionID)
	if err != nil {
		t.Fatalf("Path(%q, %q) error = %v, want nil", ArtifactManifest, doc.SessionID, err)
	}

	if err := WriteFile(path, doc); err == nil {
		t.Fatalf("WriteFile(%q, manifest) error = nil, want symlink control path error", path)
	}
	if _, err := os.Stat(filepath.Join(outside, "sessions", doc.SessionID, "manifest.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(outside manifest) error = %v, want os.ErrNotExist", err)
	}
}

func TestWriteNewFileRejectsExistingArtifact(t *testing.T) {
	root := t.TempDir()
	doc := validPairingReceipt()
	path, err := Path(root, ArtifactPairingReceipt, doc.ID)
	if err != nil {
		t.Fatalf("Path(pairing receipt) error = %v, want nil", err)
	}
	if err := WriteNewFile(path, doc); err != nil {
		t.Fatalf("WriteNewFile(first) error = %v, want nil", err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	replacement := doc
	replacement.VerificationHash = "sha256:fedcba9876543210"

	err = WriteNewFile(path, replacement)

	if err == nil {
		t.Fatalf("WriteNewFile(existing) error = nil, want already exists")
	}
	if !errors.Is(err, ErrArtifactExists) {
		t.Fatalf("WriteNewFile(existing) error = %v, want ErrArtifactExists", err)
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("WriteNewFile(existing) error = %v, want already exists", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) after rejection error = %v, want nil", path, err)
	}
	if string(got) != string(original) {
		t.Fatalf("WriteNewFile(existing) changed artifact\n got: %s\nwant: %s", string(got), string(original))
	}
}

func TestReadManifestCompatRejectsTrailingJSONDocument(t *testing.T) {
	input := `{"version":1,"id":"manifest1","session_id":"session1","created_at":"2026-05-16T00:00:00Z","entries":[{"path":"link","kind":"symlink","target_path":"link"}]}
{"ignored":true}`

	_, err := ReadManifestCompat(strings.NewReader(input))
	if err == nil {
		t.Fatalf("ReadManifestCompat() error = nil, want trailing JSON error")
	}
	if !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("ReadManifestCompat() error = %q, want trailing JSON error", err.Error())
	}
}

func TestReadManifestCompatAllowsLegacySymlinkTarget(t *testing.T) {
	input := `{"version":1,"id":"manifest1","session_id":"session1","created_at":"2026-05-16T00:00:00Z","entries":[{"path":"link","kind":"symlink","target_path":"link"}]}`

	if _, err := Read[Manifest](strings.NewReader(input)); err == nil {
		t.Fatalf("Read[Manifest](legacy symlink) error = nil, want strict symlink target error")
	}
	got, err := ReadManifestCompat(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadManifestCompat(legacy symlink) error = %v, want nil", err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Path != "link" {
		t.Fatalf("ReadManifestCompat(legacy symlink).Entries = %#v, want link entry", got.Entries)
	}
}

func TestReadManifestCompatRejectsUnsafeSymlinkTarget(t *testing.T) {
	input := `{"version":1,"id":"manifest1","session_id":"session1","created_at":"2026-05-16T00:00:00Z","entries":[{"path":"link","kind":"symlink","target_path":"link","symlink_target":"../outside"}]}`

	if _, err := ReadManifestCompat(strings.NewReader(input)); err == nil {
		t.Fatalf("ReadManifestCompat(unsafe symlink) error = nil, want unsafe symlink target error")
	}
}

func TestReadRejectsUnknownFields(t *testing.T) {
	input := `{"version":1,"id":"w1","code":"privacy","message":"warning","created_at":"2026-05-16T00:00:00Z","extra":true}`

	_, err := Read[Warning](strings.NewReader(input))
	if err == nil {
		t.Fatalf("Read() error = nil, want unknown field error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Read() error = %q, want unknown field error", err.Error())
	}
}

func TestReadPruneApprovalRejectsUnknownFields(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, validPruneApproval()); err != nil {
		t.Fatalf("Write(prune approval) error = %v, want nil", err)
	}
	input := strings.Replace(buf.String(), "\n}", ",\n  \"extra\": true\n}", 1)

	_, err := Read[PruneApproval](strings.NewReader(input))
	if err == nil {
		t.Fatalf("Read[PruneApproval](unknown field) error = nil, want unknown field error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Read[PruneApproval](unknown field) error = %q, want unknown field", err.Error())
	}
}

func TestReadPruneReceiptRejectsTrailingJSONDocument(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, validPruneReceipt()); err != nil {
		t.Fatalf("Write(prune receipt) error = %v, want nil", err)
	}
	buf.WriteString(`{"ignored":true}`)

	_, err := Read[PruneReceipt](&buf)
	if err == nil {
		t.Fatalf("Read[PruneReceipt](trailing JSON) error = nil, want trailing JSON error")
	}
	if !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("Read[PruneReceipt](trailing JSON) error = %q, want trailing JSON", err.Error())
	}
}

func validProfileSnapshot() ProfileSnapshot {
	return ProfileSnapshot{
		Version:    CurrentVersion,
		ID:         "snap1",
		ProfileID:  "profile1",
		SessionID:  "session1",
		CapturedAt: "2026-05-16T00:00:00Z",
		Profile:    []byte(`{"version":1,"profile_id":"profile1"}`),
	}
}

func validPairingReceipt() PairingReceipt {
	return PairingReceipt{
		Version:          CurrentVersion,
		ID:               "pair1",
		ProfileID:        "profile1",
		TargetID:         "target1",
		SourceDeviceID:   "sha256:abcdef0123456789",
		TargetDeviceID:   "sha256:0123456789abcdef",
		DevicePublicKey:  "sha256:0123456789abcdef",
		Method:           "sas",
		VerifiedAt:       "2026-05-16T00:00:00Z",
		VerificationHash: "sha256:abcdef0123456789",
		ProtocolVersion:  "supermover/1",
	}
}

func validSessionReceipt() SessionReceipt {
	return SessionReceipt{
		Version:   CurrentVersion,
		ID:        "session1",
		ProfileID: "profile1",
		TargetID:  "target1",
		StartedAt: "2026-05-16T00:00:00Z",
		Status:    "completed",
	}
}

func validManifest() Manifest {
	return Manifest{
		Version:   CurrentVersion,
		ID:        "manifest1",
		SessionID: "session1",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries: []ManifestEntry{
			{Path: "notes/a.md", Kind: "file", Size: 12, Digest: "sha256:abc"},
		},
	}
}

func manifestWithPreviousEvidence(edit func(*ManifestEntry)) Manifest {
	entry := ManifestEntry{
		Path:               "file",
		Kind:               "file",
		PreviousSessionID:  "s0",
		PreviousManifestID: "m0",
		PreviousDigest:     "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PreviousModTime:    "2026-05-15T00:00:00Z",
	}
	entry.SetPreviousSizeEvidence(0)
	entry.SetPreviousModeEvidence(0o644)
	if edit != nil {
		edit(&entry)
	}
	return Manifest{
		Version:   CurrentVersion,
		ID:        "m1",
		SessionID: "s1",
		CreatedAt: "2026-05-16T00:00:00Z",
		Entries:   []ManifestEntry{entry},
	}
}

func validWarning() Warning {
	return Warning{
		Version:   CurrentVersion,
		ID:        "warning1",
		SessionID: "session1",
		Code:      "privacy",
		Message:   "plaintext restore is enabled",
		Severity:  "warning",
		Paths:     []string{"notes/a.md"},
		CreatedAt: "2026-05-16T00:00:00Z",
	}
}

func validTargetDrift() TargetDrift {
	present := true
	expected := TargetDriftExpectedState{
		SessionID:  "previous-session",
		ManifestID: "manifest-previous-session",
		Kind:       "file",
		Path:       "notes/a.md",
		Digest:     "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ModTime:    "2026-05-15T00:00:00Z",
	}
	expected.SetSizeEvidence(0)
	expected.SetModeEvidence(0)
	observed := TargetDriftObservedState{
		Present: &present,
		Kind:    "file",
		Path:    "notes/a.md",
		Digest:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ModTime: "2026-05-16T00:00:00Z",
	}
	observed.SetSizeEvidence(5)
	observed.SetModeEvidence(0o644)
	return TargetDrift{
		Version:     CurrentVersion,
		ID:          "drift1",
		SessionID:   "session1",
		ProfileID:   "profile1",
		TargetID:    "target1",
		RootID:      "root",
		Path:        "notes/a.md",
		DetectedAt:  "2026-05-16T00:00:00Z",
		Change:      "content_mismatch",
		Expected:    expected,
		Observed:    observed,
		ReviewState: "needs_review",
		Evidence:    []string{"target content differs from previous manifest evidence"},
	}
}

func validSoftDelete() SoftDelete {
	return SoftDelete{
		Version:            CurrentVersion,
		ID:                 "delete1",
		SessionID:          "session1",
		ProfileID:          "profile1",
		TargetID:           "target1",
		RootID:             "root",
		PreviousSessionID:  "session0",
		PreviousManifestID: "manifest0",
		SourcePath:         "notes/a.md",
		TargetPath:         "notes/a.md",
		Kind:               "file",
		Size:               7,
		Digest:             "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		DetectedAt:         "2026-05-16T00:00:00Z",
	}
}

func validPruneApproval() PruneApproval {
	return PruneApproval{
		Version:               CurrentVersion,
		ID:                    "approval1",
		ProfileID:             "profile1",
		TargetID:              "target1",
		RootID:                "root",
		CreatedAt:             "2026-05-16T00:01:00Z",
		ApprovedBy:            "operator@example.com",
		ApprovedAt:            "2026-05-16T00:02:00Z",
		ReviewTool:            "deleted list",
		ProfileSnapshotID:     "profile-session1",
		ProfileSnapshotDigest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		ProfileDeletePolicy: PruneDeletePolicy{
			Mode:               "prune",
			RequireReview:      true,
			RetentionDays:      30,
			AllowPhysicalPrune: true,
		},
		Items: []PruneApprovalItem{{
			SoftDeleteID:       "delete1",
			SoftDeleteRef:      "deleted/delete1.json",
			DetectedSessionID:  "session1",
			PreviousSessionID:  "session0",
			PreviousManifestID: "manifest0",
			RootID:             "root",
			SourcePath:         "notes/a.md",
			TargetPath:         "notes/a.md",
			Kind:               "file",
			Size:               7,
			Digest:             "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			DetectedAt:         "2026-05-16T00:00:00Z",
		}},
		ApprovalScopeDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Status:              "approved",
		ApprovalReason:      "operator reviewed soft-delete evidence",
	}
}

func validPruneReceipt() PruneReceipt {
	return validPruneReceiptWithResult("would_prune", true, nil)
}

func validAppliedPruneReceipt(mutate ...func(*PruneReceipt)) PruneReceipt {
	doc := validPruneReceiptWithResult("pruned", false, nil)
	for _, fn := range mutate {
		fn(&doc)
	}
	return doc
}

func validFailedPruneReceipt(mutate ...func(*PruneReceipt)) PruneReceipt {
	doc := validPruneReceiptWithResult("failed", false, func(item *PruneReceiptItem) {
		item.ErrorCode = "target_state_changed"
		item.Error = "target changed before prune"
	})
	for _, fn := range mutate {
		fn(&doc)
	}
	return doc
}

func validStartedPruneReceipt(mutate ...func(*PruneReceipt)) PruneReceipt {
	doc := validPruneReceiptWithResult("would_prune", true, nil)
	doc.DryRun = false
	doc.Status = PruneReceiptStarted
	for _, fn := range mutate {
		fn(&doc)
	}
	return doc
}

func validPruneReceiptWithResult(result string, dryRun bool, mutateItem func(*PruneReceiptItem)) PruneReceipt {
	present := true
	observed := PruneObservedTargetState{
		Present: &present,
		Kind:    "file",
		Path:    "notes/a.md",
		Digest:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ModTime: "2026-05-16T00:00:00Z",
	}
	observed.SetSizeEvidence(7)
	observed.SetModeEvidence(0o644)
	status := PruneReceiptPlanned
	endedAt := ""
	prunedAt := ""
	if !dryRun {
		endedAt = "2026-05-16T00:05:00Z"
		switch result {
		case "pruned":
			status = PruneReceiptApplied
			prunedAt = "2026-05-16T00:04:00Z"
		case "failed", "refused":
			status = PruneReceiptFailed
		default:
			status = PruneReceiptPartial
		}
	}
	item := PruneReceiptItem{
		SoftDeleteID:     "delete1",
		TargetPath:       "notes/a.md",
		IntendedAction:   "delete_file",
		PrePruneObserved: observed,
		Result:           result,
		PrunedAt:         prunedAt,
	}
	if mutateItem != nil {
		mutateItem(&item)
	}
	return PruneReceipt{
		Version:             CurrentVersion,
		ID:                  "prune1",
		PruneSessionID:      "prune1",
		ApprovalID:          "approval1",
		ProfileID:           "profile1",
		TargetID:            "target1",
		StartedAt:           "2026-05-16T00:03:00Z",
		EndedAt:             endedAt,
		Status:              status,
		DryRun:              dryRun,
		ApprovalScopeDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Items:               []PruneReceiptItem{item},
	}
}

func validHistoryIndex() HistoryIndex {
	return HistoryIndex{
		Version:   CurrentVersion,
		UpdatedAt: "2026-05-16T00:00:00Z",
		Latest:    "session1",
		Sessions: []HistoryEntry{
			{SessionID: "session1", StartedAt: "2026-05-16T00:00:00Z", ReceiptRef: "sessions/session1/receipt.json"},
		},
	}
}

func validRecoveryState() RecoveryState {
	return RecoveryState{
		Version:   CurrentVersion,
		SessionID: "session1",
		Status:    RecoveryInterrupted,
		UpdatedAt: "2026-05-16T00:00:00Z",
	}
}

func validNetworkTransfer() NetworkTransfer {
	return NetworkTransfer{
		Version:         CurrentVersion,
		SessionID:       "session1",
		ProfileID:       "profile1",
		TargetID:        "target1",
		SourceDeviceID:  "sha256:abcdef0123456789",
		TargetDeviceID:  "sha256:0123456789abcdef",
		ProtocolVersion: "supermover/1",
		PrivacyPolicy:   transport.DefaultPrivacyPolicy(transport.PrivacyLevel2),
		Status:          NetworkTransferInterrupted,
		Stage:           "chunk",
		StartedAt:       "2026-05-16T00:00:00Z",
		UpdatedAt:       "2026-05-16T00:00:01Z",
		ErrorCode:       "receiver_unavailable",
		Error:           "receiver connection closed",
		Attempts: []NetworkTransferAttempt{{
			AttemptID: "attempt-1",
			StartedAt: "2026-05-16T00:00:00Z",
			EndedAt:   "2026-05-16T00:00:01Z",
			Stage:     "chunk",
			Status:    NetworkTransferInterrupted,
			ErrorCode: "receiver_unavailable",
			Error:     "receiver connection closed",
		}},
	}
}
