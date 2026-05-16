package profile

import (
	"path/filepath"
	"testing"
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
