package profile

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/khicago/supermover/internal/agentkb"
	"github.com/khicago/supermover/internal/scan"
)

func TestNewDefaultBuildsValidProfile(t *testing.T) {
	source := filepath.Join("tmp", "source")
	target := filepath.Join("tmp", "target")

	got := NewDefault("profile-local", "", source, target)

	if err := got.Validate(); err != nil {
		t.Fatalf("NewDefault(%q, %q, %q, %q).Validate() error = %v, want nil", "profile-local", "", source, target, err)
	}
	if got.Name != "profile-local" {
		t.Errorf("NewDefault() name = %q, want %q", got.Name, "profile-local")
	}
	if got.Roots[0].Path != filepath.Clean(source) {
		t.Errorf("NewDefault() root path = %q, want %q", got.Roots[0].Path, filepath.Clean(source))
	}
	if got.Target.LocalPath != filepath.Clean(target) {
		t.Errorf("NewDefault() target local path = %q, want %q", got.Target.LocalPath, filepath.Clean(target))
	}
	if got.Target.TargetID != "local:profile-local" {
		t.Errorf("NewDefault() target id = %q, want local:profile-local", got.Target.TargetID)
	}
	if got.Target.TargetID == got.Target.LocalPath {
		t.Errorf("NewDefault() target id = target local path = %q, want separate identity and reachability path", got.Target.TargetID)
	}
	if got.DeletePolicy.Mode != DeleteModeRecord || !got.DeletePolicy.RequireReview {
		t.Errorf("NewDefault() delete policy = (%q, %t), want record with review", got.DeletePolicy.Mode, got.DeletePolicy.RequireReview)
	}
	if got.PrivacyPolicy.TrafficLevel != 2 || !got.PrivacyPolicy.DiscoveryLowInfo {
		t.Errorf("NewDefault() privacy traffic = (%d, %t), want level 2 low-info discovery", got.PrivacyPolicy.TrafficLevel, got.PrivacyPolicy.DiscoveryLowInfo)
	}
}

func TestDefaultPrivacyPolicyIsLevel2Compatible(t *testing.T) {
	got := DefaultPrivacyPolicy()

	if got.Mode != PrivacyModePlaintext {
		t.Fatalf("DefaultPrivacyPolicy().Mode = %q, want %q", got.Mode, PrivacyModePlaintext)
	}
	if got.TrafficLevel != 2 {
		t.Fatalf("DefaultPrivacyPolicy().TrafficLevel = %d, want 2", got.TrafficLevel)
	}
	if !got.AllowPlaintextRestore || !got.AllowHiddenFiles || !got.AllowSensitiveFilenames {
		t.Fatalf("DefaultPrivacyPolicy() restore/data flags = %#v, want plaintext restore and hidden/sensitive filenames allowed", got)
	}
	if got.PaddingBucketBytes == 0 || got.BatchMaxBytes == 0 || got.BatchMaxCount == 0 || !got.DiscoveryLowInfo {
		t.Fatalf("DefaultPrivacyPolicy() = %#v, want traffic level 2 shaping fields", got)
	}

	p := validProfile()
	p.PrivacyPolicy = got
	if err := p.Validate(); err != nil {
		t.Fatalf("valid profile with DefaultPrivacyPolicy().Validate() error = %v, want nil", err)
	}
}

func TestDefaultAgentKnowledgeCategoriesAreDetected(t *testing.T) {
	knowledge := DefaultAgentKnowledge()
	var entries []scan.Entry
	want := map[string]string{}
	for _, category := range knowledge.Categories {
		for _, pattern := range category.Paths {
			path := samplePath(pattern)
			entries = append(entries, scan.Entry{Path: path, Kind: scan.KindRegular})
			want[path] = category.Name
		}
	}

	got := agentkb.Detect(entries)
	gotByPath := map[string]agentkb.Influence{}
	for _, influence := range got {
		gotByPath[influence.Path] = influence
	}
	for path, category := range want {
		influence, ok := gotByPath[path]
		if !ok {
			t.Errorf("agentkb.Detect(default profile path %q) missing influence, want category %q", path, category)
			continue
		}
		if string(influence.Category) != category {
			t.Errorf("agentkb.Detect(default profile path %q) category = %q, want %q", path, influence.Category, category)
		}
	}
	if len(got) != len(want) {
		t.Errorf("agentkb.Detect(default profile paths) returned %d influences, want %d: %#v", len(got), len(want), got)
	}
}

func samplePath(pattern string) string {
	if strings.HasSuffix(pattern, "/**") {
		return strings.TrimSuffix(pattern, "/**") + "/example.md"
	}
	return pattern
}
