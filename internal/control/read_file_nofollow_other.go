//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

package control

import (
	"fmt"
	"os"
	"runtime"
)

func openFileNoSymlink(path string) (*os.File, error) {
	return nil, fmt.Errorf("no-symlink control artifact reads are not implemented on %s for %q", runtime.GOOS, path)
}
