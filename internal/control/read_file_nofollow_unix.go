//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package control

import (
	"os"

	"golang.org/x/sys/unix"
)

func openFileNoSymlink(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, os.ErrInvalid
	}
	return file, nil
}
