package ids

import (
	"strings"
	"testing"
)

func TestNewReturnsCanonicalULID(t *testing.T) {
	actual := New()
	if len(actual) != 26 {
		t.Fatalf("New length = %d, expected 26", len(actual))
	}
	if !ValidPrefixed("", actual) {
		t.Errorf("New result = %q, expected canonical ULID", actual)
	}
}

func TestNewPrefixedReturnsCanonicalULID(t *testing.T) {
	actual := NewPrefixed("tool_")
	if !ValidPrefixed("tool_", actual) {
		t.Errorf("NewPrefixed result = %q, expected tool-prefixed ULID", actual)
	}
}

func TestValidPrefixedRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "missing prefix",
			value: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		},
		{
			name:  "old hexadecimal suffix",
			value: "tool_deadbeef",
		},
		{
			name:  "lowercase ULID",
			value: "tool_" + strings.ToLower("01ARZ3NDEKTSV4RRFFQ69G5FAV"),
		},
		{
			name:  "invalid alphabet",
			value: "tool_01ARZ3NDEKTSV4RRFFQ69G5FAI",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if ValidPrefixed("tool_", test.value) {
				t.Errorf("ValidPrefixed(%q) = true, expected false", test.value)
			}
		})
	}
}
