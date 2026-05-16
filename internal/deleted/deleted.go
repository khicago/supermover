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

	current := currentPaths(opts.CurrentScan)
	var records []control.SoftDelete
	for _, entry := range opts.PreviousManifest.Entries {
		if entry.Path == "." || entry.Kind == "dir" {
			continue
		}
		sourcePath := cleanRel(entry.Path)
		if _, ok := current[sourcePath]; ok {
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
			Reason:             "present in previous manifest and absent from current source scan",
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
	return "del_" + hex.EncodeToString(sum[:8])
}

func currentPaths(result scan.Result) map[string]struct{} {
	paths := make(map[string]struct{}, len(result.Entries))
	for _, entry := range result.Entries {
		paths[cleanRel(entry.Path)] = struct{}{}
	}
	return paths
}

func cleanRel(path string) string {
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
}
