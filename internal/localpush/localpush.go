package localpush

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/agentkb"
	"github.com/khicago/supermover/internal/audit"
	"github.com/khicago/supermover/internal/control"
	"github.com/khicago/supermover/internal/durable"
	"github.com/khicago/supermover/internal/profile"
	"github.com/khicago/supermover/internal/scan"
	"github.com/khicago/supermover/internal/transaction"
)

type Options struct {
	Profile   profile.Profile
	TargetDir string
	SessionID string
	Now       time.Time
}

type Result struct {
	SessionID  string
	Entries    int
	Copied     int
	Warnings   int
	Influences int
}

func Run(opts Options) (Result, error) {
	if err := opts.Profile.Validate(); err != nil {
		return Result{}, err
	}
	if len(opts.Profile.Roots) != 1 {
		return Result{}, fmt.Errorf("local push requires exactly one root for now")
	}
	if strings.TrimSpace(opts.TargetDir) == "" {
		return Result{}, fmt.Errorf("target directory is required")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sessionID := opts.SessionID
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "session-" + now.UTC().Format("20060102T150405Z")
	}

	root := opts.Profile.Roots[0]
	scanResult, err := scan.Scan(root.Path)
	if err != nil {
		return Result{}, err
	}
	influences := agentkb.Detect(scanResult.Entries)
	controlDir := control.ControlDir(opts.TargetDir)
	layout := transaction.NewLayout(controlDir)
	record, err := transaction.NewSessionRecord(sessionID, now)
	if err != nil {
		return Result{}, err
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		return Result{}, err
	}

	var manifestEntries []control.ManifestEntry
	warnings := append([]audit.Record(nil), scanResult.Audit...)
	copied := 0
	for _, entry := range scanResult.Entries {
		if entry.Path == "." {
			continue
		}
		sourcePath := filepath.Join(root.Path, filepath.FromSlash(entry.Path))
		targetPath := filepath.Join(opts.TargetDir, filepath.FromSlash(entry.Path))
		switch entry.Kind {
		case scan.KindDir:
			if err := os.MkdirAll(targetPath, entry.Mode.Perm()); err != nil {
				return Result{}, fmt.Errorf("create target directory %q: %w", targetPath, err)
			}
			manifestEntries = append(manifestEntries, manifestEntry(entry, "dir", ""))
		case scan.KindRegular:
			digest, err := copyRegular(sourcePath, targetPath, entry.Mode.Perm(), entry.ModTime)
			if err != nil {
				return Result{}, err
			}
			copied++
			manifestEntries = append(manifestEntries, manifestEntry(entry, "file", digest))
		case scan.KindSymlink:
			warnings = append(warnings, audit.WithDetected(
				audit.New(entry.Path, entry.Path, audit.SeverityWarning, "symlink_not_copied", "symlink copy is not implemented in local push"),
				map[string]string{"target": entry.SymlinkTarget},
			))
			manifestEntries = append(manifestEntries, manifestEntry(entry, "symlink", ""))
		default:
			warnings = append(warnings, audit.WithDetected(
				audit.New(entry.Path, entry.Path, audit.SeverityWarning, "special_not_copied", "special file copy is not supported"),
				map[string]string{"mode": entry.Mode.String()},
			))
		}
	}

	record, err = record.WithState(transaction.StateStaged, now)
	if err != nil {
		return Result{}, err
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		return Result{}, err
	}
	if err := writeControlArtifacts(opts.TargetDir, opts.Profile, sessionID, now, manifestEntries, warnings, influences); err != nil {
		return Result{}, err
	}
	record, err = record.WithState(transaction.StatePublished, now)
	if err != nil {
		return Result{}, err
	}
	if err := layout.WriteSessionRecord(record); err != nil {
		return Result{}, err
	}
	return Result{
		SessionID:  sessionID,
		Entries:    len(manifestEntries),
		Copied:     copied,
		Warnings:   len(warnings),
		Influences: len(influences),
	}, nil
}

func copyRegular(sourcePath, targetPath string, mode os.FileMode, modTime time.Time) (string, error) {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", fmt.Errorf("create target parent %q: %w", filepath.Dir(targetPath), err)
	}
	in, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open source file %q: %w", sourcePath, err)
	}
	defer in.Close()

	temp, err := os.CreateTemp(filepath.Dir(targetPath), ".supermover-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create target temp file: %w", err)
	}
	tempName := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(temp, hasher), in); err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("copy %q to temp file: %w", sourcePath, err)
	}
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("chmod temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}
	if err := durable.PromoteFile(tempName, targetPath); err != nil {
		return "", err
	}
	if !modTime.IsZero() {
		if err := os.Chtimes(targetPath, modTime, modTime); err != nil {
			return "", fmt.Errorf("preserve modification time for %q: %w", targetPath, err)
		}
	}
	cleanup = false
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func manifestEntry(entry scan.Entry, kind string, digest string) control.ManifestEntry {
	return control.ManifestEntry{
		Path:       entry.Path,
		Kind:       kind,
		Size:       entry.Size,
		ModTime:    entry.ModTime.UTC().Format(time.RFC3339Nano),
		Digest:     digest,
		TargetPath: entry.Path,
	}
}

func writeControlArtifacts(targetDir string, p profile.Profile, sessionID string, now time.Time, entries []control.ManifestEntry, warnings []audit.Record, influences []agentkb.Influence) error {
	profilePayload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal profile snapshot: %w", err)
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	snapshot := control.ProfileSnapshot{
		Version:    control.CurrentVersion,
		ID:         "profile-" + sessionID,
		ProfileID:  p.ProfileID,
		SessionID:  sessionID,
		CapturedAt: stamp,
		Profile:    profilePayload,
	}
	if path, err := control.Path(targetDir, control.ArtifactProfileSnapshot, snapshot.ID); err != nil {
		return err
	} else if err := control.WriteFile(path, snapshot); err != nil {
		return err
	}
	receipt := control.SessionReceipt{
		Version:   control.CurrentVersion,
		ID:        sessionID,
		ProfileID: p.ProfileID,
		TargetID:  p.Target.TargetID,
		StartedAt: stamp,
		EndedAt:   stamp,
		Status:    "published",
	}
	if path, err := control.Path(targetDir, control.ArtifactSessionReceipt, sessionID); err != nil {
		return err
	} else if err := control.WriteFile(path, receipt); err != nil {
		return err
	}
	manifest := control.Manifest{
		Version:   control.CurrentVersion,
		ID:        "manifest-" + sessionID,
		SessionID: sessionID,
		CreatedAt: stamp,
		Entries:   entries,
	}
	if path, err := control.Path(targetDir, control.ArtifactManifest, sessionID); err != nil {
		return err
	} else if err := control.WriteFile(path, manifest); err != nil {
		return err
	}
	for i, warning := range warnings {
		doc := control.Warning{
			Version:   control.CurrentVersion,
			ID:        fmt.Sprintf("%s-%03d-%s", sessionID, i+1, warning.ID),
			SessionID: sessionID,
			Code:      warning.Kind,
			Message:   warning.Reason,
			Paths:     []string{warning.Path},
			CreatedAt: stamp,
		}
		if path, err := control.Path(targetDir, control.ArtifactWarning, doc.ID); err != nil {
			return err
		} else if err := control.WriteFile(path, doc); err != nil {
			return err
		}
	}
	if len(influences) > 0 {
		if err := writeAgentInfluence(targetDir, sessionID, stamp, influences); err != nil {
			return err
		}
	}
	return nil
}

func writeAgentInfluence(targetDir, sessionID, stamp string, influences []agentkb.Influence) error {
	type document struct {
		Version   int                 `json:"version"`
		SessionID string              `json:"session_id"`
		CreatedAt string              `json:"created_at"`
		Influence []agentkb.Influence `json:"influence"`
	}
	path := filepath.Join(control.ControlDir(targetDir), "agent", sessionID+"-influence.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(document{Version: 1, SessionID: sessionID, CreatedAt: stamp, Influence: influences}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
