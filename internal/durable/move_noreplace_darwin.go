//go:build darwin

package durable

import "golang.org/x/sys/unix"

func renameFileNoReplace(sourcePath, finalPath string) error {
	return unix.RenameatxNp(unix.AT_FDCWD, sourcePath, unix.AT_FDCWD, finalPath, unix.RENAME_EXCL)
}
