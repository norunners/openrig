package diagnostics_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/norunners/openrig/internal/diagnostics"
)

func TestNewReportUsesHighestSeverity(t *testing.T) {
	tests := []struct {
		name     string
		issues   []diagnostics.Issue
		expected string
	}{
		{
			name:     "healthy",
			expected: diagnostics.StateHealthy,
		},
		{
			name: "recovered",
			issues: []diagnostics.Issue{
				{Action: diagnostics.ActionRolledBack},
			},
			expected: diagnostics.StateDegraded,
		},
		{
			name: "operator action",
			issues: []diagnostics.Issue{
				{Action: diagnostics.ActionFinished},
				{Action: diagnostics.ActionOperatorActionRequired},
			},
			expected: diagnostics.StateOperatorActionRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := diagnostics.NewReport(test.issues)
			if diff := cmp.Diff(test.expected, actual.State); diff != "" {
				t.Errorf("mismatch report state (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestMergeDeduplicatesIdenticalIssues(t *testing.T) {
	issue := diagnostics.Issue{
		Code:       "WORKTREE_STATE_INVALID",
		Message:    "record header is incomplete",
		Action:     diagnostics.ActionOperatorActionRequired,
		WorktreeID: "wt_01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Path:       "/state/worktrees/wt_01ARZ3NDEKTSV4RRFFQ69G5FAV/state.json",
	}

	actual := diagnostics.Merge(diagnostics.NewReport([]diagnostics.Issue{issue}), diagnostics.NewReport([]diagnostics.Issue{issue}))
	expected := diagnostics.Report{
		State:  diagnostics.StateOperatorActionRequired,
		Issues: []diagnostics.Issue{issue},
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch merged report (-expected, +actual):\n%s", diff)
	}
}
