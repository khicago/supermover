package profile

import (
	"bytes"
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
			name:    "invalid consistency mode",
			mutate:  func(p *Profile) { p.Consistency = "eventual" },
			wantErr: "consistency must be one of",
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
