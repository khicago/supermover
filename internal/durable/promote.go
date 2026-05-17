package durable

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
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
	if err := os.Rename(tempPath, finalPath); err != nil {
		return wrap("rename", finalPath, err)
	}
	if err := SyncDirBestEffort(filepath.Dir(finalPath)); err != nil {
		return err
	}
	return nil
}

func PromoteFileNoReplace(tempPath string, finalPath string) error {
	if tempPath == "" || finalPath == "" {
		return wrap("promote without replace", finalPath, fmt.Errorf("%w: temp and final paths are required", ErrValidationFailure))
	}
	if filepath.Clean(tempPath) == filepath.Clean(finalPath) {
		return wrap("promote without replace", finalPath, fmt.Errorf("%w: temp and final paths must differ", ErrValidationFailure))
	}
	if err := SyncFile(tempPath); err != nil {
		return err
	}
	if err := os.Link(tempPath, finalPath); err == nil {
		if err := SyncDirBestEffort(filepath.Dir(finalPath)); err != nil {
			return err
		}
		if err := os.Remove(tempPath); err != nil {
			return wrap("remove temp", tempPath, err)
		}
		return SyncDirBestEffort(filepath.Dir(finalPath))
	} else {
		if !canFallbackNoReplace(err) {
			return wrap("link without replace", finalPath, err)
		}
		if err := copyFileNoReplace(tempPath, finalPath); err != nil {
			return wrap("copy without replace", finalPath, err)
		}
		if err := SyncDirBestEffort(filepath.Dir(finalPath)); err != nil {
			return err
		}
		if err := os.Remove(tempPath); err != nil {
			return wrap("remove temp", tempPath, err)
		}
		return SyncDirBestEffort(filepath.Dir(finalPath))
	}
}

func MoveFileNoReplace(sourcePath string, finalPath string) error {
	if sourcePath == "" || finalPath == "" {
		return wrap("move without replace", finalPath, fmt.Errorf("%w: source and final paths are required", ErrValidationFailure))
	}
	if filepath.Clean(sourcePath) == filepath.Clean(finalPath) {
		return wrap("move without replace", finalPath, fmt.Errorf("%w: source and final paths must differ", ErrValidationFailure))
	}
	if err := renameFileNoReplace(sourcePath, finalPath); err != nil {
		return wrap("rename without replace", finalPath, err)
	}
	if err := SyncDirBestEffort(filepath.Dir(sourcePath)); err != nil {
		return err
	}
	return SyncDirBestEffort(filepath.Dir(finalPath))
}

func canFallbackNoReplace(err error) bool {
	return errors.Is(err, syscall.EXDEV)
}

func copyFileNoReplace(tempPath, finalPath string) error {
	src, err := os.Open(tempPath)
	if err != nil {
		return err
	}
	defer src.Close()

	mode := os.FileMode(0o600)
	modTime := time.Time{}
	if info, err := src.Stat(); err == nil {
		mode = info.Mode().Perm()
		modTime = info.ModTime()
	}
	finalDir := filepath.Dir(finalPath)
	dst, err := os.CreateTemp(finalDir, ".supermover-promote-*.tmp")
	if err != nil {
		return err
	}
	tmpName := dst.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Sync(); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	if !modTime.IsZero() {
		if err := os.Chtimes(tmpName, modTime, modTime); err != nil {
			return err
		}
	}
	if err := SyncFile(tmpName); err != nil {
		return err
	}
	if err := os.Link(tmpName, finalPath); err != nil {
		return err
	}
	if err := SyncDirBestEffort(finalDir); err != nil {
		return err
	}
	cleanup = false
	if err := os.Remove(tmpName); err != nil {
		return wrap("remove temp", tmpName, err)
	}
	return SyncDirBestEffort(finalDir)
}

func SyncFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		if os.IsPermission(err) {
			return syncFileWithTemporaryOwnerRead(path)
		}
		return wrap("open for sync", path, err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return wrap("sync file", path, err)
	}
	return nil
}

func syncFileWithTemporaryOwnerRead(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return wrap("stat for sync", path, err)
	}
	if !info.Mode().IsRegular() {
		return wrap("open for sync", path, os.ErrPermission)
	}
	originalMode := info.Mode().Perm()
	if err := os.Chmod(path, originalMode|0o400); err != nil {
		return wrap("chmod for sync", path, err)
	}
	f, openErr := os.OpenFile(path, os.O_RDONLY, 0)
	if restoreErr := os.Chmod(path, originalMode); restoreErr != nil {
		if openErr != nil {
			return errors.Join(wrap("open for sync", path, openErr), wrap("restore mode after sync open", path, restoreErr))
		}
		if f != nil {
			_ = f.Close()
		}
		return wrap("restore mode after sync open", path, restoreErr)
	}
	if openErr != nil {
		return wrap("open for sync", path, openErr)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return wrap("sync file", path, err)
	}
	return nil
}
