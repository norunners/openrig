package invocation

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/norunners/openrig/internal/result"
)

func TestContextStoresInvocation(t *testing.T) {
	source := Invocation{
		RuntimeID:  "runtime_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		SessionID:  "session-1",
		TurnID:     "turn_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		WorktreeID: "wt_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		CWD:        "/workspace",
		ToolCallID: "tool_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		TraceID:    "4bf92f3577b34da6a3ce929d0e0e4736",
		Transport:  "sse",
	}
	expected := source
	ctx := With(context.Background(), source)
	source.CWD = "/changed-after-storage"

	actual, ok := From(ctx)
	if !ok {
		t.Fatal("From did not find invocation")
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch invocation (-expected, +actual):\n%s", diff)
	}
}

func TestFromNilContext(t *testing.T) {
	_, ok := From(nil)
	if ok {
		t.Fatal("From found invocation in nil context")
	}
}

func TestInvocationOwnerExcludesSessionCorrelation(t *testing.T) {
	invocation := Invocation{
		SessionID:  " session-1 ",
		TurnID:     " turn-1 ",
		WorktreeID: " worktree-1 ",
		CWD:        " /workspace ",
	}
	expected := Owner{
		TurnID:     "turn-1",
		WorktreeID: "worktree-1",
		CWD:        "/workspace",
	}
	actual := invocation.Owner()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch owner (-expected, +actual):\n%s", diff)
	}
}

func TestRequireOwner(t *testing.T) {
	tests := []struct {
		name         string
		expected     Owner
		actual       Owner
		expectedCode result.Code
	}{
		{
			name: "exact turn owner",
			expected: Owner{
				TurnID:     "turn-1",
				WorktreeID: "wt-1",
				CWD:        "/workspace",
			},
			actual: Owner{
				TurnID:     " turn-1 ",
				WorktreeID: " wt-1 ",
				CWD:        " /workspace ",
			},
		},
		{
			name: "exact freestyle workspace owner",
			expected: Owner{
				CWD: "/workspace",
			},
			actual: Owner{
				CWD: "/workspace",
			},
		},
		{
			name: "exact freestyle turn owner",
			expected: Owner{
				TurnID: "turn-1",
				CWD:    "/workspace",
			},
			actual: Owner{
				TurnID: "turn-1",
				CWD:    "/workspace",
			},
		},
		{
			name: "exact worktree resource owner",
			expected: Owner{
				WorktreeID: "wt-1",
				CWD:        "/workspace",
			},
			actual: Owner{
				WorktreeID: "wt-1",
				CWD:        "/workspace",
			},
		},
		{
			name:         "ownerless",
			expectedCode: result.CodeForbidden,
		},
		{
			name: "whitespace-only workspace",
			expected: Owner{
				TurnID: "turn-1",
				CWD:    " ",
			},
			actual: Owner{
				TurnID: "turn-1",
				CWD:    "\t",
			},
			expectedCode: result.CodeForbidden,
		},
		{
			name: "different turn",
			expected: Owner{
				TurnID: "turn-1",
				CWD:    "/workspace",
			},
			actual: Owner{
				TurnID: "turn-2",
				CWD:    "/workspace",
			},
			expectedCode: result.CodeForbidden,
		},
		{
			name: "different worktree",
			expected: Owner{
				WorktreeID: "wt-1",
				CWD:        "/workspace",
			},
			actual: Owner{
				WorktreeID: "wt-2",
				CWD:        "/workspace",
			},
			expectedCode: result.CodeForbidden,
		},
		{
			name: "different workspace",
			expected: Owner{
				CWD: "/workspace",
			},
			actual: Owner{
				CWD: "/other",
			},
			expectedCode: result.CodeForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := RequireOwner(test.expected, test.actual)
			if test.expectedCode == "" {
				if err != nil {
					t.Fatalf("RequireOwner returned error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("RequireOwner error = nil, expected error")
			}
			if diff := cmp.Diff(test.expectedCode, result.ErrorOf(err).Code); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestRequireInvocation(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		expected := Invocation{
			TurnID: "turn-1",
			CWD:    "/workspace",
		}
		actual, err := Require(With(context.Background(), expected))
		if err != nil {
			t.Fatalf("Require returned error: %v", err)
		}
		if diff := cmp.Diff(expected, actual); diff != "" {
			t.Errorf("mismatch invocation (-expected, +actual):\n%s", diff)
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, err := Require(context.Background())
		if err == nil {
			t.Fatal("Require error = nil, expected error")
		}
		expected := result.CodeFailedPrecondition
		actual := result.ErrorOf(err).Code
		if diff := cmp.Diff(expected, actual); diff != "" {
			t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
		}
	})
}
