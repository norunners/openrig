package tools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/norunners/openrig/internal/result"
)

func TestSelectContracts(t *testing.T) {
	all := ContractNames()
	tests := []struct {
		name      string
		selection CatalogSelection
		expected  []string
	}{
		{
			name: "turn scope complete surface",
			selection: CatalogSelection{
				Available:       all,
				SkillsAvailable: true,
			},
			expected: all,
		},
		{
			name: "turn scope without skills",
			selection: CatalogSelection{
				Available: all,
			},
			expected: []string{
				HelpToolName,
				StatusToolName,
				WorktreeToolName,
				TurnToolName,
				DiffToolName,
				ShellToolName,
				ApplyPatchToolName,
				ProcessToolName,
			},
		},
		{
			name: "freestyle scope",
			selection: CatalogSelection{
				Scope:           FreestyleScope,
				Available:       all,
				SkillsAvailable: true,
			},
			expected: []string{
				HelpToolName,
				ShellToolName,
				ApplyPatchToolName,
				ProcessToolName,
				SkillActivateToolName,
			},
		},
		{
			name: "explicit completed handlers retain catalog order",
			selection: CatalogSelection{
				Available: []string{
					ProcessToolName,
					HelpToolName,
					TurnToolName,
				},
			},
			expected: []string{
				HelpToolName,
				TurnToolName,
				ProcessToolName,
			},
		},
		{
			name:      "no completed handlers",
			selection: CatalogSelection{},
			expected:  []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contracts, err := SelectContracts(test.selection)
			if err != nil {
				t.Fatalf("SelectContracts returned error: %v", err)
			}
			actual := contractToolNames(contracts)
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("mismatch selected contracts (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestSelectContractsRejectsInvalidSelection(t *testing.T) {
	tests := []struct {
		name      string
		selection CatalogSelection
		field     string
	}{
		{
			name: "invalid scope",
			selection: CatalogSelection{
				Scope: Scope(99),
			},
			field: "scope",
		},
		{
			name: "unknown available tool",
			selection: CatalogSelection{
				Available: []string{"turn_begin"},
			},
			field: "available",
		},
		{
			name: "duplicate available tool",
			selection: CatalogSelection{
				Available: []string{HelpToolName, HelpToolName},
			},
			field: "available",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contracts, err := SelectContracts(test.selection)
			if err == nil {
				t.Fatal("SelectContracts error = nil, expected error")
			}
			if contracts != nil {
				t.Errorf("SelectContracts result = %#v, expected nil", contracts)
			}
			var actual *result.Error
			if !errors.As(err, &actual) {
				t.Fatalf("SelectContracts error type = %T, expected *result.Error", err)
			}
			expected := test.field
			if diff := cmp.Diff(expected, actual.Field); diff != "" {
				t.Errorf("mismatch error field (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestContractAnnotationsMatchToolWideBehavior(t *testing.T) {
	type annotationState struct {
		ReadOnly    bool
		Destructive bool
		Idempotent  bool
		OpenWorld   bool
	}
	tests := []struct {
		tool     string
		expected annotationState
	}{
		{
			tool: HelpToolName,
			expected: annotationState{
				ReadOnly:   true,
				Idempotent: true,
			},
		},
		{
			tool: StatusToolName,
			expected: annotationState{
				ReadOnly:   true,
				Idempotent: true,
			},
		},
		{
			tool: WorktreeToolName,
			expected: annotationState{
				Destructive: true,
			},
		},
		{
			tool: TurnToolName,
			expected: annotationState{
				Destructive: true,
			},
		},
		{
			tool: DiffToolName,
			expected: annotationState{
				ReadOnly:   true,
				Idempotent: true,
			},
		},
		{
			tool: ShellToolName,
			expected: annotationState{
				Destructive: true,
				OpenWorld:   true,
			},
		},
		{
			tool: ApplyPatchToolName,
			expected: annotationState{
				Destructive: true,
			},
		},
		{
			tool: ProcessToolName,
			expected: annotationState{
				Destructive: true,
				OpenWorld:   true,
			},
		},
		{
			tool: SkillActivateToolName,
			expected: annotationState{
				ReadOnly:   true,
				Idempotent: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.tool, func(t *testing.T) {
			contract, err := Contract(test.tool, TurnScope)
			if err != nil {
				t.Fatalf("Contract returned error: %v", err)
			}
			annotations := contract.Annotations
			for field, value := range map[string]*bool{
				"readOnlyHint":    annotations.ReadOnlyHint,
				"destructiveHint": annotations.DestructiveHint,
				"idempotentHint":  annotations.IdempotentHint,
				"openWorldHint":   annotations.OpenWorldHint,
			} {
				if value == nil {
					t.Fatalf("annotation %s is nil", field)
				}
			}
			actual := annotationState{
				ReadOnly:    *annotations.ReadOnlyHint,
				Destructive: *annotations.DestructiveHint,
				Idempotent:  *annotations.IdempotentHint,
				OpenWorld:   *annotations.OpenWorldHint,
			}
			if diff := cmp.Diff(test.expected, actual); diff != "" {
				t.Errorf("mismatch annotations (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestContractAnnotationsAreFresh(t *testing.T) {
	first, err := Contract(HelpToolName, TurnScope)
	if err != nil {
		t.Fatalf("first Contract returned error: %v", err)
	}
	second, err := Contract(HelpToolName, TurnScope)
	if err != nil {
		t.Fatalf("second Contract returned error: %v", err)
	}
	*first.Annotations.ReadOnlyHint = false
	if !*second.Annotations.ReadOnlyHint {
		t.Error("mutating one contract changed another contract's annotations")
	}
}

func TestEffectiveCatalogStaysWithinProtocolBudget(t *testing.T) {
	// Full output schemas are deliberately omitted, so this is the permanent
	// ceiling for the complete common tools/list payload: names, descriptions,
	// input schemas, and annotations.
	const maxCatalogBytes = 8 * 1024

	contracts, err := SelectContracts(CatalogSelection{
		Available:       ContractNames(),
		SkillsAvailable: true,
	})
	if err != nil {
		t.Fatalf("SelectContracts returned error: %v", err)
	}
	data, err := json.Marshal(struct {
		Tools []mcp.Tool `json:"tools"`
	}{
		Tools: contracts,
	})
	if err != nil {
		t.Fatalf("marshal effective catalog: %v", err)
	}
	if len(data) > maxCatalogBytes {
		t.Errorf("serialized effective catalog bytes = %d, exceeds budget %d", len(data), maxCatalogBytes)
	}
}

func TestEffectiveCatalogSnapshots(t *testing.T) {
	tests := []struct {
		name      string
		selection CatalogSelection
	}{
		{
			name: "turn",
			selection: CatalogSelection{
				Available:       ContractNames(),
				SkillsAvailable: true,
			},
		},
		{
			name: "freestyle",
			selection: CatalogSelection{
				Scope:           FreestyleScope,
				Available:       ContractNames(),
				SkillsAvailable: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contracts, err := SelectContracts(test.selection)
			if err != nil {
				t.Fatalf("SelectContracts returned error: %v", err)
			}
			assertJSONSnapshot(t, filepath.Join("testdata", "catalog", test.name+".json"), struct {
				Tools []mcp.Tool `json:"tools"`
			}{
				Tools: contracts,
			})
		})
	}
}

func contractToolNames(contracts []mcp.Tool) []string {
	names := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		names = append(names, contract.Name)
	}
	return names
}

func assertJSONSnapshot(t *testing.T, path string, value any) {
	t.Helper()
	actual, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	actual = append(actual, '\n')
	if os.Getenv("UPDATE_OPENRIG_CONTRACTS") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create snapshot directory: %v", err)
		}
		if err := os.WriteFile(path, actual, 0o600); err != nil {
			t.Fatalf("write snapshot: %v", err)
		}
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if diff := cmp.Diff(string(expected), string(actual)); diff != "" {
		t.Errorf("mismatch snapshot (-expected, +actual):\n%s", diff)
	}
}

func TestCatalogNamesAreUniqueAndConstructible(t *testing.T) {
	names := make([]string, 0, len(nativeCatalog))
	for _, entry := range nativeCatalog {
		names = append(names, entry.name)
		if _, err := Contract(entry.name, TurnScope); err != nil {
			t.Errorf("Contract(%q) returned error: %v", entry.name, err)
		}
	}

	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	for index := 1; index < len(sorted); index++ {
		if sorted[index] == sorted[index-1] {
			t.Errorf("duplicate catalog name %q", sorted[index])
		}
	}
}
