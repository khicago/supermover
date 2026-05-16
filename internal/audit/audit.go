package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
)

// Severity describes how much attention an audit record needs.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Disposition tracks whether a record is still active or has been handled.
type Disposition string

const (
	DispositionOpen     Disposition = "open"
	DispositionAccepted Disposition = "accepted"
	DispositionIgnored  Disposition = "ignored"
	DispositionResolved Disposition = "resolved"
)

// Record is a stable, serializable audit finding.
type Record struct {
	ID                    string            `json:"id"`
	Path                  string            `json:"path"`
	TargetPath            string            `json:"target_path,omitempty"`
	Severity              Severity          `json:"severity"`
	Kind                  string            `json:"kind"`
	Reason                string            `json:"reason"`
	Detected              map[string]string `json:"detected,omitempty"`
	SuggestedProfilePatch map[string]string `json:"suggested_profile_patch,omitempty"`
	SuggestedConfig       map[string]string `json:"suggested_config,omitempty"`
	Disposition           Disposition       `json:"disposition"`
}

// New creates a record with a deterministic ID from the stable identifying
// fields. Metadata and suggestions are intentionally excluded so later
// enrichment does not change the finding identity.
func New(path, targetPath string, severity Severity, kind, reason string) Record {
	r := Record{
		Path:        cleanSlash(path),
		TargetPath:  cleanSlash(targetPath),
		Severity:    severity,
		Kind:        kind,
		Reason:      reason,
		Disposition: DispositionOpen,
	}
	r.ID = StableID(r)
	return r
}

// StableID returns a deterministic ID for the record's identifying fields.
func StableID(r Record) string {
	parts := []string{
		cleanSlash(r.Path),
		cleanSlash(r.TargetPath),
		string(r.Severity),
		r.Kind,
		r.Reason,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "aud_" + hex.EncodeToString(sum[:8])
}

// WithDetected returns a copy of r with sorted, copied detected metadata.
func WithDetected(r Record, detected map[string]string) Record {
	r.Detected = copyMap(detected)
	return r
}

// WithSuggestedProfilePatch returns a copy of r with a copied profile patch.
func WithSuggestedProfilePatch(r Record, patch map[string]string) Record {
	r.SuggestedProfilePatch = copyMap(patch)
	return r
}

// WithSuggestedConfig returns a copy of r with copied config suggestions.
func WithSuggestedConfig(r Record, config map[string]string) Record {
	r.SuggestedConfig = copyMap(config)
	return r
}

func cleanSlash(path string) string {
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func copyMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out[k] = in[k]
	}
	return out
}
