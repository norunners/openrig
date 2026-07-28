package state

import (
	"errors"
	"strings"
)

// ErrorCode classifies state-root failures.
type ErrorCode string

const (
	CodeInvalid             ErrorCode = "INVALID"
	CodeNotFound            ErrorCode = "NOT_FOUND"
	CodeMalformed           ErrorCode = "MALFORMED"
	CodeUnsupportedVersion  ErrorCode = "UNSUPPORTED_VERSION"
	CodeKindMismatch        ErrorCode = "KIND_MISMATCH"
	CodeIO                  ErrorCode = "IO"
	CodeUnsupportedPlatform ErrorCode = "UNSUPPORTED_PLATFORM"
)

// Error is returned by state primitives with a stable category.
type Error struct {
	Code    ErrorCode
	Path    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{string(e.Code)}
	if e.Path != "" {
		parts = append(parts, e.Path)
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.Err != nil && e.Err.Error() != e.Message {
		parts = append(parts, e.Err.Error())
	}
	return strings.Join(parts, ": ")
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// CodeOf returns the state error category, defaulting to CodeIO for foreign errors.
func CodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var stateErr *Error
	if errors.As(err, &stateErr) {
		return stateErr.Code
	}
	return CodeIO
}

// IsCode reports whether err is a state error with the requested category.
func IsCode(err error, code ErrorCode) bool {
	return CodeOf(err) == code
}

func stateError(code ErrorCode, path, message string, err error) *Error {
	return &Error{Code: code, Path: path, Message: message, Err: err}
}
