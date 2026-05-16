package profile

import (
	"path/filepath"
	"strings"
)

func NewDefault(profileID, name, sourceRoot, targetRoot string) Profile {
	if strings.TrimSpace(name) == "" {
		name = profileID
	}
	return Profile{
		Version:   CurrentVersion,
		ProfileID: profileID,
		Name:      name,
		Roots: []Root{
			{ID: "root", Path: filepath.Clean(sourceRoot)},
		},
		Include:     []Rule{{Pattern: "**"}},
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
			Mode:                    PrivacyModePlaintext,
			TrafficLevel:            2,
			AllowPlaintextRestore:   true,
			AllowHiddenFiles:        true,
			AllowSensitiveFilenames: true,
			PaddingBucketBytes:      64 * 1024,
			BatchMaxBytes:           1024 * 1024,
			BatchMaxCount:           64,
			JitterBudgetMillis:      250,
			DiscoveryLowInfo:        true,
		},
		Target: TargetIdentity{
			TargetID:  "local:" + profileID,
			Name:      filepath.Base(filepath.Clean(targetRoot)),
			LocalPath: filepath.Clean(targetRoot),
		},
		AgentKnowledge: AgentKnowledge{
			Categories: []KnowledgeCategory{
				{Name: "repo_rules", Paths: []string{"AGENTS.md"}, Manifest: true},
				{Name: "tool_project_rules", Paths: []string{"CLAUDE.md", "GEMINI.md", ".github/instructions/**", ".cursor/rules/**", ".windsurf/rules/**", ".continue/**"}, Manifest: true},
				{Name: "generated_state", Paths: []string{".codex/**"}, Manifest: true},
			},
		},
	}
}
