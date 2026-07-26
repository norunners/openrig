package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/norunners/openrig/internal/result"
)

// CatalogSelection describes the native handlers and conditional capabilities
// available to one runtime. Available must contain only complete handlers.
type CatalogSelection struct {
	Scope           Scope
	Available       []string
	SkillsAvailable bool
}

type catalogEntry struct {
	name           string
	turnScopeOnly  bool
	requiresSkills bool
	annotations    annotationPolicy
}

type annotationPolicy struct {
	readOnly    bool
	destructive bool
	idempotent  bool
	openWorld   bool
}

var nativeCatalog = []catalogEntry{
	{
		name: HelpToolName,
		annotations: annotationPolicy{
			readOnly:   true,
			idempotent: true,
		},
	},
	{
		name:          StatusToolName,
		turnScopeOnly: true,
		annotations: annotationPolicy{
			readOnly:   true,
			idempotent: true,
		},
	},
	{
		name:          WorktreeToolName,
		turnScopeOnly: true,
		annotations: annotationPolicy{
			destructive: true,
		},
	},
	{
		name:          TurnToolName,
		turnScopeOnly: true,
		annotations: annotationPolicy{
			destructive: true,
		},
	},
	{
		name:          DiffToolName,
		turnScopeOnly: true,
		annotations: annotationPolicy{
			readOnly:   true,
			idempotent: true,
		},
	},
	{
		name: ShellToolName,
		annotations: annotationPolicy{
			destructive: true,
			openWorld:   true,
		},
	},
	{
		name: ApplyPatchToolName,
		annotations: annotationPolicy{
			destructive: true,
		},
	},
	{
		name: ProcessToolName,
		annotations: annotationPolicy{
			destructive: true,
			openWorld:   true,
		},
	},
	{
		name:           SkillActivateToolName,
		requiresSkills: true,
		annotations: annotationPolicy{
			readOnly:   true,
			idempotent: true,
		},
	},
}

// SelectContracts returns the effective native catalog in stable order.
//
// A contract is selected only when its handler is explicitly available, its
// scope is valid for the runtime, and its conditional capability is present.
// No partial catalog is returned for invalid input.
func SelectContracts(selection CatalogSelection) ([]mcp.Tool, error) {
	if err := validateScope(selection.Scope); err != nil {
		return nil, err
	}
	available, err := catalogNameSet("available", selection.Available)
	if err != nil {
		return nil, err
	}

	contracts := make([]mcp.Tool, 0, len(nativeCatalog))
	for _, entry := range nativeCatalog {
		if !available[entry.name] || !entry.available(selection) {
			continue
		}
		contract, err := Contract(entry.name, selection.Scope)
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, *contract)
	}
	return contracts, nil
}

func catalogEntryByName(name string) (catalogEntry, bool) {
	for _, entry := range nativeCatalog {
		if entry.name == name {
			return entry, true
		}
	}
	return catalogEntry{}, false
}

func catalogNameSet(field string, names []string) (map[string]bool, error) {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		if _, ok := catalogEntryByName(name); !ok {
			return nil, result.NewError(result.CodeInvalidArgument, field+" contains an unknown OpenRig tool").
				WithField(field).
				WithSuggestion("use one of: " + joinFields(ContractNames()))
		}
		if set[name] {
			return nil, result.NewError(result.CodeInvalidArgument, field+" contains a duplicate OpenRig tool").
				WithField(field)
		}
		set[name] = true
	}
	return set, nil
}

func (entry catalogEntry) available(selection CatalogSelection) bool {
	if entry.turnScopeOnly && selection.Scope != TurnScope {
		return false
	}
	if entry.requiresSkills && !selection.SkillsAvailable {
		return false
	}
	return true
}

func applyCatalogAnnotations(tool *mcp.Tool) {
	entry, ok := catalogEntryByName(tool.Name)
	if !ok {
		return
	}
	tool.Annotations = entry.annotations.annotation()
}

func (policy annotationPolicy) annotation() mcp.ToolAnnotation {
	return mcp.ToolAnnotation{
		ReadOnlyHint:    mcp.ToBoolPtr(policy.readOnly),
		DestructiveHint: mcp.ToBoolPtr(policy.destructive),
		IdempotentHint:  mcp.ToBoolPtr(policy.idempotent),
		OpenWorldHint:   mcp.ToBoolPtr(policy.openWorld),
	}
}

func catalogToolNames(contracts []mcp.Tool) map[string]bool {
	names := make(map[string]bool, len(contracts))
	for _, contract := range contracts {
		names[contract.Name] = true
	}
	return names
}
