package tools

import "fmt"

type helpCapabilities struct {
	scope     Scope
	available map[string]bool
}

func renderHelp(selection CatalogSelection, topic string) (helpResponse, error) {
	contracts, err := SelectContracts(selection)
	if err != nil {
		return helpResponse{}, err
	}
	if topic != "" && !contains(helpTopics, topic) {
		return helpResponse{}, enumError("topic", helpTopics)
	}

	capabilities := helpCapabilities{
		scope:     selection.Scope,
		available: catalogToolNames(contracts),
	}
	response := helpResponse{
		Summary:    helpSummary(capabilities),
		CommonFlow: commonHelpFlow(capabilities),
		Topic:      topic,
	}
	applyHelpTopic(&response, capabilities)
	return response, nil
}

func helpSummary(capabilities helpCapabilities) string {
	if capabilities.scope == FreestyleScope {
		if !capabilities.hasExecutableWork() {
			if capabilities.hasSkillGuidance() {
				return "Skill guidance is available, but no executable work capability is available in the current freestyle catalog."
			}
			return "No work capability is available in the current freestyle catalog."
		}
		if capabilities.usesProcessForCommonWork() {
			return "In freestyle scope, supervise process work against a repository directly; no turn_id is created."
		}
		return "In freestyle scope, each work call targets a repository directly; no turn_id is created."
	}
	if capabilities.canRunCommonTurnFlow() {
		if !capabilities.canInspectCommonDiff() {
			return "Begin a turn with a repository and goal, work with its turn_id, then end the turn."
		}
		return "Begin a turn with a repository and goal, work with its turn_id, inspect the diff, then end the turn."
	}
	if capabilities.has(TurnToolName) {
		if capabilities.has(DiffToolName) && capabilities.hasSkillGuidance() {
			return "Turn lifecycle, diff inspection, and skill guidance are available, but no executable work capability is available in the current catalog."
		}
		if capabilities.has(DiffToolName) {
			return "Turn lifecycle and diff inspection are available, but no executable work capability is available in the current catalog."
		}
		if capabilities.hasSkillGuidance() {
			return "Turn lifecycle and skill guidance are available, but no executable work capability is available in the current catalog."
		}
		return "Turn lifecycle is available, but no executable work capability is available in the current catalog."
	}
	if capabilities.hasExecutableWork() {
		if capabilities.has(DiffToolName) {
			return "Executable work requires an existing active turn_id; explicit diffs remain available with known worktree or revision selectors."
		}
		return "Available executable work requires an existing active turn_id; the current catalog cannot begin a turn."
	}
	if capabilities.has(DiffToolName) {
		if capabilities.has(WorktreeToolName) {
			return "Worktree lifecycle and explicit diff inspection are available without a turn; turn diffs require a known turn_id."
		}
		return "Explicit worktree, Git, and revision diffs are available with known selectors; turn diffs require a known turn_id."
	}
	if capabilities.has(WorktreeToolName) {
		return "Worktree lifecycle operations are available without a turn."
	}
	if capabilities.hasSkillGuidance() {
		return "Skill guidance requires an existing active turn_id; the current catalog cannot begin a turn."
	}
	return "OpenRig exposes only complete capabilities available in the current runtime."
}

func commonHelpFlow(capabilities helpCapabilities) []string {
	if capabilities.scope == FreestyleScope {
		if !capabilities.hasExecutableWork() {
			return []string{}
		}
		if capabilities.usesProcessForCommonWork() {
			return []string{
				"Start process work with repo set to the configured name, alias, or allowed path.",
				"Read process_id until it reaches a terminal state, then inspect exit_code and output.",
			}
		}
		return []string{"Call an available executable work tool with repo set to the configured name, alias, or allowed path."}
	}
	if !capabilities.canRunCommonTurnFlow() {
		return []string{}
	}

	flow := []string{"turn begin(repo, goal)"}
	if capabilities.usesProcessForCommonWork() {
		flow = append(flow,
			"process start with turn_id",
			"process read(process_id) until terminal, then inspect exit_code and output",
		)
	} else {
		flow = append(flow, "work tools with turn_id")
	}
	if capabilities.canInspectCommonDiff() {
		flow = append(flow, "diff(turn_id)")
	}
	if capabilities.usesProcessForCommonWork() {
		return append(flow, "turn end only after the process outcome is known")
	}
	return append(flow, "turn end(turn_id, outcome)")
}

func applyHelpTopic(response *helpResponse, capabilities helpCapabilities) {
	switch response.Topic {
	case "", helpTopicCommon:
		response.Examples = commonHelpExamples(capabilities)
	case helpTopicAdvanced:
		applyAdvancedHelp(response, capabilities)
	case helpTopicFreestyle:
		applyFreestyleHelp(response, capabilities)
	case helpTopicWorktree:
		applyWorktreeHelp(response, capabilities)
	case helpTopicTurn:
		applyTurnHelp(response, capabilities)
	case helpTopicDiff:
		applyDiffHelp(response, capabilities)
	case helpTopicStatus:
		applyCapabilityTopic(response, capabilities, StatusToolName,
			"Status is a compact current-session recovery and diagnostic view, not a required preflight call.",
			`{"tool":"status","arguments":{}}`,
		)
	case helpTopicIDs:
		response.Details = []string{
			"turn_id scopes normal work and audit attribution.",
			"worktree_id identifies a managed isolated checkout.",
			"process_id identifies one supervised logical process.",
			"revision IDs identify persisted OpenRig snapshots.",
			"MCP session IDs are transport correlation, not workspace authority.",
		}
	case helpTopicErrors:
		response.Details = []string{
			"Failures use bounded typed codes, safe messages, optional field and path attribution, and actionable suggestions.",
			"A failed operation may include one validated next action only when recovery is deterministic.",
			"If a client advertises a removed or unavailable tool, refresh or reconnect it and request tools/list again.",
		}
	case helpTopicSchemas:
		response.Details = []string{
			"Use tools/list for the effective tool names, descriptions, input schemas, and annotations.",
			"OpenRig omits full output schemas from the common catalog to keep agent context bounded; structured results remain documented and tested.",
			"Treat client metadata that differs from tools/list as stale and refresh or reconnect the integration.",
		}
	}
}

func applyAdvancedHelp(response *helpResponse, capabilities helpCapabilities) {
	if capabilities.has(WorktreeToolName) {
		response.Details = append(response.Details,
			"Use worktree directly only for explicit reuse, inspection, recovery, or cleanup.",
		)
		response.Examples = []string{
			`{"tool":"worktree","arguments":{"op":"open","repo":"openrig","base":"HEAD"}}`,
		}
	}
	if capabilities.has(ProcessToolName) {
		detail := "Use process for supervised long-running commands; process_id is the sole identity of a supervised process."
		if capabilities.scope == TurnScope && !capabilities.has(TurnToolName) {
			detail = "Process requires an existing active turn_id because the current catalog cannot begin a turn; process_id is the sole identity of a supervised process."
		}
		response.Details = append(response.Details, detail)
		response.Examples = append(response.Examples, processWorkExamples(capabilities)...)
	}
	if capabilities.hasSkillGuidance() {
		response.Details = append(response.Details,
			"skill_activate loads known local skill guidance as text; it does not execute repository work.",
		)
	}
	if len(response.Details) == 0 {
		response.Details = []string{"No advanced lifecycle or process capability is available in the current catalog."}
	}
}

func applyFreestyleHelp(response *helpResponse, capabilities helpCapabilities) {
	if capabilities.scope == FreestyleScope {
		if !capabilities.hasExecutableWork() {
			if capabilities.hasSkillGuidance() {
				response.Details = []string{
					"Skill guidance is available, but no executable work capability is available in the current freestyle catalog.",
				}
				return
			}
			response.Details = []string{"No executable work capability is available in the current freestyle catalog."}
			return
		}
		if capabilities.usesProcessForCommonWork() {
			response.Details = []string{
				"Freestyle process work is scoped directly to a repository.",
				"Start the process with repo, then read process_id until its terminal state and exit code are known.",
			}
			response.Examples = processWorkExamples(capabilities)
			return
		}
		response.Details = []string{
			"Freestyle scopes each work call directly to a repository.",
			"Pass repo to each work tool; turn_id and worktree lifecycle operations are unavailable.",
		}
		if examples := commonWorkExamples(capabilities); len(examples) != 0 {
			response.Examples = examples
		}
		return
	}
	if !capabilities.has(TurnToolName) {
		response.Details = []string{
			"Freestyle turns are unavailable because turn is not selected in the current catalog.",
		}
		return
	}
	if !capabilities.hasExecutableWork() {
		response.Details = []string{
			"Turn can begin in freestyle mode, but no executable work capability is available in the current catalog.",
		}
		return
	}
	response.Details = []string{
		"Freestyle turns are an explicit setup or recovery escape hatch when an isolated worktree is not appropriate.",
		"Begin with mode=freestyle and repo, then use the returned turn_id for normal work tools.",
	}
	response.Examples = []string{
		`{"tool":"turn","arguments":{"op":"begin","mode":"freestyle","repo":"openrig","goal":"Inspect repository setup"}}`,
	}
}

func applyWorktreeHelp(response *helpResponse, capabilities helpCapabilities) {
	if !capabilities.has(WorktreeToolName) {
		response.Details = []string{"worktree is not available in the current catalog."}
		return
	}
	detail := "Worktree manages isolated Git checkouts directly."
	if capabilities.has(TurnToolName) {
		detail += " turn begin with repo creates one automatically for the common workflow."
	}
	response.Details = []string{detail}
	response.Examples = []string{
		`{"tool":"worktree","arguments":{"op":"open","repo":"openrig","base":"HEAD"}}`,
	}
}

func applyTurnHelp(response *helpResponse, capabilities helpCapabilities) {
	if !capabilities.has(TurnToolName) {
		response.Details = []string{"turn is not available in the current catalog."}
		return
	}
	if !capabilities.hasExecutableWork() {
		response.Details = []string{
			"Turn is the scoped capability and audit unit, but no executable work capability is available in the current catalog.",
		}
		return
	}
	response.Details = []string{
		"A turn is the scoped capability and audit unit. Begin requires a goal; normal work tools require its turn_id.",
	}
	response.Examples = []string{
		`{"tool":"turn","arguments":{"op":"begin","repo":"openrig","goal":"Fix the parser"}}`,
	}
}

func commonHelpExamples(capabilities helpCapabilities) []string {
	if capabilities.scope == FreestyleScope {
		if !capabilities.hasExecutableWork() {
			return nil
		}
		return commonWorkExamples(capabilities)
	}
	if !capabilities.canRunCommonTurnFlow() {
		return nil
	}

	examples := []string{
		`{"tool":"turn","arguments":{"op":"begin","repo":"openrig","goal":"Fix the parser"}}`,
	}
	examples = append(examples, commonWorkExamples(capabilities)...)
	if capabilities.canInspectCommonDiff() {
		examples = append(examples,
			`{"tool":"diff","arguments":{"turn_id":"turn_..."}}`,
		)
	}
	if capabilities.usesProcessForCommonWork() {
		return append(examples,
			`{"tool":"turn","arguments":{"op":"end","turn_id":"turn_...","outcome":"completed","summary":"Process completed successfully."}}`,
		)
	}
	return append(examples,
		`{"tool":"turn","arguments":{"op":"end","turn_id":"turn_...","outcome":"completed","summary":"Implemented and verified."}}`,
	)
}

func commonWorkExamples(capabilities helpCapabilities) []string {
	scopeField := `"turn_id":"turn_..."`
	if capabilities.scope == FreestyleScope {
		scopeField = `"repo":"openrig"`
	}
	switch {
	case capabilities.has(ShellToolName):
		return []string{fmt.Sprintf(
			`{"tool":"shell","arguments":{%s,"command":"go test ./..."}}`,
			scopeField,
		)}
	case capabilities.has(ApplyPatchToolName):
		return []string{fmt.Sprintf(
			`{"tool":"apply_patch","arguments":{%s,"patch":"*** Begin Patch\n*** Update File: README.md\n@@\n-old\n+new\n*** End Patch\n"}}`,
			scopeField,
		)}
	case capabilities.has(ProcessToolName):
		return processWorkExamples(capabilities)
	default:
		return nil
	}
}

func processWorkExamples(capabilities helpCapabilities) []string {
	scopeField := `"turn_id":"turn_..."`
	if capabilities.scope == FreestyleScope {
		scopeField = `"repo":"openrig"`
	}
	return []string{
		fmt.Sprintf(
			`{"tool":"process","arguments":{%s,"op":"start","command":"go test ./..."}}`,
			scopeField,
		),
		fmt.Sprintf(
			`{"tool":"process","arguments":{%s,"op":"read","process_id":"proc_..."}}`,
			scopeField,
		),
	}
}

func applyDiffHelp(response *helpResponse, capabilities helpCapabilities) {
	if !capabilities.has(DiffToolName) {
		response.Details = []string{"diff is not available in the current catalog."}
		return
	}
	if capabilities.has(TurnToolName) {
		response.Details = []string{
			"Supplying only turn_id renders the common turn diff. Explicit kinds support advanced worktree, Git, or revision comparisons.",
		}
		response.Examples = []string{`{"tool":"diff","arguments":{"turn_id":"turn_..."}}`}
		if capabilities.has(WorktreeToolName) {
			response.Details = append(response.Details,
				"Use worktree_id with kind=worktree or kind=git when comparing a managed checkout directly.",
			)
			response.Examples = append(response.Examples,
				`{"tool":"diff","arguments":{"kind":"worktree","worktree_id":"wt_..."}}`,
			)
		}
		return
	}
	if capabilities.has(WorktreeToolName) {
		response.Details = []string{
			"Open or inspect a managed worktree, then use its worktree_id with kind=worktree or kind=git; no turn_id is required.",
			"Explicit revision comparisons remain available when both revision IDs are known.",
		}
		response.Examples = []string{
			`{"tool":"worktree","arguments":{"op":"open","repo":"openrig","base":"HEAD"}}`,
			`{"tool":"diff","arguments":{"kind":"worktree","worktree_id":"wt_..."}}`,
		}
		return
	}
	response.Details = []string{
		"Explicit revision comparisons require known revision IDs. Worktree and Git comparisons similarly require a known worktree_id.",
		"The common turn diff remains available when an active or ended turn_id is known.",
	}
	response.Examples = []string{
		`{"tool":"diff","arguments":{"kind":"revision","from":"rev_...","to":"rev_..."}}`,
	}
}

func applyCapabilityTopic(response *helpResponse, capabilities helpCapabilities, tool, detail, example string) {
	if !capabilities.has(tool) {
		response.Details = []string{fmt.Sprintf("%s is not available in the current catalog.", tool)}
		return
	}
	response.Details = []string{detail}
	response.Examples = []string{example}
}

func (capabilities helpCapabilities) has(tool string) bool {
	return capabilities.available[tool]
}

func (capabilities helpCapabilities) hasExecutableWork() bool {
	for _, name := range []string{
		ShellToolName,
		ApplyPatchToolName,
		ProcessToolName,
	} {
		if capabilities.has(name) {
			return true
		}
	}
	return false
}

func (capabilities helpCapabilities) hasSkillGuidance() bool {
	return capabilities.has(SkillActivateToolName)
}

func (capabilities helpCapabilities) usesProcessForCommonWork() bool {
	return capabilities.has(ProcessToolName) &&
		!capabilities.has(ShellToolName) &&
		!capabilities.has(ApplyPatchToolName)
}

func (capabilities helpCapabilities) canRunCommonTurnFlow() bool {
	return capabilities.scope == TurnScope &&
		capabilities.has(TurnToolName) &&
		capabilities.hasExecutableWork()
}

func (capabilities helpCapabilities) canInspectCommonDiff() bool {
	return capabilities.canRunCommonTurnFlow() && capabilities.has(DiffToolName)
}
