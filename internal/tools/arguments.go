package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/norunners/openrig/internal/result"
)

type TurnID string
type WorktreeID string
type ProcessID string
type RevisionID string

type WorktreeOp string
type WorktreeState string
type WorktreeStateFilter string
type TurnOp string
type TurnMode string
type TurnState string
type TurnStateFilter string
type TurnOutcome string
type DiffKind string
type ProcessOp string
type ProcessState string
type ProcessStateFilter string

const (
	WorktreeOpOpen   WorktreeOp = "open"
	WorktreeOpList   WorktreeOp = "list"
	WorktreeOpStatus WorktreeOp = "status"
	WorktreeOpClose  WorktreeOp = "close"
	WorktreeOpDelete WorktreeOp = "delete"

	WorktreeStateOpen    WorktreeState = "open"
	WorktreeStateClosed  WorktreeState = "closed"
	WorktreeStateDeleted WorktreeState = "deleted"

	WorktreeStateFilterOpen    WorktreeStateFilter = "open"
	WorktreeStateFilterClosed  WorktreeStateFilter = "closed"
	WorktreeStateFilterDeleted WorktreeStateFilter = "deleted"
	WorktreeStateFilterAny     WorktreeStateFilter = "any"

	TurnOpStatus TurnOp = "status"
	TurnOpBegin  TurnOp = "begin"
	TurnOpEnd    TurnOp = "end"

	TurnModeWorktree  TurnMode = "worktree"
	TurnModeFreestyle TurnMode = "freestyle"

	TurnStateActive TurnState = "active"
	TurnStateEnded  TurnState = "ended"

	TurnStateFilterActive TurnStateFilter = "active"
	TurnStateFilterEnded  TurnStateFilter = "ended"
	TurnStateFilterAny    TurnStateFilter = "any"

	TurnOutcomeCompleted        TurnOutcome = "completed"
	TurnOutcomeBlocked          TurnOutcome = "blocked"
	TurnOutcomeAbandoned        TurnOutcome = "abandoned"
	TurnOutcomeDesignOnly       TurnOutcome = "design_only"
	TurnOutcomeVerificationOnly TurnOutcome = "verification_only"

	DiffKindTurn     DiffKind = "turn"
	DiffKindWorktree DiffKind = "worktree"
	DiffKindGit      DiffKind = "git"
	DiffKindRevision DiffKind = "revision"

	ProcessOpStart   ProcessOp = "start"
	ProcessOpStatus  ProcessOp = "status"
	ProcessOpRead    ProcessOp = "read"
	ProcessOpStop    ProcessOp = "stop"
	ProcessOpRestart ProcessOp = "restart"
	ProcessOpKill    ProcessOp = "kill"

	ProcessStateRunning    ProcessState = "running"
	ProcessStateExited     ProcessState = "exited"
	ProcessStateStopped    ProcessState = "stopped"
	ProcessStateKilled     ProcessState = "killed"
	ProcessStateRestarting ProcessState = "restarting"

	ProcessStateFilterRunning    ProcessStateFilter = "running"
	ProcessStateFilterExited     ProcessStateFilter = "exited"
	ProcessStateFilterStopped    ProcessStateFilter = "stopped"
	ProcessStateFilterKilled     ProcessStateFilter = "killed"
	ProcessStateFilterRestarting ProcessStateFilter = "restarting"
	ProcessStateFilterAny        ProcessStateFilter = "any"

	helpTopicCommon    = "common"
	helpTopicAdvanced  = "advanced"
	helpTopicFreestyle = "freestyle"
	helpTopicWorktree  = "worktree"
	helpTopicTurn      = "turn"
	helpTopicDiff      = "diff"
	helpTopicStatus    = "status"
	helpTopicIDs       = "ids"
	helpTopicErrors    = "errors"
	helpTopicSchemas   = "schemas"
)

var (
	helpTopics = []string{
		helpTopicCommon,
		helpTopicAdvanced,
		helpTopicFreestyle,
		helpTopicWorktree,
		helpTopicTurn,
		helpTopicDiff,
		helpTopicStatus,
		helpTopicIDs,
		helpTopicErrors,
		helpTopicSchemas,
	}
	worktreeOps = []WorktreeOp{
		WorktreeOpOpen,
		WorktreeOpList,
		WorktreeOpStatus,
		WorktreeOpClose,
		WorktreeOpDelete,
	}
	worktreeStateFilters = []WorktreeStateFilter{
		WorktreeStateFilterOpen,
		WorktreeStateFilterClosed,
		WorktreeStateFilterDeleted,
		WorktreeStateFilterAny,
	}
	turnOps = []TurnOp{
		TurnOpStatus,
		TurnOpBegin,
		TurnOpEnd,
	}
	turnModes = []TurnMode{
		TurnModeWorktree,
		TurnModeFreestyle,
	}
	turnStateFilters = []TurnStateFilter{
		TurnStateFilterActive,
		TurnStateFilterEnded,
		TurnStateFilterAny,
	}
	turnOutcomes = []TurnOutcome{
		TurnOutcomeCompleted,
		TurnOutcomeBlocked,
		TurnOutcomeAbandoned,
		TurnOutcomeDesignOnly,
		TurnOutcomeVerificationOnly,
	}
	diffInputKinds = []DiffKind{
		DiffKindWorktree,
		DiffKindGit,
		DiffKindRevision,
	}
	processOps = []ProcessOp{
		ProcessOpStart,
		ProcessOpStatus,
		ProcessOpRead,
		ProcessOpStop,
		ProcessOpRestart,
		ProcessOpKill,
	}
	processStateFilters = []ProcessStateFilter{
		ProcessStateFilterRunning,
		ProcessStateFilterExited,
		ProcessStateFilterStopped,
		ProcessStateFilterKilled,
		ProcessStateFilterRestarting,
		ProcessStateFilterAny,
	}
)

// toolArguments seals the concrete arguments accepted by OpenRig's native
// handlers. Values cross the MCP boundary after mcp-go normalizes them as any,
// then immediately become one of these operation-specific types.
type toolArguments interface {
	isToolArguments()
}

type helpArguments struct {
	Topic string `json:"topic,omitempty"`
}

type statusArguments struct{}

type worktreeOpenArguments struct {
	Op     WorktreeOp `json:"op"`
	Repo   string     `json:"repo"`
	Base   string     `json:"base,omitempty"`
	Branch string     `json:"branch,omitempty"`
	Reason string     `json:"reason,omitempty"`
}

type worktreeListArguments struct {
	Op    WorktreeOp          `json:"op"`
	Repo  string              `json:"repo,omitempty"`
	State WorktreeStateFilter `json:"state,omitempty"`
}

type worktreeStatusArguments struct {
	Op         WorktreeOp `json:"op"`
	WorktreeID WorktreeID `json:"worktree_id"`
}

type worktreeCloseArguments struct {
	Op         WorktreeOp `json:"op"`
	WorktreeID WorktreeID `json:"worktree_id"`
}

type worktreeDeleteArguments struct {
	Op         WorktreeOp `json:"op"`
	WorktreeID WorktreeID `json:"worktree_id"`
}

type turnBeginArguments struct {
	Op         TurnOp     `json:"op"`
	Mode       TurnMode   `json:"mode,omitempty"`
	Repo       string     `json:"repo,omitempty"`
	WorktreeID WorktreeID `json:"worktree_id,omitempty"`
	Goal       string     `json:"goal"`
}

type turnStatusArguments struct {
	Op         TurnOp          `json:"op"`
	Mode       TurnMode        `json:"mode,omitempty"`
	TurnID     TurnID          `json:"turn_id,omitempty"`
	WorktreeID WorktreeID      `json:"worktree_id,omitempty"`
	Repo       string          `json:"repo,omitempty"`
	State      TurnStateFilter `json:"state,omitempty"`
}

type turnEndArguments struct {
	Op      TurnOp      `json:"op"`
	TurnID  TurnID      `json:"turn_id"`
	Outcome TurnOutcome `json:"outcome"`
	Summary string      `json:"summary,omitempty"`
}

type diffTurnArguments struct {
	TurnID TurnID   `json:"turn_id"`
	Paths  []string `json:"paths,omitempty"`
	Stat   bool     `json:"stat,omitempty"`
}

type diffWorktreeArguments struct {
	Kind       DiffKind   `json:"kind"`
	WorktreeID WorktreeID `json:"worktree_id"`
	From       string     `json:"from,omitempty"`
	Paths      []string   `json:"paths,omitempty"`
	Stat       bool       `json:"stat,omitempty"`
}

type diffGitArguments struct {
	Kind       DiffKind   `json:"kind"`
	TurnID     TurnID     `json:"turn_id,omitempty"`
	WorktreeID WorktreeID `json:"worktree_id,omitempty"`
	From       string     `json:"from,omitempty"`
	To         string     `json:"to,omitempty"`
	Paths      []string   `json:"paths,omitempty"`
	Stat       bool       `json:"stat,omitempty"`
}

type diffRevisionArguments struct {
	Kind  DiffKind   `json:"kind"`
	From  RevisionID `json:"from"`
	To    RevisionID `json:"to"`
	Paths []string   `json:"paths,omitempty"`
	Stat  bool       `json:"stat,omitempty"`
}

type scopeArguments struct {
	TurnID TurnID `json:"turn_id,omitempty"`
	Repo   string `json:"repo,omitempty"`
}

type shellArguments struct {
	scopeArguments
	Command string `json:"command"`
	Workdir string `json:"workdir,omitempty"`
}

type applyPatchArguments struct {
	scopeArguments
	Patch string `json:"patch"`
}

type processStartArguments struct {
	scopeArguments
	Op      ProcessOp         `json:"op"`
	Command string            `json:"command"`
	Workdir string            `json:"workdir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type processStatusArguments struct {
	scopeArguments
	Op        ProcessOp          `json:"op"`
	ProcessID ProcessID          `json:"process_id,omitempty"`
	State     ProcessStateFilter `json:"state,omitempty"`
}

type processReadArguments struct {
	scopeArguments
	Op        ProcessOp `json:"op"`
	ProcessID ProcessID `json:"process_id"`
	Cursor    string    `json:"cursor,omitempty"`
}

type processStopArguments struct {
	scopeArguments
	Op        ProcessOp `json:"op"`
	ProcessID ProcessID `json:"process_id"`
}

type processRestartArguments struct {
	scopeArguments
	Op        ProcessOp `json:"op"`
	ProcessID ProcessID `json:"process_id"`
}

type processKillArguments struct {
	scopeArguments
	Op        ProcessOp `json:"op"`
	ProcessID ProcessID `json:"process_id"`
}

type skillActivateArguments struct {
	scopeArguments
	Skill             string `json:"skill"`
	IncludeScripts    bool   `json:"include_scripts,omitempty"`
	IncludeReferences bool   `json:"include_references,omitempty"`
}

func (helpArguments) isToolArguments()           {}
func (statusArguments) isToolArguments()         {}
func (worktreeOpenArguments) isToolArguments()   {}
func (worktreeListArguments) isToolArguments()   {}
func (worktreeStatusArguments) isToolArguments() {}
func (worktreeCloseArguments) isToolArguments()  {}
func (worktreeDeleteArguments) isToolArguments() {}
func (turnBeginArguments) isToolArguments()      {}
func (turnStatusArguments) isToolArguments()     {}
func (turnEndArguments) isToolArguments()        {}
func (diffTurnArguments) isToolArguments()       {}
func (diffWorktreeArguments) isToolArguments()   {}
func (diffGitArguments) isToolArguments()        {}
func (diffRevisionArguments) isToolArguments()   {}
func (shellArguments) isToolArguments()          {}
func (applyPatchArguments) isToolArguments()     {}
func (processStartArguments) isToolArguments()   {}
func (processStatusArguments) isToolArguments()  {}
func (processReadArguments) isToolArguments()    {}
func (processStopArguments) isToolArguments()    {}
func (processRestartArguments) isToolArguments() {}
func (processKillArguments) isToolArguments()    {}
func (skillActivateArguments) isToolArguments()  {}

type fieldSet map[string]json.RawMessage

// decodeArguments converts the third-party MCP argument value into one exact,
// operation-specific type. It never returns a partially decoded value.
func decodeArguments(toolName string, scope Scope, arguments any) (toolArguments, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	raw, err := argumentJSON(arguments)
	if err != nil {
		return nil, err
	}
	switch toolName {
	case HelpToolName:
		return decodeHelpArguments(raw)
	case StatusToolName:
		return decodeStatusArguments(raw)
	case WorktreeToolName:
		return decodeWorktreeArguments(raw)
	case TurnToolName:
		return decodeTurnArguments(raw)
	case DiffToolName:
		return decodeDiffArguments(raw)
	case ShellToolName:
		return decodeShellArguments(scope, raw)
	case ApplyPatchToolName:
		return decodeApplyPatchArguments(scope, raw)
	case ProcessToolName:
		return decodeProcessArguments(scope, raw)
	case SkillActivateToolName:
		return decodeSkillActivateArguments(scope, raw)
	default:
		return nil, result.NewError(result.CodeInvalidArgument, "unknown OpenRig tool").
			WithField("tool").
			WithSuggestion("use one of: " + joinFields(ContractNames()))
	}
}

func decodeHelpArguments(raw json.RawMessage) (toolArguments, error) {
	arguments, fields, err := decodeStrict[helpArguments](raw, HelpToolName)
	if err != nil {
		return nil, err
	}
	if fields.has("topic") && !contains(helpTopics, arguments.Topic) {
		return nil, enumError("topic", helpTopics)
	}
	return arguments, nil
}

func decodeStatusArguments(raw json.RawMessage) (toolArguments, error) {
	arguments, _, err := decodeStrict[statusArguments](raw, StatusToolName)
	if err != nil {
		return nil, err
	}
	return arguments, nil
}

func decodeWorktreeArguments(raw json.RawMessage) (toolArguments, error) {
	op, err := requiredEnum(raw, "op", worktreeOps)
	if err != nil {
		return nil, err
	}
	switch op {
	case WorktreeOpOpen:
		arguments, fields, err := decodeStrict[worktreeOpenArguments](raw, "worktree op=open")
		if err != nil {
			return nil, err
		}
		if err := requiredText(fields, "repo", arguments.Repo); err != nil {
			return nil, err
		}
		if !fields.has("base") {
			arguments.Base = "HEAD"
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{name: "base", value: arguments.Base},
			{name: "branch", value: arguments.Branch},
			{name: "reason", value: arguments.Reason},
		} {
			if err := optionalText(fields, field.name, field.value); err != nil {
				return nil, err
			}
		}
		return arguments, nil
	case WorktreeOpList:
		arguments, fields, err := decodeStrict[worktreeListArguments](raw, "worktree op=list")
		if err != nil {
			return nil, err
		}
		if err := optionalText(fields, "repo", arguments.Repo); err != nil {
			return nil, err
		}
		if fields.has("state") && !contains(worktreeStateFilters, arguments.State) {
			return nil, enumError("state", stringsOf(worktreeStateFilters))
		}
		return arguments, nil
	case WorktreeOpStatus:
		arguments, fields, err := decodeStrict[worktreeStatusArguments](raw, "worktree op=status")
		if err != nil {
			return nil, err
		}
		if err := requiredText(fields, "worktree_id", string(arguments.WorktreeID)); err != nil {
			return nil, err
		}
		return arguments, nil
	case WorktreeOpClose:
		arguments, fields, err := decodeStrict[worktreeCloseArguments](raw, "worktree op=close")
		if err != nil {
			return nil, err
		}
		if err := requiredText(fields, "worktree_id", string(arguments.WorktreeID)); err != nil {
			return nil, err
		}
		return arguments, nil
	case WorktreeOpDelete:
		arguments, fields, err := decodeStrict[worktreeDeleteArguments](raw, "worktree op=delete")
		if err != nil {
			return nil, err
		}
		if err := requiredText(fields, "worktree_id", string(arguments.WorktreeID)); err != nil {
			return nil, err
		}
		return arguments, nil
	default:
		return nil, enumError("op", stringsOf(worktreeOps))
	}
}

func decodeTurnArguments(raw json.RawMessage) (toolArguments, error) {
	op, err := requiredEnum(raw, "op", turnOps)
	if err != nil {
		return nil, err
	}
	switch op {
	case TurnOpBegin:
		arguments, fields, err := decodeStrict[turnBeginArguments](raw, "turn op=begin")
		if err != nil {
			return nil, err
		}
		if !fields.has("mode") {
			arguments.Mode = TurnModeWorktree
		} else if !contains(turnModes, arguments.Mode) {
			return nil, enumError("mode", stringsOf(turnModes))
		}
		if err := requiredText(fields, "goal", arguments.Goal); err != nil {
			return nil, err
		}
		if arguments.Mode == TurnModeFreestyle {
			if fields.has("worktree_id") {
				return nil, invalidField("worktree_id", "turn op=begin mode=freestyle")
			}
			if err := requiredText(fields, "repo", arguments.Repo); err != nil {
				return nil, err
			}
			return arguments, nil
		}
		if err := optionalText(fields, "repo", arguments.Repo); err != nil {
			return nil, err
		}
		if err := optionalText(fields, "worktree_id", string(arguments.WorktreeID)); err != nil {
			return nil, err
		}
		selectors := presentFields(fields, "repo", "worktree_id")
		if len(selectors) == 0 {
			return nil, result.NewError(result.CodeInvalidArgument, "turn op=begin requires exactly one of repo or worktree_id").
				WithField("repo")
		}
		if len(selectors) > 1 {
			return nil, mutuallyExclusiveError(selectors...)
		}
		return arguments, nil
	case TurnOpStatus:
		arguments, fields, err := decodeStrict[turnStatusArguments](raw, "turn op=status")
		if err != nil {
			return nil, err
		}
		if fields.has("mode") && !contains(turnModes, arguments.Mode) {
			return nil, enumError("mode", stringsOf(turnModes))
		}
		if fields.has("state") && !contains(turnStateFilters, arguments.State) {
			return nil, enumError("state", stringsOf(turnStateFilters))
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{name: "turn_id", value: string(arguments.TurnID)},
			{name: "worktree_id", value: string(arguments.WorktreeID)},
			{name: "repo", value: arguments.Repo},
		} {
			if err := optionalText(fields, field.name, field.value); err != nil {
				return nil, err
			}
		}
		selectors := presentFields(fields, "turn_id", "worktree_id", "repo")
		if len(selectors) > 1 {
			return nil, mutuallyExclusiveError(selectors...)
		}
		if fields.has("turn_id") {
			for _, field := range []string{"mode", "state"} {
				if fields.has(field) {
					return nil, invalidField(field, "turn op=status with turn_id")
				}
			}
		}
		return arguments, nil
	case TurnOpEnd:
		arguments, fields, err := decodeStrict[turnEndArguments](raw, "turn op=end")
		if err != nil {
			return nil, err
		}
		if err := requiredText(fields, "turn_id", string(arguments.TurnID)); err != nil {
			return nil, err
		}
		if !fields.has("outcome") || !contains(turnOutcomes, arguments.Outcome) {
			if !fields.has("outcome") {
				return nil, requiredFieldError("outcome")
			}
			return nil, enumError("outcome", stringsOf(turnOutcomes))
		}
		if err := optionalText(fields, "summary", arguments.Summary); err != nil {
			return nil, err
		}
		return arguments, nil
	default:
		return nil, enumError("op", stringsOf(turnOps))
	}
}

func decodeDiffArguments(raw json.RawMessage) (toolArguments, error) {
	kind, present, err := optionalEnum(raw, "kind", diffInputKinds)
	if err != nil {
		return nil, err
	}
	if !present {
		arguments, fields, err := decodeStrict[diffTurnArguments](raw, "diff by turn_id")
		if err != nil {
			return nil, err
		}
		if err := requiredText(fields, "turn_id", string(arguments.TurnID)); err != nil {
			return nil, err
		}
		if err := validatePaths(arguments.Paths); err != nil {
			return nil, err
		}
		return arguments, nil
	}
	switch kind {
	case DiffKindWorktree:
		arguments, fields, err := decodeStrict[diffWorktreeArguments](raw, "diff kind=worktree")
		if err != nil {
			return nil, err
		}
		if err := requiredText(fields, "worktree_id", string(arguments.WorktreeID)); err != nil {
			return nil, err
		}
		if err := optionalText(fields, "from", arguments.From); err != nil {
			return nil, err
		}
		if err := validatePaths(arguments.Paths); err != nil {
			return nil, err
		}
		return arguments, nil
	case DiffKindGit:
		arguments, fields, err := decodeStrict[diffGitArguments](raw, "diff kind=git")
		if err != nil {
			return nil, err
		}
		for _, field := range []struct {
			name  string
			value string
		}{
			{name: "turn_id", value: string(arguments.TurnID)},
			{name: "worktree_id", value: string(arguments.WorktreeID)},
			{name: "from", value: arguments.From},
			{name: "to", value: arguments.To},
		} {
			if err := optionalText(fields, field.name, field.value); err != nil {
				return nil, err
			}
		}
		selectors := presentFields(fields, "turn_id", "worktree_id")
		if len(selectors) != 1 {
			return nil, result.NewError(result.CodeInvalidArgument, "diff kind=git requires exactly one of turn_id or worktree_id").
				WithField("turn_id")
		}
		if fields.has("to") && !fields.has("from") {
			return nil, result.NewError(result.CodeInvalidArgument, "from is required when to is provided").
				WithField("from")
		}
		if err := validatePaths(arguments.Paths); err != nil {
			return nil, err
		}
		return arguments, nil
	case DiffKindRevision:
		arguments, fields, err := decodeStrict[diffRevisionArguments](raw, "diff kind=revision")
		if err != nil {
			return nil, err
		}
		if err := requiredText(fields, "from", string(arguments.From)); err != nil {
			return nil, err
		}
		if err := requiredText(fields, "to", string(arguments.To)); err != nil {
			return nil, err
		}
		if err := validatePaths(arguments.Paths); err != nil {
			return nil, err
		}
		return arguments, nil
	default:
		return nil, enumError("kind", stringsOf(diffInputKinds))
	}
}

func decodeShellArguments(scope Scope, raw json.RawMessage) (toolArguments, error) {
	arguments, fields, err := decodeStrict[shellArguments](raw, ShellToolName)
	if err != nil {
		return nil, err
	}
	if err := arguments.scopeArguments.validate(scope, fields); err != nil {
		return nil, err
	}
	if err := requiredText(fields, "command", arguments.Command); err != nil {
		return nil, err
	}
	if err := validateCommand(arguments.Command); err != nil {
		return nil, err
	}
	if err := validateWorkdir(fields, arguments.Workdir); err != nil {
		return nil, err
	}
	return arguments, nil
}

func decodeApplyPatchArguments(scope Scope, raw json.RawMessage) (toolArguments, error) {
	arguments, fields, err := decodeStrict[applyPatchArguments](raw, ApplyPatchToolName)
	if err != nil {
		return nil, err
	}
	if err := arguments.scopeArguments.validate(scope, fields); err != nil {
		return nil, err
	}
	if err := requiredText(fields, "patch", arguments.Patch); err != nil {
		return nil, err
	}
	return arguments, nil
}

func decodeProcessArguments(scope Scope, raw json.RawMessage) (toolArguments, error) {
	op, err := requiredEnum(raw, "op", processOps)
	if err != nil {
		return nil, err
	}
	switch op {
	case ProcessOpStart:
		arguments, fields, err := decodeStrict[processStartArguments](raw, "process op=start")
		if err != nil {
			return nil, err
		}
		if err := arguments.scopeArguments.validate(scope, fields); err != nil {
			return nil, err
		}
		if err := requiredText(fields, "command", arguments.Command); err != nil {
			return nil, err
		}
		if err := validateCommand(arguments.Command); err != nil {
			return nil, err
		}
		if err := validateWorkdir(fields, arguments.Workdir); err != nil {
			return nil, err
		}
		if err := validateEnvironment(arguments.Env); err != nil {
			return nil, err
		}
		return arguments, nil
	case ProcessOpStatus:
		arguments, fields, err := decodeStrict[processStatusArguments](raw, "process op=status")
		if err != nil {
			return nil, err
		}
		if err := arguments.scopeArguments.validate(scope, fields); err != nil {
			return nil, err
		}
		if err := optionalText(fields, "process_id", string(arguments.ProcessID)); err != nil {
			return nil, err
		}
		if fields.has("state") && !contains(processStateFilters, arguments.State) {
			return nil, enumError("state", stringsOf(processStateFilters))
		}
		if fields.has("process_id") && fields.has("state") {
			return nil, invalidField("state", "process op=status with process_id")
		}
		return arguments, nil
	case ProcessOpRead:
		arguments, fields, err := decodeStrict[processReadArguments](raw, "process op=read")
		if err != nil {
			return nil, err
		}
		if err := arguments.scopeArguments.validate(scope, fields); err != nil {
			return nil, err
		}
		if err := requiredText(fields, "process_id", string(arguments.ProcessID)); err != nil {
			return nil, err
		}
		return arguments, nil
	case ProcessOpStop:
		arguments, fields, err := decodeStrict[processStopArguments](raw, "process op=stop")
		if err != nil {
			return nil, err
		}
		if err := arguments.scopeArguments.validate(scope, fields); err != nil {
			return nil, err
		}
		if err := requiredText(fields, "process_id", string(arguments.ProcessID)); err != nil {
			return nil, err
		}
		return arguments, nil
	case ProcessOpRestart:
		arguments, fields, err := decodeStrict[processRestartArguments](raw, "process op=restart")
		if err != nil {
			return nil, err
		}
		if err := arguments.scopeArguments.validate(scope, fields); err != nil {
			return nil, err
		}
		if err := requiredText(fields, "process_id", string(arguments.ProcessID)); err != nil {
			return nil, err
		}
		return arguments, nil
	case ProcessOpKill:
		arguments, fields, err := decodeStrict[processKillArguments](raw, "process op=kill")
		if err != nil {
			return nil, err
		}
		if err := arguments.scopeArguments.validate(scope, fields); err != nil {
			return nil, err
		}
		if err := requiredText(fields, "process_id", string(arguments.ProcessID)); err != nil {
			return nil, err
		}
		return arguments, nil
	default:
		return nil, enumError("op", stringsOf(processOps))
	}
}

func decodeSkillActivateArguments(scope Scope, raw json.RawMessage) (toolArguments, error) {
	arguments, fields, err := decodeStrict[skillActivateArguments](raw, SkillActivateToolName)
	if err != nil {
		return nil, err
	}
	if err := arguments.scopeArguments.validate(scope, fields); err != nil {
		return nil, err
	}
	if err := requiredText(fields, "skill", arguments.Skill); err != nil {
		return nil, err
	}
	if !validSkillName(arguments.Skill) {
		return nil, result.NewError(result.CodeInvalidArgument, "skill must be lowercase alphanumeric words separated by single hyphens").
			WithField("skill")
	}
	return arguments, nil
}

func (arguments scopeArguments) validate(scope Scope, fields fieldSet) error {
	switch scope {
	case TurnScope:
		if fields.has("repo") {
			return invalidField("repo", "turn-scoped operation")
		}
		return requiredText(fields, "turn_id", string(arguments.TurnID))
	case FreestyleScope:
		if fields.has("turn_id") {
			return invalidField("turn_id", "freestyle operation")
		}
		return requiredText(fields, "repo", arguments.Repo)
	default:
		return validateScope(scope)
	}
}

func argumentJSON(arguments any) (json.RawMessage, error) {
	if raw, ok := arguments.(json.RawMessage); ok {
		return append(json.RawMessage(nil), raw...), nil
	}
	data, err := json.Marshal(arguments)
	if err != nil {
		return nil, result.NewError(result.CodeInvalidArgument, "tool arguments cannot be encoded")
	}
	return data, nil
}

func decodeStrict[T any](raw json.RawMessage, subject string) (T, fieldSet, error) {
	var zero T
	fields, err := decodeFieldSet(raw)
	if err != nil {
		return zero, nil, err
	}
	allowed := jsonFields(reflect.TypeFor[T]())
	fieldNames := make([]string, 0, len(fields))
	for field := range fields {
		fieldNames = append(fieldNames, field)
	}
	sort.Strings(fieldNames)
	for _, field := range fieldNames {
		value := fields[field]
		if !allowed[field] {
			return zero, nil, unsupportedFieldError(subject, field, allowed)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return zero, nil, result.NewError(result.CodeInvalidArgument, field+" must not be null").
				WithField(field)
		}
	}
	var value T
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return zero, nil, decodeTypeError(err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return zero, nil, result.NewError(result.CodeInvalidArgument, "tool arguments must contain one JSON object")
	}
	return value, fields, nil
}

func decodeFieldSet(raw json.RawMessage) (fieldSet, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var fields fieldSet
	if err := decoder.Decode(&fields); err != nil {
		return nil, result.NewError(result.CodeInvalidArgument, "tool arguments must be a valid JSON object")
	}
	if fields == nil {
		return nil, result.NewError(result.CodeInvalidArgument, "tool arguments must be a JSON object")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, result.NewError(result.CodeInvalidArgument, "tool arguments must contain one JSON object")
	}
	return fields, nil
}

func requiredEnum[T ~string](raw json.RawMessage, field string, values []T) (T, error) {
	value, present, err := optionalEnum(raw, field, values)
	if err != nil {
		return "", err
	}
	if !present {
		return "", requiredFieldError(field)
	}
	return value, nil
}

func optionalEnum[T ~string](raw json.RawMessage, field string, values []T) (T, bool, error) {
	var zero T
	fields, err := decodeFieldSet(raw)
	if err != nil {
		return zero, false, err
	}
	value, ok := fields[field]
	if !ok {
		return zero, false, nil
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return zero, true, result.NewError(result.CodeInvalidArgument, field+" must not be null").
			WithField(field)
	}
	var decoded T
	if err := json.Unmarshal(value, &decoded); err != nil {
		return zero, true, typeError(field, "string")
	}
	if !contains(values, decoded) {
		return zero, true, enumError(field, stringsOf(values))
	}
	return decoded, true, nil
}

func decodeTypeError(err error) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if index := strings.LastIndexByte(field, '.'); index >= 0 {
			field = field[index+1:]
		}
		if field == "" {
			return result.NewError(result.CodeInvalidArgument, "tool arguments contain an invalid value")
		}
		return typeError(field, jsonTypeName(typeErr.Type))
	}
	return result.NewError(result.CodeInvalidArgument, "tool arguments must be a valid JSON object")
}

func jsonTypeName(value reflect.Type) string {
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return value.Kind().String()
	}
}

func jsonFields(value reflect.Type) map[string]bool {
	fields := map[string]bool{}
	collectJSONFields(value, fields)
	return fields
}

func collectJSONFields(value reflect.Type, fields map[string]bool) {
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	for index := range value.NumField() {
		field := value.Field(index)
		tag := field.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if field.Anonymous && name == "" {
			collectJSONFields(field.Type, fields)
			continue
		}
		if name != "" && name != "-" {
			fields[name] = true
		}
	}
}

func validatePaths(paths []string) error {
	seen := map[string]bool{}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			return result.NewError(result.CodeInvalidArgument, "paths must not contain empty values").
				WithField("paths")
		}
		if err := validateRelativePath("paths", path, "repository root"); err != nil {
			return err
		}
		if seen[path] {
			return result.NewError(result.CodeInvalidArgument, "paths must not contain duplicate values").
				WithField("paths")
		}
		seen[path] = true
	}
	return nil
}

func validateCommand(command string) error {
	if strings.ContainsRune(command, '\x00') {
		return result.NewError(result.CodeInvalidArgument, "command must not contain NUL bytes").
			WithField("command")
	}
	return nil
}

func validateWorkdir(fields fieldSet, workdir string) error {
	if err := optionalText(fields, "workdir", workdir); err != nil {
		return err
	}
	if !fields.has("workdir") {
		return nil
	}
	return validateRelativePath("workdir", workdir, "scoped workspace root")
}

func validateRelativePath(field, value, root string) error {
	if strings.ContainsRune(value, '\x00') {
		return result.NewError(result.CodeInvalidArgument, field+" must not contain NUL bytes").
			WithField(field)
	}
	if filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return result.NewError(result.CodeInvalidArgument, field+" must be relative to the "+root).
			WithField(field)
	}
	clean := filepath.Clean(value)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return result.NewError(result.CodeInvalidArgument, field+" must not escape the "+root).
			WithField(field)
	}
	return nil
}

func validateEnvironment(environment map[string]string) error {
	names := make([]string, 0, len(environment))
	for name := range environment {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		value := environment[name]
		if name == "" ||
			strings.ContainsRune(name, '=') ||
			strings.ContainsRune(name, '\x00') ||
			strings.ContainsRune(value, '\x00') {
			return result.NewError(result.CodeInvalidArgument, "env contains an invalid entry").
				WithField("env")
		}
	}
	return nil
}

func requiredText(fields fieldSet, field, value string) error {
	if !fields.has(field) {
		return requiredFieldError(field)
	}
	if strings.TrimSpace(value) == "" {
		return result.NewError(result.CodeInvalidArgument, field+" must not be empty").
			WithField(field)
	}
	return nil
}

func optionalText(fields fieldSet, field, value string) error {
	if !fields.has(field) {
		return nil
	}
	if strings.TrimSpace(value) == "" {
		return result.NewError(result.CodeInvalidArgument, field+" must not be empty").
			WithField(field)
	}
	return nil
}

func requiredFieldError(field string) error {
	return result.NewError(result.CodeInvalidArgument, field+" is required").
		WithField(field)
}

func unsupportedFieldError(subject, field string, allowed map[string]bool) error {
	fields := make([]string, 0, len(allowed))
	for name := range allowed {
		fields = append(fields, name)
	}
	return result.NewError(result.CodeInvalidArgument, "unsupported "+subject+" field").
		WithField(field).
		WithSuggestion("supported fields: " + joinFields(fields))
}

func invalidField(field, subject string) error {
	return result.NewError(result.CodeInvalidArgument, field+" is not valid for "+subject).
		WithField(field)
}

func mutuallyExclusiveError(fields ...string) error {
	return result.NewError(result.CodeInvalidArgument, strings.Join(fields, " and ")+" are mutually exclusive").
		WithField(fields[len(fields)-1])
}

func enumError(field string, values []string) error {
	return result.NewError(result.CodeInvalidArgument, "invalid "+field).
		WithField(field).
		WithSuggestion("valid values: " + joinFields(values))
}

func typeError(field, expected string) error {
	return result.NewError(result.CodeInvalidArgument, field+" must be a "+expected).
		WithField(field)
}

func presentFields(fields fieldSet, names ...string) []string {
	present := make([]string, 0, len(names))
	for _, name := range names {
		if fields.has(name) {
			present = append(present, name)
		}
	}
	return present
}

func (fields fieldSet) has(field string) bool {
	_, ok := fields[field]
	return ok
}

func contains[T comparable](values []T, target T) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func stringsOf[T ~string](values []T) []string {
	strings := make([]string, len(values))
	for index, value := range values {
		strings[index] = string(value)
	}
	return strings
}

func validSkillName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	previousHyphen := false
	for index, character := range name {
		switch {
		case character >= 'a' && character <= 'z':
			previousHyphen = false
		case character >= '0' && character <= '9':
			previousHyphen = false
		case character == '-':
			if index == 0 || previousHyphen {
				return false
			}
			previousHyphen = true
		default:
			return false
		}
	}
	return !previousHyphen
}
