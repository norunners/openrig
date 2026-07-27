package tools

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/norunners/openrig/internal/result"
)

func TestRenderHelpDefinesCommonTurnFlow(t *testing.T) {
	selection := CatalogSelection{
		Available:       ContractNames(),
		SkillsAvailable: true,
	}
	actual, err := renderHelp(selection, "")
	if err != nil {
		t.Fatalf("renderHelp returned error: %v", err)
	}
	expected := helpResponse{
		Summary: "Begin a turn with a repository and goal, work with its turn_id, inspect the diff, then end the turn.",
		CommonFlow: []string{
			"turn begin(repo, goal)",
			"work tools with turn_id",
			"diff(turn_id)",
			"turn end(turn_id, outcome)",
		},
		Examples: []string{
			`{"tool":"turn","arguments":{"op":"begin","repo":"openrig","goal":"Fix the parser"}}`,
			`{"tool":"shell","arguments":{"turn_id":"turn_...","command":"go test ./..."}}`,
			`{"tool":"diff","arguments":{"turn_id":"turn_..."}}`,
			`{"tool":"turn","arguments":{"op":"end","turn_id":"turn_...","outcome":"completed","summary":"Implemented and verified."}}`,
		},
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch common help (-expected, +actual):\n%s", diff)
	}
}

func TestRenderHelpUsesOnlySelectedCapabilities(t *testing.T) {
	tests := []struct {
		name      string
		selection CatalogSelection
		topic     string
		contains  []string
		omits     []string
	}{
		{
			name: "common flow omits unavailable diff",
			selection: CatalogSelection{
				Available: []string{
					HelpToolName,
					TurnToolName,
					ShellToolName,
				},
			},
			contains: []string{"turn begin", "work tools", "turn end"},
			omits:    []string{"diff"},
		},
		{
			name: "advanced reports no available capability",
			selection: CatalogSelection{
				Available: []string{HelpToolName},
			},
			topic:    helpTopicAdvanced,
			contains: []string{"No advanced lifecycle or process capability"},
		},
		{
			name: "unavailable capability is explicit",
			selection: CatalogSelection{
				Available: []string{HelpToolName},
			},
			topic:    "diff",
			contains: []string{"diff is not available"},
		},
		{
			name: "freestyle uses repository scope",
			selection: CatalogSelection{
				Scope:     FreestyleScope,
				Available: []string{HelpToolName, ShellToolName},
			},
			contains: []string{"repo", "no turn_id is created"},
			omits:    []string{"diff(turn_id)", "turn end"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := renderHelp(test.selection, test.topic)
			if err != nil {
				t.Fatalf("renderHelp returned error: %v", err)
			}
			data, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("marshal help response: %v", err)
			}
			actual := string(data)
			for _, expected := range test.contains {
				if !strings.Contains(actual, expected) {
					t.Errorf("help response %q does not contain %q", actual, expected)
				}
			}
			for _, unexpected := range test.omits {
				if strings.Contains(actual, unexpected) {
					t.Errorf("help response %q contains unavailable guidance %q", actual, unexpected)
				}
			}
		})
	}
}

func TestRenderHelpHandlesPartialCatalogPrerequisites(t *testing.T) {
	tests := []struct {
		name      string
		selection CatalogSelection
		topic     string
		expected  helpResponse
	}{
		{
			name: "work tool without turn requires recovered ID",
			selection: CatalogSelection{
				Available: []string{HelpToolName, ShellToolName},
			},
			expected: helpResponse{
				Summary:    "Available executable work requires an existing active turn_id; the current catalog cannot begin a turn.",
				CommonFlow: []string{},
			},
		},
		{
			name: "turn without work does not advertise common flow",
			selection: CatalogSelection{
				Available: []string{HelpToolName, TurnToolName},
			},
			expected: helpResponse{
				Summary:    "Turn lifecycle is available, but no executable work capability is available in the current catalog.",
				CommonFlow: []string{},
			},
		},
		{
			name: "freestyle turn topic requires turn",
			selection: CatalogSelection{
				Available: []string{HelpToolName},
			},
			topic: helpTopicFreestyle,
			expected: helpResponse{
				Summary:    "OpenRig exposes only complete capabilities available in the current runtime.",
				CommonFlow: []string{},
				Topic:      helpTopicFreestyle,
				Details: []string{
					"Freestyle turns are unavailable because turn is not selected in the current catalog.",
				},
			},
		},
		{
			name: "process without turn requires recovered ID",
			selection: CatalogSelection{
				Available: []string{HelpToolName, ProcessToolName},
			},
			topic: helpTopicAdvanced,
			expected: helpResponse{
				Summary:    "Available executable work requires an existing active turn_id; the current catalog cannot begin a turn.",
				CommonFlow: []string{},
				Topic:      helpTopicAdvanced,
				Details: []string{
					"Process requires an existing active turn_id because the current catalog cannot begin a turn; process_id is the sole identity of a supervised process.",
				},
				Examples: []string{
					`{"tool":"process","arguments":{"turn_id":"turn_...","op":"start","command":"go test ./..."}}`,
					`{"tool":"process","arguments":{"turn_id":"turn_...","op":"read","process_id":"proc_..."}}`,
				},
			},
		},
		{
			name: "freestyle scope without work is explicit",
			selection: CatalogSelection{
				Scope:     FreestyleScope,
				Available: []string{HelpToolName},
			},
			expected: helpResponse{
				Summary:    "No work capability is available in the current freestyle catalog.",
				CommonFlow: []string{},
			},
		},
		{
			name: "worktree without turn omits automatic turn guidance",
			selection: CatalogSelection{
				Available: []string{HelpToolName, WorktreeToolName},
			},
			topic: helpTopicWorktree,
			expected: helpResponse{
				Summary:    "Worktree lifecycle operations are available without a turn.",
				CommonFlow: []string{},
				Topic:      helpTopicWorktree,
				Details: []string{
					"Worktree manages isolated Git checkouts directly.",
				},
				Examples: []string{
					`{"tool":"worktree","arguments":{"op":"open","repo":"openrig","base":"HEAD"}}`,
				},
			},
		},
		{
			name: "turn topic without work omits begin example",
			selection: CatalogSelection{
				Available: []string{HelpToolName, TurnToolName},
			},
			topic: helpTopicTurn,
			expected: helpResponse{
				Summary:    "Turn lifecycle is available, but no executable work capability is available in the current catalog.",
				CommonFlow: []string{},
				Topic:      helpTopicTurn,
				Details: []string{
					"Turn is the scoped capability and audit unit, but no executable work capability is available in the current catalog.",
				},
			},
		},
		{
			name: "skill guidance does not satisfy executable work",
			selection: CatalogSelection{
				Available:       []string{HelpToolName, TurnToolName, SkillActivateToolName},
				SkillsAvailable: true,
			},
			expected: helpResponse{
				Summary:    "Turn lifecycle and skill guidance are available, but no executable work capability is available in the current catalog.",
				CommonFlow: []string{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := renderHelp(test.selection, test.topic)
			if err != nil {
				t.Fatalf("renderHelp returned error: %v", err)
			}
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("mismatch partial-catalog help (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestRenderHelpDescribesReachableDiffOperations(t *testing.T) {
	tests := []struct {
		name      string
		available []string
		expected  helpResponse
	}{
		{
			name:      "diff only uses known revisions",
			available: []string{HelpToolName, DiffToolName},
			expected: helpResponse{
				Summary:    "Explicit worktree, Git, and revision diffs are available with known selectors; turn diffs require a known turn_id.",
				CommonFlow: []string{},
				Topic:      helpTopicDiff,
				Details: []string{
					"Explicit revision comparisons require known revision IDs. Worktree and Git comparisons similarly require a known worktree_id.",
					"The common turn diff remains available when an active or ended turn_id is known.",
				},
				Examples: []string{
					`{"tool":"diff","arguments":{"kind":"revision","from":"rev_...","to":"rev_..."}}`,
				},
			},
		},
		{
			name:      "worktree and diff use managed worktree",
			available: []string{HelpToolName, WorktreeToolName, DiffToolName},
			expected: helpResponse{
				Summary:    "Worktree lifecycle and explicit diff inspection are available without a turn; turn diffs require a known turn_id.",
				CommonFlow: []string{},
				Topic:      helpTopicDiff,
				Details: []string{
					"Open or inspect a managed worktree, then use its worktree_id with kind=worktree or kind=git; no turn_id is required.",
					"Explicit revision comparisons remain available when both revision IDs are known.",
				},
				Examples: []string{
					`{"tool":"worktree","arguments":{"op":"open","repo":"openrig","base":"HEAD"}}`,
					`{"tool":"diff","arguments":{"kind":"worktree","worktree_id":"wt_..."}}`,
				},
			},
		},
		{
			name:      "turn and diff lead with common turn diff",
			available: []string{HelpToolName, TurnToolName, DiffToolName},
			expected: helpResponse{
				Summary:    "Turn lifecycle and diff inspection are available, but no executable work capability is available in the current catalog.",
				CommonFlow: []string{},
				Topic:      helpTopicDiff,
				Details: []string{
					"Supplying only turn_id renders the common turn diff. Explicit kinds support advanced worktree, Git, or revision comparisons.",
				},
				Examples: []string{
					`{"tool":"diff","arguments":{"turn_id":"turn_..."}}`,
				},
			},
		},
		{
			name:      "turn worktree and diff expose both selectors",
			available: []string{HelpToolName, TurnToolName, WorktreeToolName, DiffToolName},
			expected: helpResponse{
				Summary:    "Turn lifecycle and diff inspection are available, but no executable work capability is available in the current catalog.",
				CommonFlow: []string{},
				Topic:      helpTopicDiff,
				Details: []string{
					"Supplying only turn_id renders the common turn diff. Explicit kinds support advanced worktree, Git, or revision comparisons.",
					"Use worktree_id with kind=worktree or kind=git when comparing a managed checkout directly.",
				},
				Examples: []string{
					`{"tool":"diff","arguments":{"turn_id":"turn_..."}}`,
					`{"tool":"diff","arguments":{"kind":"worktree","worktree_id":"wt_..."}}`,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selection := CatalogSelection{Available: test.available}
			actual, err := renderHelp(selection, helpTopicDiff)
			if err != nil {
				t.Fatalf("renderHelp returned error: %v", err)
			}
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("mismatch diff help (-expected, +actual):\n%s", diff)
			}
			available := make(map[string]bool, len(test.available))
			for _, name := range test.available {
				available[name] = true
			}
			assertHelpExamplesSelectedAndDecodable(
				t,
				selection,
				available,
				actual.Examples,
			)
		})
	}
}

func TestRenderHelpSupervisesProcessOnlyWork(t *testing.T) {
	tests := []struct {
		name      string
		selection CatalogSelection
		topic     string
		expected  helpResponse
	}{
		{
			name: "turn scope",
			selection: CatalogSelection{
				Available: []string{HelpToolName, TurnToolName, ProcessToolName},
			},
			expected: helpResponse{
				Summary: "Begin a turn with a repository and goal, work with its turn_id, then end the turn.",
				CommonFlow: []string{
					"turn begin(repo, goal)",
					"process start with turn_id",
					"process read(process_id) until terminal, then inspect exit_code and output",
					"turn end only after the process outcome is known",
				},
				Examples: []string{
					`{"tool":"turn","arguments":{"op":"begin","repo":"openrig","goal":"Fix the parser"}}`,
					`{"tool":"process","arguments":{"turn_id":"turn_...","op":"start","command":"go test ./..."}}`,
					`{"tool":"process","arguments":{"turn_id":"turn_...","op":"read","process_id":"proc_..."}}`,
					`{"tool":"turn","arguments":{"op":"end","turn_id":"turn_...","outcome":"completed","summary":"Process completed successfully."}}`,
				},
			},
		},
		{
			name: "freestyle common",
			selection: CatalogSelection{
				Scope:     FreestyleScope,
				Available: []string{HelpToolName, ProcessToolName},
			},
			expected: helpResponse{
				Summary: "In freestyle scope, supervise process work against a repository directly; no turn_id is created.",
				CommonFlow: []string{
					"Start process work with repo set to the configured name, alias, or allowed path.",
					"Read process_id until it reaches a terminal state, then inspect exit_code and output.",
				},
				Examples: []string{
					`{"tool":"process","arguments":{"repo":"openrig","op":"start","command":"go test ./..."}}`,
					`{"tool":"process","arguments":{"repo":"openrig","op":"read","process_id":"proc_..."}}`,
				},
			},
		},
		{
			name: "freestyle topic",
			selection: CatalogSelection{
				Scope:     FreestyleScope,
				Available: []string{HelpToolName, ProcessToolName},
			},
			topic: helpTopicFreestyle,
			expected: helpResponse{
				Summary: "In freestyle scope, supervise process work against a repository directly; no turn_id is created.",
				CommonFlow: []string{
					"Start process work with repo set to the configured name, alias, or allowed path.",
					"Read process_id until it reaches a terminal state, then inspect exit_code and output.",
				},
				Topic: helpTopicFreestyle,
				Details: []string{
					"Freestyle process work is scoped directly to a repository.",
					"Start the process with repo, then read process_id until its terminal state and exit code are known.",
				},
				Examples: []string{
					`{"tool":"process","arguments":{"repo":"openrig","op":"start","command":"go test ./..."}}`,
					`{"tool":"process","arguments":{"repo":"openrig","op":"read","process_id":"proc_..."}}`,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := renderHelp(test.selection, test.topic)
			if err != nil {
				t.Fatalf("renderHelp returned error: %v", err)
			}
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("mismatch process-only help (-expected, +actual):\n%s", diff)
			}
			contracts, err := SelectContracts(test.selection)
			if err != nil {
				t.Fatalf("SelectContracts returned error: %v", err)
			}
			assertHelpExamplesSelectedAndDecodable(
				t,
				test.selection,
				catalogToolNames(contracts),
				actual.Examples,
			)
		})
	}
}

func TestHelpIsReachableAcrossCatalogSubsets(t *testing.T) {
	names := ContractNames()
	for _, scope := range []Scope{TurnScope, FreestyleScope} {
		for mask := 0; mask < 1<<len(names); mask++ {
			availableNames := make([]string, 0, len(names))
			for index, name := range names {
				if mask&(1<<index) != 0 {
					availableNames = append(availableNames, name)
				}
			}
			selection := CatalogSelection{
				Scope:           scope,
				Available:       availableNames,
				SkillsAvailable: true,
			}
			contracts, err := SelectContracts(selection)
			if err != nil {
				t.Fatalf("SelectContracts scope=%d mask=%d: %v", scope, mask, err)
			}
			available := catalogToolNames(contracts)
			response, err := renderHelp(selection, helpTopicCommon)
			if err != nil {
				t.Fatalf("renderHelp scope=%d mask=%d: %v", scope, mask, err)
			}

			hasExecutableWork := hasAvailableExecutableWorkTool(available)
			expectedFlow := hasExecutableWork
			if scope == TurnScope {
				expectedFlow = available[TurnToolName] && hasExecutableWork
			}
			actualFlow := len(response.CommonFlow) > 0
			if diff := cmp.Diff(expectedFlow, actualFlow); diff != "" {
				t.Errorf("mismatch reachable flow scope=%d mask=%d (-expected, +actual):\n%s", scope, mask, diff)
			}
			if !expectedFlow && len(response.Examples) != 0 {
				t.Errorf("unreachable common help scope=%d mask=%d has examples: %#v", scope, mask, response.Examples)
			}
			assertHelpExamplesSelectedAndDecodable(t, selection, available, response.Examples)

			for _, topic := range helpTopics {
				response, err := renderHelp(selection, topic)
				if err != nil {
					t.Fatalf("renderHelp topic=%q scope=%d mask=%d: %v", topic, scope, mask, err)
				}
				assertHelpExamplesSelectedAndDecodable(
					t,
					selection,
					available,
					response.Examples,
				)
			}
		}
	}
}

func TestRenderHelpRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		selection CatalogSelection
		topic     string
		field     string
	}{
		{
			name: "invalid catalog",
			selection: CatalogSelection{
				Available: []string{"turn_begin"},
			},
			field: "available",
		},
		{
			name:      "invalid topic",
			selection: CatalogSelection{},
			topic:     "tools",
			field:     "topic",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := renderHelp(test.selection, test.topic)
			if err == nil {
				t.Error("renderHelp error = nil, expected error")
				return
			}
			expected := test.field
			actual := result.ErrorOf(err).Field
			if diff := cmp.Diff(expected, actual); diff != "" {
				t.Errorf("mismatch error field (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestHelpTopicsStayInSchemaDecoderAndRendererParity(t *testing.T) {
	contract, err := Contract(HelpToolName, TurnScope)
	if err != nil {
		t.Fatalf("Contract returned error: %v", err)
	}
	schemaTopics := propertySchema(t, contract, "topic").Enum
	expectedTopics := make([]any, len(helpTopics))
	for index, topic := range helpTopics {
		expectedTopics[index] = topic
	}
	if diff := cmp.Diff(expectedTopics, schemaTopics); diff != "" {
		t.Errorf("mismatch help topic schema (-expected, +actual):\n%s", diff)
	}

	selection := CatalogSelection{
		Available:       ContractNames(),
		SkillsAvailable: true,
	}
	for _, topic := range helpTopics {
		t.Run(topic, func(t *testing.T) {
			raw, err := json.Marshal(struct {
				Topic string `json:"topic"`
			}{Topic: topic})
			if err != nil {
				t.Fatalf("marshal help arguments: %v", err)
			}
			if _, err := decodeArguments(HelpToolName, TurnScope, json.RawMessage(raw)); err != nil {
				t.Errorf("decodeArguments rejected renderer topic: %v", err)
			}
			response, err := renderHelp(selection, topic)
			if err != nil {
				t.Fatalf("renderHelp returned error: %v", err)
			}
			if topic != helpTopicCommon &&
				len(response.Details) == 0 &&
				len(response.Examples) == 0 {
				t.Error("accepted help topic has no topic-specific guidance")
			}
		})
	}

	_, err = decodeArguments(
		HelpToolName,
		TurnScope,
		json.RawMessage(`{"topic":"tools"}`),
	)
	if err == nil {
		t.Fatal("decodeArguments error = nil, expected error")
	}
	expectedField := "topic"
	actualField := result.ErrorOf(err).Field
	if diff := cmp.Diff(expectedField, actualField); diff != "" {
		t.Errorf("mismatch invalid topic field (-expected, +actual):\n%s", diff)
	}
}

func TestSchemaHelpExplainsCatalogBoundaries(t *testing.T) {
	response, err := renderHelp(CatalogSelection{
		Available: ContractNames(),
	}, "schemas")
	if err != nil {
		t.Fatalf("renderHelp returned error: %v", err)
	}
	actual := strings.Join(response.Details, "\n")
	for _, expected := range []string{
		"tools/list",
		"omits full output schemas",
		"stale",
		"refresh or reconnect",
	} {
		if !strings.Contains(actual, expected) {
			t.Errorf("schema help %q does not contain %q", actual, expected)
		}
	}
}

func assertHelpExamplesSelectedAndDecodable(
	t *testing.T,
	selection CatalogSelection,
	available map[string]bool,
	examples []string,
) {
	t.Helper()
	for _, example := range examples {
		var call struct {
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(example), &call); err != nil {
			t.Fatalf("decode help example: %v", err)
		}
		if !available[call.Tool] {
			t.Errorf("help example uses unavailable tool %q", call.Tool)
			continue
		}
		if _, err := decodeArguments(call.Tool, selection.Scope, call.Arguments); err != nil {
			t.Errorf("help example is not decodable: %v", err)
		}
	}
}

func hasAvailableExecutableWorkTool(available map[string]bool) bool {
	for _, name := range []string{
		ShellToolName,
		ApplyPatchToolName,
		ProcessToolName,
	} {
		if available[name] {
			return true
		}
	}
	return false
}

func TestHelpSnapshots(t *testing.T) {
	tests := []struct {
		name      string
		selection CatalogSelection
		topic     string
	}{
		{
			name: "common",
			selection: CatalogSelection{
				Available:       ContractNames(),
				SkillsAvailable: true,
			},
		},
		{
			name: "advanced",
			selection: CatalogSelection{
				Available:       ContractNames(),
				SkillsAvailable: true,
			},
			topic: "advanced",
		},
		{
			name: "freestyle",
			selection: CatalogSelection{
				Scope:           FreestyleScope,
				Available:       ContractNames(),
				SkillsAvailable: true,
			},
			topic: "freestyle",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, err := renderHelp(test.selection, test.topic)
			if err != nil {
				t.Fatalf("renderHelp returned error: %v", err)
			}
			assertJSONSnapshot(t, filepath.Join("testdata", "help", test.name+".json"), response)
		})
	}
}
