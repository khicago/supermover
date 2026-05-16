package deleted

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/scan"
)

type Options struct {
	PreviousManifest control.Manifest
	CurrentScan      scan.Result
	SessionID        string
	ProfileID        string
	TargetID         string
	RootID           string
	DetectedAt       time.Time
}

type Result struct {
	Records []control.SoftDelete `json:"records"`
}

func Generate(opts Options) (Result, error) {
	if strings.TrimSpace(opts.PreviousManifest.SessionID) == "" {
		return Result{}, fmt.Errorf("previous manifest session_id is required")
	}
	detectedAt := opts.DetectedAt
	if detectedAt.IsZero() {
		detectedAt = time.Now().UTC()
	}
	sessionID := opts.SessionID
	if sessionID == "" {
		sessionID = opts.PreviousManifest.SessionID
	}

	current := currentEntries(opts.CurrentScan)
	var records []control.SoftDelete
	for _, entry := range opts.PreviousManifest.Entries {
		if entry.Path == "." || entry.Kind == "dir" {
			continue
		}
		sourcePath := cleanRel(entry.Path)
		if currentEntry, ok := current[sourcePath]; ok && sameLogicalSourceKind(entry.Kind, currentEntry.Kind) {
			continue
		}
		targetPath := cleanRel(entry.TargetPath)
		if targetPath == "." || targetPath == "" {
			targetPath = sourcePath
		}
		record := control.SoftDelete{
			Version:            control.CurrentVersion,
			ID:                 StableID(sessionID, sourcePath, targetPath),
			SessionID:          sessionID,
			ProfileID:          opts.ProfileID,
			TargetID:           opts.TargetID,
			RootID:             rootID(opts),
			PreviousSessionID:  opts.PreviousManifest.SessionID,
			PreviousManifestID: opts.PreviousManifest.ID,
			SourcePath:         sourcePath,
			TargetPath:         targetPath,
			Kind:               entry.Kind,
			Size:               entry.Size,
			Digest:             entry.Digest,
			DetectedAt:         detectedAt.UTC().Format(time.RFC3339Nano),
			Reason:             softDeleteReason(entry.Kind, current[sourcePath]),
		}
		if err := record.Validate(); err != nil {
			return Result{}, err
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].SourcePath == records[j].SourcePath {
			return records[i].TargetPath < records[j].TargetPath
		}
		return records[i].SourcePath < records[j].SourcePath
	})
	return Result{Records: records}, nil
}

func rootID(opts Options) string {
	if strings.TrimSpace(opts.RootID) != "" {
		return opts.RootID
	}
	return opts.PreviousManifest.RootID
}

func StableID(sessionID, sourcePath, targetPath string) string {
	parts := []string{
		sessionID,
		cleanRel(sourcePath),
		cleanRel(targetPath),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return artifactPrefix(sessionID) + "-del_" + hex.EncodeToString(sum[:8])
}

func artifactPrefix(sessionID string) string {
	var b strings.Builder
	for _, r := range sessionID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._-", r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func currentEntries(result scan.Result) map[string]scan.Entry {
	paths := make(map[string]scan.Entry, len(result.Entries))
	for _, entry := range result.Entries {
		paths[cleanRel(entry.Path)] = entry
	}
	return paths
}

func sameLogicalSourceKind(previous string, current scan.Kind) bool {
	switch previous {
	case "file":
		return current == scan.KindRegular
	case "symlink":
		return current == scan.KindSymlink
	default:
		return true
	}
}

func softDeleteReason(previous string, current scan.Entry) string {
	if strings.TrimSpace(string(current.Kind)) == "" {
		return "present in previous manifest and absent from current source scan"
	}
	return fmt.Sprintf("present in previous manifest as %s and current source scan observes %s", previous, current.Kind)
}

func cleanRel(path string) string {
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
}
