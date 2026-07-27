package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestEnsureParentDirDurablyLinksEachCreatedDirectory(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "state", "worktrees", "wt_01")
	events := make([]string, 0, 12)
	relative := func(path string) string {
		value, err := filepath.Rel(parent, path)
		if err != nil {
			t.Fatalf("resolve relative event path: %v", err)
		}
		return value
	}
	ops := directoryOps{
		stat: os.Stat,
		mkdir: func(path string, mode os.FileMode) error {
			events = append(events, "mkdir "+relative(path))
			return os.Mkdir(path, mode)
		},
		chmod: func(path string, mode os.FileMode) error {
			events = append(events, "chmod "+relative(path))
			return os.Chmod(path, mode)
		},
		syncDir: func(path string) error {
			events = append(events, "sync "+relative(path))
			return syncDir(path)
		},
	}

	if err := ensureParentDir(target, 0o700, ops); err != nil {
		t.Fatalf("ensureParentDir returned error: %v", err)
	}
	expected := []string{
		"mkdir state",
		"chmod state",
		"sync state",
		"sync .",
		"mkdir state/worktrees",
		"chmod state/worktrees",
		"sync state/worktrees",
		"sync state",
		"mkdir state/worktrees/wt_01",
		"chmod state/worktrees/wt_01",
		"sync state/worktrees/wt_01",
		"sync state/worktrees",
	}
	if diff := cmp.Diff(expected, events); diff != "" {
		t.Errorf("mismatch directory durability order (-expected, +actual):\n%s", diff)
	}
}

func TestWriteFileClassifiesDirectorySyncFailureBeforePublicationAsIO(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "worktrees", "wt_01", "state.json")
	syncFailure := errors.New("sync failed")
	ops := osDirectoryOps()
	ops.syncDir = func(string) error {
		return syncFailure
	}

	err := writeFileAtomicWithDirectoryOps(path, []byte("state"), FileOptions{}, ops)
	if err == nil {
		t.Fatalf("writeFileAtomicWithDirectoryOps error = nil, expected %s", CodeIO)
	}
	if diff := cmp.Diff(CodeIO, CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	if !errors.Is(err, syncFailure) {
		t.Errorf("error does not preserve sync failure: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("final record stat error = %v, expected not exist", statErr)
	}
}

func TestWriteFileClassifiesDirectorySyncFailureAfterPublicationAsDurability(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "state.json")
	syncFailure := errors.New("sync failed")
	ops := osDirectoryOps()
	ops.syncDir = func(string) error {
		return syncFailure
	}

	err := writeFileAtomicWithDirectoryOps(path, []byte("state"), FileOptions{}, ops)
	if err == nil {
		t.Fatalf(
			"writeFileAtomicWithDirectoryOps error = nil, expected %s",
			CodeDurability,
		)
	}
	if diff := cmp.Diff(CodeDurability, CodeOf(err)); diff != "" {
		t.Errorf("mismatch error code (-expected, +actual):\n%s", diff)
	}
	if !errors.Is(err, syncFailure) {
		t.Errorf("error does not preserve sync failure: %v", err)
	}
	actual, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read published record: %v", readErr)
	}
	if diff := cmp.Diff("state", string(actual)); diff != "" {
		t.Errorf("mismatch published record (-expected, +actual):\n%s", diff)
	}
}
