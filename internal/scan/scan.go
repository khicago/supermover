package scan

import (
	"fmt"
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
	SymlinkTarget string      `json:"symlink_target,omitempty"`
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
				audit.New(rel, "", audit.SeverityWarning, "scan_error", walkErr.Error()),
				map[string]string{"path": filepath.ToSlash(path)},
			))
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		info, err := os.Lstat(path)
		if err != nil {
			result.Audit = append(result.Audit, audit.WithDetected(
				audit.New(rel, "", audit.SeverityWarning, "scan_error", err.Error()),
				map[string]string{"path": filepath.ToSlash(path)},
			))
			return nil
		}

		entry := entryFromInfo(absRoot, path, rel, info)
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

func entryFromInfo(root, path, rel string, info fs.FileInfo) Entry {
	mode := info.Mode()
	entry := Entry{
		Path:       rel,
		Kind:       kindFromMode(mode),
		Hidden:     isHidden(rel),
		Size:       info.Size(),
		Mode:       mode,
		ModTime:    info.ModTime(),
		Executable: mode.IsRegular() && mode.Perm()&0o111 != 0,
	}
	if mode&os.ModeSymlink != 0 {
		if target, err := os.Readlink(path); err == nil {
			entry.SymlinkTarget = filepath.ToSlash(target)
		}
	}
	if path == root {
		entry.Hidden = false
	}
	return entry
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
