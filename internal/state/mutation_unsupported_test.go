//go:build windows

package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/norunners/openrig/internal/state"
)

func TestWriteFileRejectsUnsupportedPlatformBeforePublication(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	err := root.WriteFile("state.json", []byte("state"), state.FileOptions{})
	if diff := cmp.Diff(
		state.CodeUnsupportedPlatform,
		state.CodeOf(err),
	); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	if _, statErr := os.Stat(filepath.Join(root.Path(), "state.json")); !os.IsNotExist(statErr) {
		t.Errorf("state file stat error = %v, expected not exist", statErr)
	}
}

func TestWriteJSONRejectsUnsupportedPlatformBeforeMarshaling(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	err := root.WriteJSON(
		"state.json",
		func() {},
		state.JSONOptions{Kind: "worktree"},
	)
	if diff := cmp.Diff(
		state.CodeUnsupportedPlatform,
		state.CodeOf(err),
	); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	if _, statErr := os.Stat(filepath.Join(root.Path(), "state.json")); !os.IsNotExist(statErr) {
		t.Errorf("state file stat error = %v, expected not exist", statErr)
	}
}
