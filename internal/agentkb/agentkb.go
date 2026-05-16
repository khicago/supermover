package agentkb

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/khicago/supermover/internal/scan"
)

// Category describes the likely scope or provenance of an agent knowledge file.
type Category string

const (
	CategoryRepoRules        Category = "repo_rules"
	CategoryToolProjectRules Category = "tool_project_rules"
	CategoryHomeMemories     Category = "home_memories"
	CategoryGeneratedState   Category = "generated_state"
)

// Influence records a path that can affect agent behavior or memory.
type Influence struct {
	Path     string    `json:"path"`
	Pattern  string    `json:"pattern"`
	Category Category  `json:"category"`
	Kind     scan.Kind `json:"kind"`
	Hidden   bool      `json:"hidden"`
	Target   string    `json:"target,omitempty"`
}

// Detect returns known agent knowledge files and directories from scan entries.
func Detect(entries []scan.Entry) []Influence {
	var influences []Influence
	seen := map[string]bool{}
	for _, entry := range entries {
		p := clean(entry.Path)
		if p == "." {
			continue
		}
		pattern, category, ok := classify(p)
		if !ok {
			continue
		}
		key := p + "\x00" + pattern
		if seen[key] {
			continue
		}
		seen[key] = true
		influences = append(influences, Influence{
			Path:     p,
			Pattern:  pattern,
			Category: category,
			Kind:     entry.Kind,
			Hidden:   entry.Hidden,
			Target:   entry.SymlinkTarget,
		})
	}
	sort.Slice(influences, func(i, j int) bool {
		if influences[i].Path == influences[j].Path {
			return influences[i].Pattern < influences[j].Pattern
		}
		return influences[i].Path < influences[j].Path
	})
	return influences
}

func classify(path string) (string, Category, bool) {
	switch path {
	case "AGENTS.md":
		return "AGENTS.md", CategoryRepoRules, true
	case "CLAUDE.md":
		return "CLAUDE.md", CategoryToolProjectRules, true
	case "GEMINI.md":
		return "GEMINI.md", CategoryToolProjectRules, true
	case ".github/copilot-instructions.md":
		return ".github/copilot-instructions.md", CategoryToolProjectRules, true
	}

	prefixes := []struct {
		prefix   string
		pattern  string
		category Category
	}{
		{".github/instructions/", ".github/instructions/**", CategoryToolProjectRules},
		{".cursor/rules/", ".cursor/rules/**", CategoryToolProjectRules},
		{".windsurf/rules/", ".windsurf/rules/**", CategoryToolProjectRules},
		{".continue/", ".continue/**", CategoryToolProjectRules},
		{".codex/", ".codex/**", CategoryGeneratedState},
	}
	for _, rule := range prefixes {
		if strings.HasPrefix(path, rule.prefix) {
			return rule.pattern, rule.category, true
		}
	}

	if isHomeMemory(path) {
		return path, CategoryHomeMemories, true
	}
	return "", "", false
}

func isHomeMemory(path string) bool {
	return path == ".claude.json" ||
		path == ".gemini/settings.json" ||
		strings.HasPrefix(path, ".claude/") ||
		strings.HasPrefix(path, ".gemini/")
}

func clean(path string) string {
	if path == "" {
		return "."
	}
	return filepath.ToSlash(filepath.Clean(path))
}
