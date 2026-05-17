package pathguard

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

var ErrUnsafePath = errors.New("unsafe path")

const ReservedControlDir = ".supermover"
const MaxSymlinkTargetLen = 4096

func SafeJoin(root, rel string) (string, error) {
	if err := ValidateSlashRelativePath(rel, 0); err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

func IsReservedControlPath(path string) bool {
	path = pathpkg.Clean(filepath.ToSlash(path))
	first, _, _ := strings.Cut(path, "/")
	return strings.EqualFold(first, ReservedControlDir)
}

func ValidateRelativeSymlinkTarget(target string) error {
	if err := ValidateSlashRelativePath(target, MaxSymlinkTargetLen); err != nil {
		detail := strings.TrimPrefix(err.Error(), ErrUnsafePath.Error()+": data path ")
		return fmt.Errorf("%w: symlink target %s", ErrUnsafePath, detail)
	}
	if IsReservedControlPath(target) {
		return fmt.Errorf("%w: symlink target uses reserved control directory", ErrUnsafePath)
	}
	return nil
}

func ValidateSlashRelativePath(value string, maxLen int) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: data path is required", ErrUnsafePath)
	}
	if maxLen > 0 && len(value) > maxLen {
		return fmt.Errorf("%w: data path is too long", ErrUnsafePath)
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`) || strings.Contains(value, `\`) {
		return fmt.Errorf("%w: data path must be a slash-separated relative path", ErrUnsafePath)
	}
	if hasWindowsVolumeName(value) {
		return fmt.Errorf("%w: data path must not include a Windows volume name", ErrUnsafePath)
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("%w: data path contains unsafe segment %q", ErrUnsafePath, part)
		}
	}
	return nil
}

func hasWindowsVolumeName(path string) bool {
	if len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0]) {
		return true
	}
	return strings.HasPrefix(path, "//")
}

func isASCIILetter(value byte) bool {
	return ('A' <= value && value <= 'Z') || ('a' <= value && value <= 'z')
}

func EnsurePlainDirectory(root, dir string, mode os.FileMode) error {
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
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: path escapes root", ErrUnsafePath)
	}
	if err := ensurePlainRoot(rootAbs, mode); err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	current := rootAbs
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			if err := os.Mkdir(current, mode); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
			info, err = os.Lstat(current)
			if err != nil {
				return err
			}
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

func ensurePlainRoot(path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(path, mode); err != nil {
			return err
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: directory component %q is a symlink", ErrUnsafePath, path)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: directory component %q is not a directory", ErrUnsafePath, path)
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
