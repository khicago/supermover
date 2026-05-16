package durable

import (
	"fmt"
	"os"
	"path/filepath"
)

func PromoteFile(tempPath string, finalPath string) error {
	if tempPath == "" || finalPath == "" {
		return wrap("promote", finalPath, fmt.Errorf("%w: temp and final paths are required", ErrValidationFailure))
	}
	if filepath.Clean(tempPath) == filepath.Clean(finalPath) {
		return wrap("promote", finalPath, fmt.Errorf("%w: temp and final paths must differ", ErrValidationFailure))
	}
	if err := SyncFile(tempPath); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return wrap("create parent", filepath.Dir(finalPath), err)
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return wrap("rename", finalPath, err)
	}
	if err := SyncDirBestEffort(filepath.Dir(finalPath)); err != nil {
		return err
	}
	return nil
}

func SyncFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return wrap("open for sync", path, err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return wrap("sync file", path, err)
	}
	return nil
}
