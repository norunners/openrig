package tools

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/norunners/openrig/internal/result"
)

func TestDecodeArgumentsReturnsOperationSpecificValues(t *testing.T) {
	tests := []struct {
		name      string
		tool      string
		scope     Scope
		arguments json.RawMessage
		expected  toolArguments
	}{
		{
			name:      "help default",
			tool:      HelpToolName,
			arguments: json.RawMessage(`{}`),
			expected:  helpArguments{},
		},
		{
			name:      "help topic",
			tool:      HelpToolName,
			arguments: json.RawMessage(`{"topic":"common"}`),
			expected: helpArguments{
				Topic: "common",
			},
		},
		{
			name:      "status",
			tool:      StatusToolName,
			arguments: json.RawMessage(`{}`),
			expected:  statusArguments{},
		},
		{
			name:      "worktree open defaults base",
			tool:      WorktreeToolName,
			arguments: json.RawMessage(`{"op":"open","repo":"openrig"}`),
			expected: worktreeOpenArguments{
				Op:   WorktreeOpOpen,
				Repo: "openrig",
				Base: "HEAD",
			},
		},
		{
			name:      "worktree open",
			tool:      WorktreeToolName,
			arguments: json.RawMessage(`{"op":"open","repo":"openrig","base":"main","branch":"review","reason":"code review"}`),
			expected: worktreeOpenArguments{
				Op:     WorktreeOpOpen,
				Repo:   "openrig",
				Base:   "main",
				Branch: "review",
				Reason: "code review",
			},
		},
		{
			name:      "worktree list",
			tool:      WorktreeToolName,
			arguments: json.RawMessage(`{"op":"list","repo":"openrig","state":"open"}`),
			expected: worktreeListArguments{
				Op:    WorktreeOpList,
				Repo:  "openrig",
				State: WorktreeStateFilterOpen,
			},
		},
		{
			name:      "worktree status",
			tool:      WorktreeToolName,
			arguments: json.RawMessage(`{"op":"status","worktree_id":"wt_01"}`),
			expected: worktreeStatusArguments{
				Op:         WorktreeOpStatus,
				WorktreeID: WorktreeID("wt_01"),
			},
		},
		{
			name:      "worktree close",
			tool:      WorktreeToolName,
			arguments: json.RawMessage(`{"op":"close","worktree_id":"wt_01"}`),
			expected: worktreeCloseArguments{
				Op:         WorktreeOpClose,
				WorktreeID: WorktreeID("wt_01"),
			},
		},
		{
			name:      "worktree delete",
			tool:      WorktreeToolName,
			arguments: json.RawMessage(`{"op":"delete","worktree_id":"wt_01"}`),
			expected: worktreeDeleteArguments{
				Op:         WorktreeOpDelete,
				WorktreeID: WorktreeID("wt_01"),
			},
		},
		{
			name:      "turn begin creates worktree and defaults mode",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"begin","repo":"openrig","goal":"Fix the parser"}`),
			expected: turnBeginArguments{
				Op:   TurnOpBegin,
				Mode: TurnModeWorktree,
				Repo: "openrig",
				Goal: "Fix the parser",
			},
		},
		{
			name:      "turn begin reuses worktree",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"begin","mode":"worktree","worktree_id":"wt_01","goal":"Review the contract"}`),
			expected: turnBeginArguments{
				Op:         TurnOpBegin,
				Mode:       TurnModeWorktree,
				WorktreeID: WorktreeID("wt_01"),
				Goal:       "Review the contract",
			},
		},
		{
			name:      "turn begin freestyle",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"begin","mode":"freestyle","repo":"openrig","goal":"Prepare Git"}`),
			expected: turnBeginArguments{
				Op:   TurnOpBegin,
				Mode: TurnModeFreestyle,
				Repo: "openrig",
				Goal: "Prepare Git",
			},
		},
		{
			name:      "turn status exact",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"status","turn_id":"turn_01"}`),
			expected: turnStatusArguments{
				Op:     TurnOpStatus,
				TurnID: TurnID("turn_01"),
			},
		},
		{
			name:      "turn status filtered",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"status","worktree_id":"wt_01","mode":"worktree","state":"ended"}`),
			expected: turnStatusArguments{
				Op:         TurnOpStatus,
				Mode:       TurnModeWorktree,
				WorktreeID: WorktreeID("wt_01"),
				State:      TurnStateFilterEnded,
			},
		},
		{
			name:      "turn end",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"end","turn_id":"turn_01","outcome":"completed","summary":"Done"}`),
			expected: turnEndArguments{
				Op:      TurnOpEnd,
				TurnID:  TurnID("turn_01"),
				Outcome: TurnOutcomeCompleted,
				Summary: "Done",
			},
		},
		{
			name:      "diff turn",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","paths":["internal/tools"]}`),
			expected: diffTurnArguments{
				TurnID: TurnID("turn_01"),
				Paths:  []string{"internal/tools"},
			},
		},
		{
			name:      "diff worktree",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"kind":"worktree","worktree_id":"wt_01","from":"main","stat":true}`),
			expected: diffWorktreeArguments{
				Kind:       DiffKindWorktree,
				WorktreeID: WorktreeID("wt_01"),
				From:       "main",
				Stat:       true,
			},
		},
		{
			name:      "diff git",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"kind":"git","turn_id":"turn_01","from":"origin/main","to":"HEAD"}`),
			expected: diffGitArguments{
				Kind:   DiffKindGit,
				TurnID: TurnID("turn_01"),
				From:   "origin/main",
				To:     "HEAD",
			},
		},
		{
			name:      "diff revision with paths",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"kind":"revision","from":"rev_01","to":"rev_02","paths":["internal/tools"]}`),
			expected: diffRevisionArguments{
				Kind:  DiffKindRevision,
				From:  RevisionID("rev_01"),
				To:    RevisionID("rev_02"),
				Paths: []string{"internal/tools"},
			},
		},
		{
			name:      "shell turn",
			tool:      ShellToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","command":"go test ./...","workdir":"internal/tools"}`),
			expected: shellArguments{
				scopeArguments: scopeArguments{
					TurnID: TurnID("turn_01"),
				},
				Command: "go test ./...",
				Workdir: "internal/tools",
			},
		},
		{
			name:      "shell freestyle",
			tool:      ShellToolName,
			scope:     FreestyleScope,
			arguments: json.RawMessage(`{"repo":"openrig","command":"git status"}`),
			expected: shellArguments{
				scopeArguments: scopeArguments{
					Repo: "openrig",
				},
				Command: "git status",
			},
		},
		{
			name:      "apply patch turn",
			tool:      ApplyPatchToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","patch":"*** Begin Patch\n*** End Patch\n"}`),
			expected: applyPatchArguments{
				scopeArguments: scopeArguments{
					TurnID: TurnID("turn_01"),
				},
				Patch: "*** Begin Patch\n*** End Patch\n",
			},
		},
		{
			name:      "apply patch freestyle",
			tool:      ApplyPatchToolName,
			scope:     FreestyleScope,
			arguments: json.RawMessage(`{"repo":"openrig","patch":"*** Begin Patch\n*** End Patch\n"}`),
			expected: applyPatchArguments{
				scopeArguments: scopeArguments{
					Repo: "openrig",
				},
				Patch: "*** Begin Patch\n*** End Patch\n",
			},
		},
		{
			name:      "process start",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"start","command":"go run ./cmd/openrig","workdir":"cmd/openrig","env":{"PORT":"8080"}}`),
			expected: processStartArguments{
				scopeArguments: scopeArguments{
					TurnID: TurnID("turn_01"),
				},
				Op:      ProcessOpStart,
				Command: "go run ./cmd/openrig",
				Workdir: "cmd/openrig",
				Env:     map[string]string{"PORT": "8080"},
			},
		},
		{
			name:      "process status",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"status","state":"running"}`),
			expected: processStatusArguments{
				scopeArguments: scopeArguments{
					TurnID: TurnID("turn_01"),
				},
				Op:    ProcessOpStatus,
				State: ProcessStateFilterRunning,
			},
		},
		{
			name:      "process read",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"read","process_id":"proc_01","cursor":""}`),
			expected: processReadArguments{
				scopeArguments: scopeArguments{
					TurnID: TurnID("turn_01"),
				},
				Op:        ProcessOpRead,
				ProcessID: ProcessID("proc_01"),
			},
		},
		{
			name:      "process stop",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"stop","process_id":"proc_01"}`),
			expected: processStopArguments{
				scopeArguments: scopeArguments{
					TurnID: TurnID("turn_01"),
				},
				Op:        ProcessOpStop,
				ProcessID: ProcessID("proc_01"),
			},
		},
		{
			name:      "process restart",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"restart","process_id":"proc_01"}`),
			expected: processRestartArguments{
				scopeArguments: scopeArguments{
					TurnID: TurnID("turn_01"),
				},
				Op:        ProcessOpRestart,
				ProcessID: ProcessID("proc_01"),
			},
		},
		{
			name:      "process kill freestyle",
			tool:      ProcessToolName,
			scope:     FreestyleScope,
			arguments: json.RawMessage(`{"repo":"openrig","op":"kill","process_id":"proc_01"}`),
			expected: processKillArguments{
				scopeArguments: scopeArguments{
					Repo: "openrig",
				},
				Op:        ProcessOpKill,
				ProcessID: ProcessID("proc_01"),
			},
		},
		{
			name:      "skill activate turn",
			tool:      SkillActivateToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","skill":"coding-guidelines-go","include_references":true}`),
			expected: skillActivateArguments{
				scopeArguments: scopeArguments{
					TurnID: TurnID("turn_01"),
				},
				Skill:             "coding-guidelines-go",
				IncludeReferences: true,
			},
		},
		{
			name:      "skill activate freestyle",
			tool:      SkillActivateToolName,
			scope:     FreestyleScope,
			arguments: json.RawMessage(`{"repo":"openrig","skill":"go"}`),
			expected: skillActivateArguments{
				scopeArguments: scopeArguments{
					Repo: "openrig",
				},
				Skill: "go",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := decodeArguments(test.tool, test.scope, test.arguments)
			if err != nil {
				t.Fatalf("decodeArguments returned error: %v", err)
			}
			if diff := cmp.Diff(test.expected, actual, argumentExporter()); diff != "" {
				t.Errorf("mismatch decoded arguments (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestDecodeArgumentsRejectsInvalidWireValues(t *testing.T) {
	tests := []struct {
		name      string
		tool      string
		scope     Scope
		arguments any
		field     string
	}{
		{
			name:      "unknown tool",
			tool:      "turn_begin",
			arguments: json.RawMessage(`{}`),
			field:     "tool",
		},
		{
			name:      "invalid scope",
			tool:      ShellToolName,
			scope:     Scope(99),
			arguments: json.RawMessage(`{}`),
			field:     "scope",
		},
		{
			name:      "malformed JSON",
			tool:      HelpToolName,
			arguments: json.RawMessage(`{"topic":`),
		},
		{
			name:      "multiple JSON values",
			tool:      HelpToolName,
			arguments: json.RawMessage(`{} {}`),
		},
		{
			name:      "array arguments",
			tool:      HelpToolName,
			arguments: json.RawMessage(`[]`),
		},
		{
			name:      "null arguments",
			tool:      HelpToolName,
			arguments: json.RawMessage(`null`),
		},
		{
			name:      "unsupported Go value",
			tool:      HelpToolName,
			arguments: make(chan int),
		},
		{
			name:      "unknown field",
			tool:      HelpToolName,
			arguments: json.RawMessage(`{"tools":true}`),
			field:     "tools",
		},
		{
			name:      "wrong primitive type",
			tool:      HelpToolName,
			arguments: json.RawMessage(`{"topic":true}`),
			field:     "topic",
		},
		{
			name:      "status repository selector",
			tool:      StatusToolName,
			arguments: json.RawMessage(`{"repo":"openrig"}`),
			field:     "repo",
		},
		{
			name:      "explicit null",
			tool:      HelpToolName,
			arguments: json.RawMessage(`{"topic":null}`),
			field:     "topic",
		},
		{
			name:      "worktree missing op",
			tool:      WorktreeToolName,
			arguments: json.RawMessage(`{}`),
			field:     "op",
		},
		{
			name:      "worktree unknown op",
			tool:      WorktreeToolName,
			arguments: json.RawMessage(`{"op":"archive"}`),
			field:     "op",
		},
		{
			name:      "worktree open missing repository",
			tool:      WorktreeToolName,
			arguments: json.RawMessage(`{"op":"open"}`),
			field:     "repo",
		},
		{
			name:      "worktree list open-only field",
			tool:      WorktreeToolName,
			arguments: json.RawMessage(`{"op":"list","branch":"main"}`),
			field:     "branch",
		},
		{
			name:      "worktree status missing ID",
			tool:      WorktreeToolName,
			arguments: json.RawMessage(`{"op":"status"}`),
			field:     "worktree_id",
		},
		{
			name:      "turn begin selector conflict",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"begin","repo":"openrig","worktree_id":"wt_01","goal":"Fix"}`),
			field:     "worktree_id",
		},
		{
			name:      "turn begin missing goal",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"begin","repo":"openrig"}`),
			field:     "goal",
		},
		{
			name:      "turn begin blank goal",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"begin","repo":"openrig","goal":"  "}`),
			field:     "goal",
		},
		{
			name:      "turn begin freestyle worktree",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"begin","mode":"freestyle","repo":"openrig","worktree_id":"wt_01","goal":"Fix"}`),
			field:     "worktree_id",
		},
		{
			name:      "turn exact status filter",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"status","turn_id":"turn_01","state":"ended"}`),
			field:     "state",
		},
		{
			name:      "turn end missing outcome",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"end","turn_id":"turn_01"}`),
			field:     "outcome",
		},
		{
			name:      "turn removed kind",
			tool:      TurnToolName,
			arguments: json.RawMessage(`{"op":"begin","repo":"openrig","goal":"Fix","kind":"review"}`),
			field:     "kind",
		},
		{
			name:      "diff common path missing turn",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{}`),
			field:     "turn_id",
		},
		{
			name:      "diff worktree missing ID",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"kind":"worktree"}`),
			field:     "worktree_id",
		},
		{
			name:      "diff git selector conflict",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"kind":"git","turn_id":"turn_01","worktree_id":"wt_01"}`),
			field:     "turn_id",
		},
		{
			name:      "diff git to without from",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"kind":"git","turn_id":"turn_01","to":"HEAD"}`),
			field:     "from",
		},
		{
			name:      "diff revision missing to",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"kind":"revision","from":"rev_01"}`),
			field:     "to",
		},
		{
			name:      "diff response-only turn kind",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"kind":"turn","turn_id":"turn_01"}`),
			field:     "kind",
		},
		{
			name:      "diff duplicate path",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","paths":["a","a"]}`),
			field:     "paths",
		},
		{
			name:      "diff absolute path",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","paths":["/etc/passwd"]}`),
			field:     "paths",
		},
		{
			name:      "diff parent-traversing path",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","paths":["../outside"]}`),
			field:     "paths",
		},
		{
			name:      "diff NUL path",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","paths":["bad\u0000path"]}`),
			field:     "paths",
		},
		{
			name:      "diff worktree absolute path",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"kind":"worktree","worktree_id":"wt_01","paths":["/etc/passwd"]}`),
			field:     "paths",
		},
		{
			name:      "diff Git parent-traversing path",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"kind":"git","turn_id":"turn_01","paths":["../outside"]}`),
			field:     "paths",
		},
		{
			name:      "diff revision NUL path",
			tool:      DiffToolName,
			arguments: json.RawMessage(`{"kind":"revision","from":"rev_01","to":"rev_02","paths":["bad\u0000path"]}`),
			field:     "paths",
		},
		{
			name:      "shell missing turn",
			tool:      ShellToolName,
			arguments: json.RawMessage(`{"command":"pwd"}`),
			field:     "turn_id",
		},
		{
			name:      "shell wrong turn ID type",
			tool:      ShellToolName,
			arguments: json.RawMessage(`{"turn_id":1,"command":"pwd"}`),
			field:     "turn_id",
		},
		{
			name:      "shell wrong repository type",
			tool:      ShellToolName,
			scope:     FreestyleScope,
			arguments: json.RawMessage(`{"repo":1,"command":"pwd"}`),
			field:     "repo",
		},
		{
			name:      "shell wrong scope field",
			tool:      ShellToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","repo":"openrig","command":"pwd"}`),
			field:     "repo",
		},
		{
			name:      "shell empty workdir",
			tool:      ShellToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","command":"pwd","workdir":"  "}`),
			field:     "workdir",
		},
		{
			name:      "shell absolute workdir",
			tool:      ShellToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","command":"pwd","workdir":"/tmp"}`),
			field:     "workdir",
		},
		{
			name:      "shell parent-traversing workdir",
			tool:      ShellToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","command":"pwd","workdir":"../outside"}`),
			field:     "workdir",
		},
		{
			name:      "shell NUL command",
			tool:      ShellToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","command":"echo\u0000unsafe"}`),
			field:     "command",
		},
		{
			name:      "shell NUL workdir",
			tool:      ShellToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","command":"pwd","workdir":"bad\u0000dir"}`),
			field:     "workdir",
		},
		{
			name:      "shell removed timeout",
			tool:      ShellToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","command":"pwd","timeout_ms":1}`),
			field:     "timeout_ms",
		},
		{
			name:      "patch missing patch",
			tool:      ApplyPatchToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01"}`),
			field:     "patch",
		},
		{
			name:      "process missing op",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01"}`),
			field:     "op",
		},
		{
			name:      "process start missing command",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"start"}`),
			field:     "command",
		},
		{
			name:      "process start NUL command",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"start","command":"echo\u0000unsafe"}`),
			field:     "command",
		},
		{
			name:      "process start parent-traversing workdir",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"start","command":"pwd","workdir":"../outside"}`),
			field:     "workdir",
		},
		{
			name:      "process start wrong env value",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"start","command":"pwd","env":{"PORT":8080}}`),
			field:     "env",
		},
		{
			name:      "process start invalid env name",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"start","command":"pwd","env":{"BAD=NAME":"value"}}`),
			field:     "env",
		},
		{
			name:      "process start NUL in env name",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"start","command":"pwd","env":{"BAD\u0000NAME":"value"}}`),
			field:     "env",
		},
		{
			name:      "process start NUL in env value",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"start","command":"pwd","env":{"GOOD":"bad\u0000value"}}`),
			field:     "env",
		},
		{
			name:      "process removed name",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"start","command":"pwd","name":"server"}`),
			field:     "name",
		},
		{
			name:      "process exact status filter",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"status","process_id":"proc_01","state":"running"}`),
			field:     "state",
		},
		{
			name:      "process read missing ID",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"read"}`),
			field:     "process_id",
		},
		{
			name:      "process stop read-only field",
			tool:      ProcessToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","op":"stop","process_id":"proc_01","cursor":"0"}`),
			field:     "cursor",
		},
		{
			name:      "skill invalid name",
			tool:      SkillActivateToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","skill":"../go"}`),
			field:     "skill",
		},
		{
			name:      "skill wrong boolean",
			tool:      SkillActivateToolName,
			arguments: json.RawMessage(`{"turn_id":"turn_01","skill":"go","include_scripts":"true"}`),
			field:     "include_scripts",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualArguments, err := decodeArguments(test.tool, test.scope, test.arguments)
			if err == nil {
				t.Fatal("decodeArguments error = nil, expected error")
			}
			if actualArguments != nil {
				t.Errorf("decodeArguments result = %#v, expected nil", actualArguments)
			}
			var actual *result.Error
			if !errors.As(err, &actual) {
				t.Fatalf("decodeArguments error type = %T, expected *result.Error", err)
			}
			expected := struct {
				Code  result.Code
				Field string
			}{
				Code:  result.CodeInvalidArgument,
				Field: test.field,
			}
			actualState := struct {
				Code  result.Code
				Field string
			}{
				Code:  actual.Code,
				Field: actual.Field,
			}
			if diff := cmp.Diff(expected, actualState); diff != "" {
				t.Errorf("mismatch decode error (-expected, +actual):\n%s\nerror: %v", diff, err)
			}
		})
	}
}

func TestDecodeArgumentsAcceptsMCPMapBoundary(t *testing.T) {
	expected := shellArguments{
		scopeArguments: scopeArguments{
			TurnID: TurnID("turn_01"),
		},
		Command: "go test ./...",
	}
	actual, err := decodeArguments(ShellToolName, TurnScope, map[string]any{
		"turn_id": "turn_01",
		"command": "go test ./...",
	})
	if err != nil {
		t.Fatalf("decodeArguments returned error: %v", err)
	}
	if diff := cmp.Diff(toolArguments(expected), actual, argumentExporter()); diff != "" {
		t.Errorf("mismatch decoded map arguments (-expected, +actual):\n%s", diff)
	}
}

func TestRuntimeArgumentFieldsMatchContractUnions(t *testing.T) {
	argumentTypes := map[string][]reflect.Type{
		HelpToolName: {
			reflect.TypeFor[helpArguments](),
		},
		StatusToolName: {
			reflect.TypeFor[statusArguments](),
		},
		WorktreeToolName: {
			reflect.TypeFor[worktreeOpenArguments](),
			reflect.TypeFor[worktreeListArguments](),
			reflect.TypeFor[worktreeStatusArguments](),
			reflect.TypeFor[worktreeCloseArguments](),
			reflect.TypeFor[worktreeDeleteArguments](),
		},
		TurnToolName: {
			reflect.TypeFor[turnBeginArguments](),
			reflect.TypeFor[turnStatusArguments](),
			reflect.TypeFor[turnEndArguments](),
		},
		DiffToolName: {
			reflect.TypeFor[diffTurnArguments](),
			reflect.TypeFor[diffWorktreeArguments](),
			reflect.TypeFor[diffGitArguments](),
			reflect.TypeFor[diffRevisionArguments](),
		},
		ShellToolName: {
			reflect.TypeFor[shellArguments](),
		},
		ApplyPatchToolName: {
			reflect.TypeFor[applyPatchArguments](),
		},
		ProcessToolName: {
			reflect.TypeFor[processStartArguments](),
			reflect.TypeFor[processStatusArguments](),
			reflect.TypeFor[processReadArguments](),
			reflect.TypeFor[processStopArguments](),
			reflect.TypeFor[processRestartArguments](),
			reflect.TypeFor[processKillArguments](),
		},
		SkillActivateToolName: {
			reflect.TypeFor[skillActivateArguments](),
		},
	}

	for _, toolName := range ContractNames() {
		t.Run(toolName, func(t *testing.T) {
			expected := contractFieldUnion(t, toolName)
			actualSet := map[string]bool{}
			for _, argumentType := range argumentTypes[toolName] {
				for field := range jsonFields(argumentType) {
					actualSet[field] = true
				}
			}
			actual := sortedFields(actualSet)
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("mismatch runtime argument fields (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func contractFieldUnion(t *testing.T, toolName string) []string {
	t.Helper()
	fields := map[string]bool{}
	for _, scope := range []Scope{TurnScope, FreestyleScope} {
		contract, err := Contract(toolName, scope)
		if err != nil {
			t.Fatalf("Contract returned error: %v", err)
		}
		for field := range contract.InputSchema.Properties {
			fields[field] = true
		}
	}
	return sortedFields(fields)
}

func sortedFields(fields map[string]bool) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func argumentExporter() cmp.Option {
	return cmp.Exporter(func(value reflect.Type) bool {
		return value.PkgPath() == "github.com/norunners/openrig/internal/tools"
	})
}
