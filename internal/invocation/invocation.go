// Package invocation carries immutable routing and ownership metadata for one
// OpenRig tool call.
package invocation

import (
	"context"
	"strings"

	"github.com/norunners/openrig/internal/result"
)

type Invocation struct {
	RuntimeID  string
	SessionID  string
	TurnID     string
	WorktreeID string
	CWD        string
	ToolCallID string
	TraceID    string
	Transport  string
}

// Scope is the canonical workspace returned by a lifecycle resource resolver.
type Scope struct {
	TurnID       string
	WorktreeID   string
	WorkspaceCWD string
}

// Owner is the exact workspace and lifecycle scope attached to a resource.
// SessionID is deliberately absent because transport sessions are correlation,
// not ownership or authorization.
type Owner struct {
	TurnID     string `json:"turn_id,omitempty"`
	WorktreeID string `json:"worktree_id,omitempty"`
	CWD        string `json:"cwd"`
}

func (i Invocation) Owner() Owner {
	return normalizeOwner(Owner{
		TurnID:     i.TurnID,
		WorktreeID: i.WorktreeID,
		CWD:        i.CWD,
	})
}

// Valid reports whether owner contains a workspace identity. CWD must already
// have been canonicalized by the routing or lifecycle layer. Freestyle
// resources are workspace-owned without treating session IDs as principals.
func (o Owner) Valid() bool {
	return normalizeOwner(o).CWD != ""
}

// RequireOwner rejects ownerless or non-identical resource access.
func RequireOwner(expected, actual Owner) error {
	expected = normalizeOwner(expected)
	actual = normalizeOwner(actual)
	if !expected.Valid() || !actual.Valid() || expected != actual {
		return result.NewError(
			result.CodeForbidden,
			"resource owner does not match invocation owner",
		)
	}
	return nil
}

func normalizeOwner(owner Owner) Owner {
	return Owner{
		TurnID:     strings.TrimSpace(owner.TurnID),
		WorktreeID: strings.TrimSpace(owner.WorktreeID),
		CWD:        strings.TrimSpace(owner.CWD),
	}
}

type contextKey struct{}

func With(ctx context.Context, invocation Invocation) context.Context {
	return context.WithValue(ctx, contextKey{}, invocation)
}

func From(ctx context.Context) (Invocation, bool) {
	if ctx == nil {
		return Invocation{}, false
	}
	invocation, ok := ctx.Value(contextKey{}).(Invocation)
	return invocation, ok
}

func Require(ctx context.Context) (Invocation, error) {
	invocation, ok := From(ctx)
	if !ok {
		return Invocation{}, result.NewError(
			result.CodeFailedPrecondition,
			"invocation context is unavailable",
		)
	}
	return invocation, nil
}
