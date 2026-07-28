package state_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/norunners/openrig/internal/state"
)

func TestOpenUsesXDGStateHome(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	root := openStateRoot(t, "")
	physicalStateHome, err := filepath.EvalSymlinks(stateHome)
	if err != nil {
		t.Fatalf("resolve state home symlinks: %v", err)
	}
	expected := filepath.Join(physicalStateHome, "openrig")
	if diff := cmp.Diff(expected, root.Path()); diff != "" {
		t.Errorf("mismatch default state root (-expected, +actual):\n%s", diff)
	}
}

func TestOpenExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := openStateRoot(t, "~/openrig-state")
	physicalHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatalf("resolve home symlinks: %v", err)
	}
	expected := filepath.Join(physicalHome, "openrig-state")
	if diff := cmp.Diff(expected, root.Path()); diff != "" {
		t.Errorf("mismatch expanded state root (-expected, +actual):\n%s", diff)
	}
}

func TestOpenValidatesStateRootType(t *testing.T) {
	parent := t.TempDir()
	physicalParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatalf("resolve parent symlinks: %v", err)
	}
	directory := filepath.Join(parent, "state")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("create directory root: %v", err)
	}
	regularFile := filepath.Join(parent, "state-file")
	if err := os.WriteFile(regularFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create file root: %v", err)
	}

	tests := []struct {
		name          string
		path          string
		expectedCode  state.ErrorCode
		expectedPath  string
		requiresLinks bool
	}{
		{
			name:         "missing path",
			path:         filepath.Join(parent, "missing"),
			expectedPath: filepath.Join(physicalParent, "missing"),
		},
		{
			name:         "existing directory",
			path:         directory,
			expectedPath: filepath.Join(physicalParent, "state"),
		},
		{
			name:         "regular file",
			path:         regularFile,
			expectedCode: state.CodeInvalid,
		},
		{
			name:          "symlink to directory",
			path:          filepath.Join(parent, "directory-link"),
			expectedPath:  filepath.Join(physicalParent, "state"),
			requiresLinks: true,
		},
		{
			name:          "symlink to regular file",
			path:          filepath.Join(parent, "file-link"),
			expectedCode:  state.CodeInvalid,
			requiresLinks: true,
		},
	}
	if err := os.Symlink(directory, filepath.Join(parent, "directory-link")); err != nil {
		for index := range tests {
			if tests[index].requiresLinks {
				tests[index].path = ""
			}
		}
	}
	if err := os.Symlink(regularFile, filepath.Join(parent, "file-link")); err != nil {
		for index := range tests {
			if tests[index].requiresLinks {
				tests[index].path = ""
			}
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.requiresLinks && test.path == "" {
				t.Skip("symlinks unavailable")
			}
			actual, err := state.Open(test.path)
			if state.IsCode(err, state.CodeUnsupportedPlatform) {
				t.Skip(err)
			}
			if test.expectedCode != "" {
				if err == nil {
					t.Fatalf("Open error = nil, expected %s", test.expectedCode)
				}
				if diff := cmp.Diff(test.expectedCode, state.CodeOf(err)); diff != "" {
					t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
				}
				var stateErr *state.Error
				if !errors.As(err, &stateErr) {
					t.Errorf("error type = %T, expected *state.Error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Open returned error: %v", err)
			}
			if diff := cmp.Diff(test.expectedPath, actual.Path()); diff != "" {
				t.Errorf("mismatch root path (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestResolveRejectsLexicalEscape(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	_, err := root.Resolve(filepath.Join(root.Path(), "..", "escaped"))
	if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
}

func TestResolveRejectsSymlinkEscapes(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	outside := t.TempDir()
	link := filepath.Join(root.Path(), "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	tests := []struct {
		name string
		path string
	}{
		{
			name: "existing symlink",
			path: link,
		},
		{
			name: "missing descendant",
			path: filepath.Join(link, "new", "state.json"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := root.Resolve(test.path)
			if err == nil {
				t.Fatal("Resolve error = nil, expected error")
			}
			if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
				t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestResolveAllowsMissingDescendants(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	actual, err := root.Resolve("worktrees/wt_01/state.json")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	expected := filepath.Join(root.Path(), "worktrees", "wt_01", "state.json")
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch resolved path (-expected, +actual):\n%s", diff)
	}
}

func TestResolveUsesContainedIntermediateSymlink(t *testing.T) {
	root := openStateRoot(t, t.TempDir())
	physicalDir := filepath.Join(root.Path(), "physical")
	if err := os.Mkdir(physicalDir, 0o700); err != nil {
		t.Fatalf("create physical directory: %v", err)
	}
	link := filepath.Join(root.Path(), "link")
	if err := os.Symlink(physicalDir, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	actual, err := root.Resolve(filepath.Join(link, "state.json"))
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	expected := filepath.Join(physicalDir, "state.json")
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("mismatch resolved path (-expected, +actual):\n%s", diff)
	}
}

func TestResolveRejectsUninitializedRoot(t *testing.T) {
	var root state.Root
	_, err := root.Resolve("state.json")
	if diff := cmp.Diff(state.CodeInvalid, state.CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
}

func openStateRoot(t *testing.T, path string) *state.Root {
	t.Helper()
	root, err := state.Open(path)
	if state.IsCode(err, state.CodeUnsupportedPlatform) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	return root
}
