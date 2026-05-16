package durable

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
)

type Status string

const (
	StatusOK                Status = "ok"
	StatusDiskFull          Status = "disk_full"
	StatusInterrupted       Status = "interrupted"
	StatusValidationFailure Status = "validation_failure"
	StatusIOError           Status = "io_error"
)

var (
	ErrDiskFull          = errors.New("disk full")
	ErrInterrupted       = errors.New("operation interrupted")
	ErrValidationFailure = errors.New("validation failure")
)

type Error struct {
	Status Status
	Op     string
	Path   string
	Err    error
}

func (e *Error) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("%s %s: %v", e.Op, e.Path, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func ClassifyError(err error) Status {
	if err == nil {
		return StatusOK
	}
	if errors.Is(err, ErrValidationFailure) {
		return StatusValidationFailure
	}
	if errors.Is(err, ErrDiskFull) || errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) {
		return StatusDiskFull
	}
	if errors.Is(err, ErrInterrupted) || errors.Is(err, syscall.EINTR) {
		return StatusInterrupted
	}
	if errors.Is(err, fs.ErrInvalid) {
		return StatusValidationFailure
	}
	return StatusIOError
}

func wrap(op string, path string, err error) error {
	if err == nil {
		return nil
	}
	status := ClassifyError(err)
	switch status {
	case StatusDiskFull:
		err = fmt.Errorf("%w: %v", ErrDiskFull, err)
	case StatusInterrupted:
		err = fmt.Errorf("%w: %v", ErrInterrupted, err)
	case StatusValidationFailure:
		err = fmt.Errorf("%w: %v", ErrValidationFailure, err)
	}
	return &Error{Status: status, Op: op, Path: path, Err: err}
}
