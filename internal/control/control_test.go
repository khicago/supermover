package control

import (
	"bytes"
	"errors"
	"os"
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

func TestPathRejectsUnsafeID(t *testing.T) {
	for _, id := range []string{"../escape", "bad/id", `bad\id`} {
		t.Run(id, func(t *testing.T) {
			if _, err := Path("/target", ArtifactManifest, id); err == nil {
				t.Fatalf("Path(%q) error = nil, want unsafe id error", id)
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
		{name: "history index", doc: validHistoryIndex()},
		{name: "recovery state", doc: validRecoveryState()},
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
		{name: "invalid soft delete missing profile", doc: SoftDelete{Version: CurrentVersion, ID: "d1", SessionID: "s1", TargetID: "t1", RootID: "root", PreviousSessionID: "s0", PreviousManifestID: "m0", SourcePath: "a", TargetPath: "a", Kind: "file", DetectedAt: "2026-05-16T00:00:00Z"}, wantErr: "profile_id is required"},
		{name: "invalid soft delete missing previous evidence", doc: SoftDelete{Version: CurrentVersion, ID: "d1", SessionID: "s1", ProfileID: "p1", TargetID: "t1", RootID: "root", SourcePath: "a", TargetPath: "a", Kind: "file", DetectedAt: "2026-05-16T00:00:00Z"}, wantErr: "previous_session_id is required"},
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
		DetectedAt:         "2026-05-16T00:00:00Z",
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
