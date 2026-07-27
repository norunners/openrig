// Package ids generates non-secret OpenRig runtime identifiers.
package ids

import (
	"strings"

	"github.com/oklog/ulid/v2"
)

// New returns a canonical 26-character ULID.
func New() string {
	return ulid.Make().String()
}

// NewPrefixed returns prefix followed by a canonical ULID.
func NewPrefixed(prefix string) string {
	return prefix + New()
}

// ValidPrefixed reports whether value consists of prefix and one canonical
// ULID. IDs in this package are opaque correlation values, not secrets.
func ValidPrefixed(prefix, value string) bool {
	suffix, ok := strings.CutPrefix(value, prefix)
	if !ok || len(suffix) != 26 {
		return false
	}
	parsed, err := ulid.ParseStrict(suffix)
	return err == nil && parsed.String() == suffix
}
