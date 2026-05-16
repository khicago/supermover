package pathguard

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ErrUnsafePath = errors.New("unsafe path")

const ReservedControlDir = ".supermover"

func SafeJoin(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: absolute path %q", ErrUnsafePath, rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: unsafe relative path %q", ErrUnsafePath, rel)
	}
	return filepath.Join(root, clean), nil
}

func IsReservedControlPath(path string) bool {
	path = filepath.ToSlash(path)
	first, _, _ := strings.Cut(path, "/")
	return strings.EqualFold(first, ReservedControlDir)
}

func EnsurePlainDirectory(dir string, mode os.FileMode) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	var missing []string
	current := abs
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%w: directory component %q is a symlink", ErrUnsafePath, current)
			}
			if !info.IsDir() {
				return fmt.Errorf("%w: directory component %q is not a directory", ErrUnsafePath, current)
			}
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		missing = append(missing, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	for i := len(missing) - 1; i >= 0; i-- {
		path := missing[i]
		if err := os.Mkdir(path, mode); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: directory component %q is a symlink", ErrUnsafePath, path)
		}
		if !info.IsDir() {
			return fmt.Errorf("%w: directory component %q is not a directory", ErrUnsafePath, path)
		}
	}
	return nil
}

// CanonicalPath returns an absolute path with the existing symlink prefix resolved.
func CanonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(abs)
	if real, err := filepath.EvalSymlinks(clean); err == nil {
		return real, nil
	}

	var suffix []string
	current := clean
	for {
		parent := filepath.Dir(current)
		if parent == current {
			return clean, nil
		}
		suffix = append([]string{filepath.Base(current)}, suffix...)
		if realParent, err := filepath.EvalSymlinks(parent); err == nil {
			parts := append([]string{realParent}, suffix...)
			return filepath.Join(parts...), nil
		}
		current = parent
	}
}

func SafeJoinParent(root, rel string) (string, error) {
	path, err := SafeJoin(root, rel)
	if err != nil {
		return "", err
	}
	if err := EnsureDirectory(root, filepath.Dir(path)); err != nil {
		return "", err
	}
	return path, nil
}

func SafeJoinDirectory(root, rel string) (string, error) {
	path, err := SafeJoin(root, rel)
	if err != nil {
		return "", err
	}
	if err := EnsureDirectory(root, path); err != nil {
		return "", err
	}
	return path, nil
}

func EnsureDirectory(root, dir string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	dirAbs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, dirAbs)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: path escapes root", ErrUnsafePath)
	}

	current := rootAbs
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: directory component %q is a symlink", ErrUnsafePath, current)
		}
		if !info.IsDir() {
			return fmt.Errorf("%w: directory component %q is not a directory", ErrUnsafePath, current)
		}
	}
	return nil
}
