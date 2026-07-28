//go:build darwin || linux

package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRemoveDurablyUnlinksFileAndAllowsMissingFile(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	if err := root.WriteFile("state.json", []byte("state"), FileOptions{}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if err := root.Remove("state.json"); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root.Path(), "state.json")); !os.IsNotExist(err) {
		t.Errorf("stat removed file error = %v, expected not exist", err)
	}
	if err := root.Remove("state.json"); err != nil {
		t.Errorf("Remove missing file returned error: %v", err)
	}
}

func TestRemoveClassifiesParentSyncFailureAsDurability(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "state.json")
	if err := os.WriteFile(path, []byte("state"), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}
	syncFailure := errors.New("sync failed")

	err := root.removeFile(
		"state.json",
		path,
		func(*os.Root, string) error {
			return syncFailure
		},
	)
	if diff := cmp.Diff(CodeDurability, CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	if !errors.Is(err, syncFailure) {
		t.Errorf("error does not preserve sync failure: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("removed file stat error = %v, expected not exist", statErr)
	}
}

func TestRemoveRejectsFinalSymlinkAlias(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	target := filepath.Join(root.Path(), "runtime.json")
	expected := []byte("runtime state\n")
	if err := os.WriteFile(target, expected, 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	alias := filepath.Join(root.Path(), "operation.json")
	if err := os.Symlink("runtime.json", alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	err := root.Remove("operation.json")
	if diff := cmp.Diff(CodeInvalid, CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	actual, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read target after rejected remove: %v", readErr)
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch target content (-expected, +actual):\n%s", diff)
	}
	info, lstatErr := os.Lstat(alias)
	if lstatErr != nil {
		t.Fatalf("lstat alias after rejected remove: %v", lstatErr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("alias mode = %s, expected symbolic link", info.Mode())
	}
}

func TestRemoveRejectsInvalidResourceEntries(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	if err := os.Mkdir(filepath.Join(root.Path(), "resource"), 0o700); err != nil {
		t.Fatalf("create directory fixture: %v", err)
	}
	tests := []struct {
		name     string
		resource string
	}{
		{
			name: "empty path",
		},
		{
			name:     "absolute root",
			resource: root.Path(),
		},
		{
			name:     "parent traversal",
			resource: filepath.Join("..", "state.json"),
		},
		{
			name:     "directory",
			resource: "resource",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := root.Remove(test.resource)
			if diff := cmp.Diff(CodeInvalid, CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
	info, err := os.Stat(filepath.Join(root.Path(), "resource"))
	if err != nil {
		t.Fatalf("stat directory after rejected remove: %v", err)
	}
	if !info.IsDir() {
		t.Error("fixture is no longer a directory")
	}
}

func TestRemoveRemainsAttachedAfterRootPathReplacement(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "state")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create state root: %v", err)
	}
	root := openInternalRoot(t, path)
	if err := root.WriteFile("state.json", []byte("inside"), FileOptions{}); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	moved := filepath.Join(parent, "state-moved")
	if err := os.Rename(path, moved); err != nil {
		t.Fatalf("move opened state root: %v", err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	outsidePath := filepath.Join(outside, "state.json")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write outside state file: %v", err)
	}
	if err := os.Symlink("outside", path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := root.Remove("state.json"); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(moved, "state.json")); !os.IsNotExist(err) {
		t.Errorf("moved-root file stat error = %v, expected not exist", err)
	}
	actual, err := os.ReadFile(outsidePath)
	if err != nil {
		t.Fatalf("read outside state file: %v", err)
	}
	if diff := cmp.Diff("outside", string(actual)); diff != "" {
		t.Errorf("mismatch outside content (-expected, +actual):\n%s", diff)
	}
}

func TestRemoveRejectsEscapingIntermediateSymlink(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	outside := t.TempDir()
	outsidePath := filepath.Join(outside, "state.json")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root.Path(), "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	err := root.Remove(filepath.Join("escape", "state.json"))
	if err == nil {
		t.Fatal("Remove error = nil, expected escaping symlink rejection")
	}
	actual, readErr := os.ReadFile(outsidePath)
	if readErr != nil {
		t.Fatalf("read outside file: %v", readErr)
	}
	if diff := cmp.Diff("outside", string(actual)); diff != "" {
		t.Errorf("mismatch outside content (-expected, +actual):\n%s", diff)
	}
}
