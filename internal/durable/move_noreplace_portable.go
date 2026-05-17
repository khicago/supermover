//go:build !darwin && !linux && !windows

package durable

import (
	"fmt"
)

func renameFileNoReplace(sourcePath, finalPath string) error {
	return fmt.Errorf("%w: atomic no-replace move is not implemented for this platform", ErrValidationFailure)
}
