package profile

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRejectsInvalidProfiles(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Profile)
		wantErr string
	}{
		{
			name:    "missing profile id",
			mutate:  func(p *Profile) { p.ProfileID = "" },
			wantErr: "profile_id is required",
		},
		{
			name:    "empty roots",
			mutate:  func(p *Profile) { p.Roots = nil },
			wantErr: "roots must contain at least one root",
		},
		{
			name:    "missing root id",
			mutate:  func(p *Profile) { p.Roots[0].ID = " \t" },
			wantErr: "roots[0].id is required",
		},
		{
			name:    "missing root path",
			mutate:  func(p *Profile) { p.Roots[0].Path = "" },
			wantErr: "roots[0].path is required",
		},
		{
			name:    "empty include pattern",
			mutate:  func(p *Profile) { p.Include = []Rule{{Pattern: " "}} },
			wantErr: "include[0].pattern is required",
		},
		{
			name:    "empty exclude pattern",
			mutate:  func(p *Profile) { p.Exclude = []Rule{{Pattern: ""}} },
			wantErr: "exclude[0].pattern is required",
		},
		{
			name:    "invalid consistency mode",
			mutate:  func(p *Profile) { p.Consistency = "eventual" },
			wantErr: "consistency must be one of",
		},
		{
			name:    "invalid delete mode",
			mutate:  func(p *Profile) { p.DeletePolicy.Mode = "trash" },
			wantErr: "delete_policy.mode must be one of",
		},
		{
			name:    "negative retention",
			mutate:  func(p *Profile) { p.DeletePolicy.RetentionDays = -1 },
			wantErr: "delete_policy.retention_days cannot be negative",
		},
		{
			name:    "invalid metadata mode",
			mutate:  func(p *Profile) { p.MetadataPolicy.Mode = "full" },
			wantErr: "metadata_policy.mode must be one of",
		},
		{
			name:    "invalid privacy mode",
			mutate:  func(p *Profile) { p.PrivacyPolicy.Mode = "sealed" },
			wantErr: "privacy_policy.mode must be one of",
		},
		{
			name:    "unsafe plaintext privacy",
			mutate:  func(p *Profile) { p.PrivacyPolicy.AllowPlaintextRestore = false },
			wantErr: "allow_plaintext_restore must be true",
		},
		{
			name:    "invalid traffic level",
			mutate:  func(p *Profile) { p.PrivacyPolicy.TrafficLevel = 9 },
			wantErr: "traffic_level must be 1, 2, or 3",
		},
		{
			name:    "level 2 without padding",
			mutate:  func(p *Profile) { p.PrivacyPolicy.PaddingBucketBytes = 0 },
			wantErr: "padding_bucket_bytes is required",
		},
		{
			name:    "level 2 without batching",
			mutate:  func(p *Profile) { p.PrivacyPolicy.BatchMaxBytes = 0 },
			wantErr: "batching is required",
		},
		{
			name:    "level 2 high info discovery",
			mutate:  func(p *Profile) { p.PrivacyPolicy.DiscoveryLowInfo = false },
			wantErr: "discovery_low_info must be true",
		},
		{
			name:    "negative padding",
			mutate:  func(p *Profile) { p.PrivacyPolicy.PaddingBucketBytes = -1 },
			wantErr: "privacy_policy.padding_bucket_bytes cannot be negative",
		},
		{
			name:    "negative batch bytes",
			mutate:  func(p *Profile) { p.PrivacyPolicy.BatchMaxBytes = -1 },
			wantErr: "privacy_policy.batch_max_bytes cannot be negative",
		},
		{
			name:    "negative batch count",
			mutate:  func(p *Profile) { p.PrivacyPolicy.BatchMaxCount = -1 },
			wantErr: "privacy_policy.batch_max_count cannot be negative",
		},
		{
			name:    "negative jitter",
			mutate:  func(p *Profile) { p.PrivacyPolicy.JitterBudgetMillis = -1 },
			wantErr: "privacy_policy.jitter_budget_millis cannot be negative",
		},
		{
			name:    "prune without review",
			mutate:  func(p *Profile) { p.DeletePolicy.Mode = DeleteModePrune; p.DeletePolicy.RequireReview = false },
			wantErr: "require_review must be true",
		},
		{
			name:    "missing target id",
			mutate:  func(p *Profile) { p.Target.TargetID = "" },
			wantErr: "target.target_id is required",
		},
		{
			name: "target id equals local path",
			mutate: func(p *Profile) {
				p.Target.LocalPath = "/tmp/target"
				p.Target.TargetID = "/tmp/target"
			},
			wantErr: "target.target_id must not equal target.local_path",
		},
		{
			name: "target id equals local path after clean",
			mutate: func(p *Profile) {
				p.Target.LocalPath = "/tmp/target"
				p.Target.TargetID = "/tmp/target/"
			},
			wantErr: "target.target_id must not equal target.local_path",
		},
		{
			name:    "missing knowledge category name",
			mutate:  func(p *Profile) { p.AgentKnowledge.Categories = []KnowledgeCategory{{Name: " "}} },
			wantErr: "agent_knowledge.categories[0].name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			tt.mutate(&p)

			err := p.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateCollectsMultipleProfileErrors(t *testing.T) {
	p := validProfile()
	p.ProfileID = ""
	p.DeletePolicy.Mode = DeleteModePrune
	p.DeletePolicy.RequireReview = false
	p.PrivacyPolicy.PaddingBucketBytes = 0
	p.Target.TargetID = ""

	err := p.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want multiple validation errors")
	}
	for _, want := range []string{
		"profile_id is required",
		"delete_policy.require_review must be true",
		"privacy_policy.padding_bucket_bytes is required",
		"target.target_id is required",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Validate() error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestValidateAcceptsPrivacyTrafficLevelsWithRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Profile)
	}{
		{
			name: "level 1 plaintext without traffic shaping fields",
			mutate: func(p *Profile) {
				p.PrivacyPolicy.TrafficLevel = 1
				p.PrivacyPolicy.PaddingBucketBytes = 0
				p.PrivacyPolicy.BatchMaxBytes = 0
				p.PrivacyPolicy.BatchMaxCount = 0
				p.PrivacyPolicy.DiscoveryLowInfo = false
			},
		},
		{
			name: "level 2 requires padding batching and low info discovery",
			mutate: func(p *Profile) {
				p.PrivacyPolicy.TrafficLevel = 2
				p.PrivacyPolicy.PaddingBucketBytes = 4096
				p.PrivacyPolicy.BatchMaxBytes = 8192
				p.PrivacyPolicy.BatchMaxCount = 8
				p.PrivacyPolicy.DiscoveryLowInfo = true
			},
		},
		{
			name: "level 3 with explicit shaping fields",
			mutate: func(p *Profile) {
				p.PrivacyPolicy.TrafficLevel = 3
				p.PrivacyPolicy.PaddingBucketBytes = 4096
				p.PrivacyPolicy.BatchMaxBytes = 8192
				p.PrivacyPolicy.BatchMaxCount = 8
				p.PrivacyPolicy.DiscoveryLowInfo = true
			},
		},
		{
			name: "redacted mode does not require plaintext restore",
			mutate: func(p *Profile) {
				p.PrivacyPolicy.Mode = PrivacyModeRedacted
				p.PrivacyPolicy.AllowPlaintextRestore = false
			},
		},
		{
			name: "prune with review gate",
			mutate: func(p *Profile) {
				p.DeletePolicy.Mode = DeleteModePrune
				p.DeletePolicy.RequireReview = true
				p.DeletePolicy.AllowPhysicalPrune = true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProfile()
			tt.mutate(&p)

			if err := p.Validate(); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := validProfile()

	if err := Write(&buf, want); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if got := buf.String(); !strings.HasSuffix(got, "\n") {
		t.Fatalf("Write() output = %q, want trailing newline", got)
	}

	got, err := Read(&buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got.ProfileID != want.ProfileID {
		t.Fatalf("Read() profile_id = %q, want %q", got.ProfileID, want.ProfileID)
	}
	if got.Roots[0].Path != want.Roots[0].Path {
		t.Fatalf("Read() roots[0].path = %q, want %q", got.Roots[0].Path, want.Roots[0].Path)
	}
}

func TestWriteRejectsInvalidProfile(t *testing.T) {
	var buf bytes.Buffer
	p := validProfile()
	p.Target.TargetID = ""

	err := Write(&buf, p)
	if err == nil {
		t.Fatal("Write(invalid profile) error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "target.target_id is required") {
		t.Fatalf("Write(invalid profile) error = %q, want target id validation", err.Error())
	}
	if buf.Len() != 0 {
		t.Fatalf("Write(invalid profile) wrote %d bytes, want 0", buf.Len())
	}
}

func TestWriteReadFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles", "profile.json")
	want := validProfile()

	if err := WriteFile(path, want); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", path, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "profiles")); err != nil {
		t.Fatalf("os.Stat(profile dir) error = %v, want nil", err)
	}
	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v, want nil", path, err)
	}
	if got.ProfileID != want.ProfileID || got.Target.TargetID != want.Target.TargetID {
		t.Fatalf("ReadFile(%q) = %#v, want profile_id %q and target_id %q", path, got, want.ProfileID, want.Target.TargetID)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil", filepath.Dir(path), err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".profile-") {
			t.Fatalf("WriteFile(%q) left temporary file %q", path, entry.Name())
		}
	}
}

func TestReadFilePropagatesOpenError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")

	_, err := ReadFile(path)
	if err == nil {
		t.Fatalf("ReadFile(%q) error = nil, want open error", path)
	}
	if !os.IsNotExist(err) {
		t.Fatalf("ReadFile(%q) error = %v, want not exist", path, err)
	}
}

func TestReadRejectsUnknownFields(t *testing.T) {
	input := `{"version":1,"profile_id":"p","name":"n","roots":[{"id":"home","path":"/home/me"}],"consistency":"strict","delete_policy":{"mode":"record","require_review":true},"metadata_policy":{"mode":"basic"},"privacy_policy":{"mode":"plaintext","traffic_level":1,"allow_plaintext_restore":true},"target":{"target_id":"target"},"agent_knowledge":{},"extra":true}`

	_, err := Read(strings.NewReader(input))
	if err == nil {
		t.Fatalf("Read() error = nil, want unknown field error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Read() error = %q, want unknown field error", err.Error())
	}
}

func TestReadRejectsTrailingJSONDocument(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, validProfile()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	buf.WriteString(`{"ignored":true}`)

	_, err := Read(&buf)
	if err == nil {
		t.Fatalf("Read() error = nil, want trailing JSON error")
	}
	if !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("Read() error = %q, want trailing JSON error", err.Error())
	}
}

func TestReadRejectsMalformedTrailingJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, validProfile()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	buf.WriteString(`{"ignored":`)

	_, err := Read(&buf)
	if err == nil {
		t.Fatal("Read() error = nil, want malformed trailing JSON error")
	}
	if strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("Read() error = %q, want decoder error for malformed trailing JSON", err.Error())
	}
}

func TestReadForTargetRepairAllowsLegacyPathIdentity(t *testing.T) {
	input := `{"version":1,"profile_id":"p","name":"n","roots":[{"id":"home","path":"/home/me"}],"consistency":"strict","delete_policy":{"mode":"record","require_review":true},"metadata_policy":{"mode":"basic"},"privacy_policy":{"mode":"plaintext","traffic_level":1,"allow_plaintext_restore":true},"target":{"target_id":"/tmp/target","local_path":"/tmp/target"},"agent_knowledge":{}}`

	if _, err := Read(strings.NewReader(input)); err == nil {
		t.Fatalf("Read(legacy path identity) error = nil, want strict validation error")
	}
	got, err := ReadForTargetRepair(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadForTargetRepair(legacy path identity) error = %v, want nil", err)
	}
	if got.Target.TargetID != "/tmp/target" || got.Target.LocalPath != "/tmp/target" {
		t.Fatalf("ReadForTargetRepair target = %#v, want legacy target loaded", got.Target)
	}
}

func TestReadFileForTargetRepairAllowsLegacyPathIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	input := `{"version":1,"profile_id":"p","name":"n","roots":[{"id":"home","path":"/home/me"}],"consistency":"strict","delete_policy":{"mode":"record","require_review":true},"metadata_policy":{"mode":"basic"},"privacy_policy":{"mode":"plaintext","traffic_level":1,"allow_plaintext_restore":true},"target":{"target_id":"/tmp/target","local_path":"/tmp/target"},"agent_knowledge":{}}`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	got, err := ReadFileForTargetRepair(path)
	if err != nil {
		t.Fatalf("ReadFileForTargetRepair(%q) error = %v, want nil", path, err)
	}
	if got.Target.TargetID != got.Target.LocalPath {
		t.Fatalf("ReadFileForTargetRepair(%q) target = %#v, want equal legacy identity", path, got.Target)
	}
}

func TestReadForTargetRepairRejectsTrailingJSONDocument(t *testing.T) {
	input := `{"version":1,"profile_id":"p","name":"n","roots":[{"id":"home","path":"/home/me"}],"consistency":"strict","delete_policy":{"mode":"record","require_review":true},"metadata_policy":{"mode":"basic"},"privacy_policy":{"mode":"plaintext","traffic_level":1,"allow_plaintext_restore":true},"target":{"target_id":"/tmp/target","local_path":"/tmp/target"},"agent_knowledge":{}}
{"ignored":true}`

	_, err := ReadForTargetRepair(strings.NewReader(input))
	if err == nil {
		t.Fatalf("ReadForTargetRepair() error = nil, want trailing JSON error")
	}
	if !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("ReadForTargetRepair() error = %q, want trailing JSON error", err.Error())
	}
}

func validProfile() Profile {
	return Profile{
		Version:   CurrentVersion,
		ProfileID: "profile-local",
		Name:      "Local profile",
		Roots: []Root{
			{ID: "home", Path: "/Users/example"},
		},
		Include:     []Rule{{Pattern: "**"}},
		Exclude:     []Rule{{Pattern: ".git/**"}},
		Consistency: ConsistencyStrict,
		DeletePolicy: DeletePolicy{
			Mode:          DeleteModeRecord,
			RequireReview: true,
			RetentionDays: 30,
		},
		MetadataPolicy: MetadataPolicy{
			Mode:                MetadataModeBasic,
			PreservePermissions: true,
			PreserveModTime:     true,
		},
		PrivacyPolicy: PrivacyPolicy{
			Mode:                  PrivacyModePlaintext,
			TrafficLevel:          2,
			AllowPlaintextRestore: true,
			PaddingBucketBytes:    64 * 1024,
			BatchMaxBytes:         1024 * 1024,
			BatchMaxCount:         64,
			JitterBudgetMillis:    250,
			DiscoveryLowInfo:      true,
		},
		Target: TargetIdentity{
			TargetID:         "target-local",
			Name:             "Target",
			DevicePublicKey:  "ed25519:example",
			PairingReceiptID: "pairing-1",
		},
		AgentKnowledge: AgentKnowledge{
			Categories: []KnowledgeCategory{
				{Name: "codex", Paths: []string{".codex/**"}, Manifest: true},
			},
		},
	}
}
