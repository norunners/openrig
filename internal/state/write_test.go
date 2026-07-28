//go:build darwin || linux

package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWriteFileAtomicallyReplacesFileAndLeavesStaleTemps(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	path := filepath.Join(root.Path(), "state.json")
	staleTemp := filepath.Join(root.Path(), ".openrig-state-stale")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	if err := os.WriteFile(staleTemp, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale temp: %v", err)
	}

	if err := root.WriteFile("state.json", []byte("new"), FileOptions{}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	if diff := cmp.Diff("new", string(actual)); diff != "" {
		t.Errorf("mismatch final content (-expected, +actual):\n%s", diff)
	}
	stale, err := os.ReadFile(staleTemp)
	if err != nil {
		t.Fatalf("read stale temp: %v", err)
	}
	if diff := cmp.Diff("stale", string(stale)); diff != "" {
		t.Errorf("mismatch stale temp content (-expected, +actual):\n%s", diff)
	}
}

func TestWriteFileUsesRestrictivePermissions(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	if err := root.WriteFile("state.json", []byte("state"), FileOptions{}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	info, err := os.Stat(filepath.Join(root.Path(), "state.json"))
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if diff := cmp.Diff(os.FileMode(0o600), info.Mode().Perm()); diff != "" {
		t.Errorf("mismatch file mode (-expected, +actual):\n%s", diff)
	}
}

func TestWriteFileClassifiesParentSyncFailureAsDurability(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	syncFailure := errors.New("sync failed")

	err := root.writeFileAtomicWithSync(
		"state.json",
		filepath.Join(root.Path(), "state.json"),
		[]byte("state"),
		FileOptions{},
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
	actual, readErr := os.ReadFile(filepath.Join(root.Path(), "state.json"))
	if readErr != nil {
		t.Fatalf("read published record: %v", readErr)
	}
	if diff := cmp.Diff("state", string(actual)); diff != "" {
		t.Errorf("mismatch published record (-expected, +actual):\n%s", diff)
	}
}
func TestWriteFileRejectsFinalSymlinkAlias(t *testing.T) {
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

	err := root.WriteFile("operation.json", []byte("operation"), FileOptions{})
	if diff := cmp.Diff(CodeInvalid, CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	actual, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read target after rejected write: %v", readErr)
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch target content (-expected, +actual):\n%s", diff)
	}
	info, lstatErr := os.Lstat(alias)
	if lstatErr != nil {
		t.Fatalf("lstat alias after rejected write: %v", lstatErr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("alias mode = %s, expected symbolic link", info.Mode())
	}
}

func TestWriteFileRejectsInvalidResourceEntries(t *testing.T) {
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
			err := root.WriteFile(test.resource, nil, FileOptions{})
			if diff := cmp.Diff(CodeInvalid, CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestWriteFileCleansTemporaryFileAfterFailedPublication(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	if err := os.Mkdir(filepath.Join(root.Path(), "state.json"), 0o700); err != nil {
		t.Fatalf("create final-path blocker: %v", err)
	}

	err := root.writeFileAtomic(
		"state.json",
		filepath.Join(root.Path(), "state.json"),
		[]byte("state"),
		FileOptions{},
	)
	if err == nil {
		t.Fatal("writeFileAtomic error = nil, expected publication failure")
	}
	entries, readErr := os.ReadDir(root.Path())
	if readErr != nil {
		t.Fatalf("read state directory: %v", readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".openrig-state-") {
			t.Errorf("failed publication left temporary file %q", entry.Name())
		}
	}
}

func TestWriteFilePreservesResourceNameWhitespace(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	name := " state.json "
	if err := root.WriteFile(name, []byte("state"), FileOptions{}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root.Path(), name))
	if err != nil {
		t.Fatalf("read whitespace resource: %v", err)
	}
	if diff := cmp.Diff("state", string(data)); diff != "" {
		t.Errorf("mismatch resource content (-expected, +actual):\n%s", diff)
	}
}

func TestWriteFileRemainsAttachedAfterRootPathReplacement(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "state")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create state root: %v", err)
	}
	root := openInternalRoot(t, path)

	moved := filepath.Join(parent, "state-moved")
	if err := os.Rename(path, moved); err != nil {
		t.Fatalf("move opened state root: %v", err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	if err := os.Symlink("outside", path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := root.WriteFile("runtime.json", []byte("state"), FileOptions{}); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "runtime.json")); !os.IsNotExist(err) {
		t.Errorf("outside file stat error = %v, expected not exist", err)
	}
	data, err := os.ReadFile(filepath.Join(moved, "runtime.json"))
	if err != nil {
		t.Fatalf("read file beneath moved state root: %v", err)
	}
	if diff := cmp.Diff("state", string(data)); diff != "" {
		t.Errorf("mismatch moved-root content (-expected, +actual):\n%s", diff)
	}
}

func TestWriteFileRejectsEscapingIntermediateSymlink(t *testing.T) {
	root := openInternalRoot(t, t.TempDir())
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root.Path(), "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	err := root.WriteFile(
		filepath.Join("escape", "state.json"),
		[]byte("state"),
		FileOptions{},
	)
	if err == nil {
		t.Fatal("WriteFile error = nil, expected escaping symlink rejection")
	}
	if _, err := os.Stat(filepath.Join(outside, "state.json")); !os.IsNotExist(err) {
		t.Errorf("outside file stat error = %v, expected not exist", err)
	}
}

func openInternalRoot(t *testing.T, path string) *Root {
	t.Helper()
	root, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("close state root: %v", err)
		}
	})
	return root
}
