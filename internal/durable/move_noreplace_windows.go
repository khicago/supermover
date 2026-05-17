//go:build windows

package durable

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func renameFileNoReplace(sourcePath, finalPath string) error {
	source, err := windows.UTF16PtrFromString(sourcePath)
	if err != nil {
		return err
	}
	final, err := windows.UTF16PtrFromString(finalPath)
	if err != nil {
		return err
	}
	if err := windows.MoveFileEx(source, final, windows.MOVEFILE_WRITE_THROUGH); err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) || errors.Is(err, windows.ERROR_FILE_EXISTS) {
			return os.ErrExist
		}
		return err
	}
	return nil
}
