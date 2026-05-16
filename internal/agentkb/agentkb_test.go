package agentkb

import (
	"testing"

	"github.com/khicago/supermover/internal/scan"
)

func TestDetectAgentKnowledgeInfluences(t *testing.T) {
	entries := []scan.Entry{
		{Path: "AGENTS.md", Kind: scan.KindRegular},
		{Path: "CLAUDE.md", Kind: scan.KindRegular},
		{Path: "GEMINI.md", Kind: scan.KindRegular},
		{Path: ".github/copilot-instructions.md", Kind: scan.KindRegular, Hidden: true},
		{Path: ".github/instructions/review.instructions.md", Kind: scan.KindRegular, Hidden: true},
		{Path: ".cursor/rules/project.mdc", Kind: scan.KindRegular, Hidden: true},
		{Path: ".windsurf/rules/style.md", Kind: scan.KindRegular, Hidden: true},
		{Path: ".continue/config.json", Kind: scan.KindRegular, Hidden: true},
		{Path: ".codex/state.json", Kind: scan.KindRegular, Hidden: true},
		{Path: ".claude.json", Kind: scan.KindRegular, Hidden: true},
		{Path: "src/main.go", Kind: scan.KindRegular},
	}

	got := Detect(entries)
	if len(got) != 10 {
		t.Fatalf("expected 10 influences, got %d: %#v", len(got), got)
	}

	byPath := map[string]Influence{}
	for _, influence := range got {
		byPath[influence.Path] = influence
	}
	assertCategory(t, byPath, "AGENTS.md", CategoryRepoRules)
	assertCategory(t, byPath, "CLAUDE.md", CategoryToolProjectRules)
	assertCategory(t, byPath, ".github/instructions/review.instructions.md", CategoryToolProjectRules)
	assertCategory(t, byPath, ".codex/state.json", CategoryGeneratedState)
	assertCategory(t, byPath, ".claude.json", CategoryHomeMemories)
}

func TestDetectKeepsSymlinkTarget(t *testing.T) {
	got := Detect([]scan.Entry{{
		Path:          "AGENTS.md",
		Kind:          scan.KindSymlink,
		SymlinkTarget: "../shared/AGENTS.md",
	}})
	if len(got) != 1 {
		t.Fatalf("expected one influence, got %#v", got)
	}
	if got[0].Target != "../shared/AGENTS.md" {
		t.Fatalf("target = %q", got[0].Target)
	}
}

func TestDetectWithExplicitCategoriesDoesNotUseDefaultRules(t *testing.T) {
	entries := []scan.Entry{
		{Path: "AGENTS.md", Kind: scan.KindRegular},
		{Path: "TEAM.md", Kind: scan.KindRegular},
		{Path: ".codex/state.json", Kind: scan.KindRegular, Hidden: true},
	}
	got := Detect(entries, []KnowledgeCategory{
		{Name: CategoryRepoRules, Paths: []string{"TEAM.md"}, Manifest: true},
	})
	if len(got) != 1 {
		t.Fatalf("Detect(%#v, explicit categories) returned %d influences, want 1: %#v", entries, len(got), got)
	}
	if got[0].Path != "TEAM.md" || got[0].Pattern != "TEAM.md" || got[0].Category != CategoryRepoRules {
		t.Fatalf("Detect(%#v, explicit categories)[0] = %#v, want TEAM.md repo_rules influence", entries, got[0])
	}
}

func assertCategory(t *testing.T, byPath map[string]Influence, path string, category Category) {
	t.Helper()
	influence, ok := byPath[path]
	if !ok {
		t.Fatalf("missing influence for %s", path)
	}
	if influence.Category != category {
		t.Fatalf("%s category = %q, want %q", path, influence.Category, category)
	}
}
