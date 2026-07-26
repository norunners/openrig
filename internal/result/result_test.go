package result

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCodeContract(t *testing.T) {
	expected := []Code{
		"INVALID_ARGUMENT",
		"NOT_FOUND",
		"FAILED_PRECONDITION",
		"FORBIDDEN",
		"RESOURCE_BUSY",
		"RESOURCE_EXHAUSTED",
		"CANCELLED",
		"TIMEOUT",
		"UNAVAILABLE",
		"MALFORMED",
		"INTERNAL",
	}
	actual := []Code{
		CodeInvalidArgument,
		CodeNotFound,
		CodeFailedPrecondition,
		CodeForbidden,
		CodeResourceBusy,
		CodeResourceExhausted,
		CodeCancelled,
		CodeTimeout,
		CodeUnavailable,
		CodeMalformed,
		CodeInternal,
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch codes (-expected, +actual):\n%s", diff)
	}
}

func TestNewErrorNormalizesRequiredFields(t *testing.T) {
	tests := []struct {
		name            string
		code            Code
		message         string
		expectedCode    Code
		expectedMessage string
	}{
		{
			name:            "empty code and message",
			expectedCode:    CodeInternal,
			expectedMessage: "internal error",
		},
		{
			name:            "empty code preserves safe message",
			message:         "request failed",
			expectedCode:    CodeInternal,
			expectedMessage: "request failed",
		},
		{
			name:            "unsupported code becomes internal",
			code:            Code("UNSUPPORTED"),
			message:         "request failed",
			expectedCode:    CodeInternal,
			expectedMessage: "request failed",
		},
		{
			name:            "internal empty message",
			code:            CodeInternal,
			expectedCode:    CodeInternal,
			expectedMessage: "internal error",
		},
		{
			name:            "non-internal empty message",
			code:            CodeInvalidArgument,
			expectedCode:    CodeInvalidArgument,
			expectedMessage: "operation failed",
		},
		{
			name:            "complete error is unchanged",
			code:            CodeNotFound,
			message:         "resource not found",
			expectedCode:    CodeNotFound,
			expectedMessage: "resource not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := NewError(test.code, test.message)
			expectedCode := test.expectedCode
			actualCode := actual.Code
			if diff := cmp.Diff(expectedCode, actualCode); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
			expectedMessage := test.expectedMessage
			actualMessage := actual.Message
			if diff := cmp.Diff(expectedMessage, actualMessage); diff != "" {
				t.Errorf("mismatch error message (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestErrorIncludesStructuredContextAndOmitsCauseFromJSON(t *testing.T) {
	cause := errors.New("private cause")
	err := Wrap(CodeInternal, "read state", cause).
		WithField("resource_id").
		WithPath("/state/resources/item_1/state.json").
		WithSuggestion("remove or repair the malformed record")

	expected := "INTERNAL: field resource_id: /state/resources/item_1/state.json: read state"
	actual := err.Error()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch error text (-expected, +actual):\n%s", diff)
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is(err, cause) = false, expected true")
	}

	data, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("json.Marshal returned error: %v", marshalErr)
	}
	expected = `{"code":"INTERNAL","message":"read state","field":"resource_id","path":"/state/resources/item_1/state.json","suggestion":"remove or repair the malformed record"}`
	actual = string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch error JSON (-expected, +actual):\n%s", diff)
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("permission denied")
	tests := []struct {
		name            string
		code            Code
		message         string
		cause           error
		expectedMessage string
		expectNil       bool
	}{
		{
			name:      "nil cause",
			code:      CodeInternal,
			message:   "read state",
			expectNil: true,
		},
		{
			name:            "internal cause is private",
			code:            CodeInternal,
			message:         "read state",
			cause:           cause,
			expectedMessage: "read state",
		},
		{
			name:            "internal default is private",
			code:            CodeInternal,
			cause:           cause,
			expectedMessage: "internal error",
		},
		{
			name:            "public cause is private",
			code:            CodeInvalidArgument,
			message:         "read config",
			cause:           cause,
			expectedMessage: "read config",
		},
		{
			name:            "public default is private",
			code:            CodeMalformed,
			cause:           cause,
			expectedMessage: "operation failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := Wrap(test.code, test.message, test.cause)
			if test.expectNil {
				if actual != nil {
					t.Errorf("expected Wrap() nil, actual %v", actual)
				}
				return
			}
			if actual == nil {
				t.Error("Wrap() = nil, expected error")
				return
			}
			expectedCode := test.code
			actualCode := actual.Code
			if diff := cmp.Diff(expectedCode, actualCode); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
			expectedMessage := test.expectedMessage
			actualMessage := actual.Message
			if diff := cmp.Diff(expectedMessage, actualMessage); diff != "" {
				t.Errorf("mismatch error message (-expected, +actual):\n%s", diff)
			}
			if !errors.Is(actual, test.cause) {
				t.Error("errors.Is(actual, cause) = false, expected true")
			}
		})
	}
}

func TestErrorOf(t *testing.T) {
	typed := NewError(CodeNotFound, "resource not found")
	unknown := errors.New("database password appeared in an internal error")
	tests := []struct {
		name            string
		err             error
		expected        *Error
		expectedCode    Code
		expectedMessage string
		expectedCause   error
	}{
		{
			name: "nil",
		},
		{
			name:          "typed error",
			err:           fmt.Errorf("load resource: %w", typed),
			expected:      typed,
			expectedCause: typed,
		},
		{
			name:            "cancelled",
			err:             fmt.Errorf("run command: %w", context.Canceled),
			expectedCode:    CodeCancelled,
			expectedMessage: "operation cancelled",
			expectedCause:   context.Canceled,
		},
		{
			name:            "deadline",
			err:             fmt.Errorf("run command: %w", context.DeadlineExceeded),
			expectedCode:    CodeTimeout,
			expectedMessage: "operation timed out",
			expectedCause:   context.DeadlineExceeded,
		},
		{
			name:            "unknown",
			err:             unknown,
			expectedCode:    CodeInternal,
			expectedMessage: "internal error",
			expectedCause:   unknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := ErrorOf(test.err)
			if test.err == nil {
				if actual != nil {
					t.Errorf("expected ErrorOf(nil) nil, actual %v", actual)
				}
				return
			}
			if actual == nil {
				t.Error("ErrorOf() = nil, expected error")
				return
			}
			if test.expected != nil && actual != test.expected {
				t.Errorf("expected existing error %p, actual %p", test.expected, actual)
			}
			if test.expectedCode != "" {
				expectedCode := test.expectedCode
				actualCode := actual.Code
				if diff := cmp.Diff(expectedCode, actualCode); diff != "" {
					t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
				}
			}
			if test.expectedMessage != "" {
				expectedMessage := test.expectedMessage
				actualMessage := actual.Message
				if diff := cmp.Diff(expectedMessage, actualMessage); diff != "" {
					t.Errorf("mismatch error message (-expected, +actual):\n%s", diff)
				}
			}
			if !errors.Is(actual, test.expectedCause) {
				t.Errorf("errors.Is(actual, %v) = false, expected true", test.expectedCause)
			}
		})
	}
}

func TestErrorOfNormalizesStructuredError(t *testing.T) {
	original := &Error{}
	wrapped := fmt.Errorf("operation failed: %w", original)
	actual := ErrorOf(wrapped)

	if actual == original {
		t.Errorf("expected normalized copy, actual original error %p", actual)
	}
	type errorState struct {
		Code    Code
		Message string
	}
	expected := errorState{
		Code:    CodeInternal,
		Message: "internal error",
	}
	actualState := errorState{
		Code:    actual.Code,
		Message: actual.Message,
	}
	if diff := cmp.Diff(expected, actualState); diff != "" {
		t.Errorf("mismatch normalized error (-expected, +actual):\n%s", diff)
	}
	expected = errorState{}
	actualState = errorState{
		Code:    original.Code,
		Message: original.Message,
	}
	if diff := cmp.Diff(expected, actualState); diff != "" {
		t.Errorf("mismatch original error (-expected, +actual):\n%s", diff)
	}
	if !errors.Is(actual, original) {
		t.Error("expected normalized error to contain original structured error")
	}
	if !errors.Is(actual, wrapped) {
		t.Error("expected normalized error to contain outer wrapped error")
	}
	if errors.Unwrap(actual) != wrapped {
		t.Errorf("expected direct cause %p, actual %p", wrapped, errors.Unwrap(actual))
	}
}

func TestEnvelopeJSONContract(t *testing.T) {
	type payload struct {
		Value string `json:"value"`
	}

	success := Success(payload{Value: "complete"})
	success.Warnings = []string{"one file was skipped"}
	success.Truncated = true

	tests := []struct {
		name         string
		envelope     any
		expectedJSON string
	}{
		{
			name:         "success",
			envelope:     success,
			expectedJSON: `{"ok":true,"data":{"value":"complete"},"warnings":["one file was skipped"],"truncated":true}`,
		},
		{
			name: "failure",
			envelope: Failure[payload](
				NewError(CodeInvalidArgument, "value is required").
					WithField("value").
					WithSuggestion("provide value"),
			),
			expectedJSON: `{"ok":false,"error":{"code":"INVALID_ARGUMENT","message":"value is required","field":"value","suggestion":"provide value"}}`,
		},
		{
			name: "failure with partial data",
			envelope: FailureWithData(
				NewError(CodeTimeout, "operation timed out"),
				payload{Value: "partial"},
			),
			expectedJSON: `{"ok":false,"data":{"value":"partial"},"error":{"code":"TIMEOUT","message":"operation timed out"}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.envelope)
			if err != nil {
				t.Fatalf("json.Marshal returned error: %v", err)
			}
			expected := test.expectedJSON
			actual := string(data)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("mismatch envelope JSON (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestEnvelopeConstructorsMaintainInvariants(t *testing.T) {
	type payload struct {
		Value string
	}
	type envelopeState struct {
		OK        bool
		HasData   bool
		HasError  bool
		ErrorCode Code
	}

	tests := []struct {
		name     string
		envelope Envelope[payload]
		expected envelopeState
	}{
		{
			name:     "success",
			envelope: Success(payload{Value: "complete"}),
			expected: envelopeState{
				OK:      true,
				HasData: true,
			},
		},
		{
			name:     "failure",
			envelope: Failure[payload](NewError(CodeNotFound, "resource not found")),
			expected: envelopeState{
				HasError:  true,
				ErrorCode: CodeNotFound,
			},
		},
		{
			name:     "failure with partial data",
			envelope: FailureWithData(NewError(CodeTimeout, ""), payload{Value: "partial"}),
			expected: envelopeState{
				HasData:   true,
				HasError:  true,
				ErrorCode: CodeTimeout,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := envelopeState{
				OK:       test.envelope.OK,
				HasData:  test.envelope.Data != nil,
				HasError: test.envelope.Error != nil,
			}
			if test.envelope.Error != nil {
				actual.ErrorCode = test.envelope.Error.Code
			}
			expected := test.expected
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("mismatch envelope state (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestFailureNormalizesNilError(t *testing.T) {
	type payload struct {
		Value string
	}
	type failureState struct {
		OK           bool
		HasData      bool
		HasError     bool
		ErrorCode    Code
		ErrorMessage string
	}

	tests := []struct {
		name     string
		envelope Envelope[payload]
		expected failureState
	}{
		{
			name:     "failure",
			envelope: Failure[payload](nil),
			expected: failureState{
				HasError:     true,
				ErrorCode:    CodeInternal,
				ErrorMessage: "internal error",
			},
		},
		{
			name:     "failure with partial data",
			envelope: FailureWithData(nil, payload{Value: "partial"}),
			expected: failureState{
				HasData:      true,
				HasError:     true,
				ErrorCode:    CodeInternal,
				ErrorMessage: "internal error",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := failureState{
				OK:       test.envelope.OK,
				HasData:  test.envelope.Data != nil,
				HasError: test.envelope.Error != nil,
			}
			if test.envelope.Error != nil {
				actual.ErrorCode = test.envelope.Error.Code
				actual.ErrorMessage = test.envelope.Error.Message
			}
			expected := test.expected
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("mismatch failure state (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestErrorMethodsPanicForNilReceiver(t *testing.T) {
	tests := []struct {
		name string
		call func(*Error)
	}{
		{
			name: "error",
			call: func(err *Error) {
				_ = err.Error()
			},
		},
		{
			name: "unwrap",
			call: func(err *Error) {
				_ = err.Unwrap()
			},
		},
		{
			name: "field",
			call: func(err *Error) {
				err.WithField("field")
			},
		},
		{
			name: "path",
			call: func(err *Error) {
				err.WithPath("/path")
			},
		},
		{
			name: "suggestion",
			call: func(err *Error) {
				err.WithSuggestion("retry")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("expected panic, actual nil")
				}
			}()
			test.call(nil)
		})
	}
}
