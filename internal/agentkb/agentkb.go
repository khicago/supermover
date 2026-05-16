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

// KnowledgeCategory describes a configured group of agent knowledge paths.
type KnowledgeCategory struct {
	Name     Category
	Paths    []string
	Manifest bool
}

// DefaultCategories returns the built-in agent knowledge categories used by
// default profiles and default detection.
func DefaultCategories() []KnowledgeCategory {
	categories := []KnowledgeCategory{
		{Name: CategoryRepoRules, Paths: []string{"AGENTS.md"}, Manifest: true},
		{Name: CategoryToolProjectRules, Paths: []string{"CLAUDE.md", "GEMINI.md", ".github/copilot-instructions.md", ".github/instructions/**", ".cursor/rules/**", ".windsurf/rules/**", ".continue/**"}, Manifest: true},
		{Name: CategoryHomeMemories, Paths: []string{".claude.json", ".claude/**", ".gemini/settings.json", ".gemini/**"}, Manifest: true},
		{Name: CategoryGeneratedState, Paths: []string{".codex/**"}, Manifest: true},
	}
	out := make([]KnowledgeCategory, 0, len(categories))
	for _, category := range categories {
		out = append(out, KnowledgeCategory{
			Name:     category.Name,
			Paths:    append([]string(nil), category.Paths...),
			Manifest: category.Manifest,
		})
	}
	return out
}

// Detect returns known agent knowledge files and directories from scan entries.
func Detect(entries []scan.Entry, categories ...[]KnowledgeCategory) []Influence {
	activeCategories := DefaultCategories()
	if len(categories) > 0 {
		activeCategories = cloneCategories(categories[0])
	}
	var influences []Influence
	seen := map[string]bool{}
	for _, entry := range entries {
		p := clean(entry.Path)
		if p == "." {
			continue
		}
		pattern, category, ok := classify(p, activeCategories)
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

func classify(path string, categories []KnowledgeCategory) (string, Category, bool) {
	for _, category := range categories {
		for _, pattern := range category.Paths {
			if matches(pattern, path) {
				return pattern, category.Name, true
			}
		}
	}
	return "", "", false
}

func matches(pattern, path string) bool {
	pattern = clean(pattern)
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return strings.HasPrefix(path, prefix+"/")
	}
	return path == pattern
}

func cloneCategories(categories []KnowledgeCategory) []KnowledgeCategory {
	out := make([]KnowledgeCategory, 0, len(categories))
	for _, category := range categories {
		out = append(out, KnowledgeCategory{
			Name:     category.Name,
			Paths:    append([]string(nil), category.Paths...),
			Manifest: category.Manifest,
		})
	}
	return out
}

func clean(path string) string {
	if path == "" {
		return "."
	}
	return filepath.ToSlash(filepath.Clean(path))
}
