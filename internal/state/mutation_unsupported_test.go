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

func TestOpenRejectsMissingRootOnUnsupportedPlatform(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	root, err := state.Open(path)
	if root != nil {
		_ = root.Close()
		t.Error("Open result is non-nil after unsupported creation")
	}
	if diff := cmp.Diff(
		state.CodeUnsupportedPlatform,
		state.CodeOf(err),
	); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("state root stat error = %v, expected not exist", statErr)
	}
}

func TestRemoveRejectsUnsupportedPlatformBeforeMutation(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "state.json")
	if err := os.WriteFile(path, []byte("state"), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	err := root.Remove("state.json")
	if diff := cmp.Diff(
		state.CodeUnsupportedPlatform,
		state.CodeOf(err),
	); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	actual, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read state file: %v", readErr)
	}
	if diff := cmp.Diff("state", string(actual)); diff != "" {
		t.Errorf("mismatch state content (-expected, +actual):\n%s", diff)
	}
}
