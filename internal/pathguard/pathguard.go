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
