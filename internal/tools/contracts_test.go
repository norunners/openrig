package tools

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/norunners/openrig/internal/result"
)

func TestContractNamesDefineTheNativeSurface(t *testing.T) {
	expected := []string{
		"help",
		"status",
		"worktree",
		"turn",
		"diff",
		"shell",
		"apply_patch",
		"process",
		"skill_activate",
	}
	actual := ContractNames()
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch contract names (-expected, +actual):\n%s", diff)
	}

	actual[0] = "mutated"
	if diff := cmp.Diff(expected, ContractNames()); diff != "" {
		t.Errorf("mismatch contract names after caller mutation (-expected, +actual):\n%s", diff)
	}
}

func TestContractsDefineExactInputFields(t *testing.T) {
	tests := []struct {
		name       string
		scope      Scope
		properties []string
		required   []string
	}{
		{
			name:       HelpToolName,
			properties: []string{"topic"},
		},
		{
			name:       StatusToolName,
			properties: []string{},
		},
		{
			name:       WorktreeToolName,
			properties: []string{"base", "branch", "op", "reason", "repo", "state", "worktree_id"},
			required:   []string{"op"},
		},
		{
			name:       TurnToolName,
			properties: []string{"goal", "mode", "op", "outcome", "repo", "state", "summary", "turn_id", "worktree_id"},
			required:   []string{"op"},
		},
		{
			name:       DiffToolName,
			properties: []string{"from", "kind", "paths", "stat", "to", "turn_id", "worktree_id"},
		},
		{
			name:       ShellToolName,
			properties: []string{"command", "turn_id", "workdir"},
			required:   []string{"turn_id", "command"},
		},
		{
			name:       ApplyPatchToolName,
			properties: []string{"patch", "turn_id"},
			required:   []string{"turn_id", "patch"},
		},
		{
			name:       ProcessToolName,
			properties: []string{"command", "cursor", "env", "op", "process_id", "state", "turn_id", "workdir"},
			required:   []string{"turn_id", "op"},
		},
		{
			name:       SkillActivateToolName,
			properties: []string{"include_references", "include_scripts", "skill", "turn_id"},
			required:   []string{"turn_id", "skill"},
		},
		{
			name:       ShellToolName,
			scope:      FreestyleScope,
			properties: []string{"command", "repo", "workdir"},
			required:   []string{"repo", "command"},
		},
		{
			name:       ApplyPatchToolName,
			scope:      FreestyleScope,
			properties: []string{"patch", "repo"},
			required:   []string{"repo", "patch"},
		},
		{
			name:       ProcessToolName,
			scope:      FreestyleScope,
			properties: []string{"command", "cursor", "env", "op", "process_id", "repo", "state", "workdir"},
			required:   []string{"repo", "op"},
		},
		{
			name:       SkillActivateToolName,
			scope:      FreestyleScope,
			properties: []string{"include_references", "include_scripts", "repo", "skill"},
			required:   []string{"repo", "skill"},
		},
	}

	for _, test := range tests {
		testName := test.name + "/" + scopeName(test.scope)
		t.Run(testName, func(t *testing.T) {
			contract, err := Contract(test.name, test.scope)
			if err != nil {
				t.Fatalf("Contract returned error: %v", err)
			}
			actualProperties := make([]string, 0, len(contract.InputSchema.Properties))
			for property := range contract.InputSchema.Properties {
				actualProperties = append(actualProperties, property)
			}
			sort.Strings(actualProperties)
			if diff := cmp.Diff(test.properties, actualProperties); diff != "" {
				t.Errorf("mismatch input properties (-expected, +actual):\n%s", diff)
			}
			expectedRequired := append([]string(nil), test.required...)
			actualRequired := append([]string(nil), contract.InputSchema.Required...)
			sort.Strings(expectedRequired)
			sort.Strings(actualRequired)
			if diff := cmp.Diff(expectedRequired, actualRequired); diff != "" {
				t.Errorf("mismatch required input fields (-expected, +actual):\n%s", diff)
			}
			if diff := cmp.Diff(any(false), contract.InputSchema.AdditionalProperties); diff != "" {
				t.Errorf("mismatch additionalProperties (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestContractsOmitOutputSchemas(t *testing.T) {
	contracts, err := Contracts(TurnScope)
	if err != nil {
		t.Fatalf("Contracts returned error: %v", err)
	}
	for _, contract := range contracts {
		t.Run(contract.Name, func(t *testing.T) {
			encoded, err := json.Marshal(contract)
			if err != nil {
				t.Fatalf("marshal contract: %v", err)
			}
			var document map[string]any
			if err := json.Unmarshal(encoded, &document); err != nil {
				t.Fatalf("unmarshal contract: %v", err)
			}
			if _, ok := document["outputSchema"]; ok {
				t.Error("outputSchema is published in the bounded common catalog")
			}
		})
	}
}

func TestContractDescriptionsAreComplete(t *testing.T) {
	for _, scope := range []Scope{TurnScope, FreestyleScope} {
		contracts, err := Contracts(scope)
		if err != nil {
			t.Fatalf("Contracts returned error: %v", err)
		}
		for _, contract := range contracts {
			t.Run(contract.Name+"/"+scopeName(scope), func(t *testing.T) {
				if contract.Description == "" {
					t.Error("contract description is empty")
				}
				for field := range contract.InputSchema.Properties {
					property := propertySchema(t, &contract, field)
					if property.Description == "" {
						t.Errorf("input property %q description is missing", field)
					}
				}
			})
		}
	}
}

func TestContractSchemaConstraints(t *testing.T) {
	tests := []struct {
		tool     string
		field    string
		expected schemaConstraints
	}{
		{
			tool:     WorktreeToolName,
			field:    "op",
			expected: schemaConstraints{Enum: []any{"open", "list", "status", "close", "delete"}},
		},
		{
			tool:     TurnToolName,
			field:    "mode",
			expected: schemaConstraints{Enum: []any{"worktree", "freestyle"}},
		},
		{
			tool:     TurnToolName,
			field:    "outcome",
			expected: schemaConstraints{Enum: []any{"completed", "blocked", "abandoned", "design_only", "verification_only"}},
		},
		{
			tool:     ShellToolName,
			field:    "workdir",
			expected: schemaConstraints{MinLength: intPointer(1)},
		},
		{
			tool:     DiffToolName,
			field:    "kind",
			expected: schemaConstraints{Enum: []any{"worktree", "git", "revision"}},
		},
		{
			tool:     ProcessToolName,
			field:    "op",
			expected: schemaConstraints{Enum: []any{"start", "status", "read", "stop", "restart", "kill"}},
		},
		{
			tool:  SkillActivateToolName,
			field: "skill",
			expected: schemaConstraints{
				MaxLength: intPointer(64),
				Pattern:   `^[a-z0-9]+(?:-[a-z0-9]+)*$`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.tool+"/"+test.field, func(t *testing.T) {
			contract, err := Contract(test.tool, TurnScope)
			if err != nil {
				t.Fatalf("Contract returned error: %v", err)
			}
			property := propertySchema(t, contract, test.field)
			actual := schemaConstraints{
				Enum:      property.Enum,
				MinLength: property.MinLength,
				MaxLength: property.MaxLength,
				Pattern:   property.Pattern,
			}
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("mismatch schema constraints (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestExecutionDescriptionsDefineWorkdirSemantics(t *testing.T) {
	tests := []struct {
		tool     string
		field    string
		contains []string
	}{
		{
			tool:     ShellToolName,
			field:    "workdir",
			contains: []string{"scoped workspace root", "Defaults to that root"},
		},
		{
			tool:     ProcessToolName,
			field:    "workdir",
			contains: []string{"scoped workspace root", "Defaults to that root"},
		},
	}

	for _, test := range tests {
		t.Run(test.tool+"/"+test.field, func(t *testing.T) {
			contract, err := Contract(test.tool, TurnScope)
			if err != nil {
				t.Fatalf("Contract returned error: %v", err)
			}
			description := propertySchema(t, contract, test.field).Description
			for _, expected := range test.contains {
				if !strings.Contains(description, expected) {
					t.Errorf("description %q does not contain %q", description, expected)
				}
			}
		})
	}
}

func TestLifecycleDescriptionsDefineRequiredIntentAndBoundedListings(t *testing.T) {
	tests := []struct {
		tool     string
		field    string
		contains []string
	}{
		{
			tool:     TurnToolName,
			field:    "goal",
			contains: []string{"required", "begin"},
		},
		{
			tool:     WorktreeToolName,
			contains: []string{"runtime-bounded snapshot", "recent matches", "deterministic order"},
		},
		{
			tool:     TurnToolName,
			contains: []string{"runtime-bounded snapshot", "recent matches", "deterministic order"},
		},
		{
			tool:     ProcessToolName,
			contains: []string{"runtime-bounded snapshot", "recent matches", "deterministic order"},
		},
	}

	for _, test := range tests {
		testName := test.tool
		if test.field != "" {
			testName += "/" + test.field
		}
		t.Run(testName, func(t *testing.T) {
			contract, err := Contract(test.tool, TurnScope)
			if err != nil {
				t.Fatalf("Contract returned error: %v", err)
			}
			description := contract.Description
			if test.field != "" {
				description = propertySchema(t, contract, test.field).Description
			}
			for _, expected := range test.contains {
				if !strings.Contains(description, expected) {
					t.Errorf("description %q does not contain %q", description, expected)
				}
			}
		})
	}
}

func TestContractsPublishOnlyToolWideDefaults(t *testing.T) {
	expected := map[string]any{
		"diff.stat":                         false,
		"skill_activate.include_scripts":    false,
		"skill_activate.include_references": false,
	}

	contracts, err := Contracts(TurnScope)
	if err != nil {
		t.Fatalf("Contracts returned error: %v", err)
	}
	actual := make(map[string]any)
	for _, contract := range contracts {
		for field := range contract.InputSchema.Properties {
			property := propertySchema(t, &contract, field)
			if len(property.Default) > 0 {
				var value any
				if err := json.Unmarshal(property.Default, &value); err != nil {
					t.Fatalf("decode %s.%s default: %v", contract.Name, field, err)
				}
				actual[contract.Name+"."+field] = value
			}
		}
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch published schema defaults (-expected, +actual):\n%s", diff)
	}
}

func TestInputContractsStayWithinProtocolBudgets(t *testing.T) {
	// These budgets cover only names, descriptions, and input schemas. They
	// prevent growth in the model-visible input vocabulary independently from
	// the full effective-catalog budget. Output schemas are intentionally
	// omitted; reviewed annotations are covered by catalog tests.
	const (
		maxInputCatalogBytes     = 8 * 1024
		maxInputContractBytes    = 2 * 1024
		maxInputSchemaBytes      = 1800
		maxToolDescriptionBytes  = 200
		maxFieldDescriptionBytes = 240
	)

	type inputContract struct {
		Name        string              `json:"name"`
		Description string              `json:"description,omitempty"`
		InputSchema mcp.ToolInputSchema `json:"inputSchema"`
	}

	for _, scope := range []Scope{TurnScope, FreestyleScope} {
		t.Run(scopeName(scope), func(t *testing.T) {
			contracts, err := Contracts(scope)
			if err != nil {
				t.Fatalf("Contracts returned error: %v", err)
			}
			inputContracts := make([]inputContract, 0, len(contracts))
			for _, contract := range contracts {
				inputContracts = append(inputContracts, inputContract{
					Name:        contract.Name,
					Description: contract.Description,
					InputSchema: contract.InputSchema,
				})
			}
			catalog, err := json.Marshal(struct {
				Tools []inputContract `json:"tools"`
			}{
				Tools: inputContracts,
			})
			if err != nil {
				t.Fatalf("marshal input catalog: %v", err)
			}
			if len(catalog) > maxInputCatalogBytes {
				t.Errorf("serialized input catalog bytes = %d, exceeds budget %d", len(catalog), maxInputCatalogBytes)
			}

			for index, contract := range contracts {
				t.Run(contract.Name, func(t *testing.T) {
					encoded, err := json.Marshal(inputContracts[index])
					if err != nil {
						t.Fatalf("marshal input contract: %v", err)
					}
					if len(encoded) > maxInputContractBytes {
						t.Errorf("serialized input contract bytes = %d, exceeds budget %d", len(encoded), maxInputContractBytes)
					}
					encoded, err = json.Marshal(contract.InputSchema)
					if err != nil {
						t.Fatalf("marshal input schema: %v", err)
					}
					if len(encoded) > maxInputSchemaBytes {
						t.Errorf("serialized input schema bytes = %d, exceeds budget %d", len(encoded), maxInputSchemaBytes)
					}
					if len(contract.Description) > maxToolDescriptionBytes {
						t.Errorf("tool description bytes = %d, exceeds budget %d", len(contract.Description), maxToolDescriptionBytes)
					}
					for field := range contract.InputSchema.Properties {
						description := propertySchema(t, &contract, field).Description
						if len(description) > maxFieldDescriptionBytes {
							t.Errorf("%s description bytes = %d, exceeds budget %d", field, len(description), maxFieldDescriptionBytes)
						}
					}
				})
			}
		})
	}
}

func TestContractsOmitNonessentialAgentKnobs(t *testing.T) {
	expected := map[string][]string{
		ApplyPatchToolName: {"dry_run"},
		DiffToolName:       {"max_bytes"},
		ProcessToolName:    {"grace_ms", "limit", "max_buffer_bytes", "max_bytes", "name"},
		ShellToolName:      {"max_output_bytes", "stdin", "timeout_ms"},
		SkillActivateToolName: {
			"max_bytes",
		},
		TurnToolName:     {"limit"},
		WorktreeToolName: {"limit"},
	}

	for toolName, fields := range expected {
		t.Run(toolName, func(t *testing.T) {
			contract, err := Contract(toolName, TurnScope)
			if err != nil {
				t.Fatalf("Contract returned error: %v", err)
			}
			for _, field := range fields {
				if _, exists := contract.InputSchema.Properties[field]; exists {
					t.Errorf("runtime policy field %q is published", field)
				}
			}
		})
	}
}

func TestContractRejectsUnknownNameAndScope(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		scope Scope
		field string
	}{
		{
			name:  "unknown tool",
			tool:  "turn_begin",
			field: "tool",
		},
		{
			name:  "invalid scope",
			tool:  ShellToolName,
			scope: Scope(99),
			field: "scope",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contract, err := Contract(test.tool, test.scope)
			if err == nil {
				t.Fatal("Contract error = nil, expected error")
			}
			if contract != nil {
				t.Errorf("Contract result = %#v, expected nil", contract)
			}
			var actual *result.Error
			if !errors.As(err, &actual) {
				t.Fatalf("Contract error type = %T, expected *result.Error", err)
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
				t.Errorf("mismatch contract error (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestContractsRejectUnsupportedScope(t *testing.T) {
	contracts, err := Contracts(Scope(99))
	if err == nil {
		t.Fatal("Contracts error = nil, expected error")
	}
	if contracts != nil {
		t.Errorf("Contracts result = %#v, expected nil", contracts)
	}
}

func TestScopedContractConstructorsRejectUnsupportedScope(t *testing.T) {
	tests := []struct {
		name        string
		constructor func(Scope) (*mcp.Tool, error)
	}{
		{
			name:        ShellToolName,
			constructor: shellContract,
		},
		{
			name:        ApplyPatchToolName,
			constructor: applyPatchContract,
		},
		{
			name:        ProcessToolName,
			constructor: processContract,
		},
		{
			name:        SkillActivateToolName,
			constructor: skillActivateContract,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contract, err := test.constructor(Scope(99))
			if err == nil {
				t.Fatal("scoped contract constructor error = nil, expected error")
			}
			if contract != nil {
				t.Errorf("scoped contract constructor result = %#v, expected nil", contract)
			}
		})
	}
}

func TestScopeOptionRejectsUnsupportedScope(t *testing.T) {
	option, err := scopeOption(Scope(99))
	if err == nil {
		t.Fatal("scopeOption error = nil, expected error")
	}
	if option != nil {
		t.Errorf("scopeOption result = %#v, expected nil", option)
	}
	var actual *result.Error
	if !errors.As(err, &actual) {
		t.Fatalf("scopeOption error type = %T, expected *result.Error", err)
	}
	expected := struct {
		Code       result.Code
		Field      string
		Suggestion string
	}{
		Code:       result.CodeInvalidArgument,
		Field:      "scope",
		Suggestion: "use TurnScope or FreestyleScope",
	}
	actualState := struct {
		Code       result.Code
		Field      string
		Suggestion string
	}{
		Code:       actual.Code,
		Field:      actual.Field,
		Suggestion: actual.Suggestion,
	}
	if diff := cmp.Diff(expected, actualState); diff != "" {
		t.Errorf("mismatch scope option error (-expected, +actual):\n%s", diff)
	}
}

func scopeName(scope Scope) string {
	if scope == FreestyleScope {
		return "freestyle"
	}
	return "turn"
}

type schemaConstraints struct {
	Enum      []any
	MinLength *int
	MaxLength *int
	Pattern   string
}

func propertySchema(t *testing.T, contract *mcp.Tool, field string) jsonschema.Schema {
	t.Helper()
	raw, ok := contract.InputSchema.Properties[field]
	if !ok {
		t.Fatalf("input property %q is missing", field)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal input property %q: %v", field, err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode input property %q: %v", field, err)
	}
	return schema
}

func intPointer(value int) *int {
	return &value
}
