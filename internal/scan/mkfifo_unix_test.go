//go:build !windows

package scan

import "syscall"

func unixMkfifo(path string) error {
	return syscall.Mkfifo(path, 0o600)
}
