package diagnostics

const (
	StateHealthy                = "healthy"
	StateDegraded               = "degraded"
	StateOperatorActionRequired = "operator_action_required"

	ActionCompleted              = "completed"
	ActionFinished               = "finished"
	ActionRolledBack             = "rolled_back"
	ActionOperatorActionRequired = "operator_action_required"
)

// Issue describes a lifecycle consistency finding and any recovery already performed.
type Issue struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Action      string `json:"action"`
	OperationID string `json:"operation_id,omitempty"`
	WorktreeID  string `json:"worktree_id,omitempty"`
	TurnID      string `json:"turn_id,omitempty"`
	Path        string `json:"path,omitempty"`
}

// Report summarizes runtime lifecycle health for operator-facing status.
type Report struct {
	State  string  `json:"state"`
	Issues []Issue `json:"issues,omitempty"`
}

// NewReport derives the highest-severity state from issues and removes duplicates.
func NewReport(issues []Issue) Report {
	var unique []Issue
	if len(issues) > 0 {
		unique = make([]Issue, 0, len(issues))
	}
	seen := make(map[Issue]struct{}, len(issues))
	for _, issue := range issues {
		if _, ok := seen[issue]; ok {
			continue
		}
		seen[issue] = struct{}{}
		unique = append(unique, issue)
	}
	report := Report{State: StateHealthy, Issues: unique}
	for _, issue := range unique {
		if issue.Action == ActionOperatorActionRequired {
			report.State = StateOperatorActionRequired
			return report
		}
		report.State = StateDegraded
	}
	return report
}

// Merge combines reports and derives their aggregate state.
func Merge(reports ...Report) Report {
	issues := []Issue{}
	for _, report := range reports {
		issues = append(issues, report.Issues...)
	}
	return NewReport(issues)
}
