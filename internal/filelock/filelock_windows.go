//go:build windows

package filelock

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	lockfileExclusiveLock = 0x00000002
	lockRangeLow          = ^uint32(0)
	lockRangeHigh         = ^uint32(0)
)

var (
	modkernel32        = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx     = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx   = modkernel32.NewProc("UnlockFileEx")
	errReparseLockPath = fmt.Errorf("lock path is a reparse point")
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
	dirHandle, err := openLockDirectory(dir)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(dirHandle)
	handle, err := openLockFileAt(dirHandle, name)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name)
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, fmt.Errorf("open lock path %q: invalid handle", path)
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		_ = file.Close()
		return nil, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = file.Close()
		return nil, fmt.Errorf("%w: %q", errReparseLockPath, path)
	}
	var overlapped windows.Overlapped
	if err := lockFileEx(handle, lockfileExclusiveLock, lockRangeLow, lockRangeHigh, &overlapped); err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() {
		_ = unlockFileEx(handle, lockRangeLow, lockRangeHigh, &overlapped)
		_ = file.Close()
	}, nil
}

func validLockName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.Contains(name, "/") && !strings.Contains(name, `\`)
}

func openLockDirectory(dir string) (windows.Handle, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return windows.InvalidHandle, err
	}
	objectName, err := windows.NewNTUnicodeString(`\??\` + abs)
	if err != nil {
		return windows.InvalidHandle, err
	}
	attrs := &windows.OBJECT_ATTRIBUTES{
		ObjectName: objectName,
		Attributes: windows.OBJ_DONT_REPARSE,
	}
	attrs.Length = uint32(unsafe.Sizeof(*attrs))
	var handle windows.Handle
	err = windows.NtCreateFile(
		&handle,
		windows.FILE_GENERIC_READ|windows.FILE_LIST_DIRECTORY,
		attrs,
		&windows.IO_STATUS_BLOCK{},
		nil,
		windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		windows.FILE_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0,
		0,
	)
	if err != nil {
		return windows.InvalidHandle, mapNTCreateFileError(err)
	}
	return handle, nil
}

func openLockFileAt(dir windows.Handle, name string) (windows.Handle, error) {
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return windows.InvalidHandle, err
	}
	attrs := &windows.OBJECT_ATTRIBUTES{
		RootDirectory: dir,
		ObjectName:    objectName,
		Attributes:    windows.OBJ_DONT_REPARSE,
	}
	attrs.Length = uint32(unsafe.Sizeof(*attrs))
	var handle windows.Handle
	err = windows.NtCreateFile(
		&handle,
		windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE,
		attrs,
		&windows.IO_STATUS_BLOCK{},
		nil,
		windows.FILE_ATTRIBUTE_NORMAL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN_IF,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_REPARSE_POINT,
		0,
		0,
	)
	if err != nil {
		return windows.InvalidHandle, mapNTCreateFileError(err)
	}
	return handle, nil
}

func mapNTCreateFileError(err error) error {
	status, ok := err.(windows.NTStatus)
	if !ok {
		return err
	}
	switch status {
	case windows.STATUS_REPARSE_POINT_ENCOUNTERED:
		return syscall.ELOOP
	case windows.STATUS_NOT_A_DIRECTORY:
		return syscall.ENOTDIR
	case windows.STATUS_FILE_IS_A_DIRECTORY:
		return syscall.EISDIR
	case windows.STATUS_OBJECT_NAME_COLLISION:
		return syscall.EEXIST
	default:
		return status.Errno()
	}
}

func lockFileEx(file windows.Handle, flags uint32, bytesLow uint32, bytesHigh uint32, overlapped *windows.Overlapped) error {
	r1, _, e1 := syscall.Syscall6(procLockFileEx.Addr(), 6, uintptr(file), uintptr(flags), 0, uintptr(bytesLow), uintptr(bytesHigh), uintptr(unsafe.Pointer(overlapped)))
	if r1 == 0 {
		return e1
	}
	return nil
}

func unlockFileEx(file windows.Handle, bytesLow uint32, bytesHigh uint32, overlapped *windows.Overlapped) error {
	r1, _, e1 := syscall.Syscall6(procUnlockFileEx.Addr(), 5, uintptr(file), 0, uintptr(bytesLow), uintptr(bytesHigh), uintptr(unsafe.Pointer(overlapped)), 0)
	if r1 == 0 {
		return e1
	}
	return nil
}
