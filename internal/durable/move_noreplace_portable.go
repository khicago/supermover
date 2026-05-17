//go:build !darwin && !linux && !windows

package durable

import (
	"errors"
	"os"
)

func renameFileNoReplace(sourcePath, finalPath string) error {
	if _, err := os.Lstat(finalPath); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(sourcePath, finalPath); err != nil {
		return err
	}
	return nil
}
