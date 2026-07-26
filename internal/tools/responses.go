package tools

import (
	"encoding/json"
	"fmt"
)

type ToolName string

// responseList keeps successful collection responses unambiguous: an absent
// Go slice is still an explicitly empty JSON array.
type responseList[T any] []T

func (values responseList[T]) MarshalJSON() ([]byte, error) {
	if values == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]T(values))
}

type fileProvenance string

// diffFileStatus is intentionally path-oriented. A rename is normalized to a
// deleted old path and an added new path. A copy contributes only its added
// destination because the source remains present. diffSummary.FilesChanged
// counts the resulting normalized path entries.
type diffFileStatus string
type patchAction string
type skillOrigin string

const (
	fileProvenanceKnownWrite             fileProvenance = "known_write"
	fileProvenanceObservedExternalChange fileProvenance = "observed_external_change"
	fileProvenancePreexistingChange      fileProvenance = "preexisting_change"
	fileProvenanceUnknownChange          fileProvenance = "unknown_change"

	diffFileStatusAdded    diffFileStatus = "added"
	diffFileStatusModified diffFileStatus = "modified"
	diffFileStatusDeleted  diffFileStatus = "deleted"

	patchActionAdd    patchAction = "add"
	patchActionUpdate patchAction = "update"
	patchActionDelete patchAction = "delete"
	patchActionMove   patchAction = "move"

	skillOriginUser    skillOrigin = "user"
	skillOriginProject skillOrigin = "project"
)

// helpResponse describes OpenRig's lifecycle without duplicating tools/list.
type helpResponse struct {
	Summary    string   `json:"summary"`
	CommonFlow []string `json:"common_flow"`
	Topic      string   `json:"topic,omitempty"`
	Details    []string `json:"details,omitempty"`
	Examples   []string `json:"examples,omitempty"`
}

// statusResponse reports only state attributable to the current session.
// Historical resource queries belong to worktree, turn, and process.
type statusResponse struct {
	Ready      bool               `json:"ready"`
	ActiveTurn *activeTurnSummary `json:"active_turn,omitempty"`
}

type activeTurnSummary struct {
	TurnID     TurnID     `json:"turn_id"`
	Repo       string     `json:"repo"`
	WorktreeID WorktreeID `json:"worktree_id,omitempty"`
}

type worktreeResponse struct {
	Worktree worktree `json:"worktree"`
}

type worktreeListResponse struct {
	Worktrees responseList[worktree] `json:"worktrees"`
}

type worktree struct {
	WorktreeID   WorktreeID    `json:"worktree_id"`
	State        WorktreeState `json:"state"`
	SourceRoot   string        `json:"source_root"`
	WorktreeRoot string        `json:"worktree_root"`
	Base         string        `json:"base"`
	BaseSHA      string        `json:"base_sha"`
	Branch       string        `json:"branch,omitempty"`
	Reason       string        `json:"reason,omitempty"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
	LastTurnID   TurnID        `json:"last_turn_id,omitempty"`
	ActiveTurnID TurnID        `json:"active_turn_id,omitempty"`
}

type turnResponse struct {
	Turn turn `json:"turn"`
}

type turnListResponse struct {
	Turns responseList[turn] `json:"turns"`
}

type nextActionResponse struct {
	Next nextAction `json:"next"`
}

// nextAction is returned only when one complete, deterministic, and safe
// recovery operation is available. Failed envelopes carry it as partial data.
type nextAction struct {
	Tool      ToolName        `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

func nextTurnBegin(arguments turnBeginArguments) (nextAction, error) {
	arguments.Op = TurnOpBegin
	data, err := json.Marshal(arguments)
	if err != nil {
		return nextAction{}, fmt.Errorf("marshal turn begin action: %w", err)
	}
	if _, err := decodeArguments(TurnToolName, TurnScope, json.RawMessage(data)); err != nil {
		return nextAction{}, fmt.Errorf("validate turn begin action: %w", err)
	}
	return nextAction{
		Tool:      ToolName(TurnToolName),
		Arguments: data,
	}, nil
}

func nextWorktreeClose(arguments worktreeCloseArguments) (nextAction, error) {
	arguments.Op = WorktreeOpClose
	data, err := json.Marshal(arguments)
	if err != nil {
		return nextAction{}, fmt.Errorf("marshal worktree close action: %w", err)
	}
	if _, err := decodeArguments(WorktreeToolName, TurnScope, json.RawMessage(data)); err != nil {
		return nextAction{}, fmt.Errorf("validate worktree close action: %w", err)
	}
	return nextAction{
		Tool:      ToolName(WorktreeToolName),
		Arguments: data,
	}, nil
}

// turn is the stable resource representation returned by begin, status, and
// end. Operation-specific details are omitted when they do not apply.
type turn struct {
	TurnID          TurnID            `json:"turn_id"`
	Mode            TurnMode          `json:"mode"`
	Worktree        *turnWorktree     `json:"worktree,omitempty"`
	Workspace       workspace         `json:"workspace"`
	State           TurnState         `json:"state"`
	Outcome         TurnOutcome       `json:"outcome,omitempty"`
	Goal            string            `json:"goal"`
	Summary         string            `json:"summary,omitempty"`
	BeginRevisionID RevisionID        `json:"begin_revision_id,omitempty"`
	EndRevisionID   RevisionID        `json:"end_revision_id,omitempty"`
	StartedAt       string            `json:"started_at"`
	EndedAt         string            `json:"ended_at,omitempty"`
	DurationMS      int64             `json:"duration_ms,omitempty"`
	ChangedFiles    []string          `json:"changed_files,omitempty"`
	Files           []fileChange      `json:"files,omitempty"`
	GitState        *gitState         `json:"git_state,omitempty"`
	Instructions    []instructionFile `json:"instructions,omitempty"`
	Skills          []skillSummary    `json:"skills,omitempty"`
	// Warnings are persisted lifecycle warnings for this turn. Warnings from
	// the current tool call belong on result.Envelope.
	Warnings []string `json:"warnings,omitempty"`
}

type turnWorktree struct {
	WorktreeID WorktreeID    `json:"worktree_id"`
	BaseSHA    string        `json:"base_sha"`
	State      WorktreeState `json:"state"`
	Retained   bool          `json:"retained"`
}

type workspace struct {
	CWD     string `json:"cwd"`
	GitRoot string `json:"git_root"`
	Branch  string `json:"branch,omitempty"`
}

type gitState struct {
	Status            string   `json:"status"`
	Clean             bool     `json:"clean"`
	Staged            []string `json:"staged"`
	Unstaged          []string `json:"unstaged"`
	Untracked         []string `json:"untracked"`
	UnstagedDiffBytes int      `json:"unstaged_diff_bytes"`
	StagedDiffBytes   int      `json:"staged_diff_bytes"`
}

type instructionFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type skillSummary struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Origin      skillOrigin `json:"origin"`
}

type fileChange struct {
	Path        string         `json:"path"`
	Provenance  fileProvenance `json:"provenance"`
	ToolCallIDs []string       `json:"tool_call_ids,omitempty"`
}

type diffResponse struct {
	Kind       DiffKind    `json:"kind"`
	TurnID     TurnID      `json:"turn_id,omitempty"`
	WorktreeID WorktreeID  `json:"worktree_id,omitempty"`
	From       string      `json:"from,omitempty"`
	To         string      `json:"to,omitempty"`
	Paths      []string    `json:"paths,omitempty"`
	Summary    diffSummary `json:"summary"`
	Files      []diffFile  `json:"files,omitempty"`
	Patch      string      `json:"patch,omitempty"`
}

type diffSummary struct {
	FilesChanged int  `json:"files_changed"`
	Additions    int  `json:"additions"`
	Deletions    int  `json:"deletions"`
	Bytes        int  `json:"bytes"`
	Truncated    bool `json:"truncated"`
}

type diffFile struct {
	Path      string         `json:"path"`
	Status    diffFileStatus `json:"status,omitempty"`
	Additions int            `json:"additions,omitempty"`
	Deletions int            `json:"deletions,omitempty"`
	Binary    bool           `json:"binary,omitempty"`
}

type shellResponse struct {
	CWD        string        `json:"cwd"`
	Stdout     outputExcerpt `json:"stdout"`
	Stderr     outputExcerpt `json:"stderr"`
	ExitCode   int           `json:"exit_code"`
	TimedOut   bool          `json:"timed_out"`
	DurationMS int64         `json:"duration_ms"`
}

// outputExcerpt contains one stream under the runtime response budget. When
// truncated, Text contains one-third head and two-thirds tail at valid UTF-8
// boundaries. OmittedBytes is zero when Truncated is false.
type outputExcerpt struct {
	Text         string `json:"text"`
	Truncated    bool   `json:"truncated"`
	OmittedBytes int64  `json:"omitted_bytes,omitempty"`
}

type applyPatchResponse struct {
	CWD   string             `json:"cwd"`
	Files []patchFileSummary `json:"files"`
}

type patchFileSummary struct {
	Action       patchAction `json:"action"`
	Path         string      `json:"path"`
	OldPath      string      `json:"old_path,omitempty"`
	AddedLines   int         `json:"added_lines"`
	DeletedLines int         `json:"deleted_lines"`
	Bytes        int         `json:"bytes"`
}

type processResponse struct {
	Process processInfo `json:"process"`
}

type processListResponse struct {
	Processes responseList[processInfo] `json:"processes"`
}

type processReadResponse struct {
	Output processOutput `json:"output"`
}

type processRestartResponse struct {
	Process  processInfo `json:"process"`
	Previous processInfo `json:"previous"`
}

type processInfo struct {
	ProcessID    ProcessID    `json:"process_id"`
	CWD          string       `json:"cwd"`
	Command      string       `json:"command"`
	PID          int          `json:"pid,omitempty"`
	State        ProcessState `json:"state"`
	StartedAt    string       `json:"started_at"`
	EndedAt      string       `json:"ended_at,omitempty"`
	ExitCode     *int         `json:"exit_code,omitempty"`
	Generation   int          `json:"generation"`
	RestartCount int          `json:"restart_count"`
}

type processOutput struct {
	ProcessID    ProcessID     `json:"process_id"`
	PID          int           `json:"pid,omitempty"`
	State        ProcessState  `json:"state"`
	ExitCode     *int          `json:"exit_code,omitempty"`
	Generation   int           `json:"generation"`
	RestartCount int           `json:"restart_count"`
	Stdout       outputExcerpt `json:"stdout"`
	Stderr       outputExcerpt `json:"stderr"`
	// Cursor is the continuation cursor for the next process op=read call.
	Cursor string `json:"cursor"`
}

type skillActivateResponse struct {
	Name                string             `json:"name"`
	Description         string             `json:"description"`
	Instructions        string             `json:"instructions"`
	Origin              skillOrigin        `json:"origin"`
	Scripts             []skillFileContent `json:"scripts,omitempty"`
	ScriptsTruncated    bool               `json:"scripts_truncated,omitempty"`
	References          []skillFileContent `json:"references,omitempty"`
	ReferencesTruncated bool               `json:"references_truncated,omitempty"`
}

type skillFileContent struct {
	Name      string `json:"name"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}
