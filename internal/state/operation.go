package state

import (
	"strings"

	"github.com/norunners/openrig/internal/ids"
)

const OperationIDPrefix = "op_"

// NewOperationID returns a sortable identifier for a durable lifecycle operation.
func NewOperationID() string {
	return ids.NewPrefixed(OperationIDPrefix)
}

// ValidOperationID reports whether operationID is a canonical prefixed ULID.
func ValidOperationID(operationID string) bool {
	return ids.ValidPrefixed(OperationIDPrefix, strings.TrimSpace(operationID))
}
