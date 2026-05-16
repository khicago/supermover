//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package filelock

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// Lock takes an exclusive advisory lock on path until the returned function is called.
func Lock(path string) (func(), error) {
	return LockInDir(filepath.Dir(path), filepath.Base(path))
}

// LockInDir takes an exclusive advisory lock on name under dir until the returned function is called.
func LockInDir(dir, name string) (func(), error) {
	if !validLockName(name) {
		return nil, fmt.Errorf("invalid lock filename %q", name)
	}
	dirfd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(dirfd)
	fd, err := unix.Openat(dirfd, name, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), filepath.Join(dir, name))
	if file == nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("open lock path %q: invalid handle", filepath.Join(dir, name))
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("lock path %q is not a regular file", filepath.Join(dir, name))
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

func validLockName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.Contains(name, "/") && !strings.Contains(name, `\`)
}
