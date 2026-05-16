//go:build !(aix || darwin || dragonfly || freebsd || hurd || illumos || linux || netbsd || openbsd || solaris)

package durable

func SyncDirBestEffort(path string) error {
	return nil
}
