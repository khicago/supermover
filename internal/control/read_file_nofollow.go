package control

import (
	"fmt"
	"path/filepath"

	"github.com/khicago/supermover/internal/pathguard"
)

func ReadFileNoSymlink[T Document](path string) (T, error) {
	file, err := openFileNoSymlink(path)
	if err != nil {
		var zero T
		return zero, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		var zero T
		return zero, err
	}
	if !info.Mode().IsRegular() {
		var zero T
		return zero, fmt.Errorf("control artifact %q is not a regular file", path)
	}
	return Read[T](file)
}

func ReadFileNoSymlinkUnderRoot[T Document](root string, path string) (T, error) {
	var zero T
	if err := pathguard.EnsureDirectory(root, filepath.Dir(path)); err != nil {
		return zero, err
	}
	return ReadFileNoSymlink[T](path)
}
