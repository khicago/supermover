package control

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
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
		{"history", ArtifactHistoryIndex, "", filepath.Join(root, DirName, "history", "index.json")},
		{"recovery", ArtifactRecoveryState, "", filepath.Join(root, DirName, "recovery", "state.json")},
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

func TestPathRejectsMissingID(t *testing.T) {
	_, err := Path("/target", ArtifactManifest, "")
	if err == nil {
		t.Fatalf("Path() error = nil, want missing id error")
	}
	if !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("Path() error = %q, want missing id error", err.Error())
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
		{name: "history index", doc: validHistoryIndex()},
		{name: "recovery state", doc: validRecoveryState()},
		{name: "invalid recovery", doc: RecoveryState{Version: CurrentVersion, Status: "unknown", UpdatedAt: "2026-05-16T00:00:00Z"}, wantErr: "status must be one of"},
		{name: "invalid manifest entry", doc: Manifest{Version: CurrentVersion, ID: "m1", SessionID: "s1", CreatedAt: "2026-05-16T00:00:00Z", Entries: []ManifestEntry{{Kind: "file"}}}, wantErr: "entries[0].path is required"},
		{name: "invalid symlink manifest entry", doc: Manifest{Version: CurrentVersion, ID: "m1", SessionID: "s1", CreatedAt: "2026-05-16T00:00:00Z", Entries: []ManifestEntry{{Path: "link", Kind: "symlink"}}}, wantErr: "entries[0].symlink_target is required"},
		{name: "invalid snapshot payload", doc: ProfileSnapshot{Version: CurrentVersion, ID: "snap1", ProfileID: "p1", CapturedAt: "2026-05-16T00:00:00Z", Profile: []byte(`{`)}, wantErr: "profile must contain valid JSON"},
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
		Version:         CurrentVersion,
		ID:              "pair1",
		ProfileID:       "profile1",
		TargetID:        "target1",
		DevicePublicKey: "ed25519:example",
		VerifiedAt:      "2026-05-16T00:00:00Z",
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

func validWarning() Warning {
	return Warning{
		Version:   CurrentVersion,
		ID:        "warning1",
		Code:      "privacy",
		Message:   "plaintext restore is enabled",
		CreatedAt: "2026-05-16T00:00:00Z",
	}
}

func validTargetDrift() TargetDrift {
	return TargetDrift{
		Version:    CurrentVersion,
		ID:         "drift1",
		Path:       "notes/a.md",
		DetectedAt: "2026-05-16T00:00:00Z",
		Change:     "modified",
	}
}

func validSoftDelete() SoftDelete {
	return SoftDelete{
		Version:    CurrentVersion,
		ID:         "delete1",
		SourcePath: "notes/a.md",
		TargetPath: "notes/a.md",
		DetectedAt: "2026-05-16T00:00:00Z",
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
