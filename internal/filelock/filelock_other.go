//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

package filelock

import (
	"fmt"
	"runtime"
)

// Lock is intentionally unsupported on platforms without a real cross-process
// file lock implementation.
func Lock(path string) (func(), error) {
	return nil, fmt.Errorf("file locks are not implemented on %s", runtime.GOOS)
}

// LockInDir is intentionally unsupported on platforms without a real cross-process
// file lock implementation.
func LockInDir(_, _ string) (func(), error) {
	return nil, fmt.Errorf("file locks are not implemented on %s", runtime.GOOS)
}
