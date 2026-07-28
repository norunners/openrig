//go:build darwin || linux || windows

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
	expected := filepath.Join(stateHome, "openrig")
	if err := os.Mkdir(expected, 0o700); err != nil {
		t.Fatalf("create default state root: %v", err)
	}

	root := openStateRoot(t, "")
	if diff := cmp.Diff(expected, root.Path()); diff != "" {
		t.Errorf("mismatch default state root (-expected, +actual):\n%s", diff)
	}
}

func TestOpenExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	expected := filepath.Join(home, "openrig-state")
	if err := os.Mkdir(expected, 0o700); err != nil {
		t.Fatalf("create home state root: %v", err)
	}

	root := openStateRoot(t, "~"+string(filepath.Separator)+"openrig-state")
	if diff := cmp.Diff(expected, root.Path()); diff != "" {
		t.Errorf("mismatch expanded state root (-expected, +actual):\n%s", diff)
	}
}

func TestOpenPreservesBackslashAfterTildeWhenNotPathSeparator(t *testing.T) {
	if os.IsPathSeparator('\\') {
		t.Skip("backslash is a path separator on this platform")
	}

	cwd := t.TempDir()
	t.Chdir(cwd)
	expected := filepath.Join(cwd, `~\state`)
	if err := os.Mkdir(expected, 0o700); err != nil {
		t.Fatalf("create literal state root: %v", err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Mkdir(filepath.Join(home, "state"), 0o700); err != nil {
		t.Fatalf("create home state root: %v", err)
	}

	root := openStateRoot(t, `~\state`)
	if diff := cmp.Diff(expected, root.Path()); diff != "" {
		t.Errorf("mismatch literal state root (-expected, +actual):\n%s", diff)
	}
}

func TestOpenPreservesExplicitPathWhitespace(t *testing.T) {
	parent := t.TempDir()
	expected := filepath.Join(parent, " state ")
	if err := os.Mkdir(expected, 0o700); err != nil {
		t.Fatalf("create whitespace state root: %v", err)
	}

	root := openStateRoot(t, expected)
	if diff := cmp.Diff(expected, root.Path()); diff != "" {
		t.Errorf("mismatch state root (-expected, +actual):\n%s", diff)
	}
}

func TestOpenValidatesExistingStateRoot(t *testing.T) {
	parent := t.TempDir()
	directory := filepath.Join(parent, "state")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatalf("create directory root: %v", err)
	}
	regularFile := filepath.Join(parent, "state-file")
	if err := os.WriteFile(regularFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create file root: %v", err)
	}

	tests := []struct {
		name         string
		path         string
		expectedCode state.ErrorCode
	}{
		{
			name: "existing directory",
			path: directory,
		},
		{
			name:         "regular file",
			path:         regularFile,
			expectedCode: state.CodeInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := state.Open(test.path)
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
			t.Cleanup(func() {
				if err := actual.Close(); err != nil {
					t.Errorf("close state root: %v", err)
				}
			})
			if diff := cmp.Diff(test.path, actual.Path()); diff != "" {
				t.Errorf("mismatch root path (-expected, +actual):\n%s", diff)
			}
		})
	}
}

func TestOpenFollowsStateRootSymlink(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("create target root: %v", err)
	}
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink("target", alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	root := openStateRoot(t, alias)
	if diff := cmp.Diff(alias, root.Path()); diff != "" {
		t.Errorf("mismatch diagnostic root path (-expected, +actual):\n%s", diff)
	}
}

func TestCloseAcceptsNilRoot(t *testing.T) {
	var root *state.Root
	if err := root.Close(); err != nil {
		t.Errorf("nil Root.Close returned error: %v", err)
	}
}

func TestStateErrorPreservesDetailsAndCause(t *testing.T) {
	cause := errors.New("underlying failure")
	err := &state.Error{
		Code:    state.CodeIO,
		Path:    "/state",
		Message: "open state root",
		Err:     cause,
	}
	if !errors.Is(err, cause) {
		t.Errorf("state error does not preserve its cause: %v", err)
	}
	if err.Error() == "" {
		t.Error("state error text is empty")
	}
}

func openStateRoot(t *testing.T, path string) *state.Root {
	t.Helper()
	root, err := state.Open(path)
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
