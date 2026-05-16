//go:build aix || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris

package durable

import "os"

func SyncDirBestEffort(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return wrap("open parent for sync", path, err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return nil
	}
	return nil
}
