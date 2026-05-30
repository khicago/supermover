package profile

import (
	"path/filepath"
	"strings"

	"github.com/khicago/supermover/internal/agentkb"
)

func NewSourceOnly(profileID, name, sourceRoot string) Profile {
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
		PrivacyPolicy: DefaultPrivacyPolicy(),
		Target: TargetIdentity{
			TargetID: "local:" + profileID,
		},
		AgentKnowledge: DefaultAgentKnowledge(),
	}
}

func NewDefault(profileID, name, sourceRoot, targetRoot string) Profile {
	p := NewSourceOnly(profileID, name, sourceRoot)
	cleanTarget := filepath.Clean(targetRoot)
	p.Target.Name = filepath.Base(cleanTarget)
	p.Target.LocalPath = cleanTarget
	return p
}

func DefaultPrivacyPolicy() PrivacyPolicy {
	return PrivacyPolicy{
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
	}
}

func DefaultAgentKnowledge() AgentKnowledge {
	defaults := agentkb.DefaultCategories()
	categories := make([]KnowledgeCategory, 0, len(defaults))
	for _, category := range defaults {
		categories = append(categories, KnowledgeCategory{
			Name:     string(category.Name),
			Paths:    append([]string(nil), category.Paths...),
			Manifest: category.Manifest,
		})
	}
	return AgentKnowledge{
		Categories: categories,
	}
}
