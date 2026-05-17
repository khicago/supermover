//go:build linux

package durable

import "golang.org/x/sys/unix"

func renameFileNoReplace(sourcePath, finalPath string) error {
	return unix.Renameat2(unix.AT_FDCWD, sourcePath, unix.AT_FDCWD, finalPath, unix.RENAME_NOREPLACE)
}
