// Package result defines the common structured result and error contract used
// by OpenRig-owned tools.
package result

import (
	"context"
	"errors"
)

// Code identifies a stable failure class an agent can use to choose its next
// action. Resource-specific details belong in the message, field, path, or
// suggestion rather than in an unbounded set of codes.
type Code string

const (
	CodeInvalidArgument    Code = "INVALID_ARGUMENT"
	CodeNotFound           Code = "NOT_FOUND"
	CodeFailedPrecondition Code = "FAILED_PRECONDITION"
	CodeForbidden          Code = "FORBIDDEN"
	CodeResourceBusy       Code = "RESOURCE_BUSY"
	CodeResourceExhausted  Code = "RESOURCE_EXHAUSTED"
	CodeCancelled          Code = "CANCELLED"
	CodeTimeout            Code = "TIMEOUT"
	CodeUnavailable        Code = "UNAVAILABLE"
	CodeMalformed          Code = "MALFORMED"
	CodeInternal           Code = "INTERNAL"
)

// Error is the stable error object returned inside an OpenRig envelope.
//
// The wrapped cause is available to Go callers through errors.Is and errors.As,
// but is never serialized.
//
// The zero value is invalid. Construct errors with NewError, Wrap, or ErrorOf.
type Error struct {
	Code       Code   `json:"code"`
	Message    string `json:"message"`
	Field      string `json:"field,omitempty"`
	Path       string `json:"path,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`

	cause error
}

// NewError creates a public error whose message is already safe to return to a
// client.
func NewError(code Code, message string) *Error {
	code, message = normalizeError(code, message)
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Wrap creates a public error while preserving cause for Go diagnostics.
//
// Causes are never appended to the public message because an error class alone
// cannot prove that cause text is safe to disclose.
func Wrap(code Code, message string, cause error) *Error {
	if cause == nil {
		return nil
	}
	err := NewError(code, message)
	err.cause = cause
	return err
}

func (e *Error) Error() string {
	prefix := string(e.Code)
	if e.Field != "" {
		prefix += ": field " + e.Field
	}
	if e.Path != "" {
		prefix += ": " + e.Path
	}
	return prefix + ": " + e.Message
}

func (e *Error) Unwrap() error {
	return e.cause
}

// WithField identifies the invalid or rejected input field.
func (e *Error) WithField(field string) *Error {
	e.Field = field
	return e
}

// WithPath identifies the filesystem path or persisted resource involved.
func (e *Error) WithPath(path string) *Error {
	e.Path = path
	return e
}

// WithSuggestion adds a concise next action for the agent.
func (e *Error) WithSuggestion(suggestion string) *Error {
	e.Suggestion = suggestion
	return e
}

// ErrorOf preserves an existing structured error and maps common Go lifecycle
// errors to stable public codes. Unknown errors become INTERNAL without
// exposing their messages.
func ErrorOf(err error) *Error {
	if err == nil {
		return nil
	}
	var resultErr *Error
	if errors.As(err, &resultErr) {
		code, message := normalizeError(resultErr.Code, resultErr.Message)
		if code == resultErr.Code && message == resultErr.Message {
			return resultErr
		}
		normalized := *resultErr
		normalized.Code = code
		normalized.Message = message
		normalized.cause = err
		return &normalized
	}
	switch {
	case errors.Is(err, context.Canceled):
		return &Error{
			Code:    CodeCancelled,
			Message: "operation cancelled",
			cause:   err,
		}
	case errors.Is(err, context.DeadlineExceeded):
		return &Error{
			Code:    CodeTimeout,
			Message: "operation timed out",
			cause:   err,
		}
	default:
		return &Error{
			Code:    CodeInternal,
			Message: "internal error",
			cause:   err,
		}
	}
}

func normalizeError(code Code, message string) (Code, string) {
	if !validCode(code) {
		code = CodeInternal
	}
	if message != "" {
		return code, message
	}
	if code == CodeInternal {
		return code, "internal error"
	}
	return code, "operation failed"
}

func validCode(code Code) bool {
	switch code {
	case CodeInvalidArgument,
		CodeNotFound,
		CodeFailedPrecondition,
		CodeForbidden,
		CodeResourceBusy,
		CodeResourceExhausted,
		CodeCancelled,
		CodeTimeout,
		CodeUnavailable,
		CodeMalformed,
		CodeInternal:
		return true
	default:
		return false
	}
}

// Envelope is the common structured result returned by OpenRig-owned tools.
//
// Data may accompany a failure only when it is useful partial output from the
// attempted operation. Warnings describe non-fatal issues. Truncated tells the
// client that returned data was intentionally bounded.
//
// The zero value is invalid. Construct envelopes with Success, Failure, or
// FailureWithData.
type Envelope[T any] struct {
	OK        bool     `json:"ok"`
	Data      *T       `json:"data,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Truncated bool     `json:"truncated,omitempty"`
	Error     *Error   `json:"error,omitempty"`
}

// Success returns a successful envelope with typed data.
func Success[T any](data T) Envelope[T] {
	return Envelope[T]{
		OK:   true,
		Data: &data,
	}
}

// Failure returns a failed envelope without partial data.
//
// Passing nil is a caller bug; it is defensively normalized to INTERNAL so
// result construction never crashes the runtime.
func Failure[T any](err error) Envelope[T] {
	resultErr := ErrorOf(err)
	if resultErr == nil {
		resultErr = NewError(CodeInternal, "internal error")
	}
	return Envelope[T]{Error: resultErr}
}

// FailureWithData returns a failed envelope with useful partial output.
func FailureWithData[T any](err error, data T) Envelope[T] {
	envelope := Failure[T](err)
	envelope.Data = &data
	return envelope
}
