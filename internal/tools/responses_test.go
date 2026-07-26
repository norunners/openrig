package tools

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/norunners/openrig/internal/result"
)

func TestResponseDTOsDefineExactTopLevelFields(t *testing.T) {
	tests := []struct {
		name     string
		response any
		expected []string
	}{
		{
			name:     HelpToolName,
			response: helpResponse{},
			expected: []string{"common_flow", "details", "examples", "summary", "topic"},
		},
		{
			name:     StatusToolName,
			response: statusResponse{},
			expected: []string{"active_turn", "ready"},
		},
		{
			name:     WorktreeToolName + "/exact",
			response: worktreeResponse{},
			expected: []string{"worktree"},
		},
		{
			name:     WorktreeToolName + "/list",
			response: worktreeListResponse{},
			expected: []string{"worktrees"},
		},
		{
			name:     TurnToolName + "/exact",
			response: turnResponse{},
			expected: []string{"turn"},
		},
		{
			name:     TurnToolName + "/list",
			response: turnListResponse{},
			expected: []string{"turns"},
		},
		{
			name:     "recovery",
			response: nextActionResponse{},
			expected: []string{"next"},
		},
		{
			name:     DiffToolName,
			response: diffResponse{},
			expected: []string{"files", "from", "kind", "patch", "paths", "summary", "to", "turn_id", "worktree_id"},
		},
		{
			name:     ShellToolName,
			response: shellResponse{},
			expected: []string{"cwd", "duration_ms", "exit_code", "stderr", "stdout", "timed_out"},
		},
		{
			name:     ApplyPatchToolName,
			response: applyPatchResponse{},
			expected: []string{"cwd", "files"},
		},
		{
			name:     ProcessToolName + "/exact",
			response: processResponse{},
			expected: []string{"process"},
		},
		{
			name:     ProcessToolName + "/list",
			response: processListResponse{},
			expected: []string{"processes"},
		},
		{
			name:     ProcessToolName + "/read",
			response: processReadResponse{},
			expected: []string{"output"},
		},
		{
			name:     ProcessToolName + "/restart",
			response: processRestartResponse{},
			expected: []string{"previous", "process"},
		},
		{
			name:     SkillActivateToolName,
			response: skillActivateResponse{},
			expected: []string{"description", "instructions", "name", "origin", "references", "references_truncated", "scripts", "scripts_truncated"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := jsonFieldNames(reflect.TypeOf(test.response))
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("mismatch response fields (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestWorktreeResponseUsesBaseVocabulary(t *testing.T) {
	response := worktreeResponse{
		Worktree: worktree{
			WorktreeID:   "wt_01",
			State:        WorktreeStateOpen,
			SourceRoot:   "/repo",
			WorktreeRoot: "/worktree",
			Base:         "main",
			BaseSHA:      "abc123",
			CreatedAt:    "2026-07-25T00:00:00Z",
			UpdatedAt:    "2026-07-25T00:01:00Z",
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal worktree response: %v", err)
	}
	expected := `{"worktree":{"worktree_id":"wt_01","state":"open","source_root":"/repo","worktree_root":"/worktree","base":"main","base_sha":"abc123","created_at":"2026-07-25T00:00:00Z","updated_at":"2026-07-25T00:01:00Z"}}`
	actual := string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch worktree response JSON (-expected, +actual):\n%s", diff)
	}
}

func TestListResponsesEncodeEmptyCollections(t *testing.T) {
	tests := []struct {
		name     string
		response any
		expected string
	}{
		{
			name:     "worktrees zero value",
			response: worktreeListResponse{},
			expected: `{"worktrees":[]}`,
		},
		{
			name: "worktrees explicit nil",
			response: worktreeListResponse{
				Worktrees: nil,
			},
			expected: `{"worktrees":[]}`,
		},
		{
			name:     "turns zero value",
			response: turnListResponse{},
			expected: `{"turns":[]}`,
		},
		{
			name: "turns explicit nil",
			response: turnListResponse{
				Turns: nil,
			},
			expected: `{"turns":[]}`,
		},
		{
			name:     "processes zero value",
			response: processListResponse{},
			expected: `{"processes":[]}`,
		},
		{
			name: "processes explicit nil",
			response: processListResponse{
				Processes: nil,
			},
			expected: `{"processes":[]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.response)
			if err != nil {
				t.Fatalf("marshal list response: %v", err)
			}
			actual := string(data)
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("mismatch empty list JSON (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestResponseListPreservesPopulatedValues(t *testing.T) {
	data, err := json.Marshal(responseList[string]{"first", "second"})
	if err != nil {
		t.Fatalf("marshal populated response list: %v", err)
	}
	expected := `["first","second"]`
	actual := string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch populated response list JSON (-expected, +actual):\n%s", diff)
	}
}

func TestTurnResponseUsesOneResourceShape(t *testing.T) {
	expectedFields := []string{
		"begin_revision_id",
		"changed_files",
		"duration_ms",
		"end_revision_id",
		"ended_at",
		"files",
		"git_state",
		"goal",
		"instructions",
		"mode",
		"outcome",
		"skills",
		"started_at",
		"state",
		"summary",
		"turn_id",
		"warnings",
		"workspace",
		"worktree",
	}
	actualFields := jsonFieldNames(reflect.TypeFor[turn]())
	if diff := cmp.Diff(expectedFields, actualFields); diff != "" {
		t.Errorf("mismatch turn resource fields (-expected, +actual):\n%s", diff)
	}

	response := turnResponse{
		Turn: turn{
			TurnID: "turn_01",
			Mode:   TurnModeWorktree,
			Worktree: &turnWorktree{
				WorktreeID: "wt_01",
				BaseSHA:    "abc123",
				State:      WorktreeStateClosed,
				Retained:   true,
			},
			Workspace: workspace{
				CWD:     "/worktree",
				GitRoot: "/worktree",
				Branch:  "review",
			},
			State:     TurnStateEnded,
			Outcome:   TurnOutcomeCompleted,
			Goal:      "Review the contract",
			StartedAt: "2026-07-25T00:00:00Z",
			EndedAt:   "2026-07-25T00:01:00Z",
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal turn response: %v", err)
	}
	expected := `{"turn":{"turn_id":"turn_01","mode":"worktree","worktree":{"worktree_id":"wt_01","base_sha":"abc123","state":"closed","retained":true},"workspace":{"cwd":"/worktree","git_root":"/worktree","branch":"review"},"state":"ended","outcome":"completed","goal":"Review the contract","started_at":"2026-07-25T00:00:00Z","ended_at":"2026-07-25T00:01:00Z"}}`
	actual := string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch turn response JSON (-expected, +actual):\n%s", diff)
	}
}

func TestResourceStateTypesExcludeFilterSentinels(t *testing.T) {
	tests := []struct {
		name         string
		resourceType reflect.Type
		filterType   reflect.Type
	}{
		{
			name:         "worktree",
			resourceType: reflect.TypeFor[WorktreeState](),
			filterType:   reflect.TypeFor[WorktreeStateFilter](),
		},
		{
			name:         "turn",
			resourceType: reflect.TypeFor[TurnState](),
			filterType:   reflect.TypeFor[TurnStateFilter](),
		},
		{
			name:         "process",
			resourceType: reflect.TypeFor[ProcessState](),
			filterType:   reflect.TypeFor[ProcessStateFilter](),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.resourceType == test.filterType {
				t.Errorf("resource and filter state types are both %v, expected distinct types", test.resourceType)
			}
		})
	}
}

func TestDiffResponseResolvesCommonTurnKind(t *testing.T) {
	response := diffResponse{
		Kind:   DiffKindTurn,
		TurnID: TurnID("turn_01"),
		Summary: diffSummary{
			FilesChanged: 1,
			Additions:    2,
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal diff response: %v", err)
	}
	expected := `{"kind":"turn","turn_id":"turn_01","summary":{"files_changed":1,"additions":2,"deletions":0,"bytes":0,"truncated":false}}`
	actual := string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch turn diff response JSON (-expected, +actual):\n%s", diff)
	}
}

func TestDiffResponseNormalizesRenameAsDeletedAndAdded(t *testing.T) {
	response := diffResponse{
		Kind: DiffKindGit,
		Summary: diffSummary{
			FilesChanged: 2,
		},
		Files: []diffFile{
			{
				Path:   "old.txt",
				Status: diffFileStatusDeleted,
			},
			{
				Path:   "new.txt",
				Status: diffFileStatusAdded,
			},
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal normalized rename response: %v", err)
	}
	expected := `{"kind":"git","summary":{"files_changed":2,"additions":0,"deletions":0,"bytes":0,"truncated":false},"files":[{"path":"old.txt","status":"deleted"},{"path":"new.txt","status":"added"}]}`
	actual := string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch normalized rename response JSON (-expected, +actual):\n%s", diff)
	}
}

func TestDiffResponseNormalizesCopyAsAddedDestination(t *testing.T) {
	response := diffResponse{
		Kind: DiffKindGit,
		Summary: diffSummary{
			FilesChanged: 1,
		},
		Files: []diffFile{
			{
				Path:   "copy.txt",
				Status: diffFileStatusAdded,
			},
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal normalized copy response: %v", err)
	}
	expected := `{"kind":"git","summary":{"files_changed":1,"additions":0,"deletions":0,"bytes":0,"truncated":false},"files":[{"path":"copy.txt","status":"added"}]}`
	actual := string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch normalized copy response JSON (-expected, +actual):\n%s", diff)
	}
}

func TestStatusResponseIsSessionScoped(t *testing.T) {
	response := statusResponse{
		Ready: true,
		ActiveTurn: &activeTurnSummary{
			TurnID:     "turn_01",
			Repo:       "openrig",
			WorktreeID: "wt_01",
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal status response: %v", err)
	}
	expected := `{"ready":true,"active_turn":{"turn_id":"turn_01","repo":"openrig","worktree_id":"wt_01"}}`
	actual := string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch status response JSON (-expected, +actual):\n%s", diff)
	}
}

func TestLifecycleResponseCanReturnOneExecutableNextAction(t *testing.T) {
	next, err := nextTurnBegin(turnBeginArguments{
		Repo: "openrig",
		Goal: "Fix the parser",
	})
	if err != nil {
		t.Fatalf("nextTurnBegin returned error: %v", err)
	}
	response := nextActionResponse{
		Next: next,
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal turn response: %v", err)
	}
	expected := `{"next":{"tool":"turn","arguments":{"op":"begin","repo":"openrig","goal":"Fix the parser"}}}`
	actual := string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch next action JSON (-expected, +actual):\n%s", diff)
	}
}

func TestNextActionConstructorsBindToolAndOperation(t *testing.T) {
	tests := []struct {
		name     string
		action   func() (nextAction, error)
		expected string
	}{
		{
			name: "turn begin",
			action: func() (nextAction, error) {
				return nextTurnBegin(turnBeginArguments{
					Op:   TurnOpEnd,
					Repo: "openrig",
					Goal: "Fix the parser",
				})
			},
			expected: `{"tool":"turn","arguments":{"op":"begin","repo":"openrig","goal":"Fix the parser"}}`,
		},
		{
			name: "worktree close",
			action: func() (nextAction, error) {
				return nextWorktreeClose(worktreeCloseArguments{
					Op:         WorktreeOpDelete,
					WorktreeID: WorktreeID("wt_01"),
				})
			},
			expected: `{"tool":"worktree","arguments":{"op":"close","worktree_id":"wt_01"}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action, err := test.action()
			if err != nil {
				t.Fatalf("construct next action: %v", err)
			}
			if _, err := decodeArguments(string(action.Tool), TurnScope, action.Arguments); err != nil {
				t.Fatalf("decode constructed next action: %v", err)
			}
			data, err := json.Marshal(action)
			if err != nil {
				t.Fatalf("marshal next action: %v", err)
			}
			actual := string(data)
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("mismatch next action JSON (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestNextActionConstructorsRejectInvalidOperations(t *testing.T) {
	tests := []struct {
		name     string
		action   func() (nextAction, error)
		expected string
	}{
		{
			name: "turn begin missing goal",
			action: func() (nextAction, error) {
				return nextTurnBegin(turnBeginArguments{
					Repo: "openrig",
				})
			},
			expected: "goal",
		},
		{
			name: "turn begin conflicting selectors",
			action: func() (nextAction, error) {
				return nextTurnBegin(turnBeginArguments{
					Repo:       "openrig",
					WorktreeID: "wt_01",
					Goal:       "Fix the parser",
				})
			},
			expected: "worktree_id",
		},
		{
			name: "worktree close missing ID",
			action: func() (nextAction, error) {
				return nextWorktreeClose(worktreeCloseArguments{})
			},
			expected: "worktree_id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.action()
			if err == nil {
				t.Fatal("next action error = nil, expected error")
			}
			actual := result.ErrorOf(err).Field
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("mismatch next action error field (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestShellResponseUsesPerStreamExcerpts(t *testing.T) {
	response := shellResponse{
		CWD: "/worktree",
		Stdout: outputExcerpt{
			Text:         "head\n...\ntail",
			Truncated:    true,
			OmittedBytes: 2048,
		},
		Stderr: outputExcerpt{
			Text: "compile failed",
		},
		ExitCode:   1,
		DurationMS: 842,
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal shell response: %v", err)
	}
	expected := `{"cwd":"/worktree","stdout":{"text":"head\n...\ntail","truncated":true,"omitted_bytes":2048},"stderr":{"text":"compile failed","truncated":false},"exit_code":1,"timed_out":false,"duration_ms":842}`
	actual := string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch shell response JSON (-expected, +actual):\n%s", diff)
	}
}

func TestProcessResourcesUseProcessIDAsSoleIdentity(t *testing.T) {
	expectedInfoFields := []string{
		"command",
		"cwd",
		"ended_at",
		"exit_code",
		"generation",
		"pid",
		"process_id",
		"restart_count",
		"started_at",
		"state",
	}
	actualInfoFields := jsonFieldNames(reflect.TypeFor[processInfo]())
	if diff := cmp.Diff(expectedInfoFields, actualInfoFields); diff != "" {
		t.Errorf("mismatch process info fields (-expected, +actual):\n%s", diff)
	}

	expectedOutputFields := []string{
		"cursor",
		"exit_code",
		"generation",
		"pid",
		"process_id",
		"restart_count",
		"state",
		"stderr",
		"stdout",
	}
	actualOutputFields := jsonFieldNames(reflect.TypeFor[processOutput]())
	if diff := cmp.Diff(expectedOutputFields, actualOutputFields); diff != "" {
		t.Errorf("mismatch process output fields (-expected, +actual):\n%s", diff)
	}
}

func TestProcessReadResponseUsesOneContinuationCursor(t *testing.T) {
	exitCode := 0
	response := processReadResponse{
		Output: processOutput{
			ProcessID:    "proc_01",
			PID:          1234,
			State:        ProcessStateExited,
			ExitCode:     &exitCode,
			Generation:   2,
			RestartCount: 1,
			Stdout: outputExcerpt{
				Text: "done\n",
			},
			Stderr: outputExcerpt{},
			Cursor: "cursor_02",
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal process read response: %v", err)
	}
	expected := `{"output":{"process_id":"proc_01","pid":1234,"state":"exited","exit_code":0,"generation":2,"restart_count":1,"stdout":{"text":"done\n","truncated":false},"stderr":{"text":"","truncated":false},"cursor":"cursor_02"}}`
	actual := string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch process read response JSON (-expected, +actual):\n%s", diff)
	}
}

func TestFiniteResponseVocabulariesMarshalCanonicalValues(t *testing.T) {
	expectedValues := map[string][]string{
		"file provenance": {
			"known_write",
			"observed_external_change",
			"preexisting_change",
			"unknown_change",
		},
		"diff file status": {"added", "modified", "deleted"},
		"patch action":     {"add", "update", "delete", "move"},
		"skill origin":     {"user", "project"},
	}
	actualValues := map[string][]string{
		"file provenance": {
			string(fileProvenanceKnownWrite),
			string(fileProvenanceObservedExternalChange),
			string(fileProvenancePreexistingChange),
			string(fileProvenanceUnknownChange),
		},
		"diff file status": {
			string(diffFileStatusAdded),
			string(diffFileStatusModified),
			string(diffFileStatusDeleted),
		},
		"patch action": {
			string(patchActionAdd),
			string(patchActionUpdate),
			string(patchActionDelete),
			string(patchActionMove),
		},
		"skill origin": {
			string(skillOriginUser),
			string(skillOriginProject),
		},
	}
	if diff := cmp.Diff(expectedValues, actualValues); diff != "" {
		t.Errorf("mismatch finite response vocabularies (-expected, +actual):\n%s", diff)
	}

	response := struct {
		File  fileChange       `json:"file"`
		Diff  diffFile         `json:"diff"`
		Patch patchFileSummary `json:"patch"`
		Skill skillSummary     `json:"skill"`
	}{
		File: fileChange{
			Path:       "internal/tools/responses.go",
			Provenance: fileProvenanceKnownWrite,
		},
		Diff: diffFile{
			Path:   "internal/tools/responses.go",
			Status: diffFileStatusModified,
		},
		Patch: patchFileSummary{
			Action: patchActionUpdate,
			Path:   "internal/tools/responses.go",
		},
		Skill: skillSummary{
			Name:   "go",
			Origin: skillOriginUser,
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal finite response vocabularies: %v", err)
	}
	expected := `{"file":{"path":"internal/tools/responses.go","provenance":"known_write"},"diff":{"path":"internal/tools/responses.go","status":"modified"},"patch":{"action":"update","path":"internal/tools/responses.go","added_lines":0,"deleted_lines":0,"bytes":0},"skill":{"name":"go","description":"","origin":"user"}}`
	actual := string(data)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch finite response vocabulary JSON (-expected, +actual):\n%s", diff)
	}
}

func TestRepresentativeResponsesStayCompact(t *testing.T) {
	tests := []struct {
		name     string
		response any
		maxBytes int
	}{
		{
			name: HelpToolName,
			response: helpResponse{
				Summary: "Begin a turn, work with its turn_id, inspect the diff, then end the turn.",
				CommonFlow: []string{
					"turn begin(repo, goal)",
					"shell, apply_patch, or process with turn_id",
					"diff(turn_id)",
					"turn end(turn_id, outcome)",
				},
			},
			maxBytes: 1024,
		},
		{
			name:     StatusToolName,
			response: statusResponse{Ready: true},
			maxBytes: 512,
		},
		{
			name: WorktreeToolName,
			response: worktreeResponse{
				Worktree: worktree{
					WorktreeID:   "wt_01",
					State:        WorktreeStateOpen,
					SourceRoot:   "/repo",
					WorktreeRoot: "/worktree",
					Base:         "HEAD",
					BaseSHA:      "abc123",
					CreatedAt:    "2026-07-25T00:00:00Z",
					UpdatedAt:    "2026-07-25T00:00:00Z",
				},
			},
			maxBytes: 1024,
		},
		{
			name: TurnToolName,
			response: turnResponse{
				Turn: turn{
					TurnID: "turn_01",
					Mode:   TurnModeWorktree,
					Worktree: &turnWorktree{
						WorktreeID: "wt_01",
						BaseSHA:    "abc123",
						State:      WorktreeStateOpen,
					},
					Workspace: workspace{
						CWD:     "/worktree",
						GitRoot: "/worktree",
					},
					State:     TurnStateActive,
					Goal:      "Fix the parser",
					StartedAt: "2026-07-25T00:00:00Z",
				},
			},
			maxBytes: 1536,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.response)
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			if len(data) > test.maxBytes {
				t.Errorf("serialized response bytes = %d, exceeds budget %d", len(data), test.maxBytes)
			}
		})
	}
}

func jsonFieldNames(value reflect.Type) []string {
	names := make([]string, 0, value.NumField())
	for index := range value.NumField() {
		tag := value.Field(index).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name != "" && name != "-" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
