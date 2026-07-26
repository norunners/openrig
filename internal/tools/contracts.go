// Package tools defines the public MCP contracts for OpenRig-owned tools.
//
// Contract definition is intentionally separate from handler registration.
// A tool must have a complete implementation before a runtime advertises it.
package tools

import (
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/norunners/openrig/internal/result"
)

const (
	HelpToolName          = "help"
	StatusToolName        = "status"
	WorktreeToolName      = "worktree"
	TurnToolName          = "turn"
	DiffToolName          = "diff"
	ShellToolName         = "shell"
	ApplyPatchToolName    = "apply_patch"
	ProcessToolName       = "process"
	SkillActivateToolName = "skill_activate"
)

// Scope selects the identity field used by normal work tools.
//
// The zero value is TurnScope because turn-scoped execution is OpenRig's
// default agent mode.
type Scope uint8

const (
	TurnScope Scope = iota
	FreestyleScope
)

// ContractNames returns the complete OpenRig-owned native tool surface.
//
// Availability and registration are separate runtime concerns. For example,
// skill_activate is registered only when skill activation is available.
func ContractNames() []string {
	names := make([]string, 0, len(nativeCatalog))
	for _, entry := range nativeCatalog {
		names = append(names, entry.name)
	}
	return names
}

// Contracts returns fresh MCP definitions for the complete native surface.
// It returns no partial definitions when scope is invalid.
func Contracts(scope Scope) ([]mcp.Tool, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	names := ContractNames()
	contracts := make([]mcp.Tool, 0, len(names))
	for _, name := range names {
		contract, err := Contract(name, scope)
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, *contract)
	}
	return contracts, nil
}

// Contract returns a fresh MCP definition for name.
// It returns nil when name or scope is invalid.
func Contract(name string, scope Scope) (*mcp.Tool, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	var (
		contract *mcp.Tool
		err      error
	)
	switch name {
	case HelpToolName:
		contract = helpContract()
	case StatusToolName:
		contract = statusContract()
	case WorktreeToolName:
		contract = worktreeContract()
	case TurnToolName:
		contract = turnContract()
	case DiffToolName:
		contract = diffContract()
	case ShellToolName:
		contract, err = shellContract(scope)
	case ApplyPatchToolName:
		contract, err = applyPatchContract(scope)
	case ProcessToolName:
		contract, err = processContract(scope)
	case SkillActivateToolName:
		contract, err = skillActivateContract(scope)
	default:
		return nil, result.NewError(result.CodeInvalidArgument, "unknown OpenRig tool").
			WithField("tool").
			WithSuggestion("use one of: " + joinFields(ContractNames()))
	}
	if err != nil {
		return nil, err
	}
	applyCatalogAnnotations(contract)
	return contract, nil
}

func helpContract() *mcp.Tool {
	return newContract(
		HelpToolName,
		"Explain OpenRig's common, advanced, recovery, and freestyle workflows.",
		mcp.WithString("topic",
			mcp.Description("Optional guidance topic."),
			mcp.Enum("common", "advanced", "freestyle", "worktree", "turn", "diff", "status", "ids", "errors", "schemas"),
		),
	)
}

func statusContract() *mcp.Tool {
	return newContract(
		StatusToolName,
		"Inspect current-session readiness, active work, and health when recovering or diagnosing OpenRig.",
	)
}

func worktreeContract() *mcp.Tool {
	return newContract(
		WorktreeToolName,
		"Manage isolated Git worktrees. List returns a runtime-bounded snapshot of recent matches in deterministic order.",
		mcp.WithString("op",
			mcp.Required(),
			mcp.Description("Operation to perform."),
			mcp.Enum("open", "list", "status", "close", "delete"),
		),
		mcp.WithString("worktree_id", mcp.Description("Worktree ID required by status, close, and delete.")),
		mcp.WithString("repo", mcp.Description("Repository selector for open or list: configured name, alias, or allowed path.")),
		mcp.WithString("base",
			mcp.Description("Optional Git base for open. Omitted means HEAD."),
		),
		mcp.WithString("branch", mcp.Description("Optional new branch name for open. Omit to create a detached worktree.")),
		mcp.WithString("reason", mcp.Description("Optional concise audit reason recorded with open.")),
		mcp.WithString("state",
			mcp.Description("Optional list filter."),
			mcp.Enum("open", "closed", "deleted", "any"),
		),
	)
}

func turnContract() *mcp.Tool {
	return newContract(
		TurnToolName,
		"Begin, inspect, or end scoped work. Status returns a runtime-bounded snapshot of recent matches in deterministic order.",
		mcp.WithString("op",
			mcp.Required(),
			mcp.Description("Operation to perform."),
			mcp.Enum("status", "begin", "end"),
		),
		mcp.WithString("mode",
			mcp.Description("Optional begin or status mode. Begin defaults to an isolated worktree; use freestyle only as an escape hatch."),
			mcp.Enum("worktree", "freestyle"),
		),
		mcp.WithString("turn_id", mcp.Description("Turn ID required by end or used as an exact status selector.")),
		mcp.WithString("worktree_id", mcp.Description("Optional existing worktree to reuse for begin, or a status filter. Mutually exclusive with repo for begin.")),
		mcp.WithString("repo", mcp.Description("Repository selector for begin or status. Worktree-mode begin creates an isolated worktree automatically.")),
		mcp.WithString("goal", mcp.Description("Concise goal required by begin.")),
		mcp.WithString("outcome",
			mcp.Description("Required terminal outcome for end."),
			mcp.Enum("completed", "blocked", "abandoned", "design_only", "verification_only"),
		),
		mcp.WithString("summary", mcp.Description("Optional concise summary recorded with end.")),
		mcp.WithString("state",
			mcp.Description("Optional status filter."),
			mcp.Enum("active", "ended", "any"),
		),
	)
}

func diffContract() *mcp.Tool {
	return newContract(
		DiffToolName,
		"Render a bounded diff. Supplying only turn_id compares the turn's start with its current or persisted end state.",
		mcp.WithString("kind",
			mcp.Description("Optional advanced diff kind. Omit for the common turn_id operation."),
			mcp.Enum("worktree", "git", "revision"),
		),
		mcp.WithString("turn_id", mcp.Description("Turn to diff from its starting revision to current or persisted end state, or a scope for an advanced Git diff.")),
		mcp.WithString("worktree_id", mcp.Description("Worktree selector for worktree or Git diffs.")),
		mcp.WithString("from", mcp.Description("Starting Git ref or revision ID when required by the selected kind.")),
		mcp.WithString("to", mcp.Description("Ending Git ref or revision ID when required by the selected kind.")),
		mcp.WithArray("paths",
			mcp.Description("Optional repository-relative path filters."),
			mcp.WithStringItems(mcp.MinLength(1)),
			mcp.UniqueItems(true),
		),
		mcp.WithBoolean("stat",
			mcp.Description("Return summary statistics instead of a unified patch."),
			mcp.DefaultBool(false),
		),
	)
}

func shellContract(scope Scope) (*mcp.Tool, error) {
	return newScopedContract(
		scope,
		ShellToolName,
		"Run one bounded shell command in the scoped repository.",
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("Shell script to run in the configured shell."),
		),
		mcp.WithString("workdir",
			mcp.Description("Optional working directory relative to the scoped workspace root. Defaults to that root."),
			mcp.MinLength(1),
		),
	)
}

func applyPatchContract(scope Scope) (*mcp.Tool, error) {
	return newScopedContract(
		scope,
		ApplyPatchToolName,
		"Apply a structured patch in the scoped repository.",
		mcp.WithString("patch",
			mcp.Required(),
			mcp.Description("Complete patch text in the supported patch grammar."),
		),
	)
}

func processContract(scope Scope) (*mcp.Tool, error) {
	return newScopedContract(
		scope,
		ProcessToolName,
		"Manage supervised processes. Status returns a runtime-bounded snapshot of recent matches in deterministic order.",
		mcp.WithString("op",
			mcp.Required(),
			mcp.Description("Operation to perform."),
			mcp.Enum("start", "status", "read", "stop", "restart", "kill"),
		),
		mcp.WithString("process_id", mcp.Description("Process ID required by read, stop, restart, and kill or used as an exact status selector.")),
		mcp.WithString("command", mcp.Description("Shell command required by start.")),
		mcp.WithString("workdir",
			mcp.Description("Optional start working directory relative to the scoped workspace root. Defaults to that root."),
			mcp.MinLength(1),
		),
		mcp.WithObject("env",
			mcp.Description("Optional environment variables for start."),
			mcp.AdditionalProperties(&jsonschema.Schema{Type: "string"}),
		),
		mcp.WithString("cursor", mcp.Description("Optional read cursor returned by a previous read.")),
		mcp.WithString("state",
			mcp.Description("Optional status filter."),
			mcp.Enum("running", "exited", "stopped", "killed", "restarting", "any"),
		),
	)
}

func skillActivateContract(scope Scope) (*mcp.Tool, error) {
	return newScopedContract(
		scope,
		SkillActivateToolName,
		"Load instructions for one local agent skill. Scripts and references are returned as text and never executed.",
		mcp.WithString("skill",
			mcp.Required(),
			mcp.Description("Skill directory name."),
			mcp.Pattern(`^[a-z0-9]+(?:-[a-z0-9]+)*$`),
			mcp.MaxLength(64),
		),
		mcp.WithBoolean("include_scripts",
			mcp.Description("Include bounded script file contents."),
			mcp.DefaultBool(false),
		),
		mcp.WithBoolean("include_references",
			mcp.Description("Include bounded reference file contents."),
			mcp.DefaultBool(false),
		),
	)
}

func newContract(name, description string, options ...mcp.ToolOption) *mcp.Tool {
	options = append([]mcp.ToolOption{mcp.WithDescription(description)}, options...)
	tool := mcp.NewTool(name, options...)
	tool.Annotations = mcp.ToolAnnotation{}
	tool.InputSchema.AdditionalProperties = false
	return &tool
}

func newScopedContract(scope Scope, name, description string, options ...mcp.ToolOption) (*mcp.Tool, error) {
	identityOption, err := scopeOption(scope)
	if err != nil {
		return nil, err
	}
	options = append([]mcp.ToolOption{identityOption}, options...)
	return newContract(name, description, options...), nil
}

func scopeOption(scope Scope) (mcp.ToolOption, error) {
	switch scope {
	case TurnScope:
		return mcp.WithString("turn_id",
			mcp.Required(),
			mcp.Description("Active turn ID that scopes this operation."),
		), nil
	case FreestyleScope:
		return mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Configured repository name, alias, or allowed path."),
		), nil
	default:
		return nil, result.NewError(result.CodeInvalidArgument, "unsupported tool scope").
			WithField("scope").
			WithSuggestion("use TurnScope or FreestyleScope")
	}
}

func validateScope(scope Scope) error {
	switch scope {
	case TurnScope, FreestyleScope:
		return nil
	default:
		return result.NewError(result.CodeInvalidArgument, "unsupported tool scope").
			WithField("scope").
			WithSuggestion("use TurnScope or FreestyleScope")
	}
}

func joinFields(fields []string) string {
	fields = append([]string(nil), fields...)
	sort.Strings(fields)
	return strings.Join(fields, ", ")
}
