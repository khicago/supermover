package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/khicago/supermover/internal/audit"
)

// Kind is the filesystem type detected without following symlinks.
type Kind string

const (
	KindRegular Kind = "regular"
	KindDir     Kind = "dir"
	KindSymlink Kind = "symlink"
	KindSpecial Kind = "special"
)

// Entry is a non-destructive observation of one filesystem path.
type Entry struct {
	Path          string      `json:"path"`
	Kind          Kind        `json:"kind"`
	Hidden        bool        `json:"hidden"`
	Size          int64       `json:"size"`
	Mode          fs.FileMode `json:"mode"`
	ModTime       time.Time   `json:"mtime"`
	Executable    bool        `json:"executable"`
	Digest        string      `json:"digest,omitempty"`
	SymlinkTarget string      `json:"symlink_target,omitempty"`

	observed fs.FileInfo
}

// Result contains scanned entries and warnings for unsupported/special paths.
type Result struct {
	Root    string         `json:"root"`
	Entries []Entry        `json:"entries"`
	Audit   []audit.Record `json:"audit,omitempty"`
}

// Scan observes root recursively. It records symlinks as symlinks and does not
// follow them.
func Scan(root string) (Result, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Result{}, err
	}
	rootInfo, err := os.Lstat(absRoot)
	if err != nil {
		return Result{}, err
	}
	if !rootInfo.IsDir() {
		return Result{}, fmt.Errorf("scan root %q is not a directory", root)
	}

	result := Result{Root: filepath.ToSlash(absRoot)}
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		rel := relPath(absRoot, path)
		if walkErr != nil {
			result.Audit = append(result.Audit, audit.WithDetected(
				audit.New(rel, "", audit.SeverityWarning, "scan_error", "walk error"),
				map[string]string{"error": walkErr.Error(), "path": filepath.ToSlash(path)},
			))
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		info, err := os.Lstat(path)
		if err != nil {
			result.Audit = append(result.Audit, audit.WithDetected(
				audit.New(rel, "", audit.SeverityWarning, "scan_error", "lstat error"),
				map[string]string{"error": err.Error(), "path": filepath.ToSlash(path)},
			))
			return nil
		}

		entry, err := entryFromInfo(absRoot, path, rel, info)
		if err != nil {
			result.Audit = append(result.Audit, audit.WithDetected(
				audit.New(rel, "", audit.SeverityWarning, "scan_error", "digest error"),
				map[string]string{"error": err.Error(), "path": filepath.ToSlash(path)},
			))
			return nil
		}
		result.Entries = append(result.Entries, entry)
		if entry.Kind == KindSpecial {
			result.Audit = append(result.Audit, audit.WithDetected(
				audit.New(entry.Path, "", audit.SeverityWarning, "special_file", "path is neither regular file, directory, nor symlink"),
				map[string]string{"mode": entry.Mode.String()},
			))
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	sort.Slice(result.Entries, func(i, j int) bool {
		return result.Entries[i].Path < result.Entries[j].Path
	})
	sort.Slice(result.Audit, func(i, j int) bool {
		return result.Audit[i].ID < result.Audit[j].ID
	})
	return result, nil
}

func entryFromInfo(root, path, rel string, info fs.FileInfo) (Entry, error) {
	mode := info.Mode()
	entry := Entry{
		Path:       rel,
		Kind:       kindFromMode(mode),
		Hidden:     isHidden(rel),
		Size:       info.Size(),
		Mode:       mode,
		ModTime:    info.ModTime(),
		Executable: mode.IsRegular() && mode.Perm()&0o111 != 0,
		observed:   info,
	}
	if mode.IsRegular() {
		digest, err := digestObservedRegular(path, info)
		if err != nil {
			return Entry{}, err
		}
		entry.Digest = digest
	}
	if mode&os.ModeSymlink != 0 {
		if target, err := os.Readlink(path); err == nil {
			entry.SymlinkTarget = filepath.ToSlash(target)
		}
	}
	if path == root {
		entry.Hidden = false
	}
	return entry, nil
}

func digestObservedRegular(path string, observed fs.FileInfo) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(observed, opened) {
		return "", fmt.Errorf("regular file changed before digest")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !after.Mode().IsRegular() || !os.SameFile(observed, after) || after.Size() != observed.Size() || after.ModTime() != observed.ModTime() || after.Mode().Perm() != observed.Mode().Perm() {
		return "", fmt.Errorf("regular file changed during digest")
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

// MatchesObservedRegular reports whether info still describes the exact
// regular file observed during Scan, including object identity when available.
func (e Entry) MatchesObservedRegular(info fs.FileInfo) bool {
	if info == nil || e.Kind != KindRegular || !info.Mode().IsRegular() {
		return false
	}
	if e.observed != nil && !os.SameFile(e.observed, info) {
		return false
	}
	return info.Size() == e.Size &&
		info.Mode().Perm() == e.Mode.Perm() &&
		info.ModTime().Equal(e.ModTime)
}

func kindFromMode(mode fs.FileMode) Kind {
	switch {
	case mode.IsRegular():
		return KindRegular
	case mode.IsDir():
		return KindDir
	case mode&os.ModeSymlink != 0:
		return KindSymlink
	default:
		return KindSpecial
	}
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}

func isHidden(rel string) bool {
	if rel == "." || rel == "" {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if strings.HasPrefix(part, ".") && part != "." && part != ".." {
			return true
		}
	}
	return false
}
