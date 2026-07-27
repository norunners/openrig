package tools

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/norunners/openrig/internal/ids"
	"github.com/norunners/openrig/internal/invocation"
	"github.com/norunners/openrig/internal/project"
	"github.com/norunners/openrig/internal/result"
)

func TestResolveCallRoutesOperationSpecificArguments(t *testing.T) {
	sourceRoot := t.TempDir()
	worktreeRoot := t.TempDir()
	resolver, err := project.NewResolver(project.ResolverOptions{
		Source: project.SourceFunc(func() map[string]project.Definition {
			return map[string]project.Definition{
				"openrig": {
					Root:    sourceRoot,
					Aliases: []string{"rig"},
				},
			}
		}),
	})
	if err != nil {
		t.Fatalf("NewResolver returned error: %v", err)
	}
	sourceRoot, err = resolver.ResolveRepo(sourceRoot)
	if err != nil {
		t.Fatalf("resolve source root: %v", err)
	}
	worktreeRoot, err = resolver.ResolveRepo(worktreeRoot)
	if err != nil {
		t.Fatalf("resolve worktree root: %v", err)
	}

	turnID := TurnID("turn_01ARZ3NDEKTSV4RRFFQ69G5FAV")
	endedTurnID := TurnID("turn_01ARZ3NDEKTSV4RRFFQ69G5FAW")
	worktreeID := WorktreeID("wt_01ARZ3NDEKTSV4RRFFQ69G5FAV")
	resources := &stubResourceResolver{
		activeTurns: map[TurnID]invocation.Scope{
			turnID: {
				TurnID:       string(turnID),
				WorktreeID:   string(worktreeID),
				WorkspaceCWD: worktreeRoot,
			},
		},
		turns: map[TurnID]invocation.Scope{
			turnID: {
				TurnID:       string(turnID),
				WorktreeID:   string(worktreeID),
				WorkspaceCWD: worktreeRoot,
			},
			endedTurnID: {
				TurnID:       string(endedTurnID),
				WorktreeID:   string(worktreeID),
				WorkspaceCWD: worktreeRoot,
			},
		},
		worktrees: map[WorktreeID]invocation.Scope{
			worktreeID: {
				WorktreeID:   string(worktreeID),
				WorkspaceCWD: worktreeRoot,
			},
		},
		revisions: map[RevisionID]invocation.Scope{
			RevisionID("rev_01"): {
				TurnID:       string(turnID),
				WorktreeID:   string(worktreeID),
				WorkspaceCWD: worktreeRoot,
			},
			RevisionID("rev_02"): {
				TurnID:       string(turnID),
				WorktreeID:   string(worktreeID),
				WorkspaceCWD: worktreeRoot,
			},
		},
	}
	turnOptions := RoutingOptions{
		Projects:  resolver,
		Resources: resources,
		RuntimeID: "runtime_01",
		Transport: "sse",
	}
	freestyleOptions := RoutingOptions{
		Scope:    FreestyleScope,
		Projects: resolver,
	}

	tests := []struct {
		name               string
		request            mcp.CallToolRequest
		options            RoutingOptions
		expectedInvocation invocation.Invocation
		expectedArguments  toolArguments
	}{
		{
			name:    "help has no workspace",
			request: routingRequest(HelpToolName, `{}`),
			options: RoutingOptions{},
			expectedInvocation: invocation.Invocation{
				Transport: "stdio",
			},
			expectedArguments: helpArguments{},
		},
		{
			name:    "worktree open resolves repository alias",
			request: routingRequest(WorktreeToolName, `{"op":"open","repo":"rig"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID: "runtime_01",
				CWD:       sourceRoot,
				Transport: "sse",
			},
			expectedArguments: worktreeOpenArguments{
				Op:   WorktreeOpOpen,
				Repo: "rig",
				Base: "HEAD",
			},
		},
		{
			name:    "worktree list without filter has no workspace",
			request: routingRequest(WorktreeToolName, `{"op":"list"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID: "runtime_01",
				Transport: "sse",
			},
			expectedArguments: worktreeListArguments{
				Op: WorktreeOpList,
			},
		},
		{
			name:    "worktree list resolves repository filter",
			request: routingRequest(WorktreeToolName, `{"op":"list","repo":"openrig"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID: "runtime_01",
				CWD:       sourceRoot,
				Transport: "sse",
			},
			expectedArguments: worktreeListArguments{
				Op:   WorktreeOpList,
				Repo: "openrig",
			},
		},
		{
			name:    "worktree status resolves worktree",
			request: routingRequest(WorktreeToolName, `{"op":"status","worktree_id":"wt_01ARZ3NDEKTSV4RRFFQ69G5FAV"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: worktreeStatusArguments{
				Op:         WorktreeOpStatus,
				WorktreeID: worktreeID,
			},
		},
		{
			name:    "worktree close resolves worktree",
			request: routingRequest(WorktreeToolName, `{"op":"close","worktree_id":"wt_01ARZ3NDEKTSV4RRFFQ69G5FAV"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: worktreeCloseArguments{
				Op:         WorktreeOpClose,
				WorktreeID: worktreeID,
			},
		},
		{
			name:    "worktree delete resolves worktree",
			request: routingRequest(WorktreeToolName, `{"op":"delete","worktree_id":"wt_01ARZ3NDEKTSV4RRFFQ69G5FAV"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: worktreeDeleteArguments{
				Op:         WorktreeOpDelete,
				WorktreeID: worktreeID,
			},
		},
		{
			name:    "turn begin resolves repository",
			request: routingRequest(TurnToolName, `{"op":"begin","repo":"openrig","goal":"Fix routing"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID: "runtime_01",
				CWD:       sourceRoot,
				Transport: "sse",
			},
			expectedArguments: turnBeginArguments{
				Op:   TurnOpBegin,
				Mode: TurnModeWorktree,
				Repo: "openrig",
				Goal: "Fix routing",
			},
		},
		{
			name:    "turn begin resolves reused worktree",
			request: routingRequest(TurnToolName, `{"op":"begin","worktree_id":"wt_01ARZ3NDEKTSV4RRFFQ69G5FAV","goal":"Review routing"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: turnBeginArguments{
				Op:         TurnOpBegin,
				Mode:       TurnModeWorktree,
				WorktreeID: worktreeID,
				Goal:       "Review routing",
			},
		},
		{
			name:    "turn status resolves ended turn",
			request: routingRequest(TurnToolName, `{"op":"status","turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAW"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				TurnID:     string(endedTurnID),
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: turnStatusArguments{
				Op:     TurnOpStatus,
				TurnID: endedTurnID,
			},
		},
		{
			name:    "turn status resolves worktree filter",
			request: routingRequest(TurnToolName, `{"op":"status","worktree_id":"wt_01ARZ3NDEKTSV4RRFFQ69G5FAV"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: turnStatusArguments{
				Op:         TurnOpStatus,
				WorktreeID: worktreeID,
			},
		},
		{
			name:    "turn status resolves repository filter",
			request: routingRequest(TurnToolName, `{"op":"status","repo":"openrig"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID: "runtime_01",
				CWD:       sourceRoot,
				Transport: "sse",
			},
			expectedArguments: turnStatusArguments{
				Op:   TurnOpStatus,
				Repo: "openrig",
			},
		},
		{
			name:    "turn status without filter has no workspace",
			request: routingRequest(TurnToolName, `{"op":"status"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID: "runtime_01",
				Transport: "sse",
			},
			expectedArguments: turnStatusArguments{
				Op: TurnOpStatus,
			},
		},
		{
			name:    "turn end requires active turn",
			request: routingRequest(TurnToolName, `{"op":"end","turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","outcome":"completed"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				TurnID:     string(turnID),
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: turnEndArguments{
				Op:      TurnOpEnd,
				TurnID:  turnID,
				Outcome: TurnOutcomeCompleted,
			},
		},
		{
			name:    "turn diff resolves ended turn",
			request: routingRequest(DiffToolName, `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAW"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				TurnID:     string(endedTurnID),
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: diffTurnArguments{
				TurnID: endedTurnID,
			},
		},
		{
			name:    "revision diff resolves common workspace",
			request: routingRequest(DiffToolName, `{"kind":"revision","from":"rev_01","to":"rev_02"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				TurnID:     string(turnID),
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: diffRevisionArguments{
				Kind: DiffKindRevision,
				From: RevisionID("rev_01"),
				To:   RevisionID("rev_02"),
			},
		},
		{
			name:    "worktree diff resolves worktree",
			request: routingRequest(DiffToolName, `{"kind":"worktree","worktree_id":"wt_01ARZ3NDEKTSV4RRFFQ69G5FAV"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: diffWorktreeArguments{
				Kind:       DiffKindWorktree,
				WorktreeID: worktreeID,
			},
		},
		{
			name:    "Git diff resolves ended turn",
			request: routingRequest(DiffToolName, `{"kind":"git","turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAW","from":"HEAD~1","to":"HEAD"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				TurnID:     string(endedTurnID),
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: diffGitArguments{
				Kind:   DiffKindGit,
				TurnID: endedTurnID,
				From:   "HEAD~1",
				To:     "HEAD",
			},
		},
		{
			name:    "Git diff resolves worktree",
			request: routingRequest(DiffToolName, `{"kind":"git","worktree_id":"wt_01ARZ3NDEKTSV4RRFFQ69G5FAV","from":"HEAD~1","to":"HEAD"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: diffGitArguments{
				Kind:       DiffKindGit,
				WorktreeID: worktreeID,
				From:       "HEAD~1",
				To:         "HEAD",
			},
		},
		{
			name:    "turn work resolves active turn",
			request: routingRequest(ShellToolName, `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","command":"pwd"}`),
			options: turnOptions,
			expectedInvocation: invocation.Invocation{
				RuntimeID:  "runtime_01",
				TurnID:     string(turnID),
				WorktreeID: string(worktreeID),
				CWD:        worktreeRoot,
				Transport:  "sse",
			},
			expectedArguments: shellArguments{
				scopeArguments: scopeArguments{
					TurnID: turnID,
				},
				Command: "pwd",
			},
		},
		{
			name:    "freestyle work resolves repository",
			request: routingRequest(ShellToolName, `{"repo":"openrig","command":"pwd"}`),
			options: freestyleOptions,
			expectedInvocation: invocation.Invocation{
				CWD:       sourceRoot,
				Transport: "stdio",
			},
			expectedArguments: shellArguments{
				scopeArguments: scopeArguments{
					Repo: "openrig",
				},
				Command: "pwd",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := append(
				json.RawMessage(nil),
				test.request.Params.Arguments.(json.RawMessage)...,
			)
			actual, err := resolveCall(t.Context(), test.request, test.options)
			if err != nil {
				t.Fatalf("resolveCall returned error: %v", err)
			}
			if !ids.ValidPrefixed("tool_", actual.Invocation.ToolCallID) {
				t.Errorf(
					"tool call ID = %q, expected generated tool ULID",
					actual.Invocation.ToolCallID,
				)
			}
			actual.Invocation.ToolCallID = ""
			if diff := cmp.Diff(test.expectedInvocation, actual.Invocation); diff != "" {
				t.Errorf("mismatch invocation (-expected, +actual):\n%s", diff)
			}
			if diff := cmp.Diff(
				test.expectedArguments,
				actual.Arguments,
				argumentExporter(),
			); diff != "" {
				t.Errorf("mismatch arguments (-expected, +actual):\n%s", diff)
			}
			after := test.request.Params.Arguments.(json.RawMessage)
			if diff := cmp.Diff(before, after); diff != "" {
				t.Errorf("request arguments mutated (-before, +after):\n%s", diff)
			}
		})
	}
}

func TestResolveCallRoutesEveryScopedWorkOperation(t *testing.T) {
	root := t.TempDir()
	resolver, err := project.NewResolver(project.ResolverOptions{
		Source: project.SourceFunc(func() map[string]project.Definition {
			return map[string]project.Definition{
				"openrig": {Root: root},
			}
		}),
	})
	if err != nil {
		t.Fatalf("NewResolver returned error: %v", err)
	}
	root, err = resolver.ResolveRepo(root)
	if err != nil {
		t.Fatalf("ResolveRepo returned error: %v", err)
	}
	turnID := TurnID("turn_01ARZ3NDEKTSV4RRFFQ69G5FAV")
	worktreeID := WorktreeID("wt_01ARZ3NDEKTSV4RRFFQ69G5FAV")
	resources := &stubResourceResolver{
		activeTurns: map[TurnID]invocation.Scope{
			turnID: {
				TurnID:       string(turnID),
				WorktreeID:   string(worktreeID),
				WorkspaceCWD: root,
			},
		},
	}
	tests := []struct {
		name     string
		tool     string
		turnJSON string
		freeJSON string
	}{
		{
			name:     "shell",
			tool:     ShellToolName,
			turnJSON: `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","command":"pwd"}`,
			freeJSON: `{"repo":"openrig","command":"pwd"}`,
		},
		{
			name:     "apply patch",
			tool:     ApplyPatchToolName,
			turnJSON: `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","patch":"*** Begin Patch\n*** End Patch"}`,
			freeJSON: `{"repo":"openrig","patch":"*** Begin Patch\n*** End Patch"}`,
		},
		{
			name:     "process start",
			tool:     ProcessToolName,
			turnJSON: `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","op":"start","command":"go test ./..."}`,
			freeJSON: `{"repo":"openrig","op":"start","command":"go test ./..."}`,
		},
		{
			name:     "process status",
			tool:     ProcessToolName,
			turnJSON: `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","op":"status"}`,
			freeJSON: `{"repo":"openrig","op":"status"}`,
		},
		{
			name:     "process read",
			tool:     ProcessToolName,
			turnJSON: `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","op":"read","process_id":"proc_01"}`,
			freeJSON: `{"repo":"openrig","op":"read","process_id":"proc_01"}`,
		},
		{
			name:     "process stop",
			tool:     ProcessToolName,
			turnJSON: `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","op":"stop","process_id":"proc_01"}`,
			freeJSON: `{"repo":"openrig","op":"stop","process_id":"proc_01"}`,
		},
		{
			name:     "process restart",
			tool:     ProcessToolName,
			turnJSON: `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","op":"restart","process_id":"proc_01"}`,
			freeJSON: `{"repo":"openrig","op":"restart","process_id":"proc_01"}`,
		},
		{
			name:     "process kill",
			tool:     ProcessToolName,
			turnJSON: `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","op":"kill","process_id":"proc_01"}`,
			freeJSON: `{"repo":"openrig","op":"kill","process_id":"proc_01"}`,
		},
		{
			name:     "skill activate",
			tool:     SkillActivateToolName,
			turnJSON: `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","skill":"go"}`,
			freeJSON: `{"repo":"openrig","skill":"go"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name+"/turn", func(t *testing.T) {
			actual, err := resolveCall(
				t.Context(),
				routingRequest(test.tool, test.turnJSON),
				RoutingOptions{
					Projects:  resolver,
					Resources: resources,
				},
			)
			if err != nil {
				t.Fatalf("resolveCall returned error: %v", err)
			}
			expected := invocation.Owner{
				TurnID:     string(turnID),
				WorktreeID: string(worktreeID),
				CWD:        root,
			}
			if diff := cmp.Diff(expected, actual.Invocation.Owner()); diff != "" {
				t.Errorf("mismatch invocation owner (-expected, +actual):\n%s", diff)
			}
		})
		t.Run(test.name+"/freestyle", func(t *testing.T) {
			actual, err := resolveCall(
				t.Context(),
				routingRequest(test.tool, test.freeJSON),
				RoutingOptions{
					Scope:    FreestyleScope,
					Projects: resolver,
				},
			)
			if err != nil {
				t.Fatalf("resolveCall returned error: %v", err)
			}
			expected := invocation.Owner{CWD: root}
			if diff := cmp.Diff(expected, actual.Invocation.Owner()); diff != "" {
				t.Errorf("mismatch invocation owner (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestResolveCallPreservesClientCorrelationMetadata(t *testing.T) {
	tests := []struct {
		name               string
		meta               *mcp.Meta
		expectedToolCallID string
		expectedTraceID    string
		generatedToolCall  bool
	}{
		{
			name: "primary client identifiers",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"tool_call_id": "client-call",
					"trace_id":     "client-trace",
				},
			},
			expectedToolCallID: "client-call",
			expectedTraceID:    "client-trace",
		},
		{
			name: "alternate client identifiers",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"request_id": 42,
					"traceId":    "trace-42",
				},
			},
			expectedToolCallID: "42",
			expectedTraceID:    "trace-42",
		},
		{
			name: "numeric progress token",
			meta: &mcp.Meta{
				ProgressToken: json.Number("1e3"),
			},
			expectedToolCallID: "1000",
		},
		{
			name: "progress token fallback",
			meta: &mcp.Meta{
				ProgressToken: "progress-1",
			},
			expectedToolCallID: "progress-1",
		},
		{
			name: "W3C traceparent",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"call_id":     "client-call",
					"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
				},
			},
			expectedToolCallID: "client-call",
			expectedTraceID:    "4bf92f3577b34da6a3ce929d0e0e4736",
		},
		{
			name: "uppercase W3C traceparent",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"traceparent": "00-4BF92F3577B34DA6A3CE929D0E0E4736-00F067AA0BA902B7-01",
				},
			},
			expectedTraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
			generatedToolCall: true,
		},
		{
			name: "safe aliases follow invalid preferred metadata",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"tool_call_id": map[string]any{"invalid": true},
					"request_id":   "fallback-call",
					"trace_id":     []any{"invalid"},
					"traceparent":  "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
				},
			},
			expectedToolCallID: "fallback-call",
			expectedTraceID:    "4bf92f3577b34da6a3ce929d0e0e4736",
		},
		{
			name:              "generated tool call ID",
			generatedToolCall: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := routingRequest(HelpToolName, `{}`)
			request.Params.Meta = test.meta
			actual, err := resolveCall(t.Context(), request, RoutingOptions{})
			if err != nil {
				t.Fatalf("resolveCall returned error: %v", err)
			}
			if test.generatedToolCall {
				if !ids.ValidPrefixed("tool_", actual.Invocation.ToolCallID) {
					t.Errorf(
						"tool call ID = %q, expected generated tool ULID",
						actual.Invocation.ToolCallID,
					)
				}
			} else if diff := cmp.Diff(
				test.expectedToolCallID,
				actual.Invocation.ToolCallID,
			); diff != "" {
				t.Errorf("mismatch tool call ID (-expected, +actual):\n%s", diff)
			}
			if diff := cmp.Diff(test.expectedTraceID, actual.Invocation.TraceID); diff != "" {
				t.Errorf("mismatch trace ID (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestResolveCallIgnoresUnsafeClientCorrelationMetadata(t *testing.T) {
	tests := []struct {
		name          string
		meta          *mcp.Meta
		expectedTrace string
	}{
		{
			name: "object tool call ID",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"tool_call_id": map[string]any{"secret": "value"},
				},
			},
		},
		{
			name: "array progress token",
			meta: &mcp.Meta{
				ProgressToken: []any{"progress"},
			},
		},
		{
			name: "nonfinite progress token",
			meta: &mcp.Meta{
				ProgressToken: math.Inf(1),
			},
		},
		{
			name: "boolean request ID",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"request_id": true,
				},
			},
		},
		{
			name: "tool call ID with newline",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"tool_call_id": "call\nforged",
				},
			},
		},
		{
			name: "right-to-left override",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"tool_call_id": "call-\u202etxt",
					"trace_id":     "trace-\u202etxt",
				},
			},
		},
		{
			name: "left-to-right isolate",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"tool_call_id": "call-\u2066txt",
					"trace_id":     "trace-\u2066txt",
				},
			},
		},
		{
			name: "zero-width joiner",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"tool_call_id": "call-\u200dtxt",
					"trace_id":     "trace-\u200dtxt",
				},
			},
		},
		{
			name: "non-ASCII identifier",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"tool_call_id": "call-\u00e9",
					"trace_id":     "trace-\u00e9",
				},
			},
		},
		{
			name: "Unicode line separator",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"tool_call_id": "call-\u2028forged",
					"trace_id":     "trace-\u2028forged",
				},
			},
		},
		{
			name: "Unicode paragraph separator",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"tool_call_id": "call-\u2029forged",
					"trace_id":     "trace-\u2029forged",
				},
			},
		},
		{
			name: "overlong tool call ID",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"tool_call_id": strings.Repeat(
						"x",
						maxCorrelationMetadataBytes+1,
					),
				},
			},
		},
		{
			name: "trace ID with control character",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"trace_id": "trace\x00id",
				},
			},
		},
		{
			name: "overlong trace ID",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"trace_id": strings.Repeat(
						"x",
						maxCorrelationMetadataBytes+1,
					),
				},
			},
		},
		{
			name: "malformed traceparent",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"traceparent": "not-a-valid-traceparent",
				},
			},
		},
		{
			name: "all-zero W3C trace ID",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"traceparent": "00-00000000000000000000000000000000-00f067aa0ba902b7-01",
				},
			},
		},
		{
			name: "all-zero W3C parent ID",
			meta: &mcp.Meta{
				AdditionalFields: map[string]any{
					"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := routingRequest(HelpToolName, `{}`)
			request.Params.Meta = test.meta
			actual, err := resolveCall(t.Context(), request, RoutingOptions{})
			if err != nil {
				t.Fatalf("resolveCall returned error: %v", err)
			}
			if !ids.ValidPrefixed("tool_", actual.Invocation.ToolCallID) {
				t.Errorf(
					"tool call ID = %q, expected generated tool ULID",
					actual.Invocation.ToolCallID,
				)
			}
			if diff := cmp.Diff(test.expectedTrace, actual.Invocation.TraceID); diff != "" {
				t.Errorf("mismatch trace ID (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestResolveCallAcceptsMaximumCorrelationMetadataLength(t *testing.T) {
	expected := strings.Repeat("x", maxCorrelationMetadataBytes)
	request := routingRequest(HelpToolName, `{}`)
	request.Params.Meta = &mcp.Meta{
		AdditionalFields: map[string]any{
			"tool_call_id": expected,
			"trace_id":     expected,
		},
	}
	actual, err := resolveCall(t.Context(), request, RoutingOptions{})
	if err != nil {
		t.Fatalf("resolveCall returned error: %v", err)
	}
	if diff := cmp.Diff(expected, actual.Invocation.ToolCallID); diff != "" {
		t.Errorf("mismatch tool call ID (-expected, +actual):\n%s", diff)
	}
	if diff := cmp.Diff(expected, actual.Invocation.TraceID); diff != "" {
		t.Errorf("mismatch trace ID (-expected, +actual):\n%s", diff)
	}
}

func TestResolveCallRoutesRevisionOwnersExactly(t *testing.T) {
	root := t.TempDir()
	resolver, err := project.NewResolver(project.ResolverOptions{})
	if err != nil {
		t.Fatalf("NewResolver returned error: %v", err)
	}
	root, err = resolver.ResolveRepo(root)
	if err != nil {
		t.Fatalf("ResolveRepo returned error: %v", err)
	}
	turnID := "turn_01ARZ3NDEKTSV4RRFFQ69G5FAV"
	worktreeID := "wt_01ARZ3NDEKTSV4RRFFQ69G5FAV"
	otherTurnID := "turn_01ARZ3NDEKTSV4RRFFQ69G5FAW"
	otherWorktreeID := "wt_01ARZ3NDEKTSV4RRFFQ69G5FAW"

	tests := []struct {
		name          string
		from          invocation.Scope
		to            invocation.Scope
		expectedOwner invocation.Owner
		expectedCode  result.Code
	}{
		{
			name: "same turn and worktree",
			from: invocation.Scope{
				TurnID:       turnID,
				WorktreeID:   worktreeID,
				WorkspaceCWD: root,
			},
			to: invocation.Scope{
				TurnID:       turnID,
				WorktreeID:   worktreeID,
				WorkspaceCWD: root,
			},
			expectedOwner: invocation.Owner{
				TurnID:     turnID,
				WorktreeID: worktreeID,
				CWD:        root,
			},
		},
		{
			name: "different turns in same worktree",
			from: invocation.Scope{
				TurnID:       turnID,
				WorktreeID:   worktreeID,
				WorkspaceCWD: root,
			},
			to: invocation.Scope{
				TurnID:       otherTurnID,
				WorktreeID:   worktreeID,
				WorkspaceCWD: root,
			},
			expectedCode: result.CodeInvalidArgument,
		},
		{
			name: "same turn with different worktrees",
			from: invocation.Scope{
				TurnID:       turnID,
				WorktreeID:   worktreeID,
				WorkspaceCWD: root,
			},
			to: invocation.Scope{
				TurnID:       turnID,
				WorktreeID:   otherWorktreeID,
				WorkspaceCWD: root,
			},
			expectedCode: result.CodeInvalidArgument,
		},
		{
			name: "freestyle revisions",
			from: invocation.Scope{
				WorkspaceCWD: root,
			},
			to: invocation.Scope{
				WorkspaceCWD: root,
			},
			expectedOwner: invocation.Owner{
				CWD: root,
			},
		},
		{
			name: "turn and worktree owners",
			from: invocation.Scope{
				TurnID:       turnID,
				WorktreeID:   worktreeID,
				WorkspaceCWD: root,
			},
			to: invocation.Scope{
				WorktreeID:   worktreeID,
				WorkspaceCWD: root,
			},
			expectedCode: result.CodeInvalidArgument,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resources := &stubResourceResolver{
				revisions: map[RevisionID]invocation.Scope{
					RevisionID("rev_from"): test.from,
					RevisionID("rev_to"):   test.to,
				},
			}
			actual, err := resolveCall(
				t.Context(),
				routingRequest(
					DiffToolName,
					`{"kind":"revision","from":"rev_from","to":"rev_to"}`,
				),
				RoutingOptions{
					Projects:  resolver,
					Resources: resources,
				},
			)
			if test.expectedCode != "" {
				if err == nil {
					t.Fatal("resolveCall error = nil, expected error")
				}
				if actual.Arguments != nil {
					t.Errorf(
						"resolveCall arguments = %#v, expected nil on error",
						actual.Arguments,
					)
				}
				structured := result.ErrorOf(err)
				if diff := cmp.Diff(test.expectedCode, structured.Code); diff != "" {
					t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
				}
				expectedField := "to"
				if diff := cmp.Diff(expectedField, structured.Field); diff != "" {
					t.Errorf("mismatch error field (-expected, +actual):\n%s", diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveCall returned error: %v", err)
			}
			if diff := cmp.Diff(test.expectedOwner, actual.Invocation.Owner()); diff != "" {
				t.Errorf("mismatch invocation owner (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestResolveCallKeepsSessionAsCorrelationOnly(t *testing.T) {
	mcpServer := server.NewMCPServer("test", "1.0")
	ctx := mcpServer.WithContext(
		t.Context(),
		&routingTestSession{id: " session-1 "},
	)
	actual, err := resolveCall(
		ctx,
		routingRequest(HelpToolName, `{}`),
		RoutingOptions{},
	)
	if err != nil {
		t.Fatalf("resolveCall returned error: %v", err)
	}
	expected := "session-1"
	if diff := cmp.Diff(expected, actual.Invocation.SessionID); diff != "" {
		t.Errorf("mismatch session ID (-expected, +actual):\n%s", diff)
	}
	if actual.Invocation.Owner().Valid() {
		t.Errorf(
			"session-only invocation produced valid owner: %#v",
			actual.Invocation.Owner(),
		)
	}
}

func TestResolveCallRejectsInvalidOrAmbiguousRoutes(t *testing.T) {
	root := t.TempDir()
	otherRoot := t.TempDir()
	resolver, err := project.NewResolver(project.ResolverOptions{})
	if err != nil {
		t.Fatalf("NewResolver returned error: %v", err)
	}
	root, err = resolver.ResolveRepo(root)
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	otherRoot, err = resolver.ResolveRepo(otherRoot)
	if err != nil {
		t.Fatalf("resolve other root: %v", err)
	}
	turnID := TurnID("turn_01ARZ3NDEKTSV4RRFFQ69G5FAV")
	worktreeID := WorktreeID("wt_01ARZ3NDEKTSV4RRFFQ69G5FAV")
	baseResources := &stubResourceResolver{
		activeTurns: map[TurnID]invocation.Scope{
			turnID: {
				TurnID:       string(turnID),
				WorktreeID:   string(worktreeID),
				WorkspaceCWD: root,
			},
		},
		turns: map[TurnID]invocation.Scope{
			turnID: {
				TurnID:       string(turnID),
				WorktreeID:   string(worktreeID),
				WorkspaceCWD: root,
			},
		},
		worktrees: map[WorktreeID]invocation.Scope{
			worktreeID: {
				WorktreeID:   string(worktreeID),
				WorkspaceCWD: root,
			},
		},
		revisions: map[RevisionID]invocation.Scope{
			RevisionID("rev_01"): {
				WorktreeID:   string(worktreeID),
				WorkspaceCWD: root,
			},
			RevisionID("rev_02"): {
				WorktreeID:   "wt_other",
				WorkspaceCWD: otherRoot,
			},
		},
	}
	mismatchedTurn := *baseResources
	mismatchedTurn.activeTurns = map[TurnID]invocation.Scope{
		turnID: {
			TurnID:       "turn_other",
			WorktreeID:   string(worktreeID),
			WorkspaceCWD: root,
		},
	}
	mismatchedWorktree := *baseResources
	mismatchedWorktree.worktrees = map[WorktreeID]invocation.Scope{
		worktreeID: {
			WorktreeID:   "wt_other",
			WorkspaceCWD: root,
		},
	}

	tests := []struct {
		name          string
		request       mcp.CallToolRequest
		options       RoutingOptions
		expectedCode  result.Code
		expectedField string
	}{
		{
			name:          "invalid arguments",
			request:       routingRequest(ShellToolName, `{"turn_id":"turn_01","command":1}`),
			options:       RoutingOptions{},
			expectedCode:  result.CodeInvalidArgument,
			expectedField: "command",
		},
		{
			name:         "missing project resolver",
			request:      routingRequest(WorktreeToolName, `{"op":"open","repo":"openrig"}`),
			options:      RoutingOptions{},
			expectedCode: result.CodeInternal,
		},
		{
			name:    "missing resource resolver",
			request: routingRequest(ShellToolName, `{"turn_id":"turn_01","command":"pwd"}`),
			options: RoutingOptions{
				Projects: resolver,
			},
			expectedCode: result.CodeInternal,
		},
		{
			name:    "mismatched turn scope",
			request: routingRequest(ShellToolName, `{"turn_id":"turn_01ARZ3NDEKTSV4RRFFQ69G5FAV","command":"pwd"}`),
			options: RoutingOptions{
				Projects:  resolver,
				Resources: &mismatchedTurn,
			},
			expectedCode: result.CodeInternal,
		},
		{
			name:    "mismatched worktree scope",
			request: routingRequest(WorktreeToolName, `{"op":"status","worktree_id":"wt_01ARZ3NDEKTSV4RRFFQ69G5FAV"}`),
			options: RoutingOptions{
				Projects:  resolver,
				Resources: &mismatchedWorktree,
			},
			expectedCode: result.CodeInternal,
		},
		{
			name:    "revision endpoints in different workspaces",
			request: routingRequest(DiffToolName, `{"kind":"revision","from":"rev_01","to":"rev_02"}`),
			options: RoutingOptions{
				Projects:  resolver,
				Resources: baseResources,
			},
			expectedCode:  result.CodeInvalidArgument,
			expectedField: "to",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := resolveCall(t.Context(), test.request, test.options)
			if err == nil {
				t.Fatal("resolveCall error = nil, expected error")
			}
			if actual.Arguments != nil {
				t.Errorf(
					"resolveCall arguments = %#v, expected nil on error",
					actual.Arguments,
				)
			}
			structured := result.ErrorOf(err)
			if diff := cmp.Diff(test.expectedCode, structured.Code); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
			if diff := cmp.Diff(test.expectedField, structured.Field); diff != "" {
				t.Errorf("mismatch error field (-expected, +actual):\n%s", diff)
			}
			if !ids.ValidPrefixed("tool_", actual.Invocation.ToolCallID) {
				t.Errorf(
					"tool call ID = %q, expected generated tool ULID",
					actual.Invocation.ToolCallID,
				)
			}
		})
	}
}

func routingRequest(name, arguments string) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: json.RawMessage(arguments),
		},
	}
}

type stubResourceResolver struct {
	activeTurns map[TurnID]invocation.Scope
	turns       map[TurnID]invocation.Scope
	worktrees   map[WorktreeID]invocation.Scope
	revisions   map[RevisionID]invocation.Scope
}

func (s *stubResourceResolver) ResolveActiveTurn(
	_ context.Context,
	turnID TurnID,
) (invocation.Scope, error) {
	scope, ok := s.activeTurns[turnID]
	if !ok {
		return invocation.Scope{}, result.NewError(
			result.CodeFailedPrecondition,
			"turn is not active",
		)
	}
	return scope, nil
}

func (s *stubResourceResolver) ResolveTurn(
	_ context.Context,
	turnID TurnID,
) (invocation.Scope, error) {
	scope, ok := s.turns[turnID]
	if !ok {
		return invocation.Scope{}, result.NewError(
			result.CodeNotFound,
			"turn was not found",
		)
	}
	return scope, nil
}

func (s *stubResourceResolver) ResolveWorktree(
	_ context.Context,
	worktreeID WorktreeID,
) (invocation.Scope, error) {
	scope, ok := s.worktrees[worktreeID]
	if !ok {
		return invocation.Scope{}, result.NewError(
			result.CodeNotFound,
			"worktree was not found",
		)
	}
	return scope, nil
}

func (s *stubResourceResolver) ResolveRevision(
	_ context.Context,
	revisionID RevisionID,
) (invocation.Scope, error) {
	scope, ok := s.revisions[revisionID]
	if !ok {
		return invocation.Scope{}, result.NewError(
			result.CodeNotFound,
			"revision was not found",
		)
	}
	return scope, nil
}

type routingTestSession struct {
	id string
}

func (*routingTestSession) Initialize() {}

func (*routingTestSession) Initialized() bool {
	return true
}

func (*routingTestSession) NotificationChannel() chan<- mcp.JSONRPCNotification {
	return nil
}

func (s *routingTestSession) SessionID() string {
	return s.id
}

var _ ResourceResolver = (*stubResourceResolver)(nil)
var _ server.ClientSession = (*routingTestSession)(nil)
